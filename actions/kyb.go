package actions

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	awsutils "gitlab.com/paramountdax-exchange/exchange_api_v2/lib/aws"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

func (actions *Actions) GetKYB(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusUnauthorized, "user not found")
		return
	}

	kyb, err := actions.service.GetKYBStatusesByUserID(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Unable to get kyb by user id")
		return
	}

	c.JSON(http.StatusOK, kyb)
}

func (actions *Actions) KYBBasicInformation(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	logger := log.With().
		Str("section", "kyc").
		Str("action", "KycDocumentVerification").
		Uint64("user_id", user.ID).
		Logger()

	var request model.KYBBusinessRegistration

	if err := c.ShouldBind(&request); err != nil {
		logger.Error().Err(err).Msg("KYBBasicInformationError: can not parse request")
		abortWithError(c, http.StatusBadRequest, "Unable to parse request, error: "+err.Error())
		return
	}

	request.UserID = user.ID

	if err := actions.service.KYBBasicInformation(user, request); err != nil {
		logger.Error().Err(err).Msg("KYBBasicInformationError: error in KYBBasicInformation service method")
		abortWithError(c, http.StatusInternalServerError, "Unable to save data in db, error: "+err.Error())
		return
	}

	c.Status(http.StatusOK)
}

// Step 1: Account Information Handler
func (a *Actions) KYBAccountInformation(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	logger := log.With().
		Str("section", "kyb").
		Str("action", "AccountInformation").
		Uint64("user_id", user.ID).
		Logger()

	var request struct {
		BusinessName        string    `form:"business_name"`
		RegistrationNumber  string    `form:"registrations_number"`
		DateOfIncorporation time.Time `form:"date_of_incorporation" time_format:"02/01/2006"`
		Phone               string    `form:"phone"`
		Website             string    `form:"website"`
	}

	if err := c.ShouldBind(&request); err != nil {
		logger.Error().Err(err).Msg("AccountInformationError: cannot parse request")
		abortWithError(c, http.StatusBadRequest, "Unable to parse request, error: "+err.Error())
		return
	}

	// Assuming the existence of a method to update or insert account information
	if err := a.service.GetRepo().UpdateOrInsertAccountInformation(model.KYBBusinessRegistration{
		UserID:              user.ID,
		BusinessName:        request.BusinessName,
		RegistrationNumber:  request.RegistrationNumber,
		DateOfIncorporation: request.DateOfIncorporation,
		Phone:               request.Phone,
		Website:             request.Website,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to save account information")
		abortWithError(c, http.StatusInternalServerError, "Failed to save account information, error: "+err.Error())
		return
	}

	c.Status(http.StatusOK)
}

// Step 2: Registration Address Handler
func (a *Actions) KYBRegistrationAddress(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	logger := log.With().
		Str("section", "kyb").
		Str("action", "RegistrationAddress").
		Uint64("user_id", user.ID).
		Logger()

	var request struct {
		RegisteredCountry string `form:"registered_country"`
		RegisteredCity    string `form:"registered_city"`
		RegisteredAddress string `form:"registered_address"`
		RegisteredZipcode string `form:"registered_zipcode"`
	}

	if err := c.ShouldBind(&request); err != nil {
		logger.Error().Err(err).Msg("RegistrationAddressError: cannot parse request")
		abortWithError(c, http.StatusBadRequest, "Unable to parse request, error: "+err.Error())
		return
	}

	// Assuming the existence of a method to update or insert registration address
	if err := a.service.GetRepo().UpdateOrInsertRegistrationAddress(model.KYBBusinessRegistration{
		UserID:            user.ID,
		RegisteredCountry: request.RegisteredCountry,
		RegisteredCity:    request.RegisteredCity,
		RegisteredAddress: request.RegisteredAddress,
		RegisteredZipcode: request.RegisteredZipcode,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to save registration address")
		abortWithError(c, http.StatusInternalServerError, "Failed to save registration address, error: "+err.Error())
		return
	}

	c.Status(http.StatusOK)
}

// Step 3: Operational Address Handler
func (a *Actions) KYBOperationalAddress(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	logger := log.With().
		Str("section", "kyb").
		Str("action", "OperationalAddress").
		Uint64("user_id", user.ID).
		Logger()

	var request struct {
		OperatingBusinessCountry string `form:"operating_business_country"`
		OperatingBusinessCity    string `form:"operating_business_city"`
		OperatingBusinessAddress string `form:"operating_business_address"`
		OperatingBusinessZipcode string `form:"operating_business_zipcode"`
	}

	if err := c.ShouldBind(&request); err != nil {
		logger.Error().Err(err).Msg("OperationalAddressError: cannot parse request")
		abortWithError(c, http.StatusBadRequest, "Unable to parse request, error: "+err.Error())
		return
	}

	// Assuming the existence of a method to update or insert operational address
	if err := a.service.GetRepo().UpdateOrInsertOperationalAddress(model.KYBBusinessRegistration{
		UserID:                   user.ID,
		OperatingBusinessCountry: request.OperatingBusinessCountry,
		OperatingBusinessCity:    request.OperatingBusinessCity,
		OperatingBusinessAddress: request.OperatingBusinessAddress,
		OperatingBusinessZipcode: request.OperatingBusinessZipcode,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to save operational address")
		abortWithError(c, http.StatusInternalServerError, "Failed to save operational address, error: "+err.Error())
		return
	}

	c.Status(http.StatusOK)
}

// Step 4: Source of Funds Handler
func (a *Actions) KYBSourceOfFunds(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	logger := log.With().
		Str("section", "kyb").
		Str("action", "SourceOfFunds").
		Uint64("user_id", user.ID).
		Logger()

	var request struct {
		SourceOfFunds   string `form:"source_of_funds"`
		SourceOfCapital string `form:"source_of_capital"` // Assuming this is added as per KYB requirements
	}

	if err := c.ShouldBind(&request); err != nil {
		logger.Error().Err(err).Msg("SourceOfFundsError: cannot parse request")
		abortWithError(c, http.StatusBadRequest, "Unable to parse request, error: "+err.Error())
		return
	}

	// Assuming the existence of a method to update or insert source of funds
	if err := a.service.GetRepo().UpdateOrInsertSourceOfFunds(model.KYBBusinessRegistration{
		UserID:          user.ID,
		SourceOfFunds:   request.SourceOfFunds,
		SourceOfCapital: request.SourceOfCapital, // Ensure your model supports this field
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to save source of funds")
		abortWithError(c, http.StatusInternalServerError, "Failed to save source of funds, error: "+err.Error())
		return
	}

	c.Status(http.StatusOK)
}

// Step 5: Additional Information Handler
func (a *Actions) KYBAdditionalInformation(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	logger := log.With().
		Str("section", "kyb").
		Str("action", "AdditionalInformation").
		Uint64("user_id", user.ID).
		Logger()

	var request struct {
		NatureOfBusiness   string `form:"nature_of_business"`
		ApplicationReasons string `form:"application_reasons"`
	}

	if err := c.ShouldBind(&request); err != nil {
		logger.Error().Err(err).Msg("AdditionalInformationError: cannot parse request")
		abortWithError(c, http.StatusBadRequest, "Unable to parse request, error: "+err.Error())
		return
	}

	// Assuming the existence of a method to update or insert additional information
	if err := a.service.GetRepo().UpdateOrInsertAdditionalInformation(model.KYBBusinessRegistration{
		UserID:             user.ID,
		NatureOfBusiness:   request.NatureOfBusiness,
		ApplicationReasons: request.ApplicationReasons,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to save additional information")
		abortWithError(c, http.StatusInternalServerError, "Failed to save additional information, error: "+err.Error())
		return
	}

	c.Status(http.StatusOK)
}

func (actions *Actions) KYBDocumentsRegistration(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusUnauthorized, "user not found")
		return
	}

	logger := log.With().
		Str("section", "kyc").
		Str("action", "KycDocumentVerification").
		Uint64("user_id", userID).
		Logger()

	var data model.KYBDocumentsRegistration
	data.UserID = userID

	userBusinessDetails, err := actions.service.GetUserBusinessDetailsByUserID(userID)
	if err != nil {
		logger.Error().Err(err).Msg("unable to get kyb id from user business details")
		abortWithError(c, http.StatusInternalServerError, "Unable to get kyb id from user business details")
		return
	}

	kybID := userBusinessDetails.KYBID

	stepTwoStatus := model.KYBStatusStepTwoPending
	err = actions.service.KYBUpdateStepTwoStatus(kybID, stepTwoStatus)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to save verification status, error: "+err.Error())
		return
	}

	// Update status with result status when at the end of the method
	defer func() {
		tempErr := actions.service.KYBUpdateStepTwoStatus(kybID, stepTwoStatus)
		if tempErr != nil {
			err = tempErr
			logger.Error().Err(err).Msg("can not update step two status")
			abortWithError(c, http.StatusInternalServerError, "Unable to update kyb status")
		}
	}()

	// Form files
	form, _ := c.MultipartForm()
	certificateOfIncorporation := form.File["certificate_of_incorporation"]
	memorandumArticleOfAssociation := form.File["memorandum_article_association"]
	registerDirectors := form.File["register_directors"]
	registerOfMembers := form.File["register_of_members"]
	ownershipStructure := form.File["ownership_structure"]
	sanctionsQuestionnaire := form.File["sanctions_questionnaire"]

	if err := actions.validateKYBDocuments(certificateOfIncorporation); err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Invalid document type")
		abortWithError(c, http.StatusBadRequest, "Invalid document type")
		return
	}

	if err := actions.validateKYBDocuments(memorandumArticleOfAssociation); err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Invalid document type")
		abortWithError(c, http.StatusBadRequest, "Invalid document type")
		return
	}

	if err := actions.validateKYBDocuments(registerDirectors); err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Invalid document type")
		abortWithError(c, http.StatusBadRequest, "Invalid document type")
		return
	}

	if err := actions.validateKYBDocuments(registerOfMembers); err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Invalid document type")
		abortWithError(c, http.StatusBadRequest, "Invalid document type")
		return
	}

	if err := actions.validateKYBDocuments(ownershipStructure); err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Invalid document type")
		abortWithError(c, http.StatusBadRequest, "Invalid document type")
		return
	}

	if err := actions.validateKYBDocuments(sanctionsQuestionnaire); err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Invalid document type")
		abortWithError(c, http.StatusBadRequest, "Invalid document type")
		return
	}

	// The session the S3 Uploader will use
	logger.Debug().Msg(actions.cfg.AWS.Session.Credentials.Secret)
	sess, err := awsutils.CreateSession(&actions.cfg.AWS.Session)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not create session")
		abortWithError(c, http.StatusBadRequest, "Can not create session s3")
		return
	}

	// Create an uploader with the session and default options
	uploader := s3manager.NewUploader(sess)

	// Save multipart files in AWS bucket

	fileExt := utils.FileExtension(certificateOfIncorporation[0].Filename)
	userBusinessDetails.CertificateOfIncorporationName = actions.service.GenerateFilenameForKYB("certificate_of_incorporation", userBusinessDetails.RegistrationNumber, fileExt)
	err = awsutils.SaveMultipartFileInAWSBucket(certificateOfIncorporation, uploader, &actions.cfg.AWS.Bucket, userBusinessDetails.CertificateOfIncorporationName)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not save file in s3 bucket 1")
		abortWithError(c, http.StatusBadRequest, "Can not save file in s3 bucket")
		return
	}

	fileExt = utils.FileExtension(memorandumArticleOfAssociation[0].Filename)
	userBusinessDetails.MemorandumArticleOfAssociationName = actions.service.GenerateFilenameForKYB("memorandum_article_association", userBusinessDetails.RegistrationNumber, fileExt)
	err = awsutils.SaveMultipartFileInAWSBucket(memorandumArticleOfAssociation, uploader, &actions.cfg.AWS.Bucket, userBusinessDetails.MemorandumArticleOfAssociationName)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not save file in s3 bucket 2")
		abortWithError(c, http.StatusBadRequest, "Can not save file in s3 bucket")
		return
	}

	fileExt = utils.FileExtension(registerDirectors[0].Filename)
	userBusinessDetails.RegisterDirectorsName = actions.service.GenerateFilenameForKYB("register_directors", userBusinessDetails.RegistrationNumber, fileExt)
	err = awsutils.SaveMultipartFileInAWSBucket(registerDirectors, uploader, &actions.cfg.AWS.Bucket, userBusinessDetails.RegisterDirectorsName)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not save file in s3 bucket 3")
		abortWithError(c, http.StatusBadRequest, "Can not save file in s3 bucket")
		return
	}

	fileExt = utils.FileExtension(registerOfMembers[0].Filename)
	userBusinessDetails.RegisterOfMembers = actions.service.GenerateFilenameForKYB("register_of_members", userBusinessDetails.RegistrationNumber, fileExt)
	err = awsutils.SaveMultipartFileInAWSBucket(registerOfMembers, uploader, &actions.cfg.AWS.Bucket, userBusinessDetails.RegisterOfMembers)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not save file in s3 bucket 4")
		abortWithError(c, http.StatusBadRequest, "Can not save file in s3 bucket")
		return
	}

	fileExt = utils.FileExtension(ownershipStructure[0].Filename)
	userBusinessDetails.OwnershipStructure = actions.service.GenerateFilenameForKYB("ownership_structure", userBusinessDetails.RegistrationNumber, fileExt)
	err = awsutils.SaveMultipartFileInAWSBucket(ownershipStructure, uploader, &actions.cfg.AWS.Bucket, userBusinessDetails.OwnershipStructure)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not save file in s3 bucket 5")
		abortWithError(c, http.StatusBadRequest, "Can not save file in s3 bucket")
		return
	}

	fileExt = utils.FileExtension(sanctionsQuestionnaire[0].Filename)
	userBusinessDetails.SanctionsQuestionnaire = actions.service.GenerateFilenameForKYB("sanctions_questionnaire", userBusinessDetails.RegistrationNumber, fileExt)
	err = awsutils.SaveMultipartFileInAWSBucket(sanctionsQuestionnaire, uploader, &actions.cfg.AWS.Bucket, userBusinessDetails.SanctionsQuestionnaire)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not save file in s3 bucket 6")
		abortWithError(c, http.StatusBadRequest, "Can not save file in s3 bucket")
		return
	}

	logger.Debug().Msg(certificateOfIncorporation[0].Header.Get("Content-Type"))
	logger.Debug().Msg(userBusinessDetails.CertificateOfIncorporationName)
	logger.Debug().Msg(userBusinessDetails.MemorandumArticleOfAssociationName)
	logger.Debug().Msg(userBusinessDetails.RegisterDirectorsName)
	logger.Debug().Msg(userBusinessDetails.OwnershipStructure)
	logger.Debug().Msg(userBusinessDetails.SanctionsQuestionnaire)
	logger.Debug().Msg(userBusinessDetails.RegisterOfMembers)

	// Save user`s filenames in user_business_details
	err = actions.service.UpdateUserBusinessDetails(userBusinessDetails)
	if err != nil {
		stepTwoStatus = model.KYBStatusStepTwoFail

		logger.Error().Err(err).Msg("Can not update user business details")
		abortWithError(c, http.StatusBadRequest, "Can not update user business details")
		return
	}

	defer c.Status(http.StatusOK)
}

