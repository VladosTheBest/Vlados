package model

import "time"

type MonitoringOptionsStatus string

const (
	MonitoringOptionStatus_Enabled  MonitoringOptionsStatus = "enabled"
	MonitoringOptionStatus_Disabled MonitoringOptionsStatus = "disabled"
)

type MonitoringOptionOption string

const (
	MonitoringOption_LowLiquidityMmAccount = "low_liquidity_mm_account"
	MonitoringOption_LowLiquidityDayOffset = "low_liquidity_day_offset"
)

type MonitoringOption struct {
	Id        uint64                  `sql:"type:bigint" gorm:"PRIMARY_KEY"`
	Option    string                  `json:"option"`
	Value     string                  `json:"value"`
	Status    MonitoringOptionsStatus `json:"status"`
	CreatedAt time.Time               `json:"-"`
	UpdatedAt time.Time               `json:"-"`
}

type MonitoringOptionUpdateRequest struct {
	Id     uint64                  `json:"id"`
	Value  string                  `json:"value"`
	Status MonitoringOptionsStatus `json:"status"`
}

type MonitoringOptionsList struct {
	MonitoringOptions []MonitoringOption
	Meta              PagingMeta
}

func (monitoringOption MonitoringOption) UpdateMonitoringOption(updateRequest MonitoringOptionUpdateRequest) MonitoringOption {
	monitoringOption.Value = updateRequest.Value
	monitoringOption.Status = updateRequest.Status
	monitoringOption.UpdatedAt = time.Now()

	return monitoringOption
}
