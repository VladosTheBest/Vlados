package actions

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GeeTeam/gt3-golang-sdk/geetest"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/httputils"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/manage_token"
)

const geetestV4URL = "http://gcaptcha4.geetest.com"

var store = sessions.NewCookieStore([]byte("geetest"))

var UserActivityLog = &UserOnlineLog{
	Lock: sync.Mutex{},
	Log:  map[uint64]int64{},
}

type UserOnlineLog struct {
	Log  map[uint64]int64
	Lock sync.Mutex
}

func (actions *Actions) InitUserActivityLogger(ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("cron", "user_activity_tracker").Str("action", "start").Msg("User activity tracker - started")
	everySecond := time.Tick(time.Second)
loop:
	for {
		select {
		case now := <-everySecond:
			currentTime := now.Unix()
			UserActivityLog.Lock.Lock()
			for userId, userActivity := range UserActivityLog.Log {
				if currentTime-userActivity > 15 {
					delete(UserActivityLog.Log, userId)
				}
			}
			UserActivityLog.Lock.Unlock()
		case <-ctx.Done():
			wait.Done()
			break loop
		}
	}
	log.Info().Str("cron", "user_activity_tracker").Str("action", "stop").Msg("5 => User activity tracker - stopped")
}

// CheckGeetest middleware allows us to restrict access to an endpoint if the token is invalid
func (actions *Actions) CheckGeetest() gin.HandlerFunc {
	return func(c *gin.Context) {

		if featureflags.IsEnabled("api.disable_captcha") {
			c.Next()
			return
		}

		iToken, _ := c.Get("partial_token")
		if iToken != nil {
			c.Next()
			return
		}

		if lotNumber, ok := c.GetPostForm("lot_number"); ok {
			captchaOutput, _ := c.GetPostForm("captcha_output")
			passToken, _ := c.GetPostForm("pass_token")
			genTime, _ := c.GetPostForm("gen_time")

			if err := actions.CheckGeetestV4(lotNumber, captchaOutput, passToken, genTime); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]interface{}{
					"error":  "Invalid captcha",
					"code":   -100,
					"msg":    "Failed",
					"reason": err.Error(),
				})
				return
			}

			c.Next()
			return
		}

		var geetestRes bool
		geetest := geetest.NewGeetestLib(actions.cfg.Server.GeeTest.ID, actions.cfg.Server.GeeTest.Key, 2*time.Second)
		// res := make(map[string]interface{})
		session, _ := store.Get(c.Request, "geetest")
		challenge, _ := c.GetPostForm("geetest_challenge")
		validate, _ := c.GetPostForm("geetest_validate")
		seccode, _ := c.GetPostForm("geetest_seccode")
		val := session.Values["geetest_status"]
		status := int8(1)
		if val != nil {
			status = val.(int8)
		}

		if status == 1 {
			geetestRes = geetest.SuccessValidate(challenge, validate, seccode, "", "")
		} else {
			geetestRes = geetest.FailbackValidate(challenge, validate, seccode)
		}
		if !geetestRes {
			c.AbortWithStatusJSON(500, map[string]interface{}{
				"error": "Invalid captcha",
				"code":  -100,
				"msg":   "Failed",
			})

			return
		}

		c.Next()
	}
}

// CheckGeetestV4 middleware allows us to restrict access to an endpoint if the token is invalid
// using v4 of geetest
func (actions *Actions) CheckGeetestV4(lotNumber, captchaOutput, passToken, genTime string) error {

	signToken := hmacEncode(actions.cfg.Server.GeeTestV4.Key, lotNumber)

	formData := make(url.Values)
	formData["lot_number"] = []string{lotNumber}
	formData["captcha_output"] = []string{captchaOutput}
	formData["pass_token"] = []string{passToken}
	formData["gen_time"] = []string{genTime}
	formData["sign_token"] = []string{signToken}

	URL := geetestV4URL + "/validate?captcha_id=" + actions.cfg.Server.GeeTestV4.ID
	cli := http.Client{Timeout: 2 * time.Second}

	resp, err := cli.PostForm(URL, formData)
	if err != nil {
		log.Error().Err(err).Msg("unable to send request to load captcha, url: " + URL)
		return err
	}

	resJSON, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Error().Err(err).Msg("unable read request body handle captcha")
		return err
	}
	var resMap map[string]interface{}

	if err = json.Unmarshal(resJSON, &resMap); err != nil {
		log.Error().Err(err).Msg("unable to unmarshal body to handle captcha")
		return err
	}

	result := resMap["result"]
	if result != "success" {
		return fmt.Errorf("failed captcha: %v", resMap["reason"])
	}

	return nil
}

// hmac-sha256 encrypt: CAPTCHA_KEY, lot_number
func hmacEncode(key string, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func (actions *Actions) GeetestRegisterV4(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"captcha_id": actions.cfg.Server.GeeTestV4.ID,
	})
}

// GeetestRegister godoc
// swagger:route GET /auth/geetest/register
// Geetest register captcha
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
//	  200: SuccessMessageResp
func (actions *Actions) GeetestRegister(c *gin.Context) {
	geetest := geetest.NewGeetestLib(actions.cfg.Server.GeeTest.ID, actions.cfg.Server.GeeTest.Key, 2*time.Second)
	status, response := geetest.PreProcess("", "")
	session, _ := store.Get(c.Request, "geetest")
	session.Values["geetest_status"] = status
	_ = session.Save(c.Request, c.Writer)

	c.JSON(200, json.RawMessage(response))
}

