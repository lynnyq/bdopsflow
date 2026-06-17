<template>
  <div class="kv-editor" :class="{ 'is-disabled': disabled }">
    <div class="kv-header">
      <span v-if="enableToggle" class="kv-col-toggle"></span>
      <span class="kv-col-key">{{ keyLabel }}</span>
      <span class="kv-col-value">{{ valueLabel }}</span>
      <span class="kv-col-action"></span>
    </div>
    <p v-if="description" class="kv-description">{{ description }}</p>
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
    <div class="kv-actions">
      <el-button type="primary" :icon="Plus" size="small" :disabled="disabled" @click="addRow">{{ addLabel }}</el-button>
      <el-button v-if="modelValue.length > 0" :icon="Delete" size="small" :disabled="disabled" @click="clearAll">清空</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { Delete, Plus } from '@element-plus/icons-vue'

export interface KVItem {
  key: string
  value: string
  enabled?: boolean
}

const props = withDefaults(defineProps<{
  modelValue: KVItem[]
  keyLabel?: string
  valueLabel?: string
  keyPlaceholder?: string
  valuePlaceholder?: string
  addLabel?: string
  disabled?: boolean
  description?: string
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

const emit = defineEmits<{
  'update:modelValue': [value: KVItem[]]
}>()

const keyInputs = ref<InstanceType<typeof import('element-plus')['ElInput']>[]>([])
const valueInputs = ref<InstanceType<typeof import('element-plus')['ElInput']>[]>([])

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

const removeRow = (index: number) => {
  const updated = [...props.modelValue]
  updated.splice(index, 1)
  emit('update:modelValue', updated)
}

const clearAll = () => {
  emit('update:modelValue', [])
}

const toggleRow = (index: number, enabled: boolean) => {
  const updated = [...props.modelValue]
  updated[index] = { ...updated[index], enabled }
  emit('update:modelValue', updated)
}

const emitUpdate = () => {
  emit('update:modelValue', [...props.modelValue])
}

const onKeyTab = (index: number, e: KeyboardEvent) => {
  if (e.shiftKey) return
  const input = valueInputs.value?.[index]
  if (input) {
    input.focus()
  }
}

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
