package order_repository

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
)

const (
	defaultDocCount int = 1024
)

var ErrorTotalCountExceeded = errors.New("total count exceeded")
var ErrorPageNotAvailable = errors.New("page not available")
var ErrorDeleteFailed = errors.New("update deletedAt field failed")
var ErrorRemoveFailed = errors.New("remove SellerOrder failed")
var ErrorUpdateFailed = errors.New("update SellerOrder failed")

type iSellerOrderRepositoryImpl struct {
	mongoAdapter *mongoadapter.Mongo
	database     string
	collection   string
}

func NewSellerOrderRepository(mongoDriver *mongoadapter.Mongo, database, collection string) ISellerOrderRepository {
	return &iSellerOrderRepositoryImpl{mongoDriver, database, collection}
}

// return data *entities.SellerOrder , error
func (repo iSellerOrderRepositoryImpl) SaveOrderInfo(ctx context.Context, fid string, orderInfo entities.OrderInfo) future.IFuture {
	updateResult, err := repo.mongoAdapter.UpdateOne(repo.database, repo.collection, bson.D{
		{"deletedAt", nil},
		{"fid", fid}},
		bson.D{{"$push", bson.D{{"ordersInfo", orderInfo}}}})

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "UpdateOne SellerOrder Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", ErrorUpdateFailed).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&orderInfo).
		BuildAndSend()
}

// return data *entities.SellerOrder , error
func (repo iSellerOrderRepositoryImpl) SaveOrder(ctx context.Context, triggerHistoryId primitive.ObjectID, order entities.SellerOrder) future.IFuture {
	updateResult, err := repo.mongoAdapter.UpdateOne(repo.database, repo.collection, bson.D{
		{"deletedAt", nil},
		{"fid", order.FId},
		{"ordersInfo.triggerHistory", triggerHistoryId}},
		bson.D{{"$push", bson.D{{"ordersInfo.$.orders", order}}}})

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "UpdateOne SellerOrder Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", ErrorUpdateFailed).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&order).
		BuildAndSend()
}

// return data []*entities.SellerOrder , error
func (repo iSellerOrderRepositoryImpl) SaveAll(ctx context.Context, orders []entities.SellerOrder) future.IFuture {
	panic("must be implement")
}

// return data *entities.SellerOrder, error
func (repo iSellerOrderRepositoryImpl) FindByFIdAndOId(ctx context.Context, fid string, oid uint64) future.IFuture {
	var order entities.SellerOrder
	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "deletedAt": nil, "ordersInfo.orders.oid": oid}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": oid, "deletedAt": nil}},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)

	for cursor.Next(ctx) {
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}
	}

	return future.FactorySync().
		SetData(&order).
		BuildAndSend()
}

// return data *[]entities.SellerOrder, error
func (repo iSellerOrderRepositoryImpl) FindBySellerIdAndOId(ctx context.Context, sellerId, oid uint64) future.IFuture {

	pipeline := []bson.M{
		{"$match": bson.M{"sellerId": sellerId, "ordersInfo.orders.oid": oid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": oid, "deletedAt": nil}},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)

	orders := make([]*entities.SellerOrder, 0, defaultDocCount)

	for cursor.Next(ctx) {
		var order entities.SellerOrder
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}

		orders = append(orders, &order)
	}

	if len(orders) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(orders).
		BuildAndSend()
}

// return data *[]entities.SellerOrder, error
func (repo iSellerOrderRepositoryImpl) FindById(ctx context.Context, oid uint64) future.IFuture {

	pipeline := []bson.M{
		{"$match": bson.M{"ordersInfo.orders.oid": oid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": oid, "deletedAt": nil}},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)

	orders := make([]*entities.SellerOrder, 0, defaultDocCount)

	for cursor.Next(ctx) {
		var order entities.SellerOrder
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}

		orders = append(orders, &order)
	}

	if len(orders) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(orders).
		BuildAndSend()
}

// return data []*entities.SellerOrder, error
func (repo iSellerOrderRepositoryImpl) FindAll(ctx context.Context, fid string) future.IFuture {
	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)

	orders := make([]*entities.SellerOrder, 0, defaultDocCount)

	for cursor.Next(ctx) {
		var subpackage entities.SellerOrder
		if err := cursor.Decode(&subpackage); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}

		orders = append(orders, &subpackage)
	}

	if len(orders) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(orders).
		BuildAndSend()
}

// return data []*entities.SellerOrder, error
func (repo iSellerOrderRepositoryImpl) FindAllWithSort(ctx context.Context, fid string, fieldName string, direction int) future.IFuture {
	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$sort": bson.M{"orders." + fieldName: direction}},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	orders := make([]*entities.SellerOrder, 0, defaultDocCount)
	for cursor.Next(ctx) {
		var subpackage entities.SellerOrder
		if err := cursor.Decode(&subpackage); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}

		orders = append(orders, &subpackage)
	}

	if len(orders) == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(orders).
		BuildAndSend()
}

// return data SellerOrderPageableResult, error
func (repo iSellerOrderRepositoryImpl) FindAllWithPage(ctx context.Context, fid string, page, perPage int64) future.IFuture {

	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	iFuture := repo.Count(ctx, fid).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
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

	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$skip": offset},
		{"$limit": perPage},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, e := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	orders := make([]*entities.SellerOrder, 0, perPage)

	for cursor.Next(ctx) {
		var order entities.SellerOrder
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}

		orders = append(orders, &order)
	}

	return future.FactorySync().
		SetData(SellerOrderPageableResult{
			orders,
			totalCount}).
		BuildAndSend()
}

