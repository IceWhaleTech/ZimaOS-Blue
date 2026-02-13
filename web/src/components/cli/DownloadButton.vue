<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { cliDownloadApi, type CLIDownloadInfo, type CLIDownloadProgress } from '@/api/setup'
import DownloadProgress from './DownloadProgress.vue'

const { t } = useI18n()

const props = defineProps<{
  showInfo?: boolean
}>()

const emit = defineEmits<{
  (e: 'download-start'): void
  (e: 'download-complete'): void
  (e: 'download-error', error: string): void
}>()

type ButtonState = 'idle' | 'loading-info' | 'ready' | 'downloading' | 'complete' | 'error'

const state = ref<ButtonState>('idle')
const downloadInfo = ref<CLIDownloadInfo | null>(null)
const progress = ref<CLIDownloadProgress | null>(null)
const error = ref<string | null>(null)
let progressInterval: ReturnType<typeof setInterval> | null = null

const buttonText = computed(() => {
  switch (state.value) {
    case 'loading-info':
      return t('common.loading')
    case 'ready':
      return downloadInfo.value
        ? t('cli.downloadWithSize', { size: downloadInfo.value.size_human })
        : t('cli.download')
    case 'downloading':
      return t('cli.downloading')
    case 'complete':
      return t('cli.downloadComplete')
    case 'error':
      return t('common.retry')
    default:
      return t('cli.download')
  }
})

const buttonDisabled = computed(() => {
  return state.value === 'loading-info' || state.value === 'downloading'
})

async function loadDownloadInfo() {
  state.value = 'loading-info'
  error.value = null
  try {
    const response = await cliDownloadApi.getDownloadInfo()
    downloadInfo.value = response.data
    state.value = 'ready'
  } catch (e) {
    error.value = t('cli.downloadInfoError')
    state.value = 'error'
    console.error('Failed to load download info:', e)
  }
}

async function startDownload() {
  if (state.value === 'downloading') return

  state.value = 'downloading'
  error.value = null
  emit('download-start')

  try {
    await cliDownloadApi.startDownload()
    startProgressPolling()
  } catch (e) {
    error.value = t('cli.downloadStartError')
    state.value = 'error'
    emit('download-error', error.value)
    console.error('Failed to start download:', e)
  }
}

function startProgressPolling() {
  progressInterval = setInterval(async () => {
    try {
      const response = await cliDownloadApi.getProgress()
      progress.value = response.data

      if (response.data.percentage >= 100) {
        stopProgressPolling()
        state.value = 'complete'
        emit('download-complete')
      }
    } catch (e) {
      console.error('Failed to get progress:', e)
    }
  }, 500)
}

function stopProgressPolling() {
  if (progressInterval) {
    clearInterval(progressInterval)
    progressInterval = null
  }
}

async function cancelDownload() {
  try {
    await cliDownloadApi.cancelDownload()
    stopProgressPolling()
    state.value = 'ready'
    progress.value = null
  } catch (e) {
    console.error('Failed to cancel download:', e)
  }
}

function handleClick() {
  if (state.value === 'idle') {
    loadDownloadInfo()
  } else if (state.value === 'ready' || state.value === 'error') {
    startDownload()
  }
}

onUnmounted(() => {
  stopProgressPolling()
})

defineExpose({
  loadInfo: loadDownloadInfo,
  startDownload,
  cancelDownload,
})
</script>

<template>
  <div class="download-button">
    <!-- Download Info -->
    <div v-if="props.showInfo && downloadInfo && state !== 'downloading'" class="mb-3 text-sm text-gray-500 dark:text-gray-400">
      <div class="flex items-center gap-4">
        <span>{{ t('cli.version') }}: {{ downloadInfo.version }}</span>
        <span>{{ t('cli.size') }}: {{ downloadInfo.size_human }}</span>
        <span>{{ t('cli.platform') }}: {{ downloadInfo.platform }}</span>
      </div>
    </div>

    <!-- Progress Bar (when downloading) -->
    <DownloadProgress
      v-if="state === 'downloading'"
      :progress="progress"
      :downloading="true"
      @cancel="cancelDownload"
    />

    <!-- Download Button -->
    <button
      v-else
      :disabled="buttonDisabled"
      :class="[
        'w-full px-4 py-3 rounded-lg font-medium transition-colors flex items-center justify-center gap-2',
        state === 'complete'
          ? 'bg-green-600 text-white cursor-default'
          : state === 'error'
            ? 'bg-red-600 hover:bg-red-700 text-white'
            : 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white disabled:opacity-50 disabled:cursor-not-allowed'
      ]"
      @click="handleClick"
    >
      <!-- Loading Spinner -->
      <svg
        v-if="state === 'loading-info'"
        class="animate-spin h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
      </svg>

      <!-- Download Icon -->
      <svg
        v-else-if="state === 'idle' || state === 'ready'"
        class="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
      </svg>

      <!-- Check Icon -->
      <svg
        v-else-if="state === 'complete'"
        class="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
      </svg>

      <!-- Error Icon -->
      <svg
        v-else-if="state === 'error'"
        class="h-5 w-5"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
      </svg>

      <span>{{ buttonText }}</span>
    </button>

    <!-- Error Message -->
    <p v-if="error" class="mt-2 text-sm text-red-600 dark:text-red-400">
      {{ error }}
    </p>
  </div>
</template>
