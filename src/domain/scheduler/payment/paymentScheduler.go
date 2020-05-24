package payment_scheduler

import (
	"context"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"sync"
	"time"
)

type PaymentState string

const (
	PaymentProcessState  PaymentState = "PAYMENT_PROCESS"
	PaymentTrackingState PaymentState = "PAYMENT_TRACKING"
)

type StateConfig struct {
	State            PaymentState
	ScheduleInterval time.Duration
}

type startWardFn func(ctx context.Context, pulseInterval time.Duration, scheduleInterval time.Duration, state PaymentState) (heartbeat <-chan interface{})
type startStewardFn func(ctx context.Context, pulseInterval time.Duration) (heartbeat <-chan interface{})

type PaymentScheduler struct {
	states                  []StateConfig
	schedulerInterval       time.Duration
	schedulerStewardTimeout time.Duration
	schedulerWorkerTimeout  time.Duration
	waitGroup               sync.WaitGroup
	mux                     sync.Mutex
}

func NewPaymentScheduler(schedulerInterval, schedulerStewardTimeout, schedulerWorkerTimeout time.Duration,
	states ...StateConfig) *PaymentScheduler {
	for i := 0; i < len(states); i++ {
		if states[i].ScheduleInterval == 0 {
			states[i].ScheduleInterval = schedulerInterval
		}
	}
	return &PaymentScheduler{
		schedulerInterval: schedulerInterval, schedulerStewardTimeout: schedulerStewardTimeout,
		schedulerWorkerTimeout: schedulerWorkerTimeout,
		states:                 states}
}

func (scheduler *PaymentScheduler) Scheduler(ctx context.Context) {

	for _, state := range scheduler.states {
		scheduler.waitGroup.Add(1)
		go scheduler.scheduleProcess(ctx, state)
	}
	scheduler.waitGroup.Wait()
}

func (scheduler *PaymentScheduler) scheduleProcess(ctx context.Context, config StateConfig) {

	stewardCtx, stewardCtxCancel := context.WithCancel(context.Background())
	stewardWorkerFn := scheduler.stewardFn(utils.ORContext(ctx, stewardCtx), scheduler.schedulerWorkerTimeout, config.ScheduleInterval, config.State, scheduler.worker)
	heartbeat := stewardWorkerFn(ctx, scheduler.schedulerStewardTimeout)
	stewardTimer := time.NewTimer(scheduler.schedulerStewardTimeout * 2)

	for {
		select {
		case <-ctx.Done():
			log.GLog.Logger.Debug("payment scheduler stewardWorkerFn goroutine context down!",
				"fn", "scheduleProcess",
				"state", config.State)
			stewardTimer.Stop()
			scheduler.waitGroup.Done()
			return
		case _, ok := <-heartbeat:
			if ok == false {
				log.GLog.Logger.Debug("payment scheduler heartbeat of stewardWorkerFn closed",
					"fn", "scheduleProcess",
					"state", config.State)
				stewardCtxCancel()
				stewardCtx, stewardCtxCancel = context.WithCancel(context.Background())
				stewardWorkerFn := scheduler.stewardFn(utils.ORContext(ctx, stewardCtx), scheduler.schedulerWorkerTimeout, config.ScheduleInterval, config.State, scheduler.worker)
				heartbeat = stewardWorkerFn(ctx, scheduler.schedulerStewardTimeout)
				stewardTimer.Reset(scheduler.schedulerStewardTimeout * 2)
			} else {
				//logger.Audit("scheduleProcess() => heartbeat stewardWorkerFn , state: %s", state.StateName())
				stewardTimer.Stop()
				stewardTimer.Reset(scheduler.schedulerStewardTimeout * 2)
			}

		case <-stewardTimer.C:
			log.GLog.Logger.Debug("payment scheduler stewardWorkerFn goroutine is not healthy!",
				"fn", "scheduleProcess",
				"state:", config.State)
			stewardCtxCancel()
			stewardCtx, stewardCtxCancel = context.WithCancel(context.Background())
			stewardWorkerFn := scheduler.stewardFn(utils.ORContext(ctx, stewardCtx), scheduler.schedulerWorkerTimeout, config.ScheduleInterval, config.State, scheduler.worker)
			heartbeat = stewardWorkerFn(ctx, scheduler.schedulerStewardTimeout)
			stewardTimer.Reset(scheduler.schedulerStewardTimeout * 2)
		}
	}
}

func (scheduler *PaymentScheduler) stewardFn(ctx context.Context, wardPulseInterval time.Duration, wardScheduleInterval time.Duration, state PaymentState, startWorker startWardFn) startStewardFn {
	return func(ctx context.Context, stewardPulse time.Duration) <-chan interface{} {
		heartbeat := make(chan interface{}, 1)
		go func() {
			defer close(heartbeat)

			var wardCtx context.Context
			var wardCtxCancel context.CancelFunc
			var wardHeartbeat <-chan interface{}
			startWard := func() {
				wardCtx, wardCtxCancel = context.WithCancel(context.Background())
				wardHeartbeat = startWorker(utils.ORContext(ctx, wardCtx), wardPulseInterval, wardScheduleInterval, state)
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
					log.GLog.Logger.Error("payment scheduler ward unhealthy; restarting ward",
						"fn", "stewardFn",
						"state", state)
					wardCtxCancel()
					startWard()
					wardTimer.Reset(wardPulseInterval * 2)

				case <-ctx.Done():
					wardTimer.Stop()
					log.GLog.Logger.Debug("payment scheduler context done . . .",
						"fn", "stewardFn",
						"state", state, "cause", ctx.Err())
					return
				}
			}
		}()
		return heartbeat
	}
}

func (scheduler *PaymentScheduler) worker(ctx context.Context, pulseInterval time.Duration,
	scheduleInterval time.Duration, state PaymentState) <-chan interface{} {

	log.GLog.Logger.Debug("payment scheduler start worker . . .",
		"fn", "worker",
		"pulse", pulseInterval,
		"schedule", scheduleInterval,
		"state", state)
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
				log.GLog.Logger.Debug("payment scheduler context down",
					"fn", "worker",
					"state", state,
					"cause", ctx.Err())
				return
			case <-pulseTimer.C:
				//logger.Audit("worker() => send pulse, state: %s", state.StateName())
				sendPulse()
				pulseTimer.Reset(pulseInterval)
			case <-scheduleTimer.C:
				//logger.Audit("worker() => schedule, state: %s", state.StateName())
				scheduler.doProcess(ctx, state)
				scheduleTimer.Reset(scheduleInterval)
			}
		}
	}()
	return heartbeat
}

func (scheduler *PaymentScheduler) doProcess(ctx context.Context, state PaymentState) {
	log.GLog.Logger.Debug("payment scheduler doProcess",
		"fn", "doProcess",
		"state", state)

	if state == PaymentProcessState {
		PaymentProcessTask(ctx).Get()
	} else {
		PaymentTrackingTask(ctx).Get()
	}
}
