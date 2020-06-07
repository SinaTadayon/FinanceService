package payment_scheduler

import (
	"context"
	"github.com/pkg/errors"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	payment_service "gitlab.faza.io/services/finance/infrastructure/services/payment"
	worker_pool "gitlab.faza.io/services/finance/infrastructure/workerPool"
	"go.mongodb.org/mongo-driver/bson"
	"strconv"
	"sync"
	"time"
)

func PaymentTrackingTask(ctx context.Context) future.IFuture {
	financeStream, err := findPaymentPendingFinance(ctx)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "findPaymentPendingFinance failed", errors.Wrap(err, "findPaymentPendingFinance failed")).
			BuildAndSend()
	}

	financeChannelStream, err := fanOutFinancePipeline(ctx, financeStream)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "fanOutPaymentFinances failed", errors.Wrap(err, "fanOutPaymentFinances failed")).
			BuildAndSend()
	}

	financeOutStream, err := fanInPipelineStream(ctx, financeChannelStream)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "fanInPaymentTrackingStreams failed", errors.Wrap(err, "fanInPaymentTrackingStreams failed")).
			BuildAndSend()
	}

	for {
		select {
		case <-ctx.Done():
			return future.FactorySync().SetData(struct{}{}).BuildAndSend()
		case _, ok := <-financeOutStream:
			if !ok {
				return future.FactorySync().SetData(struct{}{}).BuildAndSend()
			}
		}
	}
}

func findPaymentPendingFinance(ctx context.Context) (FinanceReaderStream, error) {

	financeStream := make(chan *entities.SellerFinance, DefaultFinanceStreamSize)
	findPendingFinanceTask := func() {
		defer func() {
			close(financeStream)
		}()
		var availablePages = 1
		for i := 0; i < availablePages; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			iFuture := app.Globals.SellerFinanceRepository.FindByFilterWithPage(ctx, func() interface{} {
				return bson.D{
					{"status", entities.FinancePaymentProcessStatus},
					{"deletedAt", nil}}
			}, int64(i+1), DefaultFetchFinancePerPage).Get()

			if iFuture.Error() != nil {
				if iFuture.Error().Code() != future.NotFound {
					log.GLog.Logger.Error("SellerFinanceRepository.FindByFilterWithPage failed",
						"fn", "findPaymentProcessFinance",
						"error", iFuture.Error())
					return
				}
				log.GLog.Logger.Info("payment process finance not found",
					"fn", "findPaymentProcessFinance")
				return
			}

			financeResult := iFuture.Data().(finance_repository.FinancePageableResult)

			if financeResult.TotalCount%DefaultFetchFinancePerPage != 0 {
				availablePages = (int(financeResult.TotalCount) / DefaultFetchFinancePerPage) + 1
			} else {
				availablePages = int(financeResult.TotalCount) / DefaultFetchFinancePerPage
			}

			if financeResult.TotalCount < DefaultFetchFinancePerPage {
				availablePages = 1
			}

			for _, finance := range financeResult.SellerFinances {
				select {
				case <-ctx.Done():
					return
				case financeStream <- finance:
				}
			}
		}
	}

	if err := app.Globals.WorkerPool.SubmitTask(findPendingFinanceTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "findPaymentProcessFinance",
			"error", err)

		return nil, err
	}

	return financeStream, nil
}

func fanOutFinancePipeline(ctx context.Context, financeStream FinanceReaderStream) (PipelineInStream, error) {
	financeWriterChannels := make([]FinanceWriterStream, 0, app.Globals.Config.Mongo.MinPoolSize/2)
	pipelineStream := make(chan *Pipeline)

	fanOutTask := func() {
		defer func() {
			for _, stream := range financeWriterChannels {
				close(stream)
			}

			close(pipelineStream)
		}()

		index := 0
		initIndex := 0
		for sellerFinance := range financeStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if initIndex < cap(financeWriterChannels) {
				financeStream := make(chan *entities.SellerFinance, DefaultFanOutStreamBuffer)
				financeWriterChannels = append(financeWriterChannels, financeStream)
				pipelineStream <- &Pipeline{dataStream: financeStream}
				initIndex++
			}

			if index > len(financeWriterChannels) {
				index = 0
			}

			financeWriterChannels[index] <- sellerFinance
			index++
		}
	}

	if err := app.Globals.WorkerPool.SubmitTask(fanOutTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "fanOutFinancePipeline",
			"error", err)

		return nil, err
	}

	return pipelineStream, nil

}

