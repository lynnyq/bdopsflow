<template>
  <div class="tasks-page">
    <div class="main-content">
      <div class="tasks-section" :class="{ 'with-logs': showLogs }">
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
        <el-button :icon="Refresh" @click="loadTasks" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Plus" @click="handleCreate" class="create-btn">
          创建任务
        </el-button>
      </div>
    </div>

    <!-- Tasks Grid -->
    <div v-loading="loading" class="tasks-grid" ref="tasksGridRef">
      <div
        v-for="task in pagedTasks"
        :key="task.id"
        :ref="el => setTaskCardRef(el, task.id)"
        class="task-card"
        :class="{ 
          'task-card-disabled': !task.is_enabled,
          'task-card-highlighted': selectedTaskId === task.id
        }"
      >
        <div class="task-card-header">
          <div class="task-header-left">
            <div class="task-type-badge" :class="task.type">
              <el-icon :size="18">
                <component :is="getTypeIcon(task.type)" />
              </el-icon>
            </div>
            <div class="task-title-info">
              <h3 class="task-name">{{ task.name }}</h3>
              <p class="task-id">ID: {{ task.id }}</p>
            </div>
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

    </div>

    <!-- Logs Section -->
    <div class="logs-section" v-if="showLogs">
      <TaskLogViewer
        v-if="selectedExecutionId"
        :execution-id="selectedExecutionId"
        :execution-status="selectedExecutionStatus"
        :output="selectedExecutionOutput"
        :error="selectedExecutionError"
        @close="closeLogs"
      />
      <div v-else class="no-execution-container">
        <div class="log-header">
          <div class="log-title">
            <el-icon><Document /></el-icon>
            <span>执行详情</span>
          </div>
          <div class="log-actions">
            <el-button :icon="Close" size="small" text @click="closeLogs">关闭</el-button>
          </div>
        </div>
        <el-tabs class="log-tabs">
          <el-tab-pane label="执行日志" name="logs">
            <div class="log-body">
              <div class="log-empty">
                <el-icon><InfoFilled /></el-icon>
                <span>暂无执行记录</span>
              </div>
            </div>
          </el-tab-pane>
          <el-tab-pane label="标准输出" name="output">
            <div class="output-body">
              <div class="log-empty">
                <el-icon><InfoFilled /></el-icon>
                <span>暂无标准输出</span>
              </div>
            </div>
          </el-tab-pane>
          <el-tab-pane label="标准错误" name="error">
            <div class="output-body">
              <div class="log-empty">
                <el-icon><InfoFilled /></el-icon>
                <span>暂无标准错误</span>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>
    </div>

    <!-- Task Form Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingTask ? '编辑任务' : '创建任务'"
      width="800px"
      class="task-dialog"
      :close-on-click-modal="false"
      @close="handleDialogClose"
    >
      <div class="task-form-container">
        <el-form ref="formRef" :model="form" :rules="rules" class="task-form">
          <!-- 基本信息 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Document /></el-icon>
              <span>基本信息</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="任务名称" prop="name" class="form-item">
                    <el-input 
                      v-model="form.name" 
                      placeholder="请输入任务名称"
                      class="form-input"
                    >
                      <template #prefix>
                        <el-icon><Bell /></el-icon>
                      </template>
                    </el-input>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="任务类型" prop="type" class="form-item">
                    <el-select 
                      v-model="form.type" 
                      placeholder="请选择任务类型" 
                      class="form-select"
                    >
                      <el-option label="HTTP 请求" value="http">
                        <span class="option-content">
                          <el-icon :size="16"><Connection /></el-icon>
                          HTTP 请求
                        </span>
                      </el-option>
                      <el-option label="Shell 脚本" value="shell">
                        <span class="option-content">
                          <el-icon :size="16"><Cpu /></el-icon>
                          Shell 脚本
                        </span>
                      </el-option>
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>

              <!-- 执行器选择 -->
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="执行器" class="form-item">
                    <el-select
                      v-model="form.assigned_executor_id"
                      clearable
                      placeholder="默认调度（自动选择）"
                      class="form-select"
                    >
                      <template #prefix>
                        <el-icon><Cpu /></el-icon>
                      </template>
                      <el-option label="默认调度（自动选择）" value="" />
                      <el-option
                        v-for="executor in executors"
                        :key="executor.executor_id"
                        :label="`${executor.name} (${executor.current_load}/${executor.capacity})`"
                        :value="executor.executor_id"
                      >
                        <div class="executor-option">
                          <span>{{ executor.name }}</span>
                          <span class="executor-status" :class="executor.status">
                            {{ executor.status === 'online' ? '在线' : '离线' }}
                          </span>
                          <span class="executor-load">
                            负载: {{ executor.current_load }}/{{ executor.capacity }}
                          </span>
                        </div>
                      </el-option>
                    </el-select>
                    <div class="form-tip">
                      <el-icon><InfoFilled /></el-icon>
                      <span>留空则使用默认调度算法，指定执行器则任务将在该执行器上执行</span>
                    </div>
                  </el-form-item>
                </el-col>
              </el-row>
            </div>
          </div>

          <!-- 调度配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Clock /></el-icon>
              <span>调度配置</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="执行频率" prop="cron_expression" class="form-item">
                    <div class="cron-wrapper">
                      <el-select 
                        v-model="cronPreset" 
                        placeholder="选择预设" 
                        class="cron-preset-select"
                        @change="handleCronPresetChange"
                      >
                        <el-option label="手动触发" value="manual">
                          <span class="option-content">
                            <el-icon :size="14"><DataAnalysis /></el-icon>
                            手动触发
                          </span>
                        </el-option>
                        <el-option label="每30秒" value="30sec">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每30秒
                          </span>
                        </el-option>
                        <el-option label="每分钟" value="minute">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每分钟
                          </span>
                        </el-option>
                        <el-option label="每5分钟" value="5minute">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每5分钟
                          </span>
                        </el-option>
                        <el-option label="每10分钟" value="10minute">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每10分钟
                          </span>
                        </el-option>
                        <el-option label="每小时" value="hour">
                          <span class="option-content">
                            <el-icon :size="14"><Clock /></el-icon>
                            每小时
                          </span>
                        </el-option>
                        <el-option label="每天" value="day">
                          <span class="option-content">
                            <el-icon :size="14"><Calendar /></el-icon>
                            每天 00:00
                          </span>
                        </el-option>
                        <el-option label="每周" value="week">
                          <span class="option-content">
                            <el-icon :size="14"><Calendar /></el-icon>
                            每周一 00:00
                          </span>
                        </el-option>
                        <el-option label="每月" value="month">
                          <span class="option-content">
                            <el-icon :size="14"><Calendar /></el-icon>
                            每月1日 00:00
                          </span>
                        </el-option>
                        <el-option label="自定义" value="custom">
                          <span class="option-content">
                            <el-icon :size="14"><Setting /></el-icon>
                            自定义表达式
                          </span>
                        </el-option>
                      </el-select>
                    </div>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="Cron 值" prop="cron_expression" class="form-item">
                    <el-input 
                      v-model="form.cron_expression" 
                      :placeholder="cronPlaceholder"
                      :disabled="cronPreset !== 'custom'"
                      class="form-input"
                    >
                      <template #suffix>
                        <span class="cron-hint" title="支持5位（分 时 日 月 周）或6位（秒 分 时 日 月 周）格式">秒 分 时 日 月 周</span>
                      </template>
                    </el-input>
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="超时时间" prop="timeout_seconds" class="form-item">
                    <div class="timeout-input-wrapper">
                      <el-input-number
                        v-model="form.timeout_seconds"
                        :min="0"
                        :max="3600"
                        placeholder="秒"
                        class="form-input-number"
                        controls-position="right"
                      >
                      </el-input-number>
                      <span class="timeout-hint">秒 (0=不限制)</span>
                    </div>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <div class="empty-col"></div>
                </el-col>
              </el-row>
            </div>
          </div>

          <!-- 任务配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Setting /></el-icon>
              <span>任务配置</span>
            </div>
            <div class="section-body">
              <!-- HTTP 配置 -->
              <div v-if="form.type === 'http'" class="config-panel">
                <el-row :gutter="16">
                  <el-col :span="24">
                    <el-form-item label="请求 URL" prop="config.url" class="form-item">
                      <el-input 
                        v-model="form.config.url" 
                        placeholder="https://example.com/api"
                        class="form-input"
                      >
                        <template #prefix>
                          <el-icon><Connection /></el-icon>
                        </template>
                      </el-input>
                    </el-form-item>
                  </el-col>
                </el-row>
                <el-row :gutter="16">
                  <el-col :span="8">
                    <el-form-item label="请求方法" prop="config.method" class="form-item">
                      <el-select 
                        v-model="form.config.method" 
                        class="form-select"
                      >
                        <el-option label="GET" value="GET" />
                        <el-option label="POST" value="POST" />
                        <el-option label="PUT" value="PUT" />
                        <el-option label="DELETE" value="DELETE" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                  <el-col :span="8">
                    <el-form-item label="连接超时" prop="config.timeout" class="form-item">
                      <el-input-number
                        v-model="form.config.timeout"
                        :min="1"
                        :max="300"
                        placeholder="秒"
                        class="form-input-number"
                      >
                        <template #suffix>
                          <span>秒</span>
                        </template>
                      </el-input-number>
                    </el-form-item>
                  </el-col>
                  <el-col :span="8">
                    <div class="empty-col"></div>
                  </el-col>
                </el-row>
                <el-row :gutter="16">
                  <el-col :span="12">
                    <el-form-item label="请求头" prop="config.headers" class="form-item">
                      <el-input
                        v-model="form.config.headers"
                        type="textarea"
                        :rows="3"
                        placeholder='{"Authorization": "Bearer xxx"}'
                        class="form-textarea"
                      />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="请求体" prop="config.body" class="form-item">
                      <el-input
                        v-model="form.config.body"
                        type="textarea"
                        :rows="3"
                        placeholder="请求体内容（JSON或表单）"
                        class="form-textarea"
                      />
                    </el-form-item>
                  </el-col>
                </el-row>
              </div>

              <!-- Shell 配置 -->
              <div v-if="form.type === 'shell'" class="config-panel">
                <el-form-item label="Shell 脚本" prop="config.script" class="form-item">
                  <div class="script-editor">
                    <el-input
                      v-model="form.config.script"
                      type="textarea"
                      :rows="8"
                      placeholder="echo 'Hello World'"
                      class="form-textarea script-textarea"
                    />
                  </div>
                </el-form-item>
              </div>
            </div>
          </div>

          <!-- 重试配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Refresh /></el-icon>
              <span>重试配置</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="失败重试次数" prop="retry_count" class="form-item">
                    <el-input-number
                      v-model="form.retry_count"
                      :min="0"
                      :max="10"
                      placeholder="重试次数"
                      class="form-input-number"
                    >
                      <template #suffix>
                        <span>次</span>
                      </template>
                    </el-input-number>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="重试间隔" prop="retry_interval" class="form-item">
                    <el-input-number
                      v-model="form.retry_interval"
                      :min="1"
                      :max="300"
                      placeholder="秒"
                      class="form-input-number"
                    >
                      <template #suffix>
                        <span>秒</span>
                      </template>
                    </el-input-number>
                  </el-form-item>
                </el-col>
              </el-row>
            </div>
          </div>

          <!-- 初始状态 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><SwitchButton /></el-icon>
              <span>初始状态</span>
            </div>
            <div class="section-body">
              <div class="switch-wrapper">
                <el-switch v-model="form.is_enabled" size="large" />
                <span class="switch-text">{{ form.is_enabled ? '任务将立即启用' : '任务将处于停用状态' }}</span>
              </div>
            </div>
          </div>

          <!-- Webhook推送配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Connection /></el-icon>
              <span>Webhook推送配置</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="Webhook URL" class="form-item">
                    <el-input 
                      v-model="form.webhook_url" 
                      placeholder="https://example.com/webhook"
                      class="form-input"
                    >
                      <template #prefix>
                        <el-icon><Link /></el-icon>
                      </template>
                    </el-input>
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="推送时机" class="form-item">
                    <el-select 
                      v-model="form.webhook_events" 
                      multiple
                      placeholder="选择推送时机"
                      class="form-select"
                    >
                      <el-option label="任务成功" value="success" />
                      <el-option label="任务失败" value="failed" />
                      <el-option label="每次执行" value="*" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
            </div>
          </div>
        </el-form>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            {{ editingTask ? '更新' : '创建' }}
          </el-button>
        </div>
      </template>
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
  Calendar,
  VideoPlay,
  CircleCheck,
  Loading,
  DocumentCopy,
  Close,
  InfoFilled,
  Tools,
  Promotion,
  Connection,
  Cpu,
  Bell,
  Setting,
  SwitchButton,
  DataAnalysis,
  Link
} from '@element-plus/icons-vue'
import { taskAPI, executorAPI } from '@/api'
import type { Task, TaskConfig, Executor } from '@/types'
import TaskLogViewer from '@/components/TaskLogViewer.vue'

