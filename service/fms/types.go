package fms

import (
	"context"
	"sync"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

type BalanceView struct {
	Available *decimal.Big `json:"available"`
	Locked    *decimal.Big `json:"locked"`
	InOrders  *decimal.Big `json:"in_orders"`
}

// Balances godoc
type Balances map[string]BalanceView

type AccountBalances struct {
	balancesLock *sync.RWMutex
	balances     Balances
	userID       uint64
	subAccountID uint64
}

type user struct {
	accountsLock *sync.RWMutex
	accounts     map[uint64]*AccountBalances
}

type FundsEngine struct {
	ctx       context.Context
	usersLock *sync.RWMutex
	repo      *queries.Repo
	users     map[uint64]*user
}
