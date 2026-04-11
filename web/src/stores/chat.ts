import { defineStore } from 'pinia'
import { ref, shallowRef, computed, watch, triggerRef } from 'vue'
import type {
  Conversation,
  Message,
  SendMessageRequest,
  MessageStats,
  MessageAttachment,
  ConversationCommandState,
  ConversationActiveStreamState as ConversationActiveStreamSnapshot,
  ConversationCommandStatePatch,
  StreamChunk,
} from '@/api/chat'
import { chatBootstrapApi } from '@/api/chatBootstrap'
import type { ConversationBootstrapResponse } from '@/api/chatBootstrap'
import type { Decision, ExecDecision } from '@/api/approval'
import { SSEClient } from '@/utils/sse'
import type { SSEClientOptions } from '@/utils/sse'
import { i18n } from '@/i18n'
import { useSettingsStore } from './settings'
import { useProviderPoolStore } from './providerPool'
import {
  cloneProcessTrace,
  createProcessTraceItem,
  type ProcessTraceItem,
  type ProcessTraceStatus,
} from '@/utils/processTrace'
import { localizeResearchSurfaceTitle } from '@/utils/deepResearchText'
import { reportStartupMark } from '@/utils/startupTrace'

type PendingConfirmationSnapshot = Pick<
  ConversationBootstrapResponse,
  'pending_approval' | 'pending_question' | 'pending_exec_approval'
>

type ChatApiModule = typeof import('@/api/chat')
type ApprovalApiModule = typeof import('@/api/approval')
type ApiClientModule = typeof import('@/api/client')
type SystemApiModule = typeof import('@/api/system')

let chatApiModulePromise: Promise<ChatApiModule> | null = null
let approvalApiModulePromise: Promise<ApprovalApiModule> | null = null
let apiClientModulePromise: Promise<ApiClientModule> | null = null
let systemApiModulePromise: Promise<SystemApiModule> | null = null

function createLazyApiProxy<T extends object>(load: () => Promise<T>): T {
  return new Proxy(
    {},
    {
      get(_target, prop) {
        return async (...args: unknown[]) => {
          const api = await load()
          const value = Reflect.get(api as object, prop)
          if (typeof value !== 'function') {
            return value
          }
          return Reflect.apply(value as (...callArgs: unknown[]) => unknown, api, args)
        }
      },
    }
  ) as T
}

async function loadChatApiModule() {
  if (!chatApiModulePromise) {
    chatApiModulePromise = import('@/api/chat')
  }
  return await chatApiModulePromise
}

async function loadApprovalApi() {
  if (!approvalApiModulePromise) {
    approvalApiModulePromise = import('@/api/approval')
  }
  return (await approvalApiModulePromise).approvalApi
}

async function loadApiClient() {
  if (!apiClientModulePromise) {
    apiClientModulePromise = import('@/api/client')
  }
  return (await apiClientModulePromise).default
}

async function loadSystemApi() {
  if (!systemApiModulePromise) {
    systemApiModulePromise = import('@/api/system')
  }
  return (await systemApiModulePromise).systemApi
}

const conversationApi = createLazyApiProxy<ChatApiModule['conversationApi']>(async () => {
  const module = await loadChatApiModule()
  return module.conversationApi
})
const messageApi = createLazyApiProxy<ChatApiModule['messageApi']>(async () => {
  const module = await loadChatApiModule()
  return module.messageApi
})
const warmupApi = createLazyApiProxy<ChatApiModule['warmupApi']>(async () => {
  const module = await loadChatApiModule()
  return module.warmupApi
})
const injectionApi = createLazyApiProxy<ChatApiModule['injectionApi']>(async () => {
  const module = await loadChatApiModule()
  return module.injectionApi
})
const approvalApi = createLazyApiProxy<ApprovalApiModule['approvalApi']>(loadApprovalApi)
const api = createLazyApiProxy<ApiClientModule['default']>(loadApiClient)
const systemApi = createLazyApiProxy<SystemApiModule['systemApi']>(loadSystemApi)

const PAGE_SIZE = 50
const CHAT_MODEL_PREF_KEY = 'chat.modelPreference'
const CHAT_OFFLINE_MODE_KEY = 'chat.offlineMode'
const STREAM_PROCESS_CARD_TYPES = new Set([
  'ui-review-progress',
  'analyze-progress',
  'browser-progress',
  'deep-research-progress',
  'deep-research-event',
  'deep-research-timeline',
])

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

function extractTypelessBlocksByType(
  content: string
): Array<{ raw: string; type: string; key: string }> {
  if (!content.includes('```typeless')) return []

  const blocks: Array<{ raw: string; type: string; key: string }> = []
  const blockRegex = /```typeless\s*\n([\s\S]*?)\n```/g
  let match: RegExpExecArray | null = null

  while ((match = blockRegex.exec(content)) !== null) {
    const raw = match[0]?.trim()
    const payload = match[1]?.trim()
    if (!raw || !payload) continue

    try {
      const parsed = JSON.parse(payload) as { type?: unknown; id?: unknown }
      const type = typeof parsed.type === 'string' ? parsed.type.trim() : ''
      if (!type) continue
      const id = typeof parsed.id === 'string' ? parsed.id.trim() : ''
      blocks.push({ raw, type, key: id ? `${type}:${id}` : raw })
    } catch {
      continue
    }
  }

  return blocks
}

function dedupeBlocksByLastKey(
  blocks: Array<{ raw: string; type: string; key: string }>
): Array<{ raw: string; type: string; key: string }> {
  const seen = new Set<string>()
  const deduped: Array<{ raw: string; type: string; key: string }> = []
  for (let index = blocks.length - 1; index >= 0; index -= 1) {
    const block = blocks[index]
    if (!block || seen.has(block.key)) continue
    seen.add(block.key)
    deduped.unshift(block)
  }
  return deduped
}

