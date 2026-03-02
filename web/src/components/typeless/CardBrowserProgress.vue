<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface ProgressStep {
  step: string
  name: string
  status: string
  url?: string
}

const props = defineProps<{
  card: {
    type: string
    id?: string
    step?: string
    name?: string
    status?: string
    url?: string
    steps?: ProgressStep[]
    _streaming?: boolean
  }
}>()

const steps = computed<ProgressStep[]>(() => {
  if (props.card.steps && props.card.steps.length > 0) {
    return props.card.steps
  }
  if (props.card.step && props.card.name) {
    return [{
      step: props.card.step,
      name: props.card.name,
      status: props.card.status || 'running',
      url: props.card.url,
    }]
  }
  return []
})

const isRunning = computed(() => steps.value.some(s => s.status === 'running'))

const progressPercent = computed(() => {
  if (steps.value.length === 0) return 0
  const done = steps.value.filter(s => s.status === 'success' || s.status === 'failed').length
  return Math.round((done / steps.value.length) * 100)
})

const currentURL = computed(() => {
  for (let i = steps.value.length - 1; i >= 0; i--) {
    const url = steps.value[i]?.url
    if (url) return url
  }
  return ''
})

function stepIcon(status: string): string {
  if (status === 'running') return '⟳'
  if (status === 'success' || status === 'completed') return '✓'
  if (status === 'failed' || status === 'error') return '✗'
  return '○'
}

function stepColor(status: string): string {
  if (status === 'running') return 'text-blue-500'
  if (status === 'success' || status === 'completed') return 'text-emerald-500'
  if (status === 'failed' || status === 'error') return 'text-red-500'
  return 'text-gray-400'
}

function stepBg(status: string): string {
  if (status === 'running') return 'bg-blue-100 dark:bg-blue-900/30'
  if (status === 'success' || status === 'completed') return 'bg-emerald-100 dark:bg-emerald-900/30'
  if (status === 'failed' || status === 'error') return 'bg-red-100 dark:bg-red-900/30'
  return 'bg-gray-100 dark:bg-gray-700'
}
</script>

<template>
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm">
    <div class="flex items-center gap-2 px-3 py-2.5 border-b border-gray-100 dark:border-gray-700/50">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-blue-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L5 12.25l1.41-1.41 3.34 3.34 7.84-7.84L19 7.75z" />
      </svg>
      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ t('browserProgress.title') }}</span>
      <span v-if="currentURL" class="text-xs text-gray-400 truncate max-w-[220px]">{{ currentURL }}</span>
      <span v-if="isRunning" class="ml-auto inline-block w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse flex-shrink-0" />
      <span v-else class="ml-auto text-xs text-gray-400 tabular-nums">{{ progressPercent }}%</span>
    </div>

    <div class="h-0.5 bg-gray-100 dark:bg-gray-700">
      <div
        class="h-full transition-all duration-500 ease-out"
        :class="isRunning ? 'bg-blue-400 animate-pulse' : 'bg-emerald-400'"
        :style="{ width: isRunning ? '60%' : progressPercent + '%' }"
      />
    </div>

    <div class="divide-y divide-gray-50 dark:divide-gray-700/30">
      <div
        v-for="(step, i) in steps"
        :key="step.step + '-' + i"
        class="flex items-center gap-2.5 px-3 py-2"
      >
        <span
          class="w-5 h-5 rounded-full flex items-center justify-center text-xs font-medium flex-shrink-0"
          :class="[stepColor(step.status), stepBg(step.status)]"
        >
          <span v-if="step.status === 'running'" class="animate-spin">{{ stepIcon(step.status) }}</span>
          <span v-else>{{ stepIcon(step.status) }}</span>
        </span>
        <span class="text-sm text-gray-700 dark:text-gray-300 flex-1 truncate">{{ step.name || step.step }}</span>
        <span v-if="step.url" class="text-xs text-gray-400 truncate max-w-[180px]">{{ step.url }}</span>
      </div>
    </div>
  </div>
</template>
