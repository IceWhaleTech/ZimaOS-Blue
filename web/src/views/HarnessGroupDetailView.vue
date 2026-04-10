<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import {
  harnessApi,
  type HarnessArtifactRef,
  type HarnessRun,
  type HarnessRunDetail,
  type HarnessRunGroup,
  type HarnessRunGroupItem,
  type HarnessRunGroupReport,
  type HarnessRunGroupStatus,
  type HarnessRunSummary,
} from '@/api/harness'
import AutomationTabs from '@/components/automation/AutomationTabs.vue'
import HarnessRunDetailDrawer from '@/components/harness/HarnessRunDetailDrawer.vue'
import HarnessRunTree from '@/components/harness/HarnessRunTree.vue'
import { useNotificationStore } from '@/stores/notification'
import { getErrorMessage } from '@/utils/error'
import {
  buildHarnessRunTree,
  deriveHarnessRunPreview,
  flattenHarnessRunTree,
  isHarnessRunActiveStatus,
} from '@/utils/harnessRunTree'

type NumberEntry = {
  key: string
  value: number
}

type FailedItemRow = {
  id: string
  itemIndex: number | null
  title: string
  runKind: string
  attempts: string
  status: string
  failureLabel: string
  reason: string
  runID: string
}

type ItemRow = {
  id: string
  index: number | null
  title: string
  status: string
  attempts: string
  runID: string
  runStatus: string
  runKind: string
}

const route = useRoute()
const { t, te } = useI18n()
const notification = useNotificationStore()

const loading = ref(false)
const refreshing = ref(false)
const actionLoading = ref<'cancel' | 'retry' | ''>('')
const error = ref('')
const report = ref<HarnessRunGroupReport | null>(null)

const selectedRunID = ref('')
const runDetailLoading = ref(false)
const runDetailError = ref('')
const runDetailWarning = ref('')
const runDetailCache = ref<Record<string, HarnessRunDetail>>({})
const isNarrowScreen = ref(false)

const terminalStatuses = new Set<HarnessRunGroupStatus>([
  'completed',
  'partial',
  'failed',
  'cancelled',
])

let refreshTimer: ReturnType<typeof setInterval> | null = null
let runDetailRequestToken = 0

