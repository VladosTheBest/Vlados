package model

import (
	"encoding/json"
	"hash/fnv"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
)

// WithdrawStatus marks the status of the withdraw request
// swagger:model WithdrawStatus
// example: pending
// enum: pending,user_approved,admin_approved,in_progress,completed,failed
type WithdrawStatus string

const (
	WithdrawStatus_Pending          WithdrawStatus = "pending"
	WithdrawStatus_UserApproved     WithdrawStatus = "user_approved"
	WithdrawStatus_AdminApproved    WithdrawStatus = "admin_approved"
	WithdrawStatus_InProgress       WithdrawStatus = "in_progress"
	WithdrawStatus_Completed        WithdrawStatus = "completed"
	WithdrawStatus_Failed           WithdrawStatus = "failed"
	WithdrawStatus_Pending_Canceled WithdrawStatus = "pending_cancellation"
)

func (w WithdrawStatus) IsValid() bool {
	switch w {
	case WithdrawStatus_Pending,
		WithdrawStatus_UserApproved,
		WithdrawStatus_AdminApproved,
		WithdrawStatus_InProgress,
		WithdrawStatus_Completed,
		WithdrawStatus_Failed,
		WithdrawStatus_Pending_Canceled:
		return true
	default:
		return false
	}
}

// WithdrawExternalSystem external system
// swagger:model WithdrawExternalSystem
// example: bitgo
// enum: bitgo, advcash, clear_junction
type WithdrawExternalSystem string

const (
	WithdrawExternalSystem_Advcash       WithdrawExternalSystem = "advcash"
	WithdrawExternalSystem_Bitgo         WithdrawExternalSystem = "bitgo"
	WithdrawExternalSystem_ClearJunction WithdrawExternalSystem = "clear_junction"
	WithdrawExternalSystem_Default       WithdrawExternalSystem = "default"
)

func (u WithdrawExternalSystem) String() string {
	return string(u)
}

func (u WithdrawExternalSystem) IsValid() bool {
	switch u {
	case WithdrawExternalSystem_Advcash,
		WithdrawExternalSystem_Bitgo,
		WithdrawExternalSystem_ClearJunction,
		WithdrawExternalSystem_Default:
		return true
	default:
		return false
	}
}

// WithdrawRequest request to move funds outside the system
//
// # WithdrawRequest request to move funds outside the system
//
// swagger:model WithdrawRequest
type WithdrawRequest struct {
	// A unique identifier
	//
	// required: true
	ID string `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	// Amount that will be received
	//
	// example: 0.234
	// required: true
	Amount *postgres.Decimal `sql:"type:decimal(36,18)"`
	// Amount in fees paid to the exchange
	//
	// example: 0.001
	// required: true
	FeeAmount *postgres.Decimal `sql:"type:decimal(36,18)"`
	// The coin to withdraw
	//
	// example: eth
	// required: true
	CoinSymbol string `gorm:"column:coin_symbol" json:"coin_symbol"`
	Coin       Coin   `gorm:"foreignkey:CoinSymbol"`
	// The id of the current user
	//
	// example: 231
	// required: true
	UserID uint64 `gorm:"column:user_id" json:"user_id"`
	User   User   `gorm:"foreignkey:UserID"`

	// Destination address
	//
	// example: 1F1tAAsdx1HUXrCNLbtMDqcw6o5GNn4xqX
	// required: true
	To string
	// Extra data associated with the request
	//
	// required: false
	Data string `gorm:"column:data" json:"-"`
	// The status of the request
	//
	// required: true
	// example: pending
	Status WithdrawStatus `sql:"not null;type:withdraw_status_t;default:'pending'" json:"status"`

	// The ID of the transaction that included this withdraw request if processed
	//
	// required: false
	TxID string `gorm:"column:txid" json:"txid"`
	// Extra notes for the transaction
	//
	// required: false
	Notes string

	// Created date
	//
	// required: true
	CreatedAt time.Time
	// Updated date
	//
	// required: false
	UpdatedAt time.Time

	// External system
	//
	// required: true
	// example: bitgo
	ExternalSystem WithdrawExternalSystem `sql:"not null;type:withdrawal_external_system_t" json:"external_system"`
}

func (t *WithdrawRequest) GetHashID() uint64 {
	h := fnv.New64()
	_, _ = h.Write([]byte(t.ID))
	return h.Sum64()
}

type WithdrawRequestWithUserEmail struct {
	WithdrawRequest
	UserEmail          string `gorm:"column:email" json:"user_email"`
	BlockchainExplorer string `json:"blockchain_explorer"`
}

// WithdrawRequestWithBlockchainLink structure
type WithdrawRequestWithBlockchainLink struct {
	ID         string            `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	Amount     *postgres.Decimal `sql:"type:decimal(36,18)"`
	FeeAmount  *postgres.Decimal `sql:"type:decimal(36,18)"`
	CoinSymbol string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	UserID     uint64            `gorm:"column:user_id" json:"user_id"`
	To         string
	Status     WithdrawStatus `sql:"not null;type:withdraw_status_t;default:'pending'" json:"status"`

	TxID               string `gorm:"column:txid" json:"txid"`
	BlockchainExplorer string `json:"blockchain_explorer"`
	CreatedAt          time.Time
}

// WithdrawRequestList structure
type WithdrawRequestList struct {
	WithdrawRequests []WithdrawRequestWithUserEmail
	Meta             PagingMeta
}

