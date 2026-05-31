<template>
  <div class="login-page">
    <div class="login-background">
      <div class="bg-gradient"></div>
      <div class="bg-grid"></div>
      <div class="bg-glow bg-glow-1"></div>
      <div class="bg-glow bg-glow-2"></div>
      <div class="bg-particles">
        <div class="particle" v-for="i in 20" :key="i" :style="getParticleStyle(i)"></div>
      </div>
    </div>

    <div class="login-container">
      <div class="login-decoration">
        <div class="brand-section">
          <div class="brand-logo">
            <svg width="64" height="64" viewBox="0 0 64 64" fill="none">
              <rect x="4" y="4" width="56" height="56" rx="12" stroke="currentColor" stroke-width="2"/>
              <path d="M16 32L24 24L32 32L40 24" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M16 40L24 32L32 40L40 32" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5"/>
            </svg>
          </div>
          <h1 class="brand-title">BD<span class="accent">ops</span>Flow</h1>
          <p class="brand-subtitle">企业级分布式工作流编排平台</p>
        </div>

        <div class="stats-preview">
          <div class="stat-item">
            <div class="stat-icon stat-icon-primary">
              <el-icon :size="20"><Cpu /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">分布式</div>
              <div class="stat-label">集群调度</div>
            </div>
          </div>
          <div class="stat-item">
            <div class="stat-icon stat-icon-success">
              <el-icon :size="20"><Connection /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">DAG</div>
              <div class="stat-label">工作流引擎</div>
            </div>
          </div>
          <div class="stat-item">
            <div class="stat-icon stat-icon-info">
              <el-icon :size="20"><Monitor /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-value">实时</div>
              <div class="stat-label">任务监控</div>
            </div>
          </div>
        </div>

        <div class="features-list">
          <div class="feature-item">
            <div class="feature-icon">
              <el-icon><Timer /></el-icon>
            </div>
            <div class="feature-text">
              <h4>高性能</h4>
              <p>基于 Go 语言，高并发低延迟</p>
            </div>
          </div>
          <div class="feature-item">
            <div class="feature-icon">
              <el-icon><Expand /></el-icon>
            </div>
            <div class="feature-text">
              <h4>可扩展</h4>
              <p>分布式架构，支持水平扩展</p>
            </div>
          </div>
          <div class="feature-item">
            <div class="feature-icon">
              <el-icon><Lock /></el-icon>
            </div>
            <div class="feature-text">
              <h4>高可靠</h4>
              <p>任务持久化，失败自动重试</p>
            </div>
          </div>
        </div>

        <div class="decoration-footer">
          <div class="system-status" :class="{ healthy: systemHealthy, unhealthy: !systemHealthy }">
            <span class="status-dot"></span>
            <span class="status-text">{{ systemStatusText }}</span>
          </div>
        </div>
      </div>

      <div class="login-card-wrapper">
        <div class="login-card">
          <div class="card-header">
            <h2>欢迎回来</h2>
            <p>{{ isSso ? 'SSO 统一认证登录' : '登录访问您的工作空间' }}</p>
          </div>

          <div v-if="authStore.ssoEnabled" class="login-mode-switch">
            <span :class="{ active: isSso }" @click="isSso = true">SSO 登录</span>
            <span class="divider">|</span>
            <span :class="{ active: !isSso }" @click="isSso = false">本地登录</span>
          </div>

          <el-form
            ref="loginFormRef"
            :model="loginForm"
            :rules="loginRules"
            class="login-form"
            @keyup.enter="handleLogin"
          >
            <div class="form-group">
              <span class="form-label">用户名</span>
              <div class="input-wrapper">
                <div class="input-icon">
                  <el-icon><User /></el-icon>
                </div>
                <el-input
                  v-model="loginForm.username"
                  placeholder="请输入用户名"
                  size="large"
                  class="modern-input"
                  clearable
                />
              </div>
            </div>

            <div class="form-group">
              <span class="form-label">密码</span>
              <div class="input-wrapper">
                <div class="input-icon">
                  <el-icon><Lock /></el-icon>
                </div>
                <el-input
                  v-model="loginForm.password"
                  type="password"
                  placeholder="请输入密码"
                  size="large"
                  show-password
                  class="modern-input"
                />
              </div>
            </div>

            <div class="form-options">
              <el-checkbox v-model="rememberMe" label="记住我" />
            </div>

            <el-button
              type="primary"
              size="large"
              :loading="isLoading"
              @click="handleLogin"
              class="login-btn"
              :disabled="!systemHealthy"
            >
              <span v-if="!isLoading">
                <el-icon class="btn-icon"><Key /></el-icon>
                {{ isSso ? 'SSO 登录' : '登录' }}
              </span>
              <span v-else>正在验证...</span>
            </el-button>
          </el-form>

          <div v-if="errorMessage" class="error-message">
            <el-icon><CircleClose /></el-icon>
            {{ errorMessage }}
          </div>
        </div>
      </div>
    </div>

    <div class="login-footer">
      <p>BDopsFlow v1.0.0 — 让工作流编排更简单</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock, Key, Cpu, Connection, Monitor, CircleClose, Timer, Expand } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import axios from 'axios'
