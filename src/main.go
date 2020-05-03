package main

import (
	"fmt"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	"os"
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

	if err != nil {
		log.GLog.Logger.Error("LoadConfig of main init failed",
			"fn", "main", "error", err)
		os.Exit(1)
	}

	//mongoDriver, err := app.SetupMongoDriver(*app.Globals.Config)
	//if err != nil {
	//	log.GLog.Logger.Error("main SetupMongoDriver failed", "fn", "main",
	//		"configs", app.Globals.Config.Mongo, "error", err)
	//}

	if app.Globals.Config.App.ServiceMode == "server" {
		log.GLog.Logger.Info("Order Service Run in Server Mode . . . ", "fn", "main")
		log.GLog.Logger.Info(buildInfo())
	}
}
