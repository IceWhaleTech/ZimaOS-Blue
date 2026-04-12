import { createRouter, createWebHashHistory, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { PagePermissions } from '@/constants/pagePermissions'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'
import { getCurrentAppLocation } from '@/utils/appLocation'
import { reportStartupMark } from '@/utils/startupTrace'
import {
  getOptimisticStartupPreviewCheckTimeout,
  shouldPrefetchPreviewModeOnRouterInit,
} from '@/utils/desktopStartup'
import {
  clearStoredAccessToken,
  clearStoredAuthSession,
  clearStoredPreviewToken,
  clearStoredRefreshToken,
  getStoredAccessToken,
  getStoredPreviewToken,
  hasStoredSessionHint,
  setStoredAccessToken,
  setStoredPreviewToken,
  syncAuthSessionStorage,
} from '@/utils/authStorage'

// Desktop detection: __BLUE_DESKTOP__ is injected by the Tauri on_page_load handler.
// In desktop mode, the page is loaded from http://localhost:{port} (same-origin as the
// Go server), so all API calls use relative URLs — no special URL construction needed.
const isDesktop = typeof window !== 'undefined' && !!(window as any).__BLUE_DESKTOP__
type PreviewModeCheckResult = {
  preview: boolean
  connectionError: boolean
}

// Preview mode state (cached to avoid repeated API calls)
let previewModeChecked = false
let isPreviewMode = false
let connectionFailed = false
let previewTokenFetched = false
let pendingCheck: Promise<PreviewModeCheckResult> | null = null

syncAuthSessionStorage()

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

async function checkPreviewMode(): Promise<PreviewModeCheckResult> {
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

async function doCheckPreviewMode(): Promise<PreviewModeCheckResult> {
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
  const existingToken = getStoredPreviewToken()
  if (existingToken) {
    setStoredAccessToken(existingToken)
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
        setStoredPreviewToken(data.token)
        setStoredAccessToken(data.token)
        previewTokenFetched = true
      }
    }
  } catch {
    console.error('Failed to fetch preview token')
  }
}

function isChatRouteLocation(path: string, name: unknown): boolean {
  return path === '/chat' || name === 'Chat'
}

function isLoginRouteLocation(path: string, name: unknown): boolean {
  return path === '/login' || name === 'Login'
}

function normalizeRoutePermissions(meta: {
  permission?: unknown
  permissions?: unknown
}): string[] {
  const permissions = new Set<string>()
  const singlePermission = String(meta.permission ?? '').trim()
  if (singlePermission) {
    permissions.add(singlePermission)
  }
  if (Array.isArray(meta.permissions)) {
    for (const permission of meta.permissions) {
      const normalized = String(permission ?? '').trim()
      if (normalized) {
        permissions.add(normalized)
      }
    }
  }
  return [...permissions]
}

function hasRequiredRoutePermissions(requiredPermissions: string[]): boolean {
  if (requiredPermissions.length === 0) return true
  const authStore = useAuthStore()
  if (requiredPermissions.length === 1) {
    return authStore.hasPermission(requiredPermissions[0]!)
  }
  return authStore.hasAnyPermission(requiredPermissions)
}

function shouldUseOptimisticStartupPreviewCheck(path: string, name: unknown): boolean {
  if (!isDesktop || previewModeChecked) return false
  if (isChatRouteLocation(path, name)) return true
  return isLoginRouteLocation(path, name) && !hasStoredSessionHint()
}

async function waitForPreviewCheckResult(
  checkPromise: Promise<PreviewModeCheckResult>,
  timeoutMs: number
): Promise<PreviewModeCheckResult | null> {
  let timeoutHandle: number | undefined
  try {
    return await Promise.race([
      checkPromise,
      new Promise<null>((resolve) => {
        timeoutHandle = window.setTimeout(() => resolve(null), timeoutMs)
      }),
    ])
  } finally {
    if (timeoutHandle !== undefined) {
      window.clearTimeout(timeoutHandle)
    }
  }
}