func fanInPipelineStream(ctx context.Context, pipelineStream PipelineInStream) (FinanceReaderStream, error) {
	multiplexedFinanceStream := make(chan *entities.SellerFinance)

	var wg sync.WaitGroup
	fanInTask := func() {
		defer func() {
			close(multiplexedFinanceStream)
		}()
		for pipeline := range pipelineStream {
			select {
			case <-ctx.Done():
				break
			default:
			}

			financeStream, pipelineTask := pipeline.financePaymentTracking(ctx)

			if err := app.Globals.WorkerPool.SubmitTask(pipelineTask); err != nil {
				log.GLog.Logger.Error("submit pipelineTask to WorkerPool.SubmitTask failed",
					"fn", "fanInPipelineStream", "error", err)
				continue
			}

			fanInMultiplexTask := func() {
				defer wg.Done()
				for finance := range financeStream {
					select {
					case <-ctx.Done():
						break
					case multiplexedFinanceStream <- finance:
					}
				}
			}

			wg.Add(1)
			if err := app.Globals.WorkerPool.SubmitTask(fanInMultiplexTask); err != nil {
				log.GLog.Logger.Error("submit fanInMultiplexTask to WorkerPool.SubmitTask failed",
					"fn", "fanInPipelineStream", "error", err)
				wg.Done()
			}
		}

		wg.Wait()
	}

	if err := app.Globals.WorkerPool.SubmitTask(fanInTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "fanInPipelineStream",
			"error", err)

		return nil, err
	}

	return multiplexedFinanceStream, nil
}

func (pipeline *Pipeline) financePaymentTracking(ctx context.Context) (FinanceReaderStream, worker_pool.Task) {
	financeOutStream := make(chan *entities.SellerFinance)

	return financeOutStream, func() {
		defer close(financeOutStream)

		for sellerFinance := range pipeline.dataStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if sellerFinance.Payment == nil {
				log.GLog.Logger.Debug("start finance payment transfer money",
					"fn", "financePaymentTracking",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId)
				err := pipeline.financeTrackingStartPayment(ctx, sellerFinance)
				if err != nil {
					log.GLog.Logger.Warn("financePaymentTracking StartPayment failed",
						"fn", "financePaymentTracking",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId)
				}
			} else if sellerFinance.Payment.Status == entities.TransferPendingState {
				log.GLog.Logger.Debug("tracking finance payment transfer money",
					"fn", "financePaymentTracking",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId)

				iFuture := app.Globals.PaymentService.GetSingleTransferMoneyResult(ctx, sellerFinance.FId,
					sellerFinance.Payment.TransferResponse.TransferId).Get()

				if iFuture.Error() != nil {
					log.GLog.Logger.Error("PaymentService.GetSingleTransferMoneyResult failed",
						"fn", "financePaymentTracking",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error())
					continue
				}

				transferResult := iFuture.Data().(payment_service.TransferMoneyResult)
				if sellerFinance.Payment.TransferResult == nil {
					sellerFinance.Payment.TransferResult = &entities.TransferResult{
						TransferId: transferResult.TransferId,
						SuccessTransfer: &entities.Money{
							Amount:   strconv.Itoa(int(transferResult.Total)),
							Currency: transferResult.Currency,
						},

						PendingTransfer: &entities.Money{
							Amount:   strconv.Itoa(int(transferResult.Pending)),
							Currency: transferResult.Currency,
						},

						FailedTransfer: &entities.Money{
							Amount:   strconv.Itoa(int(transferResult.Failed)),
							Currency: transferResult.Currency,
						},
						CreatedAt: time.Now().UTC(),
						UpdatedAt: time.Now().UTC(),
					}
				} else {
					sellerFinance.Payment.TransferResult.SuccessTransfer = &entities.Money{
						Amount:   strconv.Itoa(int(transferResult.Total)),
						Currency: transferResult.Currency,
					}

					sellerFinance.Payment.TransferResult.PendingTransfer = &entities.Money{
						Amount:   strconv.Itoa(int(transferResult.Pending)),
						Currency: transferResult.Currency,
					}

					sellerFinance.Payment.TransferResult.FailedTransfer = &entities.Money{
						Amount:   strconv.Itoa(int(transferResult.Failed)),
						Currency: transferResult.Currency,
					}
					sellerFinance.Payment.TransferResult.UpdatedAt = time.Now().UTC()
				}

				sellerFinance.Payment.UpdatedAt = sellerFinance.Payment.TransferResult.UpdatedAt

				if transferResult.Pending == 0 {
					log.GLog.Logger.Debug("finance transfer money complete",
						"fn", "financePaymentTracking",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"result", transferResult)

					if transferResult.Total == 0 {
						sellerFinance.Payment.Status = entities.TransferFailedState
					} else if transferResult.Failed == 0 {
						sellerFinance.Payment.Status = entities.TransferSuccessState
					} else {
						sellerFinance.Payment.Status = entities.TransferPartialState
					}

					sellerFinance.Status = entities.FinanceClosedStatus
				} else {
					log.GLog.Logger.Debug("finance transfer money in progress . . .",
						"fn", "financePaymentTracking",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"result", transferResult)
				}

				iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("sellerFinance transfer money result failed",
						"fn", "financeProcess",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error())
					continue
				}
			}

			financeOutStream <- sellerFinance
		}
	}
}

