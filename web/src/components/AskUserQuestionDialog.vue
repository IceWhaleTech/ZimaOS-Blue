<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChatStore } from '@/stores/chat'

const { t } = useI18n()
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
const isFirstQuestion = computed(() => activeTab.value === 0)
const isLastQuestion = computed(() => activeTab.value >= totalQuestions.value - 1)

// Check if we can use quick-submit mode: single question, single-select, short options
const isQuickMode = computed(() => {
  if (!question.value) return false
  const q = question.value
  if (q.questions.length !== 1) return false
  const first = q.questions[0]
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

function toggleOther(qId: string, multiSelect: boolean) {
  toggleOption(qId, '__other__', multiSelect)
}

// Check if current question is answered
const currentQuestionAnswered = computed(() => {
  if (!currentQuestion.value) return false
  const ans = answers.value[currentQuestion.value.id]
  if (!ans) return false
  return ans.selected.length > 0 || ans.otherText.trim() !== ''
})

// Can proceed to next question
const canProceed = computed(() => {
  return currentQuestionAnswered.value
})

const canSubmit = computed(() => {
  if (!question.value) return false
  // For single question, check current
  if (question.value.questions.length === 1) {
    return currentQuestionAnswered.value
  }
  // For multiple questions, all must be answered to submit
  return question.value.questions.every((q) => {
    const ans = answers.value[q.id]
    if (!ans) return false
    return ans.selected.length > 0 || ans.otherText.trim() !== ''
  })
})

function isTabAnswered(qId: string): boolean {
  const ans = answers.value[qId]
  if (!ans) return false
  return ans.selected.length > 0 || ans.otherText.trim() !== ''
}

// Countdown timer
const remainingSeconds = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

watch(question, (q) => {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  if (q) {
    const updateRemaining = () => {
      const ms = q.expires_at - Date.now()
      remainingSeconds.value = Math.max(0, Math.ceil(ms / 1000))
      if (remainingSeconds.value <= 0 && countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = null
        chatStore.dismissQuestion()
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
    const selected = ans.selected.filter((v) => v !== '__other__')
    return {
      question_id: q.id,
      selected,
      other_text: ans.selected.includes('__other__') ? ans.otherText : '',
    }
  })
  chatStore.submitQuestionAnswers(result)
}

function dismiss() {
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
        <div class="w-full max-w-lg mx-4 rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-2xl overflow-hidden">
          <!-- Header -->
          <div class="flex items-center gap-3 px-5 py-4 border-b border-gray-100 dark:border-gray-700 bg-blue-50 dark:bg-blue-900/20">
            <div class="flex-shrink-0 w-10 h-10 rounded-full bg-blue-100 dark:bg-blue-800/40 flex items-center justify-center">
              <svg class="w-5 h-5 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ t('askQuestion.title') }}
              </h3>
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('askQuestion.subtitle') }}
              </p>
            </div>
            <span v-if="remainingSeconds > 0" class="text-xs text-gray-400 dark:text-gray-500 tabular-nums">
              {{ t('askQuestion.timeout', { seconds: remainingSeconds }) }}
            </span>
          </div>

          <!-- Step indicator (if multiple questions) -->
          <div v-if="totalQuestions > 1" class="px-5 py-3 border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
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
                :class="idx === activeTab
                  ? 'bg-blue-500'
                  : isTabAnswered(q.id)
                    ? 'bg-green-500'
                    : 'bg-gray-300 dark:bg-gray-600'"
                @click="activeTab = idx"
              />
            </div>
          </div>

          <!-- Question body - show current question only -->
          <div v-if="currentQuestion" class="px-5 py-4 space-y-3 max-h-80 overflow-y-auto">
            <p class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ currentQuestion.question }}</p>

            <!-- Options -->
            <div class="space-y-2">
              <label
                v-for="opt in currentQuestion.options"
                :key="opt.label"
                class="flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors"
                :class="isSelected(currentQuestion.id, opt.value || opt.label)
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 dark:border-blue-500'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'"
                @click="toggleOption(currentQuestion.id, opt.value || opt.label, currentQuestion.multi_select)"
              >
                <!-- Radio / Checkbox indicator -->
                <div class="flex-shrink-0 mt-0.5">
                  <div v-if="!currentQuestion.multi_select" class="w-4 h-4 rounded-full border-2 flex items-center justify-center"
                    :class="isSelected(currentQuestion.id, opt.value || opt.label) ? 'border-blue-500' : 'border-gray-300 dark:border-gray-600'">
                    <div v-if="isSelected(currentQuestion.id, opt.value || opt.label)" class="w-2 h-2 rounded-full bg-blue-500" />
                  </div>
                  <div v-else class="w-4 h-4 rounded border-2 flex items-center justify-center"
                    :class="isSelected(currentQuestion.id, opt.value || opt.label) ? 'border-blue-500 bg-blue-500' : 'border-gray-300 dark:border-gray-600'">
                    <svg v-if="isSelected(currentQuestion.id, opt.value || opt.label)" class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                  </div>
                </div>
                <div class="flex-1 min-w-0">
                  <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ opt.label }}</span>
                  <p v-if="opt.description" class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ opt.description }}</p>
                </div>
              </label>

              <!-- Other option -->
              <label
                class="flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors"
                :class="isOtherSelected(currentQuestion.id)
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 dark:border-blue-500'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'"
                @click="toggleOther(currentQuestion.id, currentQuestion.multi_select)"
              >
                <div class="flex-shrink-0 mt-0.5">
                  <div v-if="!currentQuestion.multi_select" class="w-4 h-4 rounded-full border-2 flex items-center justify-center"
                    :class="isOtherSelected(currentQuestion.id) ? 'border-blue-500' : 'border-gray-300 dark:border-gray-600'">
                    <div v-if="isOtherSelected(currentQuestion.id)" class="w-2 h-2 rounded-full bg-blue-500" />
                  </div>
                  <div v-else class="w-4 h-4 rounded border-2 flex items-center justify-center"
                    :class="isOtherSelected(currentQuestion.id) ? 'border-blue-500 bg-blue-500' : 'border-gray-300 dark:border-gray-600'">
                    <svg v-if="isOtherSelected(currentQuestion.id)" class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                      <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                    </svg>
                  </div>
                </div>
                <div class="flex-1 min-w-0">
                  <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('askQuestion.other') }}</span>
                  <input
                    v-if="isOtherSelected(currentQuestion.id)"
                    v-model="answers[currentQuestion.id].otherText"
                    type="text"
                    class="mt-2 w-full px-3 py-1.5 text-sm rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 text-gray-800 dark:text-gray-200 focus:outline-none focus:ring-1 focus:ring-blue-500"
                    :placeholder="t('askQuestion.otherPlaceholder')"
                    @click.stop
                  />
                </div>
              </label>
            </div>
          </div>

          <!-- Actions (hidden in quick mode) -->
          <div v-if="!isQuickMode" class="flex gap-2 px-5 py-4 border-t border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
            <button
              class="px-4 py-2.5 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors cursor-pointer"
              @click="dismiss"
            >
              {{ t('askQuestion.skip') }}
            </button>
            <button
              v-if="totalQuestions > 1 && !isLastQuestion"
              class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg bg-blue-600 hover:bg-blue-700 text-white transition-colors cursor-pointer"
              @click="activeTab++"
            >
              {{ t('askQuestion.next') }}
            </button>
            <button
              v-else
              class="flex-1 px-4 py-2.5 text-sm font-medium rounded-lg transition-colors cursor-pointer"
              :class="canSubmit
                ? 'bg-blue-600 hover:bg-blue-700 text-white'
                : 'bg-gray-200 dark:bg-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed'"
              :disabled="!canSubmit"
              @click="submit"
            >
              {{ t('askQuestion.submit') }}
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
