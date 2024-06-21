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
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// CoinStatus contains the list of available statuses for the coin
// swagger:model CoinStatus
// example: active
// enum: active,inactive,presale
type CoinStatus string

const (
	CoinStatusActive   CoinStatus = "active"
	CoinStatusInactive CoinStatus = "inactive"
	CoinStatusPresale  CoinStatus = "presale"
)

// CoinType represents the type of coin
// swagger:model CoinType
// example: crypto
// enum: fiat,crypto,asset
type CoinType string

const (
	CoinTypeFiat   CoinType = "fiat"
	CoinTypeCrypto CoinType = "crypto"
	CoinTypeAsset  CoinType = "asset"
)

func (c CoinType) IsValid() bool {
	switch c {
	case CoinTypeFiat,
		CoinTypeCrypto,
		CoinTypeAsset:
		return true
	default:
		return false
	}
}

// Coin represents a supported currency
//
// The coin is a currency that can be used to deposit and withdraw funds with.
// Each coin has a unique symbol and can either be a blockchain coin/token or fiat.
//
// swagger:model Coin
type Coin struct {
	// A unique identifier
	//
	// required: true
	// example: eth
	// min length: 3
	Symbol string `gorm:"type:varchar(10);PRIMARY_KEY;UNIQUE;NOT NULL;" json:"symbol"`
	// The name of the coin
	//
	// required: true
	// example: Ethereum
	Name string `json:"name"`
	// The type of the coin
	//
	// required: true
	// example: crypto
	Type CoinType `sql:"not null;type:coin_type_t;default:'crypto'" json:"type"`
	// The status
	//
	// required: true
	// example: active
	Status CoinStatus `sql:"not null;type:coin_status_t;default:'active'" json:"status"`
	// Number of digits after decimal point to support for amounts received/displayed
	//
	// required: true
	// example: 5
	Digits int `json:"digits"`
	// Number of digits after decimal point suported by the chain
	//
	// required: true
	// example: 18
	TokenPrecision int `json:"token_precision"`
	// Minimum Deposit Confirmations
	//
	// required: true
	// example: 2
	MinConfirmations int `json:"min_confirmations"`
	// Minimum Withdraw amount
	//
	// required: true
	// example: 0.005
	MinWithdraw *postgres.Decimal `sql:"type:decimal(36,18)" json:"min_withdraw"`
	// Withdraw fee
	//
	// required: true
	// example: 0.001
	WithdrawFee *postgres.Decimal `sql:"type:decimal(36,18)" json:"withdraw_fee"`
	// Deposit fee
	//
	// required: true
	// example: 0.000
	DepositFee *postgres.Decimal `sql:"type:decimal(36,18)" json:"deposit_fee"`
	// The ETH address of the contract
	//
	// required: false
	// example: ""
	ContractAddress string `gorm:"column:contract_address" json:"contract_address"`
	// The coin in which to calculate the transfer fees
	//
	// required: true
	// example: "eth"
	CostSymbol string `json:"cost_symbol"`
	// The id of the chain
	//
	// required: true
	// example: "eth"
	ChainSymbol string `gorm:"column:chain_symbol" json:"chain_symbol"`
	Chain       Chain  `json:"-"`
	// A URL to use a a block explorer for the coin
	//
	// required: false
	BlockchainExplorer string `gorm:"column:blockchain_explorer" json:"blockchain_explorer"`

	WithdrawFeeAdvCash       *postgres.Decimal `sql:"type:decimal(36,18)" json:"withdraw_fee_adv_cash"`
	WithdrawFeeClearJunction *postgres.Decimal `sql:"type:decimal(36,18)" json:"withdraw_fee_clear_junction"`

	BlockDeposit  bool `sql:"type:bool" json:"block_deposit"`
	BlockWithdraw bool `sql:"type:bool" json:"block_withdraw"`

	Highlight  bool `sql:"type:bool" json:"highlight"`
	NewListing bool `sql:"type:bool" json:"new_listing"`

	// Should calculate cross-curs
	//
	// required: true
	ShouldGetValue bool `gorm:"column:should_get_value" json:"should_get_value"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (coin *Coin) IsCoinStatusPresale() bool {
	return coin.Status == CoinStatusPresale
}

// NewCoin creates a new Coin
func NewCoin(
	coinType CoinType,
	chainSymbol,
	symbol, name string,
	digits, precision int,
	minWithdraw, withdrawFee, depositFee *decimal.Big,
	contractAddress string,
	status CoinStatus,
	costSymbol, blockchainExplorer string,
	minConfirmations int,
	shouldGetValue bool,
	withdrawFeeAdvCash, withdrawFeeClearJunction *decimal.Big,
) *Coin {
	return &Coin{
		Type:                     coinType,
		ChainSymbol:              chainSymbol,
		Symbol:                   symbol,
		Name:                     name,
		Digits:                   digits,
		TokenPrecision:           precision,
		MinConfirmations:         minConfirmations,
		MinWithdraw:              &postgres.Decimal{V: minWithdraw},
		WithdrawFee:              &postgres.Decimal{V: withdrawFee},
		DepositFee:               &postgres.Decimal{V: depositFee},
		WithdrawFeeAdvCash:       &postgres.Decimal{V: withdrawFeeAdvCash},
		WithdrawFeeClearJunction: &postgres.Decimal{V: withdrawFeeClearJunction},
		ContractAddress:          contractAddress,
		Status:                   status,
		CostSymbol:               costSymbol,
		BlockchainExplorer:       blockchainExplorer,
		ShouldGetValue:           shouldGetValue,
	}
}

func GetCoinStatusFromString(s string) (CoinStatus, error) {
	switch s {
	case "active":
		return CoinStatusActive, nil
	case "inactive":
		return CoinStatusInactive, nil
	case "presale":
		return CoinStatusPresale, nil
	default:
		return CoinStatusInactive, errors.New("Status is not valid")
	}
}

func GetCoinTypeFromString(s string) (CoinType, error) {
	switch s {
	case "fiat":
		return CoinTypeFiat, nil
	case "crypto":
		return CoinTypeCrypto, nil
	case "asset":
		return CoinTypeAsset, nil
	default:
		return CoinTypeCrypto, errors.New("Coin type is not valid")
	}
}

// Model Methods
func (coin *Coin) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"symbol":                      coin.Symbol,
		"name":                        coin.Name,
		"type":                        coin.Type,
		"status":                      coin.Status,
		"digits":                      coin.Digits,
		"token_precision":             coin.TokenPrecision,
		"min_confirmations":           coin.MinConfirmations,
		"min_withdraw":                utils.FmtDecimalWithPrecision(coin.MinWithdraw, coin.TokenPrecision),
		"withdraw_fee":                utils.FmtDecimalWithPrecision(coin.WithdrawFee, coin.TokenPrecision),
		"withdraw_fee_adv_cash":       utils.FmtDecimalWithPrecision(coin.WithdrawFeeAdvCash, coin.TokenPrecision),
		"withdraw_fee_clear_junction": utils.FmtDecimalWithPrecision(coin.WithdrawFeeClearJunction, coin.TokenPrecision),
		"deposit_fee":                 utils.FmtDecimalWithPrecision(coin.DepositFee, coin.TokenPrecision),
		"contract_address":            coin.ContractAddress,
		"cost_symbol":                 coin.CostSymbol,
		"blockchain_explorer":         coin.BlockchainExplorer,
		"block_deposit":               coin.BlockDeposit,
		"block_withdraw":              coin.BlockWithdraw,
		"chain_symbol":                coin.ChainSymbol,
		"highlight":                   coin.Highlight,
		"new_listing":                 coin.NewListing,
	})
}

// GORM Event Handlers
