package seller

import (
	"context"
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerOrderItem"
	"gitlab.faza.io/services/finance/infrastructure/converter"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"gitlab.faza.io/services/finance/infrastructure/handler"
	"go.mongodb.org/mongo-driver/bson"
)

type sellerFinanceOrderItemListHandler struct {
	repo      finance_repository.ISellerOrderItemRepository
	converter converter.IConverter
}

func NewSellerFinanceOrderItemListHandler(repo finance_repository.ISellerOrderItemRepository, converter converter.IConverter) handler.IHandler {
	handler := sellerFinanceOrderItemListHandler{
		repo:      repo,
		converter: converter,
	}

	return &handler
}

func (s sellerFinanceOrderItemListHandler) Handle(input interface{}) future.IFuture {
	var (
		sortName string
		sortDire int
		uid      uint64
		fid      string
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
	uid = req.Header.UID

	if req.Header.FID == "" {
		return future.FactorySync().
			SetError(future.BadRequest, "Missing parameter FID", errors.New("Missing parameter FID")).
			BuildAndSend()
	}
	fid = req.Header.FID

	totalPipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "sellerId": uid}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$unwind": "$ordersInfo.orders.items"},
		{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": 1}}},
		{"$project": bson.M{"_id": 0, "count": 1}},
	}

	pipeline := []bson.M{
		{"$match": bson.M{"fid": fid, "sellerId": uid}},
		{"$unwind": "$ordersInfo"},
		{"$unwind": "$ordersInfo.orders"},
		{"$unwind": "$ordersInfo.orders.items"},
		{"$project": bson.M{"_id": 0}},
	}

	totalFunc := func() interface{} {
		return totalPipeline
	}

	pipeFunc := func() interface{} {
		return pipeline
	}

	ctx := context.Background()
	res := s.repo.FindOrderItemsByFilterWithPage(ctx, totalFunc, pipeFunc, int64(req.Header.Page), int64(req.Header.PerPage), sortName, sortDire).Get()

	if res.Error() != nil {
		return future.FactorySync().
			SetError(res.Error().Code(), res.Error().Message(), res.Error().Reason()).
			BuildAndSend()
	}

	dbResult := res.Data().(finance_repository.SellerOrderItems)
	dtoResult, err := s.converter.Convert(context.Background(), dbResult, finance_proto.SellerFinanceOrderItemCollection{})

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, err.Error(), err).
			BuildAndSend()
	}

	response := dtoResult.(finance_proto.SellerFinanceOrderItemCollection)
	responseByte, err := proto.Marshal(&response)

	if err != nil {
		return future.FactorySync().
			SetError(future.InternalError, err.Error(), err).
			BuildAndSend()
	}

	responseMessage := finance_proto.ResponseMessage{
		Entity: "SellerFinanceOrderItemCollection",
		Meta: &finance_proto.ResponseMetadata{
			Total:   uint32(response.Total),
			Page:    req.Header.Page,
			PerPage: req.Header.PerPage,
		},
		Data: &any.Any{
			Value: responseByte,
		},
	}

	return future.FactorySync().SetData(&responseMessage).BuildAndSend()
}
