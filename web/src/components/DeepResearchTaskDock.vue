<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DeepResearchJobSummary } from '@/api/deepResearch'
import { useDeepResearchJobsStore } from '@/stores/deepResearchJobs'
import {
  localizeDeepResearchAction,
  localizeDeepResearchGap,
  localizeDeepResearchStage,
  localizeResearchProgressLabel,
  localizeResearchRunningElsewhereLabel,
  localizeResearchRunningTasksLabel,
} from '@/utils/deepResearchText'

const DEEP_RESEARCH_DOCK_COLLAPSED_KEY = 'zima.chat.deep_research_dock_collapsed.v1'

const emit = defineEmits<{
  view: [job: DeepResearchJobSummary]
}>()

const { t, te } = useI18n()
const deepResearchJobs = useDeepResearchJobsStore()
const cancellingJobId = ref('')
const collapsed = ref(loadCollapsedState())

const jobs = computed(() => deepResearchJobs.activeJobs)
const leadJob = computed(() => jobs.value[0] || null)
const runningTasksLabel = computed(() => localizeResearchRunningTasksLabel(tr))
const runningElsewhereLabel = computed(() => localizeResearchRunningElsewhereLabel(tr))

function tr(key: string, fallback: string): string {
  return te(key) ? String(t(key)) : fallback
}

function loadCollapsedState(): boolean {
  try {
    return localStorage.getItem(DEEP_RESEARCH_DOCK_COLLAPSED_KEY) !== '0'
  } catch {
    return true
  }
}

function persistCollapsedState(value: boolean) {
  try {
    if (value) {
      localStorage.removeItem(DEEP_RESEARCH_DOCK_COLLAPSED_KEY)
      return
    }
    localStorage.setItem(DEEP_RESEARCH_DOCK_COLLAPSED_KEY, '0')
  } catch {
    // Ignore storage errors
  }
}

function stageLabel(stage?: string): string {
  return localizeDeepResearchStage(stage, tr) || stage || localizeResearchProgressLabel(tr)
}

function latestActionLabel(action?: string): string {
  return localizeDeepResearchAction(action, tr) || action || ''
}

async function handleCancel(job: DeepResearchJobSummary) {
  if (!job?.job_id || cancellingJobId.value) return
  cancellingJobId.value = job.job_id
  try {
    await deepResearchJobs.cancelJob(job.job_id)
  } finally {
    cancellingJobId.value = ''
  }
}

function toggleCollapsed() {
  collapsed.value = !collapsed.value
}

watch(collapsed, (value) => {
  persistCollapsedState(value)
})

watch(
  () => jobs.value.length,
  (count) => {
    if (count === 0) collapsed.value = true
  }
)
</script>

