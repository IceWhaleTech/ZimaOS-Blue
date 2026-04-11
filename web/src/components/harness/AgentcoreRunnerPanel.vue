<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  settingsApi,
  type AgentcoreRunnerReflectiveCandidate,
  type AgentcoreRunnerTagList,
} from '@/api/settings'
import { useSettingsStore } from '@/stores/settings'

const props = withDefaults(
  defineProps<{
    showRefreshButton?: boolean
    embedded?: boolean
  }>(),
  {
    showRefreshButton: true,
    embedded: false,
  }
)

const emit = defineEmits<{
  (
    e: 'status-change',
    payload: {
      message: string
      tone: 'success' | 'error'
    }
  ): void
}>()

const { t } = useI18n()
const settingsStore = useSettingsStore()

const AGENTCORE_RUNNER_EVOLVABLE_PART_ORDER = [
  'constraints',
  'skill_definition',
  'prompt_template',
  'context_assembly',
  'coordinator_policy',
  'orchestrator_policy',
  'tool_exposure',
  'verification_policy',
  'runner_code',
  'build_recipe',
] as const

const AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_LABELS: Record<string, string> = {
  constraints: 'Constraints',
  skill_definition: 'Skill definition',
  prompt_template: 'Prompt template',
  context_assembly: 'Context assembly',
  coordinator_policy: 'Coordinator',
  orchestrator_policy: 'Orchestrator',
  tool_exposure: 'Tool exposure',
  verification_policy: 'Verification policy',
  runner_code: 'Runner code',
  build_recipe: 'Build recipe',
}

const AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_DESCRIPTIONS: Record<string, string> = {
  constraints: 'Defines the hard limits and guardrails the runner must follow.',
  skill_definition: 'Describes the skill contract, responsibilities, and expected capabilities.',
  prompt_template:
    'Shapes the reusable instructions and response structure sent to the model.',
  context_assembly:
    'Controls how evidence, state, and workspace context are gathered before each run.',
  coordinator_policy:
    'Decides how top-level tasks are broken down, sequenced, and handed off.',
  orchestrator_policy:
    'Governs multi-step flow control, retries, and cross-stage coordination.',
  tool_exposure:
    'Chooses which tools are available to the runner and how they are presented.',
  verification_policy:
    'Defines how outputs are checked before they are accepted or persisted.',
  runner_code: 'Implements the runtime logic that executes the agent loop and integrations.',
  build_recipe: 'Specifies how the runner is prepared, built, and packaged for execution.',
}

const AGENTCORE_RUNNER_EVOLVABLE_PART_ACTIVE_TOOLTIP_FALLBACK =
  'This part was optimized in the current candidate'
const AGENTCORE_RUNNER_EVOLVABLE_PART_INACTIVE_TOOLTIP_FALLBACK =
  'This part can participate in self-evolution, but the current version did not change it'
const AGENTCORE_RUNNER_PARETO_OBJECTIVES = [
  {
    key: 'execution_pass_rate_delta',
    label: 'Execution pass rate',
    shortLabel: 'Exec',
    goal: 'maximize' as const,
  },
  {
    key: 'verification_pass_rate_delta',
    label: 'Verification pass rate',
    shortLabel: 'Verify',
    goal: 'maximize' as const,
  },
  {
    key: 'evidence_backed_pass_rate_delta',
    label: 'Evidence-backed pass rate',
    shortLabel: 'Evidence',
    goal: 'maximize' as const,
  },
  {
    key: 'median_latency_increase_rate',
    label: 'Latency increase',
    shortLabel: 'Latency',
    goal: 'minimize' as const,
  },
  {
    key: 'repeat_failure_recurrence',
    label: 'Repeat failure recurrence',
    shortLabel: 'Failure',
    goal: 'minimize' as const,
  },
] as const

const agentcoreRunnerSaving = ref(false)
const agentcoreRunnerPreparing = ref(false)
const agentcoreRunnerRefreshing = ref(false)
const agentcoreRunnerSourceExpanded = ref(false)
const agentcoreRunnerStatusExpanded = ref(false)
const agentcoreRunnerRepoURL = ref('')
const agentcoreRunnerRef = ref('')
const agentcoreRunnerTags = ref<AgentcoreRunnerTagList | null>(null)
const agentcoreRunnerTagsLoading = ref(false)
const agentcoreRunnerTranscriptExpanded = ref(false)
const agentcoreRunnerTranscriptPreviewCount = 2
const agentcoreRunnerRootClass = computed(() =>
  props.embedded
    ? 'agentcore-runner-panel agentcore-runner-panel--embedded'
    : 'agentcore-runner-panel dashboard-card-surface'
)
const agentcoreRunnerBodyClass = computed(() =>
  props.embedded
    ? 'agentcore-runner-panel__body agentcore-runner-panel__body--embedded space-y-4'
    : 'dashboard-card-subsurface agentcore-runner-panel__body p-4 space-y-4'
)
const agentcoreRunnerSectionCardClass = computed(() =>
  props.embedded
    ? 'rounded-2xl bg-slate-50/85 p-4 ring-1 ring-slate-200/80 dark:bg-slate-900/55 dark:ring-slate-800/80'
    : 'rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-gray-700 dark:bg-slate-900/60'
)
const agentcoreRunnerLastRunCardClass = computed(() =>
  props.embedded
    ? 'mt-2 space-y-2 rounded-xl bg-white/90 p-3 ring-1 ring-slate-200/80 dark:bg-slate-950/60 dark:ring-slate-800/80'
    : 'mt-2 space-y-2 rounded-lg border border-gray-200 bg-white/80 p-3 dark:border-gray-700 dark:bg-slate-950/50'
)

