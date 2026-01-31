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
  ListItem,
} from '@/types/typeless'
import { parseInline } from './markdown'

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
    // Use card id if available, otherwise hash the content
    if (card.id) return `${card.type}:${card.id}`
    return `${card.type}:${this.hashCard(card)}`
  }

  private hashCard(card: TypelessCard): string {
    // Simple hash based on JSON stringification
    const str = JSON.stringify(card)
    let hash = 0
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i)
      hash = ((hash << 5) - hash) + char
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

// Card types that can be rendered functionally (simple, no interactivity beyond copy)
export const FUNCTIONAL_CARD_TYPES = new Set([
  'table',
  'code',
  'list',
  'info',
  'quote',
  'alert',
])

// Card types that require Vue components (complex interactivity, async loading, etc.)
export const COMPONENT_CARD_TYPES = new Set([
  'link',      // Needs async link preview fetch
  'file',      // Has download/preview buttons
  'gallery',   // Has lightbox, scroll buttons
  'chart',     // Uses chart library
  'map',       // Uses map library
  'action',    // Has interactive buttons
  'choice',    // Has selection state
  'progress',  // May have animations
  'result',    // Has action buttons
  'detection', // Complex UI
  'metric',    // May have animations
  'comparison',// Complex layout
  'steps',     // Interactive steps
  'weather',   // Complex UI
  'profile',   // Complex UI
  'countdown', // Has timer
  'rating',    // Interactive
  'accordion', // Has expand/collapse state
  'audio',     // Has audio player
  'collapsible-code', // Has expand/collapse state
  'diff',      // Complex highlighting
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
  // Check cache first
  const cached = renderCache.get(card)
  if (cached) return cached

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
    default:
      html = `<div class="text-red-500">Unknown card type: ${card.type}</div>`
  }

  renderCache.set(card, html)
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

/**
 * Render table card
 */
function renderTable(card: TypelessCardTable): string {
  const titleHtml = card.title
    ? `<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
        <h4 class="font-medium text-gray-900 dark:text-white">${escapeHtml(card.title)}</h4>
      </div>`
    : ''

  const headersHtml = card.headers.length > 0
    ? `<thead>
        <tr class="bg-gray-50 dark:bg-gray-800/50">
          ${card.headers.map(h => `<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">${parseInline(String(h))}</th>`).join('')}
        </tr>
      </thead>`
    : ''

  const rowsHtml = card.rows.map((row, rowIndex) => {
    const stripedClass = card.striped && rowIndex % 2 === 1 ? 'bg-gray-50 dark:bg-gray-800/30' : ''
    const cells = row.map(cell =>
      `<td class="px-4 py-3 text-gray-700 dark:text-gray-300${card.compact ? ' py-2' : ''}">${parseInline(String(cell))}</td>`
    ).join('')
    return `<tr class="${stripedClass} hover:bg-gray-50 dark:hover:bg-gray-800/50">${cells}</tr>`
  }).join('')

  const footerHtml = card.footer
    ? `<div class="px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
        <p class="text-xs text-gray-500 dark:text-gray-400">${escapeHtml(card.footer)}</p>
      </div>`
    : ''

  return `<div class="table-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
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
    js: 'JavaScript', javascript: 'JavaScript',
    ts: 'TypeScript', typescript: 'TypeScript',
    py: 'Python', python: 'Python',
    go: 'Go', rust: 'Rust', java: 'Java',
    cpp: 'C++', c: 'C', html: 'HTML', css: 'CSS',
    json: 'JSON', yaml: 'YAML', sql: 'SQL',
    bash: 'Bash', shell: 'Shell',
    md: 'Markdown', markdown: 'Markdown',
  }

  const langDisplay = card.language
    ? languageNames[card.language.toLowerCase()] || card.language
    : 'Text'

  const lines = card.code.split('\n')
  // Disable line numbers for plain text (no language specified)
  const isPlainText = !card.language
  const showLineNumbers = !isPlainText && card.showLineNumbers !== false
  const highlightLines = new Set(card.highlightLines || [])

  const linesHtml = lines.map((line, index) => {
    const lineNum = index + 1
    const highlighted = highlightLines.has(lineNum) ? ' bg-yellow-500/20' : ''
    const lineNumHtml = showLineNumbers
      ? `<span class="inline-block w-8 text-right mr-4 text-gray-500 select-none">${lineNum}</span>`
      : ''
    // Add newline at the end for proper copying
    const lineContent = index < lines.length - 1 ? `${escapeHtml(line)}\n` : escapeHtml(line)
    return `<span class="block${highlighted}">${lineNumHtml}${lineContent}</span>`
  }).join('')

  const titleOrFilename = card.filename || card.title || ''
  const titleHtml = titleOrFilename
    ? `<span class="text-sm text-gray-600 dark:text-gray-400">${escapeHtml(titleOrFilename)}</span>`
    : ''
  const langBadge = langDisplay
    ? `<span class="px-2 py-0.5 text-xs rounded bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300">${escapeHtml(langDisplay)}</span>`
    : ''

  // Generate unique ID for copy functionality
  const codeId = `code-${card.id || Math.random().toString(36).substr(2, 9)}`

  return `<div class="code-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-900">
    <div class="flex items-center justify-between px-4 py-2 bg-gray-50 dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
      <div class="flex items-center gap-3">
        <div class="flex gap-1.5">
          <div class="w-3 h-3 rounded-full bg-red-500"></div>
          <div class="w-3 h-3 rounded-full bg-yellow-500"></div>
          <div class="w-3 h-3 rounded-full bg-green-500"></div>
        </div>
        ${titleHtml}
        ${langBadge}
      </div>
      <button
        class="typeless-copy-btn flex items-center gap-1.5 px-2 py-1 text-xs text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors rounded hover:bg-gray-200 dark:hover:bg-gray-700"
        data-code-id="${codeId}"
        onclick="window.__typelessCopyCode && window.__typelessCopyCode('${codeId}')"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
        <span>Copy</span>
      </button>
    </div>
    <div class="overflow-x-auto">
      <pre class="p-4 text-sm leading-relaxed" style="margin: 0; font-family: 'Fira Code', 'Monaco', 'Consolas', monospace;"><code id="${codeId}" class="text-gray-800 dark:text-gray-100">${linesHtml}</code></pre>
    </div>
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
          ${item.subItems.map(sub => renderListItem(sub, true)).join('')}
        </ul>`
      : ''

    return `<li class="flex items-start gap-2 ${textClass}">
      ${iconHtml}
      <div class="flex-1">
        <span>${parseInline(item.content)}</span>
        ${subItemsHtml}
      </div>
    </li>`
  }

  // Default variant
  if (!card.variant || card.variant === 'default') {
    const tag = card.ordered ? 'ol' : 'ul'
    const listClass = card.ordered ? 'list-decimal list-inside' : ''
    const itemsHtml = card.items.map(item => renderListItem(item)).join('')

    return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
      ${titleHtml}
      <${tag} class="p-4 space-y-2 ${listClass}">
        ${itemsHtml}
      </${tag}>
    </div>`
  }

  // Checklist variant
  if (card.variant === 'checklist') {
    const itemsHtml = card.items.map((item, index) => {
      const checked = item.checked || false
      const checkboxClass = checked
        ? 'bg-blue-500 border-blue-500'
        : 'border-gray-300 dark:border-gray-600'
      const checkIcon = checked
        ? `<svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
          </svg>`
        : ''
      const textClass = checked ? 'line-through text-gray-400 dark:text-gray-500' : ''

      return `<li class="flex items-center gap-3" data-item-index="${index}">
        <div class="flex-shrink-0 w-5 h-5 rounded border-2 flex items-center justify-center ${checkboxClass}">
          ${checkIcon}
        </div>
        <span class="flex-1 text-gray-700 dark:text-gray-300 transition-colors ${textClass}">${parseInline(item.content)}</span>
      </li>`
    }).join('')

    return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
      ${titleHtml}
      <ul class="p-4 space-y-2">
        ${itemsHtml}
      </ul>
    </div>`
  }

  // Timeline variant
  if (card.variant === 'timeline') {
    const itemsHtml = card.items.map((item, index) => {
      const dotClass = index === 0
        ? 'border-blue-500 bg-blue-500'
        : 'border-gray-300 dark:border-gray-600'
      const timestampHtml = item.timestamp
        ? `<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">${escapeHtml(item.timestamp)}</p>`
        : ''

      return `<div class="relative flex items-start gap-4 pb-4 last:pb-0">
        <div class="absolute left-0 w-4 h-4 rounded-full border-2 bg-white dark:bg-gray-800 ${dotClass}"></div>
        <div class="flex-1 min-w-0 ml-6">
          <p class="text-gray-700 dark:text-gray-300">${parseInline(item.content)}</p>
          ${timestampHtml}
        </div>
      </div>`
    }).join('')

    return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
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
  return `<div class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800 p-4">
    <p class="text-gray-500">Unknown list variant: ${card.variant}</p>
  </div>`
}

