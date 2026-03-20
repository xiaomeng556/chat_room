package storage

import (
	"context"
	"database/sql"
	"time"
)

// MigrateMySQL 在启动时自动创建必要的表结构。
// 当前使用 CREATE TABLE IF NOT EXISTS 的方式做轻量级迁移，便于本地快速启动。
// 生产环境建议切换为专业迁移工具（例如 golang-migrate）以进行版本化管理。
func MigrateMySQL(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			username VARCHAR(64) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			nickname VARCHAR(64) NOT NULL DEFAULT '',
			avatar_url VARCHAR(255) NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uk_users_username (username)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS rooms (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(64) NOT NULL,
			type TINYINT NOT NULL DEFAULT 0,
			owner_user_id BIGINT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			KEY idx_rooms_owner_user_id (owner_user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS room_members (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			room_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			role TINYINT NOT NULL DEFAULT 0,
			joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uk_room_members_room_user (room_id, user_id),
			KEY idx_room_members_user_id (user_id),
			KEY idx_room_members_room_id (room_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE IF NOT EXISTS messages (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			room_id BIGINT NOT NULL DEFAULT 0,
			from_user_id BIGINT NOT NULL,
			to_user_id BIGINT NULL,
			msg_type TINYINT NOT NULL DEFAULT 0,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			KEY idx_messages_room_created_at (room_id, created_at),
			KEY idx_messages_to_created_at (to_user_id, created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
	}

	// 为避免启动阻塞过久，这里设置较短超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
