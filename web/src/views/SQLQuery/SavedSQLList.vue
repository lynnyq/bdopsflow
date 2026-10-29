<!-- 已保存 SQL 列表页面：展示用户收藏/共享的 SQL 语句，支持重新执行 -->
<template>
  <div class="saved-sql-page">
    <!-- 工具栏：搜索 + 刷新 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索已保存SQL..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadSavedSQL" :loading="loading" class="refresh-btn">刷新</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="savedList" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" :min-width="150" show-overflow-tooltip />
        <el-table-column label="数据源" width="150">
          <template #default="{ row }">
            {{ row.datasource_name || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="保存用户" width="120" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.created_by_name" type="info" effect="plain" size="small">
              {{ row.created_by_name }}
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="SQL 摘要" :min-width="250" show-overflow-tooltip>
          <template #default="{ row }">
            <code class="sql-snippet">{{ row.sql_text }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="is_public" label="公开" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_public ? 'success' : 'info'" effect="light" size="small">
              {{ row.is_public ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleLoad(row)">
              <el-icon><VideoPlay /></el-icon> 加载
            </el-button>
            <el-button type="warning" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无已保存SQL</p>
          </div>
        </template>
      </el-table>
    </div>

    <div v-if="total > 0" class="pagination-container">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        :pager-count="5"
        @current-change="loadSavedSQL"
        @size-change="loadSavedSQL"
      />
    </div>

    <!-- 编辑对话框 -->
    <el-dialog
      v-model="editDialogVisible"
      title="编辑已保存SQL"
      width="560px"
      :close-on-click-modal="false"
    >
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-position="top" class="edit-form">
        <el-form-item label="名称" prop="name">
          <el-input v-model="editForm.name" placeholder="请输入SQL名称" />
        </el-form-item>
        <el-form-item label="数据源" prop="datasource_id">
          <el-select v-model="editForm.datasource_id" placeholder="请选择数据源" filterable style="width: 100%">
            <el-option
              v-for="ds in datasourceList"
              :key="ds.id"
              :label="ds.name"
              :value="ds.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="数据库">
          <el-input v-model="editForm.database" placeholder="可选，留空使用默认数据库" />
        </el-form-item>
        <el-form-item label="SQL 语句" prop="sql_text">
          <el-input
            v-model="editForm.sql_text"
            type="textarea"
            :rows="6"
            placeholder="请输入SQL语句"
            class="sql-textarea"
          />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="editForm.description" type="textarea" :rows="2" placeholder="请输入描述" />
        </el-form-item>
        <el-form-item label="公开">
          <el-switch v-model="editForm.is_public" />
          <span class="form-hint">公开的 SQL 可被同领域其他用户使用</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="editDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleConfirmEdit" :loading="editing">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * SavedSQLList.vue - 已保存 SQL 列表页面
 *
 * 展示用户在 SQLQuery 页面收藏（保存）的 SQL 语句，
 * 包含公开/私有标记、保存者、数据源等信息，支持跳转到 SQLQuery 重新执行。
 *
 * Props: 无
 * Emits: 无
 */
// ========== Vue 核心 API ==========
import { ref, onMounted, watch } from 'vue'
// ========== Vue Router ==========
import { useRouter } from 'vue-router'
// ========== Element Plus ==========
import { ElMessage, ElMessageBox } from 'element-plus'
// ========== Element Plus 图标 ==========
import { Search, Refresh, Delete, Document, VideoPlay, Edit } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { queryAPI, datasourceAPI } from '@/api'
// ========== 工具函数 ==========
import { isHandledError } from '@/utils/api'
// ========== 类型定义 ==========
import type { SavedSQL, Datasource } from '@/types'

// Router 实例：用于跳转到 SQLQuery 页面
const router = useRouter()

// ========== 列表状态 ==========
const savedList = ref<SavedSQL[]>([])          // 已保存 SQL 列表
const loading = ref(false)                      // 列表加载状态
const searchQuery = ref('')                     // 搜索关键字
const currentPage = ref(1)                      // 当前页码
const pageSize = ref(20)                        // 每页条数
const total = ref(0)                            // 总记录数

// ========== 编辑弹窗状态 ==========
const editDialogVisible = ref(false)            // 编辑对话框可见性
const editing = ref(false)                       // 保存加载状态
const editingId = ref<number>(0)                // 当前编辑的 SQL ID
const editFormRef = ref()                       // 编辑表单 DOM 引用
const editForm = ref({
  name: '',            // SQL 名称
  datasource_id: 0 as number,  // 关联数据源 ID
  database: '',        // 目标数据库
  sql_text: '',        // SQL 文本
  description: '',     // 描述备注
  is_public: false     // 是否公开
})
// 编辑表单校验规则
const editRules = {
  name: [
    { required: true, message: '请输入名称', trigger: 'blur' },
    { min: 1, max: 100, message: '名称长度在1到100个字符', trigger: 'blur' }
  ],
  datasource_id: [
    { required: true, message: '请选择数据源', trigger: 'change' }
  ],
  sql_text: [
    { required: true, message: '请输入SQL语句', trigger: 'blur' }
  ]
}
// 数据源下拉选项列表（用于编辑弹窗）
const datasourceList = ref<Datasource[]>([])

/**
 * 格式化日期时间为中文本地化格式
 * @param dateStr - ISO 日期字符串
 * @returns 格式化后的日期时间字符串，空值返回 '-'
 */
const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

/**
 * 加载已保存 SQL 列表（支持分页和搜索）
 */
const loadSavedSQL = async () => {
  loading.value = true
  try {
    const res = await queryAPI.listSavedSQL({
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchQuery.value || undefined,
    })
    savedList.value = res.data.items || []
    total.value = res.data.total || 0
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载已保存SQL列表失败')
    }
  } finally {
    loading.value = false
  }
}

// searchQuery 变化：防抖后重置到第 1 页并重新请求后端
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
watch(searchQuery, () => {
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }
  searchDebounceTimer = setTimeout(() => {
    currentPage.value = 1
    loadSavedSQL()
  }, 300)
})

/**
 * 加载数据源列表（用于编辑弹窗的数据源下拉选择）
 */
const loadDatasources = async () => {
  try {
    const res = await datasourceAPI.list({ page: 1, page_size: 200 })
    datasourceList.value = res.data.items || []
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载数据源列表失败')
    }
  }
}

/**
 * 加载已保存 SQL 到 SQLQuery 页面重新执行
 * @param row - 选中的已保存 SQL 行数据
 */
const handleLoad = (row: SavedSQL) => {
  const query: Record<string, string> = { sql: row.sql_text }
  if (row.datasource_id != null) {
    query.datasource_id = String(row.datasource_id)
  }
  if (row.database) {
    query.database = row.database
  }
  router.push({
    name: 'SQLQuery',
    query,
  })
}

/**
 * 打开编辑弹窗，填充表单数据
 * @param row - 要编辑的已保存 SQL
 */
const handleEdit = (row: SavedSQL) => {
  editingId.value = row.id
  editForm.value = {
    name: row.name,
    datasource_id: row.datasource_id,
    database: row.database || '',
    sql_text: row.sql_text,
    description: row.description || '',
    is_public: row.is_public
  }
  editDialogVisible.value = true
}

/**
 * 确认保存编辑后的 SQL（表单校验后调用后端 updateSavedSQL）
 */
const handleConfirmEdit = async () => {
  if (!editFormRef.value) return

  const valid = await editFormRef.value.validate()
  if (!valid) return

  editing.value = true
  try {
    await queryAPI.updateSavedSQL(editingId.value, {
      name: editForm.value.name,
      datasource_id: editForm.value.datasource_id,
      database: editForm.value.database,
      sql_text: editForm.value.sql_text,
      description: editForm.value.description,
      is_public: editForm.value.is_public
    })
    ElMessage.success('更新成功')
    editDialogVisible.value = false
    await loadSavedSQL()
  } catch (err: any) {
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '更新失败')
    }
  } finally {
    editing.value = false
  }
}

