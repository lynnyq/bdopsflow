<template>
  <div class="tasks-page">
    <!-- Stats Cards -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon stat-icon-primary">
          <el-icon :size="24"><List /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ tasks.length }}</div>
          <div class="stat-label">总任务数</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-success">
          <el-icon :size="24"><CircleCheck /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ enabledTasks }}</div>
          <div class="stat-label">启用</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-warning">
          <el-icon :size="24"><Clock /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ cronTasks }}</div>
          <div class="stat-label">定时任务</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-info">
          <el-icon :size="24"><Timer /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ manualTasks }}</div>
          <div class="stat-label">手动任务</div>
        </div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索任务..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-select v-model="filterType" placeholder="任务类型" clearable class="filter-select">
          <el-option label="HTTP" value="http" />
          <el-option label="Shell" value="shell" />
        </el-select>
        <el-select v-model="filterStatus" placeholder="状态" clearable class="filter-select">
          <el-option label="启用" :value="true" />
          <el-option label="停用" :value="false" />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadTasks" :loading="loading">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="handleCreate">
          创建任务
        </el-button>
      </div>
    </div>

    <!-- Tasks Grid -->
    <div v-loading="loading" class="tasks-grid">
      <div
        v-for="task in pagedTasks"
        :key="task.id"
        class="task-card"
        :class="{ 'task-card-disabled': !task.is_enabled }"
      >
        <div class="task-card-header">
          <div class="task-type-badge" :class="task.type">
            <el-icon :size="18">
              <component :is="getTypeIcon(task.type)" />
            </el-icon>
          </div>
          <div class="task-status-toggle">
            <el-switch
              v-model="task.is_enabled"
              @change="() => handleToggleStatus(task)"
              :loading="toggleLoading === task.id"
              size="small"
            />
          </div>
        </div>

        <div class="task-card-body">
          <h3 class="task-name">{{ task.name }}</h3>
          <p class="task-id">ID: {{ task.id }}</p>

          <div class="task-meta">
            <div class="meta-item" v-if="task.cron_expression">
              <el-icon><Calendar /></el-icon>
              <span class="meta-value">{{ task.cron_expression }}</span>
            </div>
            <div class="meta-item">
              <el-icon><Clock /></el-icon>
              <span class="meta-value">{{ task.cron_expression ? '定时' : '手动' }}</span>
            </div>
            <div class="meta-item">
              <el-icon><Timer /></el-icon>
              <span class="meta-value">
                {{ task.timeout_seconds }}s 超时
              </span>
            </div>
          </div>

          <div class="task-execution">
            <div class="execution-info" v-if="task.is_enabled && task.next_execution_time">
              <div class="execution-label">下次执行</div>
              <div class="execution-time">
                <el-icon :size="14"><Timer /></el-icon>
                {{ formatDateTime(task.next_execution_time) }}
              </div>
            </div>
            <div class="execution-info" v-if="task.last_execution_status">
              <div class="execution-label">上次执行</div>
              <div class="execution-result" :class="getResultClass(task.last_execution_status)">
                <span class="result-dot"></span>
                {{ getResultText(task.last_execution_status) }}
              </div>
            </div>
          </div>
        </div>

        <div class="task-card-footer">
          <div class="task-actions">
            <el-button
              type="primary"
              size="small"
              :icon="VideoPlay"
              @click="handleTrigger(task)"
              :loading="triggeringId === task.id"
            >
              运行
            </el-button>
            <el-button
              size="small"
              :icon="View"
              @click="handleViewLastExecution(task)"
            >
              日志
            </el-button>
            <el-button
              size="small"
              :icon="DocumentCopy"
              @click="handleViewHistory(task)"
            >
              历史
            </el-button>
            <el-button
              size="small"
              :icon="Edit"
              @click="handleEdit(task)"
            >
              编辑
            </el-button>
            <el-button
              size="small"
              :icon="Delete"
              type="danger"
              @click="handleDelete(task)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-if="!loading && filteredTasks.length === 0" class="empty-state">
        <div class="empty-icon">
          <el-icon :size="64"><Document /></el-icon>
        </div>
        <div class="empty-text">
          <h3>暂无任务</h3>
          <p>点击右上角按钮创建第一个任务</p>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="filteredTasks.length > 0" class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="filteredTasks.length"
        layout="total, sizes, prev, pager, next, jumper"
        :pager-count="5"
      />
    </div>

    <!-- Task Form Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingTask ? '编辑任务' : '创建任务'"
      width="700px"
      class="task-dialog"
      @close="handleDialogClose"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px" class="task-form">
        <el-form-item label="任务名称" prop="name">
          <el-input v-model="form.name" placeholder="输入任务名称" />
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="任务类型" prop="type">
              <el-select v-model="form.type" placeholder="选择任务类型" style="width: 100%">
                <el-option label="HTTP" value="http">
                  <span class="type-option">
                    <el-icon><Position /></el-icon>
                    HTTP
                  </span>
                </el-option>
                <el-option label="Shell" value="shell">
                  <span class="type-option">
                    <el-icon><Operation /></el-icon>
                    Shell
                  </span>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="超时时间" prop="timeout_seconds">
              <el-input-number
                v-model="form.timeout_seconds"
                :min="1"
                :max="3600"
                placeholder="秒"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="Cron 表达式" prop="cron_expression">
          <div class="cron-input-wrapper">
            <el-input v-model="form.cron_expression" placeholder="留空则为手动触发任务" clearable>
              <template #suffix>
                <div class="cron-hint">秒 分 时 日 月 周</div>
              </template>
            </el-input>
          </div>
        </el-form-item>

        <el-form-item label="任务配置" prop="config">
          <div class="config-card">
            <div v-if="form.type === 'http'" class="config-http">
              <el-form-item label="URL" prop="config.url" class="config-input">
                <el-input v-model="form.config.url" placeholder="https://example.com" />
              </el-form-item>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="方法" prop="config.method" class="config-input">
                    <el-select v-model="form.config.method" style="width: 100%">
                      <el-option label="GET" value="GET" />
                      <el-option label="POST" value="POST" />
                      <el-option label="PUT" value="PUT" />
                      <el-option label="DELETE" value="DELETE" />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="超时" prop="config.timeout" class="config-input">
                    <el-input-number
                      v-model="form.config.timeout"
                      :min="1"
                      :max="300"
                      style="width: 100%"
                    />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-form-item label="请求头" prop="config.headers" class="config-input">
                <el-input
                  v-model="form.config.headers"
                  type="textarea"
                  :rows="3"
                  placeholder='{"Authorization": "Bearer xxx"}'
                />
              </el-form-item>
              <el-form-item label="请求体" prop="config.body" class="config-input">
                <el-input
                  v-model="form.config.body"
                  type="textarea"
                  :rows="3"
                  placeholder="请求体内容"
                />
              </el-form-item>
            </div>
            <div v-if="form.type === 'shell'" class="config-shell">
              <el-form-item label="脚本" prop="config.script" class="config-input">
                <el-input
                  v-model="form.config.script"
                  type="textarea"
                  :rows="8"
                  placeholder="echo 'Hello World'"
                  class="code-textarea"
                />
              </el-form-item>
            </div>
          </div>
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="失败重试" prop="retry_max">
              <el-input-number
                v-model="form.retry_max"
                :min="0"
                :max="10"
                placeholder="重试次数"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="重试间隔" prop="retry_delay_seconds">
              <el-input-number
                v-model="form.retry_delay_seconds"
                :min="1"
                :max="300"
                placeholder="秒"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="初始状态">
          <div class="switch-wrapper">
            <el-switch v-model="form.is_enabled" size="large" />
            <span class="switch-text">{{ form.is_enabled ? '任务将立即启用' : '任务将处于停用状态' }}</span>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            保存
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Task Log Viewer Dialog - 显示最后一次执行日志 -->
    <el-dialog
      v-model="logViewerVisible"
      :title="selectedTaskName"
      width="80%"
      class="log-dialog"
      destroy-on-close
    >
      <TaskLogViewer
        v-if="selectedExecutionId"
        :execution-id="selectedExecutionId"
        :execution-status="selectedExecutionStatus"
        :task-name="selectedTaskName"
        :in-dialog="true"
        @close="logViewerVisible = false"
      />
      <div v-else class="no-execution">
        <el-icon :size="48"><Document /></el-icon>
        <p>暂无执行记录</p>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import {
  Plus,
  List,
  Edit,
  Delete,
  View,
  Document,
  Search,
  Refresh,
  Timer,
  Clock,
  Position,
  Operation,
  Calendar,
  VideoPlay,
  CircleCheck,
  Loading,
  DocumentCopy
} from '@element-plus/icons-vue'
import { taskAPI } from '@/api'
import type { Task, TaskConfig } from '@/types'
import TaskLogViewer from '@/components/TaskLogViewer.vue'

