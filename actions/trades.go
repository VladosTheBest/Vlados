package actions

import (
	"github.com/rs/zerolog/log"
	"net/http"
	"strconv"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/queries"
)

// GetTradeHistory godoc
// swagger:route GET /markets/{market}/history market get_market_trades
// Get trades
//
// Get last 100 trades history for a market
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Responses:
//	  200: PublicTrades
//	  500: RequestErrorResp
func (actions *Actions) GetTradeHistory(c *gin.Context) {
	iMarket, _ := c.Get("data_market")
	market := iMarket.(*model.Market)
	data, err := actions.service.GetTrades(market.ID, 100, 1)
	if err != nil {
		abortWithError(c, 500, "Unable to get trade history")
		return
	}
	c.JSON(200, data)
}

// GetTrades godoc
// swagger:route GET /trades/{market} trades get_user_trades
// Get latest trades
//
// Get last 100 trades history in a market for the user
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security:
//	  UserToken:
//
//	Responses:
//	  200: UserTrades
//	  500: RequestErrorResp
func (actions *Actions) GetTrades(c *gin.Context) {
	userID, _ := getUserID(c)
	iMarket, _ := c.Get("data_market")
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 25)
	stimestamp := c.Query("since")
	timestamp, err := strconv.ParseInt(stimestamp, 10, 64)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, 500, err.Error())
		return
	}

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	sortAsc := false
	market := iMarket.(*model.Market)
	data, meta, err := actions.service.GetUserTrades(market.ID, userID, timestamp, sortAsc, limit, page, account)
	if err != nil {
		abortWithError(c, 500, "Unable to get user trade history")
		return
	}
	c.JSON(200, map[string]interface{}{
		"data": data,
		"meta": meta,
	})
}

// GetUserTradesOrFees godoc
// swagger:route GET /users/trades trades list_user_trades
// Get user trades
//
// Get a paginated list of trades for the user
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security:
//	  UserToken:
//
//	Responses:
//	  200: UserTrades
//	  500: RequestErrorResp
func (actions *Actions) GetUserTradesOrFees(c *gin.Context) {
	userID, _ := getUserID(c)
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
		abortWithError(c, NotFound, err.Error())
		return
	}

	if limit > 100 {
		limit = 100
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "trades.go").
			Str("GetUserTradesOrFees", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	data, err := actions.service.GetUserTradeHistory(userID, status, limit, page, from, to, side, markets, query, sort, account)
	if err != nil {
		abortWithError(c, 500, "Unable to get user trade history")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) GetUserTradesWithOrders(c *gin.Context) {
	userID, _ := getUserID(c)
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 100)
	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)
	excludeSelfTrades, _ := strconv.ParseBool(c.Query("excludeSelfTrades"))

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	if limit > 1000 {
		limit = 1000
	}

	data, err := actions.service.GetUserTradesWithOrders(userID, from, to, account, excludeSelfTrades, limit, page)
	if err != nil {
		abortWithError(c, 500, "Unable to get user trade history")
		return
	}
	c.JSON(200, data)
}

// ExportUserFees godoc
// swagger:route GET /users/fees/export trades export_user_fees
// Export fees
//
// Export user fee data
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security:
//	  UserToken:
//
//	Responses:
//	  200: GeneratedFile
//	  500: RequestErrorResp
func (actions *Actions) ExportUserFees(c *gin.Context) {
	userID, _ := getUserID(c)
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

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "trades.go").
			Str("ExportUserFees", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
	}

	// 0 limit to get all data, no paging
	trades, err := actions.service.GetUserTradeHistory(userID, status, 0, 1, from, to, side, markets, query, sort, account)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}

	data, err := actions.service.ExportUserFees(format, status, trades.Trades.Trades)
	if err != nil {
		abortWithError(c, 500, "Unable to export data")
		return
	}
	c.JSON(200, data)
}

// ValidateTradePassword - middleware to validate trade password if set
func (actions *Actions) ValidateTradePassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		return

		// skip trade password check for api keys
		if getBoolFromContext(c, "auth_is_api_key") {
			c.Next()
			return
		}
		userID, _ := getUserID(c)
		// get the settings from the database or exist with 500 on error
		settings, err := queries.GetRepo().GetUserSettings(userID)
		if err != nil {
			abortWithError(c, ServerError, "Cannot get users trade password")
			return
		}

		// if no Trade password is set then continue
		if settings.TradePassword == "" {
			c.Next()
			return
		}
		// get trade password received from request
		tradePassword, _ := c.GetPostForm("trade_password")

		// Validate password
		if !settings.ValidatePass(tradePassword) {
			abortWithError(c, Unauthorized, "Invalid trade password")
			return
		}
		c.Next()
	}
}
