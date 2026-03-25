<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewApi, type PresetQuestion, type PresetQuestionAttachment } from '@/api/preview'
import PresetQuestionCard from './PresetQuestionCard.vue'
import type { FileAttachment } from '@/components/ChatInput.vue'
import {
  PRESET_FEED_INTERESTS,
  PRESET_FEED_INTEREST_I18N_KEYS,
  PRESET_FEED_PAGE_SIZE,
  getPresetQuestionTags,
  loadTryFeedState,
  rankPresetQuestions,
  recordPresetQuestionSend,
  saveTryFeedState,
  type PresetFeedInterestId,
} from '@/utils/presetQuestionFeed'
import { localizePresetQuestion } from '@/utils/presetQuestionI18n'

const { t, te, locale } = useI18n()
const props = defineProps<{
  contextText?: string
}>()

const emit = defineEmits<{
  select: [text: string, attachments?: FileAttachment[]]
}>()

const questions = ref<PresetQuestion[]>([])
const loading = ref(false)
const feedState = ref(loadTryFeedState())
const scrollContainerRef = ref<HTMLDivElement | null>(null)

const isChineseLocale = computed(() => locale.value.toLowerCase().startsWith('zh'))
const interestFallbackLabels: Record<PresetFeedInterestId, { en: string; zh: string }> = {
  'personal-knowledge': { en: 'Personal Knowledge', zh: '个人知识' },
  'learning-growth': { en: 'Learning Growth', zh: '学习成长' },
  'content-creation': { en: 'Content Creation', zh: '内容创作' },
  'market-investing': { en: 'Market Investing', zh: '市场投资' },
  'product-design': { en: 'Product Design', zh: '产品设计' },
  'user-research': { en: 'User Research', zh: '用户研究' },
  'psychological-exploration': { en: 'Psychological Exploration', zh: '心理探索' },
  'philosophical-dialogue': { en: 'Philosophical Dialogue', zh: '思想对话' },
}

const interestOptions = computed(() =>
  PRESET_FEED_INTERESTS.map((interest) => ({
    id: interest,
    label: t(
      PRESET_FEED_INTEREST_I18N_KEYS[interest],
      isChineseLocale.value
        ? interestFallbackLabels[interest].zh
        : interestFallbackLabels[interest].en
    ),
  }))
)

const localizedQuestions = computed(() =>
  questions.value.map((question) => localizePresetQuestion(question, te, (key) => t(key)))
)

const orderedQuestions = computed(() =>
  rankPresetQuestions(localizedQuestions.value, {
    contextText: props.contextText || '',
    state: {
      ...feedState.value,
      selectedTags: [],
    },
  })
)

const filteredQuestions = computed(() => {
  if (feedState.value.selectedTags.length === 0) {
    return orderedQuestions.value
  }

  const selectedTags = new Set(feedState.value.selectedTags)
  return orderedQuestions.value.filter((question) =>
    getPresetQuestionTags(question).some((tag) => selectedTags.has(tag))
  )
})

const visibleQuestions = computed(() => filteredQuestions.value.slice(0, PRESET_FEED_PAGE_SIZE))

