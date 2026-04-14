<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  summarizeHarnessRunTree,
  type HarnessRunTreeNode as HarnessRunTreeNodeType,
} from '@/utils/harnessRunTree'

import HarnessRunTreeNode from './HarnessRunTreeNode.vue'

defineOptions({
  name: 'HarnessRunTree',
})

const props = withDefaults(
  defineProps<{
    nodes?: HarnessRunTreeNodeType[]
    detachedNodes?: HarnessRunTreeNodeType[]
    selectedRunID?: string
    eventPreviewMap?: Record<string, string>
  }>(),
  {
    nodes: () => [],
    detachedNodes: () => [],
    selectedRunID: '',
    eventPreviewMap: () => ({}),
  }
)

const emit = defineEmits<{
  select: [runID: string]
}>()

const { t, te } = useI18n()

function tr(key: string, fallback: string): string {
  return te(key) ? t(key) : fallback
}

const summary = computed(() =>
  summarizeHarnessRunTree([...props.nodes, ...props.detachedNodes])
)

const coordinatorLabel = computed(() => {
  const count = summary.value.coordinatorCount
  return count === 1
    ? tr('harness.group.oneCoordinator', '1 coordinator')
    : `${count} coordinators`
})

const workerLabel = computed(() => {
  const count = summary.value.workerCount
  return count === 1 ? tr('harness.group.oneWorker', '1 worker') : `${count} workers`
})

const runningLabel = computed(() => {
  const count = summary.value.runningCount
  return count === 1 ? tr('harness.group.oneRunningWorker', '1 running') : `${count} running`
})

const completedLabel = computed(() => {
  const count = summary.value.completedCount
  return count === 1
    ? tr('harness.group.oneCompletedWorker', '1 completed')
    : `${count} completed`
})

const failedLabel = computed(() => {
  const count = summary.value.failedCount
  return count === 1 ? tr('harness.group.oneFailedWorker', '1 failed') : `${count} failed`
})

const waitingInputLabel = computed(() => {
  const count = summary.value.waitingInputCount
  return count === 1
    ? tr('harness.group.oneWaitingWorker', '1 waiting input')
    : `${count} waiting input`
})

const detachedLabel = computed(() => {
  const count = summary.value.detachedCount
  return count === 1
    ? tr('harness.group.oneDetachedWorker', 'Detached workers: 1')
    : `Detached workers: ${count}`
})

function selectRun(runID?: string) {
  if (!runID) return
  emit('select', runID)
}
</script>

<template>
  <section class="harness-run-tree">
    <div class="summary-card">
      <div class="summary-copy">
        <p class="summary-kicker">
          {{ tr('harness.group.executionSummary', 'Execution summary') }}
        </p>
        <h3>{{ coordinatorLabel }} · {{ workerLabel }}</h3>
        <p class="summary-body">
          {{
            tr(
              'harness.group.executionSummaryHint',
              'Open any node to inspect timeline events, artifacts, pending input, and errors.'
            )
          }}
        </p>
      </div>

      <div class="summary-actions">
        <span class="summary-chip">{{ coordinatorLabel }}</span>
        <span class="summary-chip">{{ workerLabel }}</span>
        <span class="summary-chip is-running">{{ runningLabel }}</span>
        <span class="summary-chip is-success">{{ completedLabel }}</span>
        <button
          type="button"
          class="summary-chip summary-button is-danger"
          :disabled="!summary.failedCount"
          @click="selectRun(summary.firstFailedRunID)"
        >
          {{ failedLabel }}
        </button>
        <button
          type="button"
          class="summary-chip summary-button is-blocked"
          :disabled="!summary.waitingInputCount"
          @click="selectRun(summary.firstWaitingInputRunID)"
        >
          {{ waitingInputLabel }}
        </button>
        <button
          v-if="summary.detachedCount"
          type="button"
          class="summary-chip summary-button"
          @click="selectRun(summary.firstDetachedRunID)"
        >
          {{ detachedLabel }}
        </button>
      </div>
    </div>

    <div class="tree-stack">
      <HarnessRunTreeNode
        v-for="node in props.nodes"
        :key="node.run.id"
        :node="node"
        :selected-run-id="props.selectedRunID"
        :event-preview-map="props.eventPreviewMap"
        @select="emit('select', $event)"
      />
    </div>

    <section
      v-if="props.detachedNodes.length"
      id="detached-workers"
      class="detached-section"
    >
      <div class="detached-header">
        <div>
          <p class="summary-kicker">
            {{ tr('harness.group.detachedWorkers', 'Detached workers') }}
          </p>
          <h3>{{ detachedLabel }}</h3>
        </div>
        <p class="summary-body">
          {{
            tr(
              'harness.group.detachedWorkersHint',
              'These workers reference a missing parent run, so they are grouped separately.'
            )
          }}
        </p>
      </div>

      <div class="tree-stack detached-stack">
        <HarnessRunTreeNode
          v-for="node in props.detachedNodes"
          :key="node.run.id"
          :node="node"
          :selected-run-id="props.selectedRunID"
          :event-preview-map="props.eventPreviewMap"
          @select="emit('select', $event)"
        />
      </div>
    </section>
  </section>
</template>

<style scoped>
.harness-run-tree {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.summary-card,
.detached-section {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1rem 1.05rem;
  border-radius: 1rem;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 250, 252, 0.92));
}

.summary-copy h3,
.detached-header h3 {
  margin: 0.25rem 0 0;
  color: #0f172a;
}

.summary-kicker,
.summary-body {
  margin: 0;
}

.summary-kicker {
  color: #0f766e;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.summary-body {
  color: #64748b;
  line-height: 1.55;
}

.summary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.summary-chip {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 2.1rem;
  padding: 0.45rem 0.78rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: #334155;
  font-size: 0.84rem;
  font-weight: 700;
  line-height: 1.2;
}

.summary-chip.is-running {
  background: rgba(59, 130, 246, 0.14);
  color: #1d4ed8;
}

.summary-chip.is-success {
  background: rgba(34, 197, 94, 0.14);
  color: #166534;
}

.summary-chip.is-danger {
  background: rgba(239, 68, 68, 0.13);
  color: #b91c1c;
}

.summary-chip.is-blocked {
  background: rgba(245, 158, 11, 0.18);
  color: #92400e;
}

.summary-button {
  appearance: none;
  border: 0;
  cursor: pointer;
  font: inherit;
}

.summary-button:disabled {
  cursor: default;
  opacity: 0.62;
}

.tree-stack,
.detached-stack {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
}

.detached-header {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

@media (max-width: 720px) {
  .summary-card,
  .detached-section {
    padding: 0.95rem;
  }
}
</style>
