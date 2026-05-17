import { ElMessage } from 'element-plus'

export enum ErrorType {
  NETWORK = '网络错误，请检查网络连接',
  TIMEOUT = '请求超时，请稍后重试',
  SERVER = '服务器错误，请联系管理员',
  AUTH = '认证失败，请重新登录',
  VALIDATION = '数据验证失败',
  UNKNOWN = '操作失败，请稍后重试',
  NOT_FOUND = '请求的资源不存在',
  FORBIDDEN = '没有权限执行此操作',
}

// 英文化错误信息映射表
const errorMessageMap: Record<string, string> = {
  // 认证相关
  'unauthorized': '未授权，请重新登录',
  'invalid credentials': '用户名或密码错误',
  'invalid user': '用户无效',
  'permission denied': '没有权限执行此操作',
  'user not found': '用户不存在',
  // 参数相关
  'invalid id': '无效的ID',
  'id must be positive': 'ID必须为正数',
  'invalid request': '无效的请求',
  'invalid request body': '无效的请求体',
  'name is required': '名称是必填项',
  'type is required': '类型是必填项',
  // 资源相关
  'not found': '资源不存在',
  'task not found': '任务不存在',
  'workflow not found': '工作流不存在',
  'workflow execution not found': '工作流执行记录不存在',
  'role not found': '角色不存在',
  'domain not found': '领域不存在',
  // 服务器相关
  'internal server error': '服务器错误，请稍后重试',
  'server error': '服务器错误，请稍后重试',
  // 其他
  'executionId required': '执行ID是必填项',
  'invalid request: capacity must be a positive integer': '无效的请求：容量必须为正整数',
  'invalid domain id': '无效的领域ID',
  'domain id must be positive': '领域ID必须为正数',
}

// 将英文错误信息转换为中文
export const translateErrorMessage = (errorMsg: string): string => {
  if (!errorMsg) return ErrorType.UNKNOWN

  const lowerMsg = errorMsg.toLowerCase().trim()

  // 首先查找精确匹配
  if (errorMessageMap[lowerMsg]) {
    return errorMessageMap[lowerMsg]
  }

  // 查找部分匹配
  for (const [key, value] of Object.entries(errorMessageMap)) {
    if (lowerMsg.includes(key)) {
      return value
    }
  }

  // 检查是否包含常见的英文关键词
  if (lowerMsg.includes('unauthorized') || lowerMsg.includes('401')) {
    return '未授权，请重新登录'
  }
  if (lowerMsg.includes('forbidden') || lowerMsg.includes('403')) {
    return '没有权限执行此操作'
  }
  if (lowerMsg.includes('not found') || lowerMsg.includes('404')) {
    return '请求的资源不存在'
  }
  if (lowerMsg.includes('server') || lowerMsg.includes('500')) {
    return '服务器错误，请稍后重试'
  }
  if (lowerMsg.includes('network') || lowerMsg.includes('timeout')) {
    return '网络错误，请检查网络连接'
  }
  if (lowerMsg.includes('invalid') || lowerMsg.includes('bad')) {
    return '请求参数错误'
  }

  // 如果已经是中文，直接返回
  if (/[\u4e00-\u9fa5]/.test(errorMsg)) {
    return errorMsg
  }

  return errorMsg
}

export const handleError = (
  error: any,
  fallback: string = ErrorType.UNKNOWN
): string => {
  // 网络错误
  if (!error.response) {
    if (error.code === 'ECONNABORTED') {
      ElMessage.error(ErrorType.TIMEOUT)
      return ErrorType.TIMEOUT
    }
    ElMessage.error(ErrorType.NETWORK)
    return ErrorType.NETWORK
  }

  // 服务器错误
  const status = error.response?.status
  let errorMsg = error.response?.data?.error || error.message

  // 翻译错误信息
  errorMsg = translateErrorMessage(errorMsg)

  switch (status) {
    case 400:
      ElMessage.error(errorMsg || ErrorType.VALIDATION)
      return errorMsg || ErrorType.VALIDATION
    case 401:
      ElMessage.error(errorMsg || ErrorType.AUTH)
      return errorMsg || ErrorType.AUTH
    case 403:
      ElMessage.error(errorMsg || ErrorType.FORBIDDEN)
      return errorMsg || ErrorType.FORBIDDEN
    case 404:
      ElMessage.error(errorMsg || ErrorType.NOT_FOUND)
      return errorMsg || ErrorType.NOT_FOUND
    case 500:
    case 502:
    case 503:
      ElMessage.error(errorMsg || ErrorType.SERVER)
      return errorMsg || ErrorType.SERVER
    default:
      ElMessage.error(errorMsg || fallback)
      return errorMsg || fallback
  }
}

export const handleSuccess = (message: string = '操作成功') => {
  ElMessage.success(message)
}

export const handleWarning = (message: string) => {
  ElMessage.warning(message)
}

export const handleInfo = (message: string) => {
  ElMessage.info(message)
}

// 格式化错误消息
export const formatErrorMessage = (error: any): string => {
  if (!error) return '未知错误'
  if (typeof error === 'string') return error
  if (error.message) return error.message
  if (error.response?.data?.error) return error.response.data.error
  if (error.response?.data?.message) return error.response.data.message
  return '操作失败'
}

// 验证表单字段错误
export const validateFieldError = (errors: any, field: string): string | undefined => {
  if (!errors) return undefined
  const fieldError = errors[field]
  if (Array.isArray(fieldError) && fieldError.length > 0) {
    return fieldError[0]?.message || fieldError[0]
  }
  return undefined
}

// 网络请求重试包装器
export const withRetry = async <T>(
  fn: () => Promise<T>,
  maxRetries: number = 3,
  delay: number = 1000
): Promise<T> => {
  let lastError: Error

  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn()
    } catch (error: any) {
      lastError = error

      // 如果是客户端错误，不重试
      if (error.response?.status < 500) {
        throw error
      }

      // 等待后重试
      await new Promise((resolve) => setTimeout(resolve, delay * Math.pow(2, i)))
    }
  }

  throw lastError!
}

// 空值处理工具
export const formatValue = (value: any, fallback: string = '-'): string => {
  if (value === null || value === undefined || value === '') {
    return fallback
  }
  if (typeof value === 'object') {
    return JSON.stringify(value)
  }
  return String(value)
}

// 数字格式化
export const formatNumber = (num: number | null | undefined, decimals: number = 0): string => {
  if (num === null || num === undefined) return '-'
  return num.toFixed(decimals)
}

// 日期格式化
export const formatDate = (date: string | Date | null | undefined, format: string = 'YYYY-MM-DD HH:mm:ss'): string => {
  if (!date) return '-'

  const d = typeof date === 'string' ? new Date(date) : date

  if (isNaN(d.getTime())) return '-'

  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  const hours = String(d.getHours()).padStart(2, '0')
  const minutes = String(d.getMinutes()).padStart(2, '0')
  const seconds = String(d.getSeconds()).padStart(2, '0')

  return format
    .replace('YYYY', String(year))
    .replace('MM', month)
    .replace('DD', day)
    .replace('HH', hours)
    .replace('mm', minutes)
    .replace('ss', seconds)
}

// 文本截断
export const truncateText = (text: string, maxLength: number = 50): string => {
  if (!text) return ''
  if (text.length <= maxLength) return text
  return text.substring(0, maxLength) + '...'
}

// 脱敏工具
export const maskSensitive = (text: string, showChars: number = 4): string => {
  if (!text || text.length <= showChars) return '***'
  const maskLength = text.length - showChars
  return '*'.repeat(maskLength) + text.substring(text.length - showChars)
}
