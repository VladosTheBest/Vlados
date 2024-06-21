package actions

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/manage_token"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

// GetProfile godoc
// swagger:route GET /profile profile get_profile
// Get profile
//
// Returns the current user profile with all relevant information.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) GetProfile(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	profile, err := actions.service.GetProfile(user)
	if err != nil {
		abortWithError(c, 404, "Unable to get profile")
		return
	}

	pd, err := actions.service.GetUserPaymentDetails(user.ID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	var adminFeatureSettings []model.AdminFeatureSettings
	q := actions.service.GetRepo().ConnReader

	err = q.Where("feature IN (?)", []string{"block_deposit", "block_withdraw"}).Find(&adminFeatureSettings).Error
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	type SecuritySetting struct {
		TradePasswordExists bool `json:"trade_password_exists"`
		DetectIPChange      bool `json:"detect_ip_change"`
		LoginPasswordExists bool `json:"login_password_exists"`
		Google2FaExists     bool `json:"google_2fa_exists"`
		SMS2FaExists        bool `json:"sms_2fa_exists"`
		AntiPhishingExists  bool `json:"anti_phishing_exists"`
	}
	type userWithSettingsWithPaymentBlocks struct {
		User                    *model.User              `json:"User"`
		UserSettings            *model.UserSettings      `json:"Settings"`
		UserVerifications       *model.UserVerifications `json:"Verifications"`
		SecuritySettings        *SecuritySetting         `json:"security_settings"`
		DepositWithdrawBlocking map[string]interface{}   `json:"DepositWithdrawBlocking"`
		LastLogin               string                   `json:"last_login"`
	}

	data := userWithSettingsWithPaymentBlocks{
		User:              profile.User,
		UserSettings:      profile.Settings,
		UserVerifications: &profile.Verifications,
		SecuritySettings: &SecuritySetting{
			TradePasswordExists: profile.TradePasswordExists,
			DetectIPChange:      profile.DetectIPChange,
			LoginPasswordExists: profile.LoginPasswordExists,
			Google2FaExists:     profile.Google2FaExists,
			SMS2FaExists:        profile.SMS2FaExists,
			AntiPhishingExists:  profile.AntiPhishingExists,
		},
		DepositWithdrawBlocking: map[string]interface{}{
			"all": adminFeatureSettings,
			"user": map[string]bool{
				"withdraw_crypto": pd.BlockWithdrawCrypto,
				"withdraw_fiat":   pd.BlockWithdrawFiat,
				"deposit_crypto":  pd.BlockDepositCrypto,
				"deposit_fiat":    pd.BlockDepositFiat,
			},
		},
		LastLogin: profile.LastLogin,
	}

	c.JSON(200, data)
}

// GetProfileDetails godoc
// swagger:route GET /profile/details profile get_profile_details
// Get profile details
//
// Returns the current user profile details with all relevant information.
//
// Deprecated: in favor of GET /profile
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Deprecated: true
//
//	    Responses:
//	      200: ProfileDetails
//	      404: RequestErrorResp
func (actions *Actions) GetProfileDetails(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	// @deprecated CH: Remove this and refactor the code to only use get profile
	data, err := actions.service.GetProfileDetails(user)
	if err != nil {
		abortWithError(c, 404, "Unable to get profile details")
		return
	}
	data.FirstName = user.FirstName
	data.LastName = user.LastName
	data.Email = user.Email
	c.JSON(200, data)
}

// SetProfileDetails godoc
// swagger:route POST /profile/details profile set_profile_details
// Set profile details
//
// Update user profile details
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: ProfileDetails
//	      404: RequestErrorResp
func (actions *Actions) SetProfileDetails(c *gin.Context) {
	userID, _ := getUserID(c)
	userDetails := service.UserDetails{}

	if err := c.ShouldBind(&userDetails); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if !userDetails.Language.IsValid() {
		abortWithError(c, http.StatusBadRequest, "language is invalid")
		return
	}

	profileData, err := actions.service.UpdateProfileDetails(userID, userDetails)
	if err != nil {
		abortWithError(c, 500, "Unable to update profile")
		return
	}

	c.JSON(200, profileData)
}

// GetProfileSettings godoc
// swagger:route GET /profile/settings profile get_profile_settings
// Get profile settings
//
// Returns the current user profile settings with all relevant information.
//
// Deprecated: in favor of GET /profile
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Deprecated: true
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) GetProfileSettings(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	userWithSettings := model.UserWithSettings{}
	data, err := actions.service.GetProfileSettings(user.ID)
	if err != nil {
		abortWithError(c, 404, "Unable to get profile settings")
		return
	}
	userWithSettings.Settings = data
	userWithSettings.TradePasswordExists = len(data.TradePassword) > 0
	userWithSettings.LoginPasswordExists = len(user.Password) > 0
	userWithSettings.Google2FaExists = len(data.GoogleAuthKey) > 0
	userWithSettings.SMS2FaExists = len(data.SmsAuthKey) > 0

	userWithSettings.Verifications.Tfa = userWithSettings.Google2FaExists || userWithSettings.SMS2FaExists
	userWithSettings.Verifications.Kyc = data.UserLevel >= 2
	userWithSettings.Verifications.Account = user.Status == "active"

	c.JSON(200, userWithSettings)
}

