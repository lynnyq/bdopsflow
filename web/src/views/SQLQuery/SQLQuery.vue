<template>
  <div class="sql-query-page">
    <div class="main-content">
      <aside class="metadata-panel">
        <div class="panel-header">
          <div class="panel-title">
            <el-icon :size="18"><DataLine /></el-icon>
            <span>数据源</span>
          </div>
          <el-button
            link
            type="primary"
            size="small"
            @click="refreshMetadata"
            :disabled="!selectedDatasourceId"
          >
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div class="panel-section selector-section" :class="{ collapsed: selectorCollapsed }">
          <div class="section-label selector-toggle" @click="selectorCollapsed = !selectorCollapsed">
            <span>数据源选择</span>
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
              <el-select
                v-model="selectedDatabase"
                placeholder="选择数据库"
                class="field-select"
                filterable
                :disabled="!selectedDatasourceId || loadingDatabases"
                :loading="loadingDatabases"
                @change="handleDatabaseChange"
                @visible-change="onDatabaseVisibleChange"
              >
                <el-option
                  v-for="db in databases"
                  :key="db"
                  :label="db"
                  :value="db"
                />
              </el-select>
            </div>
            <div class="selector-field">
              <span class="field-label">表</span>
              <el-select
                v-model="selectedTable"
                placeholder="选择表"
                class="field-select"
                filterable
                :disabled="!selectedDatabase || loadingTables"
                :loading="loadingTables"
                @change="handleTableChange"
              >
                <el-option
                  v-for="table in availableTables"
                  :key="table.name"
                  :label="table.name"
                  :value="table.name"
                />
              </el-select>
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

      <main class="query-area">
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
          <div class="editor-header">
            <div class="tab-bar">
              <div class="tabs-scroll">
                <div
                  v-for="tab in tabs"
                  :key="tab.id"
                  class="tab-item"
                  :class="{ active: tab.id === activeTabId }"
                  @click="handleTabSwitch(tab.id)"
                >
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
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="(item, index) in recentSQLHistory"
                      :key="index"
                      :command="item"
                    >
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
          <div
            class="editor-wrapper"
            :style="{ height: `${editorHeight}px` }"
          >
            <div ref="editorRef" class="sql-editor"></div>
          </div>
          <div
            class="resize-handle"
            @mousedown="startResize"
            title="拖拽调整高度"
          ></div>
        </div>

        <div class="result-container" v-if="queryResult || errorMessage">
          <div class="result-header">
            <div class="result-info">
              <template v-if="queryResult">
                <el-tag type="success" effect="light" size="small">
                  <el-icon><CircleCheck /></el-icon> 查询成功
                </el-tag>
                <span class="result-meta">
                  <span class="meta-item">
                    <el-icon :size="14"><DataLine /></el-icon>
                    {{ queryResult.row_count }} 行
                    <span v-if="queryResult.row_count > MAX_DISPLAY_ROWS" class="truncated-hint">(显示前 {{ MAX_DISPLAY_ROWS }} 行)</span>
                  </span>
                  <span class="meta-item">
                    <el-icon :size="14"><Clock /></el-icon>
                    {{ formatTime(queryResult.execution_time) }}
                  </span>
                  <span v-if="queryResult.from_cache" class="meta-item cache-badge">
                    <el-icon :size="14"><Clock /></el-icon> 缓存
                  </span>
                </span>
              </template>
              <template v-if="errorMessage">
                <el-tag type="danger" effect="light" size="small">
                  <el-icon><CircleClose /></el-icon> 查询失败
                </el-tag>
                <span class="result-meta error-text">{{ errorMessage }}</span>
              </template>
            </div>
          </div>

          <div class="result-body" v-if="queryResult">
            <div class="table-container">
              <el-table
                v-if="queryResult.rows && queryResult.rows.length > 0"
                :data="formatResultRows"
                stripe
                border
                class="result-table"
                :header-cell-style="{ background: 'var(--bg-secondary)', fontWeight: 600 }"
              >
                <el-table-column
                  v-for="col in queryResult.columns"
                  :key="col"
                  :prop="col"
                  :label="col"
                  :min-width="120"
                  :align="getColumnAlign(col)"
                  show-overflow-tooltip
                />
              </el-table>
              <div v-else class="empty-result">
                <el-icon :size="48"><Document /></el-icon>
                <p>查询结果为空</p>
              </div>
            </div>
          </div>
        </div>

        <div class="empty-query" v-if="!queryResult && !errorMessage">
          <div class="empty-icon">
            <el-icon :size="64"><Document /></el-icon>
          </div>
          <h3>开始查询</h3>
          <p>选择数据源和表，编写 SQL 语句后点击执行查询</p>
          <div class="empty-hint">
            <el-icon :size="16"><Key /></el-icon>
            <span>提示：Ctrl+Enter 执行（选中内容优先），Ctrl+Shift+F 格式化，点击字段名可插入到编辑器</span>
          </div>
        </div>
      </main>
    </div>

    <el-dialog
      v-model="saveDialogVisible"
      title="保存 SQL"
      width="480px"
      :close-on-click-modal="false"
    >
      <el-form ref="saveFormRef" :model="saveForm" :rules="saveRules" label-position="top" class="save-form">
        <el-form-item label="名称" prop="name">
          <el-input v-model="saveForm.name" placeholder="请输入SQL名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="saveForm.description" type="textarea" :rows="2" placeholder="请输入描述" />
        </el-form-item>
        <el-form-item label="公开">
          <el-switch v-model="saveForm.is_public" />
          <span class="form-hint">公开的 SQL 可被同领域其他用户使用</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="saveDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleConfirmSave" :loading="saving">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted, nextTick } from 'vue';
