/**
 * Web Worker for parsing typeless content
 * Offloads heavy parsing work from the main thread
 */

import type { TypelessCard, ParsedContent } from '@/types/typeless'

// Message types
interface ParseRequest {
  type: 'parse'
  id: string
  content: string
}

interface ParseResponse {
  type: 'parsed'
  id: string
  result: ParsedContent
}

type WorkerMessage = ParseRequest
type WorkerResponse = ParseResponse

// Import parsing logic (will be bundled into worker)
// Note: We need to duplicate some logic here since workers can't import from main bundle

const TYPELESS_MARKER_START = '```typeless'
const TYPELESS_MARKER_END = '```'

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

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function parseTableRow(line: string): string[] {
  const trimmed = line.trim()
  const withoutPipes = trimmed.startsWith('|') ? trimmed.slice(1) : trimmed
  const withoutEndPipe = withoutPipes.endsWith('|') ? withoutPipes.slice(0, -1) : withoutPipes
  return withoutEndPipe.split('|').map(cell => cell.trim())
}

function isTableSeparator(line: string): boolean {
  const trimmed = line.trim()
  if (!trimmed.includes('|')) return false
  const content = trimmed.replace(/\|/g, '').trim()
  return /^[\s:-]+$/.test(content) && content.includes('-')
}

function isTableRow(line: string): boolean {
  const trimmed = line.trim()
  return trimmed.includes('|') && !isTableSeparator(trimmed)
}

// Simplified parsing for worker (main parsing logic)
function parseTypelessContent(content: string): ParsedContent {
  const cards: TypelessCard[] = []
  let text = content
  const cardIndex = { value: 0 }

  // Find all typeless blocks first
  const regex = new RegExp(
    `${escapeRegex(TYPELESS_MARKER_START)}\\s*([\\s\\S]*?)\\s*${escapeRegex(TYPELESS_MARKER_END)}`,
    'g'
  )

  let match
  const replacements: { start: number; end: number; placeholder: string }[] = []

  while ((match = regex.exec(content)) !== null) {
    try {
      const jsonStr = match[1]?.trim()
      if (!jsonStr) continue
      const card = JSON.parse(jsonStr) as TypelessCard

      if (card && typeof card.type === 'string') {
        if (!card.id) {
          card.id = `card-${cardIndex.value++}`
        }
        cards.push(card)

        replacements.push({
          start: match.index,
          end: match.index + match[0].length,
          placeholder: `[[TYPELESS_CARD:${card.id}]]`,
        })
      }
    } catch {
      // Invalid JSON, leave as-is
    }
  }

  // Replace card blocks with placeholders (in reverse order)
  for (let i = replacements.length - 1; i >= 0; i--) {
    const replacement = replacements[i]
    if (replacement) {
      const { start, end, placeholder } = replacement
      text = text.slice(0, start) + placeholder + text.slice(end)
    }
  }

  // Parse markdown elements
  text = parseMarkdownCodeBlocks(text, cards, cardIndex)
  text = parseMarkdownTables(text, cards, cardIndex)
  text = parseMarkdownLists(text, cards, cardIndex)

  return { text, cards }
}

function parseMarkdownCodeBlocks(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let inCodeBlock = false
  let codeBlockLang = ''
  let codeBlockContent: string[] = []

  for (const line of lines) {
    if (line === undefined) continue

    if (line.startsWith('```') && !line.startsWith('```typeless')) {
      if (!inCodeBlock) {
        inCodeBlock = true
        const lang = line.slice(3).trim()
        codeBlockLang = languageAliases[lang.toLowerCase()] || lang
        codeBlockContent = []
      } else {
        const card = {
          type: 'code' as const,
          id: `md-code-${cardIndex.value++}`,
          code: codeBlockContent.join('\n'),
          language: codeBlockLang || undefined,
          showLineNumbers: true,
        }
        cards.push(card)
        result.push(`[[TYPELESS_CARD:${card.id}]]`)
        inCodeBlock = false
        codeBlockLang = ''
      }
      continue
    }

    if (inCodeBlock) {
      codeBlockContent.push(line)
    } else {
      result.push(line)
    }
  }

  if (inCodeBlock && codeBlockContent.length > 0) {
    result.push('```' + codeBlockLang)
    result.push(...codeBlockContent)
  }

  return result.join('\n')
}

function parseMarkdownTables(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let inTable = false
  let tableRows: string[][] = []
  let hasTableHeader = false

  const flushTable = () => {
    if (inTable && tableRows.length > 0) {
      const firstRow = tableRows[0]
      const card = {
        type: 'table' as const,
        id: `md-table-${cardIndex.value++}`,
        headers: hasTableHeader && firstRow ? firstRow : [],
        rows: hasTableHeader ? tableRows.slice(1) : tableRows,
        striped: true,
      }
      cards.push(card)
      result.push(`[[TYPELESS_CARD:${card.id}]]`)
    }
    tableRows = []
    inTable = false
    hasTableHeader = false
  }

  for (const line of lines) {
    if (line === undefined) continue

    if (isTableSeparator(line)) {
      if (inTable && tableRows.length === 1) {
        hasTableHeader = true
      }
      continue
    }

    if (isTableRow(line)) {
      if (!inTable) {
        inTable = true
        tableRows = []
      }
      tableRows.push(parseTableRow(line))
      continue
    }

    if (inTable) {
      flushTable()
    }

    result.push(line)
  }

  flushTable()
  return result.join('\n')
}

function parseMarkdownLists(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let inList = false
  let listItems: Array<{ content: string; subItems?: Array<{ content: string }> }> = []
  let isOrdered = false

  const flushList = () => {
    if (inList && listItems.length > 0) {
      const card = {
        type: 'list' as const,
        id: `md-list-${cardIndex.value++}`,
        items: listItems,
        ordered: isOrdered,
      }
      cards.push(card)
      result.push(`[[TYPELESS_CARD:${card.id}]]`)
    }
    listItems = []
    inList = false
    isOrdered = false
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    const ulMatch = line.match(/^(\s*)[-*+]\s+(.+)$/)
    const olMatch = line.match(/^(\s*)\d+\.\s+(.+)$/)

    if (ulMatch || olMatch) {
      const match = ulMatch || olMatch
      if (!match) continue
      const itemContent = match[2] ?? ''
      const itemIsOrdered = !!olMatch

      if (!inList) {
        inList = true
        isOrdered = itemIsOrdered
      }

      listItems.push({ content: itemContent })
      continue
    }

    if (line.trim() === '' && inList) {
      const nextLine = lines[i + 1]
      if (nextLine && (nextLine.match(/^(\s*)[-*+]\s+/) || nextLine.match(/^(\s*)\d+\.\s+/))) {
        result.push(line)
        continue
      }
      flushList()
    }

    if (inList && !line.match(/^(\s*)[-*+]\s+/) && !line.match(/^(\s*)\d+\.\s+/)) {
      flushList()
    }

    result.push(line)
  }

  flushList()
  return result.join('\n')
}

// Worker message handler
self.onmessage = (event: MessageEvent<WorkerMessage>) => {
  const message = event.data

  if (message.type === 'parse') {
    const result = parseTypelessContent(message.content)
    const response: WorkerResponse = {
      type: 'parsed',
      id: message.id,
      result,
    }
    self.postMessage(response)
  }
}

export {}
