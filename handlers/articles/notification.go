package articles

import (
	"net/http"
	"strconv"

	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/skywalking"

	"github.com/gin-gonic/gin"
)

// NotificationListPage 消息通知列表页
// @Summary 消息通知页
// @Description 展示当前用户的所有消息通知（评论/回复提醒），支持分页，点击单条跳转对应文章并定位评论
// @Tags 消息通知
// @Produce html
// @Param page query int false "页码（默认1）"
// @Success 200 {string} string "消息通知页HTML"
// @Security BearerAuth
// @Router /notifications/list [get]
func NotificationListPage(c *gin.Context) {
	username := c.GetString("username")
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)
	avatar := ""
	if userID > 0 {
		var user models.User
		if err := models.DB.WithContext(ctx).Select("avatar").First(&user, userID).Error; err == nil {
			avatar = user.Avatar
		}
	}

	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 15

	// 查询当前用户的通知（按时间倒序）
	query := models.ArticleDB.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Error("NotificationListPage 统计通知总数失败")
	}

	var notifications []models.Notification
	if err := query.
		Order("created_at desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&notifications).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"page":    page,
			"error":   err,
		}).Error("NotificationListPage 查询通知列表失败")
	}

	// 跨库批量填充触发者信息和文章标题
	fillNotificationActors(notifications)

	// 用户打开消息页即标记全部未读为已读（红点下次轮询自动消失）
	// 注意：在查询之后执行，保证本页仍能展示未读高亮样式
	if err := models.ArticleDB.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Update("is_read", 1).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("NotificationListPage 批量标记已读失败")
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	c.HTML(http.StatusOK, "notification_list.html", gin.H{
		"title":              "我的消息",
		"username":           username,
		"avatar_url":         avatar,
		"url_avatar_upload":  routes.Reverse(routes.UserAvatarUpload),
		"notifications":      notifications,
		"page":               page,
		"page_size":          pageSize,
		"total":              total,
		"total_pages":        totalPages,
		"url_home":           routes.Reverse(routes.Home),
		"url_logout":         routes.Reverse(routes.UserLogout),
		"url_article_view":   routes.Reverse(routes.ArticleView),
		"url_article_create": routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":   routes.Reverse(routes.ArticleListPage),
		"url_tag_manage":     routes.Reverse(routes.TagManagePage),
		"url_approval_list":  routes.Reverse(routes.ApprovalListPage),
		"url_notification_list": routes.Reverse(routes.NotificationListPage),
		"url_notification_read": routes.Reverse(routes.NotificationRead),
	})
}

// NotificationListAPI 获取当前用户的通知列表（最新的在前，默认返回最近 20 条）
// @Summary 通知列表
// @Description 获取当前登录用户的消息通知列表（评论/回复提醒），按时间倒序
// @Tags 消息通知
// @Produce json
// @Param limit query int false "返回条数（默认20）"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /articles/api/notifications [get]
func NotificationListAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit < 1 || limit > 50 {
		limit = 20
	}

	var notifications []models.Notification
	if err := models.ArticleDB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Find(&notifications).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("NotificationListAPI 查询通知列表失败")
		c.JSON(http.StatusOK, gin.H{"notifications": []interface{}{}, "count": 0})
		return
	}

	// 跨库批量填充触发者信息和文章标题
	fillNotificationActors(notifications)

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"count":         len(notifications),
	})
}

// NotificationUnreadCountAPI 获取当前用户的未读通知数量
// @Summary 未读通知数
// @Description 返回当前登录用户的未读消息通知数量，供前端轮询展示红点
// @Tags 消息通知
// @Produce json
// @Success 200 {object} map[string]int
// @Security BearerAuth
// @Router /articles/api/notifications/unread-count [get]
func NotificationUnreadCountAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	var count int64
	if err := models.ArticleDB.WithContext(ctx).
		Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, 0).
		Count(&count).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("NotificationUnreadCountAPI 统计未读通知失败")
		c.JSON(http.StatusOK, gin.H{"count": 0})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": count})
}

// NotificationReadAPI 将指定通知标记为已读（通过 id 或 all=1 全部已读）
// @Summary 标记通知已读
// @Description 将指定通知标记为已读。传 id 标记单条；传 all=1 标记当前用户全部未读为已读。
// @Tags 消息通知
// @Accept x-www-form-urlencoded
// @Produce json
// @Param id formData int false "通知ID（单条已读时传）"
// @Param all formData int false "是否全部已读（1=全部已读）"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string "通知不存在"
// @Security BearerAuth
// @Router /articles/api/notifications/read [post]
func NotificationReadAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	ctx := skywalking.WithTraceContext(c)

	all := c.PostForm("all")
	if all == "1" {
		// 全部已读
		if err := models.ArticleDB.WithContext(ctx).
			Model(&models.Notification{}).
			Where("user_id = ? AND is_read = ?", userID, 0).
			Update("is_read", 1).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"user_id": userID,
				"error":   err,
			}).Error("NotificationReadAPI 全部标记已读失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"msg": "已全部标记为已读"})
		return
	}

	// 单条已读
	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的通知ID"})
		return
	}

	// 校验通知属于当前用户
	var n models.Notification
	if err := models.ArticleDB.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&n, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通知不存在"})
		return
	}

	if n.IsRead == 0 {
		if err := models.ArticleDB.WithContext(ctx).
			Model(&n).
			Update("is_read", 1).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"notification_id": id,
				"user_id":         userID,
				"error":           err,
			}).Error("NotificationReadAPI 标记已读失败")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"msg": "已标记为已读"})
}

// fillNotificationActors 批量填充通知的触发者用户名/头像 和 文章标题（跨库）
func fillNotificationActors(notifications []models.Notification) {
	if len(notifications) == 0 {
		return
	}

	// 收集 actorID 和 articleID
	actorIDs := make(map[uint]bool)
	articleIDs := make(map[uint]bool)
	for _, n := range notifications {
		if n.ActorID > 0 {
			actorIDs[n.ActorID] = true
		}
		if n.ArticleID > 0 {
			articleIDs[n.ArticleID] = true
		}
	}

	// 批量查 users（gin_project 库）
	userMap := make(map[uint]models.User)
	if len(actorIDs) > 0 {
		ids := make([]uint, 0, len(actorIDs))
		for id := range actorIDs {
			ids = append(ids, id)
		}
		var users []models.User
		if err := models.DB.Select("id, user_name, avatar").Where("id IN ?", ids).Find(&users).Error; err == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}

	// 批量查 articles（articles 库）
	articleMap := make(map[uint]models.Article)
	if len(articleIDs) > 0 {
		ids := make([]uint, 0, len(articleIDs))
		for id := range articleIDs {
			ids = append(ids, id)
		}
		var arts []models.Article
		if err := models.ArticleDB.Select("id, title").Where("id IN ?", ids).Find(&arts).Error; err == nil {
			for _, a := range arts {
				articleMap[a.ID] = a
			}
		}
	}

	// 填充
	for i := range notifications {
		if u, ok := userMap[notifications[i].ActorID]; ok {
			notifications[i].ActorName = u.UserName
			notifications[i].ActorAvatar = u.Avatar
		}
		if a, ok := articleMap[notifications[i].ArticleID]; ok {
			notifications[i].ArticleTitle = a.Title
		}
	}
}
