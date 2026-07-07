<template>
  <div class="system-config-page">
    <div class="page-header">
      <h2 class="page-title">系统配置</h2>
      <span class="page-subtitle">管理系统全局配置参数</span>
      <el-button
        class="reload-btn"
        :icon="Refresh"
        :loading="reloading"
        @click="handleReload"
      >
        重载配置
      </el-button>
    </div>

    <div v-for="group in groupedConfigs" :key="group.name" class="config-group">
      <div class="group-header">
        <el-icon :size="16"><component :is="group.icon" /></el-icon>
        <span class="group-name">{{ group.name }}</span>
        <span class="group-count">{{ group.items.length }} 项</span>
      </div>
      <div class="config-cards">
        <div v-for="item in group.items" :key="item.key" class="config-card">
          <div class="config-card-header">
            <span class="config-label">{{ item.label }}</span>
            <el-tag v-if="item.value !== item.default_value" type="warning" effect="light" size="small" class="modified-tag">
              已修改
            </el-tag>
            <el-button
              v-if="item.value !== item.default_value"
              link
              type="primary"
              size="small"
              class="restore-btn"
              :loading="loadingKeys[item.key]"
              @click="handleRestoreDefault(item)"
            >
              恢复默认
            </el-button>
          </div>
          <div class="config-description">{{ item.description }}</div>
          <div class="config-control">
            <template v-if="item.type === 'boolean'">
              <el-switch
                v-model="boolValues[item.key]"
                @change="(val: boolean) => handleUpdate(item.key, String(val))"
                :loading="loadingKeys[item.key]"
                active-text="开启"
                inactive-text="关闭"
              />
            </template>
            <template v-else-if="item.type === 'number'">
              <el-input-number
                v-model="numValues[item.key]"
                :min="item.min_value ?? 0"
                :max="item.max_value ?? 999999"
                :step="getStep(item)"
                @change="(val: number | undefined) => val != null && debouncedUpdate(item.key, String(val))"
                :loading="loadingKeys[item.key]"
                size="default"
                controls-position="right"
              />
            </template>
            <template v-else-if="item.type === 'text'">
              <el-input
                v-model="textValues[item.key]"
                @input="(val: string) => debouncedUpdate(item.key, val)"
                :loading="loadingKeys[item.key]"
                size="default"
                placeholder="请输入配置值"
              />
            </template>
          </div>
          <div class="config-meta">
            <span class="meta-item">
              <span class="meta-label">默认值</span>
              <span class="meta-value">{{ formatDefaultValue(item) }}</span>
            </span>
            <span v-if="item.unit" class="meta-item">
              <span class="meta-label">单位</span>
              <span class="meta-value">{{ item.unit }}</span>
            </span>
            <span v-if="item.min_value != null" class="meta-item">
              <span class="meta-label">范围</span>
              <span class="meta-value">{{ item.min_value }} ~ {{ item.max_value }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Connection, Setting, Lock, DataLine, Monitor, ChatDotRound, Refresh } from '@element-plus/icons-vue'
import { systemConfigAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import type { SystemConfigItem } from '@/types'

const configs = ref<SystemConfigItem[]>([])
const loadingKeys = reactive<Record<string, boolean>>({})
const boolValues = reactive<Record<string, boolean>>({})
const numValues = reactive<Record<string, number>>({})
const textValues = reactive<Record<string, string>>({})
const debounceTimers: Record<string, ReturnType<typeof setTimeout> | null> = {}
const reloading = ref(false)

const groupIcons: Record<string, any> = {
  '查询': Search,
  '并发': Monitor,
  '安全': Lock,
  '缓存': DataLine,
  '连接池': Connection,
  '系统': Setting,
  '消息通知': ChatDotRound,
  '其他': Setting,
}

const groupedConfigs = computed(() => {
  const groups: Record<string, SystemConfigItem[]> = {}
  for (const item of configs.value) {
    if (!groups[item.group]) {
      groups[item.group] = []
    }
    groups[item.group].push(item)
  }
  return Object.entries(groups).map(([name, items]) => ({
    name,
    items,
    icon: groupIcons[name] || Setting,
  }))
})

const getStep = (item: SystemConfigItem) => {
  // 使用 == null 同时过滤 null 和 undefined；min_value=0 时不应被当作"缺失"
  if (item.max_value == null || item.min_value == null) return 1
  const range = item.max_value - item.min_value
  if (range > 10000) return 100
  if (range > 1000) return 10
  return 1
}

const formatDefaultValue = (item: SystemConfigItem) => {
  if (item.type === 'boolean') {
    return item.default_value === 'true' ? '开启' : '关闭'
  }
  return item.default_value
}

const loadConfigs = async () => {
  try {
    const res = await systemConfigAPI.list()
    const data = res.data
    const items = Array.isArray(data) ? data : []
    configs.value = items
    for (const item of items) {
      if (item.type === 'boolean') {
        boolValues[item.key] = item.value === 'true'
      } else if (item.type === 'number') {
        // 使用 isNaN 显式判断，避免 value="0" 时被 || 当作 falsy 而错误回退到 default_value
        const parsed = parseInt(item.value, 10)
        numValues[item.key] = isNaN(parsed) ? (parseInt(item.default_value, 10) || 0) : parsed
      } else if (item.type === 'text') {
        textValues[item.key] = item.value || item.default_value || ''
      }
    }
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载配置失败')
    }
  }
}

