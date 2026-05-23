<template>
  <div class="system-config-page">
    <div class="page-header">
      <h2 class="page-title">系统配置</h2>
      <span class="page-subtitle">管理数据源查询相关的全局配置参数</span>
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
                :min="item.min_value || 0"
                :max="item.max_value || 999999"
                :step="getStep(item)"
                @change="(val: number) => handleUpdate(item.key, String(val))"
                :loading="loadingKeys[item.key]"
                size="default"
                controls-position="right"
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
import { ref, computed, onMounted, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Connection, Setting, Lock, DataLine, Monitor } from '@element-plus/icons-vue'
import { systemConfigAPI } from '@/api'

interface ConfigItem {
  key: string
  label: string
  description: string
  type: string
  default_value: string
  value: string
  min_value?: number | null
  max_value?: number | null
  unit?: string
  group: string
}

const configs = ref<ConfigItem[]>([])
const loadingKeys = reactive<Record<string, boolean>>({})
const boolValues = reactive<Record<string, boolean>>({})
const numValues = reactive<Record<string, number>>({})

const groupIcons: Record<string, any> = {
  '查询': Search,
  '并发': Monitor,
  '安全': Lock,
  '缓存': DataLine,
  '连接池': Connection,
  '其他': Setting,
}

const groupedConfigs = computed(() => {
  const groups: Record<string, ConfigItem[]> = {}
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

const getStep = (item: ConfigItem) => {
  if (!item.max_value || !item.min_value) return 1
  const range = item.max_value - item.min_value
  if (range > 10000) return 100
  if (range > 1000) return 10
  return 1
}

const formatDefaultValue = (item: ConfigItem) => {
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
        numValues[item.key] = parseInt(item.value, 10) || parseInt(item.default_value, 10) || 0
      }
    }
  } catch (err: any) {
    ElMessage.error(err.message || '加载配置失败')
  }
}

const handleUpdate = async (key: string, value: string) => {
  loadingKeys[key] = true
  try {
    await systemConfigAPI.update(key, { value })
    const item = configs.value.find(c => c.key === key)
    if (item) {
      item.value = value
    }
    ElMessage.success('配置已更新')
  } catch (err: any) {
    ElMessage.error(err.message || '更新配置失败')
    const item = configs.value.find(c => c.key === key)
    if (item) {
      if (item.type === 'boolean') {
        boolValues[key] = item.value === 'true'
      } else if (item.type === 'number') {
        numValues[key] = parseInt(item.value, 10) || 0
      }
    }
  } finally {
    loadingKeys[key] = false
  }
}

onMounted(() => {
  loadConfigs()
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
