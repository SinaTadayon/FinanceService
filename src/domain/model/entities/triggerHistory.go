package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	TriggerHistoryDocumentVersion string = "1.0.0"
)

type TriggerExecResult string
type TriggerRunMode string

const (
	TriggerRunModeNone     TriggerRunMode = "NONE"
	TriggerRunModeRunning  TriggerRunMode = "RUNNING"
	TriggerRunModeComplete TriggerRunMode = "COMPLETE"
)

const (
	TriggerExecResultNone    TriggerExecResult = "NONE"
	TriggerExecResultSuccess TriggerExecResult = "SUCCESS"
	TriggerExecResultFail    TriggerExecResult = "FAIL"
	TriggerExecResultPartial TriggerExecResult = "PARTIAL"
)

type TriggerHistory struct {
	ID           primitive.ObjectID `bson:"-"`
	TriggerName  string             `bson:"triggerName"`
	Version      uint64             `bson:"version"`
	DocVersion   string             `bson:"docVersion"`
	ExecResult   TriggerExecResult  `bson:"execResult"`
	RunMode      TriggerRunMode     `bson:"RunMode"`
	TriggeredAt  *time.Time         `bson:"triggeredAt"`
	IsMissedFire bool               `bson:"isMissedFire"`
	CreatedAt    time.Time          `bson:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"`
	DeletedAt    *time.Time         `bson:"deletedAt"`
}
