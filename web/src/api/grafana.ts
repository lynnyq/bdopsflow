import api from '@/utils/api'
import type { GrafanaDashboard, GrafanaDashboardInput, GrafanaReorderItem } from '@/types'

export const grafanaAPI = {
  // 列出启用的图表（Dashboard 嵌入用）
  listEnabled: () => api.get<{ items: GrafanaDashboard[] }>('/grafana-dashboards'),
  // 列出全部图表（管理页用）
  listAll: () => api.get<{ items: GrafanaDashboard[] }>('/admin/grafana-dashboards'),
  get: (id: number) => api.get<GrafanaDashboard>(`/admin/grafana-dashboards/${id}`),
  create: (data: GrafanaDashboardInput) => api.post<GrafanaDashboard>('/admin/grafana-dashboards', data),
  update: (id: number, data: GrafanaDashboardInput) => api.put(`/admin/grafana-dashboards/${id}`, data),
  delete: (id: number) => api.delete(`/admin/grafana-dashboards/${id}`),
  reorder: (items: GrafanaReorderItem[]) =>
    api.put('/admin/grafana-dashboards/reorder', { items }),
}
