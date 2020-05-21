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
	DefaultFanOutStreamBuffers = 256
)

type ErrorType int
type FuncType string

const (
	FetchOrderFn          FuncType = "FETCH_ORDER_FN"
	FanInOrderStreamsFn   FuncType = "FAN_IN_ORDER_STREAMS_FN"
	CreateUpdateFinanceFn FuncType = "CREATE_UPDATE_FINANCE_FN"
	FanInFinanceStreamsFn FuncType = "FAN_IN_FINANCE_STREAMS_FN"
)

const (
	NoError ErrorType = iota
	DatabaseError
	OrderServiceError
	WorkerPoolError
)

type ProcessResult struct {
	Function      FuncType
	SellerFinance *entities.SellerFinance
	ErrType       ErrorType
	Result        bool
}

type OrderWriterStream chan<- *entities.SellerOrder
type OrderReaderStream <-chan *entities.SellerOrder
type ORSStream <-chan OrderReaderStream

type FinanceWriterStream chan<- *entities.SellerFinance
type FinanceReaderStream <-chan *entities.SellerFinance
type FRSStream <-chan FinanceReaderStream

type ResultWriteStream chan<- *ProcessResult
type ResultReaderStream <-chan *ProcessResult

type OrderReduceFunc func(ctx context.Context, orderChannel OrderReaderStream)
type CreateUpdateFinanceFunc func(ctx context.Context, financeChannel FinanceReaderStream)

func (scheduler OrderScheduler) OrderSchedulerTask(ctx context.Context, triggerHistory entities.TriggerHistory) future.IFuture {

	startAt := triggerHistory.TriggeredAt.Add(-scheduler.financeTriggerInterval)
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerHistory), triggerHistory)
	var iFuture = future.FactorySync().Build()

	task := func() {
		orderStream, fetchOrderResultStream, fetchOrderTask := scheduler.FetchOrders(ctx, startAt, *triggerHistory.TriggeredAt)
		orderChannelStream, fanOutOrderTask := scheduler.FanOutOrders(ctx, orderStream)
		financeStream, fanInOrderResultStream, fanInOrderTask := scheduler.FanInOrderStreams(ctx, orderChannelStream)
		financeChannelStream, fanOutFinanceTask := scheduler.FanOutFinances(ctx, financeStream)
		fanInFinanceResultStream, fanInFinanceTask := scheduler.FanInFinanceStreams(ctx, financeChannelStream)

		resultStream, err := scheduler.fanInResultStream(ctx, fetchOrderResultStream, fanInOrderResultStream, fanInFinanceResultStream)
		if err != nil {
			cancel()
			future.FactoryOf(iFuture).
				SetError(future.InternalError, "OrderSchedulerTask failed", errors.Wrap(err, "fanInResultStream failed")).
				BuildAndSend()
			return
		}

		err = scheduler.taskLauncher(fanInFinanceTask, fanOutFinanceTask, fanInOrderTask, fanOutOrderTask, fetchOrderTask)
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
				if processResult.ErrType != NoError {
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

func (scheduler OrderScheduler) FetchOrders(ctx context.Context, startAt, endAt time.Time) (OrderReaderStream, ResultReaderStream, worker_pool.Task) {
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
						"fn", "FetchOrders", "error", iFuture.Error().Reason())

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
					continue
				}
			}
		}
	}
	return orderStream, resultStream, fetchOrderTask
}

func (scheduler OrderScheduler) FanOutOrders(ctx context.Context, orderChannel OrderReaderStream) (ORSStream, worker_pool.Task) {

	sellerStreamMap := make(map[uint64]OrderWriterStream, DefaultSellerSize)
	orderChannelStream := make(chan OrderReaderStream)

	fanOutTask := func() {

		defer func() {
			for _, stream := range sellerStreamMap {
				close(stream)
			}

			close(orderChannelStream)
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
				orderChannelStream <- orderStream
			} else {
				writerStream <- sellerOrder
			}
		}
	}

	return orderChannelStream, fanOutTask
}