const groupID = computed(() => String(route.params.id || '').trim())

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function safeJSON(raw?: string | null): Record<string, unknown> | null {
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

function humanizeEnum(value: string | null | undefined): string {
  const normalized = String(value || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')
  return normalized.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function humanizeStatus(value: string | null | undefined): string {
  const normalized = String(value || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')

  switch (normalized) {
    case 'waiting_input':
      return tr('harness.group.waitingInput', 'Waiting input')
    case 'pending':
    case 'queued':
      return tr('harness.group.queuedCount', 'Queued')
    case 'planning':
      return tr('common.taskRuntimePlan', 'Planning')
    case 'executing':
      return tr('common.taskRuntimeExecute', 'Executing')
    case 'verifying':
      return tr('common.taskRuntimeVerify', 'Verifying')
    case 'completed':
      return tr('common.taskRuntimeDone', 'Completed')
    case 'failed':
      return tr('common.taskStageFailed', 'Failed')
    case 'cancelled':
      return tr('common.taskStageCancelled', 'Cancelled')
    case 'aborted':
      return tr('common.taskRuntimeAborted', 'Aborted')
    case 'partial':
      return tr('harness.group.partialVerdict', 'Partial')
    default:
      return humanizeEnum(normalized)
  }
}

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function percentLabel(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return tr('common.notAvailable', 'Not available')
  return `${Math.round(Number(value) * 100)}%`
}

function scoreLabel(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return tr('common.notAvailable', 'Not available')
  const numeric = Number(value)
  const scaled = numeric >= 0 && numeric <= 1 ? numeric * 100 : numeric
  return `${Math.round(scaled)}%`
}

function statusTone(status?: string | null): string {
  switch (String(status || '').trim()) {
    case 'completed':
    case 'passed':
    case 'pass':
      return 'is-success'
    case 'partial':
      return 'is-warning'
    case 'failed':
    case 'cancelled':
    case 'aborted':
    case 'error':
    case 'fail':
      return 'is-danger'
    case 'running':
    case 'queued':
    case 'pending':
    case 'scoring':
    case 'planning':
    case 'executing':
    case 'verifying':
      return 'is-active'
    case 'waiting_input':
      return 'is-blocked'
    default:
      return 'is-muted'
  }
}

function summaryCount(key: string): number {
  const summary = asRecord(report.value?.group?.summary)
  const counts = asRecord(summary?.counts)
  if (!counts) return 0
  const value = Number(counts[key])
  return Number.isFinite(value) ? value : 0
}

function numberMap(value: unknown): Record<string, number> {
  const source = asRecord(value)
  if (!source) return {}
  const out: Record<string, number> = {}
  for (const [entryKey, raw] of Object.entries(source)) {
    const numeric = Number(raw)
    if (Number.isFinite(numeric) && numeric > 0) out[entryKey] = numeric
  }
  return out
}

function summaryNumberMap(key: string): Record<string, number> {
  const summary = asRecord(report.value?.group?.summary)
  return numberMap(summary?.[key])
}

function sortedEntries(values: Record<string, number>): NumberEntry[] {
  return Object.entries(values)
    .map(([key, value]) => ({ key, value }))
    .sort((left, right) => {
      if (right.value !== left.value) return right.value - left.value
      return left.key.localeCompare(right.key)
    })
}

function syncScreenSize() {
  isNarrowScreen.value = window.innerWidth < 960
}

async function scrollToRunNode(runID: string) {
  await nextTick()
  const element = document.getElementById(`harness-run-${runID}`)
  element?.scrollIntoView({
    block: 'center',
    behavior: 'smooth',
  })
}

async function loadReport(options: { silent?: boolean } = {}): Promise<boolean> {
  if (!groupID.value) return false

  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
    error.value = ''
  }

  try {
    const response = await harnessApi.getGroupReport(groupID.value)
    report.value = response.data
    error.value = ''

    if (
      selectedRunID.value &&
      !(response.data.linked_runs || []).some((run) => run.id === selectedRunID.value)
    ) {
      closeRunDetail()
    }

    return true
  } catch (err) {
    if (!options.silent || !report.value) {
      error.value = getErrorMessage(err)
    }
    return false
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

const group = computed<HarnessRunGroup | null>(() => report.value?.group || null)
const items = computed<HarnessRunGroupItem[]>(() => report.value?.items || [])
const linkedRuns = computed<HarnessRunSummary[]>(() => report.value?.linked_runs || [])
const artifacts = computed<HarnessArtifactRef[]>(() => report.value?.artifacts || [])
const verdictCounts = computed(() => report.value?.verdict_counts || {})
const hasTerminalGroup = computed(() =>
  group.value ? terminalStatuses.has(group.value.status) : false
)
const totalAttempts = computed(() =>
  items.value.reduce((sum, item) => sum + Number(item.attempt_count || 0), 0)
)

const linkedRunMap = computed(() => {
  const map = new Map<string, HarnessRunSummary>()
  for (const run of linkedRuns.value) map.set(run.id, run)
  return map
})

const itemByID = computed(() => {
  const map = new Map<string, HarnessRunGroupItem>()
  for (const item of items.value) map.set(item.id, item)
  return map
})

const runTreeNodes = computed(() => buildHarnessRunTree(linkedRuns.value))
const flattenedRunTreeNodes = computed(() => flattenHarnessRunTree(runTreeNodes.value))
const rootRunTreeNodes = computed(() => runTreeNodes.value.filter((node) => !node.isDetached))
const detachedRunTreeNodes = computed(() => runTreeNodes.value.filter((node) => node.isDetached))
const runTreeNodeByID = computed(() => {
  const map = new Map<string, (typeof flattenedRunTreeNodes.value)[number]>()
  for (const node of flattenedRunTreeNodes.value) {
    map.set(node.run.id, node)
  }
  return map
})

const selectedRunSummary = computed<HarnessRunSummary | null>(() => {
  if (!selectedRunID.value) return null
  return linkedRunMap.value.get(selectedRunID.value) || null
})

const selectedRunDetail = computed<HarnessRunDetail | null>(() => {
  if (!selectedRunID.value) return null
  return runDetailCache.value[selectedRunID.value] || null
})

const selectedRun = computed<HarnessRun | HarnessRunSummary | null>(() => {
  if (!selectedRunDetail.value?.run && !selectedRunSummary.value) return null
  return {
    ...(selectedRunDetail.value?.run || {}),
    ...(selectedRunSummary.value || {}),
  } as HarnessRun
})

const selectedChildRuns = computed(() => {
  if (!selectedRunID.value) return []
  return runTreeNodeByID.value.get(selectedRunID.value)?.children.map((child) => child.run) || []
})

const runEventPreviewMap = computed<Record<string, string>>(() => {
  const map: Record<string, string> = {}
  const runtimeTraces = report.value?.runtime_traces || {}

  for (const run of linkedRuns.value) {
    const detail = runDetailCache.value[run.id]
    const trace = runtimeTraces[run.id]
    const preview = detail
      ? deriveHarnessRunPreview(run, detail.events || [])
      : deriveHarnessRunPreview(run, trace?.events || [])

    if (preview) map[run.id] = preview
  }

  return map
})

const failureLabelEntries = computed(() => sortedEntries(summaryNumberMap('failure_label_counts')))
const contextPackBreakdown = computed(() =>
  asRecord(asRecord(report.value?.group?.summary)?.contextpack_breakdown)
)
const contextPackItemsWithSnapshot = computed(() => {
  const value = Number(contextPackBreakdown.value?.items_with_snapshot || 0)
  return Number.isFinite(value) && value > 0 ? value : 0
})
const contextPackSelectedSkillEntries = computed(() =>
  sortedEntries(numberMap(contextPackBreakdown.value?.selected_skill_counts))
)
const contextPackSourceTrustEntries = computed(() =>
  sortedEntries(numberMap(contextPackBreakdown.value?.source_trust_counts))
)
const contextPackEntryEntries = computed(() =>
  sortedEntries(numberMap(contextPackBreakdown.value?.entry_id_counts))
)

const failedItems = computed<FailedItemRow[]>(() =>
  (report.value?.failed_items || []).map((entry, index) => {
    const record = asRecord(entry)
    const partialItem = asRecord(record?.item)
    const run = asRecord(record?.run)
    const scorecard = asRecord(record?.scorecard)
    const breakdown = safeJSON(String(scorecard?.breakdown_json || ''))
    const itemID = String(partialItem?.id || '')
    const fullItem = itemByID.value.get(itemID)
    const input = asRecord(fullItem?.input)

    const attemptCount = Number(fullItem?.attempt_count || 0)
    const maxAttempts = Number(fullItem?.max_attempts || 0)
    const attemptLabel = attemptCount > 0 && maxAttempts > 0 ? `${attemptCount}/${maxAttempts}` : ''

    return {
      id: itemID || `failed-${index}`,
      itemIndex:
        fullItem?.index ??
        (Number.isFinite(Number(partialItem?.index)) ? Number(partialItem?.index) : null),
      title: firstNonEmpty(
        String(input?.goal || ''),
        String(input?.query || ''),
        String(input?.prompt || ''),
        String(run?.goal || ''),
        String(group.value?.subject || '')
      ),
      runKind: String(fullItem?.run_kind || partialItem?.run_kind || ''),
      attempts: attemptLabel,
      status: firstNonEmpty(String(run?.status || ''), String(fullItem?.status || '')),
      failureLabel: String(breakdown?.failure_label || '').trim(),
      reason: firstNonEmpty(
        String(run?.error || ''),
        String(breakdown?.reason || ''),
        String(scorecard?.reason || '')
      ),
      runID: String(run?.id || fullItem?.latest_run_id || ''),
    }
  })
)

const itemRows = computed<ItemRow[]>(() =>
  items.value.map((item) => {
    const input = asRecord(item.input)
    const run = item.latest_run_id ? linkedRunMap.value.get(item.latest_run_id) : null
    return {
      id: item.id,
      index: Number.isFinite(Number(item.index)) ? Number(item.index) : null,
      title: firstNonEmpty(
        String(input?.goal || ''),
        String(input?.query || ''),
        String(input?.prompt || ''),
        String(group.value?.subject || ''),
        item.id
      ),
      status: item.status,
      attempts:
        item.attempt_count > 0 && item.max_attempts > 0
          ? `${item.attempt_count}/${item.max_attempts}`
          : '',
      runID: String(item.latest_run_id || ''),
      runStatus: String(run?.status || ''),
      runKind: String(run?.kind || item.run_kind || ''),
    }
  })
)

async function loadRunDetail(
  runID: string,
  options: { silent?: boolean; force?: boolean } = {}
): Promise<HarnessRunDetail | null> {
  if (!runID) return null

  if (!options.force && runDetailCache.value[runID]) {
    if (!options.silent) {
      runDetailError.value = ''
      runDetailWarning.value = ''
    }
    return runDetailCache.value[runID]
  }

  const requestToken = ++runDetailRequestToken

  if (!options.silent) {
    runDetailLoading.value = true
    runDetailError.value = ''
    runDetailWarning.value = ''
  }

  try {
    const response = await harnessApi.getRunDetail(runID)
    runDetailCache.value = {
      ...runDetailCache.value,
      [runID]: response.data,
    }

    if (requestToken === runDetailRequestToken) {
      runDetailError.value = ''
      if (options.silent) runDetailWarning.value = ''
    }

    return response.data
  } catch (err) {
    const message = getErrorMessage(err)

    if (requestToken === runDetailRequestToken) {
      if (options.silent && runDetailCache.value[runID]) {
        runDetailWarning.value = tr(
          'harness.group.runDetailRefreshWarning',
          'Run detail could not be refreshed. Showing the latest cached data.'
        )
      } else {
        runDetailError.value = message
      }
    }

    return runDetailCache.value[runID] || null
  } finally {
    if (!options.silent && requestToken === runDetailRequestToken) {
      runDetailLoading.value = false
    }
  }
}

async function openRunDetail(runID: string) {
  if (!runID) return
  selectedRunID.value = runID
  runDetailError.value = ''
  runDetailWarning.value = ''
  await scrollToRunNode(runID)
  await loadRunDetail(runID)
}

function closeRunDetail() {
  selectedRunID.value = ''
  runDetailLoading.value = false
  runDetailError.value = ''
  runDetailWarning.value = ''
}

async function refreshLiveData() {
  if (!group.value || hasTerminalGroup.value) return
  const refreshed = await loadReport({ silent: true })
  if (!refreshed || hasTerminalGroup.value) return

  if (
    selectedRunID.value &&
    selectedRun.value &&
    isHarnessRunActiveStatus(selectedRun.value.status)
  ) {
    await loadRunDetail(selectedRunID.value, {
      silent: true,
      force: true,
    })
  }
}

async function cancelGroup() {
  if (!group.value || hasTerminalGroup.value || actionLoading.value) return

  actionLoading.value = 'cancel'
  try {
    await harnessApi.cancelGroup(group.value.id)
    notification.info(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.group.cancelled', 'Group cancelled')
    )
    await loadReport({ silent: true })
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    actionLoading.value = ''
  }
}

async function retryFailed() {
  if (!group.value || actionLoading.value) return

  actionLoading.value = 'retry'
  try {
    const response = await harnessApi.retryFailedGroup(group.value.id)
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.group.retryResult', `Requeued ${response.data?.retried || 0} failed items.`)
    )
    await loadReport({ silent: true })
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    actionLoading.value = ''
  }
}

watch(
  groupID,
  () => {
    report.value = null
    error.value = ''
    closeRunDetail()
    runDetailCache.value = {}
    void loadReport()
  },
  { immediate: true }
)

onMounted(() => {
  syncScreenSize()
  window.addEventListener('resize', syncScreenSize)

  refreshTimer = setInterval(() => {
    void refreshLiveData()
  }, 4000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  window.removeEventListener('resize', syncScreenSize)
})
</script>

<template>
  <div class="harness-detail-page">
    <header class="detail-hero">
      <div class="hero-main">
        <RouterLink class="back-link" :to="{ name: 'HarnessGroups' }">
          {{ tr('common.back', 'Back') }}
        </RouterLink>

        <div class="hero-heading">
          <span class="kind-chip">{{ humanizeEnum(group?.kind) }}</span>
          <span class="status-chip" :class="statusTone(group?.status)">
            {{ humanizeStatus(group?.status) }}
          </span>
        </div>

        <h1>{{ group?.title || group?.subject || groupID }}</h1>
        <p class="hero-description">
          {{
            group?.subject ||
            tr('harness.group.noSubject', 'This group does not include a subject line.')
          }}
        </p>

        <dl v-if="group" class="hero-meta">
          <div>
            <dt>{{ tr('harness.groups.owner', 'Owner') }}</dt>
            <dd>{{ group.owner_user_id || tr('common.notAvailable', 'Not available') }}</dd>
          </div>
          <div>
            <dt>{{ tr('common.updatedAt', 'Updated') }}</dt>
            <dd>{{ formatDate(group.updated_at) }}</dd>
          </div>
          <div>
            <dt>{{ tr('harness.groups.startedAt', 'Started') }}</dt>
            <dd>{{ formatDate(group.started_at) }}</dd>
          </div>
          <div>
            <dt>{{ tr('harness.groups.finishedAt', 'Finished') }}</dt>
            <dd>{{ formatDate(group.finished_at) }}</dd>
          </div>
        </dl>
      </div>

      <div id="retry" class="hero-actions">
        <button
          type="button"
          class="ghost-button"
          :disabled="loading || refreshing"
          @click="loadReport({ silent: true })"
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

    <AutomationTabs class="automation-tab-strip" />

    <div v-if="error" class="state-card is-error">
      <h2>{{ tr('common.error', 'Error') }}</h2>
      <p>{{ error }}</p>
    </div>

    <div v-else-if="loading && !report" class="state-card">
      <h2>{{ tr('common.loading', 'Loading') }}</h2>
      <p>{{ tr('harness.group.loading', 'Loading the latest group report.') }}</p>
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
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.scoreDistribution', 'Score distribution') }}</h2>
          <span class="panel-caption">
            {{ tr('harness.group.scoringMode', 'Scoring mode') }}:
            {{ humanizeEnum(group.scoring_config?.mode || '') }}
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
          <span>
            {{ tr('harness.group.queuedCount', 'Queued') }}:
            {{ summaryCount('queued') + summaryCount('pending') }}
          </span>
          <span>
            {{ tr('harness.groups.running', 'Running') }}:
            {{ summaryCount('running') + summaryCount('scoring') }}
          </span>
          <span>
            {{ tr('harness.group.failedCount', 'Failed') }}:
            {{ summaryCount('failed') + summaryCount('error') }}
          </span>
        </div>

        <div v-if="failureLabelEntries.length" class="detail-pills">
          <span v-for="entry in failureLabelEntries" :key="`failure-label-${entry.key}`">
            {{ tr('harness.group.failureLabel', 'Failure label') }}: {{ entry.key }} ·
            {{ entry.value }}
          </span>
        </div>
        <p v-else class="empty-text">
          {{ tr('harness.group.noFailureLabels', 'No failure labels recorded.') }}
        </p>
      </section>

      <section v-if="contextPackBreakdown" class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.contextPacks', 'Context Packs') }}</h2>
          <span class="panel-caption">
            {{ contextPackItemsWithSnapshot }} / {{ items.length }}
            {{ tr('harness.groups.itemCount', 'Items') }}
          </span>
        </div>

        <div class="verdict-grid">
          <div class="verdict-card">
            <span>{{ tr('harness.group.contextPackItems', 'Items with packs') }}</span>
            <strong>{{ contextPackItemsWithSnapshot }}</strong>
          </div>
          <div class="verdict-card">
            <span>{{ tr('harness.group.contextPackSelectedSkills', 'Selected skills') }}</span>
            <strong>{{ contextPackSelectedSkillEntries.length }}</strong>
          </div>
          <div class="verdict-card">
            <span>{{ tr('harness.group.contextPackSources', 'Sources') }}</span>
            <strong>{{ contextPackSourceTrustEntries.length }}</strong>
          </div>
          <div class="verdict-card">
            <span>{{ tr('harness.group.contextPackEntries', 'Entries') }}</span>
            <strong>{{ contextPackEntryEntries.length }}</strong>
          </div>
        </div>

        <div v-if="contextPackSelectedSkillEntries.length" class="panel-subsection">
          <h3>{{ tr('harness.group.contextPackSelectedSkills', 'Selected skills') }}</h3>
          <div class="detail-pills">
            <span v-for="entry in contextPackSelectedSkillEntries" :key="`cp-skill-${entry.key}`">
              {{ entry.key }} · {{ entry.value }}
            </span>
          </div>
        </div>

        <div v-if="contextPackSourceTrustEntries.length" class="panel-subsection">
          <h3>{{ tr('harness.group.contextPackSources', 'Sources') }}</h3>
          <div class="detail-pills">
            <span v-for="entry in contextPackSourceTrustEntries" :key="`cp-source-${entry.key}`">
              {{ entry.key }} · {{ entry.value }}
            </span>
          </div>
        </div>

        <div v-if="contextPackEntryEntries.length" class="panel-subsection">
          <h3>{{ tr('harness.group.contextPackEntries', 'Entries') }}</h3>
          <div class="detail-pills">
            <span v-for="entry in contextPackEntryEntries" :key="`cp-entry-${entry.key}`">
              {{ entry.key }} · {{ entry.value }}
            </span>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.failedItems', 'Failed items') }}</h2>
          <span class="panel-caption">{{ failedItems.length }} / {{ items.length }}</span>
        </div>

        <div v-if="failedItems.length" class="failed-list">
          <article v-for="item in failedItems" :key="item.id" class="failed-card">
            <div class="failed-card-header">
              <strong>#{{ item.itemIndex ?? '?' }}</strong>
              <span class="status-chip" :class="statusTone(item.status)">
                {{ humanizeStatus(item.status) }}
              </span>
            </div>
            <p class="failed-title">{{ item.title || group.subject || item.id }}</p>
            <div class="detail-pills">
              <span v-if="item.runKind">{{ humanizeEnum(item.runKind) }}</span>
              <span v-if="item.attempts">
                {{ tr('harness.group.attempts', 'Attempts') }}: {{ item.attempts }}
              </span>
              <span v-if="item.failureLabel">
                {{ tr('harness.group.failureLabel', 'Failure label') }}: {{ item.failureLabel }}
              </span>
            </div>
            <p class="failed-reason">
              {{
                item.reason || tr('harness.group.noFailureReason', 'No failure reason recorded.')
              }}
            </p>
            <div class="card-actions">
              <button
                type="button"
                class="secondary-button inspect-run-button"
                :disabled="!item.runID"
                @click="openRunDetail(item.runID)"
              >
                {{ tr('harness.group.inspectRun', 'Inspect run') }}
              </button>
            </div>
          </article>
        </div>
        <p v-else class="empty-text">
          {{ tr('harness.group.noFailedItems', 'No failed items in the latest report.') }}
        </p>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.items', 'Items') }}</h2>
          <span class="panel-caption">{{ itemRows.length }}</span>
        </div>

        <div v-if="itemRows.length" class="item-list">
          <article v-for="item in itemRows" :key="item.id" class="item-card">
            <div class="failed-card-header">
              <strong>#{{ item.index ?? '?' }}</strong>
              <span class="status-chip" :class="statusTone(item.runStatus || item.status)">
                {{ humanizeStatus(item.runStatus || item.status) }}
              </span>
            </div>
            <p class="failed-title">{{ item.title }}</p>
            <div class="detail-pills">
              <span v-if="item.runKind">{{ humanizeEnum(item.runKind) }}</span>
              <span v-if="item.attempts">
                {{ tr('harness.group.attempts', 'Attempts') }}: {{ item.attempts }}
              </span>
            </div>
            <div class="card-actions">
              <button
                type="button"
                class="secondary-button inspect-run-button"
                :disabled="!item.runID"
                @click="openRunDetail(item.runID)"
              >
                {{ tr('harness.group.inspectRun', 'Inspect run') }}
              </button>
            </div>
          </article>
        </div>
        <p v-else class="empty-text">
          {{ tr('harness.group.noItems', 'No items were recorded in the latest report.') }}
        </p>
      </section>

      <section class="panel run-graph-panel">
        <div class="panel-header panel-header-start">
          <div>
            <h2>{{ tr('harness.group.runGraph', 'Run graph') }}</h2>
            <p class="panel-description">
              {{
                tr(
                  'harness.group.runGraphHint',
                  'Coordinator and worker runs are grouped here so you can inspect the execution tree.'
                )
              }}
            </p>
          </div>
          <span class="panel-caption">{{ linkedRuns.length }}</span>
        </div>

        <div
          v-if="linkedRuns.length"
          class="run-graph-layout"
          :class="{ 'is-mobile': isNarrowScreen }"
        >
          <HarnessRunTree
            class="run-graph-tree"
            :nodes="rootRunTreeNodes"
            :detached-nodes="detachedRunTreeNodes"
            :selected-run-id="selectedRunID"
            :event-preview-map="runEventPreviewMap"
            @select="openRunDetail"
          />

          <HarnessRunDetailDrawer
            v-if="!isNarrowScreen"
            :open="!!selectedRunID"
            :mobile="false"
            :run="selectedRun"
            :detail="selectedRunDetail"
            :child-runs="selectedChildRuns"
            :loading="runDetailLoading"
            :error="runDetailError"
            :warning="runDetailWarning"
            @close="closeRunDetail"
          />
        </div>
        <p v-else class="empty-text">
          {{
            tr('harness.group.noLinkedRuns', 'No linked runs were persisted for this group yet.')
          }}
        </p>
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.artifacts', 'Artifacts') }}</h2>
          <span class="panel-caption">{{ artifacts.length }}</span>
        </div>

        <div v-if="artifacts.length" class="artifact-list">
          <article v-for="artifact in artifacts" :key="artifact.id" class="artifact-card">
            <strong>{{ artifact.label || artifact.kind || artifact.id }}</strong>
            <p>{{ artifact.path_or_url || tr('common.notAvailable', 'Not available') }}</p>
          </article>
        </div>
        <p v-else class="empty-text">
          {{ tr('harness.group.noArtifacts', 'No artifacts attached to the linked runs yet.') }}
        </p>
      </section>
    </template>

    <HarnessRunDetailDrawer
      v-if="isNarrowScreen"
      :open="!!selectedRunID"
      :mobile="true"
      :run="selectedRun"
      :detail="selectedRunDetail"
      :child-runs="selectedChildRuns"
      :loading="runDetailLoading"
      :error="runDetailError"
      :warning="runDetailWarning"
      @close="closeRunDetail"
    />
  </div>
</template>

<style scoped>
.harness-detail-page {
  --harness-detail-surface: var(--color-background-soft);
  --harness-detail-subsurface: var(--color-bg-surface);
  --harness-detail-text-strong: var(--color-text);
  --harness-detail-text-secondary: var(--color-text-secondary);
  --harness-detail-text-muted: var(--color-text-muted);
  display: flex;
  flex-direction: column;
  gap: 1rem;
  color: var(--harness-detail-text-strong);
}

.automation-tab-strip {
  position: relative;
  z-index: 1;
}

.detail-hero,
.state-card,
.stat-card,
.panel,
.failed-card,
.item-card,
.artifact-card {
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.04);
  border-radius: 1rem;
}

.detail-hero,
.state-card,
.panel {
  padding: 1.25rem;
}

.detail-hero {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}

.hero-main {
  min-width: 0;
}

.back-link {
  display: inline-flex;
  align-items: center;
  margin-bottom: 0.85rem;
  color: #2563eb;
  text-decoration: none;
  font-weight: 600;
}

.hero-heading {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.detail-hero h1,
.state-card h2,
.panel h2,
.failed-card strong,
.item-card strong {
  margin: 0;
}

.hero-description,
.state-card p,
.failed-title,
.failed-reason,
.artifact-card p {
  margin: 0.45rem 0 0;
  color: rgba(15, 23, 42, 0.72);
  line-height: 1.55;
}

.hero-meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 0.85rem;
  margin: 1rem 0 0;
}

.hero-meta dt,
.panel-caption,
.summary-row,
.detail-pills,
.failed-card-header {
  color: rgba(15, 23, 42, 0.72);
}

.hero-meta dt {
  font-size: 0.82rem;
}

.hero-meta dd {
  margin: 0.2rem 0 0;
  font-weight: 600;
}

.hero-actions {
  display: flex;
  gap: 0.6rem;
  flex-wrap: wrap;
}

.ghost-button,
.warning-button,
.primary-button,
.secondary-button {
  border-radius: 999px;
  padding: 0.7rem 1rem;
  font: inherit;
  cursor: pointer;
}

.ghost-button {
  border: 1px solid rgba(148, 163, 184, 0.28);
  background: white;
}

.warning-button {
  border: 1px solid rgba(245, 158, 11, 0.28);
  background: rgba(245, 158, 11, 0.1);
  color: #92400e;
}

.primary-button {
  border: 1px solid #0f172a;
  background: #0f172a;
  color: white;
}

.secondary-button {
  border: 1px solid rgba(15, 118, 110, 0.22);
  background: rgba(240, 253, 250, 0.9);
  color: #0f766e;
  font-weight: 700;
}

.ghost-button:disabled,
.warning-button:disabled,
.primary-button:disabled,
.secondary-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.is-error {
  border-color: rgba(239, 68, 68, 0.28);
  background: rgba(239, 68, 68, 0.08);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 0.85rem;
}

.stat-card {
  padding: 1rem;
}

.stat-card span {
  display: block;
  font-size: 0.88rem;
  color: rgba(15, 23, 42, 0.68);
}

.stat-card strong {
  display: block;
  margin-top: 0.25rem;
  font-size: 1.45rem;
}

.panel {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.panel-subsection {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.panel-subsection h3 {
  margin: 0;
  font-size: 0.92rem;
  color: rgba(15, 23, 42, 0.78);
}

.panel-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: center;
}

.panel-header-start {
  align-items: flex-start;
}

.panel-description {
  margin: 0.35rem 0 0;
  color: rgba(15, 23, 42, 0.68);
  line-height: 1.55;
}

.verdict-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 0.75rem;
}

.verdict-card {
  border-radius: 0.85rem;
  padding: 0.9rem;
  background: rgba(255, 255, 255, 0.72);
}

.verdict-card span {
  display: block;
  font-size: 0.84rem;
  color: rgba(15, 23, 42, 0.64);
}

.verdict-card strong {
  display: block;
  margin-top: 0.2rem;
  font-size: 1.25rem;
}

.summary-row,
.detail-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  font-size: 0.9rem;
}

.detail-pills span {
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.78);
  padding: 0.35rem 0.65rem;
}

.failed-list,
.item-list,
.artifact-list {
  display: grid;
  gap: 0.75rem;
}

.failed-card,
.item-card,
.artifact-card {
  padding: 1rem;
}

.failed-card-header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: flex-start;
}

.failed-title {
  font-weight: 600;
  color: #0f172a;
}

.card-actions {
  display: flex;
  justify-content: flex-start;
  margin-top: 0.2rem;
}

.run-graph-panel {
  overflow: hidden;
}

.run-graph-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(22rem, 0.9fr);
  gap: 1rem;
  align-items: start;
}

