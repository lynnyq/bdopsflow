/**
 * @file webhook.ts - Webhook 管理 API 封装
 * 提供 Webhook 的列表查询（支持按领域过滤）、创建、更新、删除、测试触发等接口，
 * 供 api/index.ts 二次导出。
 */
import api from '@/utils/api'
import type { Webhook } from '@/types'

/** Webhook 管理 API 封装对象 */
export const webhookAPI = {
  /**
   * 查询 Webhook 列表。可选按领域过滤；传入 domainId > 0 时拼接 query string。
   * @param domainId - 可选的领域 ID 过滤条件
   */
  list: (domainId?: number) => {
    if (domainId && domainId > 0) {
      return api.get<{ items: Webhook[] }>(`/webhooks?domain_id=${domainId}`)
    }
    return api.get<{ items: Webhook[] }>('/webhooks')
  },
  /**
   * 创建新的 Webhook 回调配置。
   * @param data - Webhook 字段
   */
  create: (data: Partial<Webhook>) => api.post<Webhook>('/webhooks', data),
  /**
   * 更新已有 Webhook。
   * @param id - Webhook ID
   * @param data - 更新字段
   */
  update: (id: number, data: Partial<Webhook>) => api.put(`/webhooks/${id}`, data),
  /**
   * 删除 Webhook。
   * @param id - Webhook ID
   */
  delete: (id: number) => api.delete(`/webhooks/${id}`),
  /**
   * 手动触发一次 Webhook 测试，后端会向配置的 URL 发送一次请求并返回结果。
   * @param id - Webhook ID
   */
  test: (id: number) => api.post(`/webhooks/${id}/test`),
}
