<template>
  <div class="datasource-form-page">
    <div class="page-header">
      <el-button :icon="ArrowLeft" @click="handleBack" text>返回列表</el-button>
      <h2 class="page-title">{{ isEdit ? '编辑数据源' : '新建数据源' }}</h2>
    </div>

    <div class="form-container">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="ds-form">
        <div class="form-section">
          <div class="section-header">
            <el-icon :size="18"><Document /></el-icon>
            <span>基本信息</span>
          </div>
          <div class="section-body">
            <el-row :gutter="16">
              <el-col :span="12">
                <el-form-item label="名称" prop="name">
                  <el-input v-model="form.name" placeholder="请输入数据源名称" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="类型" prop="type">
                  <el-select v-model="form.type" placeholder="请选择数据源类型" :disabled="isEdit" @change="handleTypeChange">
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
            <el-row :gutter="16">
              <el-col :span="24">
                <el-form-item label="描述">
                  <el-input v-model="form.description" type="textarea" :rows="2" placeholder="请输入描述" />
                </el-form-item>
              </el-col>
            </el-row>
          </div>
        </div>

        <div class="form-section" v-if="form.type">
          <div class="section-header">
            <el-icon :size="18"><Connection /></el-icon>
            <span>连接配置</span>
          </div>
          <div class="section-body">
            <template v-if="form.type === 'sqlite'">
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="文件路径" prop="path">
                    <el-input v-model="form.path" placeholder="/path/to/database.db" />
                  </el-form-item>
                </el-col>
              </el-row>
            </template>

            <template v-else-if="['mysql', 'starrocks', 'doris'].includes(form.type)">
              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="主机" prop="host">
                    <el-input v-model="form.host" placeholder="请输入主机地址" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="端口" prop="port">
                    <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                  </el-form-item>
                </el-col>
              </el-row>
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
                  <el-form-item label="密码" prop="password">
                    <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
                  </el-form-item>
                </el-col>
              </el-row>
            </template>

            <template v-else-if="form.type === 'rqlite'">
              <el-row :gutter="16">
                <el-col :span="24">
                  <el-form-item label="连接模式" prop="connection_mode">
                    <el-select v-model="form.connection_mode" placeholder="请选择连接模式" @change="handleRqliteConnectionModeChange">
                      <el-option label="单节点" value="single" />
                      <el-option label="多节点" value="multi" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
              <template v-if="form.connection_mode === 'single'">
                <el-row :gutter="16">
                  <el-col :span="12">
                    <el-form-item label="主机" prop="host">
                      <el-input v-model="form.host" placeholder="请输入主机地址" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="端口" prop="port">
                      <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" style="width: 100%" />
                    </el-form-item>
                  </el-col>
                </el-row>
              </template>
              <template v-if="form.connection_mode === 'multi'">
                <el-row :gutter="16">
                  <el-col :span="24">
                    <el-form-item label="节点地址" prop="rqlite_hosts">
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

            <template v-else-if="['hive', 'kyuubi', 'spark', 'trino'].includes(form.type)">
              <template v-if="form.connection_mode === 'direct'">
                <el-row :gutter="16">
                  <el-col :span="12">
                    <el-form-item label="主机" prop="host">
                      <el-input v-model="form.host" placeholder="请输入主机地址" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="端口" prop="port">
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
              <template v-if="form.connection_mode === 'zookeeper'">
                <el-row :gutter="16">
                  <el-col :span="12">
                    <el-form-item label="ZooKeeper 地址" prop="zk_hosts">
                      <el-input v-model="form.zk_hosts" placeholder="host1:2181,host2:2181" />
                    </el-form-item>
                  </el-col>
                  <el-col :span="12">
                    <el-form-item label="ZooKeeper 路径" prop="zk_path">
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

    <div class="form-footer">
      <el-button @click="handleBack">取消</el-button>
      <el-button type="primary" @click="handleTestConnection" :loading="testing">
        测试连接
      </el-button>
      <template v-if="!isEdit">
        <el-button
          type="primary"
          @click="handleSubmit"
          :loading="submitting"
          :disabled="!testPassed"
        >
          创建
        </el-button>
        <span v-if="!testPassed" class="test-hint">
          <el-icon><Warning /></el-icon>
          请先测试连接通过后再创建
        </span>
        <el-tag v-else type="success" effect="light" size="small" class="test-passed-tag">
          <el-icon><CircleCheck /></el-icon> 连接测试通过
        </el-tag>
      </template>
      <el-button v-else type="primary" @click="handleSubmit" :loading="submitting">
        保存
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { ArrowLeft, Document, Connection, Setting, Warning, CircleCheck } from '@element-plus/icons-vue'
import { datasourceAPI } from '@/api'
import { useAuthStore } from '@/stores/auth'
import type { Datasource } from '@/types'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const dsTypeLabels: Record<string, string> = {
  mysql: 'MySQL',
  sqlite: 'SQLite',
  rqlite: 'Rqlite',
  hive: 'Hive',
  kyuubi: 'Kyuubi',
  trino: 'Trino',
  spark: 'Spark',
  starrocks: 'StarRocks',
  doris: 'Doris',
}

