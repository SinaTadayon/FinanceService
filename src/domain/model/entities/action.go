package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	ActionDocumentVersion string = "1.0.0"
)

type Action struct {
	ID         primitive.ObjectID     `bson:"_id"`
	Version    uint64                 `bson:"version"`
	DocVersion string                 `bson:"docVersion"`
	Name       string                 `bson:"name"`
	Type       string                 `bson:"type"`
	UId        uint64                 `bson:"uid"`
	UTP        string                 `bson:"utp"`
	Result     string                 `bson:"result"`
	Note       string                 `bson:"note"`
	Data       map[string]interface{} `bson:"data"`
	CreatedAt  time.Time              `bson:"createdAt"`
	UpdatedAt  time.Time              `bson:"updatedAt"`
}
