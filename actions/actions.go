package actions

import (
	"fmt"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/service/auth_service"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetActionByUUID godoc
func (actions *Actions) GetActionByUUID() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("action_id")
		action, err := actions.service.GetActionByUUID(uuid)
		if err != nil {
			abortWithError(c, NotFound, "Unable to find specified action")
			return
		}

		c.Set("data_action", action)
		c.Next()
	}
}

// ApproveAction godoc
// swagger:route PUT /actions/{action_id}/approve/{key} actions approve_action
// Approve action
//
// Approve a system generated action
//
//	Consumes:
//	- multipart/form-data
//
//	Produces:
//	- application/json
//
//	Schemes: http, https
//
//	Security: []
//
//	Responses:
//	  200: StringResp
//	  404: RequestErrorResp
//	  500: RequestErrorResp
func (actions *Actions) ApproveAction(c *gin.Context) {
	log := getlog(c)
	iAction, _ := c.Get("data_action")
	action := iAction.(*model.Action)
	key := c.Param("key")

	if !action.Approve(key) {
		log.Warn().
			Str("section", "actions").
			Str("action", "approve").
			Str("action_id", action.UUID).
			Msg("Unable to approve action with the given key")
		abortWithError(c, BadRequest, "Invalid key provided")
		return
	}

	err := actions.service.UpdateActionStatus(action)
	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "approve").
			Str("action_id", action.UUID).
			Msg("Unable to save updated action")
		abortWithError(c, ServerError, "Unable to approve action")
		return
	}

	if action.Status != model.ActionStatus_Approved {
		c.JSON(OK, map[string]string{"message": "Action approved. Waiting on secondary approval."})
		return
	}

	switch action.Type {
	case model.ActionType_ConfirmIP:
		err := actions.ActionApproveIP(action)
		if err != nil {
			log.Error().Err(err).
				Str("section", "actions").
				Str("action", "approve").
				Str("action_id", action.UUID).
				Msg("Unable to approve new IP address")
			abortWithError(c, ServerError, "Unable to approve new IP address")
			return
		}
		c.JSON(OK, map[string]string{"message": "New IP address approved successfully", "type": "confirm_ip"})
		return
	case model.ActionType_APIKey:
		err := actions.ActionApproveAPIKey(action)
		if err != nil {
			log.Error().Err(err).
				Str("section", "actions").
				Str("action", "approve").
				Str("action_id", action.UUID).
				Msg("Unable to approve API Key")
			abortWithError(c, ServerError, "Unable to approve API key")
			return
		}
		c.JSON(OK, map[string]string{"message": "Your new API key has been approved successfully", "type": "confirm_api_key"})
		return
	case model.ActionType_APIKeyV2:
		err := actions.ActionApproveAPIKeyV2(action)
		if err != nil {
			log.Error().Err(err).
				Str("section", "actions").
				Str("action", "approve").
				Str("action_id", action.UUID).
				Msg("Unable to approve API Key")
			abortWithError(c, ServerError, "Unable to approve API key")
			return
		}
		c.JSON(OK, map[string]string{"message": "Your new API key has been approved successfully", "type": "confirm_api_key"})
		return
	case model.ActionType_Withdraw:
		{
			data := model.WithdrawActionData{}
			err := data.FromString(action.Data)
			if err != nil {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Str("action_id", action.UUID).
					Msg("Unable to parse action data")
				abortWithError(c, BadRequest, "Unable to approve action")
				return
			}
			request, err := actions.service.GetWithdrawRequest(data.WithdrawRequestID)
			if err != nil {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Uint64("user_id", request.UserID).
					Str("action_id", action.UUID).
					Str("withdraw_request_id", request.ID).
					Msg("Withdraw request not found")
				abortWithError(c, NotFound, "Withdraw request not found")
				return
			}
			// withdraw request should have "pending" status
			if request.Status != model.WithdrawStatus_Pending {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Uint64("user_id", request.UserID).
					Str("action_id", action.UUID).
					Str("withdraw_request_id", request.ID).
					Msg("Withdraw has no pending status")
				abortWithError(c, NotFound, fmt.Sprintf("Withdraw status already changed to '%s'", string(request.Status)))
				return
			}
			// get request coin
			coin, err := actions.service.GetCoin(request.CoinSymbol)
			if err != nil {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Uint64("user_id", request.UserID).
					Str("action_id", action.UUID).
					Str("withdraw_request_id", request.ID).
					Msg("Withdraw coin not found")
				abortWithError(c, NotFound, "Invalid coin associated with withdraw request")
				return
			}
			// the request should't be expired
			if request.CreatedAt.Sub(time.Now().Add(-60*time.Minute)) <= 0 {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Uint64("user_id", request.UserID).
					Str("action_id", action.UUID).
					Str("withdraw_request_id", request.ID).
					Msg("The withdrawal request has expired")
				abortWithError(c, NotFound, "The withdrawal request has expired")
				return
			}
			_, err = actions.service.AcceptWithdrawRequest(request)
			if err != nil {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Uint64("user_id", request.UserID).
					Str("action_id", action.UUID).
					Str("withdraw_request_id", request.ID).
					Msg("Error on accept withdraw request")
				abortWithError(c, NotFound, "Error on accept withdraw request")
				return
			}

			// send the request over to the wallet
			err = actions.service.WalletCreateWithdrawRequest(
				request.UserID,
				request.ID,
				coin.ChainSymbol,
				coin.Symbol,
				request.Amount.V,
				request.To,
				coin.TokenPrecision,
				request.ExternalSystem,
			)
			if err != nil {
				log.Error().Err(err).
					Str("section", "actions").
					Str("action", "wallet:withdraw_request").
					Uint64("user_id", request.UserID).
					Str("action_id", action.UUID).
					Str("withdraw_request_id", request.ID).
					Msg("Error trasmitting withdraw request")
				abortWithError(c, NotFound, "Unable to process withdraw request")
				return
			}
			c.JSON(OK, map[string]string{"message": "Withdraw request approved and sent for processing", "type": "withdraw_request"})
			return
		}
	}
	abortWithError(c, BadRequest, "Unable to approve action")
}