// 将单个配置项的当前值同步到对应的 reactive（boolValues/numValues/textValues）。
// 在 handleUpdate 成功后调用，避免后端 normalize（如 clamp 到 min/max）后前端状态不一致。
const syncReactive = (item: SystemConfigItem) => {
  if (item.type === 'boolean') {
    boolValues[item.key] = item.value === 'true'
  } else if (item.type === 'number') {
    numValues[item.key] = parseInt(item.value, 10) || 0
  } else if (item.type === 'text') {
    textValues[item.key] = item.value || item.default_value || ''
  }
}

const handleUpdate = async (key: string, value: string) => {
  // 取消尚未触发的防抖调用，避免重复提交
  if (debounceTimers[key]) {
    clearTimeout(debounceTimers[key]!)
    debounceTimers[key] = null
  }
  loadingKeys[key] = true
  try {
    await systemConfigAPI.update(key, { value })
    const item = configs.value.find(c => c.key === key)
    if (item) {
      item.value = value
      syncReactive(item)
    }
    ElMessage.success('配置已更新')
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '更新配置失败')
    }
    const item = configs.value.find(c => c.key === key)
    if (item) {
      syncReactive(item)
    }
  } finally {
    loadingKeys[key] = false
  }
}

// 对 number/text 输入做 300ms 防抖，避免连续输入或快速点击步进按钮时频繁调用 API。
// boolean switch 不需要防抖（仅在用户切换时触发一次）。
const debouncedUpdate = (key: string, value: string) => {
  if (debounceTimers[key]) {
    clearTimeout(debounceTimers[key]!)
  }
  debounceTimers[key] = setTimeout(() => {
    debounceTimers[key] = null
    handleUpdate(key, value)
  }, 300)
}

// 恢复单个配置项的默认值（按钮点击，无需防抖）。
const handleRestoreDefault = (item: SystemConfigItem) => {
  handleUpdate(item.key, item.default_value)
}

// 手动触发后端从 DB 重新加载配置到内存缓存。
// 用于多节点部署时强制同步其他节点刚写入的配置，避免等待 5 分钟自动刷新周期。
const handleReload = async () => {
  reloading.value = true
  try {
    await systemConfigAPI.reload()
    await loadConfigs()
    ElMessage.success('配置已重载')
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '重载配置失败')
    }
  } finally {
    reloading.value = false
  }
}

onMounted(() => {
  loadConfigs()
})

// 组件卸载时清理所有未触发的防抖定时器，避免幽灵 API 请求
onUnmounted(() => {
  for (const key in debounceTimers) {
    if (debounceTimers[key]) {
      clearTimeout(debounceTimers[key]!)
      debounceTimers[key] = null
    }
  }
})
</script>

<style scoped>
.system-config-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-5);
  min-height: 0;
}

.page-header {
  display: flex;
  align-items: baseline;
  gap: var(--space-3);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.page-subtitle {
  font-size: 0.85rem;
  color: var(--text-muted);
}

.reload-btn {
  margin-left: auto;
}

.config-group {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.group-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-4) var(--space-5);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.04), rgba(99, 102, 241, 0.04));
  border-bottom: 1px solid var(--border-subtle);
}

.group-header .el-icon {
  color: var(--accent-primary);
}

.group-name {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
}

.group-count {
  font-size: 0.75rem;
  color: var(--text-muted);
  background: var(--bg-secondary);
  padding: 2px 8px;
  border-radius: 10px;
  margin-left: auto;
}

.config-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 12px;
  padding: var(--space-4);
}

.config-card {
  background: var(--bg-card);
  padding: var(--space-4) var(--space-5);
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: all var(--duration-normal) var(--ease-out);
}

.config-card:hover {
  background: var(--bg-secondary);
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.08);
}

.config-card-header {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.config-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
}

.modified-tag {
  font-size: 0.7rem;
}

.restore-btn {
  margin-left: auto;
  font-size: 0.75rem;
  padding: 0;
}

.config-description {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.5;
}

.config-control {
  padding: var(--space-1) 0;
}

.config-control :deep(.el-input-number) {
  width: 200px;
}

.config-control :deep(.el-input-number .el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
}

.config-control :deep(.el-input-number .el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

.config-control :deep(.el-switch) {
  --el-switch-on-color: var(--accent-primary);
}

.config-control :deep(.el-input) {
  width: 100%;
}

.config-control :deep(.el-input .el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
}

.config-control :deep(.el-input .el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

.config-control :deep(.el-input .el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 1px var(--accent-primary);
}

.config-meta {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding-top: var(--space-1);
  border-top: 1px dashed var(--border-subtle);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.75rem;
}

.meta-label {
  color: var(--text-muted);
}

.meta-value {
  color: var(--text-secondary);
  font-weight: 500;
  font-family: var(--font-mono, 'SF Mono', 'Menlo', monospace);
}
</style>
