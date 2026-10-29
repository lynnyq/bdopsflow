<!-- mTLS 证书管理页面：支持 CA/客户端证书/私钥的上传、编辑、删除 -->
<template>
  <div class="certificate-list-page">
    <!-- 页面工具栏：搜索 + 新增按钮 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <h3 class="page-title">证书管理</h3>
      </div>
      <div class="toolbar-right">
        <el-input
          v-model="searchQuery"
          placeholder="搜索证书名称"
          :prefix-icon="Search"
          clearable
          size="default"
          class="search-input"
        />
        <el-button :icon="Plus" @click="handleCreate" class="create-btn">新增证书</el-button>
      </div>
    </div>

    <div class="table-wrapper">
      <el-table :data="certificates" v-loading="loading" stripe height="100%">
        <el-table-column prop="name" label="名称" :min-width="150" show-overflow-tooltip />
        <el-table-column label="类型" :min-width="200">
          <template #default="{ row }">
            <el-tag
              v-if="row.has_ca_cert && row.has_client_cert && row.has_client_key"
              effect="light"
              size="small"
              class="cert-tag cert-tag-mtls"
            >完整mTLS</el-tag>
            <template v-else>
              <el-tag v-if="row.has_ca_cert" effect="light" size="small" class="cert-tag cert-tag-ca">CA证书</el-tag>
              <el-tag v-if="row.has_client_cert" effect="light" size="small" class="cert-tag cert-tag-client">客户端证书</el-tag>
              <el-tag v-if="row.has_client_key && !row.has_client_cert" effect="light" size="small" class="cert-tag cert-tag-key">客户端私钥</el-tag>
            </template>
            <span v-if="!row.has_ca_cert && !row.has_client_cert && !row.has_client_key" class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="引用" width="80" align="center">
          <template #default="{ row }">
            <el-tooltip v-if="getCertUsageCount(row.id) > 0" :content="`被 ${getCertUsageCount(row.id)} 个 gRPC 测试引用`" placement="top">
              <el-tag size="small" effect="plain" type="info">{{ getCertUsageCount(row.id) }}</el-tag>
            </el-tooltip>
            <span v-else class="text-muted">0</span>
          </template>
        </el-table-column>
        <el-table-column v-if="authStore.isSystemAdmin" label="创建者" width="120" show-overflow-tooltip>
          <template #default="{ row }">
            {{ row.created_by_name || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right" align="center">
          <template #default="{ row }">
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
            <p>暂无证书</p>
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
        @current-change="loadCertificates"
        @size-change="handleSizeChange"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑证书' : '新增证书'"
      width="600px"
      :close-on-click-modal="false"
      @close="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入证书名称" />
        </el-form-item>
        <el-form-item label="CA 证书" prop="ca_cert">
          <div class="pem-field">
            <el-input
              v-model="form.ca_cert"
              type="textarea"
              :rows="4"
              placeholder="PEM 格式 CA 证书（可选）"
              @blur="validatePemField('ca_cert')"
            />
            <a class="paste-sample-link" @click="pasteSample('ca_cert')">粘贴示例</a>
          </div>
          <div v-if="pemErrors.ca_cert" class="pem-error">{{ pemErrors.ca_cert }}</div>
        </el-form-item>
        <el-form-item label="客户端证书" prop="client_cert">
          <div class="pem-field">
            <el-input
              v-model="form.client_cert"
              type="textarea"
              :rows="4"
              placeholder="PEM 格式客户端证书（可选）"
              @blur="validatePemField('client_cert')"
            />
            <a class="paste-sample-link" @click="pasteSample('client_cert')">粘贴示例</a>
          </div>
          <div v-if="pemErrors.client_cert" class="pem-error">{{ pemErrors.client_cert }}</div>
        </el-form-item>
        <el-form-item label="客户端私钥" prop="client_key">
          <div class="private-key-field">
            <div v-if="!privateKeyRevealed && isEditing && hasPrivateKey && !privateKeyChanged" class="private-key-masked">
              <span class="masked-dots">••••••••••••••••••••••••••••••••</span>
              <el-button size="small" link type="primary" @click="privateKeyRevealed = true">
                <el-icon><View /></el-icon> 显示
              </el-button>
            </div>
            <template v-else>
              <div class="pem-field">
                <el-input
                  v-model="form.client_key"
                  type="textarea"
                  :rows="4"
                  :placeholder="isEditing && hasPrivateKey ? '未修改（留空保持原私钥不变）' : '粘贴PEM格式私钥'"
                  @input="onPrivateKeyInput"
                  @blur="validatePemField('client_key')"
                />
                <a class="paste-sample-link" @click="pasteSample('client_key')">粘贴示例</a>
              </div>
              <div v-if="pemErrors.client_key" class="pem-error">{{ pemErrors.client_key }}</div>
            </template>
            <div v-if="isEditing && hasPrivateKey && !privateKeyChanged" class="private-key-hint">
              <el-icon><Lock /></el-icon>
              <span>私钥已加密存储，如需修改请在上方输入新私钥</span>
            </div>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * Certificates.vue - mTLS 证书管理页面
 *
 * 功能：CA 证书 / 客户端证书 / 客户端私钥 的上传、编辑、删除。
 * 列表页展示证书摘要（不含私钥），并显示每个证书被多少个 gRPC 测试引用。
 */
// ========== Vue 核心 API ==========
import { ref, reactive, onMounted, watch } from 'vue'
// ========== Element Plus ==========
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
// ========== Element Plus 图标 ==========
import { Plus, Edit, Delete, Document, Lock, View, Search } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { certificateAPI, apiTestAPI } from '@/api/apiTest'
// ========== API 类型 ==========
import type { CertificateSummary, ApiTest } from '@/api/apiTest'
// ========== Pinia Store ==========
import { useAuthStore } from '@/stores/auth'
// ========== 工具函数 ==========
import { isHandledError } from '@/utils/api'
import { formatDateTime } from '@/utils/format'

const authStore = useAuthStore()  // 认证 Store：判断是否系统管理员

// ========== 列表状态 ==========
const certificates = ref<CertificateSummary[]>([])  // 证书列表
const grpcTests = ref<ApiTest[]>([])                  // gRPC 测试列表（用于计算引用数）
const loading = ref(false)                             // 列表加载状态
const total = ref(0)                                   // 总记录数
const currentPage = ref(1)                             // 当前页码
const pageSize = ref(20)                               // 每页条数
const searchQuery = ref('')                            // 搜索关键字

// ========== 编辑弹窗状态 ==========
const dialogVisible = ref(false)                       // 弹窗可见性
const isEditing = ref(false)                           // 是否编辑模式
const editingId = ref<number | null>(null)             // 当前编辑的证书 ID
const submitting = ref(false)                           // 提交加载状态
const privateKeyChanged = ref(false)                    // 私钥是否被修改
const hasPrivateKey = ref(false)                        // 原证书是否含私钥
const privateKeyRevealed = ref(false)                   // 私钥是否已显示

const formRef = ref<FormInstance>()                    // 表单 DOM 引用

// 编辑表单数据
const form = ref({
  name: '',          // 证书名称
  ca_cert: '',       // CA 证书 PEM 内容
  client_cert: '',   // 客户端证书 PEM 内容
  client_key: '',    // 客户端私钥 PEM 内容（编辑时留空保持不变）
})

// PEM 格式校验错误信息（按字段名索引）
const pemErrors = reactive<Record<string, string>>({
  ca_cert: '',
  client_cert: '',
  client_key: '',
})

/** PEM 格式示例（点击"粘贴示例"使用） */
const PEM_SAMPLES: Record<string, string> = {
  ca_cert: '-----BEGIN CERTIFICATE-----\nMIID...CA证书内容...\n-----END CERTIFICATE-----',
  client_cert: '-----BEGIN CERTIFICATE-----\nMIID...客户端证书内容...\n-----END CERTIFICATE-----',
  client_key: '-----BEGIN PRIVATE KEY-----\nMIIE...私钥内容...\n-----END PRIVATE KEY-----',
}

/** 各字段有效的 PEM 文件头前缀 */
const PEM_PREFIXES: Record<string, string[]> = {
  ca_cert: ['-----BEGIN CERTIFICATE-----'],
  client_cert: ['-----BEGIN CERTIFICATE-----'],
  client_key: ['-----BEGIN PRIVATE KEY-----', '-----BEGIN RSA PRIVATE KEY-----', '-----BEGIN EC PRIVATE KEY-----'],
}

// 表单校验规则（目前只要求名称必填）
const formRules: FormRules = {
  name: [{ required: true, message: '请输入证书名称', trigger: 'blur' }],
}

/**
 * 统计指定证书被多少个 gRPC 测试引用
 * @param certId - 证书 ID
 * @returns 引用次数
 */
const getCertUsageCount = (certId: number): number => {
  return grpcTests.value.filter(t => {
    if (t.type !== 'grpc') return false
    try {
      const config = JSON.parse(t.config) as { certificate_id?: number }
      return config.certificate_id === certId
    } catch {
      return false
    }
  }).length
}

/**
 * 校验指定字段的 PEM 格式是否有效
 * @param field - 'ca_cert' | 'client_cert' | 'client_key'
 */
const validatePemField = (field: 'ca_cert' | 'client_cert' | 'client_key') => {
  const value = form.value[field]?.trim()
  pemErrors[field] = ''
  if (!value) return
  const prefixes = PEM_PREFIXES[field]
  const valid = prefixes.some(prefix => value.startsWith(prefix))
  if (!valid) {
    pemErrors[field] = '内容不是有效的 PEM 格式，应以 "-----BEGIN" 开头'
  }
}

/**
 * 将 PEM 示例内容粘贴到指定字段（便于快速填入测试数据）
 * @param field - 目标 PEM 字段名
 */
const pasteSample = (field: 'ca_cert' | 'client_cert' | 'client_key') => {
  form.value[field] = PEM_SAMPLES[field]
  pemErrors[field] = ''
  if (field === 'client_key') {
    privateKeyChanged.value = true
  }
}

/**
 * 加载证书列表（支持分页和搜索）
 */
const loadCertificates = async () => {
  loading.value = true
  try {
    const res = await certificateAPI.list({
      page: currentPage.value,
      page_size: pageSize.value,
      search: searchQuery.value || undefined,
    })
    certificates.value = res.data.items || []
    total.value = res.data.total || 0
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err instanceof Error ? err.message : String(err)) || '加载证书列表失败')
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
    loadCertificates()
  }, 300)
})

