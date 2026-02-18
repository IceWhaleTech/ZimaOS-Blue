import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { PagePermissions } from '@/api/users'
import { useAuthStore } from '@/stores/auth'

// Detect if running in Tauri
const isTauri = typeof window !== 'undefined' && '__TAURI__' in window

// Preview mode state (cached to avoid repeated API calls)
let previewModeChecked = false
let isPreviewMode = false
let connectionFailed = false
let previewTokenFetched = false
let pendingCheck: Promise<{ preview: boolean; connectionError: boolean }> | null = null

async function fetchSystemMode(): Promise<Response> {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 5000)
  const url = isTauri ? 'http://localhost/api/v1/system/mode' : '/api/v1/system/mode'
  try {
    const response = await fetch(url, { signal: controller.signal })
    clearTimeout(timeoutId)
    return response
  } catch (err) {
    clearTimeout(timeoutId)
    throw err
  }
}

async function checkPreviewMode(): Promise<{ preview: boolean; connectionError: boolean }> {
  if (previewModeChecked) return { preview: isPreviewMode, connectionError: connectionFailed }

  // Deduplicate concurrent calls — only one inflight request at a time
  if (pendingCheck) return pendingCheck

  pendingCheck = doCheckPreviewMode()
  try {
    return await pendingCheck
  } finally {
    pendingCheck = null
  }
}

async function doCheckPreviewMode(): Promise<{ preview: boolean; connectionError: boolean }> {
  try {
    const response = await fetchSystemMode()

    // Treat 500+ errors as connection/server errors
    if (response.status >= 500) {
      previewModeChecked = true
      isPreviewMode = false
      connectionFailed = true
      return { preview: false, connectionError: true }
    }
    if (!response.ok) {
      previewModeChecked = true
      isPreviewMode = false
      connectionFailed = false
      return { preview: false, connectionError: false }
    }
    const data = await response.json()
    isPreviewMode = data.mode === 'preview'
    previewModeChecked = true
    connectionFailed = false

    // If in preview mode, fetch a token
    if (isPreviewMode && !previewTokenFetched) {
      await fetchPreviewToken()
    }

    return { preview: isPreviewMode, connectionError: false }
  } catch {
    previewModeChecked = true
    isPreviewMode = false
    connectionFailed = true
    return { preview: false, connectionError: true }
  }
}

async function fetchPreviewToken(): Promise<void> {
  // Check if we already have a token
  const existingToken = localStorage.getItem('preview_token')
  if (existingToken) {
    localStorage.setItem('token', existingToken)
    previewTokenFetched = true
    return
  }

  try {
    // Use absolute URL in Tauri, relative URL in browser
    const url = isTauri ? 'http://localhost/api/v1/preview/token' : '/api/v1/preview/token'
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
        previewTokenFetched = true
      }
    }
  } catch {
    console.error('Failed to fetch preview token')
  }
}

// Reset preview mode status (call this after upgrade completes)
export function resetPreviewModeStatus(): void {
  previewModeChecked = false
  isPreviewMode = false
  connectionFailed = false
  previewTokenFetched = false
  localStorage.removeItem('preview_token')
}

