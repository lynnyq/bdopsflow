<template>
  <el-container class="main-layout">
    <el-aside class="sidebar" :width="isCollapse ? '72px' : sidebarExpandedWidth">
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
        v-model:openeds="openKeys"
        :collapse="isCollapse"
        :unique-opened="true"
        router
        class="sidebar-nav"
      >
        <template v-for="menu in visibleMenus" :key="menu.key">
          <el-sub-menu v-if="menu.children && menu.children.length > 0" :index="menu.key">
            <template #title>
              <el-icon><component :is="menu.icon" /></el-icon>
              <span>{{ menu.label }}</span>
            </template>
            <el-menu-item v-for="child in getVisibleChildren(menu)" :key="child.key" :index="child.path">
              <el-icon><component :is="child.icon" /></el-icon>
              <template #title>{{ child.label }}</template>
            </el-menu-item>
          </el-sub-menu>
          <el-menu-item v-else :index="menu.path">
            <el-icon><component :is="menu.icon" /></el-icon>
            <template #title>{{ menu.label }}</template>
          </el-menu-item>
        </template>
      </el-menu>

      <div class="sidebar-footer">
        <div class="system-status">
          <div class="status-indicator">
            <span class="status-dot" :class="{ healthy: systemHealthy, unhealthy: !systemHealthy }"></span>
            <Transition name="fade">
              <span v-if="!isCollapse" class="status-text">
                {{ systemHealthy ? '系统运行正常' : '系统异常' }}
              </span>
            </Transition>
          </div>
        </div>
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
          <el-dropdown v-if="authStore.domains.length > 1" @command="handleSwitchDomain" class="domain-switcher">
            <span class="domain-switcher-trigger">
              {{ currentDomainName }} <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  v-for="domain in authStore.domains"
                  :key="domain.domain_id"
                  :command="domain.domain_id"
                  :disabled="domain.domain_id === authStore.currentDomainId"
                >
                  {{ domain.domain_name }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <div class="header-actions">
            <el-button :icon="UserFilled" circle size="small" @click="$router.push('/profile')" />
          </div>

          <div class="header-divider"></div>

          <el-dropdown trigger="click" @command="handleCommand">
            <div class="user-menu">
              <div class="user-avatar-small">
                {{ user?.real_name?.charAt(0)?.toUpperCase() || user?.username?.charAt(0)?.toUpperCase() || 'U' }}
              </div>
              <Transition name="fade">
                <div v-if="true" class="user-info-header">
                  <div class="user-name">{{ user?.real_name || user?.username || '用户' }}</div>
                </div>
              </Transition>
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">
                  <el-icon><User /></el-icon>
                  个人设置
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { dashboardAPI } from '@/api'
import { menuPermissionMap } from '@/config/menuPermissionMap'
import type { MenuPermissionDef } from '@/types'
import {
  DataAnalysis,
  List,
  Connection,
  Cpu,
  Document,
  SwitchButton,
  Setting,
  Expand,
  Fold,
  User,
  Key,
  Grid,
  UserFilled,
  ArrowDown,
  Search,
  Operation,
  Notebook
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const isCollapse = ref(false)
const systemHealthy = ref(true)
const openKeys = ref<string[]>([])
const viewportWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1920)

const sidebarExpandedWidth = computed(() => {
  return viewportWidth.value < 1200 ? '200px' : '260px'
})

const user = computed(() => authStore.user)
const activeMenu = computed(() => route.path)

const currentDomainName = computed(() => {
  const domain = authStore.domains.find(d => d.domain_id === authStore.currentDomainId)
  return domain?.domain_name || ''
})

const visibleMenus = computed(() => {
  return menuPermissionMap.filter(menu => deriveMenuVisibility(menu))
})

function getVisibleChildren(menu: MenuPermissionDef): MenuPermissionDef[] {
  if (!menu.children) return []
  return menu.children.filter(child => deriveMenuVisibility(child))
}

function deriveMenuVisibility(menu: MenuPermissionDef): boolean {
  if (menu.children && menu.children.length > 0) {
    return menu.children.some(child => deriveMenuVisibility(child))
  }
  return menu.resources.some(r => authStore.hasAnyPermission(r))
}

const updateOpenKeys = () => {
  const path = route.path
  if (path.startsWith('/datasources') || path.startsWith('/sql-query') || path.startsWith('/query-history') || path.startsWith('/saved-sql')) {
    openKeys.value = ['data-query']
  } else if (path.startsWith('/admin/')) {
    openKeys.value = ['system-admin']
  } else {
    openKeys.value = []
  }
}

watch(() => route.path, updateOpenKeys, { immediate: true })

let healthCheckInterval: number | null = null

const checkSystemHealth = async () => {
  try {
    const response = await dashboardAPI.getHealth()
    systemHealthy.value = response.data.status === 'healthy'
  } catch {
    systemHealthy.value = false
  }
}

const pageTitle = computed(() => {
  const titles: Record<string, string> = {
    '/': '仪表盘',
    '/tasks': '任务管理',
    '/executors': '执行器集群',
    '/logs': '任务日志',
    '/profile': '个人设置',
    '/admin/users': '用户管理',
    '/admin/roles': '角色管理',
    '/admin/domains': '领域管理',
    '/admin/webhooks': 'Webhook管理',
    '/datasources': '数据源管理',
    '/datasources/create': '创建数据源',
    '/sql-query': 'SQL 查询',
    '/query-history': '查询历史',
    '/saved-sql': '已保存 SQL',
    '/admin/system-config': '系统配置',
    '/admin/audit-logs': '审计日志',
    '/workflows': '工作流管理',
  }
  if (route.path.match(/\/datasources\/\d+\/edit/)) return '编辑数据源'
  if (route.path.match(/\/datasources\/\d+\/permissions/)) return '权限管理'
  return titles[route.path] || '仪表盘'
})

const handleCommand = (command: string) => {
  if (command === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (command === 'profile') {
    router.push('/profile')
  }
}

async function handleSwitchDomain(domainId: number) {
  await authStore.switchDomain(domainId)
}

const handleResize = () => {
  viewportWidth.value = window.innerWidth
}

onMounted(() => {
  checkSystemHealth()
  healthCheckInterval = window.setInterval(checkSystemHealth, 30000)
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  if (healthCheckInterval) {
    clearInterval(healthCheckInterval)
  }
  window.removeEventListener('resize', handleResize)
})
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
  border-radius: 50%;
}

.status-dot.healthy {
  background: var(--accent-success);
  animation: pulse 2s ease-in-out infinite;
}

.status-dot.unhealthy {
  background: var(--accent-danger);
  animation: pulse-danger 1s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

@keyframes pulse-danger {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.7; transform: scale(1.2); }
}

.status-text {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.status-dot.healthy + .status-text {
  color: var(--accent-success);
}

.status-dot.unhealthy + .status-text {
  color: var(--accent-danger);
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

.domain-switcher {
  cursor: pointer;
}

.domain-switcher-trigger {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--text-secondary);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-default);
  transition: all 0.2s ease;
}

.domain-switcher-trigger:hover {
  color: var(--accent-primary);
  border-color: var(--accent-primary);
}

.header-actions {
  display: flex;
  gap: var(--space-2);
}

.header-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
}

.user-menu {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  transition: background-color 0.2s ease;
}

.user-menu:hover {
  background-color: var(--bg-tertiary);
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
  flex-shrink: 0;
}

.user-avatar-small:hover {
  transform: scale(1.05);
}

.user-info-header {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.user-info-header .user-name {
  font-family: var(--font-display);
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.main-content {
  flex: 1;
  padding: var(--space-6);
  overflow-y: auto;
  overflow-x: hidden;
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
