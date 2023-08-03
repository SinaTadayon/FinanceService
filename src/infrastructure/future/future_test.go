package future

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSendFutureChannel(t *testing.T) {
	futureTest := Factory().SetCapacity(1).SetData("Salaaam").BuildAndSend()
	futureData := futureTest.Get()
	require.Equal(t, "Salaaam", futureData.Data())
	require.Nil(t, futureData.Error())
}

func TestSendTimeoutFutureChannel(t *testing.T) {
	futureTest := Factory().SetData("Salaaam").Build()
	var err error
	go func() {
		err = FactoryOf(futureTest).SetData("Salaaam").SendTimeout(100 * time.Second)
	}()
	futureData := futureTest.Get()
	require.Nil(t, err)
	require.Equal(t, "Salaaam", futureData.Data())
	require.Nil(t, futureData.Error())
}

func TestSendTimeoutWithErrorFutureChannel(t *testing.T) {
	futureTest := Factory().SetData("Salaaam").Build()
	var err error
	go func() {
		err = FactoryOf(futureTest).SetError(ErrorCode(400), "This is a test", nil).SendTimeout(100 * time.Second)
	}()
	futureData := futureTest.Get()
	require.Nil(t, err)
	require.Equal(t, ErrorCode(400), futureData.Error().Code())
	require.Nil(t, futureData.Data())
}
