package service

/*
 * Copyright © 2006-2019 Around25 SRL <office@around25.com>
 *
 * Licensed under the Around25 Exchange License Agreement (the "License");
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
 * @copyright 2006-2019 Around25 SRL <office@around25.com>
 * @license 	EXCHANGE_LICENSE
 */

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/biter777/countries"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"golang.org/x/sync/syncmap"

	"gorm.io/gorm"
)

var kycMutex syncmap.Map

func (service *Service) GetKYCMutexFromMap(id string) *sync.Mutex {
	mutex := &sync.Mutex{}
	tmpMutex, ok := kycMutex.Load(id)
	if !ok {
		kycMutex.Store(id, mutex)
		return mutex
	}

	return tmpMutex.(*sync.Mutex)
}

// Creates a new files upload http request with optional extra params
func (service *Service) stepTwoDocumentVerificationRequest(url string, params model.ShuftiProKYCRequest) (*http.Request, error) {
	// add params
	jsonBody, _ := json.Marshal(params)

	// Determine Content-Length
	contentLength := len(jsonBody)

	logger := log.With().
		Str("section", "kyc").
		Str("action", "KycDocumentVerification").
		Logger()

	logger.Error().Str("URL_request", url) // todo

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	req.SetBasicAuth(service.cfg.Server.KYB.ClientID, service.cfg.Server.KYB.SecretKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Add("User-Agent", "ParamountDax Exchange")
	req.ContentLength = int64(contentLength)

	return req, err
}

// StepTwoDocumentsVerification - call the document verification endpoint
func (service *Service) StepTwoDocumentsVerification(referenceID string, kycDoc *model.KYCDocuments, userKycDetails *model.KycCustomerInfo, callbackURL string) ([]byte, error) {
	country := countries.ByName(userKycDetails.Country)
	if country == countries.Unknown {
		return nil, fmt.Errorf("Invalid country code: " + userKycDetails.Country)
	}

	requestShuftipro := model.ShuftiProKYCRequest{
		ShuftiProAuthRequest: model.ShuftiProAuthRequest{
			Reference:        referenceID,
			CallbackURL:      callbackURL,
			Country:          country.Alpha2(),
			Language:         "EN",
			Email:            service.cfg.Server.KYB.CompanyEmail,
			VerificationMode: "any",
		},
		ShuftiProFaceRequest: model.ShuftiProFaceRequest{},
		ShuftiProDocumentRequest: model.ShuftiProDocumentRequest{
			Name: model.ShuftiProName{
				FirstName: userKycDetails.FirstName,
				LastName:  userKycDetails.LastName,
			},
			Dob:    userKycDetails.DOB.Format("2006-01-02"),
			Gender: userKycDetails.Gender.ToShuftiProGenderType().String(),
		},
	}

	// encode photos in Base64 and add to extra params as string
	for _, file := range kycDoc.Documents {
		isFrontSide := true
		for _, doc := range file.Files {
			docBase64, err := service.EncodeFileToBase64(doc)
			if err != nil {
				return nil, err
			}

			switch file.Type {
			case "selfie":
				requestShuftipro.ShuftiProFaceRequest.Selfie = docBase64
			default:
				if file.Type != "" {
					uniqTypeFlag := true
					for _, dtype := range requestShuftipro.SupportedTypes {
						if string(file.Type) == dtype {
							uniqTypeFlag = false
						}
					}

					if uniqTypeFlag {
						requestShuftipro.SupportedTypes = append(requestShuftipro.SupportedTypes, string(file.Type))
					}
				}
				if isFrontSide {
					requestShuftipro.Proof = docBase64
					isFrontSide = false
				} else {
					requestShuftipro.AdditionalProof = docBase64
				}
			}

		}
	}

	request, err := service.stepTwoDocumentVerificationRequest("https://api.shuftipro.com/", requestShuftipro)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, err := ioutil.ReadAll(resp.Body)

	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}

func (service *Service) ValidateKYCDocuments(file []*multipart.FileHeader) error {
	for _, documentFile := range file {
		err := service.ValidateDocumentMimeTypes(documentFile)
		if err != nil {
			return err
		}
		documentFile.Filename = strings.ToLower(documentFile.Filename)
		if documentFile.Size > utils.MaxKYCFilesSize {
			return fmt.Errorf("size of attachements should be <= %s", utils.HumaneFileSize(utils.MaxKYCFilesSize))
		}
	}

	return nil
}

func (service *Service) ValidateDocumentMimeTypes(header *multipart.FileHeader) error {
	file, err := header.Open()
	if err != nil {
		return err
	}
	fileHeader := make([]byte, 512)
	if _, err := file.Read(fileHeader); err != nil {
		return err
	}

	mimeType := http.DetectContentType(fileHeader)

	switch mimeType {
	case "image/jpeg":
	case "image/jpg":
	case "image/png":
	case "application/pdf":
	case "application/msword":
	case "application/octet-stream":
	case "image/gif":
	case "image/bmp":
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
	case "application/zip":
	default:
		return fmt.Errorf("image type %s not allowed", mimeType)
	}

	return nil
}

func (service *Service) DoShuftiproKYCRequestAsync(kyc *model.KYC, userKYCDetails *model.KycCustomerInfo, kycDocs *model.KYCDocuments, user *model.User, logger *zerolog.Logger) {
	kyc.ReferenceID = service.GenerateShuftiproKYCReferenceID(kyc.ID, user.ID)
	//logger.Debug().Msg("REFERENCEID: " + kyc.ReferenceID)
	//time.Sleep(time.Second * 5)

	userMutex := service.GetKYCMutexFromMap(kyc.ReferenceID)

	userMutex.Lock()
	defer userMutex.Unlock()
	kyc, err := service.SaveShuftiproReferenceID(kyc)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")
		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")

		return
	}

	responseBody, err := service.StepTwoDocumentsVerification(kyc.ReferenceID, kycDocs, userKYCDetails, service.cfg.Server.KYC.CallbackUrl)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")

		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")

		return
	}

	result, err := service.ParseJumioStepTwoCallback(responseBody)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")

		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")

		return
	}

	logger.Debug().Msg("Response from Shuftipro kyc: \n" + string(responseBody))

	_, err = service.UpdateKycOnStep2(kyc, result, user)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")

		}

		logger.Error().Err(err).
			Str("http_resp", string(responseBody)).
			Msg("Unable to update kyc profile")

		return
	}

	if user.EmailStatus.IsAllowed() {
		apCode, _ := service.GetAntiPhishingCode(user.ID)
		userDetails := model.UserDetails{}
		db := service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", user.ID)
		if db.Error != nil {
			logger.Error().Err(db.Error).
				Msg("Unable to update kyc profile")
		}
		err = service.KycNoticePending(user.Email, userDetails.Language.String(), apCode, userDetails.Timezone)
		if err != nil {
			logger.Error().Err(err).
				Msg("Unable to update kyc profile")
		}
	}

}

