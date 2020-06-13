package converter

import (
	"context"
	"github.com/pkg/errors"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
)

type Converter ConverterT

func NewConverter() IConverter {
	return new(Converter)
}

func (c *Converter) Convert(ctx context.Context, in interface{}, out interface{}) (interface{}, error) {

	switch in.(type) {
	}

	log.GLog.Logger.Error("mapping from input type not supported",
		"fn", "Convert",
		"in", in)
	return nil, errors.New("mapping from input type not supported")
}
