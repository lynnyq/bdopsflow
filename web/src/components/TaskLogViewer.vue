<!-- 任务执行日志查看组件：支持 SSE 实时日志流、标准输出/错误切换、任务终止等 -->
<template>
  <div class="task-log-viewer">
    <!-- 日志头部：任务名称、状态标签、操作按钮（终止/关闭） -->
    <div class="log-header" v-if="!props.inDialog">
      <div class="log-title">
        <el-icon><Document /></el-icon>
        <span v-if="props.taskName">{{ props.taskName }}</span>
        <span v-else>执行详情</span>
        <el-tag v-if="executionStatus" :type="statusTagType" size="small" class="status-tag">
          {{ statusText }}
        </el-tag>
      </div>
      <div class="log-actions">
        <el-button
          v-if="canCancel"
          :icon="CircleClose"
          size="small"
          class="cancel-btn"
          :loading="isCanceling"
          @click="handleCancel"
        >终止任务</el-button>
        <el-button :icon="Close" size="small" text class="close-btn" @click="$emit('close')">关闭</el-button>
      </div>
    </div>
    <div class="log-header in-dialog" v-else>
      <div class="log-title">
        <el-tag v-if="executionStatus" :type="statusTagType" size="small" class="status-tag">
          {{ statusText }}
        </el-tag>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="log-tabs">
      <el-tab-pane label="执行日志" name="logs">
        <div class="log-body" ref="logBodyRef">
          <div v-if="isLoadingHistory && logs.length === 0" class="log-empty">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>加载历史日志...</span>
          </div>

          <div v-if="logs.length === 0 && !isConnecting && !isLoadingHistory" class="log-empty">
            <el-icon><InfoFilled /></el-icon>
            <span>暂无日志，点击运行按钮开始执行</span>
          </div>

          <div v-if="isConnecting && logs.length === 0 && !isLoadingHistory" class="log-empty">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>等待执行日志...</span>
          </div>

          <div
            v-for="log in logs"
            :key="log.id || log._key"
            :class="['log-line', `level-${log.log_level || log.level}`]"
          >
            <span class="log-time">{{ formatLogTime(log.log_time || log.timestamp) }}</span>
            <span class="log-level-tag" :class="log.log_level || log.level">{{ (log.log_level || log.level || 'info').toUpperCase() }}</span>
            <span class="log-msg">{{ log.message || log.log_content }}</span>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="标准输出" name="output">
        <div class="output-body">
          <div v-if="!realtimeOutput" class="log-empty">
            <el-icon><InfoFilled /></el-icon>
            <span>暂无标准输出</span>
          </div>
          <pre v-else class="output-content">{{ realtimeOutput }}</pre>
        </div>
      </el-tab-pane>

      <el-tab-pane label="标准错误" name="error">
        <div class="output-body">
          <div v-if="!realtimeError" class="log-empty">
            <el-icon><InfoFilled /></el-icon>
            <span>暂无标准错误</span>
          </div>
          <pre v-else class="error-content">{{ realtimeError }}</pre>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
/**
 * TaskLogViewer.vue - 任务执行日志查看组件
 *
 * 通过 SSE（EventSource）订阅 `/task-executions/{executionId}/logs` 实时日志流，
 * 同时提供历史日志拉取、标准输出/错误 Tab 切换、任务终止按钮等功能。
 * 可作为独立页面或嵌套在 Dialog 中（通过 inDialog prop 控制是否显示头部）。
 */
// ========== Vue 核心 API ==========
import { ref, watch, onUnmounted, nextTick, computed } from 'vue'
// ========== Element Plus 图标 ==========
import { Document, Close, InfoFilled, Loading, CircleClose } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { taskAPI } from '@/api'
// ========== 工具函数 ==========
import { secureRandom } from '@/utils/random'
import { formatErrorMessage } from '@/utils/error'
// ========== Element Plus ==========
import { ElMessage, ElMessageBox } from 'element-plus'

/** 日志条目最大缓存数量，超过后丢弃最早的条目避免内存溢出 */
const MAX_LOG_ENTRIES = 5000
/** SSE 断线后重连基础间隔（毫秒），采用指数退避 */
const SSE_RECONNECT_DELAY = 3000
/** SSE 最大重连次数，超过后停止自动重连并提示用户 */
const SSE_MAX_RECONNECTS = 5

/** 日志条目数据结构（兼容历史日志拉取和 SSE 推送两种来源） */
interface LogEntry {
  id?: number
  _key?: string
  execution_id?: string
  task_id?: number
  node_id?: string
  log_level?: string
  level?: string
  message?: string
  log_content?: string
  log_time?: string
  timestamp?: number
}

