package order_scheduler

import (
	"context"
	"errors"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"go.mongodb.org/mongo-driver/bson"
	"strings"
	"sync"
	"time"
)

type startWardFn func(ctx context.Context, pulseInterval time.Duration, scheduleInterval time.Duration) (heartbeat <-chan interface{})
type startStewardFn func(ctx context.Context, pulseInterval time.Duration) (heartbeat <-chan interface{})

type OrderScheduler struct {
	schedulerInterval       time.Duration
	schedulerStewardTimeout time.Duration
	schedulerWorkerTimeout  time.Duration

	schedulerTriggerOffsetPoint time.Duration
	schedulerTriggerInterval    time.Duration
	schedulerTriggerDuration    time.Duration
	schedulerTriggerPointType   entities.TriggerPointType

	schedulerTimeUnit        utils.TimeUnit
	schedulerTriggerTimeUnit utils.TimeUnit
	waitGroup                sync.WaitGroup
	mux                      sync.Mutex
}

func NewOrderScheduler(schedulerInterval, schedulerStewardTimeout, schedulerWorkerTimeout,
	schedulerTriggerInterval, triggerPointOffset, schedulerTriggerDuration time.Duration,
	schedulerTriggerPointType entities.TriggerPointType,
	schedulerTimeUnit, schedulerTriggerTimeUnit utils.TimeUnit) OrderScheduler {

	return OrderScheduler{
		schedulerInterval:           schedulerInterval,
		schedulerStewardTimeout:     schedulerStewardTimeout,
		schedulerWorkerTimeout:      schedulerWorkerTimeout,
		schedulerTriggerOffsetPoint: triggerPointOffset,
		schedulerTriggerInterval:    schedulerTriggerInterval,
		schedulerTriggerDuration:    schedulerTriggerDuration,
		schedulerTriggerPointType:   schedulerTriggerPointType,
		schedulerTimeUnit:           schedulerTimeUnit,
		schedulerTriggerTimeUnit:    schedulerTriggerTimeUnit,
	}
}

func (scheduler OrderScheduler) SchedulerStart(ctx context.Context) error {

	if err := scheduler.init(ctx); err != nil {
		return err
	}

	scheduler.waitGroup.Add(1)
	go scheduler.scheduleProcess(ctx)
	scheduler.waitGroup.Wait()
	return nil
}

