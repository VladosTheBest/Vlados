package model

import (
	"time"

	"github.com/ericlagergren/decimal/sql/postgres"
)

type BalancesSchema struct {
	ID              uint64
	UserID          uint64
	LastLiabilityID uint64
	CoinSymbol      string
	Available       *postgres.Decimal
	Locked          *postgres.Decimal
	InOrders        *postgres.Decimal
	CreatedAt       time.Time
	UpdatedAt       time.Time
	AccountGroup    AccountGroup
	SubAccount      SubAccount
}

func (b BalancesSchema) Table() string {
	return "balances"
}
