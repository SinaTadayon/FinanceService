package sellerFinance

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
	"strconv"
	"time"
)

type iSellerFinanceRepositoryImpl struct {
	mongoAdapter *mongoadapter.Mongo
	database     string
	collection   string
}

var ErrorTotalCountExceeded = errors.New("total count exceeded")
var ErrorPageNotAvailable = errors.New("page not available")
var ErrorDeleteFailed = errors.New("update deletedAt field failed")
var ErrorRemoveFailed = errors.New("remove subpackage failed")
var ErrorUpdateFailed = errors.New("update subpackage failed")

func NewSellerFinanceRepository(mongoDriver *mongoadapter.Mongo, database, collection string) ISellerFinanceRepository {
	return &iSellerFinanceRepositoryImpl{mongoDriver, database, collection}
}

func (repo iSellerFinanceRepositoryImpl) generateId(sellerId uint64) string {
	return strconv.Itoa(int(sellerId)) + strconv.Itoa(int(time.Now().UnixNano()/1000))
}

// (*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) Save(ctx context.Context, finance entities.SellerFinance) future.IFuture {

	if finance.FId == "" {
		finance.FId = repo.generateId(finance.SellerId)
		for i := 0; i < len(finance.Orders); i++ {
			finance.Orders[i].FId = finance.FId
		}

		var insertOneResult, err = repo.mongoAdapter.InsertOne(repo.database, repo.collection, finance)
		if err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Save SellerFinance Failed")).
				BuildAndSend()
		}
		finance.ID = insertOneResult.InsertedID.(primitive.ObjectID)
		return future.FactorySync().
			SetData(&finance).
			BuildAndSend()
	} else {
		finance.UpdatedAt = time.Now().UTC()
		currentVersion := finance.Version
		finance.Version += 1
		updateOptions := &options.UpdateOptions{}
		updateOptions.SetUpsert(true)
		updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection, bson.D{{"fid", finance.FId}, {"deletedAt", nil}, {"version", currentVersion}},
			bson.D{{"$set", finance}}, updateOptions)
		if e != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "SellerFinance UpdateOne Failed")).
				BuildAndSend()
		}

		if updateResult.MatchedCount != 1 || updateResult.ModifiedCount != 1 {
			return future.FactorySync().
				SetError(future.NotFound, "SellerFinance Not Found", errors.Wrap(e, "SellerFinance Not Found")).
				BuildAndSend()
		}
	}

	return future.FactorySync().
		SetData(&finance).
		BuildAndSend()
}

// return ([]*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) SaveAll(ctx context.Context, finances []entities.SellerFinance) future.IFuture {
	panic("implementation required")
}

// return error
func (repo iSellerFinanceRepositoryImpl) UpdateStatus(ctx context.Context, finance *entities.SellerFinance) future.IFuture {
	finance.UpdatedAt = time.Now().UTC()
	currentVersion := finance.Version
	finance.Version += 1
	opt := options.FindOneAndUpdate()
	opt.SetUpsert(false)
	singleResult := repo.mongoAdapter.GetConn().Database(repo.database).Collection(repo.collection).FindOneAndUpdate(ctx,
		bson.D{
			{"fid", finance.FId},
			{"version", currentVersion},
		},
		bson.D{{"$set", bson.D{{"version", finance.Version}, {"status", finance.Status}, {"updatedAt", finance.UpdatedAt}}}}, opt)
	if singleResult.Err() != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetError(future.NotFound, "SellerFinance Not Found", errors.Wrap(singleResult.Err(), "SellerFinance Not Found")).
				BuildAndSend()
		}
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "SellerFinance UpdateOne Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(struct{}{}).
		BuildAndSend()
}

// return (*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) Insert(ctx context.Context, finance entities.SellerFinance) future.IFuture {

	if finance.FId == "" {
		finance.FId = repo.generateId(finance.SellerId)
		for i := 0; i < len(finance.Orders); i++ {
			finance.Orders[i].FId = finance.FId
		}

		var insertOneResult, err = repo.mongoAdapter.InsertOne(repo.database, repo.collection, finance)
		if err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Save SellerFinance Failed")).
				BuildAndSend()
		}
		finance.ID = insertOneResult.InsertedID.(primitive.ObjectID)
		return future.FactorySync().
			SetData(&finance).
			BuildAndSend()
	} else {
		var insertOneResult, err = repo.mongoAdapter.InsertOne(repo.database, repo.collection, &finance)
		if err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Insert SellerFinance Failed")).
				BuildAndSend()
		}
		finance.ID = insertOneResult.InsertedID.(primitive.ObjectID)
	}
	return future.FactorySync().
		SetData(&finance).
		BuildAndSend()
}

// ([]*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) InsertAll(ctx context.Context, finances []entities.SellerFinance) future.IFuture {
	panic("implementation required")
}

// ([]*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) FindAll(ctx context.Context) future.IFuture {
	iFuture := repo.Count(ctx).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	total := iFuture.Data().(int64)
	if total == 0 {
		return future.FactorySync().SetData(struct{}{}).BuildAndSend()
	}

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, bson.D{{"deletedAt", nil}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, total)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(finances).
		BuildAndSend()
}

