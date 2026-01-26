import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Conversation, Message, SendMessageRequest } from '@/api/chat'
import { conversationApi, messageApi } from '@/api/chat'
import { SSEClient } from '@/utils/sse'
import { useSettingsStore } from './settings'

const PAGE_SIZE = 50

export const useChatStore = defineStore('chat', () => {
  // State
  const conversations = ref<Conversation[]>([])
  const currentConversationId = ref<string | null>(null)
  const messages = ref<Message[]>([])
  const loading = ref(false)
  const sending = ref(false)
  const streaming = ref(false)
  const streamingContent = ref('')
  const error = ref<string | null>(null)

  // Pagination state
  const hasMoreMessages = ref(false)
  const loadingMore = ref(false)
  const currentPage = ref(0)

  // SSE client for streaming
  const sseClient = new SSEClient()

  // Computed
  const currentConversation = computed(() =>
    conversations.value.find((c) => c.id === currentConversationId.value)
  )

  const sortedConversations = computed(() =>
    [...conversations.value].sort(
      (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    )
  )

  // Actions
  async function fetchConversations() {
    try {
      loading.value = true
      error.value = null
      const response = await conversationApi.list()
      conversations.value = response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch conversations'
    } finally {
      loading.value = false
    }
  }

  async function createConversation(title?: string) {
    try {
      loading.value = true
      error.value = null
      const response = await conversationApi.create(title)
      conversations.value.unshift(response.data)
      currentConversationId.value = response.data.id
      messages.value = []
      hasMoreMessages.value = false
      currentPage.value = 0
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to create conversation'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function deleteConversation(id: string) {
    try {
      await conversationApi.delete(id)
      conversations.value = conversations.value.filter((c) => c.id !== id)
      if (currentConversationId.value === id) {
        currentConversationId.value = null
        messages.value = []
        hasMoreMessages.value = false
        currentPage.value = 0
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete conversation'
      throw e
    }
  }

  async function selectConversation(id: string) {
    if (currentConversationId.value === id) return

    currentConversationId.value = id
    messages.value = []
    hasMoreMessages.value = false
    currentPage.value = 0
    await fetchMessages(id)
  }

  async function fetchMessages(conversationId: string, page = 0) {
    try {
      loading.value = true
      error.value = null
      const response = await messageApi.list(conversationId, {
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      })
      const fetchedMessages = response.data

      if (page === 0) {
        messages.value = fetchedMessages
      } else {
        // Prepend older messages
        messages.value = [...fetchedMessages, ...messages.value]
      }

      hasMoreMessages.value = fetchedMessages.length === PAGE_SIZE
      currentPage.value = page
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch messages'
    } finally {
      loading.value = false
    }
  }

  async function loadMoreMessages() {
    if (!currentConversationId.value || loadingMore.value || !hasMoreMessages.value) {
      return
    }

    try {
      loadingMore.value = true
      await fetchMessages(currentConversationId.value, currentPage.value + 1)
    } finally {
      loadingMore.value = false
    }
  }

  async function sendMessage(content: string) {
    if (!currentConversationId.value) {
      await createConversation()
    }

    const conversationId = currentConversationId.value!
    const settingsStore = useSettingsStore()

    // Add user message to local state immediately
    const userMessage: Message = {
      id: `temp-${Date.now()}`,
      conversation_id: conversationId,
      role: 'user',
      content,
      created_at: new Date().toISOString(),
    }
    messages.value.push(userMessage)

    const request: SendMessageRequest = {
      message: content,
      provider: settingsStore.selectedProvider,
      model: settingsStore.selectedModel,
      temperature: settingsStore.temperature,
      max_tokens: settingsStore.maxTokens,
    }

    try {
      sending.value = true
      streaming.value = true
      streamingContent.value = ''
      error.value = null

      // Add placeholder for assistant message
      const assistantMessage: Message = {
        id: `streaming-${Date.now()}`,
        conversation_id: conversationId,
        role: 'assistant',
        content: '',
        created_at: new Date().toISOString(),
      }
      messages.value.push(assistantMessage)

      await sseClient.connect(conversationId, request, {
        onMessage: (chunk) => {
          streamingContent.value += chunk.delta
          // Update the last message (assistant's response)
          const lastMessage = messages.value[messages.value.length - 1]
          if (lastMessage.role === 'assistant') {
            lastMessage.content = streamingContent.value
          }
        },
        onError: (err) => {
          error.value = err.message
          // Remove the placeholder message on error
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
        },
        onComplete: () => {
          streaming.value = false
          // Refresh messages to get the actual IDs from server
          fetchMessages(conversationId)
        },
      })
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to send message'
      // Remove placeholder messages on error
      messages.value = messages.value.filter(
        (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
      )
    } finally {
      sending.value = false
      streaming.value = false
      streamingContent.value = ''
    }
  }

  function cancelStreaming() {
    sseClient.disconnect()
    streaming.value = false
    streamingContent.value = ''
  }

  async function searchConversations(query: string) {
    try {
      const response = await conversationApi.search(query)
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to search conversations'
      return []
    }
  }

  function clearError() {
    error.value = null
  }

  return {
    // State
    conversations,
    currentConversationId,
    messages,
    loading,
    sending,
    streaming,
    streamingContent,
    error,
    hasMoreMessages,
    loadingMore,

    // Computed
    currentConversation,
    sortedConversations,

    // Actions
    fetchConversations,
    createConversation,
    deleteConversation,
    selectConversation,
    fetchMessages,
    loadMoreMessages,
    sendMessage,
    cancelStreaming,
    searchConversations,
    clearError,
  }
})
