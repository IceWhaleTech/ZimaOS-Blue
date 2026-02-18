import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { getErrorMessage } from '@/utils/error'

// Detect if running in Tauri
const isTauri = typeof window !== 'undefined' && '__TAURI__' in window

// Use absolute URL in Tauri, relative URL in browser
const baseURL = isTauri ? 'http://localhost/api/v1' : '/api/v1'

const api = axios.create({
  baseURL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Token refresh state — shared across concurrent 401s
let isRefreshing = false
let refreshSubscribers: ((token: string) => void)[] = []

function onRefreshed(token: string) {
  refreshSubscribers.forEach((cb) => cb(token))
  refreshSubscribers = []
}

function addRefreshSubscriber(cb: (token: string) => void) {
  refreshSubscribers.push(cb)
}

function clearAuthAndRedirect() {
  localStorage.removeItem('token')
  localStorage.removeItem('refresh_token')
  if (isTauri) {
    window.dispatchEvent(new CustomEvent('auth:unauthorized'))
  } else if (!window.location.pathname.startsWith('/login')) {
    window.location.href = '/login'
  }
}

// Request interceptor
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }

    if (isTauri && config.url) {
      if (config.url.startsWith('https://localhost') || config.url.startsWith('https://127.0.0.1')) {
        config.url = config.url.replace('https://', 'http://')
      }
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }

    if (error.response?.status === 401 && originalRequest && !originalRequest._retry) {
      // Skip refresh for preview mode
      const isPreview = !!localStorage.getItem('preview_token')
      if (isPreview) {
        return Promise.reject(error)
      }

      // Don't try to refresh the refresh request itself
      if (originalRequest.url?.includes('/auth/refresh')) {
        clearAuthAndRedirect()
        return Promise.reject(error)
      }

      originalRequest._retry = true

      if (isRefreshing) {
        // Another request is already refreshing — queue this one
        return new Promise((resolve) => {
          addRefreshSubscriber((newToken: string) => {
            originalRequest.headers.Authorization = `Bearer ${newToken}`
            resolve(api(originalRequest))
          })
        })
      }

      isRefreshing = true
      const refreshTokenValue = localStorage.getItem('refresh_token')

      if (!refreshTokenValue) {
        isRefreshing = false
        clearAuthAndRedirect()
        return Promise.reject(error)
      }

      try {
        const response = await api.post('/auth/refresh', { refresh_token: refreshTokenValue })
        const { token, refresh_token: newRefreshToken } = response.data

        localStorage.setItem('token', token)
        localStorage.setItem('refresh_token', newRefreshToken)

        isRefreshing = false
        onRefreshed(token)

        // Retry original request with new token
        originalRequest.headers.Authorization = `Bearer ${token}`
        return api(originalRequest)
      } catch {
        isRefreshing = false
        refreshSubscribers = []
        clearAuthAndRedirect()
        return Promise.reject(error)
      }
    }

    // Enhance error with translated message
    if (error.response) {
      ;(error as AxiosError & { translatedMessage?: string }).translatedMessage = getErrorMessage(error)
    }

    return Promise.reject(error)
  }
)

/** Returns auth headers for use with native fetch/EventSource. */
export function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('token')
  return token ? { Authorization: `Bearer ${token}` } : {}
}

/** Authenticated fetch wrapper — injects Bearer token automatically. */
export function authFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers)
  const token = localStorage.getItem('token')
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  return fetch(input, { ...init, headers })
}

export default api
