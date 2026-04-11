type Translate = (key: string, fallback: string) => string

const STATUS_WRAPPER_PREFIX_RE = /^[\[\(\{<【「『"'`]+/
const STATUS_WRAPPER_SUFFIX_RE = /[\]\)\}>】」』"'`]+$/
const STATUS_EDGE_PUNCTUATION_RE = /^[,.:;!?]+|[,.:;!?]+$/g

function unwrapStatusToken(value: string): string {
  let next = value.trim()
  while (next) {
    const unwrapped = next
      .replace(STATUS_WRAPPER_PREFIX_RE, '')
      .replace(STATUS_WRAPPER_SUFFIX_RE, '')
      .trim()
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
