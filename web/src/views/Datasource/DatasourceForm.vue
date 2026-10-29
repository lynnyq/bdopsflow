<!--
  数据源编辑表单组件
  功能：新建或编辑数据源配置，根据类型（MySQL/SQLite/rqlite/Redis 等）动态展示对应的连接配置字段
  主要区域：基本信息分区、连接配置分区（根据类型切换）、高级设置分区（SSL、读写分离等）
  支持的数据源：mysql, postgres, sqlite, rqlite, redis, doris, starrocks, prometheus, kafka
-->
<template>
  <!-- === .datasource-form-page 区块 === -->
  <div class="datasource-form-page">
    <!-- === 页面头部：返回按钮 + 页面标题 === -->
    <div class="page-header">
      <!-- === 按钮组件 === -->
      <el-button :icon="ArrowLeft" @click="handleBack" text>返回列表</el-button>
      <h2 class="page-title">{{ isEdit ? '编辑数据源' : '新建数据源' }}</h2>
    </div>

    <!-- === 表单主容器（基本信息 + 连接配置 + 其他设置 三个分区） === -->
    <div class="form-container">
      <!-- === 表单组件 === -->
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="ds-form">
        <!-- === 基本信息分区：名称 + 类型 + 描述 === -->
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
                <el-form-item label="名称" prop="name">
                  <!-- === 输入框组件 === -->
                  <el-input v-model="form.name" placeholder="请输入数据源名称" />
                </el-form-item>
              </el-col>
              <!-- === 栅格列 === -->
              <el-col :span="12">
                <!-- === 表单组件 === -->
                <el-form-item label="类型" prop="type">
                  <!-- === 下拉选择组件 === -->
                  <el-select v-model="form.type" placeholder="请选择数据源类型" :disabled="isEdit" @change="handleTypeChange">
                    <!-- === 下拉选项 === -->
                    <el-option
                      v-for="(label, key) in dsTypeLabels"
                      :key="key"
                      :label="label"
                      :value="key"
                    />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
            <!-- === 栅格行 === -->
            <el-row :gutter="16">
              <!-- === 栅格列 === -->
              <el-col :span="24">
                <!-- === 表单组件 === -->
                <el-form-item label="描述">
                  <!-- === 输入框组件 === -->
                  <el-input v-model="form.description" type="textarea" :rows="2" placeholder="请输入描述" />
                </el-form-item>
              </el-col>
            </el-row>
          </div>
        </div>

        <!-- === 连接配置分区：根据 type 动态渲染不同表单字段 === -->
        <div class="form-section" v-if="form.type">
          <!-- === .section-header 区块 === -->
          <div class="section-header">
            <!-- === 图标组件 === -->
            <el-icon :size="18"><Connection /></el-icon>
            <span>连接配置</span>
          </div>
          <!-- === .section-body 区块 === -->
          <div class="section-body">
            <!-- === SQLite 单字段：文件路径 === -->
            <template v-if="form.type === 'sqlite'">
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="24">
                  <!-- === 表单组件 === -->
                  <el-form-item label="文件路径" prop="path">
                    <!-- === 输入框组件 === -->
                    <el-input v-model="form.path" placeholder="/path/to/database.db" />
                  </el-form-item>
                </el-col>
              </el-row>
            </template>

            <!-- === MySQL / StarRocks / Doris：主机 + 端口 + 库 + 用户 + 密码 === -->
            <template v-else-if="['mysql', 'starrocks', 'doris'].includes(form.type)">
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <!-- === 表单组件 === -->
                  <el-form-item label="主机" prop="host">
                    <!-- === 输入框组件 === -->
                    <el-input v-model="form.host" placeholder="请输入主机地址" />
                  </el-form-item>
                </el-col>
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <!-- === 表单组件 === -->
                  <el-form-item label="端口" prop="port">
                    <!-- === 输入框组件 === -->
                    <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                  </el-form-item>
                </el-col>
              </el-row>
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <!-- === 表单组件 === -->
                  <el-form-item label="数据库" prop="database">
                    <!-- === 输入框组件 === -->
                    <el-input v-model="form.database" placeholder="请输入数据库名" />
                  </el-form-item>
                </el-col>
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <div style="height: 1px"></div>
                </el-col>
              </el-row>
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <!-- === 表单组件 === -->
                  <el-form-item label="用户名" prop="username">
                    <!-- === 输入框组件 === -->
                    <el-input v-model="form.username" placeholder="请输入用户名" />
                  </el-form-item>
                </el-col>
                <!-- === 栅格列 === -->
                <el-col :span="12">
                  <!-- === 表单组件 === -->
                  <el-form-item label="密码" prop="password">
                    <!-- === 输入框组件 === -->
                    <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
                  </el-form-item>
                </el-col>
              </el-row>
            </template>

            <!-- === Rqlite：单节点 / 多节点模式切换 === -->
            <template v-else-if="form.type === 'rqlite'">
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="24">
                  <!-- === 表单组件 === -->
                  <el-form-item label="连接模式" prop="connection_mode">
                    <!-- === 下拉选择组件 === -->
                    <el-select v-model="form.connection_mode" placeholder="请选择连接模式" @change="handleRqliteConnectionModeChange">
                      <!-- === 下拉选项 === -->
                      <el-option label="单节点" value="single" />
                      <!-- === 下拉选项 === -->
                      <el-option label="多节点" value="multi" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'single'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="主机" prop="host">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.host" placeholder="请输入主机地址" />
                    </el-form-item>
                  </el-col>
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="端口" prop="port">
                      <!-- === 输入框组件 === -->
                      <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'multi'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="24">
                    <!-- === 表单组件 === -->
                    <el-form-item label="节点地址" prop="rqlite_hosts">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.rqlite_hosts" placeholder="host1:4001,host2:4001,host3:4001" type="textarea" :rows="2" />
                      <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">多个节点用逗号分隔</div>
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="用户名" prop="username">
                    <el-input v-model="form.username" placeholder="请输入用户名（可选）" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="密码" prop="password">
                    <el-input v-model="form.password" type="password" show-password placeholder="请输入密码（可选）" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="SSL">
                    <el-switch v-model="form.ssl" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <div style="height: 1px"></div>
                </el-col>
              </el-row>
            </template>

            <!-- === Redis：单节点 / 哨兵 / 集群 三种模式 === -->
            <template v-else-if="form.type === 'redis'">
              <!-- === 栅格行 === -->
              <el-row :gutter="16">
                <!-- === 栅格列 === -->
                <el-col :span="24">
                  <!-- === 表单组件 === -->
                  <el-form-item label="连接模式" prop="connection_mode">
                    <!-- === 下拉选择组件 === -->
                    <el-select v-model="form.connection_mode" placeholder="请选择连接模式" @change="handleRedisConnectionModeChange">
                      <!-- === 下拉选项 === -->
                      <el-option label="单节点" value="single" />
                      <!-- === 下拉选项 === -->
                      <el-option label="哨兵模式" value="sentinel" />
                      <!-- === 下拉选项 === -->
                      <el-option label="集群模式" value="cluster" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'single'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="主机" prop="host">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.host" placeholder="请输入主机地址" />
                    </el-form-item>
                  </el-col>
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="端口" prop="port">
                      <!-- === 输入框组件 === -->
                      <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'sentinel'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="24">
                    <!-- === 表单组件 === -->
                    <el-form-item label="哨兵地址" prop="sentinel_addrs">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.sentinel_addrs" placeholder="host1:26379,host2:26379,host3:26379" type="textarea" :rows="2" />
                      <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">多个哨兵节点用逗号分隔</div>
                    </el-form-item>
                  </el-col>
                </el-row>
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="24">
                    <!-- === 表单组件 === -->
                    <el-form-item label="主节点名称" prop="master_name">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.master_name" placeholder="如 mymaster" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'cluster'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="24">
                    <!-- === 表单组件 === -->
                    <el-form-item label="集群节点地址" prop="cluster_addrs">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.cluster_addrs" placeholder="host1:6379,host2:6379,host3:6379" type="textarea" :rows="2" />
                      <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">多个集群节点用逗号分隔</div>
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="密码" prop="password">
                    <el-input v-model="form.password" type="password" show-password :placeholder="isEdit ? '留空保持原密码' : '请输入密码（可选）'" />
                  </el-form-item>
                </el-col>
                <el-col :span="12" v-if="form.connection_mode !== 'cluster'">
                  <el-form-item label="数据库编号" prop="database">
                    <el-input v-model="form.database" placeholder="默认 0" />
                    <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">单节点/哨兵模式支持 0-15，集群模式固定为 0</div>
                  </el-form-item>
                </el-col>
              </el-row>
            </template>

            <!-- === Hive / Kyuubi / Spark / Trino：Thrift 协议 + Zookeeper 直连 === -->
            <template v-else-if="['hive', 'kyuubi', 'spark', 'trino'].includes(form.type)">
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'direct'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="主机" prop="host">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.host" placeholder="请输入主机地址" />
                    </el-form-item>
                  </el-col>
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="端口" prop="port">
                      <!-- === 输入框组件 === -->
                      <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="数据库" prop="database">
                    <el-input v-model="form.database" placeholder="请输入数据库名" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <div style="height: 1px"></div>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="用户名" prop="username">
                    <el-input v-model="form.username" placeholder="请输入用户名" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="密码">
                    <el-input v-model="form.password" type="password" show-password :placeholder="isEdit ? '留空保持原密码' : '请输入密码'" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="认证方式" prop="auth_type">
                    <el-select v-model="form.auth_type" placeholder="请选择认证方式">
                      <el-option label="Simple" value="simple" />
                      <el-option label="LDAP" value="ldap" />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="连接模式" prop="connection_mode">
                    <el-select v-model="form.connection_mode" placeholder="请选择连接模式" @change="handleConnectionModeChange">
                      <el-option label="直连" value="direct" />
                      <el-option label="ZooKeeper" value="zookeeper" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
              <!-- === 条件/循环模板 === -->
              <template v-if="form.connection_mode === 'zookeeper'">
                <!-- === 栅格行 === -->
                <el-row :gutter="16">
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="ZooKeeper 地址" prop="zk_hosts">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.zk_hosts" placeholder="host1:2181,host2:2181" />
                    </el-form-item>
                  </el-col>
                  <!-- === 栅格列 === -->
                  <el-col :span="12">
                    <!-- === 表单组件 === -->
                    <el-form-item label="ZooKeeper 路径" prop="zk_path">
                      <!-- === 输入框组件 === -->
                      <el-input v-model="form.zk_path" placeholder="/hiveserver2" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="传输模式">
                    <el-select v-model="form.transport_mode" placeholder="请选择传输模式">
                      <el-option label="Binary" value="binary" />
                      <el-option label="HTTP" value="http" />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :span="12" v-if="form.transport_mode === 'http'">
                  <el-form-item label="HTTP 路径">
                    <el-input v-model="form.http_path" placeholder="cliservice" />
                  </el-form-item>
                </el-col>
                <el-col :span="12" v-else>
                  <div style="height: 1px"></div>
                </el-col>
              </el-row>
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="SSL">
                    <el-switch v-model="form.ssl" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <div style="height: 1px"></div>
                </el-col>
              </el-row>
            </template>
          </div>
        </div>

        <!-- === 其他设置分区：启用开关 + 允许 DML/DDL 开关 === -->
        <div class="form-section">
          <div class="section-header">
            <el-icon :size="18"><Setting /></el-icon>
            <span>其他设置</span>
          </div>
          <div class="section-body">
            <div class="switch-wrapper">
              <el-switch v-model="form.is_enabled" size="large" />
              <span class="switch-text">{{ form.is_enabled ? '启用数据源' : '禁用数据源' }}</span>
            </div>
            <div class="switch-wrapper" style="margin-top: 12px;">
              <el-switch v-model="form.allow_write_sql" size="large" />
              <span class="switch-text">{{ form.allow_write_sql ? '允许 DML/DDL 语句' : '仅允许 SELECT 查询' }}</span>
            </div>
            <div v-if="form.allow_write_sql" class="dml-warning-tip">
              <el-icon><Warning /></el-icon>
              <span>开启后允许执行 INSERT/UPDATE/DELETE/CREATE/DROP/ALTER 等写操作，请谨慎配置。仅系统管理员可开启此选项。</span>
            </div>
          </div>
        </div>
      </el-form>
    </div>

    <!-- === 底部操作栏：取消 / 测试连接 / 创建(或保存) === -->
    <div class="form-footer">
      <el-button @click="handleBack">取消</el-button>
      <el-button type="primary" @click="handleTestConnection" :loading="testing">
        测试连接
      </el-button>
      <!-- === 条件/循环模板 === -->
      <template v-if="!isEdit">
        <!-- === 按钮组件 === -->
        <el-button
          type="primary"
          @click="handleSubmit"
          :loading="submitting"
          :disabled="!testPassed"
        >
          创建
        </el-button>
        <!-- === 条件渲染区块 === -->
        <span v-if="!testPassed" class="test-hint">
          <!-- === 图标组件 === -->
          <el-icon><Warning /></el-icon>
          请先测试连接通过后再创建
        </span>
        <!-- === else 分支区块 === -->
        <el-tag v-else type="success" effect="light" size="small" class="test-passed-tag">
          <!-- === 图标组件 === -->
          <el-icon><CircleCheck /></el-icon> 连接测试通过
        </el-tag>
      </template>
      <el-button v-else type="primary" @click="handleSubmit" :loading="submitting">
        保存
      </el-button>
    </div>
  </div>
