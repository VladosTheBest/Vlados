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
	"time"
)

type GenderType string

const (
	GenderTypeMale            GenderType = "male"
	GenderTypeFemale          GenderType = "female"
	ShuftiProGenderTypeMale   GenderType = "M"
	ShuftiProGenderTypeFemale GenderType = "F"
)

func (g GenderType) String() string {
	return string(g)
}

func (g GenderType) ToShuftiProGenderType() GenderType {
	switch g {
	case GenderTypeMale:
		return ShuftiProGenderTypeMale
	case GenderTypeFemale:
		return ShuftiProGenderTypeFemale
	default:
		return g
	}
}

func DetermineGenderType(gender string) GenderType {
	switch gender {
	case "male", "M":
		return GenderTypeMale
	case "female", "F":
		return GenderTypeFemale
	default:
		return ""
	}
}

type LanguageCode string

const (
	LanguageCodeEnglish LanguageCode = "en"
	LanguageCodeItalian LanguageCode = "it"
	LanguageCodeRussian LanguageCode = "ru"
	LanguageCodeTurkish LanguageCode = "tr"
)

func (l LanguageCode) String() string {
	return string(l)
}

func (l LanguageCode) IsValid() bool {
	switch l {
	case LanguageCodeEnglish,
		LanguageCodeItalian,
		LanguageCodeRussian,
		LanguageCodeTurkish:
		return true
	default:
		return false
	}
}

// UserDetails structure
type UserDetails struct {
	ID               uint64       `sql:"type:bigint" gorm:"primary_key"`
	UserID           uint64       `sql:"type:bigint REFERENCES users(id)"`
	Gender           GenderType   `json:"gender"`
	DOB              *time.Time   `json:"dob"`
	Phone            string       `json:"phone"`
	Address          string       `json:"address"`
	Country          string       `json:"country"`
	State            string       `json:"state"`
	City             string       `json:"city"`
	PostalCode       string       `json:"postal_code"`
	Language         LanguageCode `json:"language"`
	Timezone         string       `json:"time_zone" gorm:"column:time_zone"`
	LeadFromResource string       `json:"lead_from_resource" gorm:"column:lead_from_resource"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// UserDetailsWithVerifications structure
type UserDetailsWithVerifications struct {
	UserDetails   UserDetails
	Verifications UserVerifications
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
}

// NewUserDetails creates a new user details structure
func NewUserDetails(userID uint64, phone, address, country, state, city, postalCode string, gender GenderType, language LanguageCode, dob *time.Time, leadFromResource string) *UserDetails {
	return &UserDetails{
		UserID:           userID,
		Gender:           gender,
		DOB:              dob,
		Phone:            phone,
		Address:          address,
		Country:          country,
		State:            state,
		City:             city,
		PostalCode:       postalCode,
		Language:         language,
		LeadFromResource: leadFromResource,
	}
}

// GetGenderTypeFromString returns the gender type for a string
func GetGenderTypeFromString(s string) (GenderType, error) {
	switch s {
	case "male":
		return GenderTypeMale, nil
	case "female":
		return GenderTypeFemale, nil
	case "":
		return GenderTypeMale, nil
	default:
		return GenderTypeMale, errors.New("Gender is not valid")
	}
}

// Model Methods

// GORM Event Handlers
