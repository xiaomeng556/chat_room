<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch } from '../services/api'
import { setToken } from '../services/auth'

const router = useRouter()
const username = ref('')
const password = ref('')
const nickname = ref('')
const avatarUrl = ref('')
const error = ref('')
const loading = ref(false)

const submit = async () => {
  error.value = ''
  loading.value = true
  try {
    const data = await apiFetch<{ token: string }>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({
        username: username.value,
        password: password.value,
        nickname: nickname.value,
        avatarUrl: avatarUrl.value,
      }),
    })
    setToken(data.token)
    router.push('/chat')
  } catch (e: any) {
    error.value = e?.message ?? '注册失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="h-full flex items-center justify-center bg-gray-100 p-6">
    <div class="w-full max-w-md bg-white rounded-xl shadow p-6">
      <h1 class="text-xl font-bold text-gray-800 mb-4">注册</h1>
      <div v-if="error" class="mb-3 text-sm text-red-600">{{ error }}</div>
      <div class="space-y-3">
        <input v-model="username" class="w-full border rounded-lg px-3 py-2" placeholder="用户名" />
        <input v-model="password" type="password" class="w-full border rounded-lg px-3 py-2" placeholder="密码" />
        <input v-model="nickname" class="w-full border rounded-lg px-3 py-2" placeholder="昵称（可选）" />
        <input v-model="avatarUrl" class="w-full border rounded-lg px-3 py-2" placeholder="头像 URL（可选）" />
        <button
          class="w-full bg-blue-600 text-white rounded-lg py-2 disabled:opacity-50"
          :disabled="loading || !username || !password"
          @click="submit"
        >
          {{ loading ? '注册中...' : '注册' }}
        </button>
      </div>
      <div class="mt-4 text-sm text-gray-600">
        已有账号？
        <router-link to="/login" class="text-blue-600 hover:underline">去登录</router-link>
      </div>
    </div>
  </div>
</template>

