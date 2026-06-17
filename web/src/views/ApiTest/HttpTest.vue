<template>
  <div class="http-test-page">
    <div class="main-content">
      <!-- Left Panel: Saved Tests -->
      <aside class="saved-panel">
        <div class="panel-header">
          <span class="panel-title">接口列表</span>
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
            :style="{ borderLeftColor: getMethodColor(getConfigMethod(item.config)) }"
            @click="handleLoadTest(item)"
          >
            <el-icon class="drag-handle"><Rank /></el-icon>
            <el-tag
              :type="getMethodTagType(getConfigMethod(item.config))"
              size="small"
              effect="dark"
              class="method-badge"
              disable-transitions
            >
              {{ getConfigMethod(item.config) }}
            </el-tag>
            <span class="saved-name" :title="item.name">{{ item.name }}</span>
            <el-icon class="saved-delete" @click.stop="handleDeleteTest(item)">
              <Delete />
            </el-icon>
          </div>
          <div v-if="filteredTests.length === 0" class="empty-saved">
            <el-icon :size="24"><Document /></el-icon>
            <p>暂无保存的接口</p>
          </div>
        </div>
        <div class="panel-footer">
          <el-button type="primary" :icon="Plus" size="small" @click="handleNewRequest" class="new-btn">
            新建请求
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
          <!-- Current test name -->
          <div v-if="currentTestName" class="current-test-name">
            <el-icon :size="14"><Document /></el-icon>
            <span>{{ currentTestName }}</span>
          </div>

          <!-- Top Bar -->
          <div class="request-bar">
            <el-select
              v-model="requestConfig.method"
              class="method-select"
              size="default"
            >
              <el-option
                v-for="m in httpMethods"
                :key="m"
                :label="m"
                :value="m"
              >
                <span :class="`method-text method-${m.toLowerCase()}`">{{ m }}</span>
              </el-option>
            </el-select>
            <el-divider direction="vertical" class="bar-divider" />
            <el-input
              v-model="requestConfig.url"
              placeholder="请求 URL"
              class="url-input"
              @keydown.enter="handleSend"
              @blur="syncUrlToParams"
            >
              <template #prefix>
                <el-icon><Link /></el-icon>
              </template>
            </el-input>
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
              :icon="Promotion"
              @click="handleSend"
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
            <el-tooltip content="保存接口" placement="bottom">
              <el-button :icon="FolderOpened" @click="handleSave" :loading="saving" class="action-btn">
                保存
              </el-button>
            </el-tooltip>
            <el-tooltip content="生成 cURL 命令" placement="bottom">
              <el-button :icon="DocumentCopy" @click="handleGenerateCurl" class="action-btn">
                cURL
              </el-button>
            </el-tooltip>
            <el-tooltip :content="panelMode === 'request-max' ? '恢复' : '最大化请求'" placement="bottom">
              <el-button
                :icon="panelMode === 'request-max' ? ArrowDown : ArrowUp"
                @click="togglePanelMode('request-max')"
                class="action-btn panel-toggle-btn"
                size="small"
              />
            </el-tooltip>
          </div>

          <!-- Request Tabs -->
          <el-tabs v-model="requestTab" class="request-tabs">
            <!-- Params Tab -->
            <el-tab-pane name="params">
              <template #label>
                参数 <el-badge v-if="activeParamsCount > 0" :value="activeParamsCount" type="info" class="tab-badge" />
              </template>
              <KVEditor v-model="requestConfig.params" key-label="参数名" value-label="参数值" add-label="添加参数" @update:model-value="syncParamsToUrl" />
            </el-tab-pane>

            <!-- Headers Tab -->
            <el-tab-pane name="headers">
              <template #label>
                请求头 <el-badge v-if="activeHeadersCount > 0" :value="activeHeadersCount" type="info" class="tab-badge" />
              </template>
              <KVEditor v-model="requestConfig.headers" key-label="Header" value-label="Value" add-label="添加" />
            </el-tab-pane>

            <!-- Body Tab -->
            <el-tab-pane name="body">
              <template #label>
                请求体 <el-badge v-if="requestConfig.bodyType !== 'none'" value="*" type="warning" class="tab-badge" />
              </template>
              <div class="body-section">
                <div class="body-type-row">
                  <el-radio-group v-model="requestConfig.bodyType" size="small" class="body-type-group">
                    <el-radio-button value="none">none</el-radio-button>
                    <el-radio-button value="json">json</el-radio-button>
                    <el-radio-button value="form-urlencoded">x-www-form</el-radio-button>
                    <el-radio-button value="form-multipart">form-data</el-radio-button>
                    <el-radio-button value="raw">raw</el-radio-button>
                    <el-radio-button value="binary">binary</el-radio-button>
                  </el-radio-group>
                  <span class="content-type-hint">{{ contentTypeHint }}</span>
                </div>

                <!-- JSON: CodeMirror with format button -->
                <div
                  v-if="requestConfig.bodyType === 'json'"
                  class="body-editor-wrapper"
                >
                  <div class="body-editor-toolbar">
                    <el-button size="small" text @click="formatJsonBody">
                      <el-icon><MagicStick /></el-icon>
                      格式化
                    </el-button>
                  </div>
                  <div ref="bodyEditorRef" class="body-editor"></div>
                </div>

                <!-- Raw: CodeMirror with content-type input -->
                <div
                  v-if="requestConfig.bodyType === 'raw'"
                  class="body-editor-wrapper"
                >
                  <div class="body-editor-toolbar">
                    <el-input
                      v-model="requestConfig.rawContentType"
                      size="small"
                      placeholder="Content-Type"
                      class="raw-content-type-input"
                    />
                  </div>
                  <div ref="bodyEditorRef" class="body-editor"></div>
                </div>

                <!-- form-urlencoded / form-multipart: KV editor -->
                <KVEditor v-if="requestConfig.bodyType === 'form-urlencoded' || requestConfig.bodyType === 'form-multipart'" v-model="requestConfig.bodyForm" key-label="参数名" value-label="参数值" add-label="添加" />

                <!-- binary: file upload -->
                <div v-if="requestConfig.bodyType === 'binary'" class="binary-upload">
                  <el-upload
                    :auto-upload="false"
                    :show-file-list="false"
                    @change="handleBinaryFileChange"
                    accept="*"
                  >
                    <el-button type="primary" size="small">选择文件</el-button>
                  </el-upload>
                  <span v-if="binaryFileName" class="binary-file-name">{{ binaryFileName }}</span>
                  <el-icon v-if="binaryFileName" class="binary-clear" @click="clearBinaryFile"><Close /></el-icon>
                </div>

                <!-- none -->
                <div v-if="requestConfig.bodyType === 'none'" class="body-none">该请求没有 Body</div>
              </div>
            </el-tab-pane>

            <!-- Auth Tab -->
            <el-tab-pane name="auth">
              <template #label>
                认证 <el-badge v-if="requestConfig.authType !== 'none'" value="*" type="success" class="tab-badge" />
              </template>
              <div class="auth-section">
                <el-select v-model="requestConfig.authType" size="small" class="auth-type-select">
                  <el-option label="无认证" value="none" />
                  <el-option label="Bearer Token" value="bearer" />
                  <el-option label="Basic Auth" value="basic" />
                  <el-option label="API Key" value="apikey" />
                </el-select>

                <div v-if="requestConfig.authType === 'bearer'" class="auth-fields">
                  <el-input v-model="requestConfig.auth.token" placeholder="Token" size="small" />
                </div>
                <div v-else-if="requestConfig.authType === 'basic'" class="auth-fields">
                  <el-input v-model="requestConfig.auth.user" placeholder="用户名" size="small" />
                  <el-input v-model="requestConfig.auth.pass" placeholder="密码" size="small" type="password" show-password />
                </div>
                <div v-else-if="requestConfig.authType === 'apikey'" class="auth-fields">
                  <el-input v-model="requestConfig.auth.key" placeholder="Key 名称" size="small" />
                  <el-input v-model="requestConfig.auth.value" placeholder="Key 值" size="small" />
                  <el-select v-model="requestConfig.auth.in" size="small" class="auth-in-select">
                    <el-option label="Header" value="header" />
                    <el-option label="Query" value="query" />
                  </el-select>
                </div>
              </div>
            </el-tab-pane>

            <!-- Pre-Script Tab -->
            <el-tab-pane label="前置脚本" name="pre-script">
              <el-input
                v-model="requestConfig.preScript"
                type="textarea"
                :rows="8"
                placeholder="预请求脚本（JavaScript）"
                class="script-textarea"
              />
            </el-tab-pane>

            <!-- Post-Script Tab -->
            <el-tab-pane label="后置脚本" name="post-script">
              <el-input
                v-model="requestConfig.postScript"
                type="textarea"
                :rows="8"
                placeholder="后置脚本（可用于环境变量设置等）"
                class="script-textarea"
              />
            </el-tab-pane>

            <!-- Assertions Tab -->
            <el-tab-pane name="assertions">
              <template #label>
                断言 <el-badge v-if="requestConfig.assertions.length > 0" :value="requestConfig.assertions.length" type="danger" class="tab-badge" />
              </template>
              <div class="assertions-editor">
                <div v-for="(assertion, index) in requestConfig.assertions" :key="index" class="assertion-row">
                  <el-select v-model="assertion.type" size="small" class="assertion-field assertion-type">
                    <el-option label="状态码" value="status_code" />
                    <el-option label="JSON Path" value="json_path" />
                    <el-option label="Header" value="header" />
                  </el-select>
                  <el-input v-if="assertion.type !== 'status_code'" v-model="assertion.target" placeholder="目标" size="small" class="assertion-field assertion-target" />
                  <el-select v-model="assertion.operator" size="small" class="assertion-field assertion-operator">
                    <el-option label="等于" value="equals" />
                    <el-option label="不等于" value="not_equals" />
                    <el-option label="包含" value="contains" />
                    <el-option label="大于" value="gt" />
                    <el-option label="小于" value="lt" />
                    <el-option label="存在" value="exists" />
                  </el-select>
                  <el-input v-if="assertion.operator !== 'exists'" v-model="assertion.expected" placeholder="期望值" size="small" class="assertion-field assertion-expected" />
                  <el-icon class="kv-remove" @click="requestConfig.assertions.splice(index, 1)"><Close /></el-icon>
                </div>
                <el-button :icon="Plus" size="small" text @click="addAssertion" class="kv-add">添加断言</el-button>
              </div>
            </el-tab-pane>
          </el-tabs>
        </div>

        <!-- Resize Handle -->
        <div
          v-if="response || responseError || sending"
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
            v-if="response || responseError || sending"
          >
            <div class="response-header">
              <div class="response-info">
                <template v-if="response">
                  <el-tag
                    :type="getStatusTagType(response.status_code)"
                    effect="dark"
                    size="large"
                    class="status-badge"
                  >
                    {{ response.status_code }}
                  </el-tag>
                  <span class="response-meta-item">
                    <el-icon :size="14"><Clock /></el-icon>
                    {{ response.latency_ms }}ms
                  </span>
                  <span class="response-meta-item">
                    <el-icon :size="14"><Document /></el-icon>
                    {{ formatSize(response.body) }}
                  </span>
                </template>
                <template v-if="responseError">
                  <el-tag type="danger" effect="dark" size="small">Error</el-tag>
                  <span class="error-text">{{ responseError }}</span>
                </template>
                <template v-if="sending && !response">
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

            <el-tabs v-model="responseTab" class="response-tabs">
              <!-- Body Tab -->
              <el-tab-pane name="body">
                <template #label>
                  响应体
                </template>
                <div class="response-body-header">
                  <div class="response-body-modes">
                    <el-radio-group v-model="responseBodyMode" size="small">
                      <el-radio-button value="json">JSON</el-radio-button>
                      <el-radio-button value="raw">Raw</el-radio-button>
                      <el-radio-button value="preview">Preview</el-radio-button>
                    </el-radio-group>
                  </div>
                  <el-button
                    v-if="response?.body"
                    size="small"
                    text
                    class="copy-response-btn"
                    @click="copyResponseBody"
                  >
                    <el-icon><DocumentCopy /></el-icon>
                    复制
                  </el-button>
                </div>
                <div v-if="responseBodyMode === 'json'" class="response-body-content">
                  <div ref="responseJsonEditorRef" class="response-json-editor"></div>
                </div>
                <div v-else-if="responseBodyMode === 'raw'" class="response-body-content">
                  <pre class="response-raw">{{ response?.body || '' }}</pre>
                </div>
                <div v-else class="response-body-content">
                  <iframe
                    v-if="response?.body"
                    :srcdoc="response.body"
                    class="response-preview"
                    sandbox=""
                  />
                  <div v-else class="body-none">无预览内容</div>
                </div>
              </el-tab-pane>

              <!-- Headers Tab -->
              <el-tab-pane label="响应头" name="headers">
                <div class="response-headers">
                  <div v-for="(value, key) in parsedResponseHeaders" :key="key" class="header-row">
                    <span class="header-key">{{ key }}</span>
                    <span class="header-value">{{ value }}</span>
                  </div>
                  <div v-if="Object.keys(parsedResponseHeaders).length === 0" class="body-none">无响应头</div>
                </div>
              </el-tab-pane>

              <!-- Assertions Tab -->
              <el-tab-pane label="断言结果" name="assertions">
                <div class="assertions-results">
                  <div
                    v-for="(result, index) in assertionResults"
                    :key="index"
                    class="assertion-result"
                    :class="{ pass: result.passed, fail: !result.passed }"
                  >
                    <el-icon :size="16">
                      <CircleCheck v-if="result.passed" />
                      <CircleClose v-else />
                    </el-icon>
                    <span class="assertion-result-text">
                      {{ formatAssertionResult(result) }}
                    </span>
                  </div>
                  <div v-if="assertionResults.length === 0" class="body-none">无断言结果</div>
                </div>
              </el-tab-pane>

              <!-- History Tab -->
              <el-tab-pane label="历史" name="history">
                <div class="history-toolbar" v-if="historyList.length > 0">
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
                    v-for="item in historyList"
                    :key="item.id"
                    class="history-item"
                    :class="{ active: selectedHistoryId === item.id }"
                    @click="handleSelectHistory(item)"
                  >
                    <el-checkbox
                      :model-value="selectedHistoryIds.includes(item.id)"
                      @change="(val: boolean) => toggleHistorySelect(item.id, val)"
                      @click.stop
                      size="small"
                    />
                    <el-tag
                      :type="getStatusTagType(item.status_code)"
                      size="small"
                      effect="light"
                      class="history-status"
                      :class="`status-code-${Math.floor(item.status_code / 100)}xx`"
                      disable-transitions
                    >
                      {{ item.status_code }}
                    </el-tag>
                    <span class="history-latency">{{ item.latency_ms }}ms</span>
                    <span class="history-time" :title="formatDateTime(item.created_at)">{{ formatRelativeTime(item.created_at) }}</span>
                  </div>
                  <div v-if="historyList.length === 0" class="body-none">暂无执行历史</div>
                </div>
              </el-tab-pane>
            </el-tabs>
          </div>
        </transition>

        <!-- Empty State -->
        <div v-if="!response && !responseError && !sending" class="empty-response">
          <el-icon :size="48"><Promotion /></el-icon>
          <h3>暂无响应</h3>
          <p>输入 URL 并点击发送按钮查看响应</p>
        </div>
      </main>
    </div>

    <!-- Save Dialog -->
    <el-dialog v-model="saveDialogVisible" title="保存接口" width="440px" :close-on-click-modal="false">
      <el-form :model="saveForm" label-position="top">
        <el-form-item label="名称" required>
          <el-input v-model="saveForm.name" placeholder="请输入接口名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="saveDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleConfirmSave" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <!-- cURL Dialog -->
    <el-dialog v-model="curlDialogVisible" title="cURL 命令" width="600px">
      <div class="curl-content">
        <pre class="curl-text">{{ curlCommand }}</pre>
      </div>
      <template #footer>
        <el-button @click="curlDialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="copyCurl">复制</el-button>
      </template>
    </el-dialog>

    <!-- Compare Dialog -->
    <el-dialog v-model="compareDialogVisible" title="历史对比" width="900px">
      <div v-if="compareLeft && compareRight" class="compare-container">
        <div class="compare-panel">
          <div class="compare-header">
            <el-tag :type="getStatusTagType(compareLeft.status_code)" size="small" effect="dark">{{ compareLeft.status_code }}</el-tag>
            <span class="compare-latency">{{ compareLeft.latency_ms }}ms</span>
            <span class="compare-time">{{ formatDateTime(compareLeft.created_at) }}</span>
          </div>
          <pre class="compare-body">{{ formatCompareBody(compareLeft.body) }}</pre>
        </div>
        <div class="compare-panel">
          <div class="compare-header">
            <el-tag :type="getStatusTagType(compareRight.status_code)" size="small" effect="dark">{{ compareRight.status_code }}</el-tag>
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
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search, Delete, Document, Plus, Close, Link, Promotion,
  FolderOpened, DocumentCopy, Clock, Refresh, CircleCheck, CircleClose,
  Rank, MagicStick, ArrowUp, ArrowDown
} from '@element-plus/icons-vue'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { json } from '@codemirror/lang-json'
import { oneDark } from '@codemirror/theme-one-dark'
import { apiTestAPI } from '@/api/apiTest'
import { isHandledError } from '@/utils/api'
import { formatDateTime } from '@/utils/format'
import KVEditor from './KVEditor.vue'
import type { ApiTest, ApiTestResult, AssertionConfig, AssertionResult, HTTPRequestConfig } from '@/api/apiTest'

