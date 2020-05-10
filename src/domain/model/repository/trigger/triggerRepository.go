package trigger_repository

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type ISchedulerTriggerRepository interface {
	// return data *entities.SchedulerTrigger and error
	Save(ctx context.Context, trigger entities.SchedulerTrigger) future.IFuture

	// return data *entities.SchedulerTrigger and error
	Update(ctx context.Context, trigger entities.SchedulerTrigger) future.IFuture

	// return data *entities.SchedulerTrigger and error
	FindByName(ctx context.Context, name string) future.IFuture

	// return data *entities.SchedulerTrigger and error
	FindEnabled(ctx context.Context) future.IFuture

	// return data []*entities.SchedulerTrigger and error
	FindByFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture

	// only set DeletedAt field
	// return data *entities.SchedulerTrigger and error
	DeleteByName(ctx context.Context, name string) future.IFuture

	// return data *entities.SchedulerTrigger and error
	Delete(ctx context.Context, trigger entities.SchedulerTrigger) future.IFuture

	// remove order from db
	// return error
	RemoveByName(ctx context.Context, name string) future.IFuture

	// return error
	Remove(ctx context.Context, trigger entities.SchedulerTrigger) future.IFuture

	// return error
	RemoveAll(ctx context.Context) future.IFuture

	// return data int64 and error
	Count(ctx context.Context) future.IFuture

	// return data int64 and error
	CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture
}
