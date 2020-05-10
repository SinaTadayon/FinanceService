package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	TriggerDocumentVersion string = "1.0.0"
)

type TriggerTimeUnit string
type TriggerPointType string

const (
	MinuteUnit TriggerTimeUnit = "MINUTE"
	HourUnit   TriggerTimeUnit = "HOUR"
)

const (
	AbsoluteTrigger TriggerPointType = "ABSOLUTE"
	RelativeTrigger TriggerPointType = "RELATIVE"
)

type SchedulerTrigger struct {
	ID               primitive.ObjectID `bson:"-"`
	Version          uint64             `bson:"version"`
	DocVersion       string             `bson:"docVersion"`
	Name             string             `bson:"name"`
	Group            string             `bson:"group"`
	Cron             string             `bson:"cron"`
	Interval         int64              `bson:"interval"`
	TimeUnit         TriggerTimeUnit    `bson:"timeUnit"`
	TriggerPoint     string             `bson:"triggerPoint"`
	TriggerPointType TriggerPointType   `bson:"triggerPointType"`
	LatestTriggerAt  *time.Time         `bson:"latestTriggerAt"`
	TriggerAt        *time.Time         `bson:"triggerAt"`
	TriggerCount     int64              `bson:"triggerCount"`
	Data             interface{}        `bson:"data"`
	IsEnabled        bool               `bson:"isEnabled"`
	CreatedAt        time.Time          `bson:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt"`
	DeletedAt        *time.Time         `bson:"deletedAt"`
}
