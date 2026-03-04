import { defineStore } from 'pinia'
import { ref, shallowRef, computed, watch, triggerRef } from 'vue'
import type { Conversation, Message, SendMessageRequest, MessageStats, MessageAttachment } from '@/api/chat'
import { conversationApi, messageApi, warmupApi, injectionApi } from '@/api/chat'
import { approvalApi } from '@/api/approval'
import type { Decision, ExecDecision } from '@/api/approval'
import api from '@/api/client'
import { SSEClient } from '@/utils/sse'
import { i18n } from '@/i18n'
import { useSettingsStore } from './settings'
import { useProviderPoolStore } from './providerPool'
import { systemApi } from '@/api/system'

const PAGE_SIZE = 50
const CHAT_MODEL_PREF_KEY = 'chat.modelPreference'
const CHAT_OFFLINE_MODE_KEY = 'chat.offlineMode'
const CHAT_WEB_SEARCH_ENABLED_KEY = 'chat.webSearchEnabled'
const CHAT_DEEP_RESEARCH_ENABLED_KEY = 'chat.deepResearchEnabled'

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
          icon = '✗'; status = String(res.error)
        } else if (res.exit_code !== undefined) {
          icon = res.exit_code === 0 ? '✓' : '✗'
          exitCode = res.exit_code
          durationMs = res.duration_ms
          status = res.duration_ms ? `${res.duration_ms}ms` : ''
        } else if (res.status) {
          const statusText = String(res.status)
          status = statusText
          icon = /(error|fail)/i.test(statusText) ? '✗' : '✓'
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
        const text = r.result.slice(0, 500)
        if (/(error|failed|unsupported|not support|invalid)/i.test(text)) {
          icon = '✗'
          status = text
        } else {
          output = text
          icon = '✓'
        }
      }
    }
    return { name: r.name, id: r.id, command, args: r.args, icon, status, output, exitCode, durationMs, host, riskLevel, timestamp: Date.now() }
  })
}

function formatStreamProgress(stage: string): string {
  const t = i18n.global.t
  const te = i18n.global.te
  const resolve = (key: string, fallback: string): string => {
    return te(key) ? t(key) : fallback
  }

  switch (stage) {
    case 'response.created':
      return resolve('chat.streamProgress.requestAccepted', 'Request received, preparing response...')
    case 'response.in_progress':
      return resolve('chat.streamProgress.generating', 'Generating response...')
    case 'response.output_text.delta':
      return resolve('chat.streamProgress.writing', 'Writing response...')
    case 'response.output_item.added':
    case 'response.output_item.done':
    case 'response.function_call_arguments.delta':
    case 'response.function_call_arguments.done':
      return resolve('chat.streamProgress.processingTools', 'Processing request...')
    case 'response.completed':
      return resolve('chat.streamProgress.completed', 'Done')
    default:
      return resolve('chat.streamProgress.processing', 'Processing...')
  }
}

// Store for message metadata (provider, model, stats) - keyed by message ID
const messageMetadata = ref<Map<string, { provider?: string; model?: string; stats?: MessageStats }>>(new Map())

