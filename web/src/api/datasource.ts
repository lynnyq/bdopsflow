/**
 * @file datasource.ts - 数据源管理与 SQL 查询 API 封装
 * 包含数据源 CRUD、连接测试、元数据浏览（库/表/列）、连接池状态、
 * SQL 执行/流式结果/取消/导出 CSV、查询历史、已保存 SQL 等接口，
 * 供 api/index.ts 二次导出。
 */
import api from '@/utils/api'
import type { Datasource, DatasourcePermission, QueryResult, QueryHistory, SavedSQL, TableInfo, ColumnInfo } from '@/types'
import type { AxiosProgressEvent } from 'axios'

/** 数据源管理 API */
export const datasourceAPI = {
  /**
   * 分页查询数据源列表，支持按领域、类型、关键字过滤。
   * @param params - 查询参数
   */
  list: (params?: { domain_id?: number; type?: string; search?: string; page?: number; page_size?: number }) =>
    api.get<{ items: Datasource[]; total: number; page: number; page_size: number }>('/datasources', { params }),
  /**
   * 获取单个数据源详情。
   * @param id - 数据源 ID
   */
  get: (id: number) =>
    api.get<Datasource>(`/datasources/${id}`),
  /**
   * 创建新数据源。
   * @param data - 数据源字段
   */
  create: (data: Partial<Datasource>) =>
    api.post('/datasources', data),
  /**
   * 更新数据源配置。
   * @param id - 数据源 ID
   * @param data - 更新字段
   */
  update: (id: number, data: Partial<Datasource>) =>
    api.put(`/datasources/${id}`, data),
  /**
   * 删除数据源。
   * @param id - 数据源 ID
   */
  delete: (id: number) =>
    api.delete(`/datasources/${id}`),
  /**
   * 使用已保存的数据源配置执行连接测试。
   * @param id - 数据源 ID
   */
  testConnection: (id: number) =>
    api.post(`/datasources/${id}/test`),
  /**
   * 使用前端填写但尚未保存的参数临时测试连接（新建页常用）。
   * @param data - 数据源字段
   */
  testConnectionByParams: (data: Partial<Datasource>) =>
    api.post('/datasources/test', data),
  /**
   * 获取后端支持的所有数据库类型列表。
   */
  supportedTypes: () =>
    api.get<string[]>('/datasources/types'),
  /**
   * 为数据源分配权限给角色或用户。
   * @param id - 数据源 ID
   * @param data - 指定角色或用户 + 权限类型
   */
  grantPermission: (id: number, data: { role_id?: number; user_id?: number; permission_type: string }) =>
    api.post(`/datasources/${id}/permissions`, data),
  /**
   * 更新已存在的权限记录（修改权限类型）。
   * @param datasourceId - 数据源 ID
   * @param permId - 权限记录 ID
   * @param data - 新的权限类型
   */
  updatePermission: (datasourceId: number, permId: number, data: { permission_type: string }) =>
    api.put(`/datasources/${datasourceId}/permissions/${permId}`, data),
  /**
   * 撤销数据源上的某条权限记录。
   * @param datasourceId - 数据源 ID
   * @param permId - 权限记录 ID
   */
  revokePermission: (datasourceId: number, permId: number) =>
    api.delete(`/datasources/${datasourceId}/permissions/${permId}`),
  /**
   * 获取数据源上所有已分配的权限记录。
   * @param id - 数据源 ID
   */
  getPermissions: (id: number) =>
    api.get<DatasourcePermission[]>(`/datasources/${id}/permissions`),
  /**
   * 获取数据源中的数据库列表（元数据浏览入口）。
   * @param id - 数据源 ID
   * @param signal - AbortSignal，支持请求取消
   */
  getDatabases: (id: number, signal?: AbortSignal) =>
    api.get<string[]>(`/datasources/${id}/metadata`, { params: { level: 'databases' }, timeout: 30000, signal, _noRetry: true }),
  /**
   * 获取指定数据库中的表列表。
   * @param id - 数据源 ID
   * @param database - 数据库名
   * @param signal - AbortSignal，支持请求取消
   */
  getTables: (id: number, database: string, signal?: AbortSignal) =>
    api.get<TableInfo[]>(`/datasources/${id}/metadata`, { params: { level: 'tables', database }, timeout: 30000, signal, _noRetry: true }),
  /**
   * 获取指定表的列信息（字段名、类型、注释、是否可空）。
   * @param id - 数据源 ID
   * @param database - 数据库名
   * @param table - 表名
   * @param signal - AbortSignal，支持请求取消
   */
  getColumns: (id: number, database: string, table: string, signal?: AbortSignal) =>
    api.get<ColumnInfo[]>(`/datasources/${id}/metadata`, { params: { level: 'columns', database, table }, timeout: 30000, signal, _noRetry: true }),
  /**
   * 查询数据源连接池的实时状态（活跃数、空闲数、配置等）。
   * @param id - 数据源 ID
   */
  getPoolStats: (id: number) =>
    api.get<{
      datasource_id: number
      has_pool: boolean
      message?: string
      pool_stats?: { open_count: number; idle_count: number; in_use: number; max_open: number }
      pool_config?: { max_open: number; max_idle: number; max_lifetime: number }
    }>(`/query/pool-stats/${id}`),
  /**
   * 清空数据源的查询结果缓存（命中缓存的查询下次会重新执行）。
   * @param id - 数据源 ID
   */
  clearCache: (id: number) =>
    api.post<{ datasource_id: number; message: string }>(`/query/clear-cache/${id}`),
}

