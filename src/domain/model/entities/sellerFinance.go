package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type FinanceState string
type TransferState string

const (
	FinanceDocumentVersion string = "1.0.0"
)

const (
	FinanceOrderCollectionStatus FinanceState = "ORDER_COLLECTION"
	FinancePaymentProcessStatus  FinanceState = "PAYMENT_PROCESS"
	FinanceClosedStatus          FinanceState = "CLOSED"
)

const (
	TransferSuccessState TransferState = "SUCCESS"
	TransferFailedState  TransferState = "FAILED"
	TransferPendingState TransferState = "PENDING"
	TransferPartialState TransferState = "PARTIAL_PAYED"
)

type SellerFinance struct {
	ID         primitive.ObjectID `bson:"-"`
	FId        string             `bson:"fid"`
	SellerId   uint64             `bson:"sellerId"`
	Version    uint64             `bson:"version"`
	DocVersion string             `bson:"docVersion"`
	SellerInfo *SellerProfile     `bson:"sellerInfo"`
	Invoice    Invoice            `bson:"invoice"`
	OrdersInfo []*OrderInfo       `bson:"ordersInfo"`
	Payment    *FinancePayment    `bson:"payment"`
	Status     FinanceState       `bson:"status"`
	StartAt    *time.Time         `bson:"startAt"`
	EndAt      *time.Time         `bson:"endAt"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
	DeletedAt  *time.Time         `bson:"deletedAt"`
}

type FinancePayment struct {
	TransferRequest  *TransferRequest  `bson:"transferRequest"`
	TransferResponse *TransferResponse `bson:"transferResponse"`
	TransferResult   *TransferResult   `bson:"transferResult"`
	Status           TransferState     `bson:"status"`
	CreatedAt        time.Time         `bson:"createdAt"`
	UpdatedAt        time.Time         `bson:"updatedAt"`
}

type TransferRequest struct {
	TotalPrice         Money     `bson:"totalPrice"`
	ReceiverName       string    `bson:"receiverName"`
	ReceiverAccountId  string    `bson:"receiverAccountId"`
	PaymentDescription string    `bson:"paymentDescription"`
	TransferType       string    `bson:"transferType"`
	CreatedAt          time.Time `bson:"createdAt"`
}

type TransferResponse struct {
	TransferId string    `bson:"transferId"`
	CreatedAt  time.Time `bson:"createdAt"`
}

type TransferResult struct {
	TransferId      string    `bson:"transferId"`
	SuccessTransfer *Money    `bson:"successTransfer"`
	FailedTransfer  *Money    `bson:"failedTransfer"`
	CreatedAt       time.Time `bson:"createdAt"`
}

type Invoice struct {
	SSORawTotal            *Money `bson:"ssoRawTotal"`
	SSORoundupTotal        *Money `bson:"ssoRoundupTotal"`
	VATRawTotal            *Money `bson:"vatRawTotal"`
	VATRoundupTotal        *Money `bson:"vatRoundupTotal"`
	CommissionRawTotal     *Money `bson:"commissionRawTotal"`
	CommissionRoundupTotal *Money `bson:"commissionRoundupTotal"`
	ShareRawTotal          *Money `bson:"shareRawTotal"`
	ShareRoundupTotal      *Money `bson:"shareRoundupTotal"`
	ShipmentRawTotal       *Money `bson:"shipmentRawTotal"`
	ShipmentRoundupTotal   *Money `bson:"shipmentRoundupTotal"`
}

type OrderInfo struct {
	TriggerName    string             `bson:"triggerName"`
	TriggerHistory primitive.ObjectID `bson:"triggerHistory"`
	Orders         []*SellerOrder     `bson:"orders"`
}

type SellerOrder struct {
	OId                uint64        `bson:"oid"`
	FId                string        `bson:"fid"`
	SellerId           uint64        `bson:"sellerId"`
	ShipmentAmount     *Money        `bson:"shipmentAmount"`
	RawShippingNet     *Money        `bson:"rawShippingNet"`
	RoundupShippingNet *Money        `bson:"roundupShippingNet"`
	Items              []*SellerItem `bson:"items"`
	OrderCreatedAt     time.Time     `bson:"orderCreatedAt"`
	SubPkgCreatedAt    time.Time     `bson:"subPkgCreatedAt"`
	SubPkgUpdatedAt    time.Time     `bson:"subPkgUpdatedAt"`
	DeletedAt          *time.Time    `bson:"deletedAt"`
}

type SellerItem struct {
	SId         uint64                `bson:"sid"`
	SKU         string                `bson:"sku"`
	InventoryId string                `bson:"inventoryId"`
	Title       string                `bson:"title"`
	Brand       string                `bson:"brand"`
	Guaranty    string                `bson:"guaranty"`
	Category    string                `bson:"category"`
	Image       string                `bson:"image"`
	Returnable  bool                  `bson:"returnable"`
	Quantity    int32                 `bson:"quantity"`
	Attributes  map[string]*Attribute `bson:"attributes"`
	Invoice     *ItemInvoice          `bson:"invoice"`
}

type Attribute struct {
	KeyTranslate   map[string]string `bson:"keyTranslate"`
	ValueTranslate map[string]string `bson:"valueTranslate"`
}

type ItemInvoice struct {
	Commission *ItemCommission `bson:"commission"`
	Share      *ItemShare      `bson:"share"`
	SSO        *ItemSSO        `bson:"sso"`
	VAT        *ItemVAT        `bson:"vat"`
}

type ItemShare struct {
	RawItemNet              *Money `bson:"rawItemNet"`
	RoundupItemNet          *Money `bson:"roundupItemNet"`
	RawTotalNet             *Money `bson:"rawTotalNet"`
	RoundupTotalNet         *Money `bson:"roundupTotalNet"`
	RawUnitSellerShare      *Money `bson:"rawUnitSellerShare"`
	RoundupUnitSellerShare  *Money `bson:"roundupUnitSellerShare"`
	RawTotalSellerShare     *Money `bson:"rawTotalSellerShare"`
	RoundupTotalSellerShare *Money `bson:"roundupTotalSellerShare"`
}

type ItemCommission struct {
	ItemCommission    float32 `bson:"itemCommission"`
	RawUnitPrice      *Money  `bson:"rawUnitPrice"`
	RoundupUnitPrice  *Money  `bson:"roundupUnitPrice"`
	RawTotalPrice     *Money  `bson:"rawTotalPrice"`
	RoundupTotalPrice *Money  `bson:"roundupTotalPrice"`
}

type ItemSSO struct {
	Rate              float32 `bson:"rate"`
	IsObliged         bool    `bson:"isObliged"`
	RawUnitPrice      *Money  `bson:"rawUnitPrice"`
	RoundupUnitPrice  *Money  `bson:"roundupUnitPrice"`
	RawTotalPrice     *Money  `bson:"rawTotalPrice"`
	RoundupTotalPrice *Money  `bson:"roundupTotalPrice"`
}

type ItemVAT struct {
	Rate              float32 `bson:"rate"`
	IsObliged         bool    `bson:"isObliged"`
	RawUnitPrice      *Money  `bson:"rawUnitPrice"`
	RoundupUnitPrice  *Money  `bson:"roundupUnitPrice"`
	RawTotalPrice     *Money  `bson:"rawTotalPrice"`
	RoundupTotalPrice *Money  `bson:"roundupTotalPrice"`
}

type Money struct {
	Amount   string `bson:"amount"`
	Currency string `bson:"currency"`
}
