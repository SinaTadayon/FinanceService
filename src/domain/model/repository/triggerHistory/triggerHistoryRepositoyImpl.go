package trigger_history_repository

import (
	"context"
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

const (
	DefaultTotalSize = 1024
)

type iTriggerHistoryRepositoryImpl struct {
	mongoAdapter *mongoadapter.Mongo
	database     string
	collection   string
}

var ErrorTotalCountExceeded = errors.New("total count exceeded")
var ErrorPageNotAvailable = errors.New("page not available")

func NewTriggerHistoryRepository(mongoDriver *mongoadapter.Mongo, database, collection string) ITriggerHistoryRepository {
	return &iTriggerHistoryRepositoryImpl{mongoDriver, database, collection}
}

// return data *entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) Save(ctx context.Context, trigger entities.TriggerHistory) future.IFuture {

	trigger.Version = 1
	trigger.DocVersion = entities.TriggerHistoryDocumentVersion
	var insertOneResult, err = repo.mongoAdapter.InsertOne(repo.database, repo.collection, trigger)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Save TriggerHistory Failed")).
			BuildAndSend()
	}
	trigger.ID = insertOneResult.InsertedID.(primitive.ObjectID)
	return future.FactorySync().
		SetData(&trigger).
		BuildAndSend()
}

// return data *entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) Update(ctx context.Context, trigger entities.TriggerHistory) future.IFuture {

	trigger.UpdatedAt = time.Now().UTC()
	currentVersion := trigger.Version
	trigger.Version += 1
	updateOptions := &options.UpdateOptions{}
	updateOptions.SetUpsert(true)
	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection, bson.D{{"triggerName", trigger.TriggerName}, {"deletedAt", nil}, {"version", currentVersion}},
		bson.D{{"$set", trigger}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "TriggerHistory UpdateOne Failed")).
			BuildAndSend()
	}

	if updateResult.MatchedCount != 1 || updateResult.ModifiedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.Wrap(e, "TriggerHistory Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&trigger).
		BuildAndSend()
}

// return data []*entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) FindByName(ctx context.Context, triggerName string) future.IFuture {

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, bson.D{{"triggerName", triggerName}, {"deletedAt", nil}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SchedulerTrigger Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	triggers := make([]*entities.TriggerHistory, 0, DefaultTotalSize)

	// iterate through all documents
	for cursor.Next(ctx) {
		var trigger entities.TriggerHistory
		// decode the document
		if err := cursor.Decode(&trigger); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode TriggerHistory Failed")).
				BuildAndSend()
		}
		triggers = append(triggers, &trigger)
	}

	if len(triggers) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.Wrap(e, "TriggerHistory Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(triggers).
		BuildAndSend()
}

// return data *entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) FindById(ctx context.Context, id primitive.ObjectID) future.IFuture {
	var finance *entities.TriggerHistory
	singleResult := repo.mongoAdapter.FindOne(repo.database, repo.collection, bson.D{{"_id", id}, {"deletedAt", nil}})
	if singleResult.Err() != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetError(future.NotFound, "TriggerHistory Not Found", errors.Wrap(singleResult.Err(), "TriggerHistory Not Found")).
				BuildAndSend()
		}

		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "TriggerHistory FindByNameAndIndex failed")).
			BuildAndSend()
	}

	if err := singleResult.Decode(&finance); err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode TriggerHistory Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(finance).
		BuildAndSend()
}

// ([]*entities.SchedulerTrigger, error)
func (repo iTriggerHistoryRepositoryImpl) FindByFilter(ctx context.Context, supplier func() interface{}) future.IFuture {
	filter := supplier()

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SchedulerTrigger Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	triggers := make([]*entities.TriggerHistory, 0, DefaultTotalSize)

	// iterate through all documents
	for cursor.Next(ctx) {
		var trigger entities.TriggerHistory
		// decode the document
		if err := cursor.Decode(&trigger); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode TriggerHistory Failed")).
				BuildAndSend()
		}
		triggers = append(triggers, &trigger)
	}

	if len(triggers) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.Wrap(e, "TriggerHistory Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(triggers).
		BuildAndSend()
}

// return data []*entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) FindByFilterWithSort(ctx context.Context, supplier func() (filter interface{}, fieldName string, direction int)) future.IFuture {
	filter, fieldName, direction := supplier()
	iFuture := repo.CountWithFilter(ctx, func() interface{} { return filter }).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	total := iFuture.Data().(int64)
	if total == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.New("TriggerHistory Not Found")).
			BuildAndSend()
	}

	sortMap := make(map[string]int)
	sortMap[fieldName] = direction

	optionFind := options.Find()
	optionFind.SetSort(sortMap)

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany TriggerHistory Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	histories := make([]*entities.TriggerHistory, 0, total)

	// iterate through all documents
	for cursor.Next(ctx) {
		var history entities.TriggerHistory
		// decode the document
		if err := cursor.Decode(&history); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode TriggerHistory Failed")).
				BuildAndSend()
		}
		histories = append(histories, &history)
	}

	return future.FactorySync().
		SetData(histories).
		BuildAndSend()
}