.run-graph-layout.is-mobile {
  grid-template-columns: 1fr;
}

.run-graph-tree {
  min-width: 0;
}

.empty-text {
  margin: 0;
  color: rgba(15, 23, 42, 0.64);
}

.kind-chip,
.status-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.4rem 0.7rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 700;
}

.kind-chip,
.status-chip.is-muted {
  background: rgba(148, 163, 184, 0.16);
  color: rgba(15, 23, 42, 0.75);
}

.status-chip.is-success {
  background: rgba(34, 197, 94, 0.16);
  color: #166534;
}

.status-chip.is-warning,
.status-chip.is-blocked {
  background: rgba(245, 158, 11, 0.18);
  color: #92400e;
}

.status-chip.is-danger {
  background: rgba(239, 68, 68, 0.16);
  color: #991b1b;
}

.status-chip.is-active {
  background: rgba(59, 130, 246, 0.16);
  color: #1d4ed8;
}

.harness-detail-page :is(
  .detail-hero,
  .state-card,
  .stat-card,
  .panel,
  .failed-card,
  .item-card,
  .artifact-card
) {
  border-color: var(--color-border);
  background: var(--harness-detail-surface);
}

.harness-detail-page :is(
  .hero-description,
  .state-card p,
  .failed-reason,
  .artifact-card p,
  .panel-description
) {
  color: var(--harness-detail-text-secondary);
}

