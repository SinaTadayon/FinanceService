package logger

import (
	"github.com/rs/xid"
	log "github.com/sirupsen/logrus"
)

// LogModeErr is prefix for errors
const LogModeErr = "error"

// LogModeAudit is prefix for errors
const LogModeAudit = "audit"

// Err shows error log with id and mode
func Err(msg string, vars ...interface{}) {
	log.WithFields(log.Fields{
		"id":   generateNewID(),
		"mode": LogModeErr,
	}).Errorf(msg, vars...)
}

// Audit shows audit logs with id and mode
func Audit(msg string, vars ...interface{}) {
	log.WithFields(log.Fields{
		"id":   generateNewID(),
		"mode": LogModeAudit,
	}).Infof(msg, vars...)
}

func generateNewID() string {
	var uid = xid.New()
	return uid.String()
}
