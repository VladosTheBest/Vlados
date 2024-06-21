package actions

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"net/http"
	"strconv"
	"time"
)

var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests separated by route and HTTP method.",
		},
		[]string{"route", "method"},
	)

	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Duration of API requests.",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // This creates buckets with an exponential growth factor of 2
		},
		[]string{"route", "method"},
	)
)

func init() {
	// Register the histogram with Prometheus's default registry.
	prometheus.MustRegister(apiRequestDuration)
}

// WalletGetDepositAddress godoc
// swagger:route GET /wallets/addresses/{symbol} wallet get_deposit_address
// Get Address
//
// Return the deposit addresss for the user if one was generated
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: Address
//	      404: RequestErrorResp
func (actions *Actions) WalletGetDepositAddress(c *gin.Context) {
	userID, _ := getUserID(c)
	symbol := c.Param("symbol")
	coin, err := actions.service.GetCoin(symbol)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Invalid coin symbol")
		return
	}
	address, err := actions.service.GetDepositAddress(userID, coin.Symbol)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(200, address)
}

// WalletGetAllDepositAddresses godoc
func (actions *Actions) WalletGetAllDepositAddresses(c *gin.Context) {
	iUser, _ := c.Get("data_user")
	user := iUser.(*model.User)
	userID := user.ID
	addresses, err := actions.service.GetAllDepositAddresses(userID)
	if err != nil {
		abortWithError(c, 500, "Unable to load generated deposit addresses")
		return
	}
	c.JSON(200, addresses)
}

// WalletGetBalances godoc
// swagger:route GET /wallets/balances wallet get_balances
// Get balances
//
// Get user balances for all coins
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: Balances
//	      404: RequestErrorResp
func (actions *Actions) WalletGetBalances(c *gin.Context) {
	userID, _ := getUserID(c)
	accountParam := c.Query("account")
	accId := uint64(0)
	if accountParam != "" {
		var err error
		accId, err = strconv.ParseUint(accountParam, 10, 64)
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}
	}

	balances := service.NewBalances()

	if accId == 0 {
		err := actions.service.GetAllLiabilityBalances(balances, userID)
		if err != nil {
			log.Error().
				Err(err).
				Str("section", "app:wallet").
				Str("action", "WalletGetBalances").
				Msg("Unable to get balances")
			abortWithError(c, NotFound, "Unable to retrieve balances at this time. Please try again later.")
			return
		}
	} else {
		account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, userID, accId)
		if err != nil {
			abortWithError(c, NotFound, err.Error())
			return
		}
		err = actions.service.GetAllLiabilityBalancesForSubAccount(balances, userID, account)
		if err != nil {
			log.Error().
				Err(err).
				Str("section", "app:wallet").
				Str("action", "WalletGetBalances").
				Msg("Unable to get balances")
			abortWithError(c, NotFound, "Unable to retrieve balances at this time. Please try again later.")
			return
		}
	}

	c.JSON(200, balances.GetAll())

	// Increment the counter of balance updates via API
	apiBalanceUpdates.Add(1)
}

// WalletGetDeposits godoc
// swagger:route GET /wallets/deposits wallet get_deposits
// Get deposits
//
// Get a list of deposits
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: UserDeposits
//	      500: RequestErrorResp
func (actions *Actions) WalletGetDeposits(c *gin.Context) {
	userID, _ := getUserID(c)
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	operationType := model.RequestOperationType(c.Query("type"))

	data, err := actions.service.WalletGetDeposits(userID, operationType, limit, page)
	if err != nil {
		abortWithError(c, 500, "Unable to retrieve deposits at this time. Please try again later.")
		return
	}

	c.JSON(200, data)
}

func (actions *Actions) ExportWalletDeposit(c *gin.Context) {
	id := c.Query("id")

	data, err := actions.service.ExportWalletDeposits(id)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get receipt of payment")
		return
	}

	c.JSON(200, data)
}

// WalletGetWithdrawals godoc
// swagger:route GET /wallets/withdrawals wallet get_withdrawals
// Get withdrawals
//
// Get a list of withdrawals
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: UserWithdrawals
//	      500: RequestErrorResp
func (actions *Actions) WalletGetWithdrawals(c *gin.Context) {
	userID, _ := getUserID(c)
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	operationType := model.RequestOperationType(c.Query("type"))

	data, err := actions.service.WalletGetWithdrawals(userID, operationType, limit, page)
	if err != nil {
		abortWithError(c, 500, "Unable to retrieve withdrawals at this time. Please try again later.")
		return
	}

	c.JSON(200, data)
}

