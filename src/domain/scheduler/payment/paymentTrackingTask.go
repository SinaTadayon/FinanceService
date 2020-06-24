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
					{"paymentMode", entities.AutomaticPaymentMode},
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

			// TODO implement manual payment mode
			if sellerFinance.Payment.Mode == entities.AutomaticPaymentMode {
				pipeline.automaticPaymentHandler(ctx, sellerFinance)
			}

			financeOutStream <- sellerFinance
		}
	}
}

func (pipeline *Pipeline) automaticPaymentHandler(ctx context.Context, sellerFinance *entities.SellerFinance) {
	if sellerFinance.Payment.Status == entities.PaymentNoneState {
		if int(sellerFinance.Payment.RetryRequest) < app.Globals.Config.App.SellerFinanceRetryAutomaticPaymentRequest {
			log.GLog.Logger.Info("start automatic finance payment transfer money",
				"fn", "automaticPaymentHandler",
				"fid", sellerFinance.FId,
				"sellerId", sellerFinance.SellerId,
				"retryRequest", sellerFinance.Payment.RetryRequest)
			pipeline.financeTrackingAutomaticPayment(ctx, sellerFinance)
		} else {
			log.GLog.Logger.Info("reach to maximum automatic retry payment request, change to manual finance payment request transfer money",
				"fn", "automaticPaymentHandler",
				"fid", sellerFinance.FId,
				"sellerId", sellerFinance.SellerId,
				"retryResult", sellerFinance.Payment.RetryResult)

			timestamp := time.Now().UTC()
			sellerFinance.PaymentMode = entities.ManualPaymentMode
			sellerFinance.UpdatedAt = timestamp

			payment := &entities.FinancePayment{
				TransferRequest:  sellerFinance.Payment.TransferRequest,
				TransferResponse: sellerFinance.Payment.TransferResponse,
				TransferResult:   sellerFinance.Payment.TransferResult,
				Status:           sellerFinance.Payment.Status,
				Mode:             sellerFinance.Payment.Mode,
				Action:           sellerFinance.Payment.Action,
				RetryRequest:     sellerFinance.Payment.RetryRequest,
				RetryResult:      sellerFinance.Payment.RetryResult,
				CreatedAt:        sellerFinance.Payment.CreatedAt,
				UpdatedAt:        timestamp,
			}

			sellerFinance.PaymentHistory = make([]*entities.FinancePayment, 0, 5)
			sellerFinance.PaymentHistory = append(sellerFinance.PaymentHistory, payment)

			sellerFinance.Payment.Mode = entities.ManualPaymentMode
			sellerFinance.Payment.UpdatedAt = timestamp
		}

	} else if sellerFinance.Payment.Status == entities.PaymentPendingState {
		if int(sellerFinance.Payment.RetryResult) < app.Globals.Config.App.SellerFinanceRetryAutomaticPaymentResult {
			log.GLog.Logger.Debug("tracking finance payment transfer money",
				"fn", "automaticPaymentHandler",
				"fid", sellerFinance.FId,
				"sellerId", sellerFinance.SellerId)

			iFuture := app.Globals.PaymentService.GetSingleTransferMoneyResult(ctx, sellerFinance.FId,
				sellerFinance.Payment.TransferResponse.TransferId).Get()

			if iFuture.Error() != nil {
				log.GLog.Logger.Error("PaymentService.GetSingleTransferMoneyResult failed",
					"fn", "automaticPaymentHandler",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"error", iFuture.Error())

				timestamp := time.Now().UTC()
				sellerFinance.Payment.RetryResult++
				sellerFinance.Payment.UpdatedAt = timestamp
				sellerFinance.UpdatedAt = timestamp

				iFuture := app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("sellerFinance transfer money result failed",
						"fn", "automaticPaymentHandler",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error())
				}
				return
			}

			timestamp := time.Now().UTC()
			transferResult := iFuture.Data().(payment_service.TransferMoneyResult)
			var success int64
			if transferResult.Pending == 0 {
				success = transferResult.Total
			} else {
				success = transferResult.Total - transferResult.Pending
			}

			if sellerFinance.Payment.TransferResult == nil {
				sellerFinance.Payment.TransferResult = &entities.TransferResult{
					TransferId: transferResult.TransferId,

					TotalTransfer: &entities.Money{
						Amount:   strconv.Itoa(int(transferResult.Total)),
						Currency: transferResult.Currency,
					},

					SuccessTransfer: &entities.Money{
						Amount:   strconv.Itoa(int(success)),
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
					CreatedAt: timestamp,
					UpdatedAt: timestamp,
				}

				if sellerFinance.PaymentHistory == nil {
					sellerFinance.PaymentHistory = make([]*entities.FinancePayment, 0, 5)
				}

				sellerFinance.PaymentHistory = append(sellerFinance.PaymentHistory, sellerFinance.Payment)

			} else {
				isResultChanged := false

				if sellerFinance.Payment.TransferResult.TotalTransfer.Amount != strconv.Itoa(int(transferResult.Total)) ||
					sellerFinance.Payment.TransferResult.SuccessTransfer.Amount != strconv.Itoa(int(success)) ||
					sellerFinance.Payment.TransferResult.PendingTransfer.Amount != strconv.Itoa(int(transferResult.Pending)) ||
					sellerFinance.Payment.TransferResult.FailedTransfer.Amount != strconv.Itoa(int(transferResult.Failed)) {
					isResultChanged = true
				}

				sellerFinance.Payment.TransferResult.TotalTransfer = &entities.Money{
					Amount:   strconv.Itoa(int(transferResult.Total)),
					Currency: transferResult.Currency,
				}

				sellerFinance.Payment.TransferResult.SuccessTransfer = &entities.Money{
					Amount:   strconv.Itoa(int(success)),
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
				sellerFinance.Payment.TransferResult.UpdatedAt = timestamp

				if isResultChanged {
					sellerFinance.PaymentHistory = append(sellerFinance.PaymentHistory, sellerFinance.Payment)
				}
			}

			sellerFinance.Payment.UpdatedAt = timestamp

			if transferResult.Pending == 0 {
				log.GLog.Logger.Debug("finance transfer money complete",
					"fn", "automaticPaymentHandler",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"result", transferResult)

				if transferResult.Total == 0 {
					sellerFinance.Payment.Status = entities.PaymentFailedState
				} else if transferResult.Failed == 0 {
					sellerFinance.Payment.Status = entities.PaymentSuccessState
				} else {
					sellerFinance.Payment.Status = entities.PaymentPartialState
				}

				sellerFinance.Status = entities.FinanceClosedStatus
			} else {
				log.GLog.Logger.Debug("finance transfer money in progress . . .",
					"fn", "automaticPaymentHandler",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"result", transferResult)

				sellerFinance.Payment.RetryResult++
			}
		} else {
			log.GLog.Logger.Info("reach to maximum automatic retry payment result, change to manual finance payment result transfer money",
				"fn", "automaticPaymentHandler",
				"fid", sellerFinance.FId,
				"sellerId", sellerFinance.SellerId,
				"retryResult", sellerFinance.Payment.RetryResult)

			timestamp := time.Now().UTC()
			sellerFinance.PaymentMode = entities.ManualPaymentMode
			sellerFinance.UpdatedAt = timestamp

			if sellerFinance.PaymentHistory == nil {
				sellerFinance.PaymentHistory = make([]*entities.FinancePayment, 0, 5)
			}

			payment := &entities.FinancePayment{
				TransferRequest:  sellerFinance.Payment.TransferRequest,
				TransferResponse: sellerFinance.Payment.TransferResponse,
				TransferResult:   sellerFinance.Payment.TransferResult,
				Status:           sellerFinance.Payment.Status,
				Mode:             sellerFinance.Payment.Mode,
				Action:           sellerFinance.Payment.Action,
				RetryRequest:     sellerFinance.Payment.RetryRequest,
				RetryResult:      sellerFinance.Payment.RetryResult,
				CreatedAt:        sellerFinance.Payment.CreatedAt,
				UpdatedAt:        timestamp,
			}
			sellerFinance.PaymentHistory = append(sellerFinance.PaymentHistory, payment)

			sellerFinance.Payment.Mode = entities.ManualPaymentMode
			sellerFinance.Payment.UpdatedAt = timestamp
		}
	}

	iFuture := app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("sellerFinance transfer money result failed",
			"fn", "automaticPaymentHandler",
			"fid", sellerFinance.FId,
			"sellerId", sellerFinance.SellerId,
			"error", iFuture.Error())
	}
}

