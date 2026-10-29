<!--
  文件描述：应用主布局组件
  组件结构：左侧可折叠侧边栏（Logo + 权限过滤菜单树 + 系统健康指示）
          + 右侧 Header（页面标题 + 面包屑 + 领域切换 + 用户下拉菜单）
          + 主内容区（<router-view /> 渲染子路由）
  核心逻辑：根据用户权限过滤可见菜单；监听窗口尺寸自动调整侧边栏宽度；
           每 30 秒轮询系统健康状态并在侧边栏底部展示
-->
<template>
  <!-- 主容器：仅当已登录用户存在时渲染整个布局 -->
  <el-container v-if="authStore.user" class="main-layout">

    <!-- 左侧侧边栏：包含 Logo、导航菜单、系统健康状态；支持折叠/展开 -->
    <el-aside class="sidebar" :width="isCollapse ? '72px' : sidebarExpandedWidth">

      <!-- 侧边栏顶部：Logo 区域 + 折叠/展开按钮 -->
      <div class="sidebar-header">

        <!-- Logo：BDopsFlow 品牌标识，折叠时仅显示图标 -->
        <div class="logo">
          <div class="logo-icon">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
              <defs>
                <linearGradient id="logo-bg" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
                  <stop stop-color="#2563EB"/>
                  <stop offset="1" stop-color="#3B82F6"/>
                </linearGradient>
              </defs>
              <rect width="32" height="32" rx="8" fill="url(#logo-bg)"/>
              <path d="M7 13C9 11 11 15 13 13C15 11 17 15 19 13C21 11 23 15 25 13" stroke="white" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M7 19C9 17 11 21 13 19C15 17 17 21 19 19C21 17 23 21 25 19" stroke="white" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" opacity="0.5"/>
            </svg>
          </div>
          <Transition name="fade">
            <div v-if="!isCollapse" class="logo-text">
              <span>BD<span class="accent">ops</span>Flow</span>
            </div>
          </Transition>
        </div>
        <!-- 侧边栏折叠/展开切换按钮 -->
        <el-button
          :icon="isCollapse ? Expand : Fold"
          text
          @click="isCollapse = !isCollapse"
          class="collapse-btn"
        />
      </div>

      <!-- 侧边栏主导航菜单：使用 router 模式，根据 activeMenu 高亮当前路由 -->
      <el-menu
        :default-active="activeMenu"
        v-model:openeds="openKeys"
        :collapse="isCollapse"
        :unique-opened="true"
        router
        class="sidebar-nav"
      >
        <!-- 遍历经过权限过滤后的可见菜单项 -->
        <template v-for="menu in visibleMenus" :key="menu.key">
          <!-- 有子菜单的渲染为 el-sub-menu，无子菜单直接渲染为 el-menu-item -->
          <el-sub-menu v-if="menu.children && menu.children.length > 0" :index="menu.key">
            <template #title>
              <el-icon><component :is="menu.icon" /></el-icon>
              <span>{{ menu.label }}</span>
            </template>
            <!-- 子菜单项同样经过权限过滤 -->
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

      <!-- 侧边栏底部：系统健康状态指示灯，折叠时隐藏文字 -->
      <div class="sidebar-footer">
        <div class="system-status">
          <div class="status-indicator">
            <!-- 状态指示灯：healthy 绿色正常，unhealthy 红色异常 -->
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

    <!-- 右侧内容区：Header + Main -->
    <el-container class="content-wrapper">

      <!-- 顶部 Header：页面标题 + 面包屑 + 领域切换 + 用户信息下拉菜单 -->
      <el-header class="header">

        <!-- Header 左侧：页面大标题 + 面包屑导航 -->
        <div class="header-left">
          <h1 class="page-title">{{ pageTitle }}</h1>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item>{{ pageTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <!-- Header 右侧：领域切换 + 用户中心入口 + 用户下拉菜单 -->
        <div class="header-right">
          <!-- 多租户领域切换：仅当用户属于多个领域时显示 -->
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

          <!-- 用户中心快捷入口按钮 -->
          <div class="header-actions">
            <el-button :icon="UserFilled" circle size="small" @click="$router.push('/profile')" />
          </div>

          <!-- 分隔线 -->
          <div class="header-divider"></div>

          <!-- 用户信息下拉菜单：显示用户名、个人设置、退出登录 -->
          <el-dropdown trigger="click" @command="handleCommand">
            <div class="user-menu">
              <!-- 用户头像：取真实姓名或用户名首字母大写 -->
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

      <!-- 主内容区域：通过 router-view 渲染子路由页面 -->
      <el-main class="main-content">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
/**
 * Layout.vue - 应用主布局组件
 *
 * 结构：左侧可折叠侧边栏（Logo + 权限过滤后的菜单树 + 系统健康状态）
 *      + 右侧 Header（面包屑 + 用户下拉）+ 主内容区（<router-view />）
 * 功能：根据 authStore.permissions 过滤侧边菜单；监听窗口宽度自动调整侧边栏宽度；
 *       挂载时轮询系统健康状态。
 *
 * @component
 */
// ========== Vue 核心 API ==========
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
// ========== Vue Router ==========
import { useRoute, useRouter } from 'vue-router'
// ========== Pinia Store ==========
import { useAuthStore } from '@/stores/auth'
// ========== API 层 ==========
import { dashboardAPI } from '@/api'
// ========== 配置/常量 ==========
import { menuPermissionMap } from '@/config/menuPermissionMap'
// ========== 类型定义 ==========
import type { MenuPermissionDef } from '@/types'
// ========== Element Plus 图标 ==========
import {
  SwitchButton,  // 退出按钮图标
  Expand,        // 展开侧边栏图标
  Fold,          // 折叠侧边栏图标
  User,          // 用户图标
  UserFilled,    // 用户填充图标
  ArrowDown      // 下拉箭头图标
} from '@element-plus/icons-vue'

// ========== 路由/Store 实例 ==========
const route = useRoute()              // 当前路由信息，用于高亮菜单和页面标题
const router = useRouter()            // 路由实例，用于跳转
const authStore = useAuthStore()      // 认证状态 Store，包含用户信息、权限、领域等

// ========== 响应式状态 ==========
const isCollapse = ref(false)                    // 侧边栏是否折叠
const systemHealthy = ref(true)                  // 系统健康状态：true 正常，false 异常
const openKeys = ref<string[]>([])               // 当前展开的子菜单 key 列表
const viewportWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1920)  // 视口宽度，用于响应式调整

// ========== 计算属性 ==========

/** 根据视口宽度动态计算侧边栏展开时的宽度：小屏 200px，大屏 260px */
const sidebarExpandedWidth = computed(() => {
  return viewportWidth.value < 1200 ? '200px' : '260px'
})

/** 当前登录用户信息 */
const user = computed(() => authStore.user)
/** 当前激活的菜单路径，与路由 path 保持一致 */
const activeMenu = computed(() => route.path)

/** 当前选中的领域名称，用于 Header 领域切换器显示 */
const currentDomainName = computed(() => {
  const domain = authStore.domains.find(d => d.domain_id === authStore.currentDomainId)
  return domain?.domain_name || ''
})

/** 根据用户权限过滤后的可见顶级菜单项列表 */
const visibleMenus = computed(() => {
  return menuPermissionMap.filter(menu => deriveMenuVisibility(menu))
})

// ========== 工具函数 ==========

/**
 * 递归获取某个菜单项下的可见子菜单
 * @param menu 菜单权限定义
 * @returns 经过权限过滤的子菜单数组
 */
function getVisibleChildren(menu: MenuPermissionDef): MenuPermissionDef[] {
  if (!menu.children) return []
  return menu.children.filter(child => deriveMenuVisibility(child))
}

/**
 * 递归判断某个菜单项是否对当前用户可见
 * 规则：有子菜单时只要存在一个可见子菜单就可见；无子菜单时检查 resources 是否有任一权限匹配
 * @param menu 菜单权限定义
 * @returns 是否可见
 */
function deriveMenuVisibility(menu: MenuPermissionDef): boolean {
  if (menu.children && menu.children.length > 0) {
    return menu.children.some(child => deriveMenuVisibility(child))
  }
  return menu.resources.some(r => authStore.hasAnyPermission(r))
}

/**
 * 根据当前路由 path 更新需要展开的子菜单（openKeys）
 * 使用 manual 模式控制 el-menu 的展开状态
 */
const updateOpenKeys = () => {
  const path = route.path
  if (path.startsWith('/datasources') || path.startsWith('/sql-query') || path.startsWith('/query-history') || path.startsWith('/saved-sql')) {
    openKeys.value = ['data-query']
  } else if (path.startsWith('/interface/')) {
    openKeys.value = ['interface']
  } else if (path.startsWith('/admin/')) {
    openKeys.value = ['system-admin']
  } else {
    openKeys.value = []
  }
}

// ========== Watch 监听 ==========
/** 监听路由变化，自动展开对应的子菜单 */
watch(() => route.path, updateOpenKeys, { immediate: true })

// ========== 系统健康检查 ==========
let healthCheckInterval: number | null = null  // 健康检查定时器 ID

/**
 * 调用后端健康检查接口更新系统状态
 * 请求失败时标记为异常
 */
const checkSystemHealth = async () => {
  try {
    const response = await dashboardAPI.getHealth()
    systemHealthy.value = response.data.status === 'healthy'
  } catch {
    systemHealthy.value = false
  }
}

// ========== 页面标题计算 ==========

/**
 * 根据当前路由 path 映射页面标题文案
 * 特殊处理数据源编辑页、权限管理页等带动态 ID 的路径
 */
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
    '/admin/grafana-dashboards': 'Grafana 图表',
    '/datasources': '数据源管理',
    '/datasources/create': '创建数据源',
    '/sql-query': 'SQL 查询',
    '/query-history': '查询历史',
    '/saved-sql': '已保存 SQL',
    '/interface/http': 'HTTP 测试',
    '/interface/grpc': 'gRPC 测试',
    '/interface/proto-files': 'Proto 文件',
    '/interface/certificates': '证书管理',
    '/interface/history': '执行历史',
    '/admin/system-config': '系统配置',
    '/admin/audit-logs': '审计日志',
  }
  // 正则匹配动态路由
  if (route.path.match(/\/datasources\/\d+\/edit/)) return '编辑数据源'
  if (route.path.match(/\/datasources\/\d+\/permissions/)) return '权限管理'
  return titles[route.path] || '仪表盘'
})

