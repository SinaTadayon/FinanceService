### Go client for User Service
Import this package in your project and you can call all
available GRPC methods of user-service.

You can find a sample usage in client_test.go file. 

Create an instance of the client instance like this:
```go
uc := NewClient(&Config{ Host: "localhost", Port: "8080"})
uc.SetDialOptions(grpc.WithInsecure())
```

**Register**
```go
user := &pb1.RegisterRequest{}
user.FirstName = "Client Sample FN" // required
user.LastName = "Client Sample LN" // required
user.Mobile = "09370001110" // required
user.Email = ""
user.UserType = "customer"
user.Password = "123456" // required
user.NationalCode = "1234567891"
user.CardNumber = "1234123412341234"
user.Iban = "IR123456789123456789123456"
user.Gender = "male"
user.BirthDate = "1990-01-06" // yyyy-mm-dd
r, err := uc.RegisterUser(user,"", context.Background())
```

**Login**
```go
r, err := uc.Login("username", "password", context.Background())
```

**Verify**

If you want to call an endpoint which needs an access token, such as
TokenVerify(), then here is a sample:
```go
r, err := uc.TokenVerify("myJWTAccessToken", context.Background())
```


**Logout**
```go
r, err := uc.Logout("myJWTAccessToken", context.Background())
```

There are several other methods which are documented, 
you can check their own documentation.


### Token Verify and Shorthand Method
With the following method you can pass an incoming
context to the function plus a jwtSigningKey, and then
the function handles verification with user-service, unpacking
the token and returning a fully loaded user object inside ACL(which has
several methods for working with fields and permissions of the user)
For interacting with tokens, you have this method:
```go
acl, err := uc.VerifyAndGetUserFromContextToken(ctx, jwtSigningKey)
// if no error
acl.User().FirstName
acl.User().LastName

// checks to see if aclUser has a permission
acl.UserPerm().Has("catalog.attribute.add")

// searches for a pattern among user permissions
acl.UserPerm().Search("catalog.*")

// checks to see if user has any of the given permnissions
acl.UserPerm().Any("catalog.attribute.add", "catalog.attribute.edit")

// ensures user has all the given permissions
acl.UserPerm().All("catalog.attribute.add", "catalog.attribute.edit")
```