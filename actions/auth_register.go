package actions

import (
	"net/http"
	"strings"
	"unicode"

	unleash "github.com/Unleash/unleash-client-go/v3"
	"github.com/Unleash/unleash-client-go/v3/context"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/httputils"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
)

// Register godoc
// swagger:route POST /auth/register auth register
// Register
//
// Register a new user account
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
//	  200: AuthRegisterResp
//	  400: RequestErrorResp
//	  500: RequestErrorResp
func (actions *Actions) Register(c *gin.Context) {
	log := getlog(c)

	request := model.RegistrationRequest{}
	if err := c.ShouldBind(&request); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	request.Email = strings.TrimSpace(strings.ToLower(request.Email))

	// check if the user email address already exists
	_, err := actions.service.GetUserByEmail(request.Email)
	if err == nil {
		abortWithError(c, http.StatusBadRequest, "This email address is already in use.")
		return
	}

	// check that the user type exists
	if !request.Type.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Invalid user type")
		return
	}

	// check the password is valid
	if !isPasswordValid(request.Password) {
		abortWithError(c, http.StatusBadRequest, "The password must contain at least one lowercase (a-z) letter, one uppercase (A-Z) letter, one digit (0-9) and one special character.")
		return
	}

	ctx := context.Context{
		UserId: request.Email,
	}

	allowRegister := featureflags.IsEnabled("api.allow_register", unleash.WithContext(ctx))
	if !allowRegister {
		abortWithError(c, http.StatusBadRequest, "Registrations are currently limited. Please try again later.")
		return
	}

	leadFromResource := request.MarketingID

	lbc, err := actions.service.GetRepo().GetLeadBonusCampaignByMarketingID(leadFromResource)
	if err != nil || !lbc.IsActive() {
		leadFromResource = ""
	}

	role := actions.service.ValidateRegistrationRole(request.Role)

	// create the new user in the database
	user, err := actions.service.RegisterUser(nil, "", "", request.Email, "", request.Password, request.Type.String(), role.String(), request.ReferralCode, leadFromResource)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", request.Email).Msg("Unable to register new user")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}

	// add the IP in Approved IPs table
	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
	err = actions.service.AddApprovedIPForUser(user.ID, ip)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", user.Email).Msg("Unable to add new approved IP")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}
	preAuthToken, err := auth_service.CreatePreAuthTokenWithStage(user.ID, auth_service.PreAuthTokenStageUnApprovedEmail, actions.jwt2FATokenSecret, 30)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to complete login process")
		return
	}

	subAccount, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)

	// Initialize the balances for the new user in FMS
	_, err = actions.service.FundsEngine.InitAccountBalances(subAccount, false)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", request.Email).Msg("Unable to initialize balances for new user")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}

	// launch a goroutine that sends the email verification
	// @fixme move this to a channel instead to not create new go routines for every user request
	go func() {
		userDetails := model.UserDetails{}
		db := actions.service.GetRepo().Conn.First(&userDetails, "user_id = ?", user.ID)
		if db.Error != nil {
			log.Error().Err(db.Error).Str("section", "actions").Str("action", "auth:register").Str("email", user.Email).Msg("Unable to send account activation email")
		}
		_, err := actions.service.GenerateAndSendEmailConfirmation(user.Email, userDetails.Language.String(), userDetails.Timezone)
		if err != nil {
			log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", user.Email).Msg("Unable to send account activation email")
		}
	}()

	var data = &model.SubAccount{
		UserId:            user.ID,
		AccountGroup:      model.AccountGroupMain,
		MarketType:        model.MarketTypeSpot,
		DepositAllowed:    false,
		WithdrawalAllowed: false,
		TransferAllowed:   true,
		IsDefault:         true,
		IsMain:            false,
		Title:             "First subaccount",
		Comment:           "This is the first subaccount.",
		Status:            model.SubAccountStatusActive,
	}

	_, err = actions.service.CreateSubAccount(data)
	if err != nil {
		log.Error().Str("section", "actions").Str("action", "auth:register").Str("email", user.Email).Msg("Unable to create default subaccount for user")
		abortWithError(c, http.StatusBadRequest, "Unable to create default subaccount for user")
		return
	}

	// send user response
	c.JSON(http.StatusOK, httputils.RegisterResp{
		Token: preAuthToken,
	})
}

