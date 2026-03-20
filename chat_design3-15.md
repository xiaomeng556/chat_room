# Chat Room 设计方案（2026-03-15）

本文档用于整理本项目后续要新增的能力：登录/注册、聊天室房间、MySQL 持久化、Redis 在线状态与缓存设计，以及前后端对接方向。暂不改代码，后续按本文档逐步落地。

## 1. 总体目标

- 支持用户注册/登录，前端获得身份凭证（JWT）。
- WebSocket 连接具备身份识别能力：知道“我是谁”，并能按用户维度做在线状态、私聊、消息归属。
- 支持房间（Room）：加入/退出房间，房间公聊。
- 数据持久化：用户信息、房间信息、聊天消息可长期查询（分页、按时间范围）。
- Redis：存在线状态（presence）与热数据缓存，支持后续横向扩展。

## 2. 技术栈（保持现有推荐）

- 后端：Go + Gin + Gorilla WebSocket
- MySQL：用户/房间/消息持久化
- Redis：在线状态、会话辅助、最近消息缓存、Pub/Sub（可选）
- 前端：Vue 3 + Vite（现有 UI 继续沿用）

## 3. 核心模块划分（后端）

- Auth 模块：注册、登录、JWT 生成与校验
- User 模块：用户资料查询/修改（昵称、头像）
- Room 模块：房间 CRUD、加入/退出
- WS 模块：WebSocket 连接、消息收发、房间广播、私聊路由
- Message 模块：消息落库、历史消息查询、最近消息缓存
- Presence 模块：在线状态写入 Redis，断线/心跳更新

## 4. 业务流程设计

### 4.1 注册/登录

- 注册：
  - 前端：用户名 + 密码（可选：昵称/头像）
  - 后端：校验唯一性，密码使用 bcrypt 哈希存储
  - 返回：注册成功 + 可选择直接返回 JWT
- 登录：
  - 前端：用户名 + 密码
  - 后端：验证密码，生成 JWT（短期）+ 可选 Refresh Token（长期）
  - 返回：JWT、用户基础信息

### 4.2 WebSocket 建连与鉴权

- 前端建立 WS 时携带 JWT：
  - 推荐：`ws://host/ws?token=...`（短期方案）
  - 或：先走 HTTP 获取一次性 ws_ticket，再用 ticket 建连（更安全）
- 后端 Upgrade 前校验 token，解析出 userId
- 建连后发送 welcome：
  - `{ "type": "welcome", "user": { "id": "...", "name": "...", "avatar": "..." } }`
- Presence：
  - 建连成功：Redis 标记 online
  - 断开连接：Redis 标记 offline 或设置 last_seen

### 4.3 房间与消息

- 房间列表：HTTP 获取房间列表
- 加入房间：HTTP 或 WS 指令（推荐 HTTP，便于鉴权与持久化）
- 房间公聊：
  - 前端发送：`{type:"room_public", roomId:"...", content:"..." }`
  - 后端广播给房间内所有连接，并异步落库
- 私聊：
  - 前端发送：`{type:"private", toUserId:"...", content:"..." }`
  - 后端只推送给目标用户的在线连接 + 发送者自己回显（可选）

## 5. WebSocket 消息协议（建议）

### 5.1 通用字段

```json
{
  "type": "welcome|user_list|room_public|private|system|join_room|leave_room",
  "requestId": "可选，用于前端关联请求与回包",
  "time": "2026-03-15 16:40:00"
}
```

### 5.2 welcome

```json
{
  "type": "welcome",
  "user": { "id": "u_1", "name": "alice", "avatar": "..." }
}
```

### 5.3 user_list（按房间维度更合理）

```json
{
  "type": "user_list",
  "roomId": "r_1",
  "users": [
    { "id": "u_1", "name": "alice", "avatar": "...", "status": "online" }
  ]
}
```

### 5.4 room_public / private

```json
{
  "type": "room_public",
  "roomId": "r_1",
  "fromUserId": "u_1",
  "content": "hello"
}
```

```json
{
  "type": "private",
  "fromUserId": "u_1",
  "toUserId": "u_2",
  "content": "hi"
}
```

## 6. MySQL 数据库设计

数据库名建议：`chat_room`

### 6.1 users（用户表）

- `id` bigint/uuid（主键）
- `username` varchar(64) UNIQUE
- `password_hash` varchar(255)
- `nickname` varchar(64)
- `avatar_url` varchar(255)
- `status` tinyint（可选：0/1）
- `created_at` datetime
- `updated_at` datetime

