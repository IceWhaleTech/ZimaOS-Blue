<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/stores/chat'

const { t, te } = useI18n()
const chatStore = useChatStore()

const question = computed(() => {
  const q = chatStore.pendingQuestion
  console.log('[AskUserQuestionDialog] pendingQuestion changed:', q)
  return q
})
const activeTab = ref(0)

// Per-question answers: { [questionId]: { selected: string[], otherText: string } }
const answers = ref<Record<string, { selected: string[]; otherText: string }>>({})

// Current question index (1-based for display)
const currentIndex = computed(() => activeTab.value + 1)
const totalQuestions = computed(() => question.value?.questions.length || 0)
const isLastQuestion = computed(() => activeTab.value >= totalQuestions.value - 1)

// Check if we can use quick-submit mode: single question, single-select, short options
const isQuickMode = computed(() => {
  if (!question.value) return false
  if (checkpointContext.value) return false
  const q = question.value
  if (q.questions.length !== 1) return false
  const first = q.questions[0]
  if (!first) return false
  // multi_select is true for checkbox, false or undefined for radio
  if (first.multi_select === true) return false
  if (!first.options || first.options.length === 0) return false
  if (first.options.length > 4) return false
  // Check if all options are short enough
  const maxLen = 20
  for (const opt of first.options) {
    const label = opt.label || ''
    if (label.length > maxLen) return false
  }
  return true
})

// Reset answers when a new question arrives
watch(question, (q) => {
  if (q) {
    activeTab.value = 0
    const init: Record<string, { selected: string[]; otherText: string }> = {}
    for (const item of q.questions) {
      init[item.id] = { selected: [], otherText: '' }
    }
    answers.value = init
  }
})

const currentQuestion = computed(() => question.value?.questions[activeTab.value])

const checkpointContext = computed(() => {
  const ctx = (question.value as any)?.context
  if (!ctx || ctx.kind !== 'browser_checkpoint') return null
  return ctx as {
    required?: boolean
    risk_level?: string
    step?: string
    action?: string
    url?: string
    site_origin?: string
    screenshot?: { mime_type?: string; data?: string; url?: string }
  }
})

const checkpointScreenshotSrc = computed(() => {
  const shot = checkpointContext.value?.screenshot
  if (!shot) return ''
  if (shot.data) {
    const mime = shot.mime_type || 'image/png'
    return `data:${mime};base64,${shot.data}`
  }
  return shot.url || ''
})

const isCheckpointQuestion = computed(() => !!checkpointContext.value)
const dismissLabel = computed(() =>
  isCheckpointQuestion.value ? t('askQuestion.browserCheckpoint.cancel') : t('askQuestion.skip')
)
const submitLabel = computed(() =>
  isCheckpointQuestion.value
    ? checkpointText(
        'askQuestion.browserCheckpoint.continueOnce',
        String(t('askQuestion.browserCheckpoint.continue'))
      )
    : t('askQuestion.submit')
)
const checkpointRiskLabel = computed(() => {
  const risk = (checkpointContext.value?.risk_level || 'high').toLowerCase()
  const key = `execCard.risk.${risk}`
  return te(key) ? t(key) : risk
})
const checkpointSiteOrigin = computed(() => checkpointContext.value?.site_origin?.trim() || '')

function checkpointText(key: string, fallback: string, named?: Record<string, string>): string {
  if (!te(key)) return fallback
  return String(named ? t(key, named) : t(key))
}

function checkpointOptionValue(opt: { label?: string; value?: string }): string {
  return String(opt.value || opt.label || '')
    .trim()
    .toLowerCase()
}

function optionLabel(opt: { label?: string; value?: string }): string {
  if (!isCheckpointQuestion.value) return opt.label || ''
  switch (checkpointOptionValue(opt)) {
    case 'continue':
      return checkpointText(
        'askQuestion.browserCheckpoint.continueOnce',
        String(opt.label || t('askQuestion.browserCheckpoint.continue'))
      )
    case 'allow_site':
      return checkpointText('askQuestion.browserCheckpoint.allowSite', String(opt.label || 'Allow'))
    case 'cancel':
      return t('askQuestion.browserCheckpoint.cancel')
    default:
      return opt.label || ''
  }
}