</template>

/**
 * 数据源编辑表单脚本
 * @description 负责数据源的新建和编辑，根据类型动态切换连接配置字段，保存前自动测试连接
 *
 * 表单分区：
 * - 基本信息：名称、类型（编辑时不可改）、描述
 * - 连接配置：根据 type 动态显示（MySQL 主机/端口/库/用户/密码，SQLite 路径，rqlite 单/多节点等）
 * - 高级设置：SSL、读写分离池配置、元数据缓存 TTL
 *
 * 依赖：datasourceAPI（含测试连接接口）、authStore
 */
<script setup lang="ts">
// ========== Vue 核心 API ==========
import { ref, computed, onMounted, watch } from 'vue'
// ========== Vue Router ==========
import { useRouter, useRoute } from 'vue-router'
// ========== Element Plus ==========
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
// ========== Element Plus 图标 ==========
import { ArrowLeft, Document, Connection, Setting, Warning, CircleCheck } from '@element-plus/icons-vue'
// ========== API 层 ==========
import { datasourceAPI } from '@/api'
// ========== Pinia Store ==========
import { useAuthStore } from '@/stores/auth'
// ========== 工具函数 ==========
import { isHandledError } from '@/utils/api'

/** Vue Router 实例，用于页面跳转 */
const router = useRouter()
/** Vue Route 实例，读取路由参数（编辑模式的数据源 ID） */
const route = useRoute()
/** 认证状态 Store（用于获取当前领域 domain_id） */
const authStore = useAuthStore()

