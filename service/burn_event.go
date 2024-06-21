package service

import (
	"time"

	"github.com/ericlagergren/decimal"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
)

// GetAllBurnEvents - returns a list of PRDX token burn events
func (service *Service) GetAllBurnEvents(limit, page int, query string) (*model.BurnEventList, error) {
	events := make([]model.BurnEvent, 0)
	var rowCount int64 = 0
	q := service.repo.ConnReader.Table("burn_events")
	if len(query) > 0 {
		q = q.Where("date_burnt::date = ? ", query)
	}
	//dbc := q.Select("count(burn_events.id) as total").Row()
	//_ = dbc.Scan(&rowCount)

	dbc := q.Count(&rowCount)

	if dbc.Error != nil {
		return nil, dbc.Error
	}

	if limit != 0 {
		q = q.Limit(limit).Offset((page - 1) * limit)
	}
	q = q.Order("date_burnt DESC")
	q = q.Order("created_at DESC")
	db := q.Find(&events)
	burnEvents := model.BurnEventList{
		BurnEvents: events,
		Meta: model.PagingMeta{
			Page:  int(page),
			Count: rowCount,
			Limit: int(limit),
		},
	}
	return &burnEvents, db.Error
}

// AddBurnEvent - add a new PRDX token event
func (service *Service) AddBurnEvent(volume, amountBurnt, day string) (*model.BurnEvent, error) {
	volumeDecimal, _ := new(decimal.Big).SetString(volume)
	amountDecimal, _ := new(decimal.Big).SetString(amountBurnt)
	time, _ := time.Parse(time.RFC3339, day)
	event := model.NewBurnEvent(volumeDecimal, &time, amountDecimal)
	err := service.repo.Create(event)
	return event, err
}

// RemoveBurnEvent - remove a PRDX burn event from db
func (service *Service) RemoveBurnEvent(eventID string) error {
	event := model.BurnEvent{}
	db := service.repo.Conn.First(&event, "id=?", eventID)
	if db.Error != nil {
		return db.Error
	}
	db = service.repo.Conn.Delete(event)
	return db.Error
}
