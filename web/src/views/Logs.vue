<template>
  <div class="logs-page">
    <div class="main-content">
      <div class="executions-section" :class="{ 'with-logs': showLogs }">
        <!-- Stats Cards -->
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-icon stat-icon-primary">
              <el-icon :size="24"><List /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">{{ total }}</div>
              <div class="stat-label">总日志数</div>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-icon stat-icon-success">
              <el-icon :size="24"><CircleCheck /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">{{ successCount }}</div>
              <div class="stat-label">成功日志</div>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-icon stat-icon-danger">
              <el-icon :size="24"><CircleClose /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">{{ failedCount }}</div>
              <div class="stat-label">失败日志</div>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-icon stat-icon-warning">
              <el-icon :size="24"><Loading /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">{{ runningCount }}</div>
              <div class="stat-label">运行中</div>
            </div>
          </div>
        </div>

        <!-- 工具栏 -->
        <div class="page-toolbar">
          <div class="toolbar-left">
            <el-input
              v-model="filters.id"
              placeholder="ID"
              clearable
              class="filter-input"
              @keyup.enter="loadExecutions(1)"
            />
            <el-input
              v-model="filters.execution_id"
              placeholder="执行ID"
              clearable
              class="filter-input"
              @keyup.enter="loadExecutions(1)"
            />
            <el-input
              v-model="filters.task_name"
              placeholder="任务名称"
              clearable
              class="filter-input"
              @keyup.enter="loadExecutions(1)"
            />
            <el-input
              v-model="filters.executor_name"
              placeholder="执行节点"
              clearable
              class="filter-input"
              @keyup.enter="loadExecutions(1)"
            />
            <el-select
              v-model="filters.status"
              placeholder="状态"
              clearable
              class="filter-select"
              @change="loadExecutions(1)"
            >
              <el-option label="成功" value="success" />
              <el-option label="失败" value="failed" />
              <el-option label="运行中" value="running" />
              <el-option label="待执行" value="pending" />
            </el-select>
          </div>
          <div class="toolbar-right">
            <el-button text @click="showAdvancedFilters = !showAdvancedFilters" class="advanced-filter-btn">
              <el-icon><Filter /></el-icon>
              高级筛选
              <el-icon class="arrow-icon" :class="{ 'is-active': showAdvancedFilters }">
                <ArrowDown />
              </el-icon>
            </el-button>
            <el-button :icon="Refresh" @click="loadExecutions(1)" :loading="loading" class="refresh-btn">刷新</el-button>
            <el-button
              v-if="selectedIds && selectedIds.length > 0 && canDelete"
              type="danger"
              @click="handleBatchDelete"
              class="delete-btn"
            >
              <el-icon><Delete /></el-icon>
              批量删除 ({{ selectedIds ? selectedIds.length : 0 }})
            </el-button>
          </div>
        </div>

        <!-- 高级筛选区域 -->
        <div v-if="showAdvancedFilters" class="advanced-filters">
          <div class="advanced-filters-row">
            <div class="filter-group">
              <span class="filter-label">开始时间</span>
              <div class="filter-range">
                <el-date-picker
                  v-model="filters.start_time_from"
                  type="datetime"
                  placeholder="开始时间"
                  format="YYYY-MM-DD HH:mm:ss"
                  value-format="YYYY-MM-DD HH:mm:ss"
                  class="filter-date"
                  @change="loadExecutions(1)"
                />
                <span class="range-separator">-</span>
                <el-date-picker
                  v-model="filters.start_time_to"
                  type="datetime"
                  placeholder="结束时间"
                  format="YYYY-MM-DD HH:mm:ss"
                  value-format="YYYY-MM-DD HH:mm:ss"
                  class="filter-date"
                  @change="loadExecutions(1)"
                />
              </div>
            </div>
            <div class="filter-group">
              <span class="filter-label">结束时间</span>
              <div class="filter-range">
                <el-date-picker
                  v-model="filters.end_time_from"
                  type="datetime"
                  placeholder="开始时间"
                  format="YYYY-MM-DD HH:mm:ss"
                  value-format="YYYY-MM-DD HH:mm:ss"
                  class="filter-date"
                  @change="loadExecutions(1)"
                />
                <span class="range-separator">-</span>
                <el-date-picker
                  v-model="filters.end_time_to"
                  type="datetime"
                  placeholder="结束时间"
                  format="YYYY-MM-DD HH:mm:ss"
                  value-format="YYYY-MM-DD HH:mm:ss"
                  class="filter-date"
                  @change="loadExecutions(1)"
                />
              </div>
            </div>
            <div class="filter-group">
              <span class="filter-label">执行时长(秒)</span>
              <div class="filter-range">
                <el-input-number
                  v-model="filters.duration_min"
                  :min="0"
                  :step="0.1"
                  :precision="3"
                  placeholder="最小值"
                  controls-position="right"
                  class="filter-number"
                  @change="loadExecutions(1)"
                />
                <span class="range-separator">-</span>
                <el-input-number
                  v-model="filters.duration_max"
                  :min="0"
                  :step="0.1"
                  :precision="3"
                  placeholder="最大值"
                  controls-position="right"
                  class="filter-number"
                  @change="loadExecutions(1)"
                />
              </div>
            </div>
            <div class="filter-group filter-actions">
              <el-button @click="resetFilters" size="small">重置</el-button>
            </div>
          </div>
        </div>

        <!-- 表格 -->
        <div class="table-wrapper">
          <el-table
            :data="executions || []"
            stripe
            style="width: 100%"
            v-loading="loading"
            :row-class-name="tableRowClassName"
            @row-click="handleRowClick"
            @selection-change="handleSelectionChange"
            height="100%"
          >
            <el-table-column type="selection" width="55" fixed="left" />
            <el-table-column prop="id" label="ID" width="70" fixed="left" />
            <el-table-column prop="execution_id" label="执行ID" :minWidth="180" show-overflow-tooltip />
            <el-table-column label="任务名称" :minWidth="160" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.task_name || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="执行节点" :minWidth="150" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.executor_name || '-' }}
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="110" align="center">
              <template #default="{ row }">
                <span class="status-dot" :class="getStatusDotClass(row.status)"></span>
                <el-tag :type="getStatusType(row.status)" size="small" effect="light">
                  {{ getStatusText(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="开始时间" width="170">
              <template #default="{ row }">
                {{ formatTime(row.start_time) }}
              </template>
            </el-table-column>
            <el-table-column label="结束时间" width="170">
              <template #default="{ row }">
                {{ formatTime(row.end_time) || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="130" align="center">
              <template #default="{ row }">
                {{ formatDuration(row.start_time, row.end_time, row.status) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140" fixed="right" align="center">
              <template #default="{ row }">
                <el-button
                  type="primary"
                  size="small"
                  circle
                  @click.stop="viewLogs(row)"
                  class="action-btn view-btn"
                >
                  <el-icon><View /></el-icon>
                </el-button>
                <el-button
                  v-if="canDelete"
                  type="danger"
                  size="small"
                  circle
                  @click.stop="handleDelete(row)"
                  class="action-btn delete-btn"
                >
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-table-column>
            <template #empty>
              <div class="table-empty-state">
                <el-icon :size="32"><Document /></el-icon>
                <p>暂无执行记录</p>
              </div>
            </template>
          </el-table>
        </div>

        <!-- 分页器 -->
        <div class="pagination-container" v-if="total > 0">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="total"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="handleCurrentChange"
          />
        </div>
      </div>

      <div class="logs-section" v-if="showLogs">
        <TaskLogViewer
          :execution-id="currentExecutionId"
          :execution-status="currentExecution?.status"
          :output="currentExecution?.output"
          :error="currentExecution?.error"
          @close="closeLogs"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Delete, Document, View, List, CircleCheck, CircleClose, Loading, Filter, ArrowDown } from '@element-plus/icons-vue'
import { logAPI } from '@/api'
import { useAuthStore } from '@/stores/auth'
import TaskLogViewer from '@/components/TaskLogViewer.vue'
import type { TaskExecutionListResponse } from '@/types'
import { isHandledError } from '@/utils/api'

const route = useRoute()
const authStore = useAuthStore()

const canDelete = computed(() => {
  return authStore.hasPermission('log', 'delete') || authStore.hasPermission('log', 'manage')
})
const executions = ref<TaskExecutionListResponse[]>([])
const loading = ref(false)
const showLogs = ref(false)
const currentExecutionId = ref('')
const currentExecution = ref<TaskExecutionListResponse | null>(null)
const selectedIds = ref<number[]>([])
const showAdvancedFilters = ref(false)

const filters = ref({
  id: '',
  execution_id: '',
  task_name: '',
  executor_name: '',
  status: '',
  start_time_from: '',
  start_time_to: '',
  end_time_from: '',
  end_time_to: '',
  duration_min: null as number | null,
  duration_max: null as number | null
})

const resetFilters = () => {
  filters.value = {
    id: '',
    execution_id: '',
    task_name: '',
    executor_name: '',
    status: '',
    start_time_from: '',
    start_time_to: '',
    end_time_from: '',
    end_time_to: '',
    duration_min: null,
    duration_max: null
  }
  loadExecutions(1)
}

// 分页相关
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

// 统计数据
const stats = ref({
  success: 0,
  failed: 0,
  running: 0
})

// 统计计算属性
const successCount = computed(() => stats.value.success)
const failedCount = computed(() => stats.value.failed)
const runningCount = computed(() => stats.value.running)

const getStatusType = (status: string) => {
  switch (status) {
    case 'success': return 'success'
    case 'failed': return 'danger'
    case 'running': return 'warning'
    case 'pending': return 'info'
    default: return 'info'
  }
}

const getStatusText = (status: string) => {
  switch (status) {
    case 'success': return '成功'
    case 'failed': return '失败'
    case 'running': return '运行中'
    case 'pending': return '待执行'
    default: return status
  }
}

const getStatusDotClass = (status: string) => {
  switch (status) {
    case 'success': return 'dot-success'
    case 'failed': return 'dot-danger'
    case 'running': return 'dot-warning'
    case 'pending': return 'dot-info'
    default: return 'dot-info'
  }
}

const tableRowClassName = ({ row }: { row: TaskExecutionListResponse }) => {
  if (currentExecution.value?.execution_id === row.execution_id) {
    return 'current-row'
  }
  return ''
}

const formatTime = (timeStr: string | null | undefined) => {
  if (!timeStr) return '-'
  try {
    const date = new Date(timeStr)
    if (isNaN(date.getTime())) {
      return '-'
    }
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    const seconds = String(date.getSeconds()).padStart(2, '0')
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  } catch {
    return '-'
  }
}

const formatDuration = (startTime: string | null | undefined, endTime: string | null | undefined, status?: string | null) => {
  if (!startTime) return '-'
  
  const start = new Date(startTime).getTime()
  if (isNaN(start)) return '-'
  
  let end: number | null = null
  
  if (endTime) {
    const parsedEnd = new Date(endTime).getTime()
    if (!isNaN(parsedEnd)) {
      end = parsedEnd
    }
  }
  
  if (end === null) {
    if (status === 'running') {
      end = Date.now()
    } else {
      return '-'
    }
  }
  
  const diff = end - start
  if (diff < 0) return '-'
  
  if (diff < 1000) {
    return `${diff}ms`
  } else if (diff < 60000) {
    return `${(diff / 1000).toFixed(2)}s`
  } else if (diff < 3600000) {
    const mins = Math.floor(diff / 60000)
    const secs = Math.floor((diff % 60000) / 1000)
    return `${mins}m ${secs}s`
  } else {
    const hours = Math.floor(diff / 3600000)
    const mins = Math.floor((diff % 3600000) / 60000)
    return `${hours}h ${mins}m`
  }
}

const loadExecutions = async (page: number = currentPage.value) => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      id: filters.value.id,
      execution_id: filters.value.execution_id,
      task_name: filters.value.task_name,
      executor_name: filters.value.executor_name,
      status: filters.value.status,
      page: page,
      page_size: pageSize.value
    }
    
    if (filters.value.start_time_from) {
      params.start_time_from = filters.value.start_time_from
    }
    if (filters.value.start_time_to) {
      params.start_time_to = filters.value.start_time_to
    }
    if (filters.value.end_time_from) {
      params.end_time_from = filters.value.end_time_from
    }
    if (filters.value.end_time_to) {
      params.end_time_to = filters.value.end_time_to
    }
    if (filters.value.duration_min !== null && filters.value.duration_min !== undefined && filters.value.duration_min >= 0) {
      params.duration_min = filters.value.duration_min
    }
    if (filters.value.duration_max !== null && filters.value.duration_max !== undefined && filters.value.duration_max >= 0) {
      params.duration_max = filters.value.duration_max
    }
    
    const statsParams: Record<string, any> = { ...params }
    delete statsParams.page
    delete statsParams.page_size

    const [response, statsResponse] = await Promise.all([
      logAPI.list(params),
      logAPI.getStats(statsParams)
    ])

    executions.value = response.data.items || []
    total.value = response.data.total || 0
    currentPage.value = response.data.page || 1

    const statsData = statsResponse.data || {}
    stats.value = {
      success: statsData.success || 0,
      failed: statsData.failed || 0,
      running: statsData.running || 0
    }
  } catch (error) {
    if (!isHandledError(error)) {
      ElMessage.error('加载执行记录失败')
    }
  } finally {
    loading.value = false
  }
}

// task_name / executor_name 变化：防抖后重置到第 1 页并重新请求后端
let filterDebounceTimer: ReturnType<typeof setTimeout> | null = null
watch([() => filters.value.task_name, () => filters.value.executor_name], () => {
  if (filterDebounceTimer) {
    clearTimeout(filterDebounceTimer)
  }
  filterDebounceTimer = setTimeout(() => {
    loadExecutions(1)
  }, 300)
})

const handleSelectionChange = (selection: TaskExecutionListResponse[]) => {
  selectedIds.value = selection.map(item => item.id)
}

const handleRowClick = (row: TaskExecutionListResponse) => {
  currentExecution.value = row
}

const viewLogs = (row: TaskExecutionListResponse) => {
  currentExecution.value = row
  currentExecutionId.value = row.execution_id || ''
  showLogs.value = true
}

const closeLogs = () => {
  showLogs.value = false
  currentExecution.value = null
  currentExecutionId.value = ''
}

const handleDelete = async (row: TaskExecutionListResponse) => {
  try {
    await ElMessageBox.confirm(
      '确认要删除这个执行记录吗？此操作将同时删除所有相关日志。',
      '确认删除',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await logAPI.delete(row.id)
    ElMessage.success('删除成功')
    await loadExecutions()
  } catch (error) {
    if (error !== 'cancel' && !isHandledError(error)) {
      ElMessage.error('删除失败')
    }
  }
}

const handleBatchDelete = async () => {
  try {
    await ElMessageBox.confirm(
      `确认要删除选中的 ${selectedIds.value.length} 条执行记录吗？此操作将同时删除所有相关日志。`,
      '批量删除确认',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await logAPI.batchDelete(selectedIds.value)
    ElMessage.success('批量删除成功')
    selectedIds.value = []
    await loadExecutions()
  } catch (error) {
    if (error !== 'cancel' && !isHandledError(error)) {
      ElMessage.error('批量删除失败')
    }
  }
}

const handleSizeChange = (val: number) => {
  pageSize.value = val
  loadExecutions(1)
}

const handleCurrentChange = (val: number) => {
  currentPage.value = val
  loadExecutions(val)
}

onMounted(() => {
  // 检查路由参数
  if (route.query.task_name) {
    filters.value.task_name = route.query.task_name as string
  }

  loadExecutions()
})
</script>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  display: flex;
  align-items: center;
  gap: var(--space-4);
  box-shadow: var(--shadow-md);
  transition: all var(--duration-normal) var(--ease-out);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon-primary {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(37, 99, 235, 0.05));
  color: var(--accent-primary);
}

.stat-icon-success {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.05));
  color: var(--accent-success);
}

.stat-icon-warning {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.1), rgba(245, 158, 11, 0.05));
  color: var(--accent-warning);
}

