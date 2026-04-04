<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  TypelessCardDeepResearchEvent,
  DeepResearchPlannedTask,
  DeepResearchLiveSource,
} from '@/types/typeless'
import {
  localizeDeepResearchGap,
  localizeDeepResearchStatus,
  localizeDeepResearchStructuredValue,
} from '@/utils/deepResearchText'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardDeepResearchEvent
}>()

const tasks = computed(() => props.card.tasks || [])
const sources = computed(() => props.card.sources || [])
const parallelism = computed(() => {
  const value = Number(props.card.parallelism)
  if (!Number.isFinite(value) || value < 2) return 0
  return Math.floor(value)
})
const brief = computed(() => props.card.brief || null)
const verification = computed(() => props.card.verification || null)
const hasRetryGuidance = computed(() => {
  return !!brief.value?.retry_context || !!brief.value?.retry_queries?.length
})

function resolveLabel(key: string, fallback: string): string {
  return te(key) ? String(t(key)) : fallback
}

const statusClass = computed(() => {
  switch (props.card.status) {
    case 'warning':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
    case 'error':
      return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
    default:
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
  }
})

const kindLabel = computed(() => {
  const eventKind = (props.card.event_kind || '').trim()
  const labels: Record<string, string> = {
    brief: t('chat.deepResearchResearchBrief', 'Research brief'),
    planning: t('chat.deepResearchPlannedTasks', 'Planned tasks'),
    source: t('chat.deepResearchLiveSources', 'Live sources'),
    warning: t('chat.deepResearchStageErrors', 'Stage warnings'),
    gap: t('chat.deepResearchLatestGap', 'Research gap'),
    followup: t('chat.deepResearchFollowUpQuery', 'Follow-up query'),
    verification: t('chat.deepResearchVerificationSummary', 'Verification'),
    synthesis: t('chat.deepResearchActionSynthesizing', 'Synthesizing report'),
    'loop-stopped': t('chat.deepResearchActionLoopStopped', 'Research loop stopped'),
  }
  return labels[eventKind] || eventKind || t('chat.deepResearchProcess', 'Research process')
})

const hasDetailSections = computed(() => {
  return (
    !!brief.value ||
    tasks.value.length > 0 ||
    sources.value.length > 0 ||
    !!verification.value ||
    !!props.card.gap ||
    !!props.card.follow_up_query ||
    !!props.card.search_query ||
    !!props.card.stop_reason
  )
})

const statusLabel = computed(
  () =>
    localizeDeepResearchStatus(props.card.status || 'info', resolveLabel) ||
    props.card.status ||
    'info'
)

function domainOf(source: DeepResearchLiveSource): string {
  if (source.domain) return source.domain
  if (!source.url) return ''
  try {
    return new URL(source.url).hostname.replace(/^www\./, '')
  } catch {
    return source.url
  }
}

function taskMeta(task: DeepResearchPlannedTask): string[] {
  return [task.axis, task.category, task.time_window]
    .map((value) => localizeDeepResearchStructuredValue(value, resolveLabel))
    .filter((value): value is string => !!value)
}
</script>

