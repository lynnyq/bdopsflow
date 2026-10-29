/**
 * @file api.ts —— 全局 HTTP 请求模块
 *
 * 基于 axios 封装的 API 客户端，提供以下能力：
 * - 请求拦截器：自动注入 Bearer Token、初始化重试计数器
 * - 响应拦截器：统一解析 ApiResponse 结构、处理业务错误码
 * - 自动重试：网络错误/超时/5xx 指数退避重试（可通过 _noRetry 禁用）
 * - Token 刷新：401 时用 refresh_token 自动换取新 token，并发请求队列化等待
 * - 错误提示：通过 ElMessage 展示友好的中文错误信息
 */
import axios from 'axios'
import type { AxiosError } from 'axios'
import { ElMessage } from 'element-plus'
import { ERROR_CODE_MAP } from '@/utils/error'
import { useAuthStore } from '@/stores/auth'

/** 扩展 AxiosRequestConfig 类型以支持自定义控制属性 */
declare module 'axios' {
  interface AxiosRequestConfig {
    _noRetry?: boolean
    _retryCount?: number
    _retry?: boolean
  }
}

/**
 * 后端统一响应结构
 * @template T - data 字段的业务数据类型
 */
export interface ApiResponse<T = any> {
  /** 业务状态码，0 表示成功 */
  code: number
  /** 业务状态字符串，如 "ok"、"error" */
  status: string
  /** 业务消息，成功时通常为空 */
  message: string
  /** 业务数据 */
  data: T
}

/** 自动重试策略配置 */
interface RetryConfig {
  maxRetries: number      // 最大重试次数
  retryDelay: number      // 初始重试延迟（毫秒），后续按指数退避翻倍
  retryCondition: (error: AxiosError) => boolean  // 判断该错误是否可重试
}

/** 默认重试策略：最多 3 次、初始 1s、网络错误/超时/5xx 可重试 */
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

/** 单例 axios 实例，baseURL 指向后端 /api 前缀，30s 超时 */
const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

// ==================== Token 刷新队列管理 ====================

/** 是否有请求正在执行 token 刷新 */
let isRefreshing = false
/** 等待 token 刷新完成的请求回调队列 */
let refreshSubscribers: Array<(token: string) => void> = []

/** 将请求加入等待队列，待新 token 就绪后重试 */
function subscribeTokenRefresh(cb: (token: string) => void) {
  refreshSubscribers.push(cb)
}

/** token 刷新成功后通知所有等待请求并重试 */
function onTokenRefreshed(newToken: string) {
  refreshSubscribers.forEach(cb => cb(newToken))
  refreshSubscribers = []
}

// ==================== 请求拦截器 ====================

/**
 * 请求拦截器
 * - 自动从 sessionStorage 读取 token 注入 Authorization 头
 * - 初始化请求级别的重试计数器（用于指数退避）
 */
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

// ==================== 响应拦截器 ====================

/**
 * 响应拦截器
 *
 * 成功路径：
 * - 识别后端 ApiResponse 结构（含 code/status 字段）
 * - code !== 0 视为业务失败，弹出友好提示并 reject
 * - code === 0 时解包 response.data 为业务数据后返回
 *
 * 失败路径：
 * - 先处理指数退避自动重试（_noRetry 可禁用）
 * - 401 时尝试用 refresh_token 换新 token，并发请求队列化
 * - 刷新失败则清空登录态并跳转 /login
 * - 其他错误统一用 ElMessage 展示友好中文消息
 */
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

/**
 * 判断错误是否已被全局拦截器处理过
 * - 拦截器弹出 ElMessage 提示后会在 error 上标记 _handled
 * - 业务层可据此决定是否重复展示错误消息
 * @param err - 待判断的错误对象
 * @returns 是否已处理
 */
export function isHandledError(err: unknown): boolean {
  return !!err && typeof err === 'object' && '_handled' in err && Boolean((err as Record<string, unknown>)._handled)
}

export default api
