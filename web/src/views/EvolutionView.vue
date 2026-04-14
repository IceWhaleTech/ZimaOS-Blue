<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import type { LocationQueryRaw } from 'vue-router'

import {
  harnessApi,
  type HarnessEvalRunReport,
  type SkillDecisionHistoryEntry as SkillDecisionHistoryRecord,
  type SkillEvolutionCase,
  type SkillEvolutionCaseDetail,
  type SkillRevision,
} from '@/api/harness'
import {
  evolutionApi,
  type EvolutionOverview as EvolutionOverviewSummary,
} from '@/api/evolution'
import { knowledgeApi } from '@/api/knowledge'
import { selfReflectApi, type SelfReflectProposal } from '@/api/selfReflect'
import { skillApi, type Skill, type SkillContentResponse } from '@/api/skill'
import AutomationTabs from '@/components/automation/AutomationTabs.vue'
import EvolutionKnowledgePane from '@/components/automation/EvolutionKnowledgePane.vue'
import AgentcoreRunnerPanel from '@/components/harness/AgentcoreRunnerPanel.vue'
import { useNotificationStore } from '@/stores/notification'
import { useSettingsStore } from '@/stores/settings'
import { getErrorMessage } from '@/utils/error'

type EvolutionPane = 'knowledge' | 'skills' | 'runner' | 'instructions'
type ProposalAction = 'approve' | 'reject'
type SkillRevisionStatusFilter =
  | 'all'
  | 'accepted'
  | 'candidate'
  | 'promoted'
  | 'backup'
  | 'rejected'
type SkillCaseStatusFilter =
  | 'all'
  | 'open'
  | 'candidate_created'
  | 'accepted'
  | 'rejected'
  | 'promoted'
  | 'skipped'
type SkillCaseModeFilter = 'all' | 'fix' | 'capture'
type ProposalStatusFilter = 'all' | 'pending' | 'approved' | 'rejected'

type MetricEntry = {
  key: string
  label: string
  value: string
}

type MetricDeltaEntry = MetricEntry & {
  tone: 'positive' | 'negative' | 'neutral'
}

type PatchSummaryEntry = {
  key: string
  label: string
  value: string
}

type PatchSummary = {
  additions: number
  deletions: number
  sections: number
}

type RevisionBadge = {
  key: string
  label: string
  tone: 'sky' | 'emerald' | 'amber' | 'slate'
}

type EvidenceSummaryEntry = {
  key: string
  label: string
  value: string
  tone: 'sky' | 'emerald' | 'rose' | 'amber' | 'slate'
}

type CaseTimelineEntry = {
  key: string
  label: string
  time: string
  state: 'completed' | 'current' | 'upcoming'
}

type CaseStatusSummary = {
  label: string
  tone: 'emerald' | 'sky' | 'amber' | 'rose' | 'slate'
}

type CaseReferenceChip = {
  key: string
  label: string
  value: string
}

type EvolutionLaneCard = {
  pane: EvolutionPane
  title: string
  description: string
  metric: string
  supporting: string
}

type KnowledgeLaneSummary = {
  visiblePages: number
  totalPages: number
  conflicts: number
  gaps: number
}

type DecisionSummaryCard = {
  key: string
  label: string
  value: string
  details: string
  tone: 'sky' | 'emerald' | 'amber' | 'rose' | 'slate'
  actionLabel?: string
  action?: () => void | Promise<void>
}

type SkillDetailSectionKey = 'lineage' | 'comparison' | 'metrics' | 'evidence' | 'diff'

type SkillScorecardCard = {
  key: 'quality' | 'verification' | 'runtime' | 'tokens' | 'evidence'
  label: string
  value: string
  details: string
  tone: 'sky' | 'emerald' | 'amber' | 'rose' | 'slate'
  actionLabel: string
  action: () => void | Promise<void>
}

type SkillReviewCard = {
  key: string
  label: string
  value: string
  details: string
  tone: 'sky' | 'emerald' | 'amber' | 'rose' | 'slate'
}

type DecisionLogPreviewEntry = {
  key: string
  label: string
  value: string
  tone: 'sky' | 'emerald' | 'amber' | 'rose' | 'slate'
}

type DecisionHistoryDetail = {
  key: string
  label: string
  value: string
}

type DecisionHistoryBadge = {
  key: string
  label: string
}

type DecisionHistoryEvidenceSummary = {
  key: string
  label: string
  value: string
  tone: 'positive' | 'negative' | 'neutral'
}

type DecisionHistoryEntry = {
  key: string
  revisionID: string
  actionLabel: string
  title: string
  summary: string
  reviewedAt: string
  tone: 'sky' | 'emerald' | 'amber' | 'rose' | 'slate'
  badges: DecisionHistoryBadge[]
  details: DecisionHistoryDetail[]
  evidenceSummary?: DecisionHistoryEvidenceSummary[]
  links: {
    revisionID?: string
    evalRunID?: string
    backupRevisionID?: string
    sourceRevisionID?: string
    targetRevisionID?: string
    currentLiveRevisionID?: string
  }
}

type DecisionHistoryComparisonPair = {
  key: string
  label: string
  summary: string
  leftRevisionID: string
  rightRevisionID: string
}

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const notification = useNotificationStore()
const settingsStore = useSettingsStore()

const activePane = ref<EvolutionPane>(parseEvolutionPaneQuery(route.query.pane))
const knowledgeLaneSummary = ref<KnowledgeLaneSummary | null>(null)
const knowledgeLaneSummaryLoading = ref(false)
const evolutionOverview = ref<EvolutionOverviewSummary | null>(null)

const skillsLoading = ref(false)
const skillsError = ref('')
const availableSkills = ref<Skill[]>([])
const selectedSkillID = ref('')
const selectedSkillContent = ref<SkillContentResponse | null>(null)
const selectedSkillRevisions = ref<SkillRevision[]>([])
const selectedSkillCases = ref<SkillEvolutionCase[]>([])
const selectedSkillCaseDetailsByID = ref<Record<string, SkillEvolutionCaseDetail | null>>({})
const selectedSkillCaseDetailLoadingByID = ref<Record<string, boolean>>({})
const selectedSkillDecisionHistoryRecords = ref<SkillDecisionHistoryRecord[]>([])
const selectedSkillDecisionHistoryLoaded = ref(false)
const loadedSkillDetailsID = ref('')
const selectedRevisionID = ref('')
const decisionHistoryComparisonPair = ref<DecisionHistoryComparisonPair | null>(null)
const selectedRevisionReport = ref<HarnessEvalRunReport | null>(null)
const decisionHistoryReportsByEvalRunID = ref<Record<string, HarnessEvalRunReport | null>>({})
const decisionHistoryReportLoadingByEvalRunID = ref<Record<string, boolean>>({})
const runnerReportsByEvalRunID = ref<Record<string, HarnessEvalRunReport | null>>({})
const runnerReportLoadingByEvalRunID = ref<Record<string, boolean>>({})
const revisionReportLoading = ref(false)
const revisionReportError = ref('')
const focusedSkillDetailSection = ref<SkillDetailSectionKey | ''>('')
const promoteLoadingRevisionID = ref('')
const rollbackLoadingRevisionID = ref('')
const optimizeLoadingRevisionID = ref('')
const skillsRefreshing = ref(false)
const skillSearch = ref('')
const revisionSearch = ref('')
const revisionStatusFilter = ref<SkillRevisionStatusFilter>('all')
const caseSearch = ref('')
const caseStatusFilter = ref<SkillCaseStatusFilter>('all')
const caseModeFilter = ref<SkillCaseModeFilter>('all')

const instructionsLoaded = ref(false)
const instructionsLoading = ref(false)
const instructionsError = ref('')
const instructionProposals = ref<SelfReflectProposal[]>([])
const selectedProposalID = ref('')
const selectedProposal = ref<SelfReflectProposal | null>(null)
const selectedInstructionPatch = ref('')
const proposalDetailLoading = ref(false)
const proposalActionLoading = ref<ProposalAction | ''>('')
const proposalReviewNote = ref('')
const skillReviewNotesByRevisionID = ref<Record<string, string>>({})
const instructionSearch = ref('')
const instructionStatusFilter = ref<ProposalStatusFilter>('all')
const routedRevisionID = ref('')
const routedProposalID = ref('')

const lineageSectionRef = ref<HTMLElement | null>(null)
const comparisonSectionRef = ref<HTMLElement | null>(null)
const metricsSectionRef = ref<HTMLElement | null>(null)
const evidenceSectionRef = ref<HTMLElement | null>(null)
const diffSectionRef = ref<HTMLElement | null>(null)