// return data SellerOrderPageableResult, error
func (repo iSellerOrderRepositoryImpl) FindAllWithPageAndSort(ctx context.Context, fid string, page, perPage int64, fieldName string, direction int) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	iFuture := repo.Count(ctx, fid).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
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

	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$sort": bson.M{"ordersInfo.orders." + fieldName: direction}},
		{"$skip": offset},
		{"$limit": perPage},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	cursor, e := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	orders := make([]*entities.SellerOrder, 0, perPage)
	for cursor.Next(ctx) {
		var order entities.SellerOrder
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}

		orders = append(orders, &order)
	}

	return future.FactorySync().
		SetData(SellerOrderPageableResult{
			orders,
			totalCount}).
		BuildAndSend()
}

// return data []*entities.SellerOrder, error
func (repo iSellerOrderRepositoryImpl) FindByFilter(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{})) future.IFuture {
	filter := supplier()
	iFuture := repo.CountWithFilter(ctx, totalSupplier).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	total := iFuture.Data().(int64)
	if total == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
			BuildAndSend()
	}

	cursor, e := repo.mongoAdapter.Aggregate(repo.database, repo.collection, filter)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	orders := make([]*entities.SellerOrder, 0, total)

	// iterate through all documents
	for cursor.Next(ctx) {
		var order entities.SellerOrder
		// decode the document
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}
		orders = append(orders, &order)
	}

	return future.FactorySync().
		SetData(orders).
		BuildAndSend()
}

// return data SellerOrderPageableResult, error
func (repo iSellerOrderRepositoryImpl) FindByFilterWithPage(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{}), page, perPage int64) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	filter := supplier()
	iFuture := repo.CountWithFilter(ctx, totalSupplier).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrder Not Found", errors.New("SellerOrder Not Found")).
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

	cursor, e := repo.mongoAdapter.Aggregate(repo.database, repo.collection, filter)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "Aggregate Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	orders := make([]*entities.SellerOrder, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var order entities.SellerOrder
		// decode the document
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}
		orders = append(orders, &order)
	}

	return future.FactorySync().
		SetData(SellerOrderPageableResult{
			orders,
			totalCount}).
		BuildAndSend()
}

// return data bool, error
func (repo iSellerOrderRepositoryImpl) ExistsById(ctx context.Context, oid uint64) future.IFuture {
	singleResult := repo.mongoAdapter.FindOne(repo.database, repo.collection, bson.D{{"ordersInfo.orders.oid", oid}, {"deletedAt", nil}})
	if err := singleResult.Err(); err != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetData(false).
				BuildAndSend()
		}
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "ExistsById SellerOrder Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(true).
		BuildAndSend()
}

// return int64, error
func (repo iSellerOrderRepositoryImpl) Count(ctx context.Context, fid string) future.IFuture {
	var total struct {
		Count int
	}

	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$project": bson.M{"subSize": bson.M{"$size": "$ordersInfo.orders"}}},
		{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": "$subSize"}}},
		{"$project": bson.M{"_id": 0, "count": 1}},
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, pipeline)
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)

	if cursor.Next(ctx) {
		if err := cursor.Decode(&total); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}
	}

	return future.FactorySync().
		SetData(int64(total.Count)).
		BuildAndSend()
}

// return int64, error
func (repo iSellerOrderRepositoryImpl) CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture {
	var total struct {
		Count int
	}

	cursor, err := repo.mongoAdapter.Aggregate(repo.database, repo.collection, supplier())
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Aggregate Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	if cursor.Next(ctx) {
		if err := cursor.Decode(&total); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrder Failed")).
				BuildAndSend()
		}
	}

	return future.FactorySync().
		SetData(int64(total.Count)).
		BuildAndSend()
}

func closeCursor(context context.Context, cursor *mongo.Cursor) {
	err := cursor.Close(context)
	if err != nil {
		log.GLog.Logger.Error("cursor.Close failed", "error", err)
	}
}
