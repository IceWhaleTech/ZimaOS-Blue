import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { reactive } from 'vue'
import ChatView from '@/views/ChatView.vue'
import { i18n, setLocale } from '@/i18n'

const mocks = vi.hoisted(() => ({
  chatInputSetInput: vi.fn(),
  chatStore: {
    awaitingConfirmation: false,
    preTTFTCancelActive: false,
    pendingApproval: null,
    pendingExecApproval: null,
    pendingQuestion: null,
    contextTrimInfo: null,
    currentConversation: null as null | Record<string, unknown>,
    currentConversationId: null as null | string,
    deepResearchEnabled: false,
    error: null as null | string,
    executingConversationIds: [] as string[],
    hasMoreMessages: false,
    isMultiSelectMode: false,
    isPreTTFT: false,
    isRecovering: false,
    isStreamInterrupted: false,
    loading: false,
    loadingMore: false,
    messages: [] as Array<Record<string, unknown>>,
    modelPreference: 'auto',
    pendingModelAutoFallback: null as null | Record<string, unknown>,
    searching: false,
    securityBlocked: null as null | Record<string, unknown>,
    selectedMessageIds: new Set<string>(),
    sending: false,
    streamError: null as null | string,
    streamProgress: null as null | string,
    streamUIState: { phase: 'idle' },
    streaming: false,
    streamingContent: '',
    sortedConversations: [] as Array<Record<string, unknown>>,
    toolExecuting: false,
    toolExecutingCommands: [] as string[],
    toolExecutingNames: [] as string[],
    toolResults: [] as Array<Record<string, unknown>>,
    toolSandboxAvailable: false,
    trialExhausted: false,
    fetchConversations: vi.fn(),
    selectConversation: vi.fn(),
    getMessageMetadata: vi.fn(),
    sendMessage: vi.fn(),
    recoverPendingConfirmations: vi.fn(),
    loadMoreMessages: vi.fn(),
    searchConversations: vi.fn(),
    setModelPreference: vi.fn(),
    splitModelPreference: vi.fn((value: string) => {
      const [provider_id = '', selected_model_id = value] = value.split('/', 2)
      return { provider_id, selected_model_id }
    }),
    setDeepResearchEnabled: vi.fn(),
    createConversation: vi.fn(),
    deleteConversation: vi.fn(),
    pinConversation: vi.fn(),
    unpinConversation: vi.fn(),
    continueMessage: vi.fn(),
    regenerateMessage: vi.fn(),
    editMessageAndResubmit: vi.fn(),
    injectMessage: vi.fn(),
    warmupConversation: vi.fn(),
    resetWarmup: vi.fn(),
    clearError: vi.fn(),
    clearStreamError: vi.fn(),
    clearSecurityBlocked: vi.fn(),
    cancelStreaming: vi.fn(),
    retryInterruptedStreamRecovery: vi.fn(),
    confirmModelAutoFallbackRetry: vi.fn(),
    dismissModelAutoFallbackRetry: vi.fn(),
    cancelPreTTFT: vi.fn(),
    deleteSelectedMessages: vi.fn(),
    enterMultiSelectMode: vi.fn(),
    exitMultiSelectMode: vi.fn(),
  },
  settingsStore: {
    agentAutoConfirm: false,
    agentMode: false,
    showToolDetails: true,
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
    fetchProviders: vi.fn(),
    fetchRoutingMode: vi.fn(),
    fetchTrialQuota: vi.fn(),
    setRoutingMode: vi.fn(),
    getProviderDisplayName: vi.fn((providerId: string) => providerId),
  },
  deepResearchJobsStore: {
    activeJobs: [] as Array<Record<string, unknown>>,
    jobMap: {} as Record<string, unknown>,
    loading: false,
    hydrated: true,
    pendingFocusJobId: null as null | string,
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
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  },
  mediaGenerate: {
    showPanel: { value: false },
    intent: { value: null as null | Record<string, unknown> },
    models: { value: [] as unknown[] },
    selectedModel: { value: '' },
    generating: { value: false },
    ambiguous: { value: false },
    task: { value: null as null | Record<string, unknown> },
    classify: vi.fn(),
    generate: vi.fn(),
    reset: vi.fn(),
    confirmAmbiguous: vi.fn(),
    cancel: vi.fn(),
    switchCategory: vi.fn(),
  },
  authFetch: vi.fn(),
  onSSEEvent: vi.fn(),
  offSSEEvent: vi.fn(),
  agentApi: {
    listTasks: vi.fn(),
    cancelTask: vi.fn(),
    deleteTask: vi.fn(),
    sendMessage: vi.fn(),
    submitAnswers: vi.fn(),
  },
}))

