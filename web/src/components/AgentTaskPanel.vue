<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AgentQuestion, AgentQuestionAnswer } from '@/api/chat'

const { t } = useI18n()

interface PlanStep {
  index: number
  description: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  output?: string
  started_at?: string
  completed_at?: string
}

interface AgentTask {
  id: string
  goal: string
  plan: PlanStep[]
  status:
    | 'pending'
    | 'planning'
    | 'executing'
    | 'waiting_input'
    | 'completed'
    | 'failed'
    | 'cancelled'
  current_step: number
  progress: number
  result?: string
  error?: string
  questions?: AgentQuestion[]
}

const props = defineProps<{
  task: AgentTask
}>()

const emit = defineEmits<{
  cancel: [taskId: string]
  delete: [taskId: string]
  message: [taskId: string, message: string]
  answer: [taskId: string, answers: AgentQuestionAnswer[]]
}>()

const expandedSteps = ref<Set<number>>(new Set())
const injectionMessage = ref('')

// Q&A state
const activeTab = ref(0)
const selections = ref<Record<string, Set<string>>>({})
const otherTexts = ref<Record<string, string>>({})

function sendInjection() {
  if (!injectionMessage.value.trim()) return
  emit('message', props.task.id, injectionMessage.value.trim())
  injectionMessage.value = ''
}

// Q&A helpers
const hasQuestions = computed(() => (props.task.questions?.length ?? 0) > 0)
const questions = computed(() => props.task.questions ?? [])

function toggleOption(questionId: string, value: string, multiSelect: boolean) {
  if (!selections.value[questionId]) {
    selections.value[questionId] = new Set()
  }
  const set = selections.value[questionId]!
  if (multiSelect) {
    if (set.has(value)) set.delete(value)
    else set.add(value)
  } else {
    set.clear()
    set.add(value)
  }
  selections.value = { ...selections.value }
}

function isOptionSelected(questionId: string, value: string): boolean {
  return selections.value[questionId]?.has(value) ?? false
}

function canSubmitAnswers(): boolean {
  for (const q of questions.value) {
    if (!q.required) continue
    const sel = selections.value[q.id]
    const other = otherTexts.value[q.id]
    if ((!sel || sel.size === 0) && !other?.trim()) return false
  }
  return true
}

function submitAnswers() {
  const answers: AgentQuestionAnswer[] = questions.value.map((q) => ({
    question_id: q.id,
    values: Array.from(selections.value[q.id] ?? []),
    other_text: otherTexts.value[q.id] || undefined,
  }))
  emit('answer', props.task.id, answers)
  // Reset Q&A state
  selections.value = {}
  otherTexts.value = {}
  activeTab.value = 0
}

function handleQuestionKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey && canSubmitAnswers()) {
    e.preventDefault()
    submitAnswers()
  }
  // Left/Right arrow to switch tabs
  if (questions.value.length > 1) {
    if (e.key === 'ArrowRight' || (e.key === 'Tab' && !e.shiftKey)) {
      if (activeTab.value < questions.value.length - 1) {
        e.preventDefault()
        activeTab.value++
      }
    }
    if (e.key === 'ArrowLeft' || (e.key === 'Tab' && e.shiftKey)) {
      if (activeTab.value > 0) {
        e.preventDefault()
        activeTab.value--
      }
    }
  }
}

const statusIcon = computed(() => {
  if (hasQuestions.value) return '?'
  switch (props.task.status) {
    case 'planning':
      return '🧠'
    case 'executing':
      return '⚡'
    case 'completed':
      return '✓'
    case 'failed':
      return '✗'
    case 'cancelled':
      return '⏹'
    default:
      return '⏳'
  }
})

const statusColor = computed(() => {
  if (hasQuestions.value) return 'text-amber-500'
  switch (props.task.status) {
    case 'completed':
      return 'text-green-500'
    case 'failed':
      return 'text-red-500'
    case 'cancelled':
      return 'text-gray-400'
    default:
      return 'text-blue-500'
  }
})

const isRunning = computed(() =>
  ['pending', 'planning', 'executing', 'waiting_input'].includes(props.task.status)
)