// ConfirmUserEmail godoc
// swagger:route POST /auth/email/confirm/{token} auth confirm_email
// Confirm email
//
// Confirm the email of the user based on the received token
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
//	  200: SuccessMessageResp
//	  404: RequestErrorResp
//	  500: RequestErrorResp
func (actions *Actions) ConfirmUserEmail(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		abortWithError(c, 404, "Unable to confirm address. Missing token")
		return
	}
	claims, err := ParseToken(token, actions.jwtTokenSecret)
	if err != nil {
		abortWithError(c, 404, "Unable to confirm address. Invalid or expired token")
		return
	}
	email := claims["email"].(string)
	err = actions.service.ConfirmUserEmail(email)
	if err != nil {
		abortWithError(c, 500, "Unable to confirm address")
		return
	}
	c.JSON(200, httputils.MessageResp{Message: "Email address confirmed"})
}

// ValidateOTP middleware
func (actions *Actions) ValidateOTP(authtype string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := getUserID(c)
		valid := false

		// if user has none of the 2FA options enabled, skip validation  [2FA being optional if not enabled actions still have to available, 2FA is users responsibility]
		twofa, _ := actions.service.Is2FAEnabled(userID)
		if !twofa && authtype == "detect" {
			c.Next()
			return
		}

		valid = actions.ValidateOTPCode(c, userID, authtype)
		if valid {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(403, map[string]string{
			"error": "Access Denied",
		})
	}
}

// ValidateOTPCode godoc
func (actions *Actions) ValidateOTPCode(c *gin.Context, userID uint64, authtype string) bool {
	log := getlog(c)
	if authtype == "detect" {
		atype, _ := c.GetPostForm("twofa")
		if atype == "" {
			//because DELETE does not have post variables
			atype, _ = c.GetQuery("twofa")
		}
		authtype = atype
	}

	token, _ := c.GetPostForm("code")
	if token == "" {
		//because DELETE does not have post variables
		token, _ = c.GetQuery("code")
	}

	var err error
	valid := false
	switch authtype {
	case "google":
		key, _ := c.GetPostForm("secret_key")
		valid, err = actions.service.ValidateGoogleAuthKey(userID, key, token)
	case "sms":
		valid, err = actions.service.ValidateSmsCode(userID, token)
	}

	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "otp:check").Msg("Validation error occured")
		return false
	}
	return valid
}

func (actions *Actions) CheckMaintenanceMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		if unleash.IsEnabled("api.maintenance-mode") {
			abortWithError(c, http.StatusLocked, "Our platform under maintenance. Please try later.")
			return
		}
		c.Next()
	}
}

// CheckPartialToken - check to see if there is a partial token and if so set set it in context
func (actions *Actions) CheckPartialToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.GetPostForm("token")
		_, err := ParseToken(token, actions.jwt2FATokenSecret)
		if err == nil {
			c.Set("partial_token", token)
			c.Next()
		}
	}
}

// ParseToken from JWT to Claims
func ParseToken(tokenString string, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if token == nil {
		return jwt.MapClaims{}, fmt.Errorf("Invalid token")
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return jwt.MapClaims{}, err
	// if err == nil && token.Valid {
	// 	tokenClaims := token.Claims.(jwt.MapClaims)
	// 	// expTime, _ := time.Unix(tokenClaims["exp"], 0)
	// 	// if !expTime.After(time.Now()) {
	// 	// 	return jwt.MapClaims{}, errors.New("Your token expired")
	// 	// }
	// 	return tokenClaims, nil
	// }
	// return jwt.MapClaims{}, errors.New("Invalid or expired token provided")
}

// RememberToken - save JWT token to Redis
func (actions *Actions) RememberToken(tokenString string, userID uint64) error {
	err := manage_token.RememberToken(tokenString, userID)
	if err != nil {
		return err
	}

	return nil
}

// ValidateToken - check JWT token exists in Redis
func (actions *Actions) ValidateToken(tokenString string, userID uint64) (int, error) {
	tokenExists, err := manage_token.ValidateToken(tokenString, userID)
	return tokenExists, err
}

// RemoveToken - remove JWT token from Redis
func (actions *Actions) RemoveToken(c *gin.Context) {
	userID := int(0)
	token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	claims, err := ParseToken(token, actions.jwtTokenSecret)
	if err == nil {
		userID, err = strconv.Atoi(claims["sub"].(string))
	}

	if err == nil && userID > 0 {
		_ = manage_token.RemoveToken(token, uint64(userID))
	}
}

func (actions *Actions) CheckApprovedEmail(c *gin.Context) {

	if preAuthStage, exist := c.Get("preauth_stage"); !exist || preAuthStage.(string) != auth_service.PreAuthTokenStageUnApprovedEmail.String() {
		abortWithError(c, ServerError, "Unable to process request")
		return
	}
	iUser, _ := c.Get("preauth_user")
	user := iUser.(*model.User)
	var isApproved = user.Status == model.UserStatusActive

	message := "Email address approved successfully"
	if !isApproved {
		message = "Email address not approved yet"
	}
	c.JSON(OK, map[string]interface{}{
		"email_approved": isApproved,
		"message":        message,
		"type":           "confirm_email",
		"success":        true,
	})
}
