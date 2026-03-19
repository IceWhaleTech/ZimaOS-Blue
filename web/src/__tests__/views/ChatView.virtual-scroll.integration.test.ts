import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ChatView from '@/views/ChatView.vue'
import { i18n } from '@/i18n'
import { useChatStore } from '@/stores/chat'
import { conversationApi, messageApi } from '@/api/chat'

const mocks = vi.hoisted(() => ({
  sseConnect: vi.fn(),
  sseDisconnect: vi.fn(),
  settingsStore: {
    agentAutoConfirm: false,
    agentMode: false,
    claudeCodeEnabled: false,
    showToolDetails: true,
    selectedProvider: 'openai',
    selectedModel: 'gpt-4o-mini',
    temperature: 0.7,
    maxTokens: 8192,
    fetchTools: vi.fn(),
    updateFromPoolProviders: vi.fn(),
    setAgentAutoConfirm: vi.fn(),
    setAgentMode: vi.fn(),
    setShowToolDetails: vi.fn(),
  },
  providerPoolStore: {
    activeProviders: [] as unknown[],
    cloudProviders: [] as unknown[],
    enabledProviders: [] as unknown[],
    hasCloudProviders: false,
    hasLocalProviders: false,
    localProviders: [] as unknown[],
    models: [] as unknown[],
    providers: [] as unknown[],
    routingMode: 'auto',
    trialProviders: [] as unknown[],
    trialQuota: null as null | Record<string, unknown>,
    getProviderDisplayName: vi.fn((providerId: string) => providerId),
    fetchProviders: vi.fn(),
    fetchRoutingMode: vi.fn(),
    fetchTrialQuota: vi.fn(),
    setRoutingMode: vi.fn(),
  },
  deepResearchJobsStore: {
    activeJobs: [] as Array<Record<string, unknown>>,
    jobMap: {} as Record<string, unknown>,
    loading: false,
    hydrated: true,
    pendingFocusJobId: null as string | null,
    fetchActiveJobs: vi.fn(),
    handleGlobalEvent: vi.fn(),
    applyJobSnapshot: vi.fn(),
    openJob: vi.fn(),
    cancelJob: vi.fn(),
    consumePendingFocusJobId: vi.fn(),
  },
  mediaGenerate: {
    showPanel: { value: false },
    intent: { value: null },
    models: { value: [] as unknown[] },
    selectedModel: { value: '' },
    generating: { value: false },
    ambiguous: { value: false },
    task: { value: null },
    classify: vi.fn(),
    generate: vi.fn(),
    reset: vi.fn(),
    confirmAmbiguous: vi.fn(),
    cancel: vi.fn(),
    switchCategory: vi.fn(),
  },
  speechGetStatus: vi.fn(),
  agentApi: {
    listTasks: vi.fn(),
    cancelTask: vi.fn(),
    deleteTask: vi.fn(),
    sendMessage: vi.fn(),
    submitAnswers: vi.fn(),
  },
  authFetch: vi.fn(),
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  onSSEEvent: vi.fn(),
  offSSEEvent: vi.fn(),
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  },
  warmupTrigger: vi.fn(),
  injectMessage: vi.fn(),
  approvalApi: {
    listPending: vi.fn(),
    resolve: vi.fn(),
    getConfig: vi.fn(),
    updateConfig: vi.fn(),
  },
  systemWriteLog: vi.fn(),
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    create: vi.fn(),
    list: vi.fn(),
    get: vi.fn(),
    delete: vi.fn(),
    search: vi.fn(),
    getCommandState: vi.fn(),
    patchCommandState: vi.fn(),
  },
  messageApi: {
    list: vi.fn(),
    send: vi.fn(),
    cancelStream: vi.fn(),
  },
  warmupApi: {
    trigger: (...args: unknown[]) => mocks.warmupTrigger(...args),
  },
  injectionApi: {
    inject: (...args: unknown[]) => mocks.injectMessage(...args),
  },
  cardActionApi: {
    submit: vi.fn(),
  },
  agentApi: {
    listTasks: (...args: unknown[]) => mocks.agentApi.listTasks(...args),
    cancelTask: (...args: unknown[]) => mocks.agentApi.cancelTask(...args),
    deleteTask: (...args: unknown[]) => mocks.agentApi.deleteTask(...args),
    sendMessage: (...args: unknown[]) => mocks.agentApi.sendMessage(...args),
    submitAnswers: (...args: unknown[]) => mocks.agentApi.submitAnswers(...args),
  },
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => mocks.settingsStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/deepResearchJobs', () => ({
  useDeepResearchJobsStore: () => mocks.deepResearchJobsStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/client', () => ({
  default: {
    get: (...args: unknown[]) => mocks.apiGet(...args),
    post: (...args: unknown[]) => mocks.apiPost(...args),
  },
  authFetch: (...args: unknown[]) => mocks.authFetch(...args),
}))

