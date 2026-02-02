package routes

import (
	"github.com/gin-gonic/gin"

	articleController "gin-demo/controllers/articles"
	userCtrl "gin-demo/controllers/users"
)

// Register registers grouped routes onto the provided Gin engine.
func Register(r *gin.Engine) {
	// serve /home page (public)
	r.GET("/home", func(c *gin.Context) { c.File("./static/home.html") })

	users := r.Group("/users")
	userCtrl.RegisterRoutes(users)

	// articles routes
	articleCtrl := r.Group("/articles")
	// Register article handlers (create/delete require auth inside)
	articleController.RegisterRoutes(articleCtrl)
}