func (scheduler OrderScheduler) FanInOrderStreams(ctx context.Context, orderChanStream ORSStream) (FinanceReaderStream, ResultReaderStream, worker_pool.Task) {
	multiplexedFinanceStream := make(chan *entities.SellerFinance)
	resultStream := make(chan *ProcessResult)
	var wg sync.WaitGroup
	fanInCoreTask := func() {
		defer func() {
			close(multiplexedFinanceStream)
			close(resultStream)
		}()

		for orderStream := range orderChanStream {
			select {
			case <-ctx.Done():
				break
			default:
			}

			financeStream, orderReduceFn := ReduceOrders()
			orderReduceTask := func() {
				orderReduceFn(ctx, orderStream)
			}

			if err := app.Globals.WorkerPool.SubmitTask(orderReduceTask); err != nil {
				log.GLog.Logger.Error("submit orderReduceTask to WorkerPool.SubmitTask failed",
					"fn", "FanInOrderStreams", "error", err)

				resultStream <- &ProcessResult{
					Function:      FanInOrderStreamsFn,
					SellerFinance: nil,
					ErrType:       WorkerPoolError,
					Result:        false,
				}
				break
			}

			fanInMultiplexTask := func() {
				defer wg.Done()
				for finance := range financeStream {
					multiplexedFinanceStream <- finance
				}
			}

			wg.Add(1)
			if err := app.Globals.WorkerPool.SubmitTask(fanInMultiplexTask); err != nil {
				log.GLog.Logger.Error("submit fanInMultiplexTask to WorkerPool.SubmitTask failed",
					"fn", "FanInOrderStreams", "error", err)

				resultStream <- &ProcessResult{
					Function:      FanInOrderStreamsFn,
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
	return multiplexedFinanceStream, resultStream, fanInCoreTask
}

func ReduceOrders() (FinanceReaderStream, OrderReduceFunc) {
	financeStream := make(chan *entities.SellerFinance)

	return financeStream, func(ctx context.Context, orderStream OrderReaderStream) {
		triggerHistory := ctx.Value(string(utils.CtxTriggerHistory)).(entities.TriggerHistory)
		sellerFinance := &entities.SellerFinance{
			OrdersInfo: []*entities.OrderInfo{
				{
					TriggerHistoryId: triggerHistory.ID,
					Orders:           nil,
				},
			},
		}
		sellerFinance.OrdersInfo[0].Orders = make([]*entities.SellerOrder, 0, DefaultSellerOrders)
		defer close(financeStream)
		for sellerOrder := range orderStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			if sellerFinance.SellerId == 0 {
				sellerFinance.SellerId = sellerOrder.SellerId

			} else if sellerFinance.SellerId != sellerOrder.SellerId {
				log.GLog.Logger.Error("sellerId of sellerFinance mismatch with sellerOrder",
					"fn", "ReduceOrders",
					"sellerId", sellerFinance.SellerId,
					"sellerOrder", sellerOrder)
				continue
			}

			iFuture := app.Globals.SellerFinanceRepository.CountWithFilter(ctx, func() interface{} {
				return bson.D{
					{"sellerId", sellerFinance.SellerId},
					{"ordersInfo.orders.oid", sellerOrder.OId},
					{"deletedAt", nil}}
			}).Get()

			if iFuture.Error() != nil {
				log.GLog.Logger.Error("SellerFinanceRepository.CountWithFilter for find order history failed",
					"fn", "ReduceOrders",
					"oid", sellerOrder.OId,
					"sellerId", sellerOrder.SellerId,
					"error", iFuture.Error())
				continue
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
							"fn", "ReduceOrders",
							"oid", sellerOrder.OId,
							"sellerId", sellerOrder.SellerId,
							"sid", item.SId,
							"error", iFuture.Error())
						continue
					}

					if iFuture.Data().(int64) > 0 {
						log.GLog.Logger.Warn("duplicate order subpackage detected",
							"fn", "ReduceOrders",
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
						"fn", "ReduceOrders",
						"oid", sellerOrder.OId,
						"sellerId", sellerOrder.SellerId)
					continue
				}
			}

			sellerFinance.OrdersInfo[0].Orders = append(sellerFinance.OrdersInfo[0].Orders, sellerOrder)
		}

		if len(sellerFinance.OrdersInfo[0].Orders) > 0 {
			financeStream <- sellerFinance
		} else {
			log.GLog.Logger.Warn("seller finance dropped because of valid order not found",
				"fn", "ReduceOrders",
				"sellerId", sellerFinance.SellerId)
		}
	}
}

func (scheduler OrderScheduler) FanOutFinances(ctx context.Context, financeChannel FinanceReaderStream) (FRSStream, worker_pool.Task) {
	financeChannels := make([]chan *entities.SellerFinance, 0, app.Globals.Config.Mongo.MinPoolSize/2)
	financeChannelStream := make(chan FinanceReaderStream)

	financeFanOutTask := func() {
		defer func() {
			for _, stream := range financeChannels {
				close(stream)
			}

			close(financeChannelStream)
		}()

		index := 0
		initIndex := 0
		for sellerFinance := range financeChannel {
			log.GLog.Logger.Debug("received order",
				"fn", "FanOutFinance",
				"sellerId", sellerFinance.SellerId)
			select {
			case <-ctx.Done():
				return
			default:
			}

			if initIndex < cap(financeChannels) {
				financeStream := make(chan *entities.SellerFinance, DefaultFanOutStreamBuffers)
				financeChannels = append(financeChannels, financeStream)
				financeChannelStream <- financeStream
				initIndex++
			}

			if index > len(financeChannels) {
				index = 0
			}

			financeChannels[index] <- sellerFinance
			index++
		}
	}

	return financeChannelStream, financeFanOutTask
}

func (scheduler OrderScheduler) CreateUpdateFinance() (ResultReaderStream, CreateUpdateFinanceFunc) {

	resultStream := make(chan *ProcessResult)
	return resultStream, func(ctx context.Context, financeChannel FinanceReaderStream) {
		defer close(resultStream)

		timestamp := time.Now().UTC()
		for sellerFinance := range financeChannel {

			select {
			case <-ctx.Done():
				return
			default:
			}

			iFuture := app.Globals.SellerFinanceRepository.FindByFilter(ctx, func() interface{} {
				return bson.D{{"sellerId", sellerFinance.SellerId},
					{"status", entities.FinanceOrderCollectionStatus},
					{"deletedAt", nil},
					{"endAt", bson.D{{"$gte", timestamp}}},
				}
			}).Get()

			triggerHistory := ctx.Value(string(utils.CtxTriggerHistory)).(entities.TriggerHistory)
			//triggerInterval := ctx.Value(utils.CtxTriggerInterval).(time.Duration)
			//triggerDuration := ctx.Value(utils.CtxTriggerDuration).(time.Duration)

			isNotFoundFlag := false
			if iFuture.Error() != nil {
				if iFuture.Error().Code() != future.NotFound {
					log.GLog.Logger.Error("SellerFinanceRepository.FindByFilter failed",
						"fn", "CreateUpdateFinance",
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error().Reason())

					resultStream <- &ProcessResult{
						Function:      CreateUpdateFinanceFn,
						SellerFinance: sellerFinance,
						ErrType:       DatabaseError,
						Result:        false,
					}
					continue

				} else {
					isNotFoundFlag = true
				}
			}

			if !isNotFoundFlag {
				sellerFinances := iFuture.Data().([]*entities.SellerFinance)

				if len(sellerFinances) > 1 {
					log.GLog.Logger.Debug("many sellers found in collect order state",
						"fn", "CreateUpdateFinance",
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
								"fn", "CreateUpdateFinance",
								"fid", finance.FId,
								"sellerId", sellerFinance.SellerId,
								"state", entities.FinanceOrderCollectionStatus)
						}
					}
				}

				if foundSellerFinance != nil {
					log.GLog.Logger.Info("found valid sellerFinance in collect order state",
						"fn", "CreateUpdateFinance",
						"fid", foundSellerFinance.FId,
						"sellerId", sellerFinance.SellerId,
						"state", entities.FinanceOrderCollectionStatus)

					newOrderInfo := &entities.OrderInfo{
						TriggerName:      triggerHistory.TriggerName,
						TriggerHistoryId: triggerHistory.ID,
						Orders:           nil,
					}

					newOrderInfo.Orders = make([]*entities.SellerOrder, 0, len(sellerFinance.OrdersInfo[0].Orders))

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
								} else {
									log.GLog.Logger.Warn("duplicate order subpackage found",
										"fn", "CreateUpdateFinance",
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
										"fn", "CreateUpdateFinance",
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
							"fn", "CreateUpdateFinance",
							"sellerId", sellerFinance.SellerId,
							"trigger", triggerHistory.TriggerName,
							"triggerHistoryId", triggerHistory.ID)
						continue
					}

					iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *foundSellerFinance).Get()
					if iFuture.Error() != nil {
						log.GLog.Logger.Error("SellerFinanceRepository.Update failed",
							"fn", "CreateUpdateFinance",
							"fid", foundSellerFinance.FId,
							"sellerId", foundSellerFinance.SellerId,
							"error", iFuture.Error().Reason())

						resultStream <- &ProcessResult{
							Function:      CreateUpdateFinanceFn,
							SellerFinance: foundSellerFinance,
							ErrType:       DatabaseError,
							Result:        false,
						}
						continue
					}

					resultStream <- &ProcessResult{
						Function:      CreateUpdateFinanceFn,
						SellerFinance: foundSellerFinance,
						ErrType:       NoError,
						Result:        true,
					}

					// create new seller finance
				} else {
					startAt := triggerHistory.TriggeredAt.Add(-scheduler.financeTriggerInterval)
					endAt := startAt.Add(scheduler.financeTriggerDuration)

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
							"fn", "CreateUpdateFinance",
							"sellerId", sellerFinance.SellerId,
							"error", iFuture.Error().Reason())
					} else {
						newSellerFinance.SellerInfo = iFuture.Data().(*entities.SellerProfile)
					}

					iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *newSellerFinance).Get()
					if iFuture.Error() != nil {
						log.GLog.Logger.Error("SellerFinanceRepository.Save failed",
							"fn", "CreateUpdateFinance",
							"sellerId", newSellerFinance.SellerId,
							"startAt", startAt,
							"endAt", endAt,
							"error", iFuture.Error().Reason())

						resultStream <- &ProcessResult{
							Function:      CreateUpdateFinanceFn,
							SellerFinance: newSellerFinance,
							ErrType:       DatabaseError,
							Result:        false,
						}
						continue
					}

					resultStream <- &ProcessResult{
						Function:      CreateUpdateFinanceFn,
						SellerFinance: newSellerFinance,
						ErrType:       NoError,
						Result:        true,
					}
				}
				// create new seller finance
			} else {
				startAt := triggerHistory.TriggeredAt.Add(-scheduler.financeTriggerInterval)
				endAt := startAt.Add(scheduler.financeTriggerDuration)

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
						"fn", "CreateUpdateFinance",
						"sellerId", sellerFinance.SellerId,
						"error", iFuture.Error().Reason())
				} else {
					newSellerFinance.SellerInfo = iFuture.Data().(*entities.SellerProfile)
				}

				iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *newSellerFinance).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("SellerFinanceRepository.Save failed",
						"fn", "CreateUpdateFinance",
						"sellerId", newSellerFinance.SellerId,
						"startAt", startAt,
						"endAt", endAt,
						"error", iFuture.Error().Reason())

					resultStream <- &ProcessResult{
						Function:      CreateUpdateFinanceFn,
						SellerFinance: newSellerFinance,
						ErrType:       DatabaseError,
						Result:        false,
					}
					continue
				}

				resultStream <- &ProcessResult{
					Function:      CreateUpdateFinanceFn,
					SellerFinance: newSellerFinance,
					ErrType:       NoError,
					Result:        true,
				}
			}
		}
	}
}

