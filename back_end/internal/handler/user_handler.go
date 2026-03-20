package handler

import (
	"context"
	"net/http"
	"time"

	"chat_room/internal/auth"
	"chat_room/internal/repo"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Me(c *gin.Context) {
	// 从 JWT 中间件写入的 context 获取 userId
	userIDVal, ok := c.Get(auth.CtxUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, ok := userIDVal.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	u, err := repo.GetUserByID(ctx, h.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       u.ID,
		"username": u.Username,
		"nickname": u.Nickname,
		"avatar":   u.AvatarURL,
	})
}
