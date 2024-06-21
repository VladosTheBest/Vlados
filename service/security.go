package service

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"strings"

	"github.com/dgryski/dgoogauth"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// CheckOrGenerateGoogleSecretKey - check if user has a google secret key set, if not return a generated one for pairing
func (service *Service) CheckOrGenerateGoogleSecretKey(id uint64) (string, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return "", db.Error
	}

	if userSettings.GoogleAuthKey == "" {
		//no key found, generate one
		key := service.GenerateGoogleSecretKey()
		return key, nil
	}

	//should not return secret key to browser if one is saved to DB
	return "", nil
}

// GenerateGoogleSecretKey - generate a google secret key
func (service *Service) GenerateGoogleSecretKey() string {
	dictionary := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	//length of 15
	var bytes = make([]byte, 15)
	_, _ = rand.Read(bytes)
	for k, v := range bytes {
		bytes[k] = dictionary[v%byte(len(dictionary))]
	}
	//results in 24 character long secret key
	secret := base32.StdEncoding.EncodeToString([]byte(string(bytes)))

	return secret
}

// ValidateGoogleAuthKey - validate a google auth key
func (service *Service) ValidateGoogleAuthKey(id uint64, secret, token string) (bool, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return false, db.Error
	}

	if userSettings.GoogleAuthKey == "" && secret == "" {
		//no key found
		return false, nil
	}
	if userSettings.GoogleAuthKey != "" {
		secret = userSettings.GoogleAuthKey
	}

	otpConfig := &dgoogauth.OTPConfig{
		Secret:      strings.TrimSpace(secret),
		WindowSize:  2,
		HotpCounter: 0,
	}

	// Validate token
	ok, err := otpConfig.Authenticate(token)
	return ok, err
}

// EnableGoogleAuth - enable google auth
func (service *Service) EnableGoogleAuth(id uint64, secret, token string) (*model.UserSettings, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	if userSettings.GoogleAuthKey != "" {
		//secret key set, cannot set new one
		return nil, errors.New("Google authentification already enabled")
	}

	data, err := service.UpdateProfileSettings(id, userSettings.FeesPayedWithPrdx, userSettings.DetectIPChange, userSettings.AntiPhishingKey, secret, userSettings.SmsAuthKey)
	if err != nil {
		return nil, err
	}

	return data, err
}

// DisableGoogleAuth - enable google auth
func (service *Service) DisableGoogleAuth(id uint64) (*model.UserSettings, bool, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		return nil, false, db.Error
	}

	data, err := service.UpdateProfileSettings(id, userSettings.FeesPayedWithPrdx, userSettings.DetectIPChange, userSettings.AntiPhishingKey, "", userSettings.SmsAuthKey)
	return data, err == nil, err
}

// Is2FAEnabled - checks if user has 2FA
func (service *Service) Is2FAEnabled(id uint64) (bool, string) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		// in case of error presume 2FA enabled
		return true, "google"
	}

	if len(userSettings.GoogleAuthKey) > 0 {
		return true, "google"
	}
	if len(userSettings.SmsAuthKey) > 0 {
		return true, "sms"
	}
	return false, ""
}

// Is2FAGoogleEnabled - checks if user has Google 2FA
func (service *Service) Is2FAGoogleEnabled(id uint64) bool {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id = ?", id)
	if db.Error != nil {
		// in case of error presume 2FA enabled
		return true
	}

	if len(userSettings.GoogleAuthKey) > 0 {
		return true
	}
	return false
}

// SendSmsWithCode - sends an 2fa sms
func (service *Service) SendSmsWithCode(id uint64) (bool, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id=?", id)
	if db.Error != nil {
		return false, db.Error
	}

	data, err := service.SendSMS(userSettings.SmsAuthKey)
	return data, err
}
