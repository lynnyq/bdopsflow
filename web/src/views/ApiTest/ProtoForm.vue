<!-- Proto 消息字段表单：根据 ProtoMessageField 数组递归渲染标量/嵌套消息/数组/Map 四种输入 -->
<template>
  <div class="proto-form">
    <!-- 逐行渲染每个字段：左列展示字段名+类型标签，右列展示输入控件 -->
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
/**
 * ProtoForm.vue - Proto 消息字段表单渲染器
 *
 * 根据后端解析出的 ProtoMessageField[] 递归渲染表单，
 * 支持四种字段形态：
 *  - 标量类型（string / int32 / bool / bytes / enum:xxx 等）→ ScalarInput
 *  - 嵌套消息（message:FullTypeName）→ 递归嵌套 ProtoForm
 *  - repeated 数组 → 支持动态增删行（标量/消息通用）
 *  - map → 支持动态增删 key:value 对
 *
 * 通过 v-model<Record<string, unknown>> 双向绑定整条消息的 JSON 对象，
 * 父组件可直接序列化为 JSON 提交给 gRPC 测试接口。
 */
import { Delete, Plus } from '@element-plus/icons-vue'
import type { ProtoMessageField } from '@/api/apiTest'
import ScalarInput from './ScalarInput.vue'

/** 组件 props 定义 */
const props = defineProps<{
  /** 当前要渲染的 Proto 字段列表（来自 Proto 文件解析结果） */
  fields: ProtoMessageField[]
  /** 当前消息的 JSON 对象值（v-model） */
  modelValue: Record<string, unknown>
}>()

/** 对外 update:modelValue 事件 */
const emit = defineEmits<{
  'update:modelValue': [value: Record<string, unknown>]
}>()

// ========== 类型标签辅助函数 ==========

/** 根据字段 label/type 决定 Element Plus Tag 的颜色类型 */
function getFieldTagType(field: ProtoMessageField): 'primary' | 'warning' | 'success' | 'info' {
  if (field.label === 'map') return 'info'
  if (field.label === 'repeated') return 'warning'
  if (isMessageType(field.type)) return 'success'
  return 'primary'
}

/** 生成字段类型展示文本（如 "map<string,User>" / "[]int32" / "int32"） */
function getFieldTagText(field: ProtoMessageField): string {
  if (field.label === 'map') return `map<${field.map_key},${field.map_value}>`
  if (field.label === 'repeated') return `[]${getShortType(field.type)}`
  return getShortType(field.type)
}

// ========== 类型判断辅助函数 ==========

/** 判断是否为嵌套消息类型（type 以 "message:" 开头） */
function isMessageType(type: string): boolean {
  return type.startsWith('message:')
}

/** 去掉 Proto 完整类型名的 package 前缀，只保留最后一段短名（如 message:pkg.User → User） */
function getShortType(type: string): string {
  if (type.startsWith('message:') || type.startsWith('enum:')) {
    const fullName = type.split(':').slice(1).join(':')
    const dotIdx = fullName.lastIndexOf('.')
    return dotIdx >= 0 ? fullName.substring(dotIdx + 1) : fullName
  }
  return type
}

/** 根据 Proto 类型生成对应的零值，用于新增行时的初始值 */
function defaultScalarValue(type: string): unknown {
  if (type === 'bool') return false
  if (isNumberType(type)) return 0
  return ''
}

/** 判断是否为 Proto 数值类型 */
function isNumberType(type: string): boolean {
  return ['int32', 'int64', 'uint32', 'uint64', 'sint32', 'sint64',
    'fixed32', 'fixed64', 'sfixed32', 'sfixed64', 'float', 'double'].includes(type)
}

// ========== 标量字段读写 ==========

/** 从 modelValue 中读取指定标量字段的值，缺失时返回零值 */
function getScalarValue(name: string): unknown {
  const f = props.fields.find(fd => fd.name === name)
  return props.modelValue[name] ?? defaultScalarValue(f?.type ?? 'string')
}

/** 设置指定标量字段的值，通过 spread 触发 Vue 响应式更新 */
function setScalarValue(name: string, value: unknown) {
  emit('update:modelValue', { ...props.modelValue, [name]: value })
}

// ========== 嵌套消息字段读写 ==========

