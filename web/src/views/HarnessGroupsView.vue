<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { harnessApi, type HarnessRunGroup, type HarnessRunGroupStatus } from '@/api/harness'
import { getErrorMessage } from '@/utils/error'

const { t, te } = useI18n()

type GroupFilterMode = 'all' | 'active' | 'terminal'

const loading = ref(false)
const refreshing = ref(false)
const error = ref('')
const groups = ref<HarnessRunGroup[]>([])
const search = ref('')
const filterMode = ref<GroupFilterMode>('all')

const terminalStatuses = new Set<HarnessRunGroupStatus>([
  'completed',
  'partial',
  'failed',
  'cancelled',
])

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function humanizeEnum(value: string): string {
  return value
    .replace(/_/g, ' ')
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function summaryNumber(group: HarnessRunGroup, key: string): number {
  const summary = asRecord(group.summary)
  if (!summary) return 0
  const value = Number(summary[key])
  return Number.isFinite(value) ? value : 0
}

function summaryCounts(group: HarnessRunGroup): Record<string, number> {
  const summary = asRecord(group.summary)
  const counts = asRecord(summary?.counts)
  if (!counts) return {}
  const out: Record<string, number> = {}
  for (const [key, value] of Object.entries(counts)) {
    const next = Number(value)
    if (Number.isFinite(next)) out[key] = next
  }
  return out
}

function groupItemCount(group: HarnessRunGroup): number {
  const explicit = summaryNumber(group, 'item_count')
  if (explicit > 0) return explicit
  const counts = summaryCounts(group)
  return Object.entries(counts)
    .filter(([key]) => !key.startsWith('verdict:'))
    .reduce((sum, [, value]) => sum + value, 0)
}

function passRate(group: HarnessRunGroup): number {
  const value = summaryNumber(group, 'pass_rate')
  return value > 0 ? value : 0
}

function overallScore(group: HarnessRunGroup): number {
  const value = summaryNumber(group, 'overall_score')
  return value > 0 ? value : 0
}

function statusTone(status: HarnessRunGroupStatus): string {
  switch (status) {
    case 'completed':
      return 'is-success'
    case 'running':
    case 'scoring':
      return 'is-running'
    case 'partial':
      return 'is-warning'
    case 'failed':
    case 'cancelled':
      return 'is-danger'
    default:
      return 'is-muted'
  }
}

function kindTone(kind: string): string {
  switch (kind) {
    case 'experiment':
      return 'is-experiment'
    case 'batch':
      return 'is-batch'
    default:
      return 'is-eval'
  }
}

function statusLabel(status: HarnessRunGroupStatus): string {
  switch (status) {
    case 'running':
      return tr('harness.groups.running', 'Running')
    case 'failed':
      return tr('harness.groups.failed', 'Failed')
    case 'partial':
      return tr('harness.group.partialVerdict', 'Partial')
    case 'queued':
      return tr('harness.group.queuedCount', 'Queued')
    default:
      return humanizeEnum(status)
  }
}

function kindLabel(kind: string): string {
  return humanizeEnum(kind)
}

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function percentLabel(value: number): string {
  return `${Math.round(value * 100)}%`
}

async function loadGroups(options: { silent?: boolean } = {}) {
  if (options.silent) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  error.value = ''
  try {
    const response = await harnessApi.listGroups({ limit: 100 })
    groups.value = response.data || []
  } catch (err) {
    error.value = getErrorMessage(err)
  } finally {
    loading.value = false
    refreshing.value = false
  }
}

const filteredGroups = computed(() => {
  const query = search.value.trim().toLowerCase()
  return groups.value.filter((group) => {
    if (filterMode.value === 'active' && terminalStatuses.has(group.status)) return false
    if (filterMode.value === 'terminal' && !terminalStatuses.has(group.status)) return false
    if (!query) return true

    return [group.title, group.subject, group.kind, group.status, group.id]
      .map((value) => String(value || '').toLowerCase())
      .some((value) => value.includes(query))
  })
})

const totalGroups = computed(() => groups.value.length)
const activeGroups = computed(
  () => groups.value.filter((group) => !terminalStatuses.has(group.status)).length
)
const averagePassRate = computed(() => {
  const rated = groups.value.filter((group) => passRate(group) > 0)
  if (!rated.length) return 0
  return rated.reduce((sum, group) => sum + passRate(group), 0) / rated.length
})
const totalItems = computed(() =>
  groups.value.reduce((sum, group) => sum + groupItemCount(group), 0)
)

onMounted(() => {
  void loadGroups()
})
</script>

<template>
  <div class="harness-groups-page">
    <section class="hero-card">
      <div class="hero-copy">
        <p class="eyebrow">Harness V2</p>
        <h1>{{ tr('nav.harness', 'Harness') }}</h1>
        <p class="hero-description">
          {{
            tr(
              'harness.groups.subtitle',
              'Track eval groups, experiment projections, score breakdowns, and retries from one control-plane view.'
            )
          }}
        </p>
      </div>
      <div class="hero-actions">
        <button class="refresh-button" type="button" :disabled="loading || refreshing" @click="loadGroups()">
          {{ refreshing ? tr('common.loading', 'Loading') : tr('common.refresh', 'Refresh') }}
        </button>
      </div>
    </section>

    <section class="stats-grid">
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.totalGroups', 'Groups') }}</span>
        <strong class="stat-value">{{ totalGroups }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.activeGroups', 'Active') }}</span>
        <strong class="stat-value">{{ activeGroups }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.totalItems', 'Items') }}</span>
        <strong class="stat-value">{{ totalItems }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.avgPassRate', 'Average pass rate') }}</span>
        <strong class="stat-value">{{ percentLabel(averagePassRate) }}</strong>
      </article>
    </section>

    <section class="toolbar">
      <div class="filter-segment" role="tablist" :aria-label="tr('harness.groups.filters', 'Filters')">
        <button
          type="button"
          class="segment-button"
          :class="{ active: filterMode === 'all' }"
          @click="filterMode = 'all'"
        >
          {{ tr('common.all', 'All') }}
        </button>
        <button
          type="button"
          class="segment-button"
          :class="{ active: filterMode === 'active' }"
          @click="filterMode = 'active'"
        >
          {{ tr('harness.groups.activeOnly', 'Active') }}
        </button>
        <button
          type="button"
          class="segment-button"
          :class="{ active: filterMode === 'terminal' }"
          @click="filterMode = 'terminal'"
        >
          {{ tr('harness.groups.terminalOnly', 'Terminal') }}
        </button>
      </div>
      <label class="search-field">
        <span class="search-label">{{ tr('common.search', 'Search') }}</span>
        <input
          v-model="search"
          type="search"
          :placeholder="tr('harness.groups.searchPlaceholder', 'Search title, subject, kind, or status')"
        />
      </label>
    </section>

    <div v-if="error" class="state-card is-error">
      <h2>{{ tr('common.error', 'Error') }}</h2>
      <p>{{ error }}</p>
    </div>

    <div v-else-if="loading" class="state-card">
      <h2>{{ tr('common.loading', 'Loading') }}</h2>
      <p>{{ tr('harness.groups.loading', 'Fetching the latest harness group summaries.') }}</p>
    </div>

    <div v-else-if="filteredGroups.length === 0" class="state-card">
      <h2>{{ tr('harness.groups.emptyTitle', 'No harness groups yet') }}</h2>
      <p>
        {{
          tr(
            'harness.groups.emptyDescription',
            'Groups will appear here once eval batches, experiments, or projected research runs are recorded.'
          )
        }}
      </p>
    </div>

    <section v-else class="groups-grid">
      <RouterLink
        v-for="group in filteredGroups"
        :key="group.id"
        :to="{ name: 'HarnessGroupDetail', params: { id: group.id } }"
        class="group-card"
      >
        <div class="group-card-header">
          <span class="kind-chip" :class="kindTone(group.kind)">{{ kindLabel(group.kind) }}</span>
          <span class="status-chip" :class="statusTone(group.status)">{{ statusLabel(group.status) }}</span>
        </div>

        <h2 class="group-title">{{ group.title || group.subject || group.id }}</h2>
        <p class="group-subject">{{ group.subject || tr('harness.groups.noSubject', 'No subject provided') }}</p>

        <div class="metrics-row">
          <div class="metric">
            <span>{{ tr('harness.groups.itemCount', 'Items') }}</span>
            <strong>{{ groupItemCount(group) }}</strong>
          </div>
          <div class="metric">
            <span>{{ tr('harness.groups.passRate', 'Pass rate') }}</span>
            <strong>{{ percentLabel(passRate(group)) }}</strong>
          </div>
          <div class="metric">
            <span>{{ tr('harness.groups.score', 'Score') }}</span>
            <strong>{{ overallScore(group).toFixed(2) }}</strong>
          </div>
        </div>

        <div class="count-row">
          <span>{{ tr('harness.groups.running', 'Running') }}: {{ summaryCounts(group).running || 0 }}</span>
          <span>{{ tr('harness.groups.failed', 'Failed') }}: {{ summaryCounts(group).failed || 0 }}</span>
          <span>{{ tr('harness.groups.passed', 'Passed') }}: {{ summaryCounts(group).passed || 0 }}</span>
        </div>

        <div class="footer-row">
          <span>{{ tr('common.updatedAt', 'Updated') }}</span>
          <strong>{{ formatDate(group.updated_at) }}</strong>
        </div>
      </RouterLink>
    </section>
  </div>
</template>

<style scoped>
.harness-groups-page {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 1.5rem;
  padding: 1.5rem;
  border-radius: 1.25rem;
  background:
    radial-gradient(circle at top right, rgba(16, 185, 129, 0.16), transparent 34%),
    linear-gradient(135deg, rgba(15, 23, 42, 0.04), rgba(15, 23, 42, 0.02));
  border: 1px solid rgba(15, 23, 42, 0.08);
}

.hero-copy h1 {
  margin: 0.2rem 0 0.6rem;
  font-size: 2rem;
  line-height: 1.1;
  color: #111827;
}

.eyebrow {
  margin: 0;
  text-transform: uppercase;
  letter-spacing: 0.16em;
  font-size: 0.78rem;
  font-weight: 700;
  color: #0f766e;
}

.hero-description {
  margin: 0;
  max-width: 60rem;
  color: #475569;
  line-height: 1.6;
}

.hero-actions {
  display: flex;
  align-items: flex-start;
}

.refresh-button,
.segment-button {
  appearance: none;
  border: 0;
  cursor: pointer;
}

.refresh-button {
  padding: 0.8rem 1.1rem;
  border-radius: 0.9rem;
  background: #111827;
  color: #fff;
  font-weight: 600;
}

.refresh-button:disabled {
  opacity: 0.65;
  cursor: wait;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1rem;
}

.stat-card,
.state-card,
.group-card {
  border-radius: 1.1rem;
  border: 1px solid rgba(15, 23, 42, 0.08);
  background: #fff;
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.06);
}

.stat-card {
  padding: 1rem 1.1rem;
}

.stat-label {
  display: block;
  color: #64748b;
  font-size: 0.85rem;
}

.stat-value {
  display: block;
  margin-top: 0.45rem;
  font-size: 1.75rem;
  color: #0f172a;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 1rem;
}

.filter-segment {
  display: inline-flex;
  align-items: center;
  padding: 0.25rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
}

.segment-button {
  padding: 0.65rem 1rem;
  border-radius: 999px;
  background: transparent;
  color: #334155;
  font-weight: 600;
}

.segment-button.active {
  background: #fff;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08);
}

.search-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  min-width: min(22rem, 100%);
}

.search-label {
  font-size: 0.82rem;
  color: #64748b;
}

.search-field input {
  width: 100%;
  padding: 0.85rem 1rem;
  border-radius: 0.9rem;
  border: 1px solid rgba(148, 163, 184, 0.35);
  background: #fff;
}

.state-card {
  padding: 1.2rem 1.3rem;
}

.state-card.is-error {
  border-color: rgba(239, 68, 68, 0.25);
  background: rgba(254, 242, 242, 0.9);
}

.state-card h2 {
  margin: 0 0 0.35rem;
  color: #0f172a;
}

.state-card p {
  margin: 0;
  color: #475569;
}

.groups-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}

.group-card {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1.2rem;
  color: inherit;
  text-decoration: none;
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease,
    border-color 0.18s ease;
}

.group-card:hover {
  transform: translateY(-2px);
  border-color: rgba(16, 185, 129, 0.28);
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.1);
}

