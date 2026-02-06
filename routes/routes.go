package routes

import (
	"github.com/gin-gonic/gin"

	articleController "gin-demo/controllers/articles"
	k8sCtrl "gin-demo/controllers/kubernetes"
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

	// kubernetes routes
	k8s := r.Group("/api/k8s")
	k8sCtrl.RegisterRoutes(k8s)
}
