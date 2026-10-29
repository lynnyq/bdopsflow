<!--
  SQL 查询页面
  功能：提供多数据源 SQL 查询能力，支持表结构浏览、SQL 编辑器、查询执行、虚拟表格结果展示、CSV 导出、SQL 保存等。
  支持多标签页，每个标签页维护独立的数据源选择、SQL 文本和查询状态。
-->
<template>
  <!-- === .sql-query-page 区块 === -->
  <div class="sql-query-page">
    <!-- === .main-content 区块 === -->
    <div class="main-content">
      <!-- === 左侧元数据面板：数据源选择 + 表字段浏览 === -->
      <aside class="metadata-panel">
        <!-- === .panel-header 区块 === -->
        <div class="panel-header">
          <!-- === .panel-title 区块 === -->
          <div class="panel-title">
            <!-- === 图标组件 === -->
            <el-icon :size="18"><DataLine /></el-icon>
            <span>数据源</span>
          </div>
          <!-- === .panel-actions 区块 === -->
          <div class="panel-actions">
            <!-- === 文字提示组件 === -->
            <el-tooltip content="刷新元数据" placement="top" :show-after="500">
              <!-- === 按钮组件 === -->
              <el-button
                link
                type="primary"
                size="small"
                @click="refreshMetadata"
                :disabled="!selectedDatasourceId"
              >
                <!-- === 图标组件 === -->
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>
            <!-- === 确认弹出框 === -->
            <el-popconfirm
              title="确定清除该数据源的缓存？清除后下次查询将重新从数据源获取最新数据。"
              confirm-button-text="确定"
              cancel-button-text="取消"
              @confirm="handleClearCache"
              :disabled="!selectedDatasourceId"
            >
              <!-- === 具名插槽 === -->
              <template #reference>
                <!-- === 按钮组件 === -->
                <el-button
                  link
                  type="primary"
                  size="small"
                  :disabled="!selectedDatasourceId || clearingCache"
                  :loading="clearingCache"
                >
                  <!-- === 图标组件 === -->
                  <el-icon><Delete /></el-icon>
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>

        <div class="panel-section selector-section" :class="{ collapsed: selectorCollapsed }">
          <div class="section-label selector-toggle" @click="selectorCollapsed = !selectorCollapsed">
            <span>数据源</span>
            <el-icon :size="12"><component :is="selectorCollapsed ? ArrowDown : ArrowUp" /></el-icon>
          </div>
          <div class="selector-fields" v-show="!selectorCollapsed">
            <div class="selector-field">
              <span class="field-label">数据源</span>
              <el-select
                v-model="selectedDatasourceId"
                placeholder="选择数据源"
                class="field-select"
                filterable
                :disabled="executing"
                @change="handleDatasourceChange"
                @visible-change="onDatasourceVisibleChange"
              >
                <el-option
                  v-for="ds in datasources"
                  :key="ds.id"
                  :label="ds.name"
                  :value="ds.id"
                >
                  <span>{{ ds.name }}</span>
                  <el-tag
                    size="small"
                    effect="light"
                    class="type-tag"
                    :class="`tag-${ds.type}`"
                  >
                    {{ dsTypeLabels[ds.type] || ds.type }}
                  </el-tag>
                </el-option>
              </el-select>
            </div>
            <div class="selector-field">
              <span class="field-label">数据库</span>
              <el-select-v2
                v-model="selectedDatabase"
                placeholder="选择数据库"
                class="field-select"
                :options="databaseOptions"
                filterable
                :disabled="!selectedDatasourceId || loadingDatabases"
                :loading="loadingDatabases"
                @change="handleDatabaseChange"
                @visible-change="onDatabaseVisibleChange"
              />
            </div>
            <div class="selector-field">
              <span class="field-label">表</span>
              <el-select-v2
                v-model="selectedTable"
                placeholder="选择表"
                class="field-select"
                :options="tableOptions"
                filterable
                :disabled="!selectedDatabase || loadingTables"
                :loading="loadingTables"
                @change="handleTableChange"
              />
            </div>
          </div>
        </div>

        <div class="panel-section columns-section">
          <span class="section-label">字段</span>
          <div v-if="loadingColumns" class="columns-loading">
            <el-icon class="is-loading" :size="16"><Refresh /></el-icon>
            <span>加载中...</span>
          </div>
          <div v-else class="columns-list">
            <div
              v-for="column in selectedTableColumns"
              :key="column.name"
              class="column-item"
              @click="insertColumn(column)"
            >
              <el-icon :size="12" class="column-icon"><Key /></el-icon>
              <span class="column-name">{{ column.name }}</span>
              <span class="column-type">{{ column.type }}</span>
            </div>
            <div v-if="selectedTable && (selectedTableColumns?.length === 0 || selectedTableColumns === null)" class="empty-state">
              <el-icon :size="24"><Document /></el-icon>
              <p>暂无字段数据</p>
            </div>
            <div v-if="!selectedTable" class="empty-state">
              <el-icon :size="24"><Search /></el-icon>
              <p>请选择表</p>
            </div>
          </div>
        </div>
      </aside>

      <!-- === 主查询区域：标签页 + 编辑器 + 结果展示 === -->
      <main class="query-area">
        <!-- === 查询工具栏：常用操作按钮 === -->
        <div class="query-toolbar">
          <div class="toolbar-left">
            <el-button
              @click="insertSelectAll"
              :disabled="!selectedTable || !selectedTableColumns || selectedTableColumns.length === 0"
              class="refresh-btn"
            >
              SELECT *
            </el-button>
            <el-button
              @click="insertSelectColumns"
              :disabled="!selectedTable || !selectedTableColumns || selectedTableColumns.length === 0"
              class="refresh-btn"
            >
              SELECT
            </el-button>
            <el-button
              @click="insertCount"
              :disabled="!selectedTable"
              class="refresh-btn"
            >
              COUNT
            </el-button>
            <el-divider direction="vertical" />
            <el-button
              :icon="Refresh"
              @click="handleFormat"
              :disabled="!sqlText || !sqlText.trim()"
              class="refresh-btn"
            >
              格式化
            </el-button>
            <el-button
              :icon="Document"
              @click="handleSaveSQL"
              :disabled="!sqlText || !sqlText.trim()"
              class="refresh-btn"
            >
              保存
            </el-button>
            <el-button
              :icon="Download"
              @click="handleExportCSV"
              :loading="exporting"
              :disabled="!queryResult || exporting"
              class="refresh-btn"
            >
              导出CSV
            </el-button>
            <el-button
              :icon="CircleClose"
              @click="handleCancel"
              :loading="canceling"
              :disabled="!executing"
              class="cancel-btn"
            >
              取消查询
            </el-button>
          </div>
          <div class="toolbar-right">
            <el-tag
              v-if="!allowWriteSQL"
              type="warning"
              effect="light"
              size="small"
              class="dml-warning"
            >
              <el-icon><Warning /></el-icon> 只读模式
            </el-tag>
            <el-button
              :icon="VideoPlay"
              @click="handleExecute"
              :loading="executing"
              :disabled="!sqlText || !sqlText.trim() || !selectedDatasourceId"
              class="create-btn"
            >
              执行查询
            </el-button>
          </div>
        </div>

        <div class="editor-container">
          <!-- === 编辑器顶部工具栏：标签栏 + 历史下拉 + 清空按钮 === -->
          <div class="editor-header">
            <div class="tab-bar">
              <!-- === 可滚动的标签页列表，支持多标签切换和关闭 === -->
              <div class="tabs-scroll">
                <div
                  v-for="tab in tabs"
                  :key="tab.id"
                  class="tab-item"
                  :class="{ active: tab.id === activeTabId }"
                  @click="handleTabSwitch(tab.id)"
                >
                  <el-icon
                    v-if="tab.executing"
                    class="tab-spinner is-loading"
                    :size="12"
                  >
                    <Refresh />
                  </el-icon>
                  <span class="tab-name" @dblclick="startRenameTab(tab)">
                    {{ tab.name }}
                  </span>
                  <el-icon
                    v-if="tabs.length > 1"
                    class="tab-close"
                    :size="12"
                    @click.stop="handleCloseTab(tab.id)"
                  >
                    <Close />
                  </el-icon>
                </div>
                <el-button class="tab-add-btn" :icon="Plus" link size="small" @click="handleAddTab" />
              </div>
            </div>
            <span class="editor-meta">
              <el-dropdown trigger="click" @command="handleHistorySelect" v-if="recentSQLHistory.length > 0">
                <el-button link size="small">
                  <el-icon><Clock /></el-icon> 历史
                </el-button>
                <!-- === 具名插槽 === -->
                <template #dropdown>
                  <!-- === 下拉菜单 === -->
                  <el-dropdown-menu>
                    <!-- === 下拉菜单 === -->
                    <el-dropdown-item
                      v-for="(item, index) in recentSQLHistory"
                      :key="index"
                      :command="item"
                    >
                      <!-- === .history-item 区块 === -->
                      <div class="history-item">
                        <span class="history-sql">{{ item.sql_text }}</span>
                        <span class="history-time">{{ formatDateTime(item.created_at) }}</span>
                      </div>
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <span class="sql-length" v-if="sqlText">{{ sqlText.length }} / 65536</span>
              <el-button
                link
                size="small"
                @click="clearEditor"
              >
                <el-icon><Delete /></el-icon> 清空
              </el-button>
            </span>
          </div>
          <!-- === CodeMirror SQL 编辑器包装器，高度可拖拽调整 === -->
          <div
            class="editor-wrapper"
            :style="{ height: `${editorHeight}px` }"
          >
            <div ref="editorRef" class="sql-editor"></div>
          </div>
          <!-- === 编辑器高度拖拽调整手柄 === -->
          <div
            class="resize-handle"
            @mousedown="startResize"
            title="拖拽调整高度"
          ></div>
        </div>

        <!-- === 查询结果展示：虚拟表格 + 错误提示 === -->
        <div class="result-container" v-if="queryResult || errorMessage">
          <div class="result-header">
            <div class="result-info">
              <!-- === 条件/循环模板 === -->
              <template v-if="queryResult">
                <!-- === 标签组件 === -->
                <el-tag type="success" effect="light" size="small">
                  <!-- === 图标组件 === -->
                  <el-icon><CircleCheck /></el-icon> 查询成功
                </el-tag>
                <span class="result-meta">
                  <span class="meta-item">
                    <!-- === 图标组件 === -->
                    <el-icon :size="14"><DataLine /></el-icon>
                    {{ queryResult.row_count }} 行
                    <!-- === 条件渲染区块 === -->
                    <span v-if="queryResult.row_count > MAX_DISPLAY_ROWS" class="truncated-hint">(显示前 {{ MAX_DISPLAY_ROWS }} 行)</span>
                  </span>
                  <span class="meta-item">
                    <!-- === 图标组件 === -->
                    <el-icon :size="14"><Grid /></el-icon>
                    {{ (queryResult.columns || []).length }} 列
                  </span>
                  <span class="meta-item">
                    <!-- === 图标组件 === -->
                    <el-icon :size="14"><Clock /></el-icon>
                    {{ formatTime(queryResult.execution_time) }}
                  </span>
                  <!-- === 条件渲染区块 === -->
                  <span v-if="queryResult.from_cache" class="meta-item cache-badge">
                    <!-- === 图标组件 === -->
                    <el-icon :size="14"><Clock /></el-icon> 缓存
                  </span>
                </span>
              </template>
              <!-- === 条件/循环模板 === -->
              <template v-if="errorMessage">
                <!-- === 标签组件 === -->
                <el-tag type="danger" effect="light" size="small">
                  <!-- === 图标组件 === -->
                  <el-icon><CircleClose /></el-icon> 查询失败
                </el-tag>
                <span class="result-meta error-text">{{ errorMessage }}</span>
              </template>
            </div>
          </div>

          <div class="result-body" v-if="queryResult">
            <!-- === 虚拟表格容器：大数据量时只渲染可见行列，避免 DOM 爆炸 === -->
            <div
              v-if="queryResult.rows && queryResult.rows.length > 0"
              class="virtual-table-wrapper"
              ref="virtualTableRef"
              @scroll="onVirtualScroll"
            >
              <table class="virtual-table" :style="{ width: virtualTotalWidth + 'px' }">
                <!-- === 列组：行号列 + 左填充列 + 可见数据列 + 右填充列 === -->
                <colgroup>
                  <col style="width: 50px" />
                  <col v-if="virtualPaddingLeft > 0" :style="{ width: virtualPaddingLeft + 'px' }" />
                  <col v-for="ci in virtualVisibleColumns" :key="ci" :style="{ width: getColWidth(ci) + 'px' }" />
                  <col v-if="virtualPaddingRight > 0" :style="{ width: virtualPaddingRight + 'px' }" />
                </colgroup>
                <!-- === 表头行：行号 + 数据列（每列带拖拽调整手柄） === -->
                <thead>
                  <tr>
                    <th class="vt-col-index">#</th>
                    <th v-if="virtualPaddingLeft > 0" class="vt-spacer-cell"></th>
                    <th
                      v-for="ci in virtualVisibleColumns"
                      :key="ci"
                      class="vt-cell vt-header"
                      :class="{ 'vt-col-right': columnAlignMap[queryResult.columns[ci]] === 'right' }"
                      :title="queryResult.columns[ci]"
                    >
                      <span class="vt-header-text">{{ queryResult.columns[ci] }}</span>
                      <div
                        class="vt-resize-handle"
                        @mousedown="startColResize($event, ci)"
                      ></div>
                    </th>
                    <th v-if="virtualPaddingRight > 0" class="vt-spacer-cell"></th>
                  </tr>
                </thead>
                <!-- === 表体：上填充行 + 可见数据行 + 下填充行（虚拟滚动核心） === -->
                <tbody>
                  <tr v-if="virtualPaddingTop > 0" class="vt-spacer-row" :style="{ height: virtualPaddingTop + 'px' }">
                    <td :colspan="virtualColSpan"></td>
                  </tr>
                  <tr v-for="ri in virtualVisibleRowIndices" :key="ri">
                    <td class="vt-col-index" :class="{ 'vt-row-even': ri % 2 === 1 }">{{ ri + 1 }}</td>
                    <td v-if="virtualPaddingLeft > 0" class="vt-spacer-cell"></td>
                    <td
                      v-for="ci in virtualVisibleColumns"
                      :key="ci"
                      class="vt-cell"
                      :class="{ 'vt-col-right': columnAlignMap[queryResult.columns[ci]] === 'right', 'vt-row-even': ri % 2 === 1 }"
                      :title="formatCellValue(queryResult.rows[ri]?.[ci])"
                    >{{ formatCellValue(queryResult.rows[ri]?.[ci]) }}</td>
                    <td v-if="virtualPaddingRight > 0" class="vt-spacer-cell"></td>
                  </tr>
                  <tr v-if="virtualPaddingBottom > 0" class="vt-spacer-row" :style="{ height: virtualPaddingBottom + 'px' }">
                    <td :colspan="virtualColSpan"></td>
                  </tr>
                </tbody>
              </table>
            </div>
            <div v-else class="empty-result">
              <el-icon :size="48"><Document /></el-icon>
              <p>查询结果为空</p>
            </div>
          </div>
        </div>

        <!-- === 空查询引导 === -->
        <div class="empty-query" v-if="!queryResult && !errorMessage">
          <div class="empty-icon">
            <el-icon :size="64"><Document /></el-icon>
          </div>
          <h3>开始查询</h3>
          <p>选择数据源和表，编写 SQL 语句后点击执行查询</p>
          <div class="empty-hint">
            <el-icon :size="16"><Key /></el-icon>
            <span>提示：⌘+Enter / Ctrl+Enter 执行（选中内容优先），⌘+Shift+F / Ctrl+Shift+F 格式化，点击字段名可插入到编辑器</span>
          </div>
        </div>
      </main>
    </div>

    <!-- === 保存 SQL 对话框 === -->
    <el-dialog
      v-model="saveDialogVisible"
      title="保存 SQL"
      width="480px"
      :close-on-click-modal="false"
    >
      <el-form ref="saveFormRef" :model="saveForm" :rules="saveRules" label-position="top" class="save-form">
        <el-form-item label="保存方式">
          <el-radio-group v-model="saveMode">
            <el-radio-button value="new">保存为新的</el-radio-button>
            <el-radio-button value="overwrite">覆盖已有的</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="saveMode === 'new'" label="新名称" prop="name">
          <el-input
            v-model="saveForm.name"
            placeholder="请输入新的 SQL 名称"
            maxlength="100"
            show-word-limit
          />
        </el-form-item>
        <el-form-item v-else label="选择已有名称" prop="name">
          <el-select
            v-model="saveForm.name"
            placeholder="请选择要覆盖的已有名称"
            style="width: 100%"
            :filterable="false"
          >
            <el-option
              v-for="item in savedSQLNameList"
              :key="item.id"
              :label="item.name"
              :value="item.name"
            />
          </el-select>
          <div v-if="saveForm.name" class="save-overwrite-hint">
            <el-icon><Warning /></el-icon> 将覆盖已有的「{{ saveForm.name }}」
          </div>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="saveForm.description" type="textarea" :rows="2" placeholder="请输入描述" />
        </el-form-item>
        <el-form-item label="公开">
          <el-switch v-model="saveForm.is_public" />
          <span class="form-hint">公开的 SQL 可被同领域其他用户使用</span>
        </el-form-item>
      </el-form>
      <!-- === 具名插槽 === -->
      <template #footer>
        <!-- === .dialog-footer 区块 === -->
        <div class="dialog-footer">
          <!-- === 按钮组件 === -->
          <el-button @click="saveDialogVisible = false">取消</el-button>
          <!-- === 按钮组件 === -->
          <el-button type="primary" @click="handleConfirmSave" :loading="saving">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * SQLQuery 组件
 * 功能：多数据源 SQL 查询页面，提供 SQL 编辑器（CodeMirror 6）、虚拟表格结果展示、
 *       多标签页管理、元数据缓存、异步轮询查询、CSV 导出、SQL 保存等完整功能。
 *       支持 MySQL/PostgreSQL/ClickHouse/Doris 等 9 种数据库类型。
 *
 * 主要特性：
 * - 多标签页：每个标签维护独立的数据源、SQL 文本和查询状态，持久化到 localStorage
 * - 虚拟表格：大数据量结果使用虚拟滚动，支持列宽拖拽调整
 * - 元数据缓存：30 分钟内存缓存 + localStorage 持久化，减少重复请求
 * - 异步查询：通过 SSE + 轮询机制处理长耗时查询
 */

