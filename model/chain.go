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
)

// ChainStatus cointains the list of available statuses for the Chain
// swagger:model ChainStatus
// example: pending
// enum: pending,active,inactive
type ChainStatus string

const (
	ChainStatusPending  ChainStatus = "pending"
	ChainStatusActive   ChainStatus = "active"
	ChainStatusInactive ChainStatus = "inactive"
)

// Chain represents a source of transactions in the exchange
//
// The chain is used internally to know where a coin should be sent to
// or where it can be looked for in case of a deposit.
//
// swagger:model Chain
type Chain struct {
	// A symbol that uniquely identifies the chain
	//
	// required: true
	// example: eth
	// min length: 3
	Symbol string `gorm:"type:varchar(10);PRIMARY_KEY;UNIQUE;NOT NULL;" json:"symbol" example:"eth"`
	// The name of the chain
	//
	// required: true
	// example: Ethereum
	// min length: 3
	Name string `gorm:"column:name" json:"name" example:"Ethereum"`
	// The current status of the chain
	//
	// required: true
	Status ChainStatus `sql:"not null;type:chain_status_t;default:'pending'" json:"status" example:"pending"`
}

type UnrestrictedChain Chain

// NewChain creates a new chain
func NewChain(symbol, name string, status ChainStatus) *Chain {
	return &Chain{Symbol: symbol, Name: name, Status: status}
}

func GetStatusFromString(s string) (ChainStatus, error) {
	switch s {
	case "pending":
		return ChainStatusPending, nil
	case "active":
		return ChainStatusActive, nil
	case "inactive":
		return ChainStatusActive, nil
	default:
		return ChainStatusPending, errors.New("Status is not valid")
	}
}

// Model Methods
func (chain UnrestrictedChain) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"symbol": chain.Symbol,
		"name":   chain.Name,
		"status": chain.Status,
	})
}

// GORM Event Handlers