func (scheduler OrderScheduler) init(ctx context.Context) error {

	var isNotFoundFlag = false
	var activeTrigger *entities.SchedulerTrigger = nil
	iFuture := app.Globals.TriggerRepository.FindByName(ctx, app.Globals.Config.App.FinanceOrderSchedulerTriggerName).Get()
	if iFuture.Error() != nil {
		if iFuture.Error().Code() != future.NotFound {
			log.GLog.Logger.Error("TriggerRepository.FindByName failed",
				"fn", "init",
				"error", iFuture.Error().Reason())
			return iFuture.Error()
		} else {
			isNotFoundFlag = true
		}
	}

	if isNotFoundFlag {
		iFuture := app.Globals.TriggerRepository.FindActiveTrigger(ctx, app.Globals.Config.App.FinanceOrderSchedulerTriggerTestMode).Get()
		if iFuture.Error() != nil {
			if iFuture.Error().Code() != future.NotFound {
				log.GLog.Logger.Error("TriggerRepository.FindActiveTrigger failed",
					"fn", "init",
					"error", iFuture.Error().Reason())
				return iFuture.Error()
			}
		} else {
			activeTrigger = iFuture.Data().(*entities.SchedulerTrigger)
			activeTrigger.IsActive = false
			activeTrigger.IsEnable = false
			iFuture = app.Globals.TriggerRepository.Update(ctx, *activeTrigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerRepository.Update active trigger failed",
					"fn", "init",
					"trigger", activeTrigger,
					"error", iFuture.Error().Reason())
				return iFuture.Error()
			}

			log.GLog.Logger.Info("active trigger delete success",
				"fn", "init",
				"trigger", activeTrigger)
		}

		var triggerAt time.Time
		dt := time.Now().UTC()
		if scheduler.schedulerTriggerPointType == entities.AbsoluteTrigger {
			dateTimeString := dt.Format("2006-01-02") + "T" + app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint + ":00-0000"
			triggerPoint, err := time.Parse(utils.ISO8601, dateTimeString)
			if err != nil {
				log.GLog.Logger.FromContext(ctx).Error("current date time parse failed",
					"fn", "init",
					"timestamp", dateTimeString,
					"error", err)
				triggerPoint = dt
			}

			if dt.After(triggerPoint) {
				triggerAt = triggerPoint.AddDate(0, 0, triggerPoint.Day()+1)
			} else if dt.Before(triggerPoint) || dt.Equal(triggerPoint) {
				triggerAt = triggerPoint
			}
		} else {
			triggerAt = dt.Add(scheduler.schedulerTriggerInterval)
		}

		newTrigger := &entities.SchedulerTrigger{
			Name:             app.Globals.Config.App.FinanceOrderSchedulerTriggerName,
			Duration:         int64(app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration),
			Interval:         int64(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval),
			TimeUnit:         utils.TimeUnit(strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)),
			TriggerPoint:     app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint,
			TriggerPointType: entities.TriggerPointType(strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType)),
			TriggerAt:        &triggerAt,
			IsActive:         true,
			IsEnable:         app.Globals.Config.App.FinanceOrderSchedulerTriggerEnabled,
			TestMode:         app.Globals.Config.App.FinanceOrderSchedulerTriggerTestMode,
			CreatedAt:        dt,
			UpdatedAt:        dt,
		}

		iFuture = app.Globals.TriggerRepository.Save(ctx, *newTrigger).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerRepository.Save new trigger failed",
				"fn", "init",
				"trigger", newTrigger,
				"error", iFuture.Error().Reason())
			return iFuture.Error()
		}

		newTrigger = iFuture.Data().(*entities.SchedulerTrigger)
		if app.Globals.Config.App.FinanceOrderSchedulerUpdateFinanceDuration && activeTrigger != nil {
			if err := scheduler.updateSellerFinanceWithNewTriggerConfig(ctx, activeTrigger, newTrigger); err != nil {
				log.GLog.Logger.Error("updateSellerFinance With NewTriggerConfig failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"trigger", newTrigger,
					"error", iFuture.Error().Reason())
				return err
			} else {
				log.GLog.Logger.Info("updateSellerFinance With NewTriggerConfig success",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"trigger", newTrigger)
			}
		}

		if app.Globals.Config.App.FinanceOrderSchedulerHandleMissedFireTrigger {
			if missedCount, err := scheduler.missedFireTriggerHandler(ctx, activeTrigger); err != nil {
				log.GLog.Logger.Error("missedFireTriggerHandler failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"error", iFuture.Error().Reason())
				return err
			} else {
				log.GLog.Logger.Info("missedFireTriggerHandler success",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"missedCount", missedCount)
			}
		}

	} else {
		activeTrigger := iFuture.Data().(*entities.SchedulerTrigger)
		if activeTrigger.TimeUnit != scheduler.schedulerTriggerTimeUnit {
			log.GLog.Logger.Error("the activeTrigger exist and FinanceOrderSchedulerTriggerTimeUnit modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old timeUnit", activeTrigger.TimeUnit,
				"new timeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
			return errors.New("unit time change not acceptable")

		} else if activeTrigger.Interval != int64(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval) {
			log.GLog.Logger.Error("the activeTrigger exist and FinanceOrderSchedulerTriggerInterval modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old interval", activeTrigger.Interval,
				"new interval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
			return errors.New("interval change not acceptable")

		} else if activeTrigger.TriggerPoint != app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint {
			log.GLog.Logger.Error("the activeTrigger exist and FinanceOrderSchedulerTriggerPoint modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old triggerPoint", activeTrigger.TriggerPoint,
				"new triggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			return errors.New("triggerPoint change not acceptable")

		} else if activeTrigger.TriggerPointType != scheduler.schedulerTriggerPointType {
			log.GLog.Logger.Error("the activeTrigger exist and FinanceOrderSchedulerTriggerPointType modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old triggerPointType", activeTrigger.TriggerPointType,
				"new triggerPointType", app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType)
			return errors.New("triggerPoint change not acceptable")
		}

		if activeTrigger.IsEnable != app.Globals.Config.App.FinanceOrderSchedulerTriggerEnabled {
			activeTrigger.IsEnable = app.Globals.Config.App.FinanceOrderSchedulerTriggerEnabled
			iFuture = app.Globals.TriggerRepository.Update(ctx, *activeTrigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerRepository.Update failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"error", iFuture.Error().Reason())
				return iFuture.Error()
			}
		}

	}

	return nil
}

// TODO performance optimization required
func (scheduler OrderScheduler) updateSellerFinanceWithNewTriggerConfig(ctx context.Context, activeTrigger, newTrigger *entities.SchedulerTrigger) error {

	timestamp := time.Now().UTC()
	var activeDuration, newTriggerDuration time.Duration
	if activeTrigger.TimeUnit == utils.HourUnit {
		activeDuration = time.Duration(activeTrigger.Duration) * time.Hour
	} else {
		activeDuration = time.Duration(activeTrigger.Duration) * time.Minute
	}

	if newTrigger.TimeUnit == utils.HourUnit {
		newTriggerDuration = time.Duration(newTrigger.Duration) * time.Hour
	} else {
		newTriggerDuration = time.Duration(newTrigger.Duration) * time.Minute
	}

	newDuration := activeDuration - newTriggerDuration
	if newDuration < 0 {
		newDuration = -newDuration
		log.GLog.Logger.Info("finance Duration must be shrink according to new trigger",
			"oldDuration", activeTrigger.Duration,
			"newDuration", newTrigger.Duration,
			"diffDuration", newDuration)

		var availablePages = 1
		for i := 0; i < availablePages; i++ {
			iFuture := app.Globals.SellerFinanceRepository.FindByFilterWithPage(ctx, func() interface{} {
				return bson.D{{"ordersInfo.triggerName", activeTrigger.Name},
					{"deletedAt", nil}}
			}, int64(i+1), DefaultFetchOrderPerPage).Get()

			if iFuture.Error() != nil {
				if iFuture.Error().Code() != future.NotFound {
					log.GLog.Logger.Error("SellerFinanceRepository.FindByFilterWithPage failed",
						"fn", "updateSellerFinanceWithNewTriggerConfig",
						"activeTrigger", activeTrigger,
						"error", iFuture.Error().Reason())
					return iFuture.Error()
				}
				log.GLog.Logger.Info("seller finance not found for active trigger",
					"fn", "updateSellerFinanceWithNewTriggerConfig",
					"activeTrigger", activeTrigger)
				return nil
			}

			financeResult := iFuture.Data().(finance_repository.FinancePageableResult)

			if financeResult.TotalCount%DefaultFetchOrderPerPage != 0 {
				availablePages = (int(financeResult.TotalCount) / DefaultFetchOrderPerPage) + 1
			} else {
				availablePages = int(financeResult.TotalCount) / DefaultFetchOrderPerPage
			}

			if financeResult.TotalCount < DefaultFetchOrderPerPage {
				availablePages = 1
			}

			for _, finance := range financeResult.SellerFinances {
				offset := timestamp.Sub(*finance.StartAt)
				if offset > newDuration {
					newEndAt := finance.StartAt.Add(offset)
					finance.EndAt = &newEndAt
				} else {
					newEndAt := finance.StartAt.Add(newDuration)
					finance.EndAt = &newEndAt
				}

				iFuture = app.Globals.SellerFinanceRepository.Save(ctx, *finance).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("update of finance's endAt failed",
						"fn", "updateSellerFinanceWithNewTriggerConfig",
						"fid", finance.FId,
						"startAt", finance.StartAt,
						"endAt", finance.EndAt,
						"error", iFuture.Error().Reason())
					return iFuture.Error()
				}

				log.GLog.Logger.Debug("update of finance's endAt success",
					"fn", "updateSellerFinanceWithNewTriggerConfig",
					"fid", finance.FId,
					"startAt", finance.StartAt,
					"endAt", finance.EndAt)
			}
		}
	} else {
		log.GLog.Logger.Info("finance Duration should be gross in next duration according to new trigger",
			"oldDuration", activeTrigger.Duration,
			"newDuration", newTrigger.Duration,
			"diffDuration", newDuration)
	}

	return nil
}

