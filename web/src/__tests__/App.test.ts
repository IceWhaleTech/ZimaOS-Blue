import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import App from '@/App.vue'
import { prefetchCriticalRoutes } from '@/utils/prefetch'

const { authStore, previewStore, startupMarks } = vi.hoisted(() => ({
  authStore: {
    isAuthenticated: false,
    hasPermission: vi.fn(() => false),
  },
  previewStore: {
    isPreviewMode: false,
    initialize: vi.fn(async () => {}),
  },
  startupMarks: vi.fn(),
}))

const helpers = vi.hoisted(() => ({
  asAsyncSFCModule(component: Record<string, unknown>) {
    return {
      __esModule: true,
      __isTeleport: false,
      __isKeepAlive: false,
      default: component,
      ...component,
    }
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/preview', () => ({
  usePreviewStore: () => previewStore,
}))

vi.mock('@/utils/authStorage', () => ({
  hasStoredSessionHint: vi.fn(() => true),
}))

vi.mock('@/utils/startupTrace', () => ({
  reportStartupMark: startupMarks,
}))

vi.mock('@/utils/prefetch', () => ({
  prefetchCriticalRoutes: vi.fn(),
}))

vi.mock('@/layouts/DefaultLayout.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'DefaultLayout',
    template: '<div class="default-layout-stub" />',
  }),
}))

vi.mock('@/components/NotificationContainer.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'NotificationContainer',
    template: '<div class="notification-container-stub" />',
  }),
}))

vi.mock('@/components/AskUserQuestionDialog.vue', () => ({
  ...helpers.asAsyncSFCModule({
    name: 'AskUserQuestionDialog',
    template: '<div class="ask-user-question-dialog-stub" />',
  }),
}))

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', redirect: '/chat' },
      { path: '/chat', name: 'Chat', component: { template: '<div>Chat</div>' } },
      {
        path: '/login',
        name: 'Login',
        component: { template: '<div>Login</div>' },
        meta: { hideLayout: true },
      },
    ],
  })
}

describe('App route prefetch', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authStore.isAuthenticated = false
    authStore.hasPermission.mockReset()
    authStore.hasPermission.mockReturnValue(false)
    previewStore.isPreviewMode = false
    previewStore.initialize.mockClear()
    window.history.replaceState({}, '', '/chat')
  })

  it('schedules critical route prefetch after the initial route is ready', async () => {
    const router = createTestRouter()
    await router.push('/chat')
    await router.isReady()

    const wrapper = mount(App, {
      global: {
        plugins: [router],
      },
    })

    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    expect(prefetchCriticalRoutes).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})
