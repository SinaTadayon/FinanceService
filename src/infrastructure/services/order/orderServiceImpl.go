package order_service

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	orderProto "gitlab.faza.io/protos/order"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"google.golang.org/grpc"
	"sync"
	"time"
)

type FilterType string

const (
	// ISO8601 standard time format
	ISO8601 = "2006-01-02T15:04:05-0700"
)

const (
	OrderStateFilterType FilterType = "OrderState"
)

type iOrderServiceImpl struct {
	orderServiceClient orderProto.OrderServiceClient
	grpcConnection     *grpc.ClientConn
	serverAddress      string
	serverPort         int
	timeout            int
	mux                sync.Mutex
}

func NewOrderService(address string, port int, timeout int) IOrderService {
	return &iOrderServiceImpl{nil, nil, address, port, timeout, sync.Mutex{}}
}

func (order *iOrderServiceImpl) ConnectToOrderService() error {
	if order.grpcConnection == nil {
		order.mux.Lock()
		defer order.mux.Unlock()
		if order.grpcConnection == nil {
			var err error
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			order.grpcConnection, err = grpc.DialContext(ctx, order.serverAddress+":"+fmt.Sprint(order.serverPort),
				grpc.WithBlock(), grpc.WithInsecure())
			if err != nil {
				log.GLog.Logger.Error("GRPC connect dial to order service failed",
					"fn", "ConnectToOrderService",
					"address", order.serverAddress,
					"port", order.serverPort,
					"error", err)
				return err
			}
			order.orderServiceClient = orderProto.NewOrderServiceClient(order.grpcConnection)
		}
	}
	return nil
}

func (order *iOrderServiceImpl) CloseConnection() {
	if err := order.grpcConnection.Close(); err != nil {
		log.GLog.Logger.Error("order CloseConnection failed",
			"error", err)
	}
}

