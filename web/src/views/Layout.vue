<template>
  <el-container class="main-layout">
    <el-aside class="sidebar" :width="isCollapse ? '72px' : '260px'">
      <div class="sidebar-header">
        <div class="logo">
          <div class="logo-icon">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
              <rect x="2" y="2" width="28" height="28" rx="6" stroke="currentColor" stroke-width="2"/>
              <path d="M8 16L12 12L16 16L20 12" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M8 20L12 16L16 20L20 16" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" opacity="0.5"/>
            </svg>
          </div>
          <Transition name="fade">
            <div v-if="!isCollapse" class="logo-text">
              <span>BD<span class="accent">ops</span>Flow</span>
            </div>
          </Transition>
        </div>
        <el-button
          :icon="isCollapse ? Expand : Fold"
          text
          @click="isCollapse = !isCollapse"
          class="collapse-btn"
        />
      </div>

      <el-menu
        :default-active="activeMenu"
        :collapse="isCollapse"
        :unique-opened="false"
        router
        class="sidebar-nav"
      >
        <el-menu-item index="/">
          <el-icon><DataAnalysis /></el-icon>
          <template #title>仪表盘</template>
        </el-menu-item>
        <el-menu-item index="/tasks">
          <el-icon><List /></el-icon>
          <template #title>任务管理</template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon><Document /></el-icon>
          <template #title>任务日志</template>
        </el-menu-item>
        <el-menu-item index="/workflows">
          <el-icon><Connection /></el-icon>
          <template #title>工作流</template>
        </el-menu-item>
        <el-menu-item index="/executors">
          <el-icon><Cpu /></el-icon>
          <template #title>执行器</template>
        </el-menu-item>
      </el-menu>

      <div class="sidebar-footer">
        <div class="system-status">
          <div class="status-indicator">
            <span class="status-dot"></span>
            <Transition name="fade">
              <span v-if="!isCollapse" class="status-text">系统运行正常</span>
            </Transition>
          </div>
        </div>

        <div class="user-profile">
          <div class="user-avatar">
            {{ user?.username?.charAt(0)?.toUpperCase() || 'U' }}
          </div>
          <Transition name="fade">
            <div v-if="!isCollapse" class="user-info">
              <div class="user-name">{{ user?.username || '用户' }}</div>
              <div class="user-role">{{ user?.role || '操作员' }}</div>
            </div>
          </Transition>
        </div>

        <el-button
          :icon="SwitchButton"
          text
          @click="handleLogout"
          class="logout-btn"
        >
          <Transition name="fade">
            <span v-if="!isCollapse">退出登录</span>
          </Transition>
        </el-button>
      </div>
    </el-aside>

    <el-container class="content-wrapper">
      <el-header class="header">
        <div class="header-left">
          <h1 class="page-title">{{ pageTitle }}</h1>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item>{{ pageTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="header-right">
          <div class="header-actions">
            <el-button :icon="Search" circle size="small" />
            <el-button :icon="Bell" circle size="small">
              <template #default>
                <span class="notification-badge">3</span>
              </template>
            </el-button>
            <el-button :icon="Setting" circle size="small" />
          </div>

          <div class="header-divider"></div>

          <div class="user-menu">
            <div class="user-avatar-small">
              {{ user?.username?.charAt(0)?.toUpperCase() || 'U' }}
            </div>
          </div>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  DataAnalysis,
  List,
  Connection,
  Cpu,
  Document,
  SwitchButton,
  Bell,
  Setting,
  Expand,
  Fold,
  Search
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const isCollapse = ref(false)

const user = computed(() => authStore.user)
const activeMenu = computed(() => route.path)

const pageTitle = computed(() => {
  const titles: Record<string, string> = {
    '/': '仪表盘',
    '/tasks': '任务管理',
    '/workflows': '工作流设计',
    '/executors': '执行器集群',
    '/logs': '任务日志'
  }
  return titles[route.path] || '仪表盘'
})

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.main-layout {
  min-height: 100vh;
  background-color: var(--bg-primary);
}

.sidebar {
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-default);
  display: flex;
  flex-direction: column;
  transition: width 0.3s var(--ease-out);
  overflow: hidden;
}

.sidebar-header {
  padding: var(--space-4);
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-default);
  min-height: 64px;
}

