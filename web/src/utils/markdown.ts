// Simple markdown renderer with code syntax highlighting support
// Uses a lightweight approach without heavy dependencies

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
  return text.replace(/[&<>"']/g, (char) => map[char])
}

// Parse inline markdown elements
function parseInline(text: string): string {
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
    '<a href="$2" target="_blank" rel="noopener noreferrer" class="text-blue-500 hover:underline">$1</a>'
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

// Simple syntax highlighting for common languages
function highlightCode(code: string, language: string): string {
  const escaped = escapeHtml(code)

  // Keywords for common languages
  const keywords: Record<string, string[]> = {
    javascript: [
      'const',
      'let',
      'var',
      'function',
      'return',
      'if',
      'else',
      'for',
      'while',
      'class',
      'import',
      'export',
      'from',
      'async',
      'await',
      'try',
      'catch',
      'throw',
      'new',
      'this',
      'true',
      'false',
      'null',
      'undefined',
      'typeof',
      'instanceof',
      'delete',
      'void',
      'break',
      'continue',
      'switch',
      'case',
      'default',
      'finally',
      'yield',
      'static',
      'get',
      'set',
      'of',
    ],
    typescript: [
      'const',
      'let',
      'var',
      'function',
      'return',
      'if',
      'else',
      'for',
      'while',
      'class',
      'import',
      'export',
      'from',
      'async',
      'await',
      'try',
      'catch',
      'throw',
      'new',
      'this',
      'true',
      'false',
      'null',
      'undefined',
      'interface',
      'type',
      'enum',
      'implements',
      'extends',
      'public',
      'private',
      'protected',
      'readonly',
      'abstract',
      'as',
      'is',
      'keyof',
      'typeof',
      'infer',
      'never',
      'unknown',
      'any',
      'void',
      'namespace',
      'module',
      'declare',
      'static',
    ],
    python: [
      'def',
      'class',
      'import',
      'from',
      'return',
      'if',
      'elif',
      'else',
      'for',
      'while',
      'try',
      'except',
      'finally',
      'with',
      'as',
      'True',
      'False',
      'None',
      'and',
      'or',
      'not',
      'in',
      'is',
      'lambda',
      'yield',
      'async',
      'await',
      'pass',
      'break',
      'continue',
      'raise',
      'assert',
      'global',
      'nonlocal',
      'del',
    ],
    go: [
      'func',
      'package',
      'import',
      'return',
      'if',
      'else',
      'for',
      'range',
      'switch',
      'case',
      'default',
      'struct',
      'interface',
      'type',
      'var',
      'const',
      'true',
      'false',
      'nil',
      'go',
      'defer',
      'chan',
      'select',
      'map',
      'make',
      'new',
      'append',
      'len',
      'cap',
      'copy',
      'delete',
      'panic',
      'recover',
      'break',
      'continue',
      'fallthrough',
      'goto',
    ],
    bash: [
      'if',
      'then',
      'else',
      'elif',
      'fi',
      'for',
      'while',
      'do',
      'done',
      'case',
      'esac',
      'function',
      'return',
      'export',
      'local',
      'echo',
      'exit',
      'read',
      'source',
      'alias',
      'unset',
      'shift',
      'trap',
      'eval',
      'exec',
      'set',
      'declare',
      'readonly',
      'typeset',
    ],
    rust: [
      'fn',
      'let',
      'mut',
      'const',
      'static',
      'struct',
      'enum',
      'impl',
      'trait',
      'type',
      'where',
      'for',
      'loop',
      'while',
      'if',
      'else',
      'match',
      'return',
      'break',
      'continue',
      'pub',
      'mod',
      'use',
      'crate',
      'self',
      'super',
      'as',
      'in',
      'ref',
      'move',
      'async',
      'await',
      'dyn',
      'unsafe',
      'extern',
      'true',
      'false',
      'Some',
      'None',
      'Ok',
      'Err',
    ],
    java: [
      'public',
      'private',
      'protected',
      'class',
      'interface',
      'extends',
      'implements',
      'static',
      'final',
      'abstract',
      'new',
      'return',
      'if',
      'else',
      'for',
      'while',
      'do',
      'switch',
      'case',
      'default',
      'break',
      'continue',
      'try',
      'catch',
      'finally',
      'throw',
      'throws',
      'import',
      'package',
      'void',
      'int',
      'long',
      'double',
      'float',
      'boolean',
      'char',
      'byte',
      'short',
      'true',
      'false',
      'null',
      'this',
      'super',
      'instanceof',
      'enum',
      'synchronized',
      'volatile',
      'transient',
    ],
    cpp: [
      'int',
      'long',
      'short',
      'float',
      'double',
      'char',
      'bool',
      'void',
      'auto',
      'const',
      'static',
      'extern',
      'register',
      'volatile',
      'inline',
      'virtual',
      'explicit',
      'class',
      'struct',
      'union',
      'enum',
      'namespace',
      'using',
      'template',
      'typename',
      'public',
      'private',
      'protected',
      'friend',
      'new',
      'delete',
      'return',
      'if',
      'else',
      'for',
      'while',
      'do',
      'switch',
      'case',
      'default',
      'break',
      'continue',
      'try',
      'catch',
      'throw',
      'true',
      'false',
      'nullptr',
      'this',
      'sizeof',
      'typedef',
      'constexpr',
      'noexcept',
      'override',
      'final',
    ],
    sql: [
      'SELECT',
      'FROM',
      'WHERE',
      'AND',
      'OR',
      'NOT',
      'IN',
      'LIKE',
      'BETWEEN',
      'IS',
      'NULL',
      'ORDER',
      'BY',
      'ASC',
      'DESC',
      'LIMIT',
      'OFFSET',
      'JOIN',
      'LEFT',
      'RIGHT',
      'INNER',
      'OUTER',
      'ON',
      'GROUP',
      'HAVING',
      'UNION',
      'INSERT',
      'INTO',
      'VALUES',
      'UPDATE',
      'SET',
      'DELETE',
      'CREATE',
      'TABLE',
      'INDEX',
      'VIEW',
      'DROP',
      'ALTER',
      'ADD',
      'PRIMARY',
      'KEY',
      'FOREIGN',
      'REFERENCES',
      'UNIQUE',
      'DEFAULT',
      'CHECK',
      'CONSTRAINT',
      'CASCADE',
      'AS',
      'DISTINCT',
      'COUNT',
      'SUM',
      'AVG',
      'MIN',
      'MAX',
      'CASE',
      'WHEN',
      'THEN',
      'ELSE',
      'END',
      'TRUE',
      'FALSE',
    ],
    json: [],
    yaml: [],
    html: [],
    css: [
      'important',
      'inherit',
      'initial',
      'unset',
      'none',
      'auto',
      'block',
      'inline',
      'flex',
      'grid',
      'absolute',
      'relative',
      'fixed',
      'sticky',
      'static',
      'hidden',
      'visible',
      'scroll',
      'solid',
      'dashed',
      'dotted',
      'transparent',
    ],
  }

  const langKeywords = keywords[language] || []
  if (langKeywords.length === 0) {
    return escaped
  }

  let result = escaped

  // Highlight strings
  result = result.replace(
    /(["'`])(?:(?!\1)[^\\]|\\.)*\1/g,
    '<span class="text-green-400">$&</span>'
  )

  // Highlight comments
  result = result.replace(/(\/\/.*$|#.*$)/gm, '<span class="text-gray-500">$&</span>')
  result = result.replace(/(\/\*[\s\S]*?\*\/)/g, '<span class="text-gray-500">$&</span>')

  // Highlight keywords
  const keywordPattern = new RegExp(`\\b(${langKeywords.join('|')})\\b`, 'g')
  result = result.replace(keywordPattern, '<span class="text-purple-400">$1</span>')

  // Highlight numbers
  result = result.replace(/\b(\d+\.?\d*)\b/g, '<span class="text-orange-400">$1</span>')

  return result
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

  const flushList = () => {
    if (inList && listItems.length > 0) {
      result.push('<ul class="list-disc list-inside my-2 space-y-1">')
      result.push(...listItems)
      result.push('</ul>')
      listItems = []
      inList = false
    }
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    // Code block handling
    if (line.startsWith('```')) {
      if (!inCodeBlock) {
        flushList()
        inCodeBlock = true
        codeBlockLang = detectLanguage(line.slice(3).trim())
        codeBlockContent = []
      } else {
        const code = codeBlockContent.join('\n')
        const highlighted = highlightCode(code, codeBlockLang)
        result.push(
          `<div class="code-block my-3 rounded-lg overflow-hidden bg-gray-900">` +
            `<div class="code-header flex justify-between items-center px-4 py-2 bg-gray-800 text-gray-400 text-sm">` +
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

    // Empty line
    if (line.trim() === '') {
      flushList()
      result.push('<br>')
      continue
    }

    // Headers
    const headerMatch = line.match(/^(#{1,6})\s+(.+)$/)
    if (headerMatch) {
      flushList()
      const level = headerMatch[1].length
      const text = parseInline(headerMatch[2])
      const sizes = ['text-2xl', 'text-xl', 'text-lg', 'text-base', 'text-sm', 'text-sm']
      result.push(
        `<h${level} class="${sizes[level - 1]} font-bold my-2">${text}</h${level}>`
      )
      continue
    }

    // Horizontal rule
    if (/^[-*_]{3,}$/.test(line.trim())) {
      flushList()
      result.push('<hr class="my-4 border-gray-600">')
      continue
    }

    // Blockquote
    if (line.startsWith('>')) {
      flushList()
      const text = parseInline(line.slice(1).trim())
      result.push(
        `<blockquote class="border-l-4 border-gray-500 pl-4 my-2 text-gray-400 italic">${text}</blockquote>`
      )
      continue
    }

    // Unordered list
    const ulMatch = line.match(/^[-*+]\s+(.+)$/)
    if (ulMatch) {
      inList = true
      listItems.push(`<li>${parseInline(ulMatch[1])}</li>`)
      continue
    }

    // Ordered list
    const olMatch = line.match(/^\d+\.\s+(.+)$/)
    if (olMatch) {
      if (!inList) {
        inList = true
        listItems = []
      }
      listItems.push(`<li>${parseInline(olMatch[1])}</li>`)
      continue
    }

    // Regular paragraph
    flushList()
    result.push(`<p class="my-1">${parseInline(line)}</p>`)
  }

  flushList()

  return result.join('\n')
}

// Copy code to clipboard
export function copyCodeToClipboard(code: string): Promise<void> {
  return navigator.clipboard.writeText(code)
}
