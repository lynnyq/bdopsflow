<template>
  <div class="grpc-test-page">
    <div class="main-content">
      <!-- Left Panel: Saved Tests -->
      <aside class="saved-panel">
        <div class="panel-header">
          <span class="panel-title">gRPC 测试</span>
          <el-badge :value="savedTests.length" :max="99" type="info" class="panel-count-badge" />
        </div>
        <div class="panel-search">
          <el-input
            v-model="searchQuery"
            placeholder="搜索..."
            :prefix-icon="Search"
            size="small"
            clearable
          />
        </div>
        <div class="saved-list">
          <div
            v-for="item in filteredTests"
            :key="item.id"
            class="saved-item"
            :class="{ active: currentTestId === item.id }"
            @click="handleLoadTest(item)"
          >
            <el-tag size="small" effect="dark" class="grpc-badge" disable-transitions>gRPC</el-tag>
            <div class="saved-item-info">
              <span class="saved-name" :title="item.name">{{ item.name }}</span>
              <span class="saved-addr">{{ getAddrFromConfig(item.config) }}</span>
            </div>
            <el-icon class="saved-delete" @click.stop="handleDeleteTest(item)">
              <Delete />
            </el-icon>
          </div>
          <div v-if="filteredTests.length === 0" class="empty-saved">
            <el-icon :size="24"><Document /></el-icon>
            <p>{{ searchQuery ? '无匹配结果' : '暂无测试用例' }}</p>
          </div>
        </div>
        <div class="panel-footer">
          <el-button type="primary" :icon="Plus" size="small" @click="handleNewTest" class="new-btn">
            新建测试
          </el-button>
        </div>
      </aside>

      <!-- Right Panel -->
      <main class="editor-area">
        <!-- Request Editor -->
        <div
          ref="requestSectionRef"
          class="request-section"
          :class="{
            'panel-maximized': panelMode === 'request-max',
            'panel-minimized': panelMode === 'response-max',
          }"
        >
          <!-- Editing indicator -->
          <div v-if="currentTestName" class="current-test-name">
            <el-icon :size="14"><EditPen /></el-icon>
            <span>{{ currentTestName }}</span>
          </div>

          <!-- Top Bar -->
          <div class="request-bar">
            <el-input
              v-model="requestConfig.address"
              placeholder="服务器地址 (如: localhost:50051)"
              class="address-input"
              clearable
            >
              <template #prefix>
                <el-icon><Connection /></el-icon>
              </template>
            </el-input>
            <el-divider direction="vertical" class="bar-divider" />
            <el-select v-model="connectionMode" class="mode-select" placeholder="连接模式">
              <el-option label="Proto 文件" value="proto" />
              <el-option label="服务反射" value="reflection" />
            </el-select>
            <el-input-number
              v-model="requestConfig.timeout"
              :min="1"
              :max="300"
              :step="1"
              size="default"
              class="timeout-input"
              controls-position="right"
            />
            <el-tooltip content="超时时间（秒）" placement="bottom">
              <el-icon class="timeout-hint"><Clock /></el-icon>
            </el-tooltip>
            <el-button
              v-if="!sending"
              type="primary"
              :icon="VideoPlay"
              @click="handleSend"
              :disabled="!canSend"
              class="send-btn"
              size="large"
            >
              发送
            </el-button>
            <el-button
              v-else
              type="danger"
              :icon="Close"
              @click="handleSend"
              class="send-btn"
              size="large"
            >
              取消
            </el-button>
            <el-button
              :icon="Connection"
              @click="handleConnectTest"
              :loading="connecting"
              :disabled="!requestConfig.address"
              class="action-btn"
            >
              连接测试
            </el-button>
            <el-button :icon="FolderOpened" @click="handleSave" :disabled="!canSave" class="action-btn">保存</el-button>
            <el-tooltip :content="panelMode === 'request-max' ? '恢复' : '最大化请求'" placement="bottom">
              <el-button
                :icon="panelMode === 'request-max' ? ArrowDown : ArrowUp"
                @click="togglePanelMode('request-max')"
                class="action-btn panel-toggle-btn"
                size="small"
              />
            </el-tooltip>
          </div>

        <!-- Tab Panels -->
        <el-tabs v-model="activeRequestTab" class="request-tabs">
          <!-- Service Tab -->
          <el-tab-pane label="服务" name="service">
            <div class="service-panel">
              <!-- Proto File Mode -->
              <template v-if="connectionMode === 'proto'">
                <div class="proto-selector">
                  <el-select
                    v-model="selectedProtoFileId"
                    placeholder="选择 Proto 文件"
                    filterable
                    clearable
                    class="proto-select"
                    @change="handleProtoFileChange"
                  >
                    <el-option
                      v-for="pf in protoFiles"
                      :key="pf.id"
                      :label="pf.name"
                      :value="pf.id"
                    />
                  </el-select>
                </div>
                <div v-if="parsingProto" class="service-loading">
                  <el-icon class="is-loading"><Refresh /></el-icon>
                  <span>解析中...</span>
                </div>
                <div v-else-if="protoServices.length > 0" class="service-tree">
                  <el-collapse v-model="expandedServiceNames" class="service-collapse">
                    <el-collapse-item
                      v-for="svc in protoServices"
                      :key="svc.name"
                      :name="svc.name"
                    >
                      <template #title>
                        <div class="service-name">
                          <el-icon :size="16" class="svc-icon"><Folder /></el-icon>
                          <span>{{ svc.name }}</span>
                          <el-tag size="small" type="info" effect="plain" class="method-count-tag">{{ svc.methods.length }}</el-tag>
                        </div>
                      </template>
                      <div
                        v-for="m in svc.methods"
                        :key="m.name"
                        class="method-item"
                        :class="{ active: requestConfig.service === svc.name && requestConfig.method === m.name }"
                        @click="handleSelectMethod(svc.name, m)"
                      >
                        <el-icon :size="14" class="method-icon">
                          <VideoPlay />
                        </el-icon>
                        <span class="method-name">{{ m.name }}</span>
                        <span class="method-type">{{ m.input_type }} → {{ m.output_type }}</span>
                        <el-tag v-if="m.server_stream" size="small" type="warning" effect="light" class="stream-tag">stream</el-tag>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </div>
                <div v-else-if="selectedProtoFileId && !parsingProto" class="empty-state">
                  <el-icon :size="20"><WarningFilled /></el-icon>
                  <p>未解析到服务定义</p>
                </div>
                <div v-else class="empty-state">
                  <el-icon :size="32" class="empty-icon"><Upload /></el-icon>
                  <p class="empty-title">选择 Proto 文件或使用服务反射来发现 gRPC 服务</p>
                  <p class="empty-desc">上传 .proto 文件解析服务定义，或切换到服务反射模式</p>
                </div>
              </template>

              <!-- Reflection Mode -->
              <template v-else>
                <div class="reflection-bar">
                  <el-button
                    type="primary"
                    :icon="Search"
                    :loading="reflecting"
                    :disabled="!requestConfig.address"
                    @click="handleReflect"
                  >
                    发现服务
                  </el-button>
                </div>
                <div v-if="reflecting" class="service-loading">
                  <el-icon class="is-loading"><Refresh /></el-icon>
                  <span>发现中...</span>
                </div>
                <div v-else-if="protoServices.length > 0" class="service-tree">
                  <el-collapse v-model="expandedServiceNames" class="service-collapse">
                    <el-collapse-item
                      v-for="svc in protoServices"
                      :key="svc.name"
                      :name="svc.name"
                    >
                      <template #title>
                        <div class="service-name">
                          <el-icon :size="16" class="svc-icon"><Folder /></el-icon>
                          <span>{{ svc.name }}</span>
                          <el-tag size="small" type="info" effect="plain" class="method-count-tag">{{ svc.methods.length }}</el-tag>
                        </div>
                      </template>
                      <div
                        v-for="m in svc.methods"
                        :key="m.name"
                        class="method-item"
                        :class="{ active: requestConfig.service === svc.name && requestConfig.method === m.name }"
                        @click="handleSelectMethod(svc.name, m)"
                      >
                        <el-icon :size="14" class="method-icon">
                          <VideoPlay />
                        </el-icon>
                        <span class="method-name">{{ m.name }}</span>
                        <span class="method-type">{{ m.input_type }} → {{ m.output_type }}</span>
                        <el-tag v-if="m.server_stream" size="small" type="warning" effect="light" class="stream-tag">stream</el-tag>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </div>
                <div v-else class="empty-state">
                  <el-icon :size="32" class="empty-icon"><Search /></el-icon>
                  <p class="empty-title">选择 Proto 文件或使用服务反射来发现 gRPC 服务</p>
                  <p class="empty-desc">输入服务器地址后点击"发现服务"获取远程服务列表</p>
                </div>
              </template>
            </div>
          </el-tab-pane>

          <!-- Request Tab -->
          <el-tab-pane label="请求" name="request">
            <div class="request-mode-bar">
              <el-radio-group v-model="requestMode" size="small">
                <el-radio-button value="form">表单</el-radio-button>
                <el-radio-button value="json">JSON</el-radio-button>
              </el-radio-group>
            </div>
            <!-- Form mode -->
            <div v-show="requestMode === 'form'" class="form-wrapper">
              <div v-if="currentMessageDef" class="form-container">
                <div class="form-message-header">
                  <el-icon :size="14"><Folder /></el-icon>
                  <span>{{ currentMessageDef.name }}</span>
                  <el-tag size="small" type="info" effect="plain">{{ currentMessageDef.fields.length }} 个字段</el-tag>
                </div>
                <ProtoForm
                  :fields="currentMessageDef.fields"
                  v-model="formData"
                />
              </div>
              <div v-else class="empty-state">
                <el-icon :size="20"><WarningFilled /></el-icon>
                <p>请先选择服务和方法</p>
              </div>
            </div>
            <!-- JSON mode -->
            <div v-show="requestMode === 'json'" class="editor-wrapper">
              <div class="editor-container" ref="requestEditorRef"></div>
            </div>
          </el-tab-pane>

          <!-- Metadata Tab -->
          <el-tab-pane label="元数据" name="metadata">
            <div class="metadata-panel">
              <KVEditor v-model="metadata" key-label="Key" value-label="Value" add-label="添加元数据" />
            </div>
          </el-tab-pane>

          <!-- TLS Tab -->
          <el-tab-pane label="TLS" name="tls">
            <div class="tls-panel">
              <el-form label-width="100px" size="small">
                <el-form-item label="TLS 模式">
                  <div class="tls-mode-wrapper">
                    <el-select v-model="requestConfig.tls_mode" class="tls-mode-select">
                      <el-option label="insecure (不加密)" value="insecure">
                        <div class="tls-option">
                          <span class="tls-option-label">insecure (不加密)</span>
                          <el-tag size="small" type="danger" effect="light" class="tls-option-tag">不安全</el-tag>
                        </div>
                      </el-option>
                      <el-option label="tls (服务端验证)" value="tls">
                        <div class="tls-option">
                          <span class="tls-option-label">tls (服务端验证)</span>
                          <el-tag size="small" type="warning" effect="light" class="tls-option-tag">加密</el-tag>
                        </div>
                      </el-option>
                      <el-option label="mtls (双向认证)" value="mtls">
                        <div class="tls-option">
                          <span class="tls-option-label">mtls (双向认证)</span>
                          <el-tag size="small" type="success" effect="light" class="tls-option-tag">最安全</el-tag>
                        </div>
                      </el-option>
                    </el-select>
                    <div class="tls-security-indicator">
                      <el-icon :size="16" :class="tlsSecurityClass">
                        <component :is="tlsSecurityIcon" />
                      </el-icon>
                      <span :class="tlsSecurityClass">{{ tlsSecurityDesc }}</span>
                    </div>
                  </div>
                </el-form-item>
                <el-form-item v-if="requestConfig.tls_mode === 'insecure'" label="">
                  <div class="tls-mode-desc tls-mode-desc-danger">
                    <el-icon :size="14"><WarningFilled /></el-icon>
                    <span>通信未加密，数据以明文传输，仅建议在开发环境使用</span>
                  </div>
                </el-form-item>
                <el-form-item v-else-if="requestConfig.tls_mode === 'tls'" label="">
                  <div class="tls-mode-desc tls-mode-desc-warning">
                    <el-icon :size="14"><InfoFilled /></el-icon>
                    <span>仅验证服务端证书，客户端不提供证书，适用于大多数场景</span>
                  </div>
                </el-form-item>
                <el-form-item v-else-if="requestConfig.tls_mode === 'mtls'" label="">
                  <div class="tls-mode-desc tls-mode-desc-success">
                    <el-icon :size="14"><CircleCheckFilled /></el-icon>
                    <span>双向认证，客户端和服务端均需提供证书，安全性最高</span>
                  </div>
                </el-form-item>
                <el-form-item v-if="requestConfig.tls_mode !== 'insecure'" label="证书">
                  <el-select
                    v-model="requestConfig.certificate_id"
                    placeholder="选择证书"
                    filterable
                    clearable
                    class="cert-select"
                  >
                    <el-option
                      v-for="cert in certificates"
                      :key="cert.id"
                      :label="cert.name"
                      :value="cert.id"
                    >
                      <div class="cert-option">
                        <span class="cert-option-name">{{ cert.name }}</span>
                        <span class="cert-badges">
                          <el-tag v-if="cert.has_ca_cert" size="small" type="success" effect="light">CA</el-tag>
                          <el-tag v-if="cert.has_client_cert" size="small" type="primary" effect="light">Cert</el-tag>
                          <el-tag v-if="cert.has_client_key" size="small" type="warning" effect="light">Key</el-tag>
                        </span>
                      </div>
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-form>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>

      <!-- Resize Handle -->
      <div
        v-if="lastResult || sending"
        class="resize-handle"
        @mousedown="handleResizeDragStart"
      >
        <div class="resize-line"></div>
      </div>

      <!-- Response Viewer -->
      <transition name="response-fade">
        <div
          class="response-section"
          :class="{
            'panel-maximized': panelMode === 'response-max',
            'panel-minimized': panelMode === 'request-max',
          }"
          v-if="lastResult || sending"
        >
          <div class="response-header">
            <div class="response-info">
              <template v-if="lastResult">
                <el-tag
                  :type="lastResult.error ? 'danger' : 'success'"
                  effect="dark"
                  size="large"
                  class="status-badge"
                >
                  {{ lastResult.error ? '失败' : '成功' }}
                </el-tag>
                <span
                  v-if="lastResult.status_code !== undefined"
                  class="response-meta-item status-code"
                  :class="lastResult.status_code === 0 ? 'status-ok' : 'status-error'"
                >
                  gRPC {{ lastResult.status_code }}
                </span>
                <span class="response-meta-item">
                  <el-icon :size="14"><Clock /></el-icon>
                  {{ lastResult.latency_ms }}ms
                </span>
              </template>
              <template v-if="sending && !lastResult">
                <el-icon class="is-loading"><Refresh /></el-icon>
                <span>请求中...</span>
              </template>
            </div>
            <el-tooltip :content="panelMode === 'response-max' ? '恢复' : '最大化响应'" placement="bottom">
              <el-button
                :icon="panelMode === 'response-max' ? ArrowDown : ArrowUp"
                @click="togglePanelMode('response-max')"
                class="action-btn panel-toggle-btn"
                size="small"
              />
            </el-tooltip>
          </div>

          <el-tabs v-model="activeResponseTab" class="response-tabs">
            <!-- Body Tab -->
            <el-tab-pane label="响应体" name="body">
              <div class="response-body-content">
                <template v-if="lastResult">
                  <div v-if="lastResult.error" class="error-content">
                    <el-icon :size="16" color="var(--accent-danger)"><CircleCloseFilled /></el-icon>
                    <pre>{{ lastResult.error }}</pre>
                  </div>
                  <template v-else>
                    <div class="response-body-header">
                      <div class="response-body-modes">
                        <el-radio-group v-model="responseBodyMode" size="small">
                          <el-radio-button value="json">JSON</el-radio-button>
                          <el-radio-button value="raw">Raw</el-radio-button>
                        </el-radio-group>
                      </div>
                      <div class="response-body-actions">
                        <el-button
                          v-if="lastResult?.body"
                          size="small"
                          text
                          class="copy-response-btn"
                          @click="copyResponseBody"
                        >
                          <el-icon><DocumentCopy /></el-icon>
                          复制
                        </el-button>
                      </div>
                    </div>
                    <div v-if="responseBodyMode === 'json'" ref="responseEditorRef" class="response-json-editor"></div>
                    <div v-else class="response-raw-wrapper">
                      <pre class="response-raw">{{ lastResult?.body || '' }}</pre>
                    </div>
                  </template>
                </template>
              </div>
            </el-tab-pane>

            <!-- Metadata Tab -->
            <el-tab-pane label="响应头" name="resp-metadata">
              <div class="response-headers">
                <template v-if="lastResult && lastResult.headers">
                  <div v-for="(value, key) in parseResponseHeadersMap(lastResult.headers)" :key="key" class="header-row">
                    <span class="header-key">{{ key }}</span>
                    <span class="header-value">{{ value }}</span>
                  </div>
                  <div v-if="Object.keys(parseResponseHeadersMap(lastResult.headers)).length === 0" class="body-none">无响应头</div>
                </template>
                <div v-else class="body-none">暂无响应头</div>
              </div>
            </el-tab-pane>

            <!-- History Tab -->
            <el-tab-pane label="历史" name="history">
              <div class="history-toolbar" v-if="historyResults.length > 0">
                <el-button
                  type="primary"
                  size="small"
                  :disabled="selectedHistoryIds.length !== 2"
                  @click="handleCompare"
                >
                  对比 ({{ selectedHistoryIds.length }}/2)
                </el-button>
                <el-button
                  type="danger"
                  size="small"
                  plain
                  @click="handleClearHistory"
                >
                  清空历史
                </el-button>
              </div>
              <div class="history-list">
                <div
                  v-for="h in historyResults"
                  :key="h.id"
                  class="history-item"
                  :class="{ active: lastResult?.id === h.id }"
                  @click="lastResult = h"
                >
                  <el-checkbox
                    :model-value="selectedHistoryIds.includes(h.id)"
                    @change="(val: boolean) => toggleHistorySelect(h.id, val)"
                    @click.stop
                    size="small"
                  />
                  <el-tag
                    :type="h.error ? 'danger' : 'success'"
                    effect="light"
                    size="small"
                  >
                    {{ h.error ? '失败' : '成功' }}
                  </el-tag>
                  <span
                    v-if="h.status_code !== undefined"
                    class="history-code"
                    :class="h.status_code === 0 ? 'status-ok' : 'status-error'"
                  >
                    {{ h.status_code }}
                  </span>
                  <span class="history-latency">{{ h.latency_ms }}ms</span>
                  <span class="history-time" :title="formatDateTime(h.created_at)">{{ formatRelativeTime(h.created_at) }}</span>
                </div>
                <div v-if="historyResults.length === 0" class="body-none">暂无执行历史</div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>
      </transition>

      <!-- Empty State -->
      <div v-if="!lastResult && !sending" class="empty-response">
        <el-icon :size="48"><VideoPlay /></el-icon>
        <h3>暂无响应</h3>
        <p>选择服务和方法后点击发送按钮查看响应</p>
      </div>
    </main>
    </div>

    <!-- Save Dialog -->
    <el-dialog v-model="saveDialogVisible" title="保存测试用例" width="420px" :close-on-click-modal="false">
      <el-form label-width="80px">
        <el-form-item label="名称">
          <el-input v-model="saveName" placeholder="输入测试用例名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="saveDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSaveConfirm" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <!-- Compare Dialog -->
    <el-dialog v-model="compareDialogVisible" title="历史对比" width="900px">
      <div v-if="compareLeft && compareRight" class="compare-container">
        <div class="compare-panel">
          <div class="compare-header">
            <el-tag :type="compareLeft.error ? 'danger' : 'success'" size="small" effect="dark">{{ compareLeft.error ? '失败' : '成功' }}</el-tag>
            <span v-if="compareLeft.status_code !== undefined" class="compare-code" :class="compareLeft.status_code === 0 ? 'status-ok' : 'status-error'">gRPC {{ compareLeft.status_code }}</span>
            <span class="compare-latency">{{ compareLeft.latency_ms }}ms</span>
            <span class="compare-time">{{ formatDateTime(compareLeft.created_at) }}</span>
          </div>
          <pre class="compare-body">{{ formatCompareBody(compareLeft.body) }}</pre>
        </div>
        <div class="compare-panel">
          <div class="compare-header">
            <el-tag :type="compareRight.error ? 'danger' : 'success'" size="small" effect="dark">{{ compareRight.error ? '失败' : '成功' }}</el-tag>
            <span v-if="compareRight.status_code !== undefined" class="compare-code" :class="compareRight.status_code === 0 ? 'status-ok' : 'status-error'">gRPC {{ compareRight.status_code }}</span>
            <span class="compare-latency">{{ compareRight.latency_ms }}ms</span>
            <span class="compare-time">{{ formatDateTime(compareRight.created_at) }}</span>
          </div>
          <pre class="compare-body">{{ formatCompareBody(compareRight.body) }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="compareDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus, Delete, Search, Document, Refresh, Connection, VideoPlay,
  FolderOpened, WarningFilled, Upload, CircleCloseFilled, Clock, Folder,
  DocumentCopy, EditPen, InfoFilled, CircleCheckFilled,
  Lock, Unlock, Close, ArrowUp, ArrowDown
} from '@element-plus/icons-vue'
import { apiTestAPI, protoFileAPI, certificateAPI } from '@/api/apiTest'
import { isHandledError } from '@/utils/api'
import { formatDateTime } from '@/utils/format'
import KVEditor from './KVEditor.vue'
import ProtoForm from './ProtoForm.vue'
import type {
  ApiTest, ApiTestResult, ProtoFile, ProtoService, ProtoMethod,
  ProtoParseResult, CertificateSummary, GRPCRequestConfig, ProtoMessageDef, ProtoMessageField
} from '@/api/apiTest'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { oneDark } from '@codemirror/theme-one-dark'

