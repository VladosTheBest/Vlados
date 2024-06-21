package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/biter777/countries"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	awsutils "gitlab.com/paramountdax-exchange/exchange_api_v2/lib/aws"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"golang.org/x/sync/syncmap"

	"gorm.io/gorm"
)

var kybMutex syncmap.Map

func (service *Service) KYBBasicInformation(user *model.User, data model.KYBBusinessRegistration) error {
	kyb, err := service.repo.KYBSaveBasicInformation(data)
	if err != nil {
		return err
	}

	stepOneStatus := model.KYBStatusStepOneSuccess

	defer func() {
		tempErr := service.KYBUpdateStepOneStatus(kyb.ID, stepOneStatus)
		if tempErr != nil {
			err = tempErr
		}
	}()

	response, err := service.KYBRequest(data, kyb.ID)
	if err != nil {
		stepOneStatus = model.KYBStatusStepOneFail
		return err
	}

	if response.Event != model.ShuftiProVerificationAccepted {
		stepOneStatus = model.KYBStatusStepOneFail
		return errors.New("shuftipro verification status not accepted")
	}

	// check if response kyb has len > 0
	if len(response.VerificationData.Kyb) == 0 {
		stepOneStatus = model.KYBStatusStepOneFail
		return errors.New("shuftipro verification no such company")
	}

	shuftiproResultBytes, err := json.Marshal(response.VerificationData.Kyb)
	if err != nil {
		stepOneStatus = model.KYBStatusStepOneFail
		return err
	}

	if err := service.repo.KYBSaveShuftiProResponse(shuftiproResultBytes, response.VerificationResult.KYBService, kyb.ID, stepOneStatus); err != nil {
		stepOneStatus = model.KYBStatusStepOneFail
		return err
	}
	tx := service.repo.Conn.Begin()

	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYBStatus(kyb.Status))
	if err != nil {
		stepOneStatus = model.KYBStatusStepOneFail
		tx.Rollback()
		return err
	}

	tx.Commit()

	return nil
}

// GetUserLevelByKYCStatus - returns the user level based on KYC status
func (service *Service) GetUserLevelByKYBStatus(status model.KYBStatus) int {
	switch status {
	case model.KYBStatusSuccess:
		return 3
	default:
		return 0
	}
}

func (service *Service) GetKYBByUserID(userID uint64) (*model.KYBSchema, error) {
	kybID, err := service.GetKYBIDByUserID(userID)
	if kybID == 0 {
		defaultKyb := model.NewKYBSchema()
		return &defaultKyb, nil
	} else if err != nil {
		return nil, err
	}

	kyb, err := service.repo.GetKYBByID(kybID)
	if err == gorm.ErrRecordNotFound {
		defaultKyb := model.NewKYBSchema()
		return &defaultKyb, nil
	} else if err != nil {
		return nil, err
	}

	return kyb, err
}

func (service *Service) KYBRequest(data model.KYBBusinessRegistration, id uint64) (*model.ShuftiProKYBResponse, error) {

	country := countries.ByName(data.OperatingBusinessCountry)

	request := model.ShuftiProKYBRequest{
		ShuftiProAuthRequest: model.ShuftiProAuthRequest{
			Reference:        "pdax1_kyb_" + strconv.FormatUint(id, 10) + "_company_" + strconv.FormatUint(data.ID, 10) + "_" + strconv.FormatInt(time.Now().Unix(), 10),
			CallbackURL:      service.cfg.Server.KYB.CallbackURL,
			Country:          country.Alpha2(),
			Language:         "EN",
			Email:            service.cfg.Server.KYB.CompanyEmail,
			VerificationMode: "any",
		},
		KYB: model.KYBCompanyName{
			CompanyName: data.BusinessName,
		},
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", service.cfg.Server.KYB.ShuftiproURL, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Accept", "application/json")
	req.Host = "api.shuftipro.com"
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(service.cfg.Server.KYB.ClientID, service.cfg.Server.KYB.SecretKey)
	req.Header.Add("User-Agent", "ParamountDax Exchange")
	req.ContentLength = int64(len(requestJSON))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New(err.Error() + " " + string(requestJSON) + service.cfg.Server.KYB.ShuftiproURL) //todo: remove logs
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response model.ShuftiProKYBResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// Check shuftipro error
	if response.Error.Message != "" {
		return nil, errors.New("ShuftiProError: " + response.Error.Message)
	}

	return &response, nil
}