function getLangCode(): string {
  return isChineseLocale.value ? 'zh' : 'en'
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
async function fetchSampleFile(
  placeholder: string
): Promise<{ blob: Blob; preview?: string } | null> {
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
async function convertAttachments(
  presetAttachments?: PresetQuestionAttachment[]
): Promise<FileAttachment[]> {
  if (!presetAttachments || presetAttachments.length === 0) {
    return []
  }

  const attachments: FileAttachment[] = []

  for (const att of presetAttachments) {
    if (!att.placeholder) continue
    // Try to fetch real sample file first
    const sampleFile = await fetchSampleFile(att.placeholder)

    if (sampleFile) {
      const file = new File([sampleFile.blob], att.name, { type: att.mime_type })

      attachments.push({
        id: Math.random().toString(36).substring(2, 15),
        file,
        name: att.name,
        size: file.size,
        type: att.mime_type,
        preview: sampleFile.preview,
      })
    } else {
      console.warn(
        '[PresetQuestions] Failed to fetch sample file for placeholder:',
        att.placeholder
      )
    }
  }

  return attachments
}

async function fetchQuestions() {
  try {
    loading.value = true
    const response = await previewApi.getPresetQuestions(PRESET_FEED_PAGE_SIZE, getLangCode())
    questions.value = response.data.questions
  } catch (error) {
    console.error('Failed to fetch preset questions:', error)
  } finally {
    loading.value = false
  }
}

function persistFeedState() {
  saveTryFeedState(feedState.value)
}

function resetScrollPosition() {
  nextTick(() => {
    if (scrollContainerRef.value) {
      scrollContainerRef.value.scrollTop = 0
    }
  })
}

function toggleInterest(interest: PresetFeedInterestId) {
  const nextSelected = feedState.value.selectedTags.includes(interest)
    ? feedState.value.selectedTags.filter((tag) => tag !== interest)
    : [...feedState.value.selectedTags, interest]

  feedState.value = {
    ...feedState.value,
    selectedTags: nextSelected,
  }
  persistFeedState()
  resetScrollPosition()
}

function tagLabel(tag: PresetFeedInterestId): string {
  return t(
    PRESET_FEED_INTEREST_I18N_KEYS[tag],
    isChineseLocale.value ? interestFallbackLabels[tag].zh : interestFallbackLabels[tag].en
  )
}

function resolveQuestionTagLabels(question: PresetQuestion): string[] {
  return getPresetQuestionTags(question).map((tag) => tagLabel(tag))
}

async function handleQuestionClick(question: PresetQuestion) {
  const attachments = await convertAttachments(question.attachments)
  feedState.value = recordPresetQuestionSend(feedState.value, question)
  persistFeedState()
  emit('select', question.prompt || question.text, attachments.length > 0 ? attachments : undefined)
}

onMounted(() => {
  void fetchQuestions()
})

watch(
  () => getLangCode(),
  () => {
    resetScrollPosition()
    void fetchQuestions()
  }
)
</script>

<template>
  <div class="mx-auto w-full max-w-[42rem] px-4">
    <div class="flex items-center justify-between gap-3">
      <div class="min-w-0 flex flex-1 items-center gap-3">
        <h3 class="flex-shrink-0 text-[13px] font-medium text-gray-500 dark:text-gray-400">
          {{ t('chat.presetQuestions.title') }}
        </h3>
        <div class="preset-questions-interest-row flex flex-1 gap-1.5 overflow-x-auto pe-1">
          <button
            v-for="interest in interestOptions"
            :key="interest.id"
            type="button"
            class="flex-shrink-0 rounded-full border px-2 py-0.5 text-[9px] font-medium transition-colors"
            :class="
              feedState.selectedTags.includes(interest.id)
                ? 'border-slate-900 bg-slate-900 text-white shadow-sm dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900'
                : 'border-slate-200/90 bg-white/80 text-slate-500 hover:border-slate-300 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-400 dark:hover:border-slate-500 dark:hover:text-white'
            "
            :data-testid="`preset-interest-chip-${interest.id}`"
            @click="toggleInterest(interest.id)"
          >
            {{ interest.label }}
          </button>
        </div>
      </div>
    </div>

    <div
      ref="scrollContainerRef"
      class="preset-questions-scroll mt-2 overflow-y-auto pe-1"
      data-testid="preset-questions-scroll"
    >
      <div v-if="loading" class="preset-questions-list flex flex-col">
        <div
          v-for="i in 4"
          :key="i"
          class="h-[var(--preset-question-row-height)] animate-pulse rounded-[1.25rem] bg-gray-100 dark:bg-gray-800"
        />
      </div>
      <div v-else class="preset-questions-list flex flex-col">
        <PresetQuestionCard
          v-for="question in visibleQuestions"
          :key="question.id"
          :question="question"
          :tag-labels="resolveQuestionTagLabels(question)"
          @click="handleQuestionClick"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.preset-questions-interest-row {
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.preset-questions-interest-row::-webkit-scrollbar {
  display: none;
}

.preset-questions-scroll {
  --preset-question-row-height: 4.5rem;
  --preset-question-row-gap: 0.5rem;
  min-height: calc(var(--preset-question-row-height) * 3 + var(--preset-question-row-gap) * 2);
  max-height: calc(var(--preset-question-row-height) * 3 + var(--preset-question-row-gap) * 2);
  scrollbar-gutter: stable;
}

.preset-questions-list {
  gap: var(--preset-question-row-gap);
}
</style>
