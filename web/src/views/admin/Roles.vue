<!-- 角色权限管理页面：支持角色 CRUD 和为角色分配细粒度权限 -->
<template>
  <div class="roles-page">
    <!-- 页面工具栏：搜索框 + 刷新 + 创建按钮 -->
    <div class="page-toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="searchQuery"
          placeholder="搜索角色..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
      </div>
      <div class="toolbar-right">
        <el-button :icon="Refresh" @click="loadRoles" :loading="loading" class="refresh-btn">刷新</el-button>
        <el-button :icon="Plus" @click="showCreateDialog = true" class="create-btn">
          创建角色
        </el-button>
      </div>
    </div>

    <!-- Table -->
    <div class="table-wrapper">
      <el-table :data="filteredRoles" v-loading="loading" stripe height="100%">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="角色名称" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="code" label="角色代码" :minWidth="150" show-overflow-tooltip />
        <el-table-column prop="description" label="描述" :minWidth="200" show-overflow-tooltip />
        <el-table-column prop="is_system" label="类型" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_system ? 'danger' : 'success'" effect="light">
              {{ row.is_system ? '系统角色' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right" align="center">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleEdit(row)">
              <el-icon><Edit /></el-icon> 编辑
            </el-button>
            <el-button type="warning" link size="small" @click="handleAssignPermissions(row)">
              <el-icon><Key /></el-icon> 权限
            </el-button>
            <el-button
              v-if="!row.is_system"
              type="danger"
              link
              size="small"
              @click="handleDelete(row)"
            >
              <el-icon><Delete /></el-icon> 删除
            </el-button>
          </template>
        </el-table-column>
        <!-- 空状态：角色列表为空时展示的占位提示 -->
        <template #empty>
          <div class="table-empty-state">
            <el-icon :size="32"><Document /></el-icon>
            <p>暂无角色数据</p>
          </div>
        </template>
      </el-table>
    </div>

    <!-- 创建角色对话框 -->
    <el-dialog v-model="showCreateDialog" title="创建角色" width="500px" class="custom-dialog">
      <!-- 创建表单 -->
      <el-form :model="roleForm" :rules="roleRules" ref="formRef" label-width="100px" class="dialog-form">
        <!-- 角色名称输入 -->
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="roleForm.name" placeholder="请输入角色名称" />
        </el-form-item>
        <!-- 角色代码输入（小写字母+数字+下划线） -->
        <el-form-item label="角色代码" prop="code">
          <el-input v-model="roleForm.code" placeholder="请输入角色代码，如 custom_role" />
        </el-form-item>
        <!-- 角色描述 -->
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="roleForm.description"
            type="textarea"
            placeholder="请输入角色描述"
            :rows="3"
          />
        </el-form-item>
      </el-form>
      <!-- 创建对话框底部按钮 -->
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showCreateDialog = false">取消</el-button>
          <el-button type="primary" @click="handleCreate" :loading="submitting">创建</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 编辑角色对话框 -->
    <el-dialog v-model="showEditDialog" title="编辑角色" width="500px" class="custom-dialog">
      <!-- 编辑表单：允许修改 name 和 description，code 不可变更 -->
      <el-form :model="roleForm" :rules="roleRules" ref="editFormRef" label-width="100px" class="dialog-form">
        <!-- 角色名称输入 -->
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="roleForm.name" placeholder="请输入角色名称" />
        </el-form-item>
        <!-- 角色描述 -->
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="roleForm.description"
            type="textarea"
            placeholder="请输入角色描述"
            :rows="3"
          />
        </el-form-item>
      </el-form>
      <!-- 编辑对话框底部按钮 -->
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showEditDialog = false">取消</el-button>
          <el-button type="primary" @click="handleUpdate" :loading="submitting">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 权限分配对话框：按 resource 分组展示所有权限，支持全选/半选/单条勾选 -->
    <el-dialog v-model="showPermissionDialog" title="分配权限" width="720px" class="custom-dialog" :close-on-click-modal="false">
      <!-- 对话框内容区：loading + 系统角色告警 + 权限分组列表 -->
      <div v-loading="loadingPermissions" class="permission-dialog-content">
        <!-- 系统内置角色警告：权限不可手动修改 -->
        <el-alert
          v-if="currentRole?.is_system"
          title="系统角色"
          type="warning"
          :closable="false"
          description="系统角色的权限由系统预设，无法手动修改"
          show-icon
          class="system-role-alert"
        />
        <div class="permission-groups" :class="{ 'is-disabled': currentRole?.is_system }">
          <div
            v-for="group in permissionGroupsDisplay"
            :key="group.resource"
            class="permission-group"
          >
            <div class="group-header">
              <div class="group-icon">
                <el-icon><component :is="getResourceIcon(group.resource)" /></el-icon>
              </div>
              <div class="group-info">
                <span class="group-title">{{ group.resource_name }}</span>
                <span class="group-count">{{ group.permissions?.length || 0 }} 项权限</span>
              </div>
              <el-checkbox
                :model-value="isGroupAllSelected(group)"
                :indeterminate="isGroupIndeterminate(group)"
                @change="handleGroupSelectAll(group, $event)"
                :disabled="currentRole?.is_system"
                class="group-checkbox"
              >
                全选
              </el-checkbox>
            </div>
            <el-checkbox-group v-model="selectedPermissionIds">
              <div class="permission-list">
                <el-checkbox
                  v-for="perm in group.permissions"
                  :key="perm.id"
                  :value="perm.id"
                  :disabled="currentRole?.is_system"
                  class="permission-item"
                >
                  <div class="permission-content">
                    <span class="permission-name">{{ perm.description }}</span>
                    <el-tag size="small" type="info" effect="plain" class="permission-code">
                      {{ perm.resource }}:{{ perm.action }}
                    </el-tag>
                  </div>
                </el-checkbox>
              </div>
            </el-checkbox-group>
          </div>
        </div>
      </div>
      <!-- 权限对话框底部按钮：取消 / 保存权限（系统角色时隐藏保存按钮） -->
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showPermissionDialog = false">取消</el-button>
          <el-button
            v-if="!currentRole?.is_system"
            type="primary"
            @click="handleSavePermissions"
            :loading="submitting"
          >
            保存权限
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
/**
 * Roles.vue - 角色权限管理页面
 *
 * 提供角色的增删改查、以及为角色分配细粒度 resource/action 权限的对话框。
 * 权限分配按 resource 分组展示，支持全选/半选/单条勾选。
 * 系统内置角色（is_system=true）禁止修改和删除。
 */
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Document, Search, Refresh, Key, User, Setting, Grid, Timer, DataAnalysis, List, Lock, Connection, Monitor } from '@element-plus/icons-vue'
import {
  roleAdminAPI,
  permissionAPI,
  type Role,
  type Permission,
  type PermissionGroup
} from '@/api/admin'

import { isHandledError } from '@/utils/api'

// ============================================================================
// 页面级响应式状态（列表、筛选、对话框可见性、表单数据）
// ============================================================================

// ========== 页面状态 ==========

/** 全量角色列表 */
const roles = ref<Role[]>([])
/** 列表加载中状态 */
const loading = ref(false)
/** 提交表单中状态（按钮 loading） */
const submitting = ref(false)
/** 创建对话框可见性 */
const showCreateDialog = ref(false)
/** 编辑对话框可见性 */
const showEditDialog = ref(false)
/** 权限分配对话框可见性 */
const showPermissionDialog = ref(false)
/** 权限列表加载中 */
const loadingPermissions = ref(false)
/** 搜索关键字 */
const searchQuery = ref('')

/** 全量权限列表（用于权限分配） */
const permissions = ref<Permission[]>([])
/** 按 resource 分组后的权限展示结构 */
const permissionGroupsDisplay = ref<PermissionGroup[]>([])
/** 当前正在编辑权限的角色 */
const currentRole = ref<Role | null>(null)
/** 当前已勾选的权限 ID 集合 */
const selectedPermissionIds = ref<number[]>([])

// ============================================================================
// computed: 过滤、状态派生
// ============================================================================

/** 根据 searchQuery 在本地角色列表中做模糊搜索（匹配 name/code/description） */
const filteredRoles = computed(() => {
  if (!searchQuery.value) return roles.value
  const query = searchQuery.value.toLowerCase()
  return roles.value.filter((r: Role) =>
    r.name.toLowerCase().includes(query) || 
    r.code?.toLowerCase().includes(query) ||
    r.description?.toLowerCase().includes(query)
  )
})

/** 创建/编辑角色的表单数据（同一个 ref 同时用于创建和编辑对话框） */
const roleForm = ref({
  id: 0,
  name: '',
  code: '',
  description: '',
})

/** 角色表单校验规则：name 必填，code 必须为小写字母/数字/下划线 */
const roleRules = {
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
  code: [
    { required: true, message: '请输入角色代码', trigger: 'blur' },
    { pattern: /^[a-z0-9_]+$/, message: '角色代码只能包含小写字母、数字和下划线', trigger: 'blur' },
  ],
}

/** 创建角色对话框的 el-form 引用（用于表单校验和重置） */
const formRef = ref()
/** 编辑角色对话框的 el-form 引用 */
const editFormRef = ref()

// ============================================================================
// 角色 CRUD 函数（loadRoles / handleCreate / handleEdit / handleUpdate / handleDelete）
// ============================================================================

/**
 * 从后端加载全量角色列表，填充 roles ref
 * @description 页面初始化和 CRUD 操作完成后都会调用此函数刷新表格
 */
const loadRoles = async () => {
  loading.value = true
  try {
    const response = await roleAdminAPI.list()
    roles.value = response.data.items || []
  } catch (error) {
    if (!isHandledError(error)) {
      ElMessage.error('加载角色列表失败')
    }
  } finally {
    loading.value = false
  }
}

/**
 * 从后端加载全量权限列表及其按 resource 分组的展示结构
 * @description 用于权限分配对话框中的权限选项展示
 */
const loadPermissions = async () => {
  try {
    const response = await permissionAPI.getAllPermissions()
    permissions.value = response.data.items || []
    permissionGroupsDisplay.value = response.data.groups || []
  } catch (error) {
    // 权限加载失败时静默忽略：对话框打开时会单独加载该角色的权限
  }
}

/**
 * 提交创建角色表单
 * @description 表单校验通过后调用 roleAdminAPI.create，成功后刷新列表并重置表单
 */
const handleCreate = async () => {
  const form = formRef.value
  if (!form) return

  await form.validate(async (valid: boolean) => {
    if (valid) {
      submitting.value = true
      try {
        await roleAdminAPI.create({
          name: roleForm.value.name,
          code: roleForm.value.code,
          description: roleForm.value.description,
        })
        ElMessage.success('创建角色成功')
        showCreateDialog.value = false
        form.resetFields()
        loadRoles()
      } catch (error) {
        if (!isHandledError(error)) {
          ElMessage.error('创建角色失败')
        }
      } finally {
        submitting.value = false
      }
    }
  })
}

/**
 * 打开编辑角色对话框
 * @param row - 当前行的角色对象，用于预填充 roleForm
 */
const handleEdit = (row: Role) => {
  roleForm.value = {
    id: row.id,
    name: row.name,
    code: row.code,
    description: row.description,
  }
  showEditDialog.value = true
}

/**
 * 提交更新角色表单（编辑对话框）
 * @description 仅允许修改 name 和 description，code 不可变更
 */
const handleUpdate = async () => {
  const form = editFormRef.value
  if (!form) return

  await form.validate(async (valid: boolean) => {
    if (valid) {
      submitting.value = true
      try {
        await roleAdminAPI.update(roleForm.value.id, {
          name: roleForm.value.name,
          description: roleForm.value.description,
        })
        ElMessage.success('更新角色成功')
        showEditDialog.value = false
        loadRoles()
      } catch (error) {
        if (!isHandledError(error)) {
          ElMessage.error('更新角色失败')
        }
      } finally {
        submitting.value = false
      }
    }
  })
}

/**
 * 删除角色（需二次确认）
 * @param row - 待删除的角色对象，系统内置角色（is_system=true）在 UI 上已隐藏删除按钮
 */
const handleDelete = async (row: Role) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除角色 "${row.name}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    await roleAdminAPI.delete(row.id)
    ElMessage.success('删除角色成功')
    loadRoles()
  } catch (error: unknown) {
    if (error !== 'cancel' && !isHandledError(error)) {
      ElMessage.error('删除角色失败')
    }
  }
}