<template>
  <div
    class="rounded-xl border border-slate-200/80 dark:border-slate-700/80 bg-slate-50/90 dark:bg-slate-900/50 shadow-sm overflow-hidden"
  >
    <div
      class="px-3.5 py-2.5 border-b border-white/60 dark:border-slate-800/80 bg-white/80 dark:bg-slate-900/70"
    >
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
            <span
              class="inline-flex items-center rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            >
              {{ kindLabel }}
            </span>
            <span
              v-if="card.iteration"
              class="inline-flex items-center rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            >
              {{ t('chat.deepResearchIteration', 'Iteration') }} {{ card.iteration }}
            </span>
            <span
              v-if="card.task_count"
              class="inline-flex items-center rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
            >
              {{ card.task_count }} {{ t('chat.deepResearchPlannedTasks', 'tasks') }}
            </span>
            <span
              v-if="parallelism > 1"
              class="inline-flex items-center rounded-full px-2 py-0.5 bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-200"
            >
              {{ t('chat.deepResearchParallelism', 'Parallel') }} ×{{ parallelism }}
            </span>
          </div>
          <div
            class="mt-1.5 text-[13px] font-semibold text-slate-800 dark:text-slate-100 break-words"
          >
            {{ card.summary || t('chat.deepResearchProcess', 'Research process') }}
          </div>
          <div
            v-if="card.query"
            class="mt-1 text-xs text-slate-500 dark:text-slate-400 break-words"
          >
            {{ card.query }}
          </div>
        </div>
        <span
          class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium capitalize"
          :class="statusClass"
        >
          {{ statusLabel }}
        </span>
      </div>
    </div>

    <div class="px-3.5 py-2.5 space-y-2.5">
      <div v-if="sources.length > 0" class="space-y-2">
        <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
          {{ t('chat.deepResearchLiveSources', 'Live sources') }}
        </div>
        <div class="grid gap-2 sm:grid-cols-2">
          <a
            v-for="(source, index) in sources"
            :key="`${source.url || source.title || source.domain}-${index}`"
            :href="source.url || undefined"
            :target="source.url ? '_blank' : undefined"
            :rel="source.url ? 'noopener noreferrer' : undefined"
            class="group min-w-0 rounded-lg border border-sky-200 bg-sky-50/80 px-2.5 py-2 text-[11px] text-sky-700 transition-colors hover:bg-sky-100 dark:border-sky-900/60 dark:bg-sky-950/30 dark:text-sky-200 dark:hover:bg-sky-900/30"
          >
            <div class="min-w-0 space-y-1">
              <div class="truncate font-medium">{{ source.title || domainOf(source) }}</div>
              <div
                v-if="domainOf(source)"
                class="truncate text-[10px] text-sky-500 dark:text-sky-300"
              >
                {{ domainOf(source) }}
              </div>
              <div
                v-if="source.query"
                class="truncate text-[10px] text-sky-600/80 dark:text-sky-200/80"
              >
                {{ source.query }}
              </div>
            </div>
          </a>
        </div>
      </div>

      <details
        v-if="hasDetailSections"
        class="group rounded-xl border border-slate-200 bg-white/80 px-2.5 py-2 dark:border-slate-800 dark:bg-slate-950/50"
      >
        <summary
          class="cursor-pointer list-none text-xs font-medium text-slate-600 dark:text-slate-300 flex items-center justify-between gap-2"
        >
          <span>{{ t('chat.deepResearchProcess', 'Research process') }}</span>
          <span class="text-slate-400 transition-transform group-open:rotate-180">⌄</span>
        </summary>

        <div class="mt-2.5 space-y-2.5 text-sm text-slate-700 dark:text-slate-200">
          <div v-if="brief" class="rounded-xl bg-slate-50 px-2.5 py-2.5 dark:bg-slate-900/60">
            <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
              {{ t('chat.deepResearchResearchBrief', 'Research brief') }}
            </div>
            <div v-if="brief.goal" class="mt-2 break-words">{{ brief.goal }}</div>
            <div v-if="brief.entity" class="mt-2 text-xs text-slate-500 dark:text-slate-400">
              {{ brief.entity }}
            </div>
            <div v-if="brief.time_windows?.length" class="mt-2 flex flex-wrap gap-2">
              <span
                v-for="window in brief.time_windows"
                :key="window"
                class="rounded-full bg-white px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300"
              >
                {{ window }}
              </span>
            </div>
            <div v-if="brief.must_verify_claims?.length" class="mt-3">
              <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
                {{ t('chat.deepResearchMustVerify', 'Must verify') }}
              </div>
              <ul class="mt-2 list-disc space-y-1 ps-4 text-xs text-slate-600 dark:text-slate-300">
                <li v-for="claim in brief.must_verify_claims" :key="claim">{{ claim }}</li>
              </ul>
            </div>
            <div
              v-if="hasRetryGuidance"
              class="mt-2.5 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-2.5 dark:border-amber-900/60 dark:bg-amber-950/30"
            >
              <div class="text-xs font-medium text-amber-700 dark:text-amber-200">
                {{ t('chat.deepResearchRetryGuidance', 'Retry guidance') }}
              </div>
              <div
                v-if="brief.retry_context"
                class="mt-2 text-xs text-amber-700/90 break-words dark:text-amber-100"
              >
                {{ brief.retry_context }}
              </div>
              <div v-if="brief.retry_queries?.length" class="mt-3">
                <div class="text-xs font-medium text-amber-700 dark:text-amber-200">
                  {{ t('chat.deepResearchRetryQueries', 'Recovery queries') }}
                </div>
                <ul
                  class="mt-2 list-disc space-y-1 ps-4 text-xs text-amber-700/90 dark:text-amber-100"
                >
                  <li v-for="query in brief.retry_queries" :key="query">{{ query }}</li>
                </ul>
              </div>
            </div>
          </div>

          <div
            v-if="tasks.length > 0"
            class="rounded-xl bg-slate-50 px-2.5 py-2.5 dark:bg-slate-900/60"
          >
            <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
              {{ t('chat.deepResearchPlannedTasks', 'Planned tasks') }}
            </div>
            <div class="mt-2 space-y-2">
              <div
                v-for="(task, index) in tasks"
                :key="`${task.question}-${index}`"
                class="rounded-lg border border-slate-200 bg-white px-2.5 py-2 dark:border-slate-800 dark:bg-slate-950/60"
              >
                <div class="text-sm text-slate-800 dark:text-slate-100 break-words">
                  {{ task.question }}
                </div>
                <div
                  v-if="taskMeta(task).length"
                  class="mt-1 flex flex-wrap gap-2 text-xs text-slate-500 dark:text-slate-400"
                >
                  <span
                    v-for="meta in taskMeta(task)"
                    :key="meta"
                    class="rounded-full bg-slate-100 px-2 py-0.5 dark:bg-slate-800"
                    >{{ meta }}</span
                  >
                </div>
              </div>
            </div>
          </div>

          <div
            v-if="verification"
            class="rounded-xl bg-slate-50 px-2.5 py-2.5 dark:bg-slate-900/60"
          >
            <div class="text-xs font-medium text-slate-500 dark:text-slate-400">
              {{ t('chat.deepResearchVerificationSummary', 'Verification') }}
            </div>
            <div class="mt-2 flex flex-wrap gap-2 text-xs">
              <span
                class="rounded-full bg-emerald-100 px-2.5 py-0.5 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200"
              >
                {{ t('chat.deepResearchVerificationResolved', 'Resolved') }}
                {{ verification.resolved_count || 0 }}
              </span>
              <span
                class="rounded-full bg-amber-100 px-2.5 py-0.5 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200"
              >
                {{ t('chat.deepResearchVerificationConflicted', 'Conflicted') }}
                {{ verification.conflicted_count || 0 }}
              </span>
              <span
                class="rounded-full bg-slate-200 px-2.5 py-0.5 text-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                {{ t('chat.deepResearchVerificationInsufficient', 'Insufficient') }}
                {{ verification.insufficient_count || 0 }}
              </span>
            </div>
          </div>

          <div
            v-if="
              card.gap ||
              card.focus ||
              card.search_query ||
              card.follow_up_query ||
              card.stop_reason
            "
            class="rounded-xl border border-amber-200 bg-amber-50 px-2.5 py-2.5 dark:border-amber-900/60 dark:bg-amber-950/30"
          >
            <div class="text-xs font-medium text-amber-700 dark:text-amber-200">
              {{ t('chat.deepResearchStageErrors', 'Stage warnings') }}
            </div>
            <div v-if="card.focus" class="mt-2 text-sm break-words">{{ card.focus }}</div>
            <div v-if="card.gap" class="mt-1 text-sm break-words">
              {{ localizeDeepResearchGap(card.gap, resolveLabel) }}
            </div>
            <div
              v-if="card.search_query"
              class="mt-1 text-xs text-amber-700/80 dark:text-amber-200/80"
            >
              {{ card.search_query }}
            </div>
            <div
              v-if="card.follow_up_query"
              class="mt-1 text-xs text-amber-700/80 dark:text-amber-200/80"
            >
              {{ card.follow_up_query }}
            </div>
            <div
              v-if="card.stop_reason"
              class="mt-1 text-xs text-amber-700/80 dark:text-amber-200/80"
            >
              {{ card.stop_reason }}
            </div>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>