func (service *Service) GenerateShuftiproKYCReferenceID(kycID, userID uint64) string {
	return "pdax2_kyc_" + strconv.FormatUint(kycID, 10) + "_user_" + strconv.FormatUint(userID, 10) + "_" + strconv.FormatInt(time.Now().Unix(), 10)
}

func (service *Service) SaveShuftiproReferenceID(kyc *model.KYC) (*model.KYC, error) {
	tx := service.repo.Conn.Begin()

	kyc, err := service.UpdateKyc(tx, kyc, model.KYCStatusStepTwoPending)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()

	return kyc, err
}

func (service *Service) ParseJumioStepTwoCallback(responseBody []byte) (*model.ShuftiProKYCResponse, error) {
	if len(responseBody) == 0 {
		return nil, errors.New("error: empty Jumio step two callback body")
	}

	result := new(model.ShuftiProKYCResponse)

	err := json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, err
	}

	return result, err
}

// Creates a new files upload http request with optional extra params
func (service *Service) getJumioIdVerificationResultRequest(url string) (*http.Request, error) {
	logger := log.With().
		Str("section", "kyc").
		Str("action", "GetJumioIdVerificationResultRequest").
		Logger()

	logger.Error().Str("URL_request", url) // todo

	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(service.kyc.MerchantID, service.kyc.MerchantPassword)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Add("User-Agent", "ParamountDax Exchange")

	return req, err
}

// GetJumioIdVerificationResult - call the document verification result endpoint
func (service *Service) GetJumioIdVerificationResult(jumioScanReferenceId string) ([]byte, error) {

	requestURL := service.kyc.BuildAPIUrl("/scans/" + jumioScanReferenceId + "/data")

	request, err := service.getJumioIdVerificationResultRequest(requestURL)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	responseBody, err := ioutil.ReadAll(resp.Body)

	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	return responseBody, nil
}

