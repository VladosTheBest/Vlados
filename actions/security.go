package actions

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
	"strconv"
)

// CheckOrGenerateGoogleSecretKey action
func (actions *Actions) CheckOrGenerateGoogleSecretKey(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.CheckOrGenerateGoogleSecretKey(userID)

	if err != nil {
		_ = c.AbortWithError(404, err)
		return
	}
	c.JSON(200, data)
}

// EnableGoogleAuth action
func (actions *Actions) EnableGoogleAuth(c *gin.Context) {
	log := getlog(c)
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	token, _ := c.GetPostForm("code")
	key, _ := c.GetPostForm("secret_key")
	pass, _ := c.GetPostForm("login_password")
	if !user.ValidatePass(pass) {
		abortWithError(c, Unauthorized, "Invalid login password")
		return
	}
	data, err := actions.service.EnableGoogleAuth(user.ID, key, token)
	if err != nil {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("Unable to enable google auth")
		_ = c.AbortWithError(404, err)
		return
	}

	id := strconv.FormatUint(data.ID, 10)
	_, err = actions.service.SendNotification(user.ID, model.NotificationType_System,
		model.NotificationTitle_2fa.String(),
		model.NotificationMessage_2faGoogleON.String(),
		model.Notification_2factorAuthentication, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// DisableGoogleAuth action
func (actions *Actions) DisableGoogleAuth(c *gin.Context) {
	log := getlog(c)
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	pass, _ := c.GetPostForm("login_password")
	if !user.ValidatePass(pass) {
		abortWithError(c, Unauthorized, "Invalid login password")
		return
	}

	userSettings, data, err := actions.service.DisableGoogleAuth(user.ID)
	if err != nil {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("Unable to disable google auth")
		_ = c.AbortWithError(404, err)
		return
	}
	id := strconv.FormatUint(userSettings.ID, 10)
	_, err = actions.service.SendNotification(user.ID, model.NotificationType_System,
		model.NotificationTitle_2fa.String(),
		model.NotificationMessage_2faGoogleOFF.String(),
		model.Notification_2factorAuthentication, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	log.Warn().Str("section", "actions").Str("action", "otp:google:disable").Uint64("user_id", user.ID).Msg("Google auth disabled")
	c.JSON(200, data)
}

func (actions Actions) SendSmsForRegistration(c *gin.Context) {
	phone := c.PostForm("phone")
	log := getlog(c)

	user, err := actions.service.GetUserByPhone(phone)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Error when trying to get phone number from db. Method: SendSmsForRegistration")
	}
	//todo delete this log
	log.Info().Msg("Service_sid: " + actions.cfg.Server.Twillio.ServiceSID + " Account_SID: " + actions.cfg.Server.Twillio.AccountSID + " AUTH_TOKEN : " + actions.cfg.Server.Twillio.AuthToken)
	err = actions.service.SendVerificationCode(actions.cfg.Server.Twillio.ServiceSID, phone, actions.cfg.Server.Twillio.AccountSID, actions.cfg.Server.Twillio.AuthToken)
	if err != nil {
		log.Error().
			Err(err).Str("section", "security").Str("action", "SendVerificationCode").
			Uint64("user_id", user.ID).Str("phone", phone).
			Msg("Error when send sms to user phone number")
		c.AbortWithStatusJSON(400, map[string]string{"error": err.Error()})
		return
	}

	err = actions.service.UpdateUserPhone(user.ID, phone)
	if err != nil {
		log.Error().
			Err(err).Str("section", "security").Str("action", "UpdateUserPhone").
			Uint64("user_id", user.ID).Str("phone", phone).
			Msg("Error when save phone number to user details")
		c.AbortWithStatusJSON(400, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(200, "Verification code sent to:"+phone)
}

func (actions *Actions) ConfirmSmsRegistration(c *gin.Context) {
	phone, _ := c.GetPostForm("phone")
	validationCode, _ := c.GetPostForm("code")

	user, err := actions.service.GetUserByPhone(phone)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Error when trying to get phone number from db. Method: ConfirmSmsRegistration")
		return
	}

	ok, err := actions.service.CheckVerificationCode(actions.cfg.Server.Twillio.ServiceSID, phone, validationCode, actions.cfg.Server.Twillio.AccountSID, actions.cfg.Server.Twillio.AuthToken)
	if err != nil {
		log.Error().
			Err(err).Str("section", "security").Str("action", "UpdateUserPhone").
			Uint64("user_id", user.ID).Str("phone", phone).
			Msg("Error checking verification code:")
		abortWithError(c, http.StatusBadRequest, "Error checking verification code")
		return
	}

	if !ok {
		log.Error().
			Err(err).Str("section", "security").Str("action", "UpdateUserPhone").
			Uint64("user_id", user.ID).Str("phone", phone).
			Msg("Verification is failed, code is incorrect!")
		abortWithError(c, http.StatusBadRequest, "Verification is failed, code is incorrect!")
		return
	}

	err = actions.service.ConfirmUserPhone(user)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Error when confirming phone")
		return
	}

	c.JSON(200, "Verification successful!")
}

// SmsAuthInitBindPhone action
func (actions *Actions) SmsAuthInitBindPhone(c *gin.Context) {
	log := getlog(c)
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	phone, _ := c.GetPostForm("phone")

	data, err := actions.service.InitBindPhone(user.ID, user.Email, phone)
	if err != nil {
		log.Error().
			Err(err).Str("section", "actions").Str("action", "otp:sms:init").
			Uint64("user_id", user.ID).Str("phone", phone).
			Msg("Error binding phone sms 2FA")
		c.AbortWithStatusJSON(400, map[string]string{"error": err.Error()})
		return
	}
	err = actions.service.UpdateUserPhone(user.ID, phone)
	if err != nil {
		log.Error().
			Err(err).Str("section", "actions").Str("action", "otp:sms:init").
			Uint64("user_id", user.ID).Str("phone", phone).
			Msg("Error when save phone number to user details")
		c.AbortWithStatusJSON(400, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(200, data)
}

// SmsAuthBindPhone action
func (actions *Actions) SmsAuthBindPhone(c *gin.Context) {
	userID, _ := getUserID(c)
	authyID, _ := c.GetPostForm("id")
	validationCode, _ := c.GetPostForm("code")
	userSettings, data, err := actions.service.BindPhone(userID, authyID, validationCode)
	if err != nil {
		c.AbortWithStatusJSON(400, map[string]string{
			"error": "Invalid validation code",
		})
		return
	}

	id := strconv.FormatUint(userSettings.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_2fa.String(),
		model.NotificationMessage_2faSmsON.String(),
		model.Notification_2factorAuthentication, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, data)
}

// SmsAuthUnbindPhone action
func (actions *Actions) SmsAuthUnbindPhone(c *gin.Context) {
	log := getlog(c)
	userID, _ := getUserID(c)

	userSettings, data, err := actions.service.UnbindPhone(userID)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", userID).Msg("Unable to disable sms 2FA")
		c.AbortWithStatusJSON(401, map[string]string{
			"error": "Unauthorized: " + err.Error(),
		})
		return
	}

	id := strconv.FormatUint(userSettings.ID, 10)
	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_2fa.String(),
		model.NotificationMessage_2faSmsOFF.String(),
		model.Notification_2factorAuthentication, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	log.Warn().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", userID).Msg("SMS 2FA disabled")
	c.JSON(200, data)
}

// SendSmsWithCode action - send a key to user's phone
func (actions *Actions) SendSmsWithCode(c *gin.Context) {
	log := getlog(c)
	userID, _ := getUserID(c)
	data, err := actions.service.SendSmsWithCode(userID)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "otp:sms:send").Uint64("user_id", userID).Msg("Unable to send sms 2FA")
		_ = c.AbortWithError(404, err)
		return
	}

	c.JSON(200, data)
}
