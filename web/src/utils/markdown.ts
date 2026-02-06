// Simple markdown renderer with code syntax highlighting support
// Uses highlight.js for syntax highlighting (common languages only)

import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import typescript from 'highlight.js/lib/languages/typescript'
import python from 'highlight.js/lib/languages/python'
import go from 'highlight.js/lib/languages/go'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import xml from 'highlight.js/lib/languages/xml'
import css from 'highlight.js/lib/languages/css'
import sql from 'highlight.js/lib/languages/sql'
import rust from 'highlight.js/lib/languages/rust'
import java from 'highlight.js/lib/languages/java'
import cpp from 'highlight.js/lib/languages/cpp'
import markdown from 'highlight.js/lib/languages/markdown'
import php from 'highlight.js/lib/languages/php'
import ruby from 'highlight.js/lib/languages/ruby'
import swift from 'highlight.js/lib/languages/swift'
import kotlin from 'highlight.js/lib/languages/kotlin'
import csharp from 'highlight.js/lib/languages/csharp'
import scala from 'highlight.js/lib/languages/scala'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import nginx from 'highlight.js/lib/languages/nginx'
import ini from 'highlight.js/lib/languages/ini'
import diff from 'highlight.js/lib/languages/diff'
import plaintext from 'highlight.js/lib/languages/plaintext'

// Register languages
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('js', javascript)
hljs.registerLanguage('jsx', javascript)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('ts', typescript)
hljs.registerLanguage('tsx', typescript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('py', python)
hljs.registerLanguage('go', go)
hljs.registerLanguage('golang', go)
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('sh', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('zsh', bash)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('yml', yaml)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('html', xml)
hljs.registerLanguage('vue', xml)
hljs.registerLanguage('svg', xml)
hljs.registerLanguage('css', css)
hljs.registerLanguage('scss', css)
hljs.registerLanguage('less', css)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('rust', rust)
hljs.registerLanguage('rs', rust)
hljs.registerLanguage('java', java)
hljs.registerLanguage('cpp', cpp)
hljs.registerLanguage('c', cpp)
hljs.registerLanguage('cc', cpp)
hljs.registerLanguage('h', cpp)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('md', markdown)
hljs.registerLanguage('php', php)
hljs.registerLanguage('ruby', ruby)
hljs.registerLanguage('rb', ruby)
hljs.registerLanguage('swift', swift)
hljs.registerLanguage('kotlin', kotlin)
hljs.registerLanguage('kt', kotlin)
hljs.registerLanguage('csharp', csharp)
hljs.registerLanguage('cs', csharp)
hljs.registerLanguage('scala', scala)
hljs.registerLanguage('dockerfile', dockerfile)
hljs.registerLanguage('docker', dockerfile)
hljs.registerLanguage('nginx', nginx)
hljs.registerLanguage('ini', ini)
hljs.registerLanguage('toml', ini)
hljs.registerLanguage('conf', ini)
hljs.registerLanguage('env', ini)
hljs.registerLanguage('diff', diff)
hljs.registerLanguage('patch', diff)
hljs.registerLanguage('plaintext', plaintext)
hljs.registerLanguage('text', plaintext)
hljs.registerLanguage('txt', plaintext)

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
