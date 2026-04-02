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
  <div class="harness-groups-page dashboard-page-frame">
    <section class="automation-stage dashboard-page-stage configuration-page-stage">
      <section class="automation-hero dashboard-page-hero configuration-page-hero">
        <div class="automation-copy dashboard-page-copy configuration-page-copy">
          <p class="automation-kicker dashboard-page-eyebrow">
            {{ tr('nav.automation', 'Automation') }}
          </p>
          <h1 class="automation-title dashboard-page-title configuration-page-title">
            {{ tr('automation.tabs.harness', 'Harness') }}
          </h1>
          <p
            class="automation-description dashboard-page-description configuration-page-description"
          >
            {{
              tr(
                'harness.groups.subtitle',
                'Browse eval groups and inspect the latest run outcomes without the heavier authoring workflow.'
              )
            }}
          </p>
        </div>

        <div class="automation-hero-actions">
          <button
            class="toolbar-button"
            type="button"
            :disabled="loading || refreshing"
            @click="loadGroups({ silent: true })"
          >
            {{ refreshing ? tr('common.loading', 'Loading') : tr('common.refresh', 'Refresh') }}
          </button>
        </div>
      </section>

      <AutomationTabs class="automation-tab-strip" />

      <section class="automation-shell">
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
            <span class="stat-label">{{
              tr('harness.groups.avgPassRate', 'Average pass rate')
            }}</span>
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
                tr(
                  'harness.groups.searchPlaceholder',
                  'Search title, subject, owner, kind, or status'
                )
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
                <span
                  v-for="entry in supplementaryEntries(group)"
                  :key="`${group.id}-${entry.key}`"
                >
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
      </section>
    </section>
  </div>
</template>

<style scoped>
.harness-groups-page {
  --dashboard-page-accent: 37, 99, 235;
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}

.automation-stage {
  position: relative;
  padding: 1.15rem 0 0.35rem;
}

.automation-tab-strip {
  position: relative;
  z-index: 1;
}

.automation-hero {
  position: relative;
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 0 0 0.2rem;
}

.automation-copy {
  position: relative;
  z-index: 1;
  flex: 1 1 0%;
  min-width: 0;
  max-width: 42rem;
  padding-top: 0.1rem;
}

.automation-kicker {
  margin: 0;
}

.automation-title {
  margin: 0;
  font-size: clamp(1.34rem, 0.7vw + 0.95rem, 1.9rem);
  line-height: 1.06;
  letter-spacing: -0.04em;
  font-weight: 700;
  color: #111827;
}

.automation-description {
  margin: 0.42rem 0 0;
  max-width: 34rem;
  font-size: 0.92rem;
  line-height: 1.55;
  color: #9ca3af;
}

.automation-hero-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.75rem;
}

.automation-shell {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-width: 82rem;
  margin: 0 auto;
}

.quick-eval-card,
.filter-card,
.state-card {
  border: 1px solid rgba(203, 213, 225, 0.96);
  background: #f8fafc;
  border-radius: 1.5rem;
  box-shadow: none;
}

.stat-card,
.group-card {
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  border-radius: 1.25rem;
  box-shadow: none;
}

.quick-eval-card,
.filter-card,
.state-card {
  padding: 1.1rem;
}

.eyebrow {
  margin: 0 0 0.35rem;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgb(var(--dashboard-page-accent));
}

.state-card h2,
.group-card h2 {
  margin: 0;
}

.hero-description,
.state-card p,
.group-subject {
  margin: 0.5rem 0 0;
  color: #64748b;
  line-height: 1.55;
}

.toolbar-button,
.filter-button,
.primary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.65rem;
  border-radius: 999px;
  padding: 0.7rem 1rem;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background-color 0.18s ease,
    color 0.18s ease,
    opacity 0.18s ease;
}

.toolbar-button,
.filter-button {
  border: 1px solid rgba(226, 232, 240, 0.96);
  background: #ffffff;
  color: #475569;
}

.toolbar-button:hover:not(:disabled),
.filter-button:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: rgba(148, 163, 184, 0.52);
  background: #f8fafc;
}

.toolbar-button:disabled,
.filter-button:disabled,
.primary-button:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.primary-button {
  border: 1px solid #0f172a;
  background: #0f172a;
  color: white;
}

.primary-button:hover:not(:disabled) {
  transform: translateY(-1px);
  background: #1e293b;
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
  color: #64748b;
}

.stat-value {
  display: block;
  margin-top: 0.3rem;
  font-size: 1.45rem;
  color: #111827;
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
  color: #334155;
}

.quick-eval-field select,
.quick-eval-field textarea {
  width: 100%;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 0.8rem;
  padding: 0.75rem 0.9rem;
  font: inherit;
  background: #ffffff;
  color: #0f172a;
}

.quick-eval-field textarea {
  resize: vertical;
}

.quick-eval-field-wide {
  grid-column: 1 / -1;
}

.field-hint {
  color: #64748b;
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
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 0.8rem;
  padding: 0.75rem 0.9rem;
  font: inherit;
  background: #ffffff;
  color: #0f172a;
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
  border-color: rgba(148, 163, 184, 0.58);
  background: #f8fafc;
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

.group-main h2 {
  margin: 0;
  color: #111827;
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
  color: #64748b;
}

.group-meta dd {
  margin: 0.2rem 0 0;
  font-weight: 600;
  color: #0f172a;
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
  color: #111827;
}

.detail-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.7rem;
  margin-top: 1rem;
}