func (scheduler OrderScheduler) missedFireTriggerHandler(ctx context.Context, activeTrigger *entities.SchedulerTrigger) (int, error) {

	var missedFireCount = 0
	timestamp := time.Now().UTC()
	var activeInterval time.Duration
	if activeTrigger.TimeUnit == utils.HourUnit {
		activeInterval = time.Duration(activeTrigger.Interval) * time.Hour
	} else {
		activeInterval = time.Duration(activeTrigger.Interval) * time.Minute
	}

	timeOffset := timestamp.Sub(activeTrigger.CreatedAt)

	if timeOffset > activeInterval {
		intervalCount := timeOffset / activeInterval

		iFuture := app.Globals.TriggerHistoryRepository.CountWithFilter(ctx, func() interface{} {
			return bson.D{{"triggerName", activeTrigger.Name}}
		}).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerHistoryRepository.CountWithFilter failed",
				"fn", "missedFireTriggerHandler",
				"activeTrigger", activeTrigger,
				"error", iFuture.Error().Reason())
			return missedFireCount, iFuture.Error()
		}

		triggersCount := iFuture.Data().(int64)
		if int64(intervalCount) > triggersCount {
			log.GLog.Logger.Info("missed fire trigger history detected",
				"fn", "missedFireTriggerHandler",
				"triggerHistoryCount", triggersCount,
				"totalIntervalCount", intervalCount)

			var newTriggeredAt time.Time
			for {
				newTriggeredAt = activeTrigger.CreatedAt.Add(activeInterval)
				if timestamp.Before(newTriggeredAt) {
					break
				}

				iFuture := app.Globals.TriggerHistoryRepository.ExistsByTriggeredAt(ctx, newTriggeredAt).Get()
				if iFuture.Error() != nil {
					log.GLog.Logger.Error("TriggerHistoryRepository.ExistsByTriggeredAt failed",
						"fn", "missedFireTriggerHandler",
						"activeTrigger", activeTrigger,
						"error", iFuture.Error().Reason())
					return missedFireCount, iFuture.Error()
				}

				if !iFuture.Data().(bool) {
					log.GLog.Logger.Info("missed fire trigger found",
						"fn", "missedFireTriggerHandler",
						"triggerName", activeTrigger.Name,
						"missedTriggeredAt", newTriggeredAt)

					missedFireCount++

					missedTrigger := entities.TriggerHistory{
						TriggerName:  activeTrigger.Name,
						ExecResult:   entities.TriggerExecResultNone,
						TriggeredAt:  &newTriggeredAt,
						IsMissedFire: true,
						CreatedAt:    timestamp,
						UpdatedAt:    timestamp,
						DeletedAt:    nil,
					}

					iFuture := app.Globals.TriggerHistoryRepository.Save(ctx, missedTrigger).Get()
					if iFuture.Error() != nil {
						log.GLog.Logger.Error("TriggerHistoryRepository.Save failed",
							"fn", "missedFireTriggerHandler",
							"activeTrigger", activeTrigger,
							"missedTrigger", missedTrigger,
							"error", iFuture.Error().Reason())
						return missedFireCount, iFuture.Error()
					}
				}
			}
			//var availablePages = 1
			//for i := 0; i < availablePages; i++ {
			//	iFuture := app.Globals.TriggerHistoryRepository.FindByFilterWithPageAndSort(ctx, func() (interface{}, string, int) {
			//		return bson.D{{"triggerName", activeTrigger.Name}, {"deletedAt", nil}}, "triggeredAt", 1
			//	}, int64(i+1), DefaultFetchOrderPerPage).Get()
			//
			//	if iFuture.Error() != nil {
			//		if iFuture.Error().Code() != future.NotFound {
			//			log.GLog.Logger.Error("TriggerHistoryRepository.FindByFilterWithPageAndSort failed",
			//				"fn", "missedFireTriggerHandler",
			//				"activeTrigger", activeTrigger,
			//				"error", iFuture.Error().Reason())
			//			return iFuture.Error()
			//		}
			//		log.GLog.Logger.Error("trigger history not found for active trigger",
			//			"fn", "missedFireTriggerHandler",
			//			"activeTrigger", activeTrigger)
			//		return nil
			//	}
			//
			//	historyResult := iFuture.Data().(trigger_history_repository.HistoryPageableResult)
			//
			//	if historyResult.TotalCount%DefaultFetchOrderPerPage != 0 {
			//		availablePages = (int(historyResult.TotalCount) / DefaultFetchOrderPerPage) + 1
			//	} else {
			//		availablePages = int(historyResult.TotalCount) / DefaultFetchOrderPerPage
			//	}
			//
			//	if historyResult.TotalCount < DefaultFetchOrderPerPage {
			//		availablePages = 1
			//	}
			//
			//	var newTriggeredAt time.Time
			//	for index, triggerHistory := range historyResult.Histories {
			//		newTriggeredAt = activeTrigger.CreatedAt.Add(activeInterval)
			//
			//
			//	}
			//}
		} else if int64(intervalCount) == triggersCount {
			log.GLog.Logger.Info("missed fire not found",
				"fn", "missedFireTriggerHandler",
				"triggerHistoryCount", triggersCount,
				"totalIntervalCount", intervalCount)
		} else {
			log.GLog.Logger.Error("fatal error in missed fire calculation",
				"fn", "missedFireTriggerHandler",
				"triggerHistoryCount", triggersCount,
				"totalIntervalCount", intervalCount)
			return missedFireCount, errors.New("missed fire calculation failed")
		}
	} else {
		log.GLog.Logger.Info("trigger hasn't history",
			"fn", "missedFireTriggerHandler",
			"offset", timeOffset,
			"interval", activeInterval)
	}

	return missedFireCount, nil
}

