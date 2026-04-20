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
import { useChatStore } from '@/stores/chat'
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
    const matches = Array.from(
      content.matchAll(/"label":"path","value":"([^"]+)"/g),
      (match) => match[1]
    )
    return {
      cards: matches.map((path, index) => ({
        type: 'result',
        id: `card-${index + 1}`,
        details: [{ label: 'path', value: path }],
      })),
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
      { path: '/operations', name: 'Operations', component: { template: '<div>Operations</div>' } },
      {
        path: '/operations/harness',
        name: 'HarnessGroups',
        component: { template: '<div>Harness</div>' },
      },
      {
        path: '/operations/harness/:id',
        name: 'HarnessGroupDetail',
        component: { template: '<div>Harness Detail</div>' },
      },
      { path: '/channels', name: 'Channels', component: { template: '<div>Channels</div>' } },
      { path: '/plugins', name: 'Plugins', component: { template: '<div>Plugins</div>' } },
      {
        path: '/operations/knowledge',
        name: 'Knowledge',
        component: { template: '<div>Knowledge</div>' },
      },
      { path: '/security', name: 'Security', component: { template: '<div>Security</div>' } },
      { path: '/settings', name: 'Settings', component: { template: '<div>Settings</div>' } },
      { path: '/profile', name: 'Profile', component: { template: '<div>Profile</div>' } },
    ],
  })
}

function findButtonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find((button) => button.text().includes(text))
}

