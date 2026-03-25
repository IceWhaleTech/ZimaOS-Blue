// Simple markdown renderer with code syntax highlighting support
// Uses highlight.js for syntax highlighting (lazy-loaded on first use)

import hljs from 'highlight.js/lib/core'
import { i18n } from '@/i18n'

function t(key: string, fallback: string): string {
  const result = i18n.global.t(key)
  return result === key ? fallback : String(result)
}

let hljsReady = false
let hljsLoading: Promise<void> | null = null

/** Preload highlight.js languages. Call this when a component that needs highlighting mounts. */
export function preloadHljs(): Promise<void> {
  if (hljsReady) return Promise.resolve()
  if (hljsLoading) return hljsLoading
  hljsLoading = (async () => {
    const [
      javascript,
      typescript,
      python,
      go,
      bash,
      json,
      yaml,
      xml,
      css,
      sql,
      rust,
      java,
      cpp,
      markdown,
      php,
      ruby,
      swift,
      kotlin,
      csharp,
      scala,
      dockerfile,
      nginx,
      ini,
      diff,
      plaintext,
    ] = await Promise.all([
      import('highlight.js/lib/languages/javascript'),
      import('highlight.js/lib/languages/typescript'),
      import('highlight.js/lib/languages/python'),
      import('highlight.js/lib/languages/go'),
      import('highlight.js/lib/languages/bash'),
      import('highlight.js/lib/languages/json'),
      import('highlight.js/lib/languages/yaml'),
      import('highlight.js/lib/languages/xml'),
      import('highlight.js/lib/languages/css'),
      import('highlight.js/lib/languages/sql'),
      import('highlight.js/lib/languages/rust'),
      import('highlight.js/lib/languages/java'),
      import('highlight.js/lib/languages/cpp'),
      import('highlight.js/lib/languages/markdown'),
      import('highlight.js/lib/languages/php'),
      import('highlight.js/lib/languages/ruby'),
      import('highlight.js/lib/languages/swift'),
      import('highlight.js/lib/languages/kotlin'),
      import('highlight.js/lib/languages/csharp'),
      import('highlight.js/lib/languages/scala'),
      import('highlight.js/lib/languages/dockerfile'),
      import('highlight.js/lib/languages/nginx'),
      import('highlight.js/lib/languages/ini'),
      import('highlight.js/lib/languages/diff'),
      import('highlight.js/lib/languages/plaintext'),
    ])

    const register = (names: string[], mod: any) => {
      const lang = mod.default || mod
      for (const name of names) hljs.registerLanguage(name, lang)
    }

    register(['javascript', 'js', 'jsx'], javascript)
    register(['typescript', 'ts', 'tsx'], typescript)
    register(['python', 'py'], python)
    register(['go', 'golang'], go)
    register(['bash', 'sh', 'shell', 'zsh'], bash)
    register(['json'], json)
    register(['yaml', 'yml'], yaml)
    register(['xml', 'html', 'vue', 'svg'], xml)
    register(['css', 'scss', 'less'], css)
    register(['sql'], sql)
    register(['rust', 'rs'], rust)
    register(['java'], java)
    register(['cpp', 'c', 'cc', 'h'], cpp)
    register(['markdown', 'md'], markdown)
    register(['php'], php)
    register(['ruby', 'rb'], ruby)
    register(['swift'], swift)
    register(['kotlin', 'kt'], kotlin)
    register(['csharp', 'cs'], csharp)
    register(['scala'], scala)
    register(['dockerfile', 'docker'], dockerfile)
    register(['nginx'], nginx)
    register(['ini', 'toml', 'conf', 'env'], ini)
    register(['diff', 'patch'], diff)
    register(['plaintext', 'text', 'txt'], plaintext)

    hljsReady = true
  })()
  return hljsLoading
}

export interface RenderOptions {
  sanitize?: boolean
}

export interface ParseInlineOptions {
  allowUnderscoreEmphasis?: boolean
}

