package utils

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestORContext(t *testing.T) {
	sig := func(after time.Duration) context.Context {
		ctx, _ := context.WithTimeout(context.Background(), after)
		return ctx
	}
	start := time.Now()
	<-ORContext(
		sig(2*time.Hour),
		sig(5*time.Minute),
		sig(1*time.Second),
		sig(1*time.Hour),
		sig(1*time.Minute),
	).Done()

	assert.Condition(t, func() (success bool) {
		return time.Since(start).Seconds() >= time.Duration(1*time.Second).Seconds() &&
			time.Since(start).Seconds() < 2
	})
}
