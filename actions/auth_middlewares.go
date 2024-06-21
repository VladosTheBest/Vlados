package actions

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/dgryski/dgoogauth"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	authCache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/auth"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/httputils"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/logger"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
)

// Render as json
func (actions *Actions) Render(field string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, _ := c.Get(field)
		c.JSON(200, data)
	}
}
func (actions *Actions) PermissionsForFrontendDebug(alias string) gin.HandlerFunc {
	return func(c *gin.Context) {
		feature := c.Param("feature")

		if feature == "" {
			abortWithError(c, AccessDenied, "Access Denied")
			return
		}

		if featureflags.IsEnabled(fmt.Sprintf("api.frontend-features.%s", feature)) {
			c.Next()
			return
		}

		log := logger.GetLogger(c)
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")

		if token == "" {
			log.Debug().Str("section", "restrict").Msg("Missing token or api key")
			c.AbortWithStatusJSON(401, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		actions.restrictByToken(c, true, token)

		role, isAuth := c.Get("auth_role_alias")
		if !isAuth {
			return
		}

		if authCache.HasPerm(role.(string), alias) {
			c.Next()
			return
		}
		log.Debug().Str("section", "has_perm").
			Str("perm_alias", alias).
			Str("role_alias", role.(string)).
			Msg("Invalid access to restricted resource")
		abortWithError(c, AccessDenied, "Access Denied")
	}
}

func (actions *Actions) PermissionsForFrontendDebugBulk(alias string) gin.HandlerFunc {
	return func(c *gin.Context) {

		log := logger.GetLogger(c)
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")

		if token == "" {
			log.Debug().Str("section", "restrict").Msg("Missing token or api key")
			c.AbortWithStatusJSON(401, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		actions.restrictByToken(c, true, token)

		role, isAuth := c.Get("auth_role_alias")
		if !isAuth {
			abortWithError(c, AccessDenied, "Access Denied")
			return
		}

		if !authCache.HasPerm(role.(string), alias) {
			abortWithError(c, AccessDenied, "Access Denied")
			return
		}

		inData := struct {
			Params []string `json:",inline"`
		}{}

		if err := c.ShouldBindJSON(&inData); err != nil {
			abortWithError(c, AccessDenied, "Access Denied")
			return
		}

		data := map[string]bool{}

		for _, feature := range inData.Params {
			data[feature] = featureflags.IsEnabled(fmt.Sprintf("api.frontend-features.%s", feature))
		}

		c.JSON(http.StatusOK, map[string]interface{}{
			"data":    data,
			"success": true,
		})
	}
}

// HasPerm middleware
func (actions *Actions) HasPerm(alias string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("auth_role_alias")
		if authCache.HasPerm(role.(string), alias) {
			c.Next()
			return
		}
		log.Debug().Str("section", "has_perm").
			Str("perm_alias", alias).
			Str("role_alias", role.(string)).
			Msg("Invalid access to restricted resource")
		abortWithError(c, AccessDenied, "Access Denied")
	}
}

func (actions *Actions) TrackAdminActivity() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		if method == "GET" || method == "OPTIONS" {
			c.Next()
			return
		}
		userId, exists := c.Get("auth_user_id")
		if !exists {
			abortWithError(c, AccessDenied, "Access Denied")
		}

		ip := c.ClientIP()
		log.Debug().Str("section", "TrackAdminActivity").Str("ip", ip)
		requestUrl := fmt.Sprintf("%s%s?%s", c.Request.Host, c.Request.URL.Path, c.Request.URL.Query())

		byteBody, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			log.Error().Err(err).Msg("Unable to retrieve request body")
			abortWithError(c, http.StatusInternalServerError, "Unable to retrieve request body")
			return
		}
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(byteBody))
		encodedBody := base64.URLEncoding.EncodeToString(byteBody)
		err = actions.service.AddAdminActivity(requestUrl, encodedBody, ip, method, userId.(uint64))
		if err != nil {
			log.Error().Err(err).Msg("Unable to log activity")
			abortWithError(c, http.StatusInternalServerError, "Unable to log activity")
			return
		}

		c.Next()
	}
}

