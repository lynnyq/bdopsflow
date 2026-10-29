<!-- 数据源权限管理页面：管理指定数据源的用户/角色访问权限，支持权限授予、修改和撤销 -->
<template>
  <div class="datasource-permission-page">
    <!-- 页面头部：返回按钮、数据源信息、统计 -->
    <div class="page-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" @click="handleBack" class="back-btn">返回列表</el-button>
        <!-- 头部区域 -->
        <div class="header-divider"></div>
        <!-- 头部区域 -->
        <div class="header-info">
          <h2 class="page-title">权限管理</h2>
          <!-- 数据源 -->
          <div class="ds-badge">
            <el-icon><Connection /></el-icon>
            <span>{{ datasourceName || '加载数据源...' }}</span>
          </div>
        </div>
      </div>
      <!-- 头部区域 -->
      <div class="header-stats">
        <!-- 统计 -->
        <div class="stat-item">
          <span class="stat-value">{{ permissions.length }}</span>
          <span class="stat-label">权限总数</span>
        </div>
        <!-- 统计 -->
        <div class="stat-divider"></div>
        <!-- 统计 -->
        <div class="stat-item">
          <span class="stat-value">{{ uniqueUsersCount }}</span>
          <span class="stat-label">已授权用户</span>
        </div>
        <!-- 统计 -->
        <div class="stat-divider"></div>
        <!-- 统计 -->
        <div class="stat-item">
          <span class="stat-value">{{ uniqueRolesCount }}</span>
          <span class="stat-label">已授权角色</span>
        </div>
      </div>
    </div>

    <!-- 搜索和筛选工具栏 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索用户、角色或权限..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-select v-model="filterType" placeholder="权限类型" clearable class="filter-select">
          <el-option
            v-for="(label, key) in permissionLabels"
            :key="key"
            :label="label"
            :value="key"
          >
            <!-- 筛选器 -->
            <div class="filter-option">
              <el-icon :size="14"><component :is="getPermissionIcon(key)" /></el-icon>
              <span>{{ label }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select v-model="filterTarget" placeholder="授权对象" clearable class="filter-select">
          <el-option label="用户" value="user" />
          <el-option label="角色" value="role" />
        </el-select>
      </div>
      <!-- 工具栏右侧 -->
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadPermissions" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Plus" @click="handleAddPermission" class="create-btn">添加权限</el-button>
      </div>
    </div>

    <!-- 权限列表表格 -->
    <div class="table-wrapper">
      <el-table :data="filteredPermissions" v-loading="loading" stripe height="100%">
        <!-- ID 列 -->
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <!-- 授权对象列：用户或角色 -->
        <el-table-column label="授权对象" :min-width="200">
          <template #default="{ row }">
            <div class="target-cell">
              <!-- 用户 -->
              <div class="target-avatar" :class="row.user_id ? 'user-avatar' : 'role-avatar'">
                <el-icon :size="16"><component :is="row.user_id ? User : UserFilled" /></el-icon>
              </div>
              <!-- 信息 -->
              <div class="target-info">
                <span class="target-name">
                  <el-tag v-if="row.user_id" size="small" type="success" effect="plain" class="target-tag">用户</el-tag>
                  <el-tag v-else size="small" type="" effect="plain" class="target-tag">角色</el-tag>
                  {{ row.user_id ? getUserName(row.user_id) : getRoleName(row.role_id!) }}
                </span>
                <span class="target-id">ID: {{ row.user_id || row.role_id }}</span>
              </div>
            </div>
          </template>
        </el-table-column>
        <!-- 权限类型列：显示徽章和包含关系 -->
        <el-table-column prop="permission_type" label="权限类型" width="200" align="center">
          <template #default="{ row }">
            <!-- 权限徽章展示 -->
            <div class="permission-cell">
              <div class="permission-badge" :class="`permission-${row.permission_type}`">
                <el-icon :size="14"><component :is="getPermissionIcon(row.permission_type)" /></el-icon>
                <span>{{ getPermissionLabel(row.permission_type) }}</span>
              </div>
              <div v-if="permissionHierarchy[row.permission_type]?.length" class="permission-includes">
                含 {{ permissionHierarchy[row.permission_type].map(k => permissionLabels[k]).join('、') }}
              </div>
            </div>
          </template>
        </el-table-column>
        <!-- 授权人列 -->
        <el-table-column prop="granted_by" label="授权人" width="140" align="center">
          <template #default="{ row }">
            <div class="granted-by-cell">
              <el-icon :size="14"><User /></el-icon>
              <span>{{ row.granted_by || '-' }}</span>
            </div>
          </template>
        </el-table-column>
        <!-- 授权时间列 -->
        <el-table-column prop="granted_at" label="授权时间" width="180">
          <template #default="{ row }">
            <div class="time-cell">
              <el-icon :size="14"><Clock /></el-icon>
              <span>{{ formatDateTime(row.granted_at) }}</span>
            </div>
          </template>
        </el-table-column>
        <!-- 操作列：修改/删除权限 -->
        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleEditPermission(row)" class="edit-btn">
              <el-icon><Edit /></el-icon>
              <span>修改</span>
            </el-button>
            <el-button type="danger" link size="small" @click="handleDeletePermission(row)" class="delete-btn">
              <el-icon><Delete /></el-icon>
              <span>删除</span>
            </el-button>
          </template>
        </el-table-column>
        <!-- 表格空状态：无权限数据提示 -->
        <template #empty>
          <div class="table-empty-state">
            <div class="empty-icon">
              <el-icon :size="48"><Lock /></el-icon>
            </div>
            <p class="empty-title">暂无权限数据</p>
            <span class="empty-hint">点击"添加权限"为用户或角色分配数据源访问权限</span>
          </div>
        </template>
      </el-table>
    </div>

    <!-- 添加权限对话框 -->
    <el-dialog
      v-model="addDialogVisible"
      title="添加权限"
      width="560px"
      :close-on-click-modal="false"
      class="permission-dialog"
    >
      <!-- 对话框内容容器 -->
      <div class="dialog-content">
        <!-- 授权对象选择区块 -->
        <div class="dialog-section">
          <!-- 区块标题：图标 + 主标题 + 副标题 -->
          <div class="section-header">
            <div class="section-icon target-icon">
              <el-icon :size="18"><User /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">选择授权对象</span>
              <span class="title-desc">选择按用户或按角色分配数据源访问权限</span>
            </div>
          </div>
          <!-- 区块内容 -->
          <div class="section-body">
            <el-form ref="addFormRef" :model="addForm" :rules="addRules" label-position="top" class="dialog-form">
              <el-form-item label="授权方式" prop="target_type">
                <el-radio-group v-model="addForm.target_type" @change="handleTargetTypeChange">
                  <el-radio value="user">
                    <div class="radio-label">
                      <el-icon :size="14"><User /></el-icon>
                      <span>按用户</span>
                    </div>
                  </el-radio>
                  <el-radio value="role">
                    <div class="radio-label">
                      <el-icon :size="14"><UserFilled /></el-icon>
                      <span>按角色</span>
                    </div>
                  </el-radio>
                </el-radio-group>
              </el-form-item>
              <el-form-item v-if="addForm.target_type === 'user'" label="选择用户" prop="user_id">
                <el-select v-model="addForm.user_id" placeholder="请选择用户" style="width: 100%" filterable>
                  <el-option
                    v-for="u in users"
                    :key="u.id"
                    :label="u.username"
                    :value="u.id"
                  >
                    <div class="user-select-option">
                      <div class="user-select-avatar">
                        <el-icon :size="14"><User /></el-icon>
                      </div>
                      <span class="user-select-name">{{ u.username }}</span>
                      <el-tag size="small" :type="getRoleTagType(u.role_codes?.[0] || '')" effect="plain">{{ getRoleDisplayName(u.role_codes?.[0] || '') }}</el-tag>
                      <span class="user-select-id">ID: {{ u.id }}</span>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>
              <el-form-item v-if="addForm.target_type === 'role'" label="选择角色" prop="role_id">
                <el-select v-model="addForm.role_id" placeholder="请选择角色" style="width: 100%" filterable>
                  <el-option
                    v-for="role in roles"
                    :key="role.id"
                    :label="role.name"
                    :value="role.id"
                  >
                    <div class="role-select-option">
                      <div class="role-select-avatar">
                        <el-icon :size="14"><UserFilled /></el-icon>
                      </div>
                      <span class="role-select-name">{{ role.name }}</span>
                      <el-tag v-if="role.is_system" size="small" type="danger" effect="plain">系统</el-tag>
                      <span class="role-select-id">ID: {{ role.id }}</span>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>
            </el-form>
          </div>
        </div>

        <!-- 权限类型配置区块 -->
        <div class="dialog-section">
          <!-- 权限选择区块标题 -->
          <div class="section-header">
            <div class="section-icon perm-icon">
              <el-icon :size="18"><Lock /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">权限配置</span>
              <span class="title-desc">选择要授予的权限类型</span>
            </div>
          </div>
          <!-- 权限选择表单区 -->
          <div class="section-body">
            <el-form :model="addForm" :rules="addRules" label-position="top" class="dialog-form">
              <el-form-item label="权限类型" prop="permission_type">
                <el-select v-model="addForm.permission_type" placeholder="请选择权限类型" style="width: 100%" teleported popper-class="permission-select-popper">
                  <el-option
                    v-for="(label, key) in permissionLabels"
                    :key="key"
                    :label="label"
                    :value="key"
                  >
                    <div class="perm-select-option">
                      <div class="perm-badge-mini" :class="`permission-${key}`">
                        <el-icon :size="12"><component :is="getPermissionIcon(key)" /></el-icon>
                      </div>
                      <div class="perm-select-info">
                        <span class="perm-select-label">{{ label }}</span>
                        <span class="perm-select-desc">{{ permissionDescs[key] }}</span>
                      </div>
                    </div>
                  </el-option>
                </el-select>
                <div v-if="addForm.permission_type && permissionHierarchy[addForm.permission_type]?.length" class="perm-hierarchy-hint">
                  <el-icon :size="14"><Check /></el-icon>
                  <span>自动包含：{{ permissionHierarchy[addForm.permission_type].map(k => permissionLabels[k]).join('、') }}权限</span>
                </div>
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="addDialogVisible = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleSubmitAdd" :loading="submitting" size="large">
            <el-icon><Check /></el-icon>
            确认添加
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="editDialogVisible"
      title="修改权限"
      width="480px"
      :close-on-click-modal="false"
      class="permission-dialog"
    >
      <div class="dialog-content">
        <div class="dialog-section">
          <div class="section-header">
            <div class="section-icon target-icon">
              <el-icon :size="18"><User /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">授权对象</span>
              <span class="title-desc">授权对象不可修改</span>
            </div>
          </div>
          <div class="section-body">
            <div class="readonly-target">
              <el-tag v-if="editForm.target_type === 'user'" size="small" type="success" effect="plain">用户</el-tag>
              <el-tag v-else size="small" effect="plain">角色</el-tag>
              <span class="readonly-target-name">{{ editForm.target_name }}</span>
            </div>
          </div>
        </div>

        <div class="dialog-section">
          <div class="section-header">
            <div class="section-icon perm-icon">
              <el-icon :size="18"><Lock /></el-icon>
            </div>
            <div class="section-title">
              <span class="title-text">权限配置</span>
              <span class="title-desc">修改权限类型</span>
            </div>
          </div>
          <div class="section-body">
            <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-position="top" class="dialog-form">
              <el-form-item label="权限类型" prop="permission_type">
                <el-select v-model="editForm.permission_type" placeholder="请选择权限类型" style="width: 100%" teleported popper-class="permission-select-popper">
                  <el-option
                    v-for="(label, key) in permissionLabels"
                    :key="key"
                    :label="label"
                    :value="key"
                  >
                    <div class="perm-select-option">
                      <div class="perm-badge-mini" :class="`permission-${key}`">
                        <el-icon :size="12"><component :is="getPermissionIcon(key)" /></el-icon>
                      </div>
                      <div class="perm-select-info">
                        <span class="perm-select-label">{{ label }}</span>
                        <span class="perm-select-desc">{{ permissionDescs[key] }}</span>
                      </div>
                    </div>
                  </el-option>
                </el-select>
                <div v-if="editForm.permission_type && permissionHierarchy[editForm.permission_type]?.length" class="perm-hierarchy-hint">
                  <el-icon :size="14"><Check /></el-icon>
                  <span>自动包含：{{ permissionHierarchy[editForm.permission_type].map(k => permissionLabels[k]).join('、') }}权限</span>
                </div>
              </el-form-item>
            </el-form>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="editDialogVisible = false" size="large">取消</el-button>
          <el-button type="primary" @click="handleSubmitEdit" :loading="submitting" size="large">
            <el-icon><Check /></el-icon>
            确认修改
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * 数据源权限管理组件
 *
 * 用于管理员为指定数据源分配权限。支持按用户或角色授予以下权限：
 * - read    读取（查看配置和元数据）
 * - query   查询（执行 SQL 查询，含读取）
 * - download 下载（导出结果，含查询、读取）
 * - update  更新（修改数据源配置，含下载、查询、读取）
 * - delete  删除
 * - manage  管理（拥有所有权限，含权限管理）
 *
 * 主要功能：
 * 1. 查看当前数据源的所有权限条目
 * 2. 按名称、权限类型、目标类型（用户/角色）搜索过滤
 * 3. 添加 / 修改 / 删除权限条目
 * 4. 根据管理员作用域（系统管理员 / 领域管理员）自动过滤可分配的角色和用户
 */

// ==== Vue / UI 基础依赖 ====
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import {
  ArrowLeft, Plus, Delete, Lock, Refresh, Search, Connection, User, UserFilled,
  View, Download, Edit, Setting, Check, Clock
} from '@element-plus/icons-vue'

// ==== 业务 API 与工具 ====
import { datasourceAPI } from '@/api'
import { isHandledError } from '@/utils/api'
import { formatErrorMessage } from '@/utils/error'
import { roleAdminAPI, userAdminAPI } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'
import type { DatasourcePermission } from '@/types'
import type { Role, User as AdminUser } from '@/api/admin'

// ==== 路由 / 认证 store ====
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

// ==== 基础响应式状态 ====

/** 当前数据源的 ID（从路由参数中解析） */
const datasourceId = Number(route.params.id)

/** 数据源显示名称 */
const datasourceName = ref('')

/** 数据源所属领域的 ID，用于后续按领域过滤用户列表 */
const datasourceDomainId = ref<number | null>(null)

/** 已授予的权限条目列表 */
// ========== 权限列表状态 ==========
const permissions = ref<DatasourcePermission[]>([])

/** 可分配的角色列表（根据当前管理员作用域过滤） */
const roles = ref<Role[]>([])

/** 可分配的用户列表（根据当前管理员作用域过滤） */
// ========== 用户/角色数据 ==========
const users = ref<AdminUser[]>([])

/** 数据加载中（用于表格 loading 状态） */
const loading = ref(false)

/** 表单提交中（用于提交按钮 loading / 防重复提交） */
const submitting = ref(false)

/** 新增权限弹窗是否可见 */
const addDialogVisible = ref(false)

/** 编辑权限弹窗是否可见 */
const editDialogVisible = ref(false)

/** 新增权限的 el-form 实例引用 */
const addFormRef = ref<FormInstance>()

/** 编辑权限的 el-form 实例引用 */
const editFormRef = ref<FormInstance>()

/** 搜索关键字（匹配目标名称或权限类型） */
// ========== 基础状态 ==========
const searchQuery = ref('')

/** 权限类型筛选（read/query/download/update/delete/manage） */
const filterType = ref<string | null>(null)

/** 目标类型筛选（user / role） */
const filterTarget = ref<string | null>(null)

// ==== 表单定义与校验 ====

/** 新增权限表单数据 */
const addForm = ref({
  /** 授权目标类型：user（用户）或 role（角色） */
  target_type: 'user' as 'user' | 'role',
  /** 被授权的用户 ID（当 target_type === 'user' 时有效） */
  user_id: null as number | null,
  /** 被授权的角色 ID（当 target_type === 'role' 时有效） */
  role_id: null as number | null,
  /** 授予的权限类型（read/query/download/update/delete/manage） */
  permission_type: '',
})

/** 编辑权限表单数据（只允许修改权限类型） */
const editForm = ref({
  /** 被编辑的权限条目 ID */
  id: 0,
  /** 目标类型：user 或 role（由原始条目推导） */
  target_type: 'user' as 'user' | 'role',
  /** 目标显示名称（用户 / 角色名），只读展示 */
  target_name: '',
  /** 修改后的权限类型 */
  permission_type: '',
})

/**
 * 动态校验器：根据当前 target_type 校验对应的 user_id 或 role_id 是否填写
 * @param _rule Element Plus 规则对象
 * @param _value 被校验的值（此处未直接使用，因为校验依赖 addForm.target_type）
 * @param callback Element Plus 校验回调
 */
const validateTarget = (_rule: any, _value: any, callback: any) => {
  if (addForm.value.target_type === 'user' && !addForm.value.user_id) {
    callback(new Error('请选择用户'))
  } else if (addForm.value.target_type === 'role' && !addForm.value.role_id) {
    callback(new Error('请选择角色'))
  } else {
    callback()
  }
}

/** 新增权限表单的校验规则 */
const addRules: FormRules = {
  target_type: [{ required: true, message: '请选择授权方式', trigger: 'change' }],
  user_id: [{ validator: validateTarget, trigger: 'change' }],
  role_id: [{ validator: validateTarget, trigger: 'change' }],
  permission_type: [{ required: true, message: '请选择权限类型', trigger: 'change' }],
}

/** 编辑权限表单的校验规则（仅允许修改 permission_type） */
const editRules: FormRules = {
  permission_type: [{ required: true, message: '请选择权限类型', trigger: 'change' }],
}

// ==== 权限常量定义 ====

/** 权限类型 -> 中文标签 */
// ========== 权限常量定义 ==========
const permissionLabels: Record<string, string> = {
  read: '读取',
  query: '查询',
  download: '下载',
  update: '更新',
  delete: '删除',
  manage: '管理',
}

/** 权限类型 -> 详细描述（展示给用户了解权限含义） */
const permissionDescs: Record<string, string> = {
  read: '查看数据源配置和元数据',
  query: '执行 SQL 查询（含读取）',
  download: '导出查询结果（含查询、读取）',
  update: '修改数据源配置（含下载、查询、读取）',
  delete: '删除数据源',
  manage: '管理权限和配置（含所有权限）',
}

/**
 * 权限层级依赖表：某权限会自动包含的子权限列表
 * 用于前端展示权限说明，后端也需同等处理
 */
const permissionHierarchy: Record<string, string[]> = {
  read: [],
  query: ['read'],
  download: ['query', 'read'],
  update: ['download', 'query', 'read'],
  delete: [],
  manage: ['update', 'download', 'query', 'read', 'delete'],
}

// ==== 计算属性 ====

/** 当前权限条目中涉及的唯一用户数量（用于顶部统计） */
const uniqueUsersCount = computed(() => {
  const userIds = new Set(permissions.value.filter(p => p.user_id).map(p => p.user_id))
  return userIds.size
})

/** 当前权限条目中涉及的唯一角色数量（用于顶部统计） */
const uniqueRolesCount = computed(() => {
  const roleIds = new Set(permissions.value.filter(p => p.role_id).map(p => p.role_id))
  return roleIds.size
})

/**
 * 根据搜索关键字、权限类型、目标类型筛选权限条目
 * - 搜索匹配：目标用户名 / 角色名 / 权限类型（大小写不敏感）
 * - 类型筛选：精确匹配 permission_type
 * - 目标筛选：按 user / role 过滤
 */
// filteredPermissions
const filteredPermissions = computed(() => {
  return permissions.value.filter(p => {
    const targetName = p.user_id
      ? getUserName(p.user_id).toLowerCase()
      : getRoleName(p.role_id!).toLowerCase()
    const matchSearch = !searchQuery.value ||
      targetName.includes(searchQuery.value.toLowerCase()) ||
      p.permission_type.toLowerCase().includes(searchQuery.value.toLowerCase())
    const matchType = !filterType.value || p.permission_type === filterType.value
    const matchTarget = !filterTarget.value ||
      (filterTarget.value === 'user' && p.user_id) ||
      (filterTarget.value === 'role' && p.role_id)
    return matchSearch && matchType && matchTarget
  })
})

// ==== 辅助函数 ====

/**
 * 根据用户 ID 查找用户显示名
 * @param userId 用户 ID
 * @returns 用户用户名；找不到时返回 `用户 {userId}`
 */
const getUserName = (userId: number) => {
  const user = users.value.find((u: AdminUser) => u.id === userId)
  return user ? user.username : `用户 ${userId}`
}

/**
 * 根据角色 ID 查找角色名称
 * @param roleId 角色 ID
 * @returns 角色名称；找不到时返回 `角色 {roleId}`
 */
const getRoleName = (roleId: number) => {
  const role = roles.value.find((r: Role) => r.id === roleId)
  return role ? role.name : `角色 ${roleId}`
}

/**
 * 将后端角色标识（如 system_admin）转换为中文显示名
 * @param role 后端返回的角色标识
 * @returns 中文显示名
 */
const getRoleDisplayName = (role: string) => {
  const map: Record<string, string> = {
    system_admin: '系统管理员',
    admin: '管理员',
    domain_admin: '领域管理员',
    user: '普通用户',
  }
  return map[role] || role
}

/**
 * 根据角色标识返回 Element Plus Tag 组件的 type，用于区分严重程度
 * @param role 角色标识
 * @returns 'danger' | 'warning' | '' | 'info'
 */
const getRoleTagType = (role: string) => {
  const map: Record<string, string> = {
    system_admin: 'danger',
    admin: 'warning',
    domain_admin: '',
    user: 'info',
  }
  return map[role] || 'info'
}

/**
 * 根据权限类型返回对应的 Element Plus 图标组件
 * @param type 权限类型（read/query/download/update/delete/manage）
 * @returns Vue 图标组件
 */
const getPermissionIcon = (type: string) => {
  const iconMap: Record<string, any> = {
    read: View,
    query: Search,
    download: Download,
    update: Edit,
    delete: Delete,
    manage: Setting,
  }
  return iconMap[type] || Lock
}

/**
 * 根据权限类型返回中文标签
 * @param type 权限类型
 * @returns 中文标签
 */
const getPermissionLabel = (type: string) => {
  return permissionLabels[type] || type
}

/**
 * 将后端 ISO 日期字符串格式化为本地化的中文显示字符串
 * @param dateStr ISO 格式日期字符串（为空时返回 '-'）
 * @returns 格式化后的日期时间，如 2026/10/29 14:30:00
 */
const formatDateTime = (dateStr: string) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// ==== 数据加载函数 ====

/**
 * 加载数据源基本信息（名称、所属领域）
 * 页面进入时调用，用于在权限列表头部展示数据源名称，
 * 并在领域管理员场景下，为后续按领域过滤用户提供 domain_id
 */
const loadDatasource = async () => {
  try {
    const res = await datasourceAPI.get(datasourceId)
    datasourceName.value = res.data.name
    datasourceDomainId.value = res.data.domain_id
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error(formatErrorMessage(err))
    }
  }
}

