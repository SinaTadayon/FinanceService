package order_scheduler

import (
	"context"
	"errors"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	trigger_history_repository "gitlab.faza.io/services/finance/domain/model/repository/triggerHistory"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"go.mongodb.org/mongo-driver/bson"
	"strings"
	"time"
)

const (
	defaultPerPageSize int64 = 32
)

type startWardFn func(ctx context.Context, pulseInterval time.Duration, scheduleInterval time.Duration) (heartbeat <-chan interface{})
type startStewardFn func(ctx context.Context, pulseInterval time.Duration) (heartbeat <-chan interface{})

type OrderScheduler struct {
	schedulerInterval       time.Duration
	schedulerStewardTimeout time.Duration
	schedulerWorkerTimeout  time.Duration

	financeTriggerOffsetPoint time.Duration
	financeTriggerInterval    time.Duration
	financeTriggerDuration    time.Duration
	financeTriggerPointType   entities.TriggerPointType

	schedulerTimeUnit      utils.TimeUnit
	financeTriggerTimeUnit utils.TimeUnit
}

func NewOrderScheduler(schedulerInterval, schedulerStewardTimeout, schedulerWorkerTimeout,
	financeTriggerInterval, triggerPointOffset, financeTriggerDuration time.Duration,
	financeTriggerPointType entities.TriggerPointType,
	schedulerTimeUnit, financeTriggerTimeUnit utils.TimeUnit) OrderScheduler {

	return OrderScheduler{
		schedulerInterval:         schedulerInterval,
		schedulerStewardTimeout:   schedulerStewardTimeout,
		schedulerWorkerTimeout:    schedulerWorkerTimeout,
		financeTriggerOffsetPoint: triggerPointOffset,
		financeTriggerInterval:    financeTriggerInterval,
		financeTriggerDuration:    financeTriggerDuration,
		financeTriggerPointType:   financeTriggerPointType,
		schedulerTimeUnit:         schedulerTimeUnit,
		financeTriggerTimeUnit:    financeTriggerTimeUnit,
	}
}

func (scheduler OrderScheduler) SchedulerStart(ctx context.Context) {
	go scheduler.scheduleProcess(ctx)
}

