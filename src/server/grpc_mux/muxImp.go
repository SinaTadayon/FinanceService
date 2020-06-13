//
// grpc mux will multiplex request type into corresponding handler method
package grpc_mux

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"gitlab.faza.io/services/finance/infrastructure/handler"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	mux_error "gitlab.faza.io/services/finance/server/grpc_mux/error"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerMux struct {
	config       configs.GRPCMultiplexerConfig
	utpMethodMap map[UserTypeMethod]handler.IHandler
}

func NewServerMux(config configs.GRPCMultiplexerConfig) IServerMux {
	var mapSize int64

	if config.MapSize != 0 {
		mapSize = config.MapSize
	} else {
		mapSize = DefaultMuxMapSize
	}

	mux := ServerMux{
		config:       config,
		utpMethodMap: make(map[UserTypeMethod]handler.IHandler, mapSize),
	}

	return &mux
}

func (s *ServerMux) RegHandler(utp UserType, method Method, handler handler.IHandler) error {
	userTypeMethod := UserTypeMethod(fmt.Sprintf("%s::%s", utp, method))

	if _, contains := s.utpMethodMap[userTypeMethod]; contains {
		errMessage := fmt.Sprintf(mux_error.MethodOverride, method)
		return errors.Errorf(errMessage)
	}

	s.utpMethodMap[userTypeMethod] = handler
	return nil
}

func (s *ServerMux) Handle(ctx context.Context, utp string, method string, req *finance_proto.RequestMessage) (*finance_proto.ResponseMessage, error) {
	userTypeMethod := UserTypeMethod(fmt.Sprintf("%s::%s", utp, method))

	if hand, contains := s.utpMethodMap[userTypeMethod]; !contains {
		errMessage := fmt.Sprintf("Undefined user type %s or method %s", utp, method)
		return nil, status.Error(codes.Code(future.BadRequest), errMessage)
	} else {
		iFuture := future.FactorySync().
			SetData(req).
			BuildAndSend()

		result := hand.Handle(iFuture).Get()

		if result.Error() != nil {
			log.GLog.Logger.Error(result.Error().Message(),
				"fn", "Handle",
				"utp", utp,
				"method", method,
				"req", req.String(),
				"code", result.Error().Code())
			return nil, status.Error(codes.Code(result.Error().Code()), result.Error().Message())
		}

		response := result.Data().(*finance_proto.ResponseMessage)
		return response, nil
	}
}
