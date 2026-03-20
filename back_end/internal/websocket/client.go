package websocket

import (
	"chat_room/internal/repo"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	//等待时间
	writeWait = 10 * time.Second
	//pong超时时间
	pongWait = 60 * time.Second
	//ping时间间隔
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client 表示一个连接的用户
type Client struct {
	ID     string
	UserID int64
	Name   string
	Avatar string
	Conn   *websocket.Conn
	Send   chan []byte
}

// NewClient 创建一个 WebSocket 客户端连接包装。
func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		ID:   id,
		Conn: conn,
		Send: make(chan []byte, 256),
	}
}

// SendWelcome 发送欢迎消息，包含用户 ID
func (c *Client) SendWelcome() {
	msg := struct {
		Type string `json:"type"`
		To   string `json:"to"`
		Time string `json:"time"`
		User struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Avatar string `json:"avatar"`
		} `json:"user"`
	}{
		Type: "welcome",
		To:   c.ID,
		Time: time.Now().Format("2006-01-02 15:04:05"),
	}
	msg.User.ID = c.UserID
	msg.User.Name = c.Name
	msg.User.Avatar = c.Avatar
	jsonMsg, _ := json.Marshal(msg)
	c.Send <- jsonMsg
}

// ReadPump 循环读取客户端发送的消息
func (c *Client) ReadPump() {
	defer func() {
		// 触发 Manager 清理连接，并关闭底层 socket
		Manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	// 设置 Pong 处理函数，收到 Pong 消息后重置读取超时
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// 解析消息并广播
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("json unmarshal error: %v", err)
			continue
		}

		// 统一由服务端覆盖发送者身份，避免前端伪造
		msg.From = c.ID
		if msg.Avatar == "" {
			msg.Avatar = c.Avatar
		}
		if msg.Time == "" {
			msg.Time = time.Now().Format("15:04:05")
		}

		out, err := json.Marshal(msg)
		if err != nil {
			log.Printf("json marshal error: %v", err)
			continue
		}

		switch msg.Type {
		case "join_room":
			// 将当前连接加入某个房间（仅用于 WS 广播维度）
			Manager.JoinRoom(msg.RoomID, c)
		case "leave_room":
			// 将当前连接移出房间
			Manager.LeaveRoom(msg.RoomID, c)
		case "room_public":
			// 房间公聊：按 roomId 广播
			if msg.RoomID > 0 {
				Manager.BroadcastToRoom(msg.RoomID, out)
				// 存储消息到数据库
				if c.UserID > 0 && Manager.DB != nil {
					ctx := context.Background()
					_, err := repo.CreateMessage(ctx, Manager.DB, msg.RoomID, c.UserID, sql.NullInt64{}, 1, msg.Content)
					if err != nil {
						log.Printf("create message error: %v", err)
					}
				}
			} else {
				Manager.Broadcast <- out
			}
		case "private":
			// 私聊：发送给目标用户（To）并回显给自己
			if msg.To != "" {
				Manager.SendToClientID(msg.To, out)
			}
			Manager.SendToClientID(c.ID, out)
			// 存储私聊消息到数据库
			if c.UserID > 0 && Manager.DB != nil && msg.To != "" {
				ctx := context.Background()
				// 解析接收者用户ID
				var toUserID int64
				if len(msg.To) > 2 && msg.To[:2] == "u_" {
					if id, err := strconv.ParseInt(msg.To[2:], 10, 64); err == nil {
						toUserID = id
					}
				}
				if toUserID > 0 {
					_, err := repo.CreateMessage(ctx, Manager.DB, 0, c.UserID, sql.NullInt64{Int64: toUserID, Valid: true}, 2, msg.Content)
					if err != nil {
						log.Printf("create private message error: %v", err)
					}
				}
			}
		default:
			// 默认按全局广播处理（兼容历史 type：public/private/system）
			Manager.Broadcast <- out
		}
	}
}

// WritePump 循环将消息写入客户端
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 每条消息单独作为一帧发送，避免多个 JSON 被拼接导致前端 JSON.parse 失败
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// 顺带清空队列中已堆积的数据（仍逐条写出），降低延迟
			n := len(c.Send)
			for i := 0; i < n; i++ {
				if err := c.Conn.WriteMessage(websocket.TextMessage, <-c.Send); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
