package order_repository

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type OrderFinancePageableResult struct {
	OrderFinances []*entities.OrderFinance
	TotalCount    int64
}

type IOrderFinanceRepository interface {
	// return data *entities.OrderFinance , error
	Save(ctx context.Context, order entities.OrderFinance) future.IFuture

	// return data []*entities.OrderFinance , error
	SaveAll(ctx context.Context, orders []entities.OrderFinance) future.IFuture

	// return data *entities.OrderFinance, error
	FindByFIdAndOId(ctx context.Context, fid string, oid uint64) future.IFuture

	// return data *[]entities.OrderFinance, error
	FindBySellerIdAndOId(ctx context.Context, sellerId, oid uint64) future.IFuture

	// return data []*entities.OrderFinance, error
	FindById(ctx context.Context, oid uint64) future.IFuture

	// return data []*entities.OrderFinance, error
	FindAll(ctx context.Context, fid string) future.IFuture

	// return data []*entities.OrderFinance, error
	FindAllWithSort(ctx context.Context, fid string, fieldName string, direction int) future.IFuture

	// return data OrderFinancePageableResult, error
	FindAllWithPage(ctx context.Context, fid string, page, perPage int64) future.IFuture

	// return data OrderFinancePageableResult, error
	FindAllWithPageAndSort(ctx context.Context, fid string, page, perPage int64, fieldName string, direction int) future.IFuture

	// return data []*entities.OrderFinance, error
	FindByFilter(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{})) future.IFuture

	// return data OrderFinancePageableResult, error
	FindByFilterWithPage(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{}), page, perPage int64) future.IFuture

	// return data bool, error
	ExistsById(ctx context.Context, oid uint64) future.IFuture

	// return int64, error
	Count(ctx context.Context, fid string) future.IFuture

	// return int64, error
	CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture
}
