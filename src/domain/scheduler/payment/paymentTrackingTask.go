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
	"go.mongodb.org/mongo-driver/bson"
	"strconv"
	"sync"
	"time"
)

func PaymentTrackingTask(ctx context.Context) future.IFuture {
	financeStream, err := findFinancePaymentProcessState(ctx)
	if err != nil {
		return future.Factory().
			SetError(future.InternalError, "findFinancePaymentProcessState failed", errors.Wrap(err, "findFinancePaymentProcessState failed")).
			BuildAndSend()
	}

	financeChannelStream, err := fanOutPaymentFinances(ctx, financeStream)
	if err != nil {
		return future.Factory().
			SetError(future.InternalError, "fanOutPaymentFinances failed", errors.Wrap(err, "fanOutPaymentFinances failed")).
			BuildAndSend()
	}

	financeOutStream, err := fanInPaymentTrackingStreams(ctx, financeChannelStream)
	if err != nil {
		return future.Factory().
			SetError(future.InternalError, "fanInPaymentTrackingStreams failed", errors.Wrap(err, "fanInPaymentTrackingStreams failed")).
			BuildAndSend()
	}

	for {
		select {
		case <-ctx.Done():
			return future.Factory().SetData(struct{}{}).BuildAndSend()
		case _, ok := <-financeOutStream:
			if !ok {
				return future.Factory().SetData(struct{}{}).BuildAndSend()
			}
		}
	}
}

func findFinancePaymentProcessState(ctx context.Context) (financeReaderStream, error) {

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

			financeResult := iFuture.Data().(*finance_repository.FinancePageableResult)

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

func fanOutPaymentFinances(ctx context.Context, financeStream financeReaderStream) (financeChannelStream, error) {
	financeWriterChannels := make([]financeWriterStream, 0, app.Globals.Config.Mongo.MinPoolSize/2)
	financeChannelStream := make(chan financeReaderStream)

	fanOutTask := func() {
		defer func() {
			log.GLog.Logger.Debug("complete",
				"fn", "fanOutPaymentFinances")
			for _, stream := range financeWriterChannels {
				close(stream)
			}

			close(financeChannelStream)
		}()

		index := 0
		for sellerFinance := range financeStream {
			//log.GLog.Logger.Debug("received order",
			//	"fn", "FanOutOrders",
			//	"oid", sellerOrder.OId)
			select {
			case <-ctx.Done():
				return
			default:
			}

			if index >= cap(financeWriterChannels) {
				index = 0
			}

			if financeWriterChannels[index] == nil {
				financeStream := make(chan *entities.SellerFinance, DefaultFanOutStreamBuffer)
				financeWriterChannels[index] = financeStream
				financeChannelStream <- financeStream
				financeStream <- sellerFinance
			} else {
				financeWriterChannels[index] <- sellerFinance
			}
			index++
		}
	}

	if err := app.Globals.WorkerPool.SubmitTask(fanOutTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "fanOutPaymentFinances",
			"error", err)

		return nil, err
	}

	return financeChannelStream, nil

}

func fanInPaymentTrackingStreams(ctx context.Context, financeChannelStream financeChannelStream) (financeReaderStream, error) {
	multiplexedFinanceStream := make(chan *entities.SellerFinance)

	var wg sync.WaitGroup
	fanInTask := func() {
		defer func() {
			close(multiplexedFinanceStream)
		}()
		for financeChannel := range financeChannelStream {
			select {
			case <-ctx.Done():
				break
			default:
			}

			financeStream, financePaymentTrackingFn := financePaymentTracking()
			financePaymentTrackingTask := func() {
				financePaymentTrackingFn(ctx, financeChannel)
			}

			if err := app.Globals.WorkerPool.SubmitTask(financePaymentTrackingTask); err != nil {
				log.GLog.Logger.Error("submit financePaymentTrackingTask to WorkerPool.SubmitTask failed",
					"fn", "fanInPaymentTrackingStreams", "error", err)
				continue
			}

			fanInMultiplexTask := func() {
				defer wg.Done()
				for finance := range financeStream {
					//log.GLog.Logger.Debug("received order",
					//	"fn", "FanInOrderStreams",
					//	"sellerId", finance.SellerId)
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
					"fn", "fanInPaymentTrackingStreams", "error", err)
				wg.Done()
			}
		}

		wg.Wait()
	}

	if err := app.Globals.WorkerPool.SubmitTask(fanInTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "fanInFinanceStreams",
			"error", err)

		return nil, err
	}

	return multiplexedFinanceStream, nil
}

func financePaymentTracking() (financeReaderStream, financePipelineFunc) {
	financeOutStream := make(chan *entities.SellerFinance)

	return financeOutStream, func(ctx context.Context, financeInStream financeReaderStream) {
		defer close(financeOutStream)

		for sellerFinance := range financeInStream {
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
				_ = financeStartPayment(ctx, sellerFinance)
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
				if transferResult.Pending == 0 {
					log.GLog.Logger.Debug("finance transfer money complete",
						"fn", "financePaymentTracking",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"result", transferResult)

					sellerFinance.Payment.TransferResult = &entities.TransferResult{
						TransferId: transferResult.TransferId,
						SuccessTransfer: &entities.Money{
							Amount:   strconv.Itoa(int(transferResult.Total)),
							Currency: transferResult.Currency,
						},
						FailedTransfer: &entities.Money{
							Amount:   strconv.Itoa(int(transferResult.Failed)),
							Currency: transferResult.Currency,
						},
						CreatedAt: time.Now().UTC(),
					}

					if transferResult.Total == 0 {
						sellerFinance.Payment.Status = entities.TransferFailedState
					} else if transferResult.Failed == 0 {
						sellerFinance.Payment.Status = entities.TransferSuccessState
					} else {
						sellerFinance.Payment.Status = entities.TransferPartialState
					}

					iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
					if iFuture.Error() != nil {
						log.GLog.Logger.Error("sellerFinance transfer money result failed",
							"fn", "financeProcess",
							"fid", sellerFinance.FId,
							"sellerId", sellerFinance.SellerId,
							"error", iFuture.Error())
					}
				} else {
					log.GLog.Logger.Debug("finance transfer money in progress . . .",
						"fn", "financePaymentTracking",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"result", transferResult)
				}
			}

			financeOutStream <- sellerFinance
		}
	}
}

func financeStartPayment(ctx context.Context, sellerFinance *entities.SellerFinance) error {

	requestTimestamp := time.Now().UTC()
	paymentRequest := payment_service.PaymentRequest{
		FId:                sellerFinance.FId,
		SellerId:           sellerFinance.SellerId,
		Amount:             sellerFinance.Invoice.ShareRoundupTotal.Amount,
		Currency:           sellerFinance.Invoice.ShareRoundupTotal.Currency,
		ReceiverName:       sellerFinance.SellerInfo.GeneralInfo.ShopDisplayName,
		ReceiverAccountId:  sellerFinance.SellerInfo.FinanceData.Iban,
		PaymentDescription: "",
		PaymentType:        payment_service.SellerPayment,
	}

	iFuture := app.Globals.PaymentService.SingleTransferMoney(ctx, paymentRequest).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("SingleTransferMoney for seller payment failed",
			"fn", "financePayment",
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
			"fn", "financeProcess",
			"fid", sellerFinance.FId,
			"sellerId", sellerFinance.SellerId,
			"error", iFuture.Error())
		return iFuture.Error()
	}

	return nil
}
