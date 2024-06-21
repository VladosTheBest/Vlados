package actions

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (actions *Actions) GetPnl24h(c *gin.Context) {
	userID, _ := getUserID(c)

	accountID, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, userID, accountID)
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	data, err := actions.service.GetPercentFromBalance24h(userID, account)
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	c.JSON(200, data)
}

func (actions *Actions) GetPnlWeek(c *gin.Context) {
	userID, _ := getUserID(c)
	accountID, err := subAccounts.ConvertAccountGroupToAccount(userID, c.Query("account"))
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	account, err := subAccounts.GetUserSubAccountByID(model.MarketTypeSpot, userID, accountID)
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	data, err := actions.service.Balances24hForWeek(userID, account)
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	c.JSON(200, data)
}
