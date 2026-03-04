import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { PagePermissions } from '@/api/users'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'

// Desktop detection: __BLUE_DESKTOP__ is injected by the Tauri on_page_load handler.
// In desktop mode, the page is loaded from http://localhost:{port} (same-origin as the
// Go server), so all API calls use relative URLs — no special URL construction needed.
const isDesktop =
  typeof window !== 'undefined' && !!(window as any).__BLUE_DESKTOP__

// Preview mode state (cached to avoid repeated API calls)
let previewModeChecked = false
let isPreviewMode = false
let connectionFailed = false
let previewTokenFetched = false
let pendingCheck: Promise<{ preview: boolean; connectionError: boolean }> | null = null

async function fetchSystemMode(): Promise<Response> {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 2000)
  try {
    const response = await fetch('/api/v1/system/mode', { signal: controller.signal })
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
  // In desktop mode, the Go server may still be starting on first launch.
  // Retry with backoff instead of failing immediately.
  const maxAttempts = isDesktop ? 5 : 1

  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    if (attempt > 0) {
      // Backoff: 500ms, 1000ms, 1500ms, 2000ms
      await new Promise((r) => setTimeout(r, attempt * 500))
    }

    try {
      const response = await fetchSystemMode()

      // Treat 500+ errors as connection/server errors
      if (response.status >= 500) {
        continue // retry in desktop mode
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
    } catch (err) {
      // In desktop mode, retry; in browser, fail immediately
      if (!isDesktop) break
    }
  }

  // All attempts exhausted
  previewModeChecked = true
  isPreviewMode = false
  connectionFailed = true
  return { preview: false, connectionError: true }
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

// Expose cached preview mode result so other modules (e.g. previewStore)
// can reuse it without making a duplicate API call.
export function getCachedPreviewMode(): { checked: boolean; preview: boolean } {
  return { checked: previewModeChecked, preview: isPreviewMode }
}

// Eagerly start the preview mode check when this module loads.
// By the time the router guard fires, the result is likely cached.
checkPreviewMode().catch(() => {})

// Preload the ChatView chunk in parallel with the preview mode check.
// This overlaps the network fetch so the chunk is ready when navigation completes.
const chatViewPreload = () => import('@/views/ChatView.vue')
chatViewPreload()

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
    redirect: '/cron',
  },
  {
    path: '/browser-automation',
    redirect: '/cron',
  },
  {
    path: '/auto-reply',
    redirect: '/cron',
  },
  {
    path: '/channels',
    name: 'Channels',
    component: () => import('@/views/ChannelsView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.CHANNELS },
  },
  {
    path: '/workflows',
    redirect: '/cron',
  },
  {
    path: '/webhooks',
    redirect: '/cron',
  },
  {
    path: '/cron',
    name: 'CronJobs',
    component: () => import('@/views/CronView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/automation',
    redirect: '/cron',
  },
  {
    path: '/audit',
    name: 'AuditLogs',
    component: () => import('@/views/AuditView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
  },
  {
    path: '/billing',
    name: 'Billing',
    component: () => import('@/views/BillingView.vue'),
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
    redirect: '/cron',
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

    // In preview mode, allow access to most routes without authentication.
    // Clear any stale non-preview tokens so the UI correctly detects preview state.
    if (inPreviewMode) {
      const staleToken = localStorage.getItem('token')
      const hasPreviewToken = !!localStorage.getItem('preview_token')
      if (staleToken && !hasPreviewToken) {
        // Stale token from a previous normal-mode session — wipe it
        localStorage.removeItem('token')
        localStorage.removeItem('refresh_token')
        // Also reset the reactive auth store so isAuthenticated becomes false
        const authStore = useAuthStore()
        authStore.clearAuth()
      }
      // Eagerly initialize preview store so sidebar/header can read isPreviewMode
      const previewStore = usePreviewStore()
      await previewStore.initialize()
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

    // Ensure user info and permissions are loaded for authenticated users.
    // On page refresh, token is restored from localStorage but user/permissions
    // are not — fetch them before rendering so sidebar and guards work correctly.
    if (isAuthenticated) {
      const authStore = useAuthStore()
      if (!authStore.user) {
        await authStore.fetchUser()
      }

      // Check admin-only routes
      if (requiresAdmin && !authStore.isAdmin) {
        next({ name: 'Chat' })
        return
      }

      // Check page-level permissions
      if (requiredPermission && !authStore.hasPermission(requiredPermission)) {
        next({ name: 'Chat' })
        return
      }
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
