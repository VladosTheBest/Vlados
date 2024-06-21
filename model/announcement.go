package model

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type AnnouncementStatus string

const (
	AnnouncementStatus_Published AnnouncementStatus = "published"
	AnnouncementStatus_Draft     AnnouncementStatus = "draft"
	AnnouncementStatus_Deleted   AnnouncementStatus = "deleted"
)

func (a AnnouncementStatus) IsValid() bool {
	switch a {
	case AnnouncementStatus_Published,
		AnnouncementStatus_Draft,
		AnnouncementStatus_Deleted:
		return true
	default:
		return false
	}
}

type Announcement struct {
	ID          uint64             `form:"id"         json:"id"`
	Status      AnnouncementStatus `form:"status"     json:"status"  binding:"required"`
	Title       string             `form:"title"      json:"title"   binding:"required"`
	Message     string             `form:"message"    json:"message" binding:"required"`
	Image       string             `form:"image"      json:"image"`
	Topic       AnnouncementTopic  `form:"topic"      json:"topic"   binding:"required"`
	CreatedAt   time.Time          `form:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `form:"updated_at" json:"updated_at"`
	PublishedAt time.Time          `form:"published_at" json:"published_at"`
}

type AnnouncementWithMeta struct {
	Announcement []Announcement `json:"announcement"`
	Meta         PagingMeta     `json:"meta"`
}

func (announcement *Announcement) ShortMessage() string {
	reg := regexp.MustCompile(`<(.|\n)*?>|\n`)
	message := reg.ReplaceAllString(announcement.Message, "")
	re := regexp.MustCompile(" +")
	shortMessage := re.ReplaceAllString(strings.TrimSpace(message), " ")

	if len(shortMessage) > 128 {
		shortMessage = fmt.Sprintf("%s ...", shortMessage[:128])
	}
	return shortMessage
}

// MarshalJSON JSON encoding of a liability entry
func (announcement Announcement) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":            announcement.ID,
		"status":        announcement.Status,
		"title":         announcement.Title,
		"message":       announcement.Message,
		"topic":         announcement.Topic,
		"message_short": announcement.ShortMessage(),
		"image":         announcement.Image,
		"created_at":    announcement.CreatedAt,
		"updated_at":    announcement.UpdatedAt,
	})
}
