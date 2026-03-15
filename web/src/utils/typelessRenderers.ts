/**
 * Functional renderers for simple typeless cards.
 * These render directly to HTML strings, bypassing Vue component overhead.
 * Complex cards (Chart, Map, Gallery with lightbox) still use Vue components.
 */

import type {
  TypelessCard,
  TypelessCardTable,
  TypelessCardCode,
  TypelessCardList,
  TypelessCardInfo,
  TypelessCardQuote,
  TypelessCardAlert,
  TypelessCardTerminal,
  ListItem,
} from '@/types/typeless'
import { i18n } from '@/i18n'
import { parseInline, highlightCode } from './markdown'

const TYPELESS_INLINE_OPTIONS = { allowUnderscoreEmphasis: false } as const

function parseTypelessInline(text: string): string {
  return parseInline(text, TYPELESS_INLINE_OPTIONS)
}

// ============================================================================
// Render Cache - LRU cache for rendered card HTML
// ============================================================================

interface CacheEntry {
  html: string
  timestamp: number
}

class RenderCache {
  private cache = new Map<string, CacheEntry>()
  private maxSize: number
  private maxAge: number // ms

  constructor(maxSize = 500, maxAgeMs = 5 * 60 * 1000) {
    this.maxSize = maxSize
    this.maxAge = maxAgeMs
  }

  private generateKey(card: TypelessCard): string {
    // Always hash the full card content to avoid collisions between cards
    // with the same ID from different messages
    return `${card.type}:${this.hashCard(card)}`
  }

  private hashCard(card: TypelessCard): string {
    // Simple hash based on JSON stringification
    const str = JSON.stringify(card)
    let hash = 0
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i)
      hash = (hash << 5) - hash + char
      hash = hash & hash // Convert to 32bit integer
    }
    return hash.toString(36)
  }

  get(card: TypelessCard): string | null {
    const key = this.generateKey(card)
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

    return entry.html
  }

  set(card: TypelessCard, html: string): void {
    const key = this.generateKey(card)

    // Evict oldest if at capacity
    if (this.cache.size >= this.maxSize) {
      const firstKey = this.cache.keys().next().value
      if (firstKey) this.cache.delete(firstKey)
    }

    this.cache.set(key, { html, timestamp: Date.now() })
  }

  clear(): void {
    this.cache.clear()
  }

  get size(): number {
    return this.cache.size
  }
}

// Global render cache instance
const renderCache = new RenderCache()

// Export for testing/debugging
export { renderCache }

// Global auto-incrementing counter for DOM element IDs.
// Card IDs like "md-code-0" are per-message, NOT globally unique across the DOM.
// Using card.id as a DOM id causes document.getElementById() to always find the
// first matching element, breaking copy buttons on the 2nd/3rd/... card.
let domIdCounter = 0
const DEFAULT_CODE_COLLAPSE_LINES = 24
const CODE_LINE_HEIGHT_PX = 24

// Card types that can be rendered functionally (simple, no interactivity beyond copy)
export const FUNCTIONAL_CARD_TYPES = new Set([
  'table',
  'code',
  'list',
  'info',
  'quote',
  'alert',
  'terminal',
])

// Card types that require Vue components (complex interactivity, async loading, etc.)
export const COMPONENT_CARD_TYPES = new Set([
  'link', // Needs async link preview fetch
  'file', // Has download/preview buttons
  'gallery', // Has lightbox, scroll buttons
  'chart', // Uses chart library
  'map', // Uses map library
  'action', // Has interactive buttons
  'choice', // Has selection state
  'progress', // May have animations
  'result', // Has action buttons
  'detection', // Complex UI
  'metric', // May have animations
  'comparison', // Complex layout
  'steps', // Interactive steps
  'weather', // Complex UI
  'profile', // Complex UI
  'countdown', // Has timer
  'rating', // Interactive
  'accordion', // Has expand/collapse state
  'audio', // Has audio player
  'collapsible-code', // Has expand/collapse state
  'diff', // Complex highlighting
  'mermaid', // Mermaid diagram rendering
])

/**
 * Check if a card can be rendered functionally
 */
export function canRenderFunctionally(card: TypelessCard): boolean {
  return FUNCTIONAL_CARD_TYPES.has(card.type)
}

/**
 * Render a card to HTML string (for simple cards only)
 * Uses LRU cache to avoid re-rendering identical cards
 */
