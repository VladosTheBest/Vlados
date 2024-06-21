package model

import "time"

type UserApiKeysAllowedIps struct {
	ID        uint64    `gorm:"type:bigint;PRIMARY_KEY;UNIQUE;NOT NULL;" json:"id"`
	ApiKeyId  string    `sql:"type:bigint REFERENCES user_api_keys_v2(id)" json:"api_key_id"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"-"`
}

func NewApiKeyAllowedIp(apiKeyId string, ip string) *UserApiKeysAllowedIps {
	return &UserApiKeysAllowedIps{
		ApiKeyId: apiKeyId,
		IP:       ip,
	}
}