function getStartupPreviewCheckTimeout(path: string, name: unknown): number {
  if (!shouldUseOptimisticStartupPreviewCheck(path, name)) {
    return 0
  }

  return getOptimisticStartupPreviewCheckTimeout(hasStoredSessionHint())
}

function normalizeConnectionErrorFromPath(path: string): string {
  return path === '/login' ? '/' : path
}

async function applyDeferredPreviewModeResult(
  result: PreviewModeCheckResult,
  intendedPath: string
): Promise<void> {
  if (result.connectionError) {
    if (router.currentRoute.value.name === 'ConnectionError') return
    await router.replace({
      name: 'ConnectionError',
      query: { from: normalizeConnectionErrorFromPath(intendedPath) },
    })
    return
  }

  if (!result.preview) return

  const staleToken = getStoredAccessToken()
  const hasPreviewToken = !!getStoredPreviewToken()
  if (staleToken && !hasPreviewToken) {
    useAuthStore().clearAuth()
  }

  await usePreviewStore().initialize()
  if (router.currentRoute.value.name === 'Login') {
    await router.replace({ name: 'Home' })
  }
}

function deferPreviewModeResolution(
  intendedPath: string,
  checkPromise: Promise<PreviewModeCheckResult>
) {
  reportStartupMark('router_mode_check_deferred')
  void checkPromise
    .then(async (result) => {
      reportStartupMark('router_mode_check_deferred_done')
      await applyDeferredPreviewModeResult(result, intendedPath)
    })
    .catch(() => {})
}

// Reset preview mode status (call this after upgrade completes)
export function resetPreviewModeStatus(): void {
  previewModeChecked = false
  isPreviewMode = false
  connectionFailed = false
  previewTokenFetched = false
  clearStoredPreviewToken()
}

// Expose cached preview mode result so other modules (e.g. previewStore)
// can reuse it without making a duplicate API call.
export function getCachedPreviewMode(): { checked: boolean; preview: boolean } {
  return { checked: previewModeChecked, preview: isPreviewMode }
}

// Eagerly start the preview mode check when this module loads, unless desktop startup
// already has a stored session hint and the result is no longer on the critical path.
if (shouldPrefetchPreviewModeOnRouterInit(isDesktop, hasStoredSessionHint())) {
  reportStartupMark('router_mode_check_prefetch_start')
  checkPreviewMode().catch(() => {})
} else {
  reportStartupMark('router_mode_check_prefetch_skipped')
}

const importChatView = () => import('@/views/ChatView.vue')
const importLoginView = () => import('@/views/LoginView.vue')
const importConnectionErrorView = () => import('@/views/ConnectionErrorView.vue')
let chatViewPreloadPromise: ReturnType<typeof importChatView> | null = null
let loginViewPreloadPromise: ReturnType<typeof importLoginView> | null = null
let connectionErrorViewPreloadPromise: ReturnType<typeof importConnectionErrorView> | null = null

function preloadChatView() {
  if (!chatViewPreloadPromise) {
    reportStartupMark('chat_view_preload_start')
    chatViewPreloadPromise = importChatView()
      .then((module) => {
        reportStartupMark('chat_view_preload_done')
        return module
      })
      .catch((error) => {
        chatViewPreloadPromise = null
        reportStartupMark('chat_view_preload_error')
        throw error
      })
  }
  return chatViewPreloadPromise
}

function preloadLoginView() {
  if (!loginViewPreloadPromise) {
    loginViewPreloadPromise = importLoginView().catch((error) => {
      loginViewPreloadPromise = null
      throw error
    })
  }
  return loginViewPreloadPromise
}

function preloadConnectionErrorView() {
  if (!connectionErrorViewPreloadPromise) {
    connectionErrorViewPreloadPromise = importConnectionErrorView().catch((error) => {
      connectionErrorViewPreloadPromise = null
      throw error
    })
  }
  return connectionErrorViewPreloadPromise
}