func (scheduler OrderScheduler) FanInFinanceStreams(ctx context.Context, financeChannelStream FRSStream) (ResultReaderStream, worker_pool.Task) {
	multiplexedResultStream := make(chan *ProcessResult)
	var wg sync.WaitGroup
	fanInCoreTask := func() {
		defer close(multiplexedResultStream)
		for financeChannel := range financeChannelStream {
			select {
			case <-ctx.Done():
				break
			default:
			}

			resultReaderStream, CreateUpdateFinanceFn := scheduler.CreateUpdateFinance()
			CreateUpdateFinanceTask := func() {
				CreateUpdateFinanceFn(ctx, financeChannel)
			}

			if err := app.Globals.WorkerPool.SubmitTask(CreateUpdateFinanceTask); err != nil {
				log.GLog.Logger.Error("submit CreateUpdateFinanceTask to WorkerPool.SubmitTask failed",
					"fn", "FanInFinanceStreams", "error", err)

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
					//log.GLog.Logger.Debug("received order",
					//	"fn", "FanInOrderStreams",
					//	"sellerId", finance.SellerId)
					multiplexedResultStream <- processResult
				}
			}

			wg.Add(1)
			if err := app.Globals.WorkerPool.SubmitTask(fanInMultiplexTask); err != nil {
				log.GLog.Logger.Error("submit fanInMultiplexTask to WorkerPool.SubmitTask failed",
					"fn", "FanInFinanceStreams", "error", err)

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