.logo {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  overflow: hidden;
}

.logo-icon {
  width: 32px;
  height: 32px;
  color: var(--accent-primary);
  flex-shrink: 0;
}

.logo-text {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 700;
  white-space: nowrap;
  letter-spacing: -0.02em;
}

.logo-text .accent {
  color: var(--accent-primary);
}

.collapse-btn {
  color: var(--text-muted);
  flex-shrink: 0;
}

.collapse-btn:hover {
  color: var(--accent-primary);
  background: rgba(37, 99, 235, 0.1);
}

.sidebar-nav {
  flex: 1;
  padding: var(--space-3);
  border: none;
}

:deep(.el-menu--collapse) {
  width: 100%;
}

:deep(.el-menu-item) {
  height: 44px;
  line-height: 44px;
  margin: 2px 0;
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  transition: all 0.2s var(--ease-out);
}

:deep(.el-menu-item:hover) {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(37, 99, 235, 0.05));
  color: var(--accent-primary);
  border-left: 2px solid var(--accent-primary);
}

:deep(.el-menu-item .el-icon) {
  font-size: 1.1rem;
}

.sidebar-footer {
  padding: var(--space-4);
  border-top: 1px solid var(--border-default);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.system-status {
  padding: var(--space-2) var(--space-3);
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.status-dot {
  width: 8px;
  height: 8px;
  background: var(--accent-success);
  border-radius: 50%;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

.status-text {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  color: var(--accent-success);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.user-profile {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2);
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.user-avatar {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-sm);
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-display);
  font-weight: 700;
  color: var(--bg-deepest);
  font-size: 0.9rem;
  flex-shrink: 0;
}

.user-info {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.user-name {
  font-family: var(--font-display);
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-role {
  font-family: var(--font-mono);
  font-size: 0.65rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.logout-btn {
  color: var(--text-muted);
  width: 100%;
  justify-content: flex-start;
  padding: var(--space-2) var(--space-3);
}

.logout-btn:hover {
  color: var(--accent-danger);
  background: rgba(248, 113, 113, 0.1);
}

.content-wrapper {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.header {
  height: auto;
  padding: var(--space-4) var(--space-6);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-default);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-6);
}

.header-left {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.page-title {
  font-family: var(--font-display);
  font-size: 1.5rem;
  font-weight: 700;
  margin: 0;
  background: linear-gradient(135deg, var(--text-primary), var(--text-secondary));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

:deep(.el-breadcrumb) {
  font-size: 0.75rem;
}

:deep(.el-breadcrumb__item) {
  color: var(--text-muted);
}

:deep(.el-breadcrumb__inner) {
  color: var(--text-muted);
}

:deep(.el-breadcrumb__separator) {
  color: var(--text-disabled);
}

.header-right {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.header-actions {
  display: flex;
  gap: var(--space-2);
}

.notification-badge {
  position: absolute;
  top: -4px;
  right: -4px;
  min-width: 16px;
  height: 16px;
  background: var(--accent-danger);
  border-radius: var(--radius-full);
  font-size: 0.6rem;
  font-weight: 600;
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 4px;
}

.header-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
}

.user-menu {
  display: flex;
  align-items: center;
}

.user-avatar-small {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-display);
  font-weight: 700;
  color: var(--bg-deepest);
  font-size: 0.8rem;
  cursor: pointer;
  transition: transform 0.2s ease;
}

.user-avatar-small:hover {
  transform: scale(1.05);
}

.main-content {
  flex: 1;
  padding: var(--space-6);
  overflow: auto;
  background: var(--bg-primary);
  background-image:
    radial-gradient(circle at 50% 0%, rgba(34, 211, 238, 0.03) 0%, transparent 50%),
    linear-gradient(rgba(34, 211, 238, 0.02) 1px, transparent 1px),
    linear-gradient(90deg, rgba(34, 211, 238, 0.02) 1px, transparent 1px);
  background-size: 100% 100%, 32px 32px, 32px 32px;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
