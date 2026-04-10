<script setup lang="ts">
import type {
  HarnessArtifactRef,
  HarnessRun,
  HarnessRunDetail,
  HarnessRunSummary,
  HarnessRunTrace,
} from '@/api/harness'

import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { compareHarnessRuns, deriveHarnessRunPreview } from '@/utils/harnessRunTree'

const props = withDefaults(
  defineProps<{
    open?: boolean
    mobile?: boolean
    run?: HarnessRun | HarnessRunSummary | null
    detail?: HarnessRunDetail | null
    childRuns?: HarnessRunSummary[]
    loading?: boolean
    error?: string
    warning?: string
  }>(),
  {
    open: false,
    mobile: false,
    run: null,
    detail: null,
    childRuns: () => [],
    loading: false,
    error: '',
    warning: '',
  }
)

const emit = defineEmits<{
  close: []
}>()

const { t, te } = useI18n()
const showAllEvents = ref(false)

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function humanizeEnum(value?: string | null): string {
  const normalized = String(value || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')
  return normalized.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function compactValue(value: unknown): string {
  if (value == null || value === '') return tr('common.notAvailable', 'Not available')
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch {
      return tr('common.notAvailable', 'Not available')
    }
  }
  return String(value)
}

function compactJSON(value: unknown): string {
  if (!value || typeof value !== 'object') return ''
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return ''
  }
}

function formatLatency(value?: number | null): string {
  if (value == null || !Number.isFinite(value) || value < 0) {
    return tr('common.notAvailable', 'Not available')
  }
  if (value < 1000) return `${Math.round(value)} ms`
  return `${(value / 1000).toFixed(2)} s`
}

function artifactIsURL(artifact: HarnessArtifactRef): boolean {
  return /^(https?:)?\/\//.test(String(artifact.path_or_url || '').trim())
}

function statusTone(status?: string | null): string {
  const normalized = String(status || '').trim()
  switch (normalized) {
    case 'completed':
      return 'tone-success'
    case 'pending':
    case 'queued':
    case 'planning':
    case 'executing':
    case 'verifying':
      return 'tone-running'
    case 'waiting_input':
      return 'tone-blocked'
    case 'failed':
    case 'cancelled':
    case 'aborted':
    case 'error':
      return 'tone-danger'
    default:
      return 'tone-muted'
  }
}

function statusLabel(status?: string | null): string {
  const normalized = String(status || '').trim()
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
    default:
      return humanizeEnum(normalized)
  }
}

