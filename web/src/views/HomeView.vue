<script setup lang="ts">
import { onMounted, onUnmounted, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSystemStore } from '@/stores/system'
import { systemApi } from '@/api/index'
import type { SystemMetrics, DetailedSystemInfo } from '@/api/system'
import { storeToRefs } from 'pinia'
import ProgressBar from '@/components/ProgressBar.vue'
import DonutChart from '@/components/DonutChart.vue'
import { ConfigurableDashboard } from '@/components/dashboard'

const { t } = useI18n()
const systemStore = useSystemStore()
const { health: _health, loading: _loading } = storeToRefs(systemStore)

let refreshInterval: ReturnType<typeof setInterval> | null = null
const autoRefresh = ref(true)

// AbortController for request cancellation
let abortController: AbortController | null = null

// Metrics history for dashboard (use shallowRef for performance)
const metricsHistory = shallowRef<SystemMetrics[]>([])

// Detailed system info
const detailedInfo = ref<DetailedSystemInfo | null>(null)
const detailedInfoLoading = ref(false)
const showDetailedInfo = ref(false)

// Visibility API support
const isPageVisible = ref(true)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

async function fetchMetricsHistory() {
  try {
    // Cancel previous request if still pending
    if (abortController) {
      abortController.abort()
    }

    abortController = new AbortController()
    const response = await systemApi.getMetricsHistory('5m')
    metricsHistory.value = response.data.metrics || []
  } catch (error: any) {
    // Don't update state if request was aborted
    if (error?.name !== 'AbortError' && error?.name !== 'CanceledError') {
      metricsHistory.value = []
    }
  }
}

async function fetchDetailedInfo() {
  detailedInfoLoading.value = true
  try {
    const response = await systemApi.getInfo(true)
    detailedInfo.value = response.data.system || null
  } catch {
    detailedInfo.value = null
  } finally {
    detailedInfoLoading.value = false
  }
}

function toggleDetailedInfo() {
  showDetailedInfo.value = !showDetailedInfo.value
  if (showDetailedInfo.value && !detailedInfo.value) {
    fetchDetailedInfo()
  }
}

// Handle visibility change to pause/resume polling
function handleVisibilityChange() {
  isPageVisible.value = !document.hidden
}

// Debounced refresh function
let refreshDebounceTimer: ReturnType<typeof setTimeout> | null = null
function debouncedRefresh() {
  if (refreshDebounceTimer) {
    clearTimeout(refreshDebounceTimer)
  }
  refreshDebounceTimer = setTimeout(() => {
    if (autoRefresh.value && isPageVisible.value) {
      systemStore.fetchAll()
      fetchMetricsHistory()
    }
  }, 100)
}

// Watch autoRefresh changes with debouncing
watch(autoRefresh, (newValue) => {
  if (newValue && isPageVisible.value) {
    debouncedRefresh()
  }
})

onMounted(async () => {
  await systemStore.fetchAll()
  await fetchMetricsHistory()

  // Add visibility change listener
  document.addEventListener('visibilitychange', handleVisibilityChange)

  // Optimized polling interval: 15 seconds (reduced from 5s = 66% fewer API calls)
  refreshInterval = setInterval(() => {
    if (autoRefresh.value && isPageVisible.value) {
      systemStore.fetchAll()
      fetchMetricsHistory()
    }
  }, 15000)
})

