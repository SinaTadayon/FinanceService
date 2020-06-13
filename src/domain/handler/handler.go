//
// This file creates an abstraction over request handler types
package handler

import "gitlab.faza.io/services/finance/infrastructure/future"

type IHandler interface {
	Handle(future.IFuture) future.IFuture
}