/**
 * 切换分页大小时重置到第 1 页并重新加载
 * @param {number} size - 每页条数
 */
const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  loadCertificates()
}

/**
 * 加载所有 gRPC 测试，用于统计证书引用次数
 * @returns {Promise<void>}
 */
const loadGrpcTests = async () => {
  try {
    const res = await apiTestAPI.list({ type: 'grpc', page: 1, page_size: 1000 })
    grpcTests.value = res.data.items || []
  } catch {
    // Silently fail - usage count is non-critical
  }
}

/**
 * 打开新增证书对话框，初始化空表单
 */
const handleCreate = () => {
  isEditing.value = false
  editingId.value = null
  privateKeyChanged.value = false
  privateKeyRevealed.value = false
  pemErrors.ca_cert = ''
  pemErrors.client_cert = ''
  pemErrors.client_key = ''
  dialogVisible.value = true
}

/**
 * 打开编辑证书对话框，先获取证书详情再回填表单
 * @param {CertificateSummary} row - 当前行的证书摘要数据
 * @returns {Promise<void>}
 */
const handleEdit = async (row: CertificateSummary) => {
  isEditing.value = true
  editingId.value = row.id
  privateKeyChanged.value = false
  privateKeyRevealed.value = false
  hasPrivateKey.value = row.has_client_key
  pemErrors.ca_cert = ''
  pemErrors.client_cert = ''
  pemErrors.client_key = ''
  try {
    const res = await certificateAPI.get(row.id)
    const cert = res.data
    form.value = {
      name: cert.name,
      ca_cert: cert.ca_cert || '',
      client_cert: cert.client_cert || '',
      client_key: '',
    }
    dialogVisible.value = true
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err instanceof Error ? err.message : String(err)) || '获取证书详情失败')
    }
  }
}

