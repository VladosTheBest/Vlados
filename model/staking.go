package model

import (
	"encoding/json"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

type StakingStatus string

const (
	StakingStatusInactive StakingStatus = "inactive"
	StakingStatusActive   StakingStatus = "active"
	StakingStatusExpired  StakingStatus = "expired"
)

type StakingEarningStatus string

const (
	StakingEarningStatusPending StakingEarningStatus = "pending"
	StakingEarningStatusPayed   StakingEarningStatus = "payed"
)

type StakingPeriod string

const (
	StakingPeriodFlexible StakingPeriod = "flexible"
	StakingPeriodWeek     StakingPeriod = "week"
	StakingPeriodMonth    StakingPeriod = "month"
	StakingPeriodQuarter  StakingPeriod = "quarter"
	StakingPeriodHalfYear StakingPeriod = "half_year"
	StakingPeriodYear     StakingPeriod = "year"
)

func (s StakingPeriod) String() string {
	return string(s)
}

func (s StakingPeriod) IsValid() bool {
	switch s {
	case StakingPeriodFlexible,
		StakingPeriodWeek,
		StakingPeriodMonth,
		StakingPeriodQuarter,
		StakingPeriodHalfYear,
		StakingPeriodYear:
		return true
	default:
		return false
	}
}

func (s StakingPeriod) GetExpirationTime() time.Time {
	switch s {
	case StakingPeriodFlexible:
		return time.Now().AddDate(0, 0, 1)
	case StakingPeriodWeek:
		return time.Now().AddDate(0, 0, 7)
	case StakingPeriodMonth:
		return time.Now().AddDate(0, 1, 0)
	case StakingPeriodQuarter:
		return time.Now().AddDate(0, 3, 0)
	case StakingPeriodHalfYear:
		return time.Now().AddDate(0, 6, 0)
	case StakingPeriodYear:
		return time.Now().AddDate(1, 0, 0)
	default:
		return time.Now()
	}
}

type StakingPeriodPayoutInterval string

const (
	StakingPeriodPayoutIntervalDaily        StakingPeriodPayoutInterval = "daily"
	StakingPeriodPayoutIntervalTwicePerWeek StakingPeriodPayoutInterval = "twice_per_week"
	StakingPeriodPayoutIntervalWeekly       StakingPeriodPayoutInterval = "weekly"
	StakingPeriodPayoutIntervalMonthly      StakingPeriodPayoutInterval = "monthly"
)

type Staking struct {
	ID             uint64                      `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserID         uint64                      `gorm:"column:user_id" json:"user_id"`
	Period         StakingPeriod               `gorm:"period" json:"period"`
	PayoutInterval StakingPeriodPayoutInterval `gorm:"payout_interval" json:"payout_interval"`
	Amount         *postgres.Decimal           `gorm:"amount" json:"amount" sql:"type:decimal(36,18)"`
	CoinSymbol     string                      `gorm:"column:coin_symbol" json:"coin_symbol"`
	Percents       *postgres.Decimal           `gorm:"percents" json:"percents" sql:"type:decimal(36,18)"`
	Status         StakingStatus               `gorm:"status" json:"status"`
	RefID          string                      `gorm:"column:ref_id" json:"-"`
	SubAccount     uint64                      `gorm:"column:sub_account" json:"sub_account"`

	CreatedAt time.Time `gorm:"created_at" json:"created_at"`
	ClosedAt  time.Time `gorm:"closed_at" json:"closed_at"`
	ExpiredAt time.Time `gorm:"expired_at" json:"expired_at"`

	//User User `gorm:"foreignkey:UserID" json:"-"`
	//Coin Coin `gorm:"foreignkey:CoinSymbol" json:"-"`
}

type StakingEarning struct {
	ID         uint64               `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	StakingID  uint64               `json:"staking_id" gorm:"column:staking_id"`
	UserID     uint64               `json:"user_id" gorm:"column:user_id"`
	Amount     *postgres.Decimal    `json:"amount" gorm:"amount" sql:"type:decimal(36,18)"`
	CoinSymbol string               `json:"coin_symbol" gorm:"column:coin_symbol"`
	SubAccount uint64               `gorm:"column:sub_account" json:"sub_account"`
	Status     StakingEarningStatus `gorm:"status"`

	CreatedAt time.Time `gorm:"created_at"`
	UpdatedAt time.Time `gorm:"updated_at"`

	//User    User    `gorm:"foreignkey:UserID" json:"-"`
	//Staking Staking `gorm:"foreignkey:StakingID" json:"-"`
	//Coin    Coin    `gorm:"foreignkey:CoinSymbol" json:"-"`
}

type StakingWithEarningsAggregation struct {
	Staking
	TotalEarnings *postgres.Decimal `gorm:"total_earnings" json:"total_earnings" sql:"type:decimal(36,18)"`
	TotalBalance  *postgres.Decimal `gorm:"total_balance" json:"total_balance" sql:"type:decimal(36,18)"`
}

func (b *Staking) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":              b.ID,
		"user_id":         b.UserID,
		"period":          b.Period,
		"payout_interval": b.PayoutInterval,
		"amount":          utils.FmtDecimal(b.Amount),
		"percents":        utils.FmtDecimal(b.Percents),
		"status":          b.Status,
		"coin_symbol":     b.CoinSymbol,
		"sub_account":     b.SubAccount,
		"created_at":      b.CreatedAt,
		"closed_at":       b.ClosedAt,
		"expired_at":      b.ExpiredAt,
	})
}

func (b *StakingWithEarningsAggregation) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":              b.ID,
		"user_id":         b.UserID,
		"period":          b.Period,
		"payout_interval": b.PayoutInterval,
		"amount":          utils.FmtDecimal(b.Amount),
		"percents":        utils.FmtDecimal(b.Percents),
		"status":          b.Status,
		"coin_symbol":     b.CoinSymbol,
		"sub_account":     b.SubAccount,
		"created_at":      b.CreatedAt,
		"closed_at":       b.ClosedAt,
		"expired_at":      b.ExpiredAt,
		"total_earnings":  utils.FmtDecimal(b.TotalEarnings),
		"total_balance":   utils.FmtDecimal(b.TotalEarnings),
	})
}

func (b *StakingEarning) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":          b.ID,
		"staking_id":  b.StakingID,
		"user_id":     b.UserID,
		"amount":      utils.FmtDecimal(b.Amount),
		"coin_symbol": b.CoinSymbol,
		"created_at":  b.CreatedAt,
	})
}

type CreateStakingRequest struct {
	Period     string       `json:"period"`
	Amount     *decimal.Big `json:"amount"`
	CoinSymbol string       `json:"coin_symbol"`
}