func (scheduler OrderScheduler) scheduleProcess(ctx context.Context) {

	// first run
	scheduler.doProcess(ctx)

	stewardCtx, stewardCtxCancel := context.WithCancel(context.Background())
	stewardWorkerFn := scheduler.stewardFn(utils.ORContext(ctx, stewardCtx), scheduler.schedulerWorkerTimeout, scheduler.schedulerInterval, scheduler.worker)
	heartbeat := stewardWorkerFn(ctx, scheduler.schedulerStewardTimeout)
	stewardTimer := time.NewTimer(scheduler.schedulerStewardTimeout * 2)

	for {
		select {
		case <-ctx.Done():
			log.GLog.Logger.Debug("stewardWorkerFn goroutine context down!",
				"fn", "scheduleProcess")
			stewardTimer.Stop()
			scheduler.waitGroup.Done()
			return
		case _, ok := <-heartbeat:
			if ok == false {
				log.GLog.Logger.Debug("heartbeat of stewardWorkerFn closed",
					"fn", "scheduleProcess")
				stewardCtxCancel()
				stewardCtx, stewardCtxCancel = context.WithCancel(context.Background())
				stewardWorkerFn := scheduler.stewardFn(utils.ORContext(ctx, stewardCtx), scheduler.schedulerWorkerTimeout, scheduler.schedulerInterval, scheduler.worker)
				heartbeat = stewardWorkerFn(ctx, scheduler.schedulerStewardTimeout)
				stewardTimer.Reset(scheduler.schedulerStewardTimeout * 2)
			} else {
				//logger.Audit("scheduleProcess() => heartbeat stewardWorkerFn , state: %s", state.StateName())
				stewardTimer.Stop()
				stewardTimer.Reset(scheduler.schedulerStewardTimeout * 2)
			}

		case <-stewardTimer.C:
			log.GLog.Logger.Debug("stewardWorkerFn goroutine is not healthy!",
				"fn", "scheduleProcess")
			stewardCtxCancel()
			stewardCtx, stewardCtxCancel = context.WithCancel(context.Background())
			stewardWorkerFn := scheduler.stewardFn(utils.ORContext(ctx, stewardCtx), scheduler.schedulerWorkerTimeout, scheduler.schedulerInterval, scheduler.worker)
			heartbeat = stewardWorkerFn(ctx, scheduler.schedulerStewardTimeout)
			stewardTimer.Reset(scheduler.schedulerStewardTimeout * 2)
		}
	}
}