// --- State ---
const searchQuery = ref('')
const savedTests = ref<ApiTest[]>([])
const currentTestId = ref<number | null>(null)
const loading = ref(false)
const sending = ref(false)
const saving = ref(false)
const connecting = ref(false)

const connectionMode = ref<'proto' | 'reflection'>('proto')
const activeRequestTab = ref('service')
const activeResponseTab = ref('body')
const responseBodyMode = ref('json')

// Panel resize state
const panelMode = ref<'both' | 'request-max' | 'response-max'>('both')
const requestSectionRef = ref<HTMLElement | null>(null)
const isDragging = ref(false)

const requestConfig = ref<GRPCRequestConfig>({
  address: '',
  service: '',
  method: '',
  request_body: '{}',
  metadata: [],
  tls_mode: 'insecure',
  certificate_id: null,
  proto_file_id: null,
  use_reflection: false,
  timeout: 30,
})

const metadata = ref<{ key: string; value: string }[]>([])
const protoFiles = ref<ProtoFile[]>([])
const certificates = ref<CertificateSummary[]>([])
const selectedProtoFileId = ref<number | null>(null)
const protoServices = ref<ProtoService[]>([])
const expandedServiceNames = ref<string[]>([])
const parsingProto = ref(false)
const reflecting = ref(false)

