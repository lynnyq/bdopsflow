<template>
  <div class="users-page">
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
        <el-button :icon="Plus" @click="handleOpenCreate" class="create-btn">
          创建用户
        </el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="filteredUsers" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" :minWidth="120" show-overflow-tooltip />
        <el-table-column prop="real_name" label="姓名" :minWidth="100" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.real_name || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="phone" label="手机号" :minWidth="130" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.phone || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="email" label="邮箱" :minWidth="200" show-overflow-tooltip />
        <el-table-column label="角色" width="200" align="center">
          <template #default="{ row }">
            <el-tag v-for="roleCode in (row.role_codes || [])" :key="roleCode" :type="getRoleTagType(roleCode)" effect="light" style="margin: 2px;">
              {{ getRoleLabel(roleCode) }}
            </el-tag>
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
        <el-table-column v-if="showActions" label="操作" width="220" fixed="right" align="center">
          <template #default="{ row }">
            <template v-if="canManageUser(row)">
              <el-button type="primary" link size="small" @click="handleEdit(row)">
                <el-icon><Edit /></el-icon> 编辑
              </el-button>
              <el-button type="warning" link size="small" @click="handleResetPassword(row)">
                <el-icon><Key /></el-icon> 重置密码
              </el-button>
            </template>
            <el-button v-if="authStore.isSystemAdmin" type="danger" link size="small" @click="handleDelete(row)">
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

    <el-dialog
      v-model="showCreateDialog"
      title="创建用户"
      width="520px"
      class="custom-dialog"
      :close-on-click-modal="false"
      @closed="handleDialogClosed('create')"
    >
      <el-form
        ref="formRef"
        :model="userForm"
        :rules="userRules"
        label-position="top"
        class="dialog-form"
        status-icon
      >
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="用户名" prop="username">
              <el-input
                v-model="userForm.username"
                placeholder="3-50位字母或数字"
                maxlength="50"
                show-word-limit
                clearable
              >
                <template #prefix>
                  <el-icon><UserIcon /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="姓名" prop="real_name">
              <el-input
                v-model="userForm.real_name"
                placeholder="请输入姓名"
                maxlength="50"
                clearable
              >
                <template #prefix>
                  <el-icon><UserFilled /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="手机号" prop="phone">
              <el-input
                v-model="userForm.phone"
                placeholder="请输入手机号"
                maxlength="20"
                clearable
              >
                <template #prefix>
                  <el-icon><Phone /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="邮箱" prop="email">
              <el-input
                v-model="userForm.email"
                placeholder="请输入邮箱地址"
                maxlength="100"
                clearable
              >
                <template #prefix>
                  <el-icon><Message /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="密码" prop="password">
          <el-input
            v-model="userForm.password"
            type="password"
            placeholder="6-30位，需包含字母和数字"
            maxlength="30"
            show-password
            clearable
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
          <div class="form-tip">
            <div v-for="rule in PASSWORD_RULES.rules" :key="rule">{{ rule }}</div>
          </div>
        </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="角色" prop="role_ids">
              <el-select
                v-model="userForm.role_ids"
                placeholder="请选择用户角色"
                class="full-width"
                multiple
              >
                <el-option
                  v-for="role in roles"
                  :key="role.id"
                  :label="role.name"
                  :value="role.id"
                >
                  <div class="role-option">
                    <span>{{ role.name }}</span>
                    <el-tag v-if="role.is_system" size="small" type="danger" effect="plain" class="role-tag">系统</el-tag>
                    <span class="role-desc">{{ role.description }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="所属领域" prop="domain_ids">
              <el-select
                v-model="userForm.domain_ids"
                placeholder="可选，不选则为全局用户"
                clearable
                multiple
                class="full-width"
              >
                <el-option
                  v-for="domain in domains"
                  :key="domain.id"
                  :label="domain.name"
                  :value="domain.id"
                >
                  <div class="domain-option">
                    <el-icon><Grid /></el-icon>
                    <span>{{ domain.name }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showCreateDialog = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleCreate" :loading="submitting" size="large">
            创建用户
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showEditDialog"
      title="编辑用户"
      width="520px"
      class="custom-dialog"
      :close-on-click-modal="false"
      @closed="handleDialogClosed('edit')"
    >
      <el-form
        ref="editFormRef"
        :model="userForm"
        :rules="editUserRules"
        label-position="top"
        class="dialog-form"
        status-icon
      >
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="用户名" prop="username">
              <el-input v-model="userForm.username" disabled />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="姓名" prop="real_name">
              <el-input v-model="userForm.real_name" placeholder="请输入姓名" clearable>
                <template #prefix>
                  <el-icon><UserFilled /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="手机号" prop="phone">
              <el-input v-model="userForm.phone" placeholder="请输入手机号" clearable>
                <template #prefix>
                  <el-icon><Phone /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="邮箱" prop="email">
              <el-input v-model="userForm.email" placeholder="请输入邮箱地址" clearable>
                <template #prefix>
                  <el-icon><Message /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="角色" prop="role_ids">
              <el-select v-model="userForm.role_ids" placeholder="请选择用户角色" class="full-width" multiple>
                <el-option
                  v-for="role in roles"
                  :key="role.id"
                  :label="role.name"
                  :value="role.id"
                >
                  <div class="role-option">
                    <span>{{ role.name }}</span>
                    <el-tag v-if="role.is_system" size="small" type="danger" effect="plain" class="role-tag">系统</el-tag>
                    <span class="role-desc">{{ role.description }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="状态">
              <el-switch
                v-model="userForm.is_active"
                active-text="启用"
                inactive-text="禁用"
                :active-value="true"
                :inactive-value="false"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="所属领域" prop="domain_ids">
              <el-select
                v-model="userForm.domain_ids"
                placeholder="可选，不选则为全局用户"
                clearable
                multiple
                class="full-width"
              >
                <el-option
                  v-for="domain in domains"
                  :key="domain.id"
                  :label="domain.name"
                  :value="domain.id"
                >
                  <div class="domain-option">
                    <el-icon><Grid /></el-icon>
                    <span>{{ domain.name }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="创建时间">
              <el-tag type="info">{{ formatDate(userForm.created_at) }}</el-tag>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showEditDialog = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleUpdate" :loading="submitting" size="large">
            保存修改
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showResetPasswordDialog"
      title="重置密码"
      width="480px"
      class="custom-dialog"
      :close-on-click-modal="false"
      @closed="handleDialogClosed('reset')"
    >
      <el-form
        ref="resetPasswordFormRef"
        :model="resetPasswordForm"
        :rules="resetPasswordRules"
        label-position="top"
        class="dialog-form"
        status-icon
      >
        <el-form-item label="用户">
          <el-input v-model="resetPasswordForm.username" disabled />
        </el-form-item>

        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="resetPasswordForm.newPassword"
            type="password"
            placeholder="6-30位，需包含字母和数字"
            maxlength="30"
            show-password
            clearable
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
          <div class="form-tip">
            <div v-for="rule in PASSWORD_RULES.rules" :key="rule">{{ rule }}</div>
          </div>
        </el-form-item>

        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="resetPasswordForm.confirmPassword"
            type="password"
            placeholder="请再次输入新密码"
            show-password
            clearable
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showResetPasswordDialog = false" size="large">取消</el-button>
          <el-button type="warning" @click="handleConfirmResetPassword" :loading="submitting" size="large">
            重置密码
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Edit, Delete, Document, Search, Refresh, Key,
  Message, Lock, UserFilled, Grid, Phone,
  User as UserIcon
} from '@element-plus/icons-vue'
import { userAdminAPI, roleAdminAPI, domainAdminAPI, type User, type Role, type Domain } from '@/api/admin'
import { adminAPI } from '@/api'
import { encryptPassword, validatePassword, PASSWORD_RULES } from '@/utils/password'
import { useAuthStore } from '@/stores/auth'
import { handleError } from '@/utils/error'

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

const showActions = computed(() => authStore.isSystemAdmin || authStore.isDomainAdmin || authStore.hasPermission('user', 'update'))

const filteredUsers = computed(() => {
  if (!searchQuery.value) return users.value
  const query = searchQuery.value.toLowerCase()
  return users.value.filter(u =>
    u.username.toLowerCase().includes(query) ||
    (u.real_name && u.real_name.toLowerCase().includes(query)) ||
    (u.phone && u.phone.includes(query)) ||
    u.email?.toLowerCase().includes(query)
  )
})

const userForm = ref({
  id: 0,
  username: '',
  real_name: '',
  phone: '',
  email: '',
  password: '',
  is_active: true,
  role_ids: [] as number[],
  domain_ids: [] as number[],
  created_at: '',
})

const resetPasswordForm = ref({
  userId: 0,
  username: '',
  newPassword: '',
  confirmPassword: '',
})

const validatePasswordStrength = (rule: any, value: any, callback: any) => {
  if (value === '') {
    callback(new Error('请输入密码'))
  } else {
    const result = validatePassword(value)
    if (!result.valid) {
      callback(new Error(result.message))
    } else {
      callback()
    }
  }
}

const validateConfirmPassword = (rule: any, value: any, callback: any) => {
  if (value === '') {
    callback(new Error('请再次输入密码'))
  } else if (value !== resetPasswordForm.value.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const userRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '用户名为3-50位字母、数字、下划线或空格', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_ ]+$/, message: '用户名只能包含字母、数字、下划线或空格', trigger: 'blur' },
  ],
  real_name: [
    { max: 50, message: '姓名不能超过50个字符', trigger: 'blur' },
  ],
  phone: [
    { max: 20, message: '手机号不能超过20个字符', trigger: 'blur' },
    { pattern: /^$|^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' },
  ],
  password: [
    { required: true, validator: validatePasswordStrength, trigger: 'blur' },
  ],
  role_ids: [
    { required: true, type: 'array', min: 1, message: '请选择用户角色', trigger: 'change' },
  ],
  domain_ids: [
    { type: 'array', message: '领域ID必须是数字', trigger: 'change' },
  ],
}

const editUserRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
  ],
  real_name: [
    { max: 50, message: '姓名不能超过50个字符', trigger: 'blur' },
  ],
  phone: [
    { max: 20, message: '手机号不能超过20个字符', trigger: 'blur' },
    { pattern: /^$|^1[3-9]\d{9}$/, message: '请输入正确的手机号', trigger: 'blur' },
  ],
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入正确的邮箱格式', trigger: 'blur' },
  ],
  role_ids: [
    { required: true, type: 'array', min: 1, message: '请选择用户角色', trigger: 'change' },
  ],
}

