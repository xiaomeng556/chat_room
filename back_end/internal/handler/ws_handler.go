package handler

import (
	"chat_room/internal/websocket"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 允许跨域
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler 处理 WebSocket 连接请求
func WSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// 简单生成一个 ID，实际应该从 JWT 或 Session 中获取
	clientID := generateUniqueClientID()
	client := websocket.NewClient(clientID, conn)

	// 先启动写泵，确保后续发送的 welcome 和用户列表可以被及时写出
	go client.WritePump()
	// 再注册到管理器（会触发广播用户列表）
	websocket.Manager.Register <- client
	// 发送 Welcome 消息，告知客户端自己的 ID
	client.SendWelcome()
	log.Printf("欢迎用户 %s 加入聊天室", client.ID)
	// 最后启动读泵读取客户端消息
	go client.ReadPump()
}

// 生成唯一客户端ID（纳秒时间戳 + 随机字节）
func generateUniqueClientID() string {
	ts := time.Now().Format("150405.000")
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("user_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("user_%s_%s", ts, hex.EncodeToString(buf))
}
