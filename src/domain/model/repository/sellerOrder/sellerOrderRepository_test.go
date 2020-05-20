package order_repository

import (
	"context"
	"github.com/stretchr/testify/require"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"os"
	"strconv"
	"testing"
	"time"
)

var sellerOrderRepo ISellerOrderRepository
var mongoAdapter *mongoadapter.Mongo
var config *configs.Config

func TestMain(m *testing.M) {
	var err error
	var path string
	if os.Getenv("APP_MODE") == "dev" {
		path = "../../../../testdata/.env"
	} else {
		path = ""
	}

	log.GLog.ZapLogger = log.InitZap()
	log.GLog.Logger = logger.NewZapLogger(log.GLog.ZapLogger)

	config, err = configs.LoadConfigs(path)
	if err != nil {
		log.GLog.Logger.Error("configs.LoadConfig failed",
			"error", err)
		os.Exit(1)
	}

	// store in mongo
	mongoConf := &mongoadapter.MongoConfig{
		Host:     config.Mongo.Host,
		Port:     config.Mongo.Port,
		Username: config.Mongo.User,
		//Password:     App.Cfg.Mongo.Pass,
		ConnTimeout:     time.Duration(config.Mongo.ConnectionTimeout) * time.Second,
		ReadTimeout:     time.Duration(config.Mongo.ReadTimeout) * time.Second,
		WriteTimeout:    time.Duration(config.Mongo.WriteTimeout) * time.Second,
		MaxConnIdleTime: time.Duration(config.Mongo.MaxConnIdleTime) * time.Second,
		MaxPoolSize:     uint64(config.Mongo.MaxPoolSize),
		MinPoolSize:     uint64(config.Mongo.MinPoolSize),
		WriteConcernW:   config.Mongo.WriteConcernW,
		WriteConcernJ:   config.Mongo.WriteConcernJ,
		RetryWrites:     config.Mongo.RetryWrite,
	}

	mongoAdapter, err = mongoadapter.NewMongo(mongoConf)
	if err != nil {
		log.GLog.Logger.Error("mongoadapter.NewMongo failed", "error", err)
		os.Exit(1)
	}

	sellerOrderRepo = NewSellerOrderRepository(mongoAdapter, config.Mongo.Database, config.Mongo.SellerCollection)

	// Running Tests
	code := m.Run()
	removeCollection()
	os.Exit(code)
}

func TestSaveOrder(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerFinance fid not generated")
	ctx, _ := context.WithCancel(context.Background())
	finance1 := createFinance()
	orderFinance := finance1.OrdersInfo[0].Orders[0]
	orderFinance.OId = 999999999
	orderFinance.FId = finance.FId
	iFuture := sellerOrderRepo.SaveOrder(ctx, finance.OrdersInfo[0].TriggerHistoryId, *orderFinance).Get()
	require.Nil(t, iFuture.Error())
	updateFinance, err := getSellerFinance(finance.FId)
	require.Nil(t, err)
	require.Equal(t, 3, len(updateFinance.OrdersInfo[0].Orders))
}

func TestSaveOrderInfo(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerFinance fid not generated")
	ctx, _ := context.WithCancel(context.Background())
	finance1 := createFinance()
	orderInfo := finance1.OrdersInfo[0]
	orderInfo.TriggerHistoryId = primitive.NewObjectID()
	iFuture := sellerOrderRepo.SaveOrderInfo(ctx, finance.FId, *orderInfo).Get()
	require.Nil(t, iFuture.Error())
	updateFinance, err := getSellerFinance(finance.FId)
	require.Nil(t, err)
	require.Equal(t, 2, len(updateFinance.OrdersInfo))
	require.Equal(t, orderInfo.TriggerHistoryId, updateFinance.OrdersInfo[1].TriggerHistoryId)
}

func TestFindByFIdAndOId(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerFinance fid not generated")
	ctx, _ := context.WithCancel(context.Background())
	iFuture := sellerOrderRepo.FindByFIdAndOId(ctx, finance.FId, finance.OrdersInfo[0].Orders[1].OId).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, finance.OrdersInfo[0].Orders[1].OId, iFuture.Data().(*entities.SellerOrder).OId)
}

