<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  harnessApi,
  type HarnessArtifactRef,
  type HarnessCheckpointArtifact,
  type HarnessContract,
  type HarnessGroupPromotionResult,
  type HarnessPromoteGroupSpec,
  type HarnessRuntimeEvidenceEntry,
  type HarnessRunGroup,
  type HarnessRunGroupItem,
  type HarnessRunGroupReport,
  type HarnessRunGroupStatus,
  type HarnessRunSummary,
  type HarnessScorecard,
} from '@/api/harness'
import { useNotificationStore } from '@/stores/notification'
import { harnessFailureLabelHint } from '@/utils/harnessFailureHints'
import { getErrorMessage } from '@/utils/error'

type FailedItemRow = {
  item?: HarnessRunGroupItem | null
  scorecard?: HarnessScorecard | null
  run?: HarnessRunSummary | null
}

type NumberEntry = {
  key: string
  value: number
}

type ScorecardVerificationDiagnostics = {
  passed: boolean | null
  retryable: boolean | null
  summary: string
  failureLabel: string
  outcomeScore: number | null
  evidenceScore: number | null
  executionScore: number | null
  observations: string[]
  checks: VerificationCheckRow[]
  artifacts: VerificationArtifactRow[]
  traceSummary: VerificationTraceSummary
}

type VerificationCheckRow = {
  name: string
  expected: string
  actual: string
  passed: boolean
}

type VerificationArtifactRow = {
  target: string
  actual: string
  passed: boolean
}

type VerificationTraceSummary = {
  eventCount: number | null
  artifactCount: number | null
  toolNames: string[]
  eventsError: string
  artifactsError: string
}

type ScorecardDiagnosticRow = {
  id: string
  itemID: string
  itemIndex: number | null
  verdict: string
  score: number
  judgeBackend: string
  judgeModel: string
  calibrationRef: string
  takeawayCandidateCount: number | null
  proposalCount: number | null
  proposalIDs: string[]
  proposalSkippedReason: string
  verification: ScorecardVerificationDiagnostics
}

type FailedItemDiagnosticRow = FailedItemRow & {
  verification: ScorecardVerificationDiagnostics
}

type ContractSummaryRow = {
  itemID: string
  itemIndex: number | null
  profile: string
  contract: HarnessContract
}

type RuntimeEvidenceRunRow = {
  run: HarnessRunSummary | null
  entries: HarnessRuntimeEvidenceEntry[]
}

type GroupPromotionFormState = {
  datasetName: string
  description: string
  subject: string
  evalName: string
}

const route = useRoute()
const { t, te } = useI18n()
const notification = useNotificationStore()

const loading = ref(false)
const refreshing = ref(false)
const actionLoading = ref<'cancel' | 'retry' | ''>('')
const promotionLoading = ref(false)
const error = ref('')
const report = ref<HarnessRunGroupReport | null>(null)
const promotionResult = ref<HarnessGroupPromotionResult | null>(null)
const promotionForm = ref<GroupPromotionFormState>({
  datasetName: '',
  description: '',
  subject: '',
  evalName: '',
})

let refreshTimer: ReturnType<typeof setInterval> | null = null

const groupID = computed(() => String(route.params.id || '').trim())
const activeStatuses = new Set<HarnessRunGroupStatus>(['pending', 'queued', 'running', 'scoring'])

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function humanizeEnum(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function safeJSON(raw?: string): Record<string, unknown> | null {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw)
    return asRecord(parsed)
  } catch {
    return null
  }
}

function firstNonEmpty(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    const normalized = String(value || '').trim()
    if (normalized) return normalized
  }
  return ''
}

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function readText(records: Array<Record<string, unknown> | null>, key: string): string {
  for (const record of records) {
    const value = String(record?.[key] || '').trim()
    if (value) return value
  }
  return ''
}

function readRecord(
  records: Array<Record<string, unknown> | null>,
  key: string
): Record<string, unknown> | null {
  for (const record of records) {
    const value = asRecord(record?.[key])
    if (value) return value
  }
  return null
}

function readNumber(records: Array<Record<string, unknown> | null>, key: string): number | null {
  for (const record of records) {
    const value = Number(record?.[key])
    if (Number.isFinite(value)) return value
  }
  return null
}

function readBoolean(records: Array<Record<string, unknown> | null>, key: string): boolean | null {
  for (const record of records) {
    const value = record?.[key]
    if (typeof value === 'boolean') return value
  }
  return null
}

function readStringList(records: Array<Record<string, unknown> | null>, key: string): string[] {
  for (const record of records) {
    const value = record?.[key]
    if (!Array.isArray(value)) continue
    const items = value
      .map((entry) => String(entry || '').trim())
      .filter((entry) => entry.length > 0)
    if (items.length) return items
  }
  return []
}

function readRecordList(
  records: Array<Record<string, unknown> | null>,
  key: string
): Record<string, unknown>[] {
  for (const record of records) {
    const value = record?.[key]
    if (!Array.isArray(value)) continue
    const items = value.map((entry) => asRecord(entry)).filter(Boolean) as Record<string, unknown>[]
    if (items.length) return items
  }
  return []
}

function percentLabel(value?: number): string {
  return `${Math.round(Number(value || 0) * 100)}%`
}

function optionalPercentLabel(value?: number | null): string {
  return value == null ? tr('common.notAvailable', 'Not available') : percentLabel(value)
}

function scoreLabel(value?: number): string {
  return Number(value || 0).toFixed(2)
}

function compactValueLabel(value: unknown): string {
  if (value == null) return tr('common.notAvailable', 'Not available')
  if (Array.isArray(value)) {
    const text = value
      .map((entry) => String(entry || '').trim())
      .filter(Boolean)
      .join(', ')
    return compactValueLabel(text)
  }
  if (typeof value === 'object') {
    try {
      return compactValueLabel(JSON.stringify(value))
    } catch {
      return tr('common.notAvailable', 'Not available')
    }
  }
  const text = String(value).trim()
  if (!text) return tr('common.notAvailable', 'Not available')
  return text.length > 140 ? `${text.slice(0, 137)}...` : text
}

function summaryValue(key: string): number | null {
  const summary = asRecord(report.value?.group?.summary)
  if (!summary || !(key in summary)) return null
  const value = Number(summary[key])
  return Number.isFinite(value) ? value : null
}

function summaryNumber(key: string): number {
  return summaryValue(key) ?? 0
}

function summaryNumberMap(key: string): Record<string, number> {
  const summary = asRecord(report.value?.group?.summary)
  const source = asRecord(summary?.[key])
  if (!source) return {}
  const out: Record<string, number> = {}
  for (const [entryKey, raw] of Object.entries(source)) {
    const value = Number(raw)
    if (Number.isFinite(value)) out[entryKey] = value
  }
  return out
}

function sortedNumberEntries(values: Record<string, number>): NumberEntry[] {
  return Object.entries(values)
    .filter(([, value]) => Number.isFinite(value) && value !== 0)
    .map(([key, value]) => ({ key, value }))
    .sort((left, right) => {
      if (right.value !== left.value) return right.value - left.value
      return left.key.localeCompare(right.key)
    })
}

function boolLabel(value: boolean): string {
  return value ? tr('common.yes', 'Yes') : tr('common.no', 'No')
}

function summaryCount(key: string): number {
  const summary = asRecord(report.value?.group?.summary)
  const counts = asRecord(summary?.counts)
  if (!counts) return 0
  const value = Number(counts[key])
  return Number.isFinite(value) ? value : 0
}

const group = computed<HarnessRunGroup | null>(() => report.value?.group || null)
const isEphemeralQuickEval = computed(() => asRecord(group.value?.metadata)?.ephemeral === true)
const items = computed(() => report.value?.items || [])
const scorecards = computed(() => report.value?.scorecards || [])
const linkedRuns = computed(() => report.value?.linked_runs || [])
const artifacts = computed(() => report.value?.artifacts || [])
const checkpoints = computed(() => report.value?.checkpoints || [])
const runtimeEvidence = computed(() => report.value?.runtime_evidence || {})
const itemContracts = computed(() => report.value?.item_contracts || {})
const verdictCounts = computed(() => report.value?.verdict_counts || {})
const failedItems = computed<FailedItemDiagnosticRow[]>(() =>
  (report.value?.failed_items || []).map((entry) => {
    const record = asRecord(entry)
    const scorecard = (record?.scorecard as HarnessScorecard | undefined) || null
    return {
      item: (record?.item as HarnessRunGroupItem | undefined) || null,
      scorecard,
      run: (record?.run as HarnessRunSummary | undefined) || null,
      verification: buildScorecardVerificationDiagnostics(scorecard),
    }
  })
)

