package websocket

import (
	"encoding/json"
	"log"
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
	ID   string
	Conn *websocket.Conn
	Send chan []byte
}

func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		ID:   id,
		Conn: conn,
		Send: make(chan []byte, 256),
	}
}

// SendWelcome 发送欢迎消息，包含用户 ID
func (c *Client) SendWelcome() {
	msg := Message{
		Type:    "welcome",
		From:    "System",
		To:      c.ID,
		Content: "Welcome to the chat room!",
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	}
	jsonMsg, _ := json.Marshal(msg)
	c.Send <- jsonMsg
}

// ReadPump 循环读取客户端发送的消息
func (c *Client) ReadPump() {
	defer func() {
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

		msg.From = c.ID // 确保发送者是当前连接的 ID

		// TODO: 这里应该处理私聊逻辑，目前先全部广播
		out, err := json.Marshal(msg)
		if err != nil {
			log.Printf("json marshal error: %v", err)
			continue
		}
		Manager.Broadcast <- out
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

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

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
