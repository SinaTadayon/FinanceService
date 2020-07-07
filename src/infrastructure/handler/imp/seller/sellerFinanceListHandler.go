//
// return a seller finance list for request
package seller

import (
	"context"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/infrastructure/converter"
	"gitlab.faza.io/services/finance/infrastructure/future"
	handler2 "gitlab.faza.io/services/finance/infrastructure/handler"
	"go.mongodb.org/mongo-driver/bson"
)

type sellerFinanceListHandler struct {
	converter converter.IConverter
	repo      finance_repository.ISellerFinanceRepository
}

func NewSellerFinanceListHandler(repo finance_repository.ISellerFinanceRepository, converter converter.IConverter) handler2.IHandler {
	handler := sellerFinanceListHandler{
		repo:      repo,
		converter: converter,
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

	if req.Header.UID == 0 {
		return future.FactorySync().
			SetError(future.BadRequest, "Missing parameter UID", errors.New("Missing parameter UID")).
			BuildAndSend()
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
	dtoResult, err := s.converter.Convert(context.Background(), dbResult, finance_proto.SellerFinanceListCollection{})

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, err.Error(), err).
			BuildAndSend()
	}

	coll := dtoResult.(finance_proto.SellerFinanceListCollection)
	data, err := proto.Marshal(&coll)

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, err.Error(), err).
			BuildAndSend()
	}

	response := finance_proto.ResponseMessage{
		Entity: "SellerFinanceListCollection",
		Meta: &finance_proto.ResponseMetadata{
			Total:   uint32(len(coll.Items)),
			Page:    req.Header.Page,
			PerPage: req.Header.PerPage,
		},
		Data: &any.Any{
			Value: data,
		},
	}

	return future.FactorySync().SetData(&response).BuildAndSend()
}
