<template>
  <div class="executors-page">
    <!-- Stats Cards -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon stat-icon-primary">
          <el-icon :size="24"><List /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ total }}</div>
          <div class="stat-label">总执行器</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-success">
          <el-icon :size="24"><CircleCheck /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ onlineCount }}</div>
          <div class="stat-label">在线执行器</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-danger">
          <el-icon :size="24"><CircleClose /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ offlineCount }}</div>
          <div class="stat-label">离线执行器</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-warning">
          <el-icon :size="24"><DataLine /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ totalCapacity }}</div>
          <div class="stat-label">总容量</div>
        </div>
      </div>
    </div>

    <!-- 工具栏 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="filters.executor_id"
          placeholder="执行器ID"
          clearable
          class="filter-input"
          @keyup.enter="loadExecutors"
        />
        <el-input
          v-model="filters.name"
          placeholder="执行器名称"
          clearable
          class="filter-input"
          @keyup.enter="loadExecutors"
        />
        <el-select
          v-model="filters.status"
          placeholder="状态"
          clearable
          class="filter-select"
          @change="loadExecutors"
        >
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadExecutors" :loading="loading" class="refresh-btn">刷新</el-button>
      </div>
    </div>

    <!-- 表格 -->
    <div class="table-wrapper">
      <el-table
        :data="filteredExecutors"
        stripe
        style="width: 100%"
        v-loading="loading"
        height="100%"
      >
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="executor_id" label="执行器ID" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="name" label="名称" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="address" label="地址" :minWidth="180" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="120" align="center">
          <template #default="{ row }">
            <span class="status-dot" :class="getStatusDotClass(row.status)"></span>
            <el-tag :type="getStatusType(row.status)" size="small" effect="light">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="capacity" label="容量" width="140" align="center">
          <template #default="{ row }">
            <el-input-number
              v-if="editingRow === row.id"
              v-model="tempCapacity"
              :min="1"
              size="small"
              controls-position="right"
              @blur="handleSaveCapacity(row)"
              @keyup.enter="handleSaveCapacity(row)"
              ref="capacityInputRef"
            />
            <div v-else class="capacity-display" @click="handleEditCapacity(row)">
              <span class="capacity-value">{{ row.capacity }}</span>
              <el-icon class="edit-icon"><Edit /></el-icon>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="current_load" label="当前负载" width="110" align="center" />
        <el-table-column label="最后心跳" width="180">
          <template #default="{ row }">
            {{ formatTime(row.last_heartbeat) }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right" align="center">
          <template #default="{ row }">
            <el-button
              type="success"
              size="small"
              circle
              :disabled="row.status === 'online'"
              @click="handleChangeStatus(row, 'online')"
              class="action-btn online-btn"
              title="上线"
            >
              <el-icon><CircleCheck /></el-icon>
            </el-button>
            <el-button
              type="warning"
              size="small"
              circle
              :disabled="row.status === 'offline'"
              @click="handleChangeStatus(row, 'offline')"
              class="action-btn offline-btn"
              title="下线"
            >
              <el-icon><SwitchButton /></el-icon>
            </el-button>
            <el-button
              type="danger"
              size="small"
              circle
              @click="handleDelete(row)"
              class="action-btn delete-btn"
              title="删除"
            >
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无执行器</p>
          </div>
        </template>
      </el-table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Delete, Document, List, CircleCheck, CircleClose, DataLine, SwitchButton, Edit } from '@element-plus/icons-vue'
import { executorAPI } from '@/api'
import type { Executor } from '@/types'

const executors = ref<Executor[]>([])
const loading = ref(false)
const editingRow = ref<number | null>(null)
const tempCapacity = ref<number>(1)
const capacityInputRef = ref()

const filters = ref({
  executor_id: '',
  name: '',
  status: ''
})