// ============================================================================
// 权限分配相关函数（handleAssignPermissions / getResourceIcon / isGroup* / handleGroup* / handleSavePermissions）
// ============================================================================

/**
 * 打开权限分配对话框，加载当前角色已有的权限 ID 列表
 * @param row - 正在分配权限的角色对象
 */
const handleAssignPermissions = async (row: Role) => {
  currentRole.value = row
  loadingPermissions.value = true
  showPermissionDialog.value = true

  try {
    const response = await roleAdminAPI.getPermissions(row.id)
    selectedPermissionIds.value = response.data.items?.map((p: Permission) => p.id) || []
  } catch (error) {
    selectedPermissionIds.value = []
  } finally {
    loadingPermissions.value = false
  }
}

/**
 * 根据权限资源标识符返回对应的 Element Plus 图标组件
 * @param resource - 权限资源标识符，如 'task', 'datasource', 'user' 等
 * @returns 对应的图标组件，未匹配时返回 Document 作为默认
 */
const getResourceIcon = (resource: string) => {
  const iconMap: Record<string, any> = {
    menu: Monitor,
    user: User,
    role: Key,
    domain: Grid,
    task: Timer,
    executor: Setting,
    log: List,
    dashboard: DataAnalysis,
    permission: Lock,
    datasource: Connection,
    audit_log: Document,
    config: Setting,
  }
  return iconMap[resource] || Document
}