export const useChatStore = defineStore('chat', () => {
  const loadModelPreference = (): string => {
    try {
      const value = localStorage.getItem(CHAT_MODEL_PREF_KEY)?.trim()
      if (!value) return 'auto'
      return value
    } catch {
      return 'auto'
    }
  }

  const loadOfflineMode = (): boolean => {
    try {
      return localStorage.getItem(CHAT_OFFLINE_MODE_KEY) === '1'
    } catch {
      return false
    }
  }

  const saveModelPreference = (value: string) => {
    try {
      localStorage.setItem(CHAT_MODEL_PREF_KEY, value)
    } catch {
      // ignore storage errors
    }
  }

  const saveOfflineMode = (enabled: boolean) => {
    try {
      localStorage.setItem(CHAT_OFFLINE_MODE_KEY, enabled ? '1' : '0')
    } catch {
      // ignore storage errors
    }
  }

  const loadWebSearchEnabled = (): boolean => {
    try {
      const value = localStorage.getItem(CHAT_WEB_SEARCH_ENABLED_KEY)
      if (value === null) return true
      return value !== '0'
    } catch {
      return true
    }
  }

  const saveWebSearchEnabled = (enabled: boolean) => {
    try {
      localStorage.setItem(CHAT_WEB_SEARCH_ENABLED_KEY, enabled ? '1' : '0')
    } catch {
      // ignore storage errors
    }
  }

  const loadDeepResearchEnabled = (): boolean => {
    try {
      const value = localStorage.getItem(CHAT_DEEP_RESEARCH_ENABLED_KEY)
      if (value === null) return false
      return value !== '0'
    } catch {
      return false
    }
  }

  const saveDeepResearchEnabled = (enabled: boolean) => {
    try {
      localStorage.setItem(CHAT_DEEP_RESEARCH_ENABLED_KEY, enabled ? '1' : '0')
    } catch {
      // ignore storage errors
    }
  }

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
  const streamProgress = ref<string | null>(null) // Upstream metadata progress before first visible delta
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
      detail?: string
      header: string
      options?: Array<{ label: string; description?: string; value?: string }>
      multi_select?: boolean
    }>
    context?: {
      kind?: string
      checkpoint_id?: string
      required?: boolean
      risk_level?: 'low' | 'high'
      step?: string
      action?: string
      url?: string
      screenshot?: { mime_type?: string; data?: string; url?: string }
    }
    expires_at: number
  } | null>(null)
  const awaitingConfirmation = ref(false)

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
  const modelPreference = ref<string>(loadModelPreference())
  const offlineMode = ref<boolean>(loadOfflineMode())
  const webSearchEnabled = ref<boolean>(loadWebSearchEnabled())
  const deepResearchEnabled = ref<boolean>(loadDeepResearchEnabled())
  const activeStreamId = ref<string | null>(null)

  // SSE client for streaming
  const sseClient = new SSEClient()

  function rememberActiveStreamId(streamId?: string | null) {
    const next = streamId?.trim()
    if (!next) return
    activeStreamId.value = next
  }

  async function cancelActiveStreamOnServer(conversationId?: string | null) {
    const streamId = activeStreamId.value
    if (!conversationId || !streamId) return
    try {
      await messageApi.cancelStream(conversationId, streamId)
    } catch {
      // Best-effort cancellation.
    }
  }

  // Buffer incoming deltas and commit at most once per animation frame.
  // This cuts down message array churn and expensive markdown/card re-parsing.
  let pendingStreamDelta = ''
  let pendingStreamConversationId: string | null = null
  let streamCommitRaf: number | null = null
  let streamCommitTimer: ReturnType<typeof setTimeout> | null = null

  function cancelPendingStreamCommit() {
    if (streamCommitRaf !== null && typeof window !== 'undefined') {
      window.cancelAnimationFrame(streamCommitRaf)
      streamCommitRaf = null
    }
    if (streamCommitTimer) {
      clearTimeout(streamCommitTimer)
      streamCommitTimer = null
    }
  }

  function patchLastAssistantMessageContent(expectedConversationId?: string | null) {
    if (expectedConversationId && currentConversationId.value !== expectedConversationId) return
    const lastIndex = messages.value.length - 1
    if (lastIndex < 0) return
    const lastMsg = messages.value[lastIndex]
    if (!lastMsg || lastMsg.role !== 'assistant') return
    if (lastMsg.content === streamingContent.value) return
    lastMsg.content = streamingContent.value
    triggerRef(messages)
  }

  function flushPendingStreamDelta(expectedConversationId?: string | null) {
    cancelPendingStreamCommit()
    if (expectedConversationId && pendingStreamConversationId && expectedConversationId !== pendingStreamConversationId) {
      pendingStreamDelta = ''
      pendingStreamConversationId = null
      return
    }
    const targetConversationId = expectedConversationId ?? pendingStreamConversationId
    if (pendingStreamDelta) {
      streamingContent.value += pendingStreamDelta
      pendingStreamDelta = ''
    }
    patchLastAssistantMessageContent(targetConversationId)
    pendingStreamConversationId = null
  }

  function schedulePendingStreamCommit() {
    if (streamCommitRaf !== null || streamCommitTimer) return
    const commit = () => {
      streamCommitRaf = null
      streamCommitTimer = null
      flushPendingStreamDelta()
    }
    if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
      streamCommitRaf = window.requestAnimationFrame(commit)
      return
    }
    streamCommitTimer = setTimeout(commit, 16)
  }

  function enqueueStreamDelta(conversationId: string, delta: string) {
    if (!delta) return
    if (pendingStreamConversationId && pendingStreamConversationId !== conversationId) {
      flushPendingStreamDelta(pendingStreamConversationId)
    }
    pendingStreamConversationId = conversationId
    pendingStreamDelta += delta
    schedulePendingStreamCommit()
  }

  function resetPendingStreamDelta() {
    cancelPendingStreamCommit()
    pendingStreamDelta = ''
    pendingStreamConversationId = null
  }

  watch(pendingQuestion, (q) => {
    if (q) {
      awaitingConfirmation.value = true
      return
    }
    if (!streaming.value) {
      awaitingConfirmation.value = false
    }
  })

  function hasPendingConfirmations(): boolean {
    return !!pendingQuestion.value || !!pendingApproval.value || !!pendingExecApproval.value
  }

  function extractInlineQuestionLabel(raw: string): string {
    let text = raw
      .replace(/^[-*•]\s+/, '')
      .replace(/^\d+[.)]\s+/, '')
      .replace(/\*\*/g, '')
      .replace(/`/g, '')
      .trim()
    const colonIndex = Math.max(text.indexOf(':'), text.indexOf('：'))
    if (colonIndex > 0) {
      text = text.slice(0, colonIndex).trim()
    }
    return text
  }

  function inferInlinePendingQuestionFromAssistantMessage(): boolean {
    if (hasPendingConfirmations()) return true

    let content = ''
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const m = messages.value[i]
      if (m?.role === 'assistant' && m.content?.trim()) {
        content = m.content
        break
      }
    }
    if (!content) return false

    const lines = content.split('\n').map(line => line.trim()).filter(Boolean)
    if (lines.length === 0) return false

    const question = lines.find(line =>
      /[?？]\s*$/.test(line) &&
      !/^[-*•]\s+/.test(line) &&
      !/^\d+[.)]\s+/.test(line),
    )
    if (!question) return false

    const optionLines = lines.filter(line => /^[-*•]\s+/.test(line) || /^\d+[.)]\s+/.test(line))
    if (optionLines.length < 2 || optionLines.length > 8) return false

    const seen = new Set<string>()
    const options = optionLines
      .map(extractInlineQuestionLabel)
      .filter(label => {
        if (!label) return false
        const key = label.toLowerCase()
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
      .map(label => ({ label, value: label }))

    if (options.length < 2) return false

    pendingQuestion.value = {
      id: `inline:${Date.now()}`,
      questions: [{
        id: 'q1',
        question: question.replace(/\*\*/g, '').trim(),
        header: 'Question',
        options,
        multi_select: false,
      }],
      expires_at: Date.now() + 10 * 60 * 1000,
    }
    awaitingConfirmation.value = true
    return true
  }

  async function recoverPendingConfirmations(allowInlineFallback = false): Promise<void> {
    await Promise.all([
      checkPendingApprovals(),
      checkPendingQuestion(),
      checkPendingExecApproval(),
    ])
    if (hasPendingConfirmations()) {
      awaitingConfirmation.value = true
      return
    }
    if (allowInlineFallback && inferInlinePendingQuestionFromAssistantMessage()) {
      return
    }
    if (!streaming.value) {
      awaitingConfirmation.value = false
    }
  }

  function markAwaitingConfirmation() {
    const wasAwaiting = awaitingConfirmation.value
    awaitingConfirmation.value = true
    // If the out-of-band approval event was missed, recover pending payloads.
    if (!wasAwaiting) {
      void recoverPendingConfirmations(false)
    }
  }

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

  function stopActiveStreamForConversationSwitch() {
    if (!streaming.value && !sending.value) return
    flushPendingStreamDelta(currentConversationId.value)
    // Conversation switch should only detach local streaming UI.
    // Do not cancel server-side generation; let it finish in background.
    sseClient.disconnect()
    streaming.value = false
    sending.value = false
    toolExecuting.value = false
    activeStreamId.value = null
    resetPendingStreamDelta()
    streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
  }

  async function createConversation(title?: string) {
    // Clicking "new chat" should immediately leave any active stream and
    // unlock the input in the new conversation.
    stopActiveStreamForConversationSwitch()
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

    // Prevent old conversation stream callbacks from writing into the newly
    // selected conversation.
    stopActiveStreamForConversationSwitch()

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

  function touchConversationLocal(conversationId: string) {
    const idx = conversations.value.findIndex((c) => c.id === conversationId)
    if (idx < 0) return
    const now = new Date().toISOString()
    const updated = { ...conversations.value[idx]!, updated_at: now }
    const next = [...conversations.value]
    next[idx] = updated
    conversations.value = next
  }

  function appendAssistantLocalMessage(conversationId: string, content: string) {
    const assistantMessage: Message = {
      id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      conversation_id: conversationId,
      role: 'assistant',
      content,
      created_at: new Date().toISOString(),
      provider: 'local',
      model: offlineMode.value ? 'offline' : undefined,
    }
    messages.value = [...messages.value, assistantMessage]
    touchConversationLocal(conversationId)
  }

  async function handleSlashCommand(conversationId: string, raw: string): Promise<boolean> {
    const text = raw.trim()
    if (!text.startsWith('/')) return false

    const parts = text.slice(1).trim().split(/\s+/)
    const cmd = (parts[0] || '').toLowerCase()
    const args = parts.slice(1)

    if (!cmd) return false

    const userMessage: Message = {
      id: `temp-${Date.now()}`,
      conversation_id: conversationId,
      role: 'user',
      content: raw,
      created_at: new Date().toISOString(),
    }
    messages.value = [...messages.value, userMessage]

    if (cmd === 'help' || cmd === 'commands') {
      appendAssistantLocalMessage(
        conversationId,
        [
          '可用命令：',
          '`/commands` 同 `/help`',
          '`/status` 查看当前会话命令状态',
          '`/model` 查看当前模型偏好',
          '`/model auto` 使用自动路由',
          '`/model <模型ID>` 固定模型',
          '`/model list` 或 `/models` 列出可用模型',
          '`/offline on|off|status` 切换或查看离线模式',
          '`/clear` 或 `/reset` 清空当前会话消息',
        ].join('\n')
      )
      return true
    }

    if (cmd === 'status') {
      const status = [
        '会话状态：',
        `- model: \`${modelPreference.value}\``,
        `- offline: \`${offlineMode.value ? 'ON' : 'OFF'}\``,
        `- messages: \`${messages.value.length}\``,
      ].join('\n')
      appendAssistantLocalMessage(conversationId, status)
      return true
    }

    if (cmd === 'model' || cmd === 'models') {
      if (cmd === 'models' && args.length === 0) {
        args.push('list')
      }
      const sub = (args[0] || '').trim()
      if (!sub) {
        appendAssistantLocalMessage(conversationId, `当前模型偏好：\`${modelPreference.value}\``)
        return true
      }
      if (sub.toLowerCase() === 'list') {
        const providerStore = useProviderPoolStore()
        if (providerStore.models.length === 0) {
          try {
            await providerStore.fetchModels()
          } catch {
            // ignore
          }
        }
        const all = Array.from(new Set(providerStore.models.map((m) => m.id))).sort()
        if (all.length === 0) {
          appendAssistantLocalMessage(conversationId, '当前没有可用模型列表，请先在 Provider 配置页完成模型拉取。')
          return true
        }
        const preview = all.slice(0, 30).map((m) => `- \`${m}\``).join('\n')
        const suffix = all.length > 30 ? `\n... 共 ${all.length} 个模型` : ''
        appendAssistantLocalMessage(conversationId, `可用模型：\n${preview}${suffix}`)
        return true
      }

      const nextModel = sub.toLowerCase() === 'auto' ? 'auto' : sub
      modelPreference.value = nextModel
      saveModelPreference(nextModel)
      appendAssistantLocalMessage(conversationId, `模型偏好已设置为：\`${nextModel}\``)
      return true
    }

    if (cmd === 'offline') {
      const sub = (args[0] || 'status').toLowerCase()
      if (sub === 'on') {
        offlineMode.value = true
        saveOfflineMode(true)
        appendAssistantLocalMessage(conversationId, '离线模式已开启。后续消息不会调用模型，只返回本地离线响应。')
        return true
      }
      if (sub === 'off') {
        offlineMode.value = false
        saveOfflineMode(false)
        appendAssistantLocalMessage(conversationId, '离线模式已关闭。后续消息将恢复模型调用。')
        return true
      }
      appendAssistantLocalMessage(conversationId, `离线模式当前为：${offlineMode.value ? 'ON' : 'OFF'}`)
      return true
    }

    if (cmd === 'clear' || cmd === 'reset') {
      try {
        const idsToDelete = messages.value
          .map(m => m.id)
          .filter(id => !id.startsWith('temp-') && !id.startsWith('local-') && !id.startsWith('streaming-'))
        if (idsToDelete.length > 0) {
          await messageApi.delete(conversationId, idsToDelete)
        }
        messages.value = [userMessage]
        appendAssistantLocalMessage(conversationId, `已清空当前会话（删除 ${idsToDelete.length} 条消息）。`)
      } catch {
        appendAssistantLocalMessage(conversationId, '清空会话失败，请稍后重试。')
      }
      return true
    }

    // Let backend handle commands not implemented in local fast-path.
    return false
  }

  async function sendMessage(content: string, fileAttachments?: { id: string; file: File; name: string; size: number; type: string; preview?: string; duration?: number }[]) {
    if (!currentConversationId.value) {
      // Use the first part of the message as the conversation title
      const title = content.length > 30 ? content.substring(0, 30) + '...' : content
      await createConversation(title)
    }

    const conversationId = currentConversationId.value!
    const settingsStore = useSettingsStore()

    if ((!fileAttachments || fileAttachments.length === 0) && await handleSlashCommand(conversationId, content)) {
      return
    }

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

    if (offlineMode.value) {
      appendAssistantLocalMessage(
        conversationId,
        `离线模式响应：已收到你的消息（${content.length} 字）。该模式不调用模型推理，仅做本地命令与占位回复。`
      )
      return
    }

    // Keep provider empty so backend router can choose provider.
    // Model can be overridden by `/model` command; empty string means auto.
    const request: SendMessageRequest = {
      message: content,
      provider: '',
      model: modelPreference.value === 'auto' ? '' : modelPreference.value,
      temperature: settingsStore.temperature,
      max_tokens: settingsStore.maxTokens,
      attachments: attachments.length > 0 ? attachments : undefined,
      web_search_enabled: webSearchEnabled.value,
      deep_research_enabled: deepResearchEnabled.value,
    }

    try {
      sending.value = true
      streaming.value = true
      streamProgress.value = null
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
      awaitingConfirmation.value = false
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
        onStreamId: (streamId) => {
          if (currentConversationId.value !== sendConvId) return
          rememberActiveStreamId(streamId)
        },
        onStreamProgress: (progress) => {
          if (currentConversationId.value !== sendConvId) return
          if (!_receivedFirstChunk.value) {
            streamProgress.value = formatStreamProgress(progress)
          }
        },
        onMessage: (chunk) => {
          // Guard: ignore chunks if user switched to a different conversation
          if (currentConversationId.value !== sendConvId) return
          if (chunk.awaiting_user_input) {
            markAwaitingConfirmation()
          }
          if (!chunk.delta) return
          _receivedFirstChunk.value = true
          streamProgress.value = null
          // Clear tool executing state when new content arrives
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          enqueueStreamDelta(sendConvId, chunk.delta)
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== sendConvId) return
          flushPendingStreamDelta(sendConvId)
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== sendConvId) return
          // Store structured tool results for detail cards
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
        },
        onNewMessage: () => {
          if (currentConversationId.value !== sendConvId) return
          flushPendingStreamDelta(sendConvId)
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
        onTodoUpdated: (messageId, content) => {
          if (currentConversationId.value !== sendConvId) return
          // Backend updated TODO list — prefer exact message id, fallback to first checklist bubble.
          let idx = -1
          if (messageId) {
            idx = messages.value.findIndex(m => m.id === messageId)
          }
          if (idx < 0) {
            idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          }
          if (idx >= 0) {
            const msg = messages.value[idx]
            if (msg && msg.content !== content) {
              msg.content = content
              triggerRef(messages)
            }
          }
        },
        onInjection: () => {
          if (currentConversationId.value !== sendConvId) return
          flushPendingStreamDelta(sendConvId)
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
          flushPendingStreamDelta(sendConvId)
          streamProgress.value = null
          const wasToolExecuting = toolExecuting.value
          toolExecuting.value = false
          // Map error codes to i18n keys for accurate error messages.
          // Supports exact matches and "contains" matching for enriched HTTP errors.
          const errorMap: Record<string, string> = {
            'STREAM_EMPTY': 'streamEmpty',
            'STREAM_ERROR': 'streamError',
            'PROVIDER_NO_RESPONSE': 'providerNoResponse',
            'PROVIDER_RETURNED_EMPTY': 'providerReturnedEmpty',
            'No response body': 'noResponseBody',
            'provider_tool_unsupported': 'provider_tool_unsupported',
            'provider_unavailable': 'provider_unavailable',
            'provider_auth_error': 'provider_auth_error',
            'provider_rate_limited': 'provider_rate_limited',
            'provider_openrouter_privacy_policy': 'provider_openrouter_privacy_policy',
            'trial_service_busy': 'trial_service_busy',
          }
          const resolveErrorKey = (message: string): string | undefined => {
            if (errorMap[message]) return errorMap[message]
            const lower = message.toLowerCase()
            if (lower.includes('provider_openrouter_privacy_policy')) return 'provider_openrouter_privacy_policy'
            if (lower.includes('no endpoints found matching your data policy') && lower.includes('free model publication')) return 'provider_openrouter_privacy_policy'
            if (lower.includes('provider_tool_unsupported')) return 'provider_tool_unsupported'
            if (lower.includes('provider_unavailable') || lower.includes('no available provider')) return 'provider_unavailable'
            if (lower.includes('provider_auth_error') || lower.includes('auth error')) return 'provider_auth_error'
            if (lower.includes('provider_rate_limited') || lower.includes('429') || lower.includes('throttled')) return 'provider_rate_limited'
            if (lower.includes('trial_service_busy')) return 'trial_service_busy'
            return undefined
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
                const errorKey = resolveErrorKey(err.message)
                streamError.value = errorKey || err.message
              }
            }).catch(() => {
              const errorKey = resolveErrorKey(err.message)
              streamError.value = errorKey || err.message
              messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            })
            return
          }

          const errorKey = resolveErrorKey(err.message)
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
          flushPendingStreamDelta(sendConvId)
          streaming.value = false
          streamProgress.value = null
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
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
              // Also store in metadata map using streaming ID as backup
              const msgId = lastMsg.id
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
          if (awaitingConfirmation.value) {
            void recoverPendingConfirmations(true)
          }
        },
      })
    } catch (e) {
      flushPendingStreamDelta(conversationId)
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
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
  }

  function cancelStreaming() {
    flushPendingStreamDelta(currentConversationId.value)
    void cancelActiveStreamOnServer(currentConversationId.value)
    sseClient.disconnect()
    streaming.value = false
    toolExecuting.value = false
    activeStreamId.value = null
    resetPendingStreamDelta()
    streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    if (!hasPendingConfirmations()) {
      awaitingConfirmation.value = false
    }
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
    void cancelActiveStreamOnServer(convId)
    sseClient.disconnect()
    streaming.value = false
    toolExecuting.value = false
    activeStreamId.value = null
    resetPendingStreamDelta()
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
      streamProgress.value = null
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
      awaitingConfirmation.value = false
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
        provider: '',
        model: modelPreference.value === 'auto' ? '' : modelPreference.value,
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }

      await sseClient.connect(convId, request, {
        onStreamId: (streamId) => {
          if (currentConversationId.value !== convId) return
          rememberActiveStreamId(streamId)
        },
        onStreamProgress: (progress) => {
          if (currentConversationId.value !== convId) return
          if (!_receivedFirstChunk.value) {
            streamProgress.value = formatStreamProgress(progress)
          }
        },
        onMessage: (chunk) => {
          if (currentConversationId.value !== convId) return
          if (chunk.awaiting_user_input) {
            markAwaitingConfirmation()
          }
          if (!chunk.delta) return
          _receivedFirstChunk.value = true
          streamProgress.value = null
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          enqueueStreamDelta(convId, chunk.delta)
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== convId) return
          flushPendingStreamDelta(convId)
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== convId) return
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
        },
        onNewMessage: () => {
          if (currentConversationId.value !== convId) return
          flushPendingStreamDelta(convId)
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
        onTodoUpdated: (messageId, content) => {
          if (currentConversationId.value !== convId) return
          let idx = -1
          if (messageId) {
            idx = messages.value.findIndex(m => m.id === messageId)
          }
          if (idx < 0) {
            idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          }
          if (idx >= 0) {
            const msg = messages.value[idx]
            if (msg && msg.content !== content) {
              msg.content = content
              triggerRef(messages)
            }
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== convId) return
          flushPendingStreamDelta(convId)
          streamProgress.value = null
          toolExecuting.value = false
          streamError.value = err.message
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
          streaming.value = false
        },
        onComplete: (finalChunk) => {
          flushPendingStreamDelta(convId)
          streaming.value = false
          streamProgress.value = null
          toolExecuting.value = false
          if (currentConversationId.value !== convId) return
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
          }
          fetchMessages(convId)
          fetchConversations()
          useProviderPoolStore().fetchTrialQuota()
          if (awaitingConfirmation.value) {
            void recoverPendingConfirmations(true)
          }
        },
      })
    } catch {
      flushPendingStreamDelta(convId)
      messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
    } finally {
      sending.value = false
      streaming.value = false
      toolExecuting.value = false
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
  }

  // Continue generating from where it stopped
  async function continueMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()

    if (offlineMode.value) {
      appendAssistantLocalMessage(conversationId, '离线模式下不支持继续生成，请先执行 `/offline off`。')
      return
    }

    // Get the last assistant message content to continue from
    const lastMessage = messages.value[messages.value.length - 1]
    if (!lastMessage || lastMessage.role !== 'assistant') return

    const existingContent = lastMessage.content

    try {
      sending.value = true
      streaming.value = true
      streamProgress.value = null
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = existingContent // Start with existing content
      awaitingConfirmation.value = false
      error.value = null

      const request: SendMessageRequest = {
        message: '[CONTINUE]', // Special marker for continue
        provider: '',
        model: modelPreference.value === 'auto' ? '' : modelPreference.value,
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }

      await sseClient.connect(conversationId, request, {
        onStreamId: (streamId) => {
          if (currentConversationId.value !== conversationId) return
          rememberActiveStreamId(streamId)
        },
        onStreamProgress: (progress) => {
          if (currentConversationId.value !== conversationId) return
          if (!_receivedFirstChunk.value) {
            streamProgress.value = formatStreamProgress(progress)
          }
        },
        onMessage: (chunk) => {
          if (currentConversationId.value !== conversationId) return
          if (chunk.awaiting_user_input) {
            markAwaitingConfirmation()
          }
          if (!chunk.delta) return
          streamProgress.value = null
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          enqueueStreamDelta(conversationId, chunk.delta)
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== conversationId) return
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
        },
        onNewMessage: () => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
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
        onTodoUpdated: (messageId, content) => {
          if (currentConversationId.value !== conversationId) return
          let idx = -1
          if (messageId) {
            idx = messages.value.findIndex(m => m.id === messageId)
          }
          if (idx < 0) {
            idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          }
          if (idx >= 0) {
            const msg = messages.value[idx]
            if (msg && msg.content !== content) {
              msg.content = content
              triggerRef(messages)
            }
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          streamProgress.value = null
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
          flushPendingStreamDelta(conversationId)
          streaming.value = false
          streamProgress.value = null
          toolExecuting.value = false
          if (currentConversationId.value !== conversationId) return
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.content = streamingContent.value
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
          }
          fetchMessages(conversationId)
          // Refresh trial quota to update progress bar
          useProviderPoolStore().fetchTrialQuota()
          if (awaitingConfirmation.value) {
            void recoverPendingConfirmations(true)
          }
        },
      })
    } catch (e) {
      flushPendingStreamDelta(conversationId)
      error.value = e instanceof Error ? e.message : 'Failed to continue message'
    } finally {
      sending.value = false
      streaming.value = false
      toolExecuting.value = false
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
    }
  }

  // Regenerate the last assistant message
  async function regenerateMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()

    if (offlineMode.value) {
      appendAssistantLocalMessage(conversationId, '离线模式下不支持重新生成，请先执行 `/offline off`。')
      return
    }

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
      streamProgress.value = null
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = ''; processContentLength.value = 0; toolResults.value = []
      awaitingConfirmation.value = false
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
        provider: '',
        model: modelPreference.value === 'auto' ? '' : modelPreference.value,
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        attachments: lastUserMessage.attachments,
        regenerate: true,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }

      await sseClient.connect(conversationId, request, {
        onStreamId: (streamId) => {
          if (currentConversationId.value !== conversationId) return
          rememberActiveStreamId(streamId)
        },
        onStreamProgress: (progress) => {
          if (currentConversationId.value !== conversationId) return
          if (!_receivedFirstChunk.value) {
            streamProgress.value = formatStreamProgress(progress)
          }
        },
        onMessage: (chunk) => {
          if (currentConversationId.value !== conversationId) return
          if (chunk.awaiting_user_input) {
            markAwaitingConfirmation()
          }
          if (!chunk.delta) return
          streamProgress.value = null
          if (toolExecuting.value) {
            toolExecuting.value = false
          }
          enqueueStreamDelta(conversationId, chunk.delta)
        },
        onToolExecuting: (_toolCount, toolNames, sandboxAvailable, toolCommands) => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          toolExecuting.value = true
          toolExecutingStartTime.value = Date.now()
          toolExecutingNames.value = toolNames || []
          toolExecutingCommands.value = toolCommands || []
          toolSandboxAvailable.value = !!sandboxAvailable
        },
        onToolResults: (results, _toolRound) => {
          if (currentConversationId.value !== conversationId) return
          toolResults.value = [...toolResults.value, ...parseToolResults(results)]
        },
        onNewMessage: () => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
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
        onTodoUpdated: (messageId, content) => {
          if (currentConversationId.value !== conversationId) return
          let idx = -1
          if (messageId) {
            idx = messages.value.findIndex(m => m.id === messageId)
          }
          if (idx < 0) {
            idx = messages.value.findIndex(m => m.role === 'assistant' && (m.content.includes('- [ ]') || m.content.includes('- [x]')))
          }
          if (idx >= 0) {
            const msg = messages.value[idx]
            if (msg && msg.content !== content) {
              msg.content = content
              triggerRef(messages)
            }
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          streamProgress.value = null
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
          flushPendingStreamDelta(conversationId)
          streaming.value = false
          streamProgress.value = null
          toolExecuting.value = false
          if (currentConversationId.value !== conversationId) return
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
          }
          fetchMessages(conversationId)
          // Refresh trial quota to update progress bar
          useProviderPoolStore().fetchTrialQuota()
          if (awaitingConfirmation.value) {
            void recoverPendingConfirmations(true)
          }
        },
      })
    } catch (e) {
      flushPendingStreamDelta(conversationId)
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
      activeStreamId.value = null
      resetPendingStreamDelta()
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
    streamProgress.value = null
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

  function setPendingApproval(data: any) {
    const requestId = data?.id || data?.request_id
    if (!requestId) return
    pendingApproval.value = {
      request_id: requestId,
      tool_name: data.tool_name || '',
      tool_call_id: data.tool_call_id || '',
      arguments: data.arguments || {},
    }
    awaitingConfirmation.value = true
  }

  // --- Ask-user-question methods ---
  function setPendingQuestion(data: any) {
    console.log('[ChatStore] setPendingQuestion called with:', data)
    pendingQuestion.value = data
    awaitingConfirmation.value = !!data
    console.log('[ChatStore] pendingQuestion.value is now:', pendingQuestion.value)
  }

  async function submitQuestionAnswers(answers: Array<{ question_id: string; selected: string[]; other_text?: string }>) {
    if (!pendingQuestion.value) return
    const id = pendingQuestion.value.id
    if (id.startsWith('inline:')) {
      const parts: string[] = []
      for (const ans of answers) {
        const selected = (ans.selected || []).map(v => v.trim()).filter(Boolean)
        if (selected.length > 0) {
          parts.push(selected.join(', '))
        }
        const other = ans.other_text?.trim()
        if (other) {
          parts.push(other)
        }
      }
      const reply = parts.join('\n').trim()
      pendingQuestion.value = null
      awaitingConfirmation.value = false
      if (reply) {
        await sendMessage(reply)
      }
      return
    }
    try {
      await api.post(`/ask-user-question/${id}/answer`, { answers })
      pendingQuestion.value = null
      awaitingConfirmation.value = false
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
    awaitingConfirmation.value = false
    if (id.startsWith('inline:')) {
      return
    }
    // Notify backend to unblock the tool call immediately with default answers
    try {
      await api.post(`/ask-user-question/${id}/dismiss`)
    } catch {
      // Best-effort — question will timeout on backend if this fails
    }
  }

  async function checkPendingQuestion() {
    try {
      const params: Record<string, string> = {}
      if (currentConversationId.value) {
        params.session_id = currentConversationId.value
      }
      const res = await api.get<{ pending: boolean; question?: any }>('/ask-user-question/pending', { params })
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
    if (data) awaitingConfirmation.value = true
  }

  async function checkPendingExecApproval() {
    try {
      const res = await api.get<{ pending: boolean; approval?: any }>('/exec/approvals/pending')
      if (res.data.pending && res.data.approval) {
        pendingExecApproval.value = res.data.approval
        awaitingConfirmation.value = true
      }
    } catch {
      // endpoint may not exist — ignore
    }
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

  function setWebSearchEnabled(enabled: boolean) {
    webSearchEnabled.value = enabled
    saveWebSearchEnabled(enabled)
  }

  function setModelPreference(value: string) {
    const next = value.trim()
    modelPreference.value = next || 'auto'
    saveModelPreference(modelPreference.value)
  }

  function setDeepResearchEnabled(enabled: boolean) {
    deepResearchEnabled.value = enabled
    saveDeepResearchEnabled(enabled)
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
    streamProgress,
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
    awaitingConfirmation,
    pendingExecApproval,
    modelPreference,
    offlineMode,
    webSearchEnabled,
    deepResearchEnabled,

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
    setPendingApproval,
    checkPendingApprovals,
    recoverPendingConfirmations,
    setPendingQuestion,
    submitQuestionAnswers,
    dismissQuestion,
    checkPendingQuestion,
    setPendingExecApproval,
    checkPendingExecApproval,
    resolveExecApproval,
    dismissExecApproval,
    warmupConversation,
    resetWarmup,
    setModelPreference,
    setWebSearchEnabled,
    setDeepResearchEnabled,
  }
})
