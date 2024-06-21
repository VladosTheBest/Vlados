package model

// /*
//  * Copyright © 2018-2019 Around25 SRL <office@around25.com>
//  *
//  * Licensed under the Around25 Wallet License Agreement (the "License");
//  * you may not use this file except in compliance with the License.
//  * You may obtain a copy of the License at
//  *
//  *     http://www.around25.com/licenses/EXCHANGE_LICENSE
//  *
//  * Unless required by applicable law or agreed to in writing, software
//  * distributed under the License is distributed on an "AS IS" BASIS,
//  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  * See the License for the specific language governing permissions and
//  * limitations under the License.
//  *
//  * @author		Cosmin Harangus <cosmin@around25.com>
//  * @copyright 2018-2019 Around25 SRL <office@around25.com>
//  * @license 	EXCHANGE_LICENSE
//  */

import (
	"encoding/json"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// BurntPRDX structure
type BurnEvent struct {
	ID          uint64            `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"-"`
	Volume      *postgres.Decimal `sql:"type:decimal(36, 18)" json:"volume"`
	DateBurnt   *time.Time        `json:"date_burnt"`
	AmountBurnt *postgres.Decimal `sql:"type:decimal(36, 18)" json:"amount_burnt"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// MarshalJSON convert the burn event into a json string
func (event *BurnEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":           event.ID,
		"amount_burnt": utils.Fmt(event.AmountBurnt.V),
		"volume":       utils.Fmt(event.Volume.V),
		"date_burnt":   event.DateBurnt,
		"created_at":   event.CreatedAt,
		"updated_at":   event.UpdatedAt,
	})
}

// BurnEventList structure
type BurnEventList struct {
	BurnEvents []BurnEvent `json:"burn_events"`
	Meta       PagingMeta  `json:"meta"`
}

// NewBurnEvent structure
func NewBurnEvent(volume *decimal.Big, day *time.Time, amountBurnt *decimal.Big) *BurnEvent {
	return &BurnEvent{
		Volume:      &postgres.Decimal{V: volume},
		DateBurnt:   day,
		AmountBurnt: &postgres.Decimal{V: amountBurnt},
	}
}