// ==================== Types ====================
interface KVPair {
  key: string
  value: string
}

interface RequestConfigLocal {
  method: string
  url: string
  headers: KVPair[]
  params: KVPair[]
  bodyType: 'none' | 'json' | 'form-urlencoded' | 'form-multipart' | 'raw' | 'binary'
  bodyContent: string
  bodyForm: KVPair[]
  bodyBinary: string
  rawContentType: string
  authType: 'none' | 'bearer' | 'basic' | 'apikey'
  auth: {
    token: string
    user: string
    pass: string
    key: string
    value: string
    in: 'header' | 'query'
  }
  preScript: string
  postScript: string
  timeout: number
  assertions: AssertionConfig[]
}

// ==================== Constants ====================
const httpMethods = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']

const methodTagTypeMap: Record<string, string> = {
  GET: 'success',
  POST: 'warning',
  PUT: 'primary',
  DELETE: 'danger',
  PATCH: 'warning',
  HEAD: 'info',
  OPTIONS: 'info',
}

const methodColorMap: Record<string, string> = {
  GET: '#67c23a',
  POST: '#e6a23c',
  PUT: '#409eff',
  DELETE: '#f56c6c',
  PATCH: '#ff9800',
  HEAD: '#909399',
  OPTIONS: '#909399',
}

