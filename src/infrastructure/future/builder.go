package future

import (
	"github.com/pkg/errors"
	"time"
)

type Builder struct {
	iFuture    *iFutureImpl
	dataFuture *iDataFutureImpl
}

func Factory() Builder {
	return Builder{
		iFuture: &iFutureImpl{},
	}
}

func FactorySync() Builder {
	return Builder{
		iFuture: &iFutureImpl{
			channel:  nil,
			count:    0,
			capacity: 1,
		},
	}
}

func FactoryOf(future IFuture) Builder {
	return Builder{
		iFuture: future.(*iFutureImpl),
	}
}

func FactoryDataOf(iDataFuture IDataFuture) Builder {
	return Builder{
		iFuture:    &iFutureImpl{},
		dataFuture: iDataFuture.(*iDataFutureImpl),
	}
}

func FactorySyncDataOf(iDataFuture IDataFuture) Builder {
	return Builder{
		iFuture: &iFutureImpl{
			channel:  nil,
			count:    0,
			capacity: 1,
		},
		dataFuture: iDataFuture.(*iDataFutureImpl),
	}
}

func FactorySyncErrorOf(iErrorFuture IErrorFuture) Builder {
	builder := Builder{
		iFuture: &iFutureImpl{
			channel:  nil,
			count:    0,
			capacity: 1,
		},
		dataFuture: &iDataFutureImpl{},
	}
	errorFuture := iErrorFuture.(*iErrorFutureImpl)
	builder.dataFuture.futureError = errorFuture
	return builder
}

func (builder Builder) SetCapacity(capacity int) Builder {
	builder.iFuture.capacity = capacity
	return builder
}

func (builder Builder) SetCount(count int) Builder {
	builder.iFuture.count = count
	return builder
}

func (builder Builder) SetData(data interface{}) Builder {

	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}
	builder.dataFuture.data = data
	return builder
}

func (builder Builder) SetError(code ErrorCode, message string, reason error) Builder {
	errorFuture := &iErrorFutureImpl{}
	errorFuture.ErrCode = code
	errorFuture.ErrMsg = message
	errorFuture.ErrReason = reason

	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}

	builder.dataFuture.futureError = errorFuture
	return builder
}

func (builder Builder) SetErrorOf(errFuture IErrorFuture) Builder {
	errorFuture := &iErrorFutureImpl{}
	errorFuture.ErrCode = errFuture.Code()
	errorFuture.ErrMsg = errFuture.Message()
	errorFuture.ErrReason = errFuture.Reason()

	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}

	builder.dataFuture.futureError = errorFuture
	return builder
}

func (builder Builder) Send() {
	if builder.iFuture.channel == nil {
		builder.iFuture.channel = make(chan IDataFuture, builder.iFuture.capacity)
	}

	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}

	defer close(builder.iFuture.channel)
	builder.iFuture.channel <- builder.dataFuture
}

func (builder Builder) SendTimeout(duration time.Duration) error {
	if builder.iFuture.channel == nil {
		builder.iFuture.channel = make(chan IDataFuture, builder.iFuture.capacity)
	}
	defer close(builder.iFuture.channel)
	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}
	select {
	case builder.iFuture.channel <- builder.dataFuture:
		return nil
	case <-time.After(duration):
		return errors.New("Send Timeout")
	}
}

func (builder Builder) BuildAndSend() IFuture {
	if builder.iFuture.channel == nil {
		builder.iFuture.channel = make(chan IDataFuture, builder.iFuture.capacity)
	}
	defer close(builder.iFuture.channel)
	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}
	builder.iFuture.channel <- builder.dataFuture
	return builder.iFuture
}

func (builder Builder) BuildAndSendTimeout(duration time.Duration) (IFuture, error) {
	if builder.iFuture.channel == nil {
		builder.iFuture.channel = make(chan IDataFuture, builder.iFuture.capacity)
	}
	defer close(builder.iFuture.channel)
	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}
	select {
	case builder.iFuture.channel <- builder.dataFuture:
		return builder.iFuture, nil
	case <-time.After(duration):
		return nil, errors.New("Send Timeout")
	}
}

func (builder Builder) Build() IFuture {
	if builder.iFuture.channel == nil {
		builder.iFuture.channel = make(chan IDataFuture, builder.iFuture.capacity)
	}
	if builder.dataFuture == nil {
		builder.dataFuture = &iDataFutureImpl{}
	}
	return builder.iFuture
}