/** 数据源类型代码 → 中文标签映射（用于 type 下拉选项） */
const dsTypeLabels: Record<string, string> = {
  mysql: 'MySQL',
  sqlite: 'SQLite',
  rqlite: 'Rqlite',
  redis: 'Redis',
  hive: 'Hive',
  kyuubi: 'Kyuubi',
  trino: 'Trino',
  spark: 'Spark',
  starrocks: 'StarRocks',
  doris: 'Doris',
}

/** 是否为编辑模式（路由存在 :id 参数时为 true） */
const isEdit = computed(() => !!route.params.id)
/** 表单 DOM 引用（用于调用 validate 提交表单） */
const formRef = ref<FormInstance>()
/** 提交按钮 loading 状态 */
const submitting = ref(false)
/** 测试连接按钮 loading 状态 */
const testing = ref(false)
/** 测试连接是否已通过（新增模式下创建按钮需依赖此状态） */
const testPassed = ref(false)

/**
 * 表单主数据对象，包含所有可能的数据源字段
 * 不同 type/connection_mode 组合下只有部分字段生效
 */
const form = ref({
  name: '',
  type: '',
  host: '',
  port: 3306,
  path: '',
  database: '',
  username: '',
  password: '',
  auth_type: 'simple',
  connection_mode: 'direct',
  zk_hosts: '',
  zk_path: '',
  rqlite_hosts: '',
  transport_mode: 'binary',
  http_path: '',
  ssl: false,
  description: '',
  is_enabled: true,
  allow_write_sql: false,
  domain_id: 0 as number,
  // Redis 专用字段
  sentinel_addrs: '',
  cluster_addrs: '',
  master_name: '',
})