onUnmounted(() => {
  // Cleanup interval
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }

  // Cleanup debounce timer
  if (refreshDebounceTimer) {
    clearTimeout(refreshDebounceTimer)
    refreshDebounceTimer = null
  }

  // Cancel pending requests
  if (abortController) {
    abortController.abort()
    abortController = null
  }

  // Remove visibility listener
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <div class="home-page">
    <!-- Hero Section -->
    <div class="hero-section">
      <div class="hero-icon">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
        </svg>
      </div>
      <h1 class="hero-title">{{ t('home.welcome') }}</h1>
      <p class="hero-description">{{ t('home.description') }}</p>
    </div>

    <!-- Dashboard Section -->
    <div class="dashboard-section">
      <ConfigurableDashboard :metrics-history="metricsHistory">
        <template #header-left>
          <label class="toggle-label">
            <input v-model="autoRefresh" type="checkbox" class="toggle-checkbox" />
            <span class="toggle-text">{{ t('dashboard.autoRefresh') }}</span>
          </label>
        </template>
      </ConfigurableDashboard>
    </div>

    <!-- Detailed System Info Toggle -->
    <div class="glass-card p-6 mb-4">
      <div class="flex items-center justify-between">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('system.detailedSystemInfo') }}</h3>
        <button
          class="px-3 py-1.5 text-sm bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
          @click="toggleDetailedInfo"
        >
          {{ showDetailedInfo ? t('common.close') : t('system.detailedInfo') }}
        </button>
      </div>
    </div>

    <!-- Detailed System Info -->
    <div v-if="showDetailedInfo" class="details-section space-y-4">
      <div v-if="detailedInfoLoading" class="glass-card p-6 text-center text-gray-500 dark:text-gray-400">
        {{ t('system.loadingDetailedInfo') }}
      </div>
      <template v-else-if="detailedInfo">
        <!-- OS Info -->
        <div class="glass-card p-6">
          <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.osInfo') }}</h4>
          <div class="grid md:grid-cols-3 gap-4 text-sm">
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('system.osVersion') }}:</span>
              <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.version || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('system.kernel') }}:</span>
              <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.kernel || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('system.architecture') }}:</span>
              <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.architecture || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('system.hostname') }}:</span>
              <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.hostname || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('system.uptime') }}:</span>
              <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.uptime_human || '-' }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('system.bootTime') }}:</span>
              <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.os.boot_time ? new Date(detailedInfo.os.boot_time * 1000).toLocaleString() : '-' }}</span>
            </div>
          </div>
        </div>

        <!-- CPU Info -->
        <div class="glass-card p-6">
          <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.cpuInfo') }}</h4>
          <div class="flex flex-col md:flex-row gap-6">
            <div class="flex-shrink-0 flex justify-center">
              <DonutChart
                :value="detailedInfo.hardware.cpu.usage || 0"
                :max="100"
                :label="t('system.cpuUsage')"
                color="auto"
                :size="140"
              />
            </div>
            <div class="flex-1 grid sm:grid-cols-2 gap-4 text-sm">
              <div class="sm:col-span-2">
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuModel') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.model || '-' }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuCores') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.cores || '-' }} {{ t('system.cores') }} / {{ detailedInfo.hardware.cpu.threads || '-' }} {{ t('system.threads') }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-gray-400">{{ t('system.cpuFrequency') }}:</span>
                <span class="text-gray-900 dark:text-white ml-2">{{ detailedInfo.hardware.cpu.frequency ? `${detailedInfo.hardware.cpu.frequency.toFixed(0)} MHz` : '-' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Memory Info -->
        <div class="glass-card p-6">
          <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.memoryInfo') }}</h4>
          <div class="flex flex-col md:flex-row gap-6">
            <div class="flex-shrink-0 flex gap-6 justify-center">
              <DonutChart
                :value="detailedInfo.hardware.memory.used"
                :max="detailedInfo.hardware.memory.total"
                :label="t('system.ram')"
                :value-label="formatBytes(detailedInfo.hardware.memory.used)"
                color="auto"
                :size="120"
              />
              <DonutChart
                v-if="detailedInfo.hardware.memory.swap_total > 0"
                :value="detailedInfo.hardware.memory.swap_used"
                :max="detailedInfo.hardware.memory.swap_total"
                :label="t('system.swap')"
                :value-label="formatBytes(detailedInfo.hardware.memory.swap_used)"
                color="purple"
                :size="120"
              />
            </div>
            <div class="flex-1 space-y-4">
              <div>
                <div class="flex justify-between text-sm mb-1">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.ram') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ formatBytes(detailedInfo.hardware.memory.used) }} / {{ formatBytes(detailedInfo.hardware.memory.total) }}</span>
                </div>
                <ProgressBar
                  :value="detailedInfo.hardware.memory.used"
                  :max="detailedInfo.hardware.memory.total"
                  :show-percent="false"
                  color="auto"
                  size="md"
                />
              </div>
              <div v-if="detailedInfo.hardware.memory.swap_total > 0">
                <div class="flex justify-between text-sm mb-1">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.swap') }}</span>
                  <span class="text-gray-900 dark:text-white">{{ formatBytes(detailedInfo.hardware.memory.swap_used) }} / {{ formatBytes(detailedInfo.hardware.memory.swap_total) }}</span>
                </div>
                <ProgressBar
                  :value="detailedInfo.hardware.memory.swap_used"
                  :max="detailedInfo.hardware.memory.swap_total"
                  :show-percent="false"
                  color="purple"
                  size="md"
                />
              </div>
            </div>
          </div>
        </div>

        <!-- Disk Info -->
        <div v-if="detailedInfo.hardware.disk?.length" class="glass-card p-6">
          <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.diskInfo') }}</h4>
          <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
            <div
              v-for="disk in detailedInfo.hardware.disk.slice(0, 6)"
              :key="disk.device"
              class="bg-gray-50 dark:bg-gray-700/50 rounded-lg p-4"
            >
              <div class="flex items-center justify-between mb-2">
                <span class="font-medium text-gray-900 dark:text-white text-sm truncate" :title="disk.mount_point">{{ disk.mount_point }}</span>
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ disk.fs_type }}</span>
              </div>
              <ProgressBar
                :value="disk.used"
                :max="disk.total"
                :show-percent="false"
                color="auto"
                size="lg"
              />
              <div class="flex justify-between text-xs text-gray-500 dark:text-gray-400 mt-2">
                <span>{{ formatBytes(disk.used) }} {{ t('system.used') }}</span>
                <span>{{ formatBytes(disk.available) }} {{ t('system.free') }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Network Info -->
        <div v-if="detailedInfo.network?.interfaces?.length" class="glass-card p-6">
          <h4 class="text-base font-semibold text-gray-900 dark:text-white mb-4">{{ t('system.networkInfo') }}</h4>
          <div class="space-y-4">
            <div v-for="iface in detailedInfo.network.interfaces.filter(i => !i.is_loopback && i.is_up)" :key="iface.name" class="border-b border-gray-100 dark:border-gray-700/50 pb-4 last:border-0 last:pb-0">
              <div class="flex items-center gap-2 mb-2">
                <span class="font-medium text-gray-900 dark:text-white">{{ iface.name }}</span>
                <span class="px-2 py-0.5 text-xs rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">
                  {{ t('system.interfaceUp') }}
                </span>
              </div>
              <div class="grid md:grid-cols-3 gap-2 text-sm">
                <div v-if="iface.mac">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.macAddress') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2 font-mono text-xs">{{ iface.mac }}</span>
                </div>
                <div v-if="iface.ipv4?.length">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.ipv4Address') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2 font-mono text-xs">{{ iface.ipv4.join(', ') }}</span>
                </div>
                <div>
                  <span class="text-gray-500 dark:text-gray-400">{{ t('system.mtu') }}:</span>
                  <span class="text-gray-900 dark:text-white ml-2">{{ iface.mtu }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
      <div v-else class="glass-card p-6 text-center text-gray-500 dark:text-gray-400">
        {{ t('system.noDetailedInfo') }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 16px;
}

/* Hero Section */
.hero-section {
  text-align: center;
  margin-bottom: 24px;
}

.hero-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 16px;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--color-accent, #3B82F6), var(--color-cta, #10B981));
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 24px rgba(59, 130, 246, 0.3);
}

.hero-title {
  font-size: 24px;
  font-weight: 700;
  color: var(--color-text-primary);
  margin: 0 0 8px;
}

.hero-description {
  font-size: 14px;
  color: var(--color-text-secondary);
  max-width: 480px;
  margin: 0 auto;
  line-height: 1.5;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.toggle-checkbox {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1px solid var(--glass-border);
  background: var(--glass-bg);
  cursor: pointer;
  accent-color: var(--color-accent);
}

.toggle-text {
  font-size: 13px;
  color: var(--color-text-secondary);
}

/* Sections */
.dashboard-section,
.details-section {
  margin-bottom: 24px;
}

.glass-card {
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: 12px;
}

/* Light mode adjustments */
:root.light .glass-card {
  background: rgba(255, 255, 255, 0.8);
  border-color: rgba(0, 0, 0, 0.08);
}
</style>
