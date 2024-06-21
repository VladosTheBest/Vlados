package actions

import (
	"errors"
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	cache "gitlab.com/paramountdax-exchange/exchange_api_v2/cache/userbalance"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// Withdraw godoc
// swagger:route POST /wallets/withdraw/{symbol} wallet withdraw
// Withdraw funds
//
// Send a withdraw request to move funds outside the exchange
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
//			   UserToken:
//
//	    Responses:
//	      default: RequestErrorResp
//	      200: WithdrawRequest
//	      400: RequestErrorResp
//	      404: RequestErrorResp
func (actions *Actions) Withdraw(c *gin.Context) {
	log := getlog(c)
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	apCode, _ := actions.service.GetAntiPhishingCode(user.ID)
	symbol := c.Param("symbol")
	address, _ := c.GetPostForm("address")
	amount, _ := c.GetPostForm("amount")
	externalSystem, _ := c.GetPostForm("system")
	reqData, _ := c.GetPostForm("data")
	externalSystemValue := model.WithdrawExternalSystem(externalSystem)

	if !externalSystemValue.IsValid() {
		externalSystemValue = model.WithdrawExternalSystem_Bitgo
	}

	decAmount, ok := (&decimal.Big{}).SetString(amount)
	if !ok {
		abortWithError(c, BadRequest, "Invalid amount provided")
		return
	}

	err := actions.service.IsWithdrawBlocked(user.ID, symbol, externalSystemValue)
	if err != nil {
		abortWithError(c, BadRequest, err.Error())
		return
	}

	coin, err := actions.service.GetCoin(symbol)
	if err != nil {
		abortWithError(c, NotFound, "Invalid coin symbol")
		return
	}
	if externalSystemValue == model.WithdrawExternalSystem_Bitgo {
		isValidate, err := validateAddress(coin.ChainSymbol, address)
		if err != nil {
			abortWithError(c, BadRequest, err.Error())
			return
		}
		if !isValidate {
			abortWithError(c, BadRequest, "Invalid crypto address provided")
			return
		}
	}
	decAmount.Context = decimal.Context128
	decAmount.Context.RoundingMode = decimal.ToZero
	decAmount.Quantize(coin.TokenPrecision)
	// check if the min withdraw amount was reached
	if coin.MinWithdraw.V.Cmp(decAmount) > 0 {
		abortWithError(c, BadRequest, fmt.Sprintf("Minimum withdraw amount %v not reached", coin.MinWithdraw.V))
		return
	}

	withdrawFeeWithPaymentMethod := conv.NewDecimalWithPrecision()
	switch model.WithdrawExternalSystem(externalSystem) {
	case model.WithdrawExternalSystem_Default,
		model.WithdrawExternalSystem_Bitgo:
		withdrawFeeWithPaymentMethod = coin.WithdrawFee.V
	case model.WithdrawExternalSystem_Advcash:
		withdrawFeeWithPaymentMethod = coin.WithdrawFeeAdvCash.V
	case model.WithdrawExternalSystem_ClearJunction:
		withdrawFeeWithPaymentMethod = coin.WithdrawFeeClearJunction.V
	}

	// Check if the amount is higher than the fees that should be paid
	if withdrawFeeWithPaymentMethod.Cmp(decAmount) >= 0 {
		abortWithError(c, BadRequest, fmt.Sprintf("Minimum fee amount %v not reached", withdrawFeeWithPaymentMethod))
		return
	}

	// check if user is allowed to withdraw amount
	allowed, err := actions.service.CanUserWithdraw(user.ID, symbol, decAmount, externalSystemValue)
	if err != nil {
		abortWithError(c, BadRequest, "An error occurred please try again later")
		return
	}
	if !allowed {
		abortWithError(c, BadRequest, "Withdraw amount is higher then user limit")
		return
	}

	// calculate the payable amount trimmed to the lowest number of decimals
	payable := conv.NewDecimalWithPrecision().Sub(decAmount, withdrawFeeWithPaymentMethod)
	payable.Context = decimal.Context128
	payable.Context.RoundingMode = decimal.ToZero
	payable.Quantize(coin.TokenPrecision)

	// check if there are funds for fees
	if payable.Sign() < 1 {
		abortWithError(c, BadRequest, "Insufficient funds to pay for fees")
		return
	}

	account, err := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)
	if err != nil {
		abortWithError(c, BadRequest, "Unable to load account")
		return
	}

	accountBalances, err := actions.service.FundsEngine.GetAccountBalances(user.ID, account.ID)
	if err != nil {
		abortWithError(c, BadRequest, "Unable to load account balances")
		return
	}

	accountBalances.LockAccount()
	defer accountBalances.UnlockAccount()

	// @todo maybe do this after the use clicks the approve email
	// add the withdraw request to the database
	withdrawRequest, err := actions.service.AddWithdrawRequest(user.ID, coin.Symbol, payable, withdrawFeeWithPaymentMethod, address, account, externalSystemValue, reqData, accountBalances)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "actions").
			Str("action", "wallet:withdraw_request").
			Str("amount", amount).
			Str("coin", symbol).
			Str("system", externalSystem).
			Uint64("user_id", user.ID).
			Msg("Unable to add withdraw request")
		abortWithError(c, NotFound, err.Error())
		return
	}
	log.Info().
		Str("section", "actions").
		Str("action", "wallet:withdraw_request").
		Str("amount", amount).
		Str("coin", symbol).
		Str("system", externalSystem).
		Str("withdraw_request_id", withdrawRequest.ID).
		Uint64("user_id", user.ID).
		Msg("Withdraw amount deducted from user balance")
	//  block user funds
	err = actions.service.BlockUserFunds(user.ID, coin.Symbol, withdrawRequest, accountBalances)
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "wallet:withdraw_request").
			Str("amount", amount).
			Str("coin", symbol).
			Str("system", externalSystem).
			Str("withdraw_request_id", withdrawRequest.ID).
			Uint64("user_id", user.ID).
			Msg("Unable to block user funds")
		abortWithError(c, ServerError, "Unable to block user funds")
		return
	}
	// Create and send email verification for the withdraw action
	actionData := model.WithdrawActionData{WithdrawRequestID: withdrawRequest.ID}
	data, err := actionData.ToString()
	if err != nil {
		abortWithError(c, ServerError, "Unexpected conversion error")
		return
	}
	action, err := actions.service.CreateAction(user.ID, model.ActionType_Withdraw, data)
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "wallet:withdraw_request").
			Str("amount", amount).
			Str("coin", symbol).
			Str("withdraw_request_id", withdrawRequest.ID).
			Uint64("user_id", user.ID).
			Msg("Unable to generate action for withdraw request. No emails sent. Funds locked")
		abortWithError(c, ServerError, "Unable to send email confirmations for withdraw. Please contact support")
		return
	}

	// send notification to user
	err = actions.service.NotifyAction(action, map[string]string{
		"coin":       coin.Symbol,
		"amount":     decAmount.String(),
		"payable":    payable.String(),
		"fee_amount": withdrawFeeWithPaymentMethod.String(),
		"address":    address,
		"apcode":     apCode,
		"date":       time.Now().Format(time.RFC822),
	}, user.Email)

	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "wallet:withdraw_request").
			Str("amount", amount).
			Str("coin", symbol).
			Str("withdraw_request_id", withdrawRequest.ID).
			Uint64("user_id", user.ID).
			Msg("Unable to send confirmation email for withdraw request. No emails sent. Funds locked")
		abortWithError(c, ServerError, "Unable to send email confirmations for withdraw. Please contact support")
		return
	}
	// push balance update
	cache.SetWithPublish(user.ID, account.ID)

	//send notification to user
	var relatedObject model.RelatedObjectType
	switch externalSystemValue {
	case model.WithdrawExternalSystem_Advcash,
		model.WithdrawExternalSystem_ClearJunction:
		relatedObject = model.Notification_Withdraw_Fiat
	case model.WithdrawExternalSystem_Bitgo:
		relatedObject = model.Notification_Withdraw_Crypto
	}

	withdrawAmount := withdrawRequest.Amount.V.Quantize(coin.TokenPrecision)
	_, err = actions.service.SendNotification(user.ID, model.NotificationType_Info,
		model.NotificationTitle_Withdraw.String(),
		fmt.Sprintf(model.NotificationMessage_Withdraw.String(), withdrawAmount),
		relatedObject, withdrawRequest.ID)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	// send emails for withdraw request
	go func() {
		if user.EmailStatus.IsAllowed() {
			userDetails := model.UserDetails{}
			db := actions.service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", user.ID)
			if db.Error != nil {
				log.Error().Err(db.Error).
					Str("section", "actions").
					Str("action", "wallet:SendEmailForWithdrawRequest").
					Msg("Unable to send email for withdraw request")
			}
			err := actions.service.SendEmailForWithdrawRequest(user.Email, decAmount.String(), coin.Symbol, userDetails.Timezone)
			if err != nil {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:SendEmailForWithdrawRequest").
					Msg("Unable to send email for withdraw request")
			}
		}
	}()
	// send response to user
	c.JSON(OK, withdrawRequest)
}

