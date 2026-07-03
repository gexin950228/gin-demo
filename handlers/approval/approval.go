package approval

import (
	"fmt"
	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/skywalking"
	"gin-demo/ws"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ApprovalListPage 审批列表页（作者视角：查看别人对我文章的编辑申请）
func ApprovalListPage(c *gin.Context) {
	username := c.GetString("username")
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)
	avatar := ""
	if userID > 0 {
		var user models.User
		if err := models.DB.Select("avatar").First(&user, userID).Error; err == nil {
			avatar = user.Avatar
		}
	}

	// 可选状态筛选
	statusStr := c.Query("status")
	var approvals []models.ArticleEditApproval

	query := models.ArticleDB.WithContext(ctx).
		Preload("Article").
		Where("author_id = ?", userID).
		Order("created_at DESC")

	if statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			query = query.Where("status = ?", status)
		}
	}

	if err := query.Find(&approvals).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Error("查询审批列表失败")
	}

	// 跨库手动填充 Editor/Author 信息（User 表在 gin_project 库，不能用 Preload）
	fillApprovalsUsers(approvals)

	// 统计待审批数量
	pendingCount, _ := models.GetPendingApprovalCount(ctx, userID)

	c.HTML(http.StatusOK, "approval_list.html", gin.H{
		"username":      username,
		"avatar_url":    avatar,
		"url_avatar_upload": routes.Reverse(routes.UserAvatarUpload),
		"approvals":     approvals,
		"pendingCount":  pendingCount,
		"currentStatus": statusStr,
		"url_home":              routes.Reverse(routes.Home),
		"url_logout":            routes.Reverse(routes.UserLogout),
		"url_article_create":    routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":      routes.Reverse(routes.ArticleListPage),
		"url_tag_manage":        routes.Reverse(routes.TagManagePage),
		"url_approval_list":     routes.Reverse(routes.ApprovalListPage),
		"url_notification_list": routes.Reverse(routes.NotificationListPage),
	})
}

// ApprovalDetailAPI 审批详情 API（返回审批记录的完整信息，供前端预览）
func ApprovalDetailAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的审批ID"})
		return
	}

	var approval models.ArticleEditApproval
	if err := models.ArticleDB.WithContext(ctx).
		Preload("Article").
		First(&approval, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "审批记录不存在"})
		return
	}

	// 跨库手动填充 Editor/Author 信息（User 表在 gin_project 库，不能用 Preload）
	fillApprovalUsers(&approval)

	// 权限校验：只有作者本人能查看
	if approval.AuthorID != userID {
		logger.WithFields(map[string]interface{}{
			"approval_id":  id,
			"user_id":      userID,
			"author_id":    approval.AuthorID,
		}).Warn("无权查看此审批")
		c.JSON(http.StatusForbidden, gin.H{"error": "无权查看此审批"})
		return
	}

	// 解析编辑后的标签
	var tags []string
	if approval.Tags != "" {
		for _, t := range strings.Split(approval.Tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	// 优先使用审批记录中保存的原文快照（确保历史审批也能正确对比）
	// 兼容老数据：若快照为空，回退到查询 articles 表
	var originalTitle, originalContent string
	var originalTags []string
	if approval.OriginalTitle != "" || approval.OriginalContent != "" || approval.OriginalTags != "" {
		originalTitle = approval.OriginalTitle
		originalContent = approval.OriginalContent
		if approval.OriginalTags != "" {
			for _, t := range strings.Split(approval.OriginalTags, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					originalTags = append(originalTags, t)
				}
			}
		}
	} else {
		var article models.Article
		models.ArticleDB.WithContext(ctx).Preload("Tags").First(&article, approval.ArticleID)
		originalTitle = article.Title
		originalContent = article.Content
		for _, t := range article.Tags {
			originalTags = append(originalTags, t.Name)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":              approval.ID,
			"article_id":      approval.ArticleID,
			"article_title":   approval.Article.Title,
			"original_title":  originalTitle,
			"original_content": originalContent,
			"original_tags":   originalTags,
			"editor_name":     approval.Editor.UserName,
			"title":           approval.Title,
			"content":         approval.Content,
			"tags":            tags,
			"status":          approval.Status,
			"review_comment":  approval.ReviewComment,
			"created_at":      approval.CreatedAt,
			"reviewed_at":     approval.ReviewedAt,
		},
	})
}

