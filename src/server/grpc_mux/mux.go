package grpc_mux

import (
	"context"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/infrastructure/handler"
)

const (
	DefaultMuxMapSize = 20
)

const (
	// defined users
	TestUserType   UserType = "test"
	SellerUserType UserType = "seller"

	// defined methods
	TestMethod                Method = "test"
	SellerFinanceListMethod   Method = "finance_list"
	SellerOrderItemListMethod Method = "finance_order_item_list"
)

type (
	UserType       string
	Method         string
	UserTypeMethod string // This type compose of utp::method
	IServerMux     interface {
		//
		// This is used for registering handler type for specified user type and
		// method
		RegHandler(utp UserType, method Method, handler handler.IHandler) error

		//
		// This method will route incoming request into specified handler
		Handle(ctx context.Context, utp string, method string, req *finance_proto.RequestMessage) (*finance_proto.ResponseMessage, error)
	}
)
