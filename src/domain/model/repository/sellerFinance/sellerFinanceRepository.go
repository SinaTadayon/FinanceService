package sellerFinance

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type FinancePageableResult struct {
	SellerFinances []*entities.SellerFinance
	TotalCount     int64
}

type ISellerFinanceRepository interface {
	// return data *entities.SellerFinance and error
	Save(ctx context.Context, finance entities.SellerFinance) future.IFuture

	// return data []*entities.SellerFinance and error
	SaveAll(ctx context.Context, finances []entities.SellerFinance) future.IFuture

	// return error
	UpdateStatus(ctx context.Context, finance *entities.SellerFinance) future.IFuture

	// return data *entities.SellerFinance and error
	Insert(ctx context.Context, finance entities.SellerFinance) future.IFuture

	// return data []*entities.SellerFinance and error
	InsertAll(ctx context.Context, finances []entities.SellerFinance) future.IFuture

	// return data []*entities.SellerFinance and error
	FindAll(ctx context.Context) future.IFuture

	// return data []*entities.SellerFinance and error
	FindAllWithSort(ctx context.Context, fieldName string, direction int) future.IFuture

	// return data FinancePageableResult and error
	FindAllWithPage(ctx context.Context, page, perPage int64) future.IFuture

	// return data FinancePageableResult and error
	FindAllWithPageAndSort(ctx context.Context, page, perPage int64, fieldName string, direction int) future.IFuture

	// return data []*entities.SellerFinance and error
	FindAllById(ctx context.Context, fids ...string) future.IFuture

	// return data *entities.SellerFinance and error
	FindById(ctx context.Context, fid string) future.IFuture

	// return data []*entities.SellerFinance and error
	FindByFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture

	// return data []*entities.SellerFinance and error
	FindByFilterWithSort(ctx context.Context, supplier func() (filter interface{}, fieldName string, direction int)) future.IFuture

	// return data FinancePageableResult and error
	FindByFilterWithPage(ctx context.Context, supplier func() (filter interface{}), page, perPage int64) future.IFuture

	// return data FinancePageableResult and error
	FindByFilterWithPageAndSort(ctx context.Context, supplier func() (filter interface{}, fieldName string, direction int), page, perPage int64) future.IFuture

	// return data bool and error
	ExistsById(ctx context.Context, fid string) future.IFuture

	// return data int64 and error
	Count(ctx context.Context) future.IFuture

	// return data int64 and error
	CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture

	// only set DeletedAt field
	// return data *entities.SellerFinance and error
	DeleteById(ctx context.Context, fid string) future.IFuture

	// return data *entities.SellerFinance and error
	Delete(ctx context.Context, finance entities.SellerFinance) future.IFuture

	// return error
	DeleteAllWithFinances(ctx context.Context, finances []entities.SellerFinance) future.IFuture

	// return error
	DeleteAll(ctx context.Context) future.IFuture

	// remove order from db
	// return error
	RemoveById(ctx context.Context, fid string) future.IFuture

	// return error
	Remove(ctx context.Context, finance entities.SellerFinance) future.IFuture

	// return error
	RemoveAllWithFinances(ctx context.Context, finances []entities.SellerFinance) future.IFuture

	// return error
	RemoveAll(ctx context.Context) future.IFuture
}
