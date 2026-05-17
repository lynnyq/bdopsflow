import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User } from '@/types'
import { authAPI } from '@/api'
import { passwordUtils } from '@/utils/password'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem('token'))

  const isAdmin = computed(() => {
    return user.value?.role === 'admin' || user.value?.role === 'system_admin'
  })

  const isSystemAdmin = computed(() => {
    return user.value?.role === 'system_admin' || user.value?.role === 'admin'
  })

  const isDomainAdmin = computed(() => {
    return user.value?.role === 'domain_admin'
  })

  const setToken = (newToken: string) => {
    token.value = newToken
    localStorage.setItem('token', newToken)
  }

  const setUser = (newUser: User) => {
    user.value = newUser
  }

  const logout = () => {
    user.value = null
    token.value = null
    localStorage.removeItem('token')
  }

  const login = async (username: string, password: string) => {
    // 使用 Base64 编码密码后发送给后端
    const encryptedPassword = passwordUtils.encodePassword(password)
    const response = await authAPI.login({ username, password: encryptedPassword })
    const { token: newToken, user: newUser } = response.data
    setToken(newToken)
    setUser(newUser)
    return newUser
  }

  const fetchCurrentUser = async () => {
    if (!token.value) return null
    try {
      const response = await authAPI.getCurrentUser()
      user.value = response.data
      return user.value
    } catch (error) {
      logout()
      return null
    }
  }

  return {
    user,
    token,
    isAdmin,
    isSystemAdmin,
    isDomainAdmin,
    setToken,
    setUser,
    logout,
    login,
    fetchCurrentUser,
  }
})
