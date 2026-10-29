<!--
  用户个人中心组件
  功能：展示和修改当前登录用户个人信息、修改密码、查看/生成 API Token
  主要区域：个人信息展示卡片、个人信息编辑表单、修改密码表单、API Token 管理卡片
-->
<template>
  <!-- === .profile-page 区块 === -->
  <div class="profile-page">
    <!-- === 个人信息概览卡片（6 项快速展示：用户名/姓名/手机/邮箱/角色/状态） === -->
    <div class="stats-grid">
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-primary">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><User /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ profileForm.username }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">用户名</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-info">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><UserFilled /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ profileForm.real_name || '-' }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">姓名</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-info">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><Phone /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value stat-email">{{ profileForm.phone || '-' }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">手机号</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-success">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><Message /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value stat-email">{{ profileForm.email || '-' }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">邮箱</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-warning">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><Key /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ getRoleText(profileForm.role) }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">角色</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon" :class="profileForm.is_active ? 'stat-icon-success' : 'stat-icon-danger'">
          <!-- === 条件渲染区块 === -->
          <el-icon :size="24"><CircleCheck v-if="profileForm.is_active" /><CircleClose v-else /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ profileForm.is_active ? '激活' : '未激活' }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">状态</div>
        </div>
      </div>
    </div>

    <!-- === 个人信息编辑卡片（可修改姓名/手机/邮箱，用户名/角色/状态只读展示） === -->
    <div class="profile-content">
      <!-- === .profile-card 区块 === -->
      <div class="profile-card">
        <!-- === 页面工具栏 === -->
        <div class="page-toolbar">
          <!-- === .toolbar-left 区块 === -->
          <div class="toolbar-left">
            <span class="toolbar-title">个人信息</span>
          </div>
          <!-- === .toolbar-right 区块 === -->
          <div class="toolbar-right">
            <!-- === 按钮组件 === -->
            <el-button :icon="Refresh" @click="loadCurrentUser" class="refresh-btn">刷新</el-button>
          </div>
        </div>

        <!-- === .card-body 区块 === -->
        <div class="card-body">
          <!-- === 表单组件 === -->
          <el-form :model="profileForm" label-width="auto" label-position="top" class="profile-form">
            <!-- === 表单组件 === -->
            <el-form-item label="用户名">
              <!-- === 输入框组件 === -->
              <el-input v-model="profileForm.username" disabled />
            </el-form-item>
            
            <!-- === 表单组件 === -->
            <el-form-item label="姓名">
              <!-- === 输入框组件 === -->
              <el-input v-model="profileForm.real_name" placeholder="请输入姓名" clearable />
            </el-form-item>
            
            <!-- === 表单组件 === -->
            <el-form-item label="手机号">
              <!-- === 输入框组件 === -->
              <el-input v-model="profileForm.phone" placeholder="请输入手机号" clearable />
            </el-form-item>
            
            <!-- === 表单组件 === -->
            <el-form-item label="邮箱">
              <!-- === 输入框组件 === -->
              <el-input v-model="profileForm.email" placeholder="请输入邮箱" clearable />
            </el-form-item>
            
            <!-- === 表单组件 === -->
            <el-form-item label="角色">
              <!-- === 标签组件 === -->
              <el-tag :type="getRoleTagType(profileForm.role)" effect="plain" class="info-tag">
                {{ getRoleText(profileForm.role) }}
              </el-tag>
            </el-form-item>

            <!-- === 表单组件 === -->
            <el-form-item label="状态">
              <!-- === 标签组件 === -->
              <el-tag :type="profileForm.is_active ? 'success' : 'danger'" effect="plain" class="info-tag">
                {{ profileForm.is_active ? '激活' : '未激活' }}
              </el-tag>
            </el-form-item>
            
            <!-- === 表单组件 === -->
            <el-form-item>
              <!-- === 按钮组件 === -->
              <el-button type="primary" @click="handleUpdateProfile" :loading="updateProfileLoading" class="submit-btn">
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- === 修改密码卡片（需输入原密码 + 符合密码规则的新密码） === -->
      <div class="password-card">
        <!-- === 页面工具栏 === -->
        <div class="page-toolbar">
          <!-- === .toolbar-left 区块 === -->
          <div class="toolbar-left">
            <span class="toolbar-title">修改密码</span>
          </div>
        </div>

        <!-- === .card-body 区块 === -->
        <div class="card-body">
          <!-- === 表单组件 === -->
          <el-form :model="passwordForm" :rules="passwordRules" ref="passwordFormRef" label-width="auto" label-position="top" class="password-form">
            <!-- === 表单组件 === -->
            <el-form-item label="原密码" prop="oldPassword">
              <!-- === 输入框组件 === -->
              <el-input
                v-model="passwordForm.oldPassword"
                type="password"
                placeholder="请输入原密码"
                show-password
                clearable
              />
            </el-form-item>

            <!-- === 表单组件 === -->
            <el-form-item label="新密码" prop="newPassword">
              <!-- === 输入框组件 === -->
              <el-input
                v-model="passwordForm.newPassword"
                type="password"
                placeholder="6-30位，需包含字母和数字"
                maxlength="30"
                show-password
                clearable
              />
              <!-- === .password-rules-tip 区块 === -->
              <div class="password-rules-tip">
                <!-- === v-for 渲染区块 === -->
                <div v-for="rule in PASSWORD_RULES.rules" :key="rule">{{ rule }}</div>
              </div>
            </el-form-item>

            <!-- === 表单组件 === -->
            <el-form-item label="确认新密码" prop="confirmPassword">
              <!-- === 输入框组件 === -->
              <el-input
                v-model="passwordForm.confirmPassword"
                type="password"
                placeholder="请再次输入新密码"
                show-password
                clearable
              />
            </el-form-item>

            <!-- === 表单组件 === -->
            <el-form-item>
              <!-- === 按钮组件 === -->
              <el-button type="primary" @click="handleChangePassword" :loading="changePasswordLoading" class="submit-btn">
                修改密码
              </el-button>
              <!-- === 按钮组件 === -->
              <el-button @click="handleResetPasswordForm" class="reset-btn">重置</el-button>
            </el-form-item>
          </el-form>
        </div>
      </div>

      <!-- === API Token 管理卡片（生成/查看明文/复制/吊销） === -->
      <div class="api-token-card">
        <!-- === 页面工具栏 === -->
        <div class="page-toolbar">
          <!-- === .toolbar-left 区块 === -->
          <div class="toolbar-left">
            <span class="toolbar-title">API Token</span>
          </div>
          <!-- === .toolbar-right 区块 === -->
          <div class="toolbar-right">
            <!-- === 按钮组件 === -->
            <el-button :icon="Refresh" @click="loadApiTokenInfo" class="refresh-btn">刷新</el-button>
          </div>
        </div>

        <!-- === .card-body 区块 === -->
        <div class="card-body">
          <!-- === 条件渲染区块 === -->
          <div v-if="!apiTokenInfo.has_token" class="token-empty">
            <p class="token-empty-text">尚未创建 API Token</p>
            <!-- === 按钮组件 === -->
            <el-button type="primary" @click="handleGenerateToken" :loading="generateTokenLoading" class="submit-btn">
              生成 Token
            </el-button>
          </div>

          <!-- === else 分支区块 === -->
          <div v-else class="token-info">
            <!-- === 表单组件 === -->
            <el-form label-width="auto" label-position="top" class="profile-form">
              <!-- === 表单组件 === -->
              <el-form-item label="Token">
                <!-- === .token-display 区块 === -->
                <div class="token-display">
                  <!-- === 输入框组件 === -->
                  <el-input
                    :model-value="tokenRevealed ? tokenPlainText : 'bdf_••••••••••••••••••••••••••••••••'"
                    readonly
                    class="token-input"
                  />
                  <!-- === 按钮组件 === -->
                  <el-button @click="handleRevealToken" :loading="revealTokenLoading" class="token-action-btn">
                    {{ tokenRevealed ? '隐藏' : '查看' }}
                  </el-button>
                  <!-- === 按钮组件 === -->
                  <el-button @click="handleCopyToken" :disabled="!tokenRevealed" class="token-action-btn">
                    复制
                  </el-button>
                </div>
              </el-form-item>

              <!-- === 表单组件 === -->
              <el-form-item label="Token 前缀">
                <span class="token-meta">{{ apiTokenInfo.token_prefix }}...</span>
              </el-form-item>

              <!-- === 表单组件 === -->
              <el-form-item label="创建时间">
                <span class="token-meta">{{ apiTokenInfo.created_at || '-' }}</span>
              </el-form-item>

              <!-- === 表单组件 === -->
              <el-form-item label="最后使用">
                <span class="token-meta">{{ apiTokenInfo.last_used_at || '从未使用' }}</span>
              </el-form-item>

              <!-- === 表单组件 === -->
              <el-form-item>
                <!-- === 按钮组件 === -->
                <el-button type="primary" @click="handleGenerateToken" :loading="generateTokenLoading" class="submit-btn">
                  重新生成
                </el-button>
                <!-- === 按钮组件 === -->
                <el-button type="danger" @click="handleRevokeToken" :loading="revokeTokenLoading" class="reset-btn">
                  吊销 Token
                </el-button>
              </el-form-item>
            </el-form>

            <!-- === .token-tips 区块 === -->
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

