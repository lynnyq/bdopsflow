<template>
  <div class="logs-container">
    <div class="page-header">
      <h2>任务执行历史</h2>
      <p>查看所有任务的执行记录和详细日志</p>
    </div>

    <div class="main-content">
      <div class="executions-section" :class="{ 'with-logs': showLogs }">
        <!-- 筛选栏 -->
        <div class="filter-bar">
          <div class="filter-grid">
            <div class="filter-item">
              <label class="filter-label">执行节点</label>
              <el-select
                v-model="filters.executor_name"
                placeholder="选择执行节点"
                clearable
                class="filter-select"
                @change="loadExecutions(1)"
              >
                <el-option label="全部" value="" />
                <el-option
                  v-for="exec in executors"
                  :key="exec.executor_id"
                  :label="exec.name"
                  :value="exec.name"
                />
              </el-select>
            </div>
            
            <div class="filter-item">
              <label class="filter-label">任务名称</label>
              <el-select
                v-model="filters.task_name"
                placeholder="选择任务名称"
                clearable
                class="filter-select"
                @change="loadExecutions(1)"
              >
                <el-option label="全部" value="" />
                <el-option
                  v-for="task in tasks"
                  :key="task.id"
                  :label="task.name"
                  :value="task.name"
                />
              </el-select>
            </div>
            
            <div class="filter-item">
              <label class="filter-label">任务类型</label>
              <el-select
                v-model="filters.task_type"
                placeholder="选择任务类型"
                clearable
                class="filter-select"
                @change="loadExecutions(1)"
              >
                <el-option label="全部" value="" />
                <el-option label="Shell" value="shell" />
                <el-option label="HTTP" value="http" />
                <el-option label="Delay" value="delay" />
              </el-select>
            </div>
            
            <div class="filter-item">
              <label class="filter-label">执行状态</label>
              <el-select
                v-model="filters.status"
                placeholder="选择执行状态"
                clearable
                class="filter-select"
                @change="loadExecutions(1)"
              >
                <el-option label="全部" value="" />
                <el-option label="成功" value="success" />
                <el-option label="失败" value="failed" />
                <el-option label="运行中" value="running" />
                <el-option label="待执行" value="pending" />
              </el-select>
            </div>
          </div>
          
          <div class="filter-actions">
            <el-button type="primary" @click="loadExecutions(1)" class="refresh-btn">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
            <el-button
              v-if="selectedIds && selectedIds.length > 0"
              type="danger"
              @click="handleBatchDelete"
              class="delete-btn"
            >
              <el-icon><Delete /></el-icon>
              批量删除 ({{ selectedIds ? selectedIds.length : 0 }})
            </el-button>
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
            <el-table-column prop="execution_id" label="执行ID" min-width="180" show-overflow-tooltip />
            <el-table-column label="任务名称" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">
                {{ row.task_name || '-' }}
              </template>
            </el-table-column>
            <el-table-column label="执行节点" min-width="150" show-overflow-tooltip>
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
            <el-table-column label="操作" width="160" fixed="right" align="center">
              <template #default="{ row }">
                <el-button
                  type="primary"
                  link
                  size="small"
                  @click.stop="viewLogs(row)"
                >
                  查看日志
                </el-button>
                <el-button
                  type="danger"
                  link
                  size="small"
                  @click.stop="handleDelete(row)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
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

        <div class="empty-state" v-if="!loading && (!executions || executions.length === 0)">
          <el-empty description="暂无执行记录" />
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
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Delete } from '@element-plus/icons-vue'
import { logAPI, taskAPI, executorAPI } from '@/api'
import TaskLogViewer from '@/components/TaskLogViewer.vue'
import type { TaskExecutionListResponse, Task, Executor } from '@/types'

const executions = ref<TaskExecutionListResponse[]>([])
const executors = ref<Executor[]>([])
const tasks = ref<Task[]>([])
const loading = ref(false)
const showLogs = ref(false)
const currentExecutionId = ref('')
const currentExecution = ref<TaskExecutionListResponse | null>(null)
const selectedIds = ref<number[]>([])

