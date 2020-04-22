package user_service

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"gitlab.faza.io/go-framework/acl"
	"gitlab.faza.io/go-framework/logger"
	"gitlab.faza.io/services/finance/configs"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	"google.golang.org/grpc/metadata"
	"os"
	"testing"
	"time"
)

var config *configs.Config
var userService *iUserServiceImpl

func TestMain(m *testing.M) {
	var err error
	var path string
	if os.Getenv("APP_MODE") == "dev" {
		path = "../../../testdata/.env"
	} else {
		path = ""
	}

	log.GLog.ZapLogger = log.InitZap()
	log.GLog.Logger = logger.NewZapLogger(log.GLog.ZapLogger)

	config, err = configs.LoadConfigs(path)
	if err != nil {
		log.GLog.Logger.Error("configs.LoadConfig failed",
			"error", err)
		os.Exit(1)
	}

	userService = &iUserServiceImpl{
		client:        nil,
		serverAddress: config.UserService.Address,
		serverPort:    config.UserService.Port,
		timeout:       config.UserService.Timeout,
	}

	// Running Tests
	code := m.Run()
	os.Exit(code)

}

func TestGetSellerInfo(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	err := userService.getUserService(ctx)
	require.Nil(t, err)

	ctx, _ = context.WithCancel(context.Background())

	// user service create dummy user with id 1000001
	iFuture := userService.GetSellerProfile(ctx, "1000001")
	futureData := iFuture.Get()
	require.Nil(t, futureData.Error())
	require.Equal(t, futureData.Data().(*entities.SellerProfile).SellerId, int64(1000001))
}

func TestAuthenticationToken(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	err := userService.getUserService(ctx)
	require.Nil(t, err)

	result, err := userService.client.Login("989100000002", "123456", ctx)
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, 200, int(result.Code))

	var authorization = map[string]string{"authorization": fmt.Sprintf("Bearer %v", result.Data.AccessToken)}
	md := metadata.New(authorization)
	ctxToken := metadata.NewIncomingContext(context.Background(), md)

	iFuture := userService.AuthenticateContextToken(ctxToken).Get()

	require.Nil(t, iFuture.Error())
	require.Equal(t, iFuture.Data().(*acl.Acl).User().UserID, int64(1000001))
}
