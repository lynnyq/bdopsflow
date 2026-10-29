<!--
  任务管理页面
  功能：HTTP/Shell 类型调度任务的 CRUD、启停控制、手动触发、执行日志查看。
  顶部统计卡片 + 筛选工具栏 + 任务卡片网格 + 侧边执行日志面板。
-->
<template>
  <!-- === .tasks-page 区块 === -->
  <div class="tasks-page">
    <!-- === .main-content 区块 === -->
    <div class="main-content">
      <!-- === 主任务区域（包含统计、筛选、卡片列表） === -->
      <div class="tasks-section" :class="{ 'with-logs': showLogs }">
        <!-- === 顶部统计卡片 === -->
        <div class="stats-grid">
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-primary">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><List /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ total }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">总任务数</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-success">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><CircleCheck /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ enabledTasks }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">启用</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-warning">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><Clock /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ cronTasks }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">定时任务</div>
        </div>
      </div>
      <!-- === .stat-card 区块 === -->
      <div class="stat-card">
        <!-- === .stat-icon 区块 === -->
        <div class="stat-icon stat-icon-info">
          <!-- === 图标组件 === -->
          <el-icon :size="24"><Timer /></el-icon>
        </div>
        <!-- === .stat-content 区块 === -->
        <div class="stat-content">
          <!-- === .stat-value 区块 === -->
          <div class="stat-value">{{ manualTasks }}</div>
          <!-- === .stat-label 区块 === -->
          <div class="stat-label">手动任务</div>
        </div>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="page-toolbar">
      <!-- === 工具栏左侧：搜索框 + 类型/状态/创建者筛选 === -->
      <div class="toolbar-left">
        <!-- === 输入框组件 === -->
        <el-input
          v-model="searchQuery"
          placeholder="搜索任务..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <!-- === 下拉选择组件 === -->
        <el-select v-model="filterType" placeholder="任务类型" clearable class="filter-select">
          <!-- === 下拉选项 === -->
          <el-option label="HTTP" value="http" />
          <!-- === 下拉选项 === -->
          <el-option label="Shell" value="shell" />
        </el-select>
        <!-- === 下拉选择组件 === -->
        <el-select v-model="filterStatus" placeholder="状态" clearable class="filter-select">
          <!-- === 下拉选项 === -->
          <el-option label="启用" :value="true" />
          <!-- === 下拉选项 === -->
          <el-option label="停用" :value="false" />
        </el-select>
        <!-- === 下拉选择组件 === -->
        <el-select v-model="filterCreatedBy" placeholder="创建者" clearable class="filter-select">
          <!-- === 下拉选项 === -->
          <el-option
            v-for="user in users"
            :key="user.id"
            :label="user.real_name || user.username"
            :value="user.id"
          />
        </el-select>
      </div>
      <!-- === 工具栏右侧：刷新 + 创建按钮（仅管理员可见创建） === -->
      <div class="toolbar-right">
        <!-- === 按钮组件 === -->
        <el-button :icon="Refresh" @click="loadTasks" :loading="loading" class="refresh-btn">刷新</el-button>
        <!-- === 条件渲染区块 === -->
        <el-button v-if="canManageTask" :icon="Plus" @click="handleCreate" class="create-btn">
          创建任务
        </el-button>
      </div>
    </div>

    <!-- Tasks Grid：可加载的卡片网格，每个卡片包含头部/主体/底部三个区域 -->
    <div v-loading="loading" class="tasks-grid" ref="tasksGridRef">
      <div
        v-for="task in tasks"
        :key="task.id"
        :ref="el => setTaskCardRef(el, task.id)"
        class="task-card"
        :class="{
          'task-card-disabled': !task.is_enabled,
          'task-card-highlighted': selectedTaskId === task.id
        }"
      >
        <!-- 任务卡片头部：类型图标 + 名称/ID/创建者 + 启停开关 -->
        <div class="task-card-header">
          <!-- === .task-header-left 区块 === -->
          <div class="task-header-left" @click="handleCardClick(task)">
            <!-- 任务类型图标徽章 -->
            <div class="task-type-badge" :class="task.type">
              <!-- === 图标组件 === -->
              <el-icon :size="18">
                <!-- === 动态组件 === -->
                <component :is="getTypeIcon(task.type)" />
              </el-icon>
            </div>
            <!-- 任务标题信息：名称、ID、创建者 -->
            <div class="task-title-info">
              <h3 class="task-name">{{ task.name }}</h3>
              <p class="task-id">ID: {{ task.id }}</p>
              <!-- === 条件渲染区块 === -->
              <p class="task-creator" v-if="task.created_by_name">
                <!-- === 图标组件 === -->
                <el-icon><UserIcon /></el-icon>
                {{ task.created_by_name }}
              </p>
            </div>
          </div>
          <!-- 任务启停开关（管理员可见 Switch，普通用户只读 Tag） -->
          <div class="task-status-toggle">
            <!-- === 开关组件 === -->
            <el-switch
              v-if="canManageTask"
              v-model="task.is_enabled"
              @change="() => handleToggleStatus(task)"
              :loading="toggleLoading === task.id"
              size="small"
            />
            <!-- === else 分支区块 === -->
            <el-tag v-else :type="task.is_enabled ? 'success' : 'info'" size="small">
              {{ task.is_enabled ? '启用' : '停用' }}
            </el-tag>
          </div>
        </div>

        <!-- 任务卡片主体：元信息、执行状态、时间戳 -->
        <div class="task-card-body" @click="handleCardClick(task)">

          <!-- 任务元信息：Cron 表达式、类型（定时/手动）、超时时间、下次执行 -->
          <div class="task-meta">
            <!-- === 条件渲染区块 === -->
            <div class="meta-item" v-if="task.cron_expression">
              <!-- === 图标组件 === -->
              <el-icon><Calendar /></el-icon>
              <span class="meta-value">{{ task.cron_expression }}</span>
            </div>
            <!-- === .meta-item 区块 === -->
            <div class="meta-item">
              <!-- === 图标组件 === -->
              <el-icon><Clock /></el-icon>
              <span class="meta-value">{{ task.cron_expression ? '定时' : '手动' }}</span>
            </div>
            <!-- === .meta-item 区块 === -->
            <div class="meta-item">
              <!-- === 图标组件 === -->
              <el-icon><Timer /></el-icon>
              <span class="meta-value">
                {{ task.timeout_seconds }}s 超时
              </span>
            </div>
            <!-- === 条件渲染区块 === -->
            <div class="meta-item meta-item-next-run" v-if="task.is_enabled && task.cron_expression && task.next_execution_time">
              <!-- === 图标组件 === -->
              <el-icon><Timer /></el-icon>
              <span class="meta-value">{{ formatRelativeTime(task.next_execution_time) }}</span>
            </div>
          </div>

          <!-- 任务执行信息：下次执行时间 + 上次执行状态 -->
          <div class="task-execution">
            <!-- === 条件渲染区块 === -->
            <div class="execution-info" v-if="task.is_enabled && task.cron_expression && task.next_execution_time">
              <!-- === .execution-label 区块 === -->
              <div class="execution-label">下次执行</div>
              <!-- === .execution-time 区块 === -->
              <div class="execution-time">
                <!-- === 图标组件 === -->
                <el-icon :size="14"><Timer /></el-icon>
                {{ formatDateTime(task.next_execution_time) }}
              </div>
            </div>
            <!-- === 条件渲染区块 === -->
            <div class="execution-info" v-if="task.last_execution_status">
              <!-- === .execution-label 区块 === -->
              <div class="execution-label">上次执行</div>
              <!-- === .execution-result 区块 === -->
              <div class="execution-result" :class="getResultClass(task.last_execution_status)">
                <span class="result-dot"></span>
                {{ getResultText(task.last_execution_status) }}
              </div>
            </div>
          </div>

          <!-- 创建/更新时间戳 -->
          <div class="task-timestamps">
            <!-- === .timestamp-item 区块 === -->
            <div class="timestamp-item">
              <!-- === 图标组件 === -->
              <el-icon :size="12"><Clock /></el-icon>
              <span>创建: {{ formatDateTime(task.created_at) }}</span>
            </div>
            <!-- === 条件渲染区块 === -->
            <div class="timestamp-item" v-if="task.updated_at !== task.created_at">
              <!-- === 图标组件 === -->
              <el-icon :size="12"><Refresh /></el-icon>
              <span>更新: {{ formatDateTime(task.updated_at) }}</span>
            </div>
          </div>
        </div>

        <!-- 任务卡片底部：操作按钮（运行、日志、历史、编辑、删除） -->
        <div class="task-card-footer">
          <!-- === .task-actions 区块 === -->
          <div class="task-actions">
            <!-- === 按钮组件 === -->
            <el-button
              type="primary"
              size="small"
              :icon="VideoPlay"
              @click="handleTrigger(task)"
              :loading="triggeringId === task.id"
            >
              运行
            </el-button>
            <!-- === 按钮组件 === -->
            <el-button
              size="small"
              :icon="View"
              @click="handleViewLastExecution(task)"
            >
              日志
            </el-button>
            <!-- === 按钮组件 === -->
            <el-button
              size="small"
              :icon="DocumentCopy"
              @click="handleViewHistory(task)"
            >
              历史
            </el-button>
            <!-- === 按钮组件 === -->
            <el-button
              v-if="canManageTask"
              size="small"
              :icon="Edit"
              @click="handleEdit(task)"
            >
              编辑
            </el-button>
            <!-- === 按钮组件 === -->
            <el-button
              v-if="canManageTask"
              size="small"
              :icon="Delete"
              type="danger"
              @click="handleDelete(task)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>

      <!-- Empty State：无任务时的引导空状态 -->
      <div v-if="!loading && tasks.length === 0" class="empty-state">
        <!-- === .empty-icon 区块 === -->
        <div class="empty-icon">
          <!-- === 图标组件 === -->
          <el-icon :size="64"><Document /></el-icon>
        </div>
        <!-- === .empty-text 区块 === -->
        <div class="empty-text">
          <h3>暂无任务</h3>
          <p>点击右上角按钮创建第一个任务</p>
        </div>
      </div>
    </div>

    <!-- Pagination：后端分页，支持切换每页条数 10/20/50/100 -->
    <div v-if="total > 0" class="pagination-container">
      <!-- === 分页器组件 === -->
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        :pager-count="5"
      />
    </div>

    </div>

    <!-- Logs Section：右侧执行日志面板，支持 TaskLogViewer 组件详情或 Tab 切换展示占位 -->
    <div class="logs-section" v-if="showLogs">
      <TaskLogViewer
        v-if="selectedExecutionId"
        :execution-id="selectedExecutionId"
        :execution-status="selectedExecutionStatus"
        :output="selectedExecutionOutput ?? undefined"
        :error="selectedExecutionError ?? undefined"
        @close="closeLogs"
      />
      <!-- === else 分支区块 === -->
      <div v-else class="no-execution-container">
        <!-- === .log-header 区块 === -->
        <div class="log-header">
          <!-- === .log-title 区块 === -->
          <div class="log-title">
            <!-- === 图标组件 === -->
            <el-icon><Document /></el-icon>
            <span>执行详情</span>
          </div>
          <!-- === .log-actions 区块 === -->
          <div class="log-actions">
            <!-- === 按钮组件 === -->
            <el-button :icon="Close" size="small" text @click="closeLogs">关闭</el-button>
          </div>
        </div>
        <!-- === Tabs 标签页 === -->
        <el-tabs class="log-tabs">
          <!-- === 标签页面板 === -->
          <el-tab-pane label="执行日志" name="logs">
            <!-- === .log-body 区块 === -->
            <div class="log-body">
              <!-- === .log-empty 区块 === -->
              <div class="log-empty">
                <!-- === 图标组件 === -->
                <el-icon><InfoFilled /></el-icon>
                <span>暂无执行记录</span>
              </div>
            </div>
          </el-tab-pane>
          <!-- === 标签页面板 === -->
          <el-tab-pane label="标准输出" name="output">
            <!-- === .output-body 区块 === -->
            <div class="output-body">
              <!-- === .log-empty 区块 === -->
              <div class="log-empty">
                <!-- === 图标组件 === -->
                <el-icon><InfoFilled /></el-icon>
                <span>暂无标准输出</span>
              </div>
            </div>
          </el-tab-pane>
          <!-- === 标签页面板 === -->
          <el-tab-pane label="标准错误" name="error">
            <!-- === .output-body 区块 === -->
            <div class="output-body">
              <!-- === .log-empty 区块 === -->
              <div class="log-empty">
                <!-- === 图标组件 === -->
                <el-icon><InfoFilled /></el-icon>
                <span>暂无标准错误</span>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </div>
    </div>

    <!-- Task Form Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingTask ? '编辑任务' : '创建任务'"
      width="800px"
      class="task-dialog"
      :close-on-click-modal="false"
      @close="handleDialogClose"
    >
      <!-- === .task-form-container 区块 === -->
      <div class="task-form-container">
        <!-- === 表单组件 === -->
        <el-form ref="formRef" :model="form" :rules="rules" class="task-form">
          <!-- 基本信息 -->
          <div class="form-section">
            <!-- === .section-header 区块 === -->
            <div class="section-header">
              <!-- === 图标组件 === -->
              <el-icon :size="18"><Document /></el-icon>
              <span>基本信息</span>
            </div>
            <!-- === .section-body 区块 === -->
            <div class="section-body">
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <!-- === 表单组件 === -->
                  <el-form-item label="任务名称" prop="name" class="form-item">
                    <!-- === 输入框组件 === -->
                    <el-input 
                      v-model="form.name" 
                      placeholder="请输入任务名称"
                      class="form-input"
                    >
                      <!-- === 具名插槽 === -->
                      <template #prefix>
                        <!-- === 图标组件 === -->
                        <el-icon><Bell /></el-icon>
                      </template>
                    </el-input>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="任务类型" prop="type" class="form-item">
                    <el-select 
                      v-model="form.type" 
                      placeholder="请选择任务类型" 
                      class="form-select"
                    >
                      <el-option label="HTTP 请求" value="http">
                        <span class="option-content">
                          <el-icon :size="16"><Connection /></el-icon>
                          HTTP 请求
                        </span>
                      </el-option>
                      <el-option label="Shell 脚本" value="shell">
                        <span class="option-content">
                          <el-icon :size="16"><Cpu /></el-icon>
                          Shell 脚本
                        </span>
                      </el-option>
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>

              <!-- 执行器选择 -->
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="执行器" prop="assigned_executor_id" class="form-item">
                    <el-select
                      v-model="form.assigned_executor_id"
                      clearable
                      placeholder="默认调度（自动选择）"
                      class="form-select"
                    >
                      <!-- === 具名插槽 === -->
                      <template #prefix>
                        <!-- === 图标组件 === -->
                        <el-icon><Cpu /></el-icon>
                      </template>
                      <el-option
                        v-for="executor in safeExecutors"
                        :key="executor.id"
                        :label="`${executor.name} (${executor.current_load}/${executor.capacity})`"
                        :value="executor.id"
                      >
                        <div class="executor-option">
                          <span>{{ executor.name }}</span>
                          <span class="executor-status" :class="executor.status">
                            {{ executor.status === 'online' ? '在线' : '离线' }}
                          </span>
                          <span class="executor-load">
                            负载: {{ executor.current_load }}/{{ executor.capacity }}
                          </span>
                        </div>
                      </el-option>
                    </el-select>
                    <div class="form-tip">
                      <el-icon><InfoFilled /></el-icon>
                      <span>留空则使用默认调度算法，指定执行器则任务将在该执行器上执行</span>
                    </div>
                  </el-form-item>
                </el-col>
              </el-row>
            </div>
          </div>

          <!-- 调度配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Clock /></el-icon>
              <span>调度配置</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="执行频率" prop="cron_expression" class="form-item">
                    <div class="cron-wrapper">
                      <el-select 
                        v-model="cronPreset" 
                        placeholder="选择预设" 
                        class="cron-preset-select"
                        @change="handleCronPresetChange"
                      >
                        <el-option label="手动触发" value="manual">
                          <span class="option-content">
                            <el-icon :size="14"><DataAnalysis /></el-icon>
                            手动触发
                          </span>
                        </el-option>
                        <el-option label="每30秒" value="30sec">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每30秒
                          </span>
                        </el-option>
                        <el-option label="每分钟" value="minute">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每分钟
                          </span>
                        </el-option>
                        <el-option label="每5分钟" value="5minute">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每5分钟
                          </span>
                        </el-option>
                        <el-option label="每10分钟" value="10minute">
                          <span class="option-content">
                            <el-icon :size="14"><Timer /></el-icon>
                            每10分钟
                          </span>
                        </el-option>
                        <el-option label="每小时" value="hour">
                          <span class="option-content">
                            <el-icon :size="14"><Clock /></el-icon>
                            每小时
                          </span>
                        </el-option>
                        <el-option label="每天" value="day">
                          <span class="option-content">
                            <el-icon :size="14"><Calendar /></el-icon>
                            每天 00:00
                          </span>
                        </el-option>
                        <el-option label="每周" value="week">
                          <span class="option-content">
                            <el-icon :size="14"><Calendar /></el-icon>
                            每周一 00:00
                          </span>
                        </el-option>
                        <el-option label="每月" value="month">
                          <span class="option-content">
                            <el-icon :size="14"><Calendar /></el-icon>
                            每月1日 00:00
                          </span>
                        </el-option>
                        <el-option label="自定义" value="custom">
                          <span class="option-content">
                            <el-icon :size="14"><Setting /></el-icon>
                            自定义表达式
                          </span>
                        </el-option>
                      </el-select>
                    </div>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="Cron 值" prop="cron_expression" class="form-item">
                    <el-input 
                      v-model="form.cron_expression" 
                      :placeholder="cronPlaceholder"
                      :disabled="cronPreset !== 'custom'"
                      class="form-input"
                    >
                      <!-- === 具名插槽 === -->
                      <template #suffix>
                        <span class="cron-hint" title="支持5位（分 时 日 月 周）或6位（秒 分 时 日 月 周）格式">秒 分 时 日 月 周</span>
                      </template>
                    </el-input>
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="超时时间" prop="timeout_seconds" class="form-item">
                    <div class="timeout-input-wrapper">
                      <el-input-number
                        v-model="form.timeout_seconds"
                        :min="0"
                        :max="86400"
                        placeholder="秒"
                        class="form-input-number"
                        controls-position="right"
                      >
                      </el-input-number>
                      <span class="timeout-hint">秒 (0=不限制)</span>
                    </div>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <div class="empty-col"></div>
                </el-col>
              </el-row>
            </div>
          </div>

          <!-- 任务配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Setting /></el-icon>
              <span>任务配置</span>
            </div>
            <div class="section-body">
              <!-- HTTP 配置 -->
              <div v-if="form.type === 'http'" class="config-panel">
                <el-row :gutter="16">
                  <el-col :span="24">
                    <el-form-item label="请求 URL" prop="config.url" class="form-item">
                      <el-input 
                        v-model="form.config.url" 
                        placeholder="https://example.com/api"
                        class="form-input"
                      >
                        <!-- === 具名插槽 === -->
                        <template #prefix>
                          <!-- === 图标组件 === -->
                          <el-icon><Connection /></el-icon>
                        </template>
                      </el-input>
                    </el-form-item>
                  </el-col>
                </el-row>
                <el-row :gutter="16">
                  <el-col :span="8">
                    <el-form-item label="请求方法" prop="config.method" class="form-item">
                      <el-select 
                        v-model="form.config.method" 
                        class="form-select"
                      >
                        <el-option label="GET" value="GET" />
                        <el-option label="POST" value="POST" />
                        <el-option label="PUT" value="PUT" />
                        <el-option label="DELETE" value="DELETE" />
                      </el-select>
                    </el-form-item>
                  </el-col>
                  <el-col :span="8">
                    <el-form-item label="连接超时" prop="config.timeout" class="form-item">
                      <el-input-number
                        v-model="form.config.timeout"
                        :min="1"
                        :max="300"
                        placeholder="秒"
                        class="form-input-number"
                      >
                        <!-- === 具名插槽 === -->
                        <template #suffix>
                          <span>秒</span>
                        </template>
                      </el-input-number>
                    </el-form-item>
                  </el-col>
                  <el-col :span="8">
                    <div class="empty-col"></div>
                  </el-col>
                </el-row>
                <el-row :gutter="16">
                  <el-col :span="12">
                    <el-form-item label="请求头" prop="config.headers" class="form-item">
                      <el-input
                        v-model="form.config.headers"
                        type="textarea"
                        :rows="3"
                        placeholder='{"Authorization": "Bearer xxx"}'
                        class="form-textarea"
                      />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="请求体" prop="config.body" class="form-item">
                      <el-input
                        v-model="form.config.body"
                        type="textarea"
                        :rows="3"
                        placeholder="请求体内容（JSON或表单）"
                        class="form-textarea"
                      />
                    </el-form-item>
                  </el-col>
                </el-row>
              </div>

              <!-- Shell 配置 -->
              <div v-if="form.type === 'shell'" class="config-panel">
                <el-form-item label="Shell 脚本" prop="config.script" class="form-item">
                  <div class="script-editor">
                    <el-input
                      v-model="form.config.script"
                      type="textarea"
                      :rows="8"
                      placeholder=""
                      class="form-textarea script-textarea"
                    />
                  </div>
                </el-form-item>
              </div>
            </div>
          </div>

          <!-- 重试配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Refresh /></el-icon>
              <span>重试配置</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="失败重试次数" prop="retry_count" class="form-item">
                    <el-input-number
                      v-model="form.retry_count"
                      :min="0"
                      :max="10"
                      placeholder="重试次数"
                      class="form-input-number"
                    >
                      <!-- === 具名插槽 === -->
                      <template #suffix>
                        <span>次</span>
                      </template>
                    </el-input-number>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="重试间隔" prop="retry_interval" class="form-item">
                    <el-input-number
                      v-model="form.retry_interval"
                      :min="0"
                      :max="3600"
                      placeholder="秒"
                      class="form-input-number"
                      :disabled="form.retry_count === 0"
                    >
                      <!-- === 具名插槽 === -->
                      <template #suffix>
                        <span>秒</span>
                      </template>
                    </el-input-number>
                  </el-form-item>
                </el-col>
              </el-row>
            </div>
          </div>

          <!-- 初始状态 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><SwitchButton /></el-icon>
              <span>初始状态</span>
            </div>
            <div class="section-body">
              <el-form-item label="初始状态" prop="is_enabled" class="form-item">
                <div class="switch-wrapper">
                  <el-switch v-model="form.is_enabled" size="large" />
                  <span class="switch-text">{{ form.is_enabled ? '任务将立即启用' : '任务将处于停用状态' }}</span>
                </div>
              </el-form-item>
            </div>
          </div>

          <!-- Webhook推送配置 -->
          <div class="form-section">
            <div class="section-header">
              <el-icon :size="18"><Connection /></el-icon>
              <span>Webhook推送配置</span>
            </div>
            <div class="section-body">
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="Webhook" prop="webhook_id" class="form-item">
                    <el-select 
                      v-model="form.webhook_id" 
                      placeholder="选择Webhook"
                      class="form-select"
                      clearable
                      filterable
                    >
                      <el-option 
                        v-for="wh in webhookList" 
                        :key="wh.id" 
                        :label="wh.name" 
                        :value="wh.id" 
                      />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16" v-if="form.webhook_id">
                <el-col :span="24">
                  <el-form-item label="推送时机" prop="webhook_events" class="form-item">
                    <el-select 
                      v-model="form.webhook_events" 
                      multiple
                      placeholder="选择推送时机"
                      class="form-select"
                    >
                      <el-option label="任务成功" value="success" />
                      <el-option label="任务失败" value="failed" />
                      <el-option label="任务跳过" value="skipped" />
                      <el-option label="每次执行" value="*" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
            </div>
          </div>
        </el-form>
      </div>

      <!-- === 具名插槽 === -->
      <template #footer>
        <!-- === .dialog-footer 区块 === -->
        <div class="dialog-footer">
          <!-- === 按钮组件 === -->
          <el-button @click="dialogVisible = false">取消</el-button>
          <!-- === 按钮组件 === -->
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            {{ editingTask ? '更新' : '创建' }}
          </el-button>
        </div>
      </template>
    </el-dialog>


  </div>
