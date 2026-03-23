<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardDeepResearchProgress } from '@/types/typeless'
import { useDeepResearchJobsStore } from '@/stores/deepResearchJobs'
import { localizeDeepResearchMode } from '@/utils/deepResearchText'

const { t } = useI18n()
const deepResearchJobs = useDeepResearchJobsStore()

const props = defineProps<{
  card: TypelessCardDeepResearchProgress
}>()

const cancelling = ref(false)

const liveJob = computed(() => {
  const jobId = props.card.job_id?.trim()
  return jobId ? deepResearchJobs.jobMap[jobId] || null : null
})

const effectiveCard = computed<TypelessCardDeepResearchProgress>(() => ({
  ...props.card,
  ...(liveJob.value || {}),
  type: 'deep-research-progress',
}))

const progress = computed(() => {
  const value = Number(effectiveCard.value.progress)
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, Math.round(value)))
})
const stage = computed(() => effectiveCard.value.stage || 'running')
const iteration = computed(() => effectiveCard.value.iteration || 0)
const latestGap = computed(() => effectiveCard.value.latest_gap || '')
const latestAction = computed(() => effectiveCard.value.latest_action || '')
const query = computed(() => effectiveCard.value.query || '')
const modeLabel = computed(() =>
  localizeDeepResearchMode(effectiveCard.value.mode || 'standard', t)
)
const conversationId = computed(() => effectiveCard.value.conversation_id || '')
const jobId = computed(() => effectiveCard.value.job_id || '')
const status = computed(() => effectiveCard.value.status || 'running')
const isDone = computed(() => status.value === 'completed')
const isFailed = computed(() => status.value === 'failed')
const isCancelled = computed(() => status.value === 'cancelled')
const isTerminal = computed(() => isDone.value || isFailed.value || isCancelled.value)

const stageLabel = computed(() => {
  const stageMap: Record<string, string> = {
    intake: t('chat.deepResearchStageIntake', 'Intake'),
    planning: t('chat.deepResearchStagePlanning', 'Planning'),
    retrieve: t('chat.deepResearchStageRetrieve', 'Retrieving'),
    verify: t('chat.deepResearchStageVerify', 'Verifying'),
    synthesize: t('chat.deepResearchStageSynthesize', 'Synthesizing'),
    completed: t('chat.deepResearchStageCompleted', 'Completed'),
    failed: t('chat.deepResearchStageFailed', 'Failed'),
    cancelled: t('chat.deepResearchStageCancelled', 'Cancelled'),
  }
  return stageMap[stage.value] || stage.value
})

const statusClass = computed(() => {
  if (isFailed.value) return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200'
  if (isCancelled.value)
    return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200'
  if (isDone.value)
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-200'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-200'
})

const progressClass = computed(() => {
  if (isFailed.value) return 'bg-red-500'
  if (isCancelled.value) return 'bg-amber-500'
  if (isDone.value) return 'bg-emerald-500'
  return 'bg-blue-500'
})

function latestActionLabel(action?: string): string {
  switch (action) {
    case 'augment_query':
      return t('chat.deepResearchActionAugmentQuery', 'Augmenting query')
    case 'initial_retrieve':
      return t('chat.deepResearchActionInitialRetrieve', 'Running initial retrieval')
    case 'followup_retrieve':
      return t('chat.deepResearchActionFollowupRetrieve', 'Running follow-up retrieval')
    case 'verification':
      return t('chat.deepResearchActionVerification', 'Verifying evidence')
    case 'verification_completed':
      return t('chat.deepResearchActionVerificationCompleted', 'Verification completed')
    case 'followup_planned':
      return t('chat.deepResearchActionFollowupPlanned', 'Follow-up planned')
    case 'loop_stopped':
      return t('chat.deepResearchActionLoopStopped', 'Research loop stopped')
    case 'synthesizing':
      return t('chat.deepResearchActionSynthesizing', 'Synthesizing report')
    case 'completed':
      return t('chat.deepResearchActionCompleted', 'Completed')
    default:
      return action || ''
  }
}

