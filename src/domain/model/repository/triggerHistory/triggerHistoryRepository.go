package trigger_history_repository

import (
	"context"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type HistoryPageableResult struct {
	Histories  []*entities.TriggerHistory
	TotalCount int64
}

type ITriggerHistoryRepository interface {
	// return data *entities.TriggerHistory and error
	Save(ctx context.Context, history entities.TriggerHistory) future.IFuture

	// return data *entities.TriggerHistory and error
	Update(ctx context.Context, history entities.TriggerHistory) future.IFuture

	// return data []*entities.TriggerHistory and error
	FindByName(ctx context.Context, triggerName string) future.IFuture

	// return data *entities.TriggerHistory and error
	FindById(ctx context.Context, id primitive.ObjectID) future.IFuture

	// return data []*entities.TriggerHistory and error
	FindByFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture

	// return data []*entities.TriggerHistory and error
	FindByFilterWithSort(ctx context.Context, supplier func() (filter interface{}, fieldName string, direction int)) future.IFuture

	// return data TriggerHistoryPageableResult and error
	FindByFilterWithPage(ctx context.Context, supplier func() (filter interface{}), page, perPage int64) future.IFuture

	// return data TriggerHistoryPageableResult and error
	FindByFilterWithPageAndSort(ctx context.Context, supplier func() (filter interface{}, fieldName string, direction int), page, perPage int64) future.IFuture

	// return data bool and error
	ExistsByTriggeredAt(ctx context.Context, triggeredAt time.Time) future.IFuture

	// only set DeletedAt field
	// return data *entities.TriggerHistory and error
	DeleteById(ctx context.Context, id primitive.ObjectID) future.IFuture

	// return data *entities.TriggerHistory and error
	Delete(ctx context.Context, trigger entities.TriggerHistory) future.IFuture

	// return error
	RemoveAll(ctx context.Context) future.IFuture

	// return data int64 and error
	Count(ctx context.Context) future.IFuture

	// return data int64 and error
	CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture
}