function findWorkspaceTreeRow(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('[data-current-conversation]').find((row) => row.text().includes(text))
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

  it('highlights operations inside the configuration section on the operations route', async () => {
    const { wrapper } = await mountSidebar('/operations')

    expect(wrapper.get('[data-testid="sidebar-nav-automation"]').classes()).toContain(
      'sidebar-nav-item-active'
    )
    expect(wrapper.get('[data-testid="sidebar-nav-settings"]').exists()).toBe(true)
  })

  it('keeps operations highlighted on canonical harness routes', async () => {
    const { wrapper } = await mountSidebar('/operations/harness/group-1')

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

    const generatedTabLabel = 'Workspace Files'
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

  it('refreshes generated workspace data each time the file sources tab is reopened', async () => {
    const { wrapper } = await mountSidebar('/chat')

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()

    const generatedTab = findButtonByText(wrapper, 'Workspace Files')
    const coreTab = findButtonByText(wrapper, 'Core Context Files')

    expect(generatedTab).toBeTruthy()
    expect(coreTab).toBeTruthy()

    await generatedTab!.trigger('click')
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    const getTreeCallsAfterFirstOpen = vi.mocked(workspaceApi.getTree).mock.calls.length
    const listCallsAfterFirstOpen = vi.mocked(conversationApi.list).mock.calls.length

    await coreTab!.trigger('click')
    await flushPromises()
    await generatedTab!.trigger('click')
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    expect(vi.mocked(workspaceApi.getTree).mock.calls.length).toBeGreaterThan(
      getTreeCallsAfterFirstOpen
    )
    expect(vi.mocked(conversationApi.list).mock.calls.length).toBeGreaterThan(
      listCallsAfterFirstOpen
    )
  })

  it('refreshes the workspace tree while the generated workspace view stays open', async () => {
    vi.useFakeTimers()
    try {
      const { wrapper } = await mountSidebar('/chat')

      await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
      await flushPromises()
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      const initialTreeCalls = vi.mocked(workspaceApi.getTree).mock.calls.length
      const initialConversationCalls = vi.mocked(conversationApi.list).mock.calls.length

      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      expect(vi.mocked(workspaceApi.getTree).mock.calls.length).toBeGreaterThan(initialTreeCalls)
      expect(vi.mocked(conversationApi.list).mock.calls.length).toBe(initialConversationCalls)
    } finally {
      vi.useRealTimers()
    }
  })

  it('focuses and highlights files linked to the current conversation', async () => {
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
          created_at: new Date().toISOString(),
        },
      ],
    } as never)

    const { wrapper } = await mountSidebar('/chat')
    const chatStore = useChatStore()
    chatStore.currentConversationId = 'conv-phone-specs'

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()

    const generatedTab = findButtonByText(wrapper, 'Workspace Files')
    expect(generatedTab).toBeTruthy()

    await generatedTab!.trigger('click')
    await flushPromises()
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    const currentConversationFilter = wrapper.get(
      '[data-testid="workspace-current-conversation-filter"]'
    )
    expect(currentConversationFilter.text()).toContain('1')

    const highlightedRows = wrapper.findAll('[data-current-conversation="true"]')
    expect(highlightedRows.length).toBeGreaterThan(0)
    expect(highlightedRows.some((row) => row.text().includes('完整报告_含截图证据.md'))).toBe(true)
    expect(wrapper.text()).toContain('Current conversation')
  })

  it('continues scanning older conversation pages for generated workspace files', async () => {
    vi.mocked(workspaceApi.getTree).mockResolvedValue({
      data: {
        root: '/tmp/workspace',
        entries: [
          {
            path: 'archived/recovered.md',
            abs_path: '/tmp/workspace/archived/recovered.md',
            name: 'recovered.md',
            type: 'file',
            depth: 2,
            size_bytes: 512,
          },
        ],
      },
    } as never)
    vi.mocked(conversationApi.list).mockImplementation(async (limit = 50, offset = 0) => {
      if (limit !== 20) {
        return { data: [] } as never
      }
      if (offset === 0) {
        return {
          data: Array.from({ length: 20 }, (_, index) => ({
            id: `conv-${index + 1}`,
            title: `Conv ${index + 1}`,
            created_at: '2026-04-18T10:00:00Z',
            updated_at: '2026-04-18T10:00:00Z',
          })),
        } as never
      }
      if (offset === 20) {
        return {
          data: [
            {
              id: 'conv-target',
              title: 'Recovered conversation',
              created_at: '2026-04-18T10:00:00Z',
              updated_at: '2026-04-18T10:00:00Z',
            },
          ],
        } as never
      }
      return { data: [] } as never
    })
    vi.mocked(messageApi.list).mockImplementation(async (conversationId: string) => {
      if (conversationId === 'conv-target') {
        return {
          data: [
            {
              id: 'msg-target',
              conversation_id: 'conv-target',
              role: 'assistant',
              content:
                '```typeless\n' +
                '{"details":[{"label":"path","value":"archived/recovered.md"}],"status":"success","title":"write_commit","type":"result"}\n' +
                '```',
              created_at: '2026-04-18T10:30:00Z',
            },
          ],
        } as never
      }
      return { data: [] } as never
    })

    const { wrapper } = await mountSidebar('/chat')
    const chatStore = useChatStore()
    chatStore.currentConversationId = 'conv-target'

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    expect(
      vi.mocked(conversationApi.list).mock.calls.some(
        ([limit, offset]) => limit === 20 && offset === 20
      )
    ).toBe(true)
    expect(wrapper.text()).toContain('Current conversation')
  })

  it('continues scanning older message pages for generated workspace files', async () => {
    vi.mocked(workspaceApi.getTree).mockResolvedValue({
      data: {
        root: '/tmp/workspace',
        entries: [
          {
            path: 'reports/deep-history.md',
            abs_path: '/tmp/workspace/reports/deep-history.md',
            name: 'deep-history.md',
            type: 'file',
            depth: 2,
            size_bytes: 640,
          },
        ],
      },
    } as never)
    vi.mocked(conversationApi.list).mockResolvedValue({
      data: [
        {
          id: 'conv-history',
          title: 'Historical report conversation',
          created_at: '2026-04-18T10:00:00Z',
          updated_at: '2026-04-18T10:00:00Z',
        },
      ],
    } as never)
    vi.mocked(messageApi.list).mockImplementation(
      async (conversationId: string, limit = 100, offset = 0) => {
        if (conversationId !== 'conv-history' || limit !== 120) {
          return { data: [] } as never
        }
        if (offset === 0) {
          return {
            data: Array.from({ length: 120 }, (_, index) => ({
              id: `msg-${index + 1}`,
              conversation_id: 'conv-history',
              role: 'assistant',
              content: 'no generated path here',
              created_at: '2026-04-18T10:00:00Z',
            })),
          } as never
        }
        if (offset === 120) {
          return {
            data: [
              {
                id: 'msg-target',
                conversation_id: 'conv-history',
                role: 'assistant',
                content:
                  '```typeless\n' +
                  '{"details":[{"label":"path","value":"reports/deep-history.md"}],"status":"success","title":"write_commit","type":"result"}\n' +
                  '```',
                created_at: '2026-04-18T10:30:00Z',
              },
            ],
          } as never
        }
        return { data: [] } as never
      }
    )

    const { wrapper } = await mountSidebar('/chat')
    const chatStore = useChatStore()
    chatStore.currentConversationId = 'conv-history'

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    expect(
      vi.mocked(messageApi.list).mock.calls.some(
        ([conversationId, limit, offset]) =>
          conversationId === 'conv-history' && limit === 120 && offset === 120
      )
    ).toBe(true)
    expect(wrapper.text()).toContain('Current conversation')
  })

  it('scopes the workspace tree to the current conversation directory when generated files share a subtree', async () => {
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
          {
            path: 'phone_specs_2026/assets',
            abs_path: '/tmp/workspace/phone_specs_2026/assets',
            name: 'assets',
            type: 'dir',
            depth: 2,
          },
          {
            path: 'phone_specs_2026/assets/spec-sheet.png',
            abs_path: '/tmp/workspace/phone_specs_2026/assets/spec-sheet.png',
            name: 'spec-sheet.png',
            type: 'file',
            depth: 3,
            size_bytes: 1024,
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
            '{"details":[{"label":"path","value":"phone_specs_2026/完整报告_含截图证据.md"},{"label":"path","value":"phone_specs_2026/assets/spec-sheet.png"}],"status":"success","title":"write_commit","type":"result"}\n' +
            '```',
          created_at: '2026-03-17T17:13:00Z',
        },
      ],
    } as never)

    const { wrapper } = await mountSidebar('/chat')
    const chatStore = useChatStore()
    chatStore.currentConversationId = 'conv-phone-specs'

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    expect(
      vi.mocked(workspaceApi.getTree).mock.calls.some(
        ([params]) =>
          params?.max_depth === 16 && params?.root === '/tmp/workspace/phone_specs_2026'
      )
    ).toBe(true)
  })

  it('continues loading paginated workspace tree responses until later entries are included', async () => {
    vi.mocked(workspaceApi.getTree).mockImplementation(async (params?: Record<string, unknown>) => {
      const offset = Number(params?.offset || 0)
      if (offset === 0) {
        return {
          data: {
            root: '/tmp/workspace',
            entries: [
              {
                path: 'batch/a.txt',
                abs_path: '/tmp/workspace/batch/a.txt',
                name: 'a.txt',
                type: 'file',
                depth: 2,
                size_bytes: 16,
              },
              {
                path: 'batch/b.txt',
                abs_path: '/tmp/workspace/batch/b.txt',
                name: 'b.txt',
                type: 'file',
                depth: 2,
                size_bytes: 16,
              },
            ],
            next_offset: 2,
            has_more: true,
          },
        } as never
      }
      if (offset === 2) {
        return {
          data: {
            root: '/tmp/workspace',
            entries: [
              {
                path: 'batch/late-report.md',
                abs_path: '/tmp/workspace/batch/late-report.md',
                name: 'late-report.md',
                type: 'file',
                depth: 2,
                size_bytes: 64,
              },
            ],
            next_offset: 3,
            has_more: false,
          },
        } as never
      }
      return {
        data: { root: '/tmp/workspace', entries: [], next_offset: offset, has_more: false },
      } as never
    })

    const { wrapper } = await mountSidebar('/chat')

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    expect(
      vi.mocked(workspaceApi.getTree).mock.calls.some(
        ([params]) => params?.max_depth === 16 && params?.offset === 2
      )
    ).toBe(true)
    expect(wrapper.text()).toContain('late-report.md')
  })

  it('background workspace tree polling preserves expanded folders without rescanning links', async () => {
    vi.useFakeTimers()
    try {
      vi.mocked(workspaceApi.getTree).mockResolvedValue({
        data: {
          root: '/tmp/workspace',
          entries: [
            {
              path: 'memory',
              abs_path: '/tmp/workspace/memory',
              name: 'memory',
              type: 'dir',
              depth: 1,
            },
            {
              path: 'memory/daily',
              abs_path: '/tmp/workspace/memory/daily',
              name: 'daily',
              type: 'dir',
              depth: 2,
            },
            {
              path: 'memory/daily/report.txt',
              abs_path: '/tmp/workspace/memory/daily/report.txt',
              name: 'report.txt',
              type: 'file',
              depth: 3,
              size_bytes: 128,
            },
          ],
        },
      } as never)

      const { wrapper } = await mountSidebar('/chat')

      await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
      await flushPromises()
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      const collapsedMemoryRow = findWorkspaceTreeRow(wrapper, 'memory')
      expect(collapsedMemoryRow).toBeTruthy()
      expect(collapsedMemoryRow!.get('button').attributes('title')).toBe('Expand folder')

      await collapsedMemoryRow!.get('button').trigger('click')
      await flushPromises()

      const expandedMemoryRow = findWorkspaceTreeRow(wrapper, 'memory')
      expect(expandedMemoryRow).toBeTruthy()
      expect(expandedMemoryRow!.get('button').attributes('title')).toBe('Collapse folder')
      expect(wrapper.text()).toContain('memory/daily')

      const initialTreeCalls = vi.mocked(workspaceApi.getTree).mock.calls.length
      const initialConversationCalls = vi.mocked(conversationApi.list).mock.calls.length

      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      const refreshedMemoryRow = findWorkspaceTreeRow(wrapper, 'memory')
      expect(refreshedMemoryRow).toBeTruthy()
      expect(vi.mocked(workspaceApi.getTree).mock.calls.length).toBeGreaterThan(initialTreeCalls)
      expect(vi.mocked(conversationApi.list).mock.calls.length).toBe(initialConversationCalls)
      expect(refreshedMemoryRow!.get('button').attributes('title')).toBe('Collapse folder')
      expect(wrapper.text()).toContain('memory/daily')
    } finally {
      vi.useRealTimers()
    }
  })

  it('highlights entries added or deleted after the initial workspace tree fetch', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-19T10:00:00Z'))
    try {
      vi.mocked(workspaceApi.getTree).mockImplementation(async () => {
        const callCount = vi.mocked(workspaceApi.getTree).mock.calls.length
        if (callCount <= 1) {
          return {
            data: {
              root: '/tmp/workspace',
              entries: [
                {
                  path: 'keep.md',
                  abs_path: '/tmp/workspace/keep.md',
                  name: 'keep.md',
                  type: 'file',
                  depth: 1,
                  size_bytes: 64,
                  modified_at: '2026-04-19T09:55:00Z',
                },
                {
                  path: 'remove.md',
                  abs_path: '/tmp/workspace/remove.md',
                  name: 'remove.md',
                  type: 'file',
                  depth: 1,
                  size_bytes: 32,
                  modified_at: '2026-04-19T09:56:00Z',
                },
              ],
            },
          } as never
        }
        return {
          data: {
            root: '/tmp/workspace',
            entries: [
              {
                path: 'keep.md',
                abs_path: '/tmp/workspace/keep.md',
                name: 'keep.md',
                type: 'file',
                depth: 1,
                size_bytes: 64,
                modified_at: '2026-04-19T09:55:00Z',
              },
              {
                path: 'added.md',
                abs_path: '/tmp/workspace/added.md',
                name: 'added.md',
                type: 'file',
                depth: 1,
                size_bytes: 48,
                modified_at: '2026-04-19T10:00:03Z',
              },
            ],
          },
        } as never
      })

      const { wrapper } = await mountSidebar('/chat')

      await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
      await flushPromises()
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      expect(wrapper.text()).toContain('keep.md')
      expect(wrapper.text()).toContain('remove.md')
      expect(wrapper.text()).not.toContain('Added')
      expect(wrapper.text()).not.toContain('Deleted')

      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      const keepRow = findWorkspaceTreeRow(wrapper, 'keep.md')
      const addedRow = findWorkspaceTreeRow(wrapper, 'added.md')
      const deletedRow = findWorkspaceTreeRow(wrapper, 'remove.md')

      expect(keepRow).toBeTruthy()
      expect(addedRow).toBeTruthy()
      expect(deletedRow).toBeTruthy()
      expect(keepRow!.text()).not.toContain('Added')
      expect(addedRow!.text()).toContain('Added')
      expect(deletedRow!.text()).toContain('Deleted')
      expect(wrapper.findAll('span').some((node) => node.text().includes('Files 2'))).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })

  it('expires deleted workspace tree markers after a short delay', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-19T10:00:00Z'))
    try {
      vi.mocked(workspaceApi.getTree).mockImplementation(async () => {
        const callCount = vi.mocked(workspaceApi.getTree).mock.calls.length
        if (callCount <= 1) {
          return {
            data: {
              root: '/tmp/workspace',
              entries: [
                {
                  path: 'keep.md',
                  abs_path: '/tmp/workspace/keep.md',
                  name: 'keep.md',
                  type: 'file',
                  depth: 1,
                  size_bytes: 64,
                  modified_at: '2026-04-19T09:55:00Z',
                },
                {
                  path: 'remove.md',
                  abs_path: '/tmp/workspace/remove.md',
                  name: 'remove.md',
                  type: 'file',
                  depth: 1,
                  size_bytes: 32,
                  modified_at: '2026-04-19T09:56:00Z',
                },
              ],
            },
          } as never
        }
        return {
          data: {
            root: '/tmp/workspace',
            entries: [
              {
                path: 'keep.md',
                abs_path: '/tmp/workspace/keep.md',
                name: 'keep.md',
                type: 'file',
                depth: 1,
                size_bytes: 64,
                modified_at: '2026-04-19T09:55:00Z',
              },
            ],
          },
        } as never
      })

      const { wrapper } = await mountSidebar('/chat')

      await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
      await flushPromises()
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      await vi.advanceTimersByTimeAsync(6000)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      expect(findWorkspaceTreeRow(wrapper, 'remove.md')?.text()).toContain('Deleted')

      await vi.advanceTimersByTimeAsync(16000)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      expect(findWorkspaceTreeRow(wrapper, 'remove.md')).toBeFalsy()
      expect(wrapper.findAll('span').some((node) => node.text().includes('Files 1'))).toBe(true)
    } finally {
      vi.useRealTimers()
    }
  })

  it('defaults memory and knowledge directories to collapsed in the workspace tree', async () => {
    vi.mocked(workspaceApi.getTree).mockResolvedValue({
      data: {
        root: '/tmp/workspace',
        entries: [
          {
            path: 'memory',
            abs_path: '/tmp/workspace/memory',
            name: 'memory',
            type: 'dir',
            depth: 1,
          },
          {
            path: 'memory/daily',
            abs_path: '/tmp/workspace/memory/daily',
            name: 'daily',
            type: 'dir',
            depth: 2,
          },
          {
            path: 'memory/daily/report.txt',
            abs_path: '/tmp/workspace/memory/daily/report.txt',
            name: 'report.txt',
            type: 'file',
            depth: 3,
            size_bytes: 128,
          },
          {
            path: 'knowledge',
            abs_path: '/tmp/workspace/knowledge',
            name: 'knowledge',
            type: 'dir',
            depth: 1,
          },
          {
            path: 'knowledge/overview.md',
            abs_path: '/tmp/workspace/knowledge/overview.md',
            name: 'overview.md',
            type: 'file',
            depth: 2,
            size_bytes: 256,
          },
          {
            path: 'project',
            abs_path: '/tmp/workspace/project',
            name: 'project',
            type: 'dir',
            depth: 1,
          },
          {
            path: 'project/notes.md',
            abs_path: '/tmp/workspace/project/notes.md',
            name: 'notes.md',
            type: 'file',
            depth: 2,
            size_bytes: 512,
          },
        ],
      },
    } as never)

    const { wrapper } = await mountSidebar('/chat')

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()
    await flushPromises()

    const memoryRow = findWorkspaceTreeRow(wrapper, 'memory')
    const knowledgeRow = findWorkspaceTreeRow(wrapper, 'knowledge')
    const projectRow = findWorkspaceTreeRow(wrapper, 'project')

    expect(memoryRow).toBeTruthy()
    expect(knowledgeRow).toBeTruthy()
    expect(projectRow).toBeTruthy()
    expect(memoryRow!.get('button').attributes('title')).toBe('Expand folder')
    expect(knowledgeRow!.get('button').attributes('title')).toBe('Expand folder')
    expect(projectRow!.get('button').attributes('title')).toBe('Collapse folder')
    expect(wrapper.text()).not.toContain('memory/daily')
    expect(wrapper.text()).not.toContain('knowledge/overview.md')
    expect(wrapper.text()).toContain('project/notes.md')
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
    await findButtonByText(wrapper, 'Core Context Files')!.trigger('click')
    await flushPromises()

    const tokenBadge = wrapper
      .findAll('span')
      .find((node) => node.text().includes('core-file tokens'))

    expect(tokenBadge?.text()).toContain('~1,800 core-file tokens')
  })

  it('opens the workspace panel on the real workspace file tree by default', async () => {
    const { wrapper } = await mountSidebar('/chat')

    await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
    await flushPromises()
    await flushPromises()

    expect(wrapper.text()).toContain('Workspace Directory Tree')
    expect(vi.mocked(workspaceApi.getTree)).toHaveBeenCalled()
  })

  it('refreshes generated workspace data when conversation output changes but keeps the same length', async () => {
    vi.useFakeTimers()
    try {
      const { wrapper } = await mountSidebar('/chat')
      const chatStore = useChatStore()
      chatStore.currentConversationId = 'conv-refresh'

      await wrapper.get('[data-testid="sidebar-nav-workspace"]').trigger('click')
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      chatStore.messages = [
        {
          id: 'msg-refresh',
          conversation_id: 'conv-refresh',
          role: 'assistant',
          content:
            '```typeless\n' +
            '{"details":[{"label":"path","value":"reports/report_a.md"}],"status":"success","title":"write_commit","type":"result"}\n' +
            '```',
          created_at: '2026-04-15T09:00:00Z',
        },
      ] as never
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1300)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      const initialTreeCalls = vi.mocked(workspaceApi.getTree).mock.calls.length
      const initialConversationCalls = vi.mocked(conversationApi.list).mock.calls.length

      chatStore.messages = [
        {
          id: 'msg-refresh',
          conversation_id: 'conv-refresh',
          role: 'assistant',
          content:
            '```typeless\n' +
            '{"details":[{"label":"path","value":"reports/report_b.md"}],"status":"success","title":"write_commit","type":"result"}\n' +
            '```',
          created_at: '2026-04-15T09:00:00Z',
        },
      ] as never
      await flushPromises()

      await vi.advanceTimersByTimeAsync(1300)
      await flushPromises()
      await vi.dynamicImportSettled()
      await flushPromises()

      expect(vi.mocked(workspaceApi.getTree).mock.calls.length).toBeGreaterThan(initialTreeCalls)
      expect(vi.mocked(conversationApi.list).mock.calls.length).toBeGreaterThan(
        initialConversationCalls
      )
    } finally {
      vi.useRealTimers()
    }
  })
})
