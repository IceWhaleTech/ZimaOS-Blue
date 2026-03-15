<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardUIReview, UIReviewStep, UIReviewIssue } from '@/types/typeless'
import { translateCardActionLabel } from '@/utils/cardActionLabels'

const { t, te } = useI18n()

const props = defineProps<{
  card: TypelessCardUIReview
  actionLoading?: boolean
  activeActionId?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
}>()

const expanded = ref(false)
const showScreenshot = ref(false)

const overall = computed(() => props.card.overall ?? 0)
const pass = computed(() => props.card.pass ?? false)
const steps = computed(() => props.card.steps ?? [])
const issues = computed(() => props.card.issues ?? [])
const suggestions = computed(() => props.card.suggestions ?? [])
const isError = computed(() => props.card.status === 'error')
const isStreaming = computed(() => props.card._streaming === true)
const screenshotSources = computed(() => {
  const sources: string[] = []
  if (props.card.screenshot) {
    sources.push(`data:image/png;base64,${props.card.screenshot}`)
  }
  if (props.card.thumbnail_url) {
    sources.push(props.card.thumbnail_url)
  }
  if (props.card.media_url) {
    sources.push(props.card.media_url)
  }
  for (const url of props.card.screenshots || []) {
    if (url) sources.push(url)
  }
  return Array.from(new Set(sources))
})

const scoreColor = computed(() => {
  if (overall.value >= 80) return 'text-green-500'
  if (overall.value >= 60) return 'text-yellow-500'
  return 'text-red-500'
})

const passBadge = computed(() => {
  if (pass.value)
    return {
      text: t('security.scan.passed', 'PASS'),
      bg: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400',
    }
  return {
    text: t('security.scan.failed', 'FAIL'),
    bg: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400',
  }
})

const criticalCount = computed(
  () => issues.value.filter((i: UIReviewIssue) => i.severity === 'critical').length
)
const majorCount = computed(
  () => issues.value.filter((i: UIReviewIssue) => i.severity === 'major').length
)
const minorCount = computed(
  () => issues.value.filter((i: UIReviewIssue) => i.severity === 'minor').length
)

function stepIcon(step: UIReviewStep): string {
  if (step.status === 'success') return '✓'
  if (step.status === 'failed') return '✗'
  return '—'
}

function stepColor(step: UIReviewStep): string {
  if (step.status === 'success') return 'text-green-500'
  if (step.status === 'failed') return 'text-red-500'
  return 'text-gray-400'
}

function severityIcon(severity: string): string {
  if (severity === 'critical') return '🔴'
  if (severity === 'major') return '🟡'
  return '🔵'
}

function scoreBarWidth(score: number): string {
  return Math.max(0, Math.min(100, score)) + '%'
}

function scoreBarColor(score: number): string {
  if (score >= 80) return 'bg-green-500'
  if (score >= 60) return 'bg-yellow-500'
  return 'bg-red-500'
}

function isActionActive(actionId: string): boolean {
  return props.actionLoading === true && props.activeActionId === actionId
}

function isActionDisabled(action: { disabled?: boolean }): boolean {
  return props.actionLoading === true || action.disabled === true
}

function actionLabel(action: { id: string; label: string }): string {
  if (isActionActive(action.id)) return t('common.processing', 'Processing...')
  return translateCardActionLabel({
    id: action.id,
    fallback: action.label,
    t,
    te,
    scopes: ['uiReview.actions'],
  })
}

function handleAction(actionId: string, disabled = false) {
  if (props.actionLoading || disabled) return
  emit('action', actionId, props.card.id)
}
</script>

