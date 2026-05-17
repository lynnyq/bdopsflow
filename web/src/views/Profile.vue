<template>
  <div class="profile-container">
    <el-card class="profile-card">
      <template #header>
        <div class="card-header">
          <span>个人信息</span>
        </div>
      </template>
      
      <el-form :model="profileForm" label-width="100px" class="profile-form">
        <el-form-item label="用户名">
          <el-input v-model="profileForm.username" disabled />
        </el-form-item>
        
        <el-form-item label="邮箱">
          <el-input v-model="profileForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        
        <el-form-item label="角色">
          <el-input v-model="profileForm.role" disabled />
        </el-form-item>
        
        <el-form-item label="状态">
          <el-tag :type="profileForm.is_active ? 'success' : 'danger'">
            {{ profileForm.is_active ? '激活' : '未激活' }}
          </el-tag>
        </el-form-item>
        
        <el-form-item>
          <el-button type="primary" @click="handleUpdateProfile" :loading="updateProfileLoading">
            保存修改
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="password-card">
      <template #header>
        <div class="card-header">
          <span>修改密码</span>
        </div>
      </template>
      
      <el-form :model="passwordForm" :rules="passwordRules" ref="passwordFormRef" label-width="120px">
        <el-form-item label="原密码" prop="oldPassword">
          <el-input 
            v-model="passwordForm.oldPassword" 
            type="password" 
            placeholder="请输入原密码" 
            show-password
          />
        </el-form-item>
        
        <el-form-item label="新密码" prop="newPassword">
          <el-input 
            v-model="passwordForm.newPassword" 
            type="password" 
            placeholder="请输入新密码（至少6位）" 
            show-password
          />
        </el-form-item>
        
        <el-form-item label="确认新密码" prop="confirmPassword">
          <el-input 
            v-model="passwordForm.confirmPassword" 
            type="password" 
            placeholder="请再次输入新密码" 
            show-password
          />
        </el-form-item>
        
        <el-form-item>
          <el-button type="primary" @click="handleChangePassword" :loading="changePasswordLoading">
            修改密码
          </el-button>
          <el-button @click="handleResetPasswordForm">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, FormInstance, FormRules } from 'element-plus'
import { authAPI } from '@/api'
import { passwordUtils, validatePassword } from '@/utils/password'
import { useAuthStore } from '@/stores/auth'
import { handleError, handleSuccess, formatValue } from '@/utils/error'

const authStore = useAuthStore()

const profileForm = reactive({
  username: '',
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

const validateConfirmPassword = (rule: any, value: any, callback: any) => {
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
    { min: 6, message: '密码长度至少为6位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

const loadCurrentUser = async () => {
  try {
    const response = await authAPI.getCurrentUser()
    const user = response.data
    
    profileForm.username = user.username || ''
    profileForm.email = user.email || ''
    profileForm.role = user.role || ''
    profileForm.is_active = user.is_active ?? true
  } catch (error: any) {
    ElMessage.error('加载用户信息失败：' + (error.message || '未知错误'))
  }
}

const handleUpdateProfile = async () => {
  updateProfileLoading.value = true
  try {
    await authAPI.updateProfile({
      email: profileForm.email,
    })
    ElMessage.success('个人信息更新成功')
  } catch (error: any) {
    ElMessage.error('更新失败：' + (error.message || '未知错误'))
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
      old_password: passwordUtils.encodePassword(passwordForm.oldPassword),
      new_password: passwordUtils.encodePassword(passwordForm.newPassword),
    })
    
    ElMessage.success('密码修改成功')
    handleResetPasswordForm()
  } catch (error: any) {
    const errorMsg = error.response?.data?.error || error.message || '未知错误'
    ElMessage.error('密码修改失败：' + errorMsg)
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
.profile-container {
  padding: 20px;
  max-width: 800px;
  margin: 0 auto;
}

.profile-card {
  margin-bottom: 20px;
}

.password-card {
  margin-bottom: 20px;
}

.card-header {
  font-size: 18px;
  font-weight: bold;
}

.profile-form {
  max-width: 500px;
}
</style>