// ApproveArticle 通过审批：把审批内容写入 articles 表
func ApproveArticle(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的审批ID"})
		return
	}

	var approval models.ArticleEditApproval
	if err := models.ArticleDB.WithContext(ctx).First(&approval, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "审批记录不存在"})
		return
	}

	// 权限校验
	if approval.AuthorID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此审批"})
		return
	}

	if approval.Status != models.ApprovalStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该审批已处理"})
		return
	}

	// 事务：更新文章 + 更新审批状态 + 同步标签
	tx := models.ArticleDB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			logger.WithFields(map[string]interface{}{
				"approval_id": id,
				"panic":        r,
				"stack":        string(debug.Stack()),
			}).Error("ApproveArticle panic 触发事务回滚")
		}
	}()

	// 更新文章
	var article models.Article
	if err := tx.First(&article, approval.ArticleID).Error; err != nil {
		tx.Rollback()
		logger.WithFields(map[string]interface{}{
			"approval_id": id,
			"article_id":  approval.ArticleID,
			"error":       err,
		}).Error("通过审批时文章不存在")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文章不存在"})
		return
	}
	if err := tx.Model(&article).Updates(map[string]interface{}{
		"title":   approval.Title,
		"content": approval.Content,
	}).Error; err != nil {
		tx.Rollback()
		logger.WithFields(map[string]interface{}{
			"approval_id": id,
			"article_id":  approval.ArticleID,
			"error":       err,
		}).Error("通过审批时更新文章失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新文章失败"})
		return
	}

	// 同步标签：直接操作关联表 article_tags，避免 GORM Association 在事务中的潜在问题
	// 1. 清空旧关联
	if err := tx.Exec("DELETE FROM article_tags WHERE article_id = ?", article.ID).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": article.ID,
			"error":      err,
		}).Warn("通过审批时清空旧标签关联失败")
	}
	// 2. 建立新关联
	if approval.Tags != "" {
		for _, name := range strings.Split(approval.Tags, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			var tag models.Tag
			result := tx.Where("name = ?", name).First(&tag)
			if result.RowsAffected == 0 {
				tag = models.Tag{Name: name}
				if err := tx.Create(&tag).Error; err != nil {
					logger.WithFields(map[string]interface{}{
						"tag":   name,
						"error": err,
					}).Warn("通过审批时创建标签失败")
					continue
				}
			}
			if err := tx.Exec("INSERT INTO article_tags (article_id, tag_id) VALUES (?, ?)", article.ID, tag.ID).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"article_id": article.ID,
					"tag_id":     tag.ID,
					"tag":        name,
					"error":      err,
				}).Warn("通过审批时关联标签失败")
			}
		}
	}

	// 更新审批状态
	now := time.Now()
	if err := tx.Model(&approval).Updates(map[string]interface{}{
		"status":         models.ApprovalStatusApproved,
		"reviewed_at":    &now,
		"review_comment": c.PostForm("comment"),
	}).Error; err != nil {
		tx.Rollback()
		logger.WithFields(map[string]interface{}{
			"approval_id": id,
			"error":       err,
		}).Error("通过审批时更新审批状态失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新审批状态失败"})
		return
	}

	tx.Commit()

	// WebSocket 推送最新待审批数量给作者（审批已处理，数量减少）
	ws.PushApprovalCount(approval.AuthorID)

	// 邮件通知编辑者
	go func() {
		var editor models.User
		if err := models.DB.First(&editor, approval.EditorID).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"editor_id": approval.EditorID,
				"error":     err,
			}).Warn("通过审批通知邮件时查询编辑者失败")
			return
		}
		if editor.Email == "" {
			return
		}
		subject := "【Gin Demo】您的文章编辑申请已通过"
		body := fmt.Sprintf(`
			<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #27ae60;">编辑申请已通过</h2>
				<p>您对文章《<strong>%s</strong>》的编辑申请已通过审批，修改已生效。</p>
				<p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
			</div>
		`, article.Title)
		if err := models.SendEmail(editor.Email, subject, body); err != nil {
			logger.WithFields(map[string]interface{}{
				"to":    editor.Email,
				"error": err,
			}).Error("通过审批通知邮件发送失败")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已通过"})
}