/**
 * 判断某个权限分组是否已全部选中（用于分组头部的全选 checkbox 状态）
 * @param group - 权限分组对象，包含该 resource 下的所有权限
 * @returns 所有权限 ID 都在 selectedPermissionIds 中时返回 true
 */
const isGroupAllSelected = (group: PermissionGroup) => {
  const allIds = group.permissions.map((p: Permission) => p.id)
  return allIds.length > 0 && allIds.every((id: number) => selectedPermissionIds.value.includes(id))
}

/**
 * 判断某个权限分组是否处于半选状态（部分选中但非全部）
 * @param group - 权限分组对象
 * @returns 选中数量 > 0 且 < 总数时返回 true
 */
const isGroupIndeterminate = (group: PermissionGroup) => {
  const allIds = group.permissions.map((p: Permission) => p.id)
  const selectedCount = allIds.filter((id: number) => selectedPermissionIds.value.includes(id)).length
  return selectedCount > 0 && selectedCount < allIds.length
}

/**
 * 切换某个权限分组的全选/取消全选
 * @param group - 权限分组对象
 * @param checked - true 表示全选，false 表示取消全选
 */
const handleGroupSelectAll = (group: PermissionGroup, checked: boolean) => {
  const allIds = group.permissions.map((p: Permission) => p.id)
  if (checked) {
    allIds.forEach((id: number) => {
      if (!selectedPermissionIds.value.includes(id)) {
        selectedPermissionIds.value.push(id)
      }
    })
  } else {
    selectedPermissionIds.value = selectedPermissionIds.value.filter((id: number) => !allIds.includes(id))
  }
}

