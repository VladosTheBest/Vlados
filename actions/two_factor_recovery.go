package actions

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/two_fa_recovery"
	"net/http"
)

func (actions *Actions) TwoFactorRecoverySendCode(c *gin.Context) {

	iUser, ok := c.Get("preauth_user")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "User not found")
		return
	}
	user := iUser.(*model.User)

	iAuthType, ok := c.Get("preauth_type")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "auth type can't be empty")
		return
	}
	authType := iAuthType.(string)

	twofa, savedType := actions.service.Is2FAEnabled(user.ID)
	if !twofa || savedType != authType {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("%s 2fa authentication is not enabled", authType))
		return
	}

	headline := ""

	switch authType {
	case two_fa_recovery.TwoFaAuthTypeGoogle.String():
		headline = "Reset Google Authenticator"
	case two_fa_recovery.TwoFaAuthTypeSms.String():
		headline = "Reset SMS Authenticator"
	default:
		abortWithError(c, http.StatusBadRequest, "Wrong auth type")
		return
	}

	code, err := two_fa_recovery.CreateNewCode(user.ID, two_fa_recovery.TwoFaAuthType(authType))
	if err != nil {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("code sending trottle")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	userDetails, err := actions.service.GetUserDetails(user.ID)
	if err != nil {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("unable to get user details")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err = actions.service.SendTwoFactorRecoveryCode(user.Email, userDetails.Language.String(), user.FullName(), headline, code)
	if err != nil {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("unable to send request")
		abortWithError(c, http.StatusBadRequest, "unable to send request")
		return
	}

	c.JSON(http.StatusOK, map[string]string{"message": "reset 2fa authentication sent successfully"})
}

func (actions *Actions) TwoFactorRecoveryVerifyCode(c *gin.Context) {
	iUser, _ := c.Get("preauth_user")
	user := iUser.(*model.User)

	iAuthType, ok := c.Get("preauth_type")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "auth type can't be empty")
		return
	}
	authType := iAuthType.(string)

	twofa, savedType := actions.service.Is2FAEnabled(user.ID)
	if !twofa || savedType != authType {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("%s 2fa authentication is not enabled", authType))
		return
	}

	code, ok := c.GetPostForm("code")
	if !ok {
		log.Error().Uint64("user_id", user.ID).Msg("empty code")
		abortWithError(c, http.StatusBadRequest, "code can't be empty")
		return
	}

	isEqual, err := two_fa_recovery.CompareCodes(user.ID, code, two_fa_recovery.TwoFaAuthType(authType))
	if err != nil {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("code comparing error")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if !isEqual {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("code is not equal")
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var data bool
	switch authType {
	case two_fa_recovery.TwoFaAuthTypeGoogle.String():
		_, data, err = actions.service.DisableGoogleAuth(user.ID)
		if err != nil {
			log.Error().Err(err).Uint64("user_id", user.ID).Msg("unable to disable google auth")
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		log.Warn().Str("section", "actions").Str("action", "otp:google:disable").Uint64("user_id", user.ID).Msg("Google auth disabled")
	case two_fa_recovery.TwoFaAuthTypeSms.String():
		_, data, err = actions.service.UnbindPhone(user.ID)
		if err != nil {
			log.Error().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", user.ID).Msg("Unable to disable sms 2FA")
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized: " + err.Error(),
			})
			return
		}
		log.Warn().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", user.ID).Msg("SMS 2FA disabled")
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": data,
		"message": "Two factor authentication is off",
	})
}