const scorecardByItemID = computed<Record<string, HarnessScorecard>>(() => {
  const out: Record<string, HarnessScorecard> = {}
  for (const card of scorecards.value) {
    if (!card.group_item_id || out[card.group_item_id]) continue
    out[card.group_item_id] = card
  }
  return out
})
const itemByID = computed<Record<string, HarnessRunGroupItem>>(() => {
  const out: Record<string, HarnessRunGroupItem> = {}
  for (const item of items.value) out[item.id] = item
  return out
})

function buildScorecardVerificationDiagnostics(
  scorecard?: HarnessScorecard | null
): ScorecardVerificationDiagnostics {
  const breakdown = safeJSON(scorecard?.breakdown_json)
  const evidence = safeJSON(scorecard?.evidence_json)
  const trace = safeJSON(scorecard?.judge_trace_json)
  const verification = readRecord([evidence, trace], 'verification')
  const checks = readRecordList([verification], 'checks')
    .map((entry) => ({
      name: String(entry.name || '').trim(),
      expected: compactValueLabel(entry.expected),
      actual: compactValueLabel(entry.actual),
      passed: entry.passed === true,
    }))
    .sort((left, right) => Number(left.passed) - Number(right.passed))
  const artifacts = readRecordList([verification], 'artifacts')
    .map((entry) => ({
      target: compactValueLabel(entry.path || entry.label),
      actual: compactValueLabel(entry.actual),
      passed: entry.passed === true,
    }))
    .sort((left, right) => Number(left.passed) - Number(right.passed))
  const traceSummary = readRecord([verification], 'trace_summary')

  return {
    passed:
      readBoolean([verification], 'passed') ?? readBoolean([breakdown], 'verification_passed'),
    retryable: readBoolean([verification, breakdown, trace], 'retryable'),
    summary: readText([verification, breakdown, trace], 'summary'),
    failureLabel: readText([verification, breakdown, trace], 'failure_label'),
    outcomeScore: readNumber([verification, breakdown], 'outcome_score'),
    evidenceScore: readNumber([verification, breakdown], 'evidence_score'),
    executionScore: readNumber([verification, breakdown], 'execution_score'),
    observations: readStringList([verification, breakdown, trace], 'observations'),
    checks,
    artifacts,
    traceSummary: {
      eventCount: readNumber([traceSummary], 'event_count'),
      artifactCount: readNumber([traceSummary], 'artifact_count'),
      toolNames: readStringList([traceSummary], 'tool_names'),
      eventsError: readText([traceSummary], 'events_error'),
      artifactsError: readText([traceSummary], 'artifacts_error'),
    },
  }
}

const scorecardDiagnostics = computed<ScorecardDiagnosticRow[]>(() =>
  scorecards.value.map((card) => {
    const breakdown = safeJSON(card.breakdown_json)
    const evidence = safeJSON(card.evidence_json)
    const trace = safeJSON(card.judge_trace_json)
    const records = [breakdown, trace]
    return {
      id: card.id,
      itemID: card.group_item_id,
      itemIndex: itemByID.value[card.group_item_id]?.index ?? null,
      verdict: card.verdict,
      score: Number(card.score || 0),
      judgeBackend: readText(records, 'judge_backend'),
      judgeModel: readText(records, 'judge_model'),
      calibrationRef: readText(records, 'calibration_ref'),
      takeawayCandidateCount: readNumber(records, 'takeaway_candidate_count'),
      proposalCount: readNumber(records, 'proposal_count'),
      proposalIDs: readStringList(records, 'proposal_ids'),
      proposalSkippedReason: readText(records, 'proposal_skipped_reason'),
      verification: buildScorecardVerificationDiagnostics({
        ...card,
        evidence_json: card.evidence_json || JSON.stringify(evidence || {}),
      }),
    }
  })
)
const scorecardDiagnosticsByItemID = computed<Record<string, ScorecardDiagnosticRow>>(() => {
  const out: Record<string, ScorecardDiagnosticRow> = {}
  for (const row of scorecardDiagnostics.value) {
    if (!row.itemID || out[row.itemID]) continue
    out[row.itemID] = row
  }
  return out
})
const contractRows = computed<ContractSummaryRow[]>(() =>
  Object.entries(itemContracts.value)
    .map(([itemID, contract]) => ({
      itemID,
      itemIndex: itemByID.value[itemID]?.index ?? null,
      profile: itemByID.value[itemID]?.profile || '',
      contract,
    }))
    .sort((left, right) => Number(left.itemIndex ?? 9999) - Number(right.itemIndex ?? 9999))
)
const runtimeEvidenceRows = computed<RuntimeEvidenceRunRow[]>(() =>
  linkedRuns.value
    .map((run) => ({
      run,
      entries: [...(runtimeEvidence.value[run.id] || [])].sort((left, right) => {
        const leftTime = Date.parse(left.created_at || '')
        const rightTime = Date.parse(right.created_at || '')
        return leftTime - rightTime
      }),
    }))
    .filter((row) => row.entries.length > 0)
)
const hasScorecardDiagnostics = computed(() =>
  scorecardDiagnostics.value.some(
    (row) =>
      row.judgeBackend ||
      row.judgeModel ||
      row.calibrationRef ||
      row.takeawayCandidateCount != null ||
      row.proposalCount != null ||
      row.proposalIDs.length > 0 ||
      row.proposalSkippedReason ||
      row.verification.passed != null ||
      row.verification.retryable != null ||
      row.verification.failureLabel ||
      row.verification.summary ||
      row.verification.outcomeScore != null ||
      row.verification.evidenceScore != null ||
      row.verification.executionScore != null ||
      row.verification.observations.length > 0 ||
      row.verification.checks.length > 0 ||
      row.verification.artifacts.length > 0 ||
      row.verification.traceSummary.eventCount != null ||
      row.verification.traceSummary.artifactCount != null ||
      row.verification.traceSummary.toolNames.length > 0 ||
      row.verification.traceSummary.eventsError ||
      row.verification.traceSummary.artifactsError
  )
)

const failureLabelEntries = computed(() =>
  sortedNumberEntries(summaryNumberMap('failure_label_counts'))
)
const failureLabelHintEntries = computed(() =>
  failureLabelEntries.value.map((entry) => ({
    ...entry,
    hint: remediationHint(entry.key),
  }))
)

const hasTerminalGroup = computed(() => {
  return !group.value || !activeStatuses.has(group.value.status)
})

const totalAttempts = computed(() =>
  items.value.reduce((sum, item) => sum + Number(item.attempt_count || 0), 0)
)

function statusTone(status: string): string {
  switch (status) {
    case 'completed':
    case 'passed':
    case 'pass':
      return 'is-success'
    case 'running':
    case 'scoring':
    case 'executing':
      return 'is-running'
    case 'partial':
    case 'queued':
    case 'pending':
    case 'planning':
    case 'waiting_input':
      return 'is-warning'
    case 'failed':
    case 'cancelled':
    case 'aborted':
    case 'error':
    case 'fail':
      return 'is-danger'
    default:
      return 'is-muted'
  }
}

function statusLabel(status?: string | null): string {
  const value = String(status || '').trim()
  switch (value) {
    case 'running':
      return tr('harness.groups.running', 'Running')
    case 'failed':
      return tr('harness.groups.failed', 'Failed')
    case 'passed':
      return tr('harness.groups.passed', 'Passed')
    case 'pass':
      return tr('harness.group.passVerdict', 'Pass')
    case 'fail':
      return tr('harness.group.failVerdict', 'Fail')
    case 'partial':
      return tr('harness.group.partialVerdict', 'Partial')
    case 'error':
      return tr('harness.group.errorVerdict', 'Error')
    case 'queued':
      return tr('harness.group.queuedCount', 'Queued')
    default:
      return value ? humanizeEnum(value) : tr('common.notAvailable', 'Not available')
  }
}

function kindLabel(kind?: string | null): string {
  const value = String(kind || '').trim()
  return value ? humanizeEnum(value) : tr('nav.harness', 'Harness')
}