const router = useRouter()
const route = useRoute()

const tasks = ref<Task[]>([])
const executors = ref<Executor[]>([])
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

const showLogs = ref(false)
const selectedTaskId = ref<number | null>(null)
const selectedTaskName = ref('')
const selectedExecutionId = ref<string | null>(null)
const selectedExecutionStatus = ref<string | undefined>(undefined)
const selectedExecutionOutput = ref<string | null | undefined>(undefined)
const selectedExecutionError = ref<string | null | undefined>(undefined)
const tasksGridRef = ref<HTMLElement | null>(null)
const taskCardRefs = ref<Map<number, HTMLElement>>(new Map())

// Cron 预设相关
const cronPreset = ref('manual')

const cronPlaceholder = computed(() => {
  if (cronPreset.value === 'manual') return '手动触发'
  if (cronPreset.value === 'custom') return '输入 Cron 表达式（支持5位或6位）'
  return form.value.cron_expression || ''
})

const handleCronPresetChange = (preset: string) => {
  const presets: Record<string, string> = {
    manual: '',
    '30sec': '*/30 * * * * *',
    minute: '* * * * *',
    '5minute': '*/5 * * * *',
    '10minute': '*/10 * * * *',
    hour: '0 * * * *',
    day: '0 0 * * *',
    week: '0 0 * * 1',
    month: '0 0 1 * *',
    custom: form.value.cron_expression || ''
  }
  if (preset !== 'custom') {
    form.value.cron_expression = presets[preset] || ''
  }
}

