package main

import (
	"fmt"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	order_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerOrder"
	trigger_repository "gitlab.faza.io/services/finance/domain/model/repository/trigger"
	order_scheduler "gitlab.faza.io/services/finance/domain/scheduler/order"
	"gitlab.faza.io/services/finance/infrastructure/logger"
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
	}

	app.Globals.SellerFinanceRepository = finance_repository.NewSellerFinanceRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.SellerCollection)
	app.Globals.SellerOrderRepository = order_repository.NewSellerOrderRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.SellerCollection)
	app.Globals.TriggerRepository = trigger_repository.NewSchedulerTriggerRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.TriggerCollection)

	var OrderSchedulerTimeUint app.TimeUnit
	if app.Globals.Config.App.FinanceOrderSchedulerTimeUint == "" {
		app.Globals.Config.App.FinanceOrderSchedulerTimeUint = string(app.MinuteUnit)
		OrderSchedulerTimeUint = app.MinuteUnit
	} else {
		if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTimeUint) == string(app.HourUnit) {
			OrderSchedulerTimeUint = app.HourUnit
		} else if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTimeUint) == string(app.MinuteUnit) {
			OrderSchedulerTimeUint = app.MinuteUnit
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
		if OrderSchedulerTimeUint == app.HourUnit {
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
		if OrderSchedulerTimeUint == app.HourUnit {
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
		if OrderSchedulerTimeUint == app.HourUnit {
			OrderSchedulerWorkerTimeout = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerWorkerTimeout) * time.Hour
		} else {
			OrderSchedulerWorkerTimeout = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerWorkerTimeout) * time.Minute
		}
	}

	var OrderSchedulerTriggerTimeUnit app.TimeUnit
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit == "" {
		if OrderSchedulerTimeUint == app.HourUnit {
			app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit = string(app.HourUnit)
			OrderSchedulerTriggerTimeUnit = app.HourUnit
		} else {
			app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit = string(app.MinuteUnit)
			OrderSchedulerTriggerTimeUnit = app.MinuteUnit
		}
	} else {
		if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit) == string(app.HourUnit) {
			OrderSchedulerTriggerTimeUnit = app.HourUnit
		} else if strings.ToUpper(app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit) == string(app.MinuteUnit) {
			OrderSchedulerTriggerTimeUnit = app.MinuteUnit
		} else {
			log.GLog.Logger.Error("FinanceOrderSchedulerTriggerTimeUnit invalid",
				"fn", "main",
				"FinanceOrderSchedulerTriggerTimeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
			os.Exit(1)
		}

		if !app.Globals.Config.App.FinanceOrderSchedulerTriggerTestMode && OrderSchedulerTriggerTimeUnit == app.MinuteUnit {
			log.GLog.Logger.Error("Minute time unit of FinanceOrderSchedulerTriggerTimeUnit is valid in only Test mode",
				"fn", "main",
				"FinanceOrderSchedulerTriggerTimeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
			os.Exit(1)
		}

		if OrderSchedulerTimeUint == app.HourUnit && OrderSchedulerTriggerTimeUnit == app.MinuteUnit {
			log.GLog.Logger.Error("FinanceOrderSchedulerTriggerTimeUnit must be hour time unit",
				"fn", "main",
				"FinanceOrderSchedulerTriggerTimeUnit", app.Globals.Config.App.FinanceOrderSchedulerTriggerTimeUnit)
			os.Exit(1)
		}
	}

	var OrderSchedulerTriggerInterval time.Duration
	if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval <= 0 {
		log.GLog.Logger.Error("FinanceOrderSchedulerInterval invalid",
			"fn", "main",
			"FinanceOrderSchedulerInterval", app.Globals.Config.App.FinanceOrderSchedulerInterval)
		os.Exit(1)
	} else {
		if OrderSchedulerTriggerTimeUnit == app.HourUnit {
			if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval {
				log.GLog.Logger.Error("FinanceOrderSchedulerInterval less than FinanceOrderSchedulerInterval",
					"fn", "main",
					"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
				os.Exit(1)
			}

			if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval%24 != 0 {
				log.GLog.Logger.Error("FinanceOrderSchedulerInterval is not factor 24",
					"fn", "main",
					"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
				os.Exit(1)
			}
			OrderSchedulerTriggerInterval = time.Duration(app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval) * time.Hour

		} else {
			if OrderSchedulerTimeUint == app.HourUnit {
				if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval*60 {
					log.GLog.Logger.Error("FinanceOrderSchedulerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
					os.Exit(1)
				}
			} else {
				if app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval < app.Globals.Config.App.FinanceOrderSchedulerInterval {
					log.GLog.Logger.Error("FinanceOrderSchedulerInterval less than FinanceOrderSchedulerInterval",
						"fn", "main",
						"FinanceOrderSchedulerTriggerInterval", app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval)
					os.Exit(1)
				}
			}

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

			triggerPointOffset = time.Duration(offset) * time.Second
		} else {
			offset, err := strconv.Atoi(app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
			if err != nil {
				log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPoint invalid",
					"fn", "main",
					"FinanceOrderSchedulerTriggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
				os.Exit(1)
			}

			if offset > app.Globals.Config.App.FinanceOrderSchedulerTriggerInterval {
				log.GLog.Logger.Error("FinanceOrderSchedulerTriggerPoint is greater than FinanceOrderSchedulerTriggerInterval",
					"fn", "main",
					"FinanceOrderSchedulerTriggerPoint", app.Globals.Config.App.FinanceOrderSchedulerTriggerPoint)
				os.Exit(1)
			}

			triggerPointOffset = time.Duration(offset) * time.Second
		}
	}

	app.Globals.OrderScheduler = order_scheduler.NewOrderScheduler(OrderSchedulerInterval, OrderSchedulerParentWorkerTimeout,
		OrderSchedulerWorkerTimeout, OrderSchedulerTriggerInterval, triggerPointOffset,
		OrderSchedulerTimeUint, OrderSchedulerTriggerTimeUnit)

	app.Globals.OrderScheduler.SchedulerStart(nil)

}

func ParseTime(st string) (int64, error) {
	var h, m, s int
	n, err := fmt.Sscanf(st, "%d:%d:%d", &h, &m, &s)
	fmt.Print(n, err)
	if err != nil || n != 3 {
		return 0, err
	}
	return int64(h*3600 + m*60 + s), nil
}
