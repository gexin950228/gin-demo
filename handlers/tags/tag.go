package tags

import (
	"gin-demo/logger"
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/skywalking"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetAllTags 获取所有标签（用于下拉选择）
// @Summary 获取所有标签
// @Description 返回系统中所有已创建的标签列表，按名称升序排列
// @Tags 标签管理
// @Produce json
// @Success 200 {object} map[string]interface{} "标签列表"
// @Router /tags/all [get]
func GetAllTags(c *gin.Context) {
	var tags []models.Tag
	if err := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Order("name asc").Find(&tags).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("GetAllTags 查询标签列表失败")
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": tags,
	})
}

// CreateTag 创建新标签（如果已存在则返回已有标签）
// @Summary 创建标签
// @Description 创建新标签。如果同名标签已存在，直接返回已有标签而不重复创建。
// @Tags 标签管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param name formData string true "标签名称"
// @Success 200 {object} map[string]interface{} "创建成功或返回已有标签"
// @Failure 400 {object} map[string]string "参数错误"
// @Security BearerAuth
// @Router /tags/create [post]
func CreateTag(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "标签名称不能为空"})
		return
	}

	var tag models.Tag
	result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Where("name = ?", name).First(&tag)

	if result.RowsAffected > 0 {
		// 标签已存在，直接返回
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": tag})
		return
	}

	tag = models.Tag{Name: name}
	if err := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Create(&tag).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"name":  name,
			"error": err,
		}).Error("CreateTag 创建标签失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "创建标签失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tag})
}

// GetArticleTags 获取某篇文章的标签
// @Summary 获取文章标签
// @Description 返回指定文章关联的所有标签
// @Tags 标签管理
// @Produce json
// @Param id query int true "文章ID"
// @Success 200 {object} map[string]interface{} "标签列表"
// @Failure 400 {object} map[string]string "无效的文章ID"
// @Failure 404 {object} map[string]string "文章不存在"
// @Router /tags/article-tags [get]
func GetArticleTags(c *gin.Context) {
	idStr := c.Query("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效的文章ID"})
		return
	}

	var article models.Article
	result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Preload("Tags").First(&article, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "文章不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": article.Tags,
	})
}

// UpdateArticleTags 更新文章的标签（全量替换：传入新标签列表，自动处理新增/删除）
// @Summary 更新文章标签
// @Description 全量替换指定文章的标签。传入的新标签列表会完全替换旧标签。不存在的标签会自动创建。
// @Tags 标签管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param id formData int true "文章ID"
// @Param tags formData []string true "新的标签名称列表"
// @Success 200 {object} map[string]string "更新成功"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 404 {object} map[string]string "文章不存在"
// @Security BearerAuth
// @Router /tags/update-article-tags [post]
func UpdateArticleTags(c *gin.Context) {
	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效的文章ID"})
		return
	}

	tagNames := c.PostFormArray("tags")

	var article models.Article
	result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).First(&article, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "文章不存在"})
		return
	}

	// 清除旧关联
	models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Model(&article).Association("Tags").Clear()

	// 处理每个标签：查找或创建，然后关联
	for _, name := range tagNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		var tag models.Tag
		dbResult := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Where("name = ?", name).First(&tag)
		if dbResult.RowsAffected == 0 {
			// 不存在则创建
			tag = models.Tag{Name: name}
			if err := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Create(&tag).Error; err != nil {
				logger.WithFields(map[string]interface{}{
					"name":       name,
					"article_id": id,
					"error":      err,
				}).Error("UpdateArticleTags 创建标签失败")
				c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新标签失败"})
				return
			}
		}
		// 建立关联
		models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Model(&article).Association("Tags").Append(&tag)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "标签更新成功",
	})
}