</template>

<script setup lang="ts">
/**
 * 任务管理页面
 * 管理 HTTP/Shell 类型的调度任务，支持 CRUD、启停、手动执行和日志查看
 * @props 无 Props
 * @emits 无 Emits
 */
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, FormInstance } from 'element-plus'
import {
  Plus,
  List,
  Edit,
  Delete,
  View,
  Document,
  Search,
  Refresh,
  Timer,
  Clock,
  Calendar,
  VideoPlay,
  CircleCheck,
  DocumentCopy,
  Close,
  InfoFilled,
  Tools,
  Promotion,
  Connection,
  Cpu,
  Bell,
  Setting,
  SwitchButton,
  DataAnalysis,
  User as UserIcon
} from '@element-plus/icons-vue'
import { taskAPI, executorAPI, webhookAPI, userAdminAPI } from '@/api'
import type { Task, TaskConfig, Executor, Webhook, User } from '@/types'
import TaskLogViewer from '@/components/TaskLogViewer.vue'
import { isHandledError } from '@/utils/api'
import { useAuthStore } from '@/stores/auth'

/** router: Vue Router 实例 */
const router = useRouter()  // Vue Router 实例，用于页面跳转
const authStore = useAuthStore()  // 认证状态管理（用户信息、权限）

// ========== 辅助数据 ==========
const webhookList = ref<Webhook[]>([])  // 当前域可用的 Webhook 列表（用于任务配置下拉选择）

