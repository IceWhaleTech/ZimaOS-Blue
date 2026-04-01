type Translate = (key: string, fallback: string) => string

const DEEP_RESEARCH_WRAPPER_PREFIX_RE = /^[\[\(\{<【「『"'`]+/
const DEEP_RESEARCH_WRAPPER_SUFFIX_RE = /[\]\)\}>】」』"'`]+$/

const DEEP_RESEARCH_STAGE_KEYS: Record<string, [key: string, fallback: string]> = {
  intake: ['chat.deepResearchStageIntake', 'Intake'],
  planning: ['chat.deepResearchStagePlanning', 'Planning'],
  retrieve: ['chat.deepResearchStageRetrieve', 'Retrieving'],
  verify: ['chat.deepResearchStageVerify', 'Verifying'],
  synthesize: ['chat.deepResearchStageSynthesize', 'Synthesizing'],
  synthesizing: ['chat.deepResearchStageSynthesize', 'Synthesizing'],
  completed: ['chat.deepResearchStageCompleted', 'Completed'],
  failed: ['chat.deepResearchStageFailed', 'Failed'],
  cancelled: ['chat.deepResearchStageCancelled', 'Cancelled'],
}

const DEEP_RESEARCH_ACTION_KEYS: Record<string, [key: string, fallback: string]> = {
  augment_query: ['chat.deepResearchActionAugmentQuery', 'Augmenting query'],
  initial_retrieve: ['chat.deepResearchActionInitialRetrieve', 'Running initial retrieval'],
  followup_retrieve: ['chat.deepResearchActionFollowupRetrieve', 'Running follow-up retrieval'],
  verification: ['chat.deepResearchActionVerification', 'Verifying evidence'],
  verification_completed: [
    'chat.deepResearchActionVerificationCompleted',
    'Verification completed',
  ],
  followup_planned: ['chat.deepResearchActionFollowupPlanned', 'Follow-up planned'],
  loop_stopped: ['chat.deepResearchActionLoopStopped', 'Research loop stopped'],
  synthesizing: ['chat.deepResearchActionSynthesizing', 'Synthesizing report'],
  completed: ['chat.deepResearchActionCompleted', 'Completed'],
}

const DEEP_RESEARCH_STATUS_KEYS: Record<string, [key: string, fallback: string]> = {
  running: ['chat.deepResearchProgress', 'Deep Research in progress'],
  pending: ['chat.deepResearchStagePlanning', 'Planning'],
  completed: ['chat.deepResearchStageCompleted', 'Completed'],
  failed: ['chat.deepResearchStageFailed', 'Failed'],
  cancelled: ['chat.deepResearchStageCancelled', 'Cancelled'],
}

const DEEP_RESEARCH_MODE_KEYS: Record<string, [key: string, fallback: string]> = {
  fast: ['chat.deepResearchModeFast', 'Fast'],
  standard: ['chat.deepResearchModeStandard', 'Standard'],
  deep: ['chat.deepResearchModeDeep', 'Deep'],
}

const DEEP_RESEARCH_AUX_STATUS_KEYS: Record<string, [key: string, fallback: string]> = {
  pending: ['chat.deepResearchWorkflowPending', 'Pending'],
  current: ['chat.deepResearchWorkflowCurrent', 'Current'],
  resolved: ['chat.deepResearchVerificationResolved', 'Resolved'],
  conflicted: ['chat.deepResearchVerificationConflicted', 'Conflicted'],
  insufficient: ['chat.deepResearchVerificationInsufficient', 'Insufficient'],
  success: ['common.success', 'Success'],
  error: ['common.error', 'Error'],
}

const DEEP_RESEARCH_GAP_KEYS: Record<string, [key: string, fallback: string]> = {
  need_evidence_coverage: ['chat.deepResearchGapNeedEvidenceCoverage', 'Need evidence coverage'],
  need_primary_or_official_sources: [
    'chat.deepResearchGapNeedPrimaryOrOfficialSources',
    'Need primary or official sources',
  ],
  need_broader_evidence_coverage: [
    'chat.deepResearchGapNeedBroaderEvidenceCoverage',
    'Need broader evidence coverage',
  ],
  need_broader_source_diversity: [
    'chat.deepResearchGapNeedBroaderSourceDiversity',
    'Need broader source diversity',
  ],
  need_fresher_sources: ['chat.deepResearchGapNeedFresherSources', 'Need fresher sources'],
  resolve_conflicting_claims: [
    'chat.deepResearchGapResolveConflictingClaims',
    'Resolve conflicting claims',
  ],
}

function humanizeDeepResearchToken(value: string | null | undefined): string {
  const trimmed = unwrapDeepResearchToken(String(value || ''))
  if (!trimmed) return ''
  return trimmed.replace(/[_-]+/g, ' ').replace(/^./, (char) => char.toUpperCase())
}

function unwrapDeepResearchToken(value: string): string {
  let next = value.trim()
  while (next) {
    const unwrapped = next
      .replace(DEEP_RESEARCH_WRAPPER_PREFIX_RE, '')
      .replace(DEEP_RESEARCH_WRAPPER_SUFFIX_RE, '')
      .trim()
    if (!unwrapped || unwrapped === next) {
      return next
    }
    next = unwrapped
  }
  return ''
}

function normalizeDeepResearchToken(value: string): string {
  return unwrapDeepResearchToken(value)
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
}

function translateKnownToken(
  token: string,
  table: Record<string, [key: string, fallback: string]>,
  translate: Translate
): string {
  const entry = table[token]
  return entry ? translate(entry[0], entry[1]) : ''
}

function optionalTranslate(key: string, translate: Translate): string {
  const marker = `__missing__${key}`
  const value = translate(key, marker)
  return value === marker ? '' : value
}

export function localizeResearchSurfaceTitle(translate: Translate): string {
  return (
    optionalTranslate('chat.deepResearchTitle', translate) ||
    optionalTranslate('ui.deepResearchTitle', translate) ||
    optionalTranslate('chat.researchTitle', translate) ||
    optionalTranslate('harness.quickEval.researchLabel', translate) ||
    'Deep Research'
  )
}

export function localizeResearchProgressLabel(translate: Translate): string {
  return (
    optionalTranslate('chat.deepResearchProgress', translate) ||
    optionalTranslate('chat.researchProgress', translate) ||
    `${localizeResearchSurfaceTitle(translate)} in progress`
  )
}

export function localizeResearchRunningTasksLabel(translate: Translate): string {
  return (
    optionalTranslate('chat.deepResearchRunningTasks', translate) ||
    optionalTranslate('chat.researchRunningTasks', translate) ||
    'Running Deep Research tasks'
  )
}

export function localizeResearchRunningElsewhereLabel(translate: Translate): string {
  return (
    optionalTranslate('chat.deepResearchRunningElsewhere', translate) ||
    optionalTranslate('chat.researchRunningElsewhere', translate) ||
    'Track active Deep Research tasks across conversations.'
  )
}

export function localizeDeepResearchStage(
  value: string | null | undefined,
  translate: Translate
): string {
  const token = normalizeDeepResearchToken(String(value || ''))
  return (
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    humanizeDeepResearchToken(value)
  )
}

export function localizeDeepResearchAction(
  value: string | null | undefined,
  translate: Translate
): string {
  const token = normalizeDeepResearchToken(String(value || ''))
  return (
    translateKnownToken(token, DEEP_RESEARCH_AUX_STATUS_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_ACTION_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    humanizeDeepResearchToken(value)
  )
}

export function localizeDeepResearchStatus(
  value: string | null | undefined,
  translate: Translate
): string {
  const token = normalizeDeepResearchToken(String(value || ''))
  return (
    translateKnownToken(token, DEEP_RESEARCH_AUX_STATUS_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STATUS_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    humanizeDeepResearchToken(value)
  )
}

export function localizeDeepResearchMode(
  value: string | null | undefined,
  translate: Translate
): string {
  const token = normalizeDeepResearchToken(String(value || ''))
  return (
    translateKnownToken(token, DEEP_RESEARCH_MODE_KEYS, translate) ||
    humanizeDeepResearchToken(value)
  )
}

export function localizeDeepResearchSegment(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const token = normalizeDeepResearchToken(trimmed)
  return (
    translateKnownToken(token, DEEP_RESEARCH_AUX_STATUS_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_ACTION_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STATUS_KEYS, translate) ||
    humanizeDeepResearchToken(trimmed)
  )
}

export function localizeDeepResearchGap(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const token = normalizeDeepResearchToken(trimmed)
  return translateKnownToken(token, DEEP_RESEARCH_GAP_KEYS, translate) || trimmed
}

export function splitDeepResearchSegments(value: string | null | undefined): string[] {
  return String(value || '')
    .trim()
    .split(/\s*[•·]\s*/)
    .map((part) => part.trim())
    .filter(Boolean)
}