// return data TriggerHistoryPageableResult and error
func (repo iTriggerHistoryRepositoryImpl) FindByFilterWithPage(ctx context.Context, supplier func() (filter interface{}), page, perPage int64) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	filter := supplier()
	iFuture := repo.CountWithFilter(ctx, supplier).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.New("TriggerHistory Not Found")).
			BuildAndSend()
	}

	// total 160 page=6 perPage=30
	var availablePages int64

	if totalCount%perPage != 0 {
		availablePages = (totalCount / perPage) + 1
	} else {
		availablePages = totalCount / perPage
	}

	if totalCount < perPage {
		availablePages = 1
	}

	if availablePages < page {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", ErrorPageNotAvailable).
			BuildAndSend()
	}

	var offset = (page - 1) * perPage
	if offset >= totalCount {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", ErrorTotalCountExceeded).
			BuildAndSend()
	}

	optionFind := options.Find()
	optionFind.SetLimit(perPage)
	optionFind.SetSkip(offset)

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany TriggerHistory Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "FindMany TriggerHistory Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	histories := make([]*entities.TriggerHistory, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var history entities.TriggerHistory
		// decode the document
		if err := cursor.Decode(&history); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode TriggerHistory Failed")).
				BuildAndSend()
		}
		histories = append(histories, &history)
	}

	return future.FactorySync().
		SetData(HistoryPageableResult{
			histories,
			totalCount}).
		BuildAndSend()
}

// return data TriggerHistoryPageableResult and error
func (repo iTriggerHistoryRepositoryImpl) FindByFilterWithPageAndSort(ctx context.Context, supplier func() (filter interface{}, fieldName string, direction int), page, perPage int64) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	filter, fieldName, direction := supplier()
	iFuture := repo.CountWithFilter(ctx, func() interface{} { return filter }).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.New("TriggerHistory Not Found")).
			BuildAndSend()
	}

	// total 160 page=6 perPage=30
	var availablePages int64

	if totalCount%perPage != 0 {
		availablePages = (totalCount / perPage) + 1
	} else {
		availablePages = totalCount / perPage
	}

	if totalCount < perPage {
		availablePages = 1
	}

	if availablePages < page {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", ErrorPageNotAvailable).
			BuildAndSend()
	}

	var offset = (page - 1) * perPage
	if offset >= totalCount {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", ErrorTotalCountExceeded).
			BuildAndSend()
	}

	optionFind := options.Find()
	optionFind.SetLimit(perPage)
	optionFind.SetSkip(offset)

	if fieldName != "" {
		sortMap := make(map[string]int)
		sortMap[fieldName] = direction
		optionFind.SetSort(sortMap)
	}

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany TriggerHistory Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "FindMany TriggerHistory Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	histories := make([]*entities.TriggerHistory, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var history entities.TriggerHistory
		// decode the document
		if err := cursor.Decode(&history); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode TriggerHistory Failed")).
				BuildAndSend()
		}
		histories = append(histories, &history)
	}

	return future.FactorySync().
		SetData(HistoryPageableResult{
			histories,
			totalCount}).
		BuildAndSend()
}

// only set DeletedAt field
// return data *entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) DeleteById(ctx context.Context, id primitive.ObjectID) future.IFuture {

	iFuture := repo.FindById(ctx, id).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	trigger := iFuture.Data().(*entities.TriggerHistory)
	deletedAt := time.Now().UTC()
	trigger.DeletedAt = &deletedAt
	currentVersion := trigger.Version
	trigger.Version += 1

	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection,
		bson.D{{"_id", id}, {"deletedAt", nil}, {"version", currentVersion}},
		bson.D{{"$set", trigger}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "UpdateOne TriggerHistory Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 || updateResult.MatchedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.Wrap(e, "UpdateOne TriggerHistory Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(trigger).
		BuildAndSend()
}

// return data *entities.TriggerHistory and error
func (repo iTriggerHistoryRepositoryImpl) Delete(ctx context.Context, history entities.TriggerHistory) future.IFuture {
	deletedAt := time.Now().UTC()
	history.DeletedAt = &deletedAt
	currentVersion := history.Version
	history.Version += 1

	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection,
		bson.D{{"_id", history.ID}, {"deletedAt", nil}, {"version", currentVersion}},
		bson.D{{"$set", history}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "UpdateOne TriggerHistory Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 || updateResult.MatchedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "TriggerHistory Not Found", errors.Wrap(e, "UpdateOne TriggerHistory Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&history).
		BuildAndSend()
}

// error
func (repo iTriggerHistoryRepositoryImpl) RemoveAll(ctx context.Context) future.IFuture {
	_, err := repo.mongoAdapter.DeleteMany(repo.database, repo.collection, bson.M{})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "DeleteMany SchedulerTrigger Failed")).
			BuildAndSend()
	}
	return nil
}

// (int64, error)
func (repo iTriggerHistoryRepositoryImpl) Count(ctx context.Context) future.IFuture {
	total, err := repo.mongoAdapter.Count(repo.database, repo.collection, bson.D{{"deletedAt", nil}})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Count TriggerHistory Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(total).
		BuildAndSend()
}

// (int64, error)
func (repo iTriggerHistoryRepositoryImpl) CountWithFilter(ctx context.Context, supplier func() interface{}) future.IFuture {
	total, err := repo.mongoAdapter.Count(repo.database, repo.collection, supplier())
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "CountWithFilter TriggerHistory Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(total).
		BuildAndSend()
}

func closeCursor(context context.Context, cursor *mongo.Cursor) {
	err := cursor.Close(context)
	if err != nil {
		log.GLog.Logger.Error("cursor.Close failed", "error", err)
	}
}
