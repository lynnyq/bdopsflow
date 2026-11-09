<!--
  首页仪表盘组件
  功能：展示系统健康状态、执行趋势图、调度器控制、快捷导航、Grafana 监控图表
  主要区域：系统健康、调度器控制、执行趋势、快捷导航、Grafana 嵌入面板
  自动刷新：统计数据 30s，健康检查 60s
-->
<template>
  <div class="dashboard">
    <!-- Health Check -->
    <section class="health-section">
      <div class="section-header">
        <h2 class="section-title">系统健康状态</h2>
        <span class="health-status" :class="healthStatusClass">
          <span class="status-dot"></span>
          {{ healthData?.status === 'healthy' ? '健康' : '异常' }}
        </span>
      </div>
      <div class="health-grid">
        <div 
          v-for="(check, key) in healthData?.components" 
          :key="key"
          class="health-card"
          :class="{ healthy: check.status === 'healthy', unhealthy: check.status !== 'healthy' }"
        >
          <div class="health-card-header">
            <el-icon :size="20">
              <component :is="getComponentIcon(String(key))" />
            </el-icon>
            <span class="health-card-title">{{ getComponentName(String(key)) }}</span>
          </div>
          <div class="health-card-body">
            <div class="health-status-badge" :class="check.status">
              {{ key === 'scheduler' && check.message === '已暂停' ? '已暂停' : (check.status === 'healthy' ? (key === 'scheduler' ? '运行中' : '正常') : '异常') }}
            </div>
            <div class="health-message">{{ check.message }}</div>
            <div v-if="check.latency" class="health-latency">
              响应时间: {{ check.latency }}
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Scheduler Control -->
    <section class="scheduler-section">
      <div class="scheduler-info">
        <div class="scheduler-status" :class="{ paused: schedulerStatus.paused }">
          <span class="status-dot"></span>
          <span class="status-text">{{ schedulerStatus.paused ? '调度器已暂停' : '调度器运行中' }}</span>
        </div>
        <span class="scheduler-hint">暂停调度将停止所有定时任务的自动执行</span>
      </div>
      <div class="scheduler-actions">
        <el-button 
          v-if="canControlScheduler"
          :icon="VideoPlay" 
          type="success"
          @click="handleResumeScheduler"
          :disabled="!schedulerStatus.paused"
          :loading="actionLoading"
        >
          恢复调度
        </el-button>
        <el-button 
          v-if="canControlScheduler"
          :icon="VideoPause" 
          type="warning"
          @click="handlePauseScheduler"
          :disabled="schedulerStatus.paused"
          :loading="actionLoading"
        >
          暂停调度
        </el-button>
        <el-button
          :icon="Refresh"
          @click="handleRefreshAll"
          :loading="loading"
          class="refresh-btn"
        >
          刷新数据
        </el-button>
      </div>
    </section>

    <!-- Trend Chart -->
    <section class="trend-section">
      <div class="section-header">
        <h2 class="section-title">执行趋势（最近7天）</h2>
      </div>
      <div class="trend-chart">
        <div v-if="trends.length === 0" class="trend-empty">
          <el-icon :size="48"><DataLine /></el-icon>
          <p>暂无趋势数据</p>
        </div>
        <div v-else class="trend-bars">
          <div 
            v-for="trend in trends" 
            :key="trend.date" 
            class="trend-bar-item"
          >
            <div class="bar-wrapper">
              <div 
                class="bar success" 
                :style="{ height: getBarHeight(trend.success) + 'px' }"
                :title="`成功: ${trend.success}`"
              ></div>
              <div 
                class="bar failed" 
                :style="{ height: getBarHeight(trend.failed) + 'px' }"
                :title="`失败: ${trend.failed}`"
              ></div>
            </div>
            <div class="bar-label">{{ formatDate(trend.date) }}</div>
            <div class="bar-value">
              <span class="success-text">{{ trend.success }}</span>
              <span class="separator">/</span>
              <span class="failed-text">{{ trend.failed }}</span>
            </div>
          </div>
        </div>
        <div v-if="trends.length > 0" class="trend-legend">
          <span class="legend-item">
            <span class="legend-dot success"></span>
            成功
          </span>
          <span class="legend-item">
            <span class="legend-dot failed"></span>
            失败
          </span>
        </div>
      </div>
    </section>

    <!-- 快捷导航卡片：Dashboard 首页展示已启用的导航链接，3 列网格 -->
    <section v-if="navigationLinks.length > 0" class="navigation-section">
      <div class="section-header">
        <h2 class="section-title">
          <el-icon><Link /></el-icon>
          快捷导航
        </h2>
      </div>
      <div class="nav-grid">
        <a
          v-for="item in navigationLinks"
          :key="item.id"
          :href="item.url"
          target="_blank"
          rel="noopener"
          class="nav-card"
          :class="[`nav-width-${item.width}`, `nav-mode-${item.display_mode}`]"
          :style="{ height: cardHeight(item) + 'px' }"
        >
          <div
            class="nav-icon"
            :style="item.icon_data ? {} : { backgroundColor: fallbackColor(item.url) }"
          >
            <img
              v-if="item.icon_data"
              :src="item.icon_data"
              :alt="item.name"
              @error="onNavIconError"
            />
            <span v-else>{{ fallbackInitial(item.url) }}</span>
          </div>
          <div class="nav-info">
            <div class="nav-name" :title="item.name">{{ item.name }}</div>
            <div
              v-if="item.display_mode === 'banner' && item.description"
              class="nav-desc"
              :title="item.description"
            >{{ item.description }}</div>
            <div v-if="item.display_mode !== 'compact'" class="nav-domain">{{ extractDomain(item.url) }}</div>
          </div>
          <el-icon class="nav-open-icon"><TopRight /></el-icon>
        </a>
      </div>
    </section>

    <!-- Grafana 嵌入图表 -->
    <section v-if="grafanaDashboards.length > 0" class="grafana-section">
      <div class="section-header">
        <h2 class="section-title">
          <el-icon><DataLine /></el-icon>
          Grafana 监控图表
        </h2>
        <el-tooltip content="刷新图表" placement="top">
          <el-button :icon="Refresh" circle size="small" @click="handleRefreshGrafana" />
        </el-tooltip>
      </div>
      <div class="grafana-grid">
        <div
          v-for="item in grafanaDashboards"
          :key="item.id"
          class="grafana-panel"
          :class="`grafana-width-${item.width}`"
        >
          <div class="grafana-panel-header">
            <span class="grafana-panel-title" :title="item.name">{{ item.name }}</span>
            <el-tooltip content="在新窗口打开" placement="top">
              <el-icon class="grafana-open-icon" @click="openGrafanaPanel(item)">
                <TopRight />
              </el-icon>
            </el-tooltip>
          </div>
          <iframe
            :key="`${item.id}-${iframeRefreshKey}`"
            :src="item.embed_url"
            :style="{ height: item.height_px + 'px' }"
            class="grafana-iframe"
            sandbox="allow-scripts allow-same-origin allow-popups allow-forms"
            loading="lazy"
            referrerpolicy="no-referrer"
          />
        </div>
      </div>
    </section>
  </div>