/**
 * 加载当前域可用的 Webhook 列表（仅加载已启用的）
 * 系统管理员不限制 domainId，普通用户使用当前域
 * @returns {Promise<void>}
 */
const loadWebhooks = async () => {
  const domainId = authStore.isSystemAdmin ? undefined : (authStore.currentDomainId || 1)
  /** 异常处理 */
  try {
    const response = await webhookAPI.list(domainId)
    webhookList.value = (response.data?.items || []).filter((w: Webhook) => w.is_enabled)
  } catch {
    // ignore
  }
}

/** 任务列表 */
// ========== 任务列表状态 ==========
const tasks = ref<Task[]>([])                // 当前页的任务列表
const executors = ref<Executor[]>([])        // 执行器列表（用于下拉选择）
/** 是否正在加载 */
const loading = ref(false)
/** submitting 响应式引用 */
const submitting = ref(false)                // 表单提交中
const toggleLoading = ref<number | null>(null)  // 正在切换启用/停用的任务 ID
const triggeringId = ref<number | null>(null)   // 正在手动触发的任务 ID
/** dialogVisible 响应式引用 */
const dialogVisible = ref(false)             // 创建/编辑任务对话框是否可见
const editingTask = ref<Task | null>(null)   // 当前正在编辑的任务（null 表示新建）
const formRef = ref<FormInstance>()          // 表单 DOM 引用
/** 搜索关键词 */
// ========== 基础状态 ==========
const searchQuery = ref('')
/** 类型筛选 */
const filterType = ref<string | null>(null)
/** 启用状态筛选 */
const filterStatus = ref<boolean | null>(null)
/** 创建人筛选 */
const filterCreatedBy = ref<number | null>(null)
const users = ref<User[]>([])                // 用户列表（用于创建人筛选）
/** currentPage 响应式引用 */
const currentPage = ref(1)                   // 当前页码
/** pageSize 响应式引用 */
const pageSize = ref(20)                     // 每页条数
// 后端返回的真实总数，用于分页器
// ========== 统计数据 ==========
const total = ref(0)                         // 符合筛选条件的任务总数