/**
 * 根据当前 form.type 动态生成 Element Plus 表单校验规则
 * 不同数据源类型 + 连接模式组合需要校验的字段不同
 * @returns {FormRules} 表单校验规则对象
 */
const rules = computed<FormRules>(() => {
  /** base 变量声明 */
  const base: FormRules = {
    name: [{ required: true, message: '请输入数据源名称', trigger: 'blur' }],
    type: [{ required: true, message: '请选择数据源类型', trigger: 'change' }],
  }

  /** 条件分支 */
  if (form.value.type === 'sqlite') {
    base.path = [{ required: true, message: '请输入文件路径', trigger: 'blur' }]
  }

  /** 条件分支 */
  if (['mysql', 'starrocks', 'doris'].includes(form.value.type)) {
    base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
    base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    base.database = [{ required: true, message: '请输入数据库名', trigger: 'blur' }]
    base.username = [{ required: true, message: '请输入用户名', trigger: 'blur' }]
  }

  /** 条件分支 */
  if (form.value.type === 'rqlite') {
    base.connection_mode = [{ required: true, message: '请选择连接模式', trigger: 'change' }]
    /** 条件分支 */
    if (form.value.connection_mode === 'single') {
      base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
      base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    } else if (form.value.connection_mode === 'multi') {
      base.rqlite_hosts = [{ required: true, message: '请输入节点地址', trigger: 'blur' }]
    }
  }

  /** 条件分支 */
  if (form.value.type === 'redis') {
    base.connection_mode = [{ required: true, message: '请选择连接模式', trigger: 'change' }]
    /** 条件分支 */
    if (form.value.connection_mode === 'single') {
      base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
      base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    } else if (form.value.connection_mode === 'sentinel') {
      base.sentinel_addrs = [{ required: true, message: '请输入哨兵地址', trigger: 'blur' }]
      base.master_name = [{ required: true, message: '请输入主节点名称', trigger: 'blur' }]
    } else if (form.value.connection_mode === 'cluster') {
      base.cluster_addrs = [{ required: true, message: '请输入集群节点地址', trigger: 'blur' }]
    }
  }

  /** 条件分支 */
  if (['hive', 'kyuubi', 'spark', 'trino'].includes(form.value.type)) {
    base.auth_type = [{ required: true, message: '请选择认证方式', trigger: 'change' }]
    base.connection_mode = [{ required: true, message: '请选择连接模式', trigger: 'change' }]
    
    /** 条件分支 */
    if (form.value.connection_mode === 'direct') {
      base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
      base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    } else if (form.value.connection_mode === 'zookeeper') {
      base.zk_hosts = [{ required: true, message: '请输入ZooKeeper地址', trigger: 'blur' }]
      base.zk_path = [{ required: true, message: '请输入ZooKeeper路径', trigger: 'blur' }]
    }
  }

  /** 返回/抛出 */
  return base
})