import { translateErrorMessage } from '@/utils/error'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const loginFormRef = ref()
const isLoading = ref(false)
const rememberMe = ref(false)
const systemHealthy = ref(true)
const systemStatusText = ref('系统运行正常')
const errorMessage = ref('')

const isSso = ref(true)

const loginForm = reactive({
  username: '',
  password: ''
})

const loginRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const checkSystemHealth = async () => {
  try {
    const response = await axios.get('/health', { timeout: 5000 })
    const data = response.data
    if (data.status === 'healthy') {
      systemHealthy.value = true
      systemStatusText.value = '系统运行正常'
    } else {
      systemHealthy.value = false
      const unhealthyComponents = data.unhealthy_components || []
      if (unhealthyComponents.length > 0) {
        systemStatusText.value = unhealthyComponents.join('、')
      } else {
        systemStatusText.value = '系统异常'
      }
    }
  } catch (error) {
    systemHealthy.value = false
    systemStatusText.value = '系统异常'
  }
}

const handleLogin = async () => {
  errorMessage.value = ''
  await loginFormRef.value?.validate(async (valid: boolean) => {
    if (valid) {
      isLoading.value = true
      try {
        if (isSso.value && authStore.ssoEnabled) {
          await authStore.ssoLogin(loginForm.username, loginForm.password)
        } else {
          await authStore.login(loginForm.username, loginForm.password)
        }
        ElMessage.success('登录成功，欢迎回来！')
        router.push('/')
      } catch (error: any) {
      // 详细错误信息处理 - 确保全部是中文
      let errorMsg = '登录失败'
      
      // 从响应中获取错误信息
      if (error?.response?.data?.error) {
        errorMsg = error.response.data.error
      } else if (error?.response?.data?.message) {
        errorMsg = error.response.data.message
      } else if (error?.message) {
        errorMsg = error.message
      }
      
      // 使用统一的翻译函数转换为中文
      errorMsg = translateErrorMessage(errorMsg)
      
      // 如果翻译后还是英文或未知错误，设置为通用的中文提示
      if (!/[\u4e00-\u9fa5]/.test(errorMsg)) {
        if (error?.response?.status === 401) {
          errorMsg = '用户名或密码错误'
        } else if (error?.response?.status === 400) {
          errorMsg = '请求参数错误'
        } else if (error?.response?.status >= 500) {
          errorMsg = '服务器错误，请稍后重试'
        } else {
          errorMsg = '登录失败，请稍后重试'
        }
      }
      
      errorMessage.value = errorMsg
      ElMessage.error(errorMsg)
      } finally {
        isLoading.value = false
      }
    }
  })
}

const getParticleStyle = (i: number) => {
  const colors = ['rgba(59, 130, 246, 0.3)', 'rgba(6, 182, 212, 0.3)', 'rgba(167, 139, 250, 0.3)']
  return {
    left: `${Math.random() * 100}%`,
    top: `${Math.random() * 100}%`,
    width: `${Math.random() * 4 + 2}px`,
    height: `${Math.random() * 4 + 2}px`,
    background: colors[i % 3],
    animationDelay: `${Math.random() * 5}s`,
    animationDuration: `${Math.random() * 10 + 10}s`
  }
}

let healthCheckInterval: number | null = null

