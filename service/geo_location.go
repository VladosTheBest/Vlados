package service

import (
	"encoding/json"
	// "github.com/jinzhu/gorm"
	"gitlab.com/paramountdax-exchange/exchange_api_v2/lib/iplocate"
)

// GeoLocation type
type GeoLocation struct {
	IP       string                 `json:"ip"`
	Country  string                 `json:"country"`
	City     string                 `json:"city"`
	Agent    map[string]interface{} `json:"agent"`
	Domain   string                 `json:"domain"`
	Timezone string                 `json:"time_zone"`
}

// getGeoLocationFromAPILocate - get details about geolocation from database
func (service *Service) getGeoLocationFromAPILocate(ip, userAgent string) (GeoLocation, error) {
	geoLocation := GeoLocation{}
	var jsonUA map[string]interface{}
	location, err := iplocate.LocateIP(ip)
	if err != nil {
		return geoLocation, err
	}

	err = json.Unmarshal([]byte(userAgent), &jsonUA)

	geoLocation.IP = location.IP
	geoLocation.Country = location.Country
	geoLocation.City = location.City
	geoLocation.Agent = jsonUA
	geoLocation.Domain = service.apiConfig.Domain
	geoLocation.Timezone = location.Timezone

	return geoLocation, err
}

// ChooseGeoLocation - determinate what geo location we should use: from the database if the user has been logged in from the current ip, otherwise from the iplocate component
func (service *Service) ChooseGeoLocation(userID uint64, ip, userAgent string) (GeoLocation, error) {
	//disabled reading from DB because agent needs to have real data, if someone changes browser or device but is in same location / IP
	//TODO optimise this to use less call to GeoLocation API
	//db, err := service.getGeoLocationFromDB(userID, ip)
	//if err == nil {
	//	return db, nil
	//}
	//if gorm.IsRecordNotFoundError(err) {
	geoLocation, err := service.getGeoLocationFromAPILocate(ip, userAgent)
	if err != nil {
		return geoLocation, err
	}
	return geoLocation, nil
}
