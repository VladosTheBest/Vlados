package model

import (
	"hash/fnv"
	"time"

	gouuid "github.com/nu7hatch/gouuid"
)

// AccountType godoc
// The account represents the accounting section where the funds should be placed
type AccountType int

const (
	AccountType_Main AccountType = iota
	AccountType_Locked
)

type OperationStatus string

const (
	OperationStatus_Pending    OperationStatus = "pending"
	OperationStatus_Accepted   OperationStatus = "accepted"
	OperationStatus_Locked     OperationStatus = "locked"
	OperationStatus_Unlocked   OperationStatus = "unlocked"
	OperationStatus_Failed     OperationStatus = "failed"
	OperationStatus_Reverted   OperationStatus = "reverted"
	OperationStatus_Processing OperationStatus = "processing"
	OperationStatus_Completed  OperationStatus = "completed"
)

type OperationType string

const (
	OperationType_Deposit                    OperationType = "deposit"
	OperationType_DepositBonusAccount        OperationType = "deposit_bonus"
	OperationType_DepositCardPaymentAccount  OperationType = "deposit_card"
	OperationType_Withdraw                   OperationType = "withdraw"
	OperationType_WithdrawBonusAccount       OperationType = "withdraw_bonus"
	OperationType_WithdrawCardPaymentAccount OperationType = "withdraw_card"
	OperationType_DepositBot                 OperationType = "deposit_bot"
	OperationType_WithdrawBot                OperationType = "withdraw_bot"
	OperationType_Trade                      OperationType = "trade"
	OperationType_Order                      OperationType = "order"
	OperationType_OrderCancel                OperationType = "order_cancel"
	OperationType_OTCTrade                   OperationType = "otc_trade"
	OperationType_Distribution               OperationType = "distribution"
	OperationType_Referral                   OperationType = "referral"
	OperationType_Refund                     OperationType = "refund"
	OperationType_Launchpad_Trade            OperationType = "launchpad_trade"
	OperationType_Launchpad_EndPresale       OperationType = "launchpad_end_presale"
	OperationType_TransferBetweenSubAccounts OperationType = "transfer_between_sub_accounts"
	OperationType_TransferToPlatform         OperationType = "transfer_to_platform"
)

// Operation provides a referance for any balance changes
type Operation struct {
	ID      uint64          `gorm:"PRIMARY_KEY" json:"id" wire:"id"`
	RefID   string          `gorm:"column:ref_id" json:"ref_id" wire:"ref_id"`
	RefType OperationType   `sql:"not null;type:operation_type_t;" json:"ref_type" wire:"ref_type"`
	Status  OperationStatus `sql:"not null;type:operation_status_t;default:'pending'" json:"status" wire:"status"`

	CreatedAt time.Time `json:"created_at" wire:"created_at"`
	UpdatedAt time.Time `wire:"updated_at"`
}

func (t *Operation) GetHashRefID() uint64 {
	h := fnv.New64()
	_, _ = h.Write([]byte(t.RefID))
	return h.Sum64()
}

// OperationList type
type OperationList struct {
	Operations []Operation
	Meta       PagingMeta
}

// NewOperation Create a new operation for a user
func NewOperation(refType OperationType, status OperationStatus) *Operation {
	u, _ := gouuid.NewV4()
	return &Operation{
		RefID:     u.String(),
		RefType:   refType,
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Model Methods

// GORM Event Handlers
