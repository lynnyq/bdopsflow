<!--
  登录页面组件
  功能：提供 SSO 单点登录和本地账号密码登录两种模式，展示产品品牌宣传和系统健康状态
  主要区域：品牌装饰区（Logo、产品特性、系统健康）、登录表单卡片
  自动检查：系统健康状态每 30 秒轮询
-->
<template>
  <!-- === 登录页根容器（全屏 flex 居中） === -->
  <div class="login-page">
    <!-- === 背景装饰层：渐变 + 网格 + 光晕 + 浮动粒子 === -->
    <div class="login-background">
      <!-- === .bg-gradient 区块 === -->
      <div class="bg-gradient"></div>
      <!-- === .bg-grid 区块 === -->
      <div class="bg-grid"></div>
      <!-- === .bg-glow 区块 === -->
      <div class="bg-glow bg-glow-1"></div>
      <!-- === .bg-glow 区块 === -->
      <div class="bg-glow bg-glow-2"></div>
      <!-- === .bg-particles 区块 === -->
      <div class="bg-particles">
        <!-- === v-for 渲染区块 === -->
        <div class="particle" v-for="i in 20" :key="i" :style="getParticleStyle(i)"></div>
      </div>
    </div>

    <!-- === 登录主容器：左侧品牌装饰 + 右侧登录表单（两栏网格） === -->
    <div class="login-container">
      <!-- === 左侧品牌装饰栏 === -->
      <div class="login-decoration">
        <!-- === 品牌 Logo + 标题 + 副标题 === -->
        <div class="brand-section">
          <!-- === .brand-logo 区块 === -->
          <div class="brand-logo">
            <svg width="64" height="64" viewBox="0 0 64 64" fill="none">
              <defs>
                <linearGradient id="login-logo-bg" x1="0" y1="0" x2="64" y2="64" gradientUnits="userSpaceOnUse">
                  <stop stop-color="#2563EB"/>
                  <stop offset="1" stop-color="#3B82F6"/>
                </linearGradient>
              </defs>
              <rect width="64" height="64" rx="16" fill="url(#login-logo-bg)"/>
              <path d="M14 26C18 22 22 30 26 26C30 22 34 30 38 26C42 22 46 30 50 26" stroke="white" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M14 38C18 34 22 42 26 38C30 34 34 42 38 38C42 34 46 42 50 38" stroke="white" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5"/>
            </svg>
          </div>
          <h1 class="brand-title">BD<span class="accent">ops</span>Flow</h1>
          <p class="brand-subtitle">企业级分布式运维调度平台</p>
        </div>

        <!-- === 产品特性三栏预览（分布式/多数据源/实时监控） === -->
        <div class="stats-preview">
          <!-- === .stat-item 区块 === -->
          <div class="stat-item">
            <!-- === .stat-icon 区块 === -->
            <div class="stat-icon stat-icon-primary">
              <!-- === 图标组件 === -->
              <el-icon :size="20"><Cpu /></el-icon>
            </div>
            <!-- === .stat-content 区块 === -->
            <div class="stat-content">
              <!-- === .stat-value 区块 === -->
              <div class="stat-value">分布式</div>
              <!-- === .stat-label 区块 === -->
              <div class="stat-label">集群调度</div>
            </div>
          </div>
          <!-- === .stat-item 区块 === -->
          <div class="stat-item">
            <!-- === .stat-icon 区块 === -->
            <div class="stat-icon stat-icon-success">
              <!-- === 图标组件 === -->
              <el-icon :size="20"><Connection /></el-icon>
            </div>
            <!-- === .stat-content 区块 === -->
            <div class="stat-content">
              <!-- === .stat-value 区块 === -->
              <div class="stat-value">多数据源</div>
              <!-- === .stat-label 区块 === -->
              <div class="stat-label">查询引擎</div>
            </div>
          </div>
          <!-- === .stat-item 区块 === -->
          <div class="stat-item">
            <!-- === .stat-icon 区块 === -->
            <div class="stat-icon stat-icon-info">
              <!-- === 图标组件 === -->
              <el-icon :size="20"><Monitor /></el-icon>
            </div>
            <!-- === .stat-content 区块 === -->
            <div class="stat-content">
              <!-- === .stat-value 区块 === -->
              <div class="stat-value">实时</div>
              <!-- === .stat-label 区块 === -->
              <div class="stat-label">任务监控</div>
            </div>
          </div>
        </div>

        <!-- === 产品特性详细列表（高性能/可扩展/高可靠） === -->
        <div class="features-list">
          <!-- === .feature-item 区块 === -->
          <div class="feature-item">
            <!-- === .feature-icon 区块 === -->
            <div class="feature-icon">
              <!-- === 图标组件 === -->
              <el-icon><Timer /></el-icon>
            </div>
            <!-- === .feature-text 区块 === -->
            <div class="feature-text">
              <h4>高性能</h4>
              <p>基于 Go 语言，高并发低延迟</p>
            </div>
          </div>
          <!-- === .feature-item 区块 === -->
          <div class="feature-item">
            <!-- === .feature-icon 区块 === -->
            <div class="feature-icon">
              <!-- === 图标组件 === -->
              <el-icon><Expand /></el-icon>
            </div>
            <!-- === .feature-text 区块 === -->
            <div class="feature-text">
              <h4>可扩展</h4>
              <p>分布式架构，支持水平扩展</p>
            </div>
          </div>
          <!-- === .feature-item 区块 === -->
          <div class="feature-item">
            <!-- === .feature-icon 区块 === -->
            <div class="feature-icon">
              <!-- === 图标组件 === -->
              <el-icon><Lock /></el-icon>
            </div>
            <!-- === .feature-text 区块 === -->
            <div class="feature-text">
              <h4>高可靠</h4>
              <p>任务持久化，失败自动重试</p>
            </div>
          </div>
        </div>

        <!-- === 底部系统健康状态标签（绿色=正常，红色=异常） === -->
        <div class="decoration-footer">
          <!-- === .system-status 区块 === -->
          <div class="system-status" :class="{ healthy: systemHealthy, unhealthy: !systemHealthy }">
            <span class="status-dot"></span>
            <span class="status-text">{{ systemStatusText }}</span>
          </div>
        </div>
      </div>

      <!-- === 右侧登录表单卡片 === -->
      <div class="login-card-wrapper">
        <!-- === .login-card 区块 === -->
        <div class="login-card">
          <!-- === 卡片头部：欢迎标题 + 副标题 === -->
          <div class="card-header">
            <h2>欢迎回来</h2>
            <p>{{ isSso ? 'SSO 统一认证登录' : '登录访问您的工作空间' }}</p>
          </div>

          <!-- === SSO / 本地登录模式切换（仅当 authStore.ssoEnabled 为 true 时显示） === -->
          <div v-if="authStore.ssoEnabled" class="login-mode-switch">
            <span :class="{ active: isSso }" @click="isSso = true">SSO 登录</span>
            <span class="divider">|</span>
            <span :class="{ active: !isSso }" @click="isSso = false">本地登录</span>
          </div>

          <!-- === 登录表单：用户名 + 密码 + 记住我 + 登录按钮 === -->
          <el-form
            ref="loginFormRef"
            :model="loginForm"
            :rules="loginRules"
            class="login-form"
            @keyup.enter="handleLogin"
          >
            <!-- === 用户名输入框 === -->
            <div class="form-group">
              <span class="form-label">用户名</span>
              <!-- === .input-wrapper 区块 === -->
              <div class="input-wrapper">
                <!-- === .input-icon 区块 === -->
                <div class="input-icon">
                  <!-- === 图标组件 === -->
                  <el-icon><User /></el-icon>
                </div>
                <!-- === 输入框组件 === -->
                <el-input
                  v-model="loginForm.username"
                  placeholder="请输入用户名"
                  size="large"
                  class="modern-input"
                  clearable
                />
              </div>
            </div>

            <!-- === 密码输入框 === -->
            <div class="form-group">
              <span class="form-label">密码</span>
              <!-- === .input-wrapper 区块 === -->
              <div class="input-wrapper">
                <!-- === .input-icon 区块 === -->
                <div class="input-icon">
                  <!-- === 图标组件 === -->
                  <el-icon><Lock /></el-icon>
                </div>
                <!-- === 输入框组件 === -->
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

            <!-- === 表单选项：记住我 === -->
            <div class="form-options">
              <!-- === 复选框组件 === -->
              <el-checkbox v-model="rememberMe" label="记住我" />
            </div>

            <!-- === 登录主按钮（SSO 模式显示"SSO 登录"，本地模式显示"登录"，系统异常时禁用） === -->
            <el-button
              type="primary"
              size="large"
              :loading="isLoading"
              @click="handleLogin"
              class="login-btn"
              :disabled="!systemHealthy"
            >
              <!-- === 条件渲染区块 === -->
              <span v-if="!isLoading">
                <!-- === 图标组件 === -->
                <el-icon class="btn-icon"><Key /></el-icon>
                {{ isSso ? 'SSO 登录' : '登录' }}
              </span>
              <!-- === else 分支区块 === -->
              <span v-else>正在验证...</span>
            </el-button>
          </el-form>

          <!-- === 登录错误提示（红色警告框，handleLogin 设置 errorMessage 后显示） === -->
          <div v-if="errorMessage" class="error-message">
            <!-- === 图标组件 === -->
            <el-icon><CircleClose /></el-icon>
            {{ errorMessage }}
          </div>
        </div>
      </div>
    </div>

    <!-- === 页面底部：版本信息 === -->
    <div class="login-footer">
      <p>BDopsFlow v1.0.0 — 让运维调度更简单</p>
    </div>
  </div>
