type Translate = (key: string, fallback: string) => string

const STATUS_EDGE_PUNCTUATION_RE = /^[,.:;!?]+|[,.:;!?]+$/g
const STATUS_WRAPPER_CHARS = new Set(['[', '(', '{', '<', '【', '「', '『', '"', "'", '`'])

function unwrapStatusToken(value: string): string {
  let next = value.trim()
  while (next) {
    let start = 0
    let end = next.length
    while (start < end && STATUS_WRAPPER_CHARS.has(next[start]!)) {
      start += 1
    }
    while (end > start && STATUS_WRAPPER_CHARS.has(next[end - 1]!)) {
      end -= 1
    }
    const unwrapped = next.slice(start, end).trim()
    if (!unwrapped || unwrapped === next) return next
    next = unwrapped
  }
  return ''
}

function normalizeStatusToken(value: string): string {
  return unwrapStatusToken(value)
    .replace(STATUS_EDGE_PUNCTUATION_RE, '')
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
}

function humanizeStatusToken(value: string | null | undefined): string {
  const trimmed = unwrapStatusToken(String(value || ''))
  if (!trimmed) return ''
  return trimmed.replace(/[_-]+/g, ' ').replace(/^./, (char) => char.toUpperCase())
}

export function localizeStatusToken(
  value: string | null | undefined,
  translate: Translate
): string {
  const token = normalizeStatusToken(String(value || ''))
  if (!token) return ''

  switch (token) {
    case 'ok':
      return translate('system.statusOk', 'OK')
    case 'completed':
    case 'complete':
      return translate('chat.taskStageCompleted', 'Completed')
    case 'done':
      return translate('common.done', 'Done')
    case 'success':
    case 'succeeded':
      return translate('common.success', 'Success')
    case 'failed':
      return translate('chat.taskStageFailed', 'Failed')
    case 'error':
      return translate('common.error', 'Error')
    case 'cancelled':
    case 'canceled':
      return translate('chat.taskStageCancelled', 'Cancelled')
    case 'pending':
    case 'planned':
    case 'planning':
      return translate('chat.taskStagePlanning', 'Planning')
    case 'running':
    case 'working':
    case 'executing':
    case 'processing':
    case 'in_progress':
      return translate('chat.taskStageWorking', 'Working')
    case 'verifying':
    case 'verification':
      return translate('chat.taskStageVerifying', 'Verifying')
    case 'waiting':
    case 'queued':
      return translate('chat.taskStageWaiting', 'Waiting')
    case 'partial':
      return translate('chat.taskStagePartial', 'Partially passed')
    case 'warning':
      return translate('system.warning', 'Warning')
    case 'info':
      return translate('system.info', 'Info')
    default:
      return humanizeStatusToken(value)
  }
}
