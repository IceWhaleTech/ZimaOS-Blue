<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  harnessApi,
  type HarnessArtifactRef,
  type HarnessRunGroup,
  type HarnessRunGroupItem,
  type HarnessRunGroupReport,
  type HarnessRunGroupStatus,
  type HarnessRunSummary,
  type HarnessScorecard,
} from '@/api/harness'
import { useNotificationStore } from '@/stores/notification'
import { getErrorMessage } from '@/utils/error'

type FailedItemRow = {
  item?: HarnessRunGroupItem | null
  scorecard?: HarnessScorecard | null
  run?: HarnessRunSummary | null
}

const route = useRoute()
const { t, te } = useI18n()
const notification = useNotificationStore()

const loading = ref(false)
const refreshing = ref(false)
const actionLoading = ref<'cancel' | 'retry' | ''>('')
const error = ref('')
const report = ref<HarnessRunGroupReport | null>(null)

let refreshTimer: ReturnType<typeof setInterval> | null = null

const groupID = computed(() => String(route.params.id || '').trim())
const activeStatuses = new Set<HarnessRunGroupStatus>(['pending', 'queued', 'running', 'scoring'])

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
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

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function percentLabel(value?: number): string {
  return `${Math.round(Number(value || 0) * 100)}%`
}

function scoreLabel(value?: number): string {
  return Number(value || 0).toFixed(2)
}

function summaryCount(key: string): number {
  const summary = asRecord(report.value?.group?.summary)
  const counts = asRecord(summary?.counts)
  if (!counts) return 0
  const value = Number(counts[key])
  return Number.isFinite(value) ? value : 0
}

const group = computed<HarnessRunGroup | null>(() => report.value?.group || null)
const items = computed(() => report.value?.items || [])
const scorecards = computed(() => report.value?.scorecards || [])
const linkedRuns = computed(() => report.value?.linked_runs || [])
const artifacts = computed(() => report.value?.artifacts || [])
const verdictCounts = computed(() => report.value?.verdict_counts || {})
const failedItems = computed<FailedItemRow[]>(() =>
  (report.value?.failed_items || []).map((entry) => {
    const record = asRecord(entry)
    return {
      item: (record?.item as HarnessRunGroupItem | undefined) || null,
      scorecard: (record?.scorecard as HarnessScorecard | undefined) || null,
      run: (record?.run as HarnessRunSummary | undefined) || null,
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
    notification.info(tr('nav.harness', 'Harness'), tr('harness.group.cancelled', 'Group cancelled'))
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
        <RouterLink class="back-link" :to="{ name: 'HarnessGroups' }">
          {{ tr('common.back', 'Back') }}
        </RouterLink>
        <div class="hero-heading">
          <span class="kind-chip" :class="group ? `is-${group.kind}` : 'is-eval'">
            {{ group?.kind || tr('nav.harness', 'Harness') }}
          </span>
          <span class="status-chip" :class="statusTone(group?.status || '')">
            {{ group?.status || tr('common.loading', 'Loading') }}
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
        <button type="button" class="ghost-button" :disabled="loading || refreshing" @click="loadReport()">
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
      </section>

      <section class="panel">
        <div class="panel-header">
          <h2>{{ tr('harness.group.scoreDistribution', 'Score distribution') }}</h2>
          <span class="panel-caption">{{ tr('harness.group.scoringMode', 'Scoring mode') }}: {{ group.scoring_config?.mode || 'hybrid' }}</span>
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
          <span>{{ tr('harness.group.queuedCount', 'Queued') }}: {{ summaryCount('queued') + summaryCount('pending') }}</span>
          <span>{{ tr('harness.group.runningCount', 'Running') }}: {{ summaryCount('running') }}</span>
          <span>{{ tr('harness.group.failedCount', 'Failed') }}: {{ summaryCount('failed') + summaryCount('error') }}</span>
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
                <span class="status-chip" :class="statusTone(item.status)">{{ item.status }}</span>
                <span v-if="item.profile" class="profile-chip">{{ item.profile }}</span>
              </div>
              <p class="row-subtitle">
                {{
                  String(item.input?.goal || item.input?.query || item.input?.prompt || '').trim() ||
                  tr('harness.group.noInputSummary', 'No goal/query recorded for this item.')
                }}
              </p>
            </div>
            <div class="row-metrics">
              <span>{{ tr('harness.group.attempts', 'Attempts') }}: {{ item.attempt_count }}/{{ item.max_attempts || 1 }}</span>
              <span>
                {{ tr('harness.group.scorecard', 'Verdict') }}:
                {{ scorecardByItemID[item.id]?.verdict || tr('common.notAvailable', 'Not available') }}
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
          <article v-for="entry in failedItems" :key="entry.item?.id || entry.run?.id" class="failed-card">
            <div class="failed-header">
              <strong>#{{ entry.item?.index ?? '?' }}</strong>
              <span class="status-chip" :class="statusTone(entry.scorecard?.verdict || entry.item?.status || '')">
                {{ entry.scorecard?.verdict || entry.item?.status || 'unknown' }}
              </span>
            </div>
            <p class="failed-title">{{ entry.item?.profile || tr('harness.group.unprofiled', 'Unprofiled item') }}</p>
            <p class="failed-reason">
              {{ scorecardReason(entry.scorecard) || runPreview(entry.run) || tr('harness.group.noFailureReason', 'No failure reason recorded.') }}
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
          {{ tr('harness.group.noLinkedRuns', 'No linked runs were persisted for this group yet.') }}
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
                  <span class="status-chip" :class="statusTone(run.status)">{{ run.status }}</span>
                </div>
                <p class="run-meta">
                  {{ run.kind }} · {{ tr('harness.group.attemptIndex', 'Attempt') }} {{ run.attempt_index || 0 }} ·
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
              <strong>{{ artifact.label || artifact.kind }}</strong>
              <p class="artifact-meta">{{ artifact.kind }} · {{ artifact.mime_type || tr('common.notAvailable', 'Not available') }}</p>
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
            <code v-else class="artifact-path">{{ artifact.path_or_url || tr('common.notAvailable', 'Not available') }}</code>
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
  .verdict-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .row-metrics {
    align-items: flex-start;
  }
}

@media (max-width: 720px) {
  .hero-meta,
  .stats-grid,
  .verdict-grid {
    grid-template-columns: 1fr;
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
