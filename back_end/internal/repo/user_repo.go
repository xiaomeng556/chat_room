package repo

import (
	"context"
	"database/sql"
	"time"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Nickname     string
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateUser 新建用户并返回完整记录。
func CreateUser(ctx context.Context, db *sql.DB, username, passwordHash, nickname, avatarURL string) (User, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, nickname, avatar_url) VALUES (?, ?, ?, ?)`,
		username, passwordHash, nickname, avatarURL,
	)
	if err != nil {
		return User{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return GetUserByID(ctx, db, id)
}

// GetUserByUsername 通过用户名查询用户。
func GetUserByUsername(ctx context.Context, db *sql.DB, username string) (User, error) {
	var u User
	err := db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, nickname, avatar_url, created_at, updated_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// GetUserByID 通过主键查询用户。
func GetUserByID(ctx context.Context, db *sql.DB, id int64) (User, error) {
	var u User
	err := db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, nickname, avatar_url, created_at, updated_at FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Nickname, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}
