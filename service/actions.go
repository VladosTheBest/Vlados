package service

import (
	"errors"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"time"
)

// ActionNotImplementedErr godoc
var ActionNotImplementedErr = errors.New("Action type not implemented")

// GetActionByUUID godoc
func (service *Service) GetActionByUUID(uuid string) (*model.Action, error) {
	return service.repo.GetActionByUUID(uuid)
}

// CreateAction godoc
func (service *Service) CreateAction(userID uint64, actionType model.ActionType, data string) (*model.Action, error) {
	action := model.NewAction(userID, actionType, data)
	err := service.repo.Create(action)
	if err != nil {
		return nil, err
	}
	return action, nil
}

// UpdateActionStatus godoc
func (service *Service) UpdateActionStatus(action *model.Action) error {
	return service.repo.UpdateActionStatus(action)
}

// NotifyAction godoc
func (service *Service) NotifyAction(action *model.Action, data map[string]string, recepients ...string) error {
	for i, recepient := range recepients {
		if i >= action.MinApprovals {
			break
		}
		key := ""
		if i == 0 {
			key = action.UserKey
		} else {
			key = action.AdminKey
		}
		err := service.SendActionConfirmation(action, recepient, action.UUID, key, data)
		if err != nil {
			return err
		}
	}
	return nil
}

// SendActionConfirmation godoc
func (service *Service) SendActionConfirmation(action *model.Action, email, uuid, key string, params map[string]string) error {
	data := map[string]string{
		"domain":       service.apiConfig.Domain,
		"action_id":    uuid,
		"approval_key": key,
	}
	userDetails := model.UserDetails{}
	db := service.GetRepo().ConnReader.First(&userDetails, "user_id = ?", action.UserID)
	if db.Error != nil {
		return nil
	}
	// merge the params in the final list sent through email
	for k, v := range params {
		data[k] = v
	}
	return service.sendgrid.SendEmail(
		email,
		userDetails.Language.String(),
		getEmailTemplateForActionType(action.Type, action.Action),
		data,
	)
}

func getEmailTemplateForActionType(actionType model.ActionType, action string) string {
	switch actionType {
	case model.ActionType_Withdraw:
		return "confirm_withdraw"
	case model.ActionType_APIKey:
		return "confirm_api_key"
	case model.ActionType_APIKeyV2:
		return "confirm_api_key"
	case model.ActionType_ConfirmIP:
		return "confirm_ip"
	default:
		return "confirm_action"
	}
}

// CreateActionAndSendApproval godoc
func (service *Service) CreateActionAndSendApproval(actionType model.ActionType, user *model.User, propertyID interface{}, params map[string]string) error {
	var data string
	var actionData model.ActionData
	// generate action data based on action type
	switch actionType {
	case model.ActionType_Withdraw:
		actionData = &model.WithdrawActionData{WithdrawRequestID: propertyID.(string)}
	case model.ActionType_APIKey:
		actionData = &model.APIKeyData{APIKeyID: propertyID.(uint64)}
	case model.ActionType_APIKeyV2:
		actionData = &model.APIKeyData{APIKeyID: propertyID.(uint64)}
	case model.ActionType_ConfirmIP:
		actionData = &model.ConfirmIPData{IP: propertyID.(string)}
	default:
		return ActionNotImplementedErr
	}
	// convert the data into string to store it in the db
	data, err := actionData.ToString()
	if err != nil {
		return err
	}

	apCode, _ := service.GetAntiPhishingCode(user.ID)

	action, err := service.CreateAction(user.ID, actionType, data)
	if err != nil {
		return err
	}

	params["apcode"] = apCode
	params["date"] = time.Now().Format(time.RFC822)

	// send notification to user
	return service.NotifyAction(action, params, user.Email)
}
