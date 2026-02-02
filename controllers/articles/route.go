package articles

import (
	"github.com/gin-gonic/gin"

	"gin-demo/session"
)

// RegisterRoutes registers article routes onto the provided group or engine.
func RegisterRoutes(rg *gin.RouterGroup) {
	// list and get are public
	rg.GET("/", ListArticles)
	rg.GET(":id", GetArticle)
	rg.GET("/labels", ListLabels)

	// create, update and delete require auth
	rg.POST("/", session.AuthRequired(), CreateArticle)
	rg.PUT(":id", session.AuthRequired(), UpdateArticle)
	rg.DELETE(":id", session.AuthRequired(), DeleteArticle)
}
