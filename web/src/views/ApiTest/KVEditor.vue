<!-- 通用 Key-Value 编辑器：用于 HTTP Headers / Params / Body / gRPC Metadata 等场景 -->
<template>
  <div class="kv-editor" :class="{ 'is-disabled': disabled }">
    <!-- 表头区：显示键/值列名以及可选的启用开关列 -->
    <div class="kv-header">
      <span v-if="enableToggle" class="kv-col-toggle"></span>
      <span class="kv-col-key">{{ keyLabel }}</span>
      <span class="kv-col-value">{{ valueLabel }}</span>
      <span class="kv-col-action"></span>
    </div>
    <!-- 可选的描述文本（表头下方） -->
    <p v-if="description" class="kv-description">{{ description }}</p>
    <!-- 多行键值对输入：每行包含可选的启用开关 + key + value + 删除按钮 -->
    <div
      v-for="(item, index) in modelValue"
      :key="index"
      class="kv-row"
      :class="{ 'is-disabled-row': disabled || (enableToggle && item.enabled === false) }"
    >
      <el-checkbox
        v-if="enableToggle"
        :model-value="item.enabled !== false"
        size="small"
        class="kv-toggle"
        @change="(val: boolean | string | number) => toggleRow(index, !!val)"
      />
      <el-input
        ref="keyInputs"
        v-model="item.key"
        :placeholder="keyPlaceholder"
        :disabled="disabled"
        size="small"
        @change="emitUpdate"
        @keydown.tab.prevent="onKeyTab(index, $event)"
      />
      <el-input
        ref="valueInputs"
        v-model="item.value"
        :placeholder="valuePlaceholder"
        :disabled="disabled"
        size="small"
        @change="emitUpdate"
        @keydown.tab.prevent="onValueTab(index, $event)"
      />
      <el-button
        type="danger"
        :icon="Delete"
        size="small"
        circle
        :disabled="disabled"
        @click="removeRow(index)"
      />
    </div>
    <!-- 底部操作区：添加一行 / 清空全部 -->
    <div class="kv-actions">
      <el-button type="primary" :icon="Plus" size="small" :disabled="disabled" @click="addRow">{{ addLabel }}</el-button>
      <el-button v-if="modelValue.length > 0" :icon="Delete" size="small" :disabled="disabled" @click="clearAll">清空</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * KVEditor.vue - 通用 Key-Value 编辑器组件
 *
 * 以 v-model<KvItem[]> 的方式接收和更新数据，
 * 支持批量添加 / 删除 / 清空、行内启用开关、
 * 以及 Tab / Shift+Tab 跨行焦点跳转（在值末尾 Tab 自动新增一行）。
 * 广泛用于 HTTP Headers、Query Params、gRPC Metadata 等场景。
 */
import { nextTick, ref } from 'vue'
import { Delete, Plus } from '@element-plus/icons-vue'

/** 单行键值对数据结构 */
export interface KVItem {
  /** 键名 */
  key: string
  /** 值 */
  value: string
  /** 是否启用（仅 enableToggle=true 时有意义），默认启用 */
  enabled?: boolean
}

/** 组件 props 定义 */
const props = withDefaults(defineProps<{
  /** 当前键值对数组（v-model） */
  modelValue: KVItem[]
  /** 键列表头文本 */
  keyLabel?: string
  /** 值列表头文本 */
  valueLabel?: string
  /** 键输入框占位提示 */
  keyPlaceholder?: string
  /** 值输入框占位提示 */
  valuePlaceholder?: string
  /** 添加按钮文本 */
  addLabel?: string
  /** 是否整体禁用编辑 */
  disabled?: boolean
  /** 表头下方的描述文本 */
  description?: string
  /** 是否在每行前显示启用/禁用开关 */
  enableToggle?: boolean
}>(), {
  keyLabel: '键',
  valueLabel: '值',
  keyPlaceholder: 'Key',
  valuePlaceholder: 'Value',
  addLabel: '添加',
  disabled: false,
  description: '',
  enableToggle: false,
})

/** 对外 update:modelValue 事件 */
const emit = defineEmits<{
  'update:modelValue': [value: KVItem[]]
}>()

/** key 输入框 ref 数组，用于 Tab 跳转时 focus */
const keyInputs = ref<InstanceType<typeof import('element-plus')['ElInput']>[]>([])
/** value 输入框 ref 数组 */
const valueInputs = ref<InstanceType<typeof import('element-plus')['ElInput']>[]>([])

/** 追加一行空白键值对，并在 nextTick 后自动 focus 新行的 key 输入框 */
const addRow = () => {
  const newItem: KVItem = { key: '', value: '' }
  if (props.enableToggle) {
    newItem.enabled = true
  }
  const updated = [...props.modelValue, newItem]
  emit('update:modelValue', updated)
  nextTick(() => {
    const inputs = keyInputs.value
    if (inputs && inputs.length > 0) {
      inputs[inputs.length - 1]?.focus()
    }
  })
}

/** 删除指定索引行 */
const removeRow = (index: number) => {
  const updated = [...props.modelValue]
  updated.splice(index, 1)
  emit('update:modelValue', updated)
}

/** 清空所有行 */
const clearAll = () => {
  emit('update:modelValue', [])
}

/** 切换指定行的启用状态 */
const toggleRow = (index: number, enabled: boolean) => {
  const updated = [...props.modelValue]
  updated[index] = { ...updated[index], enabled }
  emit('update:modelValue', updated)
}

/**
 * 手动触发一次 update:modelValue（深拷贝数组避免 Vue 响应式引用问题）。
 * 主要绑定在 el-input 的 change 事件上。
 */
const emitUpdate = () => {
  emit('update:modelValue', [...props.modelValue])
}

/** 在 key 输入框按 Tab：跳转到同行 value 输入框；Shift+Tab 阻止默认行为 */
const onKeyTab = (index: number, e: KeyboardEvent) => {
  if (e.shiftKey) return
  const input = valueInputs.value?.[index]
  if (input) {
    input.focus()
  }
}

/**
 * 在 value 输入框按 Tab：
 *  - 正常 Tab → 跳转到下一行 key；如果已经是最后一行则自动 addRow
 *  - Shift+Tab → 跳转回同行 key
 */
const onValueTab = (index: number, e: KeyboardEvent) => {
  if (e.shiftKey) {
    const input = keyInputs.value?.[index]
    if (input) {
      input.focus()
    }
    return
  }
  if (index === props.modelValue.length - 1) {
    addRow()
  } else {
    const input = keyInputs.value?.[index + 1]
    if (input) {
      input.focus()
    }
  }
}
</script>

<style scoped>
.kv-editor {
  width: 100%;
}

.kv-editor.is-disabled {
  opacity: 0.5;
  pointer-events: none;
}

.kv-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  padding: 0 0 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.kv-header .kv-col-toggle {
  width: 24px;
  flex-shrink: 0;
}

.kv-header .kv-col-key,
.kv-header .kv-col-value {
  flex: 1;
}

.kv-header .kv-col-action {
  width: 32px;
}

.kv-description {
  margin: 4px 0 8px;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  line-height: 1.4;
}

.kv-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
  transition: background-color 0.15s ease, opacity 0.15s ease;
}

.kv-row:hover {
  background-color: var(--el-fill-color-lighter);
  border-radius: 4px;
}

.kv-row.is-disabled-row {
  opacity: 0.45;
}

.kv-row .kv-toggle {
  width: 24px;
  flex-shrink: 0;
}

.kv-row .el-input {
  flex: 1;
}

.kv-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}
</style>
