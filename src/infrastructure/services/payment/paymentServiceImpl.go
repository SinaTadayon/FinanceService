package payment_service

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	paymentProto "gitlab.faza.io/protos/payment-transfer-proto"
	"gitlab.faza.io/services/finance/infrastructure/future"
	log "gitlab.faza.io/services/finance/infrastructure/logger"
	"google.golang.org/grpc"
	"strconv"
	"sync"
	"time"
)

type iPaymentServiceImpl struct {
	paymentServiceClient paymentProto.PaymentTransferClient
	grpcConnection       *grpc.ClientConn
	serverAddress        string
	serverPort           int
	timeout              int
	mux                  sync.Mutex
}

func NewPaymentService(address string, port int, timeout int) IPaymentService {
	return &iPaymentServiceImpl{nil, nil, address, port, timeout, sync.Mutex{}}
}

func (payment *iPaymentServiceImpl) ConnectToPaymentService() error {
	if payment.grpcConnection == nil {
		payment.mux.Lock()
		defer payment.mux.Unlock()
		if payment.grpcConnection == nil {
			var err error
			ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
			payment.grpcConnection, err = grpc.DialContext(ctx, payment.serverAddress+":"+fmt.Sprint(payment.serverPort),
				grpc.WithBlock(), grpc.WithInsecure())
			if err != nil {
				log.GLog.Logger.Error("GRPC connect dial to payment service failed",
					"fn", "ConnectToPaymentService",
					"address", payment.serverAddress,
					"port", payment.serverPort,
					"error", err)
				return err
			}
			payment.paymentServiceClient = paymentProto.NewPaymentTransferClient(payment.grpcConnection)
		}
	}
	return nil
}

func (payment *iPaymentServiceImpl) CloseConnection() {
	if err := payment.grpcConnection.Close(); err != nil {
		log.GLog.Logger.Error("payment CloseConnection failed",
			"error", err)
	}
}

func (payment *iPaymentServiceImpl) SingleTransferMoney(ctx context.Context, request PaymentRequest) future.IFuture {
	if err := payment.ConnectToPaymentService(); err != nil {
		return future.Factory().SetCapacity(1).
			SetError(future.InternalError, "Unknown Error", errors.Wrap(err, "ConnectToPaymentService failed")).
			BuildAndSend()
	}

	timeoutTimer := time.NewTimer(time.Duration(payment.timeout) * time.Second)

	paymentFn := func() <-chan interface{} {
		paymentChan := make(chan interface{}, 0)
		go func() {
			transferRequest := &paymentProto.TransferRequest{
				CorelationId:       request.FId,
				Amount:             0,
				Currency:           request.Currency,
				ReceiverName:       request.ReceiverName,
				ReceiverAccountId:  request.ReceiverAccountId,
				PaymentDescription: request.PaymentDescription,
				Type:               0,
			}

			amount, err := strconv.Atoi(request.Amount)
			if err != nil {
				log.GLog.Logger.Error("Amount value invalid",
					"request", request,
					"error", err)
				paymentChan <- err
				return
			}

			transferRequest.Amount = int64(amount)

			if request.PaymentType == SellerPayment {
				transferRequest.Type = paymentProto.TransferRequest_PayToSeller
			} else {
				transferRequest.Type = paymentProto.TransferRequest_RefundByer
			}

			result, err := payment.paymentServiceClient.TransferOne(ctx, &paymentProto.TransferOneRequest{
				Req: transferRequest,
			})
			if err != nil {
				paymentChan <- err
			} else {
				paymentChan <- result
			}
		}()
		return paymentChan
	}

	var obj interface{} = nil
	select {
	case obj = <-paymentFn():
		timeoutTimer.Stop()
		break
	case <-timeoutTimer.C:
		log.GLog.Logger.FromContext(ctx).Error("request to TransferOne of payment service, grpc timeout",
			"fn", "SingleTransferMoney",
			"request", request)
		return future.FactorySync().
			SetError(future.NotAccepted, "Get Finance OrderItem Failed", errors.New("Order Service Timeout")).
			BuildAndSend()
	}

	if e, ok := obj.(error); ok {
		if e != nil {
			log.GLog.Logger.Error("TransferOne payment service failed",
				"fn", "SingleTransferMoney",
				"request", request,
				"error", e)
			return future.FactorySync().
				SetError(future.NotAccepted, "Payment TransferOne Failed", errors.New("Payment Service Timeout")).
				BuildAndSend()
		}
	} else if response, ok := obj.(*paymentProto.TransferOneResponse); ok {
		log.GLog.Logger.Debug("TransferOne payment service success",
			"fn", "SingleTransferMoney",
			"request", request)

		return future.FactorySync().
			SetData(PaymentResponse{
				TransferId: response.Res.Id,
				FId:        response.Res.CorelationId}).
			BuildAndSend()
	}

	return future.FactorySync().
		SetError(future.NotAccepted, "Payment TransferOne Failed", errors.New("Payment TransferOne Failed")).
		BuildAndSend()
}