.stat-icon-danger {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.1), rgba(239, 68, 68, 0.05));
  color: var(--accent-danger);
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.stat-value {
  font-family: var(--font-display);
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

.stat-label {
  font-size: 0.8rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

/* Main Content */
.main-content {
  display: flex;
  gap: var(--space-4);
  min-height: 0;
  flex: 1;
}

/* Executions Section */
.executions-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.executions-section.with-logs {
  flex: 0 0 60%;
}

/* Page Toolbar */
.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.filter-input {
  width: 150px;
}

.filter-select {
  width: 140px;
}

.filter-input :deep(.el-input__wrapper),
.filter-select :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.filter-input :deep(.el-input__wrapper:hover),
.filter-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.filter-input :deep(.el-input__wrapper.is-focus),
.filter-select :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

/* Toolbar Buttons */
.advanced-filter-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.advanced-filter-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.advanced-filter-btn .arrow-icon {
  transition: transform var(--duration-normal) var(--ease-out);
}

.advanced-filter-btn .arrow-icon.is-active {
  transform: rotate(180deg);
}

.refresh-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 16px;
}

.refresh-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.refresh-btn:active {
  transform: translateY(0);
}

.delete-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--accent-danger);
  color: var(--accent-danger);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 16px;
}

.delete-btn:hover {
  background: rgba(239, 68, 68, 0.05);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.delete-btn:active {
  transform: translateY(0);
}

/* Advanced Filters */
.advanced-filters {
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  margin-top: calc(-1 * var(--space-2));
}

.advanced-filters-row {
  display: flex;
  align-items: flex-end;
  gap: var(--space-4);
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.filter-group.filter-actions {
  flex-shrink: 0;
}

.filter-label {
  font-size: 0.8rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.filter-range {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.range-separator {
  color: var(--text-muted);
  font-weight: 500;
}

.filter-number {
  width: 100px;
}

.filter-number :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.filter-number :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.filter-number :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.filter-date {
  width: 200px;
}

.filter-date :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.filter-date :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.filter-date :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

/* Table */
.table-wrapper {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

:deep(.el-table) {
  border-radius: var(--radius-lg);
}

:deep(.el-table--border::after),
:deep(.el-table--group::after),
:deep(.el-table::before) {
  display: none;
}

:deep(.el-table tr) {
  transition: background-color var(--duration-normal) var(--ease-out);
}

:deep(.el-table .current-row) {
  background-color: rgba(59, 130, 246, 0.08) !important;
}

:deep(.el-table .current-row:hover > td) {
  background-color: rgba(59, 130, 246, 0.12) !important;
}

:deep(.el-table__row:hover) {
  background-color: var(--bg-secondary) !important;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 8px;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.2);
}

.dot-success {
  background: var(--accent-success);
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.2);
}

.dot-warning {
  background: var(--accent-warning);
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.2);
}

.dot-info {
  background: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.2);
}

.dot-danger {
  background: var(--accent-danger);
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.2);
}