/**
 * 用户个人中心脚本
 * @description 负责当前登录用户的信息展示、信息修改、密码修改、API Token 生成/查看/复制
 *
 * 功能模块：
 * - 个人信息表单：只读展示 username，可修改 real_name/phone/email
 * - 修改密码：需输入原密码，新密码需符合 PASSWORD_RULES
 * - API Token：生成时展示一次明文，之后以脱敏形式展示，支持查看/隐藏/复制
 *
 * 依赖：authAPI、PASSWORD_RULES、encryptPassword（加密 Token 存储）
 */
<script setup lang="ts">
// ========== Vue 核心 API ==========
import { ref, reactive, onMounted } from 'vue'
// ========== Element Plus ==========
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
// ========== Element Plus 图标 ==========
import { User, UserFilled, Phone, Message, Key, CircleCheck, CircleClose, Refresh } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { authAPI } from '@/api'
// ========== 工具函数 ==========
import { isHandledError } from '@/utils/api'
import { encryptPassword, validatePassword, PASSWORD_RULES } from '@/utils/password'

// ========== 个人信息表单区域 ==========

/** 个人信息表单数据（只读 username，可修改 real_name/phone/email） */
const profileForm = reactive({
  username: '',      // 用户名（只读）
  real_name: '',     // 姓名
  phone: '',         // 手机号
  email: '',         // 邮箱
  role: '',          // 角色（只读）
  is_active: true,   // 激活状态（只读）
})