// LoginOTP middleware
func (actions *Actions) LoginOTP(args ...bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := getUserID(c)
		partialToken, _ := c.Get("partial_token")
		// get the settings from the database or exist with 500 on error
		settings, err := queries.GetRepo().GetUserSettings(userID)
		if err != nil {
			abortWithError(c, ServerError, "Unable to load required data")
			return
		}
		// cache user settings
		c.Set("auth_user_settings", settings)
		if partialToken != nil {
			if len(args) == 1 && args[0] {
				if settings.GoogleAuthKey == "" && settings.SmsAuthKey == "" {
					abortWithError(c, OTPNotEnabled, "2FA Required")
					return
				}
			}
			// if no 2FA method is set then continue
			if settings.GoogleAuthKey == "" && settings.SmsAuthKey == "" {
				c.Next()
				return
			}
			// get auth code received from request
			authCode, _ := c.GetPostForm("otp_code")
			// first check if the google auth key is set and validate against it
			if settings.GoogleAuthKey != "" {
				otpConfig := &dgoogauth.OTPConfig{
					Secret:      settings.GoogleAuthKey,
					WindowSize:  2,
					HotpCounter: 0,
				}
				// Validate token
				ok, err := otpConfig.Authenticate(authCode)
				if ok && err == nil {
					c.Next()
					return
				}
				abortWithError(c, OTPRequired, "Invalid google auth code")
				return
			}
			// check the SMS code
			if settings.SmsAuthKey != "" {
				// code received check if it's ok
				if authCode != "" {
					ok, err := actions.service.VerifySmsCode(settings.SmsAuthKey, authCode)
					if ok && err == nil {
						c.Next()
						return
					}
					abortWithError(c, OTPRequired, "Invalid sms auth code")
					return
				}
				abortWithError(c, OTPRequired, "Invalid sms auth code")
				return
			}
		}
		// send a message with the code if enabled but no auth code was sent
		if settings.SmsAuthKey != "" && len(settings.GoogleAuthKey) == 0 {
			// no code received... send code to phone
			ok, err := actions.service.SendSMS(settings.SmsAuthKey)
			if !ok || err != nil {
				abortWithError(c, OTPRequired, "Unable to send SMS auth code. Please try again later")
				return
			}

		}
		c.Next()
	}
}

// OTP middleware
func (actions *Actions) OTP(args ...bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := getUserID(c)

		// get the settings from the database or exist with 500 on error
		settings, err := queries.GetRepo().GetUserSettings(userID)
		if err != nil {
			abortWithError(c, ServerError, "Unable to load required data")
			return
		}

		// cache user settings
		c.Set("auth_user_settings", settings)
		if len(args) == 1 && args[0] {
			if settings.GoogleAuthKey == "" && settings.SmsAuthKey == "" {
				abortWithError(c, OTPNotEnabled, "2FA Required")
				return
			}
		}
		// if no 2FA method is set then continue
		if settings.GoogleAuthKey == "" && settings.SmsAuthKey == "" {
			c.Next()
			return
		}
		// get auth code received from request
		authCode, exists := c.GetPostForm("otp_code")
		if c.Request.Method == "DELETE" || !exists {
			authCode, _ = c.GetQuery("otp_code")
		}
		// first check if the google auth key is set and validate against it
		if settings.GoogleAuthKey != "" {
			otpConfig := &dgoogauth.OTPConfig{
				Secret:      settings.GoogleAuthKey,
				WindowSize:  2,
				HotpCounter: 0,
			}
			// Validate token
			ok, err := otpConfig.Authenticate(authCode)
			if ok && err == nil {
				c.Next()
				return
			}
			abortWithError(c, OTPRequired, "Invalid google auth code")
			return
		}
		// check the SMS code and send a message with the code if enabled but no auth code was sent
		if settings.SmsAuthKey != "" {
			// code received check if it's ok
			if authCode != "" {
				ok, err := actions.service.VerifySmsCode(settings.SmsAuthKey, authCode)
				if ok && err == nil {
					c.Next()
					return
				}
				abortWithError(c, OTPRequired, "Invalid sms auth code")
				return
			} else {
				// no code received... send code to phone
				ok, err := actions.service.SendSMS(settings.SmsAuthKey)
				if ok && err == nil {
					abortWithError(c, OTPRequired, "SMS code sent. Please enter it to continue")
					return
				}
				abortWithError(c, OTPRequired, "Unable to send SMS auth code. Please try again later")
				return
			}
		}
		c.Next()
	}
}

