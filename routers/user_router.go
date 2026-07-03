package routers

import (
	userhdl "gin-demo/handlers/user"
	"github.com/gin-gonic/gin"
)

// SetupUserRoutes 设置用户相关路由（公开路由）
func SetupUserRoutes(r *gin.RouterGroup) {
	user := r.Group("/user")
	{
		user.GET("/register", userhdl.RegisterPage)
		user.POST("/send-code", userhdl.SendVerificationCode)
		user.POST("/send-login-code", userhdl.SendLoginVerificationCode)
		user.POST("/register", userhdl.Register)
		user.GET("/login", userhdl.LoginPage)
		user.POST("/login", userhdl.Login)
		user.GET("/logout", userhdl.Logout)
		// 重置密码（公开路由，无需登录态）
		user.GET("/reset-password", userhdl.ResetPasswordPage)
		user.POST("/send-reset-code", userhdl.SendResetVerificationCode)
		user.POST("/reset-password", userhdl.ResetPassword)

		// 注册时实时验证接口
		user.GET("/check-username", userhdl.CheckUsernameExists)
		user.GET("/check-email", userhdl.CheckEmailExists)
	}
}

// SetupUserAuthRoutes 设置需登录的用户路由（头像上传等）
func SetupUserAuthRoutes(r *gin.RouterGroup) {
	user := r.Group("/user")
	{
		user.POST("/avatar/upload", userhdl.UploadAvatar)
	}
}
