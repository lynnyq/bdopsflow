<template>
  <div class="login-page">
    <div class="login-background">
      <div class="bg-gradient"></div>
      <div class="bg-grid"></div>
      <div class="bg-glow bg-glow-1"></div>
      <div class="bg-glow bg-glow-2"></div>
    </div>

    <div class="login-container">
      <div class="login-decoration">
        <div class="brand-section">
          <div class="brand-logo">
            <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
              <rect x="4" y="4" width="40" height="40" rx="8" stroke="currentColor" stroke-width="2"/>
              <path d="M14 24L20 18L26 24L32 18" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M14 30L20 24L26 30L32 24" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" opacity="0.5"/>
            </svg>
          </div>
          <h1 class="brand-title">BD<span class="accent">ops</span>Flow</h1>
          <p class="brand-subtitle">分布式工作流编排平台</p>
        </div>

        <div class="features-list">
          <div class="feature-item">
            <div class="feature-icon">
              <el-icon><Cpu /></el-icon>
            </div>
            <div class="feature-text">
              <h4>分布式调度</h4>
              <p>多节点集群调度管理</p>
            </div>
          </div>
          <div class="feature-item">
            <div class="feature-icon">
              <el-icon><Connection /></el-icon>
            </div>
            <div class="feature-text">
              <h4>DAG 工作流</h4>
              <p>可视化工作流设计</p>
            </div>
          </div>
          <div class="feature-item">
            <div class="feature-icon">
              <el-icon><Monitor /></el-icon>
            </div>
            <div class="feature-text">
              <h4>实时监控</h4>
              <p>任务执行状态追踪</p>
            </div>
          </div>
        </div>

        <div class="decoration-footer">
          <div class="status-indicator">
            <span class="status-dot"></span>
            <span class="status-text">系统运行正常</span>
          </div>
        </div>
      </div>

      <div class="login-card">
        <div class="card-header">
          <h2>欢迎回来</h2>
          <p>登录访问您的工作区</p>
        </div>

        <el-form
          ref="loginFormRef"
          :model="loginForm"
          :rules="loginRules"
          class="login-form"
          @keyup.enter="handleLogin"
        >
          <div class="form-group">
            <label class="form-label">用户名</label>
            <div class="input-wrapper">
              <div class="input-icon">
                <el-icon><User /></el-icon>
              </div>
              <el-input
                v-model="loginForm.username"
                placeholder="请输入用户名"
                size="large"
                class="modern-input"
              />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">密码</label>
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
            <a href="#" class="forgot-link">忘记密码？</a>
          </div>

          <el-button
            type="primary"
            size="large"
            :loading="isLoading"
            @click="handleLogin"
            class="login-btn"
          >
            <span v-if="!isLoading">
              <el-icon class="btn-icon"><Key /></el-icon>
              登录
            </span>
            <span v-else>正在验证...</span>
          </el-button>
        </el-form>

        <div class="card-footer">
          <div class="footer-text">
            <span>首次使用 BDopsFlow？</span>
            <a href="#" class="signup-link">创建账户</a>
          </div>
        </div>
      </div>
    </div>

    <div class="login-footer">
      <p>BDopsFlow v1.0.0 — 企业级工作流编排平台</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock, Key, Cpu, Connection, Monitor } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const loginFormRef = ref()
const isLoading = ref(false)
const rememberMe = ref(false)

const loginForm = reactive({
  username: 'admin',
  password: 'admin123'
})

const loginRules = {
  username: [{ required: true, message: 'Username is required', trigger: 'blur' }],
  password: [{ required: true, message: 'Password is required', trigger: 'blur' }]
}

const handleLogin = async () => {
  await loginFormRef.value?.validate(async (valid: boolean) => {
    if (valid) {
      isLoading.value = true
      try {
        await authStore.login(loginForm.username, loginForm.password)
        ElMessage.success('Welcome back!')
        router.push('/')
      } catch (error) {
        ElMessage.error('Authentication failed. Please check your credentials.')
      } finally {
        isLoading.value = false
      }
    }
  })
}
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

.login-container {
  display: grid;
  grid-template-columns: 1.1fr 0.9fr;
  width: 100%;
  max-width: 1000px;
  min-height: 580px;
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
  margin-bottom: var(--space-10);
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
  letter-spacing: 0.1em;
  text-transform: uppercase;
  margin: 0;
}

.features-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
  margin-bottom: var(--space-10);
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

.status-indicator {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  background: rgba(52, 211, 153, 0.1);
  border: 1px solid rgba(52, 211, 153, 0.2);
  border-radius: var(--radius-full);
}

.status-dot {
  width: 8px;
  height: 8px;
  background: var(--accent-success);
  border-radius: 50%;
  animation: pulse 2s ease-in-out infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

.status-text {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  color: var(--accent-success);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.login-card {
  padding: var(--space-10);
  display: flex;
  flex-direction: column;
  justify-content: center;
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

.forgot-link {
  font-size: 0.85rem;
  color: var(--accent-primary);
  text-decoration: none;
  transition: opacity 0.2s;
}

.forgot-link:hover {
  opacity: 0.8;
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

.login-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(34, 211, 238, 0.35);
}

.login-btn .btn-icon {
  margin-right: var(--space-2);
}

.card-footer {
  margin-top: var(--space-8);
  padding-top: var(--space-6);
  border-top: 1px solid var(--border-subtle);
  text-align: center;
}

.footer-text {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.signup-link {
  color: var(--accent-primary);
  text-decoration: none;
  font-weight: 500;
  margin-left: var(--space-1);
}

.signup-link:hover {
  text-decoration: underline;
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
    max-width: 450px;
  }

  .login-decoration {
    display: none;
  }
}
</style>
