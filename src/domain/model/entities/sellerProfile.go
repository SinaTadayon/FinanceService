package entities

import "time"

type SellerProfile struct {
	SellerId    int64              `bson:"sellerId"`
	GeneralInfo *GeneralSellerInfo `bson:"generalInfo"`
	FinanceData *SellerFinanceData `bson:"financeData"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"`
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

type SellerFinanceData struct {
	Iban                    string `bson:"iban"`
	AccountHolderFirstName  string `bson:"accountHolderFirstName"`
	AccountHolderFamilyName string `bson:"accountHolderFamilyName"`
}