func (scheduler OrderScheduler) SchedulerInit(ctx context.Context) error {

	var isTriggerNotFoundFlag = false
	var activeTrigger *entities.FinanceTrigger = nil

	scheduler.abandonedTriggerHistoryHandler(ctx)

	iFuture := app.Globals.TriggerRepository.FindByName(ctx, app.Globals.Config.App.SellerFinanceTriggerName).Get()
	if iFuture.Error() != nil {
		if iFuture.Error().Code() != future.NotFound {
			log.GLog.Logger.Error("TriggerRepository.FindByName failed",
				"fn", "init",
				"error", iFuture.Error().Reason())
			return iFuture.Error()
		} else {
			isTriggerNotFoundFlag = true
		}
	}

	// create new seller trigger
	if isTriggerNotFoundFlag {
		iFuture := app.Globals.TriggerRepository.FindActiveTrigger(ctx, entities.SellerTrigger).Get()
		if iFuture.Error() != nil {
			if iFuture.Error().Code() != future.NotFound {
				log.GLog.Logger.Error("TriggerRepository.FindActiveTrigger failed",
					"fn", "init",
					"error", iFuture.Error())
				return iFuture.Error()
			}
		} else {
			activeTrigger = iFuture.Data().(*entities.FinanceTrigger)
			activeTrigger.IsActive = false
			activeTrigger.IsEnable = false
			iFuture = app.Globals.TriggerRepository.Update(ctx, *activeTrigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerRepository.Update seller active trigger failed",
					"fn", "init",
					"trigger", activeTrigger,
					"error", iFuture.Error())
				return iFuture.Error()
			}

			log.GLog.Logger.Info("disable seller active trigger success",
				"fn", "init",
				"activeTrigger", activeTrigger)

		}

		var triggerAt time.Time
		dt := time.Now().UTC()
		if scheduler.financeTriggerPointType == entities.AbsoluteTrigger {
			dateTimeString := dt.Format("2006-01-02") + "T" + app.Globals.Config.App.SellerFinanceTriggerPoint + ":00-0000"
			triggerPoint, err := time.Parse(utils.ISO8601, dateTimeString)
			if err != nil {
				log.GLog.Logger.FromContext(ctx).Error("current date time parse failed",
					"fn", "init",
					"timestamp", dateTimeString,
					"error", err)
				triggerPoint = dt
			}

			if dt.After(triggerPoint) {
				triggerAt = triggerPoint.Add(24 * time.Hour)
			} else if dt.Before(triggerPoint) || dt.Equal(triggerPoint) {
				triggerAt = triggerPoint
			}
		} else {
			triggerAt = dt.Add(scheduler.financeTriggerInterval)
		}

		newTrigger := &entities.FinanceTrigger{
			Name:             app.Globals.Config.App.SellerFinanceTriggerName,
			Duration:         int64(app.Globals.Config.App.SellerFinanceTriggerDuration),
			Interval:         int64(app.Globals.Config.App.SellerFinanceTriggerInterval),
			TimeUnit:         utils.TimeUnit(strings.ToUpper(app.Globals.Config.App.SellerFinanceTriggerTimeUnit)),
			TriggerPoint:     app.Globals.Config.App.SellerFinanceTriggerPoint,
			TriggerPointType: entities.TriggerPointType(strings.ToUpper(app.Globals.Config.App.SellerFinanceTriggerPointType)),
			TriggerAt:        &triggerAt,
			Type:             entities.SellerTrigger,
			IsActive:         true,
			IsEnable:         app.Globals.Config.App.SellerFinanceTriggerEnabled,
			TestMode:         app.Globals.Config.App.SellerFinanceTriggerTestMode,
			ExecMode:         entities.SequentialTrigger,
			JobExecType:      entities.TriggerSyncJob,
			CreatedAt:        dt,
			UpdatedAt:        dt,
		}

		iFuture = app.Globals.TriggerRepository.Save(ctx, *newTrigger).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerRepository.Save new trigger failed",
				"fn", "init",
				"trigger", newTrigger,
				"error", iFuture.Error())
			return iFuture.Error()
		}

		newTrigger = iFuture.Data().(*entities.FinanceTrigger)
		if app.Globals.Config.App.FinanceOrderSchedulerUpdateFinanceDuration && activeTrigger != nil {
			if err := scheduler.updateSellerFinanceWithNewTriggerConfig(ctx, activeTrigger, newTrigger); err != nil {
				log.GLog.Logger.Error("updateSellerFinance With NewTriggerConfig failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"trigger", newTrigger,
					"error", iFuture.Error())
				return err
			} else {
				log.GLog.Logger.Info("updateSellerFinance With NewTriggerConfig success",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"trigger", newTrigger)
			}
		}

		if app.Globals.Config.App.FinanceOrderSchedulerHandleMissedFireTrigger && activeTrigger != nil {
			if missedCount, err := scheduler.missedFireTriggerHandler(ctx, activeTrigger); err != nil {
				log.GLog.Logger.Error("missedFireTriggerHandler failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"error", iFuture.Error())
				return err
			} else {
				log.GLog.Logger.Info("missedFireTriggerHandler success",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"missedCount", missedCount)
			}
		}

	} else {
		activeTrigger := iFuture.Data().(*entities.FinanceTrigger)
		if !activeTrigger.IsActive {
			log.GLog.Logger.Error("the trigger is not active trigger",
				"fn", "init",
				"name", activeTrigger.Name)
			return errors.New("trigger is not active trigger")
		}

		if activeTrigger.TimeUnit != scheduler.financeTriggerTimeUnit {
			log.GLog.Logger.Error("the activeTrigger exist and SellerFinanceTriggerTimeUnit modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old timeUnit", activeTrigger.TimeUnit,
				"new timeUnit", app.Globals.Config.App.SellerFinanceTriggerTimeUnit)
			return errors.New("unit time change not acceptable")

		}

		if activeTrigger.Interval != int64(app.Globals.Config.App.SellerFinanceTriggerInterval) {
			log.GLog.Logger.Error("the activeTrigger exist and SellerFinanceTriggerInterval modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old interval", activeTrigger.Interval,
				"new interval", app.Globals.Config.App.SellerFinanceTriggerInterval)
			return errors.New("interval change not acceptable")

		}

		if activeTrigger.TriggerPoint != app.Globals.Config.App.SellerFinanceTriggerPoint {
			log.GLog.Logger.Error("the activeTrigger exist and SellerFinanceTriggerPoint modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old triggerPoint", activeTrigger.TriggerPoint,
				"new triggerPoint", app.Globals.Config.App.SellerFinanceTriggerPoint)
			return errors.New("triggerPoint change not acceptable")

		}

		if activeTrigger.TriggerPointType != scheduler.financeTriggerPointType {
			log.GLog.Logger.Error("the activeTrigger exist and SellerFinanceTriggerPointType modification is not allow, you must create new activeTrigger with new name",
				"fn", "init",
				"name", activeTrigger.Name,
				"old triggerPointType", activeTrigger.TriggerPointType,
				"new triggerPointType", app.Globals.Config.App.SellerFinanceTriggerPointType)
			return errors.New("triggerPoint change not acceptable")
		}

		if activeTrigger.IsEnable != app.Globals.Config.App.SellerFinanceTriggerEnabled {
			activeTrigger.IsEnable = app.Globals.Config.App.SellerFinanceTriggerEnabled
			iFuture = app.Globals.TriggerRepository.Update(ctx, *activeTrigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerRepository.Update failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"error", iFuture.Error().Reason())
				return iFuture.Error()
			}
		}

		if app.Globals.Config.App.FinanceOrderSchedulerHandleMissedFireTrigger {
			if missedCount, err := scheduler.missedFireTriggerHandler(ctx, activeTrigger); err != nil {
				log.GLog.Logger.Error("missedFireTriggerHandler failed",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"error", iFuture.Error())
				return err
			} else {
				log.GLog.Logger.Info("missedFireTriggerHandler success",
					"fn", "init",
					"activeTrigger", activeTrigger,
					"missedCount", missedCount)
			}
		}
	}

	return nil
}