const filters = ref({
  executor_name: '',
  task_name: '',
  task_type: '',
  status: ''
})

// 分页相关
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

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
    console.log('Loading executions with filters:', filters.value, 'page:', page, 'pageSize:', pageSize.value)
    const response = await logAPI.list({
      ...filters.value,
      page: page,
      page_size: pageSize.value
    })
    console.log('Received response:', response.data)
    // 安全地赋值，默认为空数组
    executions.value = response.data.data || []
    total.value = response.data.total || 0
    currentPage.value = response.data.page || 1
  } catch (error) {
    console.error('Failed to load executions:', error)
    ElMessage.error('加载执行记录失败')
  } finally {
    loading.value = false
  }
}

const loadExecutors = async () => {
  try {
    const response = await executorAPI.list()
    executors.value = response.data || []
  } catch (error) {
    console.error('加载执行器列表失败', error)
  }
}

const loadTasks = async () => {
  try {
    const response = await taskAPI.list()
    tasks.value = response.data || []
  } catch (error) {
    console.error('加载任务列表失败', error)
  }
}

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
      `确认要删除这个执行记录吗？此操作将同时删除所有相关日志。`,
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
    if (error !== 'cancel') {
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
    if (error !== 'cancel') {
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
  loadExecutions()
  loadExecutors()
  loadTasks()
})
</script>

<style scoped>
.logs-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
  padding: 0;
}

.page-header {
  background: linear-gradient(135deg, #f5f7fa 0%, #e4e8ed 100%);
  padding: 20px 24px;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.page-header h2 {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 700;
  color: #1e293b;
}

.page-header p {
  margin: 8px 0 0;
  color: #64748b;
  font-size: 0.9375rem;
}

.main-content {
  display: flex;
  gap: 20px;
  flex: 1;
  min-height: 0;
}

.executions-section {
  flex: 1;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px 0 rgb(0 0 0 / 0.1);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.executions-section.with-logs {
  flex: 0 0 60%;
}

.table-wrapper {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 400px;
}

.filter-bar {
  background: linear-gradient(to bottom, #f8fafc 0%, #ffffff 100%);
  padding: 20px;
  border-bottom: 1px solid #e2e8f0;
}

.filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.filter-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.filter-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #475569;
  margin: 0;
}

.filter-select {
  width: 100%;
}

.filter-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  border-top: 1px solid #f1f5f9;
  padding-top: 16px;
}

.refresh-btn {
  font-weight: 500;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  border: none;
}

.refresh-btn:hover {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
}

.delete-btn {
  font-weight: 500;
  background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%);
  border: none;
}

.delete-btn:hover {
  background: linear-gradient(135deg, #dc2626 0%, #b91c1c 100%);
}

.pagination-container {
  padding: 20px;
  display: flex;
  justify-content: flex-end;
  background: #f8fafc;
  border-top: 1px solid #e2e8f0;
}

.empty-state {
  padding: 60px 20px;
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logs-section {
  flex: 1;
  min-width: 400px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px 0 rgb(0 0 0 / 0.1);
  overflow: hidden;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 6px;
}

.dot-info {
  background: #60a5fa;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.2);
}

.dot-warning {
  background: #f59e0b;
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.2);
}

.dot-success {
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.2);
}

.dot-danger {
  background: #ef4444;
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.2);
}

:deep(.el-table .current-row) {
  background-color: #eff6ff !important;
}

:deep(.el-table .current-row:hover > td) {
  background-color: #dbeafe !important;
}

:deep(.el-pagination.is-background .el-pager li.is-active) {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
}

@media (max-width: 1200px) {
  .main-content {
    flex-direction: column;
  }
  
  .executions-section.with-logs {
    flex: 1;
  }
  
  .logs-section {
    min-width: 100%;
    height: 400px;
  }
}

@media (max-width: 768px) {
  .logs-container {
    gap: 12px;
  }
  
  .page-header {
    padding: 16px;
  }
  
  .page-header h2 {
    font-size: 1.25rem;
  }
  
  .filter-bar {
    padding: 16px;
  }
  
  .filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