/**
 * 删除已保存 SQL（弹窗二次确认）
 * @param row - 要删除的已保存 SQL
 */
const handleDelete = async (row: SavedSQL) => {
  try {
    await ElMessageBox.confirm(`确定要删除 "${row.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await queryAPI.deleteSavedSQL(row.id)
    ElMessage.success('已删除')
    await loadSavedSQL()
  } catch (err: any) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(err.message || '删除失败')
    }
  }
}

onMounted(() => {
  loadSavedSQL()
  loadDatasources()
})
</script>

<style scoped>
.saved-sql-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.search-input {
  width: 280px;
}

.search-input :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.search-input :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.search-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.refresh-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 16px;
}

.refresh-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.table-wrapper {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

:deep(.el-table) {
  border-radius: var(--radius-lg);
}

:deep(.el-table--border::after),
:deep(.el-table--group::after),
:deep(.el-table::before) {
  display: none;
}

:deep(.el-table tr) {
  transition: background-color var(--duration-normal) var(--ease-out);
}

:deep(.el-table__row:hover) {
  background-color: var(--bg-secondary) !important;
}

.sql-snippet {
  font-family: var(--font-mono);
  font-size: 12px;
  background: var(--bg-secondary);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-secondary);
}

.table-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-8);
  gap: var(--space-3);
  color: var(--text-muted);
}

.table-empty-state .el-icon {
  opacity: 0.4;
}

.table-empty-state p {
  margin: 0;
  font-size: 0.875rem;
}

.pagination-container {
  display: flex;
  justify-content: center;
  padding: var(--space-6);
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-sm);
  margin-top: var(--space-4);
}

.pagination-container :deep(.el-pager li) {
  border-radius: var(--radius-md);
  margin: 0 4px;
  transition: all var(--duration-normal) var(--ease-out);
  font-weight: 500;
  height: 36px;
  min-width: 36px;
  line-height: 36px;
}

.pagination-container :deep(.el-pager li.is-active) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  color: white;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

.edit-form .form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.sql-textarea :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 13px;
}
</style>
