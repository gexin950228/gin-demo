package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	UserName string `gorm:"column:user_name;size:50;not null;uniqueIndex" json:"user_name"`
	Email    string `gorm:"column:email;size:100;not null;uniqueIndex" json:"email"`
	Password string `gorm:"column:password;size:255;not null" json:"-"`
	Gender   int64  `gorm:"default:0;column:gender" json:"gender"`
	Age      int64  `gorm:"column:age;default:0" json:"age"`
	Phone    string `gorm:"column:phone_number;size:20" json:"phone"`
	Avatar   string `gorm:"column:avatar;size:255" json:"avatar"`
}

func (User) TableName() string {
	return "users"
}