func (order *iOrderServiceImpl) GetFinanceOrderItems(ctx context.Context, filterState FilterState,
	startTimestamp, endTimestamp time.Time, page, perPage uint32) future.IFuture {

	if err := order.ConnectToOrderService(); err != nil {
		return future.Factory().SetCapacity(1).
			SetError(future.InternalError, "Unknown Error", errors.Wrap(err, "ConnectToOrderService failed")).
			BuildAndSend()
	}

	timeoutTimer := time.NewTimer(time.Duration(order.timeout) * time.Second)

	orderFn := func() <-chan interface{} {
		orderChan := make(chan interface{}, 0)
		go func() {
			msgReq := &orderProto.MessageRequest{
				Time: ptypes.TimestampNow(),
				Meta: &orderProto.RequestMetadata{
					Page:           page,
					PerPage:        perPage,
					StartTimestamp: startTimestamp.Format(ISO8601),
					EndTimestamp:   endTimestamp.Format(ISO8601),
					Filters: []*orderProto.MetaFilter{
						{
							Type:  string(OrderStateFilterType),
							Opt:   "eq",
							Value: string(filterState),
						},
					},
				},
			}

			result, err := order.orderServiceClient.FinanceOrderItems(ctx, msgReq)
			if err != nil {
				orderChan <- err
			} else {
				orderChan <- result
			}
		}()
		return orderChan
	}

	var obj interface{} = nil
	select {
	case obj = <-orderFn():
		timeoutTimer.Stop()
		break
	case <-timeoutTimer.C:
		log.GLog.Logger.FromContext(ctx).Error("request to order service grpc timeout",
			"fn", "GetFinanceOrderItems",
			"state", filterState,
			"startTimestamp", startTimestamp.Format(ISO8601),
			"endTimestamp", endTimestamp.Format(ISO8601),
			"page", page,
			"perPage", perPage)
		return future.FactorySync().
			SetError(future.NotAccepted, "Get Finance OrderItem Failed", errors.New("Order Service Timeout")).
			BuildAndSend()
	}

	if e, ok := obj.(error); ok {
		if e != nil {
			log.GLog.Logger.Error("FinanceOrderItems order service failed",
				"fn", "GetFinanceOrderItems",
				"state", filterState,
				"startTimestamp", startTimestamp.Format(ISO8601),
				"endTimestamp", endTimestamp.Format(ISO8601),
				"page", page,
				"perPage", perPage,
				"error", e)
			return future.FactorySync().
				SetError(future.NotAccepted, "Get Finance OrderItem Failed", errors.New("Order Service Timeout")).
				BuildAndSend()
		}
	} else if response, ok := obj.(*orderProto.MessageResponse); ok {
		log.GLog.Logger.Debug("Order FinanceOrderItems success",
			"fn", "GetFinanceOrderItems",
			"state", filterState,
			"startTimestamp", startTimestamp.Format(ISO8601),
			"endTimestamp", endTimestamp.Format(ISO8601),
			"page", page,
			"perPage", perPage)

		if response.Meta.Total == 0 {
			return future.FactorySync().
				SetError(future.NotFound, "Orders Not Found", errors.New("Orders Not Found")).
				BuildAndSend()
		}

		var financeOrderItemDetailList orderProto.FinanceOrderItemDetailList
		if err := ptypes.UnmarshalAny(response.Data, &financeOrderItemDetailList); err != nil {
			log.GLog.Logger.Error("Could not unmarshal FinanceOrderItemDetailList from request anything field",
				"fn", "GetFinanceOrderItems",
				"state", filterState,
				"startTimestamp", startTimestamp.Format(ISO8601),
				"endTimestamp", endTimestamp.Format(ISO8601),
				"page", page,
				"perPage", perPage,
				"error", err)
			return future.FactorySync().
				SetError(future.InternalError, "Get Finance OrderItem Failed", errors.New("FinanceOrderItemDetailList unmarshal failed")).
				BuildAndSend()
		}

		if filterState == PayToSellerFilter {
			sellerOrders, err := convertOrderItemDetailToSellerFinance(ctx, financeOrderItemDetailList)
			if err != nil {
				log.GLog.Logger.Error("FinanceOrderItems of order service failed",
					"fn", "GetFinanceOrderItems",
					"state", filterState,
					"startTimestamp", startTimestamp.Format(ISO8601),
					"endTimestamp", endTimestamp.Format(ISO8601),
					"page", page,
					"perPage", perPage,
					"error", err)
				return future.FactorySync().
					SetError(future.NotAccepted, "Finance OrderItemDetail Invalid", errors.New("Finance OrderItemDetail Invalid")).
					BuildAndSend()
			}

			return future.FactorySync().
				SetData(&OrderServiceResult{
					SellerOrders: sellerOrders,
					TotalCount:   int64(response.Meta.Total),
				}).BuildAndSend()
		}
	}

	return future.FactorySync().
		SetError(future.NotAccepted, "Get FinanceOrderItemDetail Failed", errors.New("Get FinanceOrderItemDetail Failed")).
		BuildAndSend()
}

