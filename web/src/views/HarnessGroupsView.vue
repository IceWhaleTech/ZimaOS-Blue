<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  harnessApi,
  type HarnessRunGroup,
  type HarnessRunGroupSpec,
  type HarnessRunGroupStatus,
  type HarnessRunKind,
  type HarnessScoringMode,
} from '@/api/harness'
import AutomationTabs from '@/components/automation/AutomationTabs.vue'
import { useNotificationStore } from '@/stores/notification'
import { getErrorMessage } from '@/utils/error'

type GroupFilterMode = 'all' | 'active' | 'terminal'
type QuickEvalPreset = 'smoke' | 'regression' | 'research'

type NumberEntry = {
  key: string
  value: number
}

const { t, te } = useI18n()
const router = useRouter()
const notification = useNotificationStore()

const loading = ref(false)
const refreshing = ref(false)
const error = ref('')
const groups = ref<HarnessRunGroup[]>([])
const groupSearch = ref('')
const groupFilterMode = ref<GroupFilterMode>('all')
const createAction = ref<'quick' | ''>('')
const quickEvalPreset = ref<QuickEvalPreset>('smoke')

const defaultManifestExample = JSON.stringify(
  {
    dataset: {
      name: 'Smoke Dataset',
      subject: 'agent_task',
    },
    defaults: {
      run_kind: 'agent_task',
      profile: 'smoke',
      scoring: {
        mode: 'rule',
        pass_threshold: 0.5,
      },
    },
    items: [
      {
        id: 'case-1',
        input: {
          goal: 'finish and verify',
        },
        expected: {
          contains: 'verified',
        },
      },
    ],
  },
  null,
  2
)
const quickEvalManifestText = ref(defaultManifestExample)

const terminalStatuses = new Set<HarnessRunGroupStatus>([
  'completed',
  'partial',
  'failed',
  'cancelled',
])

let refreshTimer: ReturnType<typeof setInterval> | null = null

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function humanizeEnum(value: string | null | undefined): string {
  const normalized = String(value || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')
  return normalized.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
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
  return Number(value).toFixed(2)
}

function summaryValue(group: HarnessRunGroup, key: string): number | null {
  const summary = asRecord(group.summary)
  if (!summary || !(key in summary)) return null
  const value = Number(summary[key])
  return Number.isFinite(value) ? value : null
}

function summaryCount(group: HarnessRunGroup, key: string): number {
  const summary = asRecord(group.summary)
  const counts = asRecord(summary?.counts)
  if (!counts) return 0
  const value = Number(counts[key])
  return Number.isFinite(value) ? value : 0
}

function statusTone(status?: string | null): string {
  switch (String(status || '').trim()) {
    case 'completed':
      return 'is-success'
    case 'partial':
      return 'is-warning'
    case 'failed':
    case 'cancelled':
      return 'is-danger'
    case 'running':
    case 'queued':
    case 'scoring':
    case 'pending':
      return 'is-active'
    default:
      return 'is-muted'
  }
}

function normalizeRunKind(value: unknown): HarnessRunKind | null {
  const normalized = String(value || '').trim()
  return normalized === 'agent_task' ||
    normalized === 'research' ||
    normalized === 'subagent' ||
    normalized === 'workflow'
    ? normalized
    : null
}

function normalizeScoringMode(value: unknown): HarnessScoringMode | null {
  const normalized = String(value || '').trim()
  return normalized === 'rule' || normalized === 'judge' || normalized === 'hybrid'
    ? normalized
    : null
}

function firstNonEmpty(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    const normalized = String(value || '').trim()
    if (normalized) return normalized
  }
  return ''
}

function readFiniteNumber(value: unknown): number | null {
  const normalized = Number(value)
  return Number.isFinite(normalized) ? normalized : null
}

