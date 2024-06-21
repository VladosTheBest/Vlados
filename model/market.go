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
	"errors"
	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"time"
)

type MarketStatus string

const (
	MarketStatusActive   MarketStatus = "active"
	MarketStatusInactive MarketStatus = "inactive"
	MarketStatusDisabled MarketStatus = "disabled"
)

//
//func (u *MarketStatus) Scan(value interface{}) error { *u = MarketStatus(value.([]byte)); return nil }
//func (u MarketStatus) Value() (driver.Value, error)  { return string(u), nil }

// Market structure
type Market struct {
	ID                    string            `gorm:"type:varchar(10);PRIMARY_KEY;UNIQUE;NOT NULL;" json:"id"`
	Name                  string            `json:"name"`
	Status                MarketStatus      `sql:"not null;type:market_status_t;default:'active'" json:"status"`
	MarketPrecision       int               `json:"market_precision"`
	QuotePrecision        int               `json:"quote_precision"`
	MarketPrecisionFormat int               `json:"market_precision_format"`
	QuotePrecisionFormat  int               `json:"quote_precision_format"`
	MinMarketVolume       *postgres.Decimal `sql:"type:decimal(36,18)" json:"min_market_volume"`
	MinQuoteVolume        *postgres.Decimal `sql:"type:decimal(36,18)" json:"min_quote_volume"`
	MaxMarketPrice        *postgres.Decimal `sql:"type:decimal(36,18)" json:"max_market_price"`
	MaxQuotePrice         *postgres.Decimal `sql:"type:decimal(36,18)" json:"max_quote_price"`
	MaxUSDTSpendLimit     *postgres.Decimal `sql:"type:decimal(36,18)" json:"max_usdt_spend_limit"`
	MarketCoin            Coin              `gorm:"foreignkey:MarketCoinSymbol"`
	MarketCoinSymbol      string            `gorm:"column:market_coin_symbol" json:"market_coin_symbol"`
	QuoteCoin             Coin              `gorm:"foreignkey:QuoteCoinSymbol"`
	QuoteCoinSymbol       string            `gorm:"column:quote_coin_symbol" json:"quote_coin_symbol"`
	Highlight             bool              `sql:"type:bool" json:"highlight"`
	minMarketVolume       *decimal.Big
	minQuoteVolume        *decimal.Big
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (m *Market) SetCross(minMarket, minQuote *decimal.Big) {
	m.minMarketVolume = minMarket
	m.minQuoteVolume = minQuote
}

func (m *Market) GetCrossMinMarket() (*decimal.Big, error) {
	if m.minMarketVolume == nil || conv.NewDecimalWithPrecision().CheckNaNs(m.minMarketVolume, nil) {
		return m.MinMarketVolume.V, nil
		// return nil, errors.New("min market volume not setted")
	}

	return m.minMarketVolume, nil
}

func (m *Market) GetCrossMinQuote() (*decimal.Big, error) {
	if m.minQuoteVolume == nil || conv.NewDecimalWithPrecision().CheckNaNs(m.minQuoteVolume, nil) {
		return m.MinQuoteVolume.V, nil
		// return nil, errors.New("min quote volume not setted")
	}
	return m.minQuoteVolume, nil
}

// Market CoinGecko structure
type MarketCGK struct {
	Ticker string `json:"ticker"`
	Base   string `json:"base"`
	Target string `json:"target"`
}

// MarketList
type MarketList struct {
	Markets []Market
	Meta    PagingMeta
}

// NewMarket creates a new market
func NewMarket(id, name, marketCoinSymbol, quoteCoinSymbol string, status MarketStatus, mPrec, qPrec, mPrecFormat, qPrecFormat int, minMVol, minQVol, maxMPrice, maxQPrice, maxUSDTSpendLimit *decimal.Big) *Market {
	return &Market{
		ID:                    id,
		Name:                  name,
		MarketCoinSymbol:      marketCoinSymbol,
		QuoteCoinSymbol:       quoteCoinSymbol,
		MarketPrecision:       mPrec,
		QuotePrecision:        qPrec,
		MarketPrecisionFormat: mPrecFormat,
		QuotePrecisionFormat:  qPrecFormat,
		MinMarketVolume:       &postgres.Decimal{V: minMVol},
		MinQuoteVolume:        &postgres.Decimal{V: minQVol},
		MaxMarketPrice:        &postgres.Decimal{V: maxMPrice},
		MaxQuotePrice:         &postgres.Decimal{V: maxQPrice},
		MaxUSDTSpendLimit:     &postgres.Decimal{V: maxUSDTSpendLimit},
		Status:                status,
	}
}

func GetMarketStatusFromString(s string) (MarketStatus, error) {
	switch s {
	case "active":
		return MarketStatusActive, nil
	case "inactive":
		return MarketStatusInactive, nil
	case "disabled":
		return MarketStatusDisabled, nil
	default:
		return MarketStatusDisabled, errors.New("Status is not valid")
	}
}

// Model Methods
func (market *Market) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":                      market.ID,
		"name":                    market.Name,
		"status":                  market.Status,
		"market_coin_symbol":      market.MarketCoinSymbol,
		"quote_coin_symbol":       market.QuoteCoinSymbol,
		"market_precision":        market.MarketPrecision,
		"quote_precision":         market.QuotePrecision,
		"market_precision_format": market.MarketPrecisionFormat,
		"quote_precision_format":  market.QuotePrecisionFormat,
		"min_market_volume":       utils.Fmt(market.MinMarketVolume.V),
		"min_quote_volume":        utils.Fmt(market.MinQuoteVolume.V),
		"max_market_price":        utils.Fmt(market.MaxMarketPrice.V),
		"max_quote_price":         utils.Fmt(market.MaxQuotePrice.V),
		"max_usdt_spend_limit":    utils.Fmt(market.MaxUSDTSpendLimit.V),
		"highlight":               market.Highlight,
	})
}

// GORM Event Handlers
