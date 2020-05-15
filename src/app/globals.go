package app

import (
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	order_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerOrder"
	trigger_repository "gitlab.faza.io/services/finance/domain/model/repository/trigger"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/pool"
	order_service "gitlab.faza.io/services/finance/infrastructure/services/order"
	user_service "gitlab.faza.io/services/finance/infrastructure/services/user"
	"time"
)

var Globals struct {
	MongoDriver             *mongoadapter.Mongo
	Config                  *configs.Config
	UserService             user_service.IUserService
	OrderService            order_service.IOrderService
	SellerFinanceRepository finance_repository.ISellerFinanceRepository
	SellerOrderRepository   order_repository.ISellerOrderRepository
	TriggerRepository       trigger_repository.ISchedulerTriggerRepository
	WorkerPool              pool.IWorkerPool
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
