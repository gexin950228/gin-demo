package middleware

import (
	"gin-demo/models"
	"gin-demo/routes"
	"gin-demo/skywalking"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 基于Token的认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从cookie中获取token
		span1 := skywalking.NewSpan(c, "middleware:读取Cookie")
		token, err := c.Cookie("token")
		if span1 != nil { span1.End() }

		if err != nil || token == "" {
			c.Redirect(http.StatusFound, routes.Reverse(routes.UserLoginPage))
			c.Abort()
			return
		}

		// 从Redis验证token并获取用户信息
		span2 := skywalking.NewSpan(c, "redis:验证Token")
		userID, username, err := models.GetToken(token)
		if span2 != nil { span2.End() }

		if err != nil {
			span3 := skywalking.NewSpan(c, "middleware:清除无效Cookie")
			c.SetCookie("token", "", -1, "/", "", false, true)
			if span3 != nil { span3.End() }
			c.Redirect(http.StatusFound, routes.Reverse(routes.UserLoginPage))
			c.Abort()
			return
		}

		// 将用户信息存入context
		c.Set("user_id", userID)
		c.Set("username", username)
		c.Next()
	}
}
