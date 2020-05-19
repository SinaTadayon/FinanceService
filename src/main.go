package main

import (
	"context"
	"fmt"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	order_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerOrder"
	trigger_repository "gitlab.faza.io/services/finance/domain/model/repository/trigger"
	trigger_history_repository "gitlab.faza.io/services/finance/domain/model/repository/triggerHistory"
	order_scheduler "gitlab.faza.io/services/finance/domain/scheduler/order"
	payment_scheduler "gitlab.faza.io/services/finance/domain/scheduler/payment"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	order_service "gitlab.faza.io/services/finance/infrastructure/services/order"
	payment_service "gitlab.faza.io/services/finance/infrastructure/services/payment"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"gitlab.faza.io/services/finance/infrastructure/workerPool"
	"os"
	"strconv"
	"strings"
	"time"
)

// Build Information variants filled at build time by compiler through flags
var (
	GitCommit string
	GitBranch string
	BuildDate string
)

func buildInfo() string {
	return fmt.Sprintf(`
	======  Finance-Service =======

	Git Commit: %s	
	Build Date: %s
	Git Branch: %s

	======  Finance-Service =======
	`, GitCommit, BuildDate, GitBranch)
}

func main() {
	var err error
	if os.Getenv("APP_MODE") == "dev" {
		app.Globals.Config, err = configs.LoadConfig("./testdata/.env")
	} else {
		app.Globals.Config, err = configs.LoadConfig("")
	}

	log.GLog.ZapLogger = log.InitZap()
	log.GLog.Logger = logger.NewZapLogger(log.GLog.ZapLogger)

	log.GLog.Logger.Info(buildInfo())

	if err != nil {
		log.GLog.Logger.Error("LoadConfig of main init failed",
			"fn", "main", "error", err)
		os.Exit(1)
	}

	mongoDriver, err := app.SetupMongoDriver(*app.Globals.Config)
	if err != nil {
		log.GLog.Logger.Error("main SetupMongoDriver failed", "fn", "main",
			"configs", app.Globals.Config.Mongo, "error", err)
		os.Exit(1)
	}

	app.Globals.SellerFinanceRepository = finance_repository.NewSellerFinanceRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.SellerCollection)
	app.Globals.SellerOrderRepository = order_repository.NewSellerOrderRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.SellerCollection)
	app.Globals.TriggerRepository = trigger_repository.NewSchedulerTriggerRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.TriggerCollection)
	app.Globals.TriggerHistoryRepository = trigger_history_repository.NewTriggerHistoryRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.TriggerHistoryCollection)

	if app.Globals.Config.OrderService.Address == "" ||
		app.Globals.Config.OrderService.Port == 0 {
		log.GLog.Logger.Error("order service address or port invalid", "fn", "main")
		os.Exit(1)
	}

	if app.Globals.Config.PaymentTransferService.Address == "" ||
		app.Globals.Config.PaymentTransferService.Port == 0 {
		log.GLog.Logger.Error("PaymentTransfer service address or port invalid", "fn", "main")
		os.Exit(1)
	}

	app.Globals.OrderService = order_service.NewOrderService(app.Globals.Config.OrderService.Address, app.Globals.Config.OrderService.Port, app.Globals.Config.OrderService.Timeout)
	app.Globals.PaymentService = payment_service.NewPaymentService(app.Globals.Config.PaymentTransferService.Address, app.Globals.Config.PaymentTransferService.Port, app.Globals.Config.PaymentTransferService.Timeout)

	app.Globals.WorkerPool, err = worker_pool.Factory()
	if err != nil {
		log.GLog.Logger.Error("factory of worker pool failed", "fn", "main", "error", err)
		os.Exit(1)
	}

	var OrderSchedulerTimeUnit utils.TimeUnit
	if app.Globals.Config.App.FinanceOrderSchedulerTimeUint == "" {
		app.Globals.Config.App.FinanceOrderSchedulerTimeUint = string(utils.MinuteUnit)
		OrderSchedulerTimeUnit = utils.MinuteUnit
	} else {
		if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTimeUint) == string(utils.HourUnit) {
			OrderSchedulerTimeUnit = utils.HourUnit
		} else if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTimeUint) == string(utils.MinuteUnit) {
			OrderSchedulerTimeUnit = utils.MinuteUnit
		} else {
			log.GLog.Logger.Error("FinanceOrderSchedulerTimeUint invalid",
				"fn", "main",
				"FinanceOrderSchedulerTimeUint", app.Globals.Config.App.FinanceOrderSchedulerTimeUint)
			os.Exit(1)
		}
	}

	var OrderSchedulerInterval time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerInterval <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerInterval invalid",
			"fn", "main",
			"FinanceOrderSchedulerInterval", app.Globals.Config.App.FinanceOrderSchedulerInterval)
		os.Exit(1)
	} else {
		if OrderSchedulerTimeUnit == utils.HourUnit {
			OrderSchedulerInterval = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerInterval) * time.Hour
		} else {
			OrderSchedulerInterval = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerInterval) * time.Minute
		}
	}

	var OrderSchedulerParentWorkerTimeout time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerParentWorkerTimeout <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerParentWorkerTimeout invalid",
			"fn", "main",
			"FinanceOrderSchedulerParentWorkerTimeout", app.Globals.Config.App.FinanceOrderSchedulerParentWorkerTimeout)
		os.Exit(1)
	} else {
		if OrderSchedulerTimeUnit == utils.HourUnit {
			OrderSchedulerParentWorkerTimeout = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerParentWorkerTimeout) * time.Hour
		} else {
			OrderSchedulerParentWorkerTimeout = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerParentWorkerTimeout) * time.Minute
		}
	}

	var OrderSchedulerWorkerTimeout time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerWorkerTimeout <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerWorkerTimeout invalid",
			"fn", "main",
			"FinanceOrderSchedulerWorkerTimeout", app.Globals.Config.App.FinanceOrderSchedulerWorkerTimeout)
		os.Exit(1)
	} else {
		if OrderSchedulerTimeUnit == utils.HourUnit {
			OrderSchedulerWorkerTimeout = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerWorkerTimeout) * time.Hour
		} else {
			OrderSchedulerWorkerTimeout = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerWorkerTimeout) * time.Minute
		}
	}

	var OrderSchedulerTriggerTimeUnit utils.TimeUnit
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit == "" {
		if OrderSchedulerTimeUnit == utils.HourUnit {
			app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit = string(utils.HourUnit)
			OrderSchedulerTriggerTimeUnit = utils.HourUnit
		} else {
			app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit = string(utils.MinuteUnit)
			OrderSchedulerTriggerTimeUnit = utils.MinuteUnit
		}
	} else {
		if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit) == string(utils.HourUnit) {
			OrderSchedulerTriggerTimeUnit = utils.HourUnit
		} else if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit) == string(utils.MinuteUnit) {
			OrderSchedulerTriggerTimeUnit = utils.MinuteUnit
		} else {
			log.GLog.Logger.Error("FinanceOrderSchedulerTriggerTimeUnit invalid",
				"fn", "main",
				"FinanceOrderSchedulerTriggerTimeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
			os.Exit(1)
		}
	}

	if !app.Globals.Config.App.FinanceOrderSchedulerTriggerTestMode && OrderSchedulerTriggerTimeUnit == utils.MinuteUnit {
		log.GLog.Logger.Error("Minute time unit of FinanceOrderSchedulerTriggerTimeUnit is valid in only Test mode",
			"fn", "main",
			"FinanceOrderSchedulerTriggerTimeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
		os.Exit(1)
	}

	if OrderSchedulerTimeUnit == utils.HourUnit && OrderSchedulerTriggerTimeUnit == utils.MinuteUnit {
		log.GLog.Logger.Error("FinanceOrderSchedulerTriggerTimeUnit must be hour time unit",
			"fn", "main",
			"FinanceOrderSchedulerTriggerTimeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
		os.Exit(1)
	}

	var OrderSchedulerTriggerInterval time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerInterval invalid",
			"fn", "main",
			"FinanceOrderSchedulerInterval", app.Globals.Config.App.FinanceOrderSchedulerInterval)
		os.Exit(1)
	} else {
		if OrderSchedulerTriggerTimeUnit == utils.HourUnit {
			if OrderSchedulerTimeUnit == utils.HourUnit {
				if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval {
					log.GLog.Logger.Error("FinanceOrderSchedulerTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
					os.Exit(1)
				}
			} else {
				if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval/60 {
					log.GLog.Logger.Error("FinanceOrderSchedulerTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
					os.Exit(1)
				}
			}

			if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval%24 != 0 {
				log.GLog.Logger.Error("FinanceOrderSchedulerTriggerInterval is not factor 24",
					"fn", "main",
					"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
				os.Exit(1)
			}
			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval) * time.Hour
		} else {
			if OrderSchedulerTimeUnit == utils.HourUnit {
				if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval*60 {
					log.GLog.Logger.Error("FinanceOrderSchedulerTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
					os.Exit(1)
				}
			} else {
				if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval {
					log.GLog.Logger.Error("FinanceOrderSchedulerTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
					os.Exit(1)
				}
			}

			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval) * time.Minute
		}
	}

	var OrderSchedulerTriggerDuration time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerTriggerDuration invalid",
			"fn", "main",
			"FinanceOrderSchedulerTriggerDuration", app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration)
		os.Exit(1)
	} else {
		if app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration < app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval {
			log.GLog.Logger.Error("FinanceOrderSchedulerTriggerDuration less than FinanceOrderSchedulerTriggerInterval",
				"fn", "main",
				"FinanceOrderSchedulerTriggerDuration", app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration)
			os.Exit(1)
		}

		if OrderSchedulerTriggerTimeUnit == utils.HourUnit {
			if app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration%24 != 0 {
				log.GLog.Logger.Error("FinanceOrderSchedulerTriggerDuration is not factor 24",
					"fn", "main",
					"FinanceOrderSchedulerTriggerDuration", app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration)
				os.Exit(1)
			}
			OrderSchedulerTriggerDuration = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerTriggerDuration) * time.Hour
		} else {
			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval) * time.Minute
		}
	}

	if app.Globals.Config.App.FinanceOrderSchedulerTriggerName == "" {
		log.GLog.Logger.Error("FinanceOrderSchedulerTriggerName is empty",
			"fn", "main",
			"FinanceOrderSchedulerTriggerName", app.Globals.Config.App.FinanceOrderSchedulerTriggerName)
		os.Exit(1)
	}

	var triggerPointType entities.TriggerPointType
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType == "" {
		log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPointType is empty",
			"fn", "main",
			"FinanceOrderSchedulerTriggerPointType", app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType)
		os.Exit(1)
	} else {
		if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType) == string(entities.AbsoluteTrigger) {
			triggerPointType = entities.AbsoluteTrigger
		} else if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType) == string(entities.RelativeTrigger) {
			triggerPointType = entities.RelativeTrigger
		} else {
			log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPointType invalid",
				"fn", "main",
				"FinanceOrderSchedulerTriggerPointType", app.Globals.Config.App.FinanceOrderSchedulerTriggerPointType)
			os.Exit(1)
		}
	}

	var triggerPointOffset time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint == "" {
		log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPoint is empty",
			"fn", "main",
			"FinanceOrderSchedulerTriggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
		os.Exit(1)
	} else {
		if triggerPointType == entities.AbsoluteTrigger {
			offset, err := ParseTime(app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			if err != nil {
				log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPoint invalid",
					"fn", "main",
					"FinanceOrderSchedulerTriggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
				os.Exit(1)
			}

			triggerPointOffset = time.Duration(offset) * time.Minute
		} else {
			//offset, err := strconv.Atoi(app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			//if err != nil {
			//	log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPoint invalid",
			//		"fn", "main",
			//		"FinanceOrderSchedulerTriggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			//	os.Exit(1)
			//}
			//
			//if offset > app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval {
			//	log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPoint is greater than FinanceOrderSchedulerTriggerInterval",
			//		"fn", "main",
			//		"FinanceOrderSchedulerTriggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			//	os.Exit(1)
			//}
			//
			//triggerPointOffset = time.Duration(offset) * time.Second
			app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint = "0"
			triggerPointOffset = 0
		}
	}

	orderScheduler := order_scheduler.NewOrderScheduler(OrderSchedulerInterval, OrderSchedulerParentWorkerTimeout,
		OrderSchedulerWorkerTimeout, OrderSchedulerTriggerInterval, triggerPointOffset, OrderSchedulerTriggerDuration, triggerPointType,
		OrderSchedulerTimeUnit, OrderSchedulerTriggerTimeUnit)

	ctx, cancel := context.WithCancel(context.Background())
	if err := orderScheduler.SchedulerStart(ctx); err != nil {
		log.GLog.Logger.Error("OrderScheduler.SchedulerStart failed",
			"fn", "main",
			"error", err)
		os.Exit(1)
	}

	///////////////////////////////////////////////////////////////////////////////////////

	var PaymentSchedulerTimeUnit utils.TimeUnit
	if app.Globals.Config.App.FinancePaymentSchedulerTimeUint == "" {
		app.Globals.Config.App.FinancePaymentSchedulerTimeUint = string(utils.MinuteUnit)
		PaymentSchedulerTimeUnit = utils.MinuteUnit
	} else {
		if strings.ToUpper(app.Globals.Config.App.FinancePaymentSchedulerTimeUint) == string(utils.HourUnit) {
			PaymentSchedulerTimeUnit = utils.HourUnit
		} else if strings.ToUpper(app.Globals.Config.App.FinancePaymentSchedulerTimeUint) == string(utils.MinuteUnit) {
			PaymentSchedulerTimeUnit = utils.MinuteUnit
		} else {
			log.GLog.Logger.Error("FinancePaymentSchedulerTimeUint invalid",
				"fn", "main",
				"FinancePaymentSchedulerTimeUint", app.Globals.Config.App.FinancePaymentSchedulerTimeUint)
			os.Exit(1)
		}
	}

	var PaymentSchedulerInterval time.Duration
	if app.Globals.Config.App.FinancePaymentSchedulerInterval <= 0 {
		log.GLog.Logger.Error("FinancePaymentSchedulerInterval invalid",
			"fn", "main",
			"FinancePaymentSchedulerInterval", app.Globals.Config.App.FinancePaymentSchedulerInterval)
		os.Exit(1)
	} else {
		if PaymentSchedulerTimeUnit == utils.HourUnit {
			PaymentSchedulerInterval = time.Duration(app.Globals.Config.App.FinancePaymentSchedulerInterval) * time.Hour
		} else {
			PaymentSchedulerInterval = time.Duration(app.Globals.Config.App.FinancePaymentSchedulerInterval) * time.Minute
		}
	}

	var PaymentSchedulerParentWorkerTimeout time.Duration
	if app.Globals.Config.App.FinancePaymentSchedulerParentWorkerTimeout <= 0 {
		log.GLog.Logger.Error("FinancePaymentSchedulerParentWorkerTimeout invalid",
			"fn", "main",
			"FinancePaymentSchedulerParentWorkerTimeout", app.Globals.Config.App.FinancePaymentSchedulerParentWorkerTimeout)
		os.Exit(1)
	} else {
		if PaymentSchedulerTimeUnit == utils.HourUnit {
			PaymentSchedulerParentWorkerTimeout = time.Duration(app.Globals.Config.App.FinancePaymentSchedulerParentWorkerTimeout) * time.Hour
		} else {
			PaymentSchedulerParentWorkerTimeout = time.Duration(app.Globals.Config.App.FinancePaymentSchedulerParentWorkerTimeout) * time.Minute
		}
	}

	var PaymentSchedulerWorkerTimeout time.Duration
	if app.Globals.Config.App.FinancePaymentSchedulerWorkerTimeout <= 0 {
		log.GLog.Logger.Error("FinancePaymentSchedulerWorkerTimeout invalid",
			"fn", "main",
			"FinancePaymentSchedulerWorkerTimeout", app.Globals.Config.App.FinancePaymentSchedulerWorkerTimeout)
		os.Exit(1)
	} else {
		if PaymentSchedulerTimeUnit == utils.HourUnit {
			PaymentSchedulerWorkerTimeout = time.Duration(app.Globals.Config.App.FinancePaymentSchedulerWorkerTimeout) * time.Hour
		} else {
			PaymentSchedulerWorkerTimeout = time.Duration(app.Globals.Config.App.FinancePaymentSchedulerWorkerTimeout) * time.Minute
		}
	}

	var stateList = make([]payment_scheduler.StateConfig, 0, 16)
	for _, stateConfig := range strings.Split(app.Globals.Config.App.FinancePaymentSchedulerStates, ";") {
		values := strings.Split(stateConfig, ":")
		if len(values) == 1 {

			var paymentState payment_scheduler.PaymentState
			if values[0] == string(payment_scheduler.PaymentProcessState) {
				paymentState = payment_scheduler.PaymentProcessState
			} else if values[0] == string(payment_scheduler.PaymentTrackingState) {
				paymentState = payment_scheduler.PaymentTrackingState
			} else {
				log.GLog.Logger.Error("FinancePaymentSchedulerStates invalid",
					"fn", "main",
					"FinancePaymentSchedulerStates", app.Globals.Config.App.FinancePaymentSchedulerStates)
				os.Exit(1)
			}

			config := payment_scheduler.StateConfig{
				State:            paymentState,
				ScheduleInterval: 0,
			}
			stateList = append(stateList, config)

		} else if len(values) == 2 {

			temp, err := strconv.Atoi(values[1])
			var scheduleInterval time.Duration
			if err != nil {
				log.GLog.Logger.Error("scheduleInterval of SchedulerStates env is invalid", "fn", "main",
					"state", stateConfig, "error", err)
				os.Exit(1)
			}
			if PaymentSchedulerTimeUnit == utils.HourUnit {
				scheduleInterval = time.Duration(temp) * time.Hour
			} else {
				scheduleInterval = time.Duration(temp) * time.Minute
			}

			var paymentState payment_scheduler.PaymentState
			if values[0] == string(payment_scheduler.PaymentProcessState) {
				paymentState = payment_scheduler.PaymentProcessState
			} else if values[0] == string(payment_scheduler.PaymentTrackingState) {
				paymentState = payment_scheduler.PaymentTrackingState
			} else {
				log.GLog.Logger.Error("FinancePaymentSchedulerStates invalid",
					"fn", "main",
					"FinancePaymentSchedulerStates", app.Globals.Config.App.FinancePaymentSchedulerStates)
				os.Exit(1)
			}

			config := payment_scheduler.StateConfig{
				State:            paymentState,
				ScheduleInterval: scheduleInterval,
			}
			stateList = append(stateList, config)
		} else {
			log.GLog.Logger.Error("state string SchedulerStates env is invalid", "fn", "main",
				"state", stateConfig)
			os.Exit(1)
		}
	}

	paymentScheduler := payment_scheduler.NewPaymentScheduler(
		app.Globals.Config.Mongo.Database,
		app.Globals.Config.Mongo.SellerCollection,
		PaymentSchedulerInterval,
		PaymentSchedulerParentWorkerTimeout,
		PaymentSchedulerWorkerTimeout,
		stateList...)

	paymentScheduler.Scheduler(context.Background())

}

func ParseTime(st string) (int64, error) {
	var h, m int
	n, err := fmt.Sscanf(st, "%d:%d", &h, &m)
	fmt.Print(n, err)
	if err != nil || n != 3 {
		return 0, err
	}
	return int64(h*60 + m), nil
}
