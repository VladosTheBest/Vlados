package service

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"gorm.io/gorm"
)

func (service *Service) sendNotificationForAnnouncement(tx *gorm.DB, announcement *model.Announcement) error {
	if announcement.Status == model.AnnouncementStatus_Published && announcement.PublishedAt.IsZero() {
		announcement.PublishedAt = time.Now()

		ids := []uint64{}
		if err := tx.Table("users").Pluck("id", &ids).Error; err != nil {
			return err
		}
		announcementID := strconv.FormatUint(announcement.ID, 10)
		for _, userID := range ids {
			notification := model.Notification{
				UserID:            userID,
				Status:            model.NotificationStatus_UnRead,
				RelatedObjectType: model.Notification_Announcement,
				RelatedObjectID:   announcementID,
				Type:              model.NotificationType_Info,
				Title:             model.NotificationTitle(announcement.Title),
				Message:           model.NotificationMessage(announcement.ShortMessage()),
			}
			if err := tx.Create(&notification).Error; err != nil {
				tx.Rollback()
				return tx.Error
			}
		}

		if err := tx.Table("announcements").
			Where("id = ?", announcement.ID).
			Save(announcement).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}

func (service *Service) SendAnnouncement(announcement model.Announcement, image *multipart.FileHeader) (*model.Announcement, error) {
	if image != nil {
		fileBytes := bytes.NewBuffer(nil)
		imageFile, err := image.Open()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = imageFile.Close()
		}()

		if _, err := io.Copy(fileBytes, imageFile); err != nil {
			return nil, err
		}
		mimeType := http.DetectContentType(fileBytes.Bytes())

		logoBase64 := ""
		switch mimeType {
		case "image/jpeg":
			logoBase64 += "data:image/jpeg;base64,"
		case "image/png":
			logoBase64 += "data:image/png;base64,"
		default:
			return nil, errors.New("file not supported")
		}
		logoBase64 += base64.StdEncoding.EncodeToString(fileBytes.Bytes())
		announcement.Image = logoBase64
	}

	tx := service.repo.Conn.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	if err := tx.Create(&announcement).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := service.sendNotificationForAnnouncement(tx, &announcement); err != nil {
		return nil, err
	}

	return &announcement, tx.Commit().Error
}

func (service *Service) UpdateAnnouncement(announcement, announcementUpdates *model.Announcement, image *multipart.FileHeader) error {
	if image != nil {
		fileBytes := bytes.NewBuffer(nil)
		imageFile, err := image.Open()
		if err != nil {
			return err
		}
		defer func() {
			_ = imageFile.Close()
		}()

		if _, err = io.Copy(fileBytes, imageFile); err != nil {
			return err
		}
		mimeType := http.DetectContentType(fileBytes.Bytes())

		logoBase64 := ""
		switch mimeType {
		case "image/jpeg":
			logoBase64 += "data:image/jpeg;base64,"
		case "image/png":
			logoBase64 += "data:image/png;base64,"
		default:
			return errors.New("file not supported")
		}
		logoBase64 += base64.StdEncoding.EncodeToString(fileBytes.Bytes())
		announcement.Image = logoBase64
	}
	announcementUpdates.UpdatedAt = time.Now()
	tx := service.repo.Conn.Begin()

	updates := map[string]interface{}{}

	if announcement.Title != announcementUpdates.Title {
		announcement.Title = announcementUpdates.Title
		updates["title"] = announcementUpdates.Title
	}

	if announcement.Topic != announcementUpdates.Topic {
		announcement.Topic = announcementUpdates.Topic
		updates["topic"] = announcementUpdates.Topic
	}

	if announcement.Message != announcementUpdates.Message {
		announcement.Message = announcementUpdates.Message
		updates["message"] = announcementUpdates.Message
	}

	if announcement.Status != announcementUpdates.Status {
		announcement.Status = announcementUpdates.Status
	}

	if err := tx.Table("announcements").
		Where("id = ?", announcementUpdates.ID).
		Save(&announcement).Error; err != nil {
		tx.Rollback()
		return err
	}

	if announcement.Status == model.AnnouncementStatus_Published {
		if err := tx.Table("notifications").
			Where("related_object_id = ?", announcement.ID).
			Where("related_object_type = ?", model.Notification_Announcement).
			Updates(updates).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := service.sendNotificationForAnnouncement(tx, announcementUpdates); err != nil {
		return err
	}

	return tx.Commit().Error
}

func (service *Service) GetAnnouncements(page, limit int) (*model.AnnouncementWithMeta, error) {
	var announcements []model.Announcement
	var rowCount int64 = 0

	db := service.repo.ConnReader.Table("announcements").
		Where("status = ?", model.AnnouncementStatus_Published)

	dbcRow := db.Select("count(*) as total").Row()
	err := dbcRow.Scan(&rowCount)

	if err != nil {
		return nil, err
	}

	if limit == 0 || limit > 100 {
		limit = 100
	}

	db = db.Select("*")
	db = db.Limit(limit).Offset((page - 1) * limit).Order("created_at DESC").Group("id").Group("created_at").
		Find(&announcements)

	if db.Error != nil {
		return nil, db.Error
	}

	announcementList := model.AnnouncementWithMeta{
		Announcement: announcements,
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{}),
		},
	}

	return &announcementList, db.Error
}