const setTaskCardRef = (el: any, taskId: number) => {
  if (el) {
    taskCardRefs.value.set(taskId, el)
  } else {
    taskCardRefs.value.delete(taskId)
  }
}

const defaultForm = {
  name: '',
  type: 'shell' as const,
  timeout_seconds: 0,
  cron_expression: '',
  config: {
    url: '',
    method: 'GET' as const,
    timeout: 30,
    headers: '',
    body: '',
    script: ''
  } as TaskConfig,
  retry_count: 0,
  retry_interval: 5,
  is_enabled: true,
  webhook_url: '',
  webhook_events: [] as string[],
  assigned_executor_id: '' // 指定执行器，空表示使用默认调度
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
  return type === 'http' ? Promotion : Tools
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
  cronPreset.value = 'manual'
  dialogVisible.value = true
}

const handleEdit = (task: Task) => {
  editingTask.value = task
  const parsedConfig = parseConfig(task.config)
  
  // 解析webhook配置
  let webhookUrl = ''
  let webhookEvents: string[] = []
  if (task.webhook_config) {
    try {
      const parsedWebhook = typeof task.webhook_config === 'string' 
        ? JSON.parse(task.webhook_config) 
        : task.webhook_config
      webhookUrl = parsedWebhook.url || ''
      webhookEvents = parsedWebhook.events || []
    } catch {
      // 解析失败时使用默认值
    }
  }
  
  form.value = {
    name: task.name,
    type: task.type,
    timeout_seconds: task.timeout_seconds,
    cron_expression: task.cron_expression || '',
    config: { ...parsedConfig },
    retry_count: task.retry_count,
    retry_interval: task.retry_interval,
    is_enabled: task.is_enabled,
    webhook_url: webhookUrl,
    webhook_events: webhookEvents,
    assigned_executor_id: task.assigned_executor_id || ''
  }
  
  // 根据当前任务的 cron 表达式设置预设
  const cron = task.cron_expression || ''
  if (!cron) {
    cronPreset.value = 'manual'
  } else if (cron === '* * * * *') {
    cronPreset.value = 'minute'
  } else if (cron === '*/5 * * * *') {
    cronPreset.value = '5minute'
  } else if (cron === '*/10 * * * *') {
    cronPreset.value = '10minute'
  } else if (cron === '0 * * * *') {
    cronPreset.value = 'hour'
  } else if (cron === '0 0 * * *') {
    cronPreset.value = 'day'
  } else if (cron === '0 0 * * 1') {
    cronPreset.value = 'week'
  } else if (cron === '0 0 1 * *') {
    cronPreset.value = 'month'
  } else {
    cronPreset.value = 'custom'
  }
  
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      // 构建 webhook 配置
      let webhookConfig = ''
      if (form.value.webhook_url) {
        webhookConfig = JSON.stringify({
          url: form.value.webhook_url,
          method: 'POST',
          headers: {},
          events: form.value.webhook_events || []
        })
      }
      
      const submitData = {
        ...form.value,
        config: stringifyConfig(form.value.config),
        webhook_config: webhookConfig
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
  cronPreset.value = 'manual'
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
  selectedExecutionOutput.value = undefined
  selectedExecutionError.value = undefined

  try {
    const res = await taskAPI.getExecutions(task.id)
    const executions = res.data || []
    if (executions.length > 0) {
      const lastExecution = executions[0]
      selectedExecutionId.value = lastExecution.execution_id || String(lastExecution.id)
      selectedExecutionStatus.value = lastExecution.status
      selectedExecutionOutput.value = lastExecution.output
      selectedExecutionError.value = lastExecution.error
    }
  } catch (err: any) {
    ElMessage.error(err.message || '加载执行记录失败')
  } finally {
    showLogs.value = true
    
    // 滚动到被点击的任务卡片
    await nextTick()
    const taskCardEl = taskCardRefs.value.get(task.id)
    if (taskCardEl) {
      taskCardEl.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }
}

// 关闭日志查看
const closeLogs = () => {
  showLogs.value = false
  selectedExecutionId.value = null
  selectedExecutionStatus.value = undefined
  selectedExecutionOutput.value = undefined
  selectedExecutionError.value = undefined
  selectedTaskId.value = null
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

// 确保 timeout_seconds 始终是数字类型，且 0 值被正确保留
watch(() => form.value.timeout_seconds, (newVal) => {
  if (typeof newVal !== 'number') {
    form.value.timeout_seconds = 0
  }
}, { immediate: true })

onMounted(async () => {
  await Promise.all([
    loadTasks(),
    loadExecutors()
  ])
})

const loadExecutors = async () => {
  try {
    const response = await executorAPI.list()
    executors.value = response.data
  } catch (error) {
    console.error('Failed to load executors:', error)
    ElMessage.error('加载执行器列表失败')
  }
}
</script>

<style scoped>
.tasks-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  padding-bottom: var(--space-8);
  overflow-y: auto;
  height: 100%;
}

/* Main Content */
.main-content {
  display: flex;
  gap: var(--space-4);
  min-height: 0;
  flex: 1;
}

/* Tasks Section */
.tasks-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-1);
}

