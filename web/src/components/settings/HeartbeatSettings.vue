<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { heartbeatApi } from '@/api/heartbeat'
import type { HeartbeatStatus } from '@/api/heartbeat'

const { t } = useI18n()
const emit = defineEmits<{ 'status-change': [msg: string] }>()

const status = ref<HeartbeatStatus | null>(null)
const loading = ref(false)
const triggering = ref(false)

const indicatorClass = computed(() => {
  const type = status.value?.last_event?.indicator_type
  switch (type) {
    case 'ok': return 'bg-green-500'
    case 'alert': return 'bg-yellow-500'
    case 'error': return 'bg-red-500'
    default: return 'bg-gray-400'
  }
})

const statusLabel = computed(() => {
  if (!status.value?.enabled) return t('common.disabled')
  const evt = status.value?.last_event
  if (!evt) return t('heartbeat.neverRun')
  return evt.status
})

async function fetchStatus() {
  loading.value = true
  try {
    const res = await heartbeatApi.getStatus()
    status.value = res.data
  } catch (e) {
    console.error('Failed to fetch heartbeat status:', e)
  } finally {
    loading.value = false
  }
}

async function triggerNow() {
  triggering.value = true
  try {
    await heartbeatApi.trigger()
    emit('status-change', t('heartbeat.triggered'))
    // Refresh after a short delay to get the result
    setTimeout(fetchStatus, 3000)
  } catch (e) {
    console.error('Failed to trigger heartbeat:', e)
  } finally {
    triggering.value = false
  }
}

function formatTime(ts?: string) {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

onMounted(fetchStatus)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('heartbeat.title') }}</h3>
      <button
        class="px-3 py-1.5 text-sm rounded-lg transition-colors"
        :class="loading ? 'bg-gray-200 dark:bg-gray-700 text-gray-400' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'"
        :disabled="loading"
        @click="fetchStatus"
      >
        {{ loading ? t('common.refreshing') : t('common.refresh') }}
      </button>
    </div>

    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('heartbeat.description') }}</p>

    <!-- Status Card -->
    <div v-if="status" class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3">
      <!-- Enabled / Indicator -->
      <div class="flex items-center gap-3">
        <span class="w-2.5 h-2.5 rounded-full" :class="indicatorClass" />
        <span class="text-sm font-medium text-gray-900 dark:text-white">{{ statusLabel }}</span>
        <span v-if="status.enabled && status.interval" class="ml-auto text-xs text-gray-500 dark:text-gray-400">
          {{ t('heartbeat.interval') }}: {{ status.interval }}
        </span>
      </div>

      <!-- Last Event -->
      <div v-if="status.last_event" class="text-sm text-gray-600 dark:text-gray-400 space-y-1">
        <div class="flex justify-between">
          <span>{{ t('heartbeat.lastRun') }}</span>
          <span>{{ formatTime(status.last_event.timestamp) }}</span>
        </div>
        <div v-if="status.last_event.reason" class="flex justify-between">
          <span>{{ t('heartbeat.reason') }}</span>
          <span class="text-gray-500">{{ status.last_event.reason }}</span>
        </div>
        <div v-if="status.last_event.duration_ms" class="flex justify-between">
          <span>{{ t('heartbeat.duration') }}</span>
          <span>{{ status.last_event.duration_ms }}ms</span>
        </div>
      </div>

      <!-- Next Due -->
      <div v-if="status.enabled && status.next_due" class="text-sm text-gray-600 dark:text-gray-400 flex justify-between">
        <span>{{ t('heartbeat.nextDue') }}</span>
        <span>{{ formatTime(status.next_due) }}</span>
      </div>

      <!-- Trigger Button -->
      <button
        v-if="status.enabled"
        class="w-full mt-2 px-4 py-2 text-sm font-medium rounded-lg transition-colors"
        :class="triggering ? 'bg-gray-300 dark:bg-gray-600 text-gray-500' : 'bg-gray-800 dark:bg-gray-200 text-white dark:text-gray-900 hover:bg-gray-700 dark:hover:bg-gray-300'"
        :disabled="triggering"
        @click="triggerNow"
      >
        {{ triggering ? t('heartbeat.triggering') : t('heartbeat.triggerNow') }}
      </button>
    </div>

    <!-- Loading -->
    <div v-else-if="loading" class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
      {{ t('common.loading') }}
    </div>
  </div>
</template>