func (pipeline *Pipeline) financeTrackingAutomaticPayment(ctx context.Context, sellerFinance *entities.SellerFinance) {

	requestTimestamp := time.Now().UTC()
	paymentRequest := payment_service.PaymentRequest{
		FId:                sellerFinance.FId,
		SellerId:           sellerFinance.SellerId,
		Amount:             sellerFinance.Invoice.ShareRoundupTotal.Amount,
		Currency:           sellerFinance.Invoice.ShareRoundupTotal.Currency,
		ReceiverName:       sellerFinance.SellerInfo.GeneralInfo.ShopDisplayName,
		ReceiverAccountId:  sellerFinance.SellerInfo.FinanceData.Iban,
		PaymentDescription: sellerFinance.FId + "-bazlia",
		PaymentType:        payment_service.SellerPayment,
	}

	iFuture := app.Globals.PaymentService.SingleTransferMoney(ctx, paymentRequest).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("SingleTransferMoney for seller payment failed",
			"fn", "financePayment",
			"fid", sellerFinance.FId,
			"sellerId", sellerFinance.SellerId,
			"error", iFuture.Error())

		sellerFinance.Payment.UpdatedAt = time.Now().UTC()
		sellerFinance.UpdatedAt = time.Now().UTC()
		sellerFinance.Payment.RetryRequest++
		return
	}

	responseTimestamp := time.Now().UTC()
	paymentResponse := iFuture.Data().(payment_service.PaymentResponse)

	sellerFinance.Payment.TransferRequest = &entities.TransferRequest{
		TotalPrice: entities.Money{
			Amount:   sellerFinance.Invoice.ShareRoundupTotal.Amount,
			Currency: sellerFinance.Invoice.ShareRoundupTotal.Currency,
		},
		ReceiverName:       sellerFinance.SellerInfo.GeneralInfo.ShopDisplayName,
		ReceiverAccountId:  sellerFinance.SellerInfo.FinanceData.Iban,
		PaymentDescription: sellerFinance.FId + "-bazlia",
		TransferType:       string(payment_service.SellerPayment),
		CreatedAt:          requestTimestamp,
	}

	sellerFinance.Payment.TransferResponse = &entities.TransferResponse{
		TransferId: paymentResponse.TransferId,
		CreatedAt:  responseTimestamp,
	}

	sellerFinance.Payment.UpdatedAt = responseTimestamp
	sellerFinance.UpdatedAt = responseTimestamp
}
