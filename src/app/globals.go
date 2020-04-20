package app

import (
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	user_service "gitlab.faza.io/services/finance/infrastructure/services/user"
	"go.uber.org/zap"
	"time"
)

type CtxKey int

const (
	CtxUserID CtxKey = iota
	CtxAuthToken
)

var Globals struct {
	MongoDriver       *mongoadapter.Mongo
	Config            *configs.Config
	ZapLogger         *zap.Logger
	Logger            logger.Logger
	UserService       user_service.IUserService
}

func SetupMongoDriver(config configs.Config) (*mongoadapter.Mongo, error) {
	// store in mongo
	mongoConf := &mongoadapter.MongoConfig{
		Host:     config.Mongo.Host,
		Port:     config.Mongo.Port,
		Username: config.Mongo.User,
		//Password:     MainApp.Config.Mongo.Pass,
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

	mongoDriver, err := mongoadapter.NewMongo(mongoConf)
	if err != nil {
		Globals.Logger.Error("mongoadapter.NewMongo failed",
			"fn", "SetupMongoDriver",
			"Mongo", err)
		return nil, errors.Wrap(err, "mongoadapter.NewMongo init failed")
	}

	return mongoDriver, nil
}

func InitZap() (zapLogger *zap.Logger) {
	conf := zap.NewProductionConfig()
	conf.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	conf.DisableCaller = true
	conf.DisableStacktrace = true
	zapLogger, e := conf.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	// zapLogger, e := conf.Build()
	// zapLogger, e := zap.NewProduction(zap.AddCallerSkip(3))
	if e != nil {
		panic(e)
	}
	return
}
