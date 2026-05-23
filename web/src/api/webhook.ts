import api from '@/utils/api'
import type { Webhook } from '@/types'

export const webhookAPI = {
  list: (domainId: number) => api.get<{ items: Webhook[] }>(`/webhooks?domain_id=${domainId}`),
  create: (data: Partial<Webhook>) => api.post<Webhook>('/webhooks', data),
  update: (id: number, data: Partial<Webhook>) => api.put(`/webhooks/${id}`, data),
  delete: (id: number) => api.delete(`/webhooks/${id}`),
  test: (id: number) => api.post(`/webhooks/${id}/test`),
}
