import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useChatStore } from '@/stores/chat'
import { conversationApi, messageApi } from '@/api/chat'

vi.mock('@/api/chat', () => ({
  conversationApi: {
    create: vi.fn(),
    list: vi.fn(),
    get: vi.fn(),
    delete: vi.fn(),
    search: vi.fn(),
  },
  messageApi: {
    list: vi.fn(),
    send: vi.fn(),
  },
}))

vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    selectedProvider: 'openai',
    selectedModel: 'gpt-4o-mini',
    temperature: 0.7,
    maxTokens: 2048,
  }),
}))

describe('Chat Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
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
      expect(messageApi.list).toHaveBeenCalledWith('1')
      expect(store.messages).toEqual(mockMessages)
    })

    it('should not refetch if same conversation is selected', async () => {
      const store = useChatStore()
      store.currentConversationId = '1'

      await store.selectConversation('1')

      expect(messageApi.list).not.toHaveBeenCalled()
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
  })
})
