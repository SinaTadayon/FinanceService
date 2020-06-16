//
// The seller order item repository create an export over flatten data in order item level
package sellerOrderItem

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

//
// The unwind data will decoded into following structures
type SellerOrderItems struct {
	SellerFinances []*SellerFinance
	Total          int64
}

type SellerFinance struct {
	ID         primitive.ObjectID       `bson:"_"`
	FId        string                   `bson:"fid"`
	SellerId   uint64                   `bson:"sellerId"`
	Version    uint64                   `bson:"version"`
	DocVersion string                   `bson:"docVersion"`
	SellerInfo *entities.SellerProfile  `bson:"sellerInfo"`
	Invoice    *entities.Invoice        `bson:"invoice"`
	OrderInfo  *OrderInfo               `bson:"ordersInfo"`
	Payment    *entities.FinancePayment `bson:"payment"`
	Status     entities.FinanceState    `bson:"status"`
	StartAt    *time.Time               `bson:"startAt"`
	EndAt      *time.Time               `bson:"endAt"`
	CreatedAt  time.Time                `bson:"createdAt"`
	UpdatedAt  time.Time                `bson:"updatedAt"`
	DeletedAt  *time.Time               `bson:"deletedAt"`
}

type OrderInfo struct {
	TriggerName      string             `bson:"triggerName"`
	TriggerHistoryId primitive.ObjectID `bson:"triggerHistoryId"`
	Order            *SellerOrder       `bson:"orders"`
}

type SellerOrder struct {
	OId                    uint64               `bson:"oid"`
	FId                    string               `bson:"fid"`
	SellerId               uint64               `bson:"sellerId"`
	ShipmentAmount         *entities.Money      `bson:"shipmentAmount"`
	RawShippingNet         *entities.Money      `bson:"rawShippingNet"`
	RoundupShippingNet     *entities.Money      `bson:"roundupShippingNet"`
	IsAlreadyShippingPayed bool                 `bson:"isAlreadyShippingPayed"`
	Item                   *entities.SellerItem `bson:"items"`
	OrderCreatedAt         *time.Time           `bson:"orderCreatedAt"`
	SubPkgCreatedAt        *time.Time           `bson:"subPkgCreatedAt"`
	SubPkgUpdatedAt        *time.Time           `bson:"subPkgUpdatedAt"`
	DeletedAt              *time.Time           `bson:"deletedAt"`
}

type ISellerOrderItemRepository interface {
	// return data SellerOrderPageableResult, error
	FindOrderItemsByFilterWithPage(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{}), page, perPage int64, sortName string, sortDir int) future.IFuture
}
