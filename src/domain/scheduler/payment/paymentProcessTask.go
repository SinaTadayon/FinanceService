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
	worker_pool "gitlab.faza.io/services/finance/infrastructure/workerPool"
	"go.mongodb.org/mongo-driver/bson"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultFinanceStreamSize   = 1024
	DefaultFanOutStreamBuffer  = 1024
	DefaultFetchFinancePerPage = 64
)

type Pipeline struct {
	dataStream FinanceReaderStream
}

type PipelineInStream <-chan *Pipeline
type PipelineOutStream chan<- *Pipeline
type PLIStream <-chan PipelineInStream

type FinanceWriterStream chan<- *entities.SellerFinance
type FinanceReaderStream <-chan *entities.SellerFinance
type FinanceChannelStream <-chan FinanceReaderStream

func PaymentProcessTask(ctx context.Context) future.IFuture {

	financeStream, err := findEligibleFinance(ctx)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "findEligibleFinance failed", errors.Wrap(err, "findEligibleFinance failed")).
			BuildAndSend()
	}

	financeChannelStream, err := fanOutPipelines(ctx, financeStream)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "fanOutPipelines failed", errors.Wrap(err, "fanOutPipelines failed")).
			BuildAndSend()
	}

	financeOutStream, err := fanInPipelines(ctx, financeChannelStream)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "fanInPipelines failed", errors.Wrap(err, "fanInPipelines failed")).
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

func findEligibleFinance(ctx context.Context) (FinanceReaderStream, error) {

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
					{"endAt", bson.D{{"$lt", timestamp}}},
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

	if err := app.Globals.WorkerPool.SubmitTask(findEligibleFinanceTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "findEligibleFinance",
			"error", err)

		return nil, err
	}

	return financeStream, nil
}

func fanOutPipelines(ctx context.Context, financeInStream FinanceReaderStream) (PipelineInStream, error) {
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
		for sellerFinance := range financeInStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if initIndex < cap(financeWriterChannels) {
				financeChannel := make(chan *entities.SellerFinance, DefaultFanOutStreamBuffer)
				financeWriterChannels = append(financeWriterChannels, financeChannel)
				pipelineStream <- &Pipeline{dataStream: financeChannel}
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
			"fn", "fanOutPipelines",
			"error", err)

		return nil, err
	}

	return pipelineStream, nil

}

func fanInPipelines(ctx context.Context, pipelineStream PipelineInStream) (FinanceReaderStream, error) {
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

			financeStream, pipelineTask := pipeline.ExecutePipeline(ctx)

			if err := app.Globals.WorkerPool.SubmitTask(pipelineTask); err != nil {
				log.GLog.Logger.Error("submit pipelineTask to WorkerPool.SubmitTask failed",
					"fn", "fanInPipelines", "error", err)
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
					"fn", "fanInPipelines", "error", err)
				wg.Done()
			}
		}

		wg.Wait()
	}

	if err := app.Globals.WorkerPool.SubmitTask(fanInTask); err != nil {
		log.GLog.Logger.Error("WorkerPool.SubmitTask failed",
			"fn", "fanInPipelines",
			"error", err)

		return nil, err
	}

	return multiplexedFinanceStream, nil
}

func (pipeline *Pipeline) ExecutePipeline(ctx context.Context) (FinanceReaderStream, worker_pool.Task) {
	financeOutStream := make(chan *entities.SellerFinance)

	return financeOutStream, func() {
		defer close(financeOutStream)

		for sellerFinance := range pipeline.dataStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := pipeline.financeTriggerValidation(ctx, sellerFinance); err != nil {
				log.GLog.Logger.Error("finance not accepted for processing because related trigger has problem",
					"fn", "executePipeline",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"error", err)
				continue
			}

			if err := pipeline.financeInvoiceCalculation(ctx, sellerFinance); err != nil {
				log.GLog.Logger.Error("financeInvoiceCalculation failed",
					"fn", "executePipeline",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"error", err)
				continue
			} else {
				log.GLog.Logger.Info("financeInvoiceCalculation success",
					"fn", "executePipeline",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"invoice", sellerFinance.Invoice)
			}

			if sellerFinance.SellerInfo == nil {
				iFuture := app.Globals.UserService.GetSellerProfile(ctx, strconv.Itoa(int(sellerFinance.SellerId))).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("UserService.GetSellerProfile failed",
						"fn", "executePipeline",
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error())

					sellerFinance.Status = entities.FinancePaymentProcessStatus
					iFuture := app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
					if iFuture.Error() != nil {
						log.GLog.Logger.Error("sellerFinance update failed",
							"fn", "executePipeline",
							"fid", sellerFinance.FId,
							"sellerId", sellerFinance.SellerId,
							"error", iFuture.Error())
					}
					continue
				} else {
					sellerFinance.SellerInfo = iFuture.Data().(*entities.SellerProfile)
				}
			}

			updatedSellerFinance, err := pipeline.financePayment(ctx, sellerFinance)
			if err != nil {
				log.GLog.Logger.Error("financePayment failed",
					"fn", "executePipeline",
					"fid", sellerFinance.FId,
					"sellerId", sellerFinance.SellerId,
					"error", err)
				continue
			}

			financeOutStream <- updatedSellerFinance
		}
	}
}