func (service *Service) EncodeFileToBase64(fileHeader *multipart.FileHeader) (string, error) {
	openDoc, err := fileHeader.Open()
	if err != nil {
		return "", err
	}

	fileBytes, err := ioutil.ReadAll(openDoc)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(fileBytes), nil
}

// GetKycByID - get an kyc by a specific id
func (service *Service) GetKycByID(id *uint64) (*model.KYC, error) {
	kyc := model.KYC{}
	db := service.repo.Conn.Table("kycs").First(&kyc, "id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}
	return &kyc, nil
}

// GetUserByKycID - get an kyc by a specific id
func (service *Service) GetUserByKycID(tx *gorm.DB, id *uint64) (*model.User, error) {
	user := model.User{}
	db := tx.First(&user, "kyc_id = ?", id)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	return &user, nil
}

// GetBusinessMemberByKycID - get business member info by specified kyc id
func (service *Service) GetBusinessMemberByKycID(tx *gorm.DB, id *uint64) (*model.BusinessMembersSchema, error) {
	member := model.BusinessMembersSchema{}
	db := tx.Table("business_members").Where("kyc_id = ?", id).First(&member)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	return &member, nil
}

// GetKycByRefID - get an kyc by a specific id
func (service *Service) GetKycByRefID(tx *gorm.DB, id string) (*model.KYC, error) {
	kyc := model.KYC{}
	db := tx.First(&kyc, "reference_id = ? OR reference_id_2 = ?", id, id)
	if db.Error != nil {
		tx.Rollback()
		return nil, db.Error
	}
	return &kyc, nil
}

// AddKyc - add a new kyc in the database
func (service *Service) AddKyc(
	tx *gorm.DB,
	status model.KYCStatus) (*model.KYC, error) {

	kyc := model.NewKYC(status)
	db := tx.Table("kycs").Create(kyc)

	if db.Error != nil {
		return nil, db.Error
	}
	return kyc, nil
}

// AddKycToUser - add a new kyc in the database
func (service *Service) AddKycToUser(tx *gorm.DB, user *model.User, kycID uint64) (*model.User, error) {
	user.KycID = &kycID
	user.UpdatedAt = time.Now()

	db := tx.Table("users").Where("id = ?", user.ID).Save(user)
	if db.Error != nil {
		return nil, db.Error
	}
	return user, nil
}

func (service *Service) UpdateKycByCallback(tx *gorm.DB, kyc *model.KYC, user *model.User, response *model.ShuftiProKYCResponse) error {
	var kycStatus model.KYCStatus

	switch response.Event {
	case model.ShuftiProVerificationAccepted:
		kycStatus = model.KYCStatusStepTwoSuccess
		kyc.StepThreeResponse = string(response.VerificationResult)
	case model.ShuftiProVerificationDeclined:
		kycStatus = model.KYCStatusStepTwoFail
		kyc.StatusReason = response.DeclinedReason
		kyc.CallbackResponse = string(response.VerificationResult)
	case model.ShuftiProRequestError:
		kyc.StatusReason = response.Error.Message
	default:
		return errors.New("response invalid")
	}

	kyc, err := service.UpdateKyc(tx, kyc, kycStatus)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYCStatus(kycStatus))
	if err != nil {
		tx.Rollback()
		return err
	}

	// Get user details
	userDetails := model.UserDetails{}
	db := service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", user.ID)
	if db.Error != nil {
		return db.Error
	}

	// Add lead bonus if kyc approved
	kycDecision := model.KYCDecline
	if kyc.Status == model.KYCStatusStepTwoSuccess {
		if err := service.AddLeadBonusForKYC(tx, kyc, user, &userDetails); err != nil {
			tx.Rollback()
			return err
		}
		kycDecision = model.KYCApprove
	}

	// Send email to user about kyc verification
	apCode, _ := service.GetAntiPhishingCode(user.ID)
	if user.EmailStatus.IsAllowed() {
		err = service.KycNoticeStatus(user.Email, userDetails.Language.String(), kycDecision, apCode, userDetails.Timezone)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

func (service *Service) CheckShuftiproKYCResponse(response *model.ShuftiProKYCResponse) error {
	switch response.Event {
	case model.ShuftiProVerificationAccepted:
		return nil
	case model.ShuftiProVerificationDeclined:
		return errors.New(response.DeclinedReason)
	case model.ShuftiProRequestError:
		return errors.New(response.Error.Message)
	default:
		return errors.New("response invalid: " + string(response.Event) + string(response.VerificationResult))
	}
}

func (service *Service) UpdateKycOnStep2(kyc *model.KYC, result *model.ShuftiProKYCResponse, user *model.User) (*model.KYC, error) {
	status := model.KYCStatusStepTwoPending

	switch result.Event {
	case model.ShuftiProVerificationAccepted:
		kyc.StepTwoResponse = string(result.VerificationResult)
	case model.ShuftiProVerificationDeclined:
		status = model.KYCStatusStepTwoFail
		kyc.StatusReason = result.DeclinedReason
		kyc.StepTwoResponse = string(result.VerificationResult)
	case model.ShuftiProRequestError:
		kyc.StatusReason = result.Error.Message
	default:
		return nil, errors.New("response invalid")
	}

	tx := service.repo.Conn.Begin()

	kyc.ReferenceID = result.Reference

	updatedKyc, err := service.UpdateKyc(tx, kyc, status)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYCStatus(status))
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit().Error
	if err != nil {
		return nil, err
	}

	return updatedKyc, nil
}

func (service *Service) UpdateKycOnStep2BusinessMember(kyc *model.KYC, result *model.ShuftiProKYCResponse) (*model.KYC, error) {
	status := model.KYCStatusStepTwoPending

	switch result.Event {
	case model.ShuftiProVerificationAccepted:
		kyc.StepTwoResponse = string(result.VerificationResult)
	case model.ShuftiProVerificationDeclined:
		status = model.KYCStatusStepTwoFail
		kyc.StatusReason = result.DeclinedReason
		kyc.StepTwoResponse = string(result.VerificationResult)
	case model.ShuftiProRequestError:
		kyc.StatusReason = result.Error.Message
	default:
		return nil, errors.New("response invalid")
	}

	tx := service.repo.Conn.Begin()

	kyc.ReferenceID = result.Reference

	updatedKyc, err := service.UpdateKyc(tx, kyc, status)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.Commit().Error
	if err != nil {
		return nil, err
	}

	return updatedKyc, nil
}

func (service *Service) UpdateKycOnStep3(kyc *model.KYC, status model.KYCStatus, user *model.User) (*model.KYC, error) {
	tx := service.repo.Conn.Begin()

	updatedKyc, err := service.UpdateKyc(tx, kyc, status)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYCStatus(status))
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	return updatedKyc, nil
}

