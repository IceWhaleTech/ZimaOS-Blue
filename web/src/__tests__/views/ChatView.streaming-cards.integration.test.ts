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
  cardActionSubmit: vi.fn(),
  settingsStore: {
    agentAutoConfirm: false,
    agentMode: false,
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
  taskProjectionsStore: {
    currentTasks: [] as Array<Record<string, unknown>>,
    currentActiveTasks: [] as Array<Record<string, unknown>>,
    currentTerminalTasks: [] as Array<Record<string, unknown>>,
    backgroundTasks: [] as Array<Record<string, unknown>>,
    recentOutcome: null as null | Record<string, unknown>,
    loading: false,
    hydrated: true,
    hasActiveTasks: false,
    refreshNow: vi.fn(),
    setConversation: vi.fn(),
    performTaskAction: vi.fn(),
    cancelTask: vi.fn(),
    resumeTask: vi.fn(),
    openTask: vi.fn(),
    dismissRecentOutcome: vi.fn(),
    stopPolling: vi.fn(),
  },
  mediaGenerate: {
    showPanel: { value: false },
    originalPrompt: { value: '' },
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
    submit: (...args: unknown[]) => mocks.cardActionSubmit(...args),
  },
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

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => mocks.settingsStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/deepResearchJobs', () => ({
  useDeepResearchJobsStore: () => mocks.deepResearchJobsStore,
}))

vi.mock('@/stores/taskProjections', () => ({
  useTaskProjectionsStore: () => mocks.taskProjectionsStore,
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

vi.mock('@/components/ConversationList.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ConversationList',
    template: '<div class="conversation-list-stub" />',
  })
)

vi.mock('@/components/ChatInput.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ChatInput',
    props: {
      canCancel: {
        type: Boolean,
        default: false,
      },
      showInlineCancel: {
        type: Boolean,
        default: true,
      },
    },
    emits: ['send', 'draft-change', 'inject', 'cancel', 'cancel-pre-ttft', 'warmup'],
    template: `
      <div class="chat-input-stub">
        <button
          v-if="canCancel && showInlineCancel !== false"
          type="button"
          data-testid="chat-input-inline-cancel"
          @click="$emit('cancel')"
        >
          Stop generating
        </button>
      </div>
    `,
  })
)

vi.mock('@/components/onboarding/PresetQuestions.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'PresetQuestions',
    template: '<div class="preset-questions-stub" />',
  })
)

vi.mock('@/components/VirtualScroll.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'VirtualScroll',
    template: '<div class="virtual-scroll-stub"><slot /></div>',
  })
)

vi.mock('@/components/chat/TalkMode.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'TalkMode',
    template: '<div class="talk-mode-stub" />',
  })
)

vi.mock('@/components/ToolApprovalDialog.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ToolApprovalDialog',
    template: '<div class="tool-approval-stub" />',
  })
)

vi.mock('@/components/ExecApprovalDialog.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'ExecApprovalDialog',
    template: '<div class="exec-approval-stub" />',
  })
)

vi.mock('@/components/MediaParamPanel.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'MediaParamPanel',
    template: '<div class="media-param-panel-stub" />',
  })
)

vi.mock('@/components/AgentTaskPanel.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'AgentTaskPanel',
    template: '<div class="agent-task-panel-stub" />',
  })
)

vi.mock('@/components/DeepResearchTaskDock.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'DeepResearchTaskDock',
    template: '<div class="deep-research-task-dock-stub" />',
  })
)

vi.mock('@/components/UserTaskProjectionCard.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'UserTaskProjectionCard',
    template: '<div class="agent-task-panel-stub" />',
  })
)

vi.mock('@/components/UserTaskProjectionDock.vue', () =>
  helpers.asAsyncSFCModule({
    name: 'UserTaskProjectionDock',
    template: '<div class="deep-research-task-dock-stub" />',
  })
)

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

const WEB_FETCH_URL = 'https://www.reddit.com/r/test'
const WEB_FETCH_CARD_ID = 'web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'
const BROWSER_CARD_ID = 'browser-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'
const CONVERSATION = {
  id: 'conv-1',
  title: 'Reddit flow',
  created_at: '2026-03-08T00:00:00.000Z',
  updated_at: '2026-03-08T00:00:02.000Z',
}

const mountedWrappers: Array<ReturnType<typeof mount>> = []

function makeBootstrapResponse(
  conversationId: string,
  overrides: Record<string, unknown> = {}
) {
  return {
    data: {
      command_state: {
        conversation_id: conversationId,
        selected_provider_id: '',
        selected_model_id: '',
        offline: false,
      },
      active_stream: {
        conversation_id: conversationId,
        active: false,
      },
      current_tasks: [],
      background_tasks: [],
      pending_approval: null,
      pending_question: null,
      pending_exec_approval: null,
      ...overrides,
    },
  }
}

async function defaultApiGet(path: string) {
  if (path.includes('/ask-user-question/pending')) {
    return { data: { pending: false } }
  }
  if (path.includes('/exec/approvals/pending')) {
    return { data: { pending: false } }
  }
  const bootstrapMatch = String(path).match(/^\/conversations\/([^/]+)\/bootstrap$/)
  if (bootstrapMatch) {
    return makeBootstrapResponse(bootstrapMatch[1] ?? '')
  }
  return { data: {} }
}

function makeTypelessBlock(payload: Record<string, unknown>) {
  return ['```typeless', JSON.stringify(payload), '```'].join('\n')
}

function findButtonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find((button) => button.text().includes(text))
}

