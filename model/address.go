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
	"time"
)

type AddressType string

const (
	AddressType_User   AddressType = "user"
	AddressType_System AddressType = "system"
)

type AddressStatus string

const (
	AddressStatus_Active   AddressStatus = "active"
	AddressStatus_Archived AddressStatus = "archived"
	AddressStatus_Disabled AddressStatus = "disabled"
)

// Address structure
type Address struct {
	ID          string        `sql:"type:uuid" gorm:"PRIMARY_KEY" json:"id"`
	Address     string        `json:"address"`
	Wallet      string        `gorm:"default:exchange" json:"wallet"`
	Type        AddressType   `sql:"not null;type:address_type_t;default:'user'" json:"type"`
	DepositCode string        `gorm:"column:deposit_code" json:"deposit_code"`
	Status      AddressStatus `sql:"not null;type:address_status_t;default:'active'" json:"status"`
	CreatedAt   time.Time     `json:"-"`
	UpdatedAt   time.Time     `json:"-"`
	Chain       Chain         `gorm:"foreignkey:ChainSymbol" json:"-"`
	ChainSymbol string        `gorm:"column:chain_symbol" json:"chain_symbol"`

	User   User   `gorm:"foreignkey:UserID" json:"-"`
	UserID uint64 `gorm:"column:user_id" json:"-"`
}

// NewAddress create a new address for a chain
func NewAddress(id string, userID uint64, chainSymbol string, addrType AddressType, status AddressStatus, wallet, publicKey, depositCode string) *Address {
	return &Address{
		ID:          id,
		UserID:      userID,
		ChainSymbol: chainSymbol,
		Type:        addrType,
		Status:      status,
		Wallet:      wallet,
		Address:     publicKey,
		DepositCode: depositCode,
	}
}

type UserWithdrawAddress struct {
	ID      uint64 `form:"id"      json:"id"`
	UserID  uint64 `form:"user_id" json:"user_id"`
	Name    string `form:"name"    json:"name"     binding:"required"`
	Address string `form:"address" json:"address"  binding:"required"`
	Coin    string `form:"coin"    json:"coin"     binding:"required"`
}
