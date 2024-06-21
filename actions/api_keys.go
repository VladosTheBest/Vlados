package actions

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"strconv"
)

// GetAPIKeys godoc
// swagger:route GET /apikeys api_keys get_api_keys
// Get API keys
//
// Get a list of generated API keys for the current user.
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: UserAPIKeys
//	      404: RequestErrorResp
func (actions *Actions) GetAPIKeys(c *gin.Context) {
	userID, _ := getUserID(c)
	data, err := actions.service.GetAPIKeys(userID)
	if err != nil {
		abortWithError(c, 404, "Unable to get api keys")
		return
	}
	c.JSON(200, data)
}

// RemoveAPIKey godoc
// swagger:route DELETE /apikeys/{api_key_id} api_keys delete_api_key
// Remove API Key
//
// Remove an API key of the current user with the given id
//
//	    Consumes:
//	    - application/x-www-form-urlencoded
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: StringResp
//	      404: RequestErrorResp
func (actions *Actions) RemoveAPIKey(c *gin.Context) {
	userID, _ := getUserID(c)
	apiKeyID, _ := c.GetQuery("apikey") // @todo CH: refactor this to use the API prefix from path instead
	name, err := actions.service.RemoveAPIKey(userID, apiKeyID)
	if err != nil {
		abortWithError(c, 404, "Unable to remove API key")
		return
	}

	_, err = actions.service.SendNotification(userID, model.NotificationType_System,
		model.NotificationTitle_NewAPIKey.String(), fmt.Sprintf(model.NotificationMessage_NewAPIKeyOFF.String(), name),
		model.Notification_NewAPIKey, apiKeyID)

	if err != nil {
		abortWithError(c, ServerError, err.Error())
		return
	}

	c.JSON(200, true)
}

// GenerateAPIKey godoc
// swagger:route POST /apikeys api_keys add_api_key
// Generate a new API key
//
// Generate a new API key for the current user
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Responses:
//	      200: StringResp
//	      404: RequestErrorResp
func (actions *Actions) GenerateAPIKey(c *gin.Context) {
	iUser, _ := c.Get("auth_user")
	user := iUser.(*model.User)

	name, _ := c.GetPostForm("api_key_name")
	roleAlias, _ := c.GetPostForm("api_key_role")

	// generate API key
	key, apiKey, err := actions.service.GenerateAPIKey(user.ID, name, roleAlias)
	if err != nil {
		abortWithError(c, NotFound, "Unable to generate API key")
		return
	}

	// create action confirmation and send approval email
	err = actions.service.CreateActionAndSendApproval(model.ActionType_APIKey, user, apiKey.ID, map[string]string{"name": name})
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

	// return the secret key
	c.JSON(200, key)
}

// GetAPIRoles godoc
// swagger:route GET /apikeys/roles api_keys get_api_roles
// Get API roles
//
// # Get all roles available for a new API Key
//
// Deprecated: GET /roles?scope=api
//
//	    Consumes:
//	    - multipart/form-data
//
//			 Security:
//	      UserToken:
//
//	    Deprecated: true
//
//	    Responses:
//	      200: Roles
//	      404: RequestErrorResp
func (actions *Actions) GetAPIRoles(c *gin.Context) {
	data, err := actions.service.GetRolesByScope("api")
	if err != nil {
		abortWithError(c, 404, "Unable to get api roles")
		return
	}
	c.JSON(200, data)
}
