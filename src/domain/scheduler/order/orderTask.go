package order_scheduler

import (
	"context"
	"github.com/pkg/errors"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	order_service "gitlab.faza.io/services/finance/infrastructure/services/order"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"gitlab.faza.io/services/finance/infrastructure/workerPool"
	"go.mongodb.org/mongo-driver/bson"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultOrderStreamBuffer   = 8192
	DefaultSellerSize          = 2048
	DefaultSellerOrders        = 4096
	DefaultFetchOrderPerPage   = 128
	DefaultFanOutStreamBuffers = 4096
)

type ErrorType int
type FuncType string

const (
	FetchOrderFn          FuncType = "FETCH_ORDER_FN"
	ReduceOrderFn         FuncType = "REDUCE_ORDER_FN"
	CreateUpdateFinanceFn FuncType = "CREATE_UPDATE_FINANCE_FN"
	FanInFinanceStreamsFn FuncType = "FAN_IN_FINANCE_STREAMS_FN"
)

const (
	NoError ErrorType = iota
	CtxError
	DataError
	DatabaseError
	OrderServiceError
	WorkerPoolError
)

type Pipeline struct {
	orderStream OrderReaderStream
	scheduler   *OrderScheduler
}

type ProcessResult struct {
	Function      FuncType
	SellerFinance *entities.SellerFinance
	ErrType       ErrorType
	Result        bool
}

type PipelineInStream <-chan *Pipeline
type PipelineOutStream chan<- *Pipeline
type PLIStream <-chan PipelineInStream

type OrderWriterStream chan<- *entities.SellerOrder
type OrderReaderStream <-chan *entities.SellerOrder

type ResultReaderStream <-chan *ProcessResult

func (scheduler OrderScheduler) OrderSchedulerTask(ctx context.Context, triggerHistory entities.TriggerHistory) future.IFuture {

	startAt := triggerHistory.TriggeredAt.Add(-scheduler.financeTriggerInterval)
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerHistory), triggerHistory)
	var iFuture = future.FactorySync().Build()

	task := func() {
		orderStream, fetchOrderResultStream, fetchOrderTask := scheduler.fetchOrders(ctx, startAt, *triggerHistory.TriggeredAt)
		pipelineStream, fanOutOrderTask := scheduler.fanOutOrders(ctx, orderStream)
		pipelineChannelStream, fanOutPipelineTask := scheduler.fanOutPipelines(ctx, pipelineStream)
		fanInFinanceResultStream, fanInPipelineTask := scheduler.fanInPipelineStreams(ctx, pipelineChannelStream)

		resultStream, err := scheduler.fanInResultStream(ctx, fetchOrderResultStream, fanInFinanceResultStream)
		if err != nil {
			cancel()
			future.FactoryOf(iFuture).
				SetError(future.InternalError, "OrderSchedulerTask failed", errors.Wrap(err, "fanInResultStream failed")).
				BuildAndSend()
			return
		}

		err = scheduler.taskLauncher(fanInPipelineTask, fanOutPipelineTask, fanOutOrderTask, fetchOrderTask)
		if err != nil {
			cancel()
			future.FactoryOf(iFuture).
				SetError(future.InternalError, "OrderSchedulerTask failed", errors.Wrap(err, "taskLauncher failed")).
				BuildAndSend()
			return
		}

		isSuccessFlag := true
		for processResult := range resultStream {
			if !processResult.Result {
				if processResult.ErrType != NoError && processResult.ErrType != DataError {
					cancel()
					isSuccessFlag = false
					break
				}
			}
		}

		if !isSuccessFlag {
			triggerHistory.ExecResult = entities.TriggerExecResultFail
		} else {
			triggerHistory.ExecResult = entities.TriggerExecResultSuccess
		}

		triggerHistory.RunMode = entities.TriggerRunModeComplete

		iTriggerFuture := app.Globals.TriggerHistoryRepository.Update(ctx, triggerHistory).Get()
		if iTriggerFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerHistoryRepository.Update failed",
				"fn", "OrderSchedulerTask",
				"trigger", triggerHistory,
				"error", iTriggerFuture.Error().Reason())
			future.FactoryOf(iFuture).
				SetError(iTriggerFuture.Error().Code(), "OrderSchedulerTask failed", iTriggerFuture.Error().Reason()).
				BuildAndSend()
			return
		}

		future.FactoryOf(iFuture).
			SetData(struct{}{}).
			BuildAndSend()
	}

	if err := app.Globals.WorkerPool.SubmitTask(task); err != nil {
		log.GLog.Logger.Error("submit orderSchedulerTask to WorkerPool.SubmitTask failed",
			"fn", "OrderSchedulerTask")
		return future.FactorySync().
			SetError(future.InternalError, "SubmitTask of WorkerPool failed", errors.Wrap(err, "SubmitTask of WorkerPool failed")).
			BuildAndSend()
	}
	return iFuture
}

