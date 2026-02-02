package users

import "github.com/gin-gonic/gin"

// RegisterRoutes registers all user-related routes onto the provided RouterGroup.
func RegisterRoutes(users *gin.RouterGroup) {
	// pages for frontend
	users.GET("/to_register", func(c *gin.Context) { c.File("./static/register.html") })
	users.GET("/to_login", func(c *gin.Context) { c.File("./static/login.html") })

	users.POST("/send_code", SendCode)
	users.POST("/verify_code", VerifyCode)
	users.POST("/register", Register)
	users.POST("/login", Login)
	users.POST("/logout", Logout)
}
