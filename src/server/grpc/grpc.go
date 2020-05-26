package grpc

import (
	"context"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"gitlab.faza.io/go-framework/logger"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/app"
	"gitlab.faza.io/services/finance/domain/model/entities"
	order_scheduler "gitlab.faza.io/services/finance/domain/scheduler/order"
	payment_scheduler "gitlab.faza.io/services/finance/domain/scheduler/payment"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"path"
	"runtime/debug"
	"strconv"
	"time"
)

type Server struct {
	finance_proto.UnimplementedFinanceServiceServer
	orderScheduler order_scheduler.OrderScheduler
	address        string
	port           uint16
}

func NewServer(address string, port uint16, orderScheduler order_scheduler.OrderScheduler) Server {
	return Server{
		orderScheduler: orderScheduler,
		address:        address,
		port:           port,
	}
}

func (server Server) TestSellerFinance_OrderCollectionRequestHandler(ctx context.Context, req *finance_proto.OrderCollectionRequest) (*finance_proto.OrderCollectionResponse, error) {
	if !app.Globals.Config.App.ServiceTestAPIEnabled {
		return nil, status.Error(409, "Request Invalid")
	}

	var startAt, endAt time.Time
	if req.StartAt != "" {
		temp, err := time.Parse(utils.ISO8601, req.StartAt)
		if err != nil {
			log.GLog.Logger.Error("StartAt invalid",
				"fn", "TestSellerFinance_OrderCollectionRequestHandler",
				"StartAt", req.StartAt,
				"request", req)
			return nil, status.Error(codes.Code(future.BadRequest), "Request Invalid")
		}

		startAt = temp
	} else {
		log.GLog.Logger.Error("StartAt is empty",
			"fn", "TestSellerFinance_OrderCollectionRequestHandler",
			"request", req)
		return nil, status.Error(codes.Code(future.BadRequest), "Request Invalid")
	}

	if req.EndAt != "" {
		temp, err := time.Parse(utils.ISO8601, req.EndAt)
		if err != nil {
			log.GLog.Logger.Error("EndAt invalid",
				"fn", "TestSellerFinance_OrderCollectionRequestHandler",
				"EndAt", req.EndAt,
				"request", req)
			return nil, status.Error(codes.Code(future.BadRequest), "Request Invalid")
		}

		startAt = temp
	} else {
		log.GLog.Logger.Error("EndAt is empty",
			"fn", "TestSellerFinance_OrderCollectionRequestHandler",
			"request", req)
		return nil, status.Error(codes.Code(future.BadRequest), "Request Invalid")
	}

	iFuture := server.orderScheduler.TestOrderTask(ctx, startAt, endAt, req.SellerIds).Get()
	if iFuture.Error() != nil {
		return nil, status.Error(codes.Code(iFuture.Error().Code()), iFuture.Error().Message())
	}

	finances := iFuture.Data().([]*entities.SellerFinance)

	response := &finance_proto.OrderCollectionResponse{
		Finances: convertFinanceToRPC(finances),
	}

	return response, nil
}

func (server Server) TestSellerFinance_MoneyTransferRequestHandler(ctx context.Context, req *finance_proto.MoneyTransferRequest) (*finance_proto.MoneyTransferResponse, error) {
	if !app.Globals.Config.App.ServiceTestAPIEnabled {
		return nil, status.Error(409, "Request Invalid")
	}

	iFuture := payment_scheduler.PaymentProcessTask(ctx).Get()
	if iFuture.Error() != nil {
		log.GLog.Logger.Error("PaymentProcessTask failed",
			"fn", "TestSellerFinance_MoneyTransferRequestHandler",
			"request", req)
		return nil, status.Error(codes.Code(iFuture.Error().Code()), iFuture.Error().Message())
	}

	finances := make([]*entities.SellerFinance, 0, len(req.Fids))
	for _, fid := range req.Fids {
		iFuture := app.Globals.SellerFinanceRepository.FindById(ctx, fid).Get()
		if iFuture.Error() != nil {
			if iFuture.Error().Code() != future.NotFound {
				log.GLog.Logger.Error("FindById failed",
					"fn", "TestSellerFinance_MoneyTransferRequestHandler",
					"fid", fid,
					"request", req,
					"error", iFuture.Error())
				return nil, status.Error(codes.Code(iFuture.Error().Code()), iFuture.Error().Message())
			}
		}

		finances = append(finances, iFuture.Data().(*entities.SellerFinance))
	}

	response := &finance_proto.MoneyTransferResponse{
		Finances: convertFinanceToRPC(finances),
	}

	return response, nil
}

