import axios, { type AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { getErrorMessage } from '@/utils/error'

// Desktop detection: __BLUE_DESKTOP__ is injected by the Tauri on_page_load handler.
// In both browser and desktop modes, the page is same-origin with the Go server,
// so all API calls use relative URLs — no special URL construction needed.
const isDesktop = typeof window !== 'undefined' && !!(window as any).__BLUE_DESKTOP__

function decodeJwtPayload(token: string): Record<string, unknown> | null {
  const parts = token.split('.')
  if (parts.length !== 3 || !parts[1]) return null

  try {
    const normalized = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
    return JSON.parse(atob(padded)) as Record<string, unknown>
  } catch {
    return null
  }
}

export function isAccessTokenExpiredOrExpiring(token: string, skewMs = 30_000): boolean {
  const payload = decodeJwtPayload(token)
  const exp = payload?.exp
  if (typeof exp !== 'number') return false
  return exp * 1000 <= Date.now() + skewMs
}

function shouldSkipProactiveRefresh(url?: string): boolean {
  if (!url) return false
  return (
    url.includes('/auth/login') ||
    url.includes('/auth/refresh') ||
    url.includes('/preview/token')
  )
}

async function reacquirePreviewToken(): Promise<string | null> {
  try {
    const response = await fetch('/api/v1/preview/token', {
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

const api = axios.create({
  baseURL: '/api/v1',
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
  if (isDesktop) {
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
    const response = await fetch('/api/v1/auth/refresh', {
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

async function getRequestToken(requestUrl?: string): Promise<string | null> {
  let token = localStorage.getItem('token')
  if (!token) return null

  const isPreview = !!localStorage.getItem('preview_token')
  if (
    !isPreview &&
    !shouldSkipProactiveRefresh(requestUrl) &&
    isAccessTokenExpiredOrExpiring(token)
  ) {
    token = await ensureFreshToken()
  }

  return token
}

// Request interceptor
api.interceptors.request.use(
  async (config) => {
    const token = await getRequestToken(config.url)
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
      ;(error as AxiosError & { translatedMessage?: string }).translatedMessage =
        getErrorMessage(error)
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
