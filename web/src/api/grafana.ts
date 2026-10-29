/**
 * @file grafana.ts - Grafana 仪表盘管理 API 封装
 *
 * 覆盖后端 /admin/grafana-dashboards 路由组（需管理权限），
 * 提供仪表盘的增删改查、排序以及启用版本的列表接口（用于仪表盘嵌入）。
 */
import api from '@/utils/api'
import type { GrafanaDashboard, GrafanaDashboardInput, GrafanaReorderItem } from '@/types'

/** Grafana 仪表盘管理 API 封装对象 */
export const grafanaAPI = {
  /** 获取所有已启用的仪表盘列表（用于首页 Dashboard 嵌入） */
  listEnabled: () => api.get<{ items: GrafanaDashboard[] }>('/grafana-dashboards'),
  /** 获取全部仪表盘列表（管理页使用，包含启用/禁用状态） */
  listAll: () => api.get<{ items: GrafanaDashboard[] }>('/admin/grafana-dashboards'),
  /**
   * 获取单个仪表盘详情。
   * @param id - 仪表盘 ID
   */
  get: (id: number) => api.get<GrafanaDashboard>(`/admin/grafana-dashboards/${id}`),
  /**
   * 创建新的 Grafana 仪表盘嵌入配置。
   * @param data - 仪表盘字段（标题、iframe URL、排序等）
   */
  create: (data: GrafanaDashboardInput) => api.post<GrafanaDashboard>('/admin/grafana-dashboards', data),
  /**
   * 更新已有仪表盘配置。
   * @param id - 仪表盘 ID
   * @param data - 更新字段
   */
  update: (id: number, data: GrafanaDashboardInput) => api.put(`/admin/grafana-dashboards/${id}`, data),
  /**
   * 删除指定仪表盘。
   * @param id - 仪表盘 ID
   */
  delete: (id: number) => api.delete(`/admin/grafana-dashboards/${id}`),
  /**
   * 批量更新仪表盘展示顺序。
   * @param items - 包含 id 和 sort_order 的数组
   */
  reorder: (items: GrafanaReorderItem[]) =>
    api.put('/admin/grafana-dashboards/reorder', { items }),
}
