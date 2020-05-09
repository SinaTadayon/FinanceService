package order_scheduler

import (
	"context"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"sync"
	"time"
)

type startWardFn func(ctx context.Context, pulseInterval time.Duration, scheduleInterval time.Duration) (heartbeat <-chan interface{})
type startStewardFn func(ctx context.Context, pulseInterval time.Duration) (heartbeat <-chan interface{})

type OrderScheduler struct {
	schedulerInterval       time.Duration
	schedulerStewardTimeout time.Duration
	schedulerWorkerTimeout  time.Duration
	waitGroup               sync.WaitGroup
	mux                     sync.Mutex
}

func NewOrderScheduler(schedulerInterval, schedulerStewardTimeout, schedulerWorkerTimeout time.Duration) *OrderScheduler {
	return &OrderScheduler{
		schedulerInterval:       schedulerInterval,
		schedulerStewardTimeout: schedulerStewardTimeout,
		schedulerWorkerTimeout:  schedulerWorkerTimeout,
	}
}

func (scheduler OrderScheduler) Scheduler(ctx context.Context) {
	scheduler.waitGroup.Add(1)
	go scheduler.scheduleProcess(ctx)
	scheduler.waitGroup.Wait()
}

func (scheduler OrderScheduler) scheduleProcess(ctx context.Context) {

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

	log.GLog.Logger.Debug("scheduler start worker . . .",
		"fn", "worker",
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
					"fn", "worker",
					"error", ctx.Err())
				return
			case <-pulseTimer.C:
				//logger.Audit("worker() => send pulse, state: %s", state.StateName())
				sendPulse()
				pulseTimer.Reset(pulseInterval)
			case <-scheduleTimer.C:
				//logger.Audit("worker() => schedule, state: %s", state.StateName())
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

}