function optionDescription(opt: { description?: string; value?: string; label?: string }): string {
  if (!isCheckpointQuestion.value) return opt.description || ''
  switch (checkpointOptionValue(opt)) {
    case 'continue':
      return checkpointText(
        'askQuestion.browserCheckpoint.continueDescription',
        String(opt.description || '')
      )
    case 'allow_site':
      if (!checkpointSiteOrigin.value) return opt.description || ''
      return checkpointText(
        'askQuestion.browserCheckpoint.allowSiteDescription',
        String(opt.description || checkpointSiteOrigin.value),
        { site: checkpointSiteOrigin.value }
      )
    case 'cancel':
      return checkpointText(
        'askQuestion.browserCheckpoint.cancelDescription',
        String(opt.description || '')
      )
    default:
      return opt.description || ''
  }
}

function isSelected(qId: string, value: string): boolean {
  return answers.value[qId]?.selected.includes(value) ?? false
}

function toggleOption(qId: string, value: string, multiSelect: boolean) {
  const ans = answers.value[qId]
  if (!ans) return
  if (multiSelect) {
    const idx = ans.selected.indexOf(value)
    if (idx >= 0) ans.selected.splice(idx, 1)
    else ans.selected.push(value)
  } else {
    ans.selected = [value]
    if (isCheckpointQuestion.value) {
      submit()
      return
    }
    // In quick mode, auto-submit after selecting an option
    if (isQuickMode.value) {
      submit()
    }
    // Auto-advance to next question for single-select in multi-question mode
    if (!isQuickMode.value && !isLastQuestion.value && totalQuestions.value > 1) {
      activeTab.value++
    }
  }
}

function isOtherSelected(qId: string): boolean {
  return answers.value[qId]?.selected.includes('__other__') ?? false
}

function isTextOnlyQuestion(q: { options?: Array<unknown> } | undefined): boolean {
  return !!q && (!q.options || q.options.length === 0)
}

function getOrCreateAnswer(qId: string) {
  if (!answers.value[qId]) {
    answers.value[qId] = { selected: [], otherText: '' }
  }
  return answers.value[qId]
}

function toggleOther(qId: string, multiSelect: boolean) {
  const ans = getOrCreateAnswer(qId)
  if (multiSelect) {
    const idx = ans.selected.indexOf('__other__')
    if (idx >= 0) {
      ans.selected.splice(idx, 1)
      ans.otherText = ''
    } else {
      ans.selected.push('__other__')
    }
    return
  }
  const currentlySelected = ans.selected.includes('__other__')
  if (currentlySelected) {
    ans.selected = []
    ans.otherText = ''
    return
  }
  // Single-select "Other" should not auto-submit or auto-advance.
  ans.selected = ['__other__']
}

function isAnswerComplete(
  ans: { selected: string[]; otherText: string } | undefined,
  q?: { options?: Array<unknown> }
): boolean {
  if (!ans) return false
  if (isTextOnlyQuestion(q)) {
    return ans.otherText.trim() !== ''
  }
  const hasNormalOption = ans.selected.some((v) => v !== '__other__')
  const hasOther = ans.selected.includes('__other__') && ans.otherText.trim() !== ''
  return hasNormalOption || hasOther
}

// Check if current question is answered
const currentQuestionAnswered = computed(() => {
  if (!currentQuestion.value) return false
  return isAnswerComplete(answers.value[currentQuestion.value.id], currentQuestion.value)
})

const canSubmit = computed(() => {
  if (!question.value) return false
  // For single question, check current
  if (question.value.questions.length === 1) {
    return currentQuestionAnswered.value
  }
  // For multiple questions, all must be answered to submit
  return question.value.questions.every((q) => {
    return isAnswerComplete(answers.value[q.id], q)
  })
})

