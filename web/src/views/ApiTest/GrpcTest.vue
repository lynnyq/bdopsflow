<!--
  gRPC 测试页面
  功能：支持 gRPC 接口调试，包括 proto 文件上传、服务反射、方法选择、请求构造、
       发送请求、查看响应、历史记录对比等。支持 TLS 安全连接和请求元数据设置。
 左侧保存列表 + 右侧请求/响应双栏布局。
-->
<template>
  <!-- === .grpc-test-page 区块 === -->
  <div class="grpc-test-page">
    <!-- === .main-content 区块 === -->
    <div class="main-content">
      <!-- === 左侧保存列表：搜索 + 已保存用例 === -->
      <aside class="saved-panel">
        <!-- === .panel-header 区块 === -->
        <div class="panel-header">
          <span class="panel-title">gRPC 测试</span>
          <!-- === 徽章组件 === -->
          <el-badge :value="savedTests.length" :max="99" type="info" class="panel-count-badge" />
        </div>
        <!-- === .panel-search 区块 === -->
        <div class="panel-search">
          <!-- === 输入框组件 === -->
          <el-input
            v-model="searchQuery"
            placeholder="搜索..."
            :prefix-icon="Search"
            size="small"
            clearable
          />
        </div>
        <!-- === .saved-list 区块 === -->
        <div class="saved-list">
          <div
            v-for="item in filteredTests"
            :key="item.id"
            class="saved-item"
            :class="{ active: currentTestId === item.id }"
            @click="handleLoadTest(item)"
          >
            <!-- === 标签组件 === -->
            <el-tag size="small" effect="dark" class="grpc-badge" disable-transitions>gRPC</el-tag>
            <!-- === .saved-item-info 区块 === -->
            <div class="saved-item-info">
              <span class="saved-name" :title="item.name">{{ item.name }}</span>
              <span class="saved-addr">{{ getAddrFromConfig(item.config) }}</span>
            </div>
            <!-- === 图标组件 === -->
            <el-icon class="saved-delete" @click.stop="handleDeleteTest(item)">
              <Delete />
            </el-icon>
          </div>
          <!-- === 条件渲染区块 === -->
          <div v-if="filteredTests.length === 0" class="empty-saved">
            <!-- === 图标组件 === -->
            <el-icon :size="24"><Document /></el-icon>
            <p>{{ searchQuery ? '无匹配结果' : '暂无测试用例' }}</p>
          </div>
        </div>
        <!-- === .panel-footer 区块 === -->
        <div class="panel-footer">
          <!-- === 按钮组件 === -->
          <el-button type="primary" :icon="Plus" size="small" @click="handleNewTest" class="new-btn">
            新建测试
          </el-button>
        </div>
      </aside>

      <!-- === 右侧请求/响应编辑器区域 === -->
      <main class="editor-area">
        <!-- === 请求配置区域：地址 + Proto/Reflection 切换 + 服务方法选择 + 请求体 === -->
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
            <!-- === 图标组件 === -->
            <el-icon :size="14"><EditPen /></el-icon>
            <span>{{ currentTestName }}</span>
          </div>

          <!-- Top Bar -->
          <div class="request-bar">
            <!-- === 输入框组件 === -->
            <el-input
              v-model="requestConfig.address"
              placeholder="服务器地址 (如: localhost:50051)"
              class="address-input"
              clearable
            >
              <!-- === 具名插槽 === -->
              <template #prefix>
                <!-- === 图标组件 === -->
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
                <!-- === .proto-selector 区块 === -->
                <div class="proto-selector">
                  <!-- === <el-select> 组件 === -->
                  <el-select
                    v-model="selectedProtoFileId"
                    placeholder="选择 Proto 文件"
                    filterable
                    clearable
                    class="proto-select"
                    @change="handleProtoFileChange"
                  >
                    <!-- === 下拉选项 === -->
                    <el-option
                      v-for="pf in protoFiles"
                      :key="pf.id"
                      :label="pf.name"
                      :value="pf.id"
                    />
                  </el-select>
                </div>
                <!-- === 条件渲染区块 === -->
                <div v-if="parsingProto" class="service-loading">
                  <!-- === 图标组件 === -->
                  <el-icon class="is-loading"><Refresh /></el-icon>
                  <span>解析中...</span>
                </div>
                <!-- === else 分支区块 === -->
                <div v-else-if="protoServices.length > 0" class="service-tree">
                  <!-- === <el-collapse> 组件 === -->
                  <el-collapse v-model="expandedServiceNames" class="service-collapse">
                    <!-- === <el-collapse> 组件 === -->
                    <el-collapse-item
                      v-for="svc in protoServices"
                      :key="svc.name"
                      :name="svc.name"
                    >
                      <!-- === 具名插槽 === -->
                      <template #title>
                        <!-- === .service-name 区块 === -->
                        <div class="service-name">
                          <!-- === 图标组件 === -->
                          <el-icon :size="16" class="svc-icon"><Folder /></el-icon>
                          <span>{{ svc.name }}</span>
                          <!-- === 标签组件 === -->
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
                <!-- === .reflection-bar 区块 === -->
                <div class="reflection-bar">
                  <!-- === 按钮组件 === -->
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
                <!-- === 条件渲染区块 === -->
                <div v-if="reflecting" class="service-loading">
                  <!-- === 图标组件 === -->
                  <el-icon class="is-loading"><Refresh /></el-icon>
                  <span>发现中...</span>
                </div>
                <!-- === else 分支区块 === -->
                <div v-else-if="protoServices.length > 0" class="service-tree">
                  <!-- === <el-collapse> 组件 === -->
                  <el-collapse v-model="expandedServiceNames" class="service-collapse">
                    <!-- === <el-collapse> 组件 === -->
                    <el-collapse-item
                      v-for="svc in protoServices"
                      :key="svc.name"
                      :name="svc.name"
                    >
                      <!-- === 具名插槽 === -->
                      <template #title>
                        <!-- === .service-name 区块 === -->
                        <div class="service-name">
                          <!-- === 图标组件 === -->
                          <el-icon :size="16" class="svc-icon"><Folder /></el-icon>
                          <span>{{ svc.name }}</span>
                          <!-- === 标签组件 === -->
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
        v-if="(lastResult || sending) && panelMode === 'both'"
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
              <!-- === 条件/循环模板 === -->
              <template v-if="lastResult">
                <!-- === 标签组件 === -->
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
                  <!-- === 图标组件 === -->
                  <el-icon :size="14"><Clock /></el-icon>
                  {{ lastResult.latency_ms }}ms
                </span>
              </template>
              <!-- === 条件/循环模板 === -->
              <template v-if="sending && !lastResult">
                <!-- === 图标组件 === -->
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
                <!-- === 条件/循环模板 === -->
                <template v-if="lastResult">
                  <!-- === 条件渲染区块 === -->
                  <div v-if="lastResult.error" class="error-content">
                    <!-- === 图标组件 === -->
                    <el-icon :size="16" color="var(--accent-danger)"><CircleCloseFilled /></el-icon>
                    <pre>{{ lastResult.error }}</pre>
                  </div>
                  <!-- === 条件/循环模板 === -->
                  <template v-else>
                    <!-- === .response-body-header 区块 === -->
                    <div class="response-body-header">
                      <!-- === .response-body-modes 区块 === -->
                      <div class="response-body-modes">
                        <!-- === 单选框组件 === -->
                        <el-radio-group v-model="responseBodyMode" size="small">
                          <!-- === 单选框组件 === -->
                          <el-radio-button value="json">JSON</el-radio-button>
                          <!-- === 单选框组件 === -->
                          <el-radio-button value="raw">Raw</el-radio-button>
                        </el-radio-group>
                      </div>
                      <!-- === .response-body-actions 区块 === -->
                      <div class="response-body-actions">
                        <!-- === 按钮组件 === -->
                        <el-button
                          v-if="lastResult?.body"
                          size="small"
                          text
                          class="copy-response-btn"
                          @click="copyResponseBody"
                        >
                          <!-- === 图标组件 === -->
                          <el-icon><DocumentCopy /></el-icon>
                          复制
                        </el-button>
                      </div>
                    </div>
                    <!-- === 条件渲染区块 === -->
                    <div v-if="responseBodyMode === 'json'" ref="responseEditorRef" class="response-json-editor"></div>
                    <!-- === else 分支区块 === -->
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
                <!-- === 条件/循环模板 === -->
                <template v-if="lastResult && lastResult.headers">
                  <!-- === v-for 渲染区块 === -->
                  <div v-for="(value, key) in parseResponseHeadersMap(lastResult.headers)" :key="key" class="header-row">
                    <span class="header-key">{{ key }}</span>
                    <span class="header-value">{{ value }}</span>
                  </div>
                  <!-- === 条件渲染区块 === -->
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
      <div v-if="!lastResult && !sending && panelMode !== 'request-max'" class="empty-response">
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
      <!-- === 具名插槽 === -->
      <template #footer>
        <!-- === 按钮组件 === -->
        <el-button @click="saveDialogVisible = false">取消</el-button>
        <!-- === 按钮组件 === -->
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
      <!-- === 具名插槽 === -->
      <template #footer>
        <!-- === 按钮组件 === -->
        <el-button @click="compareDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

