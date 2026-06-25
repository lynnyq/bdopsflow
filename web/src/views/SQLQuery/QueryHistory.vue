<template>
  <div class="query-history-page">
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索查询历史..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-select v-model="filterDatasourceID" placeholder="数据源" clearable class="filter-select filter-datasource">
          <el-option
            v-for="ds in datasourceList"
            :key="ds.id"
            :label="ds.name"
            :value="ds.id"
          />
        </el-select>
        <el-select v-model="filterStatus" placeholder="状态" clearable class="filter-select">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-select
          v-model="filterExecutedBy"
          placeholder="执行用户"
          clearable
          filterable
          class="filter-select filter-user"
        >
          <el-option
            v-for="u in userList"
            :key="u.id"
            :label="u.real_name || u.username"
            :value="u.id"
          />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          class="filter-datepicker"
          :clearable="true"
        />
      </div>
      <div class="toolbar-right">
        <el-button
          v-if="selectedIds.length > 0"
          type="danger"
          :icon="Delete"
          @click="handleBatchDelete"
          :loading="batchDeleting"
        >
          批量删除 ({{ selectedIds.length }})
        </el-button>
        <el-button :icon="Refresh" @click="loadHistory" :loading="loading" class="refresh-btn">刷新</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table
        :data="pagedList"
        v-loading="loading"
        stripe
        height="100%"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="45" />
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column label="数据源" width="150">
          <template #default="{ row }">
            {{ row.datasource_name || row.datasource_id || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="执行用户" width="120" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.executed_by_name" type="info" effect="plain" size="small">
              {{ row.executed_by_name }}
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="SQL 摘要" :min-width="250" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="sql-snippet">{{ row.sql_text }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" effect="light" size="small">
              {{ row.status === 'success' ? '成功' : '失败' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="执行时间" width="120" align="right">
          <template #default="{ row }">
            {{ row.execution_time != null && row.execution_time > 0 ? (row.execution_time < 1 ? `${(row.execution_time * 1000).toFixed(0)}ms` : `${row.execution_time.toFixed(2)}s`) : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="row_count" label="行数" width="80" align="right">
          <template #default="{ row }">
            {{ row.row_count ?? '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleReExecute(row)">
              <el-icon><VideoPlay /></el-icon> 重新执行
            </el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无查询历史</p>
          </div>
        </template>
      </el-table>
    </div>

    <div v-if="total > 0" class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        :pager-count="5"
        @current-change="loadHistory"
        @size-change="loadHistory"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Document, VideoPlay, Delete } from '@element-plus/icons-vue'
import { queryAPI, datasourceAPI, userAdminAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import type { QueryHistory, Datasource, User } from '@/types'

const router = useRouter()

const historyList = ref<QueryHistory[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filterDatasourceID = ref<number | null>(null)
const filterStatus = ref<string | null>(null)
const filterExecutedBy = ref<number | null>(null)
const dateRange = ref<[string, string] | null>(null)
const datasourceList = ref<Datasource[]>([])
const userList = ref<User[]>([])
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const selectedIds = ref<number[]>([])
const batchDeleting = ref(false)

const filteredList = computed(() => {
  return historyList.value.filter(item => {
    const matchSearch = !searchQuery.value ||
      item.sql_text.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      (item.datasource_name || '').toLowerCase().includes(searchQuery.value.toLowerCase())
    return matchSearch
  })
})

const pagedList = computed(() => {
  return filteredList.value
})

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

const loadHistory = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value,
    }
    if (filterDatasourceID.value) {
      params.datasource_id = filterDatasourceID.value
    }
    if (filterStatus.value) {
      params.status = filterStatus.value
    }
    if (filterExecutedBy.value) {
      params.executed_by = filterExecutedBy.value
    }
    if (dateRange.value && dateRange.value[0]) {
      params.start_time = dateRange.value[0]
    }
    if (dateRange.value && dateRange.value[1]) {
      params.end_time = dateRange.value[1]
    }
    const res = await queryAPI.getHistory(params)
    historyList.value = res.data.items || []
    total.value = res.data.total || 0
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载查询历史失败')
    }
  } finally {
    loading.value = false
  }
}

const loadDatasources = async () => {
  try {
    const res = await datasourceAPI.list({ page: 1, page_size: 200 })
    datasourceList.value = res.data.items || []
  } catch (err: any) {
    // 数据源列表加载失败不影响主流程
  }
}

const loadUsers = async () => {
  try {
    const res = await userAdminAPI.listByDomain()
    userList.value = res.data.items || []
  } catch (err: any) {
    // 用户列表加载失败不影响主流程
  }
}

// 监听过滤条件变化，重新加载数据
watch([filterDatasourceID, filterStatus, filterExecutedBy, dateRange], () => {
  currentPage.value = 1
  loadHistory()
})

const handleSelectionChange = (selection: QueryHistory[]) => {
  selectedIds.value = selection.map(item => item.id)
}

const handleReExecute = (row: QueryHistory) => {
  const query: Record<string, string> = { sql: row.sql_text }
  if (row.datasource_id != null) {
    query.datasource_id = String(row.datasource_id)
  }
  router.push({
    name: 'SQLQuery',
    query,
  })
}

const handleDelete = async (row: QueryHistory) => {
  try {
    await ElMessageBox.confirm('确定要删除该查询历史记录吗？', '删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await queryAPI.deleteHistory(row.id)
    ElMessage.success('删除成功')
    loadHistory()
  } catch (err: any) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(err.message || '删除失败')
    }
  }
}

const handleBatchDelete = async () => {
  try {
    await ElMessageBox.confirm(`确定要删除选中的 ${selectedIds.value.length} 条查询历史记录吗？`, '批量删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    batchDeleting.value = true
    await queryAPI.batchDeleteHistory(selectedIds.value)
    ElMessage.success('批量删除成功')
    selectedIds.value = []
    loadHistory()
  } catch (err: any) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(err.message || '批量删除失败')
    }
  } finally {
    batchDeleting.value = false
  }
}

onMounted(() => {
  loadDatasources()
  loadUsers()
  loadHistory()
})
</script>

<style scoped>
.query-history-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  height: 100%;
  min-height: 0;
}

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
  width: 120px;
}

.filter-datasource {
  width: 160px;
}

.filter-user {
  width: 140px;
}

.filter-datepicker {
  width: 260px;
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

.table-wrapper {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  min-height: 0;
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

.sql-snippet {
  font-family: var(--font-mono);
  font-size: 12px;
  background: var(--bg-secondary);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-secondary);
}

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

.pagination-container {
  display: flex;
  justify-content: center;
  padding: var(--space-6);
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-sm);
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

.pagination-container :deep(.el-pager li.is-active) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  color: white;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}
</style>
