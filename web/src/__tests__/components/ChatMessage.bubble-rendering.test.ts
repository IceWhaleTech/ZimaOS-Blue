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
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
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

  it('renders user replies with the user bubble class instead of assistant bubble styles', async () => {
    const wrapper = await mountMessage('user', 'User prompt')
    const bubble = wrapper.get('.chat-user-bubble')

    expect(bubble.exists()).toBe(true)
    expect(bubble.classes()).toContain('chat-copy-bubble')
    expect(bubble.classes()).toContain('px-4')
    expect(bubble.classes()).toContain('py-2')
    expect(wrapper.find('.chat-assistant-bubble').exists()).toBe(false)
  })
})