onMounted(async () => {
  checkSystemHealth()
  await authStore.fetchPublicKey()
  const isSsoParam = route.query.isSso
  if (isSsoParam === 'false' || isSsoParam === 'fales') {
    isSso.value = false
  } else {
    isSso.value = authStore.ssoEnabled
  }
  healthCheckInterval = window.setInterval(checkSystemHealth, 30000)
})

onUnmounted(() => {
  if (healthCheckInterval) {
    clearInterval(healthCheckInterval)
  }
})
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  background: var(--bg-primary);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  position: relative;
  overflow: hidden;
}

.login-background {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.bg-gradient {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 80% 50% at 50% -20%, rgba(37, 99, 235, 0.15), transparent);
}

.bg-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(37, 99, 235, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(37, 99, 235, 0.03) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: radial-gradient(ellipse 80% 50% at 50% 50%, black, transparent);
}

.bg-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(100px);
  opacity: 0.5;
}

.bg-glow-1 {
  width: 600px;
  height: 600px;
  background: rgba(37, 99, 235, 0.15);
  top: -200px;
  right: -200px;
}

.bg-glow-2 {
  width: 500px;
  height: 500px;
  background: rgba(167, 139, 250, 0.15);
  bottom: -150px;
  left: -150px;
}

.bg-particles {
  position: absolute;
  inset: 0;
  overflow: hidden;
}

.particle {
  position: absolute;
  border-radius: 50%;
  animation: float-particle linear infinite;
  opacity: 0.6;
}

@keyframes float-particle {
  0% {
    transform: translateY(100vh) rotate(0deg);
    opacity: 0;
  }
  10% {
    opacity: 0.6;
  }
  90% {
    opacity: 0.6;
  }
  100% {
    transform: translateY(-100vh) rotate(720deg);
    opacity: 0;
  }
}

.login-container {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  width: 100%;
  max-width: 1100px;
  min-height: 680px;
  background: var(--bg-card);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-xl), 0 0 100px rgba(37, 99, 235, 0.08);
  position: relative;
  z-index: 1;
  overflow: hidden;
  animation: fade-in 0.6s var(--ease-out);
}

@keyframes fade-in {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.login-decoration {
  background: linear-gradient(135deg, var(--bg-secondary), var(--bg-tertiary));
  padding: var(--space-10);
  display: flex;
  flex-direction: column;
  justify-content: center;
  border-right: 1px solid var(--border-subtle);
  position: relative;
  overflow: hidden;
}

.login-decoration::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 200px;
  background: linear-gradient(180deg, rgba(37, 99, 235, 0.08), transparent);
  pointer-events: none;
}

.brand-section {
  text-align: center;
  margin-bottom: var(--space-8);
  position: relative;
}

.brand-logo {
  width: 80px;
  height: 80px;
  margin: 0 auto var(--space-4);
  color: var(--accent-primary);
  animation: float 4s ease-in-out infinite;
}

@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-8px); }
}

.brand-title {
  font-family: var(--font-display);
  font-size: 2.5rem;
  font-weight: 700;
  letter-spacing: -0.03em;
  margin: 0 0 var(--space-2) 0;
}

.brand-title .accent {
  color: var(--accent-primary);
}

.brand-subtitle {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--text-muted);
  letter-spacing: 0.05em;
  margin: 0;
}

.stats-preview {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-8);
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3);
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: all 0.3s var(--ease-out);
}

.stat-item:hover {
  transform: translateY(-2px);
  border-color: var(--accent-primary);
  box-shadow: var(--shadow-md);
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-icon-primary {
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-primary);
}

.stat-icon-success {
  background: rgba(52, 211, 153, 0.1);
  color: var(--accent-success);
}

.stat-icon-info {
  background: rgba(6, 182, 212, 0.1);
  color: var(--accent-secondary);
}

.stat-content {
  text-align: center;
}

.stat-value {
  font-family: var(--font-display);
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.stat-label {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.features-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  margin-bottom: var(--space-8);
}

.feature-item {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-3) var(--space-4);
  background: var(--bg-elevated);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  transition: all 0.3s var(--ease-out);
}

.feature-item:hover {
  border-color: var(--accent-primary);
  transform: translateX(8px);
}

.feature-icon {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(37, 99, 235, 0.1);
  border-radius: var(--radius-sm);
  color: var(--accent-primary);
  font-size: 1.2rem;
}

.feature-text h4 {
  font-family: var(--font-display);
  font-size: 0.9rem;
  font-weight: 600;
  margin: 0 0 2px 0;
  color: var(--text-primary);
}

