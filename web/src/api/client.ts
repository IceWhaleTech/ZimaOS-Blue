import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { getErrorMessage } from '@/utils/error'

// Detect if running in Tauri (v2 injects __TAURI_INTERNALS__, v1 injects __TAURI__)
const isTauri =
  typeof window !== 'undefined' &&
  ('__TAURI_INTERNALS__' in window || '__TAURI__' in window)

// Get server URL from Tauri (supports both HTTP and HTTPS)
async function getServerUrl(): Promise<string> {
  // Try Tauri v2 IPC first, then v1
  const invoke =
    window.__TAURI_INTERNALS__?.invoke ??
    (window as any).__TAURI__?.core?.invoke
  if (isTauri && invoke) {
    try {
      const url = await invoke('get_server_url')
      return url
    } catch (e) {
      console.warn('Failed to get server URL from Tauri, falling back to relative URLs', e)
    }
  }
  // When invoke is unavailable (e.g. on_page_load stub without IPC),
  // return empty string so fetches use relative URLs against the current origin.
  return ''
}

// Cache the server URL and initialization promise
let cachedServerUrl: string | null = null
let initPromise: Promise<void> | null = null

async function initializeBaseUrl(): Promise<void> {
  if (!isTauri) {
    return
  }
  if (cachedServerUrl === null) {
    cachedServerUrl = await getServerUrl()
    api.defaults.baseURL = `${cachedServerUrl}/api/v1`
    console.log('API baseURL initialized to:', api.defaults.baseURL)
  }
}

async function getBaseUrl(): Promise<string> {
  if (!isTauri) {
    return ''
  }
  // Ensure initialization is complete before returning
  if (!initPromise) {
    initPromise = initializeBaseUrl()
  }
  await initPromise
  return cachedServerUrl ?? ''
}

async function reacquirePreviewToken(): Promise<string | null> {
  try {
    const baseUrl = await getBaseUrl()
    const url = `${baseUrl}/api/v1/preview/token`
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    })
    if (response.ok) {
      const data = await response.json()
      if (data.token) {
        localStorage.setItem('preview_token', data.token)
        localStorage.setItem('token', data.token)
        return data.token
      }
    }
  } catch {
    // Preview token fetch failed
  }
  return null
}

// Use absolute URL in Tauri, relative URL in browser
// For Tauri, this will be initialized asynchronously before first request
const baseURL = isTauri ? '' : '/api/v1'

const api = axios.create({
  baseURL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Initialize baseURL for Tauri before any requests
if (isTauri) {
  initPromise = initializeBaseUrl()
}

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

/**
 * Shared token refresh/re-acquire logic for use outside the axios interceptor.
 * Returns the new token on success, or null (and redirects) on failure.
 */
export async function ensureFreshToken(): Promise<string | null> {
  // Preview mode: re-acquire preview token
  if (localStorage.getItem('preview_token')) {
    return reacquirePreviewToken()
  }

  // Normal mode: if already refreshing, wait for it
  if (isRefreshing) {
    return new Promise<string>((resolve) => {
      addRefreshSubscriber(resolve)
    })
  }

  isRefreshing = true
  const refreshTokenValue = localStorage.getItem('refresh_token')
  if (!refreshTokenValue) {
    isRefreshing = false
    clearAuthAndRedirect()
    return null
  }

  try {
    const baseUrl = await getBaseUrl()
    const url = `${baseUrl}/api/v1/auth/refresh`
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshTokenValue }),
    })
    if (!response.ok) {
      throw new Error('refresh failed')
    }
    const data = await response.json()
    localStorage.setItem('token', data.token)
    localStorage.setItem('refresh_token', data.refresh_token)
    isRefreshing = false
    onRefreshed(data.token)
    return data.token
  } catch {
    isRefreshing = false
    refreshSubscribers = []
    clearAuthAndRedirect()
    return null
  }
}

// Request interceptor
api.interceptors.request.use(
  async (config) => {
    // Ensure baseURL is initialized for Tauri before first request
    if (isTauri && initPromise) {
      await initPromise
    }

    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
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
      originalRequest._retry = true

      // Preview mode: re-acquire a preview token and retry
      const isPreview = !!localStorage.getItem('preview_token')
      if (isPreview) {
        const newToken = await reacquirePreviewToken()
        if (newToken) {
          originalRequest.headers.Authorization = `Bearer ${newToken}`
          return api(originalRequest)
        }
        // Preview token fetch failed — nothing more we can do
        return Promise.reject(error)
      }

      // Don't try to refresh the refresh request itself
      if (originalRequest.url?.includes('/auth/refresh')) {
        clearAuthAndRedirect()
        return Promise.reject(error)
      }

      // Reuse ensureFreshToken() which uses raw fetch (avoids recursive interceptor)
      const newToken = await ensureFreshToken()
      if (newToken) {
        originalRequest.headers.Authorization = `Bearer ${newToken}`
        return api(originalRequest)
      }
      return Promise.reject(error)
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

/** Authenticated fetch wrapper — injects Bearer token and retries on 401. */
export async function authFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers)
  const token = localStorage.getItem('token')
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const response = await fetch(input, { ...init, headers })

  if (response.status === 401) {
    const newToken = await ensureFreshToken()
    if (newToken) {
      headers.set('Authorization', `Bearer ${newToken}`)
      return fetch(input, { ...init, headers })
    }
  }

  return response
}

export default api