function loadChatViewForRoute() {
  reportStartupMark('chat_view_route_import_start')
  return (chatViewPreloadPromise ?? importChatView())
    .then((module) => {
      reportStartupMark('chat_view_route_import_done')
      return module
    })
    .catch((error) => {
      reportStartupMark('chat_view_route_import_error')
      throw error
    })
}

function loadLoginViewForRoute() {
  return loginViewPreloadPromise ?? importLoginView()
}

function loadConnectionErrorViewForRoute() {
  return connectionErrorViewPreloadPromise ?? importConnectionErrorView()
}

function warmInitialStartupRoutes() {
  if (typeof window === 'undefined') return
  const { path } = getCurrentAppLocation()
  if (path === '/' || path === '/chat') {
    void preloadChatView().catch(() => {})
    if (!hasStoredSessionHint()) {
      void preloadLoginView().catch(() => {})
    }
    void preloadConnectionErrorView().catch(() => {})
    return
  }

  if (path === '/login') {
    void preloadLoginView().catch(() => {})
    void preloadConnectionErrorView().catch(() => {})
  }
}

warmInitialStartupRoutes()

// Clear all cached state and tokens (for debugging/cleanup)
export function clearAllState(): void {
  previewModeChecked = false
  isPreviewMode = false
  connectionFailed = false
  previewTokenFetched = false
  clearStoredAuthSession()
}

