<!-- Cron 表达式可视化编辑器：提供预设快捷选择和自定义字段两种模式 -->
<template>
  <div class="cron-editor">
    <!-- 模式切换：预设 / 自定义 -->
    <div class="cron-mode-tabs">
      <el-radio-group v-model="mode" size="small">
        <el-radio-button value="preset">预设</el-radio-button>
        <el-radio-button value="custom">自定义</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 预设模式：网格布局展示所有快捷 Cron 选项，点击即选中 -->
    <div v-if="mode === 'preset'" class="preset-grid">
      <!-- 单个预设项卡片：label（短名称） + desc（详细描述） -->
      <div
        v-for="preset in presets"
        :key="preset.value"
        :class="['preset-item', { active: cronExpression === preset.value }]"
        @click="selectPreset(preset.value)"
      >
        <div class="preset-label">{{ preset.label }}</div>
        <div class="preset-desc">{{ preset.desc }}</div>
      </div>
    </div>

    <!-- 自定义模式：6 行下拉框分别编辑 秒/分/时/日/月/周 -->
    <div v-else class="custom-fields">
      <!-- 秒字段下拉 -->
      <div class="cron-field-row">
        <span class="field-label">秒</span>
        <el-select v-model="cronFields.second" size="small" class="field-select" @change="buildCustomCron">
          <el-option v-for="opt in secondOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="field-hint">允许值: 0-59</span>
      </div>

      <!-- 分字段下拉 -->
      <div class="cron-field-row">
        <span class="field-label">分钟</span>
        <el-select v-model="cronFields.minute" size="small" class="field-select" @change="buildCustomCron">
          <el-option v-for="opt in minuteOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="field-hint">允许值: 0-59</span>
      </div>

      <!-- 小时字段下拉 -->
      <div class="cron-field-row">
        <span class="field-label">小时</span>
        <el-select v-model="cronFields.hour" size="small" class="field-select" @change="buildCustomCron">
          <el-option v-for="opt in hourOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="field-hint">允许值: 0-23</span>
      </div>

      <!-- 日期字段下拉 -->
      <div class="cron-field-row">
        <span class="field-label">日</span>
        <el-select v-model="cronFields.day" size="small" class="field-select" @change="buildCustomCron">
          <el-option v-for="opt in dayOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="field-hint">允许值: 1-31</span>
      </div>

      <!-- 月份字段下拉 -->
      <div class="cron-field-row">
        <label class="field-label">月</label>
        <el-select v-model="cronFields.month" size="small" class="field-select" @change="buildCustomCron">
          <el-option v-for="opt in monthOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="field-hint">允许值: 1-12</span>
      </div>

      <!-- 星期字段下拉（0/7=周日） -->
      <div class="cron-field-row">
        <span class="field-label">周</span>
        <el-select v-model="cronFields.week" size="small" class="field-select" @change="buildCustomCron">
          <el-option v-for="opt in weekOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <span class="field-hint">0-7 (0和7=周日)</span>
      </div>
    </div>

    <!-- 预览区域：当前 Cron 表达式输入框 + 人类可读描述 + 接下来 3 次运行时间 -->
    <div class="cron-preview">
      <!-- Cron 表达式输入框（允许用户直接粘贴） -->
      <div class="preview-label">Cron 表达式 (6字段, 含秒)</div>
      <el-input
        :model-value="cronExpression"
        @update:model-value="updateCronExpression"
        placeholder="秒 分 时 日 月 周"
        size="small"
      >
        <!-- 表达式描述标签：显示"每分钟"/"每天9点"等中文描述 -->
        <template #append>
          <el-tag size="small" :type="cronExpression ? 'success' : 'info'">
            {{ cronDescription }}
          </el-tag>
        </template>
      </el-input>
      <!-- 接下来 3 次运行时间预览 -->
      <div class="preview-hint" v-if="cronExpression && nextRuns.length > 0">
        接下来运行时间:
        <span v-for="(t, i) in nextRuns" :key="i" class="next-run">{{ t }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * CronEditor.vue - Cron 表达式可视化编辑器
 *
 * 支持两种模式：
 *  - preset：提供常用 Cron 预设快捷按钮（每 N 秒/分/小时、每天、每月等）
 *  - custom：6 字段下拉选择自由组合（秒/分/时/日/月/周）
 *
 * 以 v-model<string> 双向绑定最终的 6 字段 Cron 表达式，
 * 并附带人类可读的中文描述（cronDescription）。
 */
import { ref, reactive, computed, watch } from 'vue'

// ============================================================================
// Props & Emits（v-model: 接收/向外 emit Cron 表达式字符串）
// ============================================================================

/** 组件 props：只接收 modelValue（Cron 表达式字符串） */
const props = defineProps<{
  modelValue: string
}>()

/** 对外 update:modelValue 事件 */
const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

// ============================================================================
// 响应式状态：编辑模式、自定义 6 字段、当前表达式
// ============================================================================

/** 当前编辑模式：预设 / 自定义 */
const mode = ref<'preset' | 'custom'>('preset')

/** 自定义模式下的 6 个字段值 */
const cronFields = reactive({
  second: '*',
  minute: '*',
  hour: '*',
  day: '*',
  month: '*',
  week: '*',
})

/** 当前完整的 Cron 表达式字符串 */
const cronExpression = ref(props.modelValue || '')

// ============================================================================
// 预设 Cron 快捷选项（18 种常用表达式）
// ============================================================================

/** 预设 Cron 快捷选项：每项包含表达式、短名称、描述 */
const presets = [
  { value: '*/10 * * * * *', label: '每10秒', desc: '每10秒执行一次' },
  { value: '*/30 * * * * *', label: '每30秒', desc: '每30秒执行一次' },
  { value: '* * * * * *', label: '每分钟', desc: '每分钟执行一次' },
  { value: '0 * * * * *', label: '每分钟(第0秒)', desc: '每分钟第0秒执行' },
  { value: '*/5 * * * * *', label: '每5分钟', desc: '每5分钟执行一次' },
  { value: '0 */5 * * * *', label: '每5分钟(整点)', desc: '每5分钟整点执行' },
  { value: '0 */15 * * * *', label: '每15分钟', desc: '每15分钟执行一次' },
  { value: '0 */30 * * * *', label: '每30分钟', desc: '每30分钟执行一次' },
  { value: '0 0 * * * *', label: '每小时', desc: '每小时整点执行' },
  { value: '0 0 */2 * * *', label: '每2小时', desc: '每2小时执行一次' },
  { value: '0 0 */6 * * *', label: '每6小时', desc: '每6小时执行一次' },
  { value: '0 0 */12 * * *', label: '每12小时', desc: '每12小时执行一次' },
  { value: '0 0 0 * * *', label: '每天午夜', desc: '每天0点执行' },
  { value: '0 0 9 * * *', label: '每天早上9点', desc: '每天早上9点执行' },
  { value: '0 0 9 * * 1-5', label: '工作日9点', desc: '周一到周五早上9点' },
  { value: '0 0 0 * * 0', label: '每周日午夜', desc: '每周日0点执行' },
  { value: '0 0 0 1 * *', label: '每月1号', desc: '每月1号0点执行' },
  { value: '0 0 0 1 1 *', label: '每年1月1日', desc: '每年元旦0点执行' },
]

// ============================================================================
// 各字段下拉选项 computed（复用 baseOptions + 具体枚举值）
// ============================================================================

/** 基础选项：复用为秒/分/小时的下拉选项基础 */
const baseOptions = [
  { value: '*', label: '每单位' },
  { value: '*/1', label: '每1' },
  { value: '*/2', label: '每2' },
  { value: '*/5', label: '每5' },
  { value: '*/10', label: '每10' },
  { value: '*/15', label: '每15' },
  { value: '*/30', label: '每30' },
]

/** 秒字段下拉选项：基础选项 + 0-59 具体值 */
const secondOptions = computed(() => [
  ...baseOptions,
  ...Array.from({ length: 60 }, (_, i) => ({ value: String(i), label: `第${i}秒` })),
])

/** 分字段下拉选项：基础选项 + 0-59 具体值 */
const minuteOptions = computed(() => [
  ...baseOptions,
  ...Array.from({ length: 60 }, (_, i) => ({ value: String(i), label: `第${i}分钟` })),
])

/** 小时字段下拉选项：基础选项 + 0-23 具体值 */
const hourOptions = computed(() => [
  ...baseOptions,
  ...Array.from({ length: 24 }, (_, i) => ({ value: String(i), label: `第${i}小时` })),
])

/** 日期字段下拉选项：每天 + 1-31 具体日期 */
const dayOptions = computed(() => [
  { value: '*', label: '每天' },
  ...Array.from({ length: 31 }, (_, i) => ({ value: String(i + 1), label: `第${i + 1}天` })),
])

/** 月份字段下拉选项：每月 + 1-12 具体月 */
const monthOptions = computed(() => [
  { value: '*', label: '每月' },
  ...Array.from({ length: 12 }, (_, i) => ({ value: String(i + 1), label: `${i + 1}月` })),
])

/** 周字段下拉选项：每周 + 周日到周六 + 工作日/周末快捷项 */
const weekOptions = computed(() => [
  { value: '*', label: '每周' },
  { value: '0', label: '周日' },
  { value: '1', label: '周一' },
  { value: '2', label: '周二' },
  { value: '3', label: '周三' },
  { value: '4', label: '周四' },
  { value: '5', label: '周五' },
  { value: '6', label: '周六' },
  { value: '1-5', label: '工作日' },
  { value: '0,6', label: '周末' },
])

// ============================================================================
// 操作函数：selectPreset（预设选择）/ buildCustomCron（自定义拼接）/ updateCronExpression（手动输入）
// ============================================================================

/** 预设模式下选择某个预设表达式 */
const selectPreset = (value: string) => {
  cronExpression.value = value
  emit('update:modelValue', value)
}

/** 自定义模式下根据 6 个字段拼接 Cron 表达式并向外 emit */
const buildCustomCron = () => {
  const expr = `${cronFields.second} ${cronFields.minute} ${cronFields.hour} ${cronFields.day} ${cronFields.month} ${cronFields.week}`
  cronExpression.value = expr
  emit('update:modelValue', expr)
}

/** 手动更新 Cron 表达式（如用户直接粘贴），并自动切换到 custom 模式 */
const updateCronExpression = (value: string) => {
  cronExpression.value = value
  emit('update:modelValue', value)
  mode.value = 'custom'
}

// ============================================================================
// computed: cronDescription（人类可读中文描述）/ nextRuns（接下来 3 次运行时间）
// ============================================================================

/** 将当前 Cron 表达式翻译为人类可读的中文描述 */
const cronDescription = computed(() => {
  if (!cronExpression.value) return '未设置'
  const parts = cronExpression.value.trim().split(/\s+/)
  if (parts.length < 6) return '格式错误(需要6字段)'

  const descriptions: string[] = []

  if (parts[0] === '*') descriptions.push('每秒')
  else if (parts[0].startsWith('*/')) descriptions.push(`每${parts[0].substring(2)}秒`)
  else descriptions.push(`第${parts[0]}秒`)

  if (parts[1] === '*') descriptions.push('每分钟')
  else if (parts[1].startsWith('*/')) descriptions.push(`每${parts[1].substring(2)}分钟`)
  else descriptions.push(`第${parts[1]}分钟`)

  if (parts[2] === '*') descriptions.push('每小时')
  else if (parts[2].startsWith('*/')) descriptions.push(`每${parts[2].substring(2)}小时`)
  else descriptions.push(`${parts[2]}点`)

  if (parts[3] === '*') descriptions.push('每天')
  else descriptions.push(`${parts[3]}号`)

  if (parts[4] === '*') descriptions.push('每月')
  else {
    const monthNames = ['','1月','2月','3月','4月','5月','6月','7月','8月','9月','10月','11月','12月']
    descriptions.push(monthNames[parseInt(parts[4])] || `${parts[4]}月`)
  }

  if (parts[5] !== '*') {
    const weekNames: Record<string, string> = { '0': '周日', '1': '周一', '2': '周二', '3': '周三', '4': '周四', '5': '周五', '6': '周六', '7': '周日' }
    if (parts[5] === '1-5') descriptions.push('工作日')
    else if (parts[5] === '0,6') descriptions.push('周末')
    else descriptions.push(weekNames[parts[5]] || parts[5])
  }

  return descriptions.join(' ')
})

/** 计算当前 Cron 表达式接下来 3 次的运行时间（用于预览） */
const nextRuns = computed(() => {
  if (!cronExpression.value) return []
  const parts = cronExpression.value.trim().split(/\s+/)
  if (parts.length < 6) return []

  try {
    const results: string[] = []
    const now = new Date()
    let current = new Date(now)

    for (let i = 0; i < 3 && results.length < 3; i++) {
      const next = getNextCronTime(parts, current)
      if (next) {
        results.push(formatDateTime(next))
        current = new Date(next.getTime() + 1000)
      } else {
        break
      }
    }
    return results
  } catch {
    return []
  }
})

// ============================================================================
// 工具函数：getNextCronTime（暴力枚举下一次执行时间）/ formatDateTime（格式化）
// ============================================================================

/**
 * 计算给定 Cron 表达式在指定时间之后的下一次执行时间（暴力枚举）
 * @param parts - 已按空格切分的 6 字段 Cron 表达式数组
 * @param after - 计算起点时间（返回值必然大于此）
 * @returns 下一次执行时间 Date，5 年内找不到则返回 null
 */
function getNextCronTime(parts: string[], after: Date): Date | null {
  const second = parts[0]
  const minute = parts[1]
  const hour = parts[2]
  const day = parts[3]
  const month = parts[4]
  const week = parts[5]

  const start = new Date(after)
  start.setMilliseconds(0)
  start.setSeconds(start.getSeconds() + 1)

  for (let y = start.getFullYear(); y <= start.getFullYear() + 5; y++) {
    const months = month === '*' ? Array.from({ length: 12 }, (_, i) => i + 1) : [parseInt(month)]
    for (const m of months) {
      const maxDay = new Date(y, m, 0).getDate()
      const days = day === '*' ? Array.from({ length: maxDay }, (_, i) => i + 1) : [parseInt(day)]
      for (const d of days) {
        if (d > maxDay) continue
        const weekDay = new Date(y, m - 1, d).getDay()
        if (week !== '*') {
          if (week === '1-5' && (weekDay === 0 || weekDay === 6)) continue
          if (week === '0,6' && weekDay !== 0 && weekDay !== 6) continue
          const weekNum = parseInt(week)
          if (!isNaN(weekNum) && weekDay !== weekNum) continue
        }

        const hours = hour === '*' ? Array.from({ length: 24 }, (_, i) => i) : [parseInt(hour)]
        for (const h of hours) {
          const minutes = minute === '*' ? Array.from({ length: 60 }, (_, i) => i) : [parseInt(minute)]
          for (const min of minutes) {
            const seconds = second === '*' ? Array.from({ length: 60 }, (_, i) => i) : [parseInt(second)]
            for (const s of seconds) {
              const candidate = new Date(y, m - 1, d, h, min, s)
              if (candidate > start) return candidate
            }
          }
        }
      }
    }
  }
  return null
}

/**
 * 将 Date 对象格式化为 YYYY-MM-DD HH:mm:ss 字符串
 * @param d - 待格式化的 Date 对象
 * @returns 格式化后的时间字符串
 */
function formatDateTime(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

// ============================================================================
// watch: 外部 modelValue 变化时同步内部状态
// ============================================================================

/** 监听外部传入的 modelValue 变化：同步内部 cronExpression 值和 6 字段下拉框状态 */
watch(() => props.modelValue, (val) => {
  if (val) {
    cronExpression.value = val
    const parts = val.trim().split(/\s+/)
    if (parts.length === 6) {
      cronFields.second = parts[0]
      cronFields.minute = parts[1]
      cronFields.hour = parts[2]
      cronFields.day = parts[3]
      cronFields.month = parts[4]
      cronFields.week = parts[5]
    }
  }
})
</script>

<style scoped>
/* ========== 组件整体布局 ========== */
.cron-editor {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ========== 模式切换 Tab ========== */
.cron-mode-tabs {
  display: flex;
  justify-content: center;
}

/* ========== 预设模式网格布局 ========== */
.preset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 8px;
}

/* 预设项卡片基础样式 */
.preset-item {
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.preset-item:hover {
  border-color: #3b82f6;
  background: #f0f9ff;
}

/* 当前选中的预设项高亮 */
.preset-item.active {
  border-color: #3b82f6;
  background: linear-gradient(135deg, #eff6ff, #dbeafe);
}

.preset-label {
  font-weight: 600;
  font-size: 0.85rem;
  color: #1f2937;
}

.preset-desc {
  font-size: 0.75rem;
  color: #6b7280;
  margin-top: 4px;
}

/* ========== 自定义模式 6 字段选择器 ========== */
.custom-fields {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* 每个字段行的水平布局 */
.cron-field-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.field-label {
  width: 36px;
  font-size: 0.85rem;
  font-weight: 600;
  color: #374151;
  text-align: right;
}

.field-select {
  flex: 1;
}

.field-hint {
  font-size: 0.7rem;
  color: #9ca3af;
  min-width: 100px;
}

/* ========== 预览区域 ========== */
.cron-preview {
  padding-top: 12px;
  border-top: 1px solid #e5e7eb;
}

.preview-label {
  font-size: 0.75rem;
  color: #6b7280;
  margin-bottom: 8px;
}

.preview-hint {
  font-size: 0.75rem;
  color: #6b7280;
  margin-top: 8px;
}

/* 下次运行时间标签（绿色小胶囊） */
.next-run {
  display: inline-block;
  background: #dcfce7;
  color: #16a34a;
  padding: 2px 8px;
  border-radius: 4px;
  margin-left: 6px;
  font-family: ui-monospace, monospace;
  font-size: 0.7rem;
}
</style>