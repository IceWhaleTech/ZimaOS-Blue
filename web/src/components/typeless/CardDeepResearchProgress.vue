<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardDeepResearchProgress } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardDeepResearchProgress
}>()

const mode = computed(() => props.card.mode || 'standard')
const progress = computed(() => {
  const p = props.card.progress
  if (typeof p !== 'number') return 0
  return Math.max(0, Math.min(100, Math.round(p)))
})
const stage = computed(() => props.card.stage || 'running')
const iteration = computed(() => props.card.iteration || 0)
const latestGap = computed(() => props.card.latest_gap || '')
const latestAction = computed(() => props.card.latest_action || '')
const isDone = computed(() => props.card.status === 'completed')
const isFailed = computed(() => props.card.status === 'failed')

const stageLabel = computed(() => {
  const raw = stage.value
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
  return stageMap[raw] || raw
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
</script>

<template>
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm">
    <div class="px-4 py-3 border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/60">
      <div class="flex items-center justify-between gap-3">
        <div class="text-sm font-semibold text-gray-800 dark:text-gray-100">
          {{ t('chat.deepResearchProgress', 'Deep Research Running') }}
        </div>
        <span class="px-2 py-0.5 rounded text-xs bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300">
          {{ mode }}
        </span>
      </div>
      <div v-if="card.query" class="mt-1 text-xs text-gray-500 dark:text-gray-400 truncate">
        {{ card.query }}
      </div>
    </div>
    <div class="px-4 py-3">
      <div class="flex items-center justify-between text-xs mb-2">
        <span class="text-gray-500 dark:text-gray-400">{{ stageLabel }}</span>
        <span class="font-medium text-gray-700 dark:text-gray-200">{{ progress }}%</span>
      </div>
      <div v-if="iteration || latestAction || latestGap" class="mb-2 space-y-1 text-xs text-gray-500 dark:text-gray-400">
        <div v-if="iteration">{{ t('chat.deepResearchIteration', 'Iteration') }}: {{ iteration }}</div>
        <div v-if="latestAction">{{ t('chat.deepResearchLatestAction', 'Latest action') }}: {{ latestActionLabel(latestAction) }}</div>
        <div v-if="latestGap">{{ t('chat.deepResearchLatestGap', 'Latest gap') }}: {{ latestGap }}</div>
      </div>
      <div class="h-1.5 rounded bg-gray-100 dark:bg-gray-700 overflow-hidden">
        <div
          class="h-full transition-all duration-500 ease-out"
          :class="isFailed ? 'bg-red-500' : isDone ? 'bg-emerald-500' : 'bg-blue-500'"
          :style="{ width: progress + '%' }"
        />
      </div>
    </div>
  </div>
</template>