// Get24HWithdrawals godoc
// swagger:route GET /wallets/withdraw-total wallet get_total_24h_withdraw
// Get 24h withdraw total
//
// Get the total withdraw amount from the last 24 hours in BTC
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: StringResp
//	      500: RequestErrorResp
func (actions *Actions) Get24HWithdrawals(c *gin.Context) {
	userID, _ := getUserID(c)

	coinValues, err := actions.service.GetCoinsValue()
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	dataBitgo, err := actions.service.Get24HWithdrawals(userID, model.WithdrawExternalSystem_Bitgo, coinValues)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	dataCash, err := actions.service.Get24HWithdrawals(userID, model.WithdrawExternalSystem_Advcash, coinValues)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	dataClearJunction, err := actions.service.Get24HWithdrawals(userID, model.WithdrawExternalSystem_ClearJunction, coinValues)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"bitgo":          dataBitgo,
		"advcash":        dataCash,
		"clear_junction": dataClearJunction,
	})
}

func (actions *Actions) Get24HWithdrawalsOld(c *gin.Context) {
	userID, _ := getUserID(c)

	coinValues, err := actions.service.GetCoinsValue()
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	dataBitgo, err := actions.service.Get24HWithdrawals(userID, model.WithdrawExternalSystem_Bitgo, coinValues)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	if !featureflags.IsEnabled("withdrawals-multi-limits") {
		c.JSON(http.StatusOK, dataBitgo)
		return
	}

	dataCash, err := actions.service.Get24HWithdrawals(userID, model.WithdrawExternalSystem_Advcash, coinValues)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	dataClearJunction, err := actions.service.Get24HWithdrawals(userID, model.WithdrawExternalSystem_ClearJunction, coinValues)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to retrieve withdrawals total at this time. Please try again later.")
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"bitgo":          dataBitgo,
		"advcash":        dataCash,
		"clear_junction": dataClearJunction,
	})
}

// GetWithdrawLimits godoc
// swagger:route GET /wallets/withdraw-limits wallet get_withdraw_limits
// Get withdraw limits
//
// Get withdrawal limits
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//	    Produces:
//	    - application/json
//
//	    Schemes: http, https
//
//	    Security:
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: WithdrawLimits
//	      500: RequestErrorResp
func (actions *Actions) GetWithdrawLimits(c *gin.Context) {
	data, err := actions.service.GetWithdrawLimits()
	if err != nil {
		abortWithError(c, 500, "Unable to get limits")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) UpdateWithdrawLimitsByUser(c *gin.Context) {
	id := c.Param("user_id")
	exSystem, _ := c.GetPostForm("external_system")
	withdrawLimit, _ := c.GetPostForm("withdraw_limit")
	decWithdrawLimit, _ := conv.NewDecimalWithPrecision().SetString(withdrawLimit)

	uID, err := strconv.Atoi(id)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}
	userID := uint64(uID)

	externalSystem := model.WithdrawExternalSystem(exSystem)

	if !externalSystem.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Payment Method parameter is wrong")
		return
	}

	pd, err := actions.service.GetUserPaymentDetails(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err = actions.service.UpdateWithdrawLimitByUser(externalSystem, *pd, decWithdrawLimit)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, "OK")
}