func (scheduler OrderScheduler) fanInResultStream(ctx context.Context, channels ...ResultReaderStream) (ResultReaderStream, error) {
	var wg sync.WaitGroup
	multiplexedStream := make(chan *ProcessResult)
	multiplex := func(resultStream ResultReaderStream) {
		defer wg.Done()
		for processResult := range resultStream {
			select {
			case <-ctx.Done():
				return
			case multiplexedStream <- processResult:
			}
		}
	}
	// Select from all the channels
	wg.Add(len(channels))
	for _, channel := range channels {
		go multiplex(channel)
	}
	// Wait for all the reads to complete
	if err := app.Globals.WorkerPool.SubmitTask(func() { wg.Wait(); close(multiplexedStream) }); err != nil {
		return nil, err
	}

	return multiplexedStream, nil
}

func (scheduler OrderScheduler) taskLauncher(tasks ...worker_pool.Task) error {
	for _, task := range tasks {
		if err := app.Globals.WorkerPool.SubmitTask(task); err != nil {
			return err
		}
	}
	return nil
}

func (scheduler OrderScheduler) fetchOrders(ctx context.Context, startAt, endAt time.Time) (OrderReaderStream, ResultReaderStream, worker_pool.Task) {
	orderStream := make(chan *entities.SellerOrder, DefaultOrderStreamBuffer)
	resultStream := make(chan *ProcessResult)
	fetchOrderTask := func() {
		defer func() {
			close(orderStream)
			close(resultStream)
		}()

		// total 160 page=6 perPage=30
		var availablePages = 1
		for i := 0; i < availablePages; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			iFuture := app.Globals.OrderService.GetFinanceOrderItems(ctx, order_service.PayToSellerFilter,
				startAt, endAt, uint32(i+1), DefaultFetchOrderPerPage).Get()
			if iFuture.Error() != nil {
				if iFuture.Error().Code() != future.NotFound {
					log.GLog.Logger.Error("GetFinanceOrderItems from order service failed",
						"fn", "fetchOrders", "error", iFuture.Error())

					resultStream <- &ProcessResult{
						Function:      FetchOrderFn,
						SellerFinance: nil,
						ErrType:       OrderServiceError,
						Result:        false,
					}

					return
				}

				resultStream <- &ProcessResult{
					Function:      FetchOrderFn,
					SellerFinance: nil,
					ErrType:       NoError,
					Result:        true,
				}
				return
			}

			orderServiceResult := iFuture.Data().(*order_service.OrderServiceResult)

			if orderServiceResult.TotalCount%DefaultFetchOrderPerPage != 0 {
				availablePages = (int(orderServiceResult.TotalCount) / DefaultFetchOrderPerPage) + 1
			} else {
				availablePages = int(orderServiceResult.TotalCount) / DefaultFetchOrderPerPage
			}

			if orderServiceResult.TotalCount < DefaultFetchOrderPerPage {
				availablePages = 1
			}

			for _, order := range orderServiceResult.SellerOrders {
				select {
				case <-ctx.Done():
					return
				case orderStream <- order:
				}
			}
		}
	}
	return orderStream, resultStream, fetchOrderTask
}

