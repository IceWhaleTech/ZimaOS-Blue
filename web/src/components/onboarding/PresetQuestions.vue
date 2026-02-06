<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewApi, type PresetQuestion, type PresetQuestionAttachment } from '@/api/preview'
import PresetQuestionCard from './PresetQuestionCard.vue'
import type { FileAttachment } from '@/components/ChatInput.vue'

const { t, locale } = useI18n()

const emit = defineEmits<{
  select: [text: string, attachments?: FileAttachment[]]
}>()

const questions = ref<PresetQuestion[]>([])
const loading = ref(false)
const refreshing = ref(false)

// Get language code for API (zh, en, etc.)
function getLangCode(): string {
  const lang = locale.value
  if (lang.startsWith('zh')) return 'zh'
  if (lang.startsWith('ja')) return 'ja'
  if (lang.startsWith('ko')) return 'ko'
  return 'en'
}

// Map of placeholder names to real sample file paths
const sampleFilePaths: Record<string, { path: string; mimeType: string }> = {
  'sample-image': { path: '/samples/landscape.jpg', mimeType: 'image/jpeg' },
  'sample-photo': { path: '/samples/room.jpg', mimeType: 'image/jpeg' },
  'sample-scene': { path: '/samples/cityscape.jpg', mimeType: 'image/jpeg' },
  'sample-chart': { path: '/samples/chart.png', mimeType: 'image/png' },
  'sample-text-image': { path: '/samples/invoice.jpg', mimeType: 'image/jpeg' },
  'sample-code': { path: '/samples/hello.py', mimeType: 'text/x-python' },
  'sample-document': { path: '/samples/report.txt', mimeType: 'text/plain' },
  'sample-csv': { path: '/samples/sales_data.csv', mimeType: 'text/csv' },
  'sample-json': { path: '/samples/config.json', mimeType: 'application/json' },
  'sample-js': { path: '/samples/buggy_calculator.js', mimeType: 'text/javascript' },
}

// Fetch a real sample file from the public folder
async function fetchSampleFile(placeholder: string): Promise<{ blob: Blob; preview?: string } | null> {
  const fileInfo = sampleFilePaths[placeholder]
  if (!fileInfo) return null

  try {
    const response = await fetch(fileInfo.path)
    if (!response.ok) return null

    const blob = await response.blob()

    // Generate preview for images
    let preview: string | undefined
    if (fileInfo.mimeType.startsWith('image/')) {
      preview = URL.createObjectURL(blob)
    }

    return { blob, preview }
  } catch (error) {
    console.error(`Failed to fetch sample file: ${placeholder}`, error)
    return null
  }
}

// Convert preset question attachments to FileAttachment format
async function convertAttachments(presetAttachments?: PresetQuestionAttachment[]): Promise<FileAttachment[]> {
  if (!presetAttachments || presetAttachments.length === 0) {
    console.log('[PresetQuestions] No attachments to convert')
    return []
  }

  console.log('[PresetQuestions] Converting attachments:', presetAttachments)
  const attachments: FileAttachment[] = []

  for (const att of presetAttachments) {
    console.log('[PresetQuestions] Processing attachment:', att.placeholder, att.name)
    if (!att.placeholder) continue
    // Try to fetch real sample file first
    const sampleFile = await fetchSampleFile(att.placeholder)

    if (sampleFile) {
      const file = new File([sampleFile.blob], att.name, { type: att.mime_type })
      console.log('[PresetQuestions] Created file:', att.name, 'size:', file.size)

      attachments.push({
        id: Math.random().toString(36).substring(2, 15),
        file,
        name: att.name,
        size: file.size,
        type: att.mime_type,
        preview: sampleFile.preview,
      })
    } else {
      console.warn('[PresetQuestions] Failed to fetch sample file for placeholder:', att.placeholder)
    }
  }

  console.log('[PresetQuestions] Final attachments count:', attachments.length)
  return attachments
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

async function handleQuestionClick(question: PresetQuestion) {
  const attachments = await convertAttachments(question.attachments)
  emit('select', question.text, attachments.length > 0 ? attachments : undefined)
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
        class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-900 dark:text-gray-300 transition-colors disabled:opacity-50"
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
        class="h-14 rounded-xl bg-gray-100 dark:bg-gray-700 animate-pulse"
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
