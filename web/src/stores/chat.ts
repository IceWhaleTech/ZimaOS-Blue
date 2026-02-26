import { defineStore } from 'pinia'
import { ref, shallowRef, computed, watch } from 'vue'
import type { Conversation, Message, SendMessageRequest, MessageStats, MessageAttachment } from '@/api/chat'
import { conversationApi, messageApi, warmupApi, injectionApi } from '@/api/chat'
import { approvalApi } from '@/api/approval'
import type { Decision, ExecDecision } from '@/api/approval'
import api from '@/api/client'
import { SSEClient } from '@/utils/sse'
import { useSettingsStore } from './settings'
import { useProviderPoolStore } from './providerPool'
import { systemApi } from '@/api/system'

const PAGE_SIZE = 50

/** Structured tool result for collapsible detail cards. */
export interface ToolResultItem {
  name: string
  id: string
  command: string       // Extracted command/query/path from args
  args?: string         // Raw args JSON
  icon: '✓' | '✗' | '⏳'
  status: string        // Duration, error message, or status text
  output: string        // Truncated stdout/result
  exitCode?: number
  durationMs?: number
  host?: 'local' | 'sandbox'
  riskLevel?: string
  timestamp: number     // When this result was received
}

/** Format tool results into a process block for styled rendering. */
function formatToolResultsSummary(results: Array<{ name: string; id: string; args?: string; result?: string }>): string {
  if (!results || results.length === 0) return ''
  const items: Array<{ cmd: string; tool: string; icon: string; status: string; output: string }> = []
  for (const r of results) {
    let command = ''
    if (r.args) {
      try {
        const parsed = JSON.parse(r.args)
        command = parsed.command || parsed.query || parsed.path || parsed.name || parsed.sq || parsed.mq || ''
      } catch {
        command = r.args.slice(0, 80)
      }
    }
    let icon = '⏳'
    let status = ''
    let output = ''
    if (r.result) {
      try {
        const res = JSON.parse(r.result)
        // Special handling for ask_user_question tool
        if (r.name === 'ask' && (res.qa || res.sq || res.mq)) {
          icon = '✓'
          const questionText = res.sq || res.mq || ''
          const qaData = res.qa
          if (qaData && Array.isArray(qaData) && qaData.length > 0) {
            const q = qaData[0]
            const question = q.q || questionText
            const answers = q.a || []
            output = `Q: ${question}\nA: ${answers.join(', ')}`
          } else {
            output = `Q: ${questionText}`
          }
        } else if (res.error) {
          icon = '✗'
          status = res.error.slice(0, 100)
        } else if (res.exit_code !== undefined) {
          icon = res.exit_code === 0 ? '✓' : '✗'
          const dur = res.duration_ms ? `${res.duration_ms}ms` : ''
          status = dur
        } else if (res.status) {
          icon = '✓'
          status = res.status
        }
        if (res.stdout && res.stdout.trim() && r.name !== 'ask') {
          output = res.stdout.trim()
          if (output.length > 200) output = output.slice(0, 200) + '...'
        }
        if (res.stderr && res.stderr.trim()) {
          const stderr = res.stderr.trim().slice(0, 120)
          output = output ? `${output}\n${stderr}` : stderr
        }
      } catch {
        output = r.result.slice(0, 200)
      }
    }
    items.push({ cmd: command, tool: r.name, icon, status, output })
  }
  const json = JSON.stringify(items)
  return `\n\n<!-- process-start -->\n\`\`\`process\n${json}\n\`\`\`\n<!-- process-end -->\n\n`
}

/** Parse raw tool results into structured ToolResultItems. */
function parseToolResults(results: Array<{ name: string; id: string; args?: string; result?: string }>): ToolResultItem[] {
  return results.map(r => {
    let command = ''
    if (r.args) {
      try {
        const parsed = JSON.parse(r.args)
        command = parsed.command || parsed.query || parsed.path || parsed.name || parsed.action || parsed.sq || parsed.mq || ''
      } catch {
        // If args is not valid JSON, use it directly for ask_user_question
        if (r.name === 'ask') {
          command = r.args
        } else {
          command = r.args.slice(0, 80)
        }
      }
    }
    let icon: '✓' | '✗' | '⏳' = '⏳'
    let status = ''
    let output = ''
    let exitCode: number | undefined
    let durationMs: number | undefined
    let host: 'local' | 'sandbox' | undefined
    let riskLevel: string | undefined
    if (r.result) {
      try {
        const res = JSON.parse(r.result)
        // Special handling for ask_user_question tool
        if (r.name === 'ask' && (res.qa || res.sq || res.mq)) {
          icon = '✓'
          const questionText = res.sq || res.mq || ''
          const qaData = res.qa
          if (qaData && Array.isArray(qaData) && qaData.length > 0) {
            const q = qaData[0]
            const question = q.q || questionText
            const options = q.o || []
            const answers = q.a || []
            output = `**Q:** ${question}\n\n**Options:** ${options.join(', ')}\n\n**Answer:** ${answers.join(', ')}`
          } else {
            output = `**Q:** ${questionText}`
          }
        } else if (res.error) {
          icon = '✗'; status = res.error.slice(0, 200)
        } else if (res.exit_code !== undefined) {
          icon = res.exit_code === 0 ? '✓' : '✗'
          exitCode = res.exit_code
          durationMs = res.duration_ms
          status = res.duration_ms ? `${res.duration_ms}ms` : ''
        } else if (res.status) {
          icon = '✓'; status = res.status
        } else {
          icon = '✓'
        }
        if (res.stdout?.trim() && r.name !== 'ask') { output = res.stdout.trim() }
        if (res.stderr?.trim()) {
          const stderr = res.stderr.trim()
          output = output ? `${output}\n${stderr}` : stderr
        }
        if (res.host) host = res.host
        if (res.risk_level) riskLevel = res.risk_level
      } catch {
        output = r.result.slice(0, 500)
        icon = '✓'
      }
    }
    return { name: r.name, id: r.id, command, args: r.args, icon, status, output, exitCode, durationMs, host, riskLevel, timestamp: Date.now() }
  })
}