func (service *Service) GetKYBIDByUserID(userID uint64) (uint64, error) {
	// Get kyb_id by user_id
	kybID, err := service.repo.GetKYBIDByUserId(userID)
	if err != nil {
		return 0, err
	}

	return kybID, err
}

func (service *Service) GetUserBusinessDetailsByUserID(userID uint64) (*model.UserBusinessDetailsSchema, error) {
	details, err := service.repo.GetUserBusinessDetailsByUserID(userID)
	if err != nil {
		return nil, err
	}

	return details, err
}

func (service *Service) UpdateUserBusinessDetails(details *model.UserBusinessDetailsSchema) error {
	err := service.repo.UpdateUserBusinessDetails(details)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) KYBUpdateStepOneStatus(kybID uint64, status model.KYBStatus) error {
	// Update KYB step two status
	err := service.repo.KYBUpdateStepOneStatus(kybID, status)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) KYBUpdateStepTwoStatus(kybID uint64, status model.KYBStatus) error {
	// Update KYB step two status
	err := service.repo.KYBUpdateStepTwoStatus(kybID, status)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) KYBUpdateStepThreeStatus(kybID uint64, status model.KYBStatus) error {
	// Update KYB step two status
	err := service.repo.KYBUpdateStepThreeStatus(kybID, status)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) KYBUpdateVerifyBusinessMemberStepStatus(kybID uint64, statusArr []model.KYBStatus) error {
	var err error
	for _, status := range statusArr {
		switch status {
		case model.KYBStatusStepThreeFail, model.KYBStatusStepThreePending, model.KYBStatusStepThreeSuccess:
			err = service.KYBUpdateStepThreeStatus(kybID, status)
		case model.KYBStatusStepFourFail, model.KYBStatusStepFourPending, model.KYBStatusStepFourSuccess:
			err = service.KYBUpdateStepFourStatus(kybID, status)
		default:
			return errors.New("invalid status")
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (service *Service) KYBUpdateStepFourStatus(kybID uint64, status model.KYBStatus) error {
	// Update KYB step two status
	err := service.repo.KYBUpdateStepFourStatus(kybID, status)
	if err != nil {
		return err
	}

	return nil
}

func (service *Service) GetKYBMutexFromMap(id string) *sync.Mutex {
	mutex := &sync.Mutex{}
	tmpMutex, ok := kybMutex.Load(id)
	if !ok {
		kybMutex.Store(id, mutex)
		return mutex
	}

	return tmpMutex.(*sync.Mutex)
}

func (service *Service) GetKYBStatusesByUserID(userID uint64) (*model.KYBSchema, error) {
	logger := log.With().
		Str("section", "kyc").
		Str("action", "KycDocumentVerification").
		Uint64("user_id", userID).
		Logger()

	kyb, err := service.GetKYBByUserID(userID)

	if err != nil {
		logger.Error().Err(err).Msg("GetKYBError: can not get kyb")
		return nil, err
	}

	return kyb, nil
}

func (service *Service) ServShuftiproKYBStepThreeFourCallback(response *model.ShuftiProKYCResponse, logger *zerolog.Logger) {
	tx := service.GetRepo().Conn.Begin()

	userMutex := service.GetKYBMutexFromMap(response.Reference)

	userMutex.Lock()
	defer kybMutex.Delete(response.Reference)
	defer userMutex.Unlock()

	kyc, err := service.GetKycByRefID(tx, response.Reference)
	if err != nil {
		logger.Error().Str("ReferenceID", response.Reference).Err(err).Msg("Unable to find Kyc by RefId")
		return
	}

	member, err := service.GetBusinessMemberByKycID(tx, &kyc.ID)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status of business member")
			return
		}

		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to find business member by KycId")
		return
	}

	// Get business kyb
	kyb, err := service.GetKybByUserBusinessDetailsID(member.BusinessDetailsID)
	if err != nil {
		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to get business member`s kyb")

		return
	}

	var stepStatus []model.KYBStatus

	// Update status with step status when at the end of the method
	defer func() {
		tempErr := service.KYBUpdateVerifyBusinessMemberStepStatus(kyb.ID, stepStatus)
		if tempErr != nil {
			err = tempErr
			logger.Error().Err(err).Msg("can not update step two status")
		}
	}()

	// Update business member kyc status
	err = service.UpdateBusinessMemberKycByCallback(tx, kyc, response)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(member.BusinessRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")

			return
		}

		_, updateErr := service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")
			return
		}

		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to update kyc by callback")
		return
	}

	// Update kyb for business related to this member
	err = service.UpdateBusinessKYBStatus(member.BusinessDetailsID, member.BusinessRole, kyb)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(member.BusinessRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")

			return
		}

		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to update kyb status")

		return
	}
}

//
//func (service *Service) KYBSaveBusinessMemberInfo(userID uint64, memberInfo *model.KYBMemberRegistration) (*model.BusinessMembersSchema, error) {
//	oldMemberData, err := service.repo.GetBusinessMemberByUniqueFields(memberInfo.Email, memberInfo.FirstName, memberInfo.LastName)
//	if err != nil {
//		return nil, err
//	}
//
//	// TODO: add active/inactive business member logic
//
//	memberData := model.BusinessMembersSchema{
//		BusinessRole:         memberInfo.MemberRole,
//		FirstName:            memberInfo.FirstName,
//		LastName:             memberInfo.LastName,
//		MiddleName:           memberInfo.MiddleName,
//		DOB:                  memberInfo.DOB,
//		Gender:               memberInfo.Gender,
//		Country:              memberInfo.Country,
//		IdentificationNumber: memberInfo.IdentificationNumber,
//		Email:                memberInfo.Email,
//	}
//
//	tx := service.repo.Conn.Begin()
//
//	var businessDetailsID uint64
//	var memberKyc *model.KYC
//
//	if oldMemberData.ID == 0 {
//		// Find user_business_details by userID
//		businessDetailsID, err = service.repo.GetUserBusinessDetailsIDByUserID(userID)
//		if err != nil || businessDetailsID == 0 {
//			return nil, err
//		}
//
//		memberKyc, err = service.AddKyc(tx, model.KYCStatusStepOneSuccess)
//		if err != nil {
//			tx.Rollback()
//			return nil, err
//		}
//
//		// Form data to save in db
//		memberData.BusinessDetailsID = businessDetailsID
//		memberData.KycID = memberKyc.ID
//
//	} else {
//		// if already exists with role director and want to be registered as ubos than update role to ubosdirectore
//		if oldMemberData.BusinessRole == model.Director && memberInfo.MemberRole == model.UBOs {
//			memberData.BusinessRole = model.UBOSDirector
//		}
//
//		memberData.BusinessDetailsID = oldMemberData.BusinessDetailsID
//		memberData.KycID = oldMemberData.KycID
//		memberData.ID = oldMemberData.ID
//	}
//
//	// Update/Create new record in business_members table
//	err = service.repo.UpdateOrCreateUserBusinessMembersTx(tx, &memberData)
//	if err != nil {
//		tx.Rollback()
//		return nil, err
//	}
//
//	return &memberData, tx.Commit().Error
//}

func (service *Service) KYBSaveBusinessMemberInfo(userID uint64, memberInfo *model.KYBMemberRegistration) (*model.BusinessMembersSchema, error) {
	// Get business details id for this member
	businessDetailsID, err := service.repo.GetUserBusinessDetailsIDByUserID(userID)
	if err != nil || businessDetailsID == 0 {
		return nil, err
	}

	// Check if entered member is the latest update
	isLatestTimestamp, err := service.repo.IsLatestTimestamp(memberInfo.Timestamp, memberInfo.MemberRole, businessDetailsID)
	if err != nil {
		return nil, err
	}

	if isLatestTimestamp {
		tx := service.repo.Conn.Begin()

		memberKyc, err := service.AddKyc(tx, model.KYCStatusStepOneSuccess)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		memberData := model.BusinessMembersSchema{
			KycID:                memberKyc.ID,
			BusinessDetailsID:    businessDetailsID,
			IsActive:             true,
			BusinessRole:         memberInfo.MemberRole,
			FirstName:            memberInfo.FirstName,
			LastName:             memberInfo.LastName,
			MiddleName:           memberInfo.MiddleName,
			Timestamp:            memberInfo.Timestamp,
			DOB:                  memberInfo.DOB,
			Gender:               memberInfo.Gender,
			Country:              memberInfo.Country,
			IdentificationNumber: memberInfo.IdentificationNumber,
			Email:                memberInfo.Email,
		}

		logger := log.With().
			Str("section", "kyc").
			Str("action", "register").
			Logger()

		oldMemberData, err := service.repo.GetBusinessMemberByUniqueFields(memberInfo.Email, memberInfo.FirstName, memberInfo.LastName, businessDetailsID)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		logger.Debug().Msg(strconv.FormatUint(oldMemberData.Timestamp, 10) + "  " + strconv.FormatUint(memberData.Timestamp, 10))

		if oldMemberData.Timestamp == memberData.Timestamp {
			tx.Rollback()
			return nil, errors.New("two same directors records provided")
		}
		// If old role is different and eq director from entered then set new role as ubosdirector
		if oldMemberData.BusinessRole != memberInfo.MemberRole && oldMemberData.BusinessRole != "" {
			if oldMemberData.BusinessRole == model.Director {
				memberData.BusinessRole = model.UBOSDirector
			}

			oldMemberData.IsActive = false
			err = service.repo.UpdateOrCreateUserBusinessMembersTx(tx, oldMemberData)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		if memberInfo.MemberRole == model.UBOSDirector {
			err = service.repo.DeactivateBusinessMembersByTimestamp(tx, businessDetailsID, model.Director, memberInfo.Timestamp)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			err = service.repo.DeactivateBusinessMembersByTimestamp(tx, businessDetailsID, model.UBOs, memberInfo.Timestamp)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			err = service.repo.DeactivateBusinessMembersByTimestamp(tx, businessDetailsID, memberInfo.MemberRole, memberInfo.Timestamp)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		err = service.repo.UpdateOrCreateUserBusinessMembersTx(tx, &memberData)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		tx.Commit()

		return &memberData, err
	} else {
		return nil, nil
	}
}

func (service *Service) UpdateBusinessMemberKycByCallback(tx *gorm.DB, kyc *model.KYC, response *model.ShuftiProKYCResponse) error {
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

	_, err = service.UpdateKyc(tx, kyc, kycStatus)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// Get business kyb
func (service *Service) GetKybByUserBusinessDetailsID(businessDetailsID uint64) (*model.KYBSchema, error) {
	kyb, err := service.repo.GetKybByUserBusinessDetailsID(businessDetailsID)
	if err != nil {
		return nil, err
	}

	return kyb, err
}

func (service *Service) UpdateBusinessKYBStatus(businessDetailsID uint64, role model.BusinessRole, kyb *model.KYBSchema) error {
	// Get all business members with specific role
	membersArray, err := service.repo.GetBusinessMembers(businessDetailsID, role)
	if err != nil {
		return err
	}

	stepStatus := "none"
	for _, member := range membersArray {
		// Get directors kyc
		memberKyc, err := service.repo.GetKycByID(member.KycID)
		if err != nil {
			return err
		}

		// Check status of kyc
		switch memberKyc.Status {
		case model.KYCStatusStepThreeFail, model.KYCStatusStepTwoFail, model.KYCStatusStepOneFail:
			stepStatus = "fail"
			kyb.Status = model.KYBStatusFail
		case model.KYCStatusStepThreeSuccess:
			if stepStatus != "pending" {
				stepStatus = "success"
			}
			continue
		default:
			stepStatus = "pending"
			continue
		}

		break
	}

	switch role {
	case model.Director:
		kyb.StepThreeStatus = model.NewKYBStatus("step_three_" + stepStatus)
	case model.UBOs:
		kyb.StepFourStatus = model.NewKYBStatus("step_four_" + stepStatus)
	case model.UBOSDirector:
		kyb.StepThreeStatus = model.NewKYBStatus("step_three_" + stepStatus)
		kyb.StepFourStatus = model.NewKYBStatus("step_four_" + stepStatus)
	}

	// If all steps are successful then kyb general status - success
	if kyb.StepOneStatus == model.KYBStatusStepOneSuccess &&
		kyb.StepTwoStatus == model.KYBStatusStepTwoSuccess &&
		kyb.StepThreeStatus == model.KYBStatusStepThreeSuccess &&
		kyb.StepFourStatus == model.KYBStatusStepFourSuccess {
		kyb.Status = model.KYBStatusSuccess
	}

	// Save kyb step three status in DB
	err = service.repo.UpdateKyb(kyb)
	if err != nil {
		return err
	}

	user, err := service.GetUserByUserBusinessDetails(businessDetailsID)
	if err != nil {
		return err
	}

	tx := service.repo.Conn.Begin()

	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYBStatus(kyb.Status))
	if err != nil {
		tx.Rollback()
		return err
	}

	tx.Commit()

	return nil
}

func (service *Service) DoKYBVerifyBusinessMember(user *model.User, logger *zerolog.Logger, businessMember *model.KYBMemberRegistration) {
	// Update kyb status
	kybID, err := service.GetKYBIDByUserID(user.ID)
	if err != nil {
		logger.Error().Err(err).Msg("unable to get kyb id from user business details")
		return
	}

	// Set default step status according to the member`s role
	stepStatus := []model.KYBStatus{model.KYBStatusNone}

	stepStatus, err = service.SetVerifyMemberStepStatusPendingByRole(businessMember.MemberRole)
	if err != nil {
		logger.Error().Err(err).Msg("invalid member role")
		return
	}

	err = service.KYBUpdateVerifyBusinessMemberStepStatus(kybID, stepStatus)
	if err != nil {
		logger.Error().Err(err).Msg("can not update kyb status")
		return
	}

	// Update status with step status when at the end of the method
	defer func() {
		tempErr := service.KYBUpdateVerifyBusinessMemberStepStatus(kybID, stepStatus)
		if tempErr != nil {
			err = tempErr
			logger.Error().Err(err).Msg("can not update step two status")
		}
	}()

	// Save or update director/UBO`s info
	member, err := service.KYBSaveBusinessMemberInfo(user.ID, businessMember)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Err(err).Msg("KYBError: can not save business member info")

		return
	}

	// If got empty member than it doesn't actual anymore, don't have to pass kyc, return success
	if member == nil {
		return
	}

	// Prepare docs for kyc
	fileBucketCount := 2

	kycDocs := model.KYCDocuments{}
	kycDocs.Documents = make([]model.KYCDocument, 2)
	kycDocs.UserID = user.ID

	err = service.ValidateKYCDocuments(businessMember.DocumentImages)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(member.BusinessRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Msg(err.Error())

		return
	}

	//Validate and prepare document  front and back image
	documentTypeJumio := model.DetermineKYCDocumentType(businessMember.DocumentType)
	if documentTypeJumio == "" {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Msg("Empty document type")

		return
	}
	kycDocs.Documents[0] = model.KYCDocument{Type: documentTypeJumio, Files: businessMember.DocumentImages}

	//Validate and prepare selfie image
	err = service.ValidateKYCDocuments(businessMember.SelfieImage)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Msg(err.Error())

		return
	}
	kycDocs.Documents[1] = model.KYCDocument{Type: model.KycSelfie, Files: businessMember.SelfieImage}

	if len(kycDocs.Documents) != fileBucketCount {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Msg("Unable to get kyc. Incorrect document amount")

		return
	}
	memberKyc, err := service.AddOrUpdateBusinessMemberKyc(model.KYCStatusStepOneSuccess, member.ID)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Msg(err.Error())

		return
	}
	//Prepare business member info for kyc
	memberKYCInfo := model.KycCustomerInfo{
		KycID:      memberKyc.ID,
		UserID:     member.ID,
		FirstName:  businessMember.FirstName,
		LastName:   businessMember.LastName,
		Gender:     businessMember.Gender,
		DOB:        businessMember.DOB,
		Country:    businessMember.Country,
		PostalCode: businessMember.PostalCode,
	}

	memberKyc.ReferenceID = service.GenerateShuftiproKYCReferenceID(memberKyc.ID, user.ID)

	userMutex := service.GetKYBMutexFromMap(memberKyc.ReferenceID)
	userMutex.Lock()
	defer userMutex.Unlock()

	memberKyc, err = service.SaveShuftiproReferenceID(memberKyc)
	if err != nil {
		_, updateErr := service.UpdateKYCStatusTransaction(memberKyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")
		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")

		return
	}

	// Call kyc check for director/UBO`s
	responseBody, err := service.StepTwoDocumentsVerification(memberKyc.ReferenceID, &kycDocs, &memberKYCInfo, service.cfg.Server.KYB.CallbackURL)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		_, updateErr := service.UpdateKYCStatusTransaction(memberKyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")
		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")

		return
	}

	// Parse response from Jumio
	result, err := service.ParseJumioStepTwoCallback(responseBody)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")

			return
		}

		_, updateErr := service.UpdateKYCStatusTransaction(memberKyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")

		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")

		return
	}

	// Save business member reference id and upd kyc status
	_, err = service.UpdateKycOnStep2BusinessMember(memberKyc, result)
	if err != nil {
		stepStatus, err = service.SetVerifyMemberStepStatusFailByRole(businessMember.MemberRole)
		if err != nil {
			logger.Error().Err(err).Msg("invalid member role")
			return
		}

		logger.Error().Err(err).
			Str("http_resp", string(responseBody)).
			Msg("Unable to update kyc profile")

		return
	}

}