export function renderCardToHtml(card: TypelessCard): string {
  // Code and terminal cards use globally unique DOM IDs for copy buttons,
  // so they must NOT be cached (cached HTML would have stale IDs).
  const useCache = card.type !== 'code' && card.type !== 'terminal'

  // Check cache first
  if (useCache) {
    const cached = renderCache.get(card)
    if (cached) return cached
  }

  // Render and cache
  let html: string
  switch (card.type) {
    case 'table':
      html = renderTable(card as TypelessCardTable)
      break
    case 'code':
      html = renderCode(card as TypelessCardCode)
      break
    case 'list':
      html = renderList(card as TypelessCardList)
      break
    case 'info':
      html = renderInfo(card as TypelessCardInfo)
      break
    case 'quote':
      html = renderQuote(card as TypelessCardQuote)
      break
    case 'alert':
      html = renderAlert(card as TypelessCardAlert)
      break
    case 'terminal':
      html = renderTerminal(card as TypelessCardTerminal)
      break
    default:
      html = `<div class="text-red-500">Unknown card type: ${card.type}</div>`
  }

  if (useCache) {
    renderCache.set(card, html)
  }
  return html
}

// Escape HTML to prevent XSS
function escapeHtml(text: string): string {
  const map: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#039;',
  }
  return text.replace(/[&<>"']/g, (char) => map[char] || char)
}

function t(key: string, fallback: string, named?: Record<string, string | number>): string {
  const result = named ? i18n.global.t(key, named) : i18n.global.t(key)
  return result === key ? fallback : String(result)
}

/**
 * Render table card
 */
function renderTable(card: TypelessCardTable): string {
  const titleHtml = card.title
    ? `<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50">
        <h4 class="font-medium text-gray-900 dark:text-white">${escapeHtml(card.title)}</h4>
      </div>`
    : ''

  const headersHtml =
    card.headers.length > 0
      ? `<thead>
        <tr class="bg-gray-50 dark:bg-gray-700/50">
          ${card.headers.map((h) => `<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">${parseTypelessInline(String(h))}</th>`).join('')}
        </tr>
      </thead>`
      : ''

  const rowsHtml = card.rows
    .map((row, rowIndex) => {
      const stripedClass =
        card.striped && rowIndex % 2 === 1 ? 'bg-gray-50 dark:bg-gray-700/30' : ''
      const cells = row
        .map(
          (cell) =>
            `<td class="px-4 py-3 text-gray-700 dark:text-gray-300${card.compact ? ' py-2' : ''}">${parseTypelessInline(String(cell))}</td>`
        )
        .join('')
      return `<tr class="${stripedClass} hover:bg-gray-50 dark:hover:bg-gray-700/50">${cells}</tr>`
    })
    .join('')

  const footerHtml = card.footer
    ? `<div class="px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50">
        <p class="text-xs text-gray-500 dark:text-gray-400">${escapeHtml(card.footer)}</p>
      </div>`
    : ''

  return `<div class="table-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
    ${titleHtml}
    <div class="overflow-x-auto">
      <table class="w-full${card.compact ? ' text-sm' : ''}">
        ${headersHtml}
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          ${rowsHtml}
        </tbody>
      </table>
    </div>
    ${footerHtml}
  </div>`
}

/**
 * Render code card
 */
function renderCode(card: TypelessCardCode): string {
  const languageNames: Record<string, string> = {
    js: 'JavaScript',
    javascript: 'JavaScript',
    ts: 'TypeScript',
    typescript: 'TypeScript',
    py: 'Python',
    python: 'Python',
    go: 'Go',
    rust: 'Rust',
    java: 'Java',
    cpp: 'C++',
    c: 'C',
    html: 'HTML',
    css: 'CSS',
    json: 'JSON',
    yaml: 'YAML',
    sql: 'SQL',
    bash: 'Bash',
    shell: 'Shell',
    md: 'Markdown',
    markdown: 'Markdown',
  }

  const langDisplay = card.language
    ? languageNames[card.language.toLowerCase()] || card.language
    : 'Text'

  // Normalize language for highlighting
  const langForHighlight = card.language?.toLowerCase() || ''

  // Disable line numbers for plain text (no language specified)
  const isPlainText = !card.language
  const showLineNumbers = !isPlainText && card.showLineNumbers !== false
  const highlightLines = new Set(card.highlightLines || [])

  // Apply syntax highlighting to the entire code block
  const highlightedCode = card.language
    ? highlightCode(card.code, langForHighlight)
    : escapeHtml(card.code)
  const highlightedLines = highlightedCode.split('\n')
  const configuredMaxCollapsedLines = (card as TypelessCardCode & { maxCollapsedLines?: number })
    .maxCollapsedLines
  const maxCollapsedLines = Math.max(1, configuredMaxCollapsedLines || DEFAULT_CODE_COLLAPSE_LINES)
  const shouldCollapse = highlightedLines.length > maxCollapsedLines
  const hiddenLinesCount = shouldCollapse ? highlightedLines.length - maxCollapsedLines : 0
  const collapsedMaxHeightPx = maxCollapsedLines * CODE_LINE_HEIGHT_PX

  const linesHtml = highlightedLines
    .map((line, index) => {
      const lineNum = index + 1
      const highlighted = highlightLines.has(lineNum) ? ' bg-yellow-500/20' : ''
      const lineNumHtml = showLineNumbers
        ? `<span class="inline-block w-8 text-right mr-4 text-gray-500 select-none">${lineNum}</span>`
        : ''
      // Add newline at the end for proper copying
      const lineContent = index < highlightedLines.length - 1 ? `${line}\n` : line
      return `<span class="block${highlighted}">${lineNumHtml}${lineContent}</span>`
    })
    .join('')

  const titleOrFilename = card.filename || card.title || ''
  const titleHtml = titleOrFilename
    ? `<span class="text-sm text-gray-600 dark:text-gray-400">${escapeHtml(titleOrFilename)}</span>`
    : ''
  const langBadge = langDisplay
    ? `<span class="px-2 py-0.5 text-xs rounded bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300">${escapeHtml(langDisplay)}</span>`
    : ''

  // Generate globally unique DOM IDs for copy/collapse functionality
  const baseDomId = ++domIdCounter
  const codeId = `code-${baseDomId}`
  const codeContainerId = `code-container-${baseDomId}`
  const codeFadeId = `code-fade-${baseDomId}`
  const codeToggleId = `code-toggle-${baseDomId}`
  const expandText = t('execCard.expand', 'Show more')
  const collapseText = t('execCard.collapse', 'Show less')
  const linesText = t('execCard.lines', 'lines', { count: hiddenLinesCount })
  const expandLabel = `${expandText} ${hiddenLinesCount} ${linesText}`
  const collapseLabel = collapseText

  // Data for fullscreen — base64-encode to avoid HTML attribute escaping issues
  // (JSON escape sequences like \n get mangled by the browser's HTML parser)
  const fullscreenDataB64 = btoa(
    unescape(
      encodeURIComponent(
        JSON.stringify({
          title: titleOrFilename || langDisplay,
          language: card.language,
          content: card.code,
        })
      )
    )
  )

  return `<div class="code-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700" ondblclick="window.__typelessOpenFullscreen && window.__typelessOpenFullscreen('code', '${fullscreenDataB64}', true)">
    <div class="flex items-center justify-between px-3 py-1.5 bg-gray-50 dark:bg-gray-700 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center gap-2">
        <div class="flex gap-1">
          <div class="w-2.5 h-2.5 rounded-full bg-red-500"></div>
          <div class="w-2.5 h-2.5 rounded-full bg-yellow-500"></div>
          <div class="w-2.5 h-2.5 rounded-full bg-green-500"></div>
        </div>
        ${titleHtml}
        ${langBadge}
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-xs text-gray-400 dark:text-gray-500 hidden sm:inline" title="${t('media.fullscreen', 'Full Screen')}">⤢</span>
        <button
          class="typeless-copy-btn flex items-center p-1 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors rounded hover:bg-gray-200 dark:hover:bg-gray-700"
          data-code-id="${codeId}"
          onclick="event.stopPropagation(); window.__typelessCopyCode && window.__typelessCopyCode('${codeId}')"
          title="${t('common.copy', 'Copy')}"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>
      </div>
    </div>
    <div
      id="${codeContainerId}"
      class="overflow-x-auto cursor-pointer relative"
      title="${t('media.fullscreen', 'Full Screen')}"
      ${shouldCollapse ? `data-collapsed="true" style="max-height: ${collapsedMaxHeightPx}px; overflow-y: hidden;"` : ''}
    >
      <pre class="p-4 text-sm leading-relaxed" style="margin: 0; font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;"><code id="${codeId}" class="text-gray-800 dark:text-gray-100">${linesHtml}</code></pre>
      ${shouldCollapse ? `<div id="${codeFadeId}" class="absolute bottom-0 left-0 right-0 h-16 bg-gradient-to-t from-white dark:from-gray-700 to-transparent pointer-events-none"></div>` : ''}
    </div>
    ${
      shouldCollapse
        ? `<div class="border-t border-gray-200 dark:border-gray-700">
          <button
            id="${codeToggleId}"
            class="w-full px-4 py-2 text-sm transition-colors flex items-center justify-center gap-2 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700"
            data-code-container-id="${codeContainerId}"
            data-code-fade-id="${codeFadeId}"
            data-collapsed-max-height="${collapsedMaxHeightPx}"
            data-expand-label="${escapeHtml(expandLabel)}"
            data-collapse-label="${escapeHtml(collapseLabel)}"
            onclick="event.stopPropagation(); window.__typelessToggleCodeCollapse && window.__typelessToggleCodeCollapse('${codeToggleId}')"
          >${escapeHtml(expandLabel)}</button>
        </div>`
        : ''
    }
  </div>`
}

/**
 * Render list card
 */
function renderList(card: TypelessCardList): string {
  const titleHtml = card.title
    ? `<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h4 class="font-medium text-gray-900 dark:text-white">${escapeHtml(card.title)}</h4>
      </div>`
    : ''

  function renderListItem(item: ListItem, isSubItem = false): string {
    const textClass = isSubItem
      ? 'text-sm text-gray-600 dark:text-gray-400'
      : 'text-gray-700 dark:text-gray-300'
    const bulletClass = isSubItem
      ? 'w-1 h-1 mt-2 rounded-full bg-gray-300'
      : 'w-1.5 h-1.5 mt-2 rounded-full bg-gray-400'

    const iconHtml = item.icon
      ? `<span class="flex-shrink-0">${escapeHtml(item.icon)}</span>`
      : `<span class="flex-shrink-0 ${bulletClass}"></span>`

    const subItemsHtml = item.subItems?.length
      ? `<ul class="mt-2 ml-4 space-y-1">
          ${item.subItems.map((sub) => renderListItem(sub, true)).join('')}
        </ul>`
      : ''

    return `<li class="flex items-start gap-2 ${textClass}">
      ${iconHtml}
      <div class="flex-1">
        <span>${parseTypelessInline(item.content)}</span>
        ${subItemsHtml}
      </div>
    </li>`
  }

  // Default variant
  if (!card.variant || card.variant === 'default') {
    const tag = card.ordered ? 'ol' : 'ul'
    const listClass = card.ordered ? 'list-decimal list-inside' : ''
    const itemsHtml = card.items.map((item) => renderListItem(item)).join('')

    return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
      ${titleHtml}
      <${tag} class="p-4 space-y-2 ${listClass}">
        ${itemsHtml}
      </${tag}>
    </div>`
  }

  // Checklist variant
  if (card.variant === 'checklist') {
    // Count completed items for progress display
    const totalItems = card.items.length
    const completedItems = card.items.filter((item) => item.checked).length
    const progressPercent = totalItems > 0 ? Math.round((completedItems / totalItems) * 100) : 0

    // Progress header
    const progressHtml = `<div class="flex items-center justify-between px-4 py-2 bg-gray-50 dark:bg-gray-700/50 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center gap-2">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-gray-500 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
        </svg>
        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">${completedItems}/${totalItems} completed</span>
      </div>
      <div class="flex items-center gap-2">
        <div class="w-24 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
          <div class="h-full ${completedItems === totalItems ? 'bg-green-500' : 'bg-gray-700 dark:bg-gray-700'} rounded-full transition-all duration-300" style="width: ${progressPercent}%"></div>
        </div>
        <span class="text-xs text-gray-500 dark:text-gray-400">${progressPercent}%</span>
      </div>
    </div>`

    const itemsHtml = card.items
      .map((item, index) => {
        const checked = item.checked || false
        const checkboxBg = checked
          ? 'bg-green-500 border-green-500'
          : 'bg-white dark:bg-gray-700 border-gray-300 dark:border-gray-600'
        const checkIcon = checked
          ? `<svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
          </svg>`
          : ''
        const textClass = checked
          ? 'line-through text-gray-400 dark:text-gray-500'
          : 'text-gray-700 dark:text-gray-300'
        const rowBg = checked
          ? 'bg-green-50/50 dark:bg-green-900/10'
          : 'hover:bg-gray-50 dark:hover:bg-gray-700/50'

        return `<li class="flex items-center gap-3 px-4 py-2.5 ${rowBg} transition-colors" data-item-index="${index}">
        <div class="flex-shrink-0 w-5 h-5 rounded border-2 flex items-center justify-center ${checkboxBg} transition-colors">
          ${checkIcon}
        </div>
        <span class="flex-1 text-sm ${textClass} transition-colors">${parseTypelessInline(item.content)}</span>
        ${checked ? '<span class="text-xs text-green-500 dark:text-green-400">✓</span>' : ''}
      </li>`
      })
      .join('')

    return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
      ${titleHtml}
      ${progressHtml}
      <ul class="divide-y divide-gray-100 dark:divide-gray-700/50">
        ${itemsHtml}
      </ul>
    </div>`
  }

  // Timeline variant
  if (card.variant === 'timeline') {
    const itemsHtml = card.items
      .map((item, index) => {
        const dotClass =
          index === 0
            ? 'border-gray-900 dark:border-white bg-gray-700 dark:bg-gray-700'
            : 'border-gray-300 dark:border-gray-600'
        const timestampHtml = item.timestamp
          ? `<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">${escapeHtml(item.timestamp)}</p>`
          : ''

        return `<div class="relative flex items-start gap-4 pb-4 last:pb-0">
        <div class="absolute left-0 w-4 h-4 rounded-full border-2 bg-white dark:bg-gray-700 ${dotClass}"></div>
        <div class="flex-1 min-w-0 ml-6">
          <p class="text-gray-700 dark:text-gray-300">${parseTypelessInline(item.content)}</p>
          ${timestampHtml}
        </div>
      </div>`
      })
      .join('')

    return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
      ${titleHtml}
      <div class="p-4">
        <div class="relative">
          <div class="absolute left-2 top-2 bottom-2 w-0.5 bg-gray-200 dark:bg-gray-700"></div>
          ${itemsHtml}
        </div>
      </div>
    </div>`
  }

  // Fallback
  return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700 p-4">
    <p class="text-gray-500">Unknown list variant: ${card.variant}</p>
  </div>`
}

