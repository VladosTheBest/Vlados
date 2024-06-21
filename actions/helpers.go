package actions

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/httpagent"
	"net/http"
	"net/url"
)

func (actions *Actions) GetCityHelper(c *gin.Context) {
	u, err := url.Parse("http://geodb-free-service.wirefreethought.com/v1/geo/cities")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "City Helper", "error_tip": err.Error()})
		return
	}

	u.RawQuery = c.Request.URL.RawQuery

	_, body, err := httpagent.Get(u.String())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Type", "application/json; charset=utf-8")
	_, _ = c.Writer.Write(body)
	c.Status(http.StatusOK)
}