const agentcoreRunnerStatus = computed(() => settingsStore.agentcoreRunnerStatus)
const agentcoreRunnerLastRun = computed(() => settingsStore.agentcoreRunnerLastRun)
const agentcoreRunnerEnabled = computed(() => settingsStore.experimentalAgentcoreRunnerEnabled)
const agentcoreRunnerLastError = computed(() =>
  normalizeAgentcoreRunnerStatusError(agentcoreRunnerStatus.value?.last_error)
)
const agentcoreRunnerHasLastError = computed(() => agentcoreRunnerLastError.value !== '')
const agentcoreRunnerRefOptions = computed(() => {
  const options: string[] = []
  const seen = new Set<string>()
  const push = (value: unknown) => {
    const normalized = normalizeAgentcoreRunnerRefValue(value)
    if (seen.has(normalized)) return
    seen.add(normalized)
    options.push(normalized)
  }
  push(agentcoreRunnerTags.value?.default_ref)
  push(agentcoreRunnerRef.value)
  for (const tag of agentcoreRunnerTags.value?.tags ?? []) {
    push(tag)
  }
  return options
})
const agentcoreRunnerBusy = computed(
  () =>
    agentcoreRunnerSaving.value ||
    agentcoreRunnerPreparing.value ||
    agentcoreRunnerRefreshing.value
)
const agentcoreRunnerSourceRepoSummary = computed(() =>
  formatAgentcoreRunnerRepoSummary(agentcoreRunnerRepoURL.value)
)
const agentcoreRunnerSourceRefSummary = computed(() =>
  normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
)
const agentcoreRunnerLastRunMeta = computed(() => {
  const run = agentcoreRunnerLastRun.value
  if (run == null) return [] as string[]
  return [
    normalizeEvidenceText(run.reason),
    normalizeEvidenceText(run.candidate_id),
    normalizeEvidenceText(run.eval_run_id),
    typeof run.runner_protocol === 'string' ? run.runner_protocol.trim() : '',
    typeof run.runner_stop_reason === 'string' ? run.runner_stop_reason.trim() : '',
    typeof run.optimization_surface === 'string' ? run.optimization_surface.trim() : '',
    formatDurationMs(run.runner_duration_ms),
  ].filter(Boolean)
})
const agentcoreRunnerLastRunTranscriptEntries = computed(() => {
  const entries = agentcoreRunnerLastRun.value?.runner_transcript ?? []
  const suppressed = new Set(
    [
      normalizeEvidenceText(agentcoreRunnerLastRun.value?.runner_response_text),
      normalizeEvidenceText(agentcoreRunnerLastRun.value?.runner_error),
    ].filter(Boolean)
  )
  const seen = new Set<string>()
  return entries.filter((entry) => {
    const text = normalizeEvidenceText(entry.text)
    if (!text) return false
    if (suppressed.has(text)) return false
    const fingerprint = [
      normalizeEvidenceText(entry.direction),
      normalizeEvidenceText(entry.method),
      text,
    ].join('|')
    if (seen.has(fingerprint)) return false
    seen.add(fingerprint)
    return true
  })
})
const agentcoreRunnerVisibleTranscriptEntries = computed(() => {
  if (agentcoreRunnerTranscriptExpanded.value) {
    return agentcoreRunnerLastRunTranscriptEntries.value
  }
  return agentcoreRunnerLastRunTranscriptEntries.value.slice(0, agentcoreRunnerTranscriptPreviewCount)
})
const agentcoreRunnerHiddenTranscriptCount = computed(() =>
  Math.max(
    0,
    agentcoreRunnerLastRunTranscriptEntries.value.length - agentcoreRunnerTranscriptPreviewCount
  )
)
const agentcoreRunnerParetoFrontier = computed(() => {
  const frontier = agentcoreRunnerLastRun.value?.pareto_frontier
  if (!Array.isArray(frontier)) return [] as string[]
  return frontier.map((item) => normalizeEvidenceText(item)).filter(Boolean)
})
const agentcoreRunnerSelectedCandidateID = computed(() =>
  normalizeEvidenceText(agentcoreRunnerLastRun.value?.selected_candidate?.candidate_id)
)
const agentcoreRunnerSelectedCandidateMeta = computed(() => {
  const candidate = agentcoreRunnerLastRun.value?.selected_candidate
  if (candidate == null) return [] as string[]
  const hardPass =
    typeof candidate.hard_pass === 'boolean'
      ? candidate.hard_pass
        ? 'Hard pass'
        : 'Did not hard pass'
      : ''
  const gate = normalizeEvidenceText(candidate.followup_gate)
  const state = normalizeEvidenceText(candidate.followup_state).replace(/_/g, ' ')
  const diffSize = normalizeEvidenceNumber(candidate.diff_size)
  return [
    hardPass,
    gate ? `Gate: ${gate}` : '',
    state ? `State: ${state}` : '',
    diffSize != null ? `Diff size: ${diffSize}` : '',
  ].filter(Boolean)
})
const agentcoreRunnerSelectedCandidateObjectives = computed(() => {
  const candidate = agentcoreRunnerLastRun.value?.selected_candidate
  if (candidate == null || candidate.objectives == null) {
    return [] as Array<{ key: string; label: string; goalLabel: string; value: string }>
  }
  return AGENTCORE_RUNNER_PARETO_OBJECTIVES.flatMap((metric) => {
    const rawValue = normalizeEvidenceNumber(candidate.objectives?.[metric.key])
    if (rawValue == null) return []
    return [
      {
        key: metric.key,
        label: metric.label,
        goalLabel: metric.goal === 'maximize' ? 'Higher is better' : 'Lower is better',
        value: formatParetoObjectiveValue(metric.key, rawValue),
      },
    ]
  })
})
const agentcoreRunnerParetoCandidateSummaries = computed(() => {
  const run = agentcoreRunnerLastRun.value
  if (run == null) return [] as Array<{ id: string; selected: boolean; summary: string }>
  const candidates = new Map<string, NonNullable<typeof run.selected_candidate>>()
  for (const candidate of run.evaluated_candidates ?? []) {
    const candidateID = normalizeEvidenceText(candidate?.candidate_id)
    if (!candidateID) continue
    candidates.set(candidateID, candidate)
  }
  const selectedCandidate = run.selected_candidate
  if (selectedCandidate?.candidate_id) {
    candidates.set(normalizeEvidenceText(selectedCandidate.candidate_id), selectedCandidate)
  }
  return agentcoreRunnerParetoFrontier.value.flatMap((candidateID) => {
    const candidate = candidates.get(candidateID)
    const summary = formatParetoCandidateSummary(candidate?.objectives)
    if (!summary) return []
    return [
      {
        id: candidateID,
        selected: candidateID === agentcoreRunnerSelectedCandidateID.value,
        summary,
      },
    ]
  })
})
const agentcoreRunnerEvaluatedCandidateCards = computed(() => {
  const run = agentcoreRunnerLastRun.value
  if (run == null) {
    return [] as Array<{
      id: string
      selected: boolean
      frontier: boolean
      meta: string[]
      objectiveSummary: string
      followupSummary: string
    }>
  }
  const frontierSet = new Set(agentcoreRunnerParetoFrontier.value)
  const seen = new Set<string>()
  const cards: Array<{
    id: string
    selected: boolean
    frontier: boolean
    meta: string[]
    objectiveSummary: string
    followupSummary: string
    rank: number
    index: number
  }> = []

  const pushCandidate = (candidate: AgentcoreRunnerReflectiveCandidate | undefined, index: number) => {
    if (candidate == null) return
    const candidateID = normalizeEvidenceText(candidate.candidate_id)
    if (!candidateID || seen.has(candidateID)) return
    seen.add(candidateID)

    const selected = candidateID === agentcoreRunnerSelectedCandidateID.value
    const frontier = frontierSet.has(candidateID)
    const hardPass =
      typeof candidate.hard_pass === 'boolean'
        ? candidate.hard_pass
          ? 'Hard pass'
          : 'Did not hard pass'
        : ''
    const gate = normalizeEvidenceText(candidate.followup_gate)
    const state = normalizeEvidenceText(candidate.followup_state).replace(/_/g, ' ')
    const diffSize = normalizeEvidenceNumber(candidate.diff_size)
    const objectiveSummary = formatParetoCandidateSummary(candidate.objectives)
    cards.push({
      id: candidateID,
      selected,
      frontier,
      meta: [
        hardPass,
        gate ? `Gate: ${gate}` : '',
        state ? `State: ${state}` : '',
        diffSize != null ? `Diff size: ${diffSize}` : '',
      ].filter(Boolean),
      objectiveSummary,
      followupSummary: normalizeEvidenceText(candidate.followup_summary),
      rank: selected ? 0 : frontier ? 1 : candidate.hard_pass ? 2 : 3,
      index,
    })
  }

  ;(run.evaluated_candidates ?? []).forEach((candidate, index) => {
    pushCandidate(candidate, index)
  })
  pushCandidate(run.selected_candidate, (run.evaluated_candidates ?? []).length)

  return cards
    .sort((left, right) => left.rank - right.rank || left.index - right.index || left.id.localeCompare(right.id))
    .map(({ rank, index, ...card }) => card)
})
const agentcoreRunnerOfflineValueSummary = computed(() =>
  normalizeEvidenceText(agentcoreRunnerLastRun.value?.offline_value_report?.value_summary)
)
const agentcoreRunnerRuntimeValueSummary = computed(() =>
  normalizeEvidenceText(agentcoreRunnerLastRun.value?.runtime_value_report?.value_summary)
)
const agentcoreRunnerOfflineValueMeta = computed(() => {
  const report = agentcoreRunnerLastRun.value?.offline_value_report
  const recommendation = normalizeEvidenceText(
    report?.offline_recommendation ?? agentcoreRunnerLastRun.value?.offline_recommendation
  ).replace(/_/g, ' ')
  const confidence = normalizeEvidenceText(report?.confidence).replace(/_/g, ' ')
  return [
    recommendation ? `Recommendation: ${recommendation}` : '',
    confidence ? `Confidence: ${confidence}` : '',
  ].filter(Boolean)
})
const agentcoreRunnerRuntimeValueMeta = computed(() => {
  const report = agentcoreRunnerLastRun.value?.runtime_value_report
  if (report == null) return [] as string[]
  const status = normalizeEvidenceText(
    report.status ?? agentcoreRunnerLastRun.value?.runtime_status
  ).replace(/_/g, ' ')
  const confidence = normalizeEvidenceText(report.confidence).replace(/_/g, ' ')
  const beforeSamples = normalizeEvidenceNumber(report.before_sample_count)
  const afterSamples = normalizeEvidenceNumber(report.after_sample_count)
  return [
    status ? `Status: ${status}` : '',
    confidence ? `Confidence: ${confidence}` : '',
    beforeSamples != null ? `${beforeSamples} before` : '',
    afterSamples != null ? `${afterSamples} after` : '',
  ].filter(Boolean)
})
const agentcoreRunnerRuntimeMetrics = computed(() => {
  const report = agentcoreRunnerLastRun.value?.runtime_value_report
  if (report == null) return [] as Array<{ label: string; value: string; tone: 'positive' | 'negative' | 'neutral' }>
  const metrics: Array<{ label: string; value: string; tone: 'positive' | 'negative' | 'neutral' }> = []
  const failureRecurrenceDelta = normalizeEvidenceNumber(report.failure_recurrence_delta)
  if (failureRecurrenceDelta != null) {
    metrics.push({
      label: 'Failure recurrence',
      value: formatSignedNumber(failureRecurrenceDelta),
      tone: failureRecurrenceDelta < 0 ? 'positive' : failureRecurrenceDelta > 0 ? 'negative' : 'neutral',
    })
  }
  const durationDelta = normalizeEvidenceNumber(report.median_duration_delta_rate)
  if (durationDelta != null) {
    metrics.push({
      label: 'Duration',
      value: formatSignedPercent(durationDelta),
      tone: durationDelta < 0 ? 'positive' : durationDelta > 0 ? 'negative' : 'neutral',
    })
  }
  const tokenDelta = normalizeEvidenceNumber(report.median_total_tokens_delta_rate)
  if (tokenDelta != null) {
    metrics.push({
      label: 'Token usage',
      value: formatSignedPercent(tokenDelta),
      tone: tokenDelta < 0 ? 'positive' : tokenDelta > 0 ? 'negative' : 'neutral',
    })
  }
  const captureQualityDelta = normalizeEvidenceNumber(report.capture_quality_delta)
  if (captureQualityDelta != null) {
    metrics.push({
      label: 'Capture quality',
      value: formatSignedNumber(captureQualityDelta),
      tone: captureQualityDelta > 0 ? 'positive' : captureQualityDelta < 0 ? 'negative' : 'neutral',
    })
  }
  const validationQualityDelta = normalizeEvidenceNumber(report.validation_quality_delta)
  if (validationQualityDelta != null) {
    metrics.push({
      label: 'Validation quality',
      value: formatSignedNumber(validationQualityDelta),
      tone: validationQualityDelta > 0 ? 'positive' : validationQualityDelta < 0 ? 'negative' : 'neutral',
    })
  }
  return metrics
})
const agentcoreRunnerProposalCandidateIDs = computed(() => {
  const proposals = agentcoreRunnerLastRun.value?.proposal_set
  if (!Array.isArray(proposals)) return [] as string[]
  return proposals
    .map((proposal) => normalizeEvidenceText(proposal?.candidate_id))
    .filter(Boolean)
})
const agentcoreRunnerOfflineImprovements = computed(() => {
  const improvements = agentcoreRunnerLastRun.value?.offline_value_report?.top_improvements
  if (!Array.isArray(improvements)) return [] as string[]
  return improvements.map((item) => normalizeEvidenceText(item)).filter(Boolean)
})
const agentcoreRunnerOfflineTradeoffs = computed(() => {
  const tradeoffs = agentcoreRunnerLastRun.value?.offline_value_report?.top_tradeoffs
  if (!Array.isArray(tradeoffs)) return [] as string[]
  return tradeoffs.map((item) => normalizeEvidenceText(item)).filter(Boolean)
})
const agentcoreRunnerRuntimeImprovements = computed(() => {
  const improvements = agentcoreRunnerLastRun.value?.runtime_value_report?.top_improvements
  if (!Array.isArray(improvements)) return [] as string[]
  return improvements.map((item) => normalizeEvidenceText(item)).filter(Boolean)
})
const agentcoreRunnerRuntimeTradeoffs = computed(() => {
  const tradeoffs = agentcoreRunnerLastRun.value?.runtime_value_report?.top_tradeoffs
  if (!Array.isArray(tradeoffs)) return [] as string[]
  return tradeoffs.map((item) => normalizeEvidenceText(item)).filter(Boolean)
})
const agentcoreRunnerSampleEfficiencySummary = computed(() => {
  const report = agentcoreRunnerLastRun.value?.sample_efficiency_report
  if (report == null || typeof report !== 'object') return ''
  const proposalCount =
    typeof report.proposal_count === 'number' ? report.proposal_count : null
  const evaluatedCount =
    typeof report.evaluated_candidate_count === 'number' ? report.evaluated_candidate_count : null
  const hardPassCount =
    typeof report.hard_pass_candidate_count === 'number' ? report.hard_pass_candidate_count : null
  const frontierSize =
    typeof report.pareto_frontier_size === 'number'
      ? report.pareto_frontier_size
      : typeof report.frontier_size === 'number'
        ? report.frontier_size
        : null
  const maxEvaluations =
    typeof report.max_evaluations === 'number' ? report.max_evaluations : null
  const parts: string[] = []
  if (proposalCount != null) parts.push(`Generated ${proposalCount} proposals`)
  if (evaluatedCount != null) parts.push(`Evaluated ${evaluatedCount} candidates`)
  if (hardPassCount != null) parts.push(`${hardPassCount} hard-pass`)
  if (frontierSize != null) parts.push(`frontier ${frontierSize}`)
  if (maxEvaluations != null) parts.push(`budget ${maxEvaluations}`)
  return parts.join(' · ')
})
const agentcoreRunnerSupportedParts = computed(() => {
  const configured = new Set(
    (agentcoreRunnerStatus.value?.supported_parts ?? [])
      .map((part) => normalizeEvidenceText(part))
      .filter(Boolean)
  )
  if (configured.size === 0) {
    return [...AGENTCORE_RUNNER_EVOLVABLE_PART_ORDER]
  }
  return AGENTCORE_RUNNER_EVOLVABLE_PART_ORDER.filter((part) => configured.has(part))
})
const agentcoreRunnerOptimizedParts = computed(() => {
  const parts = (agentcoreRunnerStatus.value?.optimized_parts ?? [])
  return new Set(parts.map((part) => normalizeEvidenceText(part)).filter(Boolean))
})
const agentcoreRunnerOptimizedPartList = computed(() =>
  agentcoreRunnerSupportedParts.value.filter((part) => agentcoreRunnerOptimizedParts.value.has(part))
)
const agentcoreRunnerOptimizedSummary = computed(() =>
  agentcoreRunnerOptimizedPartList.value.map((part) => translateAgentcoreRunnerPart(part)).join(', ')
)
const agentcoreRunnerPrimaryPart = computed(() =>
  normalizeEvidenceText(agentcoreRunnerStatus.value?.primary_part)
)
const agentcoreRunnerPrimaryPartLabel = computed(() =>
  agentcoreRunnerPrimaryPart.value ? translateAgentcoreRunnerPart(agentcoreRunnerPrimaryPart.value) : ''
)
const agentcoreRunnerSourceOptimizationRunID = computed(() =>
  normalizeEvidenceText(agentcoreRunnerStatus.value?.source_optimization_run_id)
)
const agentcoreRunnerEvolvablePartBadges = computed(() =>
  agentcoreRunnerSupportedParts.value.map((part) => ({
    part,
    label: translateAgentcoreRunnerPart(part),
    active: agentcoreRunnerOptimizedParts.value.has(part),
    tooltip: buildAgentcoreRunnerPartTooltip(part, agentcoreRunnerOptimizedParts.value.has(part)),
  }))
)

