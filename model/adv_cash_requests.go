package model

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"time"
)

type AdvRequestStatus string

const (
	AdvRequestStatus_New     AdvRequestStatus = "new"
	AdvRequestStatus_Success AdvRequestStatus = "success"
	AdvRequestStatus_Failed  AdvRequestStatus = "failed"
)

func (u AdvRequestStatus) String() string {
	return string(u)
}

type AdvTransactionStatus string

const (
	AdvTransactionStatus_Pending   AdvTransactionStatus = "pending"
	AdvTransactionStatus_Process   AdvTransactionStatus = "process"
	AdvTransactionStatus_Confirmed AdvTransactionStatus = "confirmed"
	AdvTransactionStatus_Completed AdvTransactionStatus = "completed"
	AdvTransactionStatus_Canceled  AdvTransactionStatus = "canceled"
)

func (u AdvTransactionStatus) String() string {
	return string(u)
}

// AdvDepositRequests structure
type AdvDepositRequests struct {
	ID                uint64               `sql:"type:bigint" gorm:"primary_key" json:"id"`
	AdvId             string               `gorm:"column:adv_id;" json:"adv_cash_id"`
	UserID            uint64               `sql:"type:bigint" gorm:"column:user_id" json:"user_id"`
	CoinSymbol        string               `gorm:"column:coin_symbol" json:"coin_symbol"`
	Amount            *postgres.Decimal    `sql:"type:decimal(36,18)" json:"amount"`
	Status            AdvRequestStatus     `sql:"type:adv_deposit_status_type_t" json:"status"`
	Signature         string               `gorm:"column:signature" json:"signature"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
	Response          string               `gorm:"column:adv_response;" json:"adv_response"`
	TransactionStatus AdvTransactionStatus `gorm:"column:adv_transaction_status;" sql:"type:adv_transaction_status_type_t" json:"adv_transaction_status"`
}
