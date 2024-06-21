package model

/*
 * Copyright © 2018-2019 Around25 SRL <office@around25.com>
 *
 * Licensed under the Around25 Wallet License Agreement (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.around25.com/licenses/EXCHANGE_LICENSE
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Cosmin Harangus <cosmin@around25.com>
 * @copyright 2018-2019 Around25 SRL <office@around25.com>
 * @license 	EXCHANGE_LICENSE
 */

import (
	"encoding/json"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/config"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// UserFeeStatus godoc
type UserFeeStatus string

const (
	// UserFeeStatusActive active
	UserFeeStatusActive UserFeeStatus = "active"
	// UserFeeStatusChanged changed
	UserFeeStatusChanged UserFeeStatus = "changed"
)

// UserFeeLevel godoc
type UserFeeLevel string

const (
	// UserFeeLevelNone none
	UserFeeLevelNone UserFeeLevel = "none"
	// UserFeeLevelSilver silver
	UserFeeLevelSilver UserFeeLevel = "silver"
	// UserFeeLevelGold gold
	UserFeeLevelGold UserFeeLevel = "gold"
	// UserFeeLevelPlatinum platinum
	UserFeeLevelPlatinum UserFeeLevel = "platinum"
	// UserFeeLevelBlack black
	UserFeeLevelBlack UserFeeLevel = "black"
)

func (u UserFeeLevel) IsValidUserLevel() bool {
	switch u {
	case UserFeeLevelNone,
		UserFeeLevelSilver,
		UserFeeLevelGold,
		UserFeeLevelPlatinum,
		UserFeeLevelBlack:
		return true
	default:
		return false
	}
}

// UserFee structure
type UserFee struct {
	ID     uint64 `sql:"type:bigint" gorm:"primary_key" json:"id"`
	UserID uint64 `sql:"type:bigint REFERENCES users(id)" json:"user_id"`

	// The admin can change the default maker/taker fees for a user based on which the discounts are calculated
	DefaultMakerFee *postgres.Decimal `sql:"type:decimal(36,18)" json:"default_maker_fee"`
	DefaultTakerFee *postgres.Decimal `sql:"type:decimal(36,18)" json:"default_taker_fee"`

	// this field should be read only and generated by the account summary service based on current discount level
	MakerFee *postgres.Decimal `sql:"type:decimal(36,18)" json:"maker_fee"`
	// this field should be read only and generated by the account summary service based on current discount level
	TakerFee *postgres.Decimal `sql:"type:decimal(36,18)" json:"taker_fee"`

	CurrentDiscount *postgres.Decimal `sql:"type:decimal(36,18)" json:"current_discount"`

	// Discountable godoc
	Discountable bool `json:"discountable"`

	Status        UserFeeStatus `json:"status"`
	DiscountLevel UserFeeLevel  `json:"discount_level"`
	Level         UserFeeLevel  `json:"level"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// The admin can change the default maker/taker fees for a user based on which the discounts are calculated
	DefaultMakerFeeForBonus *postgres.Decimal `sql:"type:decimal(36,18)" json:"default_maker_fee_for_bonus"`
	DefaultTakerFeeForBonus *postgres.Decimal `sql:"type:decimal(36,18)" json:"default_taker_fee_for_bonus"`

	// this field should be read only and generated by the account summary service based on current discount level
	MakerFeeForBonus *postgres.Decimal `sql:"type:decimal(36,18)" json:"maker_fee_for_bonus"`
	// this field should be read only and generated by the account summary service based on current discount level
	TakerFeeForBonus *postgres.Decimal `sql:"type:decimal(36,18)" json:"taker_fee_for_bonus"`
}

type PRDXLines struct {
	None      *postgres.Decimal `sql:"type:decimal(36,18)" gorm:"none"     json:"none"`
	Silver    *postgres.Decimal `sql:"type:decimal(36,18)" gorm:"silver"   json:"silver"`
	Gold      *postgres.Decimal `sql:"type:bigint"         gorm:"gold"     json:"gold"`
	Platinum  *postgres.Decimal `sql:"type:decimal(36,18)" gorm:"platinum" json:"platinum"`
	Black     *postgres.Decimal `sql:"type:decimal(36,18)" gorm:"black"    json:"black"`
	Precision int               `json:"-"`
}

func (prdxl *PRDXLines) MarshalJSON() ([]byte, error) {

	if prdxl.Precision == 0 {
		prdxl.Precision = 8
	}

	return json.Marshal(map[string]interface{}{
		"silver":   utils.Fmt(prdxl.Silver.V.Quantize(prdxl.Precision)),
		"gold":     utils.Fmt(prdxl.Gold.V.Quantize(prdxl.Precision)),
		"platinum": utils.Fmt(prdxl.Platinum.V.Quantize(prdxl.Precision)),
		"black":    utils.Fmt(prdxl.Black.V.Quantize(prdxl.Precision)),
		"none":     utils.Fmt(prdxl.None.V.Quantize(prdxl.Precision)),
	})
}

func NewPRDXLines(precision int) *PRDXLines {

	p := &PRDXLines{
		Silver:    &postgres.Decimal{V: Zero},
		Gold:      &postgres.Decimal{V: Zero},
		Platinum:  &postgres.Decimal{V: Zero},
		Black:     &postgres.Decimal{V: Zero},
		None:      &postgres.Decimal{V: Zero},
		Precision: precision,
	}

	return p
}

// NewUserFee godoc
func NewUserFee(userID uint64, cfg config.FeeConfig) *UserFee {
	return &UserFee{
		UserID: userID,
		// general
		DefaultMakerFee: &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.General.Maker)},
		DefaultTakerFee: &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.General.Taker)},
		MakerFee:        &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.General.Maker)},
		TakerFee:        &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.General.Taker)},
		// bonus account
		DefaultMakerFeeForBonus: &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.BonusAccount.Maker)},
		DefaultTakerFeeForBonus: &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.BonusAccount.Taker)},
		MakerFeeForBonus:        &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.BonusAccount.Maker)},
		TakerFeeForBonus:        &postgres.Decimal{V: conv.NewDecimalWithPrecision().SetFloat64(cfg.BonusAccount.Taker)},
		CurrentDiscount:         &postgres.Decimal{V: &decimal.Big{}},
		Status:                  UserFeeStatusActive,
		Discountable:            true,
		DiscountLevel:           UserFeeLevelNone,
		Level:                   UserFeeLevelNone,
	}
}

func (fee UserFee) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                          fee.ID,
		"user_id":                     fee.UserID,
		"default_maker_fee":           utils.Fmt(fee.DefaultMakerFee.V),
		"default_taker_fee":           utils.Fmt(fee.DefaultTakerFee.V),
		"maker_fee":                   utils.Fmt(fee.MakerFee.V),
		"taker_fee":                   utils.Fmt(fee.TakerFee.V),
		"default_maker_fee_for_bonus": utils.Fmt(fee.DefaultMakerFeeForBonus.V),
		"default_taker_fee_for_bonus": utils.Fmt(fee.DefaultTakerFeeForBonus.V),
		"maker_fee_for_bonus":         utils.Fmt(fee.MakerFeeForBonus.V),
		"taker_fee_for_bonus":         utils.Fmt(fee.TakerFeeForBonus.V),
		"current_discount":            utils.Fmt(fee.CurrentDiscount.V),
		"discountable":                fee.Discountable,
		"level":                       fee.Level,
		"discount_level":              fee.DiscountLevel,
		"status":                      fee.Status,
	})
}