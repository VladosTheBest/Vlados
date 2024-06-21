package actions

import (
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetReferrals godoc
// swagger:route GET /referrals
// Get Referrals
//
// Returns the current user's list of referrals and total count of refered users.
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Referrals
//	      404: RequestErrorResp
func (actions *Actions) GetReferrals(c *gin.Context) {
	if !featureflags.IsEnabled("api.referrals_v2") {
		abortWithError(c, http.StatusBadRequest, "service temporary unavailable")
		return
	}

	userID, _ := getUserID(c)
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 3)

	data, err := actions.service.GetReferrals(userID, limit, page)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get referrals")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetTopInviters godoc
// swagger:route GET /referrals/topinviters
// Get TopInviters
//
// Returns limited user list having the most referrals
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: UsersWithReferralCount
//	      404: RequestErrorResp
func (actions *Actions) GetTopInviters(c *gin.Context) {
	data, err := actions.service.GetTopInviters()
	if err != nil {
		log.Print(err)
		abortWithError(c, http.StatusNotFound, "Unable to get top inviters")
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetReferralEarningsTotal godoc
// swagger:route GET /referrals/earnings/total
// Get Earnings total
//
// Returns total of comissons earned from refered users trade fees
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: Decimal
//	      404: RequestErrorResp
func (actions *Actions) GetReferralEarningsTotal(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.GetReferralEarningsTotal(userID)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get earnings total")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetReferralEarningsTotalAll(c *gin.Context) {
	data, err := actions.service.GetReferralEarningsTotalAll()
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get earnings total")
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetReferralEarningsTotalByUser(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.GetReferralEarningsTotalAllByUser(userID)
	if err != nil {
		abortWithError(c, http.StatusNotFound, "Unable to get earnings total")
		return
	}
	c.JSON(http.StatusOK, data)
}
