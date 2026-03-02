import type {
  TypelessCard,
  TypelessCardTable,
  TypelessCardCode,
  TypelessCardList,
  TypelessCardGallery,
  TypelessCardLink,
  TypelessCardFile,
  TypelessCardTerminal,
  TypelessCardMermaid,
  TypelessCardAccordion,
  TypelessCardSteps,
  StepItem,
  GalleryImage,
  ListItem,
  ParsedContent,
} from '@/types/typeless'
import { i18n } from '@/i18n'
import {
  TYPELESS_MARKER_START,
  TYPELESS_MARKER_END,
} from '@/types/typeless'

// File extensions for different categories
const imageExtensions = ['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp', 'ico']
const documentExtensions = ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'csv', 'json', 'xml', 'md']

// Terminal block language markers (module-level Set for O(1) lookup)
const terminalMarkers = new Set(['terminal', 'console', 'shell-output', 'ansi', 'cli-output'])

// Pre-compiled regexes for line-based parsers (avoid re-creation per call)
const RE_CHECKBOX_ITEM = /^(\s*)[-*+]\s+\[([ xX])\]\s+(.+)$/
const RE_UL_ITEM = /^(\s*)[-*+]\s+(.+)$/
const RE_OL_ITEM = /^(\s*)\d+\.\s+(.+)$/
const RE_IMAGE_INLINE = /!\[([^\]]*)\]\(([^)]+)\)/g
const RE_STANDALONE_URL = /^(https?:\/\/[^\s]+)$/
const RE_FILE_PATH = /^([a-zA-Z]:\\[^\s]+\.[a-zA-Z0-9]+|\/[^\s]+\.[a-zA-Z0-9]+)$/
const RE_TABLE_SEP_CONTENT = /^[\s:-]+$/
const RE_WINDOWS_ABS_PATH = /^[a-zA-Z]:\\/

// Language display names for code blocks
const languageAliases: Record<string, string> = {
  js: 'javascript',
  ts: 'typescript',
  py: 'python',
  rb: 'ruby',
  sh: 'bash',
  shell: 'bash',
  yml: 'yaml',
  md: 'markdown',
}

// ============================================================================
// Card construction helpers — reduce duplication across parse functions
// ============================================================================

/** Push a card and its placeholder into the result arrays. Returns the placeholder string. */
function emitCard(card: TypelessCard, cards: TypelessCard[], result: string[]): void {
  cards.push(card)
  result.push(`[[TYPELESS_CARD:${card.id}]]`)
}

/** Build a TypelessCardCode. */
function makeCodeCard(
  id: string, code: string, language: string | undefined, streaming = false,
): TypelessCardCode {
  return {
    type: 'code', id, code,
    language: language || undefined,
    showLineNumbers: true,
    ...(streaming ? { _streaming: true } : {}),
  } as TypelessCardCode
}

/** Build a TypelessCardTerminal from a terminal type string. */
function makeTerminalCard(
  id: string, content: string, terminalType: string, streaming = false,
): TypelessCardTerminal {
  return {
    type: 'terminal', id, content,
    title: terminalType === 'terminal' ? 'Terminal' : terminalType.charAt(0).toUpperCase() + terminalType.slice(1),
    theme: 'dark',
    ...(streaming ? { _streaming: true } : {}),
  } as TypelessCardTerminal
}

// ============================================================================
// Parse Result Cache - LRU cache for parsed content
// ============================================================================

interface ParseCacheEntry {
  result: ParsedContent
  timestamp: number
}

class ParseResultCache {
  private cache = new Map<string, ParseCacheEntry>()
  private maxSize: number
  private maxAge: number // ms

  constructor(maxSize = 200, maxAgeMs = 5 * 60 * 1000) {
    this.maxSize = maxSize
    this.maxAge = maxAgeMs
  }

  private generateKey(content: string): string {
    // Simple hash for content
    let hash = 0
    for (let i = 0; i < content.length; i++) {
      const char = content.charCodeAt(i)
      hash = ((hash << 5) - hash) + char
      hash = hash & hash
    }
    return hash.toString(36)
  }

  get(content: string): ParsedContent | null {
    const key = this.generateKey(content)
    const entry = this.cache.get(key)

    if (!entry) return null

    // Check if expired
    if (Date.now() - entry.timestamp > this.maxAge) {
      this.cache.delete(key)
      return null
    }

    // Move to end (LRU)
    this.cache.delete(key)
    this.cache.set(key, entry)

    // Return cached object directly to avoid clone/GC overhead on hot render paths.
    // ParsedContent is treated as immutable by consumers.
    return entry.result
  }

  set(content: string, result: ParsedContent): void {
    const key = this.generateKey(content)

    // Evict oldest if at capacity
    if (this.cache.size >= this.maxSize) {
      const firstKey = this.cache.keys().next().value
      if (firstKey) this.cache.delete(firstKey)
    }

    this.cache.set(key, {
      result,
      timestamp: Date.now(),
    })
  }

  clear(): void {
    this.cache.clear()
  }

  get size(): number {
    return this.cache.size
  }
}

// Global parse cache instance
const parseCache = new ParseResultCache()

// Export for testing/debugging
export { parseCache }

// ============================================================================
// Incremental Parsing - For streaming content
// ============================================================================

interface IncrementalParseState {
  lastContent: string
  lastResult: ParsedContent
  hasStreaming: boolean
}

const incrementalStates = new Map<string, IncrementalParseState>()
const INCREMENTAL_CARD_HINT_TAIL = 4
const INCREMENTAL_STATES_MAX = 500

function setIncrementalState(cacheKey: string, state: IncrementalParseState): void {
  if (incrementalStates.size >= INCREMENTAL_STATES_MAX && !incrementalStates.has(cacheKey)) {
    const oldestKey = incrementalStates.keys().next().value
    if (oldestKey !== undefined) {
      incrementalStates.delete(oldestKey as string)
    }
  }
  incrementalStates.set(cacheKey, state)
}

function mightContainIncrementalCardHints(content: string): boolean {
  return content.includes('```')
    || content.includes('|')
    || content.includes('- ')
    || content.includes('* ')
    || content.includes('1. ')
    || content.includes('![')
    || content.includes('http')
}

/**
 * Parse content incrementally (for streaming messages)
 * Only re-parses the new portion of content when possible
 * @param content - The message content to parse
 * @param messageId - The message ID
 * @param conversationId - The conversation ID (optional, for cache key uniqueness)
 */
export function parseTypelessContentIncremental(
  content: string,
  messageId: string,
  conversationId?: string
): ParsedContent {
  // Use conversation_id + message_id as cache key to avoid cross-conversation cache collisions
  const cacheKey = conversationId ? `${conversationId}:${messageId}` : messageId
  const state = incrementalStates.get(cacheKey)

  // Helper: check if any card has _streaming flag
  const checkStreaming = (cards: TypelessCard[]) =>
    cards.some((c) => (c as TypelessCard & { _streaming?: boolean })._streaming)

  // If no previous state or content doesn't start with previous content, do full parse
  if (!state || !content.startsWith(state.lastContent)) {
    const result = parseTypelessContentInternal(content, 0, true) // isStreaming = true
    setIncrementalState(cacheKey, {
      lastContent: content,
      lastResult: result,
      hasStreaming: checkStreaming(result.cards),
    })
    return result
  }

  // Content is an extension of previous content
  const newContent = content.slice(state.lastContent.length)
  if (newContent.length === 0) {
    return state.lastResult
  }
  const tailProbe = state.lastContent.slice(-INCREMENTAL_CARD_HINT_TAIL) + newContent
  const hasIncrementalCardHints = mightContainIncrementalCardHints(tailProbe)

  // If the new chunk is very small and has no card hints, debounce parsing.
  // Preserve streaming-card behavior: always re-parse when a card is in-progress.
  if (newContent.length < 24 && !state.hasStreaming && !hasIncrementalCardHints) {
    return state.lastResult
  }

  // Check if new content might contain new cards or update streaming cards
  const mightHaveNewCards = hasIncrementalCardHints || state.hasStreaming

  if (!mightHaveNewCards) {
    // Just update text, no new cards
    const updatedResult: ParsedContent = {
      text: state.lastResult.text + newContent,
      cards: state.lastResult.cards,
    }
    setIncrementalState(cacheKey, {
      lastContent: content,
      lastResult: updatedResult,
      hasStreaming: state.hasStreaming,
    })
    return updatedResult
  }

  // Need to re-parse (new cards might be present or streaming card updated)
  const result = parseTypelessContentInternal(content, 0, true) // isStreaming = true
  setIncrementalState(cacheKey, {
    lastContent: content,
    lastResult: result,
    hasStreaming: checkStreaming(result.cards),
  })
  return result
}

