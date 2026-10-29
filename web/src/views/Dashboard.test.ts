import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import Dashboard from './Dashboard.vue'

// vi.mock 工厂会被提升到文件顶部，内部数据必须自包含
// as const 确保 width 保持字面量类型 'half' 而非被推断为 string（与 GrafanaDashboard 接口兼容）
const { mockGrafanaItems } = vi.hoisted(() => ({
  mockGrafanaItems: [
    {
      id: 1,
      name: '任务执行概览',
      description: '',
      embed_url: 'http://grafana.example.com/d/abc?kiosk',
      width: 'half' as const,
      height_px: 300,
      sort_order: 1,
      enabled: 1,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
  ],
}))

// Dashboard 挂载时会调用 dashboardAPI 与 grafanaAPI，统一 mock 避免真实请求
vi.mock('@/api', () => ({
  dashboardAPI: {
    getStats: vi.fn().mockResolvedValue({ data: {} }),
    getSchedulerStatus: vi.fn().mockResolvedValue({ data: { paused: false } }),
    getTrends: vi.fn().mockResolvedValue({ data: { items: [] } }),
    getHealth: vi.fn().mockResolvedValue({ data: {} }),
    pauseScheduler: vi.fn().mockResolvedValue({ data: {} }),
    resumeScheduler: vi.fn().mockResolvedValue({ data: {} }),
  },
  grafanaAPI: {
    listEnabled: vi.fn().mockResolvedValue({ data: { items: mockGrafanaItems } }),
  },
}))

vi.mock('@/utils/api', () => ({
  isHandledError: vi.fn(() => false),
}))

import { grafanaAPI } from '@/api'

const mountDashboard = async (): Promise<VueWrapper> => {
  const wrapper = mount(Dashboard, {
    global: {
      plugins: [createPinia(), ElementPlus],
    },
    attachTo: document.body,
  })
  await flushPromises()
  return wrapper
}

describe('Dashboard Grafana iframe 刷新', () => {
  let wrapper: VueWrapper

  beforeEach(() => {
    vi.clearAllMocks()
    // as any 绕过 AxiosResponse 完整形状要求（测试中不需要 status/headers 等）
    vi.mocked(grafanaAPI.listEnabled).mockResolvedValue({ data: { items: mockGrafanaItems } } as any)
  })

  afterEach(() => {
    // 卸载组件以清理 onMounted 中注册的定时器
    wrapper?.unmount()
  })

  it('挂载后渲染 Grafana iframe 且 src 正确', async () => {
    wrapper = await mountDashboard()

    const iframe = wrapper.find('.grafana-iframe')
    expect(iframe.exists()).toBe(true)
    expect(iframe.attributes('src')).toBe(mockGrafanaItems[0].embed_url)
  })

  it('点击主"刷新数据"按钮后 iframe 元素被重建（强制重新加载图表）', async () => {
    wrapper = await mountDashboard()

    const before = wrapper.find('.grafana-iframe')
    const beforeEl = before.element

    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()

    const after = wrapper.find('.grafana-iframe')
    // key 变更后 Vue 销毁旧 iframe 并创建新 DOM 元素，浏览器才会重新加载图表
    expect(after.element).not.toBe(beforeEl)
    expect(after.attributes('src')).toBe(mockGrafanaItems[0].embed_url)
  })

  it('点击 Grafana 区块刷新按钮后 iframe 元素被重建', async () => {
    wrapper = await mountDashboard()

    const beforeEl = wrapper.find('.grafana-iframe').element

    await wrapper.find('.grafana-section .el-button').trigger('click')
    await flushPromises()

    const after = wrapper.find('.grafana-iframe')
    expect(after.element).not.toBe(beforeEl)
    expect(after.attributes('src')).toBe(mockGrafanaItems[0].embed_url)
    // 区块刷新按钮同时会重新拉取图表列表
    expect(grafanaAPI.listEnabled).toHaveBeenCalled()
  })
})