/**
 * 删除指定证书，检查引用数后弹出确认框
 * @param {CertificateSummary} row - 要删除的证书数据
 * @returns {Promise<void>}
 */
const handleDelete = async (row: CertificateSummary) => {
  const usageCount = getCertUsageCount(row.id)
  let confirmMsg = `确定要删除证书 "${row.name}" 吗？`
  if (usageCount > 0) {
    confirmMsg = `证书 "${row.name}" 正被 ${usageCount} 个 gRPC 测试引用，删除后相关测试将无法正常使用。确定要删除吗？`
  }
  try {
    await ElMessageBox.confirm(confirmMsg, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: usageCount > 0 ? 'error' : 'warning',
    })
    await certificateAPI.delete(row.id)
    ElMessage.success('证书已删除')
    await loadCertificates()
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error((err instanceof Error ? err.message : String(err)) || '删除失败')
    }
  }
}

/**
 * 提交证书表单，校验 PEM 格式后创建或更新
 * @returns {Promise<void>}
 */
const handleSubmit = async () => {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  // Validate PEM fields
  validatePemField('ca_cert')
  validatePemField('client_cert')
  validatePemField('client_key')
  if (pemErrors.ca_cert || pemErrors.client_cert || pemErrors.client_key) {
    ElMessage.warning('请修正 PEM 格式错误后再提交')
    return
  }

  submitting.value = true
  try {
    const data: Record<string, string | undefined> = {
      name: form.value.name,
      ca_cert: form.value.ca_cert || undefined,
      client_cert: form.value.client_cert || undefined,
    }

    if (isEditing.value) {
      if (privateKeyChanged.value && form.value.client_key) {
        data.client_key = form.value.client_key
      }
      await certificateAPI.update(editingId.value!, data)
      ElMessage.success('证书已更新')
    } else {
      data.client_key = form.value.client_key || undefined
      await certificateAPI.create(data as { name: string; ca_cert?: string; client_cert?: string; client_key?: string })
      ElMessage.success('证书已创建')
    }

    dialogVisible.value = false
    await loadCertificates()
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      const msg = err instanceof Error ? err.message : String(err)
      ElMessage.error(msg || '操作失败')
    }
  } finally {
    submitting.value = false
  }
}

