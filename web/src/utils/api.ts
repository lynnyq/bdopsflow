import axios from 'axios'
import type { AxiosError } from 'axios'
import { ElMessage } from 'element-plus'
import { ERROR_CODE_MAP } from '@/utils/error'
import { useAuthStore } from '@/stores/auth'

// 扩展 AxiosRequestConfig 类型以支持自定义属性
declare module 'axios' {
  interface AxiosRequestConfig {
    _noRetry?: boolean
    _retryCount?: number
    _retry?: boolean
  }
}

export interface ApiResponse<T = any> {
  code: number
  status: string
  message: string
  data: T
}

// 重试配置
interface RetryConfig {
  maxRetries: number      // 最大重试次数
  retryDelay: number      // 重试延迟（毫秒）
  retryCondition: (error: AxiosError) => boolean  // 重试条件判断
}

const defaultRetryConfig: RetryConfig = {
  maxRetries: 3,
  retryDelay: 1000,
  retryCondition: (error: AxiosError) => {
    // 网络错误、超时、5xx 错误可重试
    if (!error.response) return true  // 网络错误
    if (error.code === 'ECONNABORTED') return true  // 超时
    const status = error.response.status
    return status >= 500 && status < 600  // 服务器错误
  }
}

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

function subscribeTokenRefresh(cb: (token: string) => void) {
  refreshSubscribers.push(cb)
}

function onTokenRefreshed(newToken: string) {
  refreshSubscribers.forEach(cb => cb(newToken))
  refreshSubscribers = []
}

api.interceptors.request.use((config) => {
  const token = sessionStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  // 初始化重试计数
  if (!config._retryCount) {
    config._retryCount = 0
  }
  return config
})

api.interceptors.response.use(
  (response) => {
    const data = response.data as ApiResponse

    if (data && typeof data === 'object' && 'code' in data && 'status' in data) {
      if (data.code !== 0) {
        const isAuthEndpoint = response.config.url?.includes('/auth/login') || response.config.url?.includes('/auth/sso-login')
        if (!isAuthEndpoint) {
          const friendlyMessage = data.message || ERROR_CODE_MAP[data.code] || '请求失败'
          ElMessage.error(friendlyMessage)
        }
        const error = new Error(data.message || '请求失败')
        ;(error as any).code = data.code
        ;(error as any)._handled = true
        ;(error as any).response = {
          data: { error: data.message, code: data.code },
          status: response.status
        }
        return Promise.reject(error)
      }
      response.data = data.data
    }

    return response
  },
  async (error: AxiosError) => {
    const originalRequest = error.config as any

    // 重试逻辑（在 token 刷新之前处理）
    // _noRetry 标志用于禁用自动重试（如 metadata 请求在慢数据源上重试会加剧连接池耗尽）
    if (originalRequest && !originalRequest._noRetry && defaultRetryConfig.retryCondition(error)) {
      const retryCount = originalRequest._retryCount || 0

      if (retryCount < defaultRetryConfig.maxRetries) {
        originalRequest._retryCount = retryCount + 1

        // 指数退避延迟：1s, 2s, 4s
        const delay = defaultRetryConfig.retryDelay * Math.pow(2, retryCount)

        if (import.meta.env.DEV) {
          console.log(`[API Retry] ${originalRequest.url} - Attempt ${retryCount + 1}/${defaultRetryConfig.maxRetries} after ${delay}ms`)
        }

        await new Promise(resolve => setTimeout(resolve, delay))

        return api(originalRequest)
      }
    }

    // Token 刷新逻辑（401 错误）
    if (error.response?.status === 401 && !originalRequest._retry) {
      const refreshToken = sessionStorage.getItem('refresh_token')

      if (refreshToken && !window.location.pathname.includes('/login')) {
        if (isRefreshing) {
          return new Promise((resolve) => {
            subscribeTokenRefresh((newToken: string) => {
              originalRequest.headers.Authorization = `Bearer ${newToken}`
              resolve(api(originalRequest))
            })
          })
        }

        originalRequest._retry = true
        isRefreshing = true

        try {
          const response = await axios.post('/api/auth/refresh', {
            refresh_token: refreshToken,
          })

          const { token: newToken, refresh_token: newRefreshToken } = response.data.data || response.data
          sessionStorage.setItem('token', newToken)
          if (newRefreshToken) {
            sessionStorage.setItem('refresh_token', newRefreshToken)
          }

          try {
            const authStore = useAuthStore()
            authStore.setToken(newToken)
            if (newRefreshToken) {
              authStore.setRefreshToken(newRefreshToken)
            }
          } catch {
            // store may not be initialized yet
          }

          onTokenRefreshed(newToken)

          originalRequest.headers.Authorization = `Bearer ${newToken}`
          return api(originalRequest)
        } catch (refreshError) {
          try {
            const authStore = useAuthStore()
            authStore.logout()
          } catch {
            sessionStorage.removeItem('token')
            sessionStorage.removeItem('refresh_token')
          }
          window.location.href = '/login'
          return Promise.reject(refreshError)
        } finally {
          isRefreshing = false
        }
      }

      if (!refreshToken && !window.location.pathname.includes('/login')) {
        try {
          const authStore = useAuthStore()
          authStore.logout()
        } catch {
          sessionStorage.removeItem('token')
          sessionStorage.removeItem('refresh_token')
        }
        window.location.href = '/login'
      }
    }

    const responseData = error.response?.data as ApiResponse
    if (responseData && typeof responseData === 'object' && 'code' in responseData && 'status' in responseData) {
      const friendlyMessage = responseData.message || ERROR_CODE_MAP[responseData.code] || '请求失败'
      ElMessage.error(friendlyMessage)
      if (error.response) {
        error.response.data = { error: responseData.message, code: responseData.code }
      }
      ;(error as any)._handled = true
    } else if (error.response?.status) {
      const statusMessages: Record<number, string> = {
        400: '请求参数错误',
        401: '登录已过期，请重新登录',
        403: '权限不足，无法执行此操作',
        404: '请求的资源不存在',
        500: '服务器内部错误',
        502: '网关错误',
        503: '服务暂不可用',
      }
      const msg = statusMessages[error.response.status] || `请求失败 (${error.response.status})`
      ElMessage.error(msg)
      ;(error as any)._handled = true
    }

    return Promise.reject(error)
  }
)

export function isHandledError(err: unknown): boolean {
  return !!err && typeof err === 'object' && '_handled' in err && Boolean((err as Record<string, unknown>)._handled)
}

export default api
