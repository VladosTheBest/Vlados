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

// ApprovedIPStatus godoc
type ApprovedIPStatus string

const (
	// ApprovedIPStatus_Pending godoc
	ApprovedIPStatus_Pending ApprovedIPStatus = "pending"
	// ApprovedIPStatus_Approved godoc
	ApprovedIPStatus_Approved ApprovedIPStatus = "approved"
)

// ApprovedIP structure
type ApprovedIP struct {
	ID        uint64           `gorm:"PRIMARY_KEY" json:"id"`
	Status    ApprovedIPStatus `json:"status"`
	IP        string           `json:"ip"`
	UserID    uint64           `gorm:"column:user_id" json:"user_id"`
	CreatedAt time.Time        `json:"-"`
	UpdatedAt time.Time        `json:"-"`
}

// NewPendingIP godoc
// Create a new pending IP
func NewPendingIP(userID uint64, ip string) *ApprovedIP {
	return &ApprovedIP{
		UserID: userID,
		Status: ApprovedIPStatus_Pending,
		IP:     ip,
	}
}

// NewApprovedIP godoc
// Create a new approved IP
func NewApprovedIP(userID uint64, ip string) *ApprovedIP {
	return &ApprovedIP{
		UserID: userID,
		Status: ApprovedIPStatus_Approved,
		IP:     ip,
	}
}