索引：
- UNIQUE(username)

### 6.2 rooms（聊天室房间表）

- `id` bigint/uuid（主键）
- `name` varchar(64)
- `type` tinyint（可选：0=public, 1=private/group）
- `owner_user_id` bigint（创建者）
- `created_at` datetime
- `updated_at` datetime

索引：
- idx_owner_user_id

### 6.3 room_members（房间成员表）（建议新增）

房间功能要支持“谁在房间里”，通常需要成员关系表。

- `id` bigint（主键）
- `room_id` bigint
- `user_id` bigint
- `role` tinyint（可选：0=member,1=admin）
- `joined_at` datetime

索引：
- UNIQUE(room_id, user_id)
- idx_user_id

### 6.4 messages（聊天消息表）

建议 MySQL 持久化所有聊天消息，Redis 用作缓存。

- `id` bigint（主键）
- `room_id` bigint（公聊/房间消息必填）
- `from_user_id` bigint
- `to_user_id` bigint NULL（私聊时填写）
- `msg_type` tinyint（0=room_public,1=private,2=system）
- `content` text
- `created_at` datetime

索引：
- idx_room_id_created_at(room_id, created_at)
- idx_to_user_id_created_at(to_user_id, created_at)（私聊收件箱查询）

### 6.5 refresh_tokens（可选：用于登录续期）

如果采用 Refresh Token：

- `id` bigint
- `user_id` bigint
- `token_hash` varchar(255)
- `expires_at` datetime
- `created_at` datetime

## 7. Redis 设计（在线状态 + 缓存）

### 7.1 在线状态（Presence）

- `online:global`（Set）
  - 成员：userId
- `online:room:{roomId}`（Set）
  - 成员：userId
- `presence:user:{userId}`（Hash）
  - `status` = online/offline
  - `last_seen` = unix_ts
  - `conn_id` = 当前连接标识（可选）

策略：
- 建连成功：SADD online:global userId；HSET presence:user:userId status online last_seen now
- 加入房间：SADD online:room:{roomId} userId
- 断开：SREM online:global userId；SREM online:room:* userId；HSET status offline last_seen now

### 7.2 最近消息缓存（可选）

- `recent:room:{roomId}`（List）
  - LPUSH 新消息 JSON（或 messageId）
  - LTRIM 保留最近 N 条（如 200）

推荐：
- MySQL 永久存储
- Redis 只缓存“最近 N 条”用于秒开

### 7.3 JWT/会话辅助（可选）

如果要做“踢下线/强制登出/续期”：
- `jwt:blocklist`（Set 或 Bloom，按需）
- `session:{token}` -> userId（如果不用 JWT 而用 session）

### 7.4 多实例扩展（后续可选）

当后端需要水平扩展（多个进程/多台机器）：
- 房间广播可使用 Redis Pub/Sub 或 Redis Stream：
  - `pub:room:{roomId}`：发布消息，各实例订阅并向本机连接推送

## 8. 前端改造点（仅规划）

### 8.1 登录/注册页面

- 路由：`/login`、`/register`
- 登录成功后保存 JWT（建议 memory + localStorage 双写，注意 XSS 风险）

### 8.2 WS 连接携带 token

- `new WebSocket("ws://localhost:8888/ws?token=" + encodeURIComponent(token))`
- 收到 welcome 后设置 currentUser

### 8.3 房间 UI

- 房间列表（左侧）：房间切换
- 在线用户列表：按 roomId 展示（user_list 包含 roomId）
- 消息列表：按房间隔离（messages[roomId]）

## 9. 接口清单（建议）

### 9.1 HTTP

- POST `/api/auth/register`
- POST `/api/auth/login`
- POST `/api/auth/logout`（可选）
- GET `/api/users/me`
- GET `/api/rooms`
- POST `/api/rooms`（创建房间）
- POST `/api/rooms/:id/join`
- POST `/api/rooms/:id/leave`
- GET `/api/rooms/:id/messages?before=&limit=`（历史消息分页）

### 9.2 WebSocket

- GET `/ws`（Upgrade，携带 token）

## 10. 消息存储：MySQL vs Redis（结论）

- MySQL：聊天消息的最终存档（可查询、可备份、可审计）
- Redis：在线状态 + 最近消息缓存（可选） + 后续多实例广播（可选）

