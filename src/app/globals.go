package app

import (
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	order_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerOrder"
	trigger_repository "gitlab.faza.io/services/finance/domain/model/repository/trigger"
	trigger_history_repository "gitlab.faza.io/services/finance/domain/model/repository/triggerHistory"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	order_service "gitlab.faza.io/services/finance/infrastructure/services/order"
	payment_service "gitlab.faza.io/services/finance/infrastructure/services/payment"
	user_service "gitlab.faza.io/services/finance/infrastructure/services/user"
	"gitlab.faza.io/services/finance/infrastructure/workerPool"
	"time"
)

var Globals struct {
	MongoDriver              *mongoadapter.Mongo
	Config                   *configs.Config
	UserService              user_service.IUserService
	OrderService             order_service.IOrderService
	PaymentService           payment_service.IPaymentService
	SellerFinanceRepository  finance_repository.ISellerFinanceRepository
	SellerOrderRepository    order_repository.ISellerOrderRepository
	TriggerRepository        trigger_repository.ISchedulerTriggerRepository
	TriggerHistoryRepository trigger_history_repository.ITriggerHistoryRepository
	WorkerPool               worker_pool.IWorkerPool
}

func SetupMongoDriver(config configs.Config) (*mongoadapter.Mongo, error) {
	// store in mongo
	mongoConf := &mongoadapter.MongoConfig{
		Username:               config.Mongo.User,
		ConnTimeout:            time.Duration(config.Mongo.ConnectionTimeout) * time.Second,
		ReadTimeout:            time.Duration(config.Mongo.ReadTimeout) * time.Second,
		WriteTimeout:           time.Duration(config.Mongo.WriteTimeout) * time.Second,
		MaxConnIdleTime:        time.Duration(config.Mongo.MaxConnIdleTime) * time.Second,
		HeartbeatInterval:      time.Duration(config.Mongo.HeartBeatInterval) * time.Second,
		ServerSelectionTimeout: time.Duration(config.Mongo.ServerSelectionTimeout) * time.Second,
		RetryConnect:           uint64(config.Mongo.RetryConnect),
		MaxPoolSize:            uint64(config.Mongo.MaxPoolSize),
		MinPoolSize:            uint64(config.Mongo.MinPoolSize),
		WriteConcernW:          config.Mongo.WriteConcernW,
		WriteConcernJ:          config.Mongo.WriteConcernJ,
		RetryWrites:            config.Mongo.RetryWrite,
		ReadConcern:            config.Mongo.ReadConcern,
		ReadPreference:         config.Mongo.ReadPreferred,
		ConnectUri:             config.Mongo.URI,
	}

	mongoDriver, err := mongoadapter.NewMongo(mongoConf)
	if err != nil {
		log.GLog.Logger.Error("mongoadapter.NewMongo failed",
			"fn", "SetupMongoDriver",
			"Mongo", err)
		return nil, errors.Wrap(err, "mongoadapter.NewMongo init failed")
	}

	_, err = mongoDriver.AddUniqueIndex(config.Mongo.Database, config.Mongo.SellerCollection, "fid")
	if err != nil {
		log.GLog.Logger.Error("create fid index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "sellerId")
	if err != nil {
		log.GLog.Logger.Error("create sellerId index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "status")
	if err != nil {
		log.GLog.Logger.Error("create status index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "startAt")
	if err != nil {
		log.GLog.Logger.Error("create startAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "endAt")
	if err != nil {
		log.GLog.Logger.Error("create endAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "createdAt")
	if err != nil {
		log.GLog.Logger.Error("create createdAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "updatedAt")
	if err != nil {
		log.GLog.Logger.Error("create updatedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "deletedAt")
	if err != nil {
		log.GLog.Logger.Error("create deletedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "payment.status")
	if err != nil {
		log.GLog.Logger.Error("create payment.status index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.triggerName")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.triggerName index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.triggerHistoryId")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.triggerHistoryId index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.orders.oid")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.orders.oid index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.orders.orderCreatedAt")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.orders.orderCreatedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.orders.subPkgCreatedAt")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.orders.subPkgCreatedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.orders.subPkgUpdatedAt")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.orders.subPkgUpdatedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.SellerCollection, "ordersInfo.orders.items.sid")
	if err != nil {
		log.GLog.Logger.Error("create ordersInfo.orders.items.sid index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.SellerCollection,
			"error", err)
		return nil, err
	}

	//////////////////////////////////////////////////
	// finance trigger
	_, err = mongoDriver.AddUniqueIndex(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "name")
	if err != nil {
		log.GLog.Logger.Error("create name index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "triggerAt")
	if err != nil {
		log.GLog.Logger.Error("create triggerAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "latestTriggerAt")
	if err != nil {
		log.GLog.Logger.Error("create latestTriggerAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "isActive")
	if err != nil {
		log.GLog.Logger.Error("create isActive index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "createdAt")
	if err != nil {
		log.GLog.Logger.Error("create createdAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "updatedAt")
	if err != nil {
		log.GLog.Logger.Error("create updatedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.FinanceTriggerCollection, "deletedAt")
	if err != nil {
		log.GLog.Logger.Error("create deletedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.FinanceTriggerCollection,
			"error", err)
		return nil, err
	}

	//////////////////////////////////////////////////
	// trigger history
	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "triggerName")
	if err != nil {
		log.GLog.Logger.Error("create triggerName index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "triggeredAt")
	if err != nil {
		log.GLog.Logger.Error("create triggeredAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "execResult")
	if err != nil {
		log.GLog.Logger.Error("create execResult index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "runMode")
	if err != nil {
		log.GLog.Logger.Error("create runMode index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "isMissedFire")
	if err != nil {
		log.GLog.Logger.Error("create isMissedFire index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "createdAt")
	if err != nil {
		log.GLog.Logger.Error("create createdAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "updatedAt")
	if err != nil {
		log.GLog.Logger.Error("create createdAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	_, err = mongoDriver.AddTextV3Index(config.Mongo.Database, config.Mongo.TriggerHistoryCollection, "deletedAt")
	if err != nil {
		log.GLog.Logger.Error("create deletedAt index failed",
			"fn", "SetupMongoDriver",
			"collection", config.Mongo.TriggerHistoryCollection,
			"error", err)
		return nil, err
	}

	return mongoDriver, nil
}