/**
 * 加载当前数据源的全部权限条目
 * 读取权限列表并兼容后端返回数组或 { items: [] } 两种格式
 */
// ========== 数据加载 ==========
const loadPermissions = async () => {
  loading.value = true
  try {
    const res = await datasourceAPI.getPermissions(datasourceId)
    const data = res.data
    if (Array.isArray(data)) {
      permissions.value = data
    } else if (data && Array.isArray((data as any).items)) {
      permissions.value = (data as any).items
    } else {
      permissions.value = []
    }
  } catch (err: unknown) {
    if (!isHandledError(err)) {
      ElMessage.error(formatErrorMessage(err))
    }
  } finally {
    loading.value = false
  }
}

/**
 * 加载可分配的角色列表
 * - 系统管理员：返回全部角色
 * - 领域管理员：仅返回非系统角色，以及属于当前领域或未归属特定领域的角色
 */
const loadRoles = async () => {
  try {
    const res = await roleAdminAPI.list()
    const allRoles = res.data?.items || []
    if (authStore.isSystemAdmin) {
      roles.value = allRoles
    } else {
      const userDomainId = authStore.currentDomainId
      roles.value = allRoles.filter((r: Role) => {
        if (r.is_system) return false
        if (r.domain_id == null || r.domain_id === undefined) return true
        return r.domain_id === userDomainId
      })
    }
  } catch (err: unknown) {
  }
}

