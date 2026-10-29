<!-- 应用根组件：提供路由容器并在挂载时恢复登录态 -->
<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script setup lang="ts">
/**
 * App.vue - 应用根组件
 * 职责：
 *  - 挂载 <router-view> 作为所有页面的容器
 *  - 在 onMounted 生命周期中检查本地是否有 token，
 *    如果有则主动调用 fetchCurrentUser 恢复登录态、权限和领域信息
 */
import { onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'

// 认证状态 store 实例
const authStore = useAuthStore()

// 应用挂载时：根据本地 token 恢复登录态
onMounted(async () => {
  if (authStore.token) {
    await authStore.fetchCurrentUser()
  }
})
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
}

#app {
  width: 100%;
  height: 100vh;
}
</style>