func (actions *Actions) GetPushNotificationSettings(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	data, err := actions.service.GetPushNotificationSettings(user.ID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to get Push notification settings")
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetLockAmount - get information about lock amount from security settings table
func (actions *Actions) GetLockAmount(c *gin.Context) {
	userID, _ := getUserID(c)
	userWithSettings := model.UserLockAmount{}
	data, err := actions.service.GetProfileSettings(userID)
	if err != nil {
		abortWithError(c, 404, "Unable to get profile settings lock amount")
		return
	}
	userWithSettings.LockAmount = data.LockAmount.V
	c.JSON(200, userWithSettings)
}

// SetTradePassword godoc
// swagger:route POST /profile/trade-password/on profile set_trade_password
// Set profile settings
//
// Returns the current user profile settings with all updated fields.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) EnableTradePassword(c *gin.Context) {
	userID, _ := getUserID(c)
	tradePassword, fieldReceived := c.GetPostForm("trade_password")
	if !fieldReceived || len(tradePassword) == 0 {
		c.JSON(200, "No trade password received")
		return
	}
	data, err := actions.service.EnableTradePassword(userID, tradePassword)
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_TradePassword.String(),
		model.NotificationMessage_TradePasswordON.String(),
		model.Notification_TradePassword, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// DisableTradePassword godoc
// swagger:route PUT /profile/trade-password/off profile set_trade_password
// Set profile settings
//
// Returns the current user profile settings with all updated fields.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) DisableTradePassword(c *gin.Context) {
	userID, _ := getUserID(c)
	tradePassword, fieldReceived := c.GetPostForm("trade_password")
	if !fieldReceived || len(tradePassword) == 0 {
		c.JSON(200, "No trade password received")
		return
	}
	data, err := actions.service.DisableTradePassword(userID, tradePassword)
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_TradePassword.String(),
		model.NotificationMessage_TradePasswordOFF.String(),
		model.Notification_TradePassword, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// DisableDetectIP godoc
// swagger:route PUT /profile/trade-password/off profile set_trade_password
// Set profile settings
//
// Returns the current user profile settings with all updated fields.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) DisableDetectIP(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.EditDetectIP(userID, "false")
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_DetectIpChange.String(),
		model.NotificationMessage_DetectIpChangeOFF.String(),
		model.Notification_DetectIpChange, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// EnableDetectIP godoc
// swagger:route PUT /profile/trade-password/off profile set_trade_password
// Set profile settings
//
// Returns the current user profile settings with all updated fields.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) EnableDetectIP(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.EditDetectIP(userID, "true")
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_DetectIpChange.String(),
		model.NotificationMessage_DetectIpChangeON.String(),
		model.Notification_DetectIpChange, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// DisableAntiPhishingCode godoc
// swagger:route PUT /anti-phishing/off profile set_trade_password
// Set profile settings
//
// Returns the current user profile settings with all updated fields.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) DisableAntiPhishingCode(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.EditAntiPhishingCode(userID, "")
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_AntiPhishingCode.String(),
		model.NotificationMessage_AntiPhishingCodeOFF.String(),
		model.Notification_AntiPhishingCode, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// EnableAntiPhishingCode godoc
// swagger:route PUT /anti-phishing/on
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) EnableAntiPhishingCode(c *gin.Context) {
	userID, _ := getUserID(c)
	antiPhishingKey, fieldReceived := c.GetPostForm("anti_phishing_key")
	if !fieldReceived || len(antiPhishingKey) == 0 {
		c.JSON(200, "No anti phishing key received")
		return
	}
	data, err := actions.service.EditAntiPhishingCode(userID, antiPhishingKey)
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_AntiPhishingCode.String(),
		model.NotificationMessage_AntiPhishingCodeON.String(),
		model.Notification_AntiPhishingCode, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// SetProfileSettings godoc
// swagger:route POST /profile/settings profile set_profile_settings
// Set profile settings
//
// Returns the current user profile settings with all updated fields.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Profile
//	      404: RequestErrorResp
func (actions *Actions) SetProfileSettings(c *gin.Context) {
	userID, _ := getUserID(c)

	profileSettings, err := actions.service.GetProfileSettings(userID)
	if err != nil {
		abortWithError(c, 404, "Unable to update profile settings")
		return
	}

	FeesPayedWithPrdx := profileSettings.FeesPayedWithPrdx
	GoogleAuthKey := profileSettings.GoogleAuthKey
	SmsAuthKey := profileSettings.SmsAuthKey

	detectIPChange, fieldReceived := c.GetPostForm("detect_ip_change")
	if !fieldReceived {
		detectIPChange = profileSettings.DetectIPChange
	}
	antiPhishingKey, fieldReceived := c.GetPostForm("anti_phishing_key")
	if !fieldReceived {
		antiPhishingKey = profileSettings.AntiPhishingKey
	}

	data, err := actions.service.UpdateProfileSettings(userID, FeesPayedWithPrdx, detectIPChange, antiPhishingKey, GoogleAuthKey, SmsAuthKey)

	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	c.JSON(200, data)
}

func (actions *Actions) SetPushNotificationSettings(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	var pushNotificationSettingsRequest = model.PushNotificationSettingsRequest{}

	if err := c.ShouldBind(&pushNotificationSettingsRequest); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Create Admin Launchpad", "error_tip": err.Error()})
		return
	}

	err := actions.service.SetPushNotificationSettings(user.ID, pushNotificationSettingsRequest)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, "ok")
}

