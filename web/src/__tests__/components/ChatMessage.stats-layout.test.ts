import { beforeEach, describe, expect, it, vi } from 'vitest'
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

describe('ChatMessage stats layout', () => {
  beforeEach(() => {
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
  })

  it('renders assistant message stats in duration, speed, input, output order with time on the right', async () => {
    const createdAt = '2026-03-14T11:15:00.000Z'
    const wrapper = mount(ChatMessage, {
      props: {
        message: {
          id: 'assistant-msg-1',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: 'reply',
          created_at: createdAt,
          stats: {
            input_tokens: 3117,
            output_tokens: 241,
            total_tokens: 3358,
            latency_ms: 3940,
            ttft_ms: 820,
            tokens_per_second: 20.2,
          },
        },
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

    const metrics = wrapper.findAll('.message-stat-chip').map((node) => node.text())
    const expectedTime = new Date(createdAt).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    })

    expect(metrics).toEqual(['3.94s', '20.2 tokens/s', '3,117', '241'])
    expect(wrapper.get('.message-meta-time').text()).toBe(expectedTime)
  })
})
