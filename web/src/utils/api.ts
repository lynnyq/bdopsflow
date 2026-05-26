import axios from 'axios'
import { ElMessage } from 'element-plus'
import { ERROR_CODE_MAP } from '@/utils/error'

// 统一响应结构
export interface ApiResponse<T = any> {
  code: number      // 业务状态码，0表示成功
  status: string    // 状态："success" 或 "error"
  message: string   // 提示信息
  data: T           // 数据
}

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
})

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
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
          const friendlyMessage = ERROR_CODE_MAP[data.code] || data.message || '请求失败'
          ElMessage.error(friendlyMessage)
        }
        const error = new Error(data.message || '请求失败')
        ;(error as any).code = data.code
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
  (error) => {
    // 在登录页时不触发 401 重定向，避免登录失败时页面刷新
    if (error.response?.status === 401 && !window.location.pathname.includes('/login')) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }

    // 处理统一错误响应格式
    const responseData = error.response?.data as ApiResponse
    if (responseData && typeof responseData === 'object' && 'code' in responseData && 'status' in responseData) {
      // 将统一格式的错误信息转换为前端可识别的格式
      const friendlyMessage = ERROR_CODE_MAP[responseData.code] || responseData.message || '请求失败'
      ElMessage.error(friendlyMessage)
      error.response.data = { error: responseData.message, code: responseData.code }
    }

    return Promise.reject(error)
  }
)

export default api
