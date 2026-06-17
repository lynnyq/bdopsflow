<template>
  <div class="test-history-page">
    <!-- Toolbar -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索接口名称..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-select v-model="filterType" placeholder="类型" clearable class="filter-select" @change="handleFilter">
          <el-option label="HTTP" value="http" />
          <el-option label="gRPC" value="grpc" />
          <el-option label="gRPC 连接测试" value="grpc_connect_test" />
        </el-select>
        <el-select v-model="filterStatus" placeholder="状态" clearable class="filter-select" @change="handleFilter">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="error" />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadResults" :loading="loading" class="refresh-btn">刷新</el-button>
      </div>
    </div>

    <!-- Table -->
    <div class="table-wrapper">
      <el-table :data="filteredResults" v-loading="loading" stripe height="100%" @row-click="handleRowClick">
        <el-table-column label="类型" width="110" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.type === 'grpc_connect_test'" type="info" effect="dark" size="small" disable-transitions>
              gRPC连接
            </el-tag>
            <el-tag v-else :type="row.type === 'grpc' ? 'warning' : 'primary'" effect="dark" size="small" disable-transitions>
              {{ row.type === 'grpc' ? 'gRPC' : 'HTTP' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="接口名称" min-width="200">
          <template #default="{ row }">
            <span class="test-name" :title="row.test_name || '临时请求'">{{ row.test_name || '临时请求' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.error ? 'danger' : 'success'" effect="light" size="small">
              {{ row.error ? '失败' : '成功' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态码" width="100" align="center">
          <template #default="{ row }">
            <span v-if="row.type === 'grpc'" class="status-code" :class="row.status_code === 0 ? 'status-ok' : 'status-error'">
              {{ row.status_code }}
            </span>
            <span v-else class="status-code" :class="row.status_code >= 200 && row.status_code < 300 ? 'status-ok' : 'status-error'">
              {{ row.status_code }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="100" align="right">
          <template #default="{ row }">
            <span class="latency">{{ row.latency_ms }}ms</span>
          </template>
        </el-table-column>
        <el-table-column label="执行时间" width="180">
          <template #default="{ row }">
            <span class="time-text" :title="formatDateTime(row.created_at)">{{ formatRelativeTime(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click.stop="handleViewDetail(row)">详情</el-button>
            <el-button type="danger" link size="small" @click.stop="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无执行记录</p>
          </div>
        </template>
      </el-table>
    </div>

    <!-- Pagination -->
    <div v-if="total > 0" class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[20, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        :pager-count="5"
        @current-change="loadResults"
        @size-change="handleSizeChange"
      />
    </div>

    <!-- Detail Dialog -->
    <el-dialog v-model="detailVisible" title="执行详情" width="700px" destroy-on-close>
      <div v-if="detailResult" class="detail-content">
        <div class="detail-header">
          <el-tag :type="detailResult.type === 'grpc' ? 'warning' : 'primary'" effect="dark" size="small">
            {{ detailResult.type === 'grpc' ? 'gRPC' : 'HTTP' }}
          </el-tag>
          <el-tag :type="detailResult.error ? 'danger' : 'success'" effect="light" size="small">
            {{ detailResult.error ? '失败' : '成功' }}
          </el-tag>
          <span class="detail-meta">状态码: {{ detailResult.status_code }}</span>
          <span class="detail-meta">耗时: {{ detailResult.latency_ms }}ms</span>
          <span class="detail-meta">{{ formatDateTime(detailResult.created_at) }}</span>
        </div>

        <div v-if="detailResult.test_name" class="detail-section">
          <div class="detail-label">接口名称</div>
          <div class="detail-value">{{ detailResult.test_name }}</div>
        </div>

        <div v-if="detailResult.error" class="detail-section">
          <div class="detail-label">错误信息</div>
          <pre class="detail-pre error-pre">{{ detailResult.error }}</pre>
        </div>

        <div v-if="detailResult.body" class="detail-section">
          <div class="detail-label">响应体</div>
          <pre class="detail-pre">{{ formatBody(detailResult.body) }}</pre>
        </div>

        <div v-if="detailResult.headers" class="detail-section">
          <div class="detail-label">响应头</div>
          <pre class="detail-pre">{{ formatHeaders(detailResult.headers) }}</pre>
        </div>

        <div v-if="detailResult.assertions_result" class="detail-section">
          <div class="detail-label">断言结果</div>
          <pre class="detail-pre">{{ formatAssertions(detailResult.assertions_result) }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Document } from '@element-plus/icons-vue'
import { apiTestAPI } from '@/api'
import type { ApiTestResult } from '@/api/apiTest'
import { isHandledError } from '@/utils/api'
import { formatDateTime } from '@/utils/format'

const loading = ref(false)
const results = ref<ApiTestResult[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const searchQuery = ref('')
const filterType = ref('')
const filterStatus = ref('')
const detailVisible = ref(false)
const detailResult = ref<ApiTestResult | null>(null)

const filteredResults = computed(() => {
  let list = results.value
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    list = list.filter(r => (r.test_name || '临时请求').toLowerCase().includes(query))
  }
  if (filterStatus.value === 'success') {
    list = list.filter(r => !r.error)
  } else if (filterStatus.value === 'error') {
    list = list.filter(r => r.error)
  }
  return list
})

watch(searchQuery, () => {
  currentPage.value = 1
})

function formatRelativeTime(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return dateStr
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return '刚刚'
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}分钟前`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}小时前`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 30) return `${diffDay}天前`
  return formatDateTime(dateStr)
}

function formatBody(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

function formatHeaders(headers: string): string {
  try {
    return JSON.stringify(JSON.parse(headers), null, 2)
  } catch {
    return headers
  }
}

function formatAssertions(assertions: string): string {
  try {
    return JSON.stringify(JSON.parse(assertions), null, 2)
  } catch {
    return assertions
  }
}

const loadResults = async () => {
  loading.value = true
  try {
    const params: { type?: string; page: number; page_size: number } = {
      page: currentPage.value,
      page_size: pageSize.value,
    }
    if (filterType.value) {
      params.type = filterType.value
    }
    const res = await apiTestAPI.listResults(params)
    results.value = res.data?.items || []
    total.value = res.data?.total || 0
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error('加载执行历史失败')
    }
  } finally {
    loading.value = false
  }
}

const handleFilter = () => {
  currentPage.value = 1
  loadResults()
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  loadResults()
}

const handleRowClick = (row: ApiTestResult) => {
  handleViewDetail(row)
}

const handleViewDetail = (row: ApiTestResult) => {
  detailResult.value = row
  detailVisible.value = true
}

const handleDelete = async (row: ApiTestResult) => {
  try {
    await ElMessageBox.confirm('确定删除该执行记录吗？', '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await apiTestAPI.deleteResult(row.id)
    ElMessage.success('已删除')
    loadResults()
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error('删除失败')
    }
  }
}

onMounted(() => {
  loadResults()
})
</script>

<style scoped>
.test-history-page {
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

.table-wrapper {
  flex: 1;
  min-height: 0;
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

.test-name {
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-code {
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 0.85rem;
}

.status-ok { color: var(--accent-success, #67c23a); }
.status-error { color: var(--accent-danger, #f56c6c); }

.latency {
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.time-text {
  font-size: 0.85rem;
  color: var(--text-secondary);
  cursor: help;
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

/* Detail Dialog */
.detail-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.detail-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding-bottom: var(--space-3);
  border-bottom: 1px solid var(--border-subtle);
}

.detail-meta {
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.detail-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.detail-value {
  font-size: 0.9rem;
  color: var(--text-primary);
}

.detail-pre {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: var(--space-3);
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 0.82rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 300px;
  overflow-y: auto;
  margin: 0;
}

.error-pre {
  color: var(--accent-danger, #f56c6c);
  background: rgba(245, 108, 108, 0.05);
  border-color: rgba(245, 108, 108, 0.15);
}
</style>