func TestFindBySellerIdAndOId(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	defer removeCollection()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := sellerOrderRepo.FindBySellerIdAndOId(ctx, finance.SellerId, finance.OrdersInfo[0].Orders[0].OId).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, finance.OrdersInfo[0].Orders[0].OId, iFuture.Data().([]*entities.SellerOrder)[0].OId)
}

func TestFindById(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	defer removeCollection()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := sellerOrderRepo.FindById(ctx, finance.OrdersInfo[0].Orders[0].OId).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, finance.OrdersInfo[0].Orders[0].OId, iFuture.Data().([]*entities.SellerOrder)[0].OId)
}

func TestFindAll(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")

	finance, err = createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	ctx, _ := context.WithCancel(context.Background())

	iFuture := sellerOrderRepo.FindAll(ctx, finance.FId).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, finance.OrdersInfo[0].Orders[0].OId, iFuture.Data().([]*entities.SellerOrder)[0].OId)
	require.Equal(t, finance.OrdersInfo[0].Orders[1].OId, iFuture.Data().([]*entities.SellerOrder)[1].OId)
}

func TestFindAllWithSort(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	ctx, _ := context.WithCancel(context.Background())
	iFuture := sellerOrderRepo.FindAllWithSort(ctx, finance.FId, "orders.oid", -1).Get()

	require.Nil(t, iFuture.Error())
	require.Equal(t, finance.OrdersInfo[0].Orders[0].OId, iFuture.Data().([]*entities.SellerOrder)[0].OId)
}

func TestFindAllWithPage(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	ctx, _ := context.WithCancel(context.Background())

	iFuture := sellerOrderRepo.FindAllWithPage(ctx, finance.FId, 1, 1).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, finance.OrdersInfo[0].Orders[0].OId, iFuture.Data().(SellerOrderPageableResult).SellerOrders[0].OId)
	require.Equal(t, 1, len(iFuture.Data().(SellerOrderPageableResult).SellerOrders))
	require.Equal(t, int64(2), iFuture.Data().(SellerOrderPageableResult).TotalCount)
}

func TestFindAllWithPageAndSort(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	ctx, _ := context.WithCancel(context.Background())

	iFuture := sellerOrderRepo.FindAllWithPageAndSort(ctx, finance.FId, 1, 1, "orders.oid", -1).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 1, len(iFuture.Data().(SellerOrderPageableResult).SellerOrders))
	require.Equal(t, int64(2), iFuture.Data().(SellerOrderPageableResult).TotalCount)
	require.Equal(t, finance.OrdersInfo[0].Orders[0].OId, iFuture.Data().(SellerOrderPageableResult).SellerOrders[0].OId)
}

func TestFindByFilter(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")

	finance, err = createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")

	ctx, _ := context.WithCancel(context.Background())
	totalPipeline := []bson.M{
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": 1}}},
		{"$project": bson.M{"_id": 0, "count": 1}},
	}
	pipeline := []bson.M{
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}

	iFuture := sellerOrderRepo.FindByFilter(ctx, func() (filter interface{}) { return totalPipeline }, func() (filter interface{}) { return pipeline }).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 2, len(iFuture.Data().([]*entities.SellerOrder)))
}

func TestFindByFilterWithPage(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")

	finance, err = createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")

	ctx, _ := context.WithCancel(context.Background())
	totalPipeline := []bson.M{
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": 1}}},
		{"$project": bson.M{"_id": 0, "count": 1}},
	}

	pipeline := []bson.M{
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$match": bson.M{"ordersInfo.orders.oid": finance.OrdersInfo[0].Orders[1].OId, "ordersInfo.orders.deletedAt": nil}},
		{"$project": bson.M{"_id": 0, "ordersInfo.orders": 1}},
		{"$skip": 0},
		{"$limit": 1},
		{"$replaceRoot": bson.M{"newRoot": "$ordersInfo"}},
		{"$replaceRoot": bson.M{"newRoot": "$orders"}},
	}
	iFuture := sellerOrderRepo.FindByFilterWithPage(ctx, func() (filter interface{}) { return totalPipeline }, func() (filter interface{}) { return pipeline }, 1, 2).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 1, len(iFuture.Data().(SellerOrderPageableResult).SellerOrders))
	require.Equal(t, int64(2), iFuture.Data().(SellerOrderPageableResult).TotalCount)
	require.Equal(t, finance.OrdersInfo[0].Orders[1].OId, iFuture.Data().(SellerOrderPageableResult).SellerOrders[0].OId)
}

