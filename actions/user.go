package actions

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetUsers godoc
// swagger:route GET /admin/users admin get_users
// Get Users
//
// Get all users from the system
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: Users
//	      400: RequestErrorResp
func (actions *Actions) GetUsers(c *gin.Context) {
	query := c.Query("query")
	filter := c.Query("filter")

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	data, err := actions.service.ListUsers(page, limit, query, filter)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get list of users")
		return
	}
	c.JSON(http.StatusOK, data)
}

// UpdateUser godoc
// swagger:route PUT /admin/users/{user_id} admin update_user
// Update user
//
// Update user information
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: User
//	      404: RequestErrorResp
func (actions *Actions) UpdateUser(c *gin.Context) {
	iUser, _ := c.Get("data_user")
	user := iUser.(*model.User)

	status, _ := c.GetPostForm("status")
	emailStatus, _ := c.GetPostForm("email_status")
	firstName, _ := c.GetPostForm("first_name")
	lastName, _ := c.GetPostForm("last_name")
	country, _ := c.GetPostForm("country")
	phone, _ := c.GetPostForm("phone")
	address, _ := c.GetPostForm("address")
	city, _ := c.GetPostForm("city")
	state, _ := c.GetPostForm("state")
	postalCode, _ := c.GetPostForm("postal_code")
	level, _ := c.GetPostForm("user_level")
	role, _ := c.GetPostForm("role_alias")
	gender, _ := c.GetPostForm("gender")
	levelInt, _ := strconv.Atoi(level)
	dateOfBirth, _ := c.GetPostForm("dob")
	var dob time.Time
	var err error
	if dateOfBirth != "" {
		dob, err = time.Parse(time.RFC3339, dateOfBirth)
		if err != nil {
			abortWithError(c, http.StatusNotFound, err.Error())
			return
		}
	}

	//  get status
	sts := user.Status
	if status != "" {
		sts, err = model.GetUserStatusFromString(status)
		if err != nil {
			//keep original
			sts = user.Status
		}
	}

	emailSts := user.EmailStatus
	if emailStatus != "" {
		emailSts, err = model.GetUserEmailStatusFromString(emailStatus)
		if err != nil {
			emailSts = user.EmailStatus
		}
	}

	//  get gender
	gen, err := model.GetGenderTypeFromString(gender)
	if err != nil {
		gen = ""
	}

	profileData, kyc, err := actions.service.UpdateUserByAdmin(user, firstName, lastName, country, phone, address, city, state, postalCode, levelInt, &dob, sts, gen, role, emailSts)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}

	id := strconv.FormatUint(kyc.ID, 10)
	_, err = actions.service.SendNotification(user.ID, model.NotificationType_Info,
		model.NotificationTitle_KYCVerification.String(),
		fmt.Sprintf(model.NotificationMessage_KYCVerification.String(), levelInt),
		model.Notification_KYCVerification, id)

	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, profileData)
}

// ChangeUserStatus godoc
// swagger:route PUT /admin/users/{user_id}/status admin set_user_status
// Change user status
//
// Change user status
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: User
//	      404: RequestErrorResp
func (actions *Actions) ChangeUserStatus(c *gin.Context) {
	status, _ := c.GetPostForm("status")
	iUser, _ := c.Get("data_user")
	user := iUser.(*model.User)

	//  get status
	sts, err := model.GetUserStatusFromString(status)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Invalid status")
		return
	}

	data, err := actions.service.UpdateUserStatus(user, sts)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to change user status")
		return
	}
	c.JSON(http.StatusOK, data)
}