function runtimeStateLabel(runtimeState?: string | null): string {
  const normalized = String(runtimeState || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')
  switch (normalized) {
    case 'pending':
      return tr('common.taskRuntimePending', 'Pending')
    case 'intake':
      return tr('common.taskRuntimeIntake', 'Intake')
    case 'clarify':
      return tr('common.taskRuntimeClarify', 'Clarifying')
    case 'plan':
      return tr('common.taskRuntimePlan', 'Planning')
    case 'confirm_gate':
      return tr('common.taskRuntimeConfirmGate', 'Waiting for confirmation')
    case 'execute':
      return tr('common.taskRuntimeExecute', 'Executing')
    case 'verify':
      return tr('common.taskRuntimeVerify', 'Verifying')
    case 'reflect':
      return tr('common.taskRuntimeReflect', 'Reflecting')
    case 'report':
      return tr('common.taskRuntimeReport', 'Preparing report')
    case 'recover':
      return tr('common.taskRuntimeRecover', 'Recovering')
    case 'done':
      return tr('common.taskRuntimeDone', 'Completed')
    case 'aborted':
      return tr('common.taskRuntimeAborted', 'Aborted')
    default:
      return humanizeEnum(normalized)
  }
}

function metadataRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function metadataRecords(value: unknown): Record<string, unknown>[] {
  if (!Array.isArray(value)) return []
  return value
    .map((item) => metadataRecord(item))
    .filter((item): item is Record<string, unknown> => !!item)
}

function metadataText(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return ''
}

function metadataCount(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

function metadataFlag(value: unknown): boolean {
  return value === true
}

const overviewRun = computed<HarnessRun | null>(() => {
  if (!props.run && !props.detail?.run) return null
  return {
    ...(props.detail?.run || {}),
    ...(props.run || {}),
  } as HarnessRun
})

const sortedChildRuns = computed(() => [...props.childRuns].sort(compareHarnessRuns))
const timelineEvents = computed(() =>
  [...(props.detail?.events || [])].sort((left, right) => {
    const leftTime = Date.parse(left.created_at || '')
    const rightTime = Date.parse(right.created_at || '')
    return rightTime - leftTime
  })
)
const recentTimelineEvents = computed(() => timelineEvents.value.slice(0, 5))
const remainingTimelineEvents = computed(() => timelineEvents.value.slice(5))
const artifacts = computed(() => props.detail?.artifacts || [])
const pendingApprovals = computed(() => props.detail?.pending_approvals || [])
const pendingQuestions = computed(() => props.detail?.pending_questions || [])
const pendingCount = computed(() => pendingApprovals.value.length + pendingQuestions.value.length)
const contextPackSnapshot = computed<Record<string, unknown> | null>(() =>
  metadataRecord(overviewRun.value?.metadata?.contextpack_snapshot)
)
const contextPackFiles = computed<Record<string, unknown>[]>(() =>
  metadataRecords(contextPackSnapshot.value?.files)
)
const contextPackSelectionDigest = computed(() =>
  metadataText(contextPackSnapshot.value?.selection_digest)
)
const contextPackSummaryPills = computed(() => {
  const snapshot = contextPackSnapshot.value
  if (!snapshot) return []

  const pills: string[] = []
  const selectedCount = metadataCount(snapshot.selected_count)
  if (selectedCount != null) {
    pills.push(`${tr('harness.group.contextPackSelectedCount', 'Selected')}: ${selectedCount}`)
  }
  const totalTokens = metadataCount(snapshot.total_tokens)
  if (totalTokens != null) {
    pills.push(`${tr('harness.group.contextPackTotalTokens', 'Tokens')}: ${totalTokens}`)
  }
  const selectedSkill = metadataText(snapshot.selected_skill)
  if (selectedSkill) {
    pills.push(`${tr('harness.group.contextPackSelectedSkill', 'Skill')}: ${selectedSkill}`)
  }
  if (metadataFlag(snapshot.truncated)) {
    pills.push(tr('harness.group.contextPackTruncated', 'Truncated'))
  }
  return pills
})
const runTrace = computed<HarnessRunTrace | null>(() => props.detail?.run_trace || null)
const traceStages = computed(() => runTrace.value?.stages || [])
const traceEvents = computed(() => runTrace.value?.events || [])
const traceArtifacts = computed(() => runTrace.value?.artifacts || [])
const traceSummaryPills = computed(() => {
  if (!runTrace.value) return []

  const pills = [
    `${tr('harness.group.traceStageCount', 'Stages')}: ${traceStages.value.length}`,
    `${tr('harness.group.traceEventCount', 'Events')}: ${traceEvents.value.length}`,
    `${tr('harness.group.traceArtifactCount', 'Trace artifacts')}: ${traceArtifacts.value.length}`,
  ]

  if (runTrace.value.latency_ms != null) {
    pills.push(`${tr('harness.group.traceLatency', 'Latency')}: ${formatLatency(runTrace.value.latency_ms)}`)
  }

  return pills
})

const roleLabel = computed(() => {
  if (overviewRun.value?.parent_run_id) return tr('harness.group.workerRole', 'Worker')
  if (sortedChildRuns.value.length > 0) return tr('harness.group.coordinatorRole', 'Coordinator')
  return ''
})

const runSummaryPreview = computed(() => deriveHarnessRunPreview(overviewRun.value, timelineEvents.value))

const spawnNarrative = computed(() => {
  if (!sortedChildRuns.value.length) return ''
  const names = sortedChildRuns.value
    .slice(0, 3)
    .map((run) => String(run.agent_id || run.kind || run.id).trim())
    .filter(Boolean)
  const prefix =
    sortedChildRuns.value.length === 1
      ? tr('harness.group.spawnedOneWorker', 'Spawned 1 worker')
      : `Spawned ${sortedChildRuns.value.length} workers`
  return names.length ? `${prefix}: ${names.join(', ')}` : prefix
})

const overviewEntries = computed(() => {
  const run = overviewRun.value
  if (!run) return []

  return [
    { label: tr('harness.groups.owner', 'Agent'), value: compactValue(run.agent_id) },
    { label: tr('harness.group.model', 'Model'), value: compactValue(run.model) },
    { label: tr('harness.groups.kind', 'Kind'), value: humanizeEnum(run.kind) },
    { label: tr('harness.groups.status', 'Status'), value: statusLabel(run.status) },
    {
      label: tr('harness.group.runtimeState', 'Runtime state'),
      value: runtimeStateLabel(run.runtime_state),
    },
    { label: tr('harness.group.depth', 'Depth'), value: compactValue(run.depth ?? 0) },
    { label: tr('harness.group.progress', 'Progress'), value: compactValue(run.progress ?? 0) },
    {
      label: tr('harness.group.sandboxMode', 'Sandbox mode'),
      value: compactValue(run.sandbox_mode),
    },
    {
      label: tr('harness.group.approvalMode', 'Approval mode'),
      value: compactValue(run.approval_mode),
    },
    {
      label: tr('harness.group.workspaceRoot', 'Workspace root'),
      value: compactValue(run.workspace_root),
    },
  ]
})

watch(
  () => overviewRun.value?.id,
  () => {
    showAllEvents.value = false
  }
)

function closeDrawer() {
  emit('close')
}
</script>

<template>
  <Transition name="sheet">
    <div v-if="props.mobile && props.open" class="detail-overlay" @click.self="closeDrawer">
      <section class="detail-panel detail-sheet">
        <header class="detail-header">
          <div>
            <p class="detail-kicker">{{ tr('harness.group.runDetail', 'Run detail') }}</p>
            <h3>{{ overviewRun?.agent_id || overviewRun?.goal || overviewRun?.id }}</h3>
            <p class="detail-subtitle">{{ overviewRun?.id }}</p>
          </div>
          <button type="button" class="close-button" @click="closeDrawer">
            {{ tr('common.cancel', 'Close') }}
          </button>
        </header>
        <div class="detail-body">
          <div v-if="props.warning" class="detail-banner is-warning">{{ props.warning }}</div>
          <div v-if="props.error && !props.detail" class="detail-banner is-error">{{ props.error }}</div>
          <div v-if="props.loading && !props.detail" class="detail-state">
            {{ tr('common.loading', 'Loading') }}
          </div>
          <div v-else-if="!overviewRun" class="detail-state">
            {{ tr('harness.group.selectRunPrompt', 'Select a run to inspect its timeline and artifacts.') }}
          </div>
          <template v-else>
            <section class="detail-section summary-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.runSummary', 'Run summary') }}</h4>
              </div>
              <div class="summary-title-row">
                <strong>{{ overviewRun.agent_id || overviewRun.goal || overviewRun.id }}</strong>
                <span v-if="roleLabel" class="summary-pill role-pill">{{ roleLabel }}</span>
                <span class="summary-pill" :class="statusTone(overviewRun.status)">
                  {{ statusLabel(overviewRun.status) }}
                </span>
              </div>
              <p class="summary-goal">
                {{
                  overviewRun.goal ||
                  tr('harness.group.noGoalSummary', 'No goal summary was recorded for this run.')
                }}
              </p>
              <p v-if="runSummaryPreview" class="summary-preview">{{ runSummaryPreview }}</p>
              <p v-if="spawnNarrative" class="spawn-narrative">{{ spawnNarrative }}</p>
              <div class="summary-pills">
                <span v-if="overviewRun.model">{{ tr('harness.group.model', 'Model') }}: {{ overviewRun.model }}</span>
                <span>{{ tr('common.updatedAt', 'Updated') }}: {{ formatDate(overviewRun.updated_at) }}</span>
                <span v-if="overviewRun.runtime_state">
                  {{ tr('harness.group.runtimeState', 'Runtime state') }}:
                  {{ runtimeStateLabel(overviewRun.runtime_state) }}
                </span>
                <span v-if="overviewRun.progress != null">
                  {{ tr('harness.group.progress', 'Progress') }}: {{ overviewRun.progress }}
                </span>
              </div>
            </section>

            <section class="detail-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.overview', 'Overview') }}</h4>
              </div>
              <dl class="overview-grid">
                <div v-for="entry in overviewEntries" :key="entry.label">
                  <dt>{{ entry.label }}</dt>
                  <dd>{{ entry.value }}</dd>
                </div>
              </dl>
            </section>

            <section v-if="contextPackSnapshot" class="detail-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.contextPacks', 'Context Packs') }}</h4>
                <span>{{ contextPackFiles.length }}</span>
              </div>
              <div v-if="contextPackSummaryPills.length" class="summary-pills">
                <span v-for="pill in contextPackSummaryPills" :key="pill">{{ pill }}</span>
              </div>
              <p v-if="contextPackSelectionDigest" class="summary-preview">
                {{ tr('harness.group.contextPackSelectionDigest', 'Selection digest') }}:
                <code>{{ contextPackSelectionDigest }}</code>
              </p>
              <div v-if="contextPackFiles.length === 0" class="detail-state">
                {{ tr('harness.group.noContextPacks', 'No context pack files were captured for this run.') }}
              </div>
              <div v-else class="artifact-list">
                <article
                  v-for="(file, index) in contextPackFiles"
                  :key="`${metadataText(file.entry_id)}-${metadataText(file.file)}-${index}`"
                  class="artifact-card"
                >
                  <div>
                    <strong>{{ metadataText(file.entry_id) || tr('harness.group.contextPackEntryId', 'Entry') }}</strong>
                    <p>
                      {{ tr('harness.group.contextPackFile', 'File') }}:
                      {{ metadataText(file.file) || tr('common.notAvailable', 'Not available') }}
                    </p>
                  </div>
                  <div class="event-pills">
                    <span v-if="metadataText(file.source_trust)">
                      {{ tr('harness.group.contextPackSourceTrust', 'Trust') }}: {{ metadataText(file.source_trust) }}
                    </span>
                    <span v-if="metadataText(file.language)">
                      {{ tr('harness.group.contextPackLanguage', 'Language') }}: {{ metadataText(file.language) }}
                    </span>
                    <span v-if="metadataText(file.version)">
                      {{ tr('harness.group.contextPackVersion', 'Version') }}: {{ metadataText(file.version) }}
                    </span>
                    <span v-if="metadataFlag(file.annotated)">
                      {{ tr('harness.group.contextPackAnnotated', 'Annotated') }}
                    </span>
                    <span v-if="metadataFlag(file.truncated)">
                      {{ tr('harness.group.contextPackTruncated', 'Truncated') }}
                    </span>
                  </div>
                  <code v-if="metadataText(file.sha256)">{{ metadataText(file.sha256) }}</code>
                </article>
              </div>
            </section>

            <section class="detail-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.runtimeTrace', 'Runtime trace') }}</h4>
                <span v-if="runTrace">{{ traceStages.length }}</span>
              </div>
              <div v-if="!runTrace" class="detail-state">
                {{ tr('harness.group.noRuntimeTrace', 'No runtime trace snapshot is available for this run yet.') }}
              </div>
              <template v-else>
                <div class="summary-pills">
                  <span v-for="pill in traceSummaryPills" :key="pill">{{ pill }}</span>
                </div>
                <div v-if="traceStages.length" class="trace-stage-list">
                  <article v-for="stage in traceStages" :key="`${stage.stage}-${stage.created_at}`" class="trace-stage-card">
                    <div class="event-header">
                      <strong>{{ humanizeEnum(stage.stage) }}</strong>
                      <span>{{ formatDate(stage.created_at) }}</span>
                    </div>
                    <p v-if="stage.message" class="event-message">{{ stage.message }}</p>
                    <div class="event-pills">
                      <span v-if="stage.status">{{ statusLabel(stage.status) }}</span>
                    </div>
                    <details v-if="stage.details" class="payload-block">
                      <summary>{{ tr('harness.group.traceDetails', 'Trace details') }}</summary>
                      <pre>{{ compactJSON(stage.details) }}</pre>
                    </details>
                  </article>
                </div>
              </template>
            </section>

            <section class="detail-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.timeline', 'Timeline') }}</h4>
                <span>{{ timelineEvents.length }}</span>
              </div>
              <div v-if="timelineEvents.length === 0" class="detail-state">
                {{ tr('harness.group.noTimelineEvents', 'No timeline events recorded for this run.') }}
              </div>
              <div v-else class="event-list">
                <article v-for="event in recentTimelineEvents" :key="event.id" class="event-card">
                  <div class="event-header">
                    <strong>{{ event.message || humanizeEnum(event.type) }}</strong>
                    <span>{{ formatDate(event.created_at) }}</span>
                  </div>
                  <div class="event-pills">
                    <span>{{ humanizeEnum(event.type) }}</span>
                    <span v-if="event.tool_name">{{ tr('harness.group.toolName', 'Tool') }}: {{ event.tool_name }}</span>
                    <span v-if="event.capability_kind">
                      {{ tr('harness.group.capability', 'Capability') }}:
                      {{ humanizeEnum(event.capability_kind) }}
                    </span>
                  </div>
                  <details v-if="event.payload_json" class="payload-block">
                    <summary>{{ tr('harness.group.rawPayload', 'Raw JSON') }}</summary>
                    <pre>{{ event.payload_json }}</pre>
                  </details>
                </article>

                <button
                  v-if="remainingTimelineEvents.length && !showAllEvents"
                  type="button"
                  class="detail-link"
                  @click="showAllEvents = true"
                >
                  {{ tr('harness.group.showAllEvents', 'Show all events') }}
                </button>

                <template v-if="showAllEvents">
                  <article
                    v-for="event in remainingTimelineEvents"
                    :key="`extra-${event.id}`"
                    class="event-card"
                  >
                    <div class="event-header">
                      <strong>{{ event.message || humanizeEnum(event.type) }}</strong>
                      <span>{{ formatDate(event.created_at) }}</span>
                    </div>
                    <div class="event-pills">
                      <span>{{ humanizeEnum(event.type) }}</span>
                      <span v-if="event.tool_name">{{ tr('harness.group.toolName', 'Tool') }}: {{ event.tool_name }}</span>
                      <span v-if="event.capability_kind">
                        {{ tr('harness.group.capability', 'Capability') }}:
                        {{ humanizeEnum(event.capability_kind) }}
                      </span>
                    </div>
                    <details v-if="event.payload_json" class="payload-block">
                      <summary>{{ tr('harness.group.rawPayload', 'Raw JSON') }}</summary>
                      <pre>{{ event.payload_json }}</pre>
                    </details>
                  </article>
                  <button type="button" class="detail-link" @click="showAllEvents = false">
                    {{ tr('harness.group.collapseEvents', 'Show fewer events') }}
                  </button>
                </template>
              </div>
            </section>

            <section class="detail-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.artifacts', 'Artifacts') }}</h4>
                <span>{{ artifacts.length }}</span>
              </div>
              <div v-if="artifacts.length === 0" class="detail-state">
                {{ tr('harness.group.noArtifacts', 'No artifacts attached to the linked runs yet.') }}
              </div>
              <div v-else class="artifact-list">
                <article v-for="artifact in artifacts" :key="artifact.id" class="artifact-card">
                  <div>
                    <strong>{{ artifact.label || humanizeEnum(artifact.kind) }}</strong>
                    <p>{{ humanizeEnum(artifact.kind) }} · {{ compactValue(artifact.mime_type) }}</p>
                  </div>
                  <a
                    v-if="artifact.path_or_url && artifactIsURL(artifact)"
                    :href="artifact.path_or_url"
                    target="_blank"
                    rel="noreferrer"
                  >
                    {{ artifact.path_or_url }}
                  </a>
                  <code v-else>{{ compactValue(artifact.path_or_url) }}</code>
                </article>
              </div>
            </section>

            <section class="detail-section">
              <div class="section-header">
                <h4>{{ tr('harness.group.pending', 'Pending') }}</h4>
                <span>{{ pendingCount }}</span>
              </div>
              <div v-if="pendingCount === 0" class="detail-state">
                {{ tr('harness.group.noPendingItems', 'No pending approvals or questions for this run.') }}
              </div>
              <div v-else class="pending-stack">
                <article class="pending-card">
                  <div class="section-header compact">
                    <strong>{{ tr('harness.group.pendingApprovals', 'Pending approvals') }}</strong>
                    <span>{{ pendingApprovals.length }}</span>
                  </div>
                  <div v-if="pendingApprovals.length === 0" class="detail-state compact">
                    {{ tr('harness.group.noPendingApprovals', 'No pending approvals.') }}
                  </div>
                  <pre
                    v-for="(approval, index) in pendingApprovals"
                    :key="`approval-${index}`"
                    class="pending-json"
                  >{{ JSON.stringify(approval, null, 2) }}</pre>
                </article>

                <article class="pending-card">
                  <div class="section-header compact">
                    <strong>{{ tr('harness.group.pendingQuestions', 'Pending questions') }}</strong>
                    <span>{{ pendingQuestions.length }}</span>
                  </div>
                  <div v-if="pendingQuestions.length === 0" class="detail-state compact">
                    {{ tr('harness.group.noPendingQuestions', 'No pending questions.') }}
                  </div>
                  <pre
                    v-for="(question, index) in pendingQuestions"
                    :key="`question-${index}`"
                    class="pending-json"
                  >{{ JSON.stringify(question, null, 2) }}</pre>
                </article>
              </div>
            </section>
          </template>
        </div>
      </section>
    </div>
  </Transition>

  <aside v-if="!props.mobile" class="detail-drawer">
    <section class="detail-panel harness-run-detail-drawer">
      <header class="detail-header">
        <div>
          <p class="detail-kicker">{{ tr('harness.group.runDetail', 'Run detail') }}</p>
          <h3 v-if="overviewRun">{{ overviewRun.agent_id || overviewRun.goal || overviewRun.id }}</h3>
          <h3 v-else>{{ tr('harness.group.selectRun', 'Select a run') }}</h3>
          <p class="detail-subtitle">
            {{ overviewRun?.id || tr('harness.group.selectRunPrompt', 'Select a run to inspect its timeline and artifacts.') }}
          </p>
        </div>
        <button v-if="props.open" type="button" class="close-button" @click="closeDrawer">
          {{ tr('common.cancel', 'Close') }}
        </button>
      </header>
      <div class="detail-body">
        <div v-if="props.warning" class="detail-banner is-warning">{{ props.warning }}</div>
        <div v-if="props.error && !props.detail" class="detail-banner is-error">{{ props.error }}</div>
        <div v-if="props.loading && !props.detail" class="detail-state">
          {{ tr('common.loading', 'Loading') }}
        </div>
        <div v-else-if="!props.open || !overviewRun" class="detail-state">
          {{ tr('harness.group.selectRunPrompt', 'Select a run to inspect its timeline and artifacts.') }}
        </div>
        <template v-else>
          <section class="detail-section summary-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.runSummary', 'Run summary') }}</h4>
            </div>
            <div class="summary-title-row">
                <strong>{{ overviewRun.agent_id || overviewRun.goal || overviewRun.id }}</strong>
                <span v-if="roleLabel" class="summary-pill role-pill">{{ roleLabel }}</span>
                <span class="summary-pill" :class="statusTone(overviewRun.status)">
                  {{ statusLabel(overviewRun.status) }}
                </span>
              </div>
            <p class="summary-goal">
              {{
                overviewRun.goal ||
                tr('harness.group.noGoalSummary', 'No goal summary was recorded for this run.')
              }}
            </p>
            <p v-if="runSummaryPreview" class="summary-preview">{{ runSummaryPreview }}</p>
            <p v-if="spawnNarrative" class="spawn-narrative">{{ spawnNarrative }}</p>
            <div class="summary-pills">
              <span v-if="overviewRun.model">{{ tr('harness.group.model', 'Model') }}: {{ overviewRun.model }}</span>
              <span>{{ tr('common.updatedAt', 'Updated') }}: {{ formatDate(overviewRun.updated_at) }}</span>
                <span v-if="overviewRun.runtime_state">
                  {{ tr('harness.group.runtimeState', 'Runtime state') }}:
                  {{ runtimeStateLabel(overviewRun.runtime_state) }}
                </span>
                <span v-if="overviewRun.progress != null">
                  {{ tr('harness.group.progress', 'Progress') }}: {{ overviewRun.progress }}
                </span>
              </div>
          </section>

          <section class="detail-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.overview', 'Overview') }}</h4>
            </div>
            <dl class="overview-grid">
              <div v-for="entry in overviewEntries" :key="entry.label">
                <dt>{{ entry.label }}</dt>
                <dd>{{ entry.value }}</dd>
              </div>
            </dl>
          </section>

          <section v-if="contextPackSnapshot" class="detail-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.contextPacks', 'Context Packs') }}</h4>
              <span>{{ contextPackFiles.length }}</span>
            </div>
            <div v-if="contextPackSummaryPills.length" class="summary-pills">
              <span v-for="pill in contextPackSummaryPills" :key="pill">{{ pill }}</span>
            </div>
            <p v-if="contextPackSelectionDigest" class="summary-preview">
              {{ tr('harness.group.contextPackSelectionDigest', 'Selection digest') }}:
              <code>{{ contextPackSelectionDigest }}</code>
            </p>
            <div v-if="contextPackFiles.length === 0" class="detail-state">
              {{ tr('harness.group.noContextPacks', 'No context pack files were captured for this run.') }}
            </div>
            <div v-else class="artifact-list">
              <article
                v-for="(file, index) in contextPackFiles"
                :key="`${metadataText(file.entry_id)}-${metadataText(file.file)}-${index}`"
                class="artifact-card"
              >
                <div>
                  <strong>{{ metadataText(file.entry_id) || tr('harness.group.contextPackEntryId', 'Entry') }}</strong>
                  <p>
                    {{ tr('harness.group.contextPackFile', 'File') }}:
                    {{ metadataText(file.file) || tr('common.notAvailable', 'Not available') }}
                  </p>
                </div>
                <div class="event-pills">
                  <span v-if="metadataText(file.source_trust)">
                    {{ tr('harness.group.contextPackSourceTrust', 'Trust') }}: {{ metadataText(file.source_trust) }}
                  </span>
                  <span v-if="metadataText(file.language)">
                    {{ tr('harness.group.contextPackLanguage', 'Language') }}: {{ metadataText(file.language) }}
                  </span>
                  <span v-if="metadataText(file.version)">
                    {{ tr('harness.group.contextPackVersion', 'Version') }}: {{ metadataText(file.version) }}
                  </span>
                  <span v-if="metadataFlag(file.annotated)">
                    {{ tr('harness.group.contextPackAnnotated', 'Annotated') }}
                  </span>
                  <span v-if="metadataFlag(file.truncated)">
                    {{ tr('harness.group.contextPackTruncated', 'Truncated') }}
                  </span>
                </div>
                <code v-if="metadataText(file.sha256)">{{ metadataText(file.sha256) }}</code>
              </article>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.runtimeTrace', 'Runtime trace') }}</h4>
              <span v-if="runTrace">{{ traceStages.length }}</span>
            </div>
            <div v-if="!runTrace" class="detail-state">
              {{ tr('harness.group.noRuntimeTrace', 'No runtime trace snapshot is available for this run yet.') }}
            </div>
            <template v-else>
              <div class="summary-pills">
                <span v-for="pill in traceSummaryPills" :key="pill">{{ pill }}</span>
              </div>
              <div v-if="traceStages.length" class="trace-stage-list">
                <article v-for="stage in traceStages" :key="`${stage.stage}-${stage.created_at}`" class="trace-stage-card">
                  <div class="event-header">
                    <strong>{{ humanizeEnum(stage.stage) }}</strong>
                    <span>{{ formatDate(stage.created_at) }}</span>
                  </div>
                  <p v-if="stage.message" class="event-message">{{ stage.message }}</p>
                  <div class="event-pills">
                    <span v-if="stage.status">{{ statusLabel(stage.status) }}</span>
                  </div>
                  <details v-if="stage.details" class="payload-block">
                    <summary>{{ tr('harness.group.traceDetails', 'Trace details') }}</summary>
                    <pre>{{ compactJSON(stage.details) }}</pre>
                  </details>
                </article>
              </div>
            </template>
          </section>

          <section class="detail-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.timeline', 'Timeline') }}</h4>
              <span>{{ timelineEvents.length }}</span>
            </div>
            <div v-if="timelineEvents.length === 0" class="detail-state">
              {{ tr('harness.group.noTimelineEvents', 'No timeline events recorded for this run.') }}
            </div>
            <div v-else class="event-list">
              <article v-for="event in recentTimelineEvents" :key="event.id" class="event-card">
                <div class="event-header">
                  <strong>{{ event.message || humanizeEnum(event.type) }}</strong>
                  <span>{{ formatDate(event.created_at) }}</span>
                </div>
                <div class="event-pills">
                  <span>{{ humanizeEnum(event.type) }}</span>
                  <span v-if="event.tool_name">{{ tr('harness.group.toolName', 'Tool') }}: {{ event.tool_name }}</span>
                  <span v-if="event.capability_kind">
                    {{ tr('harness.group.capability', 'Capability') }}:
                    {{ humanizeEnum(event.capability_kind) }}
                  </span>
                </div>
                <details v-if="event.payload_json" class="payload-block">
                  <summary>{{ tr('harness.group.rawPayload', 'Raw JSON') }}</summary>
                  <pre>{{ event.payload_json }}</pre>
                </details>
              </article>

              <button
                v-if="remainingTimelineEvents.length && !showAllEvents"
                type="button"
                class="detail-link"
                @click="showAllEvents = true"
              >
                {{ tr('harness.group.showAllEvents', 'Show all events') }}
              </button>

              <template v-if="showAllEvents">
                <article
                  v-for="event in remainingTimelineEvents"
                  :key="`extra-${event.id}`"
                  class="event-card"
                >
                  <div class="event-header">
                    <strong>{{ event.message || humanizeEnum(event.type) }}</strong>
                    <span>{{ formatDate(event.created_at) }}</span>
                  </div>
                  <div class="event-pills">
                    <span>{{ humanizeEnum(event.type) }}</span>
                    <span v-if="event.tool_name">{{ tr('harness.group.toolName', 'Tool') }}: {{ event.tool_name }}</span>
                    <span v-if="event.capability_kind">
                      {{ tr('harness.group.capability', 'Capability') }}:
                      {{ humanizeEnum(event.capability_kind) }}
                    </span>
                  </div>
                  <details v-if="event.payload_json" class="payload-block">
                    <summary>{{ tr('harness.group.rawPayload', 'Raw JSON') }}</summary>
                    <pre>{{ event.payload_json }}</pre>
                  </details>
                </article>
                <button type="button" class="detail-link" @click="showAllEvents = false">
                  {{ tr('harness.group.collapseEvents', 'Show fewer events') }}
                </button>
              </template>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.artifacts', 'Artifacts') }}</h4>
              <span>{{ artifacts.length }}</span>
            </div>
            <div v-if="artifacts.length === 0" class="detail-state">
              {{ tr('harness.group.noArtifacts', 'No artifacts attached to the linked runs yet.') }}
            </div>
            <div v-else class="artifact-list">
              <article v-for="artifact in artifacts" :key="artifact.id" class="artifact-card">
                <div>
                  <strong>{{ artifact.label || humanizeEnum(artifact.kind) }}</strong>
                  <p>{{ humanizeEnum(artifact.kind) }} · {{ compactValue(artifact.mime_type) }}</p>
                </div>
                <a
                  v-if="artifact.path_or_url && artifactIsURL(artifact)"
                  :href="artifact.path_or_url"
                  target="_blank"
                  rel="noreferrer"
                >
                  {{ artifact.path_or_url }}
                </a>
                <code v-else>{{ compactValue(artifact.path_or_url) }}</code>
              </article>
            </div>
          </section>

          <section class="detail-section">
            <div class="section-header">
              <h4>{{ tr('harness.group.pending', 'Pending') }}</h4>
              <span>{{ pendingCount }}</span>
            </div>
            <div v-if="pendingCount === 0" class="detail-state">
              {{ tr('harness.group.noPendingItems', 'No pending approvals or questions for this run.') }}
            </div>
            <div v-else class="pending-stack">
              <article class="pending-card">
                <div class="section-header compact">
                  <strong>{{ tr('harness.group.pendingApprovals', 'Pending approvals') }}</strong>
                  <span>{{ pendingApprovals.length }}</span>
                </div>
                <div v-if="pendingApprovals.length === 0" class="detail-state compact">
                  {{ tr('harness.group.noPendingApprovals', 'No pending approvals.') }}
                </div>
                <pre
                  v-for="(approval, index) in pendingApprovals"
                  :key="`approval-desktop-${index}`"
                  class="pending-json"
                >{{ JSON.stringify(approval, null, 2) }}</pre>
              </article>

              <article class="pending-card">
                <div class="section-header compact">
                  <strong>{{ tr('harness.group.pendingQuestions', 'Pending questions') }}</strong>
                  <span>{{ pendingQuestions.length }}</span>
                </div>
                <div v-if="pendingQuestions.length === 0" class="detail-state compact">
                  {{ tr('harness.group.noPendingQuestions', 'No pending questions.') }}
                </div>
                <pre
                  v-for="(question, index) in pendingQuestions"
                  :key="`question-desktop-${index}`"
                  class="pending-json"
                >{{ JSON.stringify(question, null, 2) }}</pre>
              </article>
            </div>
          </section>
        </template>
      </div>
    </section>
  </aside>
</template>

<style scoped>
.detail-drawer {
  --harness-drawer-surface: var(--color-background-soft);
  --harness-drawer-subsurface: var(--color-bg-surface);
  --harness-drawer-text: var(--color-text);
  --harness-drawer-copy: var(--color-text-secondary);
  --harness-drawer-muted: var(--color-text-muted);
  min-width: 0;
}

.detail-panel {
  border-radius: 1.1rem;
  border: 1px solid var(--color-border);
  background: var(--harness-drawer-surface);
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.08);
}

.detail-drawer .detail-panel {
  position: sticky;
  top: 1rem;
  max-height: calc(100vh - 2rem);
  overflow: hidden;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.1rem 1.15rem 1rem;
  border-bottom: 1px solid rgba(148, 163, 184, 0.16);
}

.detail-header h3,
.detail-section h4 {
  margin: 0;
  color: var(--harness-drawer-text);
}

.detail-kicker,
.detail-subtitle,
.artifact-card p,
.overview-grid dt,
.detail-state {
  color: var(--harness-drawer-muted);
}

.detail-kicker,
.detail-subtitle {
  margin: 0;
}

.detail-kicker {
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.detail-subtitle {
  margin-top: 0.3rem;
  font-size: 0.82rem;
  word-break: break-all;
}

.close-button,
.detail-link {
  appearance: none;
  border: 0;
  background: transparent;
  color: #0f766e;
  cursor: pointer;
  font: inherit;
  font-weight: 700;
}

.close-button {
  border-radius: 0.85rem;
  background: rgba(148, 163, 184, 0.14);
  color: var(--harness-drawer-copy);
  padding: 0.75rem 0.95rem;
}

.detail-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1rem 1.15rem 1.15rem;
  overflow: auto;
  max-height: calc(100vh - 8rem);
}

.detail-banner {
  padding: 0.85rem 0.95rem;
  border-radius: 0.9rem;
  font-size: 0.88rem;
  line-height: 1.5;
}

.detail-banner.is-warning {
  background: rgba(255, 247, 237, 0.96);
  border: 1px solid rgba(245, 158, 11, 0.22);
  color: #9a3412;
}

.detail-banner.is-error {
  background: rgba(254, 242, 242, 0.96);
  border: 1px solid rgba(239, 68, 68, 0.22);
  color: #b91c1c;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
  padding: 0.95rem 1rem;
  border-radius: 0.95rem;
  background: var(--harness-drawer-subsurface);
  border: 1px solid var(--color-border);
}

.summary-section {
  background: linear-gradient(180deg, rgba(59, 130, 246, 0.1), var(--harness-drawer-surface));
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.section-header span,
.summary-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.8rem;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: var(--harness-drawer-copy);
  font-size: 0.76rem;
  font-weight: 700;
}

.section-header.compact {
  margin-bottom: 0.5rem;
}

.summary-title-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}

