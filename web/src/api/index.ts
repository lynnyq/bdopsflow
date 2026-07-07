import api from '@/utils/api'
import type { Task, TaskExecution, TaskExecutionListResponse, Executor, ExecutorWithDomains, Domain, LoginRequest, LoginResponse, TaskLog, DashboardStats, TrendData, PaginatedResponse, User, Role, Permission, CurrentUserResponse } from '@/types'
import { userAdminAPI, roleAdminAPI, domainAdminAPI, permissionAPI, switchDomain } from './admin'
import { datasourceAPI, queryAPI } from './datasource'
import { systemConfigAPI } from './systemConfig'
import { auditLogAPI } from './audit'
import { webhookAPI } from './webhook'
import { apiTestAPI } from './apiTest'
import type { AuditLog, AuditLogListResponse, AuditLogStats } from './audit'

export { userAdminAPI, roleAdminAPI, domainAdminAPI, permissionAPI, switchDomain, datasourceAPI, queryAPI, systemConfigAPI, auditLogAPI, webhookAPI, apiTestAPI }
export type { User, Role, Permission, AuditLog, AuditLogListResponse, AuditLogStats }

interface UpdateProfileRequest {
  email: string
  real_name: string
  phone: string
}

interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

interface ResetPasswordRequest {
  new_password: string
}

export const authAPI = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
  ssoLogin: (data: LoginRequest) => api.post<LoginResponse>('/auth/sso-login', data),
  getCurrentUser: () => api.get<CurrentUserResponse>('/auth/current'),
  updateProfile: (data: UpdateProfileRequest) => api.put('/auth/profile', data),
  changePassword: (data: ChangePasswordRequest) => api.post('/auth/change-password', data),
  getPublicKey: () => api.get<{ public_key: string; sso_public_key?: string; sso_enabled: boolean }>('/auth/public-key'),
  refreshToken: (refreshToken: string) => api.post<{ token: string; refresh_token: string }>('/auth/refresh', { refresh_token: refreshToken }),
  apiToken: {
    generate: () => api.post<{ token: string; token_prefix: string; created_at: string }>('/auth/api-token'),
    getInfo: () => api.get<{ has_token: boolean; token_prefix: string; last_used_at: string; created_at: string }>('/auth/api-token'),
    reveal: () => api.get<{ token: string }>('/auth/api-token/reveal'),
    revoke: () => api.delete('/auth/api-token'),
  },
}

export const adminAPI = {
  resetUserPassword: (userId: number, data: ResetPasswordRequest) =>
    api.post(`/admin/users/${userId}/reset-password`, data),
}

export const taskAPI = {
  list: (params?: { page?: number; page_size?: number; search?: string; type?: string; is_enabled?: boolean; created_by?: number }) => api.get<PaginatedResponse<Task>>('/tasks', { params }),
  get: (id: number) => api.get<Task>(`/tasks/${id}`),
  create: (data: Partial<Task>) => api.post<Task>('/tasks', data),
  update: (id: number, data: Partial<Task>) => api.put(`/tasks/${id}`, data),
  delete: (id: number) => api.delete(`/tasks/${id}`),
  trigger: (id: number) => api.post(`/tasks/${id}/trigger`),
  getExecutions: (id: number) => api.get<TaskExecution[]>(`/tasks/${id}/executions`),
  getExecutionLogs: (executionId: string) => api.get<TaskLog[]>(`/tasks/executions/${executionId}/logs`),
  cancelExecution: (executionId: string) => api.post(`/tasks/executions/${executionId}/cancel`),
}

export const executorAPI = {
	list: (params?: { page?: number; page_size?: number }) => api.get<PaginatedResponse<ExecutorWithDomains>>("/executors", { params }),
	get: (name: string) => api.get<Executor>(`/executors/${name}`),
	delete: (name: string) => api.delete(`/executors/${name}`),
	online: (name: string) => api.post(`/executors/${name}/online`),
	offline: (name: string) => api.post(`/executors/${name}/offline`),
	updateCapacity: (name: string, capacity: number) =>
		api.put(`/executors/${name}/capacity`, { capacity }),
	getExecutorDomains: (name: string) => api.get<{ items: Domain[] }>(`/executors/${name}/domains`),
	assignDomains: (name: string, domainIds: number[]) => api.post(`/executors/${name}/domains`, { domain_ids: domainIds }),
	removeDomain: (name: string, domainId: number) => api.delete(`/executors/${name}/domains/${domainId}`),
	getAssignedTasks: (name: string) => api.get<{ task_count: number; task_names: string[] }>(`/executors/${name}/tasks`),
	canDelete: (name: string) => api.get<{ can_delete: boolean; reason?: string; has_tasks: boolean; task_count: number }>(`/executors/${name}/can-delete`),
}

export const logAPI = {
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
	delete: (id: number) => api.delete(`/logs/${id}`),
	batchDelete: (ids: number[]) => api.post('/logs/batch-delete', { ids }),
}

export const dashboardAPI = {
  getStats: () => api.get<DashboardStats>('/dashboard/stats'),
  getTrends: () => api.get<{ items: TrendData[] }>('/dashboard/trends'),
  getSchedulerStatus: () => api.get<{ paused: boolean }>('/dashboard/scheduler/status'),
  pauseScheduler: () => api.post('/dashboard/scheduler/pause'),
  resumeScheduler: () => api.post('/dashboard/scheduler/resume'),
  getHealth: () => api.get<HealthCheckResult>('/dashboard/health'),
}

export interface ComponentCheck {
  status: string
  message: string
  latency?: string
}

export interface HealthCheckResult {
  status: string
  timestamp: string
  components: {
    [key: string]: ComponentCheck
  }
}
