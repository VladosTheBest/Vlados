package model

import (
	"encoding/json"
	"github.com/ericlagergren/decimal"
	"time"

	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// ManualDistributionList structure
type ManualDistributionList struct {
	Distributions []ManualDistribution
	Meta          PagingMeta
}

// ManualDistribution provides a referance for each distribution event
type ManualDistribution struct {
	ID                uint64             `gorm:"PRIMARY_KEY"`
	RefID             string             `gorm:"column:ref_id" json:"ref_id"`
	CompletedByUserId uint64             `gorm:"completed_by_user_id" json:"completed_by_user_id"`
	Status            DistributionStatus `sql:"not null;type:distribution_status_t;default:'pending'" json:"status"`
	LastRevenueID     uint64             `sql:"type:bigint REFERENCES revenues(id)" json:"last_revenue_id"`
	MMPercent         *postgres.Decimal  `gorm:"mm_percent" json:"mm_percent"`
	Day               time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// MarshalJSON godoc
func (distribution ManualDistribution) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                   distribution.ID,
		"last_revenue_id":      distribution.LastRevenueID,
		"ref_id":               distribution.RefID,
		"completed_by_user_id": distribution.CompletedByUserId,
		"status":               distribution.Status,
		"mm_percent":           utils.Fmt(distribution.MMPercent.V),
		"created_at":           distribution.CreatedAt.Unix(),
		"updated_at":           distribution.UpdatedAt.Unix(),
		"day":                  distribution.Day.Unix(),
	})
}

// ManualDistributionFundsList godoc
type ManualDistributionFundsList struct {
	DistributionFunds []ManualDistributionFund
	Meta              PagingMeta
	ApproximateAmount *decimal.Big
}

// ManualDistributionFund structure
type ManualDistributionFund struct {
	ID               uint64            `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"id"`
	DistributionID   uint64            `sql:"type:bigint REFERENCES manual_distributions(id)" json:"distribution_id"`
	CoinSymbol       string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	LastRevenueID    uint64            `sql:"type:bigint REFERENCES revenues(id)" json:"last_revenue_id"`
	TotalBalance     *postgres.Decimal `sql:"type:decimal(36,18)" json:"total_balance"`
	ConvertedBalance *postgres.Decimal `sql:"type:decimal(36,18)" json:"converted_balance"`
	// ConvertedCoinSymbol *string                `sql:"type:varchar(10) REFERENCES coins(symbol)" json:"converted_coin_symbol"` TODO: set default coins
	ConvertedCoinSymbol string `sql:"type:varchar(10)" gorm:"column:converted_coin_symbol" json:"converted_coin_symbol"`
	Status              string `sql:"not null;type:dist_funds_status_t;" json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MarshalJSON godoc
func (fund ManualDistributionFund) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                    fund.ID,
		"distribution_id":       fund.DistributionID,
		"status":                fund.Status,
		"last_revenue_id":       fund.LastRevenueID,
		"total_balance":         utils.Fmt(fund.TotalBalance.V),
		"coin_symbol":           fund.CoinSymbol,
		"converted_balance":     utils.Fmt(fund.ConvertedBalance.V),
		"converted_coin_symbol": fund.ConvertedCoinSymbol,
		"created_at":            fund.CreatedAt.Unix(),
		"updated_at":            fund.UpdatedAt.Unix(),
	})
}

// ManualDistributionBalancesList godoc
type ManualDistributionBalancesList struct {
	DistributionBalances []ManualDistributionBalance
	Meta                 PagingMeta
	TotalRedeemedPRDX    *decimal.Big
}

type DistributionBalanceStatus string

const (
	DistributionBalanceStatus_Pending   DistributionBalanceStatus = "pending"
	DistributionBalanceStatus_Allocated DistributionBalanceStatus = "allocated"
	DistributionBalanceStatus_Claimed   DistributionBalanceStatus = "claimed"
	DistributionBalanceStatus_Unused    DistributionBalanceStatus = "unused"
)

// ManualDistributionBalance structure
type ManualDistributionBalance struct {
	ID                  uint64                    `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"id"`
	DistributionID      uint64                    `sql:"type:bigint REFERENCES manual_distributions(id)" json:"distribution_id"`
	CoinSymbol          string                    `gorm:"column:coin_symbol" json:"coin_symbol"`
	UserID              uint64                    `sql:"type:bigint REFERENCES users(id)" json:"user_id"`
	Level               string                    `sql:"not null;type:user_level_t;" json:"level"`
	Balance             *postgres.Decimal         `sql:"type:decimal(36,18)" json:"balance"`
	AllocatedBalance    *postgres.Decimal         `sql:"type:decimal(36,18)" json:"allocated_balance"`
	AllocatedCoinSymbol string                    `gorm:"column:allocated_coin_symbol" json:"allocated_coin_symbol"`
	UserEmail           string                    `gorm:"column:email" json:"user_email"`
	Status              DistributionBalanceStatus `sql:"not null;type:distribution_balance_status_t;" json:"status"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
}

// MarshalJSON godoc
func (balance ManualDistributionBalance) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                    balance.ID,
		"distribution_id":       balance.DistributionID,
		"user_id":               balance.UserID,
		"level":                 balance.Level,
		"balance":               utils.Fmt(balance.Balance.V),
		"coin_symbol":           balance.CoinSymbol,
		"allocated_balance":     utils.Fmt(balance.AllocatedBalance.V),
		"allocated_coin_symbol": balance.AllocatedCoinSymbol,
		"user_email":            balance.UserEmail,
		"status":                balance.Status,
		"created_at":            balance.CreatedAt.Unix(),
		"updated_at":            balance.UpdatedAt.Unix(),
	})
}