.summary-title-row strong {
  font-size: 1rem;
  line-height: 1.25;
  color: var(--harness-drawer-text);
}

.summary-pill.role-pill {
  background: rgba(15, 23, 42, 0.07);
  color: var(--harness-drawer-text);
}

.summary-pill.tone-success {
  background: rgba(34, 197, 94, 0.14);
  color: #166534;
}

.summary-pill.tone-running {
  background: rgba(59, 130, 246, 0.14);
  color: #1d4ed8;
}

.summary-pill.tone-blocked {
  background: rgba(245, 158, 11, 0.18);
  color: #92400e;
}

.summary-pill.tone-danger {
  background: rgba(239, 68, 68, 0.13);
  color: #b91c1c;
}

.summary-goal,
.summary-preview,
.spawn-narrative,
.event-message,
.artifact-card p {
  margin: 0;
  line-height: 1.55;
}

.summary-goal {
  color: var(--harness-drawer-copy);
}

.summary-preview,
.spawn-narrative {
  color: var(--harness-drawer-copy);
}

.summary-pills,
.event-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.summary-pills span,
.event-pills span {
  display: inline-flex;
  align-items: center;
  padding: 0.28rem 0.62rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: var(--harness-drawer-copy);
  font-size: 0.8rem;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
  margin: 0;
}

