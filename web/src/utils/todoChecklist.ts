import type { Message } from '@/api/chat'

const TODO_CHECKLIST_LINE_RE = /^[ \t]*[-*]\s+\[(?: |x|X)\]\s+(.+?)\s*$/
const TODO_CHECKLIST_ITEM_RE = /^[ \t]*[-*]\s+\[([ xX])\]\s+(.+?)\s*$/
const TODO_CHECKLIST_BLOCK_RE =
  /(^|\n)([ \t]*[-*]\s+\[(?: |x|X)\]\s+[^\n]+(?:\n[ \t]*[-*]\s+\[(?: |x|X)\]\s+[^\n]+)*)/m
const AFFIRMATIVE_CONTINUATION_RE =
  /^(?:ok(?:ay)?|sure|yes|yeah|yep|continue|go on|继续(?:吧|执行|一下)?|请继续(?:执行)?|接着(?:来|做)?|好的?|好|收到|行|继续工作)$/i

export interface TodoChecklistItemSummary {
  checked: boolean
  text: string
}

export interface TodoChecklistSummary {
  messageId: string
  focusMessageId: string
  todoCardId?: string
  items: TodoChecklistItemSummary[]
  totalCount: number
  completedCount: number
  pendingCount: number
  allCompleted: boolean
}

export function extractFirstTodoChecklistBlock(content: string): string | null {
  const match = TODO_CHECKLIST_BLOCK_RE.exec(content)
  return match?.[2] ?? null
}

export function stripFirstTodoChecklistBlock(content: string): string {
  const match = TODO_CHECKLIST_BLOCK_RE.exec(content)
  if (!match?.[2]) return content

  const leadingBoundary = match[1] ?? ''
  const checklistBlock = match[2]
  const start = match.index + leadingBoundary.length
  const end = start + checklistBlock.length
  const prefix = content.slice(0, start).replace(/[ \t]*\n*$/, '')
  const suffix = content.slice(end).replace(/^\n*[ \t]*/, '')

  if (prefix.trim() && suffix.trim()) {
    return `${prefix}\n\n${suffix}`.trim()
  }

  return `${prefix}${suffix}`.trim()
}

export function getTodoChecklistSignature(content: string): string | null {
  const block = extractFirstTodoChecklistBlock(content)
  if (!block) return null

  const items = block
    .split('\n')
    .map((line) => TODO_CHECKLIST_LINE_RE.exec(line)?.[1] ?? '')
    .map((item) => item.replace(/\s+/g, ' ').trim().toLowerCase())
    .filter(Boolean)

  return items.length > 0 ? items.join('\u0001') : null
}

export function summarizeTodoChecklist(
  content: string
): Omit<TodoChecklistSummary, 'messageId' | 'focusMessageId'> | null {
  const block = extractFirstTodoChecklistBlock(content)
  if (!block) return null

  const items = block
    .split('\n')
    .map((line) => {
      const match = TODO_CHECKLIST_ITEM_RE.exec(line)
      if (!match?.[2]) return null
      const checkedMarker = match[1]
      const text = match[2].trim()
      if (!checkedMarker || !text) return null

      return {
        checked: checkedMarker.toLowerCase() === 'x',
        text,
      }
    })
    .filter((item): item is TodoChecklistItemSummary => Boolean(item?.text))

  if (items.length === 0) return null

  const completedCount = items.filter((item) => item.checked).length
  const totalCount = items.length

  return {
    items,
    totalCount,
    completedCount,
    pendingCount: Math.max(0, totalCount - completedCount),
    allCompleted: completedCount === totalCount,
  }
}

export function findLatestTodoChecklistSummary(messages: Message[]): TodoChecklistSummary | null {
  for (let index = messages.length - 1; index >= 0; index--) {
    const message = messages[index]
    if (!message || message.role !== 'assistant') continue

    const summary = summarizeTodoChecklist(message.content)
    if (!summary) continue
    const focusMessage = findCanonicalTodoChecklistMessage(messages, message)

    return {
      messageId: message.id,
      focusMessageId: focusMessage.id,
      todoCardId: message.todo_card_id?.trim() || undefined,
      ...summary,
    }
  }

  return null
}

function isAffirmativeContinuationMessage(content: string): boolean {
  const normalized = content.replace(/\s+/g, ' ').trim()
  if (!normalized) return false
  return AFFIRMATIVE_CONTINUATION_RE.test(normalized)
}

function findCanonicalTodoChecklistMessage(messages: Message[], message: Message): Message {
  if (message.role !== 'assistant') return message
  if (message.todo_card_id?.trim()) return message

  const signature = getTodoChecklistSignature(message.content)
  if (!signature) return message

  const currentIndex = messages.findIndex((candidate) => candidate.id === message.id)
  if (currentIndex <= 0) return message

  for (let index = currentIndex - 1; index >= 0; index--) {
    const candidate = messages[index]
    if (!candidate || candidate.conversation_id !== message.conversation_id) continue

    if (candidate.role === 'user') {
      if (!isAffirmativeContinuationMessage(candidate.content || '')) {
        break
      }
      continue
    }

    if (candidate.role !== 'assistant') continue
    if (getTodoChecklistSignature(candidate.content) === signature) {
      return candidate
    }
  }

  return message
}

function isDuplicateTodoChecklistMessage(messages: Message[], message: Message): boolean {
  if (message.role !== 'assistant') return false
  if (message.todo_card_id?.trim()) return false

  const signature = getTodoChecklistSignature(message.content)
  if (!signature) return false

  const currentIndex = messages.findIndex((candidate) => candidate.id === message.id)
  if (currentIndex <= 0) return false

  for (let index = currentIndex - 1; index >= 0; index--) {
    const candidate = messages[index]
    if (!candidate || candidate.conversation_id !== message.conversation_id) continue

    if (candidate.role === 'user') {
      if (!isAffirmativeContinuationMessage(candidate.content || '')) {
        break
      }
      continue
    }

    if (candidate.role !== 'assistant') continue
    if (getTodoChecklistSignature(candidate.content) === signature) {
      return true
    }
  }

  return false
}

export function stripDuplicateTodoChecklistForMessage(
  messages: Message[],
  message: Message
): string {
  if (!isDuplicateTodoChecklistMessage(messages, message)) {
    return message.content
  }

  return stripFirstTodoChecklistBlock(message.content)
}
