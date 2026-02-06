import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { PagePermissions } from '@/api/users'
import { useAuthStore } from '@/stores/auth'

// Preview mode state (cached to avoid repeated API calls)
let previewModeChecked = false
let isPreviewMode = false
let connectionFailed = false
let previewTokenFetched = false

async function checkPreviewMode(): Promise<{ preview: boolean; connectionError: boolean }> {
  if (previewModeChecked) return { preview: isPreviewMode, connectionError: connectionFailed }

  try {
    // Add timeout to prevent indefinite hanging
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 5000) // 5 second timeout

    const response = await fetch('/api/v1/system/mode', {
      signal: controller.signal,
    })
    clearTimeout(timeoutId)

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
  } catch (_error) {
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
    redirect: '/home',
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
router.beforeEach(async (to, _from, next) => {
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
    // Pass the original route so we can return after connection is restored
    next({ name: 'ConnectionError', query: { from: to.fullPath } })
    return
  }

  // In preview mode, allow access to most routes without authentication
  if (inPreviewMode) {
    // In preview mode, users are treated as admin, so allow admin routes
    // Allow all other routes in preview mode (no auth required)
    if (to.name === 'Login') {
      // Redirect login to chat in preview mode
      next({ name: 'Chat' })
      return
    }

    next()
    return
  }

  // Normal mode (users exist): standard authentication flow
  // If we have a preview token but system is in normal mode, clear it and require login
  const previewToken = localStorage.getItem('preview_token')
  if (previewToken && !inPreviewMode) {
    // System has been upgraded from preview to normal mode
    // Clear preview token and require proper authentication
    localStorage.removeItem('preview_token')
    localStorage.removeItem('token')

    // If trying to access a protected route, redirect to login
    if ((requiresAuth || requiredPermission) && to.name !== 'Login') {
      next({ name: 'Login', query: { redirect: to.fullPath } })
      return
    }
  }

  // Routes with required permissions implicitly require authentication
  if ((requiresAuth || requiredPermission) && !isAuthenticated) {
    // Redirect to login with return URL
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  if (to.name === 'Login' && isAuthenticated) {
    // Already logged in, redirect to the requested page or stay on previous page
    const redirect = to.query.redirect as string
    if (redirect) {
      next(redirect)
    } else if (_from.name && _from.name !== 'Login') {
      // Stay on the page they came from
      next(_from)
    } else {
      next({ name: 'Home' })
    }
    return
  }

  // Check admin requirement
  if (requiresAdmin && isAuthenticated) {
    const authStore = useAuthStore()
    if (!authStore.isAdmin) {
      // Not admin, redirect to chat
      next({ name: 'Chat' })
      return
    }
  }

  // Check page permission requirement
  if (requiredPermission && isAuthenticated) {
    // Skip permission check if already going to Chat (prevent infinite loop)
    if (to.name === 'Chat') {
      next()
      return
    }

    const authStore = useAuthStore()

    // Admin has all permissions
    if (!authStore.isAdmin && !authStore.hasPermission(requiredPermission)) {
      // No permission, redirect to chat
      next({ name: 'Chat' })
      return
    }
  }

  if (isPublic || isAuthenticated || !requiresAuth) {
    next()
  } else {
    next()
  }
})

export default router
