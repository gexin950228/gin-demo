package articles

import (
	"fmt"
	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/skywalking"
	"gin-demo/ws"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// HomePage 显示首页
// @Summary 首页
// @Description 返回系统首页，展示用户信息和快捷入口
// @Tags 文章管理
// @Success 200 {string} string "首页HTML"
// @Security BearerAuth
// @Router /home [get]
func HomePage(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	username := c.GetString("username")
	userID := c.GetUint("user_id")
	avatar := ""
	if userID > 0 {
		var user models.User
		if err := models.DB.WithContext(ctx).Select("avatar").First(&user, userID).Error; err == nil {
			avatar = user.Avatar
		}
	}

	c.HTML(http.StatusOK, "home.html", gin.H{
		"title":                "首页",
		"username":             username,
		"avatar_url":           avatar,
		"is_admin":             userID == 1,
		"url_avatar_upload":    routes.Reverse(routes.UserAvatarUpload),
		"url_home":             routes.Reverse(routes.Home),
		"url_logout":           routes.Reverse(routes.UserLogout),
		"url_article_create":   routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":     routes.Reverse(routes.ArticleListPage),
		"url_tag_manage":       routes.Reverse(routes.TagManagePage),
		"url_approval_list":    routes.Reverse(routes.ApprovalListPage),
		"url_notification_list": routes.Reverse(routes.NotificationListPage),
	})
}

// ArticleListAPI 返回文章列表 JSON（供 AJAX 调用，不刷新页面）
func ArticleListAPI(c *gin.Context) {
	userID := c.GetUint("user_id")
	tagFilter := c.Query("tag")
	ctx := skywalking.WithTraceContext(c)

	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 10

	query := models.ArticleDB.WithContext(ctx).Model(&models.Article{}).Where("is_deleted = ?", 0)
	if tagFilter != "" {
		query = query.Joins("JOIN article_tags ON article_tags.article_id = articles.id JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.name = ?", tagFilter)
	}

	// 总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id":   userID,
			"tag_filter": tagFilter,
			"error":     err,
		}).Error("ArticleListAPI 统计文章总数失败")
	}

	// 分页查询
	var articles []models.Article
	if err := query.Preload("Tags").
		Order("created_at desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&articles).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id":   userID,
			"page":      page,
			"error":     err,
		}).Error("ArticleListAPI 查询文章列表失败")
	}

	// 跨库手动填充作者信息（User 表在 gin_project 库，不能用 Preload）
	fillArticlesUsers(articles)

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"articles":    articles,
		"user_id":     userID,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// ArticleListPage 显示文章列表页面（支持标签筛选）
// @Summary 文章列表
// @Description 返回文章列表页面，支持按标签筛选。返回所有文章及其关联的作者和标签信息。
// @Tags 文章管理
// @Produce json
// @Param tag query string false "按标签名称筛选"
// @Param page query int false "页码（默认1）"
// @Success 200 {string} string "文章列表HTML"
// @Security BearerAuth
// @Router /articles/list [get]
func ArticleListPage(c *gin.Context) {
	username := c.GetString("username")
	userID := c.GetUint("user_id")
	tagFilter := c.Query("tag")
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
	pageSize := 10

	query := models.ArticleDB.WithContext(ctx).Model(&models.Article{}).Where("is_deleted = ?", 0)
	if tagFilter != "" {
		query = query.Joins("JOIN article_tags ON article_tags.article_id = articles.id JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.name = ?", tagFilter)
	}

	// 总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"tag_filter": tagFilter,
			"error":      err,
		}).Error("ArticleListPage 统计文章总数失败")
	}

	// 分页查询
	var articles []models.Article
	if err := query.Preload("Tags").
		Order("created_at desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&articles).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"tag_filter": tagFilter,
			"page":       page,
			"error":      err,
		}).Error("ArticleListPage 查询文章列表失败")
	}

	// 跨库手动填充作者信息（User 表在 gin_project 库，不能用 Preload）
	fillArticlesUsers(articles)

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	// 获取所有标签用于筛选下拉框
	var allTags []models.Tag
	if err := models.ArticleDB.WithContext(ctx).Order("name asc").Find(&allTags).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("ArticleListPage 查询所有标签失败")
	}

	span3 := skywalking.NewSpan(c, "template:渲染文章列表页")
	c.HTML(http.StatusOK, "article_list.html", gin.H{
		"title":                "文章列表",
		"username":             username,
		"avatar_url":           avatar,
		"url_avatar_upload":    routes.Reverse(routes.UserAvatarUpload),
		"user_id":              userID,
		"articles":             articles,
		"tags":                 allTags,
		"tag_filter":           tagFilter,
		"page":                 page,
		"page_size":            pageSize,
		"total":                total,
		"total_pages":          totalPages,
		"url_home":             routes.Reverse(routes.Home),
		"url_logout":           routes.Reverse(routes.UserLogout),
		"url_article_create":   routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":     routes.Reverse(routes.ArticleListPage),
		"url_article_list_api": routes.Reverse(routes.ArticleListAPI),
		"url_article_view":     routes.Reverse(routes.ArticleView),
		"url_article_edit":     routes.Reverse(routes.ArticleEditPage),
		"url_article_delete":   routes.Reverse(routes.ArticleDelete),
		"url_tag_manage":       routes.Reverse(routes.TagManagePage),
		"url_approval_list":    routes.Reverse(routes.ApprovalListPage),
	})
	if span3 != nil { span3.End() }
}