vi.mock('@/api/approval', () => ({
  approvalApi: {
    listPending: (...args: unknown[]) => mocks.approvalApi.listPending(...args),
    resolve: (...args: unknown[]) => mocks.approvalApi.resolve(...args),
    getConfig: (...args: unknown[]) => mocks.approvalApi.getConfig(...args),
    updateConfig: (...args: unknown[]) => mocks.approvalApi.updateConfig(...args),
  },
}))

vi.mock('@/api/system', () => ({
  systemApi: {
    writeLog: (...args: unknown[]) => mocks.systemWriteLog(...args),
  },
}))

vi.mock('@/utils/sse', () => ({
  SSEClient: class {
    connect(...args: unknown[]) {
      return mocks.sseConnect(...args)
    }

    disconnect(...args: unknown[]) {
      return mocks.sseDisconnect(...args)
    }
  },
}))

vi.mock('@/api/voice', () => ({
  ttsAudioManager: {
    stop: vi.fn(),
  },
  streamingTTSManager: {
    setLocale: vi.fn(),
    stop: vi.fn(),
    streamAndPlay: vi.fn(),
    reset: vi.fn(),
    streamText: vi.fn(),
    play: vi.fn(),
    onComplete: null as null | (() => void),
  },
}))

vi.mock('@/api/speech', () => ({
  speechApi: {
    getStatus: (...args: unknown[]) => mocks.speechGetStatus(...args),
  },
}))

vi.mock('@/composables/useEventStream', () => ({
  onSSEEvent: (...args: unknown[]) => mocks.onSSEEvent(...args),
  offSSEEvent: (...args: unknown[]) => mocks.offSSEEvent(...args),
}))

vi.mock('@/composables/useKeyboardShortcuts', () => ({
  useChatShortcuts: vi.fn(),
}))

vi.mock('@/composables/useMediaGenerate', () => ({
  useMediaGenerate: () => mocks.mediaGenerate,
}))

vi.mock('@/components/ConversationList.vue', () => ({
  default: { name: 'ConversationList', template: '<div class="conversation-list-stub" />' },
}))

vi.mock('@/components/ChatInput.vue', () => ({
  default: { name: 'ChatInput', template: '<div class="chat-input-stub" />' },
}))

vi.mock('@/components/ChatMessage.vue', () => ({
  default: {
    name: 'ChatMessage',
    props: {
      message: { type: Object, required: true },
    },
    template:
      '<div class="chat-message-stub" :data-message-id="message.id" style="height: 100px;">{{ message.content }}</div>',
  },
}))

vi.mock('@/components/onboarding/PresetQuestions.vue', () => ({
  default: { name: 'PresetQuestions', template: '<div class="preset-questions-stub" />' },
}))

vi.mock('@/components/chat/TalkMode.vue', () => ({
  default: { name: 'TalkMode', template: '<div class="talk-mode-stub" />' },
}))

vi.mock('@/components/ToolApprovalDialog.vue', () => ({
  default: { name: 'ToolApprovalDialog', template: '<div class="tool-approval-stub" />' },
}))

vi.mock('@/components/ExecApprovalDialog.vue', () => ({
  default: { name: 'ExecApprovalDialog', template: '<div class="exec-approval-stub" />' },
}))

vi.mock('@/components/MediaParamPanel.vue', () => ({
  default: { name: 'MediaParamPanel', template: '<div class="media-param-panel-stub" />' },
}))

vi.mock('@/components/AgentTaskPanel.vue', () => ({
  default: { name: 'AgentTaskPanel', template: '<div class="agent-task-panel-stub" />' },
}))

