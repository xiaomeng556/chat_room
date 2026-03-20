package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"chat_room/internal/auth"
	"chat_room/internal/repo"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ListRoomMessages(c *gin.Context) {
	// 仅允许已登录用户访问房间历史消息
	userIDVal, ok := c.Get(auth.CtxUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	roomID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || roomID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	isMember, err := repo.IsRoomMember(ctx, h.DB, roomID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	if !isMember {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	// 支持 limit 与 before（RFC3339）分页
	limit, _ := strconv.Atoi(c.Query("limit"))
	beforeStr := c.Query("before")
	var before time.Time
	if beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			before = t
		}
	}

	msgs, err := repo.ListRoomMessages(ctx, h.DB, roomID, before, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	c.JSON(http.StatusOK, msgs)
}
