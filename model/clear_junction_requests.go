package model

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/payments/clear_junction"
	"time"
)

type ClearJunctionRequestStatus string

const (
	ClearJunctionRequestStatus_New     ClearJunctionRequestStatus = "new"
	ClearJunctionRequestStatus_Success ClearJunctionRequestStatus = "success"
	ClearJunctionRequestStatus_Failed  ClearJunctionRequestStatus = "failed"
)

type ClearJunctionTransactionStatus string

const (
	ClearJunctionTransactionStatus_Created    ClearJunctionTransactionStatus = "created"
	ClearJunctionTransactionStatus_Expired    ClearJunctionTransactionStatus = "expired"
	ClearJunctionTransactionStatus_Canceled   ClearJunctionTransactionStatus = "canceled"
	ClearJunctionTransactionStatus_Rejected   ClearJunctionTransactionStatus = "rejected"
	ClearJunctionTransactionStatus_Returned   ClearJunctionTransactionStatus = "returned"
	ClearJunctionTransactionStatus_Pending    ClearJunctionTransactionStatus = "pending"
	ClearJunctionTransactionStatus_Authorized ClearJunctionTransactionStatus = "authorized"
	ClearJunctionTransactionStatus_Captured   ClearJunctionTransactionStatus = "captured"
	ClearJunctionTransactionStatus_Settled    ClearJunctionTransactionStatus = "settled"
	ClearJunctionTransactionStatus_Declined   ClearJunctionTransactionStatus = "declined"
)

func (s ClearJunctionTransactionStatus) IsValidForDeposit() bool {
	switch s {
	case ClearJunctionTransactionStatus_Created,
		ClearJunctionTransactionStatus_Expired,
		ClearJunctionTransactionStatus_Canceled,
		ClearJunctionTransactionStatus_Rejected,
		ClearJunctionTransactionStatus_Returned,
		ClearJunctionTransactionStatus_Pending,
		ClearJunctionTransactionStatus_Authorized,
		ClearJunctionTransactionStatus_Captured,
		ClearJunctionTransactionStatus_Settled,
		ClearJunctionTransactionStatus_Declined:
		return true
	default:
		return false
	}
}

func (s ClearJunctionTransactionStatus) IsValidForWithdrawal() bool {
	switch s {
	case ClearJunctionTransactionStatus_Captured,
		ClearJunctionTransactionStatus_Settled:
		return true
	default:
		return false
	}
}

type ClearJunctionTransactionType string

const (
	ClearJunctionTransactionType_Deposit          ClearJunctionTransactionType = "Payin"
	ClearJunctionTransactionType_Withdrawal       ClearJunctionTransactionType = "Payout"
	ClearJunctionTransactionType_WithdrawalReturn ClearJunctionTransactionType = "PayoutReturn"
	ClearJunctionTransactionType_Internal         ClearJunctionTransactionType = "TransferWallet"
)

// ClearJunctionRequest structure
type ClearJunctionRequest struct {
	ID                uint64                         `sql:"type:bigint" gorm:"primary_key" json:"id"`
	OrderRefId        string                         `gorm:"column:order_ref_id;" json:"order_ref_id"`
	RefId             string                         `json:"ref_id"`
	TransactionType   ClearJunctionTransactionType   `gorm:"type:clear_junction_event_type;" json:"type"`
	UserID            uint64                         `sql:"type:bigint" gorm:"column:user_id" json:"user_id"`
	CoinSymbol        string                         `gorm:"column:coin_symbol" json:"coin_symbol"`
	Amount            *postgres.Decimal              `sql:"type:decimal(36,18)" json:"amount"`
	Status            ClearJunctionRequestStatus     `sql:"type:clear_junction_status_type_t" json:"status"`
	Method            string                         `json:"method"`
	CreatedAt         time.Time                      `json:"created_at"`
	UpdatedAt         time.Time                      `json:"updated_at"`
	Response          string                         `gorm:"column:response" json:"-"`
	TransactionStatus ClearJunctionTransactionStatus `gorm:"column:clear_junction_transaction_status_type_t;" json:"transaction_status"`
}

type ClearJunctionWithdrawRequestForm struct {
	Method     clear_junction.PaymentType `json:"method"`
	FirstName  string                     `json:"first_name"`
	LastName   string                     `json:"last_name"`
	Requisites struct {
		Iban              string `json:"iban,omitempty"`
		BankSwiftCode     string `json:"bank_swift_code,omitempty"`
		SortCode          string `json:"sort_code,omitempty"`
		AccountNumber     string `json:"account_number,omitempty"`
		BankAccountNumber string `json:"bank_account_number,omitempty"`
	} `json:"requisites"`
}