func (actions *Actions) GetWithdrawLimitsByUser(c *gin.Context) {
	userID, _ := getUserID(c)

	pd, err := actions.service.GetUserPaymentDetails(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(200, map[string]string{
		"withdraw_limit":                utils.FmtDecimal(pd.WithdrawLimit),
		"adv_cash_withdraw_limit":       utils.FmtDecimal(pd.AdvCashWithdrawLimit),
		"clear_junction_withdraw_limit": utils.FmtDecimal(pd.ClearJunctionWithdrawLimit),
		"default_withdraw_limit":        utils.FmtDecimal(pd.DefaultWithdrawLimit),
	})
}

// GenerateMissingAddressesForUser godoc
func (actions *Actions) GenerateMissingAddressesForUser(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.Atoi(id)
	err := actions.service.WalletCreateMissingDepositAddresses(uint64(userID))
	if err != nil {
		log.Error().Err(err).
			Str("section", "admin").Str("action", "admin:gen_missing_addr").Uint64("user_id", uint64(userID)).
			Msg("Unable to create deposit addresses")
		abortWithError(c, 500, "Unable to create missing deposit addresses for user")
		return
	}
	c.JSON(200, "OK")
}

func (actions *Actions) BlockByUser(c *gin.Context) {
	id := c.Param("user_id")
	switcher := c.Param("switcher")
	coinTyp, _ := c.GetPostForm("type")
	coinType := model.CoinType(coinTyp)
	aType := c.Param("action_type")
	actionType := model.ActionType(aType)

	uID, err := strconv.Atoi(id)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}
	userID := uint64(uID)

	if !coinType.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Incorrect coin type")
		return
	}

	if !actionType.IsValid() {
		abortWithError(c, 400, "Invalid action type")
		return
	}

	pd, err := actions.service.GetUserPaymentDetails(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	switch actionType {
	case model.ActionType_Withdraw:
		err = actions.service.BlockWithdrawByUser(coinType, switcher, pd)
		if err != nil {
			abortWithError(c, 500, "Unable to block withdrawals for user")
			return
		}

	case model.ActionType_Deposit:
		err = actions.service.BlockDepositByUser(coinType, switcher, pd)
		if err != nil {
			abortWithError(c, 500, "Unable to block withdrawals for user")
			return
		}
	}

	c.JSON(200, "OK")
}

func (actions *Actions) Block(c *gin.Context) {
	switcher := c.Param("switcher")
	aType := c.Param("action_type")
	actionType := model.ActionType(aType)

	if !actionType.IsValid() {
		abortWithError(c, 400, "Invalid action type")
		return
	}

	switch actionType {
	case model.ActionType_Withdraw:
		err := actions.service.BlockWithdraw(switcher)
		if err != nil {
			abortWithError(c, 500, err.Error())
			return
		}

	case model.ActionType_Deposit:
		err := actions.service.BlockDeposit(switcher)
		if err != nil {
			abortWithError(c, 500, err.Error())
			return
		}
	}

	c.JSON(200, "OK")
}

func (actions *Actions) BlockByCoin(c *gin.Context) {
	coinSymbol := c.Param("coin_symbol")
	switcher := c.Param("switcher")
	aType := c.Param("action_type")
	actionType := model.ActionType(aType)

	if !actionType.IsValid() {
		abortWithError(c, 400, "Invalid action type")
		return
	}

	coin, err := actions.service.GetCoin(coinSymbol)
	if err != nil {
		abortWithError(c, 400, "Invalid coin symbol")
		return
	}

	switch actionType {
	case model.ActionType_Withdraw:
		_, err = actions.service.BlockWithdrawByCoin(coin, switcher)
		if err != nil {
			abortWithError(c, 500, "Unable to block coin deposits")
			return
		}

	case model.ActionType_Deposit:
		_, err = actions.service.BlockDepositByCoin(coin, switcher)
		if err != nil {
			abortWithError(c, 500, "Unable to block coin deposits")
			return
		}
	}

	c.JSON(200, "OK")
}

func (actions *Actions) IsPaymentBlock(c *gin.Context) {
	id := c.Param("user_id")
	uID, err := strconv.Atoi(id)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := uint64(uID)

	pd, err := actions.service.GetUserPaymentDetails(userID)
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

	c.JSON(200, map[string]interface{}{
		"all": adminFeatureSettings,
		"user": map[string]bool{
			"withdraw_crypto": pd.BlockWithdrawCrypto,
			"withdraw_fiat":   pd.BlockWithdrawFiat,
			"deposit_crypto":  pd.BlockDepositCrypto,
			"deposit_fiat":    pd.BlockDepositFiat,
		},
	})
}

func (actions *Actions) CreateAddressForUser(c *gin.Context) {
	userID, _ := getUserID(c)
	symbol := c.Param("symbol")
	coin, err := actions.service.GetCoin(symbol)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Invalid coin symbol")
		return
	}

	_, err = actions.service.GetRepo().IsDepositBlocked(userID, coin)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err = actions.service.WalletCreateDepositAddress(userID, coin)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(200, "OK")
}

func (actions *Actions) MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start the timer
		startTime := time.Now()

		// Process the request
		c.Next()

		// Get the request route
		route := c.FullPath()

		// Get the request method
		reqMethod := c.Request.Method

		// Calculate the duration of the request
		duration := time.Since(startTime)

		// Increment the counters
		apiRequestsTotal.WithLabelValues(route, reqMethod).Inc()
		apiRequestDuration.WithLabelValues(route, reqMethod).Observe(duration.Seconds())
	}
}
