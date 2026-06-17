<template>
  <div class="proto-form">
    <div v-for="field in fields" :key="field.name" class="field-row">
      <!-- Label -->
      <div class="field-label-col">
        <span class="field-name" :title="field.name">{{ field.name }}</span>
        <el-tag
          :type="getFieldTagType(field)"
          size="small"
          effect="plain"
          class="field-type-tag"
          disable-transitions
        >{{ getFieldTagText(field) }}</el-tag>
      </div>

      <!-- Input -->
      <div class="field-input-col">
        <!-- Map field -->
        <template v-if="field.label === 'map'">
          <div class="map-items">
            <div v-for="(_, idx) in getMapItems(field.name)" :key="idx" class="map-item">
              <el-input
                :model-value="getMapKey(field.name, idx)"
                @update:model-value="(v: string) => setMapKey(field.name, idx, v)"
                placeholder="key"
                size="small"
                class="map-key-input"
              />
              <span class="map-sep">:</span>
              <el-input
                :model-value="getMapValue(field.name, idx)"
                @update:model-value="(v: string) => setMapValue(field.name, idx, v)"
                placeholder="value"
                size="small"
                class="map-val-input"
              />
              <el-button :icon="Delete" size="small" text type="danger" @click="removeMapItem(field.name, idx)" />
            </div>
            <el-button size="small" :icon="Plus" text @click="addMapItem(field.name)">添加</el-button>
          </div>
        </template>

        <!-- Repeated field -->
        <template v-else-if="field.label === 'repeated'">
          <div class="repeated-items">
            <div v-for="(_, idx) in getRepeatedItems(field.name)" :key="idx" class="repeated-item">
              <!-- Repeated message -->
              <template v-if="isMessageType(field.type)">
                <div class="nested-message">
                  <div class="nested-header">
                    <span class="nested-index">#{{ idx + 1 }}</span>
                    <el-button :icon="Delete" size="small" text type="danger" @click="removeRepeatedItem(field.name, idx)" />
                  </div>
                  <ProtoForm
                    :fields="field.fields || []"
                    :model-value="getRepeatedMessageValue(field.name, idx)"
                    @update:model-value="(v: Record<string, unknown>) => setRepeatedMessageValue(field.name, idx, v)"
                  />
                </div>
              </template>
              <!-- Repeated scalar -->
              <template v-else>
                <div class="repeated-scalar-row">
                  <ScalarInput
                    :type="field.type"
                    :model-value="getRepeatedScalarValue(field.name, idx)"
                    @update:model-value="(v: unknown) => setRepeatedScalarValue(field.name, idx, v)"
                    :placeholder="field.name"
                  />
                  <el-button :icon="Delete" size="small" text type="danger" @click="removeRepeatedItem(field.name, idx)" />
                </div>
              </template>
            </div>
            <el-button size="small" :icon="Plus" text @click="addRepeatedItem(field)">添加</el-button>
          </div>
        </template>

        <!-- Nested message field -->
        <template v-else-if="isMessageType(field.type)">
          <div class="nested-message">
            <ProtoForm
              :fields="field.fields || []"
              :model-value="getMessageValue(field.name)"
              @update:model-value="(v: Record<string, unknown>) => setMessageValue(field.name, v)"
            />
          </div>
        </template>

        <!-- Scalar field -->
        <template v-else>
          <ScalarInput
            :type="field.type"
            :model-value="getScalarValue(field.name)"
            @update:model-value="(v: unknown) => setScalarValue(field.name, v)"
            :placeholder="field.name"
          />
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Delete, Plus } from '@element-plus/icons-vue'
import type { ProtoMessageField } from '@/api/apiTest'
import ScalarInput from './ScalarInput.vue'

const props = defineProps<{
  fields: ProtoMessageField[]
  modelValue: Record<string, unknown>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, unknown>]
}>()

// --- Tag helpers ---
function getFieldTagType(field: ProtoMessageField): 'primary' | 'warning' | 'success' | 'info' {
  if (field.label === 'map') return 'info'
  if (field.label === 'repeated') return 'warning'
  if (isMessageType(field.type)) return 'success'
  return 'primary'
}

function getFieldTagText(field: ProtoMessageField): string {
  if (field.label === 'map') return `map<${field.map_key},${field.map_value}>`
  if (field.label === 'repeated') return `[]${getShortType(field.type)}`
  return getShortType(field.type)
}

// --- Type helpers ---
function isMessageType(type: string): boolean {
  return type.startsWith('message:')
}

