package service

import "gitlab.com/paramountdax-exchange/exchange_api_v2/model"

func (service *Service) GetMonitoringOptionsList(limit, page int) (*model.MonitoringOptionsList, error) {
	monitoringOptions := make([]model.MonitoringOption, 0)
	var rowCount int64 = 0

	reader := service.repo.ConnReader
	dbc := reader.Table("monitoring_options").Select("count(*) as total").Row()
	err := dbc.Scan(&rowCount)

	if err != nil {
		return nil, err
	}

	db := reader.Order("id DESC")

	if limit == 0 {
		db = db.Find(&monitoringOptions)
	} else {
		db = db.Limit(limit).Offset((page - 1) * limit).Find(&monitoringOptions)
	}

	if db.Error != nil {
		return nil, db.Error
	}

	return &model.MonitoringOptionsList{
		MonitoringOptions: monitoringOptions,
		Meta: model.PagingMeta{
			Page:  page,
			Count: rowCount,
			Limit: limit,
		},
	}, nil
}

func (service *Service) UpdateMonitoringOption(updateRequest model.MonitoringOptionUpdateRequest) (*model.MonitoringOption, error) {

	monitoringOption := new(model.MonitoringOption)

	db := service.repo.ConnReader.First(&monitoringOption, "id = ?", updateRequest.Id)

	if db.Error != nil {
		return nil, db.Error
	}

	monitoringOption.UpdateMonitoringOption(updateRequest)

	service.repo.Conn.Table("monitoring_options").Save(monitoringOption)

	return monitoringOption, nil
}
