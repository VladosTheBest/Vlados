package actions

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
)

func (actions *Actions) GetAdminLaunchpadList(c *gin.Context) {
	page, limit := getPagination(c)
	data, err := actions.service.GetAdminLaunchpad(limit, page)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Get Admin Launchpad", "error_tip": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetAdminLaunchpad(c *gin.Context) {
	launchpadId := c.Param("launchpad_id")
	userID, _ := getUserID(c)
	l := log.With().
		Str("section", "launchpad").
		Str("action", "GetAdminLaunchpad").
		Str("launchpad_id", launchpadId).
		Logger()

	launchpad, err := actions.service.GetLaunchpadFullInfo(launchpadId, userID)
	if err != nil {
		l.Error().Err(err).Msg("Unable to get launchpad")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to get launchpad",
		})
		return
	}

	c.JSON(http.StatusOK, launchpad)
}

func (actions *Actions) LaunchpadEndPresale(c *gin.Context) {
	launchpadId := c.Param("launchpad_id")
	l := log.With().
		Str("section", "launchpad").
		Str("action", "LaunchpadEndPresale").
		Str("launchpad_id", launchpadId).
		Logger()

	err := actions.service.LaunchpadEndPresale(launchpadId)
	if err != nil {
		l.Error().Err(err).Msg("Unable to end presale")
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{
			"error": "Unable to end presale",
		})
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (actions *Actions) CreateLaunchpad(c *gin.Context) {

	form, err := c.MultipartForm()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Create Admin Launchpad", "error_tip": err.Error()})
		return
	}

	logos := form.File["logo"]
	if len(logos) != 1 {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Create Admin Launchpad", "error_tip": "Count of logo file should be 1"})
		return
	}
	logo := logos[0]

	var launchpad = model.LaunchpadRequest{}

	if err := c.ShouldBind(&launchpad); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Create Admin Launchpad", "error_tip": err.Error()})
		return
	}

	data, err := actions.service.CreateLaunchpad(&launchpad, logo)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Create Admin Launchpad", "error_tip": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

func (actions *Actions) UpdateLaunchpad(c *gin.Context) {

	launchpadId := c.Param("launchpad_id")

	form, err := c.MultipartForm()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Update Admin Launchpad", "error_tip": err.Error()})
		return
	}

	logos := form.File["logo"]
	if len(logos) != 1 {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Update Admin Launchpad", "error_tip": "Count of logo file should be 1"})
		return
	}
	logo := logos[0]

	var launchpad = model.LaunchpadRequest{}

	if err := c.ShouldBind(&launchpad); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Update Admin Launchpad", "error_tip": err.Error()})
		return
	}

	data, err := actions.service.UpdateLaunchpad(&launchpad, logo, launchpadId)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"error": "Update Admin Launchpad", "error_tip": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}