// GetWithdrawRequest handler
func (actions *Actions) GetWithdrawRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("withdraw_request_id")
		wr, err := actions.service.GetWithdrawRequest(id)
		if err != nil {
			abortWithError(c, NotFound, "Withdraw request not found")
			return
		}
		c.Set("data_withdraw_request", wr)
		c.Next()
	}
}

// CancelWithdrawRequest
func (actions *Actions) CancelWithdrawRequest(c *gin.Context) {
	id := c.Param("id")
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	_, err := actions.service.CancelWithdrawRequest(user.ID, id)
	if err != nil {
		abortWithError(c, NotFound, "Unable to cancel withdraw request")
		return
	}

	account, _ := subAccounts.GetUserMainSubAccount(model.MarketTypeSpot, user.ID, model.AccountGroupMain)
	// push balance update
	cache.SetWithPublish(user.ID, account.ID)

	// send response to user
	c.JSON(OK, "The withdrawal was canceled")
}

// CancelUserWithdraw - cancel a user withdraw by ID
func (actions *Actions) CancelUserWithdraw(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		abortWithError(c, NotFound, "Unable to cancel withdraw request")
		return
	}

	_, err = actions.service.CancelWithdrawRequest(userID, id)
	if err != nil {
		abortWithError(c, NotFound, "Unable to cancel withdraw request")
		return
	}
	c.JSON(200, "The withdrawal was canceled")
}