import { useRoute } from 'vue-router';
import { EditorView, basicSetup } from 'codemirror';
import { keymap } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { sql } from '@codemirror/lang-sql';
import { oneDark } from '@codemirror/theme-one-dark';
import { autocompletion, CompletionContext, CompletionResult } from '@codemirror/autocomplete';
import { historyKeymap } from '@codemirror/commands';
import { searchKeymap } from '@codemirror/search';
import { format } from 'sql-formatter';
import { ElMessage } from 'element-plus';
import {
  DataLine, Refresh, Key, Document, Search,
  CircleClose, VideoPlay, Delete, CircleCheck, Clock, Download, Warning,
  Plus, Close, ArrowDown, ArrowUp
} from '@element-plus/icons-vue';
import { datasourceAPI, queryAPI } from '@/api';
import { isHandledError } from '@/utils/api';
import { useAuthStore } from '@/stores/auth';
import type { Datasource, QueryResult, QueryHistory, TableInfo, ColumnInfo } from '@/types';

const authStore = useAuthStore();
const route = useRoute();

const getStorageKey = () => `bdopsflow_sql_tabs_${authStore.user?.id || 'anonymous'}`;

interface SQLTab {
  id: string;
  name: string;
  sql: string;
  datasourceId: number | '';
  database: string;
}

const generateTabId = () => `tab_${Date.now()}_${Math.random().toString(36).substring(2, 8)}`;
let tabCounter = 0;

const loadTabsFromStorage = (): { tabs: SQLTab[]; activeTabId: string } => {
  try {
    const raw = localStorage.getItem(getStorageKey());
    if (raw) {
      const data = JSON.parse(raw);
      if (data.tabs && data.tabs.length > 0) {
        tabCounter = data.tabs.length;
        return { tabs: data.tabs, activeTabId: data.activeTabId || data.tabs[0].id };
      }
    }
  } catch (e) {
  }
  tabCounter = 1;
  const defaultTab: SQLTab = {
    id: generateTabId(),
    name: '查询 1',
    sql: '',
    datasourceId: '',
    database: '',
  };
  return { tabs: [defaultTab], activeTabId: defaultTab.id };
};

const saved = loadTabsFromStorage();
const tabs = ref<SQLTab[]>(saved.tabs);
const activeTabId = ref<string>(saved.activeTabId);

watch(() => authStore.user?.id, (newUserId, oldUserId) => {
  if (newUserId !== oldUserId && newUserId) {
    syncCurrentTabData();
    const loaded = loadTabsFromStorage();
    tabs.value = loaded.tabs;
    activeTabId.value = loaded.activeTabId;
    sqlText.value = activeTab.value.sql;
    selectedDatasourceId.value = activeTab.value.datasourceId;
    selectedDatabase.value = activeTab.value.database;
    selectedTable.value = '';
    queryResult.value = null;
    errorMessage.value = '';
    nextTick(() => {
      if (editorView) {
        editorView.dispatch({
          changes: { from: 0, to: editorView.state.doc.length, insert: sqlText.value }
        });
      }
    });
  }
});

const saveTabsToStorage = () => {
  if (saveTabsTimer) clearTimeout(saveTabsTimer);
  saveTabsTimer = setTimeout(() => {
    try {
      localStorage.setItem(getStorageKey(), JSON.stringify({
        tabs: tabs.value,
        activeTabId: activeTabId.value,
      }));
    } catch (e) {
    }
  }, 500);
};

const activeTab = computed(() => tabs.value.find(t => t.id === activeTabId.value) || tabs.value[0]);

const editorRef = ref<HTMLElement>();
const editorHeight = ref(200);
const isResizing = ref(false);
const sqlText = ref(activeTab.value.sql);
const selectorCollapsed = ref(false);
const executing = ref(false);
const exporting = ref(false);
const exportProgress = ref(0);
const allowWriteSQL = ref(false);
const canceling = ref(false);
const currentQueryId = ref('');
let pollTimer: ReturnType<typeof setInterval> | null = null;
const queryResult = ref<QueryResult | null>(null);
const errorMessage = ref('');

const datasources = ref<Datasource[]>([]);
const selectedDatasourceId = ref<number | ''>(activeTab.value.datasourceId);
const databases = ref<string[]>([]);
const selectedDatabase = ref(activeTab.value.database);
const tables = ref<TableInfo[]>([]);
const selectedTable = ref('');
const selectedTableColumns = ref<ColumnInfo[]>([]);
const loadingDatabases = ref(false);
const loadingTables = ref(false);
const loadingColumns = ref(false);

let metadataAbortController: AbortController | null = null;
const metadataCache = new Map<string, { data: any; timestamp: number }>();

const cancelPendingMetadataRequests = () => {
  if (metadataAbortController) {
    metadataAbortController.abort();
    metadataAbortController = null;
  }
};

const getMetadataSignal = (): AbortSignal | undefined => {
  cancelPendingMetadataRequests();
  metadataAbortController = new AbortController();
  return metadataAbortController.signal;
};

const isCanceledError = (err: any): boolean => {
  return err?.code === 'ERR_CANCELED' || err?.name === 'CanceledError' || err?.name === 'AbortError';
};

const METADATA_CACHE_TTL = 30 * 60 * 1000;
const METADATA_STORAGE_PREFIX = 'bdopsflow_meta_cache_';

const getStorageCacheKey = (key: string) => `${METADATA_STORAGE_PREFIX}${key}`;

const getCachedMetadata = (key: string): any | null => {
  const entry = metadataCache.get(key);
  if (entry) {
    if (Date.now() - entry.timestamp <= METADATA_CACHE_TTL) {
      return entry.data;
    }
    metadataCache.delete(key);
  }

  try {
    const raw = localStorage.getItem(getStorageCacheKey(key));
    if (!raw) return null;
    const stored = JSON.parse(raw) as { data: any; timestamp: number };
    if (Date.now() - stored.timestamp > METADATA_CACHE_TTL) {
      localStorage.removeItem(getStorageCacheKey(key));
      return null;
    }
    metadataCache.set(key, stored);
    return stored.data;
  } catch {
    return null;
  }
};

