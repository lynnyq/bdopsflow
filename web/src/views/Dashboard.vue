<template>
  <div class="dashboard">
    <!-- Header -->
    <header class="dashboard-header">
      <div class="header-left">
        <h1 class="page-title">控制中心</h1>
        <p class="page-subtitle">实时监控系统运行状态</p>
      </div>
      <div class="header-right">
        <el-button 
          :icon="Refresh" 
          @click="refreshData" 
          :loading="loading"
          circle
          size="large"
        />
      </div>
    </header>

    <!-- Scheduler Control -->
    <section class="scheduler-section">
      <div class="scheduler-status">
        <div class="status-indicator" :class="{ paused: stats.scheduler.paused }">
          <span class="status-dot"></span>
          <span class="status-text">{{ stats.scheduler.paused ? '已暂停' : '运行中' }}</span>
        </div>
        <span class="uptime">已运行 {{ formatUptime(stats.scheduler.uptime) }}</span>
      </div>
      <div class="scheduler-actions">
        <el-button 
          :icon="VideoPlay" 
          @click="handleResumeScheduler"
          :disabled="!stats.scheduler.paused"
          :loading="actionLoading"
        >
          恢复调度
        </el-button>
        <el-button 
          :icon="VideoPause" 
          @click="handlePauseScheduler"
          :disabled="stats.scheduler.paused"
          :loading="actionLoading"
        >
          暂停调度
        </el-button>
      </div>
    </section>

    <!-- Stats Overview -->
    <section class="stats-section">
      <div class="stats-grid">
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.total }}</span>
          <span class="stat-label">总任务</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.enabled }}</span>
          <span class="stat-label">已启用</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.cron }}</span>
          <span class="stat-label">定时任务</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.running }}</span>
          <span class="stat-label">运行中</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.success }}</span>
          <span class="stat-label">执行成功</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.failed }}</span>
          <span class="stat-label">执行失败</span>
        </div>
        <div class="stat-item">
          <span class="stat-value">{{ stats.tasks.avg_duration }}s</span>
          <span class="stat-label">平均耗时</span>
        </div>
      </div>
    </section>

    <!-- Two Column Layout -->
    <div class="content-grid">
      <!-- Left Column: Execution Trends -->
      <section class="trends-section">
        <div class="section-header">
          <h2 class="section-title">执行趋势</h2>
          <span class="section-hint">最近 7 天</span>
        </div>
        <div class="trends-list" v-if="trends.length > 0">
          <div v-for="trend in trends" :key="trend.date" class="trend-item">
            <span class="trend-date">{{ formatDate(trend.date) }}</span>
            <div class="trend-stats">
              <div class="trend-stat">
                <span class="trend-label">总数</span>
                <span class="trend-value">{{ trend.total }}</span>
              </div>
              <div class="trend-stat success">
                <span class="trend-label">成功</span>
                <span class="trend-value">{{ trend.success }}</span>
              </div>
              <div class="trend-stat failed">
                <span class="trend-label">失败</span>
                <span class="trend-value">{{ trend.failed }}</span>
              </div>
            </div>
          </div>
        </div>
        <div class="empty-state" v-else>
          <p>暂无执行数据</p>
        </div>
      </section>

      <!-- Right Column: Quick Actions & Executors -->
      <div class="right-column">
        <!-- Quick Actions -->
        <section class="actions-section">
          <div class="section-header">
            <h2 class="section-title">快捷操作</h2>
          </div>
          <div class="actions-grid">
            <router-link to="/tasks" class="action-item">
              <span class="action-label">管理任务</span>
            </router-link>
            <router-link to="/executors" class="action-item">
              <span class="action-label">管理执行器</span>
            </router-link>
            <router-link to="/logs" class="action-item">
              <span class="action-label">查看日志</span>
            </router-link>
          </div>
        </section>

        <!-- Executors -->
        <section class="executors-section">
          <div class="section-header">
            <h2 class="section-title">执行器</h2>
            <router-link to="/executors" class="view-all">查看全部 →</router-link>
          </div>
          <div class="executor-stats">
            <div class="executor-item">
              <span class="executor-value">{{ stats.executors.total }}</span>
              <span class="executor-label">总数</span>
            </div>
            <div class="executor-item online">
              <span class="executor-value">{{ stats.executors.online }}</span>
              <span class="executor-label">在线</span>
            </div>
            <div class="executor-item offline">
              <span class="executor-value">{{ stats.executors.offline }}</span>
              <span class="executor-label">离线</span>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  Refresh, VideoPlay, VideoPause
} from '@element-plus/icons-vue'
import { dashboardAPI } from '@/api'
import type { DashboardStats, TrendData } from '@/types'

const loading = ref(false)

const stats = ref<DashboardStats>({
  tasks: {
    total: 0,
    enabled: 0,
    cron: 0,
    running: 0,
    success: 0,
    failed: 0,
    avg_duration: 0
  },
  workflows: {
    total: 0,
    enabled: 0
  },
  executors: {
    total: 0,
    online: 0,
    offline: 0
  },
  scheduler: {
    paused: false,
    uptime: 0
  }
})

const trends = ref<TrendData[]>([])
const actionLoading = ref(false)

const maxTrendValue = computed(() => {
  if (trends.value.length === 0) return 1
  let max = 0
  for (const t of trends.value) {
    max = Math.max(max, t.total, t.success, t.failed)
  }
  return max || 1
})

const formatUptime = (seconds: number) => {
  if (!seconds) return '-';
  
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;
  
  if (hours > 0) {
    return `${hours}小时${minutes}分钟`;
  } else if (minutes > 0) {
    return `${minutes}分钟${secs}秒`;
  }
  return `${secs}秒`;
}

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return `${date.getMonth() + 1}月${date.getDate()}日`
}

