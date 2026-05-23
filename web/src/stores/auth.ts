import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User, UserPermission } from '@/types'
import { authAPI } from '@/api'
import { encryptPassword, encryptPasswordSSO, setPublicKey, setSSOPublicKey, getPublicKey, getSSOPublicKey } from '@/utils/password'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem('token'))
  const permissions = ref<UserPermission[]>([])

  const isAdmin = computed(() => {
    return user.value?.role === 'admin' || user.value?.role === 'system_admin'
  })

  const isSystemAdmin = computed(() => {
    return user.value?.role === 'system_admin' || user.value?.role === 'admin'
  })

  const isDomainAdmin = computed(() => {
    return user.value?.role === 'domain_admin'
  })

  const hasPermission = (resource: string, action: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some(p => p.resource === resource && (p.action === action || p.action === 'manage'))
  }

  const hasAnyPermission = (resource: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some(p => p.resource === resource)
  }

  const hasMenuPermission = (menuAction: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some(p => p.resource === 'menu' && (p.action === menuAction || p.action === 'manage'))
  }

  const setToken = (newToken: string) => {
    token.value = newToken
    localStorage.setItem('token', newToken)
  }

  const setUser = (newUser: User) => {
    user.value = newUser
    permissions.value = newUser.permissions || []
  }

  const logout = () => {
    user.value = null
    token.value = null
    permissions.value = []
    localStorage.removeItem('token')
  }

  const ssoEnabled = ref(false)

  const fetchPublicKey = async () => {
    if (getPublicKey()) return
    try {
      const response = await authAPI.getPublicKey()
      setPublicKey(response.data.public_key)
      ssoEnabled.value = response.data.sso_enabled || false
      if (response.data.sso_public_key) {
        setSSOPublicKey(response.data.sso_public_key)
      }
    } catch (error) {
      console.error('获取RSA公钥失败:', error)
    }
  }

  const login = async (username: string, password: string) => {
    await fetchPublicKey()
    const encryptedPassword = encryptPassword(password)
    const response = await authAPI.login({ username, password: encryptedPassword })
    const { token: newToken, user: newUser } = response.data
    setToken(newToken)
    setUser(newUser)
    return newUser
  }

  const ssoLogin = async (username: string, password: string) => {
    await fetchPublicKey()
    if (!getSSOPublicKey()) {
      throw new Error("SSO公钥未加载，无法登录")
    }
    const encryptedPassword = encryptPasswordSSO(password)
    const response = await authAPI.ssoLogin({ username, password: encryptedPassword })
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
      permissions.value = response.data.permissions || []
      await fetchPublicKey()
      return user.value
    } catch (error) {
      logout()
      return null
    }
  }

  return {
    user,
    token,
    permissions,
    ssoEnabled,
    isAdmin,
    isSystemAdmin,
    isDomainAdmin,
    hasPermission,
    hasAnyPermission,
    hasMenuPermission,
    setToken,
    setUser,
    logout,
    login,
    ssoLogin,
    fetchCurrentUser,
    fetchPublicKey,
  }
})