/**
 * 加载可分配的用户列表
 * - 系统管理员：查询全量用户（domainId 传 0 代表不限）
 * - 领域管理员：优先按数据源所属领域过滤；若未拿到则回退到 authStore.currentDomainId
 */
const loadUsers = async () => {
  try {
    let domainId: number | undefined
    if (authStore.isSystemAdmin) {
      domainId = 0
    } else {
      domainId = datasourceDomainId.value || authStore.currentDomainId || undefined
    }
    const res = await userAdminAPI.listByDomain(domainId)
    users.value = res.data?.items || []
  } catch (err: unknown) {
  }
}

// ==== 事件处理函数 ====

/**
 * 新增权限表单的 target_type 切换回调
 * 切换用户/角色授权方式时，清空另一个选择器的已选值，避免残留
 */
const handleTargetTypeChange = () => {
  addForm.value.user_id = null
  addForm.value.role_id = null
}

/** 点击「添加权限」按钮：重置表单并打开新增弹窗 */
// ========== 权限 CRUD ==========
const handleAddPermission = () => {
  addForm.value = { target_type: 'user', user_id: null, role_id: null, permission_type: '' }
  addDialogVisible.value = true
}

/**
 * 点击某条权限的「编辑」按钮：回填编辑表单并打开编辑弹窗
 * @param row 当前行的权限条目
 */
