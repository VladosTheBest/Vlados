package model

import (
	"time"
)

// SystemRevertOrderQueue structure
type SystemRevertOrderQueue struct {
	ID        uint64    `sql:"type:bigint" gorm:"primary_key" json:"id"`
	Order     Order     `gorm:"foreignkey:OrderID" json:"-"`
	OrderID   uint64    `sql:"type:bigint REFERENCES orders(id)" json:"order_id"`
	RevertAt  time.Time `json:"revert_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewSystemRevertOrderQueue godoc
func NewSystemRevertOrderQueue(orderID uint64, delayMin int) *SystemRevertOrderQueue {
	return &SystemRevertOrderQueue{
		OrderID:  orderID,
		RevertAt: time.Now().Add(time.Minute * time.Duration(delayMin)),
	}
}

// TableName godoc
func (SystemRevertOrderQueue) TableName() string {
	return "system_revert_order_queue"
}
