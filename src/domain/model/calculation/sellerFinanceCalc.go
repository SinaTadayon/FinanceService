package calculation

import (
	"context"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gitlab.faza.io/services/finance/domain/model/entities"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SellerFinanceCalc struct {
	FId        string
	SellerId   uint64
	Invoice    *InvoiceCalc
	OrdersInfo []*OrderInfoCalc
}

type InvoiceCalc struct {
	SSORawTotal            *decimal.Decimal
	SSORoundupTotal        *decimal.Decimal
	VATRawTotal            *decimal.Decimal
	VATRoundupTotal        *decimal.Decimal
	CommissionRawTotal     *decimal.Decimal
	CommissionRoundupTotal *decimal.Decimal
	ShareRawTotal          *decimal.Decimal
	ShareRoundupTotal      *decimal.Decimal
	ShipmentRawTotal       *decimal.Decimal
	ShipmentRoundupTotal   *decimal.Decimal
}

type OrderInfoCalc struct {
	TriggerName    string
	TriggerHistory primitive.ObjectID
	Orders         []*SellerOrderCalc
}

type SellerOrderCalc struct {
	OId                  uint64
	SellerId             uint64
	ShipmentAmount       *decimal.Decimal
	RawShippingNet       *decimal.Decimal
	RoundupShippingNet   *decimal.Decimal
	IsAlreadyShippingPay bool
	Items                []*SellerItemCalc
}

type SellerItemCalc struct {
	SId         uint64
	InventoryId string
	Quantity    int32
	Invoice     *ItemInvoiceCalc
}

type ItemInvoiceCalc struct {
	Commission *ItemCommissionCalc
	Share      *ItemShareCalc
	SSO        *ItemSSOCalc
	VAT        *ItemVATCalc
}

type ItemShareCalc struct {
	RawItemNet              *decimal.Decimal
	RoundupItemNet          *decimal.Decimal
	RawTotalNet             *decimal.Decimal
	RoundupTotalNet         *decimal.Decimal
	RawUnitSellerShare      *decimal.Decimal
	RoundupUnitSellerShare  *decimal.Decimal
	RawTotalSellerShare     *decimal.Decimal
	RoundupTotalSellerShare *decimal.Decimal
}

type ItemCommissionCalc struct {
	ItemCommission    float32
	RawUnitPrice      *decimal.Decimal
	RoundupUnitPrice  *decimal.Decimal
	RawTotalPrice     *decimal.Decimal
	RoundupTotalPrice *decimal.Decimal
}

type ItemSSOCalc struct {
	Rate              float32
	IsObliged         bool
	RawUnitPrice      *decimal.Decimal
	RoundupUnitPrice  *decimal.Decimal
	RawTotalPrice     *decimal.Decimal
	RoundupTotalPrice *decimal.Decimal
}

type ItemVATCalc struct {
	Rate              float32
	IsObliged         bool
	RawUnitPrice      *decimal.Decimal
	RoundupUnitPrice  *decimal.Decimal
	RawTotalPrice     *decimal.Decimal
	RoundupTotalPrice *decimal.Decimal
}