const handleEditPermission = (row: DatasourcePermission) => {
  editForm.value = {
    id: row.id,
    target_type: row.user_id ? 'user' : 'role',
    target_name: row.user_id ? getUserName(row.user_id) : getRoleName(row.role_id!),
    permission_type: row.permission_type,
  }
  editDialogVisible.value = true
}

/**
 * 编辑弹窗的确认提交：校验后调用 updatePermission 更新 permission_type
 */
const handleSubmitEdit = async () => {
  if (!editFormRef.value) return
  await editFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      await datasourceAPI.updatePermission(datasourceId, editForm.value.id, {
        permission_type: editForm.value.permission_type,
      })
      ElMessage.success('权限修改成功')
      editDialogVisible.value = false
      await loadPermissions()
    } catch (err: unknown) {
      if (!isHandledError(err)) {
        ElMessage.error(formatErrorMessage(err))
      }
    } finally {
      submitting.value = false
    }
  })
}

/**
 * 新增弹窗的确认提交：根据 target_type 组装 grantPermission 的请求体
 * user_id 与 role_id 二选一，由 target_type 决定
 */
const handleSubmitAdd = async () => {
  if (!addFormRef.value) return
  await addFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const data: { role_id?: number; user_id?: number; permission_type: string } = {
        permission_type: addForm.value.permission_type,
      }
      if (addForm.value.target_type === 'user') {
        data.user_id = addForm.value.user_id!
      } else {
        data.role_id = addForm.value.role_id!
      }
      await datasourceAPI.grantPermission(datasourceId, data)
      ElMessage.success('权限添加成功')
      addDialogVisible.value = false
      await loadPermissions()
    } catch (err: unknown) {
      if (!isHandledError(err)) {
        ElMessage.error(formatErrorMessage(err))
      }
    } finally {
      submitting.value = false
    }
  })
}