/**
 * gRPC 协议测试组件
 *
 * 功能特性：
 * - 支持 Proto 文件解析与服务反射（reflection）两种连接模式
 * - 可视化服务/方法选择，自动从 proto 定义生成请求模板
 * - 表单模式与 JSON 模式双轨编辑请求体
 * - TLS / mTLS 安全配置，关联证书管理
 * - 历史记录、对比查看、保存用例、快速重发
 * - CodeMirror 6 驱动的 JSON 请求/响应编辑器
 */
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

// ============================================================================
// State
// ============================================================================

/** 搜索过滤关键词（保存的测试用例列表） */
const searchQuery = ref('')
/** 用户保存的所有 gRPC 测试用例 */
const savedTests = ref<ApiTest[]>([])
/** 当前正在编辑/执行的测试用例 ID；null 表示新请求尚未保存 */
const currentTestId = ref<number | null>(null)
/** 加载列表通用 loading 标志 */
const loading = ref(false)
/** 发送请求中 */
const sending = ref(false)
/** 保存用例中 */
const saving = ref(false)
/** 连接性测试中 */
const connecting = ref(false)

/** 连接模式：proto 表示依赖 Proto 文件，reflection 表示走服务反射 */
const connectionMode = ref<'proto' | 'reflection'>('proto')
/** 请求侧 Tab：service(服务列表) / request(请求体编辑) */
const activeRequestTab = ref('service')
/** 响应侧 Tab：body(响应体) / headers(响应头) / meta(元信息) */
const activeResponseTab = ref('body')
/** 响应体展示模式：json(格式化) / raw(原始文本) */
const responseBodyMode = ref('json')

/** 面板布局模式：双栏 / 请求区最大化 / 响应区最大化 */
const panelMode = ref<'both' | 'request-max' | 'response-max'>('both')
/** 请求区块 DOM 引用，用于手动拖拽调整高度 */
const requestSectionRef = ref<HTMLElement | null>(null)
/** 拖拽分隔条中 */
const isDragging = ref(false)

/** 当前请求的完整 gRPC 配置（源数据） */
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

/** 独立维护的 metadata 键值列表（KVEditor 直接绑定） */
const metadata = ref<{ key: string; value: string }[]>([])
/** 当前用户可选择的 Proto 文件 */
const protoFiles = ref<ProtoFile[]>([])
/** 当前用户可选择的 TLS 证书 */
const certificates = ref<CertificateSummary[]>([])
/** 当前选中的 Proto 文件 ID */
const selectedProtoFileId = ref<number | null>(null)
/** 当前 Proto/反射得到的服务列表 */
const protoServices = ref<ProtoService[]>([])
/** 服务面板中默认展开的服务名集合 */
const expandedServiceNames = ref<string[]>([])
/** 后台正在解析 Proto 文件中 */
const parsingProto = ref(false)
/** 后台正在执行服务反射中 */
const reflecting = ref(false)

/** 所有 Message 类型定义（来自 Proto fields API） */
const messageDefs = ref<ProtoMessageDef[]>([])
/** 当前选中方法的输入 Message 定义，用于表单模式渲染 */
const currentMessageDef = ref<ProtoMessageDef | null>(null)
/** 表单模式下由 ProtoForm 组件回填的数据 */
const formData = ref<Record<string, unknown>>({})
/** 请求体编辑模式：form(可视化表单) / json(纯 JSON) */
const requestMode = ref<'form' | 'json'>('form')

/** 最近一次执行结果 */
const lastResult = ref<ApiTestResult | null>(null)
/** 当前测试用例的历史执行记录 */
const historyResults = ref<ApiTestResult[]>([])

/** 保存对话框显示状态 */
const saveDialogVisible = ref(false)
/** 保存对话框中的用例名称输入 */
const saveName = ref('')

/** 历史对比：已勾选的两条记录 ID */
const selectedHistoryIds = ref<number[]>([])
/** 历史对比弹窗显示状态 */
const compareDialogVisible = ref(false)
/** 历史对比：左面板结果 */
const compareLeft = ref<ApiTestResult | null>(null)
/** 历史对比：右面板结果 */
const compareRight = ref<ApiTestResult | null>(null)

/** CodeMirror 请求编辑器 DOM 宿主 */
const requestEditorRef = ref<HTMLElement | null>(null)
/** CodeMirror 响应编辑器 DOM 宿主 */
const responseEditorRef = ref<HTMLElement | null>(null)
/** CodeMirror 请求编辑器实例（模块级） */
let requestEditorView: EditorView | null = null
/** CodeMirror 响应编辑器实例（模块级，只读） */
let responseEditorView: EditorView | null = null

// ============================================================================
// Computed
// ============================================================================

/** 经 searchQuery 过滤后的保存测试列表 */
const filteredTests = computed(() => {
  /** 条件分支 */
  if (!searchQuery.value) return savedTests.value
  const q = searchQuery.value.toLowerCase()
  /** 返回/抛出 */
  return savedTests.value.filter(t => t.name.toLowerCase().includes(q))
})

/** 满足请求发送的基本条件：地址 + 服务 + 方法都已填写，且当前不在发送中 */
const canSend = computed(() => {
  /** 返回/抛出 */
  return requestConfig.value.address && requestConfig.value.service && requestConfig.value.method && !sending.value
})

