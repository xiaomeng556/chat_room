package main

import (
	"log"

	"chat_room/internal/auth"
	"chat_room/internal/config"
	"chat_room/internal/handler"
	"chat_room/internal/presence"
	"chat_room/internal/storage"
	"chat_room/internal/websocket"

	"github.com/gin-gonic/gin"
)

func main() {
	// 1) 加载环境变量配置
	cfg := config.Load()

	// 2) 初始化 MySQL（必须）
	if cfg.MySQLDSN == "" {
		log.Fatal("MYSQL_DSN is required")
	}

	db, err := storage.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatal("mysql init failed: ", err)
	}
	if err := storage.MigrateMySQL(db); err != nil {
		log.Fatal("mysql migrate failed: ", err)
	}

	// 3) 初始化 Redis（用于在线状态/热数据）
	rdb, err := storage.NewRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal("redis init failed: ", err)
	}
	websocket.Manager.Presence = presence.NewRedisStore(rdb)

	// 4) 初始化 JWT 服务与 handler 依赖
	jwtSvc := auth.NewService(cfg.JWTSecret)
	h := handler.New(db, jwtSvc)

	// 启动 WebSocket 管理器
	websocket.Manager.SetDB(db)
	go websocket.Manager.Start()

	// 初始化 Gin
	r := gin.Default()

	// 5) CORS（本地前后端联调用；生产环境建议限制 Origin）
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// WebSocket 路由
	r.GET("/ws", h.WSHandler)

	// 6) 无需登录的认证接口
	r.POST("/api/auth/register", h.Register)
	r.POST("/api/auth/login", h.Login)

	// 7) 需要登录的 API
	api := r.Group("/api")
	api.Use(auth.Middleware(jwtSvc))
	api.GET("/users/me", h.Me)
	api.GET("/rooms", h.ListRooms)
	api.POST("/rooms", h.CreateRoom)
	api.POST("/rooms/:id/join", h.JoinRoom)
	api.POST("/rooms/:id/leave", h.LeaveRoom)
	api.GET("/rooms/:id/messages", h.ListRoomMessages)

	// 健康检查
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	log.Println("Server starting on ", cfg.ServerAddr)
	if err := r.Run(cfg.ServerAddr); err != nil {
		log.Fatal("Server run failed:", err)
	}
}
