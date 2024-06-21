package actions

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/payments/clear_junction"
)

func (actions *Actions) ClearJunctionDepositRequest(c *gin.Context) {
	userID, _ := getUserID(c)

	pd, err := actions.service.GetUserPaymentDetails(userID)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"clientRefCode":  pd.ReferenceCode,
			"allowedSymbols": actions.cfg.ClearJunction.Assets,
			"requisites":     actions.cfg.ClearJunction.Requisites,
		},
	})
}

func (actions *Actions) ClearJunctionWithdrawalSettings(c *gin.Context) {
	userID, _ := getUserID(c)

	user, err := actions.service.GetUserByID(uint(userID))
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"firstName":      user.FirstName,
			"lastName":       user.LastName,
			"allowedMethods": clear_junction.GetWithdrawalsMethodsList(),
			"requiredFields": clear_junction.GetWithdrawalsMethodsFieldsList(),
		},
	})
}
