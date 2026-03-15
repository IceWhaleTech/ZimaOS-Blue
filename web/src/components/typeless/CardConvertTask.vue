<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardConvertTask, ConvertTaskOutput } from '@/types/typeless'
import { useTauri } from '@/composables/useTauri'

const { t } = useI18n()
const { revealInFileManager } = useTauri()

const props = defineProps<{
  card: TypelessCardConvertTask
}>()

const actionKeyByValue: Record<string, string> = {
  convert: 'convert',
  merge: 'merge',
  split: 'split',
  trim: 'trim',
  extract_audio: 'extractAudio',
  extract_frames: 'extractFrames',
  tts: 'tts',
  asr: 'asr',
}

const previewKindKeyByValue: Record<string, string> = {
  file: 'file',
  audio: 'audio',
  video: 'video',
  image: 'image',
  pdf: 'pdf',
  text: 'text',
}

const stableMessageKeyByValue: Record<string, string> = {
  Queued: 'queued',
  Processing: 'processing',
  Completed: 'completed',
  'Task cancelled': 'taskCancelled',
  'Task failed': 'taskFailed',
}

const stableSuccessMessages = new Set([
  'Completed',
  'Image PDF created',
  'Video converted',
  'PDF pages exported',
  'PDF merged',
  'Video merged',
  'PDF split',
  'Video split',
  'Video trimmed',
  'Audio extracted',
  'Frames extracted',
  'Speech audio created',
  'Transcription completed',
  'PDF created',
  'Document converted',
  'Image converted',
  'Audio converted',
])

const successMessageKeyByAction: Record<string, string> = {
  convert: 'convertCompleted',
  merge: 'mergeCompleted',
  split: 'splitCompleted',
  trim: 'trimCompleted',
  extract_audio: 'extractAudioCompleted',
  extract_frames: 'extractFramesCompleted',
  tts: 'ttsCompleted',
  asr: 'asrCompleted',
}

const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
const task = ref<TypelessCardConvertTask | null>(null)
const cancelling = ref(false)

const displayCard = computed(() => task.value || props.card)
const outputs = computed(() => displayCard.value.outputs || [])
const isRunning = computed(() => ['pending', 'processing'].includes(displayCard.value.status))
const isFailed = computed(() => displayCard.value.status === 'failed')
const isCancelled = computed(() => displayCard.value.status === 'cancelled')
const isSucceeded = computed(() => displayCard.value.status === 'succeeded')
const progressPercent = computed(() => {
  const raw = Number(displayCard.value.progress || 0)
  if (Number.isNaN(raw) || raw <= 0) return isSucceeded.value ? 100 : 0
  return raw > 1 ? Math.round(raw) : Math.round(raw * 100)
})
const primaryAudio = computed(() => outputs.value.find((output) => output.preview_kind === 'audio'))
const primaryVideo = computed(() => outputs.value.find((output) => output.preview_kind === 'video'))
const localizedAction = computed(() => actionLabel(displayCard.value.action))
const localizedMessage = computed(() => taskMessage(displayCard.value))

interface ConvertOutputLocationResponse {
  path?: string
  parent_path?: string
}

async function pollTask() {
  if (!props.card.task_id) return
  try {
    const resp = await fetch(`/api/v1/convert/tasks/${props.card.task_id}`)
    if (!resp.ok) return
    const nextTask = (await resp.json()) as TypelessCardConvertTask
    task.value = {
      ...nextTask,
      type: 'convert-task',
    }
    if (!['pending', 'processing'].includes(nextTask.status)) {
      stopPolling()
    }
  } catch {
    // ignore transient errors during polling
  }
}

async function cancelTask() {
  if (!props.card.task_id || cancelling.value) return
  cancelling.value = true
  try {
    const resp = await fetch(`/api/v1/convert/tasks/${props.card.task_id}/cancel`, {
      method: 'POST',
    })
    if (!resp.ok) return
    const nextTask = (await resp.json()) as TypelessCardConvertTask
    task.value = {
      ...nextTask,
      type: 'convert-task',
    }
    stopPolling()
  } finally {
    cancelling.value = false
  }
}