/**
 * Clear incremental parse state for a message
 * @param messageId - The message ID
 * @param conversationId - The conversation ID (optional, for cache key uniqueness)
 */
export function clearIncrementalState(messageId: string, conversationId?: string): void {
  const cacheKey = conversationId ? `${conversationId}:${messageId}` : messageId
  incrementalStates.delete(cacheKey)
  // Also try to delete with just messageId for backwards compatibility
  if (conversationId) {
    incrementalStates.delete(messageId)
  }
}

/**
 * Clear all incremental parse states
 */
export function clearAllIncrementalStates(): void {
  incrementalStates.clear()
}

/**
 * Clear all incremental parse states for a specific conversation
 * @param conversationId - The conversation ID to clear states for
 */
export function clearConversationIncrementalStates(conversationId: string): void {
  const keysToDelete: string[] = []
  for (const key of incrementalStates.keys()) {
    if (key.startsWith(`${conversationId}:`)) {
      keysToDelete.push(key)
    }
  }
  for (const key of keysToDelete) {
    incrementalStates.delete(key)
  }
}

/**
 * Parse message content and extract typeless cards.
 * Cards are embedded as JSON blocks with ```typeless markers.
 * Also converts markdown tables, code blocks, and lists to cards.
 * Uses LRU cache for performance.
 */
export function parseTypelessContent(content: string): ParsedContent {
  // Check cache first
  const cached = parseCache.get(content)
  if (cached) return cached

  // Parse and cache
  const result = parseTypelessContentInternal(content, 0)
  parseCache.set(content, result)
  return result
}

/**
 * Normalize malformed JSON-like strings:
 * - Convert single-quoted strings to double-quoted
 * - Add double quotes around unquoted keys
 * - Remove trailing commas before } or ]
 */
function normalizeLooseJSON(str: string): string {
  let result = ''
  let i = 0
  const len = str.length

  while (i < len) {
    const ch = str[i]!

    // Skip whitespace
    if (ch === ' ' || ch === '\t' || ch === '\n' || ch === '\r') {
      result += ch
      i++
      continue
    }

    // Double-quoted string — pass through as-is
    if (ch === '"') {
      result += '"'
      i++
      while (i < len) {
        const c = str[i]!
        result += c
        if (c === '\\' && i + 1 < len) { result += str[i + 1]!; i += 2; continue }
        if (c === '"') { i++; break }
        i++
      }
      continue
    }

    // Single-quoted string → convert to double-quoted
    if (ch === "'") {
      result += '"'
      i++
      while (i < len) {
        const c = str[i]!
        if (c === '\\' && i + 1 < len) { result += '\\'; result += str[i + 1]!; i += 2; continue }
        if (c === "'") { i++; break }
        if (c === '"') { result += '\\"'; i++; continue } // escape inner double quotes
        result += c
        i++
      }
      result += '"'
      continue
    }

    // Trailing comma before } or ] — skip the comma
    if (ch === ',') {
      // Look ahead past whitespace for } or ]
      let j = i + 1
      while (j < len && (str[j] === ' ' || str[j] === '\t' || str[j] === '\n' || str[j] === '\r')) j++
      if (j >= len || str[j] === '}' || str[j] === ']') {
        i++ // skip trailing comma
        continue
      }
      result += ch
      i++
      continue
    }

    // Unquoted key: identifier followed by ':'
    if ((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch === '_' || ch === '$') {
      // Check if we're in a position where a key is expected (after { or ,)
      const lastNonWS = result.trimEnd()
      const lastChar = lastNonWS[lastNonWS.length - 1]
      if (lastChar === '{' || lastChar === ',') {
        // Collect identifier
        let ident = ''
        while (i < len && /[a-zA-Z0-9_$]/.test(str[i]!)) {
          ident += str[i]!
          i++
        }
        // Skip whitespace to check for colon
        while (i < len && (str[i] === ' ' || str[i] === '\t')) i++
        if (i < len && str[i] === ':') {
          result += `"${ident}"`
          continue
        }
        // Not a key — output as-is (could be a bare value like true/false/null)
        result += ident
        continue
      }
    }

    // Everything else — pass through
    result += ch
    i++
  }

  return result
}

/**
 * Try to parse potentially incomplete JSON by adding missing closing brackets/braces.
 * Also handles malformed JSON: unquoted keys, single quotes, trailing commas.
 * This is useful for streaming scenarios where JSON arrives incrementally.
 */
function tryParseIncompleteJSON(jsonStr: string): unknown | null {
  // First try normal parse
  try {
    return JSON.parse(jsonStr)
  } catch {
    // Try to fix incomplete JSON
  }

  // Try normalizing loose JSON first (single quotes, unquoted keys, trailing commas)
  let normalized = normalizeLooseJSON(jsonStr)

  // Count opening and closing brackets/braces
  const closeBrackets = (s: string): string => {
    let braceCount = 0
    let bracketCount = 0
    let inString = false
    let escapeNext = false

    for (const char of s) {
      if (escapeNext) { escapeNext = false; continue }
      if (char === '\\') { escapeNext = true; continue }
      if (char === '"') { inString = !inString; continue }
      if (inString) continue
      if (char === '{') braceCount++
      else if (char === '}') braceCount--
      else if (char === '[') bracketCount++
      else if (char === ']') bracketCount--
    }

    let fixed = s
    if (inString) fixed += '"'
    while (bracketCount > 0) { fixed += ']'; bracketCount-- }
    while (braceCount > 0) { fixed += '}'; braceCount-- }
    return fixed
  }

  // Try normalized + closed brackets
  const closed = closeBrackets(normalized)
  try {
    return JSON.parse(closed)
  } catch {
    // Still failed, try more aggressive fixes
  }

  // Try removing trailing incomplete property
  // e.g., {"type": "progress", "title": "Test", "pro  -> {"type": "progress", "title": "Test"}
  const lastCommaIndex = closed.lastIndexOf(',')
  if (lastCommaIndex > 0) {
    const truncated = closeBrackets(closed.slice(0, lastCommaIndex))
    try {
      return JSON.parse(truncated)
    } catch {
      // Give up
    }
  }

  return null
}

// Tool icon mapping (icons don't need i18n)
const toolIconMap: Record<string, string> = {
  'Web Search': '🔍',
  'Calculator': '🧮',
  'System Info': '💻',
  'Current Time': '🕐',
  'File Read': '📄',
  'File Write': '📝',
  'memory': '🧠',
  'scheduler': '📅',
  'browser': '🌐',
  'sandbox': '📦',
  'ui_reviewer': '👁️',
  'autoreply': '💬',
  'workflows': '⚙️',
  'analyze': '📊',
}

// Parameters to show as keyword-style (just the value, no label)
const keywordParams = new Set(['query', 'keyword', 'expression'])

// Helper to get i18n translation with fallback
function t(key: string, fallback: string, named?: Record<string, string | number>): string {
  const result = named ? i18n.global.t(key, named) : i18n.global.t(key)
  return result === key ? fallback : result
}

/**
 * Extract tool invocations from a <function_calls> block into StepItems.
 */
function extractInvocations(innerXml: string): StepItem[] {
  const steps: StepItem[] = []
  const invokeRegex = /<(?:antml:)?invoke\s+name="([^"]+)">([\s\S]*?)<\/(?:antml:)?invoke>/g
  let inv
  while ((inv = invokeRegex.exec(innerXml)) !== null) {
    const rawName = inv[1] || 'unknown'
    const paramsBlock = inv[2] || ''
    const displayName = t(`tools.names.${rawName}`, rawName)
    const icon = toolIconMap[rawName] || '🔧'
    const keywords: string[] = []
    const tags: string[] = []
    const paramRegex = /<(?:antml:)?parameter\s+name="([^"]+)">([\s\S]*?)<\/(?:antml:)?parameter>/g
    let p
    while ((p = paramRegex.exec(paramsBlock)) !== null) {
      const paramName = p[1] || ''
      const val = (p[2] || '').trim()
      const truncated = val.length > 60 ? val.slice(0, 60) + '...' : val
      if (keywordParams.has(paramName)) {
        keywords.push(truncated)
      } else {
        const label = t(`tools.params.${paramName}`, paramName)
        tags.push(`${label}: ${truncated}`)
      }
    }
    const titleSuffix = keywords.length ? `  ${keywords.join(' ')}` : ''
    steps.push({
      title: displayName + titleSuffix,
      description: tags.join('|') || undefined,
      icon,
      status: 'completed',
    })
  }
  return steps
}

