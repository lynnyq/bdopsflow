/**
 * @file audit.ts - 审计日志 API 封装
 * 提供审计日志列表查询、统计、过期清理、保留天数配置等接口，
 * 供 api/index.ts 二次导出。
 */
import api from '@/utils/api'

/** 单条审计日志记录 */
export interface AuditLog {
  id: number                  // 日志 ID
  user_id?: number            // 操作者用户 ID
  username: string            // 操作者用户名
  real_name?: string          // 操作者真实姓名
  role?: string               // 操作者角色编码
  domain_id?: number          // 发生操作的领域 ID
  action: string              // 操作名称，如 'create', 'update', 'delete', 'login'
  resource: string            // 操作的资源类型
  resource_id?: string        // 资源 ID
  resource_name?: string      // 资源名称（冗余展示）
  status: string              // 操作结果：success / failed
  response_code?: number      // 后端返回的 HTTP 状态码
  ip_address?: string         // 操作者 IP
  user_agent?: string         // UA
  request_method?: string     // 请求方法
  request_path?: string       // 请求路径
  detail?: string             // 操作详情（JSON 或文本）
  created_at: string          // 日志时间
}

/** 审计日志分页响应 */
export interface AuditLogListResponse {
  items: AuditLog[]   // 数据列表
  total: number       // 总条数
  page: number        // 当前页码
  page_size: number   // 每页大小
}

/** 审计日志统计结果 */
export interface AuditLogStats {
  total: number       // 日志总条数
}

/** 审计日志 API 封装对象 */
export const auditLogAPI = {
  /**
   * 分页查询审计日志，支持用户名/动作/资源/状态/领域/时间范围等多维过滤。
   * @param params - 过滤与分页参数
   */
  list: (params?: {
    username?: string
    action?: string
    resource?: string
    status?: string
    domain_id?: number
    start_time?: string
    end_time?: string
    page?: number
    page_size?: number
  }) => api.get<AuditLogListResponse>('/admin/audit-logs', { params }),

  /** 获取审计日志总数（用于仪表盘统计） */
  getStats: () => api.get<AuditLogStats>('/admin/audit-logs/stats'),

  /**
   * 清理过期的审计日志。默认使用系统配置的保留天数，也可通过参数覆盖。
   * @param retentionDays - 要保留的天数，不传时使用系统设置
   */
  cleanExpired: (retentionDays?: number) =>
    api.post('/admin/audit-logs/clean', { retention_days: retentionDays }),

  /** 获取当前审计日志保留天数配置 */
  getRetention: () => api.get<{ retention_days: number }>('/admin/audit-logs/retention'),

  /**
   * 更新审计日志保留天数配置。
   * @param days - 新的保留天数
   */
  updateRetention: (days: number) =>
    api.put('/admin/audit-logs/retention', { retention_days: days }),
}