// ========== 事件处理 ==========

/**
 * 用户下拉菜单命令处理
 * @param command 菜单命令：'logout' 退出登录，'profile' 跳转个人设置
 */
const handleCommand = (command: string) => {
  if (command === 'logout') {
    authStore.logout()
    router.push('/login')
  } else if (command === 'profile') {
    router.push('/profile')
  }
}

/**
 * 切换当前租户领域
 * @param domainId 目标领域 ID
 */
async function handleSwitchDomain(domainId: number) {
  await authStore.switchDomain(domainId)
}

/** 窗口 resize 事件处理：同步更新视口宽度 */
const handleResize = () => {
  viewportWidth.value = window.innerWidth
}

// ========== 生命周期 ==========

onMounted(() => {
  checkSystemHealth()                                    // 组件挂载后立即检查一次健康状态
  healthCheckInterval = window.setInterval(checkSystemHealth, 30000)  // 每 30 秒轮询一次
  window.addEventListener('resize', handleResize)         // 监听窗口尺寸变化
})

onUnmounted(() => {
  if (healthCheckInterval) {
    clearInterval(healthCheckInterval)                    // 组件卸载时清除定时器
  }
  window.removeEventListener('resize', handleResize)      // 移除窗口监听
})
</script>

<style scoped>
/* ========== 主容器 ========== */
.main-layout {
  min-height: 100vh;
  background-color: var(--bg-primary);
}

