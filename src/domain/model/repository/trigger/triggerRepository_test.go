package trigger_repository

import (
	"context"
	"github.com/stretchr/testify/require"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"go.mongodb.org/mongo-driver/bson"
	"os"
	"testing"
	"time"
)

var triggerRepository ISchedulerTriggerRepository

func TestMain(m *testing.M) {
	var err error
	var path string
	if os.Getenv("APP_MODE") == "dev" {
		path = "../../../../testdata/.env"
	} else {
		path = ""
	}

	log.GLog.ZapLogger = log.InitZap()
	log.GLog.Logger = logger.NewZapLogger(log.GLog.ZapLogger)

	config, err := configs.LoadConfigs(path)
	if err != nil {
		log.GLog.Logger.Error("configs.LoadConfig failed",
			"error", err)
		os.Exit(1)
	}

	// store in mongo
	mongoConf := &mongoadapter.MongoConfig{
		Host:     config.Mongo.Host,
		Port:     config.Mongo.Port,
		Username: config.Mongo.User,
		//Password:     App.Cfg.Mongo.Pass,
		ConnTimeout:     time.Duration(config.Mongo.ConnectionTimeout) * time.Second,
		ReadTimeout:     time.Duration(config.Mongo.ReadTimeout) * time.Second,
		WriteTimeout:    time.Duration(config.Mongo.WriteTimeout) * time.Second,
		MaxConnIdleTime: time.Duration(config.Mongo.MaxConnIdleTime) * time.Second,
		MaxPoolSize:     uint64(config.Mongo.MaxPoolSize),
		MinPoolSize:     uint64(config.Mongo.MinPoolSize),
		WriteConcernW:   config.Mongo.WriteConcernW,
		WriteConcernJ:   config.Mongo.WriteConcernJ,
		RetryWrites:     config.Mongo.RetryWrite,
	}

	mongoAdapter, err := mongoadapter.NewMongo(mongoConf)
	if err != nil {
		log.GLog.Logger.Error("mongoadapter.NewMongo failed", "error", err)
		os.Exit(1)
	}

	triggerRepository = NewSchedulerTriggerRepository(mongoAdapter, config.Mongo.Database, config.Mongo.FinanceTriggerCollection)

	// Running Tests
	code := m.Run()
	removeCollection()
	os.Exit(code)
}

func TestSaveTriggerRepository(t *testing.T) {
	defer removeCollection()
	trigger := createSchedulerTrigger()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error(), "triggerRepository.Save failed")
	require.NotNil(t, iFuture.Data().(*entities.FinanceTrigger).ID, "triggerRepository.Save failed, tid not generated")
}

func TestUpdateTriggerRepository(t *testing.T) {

	defer removeCollection()
	trigger := createSchedulerTrigger()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerRepository.Save(ctx, *trigger).Get()
	trigger1 := iFuture.Data().(*entities.FinanceTrigger)
	require.Nil(t, iFuture.Error(), "triggerRepository.Save failed")
	require.NotNil(t, trigger1.ID, "triggerRepository.Save failed, id not generated")

	trigger1.Data = "PAYMENT_IN_PROGRESS"
	iFuture = triggerRepository.Update(ctx, *trigger1).Get()
	require.Nil(t, iFuture.Error(), "triggerRepository.Save failed")
	require.Equal(t, "PAYMENT_IN_PROGRESS", iFuture.Data().(*entities.FinanceTrigger).Data.(string))
}

func TestExistsByNameRepository(t *testing.T) {
	defer removeCollection()
	trigger := createSchedulerTrigger()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error())

	iFuture = triggerRepository.FindByName(ctx, iFuture.Data().(*entities.FinanceTrigger).Name).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, trigger.Name, iFuture.Data().(*entities.FinanceTrigger).Name)
}

func TestDeleteTriggerRepository(t *testing.T) {
	defer removeCollection()
	trigger := createSchedulerTrigger()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error())
	trigger1 := iFuture.Data().(*entities.FinanceTrigger)

	iFuture = triggerRepository.Delete(ctx, *trigger1).Get()
	require.Nil(t, iFuture.Error())
	require.NotNil(t, iFuture.Data().(*entities.FinanceTrigger).DeletedAt)
}

func TestRemoveTriggerRepository(t *testing.T) {
	defer removeCollection()
	trigger := createSchedulerTrigger()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error())

	iFuture = triggerRepository.Remove(ctx, *(iFuture.Data().(*entities.FinanceTrigger))).Get()
	require.Nil(t, iFuture.Error())
}

func TestFindByFilterRepository(t *testing.T) {
	defer removeCollection()
	trigger := createSchedulerTrigger()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error())

	iFuture = triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error())

	trigger.Interval = 12
	iFuture = triggerRepository.Save(ctx, *trigger).Get()
	require.Nil(t, iFuture.Error())

	iFuture = triggerRepository.FindByFilter(ctx, func() interface{} {
		return bson.D{{"duration", 12}, {"deletedAt", nil}}
	}).Get()

	require.Nil(t, iFuture.Error())
	require.Equal(t, int64(12), iFuture.Data().([]*entities.FinanceTrigger)[0].Interval)
}

func removeCollection() {
	ctx, _ := context.WithCancel(context.Background())
	if err := triggerRepository.RemoveAll(ctx); err != nil {
	}
}

func createSchedulerTrigger() *entities.FinanceTrigger {
	timestamp := time.Now().UTC()
	return &entities.FinanceTrigger{
		Version:          1,
		DocVersion:       entities.TriggerDocumentVersion,
		Name:             "Trigger-Test",
		Group:            "Trigger-Group",
		Cron:             "",
		Duration:         24,
		Interval:         24,
		TimeUnit:         utils.HourUnit,
		TriggerPoint:     "23:59",
		TriggerPointType: entities.AbsoluteTrigger,
		LatestTriggerAt:  &timestamp,
		TriggerAt:        &timestamp,
		Data:             nil,
		Type:             entities.SellerTrigger,
		IsActive:         true,
		IsEnable:         true,
		TestMode:         false,
		ExecMode:         entities.SequentialTrigger,
		JobExecType:      entities.TriggerSyncJob,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		DeletedAt:        nil,
	}
}
