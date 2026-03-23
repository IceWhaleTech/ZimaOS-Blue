import type { Message } from '@/api/chat'

const TODO_CHECKLIST_LINE_RE = /^[ \t]*[-*]\s+\[(?: |x|X)\]\s+(.+?)\s*$/
const TODO_CHECKLIST_ITEM_RE = /^[ \t]*[-*]\s+\[([ xX])\]\s+(.+?)\s*$/
const TODO_CHECKLIST_BLOCK_RE =
  /(^|\n)([ \t]*[-*]\s+\[(?: |x|X)\]\s+[^\n]+(?:\n[ \t]*[-*]\s+\[(?: |x|X)\]\s+[^\n]+)*)/m
const AFFIRMATIVE_CONTINUATION_RE =
  /^(?:ok(?:ay)?|sure|yes|yeah|yep|continue|go on|继续(?:吧|执行|一下)?|请继续(?:执行)?|接着(?:来|做)?|好的?|好|收到|行|继续工作)$/i
const COMPLETION_EN_CUES = [
  'task complete',
  'task completed',
  'completed successfully',
  'all done',
  'final summary',
] as const
const COMPLETION_ZH_CUES = [
  '任务已完成',
  '任务完成',
  '总结：任务已完成',
  '最终总结',
  '全部完成',
] as const
const COMPLETION_ZH_DELIVERY_CUES = [
  '地址',
  '链接',
  'localhost',
  'http://',
  'https://',
  '可直接运行',
  '可以直接运行',
  '运行地址',
] as const
const COMPLETION_EN_DELIVERY_CUES = [
  'localhost',
  'http://',
  'https://',
  'url',
  'link',
  'ready to run',
  'run at',
] as const
const ARTIFACT_ZH_STRONG_CUES = [
  '已写入文件',
  '写入文件：',
  '写入到文件',
  '保存到文件',
  '已保存到',
  '完整报告已写入',
  '完整报告已保存',
  '报告已写入',
  '报告已保存',
  '输出文件：',
] as const
const ARTIFACT_ZH_ACTION_CUES = ['写入', '保存', '输出', '写出', '导出', '生成'] as const
const ARTIFACT_ZH_OBJECT_CUES = [
  '文件',
  '报告',
  '文档',
  '完整报告',
  '最终报告',
  '.md',
  '.txt',
  '.html',
  '.css',
  '.json',
  '.csv',
  '.pdf',
] as const
const ARTIFACT_EN_STRONG_CUES = [
  'wrote file',
  'written to',
  'saved to',
  'report saved',
  'saved the report',
  'output file',
] as const
const ARTIFACT_EN_ACTION_CUES = [
  'write',
  'wrote',
  'written',
  'save',
  'saved',
  'output',
  'export',
  'generate',
  'deliver',
] as const
const ARTIFACT_EN_OBJECT_CUES = [
  'file',
  'report',
  'document',
  'artifact',
  '.md',
  '.txt',
  '.html',
  '.css',
  '.json',
  '.csv',
  '.pdf',
] as const
const COMPLETION_EN_SECTION_CUES = [
  'what was accomplished',
  'how to use',
  'how to test',
  'next steps',
  "if you'd like, i can help with",
  "if you'd like, i can also help with",
] as const
const COMPLETION_ZH_SECTION_CUES = [
  '完成内容',
  '使用方法',
  '测试方法',
  '下一步建议',
  '如果你愿意，我可以继续帮你',
  '如果你愿意，我还可以帮你',
] as const

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