const isEdit = computed(() => !!route.params.id)
const formRef = ref<FormInstance>()
const submitting = ref(false)
const testing = ref(false)
const testPassed = ref(false)

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
})

const rules = computed<FormRules>(() => {
  const base: FormRules = {
    name: [{ required: true, message: '请输入数据源名称', trigger: 'blur' }],
    type: [{ required: true, message: '请选择数据源类型', trigger: 'change' }],
  }

  if (form.value.type === 'sqlite') {
    base.path = [{ required: true, message: '请输入文件路径', trigger: 'blur' }]
  }

  if (['mysql', 'starrocks', 'doris'].includes(form.value.type)) {
    base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
    base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    base.database = [{ required: true, message: '请输入数据库名', trigger: 'blur' }]
    base.username = [{ required: true, message: '请输入用户名', trigger: 'blur' }]
  }

  if (form.value.type === 'rqlite') {
    base.connection_mode = [{ required: true, message: '请选择连接模式', trigger: 'change' }]
    if (form.value.connection_mode === 'single') {
      base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
      base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    } else if (form.value.connection_mode === 'multi') {
      base.rqlite_hosts = [{ required: true, message: '请输入节点地址', trigger: 'blur' }]
    }
  }

  if (['hive', 'kyuubi', 'spark', 'trino'].includes(form.value.type)) {
    base.auth_type = [{ required: true, message: '请选择认证方式', trigger: 'change' }]
    base.connection_mode = [{ required: true, message: '请选择连接模式', trigger: 'change' }]
    
    if (form.value.connection_mode === 'direct') {
      base.host = [{ required: true, message: '请输入主机地址', trigger: 'blur' }]
      base.port = [{ required: true, message: '请输入端口', trigger: 'blur' }]
    } else if (form.value.connection_mode === 'zookeeper') {
      base.zk_hosts = [{ required: true, message: '请输入ZooKeeper地址', trigger: 'blur' }]
      base.zk_path = [{ required: true, message: '请输入ZooKeeper路径', trigger: 'blur' }]
    }
  }

  return base
})

const defaultPorts: Record<string, number> = {
  mysql: 3306,
  starrocks: 9030,
  doris: 9030,
  rqlite: 4001,
  hive: 10000,
  kyuubi: 10009,
  spark: 10000,
  trino: 8080,
}

const handleTypeChange = (type: string) => {
  form.value.port = defaultPorts[type] || 3306
  form.value.connection_mode = type === 'rqlite' ? 'single' : 'direct'
  form.value.auth_type = 'simple'
  form.value.transport_mode = 'binary'
  form.value.http_path = ''
  form.value.ssl = false
  form.value.rqlite_hosts = ''
  testPassed.value = false
}

const handleRqliteConnectionModeChange = () => {
  form.value.rqlite_hosts = ''
  form.value.host = ''
  form.value.port = defaultPorts[form.value.type] || 4001
  testPassed.value = false
}

const handleConnectionModeChange = () => {
  form.value.zk_hosts = ''
  form.value.zk_path = ''
  form.value.host = ''
  form.value.port = defaultPorts[form.value.type] || 3306
  testPassed.value = false
}

