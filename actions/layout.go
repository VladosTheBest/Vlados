package actions

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

func (actions *Actions) GetLayouts(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	layouts, count, err := actions.service.GetLayouts(userID)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to get layouts, reason: %s", err.Error()))
		return
	}

	userSettings, err := actions.service.GetProfileSettings(userID)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to get profile settings, reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"layouts":         layouts,
		"count":           count,
		"selected_layout": userSettings.SelectedLayout,
	})
}

func (actions *Actions) SaveLayout(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	layout := &model.Layout{
		OwnerID: userID,
		Name:    c.PostForm("name"),
		Data:    c.PostForm("data"),
	}

	result, err := actions.service.SaveLayout(layout)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to create layout, reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, result)
}

func (actions *Actions) DeleteLayout(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	layoutID, _ := strconv.Atoi(c.PostForm("id"))

	err := actions.service.DeleteLayout(userID, uint64(layoutID))
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to delete layout, reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) UpdateLayout(c *gin.Context) {
	_, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	layoutID, _ := strconv.Atoi(c.PostForm("id"))

	layout := model.Layout{
		ID:   uint64(layoutID),
		Name: c.PostForm("name"),
		Data: c.PostForm("data"),
	}

	err := actions.service.UpdateLayout(layout)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to update layout, reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) SetActiveLayout(c *gin.Context) {
	userID, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	layoutID := c.PostForm("id")

	err := actions.service.UpdateUserSettingsSelectedLayout(userID, layoutID)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to update selected_layout, reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) SortLayouts(c *gin.Context) {
	_, exist := getUserID(c)
	if !exist {
		abortWithError(c, http.StatusBadRequest, "user not found")
		return
	}

	var request []model.SortLayoutsRequest
	if err := c.BindJSON(&request); err != nil {
		abortWithError(c, http.StatusBadRequest, "can't parse data")
		return
	}

	err := actions.service.SortLayouts(request)
	if err != nil {
		_ = c.Error(err)
		abortWithError(c, http.StatusInternalServerError, fmt.Sprintf("unable to change sort_id, reason: %s", err.Error()))
		return
	}

	c.JSON(http.StatusOK, "OK")
}
