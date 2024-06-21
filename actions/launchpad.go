package actions

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/conv"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
)

func (actions *Actions) GetLaunchpadList(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	launchpadList, err := actions.service.GetLaunchpadFullInfoList(user.ID)
	l := log.With().
		Str("section", "launchpad").
		Str("action", "GetLaunchpadList").
		Logger()
	if err != nil {
		l.Error().Err(err).Msg("Unable to get launchpadList")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to get launchpadList",
		})
		return
	}

	c.JSON(http.StatusOK, launchpadList)
}

func (actions *Actions) GetLaunchpad(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	launchpadId := c.Param("launchpad_id")
	l := log.With().
		Str("section", "launchpad").
		Str("action", "GetLaunchpad").
		Uint64("user_id", user.ID).
		Str("launchpad_id", launchpadId).
		Logger()

	launchpad, err := actions.service.GetLaunchpadFullInfo(launchpadId, user.ID)
	if err != nil {
		l.Error().Err(err).Msg("Unable to get launchpad")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to get launchpad",
		})
		return
	}

	c.JSON(http.StatusOK, launchpad)
}

func (actions *Actions) LaunchpadMakePayment(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)
	launchpadId := c.Param("launchpad_id")
	amount, _ := c.GetPostForm("amount")

	l := log.With().
		Str("section", "launchpad").
		Str("action", "LaunchpadMakePayment").
		Uint64("user_id", user.ID).
		Str("launchpad_id", launchpadId).
		Logger()
	decAmount, _ := conv.NewDecimalWithPrecision().SetString(amount)
	err := actions.service.LaunchpadMakePayment(launchpadId, user, decAmount)

	if err != nil {
		l.Error().Err(err).Msg("Unable to buy launchpad")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to buy launchpad",
		})
		return
	}

	c.JSON(http.StatusOK, nil)
}