.tasks-section.with-logs {
  flex: 0 0 60%;
}

/* Logs Section */
.logs-section {
  flex: 1;
  min-width: 400px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* No Execution Container - Dark Theme */
.no-execution-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #0d1117;
  border-radius: 8px;
  overflow: hidden;
}

.no-execution-container .log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 18px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
}

.no-execution-container .log-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #e6edf3;
  font-weight: 600;
  font-size: 14px;
}

.no-execution-container .log-title .el-icon {
  color: #58a6ff;
}

.no-execution-container .log-actions {
  display: flex;
  gap: 4px;
}

.no-execution-container .log-actions .el-button {
  color: #8b949e;
}

.no-execution-container .log-actions .el-button:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1);
}

.no-execution-container .log-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.no-execution-container .log-tabs :deep(.el-tabs__header) {
  margin: 0;
  background: #161b22;
  border-bottom: 1px solid #30363d;
}

.no-execution-container .log-tabs :deep(.el-tabs__nav-wrap) {
  padding: 0 18px;
}

.no-execution-container .log-tabs :deep(.el-tabs__item) {
  color: #8b949e;
  font-weight: 500;
}

.no-execution-container .log-tabs :deep(.el-tabs__item:hover) {
  color: #e6edf3;
}

.no-execution-container .log-tabs :deep(.el-tabs__item.is-active) {
  color: #58a6ff;
  font-weight: 600;
}

