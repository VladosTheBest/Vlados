package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"gorm.io/gorm"
)

// KycCustomerRegistrationV3 sets the status of user's kyc step one to success
func (actions *Actions) KycCustomerRegistrationV3(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
	logger := log.With().
		Str("section", "kyc").
		Str("action", "register").
		Uint64("user_id", user.ID).
		Str("ip", ip).
		Logger()

	// Parse request data
	var userData model.KycCustomerRegistrationRequest
	err := c.ShouldBind(&userData)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to parse user KYC data from request")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to parse user KYC data from request",
		})
		return
	}

	userData.UserID = user.ID

	// Check gender from frontend
	userData.Gender = model.DetermineGenderType(userData.Gender.String())
	if userData.Gender.String() == "" {
		logger.Error().Err(err).Msg("Invalid user gender type received")
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid user gender type received",
		})
		return
	}

	// Update user data in DB with received from request
	err = actions.service.UpdateUserKycData(&userData)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to update user KYC data")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to update user KYC data",
		})
		return
	}

	var data *model.KYC

	data, err = actions.service.AddOrUpdateKyc(model.KYCStatusStepOneSuccess, user)
	if err != nil {
		logger.Error().Err(err).Str("kyc_registration_id", strconv.FormatUint(user.ID, 10)).Msg("Unable to add or update KYC data for user")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to add or update kyc",
		})
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(user.ID, model.NotificationType_Info,
		model.NotificationTitle_KYCVerification.String(),
		fmt.Sprintf(model.NotificationMessage_KYCVerification.String(), actions.service.GetUserLevelByKYCStatus(data.Status)),
		model.Notification_KYCVerification, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, data)
}

// KycDocumentVerificationV3 get user photos and data then call jumio kyc
func (actions *Actions) KycDocumentVerificationV3(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	logger := log.With().
		Str("section", "kyc").
		Str("action", "KycDocumentVerification").
		Uint64("user_id", user.ID).
		Uint64("kyc_id", *user.KycID).
		Logger()

	// Get user`s kyc status
	kyc, err := actions.service.GetKycByID(user.KycID)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to get kyc")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to get kyc by user ID",
		})
		return
	}

	// Check user`s KYC Status, if already passed step two or three or not passed first - return error
	if kyc.Status != model.KYCStatusStepOneSuccess && kyc.Status != model.KYCStatusStepTwoFail {
		msg := fmt.Sprintf("KYC status incorrect. Should be step+one_success, got %s", kyc.Status)
		logger.Error().Err(err).Msg(msg)
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": msg,
		})
		return
	}

	form, _ := c.MultipartForm()
	documents := form.File["document"]
	documentType, _ := c.GetPostForm("document_type")
	documentTypeJumio := model.DetermineKYCDocumentType(documentType)
	if documentTypeJumio == "" {
		logger.Error().Msg("Empty document type")
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": "Empty document type",
		})
		return
	}

	fileBucketCount := 2

	kycDocs := model.KYCDocuments{}
	kycDocs.Documents = make([]model.KYCDocument, 2)
	kycDocs.UserID = user.ID

	for _, documentFile := range documents {
		err := actions.service.ValidateDocumentMimeTypes(documentFile)
		if err != nil {
			logger.Error().Msg(err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}
		documentFile.Filename = strings.ToLower(documentFile.Filename)
		if documentFile.Size > utils.MaxKYCFilesSize {
			msg := fmt.Sprintf("Size of attachements should be <= %s",
				utils.HumaneFileSize(utils.MaxKYCFilesSize),
			)
			logger.Error().Msg(msg)
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
				"error": msg,
			})
			return
		}
	}

	kycDocs.Documents[0] = model.KYCDocument{Type: documentTypeJumio, Files: documents}

	selfieFiles := form.File["selfie"]
	// Check if selfie file was added
	if len(selfieFiles) == 0 {
		logger.Error().Msg("Size of selfie is zero")
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": "Size of selfie is zero",
		})
		return
	}

	for _, selfieFile := range selfieFiles {
		err := actions.service.ValidateDocumentMimeTypes(selfieFile)
		if err != nil {
			logger.Error().Msg(err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}
		selfieFile.Filename = strings.ToLower(selfieFile.Filename)
		if selfieFile.Size > utils.MaxKYCFilesSize {
			msg := fmt.Sprintf("Size of attachements should be <= %s",
				utils.HumaneFileSize(utils.MaxKYCFilesSize),
			)
			logger.Error().Msg(msg)
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
				"error": msg,
			})
			return
		}
	}
	kycDocs.Documents[1] = model.KYCDocument{Type: model.KycSelfie, Files: selfieFiles}

	if len(kycDocs.Documents) != fileBucketCount {
		logger.Error().Msg("Unable to get kyc. Incorrect document amount")
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid number of documents",
		})

		return
	}

	kyc, err = actions.service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoPending)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to update kyc status")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to update kyc status",
			"value": err.Error(),
		})
	}

	userKYCDetails, err := actions.service.GetKYCUserDetails(user.ID)
	if err != nil {
		_, updateErr := actions.service.UpdateKYCStatusTransaction(kyc, model.KYCStatusStepTwoFail)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("Unable to update kyc status")
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
				"error": "Unable to update kyc status",
				"value": err.Error(),
			})
		}

		logger.Error().Err(err).Msg("Unable to send documents for verification")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to send documents for verification",
		})
		return
	}

	go actions.service.DoShuftiproKYCRequestAsync(kyc, userKYCDetails, &kycDocs, user, &logger)

	c.JSON(http.StatusOK, "OK")
}

