package model

import (
	"time"

	gouuid "github.com/nu7hatch/gouuid"
)

// UserActivity holds information about a completed user events
type UserActivity struct {
	ID        string    `sql:"type:uuid" gorm:"PRIMARY_KEY" json:"id"`
	Event     string    `sql:"type:varchar(50)" json:"event"`
	Notes     string    `sql:"type:text" json:"notes"`
	UserID    uint64    `sql:"type:text" json:"user_id"`
	IP        string    `sql:"type:text" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserActivityLogsList struct {
	Logs []UserActivity `json:"logs"`
	Meta PagingMeta     `json:"meta"`
}

// NewUserActivity creates a new user activity to save in the database
func NewUserActivity(event, notes, ip string, userID uint64) *UserActivity {
	u, _ := gouuid.NewV4()
	return &UserActivity{
		ID:     u.String(),
		Event:  event,
		Notes:  notes,
		UserID: userID,
		IP:     ip,
	}
}
