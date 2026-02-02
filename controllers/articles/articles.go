package articles

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"gin-demo/models"
)

// CreateArticle handles POST /articles
func CreateArticle(c *gin.Context) {
	// require user in context (set by session.AuthRequired middleware)
	userI, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	username, _ := userI.(string)

	type req struct {
		Title string   `json:"title" binding:"required"`
		Body  string   `json:"body" binding:"required"`
		Tags  []string `json:"tags"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		logrus.Warnf("articles: create bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	a, err := models.CreateArticle(r.Title, r.Body, username, r.Tags)
	if err != nil {
		logrus.Errorf("articles: create failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": a.ID, "created_at": a.PublishedAt})
}

// DeleteArticle handles DELETE /articles/:id
func DeleteArticle(c *gin.Context) {
	userI, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	username, _ := userI.(string)

	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := models.DeleteArticle(uint(id64), username); err != nil {
		logrus.Warnf("articles: delete failed id=%v user=%s err=%v", id64, username, err)
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden or not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListArticles handles GET /articles?page=1&limit=10&tag=name
func ListArticles(c *gin.Context) {
	page := 1
	limit := 10
	if p := c.Query("page"); p != "" {
		if pi, err := strconv.Atoi(p); err == nil && pi > 0 {
			page = pi
		}
	}
	if l := c.Query("limit"); l != "" {
		if li, err := strconv.Atoi(l); err == nil && li > 0 && li <= 100 {
			limit = li
		}
	}
	tag := c.Query("tag")
	offset := (page - 1) * limit
	as, err := models.ListArticles(offset, limit, tag)
	if err != nil {
		logrus.Errorf("articles: list failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal"})
		return
	}

	// get total count
	var total int64
	query := models.DB.Model(&models.Article{}).Where("id_deleted = ?", false)
	if tag != "" {
		query = query.Joins("JOIN article_labels al ON al.article_id = articles.id").Joins("JOIN labels l ON l.id = al.label_id").Where("l.name = ?", tag)
	}
	query.Count(&total)

	// return minimal fields including tags
	type item struct {
		ID          uint      `json:"id"`
		Title       string    `json:"title"`
		Author      string    `json:"author"`
		PublishedAt time.Time `json:"published_at"`
		Tags        []string  `json:"tags"`
	}
	res := make([]item, 0, len(as))
	for _, a := range as {
		var tags []string
		for _, t := range a.Tags {
			tags = append(tags, t.Name)
		}
		res = append(res, item{ID: a.ID, Title: a.Title, Author: a.Author, PublishedAt: a.PublishedAt, Tags: tags})
	}
	c.JSON(http.StatusOK, gin.H{"articles": res, "page": page, "limit": limit, "tag": tag, "total": total})
}

// UpdateArticle handles PUT /articles/:id
func UpdateArticle(c *gin.Context) {
	userI, ok := c.Get("user")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	username, _ := userI.(string)

	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	type req struct {
		Title string   `json:"title" binding:"required"`
		Body  string   `json:"body" binding:"required"`
		Tags  []string `json:"tags"`
	}
	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		logrus.Warnf("articles: update bad request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := models.UpdateArticle(uint(id64), r.Title, r.Body, username, r.Tags); err != nil {
		logrus.Warnf("articles: update failed id=%v user=%s err=%v", id64, username, err)
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden or not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// GetArticle handles GET /articles/:id
func GetArticle(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := models.GetArticle(uint(id64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	fmt.Printf("tags: %v\n", a.Tags)
	c.JSON(http.StatusOK, gin.H{"id": a.ID, "title": a.Title, "body": a.Body, "author": a.Author, "published_at": a.PublishedAt, "tags": func() []string {
		var t []string
		for _, x := range a.Tags {
			t = append(t, x.Name)
		}
		return t
	}()})
}
