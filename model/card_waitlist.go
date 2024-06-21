package model

import "time"

type CardWaitList struct {
	ID               uint64    `json:"id" gorm:"PRIMARY_KEY"`
	Firstname        string    `json:"first_name" gorm:"column:first_name"`
	Lastname         string    `json:"last_name" gorm:"column:last_name"`
	Email            string    `json:"email" sql:"notnull" gorm:"column:email;unique"`
	Country          string    `json:"country" sql:"notnull"`
	IsRegisteredUser bool      `json:"is_registered_user" gorm:"column:is_registered_user" sql:"not null; default:false"`
	CreatedAt        time.Time `json:"-"`
}
