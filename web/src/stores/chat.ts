import { defineStore } from 'pinia'
import { ref, shallowRef, computed, watch, triggerRef } from 'vue'
import type {
  Conversation,
  Message,
  SendMessageRequest,
  MessageStats,
  MessageAttachment,
  ConversationCommandState,
  ConversationCommandStatePatch,
  StreamChunk,
} from '@/api/chat'
import { conversationApi, messageApi, warmupApi, injectionApi } from '@/api/chat'
import { approvalApi } from '@/api/approval'
import type { Decision, ExecDecision } from '@/api/approval'
import api from '@/api/client'
import { SSEClient } from '@/utils/sse'
import type { SSEClientOptions } from '@/utils/sse'
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
  command: string // Extracted command/query/path from args
  args?: string // Raw args JSON
  icon: '✓' | '✗' | '⏳'
  status: string // Duration, error message, or status text
  output: string // Truncated stdout/result
  exitCode?: number
  durationMs?: number
  host?: 'local' | 'sandbox'
  riskLevel?: string
  warning?: string
  warningCode?: string
  timestamp: number // When this result was received
}

function formatToolWarningCode(code: string): string {
  const normalized = (code || '').trim()
  const t = i18n.global.t
  const te = i18n.global.te
  const resolve = (key: string, fallback: string, named?: Record<string, string>): string => {
    if (te(key)) return String(named ? t(key, named) : t(key))
    return fallback
  }

  switch (normalized) {
    case 'login_wall':
      return resolve('toolWarnings.statuses.loginWall', 'Login wall detected')
    case 'challenge':
      return resolve('toolWarnings.statuses.challenge', 'Verification challenge detected')
    case 'browser_required':
      return resolve('toolWarnings.statuses.browserRequired', 'Browser session required')
    default:
      return normalized
        ? resolve('toolWarnings.statuses.unknown', 'Warning: ' + normalized, { code: normalized })
        : ''
  }
}

function formatScreenshotCapturedStatus(): string {
  const key = 'toolWarnings.statuses.screenshotCaptured'
  const t = i18n.global.t
  const te = i18n.global.te
  return te(key) ? String(t(key)) : 'Screenshot captured'
}

