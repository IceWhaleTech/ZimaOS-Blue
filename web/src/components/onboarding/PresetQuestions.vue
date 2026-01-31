<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewApi, type PresetQuestion } from '@/api/preview'
import PresetQuestionCard from './PresetQuestionCard.vue'

const { t, locale } = useI18n()

const emit = defineEmits<{
  select: [text: string]
}>()

const questions = ref<PresetQuestion[]>([])
const loading = ref(false)
const refreshing = ref(false)

// Get language code for API (zh, en, etc.)
function getLangCode(): string {
  const lang = locale.value
  if (lang.startsWith('zh')) return 'zh'
  return 'en'
}

async function fetchQuestions() {
  try {
    loading.value = true
    const response = await previewApi.getPresetQuestions(4, getLangCode())
    questions.value = response.data.questions
  } catch (error) {
    console.error('Failed to fetch preset questions:', error)
  } finally {
    loading.value = false
  }
}

async function refreshQuestions() {
  try {
    refreshing.value = true
    const response = await previewApi.getPresetQuestions(4, getLangCode())
    questions.value = response.data.questions
  } catch (error) {
    console.error('Failed to refresh preset questions:', error)
  } finally {
    refreshing.value = false
  }
}

function handleQuestionClick(question: PresetQuestion) {
  emit('select', question.text)
}

onMounted(() => {
  fetchQuestions()
})
</script>

<template>
  <div class="w-full max-w-2xl mx-auto px-4">
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">
        {{ t('chat.presetQuestions.title') }}
      </h3>
      <button
        class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-accent transition-colors disabled:opacity-50"
        :disabled="refreshing"
        @click="refreshQuestions"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4"
          :class="{ 'animate-spin': refreshing }"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
          />
        </svg>
        <span>{{ t('chat.presetQuestions.refresh') }}</span>
      </button>
    </div>

    <!-- Questions Grid -->
    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div
        v-for="i in 4"
        :key="i"
        class="h-14 rounded-xl bg-gray-100 dark:bg-gray-800 animate-pulse"
      />
    </div>
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3">
      <PresetQuestionCard
        v-for="question in questions"
        :key="question.id"
        :question="question"
        @click="handleQuestionClick"
      />
    </div>
  </div>
</template>
