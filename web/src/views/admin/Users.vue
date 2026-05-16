<template>
  <div class="users-page">
    <!-- Page Toolbar -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索用户..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadUsers" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Plus" @click="showCreateDialog = true" class="create-btn">
          创建用户
        </el-button>
      </div>
    </div>

    <!-- Table -->
    <div class="table-wrapper">
      <el-table :data="filteredUsers" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="email" label="邮箱" :minWidth="200" show-overflow-tooltip />
        <el-table-column prop="domain_id" label="领域" width="120" align="center">
          <template #default="{ row }">
            <el-tag type="info" effect="light">{{ getDomainName(row.domain_id) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="role" label="角色" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="getRoleTagType(row.role)" effect="light">{{ getRoleLabel(row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_active" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'danger'" effect="light">
              {{ row.is_active ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最后登录" width="180">
          <template #default="{ row }">
            {{ row.last_login_at ? formatDate(row.last_login_at) : '从未登录' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right" align="center">
          <template #default="{ row }">
            <template v-if="canManageUser(row)">
              <el-button type="primary" link size="small" @click="handleEdit(row)">
                <el-icon><Edit /></el-icon> 编辑
              </el-button>
              <el-button type="warning" link size="small" @click="handleResetPassword(row)">
                <el-icon><Key /></el-icon> 重置密码
              </el-button>
            </template>
            <el-button v-if="isSystemAdmin" type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无用户数据</p>
          </div>
        </template>
      </el-table>
    </div>

    <el-dialog v-model="showCreateDialog" title="创建用户" width="500px" class="custom-dialog">
      <el-form :model="userForm" :rules="userRules" ref="formRef" label-width="100px" class="dialog-form">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="userForm.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="userForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="userForm.password" type="password" placeholder="请输入密码" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showCreateDialog = false">取消</el-button>
          <el-button type="primary" @click="handleCreate" :loading="submitting">创建</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="showEditDialog" title="编辑用户" width="500px" class="custom-dialog">
      <el-form :model="userForm" :rules="userRules" ref="editFormRef" label-width="100px" class="dialog-form">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="userForm.username" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="userForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="userForm.is_active" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="userForm.role" placeholder="请选择角色">
            <el-option label="系统管理员" value="system_admin" />
            <el-option label="领域管理员" value="domain_admin" />
            <el-option label="普通用户" value="user" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showEditDialog = false">取消</el-button>
          <el-button type="primary" @click="handleUpdate" :loading="submitting">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="showResetPasswordDialog" title="重置密码" width="500px" class="custom-dialog">
      <el-form :model="resetPasswordForm" :rules="resetPasswordRules" ref="resetPasswordFormRef" label-width="120px" class="dialog-form">
        <el-form-item label="用户">
          <el-input v-model="resetPasswordForm.username" disabled />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input 
            v-model="resetPasswordForm.newPassword" 
            type="password" 
            placeholder="请输入新密码（至少6位）" 
            show-password
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input 
            v-model="resetPasswordForm.confirmPassword" 
            type="password" 
            placeholder="请再次输入新密码" 
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showResetPasswordDialog = false">取消</el-button>
          <el-button type="warning" @click="handleConfirmResetPassword" :loading="submitting">重置密码</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Document, Search, Refresh, Key } from '@element-plus/icons-vue'
import { userAdminAPI, roleAdminAPI, domainAdminAPI, type User, type Role, type Domain } from '@/api/admin'
import { adminAPI } from '@/api'
import { passwordUtils, validatePassword } from '@/utils/password'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const users = ref<User[]>([])
const roles = ref<Role[]>([])
const domains = ref<Domain[]>([])
const loading = ref(false)
const submitting = ref(false)
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showResetPasswordDialog = ref(false)
const searchQuery = ref('')

const isSystemAdmin = computed(() => authStore.user?.role === 'system_admin')
const isDomainAdmin = computed(() => authStore.user?.role === 'domain_admin')
const currentDomainId = computed(() => authStore.user?.domain_id)

const filteredUsers = computed(() => {
  if (!searchQuery.value) return users.value
  const query = searchQuery.value.toLowerCase()
  return users.value.filter(u => 
    u.username.toLowerCase().includes(query) || 
    u.email?.toLowerCase().includes(query)
  )
})

const userForm = ref({
  id: 0,
  username: '',
  email: '',
  password: '',
  is_active: true,
  role: 'user',
})

const resetPasswordForm = ref({
  userId: 0,
  username: '',
  newPassword: '',
  confirmPassword: '',
})

const userRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' },
  ],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

const validateConfirmPassword = (rule: any, value: any, callback: any) => {
  if (value === '') {
    callback(new Error('请再次输入新密码'))
  } else if (value !== resetPasswordForm.value.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const resetPasswordRules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码长度至少为6位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

const formRef = ref()
const editFormRef = ref()
const resetPasswordFormRef = ref()

const canManageUser = (user: User): boolean => {
  if (isSystemAdmin.value) {
    return true
  }
  if (isDomainAdmin.value && currentDomainId.value) {
    return user.domain_id === currentDomainId.value && user.role !== 'system_admin'
  }
  return false
}

const getDomainName = (domainId: number | null): string => {
  if (!domainId) return '-'
  const domain = domains.value.find(d => d.id === domainId)
  return domain?.name || `领域${domainId}`
}

const getRoleLabel = (role: string): string => {
  switch (role) {
    case 'system_admin':
      return '系统管理员'
    case 'domain_admin':
      return '领域管理员'
    case 'user':
      return '普通用户'
    default:
      return role
  }
}

const loadUsers = async () => {
  loading.value = true
  try {
    const response = await userAdminAPI.list()
    let allUsers = response.data.items || []
    if (isDomainAdmin.value && currentDomainId.value) {
      allUsers = allUsers.filter(u => u.domain_id === currentDomainId.value)
    }
    users.value = allUsers
  } catch (error) {
    ElMessage.error('加载用户列表失败')
  } finally {
    loading.value = false
  }
}

const loadRoles = async () => {
  try {
    const response = await roleAdminAPI.list()
    roles.value = response.data.items || []
  } catch (error) {
    console.error('加载角色列表失败', error)
  }
}

const loadDomains = async () => {
  try {
    const response = await domainAdminAPI.list()
    domains.value = response.data.items || []
  } catch (error) {
    console.error('加载领域列表失败', error)
  }
}

const handleCreate = async () => {
  const form = formRef.value
  if (!form) return

  await form.validate(async (valid) => {
    if (valid) {
      submitting.value = true
      try {
        await userAdminAPI.create({
          username: userForm.value.username,
          email: userForm.value.email,
          password: userForm.value.password,
        })
        ElMessage.success('创建用户成功')
        showCreateDialog.value = false
        form.resetFields()
        loadUsers()
      } catch (error) {
        ElMessage.error('创建用户失败')
      } finally {
        submitting.value = false
      }
    }
  })
}

const handleEdit = (row: User) => {
  userForm.value = {
    id: row.id,
    username: row.username,
    email: row.email,
    password: '',
    is_active: row.is_active,
    role: row.role,
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
        await userAdminAPI.update(userForm.value.id, {
          username: userForm.value.username,
          email: userForm.value.email,
          role: userForm.value.role,
          is_active: userForm.value.is_active,
        })
        ElMessage.success('更新用户成功')
        showEditDialog.value = false
        loadUsers()
      } catch (error: any) {
        const errorMsg = error.response?.data?.error || error.message || '未知错误'
        ElMessage.error('更新用户失败：' + errorMsg)
      } finally {
        submitting.value = false
      }
    }
  })
}

const handleDelete = async (row: User) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 "${row.username}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    await userAdminAPI.delete(row.id)
    ElMessage.success('删除用户成功')
    loadUsers()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除用户失败')
    }
  }
}

const handleResetPassword = (row: User) => {
  resetPasswordForm.value = {
    userId: row.id,
    username: row.username,
    newPassword: '',
    confirmPassword: '',
  }
  showResetPasswordDialog.value = true
}

const handleConfirmResetPassword = async () => {
  const form = resetPasswordFormRef.value
  if (!form) return

  await form.validate(async (valid) => {
    if (valid) {
      const passwordValidation = validatePassword(resetPasswordForm.value.newPassword)
      if (!passwordValidation.valid) {
        ElMessage.error(passwordValidation.message)
        return
      }

      submitting.value = true
      try {
        await adminAPI.resetUserPassword(resetPasswordForm.value.userId, {
          new_password: passwordUtils.encodePassword(resetPasswordForm.value.newPassword),
        })
        ElMessage.success('密码重置成功')
        showResetPasswordDialog.value = false
        form.resetFields()
      } catch (error: any) {
        const errorMsg = error.response?.data?.error || error.message || '未知错误'
        ElMessage.error('密码重置失败：' + errorMsg)
      } finally {
        submitting.value = false
      }
    }
  })
}

const getRoleTagType = (role: string) => {
  switch (role) {
    case 'system_admin':
      return 'danger'
    case 'domain_admin':
      return 'warning'
    default:
      return ''
  }
}

const formatDate = (date: string) => {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
}

onMounted(() => {
  loadUsers()
  loadRoles()
  loadDomains()
})
</script>

<style scoped>
.users-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.users-page::-webkit-scrollbar {
  width: 8px;
}

.users-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.users-page::-webkit-scrollbar-track {
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