const getBarWidth = (value: number, max: number) => {
  if (max === 0) return 0
  return Math.max((value / max) * 100, 2)
}

const loadData = async () => {
  try {
    const [statsRes, trendsRes] = await Promise.all([
      dashboardAPI.getStats(),
      dashboardAPI.getTrends()
    ])
    
    stats.value = statsRes.data;
    trends.value = trendsRes.data.items || [];
  } catch (err) {
    console.error('Failed to load dashboard data:', err);
    ElMessage.error('加载仪表盘数据失败');
  }
}

const refreshData = () => {
  loadData();
}

const handlePauseScheduler = async () => {
  try {
    await ElMessageBox.confirm(
      '确定要暂停调度器吗？所有定时任务将停止调度，正在执行的任务会继续完成。',
      '暂停调度器',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    );
    
    actionLoading.value = true;
    await dashboardAPI.pauseScheduler();
    ElMessage.success('调度器已暂停');
    await loadData();
  } catch (err: any) {
    if (err !== 'cancel') {
      console.error('Failed to pause scheduler:', err);
      ElMessage.error('暂停调度器失败');
    }
  } finally {
    actionLoading.value = false;
  }
};

const handleResumeScheduler = async () => {
  try {
    actionLoading.value = true;
    await dashboardAPI.resumeScheduler();
    ElMessage.success('调度器已恢复');
    await loadData();
  } catch (err) {
    console.error('Failed to resume scheduler:', err);
    ElMessage.error('恢复调度器失败');
  } finally {
    actionLoading.value = false;
  }
};

onMounted(() => {
  loadData();
});
</script>

<style scoped>
.dashboard {
  padding: 32px 48px;
  max-width: 1400px;
  margin: 0 auto;
}

/* Header */
.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 48px;
}

.header-left {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.page-title {
  font-size: 32px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.5px;
}

.page-subtitle {
  font-size: 14px;
  color: var(--text-muted);
  margin: 0;
}

.header-right {
  display: flex;
  gap: 12px;
}

/* Scheduler Control */
.scheduler-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  background: var(--bg-primary);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  margin-bottom: 48px;
}

.scheduler-status {
  display: flex;
  align-items: center;
  gap: 24px;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent-success);
  box-shadow: 0 0 8px var(--accent-success);
}

.status-dot.paused {
  background: var(--accent-danger);
  box-shadow: 0 0 8px var(--accent-danger);
}

.status-text {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.uptime {
  font-size: 13px;
  color: var(--text-muted);
}

.scheduler-actions {
  display: flex;
  gap: 12px;
}

/* Stats Overview */
.stats-section {
  margin-bottom: 48px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 1px;
  background: var(--border-subtle);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  overflow: hidden;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 24px 20px;
  background: var(--bg-primary);
  transition: background 0.15s ease;
}

.stat-item:hover {
  background: var(--bg-secondary);
}

.stat-value {
  font-size: 28px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.5px;
}

.stat-label {
  font-size: 13px;
  color: var(--text-muted);
}

/* Content Grid */
.content-grid {
  display: grid;
  grid-template-columns: 1fr 360px;
  gap: 48px;
}

.right-column {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

/* Section Styling */
.trends-section,
.workflows-section,
.executors-section,
.actions-section {
  background: var(--bg-primary);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 24px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.section-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.view-all {
  font-size: 13px;
  color: var(--accent-primary);
  text-decoration: none;
  transition: opacity 0.15s ease;
}

.view-all:hover {
  opacity: 0.7;
}

/* Trends */
.trends-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.trend-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: var(--bg-secondary);
  border-radius: 6px;
}

.trend-date {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.trend-stats {
  display: flex;
  gap: 32px;
}

.trend-stat {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.trend-label {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.trend-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.trend-stat.success .trend-value {
  color: var(--accent-success);
}

.trend-stat.failed .trend-value {
  color: var(--accent-danger);
}

/* Workflows & Executors */
.workflow-stats,
.executor-stats {
  display: flex;
  gap: 16px;
}

.workflow-item,
.executor-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px;
  background: var(--bg-secondary);
  border-radius: 6px;
}

.workflow-value,
.executor-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-primary);
}

.workflow-value.success,
.executor-value.online {
  color: var(--accent-success);
}

.executor-value.offline {
  color: var(--accent-danger);
}

.workflow-label,
.executor-label {
  font-size: 12px;
  color: var(--text-muted);
}

/* Quick Actions */
.actions-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.action-item {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: var(--bg-secondary);
  border-radius: 6px;
  text-decoration: none;
  transition: all 0.15s ease;
}

.action-item:hover {
  background: var(--accent-primary);
  color: white;
}

.action-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.action-item:hover .action-label {
  color: white;
}

/* Empty State */
.empty-state {
  padding: 48px 24px;
  text-align: center;
}

.empty-state p {
  font-size: 14px;
  color: var(--text-muted);
  margin: 0;
}

/* Responsive */
@media (max-width: 1200px) {
  .content-grid {
    grid-template-columns: 1fr;
  }
  
  .stats-grid {
    grid-template-columns: repeat(4, 1fr);
  }
}

@media (max-width: 768px) {
  .dashboard {
    padding: 24px;
  }
  
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .scheduler-section {
    flex-direction: column;
    gap: 16px;
    align-items: flex-start;
  }
  
  .scheduler-actions {
    width: 100%;
  }
  
  .scheduler-actions button {
    flex: 1;
  }
}
</style>