func (actions *Actions) RegisterByPhone(c *gin.Context) {
	log := getlog(c)

	request := model.RegistrationRequest{}
	if err := c.ShouldBind(&request); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	request.Phone = strings.TrimSpace(strings.ToLower(request.Phone))

	// check if the user phone number already exists
	_, err := actions.service.GetUserByPhone(request.Phone)
	if err == nil {
		abortWithError(c, http.StatusBadRequest, "This phone number is already in use.")
		return
	}

	// check that the user type exists
	if !request.Type.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Invalid user type")
		return
	}

	// check the password is valid
	if !isPasswordValid(request.Password) {
		abortWithError(c, http.StatusBadRequest, "The password must contain at least one lowercase (a-z) letter, one uppercase (A-Z) letter, one digit (0-9) and one special character.")
		return
	}

	// Check if the feature flag "api.allow_register" is enabled
	ctx := context.Context{
		UserId: request.Phone,
	}

	leadFromResource := request.MarketingID

	lbc, err := actions.service.GetRepo().GetLeadBonusCampaignByMarketingID(leadFromResource)
	if err != nil || !lbc.IsActive() {
		leadFromResource = ""
	}

	allowRegister := featureflags.IsEnabled("api.allow_register", unleash.WithContext(ctx))
	if !allowRegister {
		abortWithError(c, http.StatusBadRequest, "Registrations are currently limited. Please try again later.")
		return
	}

	request.Email = request.Phone + "@gmail.com"

	// create the new user in the database using the provided phone number, password, type, and referral code
	user, err := actions.service.RegisterUser(nil, "", "", request.Email, request.Phone, request.Password, request.Type.String(), "member", request.ReferralCode, leadFromResource)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", request.Email).Msg("Unable to register new user")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}

	// add the IP in Approved IPs table
	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
	err = actions.service.AddApprovedIPForUser(user.ID, ip)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", user.Email).Msg("Unable to add new approved IP")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}
	// Create a pre-authentication token with a stage of "UnApprovedPhone" and a lifetime of 30 min
	preAuthToken, err := auth_service.CreatePreAuthTokenWithStage(user.ID, auth_service.PreAuthTokenStageUnApprovedPhone, actions.jwt2FATokenSecret, 30)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to complete login process")
		return
	}

	subAccount, _ := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)

	// Initialize the balances for the new user in FMS
	_, err = actions.service.FundsEngine.InitAccountBalances(subAccount, true)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "auth:register").Str("email", request.Email).Msg("Unable to initialize balances for new user")
		abortWithError(c, http.StatusInternalServerError, "Something went wrong. Please try again later.")
		return
	}

	c.Set("auth_user", user)
	c.Set("auth_user_id", user.ID)
	c.Set("auth_role_alias", user.RoleAlias)

	var data = &model.SubAccount{
		UserId:            user.ID,
		AccountGroup:      model.AccountGroupMain,
		MarketType:        model.MarketTypeSpot,
		DepositAllowed:    false,
		WithdrawalAllowed: false,
		TransferAllowed:   true,
		IsDefault:         true,
		IsMain:            false,
		Title:             "First subaccount",
		Comment:           "This is the first subaccount.",
		Status:            model.SubAccountStatusActive,
	}

	_, err = actions.service.CreateSubAccount(data)
	if err != nil {
		log.Error().Str("section", "actions").Str("action", "auth:register").Str("email", user.Email).Msg("Unable to create default subaccount for user")
		abortWithError(c, http.StatusBadRequest, "Unable to create default subaccount for user")
		return
	}

	// Send a response to the client with the pre-authentication token
	c.JSON(http.StatusOK, httputils.RegisterResp{
		Token: preAuthToken,
	})
}

func isPasswordValid(s string) bool {
	var number, upper, special bool
	letters := 0
	for _, c := range s {
		switch {
		case unicode.IsNumber(c):
			number = true
		case unicode.IsUpper(c):
			upper = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c) || unicode.IsMark(c):
			special = true
		case unicode.IsLetter(c) || c == ' ':
			letters++
		default:
			//return false, false, false, false
		}
	}
	return len(s) >= 8 && letters >= 1 && number && upper && special
}
