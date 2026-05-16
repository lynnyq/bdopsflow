export const passwordUtils = {
  encodePassword(password: string): string {
    return btoa(password)
  },

  decodePassword(encodedPassword: string): string {
    try {
      return atob(encodedPassword)
    } catch {
      return ''
    }
  },
}

export const validatePassword = (password: string): { valid: boolean; message: string } => {
  if (password.length < 6) {
    return { valid: false, message: '密码长度至少为6位' }
  }
  if (password.length > 100) {
    return { valid: false, message: '密码长度不能超过100位' }
  }
  return { valid: true, message: '' }
}
