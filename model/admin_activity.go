package model

import "time"

type AdminActivity struct {
	ID          uint64    `gorm:"primary_key" json:"id"`
	RequestUrl  string    `json:"request_url"`
	RequestBody string    `sql:"type:text" json:"request_body"`
	UserID      uint64    `sql:"type:text" json:"user_id"`
	IP          string    `sql:"type:text" json:"ip"`
	Method      string    `json:"method"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func NewAdminActivity(requestUrl, requestBody, ip, method string, userId uint64) *AdminActivity {
	return &AdminActivity{
		RequestUrl:  requestUrl,
		RequestBody: requestBody,
		IP:          ip,
		Method:      method,
		UserID:      userId,
	}
}