const RE_INLINE_BOLD_ASTERISK = /\*\*(.+?)\*\*/g
const RE_INLINE_BOLD_UNDERSCORE = /__(.+?)__/g
const RE_INLINE_ITALIC_ASTERISK = /\*(.+?)\*/g
const RE_INLINE_ITALIC_UNDERSCORE = /_(.+?)_/g
const RE_INLINE_STRIKETHROUGH = /~~(.+?)~~/g
const RE_INLINE_CODE = /`([^`]+)`/g
const RE_INLINE_UNCLOSED_CODE = /`([^`]+)$/g
const RE_INLINE_LINK = /\[([^\]]+)\]\(([^)]+)\)/g
const RE_ERROR_BLOCK_TAG = /<(tool_use_error|error|system-error)>([\s\S]*?)<\/\1>/g
const INLINE_PARSE_CACHE_MAX = 2000
const INLINE_PARSE_CACHE_MAX_TEXT_LENGTH = 512
const inlineParseCache = new Map<string, string>()

function evictOldestMapEntry<K, V>(cache: Map<K, V>): void {
  const oldestKey = cache.keys().next().value
  if (oldestKey !== undefined) {
    cache.delete(oldestKey as K)
  }
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

// Parse inline markdown elements
export function parseInline(text: string, options: ParseInlineOptions = {}): string {
  const allowUnderscoreEmphasis = options.allowUnderscoreEmphasis ?? true
  const cacheKey = allowUnderscoreEmphasis ? text : `no_underscore:${text}`
  const shouldUseCache = text.length > 0 && text.length <= INLINE_PARSE_CACHE_MAX_TEXT_LENGTH
  if (shouldUseCache) {
    const cached = inlineParseCache.get(cacheKey)
    if (cached !== undefined) return cached
  }

  if (
    !text.includes('*') &&
    (!allowUnderscoreEmphasis || !text.includes('_')) &&
    !text.includes('~') &&
    !text.includes('`') &&
    !text.includes('[')
  ) {
    const escaped = escapeHtml(text)
    if (shouldUseCache) {
      if (inlineParseCache.size >= INLINE_PARSE_CACHE_MAX && !inlineParseCache.has(cacheKey)) {
        evictOldestMapEntry(inlineParseCache)
      }
      inlineParseCache.set(cacheKey, escaped)
    }
    return escaped
  }

  let result = escapeHtml(text)

  if (text.includes('**') || (allowUnderscoreEmphasis && text.includes('__'))) {
    // Bold: **text** or __text__
    result = result.replace(RE_INLINE_BOLD_ASTERISK, '<strong>$1</strong>')
    if (allowUnderscoreEmphasis) {
      result = result.replace(RE_INLINE_BOLD_UNDERSCORE, '<strong>$1</strong>')
    }
  }

  if (text.includes('*')) {
    // Italic: *text*
    result = result.replace(RE_INLINE_ITALIC_ASTERISK, '<em>$1</em>')
  }
  if (allowUnderscoreEmphasis && text.includes('_')) {
    // Italic: _text_
    result = result.replace(RE_INLINE_ITALIC_UNDERSCORE, '<em>$1</em>')
  }

  if (text.includes('~~')) {
    // Strikethrough: ~~text~~
    result = result.replace(RE_INLINE_STRIKETHROUGH, '<del>$1</del>')
  }

  if (text.includes('`')) {
    // Inline code: `code` — also handle unclosed backtick at end of line (streaming)
    result = result.replace(RE_INLINE_CODE, '<code class="inline-code">$1</code>')
    // Unclosed trailing backtick: `code... (no closing backtick)
    result = result.replace(RE_INLINE_UNCLOSED_CODE, '<code class="inline-code">$1</code>')
  }

  if (text.includes('[') && text.includes('](')) {
    // Links: [text](url) — also handle unclosed links gracefully
    result = result.replace(
      RE_INLINE_LINK,
      '<a href="$2" target="_blank" rel="noopener noreferrer" class="text-gray-900 dark:text-white hover:underline">$1</a>'
    )
  }

  if (shouldUseCache) {
    if (inlineParseCache.size >= INLINE_PARSE_CACHE_MAX && !inlineParseCache.has(cacheKey)) {
      evictOldestMapEntry(inlineParseCache)
    }
    inlineParseCache.set(cacheKey, result)
  }

  return result
}

