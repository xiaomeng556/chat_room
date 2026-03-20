package handler

import (
	"chat_room/internal/websocket"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"

	"chat_room/internal/repo"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CheckOrigin 允许跨域升级（本地开发方便联调）。
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler 处理 WebSocket 连接请求，并在建连时完成用户身份识别：
// - token 有效：绑定到真实用户（client.UserID/Name/Avatar）
// - token 缺失或无效：按游客处理（使用临时 clientID）
func (h *Handler) WSHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	clientID := generateUniqueClientID()
	var user repo.User

	// 支持两种 token 传递方式：URL query 或 Authorization: Bearer
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = extractBearerToken(c.GetHeader("Authorization"))
	}
	if token != "" {
		if claims, err := h.JWT.ParseToken(token); err == nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			if u, err := repo.GetUserByID(ctx, h.DB, claims.UserID); err == nil {
				user = u
				clientID = fmt.Sprintf("u_%d", u.ID)
			}
		}
	}

	client := websocket.NewClient(clientID, conn)
	if user.ID != 0 {
		// 绑定为已登录用户
		client.UserID = user.ID
		if user.Nickname != "" {
			client.Name = user.Nickname
		} else {
			client.Name = user.Username
		}
		client.Avatar = user.AvatarURL
	} else {
		// 游客默认展示
		client.Name = client.ID
		client.Avatar = "https://api.dicebear.com/7.x/avataaars/svg?seed=" + client.ID
	}

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

// extractBearerToken 从 Authorization 头中提取 Bearer Token。
func extractBearerToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// generateUniqueClientID 用于游客/未鉴权场景生成临时 ID。
// 格式包含毫秒时间戳 + 随机字节，减少冲突概率。
func generateUniqueClientID() string {
	ts := time.Now().Format("150405.000")
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("user_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("user_%s_%s", ts, hex.EncodeToString(buf))
}
