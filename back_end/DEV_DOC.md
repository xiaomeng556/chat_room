# Go 聊天室项目开发与设计文档

## 1. 现阶段项目概述
当前项目是一个基于 Go 原生 `net` 包实现的 TCP 聊天室，具备基础的连接管理、广播和私聊功能。

### 核心功能
- **服务端 (Server)**: 监听 TCP 端口，维护在线用户列表 `OnlineMap`，处理全局消息广播。
- **用户管理 (User)**: 处理用户上线/下线通知，支持通过 `rename:` 前缀修改用户名。
- **消息机制**:
    - **公聊**: 广播消息给所有在线用户。
    - **私聊**: 通过 `to:用户名:消息` 格式实现点对点通信。
- **超时检测**: 10秒内无活动自动关闭连接（心跳检测雏形）。
- **客户端 (Client)**: 简单的命令行交互界面，支持菜单操作。

### 现有文件结构
- [main.go](file:///d:/Tools/trae_ai/code/chat_room/main.go): 程序入口，初始化并启动服务端。
- [server.go](file:///d:/Tools/trae_ai/code/chat_room/server.go): 服务端核心逻辑，包含 `Handler`、`BroadcastMsg` 和 `ListenMsg`。
- [user.go](file:///d:/Tools/trae_ai/code/chat_room/user.go): 用户实体定义，包含消息处理逻辑 `DoMessage`。
- [client.go](file:///d:/Tools/trae_ai/code/chat_room/client.go): 命令行客户端，处理用户输入与响应。

---

## 2. 架构设计方向 (Roadmap)

### 第一阶段：通信协议与消息结构升级
- **从 TCP 转向 WebSocket**: 引入 `gorilla/websocket` 库，使后端能够支持 Web 端连接。
- **消息 JSON 化**: 定义统一的消息结构体，替代目前的字符串拼接解析，提高协议可扩展性。
  ```json
  {
    "type": "chat/private/system",
    "from": "userA",
    "to": "userB",
    "content": "hello",
    "time": "2024-03-08 12:00:00"
  }
  ```

### 第二阶段：持久化与身份认证
- **用户中心**: 引入 MySQL 存储用户信息，支持用户注册、登录、头像上传及个人信息修改。
- **安全认证**: 采用 JWT (JSON Web Token) 进行连接认证，确保通信安全。
- **历史记录**: 使用 Redis 缓存最近消息，MySQL 持久化历史聊天记录。

### 第三阶段：高并发与实时性优化
- **状态同步**: 利用 Redis 的 Pub/Sub 机制，实现多节点部署下的消息分发与在线状态同步。
- **精细化心跳**: 优化现有的超时逻辑，实现双向心跳包检测，及时发现异常断连。

### 第四阶段：现代化前端实现
- **技术栈**: Vue 3 + Vite + Tailwind CSS。
- **功能特性**: 
    - 响应式聊天界面。
    - 在线用户列表实时更新。
    - 消息未读提醒。
    - 表情包支持及文件/图片预览。

---

## 3. 技术栈规划

- **后端 (Backend)**:
    - 语言: Go (Golang)
    - 通信协议: WebSocket (`gorilla/websocket`)
    - 数据库: MySQL (GORM)
    - 缓存: Redis (用于 Pub/Sub 和会话管理)
    - 认证: JWT (JSON Web Token)
- **前端 (Frontend)**:
    - 框架: Vue 3 (Composition API)
    - 构建工具: Vite
    - 状态管理: Pinia
    - 样式框架: Tailwind CSS
- **部署 (Deployment)**:
    - Docker + Docker Compose

---

## 4. 后续开发建议 (TODO)
- [ ] 重构 [user.go](file:///d:/Tools/trae_ai/code/chat_room/user.go) 中的 `DoMessage` 逻辑，消除重复代码。
- [ ] 引入 `Gin` 或 `Echo` 框架作为基础 Web 服务。
- [ ] 切换到 WebSocket 协议。
- [ ] 初始化 Vue 3 前端项目。