/**
 * Render info card
 */
function renderInfo(card: TypelessCardInfo): string {
  const defaultStyle = {
    bg: 'bg-blue-50 dark:bg-blue-900/20',
    border: 'border-blue-200 dark:border-blue-800',
    icon: 'text-blue-600 dark:text-blue-300',
  }
  const variantStyles: Record<string, { bg: string; border: string; icon: string }> = {
    default: defaultStyle,
    success: {
      bg: 'bg-green-50 dark:bg-green-900/20',
      border: 'border-green-200 dark:border-green-800',
      icon: 'text-green-600 dark:text-green-300',
    },
    warning: {
      bg: 'bg-yellow-50 dark:bg-yellow-900/20',
      border: 'border-yellow-200 dark:border-yellow-800',
      icon: 'text-yellow-600 dark:text-yellow-300',
    },
    error: {
      bg: 'bg-red-50 dark:bg-red-900/20',
      border: 'border-red-200 dark:border-red-800',
      icon: 'text-red-600 dark:text-red-300',
    },
  }

  const style = variantStyles[card.variant || 'default'] || defaultStyle

  const iconHtml = card.icon ? `<span class="text-2xl">${escapeHtml(card.icon)}</span>` : ''

  const titleHtml = card.title
    ? `<h4 class="font-medium text-gray-900 dark:text-white">${escapeHtml(card.title)}</h4>`
    : ''

  const contentHtml = card.content
    ? `<p class="text-sm text-gray-600 dark:text-gray-400">${parseTypelessInline(card.content)}</p>`
    : ''

  return `<div class="info-card rounded-lg border ${style.border} ${style.bg} p-4">
    <div class="flex items-start gap-3">
      ${iconHtml ? `<div class="flex-shrink-0 ${style.icon}">${iconHtml}</div>` : ''}
      <div class="flex-1 min-w-0">
        ${titleHtml}
        ${contentHtml}
      </div>
    </div>
  </div>`
}