/** 数据源类型 → 默认端口映射（切换类型时自动填充） */
const defaultPorts: Record<string, number> = {
  mysql: 3306,
  starrocks: 9030,
  doris: 9030,
  rqlite: 4001,
  redis: 6379,
  hive: 10000,
  kyuubi: 10009,
  spark: 10000,
  trino: 8080,
}

/**
 * 数据源类型变更时重置默认端口、连接模式和其他专属配置字段
 * @param {string} type - 新的数据源类型（mysql/rqlite/redis 等）
 */
const handleTypeChange = (type: string) => {
  form.value.port = defaultPorts[type] || 3306
  /** 条件分支 */
  if (type === 'rqlite' || type === 'redis') {
    form.value.connection_mode = 'single'
  } else {
    form.value.connection_mode = 'direct'
  }
  form.value.auth_type = 'simple'
  form.value.transport_mode = 'binary'
  form.value.http_path = ''
  form.value.ssl = false
  form.value.rqlite_hosts = ''
  form.value.sentinel_addrs = ''
  form.value.cluster_addrs = ''
  form.value.master_name = ''
  form.value.database = ''
  testPassed.value = false
}

/**
 * Redis 连接模式切换时清空对应字段（哨兵/集群时不需要 host）
 */
const handleRedisConnectionModeChange = () => {
  form.value.host = ''
  form.value.port = defaultPorts.redis || 6379
  form.value.sentinel_addrs = ''
  form.value.cluster_addrs = ''
  form.value.master_name = ''
  testPassed.value = false
}