// Store for message metadata (provider, model, stats) - keyed by message ID
const messageMetadata = ref<Map<string, { provider?: string; model?: string; stats?: MessageStats }>>(new Map())

export const useChatStore = defineStore('chat', () => {
  // State
  const conversations = ref<Conversation[]>([])
  const currentConversationId = ref<string | null>(null)
  // Use shallowRef for messages to reduce reactivity overhead
  // Manual triggerRef() calls are needed when mutating the array
  const messages = shallowRef<Message[]>([])
  const loading = ref(false)
  const sending = ref(false)
  const streaming = ref(false)
  const streamingContent = ref('')
  const processContentLength = ref(0) // Length of tool-result process content at the start of streamingContent
  const error = ref<string | null>(null)
  const streamError = ref<string | null>(null) // Error from stream (displayed in chat area)
  const securityBlocked = ref<{ message: string; threatLevel: string } | null>(null)
  const trialExhausted = ref(false) // Trial quota exhausted flag
  const toolExecuting = ref(false) // Tool execution in progress
  const toolExecutingStartTime = ref<number>(0) // Timestamp when tool execution started
  const toolExecutingNames = ref<string[]>([]) // Names of tools being executed
  const toolExecutingCommands = ref<string[]>([]) // Commands being executed (for skill name extraction)
  const toolSandboxAvailable = ref(false) // Sandbox protection available for current exec
  const toolResults = ref<ToolResultItem[]>([]) // Structured tool results for current streaming message
  watch(toolExecuting, (v) => { if (!v) { toolExecutingNames.value = []; toolExecutingCommands.value = []; toolSandboxAvailable.value = false } })
  const contextTrimInfo = ref<{ type: 'pruned' | 'compacted'; messagesPruned?: number; tokensBefore?: number; tokensAfter?: number; before?: number; after?: number } | null>(null)

  // Pre-TTFT cancel state: when user starts typing before first token arrives
  const preTTFTCancelActive = ref(false)
  let preTTFTResumeTimer: ReturnType<typeof setTimeout> | null = null
  const _receivedFirstChunk = ref(false)

  // Computed: true when we're waiting for first token (user message sent, no content yet)
  const isPreTTFT = computed(() => sending.value && !_receivedFirstChunk.value && !preTTFTCancelActive.value)

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

  // Tool approval state
  const pendingApproval = ref<{
    request_id: string
    tool_name: string
    tool_call_id: string
    arguments: Record<string, unknown>
  } | null>(null)

  // Ask-user-question state
  const pendingQuestion = ref<{
    id: string
    questions: Array<{
      id: string
      question: string
      header: string
      options?: Array<{ label: string; description?: string; value?: string }>
      multi_select?: boolean
    }>
    expires_at: number
  } | null>(null)

  // Exec directory approval state
  const pendingExecApproval = ref<{
    id: string
    type: string
    command?: string
    directory?: string
    workdir?: string
    host?: string
    security?: string
    expires_at: number
  } | null>(null)

  const isMultiSelectMode = ref(false)

  // SSE client for streaming
  const sseClient = new SSEClient()

  // Computed
  const currentConversation = computed(() =>
    conversations.value.find((c) => c.id === currentConversationId.value)
  )

  const sortedConversations = computed(() => {
    const list = searchResults.value !== null ? searchResults.value : conversations.value
    return [...list].sort((a, b) => {
      // Pinned conversations first
      if (a.pinned && !b.pinned) return -1
      if (!a.pinned && b.pinned) return 1
      // Then sort by updated_at
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
    })
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

  async function pinConversation(id: string) {
    try {
      await conversationApi.pin(id)
      const conv = conversations.value.find((c) => c.id === id)
      if (conv) {
        conv.pinned = true
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to pin conversation'
      throw e
    }
  }

  async function unpinConversation(id: string) {
    try {
      await conversationApi.unpin(id)
      const conv = conversations.value.find((c) => c.id === id)
      if (conv) {
        conv.pinned = false
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to unpin conversation'
      throw e
    }
  }

  async function selectConversation(id: string) {
    if (currentConversationId.value === id) return

    // Cancel any active streaming before switching — prevents the old
    // conversation's SSE callbacks from writing into the new conversation's
    // message list.
    if (streaming.value || sending.value) {
      sseClient.disconnect()
      streaming.value = false
      sending.value = false
      toolExecuting.value = false
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }

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

    // Check if there's a pending question waiting for user input
    checkPendingQuestion()
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

      // Guard: don't overwrite messages if user switched to a different conversation
      if (currentConversationId.value !== conversationId) return

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

  async function sendMessage(content: string, fileAttachments?: { id: string; file: File; name: string; size: number; type: string; preview?: string; duration?: number }[]) {
    if (!currentConversationId.value) {
      // Use the first part of the message as the conversation title
      const title = content.length > 30 ? content.substring(0, 30) + '...' : content
      await createConversation(title)
    }

    const conversationId = currentConversationId.value!
    const settingsStore = useSettingsStore()

    // Convert file attachments to MessageAttachment format (base64)
    const attachments: MessageAttachment[] = []
    if (fileAttachments && fileAttachments.length > 0) {
      for (const attachment of fileAttachments) {
        // For images, check if preview is a data URL (not blob URL) or read the file
        let base64Data = ''
        if (attachment.preview && attachment.preview.startsWith('data:')) {
          // Remove data URL prefix (e.g., "data:image/png;base64,")
          base64Data = attachment.preview.split(',')[1] || ''
        } else {
          // Read file as base64 (for blob URLs or no preview)
          base64Data = await new Promise<string>((resolve) => {
            const reader = new FileReader()
            reader.onloadend = () => {
              const result = reader.result as string
              resolve(result.split(',')[1] || '')
            }
            reader.onerror = () => resolve('')
            reader.readAsDataURL(attachment.file)
          })
        }

        if (base64Data) {
          attachments.push({
            type: attachment.type.startsWith('image/') ? 'image' : (attachment.type.startsWith('audio/') ? 'audio' : 'file'),
            name: attachment.name,
            mime_type: attachment.type,
            data: base64Data,
            ...(attachment.duration ? { duration: attachment.duration } : {}),
          })
        }
      }
    }

    // Build display content for user message
    // Don't add attachment names to content - they're shown in the attachment preview
    const displayContent = content

    // Add user message to local state immediately
    const userMessage: Message = {
      id: `temp-${Date.now()}`,
      conversation_id: conversationId,
      role: 'user',
      content: displayContent,
      created_at: new Date().toISOString(),
      attachments: attachments.length > 0 ? attachments : undefined,
    }
    messages.value = [...messages.value, userMessage]

    // Don't send provider/model - let backend decide based on routing mode (auto/cloud/local)
    // This ensures the router selects the best available provider
    const request: SendMessageRequest = {
      message: content,
      provider: undefined,
      model: undefined,
      temperature: settingsStore.temperature,
      max_tokens: settingsStore.maxTokens,
      attachments: attachments.length > 0 ? attachments : undefined,
    }

    try {
      sending.value = true
      streaming.value = true
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
      error.value = null
      securityBlocked.value = null
      contextTrimInfo.value = null
      _receivedFirstChunk.value = false

      // Clear any active pre-TTFT cancel state (user is sending a new message)
      if (preTTFTCancelActive.value) {
        preTTFTCancelActive.value = false
        if (preTTFTResumeTimer) {
          clearTimeout(preTTFTResumeTimer)
          preTTFTResumeTimer = null
        }
      }

      // Add placeholder for assistant message
      const assistantMessage: Message = {
        id: `streaming-${Date.now()}`,
        conversation_id: conversationId,
        role: 'assistant',
        content: '',
        created_at: new Date().toISOString(),
      }
      messages.value = [...messages.value, assistantMessage]

      // Capture the conversation ID at send time so callbacks can detect stale streams
      const sendConvId = conversationId

      await sseClient.connect(conversationId, request, {
        onMessage: (chunk) => {
          // Guard: ignore chunks if user switched to a different conversation
          if (currentConversationId.value !== sendConvId) return
          if (!chunk.delta) return
          _receivedFirstChunk.value = true
          // Clear tool executing state when new content arrives
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          streamingContent.value += chunk.delta
          // Update the last message (assistant's response) - use shallowRef properly
          const lastIndex = messages.value.length - 1
          if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
            // Create a new array to trigger shallowRef reactivity
            const newMessages = [...messages.value]
            const currentMsg = newMessages[lastIndex]
            if (currentMsg) {
              newMessages[lastIndex] = {
                ...currentMsg,
                content: streamingContent.value,
              }
              messages.value = newMessages
            }
          }
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== sendConvId) return
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
          // Update the streaming message to show tool execution indicator
          const lastIndex = messages.value.length - 1
          if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
            const newMessages = [...messages.value]
            const currentMsg = newMessages[lastIndex]
            if (currentMsg) {
              newMessages[lastIndex] = {
                ...currentMsg,
                content: streamingContent.value,
              }
              messages.value = newMessages
            }
          }
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== sendConvId) return
          // Store structured tool results for detail cards
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
          // Append tool results as a visual block in the streaming content
          const summary = formatToolResultsSummary(results)
          if (summary) {
            streamingContent.value += summary
            processContentLength.value = streamingContent.value.length
            const lastIndex = messages.value.length - 1
            if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
              const newMessages = [...messages.value]
              const currentMsg = newMessages[lastIndex]
              if (currentMsg) {
                newMessages[lastIndex] = { ...currentMsg, content: streamingContent.value }
                messages.value = newMessages
              }
            }
          }
        },
        onNewMessage: () => {
          if (currentConversationId.value !== sendConvId) return
          // Server persisted previous round — start a new message bubble
          streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
          toolExecuting.value = false
          const newAssistant: Message = {
            id: `streaming-${Date.now()}`,
            conversation_id: conversationId,
            role: 'assistant',
            content: '',
            created_at: new Date().toISOString(),
          }
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (_messageId, content) => {
          if (currentConversationId.value !== sendConvId) return
          // Backend advanced a TODO item — update the message that has the checklist
          const idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          if (idx >= 0) {
            const newMessages = [...messages.value]
            newMessages[idx] = { ...newMessages[idx]!, content }
            messages.value = newMessages
          }
        },
        onInjection: () => {
          if (currentConversationId.value !== sendConvId) return
          // Server confirmed injection — partial response is preserved in DB,
          // new user message stored. The stream will restart server-side.
          // Reset streaming content for the new response.
          streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
          toolExecuting.value = false
          // Add a new streaming placeholder for the restarted response
          const newAssistant: Message = {
            id: `streaming-${Date.now()}`,
            conversation_id: conversationId,
            role: 'assistant',
            content: '',
            created_at: new Date().toISOString(),
          }
          messages.value = [...messages.value, newAssistant]
        },
        onError: (err) => {
          // Guard: if user already switched away, silently ignore
          if (currentConversationId.value !== sendConvId) return
          const wasToolExecuting = toolExecuting.value
          toolExecuting.value = false
          // Map error codes to i18n keys for accurate error messages
          const errorMap: Record<string, string> = {
            'STREAM_EMPTY': 'streamEmpty',
            'STREAM_ERROR': 'streamError',
            'PROVIDER_NO_RESPONSE': 'providerNoResponse',
            'PROVIDER_RETURNED_EMPTY': 'providerReturnedEmpty',
            'No response body': 'noResponseBody',
            'provider_unavailable': 'providerUnavailable',
            'provider_auth_error': 'providerAuthError',
            'provider_rate_limited': 'providerRateLimited',
            'trial_service_busy': 'trialServiceBusy',
          }

          // Transient empty-response errors that can be silently recovered
          const transientErrors = new Set(['STREAM_EMPTY', 'PROVIDER_NO_RESPONSE', 'PROVIDER_RETURNED_EMPTY'])

          // If error happened during tool execution, the server is still processing.
          // Fetch server-persisted content instead of marking as interrupted.
          if (wasToolExecuting) {
            streaming.value = false
            // Don't show error banner — we'll recover by fetching from server
            fetchMessages(conversationId).then(() => {
              if (currentConversationId.value !== sendConvId) return
              messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            })
            return
          }

          // For transient empty-response errors, try fetching server-persisted content first.
          // The backend may have persisted partial content or tool results even though
          // the stream appeared empty to the frontend.
          if (transientErrors.has(err.message)) {
            streaming.value = false
            fetchMessages(conversationId).then(() => {
              if (currentConversationId.value !== sendConvId) return
              const serverMessages = messages.value.filter((m) => !m.id.startsWith('streaming-'))
              messages.value = serverMessages
              // Only show error if server also has no new content
              const lastMsg = serverMessages[serverMessages.length - 1]
              if (!lastMsg || lastMsg.role !== 'assistant' || !lastMsg.content?.trim()) {
                const errorKey = errorMap[err.message]
                streamError.value = errorKey || err.message
              }
            }).catch(() => {
              const errorKey = errorMap[err.message]
              streamError.value = errorKey || err.message
              messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            })
            return
          }

          const errorKey = errorMap[err.message]
          if (errorKey) {
            streamError.value = errorKey
          } else {
            streamError.value = err.message
          }
          // Log error to server
          systemApi.writeLog('error', `Chat stream error: ${err.message}`, 'chat').catch(() => {})
          // If streaming message has content, keep it and mark as interrupted
          // Otherwise remove the empty placeholder
          const streamingMsg = messages.value.find((m) => m.id.startsWith('streaming-'))
          if (streamingMsg && streamingMsg.content.trim()) {
            const newMessages = [...messages.value]
            const idx = newMessages.indexOf(streamingMsg)
            if (idx >= 0) {
              newMessages[idx] = { ...streamingMsg, content: streamingMsg.content + '\n\n[Response interrupted]' }
              messages.value = newMessages
            }
          } else {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
          }
          streaming.value = false
        },
        onBlocked: (message, threatLevel) => {
          securityBlocked.value = { message, threatLevel }
          // Remove placeholder messages
          messages.value = messages.value.filter(
            (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
          )
        },
        onTrialExhausted: () => {
          trialExhausted.value = true
          streaming.value = false
          sending.value = false
          // Remove placeholder messages
          messages.value = messages.value.filter(
            (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
          )
        },
        onContextTrimmed: (info) => {
          contextTrimInfo.value = info
        },
        onNetworkInterrupt: () => {
          // Mid-stream network interrupt — the backend handles retries with
          // exponential backoff. Nothing to show in UI; the stream will resume
          // transparently when the backend reconnects.
          streamError.value = null
        },
        onComplete: (finalChunk) => {
          streaming.value = false
          toolExecuting.value = false
          // Guard: if user switched away, don't touch messages
          if (currentConversationId.value !== sendConvId) return
          // Store metadata from final chunk directly on the message object
          // This ensures metadata persists even after fetchMessages() refreshes the list
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              // Update the message with metadata inline (this will be visible immediately)
              const newMessages = [...messages.value]
              newMessages[lastIndex] = {
                ...lastMsg,
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              }
              messages.value = newMessages
              // Also store in metadata map using streaming ID as backup
              const msgId = newMessages[lastIndex]?.id
              if (msgId) {
                  messageMetadata.value.set(msgId, {
                  provider: finalChunk.provider,
                  model: finalChunk.model,
                  stats: finalChunk.stats,
                })
              }
            }
          }
          // Refresh messages to get the actual IDs from server
          fetchMessages(conversationId)
          // Refresh conversations to get updated title (auto-generated after first message)
          fetchConversations()
          // Refresh trial quota to update progress bar
          useProviderPoolStore().fetchTrialQuota()
        },
      })
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to send message'
      // Keep streaming message with content, mark as interrupted; remove empty placeholders
      const streamingMsg = messages.value.find((m) => m.id.startsWith('streaming-'))
      if (streamingMsg && streamingMsg.content.trim()) {
        const newMessages = messages.value.filter((m) => !m.id.startsWith('temp-'))
        const idx = newMessages.findIndex((m) => m.id === streamingMsg.id)
        if (idx >= 0) {
          newMessages[idx] = { ...streamingMsg, content: streamingMsg.content + '\n\n[Response interrupted]' }
        }
        messages.value = newMessages
      } else {
        messages.value = messages.value.filter(
          (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
        )
      }
    } finally {
      sending.value = false
      streaming.value = false
      toolExecuting.value = false
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
  }

  function cancelStreaming() {
    sseClient.disconnect()
    streaming.value = false
    toolExecuting.value = false
    streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
  }

  /** Inject a user message into an active stream. The backend cancels the current
   *  stream, persists partial content + new user message, and restarts the LLM call. */
  async function injectMessage(content: string) {
    if (!currentConversationId.value || !streaming.value) return

    const conversationId = currentConversationId.value

    // Add user message to local state immediately
    const userMessage: Message = {
      id: `temp-${Date.now()}`,
      conversation_id: conversationId,
      role: 'user',
      content,
      created_at: new Date().toISOString(),
    }
    messages.value = [...messages.value, userMessage]

    // Call injection API — this will cancel the active stream server-side
    try {
      await injectionApi.inject(conversationId, content)
    } catch (err) {
      console.error('[chat] injection failed, falling back to cancel + send:', err)
      // Fallback: cancel stream and send normally
      cancelStreaming()
      await sendMessage(content)
    }
  }

  // Cancel pre-TTFT: user started typing before first token arrived.
  // Abort the stream, show "listening" indicator, re-warmup, start auto-resume timer.
  function cancelPreTTFT() {
    if (!sending.value || _receivedFirstChunk.value) return // Only works pre-TTFT

    const convId = currentConversationId.value
    if (!convId) return

    // Abort the in-flight stream
    sseClient.disconnect()
    streaming.value = false
    toolExecuting.value = false
    streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    sending.value = false

    // Remove empty assistant placeholder
    messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))

    // Enter "listening" state
    preTTFTCancelActive.value = true

    // Re-warmup for the next send
    warmupConvId = null
    warmupApi.trigger(convId).catch(() => {})

    // Auto-resume after 10s if user doesn't send a follow-up
    if (preTTFTResumeTimer) clearTimeout(preTTFTResumeTimer)
    preTTFTResumeTimer = setTimeout(() => {
      preTTFTResumeTimer = null
      autoResume()
    }, 10000)
  }

  // Auto-resume: re-submit the original request after pre-TTFT cancel timeout.
  async function autoResume() {
    if (!preTTFTCancelActive.value) return
    preTTFTCancelActive.value = false

    const convId = currentConversationId.value
    if (!convId) return

    const settingsStore = useSettingsStore()

    try {
      sending.value = true
      streaming.value = true
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
      _receivedFirstChunk.value = false

      // Add placeholder for assistant message
      const assistantMessage: Message = {
        id: `streaming-${Date.now()}`,
        conversation_id: convId,
        role: 'assistant',
        content: '',
        created_at: new Date().toISOString(),
      }
      messages.value = [...messages.value, assistantMessage]

      const request: SendMessageRequest = {
        message: '[CONTINUE_AFTER_CANCEL]',
        provider: undefined,
        model: undefined,
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
      }

      await sseClient.connect(convId, request, {
        onMessage: (chunk) => {
          if (currentConversationId.value !== convId) return
          if (!chunk.delta) return
          _receivedFirstChunk.value = true
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          streamingContent.value += chunk.delta
          const lastIndex = messages.value.length - 1
          if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
            const newMessages = [...messages.value]
            const currentMsg = newMessages[lastIndex]
            if (currentMsg) {
              newMessages[lastIndex] = { ...currentMsg, content: streamingContent.value }
              messages.value = newMessages
            }
          }
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== convId) return
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== convId) return
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
          const summary = formatToolResultsSummary(results)
          if (summary) {
            streamingContent.value += summary
            processContentLength.value = streamingContent.value.length
            const lastIndex = messages.value.length - 1
            if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
              const newMessages = [...messages.value]
              const currentMsg = newMessages[lastIndex]
              if (currentMsg) {
                newMessages[lastIndex] = { ...currentMsg, content: streamingContent.value }
                messages.value = newMessages
              }
            }
          }
        },
        onNewMessage: () => {
          if (currentConversationId.value !== convId) return
          streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
          toolExecuting.value = false
          const newAssistant: Message = {
            id: `streaming-${Date.now()}`,
            conversation_id: convId,
            role: 'assistant',
            content: '',
            created_at: new Date().toISOString(),
          }
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (_messageId, content) => {
          if (currentConversationId.value !== convId) return
          const idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          if (idx >= 0) {
            const newMessages = [...messages.value]
            newMessages[idx] = { ...newMessages[idx]!, content }
            messages.value = newMessages
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== convId) return
          toolExecuting.value = false
          streamError.value = err.message
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
          streaming.value = false
        },
        onComplete: (finalChunk) => {
          streaming.value = false
          toolExecuting.value = false
          if (currentConversationId.value !== convId) return
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              const newMessages = [...messages.value]
              newMessages[lastIndex] = {
                ...lastMsg,
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              }
              messages.value = newMessages
            }
          }
          fetchMessages(convId)
          fetchConversations()
          useProviderPoolStore().fetchTrialQuota()
        },
      })
    } catch {
      messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
    } finally {
      sending.value = false
      streaming.value = false
      toolExecuting.value = false
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
  }

  // Continue generating from where it stopped
  async function continueMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()

    // Get the last assistant message content to continue from
    const lastMessage = messages.value[messages.value.length - 1]
    if (!lastMessage || lastMessage.role !== 'assistant') return

    const existingContent = lastMessage.content

    try {
      sending.value = true
      streaming.value = true
      streamingContent.value = existingContent // Start with existing content
      error.value = null

      const request: SendMessageRequest = {
        message: '[CONTINUE]', // Special marker for continue
        provider: undefined,
        model: undefined,
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
      }

      await sseClient.connect(conversationId, request, {
        onMessage: (chunk) => {
          if (currentConversationId.value !== conversationId) return
          if (!chunk.delta) return
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          streamingContent.value += chunk.delta
          // Update the last message
          const lastIndex = messages.value.length - 1
          if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
            const newMessages = [...messages.value]
            const currentMsg = newMessages[lastIndex]
            if (currentMsg) {
              newMessages[lastIndex] = {
                ...currentMsg,
                content: streamingContent.value,
              }
              messages.value = newMessages
            }
          }
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== conversationId) return
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== conversationId) return
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
          const summary = formatToolResultsSummary(results)
          if (summary) {
            streamingContent.value += summary
            processContentLength.value = streamingContent.value.length
            const lastIndex = messages.value.length - 1
            if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
              const newMessages = [...messages.value]
              const currentMsg = newMessages[lastIndex]
              if (currentMsg) {
                newMessages[lastIndex] = { ...currentMsg, content: streamingContent.value }
                messages.value = newMessages
              }
            }
          }
        },
        onNewMessage: () => {
          if (currentConversationId.value !== conversationId) return
          streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
          toolExecuting.value = false
          const newAssistant: Message = {
            id: `streaming-${Date.now()}`,
            conversation_id: conversationId,
            role: 'assistant',
            content: '',
            created_at: new Date().toISOString(),
          }
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (_messageId, content) => {
          if (currentConversationId.value !== conversationId) return
          const idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          if (idx >= 0) {
            const newMessages = [...messages.value]
            newMessages[idx] = { ...newMessages[idx]!, content }
            messages.value = newMessages
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== conversationId) return
          error.value = err.message
          streaming.value = false
          toolExecuting.value = false
        },
        onBlocked: (message, threatLevel) => {
          securityBlocked.value = { message, threatLevel }
          streaming.value = false
          toolExecuting.value = false
        },
        onTrialExhausted: () => {
          trialExhausted.value = true
          streaming.value = false
          toolExecuting.value = false
          sending.value = false
        },
        onComplete: (finalChunk) => {
          streaming.value = false
          toolExecuting.value = false
          if (currentConversationId.value !== conversationId) return
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              const newMessages = [...messages.value]
              newMessages[lastIndex] = {
                ...lastMsg,
                content: streamingContent.value,
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              }
              messages.value = newMessages
            }
          }
          fetchMessages(conversationId)
          // Refresh trial quota to update progress bar
          useProviderPoolStore().fetchTrialQuota()
        },
      })
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to continue message'
    } finally {
      sending.value = false
      streaming.value = false
      toolExecuting.value = false
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
  }

  // Regenerate the last assistant message
  async function regenerateMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()

    // Find the last user message to regenerate from
    let lastUserMessageIndex = -1
    for (let i = messages.value.length - 1; i >= 0; i--) {
      if (messages.value[i]?.role === 'user') {
        lastUserMessageIndex = i
        break
      }
    }

    if (lastUserMessageIndex === -1) return

    const lastUserMessage = messages.value[lastUserMessageIndex]
    if (!lastUserMessage) return

    // Remove the last assistant message if it exists
    const lastMessage = messages.value[messages.value.length - 1]
    if (lastMessage?.role === 'assistant') {
      messages.value = messages.value.slice(0, -1)
    }

    try {
      sending.value = true
      streaming.value = true
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
      error.value = null

      // Add placeholder for new assistant message
      const assistantMessage: Message = {
        id: `streaming-${Date.now()}`,
        conversation_id: conversationId,
        role: 'assistant',
        content: '',
        created_at: new Date().toISOString(),
      }
      messages.value = [...messages.value, assistantMessage]

      const request: SendMessageRequest = {
        message: lastUserMessage.content,
        provider: undefined,
        model: undefined,
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        attachments: lastUserMessage.attachments,
        regenerate: true,
      }

      await sseClient.connect(conversationId, request, {
        onMessage: (chunk) => {
          if (currentConversationId.value !== conversationId) return
          if (!chunk.delta) return
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          streamingContent.value += chunk.delta
          const lastIndex = messages.value.length - 1
          if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
            const newMessages = [...messages.value]
            const currentMsg = newMessages[lastIndex]
            if (currentMsg) {
              newMessages[lastIndex] = {
                ...currentMsg,
                content: streamingContent.value,
              }
              messages.value = newMessages
            }
          }
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== conversationId) return
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== conversationId) return
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
          const summary = formatToolResultsSummary(results)
          if (summary) {
            streamingContent.value += summary
            processContentLength.value = streamingContent.value.length
            const lastIndex = messages.value.length - 1
            if (lastIndex >= 0 && messages.value[lastIndex]?.role === 'assistant') {
              const newMessages = [...messages.value]
              const currentMsg = newMessages[lastIndex]
              if (currentMsg) {
                newMessages[lastIndex] = { ...currentMsg, content: streamingContent.value }
                messages.value = newMessages
              }
            }
          }
        },
        onNewMessage: () => {
          if (currentConversationId.value !== conversationId) return
          streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
          toolExecuting.value = false
          const newAssistant: Message = {
            id: `streaming-${Date.now()}`,
            conversation_id: conversationId,
            role: 'assistant',
            content: '',
            created_at: new Date().toISOString(),
          }
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (_messageId, content) => {
          if (currentConversationId.value !== conversationId) return
          const idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          if (idx >= 0) {
            const newMessages = [...messages.value]
            newMessages[idx] = { ...newMessages[idx]!, content }
            messages.value = newMessages
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== conversationId) return
          error.value = err.message
          const wasToolExecuting = toolExecuting.value
          toolExecuting.value = false
          // If error happened during tool execution, fetch server-persisted content
          if (wasToolExecuting) {
            fetchMessages(conversationId).then(() => {
              if (currentConversationId.value !== conversationId) return
              messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            })
            return
          }
          // Keep message with content, mark as interrupted
          const streamingMsg = messages.value.find((m) => m.id.startsWith('streaming-'))
          if (streamingMsg && streamingMsg.content.trim()) {
            const newMessages = [...messages.value]
            const idx = newMessages.indexOf(streamingMsg)
            if (idx >= 0) {
              newMessages[idx] = { ...streamingMsg, content: streamingMsg.content + '\n\n[Response interrupted]' }
              messages.value = newMessages
            }
          } else {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
          }
        },
        onBlocked: (message, threatLevel) => {
          securityBlocked.value = { message, threatLevel }
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
        },
        onTrialExhausted: () => {
          trialExhausted.value = true
          streaming.value = false
          toolExecuting.value = false
          sending.value = false
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
        },
        onComplete: (finalChunk) => {
          streaming.value = false
          toolExecuting.value = false
          if (currentConversationId.value !== conversationId) return
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              const newMessages = [...messages.value]
              newMessages[lastIndex] = {
                ...lastMsg,
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              }
              messages.value = newMessages
            }
          }
          fetchMessages(conversationId)
          // Refresh trial quota to update progress bar
          useProviderPoolStore().fetchTrialQuota()
        },
      })
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to regenerate message'
      const streamingMsg = messages.value.find((m) => m.id.startsWith('streaming-'))
      if (streamingMsg && streamingMsg.content.trim()) {
        const newMessages = [...messages.value]
        const idx = newMessages.indexOf(streamingMsg)
        if (idx >= 0) {
          newMessages[idx] = { ...streamingMsg, content: streamingMsg.content + '\n\n[Response interrupted]' }
          messages.value = newMessages
        }
      } else {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      }
    } finally {
      sending.value = false
      streaming.value = false
      toolExecuting.value = false
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
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

  function clearStreamError() {
    streamError.value = null
  }

  function clearSecurityBlocked() {
    securityBlocked.value = null
  }

  function clearTrialExhausted() {
    trialExhausted.value = false
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

  async function resolveApproval(decision: Decision, alwaysAllow = false) {
    if (!pendingApproval.value) return
    const toolName = pendingApproval.value.tool_name
    try {
      await approvalApi.resolve(pendingApproval.value.request_id, decision)
      // If "Always Allow", set this tool's policy to auto
      if (alwaysAllow && toolName) {
        try {
          const configRes = await approvalApi.getConfig()
          const config = configRes.data
          config.tool_policies[toolName] = 'auto'
          await approvalApi.updateConfig(config)
        } catch (e) {
          console.error('Failed to update tool policy:', e)
        }
      }
    } finally {
      pendingApproval.value = null
    }
  }

  async function checkPendingApprovals() {
    try {
      const response = await approvalApi.listPending()
      const pending = response.data
      if (pending && pending.length > 0) {
        const first = pending[0]
        if (first) {
          pendingApproval.value = {
            request_id: first.id,
            tool_name: first.tool_name,
            tool_call_id: first.tool_call_id,
            arguments: first.arguments,
          }
        }
      }
    } catch {
      // Approval endpoint may not exist yet — ignore
    }
  }

  // --- Ask-user-question methods ---
  function setPendingQuestion(data: any) {
    console.log('[ChatStore] setPendingQuestion called with:', data)
    pendingQuestion.value = data
    console.log('[ChatStore] pendingQuestion.value is now:', pendingQuestion.value)
  }

  async function submitQuestionAnswers(answers: Array<{ question_id: string; selected: string[]; other_text?: string }>) {
    if (!pendingQuestion.value) return
    const id = pendingQuestion.value.id
    try {
      await api.post(`/ask-user-question/${id}/answer`, { answers })
      pendingQuestion.value = null
    } catch (e) {
      console.error('Failed to submit question answers:', e)
      // Keep dialog open so the user can retry
    }
  }

  async function dismissQuestion() {
    if (!pendingQuestion.value) {
      return
    }
    const id = pendingQuestion.value.id
    pendingQuestion.value = null
    // Notify backend to unblock the tool call immediately with default answers
    try {
      await api.post(`/ask-user-question/${id}/dismiss`)
    } catch {
      // Best-effort — question will timeout on backend if this fails
    }
  }

  async function checkPendingQuestion() {
    try {
      const res = await api.get<{ pending: boolean; question?: any }>('/ask-user-question/pending')
      if (res.data.pending && res.data.question) {
        pendingQuestion.value = res.data.question
      }
    } catch {
      // endpoint may not exist — ignore
    }
  }

  // --- Exec approval methods ---
  function setPendingExecApproval(data: any) {
    pendingExecApproval.value = data
  }

  async function resolveExecApproval(decision: ExecDecision) {
    if (!pendingExecApproval.value) return
    try {
      await approvalApi.resolve(pendingExecApproval.value.id, decision)
    } catch (e) {
      console.error('Failed to resolve exec approval:', e)
    } finally {
      pendingExecApproval.value = null
    }
  }

  function dismissExecApproval() {
    pendingExecApproval.value = null
  }

  async function clearAllConversations() {
    try {
      // Delete all conversations one by one
      const ids = conversations.value.map(c => c.id)
      for (const id of ids) {
        await conversationApi.delete(id)
      }
      conversations.value = []
      currentConversationId.value = null
      messages.value = []
      hasMoreMessages.value = false
      currentPage.value = 0
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to clear conversations'
      throw e
    }
  }

  // Warmup: pre-compute system prompt and context to reduce TTFT.
  // Fire-and-forget — errors are silently ignored.
  let warmupConvId: string | null = null
  function warmupConversation() {
    const convId = currentConversationId.value
    if (!convId || convId === warmupConvId) return
    warmupConvId = convId
    warmupApi.trigger(convId).catch(() => {})
  }

  // Reset warmup tracking (call when conversation changes)
  function resetWarmup() {
    warmupConvId = null
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
    processContentLength,
    error,
    streamError,
    securityBlocked,
    trialExhausted,
    toolExecuting,
    toolExecutingStartTime,
    toolExecutingNames,
    toolExecutingCommands,
    toolSandboxAvailable,
    toolResults,
    contextTrimInfo,
    preTTFTCancelActive,
    hasMoreMessages,
    loadingMore,
    searchQuery,
    searching,
    selectedMessageIds,
    isMultiSelectMode,
    pendingApproval,
    pendingQuestion,
    pendingExecApproval,

    // Computed
    currentConversation,
    sortedConversations,
    isPreTTFT,

    // Actions
    fetchConversations,
    createConversation,
    deleteConversation,
    pinConversation,
    unpinConversation,
    selectConversation,
    fetchMessages,
    loadMoreMessages,
    sendMessage,
    injectMessage,
    cancelStreaming,
    cancelPreTTFT,
    continueMessage,
    regenerateMessage,
    searchConversations,
    clearSearch,
    clearError,
    clearStreamError,
    clearSecurityBlocked,
    clearTrialExhausted,
    getMessageMetadata,
    toggleMessageSelection,
    selectMessage,
    deselectMessage,
    clearSelection,
    enterMultiSelectMode,
    exitMultiSelectMode,
    deleteSelectedMessages,
    clearAllConversations,
    resolveApproval,
    checkPendingApprovals,
    setPendingQuestion,
    submitQuestionAnswers,
    dismissQuestion,
    checkPendingQuestion,
    setPendingExecApproval,
    resolveExecApproval,
    dismissExecApproval,
    warmupConversation,
    resetWarmup,
  }
})
