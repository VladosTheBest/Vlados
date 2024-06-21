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
	"github.com/ericlagergren/decimal"
	"time"
)

const MIN_MARKET_PRICE_FEATURE_KEY = "min_market_price"
const MAX_MARKET_PRICE_FEATURE_KEY = "max_market_price"
const MANUAL_DISTRIBUTION_PERCENT_FEATURE_KEY = "manual_distribution_percent"

// AdminFeatureSettings structure
type AdminFeatureSettings struct {
	ID        uint64    `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"-"`
	Feature   string    `json:"feature"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// WithdrawLimit structure
// @virtual
type WithdrawLimit struct {
	Feature string `json:"feature"`
	Value   string `json:"value"`
}

type PriceLimitResponse struct {
	MinPrice *decimal.Big `json:"min_price"`
	MaxPrice *decimal.Big `json:"max_price"`
}

func NewAdminFeatureSettings(feature string, value string) *AdminFeatureSettings {
	return &AdminFeatureSettings{
		Feature: feature,
		Value:   value,
	}
}
