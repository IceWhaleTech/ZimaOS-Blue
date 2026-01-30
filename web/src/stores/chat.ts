import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Conversation, Message, SendMessageRequest, MessageStats } from '@/api/chat'
import { conversationApi, messageApi } from '@/api/chat'
import { SSEClient } from '@/utils/sse'
import { useSettingsStore } from './settings'

const PAGE_SIZE = 50

// Store for message metadata (provider, model, stats) - keyed by message ID
const messageMetadata = ref<Map<string, { provider?: string; model?: string; stats?: MessageStats }>>(new Map())

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
  const securityBlocked = ref<{ message: string; threatLevel: string } | null>(null)

  // Pagination state
  const hasMoreMessages = ref(false)
  const loadingMore = ref(false)
  const currentPage = ref(0)

  // Search state
  const searchQuery = ref('')
  const searchResults = ref<Conversation[] | null>(null)
  const searching = ref(false)

  // Multi-select state
  const selectedMessageIds = ref<Set<string>>(new Set())
  const isMultiSelectMode = ref(false)

  // SSE client for streaming
  const sseClient = new SSEClient()

  // Computed
  const currentConversation = computed(() =>
    conversations.value.find((c) => c.id === currentConversationId.value)
  )

  const sortedConversations = computed(() => {
    const list = searchResults.value !== null ? searchResults.value : conversations.value
    return [...list].sort(
      (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    )
  })

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
    // Don't clear messages immediately to avoid flash
    // Reset pagination state
    hasMoreMessages.value = false
    currentPage.value = 0

    try {
      // Fetch messages without setting loading state to avoid flash
      error.value = null
      const response = await messageApi.list(id, PAGE_SIZE, 0)
      const fetchedMessages = response.data

      // Only update if we're still on the same conversation
      if (currentConversationId.value === id) {
        messages.value = fetchedMessages
        hasMoreMessages.value = fetchedMessages.length === PAGE_SIZE
        currentPage.value = 0
      }
    } catch (e) {
      if (currentConversationId.value === id) {
        error.value = e instanceof Error ? e.message : 'Failed to fetch messages'
        messages.value = []
      }
    }
  }

  async function fetchMessages(conversationId: string, page = 0) {
    // Save metadata from current messages before refresh (for messages not yet persisted to DB)
    const savedMetadata: Map<number, { provider?: string; model?: string; stats?: MessageStats }> = new Map()
    if (page === 0) {
      // Save metadata by index for the last few assistant messages
      messages.value.forEach((msg, index) => {
        if (msg.role === 'assistant' && (msg.provider || msg.model || msg.stats)) {
          savedMetadata.set(index, {
            provider: msg.provider,
            model: msg.model,
            stats: msg.stats,
          })
        }
      })
    }

    try {
      loading.value = true
      error.value = null
      const response = await messageApi.list(
        conversationId,
        PAGE_SIZE,
        page * PAGE_SIZE
      )
      const fetchedMessages = response.data

      if (page === 0) {
        // Only restore metadata if server didn't return it (for backwards compatibility)
        savedMetadata.forEach((meta, index) => {
          if (fetchedMessages[index] && fetchedMessages[index].role === 'assistant') {
            // Only use saved metadata if server didn't return stats
            if (!fetchedMessages[index].stats && meta.stats) {
              fetchedMessages[index] = {
                ...fetchedMessages[index],
                provider: fetchedMessages[index].provider || meta.provider,
                model: fetchedMessages[index].model || meta.model,
                stats: meta.stats,
              }
            }
          }
        })
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
      securityBlocked.value = null

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
          // Update the last message (assistant's response) - use Vue's reactivity properly
          const lastIndex = messages.value.length - 1
          if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
            // Create a new object to trigger Vue reactivity
            messages.value[lastIndex] = {
              ...messages.value[lastIndex],
              content: streamingContent.value,
            }
          }
        },
        onError: (err) => {
          error.value = err.message
          // Remove the placeholder message on error
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
        },
        onBlocked: (message, threatLevel) => {
          securityBlocked.value = { message, threatLevel }
          // Remove placeholder messages
          messages.value = messages.value.filter(
            (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
          )
        },
        onComplete: (finalChunk) => {
          streaming.value = false
          // Store metadata from final chunk directly on the message object
          // This ensures metadata persists even after fetchMessages() refreshes the list
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
              // Update the message with metadata inline (this will be visible immediately)
              messages.value[lastIndex] = {
                ...messages.value[lastIndex],
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              }
              // Also store in metadata map using streaming ID as backup
              const msgId = messages.value[lastIndex].id
              messageMetadata.value.set(msgId, {
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              })
            }
          }
          // Refresh messages to get the actual IDs from server
          fetchMessages(conversationId)
          // Refresh conversations to get updated title (auto-generated after first message)
          fetchConversations()
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
    searchQuery.value = query

    // If query is empty, clear search results and show all conversations
    if (!query.trim()) {
      searchResults.value = null
      searching.value = false
      return conversations.value
    }

    try {
      searching.value = true
      const response = await conversationApi.search(query)
      searchResults.value = response.data
      return response.data
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to search conversations'
      // Fall back to local filtering on error
      const localResults = conversations.value.filter(c =>
        c.title.toLowerCase().includes(query.toLowerCase())
      )
      searchResults.value = localResults
      return localResults
    } finally {
      searching.value = false
    }
  }

  function clearSearch() {
    searchQuery.value = ''
    searchResults.value = null
    searching.value = false
  }

  function clearError() {
    error.value = null
  }

  function clearSecurityBlocked() {
    securityBlocked.value = null
  }

  function getMessageMetadata(messageId: string) {
    return messageMetadata.value.get(messageId)
  }

  // Multi-select actions
  function toggleMessageSelection(messageId: string) {
    if (selectedMessageIds.value.has(messageId)) {
      selectedMessageIds.value.delete(messageId)
    } else {
      selectedMessageIds.value.add(messageId)
    }
    // Trigger reactivity
    selectedMessageIds.value = new Set(selectedMessageIds.value)
  }

  function selectMessage(messageId: string) {
    selectedMessageIds.value.add(messageId)
    selectedMessageIds.value = new Set(selectedMessageIds.value)
  }

  function deselectMessage(messageId: string) {
    selectedMessageIds.value.delete(messageId)
    selectedMessageIds.value = new Set(selectedMessageIds.value)
  }

  function clearSelection() {
    selectedMessageIds.value = new Set()
    isMultiSelectMode.value = false
  }

  function enterMultiSelectMode(initialMessageId?: string) {
    isMultiSelectMode.value = true
    if (initialMessageId) {
      selectedMessageIds.value = new Set([initialMessageId])
    }
  }

  function exitMultiSelectMode() {
    isMultiSelectMode.value = false
    selectedMessageIds.value = new Set()
  }

  async function deleteSelectedMessages() {
    if (!currentConversationId.value || selectedMessageIds.value.size === 0) {
      return
    }

    const idsToDelete = Array.from(selectedMessageIds.value)

    try {
      await messageApi.delete(currentConversationId.value, idsToDelete)
      // Remove deleted messages from local state
      messages.value = messages.value.filter((m) => !selectedMessageIds.value.has(m.id))
      // Clear selection
      clearSelection()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete messages'
      throw e
    }
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
    securityBlocked,
    hasMoreMessages,
    loadingMore,
    searchQuery,
    searching,
    selectedMessageIds,
    isMultiSelectMode,

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
    clearSearch,
    clearError,
    clearSecurityBlocked,
    getMessageMetadata,
    toggleMessageSelection,
    selectMessage,
    deselectMessage,
    clearSelection,
    enterMultiSelectMode,
    exitMultiSelectMode,
    deleteSelectedMessages,
  }
})