const getMethodTagType = (method: string): string => methodTagTypeMap[method] || 'info'

const getMethodColor = (method: string): string => methodColorMap[method] || '#909399'

const getStatusTagType = (code: number): string => {
  if (code >= 200 && code < 300) return 'success'
  if (code >= 300 && code < 400) return 'info'
  if (code >= 400 && code < 500) return 'warning'
  return 'danger'
}

// ==================== Reactive State ====================
const searchQuery = ref('')
const savedTests = ref<ApiTest[]>([])
const currentTestId = ref<number | null>(null)
const sending = ref(false)
const saving = ref(false)
const response = ref<ApiTestResult | null>(null)
const responseError = ref('')
const requestTab = ref('params')
const responseTab = ref('body')
const responseBodyMode = ref('json')
const binaryFileName = ref('')

// Panel resize state
const panelMode = ref<'both' | 'request-max' | 'response-max'>('both')
const requestSectionRef = ref<HTMLElement | null>(null)
const isDragging = ref(false)

const requestConfig = ref<RequestConfigLocal>(createEmptyRequest())

const saveDialogVisible = ref(false)
const saveForm = ref({ name: '' })
const curlDialogVisible = ref(false)
const curlCommand = ref('')

const assertionResults = ref<AssertionResult[]>([])
const historyList = ref<ApiTestResult[]>([])
const selectedHistoryId = ref<number | null>(null)

