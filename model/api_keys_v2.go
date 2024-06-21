package model

import (
	"encoding/json"
	"time"

	gouuid "github.com/nu7hatch/gouuid"
)

const (
	APIKeyV2Status_Pending  string = "pending"
	APIKeyV2Status_Inactive string = "inactive"
	APIKeyV2Status_Active   string = "active"
)

type UserApiKeyActionAllowance string

const (
	APIKeyV2ActionAllowance_Allowed    UserApiKeyActionAllowance = "allowed"
	APIKeyV2ActionAllowance_NotAllowed UserApiKeyActionAllowance = "not_allowed"
)

// UserAPIKeysv2 structure
type UserAPIKeysV2 struct {
	ID                uint64    `gorm:"type:bigint;PRIMARY_KEY;UNIQUE;NOT NULL;" json:"id"`
	UserID            uint64    `sql:"type:bigint REFERENCES users(id)" json:"user_id"`
	PublicKey         string    `json:"public_key"`
	PrivateKey        string    `json:"-"`
	Name              string    `json:"name"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TradingAllowed    string    `json:"trading_allowed"`
	WithdrawalAllowed string    `json:"withdrawal_allowed"`
	// MarginAllowed     string    `json:"margin_allowed"`
	// FutureAllowed     string    `json:"future_allowed"`
}

type AddUserApiKeyV2IpRequest struct {
	ApiKeyId string   `json:"api_key_id"`
	Ip       []string `json:"ip"`
}

type DeleteUserApiKeyV2IpRequest struct {
	IpId []string `json:"ip_id"`
}

type UserApiKeyV2WithIp struct {
	*UserAPIKeysV2 `json:",inline"`
	AllowedIps     []*UserApiKeyV2Ip `json:"allowed_ips" gorm:"-"`
}

type UserApiKeyV2Ip struct {
	Id uint64 `json:"id"`
	IP string `json:"ip"`
}

// NewUserAPIKeyV2 creates a new Key pair
func NewUserAPIKeyV2(userID uint64, name, tradingAllowed, withdrawalAllowed, marginAllowed, futureAllowed string) (*UserAPIKeysV2, string) {
	publicKey := generateKey()
	privateKey := generateKey()

	return &UserAPIKeysV2{
		UserID:            userID,
		Name:              name,
		PublicKey:         publicKey,
		PrivateKey:        HashString(privateKey),
		Status:            APIKeyV2Status_Pending,
		TradingAllowed:    tradingAllowed,
		WithdrawalAllowed: withdrawalAllowed,
	}, privateKey
}

func generateKey() string {
	salt := randSeq(7)
	key, _ := gouuid.NewV4()
	apikey := salt + "." + key.String()

	return apikey
}

func (apiKey UserApiKeyV2WithIp) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":         apiKey.ID,
		"user_id":    apiKey.Status,
		"public_key": apiKey.PublicKey,
		"name":       apiKey.Name,
		"status":     apiKey.Status,
		"created_at": apiKey.CreatedAt,
		"updated_at": apiKey.UpdatedAt,
		"permissions": map[string]string{
			"trading_allowed":    apiKey.TradingAllowed,
			"withdrawal_allowed": apiKey.WithdrawalAllowed,
			// "margin_allowed":     apiKey.MarginAllowed,
			// "future_allowed":     apiKey.FutureAllowed,
		},
		"allowed_ips": apiKey.AllowedIps,
	})
}
