<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { apiFetch, wsUrl } from '../services/api'
import { getToken } from '../services/auth'

interface User {
  id: string
  name: string
  avatar: string
  status: 'online' | 'offline'
}

interface Room {
  id: number
  name: string
  type: number
  ownerUserID: number
}

interface Me {
  id: number
  username: string
  nickname: string
  avatar: string
}

interface Message {
  id: string
  type: 'room_public' | 'private' | 'system' | 'welcome'
  from: string
  to?: string
  roomId?: number
  content: string
  time: string
  isSelf: boolean
}

const currentUser = ref<User>({ id: '', name: 'Loading...', avatar: '', status: 'online' })
const onlineUsers = ref<User[]>([])
const rooms = ref<Room[]>([])
const activeRoomId = ref<number | null>(null)
const selectedUser = ref<User | null>(null)

const roomMessages = ref<Record<string, Message[]>>({})
const privateMessages = ref<Record<string, Message[]>>({})

const inputMessage = ref('')
const messagesContainer = ref<HTMLElement | null>(null)
let socket: WebSocket | null = null

// 错误处理和加载状态
const loading = ref(true)
const error = ref('')
const wsError = ref('')

const activeRoom = computed(() => rooms.value.find(r => r.id === activeRoomId.value) ?? null)
const title = computed(() => {
  if (selectedUser.value) return selectedUser.value.name
  return activeRoom.value?.name ?? '请选择房间'
})

const activeKey = computed(() => {
  if (selectedUser.value) return `private:${selectedUser.value.id}`
  if (activeRoomId.value) return `room:${activeRoomId.value}`
  return 'room:0'
})

const visibleMessages = computed<Message[]>(() => {
  const key = activeKey.value
  if (key.startsWith('private:')) return privateMessages.value[key] ?? []
  return roomMessages.value[key] ?? []
})

const scrollToBottom = async () => {
  await nextTick()
  if (messagesContainer.value) messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
}

const pushMessage = (key: string, msg: Message) => {
  const store = key.startsWith('private:') ? privateMessages.value : roomMessages.value
  if (!store[key]) store[key] = []
  store[key].push(msg)
}

const joinRoomWS = (roomId: number) => {
  if (!socket || socket.readyState !== WebSocket.OPEN) return
  socket.send(JSON.stringify({ type: 'join_room', roomId }))
}

const initWebSocket = () => {
  const token = getToken()
  if (!token) return

  if (socket) socket.close()
  socket = new WebSocket(wsUrl('/ws', token))

  socket.onopen = () => {
    wsError.value = ''
    if (activeRoomId.value) joinRoomWS(activeRoomId.value)
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
    currentUser.value.status = 'offline'
    wsError.value = 'WebSocket 连接已断开，正在尝试重连...'
    // 尝试重连
    setTimeout(initWebSocket, 3000)
  }

  socket.onerror = (error) => {
    console.error('WebSocket error:', error)
    wsError.value = 'WebSocket 连接出错'
  }
}

const handleMessage = (data: any) => {
  if (data.type === 'welcome') {
    const u = data.user
    if (u && typeof u.id === 'number') {
      currentUser.value = {
        id: `u_${u.id}`,
        name: u.name ?? String(u.id),
        avatar: u.avatar ?? '',
        status: 'online'
      }
    } else {
      currentUser.value = {
        id: data.to,
        name: data.to,
        avatar: `https://api.dicebear.com/7.x/avataaars/svg?seed=${data.to}`,
        status: 'online'
      }
    }
    return
  }

  if (data.type === 'user_list') {
    const rawUsers = Array.isArray(data.users) ? data.users : (Array.isArray(data.Users) ? data.Users : [])
    const users: User[] = rawUsers.map((u: any) => ({
      id: u.id,
      name: u.name,
      avatar: u.avatar,
      status: u.status
    }))
    onlineUsers.value = currentUser.value.id ? users.filter(u => u.id !== currentUser.value.id) : users
    return
  }

  if (data.type === 'system') {
    const msg: Message = {
      id: Date.now().toString(),
      type: 'system',
      from: 'System',
      content: data.content ?? '',
      time: data.time ?? '',
      isSelf: false
    }
    const key = `room:${activeRoomId.value ?? 0}`
    pushMessage(key, msg)
    if (!selectedUser.value) scrollToBottom()
    return
  }

  if (data.type === 'room_public') {
    const roomId = Number(data.roomId ?? 0)
    const msg: Message = {
      id: Date.now().toString(),
      type: 'room_public',
      from: data.from,
      roomId,
      content: data.content ?? '',
      time: data.time ?? '',
      isSelf: data.from === currentUser.value.id
    }
    pushMessage(`room:${roomId}`, msg)
    if (!selectedUser.value && activeRoomId.value === roomId) scrollToBottom()
    return
  }

  if (data.type === 'private') {
    const isSelf = data.from === currentUser.value.id
    const peerId = isSelf ? data.to : data.from
    const key = `private:${peerId}`
    const msg: Message = {
      id: Date.now().toString(),
      type: 'private',
      from: data.from,
      to: data.to,
      content: data.content ?? '',
      time: data.time ?? '',
      isSelf
    }
    pushMessage(key, msg)
    if (selectedUser.value?.id === peerId) scrollToBottom()
  }
}