.no-execution-container .log-tabs :deep(.el-tabs__active-bar) {
  background-color: #58a6ff;
  height: 2px;
}

.no-execution-container .log-tabs :deep(.el-tabs__content) {
  flex: 1;
  overflow: hidden;
  padding: 0;
}

.no-execution-container .log-tabs :deep(.el-tab-pane) {
  height: 100%;
  overflow: hidden;
}

.no-execution-container .log-body,
.no-execution-container .output-body {
  height: 100%;
  overflow-y: auto;
  padding: 16px 20px;
  font-family: 'SF Mono', 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13.5px;
  line-height: 1.6;
  background: #0d1117;
}

.no-execution-container .log-body::-webkit-scrollbar,
.no-execution-container .output-body::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.no-execution-container .log-body::-webkit-scrollbar-thumb,
.no-execution-container .output-body::-webkit-scrollbar-thumb {
  background: #30363d;
  border-radius: 5px;
}

.no-execution-container .log-body::-webkit-scrollbar-thumb:hover,
.no-execution-container .output-body::-webkit-scrollbar-thumb:hover {
  background: #484f58;
}

.no-execution-container .log-body::-webkit-scrollbar-track,
.no-execution-container .output-body::-webkit-scrollbar-track {
  background: #0d1117;
}

.no-execution-container .log-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #8b949e;
  gap: 12px;
}

