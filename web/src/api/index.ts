import api from '@/utils/api'
import type { Task, Workflow, TaskExecution, TaskExecutionListResponse, Executor, LoginRequest, LoginResponse, WorkflowExecution, TaskLog, DashboardStats, TrendData } from '@/types'
import { userAdminAPI, roleAdminAPI, domainAdminAPI, executorDomainAPI, permissionAPI } from './admin'
import type { User, Role, Domain, Permission } from './admin'

interface TaskListResponse {
  items: Task[]
}

interface UpdateProfileRequest {
  email: string
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
  getCurrentUser: () => api.get('/auth/current'),
  updateProfile: (data: UpdateProfileRequest) => api.put('/auth/profile', data),
  changePassword: (data: ChangePasswordRequest) => api.post('/auth/change-password', data),
}

export const adminAPI = {
  resetUserPassword: (userId: number, data: ResetPasswordRequest) => 
    api.post(`/admin/users/${userId}/reset-password`, data),
}

export const taskAPI = {
  list: () => api.get<TaskListResponse>('/tasks'),
  get: (id: number) => api.get<Task>(`/tasks/${id}`),
  create: (data: Partial<Task>) => api.post<Task>('/tasks', data),
  update: (id: number, data: Partial<Task>) => api.put(`/tasks/${id}`, data),
  delete: (id: number) => api.delete(`/tasks/${id}`),
  trigger: (id: number) => api.post(`/tasks/${id}/trigger`),
  getExecutions: (id: number) => api.get<TaskExecution[]>(`/tasks/${id}/executions`),
  getExecutionLogs: (executionId: string) => api.get<TaskLog[]>(`/tasks/executions/${executionId}/logs`),
}

export const workflowAPI = {
  list: () => api.get<Workflow[]>('/workflows'),
  get: (id: number) => api.get<Workflow>(`/workflows/${id}`),
  create: (data: Partial<Workflow>) => api.post<Workflow>('/workflows', data),
  update: (id: number, data: Partial<Workflow>) => api.put(`/workflows/${id}`, data),
  delete: (id: number) => api.delete(`/workflows/${id}`),
  // 工作流执行相关 API
  trigger: (id: number) => api.post<WorkflowExecution>(`/workflows/${id}/trigger`),
  getExecutions: (id: number) => api.get<WorkflowExecution[]>(`/workflows/${id}/executions`),
  getExecution: (executionId: string) => api.get<WorkflowExecution>(`/workflows/executions/${executionId}`),
  getExecutionLogs: (executionId: string) => api.get<TaskLog[]>(`/workflows/executions/${executionId}/logs`),
}

export const executorAPI = {
	list: () => api.get<Executor[]>("/executors"),
	get: (id: string) => api.get<Executor>(`/executors/${id}`),
	delete: (executorId: string) => api.delete(`/executors/${executorId}`),
	online: (executorId: string) => api.post(`/executors/${executorId}/online`),
	offline: (executorId: string) => api.post(`/executors/${executorId}/offline`),
	updateCapacity: (executorId: string, capacity: number) =>
		api.put(`/executors/${executorId}/capacity`, { capacity }),
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
}