/* Table Empty State */
.table-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  gap: var(--space-3);
  color: var(--text-muted);
}

.table-empty-state .el-icon {
  opacity: 0.4;
}

.table-empty-state p {
  margin: 0;
  font-size: 0.875rem;
}

/* Action Buttons */
.action-btn {
  transition: all var(--duration-normal) var(--ease-out);
  opacity: 0.8;
}

.action-btn:hover {
  opacity: 1;
  transform: scale(1.1);
}

.view-btn {
  margin-right: 4px;
}

.delete-btn {
  margin-left: 4px;
}

/* Pagination */
.pagination-container {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.pagination-container :deep(.el-pagination) {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pagination-container :deep(.el-pagination__total) {
  font-weight: 500;
  color: var(--text-secondary);
}

.pagination-container :deep(.el-pagination__sizes) .el-select .el-input__wrapper {
  border-radius: var(--radius-md);
  box-shadow: 0 0 0 1px var(--border-default) inset;
  transition: all var(--duration-normal) var(--ease-out);
}

.pagination-container :deep(.el-pagination__sizes) .el-select .el-input__wrapper:hover {
  box-shadow: 0 0 0 1px var(--accent-primary) inset;
}

.pagination-container :deep(.el-pager li) {
  border-radius: var(--radius-md);
  margin: 0 4px;
  transition: all var(--duration-normal) var(--ease-out);
  font-weight: 500;
  height: 36px;
  min-width: 36px;
  line-height: 36px;
}

.pagination-container :deep(.el-pager li:not(.is-active)) {
  color: var(--text-secondary);
  background: var(--bg-secondary);
}

.pagination-container :deep(.el-pager li:not(.is-active):hover) {
  color: var(--accent-primary);
  background: var(--bg-primary);
  transform: translateY(-1px);
}

.pagination-container :deep(.el-pager li.is-active) {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  color: white;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

.pagination-container :deep(.btn-prev),
.pagination-container :deep(.btn-next) {
  border-radius: var(--radius-md);
  transition: all var(--duration-normal) var(--ease-out);
  height: 36px;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.pagination-container :deep(.btn-prev:hover),
.pagination-container :deep(.btn-next:hover) {
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

/* Logs Section */
.logs-section {
  flex: 1;
  min-width: 280px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Responsive */
@media (max-width: 1200px) {
  .main-content {
    flex-direction: column;
  }
  
  .executions-section.with-logs {
    flex: 1;
  }
  
  .logs-section {
    min-width: 100%;
    height: 500px;
  }
}

@media (max-width: 768px) {
  .logs-page {
    gap: var(--space-3);
  }
  
  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
