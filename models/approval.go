package models

import (
	"context"
	"time"
	"gorm.io/gorm"
)

// 审批状态常量
const (
	ApprovalStatusPending  = 0 // 待审批
	ApprovalStatusApproved = 1 // 已通过
	ApprovalStatusRejected = 2 // 已拒绝
	ApprovalStatusRevoked  = 3 // 已撤回（编辑者主动撤回）
)

// ArticleEditApproval 文章编辑审批
// 当非作者编辑他人文章时，提交的修改不会立即生效，而是写入此表等待作者审批
type ArticleEditApproval struct {
	gorm.Model
	ArticleID    uint   `gorm:"column:article_id;not null;index" json:"article_id"`
	EditorID     uint   `gorm:"column:editor_id;not null;index" json:"editor_id"`     // 提交编辑的人
	AuthorID     uint   `gorm:"column:author_id;not null;index" json:"author_id"`     // 文章作者（审批人）
	Title        string `gorm:"column:title;size:200;not null" json:"title"`          // 编辑后的标题
	Content      string `gorm:"column:content;type:text;not null" json:"content"`     // 编辑后的内容
	Tags         string `gorm:"column:tags;type:text" json:"tags"`                    // 编辑后的标签（逗号分隔）
	// 原文快照（提交审批时保存，确保历史审批详情始终能正确对比）
	OriginalTitle   string `gorm:"column:original_title;size:200" json:"original_title"`
	OriginalContent string `gorm:"column:original_content;type:text" json:"original_content"`
	OriginalTags    string `gorm:"column:original_tags;type:text" json:"original_tags"`
	Status       int    `gorm:"column:status;default:0;index" json:"status"`          // 0=待审批 1=通过 2=拒绝 3=撤回
	ReviewComment string `gorm:"column:review_comment;size:500" json:"review_comment"` // 审批意见
	ReviewedAt   *time.Time `gorm:"column:reviewed_at" json:"reviewed_at"`             // 审批时间（null 表示未审批）
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`

	// 关联（仅用于查询时预加载，不参与审批流程）
	Article Article `gorm:"foreignKey:ArticleID" json:"article"`
	Editor  User    `gorm:"foreignKey:EditorID" json:"editor"`
	Author  User    `gorm:"foreignKey:AuthorID" json:"author"`
}

func (ArticleEditApproval) TableName() string {
	return "article_edit_approvals"
}

// GetPendingApprovalCount 获取指定用户的待审批数量
// ctx 用于传递 SkyWalking 链路上下文，传入 nil 时回退到默认 context
func GetPendingApprovalCount(ctx context.Context, userID uint) (int64, error) {
	var count int64
	err := ArticleDB.WithContext(ctx).Model(&ArticleEditApproval{}).
		Where("author_id = ? AND status = ?", userID, ApprovalStatusPending).
		Count(&count).Error
	return count, err
}