func (scheduler OrderScheduler) abandonedTriggerHistoryHandler(ctx context.Context) {
	var availablePages = 1
	for i := 0; i < availablePages; i++ {
		iFuture := app.Globals.TriggerHistoryRepository.FindByFilterWithPage(ctx, func() interface{} {
			return bson.D{{"runMode", entities.TriggerRunModeRunning},
				{"deletedAt", nil}}
		}, int64(i+1), defaultPerPageSize).Get()

		if iFuture.Error() != nil {
			if iFuture.Error().Code() != future.NotFound {
				log.GLog.Logger.Error("TriggerHistoryRepository.FindByFilterWithPage failed",
					"fn", "abandonedTriggerHistoryHandler",
					"error", iFuture.Error())
				return
			}
			log.GLog.Logger.Info("abandoned Trigger History not found",
				"fn", "abandonedTriggerHistoryHandler")
			return
		}

		historyResult := iFuture.Data().(trigger_history_repository.HistoryPageableResult)

		if historyResult.TotalCount%defaultPerPageSize != 0 {
			availablePages = (int(historyResult.TotalCount) / int(defaultPerPageSize)) + 1
		} else {
			availablePages = int(historyResult.TotalCount) / int(defaultPerPageSize)
		}

		if historyResult.TotalCount < defaultPerPageSize {
			availablePages = 1
		}

		for _, history := range historyResult.Histories {
			history.RunMode = entities.TriggerRunModeIncomplete
			history.ExecResult = entities.TriggerExecResultFail

			iFuture = app.Globals.TriggerHistoryRepository.Update(ctx, *history).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerHistoryRepository.Update for abandoned trigger history failed",
					"fn", "abandonedTriggerHistoryHandler",
					"triggerHistory", history,
					"error", iFuture.Error())
			} else {
				log.GLog.Logger.Info("Updating abandoned trigger history success",
					"fn", "abandonedTriggerHistoryHandler",
					"triggerHistory", history)
			}
		}
	}
}

