import api from '@/utils/api'

export interface Permission {
  resource: string
  action: string
  description: string
}

export interface Role {
  id: number
  name: string
  code: string
  description: string
  is_system: boolean
  domain_id: number | null
  created_at: string
  updated_at: string
}

export interface Domain {
  id: number
  name: string
  description: string
  created_at: string
  updated_at: string
  user_count?: number
  executor_count?: number
  task_count?: number
}

export interface DomainExecutor {
  executor_id: number
  domain_id: number
  domain_name: string
  assigned_at: string
}

export const permissionAPI = {
  getAllPermissions: () => api.get<{ items: Permission[] }>('/admin/permissions'),
}

export const userAdminAPI = {
  list: () => api.get<{ items: User[] }>('/admin/users'),
  get: (id: number) => api.get<{ user: User; roles: Role[] }>(`/admin/users/${id}`),
  create: (data: CreateUserRequest) => api.post<User>('/admin/users', data),
  update: (id: number, data: UpdateUserRequest) => api.put<User>(`/admin/users/${id}`, data),
  delete: (id: number) => api.delete(`/admin/users/${id}`),
  assignRoles: (id: number, data: AssignRolesRequest) => api.post(`/admin/users/${id}/roles`, data),
  getRoles: (id: number) => api.get<{ items: Role[] }>(`/admin/users/${id}/roles`),
  assignDomains: (id: number, data: AssignDomainsRequest) => api.post(`/admin/users/${id}/domains`, data),
}

export const roleAdminAPI = {
  list: () => api.get<{ items: Role[] }>('/admin/roles'),
  get: (id: number) => api.get<Role>(`/admin/roles/${id}`),
  create: (data: CreateRoleRequest) => api.post<Role>('/admin/roles', data),
  update: (id: number, data: UpdateRoleRequest) => api.put<Role>(`/admin/roles/${id}`, data),
  delete: (id: number) => api.delete(`/admin/roles/${id}`),
  getPermissions: (id: number) => api.get<{ items: Permission[] }>(`/admin/roles/${id}/permissions`),
  assignPermissions: (id: number, data: AssignPermissionsRequest) => api.post(`/admin/roles/${id}/permissions`, data),
}

export const domainAdminAPI = {
  list: () => api.get<{ items: Domain[] }>('/admin/domains'),
  get: (id: number) => api.get<Domain>(`/admin/domains/${id}`),
  create: (data: CreateDomainRequest) => api.post<Domain>('/admin/domains', data),
  update: (id: number, data: UpdateDomainRequest) => api.put<Domain>(`/admin/domains/${id}`, data),
  delete: (id: number) => api.delete(`/admin/domains/${id}`),
}

export const executorDomainAPI = {
  getDomains: (executorId: number) => api.get<{ items: Domain[] }>(`/admin/executors/${executorId}/domains`),
  assignDomains: (executorId: number, data: AssignExecutorDomainsRequest) => api.post(`/admin/executors/${executorId}/domains`, data),
  removeDomain: (executorId: number, domainId: number) => api.delete(`/admin/executors/${executorId}/domains/${domainId}`),
}

export interface User {
  id: number
  username: string
  email: string
  role: string
  domain_id: number | null
  is_active: boolean
  last_login_at: string | null
  created_by: number
  created_at: string
  updated_at: string
}

export interface CreateUserRequest {
  username: string
  email: string
  password: string
}

export interface UpdateUserRequest {
  username: string
  email: string
  role: string
  is_active: boolean
}

export interface AssignRolesRequest {
  role_ids: number[]
  domain_ids: number[]
}

export interface AssignDomainsRequest {
  domain_ids: number[]
}

export interface CreateRoleRequest {
  name: string
  code: string
  description: string
  domain_id?: number | null
}

export interface UpdateRoleRequest {
  name: string
  description: string
}

export interface AssignPermissionsRequest {
  permission_ids: number[]
}

export interface CreateDomainRequest {
  name: string
  description: string
}

export interface UpdateDomainRequest {
  name: string
  description: string
}

export interface AssignExecutorDomainsRequest {
  domain_ids: number[]
}