const router = useRouter()
const route = useRoute()

const tasks = ref<Task[]>([])
const loading = ref(false)
const submitting = ref(false)
const toggleLoading = ref<number | null>(null)
const triggeringId = ref<number | null>(null)
const dialogVisible = ref(false)
const editingTask = ref<Task | null>(null)
const formRef = ref<FormInstance>()
const searchQuery = ref('')
const filterType = ref<string | null>(null)
const filterStatus = ref<boolean | null>(null)
const currentPage = ref(1)
const pageSize = ref(20)

const logViewerVisible = ref(false)
const selectedTaskId = ref<number | null>(null)
const selectedTaskName = ref('')
const selectedExecutionId = ref<string | null>(null)
const selectedExecutionStatus = ref<string | undefined>(undefined)

const defaultForm = {
  name: '',
  type: 'http' as const,
  timeout_seconds: 60,
  cron_expression: '',
  config: {
    url: '',
    method: 'GET' as const,
    timeout: 30,
    headers: '',
    body: '',
    script: ''
  } as TaskConfig,
  retry_max: 0,
  retry_delay_seconds: 5,
  is_enabled: true
}

const form = ref({ ...defaultForm })

const rules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择任务类型', trigger: 'change' }]
} satisfies FormRules

const enabledTasks = computed(() => tasks.value.filter(t => t.is_enabled).length)
const cronTasks = computed(() => tasks.value.filter(t => t.cron_expression).length)
const manualTasks = computed(() => tasks.value.filter(t => !t.cron_expression).length)

