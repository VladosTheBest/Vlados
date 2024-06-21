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
	"crypto/sha1"
	"encoding/hex"
	"math/rand"
	"time"

	gouuid "github.com/nu7hatch/gouuid"
)

const (
	APIKeyStatus_Pending  string = "pending"
	APIKeyStatus_Inactive string = "inactive"
	APIKeyStatus_Active   string = "active"
)

// UserAPIKeys structure
type UserAPIKeys struct {
	ID        uint64    `gorm:"type:bigint;PRIMARY_KEY;UNIQUE;NOT NULL;" json:"id"`
	UserID    uint64    `sql:"type:bigint REFERENCES users(id)" json:"user_id"`
	Prefix    string    `json:"prefix"`
	ApiKey    string    `json:"-"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	RoleAlias string    `json:"role_alias"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKeyAndUser structure
type APIKeyAndUser struct {
	UserAPIKeys UserAPIKeys
	User        User
}

// NewUserAPIKey creates a new ApiKey
func NewUserAPIKey(userID uint64, name, role string) *UserAPIKeys {
	salt := randSeq(7)
	key, _ := gouuid.NewV4()
	apikey := salt + "." + key.String()
	return &UserAPIKeys{
		UserID:    userID,
		Name:      name,
		Prefix:    salt,
		ApiKey:    apikey,
		Status:    APIKeyStatus_Pending,
		RoleAlias: role,
	}
}

// Model Methods

// EncodeKey encode the key
// Should only be called once when the key is created
func (userapikey *UserAPIKeys) EncodeKey() error {
	userapikey.ApiKey = HashString(userapikey.ApiKey)
	return nil
}

// ValidateKey check if the given apiKey matches the user's
func (userapikey *UserAPIKeys) ValidateKey(apikeyused string) bool {
	return userapikey.ApiKey == HashString(apikeyused)
}

// HashString godoc
func HashString(str string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func randSeq(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