// GetProfileLoginLogs godoc
// swagger:route GET /profile/logs profile get_profile_logs
// Get Login Logs
//
// Every successful login stores this information in the system in the form of an
// activity log. This information is then visible by the user in their profile.
// Using this endpoint you can get a list of the latest activity logs for the current user.
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: ActivityLogs
//	      404: RequestErrorResp
func (actions *Actions) GetProfileLoginLogs(c *gin.Context) {
	userID, _ := getUserID(c)
	query, _ := c.GetQuery("query")
	page, _ := c.GetQuery("page")
	limit, _ := c.GetQuery("limit")

	data, err := actions.service.GetProfileLoginLogs(userID, query, page, limit)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get login logs", "error_tip": ""})
		return
	}
	c.JSON(200, data)
}

// SetPassword godoc
// swagger:route POST /profile/password profile change_password
// Change Password
//
// Using a valid login token the user can change his password at any time by
// providing his current password and the new password.
//
// The new password must conform to a high security standard enforced by the API.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: StringResp
//	      400: RequestErrorResp
func (actions *Actions) SetPassword(c *gin.Context) {
	userID, _ := getUserID(c)
	currentpass, _ := c.GetPostForm("current")
	newpass, _ := c.GetPostForm("new")
	err := actions.service.SetPassword(userID, currentpass, newpass)
	if err != nil {
		abortWithError(c, 400, "Invalid password")
		return
	}
	// remove all tokens for this user from redis
	err = manage_token.RemoveAllUserTokens(userID)
	if err != nil {
		abortWithError(c, ServerError, "Unable to reset tokens")
		return
	}

	id := strconv.FormatUint(userID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_PasswordChanged.String(),
		model.NotificationMessage_PasswordChanged.String(),
		model.Notification_PasswordChanged, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, true)
}

// SendActivationEmail godoc
// swagger:route GET /profile/activate profile activate_email
// Send activation email
//
// In case the user did not receive their activation email they can call this
// endpoint to request that another confirmation be sent to them.
//
// This endpoint will return status 200 for success and 400 in case the account
// was already activated.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: StringResp
//	      400: RequestErrorResp
func (actions *Actions) SendActivationEmail(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	if user.Status != "pending" {
		abortWithError(c, http.StatusInternalServerError, "Account already activated")
		return
	}

	userDetails := model.UserDetails{}
	db := actions.service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", user.ID)
	if db.Error != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to send email confirmation")
		return
	}
	_, err := actions.service.GenerateAndSendEmailConfirmation(user.Email, userDetails.Language.String(), userDetails.Timezone)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to send email confirmation")
		return
	}
	c.JSON(http.StatusOK, true)
}

func (actions *Actions) GetAllUserDeposits(c *gin.Context) {
	userID := c.Param("user_id")

	id, err := strconv.Atoi(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	data, err := actions.service.GetAllDepositsInUSD(id)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to get user deposits err: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) UpdateUserNickname(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Unable to get user id")
		return
	}

	user, err := actions.service.GetUserByUserID(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "User does not exist")
		return
	}

	nickname, ok := c.GetPostForm("nickname")
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, "Invalid nickname")
		return
	}

	user.Nickname = nickname

	if nickname == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, "Empty nickname")
		return
	}

	err = actions.service.UpdateUserNickname(user)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Could not update nickname")
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) UpdateUserAvatar(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		abortWithError(c, http.StatusBadRequest, "Unable to get user id")
		return
	}

	user, err := actions.service.GetUserByUserID(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "User does not exist")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	images := form.File["avatar"]
	var image *multipart.FileHeader

	if len(images) == 1 {
		image = images[0]
		image.Filename = strings.ToLower(image.Filename)
		if image.Size > utils.MaxAvatarFileSize {
			msg := fmt.Sprintf("Size of attachements should be <= %s",
				utils.HumaneFileSize(utils.MaxAvatarFileSize),
			)
			c.AbortWithStatusJSON(http.StatusBadRequest, map[string]string{
				"error": msg,
			})
			return
		}

		err = actions.service.ValidateImageMimeType(image)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, "Invalid image mime type")
			return
		}

		imageBase64, err := actions.service.EncodeFileToBase64(image)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, "Can not encode image")
			return
		}

		user.Avatar = imageBase64
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, "Incorrect images amount")
		return
	}

	err = actions.service.UpdateUserAvatar(user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, "Can not update user nickname or avatar")
		return
	}

	c.JSON(http.StatusOK, "OK")
}