// TODO performance optimization required
func (scheduler OrderScheduler) updateSellerFinanceWithNewTriggerConfig(ctx context.Context, activeTrigger, newTrigger *entities.FinanceTrigger) error {

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
	if newDuration > 0 {
		log.GLog.Logger.Info("finance Duration must be shrink according to new trigger",
			"oldDuration", activeTrigger.Duration,
			"newDuration", newTrigger.Duration,
			"diffDuration", newDuration)

		var availablePages = 1
		for i := 0; i < availablePages; i++ {
			iFuture := app.Globals.SellerFinanceRepository.FindByFilterWithPage(ctx, func() interface{} {
				return bson.D{{"ordersInfo.triggerName", activeTrigger.Name},
					{"endAt", bson.D{{"$gt", timestamp}}},
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
				log.GLog.Logger.Info("seller finance for active trigger not found",
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
					log.GLog.Logger.Error("shrink of finance's endAt failed",
						"fn", "updateSellerFinanceWithNewTriggerConfig",
						"fid", finance.FId,
						"startAt", finance.StartAt.Format(utils.ISO8601),
						"endAt", finance.EndAt.Format(utils.ISO8601),
						"error", iFuture.Error().Reason())
					return iFuture.Error()
				}

				log.GLog.Logger.Debug("shrink of finance's endAt success",
					"fn", "updateSellerFinanceWithNewTriggerConfig",
					"fid", finance.FId,
					"startAt", finance.StartAt.Format(utils.ISO8601),
					"endAt", finance.EndAt.Format(utils.ISO8601))
			}
		}
	} else {
		newDuration = -newDuration
		log.GLog.Logger.Info("finance Duration should be gross in next duration according to new trigger",
			"oldDuration", activeTrigger.Duration,
			"newDuration", newTrigger.Duration,
			"diffDuration", newDuration)
	}

	return nil
}

func (scheduler OrderScheduler) missedFireTriggerHandler(ctx context.Context, activeTrigger *entities.FinanceTrigger) (int, error) {

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
		totalIntervalCount := int(timeOffset / activeInterval)

		iFuture := app.Globals.TriggerHistoryRepository.CountWithFilter(ctx, func() interface{} {
			return bson.D{{"triggerName", activeTrigger.Name}}
		}).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerHistoryRepository.CountWithFilter failed",
				"fn", "missedFireTriggerHandler",
				"activeTrigger", activeTrigger,
				"error", iFuture.Error())
			return missedFireCount, iFuture.Error()
		}

		triggersCount := iFuture.Data().(int64)
		if int64(totalIntervalCount) > triggersCount {
			log.GLog.Logger.Info("missed fire trigger history detected",
				"fn", "missedFireTriggerHandler",
				"triggerHistoryCount", triggersCount,
				"totalIntervalCount", totalIntervalCount)

			var newTriggeredAt, latestTriggerAt time.Time
			for newTriggeredAt = activeTrigger.CreatedAt.Add(activeInterval); timestamp.After(newTriggeredAt); newTriggeredAt = newTriggeredAt.Add(activeInterval) {

				latestTriggerAt = newTriggeredAt
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
						RunMode:      entities.TriggerRunModeNone,
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

			activeTrigger.LatestTriggerAt = &latestTriggerAt
			activeTrigger.TriggerAt = &newTriggeredAt
			iFuture = app.Globals.TriggerRepository.Update(ctx, *activeTrigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("Update active trigger failed",
					"fn", "missedFireTriggerHandler",
					"trigger", activeTrigger,
					"error", iFuture.Error())
				return missedFireCount, iFuture.Error()
			}

		} else if int64(totalIntervalCount) == triggersCount {
			log.GLog.Logger.Info("missed fire trigger not found",
				"fn", "missedFireTriggerHandler",
				"triggerHistoryCount", triggersCount,
				"totalIntervalCount", totalIntervalCount)
		} else {
			log.GLog.Logger.Error("fatal error in missed fire trigger calculation",
				"fn", "missedFireTriggerHandler",
				"triggerHistoryCount", triggersCount,
				"totalIntervalCount", totalIntervalCount)
			return missedFireCount, errors.New("missed fire trigger calculation failed")
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
			log.GLog.Logger.Debug("order scheduler stewardWorkerFn goroutine context down!",
				"fn", "scheduleProcess")
			stewardTimer.Stop()
			return
		case _, ok := <-heartbeat:
			if ok == false {
				log.GLog.Logger.Debug("order scheduler heartbeat of stewardWorkerFn closed",
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
			log.GLog.Logger.Debug("order scheduler stewardWorkerFn goroutine is not healthy!",
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
					log.GLog.Logger.Error("order scheduler ward unhealthy; restarting ward",
						"fn", "stewardFn")
					wardCtxCancel()
					startWard()
					wardTimer.Reset(wardPulseInterval * 2)

				case <-ctx.Done():
					wardTimer.Stop()
					log.GLog.Logger.Debug("order scheduler context done . . .",
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

	log.GLog.Logger.Debug("order scheduler start workers . . .",
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
				log.GLog.Logger.Debug("order scheduler context down",
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
	log.GLog.Logger.Debug("order scheduler doProcess",
		"fn", "doProcess")

	iFuture := app.Globals.TriggerRepository.FindByName(ctx, app.Globals.Config.App.SellerFinanceTriggerName).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("TriggerRepository.FindByName failed",
			"fn", "doProcess",
			"triggerName", app.Globals.Config.App.SellerFinanceTriggerName,
			"error", iFuture.Error())
		return
	}

	trigger := iFuture.Data().(*entities.FinanceTrigger)
	if !trigger.IsEnable {
		return
	}

	timestamp := time.Now().UTC()

	// if trigger greater than timestamp then wait for next scheduler
	if trigger.TriggerAt.After(timestamp) {
		log.GLog.Logger.Debug("triggerAt greater than timestamp",
			"fn", "doProcess",
			"triggerName", trigger.Name,
			"triggerAt", trigger.TriggerAt.Format(utils.ISO8601))
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
		temp := triggerPoint.Add(scheduler.financeTriggerInterval)
		trigger.TriggerAt = &temp

	} else {
		temp := timestamp.Add(scheduler.financeTriggerInterval)
		trigger.TriggerAt = &temp
	}

	iFuture = app.Globals.TriggerRepository.Update(ctx, *trigger).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("TriggerRepository.Update failed",
			"fn", "doProcess",
			"trigger", trigger,
			"error", iFuture.Error())
		return
	}

	// TODO implement automatic retry trigger
	triggerHistory := &entities.TriggerHistory{
		TriggerName:   trigger.Name,
		ExecResult:    entities.TriggerExecResultNone,
		RunMode:       entities.TriggerRunModeRunning,
		TriggeredAt:   trigger.LatestTriggerAt,
		IsMissedFire:  false,
		ActionHistory: nil,
		RetryIndex:    1,
		Finances:      nil,
		CreatedAt:     timestamp,
		UpdatedAt:     timestamp,
		DeletedAt:     nil,
	}

	iFuture = app.Globals.TriggerHistoryRepository.Save(ctx, *triggerHistory).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("TriggerHistoryRepository.Update failed",
			"fn", "doProcess",
			"trigger", triggerHistory,
			"error", iFuture.Error().Reason())
		return
	}

	triggerHistory = iFuture.Data().(*entities.TriggerHistory)

	// concurrent or sequential
	scheduler.OrderSchedulerTask(ctx, *triggerHistory).Get()
}
