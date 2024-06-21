package actions

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/featureflags"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
	"runtime"
	"strconv"
)

func (actions *Actions) SystemInfoMaintenance(c *gin.Context) {

	resp, err := actions.service.GetRepo().GetMaintenanceMessages(model.MaintenanceMessageStatusActive)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "SystemInfo").
			Str("action", "SystemInfoMaintenance").
			Msg("Unable to get messages list")
	}

	var timestamp int64 = 0

	for _, message := range resp {
		if message.CreatedAt.Unix() > timestamp {
			timestamp = message.CreatedAt.Unix()
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"success":   featureflags.IsEnabled("api.system-info.maintenance") && len(resp) > 0,
		"data":      resp,
		"timestamp": timestamp,
	})
}

func (actions *Actions) CreateSystemInfoMaintenance(c *gin.Context) {
	newMaintenanceMessage := model.MaintenanceMessage{}
	if err := c.ShouldBind(&newMaintenanceMessage); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		log.Error().
			Err(err).
			Str("section", "SystemInfo").
			Str("action", "CreateSystemInfoMaintenance").
			Msg("Unable to get message")
		return
	}

	var resp *model.MaintenanceMessage
	var err error
	if newMaintenanceMessage.ID == 0 {
		resp, err = actions.service.GetRepo().CreateMaintenanceMessages(newMaintenanceMessage)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			log.Error().
				Err(err).
				Str("section", "SystemInfo").
				Str("action", "CreateSystemInfoMaintenance").
				Msg("Unable to create message")
		}
	} else {
		maintenanceMessage := model.MaintenanceMessage{}
		if err := actions.service.GetRepo().FindByID(&maintenanceMessage, uint(newMaintenanceMessage.ID)); err != nil {
			abortWithError(c, http.StatusBadRequest, "Maintenance message not found")
			return
		}

		resp, err = actions.service.GetRepo().UpdateMaintenanceMessage(newMaintenanceMessage)
		if err != nil {
			log.Error().
				Err(err).
				Str("section", "SystemInfo").
				Str("action", "UpdateMaintenanceMessage").
				Msg("Unable to change message")
		}

	}

	c.JSON(http.StatusOK, resp)
}

func (actions *Actions) ChangeStatusSystemInfoMaintenance(c *gin.Context) {
	maintenanceMessageId, maintenanceMessageIdExist := c.GetPostForm("id")
	statusStr, _ := c.GetPostForm("status")
	status := model.MaintenanceMessageStatus(statusStr)

	if !status.IsValid() {
		abortWithError(c, http.StatusBadRequest, "Status parameter is wrong")
	}

	if !maintenanceMessageIdExist {
		abortWithError(c, http.StatusBadRequest, "id parameter is wrong")
		return
	}

	id, err := strconv.ParseUint(maintenanceMessageId, 10, 64)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Unable to parse id")
		return
	}

	maintenanceMessage := model.MaintenanceMessage{}

	if err := actions.service.GetRepo().FindByID(&maintenanceMessage, uint(id)); err != nil {
		abortWithError(c, http.StatusBadRequest, "Maintenance message not found")
		return
	}

	err = actions.service.GetRepo().ChangeMaintenanceMessagesStatus(maintenanceMessage, status)
	if err != nil {
		log.Error().
			Err(err).
			Str("section", "SystemInfo").
			Str("action", "ChangeMaintenanceMessagesStatus").
			Msg("Unable to change status")
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) SystemInfoProbeLive(c *gin.Context) {
	if runtime.NumGoroutine() > actions.cfg.Server.Debug.MaxNumberOfGoroutines {
		log.Error().
			Int("maxNumber", actions.cfg.Server.Debug.MaxNumberOfGoroutines).
			Int("currentNumber", runtime.NumGoroutine()).
			Str("section", "SystemInfo").
			Str("action", "SystemInfoProbeLive").
			Msg("Arrived max number of goroutines")
		c.Status(http.StatusInternalServerError)
		return
	}

	tx := actions.service.GetRepo().Conn.Begin()
	if tx.Error != nil {
		log.Error().
			Err(tx.Error).
			Str("section", "SystemInfo").
			Str("action", "SystemInfoProbeLive").
			Msg("Unable open new transaction")
		c.Status(http.StatusInternalServerError)
		return
	}
	tx.Rollback()

	c.Status(http.StatusOK)
}