/**
 * Parse complete <function_calls>...</function_calls> blocks into steps cards.
 */
function parseFunctionCalls(text: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const fcRegex = /<(?:antml:)?function_calls>([\s\S]*?)<\/(?:antml:)?function_calls>/g
  const reps: { start: number; end: number; placeholder: string }[] = []
  let m
  while ((m = fcRegex.exec(text)) !== null) {
    const steps = extractInvocations(m[1] || '')
    if (steps.length === 0) continue
    const card: TypelessCardSteps = {
      type: 'steps',
      id: `fc-${cardIndex.value++}`,
      title: t('tools.callCount', `Tool Calls (${steps.length})`, { count: steps.length }),
      steps,
      variant: 'vertical',
    }
    cards.push(card)
    reps.push({ start: m.index, end: m.index + m[0].length, placeholder: `[[TYPELESS_CARD:${card.id}]]` })
  }
  for (let i = reps.length - 1; i >= 0; i--) {
    const r = reps[i]!
    text = text.slice(0, r.start) + r.placeholder + text.slice(r.end)
  }
  return text
}

/**
 * Parse incomplete/streaming <function_calls> (no closing tag yet).
 */
function parseIncompleteFunctionCalls(text: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const incRegex = /<(?:antml:)?function_calls>([\s\S]*)$/
  const m = incRegex.exec(text)
  if (!m || !m[1]) return text
  const steps = extractInvocations(m[1])
  // Also check for an incomplete <invoke (plain or antml: prefixed) that hasn't closed yet
  const lastInvokeIdx = Math.max(m[1].lastIndexOf('<invoke'), m[1].lastIndexOf('<antml:invoke'))
  const partialInvoke = lastInvokeIdx >= 0
    ? /<(?:antml:)?invoke\s+name="([^"]*)"/.exec(m[1].slice(lastInvokeIdx))
    : null
  if (steps.length === 0 && !partialInvoke) return text
  if (partialInvoke && (steps.length === 0 || steps[steps.length - 1]?.title !== partialInvoke[1])) {
    steps.push({ title: partialInvoke[1] || 'loading...', icon: '⏳', status: 'current' })
  }
  const card: TypelessCardSteps = {
    type: 'steps',
    id: `fc-streaming-${cardIndex.value++}`,
    title: t('tools.callingProgress', 'Calling Tools...'),
    steps,
    variant: 'vertical',
    _streaming: true,
  }
  cards.push(card)
  return text.slice(0, m.index) + `[[TYPELESS_CARD:${card.id}]]`
}

/**
 * Parse special XML-like tags and convert to cards or remove them
 * Handles: <thinking>...</thinking>, <system_placeholder />, etc.
 */
function parseSpecialTags(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  if (!content) return content
  // Fast path: most assistant replies are plain text and contain none of these markers.
  if (!content.includes('<') && !content.includes('[SILENT_REPLY]')) {
    return content
  }

  let text = content

  // Remove <system_placeholder /> and similar self-closing system tags
  text = text.replace(/<system_placeholder\s*\/>/g, '')
  text = text.replace(/<system-reminder>[\s\S]*?<\/system-reminder>/g, '')

  // Replace [SILENT_REPLY] with a subtle icon
  text = text.replace(/\[SILENT_REPLY\]/g, '💤')

  // Parse thinking tags and convert to collapsible accordion
  // Supports: <thinking>...</thinking>, <think_context>...</think_context>, （ohan）...（ohan）
  const thinkingRegexes = [
    { regex: /<thinking>([\s\S]*?)<\/thinking>/g, title: '💭 ' + t('thinking.title', 'Thinking Process'), prefix: 'thinking' },
    { regex: /<think_context>([\s\S]*?)<\/think_context>/g, title: '💭 ' + t('thinking.context', 'Context'), prefix: 'think_context' },
    { regex: /<think>([\s\S]*?)<\/think>/g, title: '💭 ' + t('thinking.think', 'Think'), prefix: 'think' },
  ]
  const replacements: { start: number; end: number; placeholder: string }[] = []

  for (const { regex, title, prefix } of thinkingRegexes) {
    let match
    while ((match = regex.exec(text)) !== null) {
      const thinkingContent = match[1]?.trim()
      if (thinkingContent) {
        const card: TypelessCardAccordion = {
          type: 'accordion',
          id: `${prefix}-${cardIndex.value++}`,
          title,
          items: [{
            title: t('thinking.expand', 'Expand'),
            content: thinkingContent,
            defaultOpen: false,
          }],
          allowMultiple: false,
        }
        cards.push(card)
        replacements.push({
          start: match.index,
          end: match.index + match[0].length,
          placeholder: `[[TYPELESS_CARD:${card.id}]]`,
        })
      }
    }
  }

  // Replace in reverse order to preserve indices
  for (let i = replacements.length - 1; i >= 0; i--) {
    const replacement = replacements[i]
    if (replacement) {
      const { start, end, placeholder } = replacement
      text = text.slice(0, start) + placeholder + text.slice(end)
    }
  }

  // Parse <function_calls>...</function_calls> blocks into steps cards
  text = parseFunctionCalls(text, cards, cardIndex)

  // Handle incomplete/streaming <function_calls> (no closing tag yet)
  text = parseIncompleteFunctionCalls(text, cards, cardIndex)

  // Handle incomplete/streaming thinking tags (no closing tag yet)
  const incompleteThinkingRegexes = [
    { regex: /<thinking>([\s\S]*)$/, prefix: 'thinking' },
    { regex: /<think_context>([\s\S]*)$/, prefix: 'think_context' },
    { regex: /<think>([\s\S]*)$/, prefix: 'think' },
  ]

  for (const { regex, prefix } of incompleteThinkingRegexes) {
    const incompleteMatch = regex.exec(text)
    if (incompleteMatch && incompleteMatch[1]) {
      const thinkingContent = incompleteMatch[1].trim()
      // If first char is '<' and total length < 10, hide everything (no card, no text)
      // This prevents showing empty/minimal thinking UI at the start of streaming
      if (text[0] === '<' && text.length < 10) {
        text = text.slice(0, incompleteMatch.index)
        break
      }
      if (thinkingContent) {
        const card: TypelessCardAccordion = {
          type: 'accordion',
          id: `${prefix}-streaming-${cardIndex.value++}`,
          title: '💭 ' + t('thinking.inProgress', 'Thinking...'),
          items: [{
            title: t('thinking.expand', 'Expand'),
            content: thinkingContent,
            defaultOpen: false,
          }],
          allowMultiple: false,
          _streaming: true,
        }
        cards.push(card)
        text = text.slice(0, incompleteMatch.index) + `[[TYPELESS_CARD:${card.id}]]`
        break // Only handle the first match
      }
    }
  }

  return text
}