const loadInitial = async () => {
  loading.value = true
  error.value = ''
  try {
    const me = await apiFetch<Me>('/api/users/me')
    currentUser.value = {
      id: `u_${me.id}`,
      name: me.nickname || me.username,
      avatar: me.avatar,
      status: 'online'
    }

    rooms.value = await apiFetch<Room[]>('/api/rooms')
    if (rooms.value.length) {
      await selectRoom(rooms.value[0])
    }
  } catch (e: any) {
    error.value = e?.message ?? '加载失败，请刷新页面重试'
    console.error('Load initial error:', e)
  } finally {
    loading.value = false
  }
}

const selectRoom = async (room: Room) => {
  selectedUser.value = null
  activeRoomId.value = room.id

  const key = `room:${room.id}`
  if (!roomMessages.value[key]) roomMessages.value[key] = []

  try {
    await apiFetch<{ ok: boolean }>(`/api/rooms/${room.id}/join`, { method: 'POST', body: '{}' })
  } catch (e) {
    console.error('Join room error:', e)
  }

  joinRoomWS(room.id)

  try {
    const history = await apiFetch<any[]>(`/api/rooms/${room.id}/messages?limit=50`)
    const list: Message[] = history
      .slice()
      .reverse()
      .map((m) => {
        const fromUserId = m.FromUserID ?? m.from_user_id ?? m.fromUserID
        const from = `u_${fromUserId}`
        const createdAt = (m.CreatedAt ?? m.created_at ?? '').toString()
        const time = createdAt.includes('T') ? createdAt.split('T')[1]?.slice(0, 8) ?? '' : createdAt.slice(11, 19)
        return {
          id: String(m.ID ?? m.id),
          type: 'room_public',
          from,
          roomId: room.id,
          content: m.Content ?? m.content,
          time,
          isSelf: from === currentUser.value.id
        }
      })
    roomMessages.value[key] = list
    scrollToBottom()
  } catch (e) {
    console.error('Load messages error:', e)
  }
}

const selectUser = (user: User) => {
  selectedUser.value = user
  const key = `private:${user.id}`
  if (!privateMessages.value[key]) privateMessages.value[key] = []
  scrollToBottom()
}

const createRoomName = ref('')
const creatingRoom = ref(false)
const createRoom = async () => {
  if (!createRoomName.value.trim()) return
  creatingRoom.value = true
  try {
    const room = await apiFetch<Room>('/api/rooms', {
      method: 'POST',
      body: JSON.stringify({ name: createRoomName.value.trim(), type: 0 })
    })
    rooms.value = [room, ...rooms.value]
    createRoomName.value = ''
    await selectRoom(room)
  } catch (e: any) {
    error.value = e?.message ?? '创建房间失败'
    console.error('Create room error:', e)
  } finally {
    creatingRoom.value = false
  }
}

const sendMessage = () => {
  if (!inputMessage.value.trim() || !socket || socket.readyState !== WebSocket.OPEN) return

  if (selectedUser.value) {
    socket.send(JSON.stringify({
      type: 'private',
      to: selectedUser.value.id,
      content: inputMessage.value,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }))
  } else if (activeRoomId.value) {
    socket.send(JSON.stringify({
      type: 'room_public',
      roomId: activeRoomId.value,
      content: inputMessage.value,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }))
  }

  inputMessage.value = ''
}

onMounted(() => {
  loadInitial().finally(() => initWebSocket())
})

onUnmounted(() => {
  socket?.close()
})
</script>

