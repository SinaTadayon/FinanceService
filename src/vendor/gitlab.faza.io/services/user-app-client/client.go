package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitlab.faza.io/go-framework/acl"
	pb1 "gitlab.faza.io/protos/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	config *Config
	client pb1.UserClient
}

type Config struct {
	Host    string
	Port    int
	Timeout time.Duration // zero means no timeout
}

// this is a copy of signing key used in user-service for signing the
// token when generating or verifying them. We must use the same signature
// to see if a token is valid or not
var JwtSigningKey = []byte("ya2Uu:fT^2f*<y1]tU7Q1f';WbxoQT_J*A,.-Oo;^hZ2Uk}ZQ}UQQ2u+sy`2@>N")

type UserFields pb1.RegisterRequest

// this is an exact copy from user-service
// these two types (this and the one in user-service)
// must be exactly the same
type AccessTokenClaims struct {
	Permissions []string `json:"permissions"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	Mobile      string   `json:"mobile"`
	Email       string   `json:"email"`
	Audience    string   `json:"aud,omitempty"`
	ExpiresAt   float64  `json:"exp,omitempty"`
	ID          string   `json:"jti,omitempty"`
	IssuedAt    float64  `json:"iat,omitempty"`
	Issuer      string   `json:"iss,omitempty"`
	NotBefore   float64  `json:"nbf,omitempty"`
	Subject     string   `json:"sub,omitempty"`
}

// this method must be called once (ideally), because each call to an endpoint
// uses the same shared client to server connection. dial() is called in
// this method, so other methods only need reusing it.
func NewClient(ctx context.Context, config *Config, dialOpts ...grpc.DialOption) (*Client, error) {
	c := &Client{
		config: config,
	}
	conn, err := c.Connect(ctx, dialOpts...)
	if err != nil {
		return nil, errors.New("failed to create client connection, got error: " + err.Error())
	}
	c.client = conn
	return c, nil
}

func (c *Client) dial(dialOptions ...grpc.DialOption) (pb1.UserClient, error) {
	var address = "%v:%v"
	conn, err := grpc.Dial(fmt.Sprintf(address, c.config.Host, c.config.Port), dialOptions...)
	if err != nil {
		return nil, err
	}
	client := pb1.NewUserClient(conn)
	return client, nil
}

func (c *Client) dialWithContext(ctx context.Context, dialOptions ...grpc.DialOption) (pb1.UserClient, error) {
	var address = "%v:%v"
	conn, err := grpc.DialContext(ctx, fmt.Sprintf(address, c.config.Host, c.config.Port), dialOptions...)
	if err != nil {
		return nil, err
	}
	client := pb1.NewUserClient(conn)
	return client, nil
}

// this method calls dial(), it is once called in the constructor, but individual calls
// can also use it ensure that they get a connection even if the default constructor's attempt
// had failed
func (c *Client) Connect(ctx context.Context, dialOptions ...grpc.DialOption) (pb1.UserClient, error) {
	var err error
	if c.client == nil {
		if ctx == nil {
			c.client, err = c.dial(dialOptions...)
		} else {
			c.client, err = c.dialWithContext(ctx, dialOptions...)
		}

		if err != nil {
			return nil, err
		}
	}
	return c.client, nil
}

// AddUserType .
func (c *Client) AddUserType(ctx context.Context, req *pb1.ChangeUserTypeRequest) (res *pb1.ChangeUserTypeResponse, err error) {
	res, err = c.client.AddUserType(ctx, req)
	return
}

// RemoveUserType .
func (c *Client) RemoveUserType(ctx context.Context, req *pb1.ChangeUserTypeRequest) (res *pb1.ChangeUserTypeResponse, err error) {
	res, err = c.client.RemoveUserType(ctx, req)
	return
}

// pass a username and password
// username can be an email or a mobile number
// the format of mobile number is not important, if it contains
// a country code, then it handles it, if it doesn't have any
// country code, the default country of user service is applied
// in formatting.
func (c *Client) Login(username, password string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.LoginResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.LoginRequest{}
	req.Username = username
	req.Password = password
	res, err := conn.Login(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) LoginAsanPardakht(token string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.LoginAsanPardakhtResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.LoginAsanPardakhtRequest{
		Token: token,
	}
	res, err := conn.LoginAsanPardakht(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// UserAddAddress allows user to add address
func (c *Client) UserAddAddress(ctx context.Context, req *pb1.UserAddAddressRequest, grpcCallOptions ...grpc.CallOption) (*pb1.UserAddAddressResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.UserAddAddress(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// UserEditAddress allows user to add address
func (c *Client) UserEditAddress(ctx context.Context, req *pb1.UserEditAddressRequest, grpcCallOptions ...grpc.CallOption) (*pb1.UserEditAddressResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.UserEditAddress(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// UserDeleteAddress allows user to add address
func (c *Client) UserDeleteAddress(ctx context.Context, req *pb1.UserDeleteAddressRequest, grpcCallOptions ...grpc.CallOption) (*pb1.EmptyRequest, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.UserDeleteAddress(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// SellerEdit allows user to add address
func (c *Client) SellerEdit(ctx context.Context, req *pb1.SellerEditRequest, grpcCallOptions ...grpc.CallOption) (*pb1.EmptyResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.SellerEdit(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListCounties will return the list of available countries
func (c *Client) ListCounties(ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.ListCountriesResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.EmptyRequest{}
	res, err := conn.ListCountries(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListProvinces will return the list of available provinces in a country
func (c *Client) ListProvinces(ctx context.Context, countryID string, grpcCallOptions ...grpc.CallOption) (*pb1.ListProvincesResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.ListProvincesRequest{
		CountryId: countryID,
	}
	res, err := conn.ListProvinces(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListCities will return the list of available cities in a province
func (c *Client) ListCities(ctx context.Context, provinceID string, grpcCallOptions ...grpc.CallOption) (*pb1.ListCitiesResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.ListCitiesRequest{
		ProvinceId: provinceID,
	}
	res, err := conn.ListCities(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// ListNeighbourhoods will return the list of available cities in a province
func (c *Client) ListNeighbourhoods(ctx context.Context, cityID string, grpcCallOptions ...grpc.CallOption) (*pb1.ListNeighborsResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.ListNeighborsRequest{
		CityId: cityID,
	}
	res, err := conn.ListNeighborhoods(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// a valid refreshToken returns a response like that of a Login.
// Calling this method, in case it returns true, invalidates this refresh token
// and returns a new one in Login object
func (c *Client) TokenRefresh(refreshToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.LoginResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	var req = &pb1.TokenRefreshRequest{}
	req.RefreshToken = refreshToken
	res, err := conn.TokenRefresh(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) PasswordChange(oldPassword, newPassword, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.LoginResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.PasswordChange(ctx, &pb1.PasswordChangeRequest{
		PasswordOld: oldPassword,
		PasswordNew: newPassword,
	}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// req the request object
//
// accessToken [optional]
//
// This method can create a customer, a seller and an operator
// if an access token is passed, then it means the registration is not
// for a customer.
//
// An access token means it either is a customer who wants
// to become a seller or an operator who is going to create a seller/operator
// The UserType field decides which one is going to be created, and access token
// allows us to see who wants to create it, which means the service checks all
// required permissions on the owner of the token.
// if the access token is passed, and if it is valid and the owner is an operator
// or a customer (and the permissions are OK), then the user gets created in this method,
// there is no need to call RegisterUserVerify()
func (c *Client) RegisterUser(req *UserFields, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	if len(jwtAccessToken) > 0 {
		ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	}
	var regData pb1.RegisterRequest
	regData.FirstName = req.FirstName
	regData.LastName = req.LastName
	regData.Password = req.Password
	regData.Mobile = req.Mobile
	regData.NationalCode = req.NationalCode
	regData.BirthDate = req.BirthDate
	regData.Email = req.Email
	regData.Gender = req.Gender
	regData.Iban = req.Iban
	regData.Roles = req.Roles
	regData.Country = req.Country
	regData.UserType = req.UserType
	regData.CardNumber = req.CardNumber
	res, err := conn.Register(ctx, &regData, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// this method verifies an already requested RegisterUser() method.
// the method accepts an identifier which is either a mobile number
// or an email, and a `code`, which is a verification code sent to the
// user who has attempted to register.
// this method should be called if RegisterUser() is called by
// an unregistered user.
func (c *Client) RegisterUserVerify(identifier, code string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.LoginResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.RegisterVerify(ctx, &pb1.RegisterVerifyRequest{
		Identifier: identifier,
		Code:       code,
	}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// CheckVerificationCode .
func (c *Client) CheckVerificationCode(identifier, code string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.CheckVerificationCode(ctx, &pb1.CheckVerificationCodeRequest{
		Identifier: identifier,
		Code:       code,
	}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// this method checks with the server if the passed access token
// is still valid or not
func (c *Client) TokenValidate(jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.TokenVerify(ctx, &pb1.EmptyRequest{}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// calls the server and revokes the access token
func (c *Client) Logout(jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.Logout(ctx, &pb1.EmptyRequest{}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Adds a role to the system with a given set of permissions
// the operation is successful only if the given access token
// is valid and has the adequate permissions (such as user.role.add)
func (c *Client) RoleAdd(roleKey, roleTitle string, permissions []string, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	var roleReq = &pb1.RoleAddRequest{Key: roleKey, Title: roleTitle, Permissions: permissions}
	res, err := conn.RoleAdd(ctx, roleReq, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) RoleEdit(roleKey, roleTitle string, permissions []string, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	var roleReq = &pb1.RoleAddRequest{Key: roleKey, Title: roleTitle, Permissions: permissions}
	res, err := conn.RoleEdit(ctx, roleReq, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) RoleRemove(roleKey, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.RoleRemove(ctx, &pb1.RoleRemoveRequest{Key: roleKey}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) RoleGetOne(roleKey, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.RoleGetResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.RoleGetOne(ctx, &pb1.RoleGetRequest{Key: roleKey}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) RoleGetList(page, perPage int32, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.RoleListResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.RoleGetList(ctx, &pb1.ListRequest{Page: page, PerPage: perPage}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) PermGetList(page, perPage int32, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.PermissionListResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.PermissionGetList(ctx, &pb1.ListRequest{Page: page, PerPage: perPage}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// gets a user, by matching a field (such as mobile) with a value (mobile number)
func (c *Client) UserGetOne(field, value, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.UserGetResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.UserGetOne(ctx, &pb1.UserGetRequest{Field: field, Value: value}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// gets a user, by matching a field (such as mobile) with a value (mobile number)
func (c *Client) InternalUserGetOne(field, value, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.UserGetResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	res, err := conn.InternalUserGetOne(ctx, &pb1.UserGetRequest{Field: field, Value: value}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) ForgotPassword(identifier string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.ForgotPassword(ctx, &pb1.ForgotPasswordRequest{Identifier: identifier}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) ForgotPasswordVerify(identifier, verifyCode, password string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.ForgotPasswordVerify(ctx, &pb1.ForgotPasswordVerifyRequest{Identifier: identifier, Code: verifyCode, Password: password}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) CreateDummyUser(ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.CreateDummyUser(ctx, &pb1.EmptyRequest{}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *Client) DeleteDummyUser(ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	res, err := conn.DeleteDummyUser(ctx, &pb1.EmptyRequest{}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// returns a list of users
//
// it accepts optional filters and sorting params, but they are not required
//
// perPage and page params are required, you have to specify which page and how many
// items per page. You can pass 1 to Page if you have not many records

// filters schema: filters[fieldName]={"value", "operatorName"}
// sorting schema: sort[fieldName]=1 (or -1)
func (c *Client) UserList(perPage, page int32, filters map[string][]string, sorting map[string]int32, jwtAccessToken string,
	ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.UserListResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	var filtersMapped []*pb1.FilterEntry
	if filters != nil && len(filters) > 0 {
		for k, v := range filters {
			if len(v) != 2 {
				continue
			}
			filtersMapped = append(filtersMapped, &pb1.FilterEntry{
				Field: k, Value: v[0], Operator: v[1],
			})
		}
	}

	res, err := conn.UserGetList(ctx, &pb1.ListRequest{Filters: filtersMapped, Sorting: sorting, PerPage: perPage, Page: page}, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// By calling this method, one of the possible scenarios can happen:
// - a customer editing its own profile [valid jwtToken]
// - a seller editing its own profile [valid jwtToken]
// - an operator editing a seller's or a customer's profile [valid jwtToken and adequate permissions (user.seller.edit, user.customer.edit, user.operaotr.edit etc.)]
// This endpoint only allows editing firstName, lastName, gender, birthDate and first-time-if-not-set email
// for editing addresses, financial data,  or in case of seller the seller or business data and password must be changed
// via their own respective methods
func (c *Client) UserEdit(id string, userObject *UserFields, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.LoginResponse, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = c.createContext(ctx)
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	var regData pb1.UserEditRequest
	regData.UserId = id
	regData.FirstName = userObject.FirstName
	regData.LastName = userObject.LastName
	regData.NationalCode = userObject.NationalCode
	regData.BirthDate = userObject.BirthDate
	regData.Email = userObject.Email
	regData.Gender = userObject.Gender
	regData.Roles = userObject.Roles
	regData.Finance = []*pb1.UserFinanceData{&pb1.UserFinanceData{
		CardNumber: userObject.CardNumber,
		Iban:       userObject.Iban,
	}}
	res, err := conn.UserEdit(ctx, &regData, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// this method returns a context object which contains a authorization header
// containing the passed token
func CreateAuthorizationBearerInContext(token string, ctx context.Context) context.Context {
	var authorization = map[string]string{"authorization": fmt.Sprintf("Bearer %v", token)}
	md := metadata.New(authorization)
	if ctx == nil {
		ctx = context.Background()
	}
	return metadata.NewOutgoingContext(ctx, md)
}

// This the main function to be used by all services for validating a JWT token with user-service
// and getting its converted user object
func (c *Client) VerifyAndGetUserFromContextToken(ctx context.Context) (*acl.Acl, error) {
	acl := acl.NewAcl(JwtSigningKey)
	bearer := acl.GetBearerTokenFromContext(ctx)
	rawTokenStr := acl.GetBearerTokenFromBearer(bearer)
	res, err := c.TokenValidate(rawTokenStr, ctx)
	if err != nil {
		return nil, err
	} else if int(res.Code) != 200 {
		return nil, errors.New(res.Message)
	}
	tokenClaims, _, err := acl.ParseJWTAccessToken(rawTokenStr)
	if err != nil {
		return nil, err
	}
	acl.SetUser(tokenClaims)
	return acl, nil
}

func (c *Client) RegisterOperator(email, password string, roles []string, jwtAccessToken string, ctx context.Context, grpcCallOptions ...grpc.CallOption) (*pb1.Result, error) {
	ctxConn := c.createContext(nil)
	conn, err := c.Connect(ctxConn)
	if err != nil {
		return nil, errors.New("failed to connect to GRPC server, got error " + err.Error())
	}
	ctx = CreateAuthorizationBearerInContext(jwtAccessToken, ctx)
	var req = &pb1.RegisterOperatorRequest{}
	req.Email = email
	req.Password = password
	req.Roles = roles
	res, err := conn.RegisterOperator(ctx, req, grpcCallOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// checks to see if given context is nil or not,
// if it is nil, it creates a new context with or
// without timeout, based on the value of Config.Timeout
func (c *Client) createContext(ctx context.Context) context.Context {
	if ctx == nil {
		if c.config.Timeout == 0 {
			ctx = context.Background()
		} else {
			ctx, _ = context.WithTimeout(context.Background(), c.config.Timeout*time.Second)
		}
	}
	return ctx
}

func TokenIsExpiredError(err error) bool {
	return strings.ToLower(err.Error()) == "token is expired"
}
