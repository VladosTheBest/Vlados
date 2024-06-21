package model

import (
	"strconv"
	"time"
)

type MarketType string
type SubAccountStatus string

const (
	MarketTypeSpot   MarketType = "spot"
	MarketTypeMargin MarketType = "margin"

	SubAccountStatusActive SubAccountStatus = "active"
	SubAccountStatusClosed SubAccountStatus = "closed"
)

type SubAccount struct {
	ID                uint64           `json:"ID" sql:"type: bigint" gorm:"primary_key"`
	UserId            uint64           `json:"userId" sql:"type: bigint"`
	AccountGroup      AccountGroup     `json:"account_group"`
	MarketType        MarketType       `json:"market_type"`
	DepositAllowed    bool             `json:"deposit_allowed"`
	WithdrawalAllowed bool             `json:"withdrawal_allowed"`
	TransferAllowed   bool             `json:"transfer_allowed"`
	IsDefault         bool             `json:"is_default"`
	IsMain            bool             `json:"is_main"`
	Title             string           `json:"title" gorm:"type:varchar(128);"`
	Comment           string           `json:"comment" gorm:"type:varchar(256);"`
	Status            SubAccountStatus `json:"status"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (s *SubAccount) GetIDString() string {
	return strconv.FormatUint(s.ID, 10)
}

const (
	SubAccountDefaultMain uint64 = 0
)
