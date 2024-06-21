package actions

import (
	"github.com/gin-gonic/gin"
)

// GetAllBurnEvents godoc
// swagger:route GET /burn-events events get_all_burn_events
// List burn events
//
// List all burn events
//
//	Consumes:
//	- application/x-www-form-urlencoded
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security: []
//
//	Responses:
//	  200: Events
//	  404: RequestErrorResp
//	  500: RequestErrorResp
func (actions *Actions) GetAllBurnEvents(c *gin.Context) {
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)
	query := c.Query("date")
	burnEvents, err := actions.service.GetAllBurnEvents(limit, page, query)
	if err != nil {
		c.AbortWithStatusJSON(404, map[string]string{"error": "The burn events could not be retrieved"})
		return
	}
	c.JSON(200, burnEvents)
}

// AddBurnEvent godoc
// swagger:route POST /burn-events events add_burn_event
// Add burn events
//
// Add a burn event
//
//	Consumes:
//	- multipart/form-data
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security: []
//
//	Responses:
//	  200: Event
//	  400: RequestErrorResp
//	  404: RequestErrorResp
func (actions *Actions) AddBurnEvent(c *gin.Context) {
	log := getlog(c)
	volume, _ := c.GetPostForm("volume")
	prdx, _ := c.GetPostForm("prdx")
	date, _ := c.GetPostForm("date")
	data, err := actions.service.AddBurnEvent(volume, prdx, date)
	if err != nil {
		log.Error().Err(err).Str("section", "actions").Str("action", "burn-event:add").Msg("Unable to add burn event")
		abortWithError(c, 500, "Unable to add chain")
		return
	}
	c.JSON(200, data)
}

// RemoveBurnEvent godoc
// swagger:route DELETE /burn-events/:event_id events add_burn_event
// Remove burn events
//
// Remove a burn event
//
//	Consumes:
//	- multipart/form-data
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security: []
//
//	Responses:
//	  200: succes
//	  400: RequestErrorResp
//	  404: RequestErrorResp
func (actions *Actions) RemoveBurnEvent(c *gin.Context) {
	id := c.Param("event_id")
	err := actions.service.RemoveBurnEvent(id)
	if err != nil {
		abortWithError(c, 500, "Unable to remove burn event")
		return
	}
	c.JSON(200, "success")
}