</template>

/**
 * 登录页面脚本
 * @description 负责登录表单校验、SSO/本地登录切换、系统健康检查、错误信息中文翻译
 *
 * 功能点：
 * - 双模式登录：SSO（通过 authStore.ssoLogin）和本地（authStore.login）
 * - 健康检查：每 30 秒轮询 /health 接口，异常时禁用登录按钮
 * - 错误翻译：将后端返回的英文错误信息转换为中文友好提示
 */
<script setup lang="ts">
// ========== Vue 核心 API ==========
import { reactive, ref, onMounted, onUnmounted } from 'vue'
// ========== Vue Router ==========
import { useRouter, useRoute } from 'vue-router'
// ========== Element Plus ==========
import { ElMessage } from 'element-plus'
// ========== Element Plus 图标 ==========
import { User, Lock, Key, Cpu, Connection, Monitor, CircleClose, Timer, Expand } from '@element-plus/icons-vue'
// ========== Pinia Store ==========
import { useAuthStore } from '@/stores/auth'
// ========== 第三方 HTTP 库 ==========
import axios from 'axios'
// ========== 工具函数 ==========
import { translateErrorMessage } from '@/utils/error'
import { secureRandom } from '@/utils/random'

// ========== 路由和 Store 实例 ==========

/** 路由实例：登录成功后跳转首页 */
const router = useRouter()
/** 路由实例：读取 URL query 参数判断是否强制本地登录 */
const route = useRoute()
/** 认证 store：封装登录逻辑、token 管理、SSO 开关状态 */
const authStore = useAuthStore()

