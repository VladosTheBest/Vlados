package model

import (
	"github.com/ericlagergren/decimal/sql/postgres"
	"time"
)

type ReferralEarningsType string

const (
	ReferralEarningsType_Order         ReferralEarningsType = "order"
	ReferralEarningsType_BonusContract ReferralEarningsType = "bonus_contract"
)

type ReferralEarning struct {
	Id                uint64               `gorm:"column:id" wire:"id"`
	RefId             string               `gorm:"column:ref_id" wire:"ref_id"`
	ReferralId        uint64               `gorm:"column:referral_id" wire:"referral_id"`
	UserId            uint64               `gorm:"column:user_id" wire:"user_id"`
	RelatedObjectId   uint64               `gorm:"column:related_object_id" wire:"related_object_id"`
	RelatedObjectType ReferralEarningsType `gorm:"column:related_object_type" wire:"related_object_type"`
	Level             ReferralEarningLevel `gorm:"column:level" wire:"level"`
	Type              OperationType        `gorm:"column:type" wire:"type"`
	Amount            *postgres.Decimal    `gorm:"column:amount" sql:"type:decimal(36,18)" wire:"amount"`
	CoinSymbol        string               `gorm:"column:coin_symbol" wire:"coin_symbol"`
	CreatedAt         time.Time            `gorm:"column:created_at" wire:"created_at"`
}

type ReferralEarningsResponse struct {
	Data []ReferralEarningsResponseData
	Meta PagingMeta
}

type ReferralEarningsResponseData struct {
	Email        string      `json:"email" gorm:"column:email"`
	RegisterDate time.Time   `json:"register_date" gorm:"column:register_date"`
	L1Earnings   JSONDecimal `json:"l_1_earnings" gorm:"column:l1_earnings" sql:"type:decimal(36,18)"`
	L2Earnings   JSONDecimal `json:"l_2_earnings" gorm:"column:l2_earnings" sql:"type:decimal(36,18)"`
	L3Earnings   JSONDecimal `json:"l_3_earnings" gorm:"column:l3_earnings" sql:"type:decimal(36,18)"`
	L2Count      int         `json:"l_2_count" gorm:"column:l2_users"`
	L3Count      int         `json:"l_3_count" gorm:"column:l3_users"`
}

type ReferralEarningLevel int

func (r ReferralEarningLevel) Int() int {
	return int(r)
}

const (
	ReferralEarningLevel1 ReferralEarningLevel = 1
	ReferralEarningLevel2 ReferralEarningLevel = 2
	ReferralEarningLevel3 ReferralEarningLevel = 3
)

type ReferralTree struct {
	L1 []uint64 `json:"L1"`
	L2 []uint64 `json:"L2"`
	L3 []uint64 `json:"L3"`
}

type ReferralAllTree struct {
	UserId uint64   `json:"userId"`
	L1     []uint64 `json:"L1"`
	L2     []uint64 `json:"L2"`
	L3     []uint64 `json:"L3"`
}

type ReferralAllTreeList struct {
	ReferralsList []ReferralAllTree
}
