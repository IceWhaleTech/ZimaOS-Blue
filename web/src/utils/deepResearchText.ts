type Translate = (key: string, fallback: string) => string

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
  running: ['chat.deepResearchProgress', 'Deep Research Running'],
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

function humanizeDeepResearchToken(value: string | null | undefined): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  return trimmed.replace(/[_-]+/g, ' ').replace(/^./, (char) => char.toUpperCase())
}

function normalizeDeepResearchToken(value: string): string {
  return value
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

export function splitDeepResearchSegments(value: string | null | undefined): string[] {
  return String(value || '')
    .trim()
    .split(/\s*[•·]\s*/)
    .map((part) => part.trim())
    .filter(Boolean)
}
