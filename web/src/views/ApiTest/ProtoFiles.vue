<template>
  <div class="proto-files-page">
    <div class="page-toolbar">
      <div class="toolbar-left">
        <h3 class="page-title">Proto 文件管理</h3>
      </div>
      <div class="toolbar-right">
        <el-input
          v-model="searchQuery"
          placeholder="搜索文件名/包名"
          :prefix-icon="Search"
          clearable
          size="default"
          class="search-input"
        />
        <el-button :icon="Refresh" @click="loadProtoFiles" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Upload" @click="handleUpload" class="create-btn">上传</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="filteredProtoFiles" v-loading="loading" stripe height="100%">
        <el-table-column prop="name" label="文件名" :min-width="200" show-overflow-tooltip />
        <el-table-column label="包名" :min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            {{ getPackageName(row) || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="服务数" width="100" align="center">
          <template #default="{ row }">
            {{ getServiceCount(row) }}
          </template>
        </el-table-column>
        <el-table-column label="文件大小" width="120" align="center">
          <template #default="{ row }">
            <span class="file-size">{{ formatFileSize(row.content) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="依赖" :min-width="160" show-overflow-tooltip>
          <template #default="{ row }">
            {{ getDependencyNames(row) || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="上传时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="success" link size="small" @click="handleParse(row)">
              <el-icon><View /></el-icon> 解析
            </el-button>
            <el-button type="primary" link size="small" @click="handleEdit(row)">
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
            <p>暂无 Proto 文件</p>
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
        @current-change="loadProtoFiles"
        @size-change="handleSizeChange"
      />
    </div>

    <!-- Upload / Edit Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑 Proto 文件' : '上传 Proto 文件'"
      width="720px"
      :close-on-click-modal="false"
      @close="resetForm"
    >
      <el-form :model="formData" :rules="formRules" label-width="90px" label-position="top">
        <el-form-item label="文件名" prop="name">
          <el-input v-model="formData.name" placeholder="例如: helloworld.proto" :disabled="isEditing" />
        </el-form-item>
        <el-form-item label="内容" prop="content">
          <div class="proto-editor-wrapper">
            <div class="line-numbers" ref="lineNumbersRef">
              <span v-for="n in contentLineCount" :key="n" class="line-number">{{ n }}</span>
            </div>
            <el-input
              v-model="formData.content"
              type="textarea"
              :rows="18"
              placeholder="粘贴 Proto 文件内容，或点击下方按钮选择文件"
              class="proto-textarea"
              @scroll="syncLineScroll"
            />
          </div>
          <div class="file-upload-hint">
            <el-upload
              :auto-upload="false"
              :show-file-list="false"
              accept=".proto"
              :on-change="handleFileSelect"
            >
              <el-button size="small" :icon="UploadFilled">选择 .proto 文件</el-button>
            </el-upload>
            <el-button
              size="small"
              :icon="Check"
              :loading="verifying"
              :disabled="!formData.content.trim()"
              @click="handleVerify"
            >
              验证
            </el-button>
          </div>
          <!-- Verify result preview -->
          <div v-if="verifyResult" class="verify-result">
            <div class="verify-header">
              <el-icon color="var(--el-color-success)"><CircleCheck /></el-icon>
              <span>解析成功</span>
            </div>
            <div v-if="verifyResult.package" class="verify-item">
              <span class="verify-label">包名:</span> {{ verifyResult.package }}
            </div>
            <div v-if="verifyResult.services?.length" class="verify-item">
              <span class="verify-label">服务:</span>
              <span v-for="svc in verifyResult.services" :key="svc.name" class="verify-service-tag">
                {{ svc.name }} ({{ svc.methods.length }} 方法)
              </span>
            </div>
          </div>
          <div v-if="verifyError" class="verify-result verify-error">
            <div class="verify-header">
              <el-icon color="var(--el-color-danger)"><CircleClose /></el-icon>
              <span>解析失败</span>
            </div>
            <div class="verify-item">{{ verifyError }}</div>
          </div>
        </el-form-item>
        <el-form-item label="依赖文件">
          <el-select
            v-model="formData.dependencies"
            multiple
            placeholder="选择依赖的 Proto 文件"
            class="dep-select"
          >
            <el-option
              v-for="file in dependencyOptions"
              :key="file.id"
              :label="file.name"
              :value="file.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">确认</el-button>
      </template>
    </el-dialog>

    <!-- Parse Result Dialog -->
    <el-dialog
      v-model="parseDialogVisible"
      :title="`解析结果 - ${parseDialogFileName}`"
      width="640px"
    >
      <div v-if="parseDialogData" class="parse-result-content">
        <div v-if="parseDialogData.package" class="parse-section">
          <div class="parse-section-title">包名</div>
          <el-tag type="info" effect="plain">{{ parseDialogData.package }}</el-tag>
        </div>
        <div v-if="parseDialogData.services?.length" class="parse-section">
          <div class="parse-section-title">服务</div>
          <div class="parse-tree">
            <div v-for="svc in parseDialogData.services" :key="svc.name" class="parse-tree-node">
              <div class="parse-tree-service">
                <el-icon><FolderOpened /></el-icon>
                <span>{{ svc.name }}</span>
              </div>
              <div v-for="method in svc.methods" :key="method.name" class="parse-tree-method">
                <el-icon><Document /></el-icon>
                <span class="method-name">{{ method.name }}</span>
                <span class="method-signature">
                  ({{ method.input_type }}) → {{ method.output_type }}
                  <el-tag v-if="method.client_stream" size="small" type="warning" effect="light" class="stream-tag">CS</el-tag>
                  <el-tag v-if="method.server_stream" size="small" type="success" effect="light" class="stream-tag">SS</el-tag>
                </span>
              </div>
            </div>
          </div>
        </div>
        <div v-if="parseDialogData.messages?.length" class="parse-section">
          <div class="parse-section-title">消息类型</div>
          <div class="parse-messages">
            <el-tag v-for="msg in parseDialogData.messages" :key="msg" size="small" effect="plain" class="message-tag">{{ msg }}</el-tag>
          </div>
        </div>
        <div v-if="!parseDialogData.package && !parseDialogData.services?.length && !parseDialogData.messages?.length" class="parse-empty">
          解析结果为空
        </div>
      </div>
      <div v-else class="parse-empty">无法解析此文件</div>
      <template #footer>
        <el-button @click="parseDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile, FormRules } from 'element-plus'
import {
  Refresh, Upload, Edit, Delete, Document, UploadFilled,
  Search, View, Check, CircleCheck, CircleClose, FolderOpened
} from '@element-plus/icons-vue'
import { protoFileAPI } from '@/api/apiTest'
import { isHandledError } from '@/utils/api'
import { formatDateTime } from '@/utils/format'
import type { ProtoFile, ProtoParseResult } from '@/api/apiTest'

const protoFiles = ref<ProtoFile[]>([])
const loading = ref(false)
const submitting = ref(false)
const verifying = ref(false)
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const searchQuery = ref('')

// Dialog state
const dialogVisible = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const formData = ref({
  name: '',
  content: '',
  dependencies: [] as number[],
})

// Verify state
const verifyResult = ref<ProtoParseResult | null>(null)
const verifyError = ref('')

// Parse dialog state
const parseDialogVisible = ref(false)
const parseDialogFileName = ref('')
const parseDialogData = ref<ProtoParseResult | null>(null)
const lineNumbersRef = ref<HTMLElement | null>(null)

const validateProtoName = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (!value) {
    callback(new Error('请输入文件名'))
  } else if (!value.endsWith('.proto')) {
    callback(new Error('文件名必须以 .proto 结尾'))
  } else if (value.includes('/') || value.includes('\\')) {
    callback(new Error('文件名不能包含路径分隔符'))
  } else {
    callback()
  }
}

const formRules: FormRules = {
  name: [{ required: true, validator: validateProtoName, trigger: 'blur' }],
  content: [{ required: true, message: '请输入 Proto 文件内容', trigger: 'blur' }],
}

const filteredProtoFiles = computed(() => {
  if (!searchQuery.value.trim()) return protoFiles.value
  const q = searchQuery.value.toLowerCase()
  return protoFiles.value.filter(f => {
    const nameMatch = f.name.toLowerCase().includes(q)
    const pkgMatch = getPackageName(f).toLowerCase().includes(q)
    return nameMatch || pkgMatch
  })
})

// All proto files for dependency selection (exclude current editing item)
const dependencyOptions = computed(() => {
  if (!isEditing.value || !editingId.value) return protoFiles.value
  return protoFiles.value.filter(f => f.id !== editingId.value)
})

const contentLineCount = computed(() => {
  if (!formData.value.content) return 1
  return formData.value.content.split('\n').length
})

const syncLineScroll = (e: Event) => {
  const target = e.target as HTMLElement
  if (lineNumbersRef.value && target) {
    lineNumbersRef.value.scrollTop = target.scrollTop
  }
}

const parseProtoResult = (row: ProtoFile): ProtoParseResult | null => {
  if (!row.parsed_result) return null
  try {
    return JSON.parse(row.parsed_result) as ProtoParseResult
  } catch {
    return null
  }
}

const getPackageName = (row: ProtoFile): string => {
  const parsed = parseProtoResult(row)
  return parsed?.package || ''
}

const getServiceCount = (row: ProtoFile): number => {
  const parsed = parseProtoResult(row)
  return parsed?.services?.length || 0
}

const formatFileSize = (content: string): string => {
  if (!content) return '0 字符'
  const len = content.length
  if (len < 1000) return `${len} 字符`
  return `${(len / 1000).toFixed(1)}k 字符`
}

const getDependencyNames = (row: ProtoFile): string => {
  if (!row.dependencies) return ''
  try {
    const depIds = JSON.parse(row.dependencies) as number[]
    if (!depIds.length) return ''
    const names = depIds
      .map(id => protoFiles.value.find(f => f.id === id)?.name)
      .filter(Boolean)
    return names.join(', ')
  } catch {
    return ''
  }
}

const loadProtoFiles = async () => {
  loading.value = true
  try {
    const res = await protoFileAPI.list({ page: currentPage.value, page_size: pageSize.value })
    protoFiles.value = res.data.items || []
    total.value = res.data.total || 0
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载 Proto 文件列表失败')
    }
  } finally {
    loading.value = false
  }
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  loadProtoFiles()
}

const handleUpload = () => {
  isEditing.value = false
  editingId.value = null
  formData.value = { name: '', content: '', dependencies: [] }
  verifyResult.value = null
  verifyError.value = ''
  dialogVisible.value = true
}

const handleEdit = (row: ProtoFile) => {
  isEditing.value = true
  editingId.value = row.id
  formData.value = {
    name: row.name,
    content: row.content,
    dependencies: row.dependencies
      ? (JSON.parse(row.dependencies) as number[])
      : [],
  }
  verifyResult.value = null
  verifyError.value = ''
  dialogVisible.value = true
}

const handleFileSelect = (file: UploadFile) => {
  if (!file.raw) return
  const reader = new FileReader()
  reader.onload = (e) => {
    const text = e.target?.result as string
    formData.value.content = text
    verifyResult.value = null
    verifyError.value = ''
    if (!formData.value.name) {
      formData.value.name = file.name
    }
  }
  reader.readAsText(file.raw)
}

const handleVerify = async () => {
  if (!formData.value.content.trim()) return
  verifying.value = true
  verifyResult.value = null
  verifyError.value = ''
  try {
    const depContents: string[] = []
    if (formData.value.dependencies.length > 0) {
      for (const depId of formData.value.dependencies) {
        const depFile = protoFiles.value.find(f => f.id === depId)
        if (depFile) {
          depContents.push(depFile.content)
        }
      }
    }
    const res = await protoFileAPI.parse({
      content: formData.value.content,
      dependencies: depContents.length > 0 ? depContents : undefined,
    })
    verifyResult.value = res.data
    ElMessage.success('Proto 文件验证通过')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    verifyError.value = msg || '解析失败'
  } finally {
    verifying.value = false
  }
}

const handleParse = (row: ProtoFile) => {
  const parsed = parseProtoResult(row)
  if (!parsed) {
    ElMessage.warning('此文件暂无解析结果，请先编辑保存后重试')
    return
  }
  parseDialogFileName.value = row.name
  parseDialogData.value = parsed
  parseDialogVisible.value = true
}

const handleSubmit = async () => {
  if (!formData.value.name.trim()) {
    ElMessage.warning('请输入文件名')
    return
  }
  if (!formData.value.content.trim()) {
    ElMessage.warning('请输入 Proto 文件内容')
    return
  }
  submitting.value = true
  try {
    if (isEditing.value && editingId.value) {
      await protoFileAPI.update(editingId.value, {
        name: formData.value.name,
        content: formData.value.content,
        dependencies: formData.value.dependencies,
      })
      ElMessage.success('Proto 文件已更新')
    } else {
      await protoFileAPI.create({
        name: formData.value.name,
        content: formData.value.content,
        dependencies: formData.value.dependencies,
      })
      ElMessage.success('Proto 文件已上传')
    }
    dialogVisible.value = false
    await loadProtoFiles()
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '操作失败')
    }
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row: ProtoFile) => {
  try {
    await ElMessageBox.confirm(`确定要删除 Proto 文件 "${row.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await protoFileAPI.delete(row.id)
    ElMessage.success('Proto 文件已删除')
    await loadProtoFiles()
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error((err as Error).message || '删除失败')
    }
  }
}

const resetForm = () => {
  formData.value = { name: '', content: '', dependencies: [] }
  isEditing.value = false
  editingId.value = null
  verifyResult.value = null
  verifyError.value = ''
}

onMounted(() => {
  loadProtoFiles()
})
</script>

<style scoped>
.proto-files-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  height: 100%;
  min-height: 0;
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

.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-primary);
}

.search-input {
  width: 220px;
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

.create-btn {
  font-weight: 500;
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-secondary) 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
  padding: 8px 20px;
}

.create-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
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

.file-size {
  font-size: 12px;
  color: var(--el-text-color-secondary);
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

.pagination-container :deep(.el-pagination) {
  display: flex;
  align-items: center;
  gap: 8px;
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

/* Proto editor with line numbers */
.proto-editor-wrapper {
  display: flex;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  overflow: hidden;
  width: 100%;
}

.line-numbers {
  display: flex;
  flex-direction: column;
  padding: 8px 8px 8px 4px;
  background: var(--el-fill-color-lighter);
  border-right: 1px solid var(--el-border-color-lighter);
  user-select: none;
  text-align: right;
  min-width: 36px;
  overflow: hidden;
}

.line-number {
  font-size: 12px;
  line-height: 20px;
  color: var(--el-text-color-placeholder);
  font-family: monospace;
}

.proto-textarea {
  flex: 1;
}

.proto-textarea :deep(.el-textarea__inner) {
  border: none;
  border-radius: 0;
  font-family: monospace;
  line-height: 20px;
  padding: 8px 12px;
}

.file-upload-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.dep-select {
  width: 100%;
}

/* Verify result */
.verify-result {
  margin-top: 8px;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--el-color-success-light-9);
  border: 1px solid var(--el-color-success-light-7);
}

.verify-result.verify-error {
  background: var(--el-color-danger-light-9);
  border-color: var(--el-color-danger-light-7);
}

.verify-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 6px;
}

.verify-item {
  font-size: 12px;
  color: var(--el-text-color-regular);
  margin-bottom: 2px;
}

.verify-label {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.verify-service-tag {
  display: inline-block;
  margin-right: 6px;
  padding: 2px 8px;
  background: var(--el-color-primary-light-9);
  border-radius: 4px;
  font-size: 12px;
}

/* Parse result dialog */
.parse-result-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.parse-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.parse-section-title {
  font-weight: 600;
  font-size: 13px;
  color: var(--el-text-color-primary);
  padding-bottom: 4px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.parse-tree {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-left: 8px;
}

.parse-tree-node {
  border-left: 2px solid var(--el-color-primary-light-5);
  padding-left: 12px;
}

.parse-tree-service {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13px;
  color: var(--el-color-primary);
  margin-bottom: 4px;
}

.parse-tree-method {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 3px 0;
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.method-name {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.method-signature {
  color: var(--el-text-color-secondary);
  font-family: monospace;
  font-size: 11px;
}

.stream-tag {
  margin-left: 4px;
  font-size: 10px;
}

.parse-messages {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.message-tag {
  font-family: monospace;
}

.parse-empty {
  text-align: center;
  padding: 24px;
  color: var(--el-text-color-placeholder);
  font-size: 13px;
}
</style>