func (service *Service) SetVerifyMemberStepStatusFailByRole(role model.BusinessRole) ([]model.KYBStatus, error) {
	var statusArray []model.KYBStatus

	switch role {
	case model.Director:
		statusArray = append(statusArray, model.KYBStatusStepThreeFail)
	case model.UBOs:
		statusArray = append(statusArray, model.KYBStatusStepFourFail)
	case model.UBOSDirector:
		statusArray = append(statusArray, model.KYBStatusStepThreeFail)
		statusArray = append(statusArray, model.KYBStatusStepFourFail)
	default:
		return nil, errors.New("invalid member role")
	}

	return statusArray, nil
}

func (service *Service) SetVerifyMemberStepStatusPendingByRole(role model.BusinessRole) ([]model.KYBStatus, error) {
	var statusArray []model.KYBStatus

	switch role {
	case model.Director:
		statusArray = append(statusArray, model.KYBStatusStepThreePending)
	case model.UBOs:
		statusArray = append(statusArray, model.KYBStatusStepFourPending)
	case model.UBOSDirector:
		statusArray = append(statusArray, model.KYBStatusStepThreePending)
		statusArray = append(statusArray, model.KYBStatusStepFourPending)
	default:
		return nil, errors.New("invalid member role")
	}

	return statusArray, nil
}