/**
 * 组件 props 定义
 * - executionId: 任务执行 ID，用于拉取历史日志和建立 SSE 连接
 * - executionStatus: 当前执行状态（success/failed/running/pending），影响 UI 标签显示
 * - taskName: 任务名称，用于头部标题
 * - output/error: 初始标准输出/错误内容（可选传入）
 * - inDialog: 是否嵌套在 Dialog 中，为 true 时隐藏头部区域
 */
const props = defineProps<{
  executionId: string
  executionStatus?: string
  taskName?: string
  output?: string
  error?: string
  inDialog?: boolean
}>()

/** 对外事件：关闭日志查看器 */
const emit = defineEmits<{
  close: []
}>()

// ========== DOM 引用 ==========
const logBodyRef = ref<HTMLElement>()              // 日志区域 DOM 引用（用于自动滚动到底部）

// ========== SSE 连接状态 ==========
const eventSource = ref<EventSource | null>(null)    // SSE EventSource 实例
const reconnectCount = ref(0)                         // 当前重连次数

// ========== 日志数据 ==========
const logs = ref<LogEntry[]>([])                     // 日志条目列表（SSE 实时推送）
const isConnecting = ref(false)                      // SSE 连接中状态
const isLoadingHistory = ref(false)                  // 历史日志加载状态
const activeTab = ref('logs')                        // 当前激活的 Tab（logs/output/error）
const isCanceling = ref(false)                       // 终止任务加载状态

// 实时输出和错误
const realtimeOutput = ref('')                       // 任务标准输出（SSE 推送）
const realtimeError = ref('')                        // 任务标准错误（SSE 推送）
const currentStatus = ref(props.executionStatus || '')  // 当前执行状态（随 SSE 事件更新）

const canCancel = computed(() => {
  return currentStatus.value === 'running' || currentStatus.value === 'pending'
})

/**
 * 终止当前正在运行的任务执行
 * @returns {Promise<void>}
 */
const handleCancel = async () => {
  if (!props.executionId || !canCancel.value) return

  try {
    await ElMessageBox.confirm(
      '确认要终止当前任务吗？此操作不可恢复。',
      '终止任务',
      {
        confirmButtonText: '终止',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger',
        customClass: 'cancel-task-dialog'
      }
    )
    
    isCanceling.value = true
    await taskAPI.cancelExecution(props.executionId)
    ElMessage.success('任务已终止')
    currentStatus.value = 'failed'
  } catch (err: unknown) {
    if (err !== 'cancel') {
      ElMessage.error(formatErrorMessage(err))
    }
  } finally {
    isCanceling.value = false
  }
}

/**
 * 加载任务执行的历史日志列表，并收集 stdout/stderr 内容
 * @returns {Promise<void>}
 */
const loadHistoryLogs = async () => {
  if (!props.executionId) return
  
  isLoadingHistory.value = true
  try {
    const res = await taskAPI.getExecutionLogs(props.executionId)
    if (res.data && res.data.length > 0) {
      logs.value = res.data.map(log => ({
        ...log,
        _key: `${log.id || Date.now()}-${secureRandom()}`
      }))
      
      // 同时从历史日志中收集 stdout 和 stderr 的内容
      let historyOutput = ''
      let historyError = ''
      for (const log of res.data) {
        const logLevel = log.log_level || log.level
        const message = log.message || log.log_content
        if (logLevel === 'stdout' && message) {
          historyOutput += message
        }
        if (logLevel === 'stderr' && message) {
          historyError += message
        }
      }
      
      // 更新实时输出和错误
      realtimeOutput.value = historyOutput
      realtimeError.value = historyError
      
      scrollToBottom()
    }
  } catch (err) {
  } finally {
    isLoadingHistory.value = false
  }
}

const statusText = computed(() => {
  const map: Record<string, string> = {
    pending: '待执行',
    running: '运行中',
    success: '成功',
    failed: '失败',
  }
  return map[currentStatus.value || ''] || currentStatus.value || ''
})

const statusTagType = computed(() => {
  const map: Record<string, string> = {
    pending: 'info',
    running: 'warning',
    success: 'success',
    failed: 'danger',
  }
  return map[currentStatus.value || ''] || 'info'
})

/**
 * 建立 SSE 连接实时推送任务日志，支持自动重连
 */
