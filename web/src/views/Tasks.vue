<template>
  <div class="tasks-page">
    <div class="action-bar">
      <el-button :icon="Plus" type="primary" size="default" @click="handleCreate" class="create-btn">
        <span>创建任务</span>
      </el-button>
    </div>

    <div class="tasks-content">
      <div class="tasks-table-section" :class="{ 'with-logs': showLogs }">
        <div class="table-wrapper">
          <el-table 
            :data="tasks" 
            stripe 
            v-loading="loading" 
            class="tasks-table"
            :header-cell-style="{ background: '#f8fafc', color: '#475569', fontWeight: 600 }"
            :cell-style="{ padding: '12px 16px' }"
          >
            <el-table-column prop="id" label="ID" width="70" align="center" />
            <el-table-column prop="name" label="任务名称" min-width="200">
              <template #default="{ row }">
                <div class="task-name-cell">
                  <div class="type-icon-wrapper" :style="{ background: getTypeBg(row.type) }">
                    <el-icon :size="18" :color="getTypeColor(row.type)">
                      <component :is="getTypeIcon(row.type)" />
                    </el-icon>
                  </div>
                  <div class="task-info">
                    <div class="task-name">{{ row.name }}</div>
                    <div class="task-id">ID: {{ row.id }}</div>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="type" label="类型" width="120" align="center">
              <template #default="{ row }">
                <div class="type-badge" :class="row.type">
                  <el-icon :size="14">
                    <component :is="getTypeIcon(row.type)" />
                  </el-icon>
                  <span>{{ getTypeLabel(row.type) }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="cron_expression" label="Cron表达式" min-width="180">
              <template #default="{ row }">
                <div class="cron-cell">
                  <code class="cron-code" v-if="row.cron_expression">{{ row.cron_expression }}</code>
                  <span v-else class="text-muted">
                    <el-icon><Clock /></el-icon>
                    <span>手动触发</span>
                  </span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="110" align="center">
              <template #default="{ row }">
                <div class="status-wrapper">
                  <span class="status-dot" :class="getStatusDot(row.status)"></span>
                  <el-tag :type="getStatusTag(row.status)" size="default" effect="light" class="status-tag">
                    {{ getStatusText(row.status) }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="280" fixed="right" align="center">
              <template #default="{ row }">
                <div class="action-btns">
                  <el-button
                    :icon="VideoPlay"
                    size="small"
                    type="primary"
                    @click="handleTrigger(row)"
                    :loading="triggeringId === row.id"
                    class="run-btn"
                  >
                    运行
                  </el-button>
                  <div class="icon-btns">
                    <el-button 
                      :icon="View" 
                      size="small" 
                      circle
                      @click="handleViewLogs(row)" 
                      class="icon-btn log-btn"
                    />
                    <el-button 
                      :icon="Edit" 
                      size="small" 
                      circle
                      @click="handleEdit(row)" 
                      class="icon-btn edit-btn"
                    />
                    <el-button 
                      :icon="Delete" 
                      size="small" 
                      circle
                      type="danger" 
                      @click="handleDelete(row)" 
                      class="icon-btn delete-btn"
                    />
                  </div>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
        
        <div class="empty-state" v-if="!loading && tasks.length === 0">
          <el-empty description="暂无任务，点击右上角创建">
            <el-button :icon="Plus" type="primary" @click="handleCreate">创建任务</el-button>
          </el-empty>
        </div>
      </div>

      <div class="log-section" v-if="showLogs">
        <TaskLogViewer
          :execution-id="activeExecutionId"
          :execution-status="activeExecutionStatus"
          @close="showLogs = false"
        />
      </div>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑任务' : '创建任务'"
      width="720px"
      :close-on-click-modal="false"
      destroy-on-close
      class="task-dialog"
    >
      <el-form :model="taskForm" label-position="top" class="task-form">
        <div class="form-row">
          <el-form-item label="任务名称" required class="form-item-full">
            <el-input v-model="taskForm.name" placeholder="输入任务名称" size="large" />
          </el-form-item>
        </div>

        <div class="form-row">
          <el-form-item label="任务类型" class="form-item-half">
            <el-select v-model="taskForm.type" @change="handleTypeChange" style="width:100%" size="large">
              <el-option label="HTTP 请求" value="http" />
              <el-option label="Shell 脚本" value="shell" />
            </el-select>
          </el-form-item>
          <el-form-item label="超时(秒)" class="form-item-half">
            <el-input-number v-model="taskForm.timeout_seconds" :min="1" :max="3600" style="width:100%" size="large" />
          </el-form-item>
        </div>

        <div class="form-row">
          <el-form-item label="调度配置 (Cron表达式)" class="form-item-full">
            <CronEditor v-model="taskForm.cron_expression" />
          </el-form-item>
        </div>

        <div class="form-row">
          <el-form-item label="任务配置" class="form-item-full">
            <div class="config-card">
              <template v-if="taskForm.type === 'http'">
                <el-input v-model="httpConfig.url" placeholder="请求URL" class="config-input" size="large" />
                <div class="config-row">
                  <el-select v-model="httpConfig.method" style="width:140px" size="large">
                    <el-option label="GET" value="GET" />
                    <el-option label="POST" value="POST" />
                    <el-option label="PUT" value="PUT" />
                    <el-option label="DELETE" value="DELETE" />
                  </el-select>
                  <el-input
                    v-model="httpConfig.body"
                    type="textarea"
                    :rows="4"
                    placeholder="请求体 (JSON)"
                    class="config-textarea"
                    size="large"
                  />
                </div>
              </template>
              <template v-else>
                <el-input
                  v-model="shellConfig.script"
                  type="textarea"
                  :rows="8"
                  placeholder="输入 Shell 脚本"
                  class="config-textarea"
                  size="large"
                />
              </template>
            </div>
          </el-form-item>
        </div>

        <div class="form-row">
          <el-form-item label="重试次数" class="form-item-third">
            <el-input-number v-model="taskForm.retry_count" :min="0" :max="10" style="width:100%" size="large" />
          </el-form-item>
          <el-form-item label="重试间隔(秒)" class="form-item-third">
            <el-input-number v-model="taskForm.retry_interval" :min="1" :max="300" style="width:100%" size="large" />
          </el-form-item>
          <el-form-item label="启用" class="form-item-third">
            <div class="switch-wrapper">
              <el-switch v-model="taskForm.is_enabled" size="large" />
              <span class="switch-text">{{ taskForm.is_enabled ? '已启用' : '已禁用' }}</span>
            </div>
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting" size="large">
            {{ isEdit ? '保存' : '创建' }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { Plus, VideoPlay, Edit, Delete, View, Link, Monitor, Clock } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { taskAPI } from '@/api'
import CronEditor from '@/components/CronEditor.vue'
import TaskLogViewer from '@/components/TaskLogViewer.vue'
import type { Task } from '@/types'

const tasks = ref<Task[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const submitting = ref(false)
const triggeringId = ref<number | null>(null)
const showLogs = ref(false)
const activeExecutionId = ref('')
const activeExecutionStatus = ref('')

const taskForm = reactive<Record<string, any>>({
  name: '',
  type: 'http',
  config: '',
  cron_expression: '',
  timeout_seconds: 300,
  retry_count: 3,
  retry_interval: 5,
  is_enabled: true,
  domain_id: 1,
})

const httpConfig = reactive({ url: '', method: 'GET', body: '' })
const shellConfig = reactive({ script: '' })

const getTypeIcon = (type: string) => type === 'http' ? Link : Monitor
const getTypeColor = (type: string) => type === 'http' ? '#2563eb' : '#d97706'
const getTypeBg = (type: string) => type === 'http' ? '#eff6ff' : '#fffbeb'
const getTypeTag = (type: string) => type === 'http' ? 'primary' : 'warning'
const getTypeLabel = (type: string) => type === 'http' ? 'HTTP' : 'Shell'

const getStatusTag = (status: string) => {
  const map: Record<string, string> = { pending: 'info', running: 'warning', success: 'success', failed: 'danger' }
  return map[status] || 'info'
}

const getStatusDot = (status: string) => {
  const map: Record<string, string> = { pending: 'dot-info', running: 'dot-warning', success: 'dot-success', failed: 'dot-danger' }
  return map[status] || 'dot-info'
}

const getStatusText = (status: string) => {
  const map: Record<string, string> = { pending: '待执行', running: '运行中', success: '成功', failed: '失败' }
  return map[status] || status
}

const loadTasks = async () => {
  loading.value = true
  try {
    const res = await taskAPI.list()
    tasks.value = res.data || []
  } catch (err) {
    console.error('加载任务失败', err)
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  isEdit.value = false
  delete taskForm.id
  delete taskForm.workflow_id
  Object.assign(taskForm, {
    name: '',
    type: 'http',
    config: '',
    cron_expression: '',
    timeout_seconds: 300,
    retry_count: 3,
    retry_interval: 5,
    is_enabled: true,
    domain_id: 1,
  })
  httpConfig.url = ''
  httpConfig.method = 'GET'
  httpConfig.body = ''
  shellConfig.script = ''
  dialogVisible.value = true
}

const handleEdit = (row: Task) => {
  isEdit.value = true
  Object.assign(taskForm, row)
  try {
    const config = JSON.parse(row.config || '{}')
    if (row.type === 'http') {
      httpConfig.url = config.url || ''
      httpConfig.method = config.method || 'GET'
      httpConfig.body = config.body || ''
    } else {
      shellConfig.script = config.script || ''
    }
  } catch {
    // ignore
  }
  dialogVisible.value = true
}

const handleTypeChange = () => {
  // reset config when type changes
}

const buildConfig = (): string => {
  if (taskForm.type === 'http') {
    return JSON.stringify({
      url: httpConfig.url,
      method: httpConfig.method,
      body: httpConfig.body,
    })
  }
  return JSON.stringify({
    script: shellConfig.script,
  })
}

const handleSubmit = async () => {
  submitting.value = true
  try {
    const payload = { ...taskForm, config: buildConfig() }
    if (isEdit.value) {
      await taskAPI.update(taskForm.id, payload)
      ElMessage.success('任务更新成功')
    } else {
      await taskAPI.create(payload)
      ElMessage.success('任务创建成功')
    }
    dialogVisible.value = false
    await loadTasks()
  } catch (err) {
    console.error('保存任务失败', err)
    ElMessage.error('保存任务失败')
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row: Task) => {
  try {
    await ElMessageBox.confirm(`确定删除任务 "${row.name}" 吗？`, '确认删除', { type: 'warning' })
    await taskAPI.delete(row.id)
    ElMessage.success('删除成功')
    await loadTasks()
  } catch {
    // cancelled
  }
}

const handleTrigger = async (row: Task) => {
  triggeringId.value = row.id
  try {
    const res = await taskAPI.trigger(row.id)
    const data = res.data as any
    activeExecutionId.value = data.execution_id || ''
    activeExecutionStatus.value = 'running'
    showLogs.value = true
    ElMessage.success('任务已触发')
    await loadTasks()
  } catch (err) {
    console.error('触发任务失败', err)
    ElMessage.error('触发任务失败')
  } finally {
    triggeringId.value = null
  }
}

const handleViewLogs = (row: Task) => {
  if (!row.id) return
  taskAPI.getExecutions(row.id).then((res: any) => {
    const execs = res.data || []
    if (execs.length > 0) {
      activeExecutionId.value = execs[0].execution_id || ''
      activeExecutionStatus.value = execs[0].status || ''
    } else {
      activeExecutionId.value = ''
      activeExecutionStatus.value = ''
    }
    showLogs.value = true
  }).catch(() => {
    showLogs.value = true
  })
}

onMounted(() => {
  loadTasks()
})
</script>

<style scoped>
.tasks-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: 100%;
}

.action-bar {
  display: flex;
  justify-content: flex-end;
  align-items: center;
}

.create-btn {
  font-weight: 600;
  border-radius: 8px;
  padding: 10px 20px;
  box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1);
  transition: all 0.2s;
}

.create-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.1);
}

.tasks-content {
  display: flex;
  gap: 20px;
  flex: 1;
  min-height: 0;
}

.tasks-table-section {
  flex: 1;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px 0 rgb(0 0 0 / 0.1);
  overflow: hidden;
  transition: all 0.3s;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.tasks-table-section.with-logs {
  flex: 0 0 55%;
}

.table-wrapper {
  flex: 1;
  overflow: auto;
}

.tasks-table {
  border: none;
}

.tasks-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.task-name-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.type-icon-wrapper {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.task-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.task-info .task-name {
  font-weight: 600;
  color: #1e293b;
  font-size: 0.9375rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-info .task-id {
  font-size: 0.75rem;
  color: #94a3b8;
}

.type-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 0.8125rem;
  font-weight: 600;
  transition: all 0.2s ease;
}

.type-badge.http {
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
  color: #1d4ed8;
}

.type-badge.http:hover {
  background: linear-gradient(135deg, #dbeafe, #bfdbfe);
}

.type-badge.shell {
  background: linear-gradient(135deg, #fffbeb, #fef3c7);
  color: #b45309;
}

.type-badge.shell:hover {
  background: linear-gradient(135deg, #fef3c7, #fde68a);
}

.cron-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cron-code {
  font-family: 'JetBrains Mono', 'Fira Code', ui-monospace, monospace;
  font-size: 0.8125rem;
  background: linear-gradient(135deg, #f1f5f9, #e2e8f0);
  padding: 4px 10px;
  border-radius: 6px;
  color: #1e293b;
  border: 1px solid #cbd5e1;
  white-space: nowrap;
}

.text-muted {
  display: flex;
  align-items: center;
  gap: 4px;
  color: #94a3b8;
  font-size: 0.875rem;
  white-space: nowrap;
}

.status-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  flex-shrink: 0;
}

.status-dot.dot-info {
  background: #60a5fa;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.2);
}

.status-dot.dot-warning {
  background: #f59e0b;
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.2);
  animation: pulse 2s infinite;
}

.status-dot.dot-success {
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.2);
}

.status-dot.dot-danger {
  background: #ef4444;
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.2);
}

@keyframes pulse {
  0%, 100% {
    opacity: 1;
  }
  50% {
    opacity: 0.5;
  }
}

.status-tag {
  font-weight: 500;
}

.action-btns {
  display: flex;
  gap: 8px;
  flex-wrap: nowrap;
  justify-content: center;
  align-items: center;
}

.run-btn {
  border-radius: 8px;
  font-weight: 600;
  padding: 8px 16px;
  background: linear-gradient(135deg, #3b82f6, #1d4ed8);
  border: none;
  color: white;
  transition: all 0.2s ease;
  box-shadow: 0 2px 4px -1px rgba(59, 130, 246, 0.4);
}

.run-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px -2px rgba(59, 130, 246, 0.5);
  background: linear-gradient(135deg, #2563eb, #1e40af);
}

.run-btn:active {
  transform: translateY(0);
}

.icon-btns {
  display: flex;
  gap: 4px;
}

.icon-btn {
  width: 32px;
  height: 32px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
}

.icon-btn:hover {
  transform: scale(1.1);
}

.log-btn {
  color: #6366f1;
  border-color: #c7d2fe;
  background: #eef2ff;
}

.log-btn:hover {
  background: #e0e7ff;
  color: #4f46e5;
  border-color: #a5b4fc;
}

.edit-btn {
  color: #f59e0b;
  border-color: #fde68a;
  background: #fef3c7;
}

.edit-btn:hover {
  background: #fde68a;
  color: #d97706;
  border-color: #fcd34d;
}

.delete-btn {
  color: #ef4444;
  border-color: #fecaca;
  background: #fee2e2;
}

.delete-btn:hover {
  background: #fecaca;
  color: #dc2626;
  border-color: #fca5a5;
}

.empty-state {
  padding: 60px 20px;
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.log-section {
  flex: 1;
  min-width: 400px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px 0 rgb(0 0 0 / 0.1);
  overflow: hidden;
}

.task-dialog :deep(.el-dialog__header) {
  border-bottom: 1px solid #e2e8f0;
  padding: 20px 24px;
  margin: 0;
}

.task-dialog :deep(.el-dialog__title) {
  font-size: 1.25rem;
  font-weight: 700;
  color: #1e293b;
}

.task-dialog :deep(.el-dialog__body) {
  padding: 24px;
}

.task-dialog :deep(.el-dialog__footer) {
  padding: 16px 24px;
  border-top: 1px solid #e2e8f0;
}

.task-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.form-item-full {
  flex: 1;
  margin-bottom: 0 !important;
}

.form-item-half {
  flex: 1;
  margin-bottom: 0 !important;
}

.form-item-third {
  flex: 1;
  margin-bottom: 0 !important;
}

.task-form :deep(.el-form-item__label) {
  font-weight: 600;
  color: #475569;
  margin-bottom: 6px;
  line-height: 1.4;
}

.task-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.config-card {
  background: linear-gradient(135deg, #f8fafc, #f1f5f9);
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 20px;
}

.config-row {
  display: flex;
  gap: 12px;
  margin-top: 12px;
}

.config-input {
  margin-top: 0;
}

.config-textarea {
  margin-top: 0;
}

.switch-wrapper {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 40px;
}

.switch-text {
  font-size: 0.9375rem;
  color: #475569;
  font-weight: 500;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* Responsive */
@media (max-width: 1400px) {
  .tasks-table-section.with-logs {
    flex: 0 0 50%;
  }
  
  .log-section {
    min-width: 350px;
  }
}

@media (max-width: 1200px) {
  .tasks-content {
    flex-direction: column;
  }
  
  .tasks-table-section.with-logs {
    flex: 1;
  }
  
  .log-section {
    min-width: 100%;
    height: 400px;
  }
}

@media (max-width: 768px) {
  .action-bar {
    justify-content: stretch;
  }
  
  .create-btn {
    width: 100%;
  }
  
  .tasks-content {
    gap: 12px;
  }
  
  .cron-code {
    font-size: 0.7rem;
    padding: 3px 6px;
  }
  
  .type-badge {
    padding: 5px 10px;
    font-size: 0.75rem;
  }
  
  .run-btn {
    padding: 6px 12px;
    font-size: 0.8125rem;
  }
  
  .icon-btn {
    width: 28px;
    height: 28px;
  }
}
</style>