// ========== 依赖导入 ==========
import { ref, watch, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRoute } from 'vue-router';
// CodeMirror 6 编辑器核心和扩展
import { EditorView, basicSetup } from 'codemirror';
import { keymap } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { sql } from '@codemirror/lang-sql';
import { oneDark } from '@codemirror/theme-one-dark';
import { autocompletion, CompletionContext, CompletionResult } from '@codemirror/autocomplete';
import { historyKeymap } from '@codemirror/commands';
import { searchKeymap } from '@codemirror/search';
// SQL 格式化工具
import { format } from 'sql-formatter';
import { ElMessage } from 'element-plus';
import {
  DataLine, Refresh, Key, Document, Search,
  CircleClose, VideoPlay, Delete, CircleCheck, Clock, Download, Warning,
  Plus, Close, ArrowDown, ArrowUp, Grid
} from '@element-plus/icons-vue';
import { datasourceAPI, queryAPI } from '@/api';
import { isHandledError } from '@/utils/api';
import { formatErrorMessage, getErrorResponse } from '@/utils/error';
import { secureRandomHex } from '@/utils/random';
import { useAuthStore } from '@/stores/auth';
import type { Datasource, QueryResult, QueryHistory, TableInfo, ColumnInfo, SavedSQL } from '@/types';

// ========== 全局依赖 ==========
const authStore = useAuthStore();  // 认证状态管理
/** route: 当前路由对象 */
const route = useRoute();  // 当前路由对象

/**
 * 生成本地存储键名，按用户 ID 隔离，防止多用户共用浏览器时数据串扰
 * @returns {string} localStorage 键名，格式为 bdopsflow_sql_tabs_{userId}
 */
const getStorageKey = () => `bdopsflow_sql_tabs_${authStore.user?.id || 'anonymous'}`;

// ========== 标签页数据结构 ==========

/** SQL 标签页接口 */
interface SQLTab {
  id: string;                    // 标签唯一 ID
  name: string;                  // 标签显示名称（如 "查询 1"）
  sql: string;                   // SQL 文本内容
  datasourceId: number | '';     // 当前选择的数据源 ID
  database: string;              // 当前选择的数据库名
  executing: boolean;            // 是否正在执行查询
  canceling: boolean;            // 是否正在取消查询
  queryId: string;               // 后端返回的查询 ID（用于轮询/取消）
  queryResult: QueryResult | null; // 查询结果
  errorMessage: string;          // 查询错误信息
}

/**
 * 生成唯一的标签页 ID，使用时间戳 + 6 位随机十六进制字符串组合，确保并发创建时不重复
 * @returns {string} 唯一标签页 ID，格式为 tab_{timestamp}_{randomHex}
 */
const generateTabId = () => `tab_${Date.now()}_${secureRandomHex(6)}`;

/**
 * 获取当前标签页可用的最小序号（如已有 "查询 1" 和 "查询 3"，返回 2）
 * @returns {number} 最小未使用的序号
 */
const getAvailableTabNumber = () => {
  /** usedNumbers: 变量/函数声明 */
  const usedNumbers = new Set<number>();
  tabs.value.forEach(tab => {
    /** match: 变量/函数声明 */
    const match = tab.name.match(/查询 (\d+)/);
    /** 条件分支 */
    if (match) {
      usedNumbers.add(parseInt(match[1], 10));
    }
  });

  // 找到最小的未使用的序号
  let num = 1;
  /** 循环 */
  while (usedNumbers.has(num)) {
    num++;
  }
  /** 返回/抛出 */
  return num;
};

/**
 * 从 localStorage 恢复标签页数据
 * @returns 恢复的标签页列表和活跃标签 ID；如果没有缓存则返回默认标签
 */
const loadTabsFromStorage = (): { tabs: SQLTab[]; activeTabId: string } => {
  /** 异常处理 */
  try {
    /** raw: 变量/函数声明 */
    const raw = localStorage.getItem(getStorageKey());
    /** 条件分支 */
    if (raw) {
      /** data: 变量/函数声明 */
      const data = JSON.parse(raw);
      /** 条件分支 */
      if (data.tabs && data.tabs.length > 0) {
        /** loadedTabs 变量声明 */
        const loadedTabs: SQLTab[] = data.tabs.map((t: any) => ({
          id: t.id,
          name: t.name,
          sql: t.sql || '',
          datasourceId: '',
          database: '',
          executing: false,
          canceling: false,
          queryId: '',
          queryResult: null,
          errorMessage: '',
        }));
        /** 返回/抛出 */
        return { tabs: loadedTabs, activeTabId: data.activeTabId || loadedTabs[0].id };
      }
    }
  } catch (e) {
  }
  /** defaultTab 变量声明 */
  const defaultTab: SQLTab = {
    id: generateTabId(),
    name: '查询 1',
    sql: '',
    datasourceId: '',
    database: '',
    executing: false,
    canceling: false,
    queryId: '',
    queryResult: null,
    errorMessage: '',
  };
  /** 返回/抛出 */
  return { tabs: [defaultTab], activeTabId: defaultTab.id };
};

// ========== 标签页状态 ==========
const saved = loadTabsFromStorage();  // 从 localStorage 恢复的初始标签数据
/** tabs: 变量/函数声明 */
const tabs = ref<SQLTab[]>(saved.tabs);  // 所有标签页
/** activeTabId: 变量/函数声明 */
const activeTabId = ref<string>(saved.activeTabId);  // 当前活跃标签 ID

/** watch 监听器 */
watch(() => authStore.user?.id, (newUserId, oldUserId) => {
  /** 条件分支 */
  if (newUserId !== oldUserId && newUserId) {
    syncCurrentTabData();
    stopAllPolling();
    /** loaded: 变量/函数声明 */
    const loaded = loadTabsFromStorage();
    tabs.value = loaded.tabs;
    activeTabId.value = loaded.activeTabId;
    sqlText.value = activeTab.value.sql;
    selectedDatasourceId.value = activeTab.value.datasourceId;
    selectedDatabase.value = activeTab.value.database;
    selectedTable.value = '';
    nextTick(() => {
      /** 条件分支 */
      if (editorView) {
        editorView.dispatch({
          changes: { from: 0, to: editorView.state.doc.length, insert: sqlText.value }
        });
      }
    });
  }
});

/**
 * 将标签页数据防抖后持久化到 localStorage，避免频繁写入
 * 仅存储 id/name/sql/datasourceId/database 字段，不缓存执行结果
 * @returns {void}
 */
const saveTabsToStorage = () => {
  /** 条件分支 */
  if (saveTabsTimer) clearTimeout(saveTabsTimer);
  saveTabsTimer = setTimeout(() => {
    /** 异常处理 */
    try {
      /** storableTabs: 变量/函数声明 */
      const storableTabs = tabs.value.map(t => ({
        id: t.id,
        name: t.name,
        sql: t.sql,
        datasourceId: t.datasourceId,
        database: t.database,
      }));
      localStorage.setItem(getStorageKey(), JSON.stringify({
        tabs: storableTabs,
        activeTabId: activeTabId.value,
      }));
    } catch (e) {
    }
  }, 500);
};

/** 当前活跃标签（computed） */
const activeTab = computed(() => tabs.value.find(t => t.id === activeTabId.value) || tabs.value[0]);

// ========== CodeMirror 编辑器 ==========
const editorRef = ref<HTMLElement>();  // 编辑器 DOM 引用
/** editorHeight 响应式引用 */
const editorHeight = ref(200);          // 编辑器高度（px）
/** isResizing 响应式引用 */
const isResizing = ref(false);          // 是否正在拖拽调整编辑器高度
/** sqlText 响应式引用 */
const sqlText = ref(activeTab.value.sql);  // 当前编辑器 SQL 文本（与标签同步）
/** selectorCollapsed 响应式引用 */
const selectorCollapsed = ref(false);   // 左侧数据源面板是否折叠

// ========== 操作状态 ==========
const exporting = ref(false)             // 是否正在导出 CSV
/** exportProgress 响应式引用 */
const exportProgress = ref(0)           // CSV 导出进度（百分比）
/** allowWriteSQL 响应式引用 */
const allowWriteSQL = ref(false)         // 是否允许写操作（INSERT/UPDATE/DELETE）
/** clearingCache 响应式引用 */
const clearingCache = ref(false);        // 是否正在清除缓存

// ========== 查询轮询/取消机制 ==========
const pollTimers = new Map<string, ReturnType<typeof setInterval>>();  // 每个查询 ID 对应的轮询定时器
/** sseConnections: 变量/函数声明 */
const sseConnections = new Map<string, EventSource>();  // 每个查询 ID 对应的 SSE 连接

// ========== 活跃标签状态便捷 computed ==========
const executing = computed(() => activeTab.value.executing);        // 当前标签是否执行中
/** canceling 计算属性 */
const canceling = computed(() => activeTab.value.canceling);        // 当前标签是否取消中
/** queryResult 计算属性 */
const queryResult = computed(() => activeTab.value.queryResult);    // 当前标签的查询结果
/** errorMessage 计算属性 */
const errorMessage = computed(() => activeTab.value.errorMessage);  // 当前标签的错误信息

// ========== 数据源状态 ==========
const datasources = ref<Datasource[]>([]);           // 当前用户可见的数据源列表
/** selectedDatasourceId: 变量/函数声明 */
const selectedDatasourceId = ref<number | ''>(activeTab.value.datasourceId);  // 当前选中的数据源 ID
/** databases: 变量/函数声明 */
const databases = ref<string[]>([]);                  // 当前数据源下的数据库列表
/** selectedDatabase 响应式引用 */
const selectedDatabase = ref(activeTab.value.database);  // 当前选中的数据库名
/** tables: 变量/函数声明 */
const tables = ref<TableInfo[]>([]);                  // 当前数据库下的表列表
/** selectedTable 响应式引用 */
const selectedTable = ref('');                        // 当前选中的表名
/** selectedTableColumns: 变量/函数声明 */
const selectedTableColumns = ref<ColumnInfo[]>([]);   // 当前选中表的字段列表
/** loadingDatabases 响应式引用 */
const loadingDatabases = ref(false);                  // 是否正在加载数据库列表
/** loadingTables 响应式引用 */
const loadingTables = ref(false);                     // 是否正在加载表列表
/** loadingColumns 响应式引用 */
const loadingColumns = ref(false);                    // 是否正在加载字段列表

// ========== 元数据缓存机制 ==========
let metadataAbortController: AbortController | null = null;  // 用于取消正在进行的元数据请求
/** metadataCache: 变量/函数声明 */
const metadataCache = new Map<string, { data: any; timestamp: number }>();  // 内存缓存：key → {data, timestamp}

/**
 * 取消所有正在进行的元数据请求，防止用户快速切换数据源/数据库时旧请求的响应覆盖新状态
 * 通过 AbortController.abort() 中断未完成的 fetch 请求
 * @returns {void}
 */
const cancelPendingMetadataRequests = () => {
  /** 条件分支 */
  if (metadataAbortController) {
    metadataAbortController.abort();
    metadataAbortController = null;
  }
};

/**
 * 创建新的 AbortSignal 用于元数据请求，同时通过 cancelPendingMetadataRequests() 取消上一个未完成的请求
 * @returns {AbortSignal | undefined} 新创建的 AbortSignal，可传入 fetch 的 signal 参数
 */
const getMetadataSignal = (): AbortSignal | undefined => {
  cancelPendingMetadataRequests();
  metadataAbortController = new AbortController();
  /** 返回/抛出 */
  return metadataAbortController.signal;
};

/**
 * 判断错误是否为主动取消请求导致的（ERR_CANCELED / CanceledError / AbortError）
 * 这类错误不需要向用户展示，直接忽略即可
 * @param {any} err - 待判断的错误对象
 * @returns {boolean} true 表示是取消导致的错误，false 表示是真实错误
 */
const isCanceledError = (err: any): boolean => {
  /** 返回/抛出 */
  return err?.code === 'ERR_CANCELED' || err?.name === 'CanceledError' || err?.name === 'AbortError';
};

/** 元数据缓存有效期：30 分钟 */
const METADATA_CACHE_TTL = 30 * 60 * 1000;
/** localStorage 中元数据缓存键前缀 */
const METADATA_STORAGE_PREFIX = 'bdopsflow_meta_cache_';

/**
 * 拼接 localStorage 元数据缓存键，添加统一前缀便于后续批量清理
 * @param {string} key - 原始缓存键（如 databases_1 / tables_1_dbName）
 * @returns {string} 带前缀的完整 localStorage 键名
 */
const getStorageCacheKey = (key: string) => `${METADATA_STORAGE_PREFIX}${key}`;

/**
 * 获取元数据缓存：先查内存缓存，未命中则查 localStorage，均未命中返回 null
 * @param key 缓存键
 * @returns 缓存数据或 null
 */