/** 保存用例的基本条件：至少有地址，且当前不在保存中 */
const canSave = computed(() => {
  /** 返回/抛出 */
  return requestConfig.value.address && !saving.value
})

/** 当前已加载用例的名称，用于头部标题显示 */
const currentTestName = computed(() => {
  /** 条件分支 */
  if (!currentTestId.value) return ''
  const found = savedTests.value.find(t => t.id === currentTestId.value)
  /** 返回/抛出 */
  return found?.name || ''
})

/** 最近一次执行结果中剔除 error 后的响应体字符串 */
const responseBodyText = computed(() => {
  /** 条件分支 */
  if (!lastResult.value || lastResult.value.error) return ''
  /** 返回/抛出 */
  return lastResult.value.body || ''
})

/** TLS 模式对应的图标组件 */
const tlsSecurityIcon = computed(() => {
  /** 分支选择 */
  switch (requestConfig.value.tls_mode) {
    /** 分支选择 */
    case 'insecure': return Unlock
    /** 分支选择 */
    case 'tls': return Lock
    /** 分支选择 */
    case 'mtls': return CircleCheckFilled
    /** 分支选择 */
    default: return Unlock
  }
})

/** TLS 模式对应的 CSS 类（颜色） */
const tlsSecurityClass = computed(() => {
  /** 分支选择 */
  switch (requestConfig.value.tls_mode) {
    /** 分支选择 */
    case 'insecure': return 'security-danger'
    /** 分支选择 */
    case 'tls': return 'security-warning'
    /** 分支选择 */
    case 'mtls': return 'security-success'
    /** 分支选择 */
    default: return 'security-danger'
  }
})

/** TLS 模式对应的中文描述 */
const tlsSecurityDesc = computed(() => {
  /** 分支选择 */
  switch (requestConfig.value.tls_mode) {
    /** 分支选择 */
    case 'insecure': return '不安全 - 未加密'
    /** 分支选择 */
    case 'tls': return '加密 - 服务端验证'
    /** 分支选择 */
    case 'mtls': return '最安全 - 双向认证'
    /** 分支选择 */
    default: return '不安全 - 未加密'
  }
})

// ============================================================================
// Helpers
// ============================================================================

/**
 * 从持久化的 config JSON 字符串中提取 gRPC 地址
 * @param configStr 序列化后的 GRPCRequestConfig
 * @returns 解析得到的 address；解析失败返回空字符串
 */
const getAddrFromConfig = (configStr: string): string => {
  /** 异常处理 */
  try {
    const c = JSON.parse(configStr) as GRPCRequestConfig
    /** 返回/抛出 */
    return c.address || ''
  } catch {
    /** 返回/抛出 */
    return ''
  }
}

/**
 * 把后端返回的 headers 字符串（可能是 object 或 [{key,value}] 数组）
 * 统一转换为 Record<string, string> 形式
 * @param {string} headersStr - 后端返回的序列化 headers 字符串
 * @returns {Record<string, string>} 解析后的 header 键值对对象
 */
const parseResponseHeadersMap = (headersStr: string): Record<string, string> => {
  /** 条件分支 */
  if (!headersStr) return {}
  /** 异常处理 */
  try {
    const parsed = JSON.parse(headersStr)
    /** 条件分支 */
    if (typeof parsed === 'object' && !Array.isArray(parsed)) return parsed as Record<string, string>
    /** 条件分支 */
    if (Array.isArray(parsed)) {
      const map: Record<string, string> = {}
      /** 循环 */
      for (const item of parsed) {
        /** 条件分支 */
        if (item.key && item.value !== undefined) {
          map[item.key] = String(item.value)
        }
      }
      /** 返回/抛出 */
      return map
    }
    /** 返回/抛出 */
    return {}
  } catch {
    /** 返回/抛出 */
    return {}
  }
}

/**
 * 把 ISO 日期字符串格式化为相对时间（刚刚/X分钟前/X小时前/...）
 * 超过 30 天则回退到 formatDateTime
 */
function formatRelativeTime(dateStr: string): string {
  /** 条件分支 */
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  /** 条件分支 */
  if (isNaN(date.getTime())) return dateStr
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  /** 条件分支 */
  if (diffSec < 60) return '刚刚'
  const diffMin = Math.floor(diffSec / 60)
  /** 条件分支 */
  if (diffMin < 60) return `${diffMin}分钟前`
  const diffHour = Math.floor(diffMin / 60)
  /** 条件分支 */
  if (diffHour < 24) return `${diffHour}小时前`
  const diffDay = Math.floor(diffHour / 24)
  /** 条件分支 */
  if (diffDay < 30) return `${diffDay}天前`
  /** 返回/抛出 */
  return formatDateTime(dateStr)
}

/**
 * 交互确认后，逐条删除当前测试用例的所有历史执行结果
 * 循环调用 deleteResult，单条失败不影响后续删除；完成后清空本地状态
 * @returns {Promise<void>}
 */