const setCachedMetadata = (key: string, data: any) => {
  const entry = { data, timestamp: Date.now() };
  metadataCache.set(key, entry);
  try {
    localStorage.setItem(getStorageCacheKey(key), JSON.stringify(entry));
  } catch {
  }
};

const autocompleteData = ref<{
  tables: string[];
  columns: string[];
  keywords: string[];
}>({
  tables: [],
  columns: [],
  keywords: ['SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'NOT', 'IN', 'LIKE', 'BETWEEN', 'ORDER BY', 'GROUP BY', 'HAVING', 'LIMIT', 'OFFSET', 'JOIN', 'LEFT', 'RIGHT', 'INNER', 'OUTER', 'ON', 'AS', 'DISTINCT', 'COUNT', 'SUM', 'AVG', 'MAX', 'MIN', 'CASE', 'WHEN', 'THEN', 'ELSE', 'END', 'UNION', 'ALL', 'INSERT', 'INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE', 'CREATE', 'TABLE', 'DROP', 'ALTER', 'ADD', 'INDEX', 'PRIMARY', 'KEY', 'FOREIGN', 'REFERENCES', 'DEFAULT', 'NULL', 'NOT NULL', 'UNIQUE', 'CHECK', 'CONSTRAINT']
});

const saveDialogVisible = ref(false);
const saving = ref(false);
const saveForm = ref({
  name: '',
  description: '',
  is_public: false
});
const saveRules = ref({
  name: [
    { required: true, message: '请输入SQL名称', trigger: 'blur' },
    { min: 1, max: 100, message: '名称长度在1到100个字符', trigger: 'blur' }
  ]
});
const saveFormRef = ref();

const recentSQLHistory = ref<{ sql_text: string; created_at: string }[]>([]);

const dsTypeLabels: Record<string, string> = {
  mysql: 'MySQL',
  hive: 'Hive',
  kyuubi: 'Kyuubi',
  trino: 'Trino',
  spark: 'Spark',
  starrocks: 'StarRocks',
  doris: 'Doris',
  sqlite: 'SQLite',
  rqlite: 'rqlite'
};

let editorView: EditorView | null = null;
let saveTabsTimer: ReturnType<typeof setTimeout> | null = null;
const availableTables = computed(() => tables.value);

const numericTypes = new Set([
  'int', 'integer', 'bigint', 'smallint', 'tinyint', 'mediumint',
  'float', 'double', 'decimal', 'numeric', 'real', 'number',
  'money', 'serial', 'bigserial', 'smallserial'
]);

const columnAlignMap = computed(() => {
  const map: Record<string, string> = {};
  if (!selectedTableColumns.value.length) return map;
  for (const col of selectedTableColumns.value) {
    if (numericTypes.has(col.type.toLowerCase().split('(')[0])) {
      map[col.name] = 'right';
    }
  }
  return map;
});

const getColumnAlign = (colName: string) => {
  return columnAlignMap.value[colName] || 'left';
};

const formatCellValue = (value: any): string => {
  if (value === null || value === undefined) return '-';
  if (typeof value === 'boolean') return value ? '是' : '否';
  if (value instanceof ArrayBuffer || (typeof value === 'object' && value.constructor?.name === 'ArrayBuffer')) {
    return '[BLOB]';
  }
  return String(value);
};

const MAX_DISPLAY_ROWS = 500;

const formatResultRows = computed(() => {
  if (!queryResult.value?.rows) return [];
  const rows = queryResult.value.rows;
  const displayRows = rows.length > MAX_DISPLAY_ROWS ? rows.slice(0, MAX_DISPLAY_ROWS) : rows;
  return displayRows.map(row => {
    const obj: Record<string, string> = {};
    queryResult.value!.columns.forEach((col, i) => {
      obj[col] = formatCellValue(row[i]);
    });
    return obj;
  });
});

const syncCurrentTabData = () => {
  const tab = tabs.value.find(t => t.id === activeTabId.value);
  if (!tab) return;
  tab.sql = sqlText.value;
  tab.datasourceId = selectedDatasourceId.value;
  tab.database = selectedDatabase.value;
  saveTabsToStorage();
};

const handleTabSwitch = (tabId: string) => {
  if (tabId === activeTabId.value) return;

  const currentTab = tabs.value.find(t => t.id === activeTabId.value);
  if (currentTab) {
    currentTab.sql = sqlText.value;
    currentTab.datasourceId = selectedDatasourceId.value;
    currentTab.database = selectedDatabase.value;
  }

  activeTabId.value = tabId;
  const newTab = tabs.value.find(t => t.id === tabId);
  if (!newTab) return;

  sqlText.value = newTab.sql;
  selectedDatasourceId.value = newTab.datasourceId;
  selectedDatabase.value = newTab.database;

  if (editorView) {
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: newTab.sql }
    });
  }

  if (newTab.datasourceId) {
    handleDatasourceChangeForTab(newTab.datasourceId, newTab.database);
  } else {
    databases.value = [];
    tables.value = [];
    selectedTable.value = '';
    selectedTableColumns.value = [];
    allowWriteSQL.value = false;
  }

  queryResult.value = null;
  errorMessage.value = '';
  saveTabsToStorage();
};