/** 读取嵌套消息对象，非对象或缺失时返回空对象 */
function getMessageValue(name: string): Record<string, unknown> {
  const val = props.modelValue[name]
  return (val && typeof val === 'object' && !Array.isArray(val)) ? val as Record<string, unknown> : {}
}

/** 设置嵌套消息对象的值 */
function setMessageValue(name: string, value: Record<string, unknown>) {
  emit('update:modelValue', { ...props.modelValue, [name]: value })
}

// ========== repeated 数组字段读写 ==========

/** 获取 repeated 字段的数组值，非数组或缺失时返回空数组 */
function getRepeatedItems(name: string): unknown[] {
  const val = props.modelValue[name]
  return Array.isArray(val) ? val : []
}

/** 向 repeated 数组追加一项（消息类型追加空对象，标量追加零值） */
function addRepeatedItem(field: ProtoMessageField) {
  const items = [...getRepeatedItems(field.name)]
  items.push(isMessageType(field.type) ? {} : defaultScalarValue(field.type))
  emit('update:modelValue', { ...props.modelValue, [field.name]: items })
}

/** 删除 repeated 数组指定索引的元素 */
function removeRepeatedItem(name: string, idx: number) {
  const items = [...getRepeatedItems(name)]
  items.splice(idx, 1)
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

/** 读取 repeated 数组中指定索引的标量值 */
function getRepeatedScalarValue(name: string, idx: number): unknown {
  return getRepeatedItems(name)[idx] ?? ''
}

/** 设置 repeated 数组中指定索引的标量值 */
function setRepeatedScalarValue(name: string, idx: number, value: unknown) {
  const items = [...getRepeatedItems(name)]
  items[idx] = value
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

/**
 * 获取 repeated message 字段指定索引的值
 * @param {string} name - 字段名称
 * @param {number} idx - 数组索引
 * @returns {Record<string, unknown>} message 对象，不存在返回空对象
 */
function getRepeatedMessageValue(name: string, idx: number): Record<string, unknown> {
  const val = getRepeatedItems(name)[idx]
  return (val && typeof val === 'object') ? val as Record<string, unknown> : {}
}

/**
 * 设置 repeated message 字段指定索引的值
 * @param {string} name - 字段名称
 * @param {number} idx - 数组索引
 * @param {Record<string, unknown>} value - 新的 message 对象
 */
function setRepeatedMessageValue(name: string, idx: number, value: Record<string, unknown>) {
  const items = [...getRepeatedItems(name)]
  items[idx] = value
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

// --- Map ---
/**
 * 获取 map 类型字段的所有条目
 * @param {string} name - 字段名称
 * @returns {Record<string, string>[]} 键值对数组
 */
function getMapItems(name: string): Record<string, string>[] {
  const val = props.modelValue[name]
  return Array.isArray(val) ? val : []
}

/**
 * 为 map 字段添加一条空的 key-value 条目
 * @param {string} name - 字段名称
 */
function addMapItem(name: string) {
  const items = [...getMapItems(name)]
  items.push({ key: '', value: '' })
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

/**
 * 删除 map 字段指定索引的条目
 * @param {string} name - 字段名称
 * @param {number} idx - 要删除的索引
 */
function removeMapItem(name: string, idx: number) {
  const items = [...getMapItems(name)]
  items.splice(idx, 1)
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

/**
 * 获取 map 字段指定索引条目的 key
 * @param {string} name - 字段名称
 * @param {number} idx - 数组索引
 * @returns {string} key 值
 */
function getMapKey(name: string, idx: number): string {
  return getMapItems(name)[idx]?.key ?? ''
}

/**
 * 设置 map 字段指定索引条目的 key
 * @param {string} name - 字段名称
 * @param {number} idx - 数组索引
 * @param {string} value - 新的 key 值
 */
function setMapKey(name: string, idx: number, value: string) {
  const items = [...getMapItems(name)]
  items[idx] = { ...items[idx], key: value }
  emit('update:modelValue', { ...props.modelValue, [name]: items })
}

/**
 * 获取 map 字段指定索引条目的 value
 * @param {string} name - 字段名称
 * @param {number} idx - 数组索引
 * @returns {string} value 值
 */
function getMapValue(name: string, idx: number): string {
  return getMapItems(name)[idx]?.value ?? ''
}

/**
 * 设置 map 字段指定索引条目的 value
 * @param {string} name - 字段名称
 * @param {number} idx - 数组索引
 * @param {string} value - 新的 value 值
 */
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
