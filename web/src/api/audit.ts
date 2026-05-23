import api from '@/utils/api'

export interface AuditLog {
  id: number
  user_id?: number
  username: string
  real_name?: string
  role?: string
  domain_id?: number
  action: string
  resource: string
  resource_id?: string
  resource_name?: string
  status: string
  ip_address?: string
  user_agent?: string
  request_method?: string
  request_path?: string
  detail?: string
  created_at: string
}

export interface AuditLogListResponse {
  items: AuditLog[]
  total: number
  page: number
  page_size: number
}

export interface AuditLogStats {
  total: number
}

export const auditLogAPI = {
  list: (params?: {
    username?: string
    action?: string
    resource?: string
    status?: string
    start_time?: string
    end_time?: string
    page?: number
    page_size?: number
  }) => api.get<AuditLogListResponse>('/admin/audit-logs', { params }),

  getStats: () => api.get<AuditLogStats>('/admin/audit-logs/stats'),

  cleanExpired: (retentionDays?: number) =>
    api.post('/admin/audit-logs/clean', { retention_days: retentionDays }),

  getRetention: () => api.get<{ retention_days: number }>('/admin/audit-logs/retention'),

  updateRetention: (days: number) =>
    api.put('/admin/audit-logs/retention', { retention_days: days }),
}
