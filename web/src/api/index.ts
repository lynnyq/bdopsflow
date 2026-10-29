/**
 * @file index.ts - 前端 API 统一出口
 * 聚合 auth/task/executor/log/dashboard 等核心业务的 API 封装对象，
 * 并从 ./admin / ./audit / ./datasource / ./webhook / ./apiTest / ./grafana / ./systemConfig
 * 子模块二次导出，方便业务模块统一从 '@/api' 引用。
 */
import api from '@/utils/api'
import type { Task, TaskExecution, TaskExecutionListResponse, Executor, ExecutorWithDomains, Domain, LoginRequest, LoginResponse, TaskLog, DashboardStats, TrendData, PaginatedResponse, User, Role, Permission, CurrentUserResponse } from '@/types'
import { userAdminAPI, roleAdminAPI, domainAdminAPI, permissionAPI, switchDomain } from './admin'
import { datasourceAPI, queryAPI } from './datasource'
import { systemConfigAPI } from './systemConfig'
import { auditLogAPI } from './audit'
import { webhookAPI } from './webhook'
import { apiTestAPI } from './apiTest'
import { grafanaAPI } from './grafana'
import type { AuditLog, AuditLogListResponse, AuditLogStats } from './audit'

// 从子模块统一导出所有 API 对象和类型
export { userAdminAPI, roleAdminAPI, domainAdminAPI, permissionAPI, switchDomain, datasourceAPI, queryAPI, systemConfigAPI, auditLogAPI, webhookAPI, apiTestAPI, grafanaAPI }
export type { User, Role, Permission, AuditLog, AuditLogListResponse, AuditLogStats }

/** 更新个人资料请求体 */
interface UpdateProfileRequest {
  email: string
  real_name: string
  phone: string
}

/** 修改密码请求体 */
interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

/** 管理员重置用户密码请求体 */
interface ResetPasswordRequest {
  new_password: string
}

/**
 * 认证相关 API 封装对象
 * 覆盖登录、SSO、当前用户、个人资料、密码修改、API Token 管理等接口。
 */
export const authAPI = {
  /**
   * 用户名密码登录。后端会校验密码并返回 Token、用户信息及权限集。
   * @param data - 登录请求体
   */
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
  /**
   * SSO 方式登录，使用后端下发的 SSO RSA 公钥加密密码。
   * @param data - 登录请求体
   */
  ssoLogin: (data: LoginRequest) => api.post<LoginResponse>('/auth/sso-login', data),
  /** 获取当前登录用户完整信息（包含权限、领域、角色） */
  getCurrentUser: () => api.get<CurrentUserResponse>('/auth/current'),
  /**
   * 更新当前用户个人资料（邮箱、姓名、手机号）。
   * @param data - 新的资料字段
   */
  updateProfile: (data: UpdateProfileRequest) => api.put('/auth/profile', data),
  /**
   * 修改当前用户密码。
   * @param data - 旧密码 + 新密码
   */
  changePassword: (data: ChangePasswordRequest) => api.post('/auth/change-password', data),
  /** 获取后端下发的 RSA 公钥及 SSO 开关配置 */
  getPublicKey: () => api.get<{ public_key: string; sso_public_key?: string; sso_enabled: boolean }>('/auth/public-key'),
  /**
   * 使用 refresh_token 换取新的 token 对。
   * @param refreshToken - 刷新令牌
   */
  refreshToken: (refreshToken: string) => api.post<{ token: string; refresh_token: string }>('/auth/refresh', { refresh_token: refreshToken }),
  /** API Token 管理子对象，服务于外部系统对接 */
  apiToken: {
    /** 生成一个新的长期 API Token */
    generate: () => api.post<{ token: string; token_prefix: string; created_at: string }>('/auth/api-token'),
    /** 获取当前 API Token 的元信息（是否存在、前缀、创建/最后使用时间） */
    getInfo: () => api.get<{ has_token: boolean; token_prefix: string; last_used_at: string; created_at: string }>('/auth/api-token'),
    /** 显示 API Token 完整明文（仅展示一次） */
    reveal: () => api.get<{ token: string }>('/auth/api-token/reveal'),
    /** 吊销当前 API Token，使其失效 */
    revoke: () => api.delete('/auth/api-token'),
  },
}

