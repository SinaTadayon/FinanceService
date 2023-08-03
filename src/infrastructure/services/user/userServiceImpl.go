package user_service

import (
	"context"
	"github.com/pkg/errors"
	"gitlab.faza.io/go-framework/acl"
	protoUserServiceV1 "gitlab.faza.io/protos/user"
	"gitlab.faza.io/services/finance/domain/model/entities"
	"gitlab.faza.io/services/finance/infrastructure/future"
	"gitlab.faza.io/services/finance/infrastructure/logger"
	userclient "gitlab.faza.io/services/user-app-client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"sync"
	"time"
)

const (
	// ISO8601 standard time format
	ISO8601 = "2006-01-02T15:04:05-0700"
)

type iUserServiceImpl struct {
	client        *userclient.Client
	serverAddress string
	serverPort    int
	timeout       int
	mux           sync.Mutex
}

func NewUserService(serverAddress string, serverPort int, timeout int) IUserService {
	return &iUserServiceImpl{serverAddress: serverAddress, serverPort: serverPort, timeout: timeout}
}

// TODO refactor fault-tolerant
func (userService *iUserServiceImpl) getUserService(ctx context.Context) error {

	if userService.client == nil {
		userService.mux.Lock()
		defer userService.mux.Unlock()
		if userService.client == nil {
			var err error
			config := &userclient.Config{
				Host:    userService.serverAddress,
				Port:    userService.serverPort,
				Timeout: 10 * time.Second,
			}
			userService.client, err = userclient.NewClient(ctx, config, grpc.WithInsecure(), grpc.WithBlock())
			if err != nil {
				log.GLog.Logger.FromContext(ctx).Error("userclient.NewClient failed",
					"fn", "getUserService",
					"address", userService.serverAddress,
					"port", userService.serverPort,
					"error", err)
				return err
			}
			ctx, _ = context.WithTimeout(ctx, config.Timeout)
			//defer cancel()
			_, err = userService.client.Connect(ctx)
			if err != nil {
				log.GLog.Logger.FromContext(ctx).Error("userclient.NewClient failed",
					"fn", "getUserService",
					"address", userService.serverAddress,
					"port", userService.serverPort,
					"error", err)
				return err
			}
		}
	}

	return nil
}

func (userService *iUserServiceImpl) UserLogin(ctx context.Context, username, password string) future.IFuture {
	ctx1, _ := context.WithCancel(context.Background())
	if err := userService.getUserService(ctx1); err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "UnknownError", errors.Wrap(err, "Connect to UserService Failed")).
			BuildAndSend()
	}

	var outCtx context.Context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		outCtx = metadata.NewOutgoingContext(ctx, md)
	} else {
		outCtx = metadata.NewOutgoingContext(ctx, metadata.New(nil))
	}

	timeoutTimer := time.NewTimer(time.Duration(userService.timeout) * time.Second)

	userFn := func() <-chan interface{} {
		userChan := make(chan interface{}, 0)
		go func() {
			result, err := userService.client.Login(username, password, outCtx)
			if err != nil {
				userChan <- err
			} else {
				userChan <- result
			}
		}()
		return userChan
	}

	var obj interface{} = nil
	select {
	case obj = <-userFn():
		timeoutTimer.Stop()
		break
	case <-timeoutTimer.C:
		log.GLog.Logger.FromContext(ctx).Error("userService.client.Login timeout",
			"fn", "UserLogin",
			"username", "password", username, password)
		return future.FactorySync().
			SetError(future.InternalError, "UnknownError", errors.New("UserLogin Timeout")).
			BuildAndSend()
	}

	if err, ok := obj.(error); ok {
		if err != nil {
			log.GLog.Logger.FromContext(ctx).Error("userService.client.Login failed",
				"fn", "UserLogin",
				"username", username,
				"password", password,
				"error", err)
			return future.FactorySync().
				SetError(future.InternalError, "UnknownError", errors.Wrap(err, "userService.client.Login Failed")).
				BuildAndSend()
		}
	} else if result, ok := obj.(*protoUserServiceV1.LoginResponse); ok {
		if int(result.Code) != 200 {
			log.GLog.Logger.FromContext(ctx).Error("userService.client.Login failed",
				"fn", "UserLogin",
				"username", username,
				"password", password,
				"error", err)
			return future.FactorySync().
				SetError(future.Forbidden, "User Login Failed", errors.Wrap(err, "User Login Failed")).
				BuildAndSend()
		}

		loginTokens := LoginTokens{
			AccessToken:  result.Data.AccessToken,
			RefreshToken: result.Data.RefreshToken,
		}

		return future.FactorySync().SetData(loginTokens).BuildAndSend()
	}

	log.GLog.Logger.FromContext(ctx).Error("userService.client.Login failed",
		"fn", "UserLogin",
		"username", username,
		"password", password)
	return future.FactorySync().
		SetError(future.InternalError, "UnknownError", errors.New("User Login Failed")).
		BuildAndSend()
}