function stopPolling() {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
}

function actionLabel(action?: string): string {
  const normalized = String(action || '').trim()
  const key = actionKeyByValue[normalized || 'convert']
  if (key) {
    return t(`speech.convertTask.action.${key}`, normalized || 'convert')
  }
  if (normalized === '') {
    return t('speech.convertTask.action.convert', 'Convert')
  }
  return normalized.replace(/_/g, ' ')
}

function taskMessage(card: TypelessCardConvertTask): string {
  const raw = String(card.message || '').trim()
  const action = String(card.action || '').trim()

  const directKey = stableMessageKeyByValue[raw]
  if (directKey) {
    return t(`speech.convertTask.message.${directKey}`, raw)
  }

  if (raw === '' && card.status === 'cancelled') {
    return t('speech.convertTask.message.taskCancelled', 'Task cancelled')
  }
  if (raw === '' && card.status === 'failed') {
    return t('speech.convertTask.message.taskFailed', 'Task failed')
  }
  if (stableSuccessMessages.has(raw) || (raw === '' && card.status === 'succeeded')) {
    const actionKey = successMessageKeyByAction[action]
    if (actionKey) {
      return t(`speech.convertTask.message.${actionKey}`, raw || 'Completed')
    }
    return t('speech.convertTask.message.completed', raw || 'Completed')
  }

  return raw
}

function previewKindLabel(output: ConvertTaskOutput): string {
  const kind = String(output.preview_kind || '')
    .trim()
    .toLowerCase()
  const key = previewKindKeyByValue[kind]
  if (key) {
    return t(`speech.convertTask.previewKind.${key}`, kind)
  }
  if (output.mime_type) {
    return output.mime_type
  }
  return t('speech.convertTask.previewKind.file', 'File')
}

function downloadLabel(output: ConvertTaskOutput): string {
  if (output.preview_kind === 'audio')
    return t('speech.convertTask.downloadAudio', 'Download audio')
  if (output.preview_kind === 'video')
    return t('speech.convertTask.downloadVideo', 'Download video')
  if (output.preview_kind === 'pdf') return t('speech.convertTask.downloadPdf', 'Download PDF')
  if (output.preview_kind === 'text') return t('speech.convertTask.downloadText', 'Download text')
  return t('speech.convertTask.downloadFile', 'Download file')
}