func (server Server) Start() error {
	port := strconv.Itoa(int(server.port))
	lis, err := net.Listen("tcp", server.address+":"+port)
	if err != nil {
		log.GLog.Logger.Error("Failed to listen to TCP on port", "fn", "Start", "port", port, "error", err)
		return err
	}
	log.GLog.Logger.Info("GRPC server started", "fn", "Start", "address", server.address, "port", port)

	customFunc := func(p interface{}) (err error) {
		log.GLog.Logger.Error("rpc panic recovered", "fn", "Start",
			"panic", p, "stacktrace", string(debug.Stack()))
		return grpc.Errorf(codes.Unknown, "panic triggered: %v", p)
	}

	opts := []grpc_recovery.Option{
		grpc_recovery.WithRecoveryHandler(customFunc),
	}

	uIntOpt := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		grpc_prometheus.UnaryServerInterceptor,
		grpc_recovery.UnaryServerInterceptor(opts...),
		myUnaryLogger(log.GLog.Logger),
		//grpc_zap.UnaryServerInterceptor(zapLogger),
	))

	sIntOpt := grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		grpc_prometheus.StreamServerInterceptor,
		grpc_recovery.StreamServerInterceptor(opts...),
		//grpc_zap.StreamServerInterceptor(app.Globals.ZapLogger),
	))

	// enable grpc prometheus interceptors to log timing info for grpc APIs
	grpc_prometheus.EnableHandlingTimeHistogram()

	//Start GRPC server and register the server
	grpcServer := grpc.NewServer(uIntOpt, sIntOpt)
	finance_proto.RegisterFinanceServiceServer(grpcServer, &server)
	if err := grpcServer.Serve(lis); err != nil {
		log.GLog.Logger.Error("GRPC server start field", "fn", "Start", "error", err.Error())
		return err
	}

	return nil
}

func myUnaryLogger(log logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		startTime := time.Now()
		resp, err = handler(ctx, req)
		dur := time.Since(startTime)
		lg := log.FromContext(ctx)
		lg = lg.With(
			zap.Duration("took_sec", dur),
			zap.String("grpc.Method", path.Base(info.FullMethod)),
			zap.String("grpc.Service", path.Dir(info.FullMethod)[1:]),
			zap.String("grpc.Code", grpc.Code(err).String()),
		)
		lg.Debug("finished unary call")
		return
	}
}

