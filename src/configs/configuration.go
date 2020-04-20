package configs

import (
	"flag"
	"github.com/Netflix/go-env"
	"github.com/joho/godotenv"
	applog "gitlab.faza.io/services/finance/infrastructure/logger"
	"os"
)


type Config struct {
	App struct {
		ServiceMode                          string `env:"FINANCE_SERVICE_MODE"`
		PrometheusPort                       int    `env:"PROMETHEUS_PORT"`
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
	
	Mongo struct {
		User              string `env:"FINANCE_SERVICE_MONGO_USER"`
		Pass              string `env:"FINANCE_SERVICE_MONGO_PASS"`
		Host              string `env:"FINANCE_SERVICE_MONGO_HOST"`
		Port              int    `env:"FINANCE_SERVICE_MONGO_PORT"`
		Database          string `env:"FINANCE_SERVICE_MONGO_DB_NAME"`
		Collection        string `env:"FINANCE_SERVICE_MONGO_SELLER_COLLECTION_NAME"`
		ConnectionTimeout int    `env:"FINANCE_SERVICE_MONGO_CONN_TIMEOUT"`
		ReadTimeout       int    `env:"FINANCE_SERVICE_MONGO_READ_TIMEOUT"`
		WriteTimeout      int    `env:"FINANCE_SERVICE_MONGO_WRITE_TIMEOUT"`
		MaxConnIdleTime   int    `env:"FINANCE_SERVICE_MONGO_MAX_CONN_IDLE_TIME"`
		MaxPoolSize       int    `env:"FINANCE_SERVICE_MONGO_MAX_POOL_SIZE"`
		MinPoolSize       int    `env:"FINANCE_SERVICE_MONGO_MIN_POOL_SIZE"`
		WriteConcernW     string `env:"FINANCE_SERVICE_MONGO_WRITE_CONCERN_W"`
		WriteConcernJ     string `env:"FINANCE_SERVICE_MONGO_WRITE_CONCERN_J"`
		RetryWrite        bool   `env:"FINANCE_SERVICE_MONGO_RETRY_WRITE"`
	}
}

func LoadConfig(path string) (*Config, error) {
	var config = &Config{}
	currntPath, err := os.Getwd()
	if err != nil {
		applog.GLog.Logger.Error("get current working directory failed", "error", err)
	}

	if os.Getenv("APP_MODE") == "dev" {
		if path != "" {
			err := godotenv.Load(path)
			if err != nil {
				applog.GLog.Logger.Error("Error loading testdata .env file",
					"Working Directory", currntPath,
					"path", path,
					"error", err)
			}
		} else if flag.Lookup("test.v") != nil {
			// test mode
			err := godotenv.Load("../testdata/.env")
			if err != nil {
				applog.GLog.Logger.Error("Error loading testdata .env file", "error", err)
			}
		}
	} else if os.Getenv("APP_MODE") == "docker" {
		err := godotenv.Load(path)
		if err != nil {
			applog.GLog.Logger.Error("Error loading .docker-env file", "path", path)
		}
	}

	// Get environment variables for Config
	_, err = env.UnmarshalFromEnviron(config)
	if err != nil {
		applog.GLog.Logger.Error("env.UnmarshalFromEnviron config failed")
		return nil, err
	}
	
	return config, nil
}

func LoadConfigs(configPath string) (*Config, error) {
	var config = &Config{}
	currntPath, err := os.Getwd()
	if err != nil {
		applog.GLog.Logger.Error("get current working directory failed", "error", err)
	}

	if os.Getenv("APP_MODE") == "dev" {
		if configPath != "" {
			err := godotenv.Load(configPath)
			if err != nil {
				applog.GLog.Logger.Error("Error loading testdata .env file",
					"Working Directory", currntPath,
					"path", configPath,
					"error", err)
			}
		} else if flag.Lookup("test.v") != nil {
			// test mode
			err := godotenv.Load("../testdata/.env")
			if err != nil {
				applog.GLog.Logger.Error("Error loading testdata .env file", "error", err)
			}
		}
	}

	// Get environment variables for Config
	_, err = env.UnmarshalFromEnviron(config)
	if err != nil {
		applog.GLog.Logger.Error("env.UnmarshalFromEnviron config failed")
		return nil, err
	}

	return config, nil
}
