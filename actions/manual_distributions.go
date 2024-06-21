package actions

import (
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

//// GetUserManualDistributionsByID returns the manual distribution entries for a given user
//func (actions *Actions) GetUserManualDistributionsByID(c *gin.Context) {
//	userID, _ := strconv.Atoi(c.Param("user_id"))
//	page, limit := getPagination(c)
//
//	fromDate := c.Query("fromDate")
//	from, _ := strconv.Atoi(fromDate)
//	toDate := c.Query("toDate")
//	to, _ := strconv.Atoi(toDate)
//
//	data, err := actions.service.GetManualDistributionsForUser(uint64(userID), limit, page, from, to)
//	if err != nil {
//		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get user distribution history", "error_tip": ""})
//		return
//	}
//	c.JSON(200, data)
//}

// GetUserManualDistributions godoc
// swagger:route GET /users/manual-distributions prdx_distribution get_user_distributions
// Get User Distributions
//
// Get a list of distribution events for the current user
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
//	  200: LiabilityListResp
//	  500: RequestErrorResp
func (actions *Actions) GetUserManualDistributions(c *gin.Context) {
	userID, _ := getUserID(c)
	page, limit := getPagination(c)

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	data, err := actions.service.GetManualDistributionsForUser(userID, limit, page, from, to)
	if err != nil {
		abortWithError(c, 500, "Unable to get user distribution history")
		return
	}
	c.JSON(200, data)
}

// GetUserManualDistributionsExport godoc
// swagger:route GET /users/manual-distributions/export prdx_distribution export_user_distributions
// Export User Distributions
//
// Export a list of distribution events for the current user
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
//	  500: RequestErrorResp
func (actions *Actions) GetUserManualDistributionsExport(c *gin.Context) {
	userID, _ := getUserID(c)
	page, limit := getPagination(c)
	format := c.Query("format")

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	liabilities, err := actions.service.GetManualDistributionsForUser(userID, limit, page, from, to)
	if err != nil {
		abortWithError(c, 500, "Unable to export data")
		return
	}
	data, err := actions.service.ExportManualDistributionsForUser(format, liabilities.Liabilities)
	if err != nil {
		abortWithError(c, 404, "Unable to export data")
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) GetManualDistributionInfo(c *gin.Context) {
	userID, _ := getUserID(c)

	data := actions.service.GetManualDistributionInfoByUser(userID)
	c.JSON(200, data)
}

func (actions *Actions) GetManualDistributionGetBonus(c *gin.Context) {
	userID, _ := getUserID(c)
	distId, err := actions.service.GetManualDistributionCurrentID()
	if err != nil {
		abortWithError(c, http.StatusNoContent, "dont have active distribution")
		return
	}

	err = actions.service.GetManualDistributionGetBonus(distId, userID)
	if err != nil {
		log.Error().Str("section", "manual-distribution").Str("method", "GetManualDistributionGetBonus").Err(err).Msg("Unable to get bonus")
		abortWithError(c, http.StatusInternalServerError, "Something wrong")
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Bonus successfully sent to your wallet",
	})
}

func (actions *Actions) GetMarketMakerVolumePercentValue(c *gin.Context) {

	var val string
	err := actions.service.GetRepo().ConnReader.Table("admin_feature_settings").
		Where("feature = ?", model.MANUAL_DISTRIBUTION_PERCENT_FEATURE_KEY).
		Select("value as s").
		Row().
		Scan(&val)

	valDec, _ := conv.NewDecimalWithPrecision().SetString(val)

	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, utils.Fmt(valDec))
}

func (actions *Actions) GetLastMarketMakerPercentValue(c *gin.Context) {
	distributionID, err := actions.service.GetManualDistributionCurrentID()
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	manualDistribution, err := actions.service.GetManualDistributionByID(distributionID)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, utils.Fmt(manualDistribution.MMPercent.V))
}

//
//// GetManualDistributedBonus returns the number of distributed bonus from the system
//func (actions *Actions) GetManualDistributedBonus(c *gin.Context) {
//	number, err := actions.service.GetManualDistributedBonus()
//	if err != nil {
//		log.Error().Err(err).
//			Str("section", "actions").
//			Str("action", "distributions:GetDistributedBonus").
//			Msg("Unable to get distributed bonus")
//		abortWithError(c, 500, err.Error())
//		return
//	}
//	c.JSON(200, number)
//}
//
//// GetManualDistributionEvents godoc
//func (actions *Actions) GetManualDistributionEvents(c *gin.Context) {
//	page, limit := getPagination(c)
//	events, err := actions.service.GetManualDistributionEvents(limit, page)
//	if err != nil {
//		abortWithError(c, 500, err.Error())
//		return
//	}
//	c.JSON(200, events)
//}
//
//// GetManualDistributionOrders returns the distribution orders for a given distribution event
//func (actions *Actions) GetManualDistributionOrders(c *gin.Context) {
//	id := c.Param("distribution_id")
//	page, limit := getPagination(c)
//	orders, err := actions.service.GetDistributionOrders(id, limit, page)
//	if err != nil {
//		abortWithError(c, 500, err.Error())
//		return
//	}
//	c.JSON(200, orders)
//}

//// GetManualDistributionEntries returns the distribution entries for a given distribution event
//func (actions *Actions) GetManualDistributionEntries(c *gin.Context) {
//	id, _ := strconv.Atoi(c.Param("distribution_id"))
//	page, limit := getPagination(c)
//	entries, err := actions.service.GetManualDistributionEntries(uint64(id), limit, page)
//	if err != nil {
//		abortWithError(c, 500, err.Error())
//		return
//	}
//	c.JSON(200, entries)
//}
