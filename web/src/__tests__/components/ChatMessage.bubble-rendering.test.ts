import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'
import {
  hasTypelessCards,
  parseTypelessContent,
  parseTypelessContentIncremental,
  splitIntoSegments,
} from '@/utils/typeless'

const chatStore = reactive({
  messages: [] as Array<Record<string, unknown>>,
  selectedMessageIds: new Set<string>(),
  isMultiSelectMode: false,
  streamUIState: { phase: 'idle' },
  toolExecuting: false,
  toolExecutingCommands: [] as string[],
  toolExecutingNames: [] as string[],
  toolExecutingStartTime: 0,
  toolResults: [] as unknown[],
  toolSandboxAvailable: false,
  processTrace: [] as unknown[],
  statusStartedAt: 0,
  statusSummary: null as string | null,
  streamProgress: null as string | null,
  awaitingConfirmation: false,
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

function makeMessage(role: 'assistant' | 'user', content: string) {
  return {
    id: `${role}-msg-1`,
    conversation_id: 'conv-1',
    role,
    content,
    created_at: '2026-03-14T00:00:00.000Z',
  }
}

async function mountMessage(role: 'assistant' | 'user', content: string) {
  const wrapper = mount(ChatMessage, {
    props: {
      message: makeMessage(role, content),
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

describe('ChatMessage bubble rendering', () => {
  beforeEach(() => {
    settingsStore.showToolDetails = true
    chatStore.streamUIState = { phase: 'idle' }
    chatStore.toolExecuting = false
    chatStore.toolExecutingCommands = []
    chatStore.toolExecutingNames = []
    chatStore.toolExecutingStartTime = 0
    chatStore.toolResults = []
    chatStore.toolSandboxAvailable = false
    chatStore.processTrace = []
    chatStore.statusStartedAt = 0
    chatStore.statusSummary = null
    chatStore.streamProgress = null
    chatStore.awaitingConfirmation = false
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
    vi.mocked(hasTypelessCards).mockReturnValue(false)
    vi.mocked(parseTypelessContent).mockReturnValue(null)
    vi.mocked(parseTypelessContentIncremental).mockReturnValue(null)
    vi.mocked(splitIntoSegments).mockReturnValue([])
  })

  it('renders normal assistant replies with the bordered assistant bubble class', async () => {
    const wrapper = await mountMessage('assistant', 'Blue reply')
    const bubble = wrapper.get('.chat-assistant-bubble')

    expect(bubble.classes()).toContain('assistant-message')
    expect(bubble.classes()).toContain('chat-copy-bubble')
    expect(bubble.classes()).not.toContain('assistant-message-indicator-only')
    expect(bubble.classes()).toContain('px-4')
    expect(bubble.classes()).toContain('py-3')
    expect(wrapper.get('.prose-content').text()).toBe('Blue reply')
  })

  it('renders a stopped indicator instead of exposing the raw stopped marker text', async () => {
    const wrapper = await mountMessage('assistant', '[Response stopped]')

    expect(wrapper.find('.response-stopped-indicator').exists()).toBe(true)
    expect(wrapper.find('.response-stopped-indicator').text().trim()).not.toBe('')
    expect(wrapper.html()).not.toContain('[Response stopped]')
  })

  it('renders the stopped indicator after a streaming placeholder transitions into a stopped reply', async () => {
    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('assistant', ''),
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
      message: makeMessage('assistant', '[Response stopped]'),
      isStreaming: false,
    })
    await flushPromises()

    expect(wrapper.find('.response-stopped-indicator').exists()).toBe(true)
    expect(wrapper.html()).not.toContain('[Response stopped]')
  })

  it('keeps the streaming caret visible during executing phases with assistant text', async () => {
    chatStore.streamUIState = { phase: 'executing' }
    chatStore.toolExecuting = true
    chatStore.statusStartedAt = Date.now()
    chatStore.statusSummary = 'Running tool'

    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('assistant', 'Blue reply'),
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

    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(true)
    expect(wrapper.find('.assistant-status-bar').exists()).toBe(true)
    expect(wrapper.find('.streaming-caret').exists()).toBe(true)
  })

  it('keeps the streaming caret visible when only the assistant status bubble is shown', async () => {
    chatStore.streamUIState = { phase: 'executing' }
    chatStore.toolExecuting = true
    chatStore.statusStartedAt = Date.now()
    chatStore.statusSummary = 'Running tool'

    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('assistant', ''),
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

    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(true)
    expect(wrapper.findAll('.assistant-message-block')).toHaveLength(0)
    expect(wrapper.find('.assistant-status-bar').exists()).toBe(true)
    expect(wrapper.find('.streaming-caret').exists()).toBe(true)
  })

  it('keeps markdown sections inside a single assistant bubble instead of splitting them apart', async () => {
    const wrapper = await mountMessage(
      'assistant',
      '## Summary\\n\\nFirst paragraph.\\n\\n> A quoted note\\n\\nFinal paragraph.'
    )

    expect(wrapper.findAll('.chat-assistant-bubble')).toHaveLength(1)
    expect(wrapper.findAll('.prose-content')).toHaveLength(1)
  })

  it('renders user replies with the user bubble class instead of assistant bubble styles', async () => {
    const wrapper = await mountMessage('user', 'User prompt')
    const bubble = wrapper.get('.chat-user-bubble')

    expect(bubble.exists()).toBe(true)
    expect(bubble.classes()).toContain('chat-copy-bubble')
    expect(bubble.classes()).toContain('px-4')
    expect(bubble.classes()).toContain('py-2')
    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(false)
  })

  it('keeps card-only deep research timelines inside the assistant bubble', async () => {
    const timelineCard = {
      type: 'deep-research-timeline',
      id: 'timeline-1',
      query: 'EU AI Act provider obligations',
      status: 'running',
      progress: 42,
      steps: [
        {
          type: 'deep-research-event',
          id: 'timeline-event-1',
          event_kind: 'planning',
          status: 'info',
          summary: 'Planned 5 research task(s)',
        },
      ],
    } as any

    vi.mocked(hasTypelessCards).mockReturnValue(true)
    vi.mocked(parseTypelessContent).mockReturnValue({
      text: '[[TYPELESS_CARD:timeline-1]]',
      cards: [timelineCard],
    } as any)
    vi.mocked(splitIntoSegments).mockReturnValue([{ type: 'card', content: timelineCard }] as any)

    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('assistant', '[[TYPELESS_CARD:timeline-1]]'),
        disableAutoTTS: true,
      },
      global: {
        plugins: [i18n],
        stubs: {
          MediaPlaceholder: true,
          Teleport: true,
          ToolDetailCard: true,
          Transition: true,
          TypelessCardComponent: {
            props: ['card'],
            template: '<div class="typeless-card-stub">{{ card.type }}</div>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(true)
    expect(wrapper.find('.typeless-card-stub').exists()).toBe(true)
    expect(wrapper.text()).toContain('deep-research-timeline')
  })

  it('keeps non-timeline card-only assistant messages rendered without the bubble wrapper', async () => {
    const resultCard = {
      type: 'result',
      id: 'result-1',
      title: 'Done',
      status: 'success',
    } as any

    vi.mocked(hasTypelessCards).mockReturnValue(true)
    vi.mocked(parseTypelessContent).mockReturnValue({
      text: '[[TYPELESS_CARD:result-1]]',
      cards: [resultCard],
    } as any)
    vi.mocked(splitIntoSegments).mockReturnValue([{ type: 'card', content: resultCard }] as any)

    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('assistant', '[[TYPELESS_CARD:result-1]]'),
        disableAutoTTS: true,
      },
      global: {
        plugins: [i18n],
        stubs: {
          MediaPlaceholder: true,
          Teleport: true,
          ToolDetailCard: true,
          Transition: true,
          TypelessCardComponent: {
            props: ['card'],
            template: '<div class="typeless-card-stub">{{ card.type }}</div>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(false)
    expect(wrapper.find('.typeless-card-stub').exists()).toBe(true)
    expect(wrapper.text()).toContain('result')
  })

  it('keeps advisor result cards visible when tool details are hidden', async () => {
    settingsStore.showToolDetails = false

    const advisorCard = {
      type: 'advisor',
      id: 'advisor-card-hidden-tools',
      recommendation: 'Prefer Go for the API edge.',
      confidence: 0.8,
    } as any

    vi.mocked(hasTypelessCards).mockReturnValue(true)
    vi.mocked(parseTypelessContent).mockReturnValue({
      text: '[[TYPELESS_CARD:advisor-card-hidden-tools]]',
      cards: [advisorCard],
    } as any)
    vi.mocked(splitIntoSegments).mockReturnValue([{ type: 'card', content: advisorCard }] as any)

    const wrapper = mount(ChatMessage, {
      props: {
        message: makeMessage('assistant', '[[TYPELESS_CARD:advisor-card-hidden-tools]]'),
        disableAutoTTS: true,
      },
      global: {
        plugins: [i18n],
        stubs: {
          MediaPlaceholder: true,
          Teleport: true,
          ToolDetailCard: true,
          Transition: true,
          TypelessCardComponent: {
            props: ['card'],
            template: '<div class="typeless-card-stub">{{ card.type }}</div>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.typeless-card-stub').exists()).toBe(true)
    expect(wrapper.text()).toContain('advisor')
  })

  it('keeps a completed assistant bubble visible when only local process details remain', async () => {
    const wrapper = mount(ChatMessage, {
      props: {
        message: {
          ...makeMessage('assistant', ''),
          local_process_tool_results: [
            {
              name: 'web_search',
              id: 'tool-search-1',
              command: 'Need sources',
              icon: '✓',
              status: 'Found 3 results',
              output: 'Source A\nSource B',
              timestamp: Date.now(),
            },
          ],
        },
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

    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(true)
    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(1)
    expect(wrapper.text()).toContain('Found 3 results')
  })

  it('keeps an empty assistant bubble visible with a toggle when local process details are collapsed', async () => {
    settingsStore.showToolDetails = false

    const wrapper = mount(ChatMessage, {
      props: {
        message: {
          ...makeMessage('assistant', ''),
          local_process_tool_results: [
            {
              name: 'web_search',
              id: 'tool-search-2',
              command: 'Need sources',
              icon: '✓',
              status: 'Found 3 results',
              output: 'Source A\nSource B',
              timestamp: Date.now(),
            },
          ],
        },
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

    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(true)
    expect(wrapper.find('.assistant-process-toggle').exists()).toBe(true)
    expect(wrapper.get('.assistant-message-shell').classes()).toContain(
      'assistant-message-shell--toggle-only'
    )
    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(0)
  })

  it('syncs a message-level process toggle when the global work-details setting is hidden again', async () => {
    settingsStore.showToolDetails = false

    const wrapper = mount(ChatMessage, {
      props: {
        message: {
          ...makeMessage('assistant', ''),
          local_process_tool_results: [
            {
              name: 'web_search',
              id: 'tool-search-3',
              command: 'Need sources',
              icon: '✓',
              status: 'Found 3 results',
              output: 'Source A\nSource B',
              timestamp: Date.now(),
            },
          ],
        },
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

    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(0)
    const collapsedLabel = wrapper.get('.assistant-process-toggle').text()

    await wrapper.get('.assistant-process-toggle').trigger('click')
    await flushPromises()

    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(1)
    const expandedLabel = wrapper.get('.assistant-process-toggle').text()
    expect(expandedLabel).not.toBe(collapsedLabel)

    settingsStore.showToolDetails = true
    await flushPromises()

    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(1)
    expect(wrapper.get('.assistant-process-toggle').text()).toBe(expandedLabel)

    settingsStore.showToolDetails = false
    await flushPromises()

    expect(wrapper.findAll('.tool-detail-card-stub')).toHaveLength(0)
    expect(wrapper.get('.assistant-process-toggle').text()).toBe(collapsedLabel)
  })
})
