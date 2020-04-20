package future

import (
	"time"
)

//422 - Validation Errors, an array of objects, each object containing the field and the value (message) of the error
//400 - Bad Request - Any request not properly formatted for the server to understand and parse it
//403 - Forbidden - This can be used for any authentication errors, a user not being logged in etc.
//404 - Any requested entity which is not being found on the server
//406 - Not Accepted - The example usage for this code, is an attempt on an expired or timed-out action. Such as trying to cancel an order which cannot be cancelled any more
//409 - Conflict - Anything which causes conflicts on the server, the most famous one, a not unique email error, a duplicate entity...

type ErrorCode int32

const (
	BadRequest      ErrorCode = 400
	Forbidden       ErrorCode = 403
	NotFound        ErrorCode = 404
	NotAccepted     ErrorCode = 406
	Conflict        ErrorCode = 409
	ValidationError ErrorCode = 422
	InternalError   ErrorCode = 500
)

type IFuture interface {
	Get() IDataFuture
	GetTimeout(duration time.Duration) IDataFuture
	Count() int
	Capacity() int
}

type IDataFuture interface {
	Data() interface{}
	Error() IErrorFuture
}

type IErrorFuture interface {
	Code() ErrorCode
	Message() string
	Reason() error
}