watch(
  [
    () => settingsStore.experimentalAgentcoreRunnerRepoURL,
    () => settingsStore.experimentalAgentcoreRunnerRef,
  ],
  ([repoURL, refValue]) => {
    agentcoreRunnerRepoURL.value = repoURL
    agentcoreRunnerRef.value = normalizeAgentcoreRunnerRefValue(refValue)
  },
  { immediate: true }
)

watch(
  () => agentcoreRunnerLastRun.value?.id,
  () => {
    agentcoreRunnerTranscriptExpanded.value = false
  }
)

onMounted(() => {
  void ensureLoaded()
})

function emitStatus(message: string, tone: 'success' | 'error') {
  emit('status-change', { message, tone })
}

function formatStatusTime(value?: string) {
  if (!value) return t('settings.agentcoreRunner.empty', 'Not available')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function formatDurationMs(value?: number) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return ''
  if (value < 1000) return `${Math.round(value)}ms`
  return `${(value / 1000).toFixed(1)}s`
}

function normalizeEvidenceText(value: unknown) {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function normalizeEvidenceNumber(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function formatSignedNumber(value: number, digits = 2) {
  if (!Number.isFinite(value)) return ''
  const normalized = value.toFixed(digits)
  return value > 0 ? `+${normalized}` : normalized
}

function formatSignedPercent(value: number) {
  if (!Number.isFinite(value)) return ''
  const percentage = Math.round(value * 100)
  return percentage > 0 ? `+${percentage}%` : `${percentage}%`
}

function formatUnsignedNumber(value: number, digits = 2) {
  if (!Number.isFinite(value)) return ''
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  }).format(value)
}

