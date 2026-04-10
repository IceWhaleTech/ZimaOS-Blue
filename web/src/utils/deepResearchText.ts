type Translate = (key: string, fallback: string) => string

const DEEP_RESEARCH_WRAPPER_PREFIX_RE = /^[\[\(\{<【「『"'`]+/
const DEEP_RESEARCH_WRAPPER_SUFFIX_RE = /[\]\)\}>】」』"'`]+$/
const DEEP_RESEARCH_EDGE_PUNCTUATION_RE = /^[,.:;!?]+|[,.:;!?]+$/g

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
  research_brief_prepared: [
    'chat.deepResearchActionResearchBriefPrepared',
    'Research brief prepared',
  ],
  draft_synthesis_ready: [
    'chat.deepResearchActionDraftSynthesisReady',
    'Draft synthesis ready',
  ],
  detected_a_research_gap: [
    'chat.deepResearchActionDetectedResearchGap',
    'Detected a research gap',
  ],
  completed: ['chat.deepResearchActionCompleted', 'Completed'],
}

const DEEP_RESEARCH_ACTION_ALIASES: Record<string, string> = {
  verification_pass_completed: 'verification_completed',
  planned_a_followup_research_pass: 'followup_planned',
  planned_a_follow_up_research_pass: 'followup_planned',
  deep_research_loop_stopped: 'loop_stopped',
  research_loop_stopped: 'loop_stopped',
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
  warning: ['system.warning', 'Warning'],
  info: ['system.info', 'Info'],
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

const DEEP_RESEARCH_GAP_ALIASES: Record<string, string> = {
  need_primary_evidence: 'need_primary_or_official_sources',
  need_primary_source: 'need_primary_or_official_sources',
  need_primary_sources: 'need_primary_or_official_sources',
  need_one_primary_source: 'need_primary_or_official_sources',
  need_one_more_primary_source: 'need_primary_or_official_sources',
  need_another_primary_source: 'need_primary_or_official_sources',
  need_official_source: 'need_primary_or_official_sources',
  need_official_sources: 'need_primary_or_official_sources',
  need_one_official_source: 'need_primary_or_official_sources',
  need_one_more_official_source: 'need_primary_or_official_sources',
  need_another_official_source: 'need_primary_or_official_sources',
  need_primary_or_official_source: 'need_primary_or_official_sources',
}

const DEEP_RESEARCH_TOKEN_KEYS: Record<string, [key: string, fallback: string]> = {
  identity_validation: ['chat.deepResearchAxisIdentityValidation', 'Identity verification'],
  internet_footprint: ['chat.deepResearchAxisInternetFootprint', 'Internet footprint'],
  overview: ['harness.group.overview', 'Overview'],
  latest: ['chat.deepResearchAxisLatest', 'Latest developments'],
  official: ['chat.deepResearchAxisOfficial', 'Official sources'],
  comparison: ['chat.deepResearchAxisComparison', 'Comparative information'],
  best_practices: ['chat.deepResearchAxisBestPractices', 'Best practices'],
  retry: ['chat.deepResearchAxisRetry', 'Retry correction'],
  research: ['chat.deepResearchAxisResearch', 'Research'],
  source_diversity: ['chat.deepResearchFocusSourceDiversity', 'Source diversity'],
  freshness: ['chat.deepResearchFocusFreshness', 'Freshness'],
  claim_validation: ['chat.deepResearchFocusClaimValidation', 'Claim validation'],
  earlier: ['chat.deepResearchTimeWindowEarlier', 'Early'],
  early: ['chat.deepResearchTimeWindowEarlier', 'Early'],
  middle: ['chat.deepResearchTimeWindowMiddle', 'Middle'],
  recent: ['chat.deepResearchTimeWindowRecent', 'Recent'],
}

const DEEP_RESEARCH_TOKEN_ALIASES: Record<string, string> = {
  official_sources: 'official',
  latest_developments: 'latest',
  comparative_information: 'comparison',
}

const DEEP_RESEARCH_STRUCTURED_VALUE_KEYS: Record<string, [key: string, fallback: string]> = {
  law: ['chat.deepResearchSourceTypeLaw', 'Law'],
  filing: ['chat.deepResearchSourceTypeFiling', 'Filing'],
  paper: ['chat.deepResearchSourceTypePaper', 'Paper'],
  web: ['chat.deepResearchSourceTypeWeb', 'Web'],
  financial: ['chat.deepResearchMetaFinancial', 'Financial'],
  scope: ['chat.deepResearchWorkflowScope', 'Scope'],
  sources: ['chat.deepResearchWorkflowSources', 'Sources'],
  extraction: ['chat.deepResearchWorkflowExtraction', 'Extraction'],
  audit: ['chat.deepResearchVerificationSummary', 'Verification'],
  synthesis: ['chat.deepResearchStageSynthesize', 'Synthesizing'],
}

const DEEP_RESEARCH_STOP_REASON_KEYS: Record<string, [key: string, fallback: string]> = {
  coverage_sufficient: ['chat.deepResearchStopReasonCoverage', 'Coverage target reached'],
  no_new_canonical_evidence: [
    'chat.deepResearchStopReasonNoNewEvidence',
    'No new canonical evidence found',
  ],
  budget_exhausted: ['chat.deepResearchStopReasonBudget', 'Research budget exhausted'],
}

const DEEP_RESEARCH_REPORT_STYLE_KEYS: Record<string, [key: string, fallback: string]> = {
  knowledge_base: ['chat.deepResearchReportStyleKnowledgeBase', 'Knowledge base'],
  timeline: ['chat.deepResearchTimeline', 'Timeline'],
}

const DEEP_RESEARCH_PLANNED_TASKS_RE = /^planned\s+(\d+)\s+research\s+task(?:\(s\)|s)?$/i
const DEEP_RESEARCH_COLLECTED_SOURCES_RE = /^collected\s+(\d+)\s+source(?:\(s\)|s)?$/i
const DEEP_RESEARCH_SUMMARY_GAP_COVERAGE_RE =
  /^(.+?)\s+only covers\s+(\d+)\s+evidence item\(s\)\s+across\s+(\d+)\s+domain\(s\);\s+follow-up research is needed\.?$/i
const DEEP_RESEARCH_SUMMARY_OFFICIAL_GAP_RE =
  /^(.+?)\s+still lacks stable primary or official sources\.?$/i
const DEEP_RESEARCH_SUMMARY_RESOLVED_COVERAGE_RE =
  /^(.+?)\s+is covered by\s+(\d+)\s+evidence item\(s\)\s+across\s+(\d+)\s+domain\(s\)\.?$/i
const DEEP_RESEARCH_SUMMARY_DOMAIN_COVERAGE_RE =
  /^Coverage spans\s+(\d+)\s+unique domain\(s\)\.?$/i
const DEEP_RESEARCH_SUMMARY_FRESHNESS_COVERAGE_RE =
  /^Fresh evidence reaches\s+(\d+)\.?$/i
const DEEP_RESEARCH_SUMMARY_CLAIM_SUPPORT_COVERAGE_RE =
  /^Core conclusions are supported across\s+(\d+)\s+claim group\(s\)\.?$/i
const DEEP_RESEARCH_PRIMARY_SOURCE_VERIFICATION_RE =
  /^Check primary sources for final verification\.?$/i

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
    .replace(DEEP_RESEARCH_EDGE_PUNCTUATION_RE, '')
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
}

function resolveTokenAlias(token: string, aliases: Record<string, string>): string {
  return aliases[token] || token
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

function formatDeepResearchCount(label: string, count: string): string {
  return `${label}: ${count}`
}

function formatDeepResearchTemplate(
  template: string,
  replacements: Record<string, string | number>
): string {
  return Object.entries(replacements).reduce(
    (text, [key, value]) => text.replaceAll(`{${key}}`, String(value)),
    template
  )
}

function resolveDeepResearchFocusLabel(value: string, translate: Translate): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  return (
    localizeDeepResearchStructuredValue(trimmed, translate) ||
    localizeDeepResearchSegment(trimmed, translate) ||
    trimmed
  )
}

export function localizeResearchSurfaceTitle(translate: Translate): string {
  return (
    optionalTranslate('chat.deepResearchTitle', translate) ||
    optionalTranslate('ui.deepResearchTitle', translate) ||
    optionalTranslate('chat.researchTitle', translate) ||
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
  const token = resolveTokenAlias(
    normalizeDeepResearchToken(String(value || '')),
    DEEP_RESEARCH_ACTION_ALIASES
  )
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
  const token = resolveTokenAlias(normalizeDeepResearchToken(trimmed), DEEP_RESEARCH_ACTION_ALIASES)
  const humanized = humanizeDeepResearchToken(trimmed)
  const stopReason = localizeDeepResearchStopReason(trimmed, translate)
  const gap = localizeDeepResearchGap(trimmed, translate)
  const structured = localizeDeepResearchStructuredValue(trimmed, translate)
  return (
    translateKnownToken(token, DEEP_RESEARCH_AUX_STATUS_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_ACTION_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STATUS_KEYS, translate) ||
    (stopReason !== trimmed ? stopReason : '') ||
    (gap !== trimmed ? gap : '') ||
    (structured !== humanized ? structured : '') ||
    humanized
  )
}

export function localizeDeepResearchGap(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const parts = splitDeepResearchSegments(trimmed)
  if (parts.length > 1) {
    return parts.map((part) => localizeDeepResearchGap(part, translate) || part).join(' · ')
  }
  const token = resolveTokenAlias(normalizeDeepResearchToken(trimmed), DEEP_RESEARCH_GAP_ALIASES)
  return translateKnownToken(token, DEEP_RESEARCH_GAP_KEYS, translate) || trimmed
}

export function localizeDeepResearchStructuredValue(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const token = resolveTokenAlias(
    normalizeDeepResearchToken(trimmed),
    DEEP_RESEARCH_TOKEN_ALIASES
  )
  return (
    translateKnownToken(token, DEEP_RESEARCH_TOKEN_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STRUCTURED_VALUE_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_ACTION_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STATUS_KEYS, translate) ||
    humanizeDeepResearchToken(trimmed)
  )
}

export function localizeDeepResearchTimeWindow(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const token = normalizeDeepResearchToken(trimmed)
  return (
    translateKnownToken(token, DEEP_RESEARCH_TOKEN_KEYS, translate) ||
    translateKnownToken(token, DEEP_RESEARCH_STAGE_KEYS, translate) ||
    trimmed
  )
}

export function localizeDeepResearchStopReason(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const token = normalizeDeepResearchToken(trimmed)
  return translateKnownToken(token, DEEP_RESEARCH_STOP_REASON_KEYS, translate) || trimmed
}

export function localizeDeepResearchReportStyle(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''
  const token = normalizeDeepResearchToken(trimmed)
  return (
    translateKnownToken(token, DEEP_RESEARCH_REPORT_STYLE_KEYS, translate) ||
    humanizeDeepResearchToken(trimmed)
  )
}

export function localizeDeepResearchSummary(
  value: string | null | undefined,
  translate: Translate
): string {
  const trimmed = String(value || '').trim()
  if (!trimmed) return ''

  const action = localizeDeepResearchAction(trimmed, translate)
  const humanized = humanizeDeepResearchToken(trimmed)
  if (action !== humanized) {
    return action
  }

  const gap = localizeDeepResearchGap(trimmed, translate)
  if (gap !== trimmed) {
    return gap
  }

  const stopReason = localizeDeepResearchStopReason(trimmed, translate)
  if (stopReason !== trimmed) {
    return stopReason
  }

  const gapCoverageMatch = trimmed.match(DEEP_RESEARCH_SUMMARY_GAP_COVERAGE_RE)
  if (gapCoverageMatch) {
    return formatDeepResearchTemplate(
      translate(
        'chat.deepResearchSummaryGapCoverage',
        '{focus} only covers {evidenceCount} evidence item(s) across {domainCount} domain(s); follow-up research is needed.'
      ),
      {
        focus: resolveDeepResearchFocusLabel(gapCoverageMatch[1] ?? '', translate),
        evidenceCount: gapCoverageMatch[2] ?? '0',
        domainCount: gapCoverageMatch[3] ?? '0',
      }
    )
  }

  const officialGapMatch = trimmed.match(DEEP_RESEARCH_SUMMARY_OFFICIAL_GAP_RE)
  if (officialGapMatch) {
    return formatDeepResearchTemplate(
      translate(
        'chat.deepResearchSummaryOfficialGap',
        '{focus} still lacks stable primary or official sources.'
      ),
      {
        focus: resolveDeepResearchFocusLabel(officialGapMatch[1] ?? '', translate),
      }
    )
  }

  const resolvedCoverageMatch = trimmed.match(DEEP_RESEARCH_SUMMARY_RESOLVED_COVERAGE_RE)
  if (resolvedCoverageMatch) {
    return formatDeepResearchTemplate(
      translate(
        'chat.deepResearchSummaryResolvedCoverage',
        '{focus} is covered by {evidenceCount} evidence item(s) across {domainCount} domain(s).'
      ),
      {
        focus: resolveDeepResearchFocusLabel(resolvedCoverageMatch[1] ?? '', translate),
        evidenceCount: resolvedCoverageMatch[2] ?? '0',
        domainCount: resolvedCoverageMatch[3] ?? '0',
      }
    )
  }

  const domainCoverageMatch = trimmed.match(DEEP_RESEARCH_SUMMARY_DOMAIN_COVERAGE_RE)
  if (domainCoverageMatch) {
    return formatDeepResearchTemplate(
      translate(
        'chat.deepResearchSummaryDomainCoverage',
        'Coverage spans {domainCount} unique domain(s).'
      ),
      {
        domainCount: domainCoverageMatch[1] ?? '0',
      }
    )
  }

  const freshnessCoverageMatch = trimmed.match(DEEP_RESEARCH_SUMMARY_FRESHNESS_COVERAGE_RE)
  if (freshnessCoverageMatch) {
    return formatDeepResearchTemplate(
      translate(
        'chat.deepResearchSummaryFreshnessCoverage',
        'Fresh evidence reaches {year}.'
      ),
      {
        year: freshnessCoverageMatch[1] ?? '0',
      }
    )
  }

  const claimSupportCoverageMatch = trimmed.match(DEEP_RESEARCH_SUMMARY_CLAIM_SUPPORT_COVERAGE_RE)
  if (claimSupportCoverageMatch) {
    return formatDeepResearchTemplate(
      translate(
        'chat.deepResearchSummaryClaimSupportCoverage',
        'Core conclusions are supported across {supportCount} claim group(s).'
      ),
      {
        supportCount: claimSupportCoverageMatch[1] ?? '0',
      }
    )
  }

  if (DEEP_RESEARCH_PRIMARY_SOURCE_VERIFICATION_RE.test(trimmed)) {
    return translate(
      'chat.deepResearchOpenQuestionPrimarySourceVerification',
      'Check primary sources for final verification.'
    )
  }

  const plannedTasksMatch = trimmed.match(DEEP_RESEARCH_PLANNED_TASKS_RE)
  if (plannedTasksMatch) {
    return formatDeepResearchCount(
      translate('chat.deepResearchPlannedTasks', 'Planned tasks'),
      plannedTasksMatch[1] ?? '0'
    )
  }

  const collectedSourcesMatch = trimmed.match(DEEP_RESEARCH_COLLECTED_SOURCES_RE)
  if (collectedSourcesMatch) {
    return formatDeepResearchCount(
      translate('chat.deepResearchLiveSources', 'Live sources'),
      collectedSourcesMatch[1] ?? '0'
    )
  }

  const structured = localizeDeepResearchStructuredValue(trimmed, translate)
  if (structured !== humanized) {
    return structured
  }

  return trimmed
}

export function localizeDeepResearchSourceType(
  value: string | null | undefined,
  translate: Translate
): string {
  return localizeDeepResearchStructuredValue(value, translate)
}

export function splitDeepResearchSegments(value: string | null | undefined): string[] {
  return String(value || '')
    .trim()
    .split(/\s*[•·]\s*/)
    .map((part) => part.trim())
    .filter(Boolean)
}
