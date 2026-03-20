package repo

import (
	"context"
	"database/sql"
	"time"
)

type Room struct {
	ID          int64
	Name        string
	Type        int
	OwnerUserID int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListRooms 返回房间列表（按 id 倒序）。
func ListRooms(ctx context.Context, db *sql.DB) ([]Room, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, type, owner_user_id, created_at, updated_at FROM rooms ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.OwnerUserID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rooms, nil
}

// CreateRoom 创建房间，并默认将创建者加入 room_members（role=admin）。
func CreateRoom(ctx context.Context, db *sql.DB, name string, roomType int, ownerUserID int64) (Room, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO rooms (name, type, owner_user_id) VALUES (?, ?, ?)`,
		name, roomType, ownerUserID,
	)
	if err != nil {
		return Room{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Room{}, err
	}
	if err := AddRoomMember(ctx, db, id, ownerUserID, 1); err != nil {
		return Room{}, err
	}
	return GetRoomByID(ctx, db, id)
}

// GetRoomByID 根据房间 id 获取房间信息。
func GetRoomByID(ctx context.Context, db *sql.DB, id int64) (Room, error) {
	var r Room
	err := db.QueryRowContext(ctx,
		`SELECT id, name, type, owner_user_id, created_at, updated_at FROM rooms WHERE id = ?`,
		id,
	).Scan(&r.ID, &r.Name, &r.Type, &r.OwnerUserID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return Room{}, err
	}
	return r, nil
}

// AddRoomMember 将用户加入房间成员表；重复加入会更新 role。
func AddRoomMember(ctx context.Context, db *sql.DB, roomID, userID int64, role int) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO room_members (room_id, user_id, role) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE role = VALUES(role)`,
		roomID, userID, role,
	)
	return err
}

// RemoveRoomMember 将用户移出房间。
func RemoveRoomMember(ctx context.Context, db *sql.DB, roomID, userID int64) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM room_members WHERE room_id = ? AND user_id = ?`,
		roomID, userID,
	)
	return err
}

// IsRoomMember 判断用户是否为房间成员，用于权限校验。
func IsRoomMember(ctx context.Context, db *sql.DB, roomID, userID int64) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM room_members WHERE room_id = ? AND user_id = ?`, roomID, userID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