function formatParetoObjectiveValue(key: string, value: number) {
  if (key === 'median_latency_increase_rate') {
    return formatSignedPercent(value)
  }
  if (key.endsWith('_delta')) {
    return formatSignedNumber(value)
  }
  return formatUnsignedNumber(value)
}

function formatParetoCandidateSummary(objectives: Record<string, unknown> | undefined) {
  if (objectives == null) return ''
  const parts = AGENTCORE_RUNNER_PARETO_OBJECTIVES.flatMap((metric) => {
    const rawValue = normalizeEvidenceNumber(objectives[metric.key])
    if (rawValue == null) return []
    return [`${metric.shortLabel} ${formatParetoObjectiveValue(metric.key, rawValue)}`]
  })
  return parts.join(' · ')
}

function normalizeAgentcoreRunnerStatusError(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text.toLowerCase() === 'repo url is required') {
    return ''
  }
  return text
}

function translateAgentcoreRunnerPart(part: string) {
  return t(
    `settings.agentcoreRunner.parts.${part}`,
    AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_LABELS[part] ?? part
  )
}

function translateAgentcoreRunnerPartDescription(part: string) {
  return t(
    `settings.agentcoreRunner.partDescriptions.${part}`,
    AGENTCORE_RUNNER_EVOLVABLE_PART_FALLBACK_DESCRIPTIONS[part] ?? ''
  )
}

function buildAgentcoreRunnerPartTooltip(part: string, active: boolean) {
  const statusText = active
    ? t(
        'settings.agentcoreRunner.activePartTooltip',
        AGENTCORE_RUNNER_EVOLVABLE_PART_ACTIVE_TOOLTIP_FALLBACK
      )
    : t(
        'settings.agentcoreRunner.inactivePartTooltip',
        AGENTCORE_RUNNER_EVOLVABLE_PART_INACTIVE_TOOLTIP_FALLBACK
      )

  return [
    translateAgentcoreRunnerPart(part),
    translateAgentcoreRunnerPartDescription(part),
    statusText,
  ]
    .filter(Boolean)
    .join('\n')
}

function normalizeAgentcoreRunnerRepoURLValue(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text) return text
  return 'https://github.com/IceWhaleTech/ZimaOS-Blue'
}

function normalizeAgentcoreRunnerRefValue(value: unknown) {
  const text = normalizeEvidenceText(value)
  if (text) return text
  return 'main'
}

function formatAgentcoreRunnerRepoSummary(value: unknown) {
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(value)
  return repoURL
    .replace(/^https?:\/\/github\.com\//i, '')
    .replace(/\.git$/i, '')
    .replace(/\/+$/, '')
}

async function fetchAgentcoreRunnerStatus() {
  try {
    await settingsStore.fetchAgentcoreRunnerStatus()
  } catch {
    // Ignore card bootstrap errors and keep the panel interactive.
  }
}

async function fetchAgentcoreRunnerLastRun() {
  try {
    await settingsStore.fetchAgentcoreRunnerLastRun()
  } catch {
    // Ignore card bootstrap errors and keep the panel interactive.
  }
}

async function fetchAgentcoreRunnerTags(repoURL = agentcoreRunnerRepoURL.value) {
  const resolvedRepoURL = normalizeAgentcoreRunnerRepoURLValue(repoURL)
  try {
    agentcoreRunnerTagsLoading.value = true
    const response = await settingsApi.getAgentcoreRunnerTags(resolvedRepoURL)
    agentcoreRunnerTags.value = response.data
  } catch {
    agentcoreRunnerTags.value = {
      repo_url: resolvedRepoURL,
      default_ref: 'main',
      tags: [],
    }
  } finally {
    agentcoreRunnerTagsLoading.value = false
  }
}

async function ensureLoaded() {
  const tasks: Promise<unknown>[] = []
  if (settingsStore.agentcoreRunnerStatus == null) {
    tasks.push(fetchAgentcoreRunnerStatus())
  } else if (
    settingsStore.agentcoreRunnerStatus.last_optimization_run_id &&
    settingsStore.agentcoreRunnerLastRun == null
  ) {
    tasks.push(fetchAgentcoreRunnerLastRun())
  }
  tasks.push(fetchAgentcoreRunnerTags())
  await Promise.allSettled(tasks)
}

async function refreshAgentcoreRunnerStatus() {
  if (agentcoreRunnerRefreshing.value) return
  try {
    agentcoreRunnerRefreshing.value = true
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags()])
  } finally {
    agentcoreRunnerRefreshing.value = false
  }
}

async function saveAgentcoreRunnerConfig() {
  if (agentcoreRunnerSaving.value) return
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(agentcoreRunnerRepoURL.value)
  const refValue = normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
  agentcoreRunnerRepoURL.value = repoURL
  agentcoreRunnerRef.value = refValue
  try {
    agentcoreRunnerSaving.value = true
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_repo_url: repoURL,
      experimental_agentcore_runner_ref: refValue,
    })
    emitStatus(t('settings.saved', 'Saved'), 'success')
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags(repoURL)])
  } catch {
    emitStatus(t('settings.saveFailed', 'Failed to save configuration'), 'error')
  } finally {
    agentcoreRunnerSaving.value = false
  }
}