/** Parse raw tool results into structured ToolResultItems. */
export function parseToolResults(
  results: Array<{ name: string; id: string; args?: string; result?: string }>
): ToolResultItem[] {
  return results.map((r) => {
    let command = ''
    if (r.args) {
      try {
        const parsed = JSON.parse(r.args)
        command =
          parsed.command ||
          parsed.cmd ||
          parsed.query ||
          parsed.url ||
          parsed.href ||
          parsed.path ||
          parsed.name ||
          parsed.action ||
          parsed.sq ||
          parsed.mq ||
          ''
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
    let warning: string | undefined
    let warningCode: string | undefined
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
          icon = '✗'
          status = String(res.error)
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
        if (typeof res.warning === 'string' && res.warning.trim()) warning = res.warning.trim()
        if (typeof res.warning_code === 'string' && res.warning_code.trim())
          warningCode = res.warning_code.trim()
        const screenshot = typeof res.screenshot === 'string' ? res.screenshot.trim() : ''
        if (screenshot && !status) {
          const message = typeof res.message === 'string' ? res.message.trim() : ''
          status = message || formatScreenshotCapturedStatus()
        }
        if (res.stdout?.trim() && r.name !== 'ask') {
          output = res.stdout.trim()
        }
        if (res.stderr?.trim()) {
          const stderr = res.stderr.trim()
          output = output ? `${output}\n${stderr}` : stderr
        }
        if (!status && warningCode) status = formatToolWarningCode(warningCode)
        if (!output && warning) output = warning
        if (res.host) host = res.host
        if (res.risk_level) riskLevel = res.risk_level
      } catch {
        const hasScreenshotPayload = /"screenshot"\s*:/.test(r.result)
        if (hasScreenshotPayload) {
          icon = '✓'
          const messageMatch = r.result.match(/"message"\s*:\s*"([^"]+)/)
          if (messageMatch && messageMatch[1]) {
            status = messageMatch[1]
          } else {
            status = formatScreenshotCapturedStatus()
          }
        } else {
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
    }
    return {
      name: r.name,
      id: r.id,
      command,
      args: r.args,
      icon,
      status,
      output,
      exitCode,
      durationMs,
      host,
      riskLevel,
      warning,
      warningCode,
      timestamp: Date.now(),
    }
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
      return resolve(
        'chat.streamProgress.requestAccepted',
        'Request received, preparing response...'
      )
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
const messageMetadata = ref<
  Map<string, { provider?: string; model?: string; stats?: MessageStats }>
>(new Map())

interface ActiveConversationStreamState {
  conversationId: string
  streamId: string | null
  sending: boolean
  streaming: boolean
  receivedFirstChunk: boolean
  streamProgress: string | null
  toolExecuting: boolean
  toolExecutingStartTime: number
  toolExecutingNames: string[]
  toolExecutingCommands: string[]
  toolSandboxAvailable: boolean
  awaitingConfirmation: boolean
  previewContent: string
}

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

  function splitModelPreference(value: string): {
    selected_provider_id?: string
    selected_model_id?: string
  } {
    const trimmed = value.trim()
    if (!trimmed || trimmed === 'auto') return {}
    const slash = trimmed.indexOf('/')
    if (slash > 0 && slash < trimmed.length - 1) {
      return {
        selected_provider_id: trimmed.slice(0, slash).trim(),
        selected_model_id: trimmed.slice(slash + 1).trim(),
      }
    }
    return { selected_model_id: trimmed }
  }

  function commandStateToModelPreference(state: ConversationCommandState): string {
    const provider = state.selected_provider_id?.trim() || ''
    const model = state.selected_model_id?.trim() || ''
    if (provider && model) return `${provider}/${model}`
    if (model) return model
    return 'auto'
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
  watch(toolExecuting, (v) => {
    if (!v) {
      toolExecutingNames.value = []
      toolExecutingCommands.value = []
      toolSandboxAvailable.value = false
    }
  })
  const contextTrimInfo = ref<{
    type: 'pruned' | 'compacted'
    messagesPruned?: number
    tokensBefore?: number
    tokensAfter?: number
    before?: number
    after?: number
  } | null>(null)

  // Pre-TTFT cancel state: when user starts typing before first token arrives
  const preTTFTCancelActive = ref(false)
  let preTTFTResumeTimer: ReturnType<typeof setTimeout> | null = null
  const _receivedFirstChunk = ref(false)

  // Computed: true when we're waiting for first token (user message sent, no content yet)
  const isPreTTFT = computed(
    () => sending.value && !_receivedFirstChunk.value && !preTTFTCancelActive.value
  )

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
    session_id?: string
    conversation_id?: string
    expires_at: number
  } | null>(null)

  const isMultiSelectMode = ref(false)
  const selectedProviderId = ref<string>('')
  const modelPreference = ref<string>(loadModelPreference())
  const offlineMode = ref<boolean>(loadOfflineMode())
  const webSearchEnabled = ref<boolean>(loadWebSearchEnabled())
  const deepResearchEnabled = ref<boolean>(loadDeepResearchEnabled())
  const activeStreamId = ref<string | null>(null)
  const activeStreamState = ref<ActiveConversationStreamState | null>(null)
  const commandStateHydrated = ref(false)
  let commandStateHydratePromise: Promise<ConversationCommandState> | null = null

  // SSE client for streaming
  const sseClient = new SSEClient()
  const pendingRecoveryRetryDelayMs = 700
  const pendingRecoveryRetryLimit = 8
  let pendingRecoveryRetryCount = 0
  let pendingRecoveryRetryTimer: ReturnType<typeof setTimeout> | null = null

  function getActiveStreamState(
    conversationId?: string | null
  ): ActiveConversationStreamState | null {
    const state = activeStreamState.value
    if (!state) return null
    if (conversationId && state.conversationId !== conversationId) return null
    return state
  }

  function applyVisibleStreamState(state: ActiveConversationStreamState | null) {
    sending.value = !!state?.sending
    streaming.value = !!state?.streaming
    activeStreamId.value = state?.streamId ?? null
    streamProgress.value = state?.receivedFirstChunk ? null : (state?.streamProgress ?? null)
    _receivedFirstChunk.value = !!state?.receivedFirstChunk

    if (state?.toolExecuting) {
      toolExecutingStartTime.value = state.toolExecutingStartTime
      toolExecutingNames.value = [...state.toolExecutingNames]
      toolExecutingCommands.value = [...state.toolExecutingCommands]
      toolSandboxAvailable.value = state.toolSandboxAvailable
      toolExecuting.value = true
    } else {
      toolExecuting.value = false
      toolExecutingStartTime.value = 0
    }

    if (state?.awaitingConfirmation) {
      awaitingConfirmation.value = true
    } else if (!pendingQuestion.value && !pendingApproval.value && !pendingExecApproval.value) {
      awaitingConfirmation.value = false
    }
  }

  function beginActiveStream(conversationId: string) {
    const next: ActiveConversationStreamState = {
      conversationId,
      streamId: null,
      sending: true,
      streaming: true,
      receivedFirstChunk: false,
      streamProgress: null,
      toolExecuting: false,
      toolExecutingStartTime: 0,
      toolExecutingNames: [],
      toolExecutingCommands: [],
      toolSandboxAvailable: false,
      awaitingConfirmation: false,
      previewContent: '',
    }
    activeStreamState.value = next
    if (currentConversationId.value === conversationId) {
      applyVisibleStreamState(next)
    }
  }

  function updateActiveStreamState(
    conversationId: string,
    patch: Partial<ActiveConversationStreamState>
  ) {
    const current = getActiveStreamState(conversationId)
    if (!current) return
    const next: ActiveConversationStreamState = {
      ...current,
      ...patch,
      toolExecutingNames: patch.toolExecutingNames
        ? [...patch.toolExecutingNames]
        : current.toolExecutingNames,
      toolExecutingCommands: patch.toolExecutingCommands
        ? [...patch.toolExecutingCommands]
        : current.toolExecutingCommands,
    }
    activeStreamState.value = next
    if (currentConversationId.value === conversationId) {
      applyVisibleStreamState(next)
    }
  }

  function appendActiveStreamPreview(conversationId: string, delta: string) {
    if (!delta) return
    const current = getActiveStreamState(conversationId)
    if (!current) return
    updateActiveStreamState(conversationId, {
      previewContent: current.previewContent + delta,
      receivedFirstChunk: true,
      streamProgress: null,
      toolExecuting: false,
    })
  }

  function resetActiveStreamRound(conversationId: string) {
    updateActiveStreamState(conversationId, {
      receivedFirstChunk: false,
      streamProgress: null,
      toolExecuting: false,
      toolExecutingStartTime: 0,
      toolExecutingNames: [],
      toolExecutingCommands: [],
      toolSandboxAvailable: false,
      awaitingConfirmation: false,
      previewContent: '',
    })
  }

  function clearActiveStreamState(conversationId?: string | null) {
    const current = activeStreamState.value
    if (!current) return
    if (conversationId && current.conversationId !== conversationId) return
    activeStreamState.value = null
  }

  function clearVisibleStreamState() {
    sending.value = false
    streaming.value = false
    streamProgress.value = null
    toolExecuting.value = false
    toolExecutingStartTime.value = 0
    activeStreamId.value = null
    resetPendingStreamDelta()
    streamingContent.value = ''
    processContentLength.value = 0
    toolResults.value = []
    _receivedFirstChunk.value = false
  }

  function finalizeVisibleStreamSession(conversationId: string) {
    const active = getActiveStreamState(conversationId)
    if (active) {
      if (currentConversationId.value === conversationId) {
        applyVisibleStreamState(active)
      }
      return
    }
    if (currentConversationId.value === conversationId) {
      clearVisibleStreamState()
    }
  }

  function restoreDetachedActiveStream(conversationId: string) {
    const state = getActiveStreamState(conversationId)
    if (!state?.streaming || currentConversationId.value !== conversationId) return

    resetPendingStreamDelta()
    const previewContent = state.previewContent || ''
    const lastMessage = messages.value[messages.value.length - 1]
    const canReuseLastAssistant =
      !!previewContent &&
      !!lastMessage &&
      lastMessage.role === 'assistant' &&
      previewContent.startsWith(lastMessage.content)

    if (canReuseLastAssistant && lastMessage) {
      if (lastMessage.content !== previewContent) {
        lastMessage.content = previewContent
        triggerRef(messages)
      }
    } else {
      const assistantMessage = createStreamingAssistantMessage(conversationId)
      assistantMessage.content = previewContent
      messages.value = [...messages.value, assistantMessage]
    }

    streamingContent.value = previewContent
    processContentLength.value = 0
    toolResults.value = []
    applyVisibleStreamState(state)
  }

  async function connectConversationStream(
    conversationId: string,
    request: SendMessageRequest,
    options: SSEClientOptions
  ) {
    beginActiveStream(conversationId)

    await sseClient.connect(conversationId, request, {
      ...options,
      onStreamId: (streamId) => {
        updateActiveStreamState(conversationId, { streamId })
        options.onStreamId?.(streamId)
      },
      onStreamProgress: (progress) => {
        const current = getActiveStreamState(conversationId)
        if (current && !current.receivedFirstChunk) {
          updateActiveStreamState(conversationId, {
            streamProgress: formatStreamProgress(progress),
          })
        }
        options.onStreamProgress?.(progress)
      },
      onMessage: (chunk) => {
        if (chunk.awaiting_user_input) {
          updateActiveStreamState(conversationId, { awaitingConfirmation: true })
        }
        if (chunk.delta) {
          appendActiveStreamPreview(conversationId, chunk.delta)
        }
        options.onMessage(chunk)
      },
      onToolExecuting: (toolCount, toolNames, sandboxAvailable, toolCommands) => {
        updateActiveStreamState(conversationId, {
          toolExecuting: true,
          toolExecutingStartTime: Date.now(),
          toolExecutingNames: toolNames || [],
          toolExecutingCommands: toolCommands || [],
          toolSandboxAvailable: !!sandboxAvailable,
        })
        options.onToolExecuting?.(toolCount, toolNames, sandboxAvailable, toolCommands)
      },
      onToolResults: (results, toolRound) => {
        options.onToolResults?.(results, toolRound)
      },
      onNewMessage: (toolRound) => {
        resetActiveStreamRound(conversationId)
        options.onNewMessage?.(toolRound)
      },
      onTodoUpdated: (messageId, content, todoCardId) => {
        options.onTodoUpdated?.(messageId, content, todoCardId)
      },
      onInjection: (userMessage) => {
        resetActiveStreamRound(conversationId)
        options.onInjection?.(userMessage)
      },
      onBlocked: (message, threatLevel) => {
        clearActiveStreamState(conversationId)
        options.onBlocked?.(message, threatLevel)
      },
      onTrialExhausted: (message) => {
        clearActiveStreamState(conversationId)
        options.onTrialExhausted?.(message)
      },
      onContextTrimmed: (info) => {
        options.onContextTrimmed?.(info)
      },
      onNetworkInterrupt: () => {
        options.onNetworkInterrupt?.()
      },
      onError: (err) => {
        clearActiveStreamState(conversationId)
        options.onError?.(err)
      },
      onComplete: (finalChunk) => {
        clearActiveStreamState(conversationId)
        options.onComplete?.(finalChunk)
      },
    })
  }

  function applyCommandState(state: ConversationCommandState) {
    selectedProviderId.value = state.selected_provider_id?.trim() || ''
    modelPreference.value = commandStateToModelPreference(state)
    offlineMode.value = !!state.offline
    webSearchEnabled.value = state.web_search_enabled !== false
    deepResearchEnabled.value = !!state.deep_research_enabled
  }

  function getLocalCommandStateSeed(): ConversationCommandState {
    return {
      ...splitModelPreference(loadModelPreference()),
      offline: loadOfflineMode(),
      web_search_enabled: loadWebSearchEnabled(),
      deep_research_enabled: loadDeepResearchEnabled(),
    }
  }

  async function fetchCommandState(conversationId: string, options?: { force?: boolean }) {
    const force = !!options?.force
    if (!force && commandStateHydrated.value) {
      const cached: ConversationCommandState = {
        conversation_id: conversationId,
        selected_provider_id: selectedProviderId.value,
        selected_model_id: splitModelPreference(modelPreference.value).selected_model_id || '',
        offline: offlineMode.value,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }
      return cached
    }
    if (!force && commandStateHydratePromise) {
      return commandStateHydratePromise
    }
    const req = conversationApi
      .getCommandState(conversationId)
      .then((response) => {
        applyCommandState(response.data)
        commandStateHydrated.value = true
        return response.data
      })
      .finally(() => {
        commandStateHydratePromise = null
      })
    commandStateHydratePromise = req
    return req
  }

  async function patchCommandState(conversationId: string, patch: ConversationCommandStatePatch) {
    const response = await conversationApi.patchCommandState(conversationId, patch)
    applyCommandState(response.data)
    commandStateHydrated.value = true
    return response.data
  }

  async function seedConversationCommandState(conversationId: string) {
    if (commandStateHydrated.value) return
    const seed = getLocalCommandStateSeed()
    await patchCommandState(conversationId, seed)
  }

  function isSlashCommandText(value: string): boolean {
    return value.trim().startsWith('/')
  }

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
    if (
      expectedConversationId &&
      pendingStreamConversationId &&
      expectedConversationId !== pendingStreamConversationId
    ) {
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

  function createStreamingAssistantMessage(conversationId: string): Message {
    const id = `streaming-${Date.now()}`
    return {
      id,
      render_key: id,
      conversation_id: conversationId,
      role: 'assistant',
      content: '',
      created_at: new Date().toISOString(),
    }
  }

  function isTodoChecklistContent(content?: string): boolean {
    if (!content) return false
    return /(^|\n)[ \t]*[-*]\s+\[(?: |x|X)\]\s+/.test(content)
  }

  function findTodoChecklistMessageIndex(messageId?: string): number {
    const normalizedMessageId = messageId?.trim()
    if (normalizedMessageId) {
      const exactIndex = messages.value.findIndex(
        (m) => m.id === normalizedMessageId || m.render_key === normalizedMessageId
      )
      if (exactIndex >= 0) return exactIndex
    }
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const msg = messages.value[i]
      if (!msg || msg.role !== 'assistant') continue
      if (isTodoChecklistContent(msg.content)) return i
    }
    return -1
  }

  function applyTodoChecklistUpdate(messageId: string, content: string, todoCardId?: string) {
    const idx = findTodoChecklistMessageIndex(messageId)
    if (idx < 0) return
    const msg = messages.value[idx]
    const normalizedTodoCardId = todoCardId?.trim()
    if (!msg) return

    let changed = false
    if (msg.content !== content) {
      msg.content = content
      changed = true
    }
    if (normalizedTodoCardId && msg.todo_card_id !== normalizedTodoCardId) {
      msg.todo_card_id = normalizedTodoCardId
      changed = true
    }
    if (changed) {
      triggerRef(messages)
    }
  }

  function applyFinalStreamChunk(conversationId: string, finalChunk?: StreamChunk): boolean {
    if (currentConversationId.value !== conversationId) return false
    const persistedMessageId = finalChunk?.message_id?.trim()
    if (!persistedMessageId) return false

    const lastIndex = messages.value.length - 1
    if (lastIndex < 0) return false
    const lastMsg = messages.value[lastIndex]
    if (!lastMsg || lastMsg.role !== 'assistant' || !lastMsg.id.startsWith('streaming-')) {
      return false
    }

    const nextContent = finalChunk?.content ?? streamingContent.value
    const nextMessage: Message = {
      ...lastMsg,
      id: persistedMessageId,
      render_key: lastMsg.render_key || lastMsg.id,
      content: nextContent,
      provider: finalChunk?.provider || lastMsg.provider,
      model: finalChunk?.model || lastMsg.model,
      stats: finalChunk?.stats || lastMsg.stats,
    }
    const nextMessages = [...messages.value]
    nextMessages[lastIndex] = nextMessage
    messages.value = nextMessages
    return true
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

  function normalizePendingSessionId(data: any): string {
    if (!data || typeof data !== 'object') return ''
    const sessionId = typeof data.session_id === 'string' ? data.session_id.trim() : ''
    if (sessionId) return sessionId
    const conversationId =
      typeof data.conversation_id === 'string' ? data.conversation_id.trim() : ''
    return conversationId
  }

  function stringifyOptional(value: unknown): string | undefined {
    if (typeof value === 'string') {
      const v = value.trim()
      return v || undefined
    }
    if (typeof value === 'number' || typeof value === 'boolean') {
      return String(value)
    }
    return undefined
  }

  function normalizePendingExecApproval(data: any): {
    id: string
    type: string
    command?: string
    directory?: string
    workdir?: string
    host?: string
    security?: string
    session_id?: string
    conversation_id?: string
    expires_at: number
  } | null {
    if (!data || typeof data !== 'object') return null

    const source = (() => {
      if (data.approval && typeof data.approval === 'object') return data.approval
      if (data.data && typeof data.data === 'object') return data.data
      return data
    })()

    const id = stringifyOptional(source.id || source.request_id)
    if (!id) return null

    let command = source.command ?? source.cmd
    if (Array.isArray(command)) {
      command = command.map((v) => String(v)).join(' ')
    } else if (command && typeof command === 'object') {
      try {
        command = JSON.stringify(command)
      } catch {
        command = String(command)
      }
    }

    let expiresAt = Number(source.expires_at ?? source.expiresAt ?? 0)
    if (Number.isFinite(expiresAt) && expiresAt > 0 && expiresAt < 1e12) {
      expiresAt *= 1000
    }
    if (!Number.isFinite(expiresAt) || expiresAt <= 0) {
      expiresAt = Date.now() + 2 * 60 * 1000
    }

    const normalized = {
      id,
      type: stringifyOptional(source.type) || 'directory',
      command: stringifyOptional(command),
      directory: stringifyOptional(source.directory ?? source.dir ?? source.path),
      workdir: stringifyOptional(source.workdir ?? source.cwd),
      host: stringifyOptional(source.host),
      security: stringifyOptional(source.security),
      session_id: stringifyOptional(source.session_id),
      conversation_id: stringifyOptional(source.conversation_id),
      expires_at: expiresAt,
    }

    if (!normalized.directory && normalized.workdir) {
      normalized.directory = normalized.workdir
    }

    return normalized
  }

  function clearPendingRecoveryRetryTimer() {
    if (pendingRecoveryRetryTimer) {
      clearTimeout(pendingRecoveryRetryTimer)
      pendingRecoveryRetryTimer = null
    }
    pendingRecoveryRetryCount = 0
  }

  async function recoverPendingConfirmationsWithRetry() {
    clearPendingRecoveryRetryTimer()

    const attemptRecovery = async () => {
      await recoverPendingConfirmations(false)
      if (hasPendingConfirmations() || !awaitingConfirmation.value) {
        clearPendingRecoveryRetryTimer()
        return
      }
      if (pendingRecoveryRetryCount >= pendingRecoveryRetryLimit) {
        return
      }
      pendingRecoveryRetryCount++
      pendingRecoveryRetryTimer = setTimeout(() => {
        void attemptRecovery()
      }, pendingRecoveryRetryDelayMs)
    }

    await attemptRecovery()
  }

  function shouldSurfacePendingForCurrentConversation(sessionId: string): boolean {
    if (!sessionId) return true
    const currentId = currentConversationId.value?.trim() || ''
    return !currentId || currentId === sessionId
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

    const lines = content
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean)
    if (lines.length === 0) return false

    const question = lines.find(
      (line) => /[?？]\s*$/.test(line) && !/^[-*•]\s+/.test(line) && !/^\d+[.)]\s+/.test(line)
    )
    if (!question) return false

    const optionLines = lines.filter((line) => /^[-*•]\s+/.test(line) || /^\d+[.)]\s+/.test(line))
    if (optionLines.length < 2 || optionLines.length > 8) return false

    const seen = new Set<string>()
    const options = optionLines
      .map(extractInlineQuestionLabel)
      .filter((label) => {
        if (!label) return false
        const key = label.toLowerCase()
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
      .map((label) => ({ label, value: label }))

    if (options.length < 2) return false

    pendingQuestion.value = {
      id: `inline:${Date.now()}`,
      questions: [
        {
          id: 'q1',
          question: question.replace(/\*\*/g, '').trim(),
          header: 'Question',
          options,
          multi_select: false,
        },
      ],
      expires_at: Date.now() + 10 * 60 * 1000,
    }
    awaitingConfirmation.value = true
    return true
  }

  async function recoverPendingConfirmations(allowInlineFallback = false): Promise<void> {
    await Promise.all([checkPendingApprovals(), checkPendingQuestion(), checkPendingExecApproval()])
    if (hasPendingConfirmations()) {
      clearPendingRecoveryRetryTimer()
      awaitingConfirmation.value = true
      return
    }
    if (allowInlineFallback && inferInlinePendingQuestionFromAssistantMessage()) {
      clearPendingRecoveryRetryTimer()
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
      void recoverPendingConfirmationsWithRetry()
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
      // Then sort by update recency; tie-break to keep deterministic order.
      const updatedDiff = new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      if (updatedDiff !== 0) return updatedDiff
      const createdDiff = new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
      if (createdDiff !== 0) return createdDiff
      return b.id.localeCompare(a.id)
    })
  })

  // Conversation IDs that still have an active execution stream.
  const executingConversationIds = computed(() => {
    const state = activeStreamState.value
    if (!state?.conversationId) return []
    if (state.streaming || state.sending || state.toolExecuting || state.awaitingConfirmation) {
      return [state.conversationId]
    }
    return []
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
    if (currentConversationId.value) {
      updateActiveStreamState(currentConversationId.value, {
        streamId: activeStreamId.value,
        sending: true,
        streaming: true,
        receivedFirstChunk: _receivedFirstChunk.value,
        streamProgress: streamProgress.value,
        toolExecuting: toolExecuting.value,
        toolExecutingStartTime: toolExecutingStartTime.value,
        toolExecutingNames: [...toolExecutingNames.value],
        toolExecutingCommands: [...toolExecutingCommands.value],
        toolSandboxAvailable: toolSandboxAvailable.value,
        awaitingConfirmation:
          awaitingConfirmation.value ||
          !!pendingQuestion.value ||
          !!pendingApproval.value ||
          !!pendingExecApproval.value,
        previewContent: streamingContent.value,
      })
    }
    pendingQuestion.value = null
    pendingApproval.value = null
    pendingExecApproval.value = null
    awaitingConfirmation.value = false
    clearPendingRecoveryRetryTimer()
    clearVisibleStreamState()
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
      if (!commandStateHydrated.value) {
        try {
          await seedConversationCommandState(response.data.id)
        } catch {
          applyCommandState(getLocalCommandStateSeed())
        }
      }
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
      const [messageResponse] = await Promise.all([
        messageApi.list(id, PAGE_SIZE, 0),
        fetchCommandState(id).catch(() => null),
      ])
      const fetchedMessages = messageResponse.data

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

    if (getActiveStreamState(id)?.streaming) {
      restoreDetachedActiveStream(id)
      void recoverPendingConfirmations(true)
      return
    }

    // Restore pending confirmations for this conversation if any.
    void recoverPendingConfirmations(false)
  }

  async function fetchMessages(conversationId: string, page = 0) {
    // Save metadata from current messages before refresh (for messages not yet persisted to DB)
    const savedMetadata: Map<number, { provider?: string; model?: string; stats?: MessageStats }> =
      new Map()
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
      const response = await messageApi.list(conversationId, PAGE_SIZE, page * PAGE_SIZE)
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

  async function sendMessage(
    content: string,
    fileAttachments?: {
      id: string
      file: File
      name: string
      size: number
      type: string
      preview?: string
      duration?: number
    }[]
  ) {
    if (!currentConversationId.value) {
      // Use the first part of the message as the conversation title
      const title = content.length > 30 ? content.substring(0, 30) + '...' : content
      await createConversation(title)
    }

    const conversationId = currentConversationId.value!
    const settingsStore = useSettingsStore()
    const shouldRefreshCommandStateAfterComplete =
      (!fileAttachments || fileAttachments.length === 0) && isSlashCommandText(content)

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
            type: attachment.type.startsWith('image/')
              ? 'image'
              : attachment.type.startsWith('audio/')
                ? 'audio'
                : 'file',
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

    const modelSelection = splitModelPreference(modelPreference.value)
    const request: SendMessageRequest = {
      message: content,
      provider: selectedProviderId.value || modelSelection.selected_provider_id || '',
      model: modelSelection.selected_model_id || '',
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
      streamingContent.value = ''
      processContentLength.value = 0
      toolResults.value = []
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
      const assistantMessage = createStreamingAssistantMessage(conversationId)
      messages.value = [...messages.value, assistantMessage]

      // Capture the conversation ID at send time so callbacks can detect stale streams
      const sendConvId = conversationId

      await connectConversationStream(conversationId, request, {
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
          streamingContent.value = ''
          processContentLength.value = 0
          toolResults.value = []
          toolExecuting.value = false
          const newAssistant = createStreamingAssistantMessage(conversationId)
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (messageId, content, todoCardId) => {
          if (currentConversationId.value !== sendConvId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
        },
        onInjection: () => {
          if (currentConversationId.value !== sendConvId) return
          flushPendingStreamDelta(sendConvId)
          // Server confirmed injection — partial response is preserved in DB,
          // new user message stored. The stream will restart server-side.
          // Reset streaming content for the new response.
          streamingContent.value = ''
          processContentLength.value = 0
          toolResults.value = []
          toolExecuting.value = false
          // Add a new streaming placeholder for the restarted response
          const newAssistant = createStreamingAssistantMessage(conversationId)
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
            STREAM_EMPTY: 'streamEmpty',
            STREAM_ERROR: 'streamError',
            PROVIDER_NO_RESPONSE: 'providerNoResponse',
            PROVIDER_RETURNED_EMPTY: 'providerReturnedEmpty',
            'No response body': 'noResponseBody',
            provider_tool_unsupported: 'provider_tool_unsupported',
            provider_unavailable: 'provider_unavailable',
            provider_auth_error: 'provider_auth_error',
            provider_rate_limited: 'provider_rate_limited',
            provider_openrouter_privacy_policy: 'provider_openrouter_privacy_policy',
            trial_service_busy: 'trial_service_busy',
            exec_directory_approval_timeout: 'execDirectoryApprovalTimeout',
          }
          const resolveErrorKey = (message: string): string | undefined => {
            if (errorMap[message]) return errorMap[message]
            const lower = message.toLowerCase()
            if (lower.includes('provider_openrouter_privacy_policy'))
              return 'provider_openrouter_privacy_policy'
            if (
              lower.includes('no endpoints found matching your data policy') &&
              lower.includes('free model publication')
            )
              return 'provider_openrouter_privacy_policy'
            if (lower.includes('provider_tool_unsupported')) return 'provider_tool_unsupported'
            if (lower.includes('provider_unavailable') || lower.includes('no available provider'))
              return 'provider_unavailable'
            if (lower.includes('provider_auth_error') || lower.includes('auth error'))
              return 'provider_auth_error'
            if (
              lower.includes('provider_rate_limited') ||
              lower.includes('429') ||
              lower.includes('throttled')
            )
              return 'provider_rate_limited'
            if (lower.includes('trial_service_busy')) return 'trial_service_busy'
            if (
              lower.includes('exec denied: directory approval') &&
              lower.includes('approval timed out')
            )
              return 'execDirectoryApprovalTimeout'
            return undefined
          }

          // Transient empty-response errors that can be silently recovered
          const transientErrors = new Set([
            'STREAM_EMPTY',
            'PROVIDER_NO_RESPONSE',
            'PROVIDER_RETURNED_EMPTY',
          ])

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
            fetchMessages(conversationId)
              .then(() => {
                if (currentConversationId.value !== sendConvId) return
                const serverMessages = messages.value.filter((m) => !m.id.startsWith('streaming-'))
                messages.value = serverMessages
                // Only show error if server also has no new content
                const lastMsg = serverMessages[serverMessages.length - 1]
                if (!lastMsg || lastMsg.role !== 'assistant' || !lastMsg.content?.trim()) {
                  const errorKey = resolveErrorKey(err.message)
                  streamError.value = errorKey || err.message
                }
              })
              .catch(() => {
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
              newMessages[idx] = {
                ...streamingMsg,
                content: streamingMsg.content + '\n\n[Response interrupted]',
              }
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
          const finalizedLocally = applyFinalStreamChunk(sendConvId, finalChunk)
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (!finalizedLocally && lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
            const msgId = finalChunk.message_id?.trim() || lastMsg?.id
            if (msgId) {
              messageMetadata.value.set(msgId, {
                provider: finalChunk.provider,
                model: finalChunk.model,
                stats: finalChunk.stats,
              })
            }
          }
          if (!finalizedLocally) {
            fetchMessages(conversationId)
          }
          // Refresh conversations to get updated title (auto-generated after first message)
          fetchConversations()
          if (shouldRefreshCommandStateAfterComplete) {
            void fetchCommandState(conversationId, { force: true }).catch(() => {})
          }
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
          newMessages[idx] = {
            ...streamingMsg,
            content: streamingMsg.content + '\n\n[Response interrupted]',
          }
        }
        messages.value = newMessages
      } else {
        messages.value = messages.value.filter(
          (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
        )
      }
    } finally {
      finalizeVisibleStreamSession(conversationId)
    }
  }

  function cancelStreaming() {
    flushPendingStreamDelta(currentConversationId.value)
    void cancelActiveStreamOnServer(currentConversationId.value)
    sseClient.disconnect()
    clearActiveStreamState(currentConversationId.value)
    clearVisibleStreamState()
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
    clearActiveStreamState(convId)
    clearVisibleStreamState()

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
      streamingContent.value = ''
      processContentLength.value = 0
      toolResults.value = []
      awaitingConfirmation.value = false
      _receivedFirstChunk.value = false

      // Add placeholder for assistant message
      const assistantMessage = createStreamingAssistantMessage(convId)
      messages.value = [...messages.value, assistantMessage]

      const modelSelection = splitModelPreference(modelPreference.value)
      const request: SendMessageRequest = {
        message: '[CONTINUE_AFTER_CANCEL]',
        provider: selectedProviderId.value || modelSelection.selected_provider_id || '',
        model: modelSelection.selected_model_id || '',
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }

      await connectConversationStream(convId, request, {
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
          streamingContent.value = ''
          processContentLength.value = 0
          toolResults.value = []
          toolExecuting.value = false
          const newAssistant = createStreamingAssistantMessage(convId)
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (messageId, content, todoCardId) => {
          if (currentConversationId.value !== convId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
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
          const finalizedLocally = applyFinalStreamChunk(convId, finalChunk)
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (!finalizedLocally && lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
          }
          if (!finalizedLocally) {
            fetchMessages(convId)
          }
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
      finalizeVisibleStreamSession(convId)
    }
  }

  // Continue generating from where it stopped
  async function continueMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()

    if (offlineMode.value) {
      appendAssistantLocalMessage(
        conversationId,
        '离线模式下不支持继续生成，请先执行 `/offline off`。'
      )
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

      const modelSelection = splitModelPreference(modelPreference.value)
      const request: SendMessageRequest = {
        message: '[CONTINUE]', // Special marker for continue
        provider: selectedProviderId.value || modelSelection.selected_provider_id || '',
        model: modelSelection.selected_model_id || '',
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }

      await connectConversationStream(conversationId, request, {
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
          streamingContent.value = ''
          processContentLength.value = 0
          toolResults.value = []
          toolExecuting.value = false
          const newAssistant = createStreamingAssistantMessage(conversationId)
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (messageId, content, todoCardId) => {
          if (currentConversationId.value !== conversationId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
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
          const finalizedLocally = applyFinalStreamChunk(conversationId, finalChunk)
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (!finalizedLocally && lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.content = streamingContent.value
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
          }
          if (!finalizedLocally) {
            fetchMessages(conversationId)
          }
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
      finalizeVisibleStreamSession(conversationId)
    }
  }

  // Regenerate the last assistant message
  async function regenerateMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()

    if (offlineMode.value) {
      appendAssistantLocalMessage(
        conversationId,
        '离线模式下不支持重新生成，请先执行 `/offline off`。'
      )
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
      streamingContent.value = ''
      processContentLength.value = 0
      toolResults.value = []
      awaitingConfirmation.value = false
      error.value = null

      // Add placeholder for new assistant message
      const assistantMessage = createStreamingAssistantMessage(conversationId)
      messages.value = [...messages.value, assistantMessage]

      const modelSelection = splitModelPreference(modelPreference.value)
      const request: SendMessageRequest = {
        message: lastUserMessage.content,
        provider: selectedProviderId.value || modelSelection.selected_provider_id || '',
        model: modelSelection.selected_model_id || '',
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        attachments: lastUserMessage.attachments,
        regenerate: true,
        web_search_enabled: webSearchEnabled.value,
        deep_research_enabled: deepResearchEnabled.value,
      }

      await connectConversationStream(conversationId, request, {
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
          streamingContent.value = ''
          processContentLength.value = 0
          toolResults.value = []
          toolExecuting.value = false
          const newAssistant = createStreamingAssistantMessage(conversationId)
          messages.value = [...messages.value, newAssistant]
        },
        onTodoUpdated: (messageId, content, todoCardId) => {
          if (currentConversationId.value !== conversationId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
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
              newMessages[idx] = {
                ...streamingMsg,
                content: streamingMsg.content + '\n\n[Response interrupted]',
              }
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
          const finalizedLocally = applyFinalStreamChunk(conversationId, finalChunk)
          if (finalChunk && (finalChunk.provider || finalChunk.model || finalChunk.stats)) {
            const lastIndex = messages.value.length - 1
            const lastMsg = messages.value[lastIndex]
            if (!finalizedLocally && lastIndex >= 0 && lastMsg?.role === 'assistant') {
              lastMsg.provider = finalChunk.provider
              lastMsg.model = finalChunk.model
              lastMsg.stats = finalChunk.stats
              triggerRef(messages)
            }
          }
          if (!finalizedLocally) {
            fetchMessages(conversationId)
          }
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
          newMessages[idx] = {
            ...streamingMsg,
            content: streamingMsg.content + '\n\n[Response interrupted]',
          }
          messages.value = newMessages
        }
      } else {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      }
    } finally {
      finalizeVisibleStreamSession(conversationId)
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
      const localResults = conversations.value.filter((c) =>
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
      const response = await approvalApi.listPending(currentConversationId.value || undefined)
      const pending = response.data
      if (pending && pending.length > 0) {
        const first = pending[0]
        if (first) {
          setPendingApproval(first)
        }
      } else if (!streaming.value) {
        pendingApproval.value = null
      }
    } catch {
      // Approval endpoint may not exist yet — ignore
    }
  }

  function setPendingApproval(data: any) {
    const requestId = data?.id || data?.request_id
    if (!requestId) return
    const sessionId = normalizePendingSessionId(data)
    if (sessionId) {
      updateActiveStreamState(sessionId, { awaitingConfirmation: true })
    }
    if (!shouldSurfacePendingForCurrentConversation(sessionId)) {
      return
    }
    pendingApproval.value = {
      request_id: requestId,
      tool_name: data.tool_name || '',
      tool_call_id: data.tool_call_id || '',
      arguments: data.arguments || {},
    }
    clearPendingRecoveryRetryTimer()
    awaitingConfirmation.value = true
  }

  // --- Ask-user-question methods ---
  function setPendingQuestion(data: any) {
    console.log('[ChatStore] setPendingQuestion called with:', data)
    const sessionId = normalizePendingSessionId(data)
    if (sessionId) {
      updateActiveStreamState(sessionId, { awaitingConfirmation: !!data })
    }
    if (data && !shouldSurfacePendingForCurrentConversation(sessionId)) {
      return
    }
    pendingQuestion.value = data
    if (data) {
      clearPendingRecoveryRetryTimer()
    }
    awaitingConfirmation.value = !!data
    console.log('[ChatStore] pendingQuestion.value is now:', pendingQuestion.value)
  }

  async function submitQuestionAnswers(
    answers: Array<{ question_id: string; selected: string[]; other_text?: string }>
  ) {
    if (!pendingQuestion.value) return
    const id = pendingQuestion.value.id
    if (id.startsWith('inline:')) {
      const parts: string[] = []
      for (const ans of answers) {
        const selected = (ans.selected || []).map((v) => v.trim()).filter(Boolean)
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
      const res = await api.get<{ pending: boolean; question?: any }>(
        '/ask-user-question/pending',
        { params }
      )
      if (res.data.pending && res.data.question) {
        setPendingQuestion(res.data.question)
      } else if (!streaming.value) {
        setPendingQuestion(null)
      }
    } catch {
      // endpoint may not exist — ignore
    }
  }

  // --- Exec approval methods ---
  function setPendingExecApproval(data: any) {
    const normalized = data ? normalizePendingExecApproval(data) : null
    const sessionId = normalizePendingSessionId(normalized || data)
    if (sessionId) {
      updateActiveStreamState(sessionId, { awaitingConfirmation: !!normalized })
    }
    if (normalized && !shouldSurfacePendingForCurrentConversation(sessionId)) {
      return
    }
    pendingExecApproval.value = normalized
    if (normalized) {
      clearPendingRecoveryRetryTimer()
      awaitingConfirmation.value = true
    } else if (!streaming.value && !pendingQuestion.value && !pendingApproval.value) {
      awaitingConfirmation.value = false
    }
  }

  async function checkPendingExecApproval() {
    try {
      const params: Record<string, string> = {}
      if (currentConversationId.value) {
        params.session_id = currentConversationId.value
      }
      const res = await api.get<{ pending: boolean; approval?: any }>('/exec/approvals/pending', {
        params,
      })
      if (res.data.pending && res.data.approval) {
        setPendingExecApproval(res.data.approval)
      } else if (!streaming.value) {
        setPendingExecApproval(null)
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
      setPendingExecApproval(null)
    }
  }

  function dismissExecApproval() {
    setPendingExecApproval(null)
  }

  async function clearAllConversations() {
    try {
      // Delete all conversations one by one
      const ids = conversations.value.map((c) => c.id)
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
    const previousWarmupConvId = warmupConvId
    warmupConvId = convId
    if (previousWarmupConvId && previousWarmupConvId !== convId) {
      warmupApi.cancel(previousWarmupConvId).catch(() => {})
    }
    warmupApi.trigger(convId).catch(() => {})
  }

  function setWebSearchEnabled(enabled: boolean) {
    webSearchEnabled.value = enabled
    saveWebSearchEnabled(enabled)
    const convId = currentConversationId.value
    if (convId) {
      void patchCommandState(convId, { web_search_enabled: enabled }).catch(() => {})
    }
  }

  function setModelPreference(value: string) {
    const next = value.trim()
    modelPreference.value = next || 'auto'
    saveModelPreference(modelPreference.value)
    const parsed = splitModelPreference(modelPreference.value)
    if (parsed.selected_provider_id) {
      selectedProviderId.value = parsed.selected_provider_id
    }
    const convId = currentConversationId.value
    if (convId) {
      const patch: ConversationCommandStatePatch = {}
      if (!next || next === 'auto') {
        patch.selected_model_id = ''
      } else {
        patch.selected_model_id = parsed.selected_model_id || ''
        if (parsed.selected_provider_id) {
          patch.selected_provider_id = parsed.selected_provider_id
        }
      }
      void patchCommandState(convId, patch).catch(() => {})
    }
  }

  function setDeepResearchEnabled(enabled: boolean) {
    deepResearchEnabled.value = enabled
    saveDeepResearchEnabled(enabled)
    const convId = currentConversationId.value
    if (convId) {
      void patchCommandState(convId, { deep_research_enabled: enabled }).catch(() => {})
    }
  }

  // Reset warmup tracking (call when conversation changes)
  function resetWarmup() {
    const convId = warmupConvId
    warmupConvId = null
    if (convId) {
      warmupApi.cancel(convId).catch(() => {})
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
    selectedProviderId,
    modelPreference,
    offlineMode,
    webSearchEnabled,
    deepResearchEnabled,

    // Computed
    currentConversation,
    sortedConversations,
    executingConversationIds,
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
