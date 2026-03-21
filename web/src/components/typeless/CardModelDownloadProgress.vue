<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardModelDownloadProgress } from '@/types/typeless'
import { authFetch } from '@/api/client'

const props = defineProps<{
  card: TypelessCardModelDownloadProgress
}>()

const { t } = useI18n()

const currentStatus = ref(props.card.status)
const currentMessage = ref(props.card.message || '')
const currentProgress = ref(props.card.progress)
const currentState = ref(props.card.state)
const currentError = ref(props.card.error || '')
const currentFiles = ref(props.card.files || [])
const isReady = ref(props.card.ready)
const isDownloading = ref(props.card.downloading)

const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
const pollIntervalMs = computed(() => Math.max(500, props.card.poll_interval_ms || 1500))
const isTerminal = computed(() => currentStatus.value === 'ready' || currentStatus.value === 'error')

const indicatorClass = computed(() => {
  if (currentStatus.value === 'ready') {
    return 'bg-emerald-500'
  }
  if (currentStatus.value === 'error') {
    return 'bg-rose-500'
  }
  return 'bg-amber-500 animate-pulse'
})

const statusText = computed(() => {
  if (currentStatus.value === 'downloading') {
    return t('media.modelDownload.status.downloading', 'Downloading metadata')
  }
  if (currentStatus.value === 'ready') {
    return t('media.modelDownload.status.ready', 'Model ready')
  }
  if (currentStatus.value === 'error') {
    return currentError.value || t('media.modelDownload.status.error', 'Download failed')
  }
  return t('media.modelDownload.status.pending', 'Preparing download')
})

const progressWidth = computed(() => {
  if (!currentProgress.value || currentProgress.value.total === 0) {
    return 0
  }
  return Math.min(100, Math.round(currentProgress.value.percentage))
})

function stopPolling() {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
}

function resolveStatus(payload: Partial<TypelessCardModelDownloadProgress>): TypelessCardModelDownloadProgress['status'] {
  if (payload.status) {
    return payload.status
  }
  if (payload.ready) {
    return 'ready'
  }
  if (payload.downloading) {
    return 'downloading'
  }
  if (payload.state === 'error') {
    return 'error'
  }
  return 'not_downloaded'
}

function applyPayload(payload: TypelessCardModelDownloadProgress) {
  currentStatus.value = resolveStatus(payload)
  currentMessage.value = payload.message || currentMessage.value
  currentProgress.value = payload.progress || currentProgress.value
  currentState.value = payload.state || currentState.value
  currentError.value = payload.error || ''
  currentFiles.value = payload.files || currentFiles.value
  isReady.value = payload.ready
  isDownloading.value = payload.downloading
}

async function fetchStatus() {
  if (!props.card.status_url || isTerminal.value) {
    return
  }
  try {
    const resp = await authFetch(props.card.status_url)
    if (!resp.ok) {
      return
    }
    const payload = (await resp.json()) as TypelessCardModelDownloadProgress
    if (payload.model_id !== props.card.model_id) {
      return
    }
    applyPayload(payload)
    if (isTerminal.value) {
      stopPolling()
    }
  } catch {
    // ignore transient errors
  }
}

function startPolling() {
  stopPolling()
  if (isTerminal.value) {
    return
  }
  fetchStatus()
  pollTimer.value = setInterval(fetchStatus, pollIntervalMs.value)
}

watch(
  () => props.card.status_url,
  () => {
    applyPayload(props.card)
    startPolling()
  }
)

watch(
  () => props.card,
  (value) => {
    applyPayload(value)
    if (!isTerminal.value) {
      startPolling()
    }
  },
  { deep: true }
)

onMounted(startPolling)
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden shadow-sm">
    <div class="flex items-center gap-3 px-4 py-3 border-b border-gray-100 dark:border-gray-700/50">
      <span class="w-2.5 h-2.5 rounded-full" :class="indicatorClass" />
      <div class="min-w-0">
        <p class="text-sm font-semibold text-gray-800 dark:text-gray-100 truncate">
          {{ props.card.title }}
        </p>
        <p class="text-xs text-gray-500 dark:text-gray-400 truncate">
          {{ statusText }}
        </p>
      </div>
      <span v-if="currentState" class="text-[11px] font-mono text-gray-400 dark:text-gray-500">
        {{ currentState }}
      </span>
    </div>
    <div v-if="currentStatus === 'downloading' && currentProgress" class="px-4 pt-4">
      <div class="h-1.5 rounded-full bg-gray-100 dark:bg-gray-700 overflow-hidden">
        <div class="h-full bg-amber-500 transition-all duration-300" :style="{ width: `${progressWidth}%` }" />
      </div>
      <div class="mt-2 flex items-center justify-between text-[11px] text-gray-500 dark:text-gray-400">
        <span>
          {{ currentProgress?.file || t('media.modelDownload.auto.fileName', 'Processing files') }}
        </span>
        <span>
          {{ progressWidth }}%
        </span>
      </div>
    </div>
    <div class="px-4 py-4 space-y-2">
      <p v-if="currentMessage" class="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap break-words">
        {{ currentMessage }}
      </p>
      <p v-if="currentStatus === 'error' && currentError" class="text-sm text-rose-600 dark:text-rose-300">
        {{ currentError }}
      </p>
      <ul v-if="currentFiles.length > 0" class="space-y-1">
        <li
          v-for="file in currentFiles"
          :key="file.filename"
          class="flex items-center justify-between text-xs text-gray-600 dark:text-gray-300"
        >
          <span>{{ file.filename }}</span>
          <span class="flex items-center gap-2 text-[11px]">
            <span v-if="file.downloaded">{{ t('media.modelDownload.completed', 'Downloaded') }}</span>
            <span v-else>{{ t('media.modelDownload.pending', 'Pending') }}</span>
            <span v-if="file.size">· {{ file.size }}</span>
          </span>
        </li>
      </ul>
    </div>
  </div>
</template>
