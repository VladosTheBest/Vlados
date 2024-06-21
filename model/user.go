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
	"errors"
	"fmt"
	"time"

	"github.com/ericlagergren/decimal"
	"golang.org/x/crypto/bcrypt"
)

// UserStatus defined the list of possible user statuses
type UserStatus string
type UserPhoneStatus string

const (
	// UserStatusPending when user is newly created and email address is not verified
	UserStatusPending UserStatus = "pending"
	// UserStatusActive when user is active in the system with email address confirmed
	UserStatusActive UserStatus = "active"
	// UserStatusBlocked when user is blocked by the admin
	UserStatusBlocked UserStatus = "blocked"
	// UserStatusDeleted when user is deleted by the admin
	UserStatusDeleted UserStatus = "deleted"
	// UserStatusRemoved when user is removed by the admin
	UserStatusRemoved UserStatus = "removed"
)

func (u UserStatus) String() string {
	return string(u)
}

type UserEmailStatus string

const (
	UserEmailStatusAllowed UserEmailStatus = "allowed"
	UserEmailStatusBlocked UserEmailStatus = "blocked"
	UserPhoneStatusAllowed UserPhoneStatus = "allowed"
	UserPhoneStatusBlocked UserPhoneStatus = "blocked"
)

// User structure
type User struct {
	ID uint64 `sql:"type: bigint" gorm:"primary_key" json:"id"`

	AccountType  string          `json:"account_type"`
	FirstName    string          `json:"first_name"`
	LastName     string          `json:"last_name"`
	Email        string          `gorm:"unique;" json:"email"`
	Phone        string          `gorm:"unique;" json:"phone"`
	Nickname     string          `json:"nickname" `
	Password     string          `gorm:"not null" json:"-"`
	Avatar       string          `json:"avatar"`
	Role         Role            `gorm:"foreignkey:RoleAlias" json:"-"`
	RoleAlias    string          `json:"role_alias"`
	Status       UserStatus      `sql:"not null;type:user_status_t" json:"status"`
	ReferralCode string          `gorm:"column:referral_code" json:"referral_code"`
	ReferralId   string          `gorm:"column:referral_id"`
	KycID        *uint64         `sql:"type:bigint REFERENCES kycs(id)" json:"kyc_id"`
	FirstLogin   bool            `json:"first_login"`
	EmailStatus  UserEmailStatus `sql:"type:user_email_status_t"`
	PhoneStatus  UserPhoneStatus `sql:"type:user_phone_status_t"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProfileResponse Created in terms of Task PDAX2-1563
type ProfileResponse struct {
	FirstName         string           `form:"first_name" json:"first_name" binding:"required"`
	LastName          string           `form:"last_name" json:"last_name" binding:"required"`
	Email             string           `json:"email"`
	RoleAlias         string           `json:"role_alias"`
	ReferralCode      string           `json:"referral_code"`
	DOB               *time.Time       `form:"dob" json:"dob" binding:"required"`
	Gender            string           `json:"gender"`
	Status            string           `json:"status"`
	Phone             string           `form:"phone" json:"phone" binding:"required"`
	Address           string           `form:"address" json:"address" binding:"required"`
	Country           string           `form:"country" json:"country" binding:"required"`
	State             string           `form:"state" json:"state"  binding:"required"`
	City              string           `form:"city" json:"city"  binding:"required"`
	PostalCode        string           `form:"postal_code" json:"postal_code"  binding:"required"`
	Language          LanguageCode     `form:"language" json:"language"  binding:"required"`
	FeesPayedWithPrdx bool             `json:"fees_payed_with_prdx"`
	UserLevel         int              `json:"user_level"`
	AntiPhishingKey   string           `json:"-"`
	GoogleAuthKey     string           `json:"-"`
	TradePassword     string           `json:"-"`
	SmsAuthKey        string           `json:"-"`
	ShowPayWithPrdx   bool             `json:"show_pay_with_prdx"`
	LastLogin         string           `json:"last_login"`
	SecuritySettings  SecuritySettings `json:"security_setting"`
}

// SecuritySettings Created in terms of Task PDAX2-1563
type SecuritySettings struct {
	TradePasswordExists bool `json:"trade_password_exists"`
	DetectIPChange      bool `json:"detect_ip_change"`
	LoginPasswordExists bool `json:"login_password_exists"`
	Google2FaExists     bool `json:"google_2fa_exists"`
	SMS2FaExists        bool `json:"sms_2fa_exists"`
	AntiPhishingExists  bool `json:"anti_phishing_exists"`
}

// UserList structure
type UserList struct {
	Users []UserWithKycStatus
	Meta  PagingMeta
}

type UserOnlineLogRequest struct {
	UserId []uint64 `json:"user_id" form:"user_id"`
}

type UserBalance struct {
	UserId     uint64      `json:"user_id"`
	Email      string      `json:"email"`
	Available  decimal.Big `json:"available"`
	Locked     decimal.Big `json:"locked"`
	CoinSymbol string      `json:"coin_symbol"`
}

// TopInviters structure
type TopInviters struct {
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	Level1Invited int       `json:"level_1_invited"`
}

// UserWithReferrals
type UserWithReferrals struct {
	User
	Level1Invited int         `json:"level_1_invited"`
	Earned        JSONDecimal `sql:"type:decimal(36,18)" json:"earned"`
}

// UserWithReferralsList
type UserWithReferralsList struct {
	Users []UserWithReferrals
	Meta  PagingMeta
}

type UserWithKycStatus struct {
	User
	KycStatus string `gorm:"column:status" json:"kyc_status"`
}

type TotalUserLineLevels struct {
	Count     int64  `json:"count"`
	UserLevel string `json:"user_level"`
}

type TotalUserLineLevelsList struct {
	LineLevels []TotalUserLineLevels `json:"line_levels"`
}

type UserDeposits struct {
	AmountFiat   *decimal.Big `json:"amount_fiat"`
	AmountCrypto *decimal.Big `json:"amount_crypto"`
	FirstLogin   time.Time    `json:"registration_date"`
}

// NewUser creates a new user structure

func NewUser(accountType, firstName, lastName, email, phone, pass, roleAlias, referralId string) *User {
	referralCode := randSeq(8)
	return &User{
		AccountType:  accountType,
		FirstName:    firstName,
		LastName:     lastName,
		Email:        email,
		Phone:        phone,
		Password:     pass,
		Nickname:     "",
		RoleAlias:    roleAlias,
		Status:       UserStatusPending,
		FirstLogin:   true,
		ReferralCode: referralCode,
		ReferralId:   referralId,
		EmailStatus:  UserEmailStatusAllowed,
		PhoneStatus:  UserPhoneStatusAllowed,
	}
}

// Model Methods

// EncodePass encode the password
func (user *User) EncodePass() error {
	// Generate "hash" to store from user password
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	return nil
}

// ValidatePass check if the given password matches the user
func (user *User) ValidatePass(pass string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pass)); err != nil {
		// invalid password received
		return false
	}
	// password was correct
	return true
}

// GetUserStatusFromString -
func GetUserStatusFromString(s string) (UserStatus, error) {
	switch s {
	case "pending":
		return UserStatusPending, nil
	case "active":
		return UserStatusActive, nil
	case "blocked":
		return UserStatusBlocked, nil
	case "removed":
		return UserStatusRemoved, nil
	default:
		return UserStatusPending, errors.New("Status is not valid")
	}
}

func GetUserEmailStatusFromString(s string) (UserEmailStatus, error) {
	switch s {
	case "allowed":
		return UserEmailStatusAllowed, nil
	case "blocked":
		return UserEmailStatusBlocked, nil
	default:
		return UserEmailStatusAllowed, errors.New("Email status is not valid")

	}
}

func (user *User) FullName() string {
	return fmt.Sprintf("%s %s", user.FirstName, user.LastName)
}

type RegistrationType string

const (
	RegistrationTypePrivate   RegistrationType = "private"
	RegistrationTypeCorporate RegistrationType = "corporate"
)

type RegistrationRequest struct {
	Email        string           `form:"email"`
	Phone        string           `form:"phone"`
	Password     string           `form:"password" binding:"required"`
	Type         RegistrationType `form:"type" binding:"required"`
	ReferralCode string           `form:"referral"`
	MarketingID  string           `form:"mid"`
	Role         RoleAlias        `form:"role"`
}

func (r RegistrationType) IsValid() bool {
	switch r {
	case RegistrationTypePrivate, RegistrationTypeCorporate:
		return true
	default:
		return false
	}
}

func (r RegistrationType) String() string {
	return string(r)
}

func (emailStatus UserEmailStatus) IsAllowed() bool {
	return emailStatus == UserEmailStatusAllowed
}
