### ACL Package
This package allows you to interact with JWT token,
parse them, validate its signature and expiration time
and allows you to loads it as an object and search its 
permissions

***
**Signing Key**

Tokens are validated with a signing key, this key is the same the one
use for creating the token, so when using this library, you must have
the signing key that was used for creating that token. `NewAcl()` method
accepts a signingKey as its argument and this argument would be used through
the whole object.

**Usage**

****Acl is like Context, it must live only during a request, it must
not be global or managed via singleton pattern.****

Creating a new instance of Acl object.
```go
// signingKey used for creating the token
acl := NewAcl(jwtSigningKey)
// Getting a token from Context (used in GRPC apps)
tokenStr := acl.GetBearerTokenFromContext(ctx)
if len(tokenStr) == 0 {
	fmt.Println("No token found")
	os.Exit(1)
}
```

**Creating a User Object from token**

After you have token string, simply load it:
```go
err := acl.LoadUserFromToken(tokenStr)
if err != nil {
	os.Exit(1)
}
```

**Accessing User Fields**

Note: User is loaded from the token, there is no call to any user service or storage call,
the loaded user's data is read from the token. To check if the token is actually existing
in a user service, simply call that service.

To access fields of loaded user object, do:
```go
fmt.Println(acl.User().FirstName)
```
**Permissions**

To search for the permission of the user:

```go
if acl.UserPerm().Has("cart.coupon.add") {
	// do something
} else {
	fmt.Println("Forbidden!")
}
```
If you want to search if a user has one of the several permissions, do:
```go
if acl.UserPerm().AnyOf("cart.coupon.add", "cart.coupon.delete") {
	// do something
} else {
	fmt.Println("Forbidden!")
}
```

If you want to search if a user has all of the given permissions:
```go
if acl.UserPerm().All("cart.coupon.add", "cart.coupon.delete") {
	// do something
} else {
	fmt.Println("Forbidden!")
}
```

**Searching**

You can also do a wild-card search:
```go
if len(acl.UserPerm().Search("coupon.*")) > 0 {
	// do something
} else {
	fmt.Println("Forbidden!")
}
```