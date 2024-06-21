package actions

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"

	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/data/wallet"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gorm.io/gorm"
)

// GetFeatureValue - Check a feature
func (actions *Actions) GetFeatureValue(c *gin.Context) {
	feature := c.Param("feature_name")

	data, _ := actions.service.GetFeatureValue(feature)
	if data != "false" {
		c.JSON(200, "true")
		return
	}
	c.JSON(http.StatusOK, "false")
}

// UpdateFeature - Update a feature
func (actions *Actions) UpdateFeature(c *gin.Context) {
	feature := c.Param("feature_name")
	value, _ := c.GetPostForm("feature_value")

	data, err := actions.service.UpdateFeature(feature, value)
	if err != nil {
		c.AbortWithStatusJSON(404, map[string]string{
			"error": "Unable to update feature",
		})
		return
	}
	c.JSON(http.StatusOK, data)
}

// UpdateFeatures - Update a set of features
func (actions *Actions) UpdateFeatures(c *gin.Context) {
	data := &model.AdminFeatureSettings{}
	var err error
	_ = c.Request.ParseForm()
	for feature, value := range c.Request.PostForm {
		data, err = actions.service.UpdateFeature(feature, value[0])
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			abortWithError(c, 404, "Unable to update features")
			return
		}
	}

	c.JSON(http.StatusOK, data)
}

// GetUsersAndOrdersCount godoc
func (actions *Actions) GetUsersAndOrdersCount(c *gin.Context) {
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	data, err := actions.service.GetUsersCountByLevel(from, to)
	if err != nil {
		c.AbortWithStatusJSON(404, map[string]string{
			"error": "Unable to get user count statistics",
		})
		return
	}
	orderCount := actions.service.GetActiveOrderCount()
	data.OrdersCount = orderCount
	c.JSON(http.StatusOK, data)
}