func TestExitsById_Success(t *testing.T) {
	defer removeCollection()
	finance, err := createFinanceAndSave()
	require.Nil(t, err, "createFinanceAndSave failed")
	require.NotEmpty(t, finance.FId, "createFinanceAndSave failed, sellerOrder id not generated")
	ctx, _ := context.WithCancel(context.Background())
	iFuture := sellerOrderRepo.ExistsById(ctx, finance.OrdersInfo[0].Orders[0].OId).Get()
	require.Nil(t, iFuture.Error())
	require.True(t, iFuture.Data().(bool))
}

func insert(finance *entities.SellerFinance) (*entities.SellerFinance, error) {
	finance.FId = strconv.Itoa(int(finance.SellerId)) + strconv.Itoa(int(time.Now().UnixNano()/1000))
	for j := 0; j < len(finance.OrdersInfo); j++ {
		for i := 0; i < len(finance.OrdersInfo[j].Orders); i++ {
			finance.OrdersInfo[j].Orders[i].FId = finance.FId
		}
	}
	var insertOneResult, err = mongoAdapter.InsertOne(config.Mongo.Database, config.Mongo.SellerCollection, finance)
	if err != nil {
		return nil, err
	}
	finance.ID = insertOneResult.InsertedID.(primitive.ObjectID)
	return finance, nil
}

func createFinanceAndSave() (*entities.SellerFinance, error) {
	return insert(createFinance())
}

func removeCollection() {
	if _, err := mongoAdapter.DeleteMany(config.Mongo.Database, config.Mongo.SellerCollection, bson.M{}); err != nil {
	}
}

