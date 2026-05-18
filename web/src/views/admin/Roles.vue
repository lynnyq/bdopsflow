<template>
  <div class="roles-page">
    <!-- Page Toolbar -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索角色..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadRoles" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Plus" @click="showCreateDialog = true" class="create-btn">
          创建角色
        </el-button>
      </div>
    </div>

    <!-- Table -->
    <div class="table-wrapper">
      <el-table :data="filteredRoles" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="角色名称" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="code" label="角色代码" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="description" label="描述" :minWidth="200" show-overflow-tooltip />
        <el-table-column prop="is_system" label="类型" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_system ? 'danger' : 'success'" effect="light">
              {{ row.is_system ? '系统角色' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button type="warning" link size="small" @click="handleAssignPermissions(row)">
              <el-icon><Key /></el-icon> 权限
            </el-button>
            <el-button
              v-if="!row.is_system"
              type="danger"
              link
              size="small"
              @click="handleDelete(row)"
            >
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无角色数据</p>
          </div>
        </template>
      </el-table>
    </div>

    <el-dialog v-model="showCreateDialog" title="创建角色" width="500px" class="custom-dialog">
      <el-form :model="roleForm" :rules="roleRules" ref="formRef" label-width="100px" class="dialog-form">
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="roleForm.name" placeholder="请输入角色名称" />
        </el-form-item>
        <el-form-item label="角色代码" prop="code">
          <el-input v-model="roleForm.code" placeholder="请输入角色代码，如 custom_role" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="roleForm.description"
            type="textarea"
            placeholder="请输入角色描述"
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

    <el-dialog v-model="showEditDialog" title="编辑角色" width="500px" class="custom-dialog">
      <el-form :model="roleForm" :rules="roleRules" ref="editFormRef" label-width="100px" class="dialog-form">
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="roleForm.name" placeholder="请输入角色名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="roleForm.description"
            type="textarea"
            placeholder="请输入角色描述"
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

    <el-dialog v-model="showPermissionDialog" title="分配权限" width="600px" class="custom-dialog">
      <div v-loading="loadingPermissions">
        <el-alert
          v-if="currentRole?.is_system"
          title="系统角色"
          type="warning"
          :closable="false"
          description="系统角色的权限由系统预设，无法手动修改"
          show-icon
          style="margin-bottom: 16px"
        />
        <el-checkbox-group v-model="selectedPermissionIds" :disabled="currentRole?.is_system">
          <el-checkbox
            v-for="group in (permissionGroups || [])"
            :key="group.resource"
            :label="group.resource"
            border
            style="margin-bottom: 16px; display: block; width: 100%"
          >
            <template #default>
              <div style="padding: 8px 0">
                <strong>{{ group.description }}</strong>
                <div style="margin-top: 8px">
                  <el-checkbox
                    v-for="perm in (group.permissions || [])"
                    :key="perm.code"
                    :value="perm.id"
                    style="margin-left: 0; margin-right: 16px"
                    @change="handlePermissionChange(group.resource)"
                  >
                    {{ perm.description }}
                  </el-checkbox>
                </div>
              </div>
            </template>
          </el-checkbox>
        </el-checkbox-group>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showPermissionDialog = false">取消</el-button>
          <el-button
            v-if="!currentRole?.is_system"
            type="primary"
            @click="handleSavePermissions"
            :loading="submitting"
          >
            保存
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Document, Search, Refresh, Key } from '@element-plus/icons-vue'
import {
  roleAdminAPI,
  permissionAPI,
  type Role,
  type Permission,
  type RolePermission
} from '@/api/admin'
import { handleError, handleSuccess, formatValue } from '@/utils/error'

interface PermissionGroup {
  resource: string
  description: string
  permissions: Array<{ id: number; code: string; description: string }>
}

const roles = ref<Role[]>([])
const loading = ref(false)
const submitting = ref(false)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showPermissionDialog = ref(false)
const loadingPermissions = ref(false)
const searchQuery = ref('')

const permissions = ref<Permission[]>([])
const currentRole = ref<Role | null>(null)
const selectedPermissionIds = ref<number[]>([])

const filteredRoles = computed(() => {
  if (!searchQuery.value) return roles.value
  const query = searchQuery.value.toLowerCase()
  return roles.value.filter(r => 
    r.name.toLowerCase().includes(query) || 
    r.code?.toLowerCase().includes(query) ||
    r.description?.toLowerCase().includes(query)
  )
})

const permissionGroups = computed<PermissionGroup[]>(() => {
  const groups: { [key: string]: PermissionGroup } = {}
  const groupDescriptions: { [key: string]: string } = {
    user: '用户管理',
    role: '角色管理',
    domain: '领域管理',
    task: '任务管理',
    workflow: '工作流管理',
    executor: '执行器管理',
    log: '日志管理',
    dashboard: '仪表盘',
  }

  permissions.value.forEach((perm) => {
    if (!groups[perm.resource]) {
      groups[perm.resource] = {
        resource: perm.resource,
        description: groupDescriptions[perm.resource] || perm.resource,
        permissions: [],
      }
    }
    groups[perm.resource].permissions.push({
      id: generatePermissionId(perm.resource, perm.action),
      code: perm.code,
      description: perm.description,
    })
  })

  return Object.values(groups)
})

const roleForm = ref({
  id: 0,
  name: '',
  code: '',
  description: '',
})

const roleRules = {
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入角色代码', trigger: 'blur' },
    { pattern: /^[a-z_]+$/, message: '角色代码只能包含小写字母和下划线', trigger: 'blur' },
  ],
}

