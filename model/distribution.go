package model

import (
	"encoding/json"
	"time"
)

type DistributionStatus string

const (
	DistributionStatus_Pending    DistributionStatus = "pending"
	DistributionStatus_Accepted   DistributionStatus = "accepted"
	DistributionStatus_Failed     DistributionStatus = "failed"
	DistributionStatus_Reverted   DistributionStatus = "reverted"
	DistributionStatus_Processing DistributionStatus = "processing"
	DistributionStatus_Completed  DistributionStatus = "completed"
)

// DistributionList structure
type DistributionList struct {
	Distributions []Distribution
	Meta          PagingMeta
}

// Distribution provides a referance for each distribution event
type Distribution struct {
	ID        uint64             `gorm:"PRIMARY_KEY"`
	RefID     string             `gorm:"column:ref_id" json:"ref_id"`
	Status    DistributionStatus `sql:"not null;type:distribution_status_t;default:'pending'" json:"status"`
	Day       time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewDistribution Create a new operation for a user
func NewDistribution(refID string, status DistributionStatus, day time.Time) *Distribution {
	return &Distribution{
		RefID:     refID,
		Status:    status,
		Day:       day,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (distribution Distribution) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"ref_id": distribution.RefID,
		"status": distribution.Status,
		"day":    distribution.Day.Unix(),
	})
}
