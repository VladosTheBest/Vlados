package service

import (
	"errors"
	apiKeyCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/apikeyV2"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetAPIKeysV2 godoc
func (service *Service) GetAPIKeysV2(userID uint64) ([]model.UserApiKeyV2WithIp, error) {
	apiKeys := make([]model.UserApiKeyV2WithIp, 0)
	db := service.repo.ConnReader.
		Table("user_api_keys_v2 uakv2").
		Where("user_id = ? and status = ?", userID, model.APIKeyStatus_Active).
		Select("uakv2.*").Find(&apiKeys)
	if db.Error != nil {
		return nil, db.Error
	}

	for id, apiKey := range apiKeys {
		db = service.repo.ConnReader.
			Table("user_api_keys_allowed_ips uakai").
			Where("api_key_id = ?", apiKey.ID).Find(&apiKeys[id].AllowedIps)
		if db.Error != nil {
			return nil, db.Error
		}
	}

	return apiKeys, nil
}

// GetAPIKeyV2ByID godoc
func (service *Service) GetAPIKeyV2ByID(id uint64) (*model.UserAPIKeysV2, error) {
	return service.repo.GetAPIKeyV2ByID(id)
}

// ActivateAPIKeyV2 godoc
func (service *Service) ActivateAPIKeyV2(apiKey *model.UserAPIKeysV2) error {
	return service.repo.ActivateAPIKeyV2(apiKey)
}

// GenerateAPIKeyV2 -
func (service *Service) GenerateAPIKeyV2(userID uint64, apiKeyName, tradingAllowed, withdrawalAllowed, marginAllowed, futureAllowed string) (string, *model.UserAPIKeysV2, error) {
	apiKey, privateKey := model.NewUserAPIKeyV2(userID, apiKeyName, tradingAllowed, withdrawalAllowed, marginAllowed, futureAllowed)

	err := service.repo.Create(&apiKey)
	if err != nil {
		return "", nil, err
	}

	return privateKey, apiKey, nil
}

func (service *Service) RemoveAPIKeyV2(userId uint64, keyID string) error {
	return service.repo.Conn.Where("user_id = ?", userId).Delete(&model.UserAPIKeysV2{}, keyID).Error
}

func (service *Service) GetAPIKeyV2AndUserByToken(token string) (*model.UserAPIKeysV2, error) {
	key := &model.UserAPIKeysV2{}

	key, found := apiKeyCache.Get(model.HashString(token))

	if found {
		if key.Status == "active" {
			return key, nil
		}
		return nil, errors.New("key is not active")
	}

	db := service.repo.ConnReader.Where("(user_api_keys_v2.public_key = ? OR user_api_keys_v2.private_key = ?) AND status = ?",
		token, model.HashString(token), model.APIKeyStatus_Active).First(&key)
	if db.Error != nil {
		return nil, db.Error
	}

	return key, nil
}

func (service *Service) GetAPIKeyV2AndUserByPrivateKey(token string) (*model.UserAPIKeysV2, error) {
	key, found := apiKeyCache.Get(model.HashString(token))

	if found {
		if key.Status == "active" {
			return key, nil
		}
		return nil, errors.New("key is not active")
	}

	key = &model.UserAPIKeysV2{}

	db := service.repo.ConnReader.Where("user_api_keys_v2.private_key = ? AND status = ?",
		model.HashString(token), model.APIKeyStatus_Active).First(&key)
	if db.Error != nil {
		return nil, db.Error
	}

	return key, nil
}

func (service *Service) AddApiKeyAllowedIp(request *model.AddUserApiKeyV2IpRequest) error {

	for _, ip := range request.Ip {
		allowedIp := model.NewApiKeyAllowedIp(request.ApiKeyId, ip)
		err := service.repo.Create(&allowedIp)
		if err != nil {
			return err
		}

	}

	return nil
}

func (service *Service) RemoveAllowedIp(ipId []string) error {
	if len(ipId) == 0 {
		return nil
	}

	return service.repo.Conn.Delete(&model.UserApiKeysAllowedIps{}, "id IN (?)", ipId).Error
}

func (service *Service) GetApiKeysPermissions() []string {
	permissions := []string{"trading_allowed", "withdrawal_allowed"}

	return permissions
}
