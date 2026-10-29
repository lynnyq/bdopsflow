/**
 * @file admin.ts - 管理员后台 API 封装
 * 包含用户管理、角色管理、领域管理、权限查询、领域切换等管理员侧接口，
 * 供 api/index.ts 二次导出。
 */
import api from '@/utils/api'
import type { User, Role, Permission, PermissionGroup, Domain, DomainWithStats, SwitchDomainResponse } from '@/types'

// 重新导出类型方便外部直接从本模块引用
export type { User, Role, Permission, PermissionGroup, Domain, DomainWithStats }

/** 执行器-领域关联记录 */
export interface DomainExecutor {
  executor_id: number   // 执行器 ID
  domain_id: number     // 领域 ID
  domain_name: string   // 领域名称
  assigned_at: string    // 分配时间
}

/** 为用户分配角色的请求体 */
export interface AssignRolesRequest {
  role_ids: number[]     // 角色 ID 数组
}

/** 为用户分配领域的请求体 */
export interface AssignDomainsRequest {
  domain_ids: number[]   // 领域 ID 数组
}

/** 创建角色请求体 */
export interface CreateRoleRequest {
  name: string              // 角色名
  code: string              // 角色编码（唯一）
  description?: string      // 说明
  is_system?: boolean       // 是否系统内置
  domain_id?: number        // 所属领域 ID，null 表示全局
}

/** 更新角色请求体 */
export interface UpdateRoleRequest {
  name?: string
  description?: string
}

/** 为角色分配权限的请求体 */
export interface AssignPermissionsRequest {
  permission_ids: number[]  // 权限 ID 数组
}

/**
 * 切换当前登录用户的领域（租户）。后端会校验该领域是否有权限访问，
 * 并返回新领域下的 Token、权限集和角色编码。
 * @param domainId - 目标领域 ID
 */
export const switchDomain = (domainId: number) =>
  api.post<SwitchDomainResponse>('/auth/switch-domain', { domain_id: domainId })

/** 权限查询 API */
export const permissionAPI = {
  /**
   * 获取系统中全部权限列表，并按 resource 分组返回，
   * 用于角色权限分配界面构建复选框树。
   */
  getAllPermissions: () => api.get<{ items: Permission[]; groups: PermissionGroup[] }>('/admin/permissions'),
}

/** 用户管理 API */
export const userAdminAPI = {
  /** 获取所有用户列表 */
  list: () => api.get<{ items: User[] }>('/admin/users'),
  /**
   * 按领域查询用户列表。
   * @param domainId - 领域 ID，不传时返回全部
   */
  listByDomain: (domainId?: number) => api.get<{ items: User[] }>('/admin/users/by-domain', { params: domainId !== undefined ? { domain_id: domainId } : {} }),
  /**
   * 获取单个用户详情（含已分配角色）。
   * @param id - 用户 ID
   */
  get: (id: number) => api.get<{ user: User; roles: Role[] }>(`/admin/users/${id}`),
  /**
   * 创建新用户。
   * @param data - 用户基础信息、初始密码、角色 ID 和领域 ID
   */
  create: (data: {
    username: string
    real_name: string
    phone: string
    email: string
    password: string
    role_ids: number[]
    domain_ids: number[]
  }) => api.post('/admin/users', data),
  /**
   * 更新用户信息（用户名不可修改）。
   * @param id - 用户 ID
   * @param data - 更新字段
   */
  update: (id: number, data: {
    username: string
    real_name: string
    phone: string
    email: string
    is_active: boolean
    role_ids?: number[]
    domain_ids?: number[]
  }) => api.put(`/admin/users/${id}`, data),
  /**
   * 删除用户。
   * @param id - 用户 ID
   */
  delete: (id: number) => api.delete(`/admin/users/${id}`),
  /**
   * 为用户批量分配角色（会覆盖之前的角色集合）。
   * @param id - 用户 ID
   * @param data - 角色 ID 数组
   */
  assignRoles: (id: number, data: AssignRolesRequest) => api.post(`/admin/users/${id}/roles`, data),
  /**
   * 查询用户已分配的角色列表。
   * @param id - 用户 ID
   */
  getRoles: (id: number) => api.get<{ items: Role[] }>(`/admin/users/${id}/roles`),
  /**
   * 为用户分配可访问的领域（多租户场景下控制数据隔离范围）。
   * @param id - 用户 ID
   * @param data - 领域 ID 数组
   */
  assignDomains: (id: number, data: AssignDomainsRequest) => api.post(`/admin/users/${id}/domains`, data),
  /**
   * 管理员重置用户密码。
   * @param id - 用户 ID
   * @param data - 新密码
   */
  resetPassword: (id: number, data: { new_password: string }) => api.post(`/admin/users/${id}/reset-password`, data),
}

/** 角色管理 API */
export const roleAdminAPI = {
  /** 获取所有角色列表 */
  list: () => api.get<{ items: Role[] }>('/admin/roles'),
  /**
   * 获取单个角色详情。
   * @param id - 角色 ID
   */
  get: (id: number) => api.get<Role>(`/admin/roles/${id}`),
  /**
   * 创建新角色。
   * @param data - 角色基础信息
   */
  create: (data: CreateRoleRequest) => api.post<Role>('/admin/roles', data),
  /**
   * 更新角色信息（code 字段不可修改）。
   * @param id - 角色 ID
   * @param data - 更新字段
   */
  update: (id: number, data: UpdateRoleRequest) => api.put<Role>(`/admin/roles/${id}`, data),
  /**
   * 删除角色（系统内置角色不可删除，后端会拒绝）。
   * @param id - 角色 ID
   */
  delete: (id: number) => api.delete(`/admin/roles/${id}`),
  /**
   * 查询角色已分配的权限列表。
   * @param id - 角色 ID
   */
  getPermissions: (id: number) => api.get<{ items: Permission[] }>(`/admin/roles/${id}/permissions`),
  /**
   * 为角色批量分配权限（会覆盖之前的权限集合）。
   * @param id - 角色 ID
   * @param data - 权限 ID 数组
   */
  assignPermissions: (id: number, data: AssignPermissionsRequest) => api.post(`/admin/roles/${id}/permissions`, data),
}

/** 领域（租户）管理 API */
export const domainAdminAPI = {
  /** 获取所有领域列表（含用户数/执行器数/任务数等统计） */
  list: () => api.get<{ items: DomainWithStats[] }>('/admin/domains'),
  /**
   * 获取单个领域详情。
   * @param id - 领域 ID
   */
  get: (id: number) => api.get<Domain>(`/admin/domains/${id}`),
  /**
   * 创建新领域。
   * @param data - 领域基础信息
   */
  create: (data: Partial<Domain>) => api.post<Domain>('/admin/domains', data),
  /**
   * 更新领域信息。
   * @param id - 领域 ID
   * @param data - 更新字段
   */
  update: (id: number, data: Partial<Domain>) => api.put<Domain>(`/admin/domains/${id}`, data),
  /**
   * 删除领域（领域下存在用户/执行器/任务时后端会拒绝）。
   * @param id - 领域 ID
   */
  delete: (id: number) => api.delete(`/admin/domains/${id}`),
}

/** 执行器-领域关联查询（在执行器页面复用） */
export const executorDomainAPI = {
  /**
   * 查询某执行器被分配到的领域列表。
   * @param executorId - 执行器 ID
   */
  getDomains: (executorId: number) => api.get<{ items: Domain[] }>(`/executors/${executorId}/domains`),
}
