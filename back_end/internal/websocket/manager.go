package websocket

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"sync"

	"chat_room/internal/presence"
)

// Message 定义统一的消息结构
type Message struct {
	Type    string `json:"type"`             // 消息类型: "public", "private", "system"
	From    string `json:"from"`             // 发送者ID/Name
	To      string `json:"to,omitempty"`     // 接收者ID/Name (私聊时使用)
	RoomID  int64  `json:"roomId,omitempty"` // 房间ID
	Content string `json:"content"`          // 消息内容
	Time    string `json:"time"`             // 发送时间
	Avatar  string `json:"avatar,omitempty"` // 发送者头像
}

// ClientManager 管理所有的 WebSocket 连接
type ClientManager struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	Lock       sync.RWMutex
	Presence   presence.Store
	Rooms      map[int64]map[*Client]bool
	DB         *sql.DB
}

var Manager = ClientManager{
	Broadcast:  make(chan []byte, 1024),
	Register:   make(chan *Client, 1024),
	Unregister: make(chan *Client, 1024),
	Clients:    make(map[*Client]bool),
	Rooms:      make(map[int64]map[*Client]bool),
}

func (manager *ClientManager) Start() {
	for {
		select {
		case client := <-manager.Register:
			// 注册连接
			manager.Lock.Lock()
			manager.Clients[client] = true
			manager.Lock.Unlock()
			if manager.Presence != nil && client.UserID > 0 {
				_ = manager.Presence.SetOnline(context.Background(), client.UserID)
			}
			log.Printf("新用户加入: %s", client.ID)
			manager.SendSystemMessage("用户 " + client.ID + " 加入了聊天室")
			manager.BroadcastUserList()

		case client := <-manager.Unregister:
			// 注销连接并清理房间映射
			manager.Lock.Lock()
			if _, ok := manager.Clients[client]; ok {
				delete(manager.Clients, client)
				close(client.Send)
			}
			for roomID, members := range manager.Rooms {
				if _, ok := members[client]; ok {
					delete(members, client)
					if len(members) == 0 {
						delete(manager.Rooms, roomID)
					}
				}
			}
			manager.Lock.Unlock()
			if manager.Presence != nil && client.UserID > 0 {
				_ = manager.Presence.SetOffline(context.Background(), client.UserID)
			}
			log.Printf("用户离开: %s", client.ID)
			manager.SendSystemMessage("用户 " + client.ID + " 离开了聊天室")
			manager.BroadcastUserList()

		case message := <-manager.Broadcast:
			// 全局广播：给所有连接发送消息帧
			// 快照当前客户端列表（读锁保护拷贝，避免并发读写 map）
			manager.Lock.RLock()
			clients := make([]*Client, 0, len(manager.Clients))
			for client := range manager.Clients {
				clients = append(clients, client)
			}
			manager.Lock.RUnlock()
			// 直接写入每个客户端的发送通道（使用缓冲通道，通常不会阻塞）
			for _, client := range clients {
				client.Send <- message
			}
		}
	}
}

// SendSystemMessage 发送系统广播
func (manager *ClientManager) SendSystemMessage(content string) {
	// system 消息统一走 Broadcast，确保与其它消息同一分发链路
	msg := Message{
		Type:    "system",
		From:    "System",
		Content: content,
		Time:    "", // 可选：添加当前时间
	}
	jsonMsg, _ := json.Marshal(msg)
	manager.Broadcast <- jsonMsg
}

// BroadcastUserList 广播在线用户列表
// manager.go - 修复 BroadcastUserList
func (manager *ClientManager) BroadcastUserList() {
	// 广播当前在线用户列表（当前为全局维度，后续可扩展为按 roomId 广播）
	// 阶段1：读锁获取用户信息（快速释放），避免并发修改引发异常
	manager.Lock.RLock()
	type UserInfo struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
		Status string `json:"status"`
	}
	var users []UserInfo
	clients := make([]*Client, 0, len(manager.Clients)) // 保存客户端引用
	for client := range manager.Clients {
		clients = append(clients, client)
		name := client.Name
		if name == "" {
			name = client.ID
		}
		avatar := client.Avatar
		if avatar == "" {
			avatar = "https://api.dicebear.com/7.x/avataaars/svg?seed=" + client.ID
		}
		users = append(users, UserInfo{
			ID:     client.ID,
			Name:   name,
			Avatar: avatar,
			Status: "online",
		})
	}
	manager.Lock.RUnlock() // 释放读锁

	// 阶段2：构造消息并发送（无锁）
	msg := struct {
		Type  string     `json:"type"`
		Users []UserInfo `json:"users"`
	}{
		Type:  "user_list",
		Users: users,
	}
	jsonMsg, _ := json.Marshal(msg)

	for _, client := range clients {
		client.Send <- jsonMsg
	}
}

// JoinRoom 将某个连接加入房间（用于 WS 广播维度）。
func (manager *ClientManager) JoinRoom(roomID int64, client *Client) {
	if roomID <= 0 {
		return
	}
	manager.Lock.Lock()
	members, ok := manager.Rooms[roomID]
	if !ok {
		members = make(map[*Client]bool)
		manager.Rooms[roomID] = members
	}
	members[client] = true
	manager.Lock.Unlock()

	if manager.Presence != nil && client.UserID > 0 {
		_ = manager.Presence.AddUserToRoom(context.Background(), roomID, client.UserID)
	}
}

// LeaveRoom 将某个连接移出房间。
func (manager *ClientManager) LeaveRoom(roomID int64, client *Client) {
	if roomID <= 0 {
		return
	}
	manager.Lock.Lock()
	if members, ok := manager.Rooms[roomID]; ok {
		delete(members, client)
		if len(members) == 0 {
			delete(manager.Rooms, roomID)
		}
	}
	manager.Lock.Unlock()

	if manager.Presence != nil && client.UserID > 0 {
		_ = manager.Presence.RemoveUserFromRoom(context.Background(), roomID, client.UserID)
	}
}

// BroadcastToRoom 将消息广播给房间内的所有连接。
func (manager *ClientManager) BroadcastToRoom(roomID int64, message []byte) {
	if roomID <= 0 {
		return
	}
	manager.Lock.RLock()
	members := manager.Rooms[roomID]
	clients := make([]*Client, 0, len(members))
	for c := range members {
		clients = append(clients, c)
	}
	manager.Lock.RUnlock()

	for _, c := range clients {
		c.Send <- message
	}
}

// SendToClientID 将消息发送给指定 clientID 的所有连接（一个用户可能多端在线）。
func (manager *ClientManager) SendToClientID(clientID string, message []byte) {
	if clientID == "" {
		return
	}
	manager.Lock.RLock()
	clients := make([]*Client, 0, len(manager.Clients))
	for c := range manager.Clients {
		if c.ID == clientID {
			clients = append(clients, c)
		}
	}
	manager.Lock.RUnlock()

	for _, c := range clients {
		c.Send <- message
	}
}

// SetDB 设置数据库连接
func (manager *ClientManager) SetDB(db *sql.DB) {
	manager.DB = db
}