// Auth - login a user using his credentials and set it on the context
func (actions *Actions) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		email, _ := c.GetPostForm("email")
		pass, _ := c.GetPostForm("password")
		user, err := actions.service.GetUserByEmail(email)
		if err != nil || !user.ValidatePass(pass) {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}
		if user.Status == model.UserStatusBlocked {
			abortWithError(c, Unauthorized, "Your account is blocked. Please contact support.")
			return
		}
		if user.Status == model.UserStatusRemoved {
			abortWithError(c, Unauthorized, "Your account is removed. Please contact support.")
			return
		}
		if user.Status == model.UserStatusPending {
			c.JSON(OK, map[string]string{
				"pending": "true",
			})
			return
		}
		c.Set("auth_user", user)
		c.Set("auth_user_id", user.ID)
		c.Set("auth_role_alias", user.RoleAlias)
		c.Next()
	}
}

// AuthByPhone - login a user using his credentials and set it on the context
func (actions *Actions) AuthByPhone() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get phone number and password from the request
		phone, _ := c.GetPostForm("phone")
		pass, _ := c.GetPostForm("password")

		// Get the user from the database by their phone number
		user, err := actions.service.GetUserByPhone(phone)

		//todo Delete, it's only for debug
		log.Info().
			Str("GetUserByPhone", "done").
			Str("UserID", strconv.FormatUint(user.ID, 10)).
			Str("UserEmail", user.Email).
			Str("UserPhone", user.Phone)

		// If there is an error getting the user or the provided password is not valid, return an error message
		if err != nil || !user.ValidatePass(pass) {
			abortWithError(c, Unauthorized, "Invalid credentials")
			return
		}

		//todo Delete, it's only for debug
		log.Info().
			Str("ValidatePass", "done").
			Str("UserID", strconv.FormatUint(user.ID, 10)).
			Str("UserEmail", user.Email).
			Str("UserPhone", user.Phone)

		// Check if the user's account is blocked or removed and return appropriate error message
		if user.Status == model.UserStatusBlocked {
			abortWithError(c, Unauthorized, "Your account is blocked. Please contact support.")
			return
		}
		if user.Status == model.UserStatusRemoved {
			abortWithError(c, Unauthorized, "Your account is removed. Please contact support.")
			return
		}

		// Check if the user's account is still pending and return appropriate message
		if user.Status == model.UserStatusPending {
			c.JSON(OK, map[string]string{
				"pending": "true",
			})
			return
		}

		// Set the authenticated user, user ID, and role alias on the context
		c.Set("auth_user", user)
		c.Set("auth_user_id", user.ID)
		c.Set("auth_role_alias", user.RoleAlias)

		// Move on to the next middleware/handler
		c.Next()
	}
}

// HasApprovedIP godoc
func (actions *Actions) HasApprovedIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		iUser, userExists := c.Get("auth_user")
		if !userExists {
			abortWithError(c, ServerError, "User not found")
			return
		}

		user := iUser.(*model.User)
		userSettings, err := actions.service.GetProfileSettings(user.ID)

		if err != nil {
			abortWithError(c, ServerError, "User Settings not found")
			return
		}
		isDetectIPEnable := userSettings.DetectIPChange == "true"

		if !isDetectIPEnable {
			c.Next()
			return
		}
		ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
		approved, err := actions.service.HasApprovedIP(user.ID, ip)
		if err != nil {
			abortWithError(c, ServerError, "Unable to complete login process")
			return
		}
		// if the IP is not approved then save it for approval and send an email notification to the user to confirm it
		if !approved {
			// add IP for approval
			err = actions.service.AddPendingIPForUser(user.ID, ip)
			if err != nil {
				abortWithError(c, ServerError, "Unable to generate confirmation for unauthorized IP")
				return
			}
			err = actions.service.CreateActionAndSendApproval(model.ActionType_ConfirmIP, user, ip, map[string]string{"ip": ip})
			if err != nil {
				abortWithError(c, ServerError, "Unable to generate IP confirmation")
				return
			}
			token, err := auth_service.CreatePreAuthTokenWithStage(user.ID, auth_service.PreAuthTokenStageUnApprovedIP, actions.jwt2FATokenSecret, 5)
			if err != nil {
				abortWithError(c, ServerError, "Unable to complete login process")
				return
			}

			go func() {
				id := strconv.FormatUint(userSettings.ID, 10)
				_, err := actions.service.SendNotification(user.ID, model.NotificationType_Warning,
					model.NotificationTitle_EntryFromAnotherIP.String(),
					model.NotificationMessage_EntryFromAnotherIP.String(),
					model.Notification_EntryFromAnotherIP, id)

				if err != nil {
					log.Error().Err(err).
						Str("action", "HasApprovedIP").
						Str("function", "SendNotification").
						Msg("Unable to sent the notification")
				}
			}()

			c.AbortWithStatusJSON(PreconditionFailed,
				map[string]string{
					"error":         "New login from unauthorized IP. Please confirm IP via email.",
					"preauth_token": token,
				},
			)
			return
		}
		c.Next()
	}
}

