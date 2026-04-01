import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'

const chatStore = reactive({
  messages: [] as Array<Record<string, unknown>>,
  selectedMessageIds: new Set<string>(),
  isMultiSelectMode: false,
  toolExecuting: false,
  toolExecutingCommands: [] as string[],
  toolExecutingNames: [] as string[],
  toolExecutingStartTime: 0,
  toolResults: [] as unknown[],
  toolSandboxAvailable: false,
  streamProgress: null as string | null,
  statusSummary: null as string | null,
  statusStartedAt: 0,
  processTrace: [] as Array<Record<string, unknown>>,
  awaitingConfirmation: false,
  pendingQuestion: null as unknown,
  pendingApproval: null as unknown,
  pendingExecApproval: null as unknown,
  sendMessage: vi.fn(),
  getMessageMetadata: vi.fn(() => null),
  toggleMessageSelection: vi.fn(),
  enterMultiSelectMode: vi.fn(),
  deleteSelectedMessages: vi.fn(),
})

const settingsStore = reactive({
  showToolDetails: true,
})

vi.mock('@/stores/chat', () => ({
  useChatStore: () => chatStore,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => settingsStore,
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => ({
    providers: [],
    getProviderDisplayName: (providerId: string) => providerId,
  }),
}))

vi.mock('@/stores/notification', () => ({
  useNotificationStore: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    remove: vi.fn(),
  }),
}))

vi.mock('@/api/chat', () => ({
  cardActionApi: {
    submit: vi.fn(),
  },
}))

vi.mock('@/api/voice', () => ({
  ttsAudioManager: {
    stop: vi.fn(),
  },
  streamingTTSManager: {
    stop: vi.fn(),
    streamAndPlay: vi.fn(),
    reset: vi.fn(),
    streamText: vi.fn(),
    play: vi.fn(),
    onComplete: null,
  },
}))

vi.mock('@/api/speech', () => ({
  speechApi: {
    getStatus: vi.fn().mockResolvedValue({ data: {} }),
  },
}))

vi.mock('@/utils/markdown', () => ({
  renderMarkdownCached: (text: string) => text,
  copyCodeToClipboard: vi.fn(),
}))

vi.mock('@/utils/chat-message-text', () => ({
  stripFirstLineHeading: (text: string) => text,
}))

vi.mock('@/utils/typeless', () => ({
  parseTypelessContent: vi.fn(() => null),
  parseTypelessContentIncremental: vi.fn(() => null),
  splitIntoSegments: vi.fn(() => []),
  hasTypelessCards: vi.fn(() => false),
  clearIncrementalState: vi.fn(),
  clearSplitSegmentsIncrementalState: vi.fn(),
}))

function makeMessage(content = '') {
  return {
    id: 'assistant-msg-1',
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-03-17T00:00:00.000Z',
  }
}

async function mountStreamingMessage(content = '') {
  const wrapper = mount(ChatMessage, {
    props: {
      message: makeMessage(content),
      isStreaming: true,
      disableAutoTTS: true,
    },
    global: {
      plugins: [i18n],
      stubs: {
        MediaPlaceholder: true,
        Teleport: true,
        ToolDetailCard: {
          props: ['item'],
          template: '<div class="tool-detail-card-stub">{{ item.status }}</div>',
        },
        Transition: true,
        TypelessCardComponent: true,
      },
    },
  })

  await flushPromises()
  return wrapper
}

function makeActiveStreamState(overrides: Record<string, unknown> = {}) {
  return {
    phase: 'executing',
    awaitingConfirmation: false,
    toolExecuting: false,
    toolExecutingCommands: [] as string[],
    toolExecutingNames: [] as string[],
    toolSandboxAvailable: false,
    statusSummary: null as string | null,
    streamProgress: null as string | null,
    processTrace: [] as Array<Record<string, unknown>>,
    toolResults: [] as Array<Record<string, unknown>>,
    statusStartedAt: 0,
    showExternalStatusRail: false,
    ...overrides,
  }
}

