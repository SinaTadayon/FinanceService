package order_scheduler

import (
	"context"
	"errors"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
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
		iFuture := app.Globals.TriggerRepository.FindActiveTrigger(ctx).Get()
		if iFuture.Error() != nil {
			if iFuture.Error().Code() != future.NotFound {
				log.GLog.Logger.Error("TriggerRepository.FindActiveTrigger failed",
					"fn", "init",
					"error", iFuture.Error().Reason())
				return iFuture.Error()
			}
		} else {
			activeTrigger := iFuture.Data().(*entities.SchedulerTrigger)
			iFuture = app.Globals.TriggerRepository.Delete(ctx, *activeTrigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerRepository.Delete active trigger failed",
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

		newTrigger := entities.SchedulerTrigger{
			Version:          1,
			DocVersion:       entities.TriggerDocumentVersion,
			Name:             app.Globals.Config.App.FinanceOrderSchedulerTriggerName,
			Duration:         int64(app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration),
			Interval:         int64(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval),
			TimeUnit:         utils.TimeUnit(strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)),
			TriggerPoint:     app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint,
			TriggerPointType: entities.TriggerPointType(strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType)),
			TriggerAt:        &triggerAt,
			IsEnabled:        app.Globals.Config.App.FinanceOrderSchedulerTriggerEnabled,
			CreatedAt:        dt,
			UpdatedAt:        dt,
		}

		iFuture = app.Globals.TriggerRepository.Save(ctx, newTrigger).Get()
		if iFuture.Error() != nil {
			log.GLog.Logger.Error("TriggerRepository.Save new trigger failed",
				"fn", "init",
				"trigger", newTrigger,
				"error", iFuture.Error().Reason())
			return iFuture.Error()
		}

	} else {
		trigger := iFuture.Data().(*entities.SchedulerTrigger)
		if trigger.TimeUnit != scheduler.schedulerTriggerTimeUnit {
			log.GLog.Logger.Error("the trigger exist and FinanceOrderSchedulerTriggerTimeUnit modification is not allow, you must create new trigger with new name",
				"fn", "init",
				"name", trigger.Name,
				"old timeUnit", trigger.TimeUnit,
				"new timeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
			return errors.New("unit time change not acceptable")

		} else if trigger.Interval != int64(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval) {
			log.GLog.Logger.Error("the trigger exist and FinanceOrderSchedulerTriggerInterval modification is not allow, you must create new trigger with new name",
				"fn", "init",
				"name", trigger.Name,
				"old interval", trigger.Interval,
				"new interval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
			return errors.New("interval change not acceptable")

		} else if trigger.TriggerPoint != app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint {
			log.GLog.Logger.Error("the trigger exist and FinanceOrderSchedulerTriggerPoint modification is not allow, you must create new trigger with new name",
				"fn", "init",
				"name", trigger.Name,
				"old triggerPoint", trigger.TriggerPoint,
				"new triggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			return errors.New("triggerPoint change not acceptable")

		} else if trigger.TriggerPointType != scheduler.schedulerTriggerPointType {
			log.GLog.Logger.Error("the trigger exist and FinanceOrderSchedulerTriggerPointType modification is not allow, you must create new trigger with new name",
				"fn", "init",
				"name", trigger.Name,
				"old triggerPointType", trigger.TriggerPointType,
				"new triggerPointType", app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType)
			return errors.New("triggerPoint change not acceptable")
		}

		if trigger.IsEnabled != app.Globals.Config.App.FinanceOrderSchedulerTriggerEnabled {
			trigger.IsEnabled = app.Globals.Config.App.FinanceOrderSchedulerTriggerEnabled
			iFuture = app.Globals.TriggerRepository.Update(ctx, *trigger).Get()
			if iFuture.Error() != nil {
				log.GLog.Logger.Error("TriggerRepository.Update failed",
					"fn", "init",
					"trigger", trigger,
					"error", iFuture.Error().Reason())
				return iFuture.Error()
			}
		}

	}

	return nil
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
	if !trigger.IsEnabled {
		return
	}

	timestamp := time.Now().UTC()

	// if trigger greater than timestamp then wait for next scheduler
	if trigger.TriggerAt.After(timestamp) {
		return
	}

	trigger.LatestTriggerAt = trigger.TriggerAt
	trigger.TriggerCount += 1

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
