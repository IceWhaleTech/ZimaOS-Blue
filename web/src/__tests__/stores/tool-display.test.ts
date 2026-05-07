/**
 * RED tests for tool call display bugs.
 *
 * Bug 1: showToolDetails defaults to false (should be true)
 * Bug 2: fetchMessages loses local_process_tool_results
 * Bug 3: Assistant message with tool_calls but no text is hidden
 * Bug 4: Tool detail cards not visible by default when message has tool results
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { reactive } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { useChatStore } from '@/stores/chat'
import type { ToolResultItem } from '@/stores/chat'
import { messageApi } from '@/api/chat'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'

// ─── Shared mocks (hoisted) ────────────────────────────────────────────────

const mocks = vi.hoisted(() => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPatch: vi.fn(),
  apiDelete: vi.fn(),
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    create: vi.fn(),
    list: vi.fn(),
    get: vi.fn(),
    delete: vi.fn(),
    pin: vi.fn(),
    unpin: vi.fn(),
    search: vi.fn(),
    patchCommandState: vi.fn(),
  },
  messageApi: {
    list: vi.fn(),
    send: vi.fn(),
    delete: vi.fn(),
    cancelStream: vi.fn(),
    getStreamUrl: vi.fn(),
  },
  warmupApi: {
    trigger: vi.fn(),
    cancel: vi.fn(),
  },
  injectionApi: {
    inject: vi.fn(),
  },
  cardActionApi: {
    submit: vi.fn(),
  },
  toolApi: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/client', () => ({
  default: {
    get: (...args: unknown[]) => mocks.apiGet(...args),
    post: (...args: unknown[]) => mocks.apiPost(...args),
    patch: (...args: unknown[]) => mocks.apiPatch(...args),
    delete: (...args: unknown[]) => mocks.apiDelete(...args),
  },
}))

vi.mock('@/api/providerPool', () => ({
  providerPoolApi: {
    listProviders: vi.fn(),
    fetchProviderModels: vi.fn(),
  },
}))

vi.mock('@/api/settings', () => ({
  settingsApi: {
    get: vi.fn(),
    update: vi.fn(),
    patch: vi.fn(),
    getSmallModelStatus: vi.fn(),
    downloadSmallModel: vi.fn(),
    cancelSmallModelDownload: vi.fn(),
    getSmallModelStats: vi.fn(),
    resetSmallModelStats: vi.fn(),
  },
}))

vi.mock('@/api/approval', () => ({
  approvalApi: {
    getConfig: vi.fn(),
    updateConfig: vi.fn(),
    resolve: vi.fn(),
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
  hasTypelessCardsForMessage: vi.fn(() => false),
  clearIncrementalState: vi.fn(),
  clearSplitSegmentsIncrementalState: vi.fn(),
}))

vi.mock('@/utils/ttsPreferences', () => ({
  isTtsAutoPlayEnabled: vi.fn(() => false),
  isTtsSpeechMuted: vi.fn(() => false),
}))

vi.mock('@/utils/chatCardUiState', () => ({
  buildChatCardUiStateKey: vi.fn(() => 'test-key'),
}))

vi.mock('@/utils/toolLocalization', () => ({
  getLocalizedToolName: vi.fn((name: string) => name),
}))

vi.mock('@/utils/chatPerf', () => ({
  measureChatPerf: vi.fn(),
  recordChatPerfCount: vi.fn(),
}))

vi.mock('@/utils/processTrace', () => ({
  createProcessTraceItem: vi.fn(),
}))

vi.mock('@/utils/completionFollowupText', () => ({
  localizeCompletionFollowupHeading: vi.fn(() => ''),
}))

vi.mock('@/utils/todoChecklist', () => ({
  stripDuplicateTodoChecklistForMessage: vi.fn((_content: string, msg: unknown) => ''),
}))

// ─── Component-test chat store mock ────────────────────────────────────────

let mockChatStoreState: Record<string, unknown> | null = null

function resetMockChatStore() {
  mockChatStoreState = reactive({
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
  return mockChatStoreState
}

vi.mock('@/stores/chat', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/stores/chat')>()
  return {
    ...actual,
    useChatStore: () => (mockChatStoreState !== null ? mockChatStoreState : actual.useChatStore()),
  }
})

// ─── Component-test settings store mock ────────────────────────────────────

let mockShowToolDetails: boolean | null = null

vi.mock('@/stores/settings', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/stores/settings')>()
  return {
    ...actual,
    useSettingsStore: () =>
      mockShowToolDetails !== null
        ? { showToolDetails: mockShowToolDetails }
        : actual.useSettingsStore(),
  }
})

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

// ─── Helpers ───────────────────────────────────────────────────────────────

function makeAssistantMessage(
  id: string,
  content: string,
  extra: Record<string, unknown> = {}
) {
  return {
    id,
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-05-07T00:00:00.000Z',
    ...extra,
  }
}

// ═══════════════════════════════════════════════════════════════════════════
// Bug 1: showToolDetails defaults to false
// ═══════════════════════════════════════════════════════════════════════════

describe('Bug 1: settings store showToolDetails default', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('showToolDetails defaults to true for a fresh profile (no localStorage)', () => {
    const store = useSettingsStore()
    expect(store.showToolDetails).toBe(true)
  })

  it('showToolDetails is true when localStorage has no stored value', () => {
    // Ensure nothing is stored
    localStorage.removeItem('zimaos-blue-settings')
    const store = useSettingsStore()
    expect(store.showToolDetails).toBe(true)
  })
})

// ═══════════════════════════════════════════════════════════════════════════
// Bug 2: fetchMessages loses local_process_tool_results
// ═══════════════════════════════════════════════════════════════════════════

describe('Bug 2: fetchMessages preserves local_process_tool_results', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('preserves local_process_tool_results on matching messages after fetch', async () => {
    const store = useChatStore()

    // Set up a message with local_process_tool_results (runtime-only data)
    const toolResults: ToolResultItem[] = [
      {
        id: 'tr-1',
        name: 'exec',
        command: 'ls -la',
        status: 'success',
        output: 'file1.txt',
        icon: '✓',
        timestamp: Date.now(),
      },
    ]

    const msgWithTools = {
      id: 'msg-1',
      conversation_id: 'conv-1',
      role: 'assistant' as const,
      content: 'some text',
      created_at: new Date().toISOString(),
      local_process_tool_results: toolResults,
    }

    // Manually set messages (simulating runtime state after streaming)
    store.messages = [msgWithTools as never]

    // Mock API to return the same message WITHOUT local_process_tool_results
    // (simulating what the API returns -- it doesn't know about runtime fields)
    const apiMessage = {
      id: 'msg-1',
      conversation_id: 'conv-1',
      role: 'assistant',
      content: 'some text',
      created_at: msgWithTools.created_at,
    }

    vi.mocked(messageApi.list).mockResolvedValue({ data: [apiMessage] } as never)

    // Set current conversation so the guard passes
    store.currentConversationId = 'conv-1'

    await store.fetchMessages('conv-1')

    // The message should still have local_process_tool_results
    const msg = store.messages.find((m: { id: string }) => m.id === 'msg-1') as Record<
      string,
      unknown
    >
    expect(msg).toBeDefined()
    expect(msg.local_process_tool_results).toBeDefined()
    expect((msg.local_process_tool_results as unknown[]).length).toBe(1)
    expect((msg.local_process_tool_results as ToolResultItem[])[0].id).toBe('tr-1')
  })

  it('does not add local_process_tool_results when none existed before', async () => {
    const store = useChatStore()

    const apiMessage = {
      id: 'msg-2',
      conversation_id: 'conv-1',
      role: 'assistant',
      content: 'hello',
      created_at: new Date().toISOString(),
    }

    store.messages = [apiMessage as never]
    vi.mocked(messageApi.list).mockResolvedValue({ data: [apiMessage] } as never)
    store.currentConversationId = 'conv-1'

    await store.fetchMessages('conv-1')

    const msg = store.messages.find((m: { id: string }) => m.id === 'msg-2') as Record<
      string,
      unknown
    >
    expect(msg).toBeDefined()
    expect(msg.local_process_tool_results).toBeUndefined()
  })
})

// ═══════════════════════════════════════════════════════════════════════════
// Bug 3: Assistant message with tool_calls but no text is hidden
// ═══════════════════════════════════════════════════════════════════════════

describe('Bug 3: assistant message with tool_calls should not be hidden', () => {
  beforeEach(() => {
    resetMockChatStore()
    mockShowToolDetails = false
  })

  it('does not hide assistant message that has tool_calls but empty content', async () => {
    const message = makeAssistantMessage('msg-tc-1', '', {
      tool_calls: [{ id: 'tc-1', name: 'exec', arguments: '{"command":"ls"}' }],
    })

    const wrapper = mount(ChatMessage, {
      props: {
        message,
        isStreaming: false,
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
          TrustedHtml: true,
        },
      },
    })

    await flushPromises()

    // The root message element uses v-show="!shouldHideMessage"
    // If shouldHideMessage is true, display will be 'none'
    const messageEl = wrapper.find('.message')
    expect(messageEl.exists()).toBe(true)
    expect(messageEl.element.style.display).not.toBe('none')
  })

  it('does not hide assistant message that has tool_calls even with blank content', async () => {
    const message = makeAssistantMessage('msg-tc-2', '   ', {
      tool_calls: [
        { id: 'tc-2', name: 'web_query', arguments: '{"query":"test"}' },
        { id: 'tc-3', name: 'exec', arguments: '{"command":"pwd"}' },
      ],
    })

    const wrapper = mount(ChatMessage, {
      props: {
        message,
        isStreaming: false,
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
          TrustedHtml: true,
        },
      },
    })

    await flushPromises()

    const messageEl = wrapper.find('.message')
    expect(messageEl.exists()).toBe(true)
    expect(messageEl.element.style.display).not.toBe('none')
  })
})

// ═══════════════════════════════════════════════════════════════════════════
// Bug 4: Tool detail cards visible by default when message has tool results
// ═══════════════════════════════════════════════════════════════════════════

describe('Bug 4: tool detail cards visibility with local_process_tool_results', () => {
  beforeEach(() => {
    resetMockChatStore()
    localStorage.clear()
    // Use real settings store to test the default showToolDetails behavior.
    // null = delegate to real store (whose default depends on the code under test).
    mockShowToolDetails = null
  })

  it('shows tool detail cards by default when message has local_process_tool_results', async () => {
    const toolResults: ToolResultItem[] = [
      {
        id: 'tr-1',
        name: 'exec',
        command: 'ls -la',
        status: 'success',
        output: 'file1.txt  file2.txt',
        icon: '✓',
        timestamp: Date.now(),
      },
      {
        id: 'tr-2',
        name: 'exec',
        command: 'cat README.md',
        status: 'success',
        output: '# Project',
        icon: '✓',
        timestamp: Date.now(),
      },
    ]

    const message = makeAssistantMessage('msg-tools-1', 'Here are the results.', {
      local_process_tool_results: toolResults,
    })

    // Also set the message in the mock chat store so the component can access it
    mockChatStoreState.messages = [message]

    const wrapper = mount(ChatMessage, {
      props: {
        message,
        isStreaming: false,
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
          TrustedHtml: true,
        },
      },
    })

    await flushPromises()

    // The tool detail cards container should be rendered (v-if="showPersistedProcessPanel")
    const cardsContainer = wrapper.find('.tool-detail-cards')
    expect(cardsContainer.exists()).toBe(true)
  })

  it('shows process details toggle button by default when message has tool results', async () => {
    const toolResults: ToolResultItem[] = [
      {
        id: 'tr-3',
        name: 'web_query',
        command: 'search query',
        status: 'success',
        output: 'search results',
        icon: '✓',
        timestamp: Date.now(),
      },
    ]

    const message = makeAssistantMessage('msg-tools-2', 'Found results.', {
      local_process_tool_results: toolResults,
    })

    mockChatStoreState.messages = [message]

    const wrapper = mount(ChatMessage, {
      props: {
        message,
        isStreaming: false,
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
          TrustedHtml: true,
        },
      },
    })

    await flushPromises()

    // The toggle button should be visible
    const toggle = wrapper.find('.assistant-process-toggle')
    expect(toggle.exists()).toBe(true)
  })
})