// Convert markdown text to plain text for voice/transcript surfaces.
export function markdownToText(markdown: string): string {
  if (!markdown) return ''

  let text = markdown.replace(/\r\n/g, '\n')

  // Fenced code blocks: keep code content, drop fences and language marker.
  text = text.replace(/```[\t ]*([\w-]+)?\n([\s\S]*?)```/g, (_m, _lang: string, code: string) =>
    code.trim()
  )
  // Inline code.
  text = text.replace(/`([^`]+)`/g, '$1')

  // Images/links: keep human-readable label.
  text = text.replace(/!\[([^\]]*)\]\([^)]+\)/g, '$1')
  text = text.replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')

  // Block prefixes.
  text = text.replace(/^[\t ]{0,3}#{1,6}[\t ]+/gm, '')
  text = text.replace(/^[\t ]{0,3}>\s?/gm, '')
  text = text.replace(/^[\t ]{0,3}(?:[-*+]|\d+[.)])\s+/gm, '')
  text = text.replace(/^[\t ]{0,3}[-*_]{3,}\s*$/gm, '')

  // Markdown table separators.
  text = text.replace(/^[\t ]*\|?[\t :\-]+\|[\t :\-|]*$/gm, '')

  // Inline emphasis.
  text = text.replace(/\*\*(.*?)\*\*/g, '$1')
  text = text.replace(/__(.*?)__/g, '$1')
  text = text.replace(/\*(.*?)\*/g, '$1')
  text = text.replace(/_(.*?)_/g, '$1')
  text = text.replace(/~~(.*?)~~/g, '$1')

  // Trim trailing space per line and collapse extra blank lines.
  text = text
    .split('\n')
    .map((line) => line.replace(/[ \t]+$/g, ''))
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()

  return text
}

// Detect language from code fence
function detectLanguage(lang: string): string {
  const aliases: Record<string, string> = {
    js: 'javascript',
    ts: 'typescript',
    py: 'python',
    rb: 'ruby',
    sh: 'bash',
    shell: 'bash',
    yml: 'yaml',
    md: 'markdown',
  }
  return aliases[lang.toLowerCase()] || lang.toLowerCase()
}

// Simple syntax highlighting using highlight.js
export function highlightCode(code: string, language: string): string {
  // Trigger lazy loading if not started yet
  if (!hljsReady) {
    preloadHljs()
    // Languages not loaded yet — return escaped code (will highlight after reload)
    return escapeHtml(code)
  }
  if (language && hljs.getLanguage(language)) {
    try {
      return hljs.highlight(code, { language }).value
    } catch {
      // Fall back to escaped code on error
    }
  }
  // Auto-detect language if not specified or not supported
  try {
    return hljs.highlightAuto(code).value
  } catch {
    return escapeHtml(code)
  }
}

// Parse a table row into cells
function parseTableRow(line: string): string[] {
  // Remove leading/trailing pipes and split by pipe
  const trimmed = line.trim()
  const withoutPipes = trimmed.startsWith('|') ? trimmed.slice(1) : trimmed
  const withoutEndPipe = withoutPipes.endsWith('|') ? withoutPipes.slice(0, -1) : withoutPipes
  return withoutEndPipe.split('|').map((cell) => cell.trim())
}

// Check if a line is a table separator (e.g., |---|---|)
function isTableSeparator(line: string): boolean {
  const trimmed = line.trim()
  // Must contain at least one pipe to be a table separator
  if (!trimmed.includes('|')) return false
  // Remove pipes and check if remaining content is only dashes, colons, and spaces
  const content = trimmed.replace(/\|/g, '').trim()
  return /^[\s:-]+$/.test(content) && content.includes('-')
}

// Check if a line looks like a table row
function isTableRow(line: string): boolean {
  const trimmed = line.trim()
  // Must contain ASCII pipe character (U+007C), not box-drawing characters
  if (!trimmed.includes('|')) return false
  if (isTableSeparator(trimmed)) return false

  // Exclude lines that look like tree structures (contain box-drawing characters)
  // Common box-drawing characters used in tree structures
  const boxDrawingChars = /[│├└┌┐┘┬┴┼─]/
  if (boxDrawingChars.test(trimmed)) return false

  // A valid table row should have pipe as a delimiter with content on both sides
  // or start/end with pipe (standard markdown table format)
  const pipeCount = (trimmed.match(/\|/g) || []).length
  if (pipeCount === 0) return false

  // If line starts or ends with pipe, it's likely a table
  if (trimmed.startsWith('|') || trimmed.endsWith('|')) return true

  // Otherwise, require at least one pipe with non-whitespace content on both sides
  const parts = trimmed.split('|')
  if (parts.length < 2) return false

  // Check that we have actual content (not just whitespace) in at least 2 cells
  const nonEmptyCells = parts.filter((p) => p.trim().length > 0)
  return nonEmptyCells.length >= 2
}

// Check if a line is part of a tree structure (uses box-drawing characters)
function isTreeLine(line: string): boolean {
  // Box-drawing characters commonly used in tree structures
  const boxDrawingChars = /[│├└┌┐┘┬┴┼─]/
  return boxDrawingChars.test(line)
}

// Render a process code block as a simple wireframe card (command above, output below)
function renderProcessCard(code: string): string {
  try {
    const items = JSON.parse(code) as Array<{
      cmd: string
      tool: string
      icon: string
      status: string
      output: string
    }>
    const rows = items
      .map((item) => {
        // Command/Input section
        const commandHtml = item.cmd
          ? `<div class="process-card__command"><span class="process-card__command-label">$</span><span class="process-card__command-text">${escapeHtml(item.cmd)}</span></div>`
          : ''

        // Output section
        let outputHtml = ''
        if (item.output) {
          const outputText = escapeHtml(item.output)
          outputHtml = `<div class="process-card__output"><pre>${outputText}</pre></div>`
        }

        return `<div class="process-card__item">${commandHtml}${outputHtml}</div>`
      })
      .join('')
    return `<div class="process-card my-2 rounded border border-gray-200 dark:border-gray-700 px-2 py-1.5">${rows}</div>`
  } catch {
    return `<pre class="text-xs opacity-60 my-2">${escapeHtml(code)}</pre>`
  }
}

const RE_MARKDOWN_HR = /^[-*_]{3,}$/
const RE_MARKDOWN_ORDERED_ITEM = /^\d+\.\s/
const RE_MARKDOWN_ORDERED_ITEM_MULTILINE = /(^|\n)\s*\d+\.\s/
const RE_BOX_DRAWING_CHARS = /[│├└┌┐┘┬┴┼─]/
type MarkdownFastPathMode = 'plain' | 'multiline-plain'

type PlainFastPathState = {
  mode: 'plain'
  markdown: string
  html: string
}

type MultilinePlainFastPathState = {
  mode: 'multiline-plain'
  markdown: string
  html: string
  lines: string[]
  renderedLines: string[]
}

type MarkdownFastPathState = PlainFastPathState | MultilinePlainFastPathState

function canUsePlainTextFastPath(markdown: string): boolean {
  if (markdown.length === 0) return false
  if (markdown.includes('\n')) return false
  const trimmed = markdown.trim()
  if (RE_MARKDOWN_HR.test(trimmed)) return false
  if (RE_MARKDOWN_ORDERED_ITEM.test(trimmed)) return false

  return (
    !markdown.includes('`') &&
    !markdown.includes('*') &&
    !markdown.includes('_') &&
    !markdown.includes('~') &&
    !markdown.includes('[') &&
    !markdown.includes(']') &&
    !markdown.includes('#') &&
    !markdown.includes('>') &&
    !markdown.includes('|') &&
    !markdown.includes('!') &&
    !markdown.includes('<') &&
    !markdown.includes('- ') &&
    !markdown.includes('+ ')
  )
}

