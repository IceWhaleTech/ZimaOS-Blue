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

function makeMessage(role: 'assistant' | 'user') {
  return {
    id: `${role}-msg-1`,
    conversation_id: 'conv-1',
    role,
    content: role === 'assistant' ? 'Blue reply' : 'User prompt',
    created_at: '2026-03-14T00:00:00.000Z',
  }
}

async function mountMessage(role: 'assistant' | 'user') {
  const wrapper = mount(ChatMessage, {
    props: {
      message: makeMessage(role),
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

describe('ChatMessage avatar swap', () => {
  beforeEach(() => {
    settingsStore.showToolDetails = true
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
  })

  it('keeps avatar positions and only swaps avatar colors', async () => {
    const assistantWrapper = await mountMessage('assistant')
    const assistantRow = assistantWrapper.find('.message > .flex').element as HTMLDivElement
    const assistantChildren = Array.from(assistantRow.children)
    const assistantAvatar = assistantWrapper.find('.avatar')

    expect(assistantChildren[0]?.className).toContain('avatar')
    expect(assistantAvatar.classes()).toContain('avatar-user')
    expect(assistantWrapper.find('.chat-assistant-bubble-swapped').exists()).toBe(false)

    const userWrapper = await mountMessage('user')
    const userRow = userWrapper.find('.message > .flex').element as HTMLDivElement
    const userChildren = Array.from(userRow.children)
    const userAvatar = userWrapper.find('.avatar')

    expect(userChildren[userChildren.length - 1]?.className).toContain('avatar')
    expect(userAvatar.classes()).toContain('avatar-assistant')
    expect(userWrapper.find('.chat-user-bubble-swapped').exists()).toBe(false)
  })
})