async function handleAgentcoreRunnerEnabledChange(next: boolean) {
  if (agentcoreRunnerSaving.value) return
  try {
    agentcoreRunnerSaving.value = true
    await settingsStore.setExperimentalAgentcoreRunnerEnabled(next)
    emitStatus(t('settings.saved', 'Saved'), 'success')
    await fetchAgentcoreRunnerStatus()
  } catch {
    emitStatus(t('settings.saveFailed', 'Failed to save configuration'), 'error')
  } finally {
    agentcoreRunnerSaving.value = false
  }
}

async function prepareAgentcoreRunner() {
  if (agentcoreRunnerPreparing.value) return
  const repoURL = normalizeAgentcoreRunnerRepoURLValue(agentcoreRunnerRepoURL.value)
  const refValue = normalizeAgentcoreRunnerRefValue(agentcoreRunnerRef.value)
  agentcoreRunnerRepoURL.value = repoURL
  agentcoreRunnerRef.value = refValue
  try {
    agentcoreRunnerPreparing.value = true
    await settingsStore.updateBackendSettings({
      experimental_agentcore_runner_repo_url: repoURL,
      experimental_agentcore_runner_ref: refValue,
    })
    await settingsStore.prepareAgentcoreRunner()
    await Promise.allSettled([fetchAgentcoreRunnerStatus(), fetchAgentcoreRunnerTags(repoURL)])
    emitStatus(
      t('settings.agentcoreRunner.prepareSuccess', 'Agentcore Runner prepared successfully'),
      'success'
    )
  } catch {
    emitStatus(
      t('settings.agentcoreRunner.prepareFailed', 'Failed to prepare Agentcore Runner'),
      'error'
    )
  } finally {
    agentcoreRunnerPreparing.value = false
  }
}
</script>