// ([]*entities.SellerFinance, repository.IRepoError)
func (repo iSellerFinanceRepositoryImpl) FindAllWithSort(ctx context.Context, fieldName string, direction int) future.IFuture {
	iFuture := repo.Count(ctx).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	total := iFuture.Data().(int64)
	if total == 0 {
		return future.FactorySync().SetData(total).BuildAndSend()
	}

	sortMap := make(map[string]int)
	sortMap[fieldName] = direction

	optionFind := options.Find()
	optionFind.SetSort(sortMap)

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, bson.D{{"deletedAt", nil}}, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, total)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(finances).
		BuildAndSend()
}

// ([]*entities.SellerFinance, int64, error)
func (repo iSellerFinanceRepositoryImpl) FindAllWithPage(ctx context.Context, page, perPage int64) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	iFuture := repo.Count(ctx).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().SetData(totalCount).BuildAndSend()
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

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, bson.D{{"deletedAt", nil}}, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(FinancePageableResult{
			finances,
			totalCount}).
		BuildAndSend()
}

// ([]*entities.SellerFinance, int64, error)
func (repo iSellerFinanceRepositoryImpl) FindAllWithPageAndSort(ctx context.Context, page, perPage int64, fieldName string, direction int) future.IFuture {
	if page <= 0 || perPage <= 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Request Operation Failed", errors.New("Page or PerPage Invalid")).
			BuildAndSend()
	}

	iFuture := repo.Count(ctx).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	totalCount := iFuture.Data().(int64)
	if totalCount == 0 {
		return future.FactorySync().SetData(totalCount).BuildAndSend()
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

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, bson.D{{"deletedAt", nil}}, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(FinancePageableResult{
			finances,
			totalCount}).
		BuildAndSend()
}

//([]*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) FindAllById(ctx context.Context, fids ...string) future.IFuture {
	panic("implementation required")
}

// (*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) FindById(ctx context.Context, fid string) future.IFuture {
	var finance *entities.SellerFinance
	singleResult := repo.mongoAdapter.FindOne(repo.database, repo.collection, bson.D{{"fid", fid}, {"deletedAt", nil}})
	if singleResult.Err() != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetError(future.NotFound, "SellerFinance Not Found", errors.Wrap(singleResult.Err(), "SellerFinance Not Found")).
				BuildAndSend()
		}

		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "FindById failed")).
			BuildAndSend()
	}

	if err := singleResult.Decode(&finance); err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(finance).
		BuildAndSend()
}

// ([]*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) FindByFilter(ctx context.Context, supplier func() interface{}) future.IFuture {
	filter := supplier()

	iFuture := repo.CountWithFilter(ctx, supplier).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	total := iFuture.Data().(int64)
	if total == 0 {
		return future.FactorySync().SetData(total).BuildAndSend()
	}

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, total)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(finances).
		BuildAndSend()
}

// ([]*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) FindByFilterWithSort(ctx context.Context, supplier func() (interface{}, string, int)) future.IFuture {
	filter, fieldName, direction := supplier()
	iFuture := repo.CountWithFilter(ctx, func() interface{} { return filter }).Get()

	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	total := iFuture.Data().(int64)
	if total == 0 {
		return future.FactorySync().SetData(total).BuildAndSend()
	}

	sortMap := make(map[string]int)
	sortMap[fieldName] = direction

	optionFind := options.Find()
	optionFind.SetSort(sortMap)

	cursor, e := repo.mongoAdapter.FindMany(repo.database, repo.collection, filter, optionFind)
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, total)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(finances).
		BuildAndSend()
}

// ([]*entities.SellerFinance, int64, error)
func (repo iSellerFinanceRepositoryImpl) FindByFilterWithPage(ctx context.Context, supplier func() interface{}, page, perPage int64) future.IFuture {

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
		return future.FactorySync().SetData(totalCount).BuildAndSend()
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
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(FinancePageableResult{
			finances,
			totalCount}).
		BuildAndSend()
}

// ([]*entities.SellerFinance, int64, error)
func (repo iSellerFinanceRepositoryImpl) FindByFilterWithPageAndSort(ctx context.Context, supplier func() (interface{}, string, int), page, perPage int64) future.IFuture {
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
		return future.FactorySync().SetData(totalCount).BuildAndSend()
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
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "FindMany SellerFinance Failed")).
			BuildAndSend()
	} else if cursor.Err() != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(cursor.Err(), "FindMany SellerFinance Failed")).
			BuildAndSend()
	}

	defer closeCursor(ctx, cursor)
	finances := make([]*entities.SellerFinance, 0, perPage)

	// iterate through all documents
	for cursor.Next(ctx) {
		var finance entities.SellerFinance
		// decode the document
		if err := cursor.Decode(&finance); err != nil {
			return future.FactorySync().
				SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Decode SellerFinance Failed")).
				BuildAndSend()
		}
		finances = append(finances, &finance)
	}

	return future.FactorySync().
		SetData(FinancePageableResult{
			finances,
			totalCount}).
		BuildAndSend()
}

