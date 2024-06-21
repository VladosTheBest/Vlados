package service

import "gitlab.com/paramountdax-exchange/exchange_api_v2/model"

func (service *Service) AddAdminActivity(requestUrl, requestBody, ip, method string, userId uint64) error {
	adminActivity := model.NewAdminActivity(requestUrl, requestBody, ip, method, userId)

	db := service.repo.Conn.Create(adminActivity)
	if db.Error != nil {
		return db.Error
	}

	return nil
}
