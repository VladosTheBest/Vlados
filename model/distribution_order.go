package model

import (
	"encoding/json"
	"time"

	"github.com/ericlagergren/decimal"
	"github.com/ericlagergren/decimal/sql/postgres"

	"gitlab.com/paramountdax-exchange/exchange_api_v2/utils"
)

type DistributionOrderStatus string

const (
	DistributionOrderStatus_Pending         DistributionOrderStatus = "pending"
	DistributionOrderStatus_Untouched       DistributionOrderStatus = "untouched"
	DistributionOrderStatus_PartiallyFilled DistributionOrderStatus = "partially_filled"
	DistributionOrderStatus_Cancelled       DistributionOrderStatus = "cancelled"
	DistributionOrderStatus_Filled          DistributionOrderStatus = "filled"
)

type DistributionOrderList struct {
	DistributionOrders []DistributionOrder
	Meta               PagingMeta
}

// Order structure
type DistributionOrder struct {
	ID       uint64                  `sql:"type:bigint" gorm:"PRIMARY_KEY" json:"id"`
	RefID    string                  `gorm:"column:ref_id" json:"ref_id"`
	OrderID  uint64                  `sql:"type:bigint REFERENCES orders(id)" json:"order_id"`
	Status   DistributionOrderStatus `sql:"not null;type:order_status_t;default:'pending'" json:"status"`
	Amount   *postgres.Decimal       `sql:"type:decimal(36,18)" json:"amount"`
	Price    *postgres.Decimal       `sql:"type:decimal(36,18)" json:"price"`
	MarketID string                  `sql:"type:varchar(10) REFERENCES markets(id)" json:"market_id"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewDistributionOrder(refID string, orderID uint64, marketID string, price, amount *decimal.Big) *DistributionOrder {
	return &DistributionOrder{
		RefID:    refID,
		Status:   DistributionOrderStatus_Pending,
		MarketID: marketID,
		OrderID:  orderID,
		Amount:   &postgres.Decimal{V: amount},
		Price:    &postgres.Decimal{V: price},
	}
}

func (distributionOrder DistributionOrder) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"ref_id":     distributionOrder.RefID,
		"status":     distributionOrder.Status,
		"market_id":  distributionOrder.MarketID,
		"order_id":   distributionOrder.OrderID,
		"debit":      utils.Fmt(distributionOrder.Amount.V),
		"price":      utils.Fmt(distributionOrder.Price.V),
		"created_at": distributionOrder.CreatedAt.Unix(),
		"updated_at": distributionOrder.UpdatedAt.Unix(),
	})
}
