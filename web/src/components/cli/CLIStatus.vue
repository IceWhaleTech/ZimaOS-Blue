<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { cliDownloadApi, type CLIStatus } from '@/api/setup'

const { t } = useI18n()

const props = defineProps<{
  compact?: boolean
}>()

const emit = defineEmits<{
  (e: 'download-click'): void
  (e: 'update-click'): void
}>()

const loading = ref(true)
const status = ref<CLIStatus | null>(null)
const error = ref<string | null>(null)

const statusIcon = computed(() => {
  if (!status.value) return 'unknown'
  if (status.value.installed) return 'installed'
  return 'not-installed'
})

const statusColor = computed(() => {
  if (!status.value) return 'gray'
  if (status.value.installed) return 'green'
  return 'yellow'
})

const statusText = computed(() => {
  if (!status.value) return t('cli.status.unknown')
  if (status.value.installed) {
    return t('cli.status.installed', { version: status.value.version || 'unknown' })
  }
  return t('cli.status.notInstalled')
})

async function loadStatus() {
  loading.value = true
  error.value = null
  try {
    const response = await cliDownloadApi.getStatus()
    status.value = response.data
  } catch (e) {
    error.value = t('cli.status.loadError')
    console.error('Failed to load CLI status:', e)
  } finally {
    loading.value = false
  }
}

function handleDownloadClick() {
  emit('download-click')
}

function handleUpdateClick() {
  emit('update-click')
}

onMounted(() => {
  loadStatus()
})

defineExpose({
  refresh: loadStatus,
})
</script>

<template>
  <div
    :class="[
      'cli-status rounded-lg border transition-colors',
      props.compact ? 'p-3' : 'p-4',
      statusColor === 'green'
        ? 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20'
        : statusColor === 'yellow'
          ? 'border-yellow-200 dark:border-yellow-800 bg-yellow-50 dark:bg-yellow-900/20'
          : 'border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-700/50',
    ]"
  >
    <!-- Loading State -->
    <div v-if="loading" class="flex items-center gap-3">
      <div
        class="animate-spin rounded-full h-5 w-5 border-2 border-gray-300 border-t-gray-900 dark:border-t-gray-400"
      />
      <span class="text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="flex items-center gap-3 text-red-600 dark:text-red-400">
      <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
      <span>{{ error }}</span>
      <button class="text-sm underline hover:no-underline" @click="loadStatus">
        {{ t('common.retry') }}
      </button>
    </div>

    <!-- Status Display -->
    <div v-else class="flex items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <!-- Status Icon -->
        <div
          :class="[
            'flex-shrink-0 rounded-full p-2',
            statusColor === 'green'
              ? 'bg-green-100 dark:bg-green-800/50 text-green-600 dark:text-green-400'
              : statusColor === 'yellow'
                ? 'bg-yellow-100 dark:bg-yellow-800/50 text-yellow-600 dark:text-yellow-400'
                : 'bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400',
          ]"
        >
          <svg
            v-if="statusIcon === 'installed'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M5 13l4 4L19 7"
            />
          </svg>
          <svg
            v-else-if="statusIcon === 'not-installed'"
            class="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
            />
          </svg>
          <svg v-else class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        </div>

        <!-- Status Text -->
        <div>
          <div class="font-medium text-gray-900 dark:text-white">
            {{ statusText }}
          </div>
          <div
            v-if="status?.source && !props.compact"
            class="text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t('cli.status.source', { source: status.source }) }}
          </div>
          <div
            v-if="status?.update_available"
            class="text-sm text-gray-900 dark:text-white dark:text-white"
          >
            {{ t('cli.status.updateAvailable', { version: status.latest_version }) }}
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex items-center gap-2">
        <button
          v-if="!status?.installed"
          class="px-3 py-1.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm rounded-lg transition-colors"
          @click="handleDownloadClick"
        >
          {{ t('cli.download') }}
        </button>
        <button
          v-else-if="status?.update_available"
          class="px-3 py-1.5 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white text-sm rounded-lg transition-colors"
          @click="handleUpdateClick"
        >
          {{ t('cli.update') }}
        </button>
      </div>
    </div>
  </div>
</template>
