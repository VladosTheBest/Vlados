package model

import (
	"encoding/json"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// Liability Amount owed to a user
type Liability struct {
	ID          uint64            `gorm:"PRIMARY_KEY" wire:"id"`
	RefID       string            `gorm:"column:ref_id" json:"ref_id" wire:"ref_id"`
	RefType     OperationType     `sql:"not null;type:operation_type_t;" json:"ref_type" wire:"ref_type"`
	Debit       *postgres.Decimal `sql:"type:decimal(36,18)" wire:"debit"`
	Credit      *postgres.Decimal `sql:"type:decimal(36,18)" wire:"credit"`
	Coin        Coin              `gorm:"foreignkey:CoinSymbol"`
	CoinSymbol  string            `gorm:"column:coin_symbol" json:"coin_symbol" wire:"coin_symbol"`
	User        User              `gorm:"foreignkey:UserID"`
	UserID      uint64            `gorm:"column:user_id" json:"user_id" wire:"user_id"`
	SubAccount  uint64            `sql:"sub_account" json:"sub_account" wire:"sub_account"`
	Comment     string            `sql:"comment" json:"comment" wire:"comment"`
	Account     AccountType       `wire:"account"`
	CreatedAt   time.Time         `wire:"created_at"`
	UpdatedAt   time.Time         `wire:"updated_at"`
	RefObjectId uint64            `gorm:"ref_object_id" wire:"ref_object_id"`
}

func (l *Liability) AddComment(text string) {
	l.Comment = text
}

// LiabilityList structure
type LiabilityList struct {
	Liabilities []Liability
	Meta        PagingMeta
}

type LiabilityDifferenceCreditDebit struct {
	RefID  string            `sql:"type:varchar(100)" json:"ref_id"`
	Profit *postgres.Decimal `sql:"type:decimal(36,18)" json:"profit"`
}

// NewLiability Create a new Liability record
func NewLiability(coinSymbol string, account AccountType, refType OperationType, refID string, userID uint64, debit, credit *decimal.Big, subAccount uint64, refObjectId uint64) *Liability {
	return &Liability{
		Account:     account,
		SubAccount:  subAccount,
		RefID:       refID,
		RefType:     refType,
		CoinSymbol:  coinSymbol,
		Debit:       &postgres.Decimal{V: debit},
		Credit:      &postgres.Decimal{V: credit},
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RefObjectId: refObjectId,
	}
}

// MarshalJSON JSON encoding of a liability entry
func (liability Liability) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"account":     liability.Account,
		"sub_account": liability.SubAccount,
		"ref_id":      liability.RefID,
		"ref_type":    liability.RefType,
		"user_id":     liability.UserID,
		"coin_symbol": liability.CoinSymbol,
		"credit":      utils.Fmt(liability.Credit.V),
		"debit":       utils.Fmt(liability.Debit.V),
		"created_at":  liability.CreatedAt.Unix(),
		"updated_at":  liability.UpdatedAt.Unix(),
	})
}