// UpdateKyc -
func (service *Service) UpdateKyc(tx *gorm.DB, kyc *model.KYC, status model.KYCStatus) (*model.KYC, error) {
	kyc.Status = status
	kyc.UpdatedAt = time.Now()

	db := tx.Table("kycs").Where("id = ?", kyc.ID).Save(kyc)

	return kyc, db.Error
}

// GetStatusByRegistrationID - returns KYCStatusStepOneSuccess or KYCStatusStepOneFail status
func (service *Service) GetStatusByRegistrationID(registrationID string) model.KYCStatus {
	var status model.KYCStatus
	if registrationID != "" {
		status = model.KYCStatusStepOneSuccess
	} else {
		status = model.KYCStatusStepOneFail
	}
	return status
}

// GetStatusByCallbackDecision - returns KYCStatusStepTwoFail or KYCStatusStepTwoTwoSuccess status
func (service *Service) GetStatusByCallbackDecision(decision string) (model.KYCCallbackStatus, model.KYCStatus) {
	if decision == "APPROVED_VERIFIED" {
		return model.KYCCallbackStatusSuccess, model.KYCStatusStepTwoSuccess
	} else {
		return model.KYCCallbackStatusError, model.KYCStatusStepTwoFail
	}
}

// GetStatusByReferenceID godoc
func (service *Service) GetStatusByReferenceID(referenceID string) model.KYCStatus {
	var status model.KYCStatus
	if referenceID != "" {
		status = model.KYCStatusStepTwoPending
	} else {
		status = model.KYCStatusStepTwoFail
	}
	return status
}

// UpdateUserLevel - put for user setting the level corresponding to the kyc status
func (service *Service) UpdateUserLevel(tx *gorm.DB, user *model.User, status int) error {
	userSettings, err := service.GetProfileSettings(user.ID)
	if err != nil {
		return err
	}
	_, err = service.UpdateUserSettings(tx, userSettings, userSettings.FeesPayedWithPrdx, userSettings.DetectIPChange, userSettings.AntiPhishingKey, userSettings.GoogleAuthKey, userSettings.SmsAuthKey, userSettings.TradePassword, status)
	if err != nil {
		return err
	}
	return nil
}