func FinanceCalcFactory(ctx context.Context, finance *entities.SellerFinance) (*SellerFinanceCalc, error) {
	financeCalc := &SellerFinanceCalc{
		FId:        finance.FId,
		SellerId:   finance.SellerId,
		Invoice:    nil,
		OrdersInfo: nil,
	}

	if financeCalc.Invoice != nil {
		financeCalc.Invoice = &InvoiceCalc{
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
			ssoRawTotal, err := decimal.NewFromString(finance.Invoice.SSORawTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.GrandTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"ssoRawTotal", finance.Invoice.SSORawTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.SSORawTotal = &ssoRawTotal
		}

		if finance.Invoice.SSORoundupTotal != nil {
			ssoRoundupTotal, err := decimal.NewFromString(finance.Invoice.SSORoundupTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.SSORoundupTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"ssoRoundupTotal", finance.Invoice.SSORoundupTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.SSORoundupTotal = &ssoRoundupTotal
		}

		if finance.Invoice.VATRawTotal != nil {
			vatRawTotal, err := decimal.NewFromString(finance.Invoice.VATRawTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.VATRawTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"vatRawTotal", finance.Invoice.VATRawTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.VATRawTotal = &vatRawTotal
		}

		if finance.Invoice.VATRoundupTotal != nil {
			vatRoundupTotal, err := decimal.NewFromString(finance.Invoice.VATRoundupTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.VATRoundupTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"vatRoundupTotal", finance.Invoice.VATRoundupTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.VATRoundupTotal = &vatRoundupTotal
		}

		if finance.Invoice.CommissionRawTotal != nil {
			commissionRawTotal, err := decimal.NewFromString(finance.Invoice.CommissionRawTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.CommissionRawTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"commissionRawTotal", finance.Invoice.CommissionRawTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.CommissionRawTotal = &commissionRawTotal
		}

		if finance.Invoice.CommissionRoundupTotal != nil {
			commissionRoundupTotal, err := decimal.NewFromString(finance.Invoice.CommissionRoundupTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.CommissionRoundupTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"commissionRoundupTotal", finance.Invoice.CommissionRoundupTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.CommissionRoundupTotal = &commissionRoundupTotal
		}

		if finance.Invoice.ShareRawTotal != nil {
			shareRawTotal, err := decimal.NewFromString(finance.Invoice.ShareRawTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.ShareRawTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"shareRawTotal", finance.Invoice.ShareRawTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.ShareRawTotal = &shareRawTotal
		}

		if finance.Invoice.ShareRoundupTotal != nil {
			shareRoundupTotal, err := decimal.NewFromString(finance.Invoice.ShareRoundupTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.ShareRoundupTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"shareRoundupTotal", finance.Invoice.ShareRoundupTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.ShareRoundupTotal = &shareRoundupTotal
		}

		if finance.Invoice.ShipmentRawTotal != nil {
			shipmentRawTotal, err := decimal.NewFromString(finance.Invoice.ShipmentRawTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.ShipmentRawTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"shipmentRawTotal", finance.Invoice.ShipmentRawTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.ShipmentRawTotal = &shipmentRawTotal
		}

		if finance.Invoice.ShipmentRoundupTotal != nil {
			shipmentRoundupTotal, err := decimal.NewFromString(finance.Invoice.ShipmentRoundupTotal.Amount)
			if err != nil {
				log.GLog.Logger.Error("finance.Invoice.ShipmentRoundupTotal.Amount invalid",
					"fn", "SellerFinanceCalcFactory",
					"shipmentRoundupTotal", finance.Invoice.ShipmentRoundupTotal.Amount,
					"fid", finance.FId,
					"sellerId", finance.SellerId,
					"error", err)
				return nil, err
			}
			financeCalc.Invoice.ShipmentRoundupTotal = &shipmentRoundupTotal
		}
	}

	financeCalc.OrdersInfo = make([]*OrderInfoCalc, 0, len(finance.OrdersInfo))
	for _, orderInfo := range finance.OrdersInfo {
		orderInfoCalc := &OrderInfoCalc{
			TriggerName:    orderInfo.TriggerName,
			TriggerHistory: orderInfo.TriggerHistoryId,
			Orders:         nil,
		}

		orderInfoCalc.Orders = make([]*SellerOrderCalc, 0, len(orderInfo.Orders))
		for _, order := range orderInfo.Orders {
			orderCalc := &SellerOrderCalc{
				OId:                  order.OId,
				SellerId:             order.SellerId,
				ShipmentAmount:       nil,
				RawShippingNet:       nil,
				RoundupShippingNet:   nil,
				IsAlreadyShippingPay: order.IsAlreadyShippingPay,
				Items:                nil,
			}

			if order.ShipmentAmount != nil {
				shipmentAmount, err := decimal.NewFromString(order.ShipmentAmount.Amount)
				if err != nil {
					log.GLog.Logger.Error("order.ShipmentAmount.Amount invalid",
						"fn", "SellerFinanceCalcFactory",
						"shipmentAmount", order.ShipmentAmount.Amount,
						"fid", finance.FId,
						"sellerId", finance.SellerId,
						"oid", order.OId,
						"error", err)
					return nil, err
				}
				orderCalc.ShipmentAmount = &shipmentAmount
			}

			if order.RawShippingNet != nil {
				rawShippingNet, err := decimal.NewFromString(order.RawShippingNet.Amount)
				if err != nil {
					log.GLog.Logger.Error("order.RawShippingNet.Amount invalid",
						"fn", "SellerFinanceCalcFactory",
						"rawShippingNet", order.RawShippingNet.Amount,
						"fid", finance.FId,
						"sellerId", finance.SellerId,
						"oid", order.OId,
						"error", err)
					return nil, err
				}
				orderCalc.RawShippingNet = &rawShippingNet
			}

			if order.RoundupShippingNet != nil {
				roundupShippingNet, err := decimal.NewFromString(order.RoundupShippingNet.Amount)
				if err != nil {
					log.GLog.Logger.Error("order.RoundupShippingNet.Amount invalid",
						"fn", "SellerFinanceCalcFactory",
						"roundupShippingNet", order.RoundupShippingNet.Amount,
						"fid", finance.FId,
						"sellerId", finance.SellerId,
						"oid", order.OId,
						"error", err)
					return nil, err
				}
				orderCalc.RoundupShippingNet = &roundupShippingNet
			}

			orderCalc.Items = make([]*SellerItemCalc, 0, len(order.Items))
			for _, item := range order.Items {
				itemCalc := &SellerItemCalc{
					SId:         item.SId,
					InventoryId: item.InventoryId,
					Quantity:    item.Quantity,
					Invoice: &ItemInvoiceCalc{
						Commission: nil,
						Share:      nil,
						SSO:        nil,
						VAT:        nil,
					},
				}

				if item.Invoice.Commission != nil {
					itemCalc.Invoice.Commission = &ItemCommissionCalc{
						ItemCommission:    item.Invoice.Commission.ItemCommission,
						RawUnitPrice:      nil,
						RoundupUnitPrice:  nil,
						RawTotalPrice:     nil,
						RoundupTotalPrice: nil,
					}

					if item.Invoice.Commission.RawUnitPrice != nil {
						rawUnitPrice, err := decimal.NewFromString(item.Invoice.Commission.RawUnitPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Commission.RawUnitPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawUnitPrice", item.Invoice.Commission.RawUnitPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Commission.RawUnitPrice = &rawUnitPrice
					}

					if item.Invoice.Commission.RawTotalPrice != nil {
						rawTotalPrice, err := decimal.NewFromString(item.Invoice.Commission.RawTotalPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Commission.RawTotalPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawTotalPrice", item.Invoice.Commission.RawTotalPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Commission.RawTotalPrice = &rawTotalPrice
					}

					if item.Invoice.Commission.RoundupUnitPrice != nil {
						roundupUnitPrice, err := decimal.NewFromString(item.Invoice.Commission.RoundupUnitPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Commission.RoundupUnitPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupUnitPrice", item.Invoice.Commission.RoundupUnitPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Commission.RoundupUnitPrice = &roundupUnitPrice
					}

					if item.Invoice.Commission.RoundupTotalPrice != nil {
						roundupTotalPrice, err := decimal.NewFromString(item.Invoice.Commission.RoundupTotalPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Commission.RoundupTotalPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupTotalPrice", item.Invoice.Commission.RoundupTotalPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Commission.RoundupTotalPrice = &roundupTotalPrice
					}
				}

				if item.Invoice.Share != nil {
					itemCalc.Invoice.Share = &ItemShareCalc{
						RawItemNet:              nil,
						RoundupItemNet:          nil,
						RawTotalNet:             nil,
						RoundupTotalNet:         nil,
						RawUnitSellerShare:      nil,
						RoundupUnitSellerShare:  nil,
						RawTotalSellerShare:     nil,
						RoundupTotalSellerShare: nil,
					}

					if item.Invoice.Share.RawItemNet != nil {
						rawItemNet, err := decimal.NewFromString(item.Invoice.Share.RawItemNet.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RawItemNet.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawItemNet", item.Invoice.Share.RawItemNet.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RawItemNet = &rawItemNet
					}

					if item.Invoice.Share.RawTotalNet != nil {
						rawTotalNet, err := decimal.NewFromString(item.Invoice.Share.RawTotalNet.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RawTotalNet.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawTotalNet", item.Invoice.Share.RawTotalNet.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RawTotalNet = &rawTotalNet
					}

					if item.Invoice.Share.RoundupItemNet != nil {
						roundupItemNet, err := decimal.NewFromString(item.Invoice.Share.RoundupItemNet.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RoundupItemNet.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupItemNet", item.Invoice.Share.RoundupItemNet.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RoundupItemNet = &roundupItemNet
					}

					if item.Invoice.Share.RoundupTotalNet != nil {
						roundupTotalNet, err := decimal.NewFromString(item.Invoice.Share.RoundupTotalNet.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RoundupTotalNet.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupTotalNet", item.Invoice.Share.RoundupTotalNet.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RoundupTotalNet = &roundupTotalNet
					}

					if item.Invoice.Share.RawUnitSellerShare != nil {
						rawUnitSellerShare, err := decimal.NewFromString(item.Invoice.Share.RawUnitSellerShare.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RawUnitSellerShare.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawUnitSellerShare", item.Invoice.Share.RawUnitSellerShare.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RawUnitSellerShare = &rawUnitSellerShare
					}

					if item.Invoice.Share.RoundupUnitSellerShare != nil {
						roundupUnitSellerShare, err := decimal.NewFromString(item.Invoice.Share.RoundupUnitSellerShare.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RoundupUnitSellerShare.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupUnitSellerShare", item.Invoice.Share.RoundupUnitSellerShare.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RoundupUnitSellerShare = &roundupUnitSellerShare
					}

					if item.Invoice.Share.RawTotalSellerShare != nil {
						rawTotalSellerShare, err := decimal.NewFromString(item.Invoice.Share.RawTotalSellerShare.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RawTotalSellerShare.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawTotalSellerShare", item.Invoice.Share.RawTotalSellerShare.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RawTotalSellerShare = &rawTotalSellerShare
					}

					if item.Invoice.Share.RoundupTotalSellerShare != nil {
						roundupTotalSellerShare, err := decimal.NewFromString(item.Invoice.Share.RoundupTotalSellerShare.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.Share.RoundupTotalSellerShare.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupTotalSellerShare", item.Invoice.Share.RoundupTotalSellerShare.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.Share.RoundupTotalSellerShare = &roundupTotalSellerShare
					}
				}

				if item.Invoice.SSO != nil {
					itemCalc.Invoice.SSO = &ItemSSOCalc{
						Rate:              item.Invoice.SSO.Rate,
						IsObliged:         item.Invoice.SSO.IsObliged,
						RawUnitPrice:      nil,
						RoundupUnitPrice:  nil,
						RawTotalPrice:     nil,
						RoundupTotalPrice: nil,
					}

					if item.Invoice.SSO.RawUnitPrice != nil {
						rawUnitPrice, err := decimal.NewFromString(item.Invoice.SSO.RawUnitPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.SSO.RawUnitPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawUnitPrice", item.Invoice.SSO.RawUnitPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.SSO.RawUnitPrice = &rawUnitPrice
					}

					if item.Invoice.SSO.RawTotalPrice != nil {
						rawTotalPrice, err := decimal.NewFromString(item.Invoice.SSO.RawTotalPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.SSO.RawTotalPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawTotalPrice", item.Invoice.SSO.RawTotalPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.SSO.RawTotalPrice = &rawTotalPrice
					}

					if item.Invoice.SSO.RoundupUnitPrice != nil {
						roundupUnitPrice, err := decimal.NewFromString(item.Invoice.SSO.RoundupUnitPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.SSO.RoundupUnitPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupUnitPrice", item.Invoice.SSO.RoundupUnitPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.SSO.RoundupUnitPrice = &roundupUnitPrice
					}

					if item.Invoice.SSO.RoundupTotalPrice != nil {
						roundupTotalPrice, err := decimal.NewFromString(item.Invoice.SSO.RoundupTotalPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.SSO.RoundupTotalPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupTotalPrice", item.Invoice.SSO.RoundupTotalPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.SSO.RoundupTotalPrice = &roundupTotalPrice
					}
				}

				if item.Invoice.VAT != nil {
					itemCalc.Invoice.VAT = &ItemVATCalc{
						Rate:              item.Invoice.VAT.Rate,
						IsObliged:         item.Invoice.VAT.IsObliged,
						RawUnitPrice:      nil,
						RoundupUnitPrice:  nil,
						RawTotalPrice:     nil,
						RoundupTotalPrice: nil,
					}

					if item.Invoice.VAT.RawUnitPrice != nil {
						rawUnitPrice, err := decimal.NewFromString(item.Invoice.VAT.RawUnitPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.VAT.RawUnitPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawUnitPrice", item.Invoice.VAT.RawUnitPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.VAT.RawUnitPrice = &rawUnitPrice
					}

					if item.Invoice.VAT.RawTotalPrice != nil {
						rawTotalPrice, err := decimal.NewFromString(item.Invoice.VAT.RawTotalPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.VAT.RawTotalPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"rawTotalPrice", item.Invoice.VAT.RawTotalPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.VAT.RawTotalPrice = &rawTotalPrice
					}

					if item.Invoice.VAT.RoundupUnitPrice != nil {
						roundupUnitPrice, err := decimal.NewFromString(item.Invoice.VAT.RoundupUnitPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.VAT.RoundupUnitPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupUnitPrice", item.Invoice.VAT.RoundupUnitPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.VAT.RoundupUnitPrice = &roundupUnitPrice
					}

					if item.Invoice.VAT.RoundupTotalPrice != nil {
						roundupTotalPrice, err := decimal.NewFromString(item.Invoice.VAT.RoundupTotalPrice.Amount)
						if err != nil {
							log.GLog.Logger.Error("item.Invoice.VAT.RoundupTotalPrice.Amount invalid",
								"fn", "SellerFinanceCalcFactory",
								"roundupTotalPrice", item.Invoice.VAT.RoundupTotalPrice.Amount,
								"fid", finance.FId,
								"sellerId", finance.SellerId,
								"oid", order.OId,
								"sid", item.SId,
								"inventoryId", item.InventoryId,
								"error", err)
							return nil, err
						}
						itemCalc.Invoice.VAT.RoundupTotalPrice = &roundupTotalPrice
					}
				}

				orderCalc.Items = append(orderCalc.Items, itemCalc)
			}
			orderInfoCalc.Orders = append(orderInfoCalc.Orders, orderCalc)
		}
		financeCalc.OrdersInfo = append(financeCalc.OrdersInfo, orderInfoCalc)
	}

	return financeCalc, nil
}

func ConvertToSellerFinance(ctx context.Context, financeCalc *SellerFinanceCalc, sellerFinance *entities.SellerFinance) error {
	if financeCalc.FId != sellerFinance.FId {
		log.GLog.Logger.Error("finance fid invalid",
			"fn", "ConvertToSellerFinance",
			"financeCalc fid", financeCalc.FId,
			"fid", sellerFinance.FId)
		return errors.New("fid mismatch with financeCalc fid")
	}

	if financeCalc.Invoice != nil {
		if sellerFinance.Invoice == nil {
			sellerFinance.Invoice = &entities.Invoice{
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
		}

		if financeCalc.Invoice.SSORawTotal != nil {
			sellerFinance.Invoice.SSORawTotal = &entities.Money{
				Amount:   financeCalc.Invoice.SSORawTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.SSORoundupTotal != nil {
			sellerFinance.Invoice.SSORoundupTotal = &entities.Money{
				Amount:   financeCalc.Invoice.SSORoundupTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.VATRawTotal != nil {
			sellerFinance.Invoice.VATRawTotal = &entities.Money{
				Amount:   financeCalc.Invoice.VATRawTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.VATRoundupTotal != nil {
			sellerFinance.Invoice.VATRoundupTotal = &entities.Money{
				Amount:   financeCalc.Invoice.VATRoundupTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.CommissionRawTotal != nil {
			sellerFinance.Invoice.CommissionRawTotal = &entities.Money{
				Amount:   financeCalc.Invoice.CommissionRawTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.CommissionRoundupTotal != nil {
			sellerFinance.Invoice.CommissionRoundupTotal = &entities.Money{
				Amount:   financeCalc.Invoice.CommissionRoundupTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.ShareRawTotal != nil {
			sellerFinance.Invoice.ShareRawTotal = &entities.Money{
				Amount:   financeCalc.Invoice.ShareRawTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.ShareRoundupTotal != nil {
			sellerFinance.Invoice.ShareRoundupTotal = &entities.Money{
				Amount:   financeCalc.Invoice.ShareRoundupTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.ShipmentRawTotal != nil {
			sellerFinance.Invoice.ShipmentRawTotal = &entities.Money{
				Amount:   financeCalc.Invoice.ShipmentRawTotal.String(),
				Currency: "IRR",
			}
		}

		if financeCalc.Invoice.ShipmentRoundupTotal != nil {
			sellerFinance.Invoice.ShipmentRoundupTotal = &entities.Money{
				Amount:   financeCalc.Invoice.ShipmentRoundupTotal.String(),
				Currency: "IRR",
			}
		}
	}

	return nil
}
