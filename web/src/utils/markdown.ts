// Simple markdown renderer with code syntax highlighting support
// Uses highlight.js for syntax highlighting (lazy-loaded on first use)

import hljs from 'highlight.js/lib/core'

let hljsReady = false
let hljsLoading: Promise<void> | null = null

/** Preload highlight.js languages. Call this when a component that needs highlighting mounts. */
export function preloadHljs(): Promise<void> {
  if (hljsReady) return Promise.resolve()
  if (hljsLoading) return hljsLoading
  hljsLoading = (async () => {
    const [
      javascript, typescript, python, go, bash, json, yaml, xml, css, sql,
      rust, java, cpp, markdown, php, ruby, swift, kotlin, csharp, scala,
      dockerfile, nginx, ini, diff, plaintext,
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
export function parseInline(text: string): string {
  let result = escapeHtml(text)

  // Bold: **text** or __text__
  result = result.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  result = result.replace(/__(.+?)__/g, '<strong>$1</strong>')

  // Italic: *text* or _text_
  result = result.replace(/\*(.+?)\*/g, '<em>$1</em>')
  result = result.replace(/_(.+?)_/g, '<em>$1</em>')

  // Strikethrough: ~~text~~
  result = result.replace(/~~(.+?)~~/g, '<del>$1</del>')

  // Inline code: `code`
  result = result.replace(/`([^`]+)`/g, '<code class="inline-code">$1</code>')

  // Links: [text](url)
  result = result.replace(
    /\[([^\]]+)\]\(([^)]+)\)/g,
    '<a href="$2" target="_blank" rel="noopener noreferrer" class="text-gray-900 dark:text-white hover:underline">$1</a>'
  )

  return result
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
  return withoutEndPipe.split('|').map(cell => cell.trim())
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
  const nonEmptyCells = parts.filter(p => p.trim().length > 0)
  return nonEmptyCells.length >= 2
}

// Check if a line is part of a tree structure (uses box-drawing characters)
function isTreeLine(line: string): boolean {
  // Box-drawing characters commonly used in tree structures
  const boxDrawingChars = /[│├└┌┐┘┬┴┼─]/
  return boxDrawingChars.test(line)
}

// Main render function
export function renderMarkdown(markdown: string, _options: RenderOptions = {}): string {
  // Pre-process: convert XML-like error/status tags into styled blocks before line splitting
  // Matches <tool_use_error>...</tool_use_error> and similar tags (may span multiple lines)
  markdown = markdown.replace(
    /<(tool_use_error|error|system-error)>([\s\S]*?)<\/\1>/g,
    (_match, _tag: string, body: string) => {
      const escaped = escapeHtml(body.trim())
      return `\n\`\`\`error-block\n${escaped}\n\`\`\`\n`
    }
  )

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
      result.push('<table class="min-w-full border-collapse border border-gray-300 dark:border-gray-600">')

      tableRows.forEach((row, rowIndex) => {
        if (rowIndex === 0 && hasTableHeader) {
          result.push('<thead class="bg-gray-100 dark:bg-gray-700">')
          result.push('<tr>')
          row.forEach(cell => {
            result.push(`<th class="border border-gray-300 dark:border-gray-600 px-4 py-2 text-left font-semibold">${parseInline(cell)}</th>`)
          })
          result.push('</tr>')
          result.push('</thead>')
          result.push('<tbody>')
        } else {
          result.push('<tr class="even:bg-gray-50 dark:even:bg-gray-700/50">')
          row.forEach(cell => {
            result.push(`<td class="border border-gray-300 dark:border-gray-600 px-4 py-2">${parseInline(cell)}</td>`)
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
      const treeContent = treeLines.map(line => escapeHtml(line)).join('\n')
      result.push(`<pre class="tree-structure my-2 font-mono text-sm whitespace-pre">${treeContent}</pre>`)
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
        } else {
        const highlighted = highlightCode(code, codeBlockLang)
        result.push(
          `<div class="code-block my-3 rounded-lg overflow-hidden bg-gray-700">` +
            `<div class="code-header flex justify-between items-center px-4 py-2 bg-gray-700 text-gray-400 text-sm">` +
            `<span>${codeBlockLang || 'code'}</span>` +
            `<button class="copy-btn hover:text-white" data-code="${escapeHtml(code)}">Copy</button>` +
            `</div>` +
            `<pre class="p-4 overflow-x-auto"><code class="text-sm font-mono text-gray-100">${highlighted}</code></pre>` +
            `</div>`
        )
        }
        inCodeBlock = false
        codeBlockLang = ''
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
      const text = parseInline(headerMatch[2])
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
      const text = parseInline(line.slice(1).trim())
      result.push(
        `<blockquote class="border-l-4 border-gray-500 pl-4 my-2 text-gray-400 italic">${text}</blockquote>`
      )
      continue
    }

    // Unordered list
    const ulMatch = line.match(/^[-*+]\s+(.+)$/)
    if (ulMatch && ulMatch[1]) {
      flushTable()
      flushTree()
      inList = true
      listItems.push(`<li>${parseInline(ulMatch[1])}</li>`)
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
      listItems.push(`<li>${parseInline(olMatch[1])}</li>`)
      continue
    }

    // Regular paragraph
    flushList()
    flushTable()
    flushTree()
    result.push(`<p class="my-1">${parseInline(line)}</p>`)
  }

  flushList()
  flushTable()
  flushTree()

  return result.join('\n')
}

// Copy code to clipboard
export function copyCodeToClipboard(code: string): Promise<void> {
  return navigator.clipboard.writeText(code)
}