func (scheduler OrderScheduler) stewardFn(ctx context.Context, wardPulseInterval, wardScheduleInterval time.Duration, startWorker startWardFn) startStewardFn {
	return func(ctx context.Context, stewardPulse time.Duration) <-chan interface{} {
		heartbeat := make(chan interface{}, 1)
		go func() {
			defer close(heartbeat)

			var wardCtx context.Context
			var wardCtxCancel context.CancelFunc
			var wardHeartbeat <-chan interface{}
			startWard := func() {
				wardCtx, wardCtxCancel = context.WithCancel(context.Background())
				wardHeartbeat = startWorker(utils.ORContext(ctx, wardCtx), wardPulseInterval, wardScheduleInterval)
			}
			startWard()
			pulseTimer := time.NewTimer(stewardPulse)
			wardTimer := time.NewTimer(wardPulseInterval * 2)

			for {
				select {
				case <-pulseTimer.C:
					select {
					case heartbeat <- struct{}{}:
					default:
					}
					pulseTimer.Reset(stewardPulse)

				case <-wardHeartbeat:
					//logger.Audit("wardHeartbeat , state: %s", state.StateName())
					wardTimer.Stop()
					wardTimer.Reset(wardPulseInterval * 2)

				case <-wardTimer.C:
					log.GLog.Logger.Error("ward unhealthy; restarting ward",
						"fn", "stewardFn")
					wardCtxCancel()
					startWard()
					wardTimer.Reset(wardPulseInterval * 2)

				case <-ctx.Done():
					wardTimer.Stop()
					log.GLog.Logger.Debug("context done . . .",
						"fn", "stewardFn",
						"error", ctx.Err())
					return
				}
			}
		}()
		return heartbeat
	}
}