function canUseMultilinePlainTextFastPath(markdown: string): boolean {
  if (markdown.length === 0) return false
  if (!markdown.includes('\n')) return false

  // Keep this path conservative: only use when markdown punctuation/features
  // are clearly absent from the whole text.
  if (
    markdown.includes('`') ||
    markdown.includes('*') ||
    markdown.includes('_') ||
    markdown.includes('~') ||
    markdown.includes('[') ||
    markdown.includes(']') ||
    markdown.includes('#') ||
    markdown.includes('>') ||
    markdown.includes('|') ||
    markdown.includes('!') ||
    markdown.includes('<') ||
    markdown.includes('- ') ||
    markdown.includes('+ ')
  ) {
    return false
  }

  // Preserve tree-structure rendering path.
  if (RE_BOX_DRAWING_CHARS.test(markdown)) {
    return false
  }

  // Ordered-list detection remains line-based.
  if (markdown.includes('.') && RE_MARKDOWN_ORDERED_ITEM_MULTILINE.test(markdown)) {
    return false
  }

  return true
}

function getMarkdownFastPathMode(markdown: string): MarkdownFastPathMode | null {
  if (canUsePlainTextFastPath(markdown)) {
    return 'plain'
  }
  if (canUseMultilinePlainTextFastPath(markdown)) {
    return 'multiline-plain'
  }
  return null
}