const filteredTasks = computed(() => {
  return tasks.value.filter(task => {
    const matchSearch = !searchQuery.value ||
      task.name.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      task.id.toString().includes(searchQuery.value)
    const matchType = filterType.value == null || task.type === filterType.value
    const matchStatus = filterStatus.value == null || task.is_enabled === filterStatus.value
    return matchSearch && matchType && matchStatus
  })
})

const pagedTasks = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredTasks.value.slice(start, end)
})

const getTypeIcon = (type: string) => {
  return type === 'http' ? Position : Operation
}

const getTypeColor = (type: string) => {
  return type === 'http' ? '#3b82f6' : '#f59e0b'
}

const getTypeBg = (type: string) => {
  return type === 'http' ? '#dbeafe' : '#fef3c7'
}

const getTypeLabel = (type: string) => {
  return type === 'http' ? 'HTTP' : 'Shell'
}

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

const getResultClass = (status: string) => {
  const map: Record<string, string> = {
    success: 'success',
    failed: 'failed',
    running: 'running',
    pending: 'pending'
  }
  return map[status] || 'none'
}

const getResultText = (status: string) => {
  const map: Record<string, string> = {
    success: '成功',
    failed: '失败',
    running: '运行中',
    pending: '等待中'
  }
  return map[status] || '未执行'
}

const parseConfig = (config: string | TaskConfig): TaskConfig => {
  if (typeof config === 'string') {
    try {
      return JSON.parse(config)
    } catch {
      return {}
    }
  }
  return config || {}
}

const stringifyConfig = (config: TaskConfig | string): string => {
  if (typeof config === 'string') {
    return config
  }
  return JSON.stringify(config)
}

