import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ChatView from '@/views/ChatView.vue'
import { i18n, setLocale } from '@/i18n'

const mocks = vi.hoisted(() => ({
  chatStore: {
    awaitingConfirmation: false,
    preTTFTCancelActive: false,
    pendingApproval: null,
    pendingExecApproval: null,
    pendingQuestion: null,
    contextTrimInfo: null,
    currentConversation: {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    },
    currentConversationId: 'conv-1',
    deepResearchEnabled: false,
    error: null,
    hasMoreMessages: false,
    isMultiSelectMode: false,
    isPreTTFT: false,
    loading: false,
    loadingMore: false,
    messages: [] as Array<Record<string, unknown>>,
    modelPreference: 'auto',
    searching: false,
    securityBlocked: null,
    selectedMessageIds: new Set<string>(),
    sending: false,
    streamError: null,
    streamProgress: null,
    streaming: false,
    streamingContent: '',
    sortedConversations: [] as Array<Record<string, unknown>>,
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
    themeStyle: 'default',
    fetchTools: vi.fn(),
    updateFromPoolProviders: vi.fn(),
    setAgentAutoConfirm: vi.fn(),
    setAgentMode: vi.fn(),
    setShowToolDetails: vi.fn(),
    setThemeStyle: vi.fn(),
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
  cardActionSubmit: vi.fn(),
  speechGetStatus: vi.fn(),
  agentApi: {
    listTasks: vi.fn(),
    cancelTask: vi.fn(),
    deleteTask: vi.fn(),
    sendMessage: vi.fn(),
    submitAnswers: vi.fn(),
  },
  authFetch: vi.fn(),
  onSSEEvent: vi.fn(),
  offSSEEvent: vi.fn(),
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  },
}))

vi.mock('@/stores/chat', () => ({
  useChatStore: () => mocks.chatStore,
}))

vi.mock('@/stores/settings', () => ({
  THEME_STYLES: [
    { id: 'default', labelKey: 'theme.styles.default' },
    { id: 'bubble', labelKey: 'theme.styles.bubble' },
  ],
  useSettingsStore: () => mocks.settingsStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => mocks.notificationStore,
}))

vi.mock('@/api/chat', () => ({
  cardActionApi: {
    submit: (...args: unknown[]) => mocks.cardActionSubmit(...args),
  },
  agentApi: {
    listTasks: (...args: unknown[]) => mocks.agentApi.listTasks(...args),
    cancelTask: (...args: unknown[]) => mocks.agentApi.cancelTask(...args),
    deleteTask: (...args: unknown[]) => mocks.agentApi.deleteTask(...args),
    sendMessage: (...args: unknown[]) => mocks.agentApi.sendMessage(...args),
    submitAnswers: (...args: unknown[]) => mocks.agentApi.submitAnswers(...args),
  },
}))