// WithdrawRequestWithUser strucutre
type WithdrawRequestWithUser struct {
	WithdrawRequest
	FirstName          string `json:"first_name"`
	LastName           string `json:"last_name"`
	BlockchainExplorer string `json:"blockchain_explorer"`
	UserEmail          string `gorm:"column:email" json:"email"`
}

// WithdrawRequestWithUserList structure
type WithdrawRequestWithUserList struct {
	WithdrawRequests []WithdrawRequestWithUser
	Meta             PagingMeta
}

// WithdrawRequestWithBlockchainLinkList structure
type WithdrawRequestWithBlockchainLinkList struct {
	WithdrawRequests []WithdrawRequestWithBlockchainLink
	Meta             PagingMeta
}

// NewWithdrawRequest make a request to withdraw your balance outside of the system
func NewWithdrawRequest(userID uint64, coinSymbol string, amount, feeAmount *decimal.Big, to, data, notes string, externalSystem WithdrawExternalSystem) *WithdrawRequest {
	return &WithdrawRequest{
		UserID:         userID,
		Status:         WithdrawStatus_Pending,
		CoinSymbol:     coinSymbol,
		Amount:         &postgres.Decimal{V: amount},
		FeeAmount:      &postgres.Decimal{V: feeAmount},
		To:             to,
		Data:           data,
		Notes:          notes,
		ExternalSystem: externalSystem,
	}
}

func (withdraw *WithdrawRequest) GetAmount() *decimal.Big {
	return withdraw.Amount.V
}

func (withdraw *WithdrawRequest) GetFeeAmount() *decimal.Big {
	return withdraw.FeeAmount.V
}

func (withdraw *WithdrawRequest) SetAmount(amount *decimal.Big) *WithdrawRequest {
	withdraw.Amount = &postgres.Decimal{V: amount}
	return withdraw
}

func (withdraw *WithdrawRequest) SetFee(amount *decimal.Big) *WithdrawRequest {
	withdraw.FeeAmount = &postgres.Decimal{V: amount}
	return withdraw
}

// SetStatusInProgress - set status in progress for a withdraw
func (withdraw *WithdrawRequest) SetStatusInProgress(txid, data, notes string) *WithdrawRequest {
	withdraw.SetStatus(WithdrawStatus_InProgress)
	withdraw.TxID = txid
	withdraw.Data = data
	withdraw.Notes = notes
	return withdraw
}

// SetStatus - change status for a withdraw
func (withdraw *WithdrawRequest) SetStatus(status WithdrawStatus) *WithdrawRequest {
	withdraw.Status = status
	return withdraw
}

// MarshalJSON - convert the request into a json
func (withdraw *WithdrawRequest) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":          withdraw.ID,
		"user_id":     withdraw.UserID,
		"status":      withdraw.Status,
		"coin_symbol": withdraw.CoinSymbol,
		"txid":        withdraw.TxID,
		"amount":      withdraw.Amount.V.String(),
		"fee_amount":  withdraw.FeeAmount.V.String(),
		"to":          withdraw.To,
		"notes":       withdraw.Notes,
		"created_at":  withdraw.CreatedAt,
		"updated_at":  withdraw.UpdatedAt,
	})
}

// MarshalJSON - convert the request into a json
func (withdraw *WithdrawRequestWithBlockchainLink) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                  withdraw.ID,
		"user_id":             withdraw.UserID,
		"status":              withdraw.Status,
		"coin_symbol":         withdraw.CoinSymbol,
		"txid":                withdraw.TxID,
		"amount":              withdraw.Amount.V.String(),
		"fee_amount":          withdraw.FeeAmount.V.String(),
		"to":                  withdraw.To,
		"blockchain_explorer": withdraw.BlockchainExplorer,
		"created_at":          withdraw.CreatedAt,
	})
}

// MarshalJSON - convert the request into a json
func (withdraw *WithdrawRequestWithUser) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                  withdraw.ID,
		"user_id":             withdraw.UserID,
		"status":              withdraw.Status,
		"coin_symbol":         withdraw.CoinSymbol,
		"txid":                withdraw.TxID,
		"amount":              withdraw.Amount.V.String(),
		"fee_amount":          withdraw.FeeAmount.V.String(),
		"blockchain_explorer": withdraw.BlockchainExplorer,
		"to":                  withdraw.To,
		"notes":               withdraw.Notes,
		"email":               withdraw.UserEmail,
		"created_at":          withdraw.CreatedAt,
		"updated_at":          withdraw.UpdatedAt,
		"first_name":          withdraw.FirstName,
		"last_name":           withdraw.LastName,
		"external_system":     withdraw.ExternalSystem,
		"data":                withdraw.Data,
	})
}

func (withdraw *WithdrawRequestWithUserEmail) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                  withdraw.ID,
		"user_id":             withdraw.UserID,
		"status":              withdraw.Status,
		"coin_symbol":         withdraw.CoinSymbol,
		"txid":                withdraw.TxID,
		"amount":              withdraw.Amount.V.String(),
		"fee_amount":          withdraw.FeeAmount.V.String(),
		"to":                  withdraw.To,
		"notes":               withdraw.Notes,
		"user_email":          withdraw.UserEmail,
		"created_at":          withdraw.CreatedAt,
		"updated_at":          withdraw.UpdatedAt,
		"blockchain_explorer": withdraw.BlockchainExplorer,
		"external_system":     withdraw.ExternalSystem,
		"data":                withdraw.Data,
	})
}