.detail-pills span {
  border-radius: 999px;
  background: #f8fafc;
  border: 1px solid rgba(226, 232, 240, 0.96);
  padding: 0.35rem 0.65rem;
  font-size: 0.88rem;
  color: #475569;
}

.count-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.85rem;
  margin-top: 1rem;
  font-size: 0.88rem;
  color: #475569;
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
  background: rgba(148, 163, 184, 0.14);
  color: #475569;
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

:root.dark .automation-title,
[data-theme='dark'] .automation-title,
html.dark .automation-title,
:root.dark .state-card h2,
[data-theme='dark'] .state-card h2,
html.dark .state-card h2,
:root.dark .group-main h2,
[data-theme='dark'] .group-main h2,
html.dark .group-main h2,
:root.dark .stat-value,
[data-theme='dark'] .stat-value,
html.dark .stat-value,
:root.dark .metric-grid strong,
[data-theme='dark'] .metric-grid strong,
html.dark .metric-grid strong,
:root.dark .group-meta dd,
[data-theme='dark'] .group-meta dd,
html.dark .group-meta dd {
  color: #f8fafc;
}

:root.dark .automation-description,
[data-theme='dark'] .automation-description,
html.dark .automation-description,
:root.dark .hero-description,
[data-theme='dark'] .hero-description,
html.dark .hero-description,
:root.dark .state-card p,
[data-theme='dark'] .state-card p,
html.dark .state-card p,
:root.dark .group-subject,
[data-theme='dark'] .group-subject,
html.dark .group-subject,
:root.dark .stat-label,
[data-theme='dark'] .stat-label,
html.dark .stat-label,
:root.dark .group-meta dt,
[data-theme='dark'] .group-meta dt,
html.dark .group-meta dt,
:root.dark .metric-grid span,
[data-theme='dark'] .metric-grid span,
html.dark .metric-grid span,
:root.dark .field-hint,
[data-theme='dark'] .field-hint,
html.dark .field-hint,
:root.dark .quick-eval-field span,
[data-theme='dark'] .quick-eval-field span,
html.dark .quick-eval-field span,
:root.dark .count-row,
[data-theme='dark'] .count-row,
html.dark .count-row {
  color: rgba(226, 232, 240, 0.72);
}

:root.dark .toolbar-button,
[data-theme='dark'] .toolbar-button,
html.dark .toolbar-button,
:root.dark .filter-button,
[data-theme='dark'] .filter-button,
html.dark .filter-button,
:root.dark .search-input,
[data-theme='dark'] .search-input,
html.dark .search-input,
:root.dark .quick-eval-field select,
[data-theme='dark'] .quick-eval-field select,
html.dark .quick-eval-field select,
:root.dark .quick-eval-field textarea,
[data-theme='dark'] .quick-eval-field textarea,
html.dark .quick-eval-field textarea {
  border-color: rgba(71, 85, 105, 0.78);
  background: rgba(15, 23, 42, 0.82);
  color: #e2e8f0;
}

:root.dark .toolbar-button:hover:not(:disabled),
[data-theme='dark'] .toolbar-button:hover:not(:disabled),
html.dark .toolbar-button:hover:not(:disabled),
:root.dark .filter-button:hover:not(:disabled),
[data-theme='dark'] .filter-button:hover:not(:disabled),
html.dark .filter-button:hover:not(:disabled),
:root.dark .group-card:hover,
[data-theme='dark'] .group-card:hover,
html.dark .group-card:hover {
  background: rgba(30, 41, 59, 0.9);
}

:root.dark .quick-eval-card,
[data-theme='dark'] .quick-eval-card,
html.dark .quick-eval-card,
:root.dark .filter-card,
[data-theme='dark'] .filter-card,
html.dark .filter-card,
:root.dark .state-card,
[data-theme='dark'] .state-card,
html.dark .state-card {
  border-color: rgba(71, 85, 105, 0.78);
  background: rgba(15, 23, 42, 0.72);
}

:root.dark .stat-card,
[data-theme='dark'] .stat-card,
html.dark .stat-card,
:root.dark .group-card,
[data-theme='dark'] .group-card,
html.dark .group-card {
  border-color: rgba(71, 85, 105, 0.78);
  background: rgba(15, 23, 42, 0.88);
}

:root.dark .detail-pills span,
[data-theme='dark'] .detail-pills span,
html.dark .detail-pills span {
  border-color: rgba(71, 85, 105, 0.7);
  background: rgba(30, 41, 59, 0.92);
  color: rgba(226, 232, 240, 0.82);
}

:root.dark .filter-button.active,
[data-theme='dark'] .filter-button.active,
html.dark .filter-button.active,
:root.dark .primary-button,
[data-theme='dark'] .primary-button,
html.dark .primary-button {
  border-color: rgba(241, 245, 249, 0.16);
  background: #f8fafc;
  color: #0f172a;
}

@media (max-width: 720px) {
  .automation-hero,
  .group-card-header,
  .filter-card {
    flex-direction: column;
    align-items: stretch;
  }

  .quick-eval-card {
    grid-template-columns: 1fr;
  }

  .quick-eval-form {
    grid-template-columns: 1fr;
  }

  .quick-eval-actions {
    justify-content: stretch;
  }

  .toolbar-button,
  .primary-button {
    width: 100%;
  }

  .automation-hero-actions {
    width: 100%;
  }

  .status-chip {
    align-self: flex-start;
  }
}
</style>
