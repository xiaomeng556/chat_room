<script setup lang="ts">
import { ref, onMounted, nextTick, onUnmounted } from 'vue'

interface User {
  id: string
  name: string
  avatar: string
  status: 'online' | 'offline'
}

interface Message {
  id: string
  type: 'public' | 'private' | 'system' | 'welcome'
  from: string
  to?: string
  content: string
  time: string
  isSelf: boolean
}

// 模拟当前用户
const currentUser = ref<User>({
  id: '',
  name: 'Connecting...',
  avatar: '',
  status: 'online'
})

// 在线用户列表
const onlineUsers = ref<User[]>([])

// 消息列表
const messages = ref<Message[]>([])

const inputMessage = ref('')
const selectedUser = ref<User | null>(null) // null 表示公聊
const messagesContainer = ref<HTMLElement | null>(null)
let socket: WebSocket | null = null

// 自动滚动到底部
const scrollToBottom = async () => {
  await nextTick()
  if (messagesContainer.value) {
    messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  }
}

// 初始化 WebSocket 连接
const initWebSocket = () => {
  // 如果已存在连接，先关闭
  if (socket) {
    socket.close()
  }
  // 添加随机时间戳参数，防止连接复用
  socket = new WebSocket(`ws://localhost:8888/ws?t=${Date.now()}`)

  socket.onopen = () => {
    console.log('WebSocket connected')
    // 连接成功后，后端通常会发送用户列表，我们通过接收到的第一条 user_list 消息来确认自己的 ID（如果有需要）
    // 但在当前简单的实现中，我们可能需要通过其他方式获取自己的 ID，或者后端在连接成功后发送一条 welcome 消息
  }

  socket.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      handleMessage(data)
    } catch (e) {
      console.error('JSON parse error:', e)
    }
  }

  socket.onclose = () => {
    console.log('WebSocket disconnected')
    currentUser.value.status = 'offline'
  }

  socket.onerror = (error) => {
    console.error('WebSocket error:', error)
  }
}

// 处理收到的消息
const handleMessage = (data: any) => {
  if (data.type === 'welcome') {
    // 收到欢迎消息，设置当前用户 ID
    currentUser.value = {
      id: data.to, // 后端将分配的 ID 放在了 to 字段
      name: data.to,
      avatar: `https://api.dicebear.com/7.x/avataaars/svg?seed=${data.to}`,
      status: 'online'
    }
    console.log('My ID is:', data.to)
    
  } else if (data.type === 'user_list') {
    // 更新在线用户列表
    const rawUsers = Array.isArray(data.users) ? data.users : (Array.isArray(data.Users) ? data.Users : [])
    const users: User[] = rawUsers.map((u: any) => ({
      id: u.id,
      name: u.name,
      avatar: u.avatar,
      status: u.status
    }))
    
    // 过滤掉自己（显示在左侧列表的应该是“其他用户”）
    // 只有当我们已经知道自己的 ID 时，才能准确过滤
    if (currentUser.value.id) {
       onlineUsers.value = users.filter(u => u.id !== currentUser.value.id)
    } else {
       // 如果还没收到 welcome，先全部显示，等有了 ID 后下次更新会自动过滤
       // 或者暂存逻辑。但在 WebSocket 顺序中，welcome 应该很快到达。
       onlineUsers.value = users
    }
    console.log('[WS] user_list received:', { total: users.length, shown: onlineUsers.value.length, me: currentUser.value.id })
    
  } else if (data.type === 'system') {
    messages.value.push({
      id: Date.now().toString(),
      type: 'system',
      from: 'System',
      content: data.content,
      time: data.time || new Date().toLocaleTimeString(),
      isSelf: false
    })
    scrollToBottom()

    // 简单的 Hack：如果收到 "用户 user_xxx 加入了聊天室"，且当前还没有 ID，可能那个就是自己
    // 但这不准确。更好的方式是后端专门发一个 init 消息。
    // 暂时先忽略这个问题，只展示消息。

  } else if (data.type === 'public' || data.type === 'private') {
    const isSelf = data.from === currentUser.value.id
    
    // 如果是私聊，且不是发给我的，也不是我发的（理论上后端不会推给我，但为了安全校验）
    if (data.type === 'private' && data.to !== currentUser.value.id && data.from !== currentUser.value.id) {
      return
    }

    messages.value.push({
      id: Date.now().toString(),
      type: data.type,
      from: data.from,
      to: data.to,
      content: data.content,
      time: data.time,
      isSelf: isSelf
    })
    scrollToBottom()
  }
}

