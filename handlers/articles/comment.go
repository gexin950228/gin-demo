package articles

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/skywalking"
	"gin-demo/ws"

	"github.com/gin-gonic/gin"
)

// 评论图片限制
const (
	commentImageMaxSize    = 1 << 20 // 1MB
	commentImageMaxCount   = 3       // 每条评论最多3张图片
	allowedImageContentTypes = "image/jpeg,image/png,image/gif,image/webp"
)

// CommentListAPI 获取文章评论列表（树形）
// @Summary 评论列表
// @Description 获取指定文章的所有评论，组装成树形（顶级评论 + Replies）
// @Tags 评论管理
// @Produce json
// @Param id query int true "文章ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /articles/api/comments [get]
func CommentListAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	idStr := c.Query("id")
	articleID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || articleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	// 查所有评论（GORM 自动过滤软删除）
	var allComments []models.Comment
	if err := models.ArticleDB.WithContext(ctx).
		Where("article_id = ?", articleID).
		Order("created_at asc").
		Find(&allComments).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": articleID,
			"user_id":    userID,
			"error":      err,
		}).Warn("CommentListAPI 查询评论列表失败")
		c.JSON(http.StatusOK, gin.H{"comments": []interface{}{}, "count": 0})
		return
	}

	// 跨库批量填充用户信息（评论作者 + 被回复者）
	fillCommentUsers(ctx, allComments)
	fillReplyToUsers(ctx, allComments)

	// 组装两级平铺结构：
	// - 顶级评论（root_id=0）作为第一层
	// - 所有回复（root_id>0）平铺挂在对应顶级评论的 Replies 下，不再递归嵌套
	// 这样无论多少轮对话都只缩进一层，避免内容被挤压
	commentMap := make(map[uint]*models.Comment, len(allComments))
	for i := range allComments {
		if allComments[i].RootID == 0 {
			commentMap[allComments[i].ID] = &allComments[i]
		}
	}
	topComments := make([]*models.Comment, 0, len(allComments))
	for i := range allComments {
		c := &allComments[i]
		if c.RootID == 0 {
			// 顶级评论
			topComments = append(topComments, c)
		} else if root, ok := commentMap[c.RootID]; ok {
			// 回复：挂到对应顶级评论下
			root.Replies = append(root.Replies, c)
		} else {
			// 顶级评论已删，回复作为顶级显示（孤儿评论）
			topComments = append(topComments, c)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"comments": topComments,
		"count":    len(allComments),
	})
}

