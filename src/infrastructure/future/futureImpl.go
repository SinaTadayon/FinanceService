package future

import (
	"fmt"
	"time"
)

type stream chan IDataFuture

type iFutureImpl struct {
	channel  stream
	count    int
	capacity int
}

func newFuture(channel stream, count int, capacity int) IFuture {
	return &iFutureImpl{channel: channel, count: count, capacity: capacity}
}

func (future iFutureImpl) Get() IDataFuture {
	select {
	case futureData, ok := <-future.channel:
		if ok != true {
			return nil
		}
		return futureData
	}
}

func (future iFutureImpl) GetTimeout(duration time.Duration) IDataFuture {
	select {
	case futureData, ok := <-future.channel:
		if ok != true {
			return nil
		}
		return futureData
	case <-time.After(duration):
		return nil
	}
}

func (future iFutureImpl) Channel() stream {
	return future.channel
}

func (future iFutureImpl) Count() int {
	return future.count
}

func (future iFutureImpl) Capacity() int {
	return future.capacity
}

type iDataFutureImpl struct {
	data        interface{}
	futureError IErrorFuture
}

func (futureData iDataFutureImpl) Data() interface{} {
	return futureData.data
}
func (futureData iDataFutureImpl) Error() IErrorFuture {
	return futureData.futureError
}

type iErrorFutureImpl struct {
	ErrCode   ErrorCode
	ErrMsg    string
	ErrReason error
}

func (errorFuture iErrorFutureImpl) Code() ErrorCode {
	return errorFuture.ErrCode
}

func (errorFuture iErrorFutureImpl) Message() string {
	return errorFuture.ErrMsg
}

func (errorFuture iErrorFutureImpl) Reason() error {
	return errorFuture.ErrReason
}

func (errorFuture iErrorFutureImpl) Error() string {
	return fmt.Sprintf("err code: %d, message: %s, reason: %s", errorFuture.ErrCode,
		errorFuture.ErrMsg, errorFuture.ErrReason)
}