const isDone = computed(() => ['completed', 'failed', 'cancelled'].includes(props.task.status))

function toggleStep(index: number) {
  if (expandedSteps.value.has(index)) {
    expandedSteps.value.delete(index)
  } else {
    expandedSteps.value.add(index)
  }
}

function stepIcon(status: string) {
  switch (status) {
    case 'completed':
      return '✓'
    case 'running':
      return '⏳'
    case 'failed':
      return '✗'
    case 'skipped':
      return '⏭'
    default:
      return '○'
  }
}

function stepColor(status: string) {
  switch (status) {
    case 'completed':
      return 'text-green-500'
    case 'running':
      return 'text-blue-500'
    case 'failed':
      return 'text-red-500'
    case 'skipped':
      return 'text-gray-300 dark:text-gray-600'
    default:
      return 'text-gray-400'
  }
}

function stepDuration(step: PlanStep): string {
  if (!step.started_at || !step.completed_at) return ''
  const ms = new Date(step.completed_at).getTime() - new Date(step.started_at).getTime()
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}
</script>

<template>
  <div
    class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-4 my-2"
  >
    <!-- Header -->
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2 min-w-0">
        <span :class="statusColor">{{ statusIcon }}</span>
        <span class="font-medium text-gray-900 dark:text-white text-sm truncate">{{
          task.goal
        }}</span>
      </div>
      <div class="flex items-center gap-1 shrink-0">
        <button
          v-if="isRunning"
          class="text-xs px-2 py-1 rounded bg-red-100 dark:bg-red-900 text-red-600 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-800"
          @click="emit('cancel', task.id)"
        >
          {{ t('agent.cancel') }}
        </button>
        <button
          v-if="isDone"
          class="text-xs px-2 py-1 rounded bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-600"
          @click="emit('delete', task.id)"
        >
          ✕
        </button>
      </div>
    </div>

    <!-- Progress bar -->
    <div class="h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden mb-3">
      <div
        class="h-full transition-all duration-300 rounded-full"
        :class="{
          'bg-amber-500': hasQuestions,
          'bg-blue-500': isRunning && !hasQuestions,
          'bg-green-500': task.status === 'completed',
          'bg-red-500': task.status === 'failed',
          'bg-gray-400': task.status === 'cancelled',
        }"
        :style="{ width: `${task.progress}%` }"
      />
    </div>

    <!-- Steps -->
    <div v-if="task.plan?.length" class="space-y-1">
      <div v-for="step in task.plan" :key="step.index" class="text-sm">
        <div
          class="flex items-center gap-2 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 rounded px-1 py-0.5"
          @click="toggleStep(step.index)"
        >
          <span :class="stepColor(step.status)" class="text-xs w-4 text-center">{{
            stepIcon(step.status)
          }}</span>
          <span
            class="text-gray-700 dark:text-gray-300 flex-1"
            :class="{ 'line-through opacity-50': step.status === 'skipped' }"
            >{{ step.description }}</span
          >
          <span v-if="stepDuration(step)" class="text-xs text-gray-400 shrink-0">{{
            stepDuration(step)
          }}</span>
        </div>
        <div
          v-if="expandedSteps.has(step.index) && step.output"
          class="ml-6 mt-1 p-2 bg-gray-50 dark:bg-gray-900 rounded text-xs text-gray-600 dark:text-gray-400 whitespace-pre-wrap max-h-32 overflow-y-auto"
        >
          {{ step.output }}
        </div>
      </div>
    </div>

    <!-- Q&A Panel -->
    <div
      v-if="hasQuestions"
      class="mt-3 rounded-lg border-2 border-amber-300 dark:border-amber-600 bg-amber-50 dark:bg-amber-900/20 p-3"
      @keydown="handleQuestionKeydown"
    >
      <div class="text-sm font-medium text-amber-700 dark:text-amber-300 mb-2">
        {{ t('agent.questionTitle') }}
      </div>

      <!-- Tab bar (multiple questions) -->
      <div v-if="questions.length > 1" class="flex gap-1 mb-3 overflow-x-auto">
        <button
          v-for="(q, idx) in questions"
          :key="q.id"
          class="px-3 py-1 text-xs rounded-full whitespace-nowrap transition-colors"
          :class="{
            'bg-amber-500 text-white': activeTab === idx,
            'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-400 hover:bg-gray-300 dark:hover:bg-gray-600':
              activeTab !== idx,
          }"
          @click="activeTab = idx"
        >
          {{ q.header || `Q${idx + 1}` }}
        </button>
      </div>

      <!-- Active question -->
      <div v-for="(q, idx) in questions" :key="q.id" v-show="activeTab === idx">
        <p class="text-sm text-gray-800 dark:text-gray-200 mb-2">
          {{ q.question }}
          <span v-if="q.required" class="text-red-500 ml-0.5">*</span>
        </p>
        <p
          v-if="q.detail"
          class="text-xs text-amber-700 dark:text-amber-300 bg-amber-100/70 dark:bg-amber-900/30 border border-amber-300/70 dark:border-amber-700 rounded-md px-2 py-1 mb-2"
        >
          ❕ {{ q.detail }}
        </p>

        <!-- Options -->
        <div v-if="q.options?.length" class="space-y-1.5">
          <button
            v-for="opt in q.options"
            :key="opt.value"
            class="w-full p-2 rounded-lg border text-left text-sm transition-all flex items-start gap-2"
            :class="{
              'border-amber-500 bg-amber-100 dark:bg-amber-800/30': isOptionSelected(
                q.id,
                opt.value
              ),
              'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500':
                !isOptionSelected(q.id, opt.value),
            }"
            @click="toggleOption(q.id, opt.value, !!q.multi_select)"
          >
            <div
              class="flex-shrink-0 w-4 h-4 mt-0.5 rounded flex items-center justify-center border-2 transition-colors"
              :class="{
                'rounded-full': !q.multi_select,
                'border-amber-500 bg-amber-500': isOptionSelected(q.id, opt.value),
                'border-gray-300 dark:border-gray-500': !isOptionSelected(q.id, opt.value),
              }"
            >
              <svg
                v-if="isOptionSelected(q.id, opt.value)"
                xmlns="http://www.w3.org/2000/svg"
                class="h-2.5 w-2.5 text-white"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="3"
                  d="M5 13l4 4L19 7"
                />
              </svg>
            </div>
            <div class="flex-1 min-w-0">
              <span class="text-gray-900 dark:text-white">{{ opt.label }}</span>
              <p v-if="opt.description" class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                {{ opt.description }}
              </p>
            </div>
          </button>
        </div>

        <!-- Free text / Other -->
        <div class="mt-2">
          <input
            v-model="otherTexts[q.id]"
            type="text"
            :placeholder="t('agent.otherPlaceholder')"
            class="w-full text-sm px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-amber-500"
          />
        </div>
      </div>

      <!-- Submit -->
      <div class="mt-3 flex justify-end">
        <button
          :disabled="!canSubmitAnswers()"
          class="px-4 py-1.5 rounded-lg bg-amber-500 text-white text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed hover:bg-amber-600 transition-colors"
          @click="submitAnswers"
        >
          {{ t('agent.submitAnswers') }}
        </button>
      </div>
    </div>

    <!-- Result / Error -->
    <div
      v-if="task.result"
      class="mt-2 text-sm text-green-600 dark:text-green-400 whitespace-pre-wrap"
    >
      {{ task.result }}
    </div>
    <div v-if="task.error" class="mt-2 text-sm text-red-600 dark:text-red-400">
      {{ task.error }}
    </div>

    <!-- Message injection (when running, no pending questions) -->
    <div v-if="isRunning && !hasQuestions" class="mt-3 flex gap-2">
      <input
        v-model="injectionMessage"
        type="text"
        :placeholder="t('agent.sendMessage')"
        class="flex-1 text-sm px-3 py-1.5 rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-blue-500"
        @keydown.enter="sendInjection"
      />
      <button
        :disabled="!injectionMessage.trim()"
        class="px-3 py-1.5 rounded-lg bg-blue-500 text-white text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-blue-600 transition-colors"
        @click="sendInjection"
      >
        {{ t('common.send') }}
      </button>
    </div>
  </div>
</template>