/** 修改密码表单（三字段：原密码/新密码/确认新密码） */
const passwordForm = reactive({
  oldPassword: '',    // 原密码
  newPassword: '',     // 新密码
  confirmPassword: '', // 确认新密码
})

/** 密码表单 DOM 引用（用于调用 validate/resetFields） */
const passwordFormRef = ref<FormInstance>()
/** 更新个人信息时的按钮 loading 状态 */
const updateProfileLoading = ref(false)
/** 修改密码时的按钮 loading 状态 */
const changePasswordLoading = ref(false)

/**
 * 确认密码二次校验：确保两次输入的新密码一致
 * @param {any} _rule - Element Plus 校验规则（未使用）
 * @param {any} value - 当前表单字段值
 * @param {any} callback - 校验回调，成功不传参，失败传 Error
 */
const validateConfirmPassword = (_rule: any, value: any, callback: any) => {
  /** 条件分支 */
  if (value === '') {
    callback(new Error('请再次输入新密码'))
  } else if (value !== passwordForm.newPassword) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

/** 密码表单校验规则配置 */
const passwordRules: FormRules = {
  oldPassword: [
    { required: true, message: '请输入原密码', trigger: 'blur' },
  ],
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: any, callback: any) => {
        /** 条件分支 */
        if (!value) {
          callback(new Error('请输入新密码'))
        } else {
          const result = validatePassword(value)
          /** 条件分支 */
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

/**
 * 将角色代码映射为中文展示文本
 * @param {string} role - 角色代码（system_admin/admin/domain_admin/user）
 * @returns {string} 中文角色名，未知角色返回原值
 */
const getRoleText = (role: string) => {
  /** 分支选择 */
  switch (role) {
    /** 分支选择 */
    case 'system_admin': return '系统管理员'
    /** 分支选择 */
    case 'admin': return '管理员'
    /** 分支选择 */
    case 'domain_admin': return '领域管理员'
    /** 分支选择 */
    case 'user': return '普通用户'
    /** 分支选择 */
    default: return role
  }
}

/**
 * 根据角色返回 Element Plus tag 类型（用于 UI 着色）
 * @param {string} role - 角色代码
 * @returns {string} tag 类型（danger/warning/success/info）
 */
const getRoleTagType = (role: string) => {
  /** 分支选择 */
  switch (role) {
    /** 分支选择 */
    case 'system_admin': return 'danger'
    /** 分支选择 */
    case 'admin': return 'warning'
    /** 分支选择 */
    case 'domain_admin': return 'success'
    /** 分支选择 */
    case 'user': return 'info'
    /** 分支选择 */
    default: return 'info'
  }
}

/**
 * 加载当前登录用户的最新信息（刷新按钮和 onMounted 均会调用）
 * 从后端获取 user 对象和 role_codes 数组，填充 profileForm
 */
const loadCurrentUser = async () => {
  /** 异常处理 */
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
    /** 条件分支 */
    if (!isHandledError(error)) {
      ElMessage.error('加载用户信息失败：' + (error.message || '未知错误'))
    }
  }
}

/**
 * 提交个人信息修改（姓名/手机/邮箱），用户名/角色/状态不可修改
 * 成功后 ElMessage 提示，失败时忽略已处理错误
 */
const handleUpdateProfile = async () => {
  updateProfileLoading.value = true
  /** 异常处理 */
  try {
    await authAPI.updateProfile({
      real_name: profileForm.real_name,
      phone: profileForm.phone,
      email: profileForm.email,
    })
    ElMessage.success('个人信息更新成功')
  } catch (error: any) {
    /** 条件分支 */
    if (!isHandledError(error)) {
      ElMessage.error('更新失败：' + (error.message || '未知错误'))
    }
  } finally {
    updateProfileLoading.value = false
  }
}

/**
 * 修改密码：需输入原密码 + 两次新密码（新密码需通过 validatePassword 校验）
 * 原密码和新密码均使用 encryptPassword 加密后传给后端
 * 成功后自动重置密码表单
 */
const handleChangePassword = async () => {
  /** 条件分支 */
  if (!passwordFormRef.value) return
  
  /** 异常处理 */
  try {
    await passwordFormRef.value.validate()
  } catch {
    return
  }

  const passwordValidation = validatePassword(passwordForm.newPassword)
  /** 条件分支 */
  if (!passwordValidation.valid) {
    ElMessage.error(passwordValidation.message)
    return
  }

  changePasswordLoading.value = true
  /** 异常处理 */
  try {
    await authAPI.changePassword({
      old_password: encryptPassword(passwordForm.oldPassword),
      new_password: encryptPassword(passwordForm.newPassword),
    })
    
    ElMessage.success('密码修改成功')
    handleResetPasswordForm()
  } catch (error: any) {
    /** 条件分支 */
    if (!isHandledError(error)) {
      const errorMsg = error.response?.data?.error || error.message || '未知错误'
      ElMessage.error('密码修改失败：' + errorMsg)
    }
  } finally {
    changePasswordLoading.value = false
  }
}

/**
 * 重置密码表单内容和校验状态
 * 清空三个输入字段并调用 Element Plus resetFields 清除校验红色提示
 */
const handleResetPasswordForm = () => {
  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordFormRef.value?.resetFields()
}

// ========== API Token 相关区域 ==========

/** API Token 信息对象（后端返回的脱敏 Token 元数据） */
const apiTokenInfo = reactive({
  has_token: false,     // 是否存在 Token
  token_prefix: '',     // Token 前缀（脱敏显示）
  last_used_at: '',     // 最后使用时间
  created_at: '',       // 创建时间
})
/** 当前 Token 明文（仅在用户点击"查看"或刚生成时填充） */
const tokenPlainText = ref('')
/** 当前是否显示明文 Token（true 显示明文，false 显示脱敏占位） */
const tokenRevealed = ref(false)
/** 生成 Token 按钮 loading 状态 */
const generateTokenLoading = ref(false)
/** 查看 Token 明文按钮 loading 状态 */
const revealTokenLoading = ref(false)
/** 吊销 Token 按钮 loading 状态 */
const revokeTokenLoading = ref(false)

/**
 * 加载当前用户的 API Token 信息（是否存在、前缀、创建时间、最后使用时间）
 * 每次刷新或进入页面时调用，会重置明文显示状态
 */
const loadApiTokenInfo = async () => {
  /** 异常处理 */
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
    /** 条件分支 */
    if (!isHandledError(error)) {
      ElMessage.error('加载 API Token 信息失败：' + (error.message || '未知错误'))
    }
  }
}

/**
 * 生成或重新生成 API Token（二次确认后调用后端），生成后展示明文供用户复制
 * 重新生成会使旧 Token 立即失效，需用户二次确认
 */
const handleGenerateToken = async () => {
  /** 异常处理 */
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
  /** 异常处理 */
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
    /** 条件分支 */
    if (!isHandledError(error)) {
      ElMessage.error('生成 API Token 失败：' + (error.message || '未知错误'))
    }
  } finally {
    generateTokenLoading.value = false
  }
}

