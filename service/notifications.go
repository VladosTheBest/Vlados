package service

import (
	"context"
	"errors"
	"github.com/Unleash/unleash-client-go/v3"
	"github.com/rs/zerolog/log"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gopkg.in/maddevsio/fcm.v1"
	"strconv"
	"sync"
	"time"
)

func (service *Service) GetNotifications(userID uint64, page, limit int) (*model.NotificationWithMeta, error) {

	notifications := make([]model.Notification, 0)
	var rowCount int64 = 0
	db := service.repo.ConnReader.Table("notifications").
		Where("user_id = ? AND status != ?", userID, model.NotificationStatus_Deleted)
	if !unleash.IsEnabled("api.notifications_include_announcements") {
		db = db.Where("related_object_type <> ?", model.Notification_Announcement)
	}
	if db.Error != nil {
		return nil, db.Error
	}

	if limit == 0 || limit > 100 {
		limit = 100
	}
	dbc := db.Select("count(*) as total").Row()
	_ = dbc.Scan(&rowCount)
	db = db.Select("*")
	db = db.Limit(limit).Offset((page - 1) * limit).Order("created_at DESC").Group("id").Group("created_at").
		Find(&notifications)

	notificationList := model.NotificationWithMeta{
		Notification: notifications,
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{}),
		},
	}

	return &notificationList, db.Error
}

func (service *Service) DeleteNotification(userID uint64, notificationID string) error {
	db := service.repo.Conn.Table("notifications")

	// Convert notificationID to uint64
	notifID, err := strconv.ParseUint(notificationID, 10, 64)
	if err != nil {
		return err
	}

	// Delete the notification
	res := db.Where("user_id = ? AND id = ?", userID, notifID).Delete(nil)

	// Check for errors
	if res.Error != nil {
		log.Info().Str("service", "notifications").Str("DeleteNotification", "delete").Msg("error when deleting notification from db")
		return res.Error
	}

	return nil
}

func (service *Service) DeleteNotificationsInRange(userID uint64, fromDate, toDate time.Time) error {
	db := service.repo.Conn.Table("notifications")

	// Delete the notifications within the date range
	res := db.Where("user_id = ? AND created_at BETWEEN ? AND ?", userID, fromDate, toDate.AddDate(0, 0, 1)).Delete(nil)

	// Check for errors
	if res.Error != nil {
		return res.Error
	}

	return nil
}