const formRef = ref()
const editFormRef = ref()

const loadRoles = async () => {
  loading.value = true
  try {
    const response = await roleAdminAPI.list()
    roles.value = response.data.items || []
  } catch (error) {
    ElMessage.error('加载角色列表失败')
  } finally {
    loading.value = false
  }
}

const loadPermissions = async () => {
  try {
    const response = await permissionAPI.getAllPermissions()
    permissions.value = response.data.items || []
  } catch (error) {
    console.error('加载权限列表失败', error)
  }
}

const generatePermissionId = (resource: string, action: string): number => {
  const resourceIndex = ['user', 'role', 'domain', 'task', 'workflow', 'executor', 'log', 'dashboard'].indexOf(resource)
  const actionIndex = ['read', 'create', 'update', 'delete', 'manage'].indexOf(action)
  return (resourceIndex + 1) * 100 + (actionIndex + 1)
}

const handleCreate = async () => {
  const form = formRef.value
  if (!form) return

  await form.validate(async (valid) => {
    if (valid) {
      submitting.value = true
      try {
        await roleAdminAPI.create({
          name: roleForm.value.name,
          code: roleForm.value.code,
          description: roleForm.value.description,
        })
        ElMessage.success('创建角色成功')
        showCreateDialog.value = false
        form.resetFields()
        loadRoles()
      } catch (error) {
        ElMessage.error('创建角色失败')
      } finally {
        submitting.value = false
      }
    }
  })
}

const handleEdit = (row: Role) => {
  roleForm.value = {
    id: row.id,
    name: row.name,
    code: row.code,
    description: row.description,
  }
  showEditDialog.value = true
}

const handleUpdate = async () => {
  const form = editFormRef.value
  if (!form) return

  await form.validate(async (valid) => {
    if (valid) {
      submitting.value = true
      try {
        await roleAdminAPI.update(roleForm.value.id, {
          name: roleForm.value.name,
          description: roleForm.value.description,
        })
        ElMessage.success('更新角色成功')
        showEditDialog.value = false
        loadRoles()
      } catch (error) {
        ElMessage.error('更新角色失败')
      } finally {
        submitting.value = false
      }
    }
  })
}

const handleDelete = async (row: Role) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除角色 "${row.name}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    await roleAdminAPI.delete(row.id)
    ElMessage.success('删除角色成功')
    loadRoles()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除角色失败')
    }
  }
}

const handleAssignPermissions = async (row: Role) => {
  currentRole.value = row
  loadingPermissions.value = true
  showPermissionDialog.value = true

  try {
    const response = await roleAdminAPI.getPermissions(row.id)
    selectedPermissionIds.value = response.data.items?.map((p: Permission) =>
      generatePermissionId(p.resource, p.action)
    ) || []
  } catch (error) {
    selectedPermissionIds.value = []
  } finally {
    loadingPermissions.value = false
  }
}

const handlePermissionChange = (resource: string) => {
  const group = permissionGroups.value.find((g) => g.resource === resource)
  if (!group) return

  const allIds = group.permissions.map((p) => p.id)
  const selectedAll = allIds.every((id) => selectedPermissionIds.value.includes(id))

  if (selectedAll) {
    allIds.forEach((id) => {
      if (!selectedPermissionIds.value.includes(id)) {
        selectedPermissionIds.value.push(id)
      }
    })
  }
}

const handleSavePermissions = async () => {
  if (!currentRole.value) return

  submitting.value = true
  try {
    await roleAdminAPI.assignPermissions(currentRole.value.id, {
      permission_ids: selectedPermissionIds.value,
    })
    ElMessage.success('分配权限成功')
    showPermissionDialog.value = false
  } catch (error) {
    ElMessage.error('分配权限失败')
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadRoles()
  loadPermissions()
})
</script>

<style scoped>
.roles-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.roles-page::-webkit-scrollbar {
  width: 8px;
}

.roles-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.roles-page::-webkit-scrollbar-track {
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