const handleClearHistory = async () => {
  /** 条件分支 */
  if (!currentTestId.value) return
  /** 异常处理 */
  try {
    await ElMessageBox.confirm('确定清空该测试的所有执行历史吗？', '确认清空', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    /** 循环 */
    for (const item of historyResults.value) {
      /** 异常处理 */
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
    /** 条件分支 */
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error('清空历史失败')
    }
  }
}

/**
 * 历史记录勾选状态切换，最多允许选中 2 条用于对比功能
 * 选中时若已有 2 条则拒绝新增；取消时从数组中过滤掉目标 ID
 * @param {number} id - 历史记录 ID
 * @param {boolean} checked - true 表示选中，false 表示取消
 * @returns {void}
 */
const toggleHistorySelect = (id: number, checked: boolean) => {
  /** 条件分支 */
  if (checked) {
    /** 条件分支 */
    if (selectedHistoryIds.value.length < 2) {
      selectedHistoryIds.value.push(id)
    }
  } else {
    selectedHistoryIds.value = selectedHistoryIds.value.filter(i => i !== id)
  }
}

/**
 * 根据 selectedHistoryIds 构造 compareLeft / compareRight 并打开对比弹窗
 * 要求必须恰好选中 2 条历史记录，否则静默跳过
 * @returns {void}
 */
const handleCompare = () => {
  /** 条件分支 */
  if (selectedHistoryIds.value.length !== 2) return
  const [leftId, rightId] = selectedHistoryIds.value
  compareLeft.value = historyResults.value.find(h => h.id === leftId) || null
  compareRight.value = historyResults.value.find(h => h.id === rightId) || null
  /** 条件分支 */
  if (compareLeft.value && compareRight.value) {
    compareDialogVisible.value = true
  }
}

/**
 * 用于对比弹窗：把响应体中的 JSON 字符串格式化缩进（2 空格）
 * 非 JSON 文本原样返回；空输入返回空字符串
 * @param {string | undefined} body - 原始响应体文本
 * @returns {string} 格式化后的 JSON 或原样返回的文本
 */
const formatCompareBody = (body: string | undefined): string => {
  /** 条件分支 */
  if (!body) return ''
  /** 异常处理 */
  try {
    /** 返回/抛出 */
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    /** 返回/抛出 */
    return body
  }
}

// --- Panel resize handlers ---

/**
 * 切换面板布局模式：both（上下平分）、request-max（请求区占主要空间）、response-max（响应区占主要空间）
 * 点击当前激活的模式按钮会自动回到 'both' 并清除可能残留的 inline 样式
 * @param {'both' | 'request-max' | 'response-max'} mode - 目标布局模式
 * @returns {void}
 */
const togglePanelMode = (mode: 'both' | 'request-max' | 'response-max') => {
  panelMode.value = panelMode.value === mode ? 'both' : mode
  // Reset inline styles set by drag handler so CSS classes take effect
  if (requestSectionRef.value) {
    requestSectionRef.value.style.flex = ''
    requestSectionRef.value.style.height = ''
  }
}

/**
 * 面板分隔条拖拽开始：监听全局 mousemove/mouseup，实时调整 requestSection 高度
 * 高度限制 120px ~ editorArea 高度 - 120px；拖拽时自动退出 panelMode
 * @param {MouseEvent} e - mousedown 事件
 * @returns {void}
 */
const handleResizeDragStart = (e: MouseEvent) => {
  e.preventDefault()
  isDragging.value = true
  const editorArea = (e.target as HTMLElement).closest('.editor-area') as HTMLElement
  /** 条件分支 */
  if (!editorArea) return
  const areaRect = editorArea.getBoundingClientRect()
  const areaHeight = areaRect.height
  const startY = e.clientY
  const startHeight = requestSectionRef.value?.offsetHeight || 0

  /** on mouse move */
  const onMouseMove = (moveEvent: MouseEvent) => {
    const delta = moveEvent.clientY - startY
    const newHeight = startHeight + delta
    const minHeight = 120
    const maxHeight = areaHeight - 120
    const clampedHeight = Math.min(Math.max(newHeight, minHeight), maxHeight)
    /** 条件分支 */
    if (requestSectionRef.value) {
      requestSectionRef.value.style.flex = 'none'
      requestSectionRef.value.style.height = `${clampedHeight}px`
    }
    panelMode.value = 'both'
  }

  /** on mouse up */
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

/**
 * 把最近一次响应体复制到剪贴板，优先使用 Clipboard API
 * 浏览器不支持时降级为 document.execCommand('copy') 创建临时 textarea 方案
 * @returns {Promise<void>}
 */
const copyResponseBody = async () => {
  /** 条件分支 */
  if (!responseBodyText.value) return
  /** 异常处理 */
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
    /** 异常处理 */
    try {
      document.execCommand('copy')
      ElMessage.success('已复制到剪贴板')
    } catch {
      ElMessage.error('复制失败，请手动复制')
    }
    document.body.removeChild(textarea)
  }
}

// ============================================================================
// Data Loading
// ============================================================================

/**
 * 拉取当前用户保存的所有 gRPC 测试用例（最多 200 条）
 * 用于左侧 saved-panel 的用例列表展示，component mount 时调用
 * @returns {Promise<void>}
 */
const loadSavedTests = async () => {
  loading.value = true
  /** 异常处理 */
  try {
    const res = await apiTestAPI.list({ type: 'grpc', page: 1, page_size: 200 })
    savedTests.value = res.data.items || []
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载测试列表失败')
    }
  } finally {
    loading.value = false
  }
}

/**
 * 拉取用户可关联的 Proto 文件列表（最多 200 条）
 * 用于连接模式下拉选择；选中后通过 handleProtoFileChange 解析服务树
 * @returns {Promise<void>}
 */
const loadProtoFiles = async () => {
  /** 异常处理 */
  try {
    const res = await protoFileAPI.list({ page: 1, page_size: 200 })
    protoFiles.value = res.data.items || []
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载 Proto 文件列表失败')
    }
  }
}

/**
 * 拉取用户可关联的 TLS 证书列表（用于 gRPC over TLS 连接）
 * 用于请求配置中的 certificate_id 下拉选择
 * @returns {Promise<void>}
 */
const loadCertificates = async () => {
  /** 异常处理 */
  try {
    const res = await certificateAPI.list({ page: 1, page_size: 200 })
    certificates.value = res.data.items || []
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载证书列表失败')
    }
  }
}

/**
 * 加载指定测试用例的历史执行记录（最多 20 条）
 * 用于右侧 history-panel 的历史列表展示，支持选中对比
 * @param {number} testId - 测试用例 ID
 * @returns {Promise<void>}
 */
const loadHistory = async (testId: number) => {
  /** 异常处理 */
  try {
    const res = await apiTestAPI.getResults(testId, { page: 1, page_size: 20 })
    historyResults.value = res.data.items || []
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '加载历史记录失败')
    }
  }
}

// ============================================================================
// Proto Parsing / Reflection / Message Templates
// ============================================================================

/**
 * Proto 文件切换时：并行调用 parse（拿服务列表）和 fields（拿 Message 定义）
 * @param fileId 选中的 Proto 文件 ID；null 时仅做清理
 */
const handleProtoFileChange = async (fileId: number | null) => {
  requestConfig.value.proto_file_id = fileId
  protoServices.value = []
  expandedServiceNames.value = []

  /** 条件分支 */
  if (!fileId) return

  parsingProto.value = true
  messageDefs.value = []
  currentMessageDef.value = null
  formData.value = {}
  /** 异常处理 */
  try {
    const pf = protoFiles.value.find(f => f.id === fileId)
    /** 条件分支 */
    if (!pf) return

    // Parse services and load field definitions in parallel
    const [parseRes, fieldsRes] = await Promise.all([
      protoFileAPI.parse({ content: pf.content }),
      protoFileAPI.fields({ proto_file_id: fileId }).catch(() => null),
    ])

    const result = parseRes.data as ProtoParseResult
    /** 条件分支 */
    if (result.services) {
      protoServices.value = result.services
      expandedServiceNames.value = result.services.map(s => s.name)
    }

    /** 条件分支 */
    if (fieldsRes) {
      messageDefs.value = fieldsRes.data.messages || []
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '解析 Proto 文件失败')
    }
  } finally {
    parsingProto.value = false
  }
}

/**
 * 通过服务反射（reflection）远程拉取服务列表，依赖已配置的 TLS 地址
 * 适合没有 .proto 文件的场景，通过后端调用 gRPC ServerReflection API
 * @returns {Promise<void>}
 */
const handleReflect = async () => {
  /** 条件分支 */
  if (!requestConfig.value.address) {
    ElMessage.warning('请输入服务器地址')
    return
  }

  reflecting.value = true
  protoServices.value = []
  expandedServiceNames.value = []

  /** 异常处理 */
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
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '服务反射失败')
    }
  } finally {
    reflecting.value = false
  }
}

/**
 * 服务树中点击某个方法后：回填 service/method，查找 Message 定义，
 * 并自动为 JSON 编辑器和 formData 填入初始模板
 */
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
    /** 条件分支 */
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

/**
 * 根据 Proto Message 定义递归生成格式化的 JSON 模板字符串（2 空格缩进）
 * 内部调用 buildTemplateObject 把字段树转为默认值对象
 * @param {ProtoMessageDef} msgDef - Proto 消息定义对象
 * @returns {string} 格式化后的 JSON 字符串，如 '{\n  "fieldName": ""\n}'
 */