const resetPasswordRules = {
  newPassword: [
    { required: true, validator: validatePasswordStrength, trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

const formRef = ref()
const editFormRef = ref()
const resetPasswordFormRef = ref()

const handleOpenCreate = () => {
  userForm.value = {
    id: 0,
    username: '',
    real_name: '',
    phone: '',
    email: '',
    password: '',
    is_active: true,
    role_ids: [],
    domain_ids: [],
    created_at: '',
  }
  showCreateDialog.value = true
}

const handleDialogClosed = (type: 'create' | 'edit' | 'reset') => {
  if (type === 'create') {
    formRef.value?.resetFields()
  } else if (type === 'edit') {
    editFormRef.value?.resetFields()
  } else {
    resetPasswordFormRef.value?.resetFields()
  }
}

const canManageUser = (user: User): boolean => {
  if (authStore.isSystemAdmin) return true
  if (authStore.isDomainAdmin) return true
  return authStore.hasPermission('user', 'update')
}

const getRoleLabel = (role: string): string => {
  const found = roles.value.find(r => r.code === role)
  return found ? found.name : role
}

const loadUsers = async () => {
  loading.value = true
  try {
    const response = await userAdminAPI.list()
    users.value = response.data.items || []
  } catch (error) {
    handleError(error, '加载用户列表失败')
  } finally {
    loading.value = false
  }
}

const loadRoles = async () => {
  try {
    const response = await roleAdminAPI.list()
    const allRoles = response.data.items || []
    if (authStore.isSystemAdmin) {
      roles.value = allRoles
    } else {
      roles.value = allRoles.filter((r: any) => r.code !== 'system_admin')
    }
  } catch (error) {
  }
}

const loadDomains = async () => {
  try {
    const response = await domainAdminAPI.list()
    const allDomains = response.data.items || []
    if (authStore.isSystemAdmin) {
      domains.value = allDomains
    } else {
      const userDomainIds = authStore.domains.map((d: any) => d.domain_id)
      domains.value = allDomains.filter((d: any) => userDomainIds.includes(d.id))
    }
  } catch (error) {
  }
}

const handleCreate = async () => {
  const form = formRef.value
  if (!form) return

  form.validate(async (valid) => {
    if (!valid) return

    const passwordValidation = validatePassword(userForm.value.password)
    if (!passwordValidation.valid) {
      ElMessage.warning(passwordValidation.message)
      return
    }

    submitting.value = true
    try {
      await userAdminAPI.create({
        username: userForm.value.username,
        real_name: userForm.value.real_name,
        phone: userForm.value.phone,
        email: userForm.value.email,
        password: encryptPassword(userForm.value.password),
        role_ids: userForm.value.role_ids,
        domain_ids: userForm.value.domain_ids,
      })
      ElMessage.success('创建用户成功')
      showCreateDialog.value = false
      loadUsers()
    } catch (error: any) {
      handleError(error, '创建用户失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleEdit = (row: User) => {
  userForm.value = {
    id: row.id,
    username: row.username,
    real_name: row.real_name || '',
    phone: row.phone || '',
    email: row.email,
    password: '',
    is_active: row.is_active,
    role_ids: row.role_ids || [],
    domain_ids: row.domain_ids || [],
    created_at: row.created_at,
  }
  showEditDialog.value = true
}

const handleUpdate = async () => {
  const form = editFormRef.value
  if (!form) return

  form.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      await userAdminAPI.update(userForm.value.id, {
        username: userForm.value.username,
        real_name: userForm.value.real_name,
        phone: userForm.value.phone,
        email: userForm.value.email,
        is_active: userForm.value.is_active,
        role_ids: userForm.value.role_ids,
        domain_ids: userForm.value.domain_ids,
      })
      ElMessage.success('更新用户成功')
      showEditDialog.value = false
      loadUsers()
    } catch (error: any) {
      handleError(error, '更新用户失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleDelete = async (row: User) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户 "${row.username}" 吗？此操作不可恢复。`,
      '删除确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger',
      }
    )
    await userAdminAPI.delete(row.id)
    ElMessage.success('删除用户成功')
    loadUsers()
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') {
      handleError(error, '删除用户失败')
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

  form.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      await adminAPI.resetUserPassword(resetPasswordForm.value.userId, {
        new_password: encryptPassword(resetPasswordForm.value.newPassword),
      })
      ElMessage.success('密码重置成功')
      showResetPasswordDialog.value = false
    } catch (error: any) {
      handleError(error, '密码重置失败')
    } finally {
      submitting.value = false
    }
  })
}

const getRoleTagType = (role: string): 'primary' | 'success' | 'info' | 'warning' | 'danger' => {
  switch (role) {
    case 'system_admin':
      return 'danger'
    case 'domain_admin':
      return 'warning'
    default:
      return 'info'
  }
}

const formatDate = (date: string | undefined) => {
  if (!date) return '-'
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

.form-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  line-height: 1.4;
}

.form-alert {
  margin-top: var(--space-3);
}

.role-option {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.role-option .el-icon {
  color: var(--text-muted);
}

.role-option .role-desc {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-muted);
}

.role-option .role-tag {
  margin-left: 4px;
}

.domain-option {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.full-width {
  width: 100%;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
  padding: var(--space-3) 0 0;
}
</style>
