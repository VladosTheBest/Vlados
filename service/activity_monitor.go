package service

import "gitlab.com/paramountdax-exchange/exchange_api_v2/model"

func (service *Service) GetActivityMonitorList() (*model.ActivityMonitorList, error) {
	activityMonitorList := make([]model.ActivityMonitoringEntry, 0)

	reader := service.repo.ConnReader
	db := reader.Select("DISTINCT ON (monitoring_type) id, monitoring_type, status, additional_info, created_at").Order("monitoring_type, id desc").
		Find(&activityMonitorList)

	if db.Error != nil {
		return nil, db.Error
	}

	return &model.ActivityMonitorList{
		ActivityMonitors: activityMonitorList,
	}, nil
}
