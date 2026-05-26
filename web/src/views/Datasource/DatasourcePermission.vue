<template>
  <div class="datasource-permission-page">
    <div class="page-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" @click="handleBack" class="back-btn">返回列表</el-button>
        <div class="header-divider"></div>
        <div class="header-info">
          <h2 class="page-title">权限管理</h2>
          <div class="ds-badge">
            <el-icon><Connection /></el-icon>
            <span>{{ datasourceName || '加载数据源...' }}</span>
          </div>
        </div>
      </div>
      <div class="header-stats">
        <div class="stat-item">
          <span class="stat-value">{{ permissions.length }}</span>
          <span class="stat-label">权限总数</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-value">{{ uniqueUsersCount }}</span>
          <span class="stat-label">已授权用户</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-value">{{ uniqueRolesCount }}</span>
          <span class="stat-label">已授权角色</span>
        </div>
      </div>
    </div>

    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索用户、角色或权限..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-select v-model="filterType" placeholder="权限类型" clearable class="filter-select">
          <el-option
            v-for="(label, key) in permissionLabels"
            :key="key"
            :label="label"
            :value="key"
          >
            <div class="filter-option">
              <el-icon :size="14"><component :is="getPermissionIcon(key)" /></el-icon>
              <span>{{ label }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select v-model="filterTarget" placeholder="授权对象" clearable class="filter-select">
          <el-option label="用户" value="user" />
          <el-option label="角色" value="role" />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadPermissions" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Plus" @click="handleAddPermission" class="create-btn">添加权限</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="filteredPermissions" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column label="授权对象" :min-width="200">
          <template #default="{ row }">
            <div class="target-cell">
              <div class="target-avatar" :class="row.user_id ? 'user-avatar' : 'role-avatar'">
                <el-icon :size="16"><component :is="row.user_id ? User : UserFilled" /></el-icon>
              </div>
              <div class="target-info">
                <span class="target-name">
                  <el-tag v-if="row.user_id" size="small" type="success" effect="plain" class="target-tag">用户</el-tag>
                  <el-tag v-else size="small" type="" effect="plain" class="target-tag">角色</el-tag>
                  {{ row.user_id ? getUserName(row.user_id) : getRoleName(row.role_id!) }}
                </span>
                <span class="target-id">ID: {{ row.user_id || row.role_id }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="permission_type" label="权限类型" width="200" align="center">
          <template #default="{ row }">
            <div class="permission-cell">
              <div class="permission-badge" :class="`permission-${row.permission_type}`">
                <el-icon :size="14"><component :is="getPermissionIcon(row.permission_type)" /></el-icon>
                <span>{{ getPermissionLabel(row.permission_type) }}</span>
              </div>
              <div v-if="permissionHierarchy[row.permission_type]?.length" class="permission-includes">
                含 {{ permissionHierarchy[row.permission_type].map(k => permissionLabels[k]).join('、') }}
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="granted_by" label="授权人" width="140" align="center">
          <template #default="{ row }">
            <div class="granted-by-cell">
              <el-icon :size="14"><User /></el-icon>
              <span>{{ row.granted_by || '-' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="granted_at" label="授权时间" width="180">
          <template #default="{ row }">
            <div class="time-cell">
              <el-icon :size="14"><Clock /></el-icon>
              <span>{{ formatDateTime(row.granted_at) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleEditPermission(row)" class="edit-btn">
              <el-icon><Edit /></el-icon>
              <span>修改</span>
            </el-button>
            <el-button type="danger" link size="small" @click="handleDeletePermission(row)" class="delete-btn">
              <el-icon><Delete /></el-icon>
              <span>删除</span>
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <div class="empty-icon">
              <el-icon :size="48"><Lock /></el-icon>
            </div>
            <p class="empty-title">暂无权限数据</p>
            <span class="empty-hint">点击"添加权限"为用户或角色分配数据源访问权限</span>
          </div>
        </template>
      </el-table>
    </div>

    <el-dialog
      v-model="addDialogVisible"
      title="添加权限"
      width="560px"
      :close-on-click-modal="false"
      class="permission-dialog"
    >
      <div class="dialog-content">
        <div class="dialog-section">
          <div class="section-header">
            <div class="section-icon target-icon">
              <el-icon :size="18"><User /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">选择授权对象</span>
              <span class="title-desc">选择按用户或按角色分配数据源访问权限</span>
            </div>
          </div>
          <div class="section-body">
            <el-form ref="addFormRef" :model="addForm" :rules="addRules" label-position="top" class="dialog-form">
              <el-form-item label="授权方式" prop="target_type">
                <el-radio-group v-model="addForm.target_type" @change="handleTargetTypeChange">
                  <el-radio value="user">
                    <div class="radio-label">
                      <el-icon :size="14"><User /></el-icon>
                      <span>按用户</span>
                    </div>
                  </el-radio>
                  <el-radio value="role">
                    <div class="radio-label">
                      <el-icon :size="14"><UserFilled /></el-icon>
                      <span>按角色</span>
                    </div>
                  </el-radio>
                </el-radio-group>
              </el-form-item>
              <el-form-item v-if="addForm.target_type === 'user'" label="选择用户" prop="user_id">
                <el-select v-model="addForm.user_id" placeholder="请选择用户" style="width: 100%" filterable>
                  <el-option
                    v-for="u in users"
                    :key="u.id"
                    :label="u.username"
                    :value="u.id"
                  >
                    <div class="user-select-option">
                      <div class="user-select-avatar">
                        <el-icon :size="14"><User /></el-icon>
                      </div>
                      <span class="user-select-name">{{ u.username }}</span>
                      <el-tag size="small" :type="getRoleTagType(u.role)" effect="plain">{{ getRoleDisplayName(u.role) }}</el-tag>
                      <span class="user-select-id">ID: {{ u.id }}</span>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>
              <el-form-item v-if="addForm.target_type === 'role'" label="选择角色" prop="role_id">
                <el-select v-model="addForm.role_id" placeholder="请选择角色" style="width: 100%" filterable>
                  <el-option
                    v-for="role in roles"
                    :key="role.id"
                    :label="role.name"
                    :value="role.id"
                  >
                    <div class="role-select-option">
                      <div class="role-select-avatar">
                        <el-icon :size="14"><UserFilled /></el-icon>
                      </div>
                      <span class="role-select-name">{{ role.name }}</span>
                      <el-tag v-if="role.is_system" size="small" type="danger" effect="plain">系统</el-tag>
                      <span class="role-select-id">ID: {{ role.id }}</span>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>
            </el-form>
          </div>
        </div>

        <div class="dialog-section">
          <div class="section-header">
            <div class="section-icon perm-icon">
              <el-icon :size="18"><Lock /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">权限配置</span>
              <span class="title-desc">选择要授予的权限类型</span>
            </div>
          </div>
          <div class="section-body">
            <el-form :model="addForm" :rules="addRules" label-position="top" class="dialog-form">
              <el-form-item label="权限类型" prop="permission_type">
                <el-select v-model="addForm.permission_type" placeholder="请选择权限类型" style="width: 100%">
                  <el-option
                    v-for="(label, key) in permissionLabels"
                    :key="key"
                    :label="label"
                    :value="key"
                  >
                    <div class="perm-select-option">
                      <div class="perm-badge-mini" :class="`permission-${key}`">
                        <el-icon :size="12"><component :is="getPermissionIcon(key)" /></el-icon>
                      </div>
                      <div class="perm-select-info">
                        <span class="perm-select-label">{{ label }}</span>
                        <span class="perm-select-desc">{{ permissionDescs[key] }}</span>
                      </div>
                    </div>
                  </el-option>
                </el-select>
                <div v-if="addForm.permission_type && permissionHierarchy[addForm.permission_type]?.length" class="perm-hierarchy-hint">
                  <el-icon :size="14"><Check /></el-icon>
                  <span>自动包含：{{ permissionHierarchy[addForm.permission_type].map(k => permissionLabels[k]).join('、') }}权限</span>
                </div>
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="addDialogVisible = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleSubmitAdd" :loading="submitting" size="large">
            <el-icon><Check /></el-icon>
            确认添加
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="editDialogVisible"
      title="修改权限"
      width="480px"
      :close-on-click-modal="false"
      class="permission-dialog"
    >
      <div class="dialog-content">
        <div class="dialog-section">
          <div class="section-header">
            <div class="section-icon target-icon">
              <el-icon :size="18"><User /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">授权对象</span>
              <span class="title-desc">授权对象不可修改</span>
            </div>
          </div>
          <div class="section-body">
            <div class="readonly-target">
              <el-tag v-if="editForm.target_type === 'user'" size="small" type="success" effect="plain">用户</el-tag>
              <el-tag v-else size="small" effect="plain">角色</el-tag>
              <span class="readonly-target-name">{{ editForm.target_name }}</span>
            </div>
          </div>
        </div>

        <div class="dialog-section">
          <div class="section-header">
            <div class="section-icon perm-icon">
              <el-icon :size="18"><Lock /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">权限配置</span>
              <span class="title-desc">修改权限类型</span>
            </div>
          </div>
          <div class="section-body">
            <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-position="top" class="dialog-form">
              <el-form-item label="权限类型" prop="permission_type">
                <el-select v-model="editForm.permission_type" placeholder="请选择权限类型" style="width: 100%">
                  <el-option
                    v-for="(label, key) in permissionLabels"
                    :key="key"
                    :label="label"
                    :value="key"
                  >
                    <div class="perm-select-option">
                      <div class="perm-badge-mini" :class="`permission-${key}`">
                        <el-icon :size="12"><component :is="getPermissionIcon(key)" /></el-icon>
                      </div>
                      <div class="perm-select-info">
                        <span class="perm-select-label">{{ label }}</span>
                        <span class="perm-select-desc">{{ permissionDescs[key] }}</span>
                      </div>
                    </div>
                  </el-option>
                </el-select>
                <div v-if="editForm.permission_type && permissionHierarchy[editForm.permission_type]?.length" class="perm-hierarchy-hint">
                  <el-icon :size="14"><Check /></el-icon>
                  <span>自动包含：{{ permissionHierarchy[editForm.permission_type].map(k => permissionLabels[k]).join('、') }}权限</span>
                </div>
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="editDialogVisible = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleSubmitEdit" :loading="submitting" size="large">
            <el-icon><Check /></el-icon>
            确认修改
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import {
  ArrowLeft, Plus, Delete, Lock, Refresh, Search, Connection, User, UserFilled,
  View, Download, Edit, Setting, Check, Clock
} from '@element-plus/icons-vue'
import { datasourceAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import { roleAdminAPI, userAdminAPI } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import type { DatasourcePermission } from '@/types'
import type { Role, User as AdminUser } from '@/api/admin'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const datasourceId = Number(route.params.id)
const datasourceName = ref('')
const datasourceDomainId = ref<number | null>(null)
const permissions = ref<DatasourcePermission[]>([])
const roles = ref<Role[]>([])
const users = ref<AdminUser[]>([])
const loading = ref(false)
const submitting = ref(false)
const addDialogVisible = ref(false)
const editDialogVisible = ref(false)
const addFormRef = ref<FormInstance>()
const editFormRef = ref<FormInstance>()
const searchQuery = ref('')
const filterType = ref<string | null>(null)
const filterTarget = ref<string | null>(null)

const addForm = ref({
  target_type: 'user' as 'user' | 'role',
  user_id: null as number | null,
  role_id: null as number | null,
  permission_type: '',
})

const editForm = ref({
  id: 0,
  target_type: 'user' as 'user' | 'role',
  target_name: '',
  permission_type: '',
})

const validateTarget = (_rule: any, _value: any, callback: any) => {
  if (addForm.value.target_type === 'user' && !addForm.value.user_id) {
    callback(new Error('请选择用户'))
  } else if (addForm.value.target_type === 'role' && !addForm.value.role_id) {
    callback(new Error('请选择角色'))
  } else {
    callback()
  }
}

const addRules: FormRules = {
  target_type: [{ required: true, message: '请选择授权方式', trigger: 'change' }],
  user_id: [{ validator: validateTarget, trigger: 'change' }],
  role_id: [{ validator: validateTarget, trigger: 'change' }],
  permission_type: [{ required: true, message: '请选择权限类型', trigger: 'change' }],
}

const editRules: FormRules = {
  permission_type: [{ required: true, message: '请选择权限类型', trigger: 'change' }],
}

const permissionLabels: Record<string, string> = {
  read: '读取',
  query: '查询',
  download: '下载',
  update: '更新',
  delete: '删除',
  manage: '管理',
}

const permissionDescs: Record<string, string> = {
  read: '查看数据源配置和元数据',
  query: '执行 SQL 查询（含读取）',
  download: '导出查询结果（含查询、读取）',
  update: '修改数据源配置（含下载、查询、读取）',
  delete: '删除数据源',
  manage: '管理权限和配置（含所有权限）',
}

const permissionHierarchy: Record<string, string[]> = {
  read: [],
  query: ['read'],
  download: ['query', 'read'],
  update: ['download', 'query', 'read'],
  delete: [],
  manage: ['update', 'download', 'query', 'read', 'delete'],
}

const uniqueUsersCount = computed(() => {
  const userIds = new Set(permissions.value.filter(p => p.user_id).map(p => p.user_id))
  return userIds.size
})

const uniqueRolesCount = computed(() => {
  const roleIds = new Set(permissions.value.filter(p => p.role_id).map(p => p.role_id))
  return roleIds.size
})

const filteredPermissions = computed(() => {
  return permissions.value.filter(p => {
    const targetName = p.user_id
      ? getUserName(p.user_id).toLowerCase()
      : getRoleName(p.role_id!).toLowerCase()
    const matchSearch = !searchQuery.value ||
      targetName.includes(searchQuery.value.toLowerCase()) ||
      p.permission_type.toLowerCase().includes(searchQuery.value.toLowerCase())
    const matchType = !filterType.value || p.permission_type === filterType.value
    const matchTarget = !filterTarget.value ||
      (filterTarget.value === 'user' && p.user_id) ||
      (filterTarget.value === 'role' && p.role_id)
    return matchSearch && matchType && matchTarget
  })
})

const getUserName = (userId: number) => {
  const user = users.value.find(u => u.id === userId)
  return user ? user.username : `用户 ${userId}`
}

const getRoleName = (roleId: number) => {
  const role = roles.value.find(r => r.id === roleId)
  return role ? role.name : `角色 ${roleId}`
}

const getRoleDisplayName = (role: string) => {
  const map: Record<string, string> = {
    system_admin: '系统管理员',
    admin: '管理员',
    domain_admin: '领域管理员',
    user: '普通用户',
  }
  return map[role] || role
}

const getRoleTagType = (role: string) => {
  const map: Record<string, string> = {
    system_admin: 'danger',
    admin: 'warning',
    domain_admin: '',
    user: 'info',
  }
  return map[role] || 'info'
}

const getPermissionIcon = (type: string) => {
  const iconMap: Record<string, any> = {
    read: View,
    query: Search,
    download: Download,
    update: Edit,
    delete: Delete,
    manage: Setting,
  }
  return iconMap[type] || Lock
}

const getPermissionLabel = (type: string) => {
  return permissionLabels[type] || type
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

const loadDatasource = async () => {
  try {
    const res = await datasourceAPI.get(datasourceId)
    datasourceName.value = res.data.name
    datasourceDomainId.value = res.data.domain_id
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载数据源失败')
    }
  }
}

const loadPermissions = async () => {
  loading.value = true
  try {
    const res = await datasourceAPI.getPermissions(datasourceId)
    const data = res.data
    if (Array.isArray(data)) {
      permissions.value = data
    } else if (data && Array.isArray((data as any).items)) {
      permissions.value = (data as any).items
    } else {
      permissions.value = []
    }
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载权限列表失败')
    }
  } finally {
    loading.value = false
  }
}

const loadRoles = async () => {
  try {
    const res = await roleAdminAPI.list()
    const allRoles = res.data?.items || []
    if (authStore.isSystemAdmin) {
      roles.value = allRoles
    } else {
      const userDomainId = authStore.currentDomainId
      roles.value = allRoles.filter((r: Role) => {
        if (r.is_system) return false
        if (r.domain_id == null || r.domain_id === undefined) return true
        return r.domain_id === userDomainId
      })
    }
  } catch (err: any) {
  }
}

const loadUsers = async () => {
  try {
    let domainId: number | undefined
    if (authStore.isSystemAdmin) {
      domainId = 0
    } else {
      domainId = datasourceDomainId.value || authStore.currentDomainId || undefined
    }
    const res = await userAdminAPI.listByDomain(domainId)
    users.value = res.data?.items || []
  } catch (err: any) {
  }
}

const handleTargetTypeChange = () => {
  addForm.value.user_id = null
  addForm.value.role_id = null
}

const handleAddPermission = () => {
  addForm.value = { target_type: 'user', user_id: null, role_id: null, permission_type: '' }
  addDialogVisible.value = true
}

const handleEditPermission = (row: DatasourcePermission) => {
  editForm.value = {
    id: row.id,
    target_type: row.user_id ? 'user' : 'role',
    target_name: row.user_id ? getUserName(row.user_id) : getRoleName(row.role_id!),
    permission_type: row.permission_type,
  }
  editDialogVisible.value = true
}

const handleSubmitEdit = async () => {
  if (!editFormRef.value) return
  await editFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      await datasourceAPI.updatePermission(datasourceId, editForm.value.id, {
        permission_type: editForm.value.permission_type,
      })
      ElMessage.success('权限修改成功')
      editDialogVisible.value = false
      await loadPermissions()
    } catch (err: any) {
      if (!isHandledError(err)) {
        ElMessage.error(err.message || '修改权限失败')
      }
    } finally {
      submitting.value = false
    }
  })
}

const handleSubmitAdd = async () => {
  if (!addFormRef.value) return
  await addFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const data: { role_id?: number; user_id?: number; permission_type: string } = {
        permission_type: addForm.value.permission_type,
      }
      if (addForm.value.target_type === 'user') {
        data.user_id = addForm.value.user_id!
      } else {
        data.role_id = addForm.value.role_id!
      }
      await datasourceAPI.grantPermission(datasourceId, data)
      ElMessage.success('权限添加成功')
      addDialogVisible.value = false
      await loadPermissions()
    } catch (err: any) {
      if (!isHandledError(err)) {
        ElMessage.error(err.message || '添加权限失败')
      }
    } finally {
      submitting.value = false
    }
  })
}