// UpdateSettings godoc
// swagger:route PUT /admin/users/{user_id}/settings admin set_user_settings
// Change user settings
//
// Update a user's settings
//
//	    Consumes:
//	    - multipart/form-data
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   AdminToken:
//
//	    Responses:
//	      200: UserSettings
//	      404: RequestErrorResp
func (actions *Actions) UpdateSettings(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	userSettings, _ := actions.service.GetProfileSettings(userID)

	detectIPChange, _ := c.GetPostForm("detect_ip_change")
	feesPayedWithPRDX, isSet := c.GetPostForm("fees_payed_with_prdx")
	if !isSet {
		feesPayedWithPRDX = userSettings.FeesPayedWithPrdx
	}
	antiphishingKey, isSet := c.GetPostForm("anti_phishing_key")
	if !isSet {
		antiphishingKey = userSettings.AntiPhishingKey
	}
	tradePassword, isSet := c.GetPostForm("trade_password")
	// update trade password
	if isSet {
		_, err := actions.service.EnableTradePassword(userID, tradePassword)
		if err != nil {
			abortWithError(c, http.StatusNotFound, err.Error())
			return
		}
	}

	googleAuthKey := userSettings.GoogleAuthKey
	smsAuthKey := userSettings.SmsAuthKey
	disable2Fa, _ := c.GetPostForm("disable_twofa")
	disable2FaBool, _ := strconv.ParseBool(disable2Fa)
	if disable2FaBool {
		googleAuthKey = ""
		smsAuthKey = ""
	}

	data, err := actions.service.UpdateProfileSettings(userID, feesPayedWithPRDX, detectIPChange, antiphishingKey, googleAuthKey, smsAuthKey)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to update user settings")
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetUserFees godoc
func (actions *Actions) GetUserFees(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	fees, err := actions.service.GetUserFeesRow(userID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to load user fees")
		return
	}
	c.JSON(http.StatusOK, fees)
}

func (actions *Actions) ExportUserBalances(c *gin.Context) {
	balances, err := actions.service.ExportUserBalances()
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, balances)
}

func (actions *Actions) GetUserFeesForUser(c *gin.Context) {
	userID, _ := getUserID(c)
	fees, err := actions.service.GetUserFeesRow(userID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to load user fees")
		return
	}
	c.JSON(http.StatusOK, fees)
}

func (actions *Actions) GetUserFeesAdmin(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")
	side := c.Query("sideParam")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	sort := c.Query("sort")
	query := c.Query("query")

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}

	if limit > 100 {
		limit = 100
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "user.go").
			Str("GetUserFeesForUser", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	data, err := actions.service.GetUserTradeHistory(userID, status, limit, page, from, to, side, markets, query, sort, account)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to get user trade history")
		return
	}
	c.JSON(http.StatusOK, data)
}

// UpdateUserFees godoc
func (actions *Actions) UpdateUserFees(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	discountable, _ := c.GetPostForm("discountable")
	bDiscountable, _ := strconv.ParseBool(discountable)

	sDefaultMakerFee, _ := c.GetPostForm("default_maker_fee")
	sDefaultTakerFee, _ := c.GetPostForm("default_taker_fee")

	// security measure to prevent an admin from settings a very high trading fee
	maxFee, _ := (&decimal.Big{}).SetString("0.01")
	// convert the default maker fee into a decimal and validate it
	defaultMakerFee, ok := (&decimal.Big{}).SetString(sDefaultMakerFee)
	if !ok || defaultMakerFee.Sign() < 0 || defaultMakerFee.Cmp(maxFee) > 0 {
		abortWithError(c, http.StatusNotFound, "Invalid default maker fee specified. Valid amounts: 0%-1% (0.00-0.01)")
		return
	}
	// convert the default maker fee into a decimal and validate it
	defaultTakerFee, ok := (&decimal.Big{}).SetString(sDefaultTakerFee)
	if !ok || defaultTakerFee.Sign() < 0 || defaultTakerFee.Cmp(maxFee) > 0 {
		abortWithError(c, http.StatusNotFound, "Invalid default taker fee specified. Valid amounts: 0%-1% (0.00-0.01)")
		return
	}

	err := actions.service.UpdateUserFees(userID, bDiscountable, defaultTakerFee, defaultMakerFee)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to update user settings")
		return
	}

	c.JSON(http.StatusCreated, "")
}