const connectSSE = () => {
  if (!props.executionId) return
  disconnectSSE()

  isConnecting.value = true
  // 记录当前已有的最大日志 ID，避免重复
  let maxLogId = 0
  for (const log of logs.value) {
    const logId = log.id || 0
    if (logId > maxLogId) {
      maxLogId = logId
    }
  }
  
  const token = sessionStorage.getItem('token') || ''
  const url = `/api/logs/stream?execution_id=${props.executionId}&token=${token}`

  const es = new EventSource(url)
  eventSource.value = es

  // 监听后端发送的 done 事件，任务终态时后端主动关闭 SSE
  es.addEventListener('done', () => {
    disconnectSSE()
  })

  es.onmessage = (event) => {
    if (event.data.startsWith(': heartbeat')) return
    try {
      const data = JSON.parse(event.data)
      
      // 判断是执行更新还是日志
      if (data.type === 'execution_update') {
        // 只更新状态，忽略 output 和 error，完全依赖实时日志更新
        if (data.status) {
          currentStatus.value = data.status
        }
        // 任务进入终态时主动断开 SSE，避免空转
        if (data.status === 'success' || data.status === 'failed') {
          disconnectSSE()
        }
      } else {
        // 处理普通日志
        const logId = data.id || 0
        
        // 只处理比历史日志 ID 更大的日志，避免重复
        if (logId > maxLogId) {
          maxLogId = logId
          
          logs.value.push({
            ...data,
            _key: `${Date.now()}-${secureRandom()}`,
          })

          if (logs.value.length > MAX_LOG_ENTRIES) {
            logs.value = logs.value.slice(logs.value.length - MAX_LOG_ENTRIES)
          }

          scrollToBottom()
          
          // 如果是 stdout 或 stderr 日志，实时更新对应的区域
          const logLevel = data.log_level || data.level
          const message = data.message || data.log_content
          
          if (logLevel === 'stdout' && message) {
            realtimeOutput.value += message
          }
          if (logLevel === 'stderr' && message) {
            realtimeError.value += message
          }
        }
      }
    } catch {
      // ignore parse errors
    }
  }

  es.onopen = () => {
    isConnecting.value = false
    reconnectCount.value = 0
  }

  es.onerror = () => {
    isConnecting.value = false
    if (reconnectCount.value < SSE_MAX_RECONNECTS) {
      const status = currentStatus.value
      if (status !== 'success' && status !== 'failed') {
        reconnectCount.value++
        setTimeout(() => {
          if (props.executionId && status === currentStatus.value) {
            connectSSE()
          }
        }, SSE_RECONNECT_DELAY)
      }
    }
  }
}

/**
 * 断开 SSE 连接并清理 EventSource 对象
 */
const disconnectSSE = () => {
  if (eventSource.value) {
    eventSource.value.close()
    eventSource.value = null
  }
}

/**
 * 将日志容器滚动到底部，显示最新日志
 */
const scrollToBottom = () => {
  nextTick(() => {
    if (logBodyRef.value) {
      logBodyRef.value.scrollTop = logBodyRef.value.scrollHeight
    }
  })
}

watch(() => props.executionId, async (newId) => {
  if (newId) {
    logs.value = []
    realtimeOutput.value = ''
    realtimeError.value = ''
    currentStatus.value = props.executionStatus || ''
    activeTab.value = 'logs'
    await loadHistoryLogs()
    // 仅运行中/待执行的任务才连接 SSE，已完成任务数据不会变化，无需长连接
    if (['running', 'pending'].includes(props.executionStatus || '')) {
      connectSSE()
    }
  } else {
    disconnectSSE()
    logs.value = []
    realtimeOutput.value = ''
    realtimeError.value = ''
  }
}, { immediate: true })

// 移除从 props 同步 output/error 的逻辑，避免与历史日志收集的内容重复

watch(() => props.executionStatus, (val) => {
  if (val && !currentStatus.value) {
    currentStatus.value = val
  }
})

onUnmounted(() => {
  disconnectSSE()
})

/**
 * 格式化日志时间为 YYYY-MM-DD HH:mm:ss，支持秒级时间戳
 * @param {string | number | undefined} timeStr - 原始时间
 * @returns {string} 格式化后的时间
 */
const formatLogTime = (timeStr: string | number | undefined): string => {
  if (!timeStr) return ''
  try {
    let d: Date
    if (typeof timeStr === 'number') {
      d = new Date(timeStr * 1000)
    } else {
      d = new Date(timeStr)
    }
    if (isNaN(d.getTime())) {
      return String(timeStr)
    }
    const year = d.getFullYear()
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    const hours = String(d.getHours()).padStart(2, '0')
    const minutes = String(d.getMinutes()).padStart(2, '0')
    const seconds = String(d.getSeconds()).padStart(2, '0')
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  } catch {
    return String(timeStr)
  }
}
</script>

<style scoped>
.task-log-viewer {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #0d1117;
  border-radius: 8px;
  overflow: hidden;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 18px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
}

.log-header.in-dialog {
  padding: 12px 18px;
  background: transparent;
  border-bottom: none;
}

.log-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #e6edf3;
  font-weight: 600;
  font-size: 14px;
}

.log-title .el-icon {
  color: #58a6ff;
}

