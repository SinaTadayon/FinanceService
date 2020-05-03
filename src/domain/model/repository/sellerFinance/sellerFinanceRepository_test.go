package sellerFinance

import (
	"context"
	"github.com/stretchr/testify/require"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/go-framework/mongoadapter"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"go.mongodb.org/mongo-driver/bson"
	"os"
	"testing"
	"time"
)

var financeRepository ISellerFinanceRepository

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

	config, err := configs.LoadConfigs(path)
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

	mongoAdapter, err := mongoadapter.NewMongo(mongoConf)
	if err != nil {
		log.GLog.Logger.Error("mongoadapter.NewMongo failed", "error", err)
		os.Exit(1)
	}

	financeRepository = NewSellerFinanceRepository(mongoAdapter, config.Mongo.Database, config.Mongo.Collection)

	// Running Tests
	code := m.Run()
	removeCollection()
	os.Exit(code)
}

func TestSaveFinanceRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Save(ctx, *finance).Get()
	require.Nil(t, iFuture.Error(), "financeRepository.Save failed")
	require.NotEmpty(t, iFuture.Data().(*entities.SellerFinance).FId, "financeRepository.Save failed, fid not generated")
}

func TestUpdateFinanceRepository(t *testing.T) {

	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Save(ctx, *finance).Get()
	finance1 := iFuture.Data().(*entities.SellerFinance)
	require.Nil(t, iFuture.Error(), "financeRepository.Save failed")
	require.NotEmpty(t, finance1.FId, "financeRepository.Save failed, fid not generated")

	finance1.Status = "IN_PROGRESS"
	iFuture = financeRepository.Save(ctx, *finance1).Get()
	require.Nil(t, iFuture.Error(), "financeRepository.Save failed")
	require.Equal(t, iFuture.Data().(*entities.SellerFinance).Status, "IN_PROGRESS")
}

func TestUpdateFinanceRepository_Failed(t *testing.T) {

	defer removeCollection()
	finance := createFinance()
	timeTmp := time.Now().UTC()
	finance.DeletedAt = &timeTmp
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Save(ctx, *finance).Get()
	finance1 := iFuture.Data().(*entities.SellerFinance)
	require.Nil(t, iFuture.Error(), "financeRepository.Save failed")
	require.NotEmpty(t, finance1.FId, "financeRepository.Save failed, fid not generated")

	finance1.Status = "IN_PROGRESS"
	iFuture = financeRepository.Save(ctx, *finance1).Get()
	require.Error(t, iFuture.Error())
}

func TestInsertFinanceRepository_Success(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error(), "financeRepository.Insert failed")
	require.NotEmpty(t, iFuture.Data().(*entities.SellerFinance).FId, "financeRepository.Insert failed, fid not generated")
}

func TestFindAllFinanceRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindAll(ctx).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 3, len(iFuture.Data().([]*entities.SellerFinance)))
}

func TestFindAllWithSortFinanceRepository(t *testing.T) {

	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	fid := iFuture.Data().(*entities.SellerFinance).FId

	iFuture = financeRepository.FindAllWithSort(ctx, "fid", -1).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, iFuture.Data().([]*entities.SellerFinance)[0].FId, fid)
}

func TestFindAllWithPageAndPerPageRepository_success(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindAllWithPage(ctx, 2, 2).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 1, len(iFuture.Data().(FinancePageableResult).SellerFinances))
}

func TestFindAllWithPageAndPerPageRepository_failed(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindAllWithPage(ctx, 2000, 2001).Get()
	require.NotNil(t, iFuture.Error())
	require.Equal(t, ErrorPageNotAvailable, iFuture.Error().Reason())
}

func TestFindAllWithPageAndPerPageAndSortRepository_success(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	fid := iFuture.Data().(*entities.SellerFinance).FId

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindAllWithPageAndSort(ctx, 1, 2, "fid", 1).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 2, len(iFuture.Data().(FinancePageableResult).SellerFinances))
	require.Equal(t, fid, iFuture.Data().(FinancePageableResult).SellerFinances[0].FId)
}

func TestFindByIdRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindById(ctx, iFuture.Data().(*entities.SellerFinance).FId).Get()
	require.Nil(t, iFuture.Error())
	require.NotNil(t, iFuture.Data())
}

func TestExistsByIdRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.ExistsById(ctx, iFuture.Data().(*entities.SellerFinance).FId).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, true, iFuture.Data().(bool))
}

func TestCountRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Count(ctx).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, int64(2), iFuture.Data().(int64))
}

func TestDeleteFinanceRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	finance1 := iFuture.Data().(*entities.SellerFinance)

	iFuture = financeRepository.Delete(ctx, *finance1).Get()
	require.Nil(t, iFuture.Error())
	require.NotNil(t, iFuture.Data().(*entities.SellerFinance).DeletedAt)
}

func TestDeleteAllRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.DeleteAll(ctx).Get()
	require.Nil(t, iFuture.Error())
}

func TestRemoveOrderRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Remove(ctx, *(iFuture.Data().(*entities.SellerFinance))).Get()
	require.Nil(t, iFuture.Error())
}

func TestFindByFilterRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	finance.Status = "IN_PROGRESS"
	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindByFilter(ctx, func() interface{} {
		return bson.D{{"status", "IN_PROGRESS"}, {"deletedAt", nil}}
	}).Get()

	require.Nil(t, iFuture.Error())
	require.Equal(t, "IN_PROGRESS", iFuture.Data().([]*entities.SellerFinance)[0].Status)
}

func TestFindByFilterWithSortOrderRepository(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	finance.Status = "IN_PROGRESS"
	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	fid := iFuture.Data().(*entities.SellerFinance).FId

	finance.Status = "IN_PROGRESS"
	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindByFilterWithSort(ctx, func() (interface{}, string, int) {
		return bson.D{{"status", "IN_PROGRESS"}, {"deletedAt", nil}}, "fid", 1
	}).Get()

	require.Nil(t, iFuture.Error())
	require.Equal(t, fid, iFuture.Data().([]*entities.SellerFinance)[0].FId)
}

func TestFindByFilterWithPageAndPerPageRepository_success(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindByFilterWithPage(ctx, func() interface{} {
		return bson.D{{}, {"deletedAt", nil}}
	}, 2, 2).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 1, len(iFuture.Data().(FinancePageableResult).SellerFinances))
}

func TestFindByFilterWithPageAndPerPageAndSortRepository_success(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	fid := iFuture.Data().(*entities.SellerFinance).FId

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	iFuture = financeRepository.FindByFilterWithPageAndSort(ctx, func() (interface{}, string, int) {
		return bson.D{{}, {"deletedAt", nil}}, "fid", 1
	}, 1, 2).Get()
	require.Nil(t, iFuture.Error())
	require.Equal(t, 2, len(iFuture.Data().(FinancePageableResult).SellerFinances))
	require.Equal(t, fid, iFuture.Data().(FinancePageableResult).SellerFinances[0].FId)
}

func removeCollection() {
	ctx, _ := context.WithCancel(context.Background())
	if err := financeRepository.RemoveAll(ctx); err != nil {
	}
}

func createFinance() *entities.SellerFinance {
	return &entities.SellerFinance{
		FId:        "",
		SellerId:   100002,
		Version:    0,
		DocVersion: "1.0.0",
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
		Invoice: entities.Invoice{
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
		Orders: []*entities.OrderFinance{
			{
				OId:      1111111111,
				FId:      "",
				SellerId: 100002,
				RawShippingNet: &entities.Money{
					Amount:   "1650000",
					Currency: "IRR",
				},
				RoundupShippingNet: &entities.Money{
					Amount:   "1650000",
					Currency: "IRR",
				},
				Items: []*entities.Item{
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
						Invoice: entities.ItemInvoice{
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
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				DeletedAt: nil,
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
		Status:    "CLOSED",
		Start:     nil,
		End:       nil,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DeletedAt: nil,
	}
}