/* ========== 侧边栏区域 ========== */
.sidebar {
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-default);
  display: flex;
  flex-direction: column;
  transition: width 0.3s var(--ease-out);  /* 折叠/展开平滑过渡 */
  overflow: hidden;
}

/* 侧边栏顶部：Logo + 折叠按钮 */
.sidebar-header {
  padding: var(--space-4);
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-default);
  min-height: 64px;
}

/* Logo 容器 */
.logo {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  overflow: hidden;
}

.logo-icon {
  width: 32px;
  height: 32px;
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
  color: var(--accent-primary);  /* "ops" 高亮色 */
}

/* 侧边栏折叠按钮 */
.collapse-btn {
  color: var(--text-muted);
  flex-shrink: 0;
}

.collapse-btn:hover {
  color: var(--accent-primary);
  background: rgba(37, 99, 235, 0.1);
}

/* ========== 侧边栏导航菜单 ========== */
.sidebar-nav {
  flex: 1;
  padding: var(--space-3);
  border: none;
}

:deep(.el-menu--collapse) {
  width: 100%;
}

/* 菜单项基础样式 */
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

/* 当前激活菜单项：左侧蓝色竖条 + 渐变背景 */
:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(37, 99, 235, 0.05));
  color: var(--accent-primary);
  border-left: 2px solid var(--accent-primary);
}