func (scheduler OrderScheduler) fanOutOrders(ctx context.Context, orderChannel OrderReaderStream) (PipelineInStream, worker_pool.Task) {

	sellerStreamMap := make(map[uint64]OrderWriterStream, DefaultSellerSize)
	pipelineStream := make(chan *Pipeline)

	fanOutTask := func() {

		defer func() {
			for _, stream := range sellerStreamMap {
				close(stream)
			}

			close(pipelineStream)
		}()

		for sellerOrder := range orderChannel {
			select {
			case <-ctx.Done():
				break
			default:
			}

			if writerStream, ok := sellerStreamMap[sellerOrder.SellerId]; !ok {
				orderStream := make(chan *entities.SellerOrder, DefaultFanOutStreamBuffers)
				sellerStreamMap[sellerOrder.SellerId] = orderStream
				orderStream <- sellerOrder
				pipelineStream <- &Pipeline{
					orderStream: orderStream,
					scheduler:   &scheduler,
				}
			} else {
				writerStream <- sellerOrder
			}
		}
	}

	return pipelineStream, fanOutTask
}

func (scheduler OrderScheduler) fanOutPipelines(ctx context.Context, pipelineStream PipelineInStream) (PLIStream, worker_pool.Task) {
	pipelineChannels := make([]chan *Pipeline, 0, app.Globals.Config.Mongo.MinPoolSize/2)
	pipelineChannelStream := make(chan PipelineInStream)

	financeFanOutTask := func() {
		defer func() {
			for _, stream := range pipelineChannels {
				close(stream)
			}

			close(pipelineChannelStream)
		}()

		index := 0
		initIndex := 0
		for pipeline := range pipelineStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if initIndex < cap(pipelineChannels) {
				pipelineStream := make(chan *Pipeline, DefaultFanOutStreamBuffers)
				pipelineChannels = append(pipelineChannels, pipelineStream)
				pipelineChannelStream <- pipelineStream
				initIndex++
			}

			if index >= len(pipelineChannels) {
				index = 0
			}

			pipelineChannels[index] <- pipeline
			index++
		}
	}

	return pipelineChannelStream, financeFanOutTask
}

func (scheduler OrderScheduler) fanInPipelineStreams(ctx context.Context, pipelineChannelStream PLIStream) (ResultReaderStream, worker_pool.Task) {
	multiplexedResultStream := make(chan *ProcessResult)
	var wg sync.WaitGroup
	fanInCoreTask := func() {
		defer close(multiplexedResultStream)
		for pipelineChannel := range pipelineChannelStream {
			select {
			case <-ctx.Done():
				break
			default:
			}

			resultReaderStream, pipelineTaskFn := scheduler.executePipeline(ctx, pipelineChannel)
			if err := app.Globals.WorkerPool.SubmitTask(pipelineTaskFn); err != nil {
				log.GLog.Logger.Error("submit pipelineTaskFn to WorkerPool.SubmitTask failed",
					"fn", "fanInPipelineStreams", "error", err)

				multiplexedResultStream <- &ProcessResult{
					Function:      FanInFinanceStreamsFn,
					SellerFinance: nil,
					ErrType:       WorkerPoolError,
					Result:        false,
				}
				break
			}

			fanInMultiplexTask := func() {
				defer wg.Done()
				for processResult := range resultReaderStream {
					multiplexedResultStream <- processResult
				}
			}

			wg.Add(1)
			if err := app.Globals.WorkerPool.SubmitTask(fanInMultiplexTask); err != nil {
				log.GLog.Logger.Error("submit fanInMultiplexTask to WorkerPool.SubmitTask failed",
					"fn", "fanInPipelineStreams", "error", err)

				multiplexedResultStream <- &ProcessResult{
					Function:      FanInFinanceStreamsFn,
					SellerFinance: nil,
					ErrType:       WorkerPoolError,
					Result:        false,
				}
				wg.Done()
				break
			}
		}

		wg.Wait()
	}
	return multiplexedResultStream, fanInCoreTask
}

func (scheduler OrderScheduler) executePipeline(ctx context.Context, pipelineStream PipelineInStream) (ResultReaderStream, worker_pool.Task) {

	resultStream := make(chan *ProcessResult)
	pipelineTask := func() {
		defer close(resultStream)
		for pipeline := range pipelineStream {
			select {
			case <-ctx.Done():
				break
			default:
			}

			sellerOrder, processResult := pipeline.reduceOrders(ctx, pipeline.orderStream)
			if processResult != nil {
				resultStream <- processResult
				continue
			}

			processResult = pipeline.createUpdateFinance(ctx, sellerOrder)
			resultStream <- processResult
		}
	}

	return resultStream, pipelineTask
}

