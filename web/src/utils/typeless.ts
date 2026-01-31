import type {
  TypelessCard,
  TypelessCardTable,
  TypelessCardCode,
  TypelessCardList,
  TypelessCardGallery,
  TypelessCardLink,
  TypelessCardFile,
  GalleryImage,
  ListItem,
  ParsedContent,
} from '@/types/typeless'
import {
  TYPELESS_MARKER_START,
  TYPELESS_MARKER_END,
} from '@/types/typeless'

// File extensions for different categories
const imageExtensions = ['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp', 'ico']
const documentExtensions = ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'csv', 'json', 'xml', 'md']

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
}

const incrementalStates = new Map<string, IncrementalParseState>()

/**
 * Parse content incrementally (for streaming messages)
 * Only re-parses the new portion of content when possible
 */
export function parseTypelessContentIncremental(
  content: string,
  messageId: string
): ParsedContent {
  const state = incrementalStates.get(messageId)

  // If no previous state or content doesn't start with previous content, do full parse
  if (!state || !content.startsWith(state.lastContent)) {
    const result = parseTypelessContentInternal(content, 0)
    incrementalStates.set(messageId, {
      lastContent: content,
      lastResult: result,
      lastCardIndex: result.cards.length,
    })
    return result
  }

  // Content is an extension of previous content
  const newContent = content.slice(state.lastContent.length)

  // If new content is small, just return cached result (debounce)
  if (newContent.length < 10) {
    return state.lastResult
  }

  // Check if new content might contain new cards
  const mightHaveNewCards =
    newContent.includes('```') ||
    newContent.includes('|') ||
    newContent.includes('- ') ||
    newContent.includes('* ') ||
    newContent.includes('1. ') ||
    newContent.includes('![') ||
    newContent.includes('http')

  if (!mightHaveNewCards) {
    // Just update text, no new cards
    const updatedResult: ParsedContent = {
      text: state.lastResult.text + newContent,
      cards: state.lastResult.cards,
    }
    incrementalStates.set(messageId, {
      lastContent: content,
      lastResult: updatedResult,
      lastCardIndex: state.lastCardIndex,
    })
    return updatedResult
  }

  // Need to re-parse (new cards might be present)
  const result = parseTypelessContentInternal(content, 0)
  incrementalStates.set(messageId, {
    lastContent: content,
    lastResult: result,
    lastCardIndex: result.cards.length,
  })
  return result
}

/**
 * Clear incremental parse state for a message
 */
export function clearIncrementalState(messageId: string): void {
  incrementalStates.delete(messageId)
}

/**
 * Clear all incremental parse states
 */
