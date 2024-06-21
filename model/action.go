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
	"time"

	gouuid "github.com/nu7hatch/gouuid"
)

type ActionType string

const (
	ActionType_Withdraw  ActionType = "withdraw"
	ActionType_Deposit   ActionType = "deposit"
	ActionType_Other     ActionType = "other"
	ActionType_ConfirmIP ActionType = "confirm_ip"
	ActionType_ResetPass ActionType = "reset_pass"
	ActionType_APIKey    ActionType = "api_key"
	ActionType_APIKeyV2  ActionType = "api_key_v2"
	ActionType_Role      ActionType = "role"
	ActionType_Admin     ActionType = "admin"
)

type ActionStatus string

const (
	ActionStatus_Pending       ActionStatus = "pending"
	ActionStatus_ApprovedUser  ActionStatus = "approved_user"
	ActionStatus_ApprovedAdmin ActionStatus = "approved_admin"
	ActionStatus_Approved      ActionStatus = "approved"
	ActionStatus_Processing    ActionStatus = "processing"
	ActionStatus_Completed     ActionStatus = "completed"
)

func (a ActionType) IsValid() bool {
	switch a {
	case ActionType_Deposit,
		ActionType_Withdraw:
		return true
	default:
		return false
	}
}

// Action structure
type Action struct {
	ID           uint64       `gorm:"PRIMARY_KEY" json:"id"`
	UUID         string       `sql:"type:uuid" json:"uuid"`
	Action       string       `json:"action"`
	Type         ActionType   `gorm:"column:action_type" sql:"not null;type:action_type_t;default:'other'" json:"type"`
	MinApprovals int          `gorm:"column:min_approvals" json:"min_approvals"`
	Status       ActionStatus `sql:"not null;type:action_status_t;default:'pending'" json:"status"`
	UserKey      string       `sql:"type:uuid" json:"-"`
	AdminKey     string       `sql:"type:uuid" json:"-"`
	Data         string       `json:"-"`
	User         User         `gorm:"foreignkey:UserID" json:"-"`
	UserID       uint64       `gorm:"column:user_id" json:"user_id"`
	CreatedAt    time.Time    `json:"-"`
	UpdatedAt    time.Time    `json:"-"`
}

// ActionData godoc
type ActionData interface {
	ToString() (string, error)
	FromString(string) error
}

// WithdrawActionData godoc
type WithdrawActionData struct {
	WithdrawRequestID string `json:"withdraw_request_id"`
}

// ToString godoc
func (data *WithdrawActionData) ToString() (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FromString godoc
func (data *WithdrawActionData) FromString(bytes string) error {
	b := []byte(bytes)
	return json.Unmarshal(b, &data)
}

// APIKeyData godoc
type APIKeyData struct {
	APIKeyID uint64 `json:"api_key_id"`
}

// ToString godoc
func (data *APIKeyData) ToString() (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FromString godoc
func (data *APIKeyData) FromString(bytes string) error {
	b := []byte(bytes)
	return json.Unmarshal(b, &data)
}

// ConfirmIPData godoc
type ConfirmIPData struct {
	IP string `json:"ip"`
}

// ToString godoc
func (data *ConfirmIPData) ToString() (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// FromString godoc
func (data *ConfirmIPData) FromString(bytes string) error {
	b := []byte(bytes)
	return json.Unmarshal(b, &data)
}

// NewAction godoc
// Create a new action that will be used for validation and processing of sensitive requests
func NewAction(userID uint64, aType ActionType, data string) *Action {
	u, _ := gouuid.NewV4()
	uk, _ := gouuid.NewV4()
	ak, _ := gouuid.NewV4()
	return &Action{
		UserID:       userID,
		Type:         aType,
		Status:       ActionStatus_Pending,
		Data:         data,
		MinApprovals: 1,
		UUID:         u.String(),
		UserKey:      uk.String(),
		AdminKey:     ak.String(),
	}
}

// Approve godoc
// Check if the given key can approve the action and change the status accordingly
func (action *Action) Approve(key string) bool {
	// already approved
	if action.MinApprovals == 1 && action.Status != ActionStatus_Pending {
		return false
	}
	// invalid key
	if key != action.UserKey && key != action.AdminKey {
		return false
	}
	// update status
	if action.MinApprovals == 1 {
		action.Status = ActionStatus_Approved
	} else {
		if action.Status == ActionStatus_Pending {
			if key == action.UserKey {
				action.Status = ActionStatus_ApprovedUser
			} else {
				action.Status = ActionStatus_ApprovedAdmin
			}
		} else if (action.Status == ActionStatus_ApprovedUser && key == action.AdminKey) ||
			(action.Status == ActionStatus_ApprovedAdmin && key == action.UserKey) {
			action.Status = ActionStatus_Approved
		}
	}
	// return true
	return true
}