// RejectArticle 拒绝审批
func RejectArticle(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的审批ID"})
		return
	}

	var approval models.ArticleEditApproval
	if err := models.ArticleDB.WithContext(ctx).First(&approval, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "审批记录不存在"})
		return
	}

	if approval.AuthorID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作此审批"})
		return
	}

	if approval.Status != models.ApprovalStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该审批已处理"})
		return
	}

	now := time.Now()
	if err := models.ArticleDB.WithContext(ctx).Model(&approval).Updates(map[string]interface{}{
		"status":         models.ApprovalStatusRejected,
		"reviewed_at":    &now,
		"review_comment": c.PostForm("comment"),
	}).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"approval_id": id,
			"error":       err,
		}).Error("拒绝审批时更新状态失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	// WebSocket 推送最新待审批数量给作者（审批已处理，数量减少）
	ws.PushApprovalCount(approval.AuthorID)

	// 邮件通知编辑者
	go func() {
		var editor models.User
		if err := models.DB.First(&editor, approval.EditorID).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"editor_id": approval.EditorID,
				"error":     err,
			}).Warn("拒绝审批通知邮件时查询编辑者失败")
			return
		}
		if editor.Email == "" {
			return
		}
		subject := "【Gin Demo】您的文章编辑申请已被拒绝"
		body := fmt.Sprintf(`
			<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
				<h2 style="color: #e74c3c;">编辑申请已被拒绝</h2>
				<p>您对文章《<strong>%s</strong>》的编辑申请已被拒绝，原文章内容保持不变。</p>
				<p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
			</div>
		`, approval.Article.Title)
		if err := models.SendEmail(editor.Email, subject, body); err != nil {
			logger.WithFields(map[string]interface{}{
				"to":    editor.Email,
				"error": err,
			}).Error("拒绝审批通知邮件发送失败")
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已拒绝"})
}

// RevokeApproval 撤回审批（编辑者主动撤回自己的编辑申请）
func RevokeApproval(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的审批ID"})
		return
	}

	var approval models.ArticleEditApproval
	if err := models.ArticleDB.WithContext(ctx).First(&approval, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "审批记录不存在"})
		return
	}

	// 权限校验：只有编辑者本人能撤回
	if approval.EditorID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权撤回此审批"})
		return
	}

	if approval.Status != models.ApprovalStatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该审批已处理，无法撤回"})
		return
	}

	if err := models.ArticleDB.WithContext(ctx).Model(&approval).Update("status", models.ApprovalStatusRevoked).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"approval_id": id,
			"error":       err,
		}).Error("撤回审批时更新状态失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}

	// WebSocket 推送最新待审批数量给作者（审批已撤回，数量减少）
	ws.PushApprovalCount(approval.AuthorID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "已撤回"})
}

// PendingApprovalCountAPI 获取当前用户的待审批数量（供前端轮询红点）
func PendingApprovalCountAPI(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	userID := c.GetUint("user_id")
	count, err := models.GetPendingApprovalCount(ctx, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Error("查询待审批数量失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "count": count})
}

// fillApprovalUsers 手动填充审批记录的编辑者和作者信息（跨库：User 表在 gin_project 库）
func fillApprovalUsers(approval *models.ArticleEditApproval) {
	if approval.EditorID > 0 {
		var editor models.User
		if err := models.DB.Select("id, user_name, email").First(&editor, approval.EditorID).Error; err == nil {
			approval.Editor = editor
		}
	}
	if approval.AuthorID > 0 {
		var author models.User
		if err := models.DB.Select("id, user_name, email").First(&author, approval.AuthorID).Error; err == nil {
			approval.Author = author
		}
	}
}

// fillApprovalsUsers 批量填充审批列表的编辑者和作者信息（跨库，避免 N+1）
func fillApprovalsUsers(approvals []models.ArticleEditApproval) {
	if len(approvals) == 0 {
		return
	}
	userIDs := make(map[uint]bool)
	for _, a := range approvals {
		if a.EditorID > 0 {
			userIDs[a.EditorID] = true
		}
		if a.AuthorID > 0 {
			userIDs[a.AuthorID] = true
		}
	}
	if len(userIDs) == 0 {
		return
	}
	ids := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		ids = append(ids, id)
	}
	var users []models.User
	if err := models.DB.Select("id, user_name, email").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return
	}
	userMap := make(map[uint]models.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	for i := range approvals {
		if u, ok := userMap[approvals[i].EditorID]; ok {
			approvals[i].Editor = u
		}
		if u, ok := userMap[approvals[i].AuthorID]; ok {
			approvals[i].Author = u
		}
	}
}