// History comparison
const selectedHistoryIds = ref<number[]>([])
const compareDialogVisible = ref(false)
const compareLeft = ref<ApiTestResult | null>(null)
const compareRight = ref<ApiTestResult | null>(null)

// CodeMirror refs
const bodyEditorRef = ref<HTMLElement>()
const responseJsonEditorRef = ref<HTMLElement>()
let bodyEditorView: EditorView | null = null
let responseJsonEditorView: EditorView | null = null

// AbortController for request cancellation
let abortController: AbortController | null = null

// ==================== Helpers ====================
function createEmptyRequest(): RequestConfigLocal {
  return {
    method: 'GET',
    url: '',
    headers: [],
    params: [],
    bodyType: 'none',
    bodyContent: '',
    bodyForm: [],
    bodyBinary: '',
    rawContentType: 'text/plain',
    authType: 'none',
    auth: { token: '', user: '', pass: '', key: '', value: '', in: 'header' },
    preScript: '',
    postScript: '',
    timeout: 30,
    assertions: [],
  }
}

function addAssertion() {
  requestConfig.value.assertions.push({
    type: 'status_code',
    target: '',
    operator: 'equals',
    expected: '',
  })
}

const filteredTests = computed(() => {
  if (!searchQuery.value) return savedTests.value
  const q = searchQuery.value.toLowerCase()
  return savedTests.value.filter(t => t.name.toLowerCase().includes(q))
})

const activeParamsCount = computed(() =>
  requestConfig.value.params.filter(p => p.key.trim()).length
)