/**
 * 重置证书表单状态
 */
const resetForm = () => {
  form.value = {
    name: '',
    ca_cert: '',
    client_cert: '',
    client_key: '',
  }
  privateKeyChanged.value = false
  privateKeyRevealed.value = false
  pemErrors.ca_cert = ''
  pemErrors.client_cert = ''
  pemErrors.client_key = ''
  formRef.value?.resetFields()
}

/**
 * 编辑模式下检测私钥字段是否被修改
 */
const onPrivateKeyInput = () => {
  if (isEditing.value) {
    privateKeyChanged.value = true
  }
}

onMounted(() => {
  loadCertificates()
  loadGrpcTests()
})
</script>

<style scoped>
.certificate-list-page {
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

.search-input {
  width: 200px;
}

.page-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-primary);
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

.cert-tag {
  margin-right: 4px;
}

.cert-tag-ca {
  --el-tag-bg-color: var(--el-color-primary-light-9);
  --el-tag-border-color: var(--el-color-primary-light-7);
  --el-tag-text-color: var(--el-color-primary);
}

.cert-tag-client {
  --el-tag-bg-color: var(--el-color-success-light-9);
  --el-tag-border-color: var(--el-color-success-light-7);
  --el-tag-text-color: var(--el-color-success);
}

.cert-tag-mtls {
  --el-tag-bg-color: #f3e8ff;
  --el-tag-border-color: #d8b4fe;
  --el-tag-text-color: #7c3aed;
}

.cert-tag-key {
  --el-tag-bg-color: var(--el-color-danger-light-9);
  --el-tag-border-color: var(--el-color-danger-light-7);
  --el-tag-text-color: var(--el-color-danger);
}

.text-muted {
  color: var(--text-muted);
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

.private-key-field {
  width: 100%;
}

.private-key-masked {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  margin-bottom: 4px;
}

.masked-dots {
  letter-spacing: 2px;
  color: var(--el-text-color-placeholder);
  font-size: 14px;
}

.private-key-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.pem-field {
  width: 100%;
  position: relative;
}

.paste-sample-link {
  display: inline-block;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-color-primary);
  cursor: pointer;
  text-decoration: none;
}

.paste-sample-link:hover {
  text-decoration: underline;
}

.pem-error {
  font-size: 12px;
  color: var(--el-color-danger);
  margin-top: 4px;
  line-height: 1.4;
}
</style>