// Message field definitions and form data
const messageDefs = ref<ProtoMessageDef[]>([])
const currentMessageDef = ref<ProtoMessageDef | null>(null)
const formData = ref<Record<string, unknown>>({})
const requestMode = ref<'form' | 'json'>('form')

const lastResult = ref<ApiTestResult | null>(null)
const historyResults = ref<ApiTestResult[]>([])

const saveDialogVisible = ref(false)
const saveName = ref('')

// History comparison
const selectedHistoryIds = ref<number[]>([])
const compareDialogVisible = ref(false)
const compareLeft = ref<ApiTestResult | null>(null)
const compareRight = ref<ApiTestResult | null>(null)

// CodeMirror
const requestEditorRef = ref<HTMLElement | null>(null)
const responseEditorRef = ref<HTMLElement | null>(null)
let requestEditorView: EditorView | null = null
let responseEditorView: EditorView | null = null

// --- Computed ---
const filteredTests = computed(() => {
  if (!searchQuery.value) return savedTests.value
  const q = searchQuery.value.toLowerCase()
  return savedTests.value.filter(t => t.name.toLowerCase().includes(q))
})

const canSend = computed(() => {
  return requestConfig.value.address && requestConfig.value.service && requestConfig.value.method && !sending.value
})

const canSave = computed(() => {
  return requestConfig.value.address && !saving.value
})