/**
 * 切换 Token 明文/脱敏显示（查看/隐藏按钮）
 * 隐藏时直接切换状态不请求后端；查看时调用 reveal 接口获取明文
 */
const handleRevealToken = async () => {
  /** 条件分支 */
  if (tokenRevealed.value) {
    tokenRevealed.value = false
    return
  }

  revealTokenLoading.value = true
  /** 异常处理 */
  try {
    const response = await authAPI.apiToken.reveal()
    tokenPlainText.value = response.data.token
    tokenRevealed.value = true
  } catch (error: any) {
    /** 条件分支 */
    if (!isHandledError(error)) {
      ElMessage.error('查看 Token 失败：' + (error.message || '未知错误'))
    }
  } finally {
    revealTokenLoading.value = false
  }
}

/**
 * 复制 Token 明文到剪贴板（需先点"查看"显示明文）
 * 优先使用 navigator.clipboard API，非安全上下文降级为 document.execCommand
 */
const handleCopyToken = async () => {
  /** 条件分支 */
  if (!tokenPlainText.value) return
  /** 异常处理 */
  try {
    /** 条件分支 */
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(tokenPlainText.value)
    } else {
      // fallback for non-secure context (HTTP non-localhost)
      const textarea = document.createElement('textarea')
      textarea.value = tokenPlainText.value
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success('Token 已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

/**
 * 吊销 API Token（二次确认后调用后端，吊销后立即失效）
 * 成功后清空所有 Token 相关状态
 */
const handleRevokeToken = async () => {
  /** 异常处理 */
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
  /** 异常处理 */
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
    /** 条件分支 */
    if (!isHandledError(error)) {
      ElMessage.error('吊销 Token 失败：' + (error.message || '未知错误'))
    }
  } finally {
    revokeTokenLoading.value = false
  }
}

/**
 * 组件挂载时初始化：加载用户信息 + 加载 API Token 信息
 */
onMounted(() => {
  loadCurrentUser()
  loadApiTokenInfo()
})
</script>

<style scoped>
/* === 页面根容器：纵向 flex 布局，全高 === */
.profile-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

/* === 统计卡片网格：响应式自适应列数 === */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: var(--space-4);
}

/* === 单个统计卡片：横向 flex 布局，hover 时上浮阴影 === */
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

/* === 统计卡片 hover 态：上移 2px + 强化阴影 === */
.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

/* === 统计图标容器：48x48 圆角方块 === */
.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* === 主色调图标（蓝色渐变背景） === */
.stat-icon-primary {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(37, 99, 235, 0.05));
  color: var(--accent-primary);
}

