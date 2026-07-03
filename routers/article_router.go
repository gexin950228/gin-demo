package routers

import (
	"gin-demo/handlers/articles"
	"gin-demo/handlers/tags"
	"github.com/gin-gonic/gin"
)

// SetupArticleRoutes 设置文章相关路由
func SetupArticleRoutes(r *gin.RouterGroup) {
	article := r.Group("/articles")
	{
		article.GET("/list", articles.ArticleListPage)
		article.GET("/api/list", articles.ArticleListAPI)
		article.GET("/create", articles.CreateArticlePage)
		article.POST("/create", articles.CreateArticle)
		article.GET("/view", articles.ViewArticle)
		article.GET("/edit", articles.EditArticlePage)
		article.POST("/edit", articles.UpdateArticle)
		article.POST("/delete", articles.DeleteArticle)

		// 评论 API
		article.GET("/api/comments", articles.CommentListAPI)
		article.POST("/api/comments/create", articles.CommentCreateAPI)
		article.POST("/api/comments/delete", articles.CommentDeleteAPI)
		article.POST("/api/comments/upload-image", articles.CommentImageUploadAPI)

		// 消息通知 API
		article.GET("/api/notifications", articles.NotificationListAPI)
		article.GET("/api/notifications/unread-count", articles.NotificationUnreadCountAPI)
		article.POST("/api/notifications/read", articles.NotificationReadAPI)
	}

	// 消息通知页面
	r.GET("/notifications/list", articles.NotificationListPage)

	// 标签 API（无需认证，供前端调用）
	tag := r.Group("/tags")
	{
		tag.GET("/all", tags.GetAllTags)
		tag.POST("/create", tags.CreateTag)
		tag.GET("/article-tags", tags.GetArticleTags)
		tag.POST("/update-article-tags", tags.UpdateArticleTags)
		tag.GET("/manage", tags.TagManagePage)
		tag.POST("/rename", tags.RenameTag)
		tag.POST("/delete", tags.DeleteTag)
	}
}