function quickEvalDefaults(preset: QuickEvalPreset): {
  runKind: HarnessRunKind
  profile: string
  scoringMode: HarnessScoringMode
  passThreshold: number
} {
  switch (preset) {
    case 'research':
      return {
        runKind: 'research',
        profile: 'research',
        scoringMode: 'hybrid',
        passThreshold: 0.65,
      }
    case 'regression':
      return {
        runKind: 'agent_task',
        profile: 'regression',
        scoringMode: 'rule',
        passThreshold: 0.8,
      }
    default:
      return {
        runKind: 'agent_task',
        profile: 'smoke',
        scoringMode: 'rule',
        passThreshold: 0.5,
      }
  }
}

function supplementaryEntries(group: HarnessRunGroup): Array<{ key: string; value: string }> {
  const entries: Array<{ key: string; value: string }> = []
  const verificationPassRate = summaryValue(group, 'verification_pass_rate')
  const evidenceBackedPassRate = summaryValue(group, 'evidence_backed_pass_rate')
  const retryRecoveredCount = summaryValue(group, 'retry_recovered_count')

  if (verificationPassRate != null) {
    entries.push({
      key: tr('harness.group.verificationPassRate', 'Verification pass rate'),
      value: percentLabel(verificationPassRate),
    })
  }

  if (evidenceBackedPassRate != null) {
    entries.push({
      key: tr('harness.group.evidenceBackedPassRate', 'Evidence-backed pass rate'),
      value: percentLabel(evidenceBackedPassRate),
    })
  }

  if (retryRecoveredCount != null) {
    entries.push({
      key: tr('harness.group.retryRecovered', 'Retry recovered'),
      value: String(retryRecoveredCount),
    })
  }

  return entries
}

function buildQuickEvalGroupSpec(): HarnessRunGroupSpec {
  let parsed: unknown
  try {
    parsed = JSON.parse(quickEvalManifestText.value)
  } catch {
    throw new Error(tr('harness.quickEval.invalidManifest', 'Cases JSON must be valid JSON.'))
  }

  const manifest = asRecord(parsed)
  const dataset = asRecord(manifest?.dataset)
  const defaults = asRecord(manifest?.defaults)
  const scoring = asRecord(defaults?.scoring)
  const rawItems = Array.isArray(manifest?.items) ? manifest?.items : []

  if (!rawItems.length) {
    throw new Error(
      tr(
        'harness.quickEval.caseRequired',
        'Add at least one real case before launching a quick eval.'
      )
    )
  }

  const presetDefaults = quickEvalDefaults(quickEvalPreset.value)
  const runKind = normalizeRunKind(defaults?.run_kind) || presetDefaults.runKind
  const profile = firstNonEmpty(String(defaults?.profile || ''), presetDefaults.profile)
  const scoringMode = normalizeScoringMode(scoring?.mode) || presetDefaults.scoringMode
  const passThreshold = readFiniteNumber(scoring?.pass_threshold) ?? presetDefaults.passThreshold
  const subject = firstNonEmpty(String(dataset?.subject || ''), runKind)
  const datasetName = firstNonEmpty(
    String(dataset?.name || ''),
    tr('harness.quickEval.title', 'Quick Eval')
  )

  const items = rawItems
    .map((entry) => {
      const item = asRecord(entry)
      return {
        run_kind: normalizeRunKind(item?.run_kind) || runKind,
        profile: firstNonEmpty(String(item?.profile || ''), profile),
        input: asRecord(item?.input) || null,
        expected: asRecord(item?.expected) || null,
        metadata: asRecord(item?.metadata) || null,
      }
    })
    .filter((item) => item.input || item.expected)

  if (!items.length) {
    throw new Error(
      tr(
        'harness.quickEval.caseRequired',
        'Add at least one real case before launching a quick eval.'
      )
    )
  }

  return {
    kind: 'eval',
    subject,
    metadata: {
      quick_eval: true,
      ephemeral: true,
      source_mode: 'manifest',
      source_ref: 'ui',
      quick_eval_preset: quickEvalPreset.value,
      quick_eval_dataset_name: datasetName,
    },
    scoring: {
      mode: scoringMode,
      pass_threshold: passThreshold,
      rule_profile: profile,
    },
    items,
  }
}