// ActionApproveAPIKey godoc
func (actions *Actions) ActionApproveAPIKey(action *model.Action) error {
	data := model.APIKeyData{}
	err := data.FromString(action.Data)
	if err != nil {
		return err
	}
	apiKey, err := actions.service.GetAPIKeyByID(data.APIKeyID)
	if err != nil {
		return err
	}
	err = actions.service.ActivateAPIKey(apiKey)
	return err
}

// ActionApproveAPIKey godoc
func (actions *Actions) ActionApproveAPIKeyV2(action *model.Action) error {
	data := model.APIKeyData{}
	err := data.FromString(action.Data)
	if err != nil {
		return err
	}
	apiKey, err := actions.service.GetAPIKeyV2ByID(data.APIKeyID)
	if err != nil {
		return err
	}
	err = actions.service.ActivateAPIKeyV2(apiKey)
	return err
}

// ActionApproveIP godoc
func (actions *Actions) ActionApproveIP(action *model.Action) error {
	data := model.ConfirmIPData{}
	err := data.FromString(action.Data)
	if err != nil {
		return err
	}
	return actions.service.ApproveIPForUser(action.UserID, data.IP)
}

func (actions *Actions) CheckApprovedIP(c *gin.Context) {
	log := getlog(c)

	if preAuthStage, exist := c.Get("preauth_stage"); !exist || preAuthStage.(string) != auth_service.PreAuthTokenStageUnApprovedIP.String() {
		abortWithError(c, ServerError, "Unable to process request")
		return
	}
	iUser, _ := c.Get("preauth_user")
	user := iUser.(*model.User)

	if user.Status != model.UserStatusActive {
		abortWithError(c, Unauthorized, "Your account is not active. Please contact support.")
		return
	}
	ip := getIPFromRequest(c.GetHeader("x-forwarded-for"))
	isApproved, err := actions.service.HasApprovedIP(user.ID, ip)

	if err != nil {
		log.Error().Err(err).
			Str("section", "actions").
			Str("action", "CheckApprovedIP").
			Uint64("ip", user.ID).
			Msg(err.Error())
		abortWithError(c, ServerError, "Unable to process request")
		return
	}
	message := "New IP address approved successfully"
	if !isApproved {
		message = "New IP address not approved yet"
	}
	c.JSON(OK, map[string]interface{}{
		"ip_approved": isApproved,
		"message":     message,
		"type":        "confirm_ip",
	})
}

func (actions *Actions) GetCountryCodeByIP(c *gin.Context) {
	ip := c.ClientIP()
	countryCode, err := actions.service.GetUserCountryByIP(ip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"country_code": countryCode})
}
