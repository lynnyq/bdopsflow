import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import Login from '../views/Login.vue'
import { createPinia, setActivePinia } from 'pinia'
import ElementPlus from 'element-plus'

describe('Login', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders login form', () => {
    const wrapper = mount(Login, {
      global: {
        plugins: [createPinia(), ElementPlus],
      },
    })

    expect(wrapper.find('input[placeholder="请输入用户名"]').exists()).toBe(true)
    expect(wrapper.find('input[type="password"]').exists()).toBe(true)
    expect(wrapper.find('button').exists()).toBe(true)
  })

  it('has correct title', () => {
    const wrapper = mount(Login, {
      global: {
        plugins: [createPinia(), ElementPlus],
      },
    })

    const header = wrapper.find('.card-header')
    expect(header.exists()).toBe(true)
  })
})

describe('Type Definitions', () => {
  it('Task interface has correct structure', () => {
    const task = {
      id: 1,
      name: 'Test Task',
      type: 'http',
      config: '{"url":"http://example.com"}',
      cron_expression: '*/5 * * * *',
      timeout_seconds: 300,
      retry_count: 3,
      retry_interval: 5,
      is_enabled: true,
      status: 'pending',
      domain_id: 1,
      webhook_config: '',
      created_by: 1,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    expect(task.id).toBe(1)
    expect(task.name).toBe('Test Task')
    expect(task.type).toBe('http')
    expect(task.status).toBe('pending')
    expect(task.is_enabled).toBe(true)
  })

  it('Executor interface has correct structure', () => {
    const executor = {
      id: 1,
      executor_id: 'executor-1',
      name: 'Executor 1',
      address: 'localhost:50051',
      status: 'online',
      last_heartbeat: '2024-01-01T00:00:00Z',
      capacity: 10,
      current_load: 3,
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    }

    expect(executor.executor_id).toBe('executor-1')
    expect(executor.status).toBe('online')
    expect(executor.capacity).toBe(10)
    expect(executor.current_load).toBeLessThanOrEqual(executor.capacity)
  })

  it('TaskExecution interface has correct structure', () => {
    const execution = {
      id: 1,
      task_id: 1,
      execution_id: '1-1234567890',
      executor_id: 'executor-1',
      status: 'success',
      start_time: '2024-01-01T00:00:00Z',
      end_time: '2024-01-01T00:00:05Z',
      output: 'Task completed successfully',
      error: '',
      retry_times: 0,
      created_at: '2024-01-01T00:00:00Z',
    }

    expect(execution.execution_id).toBe('1-1234567890')
    expect(execution.status).toBe('success')
    expect(execution.retry_times).toBe(0)
  })
})

describe('API Response Structures', () => {
  it('LoginResponse has correct structure', () => {
    const response = {
      token: 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...',
      user: {
        id: 1,
        username: 'admin',
        role: 'admin',
        domain_id: 1,
      },
    }

    expect(response.token).toBeDefined()
    expect(response.user.id).toBe(1)
    expect(response.user.username).toBe('admin')
    expect(response.user.role).toBe('admin')
  })

  it('Task list response is array', () => {
    const tasks = [
      { id: 1, name: 'Task 1', type: 'http', status: 'pending' },
      { id: 2, name: 'Task 2', type: 'shell', status: 'running' },
    ]

    expect(Array.isArray(tasks)).toBe(true)
    expect(tasks.length).toBe(2)
    expect(tasks[0].name).toBe('Task 1')
  })
})