// CommentCreateAPI 发表评论或回复
// @Summary 发表评论
// @Description 发表评论或回复评论。parent_id 为 0 表示顶级评论。
// @Tags 评论管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param article_id formData int true "文章ID"
// @Param content formData string true "评论内容"
// @Param parent_id formData int false "父评论ID（回复时传，默认0）"
// @Success 200 {object} models.Comment
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "文章不存在"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Security BearerAuth
// @Router /articles/api/comments/create [post]
func CommentCreateAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	articleIDStr := c.PostForm("article_id")
	articleID, err := strconv.ParseUint(articleIDStr, 10, 64)
	if err != nil || articleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	content := c.PostForm("content")
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容不能为空"})
		return
	}

	// 图片 URL 列表（JSON 数组字符串或逗号分隔），可选
	images := normalizeImagesField(c.PostForm("images"))

	parentID := uint(0)
	if parentIDStr := c.PostForm("parent_id"); parentIDStr != "" {
		pid, err := strconv.ParseUint(parentIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的父评论ID"})
			return
		}
		parentID = uint(pid)
	}

	// 校验文章存在且未删除
	var article models.Article
	if err := models.ArticleDB.WithContext(ctx).
		Where("is_deleted = ?", 0).
		First(&article, articleID).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": articleID,
			"user_id":    userID,
			"error":      err,
		}).Warn("CommentCreateAPI 文章不存在或已删除")
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在或已删除"})
		return
	}

	// 如果是回复，校验父评论存在且属于同一文章，并保留父评论信息用于发通知 + 确定根评论
	var parentComment models.Comment
	if parentID > 0 {
		if err := models.ArticleDB.WithContext(ctx).
			Where("article_id = ?", articleID).
			First(&parentComment, parentID).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"comment_id": parentID,
				"article_id": articleID,
				"user_id":    userID,
				"error":      err,
			}).Warn("CommentCreateAPI 父评论不存在或不属于该文章")
			c.JSON(http.StatusBadRequest, gin.H{"error": "父评论不存在或不属于该文章"})
			return
		}
	}

	// 确定根评论ID和被回复用户ID（两级平铺：所有回复都直接挂到顶级评论下）
	// - 回复顶级评论：rootID = 父评论ID，replyToUserID = 父评论的UserID（显示 @顶级评论作者）
	// - 回复其他回复：rootID = 父评论的RootID，replyToUserID = 父评论的UserID（显示 @被回复者）
	var rootID, replyToUserID uint
	if parentID > 0 {
		replyToUserID = parentComment.UserID // 不论回复谁，都记录被回复者，前端显示 @xxx
		if parentComment.RootID == 0 {
			// 父评论是顶级评论
			rootID = parentComment.ID
		} else {
			// 父评论本身是回复，继承其 rootID
			rootID = parentComment.RootID
		}
	}

	// 创建评论
	comment := models.Comment{
		ArticleID:     uint(articleID),
		UserID:        userID,
		ParentID:      parentID,
		RootID:        rootID,
		ReplyToUserID: replyToUserID,
		Content:       content,
		Images:        images,
	}
	if err := models.ArticleDB.WithContext(ctx).Create(&comment).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": articleID,
			"user_id":    userID,
			"parent_id":  parentID,
			"error":      err,
		}).Error("CommentCreateAPI 创建评论失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "评论发表失败"})
		return
	}

	// 生成消息通知（不阻断主流程，失败仅记录日志）
	// 1) 文章被评论：通知文章作者（作者自己评论自己的文章不通知）
	// 2) 评论被回复：通知被回复评论的发送者（自己回复自己不通知）
	summary := truncateSummary(content, 50)
	if parentID == 0 {
		// 顶级评论 → 通知文章作者
		if article.UserID != userID {
			notify := models.Notification{
				UserID:    article.UserID,
				ActorID:   userID,
				Type:      models.NotificationTypeComment,
				ArticleID: article.ID,
				CommentID: comment.ID,
				Summary:   summary,
			}
			if err := models.ArticleDB.WithContext(ctx).Create(&notify).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"article_id":  article.ID,
					"receiver_id": article.UserID,
					"actor_id":    userID,
					"error":       err,
				}).Warn("CommentCreateAPI 创建文章评论通知失败")
			} else {
				// WebSocket 推送最新未读通知数给文章作者
				ws.PushNotificationCount(article.UserID)
			}
		}
	} else {
		// 回复评论 → 通知被回复评论的发送者
		if parentComment.UserID != userID {
			notify := models.Notification{
				UserID:    parentComment.UserID,
				ActorID:   userID,
				Type:      models.NotificationTypeReply,
				ArticleID: article.ID,
				CommentID: comment.ID,
				Summary:   summary,
			}
			if err := models.ArticleDB.WithContext(ctx).Create(&notify).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"article_id":  article.ID,
					"receiver_id": parentComment.UserID,
					"actor_id":    userID,
					"error":       err,
				}).Warn("CommentCreateAPI 创建回复通知失败")
			} else {
				// WebSocket 推送最新未读通知数给被回复者
				ws.PushNotificationCount(parentComment.UserID)
			}
		}
	}

	// 填充用户信息后返回
	fillCommentUsers(ctx, []models.Comment{comment})

	c.JSON(http.StatusOK, comment)
}

// CommentDeleteAPI 删除评论
// @Summary 删除评论
// @Description 删除指定评论。权限：评论发送者 / 文章作者。若有子回复则禁止删除。
// @Tags 评论管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param comment_id formData int true "评论ID"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} map[string]string "参数错误或存在子回复"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 404 {object} map[string]string "评论不存在"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Security BearerAuth
// @Router /articles/api/comments/delete [post]
func CommentDeleteAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	commentIDStr := c.PostForm("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 64)
	if err != nil || commentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的评论ID"})
		return
	}

	// 查询评论
	var comment models.Comment
	if err := models.ArticleDB.WithContext(ctx).First(&comment, commentID).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"comment_id": commentID,
			"user_id":    userID,
			"error":      err,
		}).Warn("CommentDeleteAPI 评论不存在")
		c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
		return
	}

	// 校验权限：①评论发送者 ②文章作者
	allowed := false
	// ①评论发送者
	if comment.UserID == userID {
		allowed = true
	}
	// ②文章作者
	if !allowed {
		var article models.Article
		if err := models.ArticleDB.WithContext(ctx).
			Select("id, user_id").
			First(&article, comment.ArticleID).Error; err == nil {
			if article.UserID == userID {
				allowed = true
			}
		} else {
			logger.WithFields(map[string]interface{}{
				"article_id": comment.ArticleID,
				"comment_id": comment.ID,
				"user_id":    userID,
				"error":      err,
			}).Warn("CommentDeleteAPI 查询文章作者失败")
		}
	}

	if !allowed {
		logger.WithFields(map[string]interface{}{
			"comment_id": comment.ID,
			"user_id":    userID,
		}).Warn("CommentDeleteAPI 无删除权限")
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限删除该评论"})
		return
	}

	// 检查是否存在子回复，有则禁止删除
	var childCount int64
	if err := models.ArticleDB.WithContext(ctx).
		Model(&models.Comment{}).
		Where("parent_id = ?", comment.ID).
		Count(&childCount).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"comment_id": comment.ID,
			"user_id":    userID,
			"error":      err,
		}).Error("CommentDeleteAPI 统计子回复失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	if childCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先删除该评论下的回复"})
		return
	}

	// 软删除评论
	if err := models.ArticleDB.WithContext(ctx).Delete(&comment).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"comment_id": comment.ID,
			"user_id":    userID,
			"error":      err,
		}).Error("CommentDeleteAPI 删除评论失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "评论已删除",
	})
}