/**
 * rqlite 连接模式切换时清空单节点/集群各自专属字段
 */
const handleRqliteConnectionModeChange = () => {
  form.value.rqlite_hosts = ''
  form.value.host = ''
  form.value.port = defaultPorts[form.value.type] || 4001
  testPassed.value = false
}

/**
 * 多模式数据源（Hive/Kyuubi 等）连接方式变更时清空对应字段
 */
const handleConnectionModeChange = () => {
  form.value.zk_hosts = ''
  form.value.zk_path = ''
  form.value.host = ''
  form.value.port = defaultPorts[form.value.type] || 3306
  testPassed.value = false
}

/**
 * 根据表单数据和数据源类型组装后端提交对象，处理不同类型的专属字段
 * rqlite/redis/hive 等复杂字段序列化为 JSON 存入 config 字段
 * @returns {Partial<any>} 后端请求体数据
 */
const buildSubmitData = () => {
  const data: Partial<any> = {
    name: form.value.name,
    type: form.value.type,
    description: form.value.description,
    is_enabled: form.value.is_enabled,
    allow_write_sql: form.value.allow_write_sql,
    domain_id: form.value.domain_id || authStore.currentDomainId || 1,
  }

  /** 条件分支 */
  if (form.value.type === 'sqlite') {
    data.path = form.value.path
  } else if (['mysql', 'starrocks', 'doris'].includes(form.value.type)) {
    data.host = form.value.host
    data.port = form.value.port
    data.database = form.value.database
    data.username = form.value.username
    data.password = form.value.password
  } else if (form.value.type === 'rqlite') {
    data.connection_mode = form.value.connection_mode
    /** 条件分支 */
    if (form.value.connection_mode === 'single') {
      data.host = form.value.host
      data.port = form.value.port
    } else if (form.value.connection_mode === 'multi') {
      data.rqlite_hosts = form.value.rqlite_hosts
    }
    data.username = form.value.username
    data.password = form.value.password
    const config: Record<string, unknown> = {}
    /** 条件分支 */
    if (form.value.ssl) {
      config.ssl = true
    }
    data.config = JSON.stringify(config)
  } else if (form.value.type === 'redis') {
    data.connection_mode = form.value.connection_mode
    data.password = form.value.password
    data.database = form.value.database
    const config: Record<string, unknown> = {}
    /** 条件分支 */
    if (form.value.connection_mode === 'single') {
      data.host = form.value.host
      data.port = form.value.port
    } else if (form.value.connection_mode === 'sentinel') {
      config.sentinel_addrs = form.value.sentinel_addrs
      config.master_name = form.value.master_name
    } else if (form.value.connection_mode === 'cluster') {
      config.cluster_addrs = form.value.cluster_addrs
    }
    /** 条件分支 */
    if (Object.keys(config).length > 0) {
      data.config = JSON.stringify(config)
    }
  }

  /** 条件分支 */
  if (['hive', 'kyuubi', 'spark', 'trino'].includes(form.value.type)) {
    data.auth_type = form.value.auth_type
    data.connection_mode = form.value.connection_mode
    data.database = form.value.database
    data.username = form.value.username
    data.password = form.value.password
    
    /** 条件分支 */
    if (form.value.connection_mode === 'direct') {
      data.host = form.value.host
      data.port = form.value.port
    } else if (form.value.connection_mode === 'zookeeper') {
      data.zk_hosts = form.value.zk_hosts
      data.zk_path = form.value.zk_path
    }

    const config: Record<string, any> = {}
    /** 条件分支 */
    if (form.value.transport_mode && form.value.transport_mode !== 'binary') {
      config.transport_mode = form.value.transport_mode
    }
    /** 条件分支 */
    if (form.value.http_path) {
      config.http_path = form.value.http_path
    }
    /** 条件分支 */
    if (form.value.ssl) {
      config.ssl = true
    }
    /** 条件分支 */
    if (Object.keys(config).length > 0) {
      data.config = JSON.stringify(config)
    }
  }

  /** 返回/抛出 */
  return data
}