<template>
  <div class="flex h-full bg-gray-100 overflow-hidden">
    <div class="w-72 bg-white border-r border-gray-200 flex flex-col">
      <div class="p-4 border-b border-gray-200 bg-gray-50">
        <div class="flex items-center space-x-3">
          <img :src="currentUser.avatar" alt="Avatar" class="w-10 h-10 rounded-full bg-gray-200">
          <div class="min-w-0">
            <div class="font-semibold text-gray-800 truncate">{{ currentUser.name }}</div>
            <div class="text-xs flex items-center">
              <span class="w-2 h-2 rounded-full mr-1" :class="currentUser.status === 'online' ? 'bg-green-500' : 'bg-gray-400'"></span>
              {{ currentUser.status === 'online' ? '在线' : '离线' }}
            </div>
          </div>
        </div>
        <div v-if="wsError" class="mt-2 text-xs text-red-600">{{ wsError }}</div>
      </div>

      <div class="flex-1 overflow-y-auto">
        <div v-if="error" class="p-3 text-xs text-red-600">{{ error }}</div>
        
        <div class="p-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">房间</div>
        <div class="px-3 pb-3 flex gap-2">
          <input v-model="createRoomName" class="flex-1 border rounded-lg px-3 py-2 text-sm" placeholder="新建房间名称" />
          <button 
            class="px-3 py-2 bg-blue-600 text-white rounded-lg text-sm disabled:opacity-50" 
            :disabled="!createRoomName.trim() || creatingRoom" 
            @click="createRoom"
          >
            {{ creatingRoom ? '创建中...' : '创建' }}
          </button>
        </div>
        
        <div v-if="loading" class="p-3 text-xs text-gray-500">加载中...</div>
        <div v-else v-for="room in rooms" :key="room.id"
             @click="selectRoom(room)"
             :class="['px-3 py-2 mx-2 rounded-lg cursor-pointer hover:bg-gray-100', activeRoomId === room.id && !selectedUser ? 'bg-blue-50 border border-blue-200' : '']">
          <div class="text-sm font-medium text-gray-800 truncate">{{ room.name }}</div>
          <div class="text-xs text-gray-500">ID: {{ room.id }}</div>
        </div>

        <div class="p-3 mt-4 text-xs font-semibold text-gray-400 uppercase tracking-wider">
          在线用户 ({{ onlineUsers.length }})
        </div>
        <div v-for="user in onlineUsers" :key="user.id"
             @click="selectUser(user)"
             :class="['flex items-center p-3 mx-2 rounded-lg cursor-pointer hover:bg-gray-100', selectedUser?.id === user.id ? 'bg-blue-50 border border-blue-200' : '']">
          <div class="relative mr-3">
            <img :src="user.avatar" class="w-9 h-9 rounded-full bg-gray-200">
            <span :class="['absolute bottom-0 right-0 w-3 h-3 border-2 border-white rounded-full', user.status === 'online' ? 'bg-green-500' : 'bg-gray-400']"></span>
          </div>
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium text-gray-900 truncate">{{ user.name }}</p>
            <p class="text-xs text-gray-500 truncate">{{ user.id }}</p>
          </div>
        </div>
      </div>
    </div>

    <div class="flex-1 flex flex-col bg-gray-50">
      <div class="h-16 bg-white border-b border-gray-200 flex items-center justify-between px-6 shadow-sm">
        <div class="flex items-center">
          <h2 class="text-lg font-bold text-gray-800">{{ title }}</h2>
          <span v-if="selectedUser" class="ml-2 px-2 py-0.5 bg-blue-100 text-blue-800 text-xs rounded-full">私聊</span>
          <span v-else class="ml-2 px-2 py-0.5 bg-green-100 text-green-800 text-xs rounded-full">房间</span>
        </div>
        <div class="text-xs text-gray-500" v-if="!selectedUser && activeRoomId">RoomID: {{ activeRoomId }}</div>
      </div>

      <div ref="messagesContainer" class="flex-1 overflow-y-auto p-6 space-y-4">
        <div v-if="loading" class="flex justify-center items-center h-32">
          <div class="text-sm text-gray-500">加载中...</div>
        </div>
        <div v-else-if="!selectedUser && !activeRoomId" class="text-sm text-gray-500">请选择一个房间开始聊天</div>
        <div v-else v-for="msg in visibleMessages" :key="msg.id" :class="['flex', msg.isSelf ? 'justify-end' : 'justify-start']">
          <div :class="['flex max-w-[70%]', msg.isSelf ? 'flex-row-reverse' : 'flex-row']">
            <div class="flex-shrink-0">
              <img
                :src="msg.isSelf ? currentUser.avatar : `https://api.dicebear.com/7.x/avataaars/svg?seed=${msg.from}`"
                class="w-8 h-8 rounded-full bg-gray-300"
              >
            </div>
            <div :class="['mx-2 px-4 py-2 rounded-lg shadow-sm', msg.isSelf ? 'bg-blue-500 text-white rounded-tr-none' : 'bg-white text-gray-800 rounded-tl-none']">
              <div v-if="!msg.isSelf" class="text-xs text-gray-500 mb-1 font-semibold">
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
            :disabled="!inputMessage.trim() || (!selectedUser && !activeRoomId) || !socket || socket.readyState !== WebSocket.OPEN"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