func (pipeline *Pipeline) reduceOrders(ctx context.Context, orderStream OrderReaderStream) (*entities.SellerFinance, *ProcessResult) {
	triggerHistory := ctx.Value(string(utils.CtxTriggerHistory)).(entities.TriggerHistory)
	sellerFinance := &entities.SellerFinance{
		OrdersInfo: []*entities.OrderInfo{
			{
				TriggerName:      triggerHistory.TriggerName,
				TriggerHistoryId: triggerHistory.ID,
				Orders:           nil,
			},
		},
	}
	sellerFinance.OrdersInfo[0].Orders = make([]*entities.SellerOrder, 0, DefaultSellerOrders)

	for sellerOrder := range orderStream {
		select {
		case <-ctx.Done():
			return nil, &ProcessResult{
				Function:      ReduceOrderFn,
				SellerFinance: nil,
				ErrType:       CtxError,
				Result:        false,
			}
		default:
		}

		if sellerFinance.SellerId == 0 {
			sellerFinance.SellerId = sellerOrder.SellerId

		} else if sellerFinance.SellerId != sellerOrder.SellerId {
			log.GLog.Logger.Error("sellerId of sellerFinance mismatch with sellerOrder",
				"fn", "reduceOrders",
				"sellerId", sellerFinance.SellerId,
				"sellerOrder", sellerOrder)
			return nil, &ProcessResult{
				Function:      ReduceOrderFn,
				SellerFinance: nil,
				ErrType:       DataError,
				Result:        false,
			}
		}

		iFuture := app.Globals.SellerFinanceRepository.CountWithFilter(ctx, func() interface{} {
			return bson.D{
				{"sellerId", sellerFinance.SellerId},
				{"ordersInfo.orders.oid", sellerOrder.OId},
				{"deletedAt", nil}}
		}).Get()

		if iFuture.Error() != nil {
			log.GLog.Logger.Error("SellerFinanceRepository.CountWithFilter for find order history failed",
				"fn", "reduceOrders",
				"oid", sellerOrder.OId,
				"sellerId", sellerOrder.SellerId,
				"error", iFuture.Error())
			return nil, &ProcessResult{
				Function:      ReduceOrderFn,
				SellerFinance: nil,
				ErrType:       DatabaseError,
				Result:        false,
			}
		}

		if iFuture.Data().(int64) > 0 {
			sellerOrder.IsAlreadyShippingPayed = true
		} else {
			sellerOrder.IsAlreadyShippingPayed = false
		}

		if app.Globals.Config.App.SellerFinancePreventDuplicateOrderItem {
			validItems := make([]*entities.SellerItem, 0, len(sellerOrder.Items))

			for _, item := range sellerOrder.Items {
				iFuture := app.Globals.SellerOrderRepository.CountWithFilter(ctx, func() interface{} {
					return []bson.M{
						{"$match": bson.M{"ordersInfo.orders.sellerId": sellerOrder.SellerId, "ordersInfo.orders.oid": sellerOrder.OId, "ordersInfo.orders.deletedAt": nil}},
						{"$unwind": "$ordersInfo"},
						{"$unwind": "$ordersInfo.orders"},
						{"$unwind": "$ordersInfo.orders.items"},
						{"$match": bson.M{"ordersInfo.orders.oid": sellerOrder.OId, "ordersInfo.orders.items.sid": item.SId, "ordersInfo.orders.deletedAt": nil}},
						{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": 1}}},
						{"$project": bson.M{"_id": 0, "count": 1}},
					}
				}).Get()

				if iFuture.Error() != nil {
					log.GLog.Logger.Error("SellerOrderRepository.CountWithFilter for duplicate order subpackage failed",
						"fn", "reduceOrders",
						"oid", sellerOrder.OId,
						"sellerId", sellerOrder.SellerId,
						"sid", item.SId,
						"error", iFuture.Error())
					return nil, &ProcessResult{
						Function:      ReduceOrderFn,
						SellerFinance: nil,
						ErrType:       DatabaseError,
						Result:        false,
					}
				}

				if iFuture.Data().(int64) > 0 {
					log.GLog.Logger.Warn("duplicate order subpackage detected",
						"fn", "reduceOrders",
						"oid", sellerOrder.OId,
						"sellerId", sellerOrder.SellerId,
						"sid", item.SId,
						"inventoryId", item.InventoryId)
					continue
				}

				validItems = append(validItems, item)
			}

			if len(validItems) > 0 {
				sellerOrder.Items = validItems
			} else {
				log.GLog.Logger.Warn("duplicate order detected",
					"fn", "reduceOrders",
					"oid", sellerOrder.OId,
					"sellerId", sellerOrder.SellerId)
				continue
			}
		}

		sellerFinance.OrdersInfo[0].Orders = append(sellerFinance.OrdersInfo[0].Orders, sellerOrder)
	}

	if len(sellerFinance.OrdersInfo[0].Orders) > 0 {
		return sellerFinance, nil
	}
	log.GLog.Logger.Warn("seller finance dropped because of valid order not found",
		"fn", "reduceOrders",
		"sellerId", sellerFinance.SellerId)

	return nil, &ProcessResult{
		Function:      ReduceOrderFn,
		SellerFinance: nil,
		ErrType:       DataError,
		Result:        false,
	}
}

func (pipeline *Pipeline) createUpdateFinance(ctx context.Context, sellerFinance *entities.SellerFinance) *ProcessResult {

	timestamp := time.Now().UTC()
	iFuture := app.Globals.SellerFinanceRepository.FindByFilter(ctx, func() interface{} {
		return bson.D{{"sellerId", sellerFinance.SellerId},
			{"status", entities.FinanceOrderCollectionStatus},
			{"deletedAt", nil},
			{"endAt", bson.D{{"$gte", timestamp}}},
		}
	}).Get()

	triggerHistory := ctx.Value(string(utils.CtxTriggerHistory)).(entities.TriggerHistory)

	isNotFoundFlag := false
	if iFuture.Error() != nil {
		if iFuture.Error().Code() != future.NotFound {
			log.GLog.Logger.Error("SellerFinanceRepository.FindByFilter failed",
				"fn", "createUpdateFinance",
				"sellerId", sellerFinance.SellerId,
				"error", iFuture.Error().Reason())

			return &ProcessResult{
				Function:      CreateUpdateFinanceFn,
				SellerFinance: sellerFinance,
				ErrType:       DatabaseError,
				Result:        false,
			}

		} else {
			isNotFoundFlag = true
		}
	}

	if !isNotFoundFlag {
		sellerFinances := iFuture.Data().([]*entities.SellerFinance)

		if len(sellerFinances) > 1 {
			log.GLog.Logger.Debug("many sellers found in collect order state",
				"fn", "createUpdateFinance",
				"sellerId", sellerFinance.SellerId,
				"state", entities.FinanceOrderCollectionStatus,
				"founds", len(sellerFinances))
		}

		var foundSellerFinance *entities.SellerFinance
		for _, finance := range sellerFinances {
			if timestamp.Before(finance.EndAt.UTC()) || timestamp.Equal(finance.EndAt.UTC()) {
				if foundSellerFinance == nil {
					foundSellerFinance = finance
				} else {
					log.GLog.Logger.Warn("greater than one seller found in collect order state",
						"fn", "createUpdateFinance",
						"fid", finance.FId,
						"sellerId", sellerFinance.SellerId,
						"state", entities.FinanceOrderCollectionStatus)
				}
			}
		}

		if foundSellerFinance != nil {
			log.GLog.Logger.Info("found valid sellerFinance in collect order state",
				"fn", "createUpdateFinance",
				"fid", foundSellerFinance.FId,
				"sellerId", sellerFinance.SellerId,
				"state", entities.FinanceOrderCollectionStatus)

			newOrderInfo := &entities.OrderInfo{
				TriggerName:      triggerHistory.TriggerName,
				TriggerHistoryId: triggerHistory.ID,
				Orders:           nil,
			}

			newOrderInfo.Orders = make([]*entities.SellerOrder, 0, len(sellerFinance.OrdersInfo[0].Orders))

			var sids = make([]uint64, 0, len(sellerFinance.OrdersInfo[0].Orders)*2)
			for _, newOrder := range sellerFinance.OrdersInfo[0].Orders {
				collectOrders := make([]*entities.SellerOrder, 0, 1)
				for _, orderInfo := range foundSellerFinance.OrdersInfo {
					for _, order := range orderInfo.Orders {
						if order.OId == newOrder.OId {
							collectOrders = append(collectOrders, order)
						}
					}
				}

				if len(collectOrders) > 0 {
					for _, newItem := range newOrder.Items {
						diffItems := make([]*entities.SellerItem, 0, len(newOrder.Items))
						itemFindFlag := false
						for _, order := range collectOrders {
							for _, item := range order.Items {
								if newItem.SId == item.SId {
									itemFindFlag = true
									break
								}
							}

							if itemFindFlag {
								break
							}
						}

						if !itemFindFlag {
							diffItems = append(diffItems, newItem)
							sids = append(sids, newItem.SId)
						} else {
							log.GLog.Logger.Warn("duplicate order subpackage found",
								"fn", "createUpdateFinance",
								"sellerId", foundSellerFinance.SellerId,
								"oid", newOrder.OId,
								"sid", newItem.SId,
								"inventoryId", newItem.InventoryId)
						}

						if len(diffItems) > 0 {
							newOrder.Items = diffItems
							newOrder.FId = foundSellerFinance.FId
							newOrderInfo.Orders = append(newOrderInfo.Orders, newOrder)
						} else {
							log.GLog.Logger.Warn("duplicate order found",
								"fn", "createUpdateFinance",
								"sellerId", sellerFinance.SellerId,
								"oid", newOrder.OId)
						}
					}
				} else {
					newOrder.FId = foundSellerFinance.FId
					newOrderInfo.Orders = append(newOrderInfo.Orders, newOrder)
				}
			}

			if len(newOrderInfo.Orders) > 0 {
				foundSellerFinance.OrdersInfo = append(foundSellerFinance.OrdersInfo, newOrderInfo)
			} else {
				log.GLog.Logger.Warn("valid orders not found for add to orderInfo of sellerFinance",
					"fn", "createUpdateFinance",
					"sellerId", sellerFinance.SellerId,
					"trigger", triggerHistory.TriggerName,
					"triggerHistoryId", triggerHistory.ID)
				return &ProcessResult{
					Function:      CreateUpdateFinanceFn,
					SellerFinance: nil,
					ErrType:       DataError,
					Result:        false,
				}
			}

			iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *foundSellerFinance).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("SellerFinanceRepository.Update failed",
					"fn", "createUpdateFinance",
					"fid", foundSellerFinance.FId,
					"sellerId", foundSellerFinance.SellerId,
					"error", iFuture.Error())

				return &ProcessResult{
					Function:      CreateUpdateFinanceFn,
					SellerFinance: foundSellerFinance,
					ErrType:       DatabaseError,
					Result:        false,
				}
			}

			log.GLog.Logger.Info("Updating seller finance success",
				"fn", "createUpdateFinance",
				"fid", foundSellerFinance.FId,
				"sellerId", foundSellerFinance.SellerId,
				"sids", sids)

			return &ProcessResult{
				Function:      CreateUpdateFinanceFn,
				SellerFinance: foundSellerFinance,
				ErrType:       NoError,
				Result:        true,
			}

			// create new seller finance
		} else {
			startAt := triggerHistory.TriggeredAt.Add(-pipeline.scheduler.financeTriggerInterval)
			endAt := startAt.Add(pipeline.scheduler.financeTriggerDuration)

			newSellerFinance := &entities.SellerFinance{
				SellerId:   sellerFinance.SellerId,
				SellerInfo: nil,
				Invoice:    nil,
				OrdersInfo: []*entities.OrderInfo{
					{
						TriggerName:      triggerHistory.TriggerName,
						TriggerHistoryId: triggerHistory.ID,
						Orders:           sellerFinance.OrdersInfo[0].Orders,
					},
				},
				Payment:   nil,
				Status:    entities.FinanceOrderCollectionStatus,
				StartAt:   &startAt,
				EndAt:     &endAt,
				CreatedAt: timestamp,
				UpdatedAt: timestamp,
				DeletedAt: nil,
			}

			sids := make([]uint64, 0, len(sellerFinance.OrdersInfo[0].Orders)*2)
			for _, order := range sellerFinance.OrdersInfo[0].Orders {
				for _, item := range order.Items {
					sids = append(sids, item.SId)
				}
			}

			iFuture := app.Globals.UserService.GetSellerProfile(ctx, strconv.Itoa(int(sellerFinance.SellerId))).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("UserService.GetSellerProfile failed",
					"fn", "createUpdateFinance",
					"sellerId", sellerFinance.SellerId,
					"error", iFuture.Error())
			} else {
				newSellerFinance.SellerInfo = iFuture.Data().(*entities.SellerProfile)
			}

			iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *newSellerFinance).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("SellerFinanceRepository.Save failed",
					"fn", "createUpdateFinance",
					"sellerId", newSellerFinance.SellerId,
					"startAt", startAt.Format(utils.ISO8601),
					"endAt", endAt.Format(utils.ISO8601),
					"error", iFuture.Error())

				return &ProcessResult{
					Function:      CreateUpdateFinanceFn,
					SellerFinance: newSellerFinance,
					ErrType:       DatabaseError,
					Result:        false,
				}
			}

			newSellerFinance = iFuture.Data().(*entities.SellerFinance)
			log.GLog.Logger.Info("create seller finance success",
				"fn", "createUpdateFinance",
				"fid", newSellerFinance.FId,
				"sellerId", newSellerFinance.SellerId,
				"startAt", startAt.Format(utils.ISO8601),
				"endAt", endAt.Format(utils.ISO8601),
				"sids", sids)

			return &ProcessResult{
				Function:      CreateUpdateFinanceFn,
				SellerFinance: newSellerFinance,
				ErrType:       NoError,
				Result:        true,
			}
		}
		// create new seller finance
	} else {
		startAt := triggerHistory.TriggeredAt.Add(-pipeline.scheduler.financeTriggerInterval)
		endAt := startAt.Add(pipeline.scheduler.financeTriggerDuration)

		newSellerFinance := &entities.SellerFinance{
			SellerId:   sellerFinance.SellerId,
			SellerInfo: nil,
			Invoice:    nil,
			OrdersInfo: []*entities.OrderInfo{
				{
					TriggerName:      triggerHistory.TriggerName,
					TriggerHistoryId: triggerHistory.ID,
					Orders:           sellerFinance.OrdersInfo[0].Orders,
				},
			},
			Payment:   nil,
			Status:    entities.FinanceOrderCollectionStatus,
			StartAt:   &startAt,
			EndAt:     &endAt,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
			DeletedAt: nil,
		}

		iFuture := app.Globals.UserService.GetSellerProfile(ctx, strconv.Itoa(int(sellerFinance.SellerId))).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("UserService.GetSellerProfile failed",
				"fn", "createUpdateFinance",
				"sellerId", sellerFinance.SellerId,
				"error", iFuture.Error())
		} else {
			newSellerFinance.SellerInfo = iFuture.Data().(*entities.SellerProfile)
		}

		iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *newSellerFinance).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("SellerFinanceRepository.Save failed",
				"fn", "createUpdateFinance",
				"sellerId", newSellerFinance.SellerId,
				"startAt", startAt.Format(utils.ISO8601),
				"endAt", endAt.Format(utils.ISO8601),
				"error", iFuture.Error())

			return &ProcessResult{
				Function:      CreateUpdateFinanceFn,
				SellerFinance: newSellerFinance,
				ErrType:       DatabaseError,
				Result:        false,
			}
		}

		sids := make([]uint64, 0, len(sellerFinance.OrdersInfo[0].Orders)*2)
		for _, order := range sellerFinance.OrdersInfo[0].Orders {
			for _, item := range order.Items {
				sids = append(sids, item.SId)
			}
		}

		newSellerFinance = iFuture.Data().(*entities.SellerFinance)
		log.GLog.Logger.Info("create seller finance success",
			"fn", "createUpdateFinance",
			"fid", newSellerFinance.FId,
			"sellerId", newSellerFinance.SellerId,
			"startAt", startAt.Format(utils.ISO8601),
			"endAt", endAt.Format(utils.ISO8601),
			"sids", sids)

		return &ProcessResult{
			Function:      CreateUpdateFinanceFn,
			SellerFinance: newSellerFinance,
			ErrType:       NoError,
			Result:        true,
		}
	}
}
