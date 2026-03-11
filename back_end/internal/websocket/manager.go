package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

// Message 定义统一的消息结构
type Message struct {
	Type    string `json:"type"`             // 消息类型: "public", "private", "system"
	From    string `json:"from"`             // 发送者ID/Name
	To      string `json:"to,omitempty"`     // 接收者ID/Name (私聊时使用)
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
}

var Manager = ClientManager{
	Broadcast:  make(chan []byte, 1024),
	Register:   make(chan *Client, 1024),
	Unregister: make(chan *Client, 1024),
	Clients:    make(map[*Client]bool),
}

func (manager *ClientManager) Start() {
	for {
		select {
		case client := <-manager.Register:
			manager.Lock.Lock()
			manager.Clients[client] = true
			manager.Lock.Unlock()
			log.Printf("新用户加入: %s", client.ID)
			manager.SendSystemMessage("用户 " + client.ID + " 加入了聊天室")
			manager.BroadcastUserList()

		case client := <-manager.Unregister:
			manager.Lock.Lock()
			if _, ok := manager.Clients[client]; ok {
				delete(manager.Clients, client)
				close(client.Send)
			}
			manager.Lock.Unlock()
			log.Printf("用户离开: %s", client.ID)
			manager.SendSystemMessage("用户 " + client.ID + " 离开了聊天室")
			manager.BroadcastUserList()

		case message := <-manager.Broadcast:
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
		users = append(users, UserInfo{
			ID:     client.ID,
			Name:   client.ID,
			Avatar: "https://api.dicebear.com/7.x/avataaars/svg?seed=" + client.ID,
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