const handleDeletePermission = async (row: DatasourcePermission) => {
  const targetName = row.user_id ? getUserName(row.user_id) : getRoleName(row.role_id!)
  const targetType = row.user_id ? '用户' : '角色'
  try {
    await ElMessageBox.confirm(
      `确定要删除${targetType} "${targetName}" 的 "${getPermissionLabel(row.permission_type)}" 权限吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await datasourceAPI.revokePermission(datasourceId, row.id)
    ElMessage.success('权限已删除')
    await loadPermissions()
  } catch (err: any) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(err.message || '删除权限失败')
    }
  }
}

const handleBack = () => {
  router.push({ name: 'Datasources' })
}

onMounted(async () => {
  await loadDatasource()
  loadPermissions()
  loadRoles()
  loadUsers()
})
</script>

<style scoped>
.datasource-permission-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  height: 100%;
  min-height: 0;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.header-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.back-btn {
  font-weight: 500;
  color: var(--text-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--bg-secondary);
  transition: all var(--duration-normal) var(--ease-out);
}

.back-btn:hover {
  color: var(--accent-primary);
  border-color: var(--accent-primary);
  background: rgba(59, 130, 246, 0.04);
}

.header-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
}

.header-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.ds-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(99, 102, 241, 0.1));
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-primary);
  width: fit-content;
}

.header-stats {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-2) var(--space-4);
  background: var(--bg-secondary);
  border-radius: var(--radius-lg);
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--accent-primary);
  line-height: 1;
}

.stat-label {
  font-size: 12px;
  color: var(--text-muted);
}

.stat-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
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
  width: 260px;
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
  width: 150px;
}

.filter-option {
  display: flex;
  align-items: center;
  gap: 8px;
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

.target-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.target-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  color: white;
  box-shadow: 0 2px 6px rgba(59, 130, 246, 0.25);
}

.target-avatar.user-avatar {
  background: linear-gradient(135deg, #22c55e, #16a34a);
}

.target-avatar.role-avatar {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
}

.target-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.target-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.target-tag {
  flex-shrink: 0;
}

.target-id {
  font-size: 11px;
  color: var(--text-muted);
}

.permission-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 500;
}

.permission-badge.permission-read {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.permission-badge.permission-query {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

.permission-badge.permission-download {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.permission-badge.permission-update {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.permission-badge.permission-delete {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.permission-badge.permission-manage {
  background: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}

.permission-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.permission-includes {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.2;
}

.perm-badge-mini {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--radius-sm);
}

.perm-badge-mini.permission-read {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.perm-badge-mini.permission-query {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

.perm-badge-mini.permission-download {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.perm-badge-mini.permission-update {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.perm-badge-mini.permission-delete {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.perm-badge-mini.permission-manage {
  background: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}

.granted-by-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 13px;
}

.time-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 13px;
}

.delete-btn {
  gap: 4px;
}

.table-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-10);
  gap: var(--space-3);
}

.empty-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(99, 102, 241, 0.08));
  border-radius: 50%;
  color: var(--text-muted);
}

.empty-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.empty-hint {
  font-size: 0.875rem;
  color: var(--text-muted);
}

.dialog-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.dialog-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all var(--duration-normal) var(--ease-out);
}

.dialog-section:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.08);
}

.section-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.04), rgba(99, 102, 241, 0.04));
  border-bottom: 1px solid var(--border-default);
}

.section-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  color: white;
}

.section-icon.target-icon {
  background: linear-gradient(135deg, #22c55e, #16a34a);
  box-shadow: 0 2px 8px rgba(34, 197, 94, 0.3);
}

.section-icon.perm-icon {
  background: linear-gradient(135deg, #8b5cf6, #a855f7);
  box-shadow: 0 2px 8px rgba(139, 92, 246, 0.3);
}

.section-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.title-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.section-body {
  padding: var(--space-4);
}

.dialog-form :deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--text-primary);
}

.dialog-form :deep(.el-input__wrapper),
.dialog-form :deep(.el-select .el-input__wrapper) {
  background: var(--bg-card);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.dialog-form :deep(.el-input__wrapper:hover),
.dialog-form :deep(.el-select .el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

.dialog-form :deep(.el-input__wrapper.is-focus),
.dialog-form :deep(.el-select .el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.15);
}

.radio-label {
  display: flex;
  align-items: center;
  gap: 6px;
}

.readonly-target {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.readonly-target-name {
  font-weight: 500;
  color: var(--text-primary);
  font-size: 14px;
}

.edit-btn {
  gap: 4px;
}

.user-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-select-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: linear-gradient(135deg, #22c55e, #16a34a);
  border-radius: var(--radius-sm);
  color: white;
}

.user-select-name {
  font-weight: 500;
  color: var(--text-primary);
}

.user-select-id {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-muted);
}

.role-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

.role-select-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  border-radius: var(--radius-sm);
  color: white;
}

.role-select-name {
  font-weight: 500;
  color: var(--text-primary);
}

.role-select-id {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-muted);
}

.perm-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

.perm-select-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.perm-select-label {
  font-weight: 500;
  color: var(--text-primary);
}

.perm-select-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.perm-hierarchy-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 8px 12px;
  background: rgba(34, 197, 94, 0.06);
  border: 1px solid rgba(34, 197, 94, 0.2);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: #16a34a;
}

.perm-hierarchy-hint .el-icon {
  color: #22c55e;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
  padding-top: var(--space-3);
}

.dialog-footer :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  border: none;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.dialog-footer :deep(.el-button--primary:hover) {
  filter: brightness(1.05);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
}
</style>
