import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import AppSidebar from '@/components/AppSidebar.vue'
import { i18n } from '@/i18n'
import { workspaceApi } from '@/api/workspace'
import { conversationApi, messageApi } from '@/api/chat'
import { refreshTauriDetection } from '@/composables/useTauri'
import { useAuthStore } from '@/stores/auth'
import { usePreviewStore } from '@/stores/preview'
import { useSystemStore } from '@/stores/system'
import { prefetchRoute } from '@/utils/prefetch'

vi.mock('@/api/workspace', () => ({
  workspaceApi: {
    getMeta: vi.fn().mockResolvedValue({ data: { dir: '/tmp/workspace' } }),
    getTree: vi.fn().mockResolvedValue({ data: { root: '/tmp/workspace', entries: [] } }),
    listFiles: vi.fn().mockResolvedValue({ data: { files: [] } }),
    getStats: vi.fn().mockResolvedValue({ data: { files: [], total_tokens: 0, total_bytes: 0 } }),
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

vi.mock('@/utils/typeless', () => ({
  parseTypelessContent: vi.fn((content: string) => {
    const match = content.match(/phone_specs_2026\/完整报告_含截图证据\.md/)
    return {
      cards: match
        ? [
            {
              type: 'result',
              id: 'card-1',
              details: [{ label: 'path', value: match[0] }],
            },
          ]
        : [],
    }
  }),
}))

vi.mock('@/utils/prefetch', () => ({
  prefetchRoute: vi.fn(),
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

function setUserAgent(userAgent: string) {
  Object.defineProperty(window.navigator, 'userAgent', {
    configurable: true,
    value: userAgent,
  })
}

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', redirect: '/chat' },
      { path: '/home', name: 'Home', component: { template: '<div>Home</div>' } },
      { path: '/chat', name: 'Chat', component: { template: '<div>Chat</div>' } },
      { path: '/automation', name: 'Cron', component: { template: '<div>Automation</div>' } },
      { path: '/cron', redirect: '/automation' },
      {
        path: '/automation/harness',
        name: 'HarnessGroups',
        component: { template: '<div>Harness</div>' },
      },
      {
        path: '/automation/harness/:id',
        name: 'HarnessGroupDetail',
        component: { template: '<div>Harness Detail</div>' },
      },
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
    delete (window as any).__TAURI_INTERNALS__
    delete (window as any).__TAURI__
    delete (window as any).__BLUE_DESKTOP__
    setUserAgent(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36'
    )
    refreshTauriDetection()
    vi.mocked(workspaceApi.getMeta).mockResolvedValue({ data: { dir: '/tmp/workspace' } } as never)
    vi.mocked(workspaceApi.getTree).mockResolvedValue({
      data: { root: '/tmp/workspace', entries: [] },
    } as never)
    vi.mocked(workspaceApi.listFiles).mockResolvedValue({ data: { files: [] } } as never)
    vi.mocked(workspaceApi.getStats).mockResolvedValue({
      data: { files: [], total_tokens: 0, total_bytes: 0 },
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

  it('prefetches a route when a navigation item is hovered', async () => {
    const { wrapper } = await mountSidebar('/home')

    await wrapper.get('[data-testid="sidebar-nav-settings"]').trigger('mouseenter')

    expect(prefetchRoute).toHaveBeenCalledWith('Settings')
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

  it('highlights automation inside the configuration section on the automation route', async () => {
    const { wrapper } = await mountSidebar('/automation')

    expect(wrapper.get('[data-testid="sidebar-nav-automation"]').classes()).toContain(
      'sidebar-nav-item-active'
    )
    expect(wrapper.get('[data-testid="sidebar-nav-settings"]').exists()).toBe(true)
  })

  it('keeps automation highlighted on canonical harness routes', async () => {
    const { wrapper } = await mountSidebar('/automation/harness/group-1')

    expect(wrapper.get('[data-testid="sidebar-nav-automation"]').classes()).toContain(
      'sidebar-nav-item-active'
    )
    expect(wrapper.get('[data-testid="sidebar-nav-security"]').classes()).toContain(
      'sidebar-nav-item-inactive'
    )
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

  it('shows the desktop browser shortcut beside GitHub in tauri mode and opens the preferred LAN URL', async () => {
    const invoke = vi.fn().mockResolvedValue(undefined)
    ;(window as any).__TAURI_INTERNALS__ = { invoke }
    ;(window as any).__BLUE_DESKTOP__ = true
    refreshTauriDetection()

    fetchMock.mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url === '/api/v1/network/addresses') {
        return {
          ok: true,
          status: 200,
          json: vi.fn().mockResolvedValue({
            local: 'http://127.0.0.1:3000',
            lan: [
              {
                interface: 'en0',
                address: '192.168.1.23',
                type: 'wifi',
                is_up: true,
                is_ipv6: false,
              },
            ],
            port: 0,
            preferred: 'http://192.168.1.23:0',
          }),
        } as Response
      }

      return {
        ok: false,
        status: 404,
        json: vi.fn().mockResolvedValue({}),
      } as Response
    })

    const { wrapper } = await mountSidebar('/home')
    await flushPromises()
    await flushPromises()

    const browserButton = wrapper.get('[data-testid="sidebar-open-external-browser"]')
    expect(browserButton.attributes('title')).toBeTruthy()

    await browserButton.trigger('click')
    await flushPromises()

    const expectedUrl = new URL('http://192.168.1.23/')
    if (window.location.port) {
      expectedUrl.port = window.location.port
    }

    expect(invoke).toHaveBeenCalledWith('open_url', { url: expectedUrl.toString() })
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
    await flushPromises()

    const jumpLabel = 'Go to conversation'
    const jumpButtons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes(jumpLabel))

    expect(conversationApi.list).toHaveBeenCalled()
    expect(messageApi.list).toHaveBeenCalledWith('conv-phone-specs', expect.any(Number), 0)
    expect(jumpButtons.length).toBeGreaterThanOrEqual(0)
  })

  it('shows token estimate for visible core workspace files only', async () => {
    vi.mocked(workspaceApi.getStats).mockResolvedValue({
      data: {
        files: [
          { name: 'SOUL.md', bytes: 100, tokens: 1200 },
          { name: 'USER.md', bytes: 100, tokens: 600 },
          { name: '2026-04-02.md', bytes: 100, tokens: 5000 },
        ],
        total_tokens: 6800,
        total_bytes: 300,
      },
    } as never)

    const { wrapper } = await mountSidebar('/chat')

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()

    const tokenBadge = wrapper
      .findAll('span')
      .find((node) => node.text().includes('core-file tokens'))

    expect(tokenBadge?.text()).toContain('~1,800 core-file tokens')
  })
})
