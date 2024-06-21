package model

import (
	"encoding/json"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
)

type GeneratedMode string

const (
	GeneratedModeByBotAndManual GeneratedMode = ""
	GeneratedModeByBot          GeneratedMode = "bot"
	GeneratedModeByManual       GeneratedMode = "manual"
)

func (gm GeneratedMode) IsValid() bool {
	switch gm {
	case GeneratedModeByBot,
		GeneratedModeByManual,
		GeneratedModeByBotAndManual:
		return true
	default:
		return false
	}
}

// Revenue Amount in fees charged to a user
type Revenue struct {
	ID          uint64            `gorm:"PRIMARY_KEY" wire:"id"`
	RefID       string            `gorm:"column:ref_id" json:"ref_id" wire:"ref_id"`
	RefType     OperationType     `sql:"not null;type:operation_type_t;" json:"ref_type" wire:"ref_type"`
	Debit       *postgres.Decimal `sql:"type:decimal(36,18)" wire:"debit"`
	Credit      *postgres.Decimal `sql:"type:decimal(36,18)" wire:"credit"`
	Coin        Coin              `gorm:"foreignkey:CoinSymbol"`
	CoinSymbol  string            `gorm:"column:coin_symbol" json:"coin_symbol" wire:"coin_symbol"`
	User        User              `gorm:"foreignkey:UserID"`
	UserID      uint64            `gorm:"column:user_id" json:"user_id" wire:"user_id"`
	Account     AccountType       `wire:"account"`
	SubAccount  uint64            `json:"sub_account" wire:"sub_account"`
	RefObjectId uint64            `gorm:"ref_object_id" wire:"ref_object_id"`
	CreatedAt   time.Time         `wire:"created_at"`
	UpdatedAt   time.Time         `wire:"updated_at"`
}

// NewRevenue Create a new Revenue record
func NewRevenue(coinSymbol string, account AccountType, refType OperationType, refID string, userID uint64, debit, credit *decimal.Big, subAccount uint64, refObjectId uint64) *Revenue {
	return &Revenue{
		Account:     account,
		RefID:       refID,
		RefType:     refType,
		CoinSymbol:  coinSymbol,
		Debit:       &postgres.Decimal{V: debit},
		Credit:      &postgres.Decimal{V: credit},
		UserID:      userID,
		SubAccount:  subAccount,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RefObjectId: refObjectId,
	}
}

type FeesInfoStats struct {
	Day            string            `json:"day"`
	Coin           string            `json:"coin"`
	TokenPrecision int               `json:"token_precision"`
	Fee            *postgres.Decimal `json:"fee" sql:"type:decimal(36,18)"`
}

// MarshalJSON convert the order into a json string
func (fees *FeesInfoStats) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"day":  fees.Day,
		"coin": fees.Coin,
		"fee":  utils.FmtDecimalWithPrecision(fees.Fee, fees.TokenPrecision),
	})
}