/**
 * Render quote card
 */
function renderQuote(card: TypelessCardQuote): string {
  const authorHtml = card.author
    ? `<footer class="mt-2 text-sm text-gray-500 dark:text-gray-400">— ${escapeHtml(card.author)}${card.source ? `, <cite>${escapeHtml(card.source)}</cite>` : ''}</footer>`
    : ''

  return `<div class="quote-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
    <blockquote class="p-4 border-l-4 border-gray-900 dark:border-white">
      <p class="text-gray-700 dark:text-gray-300 italic">${parseTypelessInline(card.content)}</p>
      ${authorHtml}
    </blockquote>
  </div>`
}

/**
 * Render alert card
 */
function renderAlert(card: TypelessCardAlert): string {
  const defaultStyle = {
    bg: 'bg-blue-50 dark:bg-blue-900/20',
    border: 'border-blue-200 dark:border-blue-800',
    text: 'text-blue-800 dark:text-blue-200',
    icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-blue-600 dark:text-blue-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`,
  }
  const variantStyles: Record<string, { bg: string; border: string; text: string; icon: string }> =
    {
      info: defaultStyle,
      success: {
        bg: 'bg-green-50 dark:bg-green-900/20',
        border: 'border-green-200 dark:border-green-800',
        text: 'text-green-800 dark:text-green-200',
        icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`,
      },
      warning: {
        bg: 'bg-yellow-50 dark:bg-yellow-900/20',
        border: 'border-yellow-200 dark:border-yellow-800',
        text: 'text-yellow-800 dark:text-yellow-200',
        icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>`,
      },
      error: {
        bg: 'bg-red-50 dark:bg-red-900/20',
        border: 'border-red-200 dark:border-red-800',
        text: 'text-red-800 dark:text-red-200',
        icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`,
      },
    }

  const style = variantStyles[card.variant || 'info'] || defaultStyle

  const titleHtml = card.title ? `<h4 class="font-medium">${escapeHtml(card.title)}</h4>` : ''

  return `<div class="alert-card rounded-lg border ${style.border} ${style.bg} p-4">
    <div class="flex items-start gap-3 ${style.text}">
      <div class="flex-shrink-0">${style.icon}</div>
      <div class="flex-1 min-w-0">
        ${titleHtml}
        <p class="text-sm">${parseTypelessInline(card.message)}</p>
      </div>
    </div>
  </div>`
}