// (bool, error)
func (repo iSellerFinanceRepositoryImpl) ExistsById(ctx context.Context, fid string) future.IFuture {
	singleResult := repo.mongoAdapter.FindOne(repo.database, repo.collection, bson.D{{"fid", fid}, {"deletedAt", nil}})
	if singleResult.Err() != nil {
		if repo.mongoAdapter.NoDocument(singleResult.Err()) {
			return future.FactorySync().
				SetData(false).
				BuildAndSend()
		}
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(singleResult.Err(), "ExistsById SellerFinance Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(true).
		BuildAndSend()
}

// (int64, error)
func (repo iSellerFinanceRepositoryImpl) Count(ctx context.Context) future.IFuture {
	total, err := repo.mongoAdapter.Count(repo.database, repo.collection, bson.D{{"deletedAt", nil}})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "Count SellerFinance Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(total).
		BuildAndSend()
}

// (int64, error)
func (repo iSellerFinanceRepositoryImpl) CountWithFilter(ctx context.Context, supplier func() interface{}) future.IFuture {
	total, err := repo.mongoAdapter.Count(repo.database, repo.collection, supplier())
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "CountWithFilter SellerFinance Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(total).
		BuildAndSend()
}

// (*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) DeleteById(ctx context.Context, fid string) future.IFuture {

	iFuture := repo.FindById(ctx, fid).Get()
	if iFuture.Error() != nil {
		return future.FactorySyncDataOf(iFuture).BuildAndSend()
	}

	finance := iFuture.Data().(*entities.SellerFinance)
	deletedAt := time.Now().UTC()
	finance.DeletedAt = &deletedAt

	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection,
		bson.D{{"fid", finance.FId}, {"deletedAt", nil}},
		bson.D{{"$set", finance}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "UpdateOne SellerFinance Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 || updateResult.MatchedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerFinance Not Found", errors.Wrap(e, "UpdateOne SellerFinance Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(finance).
		BuildAndSend()
}

// (*entities.SellerFinance, error)
func (repo iSellerFinanceRepositoryImpl) Delete(ctx context.Context, finance entities.SellerFinance) future.IFuture {
	deletedAt := time.Now().UTC()
	finance.DeletedAt = &deletedAt

	updateResult, e := repo.mongoAdapter.UpdateOne(repo.database, repo.collection,
		bson.D{{"fid", finance.FId}, {"deletedAt", nil}},
		bson.D{{"$set", finance}})
	if e != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(e, "UpdateOne SellerFinance Failed")).
			BuildAndSend()
	}

	if updateResult.ModifiedCount != 1 || updateResult.MatchedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerFinance Not Found", errors.Wrap(e, "UpdateOne SellerFinance Failed")).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(&finance).
		BuildAndSend()
}

// error
func (repo iSellerFinanceRepositoryImpl) DeleteAllWithFinances(ctx context.Context, finances []entities.SellerFinance) future.IFuture {
	panic("implementation required")
}

// error
func (repo iSellerFinanceRepositoryImpl) DeleteAll(ctx context.Context) future.IFuture {
	_, err := repo.mongoAdapter.UpdateMany(repo.database, repo.collection,
		bson.D{{"deletedAt", nil}},
		bson.M{"$set": bson.M{"deletedAt": time.Now().UTC()}})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "UpdateMany SellerFinance Failed")).
			BuildAndSend()
	}
	return future.FactorySync().
		SetData(struct{}{}).
		BuildAndSend()
}

// error
func (repo iSellerFinanceRepositoryImpl) RemoveById(ctx context.Context, fid string) future.IFuture {
	result, err := repo.mongoAdapter.DeleteOne(repo.database, repo.collection, bson.M{"fid": fid})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "DeleteOne SellerFinance Failed")).
			BuildAndSend()
	}

	if result.DeletedCount != 1 {
		return future.FactorySync().
			SetError(future.NotFound, "SellerFinance Not Found", ErrorRemoveFailed).
			BuildAndSend()
	}

	return future.FactorySync().
		SetData(struct{}{}).
		BuildAndSend()
}

// error
func (repo iSellerFinanceRepositoryImpl) Remove(ctx context.Context, finance entities.SellerFinance) future.IFuture {
	return repo.RemoveById(ctx, finance.FId)
}

// error
func (repo iSellerFinanceRepositoryImpl) RemoveAllWithFinances(ctx context.Context, finances []entities.SellerFinance) future.IFuture {
	panic("implementation required")
}

// error
func (repo iSellerFinanceRepositoryImpl) RemoveAll(ctx context.Context) future.IFuture {
	_, err := repo.mongoAdapter.DeleteMany(repo.database, repo.collection, bson.M{})
	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "Request Operation Failed", errors.Wrap(err, "DeleteMany SellerFinance Failed")).
			BuildAndSend()
	}
	return nil
}

func closeCursor(context context.Context, cursor *mongo.Cursor) {
	err := cursor.Close(context)
	if err != nil {
		log.GLog.Logger.Error("cursor.Close failed", "error", err)
	}
}
