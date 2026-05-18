import axios from 'axios'

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
    // 处理统一响应格式
    const data = response.data as ApiResponse
    
    // 如果响应符合统一格式
    if (data && typeof data === 'object' && 'code' in data && 'status' in data) {
      // 业务错误（code !== 0）
      if (data.code !== 0) {
        const error = new Error(data.message || '请求失败')
        ;(error as any).response = {
          data: { error: data.message },
          status: response.status
        }
        return Promise.reject(error)
      }
      // 成功时，将 data.data 作为实际数据返回
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
      error.response.data = { error: responseData.message }
    }
    
    return Promise.reject(error)
  }
)

export default api
