package converter

import (
	"context"
	"github.com/pkg/errors"
	financesrv "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/domain/model/repository/sellerOrderItem"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"gitlab.faza.io/services/finance/infrastructure/utils"
)

const (
	// payment status
	paymentCalculation paymentStatus = "Calculation"
	paymentPending     paymentStatus = "Pending"
	paymentSucceed     paymentStatus = "Succeed"
	paymentFailed      paymentStatus = "Failed"
	paymentPartial     paymentStatus = "Partial"

	// payment types
	Shipment paymentType = "Shipment"
	Purchase paymentType = "Purchase"
)

type (
	paymentType   string
	paymentStatus string
	Converter     ConverterT
)

func NewConverter() IConverter {
	return new(Converter)
}

func (c *Converter) Convert(ctx context.Context, in interface{}, out interface{}) (interface{}, error) {
	switch in.(type) {
	case sellerOrderItem.SellerOrderItems:
		switch out.(type) {
		case financesrv.SellerFinanceOrderItemCollection:
			return convertSellerOrderItemsToSellerFinanceOrderItemCollection(in.(sellerOrderItem.SellerOrderItems))
		}
	case finance_repository.FinancePageableResult:
		switch out.(type) {
		case financesrv.SellerFinanceListCollection:
			return convertFinancePageableResultToSellerFinanceListCollection(in.(finance_repository.FinancePageableResult))
		}
	}

	log.GLog.Logger.Error("mapping from input type not supported",
		"fn", "Convert",
		"in", in)
	return nil, errors.New("mapping from input type not supported")

}

//========================== converter functions
func convertFinancePageableResultToSellerFinanceListCollection(input finance_repository.FinancePageableResult) (financesrv.SellerFinanceListCollection, error) {
	items := make([]*financesrv.SellerFinanceList, 0, len(input.SellerFinances))

	for _, item := range input.SellerFinances {
		var (
			paymentStatus string
			total         financesrv.Money
			startAt       string
			endAt         string
		)

		paymentStatus, total = resolveFinanceStat(item)

		if item.StartAt != nil {
			startAt = item.StartAt.Format(utils.ISO8601)
		} else {
			startAt = item.CreatedAt.Format(utils.ISO8601)
		}

		if item.EndAt != nil {
			endAt = item.EndAt.Format(utils.ISO8601)
		}

		rowItem := financesrv.SellerFinanceList{
			FID:           item.FId,
			UID:           item.SellerId,
			StartDate:     startAt,
			EndDate:       endAt,
			PaymentStatus: paymentStatus,
			Total:         &total,
		}

		items = append(items, &rowItem)
	}

	coll := financesrv.SellerFinanceListCollection{
		Items: items,
	}

	return coll, nil
}

