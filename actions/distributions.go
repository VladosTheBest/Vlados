package actions

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// GetUserDistributionsByID returns the distribution entries for a given user
func (actions *Actions) GetUserDistributionsByID(c *gin.Context) {
	userID, _ := strconv.Atoi(c.Param("user_id"))
	page, limit := getPagination(c)

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	data, err := actions.service.GetDistributionsForUser(uint64(userID), limit, page, from, to)
	if err != nil {
		c.AbortWithStatusJSON(500, map[string]string{"error": "Unable to get user distribution history", "error_tip": ""})
		return
	}
	c.JSON(200, data)
}

// GetUserDistributions godoc
// swagger:route GET /users/distributions prdx_distribution get_user_distributions
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
func (actions *Actions) GetUserDistributions(c *gin.Context) {
	userID, _ := getUserID(c)
	page, limit := getPagination(c)

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	data, err := actions.service.GetDistributionsForUser(userID, limit, page, from, to)
	if err != nil {
		abortWithError(c, 500, "Unable to get user distribution history")
		return
	}
	c.JSON(200, data)
}

// GetUserDistributionsExport godoc
// swagger:route GET /users/distributions/export prdx_distribution export_user_distributions
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
func (actions *Actions) GetUserDistributionsExport(c *gin.Context) {
	userID, _ := getUserID(c)
	page, limit := getPagination(c)
	format := c.Query("format")

	fromDate := c.Query("fromDate")
	from, _ := strconv.Atoi(fromDate)
	toDate := c.Query("toDate")
	to, _ := strconv.Atoi(toDate)

	liabilities, err := actions.service.GetDistributionsForUser(userID, limit, page, from, to)
	if err != nil {
		abortWithError(c, 500, "Unable to export data")
		return
	}
	data, err := actions.service.ExportDistributionsForUser(format, liabilities.Liabilities)
	if err != nil {
		abortWithError(c, 404, "Unable to export data")
		return
	}
	c.JSON(200, data)
}

// GetDistributedBonus returns the number of distributed bonus from the system
func (actions *Actions) GetDistributedBonus(c *gin.Context) {
	number, err := actions.service.GetDistributedBonus()
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "distributions:GetDistributedBonus").
			Msg("Unable to get distributed bonus")
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, number)
}

func (actions *Actions) GetDistributionEvents(c *gin.Context) {
	page, limit := getPagination(c)
	events, err := actions.service.GetDistributionEvents(limit, page)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, events)
}

// GetDistributionOrders returns the distribution orders for a given distribution event
func (actions *Actions) GetDistributionOrders(c *gin.Context) {
	id := c.Param("distribution_id")
	page, limit := getPagination(c)
	orders, err := actions.service.GetDistributionOrders(id, limit, page)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, orders)
}

// GetDistributionEntries returns the distribution entries for a given distribution event
func (actions *Actions) GetDistributionEntries(c *gin.Context) {
	id := c.Param("distribution_id")
	page, limit := getPagination(c)
	entries, err := actions.service.GetDistributionEntries(id, limit, page)
	if err != nil {
		abortWithError(c, 500, err.Error())
		return
	}
	c.JSON(200, entries)
}