/**
 * 保存当前角色的权限分配结果
 * @description 将 selectedPermissionIds 通过 roleAdminAPI.assignPermissions 提交给后端
 */
const handleSavePermissions = async () => {
  if (!currentRole.value) return

  submitting.value = true
  try {
    await roleAdminAPI.assignPermissions(currentRole.value.id, {
      permission_ids: selectedPermissionIds.value,
    })
    ElMessage.success('分配权限成功')
    showPermissionDialog.value = false
  } catch (error) {
    if (!isHandledError(error)) {
      ElMessage.error('分配权限失败')
    }
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  // 初始化页面：同时加载角色列表和全量权限（用于权限分配对话框）
  loadRoles()
  loadPermissions()
})
</script>

<style scoped>
/* ========== 页面整体容器 ========== */
.roles-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  padding-bottom: var(--space-6);
  height: 100%;
}

/* 自定义滚动条样式 */
.roles-page::-webkit-scrollbar {
  width: 8px;
}

.roles-page::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 4px;
}

.roles-page::-webkit-scrollbar-track {
  background: var(--bg-secondary);
}

/* ========== 工具栏（搜索 + 刷新 + 创建） ========== */
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

/* 搜索框样式：聚焦时蓝色边框 + 阴影 */
.search-input {
  width: 280px;
}

