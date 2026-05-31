<template>
  <div class="domains-page">
    <!-- Page Toolbar -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索领域..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadDomains" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button v-if="authStore.isSystemAdmin" :icon="Plus" @click="showCreateDialog = true" class="create-btn">
          创建领域
        </el-button>
      </div>
    </div>

    <!-- Table -->
    <div class="table-wrapper">
      <el-table :data="filteredDomains" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="领域名称" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="description" label="描述" :minWidth="200" show-overflow-tooltip />
        <el-table-column prop="user_count" label="用户数" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info" effect="light">{{ row.user_count || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="executor_count" label="执行器数" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info" effect="light">{{ row.executor_count || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="task_count" label="任务数" width="100" align="center">
          <template #default="{ row }">
            <el-tag type="info" effect="light">{{ row.task_count || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button v-if="authStore.isSystemAdmin" type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无领域数据</p>
          </div>
        </template>
      </el-table>
    </div>

    <el-dialog v-model="showCreateDialog" title="创建领域" width="500px" class="custom-dialog">
      <el-form :model="domainForm" :rules="domainRules" ref="formRef" label-width="100px" class="dialog-form">
        <el-form-item label="领域名称" prop="name">
          <el-input v-model="domainForm.name" placeholder="请输入领域名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="domainForm.description"
            type="textarea"
            placeholder="请输入领域描述"
            :rows="3"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showCreateDialog = false">取消</el-button>
          <el-button type="primary" @click="handleCreate" :loading="submitting">创建</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditDialog" title="编辑领域" width="500px" class="custom-dialog">
      <el-form :model="domainForm" :rules="domainRules" ref="editFormRef" label-width="100px" class="dialog-form">
        <el-form-item label="领域名称" prop="name">
          <el-input v-model="domainForm.name" placeholder="请输入领域名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="domainForm.description"
            type="textarea"
            placeholder="请输入领域描述"
            :rows="3"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showEditDialog = false">取消</el-button>
          <el-button type="primary" @click="handleUpdate" :loading="submitting">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Document, Search, Refresh } from '@element-plus/icons-vue'
import { domainAdminAPI, type Domain } from '@/api/admin'

import { isHandledError } from '@/utils/api'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const domains = ref<Domain[]>([])
const loading = ref(false)
const submitting = ref(false)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const searchQuery = ref('')

const filteredDomains = computed(() => {
  if (!searchQuery.value) return domains.value
  const query = searchQuery.value.toLowerCase()
  return domains.value.filter((d: Domain) =>
    d.name.toLowerCase().includes(query) || 
    d.description?.toLowerCase().includes(query)
  )
})

const domainForm = ref({
  id: 0,
  name: '',
  description: '',
})

const domainRules = {
  name: [{ required: true, message: '请输入领域名称', trigger: 'blur' }],
}

const formRef = ref()
const editFormRef = ref()

const loadDomains = async () => {
  loading.value = true
  try {
    const response = await domainAdminAPI.list()
    domains.value = response.data.items || []
  } catch (error) {
    if (!isHandledError(error)) {
      ElMessage.error('加载领域列表失败')
    }
  } finally {
    loading.value = false
  }
}

const handleCreate = async () => {
  const form = formRef.value
  if (!form) return

  await form.validate(async (valid: boolean) => {
    if (valid) {
      submitting.value = true
      try {
        await domainAdminAPI.create({
          name: domainForm.value.name,
          description: domainForm.value.description,
        })
        ElMessage.success('创建领域成功')
        showCreateDialog.value = false
        form.resetFields()
        loadDomains()
      } catch (error: any) {
        if (!isHandledError(error)) {
          ElMessage.error(error?.response?.data?.error || '创建领域失败')
        }
      } finally {
        submitting.value = false
      }
    }
  })
}

const handleEdit = (row: Domain) => {
  domainForm.value = {
    id: row.id,
    name: row.name,
    description: row.description,
  }
  showEditDialog.value = true
}

const handleUpdate = async () => {
  const form = editFormRef.value
  if (!form) return

  await form.validate(async (valid: boolean) => {
    if (valid) {
      submitting.value = true
      try {
        await domainAdminAPI.update(domainForm.value.id, {
          name: domainForm.value.name,
          description: domainForm.value.description,
        })
        ElMessage.success('更新领域成功')
        showEditDialog.value = false
        loadDomains()
      } catch (error: any) {
        if (!isHandledError(error)) {
          ElMessage.error(error?.response?.data?.error || '更新领域失败')
        }
      } finally {
        submitting.value = false
      }
    }
  })
}

const handleDelete = async (row: Domain) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除领域 "${row.name}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    await domainAdminAPI.delete(row.id)
    ElMessage.success('删除领域成功')
    loadDomains()
  } catch (error: any) {
    if (error !== 'cancel' && !isHandledError(error)) {
      ElMessage.error(error?.response?.data?.error || '删除领域失败')
    }
  }
}

const formatDate = (date: string) => {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
}

onMounted(() => {
  loadDomains()
})
</script>

<style scoped>
.domains-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.domains-page::-webkit-scrollbar {
  width: 8px;
}

.domains-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.domains-page::-webkit-scrollbar-track {
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

/* Dialog */
.dialog-form {
  padding: var(--space-2) 0;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
}
</style>