function isTabAnswered(qId: string): boolean {
  const q = question.value?.questions.find((item) => item.id === qId)
  return isAnswerComplete(answers.value[qId], q)
}

// Countdown timer
const remainingSeconds = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

watch(question, (q) => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  if (q) {
    const updateRemaining = () => {
      const ms = q.expires_at - Date.now()
      remainingSeconds.value = Math.max(0, Math.ceil(ms / 1000))
      if (remainingSeconds.value <= 0 && countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = null
        dismiss()
      }
    }
    updateRemaining()
    countdownTimer = setInterval(updateRemaining, 1000)
  }
})

function submit() {
  if (!question.value || !canSubmit.value) return
  const result = question.value.questions.map((q) => {
    const ans = answers.value[q.id] || { selected: [], otherText: '' }
    // Filter out __other__ and deduplicate (keep only one __other__ if present)
    const otherSelected = ans.selected.includes('__other__')
    const textOnly = isTextOnlyQuestion(q)
    const selected = ans.selected
      .filter((v) => v !== '__other__')
      .filter((v, idx, arr) => arr.indexOf(v) === idx) // deduplicate regular options
    return {
      question_id: q.id,
      selected,
      other_text: textOnly || otherSelected ? ans.otherText : '',
    }
  })
  chatStore.submitQuestionAnswers(result)
}

function dismiss() {
  if (isCheckpointQuestion.value) {
    const q = currentQuestion.value
    if (q) {
      answers.value[q.id] = { selected: ['cancel'], otherText: '' }
      submit()
      return
    }
  }
  chatStore.dismissQuestion()
}
</script>