func (actions *Actions) GetFeesAdmin(c *gin.Context) {
	if !featureflags.IsEnabled("api.admin.fee-admin") {
		abortWithError(c, http.StatusNotFound, "Coins statistics temporary turned off")
		return
	}

	datatype, _ := c.GetQuery("type")
	fromDate := c.Query("fromDate")
	toDate := c.Query("toDate")
	modeStr := c.Query("mode")

	mode := model.GeneratedMode(modeStr)

	if !mode.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Mode parameter is wrong")
		return
	}

	data, err := actions.service.GetInfoAboutFees(datatype, fromDate, toDate, mode)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get fees statistics")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetInfoAboutTrades godoc
// @Summary Get trade statistics
// @Description Get trade statistics
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Security AdminToken
// @Success 200 {object} service.TradeInfoStats TradeInfoStats
// @Failure 404 {object} httputils.RequestError
// @Router /admin/statistics/trades [get]
func (actions *Actions) GetInfoAboutTrades(c *gin.Context) {
	datatype, _ := c.GetQuery("type")
	fromDate := c.Query("fromDate")
	toDate := c.Query("toDate")

	data, err := actions.service.GetInfoAboutTrades(datatype, fromDate, toDate)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get trade statistics")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetInfoAboutUsers godoc
// @Summary Get user statistics
// @Description Get user statistics
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Security AdminToken
// @Success 200 {object} service.UsersInfo UsersInfo
// @Failure 404 {object} httputils.RequestError
// @Router /admin/statistics/users [get]
func (actions *Actions) GetInfoAboutUsers(c *gin.Context) {
	data, err := actions.service.GetUsersInfo()
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get users statistics")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetActiveUsers(c *gin.Context) {
	requestedIds := model.UserOnlineLogRequest{}
	err := c.Bind(&requestedIds)
	existingIds := make([]uint64, 0)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get users statistics")
		return
	}

	for _, requestedId := range requestedIds.UserId {
		_, exists := UserActivityLog.Log[requestedId]
		if exists {
			existingIds = append(existingIds, requestedId)
		}
	}

	c.JSON(http.StatusOK, existingIds)
}

// GetUser godoc
// @Summary Get user by id with details
// @Description Get user by id with details
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Param user_id path int true "User ID"
// @Security AdminToken
// @Success 200 {object} service.UserDetails UserDetails
// @Failure 404 {object} httputils.RequestError
// @Router /admin/users/{user_id} [get]
func (actions *Actions) GetUser(c *gin.Context) {
	id := c.Param("user_id")
	i, _ := strconv.Atoi(id)
	user, err := actions.service.GetUserByIDWithDetails(uint(i))
	if err != nil {
		abortWithError(c, http.StatusNotFound, "User not found")
		return
	}
	user.TradePasswordExists = len(user.TradePassword) > 0
	user.AntiPhishingExists = len(user.AntiPhishingKey) > 0
	user.Google2FaExists = len(user.GoogleAuthKey) > 0
	user.SMS2FaExists = len(user.SmsAuthKey) > 0
	c.JSON(http.StatusOK, user)
}

// GetUserLoginLogByID godoc
// @Summary Get user by id with details
// @Description Get user by id with details
// @Tags admin
// @Accept multipart/form-data
// @Produce json
// @Param user_id path int true "User ID"
// @Param page path int false "Page"
// @Param query path int false "Query"
// @Security AdminToken
// @Success 200 {object} model.UserActivityLogsList UserActivityLogsList
// @Failure 404 {object} httputils.RequestError
// @Router /admin/users/{user_id} [get]
func (actions *Actions) GetUserLoginLogByID(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	page := c.Query("page")
	query, _ := c.GetQuery("query")

	logs, err := actions.service.GetProfileLoginLogs(userID, query, page, "10")
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get logs")
		return
	}
	c.JSON(http.StatusOK, logs)
}

// GetUserOrdersByID godoc
func (actions *Actions) GetUserOrdersByID(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	status := c.Query("status")
	side := c.Query("sideParam")
	markets := c.Request.URL.Query()["selectedMarkets"]
	if len(markets) == 0 {
		markets, _ = actions.service.LoadMarketIDs()
	}
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	ui := c.Query("ui")
	clientOrderID := c.Query("client_order_id")

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}

	orders, err := actions.service.GetUserOrders(userID, status, limit, page, from, to, markets, side, account, ui, clientOrderID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Unable to get user orders", "error_tip": ""})
		return
	}

	trades, err := actions.service.GetTradesByOrders(orders, false)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}

	data := model.OrderListWithTrades{
		Trades: *trades,
		Orders: orders.Orders,
		Meta:   orders.Meta,
	}

	c.JSON(http.StatusOK, data)
}

