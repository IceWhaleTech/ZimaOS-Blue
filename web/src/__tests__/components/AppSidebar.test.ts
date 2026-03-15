import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import AppSidebar from '@/components/AppSidebar.vue'
import { i18n } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'

vi.mock('@/api/workspace', () => ({
  workspaceApi: {
    getMeta: vi.fn().mockResolvedValue({ data: { dir: '/tmp/workspace' } }),
  },
}))

const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => (key in store ? store[key] : null),
    setItem: (key: string, value: string) => {
      store[key] = String(value)
    },
    removeItem: (key: string) => {
      delete store[key]
    },
    clear: () => {
      store = {}
    },
  }
})()

const fetchMock = vi.fn().mockResolvedValue({
  ok: false,
  status: 404,
  json: vi.fn().mockResolvedValue({}),
})

vi.stubGlobal('localStorage', localStorageMock)
vi.stubGlobal('fetch', fetchMock)

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', redirect: '/chat' },
      { path: '/home', name: 'Home', component: { template: '<div>Home</div>' } },
      { path: '/chat', name: 'Chat', component: { template: '<div>Chat</div>' } },
      { path: '/cron', name: 'Cron', component: { template: '<div>Cron</div>' } },
      { path: '/channels', name: 'Channels', component: { template: '<div>Channels</div>' } },
      { path: '/plugins', name: 'Plugins', component: { template: '<div>Plugins</div>' } },
      { path: '/security', name: 'Security', component: { template: '<div>Security</div>' } },
      { path: '/settings', name: 'Settings', component: { template: '<div>Settings</div>' } },
      { path: '/profile', name: 'Profile', component: { template: '<div>Profile</div>' } },
    ],
  })
}

async function mountSidebar(initialPath = '/home') {
  const pinia = createPinia()
  setActivePinia(pinia)

  const authStore = useAuthStore()
  authStore.$patch({
    token: 'test-token',
    user: { username: 'admin', role: 'admin' } as never,
  })

  const router = createTestRouter()
  await router.push(initialPath)
  await router.isReady()

  const wrapper = mount(AppSidebar, {
    global: {
      plugins: [pinia, router, i18n],
    },
  })

  return { wrapper, router }
}

async function mountPreviewSidebar(initialPath = '/chat') {
  const pinia = createPinia()
  setActivePinia(pinia)

  const authStore = useAuthStore()
  authStore.$patch({
    token: 'preview-token',
    user: { username: 'preview', role: 'admin' } as never,
  })

  const previewStore = usePreviewStore()
  previewStore.$patch({
    systemMode: { mode: 'preview', features: {} } as never,
    initialized: true,
  })

  const router = createTestRouter()
  await router.push(initialPath)
  await router.isReady()

  const wrapper = mount(AppSidebar, {
    global: {
      plugins: [pinia, router, i18n],
    },
  })

  return { wrapper, router }
}

describe('AppSidebar', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorageMock.clear()
    fetchMock.mockClear()
  })

  it('renders navigation in the reference order with a configuration section', async () => {
    const { wrapper } = await mountSidebar('/home')

    expect(wrapper.get('[data-testid="sidebar-section-configuration"]').text()).toContain(
      'Configuration'
    )

    const navOrder = wrapper
      .findAll('[data-testid^="sidebar-nav-"]')
      .map((item) => item.attributes('data-testid'))

    expect(navOrder).toEqual([
      'sidebar-nav-dashboard',
      'sidebar-nav-chat',
      'sidebar-nav-workspace',
      'sidebar-nav-channels',
      'sidebar-nav-automation',
      'sidebar-nav-plugins',
      'sidebar-nav-security',
      'sidebar-nav-settings',
      'sidebar-nav-profile',
    ])
  })

  it('toggles the configuration group when clicking the section row', async () => {
    const { wrapper } = await mountSidebar('/home')

    const sectionTrigger = wrapper.get('[data-testid="sidebar-section-configuration"]')

    expect(sectionTrigger.attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[data-testid="sidebar-nav-channels"]').exists()).toBe(true)

    await sectionTrigger.trigger('click')
    await wrapper.vm.$nextTick()

    expect(sectionTrigger.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[data-testid="sidebar-nav-channels"]').exists()).toBe(false)

    await sectionTrigger.trigger('click')
    await wrapper.vm.$nextTick()

    expect(sectionTrigger.attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[data-testid="sidebar-nav-channels"]').exists()).toBe(true)
  })

  it('highlights the active primary route', async () => {
    const { wrapper } = await mountSidebar('/home')

    expect(wrapper.get('[data-testid="sidebar-nav-dashboard"]').classes()).toContain(
      'sidebar-nav-item-active'
    )
    expect(wrapper.get('[data-testid="sidebar-nav-chat"]').classes()).toContain(
      'sidebar-nav-item-inactive'
    )
  })

  it('updates highlight after navigation', async () => {
    const { wrapper, router } = await mountSidebar('/home')

    await router.push('/chat')
    await wrapper.vm.$nextTick()

    expect(wrapper.get('[data-testid="sidebar-nav-chat"]').classes()).toContain(
      'sidebar-nav-item-active'
    )
    expect(wrapper.get('[data-testid="sidebar-nav-dashboard"]').classes()).toContain(
      'sidebar-nav-item-inactive'
    )
  })

  it('highlights automation inside the configuration section on cron route', async () => {
    const { wrapper } = await mountSidebar('/cron')

    expect(wrapper.get('[data-testid="sidebar-nav-automation"]').classes()).toContain(
      'sidebar-nav-item-active'
    )
    expect(wrapper.get('[data-testid="sidebar-nav-settings"]').exists()).toBe(true)
  })

  it('renders a create account button in preview mode instead of profile entry', async () => {
    const { wrapper } = await mountPreviewSidebar('/chat')

    const createAccountButton = wrapper.get(
      '.sidebar-footer [data-testid="sidebar-preview-create-account"]'
    )
    expect(createAccountButton.element.tagName).toBe('BUTTON')
    expect(createAccountButton.text()).toContain(String(i18n.global.t('preview.createAccount')))
    expect(wrapper.find('.sidebar-profile-link').exists()).toBe(false)
    expect(wrapper.find('[data-testid="sidebar-nav-profile"]').exists()).toBe(false)
    expect(
      wrapper.findAll('.sidebar-footer .sidebar-utility-row-preview .sidebar-utility-button')
    ).toHaveLength(2)
    expect(wrapper.find('.sidebar-footer-meta').exists()).toBe(false)
  })
})
