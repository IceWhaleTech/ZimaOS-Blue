<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DeepResearchJobSummary } from '@/api/deepResearch'
import { useDeepResearchJobsStore } from '@/stores/deepResearchJobs'

const emit = defineEmits<{
  view: [job: DeepResearchJobSummary]
}>()

const { t } = useI18n()
const deepResearchJobs = useDeepResearchJobsStore()
const cancellingJobId = ref('')

const jobs = computed(() => deepResearchJobs.activeJobs)

function stageLabel(stage?: string): string {
  const stageMap: Record<string, string> = {
    intake: t('chat.deepResearchStageIntake', 'Intake'),
    planning: t('chat.deepResearchStagePlanning', 'Planning'),
    retrieve: t('chat.deepResearchStageRetrieve', 'Retrieving'),
    verify: t('chat.deepResearchStageVerify', 'Verifying'),
    synthesize: t('chat.deepResearchStageSynthesize', 'Synthesizing'),
  }
  return stageMap[String(stage || '').trim()] || stage || t('chat.deepResearchProgress', 'Running')
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
</script>

<template>
  <div class="rounded-[1.25rem] border border-slate-200/80 dark:border-slate-700/80 bg-white/95 dark:bg-slate-900/90 shadow-xl backdrop-blur-md overflow-hidden">
    <div class="px-4 py-3 border-b border-slate-100 dark:border-slate-800 bg-slate-50/80 dark:bg-slate-950/60 flex items-center justify-between gap-3">
      <div>
        <div class="text-sm font-semibold text-slate-800 dark:text-slate-100">{{ t('chat.deepResearchRunningTasks', 'Running research tasks') }}</div>
        <div class="text-xs text-slate-500 dark:text-slate-400">{{ t('chat.deepResearchRunningElsewhere', 'Track active deep research jobs across conversations.') }}</div>
      </div>
      <span class="rounded-full bg-slate-100 px-2.5 py-1 text-xs text-slate-600 dark:bg-slate-800 dark:text-slate-300">{{ jobs.length }}</span>
    </div>

    <div class="max-h-64 overflow-y-auto px-3 py-3 space-y-2">
      <div v-for="job in jobs" :key="job.job_id" class="rounded-2xl border border-slate-200 bg-white px-3 py-3 dark:border-slate-800 dark:bg-slate-950/60">
        <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span class="rounded-full bg-blue-100 px-2.5 py-1 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200">{{ stageLabel(job.stage) }}</span>
              <span>{{ Math.round(job.progress || 0) }}%</span>
              <span v-if="job.iteration">{{ t('chat.deepResearchIteration', 'Iteration') }} {{ job.iteration }}</span>
            </div>
            <div class="mt-2 text-sm font-medium text-slate-800 dark:text-slate-100 break-words">{{ job.query }}</div>
            <div v-if="job.latest_action || job.latest_gap" class="mt-2 text-xs text-slate-500 dark:text-slate-400 space-y-1">
              <div v-if="job.latest_action" class="break-words">{{ job.latest_action }}</div>
              <div v-if="job.latest_gap" class="break-words text-amber-700 dark:text-amber-200">{{ job.latest_gap }}</div>
            </div>
            <div class="mt-3 h-1.5 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
              <div class="h-full bg-blue-500 transition-all duration-500" :style="{ width: `${Math.max(0, Math.min(100, Math.round(job.progress || 0)))}%` }" />
            </div>
          </div>

          <div class="flex items-center gap-2 md:flex-col md:items-end">
            <button
              v-if="job.conversation_id"
              class="rounded-full border border-slate-200 px-3 py-1.5 text-xs font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 cursor-pointer"
              @click="emit('view', job)"
            >
              {{ t('chat.deepResearchBackToTask', 'Back to task') }}
            </button>
            <button
              class="rounded-full border border-amber-200 px-3 py-1.5 text-xs font-medium text-amber-700 hover:bg-amber-50 dark:border-amber-900/60 dark:text-amber-200 dark:hover:bg-amber-950/30 cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              :disabled="cancellingJobId === job.job_id"
              @click="handleCancel(job)"
            >
              {{ cancellingJobId === job.job_id ? t('common.loading', 'Loading...') : t('chat.deepResearchCancelTask', 'Cancel') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