vi.mock('@/api/client', () => ({
  authFetch: (...args: unknown[]) => mocks.authFetch(...args),
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

vi.mock('@/components/onboarding/PresetQuestions.vue', () => ({
  default: { name: 'PresetQuestions', template: '<div class="preset-questions-stub" />' },
}))

vi.mock('@/components/VirtualScroll.vue', () => ({
  default: { name: 'VirtualScroll', template: '<div class="virtual-scroll-stub"><slot /></div>' },
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
vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({
  matches: false,
  media: '',
  onchange: null,
  addEventListener: vi.fn(),
  removeEventListener: vi.fn(),
  addListener: vi.fn(),
  removeListener: vi.fn(),
  dispatchEvent: vi.fn(),
})))
vi.stubGlobal('ResizeObserver', class {
  observe() {}
  unobserve() {}
  disconnect() {}
})

const WEB_FETCH_URL = 'https://www.reddit.com/r/test'
const WEB_FETCH_CARD_ID = 'web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'
const BROWSER_CARD_ID = 'browser-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest'

function makeAssistantMessage(content: string, id = 'msg-1') {
  return {
    id,
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-03-08T00:00:00.000Z',
  }
}

function makeTypelessBlock(payload: Record<string, unknown>) {
  return ['```typeless', JSON.stringify(payload), '```'].join('\n')
}

async function mountChatViewWithMessages(messages: Array<{ id: string; content: string }>) {
  mocks.chatStore.messages = messages.map(message => makeAssistantMessage(message.content, message.id))

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
      plugins: [i18n, router],
      stubs: {
        Teleport: true,
        Transition: true,
      },
    },
  })

  await flushPromises()
  await vi.dynamicImportSettled()
  await flushPromises()
  return wrapper
}

async function mountChatView(content: string) {
  return mountChatViewWithMessages([{ id: 'msg-1', content }])
}

function findButtonByText(wrapper: ReturnType<typeof mount>, text: string) {
  return wrapper.findAll('button').find(button => button.text().includes(text))
}

describe('ChatView page-level card actions', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'innerWidth', { value: 1280, writable: true, configurable: true })
    Object.defineProperty(window.navigator, 'userAgent', { value: 'desktop', configurable: true })
    Object.defineProperty(window.navigator, 'platform', { value: 'MacIntel', configurable: true })

    localStorageMock.clear()
    i18n.global.locale.value = 'en-US'

    mocks.chatStore.awaitingConfirmation = false
    mocks.chatStore.preTTFTCancelActive = false
    mocks.chatStore.pendingApproval = null
    mocks.chatStore.pendingExecApproval = null
    mocks.chatStore.pendingQuestion = null
    mocks.chatStore.contextTrimInfo = null
    mocks.chatStore.currentConversation = {
      id: 'conv-1',
      title: 'Test conversation',
      created_at: '2026-03-08T00:00:00.000Z',
      updated_at: '2026-03-08T00:00:00.000Z',
    }
    mocks.chatStore.currentConversationId = 'conv-1'
    mocks.chatStore.deepResearchEnabled = false
    mocks.chatStore.error = null
    mocks.chatStore.hasMoreMessages = false
    mocks.chatStore.isMultiSelectMode = false
    mocks.chatStore.isPreTTFT = false
    mocks.chatStore.loading = false
    mocks.chatStore.loadingMore = false
    mocks.chatStore.messages = []
    mocks.chatStore.modelPreference = 'auto'
    mocks.chatStore.searching = false
    mocks.chatStore.securityBlocked = null
    mocks.chatStore.selectedMessageIds = new Set<string>()
    mocks.chatStore.sending = false
    mocks.chatStore.streamError = null
    mocks.chatStore.streamProgress = null
    mocks.chatStore.streaming = false
    mocks.chatStore.streamingContent = ''
    mocks.chatStore.sortedConversations = []
    mocks.chatStore.trialExhausted = false
    mocks.chatStore.fetchConversations.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.selectConversation.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
    mocks.chatStore.sendMessage.mockReset().mockResolvedValue(undefined)
    mocks.chatStore.recoverPendingConfirmations.mockReset()
    mocks.chatStore.loadMoreMessages.mockReset()
    mocks.chatStore.searchConversations.mockReset()
    mocks.chatStore.setModelPreference.mockReset()
    mocks.chatStore.setDeepResearchEnabled.mockReset()
    mocks.chatStore.createConversation.mockReset()
    mocks.chatStore.deleteConversation.mockReset()
    mocks.chatStore.pinConversation.mockReset()
    mocks.chatStore.unpinConversation.mockReset()
    mocks.chatStore.continueMessage.mockReset()
    mocks.chatStore.regenerateMessage.mockReset()
    mocks.chatStore.injectMessage.mockReset()
    mocks.chatStore.warmupConversation.mockReset()
    mocks.chatStore.resetWarmup.mockReset()
    mocks.chatStore.clearError.mockReset()
    mocks.chatStore.clearStreamError.mockReset()
    mocks.chatStore.clearSecurityBlocked.mockReset()
    mocks.chatStore.cancelStreaming.mockReset()
    mocks.chatStore.cancelPreTTFT.mockReset()
    mocks.chatStore.deleteSelectedMessages.mockReset()
    mocks.chatStore.enterMultiSelectMode.mockReset()
    mocks.chatStore.exitMultiSelectMode.mockReset()

    mocks.settingsStore.agentAutoConfirm = false
    mocks.settingsStore.agentMode = false
    mocks.settingsStore.claudeCodeEnabled = false
    mocks.settingsStore.showToolDetails = true
    mocks.settingsStore.themeStyle = 'default'
    mocks.settingsStore.fetchTools.mockReset().mockResolvedValue(undefined)
    mocks.settingsStore.updateFromPoolProviders.mockReset()
    mocks.settingsStore.setAgentAutoConfirm.mockReset()
    mocks.settingsStore.setAgentMode.mockReset()
    mocks.settingsStore.setShowToolDetails.mockReset()
    mocks.settingsStore.setThemeStyle.mockReset()

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
    mocks.providerPoolStore.getProviderDisplayName.mockReset().mockImplementation((providerId: string) => providerId)
    mocks.providerPoolStore.fetchProviders.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchRoutingMode.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.setRoutingMode.mockReset().mockResolvedValue(undefined)

    mocks.mediaGenerate.showPanel.value = false
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

    mocks.cardActionSubmit.mockReset()
    mocks.speechGetStatus.mockReset().mockResolvedValue({ data: {} })

    mocks.agentApi.listTasks.mockReset().mockResolvedValue({ data: [] })
    mocks.agentApi.cancelTask.mockReset().mockResolvedValue({})
    mocks.agentApi.deleteTask.mockReset().mockResolvedValue({})
    mocks.agentApi.sendMessage.mockReset().mockResolvedValue({})
    mocks.agentApi.submitAnswers.mockReset().mockResolvedValue({})

    mocks.authFetch.mockReset().mockResolvedValue({})
    mocks.onSSEEvent.mockReset()
    mocks.offSSEEvent.mockReset()

    mocks.notificationStore.success.mockReset()
    mocks.notificationStore.error.mockReset()
    mocks.notificationStore.info.mockReset()
    mocks.notificationStore.remove.mockReset()
  })

  it('localizes the awaiting confirmation indicator', async () => {
    await setLocale('zh-CN')
    mocks.chatStore.awaitingConfirmation = true

    const wrapper = await mountChatViewWithMessages([{ id: 'msg-confirm', content: 'Please confirm' }])

    expect(wrapper.text()).toContain('等待你的确认以继续')
    expect(wrapper.text()).not.toContain('Waiting for your confirmation to continue')

    i18n.global.locale.value = 'en-US'
  })

  it('submits use_browser from a web-fetch card rendered inside ChatView', async () => {
    mocks.cardActionSubmit.mockResolvedValue({
      data: {
        success: true,
        message: `Open ${WEB_FETCH_URL} with the browser tool.`,
      },
    })

    const wrapper = await mountChatView(makeTypelessBlock({
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
    }))

    const button = findButtonByText(wrapper, 'Use browser')
    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.chatStore.fetchConversations).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(`Open ${WEB_FETCH_URL} with the browser tool.`)
  })

  it('submits browser extract action from ChatView with browser_target_id intact', async () => {
    mocks.cardActionSubmit.mockResolvedValue({
      data: {
        success: true,
        message: `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
      },
    })

    const wrapper = await mountChatView(makeTypelessBlock({
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
    }))

    const button = findButtonByText(wrapper, 'Extract readable content')
    expect(button?.exists()).toBe(true)

    await button!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledWith('conv-1', 'msg-1', {
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
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`
    )
  })

  it('preserves the Reddit login-wall action chain across assistant messages in ChatView', async () => {
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

    const wrapper = await mountChatViewWithMessages([
      {
        id: 'msg-1',
        content: makeTypelessBlock({
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
        }),
      },
      {
        id: 'msg-2',
        content: makeTypelessBlock({
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
        }),
      },
    ])

    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    const extractButton = findButtonByText(wrapper, 'Extract readable content')

    expect(useBrowserButton?.exists()).toBe(true)
    expect(extractButton?.exists()).toBe(true)

    await useBrowserButton!.trigger('click')
    await flushPromises()

    await extractButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(2)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(1, 'conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(2, 'conv-1', 'msg-2', {
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
    expect(mocks.chatStore.sendMessage.mock.calls.map(call => call[0])).toEqual([
      `Open ${WEB_FETCH_URL} with the browser tool.`,
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
    ])
  })
  it('renders a streamed browser follow-up after use_browser and preserves extract form_data', async () => {
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

    const wrapper = await mountChatView(makeTypelessBlock({
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
    }))

    const useBrowserButton = findButtonByText(wrapper, 'Use browser')
    expect(useBrowserButton?.exists()).toBe(true)

    await useBrowserButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(1)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(1, 'conv-1', 'msg-1', {
      card_id: WEB_FETCH_CARD_ID,
      action_id: 'use_browser',
      action_label: 'Use browser',
      card_type: 'web-fetch',
      card_title: 'Sign in',
      form_data: { url: WEB_FETCH_URL },
    })
    expect(mocks.chatStore.sendMessage).toHaveBeenCalledWith(`Open ${WEB_FETCH_URL} with the browser tool.`)

    mocks.chatStore.messages = [
      ...mocks.chatStore.messages,
      makeAssistantMessage(
        makeTypelessBlock({
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
        }),
        'msg-2'
      ),
    ]

    wrapper.vm.$forceUpdate()
    await wrapper.vm.$nextTick()
    await flushPromises()
    await vi.dynamicImportSettled()
    await flushPromises()

    const extractButton = findButtonByText(wrapper, 'Extract readable content')
    expect(extractButton?.exists()).toBe(true)
    expect(wrapper.text()).toContain('Browser page')

    await extractButton!.trigger('click')
    await flushPromises()

    expect(mocks.cardActionSubmit).toHaveBeenCalledTimes(2)
    expect(mocks.cardActionSubmit).toHaveBeenNthCalledWith(2, 'conv-1', 'msg-2', {
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
    expect(mocks.chatStore.sendMessage.mock.calls.map(call => call[0])).toEqual([
      `Open ${WEB_FETCH_URL} with the browser tool.`,
      `Use web_fetch on ${WEB_FETCH_URL} with browser_target_id=tab-42 to extract readable content.`,
    ])
  })

})