.status-tag {
  margin-left: 8px;
}

.log-actions {
  display: flex;
  gap: 8px;
}

.cancel-btn {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.15), rgba(239, 68, 68, 0.05));
  color: #ff6b6b;
  border: 1px solid rgba(239, 68, 68, 0.3);
  font-weight: 600;
  border-radius: 6px;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(239, 68, 68, 0.1);
}

.cancel-btn:hover {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.25), rgba(239, 68, 68, 0.1));
  color: #ff8080;
  border-color: rgba(239, 68, 68, 0.5);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.2);
}

.cancel-btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 6px rgba(239, 68, 68, 0.15);
}

.close-btn {
  color: #8b949e;
  font-weight: 500;
  transition: all 0.2s ease;
}

.close-btn:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 6px;
}

.log-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

:deep(.el-tabs__header) {
  margin: 0;
  background: #161b22;
  border-bottom: 1px solid #30363d;
}

:deep(.el-tabs__nav-wrap) {
  padding: 0 18px;
}

:deep(.el-tabs__item) {
  color: #8b949e;
  font-weight: 500;
}

:deep(.el-tabs__item:hover) {
  color: #e6edf3;
}

:deep(.el-tabs__item.is-active) {
  color: #58a6ff;
  font-weight: 600;
}

:deep(.el-tabs__active-bar) {
  background-color: #58a6ff;
  height: 2px;
}

:deep(.el-tabs__content) {
  flex: 1;
  overflow: hidden;
  padding: 0;
}

:deep(.el-tab-pane) {
  height: 100%;
  overflow: hidden;
}

.log-body,
.output-body {
  height: 100%;
  overflow-y: auto;
  padding: 16px 20px;
  font-family: 'SF Mono', 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13.5px;
  line-height: 1.6;
  background: #0d1117;
}

.log-body::-webkit-scrollbar,
.output-body::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.log-body::-webkit-scrollbar-thumb,
.output-body::-webkit-scrollbar-thumb {
  background: #30363d;
  border-radius: 5px;
}

.log-body::-webkit-scrollbar-thumb:hover,
.output-body::-webkit-scrollbar-thumb:hover {
  background: #484f58;
}

.log-body::-webkit-scrollbar-track,
.output-body::-webkit-scrollbar-track {
  background: #0d1117;
}

.log-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #8b949e;
  gap: 12px;
}

.log-empty .el-icon {
  font-size: 40px;
  color: #30363d;
}

.log-line {
  display: flex;
  gap: 10px;
  padding: 6px 0;
  border-bottom: 1px solid #21262d;
  align-items: flex-start;
}

.log-time {
  color: #6e7681;
  flex-shrink: 0;
  font-size: 12px;
}

.log-level-tag {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  flex-shrink: 0;
  letter-spacing: 0.3px;
}

.level-info .log-level-tag {
  background: rgba(88, 166, 255, 0.15);
  color: #58a6ff;
  border: 1px solid rgba(88, 166, 255, 0.3);
}

.level-warn .log-level-tag,
.level-warning .log-level-tag {
  background: rgba(237, 150, 66, 0.15);
  color: #ffa657;
  border: 1px solid rgba(237, 150, 66, 0.3);
}

.level-error .log-level-tag {
  background: rgba(248, 113, 113, 0.15);
  color: #ff6b6b;
  border: 1px solid rgba(248, 113, 113, 0.3);
}

.level-debug .log-level-tag {
  background: rgba(149, 157, 165, 0.15);
  color: #8b949e;
  border: 1px solid rgba(149, 157, 165, 0.3);
}

.level-success .log-level-tag {
  background: rgba(72, 207, 173, 0.15);
  color: #3fb950;
  border: 1px solid rgba(72, 207, 173, 0.3);
}

.level-stdout .log-level-tag {
  background: rgba(46, 160, 67, 0.15);
  color: #7ee787;
  border: 1px solid rgba(46, 160, 67, 0.3);
}

.level-stderr .log-level-tag {
  background: rgba(248, 113, 113, 0.15);
  color: #ff6b6b;
  border: 1px solid rgba(248, 113, 113, 0.3);
}

.log-msg {
  color: #e6edf3;
  word-break: break-word;
}

.output-content,
.error-content {
  margin: 0;
  padding: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 14px;
  line-height: 1.7;
}

.output-content {
  color: #7ee787;
  background: rgba(46, 160, 67, 0.03);
  padding: 12px;
  border-radius: 6px;
  border: 1px solid rgba(46, 160, 67, 0.2);
}

.error-content {
  color: #ffa6a6;
  background: rgba(248, 113, 113, 0.05);
  padding: 12px;
  border-radius: 6px;
  border: 1px solid rgba(248, 113, 113, 0.25);
}
</style>