/** 管理员侧密码重置 API */
export const adminAPI = {
  /**
   * 管理员重置指定用户的密码。
   * @param userId - 目标用户 ID
   * @param data - 新密码
   */
  resetUserPassword: (userId: number, data: ResetPasswordRequest) =>
    api.post(`/admin/users/${userId}/reset-password`, data),
}

/** 调度任务 CRUD 及执行相关 API */
export const taskAPI = {
  /**
   * 分页查询任务列表，支持关键字搜索、类型/启用状态过滤。
   * @param params - 查询参数
   */
  list: (params?: { page?: number; page_size?: number; search?: string; type?: string; is_enabled?: boolean; created_by?: number }) => api.get<PaginatedResponse<Task>>('/tasks', { params }),
  /**
   * 获取单个任务详情。
   * @param id - 任务 ID
   */
  get: (id: number) => api.get<Task>(`/tasks/${id}`),
  /**
   * 创建新的调度任务。
   * @param data - 任务字段
   */
  create: (data: Partial<Task>) => api.post<Task>('/tasks', data),
  /**
   * 更新已有任务。
   * @param id - 任务 ID
   * @param data - 需要更新的字段
   */
  update: (id: number, data: Partial<Task>) => api.put(`/tasks/${id}`, data),
  /**
   * 删除任务。
   * @param id - 任务 ID
   */
  delete: (id: number) => api.delete(`/tasks/${id}`),
  /**
   * 立即手动触发一次任务执行。
   * @param id - 任务 ID
   */
  trigger: (id: number) => api.post(`/tasks/${id}/trigger`),
  /**
   * 获取任务的所有历史执行记录。
   * @param id - 任务 ID
   */
  getExecutions: (id: number) => api.get<TaskExecution[]>(`/tasks/${id}/executions`),
  /**
   * 获取某次执行的完整日志列表。
   * @param executionId - 执行唯一 ID
   */
  getExecutionLogs: (executionId: string) => api.get<TaskLog[]>(`/tasks/executions/${executionId}/logs`),
  /**
   * 取消正在运行的任务执行。
   * @param executionId - 执行唯一 ID
   */
  cancelExecution: (executionId: string) => api.post(`/tasks/executions/${executionId}/cancel`),
}

/** 执行器管理 API */
export const executorAPI = {
  /**
   * 分页查询执行器列表。
   * @param params - 查询参数
   */
	list: (params?: { page?: number; page_size?: number }) => api.get<PaginatedResponse<ExecutorWithDomains>>("/executors", { params }),
  /**
   * 获取执行器详情。
   * @param name - 执行器注册名
   */
	get: (name: string) => api.get<Executor>(`/executors/${name}`),
  /**
   * 删除执行器注册。
   * @param name - 执行器注册名
   */
	delete: (name: string) => api.delete(`/executors/${name}`),
  /**
   * 将执行器标记为 online（恢复调度）。
   * @param name - 执行器注册名
   */
	online: (name: string) => api.post(`/executors/${name}/online`),
  /**
   * 将执行器标记为 offline（暂停调度）。
   * @param name - 执行器注册名
   */
	offline: (name: string) => api.post(`/executors/${name}/offline`),
  /**
   * 更新执行器并发容量上限。
   * @param name - 执行器注册名
   * @param capacity - 新的并发上限
   */
	updateCapacity: (name: string, capacity: number) =>
		api.put(`/executors/${name}/capacity`, { capacity }),
  /**
   * 查询执行器被分配到的领域列表。
   * @param name - 执行器注册名
   */
	getExecutorDomains: (name: string) => api.get<{ items: Domain[] }>(`/executors/${name}/domains`),
  /**
   * 为执行器批量分配领域。
   * @param name - 执行器注册名
   * @param domainIds - 领域 ID 数组
   */
	assignDomains: (name: string, domainIds: number[]) => api.post(`/executors/${name}/domains`, { domain_ids: domainIds }),
  /**
   * 移除执行器上的某个领域分配。
   * @param name - 执行器注册名
   * @param domainId - 要移除的领域 ID
   */
	removeDomain: (name: string, domainId: number) => api.delete(`/executors/${name}/domains/${domainId}`),
  /**
   * 查询执行器当前绑定的任务概要。
   * @param name - 执行器注册名
   */
	getAssignedTasks: (name: string) => api.get<{ task_count: number; task_names: string[] }>(`/executors/${name}/tasks`),
  /**
   * 检查执行器是否可以安全删除（无运行中任务等）。
   * @param name - 执行器注册名
   */
	canDelete: (name: string) => api.get<{ can_delete: boolean; reason?: string; has_tasks: boolean; task_count: number }>(`/executors/${name}/can-delete`),
}