/** 是否显示日志面板 */
// ========== 日志面板状态 ==========
const showLogs = ref(false)                  // 是否显示右侧执行日志面板
const selectedTaskId = ref<number | null>(null)  // 当前选中查看日志的任务 ID
/** selectedTaskName 响应式引用 */
const selectedTaskName = ref('')             // 当前选中任务名称（日志面板标题）
const selectedExecutionId = ref<string | null>(null)  // 当前查看的执行记录 ID
const selectedExecutionStatus = ref<string | undefined>(undefined)
const selectedExecutionOutput = ref<string | null | undefined>(undefined)
const selectedExecutionError = ref<string | null | undefined>(undefined)
// ========== 任务列表状态 ==========
const tasksGridRef = ref<HTMLElement | null>(null)   // 任务卡片网格 DOM 引用
const taskCardRefs = ref<Map<number, HTMLElement>>(new Map())  // 每个任务卡片的 DOM 引用 Map

// ========== Cron 表达式预设 ==========
const cronPreset = ref('manual')   // Cron 预设选项：manual / every_minute / hourly / daily / weekly / monthly / custom

// cronPlaceholder
const cronPlaceholder = computed(() => {
  /** 条件分支 */
  if (cronPreset.value === 'manual') return '手动触发'
  /** 条件分支 */
  if (cronPreset.value === 'custom') return '输入 Cron 表达式（支持5位或6位）'
  /** 返回/抛出 */
  return form.value.cron_expression || ''
})

// 安全过滤执行器列表，确保所有元素都有有效的 id
const safeExecutors = computed(() => {
  const list = Array.isArray(executors.value) ? executors.value : []
  /** 返回/抛出 */
  return list.filter(exec => exec && typeof exec.id !== 'undefined' && exec.id !== null)
})

/**
 * Cron 预设变化处理：将预设名称转换为对应的 cron 表达式写入 form
 * @param {string} preset - 预设名称（manual/30sec/minute/5minute/10minute/hour/day/week/month/custom）
 */
const handleCronPresetChange = (preset: string) => {
  const presets: Record<string, string> = {
    manual: '',
    '30sec': '*/30 * * * * *',
    minute: '* * * * *',
    '5minute': '*/5 * * * *',
    '10minute': '*/10 * * * *',
    hour: '0 * * * *',
    day: '0 0 * * *',
    week: '0 0 * * 1',
    month: '0 0 1 * *',
    custom: form.value.cron_expression || ''
  }
  /** 条件分支 */
  if (preset !== 'custom') {
    form.value.cron_expression = presets[preset] || ''
  }
}

/**
 * 动态设置任务卡片的 DOM ref，用于后续 scrollIntoView 滚动定位
 * @param {HTMLElement | null} el - 任务卡片 DOM 元素（v-for 回调中会传 null）
 * @param {number} taskId - 任务 ID
 */
const setTaskCardRef = (el: any, taskId: number) => {
  /** 条件分支 */
  if (el) {
    taskCardRefs.value.set(taskId, el)
  } else {
    taskCardRefs.value.delete(taskId)
  }
}

/** defaultForm 对象常量 */
const defaultForm = {
  name: '',
  type: 'shell' as string,
  timeout_seconds: 3600,
  cron_expression: '',
  config: {
    url: '',
    method: 'GET' as const,
    timeout: 30,
    headers: '',
    body: '',
    script: ''
  } as TaskConfig,
  retry_count: 0,
  retry_interval: 5,
  is_enabled: true,
  webhook_id: null as number | null,
  webhook_events: [] as string[],
  assigned_executor_id: null as number | null,
  domain_id: 1 as number
}

/** form 响应式引用 */
const form = ref({ ...defaultForm })

// rules
const rules = computed(() => {
  /** baseRules 变量声明 */
  const baseRules: any = {
    name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
    type: [{ required: true, message: '请选择任务类型', trigger: 'change' }]
  }

  /** 条件分支 */
  if (form.value.type === 'http') {
    baseRules['config.url'] = [
      { required: true, message: '请输入请求 URL', trigger: 'blur' }
    ]
  }

  /** 条件分支 */
  if (form.value.type === 'shell') {
    baseRules['config.script'] = [
      { required: true, message: '请输入 Shell 脚本', trigger: 'blur' }
    ]
  }

  /** 返回/抛出 */
  return baseRules
})

// enabledTasks
const enabledTasks = computed(() => tasks.value.filter(t => t.is_enabled).length)
// cronTasks
const cronTasks = computed(() => tasks.value.filter(t => t.cron_expression).length)
// manualTasks
const manualTasks = computed(() => tasks.value.filter(t => !t.cron_expression).length)

// nManageTask
const canManageTask = computed(() => {
  /** 返回/抛出 */
  return authStore.hasPermission('task', 'create') || authStore.hasPermission('task', 'manage')
})

/**
 * 根据任务类型返回对应的图标组件（HTTP→Promotion，Shell→Tools）
 * @param {string} type - 任务类型（http/shell）
 * @returns {Object} Element Plus 图标组件
 */
const getTypeIcon = (type: string) => {
  /** 返回/抛出 */
  return type === 'http' ? Promotion : Tools
}

/**
 * 将 ISO 日期字符串格式化为 zh-CN 完整日期时间显示（如 "2026/10/29 14:30:00"）
 * @param {string} dateStr - ISO 日期字符串
 * @returns {string} 格式化后的日期时间字符串，空输入返回 "-"
 */
const formatDateTime = (dateStr: string) => {
  /** 条件分支 */
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  /** 返回/抛出 */
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
 * 将 ISO 日期字符串格式化为相对时间（如 "5分钟后"、"即将执行"）
 * 用于任务下次执行时间的友好显示
 * @param {string} dateStr - ISO 日期字符串
 * @returns {string} 格式化后的相对时间字符串
 */
const formatRelativeTime = (dateStr: string) => {
  /** 条件分支 */
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = date.getTime() - now.getTime()
  /** 条件分支 */
  if (diffMs <= 0) return '即将执行'
  const diffSec = Math.floor(diffMs / 1000)
  /** 条件分支 */
  if (diffSec < 60) return `${diffSec}秒后`
  const diffMin = Math.floor(diffSec / 60)
  /** 条件分支 */
  if (diffMin < 60) return `${diffMin}分钟后`
  const diffHour = Math.floor(diffMin / 60)
  /** 条件分支 */
  if (diffHour < 24) return `${diffHour}小时后`
  const diffDay = Math.floor(diffHour / 24)
  /** 返回/抛出 */
  return `${diffDay}天后`
}

/**
 * 根据执行状态返回对应的 CSS class（success/failed/running/pending/none）
 * @param {string} status - 执行状态字符串
 * @returns {string} CSS 类名
 */
const getResultClass = (status: string) => {
  const map: Record<string, string> = {
    success: 'success',
    failed: 'failed',
    running: 'running',
    pending: 'pending'
  }
  /** 返回/抛出 */
  return map[status] || 'none'
}

/**
 * 根据执行状态返回对应的中文显示文本（成功/失败/运行中/等待中/未执行）
 * @param {string} status - 执行状态字符串
 * @returns {string} 中文状态文本
 */
const getResultText = (status: string) => {
  const map: Record<string, string> = {
    success: '成功',
    failed: '失败',
    running: '运行中',
    pending: '等待中'
  }
  /** 返回/抛出 */
  return map[status] || '未执行'
}

/**
 * 把后端返回的 config 字段（可能是 JSON 字符串或对象）统一解析为 TaskConfig
 * @param {string | TaskConfig} config - 序列化或已解析的配置
 * @returns {TaskConfig} 解析后的任务配置对象；解析失败返回空对象
 */
const parseConfig = (config: string | TaskConfig): TaskConfig => {
  /** 条件分支 */
  if (typeof config === 'string') {
    /** 异常处理 */
    try {
      /** 返回/抛出 */
      return JSON.parse(config)
    } catch {
      /** 返回/抛出 */
      return {}
    }
  }
  /** 返回/抛出 */
  return config || {}
}

/**
 * 把前端提交的 config 统一序列化为 JSON 字符串（后端 API 要求字符串类型）
 * @param {TaskConfig | string} config - 任务配置对象或已序列化字符串
 * @returns {string} JSON 字符串
 */
const stringifyConfig = (config: TaskConfig | string): string => {
  /** 条件分支 */
  if (typeof config === 'string') {
    /** 返回/抛出 */
    return config
  }
  /** 返回/抛出 */
  return JSON.stringify(config)
}

/**
 * 分页加载任务列表
 * 支持搜索关键词、类型筛选、启用状态筛选、创建人筛选
 */
// ========== 数据加载 ==========
const loadTasks = async () => {
  loading.value = true
  /** 异常处理 */
  try {
    // 后端分页：将分页与过滤参数一并发给后端
    const params: {
      page: number
      page_size: number
      search?: string
      type?: string
      is_enabled?: boolean
      created_by?: number
    } = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    /** 条件分支 */
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    /** 条件分支 */
    if (filterType.value) {
      params.type = filterType.value
    }
    /** 条件分支 */
    if (filterStatus.value !== null) {
      params.is_enabled = filterStatus.value
    }
    /** 条件分支 */
    if (filterCreatedBy.value) {
      params.created_by = filterCreatedBy.value
    }
    const res = await taskAPI.list(params)
    total.value = res.data.total || 0
    tasks.value = (res.data.items || []).map(task => {
      /** 条件分支 */
      if (task.config && typeof task.config === 'string') {
        /** 异常处理 */
        try {
          /** 返回/抛出 */
          return {
            ...task,
            config: JSON.parse(task.config)
          }
        } catch {
        }
      }
      /** 返回/抛出 */
      return task
    })
  } catch (err: any) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载任务列表失败')
    }
  } finally {
    loading.value = false
  }
}

