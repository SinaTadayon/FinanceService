package configs

import (
	"flag"
	"github.com/Netflix/go-env"
	"github.com/joho/godotenv"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	"os"
)

type Config struct {
	App struct {
		ServiceMode           string `env:"FINANCE_SERVICE_MODE"`
		PrometheusPort        int    `env:"PROMETHEUS_PORT"`
		ServiceTestAPIEnabled bool   `env:"FINANCE_SERVER_TEST_API_ENABLED"`

		FinancePaymentSchedulerTimeUint            string `env:"FINANCE_PAYMENT_SCHEDULER_TIME_UNIT"`
		FinancePaymentSchedulerInterval            int    `env:"FINANCE_PAYMENT_SCHEDULER_INTERVAL"`
		FinancePaymentSchedulerParentWorkerTimeout int    `env:"FINANCE_PAYMENT_SCHEDULER_PARENT_WORKER_TIMEOUT"`
		FinancePaymentSchedulerWorkerTimeout       int    `env:"FINANCE_PAYMENT_SCHEDULER_WORKER_TIMEOUT"`
		FinancePaymentSchedulerStates              string `env:"FINANCE_PAYMENT_SCHEDULER_STATES"`

		FinanceOrderSchedulerTimeUint                string `env:"FINANCE_ORDER_SCHEDULER_TIME_UNIT"`
		FinanceOrderSchedulerInterval                int    `env:"FINANCE_ORDER_SCHEDULER_INTERVAL"`
		FinanceOrderSchedulerParentWorkerTimeout     int    `env:"FINANCE_ORDER_SCHEDULER_PARENT_WORKER_TIMEOUT"`
		FinanceOrderSchedulerWorkerTimeout           int    `env:"FINANCE_ORDER_SCHEDULER_WORKER_TIMEOUT"`
		FinanceOrderSchedulerUpdateFinanceDuration   bool   `env:"FINANCE_ORDER_SCHEDULER_UPDATE_FINANCE_DURATION"`
		FinanceOrderSchedulerHandleMissedFireTrigger bool   `env:"FINANCE_ORDER_SCHEDULER_HANDLE_MISSED_FIRE_TRIGGER"`

		SellerFinanceTriggerName      string `env:"FINANCE_SELLER_FINANCE_TRIGGER_NAME"`
		SellerFinanceTriggerTimeUnit  string `env:"FINANCE_SELLER_FINANCE_TRIGGER_TIME_UNIT"`
		SellerFinanceTriggerInterval  int    `env:"FINANCE_SELLER_FINANCE_TRIGGER_INTERVAL"`
		SellerFinanceTriggerDuration  int    `env:"FINANCE_SELLER_FINANCE_TRIGGER_DURATION"`
		SellerFinanceTriggerPoint     string `env:"FINANCE_SELLER_FINANCE_TRIGGER_POINT"`
		SellerFinanceTriggerPointType string `env:"FINANCE_SELLER_FINANCE_TRIGGER_POINT_TYPE"`
		SellerFinanceTriggerEnabled   bool   `env:"FINANCE_SELLER_FINANCE_TRIGGER_ENABLED"`
		SellerFinanceTriggerTestMode  bool   `env:"FINANCE_SELLER_FINANCE_TRIGGER_TEST_MODE"`

		SellerFinancePreventDuplicateOrderItem bool `env:"FINANCE_SELLER_FINANCE_PREVENT_DUPLICATE_ORDER_ITEM"`
	}

	GRPCServer struct {
		Address string `env:"FINANCE_SERVER_ADDRESS"`
		Port    int    `env:"FINANCE_SERVER_PORT"`
	}

	UserService struct {
		Address string `env:"USER_SERVICE_ADDRESS"`
		Port    int    `env:"USER_SERVICE_PORT"`
		Timeout int    `env:"USER_SERVICE_TIMEOUT"`
	}

	OrderService struct {
		Address string `env:"ORDER_SERVICE_ADDRESS"`
		Port    int    `env:"ORDER_SERVICE_PORT"`
		Timeout int    `env:"ORDER_SERVICE_TIMEOUT"`
	}

	PaymentTransferService struct {
		Address     string `env:"PAYMENT_TRANSFER_SERVICE_ADDRESS"`
		Port        int    `env:"PAYMENT_TRANSFER_SERVICE_PORT"`
		Timeout     int    `env:"PAYMENT_TRANSFER_SERVICE_TIMEOUT"`
		MockEnabled bool   `env:"PAYMENT_TRANSFER_SERVICE_MOCK_ENABLED"`
	}

	Mongo struct {
		User                     string `env:"FINANCE_SERVICE_MONGO_USER"`
		Pass                     string `env:"FINANCE_SERVICE_MONGO_PASS"`
		Host                     string `env:"FINANCE_SERVICE_MONGO_HOST"`
		Port                     int    `env:"FINANCE_SERVICE_MONGO_PORT"`
		Database                 string `env:"FINANCE_SERVICE_MONGO_DB_NAME"`
		SellerCollection         string `env:"FINANCE_SERVICE_MONGO_SELLER_COLLECTION_NAME"`
		FinanceTriggerCollection string `env:"FINANCE_SERVICE_MONGO_FINANCE_TRIGGER_COLLECTION_NAME"`
		TriggerHistoryCollection string `env:"FINANCE_SERVICE_MONGO_TRIGGER_HISTORY_COLLECTION_NAME"`
		ConnectionTimeout        int    `env:"FINANCE_SERVICE_MONGO_CONN_TIMEOUT"`
		ReadTimeout              int    `env:"FINANCE_SERVICE_MONGO_READ_TIMEOUT"`
		WriteTimeout             int    `env:"FINANCE_SERVICE_MONGO_WRITE_TIMEOUT"`
		MaxConnIdleTime          int    `env:"FINANCE_SERVICE_MONGO_MAX_CONN_IDLE_TIME"`
		MaxPoolSize              int    `env:"FINANCE_SERVICE_MONGO_MAX_POOL_SIZE"`
		MinPoolSize              int    `env:"FINANCE_SERVICE_MONGO_MIN_POOL_SIZE"`
		WriteConcernW            string `env:"FINANCE_SERVICE_MONGO_WRITE_CONCERN_W"`
		WriteConcernJ            string `env:"FINANCE_SERVICE_MONGO_WRITE_CONCERN_J"`
		RetryWrite               bool   `env:"FINANCE_SERVICE_MONGO_RETRY_WRITE"`
	}
}