.no-execution-container .log-empty .el-icon {
  font-size: 40px;
  color: #30363d;
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

.search-input :deep(.el-input__wrapper),
.filter-select :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.search-input :deep(.el-input__wrapper:hover),
.filter-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.search-input :deep(.el-input__wrapper.is-focus),
.filter-select :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.filter-select {
  width: 140px;
}

/* Toolbar Buttons */
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

.create-btn {
  font-weight: 500;
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-secondary) 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 20px;
}

.create-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

.create-btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
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

.task-card-highlighted {
  border: 2px solid var(--accent-primary);
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.15), var(--shadow-lg), var(--shadow-glow);
  z-index: 10;
}

.task-card-highlighted:hover {
  transform: translateY(-4px);
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

.task-header-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
  min-width: 0;
}

.task-title-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
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
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-id {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0;
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

/* 任务表单新样式 */
.task-form-container {
  padding: 8px 0;
}

.task-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.3s ease;
}

.form-section:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.1);
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 20px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(99, 102, 241, 0.05));
  border-bottom: 1px solid var(--border-default);
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.section-body {
  padding: 20px;
}

.form-item {
  margin-bottom: 0 !important;
}

.form-input,
.form-select,
.cron-preset-select,
.form-input-number {
  width: 100%;
}

.form-input :deep(.el-input__wrapper),
.form-select :deep(.el-input__wrapper),
.cron-preset-select :deep(.el-input__wrapper) {
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  box-shadow: none;
  transition: all 0.2s ease;
  padding: 6px 12px;
}

.form-input :deep(.el-input__wrapper:hover),
.form-select :deep(.el-input__wrapper:hover),
.cron-preset-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input :deep(.el-input__wrapper.is-focus),
.form-select :deep(.el-input__wrapper.is-focus),
.cron-preset-select :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}

.form-textarea :deep(.el-textarea__inner) {
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  font-family: var(--font-mono);
  font-size: 13px;
  transition: all 0.2s ease;
}

.form-textarea :deep(.el-textarea__inner:hover) {
  border-color: var(--accent-primary);
}

.form-textarea :deep(.el-textarea__inner:focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}

.cron-wrapper {
  width: 100%;
}

.cron-hint {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.option-content {
  display: flex;
  align-items: center;
  gap: 6px;
}

.config-panel {
  width: 100%;
}

.script-editor {
  width: 100%;
}

.script-textarea :deep(.el-textarea__inner) {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
  font-size: 14px;
  line-height: 1.6;
  padding: 16px;
}

.empty-col {
  height: 1px;
}

.task-dialog :deep(.el-dialog__header) {
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--border-default);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.03), rgba(99, 102, 241, 0.03));
}

.task-dialog :deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.task-dialog :deep(.el-dialog__body) {
  padding: 24px;
  max-height: 70vh;
  overflow-y: auto;
}

.task-dialog :deep(.el-dialog__footer) {
  padding: 16px 24px 20px;
  border-top: 1px solid var(--border-default);
  background: var(--bg-secondary);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.dialog-footer :deep(.el-button) {
  padding: 10px 24px;
  font-size: 14px;
  font-weight: 500;
  border-radius: 8px;
}

.dialog-footer :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  border: none;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  transition: all 0.2s ease;
}

.dialog-footer :deep(.el-button--primary:hover) {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

.timeout-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-left: 8px;
  white-space: nowrap;
}

.timeout-input-wrapper {
  display: flex;
  align-items: center;
  width: 100%;
}

.timeout-input-wrapper .form-input-number {
  flex: 0 0 140px;
}

.form-input-number :deep(.el-input-number__decrease),
.form-input-number :deep(.el-input-number__increase) {
  border-color: var(--border-default);
}

.form-input-number :deep(.el-input__wrapper) {
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  box-shadow: none;
  transition: all 0.2s ease;
}

.form-input-number :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

.form-input-number :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
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

@media (max-width: 1200px) {
  .main-content {
    flex-direction: column;
  }
  
  .tasks-section.with-logs {
    flex: 1;
  }
  
  .logs-section {
    min-width: 100%;
    height: 500px;
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

@media (max-width: 768px) {
  .tasks-page {
    gap: var(--space-4);
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
