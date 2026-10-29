<!-- 错误边界组件：捕获子组件渲染时的错误并展示友好的兜底页面 -->
<template>
  <!-- 正常渲染 slot；出错时展示错误面板 -->
  <slot v-if="!hasError"></slot>
  <div v-else class="error-boundary">
    <div class="error-content">
      <div class="error-icon">
        <el-icon :size="64"><WarningFilled /></el-icon>
      </div>
      <h2 class="error-title">{{ title }}</h2>
      <p class="error-message">{{ message }}</p>
      <!-- 操作按钮区 -->
      <div class="error-actions">
        <el-button type="primary" @click="handleReload">
          <el-icon><Refresh /></el-icon>
          重新加载
        </el-button>
        <el-button @click="handleReset" v-if="enableReset">
          <el-icon><RefreshLeft /></el-icon>
          重置页面
        </el-button>
      </div>
      <!-- 可折叠的错误详情（仅开发/排错时展开） -->
      <details v-if="showDetails && errorInfo" class="error-details">
        <summary>查看详细信息</summary>
        <pre>{{ errorInfo }}</pre>
      </details>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * ErrorBoundary.vue - Vue 错误边界组件
 *
 * 使用 onErrorCaptured 钩子捕获子组件渲染期间抛出的错误，
 * 展示兜底 UI 而不是白屏，同时将错误信息通过 error 事件向外透出。
 * 提供 重新加载 / 重置状态 两种恢复策略。
 */
import { ref, onErrorCaptured } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, RefreshLeft, WarningFilled } from '@element-plus/icons-vue'

/** 组件 props 接口定义 */
interface Props {
  /** 错误面板标题 */
  title?: string
  /** 错误面板描述 */
  message?: string
  /** 是否显示 "重置页面" 按钮 */
  enableReset?: boolean
  /** 是否显示可折叠的错误详情 */
  showDetails?: boolean
}

// 使用 withDefaults 提供默认值
withDefaults(defineProps<Props>(), {
  title: '页面出错了',
  message: '抱歉，页面加载时遇到了问题。请尝试刷新页面。',
  enableReset: true,
  showDetails: true
})

/**
 * 对外透出的 error 事件，便于父组件上报或记录日志
 * @param error - 原始 Error 对象
 * @param vm - 出错的组件实例
 * @param info - Vue 框架附带的错误上下文描述
 */
const emit = defineEmits<{
  error: [error: Error, vm: any, info: string]
}>()

/** 是否已捕获到错误 */
const hasError = ref(false)
/** 捕获到的错误详情文本（含堆栈） */
const errorInfo = ref('')

/**
 * 错误捕获钩子：将 hasError 置为 true，
 * 记录错误详情并通过 emit / console 对外暴露，
 * 返回 false 阻止错误继续向上传播。
 */
onErrorCaptured((err: Error, instance: any, info: string) => {
  hasError.value = true
  errorInfo.value = `错误信息: ${err.message}\n堆栈跟踪:\n${err.stack}\n\n组件信息: ${info}`

  emit('error', err, instance, info)

  // 记录错误日志
  console.error('Error captured by ErrorBoundary:', err, info)

  // 显示错误提示
  ElMessage.error('页面加载失败，请刷新重试')

  return false // 阻止错误继续传播
})

/** 执行浏览器全量刷新 */
const handleReload = () => {
  window.location.reload()
}

/** 重置错误状态，让 slot 重新渲染 */
const handleReset = () => {
  hasError.value = false
  errorInfo.value = ''
}
</script>

<style scoped>
.error-boundary {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
  padding: 48px 24px;
}

.error-content {
  text-align: center;
  max-width: 600px;
}

.error-icon {
  margin-bottom: 24px;
  color: var(--accent-danger);
  opacity: 0.8;
}

.error-title {
  font-family: var(--font-display);
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 12px 0;
}

.error-message {
  font-size: 1rem;
  color: var(--text-secondary);
  margin: 0 0 32px 0;
  line-height: 1.6;
}

.error-actions {
  display: flex;
  justify-content: center;
  gap: 16px;
  margin-bottom: 32px;
}

.error-details {
  text-align: left;
  margin-top: 24px;
  padding: 16px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-md);
  font-size: 0.85rem;
}

.error-details summary {
  cursor: pointer;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 12px;
}

.error-details pre {
  margin: 0;
  padding: 12px;
  background: var(--bg-deepest);
  border-radius: var(--radius-sm);
  overflow-x: auto;
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 0.75rem;
  line-height: 1.5;
}
</style>