<template>
  <div
    class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm"
  >
    <!-- Error state -->
    <div v-if="isError" class="p-4">
      <div class="flex items-center gap-2 text-red-500">
        <span class="text-lg">✗</span>
        <span class="text-sm font-medium">{{ t('uiReview.error', 'UI Review Failed') }}</span>
      </div>
      <p v-if="card.message" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ card.message }}
      </p>
      <!-- Retry button for error state -->
      <div v-if="card.actions?.length" class="mt-3 flex flex-wrap gap-2">
        <button
          v-for="action in card.actions"
          :key="action.id"
          class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-60 disabled:cursor-wait"
          :class="[
            action.variant === 'primary'
              ? 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white'
              : 'bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 text-gray-700 dark:text-gray-300',
            { 'opacity-60 cursor-wait': actionLoading },
          ]"
          :disabled="isActionDisabled(action)"
          :aria-busy="isActionActive(action.id) ? 'true' : undefined"
          @click="handleAction(action.id, !!action.disabled)"
        >
          <span
            v-if="isActionActive(action.id)"
            class="mr-1 inline-block h-3 w-3 animate-spin rounded-full border border-current border-r-transparent align-[-2px]"
          />
          {{ actionLabel(action) }}
        </button>
      </div>
    </div>

    <!-- Normal state -->
    <template v-else>
      <!-- Header (always visible) -->
      <button
        class="w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors text-left"
        @click="expanded = !expanded"
      >
        <!-- Eye icon -->
        <svg
          aria-hidden="true"
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 text-gray-400 flex-shrink-0"
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

        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-900 dark:text-white truncate">
              {{ t('uiReview.title', 'UI Review') }}{{ card.url ? ': ' + card.url : '' }}
            </span>
            <span
              v-if="isStreaming"
              class="inline-block w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse flex-shrink-0"
            />
            <span
              v-if="card.device"
              class="text-[11px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-300"
              >{{ card.device }}</span
            >
            <span
              v-if="card.channel"
              class="text-[11px] px-1.5 py-0.5 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-300"
              >{{ card.channel }}</span
            >
          </div>
        </div>

        <!-- Score + Pass/Fail badge -->
        <div class="flex items-center gap-2 flex-shrink-0">
          <span class="text-lg font-bold tabular-nums" :class="scoreColor">
            {{ overall.toFixed(1) }}
          </span>
          <span class="text-xs font-medium px-1.5 py-0.5 rounded" :class="passBadge.bg">
            {{ passBadge.text }}
          </span>
          <!-- Chevron -->
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-gray-400 transition-transform"
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
        </div>
      </button>

      <!-- Expanded content -->
      <Transition name="expand">
        <div v-if="expanded" class="border-t border-gray-100 dark:border-gray-700">
          <!-- Steps -->
          <div v-if="steps.length > 0" class="divide-y divide-gray-100 dark:divide-gray-700/50">
            <div v-for="step in steps" :key="step.id" class="flex items-center gap-3 px-4 py-2.5">
              <span class="text-sm flex-shrink-0 w-4 text-center" :class="stepColor(step)">
                {{ stepIcon(step) }}
              </span>
              <span class="text-sm text-gray-700 dark:text-gray-300 flex-1">{{
                t('uiReview.steps.' + step.id, step.name)
              }}</span>
              <div class="flex items-center gap-2 flex-shrink-0">
                <span
                  v-if="step.score"
                  class="text-sm font-medium tabular-nums text-gray-600 dark:text-gray-400"
                >
                  {{ step.score.toFixed(0) }}
                </span>
                <span v-if="step.issues" class="text-xs text-gray-400">
                  {{ step.issues }} {{ t('uiReview.issues', 'issues') }}
                </span>
                <span v-if="step.status === 'skipped'" class="text-xs text-gray-400 italic">
                  {{ t('uiReview.skipped', 'skipped') }}
                </span>
              </div>
            </div>
          </div>

          <!-- Score bars -->
          <div class="px-4 py-3 space-y-2 border-t border-gray-100 dark:border-gray-700">
            <div v-if="card.visual && card.visual.score > 0" class="flex items-center gap-3">
              <span class="text-xs text-gray-500 dark:text-gray-400 w-24">{{
                t('uiReview.visual', 'Visual')
              }}</span>
              <div class="flex-1 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div
                  class="h-full rounded-full transition-all"
                  :class="scoreBarColor(card.visual.score)"
                  :style="{ width: scoreBarWidth(card.visual.score) }"
                />
              </div>
              <span
                class="text-xs font-medium tabular-nums text-gray-600 dark:text-gray-400 w-8 text-right"
                >{{ card.visual.score.toFixed(0) }}</span
              >
            </div>
            <div v-if="card.functional" class="flex items-center gap-3">
              <span class="text-xs text-gray-500 dark:text-gray-400 w-24">{{
                t('uiReview.functional', 'Functional')
              }}</span>
              <div class="flex-1 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div
                  class="h-full rounded-full transition-all"
                  :class="scoreBarColor(card.functional.score)"
                  :style="{ width: scoreBarWidth(card.functional.score) }"
                />
              </div>
              <span
                class="text-xs font-medium tabular-nums text-gray-600 dark:text-gray-400 w-8 text-right"
                >{{ card.functional.score.toFixed(0) }}</span
              >
            </div>
            <div v-if="card.accessibility" class="flex items-center gap-3">
              <span class="text-xs text-gray-500 dark:text-gray-400 w-24">{{
                t('uiReview.accessibility', 'Accessibility')
              }}</span>
              <div class="flex-1 h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div
                  class="h-full rounded-full transition-all"
                  :class="scoreBarColor(card.accessibility.score)"
                  :style="{ width: scoreBarWidth(card.accessibility.score) }"
                />
              </div>
              <span
                class="text-xs font-medium tabular-nums text-gray-600 dark:text-gray-400 w-8 text-right"
                >{{ card.accessibility.score.toFixed(0) }}</span
              >
            </div>
          </div>

          <!-- Issues -->
          <div
            v-if="issues.length > 0"
            class="px-4 py-3 border-t border-gray-100 dark:border-gray-700"
          >
            <div class="flex items-center gap-3 mb-2">
              <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
                {{ t('uiReview.issuesTitle', 'Issues') }} ({{ issues.length }})
              </span>
              <div class="flex items-center gap-2 text-xs text-gray-400">
                <span v-if="criticalCount > 0">🔴 {{ criticalCount }}</span>
                <span v-if="majorCount > 0">🟡 {{ majorCount }}</span>
                <span v-if="minorCount > 0">🔵 {{ minorCount }}</span>
              </div>
            </div>
            <div class="space-y-1.5 max-h-40 overflow-y-auto">
              <div v-for="(issue, i) in issues" :key="i" class="flex items-start gap-2 text-xs">
                <span class="flex-shrink-0 mt-0.5">{{ severityIcon(issue.severity) }}</span>
                <span class="text-gray-600 dark:text-gray-400">{{ issue.description }}</span>
              </div>
            </div>
          </div>

          <!-- Suggestions -->
          <div
            v-if="suggestions.length > 0"
            class="px-4 py-3 border-t border-gray-100 dark:border-gray-700"
          >
            <div class="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">
              {{ t('uiReview.suggestions', 'Suggestions') }}
            </div>
            <ul class="space-y-1">
              <li
                v-for="(s, i) in suggestions"
                :key="i"
                class="text-xs text-gray-500 dark:text-gray-400 flex items-start gap-1.5"
              >
                <span class="flex-shrink-0">•</span>
                <span>{{ s }}</span>
              </li>
            </ul>
          </div>

          <!-- Screenshot thumbnail -->
          <div
            v-if="screenshotSources.length > 0"
            class="px-4 py-3 border-t border-gray-100 dark:border-gray-700"
          >
            <button
              class="text-xs text-blue-600 dark:text-blue-400 hover:underline"
              @click.stop="showScreenshot = !showScreenshot"
            >
              {{
                showScreenshot
                  ? t('uiReview.hideScreenshot', 'Hide screenshot')
                  : t('uiReview.showScreenshot', 'Show screenshot')
              }}
            </button>
            <div v-if="showScreenshot" class="mt-2 space-y-2">
              <div
                v-for="(src, idx) in screenshotSources"
                :key="idx"
                class="rounded-lg overflow-hidden border border-gray-200 dark:border-gray-700"
              >
                <img
                  :src="src"
                  class="w-full"
                  :alt="t('askQuestion.browserCheckpoint.screenshotAlt', 'Page screenshot')"
                />
              </div>
            </div>
          </div>

          <!-- Action buttons -->
          <div
            v-if="card.actions?.length"
            class="px-4 py-3 border-t border-gray-100 dark:border-gray-700 flex flex-wrap gap-2"
          >
            <button
              v-for="action in card.actions"
              :key="action.id"
              class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors disabled:opacity-60 disabled:cursor-wait"
              :class="[
                action.variant === 'primary'
                  ? 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white'
                  : 'bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 text-gray-700 dark:text-gray-300',
                { 'opacity-60 cursor-wait': actionLoading },
              ]"
              :disabled="isActionDisabled(action)"
              :aria-busy="isActionActive(action.id) ? 'true' : undefined"
              @click.stop="handleAction(action.id, !!action.disabled)"
            >
              <span
                v-if="isActionActive(action.id)"
                class="mr-1 inline-block h-3 w-3 animate-spin rounded-full border border-current border-r-transparent align-[-2px]"
              />
              {{ actionLabel(action) }}
            </button>
          </div>
        </div>
      </Transition>
    </template>
  </div>
</template>

<style scoped>
.expand-enter-active,
.expand-leave-active {
  transition: all 0.2s ease;
  overflow: hidden;
}
.expand-enter-from,
.expand-leave-to {
  opacity: 0;
  max-height: 0;
}
.expand-enter-to,
.expand-leave-from {
  opacity: 1;
  max-height: 800px;
}
</style>