function humanSize(size?: number): string {
  if (!size || size <= 0) return ''
  const units = ['B', 'KB', 'MB', 'GB']
  let value = size
  let idx = 0
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(value >= 10 || idx === 0 ? 0 : 1)} ${units[idx]}`
}

async function revealOutputLocation(output: ConvertTaskOutput) {
  const taskID = String(displayCard.value.task_id || '').trim()
  const outputID = String(output.output_id || '').trim()
  if (!taskID || !outputID) return

  try {
    const resp = await fetch(`/api/v1/convert/tasks/${taskID}/outputs/${outputID}/location`)
    if (!resp.ok) return
    const data = (await resp.json()) as ConvertOutputLocationResponse
    const path = String(data.path || '').trim()
    if (!path) return
    const revealed = await revealInFileManager(path)
    if (!revealed && output.download_url) {
      window.open(output.download_url, '_blank')
    }
  } catch {
    if (output.download_url) {
      window.open(output.download_url, '_blank')
    }
  }
}

onMounted(() => {
  if (isRunning.value && props.card.task_id) {
    pollTimer.value = setInterval(pollTask, 2000)
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div
    class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm"
  >
    <div
      class="flex items-center gap-2 px-3 py-2.5 border-b border-gray-100 dark:border-gray-700/50"
    >
      <div
        class="w-2 h-2 rounded-full"
        :class="
          isSucceeded
            ? 'bg-emerald-500'
            : isFailed
              ? 'bg-red-500'
              : isCancelled
                ? 'bg-gray-400'
                : 'bg-indigo-500 animate-pulse'
        "
      />
      <div class="min-w-0 flex-1">
        <div class="text-sm font-medium text-gray-800 dark:text-gray-100 truncate">
          {{ localizedAction }}
          <span
            v-if="displayCard.source_summary"
            class="font-normal text-gray-500 dark:text-gray-400"
            >· {{ displayCard.source_summary }}</span
          >
        </div>
        <div class="text-xs text-gray-500 dark:text-gray-400 truncate">
          {{ t('speech.convertTask.task', 'Task') }} {{ displayCard.task_id }}
          <span v-if="displayCard.target_format">· {{ displayCard.target_format }}</span>
        </div>
      </div>
      <button
        v-if="isRunning"
        class="text-xs px-2 py-1 rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700"
        :disabled="cancelling"
        @click="cancelTask"
      >
        {{
          cancelling
            ? t('speech.convertTask.cancelling', 'Cancelling...')
            : t('common.cancel', 'Cancel')
        }}
      </button>
    </div>

    <div v-if="isRunning" class="px-3 pt-3">
      <div class="h-1.5 rounded bg-gray-100 dark:bg-gray-700 overflow-hidden">
        <div
          class="h-full bg-indigo-500 transition-all duration-300"
          :style="{ width: `${Math.max(progressPercent, 10)}%` }"
        />
      </div>
      <div class="mt-2 text-xs text-gray-500 dark:text-gray-400">
        {{ localizedMessage || t('common.processing', 'Processing...') }}
      </div>
    </div>

    <div class="px-3 py-3 space-y-3">
      <div
        v-if="localizedMessage && !isRunning"
        class="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-words"
      >
        {{ localizedMessage }}
      </div>
      <div
        v-if="displayCard.error"
        class="text-sm text-red-600 dark:text-red-400 whitespace-pre-wrap break-words"
      >
        {{ displayCard.error }}
      </div>
      <div
        v-if="displayCard.transcript_preview"
        class="rounded-md bg-gray-50 dark:bg-gray-900/50 px-3 py-2 text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-words"
      >
        {{ displayCard.transcript_preview }}
      </div>

      <audio
        v-if="primaryAudio?.download_url"
        class="w-full"
        controls
        preload="none"
        :src="primaryAudio.download_url"
      />
      <video
        v-if="primaryVideo?.download_url"
        class="w-full rounded bg-black"
        controls
        preload="metadata"
        :src="primaryVideo.download_url"
      />

      <div v-if="outputs.length > 0" class="space-y-2">
        <div
          v-for="output in outputs"
          :key="output.output_id"
          class="flex items-center justify-between gap-3 rounded-md border border-gray-100 dark:border-gray-700 px-3 py-2"
        >
          <div class="min-w-0 flex-1">
            <div class="text-sm text-gray-800 dark:text-gray-100 truncate">{{ output.name }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400 truncate">
              {{ previewKindLabel(output) }}
              <span v-if="output.size_bytes">· {{ humanSize(output.size_bytes) }}</span>
              <span v-if="output.ref">· {{ output.ref }}</span>
            </div>
            <div
              v-if="output.preview_text && output.preview_kind === 'text'"
              class="mt-1 text-xs text-gray-600 dark:text-gray-300 whitespace-pre-wrap break-words"
            >
              {{ output.preview_text }}
            </div>
          </div>
          <div class="flex items-center gap-2">
            <button
              v-if="output.output_id"
              class="text-xs px-2.5 py-1.5 rounded border border-gray-200 text-gray-600 hover:bg-gray-100 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700 whitespace-nowrap"
              @click="revealOutputLocation(output)"
            >
              {{ t('common.openLocation', 'Open location') }}
            </button>
            <a
              v-if="output.download_url"
              class="text-xs px-2.5 py-1.5 rounded bg-indigo-50 text-indigo-600 hover:bg-indigo-100 dark:bg-indigo-900/40 dark:text-indigo-300 dark:hover:bg-indigo-900/60 whitespace-nowrap"
              :href="output.download_url"
              target="_blank"
              rel="noreferrer"
            >
              {{ downloadLabel(output) }}
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