</template>

/**
 * 首页仪表盘脚本
 * @description 负责系统健康检查、执行趋势展示、调度器暂停/恢复控制、快捷导航，以及 Grafana 监控图表嵌入
 *
 * 自动刷新策略：
 * - 趋势/调度器状态：30 秒
 * - 健康检查：60 秒
 *
 * 依赖：
 * - dashboardAPI: 仪表盘后端接口
 * - grafanaAPI: Grafana 嵌入面板接口
 * - navigationAPI: 快捷导航链接接口
 */
<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoPlay, VideoPause, Refresh, DataLine, TopRight, Link } from '@element-plus/icons-vue'
import { dashboardAPI, grafanaAPI, navigationAPI, type HealthCheckResult } from '@/api'
import { isHandledError } from '@/utils/api'
import { useAuthStore } from '@/stores/auth'
import type { TrendData, GrafanaDashboard, NavigationLink } from '@/types'

// 认证 store：用于判断是否有调度器控制权
const authStore = useAuthStore()

// 是否有调度器控制权限（admin / system_admin 角色）
const canControlScheduler = computed(() => {
  return authStore.roleCodes.includes('admin') || authStore.roleCodes.includes('system_admin')
})

const loading = ref(false)          // 数据加载状态
const actionLoading = ref(false)     // 调度器操作（暂停/恢复）加载状态
const schedulerStatus = ref({ paused: false })  // 调度器当前暂停状态
const trends = ref<TrendData[]>([])              // 最近 7 天执行趋势数据
const healthData = ref<HealthCheckResult | null>(null)  // 系统健康检查结果

