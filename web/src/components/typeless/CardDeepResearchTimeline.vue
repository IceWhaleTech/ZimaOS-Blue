<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  DeepResearchLiveSource,
  DeepResearchPlannedTask,
  DeepResearchTimelineStep,
  TypelessCardDeepResearchTimeline,
} from '@/types/typeless'
import {
  localizeDeepResearchAction,
  localizeDeepResearchGap,
  localizeDeepResearchStage,
  localizeDeepResearchStopReason,
  localizeDeepResearchMode,
  localizeDeepResearchSegment,
  localizeDeepResearchStructuredValue,
  localizeDeepResearchSummary,
  localizeDeepResearchTimeWindow,
  localizeDeepResearchStatus,
  localizeResearchProgressLabel,
  localizeResearchSurfaceTitle,
} from '@/utils/deepResearchText'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardDeepResearchTimeline
  uiStateKey?: string
}>()

function resolveLabel(key: string, fallback: string, named?: Record<string, unknown>) {
  if (te(key)) {
    return String(named ? t(key, named) : t(key))
  }
  return fallback
}

const steps = computed<DeepResearchTimelineStep[]>(() => props.card.steps || [])
const latestStepIndex = computed(() => Math.max(steps.value.length - 1, 0))
const latestStep = computed(() => steps.value[latestStepIndex.value] || null)

const status = computed(() => {
  return (
    String(props.card.status || latestStep.value?.status || 'running')
      .trim()
      .toLowerCase() || 'running'
  )
})

const progress = computed(() => {
  const value = Number(props.card.progress)
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, Math.round(value)))
})

const iteration = computed(() => {
  const value = Number(props.card.iteration || latestStep.value?.iteration || 0)
  if (!Number.isFinite(value) || value <= 0) return 0
  return Math.floor(value)
})

const stage = computed(() => {
  return String(props.card.stage || latestStep.value?.stage || '')
    .trim()
    .toLowerCase()
})

const query = computed(() => String(props.card.query || latestStep.value?.query || '').trim())
const modeLabel = computed(() =>
  localizeDeepResearchMode(
    String(props.card.mode || latestStep.value?.mode || 'standard').trim(),
    resolveLabel
  )
)
const latestGap = computed(() => {
  const raw = String(
    props.card.latest_gap || latestStep.value?.gap || latestStep.value?.latest_gap || ''
  ).trim()
  return localizeDeepResearchGap(raw, resolveLabel) || raw
})
const latestAction = computed(() => {
  return String(props.card.latest_action || latestStep.value?.latest_action || '').trim()
})

const statusClass = computed(() => {
  switch (status.value) {
    case 'failed':
    case 'error':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    case 'cancelled':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    case 'completed':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    default:
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  }
})

const progressClass = computed(() => {
  switch (status.value) {
    case 'failed':
    case 'error':
      return 'bg-red-500'
    case 'cancelled':
      return 'bg-amber-500'
    case 'completed':
      return 'bg-emerald-500'
    default:
      return 'bg-blue-500'
  }
})

function stageLabel(rawStage?: string): string {
  return (
    localizeDeepResearchStage(rawStage, resolveLabel) ||
    rawStage ||
    resolveLabel('chat.deepResearchProcess', 'Research process')
  )
}

function latestActionLabel(action?: string): string {
  return localizeDeepResearchAction(action, resolveLabel) || String(action || '').trim()
}

function stepKindLabel(step: DeepResearchTimelineStep): string {
  const eventKind = String(step.event_kind || '').trim()
  const labels: Record<string, string> = {
    brief: resolveLabel('chat.deepResearchResearchBrief', 'Research brief'),
    planning: resolveLabel('chat.deepResearchPlannedTasks', 'Planned tasks'),
    source: resolveLabel('chat.deepResearchLiveSources', 'Live sources'),
    warning: resolveLabel('chat.deepResearchStageErrors', 'Stage warnings'),
    gap: resolveLabel('chat.deepResearchLatestGap', 'Research gap'),
    followup: resolveLabel('chat.deepResearchFollowUpQuery', 'Follow-up query'),
    verification: resolveLabel('chat.deepResearchVerificationSummary', 'Verification'),
    synthesis: resolveLabel('chat.deepResearchActionSynthesizing', 'Synthesizing report'),
    'loop-stopped': resolveLabel('chat.deepResearchActionLoopStopped', 'Research loop stopped'),
  }
  if (step.type === 'deep-research-progress') {
    return localizeResearchProgressLabel(resolveLabel)
  }
  return labels[eventKind] || eventKind || stageLabel(step.stage)
}

