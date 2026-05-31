<template>
  <div class="dashboard">
    <!-- Stats Overview -->
    <section class="stats-section">
      <div class="stats-grid">
        <div class="stat-item">
          <div class="stat-icon stat-icon-primary">
            <el-icon :size="24"><List /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.tasks?.total ?? 0 }}</div>
            <div class="stat-label">总任务</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon stat-icon-success">
            <el-icon :size="24"><CircleCheck /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.tasks?.enabled ?? 0 }}</div>
            <div class="stat-label">已启用</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon stat-icon-info">
            <el-icon :size="24"><Clock /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.tasks?.cron ?? 0 }}</div>
            <div class="stat-label">定时任务</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon stat-icon-warning">
            <el-icon :size="24"><Timer /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.tasks?.running ?? 0 }}</div>
            <div class="stat-label">运行中</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon stat-icon-primary">
            <el-icon :size="24"><Cpu /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.executors?.total ?? 0 }}</div>
            <div class="stat-label">执行器总数</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon stat-icon-success">
            <el-icon :size="24"><CircleCheck /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.executors?.active ?? 0 }}</div>
            <div class="stat-label">在线执行器</div>
          </div>
        </div>
      </div>
    </section>

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

    <!-- Execution Stats -->
    <section class="execution-section">
      <div class="execution-grid">
        <div class="stat-item success">
          <div class="stat-icon stat-icon-success">
            <el-icon :size="24"><CircleCheck /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.tasks?.success ?? 0 }}</div>
            <div class="stat-label">成功执行</div>
          </div>
        </div>
        <div class="stat-item danger">
          <div class="stat-icon stat-icon-danger">
            <el-icon :size="24"><CircleClose /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ stats?.tasks?.failed ?? 0 }}</div>
            <div class="stat-label">失败执行</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon stat-icon-warning">
            <el-icon :size="24"><Timer /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-value">{{ formatDuration(stats?.tasks?.avg_duration ?? 0) }}</div>
            <div class="stat-label">平均执行时长</div>
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
          @click="refreshData" 
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { VideoPlay, VideoPause, Refresh, List, CircleCheck, CircleClose, Clock, Timer, Cpu, DataLine } from '@element-plus/icons-vue'
import { dashboardAPI, type HealthCheckResult } from '@/api'
import { isHandledError } from '@/utils/api'
import { useAuthStore } from '@/stores/auth'
import type { DashboardStats, TrendData } from '@/types'

const authStore = useAuthStore()

const canControlScheduler = computed(() => {
  return authStore.roleCodes.includes('admin') || authStore.roleCodes.includes('system_admin')
})

const loading = ref(false)
const actionLoading = ref(false)
const stats = ref<DashboardStats>({
  tasks: { total: 0, enabled: 0, cron: 0, running: 0, success: 0, failed: 0, avg_duration: 0 },
  executors: { total: 0, active: 0, online: 0, offline: 0 },
  scheduler: { paused: false, uptime: 0 }
})
const schedulerStatus = ref({ paused: false })
const trends = ref<TrendData[]>([])
const healthData = ref<HealthCheckResult | null>(null)

let refreshInterval: number | null = null
let healthInterval: number | null = null

const healthStatusClass = computed(() => {
  return healthData.value?.status === 'healthy' ? 'healthy' : 'unhealthy'
})

const getComponentName = (key: string): string => {
  const names: Record<string, string> = {
    rqlite: '数据库',
    rqlite_tables: '数据库表结构',
    redis: 'Redis 缓存',
    scheduler: '任务调度器'
  }
  return names[key] || key
}

const getComponentIcon = (key: string): string => {
  const icons: Record<string, string> = {
    rqlite: 'DataLine',
    rqlite_tables: 'Odometer',
    redis: 'Bell',
    scheduler: 'Monitor'
  }
  return icons[key] || 'Monitor'
}

const loadHealthData = async () => {
  try {
    const response = await dashboardAPI.getHealth()
    healthData.value = response.data
  } catch (error) {
  }
}