.harness-detail-page :is(
  .hero-meta dt,
  .panel-caption,
  .summary-row,
  .detail-pills,
  .failed-card-header,
  .stat-card span,
  .verdict-card span,
  .empty-text
) {
  color: var(--harness-detail-text-muted);
}

.harness-detail-page :is(
  .hero-meta dd,
  .stat-card strong,
  .panel-subsection h3,
  .verdict-card strong,
  .failed-title
) {
  color: var(--harness-detail-text-strong);
}

.harness-detail-page :is(
  .verdict-card,
  .detail-pills span
) {
  background: var(--harness-detail-subsurface);
  border: 1px solid var(--color-border);
}

.harness-detail-page .ghost-button {
  border-color: var(--color-border);
  background: var(--harness-detail-surface);
  color: var(--harness-detail-text-strong);
}

.harness-detail-page :is(.kind-chip, .status-chip.is-muted) {
  color: var(--harness-detail-text-secondary);
}

@media (max-width: 959px) {
  .run-graph-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .detail-hero,
  .panel-header,
  .failed-card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .hero-actions {
    width: 100%;
  }

  .ghost-button,
  .warning-button,
  .primary-button,
  .secondary-button {
    width: 100%;
  }

  .card-actions {
    width: 100%;
  }
}
</style>
