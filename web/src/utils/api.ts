import axios from 'axios'
import { ElMessage } from 'element-plus'
import { ERROR_CODE_MAP } from '@/utils/error'
import { useAuthStore } from '@/stores/auth'

export interface ApiResponse<T = any> {
  code: number
  status: string
  message: string
  data: T
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
  async (error) => {
    const originalRequest = error.config

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
      error.response.data = { error: responseData.message, code: responseData.code }
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

export function isHandledError(err: any): boolean {
  return !!err?._handled
}

export default api