/**
 * Render terminal card with ANSI color support
 */
function renderTerminal(card: TypelessCardTerminal): string {
  // ANSI color code to inline style mapping
  const ansiColorStyles: Record<number, string> = {
    // Standard colors (foreground)
    30: 'color: #1f2937', // black
    31: 'color: #dc2626', // red
    32: 'color: #16a34a', // green
    33: 'color: #ca8a04', // yellow
    34: 'color: #2563eb', // blue
    35: 'color: #9333ea', // purple/magenta
    36: 'color: #0891b2', // cyan
    37: 'color: #e5e7eb', // white/light gray
    // Bright colors (foreground)
    90: 'color: #6b7280', // bright black (gray)
    91: 'color: #ef4444', // bright red
    92: 'color: #22c55e', // bright green
    93: 'color: #eab308', // bright yellow
    94: 'color: #3b82f6', // bright blue
    95: 'color: #a855f7', // bright purple
    96: 'color: #06b6d4', // bright cyan
    97: 'color: #ffffff', // bright white
    // Background colors
    40: 'background-color: #000000',
    41: 'background-color: #dc2626',
    42: 'background-color: #16a34a',
    43: 'background-color: #ca8a04',
    44: 'background-color: #2563eb',
    45: 'background-color: #9333ea',
    46: 'background-color: #0891b2',
    47: 'background-color: #e5e7eb',
    // Bright background colors
    100: 'background-color: #374151',
    101: 'background-color: #ef4444',
    102: 'background-color: #22c55e',
    103: 'background-color: #eab308',
    104: 'background-color: #3b82f6',
    105: 'background-color: #a855f7',
    106: 'background-color: #06b6d4',
    107: 'background-color: #ffffff',
  }

  // Text style codes
  const ansiStyleMap: Record<number, string> = {
    1: 'font-weight: bold',
    2: 'opacity: 0.75', // Dim
    3: 'font-style: italic',
    4: 'text-decoration: underline',
    9: 'text-decoration: line-through',
  }

  // Parse ANSI escape codes and convert to HTML spans with inline styles
  function parseAnsiToHtml(text: string): string {
    // eslint-disable-next-line no-control-regex
    const ansiRegex = /\x1b\[([0-9;]*)m/g
    let result = ''
    let lastIndex = 0
    let currentStyles: string[] = []
    let match

    while ((match = ansiRegex.exec(text)) !== null) {
      // Add text before this escape sequence
      if (match.index > lastIndex) {
        const segmentText = escapeHtml(text.slice(lastIndex, match.index))
        if (segmentText) {
          if (currentStyles.length > 0) {
            result += `<span style="${currentStyles.join('; ')}">${segmentText}</span>`
          } else {
            result += segmentText
          }
        }
      }

      // Parse the escape codes
      const codes = (match[1] || '').split(';').map((c) => parseInt(c, 10) || 0)

      for (const code of codes) {
        if (code === 0) {
          // Reset all
          currentStyles = []
        } else if (ansiColorStyles[code]) {
          // Remove existing color/bg style of same type
          const isBg = code >= 40
          currentStyles = currentStyles.filter((s) => {
            if (isBg) return !s.startsWith('background-color')
            return !s.startsWith('color')
          })
          currentStyles.push(ansiColorStyles[code])
        } else if (ansiStyleMap[code]) {
          if (!currentStyles.includes(ansiStyleMap[code])) {
            currentStyles.push(ansiStyleMap[code])
          }
        }
      }

      lastIndex = match.index + match[0].length
    }

    // Add remaining text
    if (lastIndex < text.length) {
      const segmentText = escapeHtml(text.slice(lastIndex))
      if (currentStyles.length > 0) {
        result += `<span style="${currentStyles.join('; ')}">${segmentText}</span>`
      } else {
        result += segmentText
      }
    }

    return result
  }

  // Strip ANSI codes for plain text (used for copying)
  function stripAnsi(text: string): string {
    // eslint-disable-next-line no-control-regex
    return text.replace(/\x1b\[[0-9;]*m/g, '')
  }

  const titleHtml = card.title
    ? `<span class="text-sm text-gray-400">${escapeHtml(card.title)}</span>`
    : ''

  const promptHtml =
    card.showPrompt && card.prompt
      ? `<span style="color: #22c55e">${escapeHtml(card.prompt)}</span>`
      : ''

  // Parse content with ANSI codes
  const parsedContent = parseAnsiToHtml(card.content)

  // Generate globally unique DOM ID for copy functionality
  const terminalId = `terminal-${++domIdCounter}`
  const plainContent = stripAnsi(card.content)

  // Theme classes
  const themeClass = card.theme === 'light' ? 'bg-gray-100' : 'bg-gray-700'
  const textClass = card.theme === 'light' ? 'text-gray-900' : 'text-gray-100'

  // Max height style
  const maxHeightStyle = card.maxHeight ? `max-height: ${card.maxHeight}px;` : ''

  // Data for fullscreen — base64-encode to avoid HTML attribute escaping issues
  const fullscreenDataB64 = btoa(
    unescape(
      encodeURIComponent(
        JSON.stringify({
          title: card.title || 'Terminal',
          content: plainContent,
        })
      )
    )
  )

  return `<div class="terminal-card rounded-lg border border-gray-700 overflow-hidden" ondblclick="window.__typelessOpenFullscreen && window.__typelessOpenFullscreen('terminal', '${fullscreenDataB64}', true)">
    <div class="flex items-center justify-between px-3 py-1.5 bg-gray-700 border-b border-gray-700">
      <div class="flex items-center gap-2">
        <div class="flex gap-1">
          <div class="w-2.5 h-2.5 rounded-full bg-red-500"></div>
          <div class="w-2.5 h-2.5 rounded-full bg-yellow-500"></div>
          <div class="w-2.5 h-2.5 rounded-full bg-green-500"></div>
        </div>
        ${titleHtml}
        <span class="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-300">${t('terminalCard.title', 'Terminal')}</span>
      </div>
      <div class="flex items-center gap-1.5">
        <span class="text-xs text-gray-500 hidden sm:inline" title="${t('media.fullscreen', 'Full Screen')}">⤢</span>
        <button
          class="typeless-copy-btn flex items-center p-1 text-gray-400 hover:text-white transition-colors rounded hover:bg-gray-700"
          data-terminal-id="${terminalId}"
          data-terminal-content="${escapeHtml(plainContent).replace(/"/g, '&quot;')}"
          onclick="event.stopPropagation(); window.__typelessCopyTerminal && window.__typelessCopyTerminal('${terminalId}')"
          title="${t('common.copy', 'Copy')}"
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>
      </div>
    </div>
    <div class="overflow-auto ${themeClass} cursor-pointer" style="${maxHeightStyle}" title="${t('media.fullscreen', 'Full Screen')}">
      <pre id="${terminalId}" class="p-4 text-sm leading-relaxed ${textClass}" style="margin: 0; font-family: 'Fira Code', 'Monaco', 'Consolas', 'Liberation Mono', 'Courier New', monospace; white-space: pre-wrap; word-wrap: break-word;">${promptHtml}${parsedContent}</pre>
    </div>
  </div>`
}

/**
 * Initialize copy code functionality (call once on app mount)
 */
export function initTypelessCopyHandler(): void {
  // Add global copy handler for code
  ;(window as unknown as { __typelessCopyCode?: (codeId: string) => void }).__typelessCopyCode =
    async (codeId: string) => {
      const codeElement = document.getElementById(codeId)
      if (!codeElement) return

      try {
        // Get text content, removing line numbers
        const code = codeElement.textContent || ''
        await navigator.clipboard.writeText(code)

        // Find the button and update it
        const btn = document.querySelector(`[data-code-id="${codeId}"]`)
        if (btn) {
          const originalHtml = btn.innerHTML
          btn.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>`
          setTimeout(() => {
            btn.innerHTML = originalHtml
          }, 2000)
        }
      } catch (err) {
        console.error('Failed to copy code:', err)
      }
    }

  // Add global collapse toggle handler for long code cards
  ;(
    window as unknown as { __typelessToggleCodeCollapse?: (toggleButtonId: string) => void }
  ).__typelessToggleCodeCollapse = (toggleButtonId: string) => {
    const button = document.getElementById(toggleButtonId)
    if (!button) return

    const containerId = button.getAttribute('data-code-container-id')
    const fadeId = button.getAttribute('data-code-fade-id')
    if (!containerId) return

    const container = document.getElementById(containerId)
    if (!container) return

    const fade = fadeId ? document.getElementById(fadeId) : null
    const expandLabel = button.getAttribute('data-expand-label') || 'Show more'
    const collapseLabel = button.getAttribute('data-collapse-label') || 'Collapse'
    const collapsedMaxHeight = Number(button.getAttribute('data-collapsed-max-height') || 0)
    const isCollapsed = container.getAttribute('data-collapsed') !== 'false'

    if (isCollapsed) {
      container.style.maxHeight = 'none'
      container.style.overflowY = 'auto'
      container.setAttribute('data-collapsed', 'false')
      fade?.classList.add('hidden')
      button.textContent = collapseLabel
      return
    }

    container.style.maxHeight = collapsedMaxHeight > 0 ? `${collapsedMaxHeight}px` : ''
    container.style.overflowY = 'hidden'
    container.setAttribute('data-collapsed', 'true')
    fade?.classList.remove('hidden')
    button.textContent = expandLabel
  }

  // Add global copy handler for terminal
  ;(
    window as unknown as { __typelessCopyTerminal?: (terminalId: string) => void }
  ).__typelessCopyTerminal = async (terminalId: string) => {
    const btn = document.querySelector(`[data-terminal-id="${terminalId}"]`)
    if (!btn) return

    try {
      // Get plain content from data attribute (ANSI codes already stripped)
      const content = btn.getAttribute('data-terminal-content') || ''
      // Decode HTML entities
      const textarea = document.createElement('textarea')
      textarea.innerHTML = content
      const plainText = textarea.value

      await navigator.clipboard.writeText(plainText)

      // Update button
      const originalHtml = btn.innerHTML
      btn.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg>`
      setTimeout(() => {
        btn.innerHTML = originalHtml
      }, 2000)
    } catch (err) {
      console.error('Failed to copy terminal content:', err)
    }
  }

  // Add global fullscreen handler for code/terminal cards
  // This is called from the ondblclick handler in the rendered HTML
  // The actual fullscreen state is managed by the useFullscreen composable
  // which is imported by the FullscreenModal component
  ;(
    window as unknown as {
      __typelessOpenFullscreen?: (type: string, dataJson: string, isBase64?: boolean) => void
    }
  ).__typelessOpenFullscreen = (type: string, dataJson: string, isBase64?: boolean) => {
    // Decode base64 if flagged (avoids HTML attribute escaping issues with JSON)
    const json = isBase64 ? decodeURIComponent(escape(atob(dataJson))) : dataJson
    // Dispatch a custom event that the FullscreenModal component listens to
    const event = new CustomEvent('typeless-fullscreen', {
      detail: { type, dataJson: json },
    })
    window.dispatchEvent(event)
  }
}