/**
 * 调用后端接口测试数据源连接
 * 编辑模式：用已有 ID 调用 testConnection；新增模式：用表单数据调用 testConnectionByParams
 * 通过后设置 testPassed = true，失败时重置 testPassed 并提示错误
 * @returns {Promise<void>}
 */
const handleTestConnection = async () => {
  /** 条件分支 */
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    /** 条件分支 */
    if (!valid) return
    testing.value = true
    /** 异常处理 */
    try {
      /** 条件分支 */
      if (isEdit.value) {
        await datasourceAPI.testConnection(Number(route.params.id))
        ElMessage.success('连接测试成功')
      } else {
        const data = buildSubmitData()
        await datasourceAPI.testConnectionByParams(data)
        ElMessage.success('连接测试成功')
      }
      testPassed.value = true
    } catch (err: any) {
      testPassed.value = false
      /** 条件分支 */
      if (!isHandledError(err)) {
        ElMessage.error(err.response?.data?.error || err.message || '连接测试失败')
      }
    } finally {
      testing.value = false
    }
  })
}

/**
 * 提交数据源表单，根据 isEdit 决定创建或更新
 * 成功后跳转到数据源列表页
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
      const data = buildSubmitData()
      /** 条件分支 */
      if (isEdit.value) {
        await datasourceAPI.update(Number(route.params.id), data)
        ElMessage.success('数据源更新成功')
      } else {
        await datasourceAPI.create(data)
        ElMessage.success('数据源创建成功')
      }
      router.push({ name: 'Datasources' })
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

/** 返回数据源列表页 */
const handleBack = () => {
  router.push({ name: 'Datasources' })
}

/**
 * 编辑模式下加载已有数据源详情，回填表单字段
 * 解析后端返回的 config JSON 字符串填充 SSL/哨兵节点/集群节点等扩展字段
 * @returns {Promise<void>}
 */
