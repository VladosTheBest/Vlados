package model

import (
	"github.com/ericlagergren/decimal"
	"time"
)

type PRDXCirculation struct {
	TotalCirculation         *decimal.Big     `json:"total_circulation"`
	TotalOnChain             *decimal.Big     `json:"total_on_chain"`
	TotalOnExchange          *decimal.Big     `json:"total_on_exchange"`
	AccountDistribution      *decimal.Big     `json:"account_distribution"`
	AccountRecoverNotClaimed *decimal.Big     `json:"account_recover_not_claimed"`
	AccountsMM               []AccountMMValue `json:"accounts_mm"`
	TotalBurned              *decimal.Big     `json:"total_burned"`
	TotalDistributed         *decimal.Big     `json:"total_distributed"`
	TotalUserOnExchange      *decimal.Big     `json:"total_user_on_exchange"`
	PrdxDistributor          *decimal.Big     `json:"prdx_distributor"`
	UnusedPrdx               *decimal.Big     `json:"unused_prdx"`
}

type AccountMMValue struct {
	Email string       `json:"email"`
	Value *decimal.Big `json:"value"`
}

type BurnedTokensInfo struct {
	TotalSupply *decimal.Big `json:"total_supply"`
	LastBurned  *decimal.Big `json:"last_burned"`
	TotalBurned *decimal.Big `json:"total_burned"`
	LastUpdate  time.Time    `json:"last_update"`
}
