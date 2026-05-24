<template>
  <div class="audit-logs-page">
    <!-- Page Toolbar -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索操作人..."
          :prefix-icon="Search"
          class="search-input"
          clearable
          @clear="handleSearch"
          @keyup.enter="handleSearch"
        />
        <el-select v-model="filterForm.action" placeholder="操作类型" clearable class="filter-select" @change="handleSearch">
          <el-option v-for="item in actionOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-select v-model="filterForm.resource" placeholder="资源类型" clearable class="filter-select" @change="handleSearch">
          <el-option v-for="item in resourceOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-select v-model="filterForm.status" placeholder="状态" clearable class="filter-select" @change="handleSearch">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="failure" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          format="YYYY-MM-DD HH:mm:ss"
          value-format="YYYY-MM-DD HH:mm:ss"
          class="date-picker"
          @change="handleDateChange"
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="handleSearch" :loading="loading">刷新</el-button>
        <el-button :icon="Delete" @click="handleCleanExpired" type="warning">
          清理过期
        </el-button>
      </div>
    </div>

    <!-- Table -->
    <div class="table-wrapper">
      <el-table :data="logs" v-loading="loading" stripe height="100%">
        <el-table-column prop="created_at" label="时间" width="180" sortable>
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="username" label="操作人" width="150">
          <template #default="{ row }">
            <div v-if="row.real_name" class="user-info">
              <div class="user-name">{{ row.real_name }}</div>
              <div class="user-account">{{ row.username }}</div>
            </div>
            <div v-else>{{ row.username }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="role" label="角色" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="getRoleTagType(row.role)" size="small" effect="light">
              {{ getRoleLabel(row.role) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="action" label="操作类型" width="120">
          <template #default="{ row }">
            <el-tag :type="getActionTagType(row.action)" size="small" effect="light">
              {{ getActionLabel(row.action) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="resource" label="资源类型" width="110">
          <template #default="{ row }">
            {{ getResourceLabel(row.resource) }}
          </template>
        </el-table-column>
        <el-table-column prop="resource_name" label="资源名称" min-width="150" show-overflow-tooltip />
        <el-table-column prop="status" label="结果" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small" effect="light">
              {{ row.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip_address" label="IP" width="130" />
        <el-table-column prop="request_method" label="方法" width="70" align="center">
          <template #default="{ row }">
            <span class="method-badge" :class="row.request_method?.toLowerCase()">{{ row.request_method }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="request_path" label="路径" min-width="180" show-overflow-tooltip />
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无审计日志</p>
          </div>
        </template>
      </el-table>
      
      <div class="pagination-container">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Search, Refresh, Document } from '@element-plus/icons-vue'
import { auditLogAPI } from '@/api'
import type { AuditLog } from '@/api/audit'

const logs = ref<AuditLog[]>([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const dateRange = ref<[string, string] | null>(null)
const searchQuery = ref('')

const filterForm = ref({
  username: '',
  action: '',
  resource: '',
  status: '',
})

const actionOptions = [
  { value: 'login', label: '登录' },
  { value: 'register', label: '注册' },
  { value: 'create', label: '创建' },
  { value: 'update', label: '更新' },
  { value: 'delete', label: '删除' },
  { value: 'trigger', label: '触发' },
  { value: 'assign', label: '分配' },
  { value: 'revoke', label: '撤销' },
  { value: 'reset_password', label: '重置密码' },
  { value: 'change_password', label: '修改密码' },
  { value: 'test_connection', label: '测试连接' },
  { value: 'config_change', label: '配置变更' },
  { value: 'execute', label: '执行查询' },
  { value: 'export', label: '导出' },
  { value: 'clean', label: '清理' },
  { value: 'pause', label: '暂停' },
  { value: 'resume', label: '恢复' },
  { value: 'online', label: '上线' },
  { value: 'offline', label: '下线' },
]

const resourceOptions = [
  { value: 'auth', label: '认证' },
  { value: 'user', label: '用户' },
  { value: 'role', label: '角色' },
  { value: 'domain', label: '领域' },
  { value: 'datasource', label: '数据源' },
  { value: 'task', label: '任务' },
  { value: 'workflow', label: '工作流' },
  { value: 'executor', label: '执行器' },
  { value: 'config', label: '系统配置' },
  { value: 'query', label: '查询' },
  { value: 'saved_sql', label: '保存SQL' },
  { value: 'query_history', label: '查询历史' },
  { value: 'log', label: '日志' },
]

const actionLabelMap: Record<string, string> = Object.fromEntries(actionOptions.map(o => [o.value, o.label]))
const resourceLabelMap: Record<string, string> = Object.fromEntries(resourceOptions.map(o => [o.value, o.label]))

const getActionLabel = (action: string) => actionLabelMap[action] || action
const getResourceLabel = (resource: string) => resourceLabelMap[resource] || resource

const getRoleLabel = (role: string) => {
  const map: Record<string, string> = {
    system_admin: '系统管理员',
    domain_admin: '领域管理员',
    user: '普通用户',
    admin: '管理员',
  }
  return map[role] || role || '-'
}

const getRoleTagType = (role: string) => {
  const map: Record<string, string> = {
    system_admin: 'danger',
    domain_admin: 'warning',
    user: 'info',
    admin: 'danger',
  }
  return map[role] || 'info'
}

const getActionTagType = (action: string) => {
  if (['create', 'login', 'register', 'online', 'resume'].includes(action)) return 'success'
  if (['update', 'assign', 'change_password', 'config_change', 'trigger', 'test_connection', 'execute', 'export'].includes(action)) return 'warning'
  if (['delete', 'revoke', 'reset_password', 'offline', 'pause', 'clean'].includes(action)) return 'danger'
  return 'info'
}

const formatTime = (t: string) => {
  if (!t) return '-'
  try {
    const date = new Date(t)
    if (isNaN(date.getTime())) {
      return t.replace('T', ' ').substring(0, 19)
    }
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    const seconds = String(date.getSeconds()).padStart(2, '0')
    return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  } catch {
    return t.replace('T', ' ').substring(0, 19)
  }
}

const handleDateChange = () => {
  handleSearch()
}

const handleSearch = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value,
    }
    // 将 searchQuery 赋值给 username
    if (searchQuery.value) params.username = searchQuery.value
    if (filterForm.value.action) params.action = filterForm.value.action
    if (filterForm.value.resource) params.resource = filterForm.value.resource
    if (filterForm.value.status) params.status = filterForm.value.status
    if (dateRange.value && dateRange.value[0]) {
      params.start_time = dateRange.value[0]
      params.end_time = dateRange.value[1]
    }

    const response = await auditLogAPI.list(params)
    const data = response.data
    logs.value = data?.items || []
    total.value = data?.total || 0
  } catch (error: any) {
    ElMessage.error(error.message || '加载审计日志失败')
  } finally {
    loading.value = false
  }
}

const handleReset = () => {
  filterForm.value = { username: '', action: '', resource: '', status: '' }
  searchQuery.value = ''
  dateRange.value = null
  currentPage.value = 1
  handleSearch()
}

const handleCleanExpired = async () => {
  try {
    await ElMessageBox.confirm(
      '确认清理过期的审计日志？系统将根据保留天数配置自动清理过期记录。',
      '清理确认',
      { confirmButtonText: '确认清理', cancelButtonText: '取消', type: 'warning' }
    )
    const response = await auditLogAPI.cleanExpired()
    const data = response.data
    ElMessage.success(`清理完成，共删除 ${data?.deleted_count || 0} 条过期日志`)
    handleSearch()
  } catch {
    // cancelled
  }
}

onMounted(() => {
  handleSearch()
})
</script>

<style scoped>
.audit-logs-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.audit-logs-page::-webkit-scrollbar {
  width: 8px;
}

.audit-logs-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.audit-logs-page::-webkit-scrollbar-track {
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
  flex-wrap: wrap;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
  flex-wrap: wrap;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.search-input {
  width: 200px;
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

.filter-select {
  width: 140px;
}

.filter-select :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.filter-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

.date-picker {
  width: 360px;
}

.date-picker :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
}

/* User Info */
.user-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.user-name {
  font-weight: 500;
  color: var(--text-primary);
  font-size: 0.875rem;
}

.user-account {
  font-size: 0.75rem;
  color: var(--text-muted);
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

/* Pagination */
.pagination-container {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

.method-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  font-size: 0.75rem;
  font-weight: 600;
  font-family: var(--font-mono, 'SF Mono', 'Menlo', monospace);
}

.method-badge.post {
  background: rgba(103, 194, 58, 0.1);
  color: #67c23a;
}

.method-badge.put {
  background: rgba(230, 162, 60, 0.1);
  color: #e6a23c;
}

.method-badge.delete {
  background: rgba(245, 108, 108, 0.1);
  color: #f56c6c;
}
</style>