const currentTestName = computed(() => {
  if (!currentTestId.value) return ''
  const found = savedTests.value.find(t => t.id === currentTestId.value)
  return found?.name || ''
})

const responseBodyText = computed(() => {
  if (!lastResult.value || lastResult.value.error) return ''
  return lastResult.value.body || ''
})

// TLS security computed
const tlsSecurityIcon = computed(() => {
  switch (requestConfig.value.tls_mode) {
    case 'insecure': return Unlock
    case 'tls': return Lock
    case 'mtls': return CircleCheckFilled
    default: return Unlock
  }
})

const tlsSecurityClass = computed(() => {
  switch (requestConfig.value.tls_mode) {
    case 'insecure': return 'security-danger'
    case 'tls': return 'security-warning'
    case 'mtls': return 'security-success'
    default: return 'security-danger'
  }
})

const tlsSecurityDesc = computed(() => {
  switch (requestConfig.value.tls_mode) {
    case 'insecure': return '不安全 - 未加密'
    case 'tls': return '加密 - 服务端验证'
    case 'mtls': return '最安全 - 双向认证'
    default: return '不安全 - 未加密'
  }
})

// --- Helpers ---
const getAddrFromConfig = (configStr: string): string => {
  try {
    const c = JSON.parse(configStr) as GRPCRequestConfig
    return c.address || ''
  } catch {
    return ''
  }
}

const parseResponseHeadersMap = (headersStr: string): Record<string, string> => {
  if (!headersStr) return {}
  try {
    const parsed = JSON.parse(headersStr)
    if (typeof parsed === 'object' && !Array.isArray(parsed)) return parsed as Record<string, string>
    if (Array.isArray(parsed)) {
      const map: Record<string, string> = {}
      for (const item of parsed) {
        if (item.key && item.value !== undefined) {
          map[item.key] = String(item.value)
        }
      }
      return map
    }
    return {}
  } catch {
    return {}
  }
}

function formatRelativeTime(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return dateStr
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return '刚刚'
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}分钟前`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}小时前`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 30) return `${diffDay}天前`
  return formatDateTime(dateStr)
}

const handleClearHistory = async () => {
  if (!currentTestId.value) return
  try {
    await ElMessageBox.confirm('确定清空该测试的所有执行历史吗？', '确认清空', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    for (const item of historyResults.value) {
      try {
        await apiTestAPI.deleteResult(item.id)
      } catch {
        // Continue deleting even if one fails
      }
    }
    historyResults.value = []
    selectedHistoryIds.value = []
    ElMessage.success('历史已清空')
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error('清空历史失败')
    }
  }
}

const toggleHistorySelect = (id: number, checked: boolean) => {
  if (checked) {
    if (selectedHistoryIds.value.length < 2) {
      selectedHistoryIds.value.push(id)
    }
  } else {
    selectedHistoryIds.value = selectedHistoryIds.value.filter(i => i !== id)
  }
}

const handleCompare = () => {
  if (selectedHistoryIds.value.length !== 2) return
  const [leftId, rightId] = selectedHistoryIds.value
  compareLeft.value = historyResults.value.find(h => h.id === leftId) || null
  compareRight.value = historyResults.value.find(h => h.id === rightId) || null
  if (compareLeft.value && compareRight.value) {
    compareDialogVisible.value = true
  }
}

