package ws

import (
	"encoding/json"
	"sync"

	"gin-demo/logger"

	"github.com/gorilla/websocket"
)

// Client 单个 WebSocket 连接
type Client struct {
	userID uint
	conn   *websocket.Conn
	send   chan []byte
}

// Hub 连接池，按 userID 维护所有活跃连接
type Hub struct {
	mu         sync.RWMutex
	clients    map[uint]map[*Client]bool // userID -> 连接集合
	register   chan *Client
	unregister chan *Client
}

var hub = &Hub{
	clients:    make(map[uint]map[*Client]bool),
	register:   make(chan *Client),
	unregister: make(chan *Client),
}

// Start 启动 hub 事件循环（在 main 中调用一次）
func Start() {
	go hub.run()
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.userID] == nil {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			count := len(h.clients[client.userID])
			h.mu.Unlock()
			logger.WithFields(map[string]interface{}{
				"user_id":      client.userID,
				"conns_of_user": count,
			}).Info("WebSocket 客户端已连接")

		case client := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.clients[client.userID]; ok {
				if _, exists := conns[client]; exists {
					delete(conns, client)
					close(client.send)
					if len(conns) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// PushToUser 向指定用户的所有活跃连接推送一条 JSON 消息
// 如果用户不在线则静默跳过
func PushToUser(userID uint, payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err,
		}).Warn("WebSocket 序列化推送消息失败")
		return
	}

	hub.mu.RLock()
	conns := hub.clients[userID]
	clients := make([]*Client, 0, len(conns))
	for c := range conns {
		clients = append(clients, c)
	}
	hub.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- data:
		default:
			// 发送通道已满，说明客户端消费慢或断开，关闭该连接
			logger.WithFields(map[string]interface{}{
				"user_id": userID,
			}).Warn("WebSocket 发送通道已满，断开连接")
			hub.unregister <- c
		}
	}
}

// writePump 负责把 send 通道中的消息写入底层连接
func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// readPump 持续读取客户端消息（本场景客户端不主动发消息，仅用于检测断开）
func (c *Client) readPump() {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