.search-input :deep(.el-input__wrapper) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  box-shadow: none;
  transition: all var(--duration-normal) var(--ease-out);
}

.search-input :deep(.el-input__wrapper:hover) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.1);
}

.search-input :deep(.el-input__wrapper.is-focus) {
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

/* 刷新按钮：次样式，hover 时渐变高亮 */
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

/* 创建按钮：主渐变 + 投影，hover 上浮效果 */
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

/* ========== 角色列表表格 ========== */
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

/* Element Plus 表格全局样式覆盖 */
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

/* 表格空状态占位 */
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

/* ========== 对话框通用样式 ========== */
.dialog-form {
  padding: var(--space-2) 0;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-3);
}

/* ========== 权限分配对话框 ========== */
.permission-dialog-content {
  max-height: 60vh;
  overflow-y: auto;
}

/* 权限对话框内自定义滚动条 */
.permission-dialog-content::-webkit-scrollbar {
  width: 6px;
}

.permission-dialog-content::-webkit-scrollbar-thumb {
  background: var(--border-default);
  border-radius: 3px;
}

.permission-dialog-content::-webkit-scrollbar-track {
  background: var(--bg-secondary);
  border-radius: 3px;
}

/* 系统角色提示警告条 */
.system-role-alert {
  margin-bottom: var(--space-4);
  border-radius: var(--radius-md);
}

/* 权限分组卡片列表容器 */
.permission-groups {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

/* 系统角色时整个权限区域半透明禁用 */
.permission-groups.is-disabled {
  opacity: 0.6;
  pointer-events: none;
}

/* 单个权限分组卡片 */
.permission-group {
  background: var(--bg-secondary);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all var(--duration-normal) var(--ease-out);
}

.permission-group:hover {
  border-color: var(--accent-primary);
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.08);
}

/* 分组头部：图标 + 名称 + 数量 + 全选 checkbox */
.group-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.04), rgba(99, 102, 241, 0.04));
  border-bottom: 1px solid var(--border-default);
}

.group-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
  border-radius: var(--radius-md);
  color: white;
  font-size: 18px;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.25);
}

.group-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.group-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.group-count {
  font-size: 12px;
  color: var(--text-muted);
}

.group-checkbox {
  margin-left: auto;
}

/* 权限项列表：flex wrap 布局，每项带圆角边框 */
.permission-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
}

/* 单个权限项 checkbox 容器 */
.permission-item {
  margin: 0 !important;
  padding: 6px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  transition: all var(--duration-fast) var(--ease-out);
}

.permission-item:hover {
  border-color: var(--accent-primary);
  background: rgba(59, 130, 246, 0.04);
}

.permission-item.is-checked {
  border-color: var(--accent-primary);
  background: rgba(59, 130, 246, 0.08);
}

/* 权限项内部：名称 + resource:action 代码标签 */
.permission-content {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.permission-name {
  font-size: 13px;
  color: var(--text-primary);
}

.permission-code {
  font-size: 11px;
  font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
}
</style>
