package app

import (
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	user_service "gitlab.faza.io/services/finance/infrastructure/services/user"
	"time"
)

type CtxKey int

const (
	CtxUserID CtxKey = iota
	CtxAuthToken
)

var Globals struct {
	MongoDriver *mongoadapter.Mongo
	Config      *configs.Config
	UserService user_service.IUserService
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
		log.GLog.Logger.Error("mongoadapter.NewMongo failed",
			"fn", "SetupMongoDriver",
			"Mongo", err)
		return nil, errors.Wrap(err, "mongoadapter.NewMongo init failed")
	}

	return mongoDriver, nil
}