const loadDashboardStats = async () => {
  try {
    const response = await dashboardAPI.getStats()
    const data = response.data || {}
    
    // 安全地合并数据，保持默认结构
    stats.value = {
      tasks: {
        total: data.tasks?.total ?? 0,
        enabled: data.tasks?.enabled ?? 0,
        cron: data.tasks?.cron ?? 0,
        running: data.tasks?.running ?? 0,
        success: data.tasks?.success ?? 0,
        failed: data.tasks?.failed ?? 0,
        avg_duration: data.tasks?.avg_duration ?? 0
      },
      executors: {
        total: data.executors?.total ?? 0,
        active: data.executors?.active ?? 0,
        online: data.executors?.online ?? 0,
        offline: data.executors?.offline ?? 0
      },
      scheduler: {
        paused: data.scheduler?.paused ?? false,
        uptime: data.scheduler?.uptime ?? 0
      }
    }
  } catch (error) {
  }
}

const loadSchedulerStatus = async () => {
  try {
    const response = await dashboardAPI.getSchedulerStatus()
    schedulerStatus.value = {
      paused: response.data?.paused ?? false
    }
  } catch (error) {
  }
}

const loadTrends = async () => {
  try {
    const response = await dashboardAPI.getTrends()
    const data = response.data || {}
    trends.value = data.items || []
  } catch (error) {
  }
}

const refreshData = async () => {
  loading.value = true
  try {
    await Promise.all([
      loadDashboardStats(),
      loadSchedulerStatus(),
      loadTrends(),
    ])
  } finally {
    loading.value = false
  }
}

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

const getBarHeight = (value: number): number => {
  if (!trends.value || trends.value.length === 0) return 0
  const validTrends = trends.value.filter((t: TrendData) => t && typeof t.success === 'number' && typeof t.failed === 'number')
  if (validTrends.length === 0) return 0
  const maxValue = Math.max(...validTrends.map((t: TrendData) => Math.max(t.success, t.failed)))
  if (maxValue === 0) return 0
  const safeValue = typeof value === 'number' ? value : 0
  return Math.max(10, (safeValue / maxValue) * 100)
}

const formatDate = (date: string): string => {
  if (!date) return ''
  const d = new Date(date)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

const formatDuration = (seconds: number): string => {
  if (!seconds || seconds === 0) return '0s'
  if (seconds < 1) return `${(seconds).toFixed(2)}s`
  if (seconds < 60) return `${seconds.toFixed(2)}s`
  const minutes = Math.floor(seconds / 60)
  const secs = seconds % 60
  if (minutes < 60) return `${minutes}m${secs.toFixed(0)}s`
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  return `${hours}h${mins}m`
}

onMounted(() => {
  refreshData()
  loadHealthData()
  refreshInterval = window.setInterval(refreshData, 30000)
  healthInterval = window.setInterval(loadHealthData, 60000)
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  if (healthInterval) {
    clearInterval(healthInterval)
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

/* Stats Section */
.stats-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  box-shadow: var(--shadow-sm);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

@media (max-width: 1400px) {
  .stats-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 1000px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 600px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}

.stat-item {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  transition: all var(--duration-normal) var(--ease-out);
}

.stat-item:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.stat-item.success {
  background: rgba(52, 211, 153, 0.08);
}

.stat-item.danger {
  background: rgba(248, 113, 113, 0.08);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon-primary {
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-primary);
}

.stat-icon-success {
  background: rgba(52, 211, 153, 0.1);
  color: var(--accent-success);
}

.stat-icon-warning {
  background: rgba(251, 191, 36, 0.1);
  color: var(--accent-warning);
}

.stat-icon-info {
  background: rgba(6, 182, 212, 0.1);
  color: var(--accent-secondary);
}

.stat-icon-danger {
  background: rgba(248, 113, 113, 0.1);
  color: var(--accent-danger);
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-value {
  font-family: var(--font-display);
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

.stat-label {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
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

/* Execution Section */
.execution-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  box-shadow: var(--shadow-sm);
}

.execution-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-4);
}

@media (max-width: 800px) {
  .execution-grid {
    grid-template-columns: 1fr;
  }
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
</style>