const handleDatasourceChangeForTab = async (dsId: number | '', dbName: string) => {
  databases.value = [];
  selectedDatabase.value = '';
  tables.value = [];
  selectedTable.value = '';
  selectedTableColumns.value = [];

  if (!dsId) {
    allowWriteSQL.value = false;
    return;
  }

  const ds = datasources.value.find(d => d.id === dsId);
  if (!ds) {
    allowWriteSQL.value = false;
    return;
  }

  allowWriteSQL.value = ds.allow_write_sql || false;

  if (ds.type === 'sqlite' || ds.type === 'rqlite') {
    databases.value = ['main'];
    selectedDatabase.value = dbName || 'main';
    await handleDatabaseChange();
    return;
  }

  const cacheKey = `databases_${dsId}`;
  const cached = getCachedMetadata(cacheKey);
  if (cached) {
    databases.value = cached;
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
  try {
    const res = await datasourceAPI.getDatabases(dsId as number, getMetadataSignal());
    let dbList = res.data || [];
    if (dbList.length === 0) {
      dbList = ds.database ? [ds.database] : ['default'];
    }
    databases.value = dbList;
    setCachedMetadata(cacheKey, dbList);
    if (dbName && dbList.includes(dbName)) {
      selectedDatabase.value = dbName;
      await handleDatabaseChange();
    } else if (dbList.length === 1) {
      selectedDatabase.value = dbList[0];
      await handleDatabaseChange();
    }
  } catch (err: any) {
    if (!isCanceledError(err) && !isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '获取数据库列表失败';
      ElMessage.error(`获取数据库列表失败: ${msg}`);
    }
    databases.value = ds.database ? [ds.database] : ['default'];
    if (dbName) {
      selectedDatabase.value = dbName;
      await handleDatabaseChange();
    }
  } finally {
    loadingDatabases.value = false;
  }
};

const handleAddTab = () => {
  syncCurrentTabData();
  tabCounter++;
  const newTab: SQLTab = {
    id: generateTabId(),
    name: `查询 ${tabCounter}`,
    sql: '',
    datasourceId: selectedDatasourceId.value,
    database: selectedDatabase.value,
  };
  tabs.value.push(newTab);
  activeTabId.value = newTab.id;

  sqlText.value = '';
  if (editorView) {
    editorView.dispatch({
      changes: { from: 0, to: editorView.state.doc.length, insert: '' }
    });
  }

  queryResult.value = null;
  errorMessage.value = '';
  saveTabsToStorage();
};

const handleCloseTab = (tabId: string) => {
  if (tabs.value.length <= 1) return;

  const idx = tabs.value.findIndex(t => t.id === tabId);
  if (idx === -1) return;

  tabs.value.splice(idx, 1);

  if (tabId === activeTabId.value) {
    const newIdx = Math.min(idx, tabs.value.length - 1);
    handleTabSwitch(tabs.value[newIdx].id);
  } else {
    saveTabsToStorage();
  }
};

const startRenameTab = (tab: SQLTab) => {
  const newName = prompt('重命名标签页', tab.name);
  if (newName && newName.trim()) {
    tab.name = newName.trim();
    saveTabsToStorage();
  }
};

const startResize = (e: MouseEvent) => {
  isResizing.value = true;
  document.addEventListener('mousemove', onResize);
  document.addEventListener('mouseup', stopResize);
  e.preventDefault();
};

const onResize = (e: MouseEvent) => {
  if (!isResizing.value) return;
  const container = document.querySelector('.query-area');
  if (!container) return;
  const rect = container.getBoundingClientRect();
  const maxHeight = rect.height - 200;
  const minHeight = 120;
  const deltaY = e.movementY;
  const newHeight = editorHeight.value + deltaY;
  editorHeight.value = Math.max(minHeight, Math.min(maxHeight, newHeight));
};

const stopResize = () => {
  isResizing.value = false;
  document.removeEventListener('mousemove', onResize);
  document.removeEventListener('mouseup', stopResize);
};

const createEditor = () => {
  if (!editorRef.value) return;

  const sqlCompletion = (context: CompletionContext): CompletionResult | null => {
    const textBefore = context.state.doc.sliceString(0, context.pos);

    const dotMatch = context.matchBefore(/[\w]+\.[\w]*/);
    if (dotMatch) {
      const dotIdx = dotMatch.text.lastIndexOf('.');
      const tableName = dotMatch.text.substring(0, dotIdx);
      const colPrefix = dotMatch.text.substring(dotIdx + 1).toLowerCase();
      const cols = autocompleteData.value.columns
        .filter(c => c.toLowerCase().startsWith(colPrefix))
        .map(c => ({ label: c, type: 'property' as const, detail: `${tableName}.字段`, apply: c }));
      if (cols.length === 0) return null;
      return { from: dotMatch.from + dotIdx + 1, options: cols, validFor: /^\w*$/ };
    }

    const word = context.matchBefore(/\w+/);
    if (!word || (word.from === word.to && !context.explicit)) return null;

    const prefix = word.text.toLowerCase();
    const beforeWord = textBefore.slice(0, word.from).trimEnd().toUpperCase();

    const endsWithAny = (kws: string[]) => kws.some(k => beforeWord.endsWith(k));

    type CompOpt = { label: string; type: 'keyword' | 'class' | 'property' | 'function'; detail: string; apply: string };
    let options: CompOpt[] = [];

    if (endsWithAny(['SELECT', 'DISTINCT'])) {
      options = [
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c })),
        ...['COUNT', 'SUM', 'AVG', 'MAX', 'MIN', 'DISTINCT', 'CASE', 'NULL', '*'].map(k => ({
          label: k, type: (k === '*' ? 'keyword' : 'function') as CompOpt['type'], detail: k === '*' ? '通配符' : '聚合函数', apply: k
        }))
      ];
    } else if (endsWithAny(['FROM', 'JOIN', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'OUTER JOIN', 'CROSS JOIN', 'UPDATE', 'INTO'])) {
      options = autocompleteData.value.tables.map(t => ({ label: t, type: 'class' as const, detail: '表', apply: t }));
    } else if (endsWithAny(['WHERE', 'AND', 'OR', 'NOT', 'ON', 'HAVING', 'SET'])) {
      options = [
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c })),
        ...['NOT', 'NULL', 'IN', 'LIKE', 'BETWEEN', 'EXISTS', 'IS', 'TRUE', 'FALSE'].map(k => ({
          label: k, type: 'keyword' as const, detail: '关键字', apply: k
        }))
      ];
    } else if (endsWithAny(['ORDER BY', 'GROUP BY'])) {
      options = [
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c })),
        ...['ASC', 'DESC'].map(k => ({ label: k, type: 'keyword' as const, detail: '排序', apply: k }))
      ];
    } else {
      options = [
        ...autocompleteData.value.keywords.map(k => ({ label: k, type: 'keyword' as const, detail: '关键字', apply: k })),
        ...autocompleteData.value.tables.map(t => ({ label: t, type: 'class' as const, detail: '表', apply: t })),
        ...autocompleteData.value.columns.map(c => ({ label: c, type: 'property' as const, detail: '字段', apply: c }))
      ];
    }

    if (prefix) {
      options = options.filter(o => o.label.toLowerCase().startsWith(prefix));
    }
    if (options.length === 0) return null;

    options.sort((a, b) => {
      const al = a.label.toLowerCase().startsWith(prefix) ? 0 : 1;
      const bl = b.label.toLowerCase().startsWith(prefix) ? 0 : 1;
      if (al !== bl) return al - bl;
      return a.label.localeCompare(b.label);
    });

    return {
      from: word.from,
      options: options.slice(0, 80),
      validFor: /^\w*$/
    };
  };

  const updateListener = EditorView.updateListener.of((update) => {
    if (update.docChanged) {
      sqlText.value = update.state.doc.toString();
      const tab = tabs.value.find(t => t.id === activeTabId.value);
      if (tab) {
        tab.sql = sqlText.value;
        saveTabsToStorage();
      }
    }
  });

  const state = EditorState.create({
    doc: sqlText.value,
    extensions: [
      basicSetup,
      sql(),
      oneDark,
      autocompletion({ override: [sqlCompletion], activateOnTypingDelay: 150 }),
      updateListener,
      keymap.of([
        ...historyKeymap,
        ...searchKeymap,
        {
          key: 'Ctrl-Enter',
          run: () => {
            handleExecute();
            return true;
          }
        },
        {
          key: 'Ctrl-Shift-f',
          run: () => {
            handleFormat();
            return true;
          }
        }
      ]),
      EditorView.theme({
        '&': { height: '100%' },
        '.cm-content': { padding: '12px 16px' },
        '.cm-line': { padding: '2px 0' },
        '.cm-editor': { height: '100%' },
        '.cm-scroller': { overflow: 'auto' }
      }),
    ]
  });

  editorView = new EditorView({
    state,
    parent: editorRef.value
  });
};

