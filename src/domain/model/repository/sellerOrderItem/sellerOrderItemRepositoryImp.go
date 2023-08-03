package sellerOrderItem

import (
	"context"
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var ErrorTotalCountExceeded = errors.New("total count exceeded")
var ErrorPageNotAvailable = errors.New("page not available")

type iSellerOrderItemRepositoryImp struct {
	mongoAdapter *mongoadapter.Mongo
	database     string
	collection   string
}

func NewSellerOrderItemRepository(mongoDriver *mongoadapter.Mongo, database, collection string) ISellerOrderItemRepository {
	// the cost and benefits of memory allocation is obvious
	ins := iSellerOrderItemRepositoryImp{
		mongoDriver,
		database,
		collection,
	}

	return &ins
}

func (repo iSellerOrderItemRepositoryImp) FindOrderItemsByFilterWithPage(ctx context.Context, totalSupplier func() (filter interface{}), supplier func() (filter interface{}), page, perPage int64, sortName string, sortDir int) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	if sortName == "" {
		sortDir = 0
	} else if sortDir != -1 && sortDir != 1 {
		return future.FactorySync().
			SetError(future.BadRequest, "Invalid sort direction", errors.New("Invalid sort direction")).
			BuildAndSend()
	}

	iFuture := repo.CountWithFilter(ctx, totalSupplier).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerOrderItem Not Found", errors.New("SellerOrderItem Not Found")).
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

	filterWithPageableAndSort := supplier().([]bson.M)
	filterWithPageableAndSort = append(filterWithPageableAndSort, bson.M{"$skip": offset}, bson.M{"$limit": perPage})
	filterWithPageableAndSort = append(filterWithPageableAndSort, bson.M{"$sort": bson.D{{sortName, sortDir}}})

	cursor, e := repo.mongoAdapter.Aggregate(repo.database, repo.collection, filterWithPageableAndSort)

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
	orders := make([]*entities.SellerFinanceOrderItem, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var order entities.SellerFinanceOrderItem
		// decode the document
		if err := cursor.Decode(&order); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrderItem Failed")).
				BuildAndSend()
		}
		orders = append(orders, &order)
	}

	return future.FactorySync().
		SetData(SellerOrderItems{
			orders,
			totalCount}).
		BuildAndSend()
}

func (repo iSellerOrderItemRepositoryImp) CountWithFilter(ctx context.Context, supplier func() (filter interface{})) future.IFuture {
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
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerOrderItem Failed")).
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
