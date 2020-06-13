//
// return a seller finance list for request
package imp

import (
	"gitlab.faza.io/services/finance/domain/handler"
	finance_repository "gitlab.faza.io/services/finance/domain/model/repository/sellerFinance"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type sellerFinanceListHandler struct {
	repo finance_repository.ISellerFinanceRepository
}

func NewSellerFinanceListHandler(repo finance_repository.ISellerFinanceRepository) handler.IHandler {
	handler := sellerFinanceListHandler{
		repo: repo,
	}

	return &handler
}

func (s sellerFinanceListHandler) Handle(future.IFuture) future.IFuture {
	panic("Not implemented")
}
