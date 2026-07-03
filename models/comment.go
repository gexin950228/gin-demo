package models

import (
	"time"

	"gorm.io/gorm"
)

// Comment 文章评论
type Comment struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	ArticleID uint      `gorm:"column:article_id;not null;index" json:"article_id"`
	UserID    uint      `gorm:"column:user_id;not null;index" json:"user_id"`
	ParentID  uint      `gorm:"column:parent_id;default:0;index" json:"parent_id"` // 0=顶级评论，否则为直接回复的评论ID
	// RootID 根评论ID：0=顶级评论本身；>0 表示该评论是某顶级评论下的回复（不论回复的是顶级还是其他回复）
	// 用于"两级平铺"展示：所有回复都挂在对应顶级评论下，避免多轮对话无限缩进
	RootID    uint      `gorm:"column:root_id;default:0;index" json:"root_id"`
	// ReplyToUserID 被回复的用户ID（仅回复时有值，0 表示回复顶级评论或顶级评论本身）
	ReplyToUserID uint   `gorm:"column:reply_to_user_id;default:0" json:"reply_to_user_id"`
	Content   string    `gorm:"column:content;type:text;not null" json:"content"`
	// Images 存储评论图片的 MinIO URL 列表（JSON 数组字符串，如 ["http://.../1.png"]）
	// 表情包（unicode emoji）直接写在 Content 文本字段里，无需单独存储
	Images    string    `gorm:"column:images;type:text" json:"images"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"` // 软删除，不暴露给前端

	// 仅用于 JSON 响应，不映射数据库列
	UserName       string     `gorm:"-" json:"user_name"`
	Avatar         string     `gorm:"-" json:"avatar"`
	ReplyToUserName string    `gorm:"-" json:"reply_to_user_name"` // 被回复者用户名（前端显示 @xxx 用）
	Replies        []*Comment `gorm:"-" json:"replies"`             // 子回复（仅两级平铺：顶级评论下的所有回复列表，用指针避免值拷贝丢失深层回复）
}

func (Comment) TableName() string {
	return "comments"
}
