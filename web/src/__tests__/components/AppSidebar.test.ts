import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import AppSidebar from '@/components/AppSidebar.vue'
import { i18n } from '@/i18n'
import { workspaceApi } from '@/api/workspace'
import { claudeCodeApi } from '@/api/claudecode'
import { conversationApi, messageApi } from '@/api/chat'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'
import { useSystemStore } from '@/stores/system'

vi.mock('@/api/workspace', () => ({
  workspaceApi: {
    getMeta: vi.fn().mockResolvedValue({ data: { dir: '/tmp/workspace' } }),
    getTree: vi.fn().mockResolvedValue({ data: { root: '/tmp/workspace', entries: [] } }),
    listFiles: vi.fn().mockResolvedValue({ data: { files: [] } }),
    getStats: vi.fn().mockResolvedValue({ data: { files: [], total_tokens: 0, total_bytes: 0 } }),
  },
}))

vi.mock('@/api/claudecode', () => ({
  claudeCodeApi: {
    getConfig: vi.fn().mockResolvedValue({
      data: { whitelist_enabled: false, directory_whitelist: [] },
    }),
  },
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    list: vi.fn().mockResolvedValue({ data: [] }),
  },
  messageApi: {
    list: vi.fn().mockResolvedValue({ data: [] }),
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
    vi.clearAllMocks()
    vi.mocked(workspaceApi.getMeta).mockResolvedValue({ data: { dir: '/tmp/workspace' } } as never)
    vi.mocked(workspaceApi.getTree).mockResolvedValue({
      data: { root: '/tmp/workspace', entries: [] },
    } as never)
    vi.mocked(workspaceApi.listFiles).mockResolvedValue({ data: { files: [] } } as never)
    vi.mocked(workspaceApi.getStats).mockResolvedValue({
      data: { files: [], total_tokens: 0, total_bytes: 0 },
    } as never)
    vi.mocked(claudeCodeApi.getConfig).mockResolvedValue({
      data: { whitelist_enabled: false, directory_whitelist: [] },
    } as never)
    vi.mocked(conversationApi.list).mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
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

  it('keeps the online badge visible before health loads', async () => {
    const systemStore = useSystemStore()
    systemStore.$patch({ health: null })

    const { wrapper } = await mountSidebar('/home')

    const statusBadge = wrapper.get('.sidebar-status-badge')
    expect(statusBadge.text()).toContain('common.online')
    expect(statusBadge.classes()).toContain('sidebar-status-badge-online')
  })

  it('links workspace directories from relative generated paths back to the source conversation', async () => {
    vi.mocked(workspaceApi.getTree).mockResolvedValue({
      data: {
        root: '/tmp/workspace',
        entries: [
          {
            path: 'phone_specs_2026',
            abs_path: '/tmp/workspace/phone_specs_2026',
            name: 'phone_specs_2026',
            type: 'dir',
            depth: 1,
          },
          {
            path: 'phone_specs_2026/完整报告_含截图证据.md',
            abs_path: '/tmp/workspace/phone_specs_2026/完整报告_含截图证据.md',
            name: '完整报告_含截图证据.md',
            type: 'file',
            depth: 2,
            size_bytes: 27690,
          },
        ],
      },
    } as never)
    vi.mocked(conversationApi.list).mockResolvedValue({
      data: [
        {
          id: 'conv-phone-specs',
          title: '2026年1月至今（3月17日）已经发布的新手机',
          created_at: '2026-03-17T15:54:21Z',
          updated_at: '2026-03-17T17:18:51Z',
        },
      ],
    } as never)
    vi.mocked(messageApi.list).mockResolvedValue({
      data: [
        {
          id: 'msg-1',
          conversation_id: 'conv-phone-specs',
          role: 'assistant',
          content:
            '```typeless\n' +
            '{"details":[{"label":"path","value":"phone_specs_2026/完整报告_含截图证据.md"},{"label":"success","value":"true"}],"status":"success","title":"write_commit","type":"result"}\n' +
            '```',
          created_at: '2026-03-17T17:13:00Z',
        },
      ],
    } as never)

    const { wrapper } = await mountSidebar('/chat')

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()

    const generatedTabLabel = 'File Sources'
    const generatedTab = wrapper
      .findAll('button')
      .find((button) => button.text().includes(generatedTabLabel))
    expect(generatedTab).toBeTruthy()

    await generatedTab!.trigger('click')
    await flushPromises()
    await flushPromises()

    const jumpLabel = 'Go to conversation'
    const jumpButtons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes(jumpLabel))

    expect(wrapper.text()).toContain('phone_specs_2026')
    expect(wrapper.text()).toContain('完整报告_含截图证据.md')
    expect(jumpButtons).toHaveLength(2)
  })
})