// ========== 表单相关 ==========

/** 登录表单 DOM 引用：用于触发 validate */
const loginFormRef = ref()
/** 登录请求按钮 loading 状态 */
const isLoading = ref(false)
/** 记住我开关（登录成功后控制 token 持久化时长） */
const rememberMe = ref(false)
/** 表单主数据对象：用户名 + 密码 */
const loginForm = reactive({
  username: '',  // 用户名
  password: ''   // 密码
})

/** 登录表单校验规则（必填项即可，后端负责密码正确性校验） */
const loginRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

// ========== 系统健康与 UI 状态 ==========

/** 系统健康状态：true=正常，false=异常（异常时登录按钮禁用） */
const systemHealthy = ref(true)
/** 系统状态描述文本：正常时"系统运行正常"，异常时显示异常组件名 */
const systemStatusText = ref('系统运行正常')
/** 登录错误提示（handleLogin catch 时设置） */
const errorMessage = ref('')
/** 当前登录模式：true=SSO，false=本地账号密码 */
const isSso = ref(true)

/**
 * 检查系统健康状态（轮询 /health 接口，5 秒超时）
 * 检查 status 是否为 "healthy"，异常时将 systemHealthy 设为 false 并展示异常组件列表
 */
const checkSystemHealth = async () => {
  /** 异常处理 */
  try {
    const response = await axios.get('/health', { timeout: 5000 })
    const data = response.data
    /** 条件分支 */
    if (data.status === 'healthy') {
      systemHealthy.value = true
      systemStatusText.value = '系统运行正常'
    } else {
      systemHealthy.value = false
      const unhealthyComponents = data.unhealthy_components || []
      /** 条件分支 */
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

/**
 * 登录处理：先校验表单，再根据当前模式走 SSO 或本地登录
 * 错误信息优先用 translateErrorMessage 翻译，401→密码错误、5xx→服务器错误
 * @returns {Promise<void>}
 */
const handleLogin = async () => {
  errorMessage.value = ''
  await loginFormRef.value?.validate(async (valid: boolean) => {
    /** 条件分支 */
    if (valid) {
      isLoading.value = true
      /** 异常处理 */
      try {
        /** 条件分支 */
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
        /** 条件分支 */
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

/**
 * 生成背景粒子动画样式（随机位置、大小、延迟、持续时间）
 * @param {number} i - 粒子索引（1-20）
 * @returns {Object} CSS style 对象，含 left/top/width/height/background/animationDelay/animationDuration
 */
const getParticleStyle = (i: number) => {
  /** colors 数组常量 */
  const colors = ['rgba(59, 130, 246, 0.3)', 'rgba(6, 182, 212, 0.3)', 'rgba(167, 139, 250, 0.3)']
  /** 返回/抛出 */
  return {
    left: `${secureRandom() * 100}%`,
    top: `${secureRandom() * 100}%`,
    width: `${secureRandom() * 4 + 2}px`,
    height: `${secureRandom() * 4 + 2}px`,
    background: colors[i % 3],
    animationDelay: `${secureRandom() * 5}s`,
    animationDuration: `${secureRandom() * 10 + 10}s`
  }
}

// ========== 生命周期钩子 ==========

/** 系统健康检查定时器句柄，组件销毁时 clearInterval 防止内存泄漏 */
let healthCheckInterval: number | null = null

/**
 * 组件挂载生命周期钩子
 * 1. 立即执行一次健康检查
 * 2. 获取加密公钥（authStore.fetchPublicKey）
 * 3. 根据 URL query 参数 isSso 或全局 ssoEnabled 确定登录模式
 * 4. 启动 30 秒定时轮询健康状态
 */
onMounted(async () => {
  checkSystemHealth()
  await authStore.fetchPublicKey()
  const isSsoParam = route.query.isSso
  /** 条件分支 */
  if (isSsoParam === 'false' || isSsoParam === 'fales') {
    isSso.value = false
  } else {
    isSso.value = authStore.ssoEnabled
  }
  healthCheckInterval = window.setInterval(checkSystemHealth, 30000)
})

/**
 * 组件卸载生命周期钩子
 * 清理系统健康检查定时器，防止内存泄漏
 */
onUnmounted(() => {
  /** 条件分支 */
  if (healthCheckInterval) {
    clearInterval(healthCheckInterval)
  }
})
</script>

<style scoped>
/* ===== 登录页根容器 ===== */
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

/* ===== 背景装饰层（绝对定位铺满） ===== */
.login-background {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

/* ===== 径向渐变背景 ===== */
.bg-gradient {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse 80% 50% at 50% -20%, rgba(37, 99, 235, 0.15), transparent);
}

/* ===== 64px 网格线 + 径向遮罩 ===== */
.bg-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(37, 99, 235, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(37, 99, 235, 0.03) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: radial-gradient(ellipse 80% 50% at 50% 50%, black, transparent);
}

/* ===== 两个大光晕圆（右上角蓝 + 左下紫色，blur 100px） ===== */
.bg-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(100px);
  opacity: 0.5;
}

/* ===== 右上角蓝色光晕 ===== */
.bg-glow-1 {
  width: 600px;
  height: 600px;
  background: rgba(37, 99, 235, 0.15);
  top: -200px;
  right: -200px;
}

/* ===== 左下角紫色光晕 ===== */
.bg-glow-2 {
  width: 500px;
  height: 500px;
  background: rgba(167, 139, 250, 0.15);
  bottom: -150px;
  left: -150px;
}

/* ===== 粒子容器 ===== */
.bg-particles {
  position: absolute;
  inset: 0;
  overflow: hidden;
}

/* ===== 单个粒子：向上漂浮动画 ===== */
.particle {
  position: absolute;
  border-radius: 50%;
  animation: float-particle linear infinite;
  opacity: 0.6;
}

/* ===== 粒子漂浮关键帧：从下向上 + 旋转 ===== */
@keyframes float-particle {
  /* === 0% 标签样式 === */
  0% {
    transform: translateY(100vh) rotate(0deg);
    opacity: 0;
  }
  /* === 10% 标签样式 === */
  10% {
    opacity: 0.6;
  }
  /* === 90% 标签样式 === */
  90% {
    opacity: 0.6;
  }
  /* === 100% 标签样式 === */
  100% {
    transform: translateY(-100vh) rotate(720deg);
    opacity: 0;
  }
}

/* ===== 登录主容器（两栏网格：左侧装饰 1.2fr + 右侧表单 1fr） ===== */
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

/* ===== 容器入场动画 ===== */
@keyframes fade-in {
  /* === from 标签样式 === */
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  /* === to 标签样式 === */
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* ===== 左侧品牌装饰栏（渐变背景 + 右侧分隔线） ===== */
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

/* ===== 装饰栏顶部渐变遮罩 ===== */
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

/* ===== 品牌 Logo / 标题 / 副标题区域 ===== */
.brand-section {
  text-align: center;
  margin-bottom: var(--space-8);
  position: relative;
}

/* ===== 品牌 Logo 容器（SVG 图标 + 浮动动画） ===== */
.brand-logo {
  width: 80px;
  height: 80px;
  margin: 0 auto var(--space-4);
  color: var(--accent-primary);
  animation: float 4s ease-in-out infinite;
}

/* ===== Logo 浮动关键帧 ===== */
@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-8px); }
}

/* ===== 品牌主标题：等宽展示字体 ===== */
.brand-title {
  font-family: var(--font-display);
  font-size: 2.5rem;
  font-weight: 700;
  letter-spacing: -0.03em;
  margin: 0 0 var(--space-2) 0;
}

/* ===== 品牌主标题强调色（ops 部分蓝色） ===== */
.brand-title .accent {
  color: var(--accent-primary);
}

/* ===== 品牌副标题：等宽字体 + 字间距 ===== */
.brand-subtitle {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--text-muted);
  letter-spacing: 0.05em;
  margin: 0;
}

/* ===== 统计预览三栏网格（分布式/多数据源/实时） ===== */
.stats-preview {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-8);
}

/* ===== 单个统计卡片：纵向 flex + hover 上浮 ===== */
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

/* ===== 统计卡片 hover：上浮 2px + 主题色边框 ===== */
.stat-item:hover {
  transform: translateY(-2px);
  border-color: var(--accent-primary);
  box-shadow: var(--shadow-md);
}

/* ===== 统计图标容器：40x40 圆角方块 ===== */
.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
}

/* ===== 主色调图标（CPU/分布式） ===== */
.stat-icon-primary {
  background: rgba(59, 130, 246, 0.1);
  color: var(--accent-primary);
}

/* ===== 成功色图标（Connection/多数据源） ===== */
.stat-icon-success {
  background: rgba(52, 211, 153, 0.1);
  color: var(--accent-success);
}

/* ===== 信息色图标（Monitor/实时监控） ===== */
.stat-icon-info {
  background: rgba(6, 182, 212, 0.1);
  color: var(--accent-secondary);
}

/* ===== 统计内容：文字居中 ===== */
.stat-content {
  text-align: center;
}

/* ===== 统计数值 ===== */
.stat-value {
  font-family: var(--font-display);
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

/* ===== 统计标签 ===== */
.stat-label {
  font-size: 0.7rem;
  color: var(--text-muted);
}

/* ===== 产品特性列表容器 ===== */
.features-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  margin-bottom: var(--space-8);
}

/* ===== 单个特性项：横向 flex + hover 右移 ===== */
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

/* ===== 特性项 hover：右移 8px + 主题色边框 ===== */
.feature-item:hover {
  border-color: var(--accent-primary);
  transform: translateX(8px);
}

/* ===== 特性图标容器 ===== */
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

/* ===== 特性标题 ===== */
.feature-text h4 {
  font-family: var(--font-display);
  font-size: 0.9rem;
  font-weight: 600;
  margin: 0 0 2px 0;
  color: var(--text-primary);
}

/* ===== 特性描述 ===== */
.feature-text p {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0;
}

/* ===== 装饰栏底部（自动推到最下方） ===== */
.decoration-footer {
  margin-top: auto;
}

/* ===== 系统健康状态标签容器 ===== */
.system-status {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-full);
  transition: all 0.3s ease;
}

/* ===== 健康状态：绿色背景 + 绿色边框 ===== */
.system-status.healthy {
  background: rgba(52, 211, 153, 0.1);
  border: 1px solid rgba(52, 211, 153, 0.2);
}

/* ===== 异常状态：红色背景 + 红色边框 ===== */
.system-status.unhealthy {
  background: rgba(248, 113, 113, 0.1);
  border: 1px solid rgba(248, 113, 113, 0.2);
}

/* ===== 状态圆点 ===== */
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  transition: all 0.3s ease;
}