// 定时器句柄：用于 onUnmounted 时清理
let refreshInterval: number | null = null
let healthInterval: number | null = null

// 健康状态 CSS 类：healthy / unhealthy
const healthStatusClass = computed(() => {
  return healthData.value?.status === 'healthy' ? 'healthy' : 'unhealthy'
})

/**
 * 获取健康组件 key 对应的中文名称
 * @param key - 组件标识（rqlite/redis/scheduler/leader）
 * @returns 中文名称
 */
const getComponentName = (key: string): string => {
  const names: Record<string, string> = {
    rqlite: '数据库',
    redis: 'Redis 缓存',
    scheduler: '任务调度器',
    leader: '主调度节点'
  }
  return names[key] || key
}

/**
 * 获取健康组件 key 对应的图标名称
 * @param key - 组件标识
 * @returns 图标组件名称
 */
const getComponentIcon = (key: string): string => {
  const icons: Record<string, string> = {
    rqlite: 'DataLine',
    redis: 'Bell',
    scheduler: 'Monitor',
    leader: 'Cpu'
  }
  return icons[key] || 'Monitor'
}

/**
 * 加载系统健康检查数据（静默失败，不影响主流程）
 */
const loadHealthData = async () => {
  try {
    const response = await dashboardAPI.getHealth()
    healthData.value = response.data
  } catch (error) {
  }
}

/**
 * 加载调度器当前暂停状态
 */
const loadSchedulerStatus = async () => {
  try {
    const response = await dashboardAPI.getSchedulerStatus()
    schedulerStatus.value = {
      paused: response.data?.paused ?? false
    }
  } catch (error) {
  }
}

/**
 * 加载最近 7 天执行趋势数据
 */
const loadTrends = async () => {
  try {
    const response = await dashboardAPI.getTrends()
    const data = response.data || {}
    trends.value = (data.items || []).slice().sort((a: TrendData, b: TrendData) => a.date.localeCompare(b.date))
  } catch (error) {
  }
}

/**
 * 刷新所有主数据：并行加载调度器状态、趋势
 */
const refreshData = async () => {
  loading.value = true
  try {
    await Promise.all([
      loadSchedulerStatus(),
      loadTrends(),
    ])
  } finally {
    loading.value = false
  }
}

/**
 * 暂停调度器：停止所有定时任务自动执行
 */
const handlePauseScheduler = async () => {
  actionLoading.value = true
  try {
    await dashboardAPI.pauseScheduler()
    ElMessage.success('调度器已暂停')
    await loadSchedulerStatus()
  } catch (error) {
    if (!isHandledError(error)) {
      ElMessage.error('暂停调度器失败')
    }
  } finally {
    actionLoading.value = false
  }
}

/**
 * 恢复调度器：重启定时任务自动执行
 */
const handleResumeScheduler = async () => {
  actionLoading.value = true
  try {
    await dashboardAPI.resumeScheduler()
    ElMessage.success('调度器已恢复')
    await loadSchedulerStatus()
  } catch (error) {
    if (!isHandledError(error)) {
      ElMessage.error('恢复调度器失败')
    }
  } finally {
    actionLoading.value = false
  }
}

/**
 * 根据趋势数据值计算柱状图高度（相对于最大值的百分比）
 * @param value - 单日成功或失败数量
 * @returns 柱高像素值（最小 10px）
 */
