package storage

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedis 创建 Redis 客户端并做一次 Ping 校验。
func NewRedis(addr, password, dbStr string) (*redis.Client, error) {
	db := 0
	if dbStr != "" {
		if v, err := strconv.Atoi(dbStr); err == nil {
			db = v
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 启动时 Ping 校验，避免运行中首次访问才暴露连接问题
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}

	return rdb, nil
}
