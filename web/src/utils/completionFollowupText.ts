import {
  getCompletionFollowupMessagesForHeading,
  type CompletionFollowupChatMessages,
} from '../i18n/completion-followup-backfills'

const ENGLISH_COMPLETION_FOLLOWUP_HEADING = "If you'd like, I can also help with:"
const COMPLETION_FOLLOWUP_WRAPPER_PREFIX_RE = /^[\[\(\{<【「『"'`*_~]+/
const COMPLETION_FOLLOWUP_WRAPPER_SUFFIX_RE = /[\]\)\}>】」』"'`*_~]+$/
const COMPLETION_FOLLOWUP_EDGE_PUNCTUATION_RE = /^[,.:;!?，。！？；：]+|[,.:;!?，。！？；：]+$/g
const COMPLETION_FOLLOWUP_LIST_PREFIX_RE = /^([ \t]*\d+[.)]\s+)(.*)$/

const COMPLETION_FOLLOWUP_EXACT_LINE_KEYS = [
  [
    'No further action needed.',
    'completionFollowupNoFurtherActionNeeded',
  ],
  [
    "If you'd like, I can expand this into a fuller report.",
    'completionFollowupExpandFullerReport',
  ],
  [
    "If you'd like, I can help verify the key evidence in the tool cards above.",
    'completionFollowupVerifyKeyEvidence',
  ],
  [
    "If you'd like, tell me your top priority (for example performance/cost/risk), and I can reorder the takeaways for you.",
    'completionFollowupReorderTakeaways',
  ],
  [
    'If you want, I can provide 1-3 actionable optimization ideas based on these results.',
    'completionFollowupOptimizationIdeas',
  ],
  [
    "If you'd like, I can inspect the failed steps and retry with a safer fallback path.",
    'completionFollowupInspectFailedSteps',
  ],
  [
    'If you want, I can re-run validation after the recovery attempt to confirm the result.',
    'completionFollowupRerunValidation',
  ],
  [
    "If you'd like, I can keep fixing the remaining issues or finalize the report.",
    'completionFollowupKeepFixing',
  ],
  [
    "If you'd like, I can help verify the deliverables in your environment.",
    'completionFollowupVerifyDeliverables',
  ],
  [
    'If you want, I can help run the relevant tests to confirm there are no regressions.',
    'completionFollowupRunRelevantTests',
  ],
  [
    "If you'd like, I can optimize the next area you care about.",
    'completionFollowupOptimizeNextArea',
  ],
] as const satisfies readonly [
  readonly [string, keyof CompletionFollowupChatMessages],
  ...ReadonlyArray<readonly [string, keyof CompletionFollowupChatMessages]>,
]

function unwrapCompletionFollowupHeading(value: string): string {
  let next = value.trim()
  while (next) {
    const unwrapped = next
      .replace(COMPLETION_FOLLOWUP_WRAPPER_PREFIX_RE, '')
      .replace(COMPLETION_FOLLOWUP_WRAPPER_SUFFIX_RE, '')
      .trim()
    if (!unwrapped || unwrapped === next) return next
    next = unwrapped
  }
  return ''
}

function normalizeCompletionFollowupHeading(value: string): string {
  return unwrapCompletionFollowupHeading(value)
    .replace(COMPLETION_FOLLOWUP_EDGE_PUNCTUATION_RE, '')
    .replace(/’/g, "'")
    .trim()
    .toLowerCase()
    .replace(/\s+/g, ' ')
}

const COMPLETION_FOLLOWUP_HEADING_VARIANTS = new Set([
  normalizeCompletionFollowupHeading(ENGLISH_COMPLETION_FOLLOWUP_HEADING),
  normalizeCompletionFollowupHeading("如果你'd like，我还可以帮你："),
  normalizeCompletionFollowupHeading('如果你’d like，我还可以帮你：'),
])

const COMPLETION_FOLLOWUP_EXACT_LINES = new Map(
  COMPLETION_FOLLOWUP_EXACT_LINE_KEYS.map(([englishLine, key]) => [
    normalizeCompletionFollowupHeading(englishLine),
    key,
  ])
)

function isCompletionFollowupHeadingLine(value: string): boolean {
  const normalized = normalizeCompletionFollowupHeading(value)
  return normalized.length > 0 && COMPLETION_FOLLOWUP_HEADING_VARIANTS.has(normalized)
}

function localizeExactCompletionFollowupLine(
  value: string,
  messages: CompletionFollowupChatMessages | null
): string {
  if (!messages) return value

  const key = COMPLETION_FOLLOWUP_EXACT_LINES.get(normalizeCompletionFollowupHeading(value))
  if (!key) return value
  return messages[key]
}

function replaceCompletionFollowupPrefix(
  value: string,
  patterns: readonly string[],
  replacement: string
): string {
  const trimmedValue = value.trimStart()
  const indentation = value.slice(0, value.length - trimmedValue.length)
  const lowerTrimmed = trimmedValue.toLowerCase()

  for (const pattern of patterns) {
    const normalizedPattern = pattern.toLowerCase()
    if (!lowerTrimmed.startsWith(normalizedPattern)) continue
    return indentation + replacement + trimmedValue.slice(pattern.length)
  }

  return value
}

function localizeCompletionFollowupBody(
  value: string,
  messages: CompletionFollowupChatMessages | null
): string {
  const exactLocalized = localizeExactCompletionFollowupLine(value, messages)
  if (exactLocalized !== value) return exactLocalized
  if (!messages) return value

  const ifYoudLikePatterns = [
    "If you'd like, ",
    "If you'd like,",
    "如果你'd like，",
    "如果你'd like, ",
    "如果你'd like,",
    '如果你’d like，',
    '如果你’d like, ',
    '如果你’d like,',
  ] as const
  const ifYouWantPatterns = ['If you want, ', 'If you want,'] as const

  const localizedIfYoudLike = replaceCompletionFollowupPrefix(
    value,
    ifYoudLikePatterns,
    messages.completionFollowupIfYoudLikePrefix
  )
  if (localizedIfYoudLike !== value) return localizedIfYoudLike

  return replaceCompletionFollowupPrefix(
    value,
    ifYouWantPatterns,
    messages.completionFollowupIfYouWantPrefix
  )
}

export function localizeCompletionFollowupHeading(
  content: string,
  localized:
    | string
    | CompletionFollowupChatMessages
): string {
  const localizedHeading =
    typeof localized === 'string' ? localized : localized.completionFollowupHeading
  if (!content || !localizedHeading) return content
  const messages =
    typeof localized === 'string'
      ? getCompletionFollowupMessagesForHeading(localizedHeading)
      : localized

  return content
    .split('\n')
    .map((line) => {
      const match = /^([ \t]*)(.*)$/.exec(line)
      const indentation = match?.[1] ?? ''
      const body = match?.[2] ?? line
      if (isCompletionFollowupHeadingLine(body)) {
        return `${indentation}${localizedHeading}`
      }

      const listMatch = COMPLETION_FOLLOWUP_LIST_PREFIX_RE.exec(line)
      if (listMatch?.[2]) {
        const listPrefix = listMatch[1] ?? ''
        const localizedBody = localizeCompletionFollowupBody(listMatch[2], messages)
        return localizedBody === listMatch[2] ? line : `${listPrefix}${localizedBody}`
      }

      const localizedBody = localizeCompletionFollowupBody(body, messages)
      return localizedBody === body ? line : `${indentation}${localizedBody}`
    })
    .join('\n')
}
