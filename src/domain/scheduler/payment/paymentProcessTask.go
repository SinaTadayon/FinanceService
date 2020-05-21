package payment_scheduler

import (
	"context"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/calculation"
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

const (
	DefaultFinanceStreamSize   = 1024
	DefaultFanOutStreamBuffer  = 128
	DefaultFetchFinancePerPage = 64
)

type financeWriterStream chan<- *entities.SellerFinance
type financeReaderStream <-chan *entities.SellerFinance
type financeChannelStream <-chan financeReaderStream

type financePipelineFunc func(ctx context.Context, financeStream financeReaderStream)

func PaymentProcessTask(ctx context.Context) future.IFuture {

	financeStream, err := findEligibleFinance(ctx)
	if err != nil {
		return future.Factory().
			SetError(future.InternalError, "findEligibleFinance failed", errors.Wrap(err, "findEligibleFinance failed")).
			BuildAndSend()
	}

	financeChannelStream, err := fanOutFinances(ctx, financeStream)
	if err != nil {
		return future.Factory().
			SetError(future.InternalError, "fanOutFinances failed", errors.Wrap(err, "fanOutFinances failed")).
			BuildAndSend()
	}

	financeOutStream, err := fanInFinanceStreams(ctx, financeChannelStream)
	if err != nil {
		return future.Factory().
			SetError(future.InternalError, "fanInFinanceStreams failed", errors.Wrap(err, "fanInFinanceStreams failed")).
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

func findEligibleFinance(ctx context.Context) (financeReaderStream, error) {

	financeStream := make(chan *entities.SellerFinance, DefaultFinanceStreamSize)
	findEligibleFinanceTask := func() {
		defer func() {
			close(financeStream)
		}()
		timestamp := time.Now().UTC()
		var availablePages = 1
		for i := 0; i < availablePages; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			iFuture := app.Globals.SellerFinanceRepository.FindByFilterWithPage(ctx, func() interface{} {
				return bson.D{
					{"status", entities.FinanceOrderCollectionStatus},
					{"endAt", bson.D{{"$lte", timestamp}}},
					{"deletedAt", nil}}
			}, int64(i+1), DefaultFetchFinancePerPage).Get()

			if iFuture.Error() != nil {
				if iFuture.Error().Code() != future.NotFound {
					log.GLog.Logger.Error("SellerFinanceRepository.FindByFilterWithPage failed",
						"fn", "findEligibleFinance",
						"error", iFuture.Error())
					return
				}
				log.GLog.Logger.Info("seller finance not found",
					"fn", "findEligibleFinance")
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

	if err := app.Globals.WorkerPool.SubmitTask(findEligibleFinanceTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "findEligibleFinance",
			"error", err)

		return nil, err
	}

	return financeStream, nil
}

func fanOutFinances(ctx context.Context, financeStream financeReaderStream) (financeChannelStream, error) {
	financeWriterChannels := make([]financeWriterStream, 0, app.Globals.Config.Mongo.MinPoolSize/2)
	financeChannelStream := make(chan financeReaderStream)

	fanOutTask := func() {
		defer func() {
			log.GLog.Logger.Debug("complete",
				"fn", "fanOutFinances")
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
			"fn", "fanOutFinances",
			"error", err)

		return nil, err
	}

	return financeChannelStream, nil

}

func fanInFinanceStreams(ctx context.Context, financeChannelStream financeChannelStream) (financeReaderStream, error) {
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

			financeProcessStream, financeProcessFn := financeProcess()
			financeProcessTask := func() {
				financeProcessFn(ctx, financeChannel)
			}

			financePaymentStream, financePaymentFn := financePayment()
			financePaymentTask := func() {
				financePaymentFn(ctx, financeProcessStream)
			}

			if err := app.Globals.WorkerPool.SubmitTask(financeProcessTask); err != nil {
				log.GLog.Logger.Error("submit financeProcessTask to WorkerPool.SubmitTask failed",
					"fn", "fanInFinanceStreams", "error", err)
				continue
			}

			if err := app.Globals.WorkerPool.SubmitTask(financePaymentTask); err != nil {
				log.GLog.Logger.Error("submit financePaymentTask to WorkerPool.SubmitTask failed",
					"fn", "fanInFinanceStreams", "error", err)
				continue
			}

			fanInMultiplexTask := func() {
				defer wg.Done()
				for finance := range financePaymentStream {
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
					"fn", "fanInFinanceStreams", "error", err)
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

func financeProcess() (financeReaderStream, financePipelineFunc) {
	financeOutStream := make(chan *entities.SellerFinance)

	return financeOutStream, func(ctx context.Context, financeInStream financeReaderStream) {
		defer close(financeOutStream)

		for sellerFinance := range financeInStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := financeTriggerValidation(ctx, sellerFinance); err != nil {
				log.GLog.Logger.Error("finance not accepted for processing because related trigger has problem",
					"fn", "financeProcess",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"error", err)
				continue
			}

			if err := financeInvoiceCalculation(ctx, sellerFinance); err != nil {
				log.GLog.Logger.Error("financeInvoiceCalculation failed",
					"fn", "financeProcess",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"error", err)
				continue
			}

			if sellerFinance.SellerInfo == nil {
				iFuture := app.Globals.UserService.GetSellerProfile(ctx, strconv.Itoa(int(sellerFinance.SellerId))).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("UserService.GetSellerProfile failed",
						"fn", "financeProcess",
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error())

					sellerFinance.Status = entities.FinancePaymentProcessStatus
					iFuture := app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
					if iFuture.Error() != nil {
						log.GLog.Logger.Error("sellerFinance update failed",
							"fn", "financeProcess",
							"fid", sellerFinance.FId,
							"sellerId", sellerFinance.SellerId,
							"error", iFuture.Error())
					}
					continue
				} else {
					sellerFinance.SellerInfo = iFuture.Data().(*entities.SellerProfile)
				}
			}

			financeOutStream <- sellerFinance
		}
	}
}

func financeInvoiceCalculation(ctx context.Context, finance *entities.SellerFinance) error {
	financeCalc, err := calculation.FinanceCalcFactory(ctx, finance)
	if err != nil {
		log.GLog.Logger.Error("FinanceCalcFactory for converting finance to financeCalculation failed",
			"fn", "financeInvoiceCalculation",
			"fid", finance,
			"sellerId", finance.SellerId,
			"error", err)
		return err
	}

	if financeCalc.Invoice == nil {
		financeCalc.Invoice = &calculation.InvoiceCalc{
			SSORawTotal:            nil,
			SSORoundupTotal:        nil,
			VATRawTotal:            nil,
			VATRoundupTotal:        nil,
			CommissionRawTotal:     nil,
			CommissionRoundupTotal: nil,
			ShareRawTotal:          nil,
			ShareRoundupTotal:      nil,
			ShipmentRawTotal:       nil,
			ShipmentRoundupTotal:   nil,
		}
	}

	ssoRawTotal := decimal.Zero
	ssoRoundupTotal := decimal.Zero
	vatRawTotal := decimal.Zero
	vatRoundupTotal := decimal.Zero
	commissionRawTotal := decimal.Zero
	commissionRoundupTotal := decimal.Zero
	shareRawTotal := decimal.Zero
	shareRoundupTotal := decimal.Zero
	shipmentRawTotal := decimal.Zero
	shipmentRoundupTotal := decimal.Zero

	for _, orderInfoCalc := range financeCalc.OrdersInfo {
		for _, orderCalc := range orderInfoCalc.Orders {
			for _, itemCalc := range orderCalc.Items {
				shareRawTotal = shareRawTotal.Add(*itemCalc.Invoice.Share.RawTotalSellerShare)
				shareRoundupTotal = shareRoundupTotal.Add(*itemCalc.Invoice.Share.RoundupTotalSellerShare)

				commissionRawTotal = commissionRawTotal.Add(*itemCalc.Invoice.Commission.RawTotalPrice)
				commissionRoundupTotal = commissionRoundupTotal.Add(*itemCalc.Invoice.Commission.RoundupTotalPrice)

				if itemCalc.Invoice.SSO.IsObliged {
					ssoRawTotal = ssoRawTotal.Add(*itemCalc.Invoice.SSO.RawTotalPrice)
					ssoRoundupTotal = ssoRoundupTotal.Add(*itemCalc.Invoice.SSO.RoundupTotalPrice)
				}

				if itemCalc.Invoice.VAT.IsObliged {
					vatRawTotal = vatRawTotal.Add(*itemCalc.Invoice.VAT.RawTotalPrice)
					vatRoundupTotal = vatRoundupTotal.Add(*itemCalc.Invoice.VAT.RoundupTotalPrice)
				}
			}

			if !orderCalc.IsAlreadyShippingPay {
				shipmentRawTotal = shipmentRawTotal.Add(*orderCalc.RawShippingNet)
				shipmentRoundupTotal = shipmentRoundupTotal.Add(*orderCalc.RoundupShippingNet)
			}
		}
	}

	shareRawTotal = shareRawTotal.Add(shipmentRawTotal)
	shareRoundupTotal = shareRoundupTotal.Add(shipmentRoundupTotal)

	financeCalc.Invoice.SSORawTotal = &ssoRawTotal
	financeCalc.Invoice.SSORoundupTotal = &ssoRoundupTotal
	financeCalc.Invoice.VATRawTotal = &vatRawTotal
	financeCalc.Invoice.VATRoundupTotal = &vatRoundupTotal
	financeCalc.Invoice.CommissionRawTotal = &commissionRawTotal
	financeCalc.Invoice.CommissionRoundupTotal = &commissionRoundupTotal
	financeCalc.Invoice.ShareRawTotal = &shareRawTotal
	financeCalc.Invoice.ShareRoundupTotal = &shareRoundupTotal
	financeCalc.Invoice.ShipmentRawTotal = &shipmentRawTotal
	financeCalc.Invoice.ShipmentRoundupTotal = &shipmentRoundupTotal

	err = calculation.ConvertToSellerFinance(ctx, financeCalc, finance)
	if err != nil {
		log.GLog.Logger.Error("ConvertToSellerFinance for converting financeCalc to finance failed",
			"fn", "financeInvoiceCalculation",
			"fid", finance,
			"sellerId", finance.SellerId,
			"error", err)
		return err
	}

	return nil
}

func financeTriggerValidation(ctx context.Context, finance *entities.SellerFinance) error {
	for _, orderInfo := range finance.OrdersInfo {
		iFuture := app.Globals.TriggerHistoryRepository.FindById(ctx, orderInfo.TriggerHistoryId).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerHistoryRepository.FindById failed",
				"fn", "financeTriggerValidation",
				"fid", finance.FId,
				"sellerId", finance.SellerId,
				"triggerName", orderInfo.TriggerName,
				"historyId", orderInfo.TriggerHistoryId,
				"error", iFuture.Error())
			return iFuture.Error()
		}

		history := iFuture.Data().(*entities.TriggerHistory)
		if history.ExecResult != entities.TriggerExecResultSuccess {
			log.GLog.Logger.Info("trigger history result not successful",
				"fn", "financeTriggerValidation",
				"fid", finance.FId,
				"sellerId", finance.SellerId,
				"triggerName", orderInfo.TriggerName,
				"historyId", orderInfo.TriggerHistoryId,
				"result", history.ExecResult)
			return errors.New("trigger history result not successful")
		}
	}

	return nil
}

func financePayment() (financeReaderStream, financePipelineFunc) {
	financeOutStream := make(chan *entities.SellerFinance)

	return financeOutStream, func(ctx context.Context, financeInStream financeReaderStream) {
		defer close(financeOutStream)

		for sellerFinance := range financeInStream {
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

				sellerFinance.Status = entities.FinancePaymentProcessStatus
				iFuture := app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("sellerFinance update failed",
						"fn", "financePayment",
						"fid", sellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error())
				}
				continue
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
			}

			financeOutStream <- sellerFinance

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}
}