func (actions *Actions) RestrictByApiKeyPermissions(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKeyV2 := c.GetHeader("x-api-key-v2")
		if apiKeyV2 == "" {
			apiKeyV2 = c.GetHeader("X-Api-Key-V2")
		}

		if apiKeyV2 != "" {

			apikey, err := actions.service.GetAPIKeyV2AndUserByPrivateKey(apiKeyV2)
			if err != nil {
				_ = c.Error(err)
				abortWithError(c, 401, err.Error())
				return
			}

			switch permission {
			case "trading_allowed":
				if apikey.TradingAllowed != "allowed" {
					abortWithError(c, Unauthorized, "Action not allowed")
					return
				}
			case "withdrawal_allowed":
				if apikey.WithdrawalAllowed != "allowed" {
					abortWithError(c, Unauthorized, "Action not allowed")
					return
				}
				//case "margin_allowed":
				//	if apikey.MarginAllowed != "allowed" {
				//		abortWithError(c, Unauthorized, "Action not allowed")
				//		return
				//	}
				//case "future_allowed":
				//	if apikey.FutureAllowed != "allowed" {
				//		abortWithError(c, Unauthorized, "Action not allowed")
				//		return
				//	}
			}

			c.Next()
		}
	}
}

// GeneratePartialToken godoc
func (actions *Actions) GeneratePartialToken(user *model.User, settings *model.UserSettings) (string, error) {
	twoFAType := "none"
	if len(settings.GoogleAuthKey) > 0 {
		twoFAType = "google"
	} else if len(settings.SmsAuthKey) > 0 {
		twoFAType = "sms"
	}
	// generate token
	claims := jwt.MapClaims{
		"key":       fmt.Sprintf("%d", user.ID),
		"twoFAType": twoFAType,
	}
	token, err := auth_service.CreatePartialToken(claims, actions.jwt2FATokenSecret, 5)
	if err != nil {
		return "", err
	}
	return token, nil
}

// GenerateLoginToken godoc
func (actions *Actions) GenerateLoginToken(user *model.User, settings *model.UserSettings, rememberMe, ip string) (string, error) {
	isIPEnabled := settings.DetectIPChange == "true"
	duration := 4 // 4 hour
	if rememberMe == "true" {
		duration = 30 * 24 // 30 days
	}
	// generate token
	claims := jwt.MapClaims{
		"role":  user.RoleAlias,
		"sub":   fmt.Sprintf("%d", user.ID),
		"email": user.Email,
	}
	if isIPEnabled {
		claims["ip"] = ip
	}
	token, err := auth_service.CreateToken(claims, actions.jwtTokenSecret, duration)
	if err != nil {
		return "", err
	}

	err = actions.RememberToken(token, user.ID)
	if err != nil {
		log.Error().Err(err).Str("section", "RememberToken").Msg("Unable to store token to redis cache")
		return "", err
	}
	return token, nil
}

// GenerateJWTToken godoc
// @todo Add a middleware to check the scope of the login request and if the user does not have the right permission deny access
func (actions *Actions) GenerateJWTToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		iSettings, _ := c.Get("auth_user_settings")
		settings := iSettings.(*model.UserSettings)
		iUser, _ := c.Get("auth_user")
		user := iUser.(*model.User)
		iToken, _ := c.Get("partial_token")
		twoFA := len(settings.GoogleAuthKey) > 0 || len(settings.SmsAuthKey) > 0
		if twoFA && (iToken == nil) {
			token, err := actions.GeneratePartialToken(user, settings)
			if err != nil {
				abortWithError(c, Unauthorized, "Invalid credentials")
				return
			}
			c.Set("auth_partial_token", token)
			c.Next()
		} else {
			rememberMe, _ := c.GetPostForm("remember_me")
			ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
			token, err := actions.GenerateLoginToken(user, settings, rememberMe, ip)
			if err != nil {
				abortWithError(c, Unauthorized, "Invalid credentials")
				return
			}
			c.Set("auth_token", token)
			c.Next()
		}
	}
}