func (service *Service) AddKycToBusinessMember(tx *gorm.DB, memberID uint64, kycID uint64) error {
	err := tx.Table("business_members").Where("id = ?", memberID).Update("kyc_id", kycID).Error
	if err != nil {
		return err
	}

	return err
}

func (service *Service) GetKycIDByBusinessMemberID(memberID uint64) (uint64, error) {
	memberKycID, err := service.repo.GetKycIDByBusinessMemberID(memberID)
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	return memberKycID, err
}

func (service *Service) AddOrUpdateBusinessMemberKyc(status model.KYCStatus, memberID uint64) (*model.KYC, error) {
	tx := service.repo.Conn.Begin()

	var err error
	var data *model.KYC

	kycID, err := service.GetKycIDByBusinessMemberID(memberID)
	if err != nil {
		return nil, err
	}

	if kycID == 0 {
		data, err = service.AddKyc(tx, status)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = service.AddKycToBusinessMember(tx, kycID, memberID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		//return data, err
	} else {
		kyc, err := service.GetKycByID(&kycID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		data, err = service.UpdateKyc(tx, kyc, status)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return data, tx.Commit().Error
}

// AddOrUpdateKyc - add a kyc to the database for a user if it doesn't exist. if there is already a kyc for that user, update the record
func (service *Service) AddOrUpdateKyc(status model.KYCStatus, user *model.User) (*model.KYC, error) {
	var data *model.KYC
	var err error
	var KycID = user.KycID

	tx := service.repo.Conn.Begin()
	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYCStatus(status))
	if err != nil {
		tx.Rollback()
		return data, err
	}

	if KycID == nil {
		data, err = service.AddKyc(tx, status)
		if err != nil {
			tx.Rollback()
			return data, err
		}

		_, err = service.AddKycToUser(tx, user, data.ID)
		if err != nil {
			tx.Rollback()
			return data, err
		}
	} else {
		kyc, err := service.GetKycByID(KycID)
		if err != nil {
			tx.Rollback()
			return data, err
		}

		data, err = service.UpdateKyc(tx, kyc, status)
		if err != nil {
			tx.Rollback()
			return data, err
		}
	}

	return data, tx.Commit().Error
}

func (service *Service) UpdateKYCStatusTransaction(kyc *model.KYC, status model.KYCStatus) (*model.KYC, error) {
	tx := service.repo.Conn.Begin()
	defer tx.Commit()

	newKyc, err := service.UpdateKyc(tx, kyc, status)
	if err != nil {
		tx.Rollback()
		return kyc, err
	}

	return newKyc, nil
}

// GetUserLevelByKYCStatus - returns the user level based on KYC status
func (service *Service) GetUserLevelByKYCStatus(status model.KYCStatus) int {
	switch status {
	case "none":
		return 0
	case "step_one_success":
		return 1
	case "step_one_fail":
		return 0
	case "step_two_pending":
		return 1
	case "step_two_fail":
		return 1
	case "step_two_success":
		return 2
	case "step_three_pending":
		return 2
	case "step_three_success":
		return 3
	case "step_three_fail":
		return 2
	default:
		return 0
	}
}

func (service *Service) GetKYCStatusByUserLevel(level int, status model.KYCStatus) model.KYCStatus {
	switch level {
	case 0:
		return model.KYCStatusNone
	case 1:
		return model.KYCStatusStepOneSuccess
	case 2:
		return model.KYCStatusStepTwoSuccess
	case 3:
		return model.KYCStatusStepThreeSuccess
	default:
		return status
	}
}
func (service *Service) ServShuftiproKYCCallback(logger *zerolog.Logger, response *model.ShuftiProKYCResponse) {
	tx := service.GetRepo().Conn.Begin()

	userMutex := service.GetKYCMutexFromMap(response.Reference)
	userMutex.Lock()
	defer kycMutex.Delete(response.Reference)
	defer userMutex.Unlock()

	kyc, err := service.GetKycByRefID(tx, response.Reference)
	if err != nil {
		logger.Error().Str("ReferenceID", response.Reference).Err(err).Msg("Unable to find Kyc by RefId")
		return
	}

	user, err := service.GetUserByKycID(tx, &kyc.ID)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")
		}

		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to find user by KycId")

		return
	}

	err = service.UpdateKycByCallback(tx, kyc, user, response)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")

		}

		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to update kyc by callback")

		return
	}
}
