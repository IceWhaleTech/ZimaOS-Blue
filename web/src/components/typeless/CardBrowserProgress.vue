<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePersistentDisclosureState } from '@/utils/chatCardUiState'

const { t, te } = useI18n()

interface ProgressStep {
  step: string
  name: string
  status: string
  url?: string
  recipe_name?: string
}

const localizedStepKeys: Record<string, string> = {
  start: 'start',
  navigate: 'navigate',
  snapshot: 'snapshot',
  screenshot: 'screenshot',
  recipe: 'recipe',
}

const props = defineProps<{
  card: {
    type: string
    id?: string
    step?: string
    name?: string
    status?: string
    url?: string
    recipe_name?: string
    steps?: ProgressStep[]
    _streaming?: boolean
  }
  uiStateKey?: string
}>()

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
        url: props.card.url,
        recipe_name: props.card.recipe_name,
      },
    ]
  }
  return []
})

const isRunning = computed(() => steps.value.some((s) => s.status === 'running'))

const progressPercent = computed(() => {
  if (steps.value.length === 0) return 0
  const done = steps.value.filter((s) =>
    ['success', 'completed', 'failed', 'error'].includes(s.status)
  ).length
  return Math.round((done / steps.value.length) * 100)
})

const currentURL = computed(() => {
  for (let i = steps.value.length - 1; i >= 0; i--) {
    const url = steps.value[i]?.url
    if (url) return url
  }
  return ''
})

const currentStep = computed(() => {
  return (
    [...steps.value].reverse().find((step) => step.status === 'running') ||
    steps.value[steps.value.length - 1] ||
    null
  )
})

const currentHost = computed(() => {
  const url = currentURL.value
  if (!url) return ''
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return url
  }
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

function resolveRecipeName(step: ProgressStep): string {
  if (typeof step.recipe_name === 'string' && step.recipe_name.trim()) {
    return step.recipe_name.trim()
  }
  const match = (step.name || '').match(/^Running\s+(.+)$/)
  return match?.[1]?.trim() || ''
}

function stepLabel(step: ProgressStep): string {
  const stepKey = localizedStepKeys[(step.step || '').trim().toLowerCase()]
  if (!stepKey) return step.name || step.step

  if (stepKey === 'recipe') {
    const recipe = resolveRecipeName(step)
    if (!recipe) return step.name || step.step
    const translated = t('browserProgress.steps.recipe', { recipe })
    return translated === 'browserProgress.steps.recipe' ? step.name || step.step : translated
  }

  const key = `browserProgress.steps.${stepKey}`
  const translated = t(key)
  return translated === key ? step.name || step.step : translated
}

const summaryKey = computed(
  () => props.uiStateKey || props.card.id || `browser-progress:${currentURL.value || 'default'}`
)
const { expanded, toggleExpanded } = usePersistentDisclosureState(summaryKey, false)
const browserProgressTitle = computed(() =>
  te('browserProgress.title') ? String(t('browserProgress.title')) : 'Browser progress'
)
const summaryTitle = computed(() => {
  if (!currentStep.value) return browserProgressTitle.value
  return stepLabel(currentStep.value)
})
const summaryBadge = computed(() => {
  if (isRunning.value) {
    return te('common.processing') ? String(t('common.processing')) : 'Processing...'
  }
  return `${progressPercent.value}%`
})
</script>

<template>
  <div
    class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm"
  >
    <button
      class="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/30"
      :aria-expanded="expanded ? 'true' : 'false'"
      @click="toggleExpanded"
    >
      <span
        class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9.75 17L5 12.25l1.41-1.41 3.34 3.34 7.84-7.84L19 7.75z"
          />
        </svg>
      </span>
      <span class="min-w-0 flex-1">
        <span
          class="block text-[11px] font-semibold uppercase tracking-[0.08em] text-gray-500 dark:text-gray-400"
          >{{ browserProgressTitle }}</span
        >
        <span class="block truncate text-sm font-medium text-gray-800 dark:text-gray-100">
          {{ summaryTitle }}
        </span>
        <span
          v-if="currentHost || currentURL"
          class="mt-0.5 block truncate text-xs text-gray-500 dark:text-gray-400"
        >
          {{ currentHost || currentURL }}
        </span>
      </span>
      <span class="flex flex-shrink-0 items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span
          class="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10 tabular-nums"
          :class="{ 'text-blue-700 dark:text-blue-300': isRunning }"
        >
          {{ summaryBadge }}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-gray-400 transition-transform duration-200"
          :class="{ 'rotate-180': expanded }"
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

    <transition name="browser-progress-content">
      <div v-if="expanded" class="border-t border-gray-100 dark:border-gray-700/60">
        <div class="px-3 py-2.5 border-b border-gray-100 dark:border-gray-700/50">
          <div class="flex items-center gap-2">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{
              browserProgressTitle
            }}</span>
            <span
              v-if="isRunning"
              class="ml-auto inline-block w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse flex-shrink-0"
            />
            <span v-else class="ml-auto text-xs text-gray-400 tabular-nums"
              >{{ progressPercent }}%</span
            >
          </div>
          <div
            v-if="currentURL"
            class="mt-1 text-xs text-gray-400 whitespace-normal break-all leading-relaxed"
          >
            {{ currentURL }}
          </div>
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
              <span v-if="step.status === 'running'" class="animate-spin">{{
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
                v-if="step.url"
                class="text-xs text-gray-400 whitespace-normal break-all leading-relaxed"
              >
                {{ step.url }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.browser-progress-content-enter-active,
.browser-progress-content-leave-active {
  overflow: hidden;
  transition:
    max-height 220ms ease,
    opacity 180ms ease,
    transform 180ms ease;
}

.browser-progress-content-enter-from,
.browser-progress-content-leave-to {
  max-height: 0;
  opacity: 0;
  transform: translateY(-4px);
}

.browser-progress-content-enter-to,
.browser-progress-content-leave-from {
  max-height: 960px;
  opacity: 1;
  transform: translateY(0);
}
</style>