const getCachedMetadata = (key: string): any | null => {
  /** entry: 变量/函数声明 */
  const entry = metadataCache.get(key);
  /** 条件分支 */
  if (entry) {
    /** 条件分支 */
    if (Date.now() - entry.timestamp <= METADATA_CACHE_TTL) {
      /** 返回/抛出 */
      return entry.data;
    }
    metadataCache.delete(key);
  }

  /** 异常处理 */
  try {
    /** raw: 变量/函数声明 */
    const raw = localStorage.getItem(getStorageCacheKey(key));
    /** 条件分支 */
    if (!raw) return null;
    /** stored: 变量/函数声明 */
    const stored = JSON.parse(raw) as { data: any; timestamp: number };
    /** 条件分支 */
    if (Date.now() - stored.timestamp > METADATA_CACHE_TTL) {
      localStorage.removeItem(getStorageCacheKey(key));
      /** 返回/抛出 */
      return null;
    }
    metadataCache.set(key, stored);
    /** 返回/抛出 */
    return stored.data;
  } catch {
    /** 返回/抛出 */
    return null;
  }
};

/**
 * 写入元数据缓存：同时更新内存和 localStorage，超过 2MB 时跳过 localStorage
 * @param key 缓存键
 * @param data 要缓存的数据
 */
const setCachedMetadata = (key: string, data: any) => {
  /** entry 对象常量 */
  const entry = { data, timestamp: Date.now() };
  metadataCache.set(key, entry);
  /** 异常处理 */
  try {
    /** serialized: 变量/函数声明 */
    const serialized = JSON.stringify(entry);
    /** 条件分支 */
    if (serialized.length > 2 * 1024 * 1024) {
      return;
    }
    localStorage.setItem(getStorageCacheKey(key), serialized);
  } catch {
    // localStorage 可能被禁用或配额已满，静默忽略
  }
};

// ========== CodeMirror 自动补全数据源 ==========

// SQL 关键词分类
const SQL_KEYWORDS = {
  // DDL 关键词
  ddl: [
    'CREATE', 'ALTER', 'DROP', 'TRUNCATE', 'RENAME',
    'TABLE', 'DATABASE', 'SCHEMA', 'INDEX', 'VIEW', 'PROCEDURE', 'FUNCTION',
    'ADD', 'MODIFY', 'CHANGE', 'DROP COLUMN', 'ADD COLUMN',
    'PRIMARY KEY', 'FOREIGN KEY', 'UNIQUE', 'CHECK', 'CONSTRAINT',
    'DEFAULT', 'NOT NULL', 'AUTO_INCREMENT', 'IDENTITY',
    'COMMENT', 'ENGINE', 'CHARSET', 'COLLATE'
  ],
  // DML 关键词
  dml: [
    'SELECT', 'INSERT', 'UPDATE', 'DELETE', 'MERGE',
    'INTO', 'VALUES', 'SET', 'FROM', 'WHERE',
    'AS', 'DISTINCT', 'ALL', 'TOP', 'LIMIT', 'OFFSET',
    'UNION', 'UNION ALL', 'INTERSECT', 'EXCEPT'
  ],
  // 查询子句
  query: [
    'JOIN', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'OUTER JOIN',
    'FULL JOIN', 'CROSS JOIN', 'NATURAL JOIN',
    'ON', 'USING',
    'GROUP BY', 'HAVING',
    'ORDER BY', 'ASC', 'DESC',
    'WITH', 'WITH ROLLUP', 'WITH CUBE'
  ],
  // 条件和逻辑
  condition: [
    'AND', 'OR', 'NOT', 'XOR',
    'IN', 'NOT IN', 'EXISTS', 'NOT EXISTS',
    'BETWEEN', 'NOT BETWEEN',
    'LIKE', 'NOT LIKE', 'RLIKE', 'REGEXP',
    'IS NULL', 'IS NOT NULL', 'IS',
    'TRUE', 'FALSE', 'UNKNOWN',
    'ANY', 'ALL', 'SOME'
  ],
  // 聚合函数
  aggregate: [
    'COUNT', 'SUM', 'AVG', 'MAX', 'MIN',
    'COUNT(DISTINCT', 'SUM(DISTINCT',
    'STDDEV', 'STDDEV_SAMP', 'STDDEV_POP',
    'VARIANCE', 'VAR_SAMP', 'VAR_POP',
    'GROUP_CONCAT', 'STRING_AGG', 'ARRAY_AGG',
    'JSON_ARRAYAGG', 'JSON_OBJECTAGG'
  ],
  // 字符串函数
  string: [
    'CONCAT', 'CONCAT_WS', 'SUBSTRING', 'SUBSTR',
    'LEFT', 'RIGHT', 'MID',
    'LENGTH', 'CHAR_LENGTH', 'CHARACTER_LENGTH',
    'LOWER', 'UPPER', 'INITCAP',
    'TRIM', 'LTRIM', 'RTRIM',
    'REPLACE', 'TRANSLATE',
    'REVERSE', 'LPAD', 'RPAD',
    'POSITION', 'INSTR', 'LOCATE',
    'SPLIT_PART', 'REGEXP_REPLACE', 'REGEXP_SUBSTR'
  ],
  // 数值函数
  numeric: [
    'ABS', 'SIGN',
    'CEIL', 'CEILING', 'FLOOR', 'ROUND', 'TRUNC',
    'MOD', 'DIV',
    'POWER', 'SQRT', 'EXP',
    'LN', 'LOG', 'LOG10', 'LOG2',
    'SIN', 'COS', 'TAN', 'ASIN', 'ACOS', 'ATAN',
    'RADIANS', 'DEGREES', 'PI',
    'RAND', 'RANDOM', 'RANDOM()',
    'GREATEST', 'LEAST', 'COALESCE', 'NULLIF'
  ],
  // 日期和时间函数
  datetime: [
    'NOW', 'CURRENT_TIMESTAMP', 'CURRENT_DATE', 'CURRENT_TIME',
    'LOCALTIMESTAMP', 'LOCALTIME', 'SYSDATE',
    'DATE', 'TIME', 'DATETIME', 'TIMESTAMP',
    'YEAR', 'MONTH', 'DAY', 'HOUR', 'MINUTE', 'SECOND',
    'MONTHNAME', 'DAYNAME',
    'DAYOFWEEK', 'DAYOFMONTH', 'DAYOFYEAR',
    'WEEK', 'WEEKOFYEAR', 'QUARTER',
    'DATE_ADD', 'DATE_SUB', 'ADDDATE', 'SUBDATE',
    'DATEDIFF', 'TIMESTAMPDIFF',
    'DATE_FORMAT', 'TIME_FORMAT', 'TO_CHAR', 'TO_DATE', 'TO_TIMESTAMP',
    'EXTRACT', 'DATE_TRUNC', 'TRUNCATE'
  ],
  // 窗口函数
  window: [
    'ROW_NUMBER', 'RANK', 'DENSE_RANK', 'PERCENT_RANK',
    'NTILE', 'CUME_DIST',
    'FIRST_VALUE', 'LAST_VALUE', 'NTH_VALUE',
    'LAG', 'LEAD',
    'OVER', 'PARTITION BY', 'ROWS BETWEEN', 'RANGE BETWEEN'
  ],
  // CASE 表达式
  case: [
    'CASE', 'WHEN', 'THEN', 'ELSE', 'END',
    'IF', 'IFNULL', 'NULLIF', 'COALESCE'
  ],
  // JSON 函数
  json: [
    'JSON_EXTRACT', 'JSON_VALUE', 'JSON_OBJECT', 'JSON_ARRAY',
    'JSON_ARRAYAGG', 'JSON_OBJECTAGG',
    'JSON_CONTAINS', 'JSON_CONTAINS_PATH',
    'JSON_KEYS', 'JSON_LENGTH', 'JSON_DEPTH',
    'JSON_SET', 'JSON_INSERT', 'JSON_REPLACE', 'JSON_REMOVE',
    'JSON_MERGE', 'JSON_MERGE_PRESERVE',
    '->', '->>'
  ],
  // 数据类型
  types: [
    'INT', 'INTEGER', 'BIGINT', 'SMALLINT', 'TINYINT', 'MEDIUMINT',
    'FLOAT', 'DOUBLE', 'DECIMAL', 'NUMERIC', 'REAL',
    'VARCHAR', 'CHAR', 'TEXT', 'TINYTEXT', 'MEDIUMTEXT', 'LONGTEXT',
    'DATE', 'TIME', 'DATETIME', 'TIMESTAMP', 'YEAR',
    'BINARY', 'VARBINARY', 'BLOB', 'TINYBLOB', 'MEDIUMBLOB', 'LONGBLOB',
    'BOOLEAN', 'BOOL', 'BIT',
    'ENUM', 'SET',
    'JSON', 'JSONB',
    'UUID', 'SERIAL', 'BIGSERIAL'
  ],
  // 其他
  other: [
    'USE', 'DESCRIBE', 'DESC', 'EXPLAIN', 'ANALYZE',
    'SHOW', 'SHOW TABLES', 'SHOW DATABASES', 'SHOW COLUMNS',
    'BEGIN', 'COMMIT', 'ROLLBACK', 'START TRANSACTION',
    'GRANT', 'REVOKE', 'PRIVILEGES',
    'LOCK', 'UNLOCK', 'TABLES',
    '*'
  ]
};

// 合并所有关键词用于默认补全
const ALL_KEYWORDS = [
  ...SQL_KEYWORDS.dml,
  ...SQL_KEYWORDS.query,
  ...SQL_KEYWORDS.condition,
  ...SQL_KEYWORDS.aggregate,
  ...SQL_KEYWORDS.string,
  ...SQL_KEYWORDS.numeric,
  ...SQL_KEYWORDS.datetime,
  ...SQL_KEYWORDS.ddl,
  ...SQL_KEYWORDS.window,
  ...SQL_KEYWORDS.case,
  ...SQL_KEYWORDS.json,
  ...SQL_KEYWORDS.types,
  ...SQL_KEYWORDS.other
];
// ALL_KEYWORDS 预留给未来 SQL 关键字补全扩展，当前未直接使用
void ALL_KEYWORDS;

// CodeMirror 动态补全数据（当前数据源的表名和字段名）
const autocompleteData = ref<{
  tables: string[];
  columns: string[];
}>({
  tables: [],
  columns: []
});

// ========== SQL 保存对话框状态 ==========
const saveDialogVisible = ref(false);  // 保存对话框是否可见
/** saving 响应式引用 */
const saving = ref(false);              // 是否正在保存
/** saveForm 响应式引用 */
const saveForm = ref({                  // 保存表单数据
  name: '',
  description: '',
  is_public: false
});
/** saveRules 响应式引用 */
const saveRules = ref({                 // 表单校验规则
  name: [
    { required: true, message: '请输入SQL名称', trigger: 'blur' },
    { min: 1, max: 100, message: '名称长度在1到100个字符', trigger: 'blur' }
  ]
});
/** saveFormRef 响应式引用 */
const saveFormRef = ref();              // 保存表单 DOM 引用
/** savedSQLNameList: 变量/函数声明 */
const savedSQLNameList = ref<SavedSQL[]>([]);  // 已保存的 SQL 名称列表（用于覆盖模式）
/** saveMode: 变量/函数声明 */
const saveMode = ref<'new' | 'overwrite'>('new');  // 保存模式：新建/覆盖

/** recentSQLHistory: 变量/函数声明 */
const recentSQLHistory = ref<{ sql_text: string; created_at: string }[]>([]);  // 最近执行历史

/** 数据源类型标签映射 */
const dsTypeLabels: Record<string, string> = {
  mysql: 'MySQL',
  hive: 'Hive',
  kyuubi: 'Kyuubi',
  trino: 'Trino',
  spark: 'Spark',
  starrocks: 'StarRocks',
  doris: 'Doris',
  sqlite: 'SQLite',
  rqlite: 'rqlite',
  redis: 'Redis'
};

// ========== 编辑器实例和辅助状态 ==========
let editorView: EditorView | null = null;  // CodeMirror 编辑器实例引用
/** saveTabsTimer: 可变变量 */
let saveTabsTimer: ReturnType<typeof setTimeout> | null = null;  // 标签页持久化防抖定时器

// ========== 下拉选项 computed ==========
const databaseOptions = computed(() =>
  databases.value.map(db => ({ label: db, value: db }))  // 为 el-select-v2 生成选项
);

/** tableOptions 计算属性 */
const tableOptions = computed(() => {
  /** opts: 变量/函数声明 */
  const opts: { label: string; value: string; disabled?: boolean }[] = []
  // 第一项显示表总数（不可选）
  if (tables.value.length > 0) {
    opts.push({ label: `共 ${tables.value.length} 张表`, value: '__table_count__', disabled: true })
  }
  tables.value.forEach(t => opts.push({ label: t.name, value: t.name }))
  /** 返回/抛出 */
  return opts
})

/** 数值类型集合，用于虚拟表格列对齐判断 */
const numericTypes = new Set([
  'int', 'integer', 'bigint', 'smallint', 'tinyint', 'mediumint',
  'float', 'double', 'decimal', 'numeric', 'real', 'number',
  'money', 'serial', 'bigserial', 'smallserial'
]);

/** 计算每列的对齐方式（数值列右对齐，其他左对齐） */
const columnAlignMap = computed(() => {
  /** map: 变量/函数声明 */
  const map: Record<string, string> = {};
  /** 条件分支 */
  if (!selectedTableColumns.value.length) return map;
  /** 循环 */
  for (const col of selectedTableColumns.value) {
    /** 条件分支 */
    if (numericTypes.has(col.type.toLowerCase().split('(')[0])) {
      map[col.name] = 'right';
    }
  }
  /** 返回/抛出 */
  return map;
});

/**
 * 格式化单元格值用于结果表格显示
 * - null/undefined 显示为 "-"
 * - 布尔值显示为 "是"/"否"
 * - ArrayBuffer 显示为 "[BLOB]"
 * - 其他直接转字符串
 * @param {any} value - 原始单元格值
 * @returns {string} 格式化后的显示文本
 */
const formatCellValue = (value: any): string => {
  /** 条件分支 */
  if (value === null || value === undefined) return '-';
  /** 条件分支 */
  if (typeof value === 'boolean') return value ? '是' : '否';
  /** 条件分支 */
  if (value instanceof ArrayBuffer || (typeof value === 'object' && value.constructor?.name === 'ArrayBuffer')) {
    /** 返回/抛出 */
    return '[BLOB]';
  }
  /** 返回/抛出 */
  return String(value);
};

// ========== 虚拟表格（Virtual Scrolling）实现 ==========
// 大数据量结果使用虚拟滚动，只渲染视口内可见的行/列，避免 DOM 爆炸
// ==================== 虚拟滚动配置 ====================
const MAX_DISPLAY_ROWS = 500;  // 最多在前端显示的行数（超过需导出查看完整数据）
/** VT_ROW_HEIGHT: 变量/函数声明 */
const VT_ROW_HEIGHT = 33;       // 每行高度(px)
/** VT_DEFAULT_COL_WIDTH: 变量/函数声明 */
const VT_DEFAULT_COL_WIDTH = 120; // 每列默认宽度(px)
/** VT_MIN_COL_WIDTH: 变量/函数声明 */
const VT_MIN_COL_WIDTH = 60;    // 每列最小宽度(px)
/** VT_INDEX_COL_WIDTH: 变量/函数声明 */
const VT_INDEX_COL_WIDTH = 50;  // 行号列宽度(px)
/** VT_OVERSCAN: 变量/函数声明 */
const VT_OVERSCAN = 5;          // 视口外额外渲染的行/列数