const activeHeadersCount = computed(() =>
  requestConfig.value.headers.filter(h => h.key.trim()).length
)

const currentTestName = computed(() => {
  if (!currentTestId.value) return ''
  const test = savedTests.value.find(t => t.id === currentTestId.value)
  return test?.name || ''
})

const contentTypeHint = computed(() => {
  const map: Record<string, string> = {
    none: '',
    json: 'Content-Type: application/json',
    'form-urlencoded': 'Content-Type: application/x-www-form-urlencoded',
    'form-multipart': 'Content-Type: multipart/form-data',
    raw: `Content-Type: ${requestConfig.value.rawContentType || 'text/plain'}`,
    binary: 'Content-Type: application/octet-stream',
  }
  return map[requestConfig.value.bodyType] || ''
})

function getConfigMethod(configStr: string): string {
  try {
    const cfg = JSON.parse(configStr)
    return cfg.method || 'GET'
  } catch {
    return 'GET'
  }
}

function syncParamsToUrl() {
  const url = requestConfig.value.url
  const urlObj = url.split('?')[0]
  const validParams = requestConfig.value.params.filter(p => p.key.trim())
  if (validParams.length === 0) {
    requestConfig.value.url = urlObj
    return
  }
  const qs = validParams.map(p => `${encodeURIComponent(p.key)}=${encodeURIComponent(p.value)}`).join('&')
  requestConfig.value.url = `${urlObj}?${qs}`
}

const syncUrlToParams = () => {
  try {
    const urlStr = requestConfig.value.url
    if (!urlStr) return
    const url = new URL(urlStr.startsWith('http') ? urlStr : `http://${urlStr}`)
    const params: { key: string; value: string }[] = []
    url.searchParams.forEach((value, key) => {
      params.push({ key, value })
    })
    if (params.length > 0) {
      requestConfig.value.params = params
    }
  } catch { /* invalid URL, ignore */ }
}

function buildHTTPRequestConfig(): HTTPRequestConfig {
  const cfg = requestConfig.value
  const config: HTTPRequestConfig = {
    method: cfg.method,
    url: cfg.url,
  }

  // Headers
  const validHeaders = cfg.headers.filter(h => h.key.trim())
  if (validHeaders.length > 0) config.headers = validHeaders

  // Params
  const validParams = cfg.params.filter(p => p.key.trim())
  if (validParams.length > 0) config.params = validParams

  // Body
  if (cfg.bodyType !== 'none') {
    let content = ''
    if (cfg.bodyType === 'json' || cfg.bodyType === 'raw') {
      content = cfg.bodyContent
    } else if (cfg.bodyType === 'form-urlencoded' || cfg.bodyType === 'form-multipart') {
      content = JSON.stringify(cfg.bodyForm.filter(f => f.key.trim()))
    } else if (cfg.bodyType === 'binary') {
      content = cfg.bodyBinary
    }
    config.body = { type: cfg.bodyType, content }
  }

  // Auth
  if (cfg.authType !== 'none') {
    config.auth = { type: cfg.authType }
    if (cfg.authType === 'bearer') config.auth.token = cfg.auth.token
    else if (cfg.authType === 'basic') {
      config.auth.user = cfg.auth.user
      config.auth.pass = cfg.auth.pass
    } else if (cfg.authType === 'apikey') {
      config.auth.key = cfg.auth.key
      config.auth.value = cfg.auth.value
      config.auth.in = cfg.auth.in
    }
  }

  // Pre-script
  if (cfg.preScript.trim()) config.pre_script = cfg.preScript

  // Post-script
  if (cfg.postScript.trim()) config.post_script = cfg.postScript

  // Timeout
  if (cfg.timeout > 0) config.timeout = cfg.timeout

  return config
}

function loadConfigFromJSON(configStr: string) {
  try {
    const cfg = JSON.parse(configStr) as HTTPRequestConfig
    requestConfig.value.method = cfg.method || 'GET'
    requestConfig.value.url = cfg.url || ''
    requestConfig.value.headers = cfg.headers || []
    requestConfig.value.params = cfg.params || []
    requestConfig.value.bodyType = cfg.body?.type || 'none'
    requestConfig.value.bodyContent = cfg.body?.content || ''
    requestConfig.value.bodyBinary = cfg.body?.type === 'binary' ? (cfg.body?.content || '') : ''
    requestConfig.value.authType = cfg.auth?.type || 'none'
    requestConfig.value.auth = {
      token: cfg.auth?.token || '',
      user: cfg.auth?.user || '',
      pass: cfg.auth?.pass || '',
      key: cfg.auth?.key || '',
      value: cfg.auth?.value || '',
      in: cfg.auth?.in || 'header',
    }
    requestConfig.value.preScript = cfg.pre_script || ''
    requestConfig.value.postScript = cfg.post_script || ''
    requestConfig.value.timeout = cfg.timeout || 30

    // Parse body form data
    if (cfg.body?.type === 'form-urlencoded' || cfg.body?.type === 'form-multipart') {
      try {
        requestConfig.value.bodyForm = JSON.parse(cfg.body.content || '[]')
      } catch {
        requestConfig.value.bodyForm = []
      }
    } else {
      requestConfig.value.bodyForm = []
    }

    // Update body editor
    nextTick(() => {
      updateBodyEditor()
    })
  } catch {
    ElMessage.error('解析配置失败')
  }
}

const parsedResponseHeaders = computed(() => {
  if (!response.value?.headers) return {}
  try {
    return JSON.parse(response.value.headers)
  } catch {
    return {}
  }
})

