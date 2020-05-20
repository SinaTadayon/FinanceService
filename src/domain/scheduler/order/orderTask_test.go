package order_scheduler

import (
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/services/finance/infrastructure/workerPool"
	//"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/configs"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	order_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerOrder"
	trigger_repository "gitlab.faza.io/services/finance/domain/model/repository/trigger"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"os"
	"testing"
	//"time"
)

//var mongoAdapter *mongoadapter.Mongo
//var config *configs.Config

func TestMain(m *testing.M) {
	var err error
	if os.Getenv("APP_MODE") == "dev" {
		app.Globals.Config, err = configs.LoadConfig("../../../testdata/.env")
	} else {
		app.Globals.Config, err = configs.LoadConfig("")
	}

	log.GLog.ZapLogger = log.InitZap()
	log.GLog.Logger = logger.NewZapLogger(log.GLog.ZapLogger)

	if err != nil {
		os.Exit(1)
	}

	mongoDriver, err := app.SetupMongoDriver(*app.Globals.Config)
	if err != nil {
		os.Exit(1)
	}

	app.Globals.SellerFinanceRepository = finance_repository.NewSellerFinanceRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.SellerCollection)
	app.Globals.SellerOrderRepository = order_repository.NewSellerOrderRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.SellerCollection)
	app.Globals.TriggerRepository = trigger_repository.NewSchedulerTriggerRepository(mongoDriver, app.Globals.Config.Mongo.Database, app.Globals.Config.Mongo.TriggerCollection)

	workerPool, err := worker_pool.Factory()
	if err != nil {
		os.Exit(1)
	}
	app.Globals.WorkerPool = workerPool

	// Running Tests
	code := m.Run()
	os.Exit(code)
}

//func TestOrderSchedulerTask(t *testing.T) {
//	ctx, _ := context.WithCancel(context.Background())
//	iFuture := OrderSchedulerTask(ctx, entities.TriggerHistoryId{})
//	//time.Sleep(time.Second)
//	//cancel()
//	//time.Sleep(time.Second)
//	iFutureData := iFuture.Get()
//	require.Nil(t, iFutureData.Error())
//}