/**
 * Render info card
 */
function renderInfo(card: TypelessCardInfo): string {
  const defaultStyle = { bg: 'bg-blue-50 dark:bg-blue-900/20', border: 'border-blue-200 dark:border-blue-800', icon: 'text-blue-500' }
  const variantStyles: Record<string, { bg: string; border: string; icon: string }> = {
    default: defaultStyle,
    success: { bg: 'bg-green-50 dark:bg-green-900/20', border: 'border-green-200 dark:border-green-800', icon: 'text-green-500' },
    warning: { bg: 'bg-yellow-50 dark:bg-yellow-900/20', border: 'border-yellow-200 dark:border-yellow-800', icon: 'text-yellow-500' },
    error: { bg: 'bg-red-50 dark:bg-red-900/20', border: 'border-red-200 dark:border-red-800', icon: 'text-red-500' },
  }

  const style = variantStyles[card.variant || 'default'] || defaultStyle

  const iconHtml = card.icon
    ? `<span class="text-2xl">${escapeHtml(card.icon)}</span>`
    : ''

  const titleHtml = card.title
    ? `<h4 class="font-medium text-gray-900 dark:text-white">${escapeHtml(card.title)}</h4>`
    : ''

  const contentHtml = card.content
    ? `<p class="text-sm text-gray-600 dark:text-gray-400">${parseInline(card.content)}</p>`
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

  return `<div class="quote-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <blockquote class="p-4 border-l-4 border-blue-500">
      <p class="text-gray-700 dark:text-gray-300 italic">${parseInline(card.content)}</p>
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
    icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`
  }
  const variantStyles: Record<string, { bg: string; border: string; text: string; icon: string }> = {
    info: defaultStyle,
    success: {
      bg: 'bg-green-50 dark:bg-green-900/20',
      border: 'border-green-200 dark:border-green-800',
      text: 'text-green-800 dark:text-green-200',
      icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`
    },
    warning: {
      bg: 'bg-yellow-50 dark:bg-yellow-900/20',
      border: 'border-yellow-200 dark:border-yellow-800',
      text: 'text-yellow-800 dark:text-yellow-200',
      icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>`
    },
    error: {
      bg: 'bg-red-50 dark:bg-red-900/20',
      border: 'border-red-200 dark:border-red-800',
      text: 'text-red-800 dark:text-red-200',
      icon: `<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>`
    },
  }

  const style = variantStyles[card.variant || 'info'] || defaultStyle

  const titleHtml = card.title
    ? `<h4 class="font-medium">${escapeHtml(card.title)}</h4>`
    : ''

  return `<div class="alert-card rounded-lg border ${style.border} ${style.bg} p-4">
    <div class="flex items-start gap-3 ${style.text}">
      <div class="flex-shrink-0">${style.icon}</div>
      <div class="flex-1 min-w-0">
        ${titleHtml}
        <p class="text-sm">${parseInline(card.message)}</p>
      </div>
    </div>
  </div>`
}

/**
 * Initialize copy code functionality (call once on app mount)
 */
export function initTypelessCopyHandler(): void {
  // Add global copy handler
  (window as unknown as { __typelessCopyCode?: (codeId: string) => void }).__typelessCopyCode = async (codeId: string) => {
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
        btn.innerHTML = `<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" /></svg><span>Copied!</span>`
        setTimeout(() => {
          btn.innerHTML = originalHtml
        }, 2000)
      }
    } catch (err) {
      console.error('Failed to copy code:', err)
    }
  }
}