func (payment *iPaymentServiceImpl) BatchTransferMoney(ctx context.Context, requests []PaymentRequest) future.IFuture {
	panic("must be implement")
}

func (payment *iPaymentServiceImpl) GetSingleTransferMoneyResult(ctx context.Context, fid, transferId string) future.IFuture {
	if err := payment.ConnectToPaymentService(); err != nil {
		return future.Factory().SetCapacity(1).
			SetError(future.InternalError, "Unknown Error", errors.Wrap(err, "ConnectToPaymentService failed")).
			BuildAndSend()
	}

	timeoutTimer := time.NewTimer(time.Duration(payment.timeout) * time.Second)

	paymentFn := func() <-chan interface{} {
		paymentChan := make(chan interface{}, 0)
		go func() {
			filter := &paymentProto.Filter{
				Key:   "corelationId",
				Value: fid,
				Type:  paymentProto.Filter_IN,
			}

			filters := make([]*paymentProto.Filter, 0, 1)
			filters = append(filters, filter)

			msgReq := &paymentProto.TransfersListFullDetailRequest{
				Params: &paymentProto.ListingParams{
					Filters: filters,
					Sorting: nil,
					Page:    0,
					PerPage: 0,
				},
			}

			result, err := payment.paymentServiceClient.TransfersListFullDetail(ctx, msgReq)
			if err != nil {
				paymentChan <- err
			} else {
				paymentChan <- result
			}
		}()
		return paymentChan
	}

	var obj interface{} = nil
	select {
	case obj = <-paymentFn():
		timeoutTimer.Stop()
		break
	case <-timeoutTimer.C:
		log.GLog.Logger.FromContext(ctx).Error("request to TransfersListFullDetail of payment service grpc timeout",
			"fn", "GetSingleTransferMoneyResult",
			"fid", fid,
			"transferId", transferId)
		return future.FactorySync().
			SetError(future.NotAccepted, "Get TransferResultMoney Failed", errors.New("Payment Service Timeout")).
			BuildAndSend()
	}

	if e, ok := obj.(error); ok {
		if e != nil {
			log.GLog.Logger.Error("TransfersListFullDetail payment service failed",
				"fn", "GetSingleTransferMoneyResult",
				"fid", fid,
				"transferId", transferId,
				"error", e)
			return future.FactorySync().
				SetError(future.NotAccepted, "Get TransferResult Money Failed", errors.New("Payment Service Timeout")).
				BuildAndSend()
		}
	} else if response, ok := obj.(*paymentProto.TransfersListFullDetailResponse); ok {
		log.GLog.Logger.Debug("Payment TransfersListFullDetail success",
			"fn", "GetSingleTransferMoneyResult",
			"fid", fid,
			"transferId", transferId)

		var transferResult *paymentProto.TransferFullDetail = nil
		for _, transfer := range response.Transfers {
			if fid == transfer.CorelationId && transferId == transfer.Id {
				transferResult = transfer
			}
		}

		if transferResult == nil {
			log.GLog.Logger.Debug("Return transferId and correlationId from payment service invalid",
				"fn", "GetSingleTransferMoneyResult",
				"fid", fid,
				"transferId", transferId,
				"response", response)

			return future.FactorySync().
				SetError(future.NotAccepted, "Get TransferResultMoney Failed", errors.New("Get TransferResultMoney Failed")).
				BuildAndSend()
		}

		return future.FactorySync().
			SetData(TransferMoneyResult{
				FId:        fid,
				TransferId: transferId,
				Total:      transferResult.TotalState.TotalAmount,
				Pending:    transferResult.TotalState.Pending,
				Failed:     transferResult.TotalState.Failed,
				Currency:   "IRR",
			}).BuildAndSend()
	}

	return future.FactorySync().
		SetError(future.NotAccepted, "Get FinanceOrderItemDetail Failed", errors.New("Get FinanceOrderItemDetail Failed")).
		BuildAndSend()
}

func (payment *iPaymentServiceImpl) GetBatchTransferMoneyResult(ctx context.Context, fid []string) future.IFuture {
	panic("must be implement")
}
