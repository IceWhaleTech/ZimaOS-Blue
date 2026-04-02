import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { PagePermission } from '@/constants/pagePermissions'
import { reportStartupMark } from '@/utils/startupTrace'
import type { User, ApiKey, CreateApiKeyRequest } from '@/api/auth'
import {
  clearStoredAccessToken,
  clearStoredRefreshToken,
  getStoredAccessToken,
  getStoredRefreshToken,
  setStoredAccessToken,
  setStoredRefreshToken,
} from '@/utils/authStorage'

type AuthModule = typeof import('@/api/auth')
type UsersModule = typeof import('@/api/users')

let authModulePromise: Promise<AuthModule> | null = null
let usersModulePromise: Promise<UsersModule> | null = null

function loadAuthModule(): Promise<AuthModule> {
  if (!authModulePromise) {
    authModulePromise = import('@/api/auth')
  }
  return authModulePromise
}

function loadUsersModule(): Promise<UsersModule> {
  if (!usersModulePromise) {
    usersModulePromise = import('@/api/users')
  }
  return usersModulePromise
}

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | null>(null)
  const token = ref<string | null>(getStoredAccessToken())
  const refreshToken = ref<string | null>(getStoredRefreshToken())
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
    if (!permissionsLoaded.value && token.value) return true
    return perms.some((p) => permissions.value.includes(p))
  }

  function hasAllPermissions(perms: (PagePermission | string)[]): boolean {
    if (user.value?.role === 'admin') return true
    if (!permissionsLoaded.value && token.value) return true
    return perms.every((p) => permissions.value.includes(p))
  }

  // Actions
  async function login(username: string, password: string) {
    try {
      loading.value = true
      error.value = null

      const { authApi } = await loadAuthModule()
      const response = await authApi.login({ username, password })
      const data = response.data

      token.value = data.token
      refreshToken.value = data.refresh_token
      user.value = data.user

      setStoredAccessToken(data.token)
      setStoredRefreshToken(data.refresh_token)

      // Fetch permissions after login
      await fetchPermissions()

      return true
    } catch (e) {
      // Import getErrorMessage at the top if not already imported
      const axiosError = e as {
        response?: { data?: { error?: string; message?: string }; status?: number }
      }

      // Extract error message from response
      if (axiosError.response?.data?.message) {
        error.value = axiosError.response.data.message
      } else if (axiosError.response?.data?.error) {
        error.value = axiosError.response.data.error
      } else if (e instanceof Error) {
        error.value = e.message
      } else {
        error.value = 'Login failed'
      }

      return false
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      const { authApi } = await loadAuthModule()
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
    clearStoredAccessToken()
    clearStoredRefreshToken()
  }

  async function fetchUser() {
    if (!token.value) return

    try {
      loading.value = true
      error.value = null
      reportStartupMark('auth_fetch_user_start')
      const { authApi } = await loadAuthModule()
      // Fetch user info and permissions in parallel
      const [userResponse] = await Promise.all([authApi.me(), fetchPermissions()])
      user.value = userResponse.data
      reportStartupMark('auth_fetch_user_done')
    } catch (e) {
      // If unauthorized, clear auth
      if ((e as { response?: { status?: number } }).response?.status === 401) {
        clearAuth()
      }
      error.value = e instanceof Error ? e.message : 'Failed to fetch user'
      reportStartupMark('auth_fetch_user_error')
    } finally {
      loading.value = false
    }
  }

  async function fetchPermissions() {
    if (!token.value) return

    try {
      const { permissionsApi } = await loadUsersModule()
      const response = await permissionsApi.getMyPermissions()
      permissions.value = response.data.permissions ?? []
      permissionsLoaded.value = true
      reportStartupMark('auth_permissions_loaded')
    } catch {
      // Default to empty permissions on error, but mark as loaded
      permissions.value = []
      permissionsLoaded.value = true
      reportStartupMark('auth_permissions_failed')
    }
  }

  async function refreshAccessToken() {
    if (!refreshToken.value) return false

    try {
      const { authApi } = await loadAuthModule()
      const response = await authApi.refresh(refreshToken.value)
      token.value = response.data.token
      refreshToken.value = response.data.refresh_token
      setStoredAccessToken(response.data.token)
      setStoredRefreshToken(response.data.refresh_token)
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
      const { authApi } = await loadAuthModule()
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
      const { apiKeyApi } = await loadAuthModule()
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
      const { apiKeyApi } = await loadAuthModule()
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
      const { apiKeyApi } = await loadAuthModule()
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
