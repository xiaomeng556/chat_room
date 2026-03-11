package main

import (
	"chat_room/internal/handler"
	"chat_room/internal/websocket"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// 启动 WebSocket 管理器
	go websocket.Manager.Start()

	// 初始化 Gin
	r := gin.Default()

	// WebSocket 路由
	r.GET("/ws", handler.WSHandler)

	// 健康检查
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	log.Println("Server starting on :8888")
	if err := r.Run(":8888"); err != nil {
		log.Fatal("Server run failed:", err)
	}
}