export interface TodoChecklistCompletionSignal {
  messageId?: string
  todoCardId?: string
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

export function findLatestTodoChecklistSummary(
  messages: Message[],
  completionSignal?: TodoChecklistCompletionSignal | null
): TodoChecklistSummary | null {
  const activeScopeStartIndex = findActiveTodoScopeStartIndex(messages)

  for (let index = messages.length - 1; index >= 0; index--) {
    if (index < activeScopeStartIndex) {
      return null
    }

    const message = messages[index]
    if (!message || message.role !== 'assistant') continue

    const latestSummary = summarizeTodoChecklist(message.content)
    if (!latestSummary) continue
    const canonicalMessage = findCanonicalTodoChecklistMessage(messages, message)
    const summaryMessage = canonicalMessage.id === message.id ? message : canonicalMessage
    const summary = summarizeTodoChecklist(summaryMessage.content) ?? latestSummary
    if (summary.allCompleted) {
      return null
    }
    if (matchesExplicitTodoCompletion(summaryMessage, canonicalMessage, completionSignal)) {
      return null
    }

    const completionSeen = messages
      .slice(index)
      .some(
        (candidate) =>
          candidate?.role === 'assistant' && isLikelyTodoFinalizationMessage(candidate.content || '')
      )
    if (completionSeen) {
      return null
    }

    return {
      messageId: summaryMessage.id,
      focusMessageId: canonicalMessage.id,
      todoCardId:
        canonicalMessage.todo_card_id?.trim() || message.todo_card_id?.trim() || undefined,
      ...summary,
    }
  }

  return null
}

function findActiveTodoScopeStartIndex(messages: Message[]): number {
  for (let index = messages.length - 1; index >= 0; index--) {
    const message = messages[index]
    if (!message || message.role !== 'user') continue
    if (isAffirmativeContinuationMessage(message.content || '')) continue
    return index + 1
  }

  return 0
}

function isAffirmativeContinuationMessage(content: string): boolean {
  const normalized = content.replace(/\s+/g, ' ').trim()
  if (!normalized) return false
  return AFFIRMATIVE_CONTINUATION_RE.test(normalized)
}

function isLikelyTaskCompletionMessage(content: string): boolean {
  const trimmed = content.trim()
  if (!trimmed) return false

  const lower = trimmed.toLowerCase()

  if (COMPLETION_EN_CUES.some((cue) => lower.includes(cue))) {
    return true
  }

  if (COMPLETION_ZH_CUES.some((cue) => trimmed.includes(cue))) {
    return true
  }

  if (
    (trimmed.includes('已完成') || trimmed.includes('已经完成')) &&
    COMPLETION_ZH_DELIVERY_CUES.some((cue) => trimmed.includes(cue))
  ) {
    return true
  }

  if (
    (lower.includes('completed') || lower.includes('done')) &&
    COMPLETION_EN_DELIVERY_CUES.some((cue) => lower.includes(cue))
  ) {
    return true
  }

  const enSectionMatches = COMPLETION_EN_SECTION_CUES.reduce(
    (count, cue) => count + (lower.includes(cue) ? 1 : 0),
    0
  )
  if (enSectionMatches >= 2) {
    return true
  }

  const zhSectionMatches = COMPLETION_ZH_SECTION_CUES.reduce(
    (count, cue) => count + (trimmed.includes(cue) ? 1 : 0),
    0
  )
  return zhSectionMatches >= 2
}

function isLikelyArtifactDeliveryMessage(content: string): boolean {
  const trimmed = content.trim()
  if (!trimmed) return false

  const lower = trimmed.toLowerCase()

  if (ARTIFACT_ZH_STRONG_CUES.some((cue) => trimmed.includes(cue))) {
    return true
  }

  if (
    ARTIFACT_ZH_ACTION_CUES.some((cue) => trimmed.includes(cue)) &&
    ARTIFACT_ZH_OBJECT_CUES.some((cue) => trimmed.includes(cue))
  ) {
    return true
  }

  if (ARTIFACT_EN_STRONG_CUES.some((cue) => lower.includes(cue))) {
    return true
  }

  return (
    ARTIFACT_EN_ACTION_CUES.some((cue) => lower.includes(cue)) &&
    ARTIFACT_EN_OBJECT_CUES.some((cue) => lower.includes(cue))
  )
}

function isLikelyTodoFinalizationMessage(content: string): boolean {
  return isLikelyTaskCompletionMessage(content) || isLikelyArtifactDeliveryMessage(content)
}

function matchesExplicitTodoCompletion(
  summaryMessage: Message,
  canonicalMessage: Message,
  completionSignal?: TodoChecklistCompletionSignal | null
): boolean {
  const normalizedMessageId = completionSignal?.messageId?.trim()
  const normalizedTodoCardId = completionSignal?.todoCardId?.trim()
  if (!normalizedMessageId && !normalizedTodoCardId) {
    return false
  }

  const candidateTodoCardIds = [
    canonicalMessage.todo_card_id?.trim(),
    summaryMessage.todo_card_id?.trim(),
  ].filter((value): value is string => Boolean(value))

  if (normalizedTodoCardId && candidateTodoCardIds.includes(normalizedTodoCardId)) {
    return true
  }

  if (!normalizedMessageId) {
    return false
  }

  return (
    normalizedMessageId === canonicalMessage.id ||
    normalizedMessageId === summaryMessage.id ||
    normalizedMessageId === canonicalMessage.render_key ||
    normalizedMessageId === summaryMessage.render_key
  )
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
