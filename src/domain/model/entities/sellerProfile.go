package entities

import "time"

type SellerProfile struct {
	SellerId        int64                 `bson:"sellerId"`
	GeneralInfo     *GeneralSellerInfo    `bson:"generalInfo"`
	CorporationInfo *CorporateSellerInfo  `bson:"corporationInfo"`
	IndividualInfo  *IndividualSellerInfo `bson:"individualInfo"`
	ReturnInfo      *ReturnInfo           `bson:"returnInfo"`
	ContactPerson   *SellerContactPerson  `bson:"contactPerson"`
	ShipmentInfo    *SellerShipmentInfo   `bson:"shipmentInfo"`
	FinanceData     *SellerFinanceData    `bson:"financeData"`
	CreatedAt       time.Time             `bson:"createdAt"`
	UpdatedAt       time.Time             `bson:"updatedAt"`
}

type GeneralSellerInfo struct {
	ShopDisplayName          string `bson:"shopDisplayName"`
	Type                     string `bson:"type"`
	Email                    string `bson:"email"`
	LandPhone                string `bson:"landPhone"`
	MobilePhone              string `bson:"mobilePhone"`
	Website                  string `bson:"website"`
	Province                 string `bson:"province"`
	City                     string `bson:"city"`
	Neighborhood             string `bson:"neighborhood"`
	PostalAddress            string `bson:"postalAddress"`
	PostalCode               string `bson:"postalCode"`
	IsVATObliged             bool   `bson:"isVatObliged"`
	VATCertificationImageURL string `bson:"vatCertificationImageURL"`
}

type CorporateSellerInfo struct {
	CompanyRegisteredName     string `bson:"companyRegisteredName"`
	CompanyRegistrationNumber string `bson:"companyRegistrationNumber"`
	CompanyRationalId         string `bson:"companyRationalId"`
	TradeNumber               string `bson:"tradeNumber"`
}

type IndividualSellerInfo struct {
	FirstName          string `bson:"firstName"`
	FamilyName         string `bson:"familyName"`
	NationalId         string `bson:"nationalId"`
	NationalIdFrontURL string `bson:"nationalIdFrontURL"`
	NationalIdBackURL  string `bson:"nationalIdBackURL"`
}

type ReturnInfo struct {
	Country       string `bson:"country"`
	Province      string `bson:"province"`
	City          string `bson:"city"`
	Neighborhood  string `bson:"neighborhood"`
	PostalAddress string `bson:"postalAddress"`
	PostalCode    string `bson:"postalCode"`
}

type SellerContactPerson struct {
	FirstName   string `bson:"firstName"`
	FamilyName  string `bson:"familyName"`
	MobilePhone string `bson:"mobilePhone"`
	Email       string `bson:"email"`
}

type SellerShipmentInfo struct {
	SameCity      *PricePlan `bson:"sameCity"`
	DifferentCity *PricePlan `bson:"differentCity"`
}

type PricePlan struct {
	Threshold        int64 `bson:"threshold"`
	BelowPrice       int64 `bson:"belowPrice"`
	ReactionTimeDays int64 `bson:"reactionTimeDays"`
}

type SellerFinanceData struct {
	Iban                    string `bson:"iban"`
	AccountHolderFirstName  string `bson:"accountHolderFirstName"`
	AccountHolderFamilyName string `bson:"accountHolderFamilyName"`
}
