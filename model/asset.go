package model

import (
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
)

// Asset physical or virtual assets of the exchange
type Asset struct {
	ID         uint64            `gorm:"PRIMARY_KEY"`
	RefID      string            `gorm:"column:ref_id" json:"ref_id"`
	RefType    OperationType     `sql:"not null;type:operation_type_t;" json:"ref_type"`
	Debit      *postgres.Decimal `sql:"type:decimal(36,18)"`
	Credit     *postgres.Decimal `sql:"type:decimal(36,18)"`
	Coin       Coin              `gorm:"foreignkey:CoinSymbol"`
	CoinSymbol string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	Account    AccountType
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewAsset Create a new asset record
func NewAsset(coinSymbol string, account AccountType, refType OperationType, refID string, debit, credit *decimal.Big) *Asset {
	return &Asset{
		Account:    account,
		RefID:      refID,
		RefType:    refType,
		CoinSymbol: coinSymbol,
		Debit:      &postgres.Decimal{V: debit},
		Credit:     &postgres.Decimal{V: credit},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