func (scheduler OrderScheduler) worker(ctx context.Context, pulseInterval time.Duration,
	scheduleInterval time.Duration) <-chan interface{} {

	log.GLog.Logger.Debug("scheduler start workers . . .",
		"fn", "workers",
		"pulse", pulseInterval,
		"schedule", scheduleInterval)
	var heartbeat = make(chan interface{}, 1)
	go func() {
		defer close(heartbeat)
		pulseTimer := time.NewTimer(pulseInterval)
		scheduleTimer := time.NewTimer(scheduleInterval)
		sendPulse := func() {
			select {
			case heartbeat <- struct{}{}:
			default:
			}
		}

		for {
			select {
			case <-ctx.Done():
				pulseTimer.Stop()
				scheduleTimer.Stop()
				log.GLog.Logger.Debug("context down",
					"fn", "workers",
					"error", ctx.Err())
				return
			case <-pulseTimer.C:
				//logger.Audit("workers() => send pulse, state: %s", state.StateName())
				sendPulse()
				pulseTimer.Reset(pulseInterval)
			case <-scheduleTimer.C:
				//logger.Audit("workers() => schedule, state: %s", state.StateName())
				scheduler.doProcess(ctx)
				scheduleTimer.Reset(scheduleInterval)
			}
		}
	}()
	return heartbeat
}

