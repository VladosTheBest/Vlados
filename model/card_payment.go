package model

import (
	"time"

	"github.com/ericlagergren/decimal/sql/postgres"
)

type CardType struct {
	Id            uint64            `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	Code          CardCode          `gorm:"column:code" json:"code"`
	Name          string            `gorm:"name" json:"name"`
	Level         int               `gorm:"level" json:"level"`
	ShipmentPrice *postgres.Decimal `gorm:"shipment_price" json:"shipment_price"`
	MonthlyFee    *postgres.Decimal `gorm:"monthly_fee" json:"monthly_fee"`

	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"updated_at" json:"updated_at"`
}

func (c CardType) Table() string {
	return "card_type"
}

type CardCode string

const (
	CardCodeVibrantButterfly CardCode = "BUTTEUCL"
	CardCodeCrazyMonkey      CardCode = "MONKEUCL"
	CardCodeVelvetZebra      CardCode = "ZEBREUCL"
	CardCodeTitanRhino       CardCode = "RHINEUCL"
	CardCodeWildLion         CardCode = "LIONEUMT"
	CardCodePlainBlack       CardCode = "BLCKEUMT"
)

type CardAccountStatus string

const (
	CardAccountStatusUnknown     CardAccountStatus = "Unknown"
	CardAccountStatusActive      CardAccountStatus = "Active"
	CardAccountStatusInactive    CardAccountStatus = "Inactive"
	CardAccountStatusIncomplete  CardAccountStatus = "Incomplete"
	CardAccountStatusChecked     CardAccountStatus = "Checked"
	CardAccountStatusDead        CardAccountStatus = "Dead"
	CardAccountStatusPartSignup  CardAccountStatus = "PartSignup"
	CardAccountStatusDeclined    CardAccountStatus = "Declined"
	CardAccountStatusBogus       CardAccountStatus = "Bogus"
	CardAccountStatusLimited     CardAccountStatus = "Limited"
	CardAccountStatusBankCheck   CardAccountStatus = "BankCheck"
	CardAccountStatusWifiLimited CardAccountStatus = "WifiLimited"
	CardAccountStatusLockedOut   CardAccountStatus = "LockedOut"
	CardAccountStatusSuspended   CardAccountStatus = "Suspended"
)

func DetermineCardAccountStatus(status int32) CardAccountStatus {
	switch status {
	case 1:
		return CardAccountStatusActive
	case 2:
		return CardAccountStatusInactive
	case 3:
		return CardAccountStatusIncomplete
	case 4:
		return CardAccountStatusChecked
	case 5:
		return CardAccountStatusDead
	case 6:
		return CardAccountStatusPartSignup
	case 7:
		return CardAccountStatusDeclined
	case 8:
		return CardAccountStatusBogus
	case 9:
		return CardAccountStatusLimited
	case 10:
		return CardAccountStatusBankCheck
	case 11:
		return CardAccountStatusWifiLimited
	case 12:
		return CardAccountStatusLockedOut
	case 13:
		return CardAccountStatusSuspended
	default:
		return CardAccountStatusUnknown
	}
}

func (c CardAccountStatus) String() string {
	return string(c)
}

type CardAccount struct {
	Id                     uint64            `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserId                 uint64            `gorm:"user_id" json:"user_id"`
	ConsumerId             uint64            `gorm:"consumer_id" json:"consumer_id"`
	AccountId              uint64            `gorm:"account_id" json:"account_id"`
	CardTypeId             uint64            `gorm:"card_type_id" json:"card_type_id"`
	AccountNumber          string            `gorm:"account_number" json:"account_number"`
	SortCode               string            `gorm:"sort_code" json:"sort_code"`
	Iban                   string            `gorm:"iban" json:"iban"`
	Bic                    string            `gorm:"bic" json:"bic"`
	Description            string            `gorm:"description" json:"description"`
	Status                 CardAccountStatus `gorm:"status" json:"status"`
	ResponseCode           string            `gorm:"response_code" json:"response_code"`
	ResponseDateTime       time.Time         `gorm:"response_date_time" json:"response_date_time"`
	ClientRequestReference string            `gorm:"client_request_reference" json:"client_request_reference"`
	RequestId              string            `gorm:"request_id" json:"request_id"`
}

type CardAddConsumerRequest struct {
	// Consumer request part
	EncryptedPIN   string    `json:"EncryptedPIN"`
	CardDesignCode string    `json:"CardDesignCode"`
	FirstName      string    `json:"FirstName"`
	LastName       string    `json:"LastName"`
	Gender         string    `json:"Gender"`
	DOB            time.Time `json:"DOB"`

	// Add consumer request part
	PromotionalCode        string `json:"PromotionalCode,omitempty"`
	AgreementCode          string `json:"AgreementCode,omitempty"`
	Language               int32  `json:"Language,omitempty"`
	AccountFriendlyName    string `json:"AccountFriendlyName,omitempty"`
	PayeeRef               string `json:"PayeeRef,omitempty"`
	ClientRequestReference string `json:"ClientRequestReference,omitempty"`
	CultureID              int32  `json:"CultureID,omitempty"`

	// Other request parts
	DeliveryAddress string `json:"DeliveryAddress"`
}

type AllowedCard struct {
	Card      CardType `json:"card"`
	IsAllowed bool     `json:"is_allowed"`
}
