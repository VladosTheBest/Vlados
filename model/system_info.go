package model

import "time"

type MaintenanceMessageStatus string

const (
	MaintenanceMessageStatusActive   MaintenanceMessageStatus = "active"
	MaintenanceMessageStatusDisabled MaintenanceMessageStatus = "disabled"
)

func (ms MaintenanceMessageStatus) IsValid() bool {
	switch ms {
	case MaintenanceMessageStatusActive,
		MaintenanceMessageStatusDisabled:
		return true
	default:
		return false
	}
}

type MaintenanceMessage struct {
	ID        uint64                   `form:"id"         json:"id"`
	Title     string                   `form:"title"      json:"title"`
	Message   string                   `form:"message"    json:"message"`
	Status    MaintenanceMessageStatus `form:"status"     json:"status"`
	CreatedAt time.Time                `form:"-"          json:"created_at"`
}
