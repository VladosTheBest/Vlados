package actions

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
)

func (actions *Actions) CreateCardPaymentAccount(c *gin.Context) {
	if !featureflags.IsEnabled("api.cards.enable-account") {
		abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
		return
	}

	userID, _ := getUserID(c)

	if _, err := actions.service.CreateCardPaymentAccount(userID); err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusBadRequest, fmt.Sprintf("unable to create card account. Reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Card account created successfully",
	})
}
