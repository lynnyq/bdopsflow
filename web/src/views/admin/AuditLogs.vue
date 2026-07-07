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
          @input="debouncedSearch"
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
      </div>
      <div class="toolbar-right">
        <el-button text @click="showAdvancedFilters = !showAdvancedFilters" class="advanced-filter-btn">
          <el-icon><Filter /></el-icon>
          高级筛选
          <span v-if="activeFilterCount > 0" class="filter-count-badge">{{ activeFilterCount }}</span>
          <el-icon class="arrow-icon" :class="{ 'is-active': showAdvancedFilters }">
            <ArrowDown />
          </el-icon>
        </el-button>
        <el-button :icon="Refresh" @click="handleSearch" :loading="loading">刷新</el-button>
        <el-button :icon="Delete" @click="handleCleanExpired" type="warning">
          清理过期
        </el-button>
      </div>
    </div>

    <!-- 高级筛选区域：时间范围 + 保留天数 + 重置 -->
    <div v-if="showAdvancedFilters" class="advanced-filters">
      <div class="advanced-filters-row">
        <div class="filter-group">
          <span class="filter-label">时间范围</span>
          <el-date-picker
            v-model="dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            format="YYYY-MM-DD HH:mm:ss"
            value-format="YYYY-MM-DD HH:mm:ss"
            class="filter-date"
            @change="handleDateChange"
          />
        </div>
        <div class="filter-group">
          <span class="filter-label">保留天数</span>
          <div class="retention-control">
            <el-input-number
              v-model="retentionDays"
              :min="1"
              :max="3650"
              :step="1"
              size="small"
              controls-position="right"
              class="retention-input"
              :loading="retentionLoading"
            />
            <el-button
              size="small"
              type="primary"
              plain
              :loading="retentionSaving"
              :disabled="retentionDays === originalRetentionDays"
              @click="handleUpdateRetention"
            >
              保存
            </el-button>
          </div>
        </div>
        <div class="filter-group filter-actions">
          <el-button @click="resetFilters" size="small">重置</el-button>
        </div>
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
        <el-table-column label="操作" width="80" align="center" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="showDetail(row)">详情</el-button>
          </template>
        </el-table-column>
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

    <!-- 详情抽屉：展示单条审计日志的完整信息（包括 detail、user_agent 等不在表格中显示的字段） -->
    <el-drawer
      v-model="detailVisible"
      title="审计日志详情"
      direction="rtl"
      size="500px"
    >
      <div v-if="currentDetail" class="detail-content">
        <div class="detail-row"><span class="label">时间</span><span>{{ formatTime(currentDetail.created_at) }}</span></div>
        <div class="detail-row"><span class="label">操作人</span><span>{{ currentDetail.real_name || currentDetail.username }}</span></div>
        <div class="detail-row"><span class="label">用户名</span><span>{{ currentDetail.username }}</span></div>
        <div class="detail-row"><span class="label">角色</span><span>{{ currentDetail.role || '-' }}</span></div>
        <div class="detail-row"><span class="label">域 ID</span><span>{{ currentDetail.domain_id ?? '-' }}</span></div>
        <div class="detail-row"><span class="label">操作类型</span><span>{{ getActionLabel(currentDetail.action) }} ({{ currentDetail.action }})</span></div>
        <div class="detail-row"><span class="label">资源类型</span><span>{{ getResourceLabel(currentDetail.resource) }} ({{ currentDetail.resource }})</span></div>
        <div class="detail-row"><span class="label">资源 ID</span><span>{{ currentDetail.resource_id || '-' }}</span></div>
        <div class="detail-row"><span class="label">资源名称</span><span>{{ currentDetail.resource_name || '-' }}</span></div>
        <div class="detail-row">
          <span class="label">状态</span>
          <el-tag :type="currentDetail.status === 'success' ? 'success' : 'danger'" size="small">
            {{ currentDetail.status === 'success' ? '成功' : '失败' }}
          </el-tag>
        </div>
        <div class="detail-row"><span class="label">响应码</span><span>{{ currentDetail.response_code ?? '-' }}</span></div>
        <div class="detail-row"><span class="label">IP 地址</span><span>{{ currentDetail.ip_address || '-' }}</span></div>
        <div class="detail-row"><span class="label">请求方法</span><span>{{ currentDetail.request_method || '-' }}</span></div>
        <div class="detail-row"><span class="label">请求路径</span><span class="mono">{{ currentDetail.request_path || '-' }}</span></div>
        <div class="detail-row"><span class="label">User Agent</span><span class="mono">{{ currentDetail.user_agent || '-' }}</span></div>
        <div class="detail-row detail-detail">
          <span class="label">详情</span>
          <pre class="detail-pre">{{ currentDetail.detail || '-' }}</pre>
        </div>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Search, Refresh, Document, Filter, ArrowDown } from '@element-plus/icons-vue'
import { auditLogAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import type { AuditLog } from '@/api/audit'

const logs = ref<AuditLog[]>([])
const loading = ref(false)
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const dateRange = ref<[string, string] | null>(null)
const searchQuery = ref('')

// 高级筛选区域展开/收起状态
const showAdvancedFilters = ref(false)

// 审计日志详情抽屉
const detailVisible = ref(false)
const currentDetail = ref<AuditLog | null>(null)

// 审计日志保留天数：从后端 audit_log.retention_days 读取，支持在页面内修改
const retentionDays = ref(90)
const originalRetentionDays = ref(90)
const retentionLoading = ref(false)
const retentionSaving = ref(false)

// 搜索防抖定时器
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

const filterForm = ref({
  username: '',
  action: '',
  resource: '',
  status: '',
})

// 当前生效的筛选条件数量（用于高级筛选按钮上的徽标提示）
const activeFilterCount = computed(() => {
  let count = 0
  if (searchQuery.value) count++
  if (filterForm.value.action) count++
  if (filterForm.value.resource) count++
  if (filterForm.value.status) count++
  if (dateRange.value && dateRange.value[0]) count++
  return count
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
  { value: 'generate_curl', label: '生成CURL' },
  { value: 'delete_result', label: '删除结果' },
  { value: 'parse', label: '解析Proto' },
  { value: 'reflect', label: '反射调用' },
  { value: 'generate_template', label: '生成模板' },
  { value: 'generate_fields', label: '生成字段' },
  { value: 'reveal', label: '查看Token' },
  { value: 'cancel', label: '取消查询' },
  { value: 'clear_cache', label: '清理缓存' },
  { value: 'update_profile', label: '更新资料' },
  { value: 'generate', label: '生成Token' },
]

const resourceOptions = [
  { value: 'auth', label: '认证' },
  { value: 'user', label: '用户' },
  { value: 'role', label: '角色' },
  { value: 'domain', label: '领域' },
  { value: 'datasource', label: '数据源' },
  { value: 'task', label: '任务' },
  { value: 'executor', label: '执行器' },
  { value: 'config', label: '系统配置' },
  { value: 'query', label: '查询' },
  { value: 'saved_sql', label: '保存SQL' },
  { value: 'query_history', label: '查询历史' },
  { value: 'log', label: '日志' },
  { value: 'api_test', label: '接口测试' },
  { value: 'certificate', label: '证书' },
  { value: 'proto_file', label: 'Proto文件' },
  { value: 'api_token', label: 'API令牌' },
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
  if (['create', 'login', 'register', 'online', 'resume', 'generate'].includes(action)) return 'success'
  if (['update', 'assign', 'change_password', 'config_change', 'trigger', 'test_connection', 'execute', 'export', 'update_profile'].includes(action)) return 'warning'
  if (['delete', 'revoke', 'reset_password', 'offline', 'pause', 'clean', 'cancel', 'clear_cache'].includes(action)) return 'danger'
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

// 搜索输入防抖：300ms 内连续输入只触发一次查询，避免频繁 API 调用
const debouncedSearch = () => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }
  searchDebounceTimer = setTimeout(() => {
    currentPage.value = 1
    handleSearch()
  }, 300)
}

// 展示单条审计日志详情
const showDetail = (row: AuditLog) => {
  currentDetail.value = row
  detailVisible.value = true
}

// 重置所有筛选条件
const resetFilters = () => {
  searchQuery.value = ''
  filterForm.value = {
    username: '',
    action: '',
    resource: '',
    status: '',
  }
  dateRange.value = null
  currentPage.value = 1
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
    if (!isHandledError(error)) {
      ElMessage.error(error.message || '加载审计日志失败')
    }
  } finally {
    loading.value = false
  }
}

