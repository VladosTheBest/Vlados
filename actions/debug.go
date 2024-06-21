package actions

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (actions *Actions) AdminDebugFeatures(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
	})
}
