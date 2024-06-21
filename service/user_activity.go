package service

import (
	"encoding/json"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/model"
	"xojoc.pw/useragent"
)

// ParseUserAgent -
func (service *Service) ParseUserAgent(userAgent string) (string, error) {
	ua := useragent.Parse(userAgent)
	notes, err := json.Marshal(ua)
	return string(notes), err
}

// AddUserActivity - add user activity into database
func (service *Service) AddUserActivity(event string, userID uint64, geoLocation GeoLocation) (*model.UserActivity, error) {
	notes, err := json.Marshal(geoLocation)
	if err != nil {
		return nil, err
	}
	userActivity := model.NewUserActivity(event, string(notes), geoLocation.IP, userID)
	db := service.repo.Conn.Create(userActivity)
	return userActivity, db.Error
}
