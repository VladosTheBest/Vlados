package actions

import (
	"context"
	"github.com/gin-gonic/gin"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"net/http"
	"sync"
	"time"
)

var pushNotificationChan = make(chan *model.Notification, 1000)

func (actions *Actions) CreateNotifications(c *gin.Context) {
	notification := model.Notification{}
	if err := c.ShouldBind(&notification); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	var data *model.NotificationWithTotalUnread
	var err error

	if notification.ID == 0 {
		data, err = actions.service.SendNotification(notification.UserID, notification.Type, notification.Title.String(),
			notification.Message.String(), notification.RelatedObjectType, notification.RelatedObjectID)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		data, err = actions.service.UpdateNotification(notification)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, data)
}

func (actions *Actions) GetNotifications(c *gin.Context) {
	userID, _ := getUserID(c)
	page := getQueryAsInt(c, "page", 1)
	limit := getQueryAsInt(c, "limit", 10)

	notification, err := actions.service.GetNotifications(userID, page, limit)

	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, notification)
}

func (actions *Actions) DeleteNotification(c *gin.Context) {
	userID, _ := getUserID(c)
	notificationID := c.Param("notificationID")

	err := actions.service.DeleteNotification(userID, notificationID)

	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Notification deleted successfully"})
}

func (actions *Actions) DeleteNotificationsInRange(c *gin.Context) {
	userID, _ := getUserID(c)
	from := c.Query("from")
	to := c.Query("to")

	// Parse and validate the 'to' date
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "Invalid 'to' date format. Use YYYY-MM-DD.")
		return
	}

	// Check if 'from' date is provided
	var fromDate time.Time
	if from != "" && from != "0" {
		fromDate, err = time.Parse("2006-01-02", from)
		if err != nil {
			abortWithError(c, http.StatusBadRequest, "Invalid 'from' date format. Use YYYY-MM-DD.")
			return
		}
	} else {
		// If 'from' is not provided, set it to the zero value of time.Time
		fromDate = time.Time{}
	}

	// Call the service method to delete notifications
	err = actions.service.DeleteNotificationsInRange(userID, fromDate, toDate)
	if err != nil {
		abortWithError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notifications deleted successfully"})
}

func (actions *Actions) ChangeNotificationsStatus(c *gin.Context) {
	notificationsID, _ := c.GetPostFormArray("id")
	status, _ := c.GetPostForm("status")

	if !model.NotificationStatus(status).IsValid() {
		abortWithError(c, http.StatusBadRequest, "Status parameter is wrong")
		return
	}

	err := actions.service.ChangeNotificationsStatus(notificationsID, status)

	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}

	c.JSON(http.StatusOK, "OK")
}

func (actions *Actions) GetUnreadNotifications(c *gin.Context) {
	userID, _ := getUserID(c)

	notification, err := actions.service.GetTotalUnreadNotifications(userID)

	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, notification)
}

func (actions *Actions) PushToken(c *gin.Context) {
	userID, _ := getUserID(c)

	pushToken := model.PushToken{}
	if err := c.ShouldBind(&pushToken); err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	pushToken.UserID = userID

	if !pushToken.OperationSystem.IsValid() {
		abortWithError(c, http.StatusBadRequest, "operation system is not valid")
		return
	}

	err := actions.service.PushToken(pushToken)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, pushToken)
}

func (actions *Actions) DeletePushToken(c *gin.Context) {
	pushToken := c.Query("push_token")

	err := actions.service.DeletePushToken(pushToken)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, pushToken)
}

// GetPushNotificationChan get a channel to send push notification.
func GetPushNotificationChan() chan<- *model.Notification {
	return pushNotificationChan
}

func (actions *Actions) PushNotifications(ctx context.Context, wait *sync.WaitGroup) {
	actions.service.PushNotificationWorker(pushNotificationChan, ctx, wait)
}