/** virtualTableRef: 变量/函数声明 */
const virtualTableRef = ref<HTMLElement>();
/** vtScrollTop 响应式引用 */
const vtScrollTop = ref(0);
/** vtScrollLeft 响应式引用 */
const vtScrollLeft = ref(0);

// 每列宽度状态管理
const columnWidths = ref<number[]>([]);

// 监听查询结果变化，初始化列宽
watch(queryResult, (result) => {
  vtScrollTop.value = 0;
  vtScrollLeft.value = 0;
  /** 条件分支 */
  if (virtualTableRef.value) {
    virtualTableRef.value.scrollTop = 0;
    virtualTableRef.value.scrollLeft = 0;
  }
  /** 条件分支 */
  if (result?.columns) {
    columnWidths.value = result.columns.map(() => VT_DEFAULT_COL_WIDTH);
  } else {
    columnWidths.value = [];
  }
});

/**
 * 获取指定列的宽度（像素）
 * @param {number} colIndex - 列索引
 * @returns {number} 列宽度，未设置时返回默认宽度 VT_DEFAULT_COL_WIDTH
 */
const getColWidth = (colIndex: number): number => {
  /** 返回/抛出 */
  return columnWidths.value[colIndex] ?? VT_DEFAULT_COL_WIDTH;
};

// 总行数/列数
const vtTotalRows = computed(() => {
  /** 条件分支 */
  if (!queryResult.value?.rows) return 0;
  /** 返回/抛出 */
  return Math.min(queryResult.value.rows.length, MAX_DISPLAY_ROWS);
});
/** vtTotalCols 计算属性 */
const vtTotalCols = computed(() => queryResult.value?.columns?.length ?? 0);

// 撑开滚动区域的总宽度
const virtualTotalWidth = computed(() => {
  /** total 可变变量 */
  let total = VT_INDEX_COL_WIDTH;
  /** 循环 */
  for (let i = 0; i < vtTotalCols.value; i++) {
    total += getColWidth(i);
  }
  /** 返回/抛出 */
  return total;
});

// 计算可见行索引范围
const virtualVisibleRowIndices = computed(() => {
  /** 条件分支 */
  if (!virtualTableRef.value) return [] as number[];
  /** containerHeight: 变量/函数声明 */
  const containerHeight = virtualTableRef.value.clientHeight || 400;
  /** startRow: 变量/函数声明 */
  const startRow = Math.max(0, Math.floor(vtScrollTop.value / VT_ROW_HEIGHT) - VT_OVERSCAN);
  /** visibleCount: 变量/函数声明 */
  const visibleCount = Math.ceil(containerHeight / VT_ROW_HEIGHT) + VT_OVERSCAN * 2;
  /** endRow: 变量/函数声明 */
  const endRow = Math.min(vtTotalRows.value, startRow + visibleCount);
  /** indices 变量声明 */
  const indices: number[] = [];
  /** 循环 */
  for (let i = startRow; i < endRow; i++) indices.push(i);
  /** 返回/抛出 */
  return indices;
});

// 计算可见列索引范围
const virtualVisibleColumns = computed(() => {
  /** 条件分支 */
  if (!virtualTableRef.value) return [] as number[];
  /** containerWidth: 变量/函数声明 */
  const containerWidth = virtualTableRef.value.clientWidth || 800;
  /** scrollLeft: 变量/函数声明 */
  const scrollLeft = vtScrollLeft.value;
  /** effectiveLeft: 变量/函数声明 */
  const effectiveLeft = Math.max(0, scrollLeft - VT_INDEX_COL_WIDTH);

  // 根据每列实际宽度计算可见范围
  let accumulated = 0;
  /** startCol 可变变量 */
  let startCol = 0;
  /** 循环 */
  for (let i = 0; i < vtTotalCols.value; i++) {
    /** w: 变量/函数声明 */
    const w = getColWidth(i);
    /** 条件分支 */
    if (accumulated + w > effectiveLeft) {
      startCol = Math.max(0, i - VT_OVERSCAN);
      break;
    }
    accumulated += w;
  }

  // 找到可见区域结束列
  let visibleWidth = 0;
  /** endCol 可变变量 */
  let endCol = startCol;
  /** 循环 */
  for (let i = startCol; i < vtTotalCols.value; i++) {
    visibleWidth += getColWidth(i);
    endCol = i + 1;
    /** 条件分支 */
    if (visibleWidth >= containerWidth + VT_OVERSCAN * VT_DEFAULT_COL_WIDTH) break;
  }
  endCol = Math.min(vtTotalCols.value, endCol + VT_OVERSCAN);

  /** indices 变量声明 */
  const indices: number[] = [];
  /** 循环 */
  for (let i = startCol; i < endCol; i++) indices.push(i);
  /** 返回/抛出 */
  return indices;
});

// 虚拟滚动上下留白
const virtualPaddingTop = computed(() => {
  /** firstRow: 变量/函数声明 */
  const firstRow = virtualVisibleRowIndices.value[0] ?? 0;
  /** 返回/抛出 */
  return firstRow * VT_ROW_HEIGHT;
});
/** virtualPaddingBottom 计算属性 */
const virtualPaddingBottom = computed(() => {
  /** indices: 变量/函数声明 */
  const indices = virtualVisibleRowIndices.value;
  /** 条件分支 */
  if (indices.length === 0) return 0;
  /** lastRow: 变量/函数声明 */
  const lastRow = indices[indices.length - 1];
  /** 返回/抛出 */
  return Math.max(0, (vtTotalRows.value - lastRow - 1) * VT_ROW_HEIGHT);
});

// 虚拟滚动左右留白
const virtualPaddingLeft = computed(() => {
  /** firstCol: 变量/函数声明 */
  const firstCol = virtualVisibleColumns.value[0] ?? 0;
  /** left 可变变量 */
  let left = 0;
  /** 循环 */
  for (let i = 0; i < firstCol; i++) {
    left += getColWidth(i);
  }
  /** 返回/抛出 */
  return left;
});
/** virtualPaddingRight 计算属性 */
const virtualPaddingRight = computed(() => {
  /** indices: 变量/函数声明 */
  const indices = virtualVisibleColumns.value;
  /** 条件分支 */
  if (indices.length === 0) return 0;
  /** lastCol: 变量/函数声明 */
  const lastCol = indices[indices.length - 1];
  /** right 可变变量 */
  let right = 0;
  /** 循环 */
  for (let i = lastCol + 1; i < vtTotalCols.value; i++) {
    right += getColWidth(i);
  }
  /** 返回/抛出 */
  return right;
});

// spacer 行的 colspan
const virtualColSpan = computed(() => {
  /** span 可变变量 */
  let span = 1; // 行号列
  /** 条件分支 */
  if (virtualPaddingLeft.value > 0) span++;
  span += virtualVisibleColumns.value.length;
  /** 条件分支 */
  if (virtualPaddingRight.value > 0) span++;
  /** 返回/抛出 */
  return span;
});

/**
 * 虚拟表格滚动事件处理器，使用 requestAnimationFrame 节流
 * 将容器 scrollTop/scrollLeft 同步到响应式变量，驱动 computed 重新计算可见行列
 * @returns {void}
 */
let vtRafId = 0;
/** onVirtualScroll 箭头函数 */
const onVirtualScroll = () => {
  /** 条件分支 */
  if (vtRafId) return;
  vtRafId = requestAnimationFrame(() => {
    /** 条件分支 */
    if (virtualTableRef.value) {
      vtScrollTop.value = virtualTableRef.value.scrollTop;
      vtScrollLeft.value = virtualTableRef.value.scrollLeft;
    }
    vtRafId = 0;
  });
};

// ========== 虚拟表格：列宽拖拽调整 ==========
// 通过 mousedown/mousemove/mouseup 事件实现列宽拖拽
const resizingColIndex = ref<number | null>(null);
/** resizeStartX 响应式引用 */
const resizeStartX = ref(0);
/** resizeStartWidth 响应式引用 */
const resizeStartWidth = ref(0);

/**
 * 开始列宽拖拽：记录鼠标起始 X 坐标和目标列当前宽度，然后监听全局 mousemove/mouseup
 * @param {MouseEvent} e - mousedown 事件
 * @param {number} colIndex - 要拖拽调整宽度的列索引
 * @returns {void}
 */
const startColResize = (e: MouseEvent, colIndex: number) => {
  e.preventDefault();
  e.stopPropagation();
  resizingColIndex.value = colIndex;
  resizeStartX.value = e.clientX;
  resizeStartWidth.value = getColWidth(colIndex);
  document.addEventListener('mousemove', onColResize);
  document.addEventListener('mouseup', stopColResize);
};

/**
 * 列宽拖拽进行中：根据鼠标水平移动差更新当前拖拽列的宽度
 * @param {MouseEvent} e - 鼠标移动事件
 */
const onColResize = (e: MouseEvent) => {
  /** 条件分支 */
  if (resizingColIndex.value === null) return;
  /** delta: 变量/函数声明 */
  const delta = e.clientX - resizeStartX.value;
  /** newWidth: 变量/函数声明 */
  const newWidth = Math.max(VT_MIN_COL_WIDTH, resizeStartWidth.value + delta);
  columnWidths.value[resizingColIndex.value] = newWidth;
};

/**
 * 结束列宽拖拽：清除拖拽状态和 document 上的 mousemove/mouseup 全局监听
 * @returns {void}
 */
const stopColResize = () => {
  resizingColIndex.value = null;
  document.removeEventListener('mousemove', onColResize);
  document.removeEventListener('mouseup', stopColResize);
};

// ========== 标签页管理 ==========

/**
 * 将当前视图中的 SQL 文本、数据源、数据库选择同步到 activeTab 对象
 * 切换标签或保存前调用，确保 localStorage 持久化的是最新状态
 * @returns {void}
 */
const syncCurrentTabData = () => {
  /** tab: 变量/函数声明 */
  const tab = tabs.value.find(t => t.id === activeTabId.value);
  /** 条件分支 */
  if (!tab) return;
  tab.sql = sqlText.value;
  tab.datasourceId = selectedDatasourceId.value;
  tab.database = selectedDatabase.value;
  saveTabsToStorage();
};

/**
 * 切换到指定标签页，同步 SQL 文本、数据源选择和编辑器内容
 * 切换前先把当前视图状态保存到旧标签，然后从新标签恢复全部状态
 * @param {string} tabId - 目标标签页 ID
 * @returns {void}
 */
const handleTabSwitch = (tabId: string) => {
  /** 条件分支 */
  if (tabId === activeTabId.value) return;

  /** currentTab: 变量/函数声明 */
  const currentTab = tabs.value.find(t => t.id === activeTabId.value);
  /** 条件分支 */
  if (currentTab) {
    currentTab.sql = sqlText.value;
    currentTab.datasourceId = selectedDatasourceId.value;
    currentTab.database = selectedDatabase.value;
  }

  activeTabId.value = tabId;
  /** newTab: 变量/函数声明 */
  const newTab = tabs.value.find(t => t.id === tabId);
  /** 条件分支 */
  if (!newTab) return;

  sqlText.value = newTab.sql;
  selectedDatasourceId.value = newTab.datasourceId;
  selectedDatabase.value = newTab.database;

  /** 条件分支 */
  if (editorView) {
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: newTab.sql }
    });
  }

  /** 条件分支 */
  if (newTab.datasourceId) {
    handleDatasourceChangeForTab(newTab.datasourceId, newTab.database);
  } else {
    databases.value = [];
    tables.value = [];
    selectedTable.value = '';
    selectedTableColumns.value = [];
    allowWriteSQL.value = false;
  }

  saveTabsToStorage();
};

/**
 * 为指定数据源加载数据库列表（带缓存），可指定预选中的数据库名
 * @param {number | ''} dsId - 数据源 ID
 * @param {string} dbName - 预选中的数据库名（可选）
 * @returns {Promise<void>}
 */
const handleDatasourceChangeForTab = async (dsId: number | '', dbName: string) => {
  databases.value = [];
  selectedDatabase.value = '';
  tables.value = [];
  selectedTable.value = '';
  selectedTableColumns.value = [];

  /** 条件分支 */
  if (!dsId) {
    allowWriteSQL.value = false;
    return;
  }

  /** ds: 变量/函数声明 */
  const ds = datasources.value.find(d => d.id === dsId);
  /** 条件分支 */
  if (!ds) {
    allowWriteSQL.value = false;
    return;
  }

  allowWriteSQL.value = ds.allow_write_sql || false;

  /** 条件分支 */
  if (ds.type === 'sqlite' || ds.type === 'rqlite') {
    databases.value = ['main'];
    selectedDatabase.value = dbName || 'main';
    await handleDatabaseChange();
    return;
  }

  /** cacheKey: 变量/函数声明 */
  const cacheKey = `databases_${dsId}`;
  /** cached: 变量/函数声明 */
  const cached = getCachedMetadata(cacheKey);
  /** 条件分支 */
  if (cached) {
    databases.value = cached;
    /** 条件分支 */
    if (dbName && cached.includes(dbName)) {
      selectedDatabase.value = dbName;
      await handleDatabaseChange();
    } else if (cached.length === 1) {
      selectedDatabase.value = cached[0];
      await handleDatabaseChange();
    }
    return;
  }

  loadingDatabases.value = true;
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.getDatabases(dsId as number, getMetadataSignal());
    /** dbList 可变变量 */
    let dbList = res.data || [];
    /** 条件分支 */
    if (dbList.length === 0) {
      dbList = ds.database ? [ds.database] : ['default'];
    }
    databases.value = dbList;
    setCachedMetadata(cacheKey, dbList);
    /** 条件分支 */
    if (dbName && dbList.includes(dbName)) {
      selectedDatabase.value = dbName;
      await handleDatabaseChange();
    } else if (dbList.length === 1) {
      selectedDatabase.value = dbList[0];
      await handleDatabaseChange();
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isCanceledError(err) && !isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(`获取数据库列表失败: ${msg}`);
    }
    databases.value = ds.database ? [ds.database] : ['default'];
    /** 条件分支 */
    if (dbName) {
      selectedDatabase.value = dbName;
      await handleDatabaseChange();
    }
  } finally {
    loadingDatabases.value = false;
  }
};

/**
 * 新建标签页，自动编号（如 "查询 1"、"查询 2" ...）并继承当前数据源/数据库选择
 * 新标签创建后立即成为活跃标签，清空编辑器
 * @returns {void}
 */
const handleAddTab = () => {
  syncCurrentTabData();
  /** newNum: 变量/函数声明 */
  const newNum = getAvailableTabNumber();
  /** newTab 变量声明 */
  const newTab: SQLTab = {
    id: generateTabId(),
    name: `查询 ${newNum}`,
    sql: '',
    datasourceId: selectedDatasourceId.value,
    database: selectedDatabase.value,
    executing: false,
    canceling: false,
    queryId: '',
    queryResult: null,
    errorMessage: '',
  };
  tabs.value.push(newTab);
  activeTabId.value = newTab.id;

  sqlText.value = '';
  /** 条件分支 */
  if (editorView) {
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: '' }
    });
  }

  saveTabsToStorage();
};

/**
 * 关闭指定标签页，同时清理该标签的查询轮询定时器和 SSE 连接
 * 仅保留 1 个标签时不允许关闭；关闭活跃标签时自动切换到相邻标签
 * @param {string} tabId - 要关闭的标签页 ID
 * @returns {void}
 */