const loadTasks = async () => {
  loading.value = true
  try {
    const res = await taskAPI.list()
    tasks.value = (res.data.items || []).map(task => {
      if (task.config && typeof task.config === 'string') {
        try {
          return {
            ...task,
            config: JSON.parse(task.config)
          }
        } catch {
        }
      }
      return task
    })
  } catch (err: any) {
    ElMessage.error(err.message || '加载任务列表失败')
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  editingTask.value = null
  form.value = { ...defaultForm }
  dialogVisible.value = true
}

const handleEdit = (task: Task) => {
  editingTask.value = task
  const parsedConfig = parseConfig(task.config)
  form.value = {
    name: task.name,
    type: task.type,
    timeout_seconds: task.timeout_seconds,
    cron_expression: task.cron_expression || '',
    config: { ...parsedConfig },
    retry_max: task.retry_count,
    retry_delay_seconds: task.retry_interval,
    is_enabled: task.is_enabled
  }
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const submitData = {
        ...form.value,
        config: stringifyConfig(form.value.config)
      }
      if (editingTask.value) {
        await taskAPI.update(editingTask.value.id, submitData)
        ElMessage.success('任务更新成功')
      } else {
        await taskAPI.create(submitData)
        ElMessage.success('任务创建成功')
      }
      dialogVisible.value = false
      await loadTasks()
    } catch (err: any) {
      ElMessage.error(err.message || '操作失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleDialogClose = () => {
  formRef.value?.resetFields()
  form.value = { ...defaultForm }
}

const handleToggleStatus = async (task: Task) => {
  toggleLoading.value = task.id
  try {
    await taskAPI.update(task.id, { is_enabled: task.is_enabled })
    ElMessage.success(task.is_enabled ? '任务已启用' : '任务已停用')
    await loadTasks()
  } catch (err: any) {
    task.is_enabled = !task.is_enabled
    ElMessage.error(err.message || '操作失败')
  } finally {
    toggleLoading.value = null
  }
}

const handleDelete = async (task: Task) => {
  try {
    await ElMessageBox.confirm(`确定要删除任务 "${task.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await taskAPI.delete(task.id)
    ElMessage.success('任务已删除')
    await loadTasks()
  } catch (err: any) {
    if (err !== 'cancel') {
      ElMessage.error(err.message || '删除失败')
    }
  }
}

const handleTrigger = async (task: Task) => {
  triggeringId.value = task.id
  try {
    await taskAPI.trigger(task.id)
    ElMessage.success('任务已触发')
  } catch (err: any) {
    ElMessage.error(err.message || '触发失败')
  } finally {
    triggeringId.value = null
  }
}

// 查看最后一次执行日志
const handleViewLastExecution = async (task: Task) => {
  selectedTaskId.value = task.id
  selectedTaskName.value = task.name
  selectedExecutionId.value = null
  selectedExecutionStatus.value = undefined

  try {
    const res = await taskAPI.getExecutions(task.id)
    const executions = res.data || []
    if (executions.length > 0) {
      const lastExecution = executions[0]
      selectedExecutionId.value = lastExecution.execution_id || String(lastExecution.id)
      selectedExecutionStatus.value = lastExecution.status
    }
  } catch (err: any) {
    ElMessage.error(err.message || '加载执行记录失败')
  } finally {
    logViewerVisible.value = true
  }
}

// 跳转到日志页面，筛选当前任务名的日志执行历史
const handleViewHistory = (task: Task) => {
  router.push({
    path: '/logs',
    query: {
      task_name: task.name
    }
  })
}

// 监听筛选条件变化，重置页码
watch([searchQuery, filterType, filterStatus], () => {
  currentPage.value = 1
})

onMounted(() => {
  loadTasks()
})
</script>

<style scoped>
.tasks-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  padding-bottom: var(--space-8);
  overflow-y: auto;
}

.tasks-page::-webkit-scrollbar {
  width: 8px;
}

.tasks-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.tasks-page::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
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

.stat-icon-info {
  background: linear-gradient(135deg, rgba(139, 92, 246, 0.1), rgba(139, 92, 246, 0.05));
  color: var(--accent-secondary);
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

/* Toolbar */
.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
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

.search-input {
  width: 280px;
}

.filter-select {
  width: 140px;
}

/* Tasks Grid */
.tasks-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: var(--space-5);
  align-content: start;
  padding-bottom: var(--space-4);
}

.task-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
  box-shadow: var(--shadow-md);
  transition: all var(--duration-normal) var(--ease-out);
  display: flex;
  flex-direction: column;
}

.task-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

.task-card-disabled {
  opacity: 0.6;
}

.task-card-disabled:hover {
  transform: none;
  box-shadow: var(--shadow-md);
}

/* Task Card Header */
.task-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: linear-gradient(180deg, var(--bg-secondary) 0%, transparent 100%);
}

.task-type-badge {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--shadow-sm);
}

.task-type-badge.http {
  background: linear-gradient(135deg, #dbeafe, #bfdbfe);
  color: var(--accent-primary);
}

.task-type-badge.shell {
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  color: var(--accent-warning);
}

/* Task Card Body */
.task-card-body {
  padding: var(--space-5);
  flex: 1;
}

.task-name {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 var(--space-1) 0;
  letter-spacing: -0.01em;
}

.task-id {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0 0 var(--space-4) 0;
}

.task-meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  padding-bottom: var(--space-4);
  border-bottom: 1px dashed var(--border-subtle);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.8rem;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  padding: 6px 12px;
  border-radius: var(--radius-sm);
}

.meta-item .el-icon {
  color: var(--text-muted);
}

.meta-value {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

.task-execution {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.execution-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.execution-label {
  font-size: 0.72rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

.execution-time {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.execution-time .el-icon {
  color: var(--accent-success);
}

.execution-result {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
}

.execution-result.success {
  color: var(--status-success);
}

.execution-result.failed {
  color: var(--status-error);
}

.execution-result.running {
  color: var(--status-running);
}

.execution-result.pending {
  color: var(--status-pending);
}

.result-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.execution-result.success .result-dot {
  background: var(--status-success);
}

.execution-result.failed .result-dot {
  background: var(--status-error);
}

.execution-result.running .result-dot {
  background: var(--status-running);
  animation: pulse 1.5s ease-in-out infinite;
}

.execution-result.pending .result-dot {
  background: var(--status-pending);
}

/* Task Card Footer */
.task-card-footer {
  padding: var(--space-4);
  background: var(--bg-secondary);
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.task-actions {
  display: flex;
  flex-direction: row !important;
  gap: 4px;
  flex-wrap: nowrap;
  width: 100%;
  padding: 0 2px;
  box-sizing: border-box;
}

.task-actions :deep(.el-button) {
  flex: 1 1 0;
  font-size: 0.68rem;
  padding: 5px 2px;
  min-width: 0 !important;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: auto !important;
  height: auto;
}

/* Empty State */
.empty-state {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-16) var(--space-6);
  gap: var(--space-4);
}

.empty-icon {
  color: var(--text-muted);
  opacity: 0.4;
}

.empty-text h3 {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 var(--space-2) 0;
  text-align: center;
}

.empty-text p {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin: 0;
  text-align: center;
}

/* Form Styles */
.task-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.cron-input-wrapper {
  width: 100%;
}

.cron-hint {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  color: var(--text-muted);
  white-space: nowrap;
}

.config-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-4);
}

.config-input {
  margin-bottom: var(--space-3) !important;
}

.config-input:last-child {
  margin-bottom: 0 !important;
}

.code-textarea :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  line-height: 1.6;
}

.switch-wrapper {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.switch-text {
  font-size: 0.9rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
}

.type-option {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* Log Dialog Styles */
.log-dialog :deep(.el-dialog) {
  border-radius: 12px;
  overflow: hidden;
  max-width: 1400px;
}

.log-dialog :deep(.el-dialog__header) {
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-subtle);
  padding: 16px 24px;
}

.log-dialog :deep(.el-dialog__title) {
  font-family: var(--font-display);
  font-weight: 600;
}

.log-dialog :deep(.el-dialog__body) {
  padding: 0;
  height: 80vh;
  max-height: 700px;
}

.log-dialog :deep(.el-dialog__headerbtn) {
  top: 18px;
}

.no-execution {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 60vh;
  color: var(--text-muted);
  gap: 16px;
  font-size: 1rem;
}

/* Pagination */
.pagination-container {
  display: flex;
  justify-content: center;
  padding: var(--space-6);
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-sm);
  margin-top: var(--space-4);
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

.pagination-container :deep(.el-pagination__sizes) {
  .el-select .el-input__wrapper {
    border-radius: var(--radius-md);
    box-shadow: 0 0 0 1px var(--border-default) inset;
    transition: all var(--duration-normal) var(--ease-out);
    
    &:hover {
      box-shadow: 0 0 0 1px var(--accent-primary) inset;
    }
  }
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
  
  &:hover {
    color: var(--accent-primary);
    background: var(--bg-primary);
    transform: translateY(-1px);
  }
}

.pagination-container :deep(.el-pager li.is-active) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
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

.pagination-container :deep(.el-pagination__jump) {
  .el-input__wrapper {
    border-radius: var(--radius-md);
  }
}

/* Responsive */
@media (max-width: 1400px) {
  .tasks-grid {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .tasks-grid {
    grid-template-columns: 1fr;
  }

  .page-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-left {
    flex-wrap: wrap;
  }

  .search-input {
    flex: 1;
    min-width: 200px;
  }
}

@media (max-width: 640px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .task-actions {
    flex-direction: column;
  }
}
</style>