/** 任务日志查询与管理 API */
export const logAPI = {
  /**
   * 分页查询任务执行日志，支持多维过滤。
   * @param params - 执行 ID、执行器名、时间范围、状态等过滤条件
   */
  list: (params?: {
    id?: string,
    execution_id?: string,
    executor_name?: string,
    task_name?: string,
    status?: string,
    start_time_from?: string,
    start_time_to?: string,
    end_time_from?: string,
    end_time_to?: string,
    duration_min?: number,
    duration_max?: number,
    page?: number,
    page_size?: number
  }) => api.get<PaginatedResponse<TaskExecutionListResponse>>('/logs', { params }),
  /**
   * 查询日志列表的聚合统计（用于筛选面板快速展示）。
   * @param params - 同 list 的过滤条件
   */
  getStats: (params?: {
    id?: string,
    execution_id?: string,
    executor_name?: string,
    task_name?: string,
    status?: string,
    start_time_from?: string,
    start_time_to?: string,
    end_time_from?: string,
    end_time_to?: string,
    duration_min?: number,
    duration_max?: number
  }) => api.get<{ [key: string]: number }>('/logs/stats', { params }),
  /**
   * 删除单条日志。
   * @param id - 日志 ID
   */
	delete: (id: number) => api.delete(`/logs/${id}`),
  /**
   * 批量删除日志。
   * @param ids - 日志 ID 数组
   */
	batchDelete: (ids: number[]) => api.post('/logs/batch-delete', { ids }),
}

/** 仪表盘统计、趋势、调度器状态 API */
export const dashboardAPI = {
  /** 获取仪表盘概览统计：任务、执行器、调度器状态 */
  getStats: () => api.get<DashboardStats>('/dashboard/stats'),
  /** 获取最近一段时间内按日期聚合的任务执行趋势 */
  getTrends: () => api.get<{ items: TrendData[] }>('/dashboard/trends'),
  /** 查询调度器当前是否暂停 */
  getSchedulerStatus: () => api.get<{ paused: boolean }>('/dashboard/scheduler/status'),
  /** 暂停调度器（不再触发新的 cron 任务） */
  pauseScheduler: () => api.post('/dashboard/scheduler/pause'),
  /** 恢复调度器运行 */
  resumeScheduler: () => api.post('/dashboard/scheduler/resume'),
  /** 获取健康检查详情：各依赖组件（DB/Redis 等）的状态与耗时 */
  getHealth: () => api.get<HealthCheckResult>('/dashboard/health'),
}

/** 健康检查中单个组件的状态 */
export interface ComponentCheck {
  status: string       // up / down / degraded
  message: string      // 状态描述
  latency?: string     // 最近一次探测耗时
}

/** 全局健康检查响应 */
export interface HealthCheckResult {
  status: string                           // 整体状态
  timestamp: string                        // 探测时间
  components: {
    [key: string]: ComponentCheck          // 各组件状态，key 为组件名称
  }
}