function modeLabel(mode?: string | null): string {
  const value = String(mode || '').trim()
  return value ? humanizeEnum(value) : tr('common.notAvailable', 'Not available')
}

function artifactKindLabel(kind?: string | null): string {
  const value = String(kind || '').trim()
  return value ? humanizeEnum(value) : tr('common.notAvailable', 'Not available')
}

function profileLabel(profile?: string | null): string {
  const value = String(profile || '').trim()
  return value ? humanizeEnum(value) : tr('harness.group.unprofiled', 'Unprofiled item')
}

function verificationStatusLabel(diagnostics?: ScorecardVerificationDiagnostics | null): string {
  if (diagnostics?.passed === true) return statusLabel('passed')
  if (diagnostics?.passed === false) return statusLabel('failed')
  return tr('common.notAvailable', 'Not available')
}

function remediationHint(label?: string | null): string {
  const normalized = String(label || '').trim()
  if (!normalized) return ''
  return harnessFailureLabelHint(normalized, tr)
}

function verificationTraceHasContent(summary?: VerificationTraceSummary | null): boolean {
  return Boolean(
    summary &&
    (summary.eventCount != null ||
      summary.artifactCount != null ||
      summary.toolNames.length > 0 ||
      summary.eventsError ||
      summary.artifactsError)
  )
}

void verificationTraceHasContent

function contractStringList(values?: string[] | null): string[] {
  return Array.isArray(values)
    ? values.map((value) => String(value || '').trim()).filter((value) => value.length > 0)
    : []
}

function contractArtifacts(contract?: HarnessContract | null): string[] {
  return (contract?.expected_artifacts || [])
    .map((artifact) => firstNonEmpty(String(artifact?.path || ''), String(artifact?.label || '')))
    .filter((value) => value.length > 0)
}

function contractBrowserChecks(contract?: HarnessContract | null): string[] {
  return (contract?.browser_checks || [])
    .map((check) =>
      firstNonEmpty(
        String(check?.name || ''),
        String(check?.target || ''),
        String(check?.required_observation || ''),
        String(check?.required_artifact || '')
      )
    )
    .filter((value) => value.length > 0)
}

function contractAPIChecks(contract?: HarnessContract | null): string[] {
  return (contract?.api_checks || [])
    .map((check) =>
      firstNonEmpty(
        String(check?.name || ''),
        String(check?.target || ''),
        String(check?.required_check || '')
      )
    )
    .filter((value) => value.length > 0)
}

function checkpointPayload(
  record?: HarnessCheckpointArtifact | null
): Record<string, unknown> | null {
  return asRecord(record?.payload || null)
}

function checkpointStringList(record?: HarnessCheckpointArtifact | null, key?: string): string[] {
  if (!key) return []
  return readStringList([checkpointPayload(record)], key)
}

function runPreview(run?: HarnessRunSummary | null): string {
  if (!run) return ''
  const text = String(run.result || run.error || '').trim()
  if (!text) return ''
  return text.length > 180 ? `${text.slice(0, 177)}...` : text
}

function scorecardReason(scorecard?: HarnessScorecard | null): string {
  const breakdown = safeJSON(scorecard?.breakdown_json)
  if (typeof breakdown?.reason === 'string' && breakdown.reason.trim()) {
    return breakdown.reason.trim()
  }
  const trace = safeJSON(scorecard?.judge_trace_json)
  if (typeof trace?.reason === 'string' && trace.reason.trim()) {
    return trace.reason.trim()
  }
  return ''
}

function artifactIsURL(artifact: HarnessArtifactRef): boolean {
  return /^(https?:)?\/\//.test(String(artifact.path_or_url || '').trim())
}

