package model

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"time"
)

type Balance24h struct {
	ID         uint64            `sql:"type:bigint" gorm:"primary_key" json:"id"`
	UserID     uint64            `sql:"type:bigint" gorm:"column:user_id" json:"user_id"`
	SubAccount uint64            `sql:"type:bigint" json:"sub_account"`
	CoinSymbol string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	Total      *postgres.Decimal `sql:"type:decimal(36,18)" json:"total"`
	TotalCross *postgres.Decimal `sql:"type:decimal(36,18)" json:"total_cross"`
	Percent    *postgres.Decimal `sql:"type:decimal(36,18)" json:"percent"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}