func (service *Service) GetAdminAnnouncements(page, limit int, status string) (*model.AnnouncementWithMeta, error) {
	var announcements []model.Announcement
	var rowCount int64 = 0

	db := service.repo.ConnReader.Table("announcements")
	dbc := service.repo.ConnReader.Table("announcements")
	db = db.Where("status != ?", model.AnnouncementStatus_Deleted)
	dbc = dbc.Where("status != ?", model.AnnouncementStatus_Deleted)

	if len(status) > 0 && model.AnnouncementStatus(status).IsValid() {
		db = db.Where("status = ?", status)
		dbc = dbc.Where("status = ?", status)
	}

	dbcRow := dbc.Select("count(*) as total").Row()
	_ = dbcRow.Scan(&rowCount)

	if limit == 0 || limit > 100 {
		limit = 100
	}

	db = db.Limit(limit).Offset((page - 1) * limit).Order("created_at DESC").Group("id").Group("created_at").
		Find(&announcements)

	if db.Error != nil {
		return nil, db.Error
	}

	announcementList := model.AnnouncementWithMeta{
		Announcement: announcements,
		Meta: model.PagingMeta{
			Page:   page,
			Count:  rowCount,
			Limit:  limit,
			Filter: make(map[string]interface{}),
		},
	}

	return &announcementList, db.Error
}

func (service *Service) GetAdminAnnouncementByID(id uint64) (*model.Announcement, error) {
	var announcements model.Announcement

	db := service.repo.ConnReader.Table("announcements").First(&announcements, "id = ?", id)
	if db.Error != nil {
		return nil, db.Error
	}

	return &announcements, db.Error
}

func (service *Service) GetAnnouncementByTopicAndID(topic string, id uint64) (*model.Announcement, *model.Announcement, *model.Announcement, error) {
	var currentAnnouncement, previousAnnouncement, nextAnnouncement model.Announcement

	// Найти текущий анонс по ID и топику
	db := service.repo.ConnReader.Table("announcements").
		Where("topic = ?", topic).
		First(&currentAnnouncement, "id = ?", id)
	if db.Error != nil {
		return nil, nil, nil, db.Error
	}

	// Найти предыдущий анонс
	db = service.repo.ConnReader.Table("announcements").
		Where("topic = ? AND id < ?", topic, id).
		Order("id DESC").
		First(&previousAnnouncement)
	if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
		return nil, nil, nil, db.Error
	}

	// Найти следующий анонс
	db = service.repo.ConnReader.Table("announcements").
		Where("topic = ? AND id > ?", topic, id).
		Order("id ASC").
		First(&nextAnnouncement)
	if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
		return nil, nil, nil, db.Error
	}

	return &currentAnnouncement, &previousAnnouncement, &nextAnnouncement, nil
}

func (service *Service) DeleteAdminAnnouncementByID(id uint64) error {
	var announcements model.Announcement

	db := service.repo.ConnReader.Table("announcements").Delete(&announcements, "id = ?", id)
	if db.Error != nil {
		return db.Error
	}

	return db.Error
}

func (service *Service) ChangeAnnouncementStatus(announcement *model.Announcement, status model.AnnouncementStatus) error {

	tx := service.repo.Conn.Begin()

	updates := map[string]interface{}{"status": status, "updated_at": time.Now()}

	if status == model.AnnouncementStatus_Published {
		announcement.Status = model.AnnouncementStatus_Published
		updates["published_at"] = time.Now()
	}

	if err := tx.Table("announcements").
		Where("id = ?", announcement.ID).
		Updates(updates).Error; err != nil {
		return err
	}

	if err := service.sendNotificationForAnnouncement(tx, announcement); err != nil {
		return err
	}

	return tx.Commit().Error
}

func (service *Service) ValidateAnnouncement(announcement *model.Announcement) error {
	announcementsSettings, err := service.repo.GetAnnouncementSettings()
	if err != nil {
		return err
	}

	containsTopic := false
	for _, topic := range announcementsSettings.Topics {
		if announcement.Topic.ToString() == topic {
			containsTopic = true
			break
		}
	}

	if !containsTopic {
		return errors.New("invalid topic")
	}

	return nil
}

func (service *Service) GetAnnouncementsByTopic(topic string, offset, limit int) ([]model.Announcement, int, error) {
	var announcements []model.Announcement
	var totalCount int64

	db := service.repo.ConnReader.Table("announcements").
		Where("status = ?", model.AnnouncementStatus_Published).
		Where("topic = ?", topic)

	// Count total announcements before applying limit and offset
	db.Count(&totalCount)

	db = db.Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&announcements)

	if db.Error != nil {
		return nil, 0, db.Error
	}

	return announcements, int(totalCount), nil
}
