import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
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

vi.mock('@/stores/chat', () => ({
  useChatStore: () => chatStore,
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    showToolDetails: true,
  }),
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
        ToolDetailCard: true,
        Transition: true,
        TypelessCardComponent: true,
      },
    },
  })

  await flushPromises()
  return wrapper
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
    chatStore.awaitingConfirmation = false
    chatStore.pendingQuestion = null
    chatStore.pendingApproval = null
    chatStore.pendingExecApproval = null
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
    chatStore.awaitingConfirmation = true

    const wrapper = await mountStreamingMessage('')

    expect(wrapper.text()).toContain('Waiting for your confirmation to continue')
    expect(wrapper.text()).not.toContain('Reading example.com')
  })
})
