package applog

import (
	"gitlab.faza.io/go-framework/logger"
	"go.uber.org/zap"
)

var GLog struct {
	ZapLogger *zap.Logger
	Logger    logger.Logger
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