func (service *Service) ChangeNotificationsStatus(notificationID []string, status string) error {

	q := service.repo.Conn.Table("notifications").
		Where("id IN (?)", notificationID)

	return q.Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

func (service *Service) SendNotification(userID uint64, notificationType model.NotificationType, title,
	message string, relatedObjectType model.RelatedObjectType,
	relatedObjectID string) (*model.NotificationWithTotalUnread, error) {

	notification := model.Notification{
		UserID:            userID,
		Status:            model.NotificationStatus_UnRead,
		RelatedObjectType: relatedObjectType,
		RelatedObjectID:   relatedObjectID,
		Type:              notificationType,
		Title:             model.NotificationTitle(title),
		Message:           model.NotificationMessage(message),
	}

	if err := service.repo.Conn.Table("notifications").Create(&notification).Error; err != nil {
		return nil, err
	}

	totalUnreadNotifications, err := service.GetTotalUnreadNotifications(userID)

	if err != nil {
		return nil, err
	}

	data := model.NotificationWithTotalUnread{
		Notification:             notification,
		TotalUnreadNotifications: totalUnreadNotifications,
	}

	service.notificationChan <- &data
	service.pushNotificationChan <- &data.Notification

	return &data, nil
}

func (service *Service) UpdateNotification(notification model.Notification) (*model.NotificationWithTotalUnread, error) {
	notification.UpdatedAt = time.Now()

	db := service.repo.Conn.Table("notifications").
		Where("id = ?", notification.ID).
		Save(&notification)
	if db.Error != nil {
		db.Rollback()
		return nil, db.Error
	}

	totalUnreadNotifications, err := service.GetTotalUnreadNotifications(notification.UserID)
	if err != nil {
		return nil, err
	}

	data := model.NotificationWithTotalUnread{
		Notification:             notification,
		TotalUnreadNotifications: totalUnreadNotifications,
	}

	service.notificationChan <- &data
	service.pushNotificationChan <- &data.Notification

	return &data, nil
}

func (service *Service) GetTotalUnreadNotifications(userID uint64) (int, error) {

	rowCount := 0
	db := service.repo.ConnReader.Table("notifications").
		Where("user_id = ?", userID).
		Where("status = ?", model.NotificationStatus_UnRead)

	if !unleash.IsEnabled("api.notifications_include_announcements") {
		db = db.Where("related_object_type <> ?", model.Notification_Announcement)
	}

	err := db.Select("count(*) as total").Row().Scan(&rowCount)
	if err != nil {
		return rowCount, err
	}

	return rowCount, nil
}

func (service *Service) PushToken(pushToken model.PushToken) error {
	var token model.PushToken
	err := service.repo.ConnReader.Where("unique_id = ?", pushToken.UniqueID).First(&token).Error
	if err == nil {
		err = service.DeletePushToken(token.PushToken)
		if err != nil {
			return err
		}
	}

	db := service.repo.Conn.Create(&pushToken)

	return db.Error
}

func (service *Service) DeletePushToken(pushToken string) error {
	db := service.repo.Conn.Where("push_token = ?", pushToken).Delete(&model.PushToken{})

	return db.Error
}

func (service *Service) PushNotification(userID uint64) ([]string, error) {
	if !unleash.IsEnabled("api.notifications.push") {
		return nil, errors.New("push tokens temporarily disabled")
	}

	var pushTokens []*model.PushToken
	if err := service.repo.ConnReader.Find(&pushTokens, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}

	var tokens []string

	for _, pushToken := range pushTokens {
		tokens = append(tokens, pushToken.PushToken)
	}

	return tokens, nil
}

func (service *Service) PushNotificationWorker(notifications chan *model.Notification, ctx context.Context, wait *sync.WaitGroup) {
	log.Info().Str("worker", "push_notifications").Str("action", "start").Msg("Service push notifications - started")

	client := fcm.NewFCM(service.cfg.FirebaseClient.ApiKey)
	var pushNotificationSettings model.PushNotificationSettings
	for {
		select {
		case notification := <-notifications:
			db := service.repo.ConnReader.First(&pushNotificationSettings, "user_id = ?", notification.ID)
			if db.Error != nil {
				log.Error().Err(db.Error).
					Str("section", "service:notification").
					Str("action", "PushNotificationWorker").
					Msg("Unable to get push notification settings")
				continue
			}
			if !pushNotificationSettings.IsEnabled(notification.RelatedObjectType) {
				continue
			}
			tokens, err := service.PushNotification(notification.UserID)
			if err != nil {
				log.Error().Err(err).
					Str("section", "service:notification").
					Str("action", "PushNotificationWorker").
					Msg("Unable to get tokens")
				continue
			}

			_, err = client.Send(fcm.Message{
				RegistrationIDs:  tokens,
				ContentAvailable: true,
				Priority:         fcm.PriorityHigh,
				Notification: fcm.Notification{
					Title: notification.Title.String(),
					Body:  notification.Message.StringPlain(),
				},
			})
			if err != nil {
				log.Error().Err(err).
					Str("section", "service:notification").
					Str("action", "PushNotificationWorker").
					Msg("Unable to send push notification")
			}
		case <-ctx.Done():
			log.Info().Str("worker", "push_notifications").Str("action", "stop").Msg("4 => Service push notifications - stopped")
			wait.Done()
			return
		}
	}
}