func convertSellerOrderItemsToSellerFinanceOrderItemCollection(input sellerOrderItem.SellerOrderItems) (financesrv.SellerFinanceOrderItemCollection, error) {
	history := make(map[uint64]bool, 0)
	output := financesrv.SellerFinanceOrderItemCollection{
		Items: make([]*financesrv.SellerFinanceOrderItemList, 0, len(input.SellerFinances)),
	}

	var financeInvoice financesrv.SellerFinanceOrderItemCollection_SellerFinanceInvoice

	if len(input.SellerFinances) > 0 {
		item0 := input.SellerFinances[0]

		if item0.Status != entities.FinanceOrderCollectionStatus && item0.Invoice != nil {
			if item0.Invoice.CommissionRoundupTotal != nil {
				financeInvoice.Commission = &financesrv.Money{
					Amount:   item0.Invoice.CommissionRoundupTotal.Amount,
					Currency: item0.Invoice.CommissionRoundupTotal.Currency,
				}
			}

			if item0.Invoice.ShareRoundupTotal != nil {
				financeInvoice.Share = &financesrv.Money{
					Amount:   item0.Invoice.ShareRoundupTotal.Amount,
					Currency: item0.Invoice.ShareRoundupTotal.Currency,
				}
			}

			if item0.Invoice.ShipmentRoundupTotal != nil {
				financeInvoice.Shipment = &financesrv.Money{
					Amount:   item0.Invoice.ShipmentRoundupTotal.Amount,
					Currency: item0.Invoice.ShipmentRoundupTotal.Currency,
				}
			}

			if item0.Invoice.VATRoundupTotal != nil {
				financeInvoice.VAT = &financesrv.Money{
					Amount:   item0.Invoice.VATRoundupTotal.Amount,
					Currency: item0.Invoice.VATRoundupTotal.Currency,
				}
			}
		}
	}

	output.FinanceInvoice = &financeInvoice
	for _, fi := range input.SellerFinances {
		//=============== the collecting data didn't have any calculated value
		if fi.Status == entities.FinanceOrderCollectionStatus {
			attr := make(map[string]*financesrv.SellerFinanceOrderItemListAttribute)

			for key, value := range fi.OrderInfo.Order.Item.Attributes {
				attr[key] = &financesrv.SellerFinanceOrderItemListAttribute{
					KeyTrans:   value.KeyTranslate,
					ValueTrans: value.ValueTranslate,
				}
			}

			item := financesrv.SellerFinanceOrderItemList{
				PaymentType:   string(Purchase),
				OrderId:       fi.OrderInfo.Order.OId,
				Title:         fi.OrderInfo.Order.Item.Title,
				Brand:         fi.OrderInfo.Order.Item.Brand,
				Attribute:     attr,
				Category:      fi.OrderInfo.Order.Item.Category,
				Guaranty:      fi.OrderInfo.Order.Item.Guaranty,
				OrderCreateAt: fi.OrderInfo.Order.OrderCreatedAt.Format(utils.ISO8601),
				Quantity:      fi.OrderInfo.Order.Item.Quantity,
				SKU:           fi.OrderInfo.Order.Item.SKU,
			}

			output.Items = append(output.Items, &item)
			continue
		}

		//=============== find out type of payment
		if fi.OrderInfo.Order.IsAlreadyShippingPayed == false {
			if _, ok := history[fi.OrderInfo.Order.OId]; !ok {
				item := financesrv.SellerFinanceOrderItemList{
					PaymentType:   string(Shipment),
					OrderId:       fi.OrderInfo.Order.OId,
					OrderCreateAt: fi.OrderInfo.Order.OrderCreatedAt.Format(utils.ISO8601),
					Quantity:      fi.OrderInfo.Order.Item.Quantity,
					SKU:           fi.OrderInfo.Order.Item.SKU,
					ShippingFee: &financesrv.Money{
						Amount:   fi.OrderInfo.Order.RoundupShippingNet.Amount,
						Currency: fi.OrderInfo.Order.RoundupShippingNet.Currency,
					},
				}

				//=============== add a row for shipment amount
				output.Items = append(output.Items, &item)
				history[fi.OrderInfo.Order.OId] = true
			}
		}

		//=============== invoice and variables for dto
		invoice := fi.OrderInfo.Order.Item.Invoice
		var share financesrv.SellerFinanceOrderItemList_SellerShare
		var commission financesrv.SellerFinanceOrderItemList_SellerCommission
		var sso, vat financesrv.SellerFinanceOrderItemList_RatedMoney

		//=============== share amount
		if invoice.Share.RoundupTotalSellerShare != nil && invoice.Share.RoundupUnitSellerShare != nil {
			share = financesrv.SellerFinanceOrderItemList_SellerShare{
				RoundupTotal: &financesrv.Money{
					Amount:   invoice.Share.RoundupTotalSellerShare.Amount,
					Currency: invoice.Share.RoundupTotalSellerShare.Currency,
				},
				RoundupUnit: &financesrv.Money{
					Amount:   invoice.Share.RoundupUnitSellerShare.Amount,
					Currency: invoice.Share.RoundupUnitSellerShare.Currency,
				},
			}
		}

		//=============== sso per item and total package
		if invoice.SSO.Rate != 0 && invoice.SSO.RoundupTotalPrice != nil && invoice.SSO.RoundupUnitPrice != nil {
			sso = financesrv.SellerFinanceOrderItemList_RatedMoney{
				Rate:      invoice.SSO.Rate,
				IsObliged: invoice.SSO.IsObliged,
				RoundupUnit: &financesrv.Money{
					Amount:   invoice.SSO.RoundupUnitPrice.Amount,
					Currency: invoice.SSO.RoundupUnitPrice.Currency,
				},
				RoundupTotal: &financesrv.Money{
					Amount:   invoice.SSO.RoundupTotalPrice.Amount,
					Currency: invoice.SSO.RoundupTotalPrice.Currency,
				},
			}
		}

		//=============== vat per item and total package
		if invoice.VAT.Rate != 0 && invoice.VAT.RoundupTotalPrice != nil && invoice.VAT.RoundupUnitPrice != nil {
			vat = financesrv.SellerFinanceOrderItemList_RatedMoney{
				Rate:      invoice.VAT.Rate,
				IsObliged: invoice.VAT.IsObliged,
				RoundupUnit: &financesrv.Money{
					Amount:   invoice.VAT.RoundupUnitPrice.Amount,
					Currency: invoice.VAT.RoundupUnitPrice.Currency,
				},
				RoundupTotal: &financesrv.Money{
					Amount:   invoice.VAT.RoundupTotalPrice.Amount,
					Currency: invoice.VAT.RoundupTotalPrice.Currency,
				},
			}
		}

		//=============== commission per item and total package
		if invoice.Commission.ItemCommission != 0 && invoice.Commission.RoundupTotalPrice != nil && invoice.Commission.RoundupUnitPrice != nil {
			commission = financesrv.SellerFinanceOrderItemList_SellerCommission{
				Rate: invoice.Commission.ItemCommission,
				RoundupUnit: &financesrv.Money{
					Amount:   invoice.Commission.RoundupUnitPrice.Amount,
					Currency: invoice.Commission.RoundupUnitPrice.Currency,
				},
				RoundupTotal: &financesrv.Money{
					Amount:   invoice.Commission.RoundupTotalPrice.Amount,
					Currency: invoice.Commission.RoundupTotalPrice.Currency,
				},
			}
		}

		//=============== final item
		attr := make(map[string]*financesrv.SellerFinanceOrderItemListAttribute)

		for key, value := range fi.OrderInfo.Order.Item.Attributes {
			attr[key] = &financesrv.SellerFinanceOrderItemListAttribute{
				KeyTrans:   value.KeyTranslate,
				ValueTrans: value.ValueTranslate,
			}
		}

		item := financesrv.SellerFinanceOrderItemList{
			PaymentType:   string(Purchase),
			OrderId:       fi.OrderInfo.Order.OId,
			Title:         fi.OrderInfo.Order.Item.Title,
			Brand:         fi.OrderInfo.Order.Item.Brand,
			Attribute:     attr,
			Category:      fi.OrderInfo.Order.Item.Category,
			Guaranty:      fi.OrderInfo.Order.Item.Guaranty,
			OrderCreateAt: fi.OrderInfo.Order.OrderCreatedAt.Format(utils.ISO8601),
			Quantity:      fi.OrderInfo.Order.Item.Quantity,
			SKU:           fi.OrderInfo.Order.Item.SKU,
			Share:         &share,
			SSO:           &sso,
			VAT:           &vat,
			Commission:    &commission,
		}

		output.Items = append(output.Items, &item)
	}

	return output, nil
}

//============================= other functions
func resolveFinanceStat(item *entities.SellerFinance) (paymentStatus string, total financesrv.Money) {
	switch item.Status {
	case entities.FinanceOrderCollectionStatus:
		paymentStatus = string(paymentCalculation)

	case entities.FinancePaymentProcessStatus:
		paymentStatus = string(paymentPending)

	case entities.FinanceClosedStatus:
		switch item.Payment.Status {
		case entities.TransferSuccessState:
			paymentStatus = string(paymentSucceed)

		case entities.TransferFailedState:
			paymentStatus = string(paymentFailed)

		case entities.TransferPartialState:
			paymentStatus = string(paymentPartial)
		}
	}

	if item.Payment.TransferRequest != nil {
		total = financesrv.Money{
			Amount:   item.Payment.TransferRequest.TotalPrice.Amount,
			Currency: item.Payment.TransferRequest.TotalPrice.Currency,
		}
	}

	return
}