/* === 成功色图标（绿色渐变背景） === */
.stat-icon-success {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.05));
  color: var(--accent-success);
}

/* === 警告色图标（橙色渐变背景） === */
.stat-icon-warning {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.1), rgba(245, 158, 11, 0.05));
  color: var(--accent-warning);
}

/* === 危险色图标（红色渐变背景） === */
.stat-icon-danger {
  background: linear-gradient(135deg, rgba(239, 68, 68, 0.1), rgba(239, 68, 68, 0.05));
  color: var(--accent-danger);
}

/* === 信息色图标（亮蓝色渐变背景） === */
.stat-icon-info {
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(59, 130, 246, 0.05));
  color: var(--accent-primary);
}

/* === 统计内容区：纵向排列数值和标签，溢出省略 === */
.stat-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
  overflow: hidden;
}

/* === 统计数值：大号粗体等宽字体 === */
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

/* === 邮箱/手机号类长文本：字号略小 === */
.stat-email {
  font-size: 0.9rem;
}

/* === 统计标签：小号大写字母间距 === */
.stat-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

/* === 三个卡片容器纵向排列 === */
.profile-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

/* === 三个卡片统一样式：白底 + 圆角 + 轻阴影 === */
.profile-card,
.password-card,
/* === .api-token-card 卡片样式 === */
.api-token-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

