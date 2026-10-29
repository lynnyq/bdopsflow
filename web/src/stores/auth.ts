/**
 * @file auth.ts - Pinia 认证状态管理 store
 * 负责用户登录、登出、Token 刷新、权限校验、领域切换等核心认证逻辑。
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User, Permission, DomainInfo } from '@/types'
import { authAPI } from '@/api'
import { switchDomain as switchDomainAPI } from '@/api/admin'
import { encryptPassword, encryptPasswordSSO, setPublicKey, setSSOPublicKey, getPublicKey, getSSOPublicKey } from '@/utils/password'

/**
 * 认证状态管理 store
 * 采用 Setup Store 写法，集中管理用户信息、Token、权限、领域、角色等认证相关状态，
 * 并提供登录、登出、领域切换、权限校验等业务方法。
 */
export const useAuthStore = defineStore('auth', () => {
  // 当前登录用户信息
  const user = ref<User | null>(null)
  // 访问令牌，持久化在 sessionStorage
  const token = ref<string | null>(sessionStorage.getItem('token'))
  // 刷新令牌，持久化在 sessionStorage
  const refreshToken = ref<string | null>(sessionStorage.getItem('refresh_token'))
  // 当前用户拥有的权限列表
  const permissions = ref<Permission[]>([])
  // 当前用户可访问的领域列表
  const domains = ref<DomainInfo[]>([])
  // 当前选中的领域 ID
  const currentDomainId = ref<number | null>(null)
  // 当前用户的角色编码列表（如 system_admin, domain_admin）
  const roleCodes = ref<string[]>([])

  /** 判断当前用户是否为管理员（系统管理员或领域管理员） */
  const isAdmin = computed(() => {
    return roleCodes.value.includes('system_admin') || roleCodes.value.includes('domain_admin')
  })

  /** 判断当前用户是否为系统级管理员 */
  const isSystemAdmin = computed(() => {
    return roleCodes.value.includes('system_admin')
  })

  /** 判断当前用户是否为领域级管理员（系统管理员也视为领域管理员） */
  const isDomainAdmin = computed(() => {
    if (isSystemAdmin.value) return true
    return roleCodes.value.includes('domain_admin')
  })

  /**
   * 检查当前用户是否拥有指定资源上的指定操作权限。
   * 系统管理员自动放行。支持 manage 通配所有操作。
   * @param resource - 资源标识符，如 'task', 'datasource'
   * @param action - 操作标识符，如 'create', 'update', 'manage'
   * @returns 是否有权限
   */
  const hasPermission = (resource: string, action: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some((p: Permission) => p.resource === resource && (p.action === action || p.action === 'manage'))
  }

  /**
   * 检查当前用户是否拥有指定资源上的任意权限。
   * 系统管理员自动放行。
   * @param resource - 资源标识符
   * @returns 是否拥有该资源的任意权限
   */
  const hasAnyPermission = (resource: string): boolean => {
    if (isSystemAdmin.value) return true
    return permissions.value.some((p: Permission) => p.resource === resource)
  }

  /**
   * 设置访问令牌，同时同步到 sessionStorage。
   * @param newToken - 新的访问令牌
   */
  const setToken = (newToken: string) => {
    token.value = newToken
    sessionStorage.setItem('token', newToken)
  }

  /**
   * 设置刷新令牌，同时同步到 sessionStorage。
   * @param newRefreshToken - 新的刷新令牌
   */
  const setRefreshToken = (newRefreshToken: string) => {
    refreshToken.value = newRefreshToken
    sessionStorage.setItem('refresh_token', newRefreshToken)
  }

  /**
   * 设置当前登录用户信息。
   * @param newUser - 用户对象
   */
  const setUser = (newUser: User) => {
    user.value = newUser
  }

  /**
   * 设置当前用户的权限列表。
   * @param newPermissions - 权限数组
   */
  const setPermissions = (newPermissions: Permission[]) => {
    permissions.value = newPermissions
  }

  /**
   * 设置当前用户可访问的领域列表。
   * @param newDomains - 领域信息数组
   */
  const setDomains = (newDomains: DomainInfo[]) => {
    domains.value = newDomains
  }

  /**
   * 设置当前选中的领域 ID。
   * @param domainId - 领域 ID
   */
  const setCurrentDomainId = (domainId: number) => {
    currentDomainId.value = domainId
  }

  /**
   * 设置当前用户的角色编码列表。
   * @param codes - 角色编码数组
   */
  const setRoleCodes = (codes: string[]) => {
    roleCodes.value = codes
  }

  /**
   * 切换当前领域。调用后端接口换取新 Token 与权限集，失败时向上抛异常。
   * @param domainId - 目标领域 ID
   */
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

  /**
   * 清除所有认证状态，包括内存、sessionStorage 中存储的 token，
   * 以及本地保存的 SQL 标签页等用户偏好数据。
   */
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

  // SSO 登录是否启用
  const ssoEnabled = ref(false)

  /**
   * 从后端获取 RSA 公钥和 SSO 配置信息，缓存到模块内变量。
   * 已加载时直接返回，不重复请求。
   */
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

  /**
   * 用户名密码登录。会先获取 RSA 公钥对密码加密，再调用后端登录接口，
   * 成功后写入 token、刷新令牌、用户信息、权限和领域数据。
   * @param username - 用户名
   * @param password - 明文密码
   * @returns 登录成功后的用户对象
   */
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

  /**
   * SSO 方式登录。使用 SSO 专用 RSA 公钥加密密码后调用后端 SSO 接口。
   * SSO 公钥未加载时会抛出异常。
   * @param username - 用户名
   * @param password - 明文密码
   * @returns 登录成功后的用户对象
   * @throws 当 SSO 公钥未加载时抛出 Error
   */
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

  /**
   * 根据本地 Token 从后端拉取当前登录用户完整信息（包含权限、领域等），
   * 失败时自动执行登出并跳转至登录页。
   * @returns 当前用户对象；无 Token 或请求失败时返回 null
   */
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