describe('ChatMessage status bar', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:05.000Z'))
    chatStore.messages = []
    chatStore.toolExecuting = false
    chatStore.toolExecutingCommands = []
    chatStore.toolExecutingNames = []
    chatStore.toolExecutingStartTime = 0
    chatStore.toolResults = []
    chatStore.toolSandboxAvailable = false
    chatStore.streamProgress = null
    chatStore.statusSummary = null
    chatStore.statusStartedAt = 0
    chatStore.processTrace = []
    chatStore.awaitingConfirmation = false
    chatStore.pendingQuestion = null
    chatStore.pendingApproval = null
    chatStore.pendingExecApproval = null
    settingsStore.showToolDetails = true
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows the unified thinking status with three dots for empty streaming replies', async () => {
    chatStore.statusStartedAt = Date.now() - 3200

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.find('.assistant-status-bar').exists()).toBe(true)
    expect(wrapper.find('.tool-dots').exists()).toBe(true)
    expect(wrapper.text()).toContain('Thinking...')
    expect(wrapper.text()).toContain('3.2s')
  })

  it('shows concise tool progress details while tools are running', async () => {
    chatStore.toolExecuting = true
    chatStore.toolExecutingCommands = ['blue search --web OpenAI latest']
    chatStore.statusSummary = 'Searching the web'
    chatStore.statusStartedAt = Date.now() - 1800

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.text()).toContain('Searching the web')
    expect(wrapper.text()).toContain('search')
    expect(wrapper.text()).toContain('1.8s')
  })

  it('prioritizes confirmation messaging over other restored status summaries', async () => {
    chatStore.statusSummary = 'Reading example.com'
    chatStore.statusStartedAt = Date.now() - 2100
    chatStore.processTrace = [
      {
        id: 'retry-1',
        source: 'server',
        event: 'pre_content_retry_started',
        category: 'retry',
        status: 'active',
        label: 'Retrying request',
        timestamp: Date.now() - 900,
      },
    ]
    chatStore.awaitingConfirmation = true

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.get('.assistant-status-label').text()).toBe(
      'Waiting for your confirmation to continue'
    )
    expect(wrapper.get('.assistant-status-label').text()).not.toContain('Reading example.com')
  })

  it('prioritizes active recovery traces over generic status summaries', async () => {
    chatStore.statusSummary = 'Writing response...'
    chatStore.statusStartedAt = Date.now() - 1700
    chatStore.processTrace = [
      {
        id: 'retry-2',
        source: 'server',
        event: 'continuation_recovery_started',
        category: 'recovery',
        status: 'active',
        label: 'Recovering response',
        timestamp: Date.now() - 500,
      },
    ]

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.text()).toContain('Recovering response')
    expect(wrapper.text()).not.toContain('Writing response...')
  })

  it('lets users manually expand process details even when default expansion is disabled', async () => {
    settingsStore.showToolDetails = false
    chatStore.processTrace = [
      {
        id: 'summary-1',
        source: 'client',
        event: 'request_summary',
        category: 'summary',
        status: 'info',
        label: 'Request ready',
        timestamp: Date.now() - 1200,
        detail: 'message: Need help',
      },
    ]

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.find('.assistant-process-toggle').exists()).toBe(true)
    expect(wrapper.find('.assistant-process-trace-panel').exists()).toBe(false)

    await wrapper.get('.assistant-process-toggle').trigger('click')

    expect(wrapper.find('.assistant-process-trace-panel').exists()).toBe(true)
    expect(wrapper.findAll('.assistant-process-trace-item')).toHaveLength(1)
    expect(wrapper.text()).toContain('Request ready')
  })

  it('renders streaming process traces as one continuous panel instead of separate cards', async () => {
    chatStore.processTrace = [
      {
        id: 'summary-1',
        source: 'client',
        event: 'request_summary',
        category: 'summary',
        status: 'info',
        label: 'Request ready',
        timestamp: Date.now() - 1500,
        command: 'Please review this UI',
        detail: 'message: Please review this UI',
      },
      {
        id: 'dispatch-1',
        source: 'client',
        event: 'request_dispatched',
        category: 'lifecycle',
        status: 'active',
        label: 'Request sent',
        timestamp: Date.now() - 1200,
        detail: 'Waiting for the server to accept and start the response.',
      },
      {
        id: 'waiting-1',
        source: 'client',
        event: 'waiting_for_response',
        category: 'lifecycle',
        status: 'active',
        label: 'Waiting for response',
        timestamp: Date.now() - 900,
        detail: 'The request was accepted. Waiting for the first visible output.',
      },
    ]

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.find('.assistant-process-trace-panel').exists()).toBe(true)
    expect(wrapper.findAll('.assistant-process-trace-item')).toHaveLength(3)
    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(0)
    expect(wrapper.text()).toContain('Request ready')
    expect(wrapper.text()).toContain('Request sent')
    expect(wrapper.text()).toContain('Waiting for response')
  })

  it('ignores global streaming status changes for non-streaming assistant messages', async () => {
    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('Finished response'),
        disableAutoTTS: true,
      },
      global: {
        plugins: [i18n],
        stubs: {
          MediaPlaceholder: true,
          Teleport: true,
          ToolDetailCard: {
            props: ['item'],
            template: '<div class="tool-detail-card-stub">{{ item.status }}</div>',
          },
          Transition: true,
          TypelessCardComponent: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.find('.assistant-status-bar').exists()).toBe(false)

    chatStore.toolExecuting = true
    chatStore.statusSummary = 'Searching the web'
    chatStore.toolResults = [{ id: 'tool-1', status: 'success' }]
    chatStore.processTrace = [
      {
        id: 'trace-1',
        source: 'server',
        event: 'waiting_for_response',
        category: 'lifecycle',
        status: 'active',
        label: 'Waiting for response',
        timestamp: Date.now(),
      },
    ]
    await nextTick()
    await flushPromises()

    expect(wrapper.find('.assistant-status-bar').exists()).toBe(false)
    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('Searching the web')
  })

  it('prefers explicit streamState props over the global chat store state', async () => {
    chatStore.awaitingConfirmation = true
    chatStore.statusSummary = 'Global status'

    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage(''),
        isStreaming: true,
        disableAutoTTS: true,
        streamState: makeActiveStreamState({
          toolExecuting: true,
          toolExecutingCommands: ['blue search --web OpenAI latest'],
          toolSandboxAvailable: true,
          statusSummary: 'Searching the web',
          toolResults: [{ id: 'tool-1', status: 'running' }],
          statusStartedAt: Date.now() - 1500,
        }),
      },
      global: {
        plugins: [i18n],
        stubs: {
          MediaPlaceholder: true,
          Teleport: true,
          ToolDetailCard: {
            props: ['item'],
            template: '<div class="tool-detail-card-stub">{{ item.status }}</div>',
          },
          Transition: true,
          TypelessCardComponent: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.assistant-status-bar').exists()).toBe(true)
    expect(wrapper.text()).toContain('Searching the web')
    expect(wrapper.text()).not.toContain('Waiting for your confirmation to continue')
    expect(wrapper.text()).toContain('search')
    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(1)
  })
})
