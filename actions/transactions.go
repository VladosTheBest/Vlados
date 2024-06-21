package actions

import (
	"errors"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"net/http"

	//"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetUserTransactions godoc
// swagger:route GET /users/transactions wallet get_transactions
// Get transactions
//
// Get all wallet transactions
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
//	  200: UserTransactions
//	  404: RequestErrorResp
func (actions *Actions) GetUserTransactions(c *gin.Context) {
	userID, _ := getUserID(c)
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

	// get transactions
	transactions, err := actions.service.GetUserTransactions(userID, limit, page, from, to, market, transactionType, status, query)
	if err != nil {
		_ = c.AbortWithError(404, errors.New("Not found"))
		return
	}

	c.JSON(200, transactions)
}

// ExportUserTransactions godoc
// swagger:route GET /users/transactions/export wallet export_transactions
// Export transactions
//
// Export all wallet transactions
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
//	  404: RequestErrorResp
func (actions *Actions) ExportUserTransactions(c *gin.Context) {
	userID, _ := getUserID(c)
	format := c.Query("format")
	status := c.Query("status")
	side := c.Query("sideParam")
	market := c.Query("marketParam")
	marketCoinSymbol := c.Query("market_coin_symbol")
	quoteCoinSymbol := c.Query("quote_coin_symbol")
	query := c.Query("query")
	sort := c.Query("sort")
	transactionType := c.Query("transactionType")
	limit := getQueryAsInt(c, "limit", 10)
	page := getQueryAsInt(c, "page", 1)

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	account, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	markets, err := actions.service.LoadMarketIDsByCoin(marketCoinSymbol, quoteCoinSymbol)
	if err != nil {
		log.Error().
			Str("actions", "transactions.go").
			Str("ExportUserTransactions", "LoadMarketIDsByCoin").
			Msg("Unable to load market id's by coin")
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if status == "trades" {
		// 0 limit to get all data, no paging
		trades, err := actions.service.GetUserTradeHistory(userID, status, limit, page, from, to, side, markets, query, sort, account)
		if err != nil {
			_ = c.AbortWithError(500, err)
			return
		}

		data, err := actions.service.ExportUserTrades(userID, format, status, trades.Trades.Trades)
		if err != nil {
			_ = c.AbortWithError(404, errors.New("Could not export"))
			return
		}
		c.JSON(200, data)
	} else {
		// 0 limit to get all data, no paging
		transactions, err := actions.service.GetUserTransactions(userID, limit, page, from, to, market, transactionType, status, query)
		if err != nil {
			_ = c.AbortWithError(500, err)
			return
		}

		data, err := actions.service.ExportUserTransactions(userID, format, status, transactions.Transactions)
		if err != nil {
			_ = c.AbortWithError(404, errors.New("Could not export"))
			return
		}
		c.JSON(200, data)
	}
}
