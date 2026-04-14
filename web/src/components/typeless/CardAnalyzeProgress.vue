<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

interface ProgressStep {
  step: string
  name: string
  status: string
  detail?: string
  current?: number
  total?: number
  source_kind?: string
  source_label?: string
  char_count?: number
  result_count?: number
}

const props = defineProps<{
  card: {
    type: string
    id?: string
    step?: string
    name?: string
    status?: string
    detail?: string
    current?: number
    total?: number
    source_kind?: string
    source_label?: string
    char_count?: number
    result_count?: number
    steps?: ProgressStep[]
    _streaming?: boolean
  }
}>()

const { t } = useI18n()

const localizedStepKeys: Record<string, string> = {
  data_collection: 'data_collection',
  text_input: 'text_input',
  doc_extract: 'doc_extract',
  analysis: 'analysis',
  report: 'report',
  save_report: 'save_report',
}

// Normalize: support both single-step and merged-steps format
const steps = computed<ProgressStep[]>(() => {
  if (props.card.steps && props.card.steps.length > 0) {
    return props.card.steps
  }
  if (props.card.step && props.card.name) {
    return [
      {
        step: props.card.step,
        name: props.card.name,
        status: props.card.status || 'running',
        detail: props.card.detail,
        current: props.card.current,
        total: props.card.total,
        source_kind: props.card.source_kind,
        source_label: props.card.source_label,
        char_count: props.card.char_count,
        result_count: props.card.result_count,
      },
    ]
  }
  return []
})

const isRunning = computed(() => steps.value.some((s) => s.status === 'running'))

function isDoneStatus(status: string): boolean {
  return (
    status === 'success' ||
    status === 'failed' ||
    status === 'error' ||
    status === 'completed' ||
    status === 'skipped'
  )
}

// Overall progress percentage based on completed steps
const progressPercent = computed(() => {
  if (steps.value.length === 0) return 0
  const done = steps.value.filter((s) => isDoneStatus(s.status)).length
  return Math.round((done / steps.value.length) * 100)
})

function stepIcon(status: string): string {
  if (status === 'running') return '⟳'
  if (status === 'success' || status === 'completed') return '✓'
  if (status === 'failed' || status === 'error') return '✗'
  if (status === 'skipped') return '↷'
  return '○'
}

function stepColor(status: string): string {
  if (status === 'running') return 'text-indigo-500'
  if (status === 'success' || status === 'completed') return 'text-emerald-500'
  if (status === 'failed' || status === 'error') return 'text-red-500'
  if (status === 'skipped') return 'text-gray-500'
  return 'text-gray-400'
}

function stepBg(status: string): string {
  if (status === 'running') return 'bg-indigo-100 dark:bg-indigo-900/30'
  if (status === 'success' || status === 'completed') return 'bg-emerald-100 dark:bg-emerald-900/30'
  if (status === 'failed' || status === 'error') return 'bg-red-100 dark:bg-red-900/30'
  if (status === 'skipped') return 'bg-gray-100 dark:bg-gray-700'
  return 'bg-gray-100 dark:bg-gray-700'
}

function stepLabel(step: ProgressStep): string {
  const stepKey = localizedStepKeys[(step.step || '').trim()]
  if (!stepKey) return step.name || step.step
  const key = 'analyze.steps.' + stepKey
  const translated = t(key, step.name)
  return translated === key ? step.name : translated
}

function stepDetail(step: ProgressStep): string {
  const parts: string[] = []
  if (step.source_label) {
    parts.push(step.source_label)
  }
  if (step.detail) {
    parts.push(step.detail)
  } else if (typeof step.result_count === 'number') {
    parts.push(t('analyze.meta.results', { count: step.result_count }))
  } else if (typeof step.char_count === 'number') {
    parts.push(t('analyze.meta.chars', { count: step.char_count }))
  }
  return parts.join(' · ')
}

function stepCount(step: ProgressStep): string {
  if (typeof step.current === 'number' && typeof step.total === 'number' && step.total > 0) {
    return `${step.current}/${step.total}`
  }
  return ''
}
</script>

<template>
  <div
    class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm"
  >
    <!-- Header -->
    <div
      class="flex items-center gap-2 px-3 py-2.5 border-b border-gray-100 dark:border-gray-700/50"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-4 w-4 text-indigo-400 flex-shrink-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
        />
      </svg>
      <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
        {{ t('analyze.analyzing', 'Analyzing') }}
      </span>
      <span
        v-if="isRunning"
        class="ms-auto inline-block w-1.5 h-1.5 bg-indigo-500 rounded-full animate-pulse flex-shrink-0"
      />
      <span
        v-else
        class="ms-auto text-xs text-gray-400 tabular-nums"
      >{{ progressPercent }}%</span>
    </div>

    <!-- Progress bar -->
    <div class="h-0.5 bg-gray-100 dark:bg-gray-700">
      <div
        class="h-full transition-all duration-500 ease-out"
        :class="isRunning ? 'bg-indigo-400 animate-pulse' : 'bg-emerald-400'"
        :style="{ width: isRunning ? '60%' : progressPercent + '%' }"
      />
    </div>

    <!-- Steps -->
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
          <span
            v-if="step.status === 'running'"
            class="animate-spin"
          >{{
            stepIcon(step.status)
          }}</span>
          <span v-else>{{ stepIcon(step.status) }}</span>
        </span>
        <div class="min-w-0 flex-1">
          <div
            class="text-sm text-gray-700 dark:text-gray-300 whitespace-normal break-words leading-relaxed"
          >
            {{ stepLabel(step) }}
          </div>
          <div
            v-if="stepDetail(step)"
            class="text-xs text-gray-400 whitespace-normal break-words leading-relaxed"
          >
            {{ stepDetail(step) }}
          </div>
        </div>
        <span
          v-if="stepCount(step)"
          class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium text-gray-500 bg-gray-100 dark:bg-gray-700 dark:text-gray-300 tabular-nums flex-shrink-0"
        >
          {{ stepCount(step) }}
        </span>
      </div>
    </div>
  </div>
</template>
