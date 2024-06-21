package actions

//@deprecated
//
///*
//* Copyright © 2006-2019 Around25 SRL <office@around25.com>
//*
//* Licensed under the Around25 Exchange License Agreement (the "License");
//* you may not use this file except in compliance with the License.
//* You may obtain a copy of the License at
//*
//*     http://www.around25.com/licenses/EXCHANGE_LICENSE
//*
//* Unless required by applicable law or agreed to in writing, software
//* distributed under the License is distributed on an "AS IS" BASIS,
//* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//* See the License for the specific language governing permissions and
//* limitations under the License.
//*
//* @author		Cosmin Harangus <cosmin@around25.com>
//* @copyright 2006-2019 Around25 SRL <office@around25.com>
//* @license 	EXCHANGE_LICENSE
//*/
//
//import (
//	"encoding/json"
//	"fmt"
//	"github.com/gin-gonic/gin"
//	"github.com/rs/zerolog/log"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
//	"net/http"
//	"strconv"
//	"strings"
//)
//
//// KYCCallbackResponse - define a struct for KYC Callback Response
//type KYCCallbackResponse struct {
//	Data          DataCallbackResponse `form:"data" json:"data" binding:"required"`
//	KycSource     string               `form:"kyc_source" json:"kyc_source" binding:"required"`
//	ReferenceID   string               `form:"reference_id" json:"reference_id" binding:"required"`
//	ScoreComplete int                  `form:"score_complete" json:"score_complete" binding:"required"`
//}
//
//// DataCallbackResponse  - define a struct for KYC Data Callback Response
//type DataCallbackResponse struct {
//	Decision       string `form:"decision" json:"decision" binding:"required"`
//	ControlsResult string `form:"controls_result" json:"controls_result" binding:"required"`
//}
//
//// KycCustomerRegistration - Returns the list of chains
//func (actions *Actions) KycCustomerRegistration(c *gin.Context) {
//	log := logger.GetLogger(c)
//
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
//
//	var data *model.KYC
//
//	var dataRequest model.KycCustomerRegistrationRequest
//
//	if err := c.ShouldBind(&dataRequest); err != nil {
//		log.Error().
//			Err(err).
//			Str("section", "kyc").
//			Str("action", "register").
//			Str("ip", ip).
//			Uint64("user_id", user.ID).
//			Msg("Unable to parse data for KYC provider")
//
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to register data with KYC service",
//		})
//		return
//	}
//
//	dataRequest.IP = ip
//	dataRequest.UserID = user.ID
//
//	result, err := actions.service.CustomerRegistration(&dataRequest)
//	if err != nil {
//		log.Error().
//			Err(err).
//			Str("section", "kyc").
//			Str("action", "register").
//			Str("ip", ip).
//			Uint64("user_id", user.ID).
//			Msg("Unable to register data with KYC provider")
//		c.AbortWithStatusJSON(500, map[string]string{
//			"error": "Unable to register data with KYC service",
//		})
//		return
//	}
//	if len(result) == 0 {
//		log.Error().
//			Str("section", "kyc").
//			Str("action", "register").
//			Str("ip", ip).
//			Uint64("user_id", user.ID).
//			Msg("Unable to register data with KYC provider. Invalid response")
//		c.AbortWithStatusJSON(500, map[string]string{
//			"error": "Something went wrong",
//		})
//		return
//	}
//	if result["description"] != "Success" {
//		log.Error().
//			Str("section", "kyc").
//			Str("action", "register").
//			Str("ip", ip).
//			Uint64("user_id", user.ID).
//			Msg("Unable to register data with KYC provider. Invalid response")
//		c.AbortWithStatusJSON(500, map[string]string{
//			"error": "Something went wrong",
//		})
//		return
//	}
//
//	registrationID := result["id"].(string)
//	stepOneResponseJ, _ := json.Marshal(result)
//	stepOneResponse := string(stepOneResponseJ)
//	status := actions.service.GetStatusByRegistrationID(registrationID)
//	data, err = actions.service.AddOrUpdateKyc(registrationID, stepOneResponse, status, user)
//	if err != nil {
//		log.Error().
//			Err(err).
//			Str("section", "kyc").
//			Str("action", "register").
//			Str("ip", ip).
//			Uint64("user_id", user.ID).
//			Str("kyc_registration_id", registrationID).
//			Msg("Unable to add or update KYC data for user")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to add or update kyc",
//		})
//		return
//	}
//
//	id := strconv.FormatUint(data.ID, 10)
//	_, err = actions.service.SendNotification(user.ID, model.NotificationType_Info,
//		model.NotificationTitle_KYCVerification.String(),
//		fmt.Sprintf(model.NotificationMessage_KYCVerification.String(), data.Status),
//		model.Notification_KYCVerification, id)
//
//	if err != nil {
//		abortWithError(c, ServerError, err.Error())
//		return
//	}
//
//	c.JSON(200, data)
//}
//
//// KycDocumentVerification verifies the KYC documents
//func (actions *Actions) KycDocumentVerification(c *gin.Context) {
//
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	form, _ := c.MultipartForm()
//	documents := form.File["document"]
//	documentType, _ := c.GetPostForm("document_type")
//	fileBucketCount := 1
//
//	l := log.With().
//		Str("section", "launchpad").
//		Str("action", "buy_launchpad").
//		Uint64("user_id", user.ID).
//		Uint64("kyc_id", *user.KycID).
//		Logger()
//
//	kycDocs := model.KYCDocuments{}
//	kycDocs.Documents = make([]model.KYCDocument, 1)
//	kycDocs.UserID = user.ID
//
//	for _, documentFile := range documents {
//		documentFile.Filename = strings.ToLower(documentFile.Filename)
//	}
//	kycDocs.Documents[0] = model.KYCDocument{Type: documentType, Files: documents}
//
//	if featureflags.IsEnabled("kyc.selfie") {
//		fileBucketCount = 2
//		selfie := form.File["selfie"]
//		for _, selfieFile := range selfie {
//			selfieFile.Filename = strings.ToLower(selfieFile.Filename)
//		}
//		kycDocs.Documents = append(kycDocs.Documents, model.KYCDocument{Type: "selfie", Files: selfie})
//	}
//
//	if len(kycDocs.Documents) != fileBucketCount {
//		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//			"error": "Invalid number of documents",
//		})
//
//		return
//	}
//
//	kyc, err := actions.service.GetKycByID(user.KycID)
//	if err != nil {
//		l.Error().Err(err).Msg("Unable to get kyc")
//		c.AbortWithStatusJSON(404, map[string]string{
//			"error": "Unable to get kyc",
//		})
//		return
//	}
//
//	kycDocs, err = actions.service.StepTwoUploadFiles(kycDocs, c)
//	if err != nil {
//		l.Error().Err(err).Msg("Unable to save uploaded files to /tmp folder")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to upload kyc files",
//		})
//		return
//	}
//
//	referenceIDs := [2]string{"", ""}
//	level2Step := uint64(0)
//	status := model.KYCStatusStepTwoPending
//	stepTwoResponse := ""
//
//	for i, kycDoc := range kycDocs.Documents {
//		response, err := actions.service.StepTwoDocumentsVerification(kycDocs.UserID, kyc.RegistrationID, kycDoc)
//		if err != nil {
//			l.Error().Err(err).Msg("Unable to send documents for verification")
//			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//				"error": "Unable to send documents for verification",
//			})
//			return
//		}
//
//		newReferenceID := response[0]["reference_id"].(string)
//		if len(newReferenceID) > 0 {
//			referenceIDs[i] = newReferenceID
//		} else {
//			status = model.KYCStatusStepTwoFail
//			level2Step = 2
//
//			stepTwoResponseJ, _ := json.Marshal(response)
//			stepTwoResponse = string(stepTwoResponseJ)
//
//			_, err := actions.service.UpdateKyc(kyc, kyc.RegistrationID, kyc.StepOneResponse, stepTwoResponse, kyc.CallbackResponse, referenceIDs[0], referenceIDs[1], status, level2Step)
//			if err != nil {
//				l.Error().Err(err).
//					Str("kyc_registration_id", kyc.RegistrationID).
//					Str("http_resp", stepTwoResponse).
//					Msg("Unable to update kyc profile")
//				c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//					"error": "Unable to update kyc",
//				})
//				return
//			}
//			l.Error().Err(err).
//				Str("kyc_registration_id", kyc.RegistrationID).
//				Str("http_resp", stepTwoResponse).
//				Msg(response[0]["description"].(string))
//			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//				"error": response[0]["description"].(string),
//			})
//			return
//		}
//
//		stepTwoResponseJ, _ := json.Marshal(response)
//		stepTwoResponse = string(stepTwoResponseJ)
//	}
//
//	if status == model.KYCStatusStepTwoPending {
//		apCode, _ := actions.service.GetAntiPhishingCode(user.ID)
//		_ = actions.service.KycNoticePending(user.Email, apCode)
//	}
//
//	kycU, err := actions.service.UpdateKyc(kyc, kyc.RegistrationID, kyc.StepOneResponse, stepTwoResponse, kyc.CallbackResponse, referenceIDs[0], referenceIDs[1], status, level2Step)
//	if err != nil {
//		l.Error().Err(err).
//			Str("kyc_registration_id", kyc.RegistrationID).
//			Str("http_resp", stepTwoResponse).
//			Msg("Unable to update kyc profile")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to update kyc",
//		})
//		return
//	}
//
//	_ = actions.service.UpdateUserLevel(user, status)
//	c.JSON(200, kycU)
//}
//
//// KycPaymentDocumentVerification -
//func (actions *Actions) KycPaymentDocumentVerification(c *gin.Context) {
//	log := logger.GetLogger(c)
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//
//	kyc, err := actions.service.GetKycByID(user.KycID)
//
//	if err != nil {
//		abortWithError(c, http.StatusBadRequest, "Wrong KycId")
//		return
//	}
//
//	status := model.KYCStatusStepThreePending
//
//	form, _ := c.MultipartForm()
//	documents := form.File["document"]
//
//	var totalSize int64
//	for _, file := range documents {
//		totalSize += file.Size
//	}
//
//	if totalSize > utils.MaxEmailFilesSize {
//		msg := fmt.Sprintf("Total size of attachements should be <= %s",
//			utils.HumaneFileSize(utils.MaxEmailFilesSize),
//		)
//		log.Error().
//			Str("section", "kyc").
//			Str("action", "payment_document_verification").
//			Uint64("user_id", user.ID).
//			Msg(msg)
//		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//			"error": msg,
//		})
//		return
//	}
//
//	err = actions.service.SendPaymentDocumentVerificationEmail(user.Email, user.LastName, documents)
//	if err != nil {
//		abortWithError(c, http.StatusBadRequest, "Unable to send request")
//		return
//	}
//
//	updatedKyc, errUpdate := actions.service.UpdateKyc(kyc, status)
//
//	if errUpdate != nil {
//		abortWithError(c, http.StatusBadRequest, "Unable to send request")
//		return
//	}
//	err = actions.service.UpdateUserLevel(user, status)
//
//	if err != nil {
//		abortWithError(c, http.StatusBadRequest, "Unable to update user level")
//		return
//	}
//
//	c.JSON(http.StatusOK, updatedKyc)
//}
//
//// KycCallback -
//func (actions *Actions) KycCallback(c *gin.Context) {
//	log := logger.GetLogger(c)
//	var response KYCCallbackResponse
//	_ = c.BindJSON(&response)
//	callbackResponse, _ := json.Marshal(response)
//	log.Info().Str("Body", string(callbackResponse)).Msg("KycCallback from 4Stop")
//
//	kyc, err := actions.service.GetKycByRefID(response.ReferenceID)
//	if err != nil {
//		log.Error().Str("ReferenceID", response.ReferenceID).Err(err).Msg("Unable to find Kyc by RefId")
//		c.AbortWithStatusJSON(404, map[string]string{
//			"error": "Unable to get kyc",
//		})
//		return
//	}
//
//	if kyc.Level2Step == 2 {
//		log.Error().Msg("KYC.Level2Step is already 2")
//		c.Status(200)
//		return
//	}
//
//	user, err := actions.service.GetUserByKycID(&kyc.ID)
//	if err != nil {
//		log.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to find user by KycId")
//		c.AbortWithStatusJSON(404, map[string]string{
//			"error": "Unable to get user for kyc ",
//		})
//		return
//	}
//
//	// Default case (with manual verification)
//	decision := response.Data.Decision
//	var statusFromCallback = decision
//	if len(decision) == 0 {
//		// case with MRZ verification
//		statusFromCallback = actions.service.GetDecisionFromMRZCallback(response.Data.ControlsResult)
//	}
//
//	status := actions.service.GetStatusByCallbackDecision(statusFromCallback)
//
//	if status == model.KYCStatusStepTwoFail {
//		kyc.Level2Step = 2
//	} else {
//		kyc.Level2Step++
//	}
//
//	_, err = actions.service.UpdateKyc(kyc, kyc.RegistrationID, kyc.StepOneResponse, kyc.StepTwoResponse, string(callbackResponse), kyc.ReferenceID1, kyc.ReferenceID2, status, kyc.Level2Step)
//	if err != nil {
//		log.Error().Uint64("KycID", kyc.ID).Uint64("UserID", user.ID).Err(err).Msg("Unable to update KYC")
//		c.AbortWithStatusJSON(404, map[string]string{
//			"error": "Unable to update kyc",
//		})
//		return
//	}
//
//	_ = actions.service.UpdateUserLevel(user, status)
//	apCode, _ := actions.service.GetAntiPhishingCode(user.ID)
//	if response.ScoreComplete == 1 {
//		_ = actions.service.KycNoticeStatus(user.Email, statusFromCallback, apCode)
//	}
//
//	c.Status(OK)
//}
//
//// KycGetStatusForAnUser - .
//func (actions *Actions) KycGetStatusForAnUser(c *gin.Context) {
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	kyc, err := actions.service.GetKycByID(user.KycID)
//	if err != nil {
//		c.AbortWithStatusJSON(404, map[string]string{
//			"error": "Unable to get kyc status",
//		})
//		return
//	}
//	c.JSON(200, kyc)
//}