const handleCloseTab = (tabId: string) => {
  /** 条件分支 */
  if (tabs.value.length <= 1) return;

  stopPolling(tabId);

  /** idx: 变量/函数声明 */
  const idx = tabs.value.findIndex(t => t.id === tabId);
  /** 条件分支 */
  if (idx === -1) return;

  tabs.value.splice(idx, 1);

  /** 条件分支 */
  if (tabId === activeTabId.value) {
    /** newIdx: 变量/函数声明 */
    const newIdx = Math.min(idx, tabs.value.length - 1);
    handleTabSwitch(tabs.value[newIdx].id);
  } else {
    saveTabsToStorage();
  }
};

/**
 * 重命名标签页（通过 prompt 对话框输入新名称）
 * @param {SQLTab} tab - 要重命名的标签页对象
 */
const startRenameTab = (tab: SQLTab) => {
  /** newName: 变量/函数声明 */
  const newName = prompt('重命名标签页', tab.name);
  /** 条件分支 */
  if (newName && newName.trim()) {
    tab.name = newName.trim();
    saveTabsToStorage();
  }
};

/**
 * 开始拖拽调整编辑器高度，监听全局 mousemove/mouseup 事件
 * @param {MouseEvent} e - mousedown 事件
 */
const startResize = (e: MouseEvent) => {
  isResizing.value = true;
  document.addEventListener('mousemove', onResize);
  document.addEventListener('mouseup', stopResize);
  e.preventDefault();
};

/**
 * 编辑器高度拖拽中：根据鼠标纵向移动差调整 editorHeight（限制 120px ~ queryArea 高度 - 200px）
 * @param {MouseEvent} e - mousemove 事件
 */
const onResize = (e: MouseEvent) => {
  /** 条件分支 */
  if (!isResizing.value) return;
  /** container: 变量/函数声明 */
  const container = document.querySelector('.query-area');
  /** 条件分支 */
  if (!container) return;
  /** rect: 变量/函数声明 */
  const rect = container.getBoundingClientRect();
  /** maxHeight: 变量/函数声明 */
  const maxHeight = rect.height - 200;
  /** minHeight: 变量/函数声明 */
  const minHeight = 120;
  /** deltaY: 变量/函数声明 */
  const deltaY = e.movementY;
  /** newHeight: 变量/函数声明 */
  const newHeight = editorHeight.value + deltaY;
  editorHeight.value = Math.max(minHeight, Math.min(maxHeight, newHeight));
};

/** 结束编辑器高度拖拽：清除拖拽状态和全局事件监听 */
const stopResize = () => {
  isResizing.value = false;
  document.removeEventListener('mousemove', onResize);
  document.removeEventListener('mouseup', stopResize);
};

// ========== CodeMirror 编辑器初始化 ==========
/**
 * 创建并初始化 CodeMirror 6 编辑器实例
 * 配置内容：SQL 语法高亮、One Dark 主题、自定义自动补全、快捷键、搜索、撤销历史
 */
const createEditor = () => {
  /** 条件分支 */
  if (!editorRef.value) return;

  // 判断关键词属于哪一类
  const getKeywordCategory = (keyword: string): string => {
    /** 条件分支 */
    if (SQL_KEYWORDS.aggregate.includes(keyword)) return '聚合函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.string.includes(keyword)) return '字符串函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.numeric.includes(keyword)) return '数值函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.datetime.includes(keyword)) return '日期函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.window.includes(keyword)) return '窗口函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.json.includes(keyword)) return 'JSON函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.case.includes(keyword)) return '条件函数';
    /** 条件分支 */
    if (SQL_KEYWORDS.types.includes(keyword)) return '数据类型';
    /** 条件分支 */
    if (SQL_KEYWORDS.ddl.includes(keyword)) return 'DDL关键字';
    /** 条件分支 */
    if (SQL_KEYWORDS.query.includes(keyword)) return '查询子句';
    /** 条件分支 */
    if (SQL_KEYWORDS.condition.includes(keyword)) return '条件逻辑';
    /** 返回/抛出 */
    return '关键字';
  };

  // 使用 getKeywordCategory 以避免 TS6133 警告
  if (false) { getKeywordCategory(''); }

  // 智能补全函数
  const sqlCompletion = (context: CompletionContext): CompletionResult | null => {
    /** textBefore: 变量/函数声明 */
    const textBefore = context.state.doc.sliceString(0, context.pos);

    // 表.字段 的补全
    const dotMatch = context.matchBefore(/[\w]+\.[\w]*/);
    /** 条件分支 */
    if (dotMatch) {
      /** dotIdx: 变量/函数声明 */
      const dotIdx = dotMatch.text.lastIndexOf('.');
      /** tableName: 变量/函数声明 */
      const tableName = dotMatch.text.substring(0, dotIdx);
      /** colPrefix: 变量/函数声明 */
      const colPrefix = dotMatch.text.substring(dotIdx + 1).toLowerCase();
      /** cols: 变量/函数声明 */
      const cols = autocompleteData.value.columns
        .filter(c => c.toLowerCase().startsWith(colPrefix))
        .map(c => ({
          label: c,
          type: 'property' as const,
          detail: `${tableName}.字段`,
          apply: c
        }));
      /** 条件分支 */
      if (cols.length === 0) return null;
      /** 返回/抛出 */
      return { from: dotMatch.from + dotIdx + 1, options: cols, validFor: /^\w*$/ };
    }

    /** word: 变量/函数声明 */
    const word = context.matchBefore(/\w+/);
    /** 条件分支 */
    if (!word || (word.from === word.to && !context.explicit)) return null;

    /** prefix: 变量/函数声明 */
    const prefix = word.text.toLowerCase();
    /** beforeWord: 变量/函数声明 */
    const beforeWord = textBefore.slice(0, word.from).trimEnd().toUpperCase();

    /** endsWithAny: 箭头函数 */
    const endsWithAny = (kws: string[]) => kws.some(k => beforeWord.endsWith(k) || beforeWord.endsWith(k + ' '));

    /** @type CompOpt 类型别名 */
    type CompOpt = { label: string; type: 'keyword' | 'class' | 'property' | 'function'; detail: string; apply: string };
    /** options: 可变变量 */
    let options: CompOpt[] = [];

    // 根据上下文智能推荐
    if (endsWithAny(['SELECT', 'DISTINCT'])) {
      options = [
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c })),
        ...SQL_KEYWORDS.aggregate.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '聚合函数',
          apply: k + '($1)'
        })),
        ...['DISTINCT', 'CASE', 'NULL', '*'].map(k => ({
          label: k,
          type: (k === '*' ? 'keyword' : 'function') as CompOpt['type'],
          detail: k === '*' ? '通配符' : '关键字',
          apply: k === 'CASE' ? 'CASE WHEN $1 THEN $2 ELSE $3 END' : k
        }))
      ];
    } else if (endsWithAny(['FROM', 'JOIN', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'OUTER JOIN', 'CROSS JOIN', 'UPDATE', 'INTO'])) {
      options = autocompleteData.value.tables.map(t => ({ label: t, type: 'class' as const, detail: '表', apply: t }));
    } else if (endsWithAny(['WHERE', 'AND', 'OR', 'NOT', 'ON', 'HAVING', 'SET'])) {
      options = [
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c })),
        ...SQL_KEYWORDS.condition.map(k => ({
          label: k,
          type: 'keyword' as const,
          detail: '条件逻辑',
          apply: k.includes('(') ? k.replace('(', '($1)') : k
        })),
        ...SQL_KEYWORDS.string.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '字符串函数',
          apply: k + '($1)'
        })).slice(0, 10),
        ...SQL_KEYWORDS.datetime.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '日期函数',
          apply: k + '($1)'
        })).slice(0, 5)
      ];
    } else if (endsWithAny(['ORDER BY', 'GROUP BY'])) {
      options = [
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c })),
        ...SQL_KEYWORDS.aggregate.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '聚合函数',
          apply: k + '($1)'
        })),
        ...['ASC', 'DESC'].map(k => ({ label: k, type: 'keyword' as const, detail: '排序', apply: k }))
      ];
    } else if (endsWithAny(['INSERT'])) {
      options = [
        ...['INTO'].map(k => ({ label: k, type: 'keyword' as const, detail: '关键字', apply: k })),
        ...autocompleteData.value.tables.map(t => ({ label: t, type: 'class' as const, detail: '表', apply: t }))
      ];
    } else if (endsWithAny(['VALUES'])) {
      options = [...SQL_KEYWORDS.case, ...SQL_KEYWORDS.other].filter(k => k !== 'VALUES').map(k => ({
        label: k,
        type: 'keyword' as const,
        detail: '关键字',
        apply: k
      }));
    } else if (endsWithAny(['CREATE', 'ALTER', 'DROP'])) {
      options = SQL_KEYWORDS.ddl.map(k => ({
        label: k,
        type: 'keyword' as const,
        detail: 'DDL关键字',
        apply: k
      }));
    } else {
      // 默认情况：推荐所有类型，按优先级排序
      options = [
        ...SQL_KEYWORDS.dml.map(k => ({
          label: k,
          type: 'keyword' as const,
          detail: getKeywordCategory(k),
          apply: k
        })),
        ...autocompleteData.value.tables.map(t => ({
          label: t,
          type: 'class' as const,
          detail: '表',
          apply: t
        })),
        ...autocompleteData.value.columns.map(c => ({
          label: c,
          type: 'property' as const,
          detail: '字段',
          apply: c
        })),
        ...SQL_KEYWORDS.aggregate.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '聚合函数',
          apply: k + '($1)'
        })),
        ...SQL_KEYWORDS.string.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '字符串函数',
          apply: k + '($1)'
        })),
        ...SQL_KEYWORDS.datetime.map(k => ({
          label: k,
          type: 'function' as const,
          detail: '日期函数',
          apply: k + '($1)'
        })),
        ...SQL_KEYWORDS.other.map(k => ({
          label: k,
          type: 'keyword' as const,
          detail: getKeywordCategory(k),
          apply: k
        }))
      ];
    }

    // 过滤符合前缀的选项
    if (prefix) {
      options = options.filter(o => {
        /** labelLower: 变量/函数声明 */
        const labelLower = o.label.toLowerCase();
        // 支持模糊匹配，不仅限于开头
        return labelLower.includes(prefix) || labelLower.startsWith(prefix);
      });
    }

    /** 条件分支 */
    if (options.length === 0) return null;

    // 智能排序：
    // 1. 精确前缀匹配优先
    // 2. 同一类型中按字母排序
    options.sort((a, b) => {
      /** aStartsWith: 变量/函数声明 */
      const aStartsWith = a.label.toLowerCase().startsWith(prefix);
      /** bStartsWith: 变量/函数声明 */
      const bStartsWith = b.label.toLowerCase().startsWith(prefix);
      
      /** 条件分支 */
      if (aStartsWith && !bStartsWith) return -1;
      /** 条件分支 */
      if (!aStartsWith && bStartsWith) return 1;
      
      // 字段 > 表 > 函数 > 关键字
      const typeOrder = { property: 0, class: 1, function: 2, keyword: 3 };
      /** 条件分支 */
      if (typeOrder[a.type] !== typeOrder[b.type]) {
        /** 返回/抛出 */
        return typeOrder[a.type] - typeOrder[b.type];
      }
      
      /** 返回/抛出 */
      return a.label.localeCompare(b.label);
    });

    // 去重（保留第一个出现的）
    const seen = new Set<string>();
    options = options.filter(o => {
      /** 条件分支 */
      if (seen.has(o.label)) return false;
      seen.add(o.label);
      /** 返回/抛出 */
      return true;
    });

    /** 返回/抛出 */
    return {
      from: word.from,
      options: options.slice(0, 100),
      validFor: /^\w*$/
    };
  };

  /** updateListener: 变量/函数声明 */
  const updateListener = EditorView.updateListener.of((update) => {
    /** 条件分支 */
    if (update.docChanged) {
      sqlText.value = update.state.doc.toString();
      /** tab: 变量/函数声明 */
      const tab = tabs.value.find(t => t.id === activeTabId.value);
      /** 条件分支 */
      if (tab) {
        tab.sql = sqlText.value;
        saveTabsToStorage();
      }
    }
  });

  /** state: 变量/函数声明 */
  const state = EditorState.create({
    doc: sqlText.value,
    extensions: [
      basicSetup,
      sql(),
      oneDark,
      autocompletion({ 
        override: [sqlCompletion], 
        activateOnTypingDelay: 120,
        defaultKeymap: true,
        closeOnBlur: false
      }),
      updateListener,
      keymap.of([
        {
          key: 'Ctrl-Enter',
          run: () => {
            handleExecute();
            /** 返回/抛出 */
            return true;
          },
          preventDefault: true
        },
        {
          key: 'Cmd-Enter',
          run: () => {
            handleExecute();
            /** 返回/抛出 */
            return true;
          },
          preventDefault: true
        },
        {
          key: 'Ctrl-Shift-f',
          run: () => {
            handleFormat();
            /** 返回/抛出 */
            return true;
          },
          preventDefault: true
        },
        {
          key: 'Cmd-Shift-f',
          run: () => {
            handleFormat();
            /** 返回/抛出 */
            return true;
          },
          preventDefault: true
        },
        ...historyKeymap,
        ...searchKeymap
      ]),
      EditorView.theme({
        '&': { height: '100%' },
        '.cm-content': { padding: '14px 16px', fontSize: '14px', fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace' },
        '.cm-line': { padding: '3px 0', lineHeight: '1.6' },
        '.cm-editor': { height: '100%', backgroundColor: '#1e1e2e' },
        '.cm-scroller': { overflow: 'auto' },
        // 自动补全面板样式优化
        '.cm-tooltip': {
          borderRadius: '8px',
          boxShadow: '0 8px 32px rgba(0, 0, 0, 0.35)',
          border: '1px solid rgba(255, 255, 255, 0.1)'
        },
        '.cm-tooltip.cm-tooltip-autocomplete': {
          backgroundColor: '#1a1a2e',
          minWidth: '300px',
          maxHeight: '400px'
        },
        '.cm-completionLabel': {
          fontSize: '14px',
          fontFamily: 'Monaco, Menlo, "Ubuntu Mono", monospace'
        },
        '.cm-completionDetail': {
          fontSize: '12px',
          fontStyle: 'normal',
          color: '#a5b4fc',
          opacity: 0.8
        },
        '.cm-completionIcon': {
          width: '18px',
          height: '18px',
          marginRight: '8px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center'
        },
        // 为不同类型设置不同的图标颜色
        '.cm-completionIcon-function': {
          color: '#f59e0b'
        },
        '.cm-completionIcon-keyword': {
          color: '#3b82f6'
        },
        '.cm-completionIcon-property': {
          color: '#10b981'
        },
        '.cm-completionIcon-class': {
          color: '#8b5cf6'
        },
        '.cm-completionIcon-variable': {
          color: '#ec4899'
        },
        '.cm-completionIcon-text': {
          color: '#6b7280'
        },
        '.cm-completionIcon-constant': {
          color: '#ef4444'
        }
      }),
    ]
  });

  editorView = new EditorView({
    state,
    parent: editorRef.value
  });
};

/** watch 监听器 */
watch(editorHeight, () => {
  /** 条件分支 */
  if (editorView && editorRef.value) {
    requestAnimationFrame(() => {
      editorView?.requestMeasure();
    });
  }
});

/** watch 监听器 */
watch([selectedDatasourceId, selectedDatabase], () => {
  syncCurrentTabData();
});

/**
 * 加载当前用户可见的数据源列表，同时校验当前选中的数据源是否仍有效
 * 若选中的数据源已被删除，自动清空数据源/数据库/表选择
 * @returns {Promise<void>}
 */