// GetWithdrawals - get a list of all withdrawals filtered by type
func (actions *Actions) GetWithdrawals(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")
	query := c.Query("query")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	data, err := actions.service.GetWithdrawsRequests(limit, page, from, to, status, query)
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "getWithdrawals").
			Msg("Admin - Unable to get withdrawals requests")
		abortWithError(c, 505, "Unable to get withdrawals requests")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetManualWithdrawals(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")

	data, err := actions.service.GetManualWithdrawals(limit, page, status)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "unable to get withdrawals")
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetOperations - get lit of operations
func (actions *Actions) GetOperations(c *gin.Context) {
	status := c.Query("status")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	data, err := actions.service.GetOperations(limit, page, status)
	if err != nil {
		abortWithError(c, 404, "Unable to get operations")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetDeposits - get a list of all deposits filtered by type
func (actions *Actions) GetDeposits(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")
	query := c.Query("query")
	coinSymbol := c.Query("coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	data, err := actions.service.GetTransactions(limit, page, "deposit", status, query, coinSymbol, from, to)
	if err != nil {
		abortWithError(c, 404, "Unable to get deposits")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetManualDepositConfirmingUsers(c *gin.Context) {
	confirmingUsers := actions.service.GetManualDepositConfirmingUsers()

	c.JSON(http.StatusOK, confirmingUsers)
}

func (actions *Actions) GetManualDeposits(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")

	data, err := actions.service.GetManualDeposits(limit, page, status)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to get deposits")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) AdminCreateManualWithdrawal(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	var createWithdrawalRequest = model.CreateWithdrawalRequest{}
	if err := c.ShouldBind(&createWithdrawalRequest); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Admin Create Withdrawal", "error_tip": err.Error()})
		return
	}

	manualWithdrawal, err := actions.service.CreateAdminManualWithdrawal(&createWithdrawalRequest, user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Admin Create Withdrawal", "error_tip": err.Error()})
	}

	c.JSON(http.StatusOK, manualWithdrawal)
}

func (actions *Actions) AdminConfirmManualWithdrawal(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	manualTransactionId := c.Param("manual_transaction_id")

	err := actions.service.ConfirmAdminManualWithdrawal(manualTransactionId, user.ID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": err.Error(), "error_tip": ""})
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) AdminCreateManualDeposit(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	var createDepositRequest = model.CreateDepositRequest{}
	if err := c.ShouldBind(&createDepositRequest); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Admin Create Deposit", "error_tip": err.Error()})
		return
	}

	manualDeposit, err := actions.service.CreateAdminManualDeposit(&createDepositRequest, user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Admin Create Deposit", "error_tip": err.Error()})
		return
	}

	c.JSON(http.StatusOK, manualDeposit)
}

func (actions *Actions) AdminConfirmManualTransaction(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	manualTransactionId := c.Param("manual_transaction_id")

	err := actions.service.ConfirmAdminManualDeposit(manualTransactionId, user.ID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": err.Error(), "error_tip": ""})
		return
	}

	c.JSON(http.StatusOK, "OK")
}

// AdminConfirmDeposit godoc
func (actions *Actions) AdminConfirmDeposit(c *gin.Context) {

	iTx, _ := c.Get("data_wallet_tx")
	tx := iTx.(*model.Transaction)

	if tx.TxType != model.TxType_Deposit {
		abortWithError(c, BadRequest, "Invalid transaction type")
		return
	}
	confirmations := c.PostForm("confirmations")

	var coin model.Coin
	db := actions.service.GetRepo().Conn.Where("symbol = ?", tx.CoinSymbol).First(&coin)
	if db.Error != nil {
		log.Error().Err(db.Error).
			Str("section", "app:wallet").
			Str("action", "deposit").
			Str("coin_symbol", tx.CoinSymbol).
			Msg("Coin not found with symbol. Skipping processing deposit")
		abortWithError(c, 400, "Unable to complete operation")
		return
	}
	sAmount := fmt.Sprintf("%f", tx.Amount.V)
	decAmount, ok := (&decimal.Big{}).SetString(sAmount)
	if !ok {
		abortWithError(c, BadRequest, "Invalid amount provided")
		return
	}
	decAmount.Context = decimal.Context128
	decAmount.Context.RoundingMode = decimal.ToZero
	decAmount.Quantize(coin.TokenPrecision)
	precision := decimal.New(1, -1*coin.TokenPrecision)
	decAmount = decAmount.Mul(decAmount, precision)

	event := wallet.Event{
		Event:  string(wallet.EventType_Deposit),
		UserID: tx.UserID,
		ID:     tx.ID,
		Coin:   tx.CoinSymbol,
		Meta:   map[string]string{},
		Payload: map[string]string{
			"confirmations": confirmations,
			"amount":        fmt.Sprintf("%f", decAmount),
			"fee":           "0.0",
			"address":       tx.Address,
			"status":        "confirmed",
			"txid":          tx.TxID,
		},
	}
	_, err := actions.service.GetWalletApp().Deposit(&event, actions.service)
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "confirmDeposit").
			Msg("Admin - Unable to confirm deposit transaction")
		abortWithError(c, 400, "Unable to complete operation")
		return
	}

	userDetails := model.UserDetails{}
	err = actions.service.GetRepo().Conn.First(&userDetails, "user_id = ?", tx.UserID).Error
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "confirmDeposit").
			Msg("Admin - Unable to get user details")
		abortWithError(c, 400, "Unable to complete operation")
	}

	// send emails for deposit confirmed
	go func() {
		if featureflags.IsEnabled("api.wallets.send-email-for-confirmed-deposit") {
			if tx != nil && tx.TxType == model.TxType_Deposit && tx.Status == model.TxStatus_Confirmed {
				if err = actions.service.SendEmailForDepositConfirmed(tx, userDetails.Language.String()); err != nil {
					log.Error().Err(err).
						Str("section", "actions").
						Str("action", "admin:SendEmailForDepositConfirmed").
						Msg("Unable to send email for deposit confirmed")
				}

				var relatedObject model.RelatedObjectType
				switch event.System {
				case "advcash", "clear_junction":
					relatedObject = model.Notification_Deposit_Fiat
				default:
					relatedObject = model.Notification_Deposit_Crypto
				}

				amount := tx.Amount.V.Quantize(coin.TokenPrecision)
				_, err = actions.service.SendNotification(tx.UserID, model.NotificationType_System,
					model.NotificationTitle_Deposit.String(),
					fmt.Sprintf(model.NotificationMessage_Deposit.String(), amount),
					relatedObject, tx.ID)

				if err != nil {
					log.Error().Err(err).
						Str("section", "actions").
						Str("action", "admin:SendNotification").
						Msg("Unable to send notification for deposit confirmed")
				}
			}
		}
	}()

	c.JSON(http.StatusOK, "OK")
}

