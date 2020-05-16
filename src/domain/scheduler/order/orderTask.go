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
	DefaultOrderStreamSize   = 8192
	DefaultSellerSize        = 2048
	DefaultSellerOrderSize   = 4096
	DefaultFetchOrderPerPage = 128
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

func OrderSchedulerTask(ctx context.Context, triggerHistory entities.TriggerHistory) future.IFuture {

	triggerInterval := ctx.Value(utils.CtxTriggerInterval).(time.Duration)
	startAt := triggerHistory.TriggeredAt.Add(-triggerInterval)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerHistory), triggerHistory)
	ctx, cancel := context.WithCancel(ctx)
	var iFuture = future.FactorySync().Build()

	task := func() {
		orderStream, fetchOrderResultStream, fetchOrderTask := FetchOrders(ctx, startAt, *triggerHistory.TriggeredAt)
		orderChannelStream, fanOutOrderTask := FanOutOrders(ctx, orderStream)
		financeStream, fanInOrderResultStream, fanInOrderTask := FanInOrderStreams(ctx, orderChannelStream)
		financeChannelStream, fanOutFinanceTask := FanOutFinances(ctx, financeStream)
		fanInFinanceResultStream, fanInFinanceTask := FanInFinanceStreams(ctx, financeChannelStream)

		resultStream, err := fanInResultStream(ctx, fetchOrderResultStream, fanInOrderResultStream, fanInFinanceResultStream)
		if err != nil {
			cancel()
			future.FactoryOf(iFuture).
				SetError(future.InternalError, "OrderSchedulerTask failed", errors.Wrap(err, "fanInResultStream failed")).
				BuildAndSend()
			return
		}

		err = taskLauncher(fanInFinanceTask, fanOutFinanceTask, fanInOrderTask, fanOutOrderTask, fetchOrderTask)
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

func fanInResultStream(ctx context.Context, channels ...ResultReaderStream) (ResultReaderStream, error) {
	var wg sync.WaitGroup
	multiplexedStream := make(chan *ProcessResult)
	multiplex := func(c ResultReaderStream) {
		defer wg.Done()
		for i := range c {
			select {
			case <-ctx.Done():
				return
			case multiplexedStream <- i:
			}
		}
	}
	// Select from all the channels
	wg.Add(len(channels))
	for _, c := range channels {
		if err := app.Globals.WorkerPool.SubmitTask(func() { multiplex(c) }); err != nil {
			return nil, err
		}
	}
	// Wait for all the reads to complete
	if err := app.Globals.WorkerPool.SubmitTask(func() { wg.Wait(); close(multiplexedStream) }); err != nil {
		return nil, err
	}

	return multiplexedStream, nil
}

func taskLauncher(tasks ...worker_pool.Task) error {
	for _, task := range tasks {
		if err := app.Globals.WorkerPool.SubmitTask(task); err != nil {
			return err
		}
	}
	return nil
}

func FetchOrders(ctx context.Context, startAt, endAt time.Time) (OrderReaderStream, ResultReaderStream, worker_pool.Task) {
	orderStream := make(chan *entities.SellerOrder, DefaultOrderStreamSize)
	resultStream := make(chan *ProcessResult)
	//fetchOrderTask := func() {
	//	defer close(orderStream)
	//	generateOrders(orderStream)
	//}

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
					log.GLog.Logger.Error("submit fetchOrderTask to WorkerPool.SubmitTask failed",
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

func FanOutOrders(ctx context.Context, orderChannel OrderReaderStream) (ORSStream, worker_pool.Task) {

	sellerStreamMap := make(map[uint64]OrderWriterStream, DefaultSellerSize)
	orderChannelStream := make(chan OrderReaderStream)

	fanOutTask := func() {
		defer func() {
			log.GLog.Logger.Debug("complete",
				"fn", "FanOutOrders")
			for _, stream := range sellerStreamMap {
				close(stream)
			}

			close(orderChannelStream)
		}()

		for sellerOrder := range orderChannel {
			//log.GLog.Logger.Debug("received order",
			//	"fn", "FanOutOrders",
			//	"oid", sellerOrder.OId)
			select {
			case <-ctx.Done():
				break
			default:
			}

			if writerStream, ok := sellerStreamMap[sellerOrder.SellerId]; !ok {
				orderStream := make(chan *entities.SellerOrder)
				sellerStreamMap[sellerOrder.SellerId] = orderStream
				orderChannelStream <- orderStream
				orderStream <- sellerOrder
			} else {
				writerStream <- sellerOrder
			}
		}
	}

	return orderChannelStream, fanOutTask
}

func FanInOrderStreams(ctx context.Context, orderChanStream ORSStream) (FinanceReaderStream, ResultReaderStream, worker_pool.Task) {
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
					//log.GLog.Logger.Debug("received order",
					//	"fn", "FanInOrderStreams",
					//	"sellerId", finance.SellerId)
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
		triggerHistory := ctx.Value(utils.CtxTriggerHistory).(entities.TriggerHistory)
		sellerFinance := &entities.SellerFinance{
			OrdersInfo: []*entities.OrderInfo{
				{
					TriggerHistory: triggerHistory.ID,
					Orders:         nil,
				},
			},
		}
		sellerFinance.OrdersInfo[0].Orders = make([]*entities.SellerOrder, 0, DefaultSellerOrderSize)
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
			sellerFinance.OrdersInfo[0].Orders = append(sellerFinance.OrdersInfo[0].Orders, sellerOrder)
		}

		financeStream <- sellerFinance
	}
}

func FanOutFinances(ctx context.Context, financeChannel FinanceReaderStream) (FRSStream, worker_pool.Task) {
	financeChannels := make([]chan *entities.SellerFinance, 0, app.Globals.Config.Mongo.MinPoolSize)
	financeChannelStream := make(chan FinanceReaderStream)

	financeFanOutTask := func() {
		defer func() {
			for _, stream := range financeChannels {
				close(stream)
			}

			close(financeChannelStream)
		}()

		index := 0
		for sellerFinance := range financeChannel {
			//log.GLog.Logger.Debug("received order",
			//	"fn", "FanOutOrders",
			//	"oid", sellerOrder.OId)
			select {
			case <-ctx.Done():
				return
			default:
			}

			if index >= len(financeChannels) {
				index = 0
			}

			if financeChannels[index] == nil {
				financeStream := make(chan *entities.SellerFinance)
				financeChannels[index] = financeStream
				financeChannelStream <- financeStream
				financeStream <- sellerFinance
			} else {
				financeChannels[index] <- sellerFinance
			}
			index++
		}
	}

	return financeChannelStream, financeFanOutTask
}

func CreateUpdateFinance() (ResultReaderStream, CreateUpdateFinanceFunc) {

	resultStream := make(chan *ProcessResult)
	return resultStream, func(ctx context.Context, financeChannel FinanceReaderStream) {
		defer close(resultStream)
		for sellerFinance := range financeChannel {

			select {
			case <-ctx.Done():
				return
			default:
			}

			iFuture := app.Globals.SellerFinanceRepository.FindByFilter(ctx, func() interface{} {
				return bson.D{{"sellerId", sellerFinance.SellerId},
					{"status", entities.FinanceOrderCollectionStatus},
					{"deletedAt", nil}}
			}).Get()

			triggerHistory := ctx.Value(utils.CtxTriggerHistory).(entities.TriggerHistory)
			triggerInterval := ctx.Value(utils.CtxTriggerInterval).(time.Duration)
			triggerDuration := ctx.Value(utils.CtxTriggerDuration).(time.Duration)

			timestamp := time.Now().UTC()
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
						TriggerHistory: triggerHistory.ID,
						Orders:         nil,
					}

					newOrderInfo.Orders = make([]*entities.SellerOrder, 0, len(sellerFinance.OrdersInfo[0].Orders))

					for _, newOrder := range sellerFinance.OrdersInfo[0].Orders {
						for _, orderInfo := range foundSellerFinance.OrdersInfo {
							for _, order := range orderInfo.Orders {
								if order.OId == newOrder.OId {
									diffItems := make([]*entities.SellerItem, 0, len(newOrder.Items))
									for _, newItem := range newOrder.Items {
										for _, item := range order.Items {
											if newItem.SId != item.SId {
												diffItems = append(diffItems, newItem)
											} else {
												log.GLog.Logger.Warn("duplicate order subpackage found",
													"fn", "CreateUpdateFinance",
													"sellerId", sellerFinance.SellerId,
													"oid", order.OId,
													"sid", item.SId)
											}
										}
									}
									if len(diffItems) > 0 {
										newOrder.Items = diffItems
										orderInfo.Orders = append(orderInfo.Orders, newOrder)
									} else {
										log.GLog.Logger.Warn("duplicate order found",
											"fn", "CreateUpdateFinance",
											"sellerId", sellerFinance.SellerId,
											"oid", newOrder.OId)
									}
								} else {
									orderInfo.Orders = append(orderInfo.Orders, newOrder)
								}
							}
						}
					}

					foundSellerFinance.OrdersInfo = append(foundSellerFinance.OrdersInfo, newOrderInfo)

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
					startAt := triggerHistory.TriggeredAt.Add(-triggerInterval)
					endAt := startAt.Add(triggerDuration)

					newSellerFinance := &entities.SellerFinance{
						SellerId:   sellerFinance.SellerId,
						Trigger:    triggerHistory.TriggerName,
						SellerInfo: nil,
						Invoice:    nil,
						OrdersInfo: []*entities.OrderInfo{
							{
								TriggerHistory: triggerHistory.ID,
								Orders:         sellerFinance.OrdersInfo[0].Orders,
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
				startAt := triggerHistory.TriggeredAt.Add(-triggerInterval)
				endAt := startAt.Add(triggerDuration)

				newSellerFinance := &entities.SellerFinance{
					SellerId:   sellerFinance.SellerId,
					Trigger:    triggerHistory.TriggerName,
					SellerInfo: nil,
					Invoice:    nil,
					OrdersInfo: []*entities.OrderInfo{
						{
							TriggerHistory: triggerHistory.ID,
							Orders:         sellerFinance.OrdersInfo[0].Orders,
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

func FanInFinanceStreams(ctx context.Context, financeChannelStream FRSStream) (ResultReaderStream, worker_pool.Task) {
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

			resultReaderStream, CreateUpdateFinanceFn := CreateUpdateFinance()
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

func generateOrders(stream chan *entities.SellerOrder) {
	for i := 0; i < 10; i++ {
		order := createOrder()
		order.OId = uint64(i)
		stream <- order
	}

	for i := 0; i < 10; i++ {
		order := createOrder()
		order.OId = uint64(i)
		order.SellerId = 100002
		stream <- order
	}
}

func createOrder() *entities.SellerOrder {
	return &entities.SellerOrder{
		OId:      0,
		FId:      "",
		SellerId: 100001,
		RawShippingNet: &entities.Money{
			Amount:   "1650000",
			Currency: "IRR",
		},
		RoundupShippingNet: &entities.Money{
			Amount:   "1650000",
			Currency: "IRR",
		},
		Items: []*entities.SellerItem{
			{
				SId:         1111111111222,
				SKU:         "yt545-34",
				InventoryId: "666777888999",
				Title:       "Mobile",
				Brand:       "Nokia",
				Guaranty:    "Sazegar",
				Category:    "Electronic",
				Image:       "",
				Returnable:  false,
				Quantity:    5,
				Attributes: map[string]*entities.Attribute{
					"Color": {
						KeyTranslate: map[string]string{
							"en": "رنگ",
							"fa": "رنگ",
						},
						ValueTranslate: map[string]string{
							"en": "رنگ",
							"fa": "رنگ",
						},
					},
					"dial_color": {
						KeyTranslate: map[string]string{
							"fa": "رنگ صفحه",
							"en": "رنگ صفحه",
						},
						ValueTranslate: map[string]string{
							"fa": "رنگ صفحه",
							"en": "رنگ صفحه",
						},
					},
				},
				Invoice: &entities.ItemInvoice{
					Commission: &entities.ItemCommission{
						ItemCommission: 9,
						RawUnitPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupUnitPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawTotalPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupTotalPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
					},
					Share: &entities.ItemShare{
						RawItemNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupItemNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawTotalNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupTotalNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawUnitSellerShare: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupUnitSellerShare: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawTotalSellerShare: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupTotalSellerShare: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
					},
					SSO: &entities.ItemSSO{
						Rate:      8,
						IsObliged: true,
						RawUnitPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupUnitPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawTotalPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupTotalPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
					},
					VAT: &entities.ItemVAT{
						Rate:      8,
						IsObliged: true,
						RawUnitPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupUnitPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawTotalPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupTotalPrice: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
					},
				},
			},
		},
		SubPkgUpdatedAt: time.Now(),
		SubPkgCreatedAt: time.Now(),
		DeletedAt:       nil,
	}
}