// Get user transactions by ID (withdrawals / deposits)
func (actions *Actions) GetUserTransactionsById(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.Atoi(id)

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	market := c.Query("marketParam")
	transactionType := c.Query("type")
	status := c.Query("status")
	query := c.Query("query")

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	transactions, err := actions.service.GetUserTransactions(uint64(userID), limit, page, from, to, market, transactionType, status, query)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get user withdrawal history", "error_tip": ""})
		return
	}
	c.JSON(200, transactions)
}

// GetUserWithdrawsByID - get user withdraws request by ID
func (actions *Actions) GetUserWithdrawsByID(c *gin.Context) {
	id := c.Param("user_id")
	userID, _ := strconv.Atoi(id)

	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	status := c.Query("status")
	query := c.Query("query")
	transactions, err := actions.service.GetUserWithdraws(uint64(userID), status, limit, page, query)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get user withdrawal history", "error_tip": ""})
		return
	}
	c.JSON(200, transactions)
}

// ProcessWithdrawRequest godoc
// swagger:route POST /withdraw_requests/{withdraw_request_id} admin reprocess_withdraw
// Process request
//
// Resend withdraw request for processing in case it's stuck
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
//	      default: RequestErrorResp
//	      200: StringResp
//	      404: RequestErrorResp
func (actions *Actions) ProcessWithdrawRequest(c *gin.Context) {
	log := getlog(c)
	iRequest, _ := c.Get("data_withdraw_request")
	request := iRequest.(*model.WithdrawRequest)

	coin, err := actions.service.GetCoin(request.CoinSymbol)
	if err != nil {
		abortWithError(c, NotFound, "Unable to process withdraw request")
		return
	}

	// send the request over to the wallet
	err = actions.service.WalletCreateWithdrawRequest(
		request.UserID,
		request.ID,
		coin.ChainSymbol,
		coin.Symbol,
		request.Amount.V,
		request.To,
		coin.TokenPrecision,
		request.ExternalSystem,
	)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "actions").
			Str("action", "wallet:withdraw_request").
			Uint64("user_id", request.UserID).
			Str("withdraw_request_id", request.ID).
			Msg("Error trasmitting withdraw request")
		abortWithError(c, ServerError, "Unable to process withdraw request")
		return
	}
	// send response to user
	c.Status(OK)
}