const formatCompareBody = (body: string | undefined): string => {
  if (!body) return ''
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

// Panel resize handlers
const togglePanelMode = (mode: 'both' | 'request-max' | 'response-max') => {
  panelMode.value = panelMode.value === mode ? 'both' : mode
}

const handleResizeDragStart = (e: MouseEvent) => {
  e.preventDefault()
  isDragging.value = true
  const editorArea = (e.target as HTMLElement).closest('.editor-area') as HTMLElement
  if (!editorArea) return
  const areaRect = editorArea.getBoundingClientRect()
  const areaHeight = areaRect.height
  const startY = e.clientY
  const startHeight = requestSectionRef.value?.offsetHeight || 0

  const onMouseMove = (moveEvent: MouseEvent) => {
    const delta = moveEvent.clientY - startY
    const newHeight = startHeight + delta
    const minHeight = 120
    const maxHeight = areaHeight - 120
    const clampedHeight = Math.min(Math.max(newHeight, minHeight), maxHeight)
    if (requestSectionRef.value) {
      requestSectionRef.value.style.flex = 'none'
      requestSectionRef.value.style.height = `${clampedHeight}px`
    }
    panelMode.value = 'both'
  }

  const onMouseUp = () => {
    isDragging.value = false
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }

  document.body.style.cursor = 'row-resize'
  document.body.style.userSelect = 'none'
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

const copyResponseBody = async () => {
  if (!responseBodyText.value) return
  try {
    await navigator.clipboard.writeText(responseBodyText.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    const textarea = document.createElement('textarea')
    textarea.value = responseBodyText.value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    try {
      document.execCommand('copy')
      ElMessage.success('已复制到剪贴板')
    } catch {
      ElMessage.error('复制失败，请手动复制')
    }
    document.body.removeChild(textarea)
  }
}

// --- Data Loading ---
const loadSavedTests = async () => {
  loading.value = true
  try {
    const res = await apiTestAPI.list({ type: 'grpc', page: 1, page_size: 200 })
    savedTests.value = res.data.items || []
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载测试列表失败')
    }
  } finally {
    loading.value = false
  }
}

const loadProtoFiles = async () => {
  try {
    const res = await protoFileAPI.list({ page: 1, page_size: 200 })
    protoFiles.value = res.data.items || []
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载 Proto 文件列表失败')
    }
  }
}

const loadCertificates = async () => {
  try {
    const res = await certificateAPI.list({ page: 1, page_size: 200 })
    certificates.value = res.data.items || []
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载证书列表失败')
    }
  }
}

const loadHistory = async (testId: number) => {
  try {
    const res = await apiTestAPI.getResults(testId, { page: 1, page_size: 20 })
    historyResults.value = res.data.items || []
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载历史记录失败')
    }
  }
}

// --- Proto Parsing ---
const handleProtoFileChange = async (fileId: number | null) => {
  requestConfig.value.proto_file_id = fileId
  protoServices.value = []
  expandedServiceNames.value = []

  if (!fileId) return

  parsingProto.value = true
  messageDefs.value = []
  currentMessageDef.value = null
  formData.value = {}
  try {
    const pf = protoFiles.value.find(f => f.id === fileId)
    if (!pf) return

    // Parse services and load field definitions in parallel
    const [parseRes, fieldsRes] = await Promise.all([
      protoFileAPI.parse({ content: pf.content }),
      protoFileAPI.fields({ proto_file_id: fileId }).catch(() => null),
    ])

    const result = parseRes.data as ProtoParseResult
    if (result.services) {
      protoServices.value = result.services
      expandedServiceNames.value = result.services.map(s => s.name)
    }

    if (fieldsRes) {
      messageDefs.value = fieldsRes.data.messages || []
    }
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '解析 Proto 文件失败')
    }
  } finally {
    parsingProto.value = false
  }
}

const handleReflect = async () => {
  if (!requestConfig.value.address) {
    ElMessage.warning('请输入服务器地址')
    return
  }

  reflecting.value = true
  protoServices.value = []
  expandedServiceNames.value = []

  try {
    const res = await protoFileAPI.reflect({
      address: requestConfig.value.address,
      tls_mode: requestConfig.value.tls_mode,
      certificate_id: requestConfig.value.certificate_id ?? undefined,
    })
    const services = res.data.services || []
    protoServices.value = services
    expandedServiceNames.value = services.map((s: ProtoService) => s.name)
    ElMessage.success(`发现 ${services.length} 个服务`)
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '服务反射失败')
    }
  } finally {
    reflecting.value = false
  }
}

const handleSelectMethod = async (svcName: string, method: ProtoMethod) => {
  requestConfig.value.service = svcName
  requestConfig.value.method = method.name
  activeRequestTab.value = 'request'

  // Find the input message definition from messageDefs
  const inputType = method.input_type
  const msgDef = messageDefs.value.find(m =>
    m.full_name === inputType || m.name === inputType ||
    m.full_name.endsWith('.' + inputType)
  )
  currentMessageDef.value = msgDef || null
  formData.value = {}

  // Generate JSON template from message definition
  const jsonStr = msgDef ? generateTemplateFromMessage(msgDef) : '{}'
  requestConfig.value.request_body = jsonStr

  // Update JSON editor content
  nextTick(() => {
    if (requestEditorView) {
      requestEditorView.dispatch({
        changes: {
          from: 0,
          to: requestEditorView.state.doc.length,
          insert: jsonStr,
        },
      })
    }
  })
}

// Generate JSON template from a ProtoMessageDef
const generateTemplateFromMessage = (msgDef: ProtoMessageDef): string => {
  const obj = buildTemplateObject(msgDef)
  return JSON.stringify(obj, null, 2)
}

const buildTemplateObject = (msgDef: ProtoMessageDef): Record<string, unknown> => {
  const obj: Record<string, unknown> = {}
  for (const field of msgDef.fields) {
    obj[field.name] = buildFieldDefaultValue(field)
  }
  return obj
}

const buildFieldDefaultValue = (field: ProtoMessageField): unknown => {
  // Map
  if (field.label === 'map') return {}
  // Repeated
  if (field.label === 'repeated') return []
  // Nested message
  if (field.fields && field.fields.length > 0) {
    const obj: Record<string, unknown> = {}
    for (const f of field.fields) {
      obj[f.name] = buildFieldDefaultValue(f)
    }
    return obj
  }
  // Scalar
  if (field.type === 'bool') return false
  if (['int32', 'int64', 'uint32', 'uint64', 'sint32', 'sint64',
    'fixed32', 'fixed64', 'sfixed32', 'sfixed64', 'float', 'double'].includes(field.type)) return 0
  return ''
}

// --- Connect Test ---
const handleConnectTest = async () => {
  if (!requestConfig.value.address) {
    ElMessage.warning('请输入服务器地址')
    return
  }

  connecting.value = true
  try {
    const config: GRPCRequestConfig = {
      ...requestConfig.value,
      use_reflection: connectionMode.value === 'reflection',
      metadata: metadata.value.filter(m => m.key),
    }
    const res = await apiTestAPI.execute({
      type: 'grpc_connect_test',
      config: JSON.stringify(config),
      save_result: false,
    })
    const result = res.data
    if (result.error) {
      ElMessage.error(result.error)
    } else {
      ElMessage.success('连接成功')
    }
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '连接失败')
    }
  } finally {
    connecting.value = false
  }
}

