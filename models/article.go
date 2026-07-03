package models

import (
	"time"
	"gorm.io/gorm"
)

type Article struct {
	gorm.Model
	Title     string    `gorm:"column:title;size:200;not null" json:"title"`
	Content   string    `gorm:"column:content;type:text" json:"content"`
	UserID    uint      `gorm:"column:user_id;not null;index" json:"user_id"`
	IsDeleted bool      `gorm:"column:is_deleted;default:0;index" json:"is_deleted"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
	Tags      []Tag     `gorm:"many2many:article_tags;" json:"tags"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Article) TableName() string {
	return "articles"
}
