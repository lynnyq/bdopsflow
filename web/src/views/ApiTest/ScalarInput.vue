<!-- gRPC Proto 标量类型输入组件：根据 Proto 字段类型自动选择合适的输入控件 -->
<template>
  <div class="scalar-input">
    <!-- bool 类型 → Switch 开关 -->
    <el-switch
      v-if="type === 'bool'"
      :model-value="modelValue as boolean"
      @update:model-value="(v: boolean | string | number) => emit('update:modelValue', v)"
    />
    <!-- enum:xxx 类型 → 普通输入框（枚举值由用户手动填写字符串） -->
    <el-input
      v-else-if="isEnumType"
      :model-value="String(modelValue)"
      @update:model-value="(v: string | number | boolean) => emit('update:modelValue', v)"
      size="small"
      class="input-full"
      :placeholder="placeholder"
    />
    <!-- string / bytes 类型 → 多行文本域 -->
    <el-input
      v-else-if="type === 'string' || type === 'bytes'"
      :model-value="String(modelValue)"
      @update:model-value="(v: string | number | boolean) => emit('update:modelValue', v)"
      type="textarea"
      :autosize="{ minRows: 2, maxRows: 10 }"
      size="small"
      class="input-full"
      :placeholder="placeholder"
    />
    <!-- 数值类型 → 数字步进框 -->
    <el-input-number
      v-else-if="isNumberType"
      :model-value="modelValue as number"
      @update:model-value="(v: string | number | boolean) => emit('update:modelValue', v)"
      size="small"
      class="input-full"
      controls-position="right"
    />
    <!-- Fallback：其他未知类型 → 普通输入框 -->
    <el-input
      v-else
      :model-value="String(modelValue)"
      @update:model-value="(v: string | number | boolean) => emit('update:modelValue', v)"
      size="small"
      class="input-full"
      :placeholder="placeholder"
    />
  </div>
</template>

<script setup lang="ts">
/**
 * ScalarInput.vue - Proto 标量字段输入组件
 *
 * 供 ProtoForm 等表单组件使用，根据字段的 Proto 类型字符串
 * （bool / int32 / string / enum:XXX / ...）自动选择最合适的 Element Plus 控件。
 * 支持 v-model 双向绑定。
 */
import { computed } from 'vue'

/** 组件 props 定义 */
const props = defineProps<{
  /** Proto 字段类型，如 'bool', 'int32', 'string', 'enum:Status' 等 */
  type: string
  /** 当前字段值，由父组件通过 v-model 传入 */
  modelValue: unknown
  /** 可选的占位提示文本 */
  placeholder?: string
}>()

/** 对外 update:modelValue 事件 */
const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

/** 是否为 Proto 枚举类型（类型名以 "enum:" 开头） */
const isEnumType = computed(() => props.type.startsWith('enum:'))

/** Proto 数值类型列表，用于判断是否渲染 el-input-number */
const isNumberType = computed(() =>
  ['int32', 'int64', 'uint32', 'uint64', 'sint32', 'sint64',
   'fixed32', 'fixed64', 'sfixed32', 'sfixed64', 'float', 'double'].includes(props.type)
)
</script>

<style scoped>
.scalar-input {
  width: 100%;
}

.input-full {
  width: 100%;
}
</style>