async function settleView() {
  await flushPromises()
  await vi.dynamicImportSettled()
  await new Promise<void>((resolve) => {
    const scheduleFrame =
      typeof window.requestAnimationFrame === 'function'
        ? window.requestAnimationFrame.bind(window)
        : (callback: FrameRequestCallback) => window.setTimeout(() => callback(Date.now()), 0)
    scheduleFrame(() => resolve())
  })
  await flushPromises()
  await new Promise((resolve) => setTimeout(resolve, 0))
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
      { path: '/security', component: { template: '<div />' } },
    ],
  })
  await router.push('/chat?conversationId=conv-1')
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
  mountedWrappers.push(wrapper)

  await settleView()
  for (let attempt = 0; attempt < 5; attempt += 1) {
    const initialMessageLoad = vi.mocked(messageApi.list).mock.results[0]
    if (initialMessageLoad?.type === 'return') {
      await initialMessageLoad.value
      await settleView()
      break
    }
    await settleView()
  }
  return { wrapper, store: useChatStore() }
}

describe('ChatView streaming card chain integration', () => {
  beforeEach(() => {
    const activeProvider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'active',
    } as Record<string, unknown>

    delete (globalThis as Record<string, unknown>).__zima_chat_card_disclosure_state_v1__

    localStorageMock.clear()
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
    Object.defineProperty(window.navigator, 'platform', { value: 'MacIntel', configurable: true })

    vi.clearAllMocks()

    mocks.sseConnect.mockReset().mockResolvedValue(undefined)
    mocks.sseDisconnect.mockReset()
    mocks.cardActionSubmit.mockReset()

    mocks.settingsStore.agentAutoConfirm = false
    mocks.settingsStore.agentMode = false
    mocks.settingsStore.showToolDetails = true
    mocks.settingsStore.selectedProvider = 'openai'
    mocks.settingsStore.selectedModel = 'gpt-4o-mini'
    mocks.settingsStore.temperature = 0.7
    mocks.settingsStore.maxTokens = 8192
    mocks.settingsStore.fetchTools.mockReset().mockResolvedValue(undefined)
    mocks.settingsStore.updateFromPoolProviders.mockReset()
    mocks.settingsStore.setAgentAutoConfirm.mockReset()
    mocks.settingsStore.setAgentMode.mockReset()
    mocks.settingsStore.setShowToolDetails.mockReset()

    mocks.providerPoolStore.activeProviders = [activeProvider]
    mocks.providerPoolStore.cloudProviders = [activeProvider]
    mocks.providerPoolStore.enabledProviders = [activeProvider]
    mocks.providerPoolStore.hasCloudProviders = true
    mocks.providerPoolStore.hasLocalProviders = false
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.models = []
    mocks.providerPoolStore.providers = [activeProvider]
    mocks.providerPoolStore.routingMode = 'auto'
    mocks.providerPoolStore.trialProviders = []
    mocks.providerPoolStore.trialQuota = null
    mocks.providerPoolStore.getProviderDisplayName
      .mockReset()
      .mockImplementation((providerId: string) => providerId)
    mocks.providerPoolStore.fetchProviders.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.setRoutingMode.mockReset().mockResolvedValue(undefined)

    mocks.mediaGenerate.showPanel.value = false
    mocks.mediaGenerate.originalPrompt.value = ''
    mocks.mediaGenerate.intent.value = null
    mocks.mediaGenerate.models.value = []
    mocks.mediaGenerate.selectedModel.value = ''
    mocks.mediaGenerate.generating.value = false
    mocks.mediaGenerate.ambiguous.value = false
    mocks.mediaGenerate.task.value = null
    mocks.mediaGenerate.classify.mockReset()
    mocks.mediaGenerate.generate.mockReset()
    mocks.mediaGenerate.reset.mockReset()
    mocks.mediaGenerate.confirmAmbiguous.mockReset()
    mocks.mediaGenerate.cancel.mockReset()
    mocks.mediaGenerate.switchCategory.mockReset()

    mocks.speechGetStatus.mockReset().mockResolvedValue({ data: {} })
    mocks.authFetch.mockReset().mockResolvedValue({})
    mocks.apiGet.mockReset().mockImplementation(defaultApiGet)
    mocks.apiPost.mockReset().mockResolvedValue({})
    mocks.onSSEEvent.mockReset()
    mocks.offSSEEvent.mockReset()

    mocks.notificationStore.success.mockReset()
    mocks.notificationStore.error.mockReset()
    mocks.notificationStore.info.mockReset()
    mocks.notificationStore.remove.mockReset()

    mocks.warmupTrigger.mockReset().mockResolvedValue(undefined)
    mocks.injectMessage.mockReset().mockResolvedValue({})
    mocks.approvalApi.resolve.mockReset().mockResolvedValue({})
    mocks.approvalApi.getConfig.mockReset().mockResolvedValue({ data: { auto_approve_tools: [] } })
    mocks.approvalApi.updateConfig.mockReset().mockResolvedValue({})
    mocks.systemWriteLog.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.fetchActiveJobs.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.currentTasks = []
    mocks.taskProjectionsStore.currentActiveTasks = []
    mocks.taskProjectionsStore.currentTerminalTasks = []
    mocks.taskProjectionsStore.backgroundTasks = []
    mocks.taskProjectionsStore.recentOutcome = null
    mocks.taskProjectionsStore.refreshNow.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.setConversation.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.performTaskAction.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.cancelTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.resumeTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.openTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.dismissRecentOutcome.mockReset()
    mocks.taskProjectionsStore.stopPolling.mockReset()
    mocks.deepResearchJobsStore.handleGlobalEvent.mockReset()
    mocks.deepResearchJobsStore.applyJobSnapshot.mockReset()
    mocks.deepResearchJobsStore.openJob.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.cancelJob.mockReset().mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.consumePendingFocusJobId.mockReset()

    vi.mocked(conversationApi.list)
      .mockReset()
      .mockResolvedValue({ data: [CONVERSATION] } as never)
    vi.mocked(conversationApi.create).mockReset()
    vi.mocked(conversationApi.get).mockReset()
    vi.mocked(conversationApi.delete).mockReset()
    vi.mocked(conversationApi.search).mockReset()
    vi.mocked(conversationApi.patchCommandState)
      .mockReset()
      .mockResolvedValue({
        data: {
          conversation_id: 'conv-1',
          selected_provider_id: '',
          selected_model_id: '',
          offline: false,
        },
      } as never)

    vi.mocked(messageApi.list)
      .mockReset()
      .mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.send).mockReset()
    vi.mocked(messageApi.cancelStream).mockReset()
  })

  afterEach(async () => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    await flushPromises()
  })

  it('sends a question from ChatView and renders the completed assistant reply', async () => {
    const provider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'active',
    } as Record<string, unknown>
    const question = 'What is 2 + 2?'
    const answer = '2 + 2 equals 4.'
    const persistedMessages = [
      {
        id: 'msg-user-1',
        conversation_id: 'conv-1',
        role: 'user',
        content: question,
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-1',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: answer,
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.enabledProviders = [provider]
    mocks.providerPoolStore.activeProviders = [provider]

    vi.mocked(messageApi.list)
      .mockResolvedValueOnce({ data: [] } as never)
      .mockResolvedValue({ data: persistedMessages } as never)

    mocks.sseConnect.mockImplementationOnce(async (conversationId, request, options: any) => {
      expect(conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: question,
        })
      )

      options.onMessage({ delta: answer, done: false })
      options.onNewMessage?.(1)
      options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
    })

    const { wrapper, store } = await mountIntegratedChatView()

    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('send', question, [])
    await settleView()

    expect(mocks.sseConnect).toHaveBeenCalledTimes(1)
    expect(store.sending).toBe(false)
    expect(store.streaming).toBe(false)
    expect(store.messages).toEqual(persistedMessages)
    expect(wrapper.text()).toContain(question)
    expect(wrapper.text()).toContain(answer)
  })

  it('keeps the active assistant streaming state visible after mid-stream injection appends a temp user message', async () => {
    const { wrapper, store } = await mountIntegratedChatView()

    store.messages = [
      {
        id: 'streaming-1',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: '',
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]
    store.streaming = true
    store.statusStartedAt = Date.now() - 1500

    await store.injectMessage('补充一点背景')
    await settleView()

    expect(mocks.injectMessage).toHaveBeenCalledWith('conv-1', '补充一点背景')
    expect(wrapper.text()).toContain('补充一点背景')
    expect(wrapper.find('.chat-stream-status-rail').exists()).toBe(true)
    expect(wrapper.find('.assistant-status-bar').exists()).toBe(false)
  })

  it('restores a server-reported active stream with persisted preview content after the view remounts', async () => {
    const persistedMessages = [
      {
        id: 'msg-assistant-1',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: '- [x] 收集信息\n- [ ] 写总结\n\n我继续执行第二步。',
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    vi.mocked(messageApi.list).mockResolvedValue({ data: persistedMessages } as never)
    mocks.apiGet.mockImplementation(async (path: string) => {
      if (path === '/conversations/conv-1/bootstrap') {
        return makeBootstrapResponse('conv-1', {
          active_stream: {
            conversation_id: 'conv-1',
            active: true,
            stream_id: 'stream-preview-1',
          },
        })
      }
      return defaultApiGet(path)
    })

    const { wrapper, store } = await mountIntegratedChatView()

    expect(store.streaming).toBe(true)
    expect(store.sending).toBe(false)
    expect(store.streamingContent).toContain('我继续执行第二步。')
    expect(wrapper.text()).toContain('我继续执行第二步。')
    expect(wrapper.find('.chat-stream-status-rail').exists()).toBe(true)
    expect(wrapper.find('[data-testid="chat-activity-dock"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
  })

  it('shows an executing rail and stop control when the server reports an active stream without preview text', async () => {
    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
    mocks.apiGet.mockImplementation(async (path: string) => {
      if (path === '/conversations/conv-1/bootstrap') {
        return makeBootstrapResponse('conv-1', {
          active_stream: {
            conversation_id: 'conv-1',
            active: true,
            stream_id: 'stream-live-1',
          },
        })
      }
      return defaultApiGet(path)
    })

    const { wrapper, store } = await mountIntegratedChatView()

    expect(store.streaming).toBe(true)
    expect(store.sending).toBe(false)
    expect(store.toolExecuting).toBe(true)
    expect(store.streamUIState.phase).toBe('executing')
    expect(wrapper.find('.chat-stream-status-rail').exists()).toBe(true)
    expect(wrapper.find('[data-testid="chat-activity-dock"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Processing')
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
  })

  it('does not render the previous assistant reply as active preview when only the latest user turn is persisted', async () => {
    vi.mocked(messageApi.list).mockResolvedValue({
      data: [
        {
          id: 'msg-assistant-prev',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: '上一轮已经完成的回复',
          created_at: '2026-03-08T00:00:00.000Z',
        },
        {
          id: 'msg-user-latest',
          conversation_id: 'conv-1',
          role: 'user',
          content: '继续执行新的任务',
          created_at: '2026-03-08T00:00:01.000Z',
        },
      ],
    } as never)
    mocks.apiGet.mockImplementation(async (path: string) => {
      if (path === '/conversations/conv-1/bootstrap') {
        return makeBootstrapResponse('conv-1', {
          active_stream: {
            conversation_id: 'conv-1',
            active: true,
            stream_id: 'stream-live-2',
          },
        })
      }
      return defaultApiGet(path)
    })

    const { wrapper, store } = await mountIntegratedChatView()

    expect(store.streaming).toBe(true)
    expect(store.toolExecuting).toBe(true)
    expect(store.streamUIState.phase).toBe('executing')
    expect(store.streamingContent).toBe('')
    expect(store.messages).toHaveLength(3)
    expect(store.messages[2]?.id.startsWith('streaming-')).toBe(true)
    expect(store.messages[2]?.content).toBe('')
    expect(wrapper.text()).toContain('上一轮已经完成的回复')
    expect(wrapper.text()).toContain('继续执行新的任务')
    expect(wrapper.text()).toContain('Processing')
    expect(wrapper.find('.chat-stream-status-rail').exists()).toBe(true)
    expect(wrapper.get('[data-testid="chat-input-inline-cancel"]').exists()).toBe(true)
  })

  it('renders streamed web-fetch and browser cards with the real chat store, then submits both card actions', async () => {
    const webFetchBlock = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Sign in',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Log in to continue',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning: 'page appears to be a login wall; use browser or pass browser_target_id',
      warning_code: 'login_wall',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: { url: WEB_FETCH_URL },
        },
      ],
    })
    const browserBlock = makeTypelessBlock({
      type: 'result',
      id: BROWSER_CARD_ID,
      title: 'Browser page',
      status: 'info',
      message: 'Interactive page opened in the browser session.',
      details: [
        { label: 'url', value: WEB_FETCH_URL },
        { label: 'browser_target_id', value: 'tab-42' },
      ],
      actions: [
        {
          id: 'extract_with_web_fetch',
          label: 'Extract readable content',
          variant: 'primary',
          form_data: {
            url: WEB_FETCH_URL,
            browser_target_id: 'tab-42',
          },
        },
      ],
    })
    const persistedMessages = [
      {
        id: 'msg-user-1',
        conversation_id: 'conv-1',
        role: 'user',
        content: `Inspect ${WEB_FETCH_URL}`,
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-1',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: webFetchBlock,
        created_at: '2026-03-08T00:00:01.000Z',
      },
      {
        id: 'msg-assistant-2',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: browserBlock,
        created_at: '2026-03-08T00:00:02.000Z',
      },
    ]

    vi.mocked(messageApi.list)
      .mockResolvedValueOnce({ data: [] } as never)
      .mockResolvedValueOnce({ data: persistedMessages } as never)

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
      expect(_conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: `Inspect ${WEB_FETCH_URL}`,
        })
      )

      options.onMessage({ delta: webFetchBlock, done: false })
      options.onNewMessage?.(1)
      options.onMessage({ delta: browserBlock, done: false })
      options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
    })

    mocks.cardActionSubmit
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Open ${WEB_FETCH_URL} with the browser tool.`,
        },
      })
      .mockResolvedValueOnce({
        data: {
          success: true,
          message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
        },
      })

    const { wrapper, store } = await mountIntegratedChatView()

    expect(store.currentConversationId).toBe('conv-1')
    expect(vi.mocked(messageApi.list)).toHaveBeenCalledWith('conv-1', 50, 0)

    await store.sendMessage(`Inspect ${WEB_FETCH_URL}`)
    await settleView()

    expect(mocks.sseConnect).toHaveBeenCalledTimes(1)
    expect(vi.mocked(messageApi.list)).toHaveBeenCalledTimes(2)
    expect(store.messages).toEqual(persistedMessages)
    expect(wrapper.text()).toContain('Sign in')
    expect(wrapper.text()).toContain('Browser page')

    await wrapper.get(`[id="${WEB_FETCH_CARD_ID}"] button`).trigger('click')
    await settleView()

    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    const extractButton = findButtonByText(wrapper, 'Extract readable content')

    expect(useBrowserButton?.exists()).toBe(true)
    expect(extractButton?.exists()).toBe(true)

    const sendSpy = vi.spyOn(store, 'sendMessage').mockResolvedValue(undefined)

    await useBrowserButton!.trigger('click')
    await settleView()

    await extractButton!.trigger('click')
    await settleView()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(2)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(1, 'conv-1', 'msg-assistant-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(2, 'conv-1', 'msg-assistant-2', {
      card_id: BROWSER_CARD_ID,
      action_id: 'extract_with_web_fetch',
      action_label: 'Extract readable content',
      card_type: 'result',
      card_title: 'Browser page',
      form_data: {
        url: WEB_FETCH_URL,
        browser_target_id: 'tab-42',
      },
    })
    expect(sendSpy).toHaveBeenNthCalledWith(1, `Open ${WEB_FETCH_URL} with the browser tool.`)
    expect(sendSpy).toHaveBeenNthCalledWith(
      2,
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`
    )
  })

  it('keeps web search, browser progress, and web-fetch cards collapsed until expanded', async () => {
    const searchBlock = makeTypelessBlock({
      type: 'search',
      id: 'search-chain',
      query: 'OpenAI latest updates',
      results: [
        {
          title: 'OpenAI blog',
          url: 'https://openai.com/blog',
          description: 'Latest announcements and product updates.',
        },
      ],
    })
    const browserProgressBlock = makeTypelessBlock({
      type: 'browser-progress',
      id: 'browser-progress-chain',
      steps: [
        {
          step: 'navigate',
          name: 'Navigating',
          status: 'completed',
          url: 'https://openai.com/blog',
        },
        {
          step: 'snapshot',
          name: 'Reading page',
          status: 'running',
          url: 'https://openai.com/blog',
        },
      ],
    })
    const webFetchBlock = makeTypelessBlock({
      type: 'web-fetch',
      id: 'web-fetch-chain',
      title: 'web_fetch',
      url: 'https://openai.com/blog',
      status: 'success',
      content: 'Expanded page content from the OpenAI blog.',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
        },
      ],
    })

    const persistedMessages = [
      {
        id: 'msg-user-chain',
        conversation_id: 'conv-1',
        role: 'user',
        content: 'Check the latest OpenAI updates',
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-search',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: searchBlock,
        created_at: '2026-03-08T00:00:01.000Z',
      },
      {
        id: 'msg-assistant-browser',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: browserProgressBlock,
        created_at: '2026-03-08T00:00:02.000Z',
      },
      {
        id: 'msg-assistant-fetch',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: webFetchBlock,
        created_at: '2026-03-08T00:00:03.000Z',
      },
    ]

    vi.mocked(messageApi.list).mockResolvedValueOnce({ data: persistedMessages } as never)

    const { wrapper } = await mountIntegratedChatView()
    const searchCard = wrapper.get('#search-chain')
    const browserProgressCard = wrapper.get('#browser-progress-chain')
    const webFetchCard = wrapper.get('#web-fetch-chain')

    expect(wrapper.text()).toContain('OpenAI latest updates')
    expect(searchCard.text()).toContain('Latest announcements and product updates.')
    expect(browserProgressCard.text()).not.toContain('Navigating')
    expect(webFetchCard.text()).not.toContain('Expanded page content from the OpenAI blog.')

    await searchCard.get('button').trigger('click')
    await settleView()
    expect(searchCard.text()).toContain('Latest announcements and product updates.')

    await browserProgressCard.get('button').trigger('click')
    await settleView()
    expect(browserProgressCard.text()).toContain('Navigating')
    expect(browserProgressCard.text()).toContain('Reading page')

    await webFetchCard.get('button').trigger('click')
    await settleView()
    expect(webFetchCard.text()).toContain('Expanded page content from the OpenAI blog.')
    expect(webFetchCard.text()).toContain('Use browser')
  })

  it('keeps browser progress visible when final local finalization only returns the web-fetch result block', async () => {
    const browserProgressBlock = makeTypelessBlock({
      type: 'browser-progress',
      id: 'browser-progress-chain',
      steps: [
        {
          step: 'navigate',
          name: 'Navigating',
          status: 'completed',
          url: 'https://openai.com/blog',
        },
        {
          step: 'snapshot',
          name: 'Reading page',
          status: 'running',
          url: 'https://openai.com/blog',
        },
      ],
    })
    const webFetchBlock = makeTypelessBlock({
      type: 'web-fetch',
      id: 'web-fetch-chain',
      title: 'web_fetch',
      url: 'https://openai.com/blog',
      status: 'success',
      content: 'Expanded page content from the OpenAI blog.',
    })

    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
      expect(_conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: 'Check the latest OpenAI updates',
        })
      )

      options.onMessage({ delta: browserProgressBlock, done: false })
      options.onComplete?.({
        done: true,
        message_id: 'msg-assistant-browser-final',
        content: webFetchBlock,
        provider: 'openai',
        model: 'gpt-4o-mini',
      })
    })

    const { wrapper, store } = await mountIntegratedChatView()

    await store.sendMessage('Check the latest OpenAI updates')
    await settleView()

    expect(wrapper.find('#browser-progress-chain').exists()).toBe(true)
    expect(wrapper.find('#web-fetch-chain').exists()).toBe(true)

    await wrapper.get('#browser-progress-chain button').trigger('click')
    await settleView()
    expect(wrapper.text()).toContain('Navigating')
    expect(wrapper.text()).toContain('Reading page')

    await wrapper.get('#web-fetch-chain button').trigger('click')
    await settleView()
    expect(wrapper.text()).toContain('Expanded page content from the OpenAI blog.')
  })

  it('keeps fallback boilerplate when chat history contains extracted tool summaries', async () => {
    const searchBlock = makeTypelessBlock({
      type: 'search',
      id: 'search-openclaw',
      query: 'OpenClaw recent updates',
      results: [
        {
          title: 'OpenClaw Release Notes',
          url: 'https://github.com/opendungeons/openclaw/releases',
          description: 'recent release notes',
        },
      ],
    })

    const persistedMessages = [
      {
        id: 'msg-user-openclaw',
        conversation_id: 'conv-1',
        role: 'user',
        content: '帮我调研一下最近一周 openclaw 的动向吧',
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-openclaw',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: [
          '工具执行已完成，但最终总结生成失败。以下是基于工具结果整理的简要摘要：',
          '',
          'Web search fallback results for "OpenClaw 最近动向":',
          '',
          searchBlock,
          '',
          '原始 stdout/stderr/error 字段未包含在这条简要摘要中。如需我重试完整总结，请回复“重试总结”。',
        ].join('\n'),
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    vi.mocked(messageApi.list).mockResolvedValue({ data: persistedMessages } as never)

    const { wrapper, store } = await mountIntegratedChatView()

    expect(store.currentConversationId).toBe('conv-1')
    expect(wrapper.text()).toContain('Web search fallback results for "OpenClaw 最近动向"')
    expect(wrapper.text()).toContain('工具执行已完成，但最终总结生成失败')
    expect(wrapper.text()).toContain('简要摘要')
    expect(wrapper.text()).toContain('原始 stdout/stderr/error 字段未包含在这条简要摘要中')

    await wrapper.get('#search-openclaw button').trigger('click')
    await settleView()

    expect(wrapper.get('#search-openclaw').text()).toContain('OpenClaw Release Notes')
  })

  it('renders a streamed browser_required web-fetch card and still routes use_browser through card actions', async () => {
    const browserRequiredBlock = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Protected page',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Use a browser session to read this page.',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning:
        'page requires a browser session for readable extraction; switch to browser or reuse browser_target_id',
      warning_code: 'browser_required',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: { url: WEB_FETCH_URL },
        },
      ],
    })
    const persistedMessages = [
      {
        id: 'msg-user-1',
        conversation_id: 'conv-1',
        role: 'user',
        content: 'Inspect ' + WEB_FETCH_URL,
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-1',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: browserRequiredBlock,
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    vi.mocked(messageApi.list)
      .mockResolvedValueOnce({ data: [] } as never)
      .mockResolvedValueOnce({ data: persistedMessages } as never)

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
      expect(_conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: 'Inspect ' + WEB_FETCH_URL,
        })
      )

      options.onMessage({ delta: browserRequiredBlock, done: false })
      options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
    })

    mocks.cardActionSubmit.mockResolvedValueOnce({
      data: {
        success: true,
        message: 'Open ' + WEB_FETCH_URL + ' with the browser tool.',
      },
    })

    const { wrapper, store } = await mountIntegratedChatView()

    await store.sendMessage('Inspect ' + WEB_FETCH_URL)
    await settleView()

    expect(store.messages).toEqual(persistedMessages)
    const browserRequiredCard = wrapper.get(`[id="${WEB_FETCH_CARD_ID}"]`)
    expect(browserRequiredCard.text()).toContain('Protected page')
    expect(browserRequiredCard.text()).not.toContain('browser session')

    await browserRequiredCard.get('button').trigger('click')
    await settleView()

    expect(browserRequiredCard.text()).toContain('browser session')

    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    expect(useBrowserButton?.exists()).toBe(true)

    const sendSpy = vi.spyOn(store, 'sendMessage').mockResolvedValue(undefined)

    await useBrowserButton!.trigger('click')
    await settleView()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-assistant-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Protected page',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(sendSpy).toHaveBeenCalledWith('Open ' + WEB_FETCH_URL + ' with the browser tool.')
  })

  it('renders a streamed challenge web-fetch card and still routes use_browser through card actions', async () => {
    const challengeBlock = makeTypelessBlock({
      type: 'web-fetch',
      id: WEB_FETCH_CARD_ID,
      title: 'Verification required',
      status: 'warning',
      url: WEB_FETCH_URL,
      content: 'Complete the verification to continue.',
      content_type: 'text/html',
      extract_mode: 'text',
      extractor: 'html',
      warning:
        'page appears to require a verification challenge; switch to browser or reuse browser_target_id',
      warning_code: 'challenge',
      actions: [
        {
          id: 'use_browser',
          label: 'Use browser',
          variant: 'primary',
          form_data: { url: WEB_FETCH_URL },
        },
      ],
    })
    const persistedMessages = [
      {
        id: 'msg-user-1',
        conversation_id: 'conv-1',
        role: 'user',
        content: 'Inspect ' + WEB_FETCH_URL,
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-1',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: challengeBlock,
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    vi.mocked(messageApi.list)
      .mockResolvedValueOnce({ data: [] } as never)
      .mockResolvedValueOnce({ data: persistedMessages } as never)

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
      expect(_conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: 'Inspect ' + WEB_FETCH_URL,
        })
      )

      options.onMessage({ delta: challengeBlock, done: false })
      options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
    })

    mocks.cardActionSubmit.mockResolvedValueOnce({
      data: {
        success: true,
        message: 'Open ' + WEB_FETCH_URL + ' with the browser tool.',
      },
    })

    const { wrapper, store } = await mountIntegratedChatView()

    await store.sendMessage('Inspect ' + WEB_FETCH_URL)
    await settleView()

    expect(store.messages).toEqual(persistedMessages)
    await settleView()
    const challengeCard = wrapper.get(`[id="${WEB_FETCH_CARD_ID}"]`)
    expect(challengeCard.text()).toContain('Verification required')
    expect(challengeCard.text()).not.toContain('verification challenge')

    await challengeCard.get('button').trigger('click')
    await settleView()

    expect(challengeCard.text()).toContain('verification challenge')

    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    expect(useBrowserButton?.exists()).toBe(true)

    const sendSpy = vi.spyOn(store, 'sendMessage').mockResolvedValue(undefined)

    await useBrowserButton!.trigger('click')
    await settleView()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-assistant-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Verification required',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(sendSpy).toHaveBeenCalledWith('Open ' + WEB_FETCH_URL + ' with the browser tool.')
  })

  it('renders persisted knowledge-base deep research cards with workflow and source artifacts', async () => {
    const deepResearchBlock = makeTypelessBlock({
      type: 'deep-research',
      id: 'deep-research-kb-1',
      query: 'EU AI Act provider obligations knowledge base',
      mode: 'deep',
      report_style: 'knowledge_base',
      answer: 'Provider obligations are organized by role, timeline, and evidence coverage.',
      evidence_count: 5,
      support_count: 4,
      conflict_count: 1,
      citation_coverage: 0.92,
      confidence: 0.81,
      status: 'completed',
      iterations: 4,
      citations: [
        {
          title: 'EU AI Act consolidated text',
          url: 'https://eur-lex.europa.eu/eli/reg/2024/1689/oj',
        },
      ],
      open_questions: ['Need delegated acts publication date'],
      verification_summary: {
        resolved_count: 4,
        conflicted_count: 1,
        insufficient_count: 0,
      },
      workflow_phases: [
        { id: 'scope', label: 'Scope', status: 'completed' },
        { id: 'sources', label: 'Sources', status: 'completed' },
        { id: 'extraction', label: 'Extraction', status: 'current' },
      ],
      source_inventory: [
        {
          source_id: 'src-1',
          title: 'EU AI Act consolidated text',
          url: 'https://eur-lex.europa.eu/eli/reg/2024/1689/oj',
          domain: 'eur-lex.europa.eu',
          source_type: 'law',
          fetched_at: '2026-03-08T00:00:00.000Z',
          relevance_score: 0.97,
          credibility_score: 0.99,
        },
      ],
      coverage_summary: {
        task_count: 6,
        evidence_count: 5,
        distinct_domain_count: 2,
        open_question_count: 1,
      },
      object_map: [
        {
          id: 'provider-obligations',
          label: 'Provider obligations',
          task_count: 3,
          time_windows: ['2025-2026'],
          status_counts: { resolved: 2, conflicted: 1 },
          questions: ['Which GPAI duties apply to open-weight models?'],
        },
      ],
    })

    const persistedMessages = [
      {
        id: 'msg-user-kb',
        conversation_id: 'conv-1',
        role: 'user',
        content: 'Build a knowledge base for EU AI Act provider obligations.',
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-kb',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: deepResearchBlock,
        created_at: '2026-03-08T00:00:01.000Z',
      },
    ]

    vi.mocked(messageApi.list).mockResolvedValue({ data: persistedMessages } as never)

    const { wrapper, store } = await mountIntegratedChatView()

    expect(store.currentConversationId).toBe('conv-1')
    expect(wrapper.find('#deep-research-kb-1').exists()).toBe(true)
    expect(wrapper.text()).toContain('EU AI Act provider obligations knowledge base')
    expect(
      wrapper
        .get('#deep-research-kb-1 [data-testid="deep-research-summary-toggle"]')
        .attributes('aria-expanded')
    ).toBe('false')
    expect(wrapper.text()).not.toContain('Workflow phases')

    await wrapper
      .get('#deep-research-kb-1 [data-testid="deep-research-summary-toggle"]')
      .trigger('click')
    await settleView()

    expect(wrapper.text()).toContain('Workflow phases')
    expect(wrapper.text()).toContain('Scope')
    expect(wrapper.text()).toContain('Source Inventory')
    expect(wrapper.text()).toContain('EU AI Act consolidated text')
    expect(wrapper.text()).toContain('Coverage Summary')
    expect(wrapper.text()).toContain('Object Map')
    expect(wrapper.text()).toContain('Provider obligations')
    expect(wrapper.text()).toContain('Need delegated acts publication date')
  })

  it('renders streamed deep research process cards as a single timeline bubble and keeps the final result separate', async () => {
    const progressBlock = makeTypelessBlock({
      type: 'deep-research-progress',
      id: 'dr-progress-1',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'EU AI Act provider obligations',
      mode: 'deep',
      stage: 'retrieve',
      status: 'running',
      progress: 38,
      iteration: 1,
      latest_action: 'initial_retrieve',
    })
    const planningBlock = makeTypelessBlock({
      type: 'deep-research-event',
      id: 'dr-event-1',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'EU AI Act provider obligations',
      mode: 'deep',
      event_kind: 'planning',
      status: 'info',
      summary: 'Planned 5 research task(s)',
      iteration: 1,
      task_count: 5,
      tasks: [{ question: 'Review provider duties', axis: 'official' }],
    })
    const sourceBlock = makeTypelessBlock({
      type: 'deep-research-event',
      id: 'dr-event-2',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'EU AI Act provider obligations',
      mode: 'deep',
      event_kind: 'source',
      status: 'info',
      summary: 'Collected 3 source(s)',
      iteration: 1,
      parallelism: 4,
      sources: [
        {
          title: 'EU AI Act text',
          url: 'https://eur-lex.europa.eu/eli/reg/2024/1689/oj',
          domain: 'eur-lex.europa.eu',
        },
      ],
    })
    const finalResultBlock = makeTypelessBlock({
      type: 'deep-research',
      id: 'dr-result-1',
      query: 'EU AI Act provider obligations',
      mode: 'deep',
      status: 'completed',
      answer: 'Provider obligations are organized by role and timeline.',
      iterations: 2,
      evidence_count: 3,
      source_inventory: [
        {
          source_id: 'src-1',
          title: 'EU AI Act text',
          url: 'https://eur-lex.europa.eu/eli/reg/2024/1689/oj',
          domain: 'eur-lex.europa.eu',
          source_type: 'law',
          fetched_at: '2026-03-08T00:00:00.000Z',
          relevance_score: 0.97,
          credibility_score: 0.99,
        },
      ],
    })

    const persistedMessages = [
      {
        id: 'msg-user-dr',
        conversation_id: 'conv-1',
        role: 'user',
        content: 'Research EU AI Act provider obligations.',
        created_at: '2026-03-08T00:00:00.000Z',
      },
      {
        id: 'msg-assistant-dr-process',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: [progressBlock, planningBlock, sourceBlock].join('\n\n'),
        created_at: '2026-03-08T00:00:01.000Z',
      },
      {
        id: 'msg-assistant-dr-result',
        conversation_id: 'conv-1',
        role: 'assistant',
        content: finalResultBlock,
        created_at: '2026-03-08T00:00:02.000Z',
      },
    ]

    vi.mocked(messageApi.list)
      .mockResolvedValueOnce({ data: [] } as never)
      .mockResolvedValueOnce({ data: persistedMessages } as never)

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
      expect(_conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: 'Research EU AI Act provider obligations.',
        })
      )

      options.onMessage({ delta: `${progressBlock}\n\n`, done: false })
      options.onMessage({ delta: `${planningBlock}\n\n`, done: false })
      options.onMessage({ delta: sourceBlock, done: false })
      options.onNewMessage?.(1)
      options.onMessage({ delta: finalResultBlock, done: false })
      options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
    })

    const { wrapper, store } = await mountIntegratedChatView()

    await store.sendMessage('Research EU AI Act provider obligations.')
    await settleView()

    expect(store.messages).toEqual(persistedMessages)
    expect(wrapper.find('#dr-progress-1').exists()).toBe(true)
    expect(wrapper.find('#dr-result-1').exists()).toBe(true)
    expect(wrapper.text()).toContain('Planned tasks')
    expect(wrapper.text()).toContain('Review provider duties')
    expect(wrapper.text()).toContain('EU AI Act text')
    expect(wrapper.text()).toContain('Provider obligations are organized by role and timeline.')
    expect(
      wrapper
        .get('#dr-result-1 [data-testid="deep-research-summary-toggle"]')
        .attributes('aria-expanded')
    ).toBe('false')

    await wrapper.get('#dr-result-1 [data-testid="deep-research-summary-toggle"]').trigger('click')
    await settleView()

    expect(wrapper.text()).toContain('Source Inventory')
    expect(wrapper.text()).toContain('EU AI Act text')

    const timelineRoot = wrapper.get('#dr-progress-1').element as HTMLElement
    expect(timelineRoot.closest('.chat-assistant-bubble')).not.toBeNull()
  })

  it('keeps deep research process cards visible when final local finalization only returns the final result block', async () => {
    const initialProgressBlock = makeTypelessBlock({
      type: 'deep-research-progress',
      id: 'deep-research-progress-job-1',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'Deep research this topic',
      mode: 'deep',
      stage: 'retrieve',
      status: 'running',
      progress: 42,
      iteration: 1,
      latest_action: 'initial_retrieve',
    })
    const planningBlock = makeTypelessBlock({
      type: 'deep-research-event',
      id: 'deep-research-event-job-1-01',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'Deep research this topic',
      mode: 'deep',
      event_kind: 'planning',
      status: 'info',
      summary: 'Planned 4 research task(s)',
      iteration: 1,
      task_count: 4,
    })
    const fullContextBlock = makeTypelessBlock({
      type: 'deep-research-progress',
      id: 'deep-research-progress-job-1',
      job_id: 'job-1',
      conversation_id: 'conv-1',
      query: 'Deep research this topic',
      mode: 'deep',
      stage: 'fullcontext',
      status: 'running',
      progress: 91,
      iteration: 1,
      latest_action: 'synthesize_full_context',
    })
    const finalResultBlock = makeTypelessBlock({
      type: 'deep-research',
      id: 'deep-research-result-job-1',
      job_id: 'job-1',
      query: 'Deep research this topic',
      mode: 'deep',
      status: 'completed',
      answer: 'Final synthesized answer',
    })

    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
      expect(_conversationId).toBe('conv-1')
      expect(request).toEqual(
        expect.objectContaining({
          message: 'Deep research this topic',
        })
      )

      options.onMessage({ delta: `${initialProgressBlock}\n\n`, done: false })
      options.onMessage({ delta: `${planningBlock}\n\n`, done: false })
      options.onMessage({ delta: fullContextBlock, done: false })
      options.onComplete?.({
        done: true,
        message_id: 'msg-assistant-dr-final',
        content: finalResultBlock,
        provider: 'openai',
        model: 'gpt-4o-mini',
      })
    })

    const { wrapper, store } = await mountIntegratedChatView()

    await store.sendMessage('Deep research this topic')
    await settleView()

    expect(wrapper.find('#deep-research-event-job-1-01').exists()).toBe(true)
    expect(wrapper.find('#deep-research-result-job-1').exists()).toBe(true)
    expect(wrapper.text()).toContain('Planned tasks: 4')
    expect(wrapper.text()).toContain('Final synthesized answer')

    const timelineRoot = wrapper.get('#deep-research-event-job-1-01').element as HTMLElement
    expect(timelineRoot.closest('[data-message-id="msg-assistant-dr-final"]')).not.toBeNull()
  })

  it('opens the app sidebar from mobile chat view when the global header is hidden', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 390, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', {
      value:
        'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1',
      configurable: true,
    })
    Object.defineProperty(window.navigator, 'platform', { value: 'iPhone', configurable: true })
    window.history.replaceState({}, '', '/chat?conversationId=conv-1')

    const pinia = createPinia()
    setActivePinia(pinia)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/chat', component: { template: '<div />' } },
        { path: '/settings', component: { template: '<div />' } },
        { path: '/security', component: { template: '<div />' } },
      ],
    })
    router.push('/chat')
    await router.isReady()

    const toggleAppSidebar = vi.fn()
    const wrapper = mount(ChatView, {
      global: {
        plugins: [pinia, i18n, router],
        provide: {
          toggleAppSidebar,
        },
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    await settleView()

    const globalNavButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('title') === i18n.global.t('nav.expandSidebar'))
    expect(globalNavButton?.exists()).toBe(true)

    await globalNavButton!.trigger('click')

    expect(toggleAppSidebar).toHaveBeenCalledTimes(1)
  })

  it('opens the app sidebar from narrow desktop chat view without using the global layout button', async () => {
    Object.defineProperty(window, 'innerWidth', { value: 760, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', {
      value:
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36',
      configurable: true,
    })
    Object.defineProperty(window.navigator, 'platform', { value: 'MacIntel', configurable: true })
    window.history.replaceState({}, '', '/chat')

    const pinia = createPinia()
    setActivePinia(pinia)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/chat', component: { template: '<div />' } },
        { path: '/settings', component: { template: '<div />' } },
        { path: '/security', component: { template: '<div />' } },
      ],
    })
    router.push('/chat')
    await router.isReady()

    const toggleAppSidebar = vi.fn()
    const wrapper = mount(ChatView, {
      global: {
        plugins: [pinia, i18n, router],
        provide: {
          toggleAppSidebar,
        },
        stubs: {
          Teleport: true,
          Transition: true,
        },
      },
    })

    await settleView()

    const appSidebarButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('title') === i18n.global.t('nav.expandSidebar'))
    expect(appSidebarButton?.exists()).toBe(true)

    await appSidebarButton!.trigger('click')

    expect(toggleAppSidebar).toHaveBeenCalledTimes(1)
  })
})