/**
 * Internal parsing function (no caching)
 */
function parseTypelessContentInternal(content: string, startCardIndex: number, isStreaming = false): ParsedContent {
  const cards: TypelessCard[] = []
  let text = content
  const cardIndex = { value: startCardIndex }

  // Valid card type check — reject empty or clearly invalid types during streaming
  const isValidCardType = (type: string) => type.length > 0 && type.length < 30 && /^[a-z][a-z0-9-]*$/.test(type)

  // First, parse special XML-like tags
  text = parseSpecialTags(text, cards, cardIndex)

  // Find all complete typeless blocks
  // We need a more robust approach: find all ```typeless markers, then find the LAST ``` in the content
  // This handles cases where the JSON content contains inner code fences like ```mermaid

  const markerStart = TYPELESS_MARKER_START
  const markerEnd = TYPELESS_MARKER_END

  // Find all occurrences of ```typeless from the current working text.
  // Important: text may already be transformed by parseSpecialTags(), so
  // indexes must be computed against `text` (not original `content`) to avoid
  // replacement offsets during streaming.
  const startMatches: number[] = []
  let searchStart = 0
  while (true) {
    const idx = text.indexOf(markerStart, searchStart)
    if (idx === -1) break
    startMatches.push(idx)
    searchStart = idx + markerStart.length
  }

  // For each start marker, locate a closing fence that yields a valid card.
  // Important: do NOT blindly pair with the last fence — that can swallow
  // multiple consecutive typeless blocks into one invalid JSON payload.
  const replacements: { start: number; end: number; placeholder: string }[] = []
  for (const startIdx of startMatches) {
    const contentStart = startIdx + markerStart.length
    const remainingContent = text.slice(contentStart)

    const fencePositions: number[] = []
    let fenceSearchStart = 0
    while (true) {
      const fenceIdx = remainingContent.indexOf(markerEnd, fenceSearchStart)
      if (fenceIdx === -1) break
      fencePositions.push(fenceIdx)
      fenceSearchStart = fenceIdx + markerEnd.length
    }
    if (fencePositions.length === 0) continue

    let parsedCard: TypelessCard | null = null
    let endIdx = -1

    // Pass 1: strict JSON parse, pick the earliest valid closing fence.
    // This preserves multiple consecutive typeless blocks.
    for (const fenceIdx of fencePositions) {
      const jsonStr = remainingContent.slice(0, fenceIdx).trim()
      if (!jsonStr) continue

      try {
        const candidate = JSON.parse(jsonStr) as TypelessCard
        if (candidate && typeof candidate.type === 'string' && isValidCardType(candidate.type)) {
          parsedCard = candidate
          endIdx = contentStart + fenceIdx + markerEnd.length
          break
        }
      } catch {
        // Keep scanning later fences (handles inner ``` inside JSON strings).
      }
    }

    // Pass 2: lenient fallback on the last fence only.
    // We intentionally avoid lenient parsing on earlier fences, because it can
    // mistakenly accept truncated JSON and hide subsequent cards.
    if (!parsedCard) {
      const lastFenceIdx = fencePositions[fencePositions.length - 1]!
      const jsonStr = remainingContent.slice(0, lastFenceIdx).trim()
      const lastChar = jsonStr[jsonStr.length - 1]
      const looksComplete = lastChar === '}' || lastChar === ']'
      if (jsonStr && looksComplete) {
        const candidate = tryParseIncompleteJSON(jsonStr) as TypelessCard | null
        if (candidate && typeof candidate.type === 'string' && isValidCardType(candidate.type)) {
          parsedCard = candidate
          endIdx = contentStart + lastFenceIdx + markerEnd.length
        }
      }
    }

    if (parsedCard && endIdx > contentStart) {
      if (!parsedCard.id) {
        parsedCard.id = `card-${cardIndex.value++}`
      }
      cards.push(parsedCard)
      replacements.push({
        start: startIdx,
        end: endIdx,
        placeholder: `[[TYPELESS_CARD:${parsedCard.id}]]`,
      })
    }
  }

  // During streaming, also try to parse incomplete typeless blocks
  // Look for blocks that start with marker but don't have closing marker yet
  if (isStreaming) {
    const incompleteRegex = new RegExp(
      `${escapeRegex(TYPELESS_MARKER_START)}\\s*([\\s\\S]*)$`
    )
    const incompleteMatch = incompleteRegex.exec(text)
    if (incompleteMatch && incompleteMatch[1]) {
      const jsonStr = incompleteMatch[1].trim()
      // Only try to parse if it looks like JSON (starts with {)
      if (jsonStr.startsWith('{')) {
        const partialCard = tryParseIncompleteJSON(jsonStr) as TypelessCard | null
        if (partialCard && typeof partialCard.type === 'string' && isValidCardType(partialCard.type)) {
          // Mark as streaming/incomplete
          partialCard._streaming = true
          if (!partialCard.id) {
            partialCard.id = `card-streaming-${cardIndex.value++}`
          }
          cards.push(partialCard)

          // Mark for replacement with placeholder
          replacements.push({
            start: incompleteMatch.index,
            end: text.length,
            placeholder: `[[TYPELESS_CARD:${partialCard.id}]]`,
          })
        }
      }
    }
  }

  // Replace card blocks with placeholders (in reverse order to preserve indices)
  for (let i = replacements.length - 1; i >= 0; i--) {
    const replacement = replacements[i]
    if (replacement) {
      const { start, end, placeholder } = replacement
      text = text.slice(0, start) + placeholder + text.slice(end)
    }
  }

  // Parse markdown elements and convert to cards — SINGLE PASS over lines
  text = parseMarkdownElementsSinglePass(text, cards, cardIndex)

  return { text, cards }
}