const getBarHeight = (value: number): number => {
  if (!trends.value || trends.value.length === 0) return 0
  const validTrends = trends.value.filter((t: TrendData) => t && typeof t.success === 'number' && typeof t.failed === 'number')
  if (validTrends.length === 0) return 0
  const maxValue = Math.max(...validTrends.map((t: TrendData) => Math.max(t.success, t.failed)))
  if (maxValue === 0) return 0
  const safeValue = typeof value === 'number' ? value : 0
  return Math.max(10, (safeValue / maxValue) * 100)
}

/**
 * 格式化日期为 M/D 短格式
 * @param date - ISO 日期字符串
 * @returns 如 "6/30"
 */
const formatDate = (date: string): string => {
  if (!date) return ''
  const d = new Date(date)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

// Grafana 嵌入图表相关
const grafanaDashboards = ref<GrafanaDashboard[]>([])  // 已启用的 Grafana 面板列表
// iframe 刷新键：变更后强制 Vue 销毁重建 Grafana iframe，使图表真正重新加载
const iframeRefreshKey = ref(0)

// ========== 导航快捷链接 ==========
const navigationLinks = ref<NavigationLink[]>([])  // 已启用的导航链接列表
let navInterval: number | null = null              // 5 分钟刷新定时器

// display_mode 基准高度（px），height_px 为 null 或 0 时使用
const DISPLAY_MODE_BASE_HEIGHT: Record<string, number> = {
  compact: 52,
  normal:  68,
  banner:  100,
}

// fallback 色板：域名 hash 稳定取色
const FALLBACK_COLORS = ['#f46800', '#e6522c', '#00bfb3', '#24292f', '#d33833', '#0052cc', '#9333ea']

/** 计算卡片高度：微调值优先，否则用 display_mode 基准 */
const cardHeight = (item: NavigationLink): number => {
  if (item.height_px && item.height_px > 0) return item.height_px
  return DISPLAY_MODE_BASE_HEIGHT[item.display_mode] ?? 68
}

/** URL 域名 hash 选色 */
const fallbackColor = (url: string): string => {
  try {
    const host = new URL(url).hostname
    let h = 0
    for (const c of host) h = (h * 31 + c.charCodeAt(0)) >>> 0
    return FALLBACK_COLORS[h % FALLBACK_COLORS.length]
  } catch {
    return '#6b7280'
  }
}

/** URL 域名首字母 */
const fallbackInitial = (url: string): string => {
  try {
    const host = new URL(url).hostname.replace(/^www\./, '')
    return host.charAt(0).toUpperCase()
  } catch {
    return '?'
  }
}

/** 提取域名展示 */
const extractDomain = (url: string): string => {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

/** favicon img 加载失败时，让 img.style.display = 'none'，fallback span 自动显示 */
const onNavIconError = (e: Event) => {
  const el = e.target as HTMLImageElement
  el.style.display = 'none'
}

/** 加载已启用的导航链接（静默失败，不影响其他模块） */
const loadNavigationLinks = async () => {
  try {
    const response = await navigationAPI.listEnabled()
    navigationLinks.value = response.data?.items || []
  } catch (error) {
    console.warn('Failed to load navigation links:', error)
    navigationLinks.value = []
  }
}

/**
 * 强制刷新所有 Grafana iframe（递增 key 触发 Vue 重建）
 */
const refreshGrafanaCharts = () => {
  iframeRefreshKey.value++
}

/**
 * Grafana 面板单独刷新：先拉取列表再重建 iframe
 */
const handleRefreshGrafana = async () => {
  await loadGrafanaDashboards()
  refreshGrafanaCharts()
}

// 主"刷新数据"按钮：刷新统计数据并强制重载 Grafana 图表
// 30s 自动定时刷新只调用 refreshData，不重建 iframe，避免图表频繁闪烁
const handleRefreshAll = async () => {
  await refreshData()
  refreshGrafanaCharts()
}

/**
 * 加载已启用的 Grafana 面板列表（静默失败）
 */
const loadGrafanaDashboards = async () => {
  try {
    const response = await grafanaAPI.listEnabled()
    grafanaDashboards.value = response.data?.items || []
  } catch (error) {
    // 静默处理，不影响 Dashboard 其他统计展示
    console.warn('Failed to load grafana dashboards:', error)
    grafanaDashboards.value = []
  }
}

/**
 * 在新标签页打开 Grafana 面板
 * @param {GrafanaDashboard} item - Grafana 面板数据
 */
const openGrafanaPanel = (item: GrafanaDashboard) => {
  window.open(item.embed_url, '_blank', 'noopener')
}

onMounted(() => {
  refreshData()
  loadHealthData()
  loadGrafanaDashboards()
  loadNavigationLinks()
  refreshInterval = window.setInterval(refreshData, 30000)
  healthInterval = window.setInterval(loadHealthData, 60000)
  navInterval = window.setInterval(loadNavigationLinks, 5 * 60 * 1000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  if (healthInterval) {
    clearInterval(healthInterval)
  }
  if (navInterval) {
    clearInterval(navInterval)
  }
})
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
  padding-bottom: var(--space-6);
}

/* Health Section */
.health-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  box-shadow: var(--shadow-sm);
}

.health-section .section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-4);
}