export const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/chat',
  },
  {
    path: '/login',
    name: 'Login',
    component: () => loadLoginViewForRoute(),
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
    component: () => loadConnectionErrorViewForRoute(),
    meta: { public: true, hideLayout: true },
  },
  {
    path: '/chat',
    name: 'Chat',
    component: () => loadChatViewForRoute(),
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
    path: '/channels',
    name: 'Channels',
    component: () => import('@/views/ChannelsView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.CHANNELS },
  },
  {
    path: '/operations/harness',
    name: 'HarnessGroups',
    component: () => import('@/views/HarnessGroupsView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/operations/harness/:id',
    name: 'HarnessGroupDetail',
    component: () => import('@/views/HarnessGroupDetailView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/operations/knowledge',
    name: 'Knowledge',
    redirect: (to) => ({
      name: 'Evolution',
      query: {
        pane: 'knowledge',
        ...to.query,
      },
    }),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/operations/evolution',
    name: 'Evolution',
    component: () => import('@/views/EvolutionView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
  },
  {
    path: '/operations',
    name: 'Operations',
    component: () => import('@/views/CronView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.AUTOMATION },
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
    redirect: { name: 'Settings', query: { tab: 'userdata' } },
  },
  {
    path: '/security',
    name: 'Security',
    component: () => import('@/views/SecurityView.vue'),
    meta: { requiresAuth: true, permission: PagePermissions.SECURITY },
  },
  {
    path: '/security/harness/:id',
    redirect: (to) => ({
      name: 'HarnessGroupDetail',
      params: { id: to.params.id },
    }),
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
    redirect: { name: 'Chat' },
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
  history:
    import.meta.env.VITE_MODULE_UI === '1'
      ? createWebHashHistory(import.meta.env.BASE_URL)
      : createWebHistory(import.meta.env.BASE_URL),
  routes,
})

router.afterEach(() => {
  reportStartupMark('router_after_each')
})

function hydrateAuthenticatedRouteInBackground(
  requiredPermissions: string[],
  requiresAdmin: unknown
) {
  const authStore = useAuthStore()
  if (authStore.user) return

  reportStartupMark('router_fetch_user_deferred')
  void authStore.fetchUser().then(() => {
    if (requiresAdmin && !authStore.isAdmin) {
      void router.replace({ name: 'Chat' })
      return
    }

    if (!hasRequiredRoutePermissions(requiredPermissions)) {
      void router.replace({ name: 'Chat' })
    }
  })
}

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
    reportStartupMark('router_guard_enter')
    if (isChatRouteLocation(to.path, to.name)) {
      reportStartupMark('router_chat_guard_enter')
    }

    // Extract access_token from URL query (e.g. QR code deep link)
    const urlToken = to.query.access_token as string | undefined
    if (urlToken) {
      setStoredAccessToken(urlToken)
      // Strip token from URL and continue to the clean path
      const { access_token: _, ...cleanQuery } = to.query
      next({ path: to.path, query: cleanQuery, replace: true })
      return
    }

    const token = getStoredAccessToken()
    let isAuthenticated = !!token
    const requiresAuth = to.meta.requiresAuth
    const requiresAdmin = to.meta.requiresAdmin
    const requiredPermissions = normalizeRoutePermissions(to.meta)

    // Allow connection error page without checks
    if (to.name === 'ConnectionError') {
      next()
      return
    }

    // Check preview mode (no users exist)
    let previewModeResult: PreviewModeCheckResult | null = null
    if (shouldUseOptimisticStartupPreviewCheck(to.path, to.name)) {
      const previewCheckPromise = checkPreviewMode()
      previewModeResult = await waitForPreviewCheckResult(
        previewCheckPromise,
        getStartupPreviewCheckTimeout(to.path, to.name)
      )

      if (!previewModeResult) {
        reportStartupMark('router_mode_checked_optimistic')
        deferPreviewModeResolution(to.fullPath, previewCheckPromise)
      }
    } else {
      previewModeResult = await checkPreviewMode()
    }

    if (previewModeResult) {
      const { preview: inPreviewMode, connectionError } = previewModeResult
      reportStartupMark('router_mode_checked')

      // If connection error (500 or network failure), redirect to error page
      if (connectionError) {
        void preloadConnectionErrorView().catch(() => {})
        next({
          name: 'ConnectionError',
          query: { from: normalizeConnectionErrorFromPath(to.fullPath) },
        })
        return
      }

      // In preview mode, allow access to most routes without authentication.
      // Clear any stale non-preview tokens so the UI correctly detects preview state.
      if (inPreviewMode) {
        if (isChatRouteLocation(to.path, to.name)) {
          void preloadChatView()
        }
        const staleToken = getStoredAccessToken()
        const hasPreviewToken = !!getStoredPreviewToken()
        if (staleToken && !hasPreviewToken) {
          // Stale token from a previous normal-mode session — wipe it
          clearStoredAccessToken()
          clearStoredRefreshToken()
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
    }

    if (previewModeResult && isChatRouteLocation(to.path, to.name)) {
      void preloadChatView()
    }

    // Normal mode: standard authentication flow
    const previewToken = getStoredPreviewToken()
    if (previewModeResult && previewToken) {
      clearStoredPreviewToken()
      clearStoredAccessToken()
      isAuthenticated = false
      if ((requiresAuth || requiredPermissions.length > 0) && to.name !== 'Login') {
        void preloadLoginView().catch(() => {})
        next({ name: 'Login', query: { redirect: to.fullPath } })
        return
      }
    }

    if (!previewModeResult && isChatRouteLocation(to.path, to.name)) {
      void preloadChatView()
    }

    // Redirect to login if auth required but not authenticated
    if ((requiresAuth || requiredPermissions.length > 0) && !isAuthenticated) {
      void preloadLoginView().catch(() => {})
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
      if (isChatRouteLocation(to.path, to.name)) {
        void preloadChatView()
      }
      const authStore = useAuthStore()
      if (!authStore.user) {
        if (!requiresAdmin) {
          hydrateAuthenticatedRouteInBackground(requiredPermissions, requiresAdmin)
          reportStartupMark('router_guard_ready')
          next()
          return
        }

        reportStartupMark('router_fetch_user_wait')
        await authStore.fetchUser()
        reportStartupMark('router_fetch_user_done')
      }

      // Check admin-only routes
      if (requiresAdmin && !authStore.isAdmin) {
        next({ name: 'Chat' })
        return
      }

      // Check page-level permissions
      if (!hasRequiredRoutePermissions(requiredPermissions)) {
        next({ name: 'Chat' })
        return
      }
    }

    reportStartupMark('router_guard_ready')
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