func (actions *Actions) TwoFactorRecoverySendCodes(c *gin.Context) {
	iUser, _ := c.Get("preauth_user")
	user := iUser.(*model.User)
	l := log.With().
		Str("section", "two_factor_recovery").
		Str("action", "TwoFactorRecoverySendCodes").
		Logger()

	iAuthType, ok := c.Get("preauth_type")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "auth type can't be empty")
		return
	}
	authType := iAuthType.(string)

	twofa, savedType := actions.service.Is2FAEnabled(user.ID)

	if !twofa || savedType != authType {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("%s 2fa authentication is not enabled", authType))
		return
	}

	email, ok := c.GetPostForm("email")
	if !ok {
		log.Error().Uint64("user_id", user.ID).Msg("empty email")
		abortWithError(c, http.StatusBadRequest, "email can't be empty")
		return
	}
	headline := ""

	switch authType {
	case two_fa_recovery.TwoFaAuthTypeGoogle.String():
		headline = "Reset Google Authenticator"
	case two_fa_recovery.TwoFaAuthTypeSms.String():
		headline = "Reset SMS Authenticator"
	default:
		abortWithError(c, http.StatusBadRequest, "Wrong auth type")
		return
	}

	code, err := two_fa_recovery.CreateNewCode(user.ID, two_fa_recovery.TwoFaAuthType(authType))
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("code sending trottle")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	userDetails, err := actions.service.GetUserDetails(user.ID)
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("code sending trottle")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err = actions.service.SendTwoFactorRecoveryCode(user.Email, userDetails.Language.String(), user.FullName(), headline, code)
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("unable to send request")
		abortWithError(c, http.StatusBadRequest, "unable to send request")
		return
	}

	secondCode, err := two_fa_recovery.CreateNewCode(user.ID, two_fa_recovery.TwoFaAuthType(authType))
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("code sending trottle")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	err = actions.service.SendTwoFactorRecoveryCode(email, userDetails.Language.String(), user.FullName(), headline, secondCode)
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("unable to send request")
		abortWithError(c, http.StatusBadRequest, "unable to send request")
		return
	}

	c.JSON(http.StatusOK, map[string]string{"message": "reset 2fa authentication sent successfully"})
}

func (actions *Actions) TwoFactorRecoveryVerifyCodes(c *gin.Context) {
	iUser, _ := c.Get("preauth_user")
	user := iUser.(*model.User)
	l := log.With().
		Str("section", "two_factor_recovery").
		Str("action", "TwoFactorRecoveryVerifyCodes").
		Logger()

	iAuthType, ok := c.Get("preauth_type")
	if !ok {
		abortWithError(c, http.StatusBadRequest, "auth type can't be empty")
		return
	}
	authType := iAuthType.(string)

	twofa, savedType := actions.service.Is2FAEnabled(user.ID)
	if !twofa || savedType != authType {
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("%s 2fa authentication is not enabled", authType))
		return
	}

	code, ok := c.GetPostForm("code")
	if !ok {
		l.Error().Uint64("user_id", user.ID).Msg("empty code")
		abortWithError(c, http.StatusBadRequest, "code can't be empty")
		return
	}

	isEqual, err := two_fa_recovery.CompareCodes(user.ID, code, two_fa_recovery.TwoFaAuthType(authType))
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("code comparing error")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if !isEqual {
		log.Error().Err(err).Uint64("user_id", user.ID).Msg("code is not equal")
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	secondCode, ok := c.GetPostForm("second_code")
	if !ok {
		l.Error().Uint64("user_id", user.ID).Msg("empty second code")
		abortWithError(c, http.StatusBadRequest, "second code can't be empty")
		return
	}

	isEqual, err = two_fa_recovery.CompareSecondCodes(user.ID, secondCode, two_fa_recovery.TwoFaAuthType(authType))
	if err != nil {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("second code comparing error")
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	if !isEqual {
		l.Error().Err(err).Uint64("user_id", user.ID).Msg("second code is not equal")
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var data bool
	switch authType {
	case two_fa_recovery.TwoFaAuthTypeGoogle.String():
		_, data, err = actions.service.DisableGoogleAuth(user.ID)
		if err != nil {
			l.Error().Err(err).Uint64("user_id", user.ID).Msg("unable to disable google auth")
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		l.Warn().Str("section", "actions").Str("action", "otp:google:disable").Uint64("user_id", user.ID).Msg("Google auth disabled")
	case two_fa_recovery.TwoFaAuthTypeSms.String():
		_, data, err = actions.service.UnbindPhone(user.ID)
		if err != nil {
			l.Error().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", user.ID).Msg("Unable to disable sms 2FA")
			c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized: " + err.Error(),
			})
			return
		}
		l.Warn().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", user.ID).Msg("SMS 2FA disabled")
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": data,
		"message": "Two factor authentication is off",
	})
}