async function submitQuickEval() {
  createAction.value = 'quick'
  try {
    const payload = buildQuickEvalGroupSpec()
    const response = await harnessApi.createGroup(payload)
    notification.success(
      tr('automation.tabs.harness', 'Harness'),
      tr('harness.quickEval.created', 'Quick eval launched')
    )
    await router.push({
      name: 'HarnessGroupDetail',
      params: { id: response.data.id },
    })
  } catch (err) {
    notification.error(tr('automation.tabs.harness', 'Harness'), getErrorMessage(err))
  } finally {
    createAction.value = ''
  }
}

function loadGroups(options: { silent?: boolean } = {}): Promise<void> {
  const { silent = false } = options
  if (silent) {
    refreshing.value = true
  } else {
    loading.value = true
    error.value = ''
  }

  return harnessApi
    .listGroups({ limit: 100 })
    .then((response) => {
      groups.value = response.data || []
      error.value = ''
    })
    .catch((err) => {
      error.value = getErrorMessage(err)
    })
    .finally(() => {
      loading.value = false
      refreshing.value = false
    })
}

const activeGroupCount = computed(
  () => groups.value.filter((group) => !terminalStatuses.has(group.status)).length
)

const averagePassRate = computed(() => {
  const passRates = groups.value
    .map((group) => summaryValue(group, 'pass_rate'))
    .filter((value): value is number => value != null)
  if (!passRates.length) return null
  return passRates.reduce((sum, value) => sum + value, 0) / passRates.length
})

const filteredGroups = computed(() => {
  const searchToken = groupSearch.value.trim().toLowerCase()
  return groups.value.filter((group) => {
    if (groupFilterMode.value === 'active' && terminalStatuses.has(group.status)) return false
    if (groupFilterMode.value === 'terminal' && !terminalStatuses.has(group.status)) return false

    if (!searchToken) return true

    const haystack = [
      group.id,
      group.title,
      group.subject,
      group.kind,
      group.status,
      group.owner_user_id,
    ]
      .map((value) => String(value || '').toLowerCase())
      .join(' ')

    return haystack.includes(searchToken)
  })
})

function groupCountEntries(group: HarnessRunGroup): NumberEntry[] {
  return [
    {
      key: tr('harness.groups.running', 'Running'),
      value:
        summaryCount(group, 'running') +
        summaryCount(group, 'queued') +
        summaryCount(group, 'pending'),
    },
    {
      key: tr('harness.groups.failed', 'Failed'),
      value: summaryCount(group, 'failed') + summaryCount(group, 'error'),
    },
    {
      key: tr('harness.groups.passed', 'Passed'),
      value: summaryCount(group, 'passed'),
    },
  ].filter((entry) => entry.value > 0)
}

