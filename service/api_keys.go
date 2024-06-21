package service

import (
	"errors"
	"strings"

	apiKeyCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/apikey"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetAPIKeyByID godoc
func (service *Service) GetAPIKeyByID(id uint64) (*model.UserAPIKeys, error) {
	return service.repo.GetAPIKeyByID(id)
}

// ActivateAPIKey godoc
func (service *Service) ActivateAPIKey(apiKey *model.UserAPIKeys) error {
	return service.repo.ActivateAPIKey(apiKey)
}

// GetAPIKeys - get a user's API keys
func (service *Service) GetAPIKeys(userID uint64) ([]model.UserAPIKeys, error) {
	apiKeys := make([]model.UserAPIKeys, 0)
	db := service.repo.ConnReader.Where("user_id = ? and status = ?", userID, model.APIKeyStatus_Active).Find(&apiKeys)
	if db.Error != nil {
		return nil, db.Error
	}

	return apiKeys, nil
}

// GenerateAPIKey - get a user's API keys
func (service *Service) GenerateAPIKey(userID uint64, apiKeyName, role string) (string, *model.UserAPIKeys, error) {
	if !service.ValidateRoleInScope(role, "api") {
		return "", nil, errors.New("role: Invalid role")
	}
	apiKey := model.NewUserAPIKey(userID, apiKeyName, role)
	apiKeyToShow := apiKey.ApiKey
	err := apiKey.EncodeKey()
	if err != nil {
		return "", nil, err
	}

	err = service.repo.Create(&apiKey)
	if err != nil {
		return "", nil, err
	}

	return apiKeyToShow, apiKey, nil
}

// RemoveAPIKey - get a user's API keys
// @todo CH: Refactor this to only remove the KEY without finding it first
func (service *Service) RemoveAPIKey(userID uint64, keyID string) (string, error) {
	apikey := model.UserAPIKeys{}
	db := service.repo.Conn.First(&apikey, "user_id = ? AND id=?", userID, keyID)
	if db.Error != nil {
		return apikey.ApiKey, db.Error
	}
	db = service.repo.Conn.Delete(apikey)
	return apikey.Name, db.Error
}

// GetAPIKeyAndUserByToken - get a user's API key
func (service *Service) GetAPIKeyAndUserByToken(token string) (*model.UserAPIKeys, error) {
	var key *model.UserAPIKeys
	prefix := strings.Split(token, ".")[0]
	key, decoded, found, isDecoded := apiKeyCache.Get(prefix)

	// if it's decoded and it matches simply return the key
	if found && isDecoded && decoded == token {
		return key, nil
	}

	// if found and decoded but it does not match then return an error
	if found && isDecoded {
		return key, errors.New("apikey: Invalid token")
	}

	// if found but it's not decoded check if it's valid and if it is update the cache with the token
	if found && !isDecoded && key.ValidateKey(token) {
		_ = apiKeyCache.SetDecoded(prefix, token)
		return key, nil
	}

	// if the key was found but it's invalid return an error
	if found && !isDecoded {
		return key, errors.New("apikey: Invalid token")
	}

	// if not found in the cache follow the normal route and load it from db
	db := service.repo.ConnReader.Where("user_api_keys.prefix = ? AND status = ?", prefix, model.APIKeyStatus_Active).First(&key)
	if db.Error != nil {
		return key, db.Error
	}
	if !key.ValidateKey(token) {
		return key, errors.New("apikey: Invalid token")
	}
	return key, nil
}
