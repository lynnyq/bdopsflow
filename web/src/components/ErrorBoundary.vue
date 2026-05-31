<template>
  <slot v-if="!hasError"></slot>
  <div v-else class="error-boundary">
    <div class="error-content">
      <div class="error-icon">
        <el-icon :size="64"><WarningFilled /></el-icon>
      </div>
      <h2 class="error-title">{{ title }}</h2>
      <p class="error-message">{{ message }}</p>
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
      <details v-if="showDetails && errorInfo" class="error-details">
        <summary>查看详细信息</summary>
        <pre>{{ errorInfo }}</pre>
      </details>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, RefreshLeft, WarningFilled } from '@element-plus/icons-vue'

interface Props {
  title?: string
  message?: string
  enableReset?: boolean
  showDetails?: boolean
}

withDefaults(defineProps<Props>(), {
  title: '页面出错了',
  message: '抱歉，页面加载时遇到了问题。请尝试刷新页面。',
  enableReset: true,
  showDetails: true
})

const emit = defineEmits<{
  error: [error: Error, vm: any, info: string]
}>()

const hasError = ref(false)
const errorInfo = ref('')

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

const handleReload = () => {
  window.location.reload()
}

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