/* ===== 健康状态圆点：绿色 + pulse 动画 ===== */
.system-status.healthy .status-dot {
  background: var(--accent-success);
  animation: pulse 2s ease-in-out infinite;
}

/* ===== 异常状态圆点：红色（无动画） ===== */
.system-status.unhealthy .status-dot {
  background: var(--accent-danger);
}

/* ===== pulse 关键帧：圆点呼吸效果 ===== */
@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

/* ===== 状态文本 ===== */
.status-text {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  transition: all 0.3s ease;
}

/* ===== 健康状态文本颜色 ===== */
.system-status.healthy .status-text {
  color: var(--accent-success);
}

/* ===== 异常状态文本颜色 ===== */
.system-status.unhealthy .status-text {
  color: var(--accent-danger);
}

/* ===== 右侧登录表单包装器 ===== */
.login-card-wrapper {
  padding: var(--space-10);
  display: flex;
  flex-direction: column;
  justify-content: center;
}

/* ===== 登录卡片（透明背景，父容器已设） ===== */
.login-card {
  background: var(--bg-card);
}

/* ===== 卡片头部：欢迎标题 + 副标题 ===== */
.card-header {
  margin-bottom: var(--space-8);
}

/* ===== 欢迎标题：渐变蓝文字 ===== */
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