const loadDatasources = async () => {
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.list();
    /** 条件分支 */
    if (res.data) {
      datasources.value = res.data.items || res.data;
    }
    /** 条件分支 */
    if (selectedDatasourceId.value) {
      /** found: 变量/函数声明 */
      const found = datasources.value.find((d: any) => d.id === selectedDatasourceId.value);
      /** 条件分支 */
      if (!found) {
        selectedDatasourceId.value = '';
        databases.value = [];
        selectedDatabase.value = '';
        tables.value = [];
      }
    }
  } catch (err) {
  }
};

/**
 * 加载最近执行的 SQL 历史（用于历史下拉菜单）
 * @returns {Promise<void>}
 */
const loadRecentHistory = async () => {
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await queryAPI.getHistory({ page: 1, page_size: 10 });
    /** 条件分支 */
    if (res.data) {
      recentSQLHistory.value = (res.data.items || []).map((item: QueryHistory) => ({
        sql_text: item.sql_text,
        created_at: item.created_at
      }));
    }
  } catch (err) {
  }
};

/**
 * 将历史 SQL 加载到编辑器中
 * @param {{ sql_text: string; created_at: string }} item - 历史记录项
 */
const handleHistorySelect = (item: { sql_text: string; created_at: string }) => {
  /** 条件分支 */
  if (!editorView) return;
  editorView.dispatch({
    changes: {
      from: 0,
      to: editorView.state.doc.length,
      insert: item.sql_text
    }
  });
  editorView.focus();
};

/**
 * 将 ISO 日期字符串格式化为 zh-CN 本地化显示（如 "10/28, 14:30"）
 * @param {string} dateStr - ISO 日期字符串
 * @returns {string} 格式化后的日期字符串，空输入返回空字符串
 */
const formatDateTime = (dateStr: string) => {
  /** 条件分支 */
  if (!dateStr) return '';
  /** date: 变量/函数声明 */
  const date = new Date(dateStr);
  /** 返回/抛出 */
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  });
};

/**
 * 切换数据源：加载数据库列表、重置表和字段选择、更新写权限状态
 * 支持 sqlite/rqlite 直接使用 "main" 数据库
 * @returns {Promise<void>}
 */
const handleDatasourceChange = async () => {
  databases.value = [];
  selectedDatabase.value = '';
  tables.value = [];
  selectedTable.value = '';
  selectedTableColumns.value = [];
  autocompleteData.value.tables = [];
  autocompleteData.value.columns = [];

  /** 条件分支 */
  if (!selectedDatasourceId.value) {
    allowWriteSQL.value = false;
    return;
  }

  /** ds: 变量/函数声明 */
  const ds = datasources.value.find(d => d.id === selectedDatasourceId.value);
  /** 条件分支 */
  if (!ds) {
    allowWriteSQL.value = false;
    return;
  }

  allowWriteSQL.value = ds.allow_write_sql || false;

  /** 条件分支 */
  if (ds.type === 'sqlite' || ds.type === 'rqlite') {
    databases.value = ['main'];
    selectedDatabase.value = 'main';
    await handleDatabaseChange();
    return;
  }

  /** cacheKey: 变量/函数声明 */
  const cacheKey = `databases_${selectedDatasourceId.value}`;
  /** cached: 变量/函数声明 */
  const cached = getCachedMetadata(cacheKey);
  /** 条件分支 */
  if (cached) {
    databases.value = cached;
    /** 条件分支 */
    if (cached.length === 1) {
      selectedDatabase.value = cached[0];
      await handleDatabaseChange();
    }
    return;
  }

  loadingDatabases.value = true;
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.getDatabases(selectedDatasourceId.value as number, getMetadataSignal());
    /** dbList 可变变量 */
    let dbList = res.data || [];
    /** 条件分支 */
    if (dbList.length === 0) {
      dbList = ds.database ? [ds.database] : ['default'];
    }
    databases.value = dbList;
    setCachedMetadata(cacheKey, dbList);
    /** 条件分支 */
    if (dbList.length === 1) {
      selectedDatabase.value = dbList[0];
      await handleDatabaseChange();
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isCanceledError(err) && !isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(`获取数据库列表失败: ${msg}`);
    }
    databases.value = ds.database ? [ds.database] : ['default'];
  } finally {
    loadingDatabases.value = false;
  }
};

/**
 * 切换数据库：加载表列表（带缓存），更新自动补全数据源
 * @returns {Promise<void>}
 */
const handleDatabaseChange = async () => {
  tables.value = [];
  selectedTable.value = '';
  selectedTableColumns.value = [];
  autocompleteData.value.tables = [];
  autocompleteData.value.columns = [];

  /** 条件分支 */
  if (!selectedDatasourceId.value) return;

  /** dbName: 变量/函数声明 */
  const dbName = selectedDatabase.value || 'default';

  /** cacheKey: 变量/函数声明 */
  const cacheKey = `tables_${selectedDatasourceId.value}_${dbName}`;
  /** cached: 变量/函数声明 */
  const cached = getCachedMetadata(cacheKey);
  /** 条件分支 */
  if (cached) {
    tables.value = cached;
    autocompleteData.value.tables = cached.map((t: any) => t.name || '').filter(Boolean).slice(0, 500);
    return;
  }

  loadingTables.value = true;
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.getTables(selectedDatasourceId.value as number, dbName, getMetadataSignal());
    /** tableData: 变量/函数声明 */
    const tableData = res.data || [];
    tables.value = tableData;
    setCachedMetadata(cacheKey, tableData);
    autocompleteData.value.tables = tableData.map((t: any) => t.name || '').filter(Boolean).slice(0, 500);
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isCanceledError(err) && !isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(`获取数据表列表失败: ${msg}`);
    }
  } finally {
    loadingTables.value = false;
  }
};

/** datasourceIdOnOpen: 可变变量 */
let datasourceIdOnOpen: number | '' = '';
/** databaseOnOpen 可变变量 */
let databaseOnOpen = '';

/**
 * 数据源下拉框可见性变化：关闭时若选择未变则刷新元数据（探测后端新增）
 * @param {boolean} visible - 下拉框是否可见
 */
const onDatasourceVisibleChange = (visible: boolean) => {
  /** 条件分支 */
  if (visible) {
    datasourceIdOnOpen = selectedDatasourceId.value;
  } else if (selectedDatasourceId.value && selectedDatasourceId.value === datasourceIdOnOpen) {
    refreshDatasourceMetadata();
  }
};

/**
 * 数据库下拉框可见性变化：关闭时若选择未变则刷新表列表
 * @param {boolean} visible - 下拉框是否可见
 */
const onDatabaseVisibleChange = (visible: boolean) => {
  /** 条件分支 */
  if (visible) {
    databaseOnOpen = selectedDatabase.value;
  } else if (selectedDatabase.value && selectedDatabase.value === databaseOnOpen) {
    refreshDatabaseMetadata();
  }
};

/**
 * 强制刷新当前数据源的数据库列表（绕过缓存）
 * @returns {Promise<void>}
 */
const refreshDatasourceMetadata = async () => {
  /** 条件分支 */
  if (!selectedDatasourceId.value) return;
  /** ds: 变量/函数声明 */
  const ds = datasources.value.find(d => d.id === selectedDatasourceId.value);
  /** 条件分支 */
  if (!ds) return;

  /** 条件分支 */
  if (ds.type === 'sqlite' || ds.type === 'rqlite') {
    /** 条件分支 */
    if (selectedDatabase.value) {
      await refreshDatabaseMetadata();
    }
    return;
  }

  loadingDatabases.value = true;
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.getDatabases(selectedDatasourceId.value as number, getMetadataSignal());
    /** dbList 可变变量 */
    let dbList = res.data || [];
    /** 条件分支 */
    if (dbList.length === 0) {
      dbList = ds.database ? [ds.database] : ['default'];
    }
    databases.value = dbList;
    setCachedMetadata(`databases_${selectedDatasourceId.value}`, dbList);
    /** 条件分支 */
    if (selectedDatabase.value) {
      await refreshDatabaseMetadata();
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isCanceledError(err) && !isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(`获取数据库列表失败: ${msg}`);
    }
  } finally {
    loadingDatabases.value = false;
  }
};

/**
 * 强制刷新当前数据库的表列表（绕过缓存）
 * @returns {Promise<void>}
 */
const refreshDatabaseMetadata = async () => {
  /** 条件分支 */
  if (!selectedDatasourceId.value || !selectedDatabase.value) return;

  /** dbName: 变量/函数声明 */
  const dbName = selectedDatabase.value;
  loadingTables.value = true;
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.getTables(selectedDatasourceId.value as number, dbName, getMetadataSignal());
    /** tableData: 变量/函数声明 */
    const tableData = res.data || [];
    tables.value = tableData;
    setCachedMetadata(`tables_${selectedDatasourceId.value}_${dbName}`, tableData);
    autocompleteData.value.tables = tableData.map((t: any) => t.name || '').filter(Boolean).slice(0, 500);
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isCanceledError(err) && !isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(`获取数据表列表失败: ${msg}`);
    }
  } finally {
    loadingTables.value = false;
  }
};

/**
 * 切换表：加载字段列表（带缓存），更新自动补全数据源
 * @returns {Promise<void>}
 */
const handleTableChange = async () => {
  /** 条件分支 */
  if (!selectedTable.value) {
    selectedTableColumns.value = [];
    autocompleteData.value.columns = [];
    return;
  }

  /** dbName: 变量/函数声明 */
  const dbName = selectedDatabase.value || 'default';

  /** cacheKey: 变量/函数声明 */
  const cacheKey = `columns_${selectedDatasourceId.value}_${dbName}_${selectedTable.value}`;
  /** cached: 变量/函数声明 */
  const cached = getCachedMetadata(cacheKey);
  /** 条件分支 */
  if (cached) {
    selectedTableColumns.value = cached;
    autocompleteData.value.columns = cached.map((c: any) => c.name || '').filter(Boolean);
    return;
  }

  loadingColumns.value = true;
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await datasourceAPI.getColumns(selectedDatasourceId.value as number, dbName, selectedTable.value, getMetadataSignal());
    selectedTableColumns.value = res.data || [];
    setCachedMetadata(cacheKey, res.data || []);
    autocompleteData.value.columns = (res.data || []).map((c: any) => c.name || '').filter(Boolean);
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isCanceledError(err) && !isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(`获取字段列表失败: ${msg}`);
    }
    selectedTableColumns.value = [];
    autocompleteData.value.columns = [];
  } finally {
    loadingColumns.value = false;
  }
};

/**
 * 清除指定 key 的元数据缓存（同时清除内存和 localStorage）
 * @param {string} key - 缓存键
 */
const clearMetadataCache = (key: string) => {
  metadataCache.delete(key);
  /** 异常处理 */
  try {
    localStorage.removeItem(getStorageCacheKey(key));
  } catch {
    // localStorage 可能被禁用，静默忽略删除失败
  }
};

/**
 * 刷新当前数据源的元数据：清除缓存后根据当前选择重新加载
 * @returns {Promise<void>}
 */
const refreshMetadata = async () => {
  /** 条件分支 */
  if (!selectedDatasourceId.value) return;

  clearMetadataCache(`databases_${selectedDatasourceId.value}`);
  /** 条件分支 */
  if (selectedDatabase.value) {
    clearMetadataCache(`tables_${selectedDatasourceId.value}_${selectedDatabase.value}`);
  }
  /** 条件分支 */
  if (selectedTable.value) {
    clearMetadataCache(`columns_${selectedDatasourceId.value}_${selectedDatabase.value}_${selectedTable.value}`);
  }

  /** 条件分支 */
  if (selectedTable.value) {
    await handleTableChange();
  } else if (selectedDatabase.value) {
    tables.value = [];
    selectedTable.value = '';
    selectedTableColumns.value = [];
    autocompleteData.value.tables = [];
    autocompleteData.value.columns = [];
    await handleDatabaseChange();
  } else {
    databases.value = [];
    tables.value = [];
    selectedTable.value = '';
    selectedTableColumns.value = [];
    autocompleteData.value.tables = [];
    autocompleteData.value.columns = [];
    await handleDatasourceChange();
  }
};

/**
 * 清除当前数据源的后端缓存（调用 datasourceAPI.clearCache）同时清除前端元数据缓存
 * @returns {Promise<void>}
 */
const handleClearCache = async () => {
  /** 条件分支 */
  if (!selectedDatasourceId.value) return;

  clearingCache.value = true;
  /** 异常处理 */
  try {
    await datasourceAPI.clearCache(selectedDatasourceId.value as number);

    // 同时清除前端元数据缓存
    clearMetadataCache(`databases_${selectedDatasourceId.value}`);
    /** 条件分支 */
    if (selectedDatabase.value) {
      clearMetadataCache(`tables_${selectedDatasourceId.value}_${selectedDatabase.value}`);
    }
    /** 条件分支 */
    if (selectedTable.value) {
      clearMetadataCache(`columns_${selectedDatasourceId.value}_${selectedDatabase.value}_${selectedTable.value}`);
    }

    ElMessage.success('缓存已清除，下次查询将获取最新数据');
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(msg);
    }
  } finally {
    clearingCache.value = false;
  }
};

/**
 * 在编辑器光标位置插入选中的字段名（用反引号包裹）
 * @param {{ name: string }} column - 字段对象
 */
const insertColumn = (column: { name: string }) => {
  /** 条件分支 */
  if (!editorView) return;
  /** cursor: 变量/函数声明 */
  const cursor = editorView.state.selection.main.head;
  editorView.dispatch({
    changes: {
      from: cursor,
      insert: `\`${column.name}\``
    }
  });
  editorView.focus();
};

/**
 * 在编辑器中插入 SELECT * FROM `表名` 模板，覆盖当前编辑器全部内容
 * 要求已选中表，否则静默跳过
 * @returns {void}
 */
const insertSelectAll = () => {
  /** 条件分支 */
  if (!selectedTable.value || !editorView) return;
  /** sql: 变量/函数声明 */
  const sql = `SELECT * FROM \`${selectedTable.value}\`\n`;
  editorView.dispatch({
    changes: {
      from: 0,
      to: editorView.state.doc.length,
      insert: sql
    }
  });
  editorView.focus();
};

/**
 * 在编辑器中插入 SELECT 字段列表 FROM `表名` 模板，所有字段用反引号包裹
 * 覆盖当前编辑器全部内容；要求已选中表且有字段列表
 * @returns {void}
 */
const insertSelectColumns = () => {
  /** 条件分支 */
  if (!selectedTable.value || !editorView || selectedTableColumns.value.length === 0) return;
  /** columns: 变量/函数声明 */
  const columns = selectedTableColumns.value.map(c => `\`${c.name}\``).join(', ');
  /** sql: 变量/函数声明 */
  const sql = `SELECT ${columns} FROM \`${selectedTable.value}\`\n`;
  editorView.dispatch({
    changes: {
      from: 0,
      to: editorView.state.doc.length,
      insert: sql
    }
  });
  editorView.focus();
};

/**
 * 在编辑器中插入 SELECT COUNT(*) FROM `表名` 模板，覆盖当前编辑器全部内容
 * 要求已选中表，否则静默跳过
 * @returns {void}
 */
const insertCount = () => {
  /** 条件分支 */
  if (!selectedTable.value || !editorView) return;
  /** sql: 变量/函数声明 */
  const sql = `SELECT COUNT(*) FROM \`${selectedTable.value}\`\n`;
  editorView.dispatch({
    changes: {
      from: 0,
      to: editorView.state.doc.length,
      insert: sql
    }
  });
  editorView.focus();
};

/**
 * 清空 CodeMirror 编辑器中的全部 SQL 文本并聚焦，用于重新编写新查询
 * @returns {void}
 */
const clearEditor = () => {
  /** 条件分支 */
  if (!editorView) return;
  editorView.dispatch({
    changes: {
      from: 0,
      to: editorView.state.doc.length,
      insert: ''
    }
  });
  editorView.focus();
};

