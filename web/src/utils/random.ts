/**
 * 密码学安全的随机数辅助函数
 * 使用 Web Crypto API (crypto.getRandomValues) 替代不安全的 Math.random()
 */

/**
 * 生成 [0, max) 范围内的安全随机整数（无偏映射）
 * @param max 上限（不含），必须 > 0 且 <= 2^32
 */
export function secureRandomInt(max: number): number {
  if (max <= 0) return 0
  if (max > 0xffffffff) {
    throw new Error('secureRandomInt: max exceeds 32-bit range')
  }

  const crypto = globalThis.crypto
  if (!crypto || typeof crypto.getRandomValues !== 'function') {
    // 降级：理论上不应在现代浏览器触发，但避免运行时崩溃
    // eslint-disable-next-line no-console
    console.warn('[random] crypto.getRandomValues unavailable, falling back to Math.random')
    return Math.floor(Math.random() * max)
  }

  // Rejection sampling 消除取模偏差
  const range = Math.floor(0xffffffff / max) * max
  const buf = new Uint32Array(1)
  while (true) {
    crypto.getRandomValues(buf)
    if (buf[0] < range) {
      return buf[0] % max
    }
  }
}

/**
 * 生成 [0, 1) 范围内的安全随机浮点数（可直接替代 Math.random()）
 */
export function secureRandom(): number {
  const crypto = globalThis.crypto
  if (!crypto || typeof crypto.getRandomValues !== 'function') {
    // eslint-disable-next-line no-console
    console.warn('[random] crypto.getRandomValues unavailable, falling back to Math.random')
    return Math.random()
  }
  const buf = new Uint32Array(1)
  crypto.getRandomValues(buf)
  // 32 位均匀分布映射到 [0, 1)
  return buf[0] / 0xffffffff
}

/**
 * 生成随机十六进制字符串（常用于唯一 ID 的后缀）
 * @param length 字符串长度（偶数更好）
 */
export function secureRandomHex(length: number): string {
  const crypto = globalThis.crypto
  if (!crypto || typeof crypto.getRandomValues !== 'function') {
    // 降级方案
    return Array.from({ length }, () =>
      Math.floor(Math.random() * 16).toString(16),
    ).join('')
  }
  const byteLen = Math.ceil(length / 2)
  const buf = new Uint8Array(byteLen)
  crypto.getRandomValues(buf)
  let hex = ''
  for (let i = 0; i < buf.length; i++) {
    hex += buf[i].toString(16).padStart(2, '0')
  }
  return hex.slice(0, length)
}