.overview-grid dd {
  margin: 0;
  color: var(--harness-drawer-text);
  line-height: 1.5;
  word-break: break-word;
}

.event-list,
.trace-stage-list,
.artifact-list,
.pending-stack {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.event-card,
.trace-stage-card,
.artifact-card,
.pending-card {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.85rem 0.95rem;
  border-radius: 0.9rem;
  background: var(--harness-drawer-surface);
  border: 1px solid var(--color-border);
}

.event-header {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  align-items: flex-start;
}

.payload-block summary {
  cursor: pointer;
  color: #0f766e;
  font-weight: 700;
}

.payload-block pre,
.pending-json {
  margin: 0.55rem 0 0;
  padding: 0.8rem 0.9rem;
  border-radius: 0.85rem;
  background: var(--color-bg-base);
  color: var(--color-text);
  overflow: auto;
  font-size: 0.8rem;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.artifact-card a,
.artifact-card code {
  color: #0369a1;
  word-break: break-all;
}

.artifact-card code {
  padding: 0.32rem 0.55rem;
  border-radius: 0.7rem;
  background: rgba(15, 23, 42, 0.06);
  color: var(--harness-drawer-text);
}

.detail-state {
  padding: 0.4rem 0;
  line-height: 1.55;
}

.detail-state.compact {
  padding: 0;
  font-size: 0.88rem;
}

.detail-overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  background: rgba(15, 23, 42, 0.38);
  padding: 1rem 0 0;
}

.detail-sheet {
  width: min(100%, 48rem);
  max-height: calc(100vh - 1rem);
  border-bottom-left-radius: 0;
  border-bottom-right-radius: 0;
}

.detail-sheet .detail-body {
  max-height: calc(100vh - 8.5rem);
}

.sheet-enter-active,
.sheet-leave-active {
  transition: opacity 0.2s ease;
}

.sheet-enter-active .detail-sheet,
.sheet-leave-active .detail-sheet {
  transition: transform 0.24s ease;
}

.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
}

.sheet-enter-from .detail-sheet,
.sheet-leave-to .detail-sheet {
  transform: translateY(100%);
}

@media (max-width: 960px) {
  .overview-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .detail-header,
  .event-header {
    flex-direction: column;
  }

  .close-button {
    width: 100%;
  }
}
</style>
