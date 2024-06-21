package model

import (
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
)

// Expense Amount paid in fees and other expenses
type Expense struct {
	ID         uint64            `gorm:"PRIMARY_KEY"`
	RefID      string            `gorm:"column:ref_id" json:"ref_id"`
	RefType    OperationType     `sql:"not null;type:operation_type_t;" json:"ref_type"`
	Debit      *postgres.Decimal `sql:"type:decimal(36,18)"`
	Credit     *postgres.Decimal `sql:"type:decimal(36,18)"`
	Coin       Coin              `gorm:"foreignkey:CoinSymbol"`
	CoinSymbol string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewExpense Create a new Expense record
func NewExpense(coinSymbol string, refType OperationType, refID string, debit, credit *decimal.Big) *Expense {
	return &Expense{
		RefID:      refID,
		RefType:    refType,
		CoinSymbol: coinSymbol,
		Debit:      &postgres.Decimal{V: debit},
		Credit:     &postgres.Decimal{V: credit},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
