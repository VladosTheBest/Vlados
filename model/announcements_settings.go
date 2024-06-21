package model

import (
	"time"

	"github.com/lib/pq"
)

type AnnouncementTopic string

func (at *AnnouncementTopic) ToString() string {
	return string(*at)
}

type AnnouncementsSettingsSchema struct {
	ID        uint64         `json:"ID,omitempty" gorm:"PRIMARY_KEY"`
	Topics    pq.StringArray `json:"topics,omitempty" gorm:"topics;type:varchar[]"`
	CreatedAt time.Time      `gorm:"created_at"`
	UpdatedAt time.Time      `gorm:"updated_at"`
}
