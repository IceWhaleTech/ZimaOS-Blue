import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { reactive } from 'vue'
import ChatView from '@/views/ChatView.vue'
import { i18n } from '@/i18n'

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
    cancelPreTTFT: vi.fn(),
    deleteSelectedMessages: vi.fn(),
    enterMultiSelectMode: vi.fn(),
    exitMultiSelectMode: vi.fn(),
  },
  settingsStore: {
    agentAutoConfirm: false,
    agentMode: false,
    claudeCodeEnabled: false,
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
    loading: false,
    hydrated: true,
    hasActiveTasks: false,
    refreshNow: vi.fn(),
    setConversation: vi.fn(),
    cancelTask: vi.fn(),
    openTask: vi.fn(),
    stopPolling: vi.fn(),
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
  emits: ['send'],
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

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.activeProviders = []
    mocks.chatStore.isRecovering = false
    mocks.chatStore.streamUIState = { phase: 'idle' }
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockResolvedValue(undefined)
    mocks.settingsStore.fetchTools.mockResolvedValue(undefined)
    mocks.chatStore.fetchConversations.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    mocks.taskProjectionsStore.currentTasks = []
    mocks.taskProjectionsStore.currentActiveTasks = []
    mocks.taskProjectionsStore.currentTerminalTasks = []
    mocks.taskProjectionsStore.backgroundTasks = []
    mocks.taskProjectionsStore.refreshNow.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.setConversation.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.cancelTask.mockReset().mockResolvedValue(undefined)
    mocks.taskProjectionsStore.openTask.mockReset().mockResolvedValue(undefined)
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

  async function mountChatView() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/chat', name: 'Chat', component: ChatView },
        { path: '/settings', name: 'Settings', component: { template: '<div />' } },
      ],
    })
    await router.push('/chat')
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
    return wrapper
  }

  it('blocks send, opens provider setup dialog, and restores input draft when no provider is configured', async () => {
    const wrapper = await mountChatView()
    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('send', 'need provider', [])
    await flushPromises()

    expect(mocks.providerPoolStore.fetchProviders).toHaveBeenCalled()
    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
    expect(mocks.mediaGenerate.classify).not.toHaveBeenCalled()
    expect(mocks.chatInputSetInput).toHaveBeenCalledWith('need provider')
    expect(wrapper.text()).toContain('Set up an AI provider to start chatting')
    expect(wrapper.text()).toContain(
      'Your draft has been kept locally so you can continue after setup.'
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

  it('anchors the routing menu to the clicked guidance button when providers need attention', async () => {
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'auth_error:invalid_api_key',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = []

    const wrapper = await mountChatView()
    const guidanceRoutingButton = wrapper.get('[data-testid="chat-provider-guidance-routing"]')
    const buttonEl = guidanceRoutingButton.element as HTMLButtonElement

    vi.spyOn(buttonEl, 'getBoundingClientRect').mockReturnValue({
      x: 840,
      y: 580,
      width: 120,
      height: 44,
      top: 580,
      right: 960,
      bottom: 624,
      left: 840,
      toJSON: () => ({}),
    } as DOMRect)

    await guidanceRoutingButton.trigger('click')
    await settleView()

    const menuStyle = wrapper.get('.routing-menu-floating').attributes('style')
    expect(menuStyle).not.toContain('left: 0px')
    expect(menuStyle).not.toContain('top: 0px')
  })

  it('shows the enhanced mode info card on hover when Claude Code CLI is enabled', async () => {
    mocks.settingsStore.claudeCodeEnabled = true

    const wrapper = await mountChatView()
    const enhancedPill = wrapper.find('.chat-mode-pill.chat-mode-pill-enabled')

    expect(enhancedPill.exists()).toBe(true)
    expect(wrapper.find('.enhanced-mode-hover-card').exists()).toBe(false)

    await enhancedPill.trigger('mouseenter')
    await flushPromises()

    expect(wrapper.find('.enhanced-mode-hover-card').exists()).toBe(true)
    expect(wrapper.findAll('.enhanced-mode-hover-card__item')).toHaveLength(4)
  })

  it('routes talk-mode transcripts to sendMessage when no stream is active', async () => {
    mocks.providerPoolStore.providers = [
      { id: 'openai', enabled: true, status: 'active' } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = [...mocks.providerPoolStore.providers]
    mocks.chatStore.streaming = false

    const wrapper = await mountChatView()
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
    wrapper.findComponent({ name: 'TalkMode' }).vm.$emit('transcript', 'interrupt now')
    await flushPromises()

    expect(mocks.chatStore.injectMessage).toHaveBeenCalledWith('interrupt now')
    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
  })
})
