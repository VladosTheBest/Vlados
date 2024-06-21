package actions

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/httputils"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/manage_token"
)

// RequestForgotPassword godoc
// swagger:route POST /auth/forgot-password auth forgot_password
// Forgot Password
//
// Start the password recovery flow
//
//	Consumes:
//	- multipart/form-data
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Responses:
//	  200: FlashMessageResp
//	  404: RequestErrorResp
//	  500: RequestErrorResp
func (actions *Actions) RequestForgotPassword(c *gin.Context) {
	log := getlog(c)
	email, _ := c.GetPostForm("email")
	source, _ := c.GetPostForm("source")
	userAgent := c.GetHeader("user-agent")
	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
	// check if the user email address already exists
	user, err := actions.service.GetUserByEmail(email)
	if err != nil {
		abortWithError(c, 404, "Invalid email address")
		return
	}

	// generate a hash with  UpdatedAt and Password
	hash := service.MakeHash(user.UpdatedAt.String() + user.Password)
	// generate a new token for forgot password - the token will have reset_pass_user_id and user_info_hash
	token, err := auth_service.CreateToken(jwt.MapClaims{
		"reset_pass_user_id": user.ID,
		"user_info_hash":     hash,
	}, actions.jwtTokenSecret, 24)

	if err != nil {
		log.Error().Err(err).Str("section", "action").Str("action", "auth:forgot_password").Msg("Unable to create token")
		abortWithError(c, 500, "Something went wrong. Please try again later.")
		return
	}

	// get user's AP Code to be added to e-mail
	apCode, _ := actions.service.GetAntiPhishingCode(user.ID)

	// launch a goroutine that sends the email verification
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Interface("Err", err).
					Str("section", "action").
					Str("action", "auth:forgot_password").
					Uint64("user_id", user.ID).
					Msg("Panic on send forgot password email")
			}
		}()
		// get the user's IP information
		pUserAgent, _ := actions.service.ParseUserAgent(userAgent)
		geoLocation, err := actions.service.ChooseGeoLocation(user.ID, ip, pUserAgent)
		if err != nil {
			return
		}
		if user.EmailStatus.IsAllowed() {
			userDetails := model.UserDetails{}
			db := actions.service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", user.ID)
			if db.Error != nil {
				log.Error().Err(db.Error).Str("section", "action").Str("action", "auth:forgot_password").Uint64("user_id", user.ID).Msg("Unable to send forgot password email")
			}
			err = actions.service.SendForgotPasswordEmail(user.Email, userDetails.Language.String(), apCode, token, source, geoLocation, userDetails.Timezone)
			if err != nil {
				log.Error().Err(err).Str("section", "action").Str("action", "auth:forgot_password").Uint64("user_id", user.ID).Msg("Unable to send forgot password email")
			}
		}
	}()

	// send user response
	c.JSON(200, httputils.FlashMessage{FlashMessage: "An email was sent to your address to continue the forgot password process."})
}

// ResetPassword godoc
// swagger:route POST /auth/reset-password auth reset_password
// Reset Password
//
// Complete password reset flow
//
//	Consumes:
//	- multipart/form-data
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Responses:
//	  200: FlashMessageResp
//	  404: RequestErrorResp
//	  500: RequestErrorResp
func (actions *Actions) ResetPassword(c *gin.Context) {
	token, _ := c.GetPostForm("token")
	pass, _ := c.GetPostForm("password")
	// verify and parse token claims
	claims, err := ParseToken(token, actions.jwtTokenSecret)
	if err != nil {
		abortWithError(c, 404, "Invalid or expired token provided")
		return
	}

	// make sure the token is a forgot password token
	id, ok := claims["reset_pass_user_id"]
	if !ok {
		abortWithError(c, 404, "Invalid or expired token provided")
		return
	}
	// get the hash from the claims
	userInfoHash, ok := claims["user_info_hash"]
	if !ok {
		abortWithError(c, 404, "Invalid or expired token provided")
		return
	}
	// check if the user exists and load it from db
	user, err := actions.service.GetUserByID(uint(id.(float64)))

	if err != nil {
		abortWithError(c, 404, "User not found. Please try again")
		return
	}

	// make a new hash with the user details (UpdatedAt && Password)
	hash := service.MakeHash(user.UpdatedAt.String() + user.Password)
	// if the hash from token it's diffrent from user hash it means that the token is no longer active (password or updated_at was changed)
	if hash != userInfoHash {
		abortWithError(c, 404, "The token is no longer active. Please try again")
		return
	}

	// check the password is valid
	if !isPasswordValid(pass) {
		abortWithError(c, 404, "The password must contain at least one lowercase (a-z) letter, one uppercase (A-Z) letter, one digit (0-9) and one special character.")
		return
	}

	// update user password before sending a success message and asking user to login
	user, err = actions.service.UserChangePassword(user, pass)
	if err != nil {
		abortWithError(c, 500, "Unable to change your password. Please try again later.")
		return
	}

	// remove all jwt tokens assigned to user
	_ = manage_token.RemoveAllUserTokens(user.ID)

	// send user response
	c.JSON(200, httputils.FlashMessage{FlashMessage: "Your password has been successfully changed. Please login again."})
}