/* ===== 卡片副标题 ===== */
.card-header p {
  color: var(--text-secondary);
  margin: 0;
}

/* ===== SSO / 本地登录模式切换栏 ===== */
.login-mode-switch {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-bottom: var(--space-5);
  font-size: 0.9rem;
}

/* ===== 切换选项默认态 ===== */
.login-mode-switch span {
  cursor: pointer;
  color: var(--text-muted);
  transition: color 0.2s ease;
  user-select: none;
}

/* ===== 当前激活的切换选项 ===== */
.login-mode-switch span.active {
  color: var(--accent-primary);
  font-weight: 600;
}

/* ===== 切换分隔线 ===== */
.login-mode-switch span.divider {
  cursor: default;
  color: var(--border-default);
}

/* ===== 登录表单容器 ===== */
.login-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
}

/* ===== 单个表单组（label + input） ===== */
.form-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

/* ===== 表单 label：大写字母间距 ===== */
.form-label {
  font-family: var(--font-display);
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

/* ===== 输入框 wrapper（用于定位左侧图标） ===== */
.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
}

/* ===== 输入框左侧图标 ===== */
.input-icon {
  position: absolute;
  left: var(--space-4);
  color: var(--text-muted);
  z-index: 10;
  font-size: 1.1rem;
  pointer-events: none;
  transition: color 0.2s ease;
}