// 发送消息
const sendMessage = () => {
  if (!inputMessage.value.trim() || !socket || socket.readyState !== WebSocket.OPEN) return

  const msg = {
    type: selectedUser.value ? 'private' : 'public',
    from: currentUser.value.id, // 注意：后端其实会覆盖这个字段，但前端先填上
    to: selectedUser.value?.id,
    content: inputMessage.value,
    time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
  }

  // 发送给后端
  socket.send(JSON.stringify(msg))
  
  // 乐观更新：如果是公聊，后端会广播回来，所以这里不需要手动 push，否则会重复
  // 如果是私聊，通常自己这端也需要显示，后端可能会回显，或者只发给对方
  // 根据目前的后端逻辑：Manager.Broadcast -> All Clients，所以自己也会收到自己发的消息。
  // 因此，前端不需要手动 push，统一等 onmessage 处理。

  inputMessage.value = ''
}

// 选择聊天对象
const selectChatTarget = (user: User | null) => {
  selectedUser.value = user
}

onMounted(() => {
  // 页面加载时生成一个临时的本地 ID，用于识别“我是谁”，
  // 但实际上 WebSocket 连接后，后端会分配一个新的 ID (user_xxxx)。
  // 为了让前端知道“我是谁”，我们需要后端在连接后告知。
  // 目前后端没有这个逻辑，我们只能先建立连接。
  // 改进方案：前端生成 ID 传给后端？或者后端返回 Welcome 消息。
  // 让我们修改一下后端逻辑？不，用户说“后端不要动”。
  // 那我们只能通过一种 trick：
  // 后端广播 user_list 时，包含所有用户。
  // 但我们不知道哪个是自己。
  // 
  // 妥协方案：
  // 既然后端逻辑是 "clientID := user_ + time"，它是基于时间的。
  // 前端确实很难猜。
  // 
  // **关键点**：在实际 WebSocket 交互中，后端通常会将 message 原样广播。
  // 如果后端将 message.From 强制覆写为 connection ID。
  // 那么当我发送一条消息后，收到的回执里 from 字段就是我的 ID。
  // 我们可以利用第一条发送的消息来确认自己的 ID。
  // 或者，我们在连接建立后，立刻发一条 "whoami" 的特殊消息？(后端目前不支持)
  //
  // 让我们先实现基础连接，看看效果。
  // 我们可以暂时在前端随机生成一个 ID，但这会被后端覆盖。
  // 
  // 仔细看后端代码：
  // msg.From = c.ID // 确保发送者是当前连接的 ID
  // Manager.Broadcast <- message
  // 
  // 所以，只要我发一条消息，收到的广播里 from 就是我的真实 ID。
  // 我们可以利用这一点。
  
  initWebSocket()
})

onUnmounted(() => {
  if (socket) {
    socket.close()
  }
})
</script>