// Pre-compiled regexes for hasTypelessCards — avoids re-creating RegExp objects on every call
const RE_CODE_FENCE = /```[a-z]/i
const RE_IMAGE_LINE = /^\s*!\[[^\]]*\]\([^)]+\)\s*$/m
const RE_URL_LINE = /^\s*https?:\/\/[^\s]+\s*$/m
const RE_FILE_PATH_LINE = /^\s*([a-zA-Z]:\\[^\s]+\.[a-zA-Z0-9]+|\/[^\s]+\.[a-zA-Z0-9]+)\s*$/m
const RE_DIGIT_DOT = /\d+\.\s/
const HAS_TYPELESS_CACHE_MAX = 400
const HAS_TYPELESS_CACHE_MAX_CONTENT_LENGTH = 12000
const hasTypelessCardsCache = new Map<string, boolean>()

function evictOldestMapEntry<K, V>(cache: Map<K, V>): void {
  const oldestKey = cache.keys().next().value
  if (oldestKey !== undefined) {
    cache.delete(oldestKey as K)
  }
}

function isWhitespaceCharCode(code: number): boolean {
  return code === 32 || (code >= 9 && code <= 13)
}

function lineHasNonWhitespace(content: string, start: number, end: number): boolean {
  for (let index = start; index < end; index++) {
    if (!isWhitespaceCharCode(content.charCodeAt(index))) {
      return true
    }
  }
  return false
}

function lineIncludesPipe(content: string, start: number, end: number): boolean {
  for (let index = start; index < end; index++) {
    if (content.charCodeAt(index) === 124 /* | */) {
      return true
    }
  }
  return false
}

function isSeparatorLine(content: string, start: number, end: number): boolean {
  let hasDash = false
  for (let index = start; index < end; index++) {
    const code = content.charCodeAt(index)
    if (code === 45 /* - */) {
      hasDash = true
      continue
    }
    if (code === 124 /* | */ || code === 58 /* : */ || isWhitespaceCharCode(code)) {
      continue
    }
    return false
  }
  return hasDash
}

function isUnorderedListItemLine(content: string, start: number, end: number): boolean {
  let index = start
  while (index < end && isWhitespaceCharCode(content.charCodeAt(index))) {
    index++
  }
  if (index >= end) return false

  const marker = content.charCodeAt(index)
  if (marker !== 45 /* - */ && marker !== 42 /* * */ && marker !== 43 /* + */) {
    return false
  }

  index++
  if (index >= end) return false
  return isWhitespaceCharCode(content.charCodeAt(index))
}

function isOrderedListItemLine(content: string, start: number, end: number): boolean {
  let index = start
  while (index < end && isWhitespaceCharCode(content.charCodeAt(index))) {
    index++
  }
  if (index >= end) return false

  let hasDigit = false
  while (index < end) {
    const code = content.charCodeAt(index)
    if (code < 48 || code > 57) break
    hasDigit = true
    index++
  }
  if (!hasDigit || index >= end || content.charCodeAt(index) !== 46 /* . */) {
    return false
  }

  index++
  if (index >= end) return false
  return isWhitespaceCharCode(content.charCodeAt(index))
}

/**
 * Check if content contains any typeless cards or markdown elements that will be converted.
 * Optimized with fast string checks before regex, and pre-compiled regexes.
 */
export function hasTypelessCards(content: string, useCache = true): boolean {
  const shouldUseCache = useCache
    && content.length > 0
    && content.length <= HAS_TYPELESS_CACHE_MAX_CONTENT_LENGTH

  if (shouldUseCache) {
    const cached = hasTypelessCardsCache.get(content)
    if (cached !== undefined) return cached
  }

  const finalize = (result: boolean): boolean => {
    if (!shouldUseCache) return result
    if (hasTypelessCardsCache.size >= HAS_TYPELESS_CACHE_MAX && !hasTypelessCardsCache.has(content)) {
      evictOldestMapEntry(hasTypelessCardsCache)
    }
    hasTypelessCardsCache.set(content, result)
    return result
  }

  // Fast path: check for explicit typeless blocks (cheapest check)
  if (content.includes(TYPELESS_MARKER_START)) {
    return finalize(true)
  }

  // Fast path: code fences (covers terminal, mermaid, and regular code blocks)
  // Use includes('```') as a cheap gate before regex
  if (content.includes('```') && RE_CODE_FENCE.test(content)) {
    return finalize(true)
  }

  // Fast path: markdown images — gate with '!['
  if (content.includes('![') && RE_IMAGE_LINE.test(content)) {
    return finalize(true)
  }

  // Fast path: standalone URLs — gate with 'http'
  if (content.includes('http') && RE_URL_LINE.test(content)) {
    return finalize(true)
  }

  // Fast path: file paths — gate with common path separators
  if ((content.includes(':\\') || content.includes('/')) && RE_FILE_PATH_LINE.test(content)) {
    return finalize(true)
  }

  // Tables and lists require line-by-line scan — gate with cheap checks
  const hasPipe = content.includes('|')
  const hasDash = content.includes('- ') || content.includes('* ') || content.includes('+ ')
  const hasDigitDot = content.includes('.') && RE_DIGIT_DOT.test(content)

  if (!hasPipe && !hasDash && !hasDigitDot) {
    return finalize(false)
  }

  // Multi-line structures need at least one line break.
  if (!content.includes('\n')) {
    return finalize(false)
  }

  // Single pass line scan for both table and list detection.
  const hasList = hasDash || hasDigitDot
  let tableRowCount = 0
  let listItemCount = 0
  let lineStart = 0

  while (lineStart <= content.length) {
    const newlineIndex = content.indexOf('\n', lineStart)
    const lineEnd = newlineIndex === -1 ? content.length : newlineIndex
    let lineHasContentCached: boolean | null = null
    const hasLineContent = (): boolean => {
      if (lineHasContentCached === null) {
        lineHasContentCached = lineHasNonWhitespace(content, lineStart, lineEnd)
      }
      return lineHasContentCached
    }

    let lineIsSeparatorCached: boolean | null = null
    const isSeparator = (): boolean => {
      if (lineIsSeparatorCached === null) {
        lineIsSeparatorCached = isSeparatorLine(content, lineStart, lineEnd)
      }
      return lineIsSeparatorCached
    }

    if (hasPipe) {
      if (lineIncludesPipe(content, lineStart, lineEnd) && !isSeparator()) {
        tableRowCount++
        if (tableRowCount >= 2) return finalize(true)
      } else if (tableRowCount > 0 && hasLineContent() && !isSeparator()) {
        tableRowCount = 0
      }
    }

    if (hasList) {
      if (
        isUnorderedListItemLine(content, lineStart, lineEnd)
        || isOrderedListItemLine(content, lineStart, lineEnd)
      ) {
        listItemCount++
        if (listItemCount >= 2) return finalize(true)
      } else if (hasLineContent()) {
        listItemCount = 0
      }
    }

    if (newlineIndex === -1) break
    lineStart = newlineIndex + 1
  }

  return finalize(false)
}

/**
 * Create a typeless card JSON block for embedding in messages.
 */
export function createTypelessBlock(card: TypelessCard): string {
  return `${TYPELESS_MARKER_START}\n${JSON.stringify(card, null, 2)}\n${TYPELESS_MARKER_END}`
}

/**
 * Get card placeholder pattern for splitting text.
 */
export function getCardPlaceholderPattern(): RegExp {
  return RE_CARD_PLACEHOLDER
}

const RE_CARD_PLACEHOLDER = /\[\[TYPELESS_CARD:([^\]]+)\]\]/g
const CARD_PLACEHOLDER_START = '[[TYPELESS_CARD:'
const CARD_PLACEHOLDER_END = ']]'
const RE_EDGE_WHITESPACE = /^\s|\s$/
const CARD_LOOKUP_CACHE_KEY = '__zima_typeless_card_lookup_cache_v1__'
const SPLIT_SEGMENTS_INCREMENTAL_CACHE_KEY = '__zima_typeless_split_segments_incremental_cache_v1__'
const SPLIT_SEGMENTS_INCREMENTAL_CACHE_MAX = 300

type SplitSegment = { type: 'text' | 'card'; content: string | TypelessCard }

type CardLookupCacheEntry = {
  size: number
  map: Map<string, TypelessCard>
}

type SplitSegmentsIncrementalCacheEntry = {
  text: string
  cards: TypelessCard[]
  segments: SplitSegment[]
}

function trimIfNeeded(text: string): string {
  return RE_EDGE_WHITESPACE.test(text) ? text.trim() : text
}