function renderPlainTextLine(line: string): string {
  if (line.trim() === '') {
    return '<br>'
  }
  return `<p class="my-1">${escapeHtml(line)}</p>`
}

function renderPlainTextParagraph(markdown: string): string {
  return `<p class="my-1">${escapeHtml(markdown)}</p>`
}

function renderMultilinePlainTextWithMeta(markdown: string): {
  html: string
  lines: string[]
  renderedLines: string[]
} {
  const lines = markdown.split('\n')
  const renderedLines = new Array<string>(lines.length)
  for (let index = 0; index < lines.length; index++) {
    renderedLines[index] = renderPlainTextLine(lines[index] || '')
  }
  return {
    html: renderedLines.join('\n'),
    lines,
    renderedLines,
  }
}

function renderMultilinePlainText(markdown: string): string {
  return renderMultilinePlainTextWithMeta(markdown).html
}

const MARKDOWN_INLINE_OPTIONS: ParseInlineOptions = { allowUnderscoreEmphasis: false }

// Main render function
export function renderMarkdown(markdown: string, _options: RenderOptions = {}): string {
  const fastPathMode = getMarkdownFastPathMode(markdown)
  if (fastPathMode === 'plain') {
    return renderPlainTextParagraph(markdown)
  }
  if (fastPathMode === 'multiline-plain') {
    return renderMultilinePlainText(markdown)
  }

  // Pre-process: convert XML-like error/status tags into styled blocks before line splitting
  // Matches <tool_use_error>...</tool_use_error> and similar tags (may span multiple lines)
  if (markdown.includes('<')) {
    markdown = markdown.replace(RE_ERROR_BLOCK_TAG, (_match, _tag: string, body: string) => {
      const escaped = escapeHtml(body.trim())
      return `\n\`\`\`error-block\n${escaped}\n\`\`\`\n`
    })
  }

  const lines = markdown.split('\n')
  const result: string[] = []
  let inCodeBlock = false
  let codeBlockLang = ''
  let codeBlockContent: string[] = []
  let inList = false
  let listItems: string[] = []
  let inTable = false
  let tableRows: string[][] = []
  let hasTableHeader = false
  let inTree = false
  let treeLines: string[] = []

  const flushList = () => {
    if (inList && listItems.length > 0) {
      result.push('<ul class="list-disc list-inside my-2 space-y-1">')
      result.push(...listItems)
      result.push('</ul>')
      listItems = []
      inList = false
    }
  }

  const flushTable = () => {
    if (inTable && tableRows.length > 0) {
      result.push('<div class="overflow-x-auto my-3">')
      result.push(
        '<table class="min-w-full border-collapse border border-gray-300 dark:border-gray-600">'
      )

      tableRows.forEach((row, rowIndex) => {
        if (rowIndex === 0 && hasTableHeader) {
          result.push('<thead class="bg-gray-100 dark:bg-gray-700">')
          result.push('<tr>')
          row.forEach((cell) => {
            result.push(
              `<th class="border border-gray-300 dark:border-gray-600 px-4 py-2 text-start font-semibold">${parseInline(cell, MARKDOWN_INLINE_OPTIONS)}</th>`
            )
          })
          result.push('</tr>')
          result.push('</thead>')
          result.push('<tbody>')
        } else {
          result.push('<tr class="even:bg-gray-50 dark:even:bg-gray-700/50">')
          row.forEach((cell) => {
            result.push(
              `<td class="border border-gray-300 dark:border-gray-600 px-4 py-2">${parseInline(cell, MARKDOWN_INLINE_OPTIONS)}</td>`
            )
          })
          result.push('</tr>')
        }
      })

      if (hasTableHeader) {
        result.push('</tbody>')
      }
      result.push('</table>')
      result.push('</div>')

      tableRows = []
      inTable = false
      hasTableHeader = false
    }
  }

  const flushTree = () => {
    if (inTree && treeLines.length > 0) {
      // Render tree structure with preserved whitespace
      const treeContent = treeLines.map((line) => escapeHtml(line)).join('\n')
      result.push(
        `<pre class="tree-structure my-2 font-mono text-sm whitespace-pre">${treeContent}</pre>`
      )
      treeLines = []
      inTree = false
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    // Code block handling
    if (line.startsWith('```')) {
      if (!inCodeBlock) {
        flushList()
        flushTable()
        inCodeBlock = true
        codeBlockLang = detectLanguage(line.slice(3).trim())
        codeBlockContent = []
      } else {
        const code = codeBlockContent.join('\n')
        if (codeBlockLang === 'error-block') {
          // Render as styled error indicator (content already escaped during pre-processing)
          result.push(
            `<div class="error-block-indicator my-3 flex items-start gap-2 rounded-lg px-4 py-3 text-sm">` +
              `<svg class="shrink-0 mt-0.5" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>` +
              `<span>${code}</span>` +
              `</div>`
          )
        } else if (codeBlockLang === 'process') {
          // Render tool execution results as a compact process card
          result.push(renderProcessCard(code))
        } else {
          const highlighted = highlightCode(code, codeBlockLang)
          result.push(
            `<div class="code-block my-3 rounded-lg overflow-hidden bg-gray-700">` +
              `<div class="code-header flex justify-between items-center px-4 py-2 bg-gray-700 text-gray-400 text-sm">` +
              `<span>${codeBlockLang || t('codeBlock.code', 'code')}</span>` +
              `<button class="copy-btn hover:text-white" data-code="${escapeHtml(code)}">${t('common.copy', 'Copy')}</button>` +
              `</div>` +
              `<pre class="p-4 overflow-x-auto"><code class="text-sm font-mono text-gray-100">${highlighted}</code></pre>` +
              `</div>`
          )
        }
        inCodeBlock = false
        codeBlockLang = ''

        // Check for consecutive fences: ``````lang → close + open
        const rest = line.slice(3)
        if (rest.startsWith('```')) {
          flushList()
          flushTable()
          inCodeBlock = true
          codeBlockLang = detectLanguage(rest.slice(3).trim())
          codeBlockContent = []
        }
      }
      continue
    }

    if (inCodeBlock) {
      codeBlockContent.push(line)
      continue
    }

    // Table handling - check for table separator first
    if (isTableSeparator(line)) {
      flushTree()
      // If we have a pending table row, this confirms it's a header
      if (inTable && tableRows.length === 1) {
        hasTableHeader = true
      }
      continue
    }

    // Table row handling
    if (isTableRow(line)) {
      flushList()
      flushTree()
      if (!inTable) {
        inTable = true
        tableRows = []
      }
      tableRows.push(parseTableRow(line))
      continue
    }

    // If we were in a table but this line is not a table row, flush the table
    if (inTable) {
      flushTable()
    }

    // Tree structure handling (lines with box-drawing characters)
    if (isTreeLine(line)) {
      flushList()
      flushTable()
      if (!inTree) {
        inTree = true
        treeLines = []
      }
      treeLines.push(line)
      continue
    }

    // If we were in a tree but this line is not a tree line, flush the tree
    if (inTree) {
      flushTree()
    }

    // HTML comments — skip silently (used for process-start/end markers)
    if (line.trim().startsWith('<!--') && line.trim().endsWith('-->')) {
      continue
    }

    // Empty line
    if (line.trim() === '') {
      flushList()
      flushTable()
      flushTree()
      result.push('<br>')
      continue
    }

    // Headers
    const headerMatch = line.match(/^(#{1,6})\s+(.+)$/)
    if (headerMatch && headerMatch[1] && headerMatch[2]) {
      flushList()
      flushTable()
      flushTree()
      const level = headerMatch[1].length
      const text = parseInline(headerMatch[2], MARKDOWN_INLINE_OPTIONS)
      const sizes = ['text-2xl', 'text-xl', 'text-lg', 'text-base', 'text-sm', 'text-sm']
      result.push(
        `<h${level} class="${sizes[level - 1] || 'text-sm'} font-bold my-2">${text}</h${level}>`
      )
      continue
    }

    // Horizontal rule
    if (/^[-*_]{3,}$/.test(line.trim())) {
      flushList()
      flushTable()
      flushTree()
      result.push('<hr class="my-4 border-gray-600">')
      continue
    }

    // Blockquote
    if (line.startsWith('>')) {
      flushList()
      flushTable()
      flushTree()
      const text = parseInline(line.slice(1).trim(), MARKDOWN_INLINE_OPTIONS)
      result.push(
        `<blockquote class="border-s-4 border-gray-500 ps-4 my-2 text-gray-400 italic">${text}</blockquote>`
      )
      continue
    }

    // Unordered list (with optional checkbox support)
    const ulMatch = line.match(/^[-*+]\s+(.+)$/)
    if (ulMatch && ulMatch[1]) {
      flushTable()
      flushTree()
      inList = true
      let itemContent = ulMatch[1]
      // GFM task list checkbox
      const cbMatch = itemContent.match(/^\[([ xX])\]\s+(.*)$/)
      if (cbMatch) {
        const checked = cbMatch[1] !== ' '
        const cbHtml = checked
          ? '<input type="checkbox" checked disabled class="me-1.5 accent-current opacity-60 pointer-events-none" />'
          : '<input type="checkbox" disabled class="me-1.5 opacity-60 pointer-events-none" />'
        const textClass = checked ? 'line-through opacity-50' : ''
        listItems.push(
          `<li class="list-none">${cbHtml}<span class="${textClass}">${parseInline(cbMatch[2] ?? '', MARKDOWN_INLINE_OPTIONS)}</span></li>`
        )
      } else {
        listItems.push(`<li>${parseInline(itemContent, MARKDOWN_INLINE_OPTIONS)}</li>`)
      }
      continue
    }

    // Ordered list
    const olMatch = line.match(/^\d+\.\s+(.+)$/)
    if (olMatch && olMatch[1]) {
      flushTable()
      flushTree()
      if (!inList) {
        inList = true
        listItems = []
      }
      listItems.push(`<li>${parseInline(olMatch[1], MARKDOWN_INLINE_OPTIONS)}</li>`)
      continue
    }

    // Regular paragraph
    flushList()
    flushTable()
    flushTree()
    result.push(`<p class="my-1">${parseInline(line, MARKDOWN_INLINE_OPTIONS)}</p>`)
  }

  flushList()
  flushTable()
  flushTree()

  // Flush unclosed code block (streaming)
  if (inCodeBlock && codeBlockContent.length > 0) {
    const code = codeBlockContent.join('\n')
    if (codeBlockLang === 'process') {
      // Render process card even during streaming (unclosed block)
      result.push(renderProcessCard(code))
    } else if (codeBlockLang === 'error-block') {
      result.push(
        `<div class="error-block-indicator my-3 flex items-start gap-2 rounded-lg px-4 py-3 text-sm">` +
          `<svg class="shrink-0 mt-0.5" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>` +
          `<span>${code}</span>` +
          `</div>`
      )
    } else {
      const highlighted = highlightCode(code, codeBlockLang)
      result.push(
        `<div class="code-block my-3 rounded-lg overflow-hidden bg-gray-700">` +
          `<div class="code-header flex justify-between items-center px-4 py-2 bg-gray-700 text-gray-400 text-sm">` +
          `<span>${codeBlockLang || 'code'}</span>` +
          `</div>` +
          `<pre class="p-4 overflow-x-auto"><code class="text-sm font-mono text-gray-100">${highlighted}</code></pre>` +
          `</div>`
      )
    }
  }

  return result.join('\n')
}

const MARKDOWN_HTML_CACHE = new Map<string, string>()
const MARKDOWN_HTML_CACHE_MAX = 400
const MARKDOWN_CACHEABLE_TEXT_MAX = 12000
const MARKDOWN_SCOPE_FAST_PATH_CACHE = new Map<string, MarkdownFastPathState>()
const MARKDOWN_SCOPE_FAST_PATH_CACHE_MAX = 64

function setMarkdownScopeFastPathState(scope: string, state: MarkdownFastPathState) {
  if (
    MARKDOWN_SCOPE_FAST_PATH_CACHE.size >= MARKDOWN_SCOPE_FAST_PATH_CACHE_MAX &&
    !MARKDOWN_SCOPE_FAST_PATH_CACHE.has(scope)
  ) {
    evictOldestMapEntry(MARKDOWN_SCOPE_FAST_PATH_CACHE)
  }
  MARKDOWN_SCOPE_FAST_PATH_CACHE.set(scope, state)
}

function clearMarkdownScopeFastPathState(scope: string) {
  MARKDOWN_SCOPE_FAST_PATH_CACHE.delete(scope)
}

function tryRenderAppendFastPath(
  markdown: string,
  scope: string,
  mode: MarkdownFastPathMode
): MarkdownFastPathState | null {
  const previous = MARKDOWN_SCOPE_FAST_PATH_CACHE.get(scope)
  if (!previous || previous.mode !== mode) {
    return null
  }

  if (markdown.length < previous.markdown.length || !markdown.startsWith(previous.markdown)) {
    return null
  }

  if (markdown === previous.markdown) {
    return previous
  }

  const delta = markdown.slice(previous.markdown.length)
  if (delta.length === 0) {
    return previous
  }

  if (mode === 'plain') {
    const paragraphEndTag = '</p>'
    if (!previous.html.endsWith(paragraphEndTag)) {
      return null
    }

    const html = `${previous.html.slice(0, -paragraphEndTag.length)}${escapeHtml(delta)}${paragraphEndTag}`
    return {
      mode,
      markdown,
      html,
    }
  }

  const previousMultiline = previous as MultilinePlainFastPathState
  const previousLines = previousMultiline.lines
  const previousRenderedLines = previousMultiline.renderedLines
  if (previousLines.length === 0 || previousRenderedLines.length === 0) {
    return null
  }

  const nextLines = previousLines.slice(0, Math.max(previousLines.length - 1, 0))
  const lastLineBase = previousLines[previousLines.length - 1] || ''
  const deltaLines = delta.split('\n')
  nextLines.push(`${lastLineBase}${deltaLines[0] || ''}`)
  for (let index = 1; index < deltaLines.length; index++) {
    nextLines.push(deltaLines[index] || '')
  }

  // Lines before the previous last line are stable for append-only updates.
  const stableRenderedCount = Math.max(previousRenderedLines.length - 1, 0)
  const nextRenderedLines =
    stableRenderedCount > 0 ? previousRenderedLines.slice(0, stableRenderedCount) : []

  for (let index = stableRenderedCount; index < nextLines.length; index++) {
    nextRenderedLines.push(renderPlainTextLine(nextLines[index] || ''))
  }

  return {
    mode,
    markdown,
    html: nextRenderedLines.join('\n'),
    lines: nextLines,
    renderedLines: nextRenderedLines,
  }
}

function buildMarkdownFastPathState(
  markdown: string,
  mode: MarkdownFastPathMode
): MarkdownFastPathState {
  if (mode === 'plain') {
    return {
      mode,
      markdown,
      html: renderPlainTextParagraph(markdown),
    }
  }

  const rendered = renderMultilinePlainTextWithMeta(markdown)
  return {
    mode,
    markdown,
    html: rendered.html,
    lines: rendered.lines,
    renderedLines: rendered.renderedLines,
  }
}

// Shared markdown render cache for high-frequency UI paths.
// We cache plain markdown immediately; code-fence content waits for hljs readiness.
export function renderMarkdownCached(markdown: string, scope = 'default'): string {
  const containsCodeFence = markdown.includes('```')
  if (markdown.length > MARKDOWN_CACHEABLE_TEXT_MAX) {
    clearMarkdownScopeFastPathState(scope)
    return renderMarkdown(markdown)
  }

  // If code highlighting is required but hljs is not ready yet, bypass cache so
  // the next render can pick up highlighted HTML automatically.
  if (containsCodeFence && !hljsReady) {
    clearMarkdownScopeFastPathState(scope)
    return renderMarkdown(markdown)
  }

  const fastPathMode = getMarkdownFastPathMode(markdown)
  if (fastPathMode) {
    const appendFastPathState = tryRenderAppendFastPath(markdown, scope, fastPathMode)
    if (appendFastPathState) {
      setMarkdownScopeFastPathState(scope, appendFastPathState)
      return appendFastPathState.html
    }
  }

  const key = `${scope}:${markdown}`
  const cached = MARKDOWN_HTML_CACHE.get(key)
  if (cached !== undefined) return cached

  let fastPathState: MarkdownFastPathState | null = null
  if (fastPathMode) {
    fastPathState = buildMarkdownFastPathState(markdown, fastPathMode)
  }

  const html = fastPathState?.html ?? renderMarkdown(markdown)
  if (MARKDOWN_HTML_CACHE.size >= MARKDOWN_HTML_CACHE_MAX) {
    evictOldestMapEntry(MARKDOWN_HTML_CACHE)
  }
  MARKDOWN_HTML_CACHE.set(key, html)
  if (fastPathState) {
    setMarkdownScopeFastPathState(scope, fastPathState)
  } else {
    clearMarkdownScopeFastPathState(scope)
  }
  return html
}

// Copy code to clipboard
export function copyCodeToClipboard(code: string): Promise<void> {
  return navigator.clipboard.writeText(code)
}
