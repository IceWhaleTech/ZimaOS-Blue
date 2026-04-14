<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

interface ProgressStep {
  step: string
  name: string
  status: string
  url?: string
  score?: number
}

const props = defineProps<{
  card: {
    type: string
    id?: string
    // Single step (legacy)
    step?: string
    name?: string
    status?: string
    url?: string
    score?: number
    // Merged steps
    steps?: ProgressStep[]
    _streaming?: boolean
  }
}>()

const { t } = useI18n()

// Normalize: support both single-step and merged-steps format
const steps = computed<ProgressStep[]>(() => {
  if (props.card.steps && props.card.steps.length > 0) {
    return props.card.steps
  }
  // Single step fallback
  if (props.card.step && props.card.name) {
    return [
      {
        step: props.card.step,
        name: props.card.name,
        status: props.card.status || 'running',
        url: props.card.url,
        score: props.card.score,
      },
    ]
  }
  return []
})

const url = computed(() => {
  // Get URL from first step that has one
  for (const s of steps.value) {
    if (s.url) return s.url
  }
  return ''
})

const isRunning = computed(() => steps.value.some((s) => s.status === 'running'))

function stepIcon(status: string): string {
  if (status === 'running') return '⟳'
  if (status === 'success') return '✓'
  if (status === 'failed') return '✗'
  return '—'
}

function stepColor(status: string): string {
  if (status === 'running') return 'text-blue-500'
  if (status === 'success') return 'text-green-500'
  if (status === 'failed') return 'text-red-500'
  return 'text-gray-400'
}

function stepBg(status: string): string {
  if (status === 'running') return 'bg-blue-100 dark:bg-blue-900/30'
  if (status === 'success') return 'bg-green-100 dark:bg-green-900/30'
  if (status === 'failed') return 'bg-red-100 dark:bg-red-900/30'
  return 'bg-gray-100 dark:bg-gray-700'
}
</script>

<template>
  <div
    class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm"
  >
    <!-- Header -->
    <div class="px-3 py-2.5 border-b border-gray-100 dark:border-gray-700/50">
      <div class="flex items-center gap-2">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-gray-400 flex-shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
          />
        </svg>
        <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ t('uiReview.reviewing', 'Reviewing UI') }}
        </span>
        <span
          v-if="isRunning"
          class="ms-auto inline-block w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse flex-shrink-0"
        />
      </div>
      <div
        v-if="url"
        class="mt-1 ps-6 text-xs text-gray-400 whitespace-normal break-all leading-relaxed"
      >
        {{ url }}
      </div>
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
        <div
          class="min-w-0 flex-1 text-sm text-gray-700 dark:text-gray-300 whitespace-normal break-words leading-relaxed"
        >
          {{ t('uiReview.steps.' + step.step, step.name) }}
        </div>
        <span
          v-if="step.score != null"
          class="text-xs font-medium tabular-nums"
          :class="stepColor(step.status)"
        >
          {{ step.score }}
        </span>
      </div>
    </div>
  </div>
</template>
