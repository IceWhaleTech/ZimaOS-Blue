import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'

const chatStore = reactive({
  messages: [] as Array<Record<string, unknown>>,
  selectedMessageIds: new Set<string>(),
  isMultiSelectMode: false,
  processTrace: [] as unknown[],
  toolExecuting: false,
  toolExecutingCommands: [] as string[],
  toolExecutingNames: [] as string[],
  toolExecutingStartTime: 0,
  toolResults: [] as unknown[],
  toolSandboxAvailable: false,
  awaitingConfirmation: false,
  statusSummary: '',
  statusStartedAt: 0,
  streamProgress: '',
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

function makeMessage(content: string) {
  return {
    id: 'msg-1',
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-03-11T00:00:00.000Z',
  }
}

describe('ChatMessage streaming flush', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    chatStore.messages = []
    chatStore.processTrace = []
    chatStore.toolExecuting = false
    chatStore.toolExecutingCommands = []
    chatStore.toolExecutingNames = []
    chatStore.toolExecutingStartTime = 0
    chatStore.toolResults = []
    chatStore.toolSandboxAvailable = false
    chatStore.awaitingConfirmation = false
    chatStore.statusSummary = ''
    chatStore.statusStartedAt = 0
    chatStore.streamProgress = ''
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the first streamed token immediately', async () => {
    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage(''),
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
    expect(wrapper.text()).not.toContain('你')

    await wrapper.setProps({
      message: makeMessage('你'),
    })
    await nextTick()

    expect(wrapper.text()).toContain('你')
  })

  it('flushes deferred streaming text before tool execution UI appears', async () => {
    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('正在创建 PPT 结构大纲和'),
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
    expect(wrapper.text()).toContain('正在创建 PPT 结构大纲和')

    await wrapper.setProps({
      message: makeMessage('正在创建 PPT 结构大纲和内容'),
    })
    await nextTick()

    expect(wrapper.text()).toContain('正在创建 PPT 结构大纲和')
    expect(wrapper.text()).not.toContain('正在创建 PPT 结构大纲和内容')

    chatStore.toolExecuting = true
    await nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('正在创建 PPT 结构大纲和内容')
  })

  it('reveals bursty streaming updates in short phrase chunks instead of pure per-character typing', async () => {
    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage(''),
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

    await wrapper.setProps({
      message: makeMessage('hello world from blue'),
    })
    await nextTick()

    expect(wrapper.text()).toContain('h')
    expect(wrapper.text()).not.toContain('hello world from blue')

    vi.advanceTimersByTime(18)
    await nextTick()

    expect(wrapper.text()).toContain('hello world')
    expect(wrapper.text()).not.toContain('hello world from blue')

    vi.runAllTimers()
    await nextTick()

    expect(wrapper.text()).toContain('hello world from blue')
  })
})
