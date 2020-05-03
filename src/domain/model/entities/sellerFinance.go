package entities

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type FinanceState string
type TransferState string

const (
	DocumentVersion string = "1.0.0"
)

const (
	FinanceNewStatus        FinanceState = "NEW"
	FinanceInProgressStatus FinanceState = "IN_PROGRESS"
	FinanceClosedStatus     FinanceState = "CLOSED"
)

const (
	TransferSuccessState TransferState = "SUCCESS"
	TransferFAILEDState  TransferState = "FAILED"
	TransferPENDINGState TransferState = "PENDING"
	TransferPARTIALState TransferState = "PARTIAL_PAYED"
)

type SellerFinance struct {
	ID         primitive.ObjectID `bson:"-"`
	FId        string             `bson:"fid"`
	SellerId   uint64             `bson:"sellerId"`
	Version    uint64             `bson:"version"`
	DocVersion string             `bson:"docVersion"`
	SellerInfo *SellerProfile     `bson:"sellerInfo"`
	Invoice    Invoice            `bson:"invoice"`
	Orders     []*OrderFinance    `bson:"orders"`
	Payment    *FinancePayment    `bson:"payment"`
	Status     string             `bson:"status"`
	Start      *time.Time         `bson:"start"`
	End        *time.Time         `bson:"end"`
	CreatedAt  time.Time          `bson:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"`
	DeletedAt  *time.Time         `bson:"deletedAt"`
}

type FinancePayment struct {
	TransferRequest  *TransferRequest  `bson:"transferRequest"`
	TransferResponse *TransferResponse `bson:"transferResponse"`
	TransferResult   *TransferResult   `bson:"transferResult"`
	Status           string            `bson:"status"`
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

type OrderFinance struct {
	OId                uint64     `bson:"oid"`
	FId                string     `bson:"fid"`
	SellerId           uint64     `bson:"sellerId"`
	RawShippingNet     *Money     `bson:"rawShippingNet"`
	RoundupShippingNet *Money     `bson:"roundupShippingNet"`
	Items              []*Item    `bson:"items"`
	CreatedAt          time.Time  `bson:"createdAt"`
	UpdatedAt          time.Time  `bson:"updatedAt"`
	DeletedAt          *time.Time `bson:"deletedAt"`
}

type Item struct {
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
	Invoice     ItemInvoice           `bson:"invoice"`
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