watch(editorHeight, () => {
  if (editorView && editorRef.value) {
    requestAnimationFrame(() => {
      editorView?.requestMeasure();
    });
  }
});

watch([selectedDatasourceId, selectedDatabase], () => {
  syncCurrentTabData();
});

const loadDatasources = async () => {
  try {
    const res = await datasourceAPI.list();
    if (res.data) {
      datasources.value = res.data.items || res.data;
    }
    if (selectedDatasourceId.value) {
      const found = datasources.value.find((d: any) => d.id === selectedDatasourceId.value);
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

const loadRecentHistory = async () => {
  try {
    const res = await queryAPI.getHistory({ page: 1, page_size: 10 });
    if (res.data) {
      recentSQLHistory.value = (res.data.items || []).map((item: QueryHistory) => ({
        sql_text: item.sql_text,
        created_at: item.created_at
      }));
    }
  } catch (err) {
  }
};

const handleHistorySelect = (item: { sql_text: string; created_at: string }) => {
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

const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '';
  const date = new Date(dateStr);
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  });
};

const handleDatasourceChange = async () => {
  databases.value = [];
  selectedDatabase.value = '';
  tables.value = [];
  selectedTable.value = '';
  selectedTableColumns.value = [];
  autocompleteData.value.tables = [];
  autocompleteData.value.columns = [];

  if (!selectedDatasourceId.value) {
    allowWriteSQL.value = false;
    return;
  }

  const ds = datasources.value.find(d => d.id === selectedDatasourceId.value);
  if (!ds) {
    allowWriteSQL.value = false;
    return;
  }

  allowWriteSQL.value = ds.allow_write_sql || false;

  if (ds.type === 'sqlite' || ds.type === 'rqlite') {
    databases.value = ['main'];
    selectedDatabase.value = 'main';
    await handleDatabaseChange();
    return;
  }

  const cacheKey = `databases_${selectedDatasourceId.value}`;
  const cached = getCachedMetadata(cacheKey);
  if (cached) {
    databases.value = cached;
    if (cached.length === 1) {
      selectedDatabase.value = cached[0];
      await handleDatabaseChange();
    }
    return;
  }

  loadingDatabases.value = true;
  try {
    const res = await datasourceAPI.getDatabases(selectedDatasourceId.value as number, getMetadataSignal());
    let dbList = res.data || [];
    if (dbList.length === 0) {
      dbList = ds.database ? [ds.database] : ['default'];
    }
    databases.value = dbList;
    setCachedMetadata(cacheKey, dbList);
    if (dbList.length === 1) {
      selectedDatabase.value = dbList[0];
      await handleDatabaseChange();
    }
  } catch (err: any) {
    if (!isCanceledError(err) && !isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '获取数据库列表失败';
      ElMessage.error(`获取数据库列表失败: ${msg}`);
    }
    databases.value = ds.database ? [ds.database] : ['default'];
  } finally {
    loadingDatabases.value = false;
  }
};

const handleDatabaseChange = async () => {
  tables.value = [];
  selectedTable.value = '';
  selectedTableColumns.value = [];
  autocompleteData.value.tables = [];
  autocompleteData.value.columns = [];

  if (!selectedDatasourceId.value) return;

  const dbName = selectedDatabase.value || 'default';

  const cacheKey = `tables_${selectedDatasourceId.value}_${dbName}`;
  const cached = getCachedMetadata(cacheKey);
  if (cached) {
    tables.value = cached;
    autocompleteData.value.tables = cached.map((t: any) => t.name || '').filter(Boolean);
    return;
  }

  loadingTables.value = true;
  try {
    const res = await datasourceAPI.getTables(selectedDatasourceId.value as number, dbName, getMetadataSignal());
    const tableData = res.data || [];
    tables.value = tableData;
    setCachedMetadata(cacheKey, tableData);
    autocompleteData.value.tables = tableData.map((t: any) => t.name || '').filter(Boolean);
  } catch (err: any) {
    if (!isCanceledError(err) && !isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '获取数据表列表失败';
      ElMessage.error(`获取数据表列表失败: ${msg}`);
    }
  } finally {
    loadingTables.value = false;
  }
};

let datasourceIdOnOpen: number | '' = '';
let databaseOnOpen = '';

const onDatasourceVisibleChange = (visible: boolean) => {
  if (visible) {
    datasourceIdOnOpen = selectedDatasourceId.value;
  } else if (selectedDatasourceId.value && selectedDatasourceId.value === datasourceIdOnOpen) {
    refreshDatasourceMetadata();
  }
};

const onDatabaseVisibleChange = (visible: boolean) => {
  if (visible) {
    databaseOnOpen = selectedDatabase.value;
  } else if (selectedDatabase.value && selectedDatabase.value === databaseOnOpen) {
    refreshDatabaseMetadata();
  }
};

const refreshDatasourceMetadata = async () => {
  if (!selectedDatasourceId.value) return;
  const ds = datasources.value.find(d => d.id === selectedDatasourceId.value);
  if (!ds) return;

  if (ds.type === 'sqlite' || ds.type === 'rqlite') {
    if (selectedDatabase.value) {
      await refreshDatabaseMetadata();
    }
    return;
  }

  loadingDatabases.value = true;
  try {
    const res = await datasourceAPI.getDatabases(selectedDatasourceId.value as number, getMetadataSignal());
    let dbList = res.data || [];
    if (dbList.length === 0) {
      dbList = ds.database ? [ds.database] : ['default'];
    }
    databases.value = dbList;
    setCachedMetadata(`databases_${selectedDatasourceId.value}`, dbList);
    if (selectedDatabase.value) {
      await refreshDatabaseMetadata();
    }
  } catch (err: any) {
    if (!isCanceledError(err) && !isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '获取数据库列表失败';
      ElMessage.error(`获取数据库列表失败: ${msg}`);
    }
  } finally {
    loadingDatabases.value = false;
  }
};

const refreshDatabaseMetadata = async () => {
  if (!selectedDatasourceId.value || !selectedDatabase.value) return;

  const dbName = selectedDatabase.value;
  loadingTables.value = true;
  try {
    const res = await datasourceAPI.getTables(selectedDatasourceId.value as number, dbName, getMetadataSignal());
    const tableData = res.data || [];
    tables.value = tableData;
    setCachedMetadata(`tables_${selectedDatasourceId.value}_${dbName}`, tableData);
    autocompleteData.value.tables = tableData.map((t: any) => t.name || '').filter(Boolean);
  } catch (err: any) {
    if (!isCanceledError(err) && !isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '获取数据表列表失败';
      ElMessage.error(`获取数据表列表失败: ${msg}`);
    }
  } finally {
    loadingTables.value = false;
  }
};

const handleTableChange = async () => {
  if (!selectedTable.value) {
    selectedTableColumns.value = [];
    autocompleteData.value.columns = [];
    return;
  }

  const dbName = selectedDatabase.value || 'default';

  const cacheKey = `columns_${selectedDatasourceId.value}_${dbName}_${selectedTable.value}`;
  const cached = getCachedMetadata(cacheKey);
  if (cached) {
    selectedTableColumns.value = cached;
    autocompleteData.value.columns = cached.map((c: any) => c.name || '').filter(Boolean);
    return;
  }

  loadingColumns.value = true;
  try {
    const res = await datasourceAPI.getColumns(selectedDatasourceId.value as number, dbName, selectedTable.value, getMetadataSignal());
    selectedTableColumns.value = res.data || [];
    setCachedMetadata(cacheKey, res.data || []);
    autocompleteData.value.columns = (res.data || []).map((c: any) => c.name || '').filter(Boolean);
  } catch (err: any) {
    if (!isCanceledError(err) && !isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '获取字段列表失败';
      ElMessage.error(`获取字段列表失败: ${msg}`);
    }
    selectedTableColumns.value = [];
    autocompleteData.value.columns = [];
  } finally {
    loadingColumns.value = false;
  }
};

const clearMetadataCache = (key: string) => {
  metadataCache.delete(key);
  try {
    localStorage.removeItem(getStorageCacheKey(key));
  } catch {
  }
};

const refreshMetadata = async () => {
  if (!selectedDatasourceId.value) return;

  clearMetadataCache(`databases_${selectedDatasourceId.value}`);
  if (selectedDatabase.value) {
    clearMetadataCache(`tables_${selectedDatasourceId.value}_${selectedDatabase.value}`);
  }
  if (selectedTable.value) {
    clearMetadataCache(`columns_${selectedDatasourceId.value}_${selectedDatabase.value}_${selectedTable.value}`);
  }

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

const insertColumn = (column: { name: string }) => {
  if (!editorView) return;
  const cursor = editorView.state.selection.main.head;
  editorView.dispatch({
    changes: {
      from: cursor,
      insert: `\`${column.name}\``
    }
  });
  editorView.focus();
};

const insertSelectAll = () => {
  if (!selectedTable.value || !editorView) return;
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

const insertSelectColumns = () => {
  if (!selectedTable.value || !editorView || selectedTableColumns.value.length === 0) return;
  const columns = selectedTableColumns.value.map(c => `\`${c.name}\``).join(', ');
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

const insertCount = () => {
  if (!selectedTable.value || !editorView) return;
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

const clearEditor = () => {
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

const handleFormat = () => {
  if (!sqlText.value || !editorView) return;
  try {
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

const getSQLToExecute = (): string => {
  if (!editorView) return sqlText.value;

  const selection = editorView.state.selection.main;
  if (selection.from !== selection.to) {
    return editorView.state.doc.sliceString(selection.from, selection.to);
  }

  return sqlText.value;
};

const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
};

const handleExecute = async () => {
  const sql = getSQLToExecute();
  if (!sql.trim() || !selectedDatasourceId.value || executing.value) return;

  stopPolling();
  executing.value = true;
  errorMessage.value = '';
  currentQueryId.value = '';
  queryResult.value = null;

  try {
    const res = await queryAPI.execute({
      datasource_id: selectedDatasourceId.value as number,
      sql: sql,
      database: selectedDatabase.value
    });

    const data = res.data as any;
    const queryId = data.query_id;
    const status = data.status;

    if (status === 'completed') {
      queryResult.value = data;
      currentQueryId.value = queryId;
      executing.value = false;
      loadRecentHistory();
      return;
    }

    currentQueryId.value = queryId;

    pollTimer = setInterval(async () => {
      try {
        const pollRes = await queryAPI.getResult(queryId);
        const pollData = (pollRes as any).data as any;

        if (pollData.status === 'completed') {
          stopPolling();
          queryResult.value = pollData;
          executing.value = false;
          loadRecentHistory();
        } else if (pollData.status === 'failed') {
          stopPolling();
          errorMessage.value = pollData.error || '查询失败';
          queryResult.value = null;
          executing.value = false;
        } else if (pollData.status === 'cancelled') {
          stopPolling();
          errorMessage.value = pollData.error || '查询已取消';
          queryResult.value = null;
          executing.value = false;
        }
      } catch (err: any) {
        stopPolling();
        const rawMsg = err?.response?.data?.error || err?.message || '轮询查询结果失败';
        errorMessage.value = rawMsg;
        queryResult.value = null;
        executing.value = false;
      }
    }, 1000);
  } catch (err: any) {
    const rawMsg = err?.response?.data?.error || err?.message || '查询失败，请检查网络连接';
    errorMessage.value = rawMsg;
    queryResult.value = null;
    executing.value = false;
  }
};

const handleCancel = async () => {
  if (!currentQueryId.value || canceling.value) return;

  canceling.value = true;
  try {
    await queryAPI.cancel(currentQueryId.value);
    stopPolling();
    errorMessage.value = '查询已取消';
    queryResult.value = null;
    ElMessage.info('查询已取消');
  } catch (err: any) {
    if (!isHandledError(err)) {
      const msg = err?.response?.data?.message || err?.message || '取消失败';
      ElMessage.error(msg);
    }
  } finally {
    canceling.value = false;
    executing.value = false;
    currentQueryId.value = '';
  }
};

const handleSaveSQL = () => {
  saveDialogVisible.value = true;
};

const handleConfirmSave = async () => {
  if (!saveFormRef.value) return;

  const valid = await saveFormRef.value.validate();
  if (!valid) return;

  saving.value = true;
  try {
    await queryAPI.saveSQL({
      name: saveForm.value.name,
      datasource_id: selectedDatasourceId.value as number,
      sql_text: sqlText.value,
      description: saveForm.value.description,
      is_public: saveForm.value.is_public
    });
    ElMessage.success('保存成功');
    saveDialogVisible.value = false;
    saveForm.value = {
      name: '',
      description: '',
      is_public: false
    };
  } catch (err) {
    if (!isHandledError(err)) {
      ElMessage.error('保存失败');
    }
  } finally {
    saving.value = false;
  }
};

const handleExportCSV = async () => {
  if (!queryResult.value || !selectedDatasourceId.value) return;

  try {
    exporting.value = true;
    const res = await queryAPI.exportCSV({
      datasource_id: selectedDatasourceId.value,
      sql: sqlText.value,
      database: selectedDatabase.value || undefined,
    }, (event: any) => {
      if (event.lengthComputable) {
        exportProgress.value = Math.round((event.loaded / event.total) * 100);
      }
    });

    const blob = new Blob([res.data as any], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);
    link.setAttribute('href', url);
    link.setAttribute('download', `query_result_${Date.now()}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  } catch (err: any) {
    if (!isHandledError(err)) {
      if (err.response?.data instanceof Blob) {
        const text = await err.response.data.text();
        try {
          const json = JSON.parse(text);
          ElMessage.error(json.message || '导出失败');
        } catch {
          ElMessage.error('导出失败');
        }
      } else {
        ElMessage.error(err.message || '导出失败');
      }
    }
  } finally {
    exporting.value = false;
    exportProgress.value = 0;
  }
};

const formatTime = (seconds?: number) => {
  if (!seconds && seconds !== 0) return '-';
  if (seconds < 1) {
    return `${(seconds * 1000).toFixed(0)}ms`;
  } else if (seconds < 60) {
    return `${seconds.toFixed(2)}s`;
  } else {
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = (seconds % 60).toFixed(2);
    return `${minutes}m ${remainingSeconds}s`;
  }
};

const initFromRoute = () => {
  const dsId = route.query?.datasource_id as string;
  const sql = route.query?.sql as string;
  if (dsId != null && dsId !== '') {
    const parsedId = parseInt(dsId);
    if (!isNaN(parsedId)) {
      selectedDatasourceId.value = parsedId;
      handleDatasourceChange();
    }
  }
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

onMounted(async () => {
  await loadDatasources();
  loadRecentHistory();
  createEditor();

  const tab = activeTab.value;
  if (tab.datasourceId) {
    await handleDatasourceChangeForTab(tab.datasourceId, tab.database);
  }

  initFromRoute();
});

onUnmounted(() => {
  syncCurrentTabData();
  stopPolling();
  editorView?.destroy();
  document.removeEventListener('mousemove', onResize);
  document.removeEventListener('mouseup', stopResize);
  window.removeEventListener('beforeunload', handleBeforeUnload);
});

const handleBeforeUnload = () => {
  syncCurrentTabData();
};
window.addEventListener('beforeunload', handleBeforeUnload);
</script>

<style scoped>
.sql-query-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 112px);
  min-height: 0;
  overflow: hidden;
  margin: calc(-1 * var(--space-6));
}

.main-content {
  display: flex;
  flex: 1;
  gap: var(--space-4);
  min-height: 0;
  align-items: stretch;
}

.query-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  min-width: 0;
  min-height: 0;
}

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

@media (max-width: 1200px) {
  .metadata-panel {
    width: 240px;
    min-width: 200px;
  }
}

@media (max-width: 900px) {
  .metadata-panel {
    width: 200px;
    min-width: 160px;
  }
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

.panel-title {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-weight: 600;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

.panel-section {
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.selector-section {
  padding-bottom: var(--space-3);
}

.selector-section.collapsed {
  padding-bottom: var(--space-3);
}

.selector-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  user-select: none;
}

.selector-toggle:hover {
  color: var(--accent-primary);
}

.selector-fields {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding-top: var(--space-3);
}

.selector-field {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.field-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
}

.section-label {
  display: block;
  font-size: var(--font-size-sm);
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: var(--space-2);
}

.field-select {
  width: 100%;
}

.type-tag {
  margin-left: var(--space-2);
  font-size: 12px;
}

.tag-mysql { background: rgba(250, 146, 58, 0.15); color: #fa923a; }
.tag-hive { background: rgba(232, 121, 249, 0.15); color: #e879f9; }
.tag-trino { background: rgba(139, 92, 246, 0.15); color: #8b5cf6; }
.tag-spark { background: rgba(255, 152, 0, 0.15); color: #ff9800; }
.tag-kyuubi { background: rgba(6, 182, 212, 0.15); color: #06b6d4; }
.tag-starrocks { background: rgba(52, 211, 153, 0.15); color: #34d399; }
.tag-doris { background: rgba(236, 72, 153, 0.15); color: #ec4899; }
.tag-sqlite { background: rgba(107, 114, 128, 0.15); color: #6b7280; }
.tag-rqlite { background: rgba(34, 197, 94, 0.15); color: #22c55e; }

.columns-section {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 200px;
}

.columns-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  padding: var(--space-4);
  color: var(--text-muted);
  font-size: var(--font-size-sm);
}

.columns-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-height: 0;
}

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

.column-item:hover {
  background: var(--bg-hover);
  border-color: var(--border-primary);
}

.column-icon {
  color: var(--accent-primary);
  flex-shrink: 0;
}

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

.column-type {
  font-size: 0.75rem;
  color: var(--text-muted);
  background: var(--bg-card);
  padding: 2px var(--space-2);
  border-radius: var(--radius-xs);
  flex-shrink: 0;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  color: var(--text-muted);
}

.empty-state p {
  margin-top: var(--space-2);
  font-size: var(--font-size-sm);
}

.query-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  min-width: 0;
  min-height: 0;
}

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
.toolbar-right {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  align-items: center;
}

.toolbar-left .el-divider--vertical {
  height: 1.2em;
  align-self: center;
}

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

.refresh-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

.refresh-btn:disabled {
  opacity: 0.5;
  transform: none;
  box-shadow: none;
}

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

.create-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

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

.cancel-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(239, 68, 68, 0.4);
  filter: brightness(1.05);
}

.cancel-btn:disabled {
  opacity: 0.5;
  transform: none;
  box-shadow: none;
}

.dml-warning {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  font-size: var(--btn-font-size);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-md);
}

.editor-container {
  position: relative;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  flex-shrink: 0;
}

.editor-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

.tab-bar {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.tabs-scroll {
  display: flex;
  align-items: center;
  overflow-x: auto;
  flex: 1;
  min-width: 0;
  scrollbar-width: none;
}

.tabs-scroll::-webkit-scrollbar {
  display: none;
}

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

.tab-item:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.tab-item.active {
  color: var(--accent-primary);
  background: var(--bg-card);
  font-weight: 500;
}

.tab-item.active::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--accent-primary);
}

.tab-name {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tab-close {
  color: var(--text-muted);
  border-radius: 50%;
  padding: 2px;
  transition: all var(--duration-fast);
  flex-shrink: 0;
}

.tab-close:hover {
  color: var(--accent-danger);
  background: rgba(239, 68, 68, 0.1);
}

.tab-add-btn {
  color: var(--text-muted);
  padding: var(--btn-padding-y) var(--space-2);
  flex-shrink: 0;
  transition: color var(--duration-fast);
}

.tab-add-btn:hover {
  color: var(--accent-primary);
}

.editor-meta {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding-right: var(--space-4);
  flex-shrink: 0;
}

.sql-length {
  font-size: var(--font-size-xs);
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.editor-wrapper {
  position: relative;
  background: #282c34;
}

.sql-editor {
  height: 100%;
}

.resize-handle {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 100%;
  height: 6px;
  cursor: s-resize;
  z-index: 10;
}

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

.resize-handle:hover::after {
  background: rgba(255, 255, 255, 0.5);
}

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

.result-header {
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-secondary);
}

.result-info {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.result-meta {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.cache-badge {
  background: rgba(251, 191, 36, 0.1);
  color: var(--accent-warning);
  padding: 1px var(--space-2);
  border-radius: var(--radius-sm);
}

.truncated-hint {
  font-size: 0.6875rem;
  color: var(--accent-warning);
  margin-left: var(--space-1);
}

.error-text {
  color: var(--accent-danger);
}

.result-body {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.table-container {
  flex: 1;
  overflow: auto;
}

.result-table {
  --el-table-header-text-color: var(--text-primary);
  --el-table-row-hover-bg-color: var(--bg-hover);
}

.empty-result {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
}

.empty-result p {
  margin-top: var(--space-3);
}

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

.empty-query h3 {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: var(--space-2);
}

.empty-query p {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  margin-bottom: var(--space-4);
}

.empty-hint {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--font-size-sm);
  color: var(--text-muted);
}

.save-form {
  padding: var(--space-2);
}

.form-hint {
  font-size: var(--font-size-xs);
  color: var(--text-muted);
  margin-left: var(--space-2);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}

.history-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 400px;
}

.history-sql {
  font-family: var(--font-mono);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-primary);
}

.history-time {
  font-size: 11px;
  color: var(--text-muted);
}
</style>
