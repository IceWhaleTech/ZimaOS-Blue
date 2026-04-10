<script setup lang="ts">
import type { HarnessRunSummary } from '@/api/harness'
import type { HarnessRunTreeNode as HarnessRunTreeNodeType } from '@/utils/harnessRunTree'

import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  deriveHarnessRunChildVisibility,
  deriveHarnessRunPreview,
  isHarnessRunActiveStatus,
} from '@/utils/harnessRunTree'

defineOptions({
  name: 'HarnessRunTreeNode',
})

const props = withDefaults(
  defineProps<{
    node: HarnessRunTreeNodeType
    level?: number
    selectedRunID?: string
    eventPreviewMap?: Record<string, string>
    batchThreshold?: number
  }>(),
  {
    level: 0,
    selectedRunID: '',
    eventPreviewMap: () => ({}),
    batchThreshold: 3,
  }
)

const emit = defineEmits<{
  select: [runID: string]
}>()

const { t, te } = useI18n()

const batchExpanded = ref(false)

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

function humanizeEnum(value?: string | null): string {
  const normalized = String(value || '').trim()
  if (!normalized) return tr('common.notAvailable', 'Not available')
  return normalized.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

function statusTone(status?: string | null): string {
  const normalized = String(status || '').trim()
  switch (normalized) {
    case 'completed':
    case 'passed':
    case 'pass':
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
    case 'fail':
      return 'tone-danger'
    default:
      return 'tone-muted'
  }
}

function statusLabel(status?: string | null): string {
  const normalized = String(status || '').trim()
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

function formatDate(value?: string | null): string {
  if (!value) return tr('common.notAvailable', 'Not available')
  const parsed = Date.parse(value)
  if (Number.isNaN(parsed)) return value
  return new Date(parsed).toLocaleString()
}

function durationFromRun(run: HarnessRunSummary): string {
  const startedAt = Date.parse(String(run.started_at || '').trim())
  const finishedAt = Date.parse(String(run.finished_at || '').trim())
  if (!Number.isFinite(startedAt)) return ''
  const end = Number.isFinite(finishedAt) ? finishedAt : Date.now()
  const delta = Math.max(0, end - startedAt)
  if (delta < 1000) return tr('harness.group.durationNow', 'Less than 1s')

  const totalSeconds = Math.floor(delta / 1000)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${seconds}s`
  return `${seconds}s`
}

const childVisibility = computed(() =>
  deriveHarnessRunChildVisibility(props.node, props.selectedRunID, props.batchThreshold)
)
const canCollapseChildren = computed(
  () => props.level === 0 && childVisibility.value.shouldCollapse
)
const visibleChildren = computed(() => {
  if (!canCollapseChildren.value || batchExpanded.value) return props.node.children
  return childVisibility.value.visibleChildren
})
const hiddenChildren = computed(() => {
  if (!canCollapseChildren.value || batchExpanded.value) return []
  return childVisibility.value.hiddenChildren
})
const hiddenChildCount = computed(() => hiddenChildren.value.length)

const identityLabel = computed(() => props.node.run.agent_id || humanizeEnum(props.node.run.kind))
const roleLabel = computed(() => {
  if (props.node.parentID) return tr('harness.group.workerRole', 'Worker')
  if (props.node.isCoordinator) return tr('harness.group.coordinatorRole', 'Coordinator')
  return ''
})
const metaSegments = computed(() => {
  const run = props.node.run
  const segments = []

  if (run.model) segments.push(`${tr('harness.group.model', 'Model')}: ${run.model}`)
  segments.push(`${tr('harness.group.attemptIndex', 'Attempt')}: ${Number(run.attempt_index || 0)}`)
  segments.push(`${tr('harness.group.depth', 'Depth')}: ${Number(run.depth || 0)}`)

  const duration = durationFromRun(run)
  if (duration) {
    segments.push(`${tr('common.duration', 'Duration')}: ${duration}`)
  } else {
    segments.push(`${tr('common.updatedAt', 'Updated')}: ${formatDate(run.updated_at)}`)
  }

  return segments
})
const preview = computed(
  () =>
    props.eventPreviewMap[props.node.run.id] ||
    deriveHarnessRunPreview(props.node.run) ||
    ''
)
const isSelected = computed(() => props.selectedRunID === props.node.run.id)
const batchSummaryTitle = computed(() => {
  const count = props.node.children.length
  if (hiddenChildCount.value <= 0) return ''
  if (isHarnessRunActiveStatus(props.node.run.status)) {
    return count === 1
      ? tr('harness.group.spawningWorker', 'Spawning 1 worker')
      : `Spawning ${count} workers`
  }
  return count === 1
    ? tr('harness.group.groupedWorker', 'Grouped 1 worker')
    : `Grouped ${count} workers`
})
const batchSummarySubtitle = computed(() => {
  if (!childVisibility.value.representativeLabels.length) {
    return tr('harness.group.batchSummaryHint', 'Expand to inspect grouped workers.')
  }
  return childVisibility.value.representativeLabels.join(', ')
})

function selectNode() {
  emit('select', props.node.run.id)
}

function toggleBatch() {
  batchExpanded.value = !batchExpanded.value
}
</script>

<template>
  <div
    class="tree-branch"
    :class="{
      'is-nested': props.level > 0,
      'is-detached': props.node.isDetached,
    }"
  >
    <button
      :id="`harness-run-${props.node.run.id}`"
      :data-run-id="props.node.run.id"
      type="button"
      class="tree-node"
      :class="[
        statusTone(props.node.run.status),
        {
          'is-selected': isSelected,
          'is-coordinator': props.node.isCoordinator && !props.node.parentID,
          'is-worker': !!props.node.parentID,
          'is-detached': props.node.isDetached,
        },
      ]"
      @click="selectNode"
    >
      <div class="node-title-row">
        <strong>{{ identityLabel }}</strong>
        <span v-if="roleLabel" class="node-pill role-pill">{{ roleLabel }}</span>
        <span class="node-pill status-pill" :class="statusTone(props.node.run.status)">
          {{ statusLabel(props.node.run.status) }}
        </span>
      </div>
      <p class="node-goal">
        {{
          props.node.run.goal ||
          tr('harness.group.noGoalSummary', 'No goal summary was recorded for this run.')
        }}
      </p>
      <p class="node-meta">{{ metaSegments.join(' · ') }}</p>
      <p v-if="preview" class="node-preview">{{ preview }}</p>
      <code class="node-id">{{ props.node.run.id }}</code>
    </button>

    <div v-if="props.node.children.length" class="tree-children">
      <HarnessRunTreeNode
        v-for="child in visibleChildren"
        :key="child.run.id"
        :node="child"
        :level="props.level + 1"
        :selected-run-id="props.selectedRunID"
        :event-preview-map="props.eventPreviewMap"
        :batch-threshold="props.batchThreshold"
        @select="emit('select', $event)"
      />

      <button
        v-if="hiddenChildren.length"
        type="button"
        class="batch-summary-card"
        @click="toggleBatch"
      >
        <div class="batch-summary-header">
          <strong>{{ batchSummaryTitle }}</strong>
          <span>{{ tr('harness.group.expandBatch', 'Expand') }}</span>
        </div>
        <p class="batch-summary-subtitle">{{ batchSummarySubtitle }}</p>
        <p class="batch-summary-meta">
          {{
            tr(
              'harness.group.batchSummaryMeta',
              'Failed or waiting workers stay visible; the rest are grouped here.'
            )
          }}
        </p>
      </button>

      <button
        v-if="canCollapseChildren && batchExpanded"
        type="button"
        class="batch-collapse"
        @click="toggleBatch"
      >
        {{ tr('harness.group.collapseBatch', 'Collapse grouped workers') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.tree-branch {
  --harness-tree-surface: var(--color-background-soft);
  --harness-tree-subsurface: var(--color-bg-surface);
  --harness-tree-text: var(--color-text);
  --harness-tree-copy: var(--color-text-secondary);
  --harness-tree-muted: var(--color-text-muted);
  position: relative;
}

.tree-branch.is-nested {
  margin-left: 1.4rem;
}

.tree-branch.is-nested::before {
  content: '';
  position: absolute;
  top: 1.4rem;
  left: -0.85rem;
  width: 0.85rem;
  height: 1px;
  background: rgba(148, 163, 184, 0.5);
}

.tree-node,
.batch-summary-card {
  position: relative;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  padding: 1rem 1rem 1rem 1.1rem;
  border-radius: 1rem;
  border: 1px solid var(--color-border);
  background: var(--harness-tree-surface);
  color: var(--harness-tree-text);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease,
    background 0.2s ease;
}

.tree-node:hover,
.batch-summary-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.08);
}

.tree-node.is-selected {
  border-color: rgba(15, 118, 110, 0.55);
  box-shadow: 0 0 0 2px rgba(15, 118, 110, 0.12);
}

.tree-node.is-coordinator {
  background: linear-gradient(
    180deg,
    rgba(16, 185, 129, 0.12),
    var(--harness-tree-surface)
  );
  border-color: rgba(16, 185, 129, 0.22);
}

.tree-node.is-worker {
  background: var(--harness-tree-surface);
}

.tree-node.is-detached {
  border-style: dashed;
}

.tree-node.tone-danger {
  box-shadow: inset 4px 0 0 rgba(239, 68, 68, 0.7);
}

.tree-node.tone-blocked {
  box-shadow: inset 4px 0 0 rgba(245, 158, 11, 0.8);
}

.node-title-row,
.batch-summary-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.45rem;
}

.node-title-row strong,
.batch-summary-header strong {
  font-size: 1rem;
  line-height: 1.25;
}

.node-pill {
  display: inline-flex;
  align-items: center;
  padding: 0.28rem 0.62rem;
  border-radius: 999px;
  font-size: 0.76rem;
  font-weight: 700;
}

.role-pill {
  background: rgba(15, 23, 42, 0.07);
  color: var(--harness-tree-text);
}

.status-pill.tone-success {
  background: rgba(34, 197, 94, 0.14);
  color: #166534;
}

.status-pill.tone-running {
  background: rgba(59, 130, 246, 0.14);
  color: #1d4ed8;
}

.status-pill.tone-blocked {
  background: rgba(245, 158, 11, 0.18);
  color: #92400e;
}

.status-pill.tone-danger {
  background: rgba(239, 68, 68, 0.13);
  color: #b91c1c;
}

.status-pill.tone-muted {
  background: rgba(148, 163, 184, 0.18);
  color: #475569;
}

.node-goal,
.node-meta,
.node-preview,
.batch-summary-subtitle,
.batch-summary-meta {
  margin: 0;
  line-height: 1.55;
}

.node-goal,
.node-preview {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
}

.node-goal {
  color: var(--harness-tree-copy);
}

.node-meta,
.batch-summary-meta {
  color: var(--harness-tree-muted);
  font-size: 0.84rem;
}

.node-preview,
.batch-summary-subtitle {
  color: var(--harness-tree-copy);
  font-size: 0.88rem;
}

.node-id {
  align-self: flex-start;
  padding: 0.32rem 0.55rem;
  border-radius: 0.7rem;
  background: rgba(15, 23, 42, 0.06);
  color: var(--harness-tree-text);
  font-size: 0.8rem;
  word-break: break-all;
}

.tree-children {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  margin-top: 0.9rem;
}

.tree-children::before {
  content: '';
  position: absolute;
  top: 0.2rem;
  bottom: 0.65rem;
  left: 0.55rem;
  width: 1px;
  background: linear-gradient(
    180deg,
    rgba(148, 163, 184, 0.45),
    rgba(148, 163, 184, 0.08)
  );
}

.batch-summary-card {
  background: var(--harness-tree-subsurface);
  border-style: dashed;
}

.batch-summary-header span {
  margin-left: auto;
  color: #0f766e;
  font-size: 0.8rem;
  font-weight: 700;
}

.batch-collapse {
  align-self: flex-start;
  padding: 0;
  border: 0;
  background: transparent;
  color: #0f766e;
  cursor: pointer;
  font: inherit;
  font-weight: 700;
}

@media (max-width: 720px) {
  .tree-branch.is-nested {
    margin-left: 1rem;
  }

  .tree-children::before {
    left: 0.4rem;
  }
}
</style>