async function loadReport(options: { silent?: boolean } = {}) {
  if (!groupID.value) return
  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  error.value = ''
  try {
    const response = await harnessApi.getGroupReport(groupID.value)
    report.value = response.data
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

async function cancelGroup() {
  if (!group.value || !groupID.value) return
  actionLoading.value = 'cancel'
  try {
    await harnessApi.cancelGroup(groupID.value)
    notification.info(
      tr('nav.harness', 'Harness'),
      tr('harness.group.cancelled', 'Group cancelled')
    )
    await loadReport()
  } catch (err) {
    notification.error(tr('common.error', 'Error'), getErrorMessage(err))
  } finally {
    actionLoading.value = ''
  }
}

async function retryFailed() {
  if (!group.value || !groupID.value) return
  actionLoading.value = 'retry'
  try {
    const response = await harnessApi.retryFailedGroup(groupID.value)
    notification.success(
      tr('nav.harness', 'Harness'),
      tr('harness.group.retryResult', `Requeued ${response.data?.retried || 0} failed items.`)
    )
    await loadReport()
  } catch (err) {
    notification.error(tr('common.error', 'Error'), getErrorMessage(err))
  } finally {
    actionLoading.value = ''
  }
}

function syncPromotionDefaults() {
  if (!group.value || !isEphemeralQuickEval.value) return
  const metadata = asRecord(group.value.metadata)
  const baseName = firstNonEmpty(
    String(metadata?.quick_eval_dataset_name || ''),
    group.value.title,
    group.value.subject,
    group.value.id
  )
  if (!promotionForm.value.datasetName.trim()) {
    promotionForm.value.datasetName = baseName
  }
  if (!promotionForm.value.subject.trim()) {
    promotionForm.value.subject = firstNonEmpty(
      String(metadata?.quick_eval_subject || ''),
      group.value.subject
    )
  }
  if (!promotionForm.value.evalName.trim()) {
    promotionForm.value.evalName = firstNonEmpty(
      String(metadata?.quick_eval_eval_name || ''),
      `${baseName} Eval`
    )
  }
}

async function promoteGroup() {
  if (!groupID.value || !isEphemeralQuickEval.value) return
  if (!promotionForm.value.datasetName.trim() || !promotionForm.value.evalName.trim()) {
    notification.error(
      tr('common.error', 'Error'),
      tr(
        'harness.group.promotionRequiredFields',
        'Dataset name and eval name are required before promotion.'
      )
    )
    return
  }
  promotionLoading.value = true
  try {
    promotionResult.value = null
    const payload: HarnessPromoteGroupSpec = {
      dataset_name: promotionForm.value.datasetName.trim(),
      description: promotionForm.value.description.trim(),
      subject: promotionForm.value.subject.trim(),
      eval_name: promotionForm.value.evalName.trim(),
    }
    const response = await harnessApi.promoteGroup(groupID.value, payload)
    promotionResult.value = response.data || null
    notification.success(
      tr('nav.harness', 'Harness'),
      tr('harness.group.promoted', 'Group promoted into reusable eval assets')
    )
  } catch (err) {
    notification.error(tr('common.error', 'Error'), getErrorMessage(err))
  } finally {
    promotionLoading.value = false
  }
}

function syncAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  if (!group.value || !activeStatuses.has(group.value.status)) return
  refreshTimer = setInterval(() => {
    void loadReport({ silent: true })
  }, 4000)
}

function jumpToRun(runID?: string) {
  if (!runID) return
  const target = document.getElementById(`harness-run-${runID}`)
  target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

watch(groupID, () => {
  void loadReport()
})

watch(
  () => group.value?.status,
  () => {
    syncAutoRefresh()
  }
)

watch(
  () => group.value?.id,
  () => {
    syncPromotionDefaults()
  }
)

onMounted(() => {
  void loadReport()
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})
</script>

<template>
  <div class="harness-detail-page">
    <header class="detail-hero">
      <div class="hero-main">
        <RouterLink class="back-link" :to="{ name: 'Security', query: { tab: 'harness' } }">
          {{ tr('common.back', 'Back') }}
        </RouterLink>
        <div class="hero-heading">
          <span class="kind-chip" :class="group ? `is-${group.kind}` : 'is-eval'">
            {{ kindLabel(group?.kind) }}
          </span>
          <span class="status-chip" :class="statusTone(group?.status || '')">
            {{ statusLabel(group?.status) }}
          </span>
        </div>
        <h1>{{ group?.title || group?.subject || groupID }}</h1>
        <p class="hero-description">
          {{
            group?.subject ||
            tr('harness.group.noSubject', 'This group does not include a subject line.')
          }}
        </p>
        <dl class="hero-meta">
          <div>
            <dt>{{ tr('harness.groups.owner', 'Owner') }}</dt>
            <dd>{{ group?.owner_user_id || tr('common.notAvailable', 'Not available') }}</dd>
          </div>
          <div>
            <dt>{{ tr('common.updatedAt', 'Updated') }}</dt>
            <dd>{{ formatDate(group?.updated_at) }}</dd>
          </div>
          <div>
            <dt>{{ tr('harness.groups.startedAt', 'Started') }}</dt>
            <dd>{{ formatDate(group?.started_at) }}</dd>
          </div>
          <div>
            <dt>{{ tr('harness.groups.finishedAt', 'Finished') }}</dt>
            <dd>{{ formatDate(group?.finished_at) }}</dd>
          </div>
        </dl>
      </div>
      <div class="hero-actions">
        <button
          type="button"
          class="ghost-button"
          :disabled="loading || refreshing"
          @click="loadReport()"
        >
          {{ refreshing ? tr('common.loading', 'Loading') : tr('common.refresh', 'Refresh') }}
        </button>
        <button
          type="button"
          class="warning-button"
          :disabled="!group || hasTerminalGroup || actionLoading !== ''"
          @click="cancelGroup"
        >
          {{
            actionLoading === 'cancel'
              ? tr('common.loading', 'Loading')
              : tr('common.cancel', 'Cancel')
          }}
        </button>
        <button
          type="button"
          class="primary-button"
          :disabled="!group || actionLoading !== ''"
          @click="retryFailed"
        >
          {{
            actionLoading === 'retry'
              ? tr('common.loading', 'Loading')
              : tr('harness.group.retryFailed', 'Retry failed')
          }}
        </button>
      </div>
    </header>

    <div v-if="error" class="state-card is-error">
      <h2>{{ tr('common.error', 'Error') }}</h2>
      <p>{{ error }}</p>
    </div>

    <div v-else-if="loading && !report" class="state-card">
      <h2>{{ tr('common.loading', 'Loading') }}</h2>
      <p>{{ tr('harness.group.loading', 'Loading the latest group report and linked runs.') }}</p>
    </div>

    <template v-else-if="group">
      <section class="stats-grid">
        <article class="stat-card">
          <span>{{ tr('harness.groups.itemCount', 'Items') }}</span>
          <strong>{{ items.length }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ tr('harness.groups.passRate', 'Pass rate') }}</span>
          <strong>{{ percentLabel(report?.pass_rate) }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ tr('harness.groups.score', 'Score') }}</span>
          <strong>{{ scoreLabel(report?.overall_score) }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ tr('harness.group.totalAttempts', 'Attempts') }}</span>
          <strong>{{ totalAttempts }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ tr('harness.group.verificationPassRate', 'Verification pass rate') }}</span>
          <strong>{{ optionalPercentLabel(summaryValue('verification_pass_rate')) }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ tr('harness.group.evidenceBackedPassRate', 'Evidence-backed pass rate') }}</span>
          <strong>{{ optionalPercentLabel(summaryValue('evidence_backed_pass_rate')) }}</strong>
        </article>
        <article class="stat-card">
          <span>{{ tr('harness.group.retryRecovered', 'Retry recovered') }}</span>
          <strong>{{ summaryNumber('retry_recovered_count') }}</strong>
        </article>
      </section>

      <section v-if="isEphemeralQuickEval" class="panel promotion-panel">
        <div class="panel-header">
          <div>
            <h2>{{ tr('harness.group.promoteTitle', 'Promote to regression assets') }}</h2>
            <p class="panel-caption">
              {{
                tr(
                  'harness.group.promoteHint',
                  'Turn this ephemeral quick eval into a reusable dataset, snapshot, and eval spec for later reruns.'
                )
              }}
            </p>
          </div>
          <span class="status-chip is-warning">
            {{ tr('harness.group.ephemeralQuickEval', 'Ephemeral quick eval') }}
          </span>
        </div>

        <form class="promotion-form" @submit.prevent="promoteGroup">
          <label>
            <span>{{ tr('harness.dataset.name', 'Dataset name') }}</span>
            <input v-model="promotionForm.datasetName" name="promotion-dataset-name" required />
          </label>
          <label>
            <span>{{ tr('harness.dataset.subject', 'Subject') }}</span>
            <input v-model="promotionForm.subject" name="promotion-subject" />
          </label>
          <label class="promotion-form-span-2">
            <span>{{ tr('common.description', 'Description') }}</span>
            <textarea v-model="promotionForm.description" name="promotion-description" rows="3" />
          </label>
          <label class="promotion-form-span-2">
            <span>{{ tr('harness.evalSpec.name', 'Eval name') }}</span>
            <input v-model="promotionForm.evalName" name="promotion-eval-name" required />
          </label>
          <div class="promotion-actions promotion-form-span-2">
            <button type="submit" class="primary-button" :disabled="promotionLoading">
              {{
                promotionLoading
                  ? tr('common.loading', 'Loading')
                  : tr('harness.group.promoteAction', 'Promote this quick eval')
              }}
            </button>
          </div>
        </form>

        <article v-if="promotionResult" class="promotion-result-card">
          <strong>{{
            tr('harness.group.promoted', 'Group promoted into reusable eval assets')
          }}</strong>
          <div class="detail-pills">
            <span v-if="promotionResult.dataset">
              {{ tr('harness.dataset.create', 'Dataset') }}:
              {{ promotionResult.dataset.name || promotionResult.dataset.id }}
            </span>
            <span v-if="promotionResult.dataset_version">
              {{ tr('harness.dataset.publishVersion', 'Version') }}:
              {{ promotionResult.dataset_version.version || promotionResult.dataset_version.id }}
            </span>
            <span v-if="promotionResult.eval_spec">
              {{ tr('harness.evalSpec.create', 'Eval spec') }}:
              {{ promotionResult.eval_spec.name || promotionResult.eval_spec.id }}
            </span>
          </div>
        </article>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.scoreDistribution', 'Score distribution') }}</h2>
          <span class="panel-caption">
            {{ tr('harness.group.scoringMode', 'Scoring mode') }}:
            {{ modeLabel(group.scoring_config?.mode || 'hybrid') }}
          </span>
        </div>
        <div class="verdict-grid">
          <div class="verdict-card">
            <span>{{ tr('harness.group.passVerdict', 'Pass') }}</span>
            <strong>{{ verdictCounts.pass || 0 }}</strong>
          </div>
          <div class="verdict-card">
            <span>{{ tr('harness.group.failVerdict', 'Fail') }}</span>
            <strong>{{ verdictCounts.fail || 0 }}</strong>
          </div>
          <div class="verdict-card">
            <span>{{ tr('harness.group.partialVerdict', 'Partial') }}</span>
            <strong>{{ verdictCounts.partial || 0 }}</strong>
          </div>
          <div class="verdict-card">
            <span>{{ tr('harness.group.errorVerdict', 'Error') }}</span>
            <strong>{{ verdictCounts.error || 0 }}</strong>
          </div>
        </div>
        <div class="summary-row">
          <span
            >{{ tr('harness.group.queuedCount', 'Queued') }}:
            {{ summaryCount('queued') + summaryCount('pending') }}</span
          >
          <span
            >{{ tr('harness.group.runningCount', 'Running') }}: {{ summaryCount('running') }}</span
          >
          <span
            >{{ tr('harness.group.failedCount', 'Failed') }}:
            {{ summaryCount('failed') + summaryCount('error') }}</span
          >
        </div>
        <div v-if="failureLabelEntries.length" class="detail-pills summary-pills">
          <span v-for="entry in failureLabelEntries" :key="`failure-label-${entry.key}`">
            {{ tr('harness.group.failureLabel', 'Failure label') }}: {{ entry.key }} ·
            {{ entry.value }}
          </span>
        </div>
        <div v-if="failureLabelHintEntries.length" class="remediation-list">
          <article
            v-for="entry in failureLabelHintEntries"
            :key="`failure-hint-${entry.key}`"
            class="remediation-card"
          >
            <strong>{{ entry.key }}</strong>
            <p>
              {{ tr('harness.group.remediation', 'Remediation') }}:
              {{ entry.hint }}
            </p>
          </article>
        </div>
        <p v-else class="panel-caption summary-caption">
          {{ tr('harness.group.noFailureLabels', 'No failure labels recorded.') }}
        </p>
      </section>

      <section v-if="scorecards.length" class="panel">
        <div class="panel-header">
          <div>
            <h2>{{ tr('harness.group.scorecardInsights', 'Calibration & proposal summary') }}</h2>
            <p class="panel-caption">
              {{
                tr(
                  'harness.group.scorecardInsightsHint',
                  'Harness surfaces scoring diagnostics here; proposal review still happens in Memory / Self-evolution.'
                )
              }}
            </p>
          </div>
          <RouterLink class="inline-link" :to="{ name: 'Settings', query: { tab: 'userdata' } }">
            {{ tr('harness.group.reviewProposals', 'Review in Memory') }}
          </RouterLink>
        </div>

        <div v-if="!hasScorecardDiagnostics" class="empty-state">
          {{
            tr(
              'harness.group.noScorecardInsights',
              'No calibration or proposal diagnostics were attached to the current scorecards.'
            )
          }}
        </div>

        <div v-else class="table-like">
          <article v-for="row in scorecardDiagnostics" :key="row.id" class="table-row">
            <div class="row-primary">
              <div class="row-title-line">
                <strong>#{{ row.itemIndex ?? '?' }}</strong>
                <span class="status-chip" :class="statusTone(row.verdict)">{{
                  statusLabel(row.verdict)
                }}</span>
                <span v-if="row.judgeBackend" class="profile-chip">{{ row.judgeBackend }}</span>
              </div>
              <p class="row-subtitle">
                {{ tr('harness.groups.score', 'Score') }} {{ scoreLabel(row.score) }}
              </p>
              <p v-if="row.verification.summary" class="row-subtitle">
                {{ row.verification.summary }}
              </p>
              <p v-if="row.verification.failureLabel" class="failed-reason">
                {{ tr('harness.group.remediation', 'Remediation') }}:
                {{ remediationHint(row.verification.failureLabel) }}
              </p>
              <div class="detail-pills">
                <span v-if="row.verification.passed != null">
                  {{ tr('harness.group.verification', 'Verification') }}:
                  {{ verificationStatusLabel(row.verification) }}
                </span>
                <span v-if="row.verification.failureLabel">
                  {{ tr('harness.group.failureLabel', 'Failure label') }}:
                  {{ row.verification.failureLabel }}
                </span>
                <span v-if="row.verification.retryable != null">
                  {{ tr('harness.group.retryable', 'Retryable') }}:
                  {{ boolLabel(row.verification.retryable) }}
                </span>
                <span v-if="row.verification.outcomeScore != null">
                  {{ tr('harness.group.outcomeScore', 'Outcome score') }}:
                  {{ scoreLabel(row.verification.outcomeScore) }}
                </span>
                <span v-if="row.verification.evidenceScore != null">
                  {{ tr('harness.group.evidenceScore', 'Evidence score') }}:
                  {{ scoreLabel(row.verification.evidenceScore) }}
                </span>
                <span v-if="row.verification.executionScore != null">
                  {{ tr('harness.group.executionScore', 'Execution score') }}:
                  {{ scoreLabel(row.verification.executionScore) }}
                </span>
                <span v-if="row.verification.observations.length">
                  {{ tr('harness.group.observations', 'Observations') }}:
                  {{ row.verification.observations.join(', ') }}
                </span>
                <span v-if="row.judgeModel">
                  {{ tr('harness.evalSpec.judgeModel', 'Judge model') }}: {{ row.judgeModel }}
                </span>
                <span v-if="row.calibrationRef">
                  {{ tr('harness.group.calibrationRef', 'Calibration ref') }}:
                  {{ row.calibrationRef }}
                </span>
                <span v-if="row.takeawayCandidateCount != null">
                  {{ tr('harness.group.takeawayCandidateCount', 'Takeaway candidates') }}:
                  {{ row.takeawayCandidateCount }}
                </span>
                <span v-if="row.proposalCount != null">
                  {{ tr('harness.group.proposalCount', 'Proposal count') }}:
                  {{ row.proposalCount }}
                </span>
              </div>
              <div
                v-if="
                  row.verification.observations.length ||
                  row.verification.checks.length ||
                  row.verification.artifacts.length ||
                  verificationTraceHasContent(row.verification.traceSummary)
                "
                class="contract-stack"
              >
                <section v-if="row.verification.observations.length" class="contract-section">
                  <div class="contract-header">
                    <strong>{{ tr('harness.group.observations', 'Observations') }}</strong>
                    <span class="panel-caption">{{ row.verification.observations.length }}</span>
                  </div>
                  <div class="detail-pills contract-pills">
                    <span
                      v-for="observation in row.verification.observations"
                      :key="`observation-${row.id}-${observation}`"
                    >
                      {{ observation }}
                    </span>
                  </div>
                </section>

                <section v-if="row.verification.checks.length" class="contract-section">
                  <div class="contract-header">
                    <strong>{{
                      tr('harness.group.verificationChecks', 'Verification checks')
                    }}</strong>
                    <span class="panel-caption">{{ row.verification.checks.length }}</span>
                  </div>
                  <div class="contract-grid">
                    <article
                      v-for="check in row.verification.checks"
                      :key="`${check.name}-${check.expected}-${check.actual}`"
                      class="contract-card"
                      :class="{ 'is-failed': !check.passed }"
                    >
                      <div class="contract-title-line">
                        <strong>{{ check.name }}</strong>
                        <span
                          class="status-chip"
                          :class="statusTone(check.passed ? 'passed' : 'failed')"
                        >
                          {{ statusLabel(check.passed ? 'passed' : 'failed') }}
                        </span>
                      </div>
                      <dl class="contract-values">
                        <div>
                          <dt>{{ tr('harness.group.expectedValue', 'Expected') }}</dt>
                          <dd>{{ check.expected }}</dd>
                        </div>
                        <div>
                          <dt>{{ tr('harness.group.actualValue', 'Actual') }}</dt>
                          <dd>{{ check.actual }}</dd>
                        </div>
                      </dl>
                    </article>
                  </div>
                </section>

                <section v-if="row.verification.artifacts.length" class="contract-section">
                  <div class="contract-header">
                    <strong>{{
                      tr('harness.group.expectedArtifacts', 'Expected artifacts')
                    }}</strong>
                    <span class="panel-caption">{{ row.verification.artifacts.length }}</span>
                  </div>
                  <div class="contract-grid">
                    <article
                      v-for="artifact in row.verification.artifacts"
                      :key="`${artifact.target}-${artifact.actual}`"
                      class="contract-card"
                      :class="{ 'is-failed': !artifact.passed }"
                    >
                      <div class="contract-title-line">
                        <strong>{{ artifact.target }}</strong>
                        <span
                          class="status-chip"
                          :class="statusTone(artifact.passed ? 'passed' : 'failed')"
                        >
                          {{ statusLabel(artifact.passed ? 'passed' : 'failed') }}
                        </span>
                      </div>
                      <dl class="contract-values">
                        <div>
                          <dt>{{ tr('harness.group.expectedValue', 'Expected') }}</dt>
                          <dd>{{ artifact.target }}</dd>
                        </div>
                        <div>
                          <dt>{{ tr('harness.group.actualValue', 'Actual') }}</dt>
                          <dd>{{ artifact.actual }}</dd>
                        </div>
                      </dl>
                    </article>
                  </div>
                </section>

                <section
                  v-if="verificationTraceHasContent(row.verification.traceSummary)"
                  class="contract-section"
                >
                  <div class="contract-header">
                    <strong>{{ tr('harness.group.traceSummary', 'Trace summary') }}</strong>
                  </div>
                  <div class="detail-pills contract-pills">
                    <span v-if="row.verification.traceSummary.eventCount != null">
                      {{ tr('harness.group.eventCount', 'Event count') }}:
                      {{ row.verification.traceSummary.eventCount }}
                    </span>
                    <span v-if="row.verification.traceSummary.artifactCount != null">
                      {{ tr('harness.group.artifactCount', 'Artifact count') }}:
                      {{ row.verification.traceSummary.artifactCount }}
                    </span>
                    <span v-if="row.verification.traceSummary.toolNames.length">
                      {{ tr('harness.group.observedTools', 'Observed tools') }}:
                      {{ row.verification.traceSummary.toolNames.join(', ') }}
                    </span>
                  </div>
                  <p v-if="row.verification.traceSummary.eventsError" class="failed-reason">
                    {{ tr('common.error', 'Error') }}:
                    {{ row.verification.traceSummary.eventsError }}
                  </p>
                  <p v-if="row.verification.traceSummary.artifactsError" class="failed-reason">
                    {{ tr('common.error', 'Error') }}:
                    {{ row.verification.traceSummary.artifactsError }}
                  </p>
                </section>
              </div>
              <p v-if="row.proposalSkippedReason" class="failed-reason">
                {{ row.proposalSkippedReason }}
              </p>
            </div>
            <div class="row-metrics">
              <span v-if="row.proposalIDs.length">
                {{ tr('harness.group.proposalIds', 'Proposal IDs') }}:
              </span>
              <div v-if="row.proposalIDs.length" class="id-stack">
                <code v-for="proposalID in row.proposalIDs" :key="proposalID" class="run-id">
                  {{ proposalID }}
                </code>
              </div>
              <span v-else class="panel-caption">
                {{ tr('harness.group.noProposalIds', 'No review proposals attached') }}
              </span>
            </div>
          </article>
        </div>
      </section>

      <section v-if="contractRows.length" class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.contracts', 'Run contracts') }}</h2>
          <span class="panel-caption">{{ contractRows.length }}</span>
        </div>
        <div class="table-like">
          <article v-for="row in contractRows" :key="row.itemID" class="table-row">
            <div class="row-primary">
              <div class="row-title-line">
                <strong>#{{ row.itemIndex ?? '?' }}</strong>
                <span v-if="row.profile" class="profile-chip">{{ profileLabel(row.profile) }}</span>
                <span v-if="row.contract.risk_level" class="status-chip is-warning">
                  {{ tr('harness.group.riskLevel', 'Risk') }}: {{ row.contract.risk_level }}
                </span>
              </div>
              <div class="detail-pills">
                <span
                  v-for="deliverable in contractStringList(row.contract.deliverables)"
                  :key="`deliverable-${row.itemID}-${deliverable}`"
                >
                  {{ tr('harness.group.deliverables', 'Deliverable') }}: {{ deliverable }}
                </span>
                <span
                  v-for="artifact in contractArtifacts(row.contract)"
                  :key="`artifact-${row.itemID}-${artifact}`"
                >
                  {{ tr('harness.group.expectedArtifacts', 'Expected artifact') }}: {{ artifact }}
                </span>
                <span
                  v-for="toolName in contractStringList(row.contract.required_tool_calls)"
                  :key="`tool-${row.itemID}-${toolName}`"
                >
                  {{ tr('harness.group.requiredToolCalls', 'Required tool') }}: {{ toolName }}
                </span>
                <span
                  v-for="observation in contractStringList(row.contract.required_observations)"
                  :key="`observation-${row.itemID}-${observation}`"
                >
                  {{ tr('harness.group.requiredObservations', 'Required observation') }}:
                  {{ observation }}
                </span>
                <span
                  v-for="browserCheck in contractBrowserChecks(row.contract)"
                  :key="`browser-${row.itemID}-${browserCheck}`"
                >
                  {{ tr('harness.group.browserQa', 'Browser QA') }}: {{ browserCheck }}
                </span>
                <span
                  v-for="apiCheck in contractAPIChecks(row.contract)"
                  :key="`api-${row.itemID}-${apiCheck}`"
                >
                  {{ tr('harness.group.apiChecks', 'API check') }}: {{ apiCheck }}
                </span>
              </div>
              <p v-if="contractStringList(row.contract.fallback_order).length" class="row-subtitle">
                {{ tr('harness.group.fallbackOrder', 'Fallback order') }}:
                {{ contractStringList(row.contract.fallback_order).join(' -> ') }}
              </p>
              <p
                v-if="contractStringList(row.contract.stop_conditions).length"
                class="row-subtitle"
              >
                {{ tr('harness.group.stopConditions', 'Stop conditions') }}:
                {{ contractStringList(row.contract.stop_conditions).join(', ') }}
              </p>
            </div>
          </article>
        </div>
      </section>

      <section v-if="checkpoints.length" class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.checkpoints', 'Checkpoints') }}</h2>
          <span class="panel-caption">{{ checkpoints.length }}</span>
        </div>
        <div class="failed-grid">
          <article
            v-for="checkpoint in checkpoints"
            :key="checkpoint.artifact.id"
            class="failed-card"
          >
            <div class="failed-header">
              <strong>{{
                checkpoint.artifact.label || tr('harness.group.checkpoint', 'Checkpoint')
              }}</strong>
              <span class="status-chip is-warning">
                {{ tr('harness.group.attemptIndex', 'Attempt') }}
                {{ Number(checkpointPayload(checkpoint)?.attempt_index || 0) }}
              </span>
            </div>
            <p class="failed-title">
              {{
                String(checkpointPayload(checkpoint)?.summary || '').trim() ||
                tr('harness.group.noCheckpointSummary', 'No checkpoint summary recorded.')
              }}
            </p>
            <div class="detail-pills">
              <span
                v-for="label in checkpointStringList(checkpoint, 'failure_labels')"
                :key="`checkpoint-label-${checkpoint.artifact.id}-${label}`"
              >
                {{ tr('harness.group.failureLabel', 'Failure label') }}: {{ label }}
              </span>
              <span
                v-for="evidence in checkpointStringList(checkpoint, 'verified_evidence')"
                :key="`checkpoint-evidence-${checkpoint.artifact.id}-${evidence}`"
              >
                {{ tr('harness.group.evidence', 'Evidence') }}: {{ evidence }}
              </span>
            </div>
            <p
              v-if="checkpointStringList(checkpoint, 'unresolved_risks').length"
              class="failed-reason"
            >
              {{ tr('harness.group.unresolvedRisks', 'Unresolved risks') }}:
              {{ checkpointStringList(checkpoint, 'unresolved_risks').join(', ') }}
            </p>
            <p
              v-if="String(checkpointPayload(checkpoint)?.recommended_resume || '').trim()"
              class="row-subtitle"
            >
              {{ String(checkpointPayload(checkpoint)?.recommended_resume || '').trim() }}
            </p>
            <code class="artifact-path">{{
              checkpoint.artifact.path_or_url || tr('common.notAvailable', 'Not available')
            }}</code>
          </article>
        </div>
      </section>

      <section v-if="runtimeEvidenceRows.length" class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.runtimeEvidence', 'Runtime evidence') }}</h2>
          <span class="panel-caption">{{ runtimeEvidenceRows.length }}</span>
        </div>
        <div class="run-list">
          <article
            v-for="row in runtimeEvidenceRows"
            :key="row.run?.id || row.entries[0]?.run_id"
            class="run-card"
          >
            <div class="run-header">
              <div>
                <div class="run-title-line">
                  <strong>{{
                    row.run?.goal || row.run?.id || tr('harness.group.linkedRuns', 'Linked run')
                  }}</strong>
                  <span class="status-chip" :class="statusTone(row.run?.status || 'completed')">
                    {{ statusLabel(row.run?.status || 'completed') }}
                  </span>
                </div>
                <p class="run-meta">
                  {{ tr('harness.group.eventCount', 'Event count') }}: {{ row.entries.length }}
                </p>
              </div>
              <code class="run-id">{{ row.run?.id || row.entries[0]?.run_id }}</code>
            </div>
            <div class="runtime-evidence-list">
              <article v-for="entry in row.entries" :key="entry.id" class="evidence-entry">
                <div class="row-title-line">
                  <strong>{{ humanizeEnum(entry.event_type) }}</strong>
                  <span class="panel-caption">
                    {{
                      tr('harness.group.stepLabel', 'Step') + ' ' + Number(entry.step_index || 0)
                    }}
                    ·
                    {{
                      tr('harness.group.plannerRound', 'Round') +
                      ' ' +
                      Number(entry.planner_round || 0)
                    }}
                  </span>
                </div>
                <p class="row-subtitle">
                  {{ entry.summary || tr('common.notAvailable', 'Not available') }}
                </p>
                <p class="run-meta">{{ formatDate(entry.created_at) }}</p>
              </article>
            </div>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.items', 'Items') }}</h2>
          <span class="panel-caption">
            {{ tr('harness.group.linkedRuns', 'Linked runs') }}: {{ linkedRuns.length }}
          </span>
        </div>

        <div class="table-like">
          <article v-for="item in items" :key="item.id" class="table-row">
            <div class="row-primary">
              <div class="row-title-line">
                <strong>#{{ item.index }}</strong>
                <span class="status-chip" :class="statusTone(item.status)">{{
                  statusLabel(item.status)
                }}</span>
                <span v-if="item.profile" class="profile-chip">{{
                  profileLabel(item.profile)
                }}</span>
              </div>
              <p class="row-subtitle">
                {{
                  String(
                    item.input?.goal || item.input?.query || item.input?.prompt || ''
                  ).trim() ||
                  tr('harness.group.noInputSummary', 'No goal/query recorded for this item.')
                }}
              </p>
            </div>
            <div class="row-metrics">
              <span
                >{{ tr('harness.group.attempts', 'Attempts') }}: {{ item.attempt_count }}/{{
                  item.max_attempts || 1
                }}</span
              >
              <span>
                {{ tr('harness.group.scorecard', 'Verdict') }}:
                {{
                  scorecardByItemID[item.id]?.verdict
                    ? statusLabel(scorecardByItemID[item.id]?.verdict)
                    : tr('common.notAvailable', 'Not available')
                }}
              </span>
              <span v-if="scorecardDiagnosticsByItemID[item.id]?.verification.passed != null">
                {{ tr('harness.group.verification', 'Verification') }}:
                {{ verificationStatusLabel(scorecardDiagnosticsByItemID[item.id]?.verification) }}
              </span>
              <span v-if="scorecardDiagnosticsByItemID[item.id]?.verification.failureLabel">
                {{ tr('harness.group.failureLabel', 'Failure label') }}:
                {{ scorecardDiagnosticsByItemID[item.id]?.verification.failureLabel }}
              </span>
              <span v-if="scorecardDiagnosticsByItemID[item.id]?.verification.failureLabel">
                {{ tr('harness.group.remediation', 'Remediation') }}:
                {{
                  remediationHint(scorecardDiagnosticsByItemID[item.id]?.verification.failureLabel)
                }}
              </span>
              <span v-if="scorecardDiagnosticsByItemID[item.id]?.verification.observations.length">
                {{ tr('harness.group.observations', 'Observations') }}:
                {{ scorecardDiagnosticsByItemID[item.id]?.verification.observations.join(', ') }}
              </span>
              <button
                v-if="item.latest_run_id"
                type="button"
                class="inline-link"
                @click="jumpToRun(item.latest_run_id)"
              >
                {{ tr('harness.group.jumpToRun', 'Jump to run') }}
              </button>
            </div>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.failedItems', 'Failed items') }}</h2>
          <span class="panel-caption">{{ failedItems.length }}</span>
        </div>
        <div v-if="failedItems.length === 0" class="empty-state">
          {{ tr('harness.group.noFailedItems', 'No failed items in the latest report.') }}
        </div>
        <div v-else class="failed-grid">
          <article
            v-for="entry in failedItems"
            :key="entry.item?.id || entry.run?.id"
            class="failed-card"
          >
            <div class="failed-header">
              <strong>#{{ entry.item?.index ?? '?' }}</strong>
              <span
                class="status-chip"
                :class="statusTone(entry.scorecard?.verdict || entry.item?.status || '')"
              >
                {{ statusLabel(entry.scorecard?.verdict || entry.item?.status || 'unknown') }}
              </span>
            </div>
            <p class="failed-title">{{ profileLabel(entry.item?.profile) }}</p>
            <div
              v-if="
                entry.verification.passed != null ||
                entry.verification.failureLabel ||
                entry.verification.evidenceScore != null
              "
              class="detail-pills"
            >
              <span v-if="entry.verification.passed != null">
                {{ tr('harness.group.verification', 'Verification') }}:
                {{ verificationStatusLabel(entry.verification) }}
              </span>
              <span v-if="entry.verification.failureLabel">
                {{ tr('harness.group.failureLabel', 'Failure label') }}:
                {{ entry.verification.failureLabel }}
              </span>
              <span v-if="entry.verification.evidenceScore != null">
                {{ tr('harness.group.evidenceScore', 'Evidence score') }}:
                {{ scoreLabel(entry.verification.evidenceScore) }}
              </span>
              <span v-if="entry.verification.observations.length">
                {{ tr('harness.group.observations', 'Observations') }}:
                {{ entry.verification.observations.join(', ') }}
              </span>
            </div>
            <p v-if="entry.verification.failureLabel" class="failed-reason">
              {{ tr('harness.group.remediation', 'Remediation') }}:
              {{ remediationHint(entry.verification.failureLabel) }}
            </p>
            <p class="failed-reason">
              {{
                scorecardReason(entry.scorecard) ||
                runPreview(entry.run) ||
                tr('harness.group.noFailureReason', 'No failure reason recorded.')
              }}
            </p>
            <button
              v-if="entry.run?.id"
              type="button"
              class="inline-link"
              @click="jumpToRun(entry.run?.id)"
            >
              {{ tr('harness.group.inspectRun', 'Inspect run') }}
            </button>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.linkedRuns', 'Linked runs') }}</h2>
          <span class="panel-caption">{{ linkedRuns.length }}</span>
        </div>
        <div v-if="linkedRuns.length === 0" class="empty-state">
          {{
            tr('harness.group.noLinkedRuns', 'No linked runs were persisted for this group yet.')
          }}
        </div>
        <div v-else class="run-list">
          <article
            v-for="run in linkedRuns"
            :id="`harness-run-${run.id}`"
            :key="run.id"
            class="run-card"
          >
            <div class="run-header">
              <div>
                <div class="run-title-line">
                  <strong>{{ run.goal || run.id }}</strong>
                  <span class="status-chip" :class="statusTone(run.status)">{{
                    statusLabel(run.status)
                  }}</span>
                </div>
                <p class="run-meta">
                  {{ kindLabel(run.kind) }} · {{ tr('harness.group.attemptIndex', 'Attempt') }}
                  {{ run.attempt_index || 0 }} ·
                  {{ formatDate(run.updated_at) }}
                </p>
              </div>
              <code class="run-id">{{ run.id }}</code>
            </div>
            <p v-if="runPreview(run)" class="run-preview">{{ runPreview(run) }}</p>
          </article>
        </div>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.artifacts', 'Artifacts') }}</h2>
          <span class="panel-caption">{{ artifacts.length }}</span>
        </div>
        <div v-if="artifacts.length === 0" class="empty-state">
          {{ tr('harness.group.noArtifacts', 'No artifacts attached to the linked runs yet.') }}
        </div>
        <div v-else class="artifact-list">
          <article v-for="artifact in artifacts" :key="artifact.id" class="artifact-card">
            <div>
              <strong>{{ artifact.label || artifactKindLabel(artifact.kind) }}</strong>
              <p class="artifact-meta">
                {{ artifactKindLabel(artifact.kind) }} ·
                {{ artifact.mime_type || tr('common.notAvailable', 'Not available') }}
              </p>
            </div>
            <a
              v-if="artifact.path_or_url && artifactIsURL(artifact)"
              :href="artifact.path_or_url"
              target="_blank"
              rel="noreferrer"
              class="artifact-link"
            >
              {{ artifact.path_or_url }}
            </a>
            <code v-else class="artifact-path">{{
              artifact.path_or_url || tr('common.notAvailable', 'Not available')
            }}</code>
          </article>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.harness-detail-page {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.detail-hero,
.panel,
.state-card,
.stat-card {
  border-radius: 1.2rem;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: #fff;
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.06);
}

