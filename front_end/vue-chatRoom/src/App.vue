<script setup lang="ts">
import { clearToken } from './services/auth'
import { useRoute, useRouter } from 'vue-router'

const router = useRouter()
const route = useRoute()

const logout = () => {
  clearToken()
  router.push('/login')
}
</script>

<template>
  <div class="h-screen w-screen">
    <div v-if="route.path !== '/login' && route.path !== '/register'" class="h-12 flex items-center justify-between px-4 bg-white border-b border-gray-200">
      <div class="font-semibold text-gray-800">Chat Room</div>
      <button class="text-sm text-blue-600 hover:underline" @click="logout">退出登录</button>
    </div>
    <div :class="route.path !== '/login' && route.path !== '/register' ? 'h-[calc(100vh-48px)]' : 'h-screen'">
      <router-view />
    </div>
  </div>
</template>

<style>
/* 移除默认样式干扰 */
body {
  margin: 0;
  padding: 0;
}
#app {
  max-width: none;
  padding: 0;
  margin: 0;
  display: block;
}
</style>
