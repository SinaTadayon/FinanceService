package acl

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"gitlab.faza.io/go-framework/mongoadapter"
	"google.golang.org/grpc/metadata"
)

const (
	rolesColl = "roles"
	permsColl = "permissions"

	ErrorPermissionNotExisting   = "some permissions do not exists"
	ErrorRoleAlreadyExists       = "role already exists"
	ErrorRoleNotFound            = "role not found"
	ErrorAclTotalCountExceeded   = "start value exceeding total count"
	ErrorPermissionAlreadyExists = "permission already exists"

	RolesMaxItemsCountPerRequest = 100
)

type Acl struct {
	storage       *mongoadapter.Mongo
	user          *AccessTokenClaims
	permission    *PermSearch
	JWTSigningKey []byte
}

// a struct of mapped claims, both our
// custom claims with their types converted
// as well as those standard claims
type AccessTokenClaims struct {
	Permissions []string `json:"permissions"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	Mobile      string   `json:"mobile"`
	UserID      int64    `json:"userId"`
	Audience    string   `json:"aud,omitempty"`
	ExpiresAt   float64  `json:"exp,omitempty"`
	ID          string   `json:"jti,omitempty"`
	IssuedAt    float64  `json:"iat,omitempty"`
	Issuer      string   `json:"iss,omitempty"`
	NotBefore   float64  `json:"nbf,omitempty"`
	Subject     string   `json:"sub,omitempty"`
}

func NewAcl(jwtSigningKey []byte) *Acl {
	return &Acl{
		user:          nil,
		JWTSigningKey: jwtSigningKey,
	}
}

func (acl *Acl) GetBearerTokenFromContext(ctx context.Context) string {
	mdv, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if val, ok := mdv["authorization"]; ok {
		return val[0]
	}
	return ""
}

// it receives a value in this format: "Bearer {access-token}"
// If you have authorization header, directly pass the header value to this func
func (acl *Acl) GetBearerTokenFromBearer(authorizationVal string) string {
	if len(authorizationVal) > 0 {
		var tokenValue = acl.regexCheckAuthorizationMetadata(authorizationVal)
		tokenValue = strings.Trim(tokenValue, string(0x20))
		return tokenValue
	}
	return ""
}

func (acl *Acl) ParseJWTAccessToken(jwtAccessToken string) (*AccessTokenClaims, *jwt.Token, error) {
	token, err := jwt.Parse(jwtAccessToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(fmt.Sprintf("Unexpected signing method: %v", token.Header["alg"]))
		}
		return acl.JWTSigningKey, nil
	})
	if err == nil {
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			var tc = &AccessTokenClaims{}
			var data = claims["data"].(map[string]interface{})
			tc.FirstName = data["firstName"].(string)
			tc.LastName = data["lastName"].(string)
			tc.Mobile = data["mobile"].(string)
			if perms, ok := data["permissions"].([]interface{}); ok {
				for _, v := range perms {
					tc.Permissions = append(tc.Permissions, v.(string))
				}
			}
			tc.ExpiresAt = claims["exp"].(float64)
			tc.NotBefore = claims["nbf"].(float64)
			tc.IssuedAt = claims["iat"].(float64)
			tc.Issuer = claims["iss"].(string)
			tc.ID = claims["jti"].(string)
			uIdF, _ := data["userId"].(float64)
			tc.UserID = int64(uIdF)

			return tc, token, nil
		}
	}
	return nil, nil, err
}

// extract the value of authorization metadata,
// the format is: "Bearer theTokenValue" (exactly one space, case-sensitive)
func (acl *Acl) regexCheckAuthorizationMetadata(bearer string) string {
	var reg = regexp.MustCompile("^Bearer[\\s].*$")
	if reg.MatchString(bearer) {
		var res = strings.Split(bearer, "Bearer ")
		if len(res) == 2 {
			return res[1]
		}
		return ""
	}
	return ""
}

// this method parses a JWT token and loads the content into the
// acl.user.
// it returns error on failure
func (acl *Acl) LoadUserFromToken(jwt string) error {
	tokenClaims, _, err := acl.ParseJWTAccessToken(jwt)
	if err != nil {
		return err
	} else if tokenClaims == nil {
		return errors.New("tokenClaims is nil, aborted")
	}
	acl.user = tokenClaims
	return nil
}

func (acl *Acl) User() *AccessTokenClaims {
	return acl.user
}

func (acl *Acl) SetUser(accessTokenClaims *AccessTokenClaims) *Acl {
	acl.user = accessTokenClaims
	return acl
}

// allows searching the permission of currently loaded ACL user
func (acl *Acl) UserPerm() *PermSearch {
	if acl.user == nil {
		acl.permission = NewPerm([]string{})
	} else if acl.permission == nil {
		acl.permission = NewPerm(acl.user.Permissions)
	}
	return acl.permission
}

type PermSearch struct {
	perms []string
}

func NewPerm(perms []string) *PermSearch {
	return &PermSearch{
		perms: perms,
	}
}

func (p *PermSearch) Has(permName string) bool {
	for _, v := range p.perms {
		if v == permName {
			return true
		}
	}
	return false
}

// checks to see if a permission exists or not
// it returns the string if it exists, else,
// it returns an empty string
func (p *PermSearch) Get(permName string) string {
	for _, v := range p.perms {
		if v == permName {
			return v
		}
	}
	return ""
}

// returns true if any of the given perms exists, and returns false
// if all of them are absent
func (p *PermSearch) AnyOf(permName ...string) bool {
	for _, v := range permName {
		for _, vv := range p.perms {
			if v == vv {
				return true
			}
		}
	}
	return false
}

// checks to see if all given permissions exists, returns false if any of them do not exists
func (p *PermSearch) All(permName ...string) bool {
	for _, v := range permName {
		var exists = false
		for _, vv := range p.perms {
			if v == vv {
				exists = true
			}
		}
		if exists == false {
			return false
		}
	}
	return true
}

// allows wild-card searching the permissions
// it only supports *
// and returns matched permissions
func (p *PermSearch) Search(pattern string) []string {
	var rule *regexp.Regexp
	var result []string
	if strings.Index(pattern, "*") != -1 {
		rule = regexp.MustCompile(strings.Replace(pattern, "*", "[a-z\\.]+", -1))
		for _, v := range p.perms {
			if rule.MatchString(v) {
				result = append(result, v)
			}
		}
		return result
	} else {
		// in this case, the user has not provided a wild-card
		// search, which means it has passed an absolute word
		res := p.Get(pattern)
		if res != "" {
			return []string{res}
		}
		return []string{}
	}
}
