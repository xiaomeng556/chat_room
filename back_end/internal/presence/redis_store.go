package presence

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	rdb *redis.Client
}

// NewRedisStore 创建基于 Redis 的在线状态存储实现。
func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb}
}

// SetOnline 标记用户在线，并刷新 last_seen。
func (s *RedisStore) SetOnline(ctx context.Context, userID int64) error {
	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, "online:global", userID)
	pipe.HSet(ctx, fmt.Sprintf("presence:user:%d", userID), map[string]any{
		"status":    "online",
		"last_seen": strconv.FormatInt(time.Now().Unix(), 10),
	})
	_, err := pipe.Exec(ctx)
	return err
}

// SetOffline 标记用户离线，并写入 last_seen。
func (s *RedisStore) SetOffline(ctx context.Context, userID int64) error {
	pipe := s.rdb.TxPipeline()
	pipe.SRem(ctx, "online:global", userID)
	pipe.HSet(ctx, fmt.Sprintf("presence:user:%d", userID), map[string]any{
		"status":    "offline",
		"last_seen": strconv.FormatInt(time.Now().Unix(), 10),
	})
	_, err := pipe.Exec(ctx)
	return err
}

// AddUserToRoom 将用户加入某个房间的在线集合（仅做在线状态统计，不代表成员关系）。
func (s *RedisStore) AddUserToRoom(ctx context.Context, roomID, userID int64) error {
	return s.rdb.SAdd(ctx, fmt.Sprintf("online:room:%d", roomID), userID).Err()
}

// RemoveUserFromRoom 将用户从房间在线集合移除。
func (s *RedisStore) RemoveUserFromRoom(ctx context.Context, roomID, userID int64) error {
	return s.rdb.SRem(ctx, fmt.Sprintf("online:room:%d", roomID), userID).Err()
}