// CreateArticlePage 显示创建文章页面
// @Summary 新增文章页面
// @Description 返回新增文章表单页面，包含标题、内容输入和标签选择功能
// @Tags 文章管理
// @Success 200 {string} string "新增文章HTML"
// @Security BearerAuth
// @Router /articles/create [get]
func CreateArticlePage(c *gin.Context) {
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

	var allTags []models.Tag
	if err := models.ArticleDB.WithContext(ctx).Order("name asc").Find(&allTags).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("CreateArticlePage 查询标签失败")
	}

	preTag := c.Query("pre_tag") // 从列表页传来的预选标签

	c.HTML(http.StatusOK, "create_article.html", gin.H{
		"title":              "新增文章",
		"username":           username,
		"avatar_url":         avatar,
		"url_avatar_upload":  routes.Reverse(routes.UserAvatarUpload),
		"tags":               allTags,
		"pre_tag":            preTag,
		"url_home":           routes.Reverse(routes.Home),
		"url_logout":         routes.Reverse(routes.UserLogout),
		"url_article_create": routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":   routes.Reverse(routes.ArticleListPage),
		"url_tag_manage":     routes.Reverse(routes.TagManagePage),
		"url_approval_list":  routes.Reverse(routes.ApprovalListPage),
	})
}

// CreateArticle 创建文章
// @Summary 创建文章
// @Description 创建新文章，支持同时设置标签（逗号分隔或数组形式传入）
// @Tags 文章管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param title formData string true "文章标题"
// @Param content formData string true "文章内容"
// @Param tags formData []string false "标签名称列表（可选）"
// @Success 302 {string} string "重定向到文章列表"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Security BearerAuth
// @Router /articles/create [post]
func CreateArticle(c *gin.Context) {
	title := c.PostForm("title")
	content := c.PostForm("content")

	if title == "" || content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题和内容不能为空"})
		return
	}

	userID := c.GetUint("user_id")

	article := models.Article{
		Title:   title,
		Content: content,
		UserID:  userID,
	}

	if result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Create(&article); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文章创建失败"})
		return
	}

	// 处理标签
	saveArticleTags(&article, c.PostFormArray("tags"), c)

	c.Redirect(http.StatusFound, routes.Reverse(routes.ArticleListPage))
}

