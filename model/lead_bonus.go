package model

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"time"
)

type LeadBonusStatus string
type LeadBonusCampaignStatus string
type LeadBonusCampaignRewardsType string

const (
	LeadBonusStatus_Payed   LeadBonusStatus = "payed"
	LeadBonusStatus_Pending LeadBonusStatus = "pending"

	LeadBonusCampaignStatus_Active   LeadBonusCampaignStatus = "active"
	LeadBonusCampaignStatus_Disabled LeadBonusCampaignStatus = "disabled"
	LeadBonusCampaignStatus_Expired  LeadBonusCampaignStatus = "expired"

	LeadBonusCampaignRewardsType_Instant   LeadBonusCampaignRewardsType = "instant"
	LeadBonusCampaignRewardsType_WithDelay LeadBonusCampaignRewardsType = "with_delay"
)

type LeadBonus struct {
	ID             uint64            `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserID         uint64            `gorm:"column:user_id"`
	MarketingID    string            `gorm:"marketing_id"`
	LeadFromSource string            `gorm:"lead_from_source"`
	Amount         *postgres.Decimal `gorm:"amount" sql:"type:decimal(36,18)"`
	CoinSymbol     string            `gorm:"column:coin_symbol"`
	RefID          string            `gorm:"column:ref_id" json:"-"`
	SubAccount     uint64            `gorm:"column:sub_account" json:"sub_account"`
	Comment        string            `gorm:"column:comment" json:"comment"`
	CreatedAt      time.Time         `gorm:"created_at"`
	UpdatedAt      time.Time         `gorm:"updated_at"`
	Status         LeadBonusStatus   `gorm:"status"`
	RewardsTime    time.Time         `gorm:"rewards_time"`
}

type LeadBonusCampaign struct {
	ID                         uint64                       `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserID                     uint64                       `gorm:"column:user_id"`
	MarketingID                string                       `gorm:"marketing_id"`
	LeadFromSource             string                       `gorm:"lead_from_source"`
	Amount                     *postgres.Decimal            `gorm:"amount" sql:"type:decimal(36,18)"`
	CoinSymbol                 string                       `gorm:"column:coin_symbol"`
	PeriodStart                time.Time                    `gorm:"period_start" json:"period_start"`
	PeriodEnd                  time.Time                    `gorm:"period_end" json:"period_end"`
	RewardsType                LeadBonusCampaignRewardsType `gorm:"rewards_type" json:"rewards_type"`
	RewardsDelay               int                          `gorm:"rewards_delay" json:"rewards_delay"`
	AllowRewardAfterExpiration bool                         `gorm:"allow_reward_after_expiration" json:"allow_reward_after_expiration"`
	Comment                    string                       `gorm:"column:comment" json:"comment"`
	CreatedAt                  time.Time                    `gorm:"created_at"`
	UpdatedAt                  time.Time                    `gorm:"updated_at"`
	Status                     LeadBonusCampaignStatus      `gorm:"status"`
}

func (lbc LeadBonusCampaign) IsActive() bool {
	now := time.Now()
	return lbc.Status == LeadBonusCampaignStatus_Active &&
		now.After(lbc.PeriodStart) &&
		now.Before(lbc.PeriodEnd)
}
