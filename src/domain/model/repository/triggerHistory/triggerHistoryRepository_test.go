package trigger_history_repository

import (
	"context"
	"github.com/stretchr/testify/require"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"os"
	"testing"
	"time"
)

var triggerHistoryRepository ITriggerHistoryRepository

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

	triggerHistoryRepository = NewTriggerHistoryRepository(mongoAdapter, config.Mongo.Database, config.Mongo.TriggerHistoryCollection)

	// Running Tests
	code := m.Run()
	removeCollection()
	os.Exit(code)
}

func TestSaveTriggerHistoryRepository(t *testing.T) {
	defer removeCollection()
	history := createTriggerHistory()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerHistoryRepository.Save(ctx, *history).Get()
	require.Nil(t, iFuture.Error(), "triggerHistoryRepository.Save failed")
	require.NotNil(t, iFuture.Data().(*entities.TriggerHistory).ID, "triggerHistoryRepository.Save failed, tid not generated")
}

func TestUpdateTriggerHistoryRepository(t *testing.T) {

	defer removeCollection()
	history := createTriggerHistory()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerHistoryRepository.Save(ctx, *history).Get()
	history1 := iFuture.Data().(*entities.TriggerHistory)
	require.Nil(t, iFuture.Error(), "triggerHistoryRepository.Save failed")
	require.NotNil(t, history1.ID, "triggerHistoryRepository.Save failed, id not generated")

	history1.IsMissedFire = true
	iFuture = triggerHistoryRepository.Update(ctx, *history1).Get()
	require.Nil(t, iFuture.Error(), "triggerHistoryRepository.Update failed")
	require.Equal(t, true, iFuture.Data().(*entities.TriggerHistory).IsMissedFire)
}

func TestFindByIdTriggerHistoryRepository(t *testing.T) {
	defer removeCollection()
	history := createTriggerHistory()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := triggerHistoryRepository.Save(ctx, *history).Get()
	history1 := iFuture.Data().(*entities.TriggerHistory)
	require.Nil(t, iFuture.Error(), "triggerHistoryRepository.Save failed")
	require.NotNil(t, history1.ID, "triggerHistoryRepository.Save failed, id not generated")

	iFuture = triggerHistoryRepository.FindById(ctx, history1.ID).Get()
	require.Nil(t, iFuture.Error(), "triggerHistoryRepository.FindById failed")
	require.Equal(t, history.TriggerName, iFuture.Data().(*entities.TriggerHistory).TriggerName)

}

func createTriggerHistory() *entities.TriggerHistory {
	timestamp := time.Now().UTC()
	return &entities.TriggerHistory{
		TriggerName:  "SCH4",
		Version:      1,
		DocVersion:   entities.TriggerDocumentVersion,
		ExecResult:   entities.TriggerExecResultSuccess,
		TriggeredAt:  &timestamp,
		IsMissedFire: false,
		RunMode:      entities.TriggerRunModeComplete,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		DeletedAt:    nil,
	}
}

func removeCollection() {
	ctx, _ := context.WithCancel(context.Background())
	if err := triggerHistoryRepository.RemoveAll(ctx); err != nil {
	}
}