function formatSize(body: string): string {
  if (!body) return '0 B'
  const bytes = new Blob([body]).size
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
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

function formatAssertionResult(result: AssertionResult): string {
  const a = result.assertion
  const operatorMap: Record<string, string> = {
    equals: '等于',
    not_equals: '不等于',
    contains: '包含',
    gt: '大于',
    lt: '小于',
    exists: '存在',
  }
  const typeMap: Record<string, string> = {
    status_code: '状态码',
    json_path: 'JSON Path',
    header: 'Header',
  }
  let text = `${typeMap[a.type] || a.type}`
  if (a.target) text += ` [${a.target}]`
  text += ` ${operatorMap[a.operator] || a.operator}`
  if (a.expected) text += ` "${a.expected}"`
  text += ` — 实际值: "${result.actual}"`
  if (result.message) text += ` (${result.message})`
  return text
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

function formatJsonBody() {
  try {
    const parsed = JSON.parse(requestConfig.value.bodyContent)
    requestConfig.value.bodyContent = JSON.stringify(parsed, null, 2)
    // Update CodeMirror editor content
    if (bodyEditorView) {
      const state = bodyEditorView.state
      bodyEditorView.dispatch({
        changes: { from: 0, to: state.doc.length, insert: requestConfig.value.bodyContent }
      })
    }
  } catch {
    ElMessage.warning('JSON 格式不正确，无法格式化')
  }
}

// ==================== CodeMirror ====================
function createBodyEditor() {
  if (!bodyEditorRef.value) return
  if (bodyEditorView) bodyEditorView.destroy()

  const isJson = requestConfig.value.bodyType === 'json'
  const state = EditorState.create({
    doc: requestConfig.value.bodyContent,
    extensions: [
      basicSetup,
      isJson ? json() : [],
      oneDark,
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          requestConfig.value.bodyContent = update.state.doc.toString()
        }
      }),
      EditorView.theme({
        '&': { height: '100%', backgroundColor: '#282c34' },
        '.cm-scroller': { overflow: 'auto', backgroundColor: '#282c34' },
      }),
    ],
  })

  bodyEditorView = new EditorView({ state, parent: bodyEditorRef.value })
}

function updateBodyEditor() {
  if (!bodyEditorRef.value) return
  if (requestConfig.value.bodyType !== 'json' && requestConfig.value.bodyType !== 'raw') {
    if (bodyEditorView) {
      bodyEditorView.destroy()
      bodyEditorView = null
    }
    return
  }
  createBodyEditor()
}

function createResponseJsonEditor() {
  if (!responseJsonEditorRef.value) return
  if (responseJsonEditorView) responseJsonEditorView.destroy()

  let formattedBody = ''
  try {
    const parsed = JSON.parse(response.value?.body || '')
    formattedBody = JSON.stringify(parsed, null, 2)
  } catch {
    formattedBody = response.value?.body || ''
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

  responseJsonEditorView = new EditorView({ state, parent: responseJsonEditorRef.value })
}

// Watch body type changes to create/destroy editor
watch(() => requestConfig.value.bodyType, () => {
  nextTick(() => updateBodyEditor())
})

// Watch response to create JSON viewer
watch(response, () => {
  nextTick(() => {
    if (response.value && responseBodyMode.value === 'json') {
      createResponseJsonEditor()
    }
  })
})

watch(responseBodyMode, () => {
  if (responseBodyMode.value === 'json' && response.value) {
    nextTick(() => createResponseJsonEditor())
  }
})

// ==================== API Calls ====================
async function loadSavedTests() {
  try {
    const res = await apiTestAPI.list({ type: 'http' })
    savedTests.value = res.data?.items || []
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error('加载接口列表失败')
    }
  }
}

async function handleLoadTest(item: ApiTest) {
  currentTestId.value = item.id
  loadConfigFromJSON(item.config)
  response.value = null
  responseError.value = ''
  assertionResults.value = []
  // Load history
  loadHistory(item.id)
}

function handleNewRequest() {
  currentTestId.value = null
  requestConfig.value = createEmptyRequest()
  response.value = null
  responseError.value = ''
  assertionResults.value = []
  historyList.value = []
  if (bodyEditorView) {
    bodyEditorView.destroy()
    bodyEditorView = null
  }
}