// TrackActivity godoc
// Track user activity for the category by keeping a record of the request IP and user agent
func (actions *Actions) TrackActivity(category string) gin.HandlerFunc {
	return func(c *gin.Context) {
		iToken, _ := c.Get("partial_token")
		iUser, _ := c.Get("auth_user")
		user := iUser.(*model.User)
		iSettings, _ := c.Get("auth_user_settings")
		settings := iSettings.(*model.UserSettings)
		twoFA := len(settings.GoogleAuthKey) > 0 || len(settings.SmsAuthKey) > 0
		if iToken != nil || !twoFA {
			// todo: remove debug logs later
			log.Info().Str("section", "TrackActivity").
				Msg(c.GetHeader("x-forwarded-for"))

			ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
			log.Info().Str("section", "TrackActivity").
				Msg(ip)

			//ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
			userAgent := c.GetHeader("user-agent")
			antiPhishingCode := settings.AntiPhishingKey
			if antiPhishingCode == "" {
				antiPhishingCode = "-"
			}
			name := user.FirstName + " " + user.LastName
			// add extra details in the database and sent email notification after each successful login
			// @todo refactor this to not launch a goroutine for every successful login
			go func(userId uint64, category, email, ip, userAgent, antiPhishingCode string, emailStatus model.UserEmailStatus) {
				defer func() {
					if err := recover(); err != nil {
						log.Error().
							Interface("Err", err).
							Uint64("user_id", userId).
							Str("category", category).
							Str("email", email).
							Str("ip,", ip).
							Str("userAgent", userAgent).
							Str("section", "actions").
							Str("action", "TrackActivity").
							Str("input_user_agent", userAgent).
							Msg("Got new panic")
					}
				}()
				// get the user's IP information
				pUserAgent, err := actions.service.ParseUserAgent(userAgent)
				if err != nil {
					log.Error().
						Err(err).
						Str("section", "actions").
						Str("action", "TrackActivity").
						Str("input_user_agent", userAgent).
						Msg("Unable to parse user agent")
				} else {
					if geoLocation, err := actions.service.ChooseGeoLocation(userId, ip, pUserAgent); err != nil {
						log.Error().
							Err(err).
							Str("section", "actions").
							Str("action", "TrackActivity").
							Str("input_ip", ip).
							Uint64("input_user_id", userId).
							Str("input_user_agent", pUserAgent).
							Msg("Unable to get geolocation")
					} else {
						userDetails := model.UserDetails{}
						db := actions.service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", userId)
						if db.Error != nil {
							log.Error().
								Err(err).
								Str("section", "actions").
								Str("action", "TrackActivity").
								Str("input_ip", ip).
								Uint64("input_user_id", userId).
								Str("input_user_agent", pUserAgent).
								Msg("Unable to get userDetails")
						}
						if geoLocation.Timezone != "" {
							userDetails.Timezone = geoLocation.Timezone
							db = actions.service.GetRepo().Conn.Table("user_details").Where("user_id = ?", userId).Update("time_zone", userDetails.Timezone)
							if db.Error != nil {
								log.Error().
									Err(err).
									Str("section", "actions").
									Str("action", "TrackActivity").
									Str("input_ip", ip).
									Uint64("input_user_id", userId).
									Str("input_user_agent", pUserAgent).
									Msg("Unable to update userDetails")
							}
						}
						// add user activity event
						if _, err := actions.service.AddUserActivity(category, userId, geoLocation); err != nil {
							log.Error().
								Err(err).
								Str("section", "actions").
								Str("action", "TrackActivity").
								Str("input_category", category).
								Uint64("input_user_id", userId).
								Interface("input_user_geoLocation", geoLocation).
								Msg("Unable to add user activity")
						}

						if emailStatus.IsAllowed() {
							if err := actions.service.LoginNoticeEmail(email, userDetails.Language.String(), name, antiPhishingCode, geoLocation, userDetails.Timezone); err != nil {
								log.Error().
									Err(err).
									Str("section", "actions").
									Str("action", "TrackActivity").
									Str("input_email", email).
									Uint64("input_user_id", userId).
									Str("input_name", name).
									Interface("input_user_geoLocation", geoLocation).
									Msg("Unable to add user activity")
							}
						}
					}
				}
			}(user.ID, category, user.Email, ip, userAgent, antiPhishingCode, user.EmailStatus)
			c.Next()
		}
	}
}

