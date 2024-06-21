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

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"
	"golang.org/x/crypto/bcrypt"
)

// UserSettings structure
type UserSettings struct {
	ID                uint64            `sql:"type:bigint" gorm:"primary_key" json:"id"`
	UserID            uint64            `sql:"type:bigint REFERENCES users(id)" json:"user_id"`
	FeesPayedWithPrdx string            `json:"fees_payed_with_prdx"`
	DetectIPChange    string            `json:"detect_ip_change"`
	UserLevel         int               `json:"user_level"`
	AntiPhishingKey   string            `json:"-"`
	GoogleAuthKey     string            `json:"-"`
	SmsAuthKey        string            `json:"-"`
	TradePassword     string            `json:"-"`
	ShowPayWithPrdx   bool              `json:"show_pay_with_prdx"`
	LockAmount        *postgres.Decimal `sql:"type:decimal(36,18)" json:"lock_amount"`
	LockDate          *time.Time        `json:"lock_date"`
	SelectedLayout    *int              `json:"selected_layout"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserVerifications struct {
	Type    bool `json:"type"`
	Kyc     bool `json:"kyc"`
	Account bool `json:"account"`
	Tfa     bool `json:"tfa"`
}

// User with details and settings
type UserWithSettings struct {
	User                *User
	Settings            *UserSettings
	Verifications       UserVerifications
	LastLogin           string `json:"last_login"`
	TradePasswordExists bool   `json:"trade_password_exists"`
	DetectIPChange      bool   `json:"detect_ip_change"`
	LoginPasswordExists bool   `json:"login_password_exists"`
	Google2FaExists     bool   `json:"google_2fa_exists"`
	SMS2FaExists        bool   `json:"sms_2fa_exists"`
	AntiPhishingExists  bool   `json:"anti_phishing_exists"`
}

// UserLockAmount info
type UserLockAmount struct {
	LockAmount *decimal.Big ` json:"lock_amount"`
}

// NewUserSettings creates a new user settings structure - should be called when new user is created
func NewUserSettings(userID uint64, feesPayedWithPrdx, detectIPChange, antiPhishingKey, googleAuthKey, smsAuthKey, tradePassword string, userLevel int) *UserSettings {
	return &UserSettings{
		UserID:            userID,
		FeesPayedWithPrdx: feesPayedWithPrdx,
		DetectIPChange:    detectIPChange,
		UserLevel:         userLevel,
		AntiPhishingKey:   antiPhishingKey,
		GoogleAuthKey:     googleAuthKey,
		SmsAuthKey:        smsAuthKey,
		TradePassword:     tradePassword,
		LockAmount:        &postgres.Decimal{V: decimal.New(0, 4)},
	}
}

// Model Methods

// EncodePass encode the password
func (usersettings *UserSettings) EncodePass() error {
	// Generate "hash" to store from user password
	hash, err := bcrypt.GenerateFromPassword([]byte(usersettings.TradePassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	usersettings.TradePassword = string(hash)
	return nil
}

// ValidatePass check if the given password matches the user
func (usersettings *UserSettings) ValidatePass(pass string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(usersettings.TradePassword), []byte(pass)); err != nil {
		// invalid password received
		return false
	}
	// password was correct
	return true
}