// PreloadTx middleware
// - use this to preload a transaction based on the id
func (actions *Actions) PreloadTx(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param(param)
		tx, err := actions.service.GetRepo().GetTransaction(id)
		if err != nil {
			c.AbortWithStatusJSON(404, map[string]string{"error": "Invalid transaction id"})
			return
		}
		c.Set("data_wallet_tx", tx)
		c.Next()
	}
}

// GetCoinStatistics - get assets, liabilities, profit, expenses
func (actions *Actions) GetCoinStatistics(c *gin.Context) {
	if !featureflags.IsEnabled("api.admin.coins-stats") {
		abortWithError(c, 404, "Coins statistics temporary turned off")
		return
	}

	data, err := actions.service.GetCoinStatistics()
	if err != nil {
		abortWithError(c, 404, "Unable to get coins statistics")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetAdminProfile godoc
func (actions *Actions) GetAdminProfile(c *gin.Context) {
	userID, _ := getUserID(c)
	userWithDetails, err := actions.service.GetUserByIDWithDetails(uint(userID))
	if err != nil {
		abortWithError(c, 404, "Unable to get user")
		return
	}

	security := model.SecuritySettings{
		TradePasswordExists: userWithDetails.TradePasswordExists,
		DetectIPChange:      userWithDetails.DetectIPChange,
		LoginPasswordExists: userWithDetails.LoginPasswordExists,
		Google2FaExists:     userWithDetails.Google2FaExists,
		SMS2FaExists:        userWithDetails.SMS2FaExists,
		AntiPhishingExists:  userWithDetails.AntiPhishingExists,
	}

	// Map the user details to the new struct type
	adminProfile := model.ProfileResponse{
		FirstName:         userWithDetails.FirstName,
		LastName:          userWithDetails.LastName,
		Email:             userWithDetails.Email,
		RoleAlias:         userWithDetails.RoleAlias,
		ReferralCode:      userWithDetails.ReferralCode,
		DOB:               userWithDetails.DOB,
		Gender:            userWithDetails.Gender,
		Status:            userWithDetails.Status,
		Phone:             userWithDetails.Phone,
		Address:           userWithDetails.Address,
		Country:           userWithDetails.Country,
		State:             userWithDetails.State,
		City:              userWithDetails.City,
		PostalCode:        userWithDetails.PostalCode,
		Language:          userWithDetails.Language,
		FeesPayedWithPrdx: userWithDetails.FeesPayedWithPrdx,
		UserLevel:         userWithDetails.UserLevel,
		AntiPhishingKey:   userWithDetails.AntiPhishingKey,
		GoogleAuthKey:     userWithDetails.GoogleAuthKey,
		TradePassword:     userWithDetails.TradePassword,
		SmsAuthKey:        userWithDetails.SmsAuthKey,
		ShowPayWithPrdx:   userWithDetails.ShowPayWithPrdx,
		LastLogin:         userWithDetails.LastLogin,
		SecuritySettings:  security,
	}
	c.JSON(http.StatusOK, adminProfile)
}

// UpdateAdminProfile - update admin user with provided info
func (actions *Actions) UpdateAdminProfile(c *gin.Context) {
	userID, _ := getUserID(c)
	firstName, _ := c.GetPostForm("first_name")
	lastName, _ := c.GetPostForm("last_name")
	country, _ := c.GetPostForm("country")
	phone, _ := c.GetPostForm("phone")
	address, _ := c.GetPostForm("address")
	city, _ := c.GetPostForm("city")
	state, _ := c.GetPostForm("state")
	postalCode, _ := c.GetPostForm("postal_code")
	gender, _ := c.GetPostForm("gender")
	dateOfBirth, _ := c.GetPostForm("dob")
	var dob time.Time
	if dateOfBirth != "" {
		dob, _ = time.Parse(time.RFC3339, dateOfBirth)
	}

	//  get gender
	gen, err := model.GetGenderTypeFromString(gender)
	if err != nil {
		gen = ""
	}

	data, err := actions.service.UpdateAdminProfile(userID, firstName, lastName, country, phone, address, city, state, postalCode, &dob, gen)
	if err != nil {
		abortWithError(c, 404, "Unable to update profile")
		return
	}

	c.JSON(http.StatusOK, data)
}

// UpdateUserPassword - set user password by admin
func (actions *Actions) UpdateUserPassword(c *gin.Context) {
	iUser, _ := c.Get("data_user")
	user := iUser.(*model.User)

	newpass, _ := c.GetPostForm("new")

	data, err := actions.service.UpdateUserPassword(user, newpass)
	if err != nil {
		abortWithError(c, 404, "Unable to update password")
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetUserFromParam middleware
// - use this to limit requests to an action based on a given param
func (actions *Actions) GetUserFromParam(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param(param)
		userID, _ := strconv.Atoi(id)
		user, err := actions.service.GetUserByID(uint(userID))
		if err != nil {
			abortWithError(c, 404, "User not found")
			return
		}
		c.Set("data_user", user)
		c.Next()
	}
}

// GetUserRoles action
func (actions *Actions) GetUserRoles(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	data, err := actions.service.GetRolesWithStats("user", limit, page)
	if err != nil {
		abortWithError(c, 404, "Unable to get user roles")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetPermissions action
func (actions *Actions) GetPermissions(c *gin.Context) {
	data, err := actions.service.GetPermissions()
	if err != nil {
		abortWithError(c, 404, "Unable to get permissions")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetUserRole - get role details
func (actions *Actions) GetUserRole(c *gin.Context) {
	alias := c.Param("role_alias")
	data, err := actions.service.GetUserRoleWithPermissions(alias)

	if err != nil {
		abortWithError(c, 404, "Unable to get role")
		return
	}
	c.JSON(http.StatusOK, data)
}

// AddUserRole - add a role on user scope
func (actions *Actions) AddUserRole(c *gin.Context) {
	name, _ := c.GetPostForm("name")
	alias, _ := c.GetPostForm("alias")
	permissions, _ := c.GetPostFormArray("permissions")

	data, err := actions.service.AddUserRoleWithPermissions("user", alias, name, permissions)
	if err != nil {
		abortWithError(c, 404, "Unable to create role")
		return
	}

	c.JSON(http.StatusOK, data)
}

// UpdateUserRole godoc
func (actions *Actions) UpdateUserRole(c *gin.Context) {
	alias := c.Param("role_alias")
	name, _ := c.GetPostForm("name")
	permissions, _ := c.GetPostFormArray("permissions")

	data, err := actions.service.UpdateUserRoleWithPermissions(alias, name, permissions)
	if err != nil {
		abortWithError(c, 404, "Unable to update role")
		return
	}

	c.JSON(http.StatusOK, data)
}

// RemoveUserRole godoc
func (actions *Actions) RemoveUserRole(c *gin.Context) {
	alias := c.Param("role_alias")

	data, err := actions.service.RemoveUserRole(alias)
	if err != nil {
		abortWithError(c, 404, "Unable to remove role")
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetUserPermissions - get a list of permissions for current user
func (actions *Actions) GetUserPermissions(c *gin.Context) {
	role, _ := getUserRole(c)
	permissions, err := actions.service.GetPermissionsByRoleAlias(role)
	if err != nil {
		abortWithError(c, 404, "Unable to get permissions")
		return
	}
	c.JSON(http.StatusOK, permissions)
}

// GetUserWalletBalances - get balances for selected user
func (actions *Actions) GetUserWalletBalances(c *gin.Context) {
	iUser, _ := c.Get("data_user")
	user := iUser.(*model.User)

	accountParam := c.Query("account")

	balances := service.NewBalances()
	var err error

	if accountParam != "" {
		var accountID uint64
		var account *model.SubAccount
		accountID, err = subAccounts.ConvertAccountGroupToAccount(user.ID, c.Query("account"))
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}

		account, err = subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, user.ID, accountID)
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}
		err = actions.service.GetAllLiabilityBalancesForSubAccount(balances, user.ID, account)
	} else {
		err = actions.service.GetAllLiabilityBalances(balances, user.ID)
	}

	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "GetUserWalletBalances").
			Msg("Unable to get balances")
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve balances at this time. Please try again later.")
		return
	}

	data, err := actions.service.GetAllBalancesWithBotIDAndContractID(balances, user.ID)

	if err != nil {
		log.Error().Err(err).
			Str("section", "app:wallet").
			Str("action", "GetAllBalancesWithBotIDAndContractID").
			Msg("Unable to get balances")
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve balances at this time. Please try again later.")
		return
	}

	c.JSON(http.StatusOK, data)
}

// GetReferralsWithEarnings - get top referrals list including earnings
func (actions *Actions) GetReferralsWithEarnings(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	query := c.Query("query")
	id := c.Query("user_id")
	uID, _ := strconv.Atoi(id)
	userID := uint64(uID)

	referrals, err := actions.service.GetReferralsWithEarnings(userID, limit, page, query)
	if err != nil {
		abortWithError(c, 500, "Unable to retrieve referrals at this time. Please try again later.")
		return
	}
	c.JSON(http.StatusOK, referrals)
}

func (actions *Actions) GetTotalOfPRDXLine(c *gin.Context) {

	market, err := actions.service.GetMarketByID("prdxusdt")
	if err != nil {
		abortWithError(c, 500, "Unable to get market PRDX at this time. Please try again later.")
		return
	}

	totalPRDXLine, err := actions.service.GetRepo().GetTotalPRDXInLine(market.MarketPrecisionFormat)
	if err != nil {
		abortWithError(c, 500, "Unable to get total numbers of PRDX in lines at this time. Please try again later.")
		return
	}
	c.JSON(http.StatusOK, totalPRDXLine)
}

func (actions *Actions) GetTotalDistributedOfPRDXLine(c *gin.Context) {
	distributionID, _ := strconv.Atoi(c.Param("distribution_id"))

	market, err := actions.service.GetMarketByID("prdxusdt")
	if err != nil {
		abortWithError(c, 500, "Unable to get market PRDX at this time. Please try again later.")
		return
	}

	totalPRDXLine, err := actions.service.GetRepo().GetTotalDistributedPRDXInLine(market.MarketPrecisionFormat, distributionID)
	if err != nil {
		abortWithError(c, 500, "Unable to get total distribution numbers of PRDX in lines at this time. Please try again later.")
		return
	}

	c.JSON(http.StatusOK, totalPRDXLine)
}

func (actions *Actions) GetPRDXCirculation(c *gin.Context) {
	l := log.With().
		Str("section", "admin").
		Str("action", "GetPRDXCirculation").
		Logger()

	prdxCirculation, err := actions.service.GetPRDXCirculation()
	if err != nil {
		l.Err(err).Msg("Unable to get PRDX circulation")
		abortWithError(c, http.StatusInternalServerError, "Unable to get PRDX circulation")
		return
	}

	c.JSON(http.StatusOK, prdxCirculation)
}

func (actions *Actions) SetPriceLimits(c *gin.Context) {
	l := log.With().
		Str("section", "admin").
		Str("action", "SetPriceLimits").
		Logger()
	minPrice, existsMin := c.GetPostForm("min_price")
	maxPrice, existsMax := c.GetPostForm("max_price")

	if !existsMin {
		err := errors.New("No min price")
		l.Err(err).Msg("No min price")
		abortWithError(c, http.StatusInternalServerError, "No min price")
		return
	}

	if !existsMax {
		err := errors.New("No max price")
		l.Err(err).Msg("No max price")
		abortWithError(c, http.StatusInternalServerError, "No max price")
		return
	}

	err := actions.service.SetPriceLimits(minPrice, maxPrice)
	if err != nil {
		l.Err(err).Msg("Unable to set price limits")
		abortWithError(c, http.StatusInternalServerError, "Unable to set price limits")
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) GetPriceLimits(c *gin.Context) {
	l := log.With().
		Str("section", "admin").
		Str("action", "GetPriceLimits").
		Logger()

	priceLimits, err := actions.service.GetPriceLimits()
	if err != nil {
		l.Err(err).Msg("Unable to get price limits")
		abortWithError(c, http.StatusInternalServerError, "Unable to get price limits")
		return
	}

	c.JSON(http.StatusOK, priceLimits)
}

func (actions *Actions) GetBonusAccountContractsHistoryListAdmin(c *gin.Context) {
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	pair := c.Query("pair")
	status := c.Query("status")

	contracts, err := actions.service.GetBonusAccountContractsHistoryAdmin(pair, status, from, to)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    contracts,
	})
}

func (actions *Actions) ExportBonusAccountContractsHistoryListAdmin(c *gin.Context) {
	format := c.Query("format")
	side := c.Query("sideParam")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	query := c.Query("query")
	status := "fees"
	sort := c.Query("sort")
	contractID, err := strconv.Atoi(c.Param("contract_id"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	contract, err := actions.service.GetBonusAccountContractsHistoryByContractIDAdmin(uint64(contractID))
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "admin.go").
			Str("ExportBonusAccountContractsHistoryListAdmin", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 0 limit to get all data, no paging
	trades, err := actions.service.GetUserTradeHistory(contract.UserID, status, 0, 1, from, to, side, markets, query, sort, contract.SubAccount)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	data, err := actions.service.ExportBonusAccountContractsHistory(format, status, uint64(contractID), contract, trades.Trades.Trades)
	if err != nil {
		abortWithError(c, 500, "Unable to export data")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) UpdateVolumeDistributedPercent(c *gin.Context) {
	percent := struct {
		Percent string `json:"percent" form:"percent" binding:"required"`
	}{}

	if err := c.ShouldBind(&percent); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err := actions.service.UpdateVolumeDistributedPercent(percent.Percent)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) SetUserKYBStepTwoStatus(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	status := c.Query("step_two_status")
	err := actions.service.UpdateKYBStepTwoStatusByAdmin(status, userID)
	if err != nil {
		abortWithError(c, 500, "Unable to update kyb step two status")
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) GetAnnouncementsSettings(c *gin.Context) {
	settings, err := actions.service.GetAnnouncementsSettings()
	if err != nil {
		abortWithError(c, 500, "Can not update announcements settings")
		return
	}

	c.JSON(http.StatusOK, settings)
}

func (actions *Actions) UpdateAnnouncementsSettings(c *gin.Context) {
	var settings model.AnnouncementsSettingsSchema

	topics := c.PostFormArray("topics")
	if len(topics) == 0 {
		abortWithError(c, 400, "Empty topics")
		return
	}

	settings.Topics = topics
	settings.UpdatedAt = time.Now()
	settings.CreatedAt = time.Now()

	err := actions.service.UpdateAnnouncementsSettings(&settings)
	if err != nil {
		abortWithError(c, 500, "Can not update announcements settings")
		return
	}

	c.JSON(http.StatusOK, "ok")
}

func (actions *Actions) DownloadKYBDocument(c *gin.Context) {
	logger := log.With().
		Str("section", "KYBAdmin").
		Str("action", "DownloadKYB").
		Logger()

	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	fileType := c.Query("file_type")

	url, err := actions.service.DownloadKYBFileByAdmin(userID, fileType)
	if err != nil {
		logger.Error().Err(err).Msg("unable to download file by admin")
		abortWithError(c, 500, "Unable to update kyb step two status")
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"download_url": url,
	})
}

func (actions *Actions) GetUserKYBStatusByID(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.ParseUint(id, 10, 64)

	kyb, err := actions.service.GetKYBStatusesByUserID(userID)
	if err != nil {
		abortWithError(c, 500, "Unable to get kyb by admin")
		return
	}

	c.JSON(http.StatusOK, kyb)
}