func (userService iUserServiceImpl) AuthenticateContextToken(ctx context.Context) future.IFuture {
	ctx1, _ := context.WithCancel(context.Background())
	if err := userService.getUserService(ctx1); err != nil {
		return future.Factory().SetCapacity(1).
			SetError(future.InternalError, "UnknownError", errors.Wrap(err, "Connect to UserService Failed")).
			BuildAndSend()
	}

	var outCtx context.Context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		outCtx = metadata.NewOutgoingContext(ctx, md)
	} else {
		outCtx = metadata.NewOutgoingContext(ctx, metadata.New(nil))
	}

	timeoutTimer := time.NewTimer(time.Duration(userService.timeout) * time.Second)

	userFn := func() <-chan interface{} {
		userChan := make(chan interface{}, 0)
		go func() {
			result, err := userService.client.VerifyAndGetUserFromContextToken(outCtx)
			if err != nil {
				userChan <- err
			} else {
				userChan <- result
			}
		}()
		return userChan
	}

	var obj interface{} = nil
	select {
	case obj = <-userFn():
		timeoutTimer.Stop()
		break
	case <-timeoutTimer.C:
		log.GLog.Logger.FromContext(ctx).Error("userService.client.VerifyAndGetUserFromContextToken timeout",
			"fn", "AuthenticateContextToken")
		return future.Factory().SetCapacity(1).
			SetError(future.InternalError, "UnknownError", errors.New("UserLogin Timeout")).
			BuildAndSend()
	}

	if err, ok := obj.(error); ok {
		if err != nil {
			log.GLog.Logger.FromContext(ctx).Error("userService.client.AuthenticateContextToken failed",
				"fn", "AuthenticateContextToken",
				"error", err)
			return future.Factory().SetCapacity(1).
				SetError(future.InternalError, "UnknownError", errors.Wrap(err, "Connect to UserService Failed")).
				BuildAndSend()
		}
	} else if result, ok := obj.(*acl.Acl); ok {
		return future.Factory().SetCapacity(1).SetData(result).BuildAndSend()
	}

	return future.Factory().SetCapacity(1).
		SetError(future.Forbidden, "Authenticate Token Failed", errors.New("AuthenticateContextToken failed")).
		BuildAndSend()
}