// EditArticlePage 显示编辑文章页面
// @Summary 编辑文章页面
// @Description 返回指定文章的编辑页面，预填充标题、内容和当前标签
// @Tags 文章管理
// @Produce json
// @Param id query int true "文章ID"
// @Success 200 {string} string "编辑文章HTML"
// @Failure 400 {object} map[string]string "无效的文章ID"
// @Failure 404 {object} map[string]string "文章不存在"
// @Security BearerAuth
// @Router /articles/edit [get]
func EditArticlePage(c *gin.Context) {
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

	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	var article models.Article
	if result := models.ArticleDB.WithContext(ctx).Preload("Tags").Where("is_deleted = ?", 0).First(&article, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	var allTags []models.Tag
	if err := models.ArticleDB.WithContext(ctx).Order("name asc").Find(&allTags).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("EditArticlePage 查询标签失败")
	}

	span3 := skywalking.NewSpan(c, "template:渲染编辑文章页")
	c.HTML(http.StatusOK, "edit_article.html", gin.H{
		"title":              "编辑文章",
		"username":           username,
		"avatar_url":         avatar,
		"url_avatar_upload":  routes.Reverse(routes.UserAvatarUpload),
		"article":            article,
		"tags":               allTags,
		"is_author":          c.GetUint("user_id") == article.UserID,
		"url_home":           routes.Reverse(routes.Home),
		"url_logout":         routes.Reverse(routes.UserLogout),
		"url_article_create": routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":   routes.Reverse(routes.ArticleListPage),
		"url_article_view":   routes.Reverse(routes.ArticleView, "id", idStr),
		"url_article_edit":   routes.Reverse(routes.ArticleEditPage, "id", idStr),
		"url_tag_manage":     routes.Reverse(routes.TagManagePage),
		"url_approval_list":  routes.Reverse(routes.ApprovalListPage),
	})
	if span3 != nil { span3.End() }
}

// UpdateArticle 更新文章
// @Summary 更新文章
// @Description 更新指定文章的标题、内容和标签。支持全量替换标签。
// @Tags 文章管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param id formData int true "文章ID"
// @Param title formData string true "文章标题"
// @Param content formData string true "文章内容"
// @Param tags formData []string false "标签名称列表（全量替换）"
// @Success 302 {string} string "重定向到文章详情页"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "文章不存在"
// @Security BearerAuth
// @Router /articles/edit [post]
func UpdateArticle(c *gin.Context) {
	// 优先从 POST 表单获取，fallback 到 URL 查询参数
	idStr := c.PostForm("id")
	if idStr == "" {
		idStr = c.Query("id")
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	title := c.PostForm("title")
	content := c.PostForm("content")

	if title == "" || content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标题和内容不能为空"})
		return
	}

	var article models.Article
	ctx := skywalking.WithTraceContext(c)
	if result := models.ArticleDB.WithContext(ctx).Where("is_deleted = ?", 0).First(&article, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	editorID := c.GetUint("user_id")
	tagNames := c.PostFormArray("tags")

	// 获取当前文章的标签，作为原文快照的一部分
	var originalArticle models.Article
	models.ArticleDB.WithContext(ctx).Preload("Tags").First(&originalArticle, article.ID)
	var originalTagNames []string
	for _, t := range originalArticle.Tags {
		originalTagNames = append(originalTagNames, t.Name)
	}
	originalTagsStr := strings.Join(originalTagNames, ",")
	originalTitle := originalArticle.Title
	originalContent := originalArticle.Content

	// 非作者编辑他人文章 → 走审批流程
	if editorID != article.UserID {
		// 合并：把同一篇文章已有的待审批记录更新为最新内容（覆盖旧提交）
		approval := models.ArticleEditApproval{
			ArticleID:       article.ID,
			EditorID:        editorID,
			AuthorID:        article.UserID,
			Title:           title,
			Content:         content,
			Tags:            strings.Join(tagNames, ","),
			OriginalTitle:   originalTitle,
			OriginalContent: originalContent,
			OriginalTags:    originalTagsStr,
			Status:          models.ApprovalStatusPending,
		}

		// 查找是否已有该编辑者对该文章的待审批记录
		var existing models.ArticleEditApproval
		result := models.ArticleDB.WithContext(ctx).
			Where("article_id = ? AND editor_id = ? AND status = ?",
				article.ID, editorID, models.ApprovalStatusPending).
			First(&existing)

		if result.Error == nil {
			// 已有待审批记录 → 更新为最新内容（同时更新原文快照，保证一致性）
			if err := models.ArticleDB.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
				"title":            title,
				"content":          content,
				"tags":             strings.Join(tagNames, ","),
				"original_title":   originalTitle,
				"original_content": originalContent,
				"original_tags":    originalTagsStr,
			}).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"approval_id": existing.ID,
					"article_id":  article.ID,
					"error":       err,
				}).Error("合并待审批记录失败")
			}
		} else {
			// 新建审批记录
			if err := models.ArticleDB.WithContext(ctx).Create(&approval).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"article_id": article.ID,
					"editor_id":  editorID,
					"error":      err,
				}).Error("创建审批记录失败")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "提交审批失败"})
				return
			}
		}

		// 发送邮件通知作者
		go func() {
			var author models.User
			if err := models.DB.First(&author, article.UserID).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"user_id": article.UserID,
					"error":   err,
				}).Warn("审批通知邮件时查询作者失败")
				return
			}
			if author.Email == "" {
				return
			}
			var editor models.User
			if err := models.DB.First(&editor, editorID).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"editor_id": editorID,
					"error":     err,
				}).Warn("审批通知邮件时查询编辑者失败")
				return
			}
			editorName := editor.UserName
			subject := "【Gin Demo】您有一篇文章编辑待审批"
			body := fmt.Sprintf(`
				<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
					<h2 style="color: #333;">文章编辑待审批</h2>
					<p>用户 <strong>%s</strong> 编辑了您的文章《<strong>%s</strong>》，请您登录后前往"我的审批"查看并处理。</p>
					<p style="color: #666;">提交的新标题：%s</p>
					<p style="color: #999; font-size: 12px;">此邮件由系统自动发送，请勿回复。</p>
				</div>
			`, editorName, article.Title, title)
			if err := models.SendEmail(author.Email, subject, body); err != nil {
				logger.WithFields(map[string]interface{}{
					"to":    author.Email,
					"error": err,
				}).Error("审批通知邮件发送失败")
			}
	}()

		// WebSocket 推送最新待审批数量给文章作者
		ws.PushApprovalCount(article.UserID)

		// 重定向到文章详情页（显示原内容，附带提示）
		c.Redirect(http.StatusFound, routes.Reverse(routes.ArticleView, "id", idStr))
		return
	}

	// 作者本人编辑 → 直接更新
	if err := models.ArticleDB.WithContext(ctx).Model(&article).Updates(models.Article{Title: title, Content: content}).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": article.ID,
			"error":      err,
		}).Error("更新文章失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	// 处理标签
	saveArticleTags(&article, tagNames, c)

	c.Redirect(http.StatusFound, routes.Reverse(routes.ArticleView, "id", idStr))
}