// --- Send ---
const handleSend = async () => {
  if (!canSend.value) return

  sending.value = true
  try {
    // Determine request body based on mode
    let requestBody = requestConfig.value.request_body
    if (requestMode.value === 'form' && currentMessageDef.value) {
      requestBody = JSON.stringify(formData.value)
    }

    const config: GRPCRequestConfig = {
      ...requestConfig.value,
      request_body: requestBody,
      use_reflection: connectionMode.value === 'reflection',
      metadata: metadata.value.filter(m => m.key),
    }

    let res: any

    if (currentTestId.value) {
      // 已保存的测试：先更新配置，再调用 executeSaved 以正确关联 test_id
      await apiTestAPI.update(currentTestId.value, { config: JSON.stringify(config) })
      res = await apiTestAPI.executeSaved(currentTestId.value)
    } else {
      // 新请求：调用 execute
      res = await apiTestAPI.execute({
        type: 'grpc',
        config: JSON.stringify(config),
        save_result: true,
      })
    }

    lastResult.value = res.data
    activeResponseTab.value = 'body'

    if (currentTestId.value) {
      await loadHistory(currentTestId.value)
    }
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '请求执行失败')
    }
  } finally {
    sending.value = false
  }
}

// --- Save ---
const handleSave = () => {
  if (!canSave.value) return
  const existing = savedTests.value.find(t => t.id === currentTestId.value)
  saveName.value = existing?.name || `${requestConfig.value.service}/${requestConfig.value.method}`
  saveDialogVisible.value = true
}