func (userService iUserServiceImpl) GetSellerProfile(ctx context.Context, sellerId string) future.IFuture {
	ctx1, _ := context.WithCancel(context.Background())
	if err := userService.getUserService(ctx1); err != nil {
		return future.FactorySync().
			SetError(future.InternalError, "UnknownError", errors.Wrap(err, "Connect to UserService Failed")).
			BuildAndSend()
	}

	var outCtx context.Context
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		outCtx = metadata.NewOutgoingContext(ctx, md)
	} else {
		outCtx = metadata.NewOutgoingContext(ctx, metadata.New(nil))
	}

	timeoutTimer := time.NewTimer(time.Duration(userService.timeout) * time.Second)

	userFn := func() <-chan interface{} {
		userChan := make(chan interface{}, 0)
		go func() {
			result, err := userService.client.InternalUserGetOne("userId", sellerId, "", outCtx)
			if err != nil {
				userChan <- err
			} else {
				userChan <- result
			}
		}()
		return userChan
	}

	var obj interface{} = nil
	select {
	case obj = <-userFn():
		timeoutTimer.Stop()
		break
	case <-timeoutTimer.C:
		log.GLog.Logger.FromContext(ctx).Error("userService.client.InternalUserGetOne timeout",
			"fn", "GetSellerProfile")
	}

	if err, ok := obj.(error); ok {
		if err != nil {
			log.GLog.Logger.FromContext(ctx).Error("userService.client.InternalUserGetOne failed",
				"fn", "GetSellerProfile",
				"pid", sellerId, "error", err)
			return future.FactorySync().
				SetError(future.NotFound, "sellerId Not Found", errors.Wrap(err, "sellerId Not Found")).
				BuildAndSend()
		}
	}

	userProfile := obj.(*protoUserServiceV1.UserGetResponse)

	sellerProfile := &entities.SellerProfile{
		SellerId: userProfile.Data.UserId,
	}

	if userProfile.Data.Seller == nil {
		return future.FactorySync().
			SetError(future.NotFound, "sellerId Not Found", errors.New("User Not a Seller")).
			BuildAndSend()
	}

	if userProfile.Data.Seller.GeneralInfo != nil {
		sellerProfile.GeneralInfo = &entities.GeneralSellerInfo{
			ShopDisplayName:          userProfile.Data.Seller.GeneralInfo.ShopDisplayName,
			Type:                     userProfile.Data.Seller.GeneralInfo.Type,
			Email:                    userProfile.Data.Seller.GeneralInfo.Email,
			LandPhone:                userProfile.Data.Seller.GeneralInfo.LandPhone,
			MobilePhone:              userProfile.Data.Seller.GeneralInfo.MobilePhone,
			Website:                  userProfile.Data.Seller.GeneralInfo.Website,
			PostalAddress:            userProfile.Data.Seller.GeneralInfo.PostalAddress,
			PostalCode:               userProfile.Data.Seller.GeneralInfo.PostalCode,
			IsVATObliged:             userProfile.Data.Seller.GeneralInfo.IsVATObliged,
			VATCertificationImageURL: userProfile.Data.Seller.GeneralInfo.VATCertificationImageURL,
		}

		if userProfile.Data.Seller.GeneralInfo.Province != nil {
			sellerProfile.GeneralInfo.Province = userProfile.Data.Seller.GeneralInfo.Province.Name
		}

		if userProfile.Data.Seller.GeneralInfo.City != nil {
			sellerProfile.GeneralInfo.City = userProfile.Data.Seller.GeneralInfo.City.Name
		}

		if userProfile.Data.Seller.GeneralInfo.Neighborhood != nil {
			sellerProfile.GeneralInfo.Neighborhood = userProfile.Data.Seller.GeneralInfo.Neighborhood.Name
		}
	}

	if userProfile.Data.Seller.FinanceData != nil {
		sellerProfile.FinanceData = &entities.SellerFinanceData{
			Iban:                    userProfile.Data.Seller.FinanceData.Iban,
			AccountHolderFirstName:  userProfile.Data.Seller.FinanceData.AccountHolderFirstName,
			AccountHolderFamilyName: userProfile.Data.Seller.FinanceData.AccountHolderFamilyName,
		}
	} else {
		log.GLog.Logger.FromContext(ctx).Error("user FinanceData is empty",
			"fn", "GetSellerProfile",
			"userId", userProfile.Data.UserId)

		return future.FactorySync().
			SetError(future.NotAccepted, "user profile finance data is empty", errors.New("user profile finance data is empty")).
			BuildAndSend()
	}

	if sellerProfile.FinanceData.Iban == "" {
		log.GLog.Logger.FromContext(ctx).Error("user FinanceData iban field is empty",
			"fn", "GetSellerProfile",
			"userId", userProfile.Data.UserId)

		return future.FactorySync().
			SetError(future.NotAccepted, "user FinanceData iban field is empty", errors.New("user profile iban field is empty")).
			BuildAndSend()
	}

	timestamp, err := time.Parse(ISO8601, userProfile.Data.CreatedAt)
	if err != nil {
		log.GLog.Logger.FromContext(ctx).Error("createdAt time parse failed",
			"fn", "GetSellerProfile",
			"pid", sellerId, "error", err)
		timestamp = time.Now().UTC()
	}

	sellerProfile.CreatedAt = timestamp
	timestamp, err = time.Parse(ISO8601, userProfile.Data.UpdatedAt)
	if err != nil {
		log.GLog.Logger.FromContext(ctx).Error("updatedAt time parse failed",
			"fn", "GetSellerProfile",
			"pid", sellerId, "error", err)
		timestamp = time.Now().UTC()
	}
	sellerProfile.UpdatedAt = timestamp
	return future.FactorySync().SetData(sellerProfile).BuildAndSend()
}
