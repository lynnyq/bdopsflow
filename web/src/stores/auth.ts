import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User, Permission, DomainInfo } from '@/types'
import { authAPI } from '@/api'
import { switchDomain as switchDomainAPI } from '@/api/admin'
import { encryptPassword, encryptPasswordSSO, setPublicKey, setSSOPublicKey, getPublicKey, getSSOPublicKey } from '@/utils/password'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(sessionStorage.getItem('token'))
  const refreshToken = ref<string | null>(sessionStorage.getItem('refresh_token'))
  const permissions = ref<Permission[]>([])
  const domains = ref<DomainInfo[]>([])
  const currentDomainId = ref<number | null>(null)
  const roleCodes = ref<string[]>([])

  const isAdmin = computed(() => {
    return roleCodes.value.includes('system_admin') || roleCodes.value.includes('domain_admin')
  })

  const isSystemAdmin = computed(() => {
    return roleCodes.value.includes('system_admin')
  })

  const isDomainAdmin = computed(() => {
    if (isSystemAdmin.value) return true
    return roleCodes.value.includes('domain_admin')
  })

  const hasPermission = (resource: string, action: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some((p: Permission) => p.resource === resource && (p.action === action || p.action === 'manage'))
  }

  const hasAnyPermission = (resource: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some((p: Permission) => p.resource === resource)
  }

  const setToken = (newToken: string) => {
    token.value = newToken
    sessionStorage.setItem('token', newToken)
  }

  const setRefreshToken = (newRefreshToken: string) => {
    refreshToken.value = newRefreshToken
    sessionStorage.setItem('refresh_token', newRefreshToken)
  }

  const setUser = (newUser: User) => {
    user.value = newUser
  }

  const setPermissions = (newPermissions: Permission[]) => {
    permissions.value = newPermissions
  }

  const setDomains = (newDomains: DomainInfo[]) => {
    domains.value = newDomains
  }

  const setCurrentDomainId = (domainId: number) => {
    currentDomainId.value = domainId
  }

  const setRoleCodes = (codes: string[]) => {
    roleCodes.value = codes
  }

  const switchDomain = async (domainId: number) => {
    try {
      const response = await switchDomainAPI(domainId)
      const { token: newToken, refresh_token: newRefreshToken, permissions: newPermissions, current_domain_id, role_codes } = response.data
      setToken(newToken)
      if (newRefreshToken) {
        setRefreshToken(newRefreshToken)
      }
      setPermissions(newPermissions)
      setCurrentDomainId(current_domain_id)
      setRoleCodes(role_codes || [])
    } catch (error) {
      console.error('切换领域失败:', error)
      throw error
    }
  }

  const logout = () => {
    const userId = user.value?.id
    user.value = null
    token.value = null
    refreshToken.value = null
    permissions.value = []
    domains.value = []
    currentDomainId.value = null
    roleCodes.value = []
    sessionStorage.removeItem('token')
    sessionStorage.removeItem('refresh_token')
    if (userId) {
      localStorage.removeItem(`bdopsflow_sql_tabs_${userId}`)
    }
    localStorage.removeItem('bdopsflow_sql_tabs_anonymous')
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
    const { token: newToken, refresh_token: newRefreshToken, user: newUser, permissions: newPermissions, domains: newDomains, current_domain_id, role_codes } = response.data
    setToken(newToken)
    if (newRefreshToken) {
      setRefreshToken(newRefreshToken)
    }
    setUser(newUser)
    setPermissions(newPermissions)
    setDomains(newDomains)
    setCurrentDomainId(current_domain_id)
    setRoleCodes(role_codes || [])
    return newUser
  }

  const ssoLogin = async (username: string, password: string) => {
    await fetchPublicKey()
    if (!getSSOPublicKey()) {
      throw new Error("SSO公钥未加载，无法登录")
    }
    const encryptedPassword = encryptPasswordSSO(password)
    const response = await authAPI.ssoLogin({ username, password: encryptedPassword })
    const { token: newToken, refresh_token: newRefreshToken, user: newUser, permissions: newPermissions, domains: newDomains, current_domain_id, role_codes } = response.data
    setToken(newToken)
    if (newRefreshToken) {
      setRefreshToken(newRefreshToken)
    }
    setUser(newUser)
    setPermissions(newPermissions)
    setDomains(newDomains)
    setCurrentDomainId(current_domain_id)
    setRoleCodes(role_codes || [])
    return newUser
  }

  const fetchCurrentUser = async () => {
    if (!token.value) return null
    try {
      const response = await authAPI.getCurrentUser()
      const { user: currentUser, permissions: currentPermissions, domains: currentDomains, current_domain_id, role_codes } = response.data
      user.value = currentUser
      permissions.value = currentPermissions
      domains.value = currentDomains
      currentDomainId.value = current_domain_id
      roleCodes.value = role_codes || []
      await fetchPublicKey()
      return user.value
    } catch (error) {
      logout()
      if (!window.location.pathname.includes('/login')) {
        window.location.href = '/login'
      }
      return null
    }
  }

  return {
    user,
    token,
    refreshToken,
    permissions,
    domains,
    currentDomainId,
    roleCodes,
    ssoEnabled,
    isAdmin,
    isSystemAdmin,
    isDomainAdmin,
    hasPermission,
    hasAnyPermission,
    setToken,
    setRefreshToken,
    setUser,
    setPermissions,
    setDomains,
    setCurrentDomainId,
    setRoleCodes,
    switchDomain,
    logout,
    login,
    ssoLogin,
    fetchCurrentUser,
    fetchPublicKey,
  }
})
