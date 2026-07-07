import api from '@/utils/api'
import type { SystemConfigItem } from '@/types'

// 系统配置 API。
// 对应后端路由：/admin/system-config（需 SystemAdmin 权限）。
export const systemConfigAPI = {
  // 获取所有配置项（含元数据与当前值）
  list: () =>
    api.get<SystemConfigItem[]>('/admin/system-config'),
  // 更新指定配置项的值
  update: (key: string, value: { value: string }) =>
    api.put(`/admin/system-config/${key}`, value),
  // 手动触发配置重载（从 DB 全量刷新到内存缓存）
  reload: () =>
    api.post<{ message: string }>('/admin/system-config/reload'),
}