// ViewArticle 查看文章详情
// @Summary 文章详情
// @Description 返回指定文章的完整详情，包括标题、内容、作者和标签信息
// @Tags 文章管理
// @Produce json
// @Param id query int true "文章ID"
// @Success 200 {string} string "文章详情HTML"
// @Failure 400 {object} map[string]string "无效的文章ID"
// @Failure 404 {object} map[string]string "文章不存在"
// @Security BearerAuth
// @Router /articles/view [get]
func ViewArticle(c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	username := c.GetString("username")
	userID := c.GetUint("user_id")
	avatar := ""
	if userID > 0 {
		var user models.User
		if err := models.DB.WithContext(ctx).Select("avatar").First(&user, userID).Error; err == nil {
			avatar = user.Avatar
		}
	}

	id := c.Query("id")
	articleID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	var article models.Article
	if result := models.ArticleDB.WithContext(ctx).Preload("Tags").Where("is_deleted = ?", 0).First(&article, articleID); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在"})
		return
	}

	// 跨库手动填充作者信息（User 表在 gin_project 库，不能用 Preload）
	fillArticleUser(&article)

	span2 := skywalking.NewSpan(c, "template:渲染文章详情页")
	c.HTML(http.StatusOK, "view_article.html", gin.H{
		"title":              article.Title,
		"username":           username,
		"avatar_url":         avatar,
		"url_avatar_upload":  routes.Reverse(routes.UserAvatarUpload),
		"article":            article,
		"user_id":            userID,
		"url_comment_list":   routes.Reverse(routes.CommentList),
		"url_comment_create": routes.Reverse(routes.CommentCreate),
		"url_comment_delete": routes.Reverse(routes.CommentDelete),
		"url_comment_image_upload": routes.Reverse(routes.CommentImageUpload),
		"url_home":           routes.Reverse(routes.Home),
		"url_logout":         routes.Reverse(routes.UserLogout),
		"url_article_view":   routes.Reverse(routes.ArticleView, "id", id),
		"url_article_list":   routes.Reverse(routes.ArticleListPage),
		"url_article_create": routes.Reverse(routes.ArticleCreatePage),
		"url_article_edit":   routes.Reverse(routes.ArticleEditPage, "id", id),
		"url_tag_manage":     routes.Reverse(routes.TagManagePage),
		"url_approval_list":  routes.Reverse(routes.ApprovalListPage),
	})
	if span2 != nil { span2.End() }
}

