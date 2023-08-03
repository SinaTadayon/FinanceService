package entities

import (
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	TriggerDocumentVersion string = "1.0.0"
)

type TriggerTimeUnit string
type TriggerPointType string
type TriggerExecMode string
type TriggerJobExecType string
type TriggerType string

const (
	SellerTrigger TriggerType = "SELLER"
	BuyerTrigger  TriggerType = "BUYER"
)

const (
	TriggerAsyncJob TriggerJobExecType = "ASYNC"
	TriggerSyncJob  TriggerJobExecType = "SYNC"
)

const (
	ConcurrentTrigger TriggerExecMode = "CONCURRENT"
	SequentialTrigger TriggerExecMode = "SEQUENTIAL"
)

const (
	AbsoluteTrigger TriggerPointType = "ABSOLUTE"
	RelativeTrigger TriggerPointType = "RELATIVE"
)

type FinanceTrigger struct {
	ID               primitive.ObjectID `bson:"-"`
	Version          uint64             `bson:"version"`
	DocVersion       string             `bson:"docVersion"`
	Name             string             `bson:"name"`
	Group            string             `bson:"group"`
	Cron             string             `bson:"cron"`
	Duration         int64              `bson:"duration"`
	Interval         int64              `bson:"interval"`
	TimeUnit         utils.TimeUnit     `bson:"timeUnit"`
	TriggerPoint     string             `bson:"triggerPoint"`
	TriggerPointType TriggerPointType   `bson:"triggerPointType"`
	LatestTriggerAt  *time.Time         `bson:"latestTriggerAt"`
	TriggerAt        *time.Time         `bson:"triggerAt"`
	Data             interface{}        `bson:"data"`
	Type             TriggerType        `bson:"type"`
	IsActive         bool               `bson:"isActive"`
	IsEnable         bool               `bson:"isEnable"`
	TestMode         bool               `bson:"testMode"`
	ExecMode         TriggerExecMode    `bson:"execMode"`
	JobExecType      TriggerJobExecType `bson:"jobExecType"`
	CreatedAt        time.Time          `bson:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt"`
	DeletedAt        *time.Time         `bson:"deletedAt"`
}
