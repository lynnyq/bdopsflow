/**
 * @file navigation.ts - 导航快捷链接 API 封装
 *
 * 覆盖后端 /navigation-links（公开）和 /admin/navigation-links（管理）路由组，
 * 提供导航链接的增删改查、排序、手动刷新图标等操作。
 */
import api from '@/utils/api'
import type { NavigationLink, NavigationLinkInput, NavigationReorderItem } from '@/types'

/** 导航快捷链接 API 封装对象 */
export const navigationAPI = {
  /**
   * 获取所有已启用的导航链接列表（Dashboard 首页用，登录即可）。
   * 返回按 sort_order 升序排列的 enabled=1 记录。
   */
  listEnabled: () => api.get<{ items: NavigationLink[] }>('/navigation-links'),
  /**
   * 获取全部导航链接列表（管理页用，含启用/禁用状态）。
   * 需要 system_admin 权限。
   */
  listAll: () => api.get<{ items: NavigationLink[] }>('/admin/navigation-links'),
  /**
   * 获取单个导航链接详情。
   * @param id - 导航链接 ID
   */
  get: (id: number) => api.get<NavigationLink>(`/admin/navigation-links/${id}`),
  /**
   * 创建新的导航链接。后端会异步自动抓取 favicon（不阻塞主请求）。
   * @param data - 导航链接字段（名称、URL、宽度、密度等）
   */
  create: (data: NavigationLinkInput) => api.post<NavigationLink>('/admin/navigation-links', data),
  /**
   * 更新已有导航链接。URL 变更时后端会自动重新抓取 favicon。
   * @param id - 导航链接 ID
   * @param data - 更新字段
   */
  update: (id: number, data: NavigationLinkInput) => api.put(`/admin/navigation-links/${id}`, data),
  /**
   * 删除指定导航链接。
   * @param id - 导航链接 ID
   */
  delete: (id: number) => api.delete(`/admin/navigation-links/${id}`),
  /**
   * 批量更新导航链接展示顺序。
   * @param items - 包含 id 和 sort_order 的数组
   */
  reorder: (items: NavigationReorderItem[]) =>
    api.put('/admin/navigation-links/reorder', { items }),
  /**
   * 手动触发重新抓取指定导航链接的 favicon。
   * 若本次抓取失败，旧 icon 保留不清空。
   * @param id - 导航链接 ID
   */
  refreshIcon: (id: number) => api.post<NavigationLink>(`/admin/navigation-links/${id}/refresh-icon`),
}