/**
 * 使用 sql-formatter 库格式化当前编辑器中的 SQL 文本
 * 格式化失败（如语法错误）时提示用户 "SQL语法错误，无法格式化"
 * @returns {void}
 */
const handleFormat = () => {
  /** 条件分支 */
  if (!sqlText.value || !editorView) return;
  /** 异常处理 */
  try {
    /** formatted: 变量/函数声明 */
    const formatted = format(sqlText.value, { language: 'sql' });
    editorView.dispatch({
      changes: {
        from: 0,
        to: editorView.state.doc.length,
        insert: formatted + '\n'
      }
    });
    ElMessage.success('SQL格式化成功');
  } catch (err) {
    ElMessage.error('SQL语法错误，无法格式化');
  }
};

/**
 * 获取要执行的 SQL：有选中内容时返回选中内容，否则返回完整 SQL
 * @returns {string} 要执行的 SQL 文本
 */
const getSQLToExecute = (): string => {
  /** 条件分支 */
  if (!editorView) return sqlText.value;

  /** selection: 变量/函数声明 */
  const selection = editorView.state.selection.main;
  /** 条件分支 */
  if (selection.from !== selection.to) {
    /** 返回/抛出 */
    return editorView.state.doc.sliceString(selection.from, selection.to);
  }

  /** 返回/抛出 */
  return sqlText.value;
};

// ========== 查询执行与轮询 ==========

/**
 * 停止指定标签页的查询结果轮询，清除定时器并关闭 SSE 连接
 * 未指定 tabId 时默认操作当前活跃标签
 * @param {string} [tabId] - 目标标签页 ID，缺省使用 activeTabId
 * @returns {void}
 */
const stopPolling = (tabId?: string) => {
  /** id: 变量/函数声明 */
  const id = tabId || activeTabId.value;
  /** timer: 变量/函数声明 */
  const timer = pollTimers.get(id);
  /** 条件分支 */
  if (timer) {
    clearInterval(timer);
    pollTimers.delete(id);
  }
  /** es: 变量/函数声明 */
  const es = sseConnections.get(id);
  /** 条件分支 */
  if (es) {
    es.close();
    sseConnections.delete(id);
  }
};

/**
 * 停止所有标签页的查询轮询定时器和 SSE 连接，组件卸载时调用
 * 防止后台残留定时器持续消耗资源
 * @returns {void}
 */
const stopAllPolling = () => {
  /** 循环 */
  for (const [_id, timer] of pollTimers) {
    clearInterval(timer);
  }
  pollTimers.clear();
  /** 循环 */
  for (const [_id, es] of sseConnections) {
    es.close();
  }
  sseConnections.clear();
};

/**
 * 启动查询结果轮询，每秒通过 queryAPI.getResult 拉取一次查询状态
 * 适合 SSE 不可用时的降级方案，查询完成/失败/取消后自动停止
 * @param {string} tabId - 查询所属的标签页 ID
 * @param {string} queryId - 后端返回的查询 ID
 * @returns {void}
 */
const startPolling = (tabId: string, queryId: string) => {
  /** tab: 变量/函数声明 */
  const tab = tabs.value.find(t => t.id === tabId);
  /** 条件分支 */
  if (!tab) return;

  /** timer: 变量/函数声明 */
  const timer = setInterval(async () => {
    /** 异常处理 */
    try {
      /** pollRes: 变量/函数声明 */
      const pollRes = await queryAPI.getResult(queryId);
      /** pollData: 变量/函数声明 */
      const pollData = (pollRes as any).data as any;

      /** 条件分支 */
      if (pollData.status === 'completed') {
        stopPolling(tabId);
        tab.queryResult = pollData;
        tab.executing = false;
        loadRecentHistory();
      } else if (pollData.status === 'failed') {
        stopPolling(tabId);
        tab.errorMessage = pollData.error || '查询失败';
        tab.queryResult = null;
        tab.executing = false;
      } else if (pollData.status === 'cancelled') {
        stopPolling(tabId);
        tab.errorMessage = pollData.error || '查询已取消';
        tab.queryResult = null;
        tab.executing = false;
      }
    } catch (err: unknown) {
      stopPolling(tabId);
      tab.errorMessage = formatErrorMessage(err);
      tab.queryResult = null;
      tab.executing = false;
    }
  }, 1000);
  pollTimers.set(tabId, timer);
};

/**
 * 执行当前编辑器中的 SQL 查询
 * 流程：发起查询 → 获取 queryId → 优先使用 SSE 推送结果 → 降级到轮询
 * 支持同步短查询直接返回结果，长查询异步轮询/SSE 推送
 * 快捷键：Ctrl/Cmd + Enter
 * @returns {Promise<void>}
 */
const handleExecute = async () => {
  /** sql: 变量/函数声明 */
  const sql = getSQLToExecute();
  /** tab: 变量/函数声明 */
  const tab = activeTab.value;
  /** 条件分支 */
  if (!sql.trim() || !selectedDatasourceId.value || tab.executing) return;

  stopPolling(tab.id);
  cancelPendingMetadataRequests();
  tab.executing = true;
  tab.errorMessage = '';
  tab.queryId = '';
  tab.queryResult = null;

  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await queryAPI.execute({
      datasource_id: selectedDatasourceId.value as number,
      sql: sql,
      database: selectedDatabase.value
    });

    /** data: 变量/函数声明 */
    const data = res.data as any;
    /** queryId: 变量/函数声明 */
    const queryId = data.query_id;
    /** status: 变量/函数声明 */
    const status = data.status;

    /** 条件分支 */
    if (status === 'completed') {
      tab.queryResult = data;
      tab.queryId = queryId;
      tab.executing = false;
      loadRecentHistory();
      return;
    }

    tab.queryId = queryId;

    // 优先使用 SSE 推送，降级到轮询
    try {
      /** eventSource: 变量/函数声明 */
      const eventSource = queryAPI.streamResult(queryId);
      sseConnections.set(tab.id, eventSource);

      eventSource.addEventListener('query_update', (event: MessageEvent) => {
        /** 异常处理 */
        try {
          /** data: 变量/函数声明 */
          const data = JSON.parse(event.data);
          /** 条件分支 */
          if (data.status === 'completed') {
            stopPolling(tab.id);
            tab.queryResult = data;
            tab.executing = false;
            loadRecentHistory();
            eventSource.close();
          } else if (data.status === 'failed') {
            stopPolling(tab.id);
            tab.errorMessage = data.error || '查询失败';
            tab.queryResult = null;
            tab.executing = false;
            eventSource.close();
          } else if (data.status === 'cancelled') {
            stopPolling(tab.id);
            tab.errorMessage = data.error || '查询已取消';
            tab.queryResult = null;
            tab.executing = false;
            eventSource.close();
          }
        } catch (parseErr) {
          // JSON 解析失败，忽略
        }
      });

      // 监听后端发送的 error 事件（如查询不存在或已过期）
      eventSource.addEventListener('error', (event: MessageEvent) => {
        /** 异常处理 */
        try {
          /** data: 变量/函数声明 */
          const data = JSON.parse(event.data);
          /** 条件分支 */
          if (data.error) {
            stopPolling(tab.id);
            tab.errorMessage = data.error;
            tab.queryResult = null;
            tab.executing = false;
            eventSource.close();
          }
        } catch (parseErr) {
          // JSON 解析失败，忽略
        }
      });

      eventSource.onerror = () => {
        // SSE 连接失败，降级到轮询
        eventSource.close();
        sseConnections.delete(tab.id);
        startPolling(tab.id, queryId);
      };
    } catch {
      // SSE 不可用，降级到轮询
      startPolling(tab.id, queryId);
    }
  } catch (err: unknown) {
    tab.errorMessage = formatErrorMessage(err);
    tab.queryResult = null;
    tab.executing = false;
  }
};

/**
 * 取消当前标签页正在执行的查询，调用后端 queryAPI.cancel 接口终止查询
 * 成功后清理轮询/SSE 状态，将查询结果置为 null，提示 "查询已取消"
 * @returns {Promise<void>}
 */
const handleCancel = async () => {
  /** tab: 变量/函数声明 */
  const tab = activeTab.value;
  /** 条件分支 */
  if (!tab.queryId || tab.canceling) return;

  tab.canceling = true;
  /** 异常处理 */
  try {
    await queryAPI.cancel(tab.queryId);
    stopPolling(tab.id);
    tab.errorMessage = '查询已取消';
    tab.queryResult = null;
    ElMessage.info('查询已取消');
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      /** msg: 变量/函数声明 */
      const msg = formatErrorMessage(err);
      ElMessage.error(msg);
    }
  } finally {
    tab.canceling = false;
    tab.executing = false;
    tab.queryId = '';
  }
};

// ========== SQL 保存与导出 ==========

/**
 * 打开保存 SQL 对话框，加载当前用户已保存的 SQL 列表用于覆盖模式判断
 * 根据现有已保存名称列表决定 saveMode 是新建还是覆盖
 * @returns {Promise<void>}
 */
const handleSaveSQL = async () => {
  saveMode.value = 'new';
  saveForm.value = {
    name: '',
    description: '',
    is_public: false
  };
  /** 异常处理 */
  try {
    /** res: 变量/函数声明 */
    const res = await queryAPI.listSavedSQL({ page: 1, page_size: 200 });
    savedSQLNameList.value = res.data.items || [];
  } catch (err) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error('加载已保存SQL列表失败');
    }
  }
  saveDialogVisible.value = true;
};

/**
 * 保存 SQL 确认按钮：校验表单后根据 saveMode 调用新建或覆盖接口
 * @returns {Promise<void>}
 */
const handleConfirmSave = async () => {
  /** 条件分支 */
  if (!saveFormRef.value) return;

  /** valid: 变量/函数声明 */
  const valid = await saveFormRef.value.validate();
  /** 条件分支 */
  if (!valid) return;

  /** dsId: 变量/函数声明 */
  const dsId = Number(selectedDatasourceId.value);
  /** 条件分支 */
  if (!dsId || isNaN(dsId)) {
    ElMessage.error('请选择数据源');
    return;
  }

  saving.value = true;
  /** 异常处理 */
  try {
    /** 条件分支 */
    if (saveMode.value === 'overwrite') {
      // 覆盖已有记录
      const existing = savedSQLNameList.value.find(item => item.name === saveForm.value.name);
      /** 条件分支 */
      if (!existing) {
        ElMessage.error('未找到要覆盖的记录');
        return;
      }
      await queryAPI.updateSavedSQL(existing.id, {
        name: saveForm.value.name,
        datasource_id: dsId,
        database: selectedDatabase.value,
        sql_text: sqlText.value,
        description: saveForm.value.description,
        is_public: saveForm.value.is_public
      });
      ElMessage.success('已覆盖保存');
    } else {
      // 新建记录
      await queryAPI.saveSQL({
        name: saveForm.value.name,
        datasource_id: dsId,
        database: selectedDatabase.value,
        sql_text: sqlText.value,
        description: saveForm.value.description,
        is_public: saveForm.value.is_public
      });
      ElMessage.success('保存成功');
    }
    saveDialogVisible.value = false;
    saveForm.value = {
      name: '',
      description: '',
      is_public: false
    };
    saveMode.value = 'new';
  } catch (err) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error('保存失败');
    }
  } finally {
    saving.value = false;
  }
};

/**
 * 将当前查询结果（虚拟表格展示的 columns + rows）导出为 CSV 文件下载
 * 支持大数据量流式下载，显示导出进度百分比
 * @returns {Promise<void>}
 */
