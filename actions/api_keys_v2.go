package actions

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"io/ioutil"
	"net/http"
	"strconv"
)

func (actions *Actions) GetAPIKeysV2(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.GetAPIKeysV2(userID)
	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}
	c.JSON(200, data)
}

func (actions *Actions) GenerateAPIKeyV2(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	name, _ := c.GetPostForm("api_key_name")
	tradingAllowed, _ := c.GetPostForm("trading_allowed")
	withdrawalAllowed, _ := c.GetPostForm("withdrawal_allowed")
	marginAllowed, _ := c.GetPostForm("margin_allowed")
	futureAllowed, _ := c.GetPostForm("future_allowed")

	privateKey, apiKey, err := actions.service.GenerateAPIKeyV2(user.ID, name, tradingAllowed, withdrawalAllowed, marginAllowed, futureAllowed)
	if err != nil {
		abortWithError(c, NotFound, err.Error())
		return
	}

	err = actions.service.CreateActionAndSendApproval(model.ActionType_APIKeyV2, user, apiKey.ID, map[string]string{"name": name})
	if err != nil {
		abortWithError(c, ServerError, "Unable to send email confirmations for new api key. Please contact support")
		return
	}

	id := strconv.FormatUint(apiKey.ID, 10)
	_, err = actions.service.SendNotification(user.ID, model.NotificationType_System,
		model.NotificationTitle_NewAPIKey.String(),
		fmt.Sprintf(model.NotificationMessage_NewAPIKeyON.String(), apiKey.Name),
		model.Notification_NewAPIKey, id)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, map[string]interface{}{
		"privateKey": privateKey,
		"publicKey":  apiKey.PublicKey,
	})
}

func (actions *Actions) RemoveAPIKeyV2(c *gin.Context) {
	userID, _ := getUserID(c)
	apiKeyID, _ := c.GetQuery("apikey")

	intApiKeyId, err := strconv.ParseUint(apiKeyID, 0, 64)
	if err != nil {
		abortWithError(c, 404, "Unable to parse API key id")
		return
	}
	apiKey, err := actions.service.GetAPIKeyV2ByID(intApiKeyId)
	if err != nil {
		abortWithError(c, 404, "Unable to get API key")
		return
	}

	err = actions.service.RemoveAPIKeyV2(userID, apiKeyID)
	if err != nil {
		abortWithError(c, 404, "Unable to remove API key")
		return
	}

	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_NewAPIKey.String(), fmt.Sprintf(model.NotificationMessage_NewAPIKeyOFF.String(), apiKey.Name),
		model.Notification_NewAPIKey, apiKeyID)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, true)
}

func (actions *Actions) AddAPIKeyV2AllowedIp(c *gin.Context) {
	request := new(model.AddUserApiKeyV2IpRequest)
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		abortWithError(c, 400, err.Error())
		return
	}

	err = json.Unmarshal(body, &request)
	if err != nil {
		abortWithError(c, 400, err.Error())
		return
	}

	err = actions.service.AddApiKeyAllowedIp(request)

	if err != nil {
		abortWithError(c, 404, err.Error())
		return
	}

	c.JSON(200, "OK")
}

func (actions *Actions) RemoveAPIKeyV2AllowedIp(c *gin.Context) {

	request := new(model.DeleteUserApiKeyV2IpRequest)
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	err = json.Unmarshal(body, &request)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	err = actions.service.RemoveAllowedIp(request.IpId)

	if err != nil {
		abortWithError(c, http.StatusInternalServerError, "Unable to remove Allowed ip")
		return
	}

	c.JSON(200, true)
}

func (actions *Actions) GetApiKeysPermissions(c *gin.Context) {
	data := actions.service.GetApiKeysPermissions()

	c.JSON(200, data)
}
