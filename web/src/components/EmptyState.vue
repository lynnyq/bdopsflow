<template>
  <!-- 空状态组件容器 -->
  <div class="empty-state">
    <div class="empty-content">
      <!-- 图标区域 -->
      <div class="empty-icon">
        <slot name="icon">
          <el-icon :size="iconSize">
            <component :is="icon" />
          </el-icon>
        </slot>
      </div>
      <!-- 标题 -->
      <h3 class="empty-title">{{ title }}</h3>
      <!-- 描述文字 -->
      <p class="empty-description">{{ description }}</p>
      <!-- 操作按钮区域 -->
      <div v-if="$slots.action" class="empty-action">
        <slot name="action"></slot>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  icon?: string
  title?: string
  description?: string
  size?: 'small' | 'default' | 'large'
}

const props = withDefaults(defineProps<Props>(), {
  icon: 'Document',  // icon
  title: '暂无数据',  // 标题
  description: '当前没有可显示的数据',  // 描述
  size: 'default'  // size
})

/**
 * 根据 size prop 计算图标像素尺寸
 * small → 32px，default → 48px，large → 64px
 */
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
/* === empty state === */
.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  min-height: 200px;
}

/* === empty content === */
.empty-content {
  text-align: center;
  max-width: 400px;
}

/* === empty icon === */
.empty-icon {
  margin-bottom: 16px;
  color: var(--text-disabled);
  opacity: 0.5;
}

/* === empty title === */
.empty-title {
  font-family: var(--font-display);
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
}

/* === empty description === */
.empty-description {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin: 0 0 24px 0;
  line-height: 1.6;
}

/* === empty action === */
.empty-action {
  display: flex;
  justify-content: center;
  gap: 12px;
}
</style>
