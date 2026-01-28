import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
  },
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/SetupWizardView.vue'),
    meta: { public: true, hideLayout: true },
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
    path: '/chat',
    name: 'Chat',
    component: () => import('@/views/ChatView.vue'),
    meta: { requiresAuth: true, noPadding: true },
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/SettingsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/plugins',
    name: 'Plugins',
    component: () => import('@/views/PluginsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/ProfileView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/system',
    name: 'System',
    component: () => import('@/views/SystemView.vue'),
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
    meta: { requiresAuth: true },
  },
  {
    path: '/smart-home',
    name: 'SmartHome',
    component: () => import('@/views/HomeAssistantView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/browser-automation',
    name: 'BrowserAutomation',
    component: () => import('@/views/BrowserAutomationView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/auto-reply',
    name: 'AutoReply',
    component: () => import('@/views/AutoReplyView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/channels',
    name: 'Channels',
    component: () => import('@/views/ChannelsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/workflows',
    name: 'Workflows',
    component: () => import('@/views/WorkflowView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/webhooks',
    name: 'Webhooks',
    component: () => import('@/views/WebhookView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/cron',
    name: 'CronJobs',
    component: () => import('@/views/CronView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/automation',
    name: 'Automation',
    component: () => import('@/views/AutomationView.vue'),
    meta: { requiresAuth: true },
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
    path: '/tools',
    name: 'ToolStore',
    component: () => import('@/views/ToolStoreView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/security',
    name: 'Security',
    component: () => import('@/views/SecurityView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/sandbox',
    name: 'Sandbox',
    component: () => import('@/views/SandboxView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/a2ui',
    name: 'A2UI',
    component: () => import('@/views/A2UIView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/tenants',
    name: 'Tenants',
    component: () => import('@/views/TenantsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/tenants/:id',
    name: 'TenantDetail',
    component: () => import('@/views/TenantDetailView.vue'),
    meta: { requiresAuth: true },
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

// Check if setup is complete
let setupChecked = false
let setupComplete = false

async function checkSetupStatus(): Promise<boolean> {
  if (setupChecked) return setupComplete

  try {
    const response = await fetch('/api/setup/status')
    if (!response.ok) {
      // API error - redirect to setup for safety
      setupChecked = true
      setupComplete = false
      return false
    }
    const data = await response.json()
    setupComplete = data.completed
    setupChecked = true
    return setupComplete
  } catch {
    // Network error - redirect to setup for safety
    setupChecked = true
    setupComplete = false
    return false
  }
}

// Reset setup status (call this after setup completes)
export function resetSetupStatus(): void {
  setupChecked = false
  setupComplete = false
}

// Navigation guard for authentication and setup
router.beforeEach(async (to, _from, next) => {
  const token = localStorage.getItem('token')
  const isAuthenticated = !!token
  const requiresAuth = to.meta.requiresAuth
  const isPublic = to.meta.public

  // Check setup status for non-setup routes
  if (to.name !== 'Setup') {
    const isSetupComplete = await checkSetupStatus()
    if (!isSetupComplete) {
      next({ name: 'Setup' })
      return
    }
  }

  if (requiresAuth && !isAuthenticated) {
    // Redirect to login with return URL
    next({ name: 'Login', query: { redirect: to.fullPath } })
  } else if (to.name === 'Login' && isAuthenticated) {
    // Already logged in, redirect to chat
    next({ name: 'Chat' })
  } else if (isPublic || isAuthenticated || !requiresAuth) {
    next()
  } else {
    next()
  }
})

export default router