const generateTemplateFromMessage = (msgDef: ProtoMessageDef): string => {
  const obj = buildTemplateObject(msgDef)
  /** 返回/抛出 */
  return JSON.stringify(obj, null, 2)
}

/**
 * 递归把 ProtoMessageDef 转成 JavaScript 对象（用于模板生成和表单初始化）
 * 遍历 fields，对每个字段调用 buildFieldDefaultValue 填默认值
 * @param {ProtoMessageDef} msgDef - Proto 消息定义对象
 * @returns {Record<string, unknown>} 字段名到默认值的映射对象
 */
const buildTemplateObject = (msgDef: ProtoMessageDef): Record<string, unknown> => {
  const obj: Record<string, unknown> = {}
  /** 循环 */
  for (const field of msgDef.fields) {
    obj[field.name] = buildFieldDefaultValue(field)
  }
  /** 返回/抛出 */
  return obj
}

/**
 * 根据字段 label 和 type 返回合理的默认值：
 * map → {}，repeated → []，嵌套 message → 递归对象，bool → false，数值 → 0，字符串 → ""
 * @param {ProtoMessageField} field - Proto 字段定义
 * @returns {unknown} 适合 JSON 模板 / 表单初始化的默认值
 */
const buildFieldDefaultValue = (field: ProtoMessageField): unknown => {
  // Map
  if (field.label === 'map') return {}
  // Repeated
  if (field.label === 'repeated') return []
  // Nested message
  if (field.fields && field.fields.length > 0) {
    const obj: Record<string, unknown> = {}
    /** 循环 */
    for (const f of field.fields) {
      obj[f.name] = buildFieldDefaultValue(f)
    }
    /** 返回/抛出 */
    return obj
  }
  // Scalar
  if (field.type === 'bool') return false
  /** 条件分支 */
  if (['int32', 'int64', 'uint32', 'uint64', 'sint32', 'sint64',
    'fixed32', 'fixed64', 'sfixed32', 'sfixed64', 'float', 'double'].includes(field.type)) return 0
  /** 返回/抛出 */
  return ''
}

// ============================================================================
// Connect Test
// ============================================================================

/**
 * 仅校验能否成功建立 gRPC 连接（不调用具体方法）
 * 后端侧通过 grpc_connect_test type 处理，不会落库保存结果
 */
const handleConnectTest = async () => {
  /** 条件分支 */
  if (!requestConfig.value.address) {
    ElMessage.warning('请输入服务器地址')
    return
  }

  connecting.value = true
  /** 异常处理 */
  try {
    /** config 变量声明 */
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
    /** 条件分支 */
    if (result.error) {
      ElMessage.error(result.error)
    } else {
      ElMessage.success('连接成功')
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '连接失败')
    }
  } finally {
    connecting.value = false
  }
}

// ============================================================================
// Send
// ============================================================================

/**
 * 发送 gRPC 请求：form 模式下先把 formData 序列化为请求体；
 * 已保存的测试走 update + executeSaved 流程以确保 test_id 正确关联
 */
const handleSend = async () => {
  /** 条件分支 */
  if (!canSend.value) return

  sending.value = true
  /** 异常处理 */
  try {
    // Determine request body based on mode
    let requestBody = requestConfig.value.request_body
    /** 条件分支 */
    if (requestMode.value === 'form' && currentMessageDef.value) {
      requestBody = JSON.stringify(formData.value)
    }

    /** config 变量声明 */
    const config: GRPCRequestConfig = {
      ...requestConfig.value,
      request_body: requestBody,
      use_reflection: connectionMode.value === 'reflection',
      metadata: metadata.value.filter(m => m.key),
    }

    /** res: 可变变量 */
    let res: any

    /** 条件分支 */
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

    /** 条件分支 */
    if (currentTestId.value) {
      await loadHistory(currentTestId.value)
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '请求执行失败')
    }
  } finally {
    sending.value = false
  }
}

// ============================================================================
// Save
// ============================================================================

/**
 * 打开保存 gRPC 测试用例对话框
 * 已存在的用例默认沿用现有名称，新建用例自动填充 "service/method" 格式名称
 * @returns {void}
 */
const handleSave = () => {
  /** 条件分支 */
  if (!canSave.value) return
  const existing = savedTests.value.find(t => t.id === currentTestId.value)
  saveName.value = existing?.name || `${requestConfig.value.service}/${requestConfig.value.method}`
  saveDialogVisible.value = true
}

/**
 * 保存确认：校验名称非空，form 模式下先把 formData 序列化为请求体
 * 已存在的用例调用 update，新建用例调用 create，config 统一序列化为 JSON 字符串
 * @returns {Promise<void>}
 */
const handleSaveConfirm = async () => {
  /** 条件分支 */
  if (!saveName.value.trim()) {
    ElMessage.warning('请输入名称')
    return
  }

  saving.value = true
  /** 异常处理 */
  try {
    // Determine request body based on mode (mirrors handleSend logic)
    // In form mode, formData is the source of truth; without this serialization
    // the saved request_body would be the auto-generated template, not user input.
    let requestBody = requestConfig.value.request_body
    /** 条件分支 */
    if (requestMode.value === 'form' && currentMessageDef.value) {
      requestBody = JSON.stringify(formData.value)
    }

    /** config 变量声明 */
    const config: GRPCRequestConfig = {
      ...requestConfig.value,
      request_body: requestBody,
      use_reflection: connectionMode.value === 'reflection',
      metadata: metadata.value.filter(m => m.key),
    }

    /** 条件分支 */
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
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error((err as Error).message || '保存失败')
    }
  } finally {
    saving.value = false
  }
}

// ============================================================================
// Load / New / Delete
// ============================================================================

/**
 * 从 savedTests 中打开一个用例：恢复 requestConfig、metadata、Proto 解析态、
 * formData、编辑器内容以及历史记录
 */