async function handleDeleteTest(item: ApiTest) {
  try {
    await ElMessageBox.confirm(`确定删除接口 "${item.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await apiTestAPI.delete(item.id)
    ElMessage.success('已删除')
    if (currentTestId.value === item.id) {
      handleNewRequest()
    }
    await loadSavedTests()
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error('删除失败')
    }
  }
}

async function handleSend() {
  if (sending.value && abortController) {
    abortController.abort()
    return
  }

  if (!requestConfig.value.url.trim()) {
    ElMessage.warning('请输入请求 URL')
    return
  }

  sending.value = true
  response.value = null
  responseError.value = ''
  assertionResults.value = []

  abortController = new AbortController()

  try {
    const config = buildHTTPRequestConfig()
    let res: any

    if (currentTestId.value) {
      // 已保存的测试：先更新配置，再调用 executeSaved 以正确关联 test_id
      await apiTestAPI.update(currentTestId.value, { config: JSON.stringify(config) })
      res = await apiTestAPI.executeSaved(currentTestId.value, {
        assertions: requestConfig.value.assertions.length > 0 ? requestConfig.value.assertions : undefined,
      }, { signal: abortController.signal })
    } else {
      // 新请求：调用 execute
      res = await apiTestAPI.execute({
        type: 'http',
        config: JSON.stringify(config),
        save_result: true,
        assertions: requestConfig.value.assertions.length > 0 ? requestConfig.value.assertions : undefined,
      }, { signal: abortController.signal })
    }

    response.value = res.data
    // Parse assertion results
    if (res.data.assertions_result) {
      try {
        assertionResults.value = JSON.parse(res.data.assertions_result)
      } catch {
        assertionResults.value = []
      }
    }
    // Reload history if we have a saved test
    if (currentTestId.value) {
      loadHistory(currentTestId.value)
    }
  } catch (err: unknown) {
    if (err instanceof Error && (err.name === 'CanceledError' || err.name === 'AbortError')) {
      responseError.value = '请求已取消'
    } else if (!isHandledError(err)) {
      const msg = err instanceof Error ? err.message : String(err)
      responseError.value = msg || '请求失败'
    }
  } finally {
    sending.value = false
    abortController = null
  }
}

function handleSave() {
  if (currentTestId.value) {
    // Update existing
    doUpdateTest(currentTestId.value)
  } else {
    // Show save dialog with auto-generated name
    const url = requestConfig.value.url.trim()
    if (url) {
      try {
        const urlObj = new URL(url.startsWith('http') ? url : `http://${url}`)
        saveForm.value.name = `${requestConfig.value.method} ${urlObj.pathname}`
      } catch {
        saveForm.value.name = `${requestConfig.value.method} ${url}`
      }
    } else {
      saveForm.value.name = ''
    }
    saveDialogVisible.value = true
  }
}

async function doUpdateTest(id: number) {
  saving.value = true
  try {
    const config = buildHTTPRequestConfig()
    await apiTestAPI.update(id, {
      config: JSON.stringify(config),
    })
    ElMessage.success('已保存')
    await loadSavedTests()
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error('保存失败')
    }
  } finally {
    saving.value = false
  }
}

async function handleConfirmSave() {
  if (!saveForm.value.name.trim()) {
    ElMessage.warning('请输入接口名称')
    return
  }
  saving.value = true
  try {
    const config = buildHTTPRequestConfig()
    const res = await apiTestAPI.create({
      name: saveForm.value.name.trim(),
      type: 'http',
      config: JSON.stringify(config),
    })
    currentTestId.value = res.data?.id || null
    saveDialogVisible.value = false
    ElMessage.success('已保存')
    await loadSavedTests()
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error('保存失败')
    }
  } finally {
    saving.value = false
  }
}

async function handleGenerateCurl() {
  if (!requestConfig.value.url.trim()) {
    ElMessage.warning('请输入请求 URL')
    return
  }
  try {
    const config = buildHTTPRequestConfig()
    const res = await apiTestAPI.generateCurl(config)
    curlCommand.value = res.data?.curl || ''
    curlDialogVisible.value = true
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error('生成 cURL 失败')
    }
  }
}

function copyCurl() {
  copyToClipboard(curlCommand.value)
}