func validateAddress(chainSymbol, address string) (bool, error) {
	chainSymbol = strings.ToLower(chainSymbol)
	if chainSymbol == "xrp" {
		address = strings.Split(address, "?")[0]
	}
	if chainSymbol == "eos" {
		address = strings.Split(address, "?")[0]
	}

	var cryptoRegexMap = map[string]string{
		"xrp":  "^(r|X)[a-zA-Z0-9]{32,46}$",
		"btc":  "^(bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}$",
		"ltc":  "^[LM3][a-km-zA-HJ-NP-Z1-9]{26,33}$",
		"bch":  "^([13][a-km-zA-HJ-NP-Z1-9]{25,34})|^((bitcoincash:)?(q|p)[a-z0-9]{41})|^((BITCOINCASH:)?(Q|P)[A-Z0-9]{41})$",
		"eos":  "^[a-z0-9]{12}$",
		"eth":  "^(0x)[a-zA-Z0-9]{40}$",
		"dash": "^(X|7)[1-9A-HJ-NP-Za-km-z]{33}$",
	}
	regex, ok := cryptoRegexMap[chainSymbol]
	if !ok {
		return false, errors.New("chain symbol not found")
	}
	re := regexp.MustCompile(regex)

	return re.MatchString(address), nil
}

func (actions *Actions) WithdrawFeeAdmin(c *gin.Context) {
	withdrawFee, _ := c.GetPostForm("withdraw_fee")
	exSystem, _ := c.GetPostForm("external_system")
	externalSystem := model.WithdrawExternalSystem(exSystem)

	if !externalSystem.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Payment Method parameter is wrong")
		return
	}

	err := actions.service.UpdateFiatFee(withdrawFee, externalSystem)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, "OK")

}

func (actions *Actions) WithdrawFiatFees(c *gin.Context) {
	coin, err := actions.service.GetCoin("eur")
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"adv_cash_fee":         coin.WithdrawFeeAdvCash.V.Quantize(coin.TokenPrecision),
		"clear_junction_fee":   coin.WithdrawFeeClearJunction.V.Quantize(coin.TokenPrecision),
		"default_withdraw_fee": coin.WithdrawFee.V.Quantize(coin.TokenPrecision),
	})
}

func (actions *Actions) SaveUserAddress(c *gin.Context) {
	userID, _ := getUserID(c)
	userWithdrawAddress := model.UserWithdrawAddress{}
	if err := c.ShouldBind(&userWithdrawAddress); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	userWithdrawAddress.UserID = userID

	err := actions.service.SaveWalletAddress(userWithdrawAddress)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(200, userWithdrawAddress)
}

func (actions *Actions) UserAddresses(c *gin.Context) {
	userID, _ := getUserID(c)

	userAddresses, err := actions.service.WalletAddressesByUser(userID)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(200, userAddresses)
}

func (actions *Actions) DeleteUserAddress(c *gin.Context) {
	userID, _ := getUserID(c)
	address := c.Query("address")

	err := actions.service.DeleteWalletAddress(userID, address)
	if err != nil {
		abortWithError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(200, "OK")
}