// GetUserTradesByID
func (actions *Actions) GetUserTradesByID(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")
	side := c.Query("sideParam")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	query := c.Query("query")
	sort := c.Query("sort")
	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "user.go").
			Str("GetUserTradesByID", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	data, err := actions.service.GetUserTradeHistory(userID, status, limit, page, from, to, side, markets, query, sort, account)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Unable to get user trade history", "error_tip": ""})
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetUserTradesByID
func (actions *Actions) ExportUserTradesByID(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	format := c.Query("format")
	status := c.Query("status")
	side := c.Query("sideParam")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	query := c.Query("query")
	sort := c.Query("sort")
	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "user.go").
			Str("ExportUserTradesByID", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	trades, err := actions.service.GetUserTradeHistory(userID, status, limit, page, from, to, side, markets, query, sort, account)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	data, err := actions.service.ExportUserTardes(format, trades.Trades.Trades)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, data)
}

// AdminDisableTradePassword disable trade password for an user from admin
func (actions *Actions) AdminDisableTradePassword(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	_, err := actions.service.AdminDisableTradePassword(userID)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, "success")
}

// AdminEnableDetectIP - enable detect IP for an user from admin
func (actions *Actions) AdminEnableDetectIP(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	_, err := actions.service.EditDetectIP(userID, "true")
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, "success")
}

// AdminDisableDetectIP - disable detect IP for an user from admin
func (actions *Actions) AdminDisableDetectIP(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	_, err := actions.service.EditDetectIP(userID, "false")
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, "success")
}

// AdminDisableAntiPhishingCode godoc
func (actions *Actions) AdminDisableAntiPhishingCode(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	_, err := actions.service.EditAntiPhishingCode(userID, "")
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, "success")
}

// AdminDisableSmsAuth action
func (actions *Actions) AdminDisableSmsAuth(c *gin.Context) {
	log := getlog(c)
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)
	_, _, err := actions.service.UnbindPhone(userID)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", userID).Msg("Unable to disable sms 2FA")
		c.AbortWithStatusJSON(http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized: " + err.Error(),
		})
		return
	}

	log.Warn().Err(err).Str("section", "actions").Str("action", "otp:sms:disable").Uint64("user_id", userID).Msg("SMS 2FA disabled")
	c.JSON(http.StatusOK, "success")
}

// AdminDisableGoogleAuth action
func (actions *Actions) AdminDisableGoogleAuth(c *gin.Context) {
	log := getlog(c)
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	_, _, err := actions.service.DisableGoogleAuth(userID)
	if err != nil {
		log.Error().Err(err).Uint64("user_id", userID).Msg("Unable to disable google auth")
		_ = c.AbortWithError(http.StatusNotFound, err)
		return
	}
	log.Warn().Str("section", "actions").Str("action", "otp:google:disable").Uint64("user_id", userID).Msg("Google auth disabled")
	c.JSON(http.StatusOK, "success")
}

func (actions *Actions) DownloadUsersEmails(c *gin.Context) {
	query := c.Query("query")
	filter := c.Query("filter")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 100000)

	data, err := actions.service.ListUsers(page, limit, query, filter)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get list of users")
		return
	}
	emails := make([]string, 0)

	for _, user := range data.Users {
		emails = append(emails, user.Email)
	}
	result := strings.Join(emails, "\n")
	c.Writer.WriteHeader(http.StatusOK)
	c.Header("Content-Disposition", "attachment; filename=emails.txt")
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", len(result)))
	_, err = c.Writer.WriteString(result)

	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to write list of emails")
		return
	}
}

func (actions *Actions) GetTotalUserLineLevels(c *gin.Context) {
	totalUserLineLevels, err := actions.service.GetTotalUserLineLevels()

	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to write list of emails")
		return
	}

	c.JSON(http.StatusOK, totalUserLineLevels)
}
