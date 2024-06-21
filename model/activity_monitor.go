package model

import (
	postgresDialects "github.com/jinzhu/gorm/dialects/postgres"
	"time"
)

type ActivityMonitoringType string
type ActivityMonitoringStatus string

const (
	ActivityMonitoringTypeLowLiquidity         ActivityMonitoringType = "low_liquidity"
	ActivityMonitoringTypeTradeHistoryInactive ActivityMonitoringType = "trade_history_inactive"
	ActivityMonitoringTypeOrderBookStationary  ActivityMonitoringType = "order_book_stationary"

	ActivityMonitoringStatusError   ActivityMonitoringStatus = "Error"
	ActivityMonitoringStatusSuccess ActivityMonitoringStatus = "Success"
)

type ActivityMonitoringEntry struct {
	Id             uint64
	MonitoringType ActivityMonitoringType
	Status         ActivityMonitoringStatus
	AdditionalInfo postgresDialects.Jsonb
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ActivityMonitorList struct {
	ActivityMonitors []ActivityMonitoringEntry
}

func (activityMonitorEntry *ActivityMonitoringEntry) IsError() bool {
	return activityMonitorEntry.Status == ActivityMonitoringStatusError
}