func (scheduler OrderScheduler) doProcess(ctx context.Context) {
	log.GLog.Logger.Debug("scheduler doProcess",
		"fn", "doProcess")

	iFuture := app.Globals.TriggerRepository.FindByName(ctx, app.Globals.Config.App.FinanceOrderSchedulerTriggerName).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("TriggerRepository.FindByName failed",
			"fn", "doProcess",
			"triggerName", app.Globals.Config.App.FinanceOrderSchedulerTriggerName,
			"error", iFuture.Error().Reason())
		return
	}

	trigger := iFuture.Data().(*entities.SchedulerTrigger)
	if !trigger.IsEnable {
		return
	}

	timestamp := time.Now().UTC()

	// if trigger greater than timestamp then wait for next scheduler
	if trigger.TriggerAt.After(timestamp) {
		return
	}

	trigger.LatestTriggerAt = trigger.TriggerAt

	if trigger.TriggerPointType == entities.AbsoluteTrigger {
		dateTimeString := timestamp.Format("2006-01-02") + "T" + trigger.TriggerPoint + ":00-0000"
		triggerPoint, err := time.Parse(utils.ISO8601, dateTimeString)
		if err != nil {
			log.GLog.Logger.FromContext(ctx).Error("current date time parse failed",
				"fn", "doProcess",
				"timestamp", dateTimeString,
				"error", err)
			triggerPoint = timestamp
		}
		temp := triggerPoint.Add(scheduler.schedulerTriggerInterval)
		trigger.TriggerAt = &temp

	} else {
		temp := timestamp.Add(scheduler.schedulerTriggerInterval)
		trigger.TriggerAt = &temp
	}

	iFuture = app.Globals.TriggerRepository.Update(ctx, *trigger).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("TriggerRepository.Update failed",
			"fn", "doProcess",
			"trigger", trigger,
			"error", iFuture.Error().Reason())
		return
	}

	updatedTrigger := iFuture.Data().(*entities.SchedulerTrigger)

	triggerHistory := entities.TriggerHistory{
		TriggerName:  updatedTrigger.Name,
		ExecResult:   entities.TriggerExecResultNone,
		TriggeredAt:  trigger.LatestTriggerAt,
		IsMissedFire: false,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
		DeletedAt:    nil,
	}

	iFuture = app.Globals.TriggerHistoryRepository.Save(ctx, triggerHistory).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("TriggerHistoryRepository.Update failed",
			"fn", "doProcess",
			"trigger", triggerHistory,
			"error", iFuture.Error().Reason())
		return
	}

	ctx = context.WithValue(ctx, string(utils.CtxTriggerDuration), scheduler.schedulerTriggerDuration)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerInterval), scheduler.schedulerTriggerInterval)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerOffsetPoint), scheduler.schedulerTriggerOffsetPoint)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerPointType), scheduler.schedulerTriggerPointType)
	ctx = context.WithValue(ctx, string(utils.CtxTriggerTimeUnit), scheduler.schedulerTriggerTimeUnit)

	// concurrent or sequential
	OrderSchedulerTask(ctx, triggerHistory).Get()

	log.GLog.Logger.Debug("Current Trigger",
		"fn", "doProcess",
		"trigger", updatedTrigger)
}