const loadDatasource = async () => {
  /** 条件分支 */
  if (!isEdit.value) return
  /** 异常处理 */
  try {
    const res = await datasourceAPI.get(Number(route.params.id))
    const ds = res.data
    /** configData: 可变变量 */
    let configData: Record<string, unknown> = {}
    /** 条件分支 */
    if (ds.config) {
      /** 异常处理 */
      try {
        const parsed = JSON.parse(ds.config)
        /** 条件分支 */
        if (parsed && typeof parsed === 'object') {
          configData = parsed
        }
      } catch (e) {
        console.warn('Failed to parse datasource config:', e)
      }
    }
    form.value = {
      name: ds.name,
      type: ds.type,
      host: ds.host || '',
      port: ds.port || defaultPorts[ds.type] || 3306,
      path: ds.path || '',
      database: ds.database || '',
      username: ds.username || '',
      password: '',
      auth_type: ds.auth_type || 'simple',
      connection_mode: ds.connection_mode || ((ds.type === 'rqlite' || ds.type === 'redis') ? 'single' : 'direct'),
      zk_hosts: ds.zk_hosts || '',
      zk_path: ds.zk_path || '',
      rqlite_hosts: ds.rqlite_hosts || '',
      transport_mode: (configData.transport_mode as string) || 'binary',
      http_path: (configData.http_path as string) || '',
      ssl: Boolean(configData.ssl),
      description: ds.description || '',
      is_enabled: ds.is_enabled,
      allow_write_sql: ds.allow_write_sql || false,
      domain_id: ds.domain_id || 0,
      sentinel_addrs: (configData.sentinel_addrs as string) || '',
      cluster_addrs: (configData.cluster_addrs as string) || '',
      master_name: (configData.master_name as string) || '',
    }
  } catch (err: unknown) {
    /** 条件分支 */
    if (!isHandledError(err)) {
      const message = err instanceof Error ? err.message : '加载数据源失败'
      ElMessage.error(message)
    }
  }
}

/**
 * 监听表单关键字段变化，新增模式下自动重置"连接测试通过"状态
 * 字段变更后需重新测试连接才能创建
 */
watch(
  () => [
    form.value.type, form.value.host, form.value.port, form.value.path,
    form.value.database, form.value.username, form.value.password,
    form.value.auth_type, form.value.connection_mode, form.value.zk_hosts,
    form.value.zk_path, form.value.transport_mode, form.value.http_path, form.value.ssl,
    form.value.sentinel_addrs, form.value.cluster_addrs, form.value.master_name,
  ],
  () => {
    /** 条件分支 */
    if (!isEdit.value) {
      testPassed.value = false
    }
  }
)

/** 组件挂载时：编辑模式加载已有数据源详情 */
onMounted(() => {
  loadDatasource()
})
</script>

<style scoped>
/* === 页面根容器：纵向 flex + 全高可滚动 === */
.datasource-form-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
  overflow-y: auto;
}

/* === 页面头部容器：返回按钮 + 标题 === */
.page-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

/* === 页面标题文本 === */
.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

/* === 表单主容器卡片 === */
.form-container {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  padding: var(--space-4);
}

/* === el-form 布局：纵向堆叠三个分区 === */
.ds-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* === 表单分区卡片：圆角 + 边框，hover 时高亮主题色 === */
.form-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.3s ease;
}

/* === 分区 hover 态：主题色边框 + 淡蓝阴影 === */
.form-section:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.1);
}

/* === 分区标题栏：带渐变背景 + 图标 === */
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

/* === DML 开启时的警告提示（橙色背景 + 边框） === */
.dml-warning-tip {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin-top: 8px;
  padding: 8px 12px;
  background: rgba(230, 162, 60, 0.1);
  border: 1px solid rgba(230, 162, 60, 0.3);
  border-radius: var(--radius-md);
  color: #e6a23c;
  font-size: 0.8rem;
  line-height: 1.5;
}

/* === 警告图标位置微调 === */
.dml-warning-tip .el-icon {
  margin-top: 2px;
  flex-shrink: 0;
}

/* === 表单底部操作栏：右对齐按钮组 === */
.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
  padding: var(--space-4);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

/* === 底部主按钮（渐变蓝 + 阴影） === */
.form-footer :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  border: none;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

/* === 底部主按钮 hover：上浮 + 加深阴影 === */
.form-footer :deep(.el-button--primary:hover) {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

/* === 测试连接未通过时的提示文案（橙色） === */
.test-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.8rem;
  color: #e6a23c;
}

/* === 测试通过的成功 tag（绿色对勾图标） === */
.test-passed-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>
