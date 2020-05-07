package order_service

import (
	"context"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"time"
)

type FilterState string

const (
	PayToBuyerFilter  FilterState = "PayToBuyer"
	PayToSellerFilter FilterState = "PayToSeller"
)

type IOrderService interface {
	GetFinanceOrderItems(ctx context.Context, filterState FilterState,
		startTimestamp, endTimestamp time.Time, page, perPage uint32) future.IFuture
}
