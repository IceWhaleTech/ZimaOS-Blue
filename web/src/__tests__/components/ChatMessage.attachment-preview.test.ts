import { Buffer } from 'node:buffer'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import ChatMessage from '@/components/ChatMessage.vue'
import { i18n } from '@/i18n'

const renderMarkdownCachedMock = vi.hoisted(() =>
  vi.fn((text: string) => `<div class="rendered-markdown-preview">${text}</div>`)
)

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
  renderMarkdownCached: renderMarkdownCachedMock,
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

function makeAttachment(name: string, mimeType: string, content: string) {
  return {
    type: 'file' as const,
    name,
    mime_type: mimeType,
    data: Buffer.from(content, 'utf8').toString('base64'),
  }
}

function makeUserMessage(attachment: ReturnType<typeof makeAttachment>) {
  return {
    id: 'user-msg-1',
    conversation_id: 'conv-1',
    role: 'user' as const,
    content: '',
    created_at: '2026-03-22T00:00:00.000Z',
    attachments: [attachment],
  }
}

async function mountMessage(attachment: ReturnType<typeof makeAttachment>) {
  const wrapper = mount(ChatMessage, {
    props: {
      message: makeUserMessage(attachment),
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

describe('ChatMessage attachment preview', () => {
  beforeEach(() => {
    renderMarkdownCachedMock.mockClear()
    chatStore.getMessageMetadata.mockReset().mockReturnValue(null)
  })

  it('renders markdown attachments with markdown preview content', async () => {
    const wrapper = await mountMessage(
      makeAttachment('memory-system-enhancement-checklist.md', 'text/markdown', '# Checklist')
    )

    await wrapper.get('.attachment-preview').trigger('click')
    await flushPromises()

    expect(wrapper.find('.attachment-preview-shell').exists()).toBe(true)
    expect(wrapper.get('.attachment-preview-badge').text()).toBe('MD')
    expect(wrapper.get('.attachment-preview-title').text()).toBe(
      'memory-system-enhancement-checklist.md'
    )
    expect(wrapper.find('.attachment-markdown-preview').exists()).toBe(true)
    expect(wrapper.find('.attachment-text-preview').exists()).toBe(false)
    expect(renderMarkdownCachedMock).toHaveBeenCalledWith(
      '# Checklist',
      'attachment-preview:memory-system-enhancement-checklist.md'
    )
    expect(wrapper.find('.rendered-markdown-preview').text()).toContain('# Checklist')
  })

  it('keeps non-markdown text attachments in a readable preformatted preview', async () => {
    const wrapper = await mountMessage(makeAttachment('notes.txt', 'text/plain', 'plain text'))

    await wrapper.get('.attachment-preview').trigger('click')
    await flushPromises()

    expect(wrapper.find('.attachment-preview-shell').exists()).toBe(true)
    expect(wrapper.get('.attachment-preview-badge').text()).toBe('TXT')
    expect(wrapper.get('.attachment-preview-title').text()).toBe('notes.txt')
    expect(wrapper.find('.attachment-text-preview').exists()).toBe(true)
    expect(wrapper.find('.attachment-markdown-preview').exists()).toBe(false)
    expect(wrapper.find('.attachment-text-preview').text()).toContain('plain text')
    expect(renderMarkdownCachedMock).not.toHaveBeenCalled()
  })

  it('embeds PDF attachments inside the unified preview shell', async () => {
    const wrapper = await mountMessage(
      makeAttachment('report.pdf', 'application/pdf', '%PDF-1.7 sample')
    )

    await wrapper.get('.attachment-preview').trigger('click')
    await flushPromises()

    const pdfFrame = wrapper.get('.attachment-pdf-frame')
    expect(wrapper.get('.attachment-preview-badge').text()).toBe('PDF')
    expect(wrapper.get('.attachment-preview-title').text()).toBe('report.pdf')
    expect(wrapper.get('.attachment-preview-kind').text()).toBe('PDF preview')
    expect(pdfFrame.attributes('src')).toContain('data:application/pdf;base64,')
    expect(renderMarkdownCachedMock).not.toHaveBeenCalled()
  })

  it('shows unsupported binary files in the unified placeholder shell', async () => {
    const wrapper = await mountMessage(
      makeAttachment('archive.zip', 'application/zip', 'binary-content')
    )

    await wrapper.get('.attachment-preview').trigger('click')
    await flushPromises()

    expect(wrapper.get('.attachment-preview-badge').text()).toBe('ZIP')
    expect(wrapper.get('.attachment-preview-title').text()).toBe('archive.zip')
    expect(wrapper.get('.attachment-preview-kind').text()).toBe('File preview')
    expect(wrapper.find('.attachment-file-placeholder').exists()).toBe(true)
    expect(wrapper.find('.attachment-file-placeholder-text').exists()).toBe(true)
    expect(renderMarkdownCachedMock).not.toHaveBeenCalled()
  })
})