.group-card-header,
.metrics-row,
.count-row,
.footer-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.75rem;
}

.group-title {
  margin: 0;
  font-size: 1.1rem;
  color: #111827;
}

.group-subject {
  margin: 0;
  color: #64748b;
  line-height: 1.55;
}

.kind-chip,
.status-chip {
  display: inline-flex;
  align-items: center;
  padding: 0.32rem 0.7rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.kind-chip.is-eval {
  background: rgba(14, 165, 233, 0.14);
  color: #0369a1;
}

.kind-chip.is-experiment {
  background: rgba(245, 158, 11, 0.16);
  color: #b45309;
}

.kind-chip.is-batch {
  background: rgba(139, 92, 246, 0.16);
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
  background: rgba(100, 116, 139, 0.15);
  color: #475569;
}

.metric {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.metric span,
.count-row,
.footer-row span {
  color: #64748b;
  font-size: 0.84rem;
}

.metric strong,
.footer-row strong {
  color: #0f172a;
}

@media (max-width: 960px) {
  .stats-grid,
  .groups-grid {
    grid-template-columns: 1fr;
  }

  .hero-card {
    flex-direction: column;
  }

  .hero-actions {
    align-items: stretch;
  }

  .refresh-button {
    width: 100%;
  }
}
</style>