async function handleView() {
  if (!jobId.value || !conversationId.value) return
  await deepResearchJobs.openJob(jobId.value, conversationId.value)
}

async function handleCancel() {
  if (!jobId.value || isTerminal.value || cancelling.value) return
  cancelling.value = true
  try {
    await deepResearchJobs.cancelJob(jobId.value)
  } finally {
    cancelling.value = false
  }
}
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
              {{ t('chat.deepResearchProgress', 'Deep Research Running') }}
            </span>
            <span class="rounded-full px-2 py-0.5" :class="statusClass">{{ stageLabel }}</span>
            <span
              class="rounded-full px-2 py-0.5 bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
              >{{ modeLabel }}</span
            >
          </div>
          <div
            v-if="query"
            class="mt-1.5 text-[13px] font-semibold text-slate-800 dark:text-slate-100 break-words"
          >
            {{ query }}
          </div>
          <div
            class="mt-1.5 flex flex-wrap items-center gap-2.5 text-xs text-slate-500 dark:text-slate-400"
          >
            <span>{{ progress }}%</span>
            <span v-if="iteration"
              >{{ t('chat.deepResearchIteration', 'Iteration') }} {{ iteration }}</span
            >
            <span v-if="latestAction">{{ latestActionLabel(latestAction) }}</span>
          </div>
        </div>
        <div class="flex flex-col items-end gap-2">
          <span class="text-xs font-medium text-slate-600 dark:text-slate-300"
            >{{ progress }}%</span
          >
          <div class="flex items-center gap-2">
            <button
              v-if="conversationId"
              class="rounded-full border border-slate-200 px-2.5 py-1 text-[11px] font-medium text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800 cursor-pointer"
              @click="handleView"
            >
              {{ t('chat.deepResearchViewTask', 'View task') }}
            </button>
            <button
              v-if="!isTerminal && jobId"
              class="rounded-full border border-amber-200 px-2.5 py-1 text-[11px] font-medium text-amber-700 hover:bg-amber-50 dark:border-amber-900/60 dark:text-amber-200 dark:hover:bg-amber-950/30 cursor-pointer disabled:cursor-not-allowed disabled:opacity-60"
              :disabled="cancelling"
              @click="handleCancel"
            >
              {{
                cancelling
                  ? t('common.loading', 'Loading...')
                  : t('chat.deepResearchCancelTask', 'Cancel')
              }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="px-3.5 py-2.5 space-y-2.5">
      <div class="h-2 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
        <div
          class="h-full transition-all duration-500 ease-out"
          :class="progressClass"
          :style="{ width: `${progress}%` }"
        />
      </div>

      <div class="grid gap-2 md:grid-cols-2 text-xs text-slate-500 dark:text-slate-400">
        <div v-if="latestAction" class="rounded-lg bg-slate-50 px-2.5 py-2 dark:bg-slate-900/60">
          <div class="font-medium text-slate-600 dark:text-slate-300">
            {{ t('chat.deepResearchLatestAction', 'Latest action') }}
          </div>
          <div class="mt-1 break-words">{{ latestActionLabel(latestAction) }}</div>
        </div>
        <div
          v-if="latestGap"
          class="rounded-lg bg-amber-50 px-2.5 py-2 text-amber-700 dark:bg-amber-950/30 dark:text-amber-200"
        >
          <div class="font-medium">{{ t('chat.deepResearchLatestGap', 'Latest gap') }}</div>
          <div class="mt-1 break-words">{{ latestGap }}</div>
        </div>
      </div>

      <div v-if="!conversationId" class="text-xs text-slate-500 dark:text-slate-400">
        {{
          t('chat.deepResearchRunningElsewhere', 'This research task is running in the background.')
        }}
      </div>
    </div>
  </div>
</template>
