<template>
  <div class="profile-page">
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-icon stat-icon-primary">
          <el-icon :size="24"><User /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ profileForm.username }}</div>
          <div class="stat-label">用户名</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-info">
          <el-icon :size="24"><UserFilled /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ profileForm.real_name || '-' }}</div>
          <div class="stat-label">姓名</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-info">
          <el-icon :size="24"><Phone /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value stat-email">{{ profileForm.phone || '-' }}</div>
          <div class="stat-label">手机号</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-success">
          <el-icon :size="24"><Message /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value stat-email">{{ profileForm.email || '-' }}</div>
          <div class="stat-label">邮箱</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon stat-icon-warning">
          <el-icon :size="24"><Key /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ getRoleText(profileForm.role) }}</div>
          <div class="stat-label">角色</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon" :class="profileForm.is_active ? 'stat-icon-success' : 'stat-icon-danger'">
          <el-icon :size="24"><CircleCheck v-if="profileForm.is_active" /><CircleClose v-else /></el-icon>
        </div>
        <div class="stat-content">
          <div class="stat-value">{{ profileForm.is_active ? '激活' : '未激活' }}</div>
          <div class="stat-label">状态</div>
        </div>
      </div>
    </div>

    <div class="profile-content">
      <div class="profile-card">
        <div class="page-toolbar">
          <div class="toolbar-left">
            <span class="toolbar-title">个人信息</span>
          </div>
          <div class="toolbar-right">
            <el-button :icon="Refresh" @click="loadCurrentUser" class="refresh-btn">刷新</el-button>
          </div>
        </div>

        <div class="card-body">
          <el-form :model="profileForm" label-width="100px" class="profile-form">
            <el-form-item label="用户名">
              <el-input v-model="profileForm.username" disabled />
            </el-form-item>
            
            <el-form-item label="姓名">
              <el-input v-model="profileForm.real_name" placeholder="请输入姓名" clearable />
            </el-form-item>
            
            <el-form-item label="手机号">
              <el-input v-model="profileForm.phone" placeholder="请输入手机号" clearable />
            </el-form-item>
            
            <el-form-item label="邮箱">
              <el-input v-model="profileForm.email" placeholder="请输入邮箱" clearable />
            </el-form-item>
            
            <el-form-item label="角色">
              <el-tag :type="getRoleTagType(profileForm.role)" effect="light">
                {{ getRoleText(profileForm.role) }}
              </el-tag>
            </el-form-item>
            
            <el-form-item label="状态">
              <el-tag :type="profileForm.is_active ? 'success' : 'danger'" effect="light">
                {{ profileForm.is_active ? '激活' : '未激活' }}
              </el-tag>
            </el-form-item>
            
            <el-form-item>
              <el-button type="primary" @click="handleUpdateProfile" :loading="updateProfileLoading" class="submit-btn">
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <div class="password-card">
        <div class="page-toolbar">
          <div class="toolbar-left">
            <span class="toolbar-title">修改密码</span>
          </div>
        </div>

        <div class="card-body">
          <el-form :model="passwordForm" :rules="passwordRules" ref="passwordFormRef" label-width="120px" class="password-form">
            <el-form-item label="原密码" prop="oldPassword">
              <el-input 
                v-model="passwordForm.oldPassword" 
                type="password" 
                placeholder="请输入原密码" 
                show-password
                clearable
              />
            </el-form-item>
            
            <el-form-item label="新密码" prop="newPassword">
              <el-input 
                v-model="passwordForm.newPassword" 
                type="password" 
                placeholder="6-30位，需包含字母和数字" 
                maxlength="30"
                show-password
                clearable
              />
              <div class="password-rules-tip">
                <div v-for="rule in PASSWORD_RULES.rules" :key="rule">{{ rule }}</div>
              </div>
            </el-form-item>
            
            <el-form-item label="确认新密码" prop="confirmPassword">
              <el-input 
                v-model="passwordForm.confirmPassword" 
                type="password" 
                placeholder="请再次输入新密码" 
                show-password
                clearable
              />
            </el-form-item>
            
            <el-form-item>
              <el-button type="primary" @click="handleChangePassword" :loading="changePasswordLoading" class="submit-btn">
                修改密码
              </el-button>
              <el-button @click="handleResetPasswordForm" class="reset-btn">重置</el-button>
            </el-form-item>
          </el-form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import { User, UserFilled, Phone, Message, Key, CircleCheck, CircleClose, Refresh } from '@element-plus/icons-vue'
import { authAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import { encryptPassword, validatePassword, PASSWORD_RULES } from '@/utils/password'

const profileForm = reactive({
  username: '',
  real_name: '',
  phone: '',
  email: '',
  role: '',
  is_active: true,
})

const passwordForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const passwordFormRef = ref<FormInstance>()
const updateProfileLoading = ref(false)
const changePasswordLoading = ref(false)

const validateConfirmPassword = (_rule: any, value: any, callback: any) => {
  if (value === '') {
    callback(new Error('请再次输入新密码'))
  } else if (value !== passwordForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const passwordRules: FormRules = {
  oldPassword: [
    { required: true, message: '请输入原密码', trigger: 'blur' },
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: any, callback: any) => {
        if (!value) {
          callback(new Error('请输入新密码'))
        } else {
          const result = validatePassword(value)
          if (!result.valid) {
            callback(new Error(result.message))
          } else {
            callback()
          }
        }
      },
      trigger: 'blur',
    },
  ],
  confirmPassword: [
    { required: true, validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

const getRoleText = (role: string) => {
  switch (role) {
    case 'system_admin': return '系统管理员'
    case 'admin': return '管理员'
    case 'domain_admin': return '领域管理员'
    case 'user': return '普通用户'
    default: return role
  }
}

const getRoleTagType = (role: string) => {
  switch (role) {
    case 'system_admin': return 'danger'
    case 'admin': return 'warning'
    case 'domain_admin': return 'success'
    case 'user': return 'info'
    default: return 'info'
  }
}

const loadCurrentUser = async () => {
  try {
    const response = await authAPI.getCurrentUser()
    const user = response.data.user
    
    profileForm.username = user.username || ''
    profileForm.real_name = user.real_name || ''
    profileForm.phone = user.phone || ''
    profileForm.email = user.email || ''
    profileForm.role = user.role_codes?.[0] || ''
    profileForm.is_active = user.is_active ?? true
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error('加载用户信息失败：' + (error.message || '未知错误'))
    }
  }
}

const handleUpdateProfile = async () => {
  updateProfileLoading.value = true
  try {
    await authAPI.updateProfile({
      real_name: profileForm.real_name,
      phone: profileForm.phone,
      email: profileForm.email,
    })
    ElMessage.success('个人信息更新成功')
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error('更新失败：' + (error.message || '未知错误'))
    }
  } finally {
    updateProfileLoading.value = false
  }
}

const handleChangePassword = async () => {
  if (!passwordFormRef.value) return
  
  try {
    await passwordFormRef.value.validate()
  } catch {
    return
  }

  const passwordValidation = validatePassword(passwordForm.newPassword)
  if (!passwordValidation.valid) {
    ElMessage.error(passwordValidation.message)
    return
  }

  changePasswordLoading.value = true
  try {
    await authAPI.changePassword({
      old_password: encryptPassword(passwordForm.oldPassword),
      new_password: encryptPassword(passwordForm.newPassword),
    })
    
    ElMessage.success('密码修改成功')
    handleResetPasswordForm()
  } catch (error: any) {
    if (!isHandledError(error)) {
      const errorMsg = error.response?.data?.error || error.message || '未知错误'
      ElMessage.error('密码修改失败：' + errorMsg)
    }
  } finally {
    changePasswordLoading.value = false
  }
}

const handleResetPasswordForm = () => {
  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordFormRef.value?.resetFields()
}

onMounted(() => {
  loadCurrentUser()
})
</script>

<style scoped>
.profile-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: var(--space-4);
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  display: flex;
  align-items: center;
  gap: var(--space-4);
  box-shadow: var(--shadow-md);
  transition: all var(--duration-normal) var(--ease-out);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-icon-primary {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(37, 99, 235, 0.05));
  color: var(--accent-primary);
}

.stat-icon-success {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.05));
  color: var(--accent-success);
}

.stat-icon-warning {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.1), rgba(245, 158, 11, 0.05));
  color: var(--accent-warning);
}

.stat-icon-danger {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.1), rgba(239, 68, 68, 0.05));
  color: var(--accent-danger);
}

.stat-icon-info {
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(59, 130, 246, 0.05));
  color: var(--accent-primary);
}

.stat-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}

.stat-value {
  font-family: var(--font-display);
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-email {
  font-size: 1rem;
}

.stat-label {
  font-size: 0.8rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

.profile-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.profile-card,
.password-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.toolbar-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.refresh-btn {
  font-weight: 500;
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 16px;
}

.refresh-btn:hover {
  background: var(--bg-secondary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.card-body {
  padding: var(--space-6);
}

.profile-form,
.password-form {
  max-width: 500px;
}

.profile-form :deep(.el-input__wrapper),
.password-form :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.profile-form :deep(.el-input__wrapper:hover),
.password-form :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.profile-form :deep(.el-input__wrapper.is-focus),
.password-form :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.profile-form :deep(.el-input.is-disabled .el-input__wrapper),
.password-form :deep(.el-input.is-disabled .el-input__wrapper) {
  background: var(--bg-secondary);
  border-color: var(--border-subtle);
}

.profile-form :deep(.el-form-item__label),
.password-form :deep(.el-form-item__label) {
  color: var(--text-secondary);
  font-weight: 500;
}

.submit-btn {
  font-weight: 500;
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
  padding: 10px 24px;
}

.submit-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
}

.reset-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 10px 24px;
}

.reset-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.password-rules-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  line-height: 1.6;
}

@media (max-width: 1200px) {
  .stats-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .card-body {
    padding: var(--space-4);
  }

  .profile-form,
  .password-form {
    max-width: 100%;
  }
}
</style>