function stepSummary(step: DeepResearchTimelineStep): string {
  const summary = String(step.summary || '').trim()
  if (summary) return localizeDeepResearchSummary(summary, resolveLabel) || summary
  if (step.type === 'deep-research-progress') {
    return latestActionLabel(step.latest_action)
  }
  if (step.follow_up_query) return String(step.follow_up_query).trim()
  if (step.search_query) return String(step.search_query).trim()
  return stepKindLabel(step)
}

function stepStatusClass(step: DeepResearchTimelineStep): string {
  const stepStatus = String(step.status || '')
    .trim()
    .toLowerCase()
  switch (stepStatus) {
    case 'warning':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    case 'failed':
    case 'error':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    case 'completed':
    case 'success':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
    default:
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200'
  }
}

function stepStatusLabel(step: DeepResearchTimelineStep): string {
  return localizeDeepResearchStatus(step.status, resolveLabel) || String(step.status || 'info')
}

function stepDotClass(step: DeepResearchTimelineStep): string {
  const stepStatus = String(step.status || '')
    .trim()
    .toLowerCase()
  switch (stepStatus) {
    case 'warning':
      return 'bg-amber-500'
    case 'failed':
    case 'error':
      return 'bg-red-500'
    case 'completed':
    case 'success':
      return 'bg-emerald-500'
    default:
      return 'bg-sky-500'
  }
}

function taskMeta(task: DeepResearchPlannedTask): string[] {
  return [
    localizeDeepResearchStructuredValue(task.axis, resolveLabel) ||
      localizeDeepResearchSegment(task.axis, resolveLabel) ||
      String(task.axis || '').trim(),
    localizeDeepResearchStructuredValue(task.category, resolveLabel) ||
      localizeDeepResearchSegment(task.category, resolveLabel) ||
      String(task.category || '').trim(),
    localizeDeepResearchTimeWindow(task.time_window, resolveLabel) ||
      String(task.time_window || '').trim(),
  ].filter((value): value is string => !!value)
}

function domainOf(source: DeepResearchLiveSource): string {
  if (source.domain) return source.domain
  if (!source.url) return ''
  try {
    return new URL(source.url).hostname.replace(/^www\./, '')
  } catch {
    return source.url
  }
}

const headerTitle = computed(() => {
  if (query.value) return query.value
  return localizeResearchSurfaceTitle(resolveLabel)
})

const emptyStateLabel = computed(() => {
  return (
    latestActionLabel(latestAction.value) ||
    resolveLabel('chat.deepResearchProcess', 'Research process')
  )
})
</script>

