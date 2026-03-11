# WebSocket 交互与实现原理文档

本文档详细说明了本项目（Go + Gin + Gorilla WebSocket）中 WebSocket 的通信机制，包括数据格式、前后端交互流程以及核心实现逻辑。

## 1. 通信协议与数据格式

所有通信均采用 JSON 格式。

### 1.1 基础消息结构 (Message)

后端定义在 [manager.go](file:///back_end/internal/websocket/manager.go#L10-L17) 中：

```json
{
  "type": "public",           // 消息类型: public(公聊), private(私聊), system(系统), user_list(用户列表)
  "from": "user_123456",      // 发送者 ID
  "to": "user_789012",        // 接收者 ID (私聊时必填)
  "content": "你好，世界！",   // 消息内容
  "time": "2024-03-09 10:00", // 发送时间
  "avatar": "..."             // 发送者头像 URL
}
```

### 1.2 消息类型说明

| Type        | 说明     | 数据流向                              | 触发场景         |
| :---------- | :----- | :-------------------------------- | :----------- |
| `public`    | 公聊消息   | Client -> Server -> All Clients   | 用户在公共聊天室发送消息 |
| `private`   | 私聊消息   | Client -> Server -> Target Client | 用户点对点发送消息    |
| `system`    | 系统通知   | Server -> All Clients             | 用户上线/下线通知    |
| `user_list` | 在线用户列表 | Server -> All Clients             | 用户上线/下线时全量推送 |

***

## 2. 交互流程详解

### 2.1 连接建立 (Handshake)

1. **前端**: 发起连接请求 `ws://localhost:8888/ws`。
2. **后端**:
   - [ws\_handler.go](file:///back_end/internal/handler/ws_handler.go#L22) 接收 HTTP 请求。
   - 调用 `upgrader.Upgrade` 将 HTTP 协议升级为 WebSocket 协议。
   - 生成唯一的 `clientID` (如 `user_102030`)。
   - 初始化 `Client` 对象并注册到 `Manager`。
   - 启动两个协程：`ReadPump` (读) 和 `WritePump` (写)。

### 2.2 消息发送 (Client -> Server)

1. **前端**: 用户输入消息并点击发送。
   ```javascript
   // 前端代码示例
   const msg = {
     type: "public",
     content: "Hello",
     // from 和 time 通常由后端生成或校验
   };
   socket.send(JSON.stringify(msg));
   ```
2. **后端**:
   - [client.go](file:///back_end/internal/websocket/client.go#L40) 中的 `ReadPump` 循环读取 Socket 数据。
   - `conn.ReadMessage()` 收到二进制流。
   - `json.Unmarshal` 解析为 `Message` 结构体。
   - 强制设置 `msg.From = currentClientID` (防止伪造发送者)。
   - 将消息推送到全局的 `Manager.Broadcast` 通道。

### 2.3 消息广播与展示 (Server -> Client)

1. **后端**:
   - [manager.go](file:///back_end/internal/websocket/manager.go#L60) 中的 `Start` 协程监听 `Broadcast` 通道。
   - 一旦收到消息，遍历 `Clients` 映射表中的所有在线用户。
   - 将消息发送到每个 Client 的 `Send` 通道。
   - [client.go](file:///back_end/internal/websocket/client.go#L79) 中的 `WritePump` 监听到 `Send` 通道有数据。
   - 调用 `conn.WriteMessage` 将 JSON 数据写回给前端 WebSocket 连接。
2. **前端**:
   - `socket.onmessage` 收到数据。
   - 解析 JSON，判断 `type`。
   - 如果是 `public/private`，将消息追加到聊天记录数组 `messages`。
   - 如果是 `user_list`，更新左侧在线用户列表。

### 2.4 心跳保活 (Ping/Pong)

为了防止连接因长时间无数据传输而断开：

- **后端**: `WritePump` 中有一个 `Ticker` (默认 54秒)，定期发送 `PingMessage`。
- **前端**: 浏览器原生 WebSocket 会自动回复 `Pong` (通常不可见，但由底层处理)。
- **后端**: `ReadPump` 设置了 `SetReadDeadline`，如果超过 60秒 未收到任何数据（包括 Pong），则断开连接。

***

## 3. 代码结构映射

- **[ws\_handler.go](file:///back_end/internal/handler/ws_handler.go)**:
  - 入口大门。负责握手、鉴权（未来实现）、初始化连接。
- **[client.go](file:///back_end/internal/websocket/client.go)**:
  - 负责干脏活累活。
  - `ReadPump`: 耳朵，专门听前端说什么。
  - `WritePump`: 嘴巴，专门把后端的消息传给前端。
- **[manager.go](file:///back_end/internal/websocket/manager.go)**:
  - 大脑。
  - 管理所有人的名单 (`Clients`)。
  - 决定消息该发给谁 (`Broadcast` 通道逻辑)。
  - 处理人员进出 (`Register`/`Unregister`)。

# 后续完善方向

使用mysql进行数据持久化

添加登录/注册相关的用户功能

支持点击用户名进入到私聊阶段

添加JWT校验功能

添加redis一系列（在线用户redis存储，热点消息用hash存储，JWT令牌登录续期）
