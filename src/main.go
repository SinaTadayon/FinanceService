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
	user_service "gitlab.faza.io/services/finance/infrastructure/services/user"
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
	app.Globals.TriggerRepository = trigger_repository.NewSchedulerTriggerRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.FinanceTriggerCollection)
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
	app.Globals.UserService = user_service.NewUserService(app.Globals.Config.UserService.Address, app.Globals.Config.UserService.Port, app.Globals.Config.UserService.Timeout)

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
	if app.Globals.Config.App.SellerFinanceTriggerTimeUnit == "" {
		if OrderSchedulerTimeUnit == utils.HourUnit {
			app.Globals.Config.App.SellerFinanceTriggerTimeUnit = string(utils.HourUnit)
			OrderSchedulerTriggerTimeUnit = utils.HourUnit
		} else {
			app.Globals.Config.App.SellerFinanceTriggerTimeUnit = string(utils.MinuteUnit)
			OrderSchedulerTriggerTimeUnit = utils.MinuteUnit
		}
	} else {
		if strings.ToUpper(app.Globals.Config.App.SellerFinanceTriggerTimeUnit) == string(utils.HourUnit) {
			OrderSchedulerTriggerTimeUnit = utils.HourUnit
		} else if strings.ToUpper(app.Globals.Config.App.SellerFinanceTriggerTimeUnit) == string(utils.MinuteUnit) {
			OrderSchedulerTriggerTimeUnit = utils.MinuteUnit
		} else {
			log.GLog.Logger.Error("SellerFinanceTriggerTimeUnit invalid",
				"fn", "main",
				"SellerFinanceTriggerTimeUnit", app.Globals.Config.App.SellerFinanceTriggerTimeUnit)
			os.Exit(1)
		}
	}

	if !app.Globals.Config.App.SellerFinanceTriggerTestMode && OrderSchedulerTriggerTimeUnit == utils.MinuteUnit {
		log.GLog.Logger.Error("Minute time unit of SellerFinanceTriggerTimeUnit is valid in only Test mode",
			"fn", "main",
			"SellerFinanceTriggerTimeUnit", app.Globals.Config.App.SellerFinanceTriggerTimeUnit)
		os.Exit(1)
	}

	if OrderSchedulerTimeUnit == utils.HourUnit && OrderSchedulerTriggerTimeUnit == utils.MinuteUnit {
		log.GLog.Logger.Error("SellerFinanceTriggerTimeUnit must be hour time unit",
			"fn", "main",
			"SellerFinanceTriggerTimeUnit", app.Globals.Config.App.SellerFinanceTriggerTimeUnit)
		os.Exit(1)
	}

	var OrderSchedulerTriggerInterval time.Duration
	if app.Globals.Config.App.SellerFinanceTriggerInterval <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerInterval invalid",
			"fn", "main",
			"FinanceOrderSchedulerInterval", app.Globals.Config.App.FinanceOrderSchedulerInterval)
		os.Exit(1)
	} else {
		if OrderSchedulerTriggerTimeUnit == utils.HourUnit {
			if OrderSchedulerTimeUnit == utils.HourUnit {
				if app.Globals.Config.App.SellerFinanceTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval {
					log.GLog.Logger.Error("SellerFinanceTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"SellerFinanceTriggerInterval", app.Globals.Config.App.SellerFinanceTriggerInterval)
					os.Exit(1)
				}
			} else {
				if app.Globals.Config.App.SellerFinanceTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval/60 {
					log.GLog.Logger.Error("SellerFinanceTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"SellerFinanceTriggerInterval", app.Globals.Config.App.SellerFinanceTriggerInterval)
					os.Exit(1)
				}
			}

			if app.Globals.Config.App.SellerFinanceTriggerInterval%24 != 0 {
				log.GLog.Logger.Error("SellerFinanceTriggerInterval is not factor 24",
					"fn", "main",
					"SellerFinanceTriggerInterval", app.Globals.Config.App.SellerFinanceTriggerInterval)
				os.Exit(1)
			}
			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.SellerFinanceTriggerInterval) * time.Hour
		} else {
			if OrderSchedulerTimeUnit == utils.HourUnit {
				if app.Globals.Config.App.SellerFinanceTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval*60 {
					log.GLog.Logger.Error("SellerFinanceTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"SellerFinanceTriggerInterval", app.Globals.Config.App.SellerFinanceTriggerInterval)
					os.Exit(1)
				}
			} else {
				if app.Globals.Config.App.SellerFinanceTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval {
					log.GLog.Logger.Error("SellerFinanceTriggerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"SellerFinanceTriggerInterval", app.Globals.Config.App.SellerFinanceTriggerInterval)
					os.Exit(1)
				}
			}

			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.SellerFinanceTriggerInterval) * time.Minute
		}
	}

	var OrderSchedulerTriggerDuration time.Duration
	if app.Globals.Config.App.SellerFinanceTriggerDuration <= 0 {
		log.GLog.Logger.Error("SellerFinanceTriggerDuration invalid",
			"fn", "main",
			"SellerFinanceTriggerDuration", app.Globals.Config.App.SellerFinanceTriggerDuration)
		os.Exit(1)
	} else {
		if app.Globals.Config.App.SellerFinanceTriggerDuration < app.Globals.Config.App.SellerFinanceTriggerInterval {
			log.GLog.Logger.Error("SellerFinanceTriggerDuration less than SellerFinanceTriggerInterval",
				"fn", "main",
				"SellerFinanceTriggerDuration", app.Globals.Config.App.SellerFinanceTriggerDuration)
			os.Exit(1)
		}

		if OrderSchedulerTriggerTimeUnit == utils.HourUnit {
			if app.Globals.Config.App.SellerFinanceTriggerDuration%24 != 0 {
				log.GLog.Logger.Error("SellerFinanceTriggerDuration is not factor 24",
					"fn", "main",
					"SellerFinanceTriggerDuration", app.Globals.Config.App.SellerFinanceTriggerDuration)
				os.Exit(1)
			}
			OrderSchedulerTriggerDuration = time.Duration(app.Globals.Config.App.SellerFinanceTriggerDuration) * time.Hour
		} else {
			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.SellerFinanceTriggerInterval) * time.Minute
		}
	}

	if app.Globals.Config.App.SellerFinanceTriggerName == "" {
		log.GLog.Logger.Error("SellerFinanceTriggerName is empty",
			"fn", "main",
			"SellerFinanceTriggerName", app.Globals.Config.App.SellerFinanceTriggerName)
		os.Exit(1)
	}

	var triggerPointType entities.TriggerPointType
	if app.Globals.Config.App.SellerFinanceTriggerPointType == "" {
		log.GLog.Logger.Error("SellerFinanceTriggerPointType is empty",
			"fn", "main",
			"SellerFinanceTriggerPointType", app.Globals.Config.App.SellerFinanceTriggerPointType)
		os.Exit(1)
	} else {
		if strings.ToUpper(app.Globals.Config.App.SellerFinanceTriggerPointType) == string(entities.AbsoluteTrigger) {
			triggerPointType = entities.AbsoluteTrigger
		} else if strings.ToUpper(app.Globals.Config.App.SellerFinanceTriggerPointType) == string(entities.RelativeTrigger) {
			triggerPointType = entities.RelativeTrigger
		} else {
			log.GLog.Logger.Error("SellerFinanceTriggerPointType invalid",
				"fn", "main",
				"SellerFinanceTriggerPointType", app.Globals.Config.App.SellerFinanceTriggerPointType)
			os.Exit(1)
		}
	}

	var triggerPointOffset time.Duration
	if app.Globals.Config.App.SellerFinanceTriggerPoint == "" {
		log.GLog.Logger.Error("SellerFinanceTriggerPoint is empty",
			"fn", "main",
			"SellerFinanceTriggerPoint", app.Globals.Config.App.SellerFinanceTriggerPoint)
		os.Exit(1)
	} else {
		if triggerPointType == entities.AbsoluteTrigger {
			offset, err := ParseTime(app.Globals.Config.App.SellerFinanceTriggerPoint)
			if err != nil {
				log.GLog.Logger.Error("SellerFinanceTriggerPoint invalid",
					"fn", "main",
					"SellerFinanceTriggerPoint", app.Globals.Config.App.SellerFinanceTriggerPoint)
				os.Exit(1)
			}

			triggerPointOffset = time.Duration(offset) * time.Minute
		} else {
			//offset, err := strconv.Atoi(app.Globals.Config.App.SellerFinanceTriggerPoint)
			//if err != nil {
			//	log.GLog.Logger.Error("SellerFinanceTriggerPoint invalid",
			//		"fn", "main",
			//		"SellerFinanceTriggerPoint", app.Globals.Config.App.SellerFinanceTriggerPoint)
			//	os.Exit(1)
			//}
			//
			//if offset > app.Globals.Config.App.SellerFinanceTriggerInterval {
			//	log.GLog.Logger.Error("SellerFinanceTriggerPoint is greater than SellerFinanceTriggerInterval",
			//		"fn", "main",
			//		"SellerFinanceTriggerPoint", app.Globals.Config.App.SellerFinanceTriggerPoint)
			//	os.Exit(1)
			//}
			//
			//triggerPointOffset = time.Duration(offset) * time.Second
			app.Globals.Config.App.SellerFinanceTriggerPoint = "0"
			triggerPointOffset = 0
		}
	}

	orderScheduler := order_scheduler.NewOrderScheduler(OrderSchedulerInterval, OrderSchedulerParentWorkerTimeout,
		OrderSchedulerWorkerTimeout, OrderSchedulerTriggerInterval, triggerPointOffset, OrderSchedulerTriggerDuration, triggerPointType,
		OrderSchedulerTimeUnit, OrderSchedulerTriggerTimeUnit)

	ctx, _ := context.WithCancel(context.Background())
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