func createFinance() *entities.SellerFinance {
	return &entities.SellerFinance{
		FId:        "",
		SellerId:   100002,
		Version:    1,
		DocVersion: entities.FinanceDocumentVersion,
		SellerInfo: &entities.SellerProfile{
			SellerId: 100002,
			GeneralInfo: &entities.GeneralSellerInfo{
				ShopDisplayName:          "LG",
				Type:                     "Test",
				Email:                    "Test@gmail.com",
				LandPhone:                "0218283742",
				MobilePhone:              "0912822732",
				Website:                  "test@bazlia.com",
				Province:                 "Tehran",
				City:                     "Tehran",
				Neighborhood:             "Tehran",
				PostalAddress:            "Valiassr S.T",
				PostalCode:               "1928304755",
				IsVATObliged:             true,
				VATCertificationImageURL: "",
			},
			FinanceData: &entities.SellerFinanceData{
				Iban:                    "IR93240239820384024",
				AccountHolderFirstName:  "",
				AccountHolderFamilyName: "",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Invoice: &entities.Invoice{
			SSORawTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			SSORoundupTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			VATRawTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			VATRoundupTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			CommissionRawTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			CommissionRoundupTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			ShareRawTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			ShareRoundupTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			ShipmentRawTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
			ShipmentRoundupTotal: &entities.Money{
				Amount:   "1650000",
				Currency: "IRR",
			},
		},
		OrdersInfo: []*entities.OrderInfo{
			{
				TriggerName:      "SCH4",
				TriggerHistoryId: primitive.NewObjectID(),
				Orders: []*entities.SellerOrder{
					{
						OId:      1111111111,
						FId:      "",
						SellerId: 100002,
						ShipmentAmount: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawShippingNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupShippingNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						IsAlreadyShippingPay: false,
						Items: []*entities.SellerItem{
							{
								SId:         1111111111222,
								SKU:         "yt545-34",
								InventoryId: "666777888999",
								Title:       "Mobile",
								Brand:       "Nokia",
								Guaranty:    "Sazegar",
								Category:    "Electronic",
								Image:       "",
								Returnable:  false,
								Quantity:    5,
								Attributes: map[string]*entities.Attribute{
									"Color": {
										KeyTranslate: map[string]string{
											"en": "رنگ",
											"fa": "رنگ",
										},
										ValueTranslate: map[string]string{
											"en": "رنگ",
											"fa": "رنگ",
										},
									},
									"dial_color": {
										KeyTranslate: map[string]string{
											"fa": "رنگ صفحه",
											"en": "رنگ صفحه",
										},
										ValueTranslate: map[string]string{
											"fa": "رنگ صفحه",
											"en": "رنگ صفحه",
										},
									},
								},
								Invoice: &entities.ItemInvoice{
									Commission: &entities.ItemCommission{
										ItemCommission: 9,
										RawUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
									Share: &entities.ItemShare{
										RawItemNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupItemNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawUnitSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
									SSO: &entities.ItemSSO{
										Rate:      8,
										IsObliged: true,
										RawUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
									VAT: &entities.ItemVAT{
										Rate:      8,
										IsObliged: true,
										RawUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
								},
							},
						},
						OrderCreatedAt:  time.Now(),
						SubPkgCreatedAt: time.Now(),
						SubPkgUpdatedAt: time.Now(),
						DeletedAt:       nil,
					},
					{
						OId:      3333333333,
						FId:      "",
						SellerId: 100002,
						ShipmentAmount: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RawShippingNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						RoundupShippingNet: &entities.Money{
							Amount:   "1650000",
							Currency: "IRR",
						},
						IsAlreadyShippingPay: false,
						Items: []*entities.SellerItem{
							{
								SId:         3333333333444,
								SKU:         "yt545-34",
								InventoryId: "666777888999",
								Title:       "Mobile",
								Brand:       "Nokia",
								Guaranty:    "Sazegar",
								Category:    "Electronic",
								Image:       "",
								Returnable:  false,
								Quantity:    5,
								Attributes: map[string]*entities.Attribute{
									"Color": {
										KeyTranslate: map[string]string{
											"en": "رنگ",
											"fa": "رنگ",
										},
										ValueTranslate: map[string]string{
											"en": "رنگ",
											"fa": "رنگ",
										},
									},
									"dial_color": {
										KeyTranslate: map[string]string{
											"fa": "رنگ صفحه",
											"en": "رنگ صفحه",
										},
										ValueTranslate: map[string]string{
											"fa": "رنگ صفحه",
											"en": "رنگ صفحه",
										},
									},
								},
								Invoice: &entities.ItemInvoice{
									Commission: &entities.ItemCommission{
										ItemCommission: 9,
										RawUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
									Share: &entities.ItemShare{
										RawItemNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupItemNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalNet: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawUnitSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalSellerShare: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
									SSO: &entities.ItemSSO{
										Rate:      8,
										IsObliged: true,
										RawUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
									VAT: &entities.ItemVAT{
										Rate:      8,
										IsObliged: true,
										RawUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupUnitPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RawTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
										RoundupTotalPrice: &entities.Money{
											Amount:   "1650000",
											Currency: "IRR",
										},
									},
								},
							},
						},
						OrderCreatedAt:  time.Now(),
						SubPkgCreatedAt: time.Now(),
						SubPkgUpdatedAt: time.Now(),
						DeletedAt:       nil,
					},
				},
			},
		},
		Payment: &entities.FinancePayment{
			TransferRequest: &entities.TransferRequest{
				TotalPrice: entities.Money{
					Amount:   "1650000",
					Currency: "IRR",
				},
				ReceiverName:       "Test",
				ReceiverAccountId:  "IR039248389443",
				PaymentDescription: "",
				TransferType:       "Pay_To_Seller",
				CreatedAt:          time.Now(),
			},
			TransferResponse: &entities.TransferResponse{
				TransferId: "34534534535454",
				CreatedAt:  time.Now(),
			},
			TransferResult: &entities.TransferResult{
				TransferId: "34534534535454",
				SuccessTransfer: &entities.Money{
					Amount:   "1650000",
					Currency: "IRR",
				},
				FailedTransfer: nil,
				CreatedAt:      time.Now(),
			},
			Status:    "Success",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Status:    entities.FinanceClosedStatus,
		StartAt:   nil,
		EndAt:     nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
	}
}

func getSellerFinance(fid string) (*entities.SellerFinance, error) {
	var finance entities.SellerFinance
	singleResult := mongoAdapter.FindOne(config.Mongo.Database, config.Mongo.SellerCollection, bson.D{{"fid", fid}, {"deletedAt", nil}})
	if err := singleResult.Err(); err != nil {
		return nil, err
	}

	if err := singleResult.Decode(&finance); err != nil {
		return nil, err
	}

	return &finance, nil
}
