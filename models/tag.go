package models

import "gorm.io/gorm"

// Tag 标签模型
type Tag struct {
	gorm.Model
	Name string `gorm:"column:name;size:50;not null;uniqueIndex" json:"name"`
}

func (Tag) TableName() string {
	return "tags"
}
