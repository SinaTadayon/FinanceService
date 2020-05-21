package trigger_repository

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type ISchedulerTriggerRepository interface {
	// return data *entities.FinanceTrigger and error
	Save(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture

	// return data *entities.FinanceTrigger and error
	Update(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture

	// return data *entities.FinanceTrigger and error
	FindByName(ctx context.Context, name string) future.IFuture

	// return data *entities.FinanceTrigger and error
	FindActiveTrigger(ctx context.Context, triggerType entities.TriggerType) future.IFuture

	// return data []*entities.FinanceTrigger and error
	FindByFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture

	// only set DeletedAt field
	// return data *entities.FinanceTrigger and error
	DeleteByName(ctx context.Context, name string) future.IFuture

	// return data *entities.FinanceTrigger and error
	Delete(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture

	// remove order from db
	// return error
	RemoveByName(ctx context.Context, name string) future.IFuture

	// return error
	Remove(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture

	// return error
	RemoveAll(ctx context.Context) future.IFuture

	// return data int64 and error
	Count(ctx context.Context) future.IFuture

	// return data int64 and error
	CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture
}
