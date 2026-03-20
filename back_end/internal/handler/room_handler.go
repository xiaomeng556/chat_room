package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"chat_room/internal/auth"
	"chat_room/internal/repo"

	"github.com/gin-gonic/gin"
)

type createRoomReq struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

// ListRooms 返回房间列表（需要登录）。
func (h *Handler) ListRooms(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	rooms, err := repo.ListRooms(ctx, h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}
	c.JSON(http.StatusOK, rooms)
}

// CreateRoom 创建房间，并把创建者写入 room_members（role=admin）。
func (h *Handler) CreateRoom(c *gin.Context) {
	userIDVal, ok := c.Get(auth.CtxUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req createRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	room, err := repo.CreateRoom(ctx, h.DB, req.Name, req.Type, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create room failed"})
		return
	}
	c.JSON(http.StatusOK, room)
}

// JoinRoom 将当前用户加入指定房间（写入 room_members）。
func (h *Handler) JoinRoom(c *gin.Context) {
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

	if err := repo.AddRoomMember(ctx, h.DB, roomID, userID, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "join failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// LeaveRoom 将当前用户从指定房间移除（删除 room_members 记录）。
func (h *Handler) LeaveRoom(c *gin.Context) {
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

	if err := repo.RemoveRoomMember(ctx, h.DB, roomID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "leave failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
