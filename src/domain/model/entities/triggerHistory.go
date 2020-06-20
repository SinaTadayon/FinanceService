package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	TriggerHistoryDocumentVersion string = "1.0.1"
)

type TriggerExecResult string
type TriggerRunMode string

const (
	TriggerRunModeNone       TriggerRunMode = "NONE"
	TriggerRunModeRunning    TriggerRunMode = "RUNNING"
	TriggerRunModeComplete   TriggerRunMode = "COMPLETE"
	TriggerRunModeIncomplete TriggerRunMode = "INCOMPLETE"
)

const (
	TriggerExecResultNone    TriggerExecResult = "NONE"
	TriggerExecResultSuccess TriggerExecResult = "SUCCESS"
	TriggerExecResultFail    TriggerExecResult = "FAIL"
	TriggerExecResultPartial TriggerExecResult = "PARTIAL"
)

type TriggerHistory struct {
	ID            primitive.ObjectID   `bson:"_id"`
	TriggerName   string               `bson:"triggerName"`
	Version       uint64               `bson:"version"`
	DocVersion    string               `bson:"docVersion"`
	ExecResult    TriggerExecResult    `bson:"execResult"`
	RunMode       TriggerRunMode       `bson:"runMode"`
	TriggeredAt   *time.Time           `bson:"triggeredAt"`
	IsMissedFire  bool                 `bson:"isMissedFire"`
	Action        primitive.ObjectID   `bson:"action"`
	ActionHistory []primitive.ObjectID `bson:"actionHistory"`
	RetryIndex    uint32               `bson:"retryIndex"`
	Finances      []string             `bson:"finances"`
	CreatedAt     time.Time            `bson:"createdAt"`
	UpdatedAt     time.Time            `bson:"updatedAt"`
	DeletedAt     *time.Time           `bson:"deletedAt"`
}
