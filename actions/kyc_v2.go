package actions

//
//import (
//	"encoding/json"
//	"fmt"
//	"io/ioutil"
//	"mime/multipart"
//	"net/http"
//	"strconv"
//	"strings"
//
//	"github.com/gin-gonic/gin"
//	"github.com/rs/zerolog/log"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
//	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
//)
//
//func (actions *Actions) KycCustomerRegistration(c *gin.Context) {
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
//	logger := log.With().
//		Str("section", "kyc").
//		Str("action", "register").
//		Uint64("user_id", user.ID).
//		Str("ip", ip).
//		Logger()
//
//	var data *model.KYC
//
//	var dataRequest model.KycCustomerRegistrationRequest
//
//	if err := c.ShouldBind(&dataRequest); err != nil {
//		logger.Error().Err(err).Msg("Unable to parse data for KYC provider")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to register data with KYC service",
//		})
//		return
//	}
//	dataRequest.IP = ip
//	dataRequest.UserID = user.ID
//
//	responseBody, err := actions.service.CustomerRegistration(&dataRequest)
//	if err != nil {
//		logger.Error().Err(err).Msg("Unable to register data with KYC provider")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to register data with KYC service",
//		})
//		return
//	}
//
//	if len(responseBody) == 0 {
//		logger.Error().Msg("Unable to register data with KYC provider. Invalid response")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Something went wrong",
//		})
//		return
//	}
//
//	result := new(model.StepOneKycResponse)
//
//	err = json.Unmarshal(responseBody, result)
//	if err != nil {
//		logger.Error().Err(err).Msg("Unable to parse data from KYC provider")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to parse data from KYC provider",
//		})
//		return
//	}
//
//	if !result.IsSuccess() {
//		logger.Error().Msg("Unable to register data with KYC provider. Invalid response")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Something went wrong",
//		})
//		return
//	}
//
//	var status model.KYCStatus
//	if !result.IsRegistrationApproved() {
//		status = model.KYCStatusStepOneFail
//	} else {
//		status = model.KYCStatusStepOneSuccess
//	}
//
//	data, err = actions.service.AddOrUpdateKyc(result.Id, string(responseBody), status, user)
//	if err != nil {
//		logger.Error().Err(err).Str("kyc_registration_id", result.Id).Msg("Unable to add or update KYC data for user")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to add or update kyc",
//		})
//		return
//	}
//
//	id := strconv.FormatUint(data.ID, 10)
//	_, err = actions.service.SendNotification(user.ID, model.NotificationType_Info,
//		model.NotificationTitle_KYCVerification.String(),
//		fmt.Sprintf(model.NotificationMessage_KYCVerification.String(), actions.service.GetUserLevelByKYCStatus(data.Status)),
//		model.Notification_KYCVerification, id)
//
//	if err != nil {
//		abortWithError(c, ServerError, err.Error())
//		return
//	}
//
//	c.JSON(http.StatusOK, data)
//}
//
//func (actions *Actions) validateDocumentMimeTypes(header *multipart.FileHeader) error {
//	file, err := header.Open()
//	if err != nil {
//		return err
//	}
//	fileHeader := make([]byte, 512)
//	if _, err := file.Read(fileHeader); err != nil {
//		return err
//	}
//
//	mimeType := http.DetectContentType(fileHeader)
//
//	switch mimeType {
//	case "image/jpeg":
//	case "image/jpg":
//	case "image/png":
//	case "application/pdf":
//	case "application/msword":
//	case "application/octet-stream":
//	case "image/gif":
//	case "image/bmp":
//	default:
//		errMessage := fmt.Errorf("image type %s not allowed", mimeType)
//
//		return errMessage
//	}
//
//	return nil
//}
//
//func (actions *Actions) KycDocumentVerification(c *gin.Context) {
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	form, _ := c.MultipartForm()
//	documents := form.File["document"]
//	documentType, _ := c.GetPostForm("document_type")
//	fileBucketCount := 1
//
//	logger := log.With().
//		Str("section", "kyc").
//		Str("action", "KycDocumentVerification").
//		Uint64("user_id", user.ID).
//		Uint64("kyc_id", *user.KycID).
//		Logger()
//
//	kycDocs := model.KYCDocuments{}
//	kycDocs.Documents = make([]model.KYCDocument, 1)
//	kycDocs.UserID = user.ID
//
//	for _, documentFile := range documents {
//		err := actions.validateDocumentMimeTypes(documentFile)
//		if err != nil {
//			logger.Error().Msg(err.Error())
//			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//				"error": err.Error(),
//			})
//			return
//		}
//		documentFile.Filename = strings.ToLower(documentFile.Filename)
//		if documentFile.Size > utils.MaxKYCFilesSize {
//			msg := fmt.Sprintf("Size of attachements should be <= %s",
//				utils.HumaneFileSize(utils.MaxKYCFilesSize),
//			)
//			logger.Error().Msg(msg)
//			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//				"error": msg,
//			})
//			return
//		}
//	}
//
//	kycDocs.Documents[0] = model.KYCDocument{Type: documentType, Files: documents}
//	if featureflags.IsEnabled("kyc.selfie") {
//		fileBucketCount = 2
//		selfieFiles := form.File["selfie"]
//		for _, selfieFile := range selfieFiles {
//			err := actions.validateDocumentMimeTypes(selfieFile)
//			if err != nil {
//				logger.Error().Msg(err.Error())
//				c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//					"error": err.Error(),
//				})
//				return
//			}
//			selfieFile.Filename = strings.ToLower(selfieFile.Filename)
//			if selfieFile.Size > utils.MaxKYCFilesSize {
//				msg := fmt.Sprintf("Size of attachements should be <= %s",
//					utils.HumaneFileSize(utils.MaxKYCFilesSize),
//				)
//				logger.Error().Msg(msg)
//				c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//					"error": msg,
//				})
//				return
//			}
//		}
//		selfieFiles = append(selfieFiles, documents[0])
//		kycDocs.Documents = append(kycDocs.Documents, model.KYCDocument{Type: "selfie", Files: selfieFiles})
//	}
//
//	if len(kycDocs.Documents) != fileBucketCount {
//		logger.Error().Msg("Unable to get kyc")
//		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//			"error": "Invalid number of documents",
//		})
//
//		return
//	}
//
//	kyc, err := actions.service.GetKycByID(user.KycID)
//	if err != nil {
//		logger.Error().Err(err).Msg("Unable to get kyc")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to get kyc",
//		})
//		return
//	}
//
//	referenceIDs := [2]string{"", ""}
//	status := model.KYCStatusStepTwoPending
//
//	responses := make([]string, len(kycDocs.Documents))
//
//	for i, kycDocument := range kycDocs.Documents {
//		responseBody, err := actions.service.StepTwoDocumentsVerification(kycDocs.UserID, kyc.RegistrationID, kycDocument)
//		if err != nil {
//			logger.Error().Err(err).Msg("Unable to send documents for verification")
//			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//				"error": "Unable to send documents for verification",
//			})
//			return
//		}
//
//		if len(responseBody) == 0 {
//			logger.Error().Msg("Unable to send documents for verification")
//			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//				"error": "Unable to send documents for verification",
//			})
//			return
//		}
//		result := new([]model.StepTwoKycResponse)
//		err = json.Unmarshal(responseBody, result)
//		if err != nil {
//			logger.Error().Err(err).Msg("Unable to parse data from KYC provider")
//			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//				"error": "Unable to parse data from KYC provider",
//			})
//			return
//		}
//
//		kycResponse := (*result)[0]
//		if !kycResponse.IsSuccess() {
//			logger.Error().Msg("Unable to register data with KYC provider. Invalid response")
//			status = model.KYCStatusStepTwoFail
//		}
//		if len(kycResponse.ReferenceId) == 0 {
//			logger.Error().Msg("Unable to register data with KYC provider. Invalid response")
//			status = model.KYCStatusStepTwoFail
//		}
//		responses[i] = string(responseBody)
//		referenceIDs[i] = kycResponse.ReferenceId
//	}
//
//	if status == model.KYCStatusStepTwoPending {
//		if user.EmailStatus.IsAllowed() {
//			apCode, _ := actions.service.GetAntiPhishingCode(user.ID)
//			userDetails := model.UserDetails{}
//			db := actions.service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", user.ID)
//			if db.Error != nil {
//				logger.Error().Err(db.Error).
//					Str("kyc_registration_id", kyc.RegistrationID).
//					Msg("Unable to update kyc profile")
//			}
//			err = actions.service.KycNoticePending(user.Email, userDetails.Language.String(), apCode, userDetails.Timezone)
//			logger.Error().Err(err).
//				Str("kyc_registration_id", kyc.RegistrationID).
//				Msg("Unable to update kyc profile")
//		}
//	}
//
//	kycU, err := actions.service.UpdateKycOnStep2(kyc, status, user, responses, referenceIDs)
//
//	if err != nil {
//		logger.Error().Err(err).
//			Str("kyc_registration_id", kyc.RegistrationID).
//			Str("http_resp", responses[0]).
//			Str("http_resp", responses[1]).
//			Msg("Unable to update kyc profile")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to update kyc",
//		})
//		return
//	}
//
//	c.JSON(http.StatusOK, kycU)
//}
//
//// KycCallback -
//func (actions *Actions) KycCallback(c *gin.Context) {
//	logger := log.With().Str("section", "kyc").Str("action", "KycCallback").Logger()
//	var response model.KYCCallbackResponse
//
//	callbackResponse, err := ioutil.ReadAll(c.Request.Body)
//	if err != nil {
//		logger.Error().Err(err).Msg("Unable to parse callback")
//	}
//	log.Info().Str("Body", string(callbackResponse)).Msg("KycCallback from 4Stop")
//
//	err = json.Unmarshal(callbackResponse, &response)
//	if err != nil {
//		logger.Error().Err(err).Msg("Unable to parse callback")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to parse callback",
//		})
//		return
//	}
//
//	tx := actions.service.GetRepo().Conn.Begin()
//
//	kyc, err := actions.service.GetKycByRefID(tx, response.ReferenceID)
//	if err != nil {
//		logger.Error().Str("ReferenceID", response.ReferenceID).Err(err).Msg("Unable to find Kyc by RefId")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to get kyc",
//		})
//		return
//	}
//
//	user, err := actions.service.GetUserByKycID(tx, &kyc.ID)
//	if err != nil {
//		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to find user by KycId")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to get user for kyc ",
//		})
//		return
//	}
//
//	err = actions.service.UpdateKycByCallback(tx, kyc, user, response, callbackResponse)
//	if err != nil {
//		logger.Error().Uint64("KycID", kyc.ID).Err(err).Msg("Unable to update kyc by callback")
//		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
//			"error": "Unable to update kyc by callback",
//		})
//		return
//	}
//
//	c.Status(OK)
//}
//
//// KycPaymentDocumentVerification -
//func (actions *Actions) KycPaymentDocumentVerification(c *gin.Context) {
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	logger := log.With().
//		Str("section", "kyc").
//		Str("action", "KycPaymentDocumentVerification").
//		Uint64("user_id", user.ID).
//		Uint64("kyc_id", *user.KycID).
//		Logger()
//
//	kyc, err := actions.service.GetKycByID(user.KycID)
//	if err != nil {
//		logger.Error().Err(err).Msg("Wrong KycId")
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
//	if totalSize > utils.MaxKYCFilesSize {
//		msg := fmt.Sprintf("Total size of attachements should be <= %s",
//			utils.HumaneFileSize(utils.MaxKYCFilesSize),
//		)
//		logger.Error().Msg(msg)
//		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
//			"error": msg,
//		})
//		return
//	}
//
//	if user.EmailStatus.IsAllowed() {
//		err = actions.service.SendPaymentDocumentVerificationEmail(user.Email, user.LastName, documents)
//		if err != nil {
//			logger.Error().Err(err).Msg("Unable to send request")
//			abortWithError(c, http.StatusBadRequest, "Unable to send request")
//			return
//		}
//	}
//
//	updatedKyc, err := actions.service.UpdateKycOnStep3(kyc, status, user)
//
//	if err != nil {
//		logger.Error().Err(err).Msg("Unable to update user level")
//		abortWithError(c, http.StatusBadRequest, "Unable to update user level")
//		return
//	}
//
//	c.JSON(http.StatusOK, updatedKyc)
//}
//
//// KycGetStatusForAnUser - .
//func (actions *Actions) KycGetStatusForAnUser(c *gin.Context) {
//	iUser, _ := c.Get("auth_user")
//	user := iUser.(*model.User)
//	kyc, err := actions.service.GetKycByID(user.KycID)
//	if err != nil {
//		c.AbortWithStatusJSON(http.StatusNotFound, map[string]string{
//			"error": "Unable to get kyc status",
//		})
//		return
//	}
//	c.JSON(http.StatusOK, kyc)
//}