/**
 * 打开新建任务对话框，重置表单为默认值并带入当前域
 */
// ========== 任务 CRUD ==========
const handleCreate = () => {
  editingTask.value = null
  const userDomainId = authStore.currentDomainId || 1
  form.value = { ...defaultForm, domain_id: userDomainId }
  cronPreset.value = 'manual'
  loadWebhooks()
  dialogVisible.value = true
}

/**
 * 点击任务卡片的事件处理：有管理权限则打开编辑对话框
 * （无权限时不做操作，避免误打开）
 * @param {Task} task - 被点击的任务对象
 * @returns {void}
 */
const handleCardClick = (task: Task) => {
  /** 条件分支 */
  if (canManageTask.value) {
    handleEdit(task)
  }
}

/**
 * 打开编辑对话框：把任务数据回填到表单（包括 config 解析、webhook_events 解析、cron 预设匹配）
 * @param {Task} task - 要编辑的任务对象
 */
const handleEdit = (task: Task) => {
  editingTask.value = task
  loadWebhooks()
  const parsedConfig = parseConfig(task.config)
  
  /** webhookEvents: 可变变量 */
  let webhookEvents: string[] = []
  /** 条件分支 */
  if (task.webhook_events) {
    /** 异常处理 */
    try {
      webhookEvents = typeof task.webhook_events === 'string' ? JSON.parse(task.webhook_events) : task.webhook_events
    } catch {
      webhookEvents = []
    }
  }
  
  form.value = {
    name: task.name,
    type: task.type,
    timeout_seconds: task.timeout_seconds,
    cron_expression: task.cron_expression || '',
    config: { ...parsedConfig },
    retry_count: task.retry_count,
    retry_interval: task.retry_interval,
    is_enabled: task.is_enabled,
    webhook_id: task.webhook_id,
    webhook_events: webhookEvents,
    assigned_executor_id: task.assigned_executor_id || null,
    domain_id: task.domain_id
  }
  
  // 根据当前任务的 cron 表达式设置预设
  const cron = task.cron_expression || ''
  /** 条件分支 */
  if (!cron) {
    cronPreset.value = 'manual'
  } else if (cron === '* * * * *') {
    cronPreset.value = 'minute'
  } else if (cron === '*/5 * * * *') {
    cronPreset.value = '5minute'
  } else if (cron === '*/10 * * * *') {
    cronPreset.value = '10minute'
  } else if (cron === '0 * * * *') {
    cronPreset.value = 'hour'
  } else if (cron === '0 0 * * *') {
    cronPreset.value = 'day'
  } else if (cron === '0 0 * * 1') {
    cronPreset.value = 'week'
  } else if (cron === '0 0 1 * *') {
    cronPreset.value = 'month'
  } else {
    cronPreset.value = 'custom'
  }
  
  dialogVisible.value = true
}

/**
 * 表单提交：校验后根据 editingTask 调用 create 或 update API
 * 提交前自动序列化 config 和 webhook_events，创建/更新后刷新列表
 * @returns {Promise<void>}
 */
const handleSubmit = async () => {
  /** 条件分支 */
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    /** 条件分支 */
    if (!valid) return
    submitting.value = true
    /** 异常处理 */
    try {
      const userDomainId = authStore.currentDomainId || 1
      /** submitData 对象常量 */
      const submitData = {
        ...form.value,
        config: stringifyConfig(form.value.config),
        webhook_id: form.value.webhook_id || null,
        webhook_events: JSON.stringify(form.value.webhook_events || []),
        domain_id: editingTask.value ? form.value.domain_id : userDomainId
      }
      /** 条件分支 */
      if (editingTask.value) {
        const response = await taskAPI.update(editingTask.value.id, submitData)
        ElMessage.success('任务更新成功')
        // 检查是否有可用的执行器
        if (response.data.has_available_executors === false) {
          ElMessage.warning('当前领域没有可用的执行器，任务已更新但已停用，请联系管理员分配执行器')
        }
      } else {
        const response = await taskAPI.create(submitData)
        ElMessage.success('任务创建成功')
        // 检查是否有可用的执行器
        if (response.data.has_available_executors === false) {
          ElMessage.warning('当前领域没有可用的执行器，任务已创建但已停用，请联系管理员分配执行器')
        }
      }
      dialogVisible.value = false
      await loadTasks()
    } catch (err: any) {
      /** 条件分支 */
      if (!isHandledError(err)) {
        ElMessage.error(err.message || '操作失败')
      }
    } finally {
      submitting.value = false
    }
  })
}

/**
 * 关闭任务创建/编辑对话框，重置表单为 defaultForm 默认值
 * 同时清除 cronPreset 状态，为下次打开新建做准备
 * @returns {void}
 */
const handleDialogClose = () => {
  formRef.value?.resetFields()
  form.value = { ...defaultForm }
  cronPreset.value = 'manual'
}

/**
 * 切换任务启用/停用状态
 * 后端检查领域内是否有可用执行器，若无则自动回退为停用状态并提示用户
 * @param {Task} task - 目标任务对象（is_enabled 字段反映切换后的目标状态）
 * @returns {Promise<void>}
 */
const handleToggleStatus = async (task: Task) => {
  const expectedEnabled = task.is_enabled
  toggleLoading.value = task.id
  /** 异常处理 */
  try {
    const response = await taskAPI.update(task.id, { is_enabled: expectedEnabled })
    const result = response.data as { task?: Task; has_available_executors?: boolean }
    
    /** 条件分支 */
    if (expectedEnabled && result.has_available_executors === false) {
      ElMessage.warning('当前领域没有可用的执行器，任务已自动设置为停用状态，请联系管理员分配执行器')
    } else {
      ElMessage.success(expectedEnabled ? '任务已启用' : '任务已停用')
    }
    await loadTasks()
  } catch (err: any) {
    task.is_enabled = !expectedEnabled
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '操作失败')
    }
  } finally {
    toggleLoading.value = null
  }
}

/**
 * 删除任务：二次确认后调用 delete API，成功后刷新列表
 * @param {Task} task - 要删除的任务对象
 * @returns {Promise<void>}
 */
