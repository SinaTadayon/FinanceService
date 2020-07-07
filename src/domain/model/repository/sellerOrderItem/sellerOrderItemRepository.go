//
// The seller order item repository create an export over flatten data in order item level
package sellerOrderItem

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

//
// The unwind data will decoded into following structures
type SellerOrderItems struct {
	SellerFinances []*entities.SellerFinanceOrderItem
	Total          int64
}

type ISellerOrderItemRepository interface {
	// return data SellerOrderPageableResult, error
	FindOrderItemsByFilterWithPage(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{}), page, perPage int64, sortName string, sortDir int) future.IFuture
}