function getShortType(type: string): string {
  if (type.startsWith('message:') || type.startsWith('enum:')) {
    const fullName = type.split(':').slice(1).join(':')
    const dotIdx = fullName.lastIndexOf('.')
    return dotIdx >= 0 ? fullName.substring(dotIdx + 1) : fullName
  }
  return type
}

function defaultScalarValue(type: string): unknown {
  if (type === 'bool') return false
  if (isNumberType(type)) return 0
  return ''
}

function isNumberType(type: string): boolean {
  return ['int32', 'int64', 'uint32', 'uint64', 'sint32', 'sint64',
    'fixed32', 'fixed64', 'sfixed32', 'sfixed64', 'float', 'double'].includes(type)
}

// --- Scalar ---
function getScalarValue(name: string): unknown {
  const f = props.fields.find(fd => fd.name === name)
  return props.modelValue[name] ?? defaultScalarValue(f?.type ?? 'string')
}

function setScalarValue(name: string, value: unknown) {
  emit('update:modelValue', { ...props.modelValue, [name]: value })
}

// --- Message ---
function getMessageValue(name: string): Record<string, unknown> {
  const val = props.modelValue[name]
  return (val && typeof val === 'object' && !Array.isArray(val)) ? val as Record<string, unknown> : {}
}

function setMessageValue(name: string, value: Record<string, unknown>) {
  emit('update:modelValue', { ...props.modelValue, [name]: value })
}

// --- Repeated ---
function getRepeatedItems(name: string): unknown[] {
  const val = props.modelValue[name]
  return Array.isArray(val) ? val : []
}

function addRepeatedItem(field: ProtoMessageField) {
  const items = [...getRepeatedItems(field.name)]
  items.push(isMessageType(field.type) ? {} : defaultScalarValue(field.type))
  emit('update:modelValue', { ...props.modelValue, [field.name]: items })
}

function removeRepeatedItem(name: string, idx: number) {
  const items = [...getRepeatedItems(name)]
  items.splice(idx, 1)
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

function getRepeatedScalarValue(name: string, idx: number): unknown {
  return getRepeatedItems(name)[idx] ?? ''
}

function setRepeatedScalarValue(name: string, idx: number, value: unknown) {
  const items = [...getRepeatedItems(name)]
  items[idx] = value
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

function getRepeatedMessageValue(name: string, idx: number): Record<string, unknown> {
  const val = getRepeatedItems(name)[idx]
  return (val && typeof val === 'object') ? val as Record<string, unknown> : {}
}

function setRepeatedMessageValue(name: string, idx: number, value: Record<string, unknown>) {
  const items = [...getRepeatedItems(name)]
  items[idx] = value
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

// --- Map ---
function getMapItems(name: string): Record<string, string>[] {
  const val = props.modelValue[name]
  return Array.isArray(val) ? val : []
}

function addMapItem(name: string) {
  const items = [...getMapItems(name)]
  items.push({ key: '', value: '' })
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

function removeMapItem(name: string, idx: number) {
  const items = [...getMapItems(name)]
  items.splice(idx, 1)
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

function getMapKey(name: string, idx: number): string {
  return getMapItems(name)[idx]?.key ?? ''
}

function setMapKey(name: string, idx: number, value: string) {
  const items = [...getMapItems(name)]
  items[idx] = { ...items[idx], key: value }
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

function getMapValue(name: string, idx: number): string {
  return getMapItems(name)[idx]?.value ?? ''
}

function setMapValue(name: string, idx: number, value: string) {
  const items = [...getMapItems(name)]
  items[idx] = { ...items[idx], value }
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}
</script>

<style scoped>
.proto-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 6px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.04);
}

.field-row:last-child {
  border-bottom: none;
}

.field-label-col {
  width: 150px;
  min-width: 150px;
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 6px;
  flex-shrink: 0;
}

.field-name {
  font-weight: 500;
  color: var(--text-primary);
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-type-tag {
  font-size: 10px;
  flex-shrink: 0;
  transform: scale(0.9);
}

.field-input-col {
  flex: 1;
  min-width: 0;
}

/* Nested */
.nested-message {
  background: rgba(99, 102, 241, 0.03);
  border: 1px solid rgba(99, 102, 241, 0.1);
  border-radius: 6px;
  padding: 8px 10px;
}

.nested-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.nested-index {
  font-size: 11px;
  color: var(--text-muted);
  font-weight: 500;
}

/* Repeated */
.repeated-items,
.map-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.repeated-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.repeated-scalar-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.map-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.map-key-input {
  width: 120px;
  flex-shrink: 0;
}

.map-sep {
  color: var(--text-muted);
  font-size: 12px;
  flex-shrink: 0;
}

.map-val-input {
  flex: 1;
}
</style>
