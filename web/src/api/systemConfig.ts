/**
 * @file systemConfig.ts - 系统运行时配置 API 封装
 *
 * 覆盖后端 /admin/system-config 路由组（需 SystemAdmin 权限），
 * 提供配置项列表查询、单项更新、全量重载三个接口。
 * 配置项的值持久化在数据库，后端启动时会加载到内存缓存。
 */
import api from '@/utils/api'
import type { SystemConfigItem } from '@/types'

/** 系统运行时配置 API 封装对象 */
export const systemConfigAPI = {
  /** 获取所有配置项（含元数据、默认值、当前值和分组信息） */
  list: () =>
    api.get<SystemConfigItem[]>('/admin/system-config'),
  /**
   * 更新指定配置项的值。
   * @param key - 配置项键名，对应 SystemConfigItem.key
   * @param value - 更新载荷，仅需 value 字段
   */
  update: (key: string, value: { value: string }) =>
    api.put(`/admin/system-config/${key}`, value),
  /** 手动触发后端从 DB 全量重载配置到内存缓存 */
  reload: () =>
    api.post<{ message: string }>('/admin/system-config/reload'),
}
