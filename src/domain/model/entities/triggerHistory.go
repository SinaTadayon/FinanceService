package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type TriggerResult string

const (
	TriggerSuccess TriggerResult = "SUCCESS"
	TriggerFail    TriggerResult = "FAIL"
	TriggerPartial TriggerResult = "PARTIAL"
)

type TriggerHistory struct {
	ID         primitive.ObjectID `bson:"-"`
	Trigger    string             `bson:"trigger"`
	Version    uint64             `bson:"version"`
	DocVersion string             `bson:"docVersion"`
	Index      int64              `bson:"index"`
	OrderCount int64              `bson:"orderCount"`
	Sellers    []*SellerMissed    `bson:"sellers"`
	Result     TriggerResult      `bson:"result"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
	DeletedAt  *time.Time         `bson:"deletedAt"`
}

type SellerMissed struct {
	SellerId uint64
	SIds     []uint64
}
