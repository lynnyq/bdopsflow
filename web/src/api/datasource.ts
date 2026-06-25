import api from '@/utils/api'
import type { Datasource, DatasourcePermission, QueryResult, QueryHistory, SavedSQL, TableInfo, ColumnInfo, SystemConfigItem } from '@/types'
import type { AxiosProgressEvent } from 'axios'

export const datasourceAPI = {
  list: (params?: { domain_id?: number; type?: string; page?: number; page_size?: number }) =>
    api.get<{ items: Datasource[]; total: number; page: number; page_size: number }>('/datasources', { params }),
  get: (id: number) =>
    api.get<Datasource>(`/datasources/${id}`),
  create: (data: Partial<Datasource>) =>
    api.post('/datasources', data),
  update: (id: number, data: Partial<Datasource>) =>
    api.put(`/datasources/${id}`, data),
  delete: (id: number) =>
    api.delete(`/datasources/${id}`),
  testConnection: (id: number) =>
    api.post(`/datasources/${id}/test`),
  testConnectionByParams: (data: Partial<Datasource>) =>
    api.post('/datasources/test', data),
  supportedTypes: () =>
    api.get<string[]>('/datasources/types'),
  grantPermission: (id: number, data: { role_id?: number; user_id?: number; permission_type: string }) =>
    api.post(`/datasources/${id}/permissions`, data),
  updatePermission: (datasourceId: number, permId: number, data: { permission_type: string }) =>
    api.put(`/datasources/${datasourceId}/permissions/${permId}`, data),
  revokePermission: (datasourceId: number, permId: number) =>
    api.delete(`/datasources/${datasourceId}/permissions/${permId}`),
  getPermissions: (id: number) =>
    api.get<DatasourcePermission[]>(`/datasources/${id}/permissions`),
  getDatabases: (id: number, signal?: AbortSignal) =>
    api.get<string[]>(`/datasources/${id}/metadata`, { params: { level: 'databases' }, timeout: 30000, signal, _noRetry: true }),
  getTables: (id: number, database: string, signal?: AbortSignal) =>
    api.get<TableInfo[]>(`/datasources/${id}/metadata`, { params: { level: 'tables', database }, timeout: 30000, signal, _noRetry: true }),
  getColumns: (id: number, database: string, table: string, signal?: AbortSignal) =>
    api.get<ColumnInfo[]>(`/datasources/${id}/metadata`, { params: { level: 'columns', database, table }, timeout: 30000, signal, _noRetry: true }),
  getPoolStats: (id: number) =>
    api.get<{
      datasource_id: number
      has_pool: boolean
      message?: string
      pool_stats?: { open_count: number; idle_count: number; in_use: number; max_open: number }
      pool_config?: { max_open: number; max_idle: number; max_lifetime: number }
    }>(`/query/pool-stats/${id}`),
  clearCache: (id: number) =>
    api.post<{ datasource_id: number; message: string }>(`/query/clear-cache/${id}`),
}

export const queryAPI = {
  execute: (data: { datasource_id: number; sql: string; database?: string }) =>
    api.post<QueryResult>('/query/execute', data, { timeout: 120000 }),
  getResult: (queryId: string) =>
    api.get<QueryResult>(`/query/result/${queryId}`),
  streamResult: (queryId: string): EventSource => {
    const baseURL = api.defaults.baseURL || '/api'
    const token = sessionStorage.getItem('token') || ''
    const url = `${baseURL}/query/stream/${queryId}?token=${encodeURIComponent(token)}`
    return new EventSource(url)
  },
  cancel: (queryId: string) =>
    api.post(`/query/cancel/${queryId}`),
  exportCSV: (data: { datasource_id: number; sql: string; database?: string; max_rows?: number }, onDownloadProgress?: (event: AxiosProgressEvent) => void) =>
    api.post('/query/export', data, { responseType: 'blob', timeout: 120000, onDownloadProgress }),
  getHistory: (params?: { domain_id?: number; page?: number; page_size?: number; datasource_id?: number; status?: string; start_time?: string; end_time?: string }) =>
    api.get<{ items: QueryHistory[]; total: number; page: number; page_size: number }>('/query/history', { params }),
  deleteHistory: (id: number) =>
    api.delete(`/query/history/${id}`),
  batchDeleteHistory: (ids: number[]) =>
    api.post('/query/history/batch-delete', { ids }),
  listSavedSQL: (params?: { domain_id?: number; page?: number; page_size?: number }) =>
    api.get<{ items: SavedSQL[]; total: number; page: number; page_size: number }>('/query/saved-sql', { params }),
  saveSQL: (data: { name: string; datasource_id: number; sql_text: string; description?: string; is_public?: boolean }) =>
    api.post('/query/saved-sql', data),
  updateSavedSQL: (id: number, data: { name: string; datasource_id: number; sql_text: string; description?: string; is_public?: boolean }) =>
    api.put(`/query/saved-sql/${id}`, data),
  deleteSavedSQL: (id: number) =>
    api.delete(`/query/saved-sql/${id}`),
}

export const systemConfigAPI = {
  list: () =>
    api.get<SystemConfigItem[]>('/admin/system-config'),
  update: (key: string, value: { value: string }) =>
    api.put(`/admin/system-config/${key}`, value),
}