const handleDelete = async (task: Task) => {
  /** 异常处理 */
  try {
    await ElMessageBox.confirm(`确定要删除任务 "${task.name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await taskAPI.delete(task.id)
    ElMessage.success('任务已删除')
    await loadTasks()
  } catch (err: any) {
    /** 条件分支 */
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(err.message || '删除失败')
    }
  }
}

/**
 * 手动触发任务执行（不改变启用/停用状态）
 * @param {Task} task - 要触发的任务对象
 * @returns {Promise<void>}
 */
const handleTrigger = async (task: Task) => {
  triggeringId.value = task.id
  /** 异常处理 */
  try {
    await taskAPI.trigger(task.id)
    ElMessage.success('任务已触发')
  } catch (err: any) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      const errorMessage = err.response?.data?.error || err.message || '触发失败'
      ElMessage.error(errorMessage)
    }
  } finally {
    triggeringId.value = null
  }
}

/**
 * 查看任务最后一次执行日志：获取执行记录并在右侧面板显示
 * 同时滚动到被点击的任务卡片
 * @param {Task} task - 要查看日志的任务对象
 * @returns {Promise<void>}
 */
const handleViewLastExecution = async (task: Task) => {
  selectedTaskId.value = task.id
  selectedTaskName.value = task.name
  selectedExecutionId.value = null
  selectedExecutionStatus.value = undefined
  selectedExecutionOutput.value = undefined
  selectedExecutionError.value = undefined

  /** 异常处理 */
  try {
    const res = await taskAPI.getExecutions(task.id)
    const executions = res.data || []
    /** 条件分支 */
    if (executions.length > 0) {
      const lastExecution = executions[0]
      selectedExecutionId.value = lastExecution.execution_id || String(lastExecution.id)
      selectedExecutionStatus.value = lastExecution.status
      selectedExecutionOutput.value = lastExecution.output
      selectedExecutionError.value = lastExecution.error
    }
  } catch (err: any) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      ElMessage.error(err.message || '加载执行记录失败')
    }
  } finally {
    showLogs.value = true
    
    // 滚动到被点击的任务卡片
    await nextTick()
    const taskCardEl = taskCardRefs.value.get(task.id)
    /** 条件分支 */
    if (taskCardEl) {
      taskCardEl.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }
}

/**
 * 关闭右侧执行日志面板，清理所有选中状态（任务 ID、执行 ID、输出/错误信息）
 * 下次打开时将重新从后端拉取最新执行记录
 * @returns {void}
 */
const closeLogs = () => {
  showLogs.value = false
  selectedExecutionId.value = null
  selectedExecutionStatus.value = undefined
  selectedExecutionOutput.value = undefined
  selectedExecutionError.value = undefined
  selectedTaskId.value = null
}

/**
 * 跳转到日志页面，自动按任务名称过滤显示历史执行记录
 * @param {Task} task - 要查看历史的任务对象
 */
const handleViewHistory = (task: Task) => {
  router.push({
    path: '/logs',
    query: {
      task_name: task.name
    }
  })
}

// 过滤条件变化：重置到第 1 页并重新请求后端
// searchQuery 加防抖，避免输入过程中频繁请求
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null
/** watch 监听器 */
watch(searchQuery, () => {
  /** 条件分支 */
  if (searchDebounceTimer) {
    clearTimeout(searchDebounceTimer)
  }
  searchDebounceTimer = setTimeout(() => {
    currentPage.value = 1
    loadTasks()
  }, 300)
})

// 监听数据变化
watch([filterType, filterStatus, filterCreatedBy], () => {
  currentPage.value = 1
  loadTasks()
})

// 分页参数变化：重新请求后端对应页
watch([currentPage, pageSize], () => {
  loadTasks()
})

// 当 webhook_id 被清空时，同步清空 webhook_events
watch(() => form.value.webhook_id, (newVal) => {
  /** 条件分支 */
  if (!newVal) {
    form.value.webhook_events = []
  }
})

// 确保 timeout_seconds 始终是数字类型，且 0 值被正确保留
watch(() => form.value.timeout_seconds, (newVal) => {
  /** 条件分支 */
  if (typeof newVal !== 'number') {
    form.value.timeout_seconds = 0
  }
}, { immediate: true })

onMounted // 生命周期钩子
/** 组件挂载生命周期 */
onMounted(async () => {
  await Promise.all([
    loadTasks(),
    loadExecutors(),
    loadUsers()
  ])
})

/**
 * 加载系统用户列表（用于创建者筛选下拉框）
 * @returns {Promise<void>}
 */
const loadUsers = async () => {
  /** 异常处理 */
  try {
    const res = await userAdminAPI.list()
    users.value = res.data.items || []
  } catch {
    // ignore
  }
}

/**
 * 加载执行器列表（用于任务表单中的执行器下拉选择）
 * @returns {Promise<void>}
 */
const loadExecutors = async () => {
  /** 异常处理 */
  try {
    const response = await executorAPI.list()
    executors.value = response.data.items || []
  } catch (error) {
    ElMessage.error('加载执行器列表失败')
  }
}
</script>

/* ==================== 样式定义 ==================== */
<style scoped>
/* 页面整体容器 */
/* === 页面根容器：纵向 flex + 全高可滚动 === */
.tasks-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  padding-bottom: var(--space-8);
  overflow-y: auto;
  height: 100%;
}

/* Main Content */
/* === 主内容区 flex 布局（任务区 + 日志面板） === */
.main-content {
  display: flex;
  gap: var(--space-4);
  min-height: 0;
  flex: 1;
}

/* Tasks Section */
/* === 任务列表区（flex:1 + 可滚动） === */
.tasks-section {
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--space-1);
}

/* === 带日志面板时的布局（占 60%） === */
.tasks-section.with-logs {
  flex: 0 0 60%;
}

/* Logs Section */
/* === 日志面板区（右侧） === */
.logs-section {
  flex: 1;
  min-width: 280px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* No Execution Container - Dark Theme */
/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #0d1117;
  border-radius: 8px;
  overflow: hidden;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 18px;
  background: #161b22;
  border-bottom: 1px solid #30363d;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #e6edf3;
  font-weight: 600;
  font-size: 14px;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-title .el-icon {
  color: #58a6ff;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-actions {
  display: flex;
  gap: 4px;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-actions .el-button {
  color: #8b949e;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-actions .el-button:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1);
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__header) {
  margin: 0;
  background: #161b22;
  border-bottom: 1px solid #30363d;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__nav-wrap) {
  padding: 0 18px;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__item) {
  color: #8b949e;
  font-weight: 500;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__item:hover) {
  color: #e6edf3;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__item.is-active) {
  color: #58a6ff;
  font-weight: 600;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__active-bar) {
  background-color: #58a6ff;
  height: 2px;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tabs__content) {
  flex: 1;
  overflow: hidden;
  padding: 0;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-tabs :deep(.el-tab-pane) {
  height: 100%;
  overflow: hidden;
}

.no-execution-container .log-body,
/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .output-body {
  height: 100%;
  overflow-y: auto;
  padding: 16px 20px;
  font-family: 'SF Mono', 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13.5px;
  line-height: 1.6;
  background: #0d1117;
}

.no-execution-container .log-body::-webkit-scrollbar,
/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .output-body::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.no-execution-container .log-body::-webkit-scrollbar-thumb,
/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .output-body::-webkit-scrollbar-thumb {
  background: #30363d;
  border-radius: 5px;
}

.no-execution-container .log-body::-webkit-scrollbar-thumb:hover,
/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .output-body::-webkit-scrollbar-thumb:hover {
  background: #484f58;
}

.no-execution-container .log-body::-webkit-scrollbar-track,
/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .output-body::-webkit-scrollbar-track {
  background: #0d1117;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #8b949e;
  gap: 12px;
}

/* === 无执行记录暗色容器（GitHub 风格） === */
.no-execution-container .log-empty .el-icon {
  font-size: 40px;
  color: #30363d;
}

/* === 页面根容器：纵向 flex + 全高可滚动 === */
.tasks-page::-webkit-scrollbar {
  width: 8px;
}

/* === 页面根容器：纵向 flex + 全高可滚动 === */
.tasks-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

/* === 页面根容器：纵向 flex + 全高可滚动 === */
.tasks-page::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

/* Stats Grid */
/* === 顶部统计卡片网格 === */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
}

/* 统计卡片 */
/* === 单个统计卡片：横向 flex + hover 上浮 === */
.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: var(--space-5);
  display: flex;
  align-items: center;
  gap: var(--space-4);
  box-shadow: var(--shadow-md);
  transition: all var(--duration-normal) var(--ease-out);
}

/* === 单个统计卡片：横向 flex + hover 上浮 === */
.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

/* === 统计图标容器（48x48 圆角方块） === */
.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

/* === 统计图标容器（48x48 圆角方块） === */
.stat-icon-primary {
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.1), rgba(37, 99, 235, 0.05));
  color: var(--accent-primary);
}

/* === 统计图标容器（48x48 圆角方块） === */
.stat-icon-success {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.05));
  color: var(--accent-success);
}

/* === 统计图标容器（48x48 圆角方块） === */
.stat-icon-warning {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.1), rgba(245, 158, 11, 0.05));
  color: var(--accent-warning);
}

/* === 统计图标容器（48x48 圆角方块） === */
.stat-icon-info {
  background: linear-gradient(135deg, rgba(139, 92, 246, 0.1), rgba(139, 92, 246, 0.05));
  color: var(--accent-secondary);
}

/* === .stat-content 样式定义 === */
.stat-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

/* === .stat-value 样式定义 === */
.stat-value {
  font-family: var(--font-display);
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

/* === .stat-label 样式定义 === */
.stat-label {
  font-size: 0.8rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

/* Toolbar */
/* === 工具栏容器 === */
.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
}

/* === 工具栏左侧（搜索 + 筛选） === */
.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
}

/* === 工具栏右侧（刷新 + 创建） === */
.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

/* === 搜索输入框 === */
.search-input {
  width: 280px;
}

.search-input :deep(.el-input__wrapper),
/* === 类型/状态筛选下拉 === */
.filter-select :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.search-input :deep(.el-input__wrapper:hover),
/* === 类型/状态筛选下拉 === */
.filter-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.search-input :deep(.el-input__wrapper.is-focus),
/* === 类型/状态筛选下拉 === */
.filter-select :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

/* === 类型/状态筛选下拉 === */
.filter-select {
  width: 140px;
}

/* Toolbar Buttons */
/* === 刷新按钮 === */
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

/* === 刷新按钮 === */
.refresh-btn:hover {
  background: var(--bg-primary);
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  transform: translateY(-2px);
  box-shadow: var(--shadow-sm);
}

/* === 刷新按钮 === */
.refresh-btn:active {
  transform: translateY(0);
}

/* === 创建主按钮（渐变蓝） === */
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

/* === 创建主按钮（渐变蓝） === */
.create-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

/* === 创建主按钮（渐变蓝） === */
.create-btn:active {
  transform: translateY(0);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

/* Tasks Grid */
/* === 任务卡片网格（响应式 auto-fill） === */
.tasks-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: var(--space-5);
  align-content: start;
  padding-bottom: var(--space-4);
}

/* 任务卡片 */
/* === 卡片 hover：上浮 + 阴影 === */
.task-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
  box-shadow: var(--shadow-md);
  transition: all var(--duration-normal) var(--ease-out);
  display: flex;
  flex-direction: column;
}

/* === 卡片 hover：上浮 + 阴影 === */
.task-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-lg), var(--shadow-glow);
  border-color: var(--border-default);
}

/* === 停用状态卡片：半透明 + 灰色边框 === */
.task-card-disabled {
  opacity: 0.6;
}

/* === 停用状态卡片：半透明 + 灰色边框 === */
.task-card-disabled:hover {
  transform: none;
  box-shadow: var(--shadow-md);
}

/* === 选中高亮卡片（主题色边框） === */
.task-card-highlighted {
  border: 2px solid var(--accent-primary);
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.15), var(--shadow-lg), var(--shadow-glow);
  z-index: 10;
}

/* === 选中高亮卡片（主题色边框） === */
.task-card-highlighted:hover {
  transform: translateY(-4px);
}

/* Task Card Header */
/* === 卡片头部：类型图标 + 名称 + 开关 === */
.task-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  background: linear-gradient(180deg, var(--bg-secondary) 0%, transparent 100%);
}

/* === .task-header-left 样式定义 === */
.task-header-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
  min-width: 0;
  cursor: pointer;
}

/* === .task-title-info 样式定义 === */
.task-title-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

/* === .task-type-badge 样式定义 === */
.task-type-badge {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: var(--shadow-sm);
}

/* === .http 样式定义 === */
.task-type-badge.http {
  background: linear-gradient(135deg, #dbeafe, #bfdbfe);
  color: var(--accent-primary);
}

/* === .shell 样式定义 === */
.task-type-badge.shell {
  background: linear-gradient(135deg, #fef3c7, #fde68a);
  color: var(--accent-warning);
}

/* Task Card Body */
/* === 卡片 hover：上浮 + 阴影 === */
.task-card-body {
  padding: var(--space-5);
  flex: 1;
  cursor: pointer;
}

/* === 任务名称（加粗） === */
.task-name {
  font-family: var(--font-display);
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  letter-spacing: -0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* === .task-id 样式定义 === */
.task-id {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
  margin: 0;
}

/* === .task-creator 样式定义 === */
.task-creator {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin: 4px 0 0 0;
  
  /* === .el-icon 样式定义 === */
  .el-icon {
    font-size: 14px;
  }
}

/* === 任务元信息（创建者/时间） === */
.task-meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  padding-bottom: var(--space-4);
  border-bottom: 1px dashed var(--border-subtle);
}

/* === .meta-item 样式定义 === */
.meta-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.8rem;
  color: var(--text-secondary);
  background: var(--bg-secondary);
  padding: 6px 12px;
  border-radius: var(--radius-sm);
}

/* === .el-icon 样式定义 === */
.meta-item .el-icon {
  color: var(--text-muted);
}

/* === .meta-value 样式定义 === */
.meta-value {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}

/* === .meta-item-next-run 样式定义 === */
.meta-item-next-run {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.1), rgba(16, 185, 129, 0.05));
  border: 1px solid rgba(16, 185, 129, 0.2);
  color: var(--accent-success);
}

/* === .el-icon 样式定义 === */
.meta-item-next-run .el-icon {
  color: var(--accent-success);
}

/* === .meta-value 样式定义 === */
.meta-item-next-run .meta-value {
  color: var(--accent-success);
  font-weight: 500;
}

/* === .task-execution 样式定义 === */
.task-execution {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/* === .task-timestamps 样式定义 === */
.task-timestamps {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
  margin-top: var(--space-4);
  padding-top: var(--space-4);
  border-top: 1px dashed var(--border-subtle);
}

/* === .timestamp-item 样式定义 === */
.timestamp-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.72rem;
  color: var(--text-muted);
}

/* === .el-icon 样式定义 === */
.timestamp-item .el-icon {
  font-size: 12px;
}

/* === .execution-info 样式定义 === */
.execution-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* === .execution-label 样式定义 === */
.execution-label {
  font-size: 0.72rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 500;
}

/* === .execution-time 样式定义 === */
.execution-time {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.85rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

/* === .el-icon 样式定义 === */
.execution-time .el-icon {
  color: var(--accent-success);
}

/* === .execution-result 样式定义 === */
.execution-result {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-secondary);
}

/* === .success 样式定义 === */
.execution-result.success {
  color: var(--status-success);
}

/* === .failed 样式定义 === */
.execution-result.failed {
  color: var(--status-error);
}

/* === .running 样式定义 === */
.execution-result.running {
  color: var(--status-running);
}

/* === .pending 样式定义 === */
.execution-result.pending {
  color: var(--status-pending);
}

/* === .result-dot 样式定义 === */
.result-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

/* === .result-dot 样式定义 === */
.execution-result.success .result-dot {
  background: var(--status-success);
}

/* === .result-dot 样式定义 === */
.execution-result.failed .result-dot {
  background: var(--status-error);
}

/* === .result-dot 样式定义 === */
.execution-result.running .result-dot {
  background: var(--status-running);
  animation: pulse 1.5s ease-in-out infinite;
}

/* === .result-dot 样式定义 === */
.execution-result.pending .result-dot {
  background: var(--status-pending);
}

/* Task Card Footer */
/* === 卡片底部：操作按钮组 === */
.task-card-footer {
  padding: var(--space-4);
  background: var(--bg-secondary);
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

/* === 卡片底部操作按钮组 === */
.task-actions {
  display: flex;
  flex-direction: row !important;
  gap: 4px;
  flex-wrap: nowrap;
  width: 100%;
  padding: 0 2px;
  box-sizing: border-box;
}

/* === 卡片底部操作按钮组 === */
.task-actions :deep(.el-button) {
  flex: 1 1 0;
  font-size: 0.68rem;
  padding: 5px 2px;
  min-width: 0 !important;
  white-space: nowrap;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: auto !important;
  height: auto;
}

/* Empty State */
/* === .empty-state 样式定义 === */
.empty-state {
  grid-column: 1 / -1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-16) var(--space-6);
  gap: var(--space-4);
}

/* === .empty-icon 样式定义 === */
.empty-icon {
  color: var(--text-muted);
  opacity: 0.4;
}

/* === .empty-text 样式定义 === */
.empty-text h3 {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 var(--space-2) 0;
  text-align: center;
}

/* === .empty-text 样式定义 === */
.empty-text p {
  font-size: 0.9rem;
  color: var(--text-muted);
  margin: 0;
  text-align: center;
}

/* Form Styles */
/* === 任务表单纵向 flex === */
.task-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* === Cron 输入框 wrapper === */
.cron-input-wrapper {
  width: 100%;
}

/* === Cron 提示文本（等宽 + 小号） === */
.cron-hint {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  color: var(--text-muted);
  white-space: nowrap;
}

/* === 配置卡片（浅灰背景） === */
.config-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: var(--space-4);
}

/* === .config-input 样式定义 === */
.config-input {
  margin-bottom: var(--space-3) !important;
}

/* === .config-input 样式定义 === */
.config-input:last-child {
  margin-bottom: 0 !important;
}

/* === .el-textarea__inner) 样式定义 === */
.code-textarea :deep(.el-textarea__inner) {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  line-height: 1.6;
}

/* === 开关 + 标签横向排列 === */
.switch-wrapper {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

/* === 开关右侧标签文本 === */
.switch-text {
  font-size: 0.9rem;
  color: var(--text-secondary);
  font-weight: 500;
}

/* 对话框底部按钮区 */
/* === 对话框底部按钮区 === */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
}

/* === 类型下拉选项（图标 + 文本） === */
.type-option {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* 任务表单新样式 */
/* === 任务表单容器内边距 === */
.task-form-container {
  padding: 8px 0;
}

/* === 任务表单纵向 flex === */
.task-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* === 表单分区卡片（圆角 + hover 主题色边框） === */
.form-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.3s ease;
}

/* === 表单分区卡片（圆角 + hover 主题色边框） === */
.form-section:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.1);
}

/* === 分区标题栏（渐变底 + 图标） === */
.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 20px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.05), rgba(99, 102, 241, 0.05));
  border-bottom: 1px solid var(--border-default);
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

/* === 分区主体内边距 === */
.section-body {
  padding: 20px;
}

/* === .form-item 样式定义 === */
.form-item {
  margin-bottom: 0 !important;
}

.form-input,
.form-select,
.cron-preset-select,
/* === .form-input-number 样式定义 === */
.form-input-number {
  width: 100%;
}

.form-input :deep(.el-input__wrapper),
.form-select :deep(.el-input__wrapper),
/* === .el-input__wrapper) 样式定义 === */
.cron-preset-select :deep(.el-input__wrapper) {
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  box-shadow: none;
  transition: all 0.2s ease;
  padding: 6px 12px;
}

.form-input :deep(.el-input__wrapper:hover),
.form-select :deep(.el-input__wrapper:hover),
/* === .el-input__wrapper 样式定义 === */
.cron-preset-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.form-input :deep(.el-input__wrapper.is-focus),
.form-select :deep(.el-input__wrapper.is-focus),
/* === .is-focus) 样式定义 === */
.cron-preset-select :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}

/* === textarea 主题色边框样式 === */
.form-textarea :deep(.el-textarea__inner) {
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  font-family: var(--font-mono);
  font-size: 13px;
  transition: all 0.2s ease;
}

/* === textarea 主题色边框样式 === */
.form-textarea :deep(.el-textarea__inner:hover) {
  border-color: var(--accent-primary);
}

/* === textarea 主题色边框样式 === */
.form-textarea :deep(.el-textarea__inner:focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}

/* === .cron-wrapper 样式定义 === */
.cron-wrapper {
  width: 100%;
}

/* === Cron 提示文本（等宽 + 小号） === */
.cron-hint {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

/* === .option-content 样式定义 === */
.option-content {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* === .config-panel 样式定义 === */
.config-panel {
  width: 100%;
}

/* === .script-editor 样式定义 === */
.script-editor {
  width: 100%;
}

/* === Shell 脚本编辑器（VS Code 暗色主题） === */
.script-textarea :deep(.el-textarea__inner) {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, 'Courier New', monospace;
  font-size: 14px;
  line-height: 1.6;
  padding: 16px;
}

/* === .empty-col 样式定义 === */
.empty-col {
  height: 1px;
}

/* === .el-dialog__header) 样式定义 === */
.task-dialog :deep(.el-dialog__header) {
  padding: 20px 24px 16px;
  border-bottom: 1px solid var(--border-default);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.03), rgba(99, 102, 241, 0.03));
}

/* === .el-dialog__title) 样式定义 === */
.task-dialog :deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

/* === .el-dialog__body) 样式定义 === */
.task-dialog :deep(.el-dialog__body) {
  padding: 24px;
  max-height: 70vh;
  overflow-y: auto;
}

/* === .el-dialog__footer) 样式定义 === */
.task-dialog :deep(.el-dialog__footer) {
  padding: 16px 24px 20px;
  border-top: 1px solid var(--border-default);
  background: var(--bg-secondary);
}

/* 对话框底部按钮区 */
/* === 对话框底部按钮区 === */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* === 对话框底部按钮区 === */
.dialog-footer :deep(.el-button) {
  padding: 10px 24px;
  font-size: 14px;
  font-weight: 500;
  border-radius: 8px;
}

/* === 对话框底部按钮区 === */
.dialog-footer :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  border: none;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
  transition: all 0.2s ease;
}

/* === 对话框底部按钮区 === */
.dialog-footer :deep(.el-button--primary:hover) {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

/* === 超时提示文本（小号灰色） === */
.timeout-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-left: 8px;
  white-space: nowrap;
}

/* === .timeout-input-wrapper 样式定义 === */
.timeout-input-wrapper {
  display: flex;
  align-items: center;
  width: 100%;
}

/* === .form-input-number 样式定义 === */
.timeout-input-wrapper .form-input-number {
  flex: 0 0 140px;
}

.form-input-number :deep(.el-input-number__decrease),
/* === .el-input-number__increase) 样式定义 === */
.form-input-number :deep(.el-input-number__increase) {
  border-color: var(--border-default);
}

/* === .el-input__wrapper) 样式定义 === */
.form-input-number :deep(.el-input__wrapper) {
  background: var(--bg-primary);
  border: 1px solid var(--border-default);
  box-shadow: none;
  transition: all 0.2s ease;
}

/* === .el-input__wrapper 样式定义 === */
.form-input-number :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

/* === .is-focus) 样式定义 === */
.form-input-number :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.2);
}



/* Pagination */
/* === 分页器容器 === */
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

/* === 分页器容器 === */
.pagination-container :deep(.el-pagination) {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* === 分页器容器 === */
.pagination-container :deep(.el-pagination__total) {
  font-weight: 500;
  color: var(--text-secondary);
}

/* === 分页器容器 === */
.pagination-container :deep(.el-pagination__sizes) {
  /* === .el-input__wrapper 样式定义 === */
  .el-select .el-input__wrapper {
    border-radius: var(--radius-md);
    box-shadow: 0 0 0 1px var(--border-default) inset;
    transition: all var(--duration-normal) var(--ease-out);
    
    /* === CSS 规则定义 === */
    &:hover {
      box-shadow: 0 0 0 1px var(--accent-primary) inset;
    }
  }
}

/* === 分页器容器 === */
.pagination-container :deep(.el-pager li) {
  border-radius: var(--radius-md);
  margin: 0 4px;
  transition: all var(--duration-normal) var(--ease-out);
  font-weight: 500;
  height: 36px;
  min-width: 36px;
  line-height: 36px;
}

/* === 分页器容器 === */
.pagination-container :deep(.el-pager li:not(.is-active)) {
  color: var(--text-secondary);
  background: var(--bg-secondary);
  
  /* === CSS 规则定义 === */
  &:hover {
    color: var(--accent-primary);
    background: var(--bg-primary);
    transform: translateY(-1px);
  }
}

/* === 分页器容器 === */
.pagination-container :deep(.el-pager li.is-active) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  color: white;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.3);
}

.pagination-container :deep(.btn-prev),
/* === 分页器容器 === */
.pagination-container :deep(.btn-next) {
  border-radius: var(--radius-md);
  transition: all var(--duration-normal) var(--ease-out);
  height: 36px;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.pagination-container :deep(.btn-prev:hover),
/* === 分页器容器 === */
.pagination-container :deep(.btn-next:hover) {
  transform: translateY(-1px);
  box-shadow: var(--shadow-sm);
}

/* === 分页器容器 === */
.pagination-container :deep(.el-pagination__jump) {
  /* === .el-input__wrapper 样式定义 === */
  .el-input__wrapper {
    border-radius: var(--radius-md);
  }
}

/* Responsive */
@media (max-width: 1400px) {
/* 任务卡片网格 */
  /* === 任务卡片网格（响应式 auto-fill） === */
  .tasks-grid {
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  }
}

/* === 指令/媒体查询 === */
@media (max-width: 1200px) {
/* 主内容区域 */
  /* === 主内容区 flex 布局（任务区 + 日志面板） === */
  .main-content {
    flex-direction: column;
  }
  
  /* === 带日志面板时的布局（占 60%） === */
  .tasks-section.with-logs {
    flex: 1;
  }
  
  /* === 日志面板区（右侧） === */
  .logs-section {
    min-width: 100%;
    height: 500px;
  }
}

/* === 指令/媒体查询 === */
@media (max-width: 1024px) {
/* 统计卡片网格 */
  /* === 顶部统计卡片网格 === */
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

/* 任务卡片网格 */
  /* === 任务卡片网格（响应式 auto-fill） === */
  .tasks-grid {
    grid-template-columns: 1fr;
  }

/* 页面工具栏 */
  /* === 工具栏容器 === */
  .page-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  /* === 工具栏左侧（搜索 + 筛选） === */
  .toolbar-left {
    flex-wrap: wrap;
  }

  /* === 搜索输入框 === */
  .search-input {
    flex: 1;
    min-width: 200px;
  }
}

/* === 指令/媒体查询 === */
@media (max-width: 768px) {
/* 页面整体容器 */
  /* === 页面根容器：纵向 flex + 全高可滚动 === */
  .tasks-page {
    gap: var(--space-4);
  }
}

/* === 指令/媒体查询 === */
@media (max-width: 640px) {
/* 统计卡片网格 */
  /* === 顶部统计卡片网格 === */
  .stats-grid {
    grid-template-columns: 1fr;
  }

  /* === 卡片底部操作按钮组 === */
  .task-actions {
    flex-direction: column;
  }
}
</style>
