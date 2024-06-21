package model

import (
	"encoding/json"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

type BonusAccountContractStatus string

const (
	BonusAccountContractStatusInactive          BonusAccountContractStatus = "inactive"
	BonusAccountContractStatusActive            BonusAccountContractStatus = "active"
	BonusAccountContractStatusPendingExpiration BonusAccountContractStatus = "pending_expiration"
	BonusAccountContractStatusExpired           BonusAccountContractStatus = "expired"
	BonusAccountContractStatusPayed             BonusAccountContractStatus = "payed"
)

func GetBonusAccountContractStatusFromString(status string) BonusAccountContractStatus {
	switch status {
	case "inactive":
		return BonusAccountContractStatusInactive
	case "active":
		return BonusAccountContractStatusActive
	case "pending_expiration":
		return BonusAccountContractStatusPendingExpiration
	case "expired":
		return BonusAccountContractStatusExpired
	case "payed":
		return BonusAccountContractStatusPayed
	default:
		return ""
	}
}

type BonusAccountContract struct {
	ID            uint64                     `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	UserID        uint64                     `gorm:"column:user_id"`
	Term          int64                      `gorm:"term" json:"term"`
	Amount        *postgres.Decimal          `gorm:"amount" sql:"type:decimal(36,18)"`
	CoinSymbol    string                     `gorm:"column:coin_symbol"`
	BonusPercents *postgres.Decimal          `gorm:"bonus_percents" sql:"type:decimal(36,18)"`
	VolumeToTrade *postgres.Decimal          `gorm:"volume_to_trade" sql:"type:decimal(36,18)"`
	CreatedAt     time.Time                  `gorm:"created_at"`
	ClosedAt      time.Time                  `gorm:"closed_at"`
	ExpiredAt     time.Time                  `gorm:"expired_at"`
	Status        BonusAccountContractStatus `gorm:"status"`
	RefID         string                     `gorm:"column:ref_id" json:"-"`
	SubAccount    uint64                     `gorm:"column:sub_account" json:"sub_account"`

	User User `gorm:"foreignkey:UserID" json:"-"`
	Coin Coin `gorm:"foreignkey:CoinSymbol" json:"-"`
}

type BonusAccountContractBots struct {
	ID         uint64    `sql:"type:uuid" gorm:"PRIMARY_KEY"`
	BotID      uint64    `gorm:"bot_id"`
	ContractID uint64    `gorm:"contract_id"`
	Active     bool      `gorm:"active"`
	CreatedAt  time.Time `gorm:"created_at"`
	UpdatedAt  time.Time `gorm:"updated_at"`
}

func (b *BonusAccountContract) GetFullBonusAmount() *decimal.Big {
	return conv.NewDecimalWithPrecision().Mul(b.Amount.V, b.BonusPercents.V)
}

func (b *BonusAccountContract) GetTotalAmount() *decimal.Big {
	return conv.NewDecimalWithPrecision().Add(b.Amount.V, b.GetFullBonusAmount())
}

type BonusAccountContractView struct {
	ID            uint64                     `sql:"type:uuid" gorm:"PRIMARY_KEY" json:"id"`
	UserID        uint64                     `gorm:"column:user_id" json:"user_id"`
	Term          int64                      `gorm:"term" json:"term"`
	Amount        JSONDecimal                `gorm:"amount" sql:"type:decimal(36,18)" json:"amount"`
	CoinSymbol    string                     `gorm:"column:coin_symbol" json:"coin_symbol"`
	BonusPercents JSONDecimal                `gorm:"bonus_percents" sql:"type:decimal(36,18)" json:"bonus_percents"`
	VolumeToTrade JSONDecimal                `gorm:"volume_to_trade" sql:"type:decimal(36,18)" json:"volume_to_trade"`
	VolumeTraded  JSONDecimal                `gorm:"volume_traded" sql:"type:decimal(36,18)" json:"volume_traded"`
	CreatedAt     time.Time                  `gorm:"created_at" json:"created_at"`
	ClosedAt      time.Time                  `gorm:"closed_at" json:"closed_at"`
	ExpiredAt     time.Time                  `gorm:"expired_at" json:"expired_at"`
	Status        BonusAccountContractStatus `gorm:"status" json:"status"`
	RefID         string                     `gorm:"column:ref_id" json:"-"`
	BotID         uint64                     `gorm:"column:bot_id" json:"bot_id"`
}

type BonusAccountContractViewWithPair struct {
	BonusAccountContractView
	Pair       string `gorm:"pair" json:"pair"`
	SubAccount uint64 `gorm:"sub_account" json:"sub_account"`
}

type BonusAccountContractViewWithProfitLoss struct {
	BonusAccountContractView
	ProfitLoss *decimal.Big
}

type BonusAccountContractHistory struct {
	BonusAccountContractViewWithPair
	ProfitLoss *decimal.Big
}

type BonusAccountContractViewWithBonusContract struct {
	BonusAccountContractView `json:"bonus-account-contract-view"`
	ContractID               uint64 `gorm:"contract_id" json:"contract_id"`
}

func (b *BonusAccountContractView) GetFullBonusAmount() *decimal.Big {
	return conv.NewDecimalWithPrecision().Mul(b.Amount.V, b.BonusPercents.V)
}

func (b *BonusAccountContractView) GetTotalAmount() *decimal.Big {
	return conv.NewDecimalWithPrecision().Add(b.Amount.V, b.GetFullBonusAmount())
}

func (b *BonusAccountContractView) GetCurrentBonusAmount() *decimal.Big {

	currentPercent := conv.NewDecimalWithPrecision().Quo(b.VolumeTraded.V, b.VolumeToTrade.V)

	if currentPercent.Cmp(Zero) < 1 {
		return Zero
	}

	step := conv.NewDecimalWithPrecision().SetFloat64(.25)
	coefficient := conv.NewDecimalWithPrecision().QuoInt(currentPercent, step)

	if coefficient.Cmp(conv.NewDecimalWithPrecision().SetFloat64(4)) == 1 {
		coefficient.SetFloat64(4)
	}

	roundedPercentage := conv.NewDecimalWithPrecision().Mul(coefficient, step)

	return conv.NewDecimalWithPrecision().Mul(b.GetFullBonusAmount(), roundedPercentage)
}

type JSONDecimal struct {
	postgres.Decimal
}

func (b JSONDecimal) MarshalJSON() ([]byte, error) {
	out := utils.FmtDecimal(&b.Decimal)
	return json.Marshal(out)
}

type DepositBonusContractRequest struct {
	CreateBotSeparateRequest
	Period int64 `json:"period" form:"period" binding:"required"`
}

type CreateBotSeparateRequest struct {
	Amount      *decimal.Big `json:"amount" form:"amount" binding:"required"`
	CoinSymbol  string       `json:"coin" form:"coin" binding:"required"`
	BotType     BotType      `json:"bot_type" form:"bot_type"`
	BotSettings string       `json:"bot_settings" form:"bot_settings"`
}

type BonusAccountVolume struct {
	ID          uint64            `sql:"type:uuid" gorm:"PRIMARY_KEY" json:"id"`
	ContractId  uint64            `sql:"contract_id" json:"contract_id"`
	OrderId     uint64            `sql:"order_id" json:"order_id"`
	TradeId     uint64            `sql:"trade_id" json:"trade_id"`
	UserID      uint64            `gorm:"column:user_id" json:"user_id"`
	Side        MarketSide        `gorm:"column:side" json:"side"`
	CoinSymbol  string            `gorm:"column:coin_symbol" json:"coin_symbol"`
	Amount      *postgres.Decimal `gorm:"amount" sql:"type:decimal(36,18)" json:"amount"`
	AmountCross *postgres.Decimal `gorm:"amount_cross" sql:"type:decimal(36,18)" json:"amount_cross"`
	CreatedAt   time.Time         `gorm:"created_at" json:"created_at"`
	UpdatedAt   time.Time         `gorm:"updated_at" json:"updated_at"`
}

type ContractsInfo struct {
	Coin       string       `json:"coin"`
	Contracts  float64      `json:"contracts"`
	Invested   *decimal.Big `json:"invested"`
	BonusPayed *decimal.Big `json:"bonus_payed"`
}