.detail-hero {
  display: flex;
  justify-content: space-between;
  gap: 1.5rem;
  padding: 1.4rem;
  background:
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.14), transparent 32%),
    linear-gradient(135deg, rgba(248, 250, 252, 0.95), rgba(241, 245, 249, 0.92));
}

.back-link {
  display: inline-flex;
  margin-bottom: 0.8rem;
  color: #0f766e;
  text-decoration: none;
  font-weight: 600;
}

.hero-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
  margin-bottom: 0.7rem;
}

.detail-hero h1 {
  margin: 0;
  font-size: 1.9rem;
  line-height: 1.1;
  color: #0f172a;
}

.hero-description {
  margin: 0.7rem 0 1rem;
  max-width: 60rem;
  color: #475569;
  line-height: 1.6;
}

.hero-meta {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.8rem;
  margin: 0;
}

.hero-meta dt {
  margin-bottom: 0.2rem;
  color: #64748b;
  font-size: 0.82rem;
}

.hero-meta dd {
  margin: 0;
  color: #0f172a;
  font-weight: 600;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: flex-end;
  gap: 0.75rem;
}

.primary-button,
.warning-button,
.ghost-button,
.inline-link {
  appearance: none;
  border: 0;
  cursor: pointer;
  font: inherit;
}