function getCardLookupCache(): WeakMap<TypelessCard[], CardLookupCacheEntry> {
  const g = globalThis as Record<string, unknown>
  const existing = g[CARD_LOOKUP_CACHE_KEY]
  if (existing instanceof WeakMap) {
    return existing as WeakMap<TypelessCard[], CardLookupCacheEntry>
  }
  const cache = new WeakMap<TypelessCard[], CardLookupCacheEntry>()
  g[CARD_LOOKUP_CACHE_KEY] = cache
  return cache
}

function getCardLookupMap(cards: TypelessCard[]): Map<string, TypelessCard> | null {
  if (cards.length === 0) return null

  const cache = getCardLookupCache()
  const cached = cache.get(cards)
  if (cached && cached.size === cards.length) {
    return cached.map
  }

  const map = new Map<string, TypelessCard>()
  for (const card of cards) {
    if (card.id) map.set(card.id, card)
  }
  cache.set(cards, {
    size: cards.length,
    map,
  })
  return map
}

function getSplitSegmentsIncrementalCache(): Map<string, SplitSegmentsIncrementalCacheEntry> {
  const g = globalThis as Record<string, unknown>
  const existing = g[SPLIT_SEGMENTS_INCREMENTAL_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, SplitSegmentsIncrementalCacheEntry>
  }
  const cache = new Map<string, SplitSegmentsIncrementalCacheEntry>()
  g[SPLIT_SEGMENTS_INCREMENTAL_CACHE_KEY] = cache
  return cache
}

function setSplitSegmentsIncrementalCache(
  cache: Map<string, SplitSegmentsIncrementalCacheEntry>,
  key: string,
  text: string,
  cards: TypelessCard[],
  segments: SplitSegment[]
) {
  if (cache.size >= SPLIT_SEGMENTS_INCREMENTAL_CACHE_MAX && !cache.has(key)) {
    evictOldestMapEntry(cache)
  }
  cache.set(key, {
    text,
    cards,
    segments,
  })
}

export function clearSplitSegmentsIncrementalState(incrementalKey?: string): void {
  const cache = getSplitSegmentsIncrementalCache()
  if (!incrementalKey) {
    cache.clear()
    return
  }
  cache.delete(incrementalKey)
}

function isProgressMergeCandidateCard(card: TypelessCard): boolean {
  return card.type === 'ui-review-progress' || card.type === 'analyze-progress' || card.type === 'browser-progress'
}

function getProgressMergeCandidateType(card: TypelessCard): 'ui-review-progress' | 'analyze-progress' | 'browser-progress' | null {
  if (card.type === 'ui-review-progress' || card.type === 'analyze-progress' || card.type === 'browser-progress') {
    return card.type
  }
  return null
}

function hasUnclosedCardPlaceholder(content: string): boolean {
  const lastStart = content.lastIndexOf(CARD_PLACEHOLDER_START)
  if (lastStart === -1) return false
  const idStart = lastStart + CARD_PLACEHOLDER_START.length
  return content.indexOf(CARD_PLACEHOLDER_END, idStart) === -1
}

/**
 * Split parsed text into segments (text and card placeholders).
 */
export function splitIntoSegments(
  text: string,
  cards: TypelessCard[],
  incrementalKey?: string
): SplitSegment[] {
  const incrementalCache = incrementalKey ? getSplitSegmentsIncrementalCache() : null
  const finalize = (segments: SplitSegment[]): SplitSegment[] => {
    if (incrementalCache && incrementalKey) {
      setSplitSegmentsIncrementalCache(incrementalCache, incrementalKey, text, cards, segments)
    }
    return segments
  }

  if (incrementalCache && incrementalKey) {
    const cached = incrementalCache.get(incrementalKey)
    if (cached && cached.cards === cards && text.startsWith(cached.text)) {
      const delta = text.slice(cached.text.length)
      const previous = cached.segments
      const hasOpenPlaceholder = hasUnclosedCardPlaceholder(cached.text)
      if (!delta.includes(CARD_PLACEHOLDER_START)) {
        if (hasOpenPlaceholder && delta.includes(CARD_PLACEHOLDER_END)) {
          // A placeholder may complete across chunk boundary; re-scan full text.
        } else {
          const lastSegment = previous[previous.length - 1]
          if (lastSegment?.type === 'text') {
            const nextTail = trimIfNeeded((lastSegment.content as string) + delta)
            if (nextTail === lastSegment.content) {
              return finalize(previous)
            }
            const next = previous.slice()
            next[next.length - 1] = { type: 'text', content: nextTail }
            return finalize(next)
          }

          const tailText = trimIfNeeded(delta)
          if (!tailText) {
            return finalize(previous)
          }
          return finalize(previous.concat({ type: 'text', content: tailText }))
        }
      }

      // Incrementally parse appended placeholders when the previous content
      // does not end with an unfinished placeholder across the chunk boundary.
      if (!hasOpenPlaceholder) {
        const appended = splitIntoSegments(delta, cards)
        if (appended.length === 0) {
          return finalize(previous)
        }

        let appendStart = 0
        const next = previous.slice()
        const previousLast = next[next.length - 1]
        const appendedFirst = appended[0]
        if (previousLast?.type === 'text' && appendedFirst?.type === 'text') {
          const merged = trimIfNeeded((previousLast.content as string) + (appendedFirst.content as string))
          if (merged) {
            next[next.length - 1] = { type: 'text', content: merged }
          } else {
            next.pop()
          }
          appendStart = 1
        }

        if (appendStart < appended.length) {
          const tail = appended.slice(appendStart)
          const boundaryPrevious = next[next.length - 1]
          const boundaryNext = tail[0]
          if (boundaryPrevious?.type === 'card' && boundaryNext?.type === 'card') {
            const previousCard = boundaryPrevious.content as TypelessCard
            const nextCard = boundaryNext.content as TypelessCard
            const previousType = getProgressMergeCandidateType(previousCard)
            const nextType = getProgressMergeCandidateType(nextCard)
            if (previousType && previousType === nextType) {
              const boundaryMerged = mergeConsecutiveProgressCards([
                { type: 'card', content: previousCard },
                { type: 'card', content: nextCard },
              ])
              if (boundaryMerged.length === 1 && boundaryMerged[0]?.type === 'card') {
                next[next.length - 1] = boundaryMerged[0]
                next.push(...tail.slice(1))
                return finalize(next)
              }
            }
          }
          next.push(...tail)
        }
        return finalize(next)
      }
    }
  }

  if (!text.includes(CARD_PLACEHOLDER_START)) {
    const plainText = trimIfNeeded(text)
    return finalize(plainText ? [{ type: 'text', content: plainText }] : [])
  }

  const segments: SplitSegment[] = []
  const cardMap = getCardLookupMap(cards)
  let hasMergeCandidate = false

  let lastIndex = 0
  let searchIndex = 0
  while (searchIndex < text.length) {
    const start = text.indexOf(CARD_PLACEHOLDER_START, searchIndex)
    if (start === -1) {
      break
    }

    const idStart = start + CARD_PLACEHOLDER_START.length
    const end = text.indexOf(CARD_PLACEHOLDER_END, idStart)
    if (end === -1) {
      break
    }

    // Invalid placeholder with empty id: keep scanning, treat as plain text.
    if (end === idStart) {
      searchIndex = end + CARD_PLACEHOLDER_END.length
      continue
    }

    // Add text before the placeholder
    if (start > lastIndex) {
      const textContent = trimIfNeeded(text.slice(lastIndex, start))
      if (textContent) {
        segments.push({ type: 'text', content: textContent })
      }
    }

    // Add the card
    const cardId = text.slice(idStart, end)
    const card = cardMap?.get(cardId)
    if (card) {
      if (isProgressMergeCandidateCard(card)) {
        hasMergeCandidate = true
      }
      segments.push({ type: 'card', content: card })
    }

    lastIndex = end + CARD_PLACEHOLDER_END.length
    searchIndex = lastIndex
  }

  // Add remaining text
  if (lastIndex < text.length) {
    const textContent = trimIfNeeded(text.slice(lastIndex))
    if (textContent) {
      segments.push({ type: 'text', content: textContent })
    }
  }

  if (!hasMergeCandidate) {
    return finalize(segments)
  }

  // Merge consecutive ui-review-progress cards into a single aggregated card
  return finalize(mergeConsecutiveProgressCards(segments))
}

