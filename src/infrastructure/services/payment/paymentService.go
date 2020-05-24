package payment_service

import (
	"context"
	"gitlab.faza.io/services/finance/infrastructure/future"
)

type IPaymentService interface {
	SingleTransferMoney(ctx context.Context, request PaymentRequest) future.IFuture
	BatchTransferMoney(ctx context.Context, requests []PaymentRequest) future.IFuture
	GetSingleTransferMoneyResult(ctx context.Context, fid, transferId string) future.IFuture
	GetBatchTransferMoneyResult(ctx context.Context, fid []string) future.IFuture
}

type PaymentType string

const (
	SellerPayment PaymentType = "SELLER_PAYMENT"
	BuyerPayment  PaymentType = "BUYER_PAYMENT"
)

type PaymentRequest struct {
	FId                string
	SellerId           uint64
	Amount             string
	Currency           string
	ReceiverName       string
	ReceiverAccountId  string
	PaymentDescription string
	PaymentType        PaymentType
}

type PaymentResponse struct {
	TransferId string
	FId        string
}

type TransferMoneyResult struct {
	FId        string
	TransferId string
	Total      int64
	Pending    int64
	Failed     int64
	Currency   string
}
