<template>
  <div class="task-log-viewer">
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
        <el-button :icon="Close" size="small" text @click="$emit('close')">关闭</el-button>
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
import { ref, watch, onUnmounted, nextTick, computed } from 'vue'
import { Document, Close, InfoFilled, Loading } from '@element-plus/icons-vue'
import { taskAPI } from '@/api'

const MAX_LOG_ENTRIES = 5000;

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

const props = defineProps<{
  executionId: string
  executionStatus?: string
  taskName?: string
  output?: string
  error?: string
  inDialog?: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const logBodyRef = ref<HTMLElement>()
const logs = ref<LogEntry[]>([])
const isConnecting = ref(false)
const isLoadingHistory = ref(false)
const eventSource = ref<EventSource | null>(null)
const activeTab = ref('logs')

// 实时输出和错误
const realtimeOutput = ref('')
const realtimeError = ref('')
const currentStatus = ref(props.executionStatus || '')

const loadHistoryLogs = async () => {
  if (!props.executionId) return
  
  isLoadingHistory.value = true
  try {
    const res = await taskAPI.getExecutionLogs(props.executionId)
    if (res.data && res.data.length > 0) {
      logs.value = res.data.map(log => ({
        ...log,
        _key: `${log.id || Date.now()}-${Math.random()}`
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
  
  const token = localStorage.getItem('token') || ''
  const url = `/api/logs/stream?execution_id=${props.executionId}&token=${token}`

  const es = new EventSource(url)
  eventSource.value = es

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
      } else {
        // 处理普通日志
        const logId = data.id || 0
        
        // 只处理比历史日志 ID 更大的日志，避免重复
        if (logId > maxLogId) {
          maxLogId = logId
          
          logs.value.push({
            ...data,
            _key: `${Date.now()}-${Math.random()}`,
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
  }

  es.onerror = () => {
    isConnecting.value = false
  }
}

const disconnectSSE = () => {
  if (eventSource.value) {
    eventSource.value.close()
    eventSource.value = null
  }
}

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
    connectSSE()
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
  gap: 4px;
}

.log-actions .el-button {
  color: #8b949e;
}

.log-actions .el-button:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1);
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
