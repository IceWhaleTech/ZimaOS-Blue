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
    pendingProviderFailoverRetry: null as null | Record<string, unknown>,
    providerPinOnlyActive: false,
    searching: false,
    securityBlocked: null as null | Record<string, unknown>,
    selectedProviderId: '',
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
    setProviderPinOnly: vi.fn(),
    splitModelPreference: vi.fn((value: string) => {
      const [provider_id = '', selected_model_id = value] = value.split('/', 2)
      return { provider_id, selected_model_id }
    }),
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
    confirmProviderFailoverRetry: vi.fn(),
    dismissProviderFailoverRetry: vi.fn(),
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
  systemStore: {
    health: null as null | Record<string, unknown>,
  },
  providerPoolStore: {
    activeProviders: [] as unknown[],
    cloudProviders: [] as unknown[],
    clearProviderError: vi.fn(),
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
    originalPrompt: { value: '' },
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
}))

mocks.chatStore = reactive(mocks.chatStore)
mocks.settingsStore = reactive(mocks.settingsStore)
mocks.systemStore = reactive(mocks.systemStore)
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

vi.mock('@/stores/system', () => ({
  useSystemStore: () => mocks.systemStore,
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
    mocks.chatStore.pendingProviderFailoverRetry = null
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.activeProviders = []
    mocks.chatStore.isRecovering = false
    mocks.chatStore.streamUIState = { phase: 'idle' }
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockResolvedValue(undefined)
    mocks.providerPoolStore.getProviderDisplayName.mockImplementation(
      (providerId: string) => providerId
    )
    mocks.providerPoolStore.setRoutingMode.mockImplementation((mode: string) => {
      mocks.providerPoolStore.routingMode = mode as 'auto' | 'cloud' | 'local'
    })
    mocks.settingsStore.fetchTools.mockResolvedValue(undefined)
    mocks.chatStore.fetchConversations.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    mocks.chatStore.modelPreference = 'auto'
    mocks.chatStore.providerPinOnlyActive = false
    mocks.chatStore.selectedProviderId = ''
    mocks.chatStore.setModelPreference.mockImplementation((value: string) => {
      mocks.chatStore.modelPreference = value
      mocks.chatStore.providerPinOnlyActive = false
      if (value === 'auto') {
        mocks.chatStore.selectedProviderId = ''
        return
      }
      const [providerId = ''] = value.split('/', 2)
      mocks.chatStore.selectedProviderId = value.includes('/') ? providerId : ''
    })
    mocks.chatStore.setProviderPinOnly.mockImplementation((providerId: string) => {
      mocks.chatStore.modelPreference = 'auto'
      mocks.chatStore.selectedProviderId = providerId
      mocks.chatStore.providerPinOnlyActive = !!providerId
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
          ConversationList: {
            template:
              '<button class="conversation-list-stub" data-testid="conversation-list-more-actions-stub" @click="$emit(\'more-actions\')">more</button>',
          },
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

  it('opens feature guidance for Deep Research and Smart Resume from the conversation list menu', async () => {
    mocks.settingsStore.setAgentMode.mockImplementation(async (enabled: boolean) => {
      mocks.settingsStore.agentMode = enabled
    })

    const wrapper = await mountChatView()

    await wrapper.get('[data-testid="conversation-list-more-actions-stub"]').trigger('click')

    const topbarSheet = wrapper.get('[data-testid="mobile-topbar-sheet"]')
    expect(topbarSheet.text()).toContain('Deep Research')
    expect(topbarSheet.text()).toContain('Smart Resume')

    await wrapper.get('[data-testid="mobile-topbar-quick-action-deep-research"]').trigger('click')

    const deepResearchSheet = wrapper.get('[data-testid="mobile-feature-sheet"]')
    expect(deepResearchSheet.text()).toContain('Deep Research')
    expect(deepResearchSheet.text()).toContain('Retrieving')
    expect(deepResearchSheet.text()).toContain('Verifying')

    await wrapper.get('[data-testid="mobile-feature-sheet-close"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="conversation-list-more-actions-stub"]').trigger('click')
    await wrapper.get('[data-testid="mobile-topbar-quick-action-smart-resume"]').trigger('click')

    const smartResumeSheet = wrapper.get('[data-testid="mobile-feature-sheet"]')
    expect(smartResumeSheet.text()).toContain('Smart Resume')
    expect(smartResumeSheet.text()).toContain('Plan')
    expect(smartResumeSheet.text()).toContain('Auto-Confirm')

    await wrapper.get('[data-testid="mobile-feature-primary-action"]').trigger('click')

    expect(mocks.settingsStore.setAgentMode).toHaveBeenCalledWith(true)

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

  it('allows send to continue when configured providers are unavailable and keeps inline provider attention guidance', async () => {
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
    expect(mocks.mediaGenerate.classify).toHaveBeenCalledWith('need provider', false, 0, 'en-US', [])
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith('need provider', [])
    expect(mocks.chatInputSetInput).not.toHaveBeenCalled()
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

  it('treats a single errored provider as recoverable attention instead of global unavailable', async () => {
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'unexpected_status:500',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = []

    const wrapper = await mountChatView()

    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'Provider needs attention'
    )
    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'The provider returned an unexpected status (500).'
    )
    expect(wrapper.text()).not.toContain('No AI provider is available right now')
    expect(wrapper.get('[data-testid="chat-provider-guidance-retry"]').text()).toContain(
      'Retry Provider'
    )
    expect(wrapper.get('[data-testid="chat-provider-guidance-review"]').text()).toContain(
      'Review Provider'
    )
  })

  it('can clear the only errored provider directly from chat guidance', async () => {
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'unexpected_status:500',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = []

    const wrapper = await mountChatView()
    await wrapper.get('[data-testid="chat-provider-guidance-retry"]').trigger('click')

    expect(mocks.providerPoolStore.clearProviderError).toHaveBeenCalledWith('openai')
  })

  it('opens provider settings focused on the affected provider from recoverable guidance', async () => {
    mocks.providerPoolStore.providers = [
      {
        id: 'openai',
        type: 'builtin',
        enabled: true,
        status: 'error',
        last_error: 'unexpected_status:500',
      } as Record<string, unknown>,
    ]
    mocks.providerPoolStore.enabledProviders = [...mocks.providerPoolStore.providers]
    mocks.providerPoolStore.activeProviders = []

    const { wrapper, router } = await mountChatViewHarness()
    await wrapper.get('[data-testid="chat-provider-guidance-review"]').trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.fullPath).toBe('/settings?tab=llm&provider=openai')
  })

  it('still reaches media intent classification when configured providers are unavailable', async () => {
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
    mocks.mediaGenerate.classify.mockResolvedValueOnce(true)

    const wrapper = await mountChatView()
    wrapper.findComponent({ name: 'ChatInput' }).vm.$emit('send', 'make a poster', [])
    await flushPromises()

    expect(mocks.mediaGenerate.classify).toHaveBeenCalledWith('make a poster', false, 0, 'en-US', [])
    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
    expect(mocks.chatInputSetInput).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="chat-provider-guidance-card"]').text()).toContain(
      'No AI provider is available right now'
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

  it('keeps routing controls visible without a warning dot when providers need attention', async () => {
    const provider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'error',
      last_error: 'auth_error:invalid_api_key',
      location: 'cloud',
    } as Record<string, unknown>
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.enabledProviders = [provider]
    mocks.providerPoolStore.activeProviders = []
    mocks.providerPoolStore.cloudProviders = [provider]
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.hasCloudProviders = true
    mocks.providerPoolStore.hasLocalProviders = false

    const wrapper = await mountChatView()

    expect(wrapper.find('.chat-thread-routing-btn').exists()).toBe(true)
    expect(wrapper.find('.chat-thread-routing-btn__status').exists()).toBe(false)

    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    expect(wrapper.text()).toContain('Provider needs attention. Click to check settings.')
    expect(wrapper.findAll('.routing-option-row')).toHaveLength(1)
    expect(wrapper.find('.routing-manage-row').exists()).toBe(true)
  })

  it('shows provider-scoped auto status when a conversation is pinned to a provider only', async () => {
    const provider = {
      id: 'openrouter',
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
    mocks.providerPoolStore.getProviderDisplayName.mockImplementation((providerId: string) =>
      providerId === 'openrouter' ? 'OpenRouter' : providerId
    )
    mocks.chatStore.modelPreference = 'auto'
    mocks.chatStore.selectedProviderId = 'openrouter'
    mocks.chatStore.providerPinOnlyActive = true

    const wrapper = await mountChatView()

    expect(wrapper.get('.chat-thread-routing-btn').attributes('title')).toContain(
      'Routing Mode · Auto · OpenRouter'
    )

    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    expect(wrapper.get('.routing-option-status-chip').text()).toContain('Auto · OpenRouter')
    expect(wrapper.get('[data-testid="routing-provider-pin-notice"]').text()).toContain(
      'Pinned Provider'
    )
    expect(wrapper.get('[data-testid="routing-provider-pin-notice"]').text()).toContain(
      'This conversation stays on OpenRouter'
    )

    await wrapper.get('[data-testid="routing-provider-pin-clear"]').trigger('click')
    await settleView()

    expect(mocks.chatStore.setModelPreference).toHaveBeenCalledWith('auto')
    expect(wrapper.find('[data-testid="routing-provider-pin-notice"]').exists()).toBe(false)
  })

  it('hides provider-scoped auto copy when the pinned provider is no longer enabled', async () => {
    const enabledProvider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'active',
      location: 'cloud',
    } as Record<string, unknown>
    mocks.providerPoolStore.providers = [enabledProvider]
    mocks.providerPoolStore.enabledProviders = [enabledProvider]
    mocks.providerPoolStore.activeProviders = [enabledProvider]
    mocks.providerPoolStore.cloudProviders = [enabledProvider]
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.hasCloudProviders = true
    mocks.providerPoolStore.hasLocalProviders = false
    mocks.chatStore.modelPreference = 'auto'
    mocks.chatStore.selectedProviderId = 'openrouter'
    mocks.chatStore.providerPinOnlyActive = true

    const wrapper = await mountChatView()

    expect(wrapper.get('.chat-thread-routing-btn').attributes('title')).not.toContain('openrouter')

    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    expect(wrapper.find('[data-testid="routing-provider-pin-notice"]').exists()).toBe(false)
    expect(wrapper.get('.routing-option-status-chip').text()).not.toContain('openrouter')
  })

  it('lets users pin a provider while keeping model selection automatic', async () => {
    const providerA = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'active',
      location: 'cloud',
    } as Record<string, unknown>
    const providerB = {
      id: 'openrouter',
      type: 'builtin',
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
    mocks.providerPoolStore.getProviderDisplayName.mockImplementation((providerId: string) =>
      providerId === 'openrouter' ? 'OpenRouter' : providerId === 'openai' ? 'OpenAI' : providerId
    )

    const wrapper = await mountChatView()

    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    await wrapper.get('[data-testid="routing-provider-pin-row-openrouter"]').trigger('click')
    await settleView()

    expect(mocks.chatStore.setProviderPinOnly).toHaveBeenCalledWith('openrouter')
    expect(wrapper.get('[data-testid="routing-provider-pin-notice"]').text()).toContain(
      'This conversation stays on OpenRouter'
    )
  })

  it('does not label an inactive provider as still checking in routing controls', async () => {
    const provider = {
      id: 'openai',
      type: 'builtin',
      enabled: true,
      status: 'inactive',
      location: 'cloud',
    } as Record<string, unknown>
    mocks.providerPoolStore.providers = [provider]
    mocks.providerPoolStore.enabledProviders = [provider]
    mocks.providerPoolStore.activeProviders = []
    mocks.providerPoolStore.cloudProviders = [provider]
    mocks.providerPoolStore.localProviders = []
    mocks.providerPoolStore.hasCloudProviders = true
    mocks.providerPoolStore.hasLocalProviders = false

    const wrapper = await mountChatView()

    await wrapper.get('.chat-thread-routing-btn').trigger('click')
    await settleView()

    expect(wrapper.text()).not.toContain('Provider status is being checked...')
    expect(wrapper.text()).toContain('Provider needs attention. Click to check settings.')
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

  it('shows the provider failover dialog and confirms retry', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingProviderFailoverRetry = {
      conversationId: 'conv-1',
      retryKind: 'continue',
      failedProviderId: 'prov_primary',
      detail:
        'Tool follow-up hit repeated upstream 502 errors after backoff retries were exhausted. Blue is ready to switch to another available route and retry.',
      retryAttempts: 2,
      errorMessage: 'provider_failover_confirmation_required',
    }

    const wrapper = await mountChatView()
    await flushPromises()

    expect(wrapper.text()).toContain('High-availability switch')
    expect(wrapper.text()).toContain('Switch to another available route and retry?')
    expect(wrapper.text()).toContain('Current route')
    expect(wrapper.text()).toContain('prov_primary')
    expect(wrapper.text()).toContain('repeated upstream 502 errors')

    const actionButtons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes('Switch and retry'))
    expect(actionButtons).toHaveLength(1)

    await actionButtons[0]!.trigger('click')

    expect(mocks.chatStore.confirmProviderFailoverRetry).toHaveBeenCalledTimes(1)
  })

  it('opens manual routing selection from the provider failover dialog', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingProviderFailoverRetry = {
      conversationId: 'conv-1',
      retryKind: 'regenerate',
      failedProviderId: 'prov_primary',
      detail: 'Repeated 529 and 5xx errors exhausted retry budget.',
      retryAttempts: 3,
      errorMessage: 'provider_failover_confirmation_required',
    }
    const provider = {
      id: 'prov_backup',
      type: 'custom',
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
    await flushPromises()

    const manualButtons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes('Choose route manually'))
    expect(manualButtons).toHaveLength(1)

    await manualButtons[0]!.trigger('click')
    await settleView()

    expect(mocks.chatStore.dismissProviderFailoverRetry).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('.routing-strategy-chip').length).toBeGreaterThan(0)
  })

  it('dismisses the provider failover dialog on Escape', async () => {
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingProviderFailoverRetry = {
      conversationId: 'conv-1',
      retryKind: 'send',
      failedProviderId: 'prov_primary',
      detail: 'Repeated upstream 429 errors blocked the current route.',
      retryAttempts: 1,
      errorMessage: 'provider_failover_confirmation_required',
    }

    const wrapper = await mountChatView()
    await flushPromises()

    const dialog = wrapper.get('[aria-labelledby="provider-failover-dialog-title"]')
    await dialog.trigger('keydown', { key: 'Escape' })

    expect(mocks.chatStore.dismissProviderFailoverRetry).toHaveBeenCalledTimes(1)
  })

  it('dismisses the fixed-model fallback dialog on Escape', async () => {
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

    const dialog = wrapper.get('[aria-labelledby="model-auto-fallback-dialog-title"]')
    await dialog.trigger('keydown', { key: 'Escape' })

    expect(mocks.chatStore.dismissModelAutoFallbackRetry).toHaveBeenCalledTimes(1)
  })

  it('localizes the provider failover dialog in zh-CN', async () => {
    await setLocale('zh-CN')
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.pendingProviderFailoverRetry = {
      conversationId: 'conv-1',
      retryKind: 'send',
      failedProviderId: 'prov_primary',
      detail: '工具后续在 prov_primary 上连续出现 502 错误，等待你确认是否切换。',
      retryAttempts: 2,
      errorMessage: 'provider_failover_confirmation_required',
    }

    const wrapper = await mountChatView()
    await flushPromises()

    expect(wrapper.text()).toContain('高可用切换')
    expect(wrapper.text()).toContain('切换到其他可用路由并重试？')
    expect(wrapper.text()).toContain('手动选择路由')
    expect(wrapper.text()).toContain('当前路由')
    expect(wrapper.text()).toContain('prov_primary')
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
