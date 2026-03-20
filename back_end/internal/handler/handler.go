package handler

import (
	"database/sql"

	"chat_room/internal/auth"
)

// Handler 聚合所有 HTTP/WS handler 所需依赖（DB、JWT 等）。
type Handler struct {
	DB  *sql.DB
	JWT *auth.Service
}

// New 创建 Handler 实例。
func New(db *sql.DB, jwt *auth.Service) *Handler {
	return &Handler{DB: db, JWT: jwt}
}
