import type { Message } from '@/api/chat'

const TODO_CHECKLIST_LINE_RE = /^[ \t]*[-*]\s+\[(?: |x|X)\]\s+(.+?)\s*$/
const TODO_CHECKLIST_BLOCK_RE =
  /(^|\n)([ \t]*[-*]\s+\[(?: |x|X)\]\s+[^\n]+(?:\n[ \t]*[-*]\s+\[(?: |x|X)\]\s+[^\n]+)*)/m
const AFFIRMATIVE_CONTINUATION_RE =
  /^(?:ok(?:ay)?|sure|yes|yeah|yep|continue|go on|继续(?:吧|执行|一下)?|请继续(?:执行)?|接着(?:来|做)?|好的?|好|收到|行|继续工作)$/i

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

function isAffirmativeContinuationMessage(content: string): boolean {
  const normalized = content.replace(/\s+/g, ' ').trim()
  if (!normalized) return false
  return AFFIRMATIVE_CONTINUATION_RE.test(normalized)
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