func (service *Service) UpdateKYBStepTwoStatusByAdmin(status string, userID uint64) error {
	logger := log.With().
		Str("section", "kyc").
		Str("action", "DownloadKYB").
		Logger()

	kyb, err := service.GetKYBByUserID(userID)
	if err != nil {
		logger.Error().Err(err).Msg("can not get kyb by user id")
		return err
	}

	// create kyb status from received string and assign to kyb step two status
	kybStatus := model.NewKYBStatus(status)
	switch kybStatus {
	case model.KYBStatusStepTwoSuccess:
		// If success than check if general status is success
		kyb.StepTwoStatus = kybStatus
		if kyb.StepOneStatus == model.KYBStatusStepOneSuccess &&
			kyb.StepTwoStatus == model.KYBStatusStepTwoSuccess &&
			kyb.StepThreeStatus == model.KYBStatusStepThreeSuccess &&
			kyb.StepFourStatus == model.KYBStatusStepFourSuccess {
			kyb.Status = model.KYBStatusSuccess
		}
	case model.KYBStatusStepTwoFail:
		// If fail than change general status and step two status to fail
		kyb.StepTwoStatus = kybStatus
		kyb.Status = model.KYBStatusFail
	default:
		logger.Error().Err(err).Msg("Invalid status received, got" + kybStatus.ToString())
		return errors.New("invalid kyb status received")
	}

	user, err := service.GetUserByUserID(userID)
	if err != nil {
		logger.Error().Err(err).Msg("can not get user by id")
		return err
	}

	// Update kyb
	err = service.repo.UpdateKyb(kyb)
	if err != nil {
		logger.Error().Err(err).Msg("can not update kyb")
		return err
	}

	tx := service.repo.Conn.Begin()
	// Update user lvl depending on general status
	err = service.UpdateUserLevel(tx, user, service.GetUserLevelByKYBStatus(kyb.Status))
	if err != nil {
		tx.Rollback()
		logger.Error().Err(err).Msg("can not update user lvl")
		return err
	}
	tx.Commit()

	return nil
}

