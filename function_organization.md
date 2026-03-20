# 项目功能组织文档

## 1. JWT 令牌校验

### 1.1 概述
JWT（JSON Web Token）是一种基于 JSON 的开放标准（RFC 7519），用于在网络应用环境间传递声明。本项目使用 JWT 进行用户身份认证和授权。

### 1.2 实现细节

#### 1.2.1 JWT 服务
- **文件路径**：`back_end/internal/auth/jwt.go`
- **核心功能**：
  - 生成 JWT 令牌
  - 解析和校验 JWT 令牌

#### 1.2.2 JWT 中间件
- **文件路径**：`back_end/internal/auth/middleware.go`
- **核心功能**：
  - 从 HTTP 请求头中提取 Bearer Token
  - 校验 JWT 令牌的有效性
  - 将用户信息写入 Gin Context

### 1.3 工作流程

#### 1.3.1 令牌生成
1. 用户登录或注册时，后端验证用户凭据
2. 验证通过后，调用 `GenerateToken` 方法生成 JWT 令牌
3. 令牌包含用户 ID 和用户名等信息，以及过期时间
4. 令牌通过 HTTP 响应返回给前端

#### 1.3.2 令牌校验
1. 前端在后续请求中，将令牌放入 `Authorization` 头中，格式为 `Bearer <token>`
2. 后端的 JWT 中间件拦截请求，提取令牌
3. 调用 `ParseToken` 方法解析和校验令牌
4. 校验通过后，将用户信息写入 Gin Context，供后续处理函数使用
5. 校验失败时，返回 401 Unauthorized 错误

### 1.4 代码示例

#### 1.4.1 生成令牌
```go
// 创建 JWT 服务
jwtSvc := auth.NewService("your-secret-key")

// 生成令牌，有效期 24 小时
token, err := jwtSvc.GenerateToken(userID, username, 24*time.Hour)
if err != nil {
    // 处理错误
}

// 返回令牌给前端
c.JSON(http.StatusOK, gin.H{
    "token": token,
    "user": gin.H{
        "id":       userID,
        "username": username,
    },
})
```

#### 1.4.2 使用中间件
```go
// 应用 JWT 中间件到需要认证的路由组
api := r.Group("/api")
api.Use(auth.Middleware(jwtSvc))

// 受保护的路由
api.GET("/users/me", h.Me)
api.GET("/rooms", h.ListRooms)
```

#### 1.4.3 从 Context 中获取用户信息
```go
func (h *Handler) Me(c *gin.Context) {
    // 从 JWT 中间件写入的 context 获取 userId
    userIDVal, ok := c.Get(auth.CtxUserIDKey)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    userID, ok := userIDVal.(int64)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    
    // 使用 userID 获取用户信息
    // ...
}
```

### 1.5 安全注意事项
- 生产环境中应使用强随机密钥作为 JWT 签名密钥
- 令牌应设置合理的过期时间
- 前端应安全存储令牌，避免 XSS 攻击
- 考虑使用 HTTPS 传输令牌，避免中间人攻击

### 1.6 扩展功能（可选）
- 实现令牌刷新机制
- 添加令牌黑名单，用于主动失效令牌
- 集成 Redis 存储令牌状态，支持多实例部署

## 2. 房间加入机制

### 2.1 概述
本项目采用双层架构设计来实现房间加入功能，通过 HTTP 和 WebSocket 两套机制分别处理数据持久化和实时通信需求。这种设计能够同时满足数据一致性和实时性的要求。

### 2.2 架构设计

#### 2.2.1 双层机制对比

| 特性 | HTTP POST 机制 | WebSocket 机制 |
|------|--------------|----------------|
| **接口** | `POST /api/rooms/:id/join` | WS 消息: `{type: "join_room", roomId: 1}` |
| **主要作用** | 数据库持久化 | 实时消息路由 |
| **存储位置** | MySQL 数据库 | 内存映射（manager.Rooms） |
| **生命周期** | 长期存储 | 连接生命周期 |
| **文件位置** | `back_end/internal/handler/room_handler.go` | `back_end/internal/websocket/manager.go` |
| **调用时机** | 用户首次加入房间 | 用户连接或切换房间时 |

#### 2.2.2 架构原理

```
┌─────────────────────────────────────────────────────────┐
│                   HTTP 层面                            │
│  POST /api/rooms/:id/join                            │
│  → 数据库持久化（MySQL）                              │
│  → 记录用户-房间成员关系                              │
│  → 长期存储，支持历史查询                              │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│                   WebSocket 层面                       │
│  WS 消息: {type: "join_room", roomId: 1}             │
│  → 内存映射（manager.Rooms）                           │
│  → 消息广播路由                                       │
│  → 在线状态管理（Redis）                               │
│  → 实时通信，连接断开后自动清理                         │
└─────────────────────────────────────────────────────────┘
```