// LoginResp godoc
// Complete the request with a 200 status code and the login token taken from the context
func (actions *Actions) LoginResp() gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger(c)
		token, _ := c.Get("auth_token")
		if token == nil {
			token, _ = c.Get("auth_partial_token")
		}
		iUser, _ := c.Get("auth_user")
		user := iUser.(*model.User)
		firstLogin := user.FirstLogin
		if user.FirstLogin {
			_, err := actions.service.UpdateUserFirstLogin(user, false)
			if err != nil {
				_ = c.Error(err)
				log.Error().Err(err).Str("section", "actions").Str("action", "login:resp").Msg("Unable to update first login record for user")
			}
		}
		c.JSON(200, httputils.LoginResp{Token: token.(string), FirstLogin: firstLogin})

	}
}

// LogoutResp godoc
// Complete the request with a 200 status code
func (actions *Actions) LogoutResp() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, true)
	}
}

// Restrict godoc
// Middleware to restrict a route based on a valid user token or API key
func (actions *Actions) Restrict(withUser bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := logger.GetLogger(c)
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		apiKey := c.GetHeader("x-api-key")
		if apiKey == "" {
			apiKey = c.GetHeader("X-Api-Key")
		}

		apiKeyV2 := c.GetHeader("x-api-key-v2")
		if apiKeyV2 == "" {
			apiKeyV2 = c.GetHeader("X-Api-Key-V2")
		}

		if token == "" && apiKey == "" && apiKeyV2 == "" {
			log.Warn().Str("section", "restrict").Msg("Missing token or api key")
			c.AbortWithStatusJSON(401, map[string]string{
				"error": "Unauthorized",
			})
			return
		}

		switch {
		case apiKeyV2 != "":
			actions.restrictByAPIKeyV2(c, withUser, apiKeyV2)
		case apiKey != "":
			actions.restrictByAPIKey(c, withUser, apiKey)
		default:
			actions.restrictByToken(c, withUser, token)
		}

		userId, exists := c.Get("auth_user_id")
		if exists {
			actions.addUserActivity(userId.(uint64))
		}
	}
}

func (actions *Actions) addUserActivity(userId uint64) {
	UserActivityLog.Lock.Lock()
	currentTime := time.Now().Unix()
	UserActivityLog.Log[userId] = currentTime
	UserActivityLog.Lock.Unlock()
}

// Restrict user access based on the given token
func (actions *Actions) restrictByToken(c *gin.Context, withUser bool, token string) {
	log := logger.GetLogger(c)
	claims, err := ParseToken(token, actions.jwtTokenSecret)
	// check that the token is valid
	if err != nil {
		_ = c.Error(err)
		log.Warn().Err(err).Str("section", "restrict:token").Msg("Invalid token received")
		abortWithError(c, 401, "Unauthorized")
		return
	}
	// load the ID of the user from the token
	userID, err := strconv.ParseUint(claims["sub"].(string), 10, 64)
	if err != nil {
		_ = c.Error(err)
		log.Warn().Err(err).Str("section", "restrict:token").Msg("Unable to load user id from token 'sub' claim")
		abortWithError(c, 403, "Access denied")
		return
	}

	// restrict by IP if the token was restricted to an IP
	if tokenIP, ok := claims["ip"]; ok {
		currentIP := getIPFromRequest(c.GetHeader("x-forwarded-for"))
		sTokenIP, _ := tokenIP.(string)
		if currentIP != sTokenIP {
			log.Warn().Str("section", "restrict:token").Str("token_ip", sTokenIP).Str("current_ip", currentIP).Msg("Invalid IP for token")
			abortWithError(c, 403, "Access denied")
			return
		}
	}

	// Validate token - check if token is in Redis
	if timeCtx, ok := c.Get("_timecontext"); ok {
		ctx := timeCtx.(context.Context)
		logger.LogTimestamp(ctx, "token_pre_redis_check", time.Now()) // start section performance check
	}

	// Flag to disable the user token precheck in high demand periods if this becomes costly for the API
	var tokenValid int
	if !featureflags.IsEnabled("api.disable_token_precheck") {
		tokenValid = 1
	} else {
		tokenValid, err = actions.ValidateToken(token, userID)
	}

	if timeCtx, ok := c.Get("_timecontext"); ok {
		ctx := timeCtx.(context.Context)
		logger.LogTimestamp(ctx, "token_post_redis_check", time.Now()) // end section performance check
	}

	if err != nil || tokenValid == 0 {
		if err != nil {
			_ = c.Error(err)
			log.Error().Err(err).Str("section", "restrict:token").Msg("Unable to validate auth token from cache")
		} else {
			log.Warn().Str("section", "restrict:token").Msg("Token invalidated by cache")
		}
		abortWithError(c, 403, "Access denied")
		return
	}

	// set user id on request context
	c.Set("auth_user_id", userID)
	if claims["role"] == nil {
		log.Error().Str("section", "restrict:token").Msg("Invalid access token")
		abortWithError(c, 403, "Access denied")
		return
	}
	roleAlias := model.RoleAlias(claims["role"].(string))

	if !roleAlias.IsValid() {
		if !featureflags.IsEnabled("api.enable-ui-for-system-users") {
			log.Error().Uint64("user_id", userID).Str("role_alias", roleAlias.String()).Str("section", "restrict:token").Msg("Usage UI with system user prevented")
			abortWithError(c, 403, "Access denied")
			return
		}
	}

	c.Set("auth_role_alias", roleAlias.String())

	if withUser {
		// finally get the user from the db with the ID from token
		user, err := actions.service.GetUserByID(uint(userID))
		if err != nil {
			_ = c.Error(err)
			log.Error().Err(err).Str("section", "restrict:token").Msg("Unable to find user based on token claim")
			abortWithError(c, 403, "Access denied")
			return
		}
		// update context
		c.Set("auth_role_alias", user.RoleAlias)
		c.Set("auth_user", user)
	}
	c.Set("auth_is_api_key", false)
	c.Next()
}

