package actions

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (actions *Actions) CreateOTCOrderQuote(c *gin.Context) {
	_, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}
	amount := c.PostForm("amount")
	commission := c.PostForm("commission")
	primaryCoin := c.PostForm("primary")
	secondaryCoin := c.PostForm("secondary")
	side := c.PostForm("side")

	quote, err := actions.service.CreateOTCOrderQuote(primaryCoin, secondaryCoin, side, amount, commission)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(200, quote)
}