.primary-button,
.warning-button,
.ghost-button {
  padding: 0.82rem 1.1rem;
  border-radius: 0.9rem;
  font-weight: 700;
}

.primary-button {
  background: #111827;
  color: #fff;
}

.warning-button {
  background: rgba(245, 158, 11, 0.14);
  color: #92400e;
}

.ghost-button {
  background: rgba(148, 163, 184, 0.14);
  color: #334155;
}

.primary-button:disabled,
.warning-button:disabled,
.ghost-button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.stats-grid,
.verdict-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1rem;
}

.stat-card,
.verdict-card {
  padding: 1rem 1.1rem;
}

.stat-card span,
.verdict-card span,
.panel-caption,
.run-meta,
.artifact-meta,
.empty-state,
.failed-reason,
.row-subtitle {
  color: #64748b;
}

.stat-card strong,
.verdict-card strong {
  display: block;
  margin-top: 0.35rem;
  font-size: 1.6rem;
  color: #0f172a;
}

.panel {
  padding: 1.2rem;
}

.runtime-evidence-list {
  display: grid;
  gap: 0.85rem;
}

.evidence-entry {
  padding: 0.85rem 1rem;
  border-radius: 0.95rem;
  background: rgba(241, 245, 249, 0.72);
  border: 1px solid rgba(148, 163, 184, 0.16);
}