// DeleteArticle 软删除文章（仅作者本人可操作，设置 is_deleted=1）
// @Summary 删除文章
// @Description 软删除指定文章（仅作者本人可操作），将 is_deleted 设为 1
// @Tags 文章管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param id formData int true "文章ID"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 403 {object} map[string]string "无权限"
// @Failure 404 {object} map[string]string "文章不存在"
// @Security BearerAuth
// @Router /articles/delete [post]
func DeleteArticle(c *gin.Context) {
	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的文章ID"})
		return
	}

	var article models.Article
	ctx := skywalking.WithTraceContext(c)
	if result := models.ArticleDB.WithContext(ctx).Where("is_deleted = ?", 0).First(&article, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文章不存在或已删除"})
		return
	}

	// 只有作者本人可以删除
	if article.UserID != c.GetUint("user_id") {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能删除自己的文章"})
		return
	}

	if err := models.ArticleDB.WithContext(ctx).Model(&article).Update("is_deleted", true).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": article.ID,
			"error":      err,
		}).Error("软删除文章失败")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "文章已删除",
	})
}

// saveArticleTags 保存文章标签（内部辅助函数：查找或创建标签，建立关联）
// 直接操作关联表 article_tags，避免 GORM Association 在某些场景下返回 nil 导致 panic
func saveArticleTags(article *models.Article, tagNames []string, c *gin.Context) {
	ctx := skywalking.WithTraceContext(c)
	// 1. 清除旧关联
	if err := models.ArticleDB.WithContext(ctx).Exec("DELETE FROM article_tags WHERE article_id = ?", article.ID).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"article_id": article.ID,
			"error":      err,
		}).Warn("清除文章旧标签关联失败")
	}

	// 2. 重新建立关联
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var tag models.Tag
		result := models.ArticleDB.WithContext(ctx).Where("name = ?", name).First(&tag)
		if result.RowsAffected == 0 {
			tag = models.Tag{Name: name}
			if err := models.ArticleDB.WithContext(ctx).Create(&tag).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"tag":   name,
					"error": err,
				}).Warn("创建标签失败")
				continue
			}
		}
		if err := models.ArticleDB.WithContext(ctx).Exec("INSERT INTO article_tags (article_id, tag_id) VALUES (?, ?)", article.ID, tag.ID).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"article_id": article.ID,
				"tag":        name,
				"error":      err,
			}).Warn("关联标签失败")
		}
	}
}

// fillArticleUser 手动填充单个文章的作者信息（跨库：User 表在 gin_project 库）
func fillArticleUser(article *models.Article) {
	if article.UserID == 0 {
		return
	}
	var user models.User
	if err := models.DB.Select("id, user_name, email, avatar").First(&user, article.UserID).Error; err == nil {
		article.User = user
	}
}

// fillArticlesUsers 批量填充文章列表的作者信息（跨库，避免 N+1）
func fillArticlesUsers(articles []models.Article) {
	if len(articles) == 0 {
		return
	}
	userIDs := make(map[uint]bool)
	for _, a := range articles {
		if a.UserID > 0 {
			userIDs[a.UserID] = true
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
	if err := models.DB.Select("id, user_name, email, avatar").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return
	}
	userMap := make(map[uint]models.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}
	for i := range articles {
		if u, ok := userMap[articles[i].UserID]; ok {
			articles[i].User = u
		}
	}
}
