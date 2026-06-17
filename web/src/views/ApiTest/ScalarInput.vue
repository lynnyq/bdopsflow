<template>
  <div class="scalar-input">
    <!-- Bool -->
    <el-switch
      v-if="type === 'bool'"
      :model-value="modelValue as boolean"
      @update:model-value="(v: boolean | string | number) => emit('update:modelValue', v)"
    />
    <!-- Enum -->
    <el-input
      v-else-if="isEnumType"
      :model-value="String(modelValue)"
      @update:model-value="(v: string | number | boolean) => emit('update:modelValue', v)"
      size="small"
      class="input-full"
      :placeholder="placeholder"
    />
    <!-- String / bytes: multi-line textarea -->
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
    <!-- Number types -->
    <el-input-number
      v-else-if="isNumberType"
      :model-value="modelValue as number"
      @update:model-value="(v: string | number | boolean) => emit('update:modelValue', v)"
      size="small"
      class="input-full"
      controls-position="right"
    />
    <!-- Fallback -->
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
import { computed } from 'vue'

const props = defineProps<{
  type: string
  modelValue: unknown
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const isEnumType = computed(() => props.type.startsWith('enum:'))

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
