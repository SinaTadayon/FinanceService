package user_service

import (
	"context"
	"gitlab.faza.io/go-framework/acl"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type IUserService interface {
	UserLogin(ctx context.Context, username, password string) future.IFuture
	AuthenticateContextToken(ctx context.Context) (*acl.Acl, error)
	GetSellerProfile(ctx context.Context, sellerId string) future.IFuture
}

type LoginTokens struct {
	AccessToken  string
	RefreshToken string
}