/**
 * Merge consecutive ui-review-progress card segments into a single card
 * with a `steps` array, so the component renders them as one consolidated view.
 * Also merges consecutive analyze-progress cards the same way.
 */
function mergeConsecutiveProgressCards(
  segments: SplitSegment[]
): SplitSegment[] {
  const result: typeof segments = []
  let pendingSteps: TypelessCard[] = []
  let pendingType: 'ui-review-progress' | 'analyze-progress' | 'browser-progress' | null = null

  const flushPending = () => {
    if (pendingSteps.length === 0 || !pendingType) return
    const first = pendingSteps[0]!
    const firstRecord = first as unknown as Record<string, unknown>
    const merged: TypelessCard = {
      type: pendingType,
      id: (firstRecord.id as string) || `${pendingType}-merged`,
      steps: pendingSteps.map(s => ({
        step: (s as unknown as Record<string, unknown>).step,
        name: (s as unknown as Record<string, unknown>).name,
        status: (s as unknown as Record<string, unknown>).status,
        url: (s as unknown as Record<string, unknown>).url,
        score: (s as unknown as Record<string, unknown>).score,
      })),
      _streaming: pendingSteps.some(s => s._streaming),
    } as TypelessCard
    result.push({ type: 'card', content: merged })
    pendingSteps = []
    pendingType = null
  }

  for (const seg of segments) {
    const cardType = seg.type === 'card' ? (seg.content as TypelessCard).type : null
    if (cardType === 'ui-review-progress' || cardType === 'analyze-progress' || cardType === 'browser-progress') {
      if (pendingType && pendingType !== cardType) {
        flushPending()
      }
      pendingType = cardType as 'ui-review-progress' | 'analyze-progress' | 'browser-progress'
      pendingSteps.push(seg.content as TypelessCard)
    } else {
      flushPending()
      result.push(seg)
    }
  }
  flushPending()
  return result
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Parse a markdown table row into cells
 */
function parseTableRow(line: string): string[] {
  const withoutPipes = line.charCodeAt(0) === 124 /* | */ ? line.slice(1) : line
  const withoutEndPipe = withoutPipes.charCodeAt(withoutPipes.length - 1) === 124 ? withoutPipes.slice(0, -1) : withoutPipes
  return withoutEndPipe.split('|').map(cell => cell.trim())
}

function isTableSeparator(trimmed: string): boolean {
  if (!trimmed.includes('|')) return false
  const content = trimmed.replace(/\|/g, '').trim()
  return RE_TABLE_SEP_CONTENT.test(content) && content.includes('-')
}

function isTableRow(trimmed: string): boolean {
  return trimmed.includes('|') && !isTableSeparator(trimmed)
}

function getDomainFromUrl(url: string): string {
  try { return new URL(url).hostname } catch { return url }
}

function mightContainMarkdownElements(content: string): boolean {
  if (!content) return false
  if (content.includes('```')) return true
  if (content.includes('|')) return true
  if (content.includes('![')) return true
  if (content.includes('http')) return true
  if ((content.includes(':\\') || content.includes('/')) && RE_FILE_PATH_LINE.test(content)) return true
  if (content.includes('- ') || content.includes('* ') || content.includes('+ ')) return true
  if (content.includes('.') && RE_DIGIT_DOT.test(content)) return true
  return false
}

// ============================================================================
// Single-pass markdown element parser — replaces 8 separate parsers.
// Splits content into lines ONCE, iterates ONCE, handles all element types.
// ============================================================================

type FenceKind = 'terminal' | 'mermaid' | 'code'

function parseMarkdownElementsSinglePass(
  content: string,
  cards: TypelessCard[],
  cardIndex: { value: number },
): string {
  if (!mightContainMarkdownElements(content)) {
    return content
  }

  const lines = content.split('\n')
  const result: string[] = []
  const len = lines.length

  // === Fenced block state (terminal / mermaid / code) ===
  let fenceKind: FenceKind | null = null
  let fenceLang = ''
  let fenceContent: string[] = []

  // === Table state ===
  let inTable = false
  let tableRows: string[][] = []
  let hasTableHeader = false

  // === List state ===
  let inList = false
  let listItems: ListItem[] = []
  let listOrdered = false
  let listChecklist = false
  let listIndent = 0
  let listParentStack: ListItem[] = []

  // === Image accumulator ===
  let pendingImages: GalleryImage[] = []

  const flushTable = () => {
    if (!inTable || tableRows.length === 0) return
    const firstRow = tableRows[0]
    const card: TypelessCardTable = {
      type: 'table', id: `md-table-${cardIndex.value++}`,
      headers: hasTableHeader && firstRow ? firstRow : [],
      rows: hasTableHeader ? tableRows.slice(1) : tableRows,
      striped: true,
    }
    cards.push(card)
    result.push(`[[TYPELESS_CARD:${card.id}]]`)
    tableRows = []; inTable = false; hasTableHeader = false
  }

  const flushList = () => {
    if (!inList || listItems.length === 0) return
    const card: TypelessCardList = {
      type: 'list', id: `md-list-${cardIndex.value++}`,
      items: listItems, ordered: listOrdered,
      variant: listChecklist ? 'checklist' : 'default',
    }
    cards.push(card)
    result.push(`[[TYPELESS_CARD:${card.id}]]`)
    listItems = []; inList = false; listOrdered = false; listChecklist = false
    listIndent = 0; listParentStack = []
  }

  const flushImages = () => {
    if (pendingImages.length === 0) return
    const card: TypelessCardGallery = {
      type: 'gallery', id: `md-gallery-${cardIndex.value++}`,
      images: pendingImages,
      layout: pendingImages.length > 1 ? 'horizontal' : 'grid',
      columns: Math.min(pendingImages.length, 4) as 2 | 3 | 4,
    }
    cards.push(card)
    result.push(`[[TYPELESS_CARD:${card.id}]]`)
    pendingImages = []
  }

  for (let i = 0; i < len; i++) {
    const line = lines[i]
    if (line === undefined) continue

    // ===== Inside a fenced block =====
    if (fenceKind !== null) {
      if (line.startsWith('```')) {
        const rest = line.slice(3)
        const blockContent = fenceContent.join('\n')
        if (fenceKind === 'terminal') {
          emitCard(makeTerminalCard(`md-terminal-${cardIndex.value++}`, blockContent, fenceLang), cards, result)
        } else if (fenceKind === 'mermaid') {
          emitCard({ type: 'mermaid', id: `md-mermaid-${cardIndex.value++}`, code: blockContent } as TypelessCardMermaid, cards, result)
        } else {
          emitCard(makeCodeCard(`md-code-${cardIndex.value++}`, blockContent, fenceLang), cards, result)
        }
        fenceKind = null; fenceLang = ''; fenceContent = []
        // Consecutive fence: ``````lang → close + open
        if (rest.startsWith('```') && !rest.startsWith('```typeless')) {
          const nextLang = rest.slice(3).trim()
          const nextLangLower = nextLang.toLowerCase()
          if (terminalMarkers.has(nextLangLower)) { fenceKind = 'terminal'; fenceLang = nextLangLower }
          else if (nextLangLower === 'mermaid') { fenceKind = 'mermaid'; fenceLang = nextLangLower }
          else { fenceKind = 'code'; fenceLang = languageAliases[nextLangLower] || nextLang }
          fenceContent = []
        }
      } else {
        fenceContent.push(line)
      }
      continue
    }

    // ===== Opening fence =====
    if (line.startsWith('```') && !line.startsWith('```typeless')) {
      flushTable(); flushList(); flushImages()
      const lang = line.slice(3).trim()
      const langLower = lang.toLowerCase()
      if (terminalMarkers.has(langLower)) { fenceKind = 'terminal'; fenceLang = langLower }
      else if (langLower === 'mermaid') { fenceKind = 'mermaid'; fenceLang = langLower }
      else { fenceKind = 'code'; fenceLang = languageAliases[langLower] || lang }
      fenceContent = []
      continue
    }

    // ===== Table / List / Image / URL / File =====
    const trimmed = line.trim()

    if (isTableSeparator(trimmed)) {
      flushList(); flushImages()
      if (inTable && tableRows.length === 1) hasTableHeader = true
      continue
    }

    if (isTableRow(trimmed)) {
      flushList(); flushImages()
      if (!inTable) { inTable = true; tableRows = [] }
      tableRows.push(parseTableRow(trimmed))
      continue
    }

    if (inTable) flushTable()

    // List items (cheap first-char gate to avoid regex work on plain lines)
    let checkboxMatch: RegExpExecArray | null = null
    let ulMatch: RegExpExecArray | null = null
    let olMatch: RegExpExecArray | null = null

    if (trimmed !== '') {
      const firstCode = trimmed.charCodeAt(0)
      const isPossibleListStart = firstCode === 45 || firstCode === 42 || firstCode === 43 || (firstCode >= 48 && firstCode <= 57)
      if (isPossibleListStart) {
        checkboxMatch = RE_CHECKBOX_ITEM.exec(line)
        ulMatch = !checkboxMatch ? RE_UL_ITEM.exec(line) : null
        olMatch = RE_OL_ITEM.exec(line)
      }
    }

    if (checkboxMatch || ulMatch || olMatch) {
      flushImages()
      let indent: number, itemContent: string, itemIsOrdered = false, itemIsCheckbox = false, itemChecked = false
      if (checkboxMatch) {
        indent = checkboxMatch[1]?.length ?? 0; itemChecked = checkboxMatch[2]?.toLowerCase() === 'x'
        itemContent = checkboxMatch[3] ?? ''; itemIsCheckbox = true
      } else if (ulMatch) {
        indent = ulMatch[1]?.length ?? 0; itemContent = ulMatch[2] ?? ''
      } else {
        indent = olMatch![1]?.length ?? 0; itemContent = olMatch![2] ?? ''; itemIsOrdered = true
      }
      if (!inList) { inList = true; listOrdered = itemIsOrdered; listChecklist = itemIsCheckbox; listIndent = indent; listParentStack = [] }
      const newItem: ListItem = { content: itemContent, checked: itemIsCheckbox ? itemChecked : undefined }
      if (indent > listIndent) {
        const parent = listParentStack[listParentStack.length - 1]
        if (parent) { if (!parent.subItems) parent.subItems = []; parent.subItems.push(newItem) }
        else listItems.push(newItem)
        listParentStack.push(newItem)
      } else if (indent < listIndent) {
        while (listParentStack.length > 0 && indent <= listIndent) { listParentStack.pop(); listIndent -= 2 }
        const parent = listParentStack[listParentStack.length - 1]
        if (parent) { if (!parent.subItems) parent.subItems = []; parent.subItems.push(newItem) }
        else listItems.push(newItem)
        listParentStack.push(newItem)
      } else {
        if (listParentStack.length > 1) {
          listParentStack.pop()
          const parent = listParentStack[listParentStack.length - 1]
          if (parent) { if (!parent.subItems) parent.subItems = []; parent.subItems.push(newItem) }
          listParentStack.push(newItem)
        } else { listParentStack = [newItem]; listItems.push(newItem) }
      }
      listIndent = indent
      continue
    }

    if (trimmed === '' && inList) {
      const nextLine = lines[i + 1]
      const nextTrimmed = nextLine?.trim() ?? ''
      if (nextTrimmed !== '') {
        const firstCode = nextTrimmed.charCodeAt(0)
        const nextLooksLikeList = (
          firstCode === 45
          || firstCode === 42
          || firstCode === 43
          || (firstCode >= 48 && firstCode <= 57)
        ) && (RE_UL_ITEM.test(nextLine!) || RE_OL_ITEM.test(nextLine!))
        if (nextLooksLikeList) {
          result.push(line)
          continue
        }
      }
      flushList()
    }
    if (inList) flushList()

    // Image-only lines
    if (trimmed.includes('![')) {
      RE_IMAGE_INLINE.lastIndex = 0
      let match; let lastIdx = 0; let nonImageContent = ''
      const lineImages: GalleryImage[] = []
      while ((match = RE_IMAGE_INLINE.exec(trimmed)) !== null) {
        nonImageContent += trimmed.slice(lastIdx, match.index).trim()
        lastIdx = match.index + match[0].length
        const src = match[2]
        const isWindowsPath = !!src && RE_WINDOWS_ABS_PATH.test(src)
        if (src && (src.startsWith('data:image/') || src.startsWith('http') || src.startsWith('/') || isWindowsPath)) {
          lineImages.push({ src, alt: match[1] || undefined, thumbnail: (isWindowsPath || src.startsWith('/')) ? `${src}?thumbnail=true` : undefined })
        }
      }
      RE_IMAGE_INLINE.lastIndex = 0
      nonImageContent += trimmed.slice(lastIdx).trim()
      if (lineImages.length > 0 && nonImageContent === '') { pendingImages.push(...lineImages); continue }
    }
    flushImages()

    // Standalone URL
    const urlMatch = RE_STANDALONE_URL.exec(trimmed)
    if (urlMatch && urlMatch[1]) {
      const card: TypelessCardLink = { type: 'link', id: `md-link-${cardIndex.value++}`, url: urlMatch[1], title: getDomainFromUrl(urlMatch[1]) }
      cards.push(card); result.push(`[[TYPELESS_CARD:${card.id}]]`); continue
    }

    // File path
    const pathMatch = RE_FILE_PATH.exec(trimmed)
    if (pathMatch && pathMatch[1]) {
      const filePath = pathMatch[1]
      const filename = filePath.split(/[/\\]/).pop() || filePath
      const ext = filename.split('.').pop()?.toLowerCase() || ''
      if (!imageExtensions.includes(ext)) {
        const card: TypelessCardFile = { type: 'file', id: `md-file-${cardIndex.value++}`, filename, downloadUrl: filePath, previewUrl: documentExtensions.includes(ext) ? filePath : undefined }
        cards.push(card); result.push(`[[TYPELESS_CARD:${card.id}]]`); continue
      }
    }

    result.push(line)
  }

  flushTable(); flushList(); flushImages()

  // Handle unclosed fenced block (streaming)
  if (fenceKind !== null && fenceContent.length > 0) {
    const blockContent = fenceContent.join('\n')
    if (fenceKind === 'terminal') emitCard(makeTerminalCard(`md-terminal-streaming-${cardIndex.value++}`, blockContent, fenceLang, true), cards, result)
    else if (fenceKind === 'mermaid') emitCard({ type: 'mermaid', id: `md-mermaid-streaming-${cardIndex.value++}`, code: blockContent, _streaming: true } as TypelessCardMermaid, cards, result)
    else emitCard(makeCodeCard(`md-code-streaming-${cardIndex.value++}`, blockContent, fenceLang, true), cards, result)
  }

  return result.join('\n')
}
