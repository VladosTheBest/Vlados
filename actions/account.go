package actions

import (
	"github.com/ericlagergren/decimal"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/cache/subAccounts"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
)

// The CreateSubAccount function is used to create a new sub-account for a user.
func (actions *Actions) CreateSubAccount(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	count, err := actions.service.CountSubAccounts(userID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Failed to count subaccounts")
		return
	}

	if count >= 3 {
		abortWithError(c, http.StatusBadRequest, "Cannot create more than 3 subaccounts")
		return
	}

	var data = &model.SubAccount{}

	if err := c.Bind(data); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Set the user ID before passing the SubAccount instance to the service
	data.UserId = userID

	subAccount, err := actions.service.CreateSubAccount(data)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(200, subAccount)
}

// The GetUserSubAccounts function is used to get all sub-accounts for a user.
func (actions *Actions) GetUserSubAccounts(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	accounts, err := actions.service.GetRepo().GetAllSubAccounts(userID)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Error when getting sub accounts")
		return
	}

	answers, err := actions.service.GetBalancesForSubAccounts(userID, accounts)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Error when getting balance for sub accounts")
		return
	}

	c.JSON(200, answers)
}

func (actions *Actions) GetDefaultUserSubAccounts(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	account, err := actions.service.GetRepo().GetDefaultSubAccount(userID)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Error when getting default sub account")
		return
	}

	c.JSON(200, account)
}

func (actions *Actions) EditSubAccount(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	var data = &model.SubAccount{}

	if err := c.Bind(data); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Ensure the user is the owner of the sub-account
	if userID != data.UserId {
		abortWithError(c, http.StatusBadRequest, "This user does not have a sub account with this ID")
		return
	}

	subAccount, err := actions.service.GetRepo().GetSubAccountByID(data.ID)
	if err != nil || subAccount == nil {
		abortWithError(c, http.StatusNotFound, "Sub account not found")
		return
	}

	// Update the sub-account
	err = actions.service.GetRepo().UpdateSubAccount(subAccount, data)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Failed to update sub account")
		return
	}

	// If the new sub-account is set as default, update the default sub-account in the cache
	if subAccount.IsDefault {
		if err := subAccounts.SetUserSubAccountDefault(subAccount.MarketType, subAccount.UserId, subAccount.AccountGroup, subAccount); err != nil {
			abortWithError(c, http.StatusBadRequest, "error when updating default sub-account in the cache")
			return
		}
	}

	c.JSON(200, subAccount)
}

// TransferSubAccounts handles the transfer of funds between two subaccounts for a user.
func (actions *Actions) TransferSubAccounts(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	// Define a struct for binding the request data.
	var data struct {
		Amount      *decimal.Big `json:"amount"`
		CoinSymbol  string       `json:"coin_symbol"`
		AccountFrom uint64       `json:"account_from"`
		AccountTo   uint64       `json:"account_to"`
	}

	// Bind the request data to the struct.
	if err := c.Bind(&data); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Retrieve the AccountBalances for the source and destination subaccounts using GetAccountBalancesForSubAccounts.
	accountFrom, accountTo, err := actions.service.GetAccountBalancesForSubAccounts(userID, data.AccountFrom, data.AccountTo)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Execute the transfer between the two subaccounts.
	err = actions.service.TransferBetweenSubAccounts(userID, data.Amount, data.CoinSymbol, accountFrom, accountTo)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Failed to transfer between sub accounts")
		return
	}

	// Publish balance update to websocket channel
	actions.PublishTransferBtwSubAccountsUpdate(userID)

	// Return a success message to the client.
	c.JSON(http.StatusOK, gin.H{
		"message": "Transfer between sub accounts successful",
	})
}

func (actions *Actions) AddDefaultSubAccountsToCacheForOldUsers(c *gin.Context) {
	// Get subaccounts where comment = "V2 migration"
	subAccounts, err := actions.service.GetRepo().GetSubAccountsByComment("V2 migration")
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to get subaccounts with comment 'V2 migration'")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Iterate over each subaccount
	for _, subAccount := range subAccounts {
		// Add the subaccount to the FMS service
		_, err := actions.service.FundsEngine.InitAccountBalances(&subAccount, false)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Failed to init balances for subaccount")
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (actions *Actions) GetMainSubAccounBalances(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "User is not logged")
		return
	}

	// The subaccount ID is always 0
	subAccountID := uint64(0)

	// Get the balances for the subaccount
	balances, err := actions.service.GetRepo().GetBalancesForSubaccount(userID, subAccountID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Error when getting the balances")
		return
	}

	c.JSON(200, balances)
}
