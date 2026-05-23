<template>
  <div class="empty-state">
    <div class="empty-content">
      <div class="empty-icon">
        <slot name="icon">
          <el-icon :size="iconSize">
            <component :is="icon" />
          </el-icon>
        </slot>
      </div>
      <h3 class="empty-title">{{ title }}</h3>
      <p class="empty-description">{{ description }}</p>
      <div v-if="$slots.action" class="empty-action">
        <slot name="action"></slot>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Document } from '@element-plus/icons-vue'

interface Props {
  icon?: string
  title?: string
  description?: string
  size?: 'small' | 'default' | 'large'
}

const props = withDefaults(defineProps<Props>(), {
  icon: 'Document',
  title: '暂无数据',
  description: '当前没有可显示的数据',
  size: 'default'
})

const iconSize = computed(() => {
  switch (props.size) {
    case 'small':
      return 32
    case 'large':
      return 64
    default:
      return 48
  }
})
</script>

<style scoped>
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  min-height: 200px;
}

.empty-content {
  text-align: center;
  max-width: 400px;
}

.empty-icon {
  margin-bottom: 16px;
  color: var(--text-disabled);
  opacity: 0.5;
}

.empty-title {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
}

.empty-description {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin: 0 0 24px 0;
  line-height: 1.6;
}

.empty-action {
  display: flex;
  justify-content: center;
  gap: 12px;
}
</style>