onMounted(() => {
  void loadGroups()
  refreshTimer = setInterval(() => {
    void loadGroups({ silent: true })
  }, 10000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <div class="harness-groups-page">
    <section class="hero-card">
      <div>
        <p class="eyebrow">{{ tr('nav.automation', 'Automation') }}</p>
        <h1>{{ tr('automation.tabs.harness', 'Harness') }}</h1>
        <p class="hero-description">
          {{
            tr(
              'harness.groups.subtitle',
              'Browse eval groups and inspect the latest run outcomes without the heavier authoring workflow.'
            )
          }}
        </p>
      </div>
      <button
        class="refresh-button"
        type="button"
        :disabled="loading || refreshing"
        @click="loadGroups({ silent: true })"
      >
        {{ refreshing ? tr('common.loading', 'Loading') : tr('common.refresh', 'Refresh') }}
      </button>
    </section>

    <AutomationTabs class="automation-tab-strip" />

    <section class="stats-grid">
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.totalGroups', 'Groups') }}</span>
        <strong class="stat-value">{{ groups.length }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('common.active', 'Active') }}</span>
        <strong class="stat-value">{{ activeGroupCount }}</strong>
      </article>
      <article class="stat-card">
        <span class="stat-label">{{ tr('harness.groups.avgPassRate', 'Average pass rate') }}</span>
        <strong class="stat-value">{{ percentLabel(averagePassRate) }}</strong>
      </article>
    </section>

    <div v-if="error" class="state-card is-error">
      <h2>{{ tr('common.error', 'Error') }}</h2>
      <p>{{ error }}</p>
    </div>

    <div v-else-if="loading" class="state-card">
      <h2>{{ tr('common.loading', 'Loading') }}</h2>
      <p>{{ tr('harness.groups.loading', 'Fetching the latest harness group summaries.') }}</p>
    </div>

    <template v-else>
      <section class="quick-eval-card">
        <div class="quick-eval-copy">
          <p class="eyebrow">{{ tr('harness.quickEval.title', 'Quick Eval') }}</p>
          <h2>{{ tr('harness.quickEval.title', 'Quick Eval') }}</h2>
          <p class="hero-description">
            {{
              tr(
                'harness.quickEval.description',
                'Launch a lightweight eval group from a manifest without reopening the full builder.'
              )
            }}
          </p>
        </div>

        <form class="quick-eval-form" @submit.prevent="submitQuickEval">
          <label class="quick-eval-field">
            <span>{{ tr('harness.quickEval.preset', 'Preset') }}</span>
            <select v-model="quickEvalPreset">
              <option value="smoke">{{ tr('harness.quickEval.smokeLabel', 'Smoke') }}</option>
              <option value="regression">
                {{ tr('harness.quickEval.regressionLabel', 'Regression') }}
              </option>
              <option value="research">
                {{ tr('harness.quickEval.researchLabel', 'Research') }}
              </option>
            </select>
          </label>

          <label class="quick-eval-field quick-eval-field-wide">
            <span>{{ tr('harness.quickEval.caseManifest', 'Cases JSON') }}</span>
            <textarea
              v-model="quickEvalManifestText"
              name="quick-eval-manifest"
              rows="10"
              spellcheck="false"
              required
            />
            <small class="field-hint">
              {{
                tr(
                  'harness.quickEval.caseTemplateHint',
                  'Paste a dataset/defaults/items manifest here to launch a quick eval group.'
                )
              }}
            </small>
          </label>

          <div class="quick-eval-actions">
            <button type="submit" class="primary-button" :disabled="createAction === 'quick'">
              {{
                createAction === 'quick'
                  ? tr('common.loading', 'Loading')
                  : tr('harness.quickEval.launch', 'Launch quick eval')
              }}
            </button>
          </div>
        </form>
      </section>

      <section class="filter-card">
        <div class="filter-buttons" role="tablist" :aria-label="tr('common.search', 'Search')">
          <button
            type="button"
            class="filter-button"
            :class="{ active: groupFilterMode === 'all' }"
            @click="groupFilterMode = 'all'"
          >
            {{ tr('common.all', 'All') }}
          </button>
          <button
            type="button"
            class="filter-button"
            :class="{ active: groupFilterMode === 'active' }"
            @click="groupFilterMode = 'active'"
          >
            {{ tr('common.active', 'Active') }}
          </button>
          <button
            type="button"
            class="filter-button"
            :class="{ active: groupFilterMode === 'terminal' }"
            @click="groupFilterMode = 'terminal'"
          >
            {{ tr('harness.groups.terminalOnly', 'Terminal') }}
          </button>
        </div>

        <input
          v-model="groupSearch"
          class="search-input"
          type="search"
          :placeholder="
            tr('harness.groups.searchPlaceholder', 'Search title, subject, owner, kind, or status')
          "
        />
      </section>

      <section v-if="filteredGroups.length" class="group-list">
        <RouterLink
          v-for="group in filteredGroups"
          :key="group.id"
          class="group-card"
          :to="{ name: 'HarnessGroupDetail', params: { id: group.id } }"
        >
          <div class="group-card-header">
            <div class="group-main">
              <h2>{{ group.title || group.subject || group.id }}</h2>
              <p class="group-subject">
                {{ group.subject || tr('harness.groups.noSubject', 'No subject provided') }}
              </p>
            </div>
            <span class="status-chip" :class="statusTone(group.status)">
              {{ humanizeEnum(group.status) }}
            </span>
          </div>

          <dl class="group-meta">
            <div>
              <dt>{{ tr('harness.groups.kind', 'Kind') }}</dt>
              <dd>{{ humanizeEnum(group.kind) }}</dd>
            </div>
            <div>
              <dt>{{ tr('harness.groups.owner', 'Owner') }}</dt>
              <dd>{{ group.owner_user_id || tr('common.notAvailable', 'Not available') }}</dd>
            </div>
            <div>
              <dt>{{ tr('common.updatedAt', 'Updated') }}</dt>
              <dd>{{ formatDate(group.updated_at) }}</dd>
            </div>
          </dl>

          <div class="metric-grid">
            <div>
              <span>{{ tr('harness.groups.itemCount', 'Items') }}</span>
              <strong>{{ summaryValue(group, 'item_count') ?? 0 }}</strong>
            </div>
            <div>
              <span>{{ tr('harness.groups.passRate', 'Pass rate') }}</span>
              <strong>{{ percentLabel(summaryValue(group, 'pass_rate')) }}</strong>
            </div>
            <div>
              <span>{{ tr('harness.groups.score', 'Score') }}</span>
              <strong>{{ scoreLabel(summaryValue(group, 'overall_score')) }}</strong>
            </div>
          </div>

          <div v-if="supplementaryEntries(group).length" class="detail-pills">
            <span v-for="entry in supplementaryEntries(group)" :key="`${group.id}-${entry.key}`">
              {{ entry.key }}: {{ entry.value }}
            </span>
          </div>

          <div v-if="groupCountEntries(group).length" class="count-row">
            <span v-for="entry in groupCountEntries(group)" :key="`${group.id}-${entry.key}`">
              {{ entry.key }}: {{ entry.value }}
            </span>
          </div>
        </RouterLink>
      </section>

      <div v-else class="state-card">
        <h2>{{ tr('automation.tabs.harness', 'Harness') }}</h2>
        <p>
          {{
            tr(
              'harness.groups.emptyDescription',
              'No groups matched the current filters. Try a different search or wait for the next run.'
            )
          }}
        </p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.harness-groups-page {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.automation-tab-strip {
  position: relative;
  z-index: 1;
}

.hero-card,
.quick-eval-card,
.filter-card,
.state-card,
.stat-card,
.group-card {
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: rgba(15, 23, 42, 0.04);
  border-radius: 1rem;
}

.hero-card,
.quick-eval-card,
.filter-card,
.state-card {
  padding: 1.25rem;
}

.hero-card {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}

.eyebrow {
  margin: 0 0 0.35rem;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #2563eb;
}

.hero-card h1,
.state-card h2,
.group-card h2 {
  margin: 0;
}

.hero-description,
.state-card p,
.group-subject {
  margin: 0.5rem 0 0;
  color: rgba(15, 23, 42, 0.72);
  line-height: 1.55;
}

.refresh-button,
.filter-button,
.primary-button {
  border: 1px solid rgba(59, 130, 246, 0.22);
  background: white;
  color: #0f172a;
  border-radius: 999px;
  padding: 0.65rem 1rem;
  font: inherit;
  cursor: pointer;
}

.refresh-button:disabled,
.filter-button:disabled,
.primary-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.primary-button {
  background: #0f172a;
  border-color: #0f172a;
  color: white;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
  gap: 0.85rem;
}

.stat-card {
  padding: 1rem;
}

.stat-label {
  display: block;
  font-size: 0.88rem;
  color: rgba(15, 23, 42, 0.68);
}

.stat-value {
  display: block;
  margin-top: 0.3rem;
  font-size: 1.45rem;
}

.is-error {
  border-color: rgba(239, 68, 68, 0.28);
  background: rgba(239, 68, 68, 0.08);
}

.filter-card {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  justify-content: space-between;
  align-items: center;
}

.quick-eval-card {
  display: grid;
  grid-template-columns: minmax(220px, 0.95fr) minmax(0, 1.35fr);
  gap: 1rem;
}

.quick-eval-copy h2 {
  margin: 0;
}

.quick-eval-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
}

.quick-eval-field {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  min-width: 0;
}

.quick-eval-field span {
  font-size: 0.85rem;
  font-weight: 600;
  color: rgba(15, 23, 42, 0.78);
}

.quick-eval-field select,
.quick-eval-field textarea {
  width: 100%;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 0.8rem;
  padding: 0.75rem 0.9rem;
  font: inherit;
  background: white;
}

.quick-eval-field textarea {
  resize: vertical;
}

.quick-eval-field-wide {
  grid-column: 1 / -1;
}

.field-hint {
  color: rgba(15, 23, 42, 0.6);
  line-height: 1.45;
}

.quick-eval-actions {
  grid-column: 1 / -1;
  display: flex;
  justify-content: flex-end;
}

.filter-buttons {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.filter-button.active {
  background: #0f172a;
  border-color: #0f172a;
  color: white;
}

.search-input {
  flex: 1 1 280px;
  min-width: 220px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 0.8rem;
  padding: 0.75rem 0.9rem;
  font: inherit;
  background: white;
}

.group-list {
  display: grid;
  gap: 0.85rem;
}

.group-card {
  padding: 1.1rem;
  color: inherit;
  text-decoration: none;
  transition:
    transform 140ms ease,
    border-color 140ms ease,
    background 140ms ease;
}

.group-card:hover {
  transform: translateY(-1px);
  border-color: rgba(37, 99, 235, 0.28);
  background: rgba(37, 99, 235, 0.05);
}

.group-card-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  align-items: flex-start;
}

.group-main {
  min-width: 0;
}

.group-meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 0.75rem;
  margin: 1rem 0 0;
}

.group-meta dt,
.metric-grid span {
  font-size: 0.8rem;
  color: rgba(15, 23, 42, 0.62);
}

.group-meta dd {
  margin: 0.2rem 0 0;
  font-weight: 600;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0.75rem;
  margin-top: 1rem;
}

.metric-grid strong {
  display: block;
  margin-top: 0.2rem;
  font-size: 1.05rem;
}

.detail-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.7rem;
  margin-top: 1rem;
}

.detail-pills span {
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.8);
  padding: 0.35rem 0.65rem;
  font-size: 0.88rem;
  color: rgba(15, 23, 42, 0.72);
}

.count-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.85rem;
  margin-top: 1rem;
  font-size: 0.88rem;
  color: rgba(15, 23, 42, 0.72);
}

.status-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0.4rem 0.7rem;
  border-radius: 999px;
  font-size: 0.78rem;
  font-weight: 700;
  white-space: nowrap;
  background: rgba(148, 163, 184, 0.16);
  color: rgba(15, 23, 42, 0.75);
}

.status-chip.is-success {
  background: rgba(34, 197, 94, 0.16);
  color: #166534;
}

.status-chip.is-warning {
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

@media (max-width: 720px) {
  .hero-card,
  .quick-eval-card,
  .group-card-header,
  .filter-card {
    flex-direction: column;
    align-items: stretch;
  }

  .quick-eval-form {
    grid-template-columns: 1fr;
  }

  .quick-eval-actions {
    justify-content: stretch;
  }

  .primary-button {
    width: 100%;
  }

  .status-chip {
    align-self: flex-start;
  }
}
</style>
