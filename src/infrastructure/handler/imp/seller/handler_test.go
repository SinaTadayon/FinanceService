package seller

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/go-framework/mongoadapter"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/domain/model/repository/sellerOrderItem"
	"gitlab.faza.io/services/finance/infrastructure/converter"
	"gitlab.faza.io/services/finance/infrastructure/handler"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"gitlab.faza.io/services/finance/server/grpc_mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"os"
	"testing"
	"time"
)

var (
	financeRepository         finance_repository.ISellerFinanceRepository
	sellerOrderItemRepository sellerOrderItem.ISellerOrderItemRepository

	sellerFinanceHandler       handler.IHandler
	sellerOrderItemListHandler handler.IHandler
)

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
		ConnectUri: config.Mongo.URI,
		Username:   config.Mongo.User,
		//Password:     App.Cfg.Mongo.Pass,
		ConnTimeout:            time.Duration(config.Mongo.ConnectionTimeout) * time.Second,
		ReadTimeout:            time.Duration(config.Mongo.ReadTimeout) * time.Second,
		WriteTimeout:           time.Duration(config.Mongo.WriteTimeout) * time.Second,
		MaxConnIdleTime:        time.Duration(config.Mongo.MaxConnIdleTime) * time.Second,
		HeartbeatInterval:      time.Duration(config.Mongo.HeartBeatInterval) * time.Second,
		ServerSelectionTimeout: time.Duration(config.Mongo.ServerSelectionTimeout) * time.Second,
		RetryConnect:           uint64(config.Mongo.RetryConnect),
		MaxPoolSize:            uint64(config.Mongo.MaxPoolSize),
		MinPoolSize:            uint64(config.Mongo.MinPoolSize),
		WriteConcernW:          config.Mongo.WriteConcernW,
		WriteConcernJ:          config.Mongo.WriteConcernJ,
		RetryWrites:            config.Mongo.RetryWrite,
		ReadConcern:            config.Mongo.ReadConcern,
		ReadPreference:         config.Mongo.ReadPreferred,
	}

	mongoAdapter, err := mongoadapter.NewMongo(mongoConf)
	if err != nil {
		log.GLog.Logger.Error("mongoadapter.NewMongo failed", "error", err)
		os.Exit(1)
	}

	financeRepository = finance_repository.NewSellerFinanceRepository(mongoAdapter, config.Mongo.Database, config.Mongo.SellerCollection)
	sellerOrderItemRepository = sellerOrderItem.NewSellerOrderItemRepository(mongoAdapter, config.Mongo.Database, config.Mongo.SellerCollection)

	converter := converter.NewConverter()
	sellerFinanceHandler = NewSellerFinanceListHandler(financeRepository, converter)
	sellerOrderItemListHandler = NewSellerFinanceOrderItemListHandler(sellerOrderItemRepository, converter)

	// Running Tests
	code := m.Run()
	// removeCollection()
	os.Exit(code)
}

func TestSellerFinanceOrderItemListHandler_Handle(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())
	finance = iFuture.Data().(*entities.SellerFinance)

	req := finance_proto.RequestMessage{
		Name: string(grpc_mux.SellerOrderItemListMethod),
		Type: "",
		Time: time.Now().Format(utils.ISO8601),
		Header: &finance_proto.ReqMeta{
			UTP:     "Seller",
			UID:     finance.SellerId,
			FID:     finance.FId,
			Page:    1,
			PerPage: 2,
			Sorts: &finance_proto.RequestMetaSorts{
				Name: "fid",
				Dir:  uint32(finance_proto.RequestMetaSorts_Ascending),
			},
			Filters: nil,
		},
		Body: nil,
	}

	res := sellerOrderItemListHandler.Handle(&req).Get()

	require.Nil(t, res.Error())
	resp := res.Data().(*finance_proto.ResponseMessage)
	require.Equal(t, uint32(2), resp.Meta.Total)

	body := finance_proto.SellerFinanceOrderItemCollection{}
	err := proto.Unmarshal(resp.Data.Value, &body)
	require.Nil(t, err)
	require.Equal(t, body.Items[0].Payment, int32(1))
	require.Equal(t, body.Items[1].Payment, int32(0))
}

func TestSellerFinanceListHandler_Handle(t *testing.T) {
	defer removeCollection()
	finance := createFinance()
	ctx, _ := context.WithCancel(context.Background())
	iFuture := financeRepository.Insert(ctx, *finance).Get()
	require.Nil(t, iFuture.Error())

	req := finance_proto.RequestMessage{
		Name: string(grpc_mux.SellerFinanceListMethod),
		Type: "",
		Time: time.Now().Format(utils.ISO8601),
		Header: &finance_proto.ReqMeta{
			UTP:       "Seller",
			UID:       finance.SellerId,
			FID:       finance.FId,
			Page:      1,
			PerPage:   2,
			IpAddress: "",
			StartAt:   "",
			EndAt:     "",
			Sorts: &finance_proto.RequestMetaSorts{
				Name: "fid",
				Dir:  uint32(finance_proto.RequestMetaSorts_Descending),
			},
			Filters: nil,
		},
		Body: nil,
	}

	res := sellerFinanceHandler.Handle(&req).Get()

	require.Nil(t, res.Error())
	resp := res.Data().(*finance_proto.ResponseMessage)
	require.Equal(t, uint32(1), resp.Meta.Total)

	body := finance_proto.SellerFinanceListCollection{}
	err := proto.Unmarshal(resp.Data.Value, &body)
	require.Nil(t, err)
	require.Equal(t, finance.Payment.TransferRequest.TotalPrice.Amount, body.Items[0].Total.Amount)
}

func removeCollection() {
	ctx, _ := context.WithCancel(context.Background())
	if err := financeRepository.RemoveAll(ctx); err != nil {
	}
}

func createFinance() *entities.SellerFinance {
	timestamp := time.Now().UTC()
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
						IsAlreadyShippingPayed: false,
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
						OrderCreatedAt:  &timestamp,
						SubPkgCreatedAt: &timestamp,
						SubPkgUpdatedAt: &timestamp,
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
						IsAlreadyShippingPayed: false,
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
						OrderCreatedAt:  &timestamp,
						SubPkgCreatedAt: &timestamp,
						SubPkgUpdatedAt: &timestamp,
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
				FailedTransfer: &entities.Money{
					Amount:   "1620000",
					Currency: "IRR",
				},
				CreatedAt: time.Now(),
			},
			Status:    entities.TransferPartialState,
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
