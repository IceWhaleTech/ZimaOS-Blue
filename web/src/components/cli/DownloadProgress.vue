<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CLIDownloadProgress } from '@/api/setup'

const { t } = useI18n()

const props = defineProps<{
  progress: CLIDownloadProgress | null
  downloading: boolean
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const progressPercent = computed(() => {
  if (!props.progress) return 0
  return Math.round(props.progress.percentage)
})

const downloadedText = computed(() => {
  if (!props.progress) return ''
  const downloaded = formatBytes(props.progress.downloaded)
  const total = formatBytes(props.progress.total)
  return `${downloaded} / ${total}`
})

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function handleCancel() {
  emit('cancel')
}
</script>

<template>
  <div class="download-progress">
    <!-- Progress Bar -->
    <div class="mb-2">
      <div class="flex justify-between text-sm mb-1">
        <span class="text-gray-700 dark:text-gray-300">
          {{ downloading ? t('cli.downloading') : t('cli.downloadComplete') }}
        </span>
        <span class="text-gray-500 dark:text-gray-400">{{ progressPercent }}%</span>
      </div>
      <div class="h-2 bg-gray-700 dark:bg-gray-700 rounded-full overflow-hidden">
        <div
          class="h-full bg-gray-700 dark:bg-gray-700 transition-all duration-300"
          :style="{ width: `${progressPercent}%` }"
        />
      </div>
    </div>

    <!-- Progress Details -->
    <div v-if="progress" class="flex items-center justify-between text-sm text-gray-500 dark:text-gray-400">
      <div class="flex items-center gap-4">
        <span>{{ downloadedText }}</span>
        <span v-if="downloading && progress.speed_human">
          {{ progress.speed_human }}
        </span>
      </div>
      <div class="flex items-center gap-3">
        <span v-if="downloading && progress.eta">
          {{ t('cli.eta', { time: progress.eta }) }}
        </span>
        <button
          v-if="downloading"
          class="text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300"
          @click="handleCancel"
        >
          {{ t('common.cancel') }}
        </button>
      </div>
    </div>
  </div>
</template>