// Clear all cached state and tokens (for debugging/cleanup)
export function clearAllState(): void {
  previewModeChecked = false
  isPreviewMode = false
  connectionFailed = false
  previewTokenFetched = false
  localStorage.removeItem('preview_token')
  localStorage.removeItem('token')
  localStorage.removeItem('refresh_token')
}

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/chat',
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/LoginView.vue'),
    meta: { public: true, hideLayout: true },
  },
  {
    path: '/auth/callback/:provider',
    name: 'AuthCallback',
    component: () => import('@/views/AuthCallbackView.vue'),
    meta: { public: true, hideLayout: true },
  },
  {
    path: '/connection-error',
    name: 'ConnectionError',
    component: () => import('@/views/ConnectionErrorView.vue'),
    meta: { public: true, hideLayout: true },
  },
  {
    path: '/chat',
    name: 'Chat',
    component: () => import('@/views/ChatView.vue'),
    meta: { requiresAuth: true, noPadding: true, permission: PagePermissions.CHAT },
  },
  {
    path: '/home',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.HOME },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/SettingsView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.SETTINGS },
  },
  {
    path: '/plugins',
    name: 'Plugins',
    component: () => import('@/views/PluginsView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.PLUGINS },
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/ProfileView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/admin/auth-providers',
    name: 'AuthProviders',
    component: () => import('@/views/AuthProvidersView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/voice',
    name: 'VoiceChat',
    component: () => import('@/views/VoiceChatView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.CHAT },
  },
  {
    path: '/smart-home',
    name: 'SmartHome',
    component: () => import('@/views/HomeAssistantView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/browser-automation',
    name: 'BrowserAutomation',
    component: () => import('@/views/BrowserAutomationView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/auto-reply',
    name: 'AutoReply',
    component: () => import('@/views/AutoReplyView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/channels',
    name: 'Channels',
    component: () => import('@/views/ChannelsView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.CHANNELS },
  },
  {
    path: '/workflows',
    name: 'Workflows',
    component: () => import('@/views/WorkflowView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/webhooks',
    name: 'Webhooks',
    component: () => import('@/views/WebhookView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/cron',
    name: 'CronJobs',
    component: () => import('@/views/CronView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/automation',
    name: 'Automation',
    component: () => import('@/views/AutomationView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/audit',
    name: 'AuditLogs',
    component: () => import('@/views/AuditView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/backup',
    name: 'Backup',
    component: () => import('@/views/BackupView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/security',
    name: 'Security',
    component: () => import('@/views/SecurityView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.SECURITY },
  },
  {
    path: '/sandbox',
    name: 'Sandbox',
    component: () => import('@/views/SandboxView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/tenants',
    name: 'Tenants',
    component: () => import('@/views/TenantsView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/tenants/:id',
    name: 'TenantDetail',
    component: () => import('@/views/TenantDetailView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/form-filler',
    name: 'FormFiller',
    component: () => import('@/views/FormFillerView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/users',
    name: 'Users',
    component: () => import('@/views/UsersView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Navigation guard for authentication, preview mode, and permissions
let isNavigating = false
router.beforeEach(async (to, from, next) => {
  // Re-entrant navigation (triggered by next() redirects) —
  // still check preview mode (cached/deduped, no extra requests)
  // but skip redirect logic to avoid infinite loops.
  if (isNavigating) {
    // Even in re-entrant calls, ensure preview check has completed
    // so token is available. checkPreviewMode is deduped and cached.
    await checkPreviewMode()
    next()
    return
  }

  isNavigating = true
  try {
    // Extract access_token from URL query (e.g. QR code deep link)
    const urlToken = to.query.access_token as string | undefined
    if (urlToken) {
      localStorage.setItem('token', urlToken)
      // Strip token from URL and continue to the clean path
      const { access_token: _, ...cleanQuery } = to.query
      next({ path: to.path, query: cleanQuery, replace: true })
      return
    }

    const token = localStorage.getItem('token')
    const isAuthenticated = !!token
    const requiresAuth = to.meta.requiresAuth
    const requiresAdmin = to.meta.requiresAdmin
    const requiredPermission = to.meta.permission as string | undefined
    const isPublic = to.meta.public

    // Allow connection error page without checks
    if (to.name === 'ConnectionError') {
      next()
      return
    }

    // Check preview mode (no users exist)
    const { preview: inPreviewMode, connectionError } = await checkPreviewMode()

    // If connection error (500 or network failure), redirect to error page
    if (connectionError) {
      // Avoid redirect loop: don't pass /login as from, use the original intended destination
      const fromPath = to.fullPath === '/login' ? '/' : to.fullPath
      next({ name: 'ConnectionError', query: { from: fromPath } })
      return
    }

    // In preview mode, allow access to most routes without authentication
    if (inPreviewMode) {
      if (to.name === 'Login') {
        next({ name: 'Home' })
        return
      }
      next()
      return
    }

    // Normal mode: standard authentication flow
    const previewToken = localStorage.getItem('preview_token')
    if (previewToken && !inPreviewMode) {
      localStorage.removeItem('preview_token')
      localStorage.removeItem('token')
      if ((requiresAuth || requiredPermission) && to.name !== 'Login') {
        next({ name: 'Login', query: { redirect: to.fullPath } })
        return
      }
    }

    // Redirect to login if auth required but not authenticated
    if ((requiresAuth || requiredPermission) && !isAuthenticated) {
      next({ name: 'Login', query: { redirect: to.fullPath } })
      return
    }

    // Redirect authenticated users away from login
    if (to.name === 'Login' && isAuthenticated) {
      const redirect = to.query.redirect as string
      if (redirect && redirect !== '/login' && from.name !== 'Login') {
        next(redirect)
      } else {
        next({ name: 'Chat' })
      }
      return
    }

    next()
  } finally {
    isNavigating = false
  }
})

// Handle auth:unauthorized event for Tauri
if (typeof window !== 'undefined') {
  window.addEventListener('auth:unauthorized', () => {
    router.push({ name: 'Login' })
  })
}

export default router
