package presence

import "context"

// Store 定义在线状态存储能力。
// 目前实现为 Redis，后续可替换为其他存储或消息系统。
type Store interface {
	SetOnline(ctx context.Context, userID int64) error
	SetOffline(ctx context.Context, userID int64) error
	AddUserToRoom(ctx context.Context, roomID, userID int64) error
	RemoveUserFromRoom(ctx context.Context, roomID, userID int64) error
}
