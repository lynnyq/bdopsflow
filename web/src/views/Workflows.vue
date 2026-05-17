<template>
  <div class="workflows-page">
    <div class="workflows-grid" v-if="!selectedWorkflow">
      <!-- Toolbar -->
      <div class="page-toolbar">
        <div class="toolbar-left">
          <el-input
            v-model="searchQuery"
            placeholder="搜索工作流..."
            :prefix-icon="Search"
            class="search-input"
            clearable
          />
        </div>
        <div class="toolbar-right">
          <el-button :icon="Refresh" @click="loadWorkflows" :loading="loading" class="refresh-btn">刷新</el-button>
          <el-button v-if="canManageWorkflow" :icon="Plus" @click="openCreateDialog" class="create-btn">
            创建工作流
          </el-button>
        </div>
      </div>

      <div
        v-for="workflow in filteredWorkflows"
        :key="workflow.id"
        class="workflow-card"
        @click="selectWorkflow(workflow)"
      >
        <div class="card-header">
          <div class="card-icon">
            <el-icon><component :is="getWorkflowIcon(workflow)" /></el-icon>
          </div>
          <div class="card-meta">
            <span class="badge" :class="workflow.is_enabled ? 'enabled' : 'disabled'">
              {{ workflow.is_enabled ? '启用' : '禁用' }}
            </span>
          </div>
        </div>

        <div class="card-body">
          <h3>{{ workflow.name }}</h3>
          <p>{{ workflow.description }}</p>
        </div>

        <div class="card-footer">
          <div class="card-stats">
            <div class="stat">
              <el-icon><Document /></el-icon>
              <span>{{ getNodeCount(workflow) }} 个节点</span>
            </div>
            <div class="stat">
              <el-icon><Timer /></el-icon>
              <span>{{ workflow.cron_expression || '手动' }}</span>
            </div>
          </div>
          <div class="card-actions">
            <el-button v-if="canManageWorkflow" :icon="Edit" text size="small" @click.stop="editWorkflow(workflow)">
              编辑
            </el-button>
            <el-button v-if="canManageWorkflow" :icon="Delete" text size="small" type="danger" @click.stop="deleteWorkflowConfirm(workflow)">
              删除
            </el-button>
          </div>
        </div>
      </div>

      <div v-if="canManageWorkflow" class="workflow-card create-card" @click="openCreateDialog">
        <div class="create-content">
          <div class="create-icon">
            <el-icon><Plus /></el-icon>
          </div>
          <h3>创建新工作流</h3>
          <p>设计一个新的分布式工作流</p>
        </div>
      </div>
    </div>

    <div v-else class="workflow-editor">
      <div class="editor-header">
        <div class="editor-info">
          <el-button :icon="ArrowLeft" text @click="exitEditor" />
          <div class="info-text">
            <h2>{{ selectedWorkflow.name }}</h2>
            <p>{{ selectedWorkflow.description }}</p>
          </div>
        </div>
        <div class="editor-actions">
          <el-button :icon="VideoPlay" type="success" @click="triggerWorkflow" :loading="isRunning">
            运行工作流
          </el-button>
          <el-button :icon="List" @click="showHistoryDialog = true">
            运行历史
          </el-button>
          <el-button :icon="Check" type="primary" @click="saveWorkflow">
            保存工作流
          </el-button>
        </div>
      </div>

      <div class="editor-toolbar">
        <div class="toolbar-info">
          <span class="info-item">
            <el-icon><Timer /></el-icon>
            上次运行: {{ formatTime(selectedWorkflow.updated_at) || '从未' }}
          </span>
          <span class="info-item" v-if="currentExecution">
            <el-icon><Loading /></el-icon>
            当前状态: {{ getStatusText(currentExecution.status) }}
          </span>
        </div>
      </div>

      <div class="editor-content">
        <FlowCanvas 
          :initial-dag="currentDAG"
          :node-states="nodeStates"
          @update="handleDAGUpdate"
        />
      </div>

      <!-- 日志面板 -->
      <div class="log-panel" v-if="showLogPanel">
        <div class="log-panel-header">
          <h3>执行日志</h3>
          <el-button :icon="Close" text @click="showLogPanel = false" />
        </div>
        <div class="log-panel-content">
          <div v-for="log in currentLogs" :key="log.id" class="log-item" :class="log.log_level">
            <span class="log-time">{{ formatLogTime(log.log_time) }}</span>
            <span class="log-level" :class="log.log_level">[{{ log.log_level.toUpperCase() }}]</span>
            <span class="log-node" v-if="log.node_id">[{{ log.node_id }}]</span>
            <span class="log-message">{{ log.message }}</span>
          </div>
          <div v-if="currentLogs.length === 0" class="log-empty">
            暂无日志
          </div>
        </div>
      </div>
    </div>

    <!-- 创建/编辑工作流对话框 -->
    <el-dialog
      v-model="showCreateDialog"
      :title="editingWorkflow ? '编辑工作流' : '创建新工作流'"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form :model="workflowForm" label-position="top" class="create-form">
        <el-form-item label="工作流名称" :error="formErrors.name">
          <el-input
            v-model="workflowForm.name"
            placeholder="输入您的工作流描述性名称"
            size="large"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="workflowForm.description"
            type="textarea"
            :rows="3"
            placeholder="描述这个工作流的功能"
          />
        </el-form-item>
        <el-form-item label="调度 (Cron 表达式)">
          <el-input
            v-model="workflowForm.cron_expression"
            placeholder="例如，0 0 * * * (每天午夜)"
          />
          <div class="form-tip">
            留空表示手动执行。使用 cron 表达式进行调度。
          </div>
        </el-form-item>
        <el-form-item label="域">
          <el-select v-model="workflowForm.domain_id" placeholder="选择域">
            <el-option label="默认域" :value="1" />
            <el-option label="生产环境" :value="2" />
            <el-option label="开发环境" :value="3" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="closeDialog">取消</el-button>
          <el-button type="primary" @click="submitWorkflow">
            {{ editingWorkflow ? '保存修改' : '创建工作流' }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 运行历史对话框 -->
    <el-dialog
      v-model="showHistoryDialog"
      title="运行历史"
      width="800px"
    >
      <el-table :data="workflowExecutions" stripe>
        <el-table-column prop="execution_id" label="执行ID" width="200" />
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="getStatusTagType(row.status)">{{ getStatusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="start_time" label="开始时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.start_time) }}
          </template>
        </el-table-column>
        <el-table-column prop="end_time" label="结束时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.end_time) }}
          </template>
        </el-table-column>
        <el-table-column label="操作">
          <template #default="{ row }">
            <el-button text size="small" @click="viewExecutionLogs(row)">查看日志</el-button>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showHistoryDialog = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 删除确认对话框 -->
    <el-dialog
      v-model="showDeleteConfirm"
      title="确认删除"
      width="400px"
    >
      <p>确定要删除工作流 <strong>{{ deleteTarget?.name }}</strong> 吗？此操作不可撤销。</p>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showDeleteConfirm = false">取消</el-button>
          <el-button type="danger" @click="deleteWorkflow">删除</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { 
  Plus, 
  ArrowLeft, 
  VideoPlay, 
  Edit, 
  Delete, 
  Document, 
  Timer, 
  List,
  Check,
  Loading,
  Close,
  Search,
  Refresh
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import FlowCanvas from '@/components/FlowCanvas.vue'
import { workflowAPI } from '@/api'
import type { Workflow, WorkflowDAG, WorkflowExecution, TaskLog } from '@/types'
import { handleError, handleSuccess, formatValue } from '@/utils/error'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const workflows = ref<Workflow[]>([])
const selectedWorkflow = ref<Workflow | null>(null)
const showCreateDialog = ref(false)
const showDeleteConfirm = ref(false)
const showHistoryDialog = ref(false)
const editingWorkflow = ref<Workflow | null>(null)
const deleteTarget = ref<Workflow | null>(null)
const currentDAG = ref<WorkflowDAG>({ nodes: [], connections: [] })
const currentExecution = ref<WorkflowExecution | null>(null)
const currentLogs = ref<TaskLog[]>([])
const workflowExecutions = ref<WorkflowExecution[]>([])
const showLogPanel = ref(false)
const nodeStates = ref<Record<string, string>>({})
const isRunning = ref(false)
const loading = ref(false)
const searchQuery = ref('')

const filteredWorkflows = computed(() => {
  if (!searchQuery.value) return workflows.value
  const query = searchQuery.value.toLowerCase()
  return workflows.value.filter(w => 
    w.name.toLowerCase().includes(query) || 
    w.description?.toLowerCase().includes(query)
  )
})

const canManageWorkflow = computed(() => {
  const role = authStore.user?.role
  return role === 'admin' || role === 'system_admin' || role === 'domain_admin'
})

let logPollingInterval: number | null = null
let executionPollingInterval: number | null = null

const formErrors = reactive({
  name: ''
})

const workflowForm = reactive({
  name: '',
  description: '',
  cron_expression: '',
  domain_id: 1
})

const formatTime = (timeStr: string | null | undefined): string => {
  if (!timeStr) return ''
  try {
    const date = new Date(timeStr)
    return date.toLocaleString('zh-CN')
  } catch {
    return timeStr
  }
}

const formatLogTime = (timeStr: string): string => {
  if (!timeStr) return ''
  try {
    const date = new Date(timeStr)
    return date.toLocaleTimeString('zh-CN', { 
      hour12: false, 
      hour: '2-digit', 
      minute: '2-digit', 
      second: '2-digit' 
    })
  } catch {
    return timeStr
  }
}

const getStatusText = (status: string): string => {
  const statusMap: Record<string, string> = {
    pending: '待执行',
    running: '运行中',
    success: '成功',
    failed: '失败'
  }
  return statusMap[status] || status
}

const getStatusTagType = (status: string): string => {
  const typeMap: Record<string, string> = {
    pending: 'info',
    running: 'warning',
    success: 'success',
    failed: 'danger'
  }
  return typeMap[status] || 'info'
}

const loadWorkflows = async () => {
  try {
    const response = await workflowAPI.list()
    workflows.value = response.data
  } catch (error) {
    console.error('加载工作流失败:', error)
    workflows.value = []
  }
}

const selectWorkflow = async (workflow: Workflow) => {
  selectedWorkflow.value = workflow
  await loadWorkflowDAG(workflow.id)
  await loadWorkflowExecutions(workflow.id)
  currentExecution.value = null
  currentLogs.value = []
  nodeStates.value = {}
  showLogPanel.value = false
}

const loadWorkflowDAG = async (workflowId: number) => {
  try {
    const response = await workflowAPI.get(workflowId)
    const workflow = response.data
    if (workflow.dag_config) {
      try {
        currentDAG.value = JSON.parse(workflow.dag_config)
      } catch {
        currentDAG.value = { nodes: [], connections: [] }
      }
    } else {
      currentDAG.value = { nodes: [], connections: [] }
    }
  } catch {
    currentDAG.value = { nodes: [], connections: [] }
  }
}

const loadWorkflowExecutions = async (workflowId: number) => {
  try {
    const response = await workflowAPI.getExecutions(workflowId)
    workflowExecutions.value = response.data
  } catch (error) {
    console.error('加载工作流执行历史失败:', error)
    workflowExecutions.value = []
  }
}

const handleDAGUpdate = (dag: WorkflowDAG) => {
  currentDAG.value = dag
}

const openCreateDialog = () => {
  editingWorkflow.value = null
  workflowForm.name = ''
  workflowForm.description = ''
  workflowForm.cron_expression = ''
  workflowForm.domain_id = authStore.user?.domain_id || 1
  formErrors.name = ''
  showCreateDialog.value = true
}

const editWorkflow = (workflow: Workflow) => {
  editingWorkflow.value = workflow
  workflowForm.name = workflow.name
  workflowForm.description = workflow.description
  workflowForm.cron_expression = workflow.cron_expression || ''
  workflowForm.domain_id = workflow.domain_id
  showCreateDialog.value = true
}

const closeDialog = () => {
  showCreateDialog.value = false
  editingWorkflow.value = null
  workflowForm.name = ''
  workflowForm.description = ''
  workflowForm.cron_expression = ''
  workflowForm.domain_id = authStore.user?.domain_id || 1
  formErrors.name = ''
}

const submitWorkflow = async () => {
  formErrors.name = ''
  
  if (!workflowForm.name.trim()) {
    formErrors.name = '请输入工作流名称'
    return
  }

  try {
    if (editingWorkflow.value) {
      await workflowAPI.update(editingWorkflow.value.id, {
        name: workflowForm.name,
        description: workflowForm.description,
        cron_expression: workflowForm.cron_expression,
        domain_id: workflowForm.domain_id
      })
      ElMessage.success('工作流更新成功')
    } else {
      await workflowAPI.create({
        name: workflowForm.name,
        description: workflowForm.description,
        cron_expression: workflowForm.cron_expression,
        domain_id: workflowForm.domain_id,
        dag_config: JSON.stringify({ nodes: [], connections: [] })
      })
      ElMessage.success('工作流创建成功')
    }
    await loadWorkflows()
    closeDialog()
  } catch (error) {
    console.error('保存工作流失败:', error)
    ElMessage.error('保存工作流失败')
  }
}

const saveWorkflow = async () => {
  if (!selectedWorkflow.value) return

  try {
    await workflowAPI.update(selectedWorkflow.value.id, {
      dag_config: JSON.stringify(currentDAG.value)
    })
    await loadWorkflows()
    selectedWorkflow.value = workflows.value.find(w => w.id === selectedWorkflow.value?.id) || null
    ElMessage.success('工作流保存成功')
  } catch (error) {
    console.error('保存工作流失败:', error)
    ElMessage.error('保存工作流失败')
  }
}

const deleteWorkflowConfirm = (workflow: Workflow) => {
  deleteTarget.value = workflow
  showDeleteConfirm.value = true
}

const deleteWorkflow = async () => {
  if (!deleteTarget.value) return

  try {
    await workflowAPI.delete(deleteTarget.value.id)
    await loadWorkflows()
    showDeleteConfirm.value = false
    deleteTarget.value = null
    ElMessage.success('工作流删除成功')
  } catch (error) {
    console.error('删除工作流失败:', error)
    ElMessage.error('删除工作流失败')
  }
}

const exitEditor = () => {
  selectedWorkflow.value = null
  currentDAG.value = { nodes: [], connections: [] }
  currentExecution.value = null
  currentLogs.value = []
  showLogPanel.value = false
  stopPolling()
}

const getWorkflowIcon = (workflow: Workflow) => {
  if (workflow.cron_expression?.includes('*/')) return Timer
  if (workflow.cron_expression) return Timer
  return Document
}

const getNodeCount = (workflow: Workflow): number => {
  if (!workflow.dag_config) return 0
  try {
    const dag: WorkflowDAG = JSON.parse(workflow.dag_config)
    return dag.nodes?.length || 0
  } catch {
    return 0
  }
}

// 触发工作流运行
const triggerWorkflow = async () => {
  if (!selectedWorkflow.value || isRunning.value) return
  
  try {
    isRunning.value = true
    const response = await workflowAPI.trigger(selectedWorkflow.value.id)
    currentExecution.value = response.data
    showLogPanel.value = true
    
    // 开始轮询
    startPolling()
    
    ElMessage.success('工作流已启动')
  } catch (error) {
    console.error('启动工作流失败:', error)
    ElMessage.error('启动工作流失败')
    isRunning.value = false
  }
}

// 查看执行日志
const viewExecutionLogs = async (execution: WorkflowExecution) => {
  currentExecution.value = execution
  try {
    const response = await workflowAPI.getExecutionLogs(execution.execution_id)
    currentLogs.value = response.data
  } catch (error) {
    console.error('加载日志失败:', error)
    currentLogs.value = []
  }
  
  // 解析节点状态
  if (execution.node_states) {
    try {
      nodeStates.value = JSON.parse(execution.node_states)
    } catch {
      nodeStates.value = {}
    }
  }
  
  showLogPanel.value = true
  showHistoryDialog.value = false
}

// 开始轮询
const startPolling = () => {
  if (!currentExecution.value) return
  
  // 轮询日志
  logPollingInterval = window.setInterval(async () => {
    if (!currentExecution.value) return
    try {
      const response = await workflowAPI.getExecutionLogs(currentExecution.value.execution_id)
      currentLogs.value = response.data
    } catch (error) {
      console.error('轮询日志失败:', error)
    }
  }, 1000)
  
  // 轮询执行状态
  executionPollingInterval = window.setInterval(async () => {
    if (!currentExecution.value) return
    try {
      const response = await workflowAPI.getExecution(currentExecution.value.execution_id)
      currentExecution.value = response.data
      
      // 更新节点状态
      if (response.data.node_states) {
        try {
          nodeStates.value = JSON.parse(response.data.node_states)
        } catch {
          nodeStates.value = {}
        }
      }
      
      // 如果执行完成，停止轮询
      if (['success', 'failed'].includes(response.data.status)) {
        stopPolling()
        isRunning.value = false
        // 刷新执行历史
        if (selectedWorkflow.value) {
          await loadWorkflowExecutions(selectedWorkflow.value.id)
        }
      }
    } catch (error) {
      console.error('轮询执行状态失败:', error)
    }
  }, 1000)
}

// 停止轮询
const stopPolling = () => {
  if (logPollingInterval) {
    clearInterval(logPollingInterval)
    logPollingInterval = null
  }
  if (executionPollingInterval) {
    clearInterval(executionPollingInterval)
    executionPollingInterval = null
  }
}

onMounted(() => {
  loadWorkflows()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.workflows-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
  overflow-y: auto;
}

.workflows-page::-webkit-scrollbar {
  width: 8px;
}

.workflows-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.workflows-page::-webkit-scrollbar-track {
  background: var(--bg-secondary);
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

.search-input {
  width: 280px;
}

.search-input :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.search-input :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.search-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
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

.workflows-grid {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.workflow-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  cursor: pointer;
  transition: all var(--duration-normal) var(--ease-out);
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  box-shadow: var(--shadow-sm);
}

.workflow-card:hover {
  border-color: var(--accent-secondary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.card-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, rgba(6, 182, 212, 0.1), rgba(6, 182, 212, 0.05));
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-secondary);
  font-size: 1.5rem;
}

.badge {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 9999px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.badge.enabled {
  background: rgba(52, 211, 153, 0.15);
  color: var(--accent-success);
  border: 1px solid rgba(52, 211, 153, 0.2);
}

.badge.disabled {
  background: rgba(148, 163, 184, 0.15);
  color: var(--text-muted);
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.card-body h3 {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0 0 8px 0;
  color: var(--text-primary);
}

.card-body p {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

.card-stats {
  display: flex;
  gap: var(--space-4);
}

.stat {
  display: flex;
  align-items: center;
  gap: 4px;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
}

.card-actions {
  display: flex;
  gap: 8px;
  opacity: 0;
  transition: opacity var(--duration-normal) var(--ease-out);
}

.workflow-card:hover .card-actions {
  opacity: 1;
}

.create-card {
  border-style: dashed;
  border-color: var(--border-default);
  background: transparent;
  min-height: 200px;
  justify-content: center;
  align-items: center;
}

.create-card:hover {
  border-color: var(--accent-secondary);
  background: rgba(6, 182, 212, 0.02);
}

.create-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  gap: var(--space-3);
}

.create-icon {
  width: 64px;
  height: 64px;
  background: rgba(6, 182, 212, 0.1);
  border: 2px dashed var(--accent-secondary);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-secondary);
  font-size: 1.5rem;
  margin-bottom: 8px;
  transition: all var(--duration-normal) var(--ease-out);
}

.create-card:hover .create-icon {
  background: var(--accent-secondary);
  border-style: solid;
  color: var(--bg-primary);
  transform: scale(1.1);
}

.create-content h3 {
  font-family: var(--font-display);
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.create-content p {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin: 0;
}

.workflow-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
}

.editor-info {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.info-text h2 {
  font-family: var(--font-display);
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: var(--text-primary);
}

.info-text p {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 0;
}

.editor-actions {
  display: flex;
  gap: var(--space-3);
}

.editor-toolbar {
  padding: var(--space-3) var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
}

.toolbar-info {
  display: flex;
  gap: var(--space-6);
}

.info-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
}

.editor-content {
  flex: 1;
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  position: relative;
}

.log-panel {
  position: absolute;
  right: var(--space-4);
  top: var(--space-4);
  bottom: var(--space-4);
  width: 400px;
  background: var(--bg-primary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-xl);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.log-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.log-panel-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.log-panel-content {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-4);
  background: #1f2937;
}

.log-item {
  display: flex;
  gap: 8px;
  padding: 8px 0;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.log-item:last-child {
  border-bottom: none;
}

.log-time {
  color: #9ca3af;
  flex-shrink: 0;
}

.log-level {
  font-weight: 600;
  flex-shrink: 0;
}

.log-level.info { color: #60a5fa; }
.log-level.warning { color: #fbbf24; }
.log-level.error { color: #f87171; }
.log-level.success { color: #34d399; }

.log-node {
  color: #a78bfa;
  flex-shrink: 0;
}

.log-message {
  color: #f3f4f6;
  flex: 1;
}

.log-empty {
  text-align: center;
  color: #6b7280;
  padding: var(--space-8) 0;
  font-family: var(--font-mono);
  font-size: 0.85rem;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.form-tip {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-top: 8px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
}
</style>
