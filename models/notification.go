package models

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// 通知类型常量
const (
	NotificationTypeComment       = 1 // 文章被评论
	NotificationTypeReply         = 2 // 评论被回复
)

// Notification 消息通知
// 当用户收到评论或评论被回复时生成一条通知，接收者在右上角下拉窗看到提示
type Notification struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	// 接收者用户ID（文章作者 或 被回复评论的发送者）
	UserID    uint      `gorm:"column:user_id;not null;index" json:"user_id"`
	// 触发者用户ID（评论/回复的发送者）
	ActorID   uint      `gorm:"column:actor_id;not null" json:"actor_id"`
	// 通知类型：1=文章被评论 2=评论被回复
	Type      int       `gorm:"column:type;not null;index" json:"type"`
	// 关联文章ID（点击跳转用）
	ArticleID uint      `gorm:"column:article_id;not null" json:"article_id"`
	// 关联评论ID（定位到评论用，对应触发通知的那条评论）
	CommentID uint      `gorm:"column:comment_id;not null" json:"comment_id"`
	// 通知内容摘要（评论内容前 N 字，便于预览）
	Summary   string    `gorm:"column:summary;type:text" json:"summary"`
	// 是否已读：0=未读 1=已读
	IsRead    int       `gorm:"column:is_read;default:0;index" json:"is_read"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`

	// 仅用于 JSON 响应，不映射数据库列
	ActorName string `gorm:"-" json:"actor_name"`
	ActorAvatar string `gorm:"-" json:"actor_avatar"`
	ArticleTitle string `gorm:"-" json:"article_title"`
}

func (Notification) TableName() string {
	return "notifications"
}

// GetUnreadNotificationCount 返回指定用户的未读通知数量
func GetUnreadNotificationCount(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := ArticleDB.WithContext(ctx).
		Model(&Notification{}).
		Where("user_id = ? AND is_read = ?", userID, 0).
		Count(&count).Error
	return count, err
}