const handleExportCSV = async () => {
  /** 条件分支 */
  if (!queryResult.value || !selectedDatasourceId.value) return;

  /** 异常处理 */
  try {
    exporting.value = true;
    /** res: 变量/函数声明 */
    const res = await queryAPI.exportCSV({
      datasource_id: selectedDatasourceId.value,
      sql: sqlText.value,
      database: selectedDatabase.value || undefined,
    }, (event: any) => {
      /** 条件分支 */
      if (event.lengthComputable) {
        exportProgress.value = Math.round((event.loaded / event.total) * 100);
      }
    });

    /** blob: 变量/函数声明 */
    const blob = new Blob([res.data as any], { type: 'text/csv;charset=utf-8;' });
    /** link: 变量/函数声明 */
    const link = document.createElement('a');
    /** url: 变量/函数声明 */
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `query_result_${Date.now()}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      /** 条件分支 */
      const resp = getErrorResponse(err);
      const data = resp?.data;
      if (data instanceof Blob) {
        /** text: 变量/函数声明 */
        const text = await data.text();
        /** 异常处理 */
        try {
          /** json: 变量/函数声明 */
          const json = JSON.parse(text);
          ElMessage.error(json.message || '导出失败');
        } catch {
          ElMessage.error('导出失败');
        }
      } else {
        ElMessage.error(formatErrorMessage(err));
      }
    }
  } finally {
    exporting.value = false;
    exportProgress.value = 0;
  }
};

/**
 * 将秒数格式化为可读时间（<1s 显示 ms，<60s 显示 s，≥60s 显示 m s）
 * @param {number} [seconds] - 秒数
 * @returns {string} 格式化后的时间字符串，未传入时返回 "-"
 */
const formatTime = (seconds?: number) => {
  /** 条件分支 */
  if (!seconds && seconds !== 0) return '-';
  /** 条件分支 */
  if (seconds < 1) {
    /** 返回/抛出 */
    return `${(seconds * 1000).toFixed(0)}ms`;
  } else if (seconds < 60) {
    /** 返回/抛出 */
    return `${seconds.toFixed(2)}s`;
  } else {
    /** minutes: 变量/函数声明 */
    const minutes = Math.floor(seconds / 60);
    /** remainingSeconds: 变量/函数声明 */
    const remainingSeconds = (seconds % 60).toFixed(2);
    /** 返回/抛出 */
    return `${minutes}m ${remainingSeconds}s`;
  }
};

/**
 * 从路由 query 参数初始化页面状态，支持跨页面跳转预设数据源和 SQL
 * 可通过路由参数 datasource_id / database / sql 自动填充查询面板
 * @returns {void}
 */
const initFromRoute = () => {
  /** dsId: 变量/函数声明 */
  const dsId = route.query?.datasource_id as string;
  /** sql: 变量/函数声明 */
  const sql = route.query?.sql as string;
  /** dbName: 变量/函数声明 */
  const dbName = route.query?.database as string;
  /** 条件分支 */
  if (dsId != null && dsId !== '') {
    /** parsedId: 变量/函数声明 */
    const parsedId = parseInt(dsId);
    /** 条件分支 */
    if (!isNaN(parsedId)) {
      selectedDatasourceId.value = parsedId;
      // 若路由携带 database 参数，传入 handleDatasourceChangeForTab 以便加载数据库列表后自动选中
      handleDatasourceChangeForTab(parsedId, dbName || '');
    }
  }
  /** 条件分支 */
  if (sql && editorView) {
    editorView.dispatch({
      changes: {
        from: 0,
        to: editorView.state.doc.length,
        insert: sql
      }
    });
  }
};

// ========== 生命周期 ==========

/** 组件挂载：加载数据源列表、最近历史、初始化编辑器、处理路由参数 */
onMounted(async () => {
  await loadDatasources();
  loadRecentHistory();
  createEditor();

  initFromRoute();
});

/** 组件卸载：同步标签数据、停止轮询、销毁编辑器、清理事件监听 */
onUnmounted(() => {
  syncCurrentTabData();
  stopAllPolling();
  editorView?.destroy();
  document.removeEventListener('mousemove', onResize);
  document.removeEventListener('mouseup', stopResize);
  document.removeEventListener('mousemove', onColResize);
  document.removeEventListener('mouseup', stopColResize);
  window.removeEventListener('beforeunload', handleBeforeUnload);
});

/**
 * 浏览器页面刷新/关闭前同步标签页状态到 localStorage
 * 通过 window.addEventListener('beforeunload') 注册，防止未保存的 SQL 丢失
 * @returns {void}
 */
const handleBeforeUnload = () => {
  syncCurrentTabData();
};
window.addEventListener('beforeunload', handleBeforeUnload);
</script>

<!-- ========== 样式 ========== -->
<style scoped>
/* === 页面整体布局容器，使用 flex 横向排列元数据面板和查询主区域 === */
/* === .sql-query-page 样式定义 === */
.sql-query-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  margin: calc(-1 * var(--space-6));
}

/* === 左右两栏主容器：元数据面板(左) + 查询区域(右) === */
/* === 主内容区 flex 布局（任务区 + 日志面板） === */
.main-content {
  display: flex;
  flex: 1;
  gap: var(--space-4);
  min-height: 0;
  align-items: stretch;
}

/* === .query-area 样式定义 === */
.query-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  min-width: 0;
  min-height: 0;
}

/* === 左侧元数据面板：数据源选择 + 表字段浏览 === */
/* === .metadata-panel 样式定义 === */
.metadata-panel {
  width: 300px;
  min-width: 220px;
  flex-shrink: 1;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  align-self: stretch;
}

/* === 指令/媒体查询 === */
@media (max-width: 1200px) {
  /* === .metadata-panel 样式定义 === */
  .metadata-panel {
    width: 240px;
    min-width: 200px;
  }
}

/* === 指令/媒体查询 === */
@media (max-width: 900px) {
  /* === .metadata-panel 样式定义 === */
  .metadata-panel {
    width: 200px;
    min-width: 160px;
  }
}

/* === .panel-header 样式定义 === */
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

/* === .panel-title 样式定义 === */
.panel-title {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-weight: 600;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

/* === .panel-actions 样式定义 === */
.panel-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

/* === .panel-section 样式定义 === */
.panel-section {
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

/* === .selector-section 样式定义 === */
.selector-section {
  padding-bottom: var(--space-3);
}

/* === .collapsed 样式定义 === */
.selector-section.collapsed {
  padding-bottom: var(--space-3);
}

/* === .selector-toggle 样式定义 === */
.selector-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

/* === .selector-toggle 样式定义 === */
.selector-toggle:hover {
  color: var(--accent-primary);
}

/* === .selector-fields 样式定义 === */
.selector-fields {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding-top: var(--space-3);
}

/* === .selector-field 样式定义 === */
.selector-field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

/* === .field-label 样式定义 === */
.field-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
}

/* === .section-label 样式定义 === */
.section-label {
  display: block;
  font-size: var(--font-size-sm);
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: var(--space-2);
}

/* === .field-select 样式定义 === */
.field-select {
  width: 100%;
}

/* === .type-tag 样式定义 === */
.type-tag {
  margin-left: var(--space-2);
  font-size: 12px;
}

/* === 数据源类型标签颜色（每个类型配独立颜色便于区分） === */
.tag-mysql { background: rgba(250, 146, 58, 0.15); color: #fa923a; }
.tag-hive { background: rgba(232, 121, 249, 0.15); color: #e879f9; }
.tag-trino { background: rgba(139, 92, 246, 0.15); color: #8b5cf6; }
.tag-spark { background: rgba(255, 152, 0, 0.15); color: #ff9800; }
.tag-kyuubi { background: rgba(6, 182, 212, 0.15); color: #06b6d4; }
.tag-starrocks { background: rgba(52, 211, 153, 0.15); color: #34d399; }
.tag-doris { background: rgba(236, 72, 153, 0.15); color: #ec4899; }
.tag-sqlite { background: rgba(107, 114, 128, 0.15); color: #6b7280; }
.tag-rqlite { background: rgba(34, 197, 94, 0.15); color: #22c55e; }
.tag-redis { background: rgba(239, 68, 68, 0.15); color: #ef4444; }

/* === .columns-section 样式定义 === */
.columns-section {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 200px;
}

/* === .columns-loading 样式定义 === */
.columns-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  padding: var(--space-4);
  color: var(--text-muted);
  font-size: var(--font-size-sm);
}

/* === .columns-list 样式定义 === */
.columns-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-height: 0;
}

/* === .column-item 样式定义 === */
.column-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--bg-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--duration-fast);
}

/* === .column-item 样式定义 === */
.column-item:hover {
  background: var(--bg-hover);
  border-color: var(--border-primary);
}

/* === .column-icon 样式定义 === */
.column-icon {
  color: var(--accent-primary);
  flex-shrink: 0;
}

/* === .column-name 样式定义 === */
.column-name {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

/* === .column-type 样式定义 === */
.column-type {
  font-size: 0.75rem;
  color: var(--text-muted);
  background: var(--bg-card);
  padding: 2px var(--space-2);
  border-radius: var(--radius-xs);
  flex-shrink: 0;
}

/* === .empty-state 样式定义 === */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  color: var(--text-muted);
}

/* === .empty-state 样式定义 === */
.empty-state p {
  margin-top: var(--space-2);
  font-size: var(--font-size-sm);
}

/* === .query-area 样式定义 === */
.query-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  min-width: 0;
  min-height: 0;
}

/* === .query-toolbar 样式定义 === */
.query-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

.toolbar-left,
/* === 工具栏右侧（刷新 + 创建） === */
.toolbar-right {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}

/* === 工具栏左侧（搜索 + 筛选） === */
.toolbar-left .el-divider--vertical {
  height: 1.2em;
  align-self: center;
}

/* === 刷新按钮 === */
.refresh-btn {
  font-weight: 500;
  font-size: var(--btn-font-size);
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
  padding: var(--btn-padding-y) var(--btn-padding-x);
}

/* === 刷新按钮 === */
.refresh-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

/* === 刷新按钮 === */
.refresh-btn:disabled {
  opacity: 0.5;
  transform: none;
  box-shadow: none;
}

/* === 创建主按钮（渐变蓝） === */
.create-btn {
  font-weight: 500;
  font-size: var(--btn-font-size);
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-secondary) 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
  padding: var(--btn-padding-y) var(--btn-padding-x-lg);
}

/* === 创建主按钮（渐变蓝） === */
.create-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

/* === .cancel-btn 样式定义 === */
.cancel-btn {
  font-weight: 500;
  font-size: var(--btn-font-size);
  background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%);
  border: none;
  color: white;
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.3);
  transition: all var(--duration-normal) var(--ease-out);
  padding: var(--btn-padding-y) var(--btn-padding-x);
}

/* === .cancel-btn 样式定义 === */
.cancel-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(239, 68, 68, 0.4);
  filter: brightness(1.05);
}

/* === .cancel-btn 样式定义 === */
.cancel-btn:disabled {
  opacity: 0.5;
  transform: none;
  box-shadow: none;
}

/* === .dml-warning 样式定义 === */
.dml-warning {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--btn-font-size);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-md);
}

/* === .editor-container 样式定义 === */
.editor-container {
  position: relative;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  flex-shrink: 0;
}

/* === .editor-header 样式定义 === */
.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

/* === .tab-bar 样式定义 === */
.tab-bar {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

/* === .tabs-scroll 样式定义 === */
.tabs-scroll {
  display: flex;
  align-items: center;
  overflow-x: auto;
  flex: 1;
  min-width: 0;
  scrollbar-width: none;
}

/* === .tabs-scroll 样式定义 === */
.tabs-scroll::-webkit-scrollbar {
  display: none;
}

/* === .tab-item 样式定义 === */
.tab-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--btn-padding-y) var(--space-3);
  font-size: var(--btn-font-size);
  color: var(--text-secondary);
  cursor: pointer;
  border-right: 1px solid var(--border-subtle);
  white-space: nowrap;
  transition: all var(--duration-fast);
  position: relative;
  user-select: none;
}

/* === .tab-item 样式定义 === */
.tab-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

/* === .active 样式定义 === */
.tab-item.active {
  color: var(--accent-primary);
  background: var(--bg-card);
  font-weight: 500;
}

/* === .active 样式定义 === */
.tab-item.active::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--accent-primary);
}

/* === .tab-name 样式定义 === */
.tab-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* === .tab-spinner 样式定义 === */
.tab-spinner {
  color: var(--accent-primary);
  flex-shrink: 0;
  animation: spin 1s linear infinite;
}

/* === @keyframes spin { === */
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* === .tab-close 样式定义 === */
.tab-close {
  color: var(--text-muted);
  border-radius: 50%;
  padding: 2px;
  transition: all var(--duration-fast);
  flex-shrink: 0;
}

/* === .tab-close 样式定义 === */
.tab-close:hover {
  color: var(--accent-danger);
  background: rgba(239, 68, 68, 0.1);
}

/* === .tab-add-btn 样式定义 === */
.tab-add-btn {
  color: var(--text-muted);
  padding: var(--btn-padding-y) var(--space-2);
  flex-shrink: 0;
  transition: color var(--duration-fast);
}

/* === .tab-add-btn 样式定义 === */
.tab-add-btn:hover {
  color: var(--accent-primary);
}

/* === .editor-meta 样式定义 === */
.editor-meta {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding-right: var(--space-4);
  flex-shrink: 0;
}

/* === .sql-length 样式定义 === */
.sql-length {
  font-size: var(--font-size-xs);
  color: var(--text-muted);
  font-family: var(--font-mono);
}

/* === .editor-wrapper 样式定义 === */
.editor-wrapper {
  position: relative;
  background: #282c34;
}

/* === .sql-editor 样式定义 === */
.sql-editor {
  height: 100%;
}

/* === .resize-handle 样式定义 === */
.resize-handle {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 100%;
  height: 6px;
  cursor: s-resize;
  z-index: 10;
}

/* === .resize-handle 样式定义 === */
.resize-handle::after {
  content: '';
  position: absolute;
  right: 12px;
  bottom: 2px;
  width: 40px;
  height: 3px;
  background: rgba(255, 255, 255, 0.2);
  border-radius: 2px;
  transition: background var(--duration-fast);
}

/* === .resize-handle 样式定义 === */
.resize-handle:hover::after {
  background: rgba(255, 255, 255, 0.5);
}

/* === .result-container 样式定义 === */
.result-container {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 300px;
}

/* === .result-header 样式定义 === */
.result-header {
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

/* === .result-info 样式定义 === */
.result-info {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

/* === .result-meta 样式定义 === */
.result-meta {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

/* === .meta-item 样式定义 === */
.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

/* === .cache-badge 样式定义 === */
.cache-badge {
  background: rgba(251, 191, 36, 0.1);
  color: var(--accent-warning);
  padding: 1px var(--space-2);
  border-radius: var(--radius-sm);
}

/* === .truncated-hint 样式定义 === */
.truncated-hint {
  font-size: 0.6875rem;
  color: var(--accent-warning);
  margin-left: var(--space-1);
}

/* === .error-text 样式定义 === */
.error-text {
  color: var(--accent-danger);
}

/* === .result-body 样式定义 === */
.result-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

/* === .virtual-table-wrapper 样式定义 === */
.virtual-table-wrapper {
  flex: 1;
  overflow: auto;
  position: relative;
}

/* === .virtual-table 样式定义 === */
.virtual-table {
  border-collapse: separate;
  border-spacing: 0;
  font-size: 13px;
  white-space: nowrap;
  table-layout: fixed;
}

/* === .virtual-table 样式定义 === */
.virtual-table thead {
  position: sticky;
  top: 0;
  z-index: 3;
}

/* === .vt-cell 样式定义 === */
.vt-cell {
  padding: 6px 12px;
  border: 1px solid var(--el-border-color-lighter);
  overflow: hidden;
  text-overflow: ellipsis;
}

/* === .vt-header 样式定义 === */
.vt-header {
  background: var(--bg-secondary);
  font-weight: 600;
  padding: 8px 12px;
  text-align: left;
  user-select: none;
  position: relative;
}

/* === .vt-header-text 样式定义 === */
.vt-header-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
  padding-right: 8px;
}

/* === .vt-resize-handle 样式定义 === */
.vt-resize-handle {
  position: absolute;
  right: 0;
  top: 0;
  bottom: 0;
  width: 6px;
  cursor: col-resize;
  z-index: 5;
  background: transparent;
  transition: background 0.15s;
}

/* === .vt-resize-handle 样式定义 === */
.vt-resize-handle:hover {
  background: rgba(59, 130, 246, 0.3);
}

/* === .vt-resize-handle 样式定义 === */
.vt-resize-handle:active {
  background: rgba(59, 130, 246, 0.5);
}

/* === .vt-col-index 样式定义 === */
.vt-col-index {
  padding: 6px 8px;
  border: 1px solid var(--el-border-color-lighter);
  color: var(--text-muted);
  font-size: 12px;
  text-align: center;
  width: 50px;
  min-width: 50px;
  max-width: 50px;
  user-select: none;
  background: var(--bg-secondary);
  font-weight: 600;
  position: sticky;
  left: 0;
  z-index: 2;
}

/* === .vt-col-index 样式定义 === */
.virtual-table thead .vt-col-index {
  z-index: 4;
}

/* === .vt-col-right 样式定义 === */
.vt-col-right {
  text-align: right;
}

/* === .vt-row-even 样式定义 === */
.vt-row-even {
  background: var(--el-fill-color-lighter);
}

/* === .vt-row-even 样式定义 === */
.vt-col-index.vt-row-even {
  background: var(--el-fill-color-lighter);
}

/* === .vt-cell 样式定义 === */
.virtual-table tbody tr:hover .vt-cell {
  background: var(--bg-hover);
}

/* === .vt-col-index 样式定义 === */
.virtual-table tbody tr:hover .vt-col-index {
  background: var(--bg-hover);
}

/* === .vt-spacer-row 样式定义 === */
.vt-spacer-row td {
  padding: 0;
  border: none;
}

/* === .vt-spacer-cell 样式定义 === */
.vt-spacer-cell {
  padding: 0;
  border: none;
}

/* === .empty-result 样式定义 === */
.empty-result {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
}

/* === .empty-result 样式定义 === */
.empty-result p {
  margin-top: var(--space-3);
}

/* === .empty-query 样式定义 === */
.empty-query {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--bg-card);
  border: 1px dashed var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-8);
}

/* === .empty-icon 样式定义 === */
.empty-icon {
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-secondary);
  border-radius: 50%;
  margin-bottom: var(--space-4);
  color: var(--accent-primary);
}

/* === .empty-query 样式定义 === */
.empty-query h3 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: var(--space-2);
}

/* === .empty-query 样式定义 === */
.empty-query p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-bottom: var(--space-4);
}

/* === .empty-hint 样式定义 === */
.empty-hint {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--font-size-sm);
  color: var(--text-muted);
}

/* === .save-form 样式定义 === */
.save-form {
  padding: var(--space-2);
}

/* === .form-hint 样式定义 === */
.form-hint {
  font-size: var(--font-size-xs);
  color: var(--text-muted);
  margin-left: var(--space-2);
}

/* === .save-overwrite-hint 样式定义 === */
.save-overwrite-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-color-warning);
}

/* === .name-row 样式定义 === */
.name-row {
  display: flex;
  gap: 8px;
  align-items: center;
  width: 100%;
}

/* === 对话框底部按钮区 === */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

/* === .history-item 样式定义 === */
.history-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 400px;
}

/* === .history-sql 样式定义 === */
.history-sql {
  font-family: var(--font-mono);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-primary);
}

/* === .history-time 样式定义 === */
.history-time {
  font-size: 11px;
  color: var(--text-muted);
}
</style>