mocks.chatStore = reactive(mocks.chatStore)
mocks.settingsStore = reactive(mocks.settingsStore)
mocks.providerPoolStore = reactive(mocks.providerPoolStore)
mocks.deepResearchJobsStore = reactive(mocks.deepResearchJobsStore)
mocks.taskProjectionsStore = reactive(mocks.taskProjectionsStore)
mocks.mediaGenerate = reactive(mocks.mediaGenerate)

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
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

vi.mock('@/api/chat', () => ({
  agentApi: {
    listTasks: (...args: unknown[]) => mocks.agentApi.listTasks(...args),
    cancelTask: (...args: unknown[]) => mocks.agentApi.cancelTask(...args),
    deleteTask: (...args: unknown[]) => mocks.agentApi.deleteTask(...args),
    sendMessage: (...args: unknown[]) => mocks.agentApi.sendMessage(...args),
    submitAnswers: (...args: unknown[]) => mocks.agentApi.submitAnswers(...args),
  },
}))

vi.mock('@/api/client', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
  authFetch: (...args: unknown[]) => mocks.authFetch(...args),
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

vi.mock('@/api/voice', () => ({
  streamingTTSManager: {
    setLocale: vi.fn(),
    stop: vi.fn(),
  },
}))

const chatInputStub = {
  name: 'ChatInput',
  props: {
    disabled: { type: Boolean, default: false },
    streaming: { type: Boolean, default: false },
    canCancel: { type: Boolean, default: false },
  },
  emits: ['send', 'open-talk-mode'],
  methods: {
    setInput(value: string) {
      mocks.chatInputSetInput(value)
    },
    focus() {},
    resetWarmup() {},
  },
  template:
    '<button class="chat-input-send-stub" @click="$emit(\'send\', \'need provider\', [])" />',
}

const talkModeStub = {
  name: 'TalkMode',
  emits: ['transcript', 'update:modelValue'],
  template: '<div class="talk-mode-stub" />',
}

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

describe('ChatView provider gating', () => {
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

  beforeAll(async () => {
    await setLocale('en-US')
    await setLocale('zh-CN')
    await setLocale('zh-TW')
    await setLocale('en-US')
  })

  beforeEach(() => {
    vi.clearAllMocks()
    localStorageMock.clear()
    i18n.global.locale.value = 'en-US'
    mocks.chatStore.currentConversationId = null
    mocks.chatStore.pendingApproval = null
    mocks.chatStore.pendingExecApproval = null
    mocks.chatStore.pendingModelAutoFallback = null
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.activeProviders = []
    mocks.chatStore.isRecovering = false
    mocks.chatStore.streamUIState = { phase: 'idle' }
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockResolvedValue(undefined)
    mocks.providerPoolStore.setRoutingMode.mockImplementation((mode: string) => {
      mocks.providerPoolStore.routingMode = mode as 'auto' | 'cloud' | 'local'
    })
    mocks.settingsStore.fetchTools.mockResolvedValue(undefined)
    mocks.chatStore.fetchConversations.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    mocks.chatStore.modelPreference = 'auto'
    mocks.chatStore.setModelPreference.mockImplementation((value: string) => {
      mocks.chatStore.modelPreference = value
    })
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
    mocks.deepResearchJobsStore.fetchActiveJobs.mockResolvedValue(undefined)
    mocks.mediaGenerate.classify.mockResolvedValue(false)
    mocks.authFetch.mockRejectedValue(new Error('ignore'))
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
  })

  async function settleView() {
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()
    await new Promise((resolve) => setTimeout(resolve, 0))
    await flushPromises()
  }

  async function mountChatViewHarness(initialPath = '/chat') {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/chat', name: 'Chat', component: ChatView },
        { path: '/settings', name: 'Settings', component: { template: '<div />' } },
        { path: '/security', name: 'Security', component: { template: '<div />' } },
      ],
    })
    await router.push(initialPath)
    await router.isReady()

    const wrapper = mount(ChatView, {
      global: {
        plugins: [router, i18n],
        stubs: {
          ConversationList: { template: '<div class="conversation-list-stub" />' },
          ChatMessage: { template: '<div class="chat-message-stub" />' },
          ChatInput: chatInputStub,
          VirtualScroll: { template: '<div class="virtual-scroll-stub"><slot /></div>' },
          PresetQuestions: { template: '<div class="preset-questions-stub" />' },
          TalkMode: talkModeStub,
          ToolApprovalDialog: { template: '<div class="tool-approval-stub" />' },
          ExecApprovalDialog: { template: '<div class="exec-approval-stub" />' },
          MediaParamPanel: { template: '<div class="media-param-panel-stub" />' },
          UserTaskProjectionCard: { template: '<div class="agent-task-panel-stub" />' },
          UserTaskProjectionDock: { template: '<div class="deep-research-task-dock-stub" />' },
          Teleport: true,
          Transition: true,
        },
      },
    })

    await settleView()
    return { wrapper, router }
  }

  async function mountChatView() {
    const { wrapper } = await mountChatViewHarness()
    return wrapper
  }

  it('hydrates task projection context immediately on mount', async () => {
    const wrapper = await mountChatView()

    expect(mocks.taskProjectionsStore.setConversation).toHaveBeenCalledWith('')
    expect(mocks.onSSEEvent).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('switches conversations when the route conversationId query changes', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'

    const { router, wrapper } = await mountChatViewHarness('/chat')

    await router.push({ name: 'Chat', query: { conversationId: 'conv-2' } })
    await settleView()

    expect(mocks.chatStore.selectConversation).toHaveBeenCalledWith('conv-2')

    wrapper.unmount()
  })

  it('lazy mounts approval dialogs only when approvals are pending', async () => {
    const wrapper = await mountChatView()

    expect(wrapper.find('.tool-approval-stub').exists()).toBe(false)
    expect(wrapper.find('.exec-approval-stub').exists()).toBe(false)

    mocks.chatStore.pendingApproval = {
      tool_name: 'web_query',
      arguments: { q: 'hello' },
    } as Record<string, unknown>
    await settleView()
    expect(wrapper.find('.tool-approval-stub').exists()).toBe(true)

    mocks.chatStore.pendingExecApproval = {
      directory: '/tmp',
      command: 'pwd',
      expires_at: Date.now() + 10_000,
    } as Record<string, unknown>
    await settleView()
    expect(wrapper.find('.exec-approval-stub').exists()).toBe(true)

    wrapper.unmount()
  })

  it('allows send to continue when no provider is configured and keeps setup guidance inline', async () => {
    const wrapper = await mountChatView()
    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('send', 'need provider', [])
    await flushPromises()

    expect(mocks.providerPoolStore.fetchProviders).toHaveBeenCalled()
    expect(mocks.mediaGenerate.classify).toHaveBeenCalledWith('need provider', false, 0, 'en-US', [])
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith('need provider', [])
    expect(mocks.chatInputSetInput).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'Set up an AI provider to start chatting'
    )
    expect(wrapper.text()).not.toContain(
      'Your message has been saved locally. You can continue after setup.'
    )
  })

  it('shows setup guidance in the empty state when no provider is configured', async () => {
    const wrapper = await mountChatView()

    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'Set up an AI provider to start chatting'
    )
    expect(wrapper.get('[data-testid="chat-provider-guidance-action"]').text()).toContain(
      'Configure Provider'
    )
    expect(wrapper.find('[data-testid="chat-provider-guidance-routing"]').exists()).toBe(false)
  })

  it('blocks send when configured providers are unavailable and restores the draft', async () => {
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'auth_error:invalid_api_key',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'auth_error:invalid_api_key',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.activeProviders = []

    const wrapper = await mountChatView()
    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('send', 'need provider', [])
    await flushPromises()

    expect(mocks.providerPoolStore.fetchProviders).not.toHaveBeenCalled()
    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
    expect(mocks.chatInputSetInput).toHaveBeenCalledWith('need provider')
    expect(wrapper.text()).toContain('No AI provider is available right now')
    expect(wrapper.text()).toContain('Review Providers')
    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'No AI provider is available right now'
    )
    expect(wrapper.text()).toContain('openai')
    expect(wrapper.text()).toContain(
      'Authentication failed. Recheck the API key or OAuth connection.'
    )
  })

  it('does not show the routing button inside provider guidance when providers need attention', async () => {
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'auth_error:invalid_api_key',
      } as Record<string, unknown>,
      {
        id: 'anthropic',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'auth_error:invalid_api_key',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = []

    const wrapper = await mountChatView()

    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'No AI provider is available right now'
    )
    expect(wrapper.find('[data-testid="chat-provider-guidance-routing"]').exists()).toBe(false)
  })

  it('does not render the removed runtime controls on the chat page', async () => {
    const wrapper = await mountChatView()

    expect(wrapper.find('.chat-mode-pill.chat-mode-pill-enabled').exists()).toBe(false)
    expect(wrapper.find('[data-onboarding-anchor="enhanced-mode-entry"]').exists()).toBe(false)
    expect(wrapper.find('[data-onboarding-anchor="enhanced-mode-menu"]').exists()).toBe(false)
    expect(wrapper.find('.enhanced-mode-hover-card').exists()).toBe(false)
  })

  it('shows routing controls when only one provider is enabled', async () => {
    const provider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'active',
      location: 'cloud',
    } as Record<string, unknown>
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.enabledProviders = [provider]
    mocks.providerPoolStore.activeProviders = [provider]
    mocks.providerPoolStore.cloudProviders = [provider]
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.hasCloudProviders = true
    mocks.providerPoolStore.hasLocalProviders = false

    const wrapper = await mountChatView()

    expect(wrapper.find('.chat-thread-routing-btn').exists()).toBe(true)

    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    expect(wrapper.findAll('.routing-option-row')).toHaveLength(1)
  })

  it('shows the fixed-model fallback dialog and confirms auto retry', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingModelAutoFallback = {
      conversationId: 'conv-1',
      retryKind: 'send',
      requestedProviderId: 'anthropic',
      requestedModelId: 'claude-opus-4-5-20251101',
      errorMessage: "No available AI provider for model 'claude-opus-4-5-20251101'",
    }

    const wrapper = await mountChatView()
    await flushPromises()

    expect(wrapper.text()).toContain('Switch to auto routing and retry?')
    expect(wrapper.text()).toContain('anthropic/claude-opus-4-5-20251101')

    const actionButtons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes('Switch and retry'))
    expect(actionButtons).toHaveLength(1)

    await actionButtons[0]!.trigger('click')

    expect(mocks.chatStore.confirmModelAutoFallbackRetry).toHaveBeenCalledTimes(1)
  })

  it('dismisses the fixed-model fallback dialog without retrying', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingModelAutoFallback = {
      conversationId: 'conv-1',
      retryKind: 'send',
      requestedProviderId: 'anthropic',
      requestedModelId: 'claude-opus-4-5-20251101',
      errorMessage: "No available AI provider for model 'claude-opus-4-5-20251101'",
    }

    const wrapper = await mountChatView()
    await flushPromises()

    const actionButtons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes('Keep current model'))
    expect(actionButtons).toHaveLength(1)

    await actionButtons[0]!.trigger('click')

    expect(mocks.chatStore.dismissModelAutoFallbackRetry).toHaveBeenCalledTimes(1)
    expect(mocks.chatStore.confirmModelAutoFallbackRetry).not.toHaveBeenCalled()
  })

  it('localizes the fixed-model fallback dialog in zh-CN', async () => {
    await setLocale('zh-CN')
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingModelAutoFallback = {
      conversationId: 'conv-1',
      retryKind: 'send',
      requestedProviderId: 'anthropic',
      requestedModelId: 'claude-opus-4-5-20251101',
      errorMessage: "No available AI provider for model 'claude-opus-4-5-20251101'",
    }

    const wrapper = await mountChatView()
    await flushPromises()

    expect(wrapper.text()).toContain('固定模型不可用')
    expect(wrapper.text()).toContain('切换到自动路由并重试？')
    expect(wrapper.text()).toContain('保留当前模型')
    expect(wrapper.text()).toContain('切换并重试')
    expect(wrapper.text()).toContain('anthropic/claude-opus-4-5-20251101')
  })

  it('localizes the fixed-model fallback dialog in zh-TW', async () => {
    await setLocale('zh-TW')
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingModelAutoFallback = {
      conversationId: 'conv-1',
      retryKind: 'send',
      requestedProviderId: 'anthropic',
      requestedModelId: 'claude-opus-4-5-20251101',
      errorMessage: "No available AI provider for model 'claude-opus-4-5-20251101'",
    }

    const wrapper = await mountChatView()
    await flushPromises()

    expect(wrapper.text()).toContain('固定模型無法使用')
    expect(wrapper.text()).toContain('切換到自動路由並重試？')
    expect(wrapper.text()).toContain('保留目前模型')
    expect(wrapper.text()).toContain('切換並重試')
    expect(wrapper.text()).toContain('anthropic/claude-opus-4-5-20251101')
  })

  it('keeps fixed model options in backend order and hides cloud/local rows when only cloud providers exist', async () => {
    const providerA = {
      id: 'provider-a',
      type: 'custom',
      enabled: true,
      status: 'active',
      location: 'cloud',
    } as Record<string, unknown>
    const providerB = {
      id: 'provider-b',
      type: 'custom',
      enabled: true,
      status: 'active',
      location: 'cloud',
    } as Record<string, unknown>
    mocks.providerPoolStore.providers = [providerA, providerB]
    mocks.providerPoolStore.enabledProviders = [providerA, providerB]
    mocks.providerPoolStore.activeProviders = [providerA, providerB]
    mocks.providerPoolStore.cloudProviders = [providerA, providerB]
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.hasCloudProviders = true
    mocks.providerPoolStore.hasLocalProviders = false
    mocks.providerPoolStore.models = [
      {
        id: 'zzz-model',
        provider_id: 'provider-a',
        display_name: 'ZZZ Model',
        enabled: true,
      },
      {
        id: 'aaa-model',
        provider_id: 'provider-a',
        display_name: 'AAA Model',
        enabled: true,
      },
      {
        id: 'bbb-model',
        provider_id: 'provider-b',
        display_name: 'BBB Model',
        enabled: true,
      },
    ] as Array<Record<string, unknown>>

    const wrapper = await mountChatView()
    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    expect(wrapper.findAll('.routing-option-row')).toHaveLength(1)

    const strategyButtons = wrapper.findAll('.routing-strategy-chip')
    expect(strategyButtons).toHaveLength(2)

    await strategyButtons[1]!.trigger('click')
    await settleView()

    const modelRows = wrapper.findAll('.routing-model-row')
    expect(modelRows).toHaveLength(3)
    expect(modelRows[0]?.text()).toContain('ZZZ Model')
    expect(modelRows[1]?.text()).toContain('AAA Model')
    expect(modelRows[2]?.text()).toContain('BBB Model')
  })

  it('routes talk-mode transcripts to sendMessage when no stream is active', async () => {
    mocks.providerPoolStore.providers = [
      { id: 'openai', enabled: true, status: 'active' } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = [...mocks.providerPoolStore.providers]
    mocks.chatStore.streaming = false

    const wrapper = await mountChatView()
    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('open-talk-mode')
    await settleView()
    wrapper.findComponent({ name: 'TalkMode' }).vm.$emit('transcript', 'hello from talk mode')
    await flushPromises()

    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith('hello from talk mode')
    expect(mocks.chatStore.injectMessage).not.toHaveBeenCalled()
  })

  it('routes talk-mode transcripts to injectMessage while a stream is active', async () => {
    mocks.providerPoolStore.providers = [
      { id: 'openai', enabled: true, status: 'active' } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = [...mocks.providerPoolStore.providers]
    mocks.chatStore.streaming = true

    const wrapper = await mountChatView()
    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('open-talk-mode')
    await settleView()
    wrapper.findComponent({ name: 'TalkMode' }).vm.$emit('transcript', 'interrupt now')
    await flushPromises()

    expect(mocks.chatStore.injectMessage).toHaveBeenCalledWith('interrupt now')
    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
  })
})
