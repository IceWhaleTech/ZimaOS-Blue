import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { parseToolResults, useChatStore } from '@/stores/chat'
import { i18n } from '@/i18n'
import { conversationApi, messageApi } from '@/api/chat'
import { approvalApi } from '@/api/approval'

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
    getCommandState: vi.fn(),
    patchCommandState: vi.fn(),
  },
  messageApi: {
    list: vi.fn(),
    send: vi.fn(),
    delete: vi.fn(),
    cancelStream: vi.fn(),
  },
}))

vi.mock('@/api/approval', () => ({
  approvalApi: {
    getConfig: vi.fn(),
    updateConfig: vi.fn(),
    listPending: vi.fn(),
    resolve: vi.fn(),
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

async function flushMicrotasks() {
  await Promise.resolve()
  await Promise.resolve()
}

describe('Chat Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    i18n.global.locale.value = 'en-US'
    mocks.sseConnect.mockReset().mockResolvedValue(undefined)
    mocks.sseDisconnect.mockReset()
    mocks.apiGet.mockReset().mockImplementation(async (path: string) => {
      if (path.includes('/ask-user-question/pending')) {
        return { data: { pending: false } } as never
      }
      if (path.includes('/exec/approvals/pending')) {
        return { data: { pending: false } } as never
      }
      return { data: {} } as never
    })
    mocks.apiPost.mockReset().mockResolvedValue({ data: {} } as never)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.models = []
    mocks.providerPoolStore.getProviderDisplayName.mockImplementation(
      (providerId: string) => providerId
    )
    vi.mocked(conversationApi.list).mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.delete).mockResolvedValue({ data: { success: true, deleted: 0 } } as never)
    vi.mocked(approvalApi.listPending).mockResolvedValue({ data: [] } as never)
    vi.mocked(conversationApi.getCommandState).mockResolvedValue({
      data: {
        conversation_id: '1',
        selected_provider_id: '',
        selected_model_id: '',
        offline: false,
        web_search_enabled: true,
        deep_research_enabled: false,
      },
    } as never)
    vi.mocked(conversationApi.patchCommandState).mockResolvedValue({
      data: {
        conversation_id: '1',
        selected_provider_id: '',
        selected_model_id: '',
        offline: false,
        web_search_enabled: true,
        deep_research_enabled: false,
      },
    } as never)
  })

  describe('fetchConversations', () => {
    it('should fetch and store conversations', async () => {
      const mockConversations = [
        { id: '1', title: 'Test 1', created_at: '2024-01-01', updated_at: '2024-01-01' },
        { id: '2', title: 'Test 2', created_at: '2024-01-02', updated_at: '2024-01-02' },
      ]
      vi.mocked(conversationApi.list).mockResolvedValue({ data: mockConversations } as never)

      const store = useChatStore()
      await store.fetchConversations()

      expect(conversationApi.list).toHaveBeenCalled()
      expect(store.conversations).toEqual(mockConversations)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should handle fetch error', async () => {
      vi.mocked(conversationApi.list).mockRejectedValue(new Error('Network error'))

      const store = useChatStore()
      await store.fetchConversations()

      expect(store.error).toBe('Network error')
      expect(store.loading).toBe(false)
    })
  })

  describe('createConversation', () => {
    it('should create a new conversation', async () => {
      const mockConversation = {
        id: 'new-id',
        title: 'New Chat',
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      }
      vi.mocked(conversationApi.create).mockResolvedValue({ data: mockConversation } as never)

      const store = useChatStore()
      const result = await store.createConversation('New Chat')

      expect(conversationApi.create).toHaveBeenCalledWith('New Chat')
      expect(result).toEqual(mockConversation)
      expect(store.conversations).toContainEqual(mockConversation)
      expect(store.currentConversationId).toBe('new-id')
    })

    it('should stop active streaming state when creating a new conversation', async () => {
      const mockConversation = {
        id: 'new-id',
        title: 'New Chat',
        created_at: '2024-01-01',
        updated_at: '2024-01-01',
      }
      vi.mocked(conversationApi.create).mockResolvedValue({ data: mockConversation } as never)

      const store = useChatStore()
      store.sending = true
      store.streaming = true

      await store.createConversation('New Chat')

      expect(store.sending).toBe(false)
      expect(store.streaming).toBe(false)
    })
  })

  describe('deleteConversation', () => {
    it('should delete a conversation', async () => {
      vi.mocked(conversationApi.delete).mockResolvedValue({} as never)

      const store = useChatStore()
      store.conversations = [
        { id: '1', title: 'Test', created_at: '2024-01-01', updated_at: '2024-01-01' },
      ]
      store.currentConversationId = '1'

      await store.deleteConversation('1')

      expect(conversationApi.delete).toHaveBeenCalledWith('1')
      expect(store.conversations).toHaveLength(0)
      expect(store.currentConversationId).toBeNull()
    })
  })

  describe('selectConversation', () => {
    it('should select a conversation and fetch messages', async () => {
      const mockMessages = [
        {
          id: 'm1',
          conversation_id: '1',
          role: 'user',
          content: 'Hello',
          created_at: '2024-01-01',
        },
      ]
      vi.mocked(messageApi.list).mockResolvedValue({ data: mockMessages } as never)

      const store = useChatStore()
      await store.selectConversation('1')

      expect(store.currentConversationId).toBe('1')
      expect(messageApi.list).toHaveBeenCalledWith('1', 50, 0)
      expect(store.messages).toEqual(mockMessages)
    })

    it('should not refetch if same conversation is selected', async () => {
      const store = useChatStore()
      store.currentConversationId = '1'

      await store.selectConversation('1')

      expect(messageApi.list).not.toHaveBeenCalled()
    })

    it('should hydrate command state once and reuse it across conversation switches', async () => {
      const store = useChatStore()

      await store.selectConversation('1')
      await store.selectConversation('2')

      expect(conversationApi.getCommandState).toHaveBeenCalledTimes(1)
      expect(conversationApi.getCommandState).toHaveBeenCalledWith('1')
      expect(store.currentConversationId).toBe('2')
    })

    it('should keep chronological order across multi-page loadMore', async () => {
      const store = useChatStore()

      const makePage = (start: number, end: number) =>
        Array.from({ length: end - start + 1 }, (_, idx) => {
          const value = start + idx
          const n = String(value).padStart(3, '0')
          return {
            id: `msg-${n}`,
            conversation_id: 'conv-1',
            role: 'user',
            content: `m${n}`,
            created_at: `2026-03-08T00:00:${String(value).padStart(2, '0')}.000Z`,
          }
        })

      const latestPage = makePage(71, 120)
      const olderPage1 = makePage(21, 70)
      const olderPage2 = makePage(1, 20)

      vi.mocked(messageApi.list).mockImplementation(
        async (_id: string, _limit = 50, offset = 0) => {
          if (offset === 0) return { data: latestPage } as never
          if (offset === 50) return { data: olderPage1 } as never
          if (offset === 100) return { data: olderPage2 } as never
          return { data: [] } as never
        }
      )

      await store.selectConversation('conv-1')
      expect(store.messages.map((m) => m.content)).toEqual(latestPage.map((m) => m.content))
      expect(store.hasMoreMessages).toBe(true)

      await store.loadMoreMessages()
      await store.loadMoreMessages()

      const expected = [
        ...olderPage2.map((m) => m.content),
        ...olderPage1.map((m) => m.content),
        ...latestPage.map((m) => m.content),
      ]
      const actual = store.messages.map((m) => m.content)

      expect(actual).toEqual(expected)
      expect(new Set(actual).size).toBe(120)
      expect(store.hasMoreMessages).toBe(false)
      expect(messageApi.list).toHaveBeenCalledWith('conv-1', 50, 0)
      expect(messageApi.list).toHaveBeenCalledWith('conv-1', 50, 50)
      expect(messageApi.list).toHaveBeenCalledWith('conv-1', 50, 100)
    })

    it('should ignore pending question and exec approval from another conversation', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'

      store.setPendingQuestion({
        id: 'question-2',
        session_id: 'conv-2',
        questions: [{ id: 'q1', question: 'Need input?', header: 'Question' }],
        expires_at: Date.now() + 60000,
      })
      store.setPendingExecApproval({
        id: 'exec-2',
        session_id: 'conv-2',
        type: 'command',
        command: 'rm -rf /tmp/demo',
        expires_at: Date.now() + 60000,
      })

      expect(store.pendingQuestion).toBeNull()
      expect(store.pendingExecApproval).toBeNull()
      expect(store.awaitingConfirmation).toBe(false)
    })

    it('should normalize exec approval payload without mutating command text', () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      const rawCommand = `blue ask q="我需要访问 /tmp 目录。是否授权我访问该目录？" a='["授权访问 /tmp 目录","拒绝访问"]'`

      store.setPendingExecApproval({
        approval: {
          id: 'exec-1',
          type: 'directory',
          command: rawCommand,
          directory: '/tmp',
          expires_at: Math.floor((Date.now() + 60_000) / 1000),
          session_id: 'conv-1',
        },
      })

      expect(store.pendingExecApproval).toEqual(
        expect.objectContaining({
          id: 'exec-1',
          type: 'directory',
          command: rawCommand,
          directory: '/tmp',
          session_id: 'conv-1',
        })
      )
      expect(store.pendingExecApproval?.expires_at).toBeGreaterThan(Date.now())
      expect(store.awaitingConfirmation).toBe(true)
    })

    it('should query tool approvals scoped to the current conversation', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'

      vi.mocked(approvalApi.listPending).mockResolvedValue({
        data: [
          {
            id: 'approval-1',
            tool_name: 'browser',
            tool_call_id: 'tool-1',
            arguments: { url: 'https://example.com' },
            session_id: 'conv-1',
            created_at: '2026-03-11T00:00:00.000Z',
          },
        ],
      } as never)

      await store.checkPendingApprovals()

      expect(approvalApi.listPending).toHaveBeenCalledWith('conv-1')
      expect(store.pendingApproval).toEqual({
        request_id: 'approval-1',
        tool_name: 'browser',
        tool_call_id: 'tool-1',
        arguments: { url: 'https://example.com' },
        session_id: 'conv-1',
        binding_hash: undefined,
      })
    })

    it('clears waiting-for-confirmation state after resolving a tool approval', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.setPendingApproval({
        id: 'approval-1',
        tool_name: 'browser',
        tool_call_id: 'tool-1',
        arguments: { url: 'https://example.com' },
        session_id: 'conv-1',
      })

      vi.mocked(approvalApi.resolve).mockResolvedValue({ data: { status: 'approve' } } as never)

      const resolved = await store.resolveApproval('approve')

      expect(resolved).toBe(true)
      expect(approvalApi.resolve).toHaveBeenCalledWith('approval-1', 'approve', undefined)
      expect(store.pendingApproval).toBeNull()
      expect(store.awaitingConfirmation).toBe(false)
    })

    it('clears waiting-for-confirmation state after resolving an exec approval', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.setPendingExecApproval({
        id: 'exec-1',
        session_id: 'conv-1',
        type: 'directory',
        directory: '/tmp',
        expires_at: Date.now() + 60_000,
      })

      vi.mocked(approvalApi.resolve).mockResolvedValue({ data: { status: 'allow-once' } } as never)

      const resolved = await store.resolveExecApproval('allow-once')

      expect(resolved).toBe(true)
      expect(approvalApi.resolve).toHaveBeenCalledWith('exec-1', 'allow-once', undefined)
      expect(store.pendingExecApproval).toBeNull()
      expect(store.awaitingConfirmation).toBe(false)
    })

    it('forwards binding hashes when resolving approvals', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.setPendingApproval({
        id: 'approval-1',
        tool_name: 'browser',
        tool_call_id: 'tool-1',
        arguments: { url: 'https://example.com' },
        session_id: 'conv-1',
        binding_hash: 'binding-tool',
      })
      store.setPendingExecApproval({
        id: 'exec-1',
        session_id: 'conv-1',
        type: 'directory',
        directory: '/tmp',
        binding_hash: 'binding-exec',
        expires_at: Date.now() + 60_000,
      })

      vi.mocked(approvalApi.resolve).mockResolvedValue({ data: { status: 'approve' } } as never)

      await store.resolveApproval('approve')
      await store.resolveExecApproval('allow-once')

      expect(approvalApi.resolve).toHaveBeenNthCalledWith(
        1,
        'approval-1',
        'approve',
        'binding-tool'
      )
      expect(approvalApi.resolve).toHaveBeenNthCalledWith(2, 'exec-1', 'allow-once', 'binding-exec')
      expect(store.pendingExecApproval).toBeNull()
      expect(store.awaitingConfirmation).toBe(false)
    })
  })

  describe('model selection normalization', () => {
    it('should rewrite stale provider-model command state to the enabled provider route', async () => {
      mocks.providerPoolStore.enabledProviders = [{ id: 'openrouter', type: 'builtin' }]
      mocks.providerPoolStore.models = [
        {
          id: 'minimax/minimax-m2.7',
          provider_id: 'openrouter',
          enabled: true,
        },
      ]
      vi.mocked(conversationApi.getCommandState).mockResolvedValue({
        data: {
          conversation_id: '1',
          selected_provider_id: 'minimax',
          selected_model_id: 'minimax-m2.7',
          offline: false,
          web_search_enabled: true,
          deep_research_enabled: false,
        },
      } as never)

      const store = useChatStore()
      await store.selectConversation('1')
      await store.sendMessage('hi')

      expect(mocks.sseConnect).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          provider: 'openrouter',
          model: 'minimax/minimax-m2.7',
        }),
        expect.any(Object)
      )
    })
  })

  describe('sortedConversations', () => {
    it('should sort conversations by updated_at descending', () => {
      const store = useChatStore()
      store.conversations = [
        { id: '1', title: 'Old', created_at: '2024-01-01', updated_at: '2024-01-01' },
        { id: '2', title: 'New', created_at: '2024-01-02', updated_at: '2024-01-03' },
        { id: '3', title: 'Mid', created_at: '2024-01-01', updated_at: '2024-01-02' },
      ]

      expect(store.sortedConversations[0].id).toBe('2')
      expect(store.sortedConversations[1].id).toBe('3')
      expect(store.sortedConversations[2].id).toBe('1')
    })

    it('should apply deterministic tie-breakers for same updated_at', () => {
      const store = useChatStore()
      store.conversations = [
        {
          id: 'a',
          title: 'A',
          created_at: '2024-01-01T00:00:00.000Z',
          updated_at: '2024-01-03T00:00:00.000Z',
        },
        {
          id: 'c',
          title: 'C',
          created_at: '2024-01-02T00:00:00.000Z',
          updated_at: '2024-01-03T00:00:00.000Z',
        },
        {
          id: 'b',
          title: 'B',
          created_at: '2024-01-02T00:00:00.000Z',
          updated_at: '2024-01-03T00:00:00.000Z',
        },
      ]

      expect(store.sortedConversations.map((c) => c.id)).toEqual(['c', 'b', 'a'])
    })
  })

  describe('sendMessage streaming', () => {
    it('clears a stale stream error when a new request starts and completes', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Original',
          created_at: '2026-03-22T00:00:00.000Z',
          updated_at: '2026-03-22T00:00:00.000Z',
        },
      ]
      store.streamError = 'provider_auth_error'

      let streamOptions: any
      let resolveStream: (() => void) | null = null

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        streamOptions = options
        await new Promise<void>((resolve) => {
          resolveStream = resolve
        })
      })

      const sendPromise = store.sendMessage('hello')
      await flushMicrotasks()

      expect(store.streamError).toBeNull()

      streamOptions.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
      resolveStream?.()
      await sendPromise

      expect(store.streamError).toBeNull()
    })

    it('clears transient stream errors after recovering persisted assistant content', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Recovered',
          created_at: '2026-03-22T00:00:00.000Z',
          updated_at: '2026-03-22T00:00:00.000Z',
        },
      ]
      store.streamError = 'provider_auth_error'

      vi.mocked(messageApi.list).mockResolvedValue({
        data: [
          {
            id: 'msg-user-1',
            conversation_id: 'conv-1',
            role: 'user',
            content: 'Need recovery',
            created_at: '2026-03-22T00:00:00.000Z',
          },
          {
            id: 'msg-assistant-1',
            conversation_id: 'conv-1',
            role: 'assistant',
            content: 'Recovered answer',
            created_at: '2026-03-22T00:00:01.000Z',
          },
        ],
      } as never)

      let streamOptions: any
      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        streamOptions = options
        options.onError?.(new Error('STREAM_EMPTY'))
      })

      await store.sendMessage('Need recovery')
      await settleAsyncWork()

      expect(store.streamError).toBeNull()
      expect(store.messages.at(-1)?.id).toBe('msg-assistant-1')
      expect(store.messages.at(-1)?.content).toBe('Recovered answer')
    })

    it('falls back to a timer commit when animation frames are throttled', async () => {
      vi.useFakeTimers()

      const rafSpy = vi
        .spyOn(window, 'requestAnimationFrame')
        .mockImplementation(() => 1 as unknown as number)
      const cancelRafSpy = vi
        .spyOn(window, 'cancelAnimationFrame')
        .mockImplementation(() => undefined)
      try {
        const store = useChatStore()
        store.currentConversationId = 'conv-1'
        store.conversations = [
          {
            id: 'conv-1',
            title: 'Original',
            created_at: '2026-03-22T00:00:00.000Z',
            updated_at: '2026-03-22T00:00:00.000Z',
          },
        ]

        let streamOptions: any
        let resolveStream: (() => void) | null = null

        mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
          streamOptions = options
          await new Promise<void>((resolve) => {
            resolveStream = resolve
          })
        })

        const sendPromise = store.sendMessage('hello')
        await Promise.resolve()
        await Promise.resolve()

        expect(store.messages.at(-1)?.id.startsWith('streaming-')).toBe(true)

        streamOptions.onMessage({ delta: 'Hello', done: false })
        await Promise.resolve()
        await Promise.resolve()

        expect(store.messages.at(-1)?.content).toBe('Hello')

        streamOptions.onMessage({ delta: ' world', done: false })
        await Promise.resolve()
        await Promise.resolve()

        expect(store.messages.at(-1)?.content).toBe('Hello')

        vi.advanceTimersByTime(24)
        await Promise.resolve()
        await Promise.resolve()

        expect(store.messages.at(-1)?.content).toBe('Hello world')

        streamOptions.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
        resolveStream?.()
        await sendPromise
      } finally {
        rafSpy.mockRestore()
        cancelRafSpy.mockRestore()
        vi.useRealTimers()
      }
    })

    it('reattaches a detached stream when switching back to the original conversation', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Original',
          created_at: '2026-03-11T00:00:00.000Z',
          updated_at: '2026-03-11T00:00:00.000Z',
        },
        {
          id: 'conv-2',
          title: 'Other',
          created_at: '2026-03-11T00:00:01.000Z',
          updated_at: '2026-03-11T00:00:01.000Z',
        },
      ]

      vi.mocked(messageApi.list).mockImplementation(async (conversationId: string) => {
        if (conversationId === 'conv-1') {
          return {
            data: [
              {
                id: 'msg-user-1',
                conversation_id: 'conv-1',
                role: 'user',
                content: 'Need a decision',
                created_at: '2026-03-11T00:00:00.000Z',
              },
            ],
          } as never
        }
        return { data: [] } as never
      })

      let streamOptions: any
      let resolveStream: (() => void) | null = null

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Need a decision',
            web_search_enabled: true,
            deep_research_enabled: false,
          })
        )
        streamOptions = options
        await new Promise<void>((resolve) => {
          resolveStream = resolve
        })
      })

      const sendPromise = store.sendMessage('Need a decision')
      await settleAsyncWork()

      expect(store.streaming).toBe(true)
      expect(store.sending).toBe(true)
      expect(store.messages.at(-1)?.id.startsWith('streaming-')).toBe(true)

      streamOptions.onToolExecuting?.(1, ['ask'], false, ['ask'])
      streamOptions.onToolResults?.(
        [
          {
            name: 'web_search',
            id: 'tool-search-1',
            args: '{"query":"Need a decision"}',
            result: '{"status":"ok","stdout":"Found 3 results"}',
          },
        ],
        0
      )
      streamOptions.onMessage({ delta: '', done: false, awaiting_user_input: true })

      expect(store.awaitingConfirmation).toBe(true)

      await store.selectConversation('conv-2')

      expect(mocks.sseDisconnect).not.toHaveBeenCalled()
      expect(store.currentConversationId).toBe('conv-2')
      expect(store.streaming).toBe(false)
      expect(store.sending).toBe(false)
      expect(store.awaitingConfirmation).toBe(false)

      streamOptions.onMessage({ delta: 'Still working...', done: false })
      await settleAsyncWork()

      await store.selectConversation('conv-1')

      expect(store.currentConversationId).toBe('conv-1')
      expect(store.streaming).toBe(true)
      expect(store.sending).toBe(true)
      expect(store.awaitingConfirmation).toBe(true)
      expect(store.toolResults).toHaveLength(1)
      expect(store.toolResults[0]?.id).toBe('tool-search-1')
      expect(store.statusSummary).toBe('Writing response...')
      expect(store.statusStartedAt).toBeGreaterThan(0)
      expect(store.messages.at(-1)?.role).toBe('assistant')
      expect(store.messages.at(-1)?.content).toBe('Still working...')

      streamOptions.onMessage({ delta: ' More context', done: false })
      await settleAsyncWork()

      expect(store.messages.at(-1)?.content).toBe('Still working... More context')

      streamOptions.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
      resolveStream?.()
      await sendPromise

      expect(store.streaming).toBe(false)
      expect(store.sending).toBe(false)
    })

    it('records process traces from local and server events and restores them after conversation switches', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Original',
          created_at: '2026-03-11T00:00:00.000Z',
          updated_at: '2026-03-11T00:00:00.000Z',
        },
        {
          id: 'conv-2',
          title: 'Other',
          created_at: '2026-03-11T00:00:01.000Z',
          updated_at: '2026-03-11T00:00:01.000Z',
        },
      ]

      vi.mocked(messageApi.list).mockImplementation(async (conversationId: string) => {
        if (conversationId === 'conv-1') {
          return {
            data: [
              {
                id: 'msg-user-1',
                conversation_id: 'conv-1',
                role: 'user',
                content: 'Need a decision',
                created_at: '2026-03-11T00:00:00.000Z',
              },
            ],
          } as never
        }
        return { data: [] } as never
      })

      let streamOptions: any
      let resolveStream: (() => void) | null = null

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        streamOptions = options
        await new Promise<void>((resolve) => {
          resolveStream = resolve
        })
      })

      const sendPromise = store.sendMessage('Need a decision')
      await settleAsyncWork()

      expect(store.processTrace.map((item) => item.event)).toEqual([
        'request_summary',
        'request_dispatched',
        'waiting_for_response',
      ])

      streamOptions.onProcessEvent?.({
        delta: '',
        done: false,
        process_event: 'pre_content_retry_started',
        process_status: 'active',
        process_message: 'Retrying request',
        process_attempt: 1,
      })
      streamOptions.onMessage?.({ delta: '', done: false, awaiting_user_input: true })
      await settleAsyncWork()

      expect(store.processTrace.some((item) => item.event === 'pre_content_retry_started')).toBe(
        true
      )
      expect(store.processTrace.some((item) => item.event === 'awaiting_confirmation')).toBe(true)
      expect(store.statusSummary).toBe('Retrying request')

      await store.selectConversation('conv-2')
      expect(store.processTrace).toHaveLength(0)

      await store.selectConversation('conv-1')
      expect(store.processTrace.some((item) => item.event === 'request_summary')).toBe(true)
      expect(store.processTrace.some((item) => item.event === 'pre_content_retry_started')).toBe(
        true
      )
      expect(store.processTrace.some((item) => item.event === 'awaiting_confirmation')).toBe(true)

      streamOptions.onComplete?.({ done: true })
      resolveStream?.()
      await sendPromise
    })

    it('shows recovering state after a network interrupt and completes after syncing persisted content', async () => {
      vi.useFakeTimers()

      try {
        const store = useChatStore()
        store.currentConversationId = 'conv-1'
        store.conversations = [
          {
            id: 'conv-1',
            title: 'Original',
            created_at: '2026-03-11T00:00:00.000Z',
            updated_at: '2026-03-11T00:00:00.000Z',
          },
        ]

        vi.mocked(messageApi.list).mockResolvedValue({
          data: [
            {
              id: 'msg-user-1',
              conversation_id: 'conv-1',
              role: 'user',
              content: 'Need recovery',
              created_at: '2026-03-11T00:00:00.000Z',
            },
            {
              id: 'msg-assistant-1',
              conversation_id: 'conv-1',
              role: 'assistant',
              content: 'Recovered answer',
              created_at: '2026-03-11T00:00:01.000Z',
            },
          ],
        } as never)

        let streamOptions: any
        let resolveStream: (() => void) | null = null

        mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
          streamOptions = options
          await new Promise<void>((resolve) => {
            resolveStream = resolve
          })
        })

        const sendPromise = store.sendMessage('Need recovery')
        await flushMicrotasks()

        streamOptions.onMessage({ delta: 'Partial answer', done: false })
        await flushMicrotasks()
        expect(store.messages.at(-1)?.content).toBe('Partial answer')

        streamOptions.onNetworkInterrupt?.()
        for (let i = 0; i < 12 && store.streamUIState.phase === 'recovering'; i++) {
          await flushMicrotasks()
        }

        expect(store.streamUIState.phase).toBe('completed')
        expect(store.streamUIState.label).toBeTruthy()
        expect(store.messages).toHaveLength(2)
        expect(store.messages.at(-1)?.id).toBe('msg-assistant-1')
        expect(store.messages.at(-1)?.content).toBe('Recovered answer')

        await vi.advanceTimersByTimeAsync(600)
        expect(store.streamUIState.phase).toBe('idle')

        resolveStream?.()
        await sendPromise
      } finally {
        vi.useRealTimers()
      }
    })

    it('falls back to interrupted state when recovery makes no progress', async () => {
      vi.useFakeTimers()

      try {
        const store = useChatStore()
        store.currentConversationId = 'conv-1'
        store.conversations = [
          {
            id: 'conv-1',
            title: 'Original',
            created_at: '2026-03-11T00:00:00.000Z',
            updated_at: '2026-03-11T00:00:00.000Z',
          },
        ]

        vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)

        let streamOptions: any
        let resolveStream: (() => void) | null = null

        mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
          streamOptions = options
          await new Promise<void>((resolve) => {
            resolveStream = resolve
          })
        })

        const sendPromise = store.sendMessage('Need recovery')
        await flushMicrotasks()

        streamOptions.onNetworkInterrupt?.()
        await flushMicrotasks()

        expect(store.streamUIState.phase).toBe('recovering')

        await vi.advanceTimersByTimeAsync(2700)
        await flushMicrotasks()

        expect(store.streamUIState.phase).toBe('interrupted')
        expect(store.streamUIState.canRetry).toBe(true)
        expect(store.isStreamInterrupted).toBe(true)

        resolveStream?.()
        await sendPromise
      } finally {
        vi.useRealTimers()
      }
    })

    it('localizes process trace labels from known events', async () => {
      i18n.global.setLocaleMessage('zh-CN', {
        chat: {
          processTrace: {
            events: {
              requestReady: '请求已准备就绪',
              requestSent: '请求已发送',
              waitingForResponse: '正在等待响应',
              retryingRequest: '正在重试请求',
            },
            details: {
              requestDispatched: '正在等待服务器接受请求并开始响应。',
              waitingForResponse: '请求已被接受。正在等待第一段可见输出。',
              providerFailoverToolFollowUp: '正在不使用上一个固定提供商重试这轮工具后续请求。',
              recoveryStage1: '静默恢复',
            },
            fields: {
              message: '消息',
              provider: '提供商',
              model: '模型',
              webSearch: '网页搜索',
              deepResearch: '深度研究',
              on: '开启',
              off: '关闭',
              auto: '自动',
            },
            summaryValues: {
              continuePreviousReply: '继续上一条回复',
            },
          },
        },
      } as never)
      i18n.global.locale.value = 'zh-CN'

      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Localized progress',
          created_at: '2026-03-11T00:00:00.000Z',
          updated_at: '2026-03-11T00:00:00.000Z',
        },
      ]

      vi.mocked(messageApi.list).mockResolvedValue({
        data: [
          {
            id: 'msg-user-1',
            conversation_id: 'conv-1',
            role: 'user',
            content: 'Need a decision',
            created_at: '2026-03-11T00:00:00.000Z',
          },
        ],
      } as never)

      let streamOptions: any
      let resolveStream: (() => void) | null = null

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        streamOptions = options
        await new Promise<void>((resolve) => {
          resolveStream = resolve
        })
      })

      const sendPromise = store.sendMessage('[CONTINUE]')
      await settleAsyncWork()

      expect(store.processTrace.map((item) => item.label)).toEqual([
        '请求已准备就绪',
        '请求已发送',
        '正在等待响应',
      ])
      expect(store.processTrace[0]?.detail).toContain('消息: 继续上一条回复')
      expect(store.processTrace[0]?.detail).toContain('提供商: 自动')
      expect(store.processTrace[0]?.detail).toContain('模型: 自动')
      expect(store.processTrace[0]?.detail).toContain('网页搜索: 开启')
      expect(store.processTrace[1]?.detail).toBe('正在等待服务器接受请求并开始响应。')
      expect(store.processTrace[2]?.detail).toBe('请求已被接受。正在等待第一段可见输出。')

      streamOptions.onProcessEvent?.({
        delta: '',
        done: false,
        process_event: 'pre_content_retry_started',
        process_status: 'active',
        process_message: 'Retrying request',
        process_attempt: 1,
      })
      await settleAsyncWork()

      expect(
        store.processTrace.find((item) => item.event === 'pre_content_retry_started')?.label
      ).toBe('正在重试请求')
      expect(store.statusSummary).toBe('正在重试请求')

      streamOptions.onProcessEvent?.({
        delta: '',
        done: false,
        process_event: 'provider_failover',
        process_status: 'active',
        process_message: 'Switching provider',
        process_detail: 'Retrying the tool follow-up without the previously pinned provider.',
      })
      streamOptions.onProcessEvent?.({
        delta: '',
        done: false,
        process_event: 'continuation_recovery_started',
        process_status: 'active',
        process_message: 'Recovering response',
        process_detail: 'silent_recovery_stage1',
      })
      await settleAsyncWork()

      expect(
        store.processTrace.find((item) => item.event === 'provider_failover')?.detail
      ).toContain('正在不使用上一个固定提供商重试这轮工具后续请求。')
      expect(
        store.processTrace.find((item) => item.event === 'continuation_recovery_started')?.detail
      ).toContain('静默恢复')

      streamOptions.onComplete?.({ delta: '', done: true })
      resolveStream?.()
      await sendPromise
      i18n.global.locale.value = 'en-US'
    })

    it('should split streamed Reddit follow-up cards into separate assistant messages before persistence refresh', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Reddit flow',
          created_at: '2026-03-08T00:00:00.000Z',
          updated_at: '2026-03-08T00:00:00.000Z',
        },
      ]

      const webFetchBlock = makeTypelessBlock({
        type: 'web-fetch',
        id: 'web-fetch-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest',
        title: 'Sign in',
        status: 'warning',
        url: 'https://www.reddit.com/r/test',
        content: 'Log in to continue',
        content_type: 'text/html',
        extract_mode: 'text',
        extractor: 'html',
        warning: 'page appears to be a login wall; use browser or pass browser_target_id',
        warning_code: 'login_wall',
        actions: [
          {
            id: 'use_browser',
            label: 'Use browser',
            variant: 'primary',
            form_data: { url: 'https://www.reddit.com/r/test' },
          },
        ],
      })
      const browserBlock = makeTypelessBlock({
        type: 'result',
        id: 'browser-https%3A%2F%2Fwww.reddit.com%2Fr%2Ftest',
        title: 'Browser page',
        status: 'info',
        message: 'Interactive page opened in the browser session.',
        details: [
          { label: 'url', value: 'https://www.reddit.com/r/test' },
          { label: 'browser_target_id', value: 'tab-42' },
        ],
        actions: [
          {
            id: 'extract_with_web_fetch',
            label: 'Extract readable content',
            variant: 'primary',
            form_data: {
              url: 'https://www.reddit.com/r/test',
              browser_target_id: 'tab-42',
            },
          },
        ],
      })

      const persistedMessages = [
        {
          id: 'msg-user-1',
          conversation_id: 'conv-1',
          role: 'user',
          content: 'Inspect https://www.reddit.com/r/test',
          created_at: '2026-03-08T00:00:00.000Z',
        },
        {
          id: 'msg-assistant-1',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: webFetchBlock,
          created_at: '2026-03-08T00:00:01.000Z',
        },
        {
          id: 'msg-assistant-2',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: browserBlock,
          created_at: '2026-03-08T00:00:02.000Z',
        },
      ]

      vi.mocked(messageApi.list).mockResolvedValue({ data: persistedMessages } as never)
      vi.mocked(conversationApi.list).mockResolvedValue({
        data: [
          {
            id: 'conv-1',
            title: 'Reddit flow',
            created_at: '2026-03-08T00:00:00.000Z',
            updated_at: '2026-03-08T00:00:02.000Z',
          },
        ],
      } as never)

      const snapshotAfterSplit: Array<{ id: string; role: string; content: string }> = []
      const snapshotBeforeRefresh: Array<{ id: string; role: string; content: string }> = []

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Inspect https://www.reddit.com/r/test',
            web_search_enabled: true,
            deep_research_enabled: false,
          })
        )

        options.onMessage({ delta: webFetchBlock, done: false })
        options.onNewMessage?.(1)
        snapshotAfterSplit.push(
          ...store.messages.map((message) => ({
            id: message.id,
            role: message.role,
            content: message.content,
            todo_card_id: (message as any).todo_card_id,
          }))
        )

        options.onMessage({ delta: browserBlock, done: false })
        options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
        snapshotBeforeRefresh.push(
          ...store.messages.map((message) => ({
            id: message.id,
            role: message.role,
            content: message.content,
          }))
        )
      })

      await store.sendMessage('Inspect https://www.reddit.com/r/test')
      await settleAsyncWork()

      expect(mocks.sseConnect).toHaveBeenCalledTimes(1)
      expect(snapshotAfterSplit).toHaveLength(3)
      expect(snapshotAfterSplit[0]?.role).toBe('user')
      expect(snapshotAfterSplit[1]?.content).toBe(webFetchBlock)
      expect(snapshotAfterSplit[2]?.content).toBe('')

      expect(snapshotBeforeRefresh).toHaveLength(3)
      expect(snapshotBeforeRefresh[1]?.content).toBe(webFetchBlock)
      expect(snapshotBeforeRefresh[2]?.content).toBe(browserBlock)
      expect(snapshotBeforeRefresh[2]?.id.startsWith('streaming-')).toBe(true)

      expect(messageApi.list).toHaveBeenCalledWith('conv-1', 50, 0)
      expect(conversationApi.list).toHaveBeenCalledTimes(1)
      expect(mocks.providerPoolStore.fetchTrialQuota).toHaveBeenCalledTimes(1)
      expect(store.messages).toEqual(persistedMessages)
      expect(
        store.messages.some(
          (message) => message.id.startsWith('temp-') || message.id.startsWith('streaming-')
        )
      ).toBe(false)
      expect(store.streaming).toBe(false)
      expect(store.sending).toBe(false)
    })

    it('keeps tool result details attached to the completed split bubble until persistence refresh catches up', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Tool split flow',
          created_at: '2026-03-18T00:00:00.000Z',
          updated_at: '2026-03-18T00:00:00.000Z',
        },
      ]

      vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)

      let splitSnapshot: Array<{
        id: string
        role: string
        content: string
        localToolResultCount: number
      }> = []

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        options.onToolResults?.(
          [
            {
              name: 'web_search',
              id: 'tool-search-1',
              args: '{"query":"Need sources"}',
              result: '{"status":"ok","stdout":"Found 3 results"}',
            },
          ],
          1
        )
        options.onNewMessage?.(1)
        splitSnapshot = store.messages.map((message) => ({
          id: message.id,
          role: message.role,
          content: message.content,
          localToolResultCount: ((message as any).local_process_tool_results || []).length,
        }))
        options.onComplete?.({ done: true })
      })

      await store.sendMessage('Need sources')
      await settleAsyncWork()

      expect(splitSnapshot).toHaveLength(3)
      expect(splitSnapshot[0]?.role).toBe('user')
      expect(splitSnapshot[1]?.id.startsWith('streaming-')).toBe(true)
      expect(splitSnapshot[1]?.localToolResultCount).toBe(1)
      expect(splitSnapshot[2]?.localToolResultCount).toBe(0)
      expect(splitSnapshot[2]?.content).toBe('')
    })

    it('updates the most recent checklist bubble when todo_updated cannot match a local message id', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Checklist fallback',
          created_at: '2026-03-10T00:00:00.000Z',
          updated_at: '2026-03-10T00:00:00.000Z',
        },
      ]
      store.messages = [
        {
          id: 'msg-user-legacy',
          conversation_id: 'conv-1',
          role: 'user',
          content: 'Earlier task',
          created_at: '2026-03-10T00:00:00.000Z',
        },
        {
          id: 'msg-assistant-legacy',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: '- [ ] legacy task\n- [ ] legacy verify',
          created_at: '2026-03-10T00:00:01.000Z',
        },
      ]

      let snapshotAfterTodoUpdate: Array<{
        id: string
        role: string
        content: string
        todo_card_id?: string
      }> = []

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Continue the current work',
            web_search_enabled: true,
            deep_research_enabled: false,
          })
        )

        options.onMessage({ delta: '- [ ] collect facts\n- [ ] write summary', done: false })
        options.onNewMessage?.(1)
        options.onTodoUpdated?.(
          'msg-assistant-current',
          '- [x] collect facts\n- [ ] write summary',
          'todo-checklist-msg-assistant-current'
        )

        snapshotAfterTodoUpdate = store.messages.map((message) => ({
          id: message.id,
          role: message.role,
          content: message.content,
          todo_card_id: message.todo_card_id,
        }))
      })

      await store.sendMessage('Continue the current work')

      expect(mocks.sseConnect).toHaveBeenCalledTimes(1)
      expect(snapshotAfterTodoUpdate).toHaveLength(5)
      expect(snapshotAfterTodoUpdate[1]?.id).toBe('msg-assistant-legacy')
      expect(snapshotAfterTodoUpdate[1]?.content).toBe('- [ ] legacy task\n- [ ] legacy verify')

      const updatedChecklist = snapshotAfterTodoUpdate.find(
        (message) => message.content === '- [x] collect facts\n- [ ] write summary'
      )
      expect(updatedChecklist).toBeTruthy()
      expect(updatedChecklist?.id.startsWith('streaming-')).toBe(true)
      expect(updatedChecklist?.todo_card_id).toBe('todo-checklist-msg-assistant-current')

      const staleLegacyMatches = snapshotAfterTodoUpdate.filter((message) =>
        message.content.includes('legacy task')
      )
      expect(staleLegacyMatches).toHaveLength(1)
      expect(snapshotAfterTodoUpdate.at(-1)?.content).toBe('')
    })

    it('records explicit todo completion when todo_updated carries todo_completed', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Checklist completion',
          created_at: '2026-03-10T00:00:00.000Z',
          updated_at: '2026-03-10T00:00:00.000Z',
        },
      ]
      store.messages = [
        {
          id: 'msg-assistant-current',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: '- [ ] collect facts\n- [ ] write summary',
          todo_card_id: 'todo-checklist-msg-assistant-current',
          created_at: '2026-03-10T00:00:01.000Z',
        },
      ]

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        options.onTodoUpdated?.(
          'msg-assistant-current',
          '- [x] collect facts\n- [x] write summary',
          'todo-checklist-msg-assistant-current',
          true
        )
      })

      await store.sendMessage('Continue the current work')

      expect(store.recentTodoCompletion).toEqual({
        messageId: 'msg-assistant-current',
        todoCardId: 'todo-checklist-msg-assistant-current',
      })
      expect(
        store.messages.find((message) => message.id === 'msg-assistant-current')?.content
      ).toBe('- [x] collect facts\n- [x] write summary')
    })

    it('truncates later turns and resubmits when editing a user message', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.messages = [
        {
          id: 'msg-user-0',
          conversation_id: 'conv-1',
          role: 'user',
          content: 'Keep this context',
          created_at: '2026-03-12T00:00:00.000Z',
        },
        {
          id: 'msg-assistant-0',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: 'Earlier answer',
          created_at: '2026-03-12T00:00:01.000Z',
        },
        {
          id: 'msg-user-1',
          conversation_id: 'conv-1',
          role: 'user',
          content: 'Old prompt',
          created_at: '2026-03-12T00:00:02.000Z',
          attachments: [
            {
              type: 'image',
              name: 'diagram.png',
              mime_type: 'image/png',
              data: 'ZmFrZS1pbWFnZQ==',
            },
          ],
        },
        {
          id: 'msg-assistant-1',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: 'Old answer',
          created_at: '2026-03-12T00:00:03.000Z',
        },
        {
          id: 'msg-user-2',
          conversation_id: 'conv-1',
          role: 'user',
          content: 'Later follow-up',
          created_at: '2026-03-12T00:00:04.000Z',
        },
        {
          id: 'msg-assistant-2',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: 'Later answer',
          created_at: '2026-03-12T00:00:05.000Z',
        },
      ]
      store.enterMultiSelectMode('msg-user-2')

      await store.editMessageAndResubmit('msg-user-1', 'Updated prompt')

      expect(messageApi.delete).toHaveBeenCalledWith('conv-1', [
        'msg-user-1',
        'msg-assistant-1',
        'msg-user-2',
        'msg-assistant-2',
      ])
      expect(mocks.sseConnect).toHaveBeenCalledWith(
        'conv-1',
        expect.objectContaining({
          message: 'Updated prompt',
          attachments: [
            {
              type: 'image',
              name: 'diagram.png',
              mime_type: 'image/png',
              data: 'ZmFrZS1pbWFnZQ==',
            },
          ],
          web_search_enabled: true,
          deep_research_enabled: false,
        }),
        expect.any(Object)
      )

      expect(store.messages).toHaveLength(4)
      expect(store.messages[0]?.id).toBe('msg-user-0')
      expect(store.messages[1]?.id).toBe('msg-assistant-0')
      expect(store.messages[2]?.role).toBe('user')
      expect(store.messages[2]?.id.startsWith('temp-')).toBe(true)
      expect(store.messages[2]?.content).toBe('Updated prompt')
      expect(store.messages[2]?.attachments).toEqual([
        {
          type: 'image',
          name: 'diagram.png',
          mime_type: 'image/png',
          data: 'ZmFrZS1pbWFnZQ==',
        },
      ])
      expect(store.messages[3]?.role).toBe('assistant')
      expect(store.messages[3]?.id.startsWith('streaming-')).toBe(true)
      expect(store.selectedMessageIds.size).toBe(0)
      expect(store.isMultiSelectMode).toBe(false)
    })
  })

  describe('regenerateMessage', () => {
    it('removes the full assistant tail before starting a regenerate stream', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.messages = [
        {
          id: 'msg-user-1',
          conversation_id: 'conv-1',
          role: 'user',
          content: 'Deep research this topic',
          created_at: '2026-03-23T00:00:00.000Z',
        },
        {
          id: 'msg-assistant-1',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: makeTypelessBlock({
            type: 'deep-research-progress',
            job_id: 'job-1',
            query: 'Deep research this topic',
          }),
          created_at: '2026-03-23T00:00:01.000Z',
        },
        {
          id: 'msg-assistant-2',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: makeTypelessBlock({
            type: 'deep-research',
            query: 'Deep research this topic',
          }),
          created_at: '2026-03-23T00:00:02.000Z',
        },
      ]

      let snapshotDuringRegenerate: string[] = []

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Deep research this topic',
            regenerate: true,
            web_search_enabled: true,
            deep_research_enabled: false,
          })
        )
        snapshotDuringRegenerate = store.messages.map((message) => message.id)
        options.onMessage?.({ delta: 'Regenerated answer', done: false })
        options.onComplete?.({
          done: true,
          message_id: 'msg-assistant-regenerated',
          content: 'Regenerated answer',
        })
      })

      await store.regenerateMessage()
      await settleAsyncWork()

      expect(snapshotDuringRegenerate).toHaveLength(2)
      expect(snapshotDuringRegenerate[0]).toBe('msg-user-1')
      expect(snapshotDuringRegenerate[1]?.startsWith('streaming-')).toBe(true)
      expect(store.messages).toHaveLength(2)
      expect(store.messages[0]?.id).toBe('msg-user-1')
      expect(store.messages[1]?.id).toBe('msg-assistant-regenerated')
    })
  })

  describe('parseToolResults', () => {
    it('extracts exec commands from cmd arguments', () => {
      const items = parseToolResults([
        {
          name: 'exec',
          id: 'exec-cmd',
          args: JSON.stringify({
            cmd: 'mkdir -p /Users/orca/.zimaos-blue/data/workspace/tank-battle',
          }),
          result: JSON.stringify({ exit_code: 0, duration_ms: 25 }),
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.command).toBe('mkdir -p /Users/orca/.zimaos-blue/data/workspace/tank-battle')
      expect(items[0]?.status).toBe('25ms')
      expect(items[0]?.icon).toBe('✓')
    })

    it('formats convert tool commands from source refs and target format', () => {
      const items = parseToolResults([
        {
          name: 'convert',
          id: 'convert-1',
          args: JSON.stringify({
            action: 'convert',
            sources: ['/Users/orca/.zimaos-blue/data/workspace/phone_specs_2026/完整汇总表格.md'],
            target_format: 'pdf',
          }),
          result: JSON.stringify({
            task_id: 'task-1',
            status: 'succeeded',
            source_summary: '完整汇总表格.md',
            target_format: 'pdf',
          }),
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.command).toBe(
        '/Users/orca/.zimaos-blue/data/workspace/phone_specs_2026/完整汇总表格.md -> pdf'
      )
      expect(items[0]?.status).toBe('succeeded')
      expect(items[0]?.icon).toBe('✓')
    })

    it('formats convert tool commands from input_path and output_path', () => {
      const items = parseToolResults([
        {
          name: 'convert',
          id: 'convert-2',
          args: JSON.stringify({
            input_path: 'docs/phone_specs.md',
            output_path: 'exports/phone_specs.pdf',
          }),
          result: JSON.stringify({
            async: false,
            task_id: 'task-2',
            status: 'succeeded',
            output_path: '/Users/orca/.zimaos-blue/data/workspace/exports/phone_specs.pdf',
          }),
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.command).toBe('docs/phone_specs.md -> exports/phone_specs.pdf')
      expect(items[0]?.status).toBe('succeeded')
      expect(items[0]?.icon).toBe('✓')
    })

    it('should map challenge and browser_required warning codes to friendly statuses', () => {
      const items = parseToolResults([
        {
          name: 'web_fetch',
          id: 'wf-challenge',
          args: JSON.stringify({ url: 'https://example.com/challenge' }),
          result: JSON.stringify({
            url: 'https://example.com/challenge',
            warning: 'verification challenge detected',
            warning_code: 'challenge',
          }),
        },
        {
          name: 'web_fetch',
          id: 'wf-browser-required',
          args: JSON.stringify({ url: 'https://example.com/protected' }),
          result: JSON.stringify({
            url: 'https://example.com/protected',
            warning: 'browser session required',
            warning_code: 'browser_required',
          }),
        },
      ])

      expect(items).toHaveLength(2)
      expect(items[0]?.warningCode).toBe('challenge')
      expect(items[0]?.status).toBe('Verification challenge detected')
      expect(items[1]?.warningCode).toBe('browser_required')
      expect(items[1]?.status).toBe('Browser session required')
    })

    it('localizes warning statuses from warning_code', () => {
      i18n.global.setLocaleMessage('zh-CN', {
        toolWarnings: {
          statuses: {
            loginWall: '检测到登录墙',
            challenge: '检测到验证挑战',
            browserRequired: '需要浏览器会话',
            unknown: '警告：{code}',
          },
        },
      })
      i18n.global.locale.value = 'zh-CN'

      const items = parseToolResults([
        {
          name: 'web_fetch',
          id: 'wf-localized',
          args: JSON.stringify({ url: 'https://example.com' }),
          result: JSON.stringify({ warning_code: 'login_wall' }),
        },
      ])

      expect(items[0]?.status).toBe('检测到登录墙')

      i18n.global.locale.value = 'en-US'
    })
    it('should surface warning_code for web_fetch results', () => {
      const items = parseToolResults([
        {
          name: 'web_fetch',
          id: 'wf-1',
          args: JSON.stringify({ url: 'https://www.reddit.com/r/test' }),
          result: JSON.stringify({
            url: 'https://www.reddit.com/r/test',
            warning: 'page appears to be a login wall; use browser or pass browser_target_id',
            warning_code: 'login_wall',
            extractor: 'html',
          }),
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.command).toBe('https://www.reddit.com/r/test')
      expect(items[0]?.warningCode).toBe('login_wall')
      expect(items[0]?.status).toBe('Login wall detected')
      expect(items[0]?.output).toContain('browser_target_id')
      expect(items[0]?.icon).toBe('✓')
    })

    it('summarizes screenshot tool results without exposing base64 output', () => {
      const items = parseToolResults([
        {
          name: 'browser',
          id: 'browser-shot',
          args: JSON.stringify({ action: 'screenshot', url: 'https://example.com' }),
          result: JSON.stringify({
            message: 'Screenshot captured for https://example.com',
            screenshot:
              'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII=',
          }),
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.status).toBe('Screenshot captured for https://example.com')
      expect(items[0]?.output).toBe('')
      expect(items[0]?.icon).toBe('✓')
    })

    it('handles truncated screenshot payloads without showing base64 blobs', () => {
      const items = parseToolResults([
        {
          name: 'browser',
          id: 'browser-shot-truncated',
          args: JSON.stringify({ action: 'screenshot', url: 'https://example.com' }),
          result:
            '{"message":"Screenshot captured for https://example.com","screenshot":"iVBORw0KGgoAAAANSUhEUg...[truncated]',
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.status).toBe('Screenshot captured for https://example.com')
      expect(items[0]?.output).toBe('')
      expect(items[0]?.icon).toBe('✓')
    })
  })
})