/* === 卡片工具栏：左侧标题 + 右侧操作按钮 === */
.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

/* === 工具栏左侧区域 === */
.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

/* === 工具栏右侧区域（刷新按钮等） === */
.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

/* === 工具栏标题文本 === */
.toolbar-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

/* === 刷新按钮样式（次按钮，白底灰边） === */
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

/* === 刷新按钮 hover 态：上浮 + 主题色边框 === */
.refresh-btn:hover {
  background: var(--bg-secondary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

/* === 卡片主体内边距 === */
.card-body {
  padding: var(--space-6);
}

/* === 表单最大宽度限制 === */
.profile-form,
/* === .password-form 表单样式 === */
.password-form {
  max-width: 600px;
}

/* === 表单内 el-input 输入框容器样式 === */
.profile-form :deep(.el-input__wrapper),
/* === .password-form 表单样式 === */
.password-form :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

/* === 输入框 hover 态：主题色边框 + 淡蓝光晕 === */
.profile-form :deep(.el-input__wrapper:hover),
/* === .password-form 表单样式 === */
.password-form :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

/* === 输入框聚焦态：强化光晕 === */
.profile-form :deep(.el-input__wrapper.is-focus),
/* === .password-form 表单样式 === */
.password-form :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

/* === 输入框禁用态：灰色背景 + 灰色边框 === */
.profile-form :deep(.el-input.is-disabled .el-input__wrapper),
/* === .password-form 表单样式 === */
.password-form :deep(.el-input.is-disabled .el-input__wrapper) {
  background: var(--bg-secondary);
  border-color: var(--border-subtle);
}

/* === 表单 label 样式 === */
.profile-form :deep(.el-form-item__label),
/* === .password-form 表单样式 === */
.password-form :deep(.el-form-item__label) {
  color: var(--text-secondary);
  font-weight: 500;
}

/* === 主要提交按钮（渐变色主按钮） === */
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

/* === 提交按钮 hover 态：上浮 + 加深阴影 === */
.submit-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
}

/* === 重置按钮样式（次按钮） === */
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

/* === 重置按钮 hover 态 === */
.reset-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

/* === 密码规则提示文本 === */
.password-rules-tip {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  line-height: 1.6;
}

/* === Token 空状态容器 === */
.token-empty {
  text-align: center;
  padding: var(--space-6) 0;
}

/* === Token 空状态提示文案 === */
.token-empty-text {
  color: var(--text-muted);
  margin-bottom: var(--space-4);
  font-size: 0.9rem;
}

/* === Token 展示区：横向排列输入框 + 操作按钮 === */
.token-display {
  display: flex;
  gap: var(--space-2);
  width: 100%;
}

/* === Token 输入框占位（flex:1 撑满剩余宽度） === */
.token-input {
  flex: 1;
}

/* === Token 输入框内文字：等宽字体 + 字间距 === */
.token-input :deep(.el-input__wrapper) {
  font-family: var(--font-display, monospace);
  font-size: 0.85rem;
  letter-spacing: 0.02em;
}

/* === Token 操作按钮（查看/隐藏/复制） === */
.token-action-btn {
  font-weight: 500;
  flex-shrink: 0;
}

/* === Token 元信息文本（前缀/创建时间/最后使用） === */
.token-meta {
  color: var(--text-secondary);
  font-size: 0.9rem;
}

/* === Token 使用提示区域 === */
.token-tips {
  margin-top: var(--space-4);
  padding: var(--space-3) var(--space-4);
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
}

/* === Token 提示每条文本 === */
.token-tips p {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.8;
  margin: 0;
}

/* === 只读信息 tag（角色/状态展示） === */
.info-tag {
  height: 32px;
  padding: 0 12px;
  font-size: 0.875rem;
  border-radius: var(--radius-md);
}

/* === 响应式：≤1280px 中等屏幕 === */
@media (max-width: 1280px) {
  /* === .stats-grid 网格样式 === */
  .stats-grid {
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  }

  /* === .stat-icon 图标样式 === */
  .stat-icon {
    width: 42px;
    height: 42px;
  }

  /* === .stat-value 值样式 === */
  .stat-value {
    font-size: 1.05rem;
  }

  /* === .card-body 主体样式 === */
  .card-body {
    padding: var(--space-5);
  }
}

/* === 响应式：≤1024px 平板 === */
@media (max-width: 1024px) {
  /* === .stats-grid 网格样式 === */
  .stats-grid {
    grid-template-columns: repeat(3, 1fr);
  }

  /* === .stat-card 卡片样式 === */
  .stat-card {
    padding: var(--space-3);
    gap: var(--space-2);
  }

  /* === .stat-icon 图标样式 === */
  .stat-icon {
    width: 38px;
    height: 38px;
  }

  /* === .stat-icon 图标样式 === */
  .stat-icon :deep(.el-icon) {
    --font-size: 18px;
  }

  /* === .stat-value 值样式 === */
  .stat-value {
    font-size: 0.95rem;
  }

  .profile-form,
  /* === .password-form 表单样式 === */
  .password-form {
    max-width: 100%;
  }
}

/* === 响应式：≤768px 手机 === */
@media (max-width: 768px) {
  /* === .stats-grid 网格样式 === */
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  /* === .card-body 主体样式 === */
  .card-body {
    padding: var(--space-4);
  }
}
</style>