func (pipeline *Pipeline) financeTrackingStartPayment(ctx context.Context, sellerFinance *entities.SellerFinance) error {

	requestTimestamp := time.Now().UTC()
	paymentRequest := payment_service.PaymentRequest{
		FId:                sellerFinance.FId,
		SellerId:           sellerFinance.SellerId,
		Amount:             sellerFinance.Invoice.ShareRoundupTotal.Amount,
		Currency:           sellerFinance.Invoice.ShareRoundupTotal.Currency,
		ReceiverName:       sellerFinance.SellerInfo.GeneralInfo.ShopDisplayName,
		ReceiverAccountId:  sellerFinance.SellerInfo.FinanceData.Iban,
		PaymentDescription: sellerFinance.FId + "پرداخت صورت حساب شماره ",
		PaymentType:        payment_service.SellerPayment,
	}

	iFuture := app.Globals.PaymentService.SingleTransferMoney(ctx, paymentRequest).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("SingleTransferMoney for seller payment failed",
			"fn", "financeTrackingStartPayment",
			"fid", sellerFinance.FId,
			"sellerId", sellerFinance.SellerId,
			"error", iFuture.Error())
		return iFuture.Error()
	}

	responseTimestamp := time.Now().UTC()
	paymentResponse := iFuture.Data().(payment_service.PaymentResponse)

	sellerFinance.Payment = &entities.FinancePayment{
		TransferRequest: &entities.TransferRequest{
			TotalPrice:         *sellerFinance.Invoice.ShareRoundupTotal,
			ReceiverName:       sellerFinance.SellerInfo.GeneralInfo.ShopDisplayName,
			ReceiverAccountId:  sellerFinance.SellerInfo.FinanceData.Iban,
			PaymentDescription: "",
			TransferType:       string(payment_service.SellerPayment),
			CreatedAt:          requestTimestamp,
		},
		TransferResponse: &entities.TransferResponse{
			TransferId: paymentResponse.TransferId,
			CreatedAt:  responseTimestamp,
		},
		TransferResult: nil,
		Status:         entities.TransferPendingState,
		CreatedAt:      requestTimestamp,
		UpdatedAt:      responseTimestamp,
	}

	sellerFinance.Status = entities.FinancePaymentProcessStatus
	iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("sellerFinance update after transfer money request failed",
			"fn", "financeTrackingStartPayment",
			"fid", sellerFinance.FId,
			"sellerId", sellerFinance.SellerId,
			"error", iFuture.Error())
		return iFuture.Error()
	}

	return nil
}
