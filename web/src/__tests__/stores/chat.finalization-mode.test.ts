import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useChatStore } from '@/stores/chat'

const mocks = vi.hoisted(() => ({
  sseConnect: vi.fn(),
  sseDisconnect: vi.fn(),
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  providerPoolStore: {
    fetchTrialQuota: vi.fn(),
    enabledProviders: [] as Array<{ id: string; type?: string }>,
    models: [] as Array<{ id: string; provider_id: string; enabled: boolean }>,
    getProviderDisplayName: vi.fn((providerId: string) => providerId),
  },
}))

vi.mock('@/api/chat', () => ({
  conversationApi: {
    create: vi.fn(),
    list: vi.fn(),
    get: vi.fn(),
    delete: vi.fn(),
    search: vi.fn(),
    patchCommandState: vi.fn(),
  },
  messageApi: {
    list: vi.fn().mockResolvedValue({ data: [] }),
    send: vi.fn(),
    delete: vi.fn(),
    cancelStream: vi.fn(),
  },
}))

vi.mock('@/api/client', () => ({
  default: {
    get: (...args: unknown[]) => mocks.apiGet(...args),
    post: (...args: unknown[]) => mocks.apiPost(...args),
  },
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    selectedProvider: 'openai',
    selectedModel: 'gpt-4o-mini',
    temperature: 0.7,
    maxTokens: 8192,
  }),
}))

vi.mock('@/stores/providerPool', () => ({
  useProviderPoolStore: () => mocks.providerPoolStore,
}))

vi.mock('@/utils/sse', () => ({
  SSEClient: class {
    connect(...args: unknown[]) {
      return mocks.sseConnect(...args)
    }

    disconnect(...args: unknown[]) {
      return mocks.sseDisconnect(...args)
    }
  },
}))

function makeTypelessBlock(payload: Record<string, unknown>) {
  return ['```typeless', JSON.stringify(payload), '```'].join('\n')
}

async function settleAsyncWork() {
  await Promise.resolve()
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
}

describe('Chat Store finalization mode', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mocks.sseConnect.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.getProviderDisplayName.mockImplementation(
      (providerId: string) => providerId
    )
    mocks.apiGet.mockReset().mockResolvedValue({
      data: {
        command_state: {
          conversation_id: 'conv-1',
          selected_provider_id: '',
          selected_model_id: '',
          offline: false,
        },
        active_stream: { conversation_id: 'conv-1', active: false },
        current_tasks: [],
        background_tasks: [],
        pending_approval: null,
        pending_question: null,
        pending_exec_approval: null,
      },
    })
    mocks.apiPost.mockReset().mockResolvedValue({ data: {} })
  })

  it('honors explicit replace finalization mode without preserving prior process cards', async () => {
    const store = useChatStore()
    store.currentConversationId = 'conv-1'
    store.conversations = [
      {
        id: 'conv-1',
        title: 'Browser progress streaming',
        created_at: '2026-03-18T00:00:00.000Z',
        updated_at: '2026-03-18T00:00:00.000Z',
      },
    ]

    const browserProgressBlock = makeTypelessBlock({
      type: 'browser-progress',
      id: 'browser-progress-chain',
      steps: [{ step: 'navigate', name: 'Navigating', status: 'completed' }],
    })
    const webFetchBlock = makeTypelessBlock({
      type: 'web-fetch',
      id: 'web-fetch-chain',
      title: 'web_fetch',
      status: 'success',
      content: 'Expanded page content from the OpenAI blog.',
    })

    mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
      options.onMessage?.({ delta: browserProgressBlock, done: false })
      options.onComplete?.({
        done: true,
        message_id: 'msg-assistant-browser-final',
        content: webFetchBlock,
        finalization_mode: 'replace',
      })
    })

    await store.sendMessage('Check the latest OpenAI updates')
    await settleAsyncWork()

    expect(store.messages).toHaveLength(2)
    expect(store.messages[1]?.content).toBe(webFetchBlock)
    expect(store.messages[1]?.content).not.toContain(browserProgressBlock)
  })
})