const handleSaveConfirm = async () => {
  if (!saveName.value.trim()) {
    ElMessage.warning('请输入名称')
    return
  }

  saving.value = true
  try {
    const config: GRPCRequestConfig = {
      ...requestConfig.value,
      use_reflection: connectionMode.value === 'reflection',
      metadata: metadata.value.filter(m => m.key),
    }

    if (currentTestId.value) {
      await apiTestAPI.update(currentTestId.value, {
        name: saveName.value.trim(),
        type: 'grpc',
        config: JSON.stringify(config),
      })
      ElMessage.success('测试用例已更新')
    } else {
      const res = await apiTestAPI.create({
        name: saveName.value.trim(),
        type: 'grpc',
        config: JSON.stringify(config),
      })
      currentTestId.value = res.data.id
      ElMessage.success('测试用例已保存')
    }

    saveDialogVisible.value = false
    await loadSavedTests()
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

// --- Load / New / Delete ---
const handleLoadTest = async (item: ApiTest) => {
  currentTestId.value = item.id
  try {
    const config = JSON.parse(item.config) as GRPCRequestConfig
    requestConfig.value = {
      address: config.address || '',
      service: config.service || '',
      method: config.method || '',
      request_body: config.request_body || '{}',
      metadata: [],
      tls_mode: config.tls_mode || 'insecure',
      certificate_id: config.certificate_id ?? null,
      proto_file_id: config.proto_file_id ?? null,
      use_reflection: config.use_reflection || false,
      timeout: config.timeout || 30,
    }
    metadata.value = config.metadata || []
    connectionMode.value = config.use_reflection ? 'reflection' : 'proto'

    if (config.proto_file_id) {
      selectedProtoFileId.value = config.proto_file_id
      await handleProtoFileChange(config.proto_file_id)
    } else {
      protoServices.value = []
      selectedProtoFileId.value = null
    }

    // Update editor content
    if (requestEditorView) {
      requestEditorView.dispatch({
        changes: {
          from: 0,
          to: requestEditorView.state.doc.length,
          insert: requestConfig.value.request_body,
        },
      })
    }

    await loadHistory(item.id)
    lastResult.value = null
  } catch {
    ElMessage.error('加载测试用例失败')
  }
}

const handleNewTest = () => {
  currentTestId.value = null
  requestConfig.value = {
    address: '',
    service: '',
    method: '',
    request_body: '{}',
    metadata: [],
    tls_mode: 'insecure',
    certificate_id: null,
    proto_file_id: null,
    use_reflection: false,
    timeout: 30,
  }
  metadata.value = []
  protoServices.value = []
  selectedProtoFileId.value = null
  expandedServiceNames.value = []
  messageDefs.value = []
  currentMessageDef.value = null
  formData.value = {}
  lastResult.value = null
  historyResults.value = []

  if (requestEditorView) {
    requestEditorView.dispatch({
      changes: {
        from: 0,
        to: requestEditorView.state.doc.length,
        insert: '{}',
      },
    })
  }
}

const handleDeleteTest = async (item: ApiTest) => {
  try {
    await ElMessageBox.confirm(`确定要删除测试用例 "${item.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await apiTestAPI.delete(item.id)
    ElMessage.success('已删除')
    if (currentTestId.value === item.id) {
      handleNewTest()
    }
    await loadSavedTests()
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error((err as Error).message || '删除失败')
    }
  }
}

// --- CodeMirror Editor ---
const initRequestEditor = () => {
  if (!requestEditorRef.value) return

  const updateListener = EditorView.updateListener.of((update) => {
    if (update.docChanged) {
      requestConfig.value.request_body = update.state.doc.toString()
    }
  })

  const state = EditorState.create({
    doc: requestConfig.value.request_body,
    extensions: [
      basicSetup,
      json(),
      oneDark,
      updateListener,
      EditorView.theme({
        '&': { height: '100%', backgroundColor: '#282c34' },
        '.cm-content': { padding: '10px 12px', fontSize: '13px', fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace' },
        '.cm-line': { padding: '2px 0', lineHeight: '1.5' },
        '.cm-scroller': { overflow: 'auto', backgroundColor: '#282c34' },
      }),
    ],
  })

  requestEditorView = new EditorView({
    state,
    parent: requestEditorRef.value,
  })
}

const createResponseJsonEditor = () => {
  if (!responseEditorRef.value) return
  if (responseEditorView) responseEditorView.destroy()

  let formattedBody = ''
  try {
    const parsed = JSON.parse(lastResult.value?.body || '')
    formattedBody = JSON.stringify(parsed, null, 2)
  } catch {
    formattedBody = lastResult.value?.body || ''
  }

  const state = EditorState.create({
    doc: formattedBody,
    extensions: [
      basicSetup,
      json(),
      oneDark,
      EditorState.readOnly.of(true),
      EditorView.theme({
        '&': { height: '100%', backgroundColor: '#282c34' },
        '.cm-scroller': { overflow: 'auto', backgroundColor: '#282c34' },
      }),
    ],
  })

  responseEditorView = new EditorView({ state, parent: responseEditorRef.value })
}

// --- Lifecycle ---
const handleGlobalKeydown = (e: KeyboardEvent) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault()
    handleSend()
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    handleSave()
  }
}

onMounted(async () => {
  await Promise.all([loadSavedTests(), loadProtoFiles(), loadCertificates()])
  await nextTick()
  initRequestEditor()
  document.addEventListener('keydown', handleGlobalKeydown)
})

onUnmounted(() => {
  if (requestEditorView) {
    requestEditorView.destroy()
    requestEditorView = null
  }
  if (responseEditorView) {
    responseEditorView.destroy()
    responseEditorView = null
  }
  document.removeEventListener('keydown', handleGlobalKeydown)
})

// Sync metadata to requestConfig before send/save
watch(metadata, (val) => {
  requestConfig.value.metadata = val.filter(m => m.key)
}, { deep: true })

// When switching to JSON mode, generate JSON from message definition
watch(requestMode, (mode) => {
  if (mode === 'json') {
    // Generate formatted JSON from message definition (not from empty formData)
    const jsonStr = currentMessageDef.value
      ? generateTemplateFromMessage(currentMessageDef.value)
      : requestConfig.value.request_body
    requestConfig.value.request_body = jsonStr
    nextTick(() => {
      if (requestEditorView) {
        requestEditorView.dispatch({
          changes: {
            from: 0,
            to: requestEditorView.state.doc.length,
            insert: jsonStr,
          },
        })
      }
    })
  }
})

// Clear state when connection mode changes
watch(connectionMode, () => {
  protoServices.value = []
  requestConfig.value.service = ''
  requestConfig.value.method = ''
  expandedServiceNames.value = []
})

// Watch lastResult to create response JSON editor
watch(lastResult, () => {
  nextTick(() => {
    if (lastResult.value && !lastResult.value.error && activeResponseTab.value === 'body') {
      createResponseJsonEditor()
    }
  })
})

watch(activeResponseTab, () => {
  if (activeResponseTab.value === 'body' && lastResult.value && !lastResult.value.error) {
    nextTick(() => createResponseJsonEditor())
  }
})

// Watch responseBodyMode to recreate JSON editor when switching back from raw
watch(responseBodyMode, () => {
  if (responseBodyMode.value === 'json' && lastResult.value && !lastResult.value.error) {
    nextTick(() => createResponseJsonEditor())
  }
})
</script>

<style scoped>
/* ==================== CSS Variables ==================== */
.grpc-test-page {
  --panel-width: 250px;
  --panel-min-width: 200px;
  --panel-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  --section-radius: 10px;
  --transition-tab: 0.2s ease;
  --spacing-xs: 4px;
  --spacing-sm: 8px;
  --spacing-md: 12px;
  --spacing-lg: 16px;
  --spacing-xl: 24px;

  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  margin: calc(-1 * var(--space-6));
}

.main-content {
  display: flex;
  flex: 1;
  gap: var(--spacing-lg);
  min-height: 0;
  align-items: stretch;
  padding: var(--spacing-lg);
}

/* ==================== Left Panel ==================== */
.saved-panel {
  width: var(--panel-width);
  min-width: var(--panel-min-width);
  flex-shrink: 0;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--section-radius);
  box-shadow: var(--panel-shadow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: box-shadow var(--transition-tab);
}

.saved-panel:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

.panel-title {
  font-weight: 600;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

.panel-count-badge :deep(.el-badge__content) {
  font-size: 11px;
}

.panel-search {
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--border-subtle);
}

.saved-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-sm);
}

.saved-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-md);
  border-left: 3px solid transparent;
  cursor: pointer;
  transition: all var(--duration-fast);
}

.saved-item:hover {
  background: var(--bg-hover);
  transform: translateX(2px);
}

.saved-item.active {
  background: rgba(99, 102, 241, 0.1);
  border-left-color: #6366f1;
}

.saved-item:hover .saved-delete {
  opacity: 1;
}

.grpc-badge {
  flex-shrink: 0;
  font-size: 11px;
  min-width: 42px;
  text-align: center;
  background: #6366f1;
  border-color: #6366f1;
}

.saved-item-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.saved-name {
  flex: 1;
  font-size: 13px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

.saved-addr {
  font-size: 11px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.saved-delete {
  opacity: 0;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: all var(--duration-fast);
  cursor: pointer;
}

.saved-delete:hover {
  color: var(--accent-danger);
}

.empty-saved {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-xl);
  color: var(--text-muted);
  gap: var(--spacing-sm);
}

.empty-saved p {
  font-size: var(--font-size-sm);
  margin: 0;
}

.panel-footer {
  padding: var(--spacing-md);
  border-top: 1px solid var(--border-subtle);
}

.new-btn {
  width: 100%;
}

/* ==================== Right Panel ==================== */
.editor-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

/* ==================== Request Section ==================== */
.request-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--section-radius);
  box-shadow: var(--panel-shadow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex-shrink: 1;
  max-height: 55%;
  min-height: 180px;
  transition: max-height 0.3s ease, min-height 0.3s ease, flex 0.3s ease;
}

.request-section.panel-maximized {
  flex: 1;
  max-height: none;
  min-height: 0;
}

.request-section.panel-minimized {
  flex: 0;
  max-height: 0;
  min-height: 0;
  overflow: hidden;
  border: none;
  padding: 0;
  margin: 0;
}

/* ==================== Resize Handle ==================== */
.resize-handle {
  flex-shrink: 0;
  height: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: row-resize;
  position: relative;
  z-index: 10;
}

.resize-handle:hover .resize-line,
.resize-handle:active .resize-line {
  background: #6366f1;
  height: 3px;
}

.resize-line {
  width: 40px;
  height: 2px;
  border-radius: 2px;
  background: var(--border-default);
  transition: all 0.2s ease;
}

.panel-toggle-btn {
  margin-left: auto;
  flex-shrink: 0;
}

.current-test-name {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) var(--spacing-lg);
  background: rgba(99, 102, 241, 0.06);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 12px;
  color: #6366f1;
  font-weight: 500;
}

.request-bar {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
  flex-wrap: wrap;
}

.bar-divider {
  height: 24px;
  margin: 0 2px;
}

.address-input {
  flex: 1;
}

.mode-select {
  width: 140px;
  flex-shrink: 0;
}

.timeout-input {
  width: 100px;
  flex-shrink: 0;
}

.timeout-hint {
  color: var(--text-muted);
  cursor: help;
  flex-shrink: 0;
}

.send-btn {
  font-weight: 600;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
}

.send-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(99, 102, 241, 0.4);
  filter: brightness(1.05);
}

.action-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  transition: all var(--duration-normal) var(--ease-out);
}

.action-btn:hover {
  border-color: #6366f1;
  color: #6366f1;
}

.request-tabs {
  padding: 0 var(--spacing-lg);
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.request-tabs :deep(.el-tabs__header) {
  flex-shrink: 0;
}

.request-tabs :deep(.el-tabs__content) {
  padding: var(--spacing-md) 0;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  transition: opacity var(--transition-tab);
}

.request-tabs :deep(.el-tab-pane) {
  min-height: 100%;
}

/* ==================== Service Panel ==================== */
.service-panel {
  padding: var(--spacing-md);
}

.proto-selector {
  margin-bottom: var(--spacing-md);
}

.proto-select {
  width: 100%;
}

.service-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xl);
  color: var(--text-muted);
}

.service-tree {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.service-collapse {
  border: none;
}

.service-collapse :deep(.el-collapse-item__header) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md) var(--radius-md) 0 0;
  padding: 0 var(--spacing-md);
  height: 36px;
  line-height: 36px;
  font-size: 0.85rem;
  font-weight: 500;
  transition: background var(--duration-fast) var(--ease-out);
}

.service-collapse :deep(.el-collapse-item__header:hover) {
  background: var(--bg-hover);
}

.service-collapse :deep(.el-collapse-item__wrap) {
  border: 1px solid var(--border-subtle);
  border-top: none;
  border-radius: 0 0 var(--radius-md) var(--radius-md);
  overflow: hidden;
}

.service-collapse :deep(.el-collapse-item__content) {
  padding: var(--spacing-xs) 0;
}

.service-name {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--text-primary);
}

.svc-icon {
  color: #6366f1;
}

.method-count-tag {
  margin-left: auto;
  font-size: 11px;
}

.method-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md) var(--spacing-sm) var(--spacing-xl);
  cursor: pointer;
  font-size: 0.82rem;
  transition: all var(--duration-fast) var(--ease-out);
  border-left: 3px solid transparent;
}

.method-item:hover {
  background: var(--bg-hover);
}

.method-item.active {
  background: rgba(99, 102, 241, 0.08);
  color: #6366f1;
  border-left-color: #6366f1;
}

.method-icon {
  color: var(--accent-success);
  flex-shrink: 0;
}

.method-name {
  font-weight: 500;
  color: var(--text-primary);
}

.method-item.active .method-name {
  color: #6366f1;
}

.method-type {
  color: var(--text-muted);
  font-size: 0.72rem;
  margin-left: auto;
  white-space: nowrap;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.stream-tag {
  margin-left: var(--spacing-sm);
}

.reflection-bar {
  margin-bottom: var(--spacing-md);
}

/* ==================== Request Mode ==================== */
.request-mode-bar {
  margin-bottom: var(--spacing-sm);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.form-wrapper {
  min-height: 200px;
}

.form-container {
  padding: var(--spacing-sm) 0;
}

.form-message-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-md);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-radius: var(--radius-sm);
  background: rgba(99, 102, 241, 0.05);
  border: 1px solid rgba(99, 102, 241, 0.1);
  font-size: 13px;
  font-weight: 500;
  color: #6366f1;
}

.editor-wrapper {
  height: 100%;
}

/* ==================== Editor ==================== */
.editor-container {
  height: 100%;
  overflow: hidden;
}

/* ==================== Metadata Panel ==================== */
.metadata-panel {
  padding: var(--spacing-md);
}

/* ==================== TLS Panel ==================== */
.tls-panel {
  padding: var(--spacing-md);
}

.tls-mode-wrapper {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  width: 100%;
}

.tls-mode-select {
  width: 100%;
}

.tls-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.tls-option-label {
  flex: 1;
}

.tls-option-tag {
  margin-left: var(--spacing-sm);
}

.tls-security-indicator {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  font-size: 0.8rem;
  font-weight: 500;
}

.security-danger { color: var(--accent-danger); }
.security-warning { color: var(--accent-warning); }
.security-success { color: var(--accent-success); }

.tls-mode-desc {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-md);
  font-size: 0.8rem;
  line-height: 1.5;
}

.tls-mode-desc-danger {
  background: rgba(245, 108, 108, 0.06);
  color: var(--accent-danger);
  border: 1px solid rgba(245, 108, 108, 0.15);
}

.tls-mode-desc-warning {
  background: rgba(230, 162, 60, 0.06);
  color: var(--accent-warning);
  border: 1px solid rgba(230, 162, 60, 0.15);
}

.tls-mode-desc-success {
  background: rgba(103, 194, 58, 0.06);
  color: var(--accent-success);
  border: 1px solid rgba(103, 194, 58, 0.15);
}

.cert-select {
  width: 100%;
}

.cert-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.cert-option-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cert-badges {
  margin-left: auto;
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

/* ==================== Response Section ==================== */
.response-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-top: 2px solid #6366f1;
  border-radius: var(--section-radius);
  box-shadow: var(--panel-shadow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex: 1;
  min-height: 250px;
  background: linear-gradient(180deg, rgba(99, 102, 241, 0.02) 0%, var(--bg-card) 100%);
  transition: flex 0.3s ease, min-height 0.3s ease;
}

.response-section.panel-maximized {
  flex: 1;
  min-height: 0;
}

.response-section.panel-minimized {
  flex: 0;
  min-height: 0;
  max-height: 0;
  overflow: hidden;
  border: none;
  padding: 0;
  margin: 0;
}

.response-fade-enter-active {
  transition: all 0.3s ease-out;
}

.response-fade-leave-active {
  transition: all 0.2s ease-in;
}

.response-fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.response-fade-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}

.response-header {
  padding: var(--spacing-sm) var(--spacing-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.response-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.status-badge {
  font-weight: 700;
  font-size: 12px;
  min-width: 44px;
  text-align: center;
  padding: 2px 8px;
}

.response-meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.status-code {
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.status-ok { color: var(--accent-success); }
.status-error { color: var(--accent-danger); }

.copy-response-btn {
  color: var(--text-muted);
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 2px 8px;
  transition: all var(--duration-fast);
  flex-shrink: 0;
}

.copy-response-btn:hover {
  color: #6366f1;
  border-color: #6366f1;
}

.response-tabs {
  padding: 0 var(--spacing-lg);
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.response-tabs :deep(.el-tabs__header) {
  flex-shrink: 0;
}

.response-tabs :deep(.el-tabs__content) {
  padding: var(--spacing-md) 0;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  transition: opacity var(--transition-tab);
}

.response-tabs :deep(.el-tab-pane) {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.response-body-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.response-body-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-xs) 0 var(--spacing-sm);
  flex-shrink: 0;
}

.response-body-modes {
  display: flex;
  align-items: center;
}

.response-body-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

.response-raw-wrapper {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.response-raw {
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  padding: var(--spacing-sm);
  background: var(--bg-primary);
  border-radius: var(--radius-md);
}

.response-json-editor {
  flex: 1;
  min-height: 0;
}

.error-content {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  color: var(--accent-danger);
}

.error-content pre {
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 0.82rem;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  color: var(--accent-danger);
  background: rgba(239, 68, 68, 0.05);
  padding: var(--spacing-md);
  border-radius: var(--radius-md);
  flex: 1;
}

/* ==================== Response Headers ==================== */
.response-headers {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.header-row {
  display: flex;
  gap: var(--spacing-md);
  padding: var(--spacing-xs) 0;
  font-size: 13px;
  border-bottom: 1px solid var(--border-subtle);
}

.header-key {
  font-weight: 600;
  color: var(--text-primary);
  min-width: 200px;
}

.header-value {
  color: var(--text-secondary);
  word-break: break-all;
}

.body-none {
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--text-muted);
  font-size: var(--font-size-sm);
}

/* ==================== History ==================== */
.history-toolbar {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

.history-item {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: background var(--duration-fast);
  font-size: 13px;
}

.history-item:hover {
  background: var(--bg-hover);
}

.history-item.active {
  background: rgba(99, 102, 241, 0.1);
}

.history-code {
  font-size: 12px;
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.history-latency {
  color: var(--text-secondary);
  min-width: 60px;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.history-time {
  color: var(--text-muted);
  margin-left: auto;
  font-size: 12px;
}

/* ==================== Empty State ==================== */
.empty-response {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--bg-card);
  border: 1px dashed var(--border-subtle);
  border-radius: var(--section-radius);
  padding: var(--spacing-xl);
  color: var(--text-muted);
}

.empty-response h3 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-primary);
  margin: var(--spacing-md) 0 var(--spacing-sm);
}

.empty-response p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
}

/* ==================== Compare Dialog ==================== */
.compare-container {
  display: flex;
  gap: var(--spacing-lg);
}

.compare-panel {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.compare-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 13px;
}

.compare-code {
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 12px;
}

.compare-latency {
  color: var(--text-secondary);
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.compare-time {
  color: var(--text-muted);
  font-size: 12px;
  margin-left: auto;
}

.compare-body {
  padding: var(--spacing-md);
  margin: 0;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 400px;
  overflow: auto;
  background: var(--bg-primary);
}
</style>
