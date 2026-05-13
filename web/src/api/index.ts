import api from '@/utils/api'
import type { Task, Workflow, TaskExecution, TaskExecutionListResponse, Executor, LoginRequest, LoginResponse, WorkflowExecution, TaskLog } from '@/types'

interface TaskListResponse {
  items: Task[]
}

export const authAPI = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
  getCurrentUser: () => api.get('/auth/current'),
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
  list: () => api.get<Executor[]>('/executors'),
  get: (id: string) => api.get<Executor>(`/executors/${id}`),
}

export const logAPI = {
	list: (params?: {
		executor_name?: string,
		task_name?: string,
		task_type?: string,
		status?: string,
		page?: number,
		page_size?: number
	}) => api.get<PaginatedResponse<TaskExecutionListResponse>>('/logs', { params }),
	getStats: (params?: {
		executor_name?: string,
		task_name?: string,
		task_type?: string,
		status?: string
	}) => api.get<{ [key: string]: number }>('/logs/stats', { params }),
	delete: (id: number) => api.delete(`/logs/${id}`),
	batchDelete: (ids: number[]) => api.post('/logs/batch-delete', { ids }),
}
