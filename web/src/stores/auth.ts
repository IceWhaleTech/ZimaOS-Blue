import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, apiKeyApi } from '@/api/auth'
import { permissionsApi, type PagePermission } from '@/api/users'
import type { User, ApiKey, CreateApiKeyRequest } from '@/api/auth'

const TOKEN_KEY = 'token'
const REFRESH_TOKEN_KEY = 'refresh_token'

// Safe localStorage access for Safari compatibility
function getStorageItem(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    // Safari private mode or other restrictions
    return null
  }
}

function setStorageItem(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Safari private mode or other restrictions
    console.warn(`Failed to set localStorage item: ${key}`)
  }
}

function removeStorageItem(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    // Safari private mode or other restrictions
    console.warn(`Failed to remove localStorage item: ${key}`)
  }
}

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | null>(null)
  const token = ref<string | null>(getStorageItem(TOKEN_KEY))
  const refreshToken = ref<string | null>(getStorageItem(REFRESH_TOKEN_KEY))
  const permissions = ref<string[]>([])
  const permissionsLoaded = ref(false) // Track if permissions have been loaded
  const apiKeys = ref<ApiKey[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Computed
  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const username = computed(() => user.value?.username || '')

  // Permission check helper
  function hasPermission(permission: PagePermission | string): boolean {
    // Admin has all permissions
    if (user.value?.role === 'admin') return true
    // If permissions haven't been loaded yet but user is authenticated, allow access
    if (!permissionsLoaded.value && token.value) return true
    return permissions.value.includes(permission)
  }

  function hasAnyPermission(perms: (PagePermission | string)[]): boolean {
    if (user.value?.role === 'admin') return true
    return perms.some((p) => permissions.value.includes(p))
  }

  function hasAllPermissions(perms: (PagePermission | string)[]): boolean {
    if (user.value?.role === 'admin') return true
    return perms.every((p) => permissions.value.includes(p))
  }

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

      setStorageItem(TOKEN_KEY, data.token)
      setStorageItem(REFRESH_TOKEN_KEY, data.refresh_token)

      // Fetch permissions after login
      await fetchPermissions()

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
    permissions.value = []
    permissionsLoaded.value = false
    removeStorageItem(TOKEN_KEY)
    removeStorageItem(REFRESH_TOKEN_KEY)
  }

  async function fetchUser() {
    if (!token.value) return

    try {
      loading.value = true
      error.value = null
      const response = await authApi.me()
      user.value = response.data
      // Also fetch permissions
      await fetchPermissions()
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

  async function fetchPermissions() {
    if (!token.value) return

    try {
      const response = await permissionsApi.getMyPermissions()
      permissions.value = response.data.permissions
      permissionsLoaded.value = true
    } catch {
      // Default to empty permissions on error, but mark as loaded
      permissions.value = []
      permissionsLoaded.value = true
    }
  }

  async function refreshAccessToken() {
    if (!refreshToken.value) return false

    try {
      const response = await authApi.refresh(refreshToken.value)
      token.value = response.data.token
      refreshToken.value = response.data.refresh_token
      setStorageItem(TOKEN_KEY, response.data.token)
      setStorageItem(REFRESH_TOKEN_KEY, response.data.refresh_token)
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
    permissions,
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
    fetchPermissions,
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
    refreshAccessToken,
    updateProfile,
    fetchApiKeys,
    createApiKey,
    deleteApiKey,
    clearError,
  }
})