<template>
  <Teleport to="body">
    <Transition name="askq-fade">
      <div
        v-if="question"
        class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 backdrop-blur-sm"
      >
        <div
          class="w-full max-w-lg mx-4 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden"
        >
          <!-- Header -->
          <div
            class="flex items-center gap-3 px-5 py-4 border-b border-gray-100 dark:border-gray-700 bg-blue-50 dark:bg-blue-900/20"
          >
            <div
              class="flex-shrink-0 w-10 h-10 rounded-full bg-blue-100 dark:bg-blue-800/40 flex items-center justify-center"
            >
              <svg
                class="w-5 h-5 text-blue-600 dark:text-blue-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="text-xs font-semibold text-gray-900 dark:text-white">
                {{ t('askQuestion.title') }}
              </h3>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('askQuestion.subtitle') }}
              </p>
            </div>
            <span
              v-if="remainingSeconds > 0"
              class="text-xs text-gray-400 dark:text-gray-500 tabular-nums"
            >
              {{ t('askQuestion.timeout', { seconds: remainingSeconds }) }}
            </span>
          </div>

          <!-- Step indicator (if multiple questions) -->
          <div
            v-if="totalQuestions > 1"
            class="px-5 py-3 border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50"
          >
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">
                {{ t('askQuestion.step', { current: currentIndex, total: totalQuestions }) }}
              </span>
              <span class="text-xs text-gray-400 dark:text-gray-500">
                {{ Math.round((currentIndex / totalQuestions) * 100) }}%
              </span>
            </div>
            <!-- Progress bar -->
            <div class="h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
              <div
                class="h-full bg-blue-500 transition-all duration-300"
                :style="{ width: `${(currentIndex / totalQuestions) * 100}%` }"
              />
            </div>
            <!-- Step dots -->
            <div class="flex justify-center gap-1.5 mt-3">
              <button
                v-for="(q, idx) in question.questions"
                :key="q.id"
                class="w-2 h-2 rounded-full transition-colors"
                :class="
                  idx === activeTab
                    ? 'bg-blue-500'
                    : isTabAnswered(q.id)
                      ? 'bg-green-500'
                      : 'bg-gray-300 dark:bg-gray-600'
                "
                @click="activeTab = idx"
              />
            </div>
          </div>

          <!-- Question body - show current question only -->
          <div v-if="currentQuestion" class="px-4 py-3 space-y-2.5 max-h-[68vh] overflow-y-auto">
            <p class="text-xs font-medium text-gray-800 dark:text-gray-200">
              {{ currentQuestion.question }}
            </p>
            <p
              v-if="currentQuestion.detail"
              class="text-xs text-amber-700 dark:text-amber-300 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-md px-2 py-1"
            >
              ❕ {{ currentQuestion.detail }}
            </p>
            <div
              v-if="checkpointContext"
              class="space-y-1.5 rounded-md border border-blue-200 dark:border-blue-800 bg-blue-50/70 dark:bg-blue-900/20 px-2.5 py-2"
            >
              <div class="text-xs text-blue-700 dark:text-blue-200 font-medium">
                {{ t('askQuestion.browserCheckpoint.title') }}
              </div>
              <div class="text-xs text-blue-700 dark:text-blue-200">
                {{ t('askQuestion.browserCheckpoint.riskLevel') }}: {{ checkpointRiskLabel }}
              </div>
              <div v-if="checkpointContext.step" class="text-xs text-blue-700 dark:text-blue-200">
                {{ t('askQuestion.browserCheckpoint.step') }}: {{ checkpointContext.step }}
              </div>
              <div v-if="checkpointContext.action" class="text-xs text-blue-700 dark:text-blue-200">
                {{ t('askQuestion.browserCheckpoint.action') }}: {{ checkpointContext.action }}
              </div>
              <div
                v-if="checkpointContext.url"
                class="text-xs text-blue-700 dark:text-blue-200 break-all"
              >
                {{ t('askQuestion.browserCheckpoint.url') }}: {{ checkpointContext.url }}
              </div>
              <img
                v-if="checkpointScreenshotSrc"
                :src="checkpointScreenshotSrc"
                :alt="t('askQuestion.browserCheckpoint.screenshotAlt')"
                class="mt-1.5 w-full max-h-56 object-contain rounded border border-blue-200 dark:border-blue-700 bg-white/80 dark:bg-gray-900/40"
              />
            </div>

            <!-- Options -->
            <div class="space-y-2">
              <label
                v-for="opt in currentQuestion.options"
                :key="opt.label"
                class="flex items-start gap-2.5 p-2.5 rounded-lg border cursor-pointer transition-colors"
                :class="
                  isSelected(currentQuestion.id, opt.value || opt.label)
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 dark:border-blue-500'
                    : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                "
                @click="
                  toggleOption(
                    currentQuestion.id,
                    opt.value || opt.label,
                    !!currentQuestion.multi_select
                  )
                "
              >
                <!-- Radio / Checkbox indicator -->
                <div class="flex-shrink-0 mt-0.5">
                  <div
                    v-if="!currentQuestion.multi_select"
                    class="w-4 h-4 rounded-full border-2 flex items-center justify-center"
                    :class="
                      isSelected(currentQuestion.id, opt.value || opt.label)
                        ? 'border-blue-500'
                        : 'border-gray-300 dark:border-gray-600'
                    "
                  >
                    <div
                      v-if="isSelected(currentQuestion.id, opt.value || opt.label)"
                      class="w-2 h-2 rounded-full bg-blue-500"
                    />
                  </div>
                  <div
                    v-else
                    class="w-4 h-4 rounded border-2 flex items-center justify-center"
                    :class="
                      isSelected(currentQuestion.id, opt.value || opt.label)
                        ? 'border-blue-500 bg-blue-500'
                        : 'border-gray-300 dark:border-gray-600'
                    "
                  >
                    <svg
                      v-if="isSelected(currentQuestion.id, opt.value || opt.label)"
                      class="w-3 h-3 text-white"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </div>
                </div>
                <div class="flex-1 min-w-0">
                  <span class="text-xs font-medium text-gray-800 dark:text-gray-200">{{
                    optionLabel(opt)
                  }}</span>
                  <p
                    v-if="optionDescription(opt)"
                    class="text-xs text-gray-500 dark:text-gray-400 mt-0.5"
                  >
                    {{ optionDescription(opt) }}
                  </p>
                </div>
              </label>

              <!-- Other option -->
              <div
                v-if="!isCheckpointQuestion && isTextOnlyQuestion(currentQuestion)"
                class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-900 p-2.5"
              >
                <input
                  v-model="getOrCreateAnswer(currentQuestion.id).otherText"
                  type="text"
                  class="w-full px-2.5 py-1 text-xs rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
                  :placeholder="t('askQuestion.otherPlaceholder')"
                  @click.stop
                />
              </div>

              <label
                v-else-if="!isCheckpointQuestion"
                class="flex items-start gap-2.5 p-2.5 rounded-lg border cursor-pointer transition-colors"
                :class="
                  isOtherSelected(currentQuestion.id)
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 dark:border-blue-500'
                    : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
                "
                @click="toggleOther(currentQuestion.id, !!currentQuestion.multi_select)"
              >
                <div class="flex-shrink-0 mt-0.5">
                  <div
                    v-if="!currentQuestion.multi_select"
                    class="w-4 h-4 rounded-full border-2 flex items-center justify-center"
                    :class="
                      isOtherSelected(currentQuestion.id)
                        ? 'border-blue-500'
                        : 'border-gray-300 dark:border-gray-600'
                    "
                  >
                    <div
                      v-if="isOtherSelected(currentQuestion.id)"
                      class="w-2 h-2 rounded-full bg-blue-500"
                    />
                  </div>
                  <div
                    v-else
                    class="w-4 h-4 rounded border-2 flex items-center justify-center"
                    :class="
                      isOtherSelected(currentQuestion.id)
                        ? 'border-blue-500 bg-blue-500'
                        : 'border-gray-300 dark:border-gray-600'
                    "
                  >
                    <svg
                      v-if="isOtherSelected(currentQuestion.id)"
                      class="w-3 h-3 text-white"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </div>
                </div>
                <div class="flex-1 min-w-0">
                  <span class="text-xs font-medium text-gray-800 dark:text-gray-200">{{
                    t('askQuestion.other')
                  }}</span>
                  <input
                    v-if="isOtherSelected(currentQuestion.id)"
                    v-model="getOrCreateAnswer(currentQuestion.id).otherText"
                    type="text"
                    class="mt-1.5 w-full px-2.5 py-1 text-xs rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    :placeholder="t('askQuestion.otherPlaceholder')"
                    @click.stop
                  />
                </div>
              </label>
            </div>
          </div>

          <!-- Actions (hidden in quick mode) -->
          <div
            v-if="!isQuickMode"
            class="flex gap-2 px-4 py-3 border-t border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50"
          >
            <button
              class="px-3 py-2 text-xs font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer"
              @click="dismiss"
            >
              {{ dismissLabel }}
            </button>
            <button
              v-if="totalQuestions > 1 && !isLastQuestion"
              class="flex-1 px-3 py-2 text-xs font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors cursor-pointer"
              @click="activeTab++"
            >
              {{ t('askQuestion.next') }}
            </button>
            <button
              v-else
              class="flex-1 px-3 py-2 text-xs font-medium rounded-lg transition-colors cursor-pointer"
              :class="
                canSubmit
                  ? 'bg-blue-600 hover:bg-blue-700 text-white'
                  : 'bg-gray-200 dark:bg-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed'
              "
              :disabled="!canSubmit"
              @click="submit"
            >
              {{ submitLabel }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.askq-fade-enter-active,
.askq-fade-leave-active {
  transition: opacity 0.2s ease;
}
.askq-fade-enter-from,
.askq-fade-leave-to {
  opacity: 0;
}
</style>
