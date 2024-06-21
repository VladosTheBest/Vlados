package actions

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (actions *Actions) GetActivityMonitorList(c *gin.Context) {
	data, err := actions.service.GetActivityMonitorList()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Get Admin Launchpad", "error_tip": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}