function mergeMissingProcessCardsIntoFinalContent(
  previousContent: string,
  nextContent: string,
  finalizationMode?: StreamChunk['finalization_mode']
): string {
  const normalizedPrevious = previousContent.trim()
  const normalizedNext = nextContent.trim()
  if (!normalizedPrevious || !normalizedNext) return nextContent
  if (finalizationMode === 'replace') return nextContent

  const previousProcessBlocks = dedupeBlocksByLastKey(
    extractTypelessBlocksByType(normalizedPrevious).filter((block) =>
      STREAM_PROCESS_CARD_TYPES.has(block.type)
    )
  )
  if (previousProcessBlocks.length === 0) {
    return nextContent
  }

  const nextProcessKeys = new Set(
    extractTypelessBlocksByType(normalizedNext)
      .filter((block) => STREAM_PROCESS_CARD_TYPES.has(block.type))
      .map((block) => block.key)
  )
  const missingBlocks = previousProcessBlocks.filter((block) => !nextProcessKeys.has(block.key))
  if (missingBlocks.length === 0) {
    return nextContent
  }

  const mergedPrefix = missingBlocks.map((block) => block.raw).join('\n\n')
  if (!mergedPrefix) {
    return nextContent
  }

  return `${mergedPrefix}\n\n${nextContent}`
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
    let parsedArgs: Record<string, unknown> | null = null
    if (r.args) {
      try {
        const parsed = JSON.parse(r.args) as Record<string, unknown>
        parsedArgs = parsed
        const commandCandidate = [
          parsed.command,
          parsed.cmd,
          parsed.query,
          parsed.url,
          parsed.href,
          parsed.path,
          parsed.name,
          parsed.action,
          parsed.sq,
          parsed.mq,
        ].find((value) => typeof value === 'string' && value.trim())
        command = typeof commandCandidate === 'string' ? commandCandidate.trim() : ''
      } catch {
        // If args is not valid JSON, use it directly for ask_user_question
        if (r.name === 'ask') {
          command = r.args
        } else {
          command = r.args.slice(0, 80)
        }
      }
    }
    if (r.name === 'convert' && parsedArgs) {
      const rawSources = Array.isArray(parsedArgs.sources) ? parsedArgs.sources : []
      const sources = rawSources
        .map((item) => (typeof item === 'string' ? item.trim() : ''))
        .filter(Boolean)
      const inputPath =
        typeof parsedArgs.input_path === 'string'
          ? parsedArgs.input_path.trim()
          : typeof parsedArgs.inputPath === 'string'
            ? parsedArgs.inputPath.trim()
            : typeof parsedArgs.input === 'string'
              ? parsedArgs.input.trim()
              : ''
      const outputPath =
        typeof parsedArgs.output_path === 'string'
          ? parsedArgs.output_path.trim()
          : typeof parsedArgs.outputPath === 'string'
            ? parsedArgs.outputPath.trim()
            : typeof parsedArgs.output === 'string'
              ? parsedArgs.output.trim()
              : ''
      const targetFormat =
        typeof parsedArgs.target_format === 'string'
          ? parsedArgs.target_format.trim()
          : typeof parsedArgs.targetFormat === 'string'
            ? parsedArgs.targetFormat.trim()
            : ''
      const primarySource = inputPath || sources[0] || ''
      if (primarySource) {
        const sourceLabel =
          sources.length > 1 && !inputPath
            ? `${primarySource} +${sources.length - 1} more`
            : primarySource
        if (outputPath) {
          command = `${sourceLabel} -> ${outputPath}`
        } else {
          command = targetFormat ? `${sourceLabel} -> ${targetFormat}` : sourceLabel
        }
      } else if (outputPath && command) {
        command = `${command} -> ${outputPath}`
      } else if (sources.length > 0) {
        const firstSource = sources[0] ?? ''
        const sourceLabel =
          sources.length === 1 ? firstSource : `${firstSource} +${sources.length - 1} more`
        command = targetFormat ? `${sourceLabel} -> ${targetFormat}` : sourceLabel
      } else if (targetFormat && command) {
        command = `${command} -> ${targetFormat}`
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

function resolveI18nText(
  key: string,
  fallback: string,
  named?: Record<string, string | number>
): string {
  const t = i18n.global.t
  const te = i18n.global.te
  return te(key) ? String(named ? t(key, named) : t(key)) : fallback
}

function resolveProcessTraceText(
  key: string,
  fallback: string,
  named?: Record<string, string | number>
): string {
  return resolveI18nText(`chat.processTrace.${key}`, fallback, named)
}

function resolveProcessTraceField(key: string, fallback: string): string {
  if (key === 'deepResearch') {
    return (
      resolveI18nText('chat.processTrace.fields.deepResearch', '') ||
      localizeResearchSurfaceTitle((translationKey, translationFallback) =>
        resolveI18nText(translationKey, translationFallback)
      )
    )
  }
  return resolveProcessTraceText(`fields.${key}`, fallback)
}

function isFixedModelPreferenceValue(value: string): boolean {
  const trimmed = value.trim()
  return trimmed !== '' && trimmed !== 'auto'
}

function isModelUnavailableErrorText(message: string): boolean {
  const normalized = String(message || '').trim()
  if (!normalized) return false

  const lower = normalized.toLowerCase()
  return (
    lower.includes('no available ai provider for model') ||
    lower.includes('model unavailable') ||
    lower.includes('model is not available') ||
    lower.includes('model not configured') ||
    lower.includes('model not found') ||
    lower.includes('unknown model') ||
    lower.includes('model_not_found') ||
    normalized.includes('无可用渠道') ||
    normalized.includes('未配置') ||
    normalized.includes('未启用') ||
    normalized.includes('模型不存在') ||
    normalized.includes('未找到模型')
  )
}

const providerFailoverConfirmationRequiredError = 'provider_failover_confirmation_required'

function isProviderFailoverConfirmationErrorText(message: string): boolean {
  const normalized = String(message || '')
    .trim()
    .toLowerCase()
  if (!normalized) return false
  return normalized.includes(providerFailoverConfirmationRequiredError)
}

function resolveProcessTraceDetail(_event: string, detail?: string): string {
  const normalized = detail?.trim() || ''
  if (!normalized) return ''

  switch (normalized) {
    case 'Using the resolved upstream provider and model for this response.':
      return resolveProcessTraceText(
        'details.providerResolved',
        'This response will use the selected upstream provider and model.'
      )
    case 'Switched to an available upstream model for this response.':
      return resolveProcessTraceText(
        'details.providerResolvedModelSwitch',
        'This response switched to an available upstream model.'
      )
    case 'Switched to another available upstream provider or model for this response.':
      return resolveProcessTraceText(
        'details.providerResolvedFailoverSwitch',
        'This response switched to another available upstream provider or model.'
      )
    case 'Retrying the tool follow-up without the previously pinned provider.':
      return resolveProcessTraceText(
        'details.providerFailoverToolFollowUp',
        'Retrying the tool follow-up without the previously pinned provider.'
      )
    case 'Retrying the continuation follow-up without the previously pinned provider.':
      return resolveProcessTraceText(
        'details.providerFailoverContinuationFollowUp',
        'Retrying the continuation follow-up without the previously pinned provider.'
      )
    case 'silent_recovery_stage1':
      return resolveProcessTraceText('details.recoveryStage1', 'Silent recovery')
    case 'stage2_reduced_payload':
      return resolveProcessTraceText('details.recoveryStage2', 'Reduced recovery payload')
    default:
      return normalized
  }
}

function formatProcessTraceSeconds(delayMs?: number): string {
  if (typeof delayMs !== 'number' || !Number.isFinite(delayMs)) return '0'
  const seconds = Math.max(0, Math.round(delayMs / 100) / 10)
  const locale =
    String((i18n.global as { locale?: { value?: string } }).locale?.value || 'en-US') || 'en-US'
  const hasFraction = Math.abs(seconds - Math.round(seconds)) >= 0.05
  return new Intl.NumberFormat(locale, {
    maximumFractionDigits: 1,
    minimumFractionDigits: hasFraction ? 1 : 0,
  }).format(seconds)
}

function resolveProcessTraceStatusLabel(item: ProcessTraceItem): string {
  switch (item.event) {
    case 'pre_content_retry_scheduled':
      return resolveProcessTraceText('events.retryScheduled', 'Retrying request in {seconds}s', {
        seconds: formatProcessTraceSeconds(
          typeof item.metadata?.delay_ms === 'number' ? item.metadata.delay_ms : undefined
        ),
      })
    case 'pre_content_retry_started':
      return resolveProcessTraceText('events.retryingRequest', 'Retrying request')
    case 'pre_content_retry_succeeded':
      return resolveProcessTraceText('events.retrySucceeded', 'Retry succeeded')
    case 'pre_content_retry_failed':
      return resolveProcessTraceText('events.retryFailed', 'Retry failed')
    case 'continuation_recovery_started':
      return resolveProcessTraceText('events.recoveringResponse', 'Recovering response')
    case 'continuation_recovery_succeeded':
      return resolveProcessTraceText('events.recoverySucceeded', 'Recovery succeeded')
    case 'continuation_recovery_failed':
      return resolveProcessTraceText('events.recoveryFailed', 'Recovery failed')
    case 'provider_failover':
      if (item.status === 'pending' && item.metadata?.requires_confirmation) {
        return resolveProcessTraceText(
          'events.waitingForSwitchConfirmation',
          'Waiting for switch confirmation'
        )
      }
      if (item.status === 'success') {
        return resolveProcessTraceText(
          'events.providerSwitchSucceeded',
          'Provider switch succeeded'
        )
      }
      if (item.status === 'error') {
        return resolveProcessTraceText('events.providerSwitchFailed', 'Provider switch failed')
      }
      return resolveProcessTraceText('events.switchingProvider', 'Switching provider')
    case 'provider_resolved':
      return resolveProcessTraceText('events.providerResolved', 'Using available route')
    case 'injection_restart':
      return resolveProcessTraceText(
        'events.restartingWithLatestMessage',
        'Restarting with your latest message'
      )
    case 'request_summary':
      return resolveProcessTraceText('events.requestReady', 'Request ready')
    case 'request_dispatched':
      return resolveProcessTraceText('events.requestSent', 'Request sent')
    case 'waiting_for_response':
      return resolveProcessTraceText('events.waitingForResponse', 'Waiting for response')
    case 'awaiting_confirmation':
      return resolveProcessTraceText('events.waitingForConfirmation', 'Waiting for confirmation')
    case 'network_interrupt_waiting':
      return resolveProcessTraceText(
        'events.waitingForConnectionRecovery',
        'Waiting for connection recovery'
      )
    default:
      if (item.label.trim()) return item.label
      return resolveProcessTraceText('events.processing', 'Processing')
  }
}

// Store for message metadata (provider, model, stats) - keyed by message ID
const messageMetadata = ref<
  Map<string, { provider?: string; model?: string; stats?: MessageStats }>
>(new Map())

export type StreamUIPhase =
  | 'idle'
  | 'connecting'
  | 'streaming'
  | 'executing'
  | 'recovering'
  | 'awaiting_confirmation'
  | 'interrupted'
  | 'completed'

export interface StreamUIState {
  phase: StreamUIPhase
  label: string | null
  detail: string | null
  updatedAt: number
  recoveryAttempt: number
  canRetry: boolean
}

interface StreamRecoveryBaseline {
  assistantId: string | null
  assistantContent: string
  assistantCount: number
  assistantMetaKey: string
}

interface ActiveConversationStreamState {
  conversationId: string
  streamId: string | null
  sending: boolean
  streaming: boolean
  receivedFirstChunk: boolean
  providerAccelerationActive: boolean
  streamProgress: string | null
  toolExecuting: boolean
  toolExecutingStartTime: number
  toolExecutingNames: string[]
  toolExecutingCommands: string[]
  toolSandboxAvailable: boolean
  awaitingConfirmation: boolean
  previewContent: string
  processContentLength: number
  toolResults: ToolResultItem[]
  processTrace: ProcessTraceItem[]
  statusStartedAt: number
  statusSummary: string | null
  uiState: StreamUIState
  recoveryBaseline: StreamRecoveryBaseline
}

export interface ActiveMessageStreamState {
  phase: StreamUIPhase
  awaitingConfirmation: boolean
  toolExecuting: boolean
  toolExecutingCommands: string[]
  toolExecutingNames: string[]
  toolSandboxAvailable: boolean
  statusSummary: string | null
  streamProgress: string | null
  processTrace: ProcessTraceItem[]
  toolResults: ToolResultItem[]
  statusStartedAt: number
  showExternalStatusRail: boolean
}

type SendMessageFileAttachment = {
  id: string
  file: File
  name: string
  size: number
  type: string
  preview?: string
  duration?: number
}

interface SendMessageOptions {
  existingAttachments?: MessageAttachment[]
  skipConversationCreate?: boolean
}

type ModelAutoFallbackRetryKind = 'send' | 'continue' | 'regenerate'

export interface PendingModelAutoFallback {
  conversationId: string
  retryKind: ModelAutoFallbackRetryKind
  requestedModelId: string
  requestedProviderId?: string
  errorMessage: string
}

interface PendingProviderFailoverDraft {
  conversationId: string
  failedProviderId?: string
  detail: string
  retryAttempts?: number
}

export interface PendingProviderFailoverRetry extends PendingProviderFailoverDraft {
  retryKind: ModelAutoFallbackRetryKind
  errorMessage: string
}

type RuntimeProcessMessage = Message & {
  local_process_tool_results?: ToolResultItem[]
}

function createStreamUIState(
  phase: StreamUIPhase,
  overrides: Partial<StreamUIState> = {}
): StreamUIState {
  return {
    phase,
    label: overrides.label ?? null,
    detail: overrides.detail ?? null,
    updatedAt: overrides.updatedAt ?? Date.now(),
    recoveryAttempt: overrides.recoveryAttempt ?? 0,
    canRetry: overrides.canRetry ?? false,
  }
}

export const useChatStore = defineStore('chat', () => {
  const settingsStore = useSettingsStore()
  const providerPoolStore = useProviderPoolStore()

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

  function getEnabledChatProviderIds(): Set<string> {
    return new Set(
      (providerPoolStore.enabledProviders || [])
        .filter((provider) => provider.type !== 'media')
        .map((provider) => provider.id)
    )
  }

  function hasLoadedChatProviderInventory(): boolean {
    return (providerPoolStore.providers || []).some((provider) => provider.type !== 'media')
  }

  function normalizeAgentcoreRunnerRefValue(value: unknown): string {
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
    const fallback = settingsStore.experimentalAgentcoreRunnerRef?.trim()
    return fallback || 'main'
  }

  function hasConversationScopedAgentcoreRunnerRef(value: unknown): boolean {
    return (
      settingsStore.experimentalAgentcoreRunnerEnabled &&
      normalizeAgentcoreRunnerRefValue(value) !==
        normalizeAgentcoreRunnerRefValue(settingsStore.experimentalAgentcoreRunnerRef)
    )
  }

  function normalizeEnabledChatProviderId(providerIdRaw: string): string {
    const providerId = providerIdRaw.trim()
    if (!providerId) return ''
    const enabledProviderIds = getEnabledChatProviderIds()
    if (enabledProviderIds.size === 0 && !hasLoadedChatProviderInventory()) {
      return providerId
    }
    return enabledProviderIds.has(providerId) ? providerId : ''
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

  function resolveCommandStateSelection(state: ConversationCommandState) {
    const selectedProviderId = state.selected_provider_id?.trim() || ''
    const selectedModelId = state.selected_model_id?.trim() || ''
    if (selectedProviderId && !selectedModelId) {
      const normalizedProviderId = normalizeEnabledChatProviderId(selectedProviderId)
      if (!normalizedProviderId) {
        return {
          selected_provider_id: '',
          selected_model_id: '',
          model_preference: 'auto',
        }
      }
      return {
        selected_provider_id: normalizedProviderId,
        selected_model_id: '',
        model_preference: 'auto',
      }
    }
    return resolveModelSelection(selectedProviderId, commandStateToModelPreference(state))
  }

  function resolveModelSelection(providerIdRaw: string, modelPreferenceRaw: string) {
    const providerId = providerIdRaw.trim()
    const trimmed = modelPreferenceRaw.trim()
    if (!trimmed || trimmed === 'auto') {
      return {
        selected_provider_id: '',
        selected_model_id: '',
        model_preference: 'auto',
      }
    }

    const parsed = splitModelPreference(trimmed)
    const requestedProviderId = parsed.selected_provider_id || providerId
    const requestedModelId = parsed.selected_model_id || trimmed

    if (!requestedModelId) {
      return {
        selected_provider_id: requestedProviderId,
        selected_model_id: '',
        model_preference: 'auto',
      }
    }

    const enabledProviderIds = getEnabledChatProviderIds()
    const normalizedRequestedProviderId = normalizeEnabledChatProviderId(requestedProviderId)
    const enabledModels = (providerPoolStore.models || []).filter(
      (model) => model.enabled && enabledProviderIds.has(model.provider_id)
    )

    const exactMatch = enabledModels.find(
      (model) =>
        model.provider_id === normalizedRequestedProviderId && model.id === requestedModelId
    )
    if (exactMatch) {
      return {
        selected_provider_id: exactMatch.provider_id,
        selected_model_id: exactMatch.id,
        model_preference: `${exactMatch.provider_id}/${exactMatch.id}`,
      }
    }

    const providerScopedSuffixMatch = enabledModels.find(
      (model) =>
        model.provider_id === normalizedRequestedProviderId &&
        model.id.endsWith(`/${requestedModelId}`)
    )
    if (providerScopedSuffixMatch) {
      return {
        selected_provider_id: providerScopedSuffixMatch.provider_id,
        selected_model_id: providerScopedSuffixMatch.id,
        model_preference: `${providerScopedSuffixMatch.provider_id}/${providerScopedSuffixMatch.id}`,
      }
    }

    const exactModelMatches = enabledModels.filter((model) => model.id === requestedModelId)
    if (exactModelMatches.length === 1) {
      const match = exactModelMatches[0]
      if (!match) {
        return {
          selected_provider_id: requestedProviderId,
          selected_model_id: requestedModelId,
          model_preference: requestedProviderId
            ? `${requestedProviderId}/${requestedModelId}`
            : requestedModelId,
        }
      }
      return {
        selected_provider_id: match.provider_id,
        selected_model_id: match.id,
        model_preference: `${match.provider_id}/${match.id}`,
      }
    }

    const suffixMatches = enabledModels.filter((model) => model.id.endsWith(`/${requestedModelId}`))
    if (suffixMatches.length === 1) {
      const match = suffixMatches[0]
      if (!match) {
        return {
          selected_provider_id: requestedProviderId,
          selected_model_id: requestedModelId,
          model_preference: requestedProviderId
            ? `${requestedProviderId}/${requestedModelId}`
            : requestedModelId,
        }
      }
      return {
        selected_provider_id: match.provider_id,
        selected_model_id: match.id,
        model_preference: `${match.provider_id}/${match.id}`,
      }
    }

    if (normalizedRequestedProviderId) {
      return {
        selected_provider_id: normalizedRequestedProviderId,
        selected_model_id: requestedModelId,
        model_preference: `${normalizedRequestedProviderId}/${requestedModelId}`,
      }
    }

    return {
      selected_provider_id: '',
      selected_model_id: requestedModelId,
      model_preference: requestedModelId,
    }
  }

  function resolveRequestModelSelection(
    providerIdRaw: string,
    modelPreferenceRaw: string,
    preserveProviderPin = false
  ) {
    const resolved = resolveModelSelection(providerIdRaw, modelPreferenceRaw)
    if (preserveProviderPin && resolved.model_preference === 'auto') {
      const providerId = normalizeEnabledChatProviderId(providerIdRaw)
      if (providerId) {
        return {
          selected_provider_id: providerId,
          selected_model_id: '',
          model_preference: 'auto',
        }
      }
    }
    return resolved
  }

  function syncResolvedModelSelectionState(
    resolved: ReturnType<typeof resolveRequestModelSelection>
  ): boolean {
    const nextProviderId = resolved.selected_provider_id
    const nextModelPreference = resolved.model_preference
    const nextProviderPinOnlyActive =
      nextModelPreference === 'auto' && !!nextProviderId && !resolved.selected_model_id

    const changed =
      selectedProviderId.value !== nextProviderId ||
      modelPreference.value !== nextModelPreference ||
      providerPinOnlyActive.value !== nextProviderPinOnlyActive

    if (!changed) return false

    selectedProviderId.value = nextProviderId
    modelPreference.value = nextModelPreference
    providerPinOnlyActive.value = nextProviderPinOnlyActive
    saveModelPreference(nextModelPreference)
    return true
  }

  function getCurrentRequestModelSelection(options?: {
    conversationId?: string | null
    persistIfChanged?: boolean
  }) {
    const resolved = resolveRequestModelSelection(
      selectedProviderId.value,
      modelPreference.value,
      providerPinOnlyActive.value
    )
    const changed = syncResolvedModelSelectionState(resolved)
    const conversationId = normalizeConversationId(options?.conversationId)
    if (changed && options?.persistIfChanged && conversationId) {
      void patchCommandState(conversationId, {
        selected_provider_id: resolved.selected_provider_id,
        selected_model_id: resolved.selected_model_id,
      }).catch(() => {})
    }
    return resolved
  }

  // State
  const conversations = ref<Conversation[]>([])
  const currentConversationId = ref<string | null>(null)
  // Use shallowRef for messages to reduce reactivity overhead
  // Manual triggerRef() calls are needed when mutating the array
  const messages = shallowRef<Message[]>([])
  const recentTodoCompletion = ref<{ messageId: string; todoCardId?: string } | null>(null)
  const loading = ref(false)
  const sending = ref(false)
  const streaming = ref(false)
  const streamingContent = ref('')
  const processContentLength = ref(0) // Length of tool-result process content at the start of streamingContent
  const error = ref<string | null>(null)
  const streamError = ref<string | null>(null) // Error from stream (displayed in chat area)
  const streamUIState = ref<StreamUIState>(createStreamUIState('idle'))
  const streamProgress = ref<string | null>(null) // Upstream metadata progress before first visible delta
  const providerAccelerationActive = ref(false)
  const statusStartedAt = ref(0)
  const statusSummary = ref<string | null>(null)
  const processTrace = ref<ProcessTraceItem[]>([])
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
    type: 'compacting' | 'pruned' | 'compacted'
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
    session_id?: string
    binding_hash?: string
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
      site_origin?: string
      screenshot?: { mime_type?: string; data?: string; url?: string }
    }
    require_explicit_answer?: boolean
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
    binding_hash?: string
    expires_at: number
  } | null>(null)

  const isMultiSelectMode = ref(false)
  const selectedProviderId = ref<string>('')
  const modelPreference = ref<string>(loadModelPreference())
  const providerPinOnlyActive = ref(false)
  const pendingModelAutoFallback = ref<PendingModelAutoFallback | null>(null)
  const pendingProviderFailoverDraft = ref<PendingProviderFailoverDraft | null>(null)
  const pendingProviderFailoverRetry = ref<PendingProviderFailoverRetry | null>(null)
  const offlineMode = ref<boolean>(loadOfflineMode())
  const agentcoreRunnerRef = ref<string>(
    normalizeAgentcoreRunnerRefValue(settingsStore.experimentalAgentcoreRunnerRef)
  )
  const activeStreamId = ref<string | null>(null)
  const activeStreamState = ref<ActiveConversationStreamState | null>(null)
  const commandStateHydrated = ref(false)
  let commandStateHydratedConversationId: string | null = null
  let commandStateHydratePromise: Promise<ConversationCommandState> | null = null
  let commandStateHydratePromiseConversationId: string | null = null

  // SSE client for streaming
  const sseClient = new SSEClient()
  const pendingRecoveryRetryDelayMs = 700
  const pendingRecoveryRetryLimit = 8
  let pendingRecoveryRetryCount = 0
  let pendingRecoveryRetryTimer: ReturnType<typeof setTimeout> | null = null
  const pendingConfirmationRecoveryChecks = new Set<string>()
  const pendingConfirmationRecoveryRequests = new Map<string, Promise<void>>()
  const streamRecoveryRetryDelayMs = 900
  const streamRecoveryRetryLimit = 4
  let streamRecoveryTimer: ReturnType<typeof setTimeout> | null = null

  const isRecovering = computed(() => streamUIState.value.phase === 'recovering')
  const isStreamInterrupted = computed(() => streamUIState.value.phase === 'interrupted')

  function cloneToolResultItems(items?: ToolResultItem[]): ToolResultItem[] {
    if (!items || items.length === 0) return []
    return items.map((item) => ({ ...item }))
  }

  function formatBytes(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB']
    let value = bytes
    let unitIndex = 0
    while (value >= 1024 && unitIndex < units.length - 1) {
      value /= 1024
      unitIndex++
    }
    return `${value >= 10 || unitIndex === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unitIndex]}`
  }

  function estimateAttachmentBytes(attachments?: MessageAttachment[]): number {
    if (!attachments || attachments.length === 0) return 0
    return attachments.reduce((total, attachment) => {
      const dataLen = attachment.data?.length || 0
      return total + Math.max(0, Math.floor((dataLen * 3) / 4))
    }, 0)
  }

  function summarizeRequestText(value: string, maxLen = 120): string {
    const normalized = value.replace(/\s+/g, ' ').trim()
    if (!normalized) return ''
    if (normalized === '[CONTINUE]') {
      return resolveProcessTraceText(
        'summaryValues.continuePreviousReply',
        'Continue the previous reply'
      )
    }
    if (normalized === '[CONTINUE_AFTER_CANCEL]') {
      return resolveProcessTraceText(
        'summaryValues.resumePreviousRequest',
        'Resume the previous request'
      )
    }
    if (normalized.length <= maxLen) return normalized
    return normalized.slice(0, maxLen - 1) + '…'
  }

  function formatRequestSummaryDetail(request: SendMessageRequest): string {
    const lines: string[] = []
    const summary = summarizeRequestText(request.message)
    const attachments = request.attachments || []
    const attachmentTypes = Array.from(new Set(attachments.map((attachment) => attachment.type)))
    const attachmentBytes = estimateAttachmentBytes(attachments)
    const fieldMessage = resolveProcessTraceField('message', 'Message')
    const fieldProvider = resolveProcessTraceField('provider', 'Provider')
    const fieldModel = resolveProcessTraceField('model', 'Model')
    const fieldAttachments = resolveProcessTraceField('attachments', 'Attachments')
    const fieldAuto = resolveProcessTraceField('auto', 'Auto')
    const fieldFile = resolveProcessTraceField('file', 'file')
    const provider = request.provider?.trim() || fieldAuto
    const model = request.model?.trim() || fieldAuto

    if (summary) lines.push(`${fieldMessage}: ${summary}`)
    lines.push(`${fieldProvider}: ${provider}`)
    lines.push(`${fieldModel}: ${model}`)
    if (attachments.length > 0) {
      lines.push(
        `${fieldAttachments}: ${attachments.length} (${attachmentTypes.join(', ') || fieldFile}) · ${formatBytes(attachmentBytes)}`
      )
    }
    return lines.join('\n')
  }

  function createServerProcessTraceItem(chunk: StreamChunk): ProcessTraceItem | null {
    const event = chunk.process_event?.trim()
    if (!event) return null

    const attempt =
      typeof chunk.process_attempt === 'number' && Number.isFinite(chunk.process_attempt)
        ? chunk.process_attempt
        : undefined
    const delayMs =
      typeof chunk.process_delay_ms === 'number' && Number.isFinite(chunk.process_delay_ms)
        ? chunk.process_delay_ms
        : undefined
    const provider = chunk.process_provider?.trim()
    const model = chunk.process_model?.trim()
    const requiresConfirmation = chunk.process_requires_confirmation === true
    const retryAttempts =
      typeof chunk.process_retry_attempts === 'number' &&
      Number.isFinite(chunk.process_retry_attempts)
        ? chunk.process_retry_attempts
        : undefined
    const lastStatusCode =
      typeof chunk.process_last_status_code === 'number' &&
      Number.isFinite(chunk.process_last_status_code)
        ? chunk.process_last_status_code
        : undefined
    const category = event.startsWith('pre_content_retry')
      ? 'retry'
      : event.startsWith('continuation_recovery')
        ? 'recovery'
        : event === 'provider_failover'
          ? 'recovery'
          : 'lifecycle'
    const status = (chunk.process_status || 'info') as ProcessTraceStatus
    const details: string[] = []
    const attemptLabel = resolveProcessTraceField('attempt', 'Attempt')
    const delayLabel = resolveProcessTraceField('delay', 'Delay')
    const providerLabel = resolveProcessTraceField('provider', 'Provider')
    const modelLabel = resolveProcessTraceField('model', 'Model')

    const localizedProcessDetail = resolveProcessTraceDetail(event, chunk.process_detail)
    if (localizedProcessDetail) details.push(localizedProcessDetail)
    if (attempt !== undefined) details.push(`${attemptLabel}: ${attempt}`)
    if (delayMs !== undefined) details.push(`${delayLabel}: ${formatProcessTraceSeconds(delayMs)}s`)
    if (provider) details.push(`${providerLabel}: ${provider}`)
    if (model) details.push(`${modelLabel}: ${model}`)

    const fallbackLabel = chunk.process_message?.trim() || ''
    const metadata = {
      attempt,
      delay_ms: delayMs,
      provider,
      model,
      requires_confirmation: requiresConfirmation,
      retry_attempts: retryAttempts,
      last_status_code: lastStatusCode,
    }

    return createProcessTraceItem({
      source: 'server',
      event,
      category,
      status,
      label: resolveProcessTraceStatusLabel({
        id: '',
        source: 'server',
        event,
        category,
        status,
        label: fallbackLabel,
        timestamp: 0,
        metadata,
      }),
      detail: details.join('\n') || undefined,
      metadata,
    })
  }

  function extractHostLabelFromText(text: string): string {
    const match = text.match(/https?:\/\/[^\s"'`]+/i)
    if (!match) return ''
    try {
      return new URL(match[0]).hostname.replace(/^www\./, '')
    } catch {
      return ''
    }
  }

  function formatAssistantStatusSummaryFromTools(
    toolNames: string[] = [],
    toolCommands: string[] = []
  ): string {
    const t = i18n.global.t
    const te = i18n.global.te
    const resolve = (key: string, fallback: string, named?: Record<string, string>) =>
      te(key) ? String(named ? t(key, named) : t(key)) : fallback

    const normalizedNames = toolNames.map((value) => value.toLowerCase())
    const normalizedCommands = toolCommands.map((value) => value.toLowerCase())
    const combined = [...normalizedNames, ...normalizedCommands]
    const host =
      toolCommands.map((command) => extractHostLabelFromText(command)).find(Boolean) || ''

    const hasSearch = combined.some(
      (value) =>
        value.includes('search') ||
        value.includes('find') ||
        value.includes('query') ||
        value.includes('lookup')
    )
    const hasWebRead = combined.some(
      (value) =>
        value.includes('web_fetch') ||
        value.includes('browser') ||
        value.includes('navigate') ||
        value.includes('snapshot') ||
        value.includes('screenshot') ||
        value.includes('fetch') ||
        value.includes('open') ||
        value.includes('click')
    )

    if (host && hasWebRead) {
      return resolve('chat.assistantStatus.readingWebSite', `Reading ${host}`, { site: host })
    }
    if (hasSearch && hasWebRead) {
      return resolve('chat.assistantStatus.browsingWeb', 'Browsing the web')
    }
    if (hasSearch) {
      return resolve('chat.assistantStatus.searchingWeb', 'Searching the web')
    }
    if (hasWebRead) {
      return resolve('chat.assistantStatus.readingWeb', 'Reading a web page')
    }
    return resolve('chat.assistantStatus.usingTools', 'Using tools')
  }

  function resolveStreamUIStateLabel(
    phase: StreamUIPhase,
    fallback?: string | null
  ): string | null {
    if (fallback?.trim()) return fallback

    switch (phase) {
      case 'connecting':
        return formatStreamProgress('response.created')
      case 'streaming':
        return formatStreamProgress('response.output_text.delta')
      case 'executing':
        return resolveProcessTraceText('events.processing', 'Processing')
      case 'recovering':
        return resolveProcessTraceText(
          'events.waitingForConnectionRecovery',
          'Waiting for connection recovery'
        )
      case 'awaiting_confirmation':
        return resolveI18nText(
          'chat.awaitingConfirmation',
          'Waiting for your confirmation to continue'
        )
      case 'interrupted':
        return resolveI18nText('chat.responseInterrupted', 'Response interrupted')
      case 'completed':
        return formatStreamProgress('response.completed')
      default:
        return null
    }
  }

  function formatRecoveryAttemptDetail(attempt: number): string {
    return resolveProcessTraceText('details.recoveryAttempt', 'Attempt {current} of {total}', {
      current: attempt,
      total: streamRecoveryRetryLimit,
    })
  }

  function createConversationRecoveryBaseline(conversationId: string): StreamRecoveryBaseline {
    let assistantId: string | null = null
    let assistantContent = ''
    let assistantMetaKey = ''
    let assistantCount = 0

    for (const message of messages.value) {
      if (
        !message ||
        message.conversation_id !== conversationId ||
        message.role !== 'assistant' ||
        message.id.startsWith('streaming-')
      ) {
        continue
      }
      assistantCount++
      assistantId = message.id
      assistantContent = message.content || ''
      assistantMetaKey = [
        message.provider || '',
        message.model || '',
        message.stats?.latency_ms || 0,
        message.stats?.output_tokens || 0,
      ].join(':')
    }

    return {
      assistantId,
      assistantContent,
      assistantCount,
      assistantMetaKey,
    }
  }

  function recoveryBaselineAdvanced(
    baseline: StreamRecoveryBaseline,
    next: StreamRecoveryBaseline
  ): boolean {
    if (next.assistantCount > baseline.assistantCount) return true
    if (next.assistantId !== baseline.assistantId) return true
    if (next.assistantContent !== baseline.assistantContent) return true
    return next.assistantMetaKey !== baseline.assistantMetaKey
  }

  function clearStreamRecoveryTimer() {
    if (!streamRecoveryTimer) return
    clearTimeout(streamRecoveryTimer)
    streamRecoveryTimer = null
  }

  function getActiveStreamState(
    conversationId?: string | null
  ): ActiveConversationStreamState | null {
    const state = activeStreamState.value
    if (!state) return null
    if (conversationId && state.conversationId !== conversationId) return null
    return state
  }

  function upsertProcessTraceItem(
    conversationId: string,
    item: ProcessTraceItem,
    options?: { replaceLatestByEvent?: boolean; updateStatusTimer?: boolean }
  ) {
    const current = getActiveStreamState(conversationId)
    if (!current) return

    const nextTrace = cloneProcessTrace(current.processTrace)
    if (options?.replaceLatestByEvent) {
      for (let i = nextTrace.length - 1; i >= 0; i--) {
        const existing = nextTrace[i]
        if (existing && existing.event === item.event) {
          nextTrace[i] = item
          updateActiveStreamState(conversationId, {
            processTrace: nextTrace,
            statusStartedAt: options.updateStatusTimer ? item.timestamp : current.statusStartedAt,
          })
          return
        }
      }
    }

    nextTrace.push(item)
    updateActiveStreamState(conversationId, {
      processTrace: nextTrace,
      statusStartedAt: options?.updateStatusTimer ? item.timestamp : current.statusStartedAt,
    })
  }

  function addLocalProcessTrace(
    conversationId: string,
    item: Omit<ProcessTraceItem, 'id' | 'timestamp'>,
    options?: { replaceLatestByEvent?: boolean; updateStatusTimer?: boolean }
  ) {
    upsertProcessTraceItem(conversationId, createProcessTraceItem(item), options)
  }

  function addRequestProcessTrace(conversationId: string, request: SendMessageRequest) {
    addLocalProcessTrace(conversationId, {
      source: 'client',
      event: 'request_summary',
      category: 'summary',
      status: 'info',
      label: resolveProcessTraceText('events.requestReady', 'Request ready'),
      command: summarizeRequestText(request.message),
      detail: formatRequestSummaryDetail(request),
    })
    addLocalProcessTrace(
      conversationId,
      {
        source: 'client',
        event: 'request_dispatched',
        category: 'lifecycle',
        status: 'active',
        label: resolveProcessTraceText('events.requestSent', 'Request sent'),
        detail: resolveProcessTraceText(
          'details.requestDispatched',
          'Waiting for the server to accept and start the response.'
        ),
      },
      { replaceLatestByEvent: true, updateStatusTimer: true }
    )
    addLocalProcessTrace(
      conversationId,
      {
        source: 'client',
        event: 'waiting_for_response',
        category: 'lifecycle',
        status: 'active',
        label: resolveProcessTraceText('events.waitingForResponse', 'Waiting for response'),
        detail: resolveProcessTraceText(
          'details.waitingForResponse',
          'The request was accepted. Waiting for the first visible output.'
        ),
      },
      { replaceLatestByEvent: true }
    )
  }

  function applyVisibleStreamState(state: ActiveConversationStreamState | null) {
    sending.value = !!state?.sending
    streaming.value = !!state?.streaming
    activeStreamId.value = state?.streamId ?? null
    streamUIState.value = state?.uiState ?? createStreamUIState('idle')
    streamProgress.value = state?.receivedFirstChunk ? null : (state?.streamProgress ?? null)
    providerAccelerationActive.value =
      !!state?.providerAccelerationActive && !state?.receivedFirstChunk
    statusStartedAt.value = state?.statusStartedAt ?? 0
    statusSummary.value = state?.statusSummary ?? null
    processTrace.value = cloneProcessTrace(state?.processTrace)
    _receivedFirstChunk.value = !!state?.receivedFirstChunk
    processContentLength.value = state?.processContentLength ?? 0
    toolResults.value = cloneToolResultItems(state?.toolResults)

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

  function updateActiveStreamUIState(
    conversationId: string,
    phase: StreamUIPhase,
    overrides: Partial<StreamUIState> = {}
  ) {
    const current = getActiveStreamState(conversationId)
    if (!current) return
    const nextLabel = resolveStreamUIStateLabel(phase, overrides.label ?? current.uiState.label)
    updateActiveStreamState(conversationId, {
      uiState: createStreamUIState(phase, {
        ...current.uiState,
        ...overrides,
        label: nextLabel,
      }),
    })
  }

  function beginActiveStream(conversationId: string) {
    const next: ActiveConversationStreamState = {
      conversationId,
      streamId: null,
      sending: true,
      streaming: true,
      receivedFirstChunk: false,
      providerAccelerationActive: false,
      streamProgress: null,
      toolExecuting: false,
      toolExecutingStartTime: 0,
      toolExecutingNames: [],
      toolExecutingCommands: [],
      toolSandboxAvailable: false,
      awaitingConfirmation: false,
      previewContent: '',
      processContentLength: 0,
      toolResults: [],
      processTrace: [],
      statusStartedAt: Date.now(),
      statusSummary: null,
      uiState: createStreamUIState('connecting', {
        label: resolveStreamUIStateLabel('connecting'),
      }),
      recoveryBaseline: createConversationRecoveryBaseline(conversationId),
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
      toolResults: patch.toolResults
        ? cloneToolResultItems(patch.toolResults)
        : current.toolResults,
      processTrace: patch.processTrace
        ? cloneProcessTrace(patch.processTrace)
        : current.processTrace,
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
    const nextSummary = formatStreamProgress('response.output_text.delta')
    updateActiveStreamState(conversationId, {
      previewContent: current.previewContent + delta,
      receivedFirstChunk: true,
      providerAccelerationActive: false,
      streamProgress: null,
      toolExecuting: false,
      statusSummary: nextSummary,
      statusStartedAt: current.statusSummary === nextSummary ? current.statusStartedAt : Date.now(),
    })
    updateActiveStreamUIState(conversationId, 'streaming', {
      label: nextSummary,
      detail: null,
      canRetry: false,
    })
  }

  function resetActiveStreamRound(conversationId: string) {
    updateActiveStreamState(conversationId, {
      receivedFirstChunk: false,
      providerAccelerationActive: false,
      streamProgress: null,
      toolExecuting: false,
      toolExecutingStartTime: 0,
      toolExecutingNames: [],
      toolExecutingCommands: [],
      toolSandboxAvailable: false,
      awaitingConfirmation: false,
      previewContent: '',
      processContentLength: 0,
      toolResults: [],
      processTrace: [],
      statusStartedAt: Date.now(),
      statusSummary: null,
    })
    updateActiveStreamUIState(conversationId, 'connecting', {
      label: resolveStreamUIStateLabel('connecting'),
      detail: null,
      recoveryAttempt: 0,
      canRetry: false,
    })
  }

  function clearActiveStreamState(conversationId?: string | null) {
    const current = activeStreamState.value
    if (!current) return
    if (conversationId && current.conversationId !== conversationId) return
    clearStreamRecoveryTimer()
    activeStreamState.value = null
  }

  function clearVisibleStreamState() {
    sending.value = false
    streaming.value = false
    streamUIState.value = createStreamUIState('idle')
    streamProgress.value = null
    providerAccelerationActive.value = false
    statusStartedAt.value = 0
    statusSummary.value = null
    processTrace.value = []
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
    if (!state || currentConversationId.value !== conversationId) return

    resetPendingStreamDelta()
    const previewContent = state.previewContent || ''
    let lastMessage: Message | undefined
    for (let i = messages.value.length - 1; i >= 0; i--) {
      const candidate = messages.value[i]
      if (candidate?.role === 'assistant' && candidate.conversation_id === conversationId) {
        lastMessage = candidate
        break
      }
    }
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
    } else if (
      previewContent ||
      state.toolResults.length > 0 ||
      state.processTrace.length > 0 ||
      state.toolExecuting ||
      state.awaitingConfirmation ||
      state.statusSummary ||
      state.streamProgress
    ) {
      const assistantMessage = createStreamingAssistantMessage(conversationId)
      assistantMessage.content = previewContent
      messages.value = [...messages.value, assistantMessage]
    }

    streamingContent.value = previewContent
    applyVisibleStreamState(state)
  }

  function restoreServerActiveStreamState(
    conversationId: string,
    serverState?: ConversationActiveStreamSnapshot | null
  ): boolean {
    if (!serverState?.active || currentConversationId.value !== conversationId) return false
    if (getActiveStreamState(conversationId)) return true

    const lastMessage = messages.value[messages.value.length - 1]
    const previewContent =
      lastMessage?.role === 'assistant' && lastMessage.conversation_id === conversationId
        ? lastMessage.content || ''
        : ''

    const hasPreview = previewContent.trim().length > 0
    const phase: StreamUIPhase = hasPreview ? 'streaming' : 'executing'
    const statusLabel =
      resolveStreamUIStateLabel(phase) || resolveProcessTraceText('events.processing', 'Processing')
    const nextState: ActiveConversationStreamState = {
      conversationId,
      streamId: serverState.stream_id?.trim() || null,
      sending: false,
      streaming: true,
      receivedFirstChunk: hasPreview,
      providerAccelerationActive: false,
      streamProgress: null,
      toolExecuting: !hasPreview,
      toolExecutingStartTime: !hasPreview ? Date.now() : 0,
      toolExecutingNames: [],
      toolExecutingCommands: [],
      toolSandboxAvailable: false,
      awaitingConfirmation: false,
      previewContent,
      processContentLength: 0,
      toolResults: [],
      processTrace: [],
      statusStartedAt: Date.now(),
      statusSummary: statusLabel,
      uiState: createStreamUIState(phase, {
        label: statusLabel,
        detail: null,
        canRetry: false,
      }),
      recoveryBaseline: createConversationRecoveryBaseline(conversationId),
    }

    activeStreamState.value = nextState
    restoreDetachedActiveStream(conversationId)
    return true
  }

  async function connectConversationStream(
    conversationId: string,
    request: SendMessageRequest,
    options: SSEClientOptions
  ) {
    clearPendingProviderFailoverDraft(conversationId)
    beginActiveStream(conversationId)
    addRequestProcessTrace(conversationId, request)

    await sseClient.connect(conversationId, request, {
      ...options,
      onStreamId: (streamId) => {
        updateActiveStreamState(conversationId, { streamId })
        options.onStreamId?.(streamId)
      },
      onStreamProgress: (progress) => {
        const current = getActiveStreamState(conversationId)
        if (current && !current.receivedFirstChunk) {
          const nextSummary = formatStreamProgress(progress)
          updateActiveStreamState(conversationId, {
            streamProgress: nextSummary,
            statusSummary: nextSummary,
            statusStartedAt:
              current.statusSummary === nextSummary ? current.statusStartedAt : Date.now(),
          })
          updateActiveStreamUIState(conversationId, 'connecting', {
            label: nextSummary,
            detail: null,
            canRetry: false,
          })
        }
        options.onStreamProgress?.(progress)
      },
      onProcessEvent: (chunk) => {
        if (chunk.process_event === 'provider_acceleration_active') {
          const current = getActiveStreamState(conversationId)
          if (current && !current.receivedFirstChunk) {
            updateActiveStreamState(conversationId, {
              providerAccelerationActive: chunk.provider_acceleration_active === true,
            })
          }
          options.onProcessEvent?.(chunk)
          return
        }
        recordPendingProviderFailoverDraft(conversationId, chunk)
        const item = createServerProcessTraceItem(chunk)
        if (item) {
          const nextSummary = resolveProcessTraceStatusLabel(item)
          upsertProcessTraceItem(conversationId, item, {
            replaceLatestByEvent:
              item.event === 'pre_content_retry_scheduled' ||
              item.event === 'pre_content_retry_started' ||
              item.event === 'continuation_recovery_started' ||
              item.event === 'provider_failover',
            updateStatusTimer:
              item.status === 'active' || item.status === 'pending' || item.status === 'error',
          })
          const current = getActiveStreamState(conversationId)
          if (current) {
            updateActiveStreamState(conversationId, {
              statusSummary: nextSummary,
              statusStartedAt:
                current.statusSummary === nextSummary ? current.statusStartedAt : item.timestamp,
            })
            if (
              item.category === 'retry' ||
              item.category === 'recovery' ||
              item.event === 'provider_failover'
            ) {
              updateActiveStreamUIState(conversationId, 'recovering', {
                label: nextSummary,
                detail: item.detail || null,
                canRetry: false,
                updatedAt: item.timestamp,
              })
            }
          }
        }
        options.onProcessEvent?.(chunk)
      },
      onMessage: (chunk) => {
        if (chunk.awaiting_user_input) {
          addLocalProcessTrace(
            conversationId,
            {
              source: 'client',
              event: 'awaiting_confirmation',
              category: 'confirmation',
              status: 'active',
              label: resolveProcessTraceText(
                'events.waitingForConfirmation',
                'Waiting for confirmation'
              ),
              detail: resolveProcessTraceText(
                'details.awaitingConfirmation',
                'The assistant needs your confirmation before continuing.'
              ),
            },
            { replaceLatestByEvent: true, updateStatusTimer: true }
          )
          updateActiveStreamState(conversationId, {
            awaitingConfirmation: true,
            statusStartedAt: Date.now(),
          })
          updateActiveStreamUIState(conversationId, 'awaiting_confirmation', {
            label: resolveStreamUIStateLabel('awaiting_confirmation'),
            detail: null,
            canRetry: false,
          })
        }
        if (chunk.delta) {
          appendActiveStreamPreview(conversationId, chunk.delta)
        }
        options.onMessage(chunk)
      },
      onToolExecuting: (toolCount, toolNames, sandboxAvailable, toolCommands) => {
        const nextSummary = formatAssistantStatusSummaryFromTools(
          toolNames || [],
          toolCommands || []
        )
        updateActiveStreamState(conversationId, {
          toolExecuting: true,
          toolExecutingStartTime: Date.now(),
          toolExecutingNames: toolNames || [],
          toolExecutingCommands: toolCommands || [],
          toolSandboxAvailable: !!sandboxAvailable,
          statusSummary: nextSummary,
          statusStartedAt: Date.now(),
        })
        updateActiveStreamUIState(conversationId, 'executing', {
          label: nextSummary,
          detail: null,
          canRetry: false,
        })
        options.onToolExecuting?.(toolCount, toolNames, sandboxAvailable, toolCommands)
      },
      onToolResults: (results, toolRound) => {
        const parsedResults = parseToolResults(results)
        const current = getActiveStreamState(conversationId)
        if (current) {
          updateActiveStreamState(conversationId, {
            toolResults: [...current.toolResults, ...parsedResults],
          })
        }
        appendLastAssistantLocalProcessToolResults(conversationId, parsedResults)
        options.onToolResults?.(results, toolRound)
      },
      onNewMessage: (toolRound) => {
        resetActiveStreamRound(conversationId)
        options.onNewMessage?.(toolRound)
      },
      onTodoUpdated: (messageId, content, todoCardId, todoCompleted) => {
        options.onTodoUpdated?.(messageId, content, todoCardId, todoCompleted)
      },
      onInjection: (userMessage) => {
        resetActiveStreamRound(conversationId)
        addLocalProcessTrace(conversationId, {
          source: 'client',
          event: 'injection_restart',
          category: 'lifecycle',
          status: 'active',
          label: resolveProcessTraceText(
            'events.restartingWithLatestMessage',
            'Restarting with your latest message'
          ),
          detail:
            summarizeRequestText(userMessage) ||
            resolveProcessTraceText(
              'details.injectionRestart',
              'The assistant is restarting the response with your latest interruption.'
            ),
        })
        updateActiveStreamUIState(conversationId, 'connecting', {
          label: resolveProcessTraceText(
            'events.restartingWithLatestMessage',
            'Restarting with your latest message'
          ),
          detail: summarizeRequestText(userMessage) || null,
          canRetry: false,
        })
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
        addLocalProcessTrace(
          conversationId,
          {
            source: 'client',
            event: 'network_interrupt_waiting',
            category: 'recovery',
            status: 'active',
            label: resolveProcessTraceText(
              'events.waitingForConnectionRecovery',
              'Waiting for connection recovery'
            ),
            detail: resolveProcessTraceText(
              'details.networkInterruptWaiting',
              'The stream was interrupted after content started. Waiting for the backend to recover.'
            ),
          },
          { replaceLatestByEvent: true, updateStatusTimer: true }
        )
        updateActiveStreamState(conversationId, {
          sending: false,
          streaming: false,
          toolExecuting: false,
          streamProgress: null,
          statusSummary: resolveProcessTraceText(
            'events.waitingForConnectionRecovery',
            'Waiting for connection recovery'
          ),
        })
        updateActiveStreamUIState(conversationId, 'recovering', {
          label: resolveProcessTraceText(
            'events.waitingForConnectionRecovery',
            'Waiting for connection recovery'
          ),
          detail: formatRecoveryAttemptDetail(1),
          recoveryAttempt: 0,
          canRetry: false,
        })
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
    const resolved = resolveCommandStateSelection(state)
    selectedProviderId.value = resolved.selected_provider_id
    modelPreference.value = resolved.model_preference
    providerPinOnlyActive.value = !!resolved.selected_provider_id && !resolved.selected_model_id
    offlineMode.value = !!state.offline
    agentcoreRunnerRef.value = normalizeAgentcoreRunnerRefValue(state.agentcore_runner_ref)
  }

  function normalizeConversationId(conversationId?: string | null): string {
    return conversationId?.trim() || ''
  }

  function isCommandStateHydratedForConversation(conversationId?: string | null): boolean {
    const normalized = normalizeConversationId(conversationId)
    return (
      normalized !== '' &&
      commandStateHydrated.value &&
      commandStateHydratedConversationId === normalized
    )
  }

  function markCommandStateHydrated(conversationId?: string | null) {
    const normalized = normalizeConversationId(conversationId)
    commandStateHydrated.value = normalized !== ''
    commandStateHydratedConversationId = normalized || null
  }

  function resetCommandStateHydration(conversationId?: string | null) {
    const normalized = normalizeConversationId(conversationId)
    if (!normalized || commandStateHydratedConversationId === normalized) {
      commandStateHydrated.value = false
      commandStateHydratedConversationId = null
    }
    if (!normalized || commandStateHydratePromiseConversationId === normalized) {
      commandStateHydratePromise = null
      commandStateHydratePromiseConversationId = null
    }
  }

  function getLocalCommandStateSeed(options?: { useCurrentAgentcoreRunnerRef?: boolean }) {
    const resolved = getCurrentRequestModelSelection()
    const seed: ConversationCommandState = {
      selected_provider_id: resolved.selected_provider_id,
      selected_model_id: resolved.selected_model_id,
      offline: loadOfflineMode(),
    }
    const runnerRefSource = options?.useCurrentAgentcoreRunnerRef
      ? agentcoreRunnerRef.value
      : settingsStore.experimentalAgentcoreRunnerRef
    if (hasConversationScopedAgentcoreRunnerRef(runnerRefSource)) {
      seed.agentcore_runner_ref = normalizeAgentcoreRunnerRefValue(runnerRefSource)
    }
    return seed
  }

  async function fetchCommandState(conversationId: string, options?: { force?: boolean }) {
    const force = !!options?.force
    if (!force && isCommandStateHydratedForConversation(conversationId)) {
      const resolved = getCurrentRequestModelSelection()
      const cached: ConversationCommandState = {
        conversation_id: conversationId,
        selected_provider_id: resolved.selected_provider_id,
        selected_model_id: resolved.selected_model_id,
        offline: offlineMode.value,
        agentcore_runner_ref: normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value),
      }
      return cached
    }
    if (
      !force &&
      commandStateHydratePromise &&
      commandStateHydratePromiseConversationId === normalizeConversationId(conversationId)
    ) {
      return commandStateHydratePromise
    }
    const req = chatBootstrapApi
      .getConversationBootstrap(conversationId)
      .then((response) => {
        const state: ConversationCommandState = response.data?.command_state || {
          conversation_id: conversationId,
          ...getLocalCommandStateSeed(),
        }
        if (currentConversationId.value === conversationId) {
          applyCommandState(state)
          markCommandStateHydrated(conversationId)
        }
        return state
      })
      .finally(() => {
        if (commandStateHydratePromiseConversationId === normalizeConversationId(conversationId)) {
          commandStateHydratePromise = null
          commandStateHydratePromiseConversationId = null
        }
      })
    commandStateHydratePromise = req
    commandStateHydratePromiseConversationId = normalizeConversationId(conversationId)
    return req
  }

  async function patchCommandState(conversationId: string, patch: ConversationCommandStatePatch) {
    const response = await conversationApi.patchCommandState(conversationId, patch)
    if (currentConversationId.value === conversationId) {
      applyCommandState(response.data)
      markCommandStateHydrated(conversationId)
    }
    return response.data
  }

  function shouldSeedConversationCommandState(state: ConversationCommandState): boolean {
    const selectedProviderId = state.selected_provider_id?.trim() || ''
    const selectedModelId = state.selected_model_id?.trim() || ''
    return (
      !commandStateHydrated.value ||
      !!selectedProviderId ||
      !!selectedModelId ||
      !!state.offline ||
      hasConversationScopedAgentcoreRunnerRef(state.agentcore_runner_ref)
    )
  }

  async function applyAgentcoreRunnerRef(value: string, conversationId?: string | null) {
    const normalized = normalizeAgentcoreRunnerRefValue(value)
    agentcoreRunnerRef.value = normalized

    const targetConversationId = normalizeConversationId(conversationId)
    if (!targetConversationId) return

    await patchCommandState(targetConversationId, {
      agentcore_runner_ref: normalized,
    })
  }

  async function seedConversationCommandState(
    conversationId: string,
    seed: ConversationCommandState = getLocalCommandStateSeed()
  ) {
    if (!shouldSeedConversationCommandState(seed)) return
    await patchCommandState(conversationId, seed)
  }

  function clearPendingModelAutoFallback() {
    pendingModelAutoFallback.value = null
  }

  function clearPendingProviderFailoverDraft(conversationId?: string) {
    if (
      conversationId &&
      pendingProviderFailoverDraft.value &&
      pendingProviderFailoverDraft.value.conversationId !== conversationId
    ) {
      return
    }
    pendingProviderFailoverDraft.value = null
  }

  function clearPendingProviderFailoverRetry() {
    pendingProviderFailoverRetry.value = null
  }

  function recordPendingProviderFailoverDraft(conversationId: string, chunk: StreamChunk) {
    if (chunk.process_event !== 'provider_failover') return
    if (chunk.process_requires_confirmation) {
      pendingProviderFailoverDraft.value = {
        conversationId,
        failedProviderId: chunk.process_provider?.trim() || '',
        detail: chunk.process_detail?.trim() || chunk.process_message?.trim() || '',
        retryAttempts:
          typeof chunk.process_retry_attempts === 'number' &&
          Number.isFinite(chunk.process_retry_attempts)
            ? chunk.process_retry_attempts
            : undefined,
      }
      return
    }
    if (chunk.process_status === 'success' || chunk.process_status === 'error') {
      clearPendingProviderFailoverDraft(conversationId)
    }
  }

  function queueProviderFailoverRetry(params: {
    conversationId: string
    errorMessage: string
    retryKind: ModelAutoFallbackRetryKind
  }): boolean {
    if (!isProviderFailoverConfirmationErrorText(params.errorMessage)) return false
    const draft = pendingProviderFailoverDraft.value
    if (!draft || draft.conversationId !== params.conversationId) return false

    pendingProviderFailoverRetry.value = {
      ...draft,
      retryKind: params.retryKind,
      errorMessage: params.errorMessage,
    }
    clearPendingProviderFailoverDraft(params.conversationId)
    error.value = null
    streamError.value = null
    return true
  }

  function queueModelAutoFallbackRetry(params: {
    conversationId: string
    request: SendMessageRequest
    errorMessage: string
    retryKind: ModelAutoFallbackRetryKind
  }): boolean {
    const currentPreference = modelPreference.value.trim()
    if (!isFixedModelPreferenceValue(currentPreference)) return false
    if (!isModelUnavailableErrorText(params.errorMessage)) return false

    const parsedPreference = splitModelPreference(currentPreference)
    const requestedModelId =
      params.request.model.trim() || parsedPreference.selected_model_id?.trim() || ''
    if (!requestedModelId || requestedModelId.toLowerCase() === 'auto') return false

    pendingModelAutoFallback.value = {
      conversationId: params.conversationId,
      retryKind: params.retryKind,
      requestedModelId,
      requestedProviderId:
        params.request.provider.trim() || parsedPreference.selected_provider_id?.trim() || '',
      errorMessage: params.errorMessage,
    }
    error.value = null
    streamError.value = null
    return true
  }

  async function applyModelPreference(value: string, conversationId?: string | null) {
    const next = value.trim()
    const resolved = resolveModelSelection(selectedProviderId.value, next || 'auto')
    modelPreference.value = resolved.model_preference
    saveModelPreference(modelPreference.value)
    selectedProviderId.value = resolved.selected_provider_id
    providerPinOnlyActive.value = false
    clearPendingModelAutoFallback()
    clearPendingProviderFailoverRetry()

    if (!conversationId) return

    const patch: ConversationCommandStatePatch = {}
    if (!next || next === 'auto') {
      patch.selected_provider_id = ''
      patch.selected_model_id = ''
    } else {
      patch.selected_model_id = resolved.selected_model_id
      if (resolved.selected_provider_id) {
        patch.selected_provider_id = resolved.selected_provider_id
      }
    }

    await patchCommandState(conversationId, patch)
  }

  async function applyProviderPinOnly(providerIdRaw: string, conversationId?: string | null) {
    const providerId = providerIdRaw.trim()
    if (!providerId) {
      await applyModelPreference('auto', conversationId)
      return
    }

    modelPreference.value = 'auto'
    saveModelPreference('auto')
    selectedProviderId.value = providerId
    providerPinOnlyActive.value = true
    clearPendingModelAutoFallback()
    clearPendingProviderFailoverRetry()

    if (!conversationId) return

    await patchCommandState(conversationId, {
      selected_provider_id: providerId,
      selected_model_id: '',
    })
  }

  async function confirmModelAutoFallbackRetry() {
    const pending = pendingModelAutoFallback.value
    if (!pending) return

    clearPendingModelAutoFallback()
    clearError()
    clearStreamError()

    await applyModelPreference('auto', pending.conversationId).catch(() => {})

    if (currentConversationId.value !== pending.conversationId) {
      await selectConversation(pending.conversationId)
    } else {
      await fetchMessages(pending.conversationId)
    }

    if (pending.retryKind === 'continue') {
      await continueMessage()
      return
    }

    await regenerateMessage()
  }

  function dismissModelAutoFallbackRetry() {
    clearPendingModelAutoFallback()
  }

  async function confirmProviderFailoverRetry() {
    const pending = pendingProviderFailoverRetry.value
    if (!pending) return

    clearPendingProviderFailoverRetry()
    clearError()
    clearStreamError()

    if (currentConversationId.value !== pending.conversationId) {
      await selectConversation(pending.conversationId)
    } else {
      await fetchMessages(pending.conversationId)
    }

    if (pending.retryKind === 'continue') {
      await continueMessage()
      return
    }

    await regenerateMessage()
  }

  function dismissProviderFailoverRetry() {
    clearPendingProviderFailoverRetry()
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
  let lastStreamCommitAt = Date.now()
  const STREAM_COMMIT_MAX_DEFER_MS = 24
  const STREAM_COMMIT_IDLE_FLUSH_MS = 48

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

  function appendLastAssistantLocalProcessToolResults(
    conversationId: string,
    items: ToolResultItem[]
  ) {
    if (items.length === 0) return
    const lastIndex = messages.value.length - 1
    if (lastIndex < 0) return
    const lastMsg = messages.value[lastIndex]
    if (!lastMsg || lastMsg.role !== 'assistant' || lastMsg.conversation_id !== conversationId) {
      return
    }

    const runtimeMsg = lastMsg as RuntimeProcessMessage
    const existing = runtimeMsg.local_process_tool_results || []
    runtimeMsg.local_process_tool_results = [...existing, ...cloneToolResultItems(items)]
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
      lastStreamCommitAt = Date.now()
    }
    patchLastAssistantMessageContent(targetConversationId)
    pendingStreamConversationId = null
  }

  function schedulePendingStreamCommit() {
    if (streamCommitRaf !== null || streamCommitTimer) return
    const commit = (source: 'raf' | 'timer') => {
      if (
        source === 'timer' &&
        streamCommitRaf !== null &&
        typeof window !== 'undefined' &&
        typeof window.cancelAnimationFrame === 'function'
      ) {
        window.cancelAnimationFrame(streamCommitRaf)
      }
      streamCommitRaf = null
      if (streamCommitTimer) {
        clearTimeout(streamCommitTimer)
      }
      streamCommitTimer = null
      flushPendingStreamDelta()
    }
    if (typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function') {
      streamCommitRaf = window.requestAnimationFrame(() => commit('raf'))
      streamCommitTimer = setTimeout(() => commit('timer'), STREAM_COMMIT_MAX_DEFER_MS)
      return
    }
    streamCommitTimer = setTimeout(() => commit('timer'), Math.min(16, STREAM_COMMIT_MAX_DEFER_MS))
  }

  function enqueueStreamDelta(conversationId: string, delta: string) {
    if (!delta) return
    if (pendingStreamConversationId && pendingStreamConversationId !== conversationId) {
      flushPendingStreamDelta(pendingStreamConversationId)
    }
    const shouldFlushImmediately =
      pendingStreamDelta === '' &&
      (streamingContent.value === '' ||
        Date.now() - lastStreamCommitAt >= STREAM_COMMIT_IDLE_FLUSH_MS)
    pendingStreamConversationId = conversationId
    pendingStreamDelta += delta
    if (shouldFlushImmediately) {
      flushPendingStreamDelta(conversationId)
      return
    }
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

  function findLastUserMessageIndex(): number {
    for (let i = messages.value.length - 1; i >= 0; i--) {
      if (messages.value[i]?.role === 'user') {
        return i
      }
    }
    return -1
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

  function clearRecentTodoCompletion() {
    recentTodoCompletion.value = null
  }

  function markRecentTodoCompletion(messageId: string, todoCardId?: string) {
    const normalizedMessageId = messageId.trim()
    if (!normalizedMessageId) return
    const normalizedTodoCardId = todoCardId?.trim() || undefined
    recentTodoCompletion.value = {
      messageId: normalizedMessageId,
      ...(normalizedTodoCardId ? { todoCardId: normalizedTodoCardId } : {}),
    }
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

    const nextContent = mergeMissingProcessCardsIntoFinalContent(
      lastMsg.content,
      finalChunk?.content ?? streamingContent.value,
      finalChunk?.finalization_mode
    )
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

  function hasPendingConfirmationState(
    data: Partial<PendingConfirmationSnapshot> | null | undefined
  ): data is PendingConfirmationSnapshot {
    if (!data || typeof data !== 'object') return false
    return (
      Object.prototype.hasOwnProperty.call(data, 'pending_approval') &&
      Object.prototype.hasOwnProperty.call(data, 'pending_question') &&
      Object.prototype.hasOwnProperty.call(data, 'pending_exec_approval')
    )
  }

  function applyPendingConfirmations(data: PendingConfirmationSnapshot) {
    setPendingApproval(null)
    setPendingQuestion(null)
    setPendingExecApproval(null)

    if (data.pending_approval) {
      setPendingApproval(data.pending_approval)
    }
    if (data.pending_question) {
      setPendingQuestion(data.pending_question)
    }
    if (data.pending_exec_approval) {
      setPendingExecApproval(data.pending_exec_approval)
    }

    if (hasPendingConfirmations()) {
      awaitingConfirmation.value = true
    } else if (!streaming.value) {
      awaitingConfirmation.value = false
    }
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
    binding_hash?: string
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

    const type = stringifyOptional(source.type) || 'directory'
    const normalized = {
      id,
      type,
      command: stringifyOptional(command),
      directory: stringifyOptional(source.directory ?? source.dir ?? source.path),
      workdir: stringifyOptional(source.workdir ?? source.cwd),
      host: stringifyOptional(source.host),
      security: stringifyOptional(source.security),
      session_id: stringifyOptional(source.session_id),
      conversation_id: stringifyOptional(source.conversation_id),
      binding_hash: stringifyOptional(source.binding_hash ?? source.bindingHash),
      expires_at: expiresAt,
    }

    if (normalized.type === 'directory' && !normalized.directory && normalized.workdir) {
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

  function resolvePendingConfirmationRecoveryKey(sessionId?: string): string {
    const normalizedSessionId =
      typeof sessionId === 'string' ? sessionId.trim() : currentConversationId.value?.trim() || ''
    return normalizedSessionId ? `session:${normalizedSessionId}` : 'global'
  }

  function markPendingConfirmationRecoveryChecked(sessionId?: string) {
    pendingConfirmationRecoveryChecks.add(resolvePendingConfirmationRecoveryKey(sessionId))
  }

  async function checkPendingConfirmationsFromBootstrap(force = false): Promise<boolean> {
    const sessionId = currentConversationId.value?.trim() || ''
    if (!sessionId) {
      return false
    }

    const recoveryKey = resolvePendingConfirmationRecoveryKey(sessionId)
    if (!force && pendingConfirmationRecoveryChecks.has(recoveryKey)) {
      return true
    }

    const pendingRequest = pendingConfirmationRecoveryRequests.get(recoveryKey)
    if (pendingRequest) {
      await pendingRequest
      return true
    }

    let recovered = false
    const request = chatBootstrapApi
      .getConversationBootstrap(sessionId)
      .then((response) => {
        const data = response.data
        if (hasPendingConfirmationState(data)) {
          applyPendingConfirmations(data)
        } else if (!streaming.value) {
          setPendingApproval(null)
          setPendingQuestion(null)
          setPendingExecApproval(null)
        }
        if (!force) {
          pendingConfirmationRecoveryChecks.add(recoveryKey)
        }
        recovered = true
      })
      .catch(() => {
        // Bootstrap may be temporarily unavailable during startup/reconnect.
      })
      .finally(() => {
        pendingConfirmationRecoveryRequests.delete(recoveryKey)
      })

    pendingConfirmationRecoveryRequests.set(recoveryKey, request)
    await request
    return recovered
  }

  function clearAwaitingConfirmationForSession(sessionId?: string) {
    const normalizedSessionId = typeof sessionId === 'string' ? sessionId.trim() : ''
    if (normalizedSessionId) {
      updateActiveStreamState(normalizedSessionId, { awaitingConfirmation: false })
    }
    if (!pendingQuestion.value && !pendingApproval.value && !pendingExecApproval.value) {
      awaitingConfirmation.value = false
    }
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
    const forceBootstrapRefresh = allowInlineFallback || awaitingConfirmation.value
    await checkPendingConfirmationsFromBootstrap(forceBootstrapRefresh)
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
    const conversationId = currentConversationId.value
    if (conversationId) {
      updateActiveStreamState(conversationId, {
        awaitingConfirmation: true,
        toolExecuting: false,
        streamProgress: null,
        statusStartedAt: Date.now(),
      })
      updateActiveStreamUIState(conversationId, 'awaiting_confirmation', {
        label: resolveStreamUIStateLabel('awaiting_confirmation'),
        detail: null,
        canRetry: false,
      })
    }
    // If the out-of-band approval event was missed, recover pending payloads.
    if (!wasAwaiting) {
      void recoverPendingConfirmationsWithRetry()
    }
  }

  function markStreamInterrupted(conversationId: string, detail?: string) {
    updateActiveStreamState(conversationId, {
      sending: false,
      streaming: false,
      toolExecuting: false,
      streamProgress: null,
      statusSummary: resolveStreamUIStateLabel('interrupted'),
      statusStartedAt: Date.now(),
    })
    updateActiveStreamUIState(conversationId, 'interrupted', {
      label: resolveStreamUIStateLabel('interrupted'),
      detail: detail || null,
      canRetry: true,
    })
  }

  async function recoverInterruptedStreamAttempt(
    conversationId: string,
    attempt: number,
    baseline: StreamRecoveryBaseline
  ): Promise<void> {
    const state = getActiveStreamState(conversationId)
    if (!state) return

    const nextAttempt = attempt + 1
    updateActiveStreamState(conversationId, {
      sending: false,
      streaming: false,
      toolExecuting: false,
      streamProgress: null,
      statusSummary: resolveStreamUIStateLabel('recovering'),
      statusStartedAt: state.statusStartedAt || Date.now(),
    })
    updateActiveStreamUIState(conversationId, 'recovering', {
      label: resolveStreamUIStateLabel('recovering'),
      detail: formatRecoveryAttemptDetail(nextAttempt),
      recoveryAttempt: nextAttempt,
      canRetry: false,
    })

    try {
      await fetchMessages(conversationId)
    } catch {
      // Best-effort recovery fetch.
    }

    try {
      await recoverPendingConfirmations(true)
    } catch {
      // Best-effort confirmation recovery.
    }

    const pendingRecovered =
      hasPendingConfirmations() ||
      awaitingConfirmation.value ||
      getActiveStreamState(conversationId)?.awaitingConfirmation
    if (pendingRecovered) {
      updateActiveStreamState(conversationId, {
        sending: false,
        streaming: false,
        toolExecuting: false,
        streamProgress: null,
        awaitingConfirmation: true,
        statusSummary: resolveStreamUIStateLabel('awaiting_confirmation'),
        statusStartedAt: Date.now(),
      })
      updateActiveStreamUIState(conversationId, 'awaiting_confirmation', {
        label: resolveStreamUIStateLabel('awaiting_confirmation'),
        detail: null,
        canRetry: false,
      })
      clearStreamRecoveryTimer()
      return
    }

    const latest = createConversationRecoveryBaseline(conversationId)
    if (recoveryBaselineAdvanced(baseline, latest)) {
      updateActiveStreamState(conversationId, {
        sending: false,
        streaming: false,
        toolExecuting: false,
        streamProgress: null,
        statusSummary: resolveStreamUIStateLabel('completed'),
        statusStartedAt: Date.now(),
        recoveryBaseline: latest,
      })
      updateActiveStreamUIState(conversationId, 'completed', {
        label: resolveStreamUIStateLabel('completed'),
        detail: null,
        canRetry: false,
      })
      clearStreamRecoveryTimer()
      window.setTimeout(() => {
        const current = getActiveStreamState(conversationId)
        if (!current || current.uiState.phase !== 'completed') return
        clearActiveStreamState(conversationId)
        if (currentConversationId.value === conversationId) {
          clearVisibleStreamState()
        }
      }, 600)
      return
    }

    if (nextAttempt >= streamRecoveryRetryLimit) {
      markStreamInterrupted(
        conversationId,
        resolveProcessTraceText(
          'details.networkInterruptWaiting',
          'The stream was interrupted after content started. Waiting for the backend to recover.'
        )
      )
      clearStreamRecoveryTimer()
      return
    }

    streamRecoveryTimer = setTimeout(() => {
      void recoverInterruptedStreamAttempt(conversationId, nextAttempt, baseline)
    }, streamRecoveryRetryDelayMs)
  }

  function startInterruptedStreamRecovery(conversationId: string) {
    clearStreamRecoveryTimer()
    const state = getActiveStreamState(conversationId)
    if (!state) return
    void recoverInterruptedStreamAttempt(conversationId, 0, state.recoveryBaseline)
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
    if (
      state.streaming ||
      state.sending ||
      state.toolExecuting ||
      state.awaitingConfirmation ||
      state.uiState.phase === 'recovering'
    ) {
      return [state.conversationId]
    }
    return []
  })

  // Actions
  async function fetchConversations() {
    try {
      loading.value = true
      error.value = null
      reportStartupMark('chat_fetch_conversations_start')
      const response = await conversationApi.list()
      conversations.value = response.data
      reportStartupMark('chat_fetch_conversations_done', {
        conversation_count: response.data.length,
      })
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to fetch conversations'
      reportStartupMark('chat_fetch_conversations_error')
    } finally {
      loading.value = false
    }
  }

  function stopActiveStreamForConversationSwitch() {
    if (
      !streaming.value &&
      !sending.value &&
      streamUIState.value.phase !== 'recovering' &&
      streamUIState.value.phase !== 'awaiting_confirmation' &&
      streamUIState.value.phase !== 'interrupted'
    ) {
      return
    }
    flushPendingStreamDelta(currentConversationId.value)
    if (currentConversationId.value) {
      updateActiveStreamState(currentConversationId.value, {
        streamId: activeStreamId.value,
        sending: sending.value,
        streaming: streaming.value,
        receivedFirstChunk: _receivedFirstChunk.value,
        providerAccelerationActive: providerAccelerationActive.value,
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
        processContentLength: processContentLength.value,
        toolResults: cloneToolResultItems(toolResults.value),
        processTrace: cloneProcessTrace(processTrace.value),
        statusStartedAt: statusStartedAt.value,
        statusSummary: statusSummary.value,
        uiState: { ...streamUIState.value },
      })
    }
    pendingQuestion.value = null
    pendingApproval.value = null
    pendingExecApproval.value = null
    awaitingConfirmation.value = false
    clearPendingRecoveryRetryTimer()
    clearStreamRecoveryTimer()
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
      resetCommandStateHydration()
      messages.value = []
      hasMoreMessages.value = false
      currentPage.value = 0
      const seed = getLocalCommandStateSeed({ useCurrentAgentcoreRunnerRef: true })
      if (shouldSeedConversationCommandState(seed)) {
        try {
          await seedConversationCommandState(response.data.id, seed)
        } catch {
          applyCommandState({
            conversation_id: response.data.id,
            ...seed,
          })
          markCommandStateHydrated(response.data.id)
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
      resetCommandStateHydration(id)
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
    clearRecentTodoCompletion()

    currentConversationId.value = id
    resetCommandStateHydration()
    reportStartupMark('chat_select_conversation_start')
    // Don't clear messages immediately to avoid flash
    // Reset pagination state
    hasMoreMessages.value = false
    currentPage.value = 0
    let bootstrapData: ConversationBootstrapResponse | null = null
    let bootstrapHasPendingConfirmationState = false

    try {
      // Fetch messages without setting loading state to avoid flash
      error.value = null
      const [messageResponse, bootstrapResponse] = await Promise.all([
        messageApi.list(id, PAGE_SIZE, 0),
        chatBootstrapApi.getConversationBootstrap(id).catch(() => null),
      ])
      const fetchedMessages = messageResponse.data
      bootstrapData = bootstrapResponse?.data || null
      const commandStateResponse = bootstrapData?.command_state || null
      const activeStreamResponse = bootstrapData?.active_stream || null
      bootstrapHasPendingConfirmationState = Boolean(
        bootstrapData && hasPendingConfirmationState(bootstrapData)
      )

      // Only update if we're still on the same conversation
      if (currentConversationId.value === id) {
        clearRecentTodoCompletion()
        messages.value = fetchedMessages
        hasMoreMessages.value = fetchedMessages.length === PAGE_SIZE
        currentPage.value = 0
        reportStartupMark('chat_select_conversation_done', {
          message_count: fetchedMessages.length,
        })
        if (commandStateResponse) {
          applyCommandState(commandStateResponse)
          markCommandStateHydrated(id)
        } else {
          applyCommandState({
            conversation_id: id,
            ...getLocalCommandStateSeed(),
          })
          markCommandStateHydrated(id)
        }
        if (bootstrapData && hasPendingConfirmationState(bootstrapData)) {
          applyPendingConfirmations(bootstrapData)
        }

        const detachedState = getActiveStreamState(id)
        if (detachedState) {
          if (
            detachedState.streaming ||
            detachedState.previewContent ||
            detachedState.toolResults.length > 0
          ) {
            restoreDetachedActiveStream(id)
          } else if (currentConversationId.value === id) {
            applyVisibleStreamState(detachedState)
          }
          if (detachedState.uiState.phase === 'recovering') {
            startInterruptedStreamRecovery(id)
          }
          if (!bootstrapHasPendingConfirmationState) {
            void recoverPendingConfirmations(true)
          }
          return
        }

        if (restoreServerActiveStreamState(id, activeStreamResponse)) {
          if (!bootstrapHasPendingConfirmationState) {
            void recoverPendingConfirmations(true)
          }
          return
        }
      }
    } catch (e) {
      if (currentConversationId.value === id) {
        error.value = e instanceof Error ? e.message : 'Failed to fetch messages'
        messages.value = []
        reportStartupMark('chat_select_conversation_error')
      }
    }

    // Restore pending confirmations for this conversation if any.
    if (!bootstrapHasPendingConfirmationState) {
      void recoverPendingConfirmations(false)
    }
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
        clearRecentTodoCompletion()
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

  function cloneMessageAttachments(
    attachments?: MessageAttachment[]
  ): MessageAttachment[] | undefined {
    if (!attachments || attachments.length === 0) return undefined
    return attachments.map((attachment) => ({ ...attachment }))
  }

  async function sendMessage(
    content: string,
    fileAttachments?: SendMessageFileAttachment[],
    options?: SendMessageOptions
  ) {
    if (!currentConversationId.value) {
      if (options?.skipConversationCreate) return
      // Use the first part of the message as the conversation title
      const title = content.length > 30 ? content.substring(0, 30) + '...' : content
      await createConversation(title)
    }

    const conversationId = currentConversationId.value!
    const settingsStore = useSettingsStore()
    const existingAttachments = cloneMessageAttachments(options?.existingAttachments)
    const hasAnyAttachments =
      (existingAttachments?.length ?? 0) > 0 || (fileAttachments?.length ?? 0) > 0
    const shouldRefreshCommandStateAfterComplete = !hasAnyAttachments && isSlashCommandText(content)

    // Convert file attachments to MessageAttachment format (base64)
    const attachments: MessageAttachment[] = existingAttachments ? [...existingAttachments] : []
    if (!existingAttachments && fileAttachments && fileAttachments.length > 0) {
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

    const resolvedModelSelection = getCurrentRequestModelSelection({
      conversationId,
      persistIfChanged: true,
    })
    const request: SendMessageRequest = {
      message: content,
      provider: resolvedModelSelection.selected_provider_id || '',
      model: resolvedModelSelection.selected_model_id || '',
      temperature: settingsStore.temperature,
      max_tokens: settingsStore.maxTokens,
      attachments: attachments.length > 0 ? attachments : undefined,
    }

    try {
      sending.value = true
      streaming.value = true
      clearRecentTodoCompletion()
      streamProgress.value = null
      streamError.value = null
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
          streamError.value = null
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
        onToolResults: (_results, _toolRound) => {
          if (currentConversationId.value !== sendConvId) return
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
        onTodoUpdated: (messageId, content, todoCardId, todoCompleted) => {
          if (currentConversationId.value !== sendConvId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
          if (todoCompleted) {
            markRecentTodoCompletion(messageId, todoCardId)
          }
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
            context_window_exceeded: 'contextWindowExceeded',
            request_build_failed: 'requestBuildFailed',
            request_too_large: 'requestTooLarge',
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
            if (
              lower.includes('context_window_exceeded') ||
              lower.includes('context window is full') ||
              lower.includes('maximum context length') ||
              lower.includes('context_length_exceeded') ||
              lower.includes('reduce conversation history')
            )
              return 'contextWindowExceeded'
            if (
              lower.includes('request_too_large') ||
              lower.includes('内容超长') ||
              lower.includes('request too large') ||
              lower.includes('payload too large') ||
              lower.includes('entity too large')
            )
              return 'requestTooLarge'
            if (
              lower.includes('request_build_failed') ||
              lower.includes('构建请求失败') ||
              lower.includes('failed to build request') ||
              lower.includes('improperly_formed_request') ||
              lower.includes('工具参数错误')
            )
              return 'requestBuildFailed'
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
              streamError.value = null
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
                } else {
                  streamError.value = null
                }
              })
              .catch(() => {
                const errorKey = resolveErrorKey(err.message)
                streamError.value = errorKey || err.message
                messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
              })
            return
          }

          if (
            queueProviderFailoverRetry({
              conversationId,
              errorMessage: err.message,
              retryKind: 'send',
            })
          ) {
            messages.value = messages.value.filter(
              (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
            )
            return
          }

          if (
            queueModelAutoFallbackRetry({
              conversationId,
              request,
              errorMessage: err.message,
              retryKind: 'send',
            })
          ) {
            messages.value = messages.value.filter(
              (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
            )
            return
          }

          const errorKey = resolveErrorKey(err.message)
          if (errorKey) {
            streamError.value = errorKey
          } else {
            streamError.value = err.message
          }
          providerAccelerationActive.value = false
          updateActiveStreamState(sendConvId, { providerAccelerationActive: false })
          streamUIState.value = createStreamUIState('interrupted', {
            label: resolveStreamUIStateLabel('interrupted'),
            detail: errorKey || err.message,
            canRetry: true,
          })
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
          if (currentConversationId.value !== sendConvId) return
          flushPendingStreamDelta(sendConvId)
          streamError.value = null
          startInterruptedStreamRecovery(sendConvId)
        },
        onComplete: (finalChunk) => {
          flushPendingStreamDelta(sendConvId)
          streaming.value = false
          streamProgress.value = null
          providerAccelerationActive.value = false
          updateActiveStreamState(sendConvId, { providerAccelerationActive: false })
          streamError.value = null
          toolExecuting.value = false
          streamUIState.value = createStreamUIState('completed', {
            label: resolveStreamUIStateLabel('completed'),
          })
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
      const caughtMessage = e instanceof Error ? e.message : 'Failed to send message'
      if (
        queueProviderFailoverRetry({
          conversationId,
          errorMessage: caughtMessage,
          retryKind: 'send',
        })
      ) {
        messages.value = messages.value.filter(
          (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
        )
      } else if (
        queueModelAutoFallbackRetry({
          conversationId,
          request,
          errorMessage: caughtMessage,
          retryKind: 'send',
        })
      ) {
        messages.value = messages.value.filter(
          (m) => !m.id.startsWith('temp-') && !m.id.startsWith('streaming-')
        )
      } else {
        error.value = caughtMessage
        streamUIState.value = createStreamUIState('interrupted', {
          label: resolveStreamUIStateLabel('interrupted'),
          detail: error.value,
          canRetry: true,
        })
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
      }
    } finally {
      finalizeVisibleStreamSession(conversationId)
    }
  }

  async function editMessageAndResubmit(messageId: string, content: string) {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const targetIndex = messages.value.findIndex((message) => message.id === messageId)
    if (targetIndex < 0) return

    const targetMessage = messages.value[targetIndex]
    if (!targetMessage || targetMessage.role !== 'user') return
    if (targetMessage.id.startsWith('temp-') || targetMessage.id.startsWith('streaming-')) return

    const nextContent = content.trim()
    const nextAttachments = cloneMessageAttachments(targetMessage.attachments)
    if (!nextContent && (!nextAttachments || nextAttachments.length === 0)) return

    const messagesToReplace = messages.value.slice(targetIndex)
    const persistedMessageIds = messagesToReplace
      .map((message) => message.id)
      .filter((id) => !id.startsWith('temp-') && !id.startsWith('streaming-'))

    try {
      if (persistedMessageIds.length > 0) {
        await messageApi.delete(conversationId, persistedMessageIds)
      }

      messages.value = messages.value.slice(0, targetIndex)
      clearSelection()
      await sendMessage(nextContent, undefined, {
        existingAttachments: nextAttachments,
        skipConversationCreate: true,
      })
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to edit message'
      throw e
    }
  }

  function cancelStreaming() {
    const hasInterruptibleState =
      streaming.value ||
      sending.value ||
      streamUIState.value.phase === 'recovering' ||
      streamUIState.value.phase === 'awaiting_confirmation' ||
      streamUIState.value.phase === 'interrupted'
    if (!currentConversationId.value || !hasInterruptibleState) return
    flushPendingStreamDelta(currentConversationId.value)
    void cancelActiveStreamOnServer(currentConversationId.value)
    sseClient.disconnect()
    clearStreamRecoveryTimer()
    clearActiveStreamState(currentConversationId.value)
    clearVisibleStreamState()
    if (!hasPendingConfirmations()) {
      awaitingConfirmation.value = false
    }
  }

  function retryInterruptedStreamRecovery() {
    const conversationId = currentConversationId.value
    if (!conversationId) return
    clearStreamError()
    if (
      streamUIState.value.phase === 'interrupted' ||
      getActiveStreamState(conversationId)?.uiState.phase === 'interrupted'
    ) {
      const current = getActiveStreamState(conversationId)
      if (current) {
        startInterruptedStreamRecovery(conversationId)
        return
      }
    }
    void regenerateMessage()
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
    let request: SendMessageRequest | null = null

    try {
      sending.value = true
      streaming.value = true
      clearRecentTodoCompletion()
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
      const resolvedModelSelection = getCurrentRequestModelSelection({
        conversationId: convId,
        persistIfChanged: true,
      })
      request = {
        message: '[CONTINUE_AFTER_CANCEL]',
        provider: resolvedModelSelection.selected_provider_id || '',
        model: resolvedModelSelection.selected_model_id || modelSelection.selected_model_id || '',
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
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
        onToolResults: (_results, _toolRound) => {
          if (currentConversationId.value !== convId) return
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
        onTodoUpdated: (messageId, content, todoCardId, todoCompleted) => {
          if (currentConversationId.value !== convId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
          if (todoCompleted) {
            markRecentTodoCompletion(messageId, todoCardId)
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== convId) return
          flushPendingStreamDelta(convId)
          streamProgress.value = null
          toolExecuting.value = false
          if (
            queueProviderFailoverRetry({
              conversationId: convId,
              errorMessage: err.message,
              retryKind: 'continue',
            })
          ) {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            streaming.value = false
            return
          }
          if (
            queueModelAutoFallbackRetry({
              conversationId: convId,
              request: request!,
              errorMessage: err.message,
              retryKind: 'continue',
            })
          ) {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            streaming.value = false
            return
          }
          streamError.value = err.message
          streamUIState.value = createStreamUIState('interrupted', {
            label: resolveStreamUIStateLabel('interrupted'),
            detail: err.message,
            canRetry: true,
          })
          messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
          streaming.value = false
        },
        onNetworkInterrupt: () => {
          if (currentConversationId.value !== convId) return
          flushPendingStreamDelta(convId)
          streamError.value = null
          startInterruptedStreamRecovery(convId)
        },
        onComplete: (finalChunk) => {
          flushPendingStreamDelta(convId)
          streaming.value = false
          streamProgress.value = null
          toolExecuting.value = false
          streamUIState.value = createStreamUIState('completed', {
            label: resolveStreamUIStateLabel('completed'),
          })
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
    } catch (e) {
      flushPendingStreamDelta(convId)
      const fallbackMessage =
        e instanceof Error ? e.message : resolveI18nText('chat.streamError', 'Stream error')
      if (
        request &&
        queueProviderFailoverRetry({
          conversationId: convId,
          errorMessage: fallbackMessage,
          retryKind: 'continue',
        })
      ) {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      } else if (
        request &&
        queueModelAutoFallbackRetry({
          conversationId: convId,
          request,
          errorMessage: fallbackMessage,
          retryKind: 'continue',
        })
      ) {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      } else {
        streamUIState.value = createStreamUIState('interrupted', {
          label: resolveStreamUIStateLabel('interrupted'),
          detail: fallbackMessage,
          canRetry: true,
        })
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      }
    } finally {
      finalizeVisibleStreamSession(convId)
    }
  }

  // Continue generating from where it stopped
  async function continueMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()
    let request: SendMessageRequest | null = null

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
      clearRecentTodoCompletion()
      streamProgress.value = null
      activeStreamId.value = null
      resetPendingStreamDelta()
      streamingContent.value = existingContent // Start with existing content
      awaitingConfirmation.value = false
      error.value = null

      const modelSelection = splitModelPreference(modelPreference.value)
      const resolvedModelSelection = getCurrentRequestModelSelection({
        conversationId,
        persistIfChanged: true,
      })
      request = {
        message: '[CONTINUE]', // Special marker for continue
        provider: resolvedModelSelection.selected_provider_id || '',
        model: resolvedModelSelection.selected_model_id || modelSelection.selected_model_id || '',
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
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
        onToolResults: (_results, _toolRound) => {
          if (currentConversationId.value !== conversationId) return
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
        onTodoUpdated: (messageId, content, todoCardId, todoCompleted) => {
          if (currentConversationId.value !== conversationId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
          if (todoCompleted) {
            markRecentTodoCompletion(messageId, todoCardId)
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          streamProgress.value = null
          if (
            queueProviderFailoverRetry({
              conversationId,
              errorMessage: err.message,
              retryKind: 'continue',
            })
          ) {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            streaming.value = false
            toolExecuting.value = false
            return
          }
          if (
            queueModelAutoFallbackRetry({
              conversationId,
              request: request!,
              errorMessage: err.message,
              retryKind: 'continue',
            })
          ) {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            streaming.value = false
            toolExecuting.value = false
            return
          }
          error.value = err.message
          streaming.value = false
          toolExecuting.value = false
          streamUIState.value = createStreamUIState('interrupted', {
            label: resolveStreamUIStateLabel('interrupted'),
            detail: err.message,
            canRetry: true,
          })
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
        onNetworkInterrupt: () => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          streamError.value = null
          startInterruptedStreamRecovery(conversationId)
        },
        onComplete: (finalChunk) => {
          flushPendingStreamDelta(conversationId)
          streaming.value = false
          streamProgress.value = null
          toolExecuting.value = false
          streamUIState.value = createStreamUIState('completed', {
            label: resolveStreamUIStateLabel('completed'),
          })
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
      const caughtMessage = e instanceof Error ? e.message : 'Failed to continue message'
      if (
        request &&
        queueProviderFailoverRetry({
          conversationId,
          errorMessage: caughtMessage,
          retryKind: 'continue',
        })
      ) {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      } else if (
        request &&
        queueModelAutoFallbackRetry({
          conversationId,
          request,
          errorMessage: caughtMessage,
          retryKind: 'continue',
        })
      ) {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      } else {
        error.value = caughtMessage
        streamUIState.value = createStreamUIState('interrupted', {
          label: resolveStreamUIStateLabel('interrupted'),
          detail: error.value,
          canRetry: true,
        })
      }
    } finally {
      finalizeVisibleStreamSession(conversationId)
    }
  }

  // Regenerate the last assistant message
  async function regenerateMessage() {
    if (!currentConversationId.value || streaming.value || sending.value) return

    const conversationId = currentConversationId.value
    const settingsStore = useSettingsStore()
    let request: SendMessageRequest | null = null

    if (offlineMode.value) {
      appendAssistantLocalMessage(
        conversationId,
        '离线模式下不支持重新生成，请先执行 `/offline off`。'
      )
      return
    }

    // Find the last user message to regenerate from
    const lastUserMessageIndex = findLastUserMessageIndex()

    if (lastUserMessageIndex === -1) return

    const lastUserMessage = messages.value[lastUserMessageIndex]
    if (!lastUserMessage) return

    // Clear the entire assistant tail for the last turn so regenerate starts from a clean slate.
    if (lastUserMessageIndex < messages.value.length - 1) {
      messages.value = messages.value.slice(0, lastUserMessageIndex + 1)
    }

    try {
      sending.value = true
      streaming.value = true
      clearRecentTodoCompletion()
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
      const resolvedModelSelection = getCurrentRequestModelSelection({
        conversationId,
        persistIfChanged: true,
      })
      request = {
        message: lastUserMessage.content,
        provider: resolvedModelSelection.selected_provider_id || '',
        model: resolvedModelSelection.selected_model_id || modelSelection.selected_model_id || '',
        temperature: settingsStore.temperature,
        max_tokens: settingsStore.maxTokens,
        attachments: lastUserMessage.attachments,
        regenerate: true,
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
        onToolResults: (_results, _toolRound) => {
          if (currentConversationId.value !== conversationId) return
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
        onTodoUpdated: (messageId, content, todoCardId, todoCompleted) => {
          if (currentConversationId.value !== conversationId) return
          applyTodoChecklistUpdate(messageId, content, todoCardId)
          if (todoCompleted) {
            markRecentTodoCompletion(messageId, todoCardId)
          }
        },
        onError: (err) => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          streamProgress.value = null
          const wasToolExecuting = toolExecuting.value
          toolExecuting.value = false
          if (
            !wasToolExecuting &&
            queueProviderFailoverRetry({
              conversationId,
              errorMessage: err.message,
              retryKind: 'regenerate',
            })
          ) {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            return
          }
          if (
            !wasToolExecuting &&
            queueModelAutoFallbackRetry({
              conversationId,
              request: request!,
              errorMessage: err.message,
              retryKind: 'regenerate',
            })
          ) {
            messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
            return
          }
          error.value = err.message
          streamUIState.value = createStreamUIState('interrupted', {
            label: resolveStreamUIStateLabel('interrupted'),
            detail: err.message,
            canRetry: true,
          })
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
        onNetworkInterrupt: () => {
          if (currentConversationId.value !== conversationId) return
          flushPendingStreamDelta(conversationId)
          streamError.value = null
          startInterruptedStreamRecovery(conversationId)
        },
        onComplete: (finalChunk) => {
          flushPendingStreamDelta(conversationId)
          streaming.value = false
          streamProgress.value = null
          toolExecuting.value = false
          streamUIState.value = createStreamUIState('completed', {
            label: resolveStreamUIStateLabel('completed'),
          })
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
      const caughtMessage = e instanceof Error ? e.message : 'Failed to regenerate message'
      if (
        request &&
        queueProviderFailoverRetry({
          conversationId,
          errorMessage: caughtMessage,
          retryKind: 'regenerate',
        })
      ) {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      } else if (
        request &&
        queueModelAutoFallbackRetry({
          conversationId,
          request,
          errorMessage: caughtMessage,
          retryKind: 'regenerate',
        })
      ) {
        messages.value = messages.value.filter((m) => !m.id.startsWith('streaming-'))
      } else {
        error.value = caughtMessage
        streamUIState.value = createStreamUIState('interrupted', {
          label: resolveStreamUIStateLabel('interrupted'),
          detail: error.value,
          canRetry: true,
        })
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
    if (streamUIState.value.phase === 'interrupted' && !activeStreamState.value) {
      streamUIState.value = createStreamUIState('idle')
    }
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
    const approval = pendingApproval.value
    if (!approval) return false
    const toolName = approval.tool_name
    try {
      await approvalApi.resolve(approval.request_id, decision, approval.binding_hash)
      pendingApproval.value = null
      clearAwaitingConfirmationForSession(approval.session_id)
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
      return true
    } catch (e) {
      console.error('Failed to resolve tool approval:', e)
      return false
    }
  }

  async function checkPendingApprovals() {
    if (await checkPendingConfirmationsFromBootstrap(false)) {
      return
    }
    if (!streaming.value) {
      setPendingApproval(null)
    }
  }

  function setPendingApproval(data: any) {
    if (!data) {
      pendingApproval.value = null
      if (!streaming.value && !pendingQuestion.value && !pendingExecApproval.value) {
        awaitingConfirmation.value = false
      }
      return
    }
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
      session_id: sessionId || undefined,
      binding_hash: stringifyOptional(data.binding_hash ?? data.bindingHash),
    }
    clearPendingRecoveryRetryTimer()
    awaitingConfirmation.value = true
  }

  // --- Ask-user-question methods ---
  function setPendingQuestion(data: any) {
    const sessionId = normalizePendingSessionId(data)
    if (sessionId) {
      updateActiveStreamState(sessionId, { awaitingConfirmation: !!data })
    }
    if (data && !shouldSurfacePendingForCurrentConversation(sessionId)) {
      return
    }
    if (data) {
      markPendingConfirmationRecoveryChecked(sessionId)
    }
    pendingQuestion.value = data
    if (data) {
      clearPendingRecoveryRetryTimer()
    }
    awaitingConfirmation.value = !!data
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

  async function checkPendingQuestion(force = false) {
    await checkPendingConfirmationsFromBootstrap(force)
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
    if (await checkPendingConfirmationsFromBootstrap(false)) {
      return
    }
    if (!streaming.value) {
      setPendingExecApproval(null)
    }
  }

  async function resolveExecApproval(decision: ExecDecision) {
    const approval = pendingExecApproval.value
    if (!approval) return false
    try {
      await approvalApi.resolve(approval.id, decision, approval.binding_hash)
      setPendingExecApproval(null)
      clearAwaitingConfirmationForSession(approval.session_id || approval.conversation_id)
      return true
    } catch (e) {
      console.error('Failed to resolve exec approval:', e)
      return false
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
      resetCommandStateHydration()
      clearRecentTodoCompletion()
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

  async function setModelPreference(value: string) {
    await applyModelPreference(value, currentConversationId.value).catch(() => {})
  }

  async function setProviderPinOnly(providerId: string) {
    await applyProviderPinOnly(providerId, currentConversationId.value).catch(() => {})
  }

  async function setAgentcoreRunnerRef(value: string) {
    await applyAgentcoreRunnerRef(value, currentConversationId.value).catch(() => {})
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
    recentTodoCompletion,
    loading,
    sending,
    streaming,
    streamingContent,
    processContentLength,
    error,
    streamError,
    streamUIState,
    streamProgress,
    providerAccelerationActive,
    statusStartedAt,
    statusSummary,
    processTrace,
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
    providerPinOnlyActive,
    modelPreference,
    pendingModelAutoFallback,
    pendingProviderFailoverRetry,
    offlineMode,
    agentcoreRunnerRef,
    isRecovering,
    isStreamInterrupted,

    // Computed
    currentConversation,
    sortedConversations,
    executingConversationIds,
    isPreTTFT,

    // Actions
    splitModelPreference,
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
    retryInterruptedStreamRecovery,
    cancelPreTTFT,
    setProviderPinOnly,
    setAgentcoreRunnerRef,
    continueMessage,
    regenerateMessage,
    editMessageAndResubmit,
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
    confirmModelAutoFallbackRetry,
    dismissModelAutoFallbackRetry,
    confirmProviderFailoverRetry,
    dismissProviderFailoverRetry,
  }
})