.panel-header,
.table-row,
.run-header,
.artifact-card,
.failed-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.panel-header {
  align-items: flex-end;
  margin-bottom: 1rem;
}

.panel-header h2 {
  margin: 0;
  color: #0f172a;
}

.summary-row {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  margin-top: 1rem;
  color: #64748b;
  font-size: 0.92rem;
}

.summary-pills {
  margin-top: 1rem;
}

.summary-caption {
  display: block;
  margin-top: 1rem;
}

.promotion-panel {
  background: linear-gradient(135deg, rgba(255, 251, 235, 0.92), rgba(255, 255, 255, 0.96)), #fff;
}

.promotion-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.9rem;
}

.promotion-form label {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  color: #334155;
  font-weight: 600;
}

.promotion-form input,
.promotion-form textarea {
  width: 100%;
  padding: 0.82rem 0.9rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(148, 163, 184, 0.26);
  background: rgba(255, 255, 255, 0.92);
  color: #0f172a;
  font: inherit;
  box-sizing: border-box;
}

.promotion-form-span-2 {
  grid-column: span 2;
}

.promotion-actions {
  display: flex;
  justify-content: flex-start;
}

.promotion-result-card {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  margin-top: 1rem;
  padding: 1rem 1.05rem;
  border-radius: 1rem;
  background: rgba(240, 253, 244, 0.86);
  border: 1px solid rgba(34, 197, 94, 0.16);
}

