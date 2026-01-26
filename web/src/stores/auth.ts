import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, apiKeyApi } from '@/api/auth'
import type { User, ApiKey, CreateApiKeyRequest } from '@/api/auth'

const TOKEN_KEY = 'token'
const REFRESH_TOKEN_KEY = 'refresh_token'

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | null>(null)
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const refreshToken = ref<string | null>(localStorage.getItem(REFRESH_TOKEN_KEY))
  const apiKeys = ref<ApiKey[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const username = computed(() => user.value?.username || '')

  // Actions
  async function login(username: string, password: string) {
    try {
      loading.value = true
      error.value = null

      const response = await authApi.login({ username, password })
      const data = response.data

      token.value = data.token
      refreshToken.value = data.refresh_token
      user.value = data.user

      localStorage.setItem(TOKEN_KEY, data.token)
      localStorage.setItem(REFRESH_TOKEN_KEY, data.refresh_token)

      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Login failed'
      if ((e as { response?: { data?: { error?: string } } }).response?.data?.error) {
        error.value = (e as { response: { data: { error: string } } }).response.data.error
      }
      return false
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await authApi.logout()
    } catch {
      // Ignore logout errors
    } finally {
      clearAuth()
    }
  }

  function clearAuth() {
    token.value = null
    refreshToken.value = null
    user.value = null
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(REFRESH_TOKEN_KEY)
  }

  async function fetchUser() {
    if (!token.value) return

    try {
      loading.value = true
      error.value = null
      const response = await authApi.me()
      user.value = response.data
    } catch (e) {
      // If unauthorized, clear auth
      if ((e as { response?: { status?: number } }).response?.status === 401) {
        clearAuth()
      }
      error.value = e instanceof Error ? e.message : 'Failed to fetch user'
    } finally {
      loading.value = false
    }
  }

  async function refreshAccessToken() {
    if (!refreshToken.value) return false

    try {
      const response = await authApi.refresh()
      token.value = response.data.token
      localStorage.setItem(TOKEN_KEY, response.data.token)
      return true
    } catch {
      clearAuth()
      return false
    }
  }

  async function updateProfile(data: { email?: string; password?: string }) {
    try {
      loading.value = true
      error.value = null
      const response = await authApi.updateProfile(data)
      user.value = response.data
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to update profile'
      return false
    } finally {
      loading.value = false
    }
  }

  // API Keys
  async function fetchApiKeys() {
    try {
      loading.value = true
      error.value = null
      const response = await apiKeyApi.list()
      apiKeys.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch API keys'
    } finally {
      loading.value = false
    }
  }

  async function createApiKey(data: CreateApiKeyRequest) {
    try {
      loading.value = true
      error.value = null
      const response = await apiKeyApi.create(data)
      apiKeys.value = [...apiKeys.value, response.data.api_key]
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to create API key'
      return null
    } finally {
      loading.value = false
    }
  }

  async function deleteApiKey(id: string) {
    try {
      loading.value = true
      error.value = null
      await apiKeyApi.delete(id)
      apiKeys.value = apiKeys.value.filter((k) => k.id !== id)
      return true
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete API key'
      return false
    } finally {
      loading.value = false
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    user,
    token,
    apiKeys,
    loading,
    error,

    // Computed
    isAuthenticated,
    isAdmin,
    username,

    // Actions
    login,
    logout,
    clearAuth,
    fetchUser,
    refreshAccessToken,
    updateProfile,
    fetchApiKeys,
    createApiKey,
    deleteApiKey,
    clearError,
  }
})
