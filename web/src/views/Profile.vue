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
          <el-form :model="profileForm" label-width="auto" label-position="top" class="profile-form">
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
              <el-tag :type="getRoleTagType(profileForm.role)" effect="plain" class="info-tag">
                {{ getRoleText(profileForm.role) }}
              </el-tag>
            </el-form-item>

            <el-form-item label="状态">
              <el-tag :type="profileForm.is_active ? 'success' : 'danger'" effect="plain" class="info-tag">
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
          <el-form :model="passwordForm" :rules="passwordRules" ref="passwordFormRef" label-width="auto" label-position="top" class="password-form">
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

      <div class="api-token-card">
        <div class="page-toolbar">
          <div class="toolbar-left">
            <span class="toolbar-title">API Token</span>
          </div>
          <div class="toolbar-right">
            <el-button :icon="Refresh" @click="loadApiTokenInfo" class="refresh-btn">刷新</el-button>
          </div>
        </div>

        <div class="card-body">
          <div v-if="!apiTokenInfo.has_token" class="token-empty">
            <p class="token-empty-text">尚未创建 API Token</p>
            <el-button type="primary" @click="handleGenerateToken" :loading="generateTokenLoading" class="submit-btn">
              生成 Token
            </el-button>
          </div>

          <div v-else class="token-info">
            <el-form label-width="auto" label-position="top" class="profile-form">
              <el-form-item label="Token">
                <div class="token-display">
                  <el-input
                    :model-value="tokenRevealed ? tokenPlainText : 'bdf_••••••••••••••••••••••••••••••••'"
                    readonly
                    class="token-input"
                  />
                  <el-button @click="handleRevealToken" :loading="revealTokenLoading" class="token-action-btn">
                    {{ tokenRevealed ? '隐藏' : '查看' }}
                  </el-button>
                  <el-button @click="handleCopyToken" :disabled="!tokenRevealed" class="token-action-btn">
                    复制
                  </el-button>
                </div>
              </el-form-item>

              <el-form-item label="Token 前缀">
                <span class="token-meta">{{ apiTokenInfo.token_prefix }}...</span>
              </el-form-item>

              <el-form-item label="创建时间">
                <span class="token-meta">{{ apiTokenInfo.created_at || '-' }}</span>
              </el-form-item>

              <el-form-item label="最后使用">
                <span class="token-meta">{{ apiTokenInfo.last_used_at || '从未使用' }}</span>
              </el-form-item>

              <el-form-item>
                <el-button type="primary" @click="handleGenerateToken" :loading="generateTokenLoading" class="submit-btn">
                  重新生成
                </el-button>
                <el-button type="danger" @click="handleRevokeToken" :loading="revokeTokenLoading" class="reset-btn">
                  吊销 Token
                </el-button>
              </el-form-item>
            </el-form>

            <div class="token-tips">
              <p>- Token 权限与当前用户一致</p>
              <p>- 重新生成会使旧 Token 立即失效</p>
              <p>- 请妥善保管 Token，避免泄露</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
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
    const roleCodes = response.data.role_codes
    
    profileForm.username = user.username || ''
    profileForm.real_name = user.real_name || ''
    profileForm.phone = user.phone || ''
    profileForm.email = user.email || ''
    profileForm.role = roleCodes?.[0] || ''
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

// API Token 相关
const apiTokenInfo = reactive({
  has_token: false,
  token_prefix: '',
  last_used_at: '',
  created_at: '',
})
const tokenPlainText = ref('')
const tokenRevealed = ref(false)
const generateTokenLoading = ref(false)
const revealTokenLoading = ref(false)
const revokeTokenLoading = ref(false)

const loadApiTokenInfo = async () => {
  try {
    const response = await authAPI.apiToken.getInfo()
    const data = response.data
    apiTokenInfo.has_token = data.has_token
    apiTokenInfo.token_prefix = data.token_prefix || ''
    apiTokenInfo.last_used_at = data.last_used_at || ''
    apiTokenInfo.created_at = data.created_at || ''
    tokenRevealed.value = false
    tokenPlainText.value = ''
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error('加载 API Token 信息失败：' + (error.message || '未知错误'))
    }
  }
}