<template>
  <div
    v-if="jobs.length > 0"
    data-testid="deep-research-task-dock"
    class="relative z-10 w-full min-w-0 h-12 -mb-1"
  >
    <section
      class="absolute inset-x-0 bottom-0 overflow-hidden rounded-[1.25rem] border border-slate-200/80 bg-white/95 shadow-xl backdrop-blur-md transition-shadow dark:border-slate-700/80 dark:bg-slate-900/90"
    >
      <button
        data-testid="deep-research-task-dock-toggle"
        type="button"
        class="flex min-h-12 w-full items-start gap-3 bg-slate-50/80 px-4 py-3 text-start transition-colors hover:bg-slate-100/80 dark:bg-slate-950/60 dark:hover:bg-slate-900/70"
        :aria-expanded="!collapsed"
        @click="toggleCollapsed"
      >
        <span
          class="mt-0.5 inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
        >
          <svg
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.8"
              d="M12 6.75V9m0 6v2.25m5.25-5.25H15m-6 0H6.75m8.962-3.712-1.591 1.591m-4.242 4.242-1.591 1.591m0-7.424 1.591 1.591m4.242 4.242 1.591 1.591"
            />
          </svg>
        </span>

        <span class="min-w-0 flex-1">
          <span class="flex min-w-0 items-center gap-2">
            <span class="truncate text-sm font-semibold text-slate-800 dark:text-slate-100">
              {{ runningTasksLabel }}
            </span>
          </span>

          <span
            class="mt-1 flex min-w-0 flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400"
          >
            <span
              v-if="leadJob"
              class="rounded-full bg-blue-100 px-2.5 py-1 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
            >
              {{ stageLabel(leadJob.stage) }}
            </span>
            <span v-if="leadJob">{{ Math.round(leadJob.progress || 0) }}%</span>
            <span class="min-w-0 flex-1 truncate">
              {{ leadJob?.query || runningElsewhereLabel }}
            </span>
          </span>

          <span
            v-if="!collapsed"
            class="mt-1.5 block text-xs text-slate-500 dark:text-slate-400"
          >
            {{ runningElsewhereLabel }}
          </span>
        </span>

        <span
          class="mt-0.5 inline-flex h-7 min-w-7 flex-shrink-0 items-center justify-center rounded-full bg-slate-100 px-2 text-xs font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300"
        >
          {{ jobs.length }}
        </span>

        <span class="mt-1 inline-flex flex-shrink-0 items-center justify-center text-slate-400">
          <svg
            class="h-4 w-4 transition-transform duration-200"
            :class="{ 'rotate-180': !collapsed }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </span>
      </button>

      <div
        v-if="!collapsed"
        data-testid="deep-research-task-dock-details"
        class="max-h-72 overflow-y-auto border-t border-slate-100 px-3 py-3 space-y-2 dark:border-slate-800 sm:max-h-80"
      >
        <div
          v-for="job in jobs"
          :key="job.job_id"
          :data-testid="`deep-research-task-dock-job-${job.job_id}`"
          class="rounded-2xl border border-slate-200 bg-white px-3 py-3 dark:border-slate-800 dark:bg-slate-950/60"
        >
          <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div class="min-w-0 flex-1">
              <div
                class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400"
              >
                <span
                  class="rounded-full bg-blue-100 px-2.5 py-1 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200"
                >{{ stageLabel(job.stage) }}</span>
                <span>{{ Math.round(job.progress || 0) }}%</span>
                <span v-if="job.iteration">{{ t('chat.deepResearchIteration', 'Iteration') }} {{ job.iteration }}</span>
              </div>
              <div class="mt-2 break-words text-sm font-medium text-slate-800 dark:text-slate-100">
                {{ job.query }}
              </div>
              <div
                v-if="job.latest_action || job.latest_gap"
                class="mt-2 space-y-1 text-xs text-slate-500 dark:text-slate-400"
              >
                <div
                  v-if="job.latest_action"
                  class="break-words"
                >
                  {{ latestActionLabel(job.latest_action) }}
                </div>
                <div
                  v-if="job.latest_gap"
                  class="break-words text-amber-700 dark:text-amber-200"
                >
                  {{ localizeDeepResearchGap(job.latest_gap, tr) }}
                </div>
              </div>
              <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-slate-100 dark:bg-slate-800">
                <div
                  class="h-full bg-blue-500 transition-all duration-500"
                  :style="{
                    width: `${Math.max(0, Math.min(100, Math.round(job.progress || 0)))}%`,
                  }"
                />
              </div>
            </div>

            <div class="flex items-center gap-2 md:flex-col md:items-end">
              <button
                v-if="job.conversation_id"
                type="button"
                :data-testid="`deep-research-task-dock-view-${job.job_id}`"
                class="cursor-pointer rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
                @click="emit('view', job)"
              >
                {{ t('chat.deepResearchBackToTask', 'Back to task') }}
              </button>
              <button
                type="button"
                :data-testid="`deep-research-task-dock-cancel-${job.job_id}`"
                class="cursor-pointer rounded-full border border-amber-200 px-3 py-1.5 text-xs font-medium text-amber-700 hover:bg-amber-50 dark:border-amber-900/60 dark:text-amber-200 dark:hover:bg-amber-950/30 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="cancellingJobId === job.job_id"
                @click="handleCancel(job)"
              >
                {{
                  cancellingJobId === job.job_id
                    ? t('common.loading', 'Loading...')
                    : t('chat.deepResearchCancelTask', 'Cancel')
                }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>
