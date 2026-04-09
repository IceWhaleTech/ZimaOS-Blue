import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { parseToolResults, useChatStore } from '@/stores/chat'
import { i18n } from '@/i18n'
import caESMessages from '@/i18n/locales/ca-ES'
import {
  conversationApi,
  messageApi,
  type ConversationCommandState,
  type ConversationCommandStatePatch,
} from '@/api/chat'
import { approvalApi } from '@/api/approval'

const mocks = vi.hoisted(() => ({
  sseConnect: vi.fn(),
  sseDisconnect: vi.fn(),
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  providerPoolStore: {
    fetchTrialQuota: vi.fn(),
    enabledProviders: [] as Array<{ id: string; type?: string }>,
    providers: [] as Array<{ id: string; type?: string }>,
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

const commandStateByConversation = new Map<string, ConversationCommandState>()

function makeCommandState(
  conversationId: string,
  overrides: Partial<ConversationCommandState> = {}
): ConversationCommandState {
  return {
    conversation_id: conversationId,
    selected_provider_id: '',
    selected_model_id: '',
    offline: false,
    ...overrides,
  }
}

function applyCommandStatePatch(
  conversationId: string,
  patch: ConversationCommandStatePatch
): ConversationCommandState {
  const current = commandStateByConversation.get(conversationId) ?? makeCommandState(conversationId)
  const next: ConversationCommandState = {
    ...current,
    ...patch,
    conversation_id: conversationId,
  }

  commandStateByConversation.set(conversationId, next)
  return next
}

function makeBootstrapResponse(conversationId: string, overrides: Record<string, unknown> = {}) {
  return {
    data: {
      command_state:
        commandStateByConversation.get(conversationId) ?? makeCommandState(conversationId),
      active_stream: {
        conversation_id: conversationId,
        active: false,
      },
      current_tasks: [],
      background_tasks: [],
      pending_approval: null,
      pending_question: null,
      pending_exec_approval: null,
      ...overrides,
    },
  } as never
}

async function defaultApiGet(path: string) {
  const bootstrapMatch = String(path).match(/^\/conversations\/([^/]+)\/bootstrap$/)
  if (bootstrapMatch) {
    return makeBootstrapResponse(bootstrapMatch[1] ?? '')
  }
  return { data: {} } as never
}

describe('Chat Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useRealTimers()
    vi.clearAllMocks()
    commandStateByConversation.clear()
    i18n.global.locale.value = 'en-US'
    globalThis.localStorage?.clear?.()
    mocks.sseConnect.mockReset().mockResolvedValue(undefined)
    mocks.sseDisconnect.mockReset()
    mocks.apiGet.mockReset().mockImplementation(defaultApiGet)
    mocks.apiPost.mockReset().mockResolvedValue({ data: {} } as never)
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    mocks.providerPoolStore.enabledProviders = []
    mocks.providerPoolStore.providers = []
    mocks.providerPoolStore.models = []
    mocks.providerPoolStore.getProviderDisplayName.mockImplementation(
      (providerId: string) => providerId
    )
    vi.mocked(conversationApi.list).mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.delete).mockResolvedValue({ data: { success: true, deleted: 0 } } as never)
    vi.mocked(conversationApi.patchCommandState).mockImplementation(
      async (conversationId, patch) => {
        return {
          data: applyCommandStatePatch(conversationId, patch),
        } as never
      }
    )
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

  describe('pending confirmation recovery', () => {
    it('only requests bootstrap once per conversation during non-forced recovery', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'

      await store.recoverPendingConfirmations(false)
      await store.recoverPendingConfirmations(false)

      const bootstrapCalls = mocks.apiGet.mock.calls.filter(([path]) =>
        String(path).includes('/conversations/conv-1/bootstrap')
      )

      expect(bootstrapCalls).toHaveLength(1)
      expect(
        mocks.apiGet.mock.calls.some(([path]) =>
          String(path).includes('/ask-user-question/pending')
        )
      ).toBe(false)
      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/exec/approvals/pending'))
      ).toBe(false)
    })

    it('rechecks bootstrap during forced recovery', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'

      await store.recoverPendingConfirmations(false)
      await store.recoverPendingConfirmations(true)

      const bootstrapCalls = mocks.apiGet.mock.calls.filter(([path]) =>
        String(path).includes('/conversations/conv-1/bootstrap')
      )

      expect(bootstrapCalls).toHaveLength(2)
      expect(
        mocks.apiGet.mock.calls.some(([path]) =>
          String(path).includes('/ask-user-question/pending')
        )
      ).toBe(false)
      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/exec/approvals/pending'))
      ).toBe(false)
    })

    it('does not hit pending confirmation routes when no conversation is selected', async () => {
      const store = useChatStore()

      await store.checkPendingQuestion()

      expect(mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/bootstrap'))).toBe(
        false
      )
      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/pending-confirmations'))
      ).toBe(false)
      expect(
        mocks.apiGet.mock.calls.some(([path]) =>
          String(path).includes('/ask-user-question/pending')
        )
      ).toBe(false)
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

    it('hydrates sparse conversation state from the bootstrap endpoint when available', async () => {
      const mockMessages = [
        {
          id: 'm1',
          conversation_id: '1',
          role: 'assistant',
          content: 'Bootstrapped',
          created_at: '2024-01-01',
        },
      ]
      vi.mocked(messageApi.list).mockResolvedValue({ data: mockMessages } as never)
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return {
            data: {
              command_state: {
                conversation_id: '1',
                selected_provider_id: '',
                selected_model_id: '',
                offline: false,
              },
              active_stream: {
                conversation_id: '1',
                active: true,
                stream_id: 'stream-1',
              },
              current_tasks: [],
              background_tasks: [],
            },
          } as never
        }
        if (path.includes('/ask-user-question/pending')) {
          return { data: { pending: false } } as never
        }
        if (path.includes('/exec/approvals/pending')) {
          return { data: { pending: false } } as never
        }
        return { data: {} } as never
      })

      const store = useChatStore()
      await store.selectConversation('1')

      expect(store.streaming).toBe(true)
      expect(store.streamUIState.phase).toBe('streaming')
      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/messages/active-stream'))
      ).toBe(false)
    })

    it('hydrates pending confirmations from bootstrap without falling back to sparse pending endpoints', async () => {
      vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return {
            data: {
              command_state: {
                conversation_id: '1',
                selected_provider_id: '',
                selected_model_id: '',
                offline: false,
              },
              active_stream: {
                conversation_id: '1',
                active: false,
              },
              current_tasks: [],
              background_tasks: [],
              pending_approval: {
                id: 'approval-1',
                tool_name: 'browser',
                tool_call_id: 'tool-1',
                arguments: { url: 'https://example.com' },
                session_id: '1',
                binding_hash: 'bind-1',
              },
              pending_question: {
                id: 'question-1',
                session_id: '1',
                questions: [{ id: 'q1', question: 'Need input?', header: 'Question' }],
                expires_at: Date.now() + 60_000,
              },
              pending_exec_approval: {
                id: 'exec-1',
                session_id: '1',
                type: 'command',
                command: 'ls -la',
                expires_at: Date.now() + 60_000,
              },
            },
          } as never
        }
        if (path.includes('/ask-user-question/pending')) {
          return { data: { pending: false } } as never
        }
        if (path.includes('/exec/approvals/pending')) {
          return { data: { pending: false } } as never
        }
        return { data: {} } as never
      })

      const store = useChatStore()
      await store.selectConversation('1')

      expect(store.pendingApproval).toEqual({
        request_id: 'approval-1',
        tool_name: 'browser',
        tool_call_id: 'tool-1',
        arguments: { url: 'https://example.com' },
        session_id: '1',
        binding_hash: 'bind-1',
      })
      expect(store.pendingQuestion).toEqual(
        expect.objectContaining({
          id: 'question-1',
          session_id: '1',
        })
      )
      expect(store.pendingExecApproval).toEqual(
        expect.objectContaining({
          id: 'exec-1',
          session_id: '1',
          type: 'command',
          command: 'ls -la',
        })
      )
      expect(store.awaitingConfirmation).toBe(true)
      expect(
        mocks.apiGet.mock.calls.some(([path]) =>
          String(path).includes('/ask-user-question/pending')
        )
      ).toBe(false)
      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/exec/approvals/pending'))
      ).toBe(false)
    })

    it('should not refetch if same conversation is selected', async () => {
      const store = useChatStore()
      store.currentConversationId = '1'

      await store.selectConversation('1')

      expect(messageApi.list).not.toHaveBeenCalled()
    })

    it('hydrates command state from bootstrap for each selected conversation', async () => {
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: {
              conversation_id: '1',
              selected_provider_id: '',
              selected_model_id: '',
              offline: false,
            },
          })
        }
        if (path === '/conversations/2/bootstrap') {
          return makeBootstrapResponse('2', {
            command_state: {
              conversation_id: '2',
              selected_provider_id: '',
              selected_model_id: '',
              offline: false,
            },
          })
        }
        return defaultApiGet(path)
      })

      const store = useChatStore()

      await store.selectConversation('1')
      await store.selectConversation('2')

      expect(
        mocks.apiGet.mock.calls
          .map(([path]) => String(path))
          .filter((path) => path.endsWith('/bootstrap'))
      ).toEqual(['/conversations/1/bootstrap', '/conversations/2/bootstrap'])
      expect(store.currentConversationId).toBe('2')
    })

    it('hydrates bootstrap command state responses without legacy toggle fields', async () => {
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: {
              conversation_id: '1',
              selected_provider_id: '',
              selected_model_id: '',
              offline: false,
            },
          })
        }
        return defaultApiGet(path)
      })

      const store = useChatStore()
      await store.selectConversation('1')
    })

    it('does not reuse another conversation command state when bootstrap omits it', async () => {
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: {
              conversation_id: '1',
              selected_provider_id: '',
              selected_model_id: '',
              offline: false,
            },
          })
        }
        if (path === '/conversations/2/bootstrap') {
          return makeBootstrapResponse('2', {
            command_state: null,
          })
        }
        return defaultApiGet(path)
      })

      const store = useChatStore()

      await store.selectConversation('1')
      await store.selectConversation('2')

      expect(store.currentConversationId).toBe('2')
    })

    it('refreshes slash-command command state from bootstrap instead of the standalone command-state API', async () => {
      const store = useChatStore()
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Commands',
          created_at: '2026-03-22T00:00:00.000Z',
          updated_at: '2026-03-22T00:00:00.000Z',
        },
      ]

      let bootstrapReadCount = 0
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/conv-1/bootstrap') {
          bootstrapReadCount++
          return {
            data: {
              command_state: {
                conversation_id: 'conv-1',
                selected_provider_id: '',
                selected_model_id: '',
                offline: bootstrapReadCount >= 2,
              },
              active_stream: {
                conversation_id: 'conv-1',
                active: false,
              },
              current_tasks: [],
              background_tasks: [],
              pending_approval: null,
              pending_question: null,
              pending_exec_approval: null,
            },
          } as never
        }
        if (path.includes('/ask-user-question/pending')) {
          return { data: { pending: false } } as never
        }
        if (path.includes('/exec/approvals/pending')) {
          return { data: { pending: false } } as never
        }
        return { data: {} } as never
      })

      await store.selectConversation('conv-1')
      expect(store.offlineMode).toBe(false)

      let streamOptions: any
      let resolveStream: (() => void) | null = null
      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        streamOptions = options
        await new Promise<void>((resolve) => {
          resolveStream = resolve
        })
      })

      const sendPromise = store.sendMessage('/offline on')
      await flushMicrotasks()

      streamOptions.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
      resolveStream?.()
      await sendPromise
      await settleAsyncWork()

      expect(bootstrapReadCount).toBe(2)
      expect(store.offlineMode).toBe(true)
      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/command-state'))
      ).toBe(false)
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

    it('should not present command approvals as directory approvals', () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      const workspaceDir = '/Users/orca/.zimaos-blue/data/workspace'

      store.setPendingExecApproval({
        approval: {
          id: 'exec-command-1',
          type: 'command',
          command: 'rm -rf /tmp/demo',
          workdir: workspaceDir,
          expires_at: Math.floor((Date.now() + 60_000) / 1000),
          session_id: 'conv-1',
        },
      })

      expect(store.pendingExecApproval?.type).toBe('command')
      expect(store.pendingExecApproval?.workdir).toBe(workspaceDir)
      expect(store.pendingExecApproval?.directory).toBeUndefined()
      expect(store.awaitingConfirmation).toBe(true)
    })

    it('does not poll the standalone exec approval endpoint without an active conversation', async () => {
      const store = useChatStore()

      await store.checkPendingExecApproval()

      expect(
        mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/exec/approvals/pending'))
      ).toBe(false)
      expect(mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/bootstrap'))).toBe(
        false
      )
      expect(store.pendingExecApproval).toBeNull()
    })

    it('hydrates tool approvals scoped to the current conversation from bootstrap', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'

      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/conv-1/bootstrap') {
          return makeBootstrapResponse('conv-1', {
            pending_approval: {
              id: 'approval-1',
              tool_name: 'browser',
              tool_call_id: 'tool-1',
              arguments: { url: 'https://example.com' },
              session_id: 'conv-1',
              binding_hash: 'binding-1',
            },
          })
        }
        return defaultApiGet(path)
      })

      await store.checkPendingApprovals()

      expect(store.pendingApproval).toEqual({
        request_id: 'approval-1',
        tool_name: 'browser',
        tool_call_id: 'tool-1',
        arguments: { url: 'https://example.com' },
        session_id: 'conv-1',
        binding_hash: 'binding-1',
      })
    })

    it('does not poll the standalone tool approval endpoint without an active conversation', async () => {
      const store = useChatStore()

      await store.checkPendingApprovals()

      expect(mocks.apiGet.mock.calls.some(([path]) => String(path).includes('/bootstrap'))).toBe(
        false
      )
      expect(store.pendingApproval).toBeNull()
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
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: {
              conversation_id: '1',
              selected_provider_id: 'minimax',
              selected_model_id: 'minimax-m2.7',
              offline: false,
            },
          })
        }
        return defaultApiGet(path)
      })

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

    it('preserves a provider-only command-state pin and uses it for requests', async () => {
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: {
              conversation_id: '1',
              selected_provider_id: 'openrouter',
              selected_model_id: '',
              offline: false,
            },
          })
        }
        return defaultApiGet(path)
      })

      const store = useChatStore()
      await store.selectConversation('1')

      expect(store.selectedProviderId).toBe('openrouter')
      expect(store.modelPreference).toBe('auto')

      await store.sendMessage('hi')

      expect(mocks.sseConnect).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          provider: 'openrouter',
          model: '',
        }),
        expect.any(Object)
      )
    })

    it('drops an invalid provider-only pin from bootstrap when the provider is no longer enabled', async () => {
      mocks.providerPoolStore.enabledProviders = [{ id: 'openrouter', type: 'builtin' }]
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: makeCommandState('1', {
              selected_provider_id: 'removed-provider',
              selected_model_id: '',
            }),
          })
        }
        return defaultApiGet(path)
      })

      const store = useChatStore()
      await store.selectConversation('1')

      expect(store.selectedProviderId).toBe('')
      expect(store.providerPinOnlyActive).toBe(false)
      expect(store.modelPreference).toBe('auto')

      await store.sendMessage('hi')

      expect(mocks.sseConnect).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          provider: '',
          model: '',
        }),
        expect.any(Object)
      )
    })

    it('clears a provider-only pin before sending when that provider is no longer enabled', async () => {
      const provider = { id: 'openrouter', type: 'builtin' }
      mocks.providerPoolStore.providers = [provider]
      mocks.providerPoolStore.enabledProviders = [provider]

      const store = useChatStore()
      store.currentConversationId = '1'
      await store.setProviderPinOnly('openrouter')

      mocks.providerPoolStore.providers = [provider]
      mocks.providerPoolStore.enabledProviders = []

      await store.sendMessage('hi')

      expect(store.selectedProviderId).toBe('')
      expect(store.providerPinOnlyActive).toBe(false)
      expect(store.modelPreference).toBe('auto')
      expect(mocks.sseConnect).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          provider: '',
          model: '',
        }),
        expect.any(Object)
      )
    })

    it('clears the pinned provider when switching back to auto', async () => {
      const store = useChatStore()
      store.currentConversationId = '1'
      store.selectedProviderId = 'openrouter'
      store.providerPinOnlyActive = true
      store.modelPreference = 'openrouter/gpt-5'

      await store.setModelPreference('auto')

      expect(store.selectedProviderId).toBe('')
      expect(store.providerPinOnlyActive).toBe(false)
      expect(store.modelPreference).toBe('auto')
      expect(conversationApi.patchCommandState).toHaveBeenCalledWith('1', {
        selected_provider_id: '',
        selected_model_id: '',
      })
    })

    it('sets a provider-only pin and persists it as provider plus auto model', async () => {
      const store = useChatStore()
      store.currentConversationId = '1'

      await store.setProviderPinOnly('openrouter')

      expect(store.selectedProviderId).toBe('openrouter')
      expect(store.providerPinOnlyActive).toBe(true)
      expect(store.modelPreference).toBe('auto')
      expect(conversationApi.patchCommandState).toHaveBeenCalledWith('1', {
        selected_provider_id: 'openrouter',
        selected_model_id: '',
      })
    })

    it('seeds a new conversation with a provider-only pin selected before the first send', async () => {
      vi.mocked(conversationApi.create).mockResolvedValue({
        data: {
          id: 'new-id',
          title: 'New Chat',
          created_at: '2024-01-01',
          updated_at: '2024-01-01',
        },
      } as never)

      const store = useChatStore()
      await store.setProviderPinOnly('openrouter')

      await store.createConversation('New Chat')

      expect(store.selectedProviderId).toBe('openrouter')
      expect(store.providerPinOnlyActive).toBe(true)
      expect(store.modelPreference).toBe('auto')
      expect(conversationApi.patchCommandState).toHaveBeenCalledWith('new-id', {
        selected_provider_id: 'openrouter',
        selected_model_id: '',
        offline: false,
      })
    })

    it('seeds a new conversation with the current provider-only pin after hydrating another conversation', async () => {
      vi.mocked(conversationApi.create).mockResolvedValue({
        data: {
          id: 'new-id',
          title: 'New Chat',
          created_at: '2024-01-01',
          updated_at: '2024-01-01',
        },
      } as never)
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/1/bootstrap') {
          return makeBootstrapResponse('1', {
            command_state: makeCommandState('1', {
              selected_provider_id: 'openrouter',
              selected_model_id: '',
            }),
          })
        }
        return defaultApiGet(path)
      })

      const store = useChatStore()
      await store.selectConversation('1')

      expect(store.selectedProviderId).toBe('openrouter')
      expect(store.providerPinOnlyActive).toBe(true)

      await store.createConversation('New Chat')

      expect(store.currentConversationId).toBe('new-id')
      expect(store.selectedProviderId).toBe('openrouter')
      expect(store.providerPinOnlyActive).toBe(true)
      expect(conversationApi.patchCommandState).toHaveBeenCalledWith('new-id', {
        selected_provider_id: 'openrouter',
        selected_model_id: '',
        offline: false,
      })
    })

    it('does not send a provider when auto routing is selected', async () => {
      const store = useChatStore()
      store.currentConversationId = '1'
      store.selectedProviderId = 'openrouter'
      store.modelPreference = 'auto'

      await store.sendMessage('hi')

      expect(mocks.sseConnect).toHaveBeenCalledWith(
        '1',
        expect.objectContaining({
          provider: '',
          model: '',
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
    it('queues an auto-fallback prompt when a fixed model is unavailable', async () => {
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
      store.selectedProviderId = 'anthropic'
      store.modelPreference = 'anthropic/claude-opus-4-5-20251101'

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, options: any) => {
        options.onError?.(
          new Error(
            "HTTP 400: No available AI provider for model 'claude-opus-4-5-20251101' across all groups checked."
          )
        )
      })

      await store.sendMessage('hello')

      expect(store.pendingModelAutoFallback).toEqual(
        expect.objectContaining({
          conversationId: 'conv-1',
          retryKind: 'send',
          requestedProviderId: 'anthropic',
          requestedModelId: 'claude-opus-4-5-20251101',
        })
      )
      expect(store.streamError).toBeNull()
      expect(store.messages.some((message) => message.id.startsWith('temp-'))).toBe(false)
    })

    it('switches to auto routing and retries after confirming the fallback prompt', async () => {
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
      store.selectedProviderId = 'anthropic'
      store.modelPreference = 'anthropic/claude-opus-4-5-20251101'

      vi.mocked(messageApi.list)
        .mockResolvedValueOnce({
          data: [
            {
              id: 'msg-user-1',
              conversation_id: 'conv-1',
              role: 'user',
              content: 'hello',
              created_at: '2026-03-22T00:00:00.000Z',
            },
          ],
        } as never)
        .mockResolvedValueOnce({
          data: [
            {
              id: 'msg-user-1',
              conversation_id: 'conv-1',
              role: 'user',
              content: 'hello',
              created_at: '2026-03-22T00:00:00.000Z',
            },
            {
              id: 'msg-assistant-1',
              conversation_id: 'conv-1',
              role: 'assistant',
              content: 'Recovered with auto routing',
              created_at: '2026-03-22T00:00:01.000Z',
            },
          ],
        } as never)

      let retryRequest: Record<string, unknown> | null = null
      mocks.sseConnect
        .mockImplementationOnce(async (_conversationId, _request, options: any) => {
          options.onError?.(
            new Error(
              "HTTP 400: No available AI provider for model 'claude-opus-4-5-20251101' across all groups checked."
            )
          )
        })
        .mockImplementationOnce(async (_conversationId, request, options: any) => {
          retryRequest = request as Record<string, unknown>
          options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
        })

      await store.sendMessage('hello')
      await store.confirmModelAutoFallbackRetry()
      await settleAsyncWork()

      expect(store.modelPreference).toBe('auto')
      expect(store.pendingModelAutoFallback).toBeNull()
      expect(conversationApi.patchCommandState).toHaveBeenCalledWith('conv-1', {
        selected_provider_id: '',
        selected_model_id: '',
      })
      expect(retryRequest).toMatchObject({
        message: 'hello',
        provider: '',
        model: '',
        regenerate: true,
      })
    })

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

    it('replaces auto request metadata with the resolved model after stream completion and refresh', async () => {
      const routedModel = 'provider-pool-picked-2026-04-09'
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.modelPreference = 'auto'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Resolved model display',
          created_at: '2026-04-09T00:00:00.000Z',
          updated_at: '2026-04-09T00:00:00.000Z',
        },
      ]

      vi.mocked(messageApi.list).mockResolvedValue({
        data: [
          {
            id: 'msg-user-1',
            conversation_id: 'conv-1',
            role: 'user',
            content: 'Which model answered this?',
            created_at: '2026-04-09T00:00:00.000Z',
          },
          {
            id: 'msg-assistant-1',
            conversation_id: 'conv-1',
            role: 'assistant',
            content: 'A concrete routed model answered this.',
            provider: 'openrouter',
            model: routedModel,
            created_at: '2026-04-09T00:00:01.000Z',
          },
        ],
      } as never)

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Which model answered this?',
            provider: '',
            model: '',
          })
        )

        options.onMessage?.({ delta: 'A concrete routed model answered this.', done: false })
        options.onComplete?.({
          done: true,
          provider: 'openrouter',
          model: routedModel,
        })
      })

      await store.sendMessage('Which model answered this?')
      await settleAsyncWork()

      expect(store.messages).toHaveLength(2)
      expect(store.messages[1]?.role).toBe('assistant')
      expect(store.messages[1]?.model).toBe(routedModel)
      expect(store.messages[1]?.model).not.toBe('auto')
      expect(store.messages[1]?.provider).toBe('openrouter')
      expect(store.messages[1]?.id).toBe('msg-assistant-1')
      expect(store.messages.some((message) => message.id.startsWith('streaming-'))).toBe(false)
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

    it('hydrates an active server stream with persisted preview content after local state is gone', async () => {
      const store = useChatStore()

      vi.mocked(messageApi.list).mockResolvedValue({
        data: [
          {
            id: 'msg-assistant-1',
            conversation_id: 'conv-1',
            role: 'assistant',
            content: '- [x] 收集信息\n- [ ] 写总结\n\n我继续执行第二步。',
            created_at: '2026-03-12T00:00:00.000Z',
          },
        ],
      } as never)
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/conv-1/bootstrap') {
          return makeBootstrapResponse('conv-1', {
            active_stream: {
              conversation_id: 'conv-1',
              active: true,
              stream_id: 'stream-preview-1',
            },
          })
        }
        return defaultApiGet(path)
      })

      await store.selectConversation('conv-1')

      expect(store.currentConversationId).toBe('conv-1')
      expect(store.streaming).toBe(true)
      expect(store.sending).toBe(false)
      expect(store.executingConversationIds).toEqual(['conv-1'])
      expect(store.streamingContent).toContain('我继续执行第二步。')
      expect(store.messages).toHaveLength(1)
      expect(store.messages[0]?.id).toBe('msg-assistant-1')
      expect(store.messages[0]?.content).toContain('我继续执行第二步。')
    })

    it('creates a placeholder when the server reports an active stream without persisted preview content', async () => {
      const store = useChatStore()

      vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/conv-1/bootstrap') {
          return makeBootstrapResponse('conv-1', {
            active_stream: {
              conversation_id: 'conv-1',
              active: true,
              stream_id: 'stream-live-1',
            },
          })
        }
        return defaultApiGet(path)
      })

      await store.selectConversation('conv-1')

      expect(store.currentConversationId).toBe('conv-1')
      expect(store.streaming).toBe(true)
      expect(store.sending).toBe(false)
      expect(store.toolExecuting).toBe(true)
      expect(store.streamUIState.phase).toBe('executing')
      expect(store.executingConversationIds).toEqual(['conv-1'])
      expect(store.messages).toHaveLength(1)
      expect(store.messages[0]?.role).toBe('assistant')
      expect(store.messages[0]?.id.startsWith('streaming-')).toBe(true)
    })

    it('does not reuse the previous assistant reply as preview when the latest persisted message is user-only', async () => {
      const store = useChatStore()

      vi.mocked(messageApi.list).mockResolvedValue({
        data: [
          {
            id: 'msg-assistant-prev',
            conversation_id: 'conv-1',
            role: 'assistant',
            content: '上一轮已经完成的回复',
            created_at: '2026-03-12T00:00:00.000Z',
          },
          {
            id: 'msg-user-latest',
            conversation_id: 'conv-1',
            role: 'user',
            content: '继续执行新的任务',
            created_at: '2026-03-12T00:00:01.000Z',
          },
        ],
      } as never)
      mocks.apiGet.mockImplementation(async (path: string) => {
        if (path === '/conversations/conv-1/bootstrap') {
          return makeBootstrapResponse('conv-1', {
            active_stream: {
              conversation_id: 'conv-1',
              active: true,
              stream_id: 'stream-live-2',
            },
          })
        }
        return defaultApiGet(path)
      })

      await store.selectConversation('conv-1')

      expect(store.streaming).toBe(true)
      expect(store.toolExecuting).toBe(true)
      expect(store.streamUIState.phase).toBe('executing')
      expect(store.streamingContent).toBe('')
      expect(store.messages).toHaveLength(3)
      expect(store.messages[0]?.content).toBe('上一轮已经完成的回复')
      expect(store.messages[1]?.content).toBe('继续执行新的任务')
      expect(store.messages[2]?.id.startsWith('streaming-')).toBe(true)
      expect(store.messages[2]?.content).toBe('')
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
        await flushMicrotasks()
        await vi.runAllTimersAsync()
        await flushMicrotasks()

        expect(store.streamUIState.phase).not.toBe('recovering')
        expect(store.streamUIState.phase).not.toBe('interrupted')
        expect(store.messages).toHaveLength(2)
        expect(store.messages.at(-1)?.id).toBe('msg-assistant-1')
        expect(store.messages.at(-1)?.content).toBe('Recovered answer')

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

        await vi.runAllTimersAsync()
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
        harness: {
          quickEval: {
            researchLabel: '研究',
          },
        },
        chat: {
          processTrace: {
            events: {
              requestReady: '请求已准备就绪',
              requestSent: '请求已发送',
              waitingForResponse: '正在等待响应',
              retryingRequest: '正在重试请求',
              providerResolved: '已选定可用路由',
            },
            details: {
              requestDispatched: '正在等待服务器接受请求并开始响应。',
              waitingForResponse: '请求已被接受。正在等待第一段可见输出。',
              providerResolvedModelSwitch: '本次响应已切换到一个可用的上游模型。',
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
      expect(store.processTrace[0]?.detail).not.toContain('网页搜索')
      expect(store.processTrace[0]?.detail).not.toContain('深度研究')
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
      streamOptions.onProcessEvent?.({
        delta: '',
        done: false,
        process_event: 'provider_resolved',
        process_status: 'success',
        process_message: 'Using available route',
        process_detail: 'Switched to an available upstream model for this response.',
        process_provider: 'anthropic',
        process_model: 'claude-haiku-4-5',
      })
      await settleAsyncWork()

      expect(
        store.processTrace.find((item) => item.event === 'provider_failover')?.detail
      ).toContain('正在不使用上一个固定提供商重试这轮工具后续请求。')
      expect(
        store.processTrace.find((item) => item.event === 'continuation_recovery_started')?.detail
      ).toContain('静默恢复')
      expect(store.processTrace.find((item) => item.event === 'provider_resolved')?.label).toBe(
        '已选定可用路由'
      )
      expect(
        store.processTrace.find((item) => item.event === 'provider_resolved')?.detail
      ).toContain('本次响应已切换到一个可用的上游模型。')
      expect(
        store.processTrace.find((item) => item.event === 'provider_resolved')?.detail
      ).toContain('提供商: anthropic')
      expect(
        store.processTrace.find((item) => item.event === 'provider_resolved')?.detail
      ).toContain('模型: claude-haiku-4-5')

      streamOptions.onComplete?.({ delta: '', done: true })
      resolveStream?.()
      await sendPromise
      i18n.global.locale.value = 'en-US'
    })

    it('uses bundled locale process trace copy without manual overrides', async () => {
      i18n.global.setLocaleMessage('ca-ES', caESMessages as never)
      i18n.global.locale.value = 'ca-ES'

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

      let resolveStream: (() => void) | null = null

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, _request, _options: any) => {
        await new Promise<void>((resolve) => {
          resolveStream = resolve
        })
      })

      const sendPromise = store.sendMessage('[CONTINUE]')
      await settleAsyncWork()

      expect(store.processTrace.map((item) => item.label)).toEqual([
        'Sol·licitud preparada',
        'Sol·licitud enviada',
        'En espera de resposta',
      ])
      expect(store.processTrace[0]?.detail).toContain('Missatge: Continua la resposta anterior')
      expect(store.processTrace[0]?.detail).toContain('Proveïdor: Auto')
      expect(store.processTrace[0]?.detail).toContain('Model: Auto')
      expect(store.processTrace[1]?.detail).toBe(
        "S'està esperant que el servidor accepti la sol·licitud i iniciï la resposta."
      )
      expect(store.processTrace[2]?.detail).toBe(
        "La sol·licitud ha estat acceptada. S'està esperant la primera sortida visible."
      )

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

    it('preserves streamed deep research process cards when the final chunk only carries the final result card', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        {
          id: 'conv-1',
          title: 'Deep research streaming',
          created_at: '2026-03-18T00:00:00.000Z',
          updated_at: '2026-03-18T00:00:00.000Z',
        },
      ]

      const initialProgressBlock = makeTypelessBlock({
        type: 'deep-research-progress',
        id: 'deep-research-progress-job-1',
        job_id: 'job-1',
        conversation_id: 'conv-1',
        query: 'Deep research this topic',
        mode: 'deep',
        stage: 'retrieve',
        status: 'running',
        progress: 42,
        iteration: 1,
        latest_action: 'initial_retrieve',
      })
      const planningBlock = makeTypelessBlock({
        type: 'deep-research-event',
        id: 'deep-research-event-job-1-01',
        job_id: 'job-1',
        conversation_id: 'conv-1',
        query: 'Deep research this topic',
        mode: 'deep',
        event_kind: 'planning',
        status: 'info',
        summary: 'Planned 4 research task(s)',
        iteration: 1,
        task_count: 4,
      })
      const fullContextBlock = makeTypelessBlock({
        type: 'deep-research-progress',
        id: 'deep-research-progress-job-1',
        job_id: 'job-1',
        conversation_id: 'conv-1',
        query: 'Deep research this topic',
        mode: 'deep',
        stage: 'fullcontext',
        status: 'running',
        progress: 91,
        iteration: 1,
        latest_action: 'synthesize_full_context',
      })
      const finalResultBlock = makeTypelessBlock({
        type: 'deep-research',
        id: 'deep-research-result-job-1',
        job_id: 'job-1',
        query: 'Deep research this topic',
        mode: 'deep',
        status: 'completed',
        answer: 'Final synthesized answer',
      })

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Deep research this topic',
          })
        )

        options.onMessage?.({ delta: `${initialProgressBlock}\n\n`, done: false })
        options.onMessage?.({ delta: `${planningBlock}\n\n`, done: false })
        options.onMessage?.({ delta: fullContextBlock, done: false })
        options.onComplete?.({
          done: true,
          message_id: 'msg-assistant-final',
          content: finalResultBlock,
        })
      })

      await store.sendMessage('Deep research this topic')
      await settleAsyncWork()

      expect(store.messages).toHaveLength(2)
      expect(store.messages[0]?.role).toBe('user')
      expect(store.messages[1]?.id).toBe('msg-assistant-final')
      expect(store.messages[1]?.content).not.toContain(initialProgressBlock)
      expect(store.messages[1]?.content).toContain(planningBlock)
      expect(store.messages[1]?.content).toContain(fullContextBlock)
      expect(store.messages[1]?.content).toContain(finalResultBlock)
    })

    it('preserves streamed browser progress cards when the final chunk only carries the final result card', async () => {
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
        steps: [
          {
            step: 'navigate',
            name: 'Navigating',
            status: 'completed',
            url: 'https://openai.com/blog',
          },
          {
            step: 'snapshot',
            name: 'Reading page',
            status: 'running',
            url: 'https://openai.com/blog',
          },
        ],
      })
      const webFetchBlock = makeTypelessBlock({
        type: 'web-fetch',
        id: 'web-fetch-chain',
        title: 'web_fetch',
        url: 'https://openai.com/blog',
        status: 'success',
        content: 'Expanded page content from the OpenAI blog.',
      })

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(
          expect.objectContaining({
            message: 'Check the latest OpenAI updates',
          })
        )

        options.onMessage?.({ delta: browserProgressBlock, done: false })
        options.onComplete?.({
          done: true,
          message_id: 'msg-assistant-browser-final',
          content: webFetchBlock,
        })
      })

      await store.sendMessage('Check the latest OpenAI updates')
      await settleAsyncWork()

      expect(store.messages).toHaveLength(2)
      expect(store.messages[1]?.id).toBe('msg-assistant-browser-final')
      expect(store.messages[1]?.content).toContain(browserProgressBlock)
      expect(store.messages[1]?.content).toContain(webFetchBlock)
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

  describe('continueMessage', () => {
    it('queues an auto-fallback prompt when a fixed model is unavailable', async () => {
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
      store.selectedProviderId = 'anthropic'
      store.modelPreference = 'anthropic/claude-opus-4-5-20251101'
      store.messages = [
        {
          id: 'msg-assistant-1',
          conversation_id: 'conv-1',
          role: 'assistant',
          content: 'Partial answer',
          created_at: '2026-03-22T00:00:01.000Z',
        },
      ]

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(request).toEqual(
          expect.objectContaining({
            message: '[CONTINUE]',
            provider: 'anthropic',
            model: 'claude-opus-4-5-20251101',
          })
        )
        options.onError?.(
          new Error(
            "HTTP 400: No available AI provider for model 'claude-opus-4-5-20251101' across all groups checked."
          )
        )
      })

      await store.continueMessage()

      expect(store.pendingModelAutoFallback).toEqual(
        expect.objectContaining({
          conversationId: 'conv-1',
          retryKind: 'continue',
          requestedProviderId: 'anthropic',
          requestedModelId: 'claude-opus-4-5-20251101',
        })
      )
      expect(store.streamError).toBeNull()
    })

    it('queues an auto-fallback prompt when auto-resume hits a fixed-model unavailable error', async () => {
      vi.useFakeTimers()

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
        store.selectedProviderId = 'anthropic'
        store.modelPreference = 'anthropic/claude-opus-4-5-20251101'
        store.sending = true
        store.streaming = true
        store.messages = [
          {
            id: 'streaming-1',
            conversation_id: 'conv-1',
            role: 'assistant',
            content: '',
            created_at: '2026-03-22T00:00:01.000Z',
          },
        ]

        mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
          expect(request).toEqual(
            expect.objectContaining({
              message: '[CONTINUE_AFTER_CANCEL]',
              provider: 'anthropic',
              model: 'claude-opus-4-5-20251101',
            })
          )
          options.onError?.(
            new Error(
              "HTTP 400: No available AI provider for model 'claude-opus-4-5-20251101' across all groups checked."
            )
          )
        })

        store.cancelPreTTFT()
        expect(store.preTTFTCancelActive).toBe(true)

        await vi.advanceTimersByTimeAsync(10000)
        await flushMicrotasks()

        expect(store.preTTFTCancelActive).toBe(false)
        expect(store.pendingModelAutoFallback).toEqual(
          expect.objectContaining({
            conversationId: 'conv-1',
            retryKind: 'continue',
            requestedProviderId: 'anthropic',
            requestedModelId: 'claude-opus-4-5-20251101',
          })
        )
        expect(store.messages.some((message) => message.id.startsWith('streaming-'))).toBe(false)
      } finally {
        vi.useRealTimers()
      }
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
