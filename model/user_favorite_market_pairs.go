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

// UserFavoriteMarketPairs structure
type UserFavoriteMarketPairs struct {
	ID        uint64    `sql:"type:bigint" gorm:"primary_key" json:"-"`
	UserID    uint64    `sql:"type:bigint REFERENCES users(id)"`
	Pair      string    `json:"pair"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// NewUserFavoriteMarketPair creates a new user favorite market pair structure
func NewUserFavoriteMarketPairs(userID uint64, pair string) *UserFavoriteMarketPairs {
	return &UserFavoriteMarketPairs{
		UserID: userID,
		Pair:   pair}
}