.health-status {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  font-weight: 500;
  font-size: 0.875rem;
}

.health-status.healthy {
  background: rgba(52, 211, 153, 0.1);
  color: var(--accent-success);
}

.health-status.unhealthy {
  background: rgba(248, 113, 113, 0.1);
  color: var(--accent-danger);
}

.health-status .status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.health-status.healthy .status-dot {
  background: var(--accent-success);
}

.health-status.unhealthy .status-dot {
  background: var(--accent-danger);
}

.health-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

@media (max-width: 1200px) {
  .health-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 600px) {
  .health-grid {
    grid-template-columns: 1fr;
  }
}

.health-card {
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: var(--space-4);
  border: 1px solid var(--border-subtle);
  transition: all var(--duration-normal) var(--ease-out);
}

.health-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.health-card.healthy {
  border-left: 3px solid var(--accent-success);
}

.health-card.unhealthy {
  border-left: 3px solid var(--accent-danger);
}

.health-card-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
  color: var(--text-primary);
}

.health-card-title {
  font-weight: 600;
  font-size: 0.9rem;
}

.health-card-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.health-status-badge {
  display: inline-flex;
  align-items: center;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  font-size: 0.75rem;
  font-weight: 500;
  width: fit-content;
}

.health-status-badge.healthy {
  background: rgba(52, 211, 153, 0.15);
  color: var(--accent-success);
}

.health-status-badge.unhealthy {
  background: rgba(248, 113, 113, 0.15);
  color: var(--accent-danger);
}

.health-message {
  font-size: 0.8rem;
  color: var(--text-secondary);
  word-break: break-word;
}

.health-latency {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

/* Scheduler Section */
.scheduler-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-5);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  gap: var(--space-4);
  flex-wrap: wrap;
}

.scheduler-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.scheduler-status {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.status-dot {
  width: 12px;
  height: 12px;
  background: var(--accent-success);
  border-radius: 50%;
  animation: pulse 2s ease-in-out infinite;
}

.scheduler-status.paused .status-dot {
  background: var(--accent-warning);
  animation: none;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

.status-text {
  font-family: var(--font-display);
  font-weight: 600;
  font-size: 1.1rem;
  color: var(--text-primary);
}

.scheduler-hint {
  font-size: 0.8rem;
  color: var(--text-muted);
}

.scheduler-actions {
  display: flex;
  gap: var(--space-3);
  flex-shrink: 0;
}

.refresh-btn {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
}

.refresh-btn:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

/* Trend Section */
.trend-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  box-shadow: var(--shadow-sm);
}

/* Grafana 嵌入图表 */
.grafana-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  box-shadow: var(--shadow-sm);
}

.grafana-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
}

