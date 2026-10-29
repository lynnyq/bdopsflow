/**
 * 日期时间格式化工具函数集合。
 * 统一管理前端所有页面的日期格式化逻辑，避免各页面重复定义 formatDate/formatTime。
 */

/**
 * 将 ISO 日期字符串格式化为可读的 'YYYY-MM-DD HH:mm:ss' 格式。
 * 处理无效日期时返回空字符串（而非 'Invalid Date'）。
 *
 * @param dateStr - ISO 8601 或其他标准日期格式字符串
 * @returns 格式化后的日期时间字符串，无效输入返回 '-'
 *
 * @example
 * ```ts
 * formatDateTime('2024-01-15T10:30:00Z') // '2024-01-15 18:30:00'
 * formatDateTime('')                      // '-'
 * formatDateTime(undefined)               // '-'
 * ```
 */
export function formatDateTime(dateStr: string | undefined): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return dateStr
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  const h = String(date.getHours()).padStart(2, '0')
  const min = String(date.getMinutes()).padStart(2, '0')
  const s = String(date.getSeconds()).padStart(2, '0')
  return `${y}-${m}-${d} ${h}:${min}:${s}`
}

/**
 * 将日期格式化为浏览器本地时区的字符串（toLocaleString）。
 * 适用于需要本地化展示的场景。
 *
 * @param date - ISO 日期字符串或 undefined
 * @returns 本地化日期字符串，无效输入返回 '-'
 */
export function formatDateLocal(date: string | undefined): string {
  if (!date) return '-'
  return new Date(date).toLocaleString('zh-CN')
}

/**
 * 格式化时间戳或日期为紧凑时间格式 HH:mm:ss。
 *
 * @param dateStr - 日期字符串
 * @returns 时间部分字符串
 */
export function formatTimeOnly(dateStr: string): string {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  if (isNaN(date.getTime())) return '-'
  const h = String(date.getHours()).padStart(2, '0')
  const m = String(date.getMinutes()).padStart(2, '0')
  const s = String(date.getSeconds()).padStart(2, '0')
  return `${h}:${m}:${s}`
}