<template>
  <div
    class="rounded-xl border border-slate-200/80 dark:border-slate-700/80 bg-white/95 dark:bg-slate-900/80 shadow-sm overflow-hidden"
  >
    <div
      class="px-3.5 py-2.5 border-b border-slate-100 dark:border-slate-800 bg-slate-50/80 dark:bg-slate-950/60"
    >
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
            <span
              class="rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            >
              {{ localizeResearchSurfaceTitle(resolveLabel) }}
            </span>
            <span class="rounded-full px-2 py-0.5" :class="statusClass">
              {{ stageLabel(stage || status) }}
            </span>
            <span
              class="rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            >
              {{ modeLabel }}
            </span>
          </div>
          <div
            class="mt-1.5 text-[13px] font-semibold text-slate-800 dark:text-slate-100 break-words"
          >
            {{ headerTitle }}
          </div>
          <div
            class="mt-1.5 flex flex-wrap items-center gap-2.5 text-xs text-slate-500 dark:text-slate-400"
          >
            <span>{{ progress }}%</span>
            <span v-if="iteration">
              {{ resolveLabel('chat.deepResearchIteration', 'Iteration') }} {{ iteration }}
            </span>
            <span v-if="latestAction">{{ latestActionLabel(latestAction) }}</span>
          </div>
        </div>
        <div class="text-xs font-medium text-slate-600 dark:text-slate-300">{{ progress }}%</div>
      </div>
      <div class="mt-2.5 h-2 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
        <div
          class="h-full transition-all duration-500 ease-out"
          :class="progressClass"
          :style="{ width: `${progress}%` }"
        />
      </div>
    </div>

    <div class="px-3.5 py-2.5 space-y-2.5">
      <div
        v-if="latestGap"
        class="rounded-lg bg-amber-50 px-2.5 py-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
      >
        <div class="font-medium">
          {{ resolveLabel('chat.deepResearchLatestGap', 'Latest gap') }}
        </div>
        <div class="mt-1 break-words">{{ latestGap }}</div>
      </div>

      <div
        v-if="steps.length === 0"
        class="rounded-lg bg-slate-50 px-2.5 py-2.5 dark:bg-slate-900/60"
      >
        <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
          {{ resolveLabel('chat.deepResearchProcess', 'Research process') }}
        </div>
        <div class="mt-1 text-sm text-slate-700 dark:text-slate-200 break-words">
          {{ emptyStateLabel }}
        </div>
      </div>

      <div v-else class="space-y-2.5">
        <div
          v-for="(step, index) in steps"
          :key="step.id || `deep-research-step-${index}`"
          class="deep-research-timeline-step relative"
        >
          <div
            class="deep-research-timeline-connector absolute top-3 bottom-0 w-px bg-slate-200 dark:bg-slate-700"
            :class="{ hidden: index === steps.length - 1 }"
          />
          <div
            class="deep-research-timeline-dot absolute top-2.5 h-4 w-4 rounded-full ring-4 ring-white dark:ring-slate-900"
            :class="stepDotClass(step)"
          />

          <details
            class="rounded-xl border px-2.5 py-2.5 transition-colors"
            :class="[
              index === latestStepIndex
                ? 'border-sky-200 bg-sky-50/70 dark:border-sky-900/60 dark:bg-sky-950/20'
                : 'border-slate-200 bg-white/80 dark:border-slate-800 dark:bg-slate-950/40',
            ]"
            :open="index === latestStepIndex"
          >
            <summary class="cursor-pointer list-none">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0 flex-1">
                  <div
                    class="flex flex-wrap items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400"
                  >
                    <span
                      class="inline-flex items-center rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                    >
                      {{ stepKindLabel(step) }}
                    </span>
                    <span
                      v-if="step.iteration"
                      class="inline-flex items-center rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                    >
                      {{ resolveLabel('chat.deepResearchIteration', 'Iteration') }}
                      {{ step.iteration }}
                    </span>
                    <span
                      v-if="step.parallelism && step.parallelism > 1"
                      class="inline-flex items-center rounded-full px-2 py-0.5 bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200"
                    >
                      {{ resolveLabel('chat.deepResearchParallelism', 'Parallel') }} ×{{
                        step.parallelism
                      }}
                    </span>
                  </div>
                  <div
                    class="mt-1.5 text-[13px] font-medium text-slate-800 dark:text-slate-100 break-words"
                  >
                    {{ stepSummary(step) }}
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <span
                    class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium capitalize"
                    :class="stepStatusClass(step)"
                  >
                    {{ stepStatusLabel(step) }}
                  </span>
                  <span class="text-slate-400 transition-transform details-chevron">⌄</span>
                </div>
              </div>
            </summary>

            <div class="mt-2.5 space-y-2.5 text-sm text-slate-700 dark:text-slate-200">
              <div
                v-if="step.brief"
                class="rounded-lg bg-slate-50 px-2.5 py-2.5 text-xs dark:bg-slate-900/60"
              >
                <div class="font-medium text-slate-500 dark:text-slate-400">
                  {{ resolveLabel('chat.deepResearchResearchBrief', 'Research brief') }}
                </div>
                <div v-if="step.brief.goal" class="mt-2 break-words">{{ step.brief.goal }}</div>
                <div v-if="step.brief.entity" class="mt-2 text-slate-500 dark:text-slate-400">
                  {{ step.brief.entity }}
                </div>
                <div v-if="step.brief.time_windows?.length" class="mt-2 flex flex-wrap gap-2">
                  <span
                    v-for="window in step.brief.time_windows"
                    :key="window"
                    class="rounded-full bg-white px-2.5 py-1 dark:bg-slate-800"
                  >
                    {{ localizeDeepResearchTimeWindow(window, resolveLabel) }}
                  </span>
                </div>
                <div v-if="step.brief.must_verify_claims?.length" class="mt-3">
                  <div class="font-medium text-slate-500 dark:text-slate-400">
                    {{ resolveLabel('chat.deepResearchMustVerify', 'Must verify') }}
                  </div>
                  <ul
                    class="deep-research-timeline-list mt-2 list-disc space-y-1 text-slate-600 dark:text-slate-300"
                  >
                    <li v-for="claim in step.brief.must_verify_claims" :key="claim">{{ claim }}</li>
                  </ul>
                </div>
                <div
                  v-if="step.brief.retry_context || step.brief.retry_queries?.length"
                  class="mt-2.5 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-2.5 dark:border-amber-900/60 dark:bg-amber-950/30"
                >
                  <div class="font-medium text-amber-700 dark:text-amber-200">
                    {{ resolveLabel('chat.deepResearchRetryGuidance', 'Retry guidance') }}
                  </div>
                  <div
                    v-if="step.brief.retry_context"
                    class="mt-2 break-words text-amber-700/90 dark:text-amber-100"
                  >
                    {{ step.brief.retry_context }}
                  </div>
                  <div v-if="step.brief.retry_queries?.length" class="mt-3">
                    <div class="font-medium text-amber-700 dark:text-amber-200">
                      {{ resolveLabel('chat.deepResearchRetryQueries', 'Recovery queries') }}
                    </div>
                    <ul
                      class="deep-research-timeline-list mt-2 list-disc space-y-1 text-amber-700/90 dark:text-amber-100"
                    >
                      <li v-for="query in step.brief.retry_queries" :key="query">{{ query }}</li>
                    </ul>
                  </div>
                </div>
              </div>

              <div
                v-if="step.tasks?.length"
                class="rounded-lg bg-slate-50 px-2.5 py-2.5 text-xs dark:bg-slate-900/60"
              >
                <div class="font-medium text-slate-500 dark:text-slate-400">
                  {{ resolveLabel('chat.deepResearchPlannedTasks', 'Planned tasks') }}
                </div>
                <div class="mt-2 space-y-2">
                  <div
                    v-for="(task, taskIndex) in step.tasks"
                    :key="`${task.question}-${taskIndex}`"
                    class="rounded-lg border border-slate-200 bg-white px-2.5 py-2 dark:border-slate-800 dark:bg-slate-950/60"
                  >
                    <div class="break-words text-slate-800 dark:text-slate-100">
                      {{ task.question }}
                    </div>
                    <div
                      v-if="taskMeta(task).length"
                      class="mt-1 flex flex-wrap gap-2 text-slate-500 dark:text-slate-400"
                    >
                      <span
                        v-for="meta in taskMeta(task)"
                        :key="meta"
                        class="rounded-full bg-slate-100 px-2 py-0.5 dark:bg-slate-800"
                      >
                        {{ meta }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              <div
                v-if="step.sources?.length"
                class="rounded-lg bg-slate-50 px-2.5 py-2.5 text-xs dark:bg-slate-900/60"
              >
                <div class="font-medium text-slate-500 dark:text-slate-400">
                  {{ resolveLabel('chat.deepResearchLiveSources', 'Live sources') }}
                </div>
                <div class="mt-2 grid gap-2 sm:grid-cols-2">
                  <a
                    v-for="(source, sourceIndex) in step.sources"
                    :key="`${source.url || source.title || source.domain}-${sourceIndex}`"
                    :href="source.url || undefined"
                    :target="source.url ? '_blank' : undefined"
                    :rel="source.url ? 'noopener noreferrer' : undefined"
                    class="rounded-lg border border-slate-200 bg-white px-2.5 py-2 transition-colors hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-950/60 dark:hover:bg-slate-900/70"
                  >
                    <div class="truncate font-medium text-slate-800 dark:text-slate-100">
                      {{ source.title || domainOf(source) }}
                    </div>
                    <div
                      v-if="domainOf(source)"
                      class="mt-1 truncate text-slate-500 dark:text-slate-400"
                    >
                      {{ domainOf(source) }}
                    </div>
                    <div
                      v-if="source.query"
                      class="mt-1 truncate text-slate-500 dark:text-slate-400"
                    >
                      {{ source.query }}
                    </div>
                  </a>
                </div>
              </div>

              <div
                v-if="step.verification"
                class="rounded-lg bg-slate-50 px-2.5 py-2.5 text-xs dark:bg-slate-900/60"
              >
                <div class="font-medium text-slate-500 dark:text-slate-400">
                  {{ resolveLabel('chat.deepResearchVerificationSummary', 'Verification') }}
                </div>
                <div class="mt-2 flex flex-wrap gap-2">
                  <span class="rounded-full bg-white px-2.5 py-0.5 dark:bg-slate-800">
                    {{ resolveLabel('chat.deepResearchVerificationResolved', 'Resolved') }}
                    {{ step.verification.resolved_count || 0 }}
                  </span>
                  <span class="rounded-full bg-white px-2.5 py-0.5 dark:bg-slate-800">
                    {{ resolveLabel('chat.deepResearchVerificationConflicted', 'Conflicted') }}
                    {{ step.verification.conflicted_count || 0 }}
                  </span>
                  <span class="rounded-full bg-white px-2.5 py-0.5 dark:bg-slate-800">
                    {{ resolveLabel('chat.deepResearchVerificationInsufficient', 'Insufficient') }}
                    {{ step.verification.insufficient_count || 0 }}
                  </span>
                </div>
              </div>

              <div
                v-if="
                  step.follow_up_query ||
                  step.gap ||
                  step.focus ||
                  step.search_query ||
                  step.stop_reason ||
                  step.source_title
                "
                class="grid gap-2 md:grid-cols-2 text-xs"
              >
                <div
                  v-if="step.follow_up_query"
                  class="rounded-lg bg-slate-50 px-2.5 py-2 dark:bg-slate-900/60"
                >
                  <div class="font-medium text-slate-500 dark:text-slate-400">
                    {{ resolveLabel('chat.deepResearchFollowUpQuery', 'Follow-up query') }}
                  </div>
                  <div class="mt-1 break-words">{{ step.follow_up_query }}</div>
                </div>
                <div
                  v-if="step.search_query"
                  class="rounded-lg bg-slate-50 px-2.5 py-2 dark:bg-slate-900/60"
                >
                  <div class="font-medium text-slate-500 dark:text-slate-400">
                    {{ resolveLabel('chat.deepResearchSearchQuery', 'Search query') }}
                  </div>
                  <div class="mt-1 break-words">{{ step.search_query }}</div>
                </div>
                <div
                  v-if="step.gap"
                  class="rounded-lg bg-amber-50 px-2.5 py-2 text-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
                >
                  <div class="font-medium">
                    {{ resolveLabel('chat.deepResearchLatestGap', 'Research gap') }}
                  </div>
                  <div class="mt-1 break-words">
                    {{ localizeDeepResearchGap(step.gap, resolveLabel) }}
                  </div>
                </div>
                <div
                  v-if="step.focus"
                  class="rounded-lg bg-slate-50 px-2.5 py-2 dark:bg-slate-900/60"
                >
                  <div class="font-medium text-slate-500 dark:text-slate-400">
                    {{ resolveLabel('chat.deepResearchFocus', 'Focus') }}
                  </div>
                  <div class="mt-1 break-words">
                    {{
                      localizeDeepResearchStructuredValue(step.focus, resolveLabel) ||
                      localizeDeepResearchSegment(step.focus, resolveLabel) ||
                      step.focus
                    }}
                  </div>
                </div>
                <div
                  v-if="step.source_title"
                  class="rounded-lg bg-slate-50 px-2.5 py-2 dark:bg-slate-900/60"
                >
                  <div class="font-medium text-slate-500 dark:text-slate-400">
                    {{ resolveLabel('chat.deepResearchEvidence', 'Evidence') }}
                  </div>
                  <div class="mt-1 break-words">{{ step.source_title }}</div>
                </div>
                <div
                  v-if="step.stop_reason"
                  class="rounded-lg bg-slate-50 px-2.5 py-2 dark:bg-slate-900/60"
                >
                  <div class="font-medium text-slate-500 dark:text-slate-400">
                    {{ resolveLabel('chat.deepResearchStopReason', 'Stop reason') }}
                  </div>
                  <div class="mt-1 break-words">
                    {{
                      localizeDeepResearchStopReason(step.stop_reason, resolveLabel) ||
                      step.stop_reason
                    }}
                  </div>
                </div>
              </div>
            </div>
          </details>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.deep-research-timeline-step {
  padding-inline-start: 1.5rem;
}

.deep-research-timeline-connector {
  inset-inline-start: 0.5rem;
}

.deep-research-timeline-dot {
  inset-inline-start: 0;
}

.deep-research-timeline-list {
  padding-inline-start: 1rem;
}

details[open] > summary .details-chevron {
  transform: rotate(180deg);
}
</style>