### 2.3 实现细节

#### 2.3.1 HTTP POST 机制

**文件路径**: `back_end/internal/handler/room_handler.go`

**核心功能**:
- 验证用户身份（JWT 中间件）
- 调用 `repo.AddRoomMember` 将用户-房间关系写入 MySQL
- 记录成员角色和加入时间
- 支持权限管理和历史查询

**代码实现**:
```go
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
```

#### 2.3.2 WebSocket 机制

**文件路径**: `back_end/internal/websocket/manager.go`

**核心功能**:
- 将 WebSocket 连接加入内存中的 `manager.Rooms` 映射
- 确保该连接能接收到房间的实时消息
- 调用 `Presence.AddUserToRoom` 更新 Redis 中的在线状态
- 连接断开时自动清理映射关系

**代码实现**:
```go
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
```

### 2.4 工作流程

#### 2.4.1 用户首次加入房间

```javascript
// 1. HTTP 请求：持久化成员关系
await fetch('/api/rooms/1/join', { 
    method: 'POST',
    headers: {
        'Authorization': 'Bearer ' + token
    }
})

// 2. WebSocket 消息：加入实时通信
socket.send(JSON.stringify({ 
    type: 'join_room', 
    roomId: 1 
}))
```

**执行流程**:
1. 前端发送 HTTP POST 请求到 `/api/rooms/1/join`
2. 后端验证 JWT 令牌，提取用户信息
3. 后端调用数据库操作，将用户-房间关系写入 MySQL
4. 前端发送 WebSocket 消息 `{type: "join_room", roomId: 1}`
5. 后端将当前连接加入内存中的房间成员映射
6. 后端更新 Redis 中的在线状态
7. 用户开始接收房间的实时消息

#### 2.4.2 用户重新连接

```javascript
// 用户重新连接 WebSocket 时
// 不需要再次调用 HTTP 接口（成员关系已存在）
// 只需要发送 WebSocket 消息重新加入实时通信
socket.send(JSON.stringify({ 
    type: 'join_room', 
    roomId: 1 
}))
```

**执行流程**:
1. 用户重新建立 WebSocket 连接
2. 前端发送 `join_room` 消息
3. 后端将新连接加入房间成员映射
4. 后端更新 Redis 在线状态
5. 用户恢复接收实时消息

### 2.5 设计优势

#### 2.5.1 职责分离
- **HTTP 层**: 负责业务逻辑、权限管理和数据持久化
- **WebSocket 层**: 负责实时通信、消息路由和状态管理

#### 2.5.2 性能优化
- 避免每次消息广播都查询数据库
- 内存映射提供 O(1) 的消息路由效率
- 减少数据库负载，提高系统响应速度

#### 2.5.3 状态同步
- 数据库保证数据一致性和持久化
- 内存映射保证实时性和低延迟
- Redis 支持多实例部署时的状态共享

#### 2.5.4 扩展性
- 支持多实例部署，Redis 共享在线状态
- 数据库提供跨实例的数据一致性
- 内存映射支持水平扩展的消息广播

### 2.6 消息广播机制

#### 2.6.1 房间消息广播

```go
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
```

**工作原理**:
1. 从 `manager.Rooms` 中获取房间的成员连接集合
2. 将消息发送到每个连接的发送通道
3. 客户端通过 WritePump 接收并显示消息

#### 2.6.2 连接清理

```go
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
```

**清理机制**:
- 连接断开时自动从所有房间中移除
- 空房间自动清理，释放内存
- 同时更新 Redis 在线状态

### 2.7 前端集成示例

```javascript
// 选择房间时同时调用两套机制
const selectRoom = async (room) => {
    // 1. HTTP 请求：持久化成员关系
    try {
        await fetch(`/api/rooms/${room.id}/join`, { 
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${token}`
            }
        })
    } catch (e) {
        console.error('Join room error:', e)
    }

    // 2. WebSocket 消息：加入实时通信
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ 
            type: 'join_room', 
            roomId: room.id 
        }))
    }

    // 3. 加载历史消息
    loadRoomMessages(room.id)
}
```

### 2.8 总结

房间加入机制的双层架构设计是即时通讯应用中的经典模式：

1. **互补而非冗余**: 两套机制各司其职，共同构建完整的房间管理系统
2. **数据一致性**: 数据库确保成员关系的持久化和一致性
3. **实时性能**: 内存映射确保消息广播的高效性和低延迟
4. **状态管理**: Redis 提供在线状态的实时更新和多实例支持

这种设计能够同时满足数据持久化和实时通信的需求，是构建可扩展即时通讯系统的重要基础。

## 3. 后续项目亮点

> 后续将补充其他项目亮点，如：
> - WebSocket 实时通信详细机制
> - 在线状态管理系统
> - 消息持久化策略
> - Redis 缓存优化
> - 多实例部署方案
