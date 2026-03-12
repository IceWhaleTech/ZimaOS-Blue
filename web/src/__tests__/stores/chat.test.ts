import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { parseToolResults, useChatStore } from '@/stores/chat'
import { i18n } from '@/i18n'
import { conversationApi, messageApi } from '@/api/chat'
import { approvalApi } from '@/api/approval'

const mocks = vi.hoisted(() => ({
  sseConnect: vi.fn(),
  sseDisconnect: vi.fn(),
  providerPoolStore: {
    fetchTrialQuota: vi.fn(),
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
  await new Promise(resolve => setTimeout(resolve, 0))
}

describe('Chat Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mocks.sseConnect.mockReset().mockResolvedValue(undefined)
    mocks.sseDisconnect.mockReset()
    mocks.providerPoolStore.fetchTrialQuota.mockReset().mockResolvedValue(undefined)
    vi.mocked(conversationApi.list).mockResolvedValue({ data: [] } as never)
    vi.mocked(messageApi.list).mockResolvedValue({ data: [] } as never)
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

      vi.mocked(messageApi.list).mockImplementation(async (_id: string, _limit = 50, offset = 0) => {
        if (offset === 0) return { data: latestPage } as never
        if (offset === 50) return { data: olderPage1 } as never
        if (offset === 100) return { data: olderPage2 } as never
        return { data: [] } as never
      })

      await store.selectConversation('conv-1')
      expect(store.messages.map(m => m.content)).toEqual(latestPage.map(m => m.content))
      expect(store.hasMoreMessages).toBe(true)

      await store.loadMoreMessages()
      await store.loadMoreMessages()

      const expected = [
        ...olderPage2.map(m => m.content),
        ...olderPage1.map(m => m.content),
        ...latestPage.map(m => m.content),
      ]
      const actual = store.messages.map(m => m.content)

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

      expect(store.pendingExecApproval).toEqual(expect.objectContaining({
        id: 'exec-1',
        type: 'directory',
        command: rawCommand,
        directory: '/tmp',
        session_id: 'conv-1',
      }))
      expect(store.pendingExecApproval?.expires_at).toBeGreaterThan(Date.now())
      expect(store.awaitingConfirmation).toBe(true)
    })

    it('should query tool approvals scoped to the current conversation', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'

      vi.mocked(approvalApi.listPending).mockResolvedValue({
        data: [{
          id: 'approval-1',
          tool_name: 'browser',
          tool_call_id: 'tool-1',
          arguments: { url: 'https://example.com' },
          session_id: 'conv-1',
          created_at: '2026-03-11T00:00:00.000Z',
        }],
      } as never)

      await store.checkPendingApprovals()

      expect(approvalApi.listPending).toHaveBeenCalledWith('conv-1')
      expect(store.pendingApproval).toEqual({
        request_id: 'approval-1',
        tool_name: 'browser',
        tool_call_id: 'tool-1',
        arguments: { url: 'https://example.com' },
      })
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
        { id: 'a', title: 'A', created_at: '2024-01-01T00:00:00.000Z', updated_at: '2024-01-03T00:00:00.000Z' },
        { id: 'c', title: 'C', created_at: '2024-01-02T00:00:00.000Z', updated_at: '2024-01-03T00:00:00.000Z' },
        { id: 'b', title: 'B', created_at: '2024-01-02T00:00:00.000Z', updated_at: '2024-01-03T00:00:00.000Z' },
      ]

      expect(store.sortedConversations.map(c => c.id)).toEqual(['c', 'b', 'a'])
    })
  })

  describe('sendMessage streaming', () => {
    it('reattaches a detached stream when switching back to the original conversation', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        { id: 'conv-1', title: 'Original', created_at: '2026-03-11T00:00:00.000Z', updated_at: '2026-03-11T00:00:00.000Z' },
        { id: 'conv-2', title: 'Other', created_at: '2026-03-11T00:00:01.000Z', updated_at: '2026-03-11T00:00:01.000Z' },
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
        expect(request).toEqual(expect.objectContaining({
          message: 'Need a decision',
          web_search_enabled: true,
          deep_research_enabled: false,
        }))
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

    it('should split streamed Reddit follow-up cards into separate assistant messages before persistence refresh', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        { id: 'conv-1', title: 'Reddit flow', created_at: '2026-03-08T00:00:00.000Z', updated_at: '2026-03-08T00:00:00.000Z' },
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
          { id: 'conv-1', title: 'Reddit flow', created_at: '2026-03-08T00:00:00.000Z', updated_at: '2026-03-08T00:00:02.000Z' },
        ],
      } as never)

      const snapshotAfterSplit: Array<{ id: string; role: string; content: string }> = []
      const snapshotBeforeRefresh: Array<{ id: string; role: string; content: string }> = []

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(expect.objectContaining({
          message: 'Inspect https://www.reddit.com/r/test',
          web_search_enabled: true,
          deep_research_enabled: false,
        }))

        options.onMessage({ delta: webFetchBlock, done: false })
        options.onNewMessage?.(1)
        snapshotAfterSplit.push(...store.messages.map(message => ({
          id: message.id,
          role: message.role,
          content: message.content,
          todo_card_id: (message as any).todo_card_id,
        })))

        options.onMessage({ delta: browserBlock, done: false })
        options.onComplete?.({ done: true, provider: 'openai', model: 'gpt-4o-mini' })
        snapshotBeforeRefresh.push(...store.messages.map(message => ({
          id: message.id,
          role: message.role,
          content: message.content,
        })))
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
      expect(store.messages.some(message => message.id.startsWith('temp-') || message.id.startsWith('streaming-'))).toBe(false)
      expect(store.streaming).toBe(false)
      expect(store.sending).toBe(false)
    })

    it('updates the most recent checklist bubble when todo_updated cannot match a local message id', async () => {
      const store = useChatStore()
      store.currentConversationId = 'conv-1'
      store.conversations = [
        { id: 'conv-1', title: 'Checklist fallback', created_at: '2026-03-10T00:00:00.000Z', updated_at: '2026-03-10T00:00:00.000Z' },
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

      let snapshotAfterTodoUpdate: Array<{ id: string; role: string; content: string; todo_card_id?: string }> = []

      mocks.sseConnect.mockImplementationOnce(async (_conversationId, request, options: any) => {
        expect(_conversationId).toBe('conv-1')
        expect(request).toEqual(expect.objectContaining({
          message: 'Continue the current work',
          web_search_enabled: true,
          deep_research_enabled: false,
        }))

        options.onMessage({ delta: '- [ ] collect facts\n- [ ] write summary', done: false })
        options.onNewMessage?.(1)
        options.onTodoUpdated?.('msg-assistant-current', '- [x] collect facts\n- [ ] write summary', 'todo-checklist-msg-assistant-current')

        snapshotAfterTodoUpdate = store.messages.map(message => ({
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

      const updatedChecklist = snapshotAfterTodoUpdate.find(message => message.content === '- [x] collect facts\n- [ ] write summary')
      expect(updatedChecklist).toBeTruthy()
      expect(updatedChecklist?.id.startsWith('streaming-')).toBe(true)
      expect(updatedChecklist?.todo_card_id).toBe('todo-checklist-msg-assistant-current')

      const staleLegacyMatches = snapshotAfterTodoUpdate.filter(message => message.content.includes('legacy task'))
      expect(staleLegacyMatches).toHaveLength(1)
      expect(snapshotAfterTodoUpdate.at(-1)?.content).toBe('')
    })
  })

  describe('parseToolResults', () => {
    it('extracts exec commands from cmd arguments', () => {
      const items = parseToolResults([
        {
          name: 'exec',
          id: 'exec-cmd',
          args: JSON.stringify({ cmd: 'mkdir -p /Users/orca/.zimaos-blue/data/workspace/tank-battle' }),
          result: JSON.stringify({ exit_code: 0, duration_ms: 25 }),
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.command).toBe('mkdir -p /Users/orca/.zimaos-blue/data/workspace/tank-battle')
      expect(items[0]?.status).toBe('25ms')
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
            screenshot: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO7+5VQAAAAASUVORK5CYII=',
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
          result: '{"message":"Screenshot captured for https://example.com","screenshot":"iVBORw0KGgoAAAANSUhEUg...[truncated]',
        },
      ])

      expect(items).toHaveLength(1)
      expect(items[0]?.status).toBe('Screenshot captured for https://example.com')
      expect(items[0]?.output).toBe('')
      expect(items[0]?.icon).toBe('✓')
    })
  })


})
