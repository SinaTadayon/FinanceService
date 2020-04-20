package logger

import (
	"context"
	"runtime/debug"

	"go.uber.org/zap"
	"google.golang.org/grpc/metadata"
)

// NewProductionZaplogger will return a new production logger backed by zap
func NewProductionZaplogger() (Logger, error) {
	conf := zap.NewProductionConfig()
	conf.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	conf.DisableCaller = true
	conf.DisableStacktrace = true
	zapLogger, err := conf.Build(zap.AddCaller(), zap.AddCallerSkip(1))

	return zpLg{
		lg: zapLogger.Sugar(),
	}, err
}

// NewZapLogger will return a new logger backed by the provided zap instance
func NewZapLogger(lg *zap.Logger) Logger {
	return zpLg{
		lg: lg.Sugar(),
	}
}

type zpLg struct {
	lg *zap.SugaredLogger
}

func (l zpLg) Log(keyvals ...interface{}) error {
	l.lg.Infow("", keyvals...)
	return nil
}

func (l zpLg) With(keyvals ...interface{}) (ll Logger) {
	ll = zpLg{
		lg: l.lg.With(keyvals...),
	}
	return
}

func (l zpLg) FromContext(ctx context.Context) (ll Logger) {
	ll = l
	vals := extractValuesFromGRPcContectx(ctx)
	valarray := make([]interface{}, 0)
	for k, v := range vals {
		valarray = append(valarray, k, v)
	}
	ll = zpLg{
		lg: l.lg.With(valarray...),
	}
	return
}

func extractValuesFromGRPcContectx(ctx context.Context) (vals map[string]string) {
	vals = make(map[string]string, 0)
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return
	}
	keys := []string{
		"real-ip",
		"user-agent",
		"forwarded-host",
		"request-id",
		"user-id",
	}
	for _, key := range keys {
		if val, ok := md[key]; ok && len(val) > 0 {
			vals[key] = val[0]
		}
	}
	return
}

func (l zpLg) Debug(msg string, keyvals ...interface{}) {
	l.lg.Debugw(msg, keyvals...)
}

func (l zpLg) Info(msg string, keyvals ...interface{}) {
	l.lg.Infow(msg, keyvals...)
}

func (l zpLg) Warn(msg string, keyvals ...interface{}) {
	l.lg.Warnw(msg, keyvals...)
}

func (l zpLg) Error(msg string, keyvals ...interface{}) {
	l.lg.With("stacktrace", string(debug.Stack())).Errorw(msg, keyvals...)
}

func (l zpLg) Fatal(msg string, keyvals ...interface{}) {
	l.lg.Fatalw(msg, keyvals...)
}