const handleCleanExpired = async () => {
  try {
    await ElMessageBox.confirm(
      `确认清理过期的审计日志？系统将根据保留天数（${retentionDays.value} 天）自动清理过期记录。`,
      '清理确认',
      { confirmButtonText: '确认清理', cancelButtonText: '取消', type: 'warning' }
    )
    const response = await auditLogAPI.cleanExpired()
    const data = response.data
    ElMessage.success(`清理完成，共删除 ${data?.deleted_count || 0} 条过期日志`)
    handleSearch()
  } catch (err: any) {
    // ElMessageBox 取消时 reject 的值是 'cancel' / 'close'，非错误
    if (err !== 'cancel' && err !== 'close') {
      if (!isHandledError(err)) {
        ElMessage.error(err?.message || '清理失败')
      }
    }
  }
}

// 加载当前保留天数配置
const loadRetention = async () => {
  retentionLoading.value = true
  try {
    const res = await auditLogAPI.getRetention()
    const days = res.data?.retention_days
    if (typeof days === 'number' && days > 0) {
      retentionDays.value = days
      originalRetentionDays.value = days
    }
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err?.message || '加载保留天数失败')
    }
  } finally {
    retentionLoading.value = false
  }
}

// 修改保留天数，成功后同步 originalRetentionDays（用于"保存"按钮 disabled 判断）
const handleUpdateRetention = async () => {
  if (retentionDays.value === originalRetentionDays.value) return
  retentionSaving.value = true
  try {
    await auditLogAPI.updateRetention(retentionDays.value)
    originalRetentionDays.value = retentionDays.value
    ElMessage.success('保留天数已更新')
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err?.message || '更新保留天数失败')
    }
    // 失败时回滚到原始值
    retentionDays.value = originalRetentionDays.value
  } finally {
    retentionSaving.value = false
  }
}

onMounted(() => {
  handleSearch()
  loadRetention()
})

// 组件卸载时清理防抖定时器，避免内存泄漏
onUnmounted(() => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
    searchDebounceTimer = null
  }
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

/* 高级筛选按钮 */
.advanced-filter-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 6px;
  position: relative;
}

.advanced-filter-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.advanced-filter-btn .arrow-icon {
  transition: transform var(--duration-normal) var(--ease-out);
}

.advanced-filter-btn .arrow-icon.is-active {
  transform: rotate(180deg);
}

/* 筛选条件数量徽标 */
.filter-count-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  background: var(--accent-primary);
  color: #fff;
  font-size: 0.7rem;
  font-weight: 600;
  border-radius: 9px;
  line-height: 1;
}

/* 高级筛选区域 */
.advanced-filters {
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  margin-top: calc(-1 * var(--space-2));
}

.advanced-filters-row {
  display: flex;
  align-items: flex-end;
  gap: var(--space-4);
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.filter-group.filter-actions {
  flex-shrink: 0;
}

.filter-label {
  font-size: 0.8rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.filter-date {
  width: 360px;
}

.filter-date :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.filter-date :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.filter-date :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.retention-control {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.retention-input {
  width: 110px;
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

/* 详情抽屉 */
.detail-content {
  padding: 16px;
}

.detail-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 0.875rem;
}

.detail-row .label {
  flex-shrink: 0;
  width: 80px;
  color: var(--text-muted);
}

.detail-row .mono {
  font-family: var(--font-mono, 'SF Mono', 'Menlo', monospace);
  word-break: break-all;
}

.detail-detail {
  flex-direction: column;
}

.detail-pre {
  margin-top: 8px;
  padding: 12px;
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  font-family: var(--font-mono, 'SF Mono', 'Menlo', monospace);
  font-size: 0.8rem;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 300px;
  overflow-y: auto;
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