const handleGenerateToken = async () => {
  try {
    await ElMessageBox.confirm(
      apiTokenInfo.has_token
        ? '重新生成会使旧 Token 立即失效，确定继续？'
        : '确定生成 API Token？生成后请妥善保管。',
      '确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return
  }

  generateTokenLoading.value = true
  try {
    const response = await authAPI.apiToken.generate()
    const data = response.data
    tokenPlainText.value = data.token
    tokenRevealed.value = true
    apiTokenInfo.has_token = true
    apiTokenInfo.token_prefix = data.token_prefix
    apiTokenInfo.created_at = data.created_at
    apiTokenInfo.last_used_at = ''
    ElMessage.success('API Token 生成成功')
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error('生成 API Token 失败：' + (error.message || '未知错误'))
    }
  } finally {
    generateTokenLoading.value = false
  }
}

const handleRevealToken = async () => {
  if (tokenRevealed.value) {
    tokenRevealed.value = false
    return
  }

  revealTokenLoading.value = true
  try {
    const response = await authAPI.apiToken.reveal()
    tokenPlainText.value = response.data.token
    tokenRevealed.value = true
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error('查看 Token 失败：' + (error.message || '未知错误'))
    }
  } finally {
    revealTokenLoading.value = false
  }
}

const handleCopyToken = async () => {
  if (!tokenPlainText.value) return
  try {
    await navigator.clipboard.writeText(tokenPlainText.value)
    ElMessage.success('Token 已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

const handleRevokeToken = async () => {
  try {
    await ElMessageBox.confirm(
      '吊销后使用该 Token 的所有请求将立即失效，确定继续？',
      '确认吊销',
      {
        confirmButtonText: '确定吊销',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
  } catch {
    return
  }

  revokeTokenLoading.value = true
  try {
    await authAPI.apiToken.revoke()
    apiTokenInfo.has_token = false
    apiTokenInfo.token_prefix = ''
    apiTokenInfo.last_used_at = ''
    apiTokenInfo.created_at = ''
    tokenPlainText.value = ''
    tokenRevealed.value = false
    ElMessage.success('API Token 已吊销')
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error('吊销 Token 失败：' + (error.message || '未知错误'))
    }
  } finally {
    revokeTokenLoading.value = false
  }
}

onMounted(() => {
  loadCurrentUser()
  loadApiTokenInfo()
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
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--space-4);
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-4);
  display: flex;
  align-items: center;
  gap: var(--space-3);
  box-shadow: var(--shadow-md);
  transition: all var(--duration-normal) var(--ease-out);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

.stat-icon {
  width: 48px;
  height: 48px;
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
  overflow: hidden;
}

.stat-value {
  font-family: var(--font-display);
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-email {
  font-size: 0.9rem;
}

.stat-label {
  font-size: 0.75rem;
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
.password-card,
.api-token-card {
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
  max-width: 600px;
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

.token-empty {
  text-align: center;
  padding: var(--space-6) 0;
}

.token-empty-text {
  color: var(--text-muted);
  margin-bottom: var(--space-4);
  font-size: 0.9rem;
}

.token-display {
  display: flex;
  gap: var(--space-2);
  width: 100%;
}

.token-input {
  flex: 1;
}

.token-input :deep(.el-input__wrapper) {
  font-family: var(--font-display, monospace);
  font-size: 0.85rem;
  letter-spacing: 0.02em;
}

.token-action-btn {
  font-weight: 500;
  flex-shrink: 0;
}

.token-meta {
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.token-tips {
  margin-top: var(--space-4);
  padding: var(--space-3) var(--space-4);
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

.token-tips p {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.8;
  margin: 0;
}

.info-tag {
  height: 32px;
  padding: 0 12px;
  font-size: 0.875rem;
  border-radius: var(--radius-md);
}

@media (max-width: 1280px) {
  .stats-grid {
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  }

  .stat-icon {
    width: 42px;
    height: 42px;
  }

  .stat-value {
    font-size: 1.05rem;
  }

  .card-body {
    padding: var(--space-5);
  }
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(3, 1fr);
  }

  .stat-card {
    padding: var(--space-3);
    gap: var(--space-2);
  }

  .stat-icon {
    width: 38px;
    height: 38px;
  }

  .stat-icon :deep(.el-icon) {
    --font-size: 18px;
  }

  .stat-value {
    font-size: 0.95rem;
  }

  .profile-form,
  .password-form {
    max-width: 100%;
  }
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .card-body {
    padding: var(--space-4);
  }
}
</style>
