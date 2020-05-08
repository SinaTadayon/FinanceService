package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	TriggerDocumentVersion string = "1.0.0"
)

type TriggerTimeUnit string

const (
	MillisecondUnit TriggerTimeUnit = "MILLISECOND"
	SecondUnit      TriggerTimeUnit = "SECOND"
	MinuteUnit      TriggerTimeUnit = "MINUTE"
	HourUnit        TriggerTimeUnit = "Hour"
)

type SchedulerTrigger struct {
	ID              primitive.ObjectID `bson:"-"`
	TId             int64              `bson:"tid"`
	Version         uint64             `bson:"version"`
	DocVersion      string             `bson:"docVersion"`
	Name            string             `bson:"name"`
	Group           string             `bson:"group"`
	Cron            string             `bson:"cron"`
	Duration        int64              `bson:"duration"`
	TimeUnit        TriggerTimeUnit    `bson:"timeUnit"`
	TriggerValue    string             `bson:"triggerValue"`
	LatestTriggerAt *time.Time         `bson:"latestTriggerAt"`
	TriggerAt       *time.Time         `bson:"triggerAt"`
	TriggerCount    int64              `bson:"triggerCount"`
	Data            interface{}        `bson:"data"`
	IsEnabled       bool               `bson:"isEnabled"`
	CreatedAt       time.Time          `bson:"createdAt"`
	UpdatedAt       time.Time          `bson:"updatedAt"`
	DeletedAt       *time.Time         `bson:"deletedAt"`
}