func LoadConfig(path string) (*Config, error) {
	var config = &Config{}
	currentPath, err := os.Getwd()
	if err != nil {
		log.GLog.Logger.Error("get current working directory failed", "error", err)
	}

	if os.Getenv("APP_MODE") == "dev" {
		if path != "" {
			err := godotenv.Load(path)
			if err != nil {
				log.GLog.Logger.Error("Error loading testdata .env file",
					"Working Directory", currentPath,
					"path", path,
					"error", err)
			}
		} else if flag.Lookup("test.v") != nil {
			// test mode
			err := godotenv.Load("../testdata/.env")
			if err != nil {
				log.GLog.Logger.Error("Error loading testdata .env file", "error", err)
			}
		}
	}

	// Get environment variables for Config
	_, err = env.UnmarshalFromEnviron(config)
	if err != nil {
		log.GLog.Logger.Error("env.UnmarshalFromEnviron config failed", "error", err)
		return nil, err
	}

	return config, nil
}

func LoadConfigs(configPath string) (*Config, error) {
	var config = &Config{}
	currentPath, err := os.Getwd()
	if err != nil {
		log.GLog.Logger.Error("get current working directory failed", "error", err)
	}

	if os.Getenv("APP_MODE") == "dev" {
		if configPath != "" {
			err := godotenv.Load(configPath)
			if err != nil {
				log.GLog.Logger.Error("Error loading testdata .env file",
					"Working Directory", currentPath,
					"path", configPath,
					"error", err)
			}
		} else if flag.Lookup("test.v") != nil {
			// test mode
			err := godotenv.Load("../testdata/.env")
			if err != nil {
				log.GLog.Logger.Error("Error loading testdata .env file", "error", err)
			}
		}
	}

	// Get environment variables for Config
	_, err = env.UnmarshalFromEnviron(config)
	if err != nil {
		log.GLog.Logger.Error("env.UnmarshalFromEnviron config failed", "error", err)
		return nil, err
	}

	return config, nil
}