const handleLoadTest = async (item: ApiTest) => {
  currentTestId.value = item.id
  /** 异常处理 */
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

    /** 条件分支 */
    if (config.proto_file_id) {
      selectedProtoFileId.value = config.proto_file_id
      await handleProtoFileChange(config.proto_file_id)

      // Restore currentMessageDef and formData from the saved request_body.
      // handleProtoFileChange resets currentMessageDef=null and formData={};
      // without this restoration the form would appear empty after loading.
      if (config.service && config.method) {
        const svc = protoServices.value.find(s => s.name === config.service)
        const method = svc?.methods.find(m => m.name === config.method)
        /** 条件分支 */
        if (method) {
          const inputType = method.input_type
          const msgDef = messageDefs.value.find(m =>
            m.full_name === inputType || m.name === inputType ||
            m.full_name.endsWith('.' + inputType)
          )
          /** 条件分支 */
          if (msgDef) {
            currentMessageDef.value = msgDef
            /** 异常处理 */
            try {
              formData.value = JSON.parse(requestConfig.value.request_body || '{}')
            } catch {
              formData.value = {}
            }
          }
        }
      }
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

/**
 * 把所有状态重置为默认值，开始一个全新的 gRPC 测试
 * 清空 requestConfig、metadata、Proto 服务树、编辑器内容、历史记录等所有测试相关状态
 * @returns {void}
 */
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

  /** 条件分支 */
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

/**
 * 确认删除某条已保存的 gRPC 测试用例
 * 若删除的是当前正在编辑的用例，自动触发 handleNewTest 重置所有状态
 * 删除后刷新左侧用例列表
 * @param {ApiTest} item - 要删除的测试用例对象
 * @returns {Promise<void>}
 */
const handleDeleteTest = async (item: ApiTest) => {
  /** 异常处理 */
  try {
    await ElMessageBox.confirm(`确定要删除测试用例 "${item.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await apiTestAPI.delete(item.id)
    ElMessage.success('已删除')
    /** 条件分支 */
    if (currentTestId.value === item.id) {
      handleNewTest()
    }
    await loadSavedTests()
  } catch (err: unknown) {
    /** 条件分支 */
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error((err as Error).message || '删除失败')
    }
  }
}

// ============================================================================
// CodeMirror Editor
// ============================================================================

/**
 * 初始化请求体 JSON 编辑器：使用 CodeMirror 6 的 basicSetup + json + oneDark 主题
 * 通过 updateListener 把文档变更实时同步回 requestConfig.request_body
 * @returns {void}
 */
const initRequestEditor = () => {
  /** 条件分支 */
  if (!requestEditorRef.value) return

  const updateListener = EditorView.updateListener.of((update) => {
    /** 条件分支 */
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

/**
 * 创建 / 重建只读响应体 JSON 编辑器：自动尝试把 lastResult.body 解析格式化（2 空格缩进）
 * 解析失败时显示原始文本；使用 CodeMirror readOnly 扩展防止用户修改
 * @returns {void}
 */
const createResponseJsonEditor = () => {
  /** 条件分支 */
  if (!responseEditorRef.value) return
  /** 条件分支 */
  if (responseEditorView) responseEditorView.destroy()

  /** formattedBody 可变变量 */
  let formattedBody = ''
  /** 异常处理 */
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

// ============================================================================
// Lifecycle & Watch
// ============================================================================

/** 全局快捷键：Ctrl/Cmd+Enter 发送，Ctrl/Cmd+S 保存 */
const handleGlobalKeydown = (e: KeyboardEvent) => {
  /** 条件分支 */
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault()
    handleSend()
  }
  /** 条件分支 */
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    handleSave()
  }
}

/** 组件挂载：并行加载用例/Proto/证书，初始化请求编辑器，注册全局快捷键 */
onMounted(async () => {
  await Promise.all([loadSavedTests(), loadProtoFiles(), loadCertificates()])
  await nextTick()
  initRequestEditor()
  document.addEventListener('keydown', handleGlobalKeydown)
})

/** 组件卸载：销毁 CodeMirror 实例，移除全局快捷键监听 */
onUnmounted(() => {
  /** 条件分支 */
  if (requestEditorView) {
    requestEditorView.destroy()
    requestEditorView = null
  }
  /** 条件分支 */
  if (responseEditorView) {
    responseEditorView.destroy()
    responseEditorView = null
  }
  document.removeEventListener('keydown', handleGlobalKeydown)
})

/** metadata 变更时同步过滤空 key 并写回 requestConfig.metadata（发送前统一使用） */
watch(metadata, (val) => {
  requestConfig.value.metadata = val.filter(m => m.key)
}, { deep: true })

/** 切换到 JSON 编辑模式时，从 currentMessageDef 生成格式化 JSON 作为起点 */
watch(requestMode, (mode) => {
  /** 条件分支 */
  if (mode === 'json') {
    // Generate formatted JSON from message definition (not from empty formData)
    const jsonStr = currentMessageDef.value
      ? generateTemplateFromMessage(currentMessageDef.value)
      : requestConfig.value.request_body
    requestConfig.value.request_body = jsonStr
    nextTick(() => {
      /** 条件分支 */
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

/** 连接模式切换时：清空服务树、service/method 等与模式相关的状态 */
watch(connectionMode, () => {
  protoServices.value = []
  requestConfig.value.service = ''
  requestConfig.value.method = ''
  expandedServiceNames.value = []
})

/** lastResult 更新且位于响应体 Tab 时，重建只读 JSON 响应编辑器 */
watch(lastResult, () => {
  nextTick(() => {
    /** 条件分支 */
    if (lastResult.value && !lastResult.value.error && activeResponseTab.value === 'body') {
      createResponseJsonEditor()
    }
  })
})

/** 切到响应体 Tab 时，若 lastResult 有效则重建响应编辑器 */
watch(activeResponseTab, () => {
  /** 条件分支 */
  if (activeResponseTab.value === 'body' && lastResult.value && !lastResult.value.error) {
    nextTick(() => createResponseJsonEditor())
  }
})

/** 响应体展示模式切回 json 时，重建格式化 JSON 编辑器 */
watch(responseBodyMode, () => {
  /** 条件分支 */
  if (responseBodyMode.value === 'json' && lastResult.value && !lastResult.value.error) {
    nextTick(() => createResponseJsonEditor())
  }
})
</script>

<style scoped>
/* ==================== CSS Variables ==================== */
/* === .grpc-test-page 样式定义 === */
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

/* === 主内容区 flex 布局（任务区 + 日志面板） === */
.main-content {
  display: flex;
  flex: 1;
  gap: var(--spacing-lg);
  min-height: 0;
  align-items: stretch;
  padding: var(--spacing-lg);
}

/* ==================== Left Panel ==================== */
/* === .saved-panel 样式定义 === */
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

/* === .saved-panel 样式定义 === */
.saved-panel:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

/* === .panel-header 样式定义 === */
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

/* === .panel-title 样式定义 === */
.panel-title {
  font-weight: 600;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

/* === .el-badge__content) 样式定义 === */
.panel-count-badge :deep(.el-badge__content) {
  font-size: 11px;
}

/* === .panel-search 样式定义 === */
.panel-search {
  padding: var(--spacing-sm) var(--spacing-md);
  border-bottom: 1px solid var(--border-subtle);
}

/* === .saved-list 样式定义 === */
.saved-list {
  flex: 1;
  overflow-y: auto;
  padding: var(--spacing-sm);
}

/* === .saved-item 样式定义 === */
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

/* === .saved-item 样式定义 === */
.saved-item:hover {
  background: var(--bg-hover);
  transform: translateX(2px);
}

/* === .active 样式定义 === */
.saved-item.active {
  background: rgba(99, 102, 241, 0.1);
  border-left-color: #6366f1;
}

/* === .saved-delete 样式定义 === */
.saved-item:hover .saved-delete {
  opacity: 1;
}

/* === .grpc-badge 样式定义 === */
.grpc-badge {
  flex-shrink: 0;
  font-size: 11px;
  min-width: 42px;
  text-align: center;
  background: #6366f1;
  border-color: #6366f1;
}

/* === .saved-item-info 样式定义 === */
.saved-item-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

/* === .saved-name 样式定义 === */
.saved-name {
  flex: 1;
  font-size: 13px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}

/* === .saved-addr 样式定义 === */
.saved-addr {
  font-size: 11px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

/* === .saved-delete 样式定义 === */
.saved-delete {
  opacity: 0;
  color: var(--text-muted);
  flex-shrink: 0;
  transition: all var(--duration-fast);
  cursor: pointer;
}

/* === .saved-delete 样式定义 === */
.saved-delete:hover {
  color: var(--accent-danger);
}

/* === .empty-saved 样式定义 === */
.empty-saved {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--spacing-xl);
  color: var(--text-muted);
  gap: var(--spacing-sm);
}

/* === .empty-saved 样式定义 === */
.empty-saved p {
  font-size: var(--font-size-sm);
  margin: 0;
}

/* === .panel-footer 样式定义 === */
.panel-footer {
  padding: var(--spacing-md);
  border-top: 1px solid var(--border-subtle);
}

/* === .new-btn 样式定义 === */
.new-btn {
  width: 100%;
}

/* ==================== Right Panel ==================== */
/* === .editor-area 样式定义 === */
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
/* === .request-section 样式定义 === */
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

/* === .panel-maximized 样式定义 === */
.request-section.panel-maximized {
  flex: 1;
  max-height: none;
  min-height: 0;
}

/* === .panel-minimized 样式定义 === */
.request-section.panel-minimized {
  display: none;
}

/* ==================== Resize Handle ==================== */
/* === .resize-handle 样式定义 === */
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
/* === .resize-line 样式定义 === */
.resize-handle:active .resize-line {
  background: #6366f1;
  height: 3px;
}

/* === .resize-line 样式定义 === */
.resize-line {
  width: 40px;
  height: 2px;
  border-radius: 2px;
  background: var(--border-default);
  transition: all 0.2s ease;
}

/* === .panel-toggle-btn 样式定义 === */
.panel-toggle-btn {
  margin-left: auto;
  flex-shrink: 0;
}

/* === .current-test-name 样式定义 === */
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

/* === .request-bar 样式定义 === */
.request-bar {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  padding: var(--spacing-md) var(--spacing-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
  flex-wrap: wrap;
}

/* === .bar-divider 样式定义 === */
.bar-divider {
  height: 24px;
  margin: 0 2px;
}

/* === .address-input 样式定义 === */
.address-input {
  flex: 1;
}

/* === .mode-select 样式定义 === */
.mode-select {
  width: 140px;
  flex-shrink: 0;
}

/* === .timeout-input 样式定义 === */
.timeout-input {
  width: 100px;
  flex-shrink: 0;
}

/* === 超时提示文本（小号灰色） === */
.timeout-hint {
  color: var(--text-muted);
  cursor: help;
  flex-shrink: 0;
}

/* === .send-btn 样式定义 === */
.send-btn {
  font-weight: 600;
  background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
}

/* === .send-btn 样式定义 === */
.send-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(99, 102, 241, 0.4);
  filter: brightness(1.05);
}

/* === .action-btn 样式定义 === */
.action-btn {
  font-weight: 500;
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  transition: all var(--duration-normal) var(--ease-out);
}

/* === .action-btn 样式定义 === */
.action-btn:hover {
  border-color: #6366f1;
  color: #6366f1;
}

/* === .request-tabs 样式定义 === */
.request-tabs {
  padding: 0 var(--spacing-lg);
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* === .el-tabs__header) 样式定义 === */
.request-tabs :deep(.el-tabs__header) {
  flex-shrink: 0;
}

/* === .el-tabs__content) 样式定义 === */
.request-tabs :deep(.el-tabs__content) {
  padding: var(--spacing-md) 0;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  transition: opacity var(--transition-tab);
}

/* === .el-tab-pane) 样式定义 === */
.request-tabs :deep(.el-tab-pane) {
  min-height: 100%;
}

/* ==================== Service Panel ==================== */
/* === .service-panel 样式定义 === */
.service-panel {
  padding: var(--spacing-md);
}

/* === .proto-selector 样式定义 === */
.proto-selector {
  margin-bottom: var(--spacing-md);
}

/* === .proto-select 样式定义 === */
.proto-select {
  width: 100%;
}

/* === .service-loading 样式定义 === */
.service-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-xl);
  color: var(--text-muted);
}

/* === .service-tree 样式定义 === */
.service-tree {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

/* === .service-collapse 样式定义 === */
.service-collapse {
  border: none;
}

/* === .el-collapse-item__header) 样式定义 === */
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

/* === .el-collapse-item__header 样式定义 === */
.service-collapse :deep(.el-collapse-item__header:hover) {
  background: var(--bg-hover);
}

/* === .el-collapse-item__wrap) 样式定义 === */
.service-collapse :deep(.el-collapse-item__wrap) {
  border: 1px solid var(--border-subtle);
  border-top: none;
  border-radius: 0 0 var(--radius-md) var(--radius-md);
  overflow: hidden;
}

/* === .el-collapse-item__content) 样式定义 === */
.service-collapse :deep(.el-collapse-item__content) {
  padding: var(--spacing-xs) 0;
}

/* === .service-name 样式定义 === */
.service-name {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  color: var(--text-primary);
}

/* === .svc-icon 样式定义 === */
.svc-icon {
  color: #6366f1;
}

/* === .method-count-tag 样式定义 === */
.method-count-tag {
  margin-left: auto;
  font-size: 11px;
}

/* === .method-item 样式定义 === */
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

/* === .method-item 样式定义 === */
.method-item:hover {
  background: var(--bg-hover);
}

/* === .active 样式定义 === */
.method-item.active {
  background: rgba(99, 102, 241, 0.08);
  color: #6366f1;
  border-left-color: #6366f1;
}

/* === .method-icon 样式定义 === */
.method-icon {
  color: var(--accent-success);
  flex-shrink: 0;
}

/* === .method-name 样式定义 === */
.method-name {
  font-weight: 500;
  color: var(--text-primary);
}

/* === .method-name 样式定义 === */
.method-item.active .method-name {
  color: #6366f1;
}

/* === .method-type 样式定义 === */
.method-type {
  color: var(--text-muted);
  font-size: 0.72rem;
  margin-left: auto;
  white-space: nowrap;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

/* === .stream-tag 样式定义 === */
.stream-tag {
  margin-left: var(--spacing-sm);
}

/* === .reflection-bar 样式定义 === */
.reflection-bar {
  margin-bottom: var(--spacing-md);
}

/* ==================== Request Mode ==================== */
/* === .request-mode-bar 样式定义 === */
.request-mode-bar {
  margin-bottom: var(--spacing-sm);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

/* === .form-wrapper 样式定义 === */
.form-wrapper {
  min-height: 200px;
}

/* === .form-container 样式定义 === */
.form-container {
  padding: var(--spacing-sm) 0;
}

/* === .form-message-header 样式定义 === */
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

/* === .editor-wrapper 样式定义 === */
.editor-wrapper {
  height: 100%;
}

/* ==================== Editor ==================== */
/* === .editor-container 样式定义 === */
.editor-container {
  height: 100%;
  overflow: hidden;
}

/* ==================== Metadata Panel ==================== */
/* === .metadata-panel 样式定义 === */
.metadata-panel {
  padding: var(--spacing-md);
}

/* ==================== TLS Panel ==================== */
/* === .tls-panel 样式定义 === */
.tls-panel {
  padding: var(--spacing-md);
}

/* === .tls-mode-wrapper 样式定义 === */
.tls-mode-wrapper {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
  width: 100%;
}

/* === .tls-mode-select 样式定义 === */
.tls-mode-select {
  width: 100%;
}

/* === .tls-option 样式定义 === */
.tls-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

/* === .tls-option-label 样式定义 === */
.tls-option-label {
  flex: 1;
}

/* === .tls-option-tag 样式定义 === */
.tls-option-tag {
  margin-left: var(--spacing-sm);
}

/* === .tls-security-indicator 样式定义 === */
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

/* === .tls-mode-desc 样式定义 === */
.tls-mode-desc {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  border-radius: var(--radius-md);
  font-size: 0.8rem;
  line-height: 1.5;
}

/* === .tls-mode-desc-danger 样式定义 === */
.tls-mode-desc-danger {
  background: rgba(245, 108, 108, 0.06);
  color: var(--accent-danger);
  border: 1px solid rgba(245, 108, 108, 0.15);
}

/* === .tls-mode-desc-warning 样式定义 === */
.tls-mode-desc-warning {
  background: rgba(230, 162, 60, 0.06);
  color: var(--accent-warning);
  border: 1px solid rgba(230, 162, 60, 0.15);
}

/* === .tls-mode-desc-success 样式定义 === */
.tls-mode-desc-success {
  background: rgba(103, 194, 58, 0.06);
  color: var(--accent-success);
  border: 1px solid rgba(103, 194, 58, 0.15);
}

/* === .cert-select 样式定义 === */
.cert-select {
  width: 100%;
}

/* === .cert-option 样式定义 === */
.cert-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

/* === .cert-option-name 样式定义 === */
.cert-option-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* === .cert-badges 样式定义 === */
.cert-badges {
  margin-left: auto;
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

/* ==================== Response Section ==================== */
/* === .response-section 样式定义 === */
.response-section {
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

/* === .panel-maximized 样式定义 === */
.response-section.panel-maximized {
  flex: 1;
  min-height: 0;
}

/* === .panel-minimized 样式定义 === */
.response-section.panel-minimized {
  display: none;
}

/* === .response-fade-enter-active 样式定义 === */
.response-fade-enter-active {
  transition: all 0.3s ease-out;
}

/* === .response-fade-leave-active 样式定义 === */
.response-fade-leave-active {
  transition: all 0.2s ease-in;
}

/* === .response-fade-enter-from 样式定义 === */
.response-fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

/* === .response-fade-leave-to 样式定义 === */
.response-fade-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}

/* === .response-header 样式定义 === */
.response-header {
  padding: var(--spacing-sm) var(--spacing-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

/* === .response-info 样式定义 === */
.response-info {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

/* === .status-badge 样式定义 === */
.status-badge {
  font-weight: 700;
  font-size: 12px;
  min-width: 44px;
  text-align: center;
  padding: 2px 8px;
}

/* === .response-meta-item 样式定义 === */
.response-meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* === .status-code 样式定义 === */
.status-code {
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

.status-ok { color: var(--accent-success); }
.status-error { color: var(--accent-danger); }

/* === .copy-response-btn 样式定义 === */
.copy-response-btn {
  color: var(--text-muted);
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  padding: 2px 8px;
  transition: all var(--duration-fast);
  flex-shrink: 0;
}

/* === .copy-response-btn 样式定义 === */
.copy-response-btn:hover {
  color: #6366f1;
  border-color: #6366f1;
}

/* === .response-tabs 样式定义 === */
.response-tabs {
  padding: 0 var(--spacing-lg);
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* === .el-tabs__header) 样式定义 === */
.response-tabs :deep(.el-tabs__header) {
  flex-shrink: 0;
}

/* === .el-tabs__content) 样式定义 === */
.response-tabs :deep(.el-tabs__content) {
  padding: var(--spacing-md) 0;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  transition: opacity var(--transition-tab);
}

/* === .el-tab-pane) 样式定义 === */
.response-tabs :deep(.el-tab-pane) {
  height: 100%;
  display: flex;
  flex-direction: column;
}

/* === .response-body-content 样式定义 === */
.response-body-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* === .response-body-header 样式定义 === */
.response-body-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--spacing-xs) 0 var(--spacing-sm);
  flex-shrink: 0;
}

/* === .response-body-modes 样式定义 === */
.response-body-modes {
  display: flex;
  align-items: center;
}

/* === .response-body-actions 样式定义 === */
.response-body-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
}

/* === .response-raw-wrapper 样式定义 === */
.response-raw-wrapper {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

/* === .response-raw 样式定义 === */
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

/* === .response-json-editor 样式定义 === */
.response-json-editor {
  flex: 1;
  min-height: 0;
}

/* === .error-content 样式定义 === */
.error-content {
  display: flex;
  align-items: flex-start;
  gap: var(--spacing-sm);
  color: var(--accent-danger);
}

/* === .error-content 样式定义 === */
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
/* === .response-headers 样式定义 === */
.response-headers {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

/* === .header-row 样式定义 === */
.header-row {
  display: flex;
  gap: var(--spacing-md);
  padding: var(--spacing-xs) 0;
  font-size: 13px;
  border-bottom: 1px solid var(--border-subtle);
}

/* === .header-key 样式定义 === */
.header-key {
  font-weight: 600;
  color: var(--text-primary);
  min-width: 200px;
}

/* === .header-value 样式定义 === */
.header-value {
  color: var(--text-secondary);
  word-break: break-all;
}

/* === .body-none 样式定义 === */
.body-none {
  padding: var(--spacing-xl);
  text-align: center;
  color: var(--text-muted);
  font-size: var(--font-size-sm);
}

/* ==================== History ==================== */
/* === .history-toolbar 样式定义 === */
.history-toolbar {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  margin-bottom: var(--spacing-sm);
}

/* === .history-list 样式定义 === */
.history-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-xs);
}

/* === .history-item 样式定义 === */
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

/* === .history-item 样式定义 === */
.history-item:hover {
  background: var(--bg-hover);
}

/* === .active 样式定义 === */
.history-item.active {
  background: rgba(99, 102, 241, 0.1);
}

/* === .history-code 样式定义 === */
.history-code {
  font-size: 12px;
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

/* === .history-latency 样式定义 === */
.history-latency {
  color: var(--text-secondary);
  min-width: 60px;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

/* === .history-time 样式定义 === */
.history-time {
  color: var(--text-muted);
  margin-left: auto;
  font-size: 12px;
}

/* ==================== Empty State ==================== */
/* === .empty-response 样式定义 === */
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

/* === .empty-response 样式定义 === */
.empty-response h3 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-primary);
  margin: var(--spacing-md) 0 var(--spacing-sm);
}

/* === .empty-response 样式定义 === */
.empty-response p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin: 0;
}

/* ==================== Compare Dialog ==================== */
/* === .compare-container 样式定义 === */
.compare-container {
  display: flex;
  gap: var(--spacing-lg);
}

/* === .compare-panel 样式定义 === */
.compare-panel {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
}

/* === .compare-header 样式定义 === */
.compare-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: var(--spacing-sm) var(--spacing-md);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 13px;
}

/* === .compare-code 样式定义 === */
.compare-code {
  font-weight: 600;
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
  font-size: 12px;
}

/* === .compare-latency 样式定义 === */
.compare-latency {
  color: var(--text-secondary);
  font-family: Monaco, Menlo, 'Ubuntu Mono', monospace;
}

/* === .compare-time 样式定义 === */
.compare-time {
  color: var(--text-muted);
  font-size: 12px;
  margin-left: auto;
}

/* === .compare-body 样式定义 === */
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
