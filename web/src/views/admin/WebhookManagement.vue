<template>
  <div class="webhook-management-page">
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索Webhook名称..."
          :prefix-icon="Search"
          class="search-input"
          clearable
          @clear="handleSearch"
          @keyup.enter="handleSearch"
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadWebhooks" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button v-if="canCreate" :icon="Plus" @click="handleCreate" class="create-btn">创建Webhook</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="filteredWebhooks" v-loading="loading" stripe height="100%">
        <el-table-column prop="name" label="名称" width="180" />
        <el-table-column prop="url" label="URL" min-width="250" show-overflow-tooltip />
        <el-table-column prop="method" label="方法" width="80" align="center">
          <template #default="{ row }">
            <span class="method-badge" :class="row.method?.toLowerCase()">{{ row.method || 'POST' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="is_enabled" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_enabled ? 'success' : 'danger'" effect="light">
              {{ row.is_enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="150" show-overflow-tooltip />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" align="center" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canUpdate" type="primary" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button v-if="canCreate" type="success" link size="small" @click="handleTest(row)" :loading="row._testing">
              <el-icon><Promotion /></el-icon> 测试
            </el-button>
            <el-button v-if="canUpdate" type="warning" link size="small" @click="handleToggleEnabled(row)">
              <el-icon><SwitchButton /></el-icon> {{ row.is_enabled ? '禁用' : '启用' }}
            </el-button>
            <el-button v-if="canDelete" type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Connection /></el-icon>
            <p>暂无Webhook配置</p>
          </div>
        </template>
      </el-table>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑Webhook' : '创建Webhook'"
      width="520px"
      class="custom-dialog"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form :model="form" label-position="top" class="dialog-form">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：钉钉通知" clearable />
        </el-form-item>
        <el-form-item label="URL" required>
          <el-input v-model="form.url" placeholder="https://example.com/webhook" clearable />
        </el-form-item>
        <el-form-item label="HTTP方法">
          <el-select v-model="form.method" class="full-width" placeholder="请选择HTTP方法">
            <el-option label="POST" value="POST" />
            <el-option label="PUT" value="PUT" />
            <el-option label="GET" value="GET" />
          </el-select>
        </el-form-item>
        <el-form-item label="自定义Headers">
          <div class="headers-editor">
            <div v-for="header in form.headerList" :key="header._uid" class="header-row">
              <el-input v-model="header.key" placeholder="Key" class="header-key" clearable />
              <el-input v-model="header.value" placeholder="Value" class="header-value" clearable />
              <el-button :icon="Delete" circle size="small" @click="removeHeader(header)" />
            </div>
            <el-button :icon="Plus" size="small" @click="addHeader">添加Header</el-button>
          </div>
        </el-form-item>
        <el-form-item label="签名密钥">
          <el-input v-model="form.secret" placeholder="可选，用于HMAC-SHA256签名验证" type="password" show-password clearable />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="Webhook用途说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting" size="large">
            {{ isEditing ? '保存修改' : '创建Webhook' }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Plus, Search, Refresh, Connection, Edit, SwitchButton, Promotion } from '@element-plus/icons-vue'
import { webhookAPI } from '@/api'
import type { Webhook } from '@/types'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const canCreate = computed(() => authStore.hasPermission('webhook', 'create'))
const canUpdate = computed(() => authStore.hasPermission('webhook', 'update'))
const canDelete = computed(() => authStore.hasPermission('webhook', 'delete'))

const webhooks = ref<(Webhook & { _testing?: boolean })[]>([])
const loading = ref(false)
const searchQuery = ref('')
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)

const form = ref({
  name: '',
  url: '',
  method: 'POST',
  secret: '',
  description: '',
  headerList: [] as { key: string; value: string }[],
})

const currentDomainId = computed(() => authStore.currentDomainId || 1)

const filteredWebhooks = computed(() => {
  if (!searchQuery.value) return webhooks.value
  const query = searchQuery.value.toLowerCase()
  return webhooks.value.filter(w => w.name.toLowerCase().includes(query) || w.url.toLowerCase().includes(query))
})

const loadWebhooks = async () => {
  loading.value = true
  try {
    const response = await webhookAPI.list(currentDomainId.value)
    webhooks.value = response.data?.items || []
  } catch (error: any) {
    ElMessage.error(error.message || '加载Webhook列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  // filteredWebhooks is computed, auto-updates
}

const handleCreate = () => {
  isEditing.value = false
  editingId.value = null
  form.value = {
    name: '',
    url: '',
    method: 'POST',
    secret: '',
    description: '',
    headerList: [],
  }
  dialogVisible.value = true
}

const handleEdit = (row: Webhook) => {
  isEditing.value = true
  editingId.value = row.id
  let headerList: { key: string; value: string }[] = []
  if (row.headers) {
    try {
      const parsed = typeof row.headers === 'string' ? JSON.parse(row.headers) : row.headers
      headerList = Object.entries(parsed).map(([key, value]) => ({ key, value: String(value), _uid: Date.now() + Math.random() }))
    } catch { /* ignore */ }
  }
  form.value = {
    name: row.name,
    url: row.url,
    method: row.method || 'POST',
    secret: row.secret || '',
    description: row.description || '',
    headerList,
  }
  dialogVisible.value = true
}

const addHeader = () => {
  form.value.headerList.push({ key: '', value: '', _uid: Date.now() + Math.random() })
}

const removeHeader = (header: any) => {
  const idx = form.value.headerList.indexOf(header)
  if (idx !== -1) {
    form.value.headerList.splice(idx, 1)
  }
}

const buildHeadersJson = () => {
  const headers: Record<string, string> = {}
  for (const h of form.value.headerList) {
    if (h.key.trim()) {
      headers[h.key.trim()] = h.value
    }
  }
  return JSON.stringify(headers)
}

const handleSubmit = async () => {
  if (!form.value.name) {
    ElMessage.warning('名称为必填项')
    return
  }
  if (!form.value.url) {
    ElMessage.warning('URL为必填项')
    return
  }

  submitting.value = true
  try {
    const data: Partial<Webhook> = {
      name: form.value.name,
      url: form.value.url,
      method: form.value.method,
      headers: buildHeadersJson(),
      secret: form.value.secret,
      domain_id: currentDomainId.value,
      is_enabled: true,
      description: form.value.description,
    }

    if (isEditing.value && editingId.value) {
      await webhookAPI.update(editingId.value, data)
      ElMessage.success('Webhook更新成功')
    } else {
      await webhookAPI.create(data)
      ElMessage.success('Webhook创建成功')
    }

    dialogVisible.value = false
    loadWebhooks()
  } catch (error: any) {
    ElMessage.error(error.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

const handleToggleEnabled = async (row: Webhook & { _testing?: boolean }) => {
  try {
    const newStatus = !row.is_enabled
    await webhookAPI.update(row.id, { ...row, headers: row.headers, is_enabled: newStatus })
    row.is_enabled = newStatus
    ElMessage.success(newStatus ? 'Webhook已启用' : 'Webhook已禁用')
  } catch (error: any) {
    ElMessage.error(error.message || '操作失败')
  }
}

const handleTest = async (row: Webhook & { _testing?: boolean }) => {
  row._testing = true
  try {
    const response = await webhookAPI.test(row.id)
    const data = response.data
    if (data?.error) {
      ElMessage.error(`测试失败: ${data.error}`)
    } else {
      ElMessage.success(`测试成功 (状态码: ${data?.status_code}, 耗时: ${data?.response_time_ms}ms)`)
    }
  } catch (error: any) {
    ElMessage.error(error.message || '测试失败')
  } finally {
    row._testing = false
  }
}

const handleDelete = async (row: Webhook) => {
  try {
    await ElMessageBox.confirm(
      `确认删除Webhook "${row.name}"？关联的任务将不再推送通知。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' }
    )
    await webhookAPI.delete(row.id)
    ElMessage.success('Webhook已删除')
    loadWebhooks()
  } catch {
    // cancelled
  }
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

onMounted(() => {
  loadWebhooks()
})
</script>

<style scoped>
.webhook-management-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.webhook-management-page::-webkit-scrollbar {
  width: 8px;
}

.webhook-management-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.webhook-management-page::-webkit-scrollbar-track {
  background: var(--bg-secondary);
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

.method-badge.get {
  background: rgba(64, 158, 255, 0.1);
  color: #409eff;
}

.headers-editor {
  width: 100%;
}

.header-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
}

.header-key {
  width: 180px;
}

.header-value {
  flex: 1;
}

.full-width {
  width: 100%;
}

.dialog-form {
  padding: var(--space-2) 0;
}

.dialog-form :deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--text-primary);
}

.dialog-form :deep(.el-input__wrapper) {
  border-radius: var(--radius-md);
}

.dialog-form :deep(.el-select) {
  width: 100%;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
  padding: var(--space-3) 0 0;
}
</style>