// 统计计算属性
const total = computed(() => executors.value.length)
const onlineCount = computed(() => executors.value.filter(e => e.status === 'online').length)
const offlineCount = computed(() => executors.value.filter(e => e.status === 'offline').length)
const totalCapacity = computed(() => executors.value.reduce((sum, e) => sum + (e.capacity || 0), 0))

// 筛选后的执行器列表
const filteredExecutors = computed(() => {
  return executors.value.filter(ex => {
    if (filters.value.executor_id && !ex.executor_id?.toLowerCase().includes(filters.value.executor_id.toLowerCase())) {
      return false
    }
    if (filters.value.name && !ex.name?.toLowerCase().includes(filters.value.name.toLowerCase())) {
      return false
    }
    if (filters.value.status && ex.status !== filters.value.status) {
      return false
    }
    return true
  })
})

const getStatusType = (status: string) => {
  switch (status) {
    case 'online': return 'success'
    case 'offline': return 'danger'
    default: return 'info'
  }
}

const getStatusText = (status: string) => {
  switch (status) {
    case 'online': return '在线'
    case 'offline': return '离线'
    default: return status
  }
}

const getStatusDotClass = (status: string) => {
  switch (status) {
    case 'online': return 'dot-success'
    case 'offline': return 'dot-danger'
    default: return 'dot-info'
  }
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

const loadExecutors = async () => {
  loading.value = true
  try {
    const response = await executorAPI.list()
    executors.value = response.data || []
  } catch (error) {
    console.error('Failed to load executors:', error)
    ElMessage.error('加载执行器失败')
  } finally {
    loading.value = false
  }
}

const handleChangeStatus = async (row: Executor, status: string) => {
  try {
    await ElMessageBox.confirm(
      `确定要将执行器 "${row.executor_id}" 设置为${status === 'online' ? '上线' : '离线'}状态吗？`,
      '状态变更确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    if (status === 'online') {
      await executorAPI.online(row.executor_id)
      ElMessage.success('上线成功')
    } else {
      await executorAPI.offline(row.executor_id)
      ElMessage.success('离线成功')
    }
    loadExecutors()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('状态变更失败')
    }
  }
}

const handleDelete = async (row: Executor) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除执行器 "${row.executor_id}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    await executorAPI.delete(row.executor_id)
    ElMessage.success('删除成功')
    loadExecutors()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const handleEditCapacity = (row: Executor) => {
  editingRow.value = row.id
  tempCapacity.value = row.capacity || 1
  // 等待 DOM 更新后聚焦输入框
  setTimeout(() => {
    capacityInputRef.value?.focus()
  }, 50)
}

const handleSaveCapacity = async (row: Executor) => {
  if (editingRow.value === null) return
  
  const newCapacity = tempCapacity.value
  if (newCapacity === row.capacity) {
    editingRow.value = null
    return
  }
  
  try {
    await executorAPI.updateCapacity(row.executor_id, newCapacity)
    row.capacity = newCapacity
    ElMessage.success('容量更新成功')
  } catch (error) {
    console.error('Failed to update capacity:', error)
    ElMessage.error('容量更新失败')
  } finally {
    editingRow.value = null
  }
}

onMounted(() => {
  loadExecutors()
})
</script>

<style scoped>
.executors-page {
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
  width: 180px;
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
  margin: 0 4px;
}

.action-btn:hover {
  opacity: 1;
  transform: scale(1.1);
}

.action-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
  transform: none;
}

/* Capacity Display */
.capacity-display {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 4px 8px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-out);
}

.capacity-display:hover {
  background: var(--bg-secondary);
}

.capacity-display:hover .edit-icon {
  opacity: 1;
}

.capacity-value {
  font-weight: 500;
  color: var(--text-primary);
}

.edit-icon {
  opacity: 0;
  font-size: 14px;
  color: var(--accent-primary);
  transition: opacity var(--duration-fast) var(--ease-out);
}

/* Responsive */
@media (max-width: 768px) {
  .executors-page {
    gap: var(--space-3);
  }

  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 480px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
