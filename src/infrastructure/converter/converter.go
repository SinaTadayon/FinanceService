//
// The converter will convert given input data type into output data
package converter

import (
	"context"
	"fmt"
	"reflect"
)

type (
	ConverterT uint64

	IConverter interface {
		Convert(context.Context, interface{}, interface{}) (interface{}, error)
	}

	ConvertError struct {
		message string
	}
)

func NewConverterError(in interface{}, out interface{}) *ConvertError {
	err := ConvertError{
		message: fmt.Sprintf("Error on converting %s to %s", reflect.TypeOf(in), reflect.TypeOf(out)),
	}

	return &err
}

func (c *ConvertError) Error() string {
	return c.message
}