.grafana-panel {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.grafana-width-third {
  grid-column: span 1;
}

.grafana-width-half {
  grid-column: span 1;
}

.grafana-width-full {
  grid-column: span 3;
}

@media (min-width: 992px) {
  .grafana-width-half {
    grid-column: span 2;
  }
}

@media (max-width: 768px) {
  .grafana-grid {
    grid-template-columns: 1fr;
  }
  .grafana-width-third,
  .grafana-width-half {
    grid-column: span 1;
  }
}

.grafana-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-2) var(--space-3);
  background: var(--bg-card);
  border-bottom: 1px solid var(--border-subtle);
}

.grafana-panel-title {
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 80%;
}

.grafana-open-icon {
  cursor: pointer;
  color: var(--text-secondary);
  transition: color 0.2s;
}

.grafana-open-icon:hover {
  color: var(--color-primary);
}

.grafana-iframe {
  width: 100%;
  border: none;
  display: block;
}

.section-header {
  margin-bottom: var(--space-4);
}

.section-title {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.trend-chart {
  min-height: 200px;
}

.trend-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--text-muted);
  gap: var(--space-3);
}

.trend-bars {
  display: flex;
  justify-content: space-around;
  align-items: flex-end;
  height: 180px;
  padding: var(--space-4) 0;
  gap: var(--space-3);
}

.trend-bar-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  flex: 1;
  max-width: 80px;
}

.bar-wrapper {
  display: flex;
  gap: 4px;
  align-items: flex-end;
  height: 100px;
}

.bar {
  width: 24px;
  border-radius: 4px 4px 0 0;
  transition: height 0.3s ease;
  min-height: 4px;
}

.bar.success {
  background: linear-gradient(to top, var(--accent-success), rgba(52, 211, 153, 0.6));
}

.bar.failed {
  background: linear-gradient(to top, var(--accent-danger), rgba(248, 113, 113, 0.6));
}

.bar-label {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
}

.bar-value {
  display: flex;
  align-items: center;
  gap: 2px;
  font-family: var(--font-mono);
  font-size: 0.7rem;
}

.success-text {
  color: var(--accent-success);
}

.failed-text {
  color: var(--accent-danger);
}

.separator {
  color: var(--text-disabled);
}

.trend-legend {
  display: flex;
  justify-content: center;
  gap: var(--space-6);
  margin-top: var(--space-4);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

.legend-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.legend-dot {
  width: 12px;
  height: 12px;
  border-radius: 3px;
}

.legend-dot.success {
  background: var(--accent-success);
}

.legend-dot.failed {
  background: var(--accent-danger);
}

@media (max-width: 768px) {
  .scheduler-section {
    flex-direction: column;
    align-items: flex-start;
  }
  
  .scheduler-actions {
    width: 100%;
    justify-content: flex-start;
  }
}

/* ============ 导航快捷链接卡片 ============ */
.navigation-section {
  background: var(--el-bg-color);
  border-radius: var(--el-border-radius-base);
  padding: 18px 20px;
  border: 1px solid var(--el-border-color-lighter);
}

.nav-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

@media (max-width: 1199px) {
  .nav-grid { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 767px) {
  .nav-grid { grid-template-columns: 1fr; }
}

/* 列宽控制：与 grafana 完全对齐 */
.nav-width-third { grid-column: span 1; }
.nav-width-half { grid-column: span 1; }
@media (min-width: 992px) {
  .nav-width-half { grid-column: span 2; }
}
.nav-width-full { grid-column: span 3; }
@media (min-width: 768px) and (max-width: 1199px) {
  .nav-width-full { grid-column: span 2; }
}

.nav-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-lighter);
  text-decoration: none;
  color: inherit;
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
  position: relative;
  overflow: hidden;
}

.nav-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.08);
  border-color: var(--el-color-primary-light-5);
}

.nav-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: #f46800;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 15px;
  overflow: hidden;
}
.nav-icon img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.nav-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--el-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-domain {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  font-family: monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-open-icon {
  color: var(--el-text-color-placeholder);
  font-size: 14px;
  flex-shrink: 0;
}
.nav-card:hover .nav-open-icon {
  color: var(--el-color-primary);
}

/* banner 模式加粗显示 */
.nav-mode-banner .nav-name {
  font-size: 15px;
  font-weight: 700;
}
</style>
