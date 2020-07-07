//
// return a seller finance list for request
package imp

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/domain/model/entities"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/infrastructure/future"
	handler2 "gitlab.faza.io/services/finance/infrastructure/handler"
	"gitlab.faza.io/services/finance/infrastructure/utils"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	paymentCalculation paymentStatus = "Calculation"
	paymentPending     paymentStatus = "Pending"
	paymentSucceed     paymentStatus = "Succeed"
	paymentFailed      paymentStatus = "Failed"
	paymentPartial     paymentStatus = "Partial"
)

type (
	paymentStatus            string
	sellerFinanceListHandler struct {
		repo finance_repository.ISellerFinanceRepository
	}
)

func NewSellerFinanceListHandler(repo finance_repository.ISellerFinanceRepository) handler2.IHandler {
	handler := sellerFinanceListHandler{
		repo: repo,
	}

	return &handler
}

func (s sellerFinanceListHandler) Handle(input interface{}) future.IFuture {
	var (
		sortName string
		sortDire int
	)
	req := input.(*finance_proto.RequestMessage)

	if req.Header.Sorts.Name == "" {
		sortName = ""
	} else {
		sortName = req.Header.Sorts.Name
	}

	switch req.Header.Sorts.Dir {
	case uint32(finance_proto.RequestMetaSorts_Descending):
		sortDire = -1
	case uint32(finance_proto.RequestMetaSorts_Ascending):
		sortDire = 1
	default:
		sortDire = 0
	}

	filter := func() (interface{}, string, int) {
		return bson.D{{"sellerId", req.Header.UID}}, sortName, sortDire
	}

	ctx := context.Background()
	res := s.repo.FindByFilterWithPageAndSort(ctx, filter, int64(req.Header.Page), int64(req.Header.PerPage)).Get()

	if res.Error() != nil {
		return future.FactorySync().
			SetError(res.Error().Code(), res.Error().Message(), res.Error().Reason()).
			BuildAndSend()
	}

	dbResult := res.Data().(finance_repository.FinancePageableResult)
	items := make([]*finance_proto.SellerFinanceList, 0, len(dbResult.SellerFinances))

	for _, item := range dbResult.SellerFinances {
		var (
			paymentStatus string
			total         finance_proto.Money
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

		rowItem := finance_proto.SellerFinanceList{
			FID:           item.FId,
			UID:           item.SellerId,
			StartDate:     startAt,
			EndDate:       endAt,
			PaymentStatus: paymentStatus,
			Total:         &total,
		}

		items = append(items, &rowItem)
	}

	coll := finance_proto.SellerFinanceListCollection{
		Items: items,
		Total: uint64(dbResult.TotalCount),
	}

	data, err := proto.Marshal(&coll)

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, err.Error(), err).
			BuildAndSend()
	}

	response := finance_proto.ResponseMessage{
		Entity: "SellerFinanceListCollection",
		Meta: &finance_proto.ResponseMetadata{
			Total:   uint32(len(dbResult.SellerFinances)),
			Page:    req.Header.Page,
			PerPage: req.Header.PerPage,
		},
		Data: &any.Any{
			Value: data,
		},
	}

	return future.FactorySync().SetData(&response).BuildAndSend()
}

func resolveFinanceStat(item *entities.SellerFinance) (paymentStatus string, total finance_proto.Money) {
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
		total = finance_proto.Money{
			Amount:   item.Payment.TransferRequest.TotalPrice.Amount,
			Currency: item.Payment.TransferRequest.TotalPrice.Currency,
		}
	}

	return
}
