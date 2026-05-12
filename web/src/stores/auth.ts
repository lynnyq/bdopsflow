import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { User } from '@/types'
import { authAPI } from '@/api'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem('token'))

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
    const response = await authAPI.login({ username, password })
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
    setToken,
    setUser,
    logout,
    login,
    fetchCurrentUser,
  }
})