func (pipeline *Pipeline) financeInvoiceCalculation(ctx context.Context, finance *entities.SellerFinance) error {
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

				if itemCalc.Invoice.Commission.ItemCommission > 0 {
					if itemCalc.Invoice.Commission.RawTotalPrice != nil {
						commissionRawTotal = commissionRawTotal.Add(*itemCalc.Invoice.Commission.RawTotalPrice)
					}

					if itemCalc.Invoice.Commission.RoundupTotalPrice != nil {
						commissionRoundupTotal = commissionRoundupTotal.Add(*itemCalc.Invoice.Commission.RoundupTotalPrice)
					}
				}

				if itemCalc.Invoice.SSO.IsObliged {
					if itemCalc.Invoice.SSO.RawTotalPrice != nil {
						ssoRawTotal = ssoRawTotal.Add(*itemCalc.Invoice.SSO.RawTotalPrice)
					}

					if itemCalc.Invoice.SSO.RoundupTotalPrice != nil {
						ssoRoundupTotal = ssoRoundupTotal.Add(*itemCalc.Invoice.SSO.RoundupTotalPrice)
					}
				}

				if itemCalc.Invoice.VAT.IsObliged {
					if itemCalc.Invoice.VAT.RawTotalPrice != nil {
						vatRawTotal = vatRawTotal.Add(*itemCalc.Invoice.VAT.RawTotalPrice)
					}

					if itemCalc.Invoice.VAT.RoundupTotalPrice != nil {
						vatRoundupTotal = vatRoundupTotal.Add(*itemCalc.Invoice.VAT.RoundupTotalPrice)
					}
				}
			}

			if !orderCalc.IsAlreadyShippingPayed {
				if orderCalc.RawShippingNet != nil {
					shipmentRawTotal = shipmentRawTotal.Add(*orderCalc.RawShippingNet)
				}

				if orderCalc.RoundupShippingNet != nil {
					shipmentRoundupTotal = shipmentRoundupTotal.Add(*orderCalc.RoundupShippingNet)
				}
			}
		}
	}

	shareRawTotal = shareRawTotal.Add(shipmentRawTotal)
	shareRoundupTotal = shareRoundupTotal.Add(shipmentRoundupTotal)

	if !ssoRawTotal.IsZero() {
		financeCalc.Invoice.SSORawTotal = &ssoRawTotal
	}

	if !ssoRoundupTotal.IsZero() {
		financeCalc.Invoice.SSORoundupTotal = &ssoRoundupTotal
	}

	if !vatRawTotal.IsZero() {
		financeCalc.Invoice.VATRawTotal = &vatRawTotal
	}

	if !vatRoundupTotal.IsZero() {
		financeCalc.Invoice.VATRoundupTotal = &vatRoundupTotal
	}

	if !commissionRawTotal.IsZero() {
		financeCalc.Invoice.CommissionRawTotal = &commissionRawTotal
	}

	if !commissionRoundupTotal.IsZero() {
		financeCalc.Invoice.CommissionRoundupTotal = &commissionRoundupTotal
	}

	if !shareRawTotal.IsZero() {
		financeCalc.Invoice.ShareRawTotal = &shareRawTotal
	}

	if !shareRoundupTotal.IsZero() {
		financeCalc.Invoice.ShareRoundupTotal = &shareRoundupTotal
	}

	if !shipmentRawTotal.IsZero() {
		financeCalc.Invoice.ShipmentRawTotal = &shipmentRawTotal
	}

	if !shipmentRoundupTotal.IsZero() {
		financeCalc.Invoice.ShipmentRoundupTotal = &shipmentRoundupTotal
	}

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

func (pipeline *Pipeline) financeTriggerValidation(ctx context.Context, finance *entities.SellerFinance) error {
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

func (pipeline *Pipeline) financePayment(ctx context.Context, sellerFinance *entities.SellerFinance) (*entities.SellerFinance, error) {

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
			"fn", "financePayment",
			"fid", sellerFinance.FId,
			"sellerId", sellerFinance.SellerId,
			"error", iFuture.Error())

		sellerFinance.Status = entities.FinancePaymentProcessStatus
		iUpdateFuture := app.Globals.SellerFinanceRepository.Save(ctx, *sellerFinance).Get()
		if iUpdateFuture.Error() != nil {
			log.GLog.Logger.Error("sellerFinance update failed",
				"fn", "financePayment",
				"fid", sellerFinance.FId,
				"sellerId", sellerFinance.SellerId,
				"error", iUpdateFuture.Error())
		}
		return nil, iFuture.Error()
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
		Status:         entities.PaymentPendingState,
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
		return nil, iFuture.Error()
	}

	return iFuture.Data().(*entities.SellerFinance), nil
}