// Restrict user access based on the given API key
func (actions *Actions) restrictByAPIKey(c *gin.Context, withUser bool, key string) {
	log := logger.GetLogger(c)

	// add time log
	if timeCtx, ok := c.Get("_timecontext"); ok {
		ctx := timeCtx.(context.Context)
		logger.LogTimestamp(ctx, "api_key_pre_get", time.Now())
	}
	apikey, err := actions.service.GetAPIKeyAndUserByToken(key)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 401, "Unauthorized")
		return
	}

	// add time log
	if timeCtx, ok := c.Get("_timecontext"); ok {
		ctx := timeCtx.(context.Context)
		logger.LogTimestamp(ctx, "api_key_post_get", time.Now())
	}

	c.Set("auth_role_alias", apikey.RoleAlias)
	c.Set("auth_user_id", apikey.UserID)
	if withUser {
		user, err := actions.service.GetUserByID(uint(apikey.UserID))
		if err != nil {
			_ = c.Error(err)
			log.Error().Err(err).Str("section", "restrict:api").Msg("Unable to find user for api key")
			abortWithError(c, 401, "Unauthorized")
			return
		}
		c.Set("auth_user", user)
	}

	c.Set("auth_is_api_key", true)
	c.Next()
}

// Restrict user access based on the given API key
func (actions *Actions) restrictByAPIKeyV2(c *gin.Context, withUser bool, key string) {
	log := logger.GetLogger(c)

	// add time log
	if timeCtx, ok := c.Get("_timecontext"); ok {
		ctx := timeCtx.(context.Context)
		logger.LogTimestamp(ctx, "api_key_pre_get", time.Now())
	}
	apikey, err := actions.service.GetAPIKeyV2AndUserByToken(key)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 401, err.Error())
		return
	}

	// add time log
	if timeCtx, ok := c.Get("_timecontext"); ok {
		ctx := timeCtx.(context.Context)
		logger.LogTimestamp(ctx, "api_key_post_get", time.Now())
	}

	var roleAlias string
	if apikey.PublicKey == key {
		roleAlias = "api.read"
	} else {
		roleAlias = "api.write"
	}

	c.Set("auth_role_alias", roleAlias)
	c.Set("auth_user_id", apikey.UserID)
	if withUser {
		user, err := actions.service.GetUserByID(uint(apikey.UserID))
		if err != nil {
			_ = c.Error(err)
			log.Error().Err(err).Str("section", "restrict:api").Msg("Unable to find user for api key")
			abortWithError(c, 401, "Unauthorized")
			return
		}
		c.Set("auth_user", user)
	}

	c.Set("auth_is_api_key", true)
	c.Next()
}
