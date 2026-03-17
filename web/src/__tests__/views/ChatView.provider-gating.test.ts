import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
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

vi.mock('@/components/ChatInput.vue', () => ({
  default: {
    name: 'ChatInput',
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
  },
}))

describe('ChatView provider gating', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.activeProviders = []
    mocks.providerPoolStore.fetchProviders.mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockResolvedValue(undefined)
    mocks.settingsStore.fetchTools.mockResolvedValue(undefined)
    mocks.chatStore.fetchConversations.mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockResolvedValue(undefined)
    mocks.deepResearchJobsStore.fetchActiveJobs.mockResolvedValue(undefined)
    mocks.mediaGenerate.classify.mockResolvedValue(false)
    mocks.authFetch.mockRejectedValue(new Error('ignore'))
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
  })

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

    const wrapper = shallowMount(ChatView, {
      global: {
        plugins: [router, i18n],
        stubs: { teleport: true, ChatInput: false },
      },
    })

    await flushPromises()
    return wrapper
  }

  it('blocks send, opens provider setup dialog, and restores input draft when no provider is configured', async () => {
    const wrapper = await mountChatView()
    await wrapper.find('.chat-input-send-stub').trigger('click')
    await flushPromises()

    expect(mocks.providerPoolStore.fetchProviders).toHaveBeenCalled()
    expect(mocks.chatStore.sendMessage).not.toHaveBeenCalled()
    expect(mocks.mediaGenerate.classify).not.toHaveBeenCalled()
    expect(mocks.chatInputSetInput).toHaveBeenCalledWith('need provider')
    expect(wrapper.text()).toContain('chat.noProvider.title')
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
})
