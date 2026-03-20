package repo

import (
	"context"
	"database/sql"
	"time"
)

type Message struct {
	ID         int64
	RoomID     int64
	FromUserID int64
	ToUserID   sql.NullInt64
	MsgType    int
	Content    string
	CreatedAt  time.Time
}

// CreateMessage 写入一条消息记录并返回完整行（含自增 id、created_at）。
func CreateMessage(ctx context.Context, db *sql.DB, roomID, fromUserID int64, toUserID sql.NullInt64, msgType int, content string) (Message, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO messages (room_id, from_user_id, to_user_id, msg_type, content) VALUES (?, ?, ?, ?, ?)`,
		roomID, fromUserID, toUserID, msgType, content,
	)
	if err != nil {
		return Message{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Message{}, err
	}
	return GetMessageByID(ctx, db, id)
}

// GetMessageByID 根据消息 id 查询消息。
func GetMessageByID(ctx context.Context, db *sql.DB, id int64) (Message, error) {
	var m Message
	err := db.QueryRowContext(ctx,
		`SELECT id, room_id, from_user_id, to_user_id, msg_type, content, created_at FROM messages WHERE id = ?`,
		id,
	).Scan(&m.ID, &m.RoomID, &m.FromUserID, &m.ToUserID, &m.MsgType, &m.Content, &m.CreatedAt)
	if err != nil {
		return Message{}, err
	}
	return m, nil
}

// ListRoomMessages 查询房间历史消息（倒序返回），支持 before + limit。
func ListRoomMessages(ctx context.Context, db *sql.DB, roomID int64, before time.Time, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if before.IsZero() {
		before = time.Now()
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, room_id, from_user_id, to_user_id, msg_type, content, created_at
		 FROM messages
		 WHERE room_id = ? AND created_at < ?
		 ORDER BY created_at DESC
		 LIMIT ?`,
		roomID, before, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.RoomID, &m.FromUserID, &m.ToUserID, &m.MsgType, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return msgs, nil
}