.promotion-result-card strong {
  color: #166534;
}

.remediation-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 1rem;
}

.remediation-card {
  padding: 0.9rem 1rem;
  border-radius: 0.95rem;
  background: rgba(236, 253, 245, 0.92);
  border: 1px solid rgba(16, 185, 129, 0.18);
}

.remediation-card strong {
  color: #065f46;
}

.remediation-card p {
  margin: 0.35rem 0 0;
  color: #166534;
  line-height: 1.55;
}

.kind-chip,
.status-chip,
.profile-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.3rem 0.72rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 700;
}

.kind-chip.is-eval {
  background: rgba(14, 165, 233, 0.14);
  color: #0369a1;
}

.kind-chip.is-experiment {
  background: rgba(245, 158, 11, 0.15);
  color: #b45309;
}

.kind-chip.is-batch {
  background: rgba(139, 92, 246, 0.15);
  color: #6d28d9;
}

.status-chip.is-success {
  background: rgba(34, 197, 94, 0.14);
  color: #166534;
}

.status-chip.is-running {
  background: rgba(59, 130, 246, 0.14);
  color: #1d4ed8;
}

.status-chip.is-warning {
  background: rgba(245, 158, 11, 0.15);
  color: #b45309;
}

.status-chip.is-danger {
  background: rgba(239, 68, 68, 0.13);
  color: #b91c1c;
}

.status-chip.is-muted {
  background: rgba(148, 163, 184, 0.16);
  color: #475569;
}

.profile-chip {
  background: rgba(15, 23, 42, 0.06);
  color: #334155;
}

.table-like,
.run-list,
.artifact-list,
.failed-grid {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.table-row,
.run-card,
.artifact-card,
.failed-card {
  padding: 1rem;
  border-radius: 1rem;
  background: rgba(248, 250, 252, 0.88);
  border: 1px solid rgba(148, 163, 184, 0.18);
}

.table-row {
  align-items: flex-start;
}

.row-primary,
.run-card,
.failed-card {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.row-title-line,
.run-title-line {
  display: flex;
  flex-wrap: wrap;
  gap: 0.55rem;
  align-items: center;
}

.row-subtitle,
.run-preview,
.failed-title {
  margin: 0;
  line-height: 1.55;
}

.row-metrics {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.35rem;
  font-size: 0.9rem;
}

.detail-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
  font-size: 0.82rem;
  color: #475569;
}

.detail-pills span {
  display: inline-flex;
  align-items: center;
  padding: 0.3rem 0.65rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
}

.contract-stack {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}

.contract-section {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.9rem 1rem;
  border-radius: 0.95rem;
  background: rgba(255, 255, 255, 0.78);
  border: 1px solid rgba(148, 163, 184, 0.18);
}

.contract-header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: center;
}

.contract-header strong,
.contract-title-line strong {
  color: #0f172a;
}

.contract-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.contract-card {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.9rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(34, 197, 94, 0.14);
  background: rgba(240, 253, 244, 0.72);
}

.contract-card.is-failed {
  border-color: rgba(239, 68, 68, 0.18);
  background: rgba(254, 242, 242, 0.86);
}

.contract-title-line {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: flex-start;
}

.contract-values {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
  margin: 0;
}

.contract-values dt {
  margin-bottom: 0.3rem;
  color: #64748b;
  font-size: 0.8rem;
}

.contract-values dd {
  margin: 0;
  color: #0f172a;
  font-size: 0.88rem;
  line-height: 1.55;
  word-break: break-word;
}

.contract-pills {
  margin-top: -0.1rem;
}

.id-stack {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  align-items: flex-end;
}

.inline-link {
  padding: 0;
  background: transparent;
  color: #0f766e;
  font-weight: 700;
}

.failed-card {
  gap: 0.65rem;
}

.failed-title {
  font-weight: 700;
  color: #0f172a;
}

.run-id,
.artifact-path {
  padding: 0.35rem 0.55rem;
  border-radius: 0.7rem;
  background: rgba(15, 23, 42, 0.06);
  color: #1e293b;
  font-size: 0.84rem;
  word-break: break-all;
}

.artifact-card {
  align-items: center;
}

.artifact-link {
  color: #0369a1;
  text-decoration: none;
  word-break: break-all;
}

.state-card {
  padding: 1.1rem 1.2rem;
}

.state-card.is-error {
  border-color: rgba(239, 68, 68, 0.24);
  background: rgba(254, 242, 242, 0.95);
}

.state-card h2,
.state-card p {
  margin: 0;
}

.state-card h2 {
  margin-bottom: 0.35rem;
}

@media (max-width: 1100px) {
  .detail-hero,
  .table-row,
  .artifact-card {
    flex-direction: column;
  }

  .hero-meta,
  .stats-grid,
  .verdict-grid,
  .promotion-form,
  .contract-grid,
  .contract-values {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .row-metrics {
    align-items: flex-start;
  }

  .id-stack {
    align-items: flex-start;
  }
}

@media (max-width: 720px) {
  .hero-meta,
  .stats-grid,
  .verdict-grid,
  .promotion-form,
  .contract-grid,
  .contract-values {
    grid-template-columns: 1fr;
  }

  .promotion-form-span-2 {
    grid-column: span 1;
  }

  .hero-actions {
    justify-content: stretch;
  }

  .primary-button,
  .warning-button,
  .ghost-button {
    width: 100%;
  }
}
</style>
