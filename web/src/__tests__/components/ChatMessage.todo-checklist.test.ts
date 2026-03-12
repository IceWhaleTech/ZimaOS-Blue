import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'

const chatStore = {
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
}

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

function assistantMessage(id: string, content: string, extra: Record<string, unknown> = {}) {
  return {
    id,
    conversation_id: 'conv-1',
    role: 'assistant' as const,
    content,
    created_at: '2026-03-11T00:00:00.000Z',
    ...extra,
  }
}

describe('ChatMessage todo checklist dedupe', () => {
  beforeEach(() => {
    chatStore.messages = []
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
  })

  it('hides duplicate checklist echoes and keeps only the progress text', async () => {
    const canonical = assistantMessage('msg-a1', '- [ ] 收集信息\n- [ ] 写总结', {
      todo_card_id: 'todo-checklist-msg-a1',
    })
    const duplicate = assistantMessage('msg-a3', '- [x] 收集信息\n- [ ] 写总结\n\n我继续执行第二步。')
    chatStore.messages = [
      { id: 'msg-u1', conversation_id: 'conv-1', role: 'user', content: '帮我整理', created_at: '2026-03-11T00:00:00.000Z' },
      canonical,
      { id: 'msg-u2', conversation_id: 'conv-1', role: 'user', content: '继续', created_at: '2026-03-11T00:00:01.000Z' },
      duplicate,
    ]

    const wrapper = mount(ChatMessage, {
      props: {
        message: duplicate,
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

    expect(wrapper.text()).toContain('我继续执行第二步。')
    expect(wrapper.text()).not.toContain('收集信息')
    expect(wrapper.text()).not.toContain('写总结')
  })
})