const buildSubmitData = () => {
  const data: Record<string, any> = {
    name: form.value.name,
    type: form.value.type,
    description: form.value.description,
    is_enabled: form.value.is_enabled,
    allow_write_sql: form.value.allow_write_sql,
    domain_id: form.value.domain_id || authStore.currentDomainId || 1,
  }

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
    if (form.value.connection_mode === 'single') {
      data.host = form.value.host
      data.port = form.value.port
    } else if (form.value.connection_mode === 'multi') {
      data.rqlite_hosts = form.value.rqlite_hosts
    }
    data.username = form.value.username
    data.password = form.value.password
    const config: Record<string, any> = {}
    if (form.value.ssl) {
      config.ssl = true
    }
    data.config = JSON.stringify(config)
  }

  if (['hive', 'kyuubi', 'spark', 'trino'].includes(form.value.type)) {
    data.auth_type = form.value.auth_type
    data.connection_mode = form.value.connection_mode
    data.database = form.value.database
    data.username = form.value.username
    data.password = form.value.password
    
    if (form.value.connection_mode === 'direct') {
      data.host = form.value.host
      data.port = form.value.port
    } else if (form.value.connection_mode === 'zookeeper') {
      data.zk_hosts = form.value.zk_hosts
      data.zk_path = form.value.zk_path
    }

    const config: Record<string, any> = {}
    if (form.value.transport_mode && form.value.transport_mode !== 'binary') {
      config.transport_mode = form.value.transport_mode
    }
    if (form.value.http_path) {
      config.http_path = form.value.http_path
    }
    if (form.value.ssl) {
      config.ssl = true
    }
    if (Object.keys(config).length > 0) {
      data.config = JSON.stringify(config)
    }
  }

  return data
}

const handleTestConnection = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    testing.value = true
    try {
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
      ElMessage.error(err.response?.data?.error || err.message || '连接测试失败')
    } finally {
      testing.value = false
    }
  })
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const data = buildSubmitData()
      if (isEdit.value) {
        await datasourceAPI.update(Number(route.params.id), data)
        ElMessage.success('数据源更新成功')
      } else {
        await datasourceAPI.create(data)
        ElMessage.success('数据源创建成功')
      }
      router.push({ name: 'Datasources' })
    } catch (err: any) {
      ElMessage.error(err.message || '操作失败')
    } finally {
      submitting.value = false
    }
  })
}

const handleBack = () => {
  router.push({ name: 'Datasources' })
}

const loadDatasource = async () => {
  if (!isEdit.value) return
  try {
    const res = await datasourceAPI.get(Number(route.params.id))
    const ds = res.data
    let configData: Record<string, any> = {}
    if (ds.config) {
      try {
        configData = JSON.parse(ds.config)
      } catch (e) {}
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
      connection_mode: (ds as any).connection_mode || (ds.type === 'rqlite' ? 'single' : 'direct'),
      zk_hosts: (ds as any).zk_hosts || '',
      zk_path: (ds as any).zk_path || '',
      rqlite_hosts: (ds as any).rqlite_hosts || '',
      transport_mode: configData.transport_mode || 'binary',
      http_path: configData.http_path || '',
      ssl: configData.ssl || false,
      description: ds.description || '',
      is_enabled: ds.is_enabled,
      allow_write_sql: ds.allow_write_sql || false,
      domain_id: ds.domain_id || 0,
    }
  } catch (err: any) {
    ElMessage.error(err.message || '加载数据源失败')
  }
}

watch(
  () => [
    form.value.type, form.value.host, form.value.port, form.value.path,
    form.value.database, form.value.username, form.value.password,
    form.value.auth_type, form.value.connection_mode, form.value.zk_hosts,
    form.value.zk_path, form.value.transport_mode, form.value.http_path, form.value.ssl,
  ],
  () => {
    if (!isEdit.value) {
      testPassed.value = false
    }
  }
)

onMounted(() => {
  loadDatasource()
})
</script>

<style scoped>
.datasource-form-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
  overflow-y: auto;
}

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

.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.form-container {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  padding: var(--space-4);
}

.ds-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.3s ease;
}

.form-section:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.1);
}

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

.section-body {
  padding: 20px;
}

.switch-wrapper {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.switch-text {
  font-size: 0.9rem;
  color: var(--text-secondary);
  font-weight: 500;
}

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

.dml-warning-tip .el-icon {
  margin-top: 2px;
  flex-shrink: 0;
}

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

.form-footer :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary), #6366f1);
  border: none;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.form-footer :deep(.el-button--primary:hover) {
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
  filter: brightness(1.05);
}

.test-hint {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.8rem;
  color: #e6a23c;
}

.test-passed-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
</style>
