package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/nyaruka/phonenumbers"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

const (
	baseURL = "https://verify.twilio.com/v2/Services"
)

// InitBindPhone - start: bind users phone in authy/twillio
func (service *Service) InitBindPhone(id uint64, email, phone string) (uint64, error) {

	type bindInitResponse struct {
		Message string
		User    struct {
			ID uint64
		}
		Success bool
	}

	num, err := phonenumbers.Parse(phone, "")
	if err != nil {
		return 0, err
	}

	log.Info().Msg("user[email]=" + url.QueryEscape(email) + "&user[cellphone]=" + fmt.Sprint(*num.NationalNumber) + "&user[country_code]=" + fmt.Sprint(*num.CountryCode))

	if len(fmt.Sprint(*num.NationalNumber)) < 6 || len(fmt.Sprint(*num.CountryCode)) < 1 {
		return 0, errors.New("Invalid phone number")
	}

	requestUrl := service.cfg.Server.Twillio.URL + "users/new"
	payload := strings.NewReader("user[email]=" + url.QueryEscape(email) + "&user[cellphone]=" + fmt.Sprint(*num.NationalNumber) + "&user[country_code]=" + fmt.Sprint(*num.CountryCode))
	//todo delete log
	log.Info().Msg("user[email]=" + url.QueryEscape(email) + "&user[cellphone]=" + fmt.Sprint(*num.NationalNumber) + "&user[country_code]=" + fmt.Sprint(*num.CountryCode))
	req, err := http.NewRequest("POST", requestUrl, payload)
	if err != nil {
		return 0, err
	}

	req.Header.Add("X-Authy-API-Key", service.cfg.Server.Twillio.APIKey)
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}

	resp := &bindInitResponse{}
	jsonErr := json.Unmarshal(body, &resp)
	if jsonErr != nil {
		return 0, jsonErr
	}

	if !resp.Success {
		return 0, errors.New(resp.Message)
	}

	_, err = service.SendSMS(fmt.Sprint(resp.User.ID))
	if err != nil {
		return resp.User.ID, err
	}
	return resp.User.ID, nil
}

// BindPhone - bind users phone in DB after authy/twillio validation
func (service *Service) BindPhone(id uint64, authyID, code string) (*model.UserSettings, bool, error) {

	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id=?", id)
	if db.Error != nil {
		return nil, false, db.Error
	}

	if userSettings.SmsAuthKey != "" {
		return nil, false, errors.New("Phone already bound")
	}

	valid, _ := service.VerifySmsCode(authyID, code)
	if !valid {
		return nil, false, errors.New("Invalid verification code")
	}

	userSettings.SmsAuthKey = string(authyID)
	err := service.repo.Update(userSettings)
	if err != nil {
		return nil, false, err
	}

	return &userSettings, true, err
}

// UnbindPhone - bind users phone in DB after authy/twillio validation
func (service *Service) UnbindPhone(id uint64) (*model.UserSettings, bool, error) {

	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id=?", id)
	if db.Error != nil {
		return nil, false, db.Error
	}
	requestURL := service.cfg.Server.Twillio.URL + "users/" + userSettings.SmsAuthKey + "/remove"
	req, _ := http.NewRequest("GET", requestURL, nil)

	req.Header.Add("X-Authy-API-Key", service.cfg.Server.Twillio.APIKey)
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer res.Body.Close()

	userSettings.SmsAuthKey = ""
	err = service.repo.Update(userSettings)
	if err != nil {
		return nil, false, err
	}

	return &userSettings, true, err
}

// SendSMS - send sms to user's phone
func (service *Service) SendSMS(authyID string) (bool, error) {

	requestURL := service.cfg.Server.Twillio.URL + "sms/" + authyID
	req, _ := http.NewRequest("GET", requestURL, nil)

	req.Header.Add("X-Authy-API-Key", service.cfg.Server.Twillio.APIKey)
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	return true, err
}

// ValidateSmsCode - validate sms code sent to user's phone: based on user.ID
func (service *Service) ValidateSmsCode(id uint64, code string) (bool, error) {
	userSettings := model.UserSettings{}
	db := service.repo.Conn.First(&userSettings, "user_id=?", id)
	if db.Error != nil {
		return false, db.Error
	}

	if userSettings.SmsAuthKey == "" {
		return false, errors.New("Phone not bound")
	}

	return service.VerifySmsCode(userSettings.SmsAuthKey, code)
}

// VerifySmsCode - validate sms code sent to user's phone: based on authyID
func (service *Service) VerifySmsCode(authyID, code string) (bool, error) {

	type initResponse struct {
		Message string
		Token   string
		Success string
	}

	requestURL := service.cfg.Server.Twillio.URL + "verify/" + code + "/" + string(authyID)
	req, _ := http.NewRequest("GET", requestURL, nil)

	req.Header.Add("X-Authy-API-Key", service.cfg.Server.Twillio.APIKey)
	req.Header.Add("cache-control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, err
	}

	resp := &initResponse{}
	jsonErr := json.Unmarshal(body, &resp)
	if jsonErr != nil {
		return false, jsonErr
	}

	if resp.Success != "true" {
		return false, errors.New(resp.Message)
	}

	return true, nil
}

func (service Service) SendVerificationCode(serviceSid, phoneNumber, accountSid, authToken string) error {
	client := &http.Client{}

	form := url.Values{}
	form.Add("To", phoneNumber)
	form.Add("Channel", "sms")

	req, err := http.NewRequest("POST", baseURL+"/"+serviceSid+"/Verifications", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(accountSid, authToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to send verification code: %s", string(body))
	}

	return nil
}

func (service Service) CheckVerificationCode(serviceSid, phoneNumber, code, accountSid, authToken string) (bool, error) {
	client := &http.Client{}

	form := url.Values{}
	form.Add("To", phoneNumber)
	form.Add("Code", code)

	req, err := http.NewRequest("POST", baseURL+"/"+serviceSid+"/VerificationCheck", strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(accountSid, authToken)

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	fmt.Println("Raw response body:", string(body))

	var result struct {
		Status string `json:"status"`
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return false, err
	}

	return result.Status == "approved", nil
}