vi.mock('@/components/DeepResearchTaskDock.vue', () => ({
  default: {
    name: 'DeepResearchTaskDock',
    template: '<div class="deep-research-task-dock-stub" />',
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

vi.stubGlobal('localStorage', localStorageMock)
vi.stubGlobal(
  'matchMedia',
  vi.fn().mockImplementation(() => ({
    matches: false,
    media: '',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
)
vi.stubGlobal(
  'ResizeObserver',
  class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
)
if (typeof window !== 'undefined' && typeof window.requestAnimationFrame !== 'function') {
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) =>
    setTimeout(() => cb(Date.now()), 0)
  )
  vi.stubGlobal('cancelAnimationFrame', (id: number) => clearTimeout(id))
}

const CONVERSATION = {
  id: 'conv-1',
  title: 'Long thread',
  created_at: '2026-03-08T00:00:00.000Z',
  updated_at: '2026-03-08T00:00:02.000Z',
}

function mockRect(top: number, height = 40): DOMRect {
  return {
    top,
    left: 0,
    right: 320,
    bottom: top + height,
    width: 320,
    height,
    x: 0,
    y: top,
    toJSON: () => ({}),
  } as DOMRect
}

function makeAssistantMessage(id: string, content: string, createdAt: string) {
  return {
    id,
    conversation_id: CONVERSATION.id,
    role: 'assistant' as const,
    content,
    created_at: createdAt,
  }
}

async function settleView() {
  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await flushPromises()
  await new Promise((resolve) => window.requestAnimationFrame(() => resolve(undefined)))
  await flushPromises()
}

async function mountIntegratedChatView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  window.history.replaceState({}, '', '/chat?conversationId=conv-1')

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/chat', component: { template: '<div />' } },
      { path: '/settings', component: { template: '<div />' } },
    ],
  })
  router.push('/chat')
  await router.isReady()

  const wrapper = mount(ChatView, {
    global: {
      plugins: [pinia, i18n, router],
      stubs: {
        Teleport: true,
        Transition: true,
      },
    },
  })

  await settleView()
  return { wrapper, store: useChatStore() }
}

describe('ChatView virtual scroll integration', () => {
  let rectSpy: ReturnType<typeof vi.spyOn>
  let scrollToSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    localStorageMock.clear()
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
    Object.defineProperty(window.navigator, 'platform', { value: 'MacIntel', configurable: true })

    vi.clearAllMocks()

    rectSpy = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function () {
      const el = this as HTMLElement
      if (el.classList.contains('chat-messages-area')) {
        return mockRect(0, 400)
      }
      if (el.classList.contains('virtual-scroll-container')) {
        const scrollContainer = el.closest('.chat-messages-area') as HTMLElement | null
        return mockRect(-(scrollContainer?.scrollTop ?? 0), 0)
      }
      if (el.dataset.messageId) {
        return mockRect(0, 100)
      }
      return mockRect(0, 40)
    })

    scrollToSpy = vi.spyOn(HTMLElement.prototype, 'scrollTo').mockImplementation(function (arg) {
      if (typeof arg === 'object' && arg && 'top' in arg) {
        this.scrollTop = Number(arg.top) || 0
      }
    })

    mocks.sseConnect.mockReset().mockResolvedValue(undefined)
    mocks.sseDisconnect.mockReset()

    mocks.settingsStore.fetchTools.mockReset().mockResolvedValue(undefined)
    mocks.settingsStore.updateFromPoolProviders.mockReset()
    mocks.settingsStore.setAgentAutoConfirm.mockReset()
    mocks.settingsStore.setAgentMode.mockReset()
    mocks.settingsStore.setShowToolDetails.mockReset()

    mocks.providerPoolStore.activeProviders = []
    mocks.providerPoolStore.cloudProviders = []
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.hasCloudProviders = false
    mocks.providerPoolStore.hasLocalProviders = false
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.models = []
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.routingMode = 'auto'
    mocks.providerPoolStore.trialProviders = []
    mocks.providerPoolStore.trialQuota = null
    mocks.providerPoolStore.fetchProviders.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.setRoutingMode.mockReset().mockResolvedValue(undefined)

    mocks.deepResearchJobsStore.activeJobs = []
    mocks.deepResearchJobsStore.fetchActiveJobs.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.handleGlobalEvent.mockReset()
    mocks.deepResearchJobsStore.applyJobSnapshot.mockReset()
    mocks.deepResearchJobsStore.openJob.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.cancelJob.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.consumePendingFocusJobId.mockReset()

    mocks.mediaGenerate.classify.mockReset()
    mocks.mediaGenerate.generate.mockReset()
    mocks.mediaGenerate.reset.mockReset()
    mocks.mediaGenerate.confirmAmbiguous.mockReset()
    mocks.mediaGenerate.cancel.mockReset()
    mocks.mediaGenerate.switchCategory.mockReset()

    mocks.speechGetStatus.mockReset().mockResolvedValue({ data: {} })
    mocks.agentApi.listTasks.mockReset().mockResolvedValue({ data: [] })
    mocks.agentApi.cancelTask.mockReset().mockResolvedValue({})
    mocks.agentApi.deleteTask.mockReset().mockResolvedValue({})
    mocks.agentApi.sendMessage.mockReset().mockResolvedValue({})
    mocks.agentApi.submitAnswers.mockReset().mockResolvedValue({})

    mocks.authFetch.mockReset().mockResolvedValue({})
    mocks.apiGet.mockReset().mockResolvedValue({ data: { pending: false } })
    mocks.apiPost.mockReset().mockResolvedValue({})
    mocks.onSSEEvent.mockReset()
    mocks.offSSEEvent.mockReset()
    mocks.warmupTrigger.mockReset().mockResolvedValue(undefined)
    mocks.injectMessage.mockReset().mockResolvedValue({})
    mocks.approvalApi.listPending.mockReset().mockResolvedValue({ data: [] })
    mocks.approvalApi.resolve.mockReset().mockResolvedValue({})
    mocks.approvalApi.getConfig.mockReset().mockResolvedValue({ data: { auto_approve_tools: [] } })
    mocks.approvalApi.updateConfig.mockReset().mockResolvedValue({})
    mocks.systemWriteLog.mockReset().mockResolvedValue(undefined)

    vi.mocked(conversationApi.list).mockReset().mockResolvedValue({ data: [CONVERSATION] } as never)
    vi.mocked(conversationApi.create).mockReset()
    vi.mocked(conversationApi.get).mockReset()
    vi.mocked(conversationApi.delete).mockReset()
    vi.mocked(conversationApi.search).mockReset()
    vi.mocked(conversationApi.getCommandState)
      .mockReset()
      .mockResolvedValue({
        data: {
          conversation_id: CONVERSATION.id,
          selected_provider_id: '',
          selected_model_id: '',
          offline: false,
          web_search_enabled: true,
          deep_research_enabled: false,
        },
      } as never)
    vi.mocked(conversationApi.patchCommandState)
      .mockReset()
      .mockResolvedValue({
        data: {
          conversation_id: CONVERSATION.id,
          selected_provider_id: '',
          selected_model_id: '',
          offline: false,
          web_search_enabled: true,
          deep_research_enabled: false,
        },
      } as never)

    vi.mocked(messageApi.list).mockReset().mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.send).mockReset()
    vi.mocked(messageApi.cancelStream).mockReset()
  })

  afterEach(() => {
    rectSpy.mockRestore()
    scrollToSpy.mockRestore()
  })

  it('preserves the upward reading position when older messages are prepended in virtual scroll mode', async () => {
    const olderMessages = Array.from({ length: 50 }, (_, index) =>
      makeAssistantMessage(
        `older-${index}`,
        `Older ${index}`,
        `2026-03-07T00:${String(index).padStart(2, '0')}:00.000Z`
      )
    )
    const currentMessages = Array.from({ length: 51 }, (_, index) =>
      makeAssistantMessage(
        `current-${index}`,
        `Current ${index}`,
        `2026-03-08T00:${String(index).padStart(2, '0')}:00.000Z`
      )
    )

    vi.mocked(messageApi.list)
      .mockResolvedValueOnce({ data: [] } as never)
      .mockResolvedValueOnce({ data: olderMessages } as never)

    const { wrapper, store } = await mountIntegratedChatView()
    const messagesContainer = wrapper.get('.chat-messages-area').element as HTMLElement

    Object.defineProperty(messagesContainer, 'clientHeight', {
      configurable: true,
      get: () => 400,
    })
    Object.defineProperty(messagesContainer, 'scrollHeight', {
      configurable: true,
      get: () => store.messages.length * 100,
    })

    store.messages = currentMessages
    store.hasMoreMessages = false
    await settleView()

    expect(wrapper.find('.virtual-scroll-container-external').exists()).toBe(true)

    messagesContainer.scrollTop = 150
    messagesContainer.dispatchEvent(new Event('scroll'))
    await settleView()
    const scrollTopBeforeLoad = messagesContainer.scrollTop

    store.hasMoreMessages = true
    await store.loadMoreMessages()
    await settleView()

    expect(vi.mocked(messageApi.list)).toHaveBeenLastCalledWith(CONVERSATION.id, 50, 50)
    expect(store.messages[0]?.id).toBe('older-0')
    expect(messagesContainer.scrollTop).toBeGreaterThanOrEqual(
      scrollTopBeforeLoad + olderMessages.length * 100
    )
    expect(messagesContainer.scrollTop).toBeLessThanOrEqual(
      scrollTopBeforeLoad + olderMessages.length * 120
    )
  })
})