func (actions *Actions) validateKYBDocuments(files []*multipart.FileHeader) error {
	if len(files) == 0 {
		return errors.New("empty file")
	}

	for _, file := range files {
		if err := actions.service.ValidateDocumentMimeTypes(file); err != nil {
			return fmt.Errorf("%s, image: %s", err.Error(), file.Filename)
		}
		file.Filename = strings.ToLower(file.Filename)
		if file.Size > utils.MaxKYCFilesSize {
			return fmt.Errorf("size of image %s should be <= %s", file.Filename, utils.HumaneFileSize(utils.MaxKYCFilesSize))
		}
	}

	return nil
}

func (actions *Actions) KYBCallback(c *gin.Context) {
	c.Status(http.StatusOK)
}

func (actions *Actions) KYBVerifyDirectors(c *gin.Context) {
	// Receive director info
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
	logger := log.With().
		Str("section", "kyc").
		Str("action", "register").
		Uint64("user_id", user.ID).
		Str("ip", ip).
		Logger()

	businessMember := model.KYBMemberRegistration{}

	// Parse request
	err := c.ShouldBind(&businessMember)
	if err != nil {
		logger.Error().Err(err).Msg("KYBError: can not bind request on KYBMemberRegistration struct. " + err.Error())
		c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
			"error": "Can not bind request on KYBMemberRegistration struct",
		})
		return
	}

	if businessMember.Timestamp == 0 {
		businessMember.Timestamp = uint64(time.Now().Unix()) // Присваиваем текущий Unix timestamp
	}

	go actions.service.DoKYBVerifyBusinessMember(user, &logger, &businessMember)

	c.Status(http.StatusOK)
}

func (actions *Actions) KYBCallbackFromJumio(c *gin.Context) {
	logger := log.With().Str("section", "kyb").Str("action", "KybJumioCallback").Logger()
	logger.Info().Msg("KybCallback from Jumio")

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
	go actions.service.ServShuftiproKYBStepThreeFourCallback(&response, &logger)

	c.Status(http.StatusOK)
}
