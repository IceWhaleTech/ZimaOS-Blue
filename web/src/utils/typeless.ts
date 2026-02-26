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

    // Return deep clone to prevent mutation
    return {
      text: entry.result.text,
      cards: entry.result.cards.map(c => ({ ...c })),
    }
  }

  set(content: string, result: ParsedContent): void {
    const key = this.generateKey(content)

    // Evict oldest if at capacity
    if (this.cache.size >= this.maxSize) {
      const firstKey = this.cache.keys().next().value
      if (firstKey) this.cache.delete(firstKey)
    }

    // Store deep clone
    this.cache.set(key, {
      result: {
        text: result.text,
        cards: result.cards.map(c => ({ ...c })),
      },
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
  lastCardIndex: number
  hasStreaming: boolean
}

const incrementalStates = new Map<string, IncrementalParseState>()

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
    incrementalStates.set(cacheKey, {
      lastContent: content,
      lastResult: result,
      lastCardIndex: result.cards.length,
      hasStreaming: checkStreaming(result.cards),
    })
    return result
  }

  // Content is an extension of previous content
  const newContent = content.slice(state.lastContent.length)

  // If new content is small, just return cached result (debounce)
  // But if we have a streaming card, always re-parse to update it
  if (newContent.length < 10 && !state.hasStreaming) {
    return state.lastResult
  }

  // Check if new content might contain new cards or update streaming cards
  const mightHaveNewCards =
    newContent.includes('```') ||
    newContent.includes('|') ||
    newContent.includes('- ') ||
    newContent.includes('* ') ||
    newContent.includes('1. ') ||
    newContent.includes('![') ||
    newContent.includes('http') ||
    state.hasStreaming // Always re-parse if we have a streaming card

  if (!mightHaveNewCards) {
    // Just update text, no new cards
    const updatedResult: ParsedContent = {
      text: state.lastResult.text + newContent,
      cards: state.lastResult.cards,
    }
    incrementalStates.set(cacheKey, {
      lastContent: content,
      lastResult: updatedResult,
      lastCardIndex: state.lastCardIndex,
      hasStreaming: state.hasStreaming,
    })
    return updatedResult
  }

  // Need to re-parse (new cards might be present or streaming card updated)
  const result = parseTypelessContentInternal(content, 0, true) // isStreaming = true
  incrementalStates.set(cacheKey, {
    lastContent: content,
    lastResult: result,
    lastCardIndex: result.cards.length,
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
  let text = content

  // Remove <system_placeholder /> and similar self-closing system tags
  text = text.replace(/<system_placeholder\s*\/>/g, '')
  text = text.replace(/<system-reminder>[\s\S]*?<\/system-reminder>/g, '')

  // Replace [SILENT_REPLY] with a subtle icon
  text = text.replace(/\[SILENT_REPLY\]/g, '💤')

  // Parse <thinking>...</thinking> tags and convert to collapsible accordion
  const thinkingRegex = /<thinking>([\s\S]*?)<\/thinking>/g
  let match
  const replacements: { start: number; end: number; placeholder: string }[] = []

  while ((match = thinkingRegex.exec(text)) !== null) {
    const thinkingContent = match[1]?.trim()
    if (thinkingContent) {
      const card: TypelessCardAccordion = {
        type: 'accordion',
        id: `thinking-${cardIndex.value++}`,
        title: '💭 ' + t('thinking.title', 'Thinking Process'),
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

  // Handle incomplete/streaming <thinking> tags (no closing tag yet)
  const incompleteThinkingRegex = /<thinking>([\s\S]*)$/
  const incompleteMatch = incompleteThinkingRegex.exec(text)
  if (incompleteMatch && incompleteMatch[1]) {
    const thinkingContent = incompleteMatch[1].trim()
    if (thinkingContent) {
      const card: TypelessCardAccordion = {
        type: 'accordion',
        id: `thinking-streaming-${cardIndex.value++}`,
        title: '💭 ' + t('thinking.inProgress', 'Thinking...'),
        items: [{
          title: t('thinking.expand', 'Expand'),
          content: thinkingContent,
          defaultOpen: true, // Show open while streaming
        }],
        allowMultiple: false,
        _streaming: true,
      }
      cards.push(card)
      text = text.slice(0, incompleteMatch.index) + `[[TYPELESS_CARD:${card.id}]]`
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

  // Find all occurrences of ```typeless
  const startMatches: number[] = []
  let searchStart = 0
  while (true) {
    const idx = content.indexOf(markerStart, searchStart)
    if (idx === -1) break
    startMatches.push(idx)
    searchStart = idx + markerStart.length
  }

  // For each start marker, find the LAST ``` in the remaining content
  const replacements: { start: number; end: number; placeholder: string }[] = []
  for (const startIdx of startMatches) {
    // Find the content start (after ```typeless)
    const contentStart = startIdx + markerStart.length
    // Find ALL ``` after this point (not just the first one)
    const remainingContent = content.slice(contentStart)
    let lastFenceIdx = -1
    let fenceSearchStart = 0
    while (true) {
      const fenceIdx = remainingContent.indexOf(markerEnd, fenceSearchStart)
      if (fenceIdx === -1) break
      lastFenceIdx = fenceIdx
      fenceSearchStart = fenceIdx + markerEnd.length
    }

    if (lastFenceIdx === -1) continue // No closing fence found

    // The JSON is from contentStart to contentStart + lastFenceIdx
    const jsonStr = remainingContent.slice(0, lastFenceIdx).trim()
    const endIdx = contentStart + lastFenceIdx + markerEnd.length

    if (!jsonStr) continue

    try {
      // Try normal parse first, then try to fix incomplete/truncated JSON
      let card = tryParseIncompleteJSON(jsonStr) as TypelessCard | null

      // Validate card has required type field
      if (card && typeof card.type === 'string' && isValidCardType(card.type)) {
        // Assign ID if not present
        // Mark for replacement with placeholder
        replacements.push({
          start: startIdx,
          end: endIdx,
          placeholder: `[[TYPELESS_CARD:${card.id}]]`,
        })
      }
    } catch {
      // Invalid JSON, leave as-is
      console.warn('Failed to parse typeless card:', jsonStr.slice(0, 100))
    }
  }

  // During streaming, also try to parse incomplete typeless blocks
  // Look for blocks that start with marker but don't have closing marker yet
  if (isStreaming) {
    const incompleteRegex = new RegExp(
      `${escapeRegex(TYPELESS_MARKER_START)}\\s*([\\s\\S]*)$`
    )
    const incompleteMatch = incompleteRegex.exec(content)
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
            end: content.length,
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
const RE_SEPARATOR_LINE = /^[\s|:-]+$/
const RE_UNORDERED_LIST = /^(\s*)[-*+]\s+/
const RE_ORDERED_LIST = /^(\s*)\d+\.\s+/

/**
 * Check if content contains any typeless cards or markdown elements that will be converted.
 * Optimized with fast string checks before regex, and pre-compiled regexes.
 */
export function hasTypelessCards(content: string): boolean {
  // Fast path: check for explicit typeless blocks (cheapest check)
  if (content.includes(TYPELESS_MARKER_START)) {
    return true
  }

  // Fast path: code fences (covers terminal, mermaid, and regular code blocks)
  // Use includes('```') as a cheap gate before regex
  if (content.includes('```') && RE_CODE_FENCE.test(content)) {
    return true
  }

  // Fast path: markdown images — gate with '!['
  if (content.includes('![') && RE_IMAGE_LINE.test(content)) {
    return true
  }

  // Fast path: standalone URLs — gate with 'http'
  if (content.includes('http') && RE_URL_LINE.test(content)) {
    return true
  }

  // Fast path: file paths — gate with common path separators
  if ((content.includes(':\\') || content.includes('/')) && RE_FILE_PATH_LINE.test(content)) {
    return true
  }

  // Tables and lists require line-by-line scan — gate with cheap checks
  const hasPipe = content.includes('|')
  const hasDash = content.includes('- ') || content.includes('* ') || content.includes('+ ')
  const hasDigitDot = /\d+\.\s/.test(content)

  if (!hasPipe && !hasDash && !hasDigitDot) {
    return false
  }

  // Single line split for both table and list detection
  const lines = content.split('\n')

  // Check for markdown tables (at least 2 rows with pipes)
  if (hasPipe) {
    let tableRowCount = 0
    for (const line of lines) {
      if (line.includes('|') && !RE_SEPARATOR_LINE.test(line)) {
        tableRowCount++
        if (tableRowCount >= 2) return true
      } else if (tableRowCount > 0 && line.trim() !== '' && !RE_SEPARATOR_LINE.test(line)) {
        tableRowCount = 0
      }
    }
  }

  // Check for markdown lists (at least 2 items)
  if (hasDash || hasDigitDot) {
    let listItemCount = 0
    for (const line of lines) {
      if (RE_UNORDERED_LIST.test(line) || RE_ORDERED_LIST.test(line)) {
        listItemCount++
        if (listItemCount >= 2) return true
      } else if (line.trim() !== '') {
        listItemCount = 0
      }
    }
  }

  return false
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
  return /\[\[TYPELESS_CARD:([^\]]+)\]\]/g
}

/**
 * Split parsed text into segments (text and card placeholders).
 */
export function splitIntoSegments(
  text: string,
  cards: TypelessCard[]
): Array<{ type: 'text' | 'card'; content: string | TypelessCard }> {
  const segments: Array<{ type: 'text' | 'card'; content: string | TypelessCard }> = []
  const cardMap = new Map(cards.map((c) => [c.id, c]))

  const pattern = getCardPlaceholderPattern()
  let lastIndex = 0
  let match

  while ((match = pattern.exec(text)) !== null) {
    // Add text before the placeholder
    if (match.index > lastIndex) {
      const textContent = text.slice(lastIndex, match.index).trim()
      if (textContent) {
        segments.push({ type: 'text', content: textContent })
      }
    }

    // Add the card
    const cardId = match[1]
    const card = cardMap.get(cardId)
    if (card) {
      segments.push({ type: 'card', content: card })
    }

    lastIndex = match.index + match[0].length
  }

  // Add remaining text
  if (lastIndex < text.length) {
    const textContent = text.slice(lastIndex).trim()
    if (textContent) {
      segments.push({ type: 'text', content: textContent })
    }
  }

  // Merge consecutive ui-review-progress cards into a single aggregated card
  return mergeConsecutiveProgressCards(segments)
}

/**
 * Merge consecutive ui-review-progress card segments into a single card
 * with a `steps` array, so the component renders them as one consolidated view.
 * Also merges consecutive analyze-progress cards the same way.
 */
function mergeConsecutiveProgressCards(
  segments: Array<{ type: 'text' | 'card'; content: string | TypelessCard }>
): Array<{ type: 'text' | 'card'; content: string | TypelessCard }> {
  const result: typeof segments = []
  let pendingSteps: TypelessCard[] = []
  let pendingType: 'ui-review-progress' | 'analyze-progress' | null = null

  const flushPending = () => {
    if (pendingSteps.length === 0 || !pendingType) return
    const first = pendingSteps[0]!
    const merged: TypelessCard = {
      type: pendingType,
      id: (first as Record<string, unknown>).id as string || `${pendingType}-merged`,
      steps: pendingSteps.map(s => ({
        step: (s as Record<string, unknown>).step,
        name: (s as Record<string, unknown>).name,
        status: (s as Record<string, unknown>).status,
        url: (s as Record<string, unknown>).url,
        score: (s as Record<string, unknown>).score,
      })),
      _streaming: pendingSteps.some(s => s._streaming),
    } as TypelessCard
    result.push({ type: 'card', content: merged })
    pendingSteps = []
    pendingType = null
  }

  for (const seg of segments) {
    const cardType = seg.type === 'card' ? (seg.content as TypelessCard).type : null
    if (cardType === 'ui-review-progress' || cardType === 'analyze-progress') {
      if (pendingType && pendingType !== cardType) {
        flushPending()
      }
      pendingType = cardType as 'ui-review-progress' | 'analyze-progress'
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

    // List items
    const checkboxMatch = RE_CHECKBOX_ITEM.exec(line)
    const ulMatch = !checkboxMatch ? RE_UL_ITEM.exec(line) : null
    const olMatch = RE_OL_ITEM.exec(line)

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
      if (nextLine && (RE_UL_ITEM.test(nextLine) || RE_OL_ITEM.test(nextLine))) { result.push(line); continue }
      flushList()
    }
    if (inList && !RE_UL_ITEM.test(line) && !RE_OL_ITEM.test(line)) flushList()

    // Image-only lines
    if (trimmed.includes('![')) {
      RE_IMAGE_INLINE.lastIndex = 0
      let match; let lastIdx = 0; let nonImageContent = ''
      const lineImages: GalleryImage[] = []
      while ((match = RE_IMAGE_INLINE.exec(trimmed)) !== null) {
        nonImageContent += trimmed.slice(lastIdx, match.index).trim()
        lastIdx = match.index + match[0].length
        const src = match[2]
        if (src && (src.startsWith('data:image/') || src.startsWith('http') || src.startsWith('/') || /^[a-zA-Z]:\\/.test(src))) {
          lineImages.push({ src, alt: match[1] || undefined, thumbnail: (/^[a-zA-Z]:\\/.test(src) || src.startsWith('/')) ? `${src}?thumbnail=true` : undefined })
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