/** SQL 查询、历史、已保存 SQL 等 API */
export const queryAPI = {
  /**
   * 执行 SQL 查询。同步返回 QueryResult；对于长耗时查询建议使用 streamResult 走 SSE。
   * @param data - 数据源 ID + SQL + 可选数据库名
   */
  execute: (data: { datasource_id: number; sql: string; database?: string }) =>
    api.post<QueryResult>('/query/execute', data, { timeout: 120000 }),
  /**
   * 轮询获取异步查询结果（当 execute 返回 query_id 时使用）。
   * @param queryId - 查询 ID
   */
  getResult: (queryId: string) =>
    api.get<QueryResult>(`/query/result/${queryId}`),
  /**
   * 以 SSE（Server-Sent Events）方式流式接收查询结果。
   * 直接返回 EventSource 实例，token 通过 query 参数传递以绕过 Authorization 拦截器。
   * @param queryId - 查询 ID
   */
  streamResult: (queryId: string): EventSource => {
    const baseURL = api.defaults.baseURL || '/api'
    const token = sessionStorage.getItem('token') || ''
    const url = `${baseURL}/query/stream/${queryId}?token=${encodeURIComponent(token)}`
    return new EventSource(url)
  },
  /**
   * 取消正在执行的查询。
   * @param queryId - 查询 ID
   */
  cancel: (queryId: string) =>
    api.post(`/query/cancel/${queryId}`),
  /**
   * 将 SQL 查询结果导出为 CSV 文件（以二进制流形式下载）。
   * @param data - 数据源 ID + SQL + 可选数据库名 + 可选最大行数
   * @param onDownloadProgress - 下载进度回调
   */
  exportCSV: (data: { datasource_id: number; sql: string; database?: string; max_rows?: number }, onDownloadProgress?: (event: AxiosProgressEvent) => void) =>
    api.post('/query/export', data, { responseType: 'blob', timeout: 120000, onDownloadProgress }),
  /**
   * 分页查询查询历史记录，支持领域、数据源、状态、时间范围等过滤。
   * @param params - 查询参数
   */
  getHistory: (params?: { domain_id?: number; page?: number; page_size?: number; datasource_id?: number; status?: string; start_time?: string; end_time?: string; search?: string }) =>
    api.get<{ items: QueryHistory[]; total: number; page: number; page_size: number }>('/query/history', { params }),
  /**
   * 删除单条查询历史记录。
   * @param id - 历史记录 ID
   */
  deleteHistory: (id: number) =>
    api.delete(`/query/history/${id}`),
  /**
   * 批量删除查询历史记录。
   * @param ids - 历史记录 ID 数组
   */
  batchDeleteHistory: (ids: number[]) =>
    api.post('/query/history/batch-delete', { ids }),
  /**
   * 分页查询当前用户可见的已保存 SQL 片段。
   * @param params - 查询参数
   */
  listSavedSQL: (params?: { domain_id?: number; page?: number; page_size?: number; search?: string }) =>
    api.get<{ items: SavedSQL[]; total: number; page: number; page_size: number }>('/query/saved-sql', { params }),
  /**
   * 保存一条 SQL 片段（可选择是否领域内公开）。
   * @param data - SQL 片段信息
   */
  saveSQL: (data: { name: string; datasource_id: number; database?: string; sql_text: string; description?: string; is_public?: boolean }) =>
    api.post('/query/saved-sql', data),
  /**
   * 更新已保存的 SQL 片段。
   * @param id - 片段 ID
   * @param data - 更新字段
   */
  updateSavedSQL: (id: number, data: { name: string; datasource_id: number; database?: string; sql_text: string; description?: string; is_public?: boolean }) =>
    api.put(`/query/saved-sql/${id}`, data),
  /**
   * 删除已保存的 SQL 片段。
   * @param id - 片段 ID
   */
  deleteSavedSQL: (id: number) =>
    api.delete(`/query/saved-sql/${id}`),
}