// fillCommentUsers 批量填充评论的用户名和头像（跨库）
// ctx 用于传递 SkyWalking 链路上下文
func fillCommentUsers(ctx context.Context, comments []models.Comment) {
	if len(comments) == 0 {
		return
	}
	userIDs := make(map[uint]bool)
	for _, c := range comments {
		if c.UserID > 0 {
			userIDs[c.UserID] = true
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
	if err := models.DB.WithContext(ctx).Select("id, user_name, email, avatar").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return
	}
	userMap := make(map[uint]models.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	for i := range comments {
		if u, ok := userMap[comments[i].UserID]; ok {
			comments[i].UserName = u.UserName
			comments[i].Avatar = u.Avatar
		}
	}
}

// fillReplyToUsers 批量填充被回复者用户名（跨库）
// 仅对 reply_to_user_id > 0 的回复填充 reply_to_user_name
// ctx 用于传递 SkyWalking 链路上下文
func fillReplyToUsers(ctx context.Context, comments []models.Comment) {
	if len(comments) == 0 {
		return
	}
	userIDs := make(map[uint]bool)
	for _, c := range comments {
		if c.ReplyToUserID > 0 {
			userIDs[c.ReplyToUserID] = true
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
	if err := models.DB.WithContext(ctx).Select("id, user_name").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return
	}
	userMap := make(map[uint]string, len(users))
	for _, u := range users {
		userMap[u.ID] = u.UserName
	}
	for i := range comments {
		if comments[i].ReplyToUserID > 0 {
			comments[i].ReplyToUserName = userMap[comments[i].ReplyToUserID]
		}
	}
}

// CommentImageUploadAPI 上传评论图片到 MinIO，返回可访问 URL
// @Summary 上传评论图片
// @Description 上传单张评论图片到 MinIO，返回可访问的图片 URL。限制 1MB，支持 jpeg/png/gif/webp。
// @Tags 评论管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "图片文件"
// @Success 200 {object} map[string]string "返回 url"
// @Failure 400 {object} map[string]string "参数错误或文件过大/格式不支持"
// @Failure 500 {object} map[string]string "上传失败"
// @Security BearerAuth
// @Router /articles/api/comments/upload-image [post]
func CommentImageUploadAPI(c *gin.Context) {
	userID := c.GetUint("user_id")

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择要上传的图片"})
		return
	}

	// 大小限制
	if file.Size > commentImageMaxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片大小不能超过1MB"})
		return
	}

	// 格式校验
	contentType := file.Header.Get("Content-Type")
	if !isAllowedImageType(contentType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 jpeg/png/gif/webp 格式图片"})
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取文件失败"})
		return
	}
	defer src.Close()

	// 上传到 MinIO
	url, err := models.UploadCommentImage(userID, src, file.Filename, contentType)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"filename": file.Filename,
			"error":    err,
		}).Error("CommentImageUploadAPI 上传评论图片失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "图片上传失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// isAllowedImageType 判断 MIME 类型是否为允许的图片格式
func isAllowedImageType(ct string) bool {
	for _, t := range strings.Split(allowedImageContentTypes, ",") {
		if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(ct)) {
			return true
		}
	}
	return false
}

// normalizeImagesField 将 images 参数归一化为 JSON 数组字符串
// 接受两种输入：JSON 数组字符串（["url1","url2"]）或逗号分隔字符串（url1,url2）
// 同时校验数量上限，返回标准 JSON 数组字符串（无图片时返回空串）
func normalizeImagesField(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var urls []string
	// 先尝试按 JSON 数组解析
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &urls); err != nil {
			// 解析失败当作普通字符串
			urls = []string{raw}
		}
	} else {
		// 逗号分隔
		for _, u := range strings.Split(raw, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				urls = append(urls, u)
			}
		}
	}

	// 数量限制
	if len(urls) > commentImageMaxCount {
		urls = urls[:commentImageMaxCount]
	}
	if len(urls) == 0 {
		return ""
	}

	b, _ := json.Marshal(urls)
	return string(b)
}

// truncateSummary 截取字符串到指定 rune 数，超出加省略号
func truncateSummary(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}
