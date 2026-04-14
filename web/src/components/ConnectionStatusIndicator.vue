<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

// Connection state
const isOnline = ref(navigator.onLine)
const connectionQuality = ref<'excellent' | 'good' | 'poor' | 'offline'>('good')
const latency = ref<number | null>(null)
const lastPingTime = ref<number>(Date.now())
const reconnecting = ref(false)
const reconnectAttempts = ref(0)
const showReconnectBanner = ref(false)

// Ping interval
let pingInterval: ReturnType<typeof setInterval> | null = null
const PING_INTERVAL = 10000 // 10 seconds
const PING_TIMEOUT = 5000 // 5 seconds

// Measure latency by pinging the server
async function measureLatency(): Promise<number | null> {
  const start = performance.now()
  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), PING_TIMEOUT)

    await fetch('/api/health', {
      method: 'HEAD',
      signal: controller.signal,
      cache: 'no-store',
    })

    clearTimeout(timeoutId)
    const end = performance.now()
    return Math.round(end - start)
  } catch {
    return null
  }
}

// Update connection quality based on latency
function updateConnectionQuality(lat: number | null) {
  if (!isOnline.value || lat === null) {
    connectionQuality.value = 'offline'
  } else if (lat < 100) {
    connectionQuality.value = 'excellent'
  } else if (lat < 300) {
    connectionQuality.value = 'good'
  } else {
    connectionQuality.value = 'poor'
  }
}

// Perform ping and update state
async function performPing() {
  if (!isOnline.value) {
    connectionQuality.value = 'offline'
    return
  }

  const lat = await measureLatency()
  latency.value = lat
  lastPingTime.value = Date.now()
  updateConnectionQuality(lat)

  // If ping failed, show reconnect banner
  if (lat === null && isOnline.value) {
    showReconnectBanner.value = true
    reconnecting.value = true
    attemptReconnect()
  } else if (lat !== null) {
    showReconnectBanner.value = false
    reconnecting.value = false
    reconnectAttempts.value = 0
  }
}

// Attempt to reconnect
async function attemptReconnect() {
  if (!reconnecting.value) return

  reconnectAttempts.value++
  const lat = await measureLatency()

  if (lat !== null) {
    // Reconnected successfully
    latency.value = lat
    updateConnectionQuality(lat)
    showReconnectBanner.value = false
    reconnecting.value = false
    reconnectAttempts.value = 0
  } else if (reconnectAttempts.value < 5) {
    // Retry after delay
    setTimeout(attemptReconnect, 2000 * reconnectAttempts.value)
  }
}

// Handle online/offline events
function handleOnline() {
  isOnline.value = true
  showReconnectBanner.value = false
  reconnecting.value = false
  reconnectAttempts.value = 0
  performPing()
}

function handleOffline() {
  isOnline.value = false
  connectionQuality.value = 'offline'
  showReconnectBanner.value = true
}

// Computed properties
const qualityColor = computed(() => {
  switch (connectionQuality.value) {
    case 'excellent':
      return 'bg-green-500'
    case 'good':
      return 'bg-gray-700 dark:bg-gray-500'
    case 'poor':
      return 'bg-yellow-500'
    case 'offline':
      return 'bg-red-500'
    default:
      return 'bg-gray-500'
  }
})

const qualityLabel = computed(() => {
  switch (connectionQuality.value) {
    case 'excellent':
      return t('connections.quality.excellent')
    case 'good':
      return t('connections.quality.good')
    case 'poor':
      return t('connections.quality.poor')
    case 'offline':
      return t('connections.quality.offline')
    default:
      return ''
  }
})

const latencyDisplay = computed(() => {
  if (latency.value === null) return '--'
  return `${latency.value}ms`
})

// Lifecycle
onMounted(() => {
  window.addEventListener('online', handleOnline)
  window.addEventListener('offline', handleOffline)

  // Initial ping
  performPing()

  // Start ping interval
  pingInterval = setInterval(performPing, PING_INTERVAL)
})

onUnmounted(() => {
  window.removeEventListener('online', handleOnline)
  window.removeEventListener('offline', handleOffline)

  if (pingInterval) {
    clearInterval(pingInterval)
  }
})

// Watch for online status changes
watch(isOnline, (newVal) => {
  if (newVal) {
    performPing()
  }
})
</script>

<template>
  <div class="connection-status-indicator">
    <!-- Reconnect Banner -->
    <Transition name="slide-down">
      <div
        v-if="showReconnectBanner"
        class="fixed inset-x-0 top-0 z-50 bg-red-500 text-white px-4 py-2 text-center text-sm"
      >
        <div class="flex items-center justify-center gap-2">
          <svg
            v-if="reconnecting"
            class="animate-spin h-4 w-4"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
              fill="none"
            />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          <span v-if="!isOnline">{{ t('connections.offline') }}</span>
          <span v-else-if="reconnecting">
            {{ t('connections.reconnecting') }}
            <span v-if="reconnectAttempts > 1">({{ reconnectAttempts }}/5)</span>
          </span>
          <button
            v-if="!reconnecting"
            class="ms-2 px-2 py-0.5 bg-white/20 rounded hover:bg-white/30 transition-colors"
            @click="attemptReconnect"
          >
            {{ t('connections.retry') }}
          </button>
        </div>
      </div>
    </Transition>

    <!-- Status Indicator (for status bar) -->
    <div
      class="flex items-center gap-1.5 px-2 py-1 rounded-md hover:bg-gray-100 dark:hover:bg-slate-700 cursor-pointer transition-colors"
      :title="`${qualityLabel} - ${latencyDisplay}`"
    >
      <!-- Signal bars -->
      <div class="flex items-end gap-0.5 h-3">
        <div
          :class="[
            'w-1 rounded-sm transition-colors',
            connectionQuality !== 'offline' ? qualityColor : 'bg-gray-300 dark:bg-gray-600',
          ]"
          style="height: 33%"
        />
        <div
          :class="[
            'w-1 rounded-sm transition-colors',
            connectionQuality === 'excellent' || connectionQuality === 'good'
              ? qualityColor
              : 'bg-gray-300 dark:bg-gray-600',
          ]"
          style="height: 66%"
        />
        <div
          :class="[
            'w-1 rounded-sm transition-colors',
            connectionQuality === 'excellent' ? qualityColor : 'bg-gray-300 dark:bg-gray-600',
          ]"
          style="height: 100%"
        />
      </div>

      <!-- Latency text -->
      <span class="text-xs text-gray-500 dark:text-slate-400 tabular-nums">
        {{ latencyDisplay }}
      </span>
    </div>
  </div>
</template>

<style scoped>
.slide-down-enter-active,
.slide-down-leave-active {
  transition:
    transform 0.3s ease,
    opacity 0.3s ease;
}

.slide-down-enter-from,
.slide-down-leave-to {
  transform: translateY(-100%);
  opacity: 0;
}
</style>