func convertFinanceToRPC(finances []*entities.SellerFinance) []*finance_proto.SellerFinance {

	protoFinances := make([]*finance_proto.SellerFinance, 0, len(finances))
	for _, finance := range finances {
		protoFinance := &finance_proto.SellerFinance{
			FId:        finance.FId,
			SellerId:   finance.SellerId,
			SellerInfo: nil,
			Invoice:    nil,
			Orders:     nil,
			Payment:    nil,
			Status:     string(finance.Status),
			StartAt:    finance.StartAt.Format(utils.ISO8601),
			EndAt:      finance.EndAt.Format(utils.ISO8601),
			CreatedAt:  finance.CreatedAt.Format(utils.ISO8601),
			UpdatedAt:  finance.UpdatedAt.Format(utils.ISO8601),
		}

		if finance.SellerInfo != nil {
			protoFinance.SellerInfo = &finance_proto.SellerProfile{
				GeneralInfo: &finance_proto.SellerProfile_GeneralInfo{
					ShopDisplayName:          finance.SellerInfo.GeneralInfo.ShopDisplayName,
					Type:                     finance.SellerInfo.GeneralInfo.Type,
					Email:                    finance.SellerInfo.GeneralInfo.Email,
					LandPhone:                finance.SellerInfo.GeneralInfo.LandPhone,
					MobilePhone:              finance.SellerInfo.GeneralInfo.MobilePhone,
					Website:                  finance.SellerInfo.GeneralInfo.Website,
					Province:                 finance.SellerInfo.GeneralInfo.Province,
					City:                     finance.SellerInfo.GeneralInfo.City,
					Neighborhood:             finance.SellerInfo.GeneralInfo.Neighborhood,
					PostalAddress:            finance.SellerInfo.GeneralInfo.PostalAddress,
					PostalCode:               finance.SellerInfo.GeneralInfo.PostalCode,
					IsVATObliged:             finance.SellerInfo.GeneralInfo.IsVATObliged,
					VATCertificationImageURL: finance.SellerInfo.GeneralInfo.VATCertificationImageURL,
				},
				FinanceInfo: &finance_proto.SellerProfile_FinanceInfo{
					Iban:                    finance.SellerInfo.FinanceData.Iban,
					AccountHolderFirstName:  finance.SellerInfo.FinanceData.AccountHolderFirstName,
					AccountHolderFamilyName: finance.SellerInfo.FinanceData.AccountHolderFamilyName,
				},
			}
		}

		if finance.Invoice != nil {
			protoFinance.Invoice = &finance_proto.SellerInvoice{
				SSORawTotal:            nil,
				SSORoundupTotal:        nil,
				VATRawTotal:            nil,
				VATRoundupTotal:        nil,
				CommissionRawTotal:     nil,
				CommissionRoundupTotal: nil,
				ShareRawTotal:          nil,
				ShareRoundupTotal:      nil,
				ShipmentRawTotal:       nil,
				ShipmentRoundupTotal:   nil,
			}

			if finance.Invoice.SSORawTotal != nil {
				protoFinance.Invoice.SSORawTotal = &finance_proto.Money{
					Amount:   finance.Invoice.SSORawTotal.Amount,
					Currency: finance.Invoice.SSORawTotal.Currency,
				}
			}

			if finance.Invoice.SSORoundupTotal != nil {
				protoFinance.Invoice.SSORoundupTotal = &finance_proto.Money{
					Amount:   finance.Invoice.SSORoundupTotal.Amount,
					Currency: finance.Invoice.SSORoundupTotal.Currency,
				}
			}

			if finance.Invoice.VATRawTotal != nil {
				protoFinance.Invoice.VATRawTotal = &finance_proto.Money{
					Amount:   finance.Invoice.VATRawTotal.Amount,
					Currency: finance.Invoice.VATRawTotal.Currency,
				}
			}

			if finance.Invoice.VATRoundupTotal != nil {
				protoFinance.Invoice.VATRoundupTotal = &finance_proto.Money{
					Amount:   finance.Invoice.VATRoundupTotal.Amount,
					Currency: finance.Invoice.VATRoundupTotal.Currency,
				}
			}

			if finance.Invoice.CommissionRawTotal != nil {
				protoFinance.Invoice.CommissionRawTotal = &finance_proto.Money{
					Amount:   finance.Invoice.CommissionRawTotal.Amount,
					Currency: finance.Invoice.CommissionRawTotal.Currency,
				}
			}

			if finance.Invoice.CommissionRoundupTotal != nil {
				protoFinance.Invoice.CommissionRoundupTotal = &finance_proto.Money{
					Amount:   finance.Invoice.CommissionRoundupTotal.Amount,
					Currency: finance.Invoice.CommissionRoundupTotal.Currency,
				}
			}

			if finance.Invoice.ShareRawTotal != nil {
				protoFinance.Invoice.ShareRawTotal = &finance_proto.Money{
					Amount:   finance.Invoice.ShareRawTotal.Amount,
					Currency: finance.Invoice.ShareRawTotal.Currency,
				}
			}

			if finance.Invoice.ShareRoundupTotal != nil {
				protoFinance.Invoice.ShareRoundupTotal = &finance_proto.Money{
					Amount:   finance.Invoice.ShareRoundupTotal.Amount,
					Currency: finance.Invoice.ShareRoundupTotal.Currency,
				}
			}

			if finance.Invoice.ShipmentRawTotal != nil {
				protoFinance.Invoice.ShipmentRawTotal = &finance_proto.Money{
					Amount:   finance.Invoice.ShipmentRawTotal.Amount,
					Currency: finance.Invoice.ShipmentRawTotal.Currency,
				}
			}

			if finance.Invoice.ShipmentRoundupTotal != nil {
				protoFinance.Invoice.ShipmentRoundupTotal = &finance_proto.Money{
					Amount:   finance.Invoice.ShipmentRoundupTotal.Amount,
					Currency: finance.Invoice.ShipmentRoundupTotal.Currency,
				}
			}
		}

		if finance.Payment != nil {
			protoFinance.Payment = &finance_proto.SellerFinancePayment{
				TransferRequest:     nil,
				TransferResponse:    nil,
				TransferResult:      nil,
				SellerTransferState: string(finance.Payment.Status),
				CreatedAt:           finance.Payment.CreatedAt.Format(utils.ISO8601),
				UpdatedAt:           finance.Payment.UpdatedAt.Format(utils.ISO8601),
			}

			if finance.Payment.TransferRequest != nil {
				protoFinance.Payment.TransferRequest = &finance_proto.SellerTransferRequest{
					TotalPrice: &finance_proto.Money{
						Amount:   finance.Payment.TransferRequest.TotalPrice.Amount,
						Currency: finance.Payment.TransferRequest.TotalPrice.Currency,
					},
					ReceiverName:       finance.Payment.TransferRequest.ReceiverName,
					ReceiverAccountId:  finance.Payment.TransferRequest.ReceiverAccountId,
					PaymentDescription: finance.Payment.TransferRequest.PaymentDescription,
					TransferType:       finance.Payment.TransferRequest.TransferType,
					CreatedAt:          finance.Payment.TransferRequest.CreatedAt.Format(utils.ISO8601),
				}
			}

			if finance.Payment.TransferResponse != nil {
				protoFinance.Payment.TransferResponse = &finance_proto.SellerTransferResponse{
					TransferId: finance.Payment.TransferResponse.TransferId,
					CreatedAt:  finance.Payment.TransferResponse.CreatedAt.Format(utils.ISO8601),
				}
			}

			if finance.Payment.TransferResult != nil {
				protoFinance.Payment.TransferResult = &finance_proto.SellerTransferResult{
					TransferId: finance.Payment.TransferResult.TransferId,
					SuccessTransfer: &finance_proto.Money{
						Amount:   finance.Payment.TransferResult.SuccessTransfer.Amount,
						Currency: finance.Payment.TransferResult.SuccessTransfer.Currency,
					},
					PendingTransfer: &finance_proto.Money{
						Amount:   finance.Payment.TransferResult.PendingTransfer.Amount,
						Currency: finance.Payment.TransferResult.PendingTransfer.Currency,
					},
					FailedTransfer: &finance_proto.Money{
						Amount:   finance.Payment.TransferResult.FailedTransfer.Amount,
						Currency: finance.Payment.TransferResult.FailedTransfer.Currency,
					},
					CreatedAt: finance.Payment.TransferResult.CreatedAt.Format(utils.ISO8601),
					UpdatedAt: finance.Payment.TransferResult.UpdatedAt.Format(utils.ISO8601),
				}
			}

			if finance.OrdersInfo != nil {
				protoFinance.Orders = make([]*finance_proto.SellerOrder, 0, len(finance.OrdersInfo[0].Orders))
				for _, order := range finance.OrdersInfo[0].Orders {
					protoOrder := &finance_proto.SellerOrder{
						OId: order.OId,
						ShipmentAmount: &finance_proto.Money{
							Amount:   order.ShipmentAmount.Amount,
							Currency: order.ShipmentAmount.Currency,
						},
						RawShippingNet: &finance_proto.Money{
							Amount:   order.RawShippingNet.Amount,
							Currency: order.RawShippingNet.Currency,
						},
						RoundupShippingNet: &finance_proto.Money{
							Amount:   order.RoundupShippingNet.Amount,
							Currency: order.RoundupShippingNet.Currency,
						},
						IsAlreadyShippingPayed: order.IsAlreadyShippingPayed,
						Items:                  nil,
						OrderCreatedAt:         order.OrderCreatedAt.Format(utils.ISO8601),
						SubPkgCreatedAt:        order.SubPkgCreatedAt.Format(utils.ISO8601),
						SubPkgUpdatedAt:        order.SubPkgUpdatedAt.Format(utils.ISO8601),
					}

					protoOrder.Items = make([]*finance_proto.SellerItem, 0, len(order.Items))
					for _, item := range order.Items {
						protoItem := &finance_proto.SellerItem{
							SId:         item.SId,
							SKU:         item.SKU,
							InventoryId: item.InventoryId,
							Title:       item.Title,
							Brand:       item.Brand,
							Guaranty:    item.Guaranty,
							Category:    item.Category,
							Image:       item.Image,
							Returnable:  item.Returnable,
							Quantity:    item.Quantity,
							Attributes:  nil,
							Invoice: &finance_proto.SellerItemInvoice{
								Commission: nil,
								Share:      nil,
								SSO:        nil,
								VAT:        nil,
							},
						}

						if item.Attributes != nil {
							protoItem.Attributes = make(map[string]*finance_proto.Attribute, len(item.Attributes))
							for attrKey, attribute := range item.Attributes {
								keyTranslates := make(map[string]string, len(attribute.KeyTranslate))
								for keyTran, value := range attribute.KeyTranslate {
									keyTranslates[keyTran] = value
								}
								valTranslates := make(map[string]string, len(attribute.ValueTranslate))
								for valTran, value := range attribute.ValueTranslate {
									valTranslates[valTran] = value
								}
								protoItem.Attributes[attrKey] = &finance_proto.Attribute{
									KeyTranslate:   keyTranslates,
									ValueTranslate: valTranslates,
								}
							}
						}

						if item.Invoice.Commission != nil {
							protoItem.Invoice.Commission = &finance_proto.SellerItemCommission{
								ItemCommission:    item.Invoice.Commission.ItemCommission,
								RawUnitPrice:      nil,
								RoundupUnitPrice:  nil,
								RawTotalPrice:     nil,
								RoundupTotalPrice: nil,
							}

							if item.Invoice.Commission.RawUnitPrice != nil {
								protoItem.Invoice.Commission.RawUnitPrice = &finance_proto.Money{
									Amount:   item.Invoice.Commission.RawUnitPrice.Amount,
									Currency: item.Invoice.Commission.RawUnitPrice.Currency,
								}
							}

							if item.Invoice.Commission.RoundupUnitPrice != nil {
								protoItem.Invoice.Commission.RoundupUnitPrice = &finance_proto.Money{
									Amount:   item.Invoice.Commission.RoundupUnitPrice.Amount,
									Currency: item.Invoice.Commission.RoundupUnitPrice.Currency,
								}
							}

							if item.Invoice.Commission.RawTotalPrice != nil {
								protoItem.Invoice.Commission.RawTotalPrice = &finance_proto.Money{
									Amount:   item.Invoice.Commission.RawTotalPrice.Amount,
									Currency: item.Invoice.Commission.RawTotalPrice.Currency,
								}
							}

							if item.Invoice.Commission.RoundupTotalPrice != nil {
								protoItem.Invoice.Commission.RoundupTotalPrice = &finance_proto.Money{
									Amount:   item.Invoice.Commission.RoundupTotalPrice.Amount,
									Currency: item.Invoice.Commission.RoundupTotalPrice.Currency,
								}
							}
						}

						if item.Invoice.Share != nil {
							protoItem.Invoice.Share = &finance_proto.SellerItemShare{
								RawItemNet: &finance_proto.Money{
									Amount:   item.Invoice.Share.RawItemNet.Amount,
									Currency: item.Invoice.Share.RawItemNet.Currency,
								},
								RoundupItemNet: &finance_proto.Money{
									Amount:   item.Invoice.Share.RoundupItemNet.Amount,
									Currency: item.Invoice.Share.RoundupItemNet.Currency,
								},
								RawTotalNet: &finance_proto.Money{
									Amount:   item.Invoice.Share.RawTotalNet.Amount,
									Currency: item.Invoice.Share.RawTotalNet.Currency,
								},
								RoundupTotalNet: &finance_proto.Money{
									Amount:   item.Invoice.Share.RoundupTotalNet.Amount,
									Currency: item.Invoice.Share.RoundupTotalNet.Currency,
								},
								RawUnitSellerShare: &finance_proto.Money{
									Amount:   item.Invoice.Share.RawUnitSellerShare.Amount,
									Currency: item.Invoice.Share.RawUnitSellerShare.Currency,
								},
								RoundupUnitSellerShare: &finance_proto.Money{
									Amount:   item.Invoice.Share.RoundupUnitSellerShare.Amount,
									Currency: item.Invoice.Share.RoundupUnitSellerShare.Currency,
								},
								RawTotalSellerShare: &finance_proto.Money{
									Amount:   item.Invoice.Share.RawTotalSellerShare.Amount,
									Currency: item.Invoice.Share.RawTotalSellerShare.Currency,
								},
								RoundupTotalSellerShare: &finance_proto.Money{
									Amount:   item.Invoice.Share.RoundupTotalSellerShare.Amount,
									Currency: item.Invoice.Share.RoundupTotalSellerShare.Currency,
								},
							}
						}

						if item.Invoice.SSO != nil {
							protoItem.Invoice.SSO = &finance_proto.SellerItemSSO{
								Rate:              item.Invoice.SSO.Rate,
								IsObliged:         item.Invoice.SSO.IsObliged,
								RawUnitPrice:      nil,
								RoundupUnitPrice:  nil,
								RawTotalPrice:     nil,
								RoundupTotalPrice: nil,
							}

							if item.Invoice.SSO.RawUnitPrice != nil {
								protoItem.Invoice.SSO.RawUnitPrice = &finance_proto.Money{
									Amount:   item.Invoice.SSO.RawUnitPrice.Amount,
									Currency: item.Invoice.SSO.RawUnitPrice.Currency,
								}
							}

							if item.Invoice.SSO.RoundupUnitPrice != nil {
								protoItem.Invoice.SSO.RoundupUnitPrice = &finance_proto.Money{
									Amount:   item.Invoice.SSO.RoundupUnitPrice.Amount,
									Currency: item.Invoice.SSO.RoundupUnitPrice.Currency,
								}
							}

							if item.Invoice.SSO.RawTotalPrice != nil {
								protoItem.Invoice.SSO.RawTotalPrice = &finance_proto.Money{
									Amount:   item.Invoice.SSO.RawTotalPrice.Amount,
									Currency: item.Invoice.SSO.RawTotalPrice.Currency,
								}
							}

							if item.Invoice.SSO.RoundupTotalPrice != nil {
								protoItem.Invoice.SSO.RoundupTotalPrice = &finance_proto.Money{
									Amount:   item.Invoice.SSO.RoundupTotalPrice.Amount,
									Currency: item.Invoice.SSO.RoundupTotalPrice.Currency,
								}
							}
						}

						if item.Invoice.VAT != nil {
							protoItem.Invoice.VAT = &finance_proto.SellerItemVAT{
								Rate:              item.Invoice.VAT.Rate,
								IsObliged:         item.Invoice.VAT.IsObliged,
								RawUnitPrice:      nil,
								RoundupUnitPrice:  nil,
								RawTotalPrice:     nil,
								RoundupTotalPrice: nil,
							}

							if item.Invoice.VAT.RawUnitPrice != nil {
								protoItem.Invoice.VAT.RawUnitPrice = &finance_proto.Money{
									Amount:   item.Invoice.VAT.RawUnitPrice.Amount,
									Currency: item.Invoice.VAT.RawUnitPrice.Currency,
								}
							}

							if item.Invoice.VAT.RoundupUnitPrice != nil {
								protoItem.Invoice.VAT.RoundupUnitPrice = &finance_proto.Money{
									Amount:   item.Invoice.VAT.RoundupUnitPrice.Amount,
									Currency: item.Invoice.VAT.RoundupUnitPrice.Currency,
								}
							}

							if item.Invoice.VAT.RawTotalPrice != nil {
								protoItem.Invoice.VAT.RawTotalPrice = &finance_proto.Money{
									Amount:   item.Invoice.VAT.RawTotalPrice.Amount,
									Currency: item.Invoice.VAT.RawTotalPrice.Currency,
								}
							}

							if item.Invoice.VAT.RoundupTotalPrice != nil {
								protoItem.Invoice.VAT.RoundupTotalPrice = &finance_proto.Money{
									Amount:   item.Invoice.VAT.RoundupTotalPrice.Amount,
									Currency: item.Invoice.VAT.RoundupTotalPrice.Currency,
								}
							}
						}

						protoOrder.Items = append(protoOrder.Items, protoItem)
					}

					protoFinance.Orders = append(protoFinance.Orders, protoOrder)
				}
			}
		}

		protoFinances = append(protoFinances, protoFinance)
	}

	return protoFinances
}