func (service *Service) GenerateFilenameForKYB(fileType, businessRegistrationNumber, extension string) string {
	return businessRegistrationNumber + "_" + fileType + extension
}

func (service *Service) GetFilenameKYB(fileType string, businessDetails *model.UserBusinessDetailsSchema) (string, error) {
	switch fileType {
	case "certificate_of_incorporation":
		return businessDetails.CertificateOfIncorporationName, nil
	case "memorandum_article_association":
		return businessDetails.MemorandumArticleOfAssociationName, nil
	case "register_directors":
		return businessDetails.RegisterDirectorsName, nil
	case "register_of_members":
		return businessDetails.RegisterOfMembers, nil
	case "ownership_structure":
		return businessDetails.OwnershipStructure, nil
	case "sanctions_questionnaire":
		return businessDetails.SanctionsQuestionnaire, nil
	default:
		return "", errors.New("unknown doc")
	}
}

func (service *Service) DownloadKYBFileByAdmin(userID uint64, fileType string) (string, error) {
	logger := log.With().
		Str("section", "KYBAdmin").
		Str("action", "download_file").
		Logger()

	businessDetails, err := service.GetUserBusinessDetailsByUserID(userID)
	if err != nil {
		logger.Error().Err(err).Msg("can not get business details by id")
		return "", err
	}

	logger.Debug().Msg(fileType)

	fileName, err := service.GetFilenameKYB(fileType, businessDetails)
	if err != nil {
		logger.Error().Err(err).Msg("can not get filename")
		return "", err
	}

	logger.Debug().Msg(fileName)

	// The session the S3 Uploader will use
	sess, err := awsutils.CreateSession(&service.cfg.AWS.Session)
	if err != nil {
		logger.Error().Err(err).Msg("can not create s3 session")
		return "", err
	}

	url, err := awsutils.GetPresignURL(fileName, sess, &service.cfg.AWS.Bucket)
	if err != nil {
		logger.Error().Err(err).Msg("can not presign url")
		return "", err
	}

	return url, nil
}
