package grpc_mux

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	finance_proto "gitlab.faza.io/protos/finance-proto"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"gitlab.faza.io/services/finance/infrastructure/handler"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"testing"
)

var (
	mux IServerMux
)

func TestMain(m *testing.M) {
	config := configs.GRPCMultiplexerConfig{
		MapSize:           0,
		RateLimitTimeUnit: "",
		RateLimit:         0,
	}
	mux = NewServerMux(config)

	code := m.Run()
	os.Exit(code)
}

func TestServerMux(t *testing.T) {
	hand := NewMockHandler()
	err := mux.RegHandler(TestUserType, TestMethod, hand)
	require.Nil(t, err)

	err = mux.RegHandler(TestUserType, TestMethod, hand)
	require.NotNil(t, err)

	req := finance_proto.RequestMessage{
		Name:   "test",
		Type:   "test",
		Time:   nil,
		Header: nil,
		Body:   nil,
	}

	ctx := context.Background()
	result, err := mux.Handle(ctx, string(TestUserType), string(TestMethod), &req)

	require.Nil(t, err)
	require.Equal(t, result.Entity, "test")
}

func TestError(t *testing.T) {
	hand := NewMockHandler()
	err := mux.RegHandler(TestUserType, TestMethod, hand)
	require.Nil(t, err)

	req := finance_proto.RequestMessage{
		Name:   "test",
		Type:   "test",
		Time:   nil,
		Header: nil,
		Body:   nil,
	}

	ctx := context.Background()
	result, err := mux.Handle(ctx, string(TestUserType), "other", &req)

	stat, _ := status.FromError(err)

	require.Equal(t, codes.Code(future.BadRequest), stat.Code())
	require.NotNil(t, err)
	require.Nil(t, result)
}

type handlerMock uint64

func NewMockHandler() handler.IHandler {
	return new(handlerMock)
}

func (h handlerMock) Handle(f future.IFuture) future.IFuture {
	req := f.Get().Data().(*finance_proto.RequestMessage)
	fmt.Println("request name is", req.Name)

	res := finance_proto.ResponseMessage{
		Entity: "test",
	}

	return future.FactorySync().
		SetData(&res).
		BuildAndSend()
}
