package trigger_repository

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
	"time"
)

const (
	defaultTotalSize = 1024
)

type iSchedulerTriggerRepositoryImpl struct {
	mongoAdapter *mongoadapter.Mongo
	database     string
	collection   string
}

var ErrorTotalCountExceeded = errors.New("total count exceeded")
var ErrorPageNotAvailable = errors.New("page not available")
var ErrorDeleteFailed = errors.New("update deletedAt field failed")
var ErrorRemoveFailed = errors.New("remove FinanceTrigger failed")
var ErrorUpdateFailed = errors.New("update FinanceTrigger failed")

func NewSchedulerTriggerRepository(mongoDriver *mongoadapter.Mongo, database, collection string) ISchedulerTriggerRepository {
	return &iSchedulerTriggerRepositoryImpl{mongoDriver, database, collection}
}

// (*entities.SellerFinance, error)
func (repo iSchedulerTriggerRepositoryImpl) Save(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture {

	trigger.Version = 1
	trigger.DocVersion = entities.TriggerDocumentVersion
	var insertOneResult, err = repo.mongoAdapter.InsertOne(repo.database, repo.collection, trigger)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Save FinanceTrigger Failed")).
			BuildAndSend()
	}
	trigger.ID = insertOneResult.InsertedID.(primitive.ObjectID)
	return future.FactorySync().
		SetData(&trigger).
		BuildAndSend()
}

// return (*entities.FinanceTrigger, error)
func (repo iSchedulerTriggerRepositoryImpl) Update(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture {

	trigger.UpdatedAt = time.Now().UTC()
	currentVersion := trigger.Version
	trigger.Version += 1
	//updateOptions := &options.UpdateOptions{}
	//updateOptions.SetUpsert(true)
	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection, bson.D{{"name", trigger.Name}, {"deletedAt", nil}, {"version", currentVersion}},
		bson.D{{"$set", trigger}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FinanceTrigger UpdateOne Failed")).
			BuildAndSend()
	}

	if updateResult.MatchedCount != 1 || updateResult.ModifiedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(e, "FinanceTrigger Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&trigger).
		BuildAndSend()
}

// (*entities.FinanceTrigger, error)
func (repo iSchedulerTriggerRepositoryImpl) FindActiveTrigger(ctx context.Context, triggerType entities.TriggerType) future.IFuture {
	var trigger *entities.FinanceTrigger
	singleResult := repo.mongoAdapter.FindOne(repo.database, repo.collection, bson.D{{"isActive", true}, {"type", triggerType}, {"deletedAt", nil}})
	if singleResult.Err() != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(singleResult.Err(), "FinanceTrigger Not Found")).
				BuildAndSend()
		}

		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "FindById FinanceTrigger failed")).
			BuildAndSend()
	}

	if err := singleResult.Decode(&trigger); err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode FinanceTrigger Failed")).
			BuildAndSend()
	}

	if trigger.Name == "" {
		return future.FactorySync().
			SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(singleResult.Err(), "FinanceTrigger Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(trigger).
		BuildAndSend()
}

// (*entities.FinanceTrigger, error)
func (repo iSchedulerTriggerRepositoryImpl) FindByName(ctx context.Context, name string) future.IFuture {
	var trigger *entities.FinanceTrigger
	singleResult := repo.mongoAdapter.FindOne(repo.database, repo.collection, bson.D{{"name", name}, {"deletedAt", nil}})
	if singleResult.Err() != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(singleResult.Err(), "FinanceTrigger Not Found")).
				BuildAndSend()
		}

		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "FindByName FinanceTrigger failed")).
			BuildAndSend()
	}

	if err := singleResult.Decode(&trigger); err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode FinanceTrigger Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(trigger).
		BuildAndSend()
}

// ([]*entities.FinanceTrigger, error)
func (repo iSchedulerTriggerRepositoryImpl) FindByFilter(ctx context.Context, supplier func() interface{}) future.IFuture {
	filter := supplier()

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany FinanceTrigger Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	triggers := make([]*entities.FinanceTrigger, 0, defaultTotalSize)

	// iterate through all documents
	for cursor.Next(ctx) {
		var trigger entities.FinanceTrigger
		// decode the document
		if err := cursor.Decode(&trigger); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode FinanceTrigger Failed")).
				BuildAndSend()
		}
		triggers = append(triggers, &trigger)
	}

	if len(triggers) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(e, "FinanceTrigger Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(triggers).
		BuildAndSend()
}

// (*entities.SellerFinance, error)
func (repo iSchedulerTriggerRepositoryImpl) DeleteByName(ctx context.Context, name string) future.IFuture {

	iFuture := repo.FindByName(ctx, name).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	trigger := iFuture.Data().(*entities.FinanceTrigger)
	deletedAt := time.Now().UTC()
	trigger.DeletedAt = &deletedAt
	currentVersion := trigger.Version
	trigger.IsEnable = false
	trigger.IsActive = false
	trigger.Version += 1

	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection,
		bson.D{{"name", trigger.Name}, {"deletedAt", nil}, {"version", currentVersion}},
		bson.D{{"$set", trigger}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "UpdateOne FinanceTrigger Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 || updateResult.MatchedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(e, "UpdateOne FinanceTrigger Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(trigger).
		BuildAndSend()
}

// (*entities.SellerFinance, error)
func (repo iSchedulerTriggerRepositoryImpl) Delete(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture {
	deletedAt := time.Now().UTC()
	trigger.DeletedAt = &deletedAt
	currentVersion := trigger.Version
	trigger.IsEnable = false
	trigger.IsActive = false
	trigger.Version += 1

	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection,
		bson.D{{"name", trigger.Name}, {"deletedAt", nil}, {"version", currentVersion}},
		bson.D{{"$set", trigger}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "UpdateOne FinanceTrigger Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 || updateResult.MatchedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "FinanceTrigger Not Found", errors.Wrap(e, "UpdateOne FinanceTrigger Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&trigger).
		BuildAndSend()
}

// error
func (repo iSchedulerTriggerRepositoryImpl) RemoveByName(ctx context.Context, name string) future.IFuture {
	result, err := repo.mongoAdapter.DeleteOne(repo.database, repo.collection, bson.M{"name": name})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "DeleteOne FinanceTrigger Failed")).
			BuildAndSend()
	}

	if result.DeletedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "FinanceTrigger Not Found", ErrorRemoveFailed).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(struct{}{}).
		BuildAndSend()
}

// error
func (repo iSchedulerTriggerRepositoryImpl) Remove(ctx context.Context, trigger entities.FinanceTrigger) future.IFuture {
	return repo.RemoveByName(ctx, trigger.Name)
}

// error
func (repo iSchedulerTriggerRepositoryImpl) RemoveAll(ctx context.Context) future.IFuture {
	_, err := repo.mongoAdapter.DeleteMany(repo.database, repo.collection, bson.M{})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "DeleteMany FinanceTrigger Failed")).
			BuildAndSend()
	}
	return nil
}

// (int64, error)
func (repo iSchedulerTriggerRepositoryImpl) Count(ctx context.Context) future.IFuture {
	total, err := repo.mongoAdapter.Count(repo.database, repo.collection, bson.D{{"deletedAt", nil}})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Count FinanceTrigger Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(total).
		BuildAndSend()
}

// (int64, error)
func (repo iSchedulerTriggerRepositoryImpl) CountWithFilter(ctx context.Context, supplier func() interface{}) future.IFuture {
	total, err := repo.mongoAdapter.Count(repo.database, repo.collection, supplier())
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "CountWithFilter FinanceTrigger Failed")).
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