/**
 * 点击「删除」按钮：弹确认框后调用 revokePermission 撤销指定条目
 * @param row 被删除的权限条目
 */
const handleDeletePermission = async (row: DatasourcePermission) => {
  const targetName = row.user_id ? getUserName(row.user_id) : getRoleName(row.role_id!)
  const targetType = row.user_id ? '用户' : '角色'
  try {
    await ElMessageBox.confirm(
      `确定要删除${targetType} "${targetName}" 的 "${getPermissionLabel(row.permission_type)}" 权限吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await datasourceAPI.revokePermission(datasourceId, row.id)
    ElMessage.success('权限已删除')
    await loadPermissions()
  } catch (err: unknown) {
    if (err !== 'cancel' && !isHandledError(err)) {
      ElMessage.error(formatErrorMessage(err))
    }
  }
}

/** 返回数据源列表页 */
const handleBack = () => {
  router.push({ name: 'Datasources' })
}

// ==== 生命周期 ====

/** 组件挂载后：依次加载数据源信息（为后续按领域过滤铺路），再并行加载权限、角色、用户 */
onMounted(async () => {
  await loadDatasource()
  loadPermissions()
  loadRoles()
  loadUsers()
})
</script>

/* ==================== 样式定义 ==================== */
<style scoped>
/* 页面整体容器 */
.datasource-permission-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  height: 100%;
  min-height: 0;
}

/* 页面头部 */
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
}

/* 头部左侧 */
.header-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

/* 返回列表按钮 */
.back-btn {
  font-weight: 500;
  color: var(--text-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--bg-secondary);
  transition: all var(--duration-normal) var(--ease-out);
}

.back-btn:hover {
  color: var(--accent-primary);
  border-color: var(--accent-primary);
  background: rgba(59, 130, 246, 0.04);
}

/* 头部分隔线 */
.header-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
}

/* 头部信息：页面标题 + 数据源名称 */
.header-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

/* 页面主标题 */
.page-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

/* 数据源信息徽章 */
.ds-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(99, 102, 241, 0.1));
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 500;
  color: var(--accent-primary);
  width: fit-content;
}

/* 头部统计信息 */
.header-stats {
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding: var(--space-2) var(--space-4);
  background: var(--bg-secondary);
  border-radius: var(--radius-lg);
}

/* 单个统计项 */
.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

/* 统计数值 */
.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--accent-primary);
  line-height: 1;
}

/* 统计标签 */
.stat-label {
  font-size: 12px;
  color: var(--text-muted);
}

/* 统计项分隔线 */
.stat-divider {
  width: 1px;
  height: 32px;
  background: var(--border-default);
}

/* 页面工具栏 */
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

/* 工具栏左侧：搜索框 + 筛选器 */
.toolbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
}

/* 工具栏右侧：刷新 + 添加权限按钮 */
.toolbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

/* 搜索输入框 */
.search-input {
  width: 260px;
}

.search-input :deep(.el-input__wrapper),
.filter-select :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.search-input :deep(.el-input__wrapper:hover),
.filter-select :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.search-input :deep(.el-input__wrapper.is-focus),
.filter-select :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

/* 筛选下拉框 */
.filter-select {
  width: 150px;
}

/* 下拉选项 */
.filter-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 刷新按钮 */
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

/* 添加权限按钮 */
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

/* 表格容器 */
.table-wrapper {
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  min-height: 0;
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

/* 授权对象单元格 */
.target-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* 头像容器 */
.target-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  color: white;
  box-shadow: 0 2px 6px rgba(59, 130, 246, 0.25);
}

/* 用户头像样式 */
.target-avatar.user-avatar {
  background: linear-gradient(135deg, #22c55e, #16a34a);
}

/* 角色头像样式 */
.target-avatar.role-avatar {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
}

/* 授权对象信息 */
.target-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* 授权对象名称 */
.target-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 6px;
}

/* 用户/角色类型标签 */
.target-tag {
  flex-shrink: 0;
}

/* 授权对象 ID */
.target-id {
  font-size: 11px;
  color: var(--text-muted);
}

/* 权限类型徽章 */
.permission-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 500;
}

.permission-badge.permission-read {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.permission-badge.permission-query {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

.permission-badge.permission-download {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.permission-badge.permission-update {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.permission-badge.permission-delete {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.permission-badge.permission-manage {
  background: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}

/* 权限类型单元格 */
.permission-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

/* 权限包含说明 */
.permission-includes {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.2;
}

/* 迷你权限徽章（用于下拉选项） */
.perm-badge-mini {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--radius-sm);
}

.perm-badge-mini.permission-read {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.perm-badge-mini.permission-query {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

.perm-badge-mini.permission-download {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.perm-badge-mini.permission-update {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.perm-badge-mini.permission-delete {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.perm-badge-mini.permission-manage {
  background: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}

/* 授权人单元格 */
.granted-by-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 13px;
}

/* 时间单元格 */
.time-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-secondary);
  font-size: 13px;
}

/* 删除按钮 */
.delete-btn {
  gap: 4px;
}

/* 表格空状态 */
.table-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: var(--space-10);
  gap: var(--space-3);
}

/* 空状态图标 */
.empty-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 80px;
  height: 80px;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.08), rgba(99, 102, 241, 0.08));
  border-radius: 50%;
  color: var(--text-muted);
}

/* 空状态标题 */
.empty-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

/* 空状态提示 */
.empty-hint {
  font-size: 0.875rem;
  color: var(--text-muted);
}

/* 对话框内容容器 */
.dialog-content {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

/* 对话框分区 */
.dialog-section {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all var(--duration-normal) var(--ease-out);
}

.dialog-section:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.08);
}

/* 分区头部：图标 + 标题 + 描述 */
.section-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.04), rgba(99, 102, 241, 0.04));
  border-bottom: 1px solid var(--border-default);
}

/* 分区图标 */
.section-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md);
  color: white;
}

.section-icon.target-icon {
  background: linear-gradient(135deg, #22c55e, #16a34a);
  box-shadow: 0 2px 8px rgba(34, 197, 94, 0.3);
}

.section-icon.perm-icon {
  background: linear-gradient(135deg, #8b5cf6, #a855f7);
  box-shadow: 0 2px 8px rgba(139, 92, 246, 0.3);
}

/* 分区标题区 */
.section-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* 分区主标题 */
.title-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

/* 分区副标题/描述 */
.title-desc {
  font-size: 12px;
  color: var(--text-muted);
}

/* 分区内容区 */
.section-body {
  padding: var(--space-4);
}

.dialog-form :deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--text-primary);
}

.dialog-form :deep(.el-input__wrapper),
.dialog-form :deep(.el-select .el-input__wrapper) {
  background: var(--bg-card);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.dialog-form :deep(.el-input__wrapper:hover),
.dialog-form :deep(.el-select .el-input__wrapper:hover) {
  border-color: var(--accent-primary);
}

.dialog-form :deep(.el-input__wrapper.is-focus),
.dialog-form :deep(.el-select .el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.15);
}

/* 单选项标签 */
.radio-label {
  display: flex;
  align-items: center;
  gap: 6px;
}

.readonly-target {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.readonly-target-name {
  font-weight: 500;
  color: var(--text-primary);
  font-size: 14px;
}

/* 修改按钮 */
.edit-btn {
  gap: 4px;
}

/* 用户下拉选项 */
.user-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* 用户下拉头像 */
.user-select-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: linear-gradient(135deg, #22c55e, #16a34a);
  border-radius: var(--radius-sm);
  color: white;
}

/* 用户下拉名称 */
.user-select-name {
  font-weight: 500;
  color: var(--text-primary);
}

/* 用户下拉 ID */
.user-select-id {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-muted);
}

/* 角色下拉选项 */
.role-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* 角色下拉头像 */
.role-select-avatar {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  border-radius: var(--radius-sm);
  color: white;
}

/* 角色下拉名称 */
.role-select-name {
  font-weight: 500;
  color: var(--text-primary);
}

/* 角色下拉 ID */
.role-select-id {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-muted);
}

/* 权限下拉选项 */
.perm-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* 权限下拉信息 */
.perm-select-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* 权限下拉主标签 */
.perm-select-label {
  font-weight: 500;
  color: var(--text-primary);
}

/* 权限下拉描述 */
.perm-select-desc {
  font-size: 12px;
  color: var(--text-muted);
}

/* 权限层级包含提示 */
.perm-hierarchy-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 8px 12px;
  background: rgba(34, 197, 94, 0.06);
  border: 1px solid rgba(34, 197, 94, 0.2);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: #16a34a;
}

.perm-hierarchy-hint .el-icon {
  color: #22c55e;
}

/* 对话框底部按钮区 */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
  padding-top: var(--space-3);
}

.dialog-footer :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  border: none;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.dialog-footer :deep(.el-button--primary:hover) {
  filter: brightness(1.05);
  box-shadow: 0 6px 16px rgba(59, 130, 246, 0.4);
}
</style>

/* ==================== 样式定义 ==================== */
<style>
.permission-select-popper {
  z-index: 9999 !important;
}

.permission-select-popper .el-select-dropdown__item {
  height: auto !important;
  padding: 10px 12px !important;
}

.permission-select-popper .perm-select-option {
  display: flex;
  align-items: center;
  gap: 10px;
}

.permission-select-popper .perm-select-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.permission-select-popper .perm-select-label {
  font-weight: 500;
}

.permission-select-popper .perm-select-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.permission-select-popper .perm-badge-mini {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
}

.permission-select-popper .perm-badge-mini.permission-read {
  background: rgba(59, 130, 246, 0.1);
  color: #3b82f6;
}

.permission-select-popper .perm-badge-mini.permission-query {
  background: rgba(99, 102, 241, 0.1);
  color: #6366f1;
}

.permission-select-popper .perm-badge-mini.permission-download {
  background: rgba(34, 197, 94, 0.1);
  color: #22c55e;
}

.permission-select-popper .perm-badge-mini.permission-update {
  background: rgba(245, 158, 11, 0.1);
  color: #f59e0b;
}

.permission-select-popper .perm-badge-mini.permission-delete {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.permission-select-popper .perm-badge-mini.permission-manage {
  background: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}
</style>