/* ===== 输入框聚焦时图标变主题色 ===== */
.input-wrapper:focus-within .input-icon {
  color: var(--accent-primary);
}

/* ===== 自定义输入框样式：内边距 + 高度 + 边框 ===== */
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

/* ===== 输入框 hover 态 ===== */
.modern-input :deep(.el-input__wrapper:hover) {
  border-color: var(--border-strong);
}

/* ===== 输入框聚焦态：主题色边框 + 青色光晕 ===== */
.modern-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 4px rgba(34, 211, 238, 0.1);
}

/* ===== 输入框内文字 ===== */
.modern-input :deep(.el-input__inner) {
  font-family: var(--font-display);
  font-size: 0.95rem;
}

/* ===== 表单选项栏：记住我 ===== */
.form-options {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

/* ===== checkbox label 样式 ===== */
.form-options :deep(.el-checkbox__label) {
  font-size: 0.85rem;
  color: var(--text-secondary);
}

/* ===== 登录主按钮：渐变 + 全宽 + 52px 高度 ===== */
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

/* ===== 登录按钮 hover：上浮 + 加深阴影 ===== */
.login-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(34, 211, 238, 0.35);
}

/* ===== 登录按钮禁用态：灰色不可点击 ===== */
.login-btn:disabled {
  background: var(--bg-tertiary);
  color: var(--text-disabled);
  cursor: not-allowed;
}

/* ===== 登录按钮左侧图标间距 ===== */
.login-btn .btn-icon {
  margin-right: var(--space-2);
}

/* ===== 登录错误提示框（红色警告） ===== */
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

/* ===== 响应式：≤900px 平板以下，隐藏左侧装饰栏，单列显示 ===== */
@media (max-width: 900px) {
  /* === .login-container 容器样式 === */
  .login-container {
    grid-template-columns: 1fr;
    max-width: 480px;
  }

  /* === .login-decoration 登录样式 === */
  .login-decoration {
    display: none;
  }
}
</style>
