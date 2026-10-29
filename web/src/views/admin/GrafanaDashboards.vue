<!-- Grafana 仪表盘嵌入管理页面：配置要嵌入首页的 Grafana Dashboard URL 和排序 -->
<template>
  <div class="grafana-management">
    <!-- 页面工具栏：搜索 + 刷新 + 创建按钮 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索名称或URL"
          clearable
          style="width: 260px"
          :prefix-icon="Search"
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadList">刷新</el-button>
        <el-button
          v-if="hasSortChanges"
          type="warning"
          :icon="Check"
          :loading="reordering"
          @click="handleSaveSortOrder"
        >
          保存排序
        </el-button>
        <el-button type="primary" :icon="Plus" @click="handleCreate">新建图表</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="filteredList" v-loading="loading" stripe border>
        <el-table-column label="排序" width="100" align="center">
          <template #default="{ row }">
            <el-input-number
              v-model="row.sort_order"
              :min="0"
              :step="1"
              size="small"
              controls-position="right"
              style="width: 80px"
              @change="markSortChanged"
            />
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" show-overflow-tooltip />
        <el-table-column label="嵌入 URL" min-width="240" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tooltip :content="truncateUrl(row.embed_url)" placement="top">
              <span class="url-cell">{{ truncateUrl(row.embed_url) }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="宽度" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="widthTagType(row.width)" size="small">{{ widthLabel(row.width) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="height_px" label="高度(px)" width="100" align="center" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-switch
              v-model="row.enabled"
              :active-value="1"
              :inactive-value="0"
              @change="(val: number) => handleToggleEnabled(row, val)"
            />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right" align="center">
          <template #default="{ row }">
            <el-button link type="primary" size="small" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
            <el-button link type="danger" size="small" :icon="Delete" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无 Grafana 图表配置" />
        </template>
      </el-table>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑图表' : '新建图表'"
      width="640px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      >
        若使用 URL 带 cookie/token 方式嵌入，请确保 Grafana 链接使用最小权限的只读 Token，并定期轮换。该链接将通过 iframe 在所有登录用户可见的仪表盘加载。
      </el-alert>

      <el-form :model="form" label-width="100px" label-position="right">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：QPS 监控" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选说明" />
        </el-form-item>
        <el-form-item label="嵌入 URL" required>
          <el-input
            v-model="form.embed_url"
            type="textarea"
            :rows="3"
            placeholder="粘贴 Grafana 分享链接（必须以 http:// 或 https:// 开头）"
          />
        </el-form-item>
        <el-form-item label="宽度" required>
          <el-radio-group v-model="form.width">
            <el-radio value="third">1/3</el-radio>
            <el-radio value="half">1/2</el-radio>
            <el-radio value="full">整行</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="高度(px)" required>
          <el-input-number v-model="form.height_px" :min="120" :max="1200" :step="10" />
          <span class="form-hint">范围 120-1200</span>
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
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * GrafanaDashboards.vue - Grafana 仪表盘嵌入管理页面
 *
 * 配置要嵌入到首页 Dashboard 的 Grafana Panel URL 列表，
 * 支持启用/禁用、排序、宽高调整等。首页只展示 enabled=1 的记录。
 */
// ========== Vue 核心 API ==========
import { ref, computed, onMounted } from 'vue'
// ========== Element Plus ==========
import { ElMessage, ElMessageBox } from 'element-plus'
// ========== Element Plus 图标 ==========
import { Delete, Plus, Search, Refresh, Edit, Check } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { grafanaAPI } from '@/api'
// ========== 工具函数 ==========
import { isHandledError } from '@/utils/api'
// ========== 类型定义 ==========
import type { GrafanaDashboard, GrafanaDashboardInput } from '@/types'

// ========== 列表状态 ==========
const list = ref<GrafanaDashboard[]>([])             // 所有 Grafana Dashboard 配置列表
const loading = ref(false)                            // 列表加载状态
const searchQuery = ref('')                           // 搜索关键字
const dialogVisible = ref(false)                      // 编辑弹窗可见性
const isEditing = ref(false)                          // 是否编辑模式
const editingId = ref<number | null>(null)            // 当前编辑的图表 ID
const submitting = ref(false)                         // 表单提交加载状态
const reordering = ref(false)                         // 排序保存加载状态
const hasSortChanges = ref(false)                     // 是否有排序变更未保存

const form = ref<GrafanaDashboardInput>({
  name: '',
  description: '',
  embed_url: '',
  width: 'full',
  height_px: 300,
  sort_order: 0,
  enabled: 1,
})

/** 按搜索关键字过滤图表列表（匹配 name 或 embed_url） */
const filteredList = computed(() => {
  if (!searchQuery.value) return list.value
  const q = searchQuery.value.toLowerCase()
  return list.value.filter(
    (item) => item.name.toLowerCase().includes(q) || item.embed_url.toLowerCase().includes(q),
  )
})

/** 加载所有 Grafana Dashboard 配置 */
const loadList = async () => {
  loading.value = true
  try {
    const response = await grafanaAPI.listAll()
    list.value = response.data?.items || []
    hasSortChanges.value = false
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error(error.message || '加载 Grafana 图表列表失败')
    }
  } finally {
    loading.value = false
  }
}

/**
 * 打开创建 Grafana 面板对话框，初始化表单默认值（全宽、300px 高、排序 0、已启用）
 */
const handleCreate = () => {
  isEditing.value = false
  editingId.value = null
  form.value = {
    name: '',
    description: '',
    embed_url: '',
    width: 'full',
    height_px: 300,
    sort_order: 0,
    enabled: 1,
  }
  dialogVisible.value = true
}

/**
 * 打开编辑对话框，回填图表数据
 * @param {GrafanaDashboard} row - 当前行的图表数据
 */
const handleEdit = (row: GrafanaDashboard) => {
  isEditing.value = true
  editingId.value = row.id
  form.value = {
    name: row.name,
    description: row.description || '',
    embed_url: row.embed_url,
    width: row.width,
    height_px: row.height_px,
    sort_order: row.sort_order,
    enabled: row.enabled,
  }
  dialogVisible.value = true
}

/**
 * 校验表单字段有效性
 * @returns {string | null} 错误信息，校验通过返回 null
 */
const validateForm = (): string | null => {
  if (!form.value.name.trim()) return '名称为必填项'
  if (form.value.name.length > 100) return '名称最多 100 字符'
  if (!form.value.embed_url.trim()) return '嵌入 URL 为必填项'
  if (!/^https?:\/\//i.test(form.value.embed_url)) return '嵌入 URL 必须以 http:// 或 https:// 开头'
  if (form.value.embed_url.length > 2048) return '嵌入 URL 最长 2048 字符'
  if (!['third', 'half', 'full'].includes(form.value.width)) return '宽度值无效'
  if (form.value.height_px < 120 || form.value.height_px > 1200) return '高度范围 120-1200'
  if (form.value.sort_order < 0) return '排序号必须 >= 0'
  return null
}

/**
 * 提交表单，根据 isEditing 决定创建或更新图表
 * @returns {Promise<void>}
 */
const handleSubmit = async () => {
  const err = validateForm()
  if (err) {
    ElMessage.warning(err)
    return
  }

  submitting.value = true
  try {
    const data: GrafanaDashboardInput = { ...form.value }
    if (isEditing.value && editingId.value !== null) {
      await grafanaAPI.update(editingId.value, data)
      ElMessage.success('图表更新成功')
    } else {
      await grafanaAPI.create(data)
      ElMessage.success('图表创建成功')
    }
    dialogVisible.value = false
    await loadList()
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error(error.message || '保存失败')
    }
  } finally {
    submitting.value = false
  }
}

/**
 * 删除指定 Grafana 图表，弹出确认框后调用后端
 * @param {GrafanaDashboard} row - 要删除的图表数据
 * @returns {Promise<void>}
 */
const handleDelete = async (row: GrafanaDashboard) => {
  try {
    await ElMessageBox.confirm(`确认删除图表「${row.name}」？`, '删除确认', {
      type: 'warning',
    })
    await grafanaAPI.delete(row.id)
    ElMessage.success('删除成功')
    await loadList()
  } catch (error: any) {
    if (error === 'cancel') return
    if (!isHandledError(error)) {
      ElMessage.error(error.message || '删除失败')
    }
  }
}

/**
 * 切换图表启用/禁用状态，失败时自动回滚 UI
 * @param {GrafanaDashboard} row - 图表数据
 * @param {number} newVal - 新的启用状态（1=启用，0=禁用）
 * @returns {Promise<void>}
 */
const handleToggleEnabled = async (row: GrafanaDashboard, newVal: number) => {
  try {
    await grafanaAPI.update(row.id, {
      name: row.name,
      description: row.description,
      embed_url: row.embed_url,
      width: row.width,
      height_px: row.height_px,
      sort_order: row.sort_order,
      enabled: newVal,
    })
    ElMessage.success(newVal === 1 ? '已启用' : '已禁用')
  } catch (error: any) {
    // 失败时回滚
    row.enabled = newVal === 1 ? 0 : 1
    if (!isHandledError(error)) {
      ElMessage.error(error.message || '切换状态失败')
    }
  }
}

/**
 * 标记排序已变更，用于"保存排序"按钮的 disabled 判断
 */
const markSortChanged = () => {
  hasSortChanges.value = true
}

/**
 * 批量保存图表排序号到后端
 * @returns {Promise<void>}
 */
const handleSaveSortOrder = async () => {
  reordering.value = true
  try {
    const items = list.value.map((item) => ({ id: item.id, sort_order: item.sort_order }))
    await grafanaAPI.reorder(items)
    ElMessage.success('排序已保存')
    hasSortChanges.value = false
    await loadList()
  } catch (error: any) {
    if (!isHandledError(error)) {
      ElMessage.error(error.message || '保存排序失败')
    }
  } finally {
    reordering.value = false
  }
}

/**
 * 截断 URL 仅显示 origin + pathname，隐藏 query string（可能含 token）
 * @param {string} url - 完整 URL
 * @returns {string} 截断后的 URL
 */
const truncateUrl = (url: string): string => {
  try {
    const u = new URL(url)
    return u.origin + u.pathname
  } catch {
    return url
  }
}

/**
 * 将图表宽度代码映射为中文显示文本
 * @param {string} width - 宽度代码（third/half/full）
 * @returns {string} 中文标签
 */
const widthLabel = (width: string): string => {
  switch (width) {
    case 'third': return '1/3'
    case 'half': return '1/2'
    case 'full': return '整行'
    default: return width
  }
}

/**
 * 根据图表宽度返回对应的 Element Plus 标签颜色类型
 * @param {string} width - 宽度代码
 * @returns {'info' | 'warning' | 'success'} 标签类型
 */
const widthTagType = (width: string): 'info' | 'warning' | 'success' => {
  switch (width) {
    case 'third': return 'info'
    case 'half': return 'warning'
    case 'full': return 'success'
    default: return 'info'
  }
}

onMounted(() => {
  loadList()
})
</script>

<style scoped>
.grafana-management {
  padding: 16px;
}

.page-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.toolbar-right {
  display: flex;
  gap: 8px;
}

.url-cell {
  color: var(--el-text-color-secondary);
  font-family: monospace;
  font-size: 12px;
}

.form-hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
</style>