func convertOrderItemDetailToSellerFinance(ctx context.Context,
	financeOrderItemDetailList orderProto.FinanceOrderItemDetailList) ([]*entities.SellerOrder, error) {

	sellerOrders := make([]*entities.SellerOrder, 0, len(financeOrderItemDetailList.OrderItems))
	for _, orderItemDetail := range financeOrderItemDetailList.OrderItems {
		sellerOrder := &entities.SellerOrder{
			OId:      orderItemDetail.OId,
			SellerId: orderItemDetail.SellerId,
			ShipmentAmount: &entities.Money{
				Amount:   orderItemDetail.ShipmentAmount.Amount,
				Currency: orderItemDetail.ShipmentAmount.Currency,
			},
			RawShippingNet: &entities.Money{
				Amount:   orderItemDetail.RawShippingNet.Amount,
				Currency: orderItemDetail.RawShippingNet.Currency,
			},
			RoundupShippingNet: &entities.Money{
				Amount:   orderItemDetail.RoundupShippingNet.Amount,
				Currency: orderItemDetail.RoundupShippingNet.Currency,
			},
			IsAlreadyShippingPay: false,
			Items:                nil,
			OrderCreatedAt:       nil,
			SubPkgCreatedAt:      nil,
			SubPkgUpdatedAt:      nil,
			DeletedAt:            nil,
		}

		createdAt, err := time.Parse(ISO8601, orderItemDetail.CreatedAt)
		if err != nil {
			log.GLog.Logger.Error("CreatedAt timestamp invalid",
				"fn", "convertOrderItemDetailToSellerFinance",
				"oid", orderItemDetail.OId,
				"sellerId", orderItemDetail.SellerId,
				"createdAt", orderItemDetail.CreatedAt,
				"error", err)
			return nil, err
		}
		sellerOrder.SubPkgCreatedAt = createdAt

		updatedAt, err := time.Parse(ISO8601, orderItemDetail.UpdatedAt)
		if err != nil {
			log.GLog.Logger.Error("UpdatedAt timestamp invalid",
				"fn", "convertOrderItemDetailToSellerFinance",
				"oid", orderItemDetail.OId,
				"sellerId", orderItemDetail.SellerId,
				"updatedAt", orderItemDetail.UpdatedAt,
				"error", err)
			return nil, err
		}
		sellerOrder.SubPkgUpdatedAt = updatedAt

		orderCreatedAt, err := time.Parse(ISO8601, orderItemDetail.OrderCreatedAt)
		if err != nil {
			log.GLog.Logger.Error("OrderCreatedAt timestamp invalid",
				"fn", "convertOrderItemDetailToSellerFinance",
				"oid", orderItemDetail.OId,
				"sellerId", orderItemDetail.SellerId,
				"orderCreatedAt", orderItemDetail.OrderCreatedAt,
				"error", err)
			return nil, err
		}
		sellerOrder.OrderCreatedAt = orderCreatedAt

		sellerOrder.Items = make([]*entities.SellerItem, 0, len(orderItemDetail.Items))
		for _, ItemDetail := range orderItemDetail.Items {
			sellerItem := &entities.SellerItem{
				SId:         ItemDetail.SId,
				SKU:         ItemDetail.Sku,
				InventoryId: ItemDetail.InventoryId,
				Title:       ItemDetail.Title,
				Brand:       ItemDetail.Brand,
				Guaranty:    ItemDetail.Guaranty,
				Category:    ItemDetail.Category,
				Image:       ItemDetail.Image,
				Returnable:  ItemDetail.Returnable,
				Quantity:    ItemDetail.Quantity,
				Attributes:  nil,
				Invoice: &entities.ItemInvoice{
					Commission: &entities.ItemCommission{
						ItemCommission: ItemDetail.Invoice.Commission.ItemCommission,
						RawUnitPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.Commission.RawUnitPrice.Amount,
							Currency: ItemDetail.Invoice.Commission.RawUnitPrice.Currency,
						},
						RoundupUnitPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.Commission.RoundupUnitPrice.Amount,
							Currency: ItemDetail.Invoice.Commission.RoundupUnitPrice.Currency,
						},
						RawTotalPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.Commission.RawTotalPrice.Amount,
							Currency: ItemDetail.Invoice.Commission.RawTotalPrice.Currency,
						},
						RoundupTotalPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.Commission.RoundupTotalPrice.Amount,
							Currency: ItemDetail.Invoice.Commission.RoundupTotalPrice.Currency,
						},
					},
					Share: &entities.ItemShare{
						RawItemNet: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RawItemNet.Amount,
							Currency: ItemDetail.Invoice.Share.RawItemNet.Currency,
						},
						RoundupItemNet: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RoundupItemNet.Amount,
							Currency: ItemDetail.Invoice.Share.RoundupItemNet.Currency,
						},
						RawTotalNet: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RawTotalNet.Amount,
							Currency: ItemDetail.Invoice.Share.RawTotalNet.Currency,
						},
						RoundupTotalNet: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RoundupTotalNet.Amount,
							Currency: ItemDetail.Invoice.Share.RoundupTotalNet.Currency,
						},
						RawUnitSellerShare: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RawUnitSellerShare.Amount,
							Currency: ItemDetail.Invoice.Share.RawUnitSellerShare.Currency,
						},
						RoundupUnitSellerShare: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RoundupUnitSellerShare.Amount,
							Currency: ItemDetail.Invoice.Share.RoundupUnitSellerShare.Currency,
						},
						RawTotalSellerShare: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RawTotalSellerShare.Amount,
							Currency: ItemDetail.Invoice.Share.RawTotalSellerShare.Currency,
						},
						RoundupTotalSellerShare: &entities.Money{
							Amount:   ItemDetail.Invoice.Share.RoundupTotalSellerShare.Amount,
							Currency: ItemDetail.Invoice.Share.RoundupTotalSellerShare.Currency,
						},
					},
					SSO: &entities.ItemSSO{
						Rate:      ItemDetail.Invoice.SSO.Rate,
						IsObliged: ItemDetail.Invoice.SSO.IsObliged,
						RawUnitPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.SSO.RawUnitPrice.Amount,
							Currency: ItemDetail.Invoice.SSO.RawUnitPrice.Currency,
						},
						RoundupUnitPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.SSO.RoundupUnitPrice.Amount,
							Currency: ItemDetail.Invoice.SSO.RoundupUnitPrice.Currency,
						},
						RawTotalPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.SSO.RawTotalPrice.Amount,
							Currency: ItemDetail.Invoice.SSO.RawTotalPrice.Currency,
						},
						RoundupTotalPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.SSO.RoundupTotalPrice.Amount,
							Currency: ItemDetail.Invoice.SSO.RoundupTotalPrice.Currency,
						},
					},
					VAT: &entities.ItemVAT{
						Rate:      ItemDetail.Invoice.VAT.Rate,
						IsObliged: ItemDetail.Invoice.VAT.IsObliged,
						RawUnitPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.VAT.RawUnitPrice.Amount,
							Currency: ItemDetail.Invoice.VAT.RawUnitPrice.Currency,
						},
						RoundupUnitPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.VAT.RoundupUnitPrice.Amount,
							Currency: ItemDetail.Invoice.VAT.RoundupUnitPrice.Currency,
						},
						RawTotalPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.VAT.RawTotalPrice.Amount,
							Currency: ItemDetail.Invoice.VAT.RawTotalPrice.Currency,
						},
						RoundupTotalPrice: &entities.Money{
							Amount:   ItemDetail.Invoice.VAT.RoundupTotalPrice.Amount,
							Currency: ItemDetail.Invoice.VAT.RoundupTotalPrice.Currency,
						},
					},
				},
			}

			if ItemDetail.Attributes != nil {
				sellerItem.Attributes = make(map[string]*entities.Attribute, len(ItemDetail.Attributes))
				for attrKey, attribute := range ItemDetail.Attributes {
					keyTranslates := make(map[string]string, len(attribute.KeyTrans))
					for keyTran, value := range attribute.KeyTrans {
						keyTranslates[keyTran] = value
					}
					valTranslates := make(map[string]string, len(attribute.ValueTrans))
					for valTran, value := range attribute.ValueTrans {
						valTranslates[valTran] = value
					}
					sellerItem.Attributes[attrKey] = &entities.Attribute{
						KeyTranslate:   keyTranslates,
						ValueTranslate: valTranslates,
					}
				}
			}

			sellerOrder.Items = append(sellerOrder.Items, sellerItem)
		}

		sellerOrders = append(sellerOrders, sellerOrder)
	}

	return sellerOrders, nil
}