const EVOLUTION_ROUTE_QUERY_KEYS = [
  'pane',
  'skill',
  'revision',
  'skillSearch',
  'revisionSearch',
  'revisionStatus',
  'caseSearch',
  'caseStatus',
  'caseMode',
  'proposal',
  'instructionSearch',
  'instructionStatus',
] as const

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function trp(
  key: string,
  fallback: string,
  params: Record<string, string | number | boolean>
): string {
  if (te(key)) return t(key, params)
  let text = fallback
  for (const [name, value] of Object.entries(params)) {
    text = text.split(`{${name}}`).join(String(value))
  }
  return text
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function normalizeText(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value.trim()
}

function routeQueryValue(raw: unknown): string {
  if (Array.isArray(raw)) {
    return String(raw[0] ?? '').trim()
  }
  return String(raw ?? '').trim()
}

function normalizedSearchText(value: unknown): string {
  return String(value ?? '')
    .trim()
    .toLowerCase()
}

function formatCount(value: number): string {
  return new Intl.NumberFormat().format(value)
}

type CountLabelKind =
  | 'activeFilter'
  | 'hiddenSelection'
  | 'acceptedRevisionReady'
  | 'rollbackBackupReady'
  | 'pendingProposal'
  | 'approvalWaiting'
  | 'metricLoaded'
  | 'partChanged'

type TranslationDescriptor = {
  key: string
  fallback: string
}

const ENUM_TRANSLATIONS: Record<string, TranslationDescriptor> = {
  open: { key: 'evolution.enums.status.open', fallback: 'Open' },
  pending: { key: 'evolution.enums.status.pending', fallback: 'Pending' },
  approved: { key: 'evolution.enums.status.approved', fallback: 'Approved' },
  accepted: { key: 'evolution.enums.status.accepted', fallback: 'Accepted' },
  rejected: { key: 'evolution.enums.status.rejected', fallback: 'Rejected' },
  candidate: { key: 'evolution.enums.status.candidate', fallback: 'Candidate' },
  candidate_created: {
    key: 'evolution.enums.status.candidateCreated',
    fallback: 'Candidate created',
  },
  promoted: { key: 'evolution.enums.status.promoted', fallback: 'Promoted' },
  backup: { key: 'evolution.enums.status.backup', fallback: 'Backup' },
  skipped: { key: 'evolution.enums.status.skipped', fallback: 'Skipped' },
  submitted: { key: 'evolution.enums.status.submitted', fallback: 'Submitted' },
  running: { key: 'evolution.enums.status.running', fallback: 'Running' },
  queued: { key: 'evolution.enums.status.queued', fallback: 'Queued' },
  planning: { key: 'evolution.enums.status.planning', fallback: 'Planning' },
  verifying: { key: 'evolution.enums.status.verifying', fallback: 'Verifying' },
  waiting_input: { key: 'evolution.enums.status.waitingInput', fallback: 'Waiting for input' },
  completed: { key: 'evolution.enums.status.completed', fallback: 'Completed' },
  failed: { key: 'evolution.enums.status.failed', fallback: 'Failed' },
  cancelled: { key: 'evolution.enums.status.cancelled', fallback: 'Cancelled' },
  aborted: { key: 'evolution.enums.status.aborted', fallback: 'Aborted' },
  fix: { key: 'evolution.enums.mode.fix', fallback: 'Fix' },
  capture: { key: 'evolution.enums.mode.capture', fallback: 'Capture' },
  runtime_failure: { key: 'evolution.enums.reason.runtimeFailure', fallback: 'Runtime failure' },
  runtime_capture: { key: 'evolution.enums.reason.runtimeCapture', fallback: 'Runtime capture' },
  selector_gate_failed: {
    key: 'evolution.enums.reason.selectorGateFailed',
    fallback: 'Selector gate failed',
  },
  execution_gate_failed: {
    key: 'evolution.enums.reason.executionGateFailed',
    fallback: 'Execution gate failed',
  },
  budget_gate_failed: {
    key: 'evolution.enums.reason.budgetGateFailed',
    fallback: 'Budget gate failed',
  },
  manual: { key: 'evolution.enums.reason.manual', fallback: 'Manual' },
  eval_run: { key: 'evolution.enums.sourceKind.evalRun', fallback: 'Eval run' },
  runtime_run: { key: 'evolution.enums.sourceKind.runtimeRun', fallback: 'Runtime run' },
  harness_group: { key: 'evolution.enums.sourceKind.harnessGroup', fallback: 'Harness group' },
  selector: { key: 'evolution.enums.gate.selector', fallback: 'Selector' },
  selector_gate: { key: 'evolution.enums.gate.selector', fallback: 'Selector' },
  execution: { key: 'evolution.enums.gate.execution', fallback: 'Execution' },
  execution_gate: { key: 'evolution.enums.gate.execution', fallback: 'Execution' },
  budget: { key: 'evolution.enums.gate.budget', fallback: 'Budget' },
  budget_gate: { key: 'evolution.enums.gate.budget', fallback: 'Budget' },
  constraints: { key: 'settings.agentcoreRunner.parts.constraints', fallback: 'Constraints' },
  skill_definition: {
    key: 'settings.agentcoreRunner.parts.skill_definition',
    fallback: 'Skill definition',
  },
  prompt_template: {
    key: 'settings.agentcoreRunner.parts.prompt_template',
    fallback: 'Prompt template',
  },
  context_assembly: {
    key: 'settings.agentcoreRunner.parts.context_assembly',
    fallback: 'Context assembly',
  },
  coordinator_policy: {
    key: 'settings.agentcoreRunner.parts.coordinator_policy',
    fallback: 'Coordinator',
  },
  orchestrator_policy: {
    key: 'settings.agentcoreRunner.parts.orchestrator_policy',
    fallback: 'Orchestrator',
  },
  tool_exposure: {
    key: 'settings.agentcoreRunner.parts.tool_exposure',
    fallback: 'Tool exposure',
  },
  verification_policy: {
    key: 'settings.agentcoreRunner.parts.verification_policy',
    fallback: 'Verification policy',
  },
  runner_code: { key: 'settings.agentcoreRunner.parts.runner_code', fallback: 'Runner code' },
  build_recipe: { key: 'settings.agentcoreRunner.parts.build_recipe', fallback: 'Build recipe' },
  promote: { key: 'evolution.skills.decisionHistory.action.promote', fallback: 'Promote' },
  rollback: { key: 'evolution.skills.decisionHistory.action.rollback', fallback: 'Rollback' },
}

const COUNT_LABELS: Record<
  CountLabelKind,
  {
    one: TranslationDescriptor
    other: TranslationDescriptor
  }
> = {
  activeFilter: {
    one: { key: 'evolution.count.activeFilterOne', fallback: '{count} active filter' },
    other: { key: 'evolution.count.activeFilterOther', fallback: '{count} active filters' },
  },
  hiddenSelection: {
    one: { key: 'evolution.count.hiddenSelectionOne', fallback: '{count} hidden selection' },
    other: {
      key: 'evolution.count.hiddenSelectionOther',
      fallback: '{count} hidden selections',
    },
  },
  acceptedRevisionReady: {
    one: {
      key: 'evolution.count.acceptedRevisionReadyOne',
      fallback: '{count} accepted revision ready',
    },
    other: {
      key: 'evolution.count.acceptedRevisionReadyOther',
      fallback: '{count} accepted revisions ready',
    },
  },
  rollbackBackupReady: {
    one: {
      key: 'evolution.count.rollbackBackupReadyOne',
      fallback: '{count} rollback backup ready',
    },
    other: {
      key: 'evolution.count.rollbackBackupReadyOther',
      fallback: '{count} rollback backups ready',
    },
  },
  pendingProposal: {
    one: { key: 'evolution.count.pendingProposalOne', fallback: '{count} pending proposal' },
    other: {
      key: 'evolution.count.pendingProposalOther',
      fallback: '{count} pending proposals',
    },
  },
  approvalWaiting: {
    one: { key: 'evolution.count.approvalWaitingOne', fallback: '{count} approval waiting' },
    other: {
      key: 'evolution.count.approvalWaitingOther',
      fallback: '{count} approvals waiting',
    },
  },
  metricLoaded: {
    one: { key: 'evolution.count.metricLoadedOne', fallback: '{count} metric loaded' },
    other: { key: 'evolution.count.metricLoadedOther', fallback: '{count} metrics loaded' },
  },
  partChanged: {
    one: { key: 'evolution.count.partChangedOne', fallback: '{count} part changed' },
    other: { key: 'evolution.count.partChangedOther', fallback: '{count} parts changed' },
  },
}

function formatEvolutionCountLabel(kind: CountLabelKind, value: number): string {
  const descriptor = value === 1 ? COUNT_LABELS[kind].one : COUNT_LABELS[kind].other
  return trp(descriptor.key, descriptor.fallback, { count: formatCount(value) })
}

function matchesSearchQuery(query: string, ...values: unknown[]): boolean {
  const normalizedQuery = normalizedSearchText(query)
  if (!normalizedQuery) return true
  return values.some((value) => normalizedSearchText(value).includes(normalizedQuery))
}

function parseEvolutionPaneQuery(raw: unknown): EvolutionPane {
  const value = routeQueryValue(raw)
  if (
    value === 'knowledge' ||
    value === 'runner' ||
    value === 'instructions' ||
    value === 'skills'
  ) {
    return value
  }
  return 'knowledge'
}

function parseSkillCaseStatusFilterQuery(raw: unknown): SkillCaseStatusFilter {
  const value = routeQueryValue(raw)
  if (
    value === 'open' ||
    value === 'candidate_created' ||
    value === 'accepted' ||
    value === 'rejected' ||
    value === 'promoted' ||
    value === 'skipped'
  ) {
    return value
  }
  return 'all'
}

function parseSkillCaseModeFilterQuery(raw: unknown): SkillCaseModeFilter {
  const value = routeQueryValue(raw)
  if (value === 'fix' || value === 'capture') return value
  return 'all'
}

function parseProposalStatusFilterQuery(raw: unknown): ProposalStatusFilter {
  const value = routeQueryValue(raw)
  if (value === 'pending' || value === 'approved' || value === 'rejected') return value
  return 'all'
}

function parseSkillRevisionStatusFilterQuery(raw: unknown): SkillRevisionStatusFilter {
  const value = routeQueryValue(raw)
  if (
    value === 'accepted' ||
    value === 'candidate' ||
    value === 'promoted' ||
    value === 'backup' ||
    value === 'rejected'
  ) {
    return value
  }
  return 'all'
}

function parseDateValue(value?: string | null): number {
  if (!value) return 0
  const parsed = Date.parse(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function sortByDateDesc<T extends { updated_at?: string; created_at?: string }>(items: T[]): T[] {
  return [...items].sort((left, right) => {
    const rightValue = Math.max(parseDateValue(right.updated_at), parseDateValue(right.created_at))
    const leftValue = Math.max(parseDateValue(left.updated_at), parseDateValue(left.created_at))
    return rightValue - leftValue
  })
}

function humanizeEnum(value: string | null | undefined): string {
  const normalized = String(value || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')
  const descriptor = ENUM_TRANSLATIONS[normalized]
  if (descriptor) return tr(descriptor.key, descriptor.fallback)
  return normalized.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function humanizeOptionalEnum(value: unknown): string {
  const normalized = normalizeText(value)
  return normalized ? humanizeEnum(normalized) : ''
}

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return parsed.toLocaleString()
}

function decisionTone(action: string): DecisionSummaryCard['tone'] {
  switch (action) {
    case 'promote':
      return 'emerald'
    case 'rollback':
      return 'amber'
    default:
      return 'slate'
  }
}

function decisionActionLabel(action: string): string {
  switch (action) {
    case 'promote':
      return tr('evolution.skills.decisionHistory.action.promote', 'Promote')
    case 'rollback':
      return tr('evolution.skills.decisionHistory.action.rollback', 'Rollback')
    default:
      return humanizeEnum(action)
  }
}

function hasPersistedSkillRevisionDecision(revision: SkillRevision): boolean {
  return Boolean(
    normalizeText(revision.decision_action) ||
    normalizeText(revision.review_note) ||
    normalizeText(revision.reviewed_by) ||
    normalizeText(revision.decision_log_json) ||
    revision.reviewed_at
  )
}

function skillDecisionHistoryMoment(entry: SkillDecisionHistoryRecord): number {
  return Math.max(
    parseDateValue(entry.decision_at),
    parseDateValue(entry.reviewed_at),
    parseDateValue(entry.promoted_at),
    parseDateValue(entry.created_at)
  )
}

function sortDecisionHistoryByDateDesc(
  items: SkillDecisionHistoryRecord[]
): SkillDecisionHistoryRecord[] {
  return [...items].sort(
    (left, right) => skillDecisionHistoryMoment(right) - skillDecisionHistoryMoment(left)
  )
}

function fallbackDecisionHistoryRecord(revision: SkillRevision): SkillDecisionHistoryRecord {
  return {
    revision_id: revision.id,
    skill_id: revision.skill_id,
    status: revision.status,
    source_path: revision.source_path,
    candidate_id: revision.candidate_id,
    base_content_sha256: revision.base_content_sha256,
    origin_case_id: revision.origin_case_id,
    parent_revision_id: revision.parent_revision_id,
    backup_of_revision_id: revision.backup_of_revision_id,
    eval_run_id: revision.eval_run_id,
    optimization_run_id: revision.optimization_run_id,
    followup_gate: revision.followup_gate,
    optimization_surface: revision.optimization_surface,
    decision_action: revision.decision_action,
    review_note: revision.review_note,
    reviewed_by: revision.reviewed_by,
    decision_log: parseJSONRecord(revision.decision_log_json),
    decision_log_json: revision.decision_log_json,
    decision_at: revision.reviewed_at || revision.promoted_at || revision.created_at,
    created_at: revision.created_at,
    reviewed_at: revision.reviewed_at,
    promoted_at: revision.promoted_at,
  }
}

function formatDurationMs(value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 0) return tr('common.notAvailable', 'Not available')
  if (numeric < 1000) return `${Math.round(numeric)} ms`
  return `${(numeric / 1000).toFixed(1)} s`
}

function formatPercentValue(value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return tr('common.notAvailable', 'Not available')
  if (numeric >= 0 && numeric <= 1) return `${Math.round(numeric * 100)}%`
  return `${numeric.toFixed(1)}%`
}

function formatMetricValue(key: string, value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return tr('common.notAvailable', 'Not available')
  if (key.includes('token')) return new Intl.NumberFormat().format(numeric)
  if (key.includes('duration') || key.endsWith('_ms')) return formatDurationMs(numeric)
  if (key.includes('rate') || key.includes('score')) return formatPercentValue(numeric)
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(numeric)
}

function metricLabel(key: string): string {
  const labels: Record<string, string> = {
    overall_score: tr('harness.groups.score', 'Overall score'),
    pass_rate: tr('harness.groups.passRate', 'Pass rate'),
    verification_pass_rate: tr('evolution.metrics.verificationPassRate', 'Verification pass rate'),
    evidence_backed_pass_rate: tr(
      'evolution.metrics.evidenceBackedPassRate',
      'Evidence-backed pass rate'
    ),
    critical_pass_rate: tr('evolution.metrics.criticalPassRate', 'Critical pass rate'),
    total_tokens: tr('evolution.metrics.totalTokens', 'Total tokens'),
    avg_duration_ms: tr('evolution.metrics.avgDuration', 'Average runtime'),
    duration_ms: tr('evolution.metrics.runtime', 'Runtime'),
  }
  return labels[key] || humanizeEnum(key)
}

function comparisonMetricLabel(key: string): string {
  const labels: Record<string, string> = {
    overall_score_delta: tr('harness.compare.scoreDelta', 'Score delta'),
    pass_rate_delta: tr('harness.compare.passRateDelta', 'Pass rate delta'),
    verification_pass_rate_delta: tr(
      'harness.compare.verificationPassRateDelta',
      'Verification pass rate delta'
    ),
    evidence_backed_pass_rate_delta: tr(
      'harness.compare.evidenceBackedPassRateDelta',
      'Evidence-backed pass rate delta'
    ),
  }
  return labels[key] || humanizeEnum(key)
}

function formatSignedScoreValue(value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return tr('common.notAvailable', 'Not available')
  return `${numeric > 0 ? '+' : ''}${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(numeric)}`
}

function formatSignedPercentValue(value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return tr('common.notAvailable', 'Not available')
  const scaled = numeric >= -1 && numeric <= 1 ? numeric * 100 : numeric
  const useDecimal = Math.abs(scaled) > 0 && Math.abs(scaled) < 10
  return `${scaled > 0 ? '+' : ''}${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: useDecimal ? 1 : 0,
    maximumFractionDigits: useDecimal ? 1 : 0,
  }).format(scaled)}%`
}

function formatSignedMetricValue(key: string, value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return tr('common.notAvailable', 'Not available')
  if (key.includes('rate')) return formatSignedPercentValue(numeric)
  if (key.includes('score')) return formatSignedScoreValue(numeric)
  if (key.includes('token'))
    return `${numeric > 0 ? '+' : ''}${new Intl.NumberFormat().format(numeric)}`
  if (key.includes('duration') || key.endsWith('_ms')) {
    return `${numeric > 0 ? '+' : ''}${formatDurationMs(Math.abs(numeric))}`
  }
  return `${numeric > 0 ? '+' : ''}${new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 2,
  }).format(numeric)}`
}

function metricDeltaTone(value: unknown): MetricDeltaEntry['tone'] {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric === 0) return 'neutral'
  return numeric > 0 ? 'positive' : 'negative'
}

function emptyEvolutionOverview(skillID = ''): EvolutionOverviewSummary {
  return {
    skill_id: skillID,
    revisions: {
      accepted: 0,
    },
    instructions: {
      pending: evolutionOverview.value?.instructions.pending || 0,
    },
  }
}

function decisionHistoryEvidenceClasses(tone: DecisionHistoryEvidenceSummary['tone']): string {
  switch (tone) {
    case 'positive':
      return 'border-emerald-200 bg-emerald-50/70'
    case 'negative':
      return 'border-rose-200 bg-rose-50/70'
    default:
      return 'border-slate-200 bg-slate-50'
  }
}

function summarizePatch(content: string): PatchSummary {
  const lines = normalizeDiffLines(content)
  let additions = 0
  let deletions = 0
  let sections = 0
  let inSection = false

  for (const line of lines) {
    if (line.startsWith('+++') || line.startsWith('---')) {
      inSection = false
      continue
    }
    if (line.startsWith('@@')) {
      sections += 1
      inSection = true
      continue
    }
    if (line.startsWith('+')) {
      additions += 1
      if (!inSection) {
        sections += 1
        inSection = true
      }
      continue
    }
    if (line.startsWith('-')) {
      deletions += 1
      if (!inSection) {
        sections += 1
        inSection = true
      }
      continue
    }
    inSection = false
  }

  return {
    additions,
    deletions,
    sections,
  }
}

function patchSummaryEntries(summary: PatchSummary): PatchSummaryEntry[] {
  return [
    {
      key: 'additions',
      label: tr('evolution.patchSummary.additions', 'Added lines'),
      value: new Intl.NumberFormat().format(summary.additions),
    },
    {
      key: 'deletions',
      label: tr('evolution.patchSummary.deletions', 'Removed lines'),
      value: new Intl.NumberFormat().format(summary.deletions),
    },
    {
      key: 'sections',
      label: tr('evolution.patchSummary.sections', 'Change sections'),
      value: new Intl.NumberFormat().format(summary.sections),
    },
  ]
}

function revisionBadgeClasses(tone: RevisionBadge['tone']): string {
  switch (tone) {
    case 'sky':
      return 'bg-sky-100 text-sky-700'
    case 'emerald':
      return 'bg-emerald-100 text-emerald-700'
    case 'amber':
      return 'bg-amber-100 text-amber-800'
    default:
      return 'bg-slate-100 text-slate-600'
  }
}

function evidenceSummaryClasses(tone: EvidenceSummaryEntry['tone']): string {
  switch (tone) {
    case 'sky':
      return 'border-sky-200 bg-sky-50/70'
    case 'emerald':
      return 'border-emerald-200 bg-emerald-50/70'
    case 'rose':
      return 'border-rose-200 bg-rose-50/70'
    case 'amber':
      return 'border-amber-200 bg-amber-50/70'
    default:
      return 'border-slate-200 bg-slate-50'
  }
}

function evidenceSummaryValueClasses(tone: EvidenceSummaryEntry['tone']): string {
  switch (tone) {
    case 'sky':
      return 'text-sky-700'
    case 'emerald':
      return 'text-emerald-700'
    case 'rose':
      return 'text-rose-700'
    case 'amber':
      return 'text-amber-800'
    default:
      return 'text-slate-950'
  }
}

function decisionSummaryCardClasses(tone: DecisionSummaryCard['tone']): string {
  switch (tone) {
    case 'sky':
      return 'border-sky-200 bg-sky-50/70'
    case 'emerald':
      return 'border-emerald-200 bg-emerald-50/70'
    case 'amber':
      return 'border-amber-200 bg-amber-50/70'
    case 'rose':
      return 'border-rose-200 bg-rose-50/70'
    default:
      return 'border-slate-200 bg-slate-50'
  }
}

function decisionSummaryValueClasses(tone: DecisionSummaryCard['tone']): string {
  switch (tone) {
    case 'sky':
      return 'text-sky-700'
    case 'emerald':
      return 'text-emerald-700'
    case 'amber':
      return 'text-amber-800'
    case 'rose':
      return 'text-rose-700'
    default:
      return 'text-slate-950'
  }
}

function decisionTimelineMarkerClasses(tone: DecisionSummaryCard['tone']): string {
  switch (tone) {
    case 'sky':
      return 'border-sky-400 bg-sky-100'
    case 'emerald':
      return 'border-emerald-400 bg-emerald-100'
    case 'amber':
      return 'border-amber-400 bg-amber-100'
    case 'rose':
      return 'border-rose-400 bg-rose-100'
    default:
      return 'border-slate-300 bg-slate-100'
  }
}

function skillDetailSectionClasses(section: SkillDetailSectionKey): string {
  return focusedSkillDetailSection.value === section
    ? 'border-sky-300 ring-2 ring-sky-100'
    : 'border-slate-200'
}

function scorecardToneForPercent(value: unknown): SkillScorecardCard['tone'] {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 'slate'
  if (numeric >= 0.9) return 'emerald'
  if (numeric >= 0.75) return 'sky'
  if (numeric >= 0.5) return 'amber'
  return 'rose'
}

function scorecardToneForDuration(value: unknown): SkillScorecardCard['tone'] {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 'slate'
  if (numeric <= 2000) return 'emerald'
  if (numeric <= 5000) return 'sky'
  if (numeric <= 12000) return 'amber'
  return 'rose'
}

function scorecardToneForTokens(value: unknown): SkillScorecardCard['tone'] {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 'slate'
  if (numeric <= 5000) return 'sky'
  if (numeric <= 12000) return 'amber'
  return 'rose'
}

async function focusSkillDetailSection(section: SkillDetailSectionKey) {
  focusedSkillDetailSection.value = section
  await nextTick()
  const sectionRefMap: Record<SkillDetailSectionKey, HTMLElement | null> = {
    lineage: lineageSectionRef.value,
    comparison: comparisonSectionRef.value,
    metrics: metricsSectionRef.value,
    evidence: evidenceSectionRef.value,
    diff: diffSectionRef.value,
  }
  sectionRefMap[section]?.scrollIntoView?.({
    behavior: 'smooth',
    block: 'start',
  })
}

async function openDecisionHistoryComparison(pair: DecisionHistoryComparisonPair | null) {
  if (!pair) return
  decisionHistoryComparisonPair.value = pair
  await focusSkillDetailSection('comparison')
}

function caseTimelineMarkerClasses(state: CaseTimelineEntry['state']): string {
  switch (state) {
    case 'completed':
      return 'border-emerald-300 bg-emerald-500'
    case 'current':
      return 'border-sky-300 bg-sky-500'
    default:
      return 'border-slate-300 bg-white'
  }
}

function caseTimelineCardClasses(state: CaseTimelineEntry['state']): string {
  switch (state) {
    case 'completed':
      return 'border-emerald-200 bg-emerald-50/70'
    case 'current':
      return 'border-sky-200 bg-sky-50/70'
    default:
      return 'border-slate-200 bg-slate-50'
  }
}

function caseStatusSummaryClasses(tone: CaseStatusSummary['tone']): string {
  switch (tone) {
    case 'emerald':
      return 'bg-emerald-100 text-emerald-700'
    case 'sky':
      return 'bg-sky-100 text-sky-700'
    case 'amber':
      return 'bg-amber-100 text-amber-800'
    case 'rose':
      return 'bg-rose-100 text-rose-700'
    default:
      return 'bg-slate-100 text-slate-600'
  }
}

function caseReferenceChips(skillCase: SkillEvolutionCase): CaseReferenceChip[] {
  return [
    {
      key: 'source_id',
      label: tr('evolution.skills.caseRefs.source', 'Source'),
      value: normalizeText(skillCase.source_id),
    },
    {
      key: 'candidate_id',
      label: tr('evolution.skills.caseRefs.candidate', 'Candidate'),
      value: normalizeText(skillCase.candidate_id),
    },
    {
      key: 'revision_id',
      label: tr('evolution.skills.caseRefs.revision', 'Revision'),
      value: normalizeText(skillCase.revision_id),
    },
  ].filter((entry) => entry.value)
}

function revisionStatusSummary(status?: string): string {
  switch (normalizeText(status)) {
    case 'accepted':
      return tr(
        'evolution.skills.statusAcceptedHint',
        'This candidate passed the gate and is ready to be promoted as the better canonical version.'
      )
    case 'promoted':
      return tr(
        'evolution.skills.statusPromotedHint',
        'This revision has already been switched into the canonical skill.'
      )
    case 'backup':
      return tr(
        'evolution.skills.statusBackupHint',
        'This backup preserves the previous canonical version and can be used for a safe one-step rollback while the current canonical version still matches.'
      )
    case 'candidate':
      return tr(
        'evolution.skills.statusCandidateHint',
        'This candidate is still waiting for gate results before it can be considered better.'
      )
    case 'rejected':
      return tr(
        'evolution.skills.statusRejectedHint',
        'This candidate did not pass the gate and cannot replace the canonical version.'
      )
    default:
      return tr('common.notAvailable', 'Not available')
  }
}

function normalizeDiffLines(content: string): string[] {
  return content.replace(/\r\n/g, '\n').split('\n')
}

function buildUnifiedDiff(baseContent: string, nextContent: string): string {
  const before = normalizeDiffLines(baseContent)
  const after = normalizeDiffLines(nextContent)

  if (baseContent === nextContent) {
    return ['--- canonical', '+++ candidate', ' (no changes)'].join('\n')
  }

  if (before.length * after.length > 40000) {
    return [
      '--- canonical',
      '+++ candidate',
      ...before.map((line) => `-${line}`),
      ...after.map((line) => `+${line}`),
    ].join('\n')
  }

  const lcs = Array.from({ length: before.length + 1 }, () =>
    Array<number>(after.length + 1).fill(0)
  )

  for (let i = before.length - 1; i >= 0; i -= 1) {
    for (let j = after.length - 1; j >= 0; j -= 1) {
      lcs[i]![j] =
        before[i] === after[j]
          ? (lcs[i + 1]![j + 1] ?? 0) + 1
          : Math.max(lcs[i + 1]![j] ?? 0, lcs[i]![j + 1] ?? 0)
    }
  }

  const output = ['--- canonical', '+++ candidate']
  let beforeIndex = 0
  let afterIndex = 0

  while (beforeIndex < before.length && afterIndex < after.length) {
    if (before[beforeIndex] === after[afterIndex]) {
      output.push(` ${before[beforeIndex]}`)
      beforeIndex += 1
      afterIndex += 1
      continue
    }

    if ((lcs[beforeIndex + 1]![afterIndex] ?? 0) >= (lcs[beforeIndex]![afterIndex + 1] ?? 0)) {
      output.push(`-${before[beforeIndex]}`)
      beforeIndex += 1
      continue
    }

    output.push(`+${after[afterIndex]}`)
    afterIndex += 1
  }

  while (beforeIndex < before.length) {
    output.push(`-${before[beforeIndex]}`)
    beforeIndex += 1
  }

  while (afterIndex < after.length) {
    output.push(`+${after[afterIndex]}`)
    afterIndex += 1
  }

  return output.join('\n')
}

function stripDiffMarker(line: string): string {
  if (/^[ +-]/.test(line)) return line.slice(1)
  return line
}

function inferGuidanceAreasFromLine(content: string): string[] {
  const normalized = normalizedSearchText(content)
  const areas: string[] = []
  const push = (label: string) => {
    if (!areas.includes(label)) areas.push(label)
  }
  if (
    normalized.includes('when to use') ||
    normalized.includes('when ') ||
    normalized.includes('apply') ||
    normalized.includes('use this')
  ) {
    push(tr('evolution.skills.guidance.whenToUse', 'When to use'))
  }
  if (normalized.includes('example')) {
    push(tr('evolution.skills.guidance.examples', 'Examples'))
  }
  if (
    normalized.includes('recover') ||
    normalized.includes('recovery') ||
    normalized.includes('retry') ||
    normalized.includes('restore') ||
    normalized.includes('fallback') ||
    normalized.includes('failure') ||
    normalized.includes('error') ||
    normalized.includes('timeout')
  ) {
    push(tr('evolution.skills.guidance.recovery', 'Recovery guidance'))
  }
  if (
    normalized.includes('anti-pattern') ||
    normalized.includes('avoid') ||
    normalized.includes("don't") ||
    normalized.includes('do not') ||
    normalized.includes('never')
  ) {
    push(tr('evolution.skills.guidance.antiPatterns', 'Anti-patterns'))
  }
  if (
    normalized.includes('step') ||
    normalized.includes('workflow') ||
    normalized.includes('checklist') ||
    normalized.includes('procedure')
  ) {
    push(tr('evolution.skills.guidance.workflow', 'Workflow steps'))
  }
  return areas
}

function summarizeGuidanceAreas(diffContent: string): string[] {
  const areas = new Set<string>()
  let activeHeading = ''
  for (const line of normalizeDiffLines(diffContent)) {
    if (line.startsWith('---') || line.startsWith('+++')) continue
    const marker = line[0] || ''
    if (marker !== '+' && marker !== '-' && marker !== ' ') continue
    const content = stripDiffMarker(line).trim()
    if (!content || content === '(no changes)') continue
    const headingMatch = content.match(/^#{1,6}\s+(.+)$/)
    if (headingMatch) {
      activeHeading = headingMatch[1]!.trim()
      if ((marker === '+' || marker === '-') && activeHeading) {
        areas.add(activeHeading)
      }
      continue
    }
    if (marker !== '+' && marker !== '-') continue
    if (activeHeading) {
      areas.add(activeHeading)
    }
    for (const area of inferGuidanceAreasFromLine(content)) {
      areas.add(area)
    }
  }
  if (
    areas.size === 0 &&
    diffContent !== tr('evolution.noDiff', 'No candidate patch available yet.')
  ) {
    areas.add(tr('evolution.skills.guidance.general', 'General guidance'))
  }
  return Array.from(areas)
}

function patchRiskTone(summary: PatchSummary): SkillReviewCard['tone'] {
  const totalChanges = summary.additions + summary.deletions
  if (totalChanges === 0) return 'slate'
  if (totalChanges <= 4 && summary.sections <= 2) return 'emerald'
  if (totalChanges <= 12 && summary.sections <= 4) return 'amber'
  return 'rose'
}

function patchRiskLabel(summary: PatchSummary): string {
  const totalChanges = summary.additions + summary.deletions
  if (totalChanges === 0) return tr('evolution.skills.diffReview.noPatch', 'No patch')
  if (totalChanges <= 4 && summary.sections <= 2) {
    return tr('evolution.skills.diffReview.lowRisk', 'Low review risk')
  }
  if (totalChanges <= 12 && summary.sections <= 4) {
    return tr('evolution.skills.diffReview.mediumRisk', 'Moderate review risk')
  }
  return tr('evolution.skills.diffReview.highRisk', 'High review risk')
}

function patchScopeLabel(summary: PatchSummary): string {
  const totalChanges = summary.additions + summary.deletions
  if (totalChanges === 0) return tr('evolution.skills.diffReview.emptyScope', 'No change')
  if (totalChanges <= 4 && summary.sections <= 2) {
    return tr('evolution.skills.diffReview.targetedEdit', 'Targeted edit')
  }
  if (totalChanges <= 12 && summary.sections <= 4) {
    return tr('evolution.skills.diffReview.moderateRewrite', 'Moderate rewrite')
  }
  return tr('evolution.skills.diffReview.wideRewrite', 'Wide rewrite')
}

function prettyJSON(raw?: string): string {
  const normalized = normalizeText(raw)
  if (!normalized) return tr('common.notAvailable', 'Not available')
  try {
    return JSON.stringify(JSON.parse(normalized), null, 2)
  } catch {
    return normalized
  }
}

function parseJSONRecord(raw?: string): Record<string, unknown> | null {
  const normalized = normalizeText(raw)
  if (!normalized) return null
  try {
    return asRecord(JSON.parse(normalized))
  } catch {
    return null
  }
}

function decodeStringList(raw: unknown): string[] {
  if (Array.isArray(raw)) {
    return raw.map((item) => normalizeText(item)).filter(Boolean)
  }
  return []
}

function boolSummaryLabel(value: boolean): string {
  return value ? tr('common.yes', 'Yes') : tr('common.no', 'No')
}

function skillFilter(skill: Skill): boolean {
  return skill.writable === true
}

function buildSkillTranslationKeys(skill: Skill, field: 'name' | 'description'): string[] {
  const normalizedIDs = Array.from(
    new Set(
      [skill.id, skill.id.replace(/-/g, '_'), skill.id.replace(/_/g, '-')].map((value) =>
        normalizeText(value)
      )
    )
  ).filter(Boolean)
  const keys: string[] = []

  for (const id of normalizedIDs) {
    if (skill.builtin) {
      keys.push(`skills.builtin.${id}.${field}`)
    }
    keys.push(`skills.catalog.${id}.${field}`)
    keys.push(field === 'name' ? `tools.names.${id}` : `tools.descriptions.${id}`)
  }

  if (field === 'name' && normalizeText(skill.name)) {
    keys.push(`skills.names.${skill.name}`)
  }

  return keys
}

function localizedSkillName(skill: Skill): string {
  for (const key of buildSkillTranslationKeys(skill, 'name')) {
    if (te(key)) return t(key)
  }
  return skill.name || skill.id
}

function localizedSkillDescription(skill: Skill): string {
  for (const key of buildSkillTranslationKeys(skill, 'description')) {
    if (te(key)) return t(key)
  }
  return skill.description || ''
}

function selectInitialRevision(revisions: SkillRevision[]): string {
  if (revisions.length === 0) return ''
  const preferred = revisions.find((revision) => revision.status === 'accepted') || revisions[0]
  return preferred?.id || ''
}

function proposalSortWeight(proposal: SelfReflectProposal): number {
  return normalizeText(proposal.target_file).endsWith('AGENTS.md') ? 0 : 1
}

function reportSummarySources(report: HarnessEvalRunReport | null): Record<string, unknown>[] {
  return [
    asRecord(report?.eval_run?.summary),
    asRecord(report?.group_report?.group?.summary),
  ].filter(Boolean) as Record<string, unknown>[]
}

function reportSummaryValue(
  report: HarnessEvalRunReport | null,
  key: string,
  fallbackValue?: unknown
): unknown {
  for (const source of reportSummarySources(report)) {
    if (source[key] != null) return source[key]
  }
  return fallbackValue
}

function formatCompactDurationMs(value: unknown): string {
  const numeric = Number(value)
  if (!Number.isFinite(numeric) || numeric < 0) return tr('common.notAvailable', 'Not available')
  if (numeric < 1000) return `${Math.round(numeric)}ms`
  return `${(numeric / 1000).toFixed(1)}s`
}

function runnerFollowupLabel(state: string, gate: string): string {
  const normalizedState = normalizeText(state)
  const normalizedGate = normalizeText(gate)
  const gateLabel = normalizedGate ? humanizeEnum(normalizedGate) : ''
  if (normalizedState === 'accepted' && normalizedGate) {
    return trp('evolution.runner.followupAccepted', 'Accepted via {gate} gate', {
      gate: gateLabel,
    })
  }
  if (normalizedState === 'rejected' && normalizedGate) {
    return trp('evolution.runner.followupRejected', 'Rejected via {gate} gate', {
      gate: gateLabel,
    })
  }
  if (normalizedState === 'submitted' && normalizedGate) {
    return trp('evolution.runner.followupSubmitted', 'Follow-up submitted to {gate} gate', {
      gate: gateLabel,
    })
  }
  if (normalizedState === 'running' && normalizedGate) {
    return trp('evolution.runner.followupRunning', '{gate} gate still running', {
      gate: gateLabel,
    })
  }
  if (normalizedState === 'skipped') {
    return tr('evolution.runner.followupSkipped', 'Follow-up skipped')
  }
  if (normalizedState) return humanizeEnum(normalizedState)
  if (normalizedGate) {
    return trp('evolution.runner.followupGateOnly', '{gate} gate attached', {
      gate: gateLabel,
    })
  }
  return tr('common.notAvailable', 'Not available')
}

function buildRunnerReportMetrics(report: HarnessEvalRunReport | null): MetricEntry[] {
  if (!report) return []
  const preferredKeys = [
    'overall_score',
    'pass_rate',
    'verification_pass_rate',
    'evidence_backed_pass_rate',
    'total_tokens',
    'avg_duration_ms',
    'duration_ms',
  ]
  const metrics: MetricEntry[] = []
  for (const key of preferredKeys) {
    const value = reportSummaryValue(report, key)
    if (value == null) continue
    if (key === 'duration_ms' && reportSummaryValue(report, 'avg_duration_ms') != null) continue
    metrics.push({
      key,
      label: metricLabel(key),
      value: formatMetricValue(key, value),
    })
  }
  return metrics
}

function skillDecisionHistoryEvalRunID(entry: SkillDecisionHistoryRecord): string {
  const decisionLog = entry.decision_log || parseJSONRecord(entry.decision_log_json)
  return normalizeText(
    typeof decisionLog?.eval_run_id === 'string' ? decisionLog.eval_run_id : entry.eval_run_id
  )
}

function scoreTone(value: unknown): 'positive' | 'negative' | 'neutral' {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 'neutral'
  const normalized = numeric >= 0 && numeric <= 1 ? numeric : numeric / 100
  if (!Number.isFinite(normalized)) return 'neutral'
  if (normalized >= 0.8) return 'positive'
  if (normalized <= 0.5) return 'negative'
  return 'neutral'
}

function buildDecisionHistoryEvidenceSummary(
  report: HarnessEvalRunReport | null
): DecisionHistoryEvidenceSummary[] {
  if (!report) return []
  const summary: DecisionHistoryEvidenceSummary[] = []
  const push = (
    key: string,
    value: unknown,
    tone: 'positive' | 'negative' | 'neutral' = 'neutral'
  ) => {
    if (value == null) return
    summary.push({
      key,
      label: metricLabel(key),
      value: formatMetricValue(key, value),
      tone,
    })
  }

  const overallScore = reportSummaryValue(
    report,
    'overall_score',
    report?.group_report?.overall_score
  )
  const passRate = reportSummaryValue(report, 'pass_rate', report?.group_report?.pass_rate)
  const runtime =
    reportSummaryValue(report, 'avg_duration_ms') ?? reportSummaryValue(report, 'duration_ms')
  const tokens = reportSummaryValue(report, 'total_tokens')

  push('overall_score', overallScore, scoreTone(overallScore))
  push('pass_rate', passRate, scoreTone(passRate))
  push(
    reportSummaryValue(report, 'avg_duration_ms') != null ? 'avg_duration_ms' : 'duration_ms',
    runtime,
    'neutral'
  )
  push('total_tokens', tokens, 'neutral')
  return summary
}

function caseStatusSummary(skillCase: SkillEvolutionCase): CaseStatusSummary {
  switch (normalizeText(skillCase.status)) {
    case 'open':
      return {
        label: tr('evolution.skills.caseStatus.open', 'Open intake, waiting candidate'),
        tone: 'slate',
      }
    case 'candidate_created':
      return {
        label: tr(
          'evolution.skills.caseStatus.candidateCreated',
          'Candidate created, waiting gate'
        ),
        tone: 'sky',
      }
    case 'accepted':
      return {
        label: tr('evolution.skills.caseStatus.accepted', 'Accepted, waiting promote'),
        tone: 'emerald',
      }
    case 'rejected':
      return {
        label: tr('evolution.skills.caseStatus.rejected', 'Rejected by gate'),
        tone: 'rose',
      }
    case 'promoted':
      return {
        label: tr('evolution.skills.caseStatus.promoted', 'Promoted, canonical live'),
        tone: 'emerald',
      }
    case 'skipped':
      return {
        label: tr('evolution.skills.caseStatus.skipped', 'Skipped by policy'),
        tone: 'amber',
      }
    default:
      return {
        label: humanizeEnum(skillCase.status),
        tone: 'slate',
      }
  }
}

const selectedSkill = computed(
  () => availableSkills.value.find((skill) => skill.id === selectedSkillID.value) || null
)

function hasSelectedSkillRevision(revisionID?: string): boolean {
  const normalizedID = normalizeText(revisionID)
  if (!normalizedID) return false
  return selectedSkillRevisions.value.some((revision) => revision.id === normalizedID)
}

function decisionHistoryRollbackRevision(entry: DecisionHistoryEntry): SkillRevision | null {
  const backupRevisionID = normalizeText(entry.links.backupRevisionID)
  if (!backupRevisionID) return null
  return (
    selectedSkillRevisions.value.find(
      (revision) => revision.id === backupRevisionID && revision.status === 'backup'
    ) || null
  )
}

const filteredAvailableSkills = computed(() =>
  availableSkills.value.filter((skill) =>
    matchesSearchQuery(
      skillSearch.value,
      skill.id,
      localizedSkillName(skill),
      localizedSkillDescription(skill),
      skill.version
    )
  )
)

const selectedSkillHiddenByFilters = computed(
  () =>
    Boolean(selectedSkill.value) &&
    !filteredAvailableSkills.value.some((skill) => skill.id === selectedSkill.value?.id)
)

const selectedRevision = computed(
  () =>
    selectedSkillRevisions.value.find((revision) => revision.id === selectedRevisionID.value) ||
    null
)

const hasAcceptedRevision = computed(() =>
  selectedSkillRevisions.value.some((revision) => revision.status === 'accepted')
)

const latestCandidateRevision = computed(
  () => selectedSkillRevisions.value.find((revision) => revision.status === 'candidate') || null
)

const selectedSkillCase = computed(() => {
  const revision = selectedRevision.value
  if (!revision) return selectedSkillCases.value[0] || null
  return (
    selectedSkillCases.value.find((item) => item.id === revision.origin_case_id) ||
    selectedSkillCases.value.find((item) => item.revision_id === revision.id) ||
    selectedSkillCases.value[0] ||
    null
  )
})

const selectedSkillCaseDetail = computed<SkillEvolutionCaseDetail | null>(() => {
  const caseID = normalizeText(selectedSkillCase.value?.id)
  if (!caseID) return null
  return selectedSkillCaseDetailsByID.value[caseID] || null
})

const agentcoreRunnerStatus = computed(() => settingsStore.agentcoreRunnerStatus || null)
const agentcoreRunnerLastRun = computed(() => settingsStore.agentcoreRunnerLastRun || null)
const runnerOptimizationRecord = computed<Record<string, unknown> | null>(() =>
  asRecord(agentcoreRunnerLastRun.value)
)
const runnerOptimizationMetadata = computed<Record<string, unknown> | null>(() =>
  asRecord(runnerOptimizationRecord.value?.metadata)
)
const runnerMaterializedSkillCandidate = computed<Record<string, unknown> | null>(
  () =>
    asRecord(runnerOptimizationRecord.value?.materialized_skill_candidate) ||
    asRecord(runnerOptimizationMetadata.value?.skill_candidate)
)
const runnerLinkedSkillID = computed(() =>
  normalizeText(runnerMaterializedSkillCandidate.value?.skill_id)
)
const runnerLinkedRevisionID = computed(() =>
  normalizeText(
    runnerOptimizationRecord.value?.skill_revision_id ||
      runnerOptimizationMetadata.value?.skill_revision_id
  )
)
const runnerLinkedCaseID = computed(() =>
  normalizeText(
    runnerOptimizationRecord.value?.skill_evolution_case_id ||
      runnerOptimizationMetadata.value?.origin_case_id
  )
)
const runnerSourceEvalRunID = computed(() =>
  normalizeText(
    runnerOptimizationRecord.value?.source_eval_run_id ||
      runnerOptimizationRecord.value?.eval_run_id
  )
)
const runnerFollowupEvalRunID = computed(() =>
  normalizeText(runnerOptimizationRecord.value?.followup_eval_run_id)
)
const runnerFollowupGate = computed(() =>
  normalizeText(
    runnerOptimizationRecord.value?.followup_gate || runnerOptimizationMetadata.value?.followup_gate
  )
)
const runnerFollowupState = computed(() =>
  normalizeText(runnerOptimizationRecord.value?.followup_state)
)
const runnerOptimizedParts = computed(() => {
  const fromRun = decodeStringList(runnerOptimizationRecord.value?.optimized_parts)
  if (fromRun.length > 0) return fromRun
  return decodeStringList(agentcoreRunnerStatus.value?.optimized_parts)
})
const runnerPrimaryPart = computed(() =>
  normalizeText(
    runnerOptimizationRecord.value?.primary_part || agentcoreRunnerStatus.value?.primary_part
  )
)
const runnerTranscriptEntries = computed<
  Array<{ key: string; direction: string; method: string; text: string }>
>(() => {
  const rawEntries = Array.isArray(agentcoreRunnerLastRun.value?.runner_transcript)
    ? agentcoreRunnerLastRun.value?.runner_transcript
    : []
  return rawEntries
    .map((entry, index) => {
      const record = asRecord(entry)
      const direction = normalizeText(record?.direction)
      const method = normalizeText(record?.method)
      const text = normalizeText(record?.text)
      return {
        key: normalizeText(record?.id) || `${direction}-${method}-${index}`,
        direction,
        method,
        text,
      }
    })
    .filter((entry) => entry.text)
})
const runnerLinkedCase = computed(() => {
  const caseID = runnerLinkedCaseID.value
  if (!caseID) return null
  return selectedSkillCases.value.find((skillCase) => skillCase.id === caseID) || null
})
const runnerSourceReport = computed<HarnessEvalRunReport | null>(() => {
  const evalRunID = runnerSourceEvalRunID.value
  if (!evalRunID) return null
  return runnerReportsByEvalRunID.value[evalRunID] || null
})
const runnerFollowupReport = computed<HarnessEvalRunReport | null>(() => {
  const evalRunID = runnerFollowupEvalRunID.value
  if (!evalRunID) return null
  return runnerReportsByEvalRunID.value[evalRunID] || null
})
const runnerSummaryCards = computed<DecisionSummaryCard[]>(() => {
  const candidateID = normalizeText(runnerOptimizationRecord.value?.candidate_id)
  const linkedRevisionID = runnerLinkedRevisionID.value
  const followupLabel = runnerFollowupLabel(runnerFollowupState.value, runnerFollowupGate.value)
  const runtime = formatCompactDurationMs(runnerOptimizationRecord.value?.runner_duration_ms)
  const optimizedParts = runnerOptimizedParts.value
  const primaryPart = runnerPrimaryPart.value
  const localizedOptimizedParts = optimizedParts.map((part) => humanizeEnum(part))

  return [
    {
      key: 'candidate',
      label: tr('evolution.runner.summary.candidate', 'Linked revision'),
      value: linkedRevisionID || tr('common.notAvailable', 'Not available'),
      details:
        [candidateID, runnerLinkedSkillID.value].filter(Boolean).join(' · ') ||
        tr(
          'evolution.runner.summary.candidateHint',
          'The runner keeps the candidate and linked revision separate from the final skill promote step.'
        ),
      tone: linkedRevisionID ? 'emerald' : 'slate',
    },
    {
      key: 'followup',
      label: tr('evolution.runner.summary.followup', 'Follow-up gate'),
      value: followupLabel,
      details:
        normalizeText(runnerOptimizationRecord.value?.followup_message) ||
        normalizeText(agentcoreRunnerStatus.value?.last_optimization_summary) ||
        tr(
          'evolution.runner.summary.followupHint',
          'The linked follow-up eval decides whether the candidate is accepted, rejected, or still running.'
        ),
      tone:
        runnerFollowupState.value === 'accepted'
          ? 'emerald'
          : runnerFollowupState.value === 'rejected'
            ? 'rose'
            : runnerFollowupState.value === 'submitted' || runnerFollowupState.value === 'running'
              ? 'amber'
              : 'sky',
    },
    {
      key: 'runtime',
      label: tr('evolution.runner.summary.runtime', 'Runner runtime'),
      value: runtime,
      details:
        [
          normalizeText(runnerOptimizationRecord.value?.runner_protocol),
          humanizeOptionalEnum(runnerOptimizationRecord.value?.runner_stop_reason),
        ]
          .filter(Boolean)
          .join(' · ') ||
        tr(
          'evolution.runner.summary.runtimeHint',
          'Execution timing and stop reason explain how the optimization run behaved before the follow-up gate.'
        ),
      tone: normalizeText(runnerOptimizationRecord.value?.runner_error) ? 'rose' : 'sky',
    },
    {
      key: 'parts',
      label: tr('evolution.runner.summary.parts', 'Optimized parts'),
      value:
        optimizedParts.length > 0
          ? formatEvolutionCountLabel('partChanged', optimizedParts.length)
          : tr('evolution.runner.summary.partsEmpty', 'No part change'),
      details:
        [
          primaryPart
            ? `${tr('evolution.runner.summary.primaryPart', 'Primary')}: ${humanizeEnum(primaryPart)}`
            : '',
          localizedOptimizedParts.join(', '),
        ]
          .filter(Boolean)
          .join(' · ') ||
        tr(
          'evolution.runner.summary.partsHint',
          'Optimized parts show which runner surfaces changed without implying a live skill switch.'
        ),
      tone: optimizedParts.length > 0 ? 'sky' : 'slate',
    },
  ]
})
const runnerSourceReportMetrics = computed(() => buildRunnerReportMetrics(runnerSourceReport.value))
const runnerFollowupReportMetrics = computed(() =>
  buildRunnerReportMetrics(runnerFollowupReport.value)
)
const runnerCandidateContent = computed(() => {
  const content = runnerMaterializedSkillCandidate.value?.content
  return typeof content === 'string' ? content : ''
})
const runnerCandidateDiff = computed(() => {
  const content = runnerCandidateContent.value
  if (!content) return tr('evolution.noDiff', 'No candidate patch available yet.')
  if (
    selectedSkillContent.value?.content &&
    runnerLinkedSkillID.value &&
    runnerLinkedSkillID.value === selectedSkillID.value
  ) {
    return buildUnifiedDiff(selectedSkillContent.value.content || '', content)
  }
  return content
})
const runnerCandidateDiffSummary = computed(() =>
  patchSummaryEntries(summarizePatch(runnerCandidateDiff.value))
)

const selectedSkillDiff = computed(() => {
  const revision = selectedRevision.value
  if (!revision?.content) return tr('evolution.noDiff', 'No candidate patch available yet.')
  return buildUnifiedDiff(selectedSkillContent.value?.content || '', revision.content)
})

const decisionHistoryComparisonLeftRevision = computed(() => {
  const pair = decisionHistoryComparisonPair.value
  if (!pair) return null
  return (
    selectedSkillRevisions.value.find((revision) => revision.id === pair.leftRevisionID) || null
  )
})

const decisionHistoryComparisonRightRevision = computed(() => {
  const pair = decisionHistoryComparisonPair.value
  if (!pair) return null
  return (
    selectedSkillRevisions.value.find((revision) => revision.id === pair.rightRevisionID) || null
  )
})

const decisionHistoryComparisonDiff = computed(() => {
  const leftRevision = decisionHistoryComparisonLeftRevision.value
  const rightRevision = decisionHistoryComparisonRightRevision.value
  if (!leftRevision?.content || !rightRevision?.content) {
    return tr(
      'evolution.skills.decisionHistory.comparisonEmpty',
      'The selected history comparison is missing revision content.'
    )
  }
  return buildUnifiedDiff(leftRevision.content, rightRevision.content)
})

const selectedSkillPatchSummary = computed(() => summarizePatch(selectedSkillDiff.value))

const selectedSkillDiffSummary = computed(() =>
  patchSummaryEntries(selectedSkillPatchSummary.value)
)

const selectedSkillGuidanceAreas = computed(() => summarizeGuidanceAreas(selectedSkillDiff.value))

const selectedSkillEvidenceRecord = computed(() =>
  parseJSONRecord(selectedSkillCase.value?.evidence_json)
)

const selectedSkillEvidence = computed(() => prettyJSON(selectedSkillCase.value?.evidence_json))

const selectedSkillEvidenceSummary = computed<EvidenceSummaryEntry[]>(() => {
  const skillCase = selectedSkillCase.value
  const evidence = selectedSkillEvidenceRecord.value
  if (!skillCase && !evidence) return []

  const summary: EvidenceSummaryEntry[] = []
  const mode = normalizeText(skillCase?.mode)
  const validation = asRecord(evidence?.validation)
  const runtimeMetrics = asRecord(evidence?.runtime_metrics)
  const runtimeUsage = asRecord(evidence?.runtime_usage)
  const runtimeQuality = asRecord(evidence?.runtime_quality)

  if (mode === 'capture') {
    const lesson = normalizeText(evidence?.lesson)
    if (lesson) {
      summary.push({
        key: 'lesson',
        label: tr('evolution.skills.evidenceSummary.lesson', 'Captured lesson'),
        value: lesson,
        tone: 'slate',
      })
    }

    const whenToApply = normalizeText(evidence?.when_to_apply)
    if (whenToApply) {
      summary.push({
        key: 'when_to_apply',
        label: tr('evolution.skills.evidenceSummary.whenToApply', 'When to apply'),
        value: whenToApply,
        tone: 'amber',
      })
    }

    const confidence = Number(evidence?.confidence)
    if (Number.isFinite(confidence)) {
      summary.push({
        key: 'confidence',
        label: tr('evolution.skills.evidenceSummary.confidence', 'Confidence'),
        value: formatPercentValue(confidence),
        tone: confidence >= 0.75 ? 'emerald' : 'amber',
      })
    }

    const groundedness = Number(evidence?.groundedness)
    if (Number.isFinite(groundedness)) {
      summary.push({
        key: 'groundedness',
        label: tr('evolution.skills.evidenceSummary.groundedness', 'Groundedness'),
        value: formatPercentValue(groundedness),
        tone: groundedness >= 0.75 ? 'emerald' : 'amber',
      })
    }

    const evidenceIDs = decodeStringList(evidence?.evidence_ids)
    if (evidenceIDs.length > 0) {
      summary.push({
        key: 'evidence_ids',
        label: tr('evolution.skills.evidenceSummary.evidenceIDs', 'Evidence IDs'),
        value: new Intl.NumberFormat().format(evidenceIDs.length),
        tone: 'slate',
      })
    }
  } else {
    const failureSignature = normalizeText(
      skillCase?.failure_signature ||
        String(
          evidence?.failure_signature ||
            evidence?.runtime_failure_signature ||
            evidence?.capture_signature ||
            ''
        )
    )
    if (failureSignature) {
      summary.push({
        key: 'failure_signature',
        label: tr('evolution.skills.evidenceSummary.failureSignature', 'Failure signature'),
        value: failureSignature,
        tone: 'amber',
      })
    }

    const recoveredRaw = validation?.recovered ?? evidence?.recovered
    if (typeof recoveredRaw === 'boolean') {
      summary.push({
        key: 'recovered',
        label: tr('evolution.skills.evidenceSummary.recovered', 'Recovered'),
        value: boolSummaryLabel(recoveredRaw),
        tone: recoveredRaw ? 'emerald' : 'rose',
      })
    }

    const failureCountRaw = validation?.failure_count ?? evidence?.failure_count
    const failureCount = Number(failureCountRaw)
    if (Number.isFinite(failureCount)) {
      summary.push({
        key: 'failure_count',
        label: tr('evolution.skills.evidenceSummary.failureCount', 'Failure count'),
        value: new Intl.NumberFormat().format(failureCount),
        tone: failureCount > 0 ? 'amber' : 'slate',
      })
    }
  }

  const runtimeDurationMs = Number(runtimeMetrics?.duration_ms)
  if (Number.isFinite(runtimeDurationMs)) {
    summary.push({
      key: 'duration_ms',
      label: metricLabel('duration_ms'),
      value: formatMetricValue('duration_ms', runtimeDurationMs),
      tone: scorecardToneForDuration(runtimeDurationMs),
    })
  }

  const runtimeTotalTokens = Number(runtimeUsage?.total_tokens)
  if (Number.isFinite(runtimeTotalTokens)) {
    summary.push({
      key: 'total_tokens',
      label: metricLabel('total_tokens'),
      value: formatMetricValue('total_tokens', runtimeTotalTokens),
      tone: scorecardToneForTokens(runtimeTotalTokens),
    })
  }

  const verificationPassed = runtimeQuality?.verification_passed
  if (typeof verificationPassed === 'boolean') {
    summary.push({
      key: 'verification_passed',
      label: tr('evolution.skills.scorecard.verification', 'Verification'),
      value: boolSummaryLabel(verificationPassed),
      tone: verificationPassed ? 'emerald' : 'rose',
    })
  }

  const outcomeScore = Number(runtimeQuality?.outcome_score)
  if (Number.isFinite(outcomeScore)) {
    summary.push({
      key: 'outcome_score',
      label: tr('evolution.skills.scorecard.quality', 'Quality'),
      value: formatPercentValue(outcomeScore),
      tone: scorecardToneForPercent(outcomeScore),
    })
  }

  return summary
})

const selectedSkillMetrics = computed<MetricEntry[]>(() => {
  const report = selectedRevisionReport.value
  const metrics: MetricEntry[] = []
  const pushMetric = (key: string, fallbackValue?: unknown) => {
    const value = reportSummaryValue(report, key, fallbackValue)
    if (value == null) return
    metrics.push({
      key,
      label: metricLabel(key),
      value: formatMetricValue(key, value),
    })
  }

  pushMetric('overall_score', report?.group_report?.overall_score)
  pushMetric('pass_rate', report?.group_report?.pass_rate)
  pushMetric('verification_pass_rate')
  pushMetric('evidence_backed_pass_rate')
  pushMetric('critical_pass_rate')
  pushMetric('total_tokens')
  pushMetric('avg_duration_ms')
  pushMetric('duration_ms')
  return metrics
})

const acceptedRevisions = computed(() =>
  selectedSkillRevisions.value.filter((revision) => revision.status === 'accepted')
)

const selectedSkillAcceptedRevisionCount = computed(() => {
  if (loadedSkillDetailsID.value === selectedSkillID.value) {
    return acceptedRevisions.value.length
  }
  if (evolutionOverview.value?.skill_id === selectedSkillID.value) {
    return evolutionOverview.value.revisions.accepted
  }
  return 0
})

const selectedRevisionCanOptimize = computed(() => {
  const revision = selectedRevision.value
  return Boolean(revision?.eval_run_id && normalizeText(revision.skill_id))
})

const selectedSkillComparison = computed<{
  baselineEvalRunID: string
  deltas: MetricDeltaEntry[]
}>(() => {
  const report = selectedRevisionReport.value
  const baselineEvalRunID = normalizeText(
    String(
      reportSummaryValue(report, 'baseline_eval_run_id', report?.eval_run?.baseline_eval_run_id) ||
        ''
    )
  )
  const keys = [
    'overall_score_delta',
    'pass_rate_delta',
    'verification_pass_rate_delta',
    'evidence_backed_pass_rate_delta',
  ]
  const deltas = keys.flatMap((key) => {
    const value = reportSummaryValue(report, key)
    if (value == null) return []
    return [
      {
        key,
        label: comparisonMetricLabel(key),
        value: formatSignedMetricValue(key, value),
        tone: metricDeltaTone(value),
      },
    ]
  })
  return { baselineEvalRunID, deltas }
})

const selectedSkillComparisonSnapshot = computed(() => {
  return selectedSkillComparison.value.deltas
    .slice(0, 2)
    .map((entry) => `${entry.label} ${entry.value}`)
    .join(' · ')
})

const selectedSkillMetricSnapshot = computed(() => {
  const preferredKeys = ['overall_score', 'pass_rate', 'total_tokens', 'avg_duration_ms']
  const entries = preferredKeys
    .map((key) => selectedSkillMetrics.value.find((entry) => entry.key === key))
    .filter(Boolean) as MetricEntry[]
  return entries
    .slice(0, 2)
    .map((entry) => `${entry.label} ${entry.value}`)
    .join(' · ')
})

const backupRevisions = computed(() =>
  selectedSkillRevisions.value.filter((revision) => revision.status === 'backup')
)

const filteredSelectedSkillRevisions = computed(() =>
  selectedSkillRevisions.value.filter((revision) => {
    const matchesStatus =
      revisionStatusFilter.value === 'all' ||
      normalizeText(revision.status) === revisionStatusFilter.value
    const matchesQuery = matchesSearchQuery(
      revisionSearch.value,
      revision.id,
      revision.skill_id,
      revision.source_path,
      revision.candidate_id,
      revision.origin_case_id,
      revision.eval_run_id,
      revision.parent_revision_id,
      revision.backup_of_revision_id,
      revision.optimization_surface,
      humanizeEnum(revision.status)
    )
    return matchesStatus && matchesQuery
  })
)

const selectedRevisionHiddenByFilters = computed(
  () =>
    Boolean(selectedRevision.value) &&
    !filteredSelectedSkillRevisions.value.some(
      (revision) => revision.id === selectedRevision.value?.id
    )
)

const filteredSelectedSkillCases = computed(() =>
  selectedSkillCases.value.filter((skillCase) => {
    const matchesStatus =
      caseStatusFilter.value === 'all' || normalizeText(skillCase.status) === caseStatusFilter.value
    const matchesMode =
      caseModeFilter.value === 'all' || normalizeText(skillCase.mode) === caseModeFilter.value
    const matchesQuery = matchesSearchQuery(
      caseSearch.value,
      skillCase.id,
      skillCase.summary,
      skillCase.candidate_id,
      skillCase.revision_id,
      skillCase.source_id,
      humanizeEnum(skillCase.reason),
      humanizeEnum(skillCase.status),
      humanizeEnum(skillCase.mode)
    )
    return matchesStatus && matchesMode && matchesQuery
  })
)

const selectedSkillCaseHiddenByFilters = computed(
  () =>
    Boolean(selectedSkillCase.value) &&
    !filteredSelectedSkillCases.value.some(
      (skillCase) => skillCase.id === selectedSkillCase.value?.id
    )
)

const selectedCaseRevision = computed(() => {
  const skillCase = selectedSkillCase.value
  const revision = selectedRevision.value
  if (selectedSkillCaseDetail.value?.linked_revision) {
    return selectedSkillCaseDetail.value.linked_revision
  }
  if (!skillCase) return revision
  return (
    selectedSkillRevisions.value.find((item) => item.id === skillCase.revision_id) ||
    (revision && (revision.id === skillCase.revision_id || revision.origin_case_id === skillCase.id)
      ? revision
      : null)
  )
})

const selectedCasePromotedRevision = computed(() => {
  const skillCase = selectedSkillCase.value
  if (selectedSkillCaseDetail.value?.linked_revision?.status === 'promoted') {
    return selectedSkillCaseDetail.value.linked_revision
  }
  if (!skillCase) return null
  return (
    selectedSkillRevisions.value.find(
      (revision) =>
        revision.status === 'promoted' &&
        (revision.id === skillCase.revision_id || revision.origin_case_id === skillCase.id)
    ) || null
  )
})

const selectedSkillCaseTimeline = computed<CaseTimelineEntry[]>(() => {
  const skillCase = selectedSkillCase.value
  if (!skillCase) return []

  const status = normalizeText(skillCase.status)
  const caseRevision = selectedCaseRevision.value
  const promotedRevision = selectedCasePromotedRevision.value
  const candidateTime = formatDate(
    caseRevision?.created_at || skillCase.updated_at || skillCase.created_at
  )
  const gateTime = formatDate(
    skillCase.updated_at || caseRevision?.created_at || skillCase.created_at
  )
  const promotedTime = formatDate(
    promotedRevision?.promoted_at ||
      promotedRevision?.created_at ||
      caseRevision?.promoted_at ||
      skillCase.updated_at
  )

  const entries: CaseTimelineEntry[] = [
    {
      key: 'open',
      label: tr('evolution.skills.timeline.open', 'Case opened'),
      time: formatDate(skillCase.created_at),
      state: status === 'open' ? 'current' : 'completed',
    },
    {
      key: 'candidate_created',
      label: tr('evolution.skills.timeline.candidateCreated', 'Candidate created'),
      time: candidateTime,
      state:
        status === 'open' ? 'upcoming' : status === 'candidate_created' ? 'current' : 'completed',
    },
  ]

  if (status === 'rejected') {
    entries.push({
      key: 'rejected',
      label: tr('evolution.skills.timeline.rejected', 'Rejected by gate'),
      time: gateTime,
      state: 'current',
    })
    return entries
  }

  if (status === 'skipped') {
    entries.push({
      key: 'skipped',
      label: tr('evolution.skills.timeline.skipped', 'Skipped'),
      time: gateTime,
      state: 'current',
    })
    return entries
  }

  entries.push({
    key: 'accepted',
    label: tr('evolution.skills.timeline.accepted', 'Accepted by gate'),
    time: gateTime,
    state:
      status === 'open' || status === 'candidate_created'
        ? 'upcoming'
        : status === 'accepted'
          ? 'current'
          : 'completed',
  })

  entries.push({
    key: 'promoted',
    label: tr('evolution.skills.timeline.promoted', 'Promoted to canonical'),
    time: promotedTime,
    state: status === 'promoted' ? 'current' : status === 'accepted' ? 'upcoming' : 'upcoming',
  })

  return entries
})

const currentCanonicalRevision = computed(
  () => selectedSkillRevisions.value.find((revision) => revision.status === 'promoted') || null
)

const selectedSkillDecisionHistorySource = computed<SkillDecisionHistoryRecord[]>(() => {
  if (selectedSkillDecisionHistoryLoaded.value) {
    return selectedSkillDecisionHistoryRecords.value
  }
  return selectedSkillRevisions.value
    .filter((revision) => hasPersistedSkillRevisionDecision(revision))
    .map((revision) => fallbackDecisionHistoryRecord(revision))
    .sort((left, right) => skillDecisionHistoryMoment(right) - skillDecisionHistoryMoment(left))
})

const selectedSkillDecisionHistory = computed<DecisionHistoryEntry[]>(() => {
  const selectedRevisionID = normalizeText(selectedRevision.value?.id)
  const liveRevisionID = normalizeText(currentCanonicalRevision.value?.id)

  return [...selectedSkillDecisionHistorySource.value]
    .sort((left, right) => skillDecisionHistoryMoment(right) - skillDecisionHistoryMoment(left))
    .map((entry) => {
      const decisionLog = entry.decision_log || parseJSONRecord(entry.decision_log_json)
      const action = normalizeText(
        typeof decisionLog?.action === 'string' ? decisionLog.action : entry.decision_action
      )
      const selectedDecisionRevisionID = normalizeText(
        typeof decisionLog?.selected_revision_id === 'string'
          ? decisionLog.selected_revision_id
          : ''
      )
      const sourceRevisionID = normalizeText(
        typeof decisionLog?.source_revision_id === 'string' ? decisionLog.source_revision_id : ''
      )
      const targetRevisionID =
        normalizeText(
          typeof decisionLog?.target_revision_id === 'string' ? decisionLog.target_revision_id : ''
        ) || entry.revision_id
      const currentLiveRevisionID = normalizeText(
        typeof decisionLog?.current_live_revision_id === 'string'
          ? decisionLog.current_live_revision_id
          : ''
      )
      const backupRevisionID = normalizeText(
        typeof decisionLog?.backup_revision_id === 'string' ? decisionLog.backup_revision_id : ''
      )
      const evalRunID = skillDecisionHistoryEvalRunID(entry)
      const writtenSourcePath = normalizeText(
        typeof decisionLog?.written_source_path === 'string' ? decisionLog.written_source_path : ''
      )
      const evidenceSummary = buildDecisionHistoryEvidenceSummary(
        evalRunID ? decisionHistoryReportsByEvalRunID.value[evalRunID] || null : null
      )
      const reviewNote =
        normalizeText(entry.review_note) ||
        normalizeText(typeof decisionLog?.review_note === 'string' ? decisionLog.review_note : '')
      const reviewedBy =
        normalizeText(entry.reviewed_by) ||
        normalizeText(
          typeof decisionLog?.reviewed_by === 'string' ? decisionLog.reviewed_by : ''
        ) ||
        tr('common.notAvailable', 'Not available')
      const reviewedAt =
        entry.reviewed_at ||
        entry.decision_at ||
        normalizeText(
          typeof decisionLog?.reviewed_at === 'string' ? decisionLog.reviewed_at : ''
        ) ||
        entry.promoted_at ||
        entry.created_at

      let title = tr('evolution.skills.decisionHistory.title.recorded', 'Decision recorded')
      let summary = tr(
        'evolution.skills.decisionHistory.summary.recorded',
        'A persisted review decision is attached to this lineage revision.'
      )
      if (action === 'promote') {
        title = tr('evolution.skills.decisionHistory.title.promote', 'Promoted to canonical')
        summary = backupRevisionID
          ? trp(
              'evolution.skills.decisionHistory.summary.promoteWithBackup',
              'Promoted {selected} live and preserved {backup} as the rollback backup.',
              {
                selected: selectedDecisionRevisionID || targetRevisionID,
                backup: backupRevisionID,
              }
            )
          : trp(
              'evolution.skills.decisionHistory.summary.promote',
              'Promoted {selected} live as the canonical version.',
              {
                selected: selectedDecisionRevisionID || targetRevisionID,
              }
            )
      } else if (action === 'rollback') {
        title = tr('evolution.skills.decisionHistory.title.rollback', 'Rollback promoted live')
        summary = trp(
          'evolution.skills.decisionHistory.summary.rollback',
          'Restored {source} over live {live} and preserved {backup} as the new backup.',
          {
            source: sourceRevisionID || selectedDecisionRevisionID || entry.revision_id,
            live:
              currentLiveRevisionID ||
              tr('evolution.skills.decisionHistory.summary.liveUnknown', 'the prior live version'),
            backup:
              backupRevisionID ||
              tr('evolution.skills.decisionHistory.summary.backupUnknown', 'a fresh backup'),
          }
        )
      }

      return {
        key: entry.revision_id,
        revisionID: entry.revision_id,
        actionLabel: decisionActionLabel(action),
        title,
        summary,
        reviewedAt: formatDate(reviewedAt),
        tone: decisionTone(action),
        badges: [
          entry.revision_id === selectedRevisionID
            ? {
                key: 'selected',
                label: tr('evolution.skills.decisionHistory.badge.selected', 'Selected revision'),
              }
            : null,
          entry.revision_id === liveRevisionID
            ? {
                key: 'live',
                label: tr('evolution.skills.decisionHistory.badge.live', 'Live now'),
              }
            : null,
        ].filter(Boolean) as DecisionHistoryBadge[],
        details: [
          {
            key: 'recorded_revision',
            label: tr('evolution.skills.decisionHistory.recordedRevision', 'Recorded on revision'),
            value: entry.revision_id,
          },
          action === 'rollback'
            ? {
                key: 'source_revision',
                label: tr(
                  'evolution.skills.decisionHistory.sourceRevision',
                  'Restored from backup'
                ),
                value:
                  sourceRevisionID ||
                  selectedDecisionRevisionID ||
                  tr('common.notAvailable', 'Not available'),
              }
            : {
                key: 'selected_revision',
                label: tr('evolution.skills.decisionHistory.selectedRevision', 'Selected revision'),
                value:
                  selectedDecisionRevisionID ||
                  targetRevisionID ||
                  tr('common.notAvailable', 'Not available'),
              },
          {
            key: 'target_revision',
            label: tr('evolution.skills.decisionHistory.targetRevision', 'Target revision'),
            value: targetRevisionID || tr('common.notAvailable', 'Not available'),
          },
          currentLiveRevisionID
            ? {
                key: 'current_live_revision',
                label: tr(
                  'evolution.skills.decisionHistory.currentLiveRevision',
                  'Live revision at decision time'
                ),
                value: currentLiveRevisionID,
              }
            : null,
          backupRevisionID
            ? {
                key: 'backup_revision',
                label: tr('evolution.skills.decisionHistory.backupRevision', 'Backup revision'),
                value: backupRevisionID,
              }
            : null,
          {
            key: 'reviewed_by',
            label: tr('evolution.skills.meta.reviewedBy', 'Reviewed by'),
            value: reviewedBy,
          },
          {
            key: 'reviewed_at',
            label: tr('evolution.skills.meta.reviewedAt', 'Reviewed at'),
            value: formatDate(reviewedAt),
          },
          {
            key: 'review_note',
            label: tr('evolution.skills.meta.reviewNote', 'Review note'),
            value:
              reviewNote ||
              tr('evolution.skills.decisionHistory.reviewNoteEmpty', 'No review note recorded'),
          },
          writtenSourcePath
            ? {
                key: 'written_source_path',
                label: tr(
                  'evolution.skills.decisionHistory.writtenSourcePath',
                  'Written source path'
                ),
                value: writtenSourcePath,
              }
            : null,
        ].filter(Boolean) as DecisionHistoryDetail[],
        evidenceSummary: evidenceSummary.length > 0 ? evidenceSummary : undefined,
        links: {
          revisionID: entry.revision_id,
          evalRunID: evalRunID || undefined,
          backupRevisionID: backupRevisionID || undefined,
          sourceRevisionID: sourceRevisionID || undefined,
          targetRevisionID: targetRevisionID || undefined,
          currentLiveRevisionID: currentLiveRevisionID || undefined,
        },
      }
    })
})

function revisionBadges(revision: SkillRevision): RevisionBadge[] {
  const badges: RevisionBadge[] = []
  if (currentCanonicalRevision.value?.id === revision.id) {
    badges.push({
      key: 'current',
      label: tr('evolution.skills.badges.currentCanonical', 'Current canonical'),
      tone: 'sky',
    })
  } else if (revision.status === 'promoted') {
    badges.push({
      key: 'history',
      label: tr('evolution.skills.badges.promotedHistory', 'Promoted history'),
      tone: 'slate',
    })
  }
  if (revision.status === 'accepted') {
    badges.push({
      key: 'ready',
      label: tr('evolution.skills.badges.readyToPromote', 'Ready to promote'),
      tone: 'emerald',
    })
  }
  if (revision.status === 'backup') {
    badges.push({
      key: 'backup',
      label: tr('evolution.skills.badges.restorableBackup', 'Restorable backup'),
      tone: 'amber',
    })
  }
  return badges
}

const selectedRevisionBadges = computed(() =>
  selectedRevision.value ? revisionBadges(selectedRevision.value) : []
)

const selectedRevisionLineage = computed(() => {
  const revision = selectedRevision.value
  if (!revision) return [] as Array<{ label: string; value: string }>
  return [
    currentCanonicalRevision.value?.id === revision.id
      ? {
          label: tr('evolution.skills.lineage.versionRole', 'Version role'),
          value: tr('evolution.skills.badges.currentCanonical', 'Current canonical'),
        }
      : null,
    revision.status === 'accepted'
      ? {
          label: tr('evolution.skills.lineage.versionRole', 'Version role'),
          value: tr('evolution.skills.badges.readyToPromote', 'Ready to promote'),
        }
      : null,
    revision.status === 'backup'
      ? {
          label: tr('evolution.skills.lineage.versionRole', 'Version role'),
          value: tr('evolution.skills.badges.restorableBackup', 'Restorable backup'),
        }
      : null,
    normalizeText(revision.backup_of_revision_id)
      ? {
          label: tr('evolution.skills.lineage.backupOf', 'Backup of revision'),
          value: normalizeText(revision.backup_of_revision_id),
        }
      : null,
    normalizeText(revision.parent_revision_id)
      ? {
          label: tr('evolution.skills.lineage.parentRevision', 'Parent revision'),
          value: normalizeText(revision.parent_revision_id),
        }
      : null,
  ].filter(Boolean) as Array<{ label: string; value: string }>
})

const selectedRevisionMeta = computed(() => {
  const revision = selectedRevision.value
  if (!revision) return [] as Array<{ label: string; value: string }>
  return [
    {
      label: tr('evolution.skills.meta.sourcePath', 'Canonical path'),
      value: normalizeText(revision.source_path),
    },
    {
      label: tr('evolution.skills.meta.candidateId', 'Candidate ID'),
      value: normalizeText(revision.candidate_id),
    },
    {
      label: tr('evolution.skills.meta.evalRunId', 'Eval run ID'),
      value: normalizeText(revision.eval_run_id),
    },
    {
      label: tr('evolution.skills.meta.originCaseId', 'Origin case ID'),
      value: normalizeText(revision.origin_case_id),
    },
    {
      label: tr('evolution.skills.meta.baseSha', 'Base SHA256'),
      value: normalizeText(revision.base_content_sha256),
    },
  ].filter((entry) => entry.value)
})

const selectedRevisionReviewNote = computed({
  get(): string {
    const revisionID = normalizeText(selectedRevisionID.value)
    if (!revisionID) return ''
    return skillReviewNotesByRevisionID.value[revisionID] || ''
  },
  set(value: string) {
    const revisionID = normalizeText(selectedRevisionID.value)
    if (!revisionID) return
    skillReviewNotesByRevisionID.value = {
      ...skillReviewNotesByRevisionID.value,
      [revisionID]: value,
    }
  },
})

const selectedRevisionReviewNoteTrimmed = computed(() =>
  normalizeText(selectedRevisionReviewNote.value)
)

const selectedRevisionReviewNoteSuggestions = computed(() => {
  const revision = selectedRevision.value
  if (!revision) return [] as string[]
  switch (revision.status) {
    case 'accepted':
      return [
        tr(
          'evolution.skills.reviewNoteSuggestion.acceptedEvidence',
          'Passed gate with grounded evidence and clear operator value.'
        ),
        tr(
          'evolution.skills.reviewNoteSuggestion.acceptedComparison',
          'Comparison and metrics support switching this candidate live.'
        ),
      ]
    case 'backup':
      return [
        tr(
          'evolution.skills.reviewNoteSuggestion.backupRestore',
          'Restore the previous stable canonical behavior while preserving the current live version as backup.'
        ),
        tr(
          'evolution.skills.reviewNoteSuggestion.backupLineage',
          'Rollback is justified because the current lineage still matches this backup chain.'
        ),
      ]
    default:
      return [
        tr(
          'evolution.skills.reviewNoteSuggestion.general',
          'Reviewer checked the linked evidence, patch scope, and operator impact.'
        ),
      ]
  }
})

const selectedRevisionSignOffCards = computed<SkillReviewCard[]>(() => {
  const revision = selectedRevision.value
  if (!revision || (revision.status !== 'accepted' && revision.status !== 'backup')) return []

  const reviewNote = selectedRevisionReviewNoteTrimmed.value
  const currentLiveRevisionID = normalizeText(currentCanonicalRevision.value?.id)
  const evidenceSnapshot = selectedSkillEvidenceSnapshot.value
  const reviewTrailSnapshot =
    selectedSkillComparisonSnapshot.value || selectedSkillMetricSnapshot.value
  const reviewTrailDetails = [evidenceSnapshot, reviewTrailSnapshot].filter(Boolean).join(' · ')

  if (revision.status === 'accepted') {
    const hasSafetyGuard = Boolean(normalizeText(revision.base_content_sha256))
    const hasReviewTrail = Boolean(reviewTrailDetails)

    return [
      {
        key: 'decision',
        label: tr('evolution.skills.signoff.decision', 'Final action'),
        value: tr('evolution.skills.signoff.promote', 'Promote to canonical'),
        details: currentLiveRevisionID
          ? `${currentLiveRevisionID} -> ${revision.id}`
          : tr(
              'evolution.skills.signoff.promoteFirstLive',
              'This accepted revision becomes the first recorded live canonical version.'
            ),
        tone: 'emerald',
      },
      {
        key: 'checkpoint',
        label: tr('evolution.skills.signoff.checkpoint', 'Human checkpoint'),
        value: reviewNote
          ? tr('evolution.skills.signoff.rationaleCaptured', 'Rationale captured')
          : tr('evolution.skills.signoff.rationaleOptional', 'Rationale optional'),
        details:
          reviewNote ||
          tr(
            'evolution.skills.signoff.rationaleOptionalHint',
            'Promote is still explicit either way, but a short rationale makes the final switch easier to audit later.'
          ),
        tone: reviewNote ? 'emerald' : 'amber',
      },
      {
        key: 'trail',
        label: tr('evolution.skills.signoff.trail', 'Review trail'),
        value: hasReviewTrail
          ? tr('evolution.skills.signoff.trailReady', 'Evidence and metrics reviewed')
          : tr('evolution.skills.signoff.trailNeedsReview', 'Review details again'),
        details:
          reviewTrailDetails ||
          tr(
            'evolution.skills.signoff.trailNeedsReviewHint',
            'Use the evidence, comparison, metrics, and diff sections before the final promote call.'
          ),
        tone: hasReviewTrail ? 'emerald' : 'amber',
      },
      {
        key: 'guard',
        label: tr('evolution.skills.signoff.guard', 'Safety guard'),
        value: hasSafetyGuard
          ? tr('evolution.skills.signoff.baseShaActive', 'Base SHA guard active')
          : tr('evolution.skills.signoff.baseShaMissing', 'Base SHA missing'),
        details: normalizeText(revision.base_content_sha256)
          ? `${tr('evolution.skills.meta.baseSha', 'Base SHA256')}: ${normalizeText(revision.base_content_sha256)}`
          : tr(
              'evolution.skills.signoff.baseShaMissingHint',
              'Promote should only proceed when the candidate still points at the canonical content it was evaluated against.'
            ),
        tone: hasSafetyGuard ? 'emerald' : 'rose',
      },
    ]
  }

  const backupOfRevisionID = normalizeText(revision.backup_of_revision_id)

  return [
    {
      key: 'decision',
      label: tr('evolution.skills.signoff.decision', 'Final action'),
      value: tr('evolution.skills.signoff.rollback', 'Rollback to this backup'),
      details: currentLiveRevisionID
        ? `${currentLiveRevisionID} -> ${backupOfRevisionID || revision.id}`
        : tr(
            'evolution.skills.signoff.rollbackTargetHint',
            'This backup becomes the canonical source again if the lineage guard still matches.'
          ),
      tone: 'amber',
    },
    {
      key: 'checkpoint',
      label: tr('evolution.skills.signoff.checkpoint', 'Human checkpoint'),
      value: reviewNote
        ? tr('evolution.skills.signoff.rationaleCaptured', 'Rationale captured')
        : tr('evolution.skills.signoff.rationaleOptional', 'Rationale optional'),
      details:
        reviewNote ||
        tr(
          'evolution.skills.signoff.rollbackRationaleHint',
          'Rollback remains explicit either way, but a short rationale helps explain why restoring this prior behavior is the right move.'
        ),
      tone: reviewNote ? 'emerald' : 'amber',
    },
    {
      key: 'trail',
      label: tr('evolution.skills.signoff.trail', 'Review trail'),
      value: tr('evolution.skills.signoff.rollbackTrailReady', 'Rollback impact reviewed'),
      details:
        [
          selectedRevisionRollbackImpactCards.value.find((card) => card.key === 'impact')
            ?.details || '',
          selectedRevisionRollbackImpactCards.value.find((card) => card.key === 'target')
            ?.details || '',
        ]
          .filter(Boolean)
          .join(' · ') ||
        tr(
          'evolution.skills.signoff.rollbackTrailHint',
          'Confirm which live revision is being replaced and which canonical lineage this backup belongs to.'
        ),
      tone: 'sky',
    },
    {
      key: 'guard',
      label: tr('evolution.skills.signoff.guard', 'Safety guard'),
      value: tr('evolution.skills.signoff.lineageGuardActive', 'Lineage guard active'),
      details: tr(
        'evolution.skills.signoff.lineageGuardHint',
        'Rollback is rejected instead of overwriting newer canonical content when the live lineage has already moved on.'
      ),
      tone: 'emerald',
    },
  ]
})

const selectedRevisionDecisionLogEntries = computed<DecisionLogPreviewEntry[]>(() => {
  const revision = selectedRevision.value
  if (!revision || (revision.status !== 'accepted' && revision.status !== 'backup')) return []

  const reviewNote = selectedRevisionReviewNoteTrimmed.value
  const currentLiveRevisionID = normalizeText(currentCanonicalRevision.value?.id)

  if (revision.status === 'accepted') {
    return [
      {
        key: 'action',
        label: tr('evolution.skills.signoff.log.action', 'Planned action'),
        value: tr('evolution.skills.signoff.promote', 'Promote to canonical'),
        tone: 'emerald',
      },
      {
        key: 'current',
        label: tr('evolution.skills.signoff.log.current', 'Current live'),
        value: currentLiveRevisionID || tr('common.notAvailable', 'Not available'),
        tone: currentLiveRevisionID ? 'sky' : 'slate',
      },
      {
        key: 'target',
        label: tr('evolution.skills.signoff.log.target', 'Target revision'),
        value: revision.id,
        tone: 'emerald',
      },
      {
        key: 'source',
        label: tr('evolution.skills.signoff.log.source', 'Evidence source'),
        value:
          normalizeText(revision.origin_case_id) ||
          normalizeText(revision.eval_run_id) ||
          tr('evolution.skills.signoff.log.sourceMissing', 'Review selected evidence'),
        tone:
          normalizeText(revision.origin_case_id) || normalizeText(revision.eval_run_id)
            ? 'sky'
            : 'amber',
      },
      {
        key: 'guard',
        label: tr('evolution.skills.signoff.log.guard', 'Safety guard'),
        value:
          normalizeText(revision.base_content_sha256) ||
          tr('evolution.skills.signoff.log.baseShaMissing', 'Base SHA missing'),
        tone: normalizeText(revision.base_content_sha256) ? 'emerald' : 'rose',
      },
      {
        key: 'rationale',
        label: tr('evolution.skills.signoff.log.rationale', 'Operator rationale'),
        value:
          reviewNote || tr('evolution.skills.signoff.log.rationaleMissing', 'Not captured yet'),
        tone: reviewNote ? 'emerald' : 'amber',
      },
    ]
  }

  return [
    {
      key: 'action',
      label: tr('evolution.skills.signoff.log.action', 'Planned action'),
      value: tr('evolution.skills.signoff.rollback', 'Rollback to this backup'),
      tone: 'amber',
    },
    {
      key: 'current',
      label: tr('evolution.skills.signoff.log.current', 'Current live'),
      value: currentLiveRevisionID || tr('common.notAvailable', 'Not available'),
      tone: currentLiveRevisionID ? 'sky' : 'slate',
    },
    {
      key: 'source',
      label: tr('evolution.skills.signoff.log.backupSource', 'Source backup'),
      value: revision.id,
      tone: 'amber',
    },
    {
      key: 'lineage',
      label: tr('evolution.skills.signoff.log.lineage', 'Lineage link'),
      value:
        normalizeText(revision.backup_of_revision_id) ||
        tr('evolution.skills.signoff.log.lineageMissing', 'No linked canonical revision'),
      tone: normalizeText(revision.backup_of_revision_id) ? 'sky' : 'slate',
    },
    {
      key: 'guard',
      label: tr('evolution.skills.signoff.log.guard', 'Safety guard'),
      value: tr('evolution.skills.signoff.lineageGuardActive', 'Lineage guard active'),
      tone: 'emerald',
    },
    {
      key: 'rationale',
      label: tr('evolution.skills.signoff.log.rationale', 'Operator rationale'),
      value: reviewNote || tr('evolution.skills.signoff.log.rationaleMissing', 'Not captured yet'),
      tone: reviewNote ? 'emerald' : 'amber',
    },
  ]
})

const selectedSkillEvidenceSnapshot = computed(() => {
  const entries = selectedSkillEvidenceSummary.value.slice(0, 2)
  if (entries.length > 0) {
    return entries.map((entry) => `${entry.label} ${entry.value}`).join(' · ')
  }
  const summary = normalizeText(selectedSkillCase.value?.summary)
  return summary
})

const selectedRevisionPromoteReadinessCards = computed<SkillReviewCard[]>(() => {
  const revision = selectedRevision.value
  if (!revision || revision.status !== 'accepted') return []

  const hasEvidence =
    Boolean(selectedSkillCase.value) &&
    (selectedSkillEvidenceSummary.value.length > 0 ||
      Boolean(normalizeText(selectedSkillCase.value?.summary)))
  const hasEvalLink = Boolean(normalizeText(revision.eval_run_id))
  const hasMetrics = selectedSkillMetrics.value.length > 0
  const hasComparison =
    Boolean(selectedSkillComparison.value.baselineEvalRunID) ||
    selectedSkillComparison.value.deltas.length > 0
  const hasSafetyMetadata =
    Boolean(normalizeText(revision.source_path)) &&
    Boolean(normalizeText(revision.base_content_sha256))

  return [
    {
      key: 'gate',
      label: tr('evolution.skills.promoteChecklist.gate', 'Gate result'),
      value: tr('evolution.skills.promoteChecklist.gateAccepted', 'Accepted by gate'),
      details: revisionStatusSummary(revision.status),
      tone: 'emerald',
    },
    {
      key: 'evidence',
      label: tr('evolution.skills.promoteChecklist.evidence', 'Evidence link'),
      value: hasEvidence
        ? tr('evolution.skills.promoteChecklist.evidenceAttached', 'Evidence attached')
        : tr('evolution.skills.promoteChecklist.evidenceReview', 'Review evidence first'),
      details:
        selectedSkillEvidenceSnapshot.value ||
        tr(
          'evolution.skills.promoteChecklist.evidenceHint',
          'Link a case summary or grounded lesson before promoting the candidate.'
        ),
      tone: hasEvidence ? 'emerald' : 'amber',
    },
    {
      key: 'evaluation',
      label: tr('evolution.skills.promoteChecklist.evaluation', 'Eval coverage'),
      value: hasMetrics
        ? tr('evolution.skills.promoteChecklist.metricsAttached', 'Metrics attached')
        : hasEvalLink
          ? tr('evolution.skills.promoteChecklist.evalLinked', 'Eval linked')
          : tr('evolution.skills.promoteChecklist.evalMissing', 'No eval linked'),
      details: hasComparison
        ? [
            tr(
              'evolution.skills.promoteChecklist.comparisonAttached',
              'Baseline comparison is attached.'
            ),
            selectedSkillComparisonSnapshot.value,
          ]
            .filter(Boolean)
            .join(' ')
        : hasMetrics
          ? selectedSkillMetricSnapshot.value
          : tr(
              'evolution.skills.promoteChecklist.evalHint',
              'Open the linked eval or wait for metrics before the final promote decision.'
            ),
      tone: hasMetrics ? 'emerald' : hasEvalLink ? 'amber' : 'rose',
    },
    {
      key: 'safety',
      label: tr('evolution.skills.promoteChecklist.safety', 'Canonical safety'),
      value: hasSafetyMetadata
        ? tr('evolution.skills.promoteChecklist.safetyReady', 'Base SHA recorded')
        : tr('evolution.skills.promoteChecklist.safetyMissing', 'Safety metadata missing'),
      details: [
        normalizeText(revision.source_path)
          ? `${tr('evolution.skills.meta.sourcePath', 'Canonical path')}: ${normalizeText(revision.source_path)}`
          : '',
        normalizeText(revision.base_content_sha256)
          ? `${tr('evolution.skills.meta.baseSha', 'Base SHA256')}: ${normalizeText(revision.base_content_sha256)}`
          : '',
      ]
        .filter(Boolean)
        .join(' · '),
      tone: hasSafetyMetadata ? 'emerald' : 'rose',
    },
  ]
})

const selectedRevisionRollbackImpactCards = computed<SkillReviewCard[]>(() => {
  const revision = selectedRevision.value
  if (!revision || revision.status !== 'backup') return []

  const sourcePath = normalizeText(revision.source_path)
  const backupOfRevisionID = normalizeText(revision.backup_of_revision_id)
  const currentLiveRevisionID = normalizeText(currentCanonicalRevision.value?.id)

  return [
    {
      key: 'target',
      label: tr('evolution.skills.rollbackSummary.target', 'Rollback target'),
      value: backupOfRevisionID
        ? tr('evolution.skills.rollbackSummary.previousCanonical', 'Restore previous canonical')
        : tr('evolution.skills.rollbackSummary.backupSelected', 'Backup revision selected'),
      details: backupOfRevisionID
        ? `${tr('evolution.skills.lineage.backupOf', 'Backup of revision')}: ${backupOfRevisionID}`
        : tr(
            'evolution.skills.rollbackSummary.targetHint',
            'This backup preserves the previous canonical content.'
          ),
      tone: 'amber',
    },
    {
      key: 'live',
      label: tr('evolution.skills.rollbackSummary.live', 'Current live revision'),
      value: currentLiveRevisionID || tr('common.notAvailable', 'Not available'),
      details: currentLiveRevisionID
        ? tr(
            'evolution.skills.rollbackSummary.liveHint',
            'Rollback is only safe while the current canonical lineage still matches this backup chain.'
          )
        : tr(
            'evolution.skills.rollbackSummary.liveMissing',
            'No promoted canonical revision is recorded in the current lineage.'
          ),
      tone: currentLiveRevisionID ? 'sky' : 'slate',
    },
    {
      key: 'impact',
      label: tr('evolution.skills.rollbackSummary.impact', 'Rollback impact'),
      value: tr('evolution.skills.rollbackSummary.impactValue', 'Canonical skill file rewrite'),
      details: sourcePath
        ? `${tr('evolution.skills.meta.sourcePath', 'Canonical path')}: ${sourcePath}`
        : tr(
            'evolution.skills.rollbackSummary.impactHint',
            'Rollback restores the previous canonical content for this skill.'
          ),
      tone: 'amber',
    },
    {
      key: 'guard',
      label: tr('evolution.skills.rollbackSummary.guard', 'Safety guard'),
      value: tr('evolution.skills.rollbackSummary.guardValue', 'Lineage must still match'),
      details: tr(
        'evolution.skills.rollbackSummary.guardHint',
        'Rollback is rejected instead of overwriting newer canonical content when the live lineage has already moved on.'
      ),
      tone: 'emerald',
    },
  ]
})

const selectedRevisionSwitchPreviewCards = computed<SkillReviewCard[]>(() => {
  const revision = selectedRevision.value
  if (!revision) return []

  const currentLiveRevisionID = normalizeText(currentCanonicalRevision.value?.id)

  if (revision.status === 'accepted') {
    return [
      {
        key: 'current',
        label: tr('evolution.skills.switchPreview.current', 'Current live'),
        value: currentLiveRevisionID || tr('common.notAvailable', 'Not available'),
        details: currentLiveRevisionID
          ? tr(
              'evolution.skills.switchPreview.currentHint',
              'This revision currently serves routed built-in use.'
            )
          : tr(
              'evolution.skills.switchPreview.currentMissing',
              'No promoted canonical revision is recorded right now.'
            ),
        tone: currentLiveRevisionID ? 'sky' : 'slate',
      },
      {
        key: 'candidate',
        label: tr('evolution.skills.switchPreview.candidate', 'Selected candidate'),
        value: revision.id,
        details: tr(
          'evolution.skills.switchPreview.candidateHint',
          'This accepted revision would become the new live canonical version if you promote it.'
        ),
        tone: 'emerald',
      },
      {
        key: 'preserve',
        label: tr('evolution.skills.switchPreview.preserve', 'What gets preserved'),
        value: currentLiveRevisionID
          ? tr('evolution.skills.switchPreview.preserveBackup', 'Current live preserved as backup')
          : tr(
              'evolution.skills.switchPreview.preserveFirstLive',
              'Candidate becomes first live version'
            ),
        details: currentLiveRevisionID
          ? `${tr('evolution.skills.switchPreview.preserveHint', 'Current live revision')}: ${currentLiveRevisionID}`
          : tr(
              'evolution.skills.switchPreview.preserveFirstLiveHint',
              'No prior live revision is recorded, so there is no earlier canonical version to preserve.'
            ),
        tone: currentLiveRevisionID ? 'amber' : 'slate',
      },
    ]
  }

  if (revision.status === 'backup') {
    const backupOfRevisionID = normalizeText(revision.backup_of_revision_id)
    return [
      {
        key: 'current',
        label: tr('evolution.skills.switchPreview.current', 'Current live'),
        value: currentLiveRevisionID || tr('common.notAvailable', 'Not available'),
        details: currentLiveRevisionID
          ? tr(
              'evolution.skills.switchPreview.currentRollbackHint',
              'This is the canonical revision that would stop being live after rollback.'
            )
          : tr(
              'evolution.skills.switchPreview.currentMissing',
              'No promoted canonical revision is recorded right now.'
            ),
        tone: currentLiveRevisionID ? 'sky' : 'slate',
      },
      {
        key: 'source',
        label: tr('evolution.skills.switchPreview.rollbackSource', 'Selected backup'),
        value: revision.id,
        details: backupOfRevisionID
          ? `${tr('evolution.skills.lineage.backupOf', 'Backup of revision')}: ${backupOfRevisionID}`
          : tr(
              'evolution.skills.switchPreview.rollbackSourceHint',
              'This backup revision is the source content for the rollback.'
            ),
        tone: 'amber',
      },
      {
        key: 'next',
        label: tr('evolution.skills.switchPreview.nextLive', 'After rollback'),
        value:
          backupOfRevisionID ||
          tr('evolution.skills.switchPreview.restoreBackup', 'Backup content restored'),
        details: tr(
          'evolution.skills.switchPreview.nextRollbackHint',
          'The selected backup content becomes canonical again when the lineage guard still matches.'
        ),
        tone: 'emerald',
      },
      {
        key: 'preserve',
        label: tr('evolution.skills.switchPreview.preserve', 'What gets preserved'),
        value: tr(
          'evolution.skills.switchPreview.currentToBackup',
          'Current live becomes a new backup'
        ),
        details: currentLiveRevisionID
          ? `${tr('evolution.skills.switchPreview.preserveHint', 'Current live revision')}: ${currentLiveRevisionID}`
          : tr(
              'evolution.skills.switchPreview.currentToBackupHint',
              'If rollback succeeds, the current live content is preserved before the restored content becomes canonical.'
            ),
        tone: 'amber',
      },
    ]
  }

  return []
})

const selectedSkillEvidenceReviewCards = computed<SkillReviewCard[]>(() => {
  const skillCase = selectedSkillCase.value
  const evidence = selectedSkillEvidenceRecord.value
  const validation = asRecord(evidence?.validation)
  const groundedness = Number(evidence?.groundedness)
  const confidence = Number(evidence?.confidence)
  const recoveredRaw = validation?.recovered ?? evidence?.recovered
  const failureCount = Number(validation?.failure_count ?? evidence?.failure_count)
  const fallbackCaseStatus: CaseStatusSummary = {
    label: tr('evolution.skills.caseStatus.open', 'Open'),
    tone: 'slate',
  }
  const caseStatus = skillCase ? caseStatusSummary(skillCase) : fallbackCaseStatus

  const whyChanged: SkillReviewCard =
    skillCase?.mode === 'capture'
      ? {
          key: 'why_changed',
          label: tr('evolution.skills.evidenceReview.whyChanged', 'Why changed'),
          value: tr('evolution.skills.evidenceReview.captureIntent', 'Capture learned guidance'),
          details:
            normalizeText(String(evidence?.lesson || '')) ||
            normalizeText(skillCase?.summary) ||
            tr(
              'evolution.skills.evidenceReview.captureIntentHint',
              'A successful runtime lesson was strong enough to fold back into the canonical skill.'
            ),
          tone: 'sky' as const,
        }
      : {
          key: 'why_changed',
          label: tr('evolution.skills.evidenceReview.whyChanged', 'Why changed'),
          value: tr('evolution.skills.evidenceReview.fixIntent', 'Repair missing guidance'),
          details: [
            normalizeText(
              skillCase?.failure_signature ||
                String(evidence?.failure_signature || evidence?.runtime_failure_signature || '')
            )
              ? `${tr('evolution.skills.evidenceReview.trigger', 'Trigger')}: ${normalizeText(
                  skillCase?.failure_signature ||
                    String(evidence?.failure_signature || evidence?.runtime_failure_signature || '')
                )}`
              : '',
            normalizeText(skillCase?.summary),
          ]
            .filter(Boolean)
            .join(' · '),
          tone: typeof recoveredRaw === 'boolean' && recoveredRaw ? 'amber' : ('rose' as const),
        }

  const evidenceStrength: SkillReviewCard =
    skillCase?.mode === 'capture'
      ? {
          key: 'strength',
          label: tr('evolution.skills.evidenceReview.strength', 'Evidence strength'),
          value:
            Number.isFinite(groundedness) && groundedness >= 0.75
              ? tr('evolution.skills.evidenceReview.stronglyGrounded', 'Strongly grounded')
              : tr('evolution.skills.evidenceReview.partiallyGrounded', 'Partially grounded'),
          details: [
            Number.isFinite(groundedness)
              ? `${tr('evolution.skills.evidenceSummary.groundedness', 'Groundedness')} ${formatPercentValue(
                  groundedness
                )}`
              : '',
            Number.isFinite(confidence)
              ? `${tr('evolution.skills.evidenceSummary.confidence', 'Confidence')} ${formatPercentValue(
                  confidence
                )}`
              : '',
            decodeStringList(evidence?.evidence_ids).length > 0
              ? `${tr('evolution.skills.evidenceSummary.evidenceIDs', 'Evidence IDs')} ${formatCount(
                  decodeStringList(evidence?.evidence_ids).length
                )}`
              : '',
          ]
            .filter(Boolean)
            .join(' · '),
          tone:
            Number.isFinite(groundedness) &&
            groundedness >= 0.75 &&
            Number.isFinite(confidence) &&
            confidence >= 0.75
              ? 'emerald'
              : 'amber',
        }
      : {
          key: 'strength',
          label: tr('evolution.skills.evidenceReview.strength', 'Evidence strength'),
          value:
            typeof recoveredRaw === 'boolean' && recoveredRaw
              ? tr('evolution.skills.evidenceReview.recoveredFailure', 'Runtime failure recovered')
              : tr(
                  'evolution.skills.evidenceReview.runtimeFailureObserved',
                  'Runtime failure observed'
                ),
          details: [
            Number.isFinite(failureCount)
              ? `${tr('evolution.skills.evidenceSummary.failureCount', 'Failure count')} ${formatCount(
                  failureCount
                )}`
              : '',
            normalizeText(skillCase?.source_id)
              ? `${tr('evolution.skills.caseMeta.sourceId', 'Source ID')} ${normalizeText(skillCase?.source_id)}`
              : '',
          ]
            .filter(Boolean)
            .join(' · '),
          tone: typeof recoveredRaw === 'boolean' ? (recoveredRaw ? 'emerald' : 'rose') : 'amber',
        }

  const adoptionPath: SkillReviewCard = {
    key: 'adoption',
    label: tr('evolution.skills.evidenceReview.adoptionPath', 'Adoption path'),
    value: caseStatus.label,
    details: selectedRevision.value?.status
      ? revisionStatusSummary(selectedRevision.value.status)
      : tr(
          'evolution.skills.evidenceReview.adoptionPathHint',
          'Review gate outcome and revision status before deciding whether to promote.'
        ),
    tone: caseStatus.tone,
  }

  return [whyChanged, evidenceStrength, adoptionPath]
})

const selectedSkillDiffReviewCards = computed<SkillReviewCard[]>(() => {
  const summary = selectedSkillPatchSummary.value
  const guidanceAreas = selectedSkillGuidanceAreas.value
  const totalChanges = summary.additions + summary.deletions

  return [
    {
      key: 'areas',
      label: tr('evolution.skills.diffReview.areas', 'Affected guidance'),
      value:
        guidanceAreas.length > 0
          ? guidanceAreas.slice(0, 2).join(' · ')
          : tr('evolution.skills.diffReview.generalGuidance', 'General guidance'),
      details:
        guidanceAreas.length > 2
          ? trp(
              'evolution.skills.diffReview.areasMore',
              '{shown} shown, {total} affected areas inferred from the patch.',
              {
                shown: guidanceAreas.slice(0, 2).length,
                total: guidanceAreas.length,
              }
            )
          : tr(
              'evolution.skills.diffReview.areasHint',
              'These areas were inferred from the changed lines and nearby markdown structure.'
            ),
      tone: guidanceAreas.length > 0 ? 'sky' : 'slate',
    },
    {
      key: 'risk',
      label: tr('evolution.skills.diffReview.risk', 'Patch risk'),
      value: patchRiskLabel(summary),
      details: [
        `${tr('evolution.patchSummary.sections', 'Change sections')} ${formatCount(summary.sections)}`,
        `+${formatCount(summary.additions)}`,
        `-${formatCount(summary.deletions)}`,
      ].join(' · '),
      tone: patchRiskTone(summary),
    },
    {
      key: 'scope',
      label: tr('evolution.skills.diffReview.scope', 'Patch scope'),
      value: patchScopeLabel(summary),
      details: [
        totalChanges > 0
          ? trp(
              'evolution.skills.diffReview.scopeDetails',
              '{count} changed lines across {sections} sections.',
              {
                count: formatCount(totalChanges),
                sections: formatCount(summary.sections),
              }
            )
          : tr(
              'evolution.skills.diffReview.scopeEmptyDetails',
              'No candidate patch is attached to this revision yet.'
            ),
        selectedSkillCase.value?.mode === 'capture'
          ? tr(
              'evolution.skills.diffReview.scopeCaptureHint',
              'Capture revisions often expand examples or when-to-apply guidance.'
            )
          : tr(
              'evolution.skills.diffReview.scopeFixHint',
              'Fix revisions often tighten recovery guidance and failure handling.'
            ),
      ]
        .filter(Boolean)
        .join(' '),
      tone:
        totalChanges === 0
          ? 'slate'
          : totalChanges <= 4
            ? 'emerald'
            : totalChanges <= 12
              ? 'amber'
              : 'rose',
    },
  ]
})

const selectedRevisionDecisionCards = computed<DecisionSummaryCard[]>(() => {
  const revision = selectedRevision.value
  if (!revision) return []

  const currentCanonical = currentCanonicalRevision.value
  const comparisonSnapshot = selectedSkillComparisonSnapshot.value
  const metricSnapshot = selectedSkillMetricSnapshot.value
  const evidenceSnapshot = comparisonSnapshot || metricSnapshot

  let selectedValue = humanizeEnum(revision.status)
  let selectedTone: DecisionSummaryCard['tone'] = 'slate'
  switch (revision.status) {
    case 'accepted':
      selectedValue = tr('evolution.skills.decision.betterCandidate', 'Better candidate')
      selectedTone = 'emerald'
      break
    case 'promoted':
      selectedValue = tr('evolution.skills.decision.liveCanonical', 'Live canonical')
      selectedTone = currentCanonical?.id === revision.id ? 'sky' : 'slate'
      break
    case 'backup':
      selectedValue = tr('evolution.skills.decision.rollbackBackup', 'Rollback backup')
      selectedTone = 'amber'
      break
    case 'candidate':
      selectedValue = tr('evolution.skills.decision.pendingGate', 'Pending gate')
      selectedTone = 'amber'
      break
    case 'rejected':
      selectedValue = tr('evolution.skills.decision.rejectedCandidate', 'Rejected candidate')
      selectedTone = 'rose'
      break
  }

  let nextStepValue = tr('evolution.skills.decision.reviewEvidence', 'Review evidence')
  let nextStepDetails = revisionStatusSummary(revision.status)
  let nextStepTone: DecisionSummaryCard['tone'] = 'slate'

  switch (revision.status) {
    case 'accepted':
      nextStepValue = tr('evolution.skills.decision.promoteNext', 'Promote to canonical')
      nextStepDetails = [
        tr(
          'evolution.skills.decision.promoteNextHint',
          'This revision passed the gate and is ready to replace the live canonical version when you promote it.'
        ),
        evidenceSnapshot,
      ]
        .filter(Boolean)
        .join(' ')
      nextStepTone = 'emerald'
      break
    case 'backup':
      nextStepValue = tr('evolution.skills.decision.rollbackNext', 'Rollback eligible')
      nextStepDetails = [
        tr(
          'evolution.skills.decision.rollbackNextHint',
          'This backup can be restored when the current canonical lineage still matches, giving you a safe one-step rollback.'
        ),
        normalizeText(revision.backup_of_revision_id)
          ? `${tr('evolution.skills.lineage.backupOf', 'Backup of revision')}: ${normalizeText(revision.backup_of_revision_id)}`
          : '',
      ]
        .filter(Boolean)
        .join(' ')
      nextStepTone = 'amber'
      break
    case 'promoted':
      nextStepValue = tr('evolution.skills.decision.liveNext', 'Already live')
      nextStepDetails = tr(
        'evolution.skills.decision.liveNextHint',
        'No switch is needed. Use backup revisions if you need a safe rollback path later.'
      )
      nextStepTone = 'sky'
      break
    case 'candidate':
      nextStepValue = tr('evolution.skills.decision.waitGate', 'Wait for gate')
      nextStepDetails = tr(
        'evolution.skills.decision.waitGateHint',
        'This candidate still needs follow-up evaluation and gate review before it can be considered better.'
      )
      nextStepTone = 'amber'
      break
    case 'rejected':
      nextStepValue = tr('evolution.skills.decision.keepLive', 'Keep current live')
      nextStepDetails = tr(
        'evolution.skills.decision.keepLiveHint',
        'This revision was rejected by the gate, so the current canonical version should remain in place.'
      )
      nextStepTone = 'rose'
      break
  }

  return [
    {
      key: 'live',
      label: tr('evolution.skills.decision.liveCard', 'Live now'),
      value: currentCanonical?.id || tr('common.notAvailable', 'Not available'),
      details:
        currentCanonical?.id === revision.id
          ? tr(
              'evolution.skills.decision.liveCurrentHint',
              'The selected revision is already the live canonical version for this skill.'
            )
          : currentCanonical
            ? tr(
                'evolution.skills.decision.liveOtherHint',
                'This is the canonical version currently serving routed built-in use.'
              )
            : tr(
                'evolution.skills.decision.liveMissingHint',
                'No promoted canonical revision is recorded for this skill yet.'
              ),
      tone: currentCanonical?.id === revision.id ? 'sky' : 'slate',
      actionLabel: tr('evolution.skills.decision.viewLineage', 'View lineage'),
      action: () => focusSkillDetailSection('lineage'),
    },
    {
      key: 'selected',
      label: tr('evolution.skills.decision.selectedCard', 'Selected revision'),
      value: selectedValue,
      details: [revisionStatusSummary(revision.status), evidenceSnapshot].filter(Boolean).join(' '),
      tone: selectedTone,
      actionLabel: tr('evolution.skills.decision.viewDiff', 'View diff patch'),
      action: () => focusSkillDetailSection('diff'),
    },
    {
      key: 'next_step',
      label: tr('evolution.skills.decision.nextStepCard', 'Next step'),
      value: nextStepValue,
      details: nextStepDetails,
      tone: nextStepTone,
      actionLabel:
        revision.status === 'accepted'
          ? selectedSkillComparison.value.baselineEvalRunID ||
            selectedSkillComparison.value.deltas.length > 0
            ? tr('evolution.skills.decision.viewComparison', 'View comparison')
            : selectedSkillMetrics.value.length > 0
              ? tr('evolution.skills.decision.viewMetrics', 'View metrics')
              : tr('evolution.skills.decision.viewEvidence', 'View evidence')
          : revision.status === 'promoted'
            ? tr('evolution.skills.decision.viewMetrics', 'View metrics')
            : tr('evolution.skills.decision.viewEvidence', 'View evidence'),
      action: () =>
        revision.status === 'accepted'
          ? selectedSkillComparison.value.baselineEvalRunID ||
            selectedSkillComparison.value.deltas.length > 0
            ? focusSkillDetailSection('comparison')
            : selectedSkillMetrics.value.length > 0
              ? focusSkillDetailSection('metrics')
              : focusSkillDetailSection('evidence')
          : revision.status === 'promoted'
            ? focusSkillDetailSection('metrics')
            : focusSkillDetailSection('evidence'),
    },
  ]
})

const skillScorecard = computed<SkillScorecardCard[]>(() => {
  const report = selectedRevisionReport.value
  const overallScore = reportSummaryValue(
    report,
    'overall_score',
    report?.group_report?.overall_score
  )
  const passRate = reportSummaryValue(report, 'pass_rate', report?.group_report?.pass_rate)
  const overallScoreDelta = reportSummaryValue(report, 'overall_score_delta')
  const passRateDelta = reportSummaryValue(report, 'pass_rate_delta')
  const verificationPassRate = reportSummaryValue(report, 'verification_pass_rate')
  const verificationPassRateDelta = reportSummaryValue(report, 'verification_pass_rate_delta')
  const evidenceBackedPassRate = reportSummaryValue(report, 'evidence_backed_pass_rate')
  const evidenceBackedPassRateDelta = reportSummaryValue(report, 'evidence_backed_pass_rate_delta')
  const runtimeValue =
    reportSummaryValue(report, 'avg_duration_ms') ?? reportSummaryValue(report, 'duration_ms')
  const runtimeLabel =
    reportSummaryValue(report, 'avg_duration_ms') != null
      ? tr('evolution.metrics.avgDuration', 'Average runtime')
      : tr('evolution.metrics.runtime', 'Runtime')
  const totalTokens = reportSummaryValue(report, 'total_tokens')

  const evidenceTone =
    selectedSkillCase.value?.mode === 'capture'
      ? scorecardToneForPercent(selectedSkillEvidenceRecord.value?.groundedness)
      : typeof selectedSkillEvidenceRecord.value?.validation === 'object' &&
          asRecord(selectedSkillEvidenceRecord.value?.validation)?.recovered === false
        ? 'rose'
        : selectedSkillEvidenceSummary.value.some(
              (entry) => entry.key === 'recovered' && entry.value === tr('common.yes', 'Yes')
            )
          ? 'emerald'
          : selectedSkillCase.value
            ? 'amber'
            : 'slate'

  return [
    {
      key: 'quality',
      label: tr('evolution.skills.scorecard.quality', 'Quality'),
      value:
        overallScore != null
          ? formatMetricValue('overall_score', overallScore)
          : tr('common.notAvailable', 'Not available'),
      details: [
        passRate != null
          ? `${metricLabel('pass_rate')} ${formatMetricValue('pass_rate', passRate)}`
          : '',
        overallScoreDelta != null
          ? `${comparisonMetricLabel('overall_score_delta')} ${formatSignedMetricValue('overall_score_delta', overallScoreDelta)}`
          : '',
        passRateDelta != null
          ? `${comparisonMetricLabel('pass_rate_delta')} ${formatSignedMetricValue('pass_rate_delta', passRateDelta)}`
          : '',
      ]
        .filter(Boolean)
        .join(' · '),
      tone: scorecardToneForPercent(overallScore),
      actionLabel:
        overallScoreDelta != null || passRateDelta != null
          ? tr('evolution.skills.scorecardOpenComparison', 'Open comparison')
          : tr('evolution.skills.scorecardOpenMetrics', 'Open metrics'),
      action:
        overallScoreDelta != null || passRateDelta != null
          ? () => focusSkillDetailSection('comparison')
          : () => focusSkillDetailSection('metrics'),
    },
    {
      key: 'verification',
      label: tr('evolution.skills.scorecard.verification', 'Verification'),
      value:
        verificationPassRate != null
          ? formatMetricValue('verification_pass_rate', verificationPassRate)
          : verificationPassRateDelta != null
            ? formatSignedMetricValue('verification_pass_rate_delta', verificationPassRateDelta)
            : tr('common.notAvailable', 'Not available'),
      details: [
        evidenceBackedPassRate != null
          ? `${metricLabel('evidence_backed_pass_rate')} ${formatMetricValue(
              'evidence_backed_pass_rate',
              evidenceBackedPassRate
            )}`
          : '',
        verificationPassRateDelta != null
          ? `${comparisonMetricLabel('verification_pass_rate_delta')} ${formatSignedMetricValue(
              'verification_pass_rate_delta',
              verificationPassRateDelta
            )}`
          : '',
        evidenceBackedPassRateDelta != null
          ? `${comparisonMetricLabel('evidence_backed_pass_rate_delta')} ${formatSignedMetricValue(
              'evidence_backed_pass_rate_delta',
              evidenceBackedPassRateDelta
            )}`
          : '',
      ]
        .filter(Boolean)
        .join(' · '),
      tone: scorecardToneForPercent(
        verificationPassRate ??
          evidenceBackedPassRate ??
          verificationPassRateDelta ??
          evidenceBackedPassRateDelta
      ),
      actionLabel:
        verificationPassRateDelta != null || evidenceBackedPassRateDelta != null
          ? tr('evolution.skills.scorecardOpenComparison', 'Open comparison')
          : tr('evolution.skills.scorecardOpenMetrics', 'Open metrics'),
      action:
        verificationPassRateDelta != null || evidenceBackedPassRateDelta != null
          ? () => focusSkillDetailSection('comparison')
          : () => focusSkillDetailSection('metrics'),
    },
    {
      key: 'runtime',
      label: tr('evolution.skills.scorecard.runtime', 'Runtime'),
      value:
        runtimeValue != null
          ? formatMetricValue(
              reportSummaryValue(report, 'avg_duration_ms') != null
                ? 'avg_duration_ms'
                : 'duration_ms',
              runtimeValue
            )
          : tr('common.notAvailable', 'Not available'),
      details: [
        runtimeValue != null ? runtimeLabel : '',
        totalTokens != null
          ? `${metricLabel('total_tokens')} ${formatMetricValue('total_tokens', totalTokens)}`
          : '',
      ]
        .filter(Boolean)
        .join(' · '),
      tone: scorecardToneForDuration(runtimeValue),
      actionLabel: tr('evolution.skills.scorecardOpenMetrics', 'Open metrics'),
      action: () => focusSkillDetailSection('metrics'),
    },
    {
      key: 'tokens',
      label: tr('evolution.skills.scorecard.tokens', 'Token cost'),
      value:
        totalTokens != null
          ? formatMetricValue('total_tokens', totalTokens)
          : tr('common.notAvailable', 'Not available'),
      details: [
        runtimeValue != null
          ? `${runtimeLabel} ${formatMetricValue(
              reportSummaryValue(report, 'avg_duration_ms') != null
                ? 'avg_duration_ms'
                : 'duration_ms',
              runtimeValue
            )}`
          : '',
        selectedSkillMetricSnapshot.value,
      ]
        .filter(Boolean)
        .join(' · '),
      tone: scorecardToneForTokens(totalTokens),
      actionLabel: tr('evolution.skills.scorecardOpenMetrics', 'Open metrics'),
      action: () => focusSkillDetailSection('metrics'),
    },
    {
      key: 'evidence',
      label: tr('evolution.skills.scorecard.evidence', 'Grounded evidence'),
      value: selectedSkillCase.value
        ? `${humanizeEnum(selectedSkillCase.value.mode)} · ${humanizeEnum(selectedSkillCase.value.status)}`
        : tr('common.notAvailable', 'Not available'),
      details:
        selectedSkillEvidenceSnapshot.value ||
        tr(
          'evolution.skills.scorecardEvidenceHint',
          'No structured evidence summary is attached to this revision yet.'
        ),
      tone: evidenceTone,
      actionLabel: tr('evolution.skills.scorecardOpenEvidence', 'Open evidence'),
      action: () => focusSkillDetailSection('evidence'),
    },
  ]
})

const selectedCaseMeta = computed(() => {
  const skillCase = selectedSkillCase.value
  if (!skillCase) return [] as Array<{ label: string; value: string }>
  return [
    {
      label: tr('evolution.skills.caseMeta.sourceKind', 'Source kind'),
      value: humanizeOptionalEnum(skillCase.source_kind),
    },
    {
      label: tr('evolution.skills.caseMeta.sourceId', 'Source ID'),
      value: normalizeText(skillCase.source_id),
    },
    {
      label: tr('evolution.skills.caseMeta.candidateId', 'Candidate ID'),
      value: normalizeText(skillCase.candidate_id),
    },
    {
      label: tr('evolution.skills.caseMeta.revisionId', 'Revision ID'),
      value: normalizeText(
        selectedSkillCaseDetail.value?.linked_revision?.id || skillCase.revision_id
      ),
    },
    {
      label: tr('evolution.skills.caseMeta.sourceEvalRunId', 'Source eval run'),
      value: selectedSkillCaseSourceEvalRunID.value,
    },
    {
      label: tr('evolution.skills.caseMeta.linkedEvalRunId', 'Linked eval run'),
      value: selectedSkillCaseLinkedEvalRunID.value,
    },
    {
      label: tr('evolution.skills.caseMeta.skippedReason', 'Skipped reason'),
      value: normalizeText(
        selectedSkillCaseDetail.value?.skipped_reason || skillCase.skipped_reason
      ),
    },
  ].filter((entry) => entry.value)
})

const selectedSkillCaseSourceEvalRunID = computed(() => {
  if (selectedSkillCaseDetail.value?.source_eval_run_id) {
    return normalizeText(selectedSkillCaseDetail.value.source_eval_run_id)
  }
  if (normalizeText(selectedSkillCase.value?.source_kind) === 'eval_run') {
    return normalizeText(selectedSkillCase.value?.source_id)
  }
  return ''
})

const selectedSkillCaseLinkedEvalRunID = computed(() =>
  normalizeText(
    selectedSkillCaseDetail.value?.linked_eval_run_id ||
      selectedSkillCaseDetail.value?.linked_revision?.eval_run_id ||
      selectedCaseRevision.value?.eval_run_id
  )
)

const pendingInstructionCount = computed(() =>
  instructionsLoaded.value
    ? instructionProposals.value.filter((proposal) => proposal.status === 'pending').length
    : (evolutionOverview.value?.instructions.pending ?? 0)
)

const filteredInstructionProposals = computed(() =>
  instructionProposals.value.filter((proposal) => {
    const matchesStatus =
      instructionStatusFilter.value === 'all' ||
      normalizeText(proposal.status) === instructionStatusFilter.value
    const matchesQuery = matchesSearchQuery(
      instructionSearch.value,
      proposal.id,
      proposal.target_file,
      proposal.lesson,
      proposal.when_to_apply,
      proposal.evidence,
      proposal.source_id,
      humanizeEnum(proposal.status)
    )
    return matchesStatus && matchesQuery
  })
)

const selectedProposalHiddenByFilters = computed(
  () =>
    Boolean(selectedProposal.value) &&
    !filteredInstructionProposals.value.some(
      (proposal) => proposal.id === selectedProposal.value?.id
    )
)

const selectedProposalMeta = computed(() => {
  const proposal = selectedProposal.value
  if (!proposal) return [] as Array<{ label: string; value: string }>
  return [
    {
      label: tr('evolution.instructions.meta.sourceKind', 'Source kind'),
      value: humanizeOptionalEnum(proposal.source_kind),
    },
    {
      label: tr('evolution.instructions.meta.sourceId', 'Source ID'),
      value: normalizeText(proposal.source_id),
    },
    {
      label: tr('evolution.instructions.meta.evidenceIds', 'Evidence IDs'),
      value: (proposal.evidence_ids || []).filter(Boolean).join(', '),
    },
    {
      label: tr('evolution.instructions.meta.reviewNote', 'Review note'),
      value: normalizeText(proposal.review_note),
    },
  ].filter((entry) => entry.value)
})

const selectedInstructionPatchSummary = computed(() =>
  patchSummaryEntries(summarizePatch(selectedInstructionPatch.value))
)

function updateKnowledgeLaneSummary(summary: KnowledgeLaneSummary) {
  knowledgeLaneSummary.value = summary
  knowledgeLaneSummaryLoading.value = false
}

async function loadKnowledgeLaneSummary() {
  knowledgeLaneSummaryLoading.value = true
  try {
    const [pagesResult, lintResult] = await Promise.allSettled([
      knowledgeApi.listPages(),
      knowledgeApi.getLatestLint(),
    ])

    if (pagesResult.status !== 'fulfilled') throw pagesResult.reason

    const pages = pagesResult.value.data || []
    const issues = lintResult.status === 'fulfilled' ? lintResult.value.data?.issues || [] : []
    const conflicts =
      pages.filter((page) => page.status === 'conflicted').length ||
      issues.filter((issue) => issue.category === 'review_required').length ||
      0
    const gaps = issues.filter((issue) => issue.category === 'research_suggestions').length || 0

    knowledgeLaneSummary.value = {
      visiblePages: pages.length,
      totalPages: pages.length,
      conflicts,
      gaps,
    }
  } catch {
    // Keep the lane best-effort. The detailed Knowledge pane still owns full error handling.
  } finally {
    knowledgeLaneSummaryLoading.value = false
  }
}

const evolutionLaneCards = computed<EvolutionLaneCard[]>(() => [
  {
    pane: 'knowledge',
    title: tr('evolution.tabs.knowledge', 'Knowledge'),
    description: tr(
      'evolution.lanes.knowledgeDescription',
      'Ground the workspace in compiled pages, source-backed queries, and conflict or gap signals.'
    ),
    metric:
      knowledgeLaneSummaryLoading.value && knowledgeLaneSummary.value == null
        ? '...'
        : formatCount(knowledgeLaneSummary.value?.visiblePages || 0),
    supporting:
      knowledgeLaneSummaryLoading.value && knowledgeLaneSummary.value == null
        ? tr('common.loading', 'Loading')
        : knowledgeLaneSummary.value == null
          ? tr('knowledge.eyebrow', 'Knowledge Space')
          : (knowledgeLaneSummary.value.conflicts || 0) > 0
            ? trp('knowledge.conflictCount', '{count} unresolved conflicts', {
                count: knowledgeLaneSummary.value.conflicts,
              })
            : (knowledgeLaneSummary.value.gaps || 0) > 0
              ? trp('knowledge.gapCount', '{count} open gaps', {
                  count: knowledgeLaneSummary.value.gaps,
                })
              : tr('knowledge.pageList', 'Pages'),
  },
  {
    pane: 'skills',
    title: tr('evolution.tabs.skills', 'Skills'),
    description: tr(
      'evolution.lanes.skillsDescription',
      'Review canonical skills, accepted revisions, and safe rollback readiness.'
    ),
    metric: `${formatCount(filteredAvailableSkills.value.length)}/${formatCount(
      availableSkills.value.length
    )}`,
    supporting: tr('evolution.summary.visibleSkills', 'Visible skills'),
  },
  {
    pane: 'runner',
    title: tr('evolution.tabs.runner', 'Runner'),
    description: tr(
      'evolution.lanes.runnerDescription',
      'Inspect runner candidates, execution evidence, and switch boundaries before cutover.'
    ),
    metric: formatCount(selectedSkillAcceptedRevisionCount.value),
    supporting: tr('evolution.health.skillReadiness', 'Skill readiness'),
  },
  {
    pane: 'instructions',
    title: tr('evolution.tabs.instructions', 'Review Queue'),
    description: tr(
      'evolution.lanes.instructionsDescription',
      'Review instruction proposals and keep AGENTS.md approvals explicit.'
    ),
    metric: formatCount(pendingInstructionCount.value),
    supporting: tr('evolution.health.reviewQueue', 'Review queue'),
  },
])

function applyRouteFiltersFromQuery() {
  activePane.value = parseEvolutionPaneQuery(route.query.pane)
  skillSearch.value = routeQueryValue(route.query.skillSearch)
  revisionSearch.value = routeQueryValue(route.query.revisionSearch)
  revisionStatusFilter.value = parseSkillRevisionStatusFilterQuery(route.query.revisionStatus)
  caseSearch.value = routeQueryValue(route.query.caseSearch)
  caseStatusFilter.value = parseSkillCaseStatusFilterQuery(route.query.caseStatus)
  caseModeFilter.value = parseSkillCaseModeFilterQuery(route.query.caseMode)
  instructionSearch.value = routeQueryValue(route.query.instructionSearch)
  instructionStatusFilter.value = parseProposalStatusFilterQuery(route.query.instructionStatus)
  routedRevisionID.value = routeQueryValue(route.query.revision)
  routedProposalID.value = routeQueryValue(route.query.proposal)
}

function buildEvolutionRouteQuery(): Record<string, string> {
  const query: Record<string, string> = {}
  if (activePane.value !== 'knowledge') query.pane = activePane.value
  if (selectedSkillID.value) query.skill = selectedSkillID.value
  if (selectedRevisionID.value) query.revision = selectedRevisionID.value
  if (skillSearch.value.trim()) query.skillSearch = skillSearch.value.trim()
  if (revisionSearch.value.trim()) query.revisionSearch = revisionSearch.value.trim()
  if (revisionStatusFilter.value !== 'all') query.revisionStatus = revisionStatusFilter.value
  if (caseSearch.value.trim()) query.caseSearch = caseSearch.value.trim()
  if (caseStatusFilter.value !== 'all') query.caseStatus = caseStatusFilter.value
  if (caseModeFilter.value !== 'all') query.caseMode = caseModeFilter.value
  if (selectedProposalID.value) query.proposal = selectedProposalID.value
  if (instructionSearch.value.trim()) query.instructionSearch = instructionSearch.value.trim()
  if (instructionStatusFilter.value !== 'all') {
    query.instructionStatus = instructionStatusFilter.value
  }
  return query
}

function normalizeEvolutionManagedQuery(
  query: Partial<Record<(typeof EVOLUTION_ROUTE_QUERY_KEYS)[number], unknown>>
): Record<(typeof EVOLUTION_ROUTE_QUERY_KEYS)[number], string> {
  return {
    pane: routeQueryValue(query.pane),
    skill: routeQueryValue(query.skill),
    revision: routeQueryValue(query.revision),
    skillSearch: routeQueryValue(query.skillSearch),
    revisionSearch: routeQueryValue(query.revisionSearch),
    revisionStatus: routeQueryValue(query.revisionStatus),
    caseSearch: routeQueryValue(query.caseSearch),
    caseStatus: routeQueryValue(query.caseStatus),
    caseMode: routeQueryValue(query.caseMode),
    proposal: routeQueryValue(query.proposal),
    instructionSearch: routeQueryValue(query.instructionSearch),
    instructionStatus: routeQueryValue(query.instructionStatus),
  }
}

function applyRequestedSkillSelection() {
  const requestedSkillID = routeQueryValue(route.query.skill)
  if (requestedSkillID && availableSkills.value.some((skill) => skill.id === requestedSkillID)) {
    if (selectedSkillID.value !== requestedSkillID) {
      selectedSkillID.value = requestedSkillID
    }
    return
  }
  if (!selectedSkillID.value && availableSkills.value.length > 0) {
    selectedSkillID.value = availableSkills.value[0]!.id
  }
}

async function applyRequestedInstructionSelection() {
  const requestedProposalID = routedProposalID.value
  if (
    requestedProposalID &&
    instructionProposals.value.some((proposal) => proposal.id === requestedProposalID)
  ) {
    if (selectedProposalID.value !== requestedProposalID) {
      await selectProposal(requestedProposalID)
    }
    return
  }

  if (!selectedProposalID.value && instructionProposals.value.length > 0) {
    selectedProposalID.value = instructionProposals.value[0]!.id
    await loadProposalDetail(selectedProposalID.value)
  }
}

async function syncRouteQuery() {
  const nextManagedQuery = normalizeEvolutionManagedQuery(buildEvolutionRouteQuery())
  const currentManagedQuery = normalizeEvolutionManagedQuery(route.query)
  if (JSON.stringify(nextManagedQuery) === JSON.stringify(currentManagedQuery)) return

  const nextQuery: LocationQueryRaw = { ...route.query }
  for (const key of EVOLUTION_ROUTE_QUERY_KEYS) {
    delete nextQuery[key]
  }
  for (const [key, value] of Object.entries(buildEvolutionRouteQuery())) {
    nextQuery[key] = value
  }

  await router.replace({ query: nextQuery })
}

async function ensureRunnerDataLoaded() {
  try {
    if (!agentcoreRunnerStatus.value) {
      await settingsStore.fetchAgentcoreRunnerStatus()
      return
    }
    if (
      normalizeText(agentcoreRunnerStatus.value.last_optimization_run_id) &&
      !agentcoreRunnerLastRun.value
    ) {
      await settingsStore.fetchAgentcoreRunnerLastRun()
    }
  } catch {
    // The embedded runner panel still exposes the underlying status surface, so keep
    // the dedicated runner lane best-effort instead of failing the whole page.
  }
}

watch(
  activePane,
  async (pane) => {
    if (pane === 'instructions' && !instructionsLoaded.value) {
      await loadInstructionProposals()
      return
    }
    if (pane === 'skills' && selectedSkillID.value && loadedSkillDetailsID.value !== selectedSkillID.value) {
      await loadSkillDetails(selectedSkillID.value)
      return
    }
    if (pane === 'runner') {
      await ensureRunnerDataLoaded()
    }
  },
  { immediate: false }
)

watch(
  () => route.query,
  async () => {
    applyRouteFiltersFromQuery()
    applyRequestedSkillSelection()
    if (
      routedRevisionID.value &&
      selectedSkillRevisions.value.some((revision) => revision.id === routedRevisionID.value)
    ) {
      selectedRevisionID.value = routedRevisionID.value
    }
    if (instructionsLoaded.value) {
      await applyRequestedInstructionSelection()
    }
  },
  { deep: true }
)

watch(
  [
    activePane,
    selectedSkillID,
    selectedRevisionID,
    skillSearch,
    revisionSearch,
    revisionStatusFilter,
    caseSearch,
    caseStatusFilter,
    caseModeFilter,
    selectedProposalID,
    instructionSearch,
    instructionStatusFilter,
  ],
  () => {
    void syncRouteQuery()
  }
)

watch(
  selectedSkillID,
  async (skillID) => {
    if (!skillID) return
    await loadEvolutionOverview(skillID)
    if (activePane.value === 'skills') {
      await loadSkillDetails(skillID)
    }
  },
  { immediate: false }
)

watch(
  () => selectedRevision.value?.eval_run_id,
  async (evalRunID) => {
    selectedRevisionReport.value = null
    revisionReportError.value = ''
    if (!evalRunID) return
    try {
      revisionReportLoading.value = true
      const response = await harnessApi.getEvalRunReport(evalRunID)
      selectedRevisionReport.value = response.data
    } catch (error) {
      revisionReportError.value = getErrorMessage(error)
    } finally {
      revisionReportLoading.value = false
    }
  },
  { immediate: true }
)

watch(
  () => selectedSkillCase.value?.id,
  async (caseID) => {
    const normalizedID = normalizeText(caseID)
    if (!normalizedID) return
    if (selectedSkillCaseDetailsByID.value[normalizedID] !== undefined) return
    if (selectedSkillCaseDetailLoadingByID.value[normalizedID]) return
    try {
      selectedSkillCaseDetailLoadingByID.value = {
        ...selectedSkillCaseDetailLoadingByID.value,
        [normalizedID]: true,
      }
      const response = await harnessApi.getSkillEvolutionCase(normalizedID)
      selectedSkillCaseDetailsByID.value = {
        ...selectedSkillCaseDetailsByID.value,
        [normalizedID]: response.data,
      }
    } catch {
      selectedSkillCaseDetailsByID.value = {
        ...selectedSkillCaseDetailsByID.value,
        [normalizedID]: null,
      }
    } finally {
      selectedSkillCaseDetailLoadingByID.value = {
        ...selectedSkillCaseDetailLoadingByID.value,
        [normalizedID]: false,
      }
    }
  },
  { immediate: true }
)

async function loadDecisionHistoryEvalReport(evalRunID: string) {
  const normalizedID = normalizeText(evalRunID)
  if (!normalizedID) return
  if (decisionHistoryReportsByEvalRunID.value[normalizedID]) return
  if (decisionHistoryReportLoadingByEvalRunID.value[normalizedID]) return

  try {
    decisionHistoryReportLoadingByEvalRunID.value = {
      ...decisionHistoryReportLoadingByEvalRunID.value,
      [normalizedID]: true,
    }
    const response = await harnessApi.getEvalRunReport(normalizedID)
    decisionHistoryReportsByEvalRunID.value = {
      ...decisionHistoryReportsByEvalRunID.value,
      [normalizedID]: response.data,
    }
  } catch {
    decisionHistoryReportsByEvalRunID.value = {
      ...decisionHistoryReportsByEvalRunID.value,
      [normalizedID]: null,
    }
  } finally {
    decisionHistoryReportLoadingByEvalRunID.value = {
      ...decisionHistoryReportLoadingByEvalRunID.value,
      [normalizedID]: false,
    }
  }
}

async function loadRunnerEvalReport(evalRunID: string) {
  const normalizedID = normalizeText(evalRunID)
  if (!normalizedID) return
  if (runnerReportsByEvalRunID.value[normalizedID] !== undefined) return
  if (runnerReportLoadingByEvalRunID.value[normalizedID]) return

  try {
    runnerReportLoadingByEvalRunID.value = {
      ...runnerReportLoadingByEvalRunID.value,
      [normalizedID]: true,
    }
    const response = await harnessApi.getEvalRunReport(normalizedID)
    runnerReportsByEvalRunID.value = {
      ...runnerReportsByEvalRunID.value,
      [normalizedID]: response.data,
    }
  } catch {
    runnerReportsByEvalRunID.value = {
      ...runnerReportsByEvalRunID.value,
      [normalizedID]: null,
    }
  } finally {
    runnerReportLoadingByEvalRunID.value = {
      ...runnerReportLoadingByEvalRunID.value,
      [normalizedID]: false,
    }
  }
}

watch(
  () =>
    selectedSkillDecisionHistorySource.value
      .map((entry) => skillDecisionHistoryEvalRunID(entry))
      .filter(Boolean)
      .sort()
      .join('|'),
  async () => {
    const evalRunIDs = Array.from(
      new Set(
        selectedSkillDecisionHistorySource.value
          .map((entry) => skillDecisionHistoryEvalRunID(entry))
          .filter(Boolean)
      )
    )
    await Promise.all(evalRunIDs.map((evalRunID) => loadDecisionHistoryEvalReport(evalRunID)))
  },
  { immediate: true }
)

watch(
  () => [runnerSourceEvalRunID.value, runnerFollowupEvalRunID.value].filter(Boolean).join('|'),
  async () => {
    const evalRunIDs = Array.from(
      new Set([runnerSourceEvalRunID.value, runnerFollowupEvalRunID.value].filter(Boolean))
    )
    await Promise.all(evalRunIDs.map((evalRunID) => loadRunnerEvalReport(evalRunID)))
  },
  { immediate: true }
)

watch(
  () => selectedSkillRevisions.value.map((revision) => revision.id).join('|'),
  () => {
    const pair = decisionHistoryComparisonPair.value
    if (!pair) return
    const revisionIDs = new Set(selectedSkillRevisions.value.map((revision) => revision.id))
    if (!revisionIDs.has(pair.leftRevisionID) || !revisionIDs.has(pair.rightRevisionID)) {
      decisionHistoryComparisonPair.value = null
    }
  }
)

onMounted(async () => {
  applyRouteFiltersFromQuery()
  await Promise.all([loadSkillCatalog(), loadKnowledgeLaneSummary()])
  if (activePane.value === 'instructions' && !instructionsLoaded.value) {
    await loadInstructionProposals()
    return
  }
  if (activePane.value === 'runner') {
    await ensureRunnerDataLoaded()
  }
})

async function loadSkillCatalog() {
  try {
    skillsLoading.value = true
    skillsError.value = ''
    const response = await skillApi.list()
    availableSkills.value = [...response.data].filter(skillFilter).sort((left, right) => {
      return normalizeText(localizedSkillName(left)).localeCompare(
        normalizeText(localizedSkillName(right))
      )
    })
    applyRequestedSkillSelection()
  } catch (error) {
    skillsError.value = getErrorMessage(error)
  } finally {
    skillsLoading.value = false
  }
}

async function loadEvolutionOverview(skillID: string) {
  const normalizedSkillID = normalizeText(skillID)
  if (!normalizedSkillID) {
    evolutionOverview.value = emptyEvolutionOverview()
    return
  }
  try {
    const response = await evolutionApi.getOverview({ skill_id: normalizedSkillID })
    if (selectedSkillID.value && selectedSkillID.value !== normalizedSkillID) return
    evolutionOverview.value = {
      skill_id: response.data.skill_id || normalizedSkillID,
      revisions: {
        accepted: response.data.revisions?.accepted || 0,
      },
      instructions: {
        pending: response.data.instructions?.pending || 0,
      },
    }
  } catch {
    if (selectedSkillID.value && selectedSkillID.value !== normalizedSkillID) return
    evolutionOverview.value = emptyEvolutionOverview(normalizedSkillID)
  }
}

async function loadSkillDetails(skillID: string) {
  const normalizedSkillID = normalizeText(skillID)
  if (!normalizedSkillID) return
  try {
    skillsLoading.value = true
    skillsError.value = ''
    const [contentResult, revisionsResult, casesResult, decisionHistoryResult] =
      await Promise.allSettled([
        skillApi.getContent(normalizedSkillID),
        harnessApi.listSkillRevisions(normalizedSkillID, { limit: 50 }),
        harnessApi.listSkillEvolutionCases(normalizedSkillID, { limit: 50 }),
        harnessApi.listSkillDecisionHistory(normalizedSkillID, { limit: 50 }),
      ])

    if (contentResult.status !== 'fulfilled') throw contentResult.reason
    if (revisionsResult.status !== 'fulfilled') throw revisionsResult.reason
    if (casesResult.status !== 'fulfilled') throw casesResult.reason

    selectedSkillContent.value = contentResult.value.data
    selectedSkillRevisions.value = sortByDateDesc(revisionsResult.value.data)
    selectedSkillCases.value = sortByDateDesc(casesResult.value.data)
    selectedSkillCaseDetailsByID.value = {}
    selectedSkillCaseDetailLoadingByID.value = {}
    if (decisionHistoryResult.status === 'fulfilled') {
      selectedSkillDecisionHistoryRecords.value = sortDecisionHistoryByDateDesc(
        decisionHistoryResult.value.data
      )
      selectedSkillDecisionHistoryLoaded.value = true
    } else {
      selectedSkillDecisionHistoryRecords.value = []
      selectedSkillDecisionHistoryLoaded.value = false
    }
    loadedSkillDetailsID.value = normalizedSkillID
    const preferredRevisionID = routedRevisionID.value || selectedRevisionID.value
    if (
      preferredRevisionID &&
      selectedSkillRevisions.value.some((revision) => revision.id === preferredRevisionID)
    ) {
      selectedRevisionID.value = preferredRevisionID
    } else {
      selectedRevisionID.value = selectInitialRevision(selectedSkillRevisions.value)
    }
  } catch (error) {
    skillsError.value = getErrorMessage(error)
    selectedSkillContent.value = null
    selectedSkillRevisions.value = []
    selectedSkillCases.value = []
    selectedSkillCaseDetailsByID.value = {}
    selectedSkillCaseDetailLoadingByID.value = {}
    selectedSkillDecisionHistoryRecords.value = []
    selectedSkillDecisionHistoryLoaded.value = false
    loadedSkillDetailsID.value = ''
    selectedRevisionID.value = ''
  } finally {
    skillsLoading.value = false
  }
}

async function refreshSkillDetails() {
  if (!selectedSkillID.value) return
  try {
    skillsRefreshing.value = true
    await loadSkillDetails(selectedSkillID.value)
  } finally {
    skillsRefreshing.value = false
  }
}

async function promoteSelectedRevision() {
  const revision = selectedRevision.value
  if (!revision || revision.status !== 'accepted') return
  const skillLabel = normalizeText(
    selectedSkill.value?.name || selectedSkill.value?.id || revision.skill_id
  )
  const sourcePath = normalizeText(revision.source_path)
  const reviewNote = selectedRevisionReviewNoteTrimmed.value
  const confirmed = window.confirm(
    [
      tr(
        'evolution.skills.promoteConfirm',
        'Promote this accepted revision to the canonical skill?'
      ),
      skillLabel ? `${tr('evolution.skills.promoteConfirmSkill', 'Skill')}: ${skillLabel}` : '',
      sourcePath
        ? `${tr('evolution.skills.promoteConfirmPath', 'Canonical file')}: ${sourcePath}`
        : '',
      reviewNote
        ? `${tr('evolution.skills.reviewNoteLabel', 'Operator rationale')}: ${reviewNote}`
        : '',
      tr(
        'evolution.skills.promoteConfirmHint',
        'This writes the accepted content into the canonical skill file and creates a backup revision for safe rollback.'
      ),
    ]
      .filter(Boolean)
      .join('\n')
  )
  if (!confirmed) return
  try {
    promoteLoadingRevisionID.value = revision.id
    const response = reviewNote
      ? await harnessApi.promoteSkillRevision(revision.id, { review_note: reviewNote })
      : await harnessApi.promoteSkillRevision(revision.id)
    notification.success(
      tr('automation.tabs.evolution', 'Evolution'),
      tr('evolution.promoteSuccess', 'Accepted skill revision promoted successfully.')
    )
    await loadSkillDetails(revision.skill_id)
    if (response.data.promoted_revision_id) {
      selectedRevisionID.value = response.data.promoted_revision_id
    }
  } catch (error) {
    notification.error(tr('automation.tabs.evolution', 'Evolution'), getErrorMessage(error))
  } finally {
    promoteLoadingRevisionID.value = ''
  }
}

async function rollbackSelectedRevision() {
  const revision = selectedRevision.value
  if (!revision || revision.status !== 'backup') return
  const skillLabel = normalizeText(
    selectedSkill.value?.name || selectedSkill.value?.id || revision.skill_id
  )
  const sourcePath = normalizeText(revision.source_path)
  const reviewNote = selectedRevisionReviewNoteTrimmed.value
  const confirmed = window.confirm(
    [
      tr('evolution.skills.rollbackConfirm', 'Restore this backup as the canonical skill?'),
      skillLabel ? `${tr('evolution.skills.rollbackConfirmSkill', 'Skill')}: ${skillLabel}` : '',
      sourcePath
        ? `${tr('evolution.skills.rollbackConfirmPath', 'Canonical file')}: ${sourcePath}`
        : '',
      reviewNote
        ? `${tr('evolution.skills.reviewNoteLabel', 'Operator rationale')}: ${reviewNote}`
        : '',
      tr(
        'evolution.skills.rollbackConfirmHint',
        'Rollback is safe only while the current canonical file still matches the lineage this backup was created from.'
      ),
    ]
      .filter(Boolean)
      .join('\n')
  )
  if (!confirmed) return
  try {
    rollbackLoadingRevisionID.value = revision.id
    const response = reviewNote
      ? await harnessApi.rollbackSkillRevision(revision.id, { review_note: reviewNote })
      : await harnessApi.rollbackSkillRevision(revision.id)
    notification.success(
      tr('automation.tabs.evolution', 'Evolution'),
      tr('evolution.rollbackSuccess', 'Backup revision restored successfully.')
    )
    await loadSkillDetails(revision.skill_id)
    if (response.data.promoted_revision_id) {
      selectedRevisionID.value = response.data.promoted_revision_id
    }
  } catch (error) {
    notification.error(tr('automation.tabs.evolution', 'Evolution'), getErrorMessage(error))
  } finally {
    rollbackLoadingRevisionID.value = ''
  }
}

async function quickRollbackDecisionHistoryEntry(entry: DecisionHistoryEntry) {
  const backupRevision = decisionHistoryRollbackRevision(entry)
  if (!backupRevision) return
  if (selectedRevisionID.value !== backupRevision.id) {
    selectedRevisionID.value = backupRevision.id
    await nextTick()
  }
  await rollbackSelectedRevision()
}

async function exportDecisionHistory() {
  if (selectedSkillDecisionHistory.value.length === 0) return
  try {
    const payload = {
      exported_at: new Date().toISOString(),
      skill_id: normalizeText(selectedSkill.value?.id),
      skill_name: normalizeText(selectedSkill.value?.name),
      decisions: selectedSkillDecisionHistory.value.map((entry) => ({
        revision_id: entry.revisionID,
        action: entry.actionLabel,
        title: entry.title,
        summary: entry.summary,
        reviewed_at: entry.reviewedAt,
        badges: entry.badges.map((badge) => badge.label),
        details: entry.details.map((detail) => ({
          key: detail.key,
          label: detail.label,
          value: detail.value,
        })),
        evidence_summary:
          entry.evidenceSummary?.map((metric) => ({
            key: metric.key,
            label: metric.label,
            value: metric.value,
            tone: metric.tone,
          })) || [],
        links: entry.links,
      })),
    }

    const blob = new Blob([JSON.stringify(payload, null, 2)], {
      type: 'application/json;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${normalizeText(selectedSkill.value?.id) || 'skill'}-decision-history.json`
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
    notification.success(
      tr('automation.tabs.evolution', 'Evolution'),
      tr(
        'evolution.skills.decisionHistory.exportSuccess',
        'Decision history exported successfully.'
      )
    )
  } catch (error) {
    notification.error(tr('automation.tabs.evolution', 'Evolution'), getErrorMessage(error))
  }
}

async function optimizeSelectedRevision() {
  const revision = selectedRevision.value
  if (!revision?.eval_run_id) return
  try {
    optimizeLoadingRevisionID.value = revision.id
    const response = await harnessApi.optimizeSkill(revision.skill_id, {
      eval_run_id: revision.eval_run_id,
      source_path: revision.source_path,
    })
    const metadata = asRecord(response.data?.metadata)
    const candidateRevisionID =
      typeof metadata?.candidate_revision_id === 'string'
        ? metadata.candidate_revision_id.trim()
        : ''
    await loadSkillDetails(revision.skill_id)
    if (candidateRevisionID) {
      selectedRevisionID.value = candidateRevisionID
    }
    notification.info(
      tr('automation.tabs.evolution', 'Evolution'),
      tr(
        'evolution.skills.optimizeTriggered',
        'A new skill evolution run was queued from the selected eval evidence.'
      )
    )
  } catch (error) {
    notification.error(tr('automation.tabs.evolution', 'Evolution'), getErrorMessage(error))
  } finally {
    optimizeLoadingRevisionID.value = ''
  }
}

async function loadInstructionProposals() {
  try {
    instructionsLoading.value = true
    instructionsError.value = ''
    const response = await selfReflectApi.listProposals({ limit: 50 })
    instructionProposals.value = [...response.data].sort((left, right) => {
      const weightDiff = proposalSortWeight(left) - proposalSortWeight(right)
      if (weightDiff !== 0) return weightDiff
      return (
        Math.max(parseDateValue(right.updated_at), parseDateValue(right.created_at)) -
        Math.max(parseDateValue(left.updated_at), parseDateValue(left.created_at))
      )
    })
    instructionsLoaded.value = true
    await applyRequestedInstructionSelection()
  } catch (error) {
    instructionsError.value = getErrorMessage(error)
  } finally {
    instructionsLoading.value = false
  }
}

async function loadProposalDetail(proposalID: string) {
  try {
    proposalDetailLoading.value = true
    const [proposalResponse, patchResponse] = await Promise.all([
      selfReflectApi.getProposal(proposalID),
      selfReflectApi.getPatchPreview(proposalID).catch(() => null),
    ])
    selectedProposal.value = proposalResponse.data
    selectedInstructionPatch.value =
      patchResponse?.data.patch_preview ||
      proposalResponse.data.patch_preview ||
      tr('evolution.instructions.noPatch', 'No patch preview available.')
  } catch (error) {
    instructionsError.value = getErrorMessage(error)
  } finally {
    proposalDetailLoading.value = false
  }
}

async function selectProposal(proposalID: string) {
  selectedProposalID.value = proposalID
  await loadProposalDetail(proposalID)
}

function openCaseRevision(skillCase: SkillEvolutionCase) {
  const revisionID = normalizeText(skillCase.revision_id)
  if (!revisionID) return
  selectedRevisionID.value = revisionID
}

function openSelectedCaseRevision() {
  const revisionID = normalizeText(
    selectedSkillCaseDetail.value?.linked_revision?.id || selectedSkillCase.value?.revision_id
  )
  if (!revisionID) return
  selectedRevisionID.value = revisionID
}

async function openEvalRun(runID: string) {
  const normalizedID = normalizeText(runID)
  if (!normalizedID) return
  await router.push({
    name: 'HarnessGroups',
    query: { evalRunId: normalizedID },
  })
}

async function openRunnerSourceEvalRun() {
  const evalRunID = runnerSourceEvalRunID.value
  if (!evalRunID) return
  await openEvalRun(evalRunID)
}

async function openRunnerFollowupEvalRun() {
  const evalRunID = runnerFollowupEvalRunID.value
  if (!evalRunID) return
  await openEvalRun(evalRunID)
}

function openRunnerLinkedRevision() {
  const revisionID = runnerLinkedRevisionID.value
  if (!revisionID) return
  const skillID = runnerLinkedSkillID.value
  activePane.value = 'skills'
  if (skillID) {
    selectedSkillID.value = skillID
  }
  selectedRevisionID.value = revisionID
}

async function openHarnessGroups() {
  await router.push({
    name: 'HarnessGroups',
  })
}

async function openHarnessGroup(groupID: string) {
  const normalizedID = normalizeText(groupID)
  if (!normalizedID) return
  await router.push({
    name: 'HarnessGroupDetail',
    params: { id: normalizedID },
  })
}

async function openCaseSource(skillCase: SkillEvolutionCase) {
  const sourceKind = normalizeText(skillCase.source_kind)
  const sourceID = normalizeText(skillCase.source_id)
  if (sourceKind !== 'eval_run' || !sourceID) return
  await openEvalRun(sourceID)
}

async function openSelectedCaseSource() {
  const sourceEvalRunID = selectedSkillCaseSourceEvalRunID.value
  if (!sourceEvalRunID) return
  await openEvalRun(sourceEvalRunID)
}

async function openSelectedCaseLinkedEvalRun() {
  const linkedEvalRunID = selectedSkillCaseLinkedEvalRunID.value
  if (!linkedEvalRunID) return
  await openEvalRun(linkedEvalRunID)
}

async function openSelectedRevisionEvalRun() {
  const evalRunID = normalizeText(selectedRevision.value?.eval_run_id)
  if (!evalRunID) return
  await openEvalRun(evalRunID)
}

async function openSelectedBaselineEvalRun() {
  const baselineEvalRunID = normalizeText(selectedSkillComparison.value.baselineEvalRunID)
  if (!baselineEvalRunID) return
  await openEvalRun(baselineEvalRunID)
}

async function openSelectedProposalSource() {
  const proposal = selectedProposal.value
  if (!proposal) return
  const sourceKind = normalizeText(proposal.source_kind)
  const sourceID = normalizeText(proposal.source_id)
  if (!sourceID) return
  if (sourceKind === 'harness_group') {
    await openHarnessGroup(sourceID)
    return
  }
  if (sourceKind === 'eval_run') {
    await openEvalRun(sourceID)
  }
}

function revealSelectedSkill() {
  skillSearch.value = ''
}

function revealSelectedRevision() {
  revisionSearch.value = ''
  revisionStatusFilter.value = 'all'
}

function revealSelectedCase() {
  caseSearch.value = ''
  caseStatusFilter.value = 'all'
  caseModeFilter.value = 'all'
}

function revealSelectedProposal() {
  instructionSearch.value = ''
  instructionStatusFilter.value = 'all'
}

function reviewLatestCandidateRevision() {
  const candidateRevisionID = latestCandidateRevision.value?.id
  if (!candidateRevisionID) return
  selectedRevisionID.value = candidateRevisionID
}

function openRunnerEvolutionPane() {
  activePane.value = 'runner'
}

async function reviewSelectedProposal(action: ProposalAction) {
  const proposal = selectedProposal.value
  if (!proposal) return
  const patchSummary = selectedInstructionPatchSummary.value
  const confirmed = window.confirm(
    [
      action === 'approve'
        ? tr('evolution.instructions.approveConfirm', 'Approve this instruction proposal?')
        : tr('evolution.instructions.rejectConfirm', 'Reject this instruction proposal?'),
      proposal.target_file
        ? `${tr('evolution.instructions.confirmTarget', 'Target file')}: ${proposal.target_file}`
        : '',
      `${tr('evolution.patchSummary.additions', 'Added lines')}: ${
        patchSummary.find((entry) => entry.key === 'additions')?.value || '0'
      }`,
      `${tr('evolution.patchSummary.deletions', 'Removed lines')}: ${
        patchSummary.find((entry) => entry.key === 'deletions')?.value || '0'
      }`,
      `${tr('evolution.patchSummary.sections', 'Change sections')}: ${
        patchSummary.find((entry) => entry.key === 'sections')?.value || '0'
      }`,
    ]
      .filter(Boolean)
      .join('\n')
  )
  if (!confirmed) return
  try {
    proposalActionLoading.value = action
    const response =
      action === 'approve'
        ? await selfReflectApi.approveProposal(proposal.id, proposalReviewNote.value)
        : await selfReflectApi.rejectProposal(proposal.id, proposalReviewNote.value)
    const updatedProposal = response.data
    selectedProposal.value = updatedProposal
    instructionProposals.value = instructionProposals.value.map((item) =>
      item.id === updatedProposal.id ? updatedProposal : item
    )
    notification.success(
      tr('automation.tabs.evolution', 'Evolution'),
      action === 'approve'
        ? tr('evolution.instructions.approved', 'Instruction proposal approved.')
        : tr('evolution.instructions.rejected', 'Instruction proposal rejected.')
    )
  } catch (error) {
    notification.error(tr('automation.tabs.evolution', 'Evolution'), getErrorMessage(error))
  } finally {
    proposalActionLoading.value = ''
  }
}
</script>

<template>
  <div class="evolution-page dashboard-page-frame">
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-4">
      <AutomationTabs class="automation-tab-strip" />

      <section
        class="overflow-hidden rounded-2xl border border-slate-200 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.12),_transparent_38%),linear-gradient(135deg,_rgba(255,255,255,0.96),_rgba(248,250,252,0.96))] shadow-sm"
      >
        <div class="flex flex-col gap-3 border-b border-slate-200/80 px-4 py-4 sm:px-5">
          <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
            <div class="space-y-2">
              <div class="flex flex-wrap items-center gap-2">
                <p class="text-xs font-semibold uppercase tracking-[0.18em] text-sky-700">
                  {{ tr('automation.tabs.evolution', 'Evolution') }}
                </p>
                <span
                  data-testid="evolution-beta-badge"
                  class="inline-flex items-center rounded-full border border-sky-200 bg-sky-100 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-sky-700"
                >
                  Beta
                </span>
              </div>
              <h1
                data-testid="evolution-page-title"
                class="text-lg font-semibold tracking-tight text-slate-950 sm:text-xl"
              >
                {{ tr('evolution.title', 'Evolution Workspace') }}
              </h1>
              <p class="max-w-2xl text-sm leading-5 text-slate-600">
                {{
                  tr(
                    'evolution.subtitle',
                    'Review better skill versions, runner candidates, and instruction proposals from one place. Promote accepted changes with evidence, keep AGENTS.md approvals explicit, and use backup revisions for safe one-step rollback when the current canonical version still matches.'
                  )
                }}
              </p>
            </div>
          </div>

          <div
            class="flex snap-x snap-mandatory gap-3 overflow-x-auto pb-1 xl:grid xl:grid-cols-4 xl:overflow-visible xl:pb-0"
          >
            <button
              v-for="lane in evolutionLaneCards"
              :key="lane.pane"
              type="button"
              :data-testid="`evolution-tab-${lane.pane}`"
              class="min-w-[15rem] shrink-0 rounded-2xl border px-3 py-3 text-left transition xl:min-w-0"
              :class="
                activePane === lane.pane
                  ? 'border-sky-300 bg-sky-50/80 shadow-sm'
                  : 'border-slate-200 bg-white/90 hover:border-slate-300 hover:bg-slate-50'
              "
              @click="activePane = lane.pane"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span
                      class="text-sm font-semibold"
                      :class="activePane === lane.pane ? 'text-sky-950' : 'text-slate-950'"
                    >
                      {{ lane.title }}
                    </span>
                    <span
                      v-if="activePane === lane.pane"
                      class="inline-flex items-center rounded-full border border-sky-200 bg-white px-2 py-0.5 text-[11px] font-medium text-sky-700"
                    >
                      {{ tr('common.active', 'Active') }}
                    </span>
                  </div>
                  <p
                    :data-testid="`evolution-lane-description-${lane.pane}`"
                    class="mt-1.5 line-clamp-2 text-xs leading-4 text-slate-600"
                  >
                    {{ lane.description }}
                  </p>
                </div>
                <div class="shrink-0 space-y-1 text-right">
                  <div
                    :data-testid="`evolution-lane-metric-${lane.pane}`"
                    class="inline-flex min-w-[3rem] justify-center rounded-full border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold text-slate-900 shadow-sm"
                  >
                    {{ lane.metric }}
                  </div>
                  <div
                    :data-testid="`evolution-lane-supporting-${lane.pane}`"
                    class="text-[11px] leading-4 text-slate-500"
                  >
                    {{ lane.supporting }}
                  </div>
                </div>
              </div>
            </button>
          </div>
        </div>

        <div
          v-if="activePane === 'knowledge'"
          class="space-y-4 p-4 sm:p-5"
        >
          <EvolutionKnowledgePane @summary-change="updateKnowledgeLaneSummary" />
        </div>

        <div
          v-else-if="activePane === 'skills'"
          class="space-y-4 p-4 sm:p-5"
        >
          <section class="sticky top-3 z-20 xl:top-6">
            <div
              class="rounded-2xl border border-slate-200 bg-white/95 p-3 shadow-sm backdrop-blur supports-[backdrop-filter]:bg-white/85 sm:p-4"
            >
              <div class="flex items-center justify-between gap-3">
                <div>
                  <h2 class="text-sm font-semibold text-slate-950">
                    {{ tr('evolution.skills.catalog', 'Canonical Skills') }}
                  </h2>
                  <p class="mt-1 text-xs leading-4 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.catalogHint',
                        'v1 revises writable built-in skills and locally managed installed skills.'
                      )
                    }}
                  </p>
                </div>
                <button
                  type="button"
                  class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                  :disabled="skillsRefreshing"
                  @click="void refreshSkillDetails()"
                >
                  {{
                    skillsRefreshing
                      ? tr('evolution.refreshing', 'Refreshing...')
                      : tr('common.refresh', 'Refresh')
                  }}
                </button>
              </div>

              <div
                v-if="skillsLoading"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{ tr('common.loading', 'Loading') }}
              </div>
              <div
                v-else-if="skillsError"
                class="mt-4 rounded-2xl border border-red-200 bg-red-50 px-3 py-4 text-sm text-red-700"
              >
                {{ skillsError }}
              </div>
              <div
                v-else-if="availableSkills.length === 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.skills.empty',
                    'No writable skills are available for evolution review yet.'
                  )
                }}
              </div>
              <div
                v-else
                class="mt-4 space-y-3"
              >
                <input
                  v-model="skillSearch"
                  data-testid="evolution-skill-search-mobile"
                  type="search"
                  class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                  :placeholder="
                    tr(
                      'evolution.skills.searchPlaceholder',
                      'Search writable skills by name, id, or description'
                    )
                  "
                >
                <div class="text-xs text-slate-500">
                  {{
                    trp('evolution.skills.searchCount', '{visible} of {total} skills shown', {
                      visible: filteredAvailableSkills.length,
                      total: availableSkills.length,
                    })
                  }}
                </div>
              </div>

              <div
                v-if="selectedSkillHiddenByFilters"
                data-testid="evolution-skill-hidden-selection-mobile"
                class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-xs text-amber-900"
              >
                <div class="text-sm font-semibold">
                  {{
                    tr(
                      'evolution.skills.hiddenSelectionTitle',
                      'Selected skill is hidden by the current search'
                    )
                  }}
                </div>
                <p class="mt-1.5 leading-4">
                  {{
                    tr(
                      'evolution.skills.hiddenSelectionHint',
                      'The selected skill still drives the detail panel on the right, but it is not visible in the filtered catalog list.'
                    )
                  }}
                </p>
                <button
                  type="button"
                  class="mt-3 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-400"
                  @click="revealSelectedSkill"
                >
                  {{ tr('evolution.skills.hiddenSelectionAction', 'Show selected skill') }}
                </button>
              </div>

              <div
                v-if="filteredAvailableSkills.length > 0"
                class="mt-4 flex snap-x snap-mandatory gap-2.5 overflow-x-auto pb-1"
              >
                <button
                  v-for="skill in filteredAvailableSkills"
                  :key="`mobile-${skill.id}`"
                  type="button"
                  class="w-[12rem] shrink-0 snap-start rounded-2xl border px-3 py-2.5 text-left transition"
                  :data-testid="`evolution-skill-item-mobile-${skill.id}`"
                  :class="
                    selectedSkillID === skill.id
                      ? 'border-sky-300 bg-sky-50/70 shadow-sm'
                      : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50'
                  "
                  @click="selectedSkillID = skill.id"
                >
                  <div class="flex items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div
                        :data-testid="`evolution-skill-item-mobile-title-${skill.id}`"
                        class="truncate text-xs font-semibold text-slate-950"
                      >
                        {{ localizedSkillName(skill) }}
                      </div>
                      <div class="mt-1 truncate text-[11px] leading-4 text-slate-500">
                        {{ skill.id }}
                      </div>
                    </div>
                    <span
                      class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600"
                    >
                      {{ skill.version }}
                    </span>
                  </div>
                  <p
                    :data-testid="`evolution-skill-item-mobile-description-${skill.id}`"
                    class="mt-1.5 line-clamp-1 text-[11px] leading-4 text-slate-600"
                  >
                    {{ localizedSkillDescription(skill) }}
                  </p>
                </button>
              </div>

              <div
                v-else-if="availableSkills.length > 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr('evolution.skills.searchEmpty', 'No writable skills match the current search.')
                }}
              </div>
            </div>
          </section>

          <aside class="hidden">
            <div class="rounded-2xl border border-slate-200 bg-white/90 p-3 shadow-sm sm:p-4">
              <div class="flex items-center justify-between gap-2">
                <div>
                  <h2 class="text-sm font-semibold text-slate-950">
                    {{ tr('evolution.skills.catalog', 'Canonical Skills') }}
                  </h2>
                  <p class="mt-1 text-xs leading-5 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.catalogHint',
                        'v1 revises writable built-in skills and locally managed installed skills.'
                      )
                    }}
                  </p>
                </div>
                <button
                  type="button"
                  class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                  :disabled="skillsRefreshing"
                  @click="void refreshSkillDetails()"
                >
                  {{
                    skillsRefreshing
                      ? tr('evolution.refreshing', 'Refreshing...')
                      : tr('common.refresh', 'Refresh')
                  }}
                </button>
              </div>

              <div
                v-if="skillsLoading"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{ tr('common.loading', 'Loading') }}
              </div>
              <div
                v-else-if="skillsError"
                class="mt-4 rounded-2xl border border-red-200 bg-red-50 px-3 py-4 text-sm text-red-700"
              >
                {{ skillsError }}
              </div>
              <div
                v-else-if="availableSkills.length === 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.skills.empty',
                    'No canonical skills are available for evolution review yet.'
                  )
                }}
              </div>
              <div
                v-else
                class="mt-4 space-y-3"
              >
                <input
                  v-model="skillSearch"
                  data-testid="evolution-skill-search"
                  type="search"
                  class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                  :placeholder="
                    tr(
                      'evolution.skills.searchPlaceholder',
                      'Search canonical skills by name, id, or description'
                    )
                  "
                >
                <div class="text-xs text-slate-500">
                  {{
                    trp('evolution.skills.searchCount', '{visible} of {total} skills shown', {
                      visible: filteredAvailableSkills.length,
                      total: availableSkills.length,
                    })
                  }}
                </div>
              </div>
              <div
                v-if="availableSkills.length > 0 && filteredAvailableSkills.length === 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.skills.searchEmpty',
                    'No canonical skills match the current search.'
                  )
                }}
              </div>
              <div
                v-if="selectedSkillHiddenByFilters"
                data-testid="evolution-skill-hidden-selection"
                class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-xs text-amber-900"
              >
                <div class="text-sm font-semibold">
                  {{
                    tr(
                      'evolution.skills.hiddenSelectionTitle',
                      'Selected skill is hidden by the current search'
                    )
                  }}
                </div>
                <p
                  data-testid="evolution-skill-hidden-selection-details"
                  class="mt-1.5 text-xs leading-4"
                >
                  {{
                    tr(
                      'evolution.skills.hiddenSelectionHint',
                      'The selected skill still drives the detail panel on the right, but it is not visible in the filtered catalog list.'
                    )
                  }}
                </p>
                <button
                  type="button"
                  data-testid="evolution-skill-hidden-selection-reveal"
                  class="mt-3 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-400"
                  @click="revealSelectedSkill"
                >
                  {{ tr('evolution.skills.hiddenSelectionAction', 'Show selected skill') }}
                </button>
              </div>
              <div
                v-if="filteredAvailableSkills.length > 0"
                class="mt-4 space-y-2"
              >
                <button
                  v-for="skill in filteredAvailableSkills"
                  :key="skill.id"
                  type="button"
                  class="w-full rounded-2xl border px-3 py-2.5 text-left transition"
                  :data-testid="`evolution-skill-item-${skill.id}`"
                  :class="
                    selectedSkillID === skill.id
                      ? 'border-sky-300 bg-sky-50/70 shadow-sm'
                      : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50'
                  "
                  @click="selectedSkillID = skill.id"
                >
                  <div class="flex items-start justify-between gap-2">
                    <div>
                      <div
                        :data-testid="`evolution-skill-item-title-${skill.id}`"
                        class="text-xs font-semibold text-slate-950"
                      >
                        {{ localizedSkillName(skill) }}
                      </div>
                      <div class="mt-1 text-[11px] leading-4 text-slate-500">
                        {{ skill.id }}
                      </div>
                    </div>
                    <span
                      class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600"
                    >
                      {{ skill.version }}
                    </span>
                  </div>
                  <p
                    :data-testid="`evolution-skill-item-description-${skill.id}`"
                    class="mt-1.5 line-clamp-1 text-[11px] leading-4 text-slate-600"
                  >
                    {{ localizedSkillDescription(skill) }}
                  </p>
                </button>
              </div>
            </div>

            <div
              class="rounded-3xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-900 shadow-sm"
            >
              <div class="text-sm font-semibold">
                {{ tr('evolution.skills.rollbackTitle', 'Safe rollback') }}
              </div>
              <p
                data-testid="evolution-skill-safe-rollback-details"
                class="mt-1.5 text-xs leading-5"
              >
                {{
                  tr(
                    'evolution.skills.rollbackHint',
                    'Promote creates backup revisions. When the current canonical skill still matches the version a backup belongs to, you can roll back one step safely from this console. If the canonical skill has changed again since then, rollback is rejected instead of overwriting newer content.'
                  )
                }}
              </p>
            </div>
          </aside>

          <section class="space-y-5">
            <div class="rounded-3xl border border-slate-200 bg-white/90 p-4 shadow-sm">
              <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div>
                  <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ tr('evolution.skills.selected', 'Selected Skill') }}
                  </div>
                  <h2
                    data-testid="evolution-skill-selected-title"
                    class="mt-1.5 text-lg font-semibold text-slate-950"
                  >
                    {{
                      selectedSkill
                        ? localizedSkillName(selectedSkill)
                        : tr('common.notAvailable', 'Not available')
                    }}
                  </h2>
                  <p
                    data-testid="evolution-skill-selected-description"
                    class="mt-1.5 max-w-3xl text-xs leading-5 text-slate-600"
                  >
                    {{
                      (selectedSkill ? localizedSkillDescription(selectedSkill) : '') ||
                        tr('evolution.skills.noDescription', 'No skill description is available.')
                    }}
                  </p>
                </div>

                <div
                  v-if="selectedRevision"
                  class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-2.5 text-xs text-slate-600"
                >
                  <div class="font-semibold text-slate-950">
                    {{ tr('evolution.skills.currentVersion', 'Revision decision') }}
                  </div>
                  <div class="mt-1.5">
                    {{ humanizeEnum(selectedRevision.status) }}
                  </div>
                  <div
                    v-if="selectedRevisionBadges.length > 0"
                    class="mt-1.5 flex flex-wrap gap-2"
                    data-testid="evolution-skill-selected-badges"
                  >
                    <span
                      v-for="badge in selectedRevisionBadges"
                      :key="badge.key"
                      class="rounded-full px-2 py-1 text-[11px] font-semibold"
                      :class="revisionBadgeClasses(badge.tone)"
                    >
                      {{ badge.label }}
                    </span>
                  </div>
                  <div class="mt-1 text-xs text-slate-500">
                    {{ formatDate(selectedRevision.created_at) }}
                  </div>
                  <div class="mt-1.5 text-[11px] leading-4 text-slate-600">
                    {{ revisionStatusSummary(selectedRevision.status) }}
                  </div>
                  <button
                    v-if="selectedRevisionCanOptimize"
                    type="button"
                    data-testid="evolution-skill-optimize"
                    class="mt-3 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="optimizeLoadingRevisionID === selectedRevision.id"
                    @click="optimizeSelectedRevision"
                  >
                    {{
                      optimizeLoadingRevisionID === selectedRevision.id
                        ? tr('evolution.skills.optimizing', 'Queueing evolution...')
                        : tr('evolution.skills.optimize', 'Run Evolution From This Eval')
                    }}
                  </button>
                  <div
                    v-if="selectedRevision.eval_run_id"
                    class="mt-1.5 text-[11px] leading-4 text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.optimizeHint',
                        'Uses the linked completed eval run as evidence to generate a fresh candidate revision.'
                      )
                    }}
                  </div>
                  <button
                    v-if="selectedRevision.eval_run_id"
                    type="button"
                    data-testid="evolution-skill-open-eval-run"
                    class="mt-3 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                    @click="void openSelectedRevisionEvalRun()"
                  >
                    {{ tr('evolution.skills.openEvalRun', 'Open linked eval run') }}
                  </button>
                  <button
                    v-if="selectedRevision.status === 'backup'"
                    type="button"
                    data-testid="evolution-skill-rollback"
                    class="mt-3 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="rollbackLoadingRevisionID === selectedRevision.id"
                    @click="rollbackSelectedRevision"
                  >
                    {{
                      rollbackLoadingRevisionID === selectedRevision.id
                        ? tr('evolution.skills.rollingBack', 'Rolling back...')
                        : tr('evolution.skills.rollback', 'Rollback To This Backup')
                    }}
                  </button>
                  <div
                    v-if="selectedRevision.followup_gate"
                    class="mt-1.5 rounded-full bg-white px-2 py-1 text-[11px] font-medium text-slate-600"
                  >
                    {{ humanizeEnum(selectedRevision.followup_gate) }}
                  </div>
                </div>
              </div>
            </div>

            <div
              v-if="selectedRevisionDecisionCards.length > 0"
              class="grid gap-4 lg:grid-cols-3"
              data-testid="evolution-skill-decision-cards"
            >
              <div
                v-for="card in selectedRevisionDecisionCards"
                :key="card.key"
                :data-testid="`evolution-skill-decision-${card.key}`"
                class="rounded-3xl border px-4 py-4 shadow-sm"
                :class="decisionSummaryCardClasses(card.tone)"
              >
                <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                  {{ card.label }}
                </div>
                <div
                  class="mt-2 text-lg font-semibold"
                  :class="decisionSummaryValueClasses(card.tone)"
                >
                  {{ card.value }}
                </div>
                <div class="mt-2 text-sm leading-6 text-slate-700">
                  {{ card.details }}
                </div>
                <button
                  v-if="card.action && card.actionLabel"
                  type="button"
                  :data-testid="`evolution-skill-decision-${card.key}-action`"
                  class="mt-3 rounded-full border border-current/20 bg-white/80 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-current/40 hover:text-slate-950"
                  @click="void card.action()"
                >
                  {{ card.actionLabel }}
                </button>
              </div>
            </div>

            <div
              v-if="selectedRevisionPromoteReadinessCards.length > 0"
              data-testid="evolution-skill-promote-readiness"
              class="rounded-3xl border border-emerald-200 bg-emerald-50/50 p-5 shadow-sm"
            >
              <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{
                      tr('evolution.skills.promoteChecklist.title', 'Promote readiness checklist')
                    }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-600">
                    {{
                      tr(
                        'evolution.skills.promoteChecklist.subtitle',
                        'Review gate outcome, evidence, eval coverage, and canonical safety metadata before switching this candidate live.'
                      )
                    }}
                  </p>
                </div>
                <div class="text-xs text-slate-500">
                  {{
                    tr(
                      'evolution.skills.promoteChecklist.explicit',
                      'Promote remains an explicit human action.'
                    )
                  }}
                </div>
              </div>

              <div class="mt-4 grid gap-3 xl:grid-cols-4">
                <div
                  v-for="card in selectedRevisionPromoteReadinessCards"
                  :key="card.key"
                  :data-testid="`evolution-skill-promote-readiness-${card.key}`"
                  class="rounded-2xl border px-4 py-4"
                  :class="decisionSummaryCardClasses(card.tone)"
                >
                  <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ card.label }}
                  </div>
                  <div
                    class="mt-2 text-base font-semibold"
                    :class="decisionSummaryValueClasses(card.tone)"
                  >
                    {{ card.value }}
                  </div>
                  <div class="mt-2 text-sm leading-6 text-slate-700">
                    {{ card.details }}
                  </div>
                </div>
              </div>
            </div>

            <div
              v-if="selectedRevisionRollbackImpactCards.length > 0"
              data-testid="evolution-skill-rollback-impact"
              class="rounded-3xl border border-amber-200 bg-amber-50/50 p-5 shadow-sm"
            >
              <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.rollbackSummary.title', 'Rollback impact summary') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-600">
                    {{
                      tr(
                        'evolution.skills.rollbackSummary.subtitle',
                        'Review what this backup would restore, which live revision it relates to, and which safety guard still protects the canonical lineage.'
                      )
                    }}
                  </p>
                </div>
                <div class="text-xs text-slate-500">
                  {{
                    tr(
                      'evolution.skills.rollbackSummary.explicit',
                      'Rollback remains an explicit human action.'
                    )
                  }}
                </div>
              </div>

              <div class="mt-4 grid gap-3 xl:grid-cols-4">
                <div
                  v-for="card in selectedRevisionRollbackImpactCards"
                  :key="card.key"
                  :data-testid="`evolution-skill-rollback-impact-${card.key}`"
                  class="rounded-2xl border px-4 py-4"
                  :class="decisionSummaryCardClasses(card.tone)"
                >
                  <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ card.label }}
                  </div>
                  <div
                    class="mt-2 text-base font-semibold"
                    :class="decisionSummaryValueClasses(card.tone)"
                  >
                    {{ card.value }}
                  </div>
                  <div class="mt-2 text-sm leading-6 text-slate-700">
                    {{ card.details }}
                  </div>
                </div>
              </div>
            </div>

            <div
              v-if="selectedRevisionSwitchPreviewCards.length > 0"
              data-testid="evolution-skill-switch-preview"
              class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm"
            >
              <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.switchPreview.title', 'Switch preview') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-600">
                    {{
                      selectedRevision?.status === 'accepted'
                        ? tr(
                          'evolution.skills.switchPreview.promoteSubtitle',
                          'Preview which revision is live now, which candidate would become live after promote, and what gets preserved for rollback.'
                        )
                        : tr(
                          'evolution.skills.switchPreview.rollbackSubtitle',
                          'Preview which revision is live now, which backup content would become live after rollback, and what gets preserved from the current lineage.'
                        )
                    }}
                  </p>
                </div>
                <div class="text-xs text-slate-500">
                  {{
                    selectedRevision?.status === 'accepted'
                      ? tr(
                        'evolution.skills.switchPreview.promoteExplicit',
                        'Promote still requires explicit confirmation.'
                      )
                      : tr(
                        'evolution.skills.switchPreview.rollbackExplicit',
                        'Rollback still requires explicit confirmation.'
                      )
                  }}
                </div>
              </div>

              <div
                class="mt-4 grid gap-3"
                :class="selectedRevision?.status === 'backup' ? 'xl:grid-cols-4' : 'xl:grid-cols-3'"
              >
                <div
                  v-for="card in selectedRevisionSwitchPreviewCards"
                  :key="card.key"
                  :data-testid="`evolution-skill-switch-preview-${card.key}`"
                  class="rounded-2xl border px-4 py-4"
                  :class="decisionSummaryCardClasses(card.tone)"
                >
                  <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ card.label }}
                  </div>
                  <div
                    class="mt-2 text-base font-semibold"
                    :class="decisionSummaryValueClasses(card.tone)"
                  >
                    {{ card.value }}
                  </div>
                  <div class="mt-2 text-sm leading-6 text-slate-700">
                    {{ card.details }}
                  </div>
                </div>
              </div>
            </div>

            <div
              v-if="selectedRevision"
              data-testid="evolution-skill-review-note"
              class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm"
            >
              <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.reviewNoteTitle', 'Operator rationale') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-600">
                    {{
                      tr(
                        'evolution.skills.reviewNoteSubtitle',
                        'Capture the human decision rationale before promote or rollback so the final switch remains explainable.'
                      )
                    }}
                  </p>
                </div>
                <div class="text-xs text-slate-500">
                  {{
                    tr(
                      'evolution.skills.reviewNoteLocalOnly',
                      'Local draft for the current review session.'
                    )
                  }}
                </div>
              </div>

              <div
                v-if="selectedRevisionReviewNoteSuggestions.length > 0"
                class="mt-4 flex flex-wrap gap-2"
              >
                <button
                  v-for="(suggestion, index) in selectedRevisionReviewNoteSuggestions"
                  :key="suggestion"
                  type="button"
                  :data-testid="`evolution-skill-review-note-suggestion-${index}`"
                  class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:bg-white hover:text-slate-950"
                  @click="selectedRevisionReviewNote = suggestion"
                >
                  {{ suggestion }}
                </button>
              </div>

              <textarea
                v-model="selectedRevisionReviewNote"
                data-testid="evolution-skill-review-note-input"
                rows="4"
                class="mt-4 w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                :placeholder="
                  tr(
                    'evolution.skills.reviewNotePlaceholder',
                    'Optional operator rationale for why this promote or rollback decision is appropriate.'
                  )
                "
              />
              <div class="mt-2 text-xs text-slate-500">
                {{
                  selectedRevisionReviewNoteTrimmed
                    ? tr(
                      'evolution.skills.reviewNoteConfirmHint',
                      'This rationale will be echoed in the final confirmation prompt.'
                    )
                    : tr(
                      'evolution.skills.reviewNoteEmptyHint',
                      'Add a short note if you want the final confirmation to include the operator rationale.'
                    )
                }}
              </div>
            </div>

            <div
              v-if="selectedRevisionSignOffCards.length > 0"
              data-testid="evolution-skill-signoff"
              class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm"
            >
              <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.signoff.title', 'Operator sign-off') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-600">
                    {{
                      tr(
                        'evolution.skills.signoff.subtitle',
                        'Final human checkpoint before promote or rollback. Review the action, audit trail, and safety guard together before confirming the switch.'
                      )
                    }}
                  </p>
                </div>
                <div class="text-xs text-slate-500">
                  {{
                    tr(
                      'evolution.skills.signoff.explicit',
                      'This preview is local to the review flow until the final confirmation is accepted.'
                    )
                  }}
                </div>
              </div>

              <div class="mt-4 grid gap-3 xl:grid-cols-4">
                <div
                  v-for="card in selectedRevisionSignOffCards"
                  :key="card.key"
                  :data-testid="`evolution-skill-signoff-${card.key}`"
                  class="rounded-2xl border px-4 py-4"
                  :class="decisionSummaryCardClasses(card.tone)"
                >
                  <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ card.label }}
                  </div>
                  <div
                    class="mt-2 text-base font-semibold"
                    :class="decisionSummaryValueClasses(card.tone)"
                  >
                    {{ card.value }}
                  </div>
                  <div class="mt-2 text-sm leading-6 text-slate-700">
                    {{ card.details }}
                  </div>
                </div>
              </div>

              <div class="mt-5 rounded-2xl border border-slate-200 bg-slate-50/80 p-4">
                <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                      {{ tr('evolution.skills.signoff.log.title', 'Decision log preview') }}
                    </div>
                    <p class="mt-1 text-sm leading-6 text-slate-600">
                      {{
                        tr(
                          'evolution.skills.signoff.log.subtitle',
                          'What the final promote or rollback decision is about to record from this review session.'
                        )
                      }}
                    </p>
                  </div>
                  <div class="text-xs text-slate-500">
                    {{
                      tr(
                        'evolution.skills.signoff.log.hint',
                        'Use the promote or rollback action after this preview looks complete.'
                      )
                    }}
                  </div>
                </div>

                <div class="mt-4 grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
                  <div
                    v-for="entry in selectedRevisionDecisionLogEntries"
                    :key="entry.key"
                    :data-testid="`evolution-skill-signoff-log-${entry.key}`"
                    class="rounded-2xl border px-4 py-3"
                    :class="decisionSummaryCardClasses(entry.tone)"
                  >
                    <div
                      class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                    >
                      {{ entry.label }}
                    </div>
                    <div
                      class="mt-2 text-sm font-semibold leading-6"
                      :class="decisionSummaryValueClasses(entry.tone)"
                    >
                      {{ entry.value }}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div
              class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm"
              data-testid="evolution-skill-scorecard"
            >
              <div class="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.scorecard', 'Evaluation Scorecard') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.scorecardHint',
                        'Compare quality, verification, runtime, tokens, and grounded evidence before opening the deeper review sections.'
                      )
                    }}
                  </p>
                </div>
                <div class="text-xs text-slate-500">
                  {{
                    tr(
                      'evolution.skills.scorecardSubtle',
                      'Built from the linked eval report and selected case evidence.'
                    )
                  }}
                </div>
              </div>

              <div class="mt-4 grid gap-4 xl:grid-cols-5">
                <div
                  v-for="card in skillScorecard"
                  :key="card.key"
                  :data-testid="`evolution-skill-scorecard-${card.key}`"
                  class="rounded-2xl border px-4 py-4"
                  :class="decisionSummaryCardClasses(card.tone)"
                >
                  <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ card.label }}
                  </div>
                  <div
                    class="mt-2 text-lg font-semibold"
                    :class="decisionSummaryValueClasses(card.tone)"
                  >
                    {{ card.value }}
                  </div>
                  <div class="mt-2 text-sm leading-6 text-slate-700">
                    {{ card.details }}
                  </div>
                  <button
                    type="button"
                    :data-testid="`evolution-skill-scorecard-${card.key}-action`"
                    class="mt-3 rounded-full border border-current/20 bg-white/80 px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-current/40 hover:text-slate-950"
                    @click="void card.action()"
                  >
                    {{ card.actionLabel }}
                  </button>
                </div>
              </div>
            </div>

            <div class="grid gap-5 xl:grid-cols-[minmax(0,0.92fr)_minmax(0,1.08fr)]">
              <div class="space-y-5">
                <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
                  <div class="flex items-center justify-between gap-3">
                    <div>
                      <h3 class="text-base font-semibold text-slate-950">
                        {{ tr('evolution.skills.cases', 'Evolution Cases') }}
                      </h3>
                      <p class="mt-1 text-sm leading-6 text-slate-500">
                        {{
                          tr(
                            'evolution.skills.casesHint',
                            'Each case captures why the system attempted to repair or extend a skill.'
                          )
                        }}
                      </p>
                    </div>
                  </div>
                  <div class="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px_180px]">
                    <input
                      v-model="caseSearch"
                      data-testid="evolution-skill-case-search"
                      type="search"
                      class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                      :placeholder="
                        tr(
                          'evolution.skills.caseSearchPlaceholder',
                          'Search case summary, source, candidate, or revision'
                        )
                      "
                    >
                    <select
                      v-model="caseModeFilter"
                      data-testid="evolution-skill-case-mode-filter"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                    >
                      <option value="all">
                        {{ tr('evolution.skills.caseModeAll', 'All modes') }}
                      </option>
                      <option value="fix">
                        {{ humanizeEnum('fix') }}
                      </option>
                      <option value="capture">
                        {{ humanizeEnum('capture') }}
                      </option>
                    </select>
                    <select
                      v-model="caseStatusFilter"
                      data-testid="evolution-skill-case-status-filter"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                    >
                      <option value="all">
                        {{ tr('evolution.skills.caseStatusAll', 'All statuses') }}
                      </option>
                      <option value="open">
                        {{ humanizeEnum('open') }}
                      </option>
                      <option value="candidate_created">
                        {{ humanizeEnum('candidate_created') }}
                      </option>
                      <option value="accepted">
                        {{ humanizeEnum('accepted') }}
                      </option>
                      <option value="rejected">
                        {{ humanizeEnum('rejected') }}
                      </option>
                      <option value="promoted">
                        {{ humanizeEnum('promoted') }}
                      </option>
                      <option value="skipped">
                        {{ humanizeEnum('skipped') }}
                      </option>
                    </select>
                  </div>
                  <div class="mt-3 text-xs text-slate-500">
                    {{
                      trp('evolution.skills.caseFilterCount', '{visible} of {total} cases shown', {
                        visible: filteredSelectedSkillCases.length,
                        total: selectedSkillCases.length,
                      })
                    }}
                  </div>
                  <div
                    v-if="selectedSkillCases.length === 0"
                    data-testid="evolution-skill-case-empty-actions"
                    class="mt-4 rounded-2xl border border-sky-200 bg-sky-50/70 px-3 py-3 text-xs text-sky-900"
                  >
                    <div class="text-sm font-semibold">
                      {{ tr('evolution.skills.caseEmptyActionTitle', 'No cases yet') }}
                    </div>
                    <p class="mt-1.5 leading-5">
                      {{
                        tr(
                          'evolution.skills.caseEmptyActionHint',
                          'Cases appear after runtime failures, partial recoveries, or capture-worthy successes. Open Harness evals to generate or inspect the evidence that feeds evolution.'
                        )
                      }}
                    </p>
                    <button
                      type="button"
                      data-testid="evolution-skill-case-empty-open-harness"
                      class="mt-3 rounded-full border border-sky-300 bg-white px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-400"
                      @click="void openHarnessGroups()"
                    >
                      {{ tr('evolution.skills.caseEmptyAction', 'Open Harness evals') }}
                    </button>
                  </div>
                  <div
                    v-if="selectedSkillCaseHiddenByFilters"
                    data-testid="evolution-skill-case-hidden-selection"
                    class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-xs text-amber-900"
                  >
                    <div class="text-sm font-semibold">
                      {{
                        tr(
                          'evolution.skills.caseHiddenSelectionTitle',
                          'Selected case is hidden by the current filters'
                        )
                      }}
                    </div>
                    <p
                      data-testid="evolution-skill-case-hidden-selection-details"
                      class="mt-1.5 text-xs leading-4"
                    >
                      {{
                        tr(
                          'evolution.skills.caseHiddenSelectionHint',
                          'The selected case still drives the evidence and lifecycle panels, but it is not visible in the filtered case list.'
                        )
                      }}
                    </p>
                    <button
                      type="button"
                      data-testid="evolution-skill-case-hidden-selection-reveal"
                      class="mt-3 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-400"
                      @click="revealSelectedCase"
                    >
                      {{ tr('evolution.skills.caseHiddenSelectionAction', 'Show selected case') }}
                    </button>
                  </div>

                  <div
                    v-if="selectedSkillCases.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.casesEmpty',
                        'No evolution cases recorded for this skill yet.'
                      )
                    }}
                  </div>
                  <div
                    v-else-if="filteredSelectedSkillCases.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.caseFilterEmpty',
                        'No evolution cases match the current filters.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 space-y-2.5"
                  >
                    <div
                      v-for="skillCase in filteredSelectedSkillCases"
                      :key="skillCase.id"
                      :data-testid="`evolution-skill-case-${skillCase.id}`"
                      class="rounded-2xl border px-3 py-2.5"
                      :class="
                        selectedSkillCase?.id === skillCase.id
                          ? 'border-sky-300 bg-sky-50/60'
                          : 'border-slate-200 bg-white'
                      "
                    >
                      <div class="flex flex-wrap items-center gap-2">
                        <span
                          class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-600"
                        >
                          {{ humanizeEnum(skillCase.mode) }}
                        </span>
                        <span
                          class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600"
                        >
                          {{ humanizeEnum(skillCase.reason) }}
                        </span>
                        <span
                          class="rounded-full bg-white px-2 py-0.5 text-[11px] font-medium text-slate-500"
                        >
                          {{ humanizeEnum(skillCase.status) }}
                        </span>
                      </div>
                      <div
                        :data-testid="`evolution-skill-case-status-summary-${skillCase.id}`"
                        class="mt-3 inline-flex rounded-full px-2 py-1 text-[11px] font-semibold"
                        :class="caseStatusSummaryClasses(caseStatusSummary(skillCase).tone)"
                      >
                        {{ caseStatusSummary(skillCase).label }}
                      </div>
                      <div
                        v-if="caseReferenceChips(skillCase).length > 0"
                        class="mt-2.5 flex flex-wrap gap-2"
                      >
                        <span
                          v-for="entry in caseReferenceChips(skillCase)"
                          :key="entry.key"
                          :data-testid="`evolution-skill-case-chip-${skillCase.id}-${entry.key}`"
                          class="rounded-full bg-slate-100 px-2 py-1 text-[11px] font-medium text-slate-600"
                        >
                          {{ entry.label }}: {{ entry.value }}
                        </span>
                      </div>
                      <p
                        :data-testid="`evolution-skill-case-summary-${skillCase.id}`"
                        class="mt-2 line-clamp-2 text-[11px] leading-4 text-slate-700"
                      >
                        {{ skillCase.summary || tr('common.notAvailable', 'Not available') }}
                      </p>
                      <div class="mt-2 flex flex-wrap items-center justify-between gap-3">
                        <div
                          :data-testid="`evolution-skill-case-updated-${skillCase.id}`"
                          class="text-[11px] leading-4 text-slate-500"
                        >
                          {{ formatDate(skillCase.updated_at) }}
                        </div>
                        <div class="flex flex-wrap gap-2">
                          <button
                            v-if="normalizeText(skillCase.revision_id)"
                            type="button"
                            :data-testid="`evolution-skill-case-open-revision-${skillCase.id}`"
                            class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                            @click="openCaseRevision(skillCase)"
                          >
                            {{ tr('evolution.skills.caseOpenRevision', 'Open linked revision') }}
                          </button>
                          <button
                            v-if="
                              normalizeText(skillCase.source_kind) === 'eval_run' &&
                                normalizeText(skillCase.source_id)
                            "
                            type="button"
                            :data-testid="`evolution-skill-case-open-source-${skillCase.id}`"
                            class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                            @click="void openCaseSource(skillCase)"
                          >
                            {{ tr('evolution.skills.caseOpenSource', 'Open source eval run') }}
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
                  <div class="flex items-center justify-between gap-3">
                    <div>
                      <h3 class="text-base font-semibold text-slate-950">
                        {{ tr('evolution.skills.revisions', 'Revision Lineage') }}
                      </h3>
                      <p class="mt-1 text-sm leading-6 text-slate-500">
                        {{
                          tr(
                            'evolution.skills.revisionsHint',
                            'Accepted revisions represent better candidate versions. Promote is the explicit switch to canonical.'
                          )
                        }}
                      </p>
                    </div>
                    <button
                      v-if="selectedRevision?.status === 'accepted'"
                      type="button"
                      data-testid="evolution-skill-promote"
                      class="rounded-full bg-slate-950 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                      :disabled="promoteLoadingRevisionID === selectedRevision.id"
                      @click="promoteSelectedRevision"
                    >
                      {{
                        promoteLoadingRevisionID === selectedRevision.id
                          ? tr('evolution.skills.promoting', 'Promoting...')
                          : tr('evolution.skills.promote', 'Promote To Canonical')
                      }}
                    </button>
                  </div>

                  <div class="mt-4 grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px]">
                    <input
                      v-model="revisionSearch"
                      data-testid="evolution-skill-revision-search"
                      type="search"
                      class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                      :placeholder="
                        tr(
                          'evolution.skills.revisionSearchPlaceholder',
                          'Search revisions by id, candidate, case, eval, or path'
                        )
                      "
                    >
                    <select
                      v-model="revisionStatusFilter"
                      data-testid="evolution-skill-revision-status-filter"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                    >
                      <option value="all">
                        {{ tr('evolution.skills.revisionStatusAll', 'All statuses') }}
                      </option>
                      <option value="accepted">
                        {{ humanizeEnum('accepted') }}
                      </option>
                      <option value="candidate">
                        {{ humanizeEnum('candidate') }}
                      </option>
                      <option value="promoted">
                        {{ humanizeEnum('promoted') }}
                      </option>
                      <option value="backup">
                        {{ humanizeEnum('backup') }}
                      </option>
                      <option value="rejected">
                        {{ humanizeEnum('rejected') }}
                      </option>
                    </select>
                  </div>
                  <div class="mt-3 text-xs text-slate-500">
                    {{
                      trp(
                        'evolution.skills.revisionFilterCount',
                        '{visible} of {total} revisions shown',
                        {
                          visible: filteredSelectedSkillRevisions.length,
                          total: selectedSkillRevisions.length,
                        }
                      )
                    }}
                  </div>
                  <div
                    v-if="!hasAcceptedRevision"
                    data-testid="evolution-skill-no-accepted-revision"
                    class="mt-4 rounded-2xl border border-sky-200 bg-sky-50/70 px-3 py-3 text-xs text-sky-900"
                  >
                    <div class="text-sm font-semibold">
                      {{
                        tr('evolution.skills.noAcceptedRevisionTitle', 'No accepted revision yet')
                      }}
                    </div>
                    <p class="mt-1.5 leading-5">
                      {{
                        latestCandidateRevision
                          ? tr(
                            'evolution.skills.noAcceptedRevisionHintCandidate',
                            'There is not an accepted version ready to promote yet. Review the latest candidate revision next.'
                          )
                          : tr(
                            'evolution.skills.noAcceptedRevisionHintHarness',
                            'There is not an accepted version ready to promote yet. Open Harness evals to gather more runtime evidence and trigger another evolution pass.'
                          )
                      }}
                    </p>
                    <button
                      v-if="latestCandidateRevision"
                      type="button"
                      data-testid="evolution-skill-no-accepted-review-candidate"
                      class="mt-3 rounded-full border border-sky-300 bg-white px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-400"
                      @click="reviewLatestCandidateRevision"
                    >
                      {{
                        tr(
                          'evolution.skills.noAcceptedRevisionActionCandidate',
                          'Review latest candidate'
                        )
                      }}
                    </button>
                    <button
                      v-else
                      type="button"
                      data-testid="evolution-skill-no-accepted-open-harness"
                      class="mt-3 rounded-full border border-sky-300 bg-white px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-400"
                      @click="void openHarnessGroups()"
                    >
                      {{
                        tr('evolution.skills.noAcceptedRevisionActionHarness', 'Open Harness evals')
                      }}
                    </button>
                  </div>
                  <div
                    v-if="selectedRevisionHiddenByFilters"
                    data-testid="evolution-skill-revision-hidden-selection"
                    class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-xs text-amber-900"
                  >
                    <div class="text-sm font-semibold">
                      {{
                        tr(
                          'evolution.skills.revisionHiddenSelectionTitle',
                          'Selected revision is hidden by the current filters'
                        )
                      }}
                    </div>
                    <p
                      data-testid="evolution-skill-revision-hidden-selection-details"
                      class="mt-1.5 text-xs leading-4"
                    >
                      {{
                        tr(
                          'evolution.skills.revisionHiddenSelectionHint',
                          'The selected revision still drives the decision, metrics, diff, and rollback panels, but it is not visible in the filtered lineage list.'
                        )
                      }}
                    </p>
                    <button
                      type="button"
                      data-testid="evolution-skill-revision-hidden-selection-reveal"
                      class="mt-3 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-400"
                      @click="revealSelectedRevision"
                    >
                      {{
                        tr(
                          'evolution.skills.revisionHiddenSelectionAction',
                          'Show selected revision'
                        )
                      }}
                    </button>
                  </div>

                  <div
                    v-if="selectedSkillRevisions.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.revisionsEmpty',
                        'No revision lineage exists for this skill yet.'
                      )
                    }}
                  </div>
                  <div
                    v-else-if="filteredSelectedSkillRevisions.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.revisionFilterEmpty',
                        'No revisions match the current filters.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 space-y-2.5"
                  >
                    <button
                      v-for="revision in filteredSelectedSkillRevisions"
                      :key="revision.id"
                      type="button"
                      :data-testid="`evolution-skill-revision-${revision.id}`"
                      :data-active="selectedRevisionID === revision.id ? 'true' : 'false'"
                      class="w-full rounded-2xl border px-3 py-2.5 text-left transition"
                      :class="
                        selectedRevisionID === revision.id
                          ? 'border-sky-300 bg-sky-50/60'
                          : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50'
                      "
                      @click="selectedRevisionID = revision.id"
                    >
                      <div class="flex flex-wrap items-center gap-2">
                        <span
                          class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-slate-600"
                        >
                          {{ humanizeEnum(revision.status) }}
                        </span>
                        <span
                          v-for="badge in revisionBadges(revision)"
                          :key="badge.key"
                          :data-testid="`evolution-skill-revision-badge-${revision.id}-${badge.key}`"
                          class="rounded-full px-2 py-0.5 text-[11px] font-semibold"
                          :class="revisionBadgeClasses(badge.tone)"
                        >
                          {{ badge.label }}
                        </span>
                        <span
                          v-if="revision.optimization_surface"
                          class="rounded-full bg-white px-2 py-0.5 text-[11px] font-medium text-slate-500"
                        >
                          {{ humanizeEnum(revision.optimization_surface) }}
                        </span>
                      </div>
                      <div
                        :data-testid="`evolution-skill-revision-title-${revision.id}`"
                        class="mt-2 text-[11px] font-medium leading-4 text-slate-950"
                      >
                        {{ revision.id }}
                      </div>
                      <div
                        :data-testid="`evolution-skill-revision-created-${revision.id}`"
                        class="mt-1 text-[11px] leading-4 text-slate-500"
                      >
                        {{ formatDate(revision.created_at) }}
                      </div>
                    </button>
                  </div>

                  <div
                    v-if="backupRevisions.length > 0"
                    class="mt-5 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4"
                  >
                    <div class="text-sm font-semibold text-slate-950">
                      {{ tr('evolution.skills.backups', 'Backup revisions') }}
                    </div>
                    <div class="mt-2 text-sm leading-6 text-slate-600">
                      {{
                        tr(
                          'evolution.skills.backupsHint',
                          'These backups preserve previous canonical content after promote. A selected backup can be restored when it still matches the current canonical lineage, giving you a safe one-step rollback path without overwriting newer canonical content by mistake.'
                        )
                      }}
                    </div>
                  </div>
                </div>

                <div
                  class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 p-4 text-sm text-amber-900"
                >
                  <div class="font-semibold">
                    {{ tr('evolution.skills.rollbackTitle', 'Safe rollback') }}
                  </div>
                  <p class="mt-2 leading-6">
                    {{
                      tr(
                        'evolution.skills.rollbackHint',
                        'Promote creates backup revisions. When the current canonical skill still matches the version a backup belongs to, you can roll back one step safely from this console. If the canonical skill has changed again since then, rollback is rejected instead of overwriting newer content.'
                      )
                    }}
                  </p>
                </div>
              </div>

              <div class="space-y-5">
                <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.timeline', 'Case Lifecycle') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.timelineHint',
                        'Follow the selected case from intake through candidate creation, gate decision, and final promotion without reconstructing the history from scattered timestamps.'
                      )
                    }}
                  </p>
                  <div
                    v-if="selectedSkillCaseTimeline.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.timelineEmpty',
                        'No lifecycle data is available for this case yet.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 space-y-2.5"
                    data-testid="evolution-skill-case-timeline"
                  >
                    <div
                      v-for="entry in selectedSkillCaseTimeline"
                      :key="entry.key"
                      :data-testid="`evolution-skill-case-timeline-${entry.key}`"
                      class="flex items-start gap-3 rounded-2xl border px-3 py-3"
                      :class="caseTimelineCardClasses(entry.state)"
                    >
                      <div
                        class="mt-1 h-3 w-3 shrink-0 rounded-full border-2"
                        :class="caseTimelineMarkerClasses(entry.state)"
                      />
                      <div class="min-w-0">
                        <div
                          :data-testid="`evolution-skill-case-timeline-label-${entry.key}`"
                          class="text-xs font-semibold text-slate-950"
                        >
                          {{ entry.label }}
                        </div>
                        <div class="mt-1 text-[11px] leading-4 text-slate-500">
                          {{ entry.time }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div
                  ref="lineageSectionRef"
                  data-testid="evolution-skill-section-lineage"
                  :data-focused="focusedSkillDetailSection === 'lineage' ? 'true' : 'false'"
                  class="rounded-3xl border bg-white/90 p-5 shadow-sm transition"
                  :class="skillDetailSectionClasses('lineage')"
                >
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.lineage', 'Revision Lineage Summary') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.lineageHint',
                        'These role and relationship markers make it easier to distinguish the current canonical version from accepted candidates, promoted history, and rollback backups.'
                      )
                    }}
                  </p>
                  <div
                    v-if="selectedRevisionLineage.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.lineageEmpty',
                        'No additional lineage markers are available for this revision.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 space-y-2.5"
                  >
                    <div
                      v-for="(entry, index) in selectedRevisionLineage"
                      :key="entry.label + entry.value"
                      :data-testid="`evolution-skill-lineage-entry-${index}`"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                    >
                      <div
                        class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                      >
                        {{ entry.label }}
                      </div>
                      <div
                        :data-testid="`evolution-skill-lineage-value-${index}`"
                        class="mt-1.5 break-all text-xs leading-5 text-slate-700"
                      >
                        {{ entry.value }}
                      </div>
                    </div>
                  </div>
                </div>

                <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
                  <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div>
                      <h3 class="text-base font-semibold text-slate-950">
                        {{ tr('evolution.skills.decisionHistory.section', 'Decision History') }}
                      </h3>
                      <p class="mt-1 text-sm leading-6 text-slate-500">
                        {{
                          tr(
                            'evolution.skills.decisionHistory.sectionHint',
                            'Persisted promote and rollback approvals stay visible here as lineage events, so the operator can see what changed live, who approved it, and which backup path was preserved.'
                          )
                        }}
                      </p>
                    </div>
                    <button
                      v-if="selectedSkillDecisionHistory.length > 0"
                      type="button"
                      data-testid="evolution-skill-decision-history-export"
                      class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                      @click="void exportDecisionHistory()"
                    >
                      {{ tr('evolution.skills.decisionHistory.export', 'Export decision log') }}
                    </button>
                  </div>
                  <div
                    v-if="selectedSkillDecisionHistory.length === 0"
                    data-testid="evolution-skill-decision-history-empty"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.decisionHistory.empty',
                        'No persisted promote or rollback decisions have been recorded for this skill yet.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    data-testid="evolution-skill-decision-history"
                    class="mt-4 space-y-3"
                  >
                    <div
                      data-testid="evolution-skill-decision-timeline"
                      class="rounded-2xl border border-slate-200 bg-slate-50/70 p-3"
                    >
                      <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                        {{ tr('evolution.skills.decisionHistory.timeline', 'Decision Timeline') }}
                      </div>
                      <p class="mt-1 text-xs leading-5 text-slate-600">
                        {{
                          tr(
                            'evolution.skills.decisionHistory.timelineHint',
                            'A compact audit trail of promote and rollback decisions across the current skill lineage.'
                          )
                        }}
                      </p>
                      <div class="mt-4 space-y-2.5">
                        <div
                          v-for="(entry, index) in selectedSkillDecisionHistory"
                          :key="`${entry.key}-timeline`"
                          :data-testid="`evolution-skill-decision-timeline-${entry.revisionID}`"
                          class="flex items-start gap-3"
                        >
                          <div class="flex flex-col items-center">
                            <div
                              class="h-3 w-3 rounded-full border-2"
                              :class="decisionTimelineMarkerClasses(entry.tone)"
                            />
                            <div
                              v-if="index < selectedSkillDecisionHistory.length - 1"
                              class="mt-2 h-10 w-px bg-slate-200"
                            />
                          </div>
                          <div
                            :data-testid="`evolution-skill-decision-timeline-card-${entry.revisionID}`"
                            class="min-w-0 rounded-2xl border border-white/90 bg-white px-3 py-2.5 shadow-sm"
                          >
                            <div
                              class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                            >
                              {{ entry.reviewedAt }}
                            </div>
                            <div
                              :data-testid="`evolution-skill-decision-timeline-title-${entry.revisionID}`"
                              class="mt-1.5 text-xs font-semibold text-slate-950"
                            >
                              {{ entry.title }}
                            </div>
                            <div
                              :data-testid="`evolution-skill-decision-timeline-summary-${entry.revisionID}`"
                              class="mt-1 text-xs leading-5 text-slate-700"
                            >
                              {{ entry.summary }}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>

                    <article
                      v-for="entry in selectedSkillDecisionHistory"
                      :key="entry.key"
                      :data-testid="`evolution-skill-decision-history-entry-${entry.revisionID}`"
                      class="rounded-3xl border border-slate-200 bg-slate-50/80 p-3"
                    >
                      <div
                        class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between"
                      >
                        <div class="min-w-0">
                          <div class="flex flex-wrap items-center gap-2">
                            <span
                              class="inline-flex rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.16em]"
                              :class="[
                                decisionSummaryCardClasses(entry.tone),
                                decisionSummaryValueClasses(entry.tone),
                              ]"
                            >
                              {{ entry.actionLabel }}
                            </span>
                            <span
                              v-for="badge in entry.badges"
                              :key="badge.key"
                              :data-testid="`evolution-skill-decision-history-${entry.revisionID}-badge-${badge.key}`"
                              class="inline-flex rounded-full border border-slate-200 bg-white px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-600"
                            >
                              {{ badge.label }}
                            </span>
                          </div>
                          <h4
                            :data-testid="`evolution-skill-decision-history-${entry.revisionID}-title`"
                            class="mt-2.5 text-sm font-semibold text-slate-950"
                          >
                            {{ entry.title }}
                          </h4>
                          <p
                            :data-testid="`evolution-skill-decision-history-${entry.revisionID}-summary`"
                            class="mt-1 text-xs leading-5 text-slate-700"
                          >
                            {{ entry.summary }}
                          </p>
                        </div>
                        <div
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-reviewed-at`"
                          class="text-xs font-medium uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ entry.reviewedAt }}
                        </div>
                      </div>

                      <!-- Decision History Actions -->
                      <div class="mt-4 flex flex-wrap gap-2">
                        <button
                          v-if="entry.links?.revisionID"
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-view-revision`"
                          class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="selectedRevisionID = entry.links.revisionID"
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.action.viewRevision',
                              'View revision'
                            )
                          }}
                        </button>
                        <button
                          v-if="entry.links?.evalRunID"
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-open-eval`"
                          class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="
                            void router.push({
                              name: 'HarnessGroups',
                              query: { evalRunId: entry.links.evalRunID },
                            })
                          "
                        >
                          {{
                            tr('evolution.skills.decisionHistory.action.openEval', 'Open eval run')
                          }}
                        </button>
                        <button
                          v-if="entry.links?.backupRevisionID"
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-view-backup`"
                          class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="selectedRevisionID = entry.links.backupRevisionID"
                        >
                          {{
                            tr('evolution.skills.decisionHistory.action.viewBackup', 'View backup')
                          }}
                        </button>
                        <button
                          v-if="decisionHistoryRollbackRevision(entry)"
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-quick-rollback`"
                          class="rounded-full border border-amber-200 bg-amber-50 px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-300"
                          @click="void quickRollbackDecisionHistoryEntry(entry)"
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.action.quickRollback',
                              'Quick rollback'
                            )
                          }}
                        </button>
                        <button
                          v-if="
                            entry.links?.sourceRevisionID &&
                              entry.links.sourceRevisionID !== entry.links.revisionID
                          "
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-view-source`"
                          class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="selectedRevisionID = entry.links.sourceRevisionID"
                        >
                          {{
                            tr('evolution.skills.decisionHistory.action.viewSource', 'View source')
                          }}
                        </button>
                        <button
                          v-if="
                            entry.links?.targetRevisionID &&
                              entry.links.targetRevisionID !== entry.links.revisionID
                          "
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-view-target`"
                          class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="selectedRevisionID = entry.links.targetRevisionID"
                        >
                          {{
                            tr('evolution.skills.decisionHistory.action.viewTarget', 'View target')
                          }}
                        </button>
                        <button
                          v-if="
                            entry.links?.currentLiveRevisionID &&
                              entry.links.currentLiveRevisionID !== entry.links.revisionID
                          "
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-view-live`"
                          class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="selectedRevisionID = entry.links.currentLiveRevisionID"
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.action.viewLive',
                              'View prior live'
                            )
                          }}
                        </button>
                        <button
                          v-if="
                            entry.links?.sourceRevisionID &&
                              entry.links?.targetRevisionID &&
                              entry.links.sourceRevisionID !== entry.links.targetRevisionID &&
                              hasSelectedSkillRevision(entry.links.sourceRevisionID) &&
                              hasSelectedSkillRevision(entry.links.targetRevisionID)
                          "
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-compare-source-target`"
                          class="rounded-full border border-sky-200 bg-sky-50 px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-300"
                          @click="
                            void openDecisionHistoryComparison({
                              key: `${entry.revisionID}-source-target`,
                              label: tr(
                                'evolution.skills.decisionHistory.compareSourceTarget',
                                'Source vs target'
                              ),
                              summary: tr(
                                'evolution.skills.decisionHistory.compareSourceTargetHint',
                                'Compare the restored/source revision against the revision that became live from this decision.'
                              ),
                              leftRevisionID: entry.links.sourceRevisionID,
                              rightRevisionID: entry.links.targetRevisionID,
                            })
                          "
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.compareSourceTargetAction',
                              'Compare source vs target'
                            )
                          }}
                        </button>
                        <button
                          v-if="
                            entry.links?.backupRevisionID &&
                              entry.links?.targetRevisionID &&
                              entry.links.backupRevisionID !== entry.links.targetRevisionID &&
                              hasSelectedSkillRevision(entry.links.backupRevisionID) &&
                              hasSelectedSkillRevision(entry.links.targetRevisionID)
                          "
                          type="button"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-action-compare-backup-live`"
                          class="rounded-full border border-sky-200 bg-sky-50 px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-300"
                          @click="
                            void openDecisionHistoryComparison({
                              key: `${entry.revisionID}-backup-live`,
                              label: tr(
                                'evolution.skills.decisionHistory.compareBackupLive',
                                'Backup vs live'
                              ),
                              summary: tr(
                                'evolution.skills.decisionHistory.compareBackupLiveHint',
                                'Compare the preserved backup against the revision that ended up live after this decision.'
                              ),
                              leftRevisionID: entry.links.backupRevisionID,
                              rightRevisionID: entry.links.targetRevisionID,
                            })
                          "
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.compareBackupLiveAction',
                              'Compare backup vs live'
                            )
                          }}
                        </button>
                      </div>

                      <div
                        v-if="
                          (entry.evidenceSummary && entry.evidenceSummary.length > 0) ||
                            (entry.links?.evalRunID &&
                              decisionHistoryReportLoadingByEvalRunID[entry.links.evalRunID])
                        "
                        :data-testid="`evolution-skill-decision-history-${entry.revisionID}-evidence-panel`"
                        class="mt-4 rounded-2xl border border-slate-200 bg-white/80 p-3"
                      >
                        <div
                          class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between"
                        >
                          <div>
                            <div
                              class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500"
                            >
                              {{
                                tr(
                                  'evolution.skills.decisionHistory.evidenceSummary',
                                  'Linked Eval Snapshot'
                                )
                              }}
                            </div>
                            <p
                              :data-testid="`evolution-skill-decision-history-${entry.revisionID}-evidence-hint`"
                              class="mt-1 text-xs leading-5 text-slate-600"
                            >
                              {{
                                tr(
                                  'evolution.skills.decisionHistory.evidenceSummaryHint',
                                  'Key runtime and quality signals from the eval report attached to this decision.'
                                )
                              }}
                            </p>
                          </div>
                          <div
                            v-if="entry.links?.evalRunID"
                            class="text-xs text-slate-500"
                          >
                            {{ entry.links.evalRunID }}
                          </div>
                        </div>

                        <div
                          v-if="
                            entry.links?.evalRunID &&
                              decisionHistoryReportLoadingByEvalRunID[entry.links.evalRunID]
                          "
                          class="mt-3 text-sm text-slate-500"
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.evidenceSummaryLoading',
                              'Loading eval snapshot...'
                            )
                          }}
                        </div>
                        <div
                          v-else-if="entry.evidenceSummary && entry.evidenceSummary.length > 0"
                          class="mt-4 grid gap-2.5 sm:grid-cols-2 xl:grid-cols-4"
                        >
                          <div
                            v-for="metric in entry.evidenceSummary"
                            :key="metric.key"
                            :data-testid="`evolution-skill-decision-history-${entry.revisionID}-evidence-${metric.key}`"
                            class="rounded-2xl border px-3 py-2.5"
                            :class="decisionHistoryEvidenceClasses(metric.tone)"
                          >
                            <div
                              class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                            >
                              {{ metric.label }}
                            </div>
                            <div class="mt-1.5 text-xs font-semibold text-slate-950">
                              {{ metric.value }}
                            </div>
                          </div>
                        </div>
                      </div>

                      <div class="mt-4 grid gap-3 lg:grid-cols-2 xl:grid-cols-3">
                        <div
                          v-for="detail in entry.details"
                          :key="detail.key"
                          :data-testid="`evolution-skill-decision-history-${entry.revisionID}-${detail.key}`"
                          class="rounded-2xl border border-white/90 bg-white px-4 py-3 shadow-sm"
                        >
                          <div
                            class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                          >
                            {{ detail.label }}
                          </div>
                          <div class="mt-2 break-all text-sm leading-6 text-slate-700">
                            {{ detail.value }}
                          </div>
                        </div>
                      </div>
                    </article>
                  </div>
                </div>

                <div class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.versionMeta', 'Version Metadata') }}
                  </h3>
                  <p class="mt-1 text-xs leading-5 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.versionMetaHint',
                        'Use these identifiers to trace why this version exists, what it was evaluated against, and which canonical content hash it was based on.'
                      )
                    }}
                  </p>
                  <div
                    v-if="selectedRevisionMeta.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.versionMetaEmpty',
                        'No additional version metadata is available for this revision.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 space-y-2.5"
                  >
                    <div
                      v-for="(entry, index) in selectedRevisionMeta"
                      :key="entry.label"
                      :data-testid="`evolution-skill-version-meta-entry-${index}`"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                    >
                      <div
                        class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                      >
                        {{ entry.label }}
                      </div>
                      <div
                        :data-testid="`evolution-skill-version-meta-value-${index}`"
                        class="mt-1.5 break-all text-xs leading-5 text-slate-700"
                      >
                        {{ entry.value }}
                      </div>
                    </div>
                  </div>
                </div>

                <div
                  ref="comparisonSectionRef"
                  data-testid="evolution-skill-section-comparison"
                  :data-focused="focusedSkillDetailSection === 'comparison' ? 'true' : 'false'"
                  class="rounded-3xl border bg-white/90 p-5 shadow-sm transition"
                  :class="skillDetailSectionClasses('comparison')"
                >
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.comparison', 'Version Comparison') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.comparisonHint',
                        'When a follow-up eval includes a baseline, these deltas make it clear why the selected revision looks better or worse than the previous version.'
                      )
                    }}
                  </p>
                  <div
                    v-if="decisionHistoryComparisonPair"
                    data-testid="evolution-skill-history-comparison"
                    class="mt-4 rounded-2xl border border-sky-200 bg-sky-50/60 p-4"
                  >
                    <div class="flex flex-col gap-2 lg:flex-row lg:items-start lg:justify-between">
                      <div>
                        <div class="text-xs font-semibold uppercase tracking-[0.16em] text-sky-700">
                          {{
                            tr(
                              'evolution.skills.decisionHistory.historyComparison',
                              'History Pair Comparison'
                            )
                          }}
                        </div>
                        <h4 class="mt-2 text-base font-semibold text-slate-950">
                          {{ decisionHistoryComparisonPair.label }}
                        </h4>
                        <p class="mt-1 text-sm leading-6 text-slate-700">
                          {{ decisionHistoryComparisonPair.summary }}
                        </p>
                      </div>
                      <button
                        type="button"
                        data-testid="evolution-skill-history-comparison-clear"
                        class="rounded-full border border-sky-200 bg-white px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-300"
                        @click="decisionHistoryComparisonPair = null"
                      >
                        {{
                          tr('evolution.skills.decisionHistory.clearComparison', 'Clear comparison')
                        }}
                      </button>
                    </div>

                    <div class="mt-4 grid gap-3 lg:grid-cols-2">
                      <div
                        data-testid="evolution-skill-history-comparison-left"
                        class="rounded-2xl border border-white/90 bg-white px-4 py-3 shadow-sm"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ tr('evolution.skills.decisionHistory.leftRevision', 'Left revision') }}
                        </div>
                        <div class="mt-2 break-all text-sm leading-6 text-slate-700">
                          {{ decisionHistoryComparisonPair.leftRevisionID }}
                        </div>
                        <button
                          type="button"
                          data-testid="evolution-skill-history-comparison-open-left"
                          class="mt-3 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="selectedRevisionID = decisionHistoryComparisonPair.leftRevisionID"
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.openLeftRevision',
                              'View left revision'
                            )
                          }}
                        </button>
                      </div>
                      <div
                        data-testid="evolution-skill-history-comparison-right"
                        class="rounded-2xl border border-white/90 bg-white px-4 py-3 shadow-sm"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{
                            tr('evolution.skills.decisionHistory.rightRevision', 'Right revision')
                          }}
                        </div>
                        <div class="mt-2 break-all text-sm leading-6 text-slate-700">
                          {{ decisionHistoryComparisonPair.rightRevisionID }}
                        </div>
                        <button
                          type="button"
                          data-testid="evolution-skill-history-comparison-open-right"
                          class="mt-3 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                          @click="
                            selectedRevisionID = decisionHistoryComparisonPair.rightRevisionID
                          "
                        >
                          {{
                            tr(
                              'evolution.skills.decisionHistory.openRightRevision',
                              'View right revision'
                            )
                          }}
                        </button>
                      </div>
                    </div>

                    <pre
                      data-testid="evolution-skill-history-comparison-diff"
                      class="mt-4 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-2xl border border-sky-100 bg-slate-950 px-4 py-4 text-xs leading-6 text-slate-100"
                    >{{ decisionHistoryComparisonDiff }}</pre>
                  </div>
                  <div
                    v-if="
                      !decisionHistoryComparisonPair &&
                        !selectedSkillComparison.baselineEvalRunID &&
                        selectedSkillComparison.deltas.length === 0
                    "
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.comparisonEmpty',
                        'No baseline comparison is attached to this revision yet.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 space-y-3"
                    data-testid="evolution-skill-comparison"
                  >
                    <div
                      v-if="selectedSkillComparison.baselineEvalRunID"
                      data-testid="evolution-skill-baseline-id"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4"
                    >
                      <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                        {{ tr('harness.evalRun.baseline', 'Baseline run') }}
                      </div>
                      <div class="mt-2 break-all text-sm leading-6 text-slate-700">
                        {{ selectedSkillComparison.baselineEvalRunID }}
                      </div>
                      <button
                        type="button"
                        data-testid="evolution-skill-open-baseline-eval-run"
                        class="mt-3 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                        @click="void openSelectedBaselineEvalRun()"
                      >
                        {{ tr('evolution.skills.openBaselineEvalRun', 'Open baseline eval run') }}
                      </button>
                    </div>
                    <div
                      v-if="selectedSkillComparison.deltas.length > 0"
                      class="grid gap-3 sm:grid-cols-2"
                    >
                      <div
                        v-for="metric in selectedSkillComparison.deltas"
                        :key="metric.key"
                        :data-testid="`evolution-skill-delta-${metric.key}`"
                        class="rounded-2xl border px-4 py-4"
                        :class="
                          metric.tone === 'positive'
                            ? 'border-emerald-200 bg-emerald-50/70'
                            : metric.tone === 'negative'
                              ? 'border-rose-200 bg-rose-50/70'
                              : 'border-slate-200 bg-slate-50'
                        "
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ metric.label }}
                        </div>
                        <div
                          class="mt-1.5 text-base font-semibold"
                          :class="
                            metric.tone === 'positive'
                              ? 'text-emerald-700'
                              : metric.tone === 'negative'
                                ? 'text-rose-700'
                                : 'text-slate-950'
                          "
                        >
                          {{ metric.value }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div
                  ref="metricsSectionRef"
                  data-testid="evolution-skill-section-metrics"
                  :data-focused="focusedSkillDetailSection === 'metrics' ? 'true' : 'false'"
                  class="rounded-3xl border bg-white/90 p-5 shadow-sm transition"
                  :class="skillDetailSectionClasses('metrics')"
                >
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.metrics', 'Reported Metrics') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.metricsHint',
                        'Quality, runtime, and token metrics come from the linked follow-up eval report when available.'
                      )
                    }}
                  </p>

                  <div
                    v-if="revisionReportLoading"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{ tr('common.loading', 'Loading') }}
                  </div>
                  <div
                    v-else-if="revisionReportError"
                    class="mt-4 rounded-2xl border border-red-200 bg-red-50 px-3 py-4 text-sm text-red-700"
                  >
                    {{ revisionReportError }}
                  </div>
                  <div
                    v-else-if="selectedSkillMetrics.length === 0"
                    class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
                  >
                    {{
                      tr(
                        'evolution.skills.metricsEmpty',
                        'No structured metrics are attached to this revision yet.'
                      )
                    }}
                  </div>
                  <div
                    v-else
                    class="mt-4 grid gap-3 sm:grid-cols-2"
                  >
                    <div
                      v-for="metric in selectedSkillMetrics"
                      :key="metric.key"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                    >
                      <div
                        class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                      >
                        {{ metric.label }}
                      </div>
                      <div class="mt-1.5 text-base font-semibold text-slate-950">
                        {{ metric.value }}
                      </div>
                    </div>
                  </div>
                </div>

                <div
                  ref="evidenceSectionRef"
                  data-testid="evolution-skill-section-evidence"
                  :data-focused="focusedSkillDetailSection === 'evidence' ? 'true' : 'false'"
                  class="rounded-3xl border bg-white/90 p-5 shadow-sm transition"
                  :class="skillDetailSectionClasses('evidence')"
                >
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.evidence', 'Case Evidence') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      selectedSkillCase?.mode === 'capture'
                        ? tr(
                          'evolution.skills.evidenceHintCapture',
                          'Capture evidence emphasizes the reusable lesson, when it should be applied, and how grounded the new experience is.'
                        )
                        : tr(
                          'evolution.skills.evidenceHint',
                          'Evidence stays tied to the specific skill version and case, so reviewers can see why the candidate exists.'
                        )
                    }}
                  </p>
                  <div class="mt-4 grid gap-3 lg:grid-cols-3">
                    <div
                      v-for="card in selectedSkillEvidenceReviewCards"
                      :key="card.key"
                      :data-testid="`evolution-skill-evidence-review-${card.key}`"
                      class="rounded-2xl border px-4 py-4"
                      :class="decisionSummaryCardClasses(card.tone)"
                    >
                      <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                        {{ card.label }}
                      </div>
                      <div
                        class="mt-2 text-base font-semibold"
                        :class="decisionSummaryValueClasses(card.tone)"
                      >
                        {{ card.value }}
                      </div>
                      <div class="mt-2 text-sm leading-6 text-slate-700">
                        {{ card.details }}
                      </div>
                    </div>
                  </div>
                  <div
                    v-if="selectedSkillEvidenceSummary.length > 0"
                    class="mt-4 grid gap-3 sm:grid-cols-3"
                  >
                    <div
                      v-for="entry in selectedSkillEvidenceSummary"
                      :key="entry.key"
                      :data-testid="`evolution-skill-evidence-summary-${entry.key}`"
                      class="rounded-2xl border px-3 py-3"
                      :class="evidenceSummaryClasses(entry.tone)"
                    >
                      <div
                        class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                      >
                        {{ entry.label }}
                      </div>
                      <div
                        :data-testid="`evolution-skill-evidence-summary-value-${entry.key}`"
                        class="mt-1.5 break-all text-base font-semibold"
                        :class="evidenceSummaryValueClasses(entry.tone)"
                      >
                        {{ entry.value }}
                      </div>
                    </div>
                  </div>
                  <div
                    v-if="
                      normalizeText(selectedCaseRevision?.id) ||
                        selectedSkillCaseSourceEvalRunID ||
                        selectedSkillCaseLinkedEvalRunID
                    "
                    class="mt-4 flex flex-wrap gap-2"
                  >
                    <button
                      v-if="normalizeText(selectedCaseRevision?.id)"
                      type="button"
                      data-testid="evolution-skill-selected-case-open-revision"
                      class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                      @click="openSelectedCaseRevision"
                    >
                      {{ tr('evolution.skills.caseOpenRevision', 'Open linked revision') }}
                    </button>
                    <button
                      v-if="selectedSkillCaseSourceEvalRunID"
                      type="button"
                      data-testid="evolution-skill-selected-case-open-source"
                      class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                      @click="void openSelectedCaseSource()"
                    >
                      {{ tr('evolution.skills.caseOpenSource', 'Open source eval run') }}
                    </button>
                    <button
                      v-if="selectedSkillCaseLinkedEvalRunID"
                      type="button"
                      data-testid="evolution-skill-selected-case-open-linked-eval"
                      class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                      @click="void openSelectedCaseLinkedEvalRun()"
                    >
                      {{ tr('evolution.skills.openEvalRun', 'Open linked eval run') }}
                    </button>
                  </div>
                  <div
                    v-if="selectedCaseMeta.length > 0"
                    data-testid="evolution-skill-case-meta-compact"
                    class="mt-4 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4"
                  >
                    <div class="grid gap-x-4 gap-y-3 sm:grid-cols-2 xl:grid-cols-3">
                      <div
                        v-for="entry in selectedCaseMeta"
                        :key="entry.label"
                        class="space-y-1"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ entry.label }}
                        </div>
                        <div class="break-all text-sm leading-5 text-slate-700">
                          {{ entry.value }}
                        </div>
                      </div>
                    </div>
                  </div>
                  <pre
                    data-testid="evolution-skill-case-evidence"
                    class="mt-4 max-h-80 overflow-auto rounded-2xl border border-slate-200 bg-slate-950 px-4 py-4 text-xs leading-6 whitespace-pre-wrap text-slate-100"
                  >{{ selectedSkillEvidence }}</pre>
                </div>

                <div
                  ref="diffSectionRef"
                  data-testid="evolution-skill-section-diff"
                  :data-focused="focusedSkillDetailSection === 'diff' ? 'true' : 'false'"
                  class="rounded-3xl border bg-white/90 p-5 shadow-sm transition"
                  :class="skillDetailSectionClasses('diff')"
                >
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.skills.diff', 'Candidate Diff Patch') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.skills.diffHint',
                        'This compares the current canonical skill against the selected candidate revision so reviewers can see what the better version would change.'
                      )
                    }}
                  </p>
                  <div class="mt-4 grid gap-3 lg:grid-cols-3">
                    <div
                      v-for="card in selectedSkillDiffReviewCards"
                      :key="card.key"
                      :data-testid="`evolution-skill-diff-review-${card.key}`"
                      class="rounded-2xl border px-4 py-4"
                      :class="decisionSummaryCardClasses(card.tone)"
                    >
                      <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                        {{ card.label }}
                      </div>
                      <div
                        class="mt-2 text-base font-semibold"
                        :class="decisionSummaryValueClasses(card.tone)"
                      >
                        {{ card.value }}
                      </div>
                      <div class="mt-2 text-sm leading-6 text-slate-700">
                        {{ card.details }}
                      </div>
                    </div>
                  </div>
                  <div class="mt-4 grid gap-3 sm:grid-cols-3">
                    <div
                      v-for="entry in selectedSkillDiffSummary"
                      :key="entry.key"
                      :data-testid="`evolution-skill-diff-summary-${entry.key}`"
                      class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                    >
                      <div
                        class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                      >
                        {{ entry.label }}
                      </div>
                      <div
                        :data-testid="`evolution-skill-diff-summary-value-${entry.key}`"
                        class="mt-1.5 text-base font-semibold text-slate-950"
                      >
                        {{ entry.value }}
                      </div>
                    </div>
                  </div>
                  <pre
                    data-testid="evolution-skill-diff"
                    class="mt-4 max-h-[520px] overflow-auto rounded-2xl border border-slate-200 bg-slate-950 px-4 py-4 text-xs leading-6 whitespace-pre-wrap text-slate-100"
                  >{{ selectedSkillDiff }}</pre>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div
          v-else-if="activePane === 'runner'"
          class="space-y-4 p-4 sm:p-5"
        >
          <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
            <h2 class="text-base font-semibold text-slate-950 sm:text-lg">
              {{ tr('evolution.runner.title', 'Runner Evolution') }}
            </h2>
            <p class="mt-2 max-w-2xl text-sm leading-5 text-slate-600">
              {{
                tr(
                  'evolution.runner.subtitle',
                  'Runner evolution is tracked separately from skill evolution so you can inspect optimized parts, runtime transcripts, and candidate/eval identifiers without mixing them into skill patch review.'
                )
              }}
            </p>
            <div
              v-if="!runnerOptimizationRecord && !agentcoreRunnerStatus"
              class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-4 py-4 text-sm leading-5 text-slate-500"
            >
              {{
                tr(
                  'evolution.runner.empty',
                  'No runner optimization record is available yet. Trigger optimization from Harness or Skills first, then return here to review the execution evidence and linked candidate.'
                )
              }}
            </div>
            <div
              v-else
              class="mt-4 space-y-4"
            >
              <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                <h3 class="text-base font-semibold text-slate-950">
                  {{ tr('evolution.runner.panelTitle', 'Runner Settings And Build Status') }}
                </h3>
                <p class="mt-1 text-sm leading-5 text-slate-500">
                  {{
                    tr(
                      'evolution.runner.panelSubtitle',
                      'The build and prepare controls stay available here so operators can inspect the candidate and still manage the underlying runner binary without leaving the Evolution console.'
                    )
                  }}
                </p>
                <div class="mt-4">
                  <AgentcoreRunnerPanel
                    :show-refresh-button="true"
                    :embedded="true"
                  />
                </div>
              </section>

              <div
                class="flex gap-3 overflow-x-auto pb-1 xl:grid xl:grid-cols-4 xl:overflow-visible xl:pb-0"
              >
                <div
                  v-for="card in runnerSummaryCards"
                  :key="card.key"
                  :data-testid="`evolution-runner-summary-${card.key}`"
                  class="min-w-[15rem] shrink-0 rounded-2xl border px-3 py-3 xl:min-w-0"
                  :class="decisionSummaryCardClasses(card.tone)"
                >
                  <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ card.label }}
                  </div>
                  <div
                    class="mt-2 text-base font-semibold"
                    :class="decisionSummaryValueClasses(card.tone)"
                  >
                    {{ card.value }}
                  </div>
                  <div class="mt-2 text-sm leading-5 text-slate-700">
                    {{ card.details }}
                  </div>
                </div>
              </div>

              <div class="grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
                <div class="space-y-4">
                  <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                    <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                      <div>
                        <h3 class="text-base font-semibold text-slate-950">
                          {{ tr('evolution.runner.candidate.title', 'Runner Candidate Patch') }}
                        </h3>
                        <p class="mt-1 text-sm leading-5 text-slate-500">
                          {{
                            tr(
                              'evolution.runner.candidate.subtitle',
                              'This shows the materialized skill candidate emitted by the optimization run. When the linked skill is already selected in the Skills lane, the panel renders a unified diff against the current canonical content.'
                            )
                          }}
                        </p>
                      </div>
                      <div
                        v-if="normalizeText(runnerOptimizationRecord?.runner_error)"
                        class="rounded-full border border-rose-200 bg-rose-50 px-3 py-1 text-xs font-semibold text-rose-700"
                      >
                        {{ tr('evolution.runner.errorBadge', 'Runner error recorded') }}
                      </div>
                    </div>

                    <div class="mt-4 grid gap-3 sm:grid-cols-3">
                      <div
                        v-for="entry in runnerCandidateDiffSummary"
                        :key="entry.key"
                        :data-testid="`evolution-runner-diff-summary-${entry.key}`"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ entry.label }}
                        </div>
                        <div
                          :data-testid="`evolution-runner-diff-summary-value-${entry.key}`"
                          class="mt-1.5 text-base font-semibold text-slate-950"
                        >
                          {{ entry.value }}
                        </div>
                      </div>
                    </div>

                    <pre
                      data-testid="evolution-runner-candidate-diff"
                      class="mt-4 max-h-[420px] overflow-auto whitespace-pre-wrap rounded-2xl border border-slate-200 bg-slate-950 px-4 py-3 text-xs leading-5 text-slate-100"
                    >{{ runnerCandidateDiff }}</pre>
                  </section>

                  <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                    <h3 class="text-base font-semibold text-slate-950">
                      {{ tr('evolution.runner.transcript.title', 'Runner Transcript') }}
                    </h3>
                    <p class="mt-1 text-sm leading-5 text-slate-500">
                      {{
                        tr(
                          'evolution.runner.transcript.subtitle',
                          'The transcript keeps the optimization exchange visible so operators can verify the runner actually used the expected prompt, response shape, and stop condition.'
                        )
                      }}
                    </p>
                    <div
                      v-if="runnerTranscriptEntries.length === 0"
                      class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-4 py-4 text-sm text-slate-500"
                    >
                      {{
                        tr(
                          'evolution.runner.transcript.empty',
                          'No transcript entries were captured for this optimization run.'
                        )
                      }}
                    </div>
                    <div
                      v-else
                      data-testid="evolution-runner-transcript"
                      class="mt-4 space-y-2.5"
                    >
                      <div
                        v-for="(entry, index) in runnerTranscriptEntries"
                        :key="entry.key"
                        :data-testid="`evolution-runner-transcript-entry-${index}`"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{
                            [
                              entry.direction || tr('evolution.runner.transcript.event', 'event'),
                              entry.method,
                            ]
                              .filter(Boolean)
                              .join(' · ')
                          }}
                        </div>
                        <div
                          :data-testid="`evolution-runner-transcript-text-${index}`"
                          class="mt-1.5 whitespace-pre-wrap break-words text-xs leading-5 text-slate-700"
                        >
                          {{ entry.text }}
                        </div>
                      </div>
                    </div>
                  </section>
                </div>

                <div class="space-y-4">
                  <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                    <h3 class="text-base font-semibold text-slate-950">
                      {{ tr('evolution.runner.links.title', 'Linked Evidence And Outputs') }}
                    </h3>
                    <p class="mt-1 text-xs leading-5 text-slate-500">
                      {{
                        tr(
                          'evolution.runner.links.subtitle',
                          'Use these links to jump from the runner evidence into the follow-up eval, the linked skill revision, or the original source eval that produced the optimization trigger.'
                        )
                      }}
                    </p>

                    <div class="mt-4 space-y-3">
                      <div
                        data-testid="evolution-runner-link-source-eval"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ tr('evolution.runner.links.sourceEval', 'Source eval run') }}
                        </div>
                        <div class="mt-1.5 break-all text-xs leading-5 text-slate-700">
                          {{ runnerSourceEvalRunID || tr('common.notAvailable', 'Not available') }}
                        </div>
                        <button
                          type="button"
                          data-testid="evolution-runner-open-source-eval"
                          class="mt-2.5 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                          :disabled="!runnerSourceEvalRunID"
                          @click="void openRunnerSourceEvalRun()"
                        >
                          {{ tr('evolution.runner.links.openSourceEval', 'Open source eval') }}
                        </button>
                      </div>

                      <div
                        data-testid="evolution-runner-link-followup-eval"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ tr('evolution.runner.links.followupEval', 'Follow-up eval run') }}
                        </div>
                        <div class="mt-1.5 break-all text-xs leading-5 text-slate-700">
                          {{
                            runnerFollowupEvalRunID || tr('common.notAvailable', 'Not available')
                          }}
                        </div>
                        <button
                          type="button"
                          data-testid="evolution-runner-open-followup-eval"
                          class="mt-2.5 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                          :disabled="!runnerFollowupEvalRunID"
                          @click="void openRunnerFollowupEvalRun()"
                        >
                          {{ tr('evolution.runner.links.openFollowupEval', 'Open follow-up eval') }}
                        </button>
                      </div>

                      <div
                        data-testid="evolution-runner-link-linked-revision"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ tr('evolution.runner.links.linkedRevision', 'Linked skill revision') }}
                        </div>
                        <div class="mt-1.5 break-all text-xs leading-5 text-slate-700">
                          {{ runnerLinkedRevisionID || tr('common.notAvailable', 'Not available') }}
                        </div>
                        <div class="mt-1.5 text-[11px] leading-4 text-slate-500">
                          {{
                            [runnerLinkedSkillID, runnerLinkedCase ? runnerLinkedCase.status : '']
                              .filter(Boolean)
                              .join(' · ') ||
                              tr(
                                'evolution.runner.links.linkedRevisionHint',
                                'Open the linked revision in the Skills lane before promoting or rolling back.'
                              )
                          }}
                        </div>
                        <button
                          type="button"
                          data-testid="evolution-runner-open-linked-revision"
                          class="mt-2.5 rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                          :disabled="!runnerLinkedRevisionID"
                          @click="openRunnerLinkedRevision"
                        >
                          {{ tr('evolution.runner.links.openLinkedRevision', 'Open in Skills') }}
                        </button>
                      </div>
                    </div>
                  </section>

                  <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                    <h3 class="text-base font-semibold text-slate-950">
                      {{ tr('evolution.runner.metrics.followupTitle', 'Follow-up Eval Snapshot') }}
                    </h3>
                    <p class="mt-1 text-xs leading-5 text-slate-500">
                      {{
                        tr(
                          'evolution.runner.metrics.followupSubtitle',
                          'These metrics come from the linked follow-up eval and should drive the accept or reject decision before any human promote step.'
                        )
                      }}
                    </p>
                    <div
                      v-if="
                        runnerFollowupEvalRunID &&
                          runnerReportLoadingByEvalRunID[runnerFollowupEvalRunID]
                      "
                      class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500"
                    >
                      {{ tr('evolution.runner.metrics.loading', 'Loading eval snapshot...') }}
                    </div>
                    <div
                      v-else-if="runnerFollowupReportMetrics.length === 0"
                      class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500"
                    >
                      {{
                        tr(
                          'evolution.runner.metrics.empty',
                          'No structured follow-up eval metrics are attached yet.'
                        )
                      }}
                    </div>
                    <div
                      v-else
                      class="mt-4 grid gap-3 sm:grid-cols-2"
                    >
                      <div
                        v-for="entry in runnerFollowupReportMetrics"
                        :key="entry.key"
                        :data-testid="`evolution-runner-report-followup-${entry.key}`"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ entry.label }}
                        </div>
                        <div class="mt-1.5 text-sm font-semibold text-slate-950">
                          {{ entry.value }}
                        </div>
                      </div>
                    </div>
                  </section>

                  <section class="rounded-2xl border border-slate-200 bg-white/90 p-4 shadow-sm">
                    <h3 class="text-base font-semibold text-slate-950">
                      {{ tr('evolution.runner.metrics.sourceTitle', 'Source Eval Snapshot') }}
                    </h3>
                    <p class="mt-1 text-xs leading-5 text-slate-500">
                      {{
                        tr(
                          'evolution.runner.metrics.sourceSubtitle',
                          'Source metrics explain why the runner was triggered in the first place and help compare pre-change evidence with the follow-up result.'
                        )
                      }}
                    </p>
                    <div
                      v-if="
                        runnerSourceEvalRunID &&
                          runnerReportLoadingByEvalRunID[runnerSourceEvalRunID]
                      "
                      class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500"
                    >
                      {{ tr('evolution.runner.metrics.loading', 'Loading eval snapshot...') }}
                    </div>
                    <div
                      v-else-if="runnerSourceReportMetrics.length === 0"
                      class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-3 text-xs leading-5 text-slate-500"
                    >
                      {{
                        tr(
                          'evolution.runner.metrics.sourceEmpty',
                          'No structured source eval metrics are attached yet.'
                        )
                      }}
                    </div>
                    <div
                      v-else
                      class="mt-4 grid gap-3 sm:grid-cols-2"
                    >
                      <div
                        v-for="entry in runnerSourceReportMetrics"
                        :key="entry.key"
                        :data-testid="`evolution-runner-report-source-${entry.key}`"
                        class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                      >
                        <div
                          class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                        >
                          {{ entry.label }}
                        </div>
                        <div class="mt-1.5 text-sm font-semibold text-slate-950">
                          {{ entry.value }}
                        </div>
                      </div>
                    </div>
                  </section>
                </div>
              </div>
            </div>
          </section>
        </div>

        <div
          v-else
          class="space-y-4 p-4 sm:p-5"
        >
          <section class="sticky top-3 z-20 xl:top-6">
            <div
              class="rounded-2xl border border-slate-200 bg-white/95 p-3 shadow-sm backdrop-blur supports-[backdrop-filter]:bg-white/85 sm:p-4"
            >
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h2 class="text-sm font-semibold text-slate-950 sm:text-base">
                    {{ tr('evolution.instructions.title', 'Instruction Review Queue') }}
                  </h2>
                  <p class="mt-2 text-sm leading-5 text-slate-600">
                    {{
                      tr(
                        'evolution.instructions.subtitle',
                        'Review AGENTS.md and related instruction patch proposals generated from grounded research takeaways. Final approval remains an explicit human decision.'
                      )
                    }}
                  </p>
                </div>
                <button
                  type="button"
                  class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                  :disabled="instructionsLoading"
                  @click="void loadInstructionProposals()"
                >
                  {{
                    instructionsLoading
                      ? tr('evolution.refreshing', 'Refreshing...')
                      : tr('common.refresh', 'Refresh')
                  }}
                </button>
              </div>

              <div
                v-if="instructionsLoading"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{ tr('common.loading', 'Loading') }}
              </div>
              <div
                v-else-if="instructionsError"
                class="mt-4 rounded-2xl border border-red-200 bg-red-50 px-3 py-4 text-sm text-red-700"
              >
                {{ instructionsError }}
              </div>
              <div
                v-else-if="instructionProposals.length === 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.instructions.empty',
                    'No instruction proposals are queued right now.'
                  )
                }}
              </div>
              <div
                v-else
                class="mt-4 space-y-3"
              >
                <input
                  v-model="instructionSearch"
                  data-testid="evolution-instructions-search-mobile"
                  type="search"
                  class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                  :placeholder="
                    tr(
                      'evolution.instructions.searchPlaceholder',
                      'Search proposals by file, lesson, or evidence'
                    )
                  "
                >
                <select
                  v-model="instructionStatusFilter"
                  data-testid="evolution-instructions-status-filter-mobile"
                  class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                >
                  <option value="all">
                    {{ tr('evolution.instructions.statusAll', 'All statuses') }}
                  </option>
                  <option value="pending">
                    {{ humanizeEnum('pending') }}
                  </option>
                  <option value="approved">
                    {{ humanizeEnum('approved') }}
                  </option>
                  <option value="rejected">
                    {{ humanizeEnum('rejected') }}
                  </option>
                </select>
                <div class="text-xs text-slate-500">
                  {{
                    trp(
                      'evolution.instructions.searchCount',
                      '{visible} of {total} proposals shown',
                      {
                        visible: filteredInstructionProposals.length,
                        total: instructionProposals.length,
                      }
                    )
                  }}
                </div>
              </div>

              <div
                v-if="selectedProposalHiddenByFilters"
                data-testid="evolution-instructions-hidden-selection-mobile"
                class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 px-3 py-3 text-sm text-amber-900"
              >
                <div class="font-semibold">
                  {{
                    tr(
                      'evolution.instructions.hiddenSelectionTitle',
                      'Selected proposal is hidden by the current filters'
                    )
                  }}
                </div>
                <p class="mt-2 leading-5">
                  {{
                    tr(
                      'evolution.instructions.hiddenSelectionHint',
                      'The selected proposal still drives the review panel on the right, but it is not visible in the filtered proposal list.'
                    )
                  }}
                </p>
                <button
                  type="button"
                  class="mt-3 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-400"
                  @click="revealSelectedProposal"
                >
                  {{ tr('evolution.instructions.hiddenSelectionAction', 'Show selected proposal') }}
                </button>
              </div>

              <div
                v-if="filteredInstructionProposals.length > 0"
                class="mt-4 flex snap-x snap-mandatory gap-3 overflow-x-auto pb-1"
              >
                <button
                  v-for="proposal in filteredInstructionProposals"
                  :key="`mobile-${proposal.id}`"
                  type="button"
                  class="min-w-[15.25rem] shrink-0 snap-start rounded-2xl border px-3 py-3 text-left transition"
                  :class="
                    selectedProposalID === proposal.id
                      ? 'border-sky-300 bg-sky-50/70 shadow-sm'
                      : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50'
                  "
                  @click="void selectProposal(proposal.id)"
                >
                  <div class="flex items-center justify-between gap-2">
                    <div class="truncate text-sm font-semibold text-slate-950">
                      {{ proposal.target_file }}
                    </div>
                    <span
                      class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600"
                    >
                      {{ humanizeEnum(proposal.status) }}
                    </span>
                  </div>
                  <p class="mt-2 line-clamp-3 text-xs leading-5 text-slate-600">
                    {{ proposal.lesson }}
                  </p>
                </button>
              </div>

              <div
                v-else-if="instructionProposals.length > 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.instructions.searchEmpty',
                    'No instruction proposals match the current filters.'
                  )
                }}
              </div>
            </div>
          </section>

          <aside class="hidden">
            <div class="rounded-2xl border border-slate-200 bg-white/90 p-3 shadow-sm sm:p-4">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h2 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.instructions.title', 'Instruction Review Queue') }}
                  </h2>
                  <p class="mt-2 text-sm leading-6 text-slate-600">
                    {{
                      tr(
                        'evolution.instructions.subtitle',
                        'Review AGENTS.md and related instruction patch proposals generated from grounded research takeaways. Final approval remains an explicit human decision.'
                      )
                    }}
                  </p>
                </div>
                <button
                  type="button"
                  class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                  :disabled="instructionsLoading"
                  @click="void loadInstructionProposals()"
                >
                  {{
                    instructionsLoading
                      ? tr('evolution.refreshing', 'Refreshing...')
                      : tr('common.refresh', 'Refresh')
                  }}
                </button>
              </div>

              <div
                v-if="instructionsLoading"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{ tr('common.loading', 'Loading') }}
              </div>
              <div
                v-else-if="instructionsError"
                class="mt-4 rounded-2xl border border-red-200 bg-red-50 px-3 py-4 text-sm text-red-700"
              >
                {{ instructionsError }}
              </div>
              <div
                v-else-if="instructionProposals.length === 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.instructions.empty',
                    'No instruction proposals are queued right now.'
                  )
                }}
              </div>
              <div
                v-if="instructionProposals.length === 0"
                data-testid="evolution-instructions-empty-actions"
                class="mt-4 rounded-2xl border border-sky-200 bg-sky-50/70 px-4 py-4 text-sm text-sky-900"
              >
                <div class="font-semibold">
                  {{ tr('evolution.instructions.emptyActionTitle', 'No proposals yet') }}
                </div>
                <p class="mt-2 leading-6">
                  {{
                    tr(
                      'evolution.instructions.emptyActionHint',
                      'Instruction proposals are generated from grounded lessons after real execution. Open Runner Evolution to inspect the source activity, then refresh proposals.'
                    )
                  }}
                </p>
                <button
                  type="button"
                  data-testid="evolution-instructions-empty-open-runner"
                  class="mt-3 rounded-full border border-sky-300 bg-white px-3 py-1.5 text-xs font-medium text-sky-900 transition hover:border-sky-400"
                  @click="openRunnerEvolutionPane"
                >
                  {{ tr('evolution.instructions.emptyAction', 'Open Runner Evolution') }}
                </button>
              </div>
              <div
                v-else
                class="mt-4 space-y-3"
              >
                <input
                  v-model="instructionSearch"
                  data-testid="evolution-instructions-search"
                  type="search"
                  class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                  :placeholder="
                    tr(
                      'evolution.instructions.searchPlaceholder',
                      'Search proposals by file, lesson, or evidence'
                    )
                  "
                >
                <select
                  v-model="instructionStatusFilter"
                  data-testid="evolution-instructions-status-filter"
                  class="w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-2.5 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                >
                  <option value="all">
                    {{ tr('evolution.instructions.statusAll', 'All statuses') }}
                  </option>
                  <option value="pending">
                    {{ humanizeEnum('pending') }}
                  </option>
                  <option value="approved">
                    {{ humanizeEnum('approved') }}
                  </option>
                  <option value="rejected">
                    {{ humanizeEnum('rejected') }}
                  </option>
                </select>
                <div class="text-xs text-slate-500">
                  {{
                    trp(
                      'evolution.instructions.searchCount',
                      '{visible} of {total} proposals shown',
                      {
                        visible: filteredInstructionProposals.length,
                        total: instructionProposals.length,
                      }
                    )
                  }}
                </div>
              </div>
              <div
                v-if="instructionProposals.length > 0 && filteredInstructionProposals.length === 0"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{
                  tr(
                    'evolution.instructions.searchEmpty',
                    'No instruction proposals match the current filters.'
                  )
                }}
              </div>
              <div
                v-if="selectedProposalHiddenByFilters"
                data-testid="evolution-instructions-hidden-selection"
                class="mt-4 rounded-2xl border border-amber-200 bg-amber-50/80 px-4 py-4 text-sm text-amber-900"
              >
                <div class="font-semibold">
                  {{
                    tr(
                      'evolution.instructions.hiddenSelectionTitle',
                      'Selected proposal is hidden by the current filters'
                    )
                  }}
                </div>
                <p class="mt-2 leading-6">
                  {{
                    tr(
                      'evolution.instructions.hiddenSelectionHint',
                      'The selected proposal still drives the review panel on the right, but it is not visible in the filtered proposal list.'
                    )
                  }}
                </p>
                <button
                  type="button"
                  data-testid="evolution-instructions-hidden-selection-reveal"
                  class="mt-3 rounded-full border border-amber-300 bg-white px-3 py-1.5 text-xs font-medium text-amber-900 transition hover:border-amber-400"
                  @click="revealSelectedProposal"
                >
                  {{ tr('evolution.instructions.hiddenSelectionAction', 'Show selected proposal') }}
                </button>
              </div>
              <div
                v-if="filteredInstructionProposals.length > 0"
                class="mt-4 space-y-2"
              >
                <button
                  v-for="proposal in filteredInstructionProposals"
                  :key="proposal.id"
                  type="button"
                  :data-testid="`evolution-instructions-proposal-${proposal.id}`"
                  class="w-full rounded-2xl border px-3 py-3 text-left transition"
                  :class="
                    selectedProposalID === proposal.id
                      ? 'border-sky-300 bg-sky-50/70 shadow-sm'
                      : 'border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50'
                  "
                  @click="void selectProposal(proposal.id)"
                >
                  <div class="flex items-center justify-between gap-2">
                    <div class="text-sm font-semibold text-slate-950">
                      {{ proposal.target_file }}
                    </div>
                    <span
                      class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium text-slate-600"
                    >
                      {{ humanizeEnum(proposal.status) }}
                    </span>
                  </div>
                  <p class="mt-2 line-clamp-3 text-xs leading-5 text-slate-600">
                    {{ proposal.lesson }}
                  </p>
                </button>
              </div>
            </div>
          </aside>

          <section class="space-y-5">
            <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
              <div class="flex flex-col gap-4">
                <div>
                  <div class="text-xs font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ tr('evolution.instructions.review', 'Review Proposal') }}
                  </div>
                  <h2
                    data-testid="evolution-instructions-review-title"
                    class="mt-2 text-lg font-semibold text-slate-950 sm:text-xl"
                  >
                    {{
                      selectedProposal?.target_file || tr('common.notAvailable', 'Not available')
                    }}
                  </h2>
                  <p class="mt-2 text-sm leading-5 text-slate-600">
                    {{
                      selectedProposal?.lesson ||
                        tr(
                          'evolution.instructions.reviewHint',
                          'Select a proposal to inspect the lesson, evidence, and patch preview.'
                        )
                    }}
                  </p>
                </div>

                <div
                  v-if="selectedProposal"
                  data-testid="evolution-instructions-review-meta"
                  class="flex flex-wrap items-center gap-2 rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-600"
                >
                  <div
                    class="inline-flex items-center rounded-full border border-slate-200 bg-white px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700"
                  >
                    {{ humanizeEnum(selectedProposal.status) }}
                  </div>
                  <div class="text-xs font-medium text-slate-500">
                    {{ formatDate(selectedProposal.updated_at) }}
                  </div>
                  <button
                    v-if="
                      normalizeText(selectedProposal.source_kind) === 'harness_group' &&
                        normalizeText(selectedProposal.source_id)
                    "
                    type="button"
                    data-testid="evolution-instructions-open-source-group"
                    class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                    @click="void openSelectedProposalSource()"
                  >
                    {{ tr('evolution.instructions.openSourceGroup', 'Open source harness group') }}
                  </button>
                  <button
                    v-else-if="
                      normalizeText(selectedProposal.source_kind) === 'eval_run' &&
                        normalizeText(selectedProposal.source_id)
                    "
                    type="button"
                    data-testid="evolution-instructions-open-source-eval-run"
                    class="rounded-full border border-slate-200 bg-white px-3 py-1.5 text-xs font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950"
                    @click="void openSelectedProposalSource()"
                  >
                    {{ tr('evolution.instructions.openSourceEvalRun', 'Open source eval run') }}
                  </button>
                </div>
              </div>
            </div>

            <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
              <h3 class="text-base font-semibold text-slate-950">
                {{ tr('evolution.instructions.evidence', 'Proposal Evidence') }}
              </h3>
              <div
                v-if="proposalDetailLoading"
                class="mt-4 rounded-2xl border border-dashed border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
              >
                {{ tr('common.loading', 'Loading') }}
              </div>
              <div
                v-else-if="selectedProposal"
                data-testid="evolution-instructions-evidence-compact"
                class="mt-4 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4"
              >
                <div class="grid gap-4 lg:grid-cols-2">
                  <div class="space-y-1.5">
                    <div
                      class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                    >
                      {{ tr('evolution.instructions.whenToApply', 'When To Apply') }}
                    </div>
                    <div class="text-sm leading-5 text-slate-700">
                      {{
                        selectedProposal.when_to_apply || tr('common.notAvailable', 'Not available')
                      }}
                    </div>
                  </div>
                  <div
                    class="space-y-1.5 border-t border-slate-200 pt-4 lg:border-t-0 lg:border-l lg:pl-4 lg:pt-0"
                  >
                    <div
                      class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                    >
                      {{ tr('evolution.instructions.evidenceText', 'Evidence') }}
                    </div>
                    <div class="text-sm leading-5 text-slate-700">
                      {{ selectedProposal.evidence || tr('common.notAvailable', 'Not available') }}
                    </div>
                  </div>
                </div>
              </div>
              <div
                v-if="selectedProposalMeta.length > 0"
                data-testid="evolution-instructions-meta-compact"
                class="mt-3 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4"
              >
                <div class="grid gap-x-4 gap-y-3 sm:grid-cols-2 xl:grid-cols-3">
                  <div
                    v-for="entry in selectedProposalMeta"
                    :key="entry.label"
                    class="space-y-1"
                  >
                    <div
                      class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500"
                    >
                      {{ entry.label }}
                    </div>
                    <div class="break-all text-sm leading-5 text-slate-700">
                      {{ entry.value }}
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="rounded-3xl border border-slate-200 bg-white/90 p-5 shadow-sm">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <h3 class="text-base font-semibold text-slate-950">
                    {{ tr('evolution.instructions.patch', 'Patch Preview') }}
                  </h3>
                  <p class="mt-1 text-sm leading-6 text-slate-500">
                    {{
                      tr(
                        'evolution.instructions.patchHint',
                        'AGENTS.md and other instruction files remain approval-driven even when the proposal comes from self-reflect.'
                      )
                    }}
                  </p>
                </div>
                <div
                  v-if="selectedProposal"
                  class="flex flex-wrap gap-2"
                >
                  <button
                    type="button"
                    data-testid="evolution-instructions-approve"
                    class="rounded-full bg-slate-950 px-4 py-2 text-sm font-medium text-white transition hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="proposalActionLoading !== ''"
                    @click="reviewSelectedProposal('approve')"
                  >
                    {{
                      proposalActionLoading === 'approve'
                        ? tr('evolution.instructions.approving', 'Approving...')
                        : tr('evolution.instructions.approve', 'Approve')
                    }}
                  </button>
                  <button
                    type="button"
                    data-testid="evolution-instructions-reject"
                    class="rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition hover:border-slate-300 hover:text-slate-950 disabled:cursor-not-allowed disabled:opacity-60"
                    :disabled="proposalActionLoading !== ''"
                    @click="reviewSelectedProposal('reject')"
                  >
                    {{
                      proposalActionLoading === 'reject'
                        ? tr('evolution.instructions.rejecting', 'Rejecting...')
                        : tr('evolution.instructions.reject', 'Reject')
                    }}
                  </button>
                </div>
              </div>

              <textarea
                v-model="proposalReviewNote"
                class="mt-4 min-h-[96px] w-full rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-700 outline-none transition focus:border-slate-300 focus:bg-white"
                :placeholder="
                  tr(
                    'evolution.instructions.reviewNotePlaceholder',
                    'Optional review note for the proposal decision.'
                  )
                "
              />

              <div class="mt-4 grid gap-3 sm:grid-cols-3">
                <div
                  v-for="entry in selectedInstructionPatchSummary"
                  :key="entry.key"
                  :data-testid="`evolution-instructions-patch-summary-${entry.key}`"
                  class="rounded-2xl border border-slate-200 bg-slate-50 px-3 py-3"
                >
                  <div class="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">
                    {{ entry.label }}
                  </div>
                  <div
                    :data-testid="`evolution-instructions-patch-summary-value-${entry.key}`"
                    class="mt-1.5 text-base font-semibold text-slate-950"
                  >
                    {{ entry.value }}
                  </div>
                </div>
              </div>

              <pre
                data-testid="evolution-instructions-patch"
                class="mt-4 max-h-[520px] overflow-auto rounded-2xl border border-slate-200 bg-slate-950 px-4 py-4 text-xs leading-6 whitespace-pre-wrap text-slate-100"
              >{{
                  selectedInstructionPatch ||
                  tr('evolution.instructions.noPatch', 'No patch preview available.')
              }}</pre>
            </div>
          </section>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.evolution-page {
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}
</style>
