package ws

import (
	"context"
	"net/http"

	"gin-demo/logger"
	"gin-demo/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// 允许所有来源（同源时浏览器会自动携带 cookie）
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWS WebSocket 入口：从 cookie 认证用户，升级连接，注册到 hub，推送初始数据
func HandleWS(c *gin.Context) {
	token, err := c.Cookie("token")
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	userID, _, err := models.GetToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("WebSocket 升级失败")
		return
	}

	client := &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, 32),
	}
	hub.register <- client

	// 连接建立后推送一次当前审批未审批数 + 未读通知数
	pushInitialCounts(userID)

	go client.writePump()
	go client.readPump()
}

// pushInitialCounts 推送连接建立时的初始数量（审批未审批数 + 未读通知数）
func pushInitialCounts(userID uint) {
	ctx := context.Background()

	if count, err := models.GetPendingApprovalCount(ctx, userID); err == nil {
		PushToUser(userID, map[string]interface{}{
			"type":  "approval_count",
			"count": count,
		})
	}

	if count, err := models.GetUnreadNotificationCount(ctx, userID); err == nil {
		PushToUser(userID, map[string]interface{}{
			"type":  "notification_count",
			"count": count,
		})
	}
}

// PushApprovalCount 向指定用户推送当前待审批数量
func PushApprovalCount(userID uint) {
	ctx := context.Background()
	count, err := models.GetPendingApprovalCount(ctx, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("WebSocket 推送审批数量时查询失败")
		return
	}
	PushToUser(userID, map[string]interface{}{
		"type":  "approval_count",
		"count": count,
	})
}

// PushNotificationCount 向指定用户推送当前未读通知数量
func PushNotificationCount(userID uint) {
	ctx := context.Background()
	count, err := models.GetUnreadNotificationCount(ctx, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("WebSocket 推送通知数量时查询失败")
		return
	}
	PushToUser(userID, map[string]interface{}{
		"type":  "notification_count",
		"count": count,
	})
}