<template>
  <section :class="agentcoreRunnerRootClass" data-testid="agentcore-runner-card">
    <div v-if="!embedded" class="space-y-1.5">
      <span class="settings-module__eyebrow agentcore-runner-panel__eyebrow inline-flex w-fit">{{
        t('settings.agentcoreRunner.eyebrow', 'Harness · Beta')
      }}</span>
      <h2 class="settings-module__title agentcore-runner-panel__title">
        {{
          t(
            'settings.agentcoreRunner.title',
            'Harness Self-Iterating Agentcore Runner'
          )
        }}
      </h2>
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{
          t(
            'settings.agentcoreRunner.description',
            'Use Harness to iterate on an Agentcore runner by preparing a standalone runner from a public GitHub repo for local build, evaluation, and optimisation.'
          )
        }}
      </p>
    </div>

    <div :class="agentcoreRunnerBodyClass">
      <div class="settings-field-card__row flex items-start justify-between gap-3">
        <div class="settings-card-heading min-w-0">
          <label class="settings-field-label block font-medium text-gray-900 dark:text-gray-100">{{
            t('settings.agentcoreRunner.enabled', 'Enable Agentcore Runner')
          }}</label>
          <p class="settings-field-hint mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{
              t(
                'settings.agentcoreRunner.enabledHint',
                'Allow Harness beta flows to prepare and reuse a managed local runner for self-iteration.'
              )
            }}
          </p>
        </div>
        <button
          data-testid="agentcore-runner-enabled-switch"
          type="button"
          role="switch"
          :aria-checked="agentcoreRunnerEnabled"
          :disabled="agentcoreRunnerBusy"
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-gray-400 focus:ring-offset-2 disabled:opacity-50"
          :class="
            agentcoreRunnerEnabled
              ? 'bg-green-600 dark:bg-green-500'
              : 'bg-gray-300 dark:bg-gray-600'
          "
          @click="handleAgentcoreRunnerEnabledChange(!agentcoreRunnerEnabled)"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="agentcoreRunnerEnabled ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>

      <div
        :class="agentcoreRunnerSectionCardClass"
      >
        <button
          data-testid="agentcore-runner-source-toggle"
          type="button"
          :aria-expanded="agentcoreRunnerSourceExpanded ? 'true' : 'false'"
          class="flex w-full items-center justify-between gap-3 text-left"
          @click="agentcoreRunnerSourceExpanded = !agentcoreRunnerSourceExpanded"
        >
          <div class="min-w-0">
            <div class="text-sm font-medium text-gray-800 dark:text-gray-100">
              {{ t('settings.agentcoreRunner.source', 'GitHub Repo & Ref') }}
            </div>
            <div class="mt-2 flex flex-wrap gap-1.5">
              <span
                class="max-w-full break-all rounded-lg border border-blue-200 bg-blue-50 px-2.5 py-1 text-[11px] font-medium text-blue-700 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200"
              >
                {{ agentcoreRunnerSourceRepoSummary }}
              </span>
              <span
                class="rounded-full border border-gray-200 bg-white px-2.5 py-1 text-[11px] font-medium text-gray-700 dark:border-gray-700 dark:bg-slate-950/60 dark:text-gray-200"
              >
                {{ agentcoreRunnerSourceRefSummary }}
              </span>
            </div>
          </div>
          <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{
            agentcoreRunnerSourceExpanded
              ? t('settings.smallModel.collapse', 'Collapse')
              : t('settings.smallModel.expand', 'Expand')
          }}</span>
        </button>

        <div
          v-if="agentcoreRunnerSourceExpanded"
          data-testid="agentcore-runner-source-content"
          class="mt-3"
        >
          <div class="grid gap-3 md:grid-cols-[minmax(0,1fr),180px]">
            <label class="block">
              <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('settings.agentcoreRunner.repoUrl', 'GitHub Repo URL') }}
              </span>
              <input
                data-testid="agentcore-runner-repo-input"
                v-model="agentcoreRunnerRepoURL"
                type="text"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-green-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-100"
                :placeholder="
                  t(
                    'settings.agentcoreRunner.repoPlaceholder',
                    'https://github.com/owner/repo or owner/repo'
                  )
                "
                @blur="saveAgentcoreRunnerConfig"
              />
            </label>

            <label class="block">
              <span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ t('settings.agentcoreRunner.ref', 'Ref') }}
              </span>
              <select
                data-testid="agentcore-runner-ref-input"
                v-model="agentcoreRunnerRef"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 outline-none transition focus:border-green-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-100"
                :disabled="agentcoreRunnerSaving"
                @change="saveAgentcoreRunnerConfig"
              >
                <option v-for="option in agentcoreRunnerRefOptions" :key="option" :value="option">
                  {{ option }}
                </option>
              </select>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{
                  agentcoreRunnerTagsLoading
                    ? t('settings.agentcoreRunner.refLoading', 'Loading tags...')
                    : t(
                        'settings.agentcoreRunner.refHint',
                        'Defaults to main and lists tags from the selected repo.'
                      )
                }}
              </p>
            </label>
          </div>
        </div>
      </div>

      <div class="agentcore-runner-panel__actions flex items-center justify-between gap-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{
            t(
              'settings.agentcoreRunner.prepareHint',
              'Prepare downloads the repo, installs the required Go toolchain, and builds ./cmd/agentcore-runner in the managed cache.'
            )
          }}
        </p>
        <div class="flex items-center gap-2">
          <button
            v-if="showRefreshButton"
            data-testid="agentcore-runner-refresh"
            type="button"
            class="rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-slate-800"
            :disabled="agentcoreRunnerBusy"
            @click="refreshAgentcoreRunnerStatus"
          >
            {{ t('common.refresh', 'Refresh') }}
          </button>
          <button
            data-testid="agentcore-runner-prepare"
            type="button"
            class="rounded-lg bg-green-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-green-500 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="agentcoreRunnerBusy"
            @click="prepareAgentcoreRunner"
          >
            {{
              agentcoreRunnerPreparing
                ? t('settings.agentcoreRunner.preparing', 'Preparing...')
                : t('settings.agentcoreRunner.prepare', 'Prepare Runner')
            }}
          </button>
        </div>
      </div>

      <div :class="agentcoreRunnerSectionCardClass">
        <button
          data-testid="agentcore-runner-status-toggle"
          type="button"
          :aria-expanded="agentcoreRunnerStatusExpanded ? 'true' : 'false'"
          class="flex w-full items-center justify-between gap-3 text-left"
          @click="agentcoreRunnerStatusExpanded = !agentcoreRunnerStatusExpanded"
        >
          <div class="text-sm font-medium text-gray-800 dark:text-gray-100">
            {{ t('settings.agentcoreRunner.status', 'Status') }}
          </div>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{
            agentcoreRunnerStatusExpanded
              ? t('settings.smallModel.collapse', 'Collapse')
              : t('settings.smallModel.expand', 'Expand')
          }}</span>
        </button>
        <div
          v-if="agentcoreRunnerStatusExpanded"
          data-testid="agentcore-runner-status-content"
          class="mt-3 grid gap-2 sm:grid-cols-2"
        >
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.resolvedCommit', 'Resolved commit')
            }}</span>
            <div data-testid="agentcore-runner-resolved-commit" class="break-all text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.resolved_commit ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.requiredGoVersion', 'Required Go version')
            }}</span>
            <div data-testid="agentcore-runner-required-go" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.required_go_version ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.installedGoVersion', 'Installed Go version')
            }}</span>
            <div data-testid="agentcore-runner-installed-go" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.installed_go_version ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.toolchainReady', 'Toolchain ready')
            }}</span>
            <div data-testid="agentcore-runner-toolchain-ready" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.toolchain_ready
                  ? t('common.yes', 'Yes')
                  : t('common.no', 'No')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.binaryReady', 'Binary ready')
            }}</span>
            <div data-testid="agentcore-runner-binary-ready" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.binary_ready
                  ? t('common.yes', 'Yes')
                  : t('common.no', 'No')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastPrepareState', 'Last prepare state')
            }}</span>
            <div data-testid="agentcore-runner-last-prepare-state" class="text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.last_prepare_state ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.binaryPath', 'Binary path')
            }}</span>
            <div data-testid="agentcore-runner-binary-path" class="break-all text-gray-900 dark:text-gray-100">
              {{
                agentcoreRunnerStatus?.binary_path ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.binaryChecksum', 'Binary checksum')
            }}</span>
            <div
              data-testid="agentcore-runner-binary-checksum"
              class="break-all text-gray-900 dark:text-gray-100"
            >
              {{
                agentcoreRunnerStatus?.binary_sha256 ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastPrepareAt', 'Last prepare time')
            }}</span>
            <div class="text-gray-900 dark:text-gray-100">
              {{ formatStatusTime(agentcoreRunnerStatus?.last_prepare_at) }}
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.evolvableParts', 'Evolvable parts')
            }}</span>
            <div class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="badge in agentcoreRunnerEvolvablePartBadges"
                :key="badge.part"
                data-testid="agentcore-runner-evolvable-part"
                :data-part="badge.part"
                :data-active="badge.active ? 'true' : 'false'"
                :title="badge.tooltip"
                class="rounded-full border px-2.5 py-1 text-[11px] font-medium transition"
                :class="
                  badge.active
                    ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-200'
                    : 'border-gray-200 bg-gray-100 text-gray-500 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-400'
                "
              >
                {{ badge.label }}
              </span>
            </div>
            <div
              v-if="agentcoreRunnerOptimizedSummary"
              data-testid="agentcore-runner-optimized-summary"
              class="mt-2 text-xs text-gray-700 dark:text-gray-200"
            >
              {{ agentcoreRunnerOptimizedSummary }}
            </div>
          </div>
          <div class="text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastOptimizationRunId', 'Last optimization run ID')
            }}</span>
            <div
              data-testid="agentcore-runner-last-optimization-run-id"
              class="break-all text-gray-900 dark:text-gray-100"
            >
              {{
                agentcoreRunnerStatus?.last_optimization_run_id ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
            <div
              data-testid="agentcore-runner-last-optimization-state"
              class="mt-1 text-xs text-gray-600 dark:text-gray-300"
            >
              {{
                agentcoreRunnerStatus?.last_optimization_state ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
            <div
              data-testid="agentcore-runner-last-optimization-time"
              class="text-xs text-gray-500 dark:text-gray-400"
            >
              {{ formatStatusTime(agentcoreRunnerStatus?.last_optimization_at) }}
            </div>
            <div
              data-testid="agentcore-runner-last-optimization-summary"
              class="mt-1 break-words text-xs text-gray-700 dark:text-gray-200"
            >
              {{
                agentcoreRunnerStatus?.last_optimization_summary ||
                t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
            <div
              v-if="agentcoreRunnerPrimaryPartLabel"
              data-testid="agentcore-runner-primary-part"
              class="mt-1 text-xs text-gray-600 dark:text-gray-300"
            >
              {{
                `${t('settings.agentcoreRunner.primaryPart', 'Primary')}: ${agentcoreRunnerPrimaryPartLabel}`
              }}
            </div>
            <div
              v-if="agentcoreRunnerSourceOptimizationRunID"
              data-testid="agentcore-runner-source-optimization-run-id"
              class="text-xs text-gray-500 dark:text-gray-400"
            >
              {{
                `${t('settings.agentcoreRunner.sourceOptimization', 'Source optimization')}: ${agentcoreRunnerSourceOptimizationRunID}`
              }}
            </div>
            <div
              v-if="agentcoreRunnerLastRun"
              data-testid="agentcore-runner-last-run-detail"
              :class="agentcoreRunnerLastRunCardClass"
            >
              <div
                v-if="agentcoreRunnerLastRunMeta.length > 0"
                data-testid="agentcore-runner-last-run-meta"
                class="flex flex-wrap gap-1.5"
              >
                <span
                  v-for="item in agentcoreRunnerLastRunMeta"
                  :key="item"
                  class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                >
                  {{ item }}
                </span>
              </div>
              <div
                v-if="agentcoreRunnerLastRun?.runner_error"
                data-testid="agentcore-runner-last-run-error"
                class="rounded-lg bg-red-50 px-3 py-2 text-[11px] leading-5 whitespace-pre-wrap text-red-700 dark:bg-red-950/30 dark:text-red-300"
              >
                {{ agentcoreRunnerLastRun.runner_error }}
              </div>
              <div
                v-if="agentcoreRunnerLastRun?.runner_response_text"
                data-testid="agentcore-runner-last-run-response"
                class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-[11px] leading-5 whitespace-pre-wrap text-gray-700 dark:border-gray-700 dark:bg-slate-900 dark:text-gray-200"
              >
                {{ agentcoreRunnerLastRun.runner_response_text }}
              </div>
              <div
                v-if="agentcoreRunnerSelectedCandidateID || agentcoreRunnerParetoFrontier.length > 0"
                data-testid="agentcore-runner-last-run-selection"
                class="space-y-2 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 text-[11px] leading-5 text-gray-700 dark:border-gray-700 dark:bg-slate-900/80 dark:text-gray-200"
              >
                <div v-if="agentcoreRunnerSelectedCandidateID">
                  {{
                    `${t('settings.agentcoreRunner.selectedCandidate', 'Selected candidate')}: ${agentcoreRunnerSelectedCandidateID}`
                  }}
                </div>
                <div v-if="agentcoreRunnerParetoFrontier.length > 0">
                  <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ t('settings.agentcoreRunner.paretoFrontier', 'Pareto frontier') }}
                  </div>
                  <div class="mt-1 flex flex-wrap gap-1.5">
                    <span
                      v-for="candidateID in agentcoreRunnerParetoFrontier"
                      :key="candidateID"
                      class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                    >
                      {{ candidateID }}
                    </span>
                  </div>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerSelectedCandidateObjectives.length > 0"
                data-testid="agentcore-runner-selected-candidate-objectives"
                class="space-y-2 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 text-[11px] leading-5 text-gray-700 dark:border-gray-700 dark:bg-slate-900/80 dark:text-gray-200"
              >
                <div class="flex flex-wrap gap-1.5">
                  <span
                    v-for="item in agentcoreRunnerSelectedCandidateMeta"
                    :key="item"
                    class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                  >
                    {{ item }}
                  </span>
                </div>
                <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('settings.agentcoreRunner.selectionBasis', 'Selection basis') }}
                </div>
                <div class="grid gap-2 sm:grid-cols-2">
                  <div
                    v-for="metric in agentcoreRunnerSelectedCandidateObjectives"
                    :key="metric.key"
                    class="rounded-lg border border-gray-200 bg-gray-50/80 px-3 py-2 dark:border-gray-700 dark:bg-slate-950/40"
                  >
                    <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                      {{ metric.label }}
                    </div>
                    <div class="mt-1 text-sm font-semibold text-gray-900 dark:text-gray-100">
                      {{ metric.value }}
                    </div>
                    <div class="mt-1 text-[10px] text-gray-500 dark:text-gray-400">
                      {{ metric.goalLabel }}
                    </div>
                  </div>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerParetoCandidateSummaries.length > 0"
                data-testid="agentcore-runner-pareto-candidate-summaries"
                class="space-y-2 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 text-[11px] leading-5 text-gray-700 dark:border-gray-700 dark:bg-slate-900/80 dark:text-gray-200"
              >
                <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('settings.agentcoreRunner.frontierCandidates', 'Frontier candidates') }}
                </div>
                <div class="space-y-2">
                  <div
                    v-for="candidate in agentcoreRunnerParetoCandidateSummaries"
                    :key="candidate.id"
                    class="rounded-lg border px-3 py-2"
                    :class="
                      candidate.selected
                        ? 'border-emerald-200 bg-emerald-50/70 dark:border-emerald-900/60 dark:bg-emerald-950/20'
                        : 'border-gray-200 bg-gray-50/80 dark:border-gray-700 dark:bg-slate-950/40'
                    "
                  >
                    <div class="flex items-center justify-between gap-2">
                      <div class="font-medium text-gray-900 dark:text-gray-100">
                        {{ candidate.id }}
                      </div>
                      <span
                        v-if="candidate.selected"
                        class="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-200"
                      >
                        {{ t('settings.agentcoreRunner.selectedCandidate', 'Selected candidate') }}
                      </span>
                    </div>
                    <div class="mt-1 text-[11px] leading-5 text-gray-700 dark:text-gray-200">
                      {{ candidate.summary }}
                    </div>
                  </div>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerEvaluatedCandidateCards.length > 0"
                data-testid="agentcore-runner-evaluated-candidates"
                class="space-y-2 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 text-[11px] leading-5 text-gray-700 dark:border-gray-700 dark:bg-slate-900/80 dark:text-gray-200"
              >
                <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('settings.agentcoreRunner.evaluatedCandidates', 'Evaluated candidates') }}
                </div>
                <div class="space-y-2">
                  <div
                    v-for="candidate in agentcoreRunnerEvaluatedCandidateCards"
                    :key="candidate.id"
                    :data-candidate-id="candidate.id"
                    data-testid="agentcore-runner-evaluated-candidate"
                    class="rounded-lg border px-3 py-2"
                    :class="
                      candidate.selected
                        ? 'border-emerald-200 bg-emerald-50/70 dark:border-emerald-900/60 dark:bg-emerald-950/20'
                        : candidate.frontier
                          ? 'border-sky-200 bg-sky-50/70 dark:border-sky-900/60 dark:bg-sky-950/20'
                          : 'border-gray-200 bg-gray-50/80 dark:border-gray-700 dark:bg-slate-950/40'
                    "
                  >
                    <div class="flex flex-wrap items-center justify-between gap-2">
                      <div class="font-medium text-gray-900 dark:text-gray-100">
                        {{ candidate.id }}
                      </div>
                      <div class="flex flex-wrap gap-1.5">
                        <span
                          v-if="candidate.selected"
                          class="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-200"
                        >
                          {{ t('settings.agentcoreRunner.selectedCandidate', 'Selected candidate') }}
                        </span>
                        <span
                          v-if="candidate.frontier"
                          class="rounded-full border border-sky-200 bg-sky-50 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-sky-800 dark:border-sky-900/60 dark:bg-sky-950/40 dark:text-sky-200"
                        >
                          {{ t('settings.agentcoreRunner.paretoFrontier', 'Pareto frontier') }}
                        </span>
                      </div>
                    </div>
                    <div v-if="candidate.meta.length > 0" class="mt-2 flex flex-wrap gap-1.5">
                      <span
                        v-for="item in candidate.meta"
                        :key="item"
                        class="rounded-full bg-white/80 px-2 py-0.5 text-[11px] font-medium text-gray-700 ring-1 ring-inset ring-gray-200 dark:bg-slate-900/60 dark:text-gray-200 dark:ring-slate-700"
                      >
                        {{ item }}
                      </span>
                    </div>
                    <div
                      v-if="candidate.objectiveSummary"
                      class="mt-2 rounded-lg bg-white/70 px-3 py-2 text-[11px] leading-5 text-gray-700 ring-1 ring-inset ring-gray-200 dark:bg-slate-900/50 dark:text-gray-200 dark:ring-slate-700"
                    >
                      <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        {{ t('settings.agentcoreRunner.objectiveVector', 'Objective vector') }}
                      </div>
                      <div class="mt-1">
                        {{ candidate.objectiveSummary }}
                      </div>
                    </div>
                    <div
                      v-if="candidate.followupSummary"
                      class="mt-2 rounded-lg bg-white/70 px-3 py-2 text-[11px] leading-5 text-gray-700 ring-1 ring-inset ring-gray-200 dark:bg-slate-900/50 dark:text-gray-200 dark:ring-slate-700"
                    >
                      <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                        {{ t('settings.agentcoreRunner.followupOutcome', 'Follow-up outcome') }}
                      </div>
                      <div class="mt-1">
                        {{ candidate.followupSummary }}
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerProposalCandidateIDs.length > 0 || agentcoreRunnerSampleEfficiencySummary"
                data-testid="agentcore-runner-last-run-search-details"
                class="space-y-2 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 text-[11px] leading-5 text-gray-700 dark:border-gray-700 dark:bg-slate-900/80 dark:text-gray-200"
              >
                <div v-if="agentcoreRunnerProposalCandidateIDs.length > 0">
                  <div class="text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {{ t('settings.agentcoreRunner.proposalSet', 'Proposal set') }}
                  </div>
                  <div class="mt-1 flex flex-wrap gap-1.5">
                    <span
                      v-for="candidateID in agentcoreRunnerProposalCandidateIDs"
                      :key="candidateID"
                      class="rounded-full bg-gray-100 px-2 py-0.5 text-[11px] font-medium text-gray-700 dark:bg-slate-800 dark:text-gray-200"
                    >
                      {{ candidateID }}
                    </span>
                  </div>
                </div>
                <div v-if="agentcoreRunnerSampleEfficiencySummary" data-testid="agentcore-runner-sample-efficiency-summary">
                  {{ agentcoreRunnerSampleEfficiencySummary }}
                </div>
              </div>
              <div
                v-if="agentcoreRunnerOfflineValueSummary"
                data-testid="agentcore-runner-offline-value-summary"
                class="rounded-lg border border-emerald-200 bg-emerald-50/80 px-3 py-2 text-[11px] leading-5 whitespace-pre-wrap text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200"
              >
                {{ agentcoreRunnerOfflineValueSummary }}
              </div>
              <div
                v-if="agentcoreRunnerOfflineValueMeta.length > 0"
                data-testid="agentcore-runner-offline-value-meta"
                class="flex flex-wrap gap-1.5"
              >
                <span
                  v-for="item in agentcoreRunnerOfflineValueMeta"
                  :key="item"
                  class="rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/40 dark:text-emerald-200"
                >
                  {{ item }}
                </span>
              </div>
              <div
                v-if="agentcoreRunnerOfflineImprovements.length > 0 || agentcoreRunnerOfflineTradeoffs.length > 0"
                data-testid="agentcore-runner-offline-value-details"
                class="space-y-2 rounded-lg border border-emerald-200 bg-emerald-50/60 px-3 py-2 text-[11px] leading-5 text-emerald-900 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-emerald-100"
              >
                <div v-if="agentcoreRunnerOfflineImprovements.length > 0">
                  <div class="text-[10px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                    {{ t('settings.agentcoreRunner.topImprovements', 'Top improvements') }}
                  </div>
                  <ul class="mt-1 space-y-1">
                    <li v-for="item in agentcoreRunnerOfflineImprovements" :key="item">{{ item }}</li>
                  </ul>
                </div>
                <div v-if="agentcoreRunnerOfflineTradeoffs.length > 0">
                  <div class="text-[10px] font-medium uppercase tracking-wide text-emerald-700 dark:text-emerald-300">
                    {{ t('settings.agentcoreRunner.topTradeoffs', 'Top tradeoffs') }}
                  </div>
                  <ul class="mt-1 space-y-1">
                    <li v-for="item in agentcoreRunnerOfflineTradeoffs" :key="item">{{ item }}</li>
                  </ul>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerRuntimeValueSummary"
                data-testid="agentcore-runner-runtime-value-summary"
                class="rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 text-[11px] leading-5 whitespace-pre-wrap text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200"
              >
                {{ agentcoreRunnerRuntimeValueSummary }}
              </div>
              <div
                v-if="agentcoreRunnerRuntimeValueMeta.length > 0"
                data-testid="agentcore-runner-runtime-value-meta"
                class="flex flex-wrap gap-1.5"
              >
                <span
                  v-for="item in agentcoreRunnerRuntimeValueMeta"
                  :key="item"
                  class="rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200"
                >
                  {{ item }}
                </span>
              </div>
              <div
                v-if="agentcoreRunnerRuntimeMetrics.length > 0"
                data-testid="agentcore-runner-runtime-value-metrics"
                class="grid gap-2 sm:grid-cols-2"
              >
                <div
                  v-for="metric in agentcoreRunnerRuntimeMetrics"
                  :key="metric.label"
                  class="rounded-lg border px-3 py-2 text-[11px] leading-5"
                  :class="
                    metric.tone === 'positive'
                      ? 'border-emerald-200 bg-emerald-50/70 text-emerald-900 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-emerald-100'
                      : metric.tone === 'negative'
                        ? 'border-amber-200 bg-amber-50/70 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-100'
                        : 'border-gray-200 bg-white/80 text-gray-700 dark:border-gray-700 dark:bg-slate-900/80 dark:text-gray-200'
                  "
                >
                  <div class="text-[10px] font-medium uppercase tracking-wide opacity-75">
                    {{ metric.label }}
                  </div>
                  <div class="mt-1 text-sm font-semibold">
                    {{ metric.value }}
                  </div>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerRuntimeImprovements.length > 0 || agentcoreRunnerRuntimeTradeoffs.length > 0"
                data-testid="agentcore-runner-runtime-value-details"
                class="space-y-2 rounded-lg border border-amber-200 bg-amber-50/60 px-3 py-2 text-[11px] leading-5 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/20 dark:text-amber-100"
              >
                <div v-if="agentcoreRunnerRuntimeImprovements.length > 0">
                  <div class="text-[10px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">
                    {{ t('settings.agentcoreRunner.topImprovements', 'Top improvements') }}
                  </div>
                  <ul class="mt-1 space-y-1">
                    <li v-for="item in agentcoreRunnerRuntimeImprovements" :key="item">{{ item }}</li>
                  </ul>
                </div>
                <div v-if="agentcoreRunnerRuntimeTradeoffs.length > 0">
                  <div class="text-[10px] font-medium uppercase tracking-wide text-amber-700 dark:text-amber-300">
                    {{ t('settings.agentcoreRunner.topTradeoffs', 'Top tradeoffs') }}
                  </div>
                  <ul class="mt-1 space-y-1">
                    <li v-for="item in agentcoreRunnerRuntimeTradeoffs" :key="item">{{ item }}</li>
                  </ul>
                </div>
              </div>
              <div
                v-if="agentcoreRunnerLastRunTranscriptEntries.length > 0"
                data-testid="agentcore-runner-last-run-transcript"
                class="max-h-48 space-y-2 overflow-auto"
              >
                <div
                  v-for="(entry, index) in agentcoreRunnerVisibleTranscriptEntries"
                  :key="`${entry.direction || 'run'}-${entry.method || 'message'}-${index}`"
                  class="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 dark:border-gray-700 dark:bg-slate-900/70"
                >
                  <div
                    class="flex flex-wrap gap-1.5 text-[10px] font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400"
                  >
                    <span>{{ entry.direction || 'run' }}</span>
                    <span v-if="entry.method">{{ entry.method }}</span>
                  </div>
                  <div class="mt-1 text-[11px] leading-5 whitespace-pre-wrap text-gray-700 dark:text-gray-200">
                    {{ entry.text }}
                  </div>
                </div>
              </div>
              <button
                v-if="agentcoreRunnerHiddenTranscriptCount > 0"
                data-testid="agentcore-runner-transcript-toggle"
                type="button"
                class="text-xs font-medium text-gray-500 transition hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
                :aria-expanded="agentcoreRunnerTranscriptExpanded ? 'true' : 'false'"
                @click="agentcoreRunnerTranscriptExpanded = !agentcoreRunnerTranscriptExpanded"
              >
                {{
                  agentcoreRunnerTranscriptExpanded
                    ? t('settings.smallModel.collapse', 'Collapse')
                    : t('settings.smallModel.expand', 'Expand')
                }}
              </button>
            </div>
          </div>
          <div class="text-sm sm:col-span-2">
            <span class="text-gray-500 dark:text-gray-400">{{
              t('settings.agentcoreRunner.lastError', 'Last error')
            }}</span>
            <div
              data-testid="agentcore-runner-last-error"
              class="break-all"
              :class="
                agentcoreRunnerHasLastError
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-gray-500 dark:text-gray-400'
              "
            >
              {{
                agentcoreRunnerLastError || t('settings.agentcoreRunner.empty', 'Not available')
              }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.agentcore-runner-panel {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1.1rem;
  border-radius: 1.35rem;
  box-shadow: none;
}

.agentcore-runner-panel--embedded {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.agentcore-runner-panel__title {
  margin: 0;
  font-size: 1.22rem;
  line-height: 1.15;
}

.agentcore-runner-panel__eyebrow {
  color: rgb(var(--dashboard-page-accent, 37, 99, 235));
}

.agentcore-runner-panel__body {
  border-radius: 1rem;
}

.agentcore-runner-panel__body--embedded {
  padding: 0;
  border-radius: 0;
  background: transparent;
}

.agentcore-runner-panel__actions {
  align-items: flex-start;
}

@media (max-width: 768px) {
  .agentcore-runner-panel {
    padding: 1rem;
  }

  .agentcore-runner-panel--embedded {
    padding: 0;
  }

  .agentcore-runner-panel__actions {
    flex-direction: column;
  }

  .agentcore-runner-panel__actions > div {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