:deep(.el-menu-item .el-icon) {
  font-size: 1.1rem;
}

/* ========== 侧边栏底部：系统健康状态 ========== */
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

/* 状态指示灯圆点 */
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

/* 健康状态：绿色脉冲动画 */
.status-dot.healthy {
  background: var(--accent-success);
  animation: pulse 2s ease-in-out infinite;
}

/* 异常状态：红色快速脉冲 */
.status-dot.unhealthy {
  background: var(--accent-danger);
  animation: pulse-danger 1s ease-in-out infinite;
}

/* 正常脉冲动画：呼吸效果 */
@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

/* 异常脉冲动画：更快的呼吸效果 */
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

/* ========== 右侧内容区 ========== */
.content-wrapper {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

/* ========== 顶部 Header 区域 ========== */
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

/* Header 左侧：标题 + 面包屑 */
.header-left {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

/* 页面标题：渐变文字效果 */
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

/* 面包屑样式微调 */
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

/* Header 右侧：领域切换 + 用户菜单 */
.header-right {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

/* 领域切换器 */
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

/* Header 右侧按钮组 */
.header-actions {
  display: flex;
  gap: var(--space-2);
}

/* Header 分隔线 */
.header-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
}

/* 用户下拉菜单容器 */
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

/* 用户头像：渐变色背景 + 首字母 */
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
  transform: scale(1.05);  /* hover 微放大效果 */
}

/* 用户信息区域：折叠时隐藏 */
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

/* ========== 主内容区 ========== */
.main-content {
  flex: 1;
  padding: var(--space-6);
  overflow-y: auto;
  overflow-x: hidden;
  background: var(--bg-primary);
  /* 网格背景装饰效果 */
  background-image:
    radial-gradient(circle at 50% 0%, rgba(34, 211, 238, 0.03) 0%, transparent 50%),
    linear-gradient(rgba(34, 211, 238, 0.02) 1px, transparent 1px),
    linear-gradient(90deg, rgba(34, 211, 238, 0.02) 1px, transparent 1px);
  background-size: 100% 100%, 32px 32px, 32px 32px;
}

/* ========== 过渡动画 ========== */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