.feature-text p {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0;
}

.decoration-footer {
  margin-top: auto;
}

.system-status {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-full);
  transition: all 0.3s ease;
}

.system-status.healthy {
  background: rgba(52, 211, 153, 0.1);
  border: 1px solid rgba(52, 211, 153, 0.2);
}

.system-status.unhealthy {
  background: rgba(248, 113, 113, 0.1);
  border: 1px solid rgba(248, 113, 113, 0.2);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  transition: all 0.3s ease;
}

.system-status.healthy .status-dot {
  background: var(--accent-success);
  animation: pulse 2s ease-in-out infinite;
}

.system-status.unhealthy .status-dot {
  background: var(--accent-danger);
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

.status-text {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  transition: all 0.3s ease;
}

.system-status.healthy .status-text {
  color: var(--accent-success);
}

.system-status.unhealthy .status-text {
  color: var(--accent-danger);
}

.login-card-wrapper {
  padding: var(--space-10);
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.login-card {
  background: var(--bg-card);
}

.card-header {
  margin-bottom: var(--space-8);
}

.card-header h2 {
  font-family: var(--font-display);
  font-size: 1.75rem;
  font-weight: 700;
  margin: 0 0 var(--space-2) 0;
  background: linear-gradient(135deg, var(--text-primary), var(--accent-primary));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.card-header p {
  color: var(--text-secondary);
  margin: 0;
}

.login-mode-switch {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-5);
  font-size: 0.9rem;
}

.login-mode-switch span {
  cursor: pointer;
  color: var(--text-muted);
  transition: color 0.2s ease;
  user-select: none;
}

.login-mode-switch span.active {
  color: var(--accent-primary);
  font-weight: 600;
}

.login-mode-switch span.divider {
  cursor: default;
  color: var(--border-default);
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.form-label {
  font-family: var(--font-display);
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  left: var(--space-4);
  color: var(--text-muted);
  z-index: 10;
  font-size: 1.1rem;
  pointer-events: none;
  transition: color 0.2s ease;
}

.input-wrapper:focus-within .input-icon {
  color: var(--accent-primary);
}

.modern-input :deep(.el-input__wrapper) {
  padding-left: var(--space-12) !important;
  padding-right: var(--space-4);
  height: 52px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all 0.2s ease;
}

.modern-input :deep(.el-input__wrapper:hover) {
  border-color: var(--border-strong);
}

.modern-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 4px rgba(34, 211, 238, 0.1);
}

.modern-input :deep(.el-input__inner) {
  font-family: var(--font-display);
  font-size: 0.95rem;
}

.form-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.form-options :deep(.el-checkbox__label) {
  font-size: 0.85rem;
  color: var(--text-secondary);
}

.login-btn {
  width: 100%;
  height: 52px;
  font-family: var(--font-display);
  font-weight: 600;
  font-size: 1rem;
  letter-spacing: 0.02em;
  background: linear-gradient(135deg, var(--accent-primary), #06b6d4);
  border: none;
  border-radius: var(--radius-md);
  color: var(--bg-deepest);
  transition: all 0.3s var(--ease-out);
  margin-top: var(--space-2);
}

.login-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(34, 211, 238, 0.35);
}

.login-btn:disabled {
  background: var(--bg-tertiary);
  color: var(--text-disabled);
  cursor: not-allowed;
}

.login-btn .btn-icon {
  margin-right: var(--space-2);
}

.error-message {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-4);
  padding: var(--space-3) var(--space-4);
  background: rgba(248, 113, 113, 0.1);
  border: 1px solid rgba(248, 113, 113, 0.2);
  border-radius: var(--radius-md);
  color: var(--accent-danger);
  font-size: 0.85rem;
}

.login-tips {
  margin-top: var(--space-6);
  padding-top: var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

.tip-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: 0.8rem;
  color: var(--text-muted);
}

.tip-item .el-icon {
  color: var(--accent-info);
}

.login-footer {
  margin-top: var(--space-6);
  text-align: center;
}

.login-footer p {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  color: var(--text-disabled);
  margin: 0;
}

@media (max-width: 900px) {
  .login-container {
    grid-template-columns: 1fr;
    max-width: 480px;
  }

  .login-decoration {
    display: none;
  }
}
</style>
