<template>
  <div class="workflows-page">
    <div class="page-header">
      <div class="header-left">
        <h1>工作流设计器</h1>
        <p>创建、编辑和管理您的分布式工作流</p>
      </div>
      <div class="header-right">
        <el-button :icon="Plus" type="primary" @click="showCreateDialog = true">
          创建工作流
        </el-button>
      </div>
    </div>

    <div class="workflows-grid" v-if="!selectedWorkflow">
      <div
        v-for="workflow in workflows"
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
            <el-button :icon="Edit" text size="small" @click.stop="editWorkflow(workflow)">
              编辑
            </el-button>
            <el-button :icon="Delete" text size="small" type="danger" @click.stop="deleteWorkflowConfirm(workflow)">
              删除
            </el-button>
          </div>
        </div>
      </div>

      <div class="workflow-card create-card" @click="showCreateDialog = true">
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
import { ref, reactive, onMounted, onUnmounted } from 'vue'
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
  Close
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import FlowCanvas from '@/components/FlowCanvas.vue'
import { workflowAPI } from '@/api'
import type { Workflow, WorkflowDAG, WorkflowExecution, TaskLog } from '@/types'

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
  workflowForm.domain_id = 1
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
  height: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid #e5e7eb;
}

.header-left h1 {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 1.75rem;
  font-weight: 700;
  margin: 0 0 4px 0;
  background: linear-gradient(135deg, #1f2937, #06b6d4);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.header-left p {
  color: #6b7280;
  margin: 0;
}

.workflows-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 20px;
}

.workflow-card {
  background: white;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 20px;
  cursor: pointer;
  transition: all 0.3s ease-out;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.workflow-card:hover {
  border-color: #06b6d4;
  transform: translateY(-4px);
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04), 0 0 0 1px rgba(34, 211, 238, 0.1);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.card-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, rgba(34, 211, 238, 0.15), rgba(34, 211, 238, 0.05));
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #06b6d4;
  font-size: 1.5rem;
}

.badge {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.7rem;
  font-weight: 500;
  padding: 4px 10px;
  border-radius: 9999px;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.badge.enabled {
  background: rgba(52, 211, 153, 0.15);
  color: #10b981;
  border: 1px solid rgba(52, 211, 153, 0.2);
}

.badge.disabled {
  background: rgba(148, 163, 184, 0.15);
  color: #9ca3af;
  border: 1px solid rgba(148, 163, 184, 0.2);
}

.card-body h3 {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 1.1rem;
  font-weight: 600;
  margin: 0 0 8px 0;
  color: #1f2937;
}

.card-body p {
  font-size: 0.85rem;
  color: #6b7280;
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
  padding-top: 16px;
  border-top: 1px solid #e5e7eb;
}

.card-stats {
  display: flex;
  gap: 16px;
}

.stat {
  display: flex;
  align-items: center;
  gap: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: #9ca3af;
}

.card-actions {
  display: flex;
  gap: 8px;
  opacity: 0;
  transition: opacity 0.2s;
}

.workflow-card:hover .card-actions {
  opacity: 1;
}

.create-card {
  border-style: dashed;
  border-color: #e5e7eb;
  background: transparent;
  min-height: 280px;
}

.create-card:hover {
  border-color: #06b6d4;
  background: rgba(34, 211, 238, 0.02);
}

.create-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  height: 100%;
  gap: 12px;
}

.create-icon {
  width: 64px;
  height: 64px;
  background: rgba(34, 211, 238, 0.1);
  border: 2px dashed #06b6d4;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #06b6d4;
  font-size: 1.5rem;
  margin-bottom: 8px;
  transition: all 0.3s ease;
}

.create-card:hover .create-icon {
  background: #06b6d4;
  border-style: solid;
  color: #111827;
  transform: scale(1.1);
}

.create-content h3 {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 1rem;
  font-weight: 600;
  margin: 0;
  color: #1f2937;
}

.create-content p {
  font-size: 0.85rem;
  color: #9ca3af;
  margin: 0;
}

.workflow-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin: -24px;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: white;
  border-bottom: 1px solid #e5e7eb;
}

.editor-info {
  display: flex;
  align-items: center;
  gap: 16px;
}

.info-text h2 {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0 0 4px 0;
}

.info-text p {
  font-size: 0.85rem;
  color: #6b7280;
  margin: 0;
}

.editor-actions {
  display: flex;
  gap: 12px;
}

.editor-toolbar {
  padding: 12px 24px;
  background: #f9fafb;
  border-bottom: 1px solid #e5e7eb;
}

.toolbar-info {
  display: flex;
  gap: 24px;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: #9ca3af;
}

.editor-content {
  flex: 1;
  padding: 24px;
  background: #f9fafb;
  position: relative;
}

.log-panel {
  position: absolute;
  right: 24px;
  top: 24px;
  bottom: 24px;
  width: 400px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.12);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.log-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  border-bottom: 1px solid #e5e7eb;
}

.log-panel-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: #1f2937;
}

.log-panel-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  background: #1f2937;
}

.log-item {
  display: flex;
  gap: 8px;
  padding: 8px 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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
  padding: 32px 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.85rem;
}

.create-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-tip {
  font-size: 0.75rem;
  color: #9ca3af;
  margin-top: 8px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
