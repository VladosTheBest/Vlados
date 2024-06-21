package actions

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
)

func (actions *Actions) GetMonitoringOptions(c *gin.Context) {
	page, limit := getPagination(c)

	data, err := actions.service.GetMonitoringOptionsList(page, limit)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
	}

	c.JSON(http.StatusOK, data)
}

func (actions *Actions) UpdateMonitoringOption(c *gin.Context) {
	var updateRequest model.MonitoringOptionUpdateRequest
	if err := c.ShouldBind(updateRequest); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
	}

	data, err := actions.service.UpdateMonitoringOption(updateRequest)

	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
	}

	c.JSON(http.StatusOK, data)
}