function copyResponseBody() {
  if (response.value?.body) {
    copyToClipboard(response.value.body)
  }
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch {
    // Fallback for non-HTTPS or older browsers
    const textarea = document.createElement('textarea')
    textarea.value = text
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

function handleBinaryFileChange(file: any) {
  const raw = file.raw || file
  binaryFileName.value = raw.name || ''
  const reader = new FileReader()
  reader.onload = () => {
    const base64 = (reader.result as string).split(',')[1] || ''
    requestConfig.value.bodyBinary = base64
  }
  reader.readAsDataURL(raw)
}

function clearBinaryFile() {
  binaryFileName.value = ''
  requestConfig.value.bodyBinary = ''
}

async function loadHistory(testId: number) {
  try {
    const res = await apiTestAPI.getResults(testId, { page: 1, page_size: 20 })
    historyList.value = res.data?.items || []
  } catch {
    historyList.value = []
  }
  selectedHistoryIds.value = []
}

async function handleClearHistory() {
  if (!currentTestId.value) return
  try {
    await ElMessageBox.confirm('确定清空该接口的所有执行历史吗？', '确认清空', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    // Delete all history results
    for (const item of historyList.value) {
      try {
        await apiTestAPI.deleteResult(item.id)
      } catch {
        // Continue deleting even if one fails
      }
    }
    historyList.value = []
    selectedHistoryIds.value = []
    ElMessage.success('历史已清空')
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error('清空历史失败')
    }
  }
}

function handleSelectHistory(item: ApiTestResult) {
  selectedHistoryId.value = item.id
  response.value = item
  responseError.value = ''
  // Parse assertion results
  if (item.assertions_result) {
    try {
      assertionResults.value = JSON.parse(item.assertions_result)
    } catch {
      assertionResults.value = []
    }
  } else {
    assertionResults.value = []
  }
}

function toggleHistorySelect(id: number, checked: boolean) {
  if (checked) {
    if (selectedHistoryIds.value.length < 2) {
      selectedHistoryIds.value.push(id)
    }
  } else {
    selectedHistoryIds.value = selectedHistoryIds.value.filter(i => i !== id)
  }
}

function handleCompare() {
  if (selectedHistoryIds.value.length !== 2) return
  const [leftId, rightId] = selectedHistoryIds.value
  compareLeft.value = historyList.value.find(h => h.id === leftId) || null
  compareRight.value = historyList.value.find(h => h.id === rightId) || null
  if (compareLeft.value && compareRight.value) {
    compareDialogVisible.value = true
  }
}

function formatCompareBody(body: string): string {
  if (!body) return ''
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

// ==================== Lifecycle ====================
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

onMounted(() => {
  loadSavedTests()
  document.addEventListener('keydown', handleGlobalKeydown)
})

onUnmounted(() => {
  bodyEditorView?.destroy()
  responseJsonEditorView?.destroy()
  document.removeEventListener('keydown', handleGlobalKeydown)
})
</script>

<style scoped>
/* ==================== CSS Variables ==================== */
.http-test-page {
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
  background: rgba(59, 130, 246, 0.1);
}

.saved-item:hover .saved-delete {
  opacity: 1;
}

.drag-handle {
  color: var(--text-muted);
  cursor: grab;
  flex-shrink: 0;
  font-size: 12px;
  opacity: 0.4;
  transition: opacity var(--duration-fast);
}

.saved-item:hover .drag-handle {
  opacity: 0.7;
}

.method-badge {
  flex-shrink: 0;
  font-size: 11px;
  min-width: 42px;
  text-align: center;
}

.saved-name {
  flex: 1;
  font-size: 13px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  background: var(--accent-primary);
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
  background: rgba(59, 130, 246, 0.06);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 12px;
  color: var(--accent-primary);
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

.method-select {
  width: 110px;
  flex-shrink: 0;
}

.method-text {
  font-weight: 600;
  font-size: 13px;
}

.method-get { color: #67c23a; }
.method-post { color: #e6a23c; }
.method-put { color: #409eff; }
.method-delete { color: #f56c6c; }
.method-patch { color: #ff9800; }
.method-head { color: #909399; }
.method-options { color: #909399; }

.url-input {
  flex: 1;
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
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-secondary) 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
}

.send-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
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
  border-color: var(--accent-primary);
  color: var(--accent-primary);
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

.tab-badge :deep(.el-badge__content) {
  font-size: 10px;
  height: 16px;
  line-height: 16px;
  padding: 0 4px;
}

/* ==================== Body Section ==================== */
.body-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.body-type-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  flex-wrap: wrap;
}

.body-type-group {
  flex-wrap: wrap;
}

.content-type-hint {
  font-size: 12px;
  color: var(--text-muted);
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  background: var(--bg-secondary);
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.body-editor-wrapper {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.body-editor-toolbar {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xs) var(--spacing-sm);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
  flex-shrink: 0;
}

.raw-content-type-input {
  width: 240px;
}

.body-editor {
  flex: 1;
  min-height: 180px;
  overflow: auto;
}

.body-none {
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--text-muted);
  font-size: var(--font-size-sm);
}

.binary-upload {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
}

.binary-file-name {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.binary-clear {
  color: var(--text-muted);
  cursor: pointer;
  flex-shrink: 0;
  transition: color var(--duration-fast);
}

.binary-clear:hover {
  color: var(--accent-danger);
}

/* ==================== Auth Section ==================== */
.auth-section {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.auth-type-select {
  width: 180px;
}

.auth-fields {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  max-width: 400px;
}

.auth-in-select {
  width: 140px;
}

/* ==================== Assertions Editor ==================== */
.assertions-editor {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.assertion-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.assertion-field {
  flex: 1;
}

.assertion-type {
  width: 120px;
  flex: none;
}

.assertion-operator {
  width: 110px;
  flex: none;
}

.assertion-target {
  flex: 2;
}

.assertion-expected {
  flex: 2;
}

/* ==================== Script Textarea ==================== */
.script-textarea :deep(.el-textarea__inner) {
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 13px;
}

/* ==================== Response Section ==================== */
.response-section {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-top: 2px solid var(--accent-primary);
  border-radius: var(--section-radius);
  box-shadow: var(--panel-shadow);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  flex: 1;
  min-height: 250px;
  background: linear-gradient(180deg, rgba(59, 130, 246, 0.02) 0%, var(--bg-card) 100%);
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

.error-text {
  color: var(--accent-danger);
  font-size: var(--font-size-sm);
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

.response-body-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--spacing-sm);
  flex-shrink: 0;
}

.response-body-modes {
  display: flex;
  align-items: center;
}

.response-body-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.response-json-editor {
  flex: 1;
  min-height: 0;
}

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
  color: var(--accent-primary);
  border-color: var(--accent-primary);
}

.response-raw {
  background: var(--bg-secondary);
  padding: var(--spacing-md);
  border-radius: var(--radius-md);
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
  flex: 1;
  min-height: 0;
  overflow: auto;
  margin: 0;
}

.response-preview {
  width: 100%;
  height: 100%;
  min-height: 200px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: white;
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

/* ==================== Assertions Results ==================== */
.assertions-results {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.assertion-result {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-md);
  font-size: 13px;
}

.assertion-result.pass {
  background: rgba(103, 194, 58, 0.08);
  color: var(--accent-success);
}

.assertion-result.fail {
  background: rgba(245, 108, 108, 0.08);
  color: var(--accent-danger);
}

.assertion-result-text {
  flex: 1;
  line-height: 1.5;
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
  background: rgba(59, 130, 246, 0.1);
}

.history-status {
  min-width: 42px;
  text-align: center;
}

.status-code-2xx { color: #67c23a; font-weight: 600; }
.status-code-3xx { color: #909399; }
.status-code-4xx { color: #e6a23c; font-weight: 600; }
.status-code-5xx { color: #f56c6c; font-weight: 600; }

.history-latency {
  color: var(--text-secondary);
  min-width: 60px;
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

/* ==================== cURL Dialog ==================== */
.curl-content {
  background: var(--bg-secondary);
  border-radius: var(--radius-md);
  padding: var(--spacing-lg);
  max-height: 400px;
  overflow: auto;
}

.curl-text {
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
  color: var(--text-primary);
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