<template>
  <div class="flex h-screen bg-gray-100 overflow-hidden">
    <!-- 左侧侧边栏：用户列表 -->
    <div class="w-64 bg-white border-r border-gray-200 flex flex-col">
      <!-- 当前用户信息 -->
      <div class="p-4 border-b border-gray-200 bg-gray-50">
        <div class="flex items-center space-x-3">
          <img :src="currentUser.avatar" alt="Avatar" class="w-10 h-10 rounded-full bg-gray-200">
          <div>
            <div class="font-semibold text-gray-800">{{ currentUser.name }}</div>
            <div class="text-xs text-green-500 flex items-center">
              <span class="w-2 h-2 bg-green-500 rounded-full mr-1"></span> 在线
            </div>
          </div>
        </div>
      </div>

      <!-- 用户列表 -->
      <div class="flex-1 overflow-y-auto">
        <div class="p-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
          在线用户 ({{ onlineUsers.length }})
        </div>
        
        <!-- 公聊入口 -->
        <div 
          @click="selectChatTarget(null)"
          :class="['flex items-center p-3 cursor-pointer hover:bg-gray-100 transition-colors', !selectedUser ? 'bg-blue-50 border-r-4 border-blue-500' : '']"
        >
          <div class="w-10 h-10 rounded-full bg-blue-100 flex items-center justify-center text-blue-500 mr-3">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
          </div>
          <span class="font-medium text-gray-700">公共聊天室</span>
        </div>

        <!-- 用户列表项 -->
        <div 
          v-for="user in onlineUsers" 
          :key="user.id"
          @click="selectChatTarget(user)"
          :class="['flex items-center p-3 cursor-pointer hover:bg-gray-100 transition-colors', selectedUser?.id === user.id ? 'bg-blue-50 border-r-4 border-blue-500' : '']"
        >
          <div class="relative mr-3">
            <img :src="user.avatar" class="w-10 h-10 rounded-full bg-gray-200">
            <span :class="['absolute bottom-0 right-0 w-3 h-3 border-2 border-white rounded-full', user.status === 'online' ? 'bg-green-500' : 'bg-gray-400']"></span>
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-900 truncate">{{ user.name }}</p>
            <p class="text-xs text-gray-500 truncate">{{ user.status === 'online' ? '在线' : '离线' }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- 右侧聊天区域 -->
    <div class="flex-1 flex flex-col bg-gray-50">
      <!-- 顶部导航栏 -->
      <div class="h-16 bg-white border-b border-gray-200 flex items-center justify-between px-6 shadow-sm">
        <div class="flex items-center">
          <h2 class="text-lg font-bold text-gray-800">
            {{ selectedUser ? selectedUser.name : '公共聊天室' }}
          </h2>
          <span v-if="selectedUser" class="ml-2 px-2 py-0.5 bg-blue-100 text-blue-800 text-xs rounded-full">私聊</span>
          <span v-else class="ml-2 px-2 py-0.5 bg-green-100 text-green-800 text-xs rounded-full">公聊</span>
        </div>
      </div>

      <!-- 消息展示区 -->
      <div ref="messagesContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
        <div 
          v-for="msg in messages" 
          :key="msg.id" 
          :class="['flex', msg.isSelf ? 'justify-end' : 'justify-start']"
        >
          <div :class="['flex max-w-[70%]', msg.isSelf ? 'flex-row-reverse' : 'flex-row']">
            <!-- 头像 -->
            <div class="flex-shrink-0">
              <img 
                :src="msg.isSelf ? currentUser.avatar : (selectedUser?.id === msg.from ? selectedUser.avatar : `https://api.dicebear.com/7.x/avataaars/svg?seed=${msg.from}`)" 
                class="w-8 h-8 rounded-full bg-gray-300"
              >
            </div>
            
            <!-- 消息气泡 -->
            <div :class="['mx-2 px-4 py-2 rounded-lg shadow-sm', msg.isSelf ? 'bg-blue-500 text-white rounded-tr-none' : 'bg-white text-gray-800 rounded-tl-none']">
              <div v-if="!msg.isSelf && !selectedUser" class="text-xs text-gray-500 mb-1 font-semibold">
                {{ msg.from }}
              </div>
              <p class="text-sm break-words">{{ msg.content }}</p>
              <div :class="['text-[10px] mt-1 text-right', msg.isSelf ? 'text-blue-100' : 'text-gray-400']">
                {{ msg.time }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 输入框区域 -->
      <div class="bg-white p-4 border-t border-gray-200">
        <div class="flex space-x-3">
          <input 
            v-model="inputMessage" 
            @keyup.enter="sendMessage"
            type="text" 
            placeholder="输入消息..." 
            class="flex-1 px-4 py-2 border border-gray-300 rounded-full focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          >
          <button 
            @click="sendMessage"
            class="px-6 py-2 bg-blue-500 text-white rounded-full hover:bg-blue-600 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="!inputMessage.trim()"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
