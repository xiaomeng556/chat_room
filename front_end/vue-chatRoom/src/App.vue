<script setup lang="ts">
import ChatRoom from './components/ChatRoom.vue'
</script>

<template>
  <ChatRoom />
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
