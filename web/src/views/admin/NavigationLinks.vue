<!-- 导航快捷链接管理页面：配置 Dashboard 首页快捷入口列表 -->
<template>
  <div class="navigation-links-page page-container">
    <!-- 页面工具栏：搜索 + 刷新 + 保存排序 + 创建按钮 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索名称或URL..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadList" class="refresh-btn">刷新</el-button>
        <el-button
          v-if="hasSortChanges"
          type="warning"
          :icon="Check"
          :loading="reordering"
          @click="handleSaveSortOrder"
        >
          保存排序
        </el-button>
        <el-button type="primary" :icon="Plus" @click="handleCreate" class="create-btn">新建导航</el-button>
      </div>
    </div>

    <!-- 导航列表表格 -->
    <div class="table-wrapper">
      <el-table :data="filteredList" v-loading="loading" stripe height="100%">
        <el-table-column label="排序" width="90" align="center">
          <template #default="{ row }">
            <el-input-number
              v-model="row.sort_order"
              :min="0"
              :step="1"
              size="small"
              controls-position="right"
              style="width: 72px"
              @change="markSortChanged"
            />
          </template>
        </el-table-column>
        <el-table-column label="图标" width="120" align="center">
          <template #default="{ row }">
            <div class="icon-cell">
              <div class="icon-thumb" :style="row.icon_data ? '' : { backgroundColor: fallbackColor(row.url) }">
                <img v-if="row.icon_data" :src="row.icon_data" :alt="row.name" />
                <span v-else>{{ fallbackInitial(row.url) }}</span>
              </div>
              <el-button
                link
                size="small"
                :icon="RefreshRight"
                :loading="refreshingIds.has(row.id)"
                title="重新抓取图标"
                @click="handleRefreshIcon(row)"
              />
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
        <el-table-column label="URL" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tooltip :content="truncateUrl(row.url)" placement="top">
              <span class="url-cell">{{ truncateUrl(row.url) }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="宽度" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="widthTagType(row.width)" size="small">{{ widthLabel(row.width) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="密度" width="110" align="center">
          <template #default="{ row }">
            <el-tag :type="displayModeTagType(row.display_mode)" size="small" effect="light">{{ displayModeLabel(row.display_mode) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="高度(px)" width="96" align="center">
          <template #default="{ row }">
            {{ row.height_px ? row.height_px : '—' }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              :active-value="1"
              :inactive-value="0"
              @change="(val: number) => handleToggleEnabled(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" size="small" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" :icon="Delete" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无导航配置</p>
          </div>
        </template>
      </el-table>
    </div>

    <!-- 新建/编辑导航对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑导航链接' : '新建导航链接'"
      width="620px"
      class="custom-dialog"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <!-- 图标预览区：新建时用占位，编辑时用已有 icon -->
      <div class="icon-preview" v-if="form.name || form.url || (isEditing && currentRowIcon)">
        <div class="icon-preview-img" :style="form.icon_data ? '' : { backgroundColor: form.icon_data ? '' : '#f46800' }">
          <img v-if="form.icon_data" :src="form.icon_data" alt="" />
          <span v-else>{{ previewInitial(form.name, form.url) }}</span>
        </div>
        <div class="icon-preview-info">
          <div class="icon-preview-name">{{ form.name || '新导航' }}</div>
          <div class="icon-preview-url">{{ form.url || '—' }}</div>
          <div class="icon-preview-sub">
            <span v-if="form.icon_data">✓ 已获取网站图标</span>
            <span v-else class="icon-pending">图标将在保存后自动获取</span>
            <span v-if="isEditing && editingId !== null" class="refresh-link" @click="handleRefreshIconOnForm">🔄 立即刷新</span>
          </div>
        </div>
      </div>

      <el-form :model="form" label-width="100px" label-position="right" class="dialog-form" style="margin-top: 16px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：Grafana 监控大盘" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="URL" required>
          <el-input
            v-model="form.url"
            placeholder="https://grafana.example.com"
            maxlength="2048"
            show-word-limit
          />
          <div class="form-hint">必须以 http:// 或 https:// 开头，Dashboard 点击后将在新标签页打开</div>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选，banner 卡片模式下会展示" />
        </el-form-item>
        <el-form-item label="宽度" required>
          <el-radio-group v-model="form.width">
            <el-radio value="third">1/3</el-radio>
            <el-radio value="half">1/2</el-radio>
            <el-radio value="full">整行</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="卡片密度" required>
          <el-radio-group v-model="form.display_mode">
            <el-radio value="compact">紧凑</el-radio>
            <el-radio value="normal">标准</el-radio>
            <el-radio value="banner">大卡片</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="高度微调">
          <el-input-number
            v-model="form.height_px"
            :min="0"
            :max="200"
            :step="4"
            controls-position="right"
            placeholder="留空或 0 使用默认"
            style="width: 140px"
          />
          <span class="form-hint">留空/0 时自动使用密度默认高度（紧凑 52、标准 68、大卡片 100）</span>
        </el-form-item>
        <el-form-item label="排序号">
          <el-input-number v-model="form.sort_order" :min="0" :step="1" />
          <span class="form-hint">越小越靠前</span>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
        </el-form-item>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * NavigationLinks.vue - 导航快捷链接管理页面
 *
 * 配置 Dashboard 首页展示的外部网站快捷入口列表，
 * 支持启用/禁用、排序、宽度/密度/高度微调、favicon 自动抓取和手动刷新。
 * 首页只展示 enabled=1 的记录。
 */
// ========== Vue 核心 API ==========
import { ref, computed, onMounted } from 'vue'
// ========== Element Plus ==========
import { ElMessage, ElMessageBox } from 'element-plus'
// ========== Element Plus 图标 ==========
import { Delete, Plus, Search, Refresh, Edit, Check, RefreshRight, Link as LinkIcon, Document } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { navigationAPI } from '@/api'
// ========== 工具函数 ==========
import { isHandledError } from '@/utils/api'
import { formatErrorMessage } from '@/utils/error'
// ========== 类型定义 ==========
import type { NavigationLink, NavigationLinkInput } from '@/types'

// ========== 列表状态 ==========
const list = ref<NavigationLink[]>([])
const loading = ref(false)
const searchQuery = ref('')
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)
const reordering = ref(false)
const hasSortChanges = ref(false)

/** 记录哪些行正在执行"刷新图标"操作（避免重复点击） */
const refreshingIds = ref<Set<number>>(new Set())

/** 编辑时保留原记录的 icon_data，用于表单回填预览 */
const currentRowIcon = ref<string | null>(null)

const form = ref<NavigationLinkInput & { icon_data: string | null }>({
  name: '',
  description: '',
  url: '',
  width: 'third',
  display_mode: 'normal',
  height_px: null,
  sort_order: 0,
  enabled: 1,
  icon_data: null,
})

/** 按搜索关键字过滤（匹配 name 或 url） */
const filteredList = computed(() => {
  if (!searchQuery.value) return list.value
  const q = searchQuery.value.toLowerCase()
  return list.value.filter(
    (item) => item.name.toLowerCase().includes(q) || item.url.toLowerCase().includes(q),
  )
})

/** 加载所有导航配置 */
const loadList = async () => {
  loading.value = true
  try {
    const response = await navigationAPI.listAll()
    list.value = response.data?.items || []
    hasSortChanges.value = false
  } catch (error: unknown) {
    if (!isHandledError(error)) {
      ElMessage.error(formatErrorMessage(error))
    }
  } finally {
    loading.value = false
  }
}

/** 新建弹窗初始化 */
const handleCreate = () => {
  isEditing.value = false
  editingId.value = null
  currentRowIcon.value = null
  form.value = {
    name: '',
    description: '',
    url: '',
    width: 'third',
    display_mode: 'normal',
    height_px: null,
    sort_order: 0,
    enabled: 1,
    icon_data: null,
  }
  dialogVisible.value = true
}

/** 编辑弹窗初始化 */
const handleEdit = (row: NavigationLink) => {
  isEditing.value = true
  editingId.value = row.id
  currentRowIcon.value = row.icon_data
  form.value = {
    name: row.name,
    description: row.description || '',
    url: row.url,
    width: row.width,
    display_mode: row.display_mode,
    height_px: row.height_px,
    sort_order: row.sort_order,
    enabled: row.enabled,
    icon_data: row.icon_data,
  }
  dialogVisible.value = true
}

/** 表单校验 */
const validateForm = (): string | null => {
  if (!form.value.name.trim()) return '名称为必填项'
  if (form.value.name.length > 100) return '名称最多 100 字符'
  if (!form.value.url.trim()) return 'URL 为必填项'
  if (!/^https?:\/\//i.test(form.value.url)) return 'URL 必须以 http:// 或 https:// 开头'
  if (form.value.url.length > 2048) return 'URL 最长 2048 字符'
  if (!['third', 'half', 'full'].includes(form.value.width)) return '宽度值无效'
  if (!['compact', 'normal', 'banner'].includes(form.value.display_mode)) return '密度值无效'
  if (form.value.height_px !== null && form.value.height_px !== 0) {
    if (form.value.height_px < 40 || form.value.height_px > 200) return '高度微调范围 40-200（留空或 0 使用默认）'
  }
  if (form.value.sort_order < 0) return '排序号必须 >= 0'
  return null
}

/** 表单提交 */
const handleSubmit = async () => {
  const err = validateForm()
  if (err) {
    ElMessage.warning(err)
    return
  }

  submitting.value = true
  try {
    // height_px: 0 当作 null 发送
    const heightPx = form.value.height_px === 0 ? null : form.value.height_px
    const data: NavigationLinkInput = {
      name: form.value.name.trim(),
      description: form.value.description?.trim() || '',
      url: form.value.url.trim(),
      width: form.value.width,
      display_mode: form.value.display_mode,
      height_px: heightPx,
      sort_order: form.value.sort_order,
      enabled: form.value.enabled,
    }

    if (isEditing.value && editingId.value !== null) {
      await navigationAPI.update(editingId.value, data)
      ElMessage.success('导航更新成功')
    } else {
      await navigationAPI.create(data)
      ElMessage.success('导航创建成功，正在后台获取图标')
    }
    dialogVisible.value = false
    await loadList()
  } catch (error: unknown) {
    if (!isHandledError(error)) {
      ElMessage.error(formatErrorMessage(error))
    }
  } finally {
    submitting.value = false
  }
}

/** 删除 */
const handleDelete = async (row: NavigationLink) => {
  try {
    await ElMessageBox.confirm(`确认删除导航「${row.name}」？`, '删除确认', { type: 'warning' })
    await navigationAPI.delete(row.id)
    ElMessage.success('删除成功')
    await loadList()
  } catch (error: unknown) {
    if (error === 'cancel') return
    if (!isHandledError(error)) {
      ElMessage.error(formatErrorMessage(error))
    }
  }
}

/** 切换启用/禁用（失败自动回滚） */
const handleToggleEnabled = async (row: NavigationLink, newVal: number) => {
  try {
    await navigationAPI.update(row.id, {
      name: row.name,
      description: row.description,
      url: row.url,
      width: row.width,
      display_mode: row.display_mode,
      height_px: row.height_px,
      sort_order: row.sort_order,
      enabled: newVal,
    })
    ElMessage.success(newVal === 1 ? '已启用' : '已禁用')
  } catch (error: unknown) {
    row.enabled = newVal === 1 ? 0 : 1
    if (!isHandledError(error)) {
      ElMessage.error(formatErrorMessage(error))
    }
  }
}

/** 手动刷新图标（列表行） */
const handleRefreshIcon = async (row: NavigationLink) => {
  if (refreshingIds.value.has(row.id)) return
  refreshingIds.value.add(row.id)
  try {
    const response = await navigationAPI.refreshIcon(row.id)
    const updated = response.data
    if (updated) {
      row.icon_data = updated.icon_data
      row.icon_fetched_at = updated.icon_fetched_at
      ElMessage.success('图标已刷新')
    }
  } catch (error: unknown) {
    if (!isHandledError(error)) {
      ElMessage.error(formatErrorMessage(error))
    }
  } finally {
    refreshingIds.value.delete(row.id)
  }
}

/** 手动刷新图标（编辑弹窗内） */
const handleRefreshIconOnForm = async () => {
  if (editingId.value === null) return
  const response = await navigationAPI.refreshIcon(editingId.value)
  const updated = response.data
  if (updated) {
    form.value.icon_data = updated.icon_data
    ElMessage.success('图标已刷新')
  }
}

/** 批量保存排序 */
const markSortChanged = () => {
  hasSortChanges.value = true
}
const handleSaveSortOrder = async () => {
  reordering.value = true
  try {
    const items = list.value.map((item) => ({ id: item.id, sort_order: item.sort_order }))
    await navigationAPI.reorder(items)
    ElMessage.success('排序已保存')
    hasSortChanges.value = false
    await loadList()
  } catch (error: unknown) {
    if (!isHandledError(error)) {
      ElMessage.error(formatErrorMessage(error))
    }
  } finally {
    reordering.value = false
  }
}

// ========== 工具函数 ==========

const truncateUrl = (url: string): string => {
  try {
    const u = new URL(url)
    return u.origin + u.pathname
  } catch {
    return url
  }
}

const hostnameOf = (url: string): string => {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return ''
  }
}

/** fallback 首字母（URL 无效时用 name） */
const fallbackInitial = (url: string, name?: string): string => {
  const host = hostnameOf(url)
  if (host) return host.charAt(0).toUpperCase()
  if (name) return name.charAt(0).toUpperCase()
  return '?'
}

const FALLBACK_COLORS = ['#f46800', '#e6522c', '#00bfb3', '#24292f', '#d33833', '#0052cc', '#9333ea']

/** 域名 hash 稳定取色 */
const fallbackColor = (url: string): string => {
  const host = hostnameOf(url) || url
  let h = 0
  for (const c of host) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return FALLBACK_COLORS[h % FALLBACK_COLORS.length]
}

/** 弹窗预览用的初始首字母 */
const previewInitial = (name: string, url: string): string => {
  if (name) return name.charAt(0).toUpperCase()
  return fallbackInitial(url)
}

const widthLabel = (w: string) => ({ third: '1/3', half: '1/2', full: '整行' }[w] ?? w)
const widthTagType = (w: string) => ({ third: 'info', half: 'warning', full: 'success' } as const)[w] ?? 'info'

const displayModeLabel = (m: string) => ({ compact: '紧凑', normal: '标准', banner: '大卡片' }[m] ?? m)
const displayModeTagType = (m: string) => ({ compact: 'info', normal: '', banner: 'danger' } as const)[m] ?? 'info'

onMounted(() => {
  loadList()
})
</script>

<style scoped>
/* ========== 页面整体容器 ========== */
.navigation-links-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.navigation-links-page::-webkit-scrollbar {
  width: 8px;
}

.navigation-links-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.navigation-links-page::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

/* ========== 特有工具类 ========== */
.url-cell {
  color: var(--el-text-color-secondary);
  font-family: var(--font-mono);
  font-size: 12px;
}

.form-hint {
  margin-left: var(--space-2);
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

/* 表格中的图标缩略图 */
.icon-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.icon-thumb {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  background: #e5e7eb;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-weight: 700;
  font-size: 12px;
  font-family: sans-serif;
  overflow: hidden;
}
.icon-thumb img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

/* 弹窗里的图标预览卡片 */
.icon-preview {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  border: 1px solid var(--el-border-color-lighter);
}
.icon-preview-img {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  background: #f46800;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 18px;
  font-weight: 700;
  flex-shrink: 0;
  overflow: hidden;
}
.icon-preview-img img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.icon-preview-info {
  flex: 1;
  min-width: 0;
}
.icon-preview-name {
  font-weight: 600;
  font-size: 14px;
  color: var(--el-text-color-primary);
}
.icon-preview-url {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-family: var(--font-mono);
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.icon-preview-sub {
  margin-top: 4px;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
.icon-pending {
  color: var(--el-color-warning);
}
.refresh-link {
  color: var(--el-color-primary);
  cursor: pointer;
  margin-left: 8px;
}
.refresh-link:hover {
  text-decoration: underline;
}
</style>
