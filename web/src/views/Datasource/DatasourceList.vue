<template>
  <div class="datasource-list-page">
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索数据源..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-select v-model="filterType" placeholder="类型" clearable class="filter-select">
          <el-option
            v-for="(label, key) in dsTypeLabels"
            :key="key"
            :label="label"
            :value="key"
          />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadDatasources" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button v-if="canCreate" :icon="Plus" @click="handleCreate" class="create-btn">新建数据源</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="pagedDatasources" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" :min-width="150" show-overflow-tooltip />
        <el-table-column prop="type" label="类型" width="130" align="center">
          <template #default="{ row }">
            <el-tag effect="light">{{ dsTypeLabels[row.type] || row.type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="主机" :min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            {{ getHostDisplay(row) }}
          </template>
        </el-table-column>
        <el-table-column prop="port" label="端口" width="90" align="center">
          <template #default="{ row }">
            {{ row.port || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="test_status" label="连接状态" width="120" align="center">
          <template #default="{ row }">
            <el-tag
              :type="getTestStatusType(row.test_status)"
              effect="light"
              size="small"
            >
              {{ getTestStatusLabel(row.test_status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_enabled" label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_enabled ? 'success' : 'info'" effect="light" size="small">
              {{ row.is_enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="domain_name" label="所属领域" width="130" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.domain_name || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_by_name" label="创建者" width="100" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.created_by_name || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="allow_write_sql" label="DML" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.allow_write_sql ? 'warning' : 'info'" effect="light" size="small">
              {{ row.allow_write_sql ? '允许' : '只读' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_test_at" label="最后测试" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.last_test_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right" align="center">
          <template #default="{ row }">
            <el-button v-if="canUpdate(row)" type="primary" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button
              type="success"
              link
              size="small"
              @click="handleTestConnection(row)"
              :loading="testingId === row.id"
            >
              <el-icon><Connection /></el-icon> 测试
            </el-button>
            <el-button v-if="canManage(row)" type="warning" link size="small" @click="handlePermission(row)">
              <el-icon><Lock /></el-icon> 权限
            </el-button>
            <el-button v-if="canDelete(row)" type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无数据源</p>
          </div>
        </template>
      </el-table>
    </div>

    <div v-if="filteredDatasources.length > 0" class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="filteredDatasources.length"
        layout="total, sizes, prev, pager, next, jumper"
        :pager-count="5"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Edit, Delete, Document, Search, Refresh, Connection, Lock
} from '@element-plus/icons-vue'
import { datasourceAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import type { Datasource } from '@/types'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const canCreate = computed(() => authStore.hasPermission('datasource', 'create'))

const permWeight: Record<string, number> = {
  manage: 100, update: 50, download: 40, query: 30, read: 20, delete: 10,
}

const hasPermLevel = (row: Datasource, required: string): boolean => {
  const userPerm = permWeight[row.user_permission] || 0
  const reqPerm = permWeight[required] || 0
  return userPerm >= reqPerm
}

const canUpdate = (row: Datasource): boolean => hasPermLevel(row, 'update')

const canManage = (row: Datasource): boolean => hasPermLevel(row, 'manage')

const canDelete = (row: Datasource): boolean => {
  if (hasPermLevel(row, 'manage')) return true
  return row.user_permission === 'delete'
}

const dsTypeLabels: Record<string, string> = {
  mysql: 'MySQL',
  sqlite: 'SQLite',
  rqlite: 'Rqlite',
  hive: 'Hive',
  kyuubi: 'Kyuubi',
  trino: 'Trino',
  spark: 'Spark',
  starrocks: 'StarRocks',
  doris: 'Doris',
}

const datasources = ref<Datasource[]>([])
const loading = ref(false)
const testingId = ref<number | null>(null)
const searchQuery = ref('')
const filterType = ref<string | null>(null)
const currentPage = ref(1)
const pageSize = ref(20)

const filteredDatasources = computed(() => {
  return datasources.value.filter(ds => {
    const matchSearch = !searchQuery.value ||
      ds.name.toLowerCase().includes(searchQuery.value.toLowerCase())
    const matchType = !filterType.value || ds.type === filterType.value
    return matchSearch && matchType
  })
})

const pagedDatasources = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  const end = start + pageSize.value
  return filteredDatasources.value.slice(start, end)
})

const getTestStatusType = (status: string) => {
  switch (status) {
    case 'success': return 'success'
    case 'failed': return 'danger'
    default: return 'info'
  }
}

const getTestStatusLabel = (status: string) => {
  switch (status) {
    case 'success': return '成功'
    case 'failed': return '失败'
    default: return '未测试'
  }
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

const getHostDisplay = (row: Datasource): string => {
  if (row.type === 'sqlite') return row.path || '-'
  if (row.type === 'rqlite') return row.rqlite_hosts || row.host || '-'
  if (['hive', 'kyuubi', 'spark'].includes(row.type)) return row.zk_hosts || row.host || '-'
  return row.host || '-'
}

const loadDatasources = async () => {
  loading.value = true
  try {
    const res = await datasourceAPI.list()
    datasources.value = res.data.items || []
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载数据源列表失败')
    }
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  router.push({ name: 'CreateDatasource' })
}

const handleEdit = (row: Datasource) => {
  router.push({ name: 'EditDatasource', params: { id: row.id } })
}

const handleTestConnection = async (row: Datasource) => {
  testingId.value = row.id
  try {
    await datasourceAPI.testConnection(row.id)
    ElMessage.success('连接测试成功')
    await loadDatasources()
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.response?.data?.error || err.message || '连接测试失败')
    }
  } finally {
    testingId.value = null
  }
}

const handlePermission = (row: Datasource) => {
  router.push({ name: 'DatasourcePermission', params: { id: row.id } })
}

const handleDelete = async (row: Datasource) => {
  try {
    await ElMessageBox.confirm(`确定要删除数据源 "${row.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await datasourceAPI.delete(row.id)
    ElMessage.success('数据源已删除')
    await loadDatasources()
  } catch (err: any) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(err.message || '删除失败')
    }
  }
}

watch([searchQuery, filterType], () => {
  currentPage.value = 1
})

onMounted(() => {
  loadDatasources()
})
</script>

<style scoped>
.datasource-list-page {
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
  width: 140px;
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
  margin-top: var(--space-4);
}

.pagination-container :deep(.el-pagination) {
  display: flex;
  align-items: center;
  gap: 8px;
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
