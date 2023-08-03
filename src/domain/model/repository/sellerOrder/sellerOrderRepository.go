package order_repository

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SellerOrderPageableResult struct {
	SellerOrders []*entities.SellerOrder
	TotalCount   int64
}

type ISellerOrderRepository interface {
	// return data *entities.OrderInfo , error
	SaveOrderInfo(ctx context.Context, fid string, orderInfo entities.OrderInfo) future.IFuture

	// return data *entities.SellerOrder , error
	SaveOrder(ctx context.Context, triggerHistoryId primitive.ObjectID, order entities.SellerOrder) future.IFuture

	// return data []*entities.SellerOrder , error
	SaveAll(ctx context.Context, orders []entities.SellerOrder) future.IFuture

	// return data *entities.SellerOrder, error
	FindByFIdAndOId(ctx context.Context, fid string, oid uint64) future.IFuture

	// return data *[]entities.SellerOrder, error
	FindBySellerIdAndOId(ctx context.Context, sellerId, oid uint64) future.IFuture

	// return data []*entities.SellerOrder, error
	FindById(ctx context.Context, oid uint64) future.IFuture

	// return data []*entities.SellerOrder, error
	FindAll(ctx context.Context, fid string) future.IFuture

	// return data []*entities.SellerOrder, error
	FindAllWithSort(ctx context.Context, fid string, fieldName string, direction int) future.IFuture

	// return data SellerOrderPageableResult, error
	FindAllWithPage(ctx context.Context, fid string, page, perPage int64) future.IFuture

	// return data SellerOrderPageableResult, error
	FindAllWithPageAndSort(ctx context.Context, fid string, page, perPage int64, fieldName string, direction int) future.IFuture

	// return data []*entities.SellerOrder, error
	FindByFilter(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{})) future.IFuture

	// return data SellerOrderPageableResult, error
	FindByFilterWithPage(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{}), page, perPage int64) future.IFuture

	// return data bool, error
	ExistsById(ctx context.Context, oid uint64) future.IFuture

	// return int64, error
	Count(ctx context.Context, fid string) future.IFuture

	// return int64, error
	CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture
}