export function clearAllIncrementalStates(): void {
  incrementalStates.clear()
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
 * Internal parsing function (no caching)
 */
function parseTypelessContentInternal(content: string, startCardIndex: number): ParsedContent {
  const cards: TypelessCard[] = []
  let text = content
  const cardIndex = { value: startCardIndex }

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

      // Validate card has required type field
      if (card && typeof card.type === 'string') {
        // Assign ID if not present
        if (!card.id) {
          card.id = `card-${cardIndex.value++}`
        }
        cards.push(card)

        // Mark for replacement with placeholder
        replacements.push({
          start: match.index,
          end: match.index + match[0].length,
          placeholder: `[[TYPELESS_CARD:${card.id}]]`,
        })
      }
    } catch {
      // Invalid JSON, leave as-is
      console.warn('Failed to parse typeless card:', match[1])
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

  // Parse markdown elements and convert to cards
  // Order matters: code blocks first (to avoid parsing code content), then tables, then lists, then images, then files, then links
  text = parseMarkdownCodeBlocks(text, cards, cardIndex)
  text = parseMarkdownTables(text, cards, cardIndex)
  text = parseMarkdownLists(text, cards, cardIndex)
  text = parseMarkdownImages(text, cards, cardIndex)
  text = parseFilePaths(text, cards, cardIndex)
  text = parseMarkdownLinks(text, cards, cardIndex)

  return { text, cards }
}

/**
 * Check if content contains any typeless cards or markdown elements that will be converted.
 */
export function hasTypelessCards(content: string): boolean {
  // Check for explicit typeless blocks
  if (content.includes(TYPELESS_MARKER_START)) {
    return true
  }

  // Check for markdown code blocks (but not typeless blocks)
  if (/```(?!typeless)[a-z]*\n[\s\S]*?```/i.test(content)) {
    return true
  }

  // Check for markdown images (standalone on a line)
  if (/^\s*!\[[^\]]*\]\([^)]+\)\s*$/m.test(content)) {
    return true
  }

  // Check for standalone URLs
  if (/^\s*https?:\/\/[^\s]+\s*$/m.test(content)) {
    return true
  }

  // Check for file paths (Windows or Unix with extension)
  if (/^\s*([a-zA-Z]:\\[^\s]+\.[a-zA-Z0-9]+|\/[^\s]+\.[a-zA-Z0-9]+)\s*$/m.test(content)) {
    return true
  }

  // Check for markdown tables (at least 2 rows with pipes)
  const lines = content.split('\n')
  let tableRowCount = 0
  for (const line of lines) {
    if (line.includes('|') && !line.match(/^[\s|:-]+$/)) {
      tableRowCount++
      if (tableRowCount >= 2) {
        return true
      }
    } else if (tableRowCount > 0 && line.trim() !== '' && !line.match(/^[\s|:-]+$/)) {
      tableRowCount = 0
    }
  }

  // Check for markdown lists (at least 2 items)
  let listItemCount = 0
  for (const line of lines) {
    if (line.match(/^(\s*)[-*+]\s+/) || line.match(/^(\s*)\d+\.\s+/)) {
      listItemCount++
      if (listItemCount >= 2) {
        return true
      }
    } else if (line.trim() !== '') {
      listItemCount = 0
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

  return segments
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Parse a markdown table row into cells
 */
function parseTableRow(line: string): string[] {
  const trimmed = line.trim()
  const withoutPipes = trimmed.startsWith('|') ? trimmed.slice(1) : trimmed
  const withoutEndPipe = withoutPipes.endsWith('|') ? withoutPipes.slice(0, -1) : withoutPipes
  return withoutEndPipe.split('|').map(cell => cell.trim())
}

/**
 * Check if a line is a table separator (e.g., |---|---|)
 */
function isTableSeparator(line: string): boolean {
  const trimmed = line.trim()
  if (!trimmed.includes('|')) return false
  const content = trimmed.replace(/\|/g, '').trim()
  return /^[\s:-]+$/.test(content) && content.includes('-')
}

/**
 * Check if a line looks like a table row
 */
function isTableRow(line: string): boolean {
  const trimmed = line.trim()
  return trimmed.includes('|') && !isTableSeparator(trimmed)
}

/**
 * Parse markdown tables and convert to CardTable
 */
function parseMarkdownTables(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let inTable = false
  let tableRows: string[][] = []
  let hasTableHeader = false

  const flushTable = () => {
    if (inTable && tableRows.length > 0) {
      const firstRow = tableRows[0]
      const card: TypelessCardTable = {
        type: 'table',
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

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
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

/**
 * Parse markdown code blocks and convert to CardCode
 */
function parseMarkdownCodeBlocks(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let inCodeBlock = false
  let codeBlockLang = ''
  let codeBlockContent: string[] = []

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    // Check for code block markers (but not typeless blocks)
    if (line.startsWith('```') && !line.startsWith('```typeless')) {
      if (!inCodeBlock) {
        inCodeBlock = true
        const lang = line.slice(3).trim()
        codeBlockLang = languageAliases[lang.toLowerCase()] || lang
        codeBlockContent = []
      } else {
        // End of code block - create card
        const card: TypelessCardCode = {
          type: 'code',
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

  // Handle unclosed code block (streaming)
  if (inCodeBlock && codeBlockContent.length > 0) {
    result.push('```' + codeBlockLang)
    result.push(...codeBlockContent)
  }

  return result.join('\n')
}

/**
 * Parse markdown lists and convert to CardList
 */
function parseMarkdownLists(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let inList = false
  let listItems: ListItem[] = []
  let isOrdered = false
  let currentIndentLevel = 0
  let parentStack: ListItem[] = []

  const flushList = () => {
    if (inList && listItems.length > 0) {
      const card: TypelessCardList = {
        type: 'list',
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
    currentIndentLevel = 0
    parentStack = []
  }

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    // Check for unordered list item
    const ulMatch = line.match(/^(\s*)[-*+]\s+(.+)$/)
    // Check for ordered list item
    const olMatch = line.match(/^(\s*)\d+\.\s+(.+)$/)

    if (ulMatch || olMatch) {
      const match = ulMatch || olMatch
      if (!match) continue
      const indent = match[1]?.length ?? 0
      const itemContent = match[2] ?? ''
      const itemIsOrdered = !!olMatch

      if (!inList) {
        inList = true
        isOrdered = itemIsOrdered
        currentIndentLevel = indent
        parentStack = []
      }

      const newItem: ListItem = { content: itemContent }

      // Handle nested lists
      if (indent > currentIndentLevel) {
        // This is a sub-item
        const parent = parentStack[parentStack.length - 1]
        if (parent) {
          if (!parent.subItems) {
            parent.subItems = []
          }
          parent.subItems.push(newItem)
        } else {
          listItems.push(newItem)
        }
        parentStack.push(newItem)
      } else if (indent < currentIndentLevel) {
        // Going back up
        while (parentStack.length > 0 && indent <= currentIndentLevel) {
          parentStack.pop()
          currentIndentLevel -= 2 // Assume 2-space indent
        }
        const parent = parentStack[parentStack.length - 1]
        if (parent) {
          if (!parent.subItems) {
            parent.subItems = []
          }
          parent.subItems.push(newItem)
        } else {
          listItems.push(newItem)
        }
        parentStack.push(newItem)
      } else {
        // Same level
        if (parentStack.length > 1) {
          parentStack.pop()
          const parent = parentStack[parentStack.length - 1]
          if (parent) {
            if (!parent.subItems) {
              parent.subItems = []
            }
            parent.subItems.push(newItem)
          }
          parentStack.push(newItem)
        } else {
          parentStack = [newItem]
          listItems.push(newItem)
        }
      }

      currentIndentLevel = indent
      continue
    }

    // Empty line might end the list
    if (line.trim() === '' && inList) {
      // Check if next line continues the list
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

/**
 * Parse markdown images and convert to CardGallery
 * Supports: ![alt](url), base64 images, local file paths
 * Groups consecutive images into a single gallery with horizontal scroll
 */
function parseMarkdownImages(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []
  let pendingImages: GalleryImage[] = []

  const flushImages = () => {
    if (pendingImages.length > 0) {
      const card: TypelessCardGallery = {
        type: 'gallery',
        id: `md-gallery-${cardIndex.value++}`,
        images: pendingImages,
        layout: pendingImages.length > 1 ? 'horizontal' : 'grid',
        columns: Math.min(pendingImages.length, 4) as 2 | 3 | 4,
      }
      cards.push(card)
      result.push(`[[TYPELESS_CARD:${card.id}]]`)
      pendingImages = []
    }
  }

  // Image regex: ![alt](src) or ![](src)
  const imageRegex = /!\[([^\]]*)\]\(([^)]+)\)/g

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    // Check if line contains only images (possibly multiple)
    const trimmedLine = line.trim()
    let hasOnlyImages = false
    const lineImages: GalleryImage[] = []

    // Find all images in the line
    let match
    let lastIndex = 0
    let nonImageContent = ''

    while ((match = imageRegex.exec(trimmedLine)) !== null) {
      // Check for non-image content before this match
      nonImageContent += trimmedLine.slice(lastIndex, match.index).trim()
      lastIndex = match.index + match[0].length

      const alt = match[1] || ''
      const src = match[2]

      // Support base64, URLs, and local paths
      if (src && (src.startsWith('data:image/') || src.startsWith('http') || src.startsWith('/') || src.match(/^[a-zA-Z]:\\/))) {
        lineImages.push({
          src,
          alt: alt || undefined,
          // For local paths, generate thumbnail URL (backend should handle this)
          thumbnail: src.match(/^[a-zA-Z]:\\/) || src.startsWith('/') ? `${src}?thumbnail=true` : undefined,
        })
      }
    }

    // Check remaining content after last match
    nonImageContent += trimmedLine.slice(lastIndex).trim()
    imageRegex.lastIndex = 0 // Reset regex

    // If line has images and no other content, treat as image-only line
    if (lineImages.length > 0 && nonImageContent === '') {
      hasOnlyImages = true
      pendingImages.push(...lineImages)
    }

    // If this line is not image-only, flush pending images and add line
    if (!hasOnlyImages) {
      flushImages()
      result.push(line)
    }
  }

  flushImages()
  return result.join('\n')
}

/**
 * Parse standalone URLs and convert to CardLink
 * Only converts URLs that are on their own line (not inline links)
 */
function parseMarkdownLinks(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []

  // URL regex for standalone URLs
  const urlRegex = /^(https?:\/\/[^\s]+)$/

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    const trimmedLine = line.trim()
    const urlMatch = trimmedLine.match(urlRegex)

    if (urlMatch && urlMatch[1]) {
      const url = urlMatch[1]
      // Create a link card with basic info (metadata can be fetched later)
      const card: TypelessCardLink = {
        type: 'link',
        id: `md-link-${cardIndex.value++}`,
        url,
        title: getDomainFromUrl(url),
        // description and image will be populated by link preview service
      }
      cards.push(card)
      result.push(`[[TYPELESS_CARD:${card.id}]]`)
    } else {
      result.push(line)
    }
  }

  return result.join('\n')
}

/**
 * Extract domain from URL for display
 */
function getDomainFromUrl(url: string): string {
  try {
    const urlObj = new URL(url)
    return urlObj.hostname
  } catch {
    return url
  }
}

/**
 * Parse file paths and convert to CardFile
 * Supports Windows paths (C:\...) and Unix paths (/...)
 * Only converts paths that are on their own line
 */
function parseFilePaths(content: string, cards: TypelessCard[], cardIndex: { value: number }): string {
  const lines = content.split('\n')
  const result: string[] = []

  // File path regex: Windows (C:\path\file.ext) or Unix (/path/file.ext)
  // Must have a file extension
  const filePathRegex = /^([a-zA-Z]:\\[^\s]+\.[a-zA-Z0-9]+|\/[^\s]+\.[a-zA-Z0-9]+)$/

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line === undefined) continue

    const trimmedLine = line.trim()
    const pathMatch = trimmedLine.match(filePathRegex)

    if (pathMatch && pathMatch[1]) {
      const filePath = pathMatch[1]
      const filename = filePath.split(/[/\\]/).pop() || filePath
      const ext = filename.split('.').pop()?.toLowerCase() || ''

      // Skip if it's an image (handled by parseMarkdownImages)
      if (imageExtensions.includes(ext)) {
        result.push(line)
        continue
      }

      // Determine file size display (would need backend to get actual size)
      const card: TypelessCardFile = {
        type: 'file',
        id: `md-file-${cardIndex.value++}`,
        filename,
        downloadUrl: filePath,
        previewUrl: documentExtensions.includes(ext) ? filePath : undefined,
      }
      cards.push(card)
      result.push(`[[TYPELESS_CARD:${card.id}]]`)
    } else {
      result.push(line)
    }
  }

  return result.join('\n')
}