// TagManagePage 显示标签管理页面（仅 user_id=1 可访问）
// @Summary 标签管理页
// @Description 返回标签管理页面，展示所有标签，支持新增、重命名、删除操作。仅管理员(user_id=1)可访问。
// @Tags 标签管理
// @Produce html
// @Param page query int false "页码（默认1）"
// @Success 200 {string} string "标签管理HTML"
// @Failure 403 {string} string "无权限提示"
// @Security BearerAuth
// @Router /tags/manage [get]
func TagManagePage(c *gin.Context) {
	if !IsAdmin(c) {
		c.HTML(http.StatusForbidden, "forbidden.html", gin.H{
			"title":    "无权限",
			"message":  "您没有权限访问此页面，请联系管理员。",
			"url_home": routes.Reverse(routes.Home),
		})
		return
	}

	username := c.GetString("username")
	ctx := skywalking.WithTraceContext(c)
	userID := c.GetUint("user_id")
	avatar := ""
	if userID > 0 {
		var user models.User
		if err := models.DB.Select("avatar").First(&user, userID).Error; err == nil {
			avatar = user.Avatar
		}
	}

	// 分页参数
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	pageSize := 10

	// 总数
	var total int64
	if err := models.ArticleDB.WithContext(ctx).Model(&models.Tag{}).Count(&total).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("TagManagePage 统计标签总数失败")
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	// 页码超出范围时自动跳到最后一页（如删除后当前页已不存在）
	if page > totalPages {
		c.Redirect(http.StatusFound, routes.Reverse(routes.TagManagePage)+"?page="+strconv.Itoa(totalPages))
		return
	}

	// 当前页标签
	var tags []models.Tag
	if err := models.ArticleDB.WithContext(ctx).Order("name asc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&tags).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err,
		}).Error("TagManagePage 查询标签列表失败")
	}

	// 统计每个标签关联的文章数（通过 article_tags 关联表直接计数）
	type TagWithCount struct {
		models.Tag
		ArticleCount int64 `json:"article_count"`
	}
	var tagCounts []TagWithCount
	for _, t := range tags {
		var count int64
		if err := models.ArticleDB.WithContext(ctx).Table("article_tags").
			Where("tag_id = ?", t.ID).Count(&count).Error; err != nil {
			logger.WithFields(map[string]interface{}{
				"tag_id": t.ID,
				"error":  err,
			}).Warn("TagManagePage 统计标签文章数失败")
		}
		tagCounts = append(tagCounts, TagWithCount{Tag: t, ArticleCount: count})
	}

	span2 := skywalking.NewSpan(c, "template:渲染标签管理页")
	c.HTML(http.StatusOK, "tag_manage.html", gin.H{
		"title":       "标签管理",
		"username":    username,
		"avatar_url":  avatar,
		"url_avatar_upload": routes.Reverse(routes.UserAvatarUpload),
		"tags":        tagCounts,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
		"url_home":           routes.Reverse(routes.Home),
		"url_logout":         routes.Reverse(routes.UserLogout),
		"url_article_create": routes.Reverse(routes.ArticleCreatePage),
		"url_article_list":   routes.Reverse(routes.ArticleListPage),
		"url_tag_manage":     routes.Reverse(routes.TagManagePage),
		"url_approval_list":  routes.Reverse(routes.ApprovalListPage),
	})
	if span2 != nil { span2.End() }
}

// RenameTag 重命名标签
// @Summary 重命名标签
// @Description 修改指定标签的名称。新名称不能与已有标签重复。
// @Tags 标签管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param id formData int true "标签ID"
// @Param name formData string true "新的标签名称"
// @Success 200 {object} map[string]string "重命名成功"
// @Failure 400 {object} map[string]string "参数错误或名称已存在"
// @Failure 404 {object} map[string]string "标签不存在"
// @Security BearerAuth
// @Router /tags/rename [post]
func RenameTag(c *gin.Context) {
	if !IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "您没有权限执行此操作，请联系管理员。"})
		return
	}

	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效的标签ID"})
		return
	}

	newName := strings.TrimSpace(c.PostForm("name"))
	if newName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "标签名称不能为空"})
		return
	}

	var tag models.Tag
	if result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).First(&tag, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "标签不存在"})
		return
	}

	// 检查新名称是否与其他标签重复（排除自身）
	var existing models.Tag
	result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Where("name = ? AND id != ?", newName, id).First(&existing)
	if result.RowsAffected > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "标签名称已存在"})
		return
	}

	if err := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Model(&tag).Update("name", newName).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"tag_id":  tag.ID,
			"old_name": tag.Name,
			"new_name": newName,
			"error":   err,
		}).Error("RenameTag 重命名标签失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "重命名失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "重命名成功"})
}

// DeleteTag 删除标签（同时清除文章关联）
// @Summary 删除标签
// @Description 删除指定标签，并自动清除该标签与所有文章的关联关系。
// @Tags 标签管理
// @Accept x-www-form-urlencoded
// @Produce json
// @Param id formData int true "标签ID"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} map[string]string "无效的标签ID"
// @Failure 404 {object} map[string]string "标签不存在"
// @Security BearerAuth
// @Router /tags/delete [post]
func DeleteTag(c *gin.Context) {
	if !IsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "您没有权限执行此操作，请联系管理员。"})
		return
	}

	idStr := c.PostForm("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "无效的标签ID"})
		return
	}

	var tag models.Tag
	if result := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).First(&tag, id); result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "标签不存在"})
		return
	}

	// 先清除文章关联，再删除标签本身
	models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Model(&tag).Association("Articles").Clear()
	if err := models.ArticleDB.WithContext(skywalking.WithTraceContext(c)).Delete(&tag).Error; err != nil {
		logger.WithFields(map[string]interface{}{
			"tag_id": tag.ID,
			"name":   tag.Name,
			"error":  err,
		}).Error("DeleteTag 删除标签失败")
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "删除成功"})
}

// IsAdmin 检查当前用户是否为管理员（user_id=1）
func IsAdmin(c *gin.Context) bool {
	return c.GetUint("user_id") == 1
}