func ReturnJumioStatusOk(c *gin.Context) {
	c.Status(http.StatusOK)
}

// KycCallback -
func (actions *Actions) KycCallbackV3(c *gin.Context) {
	logger := log.With().Str("section", "kyc").Str("action", "KycCallback").Logger()
	logger.Info().Msg("KycCallback from Jumio")

	var response model.ShuftiProKYCResponse

	err := c.ShouldBind(&response)
	if err != nil {
		logger.Error().Err(err).Msg("Unable to parse callback, invalid request")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to parse callback" + err.Error(),
		})
		return
	}

	// Return StatusOk to Jumio
	go actions.service.ServShuftiproKYCCallback(&logger, &response)

	c.Status(http.StatusOK)
}

func (actions *Actions) GetKycResult(jumioScanReferenceId string) (*model.JumioIdVerificationResult, error) {
	responseBody, err := actions.service.GetJumioIdVerificationResult(jumioScanReferenceId)

	//todo check response status
	if err != nil {
		return nil, err
	}

	if len(responseBody) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	result := new(model.JumioIdVerificationResult)

	err = json.Unmarshal(responseBody, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// KycGetStatusForAnUser - .
func (actions *Actions) KycGetStatusForAnUserV3(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	var err error
	kyc := model.NewKYC(model.KYCStatusStepTwoPending)

	kyc, err = actions.service.GetKycByID(user.KycID)
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, model.NewKYC(model.KYCStatusNone))
		return
	} else if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to get kyc status",
		})
		return
	}

	c.JSON(http.StatusOK, kyc)
}

// KycPaymentDocumentVerification -
func (actions *Actions) KycPaymentDocumentVerificationV3(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	logger := log.With().
		Str("section", "kyc").
		Str("action", "KycPaymentDocumentVerification").
		Uint64("user_id", user.ID).
		Uint64("kyc_id", *user.KycID).
		Logger()

	kyc, err := actions.service.GetKycByID(user.KycID)
	if err != nil {
		logger.Error().Err(err).Msg("Wrong KycId")
		abortWithError(c, http.StatusBadRequest, "Wrong KycId")
		return
	}

	form, _ := c.MultipartForm()
	documents := form.File["document"]

	var totalSize int64
	for _, file := range documents {
		totalSize += file.Size
	}

	if totalSize > utils.MaxKYCFilesSize {
		msg := fmt.Sprintf("Total size of attachements should be <= %s",
			utils.HumaneFileSize(utils.MaxKYCFilesSize),
		)
		logger.Error().Msg(msg)
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": msg,
		})
		return
	}

	if user.EmailStatus.IsAllowed() {
		err = actions.service.SendPaymentDocumentVerificationEmail(user.Email, user.LastName, documents)
		if err != nil {
			logger.Error().Err(err).Msg("Unable to send request")
			abortWithError(c, http.StatusBadRequest, "Unable to send request")
			return
		}
	}

	updatedKyc, err := actions.service.UpdateKycOnStep3(kyc, model.KYCStatusStepThreePending, user)

	if err != nil {
		logger.Error().Err(err).Msg("Unable to update user level")
		abortWithError(c, http.StatusBadRequest, "Unable to update user level")
		return
	}

	c.JSON(http.StatusOK, updatedKyc)
}
