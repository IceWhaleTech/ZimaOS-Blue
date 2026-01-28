<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import DonutChart from '@/components/DonutChart.vue'
import { metricsApi, type ProcessMetrics as ProcessMetricsType } from '@/api/metrics'

const { t } = useI18n()

const echoProcess = ref<ProcessMetricsType | null>(null)
const ccCliProcess = ref<ProcessMetricsType | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

let refreshTimer: ReturnType<typeof setInterval> | null = null

async function fetchProcessMetrics() {
  loading.value = true
  error.value = null
  try {
    // Fetch Echo server process metrics
    const echoResponse = await metricsApi.getProcessMetrics()
    echoProcess.value = echoResponse.data

    // TODO: Fetch CC CLI process metrics when endpoint is available
    // For now, simulate with mock data or leave null
    ccCliProcess.value = null
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to fetch process metrics'
  } finally {
    loading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GB'
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + ' MB'
  if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return bytes + ' B'
}

function formatUptime(nanoseconds: number): string {
  const seconds = nanoseconds / 1e9
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

const echoStats = computed(() => {
  if (!echoProcess.value) return null
  return {
    name: t('metrics.echoServer'),
    pid: echoProcess.value.pid,
    cpu: echoProcess.value.cpu_percent,
    memory: echoProcess.value.memory_rss_bytes,
    memoryPercent: echoProcess.value.memory_percent,
    threads: echoProcess.value.num_threads,
    uptime: echoProcess.value.uptime,
  }
})

const ccCliStats = computed(() => {
  if (!ccCliProcess.value) return null
  return {
    name: t('metrics.claudeCodeCli'),
    pid: ccCliProcess.value.pid,
    cpu: ccCliProcess.value.cpu_percent,
    memory: ccCliProcess.value.memory_rss_bytes,
    memoryPercent: ccCliProcess.value.memory_percent,
    threads: ccCliProcess.value.num_threads,
    uptime: ccCliProcess.value.uptime,
  }
})

onMounted(() => {
  fetchProcessMetrics()
  refreshTimer = setInterval(fetchProcessMetrics, 5000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<template>
  <div class="bg-white dark:bg-gray-800 rounded-lg p-6 shadow">
    <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
      {{ t('metrics.processMetrics') }}
    </h3>

    <div v-if="loading && !echoStats" class="flex items-center justify-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
    </div>

    <div v-else-if="error" class="text-center py-8 text-red-500">
      {{ error }}
    </div>

    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Echo Server Process -->
      <div class="border dark:border-gray-700 rounded-lg p-4">
        <div class="flex items-center justify-between mb-4">
          <div class="flex items-center gap-2">
            <div class="w-3 h-3 rounded-full bg-green-500"></div>
            <h4 class="font-medium text-gray-900 dark:text-white">{{ t('metrics.echoServer') }}</h4>
          </div>
          <span v-if="echoStats" class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('metrics.pid') }}: {{ echoStats.pid }}
          </span>
        </div>

        <div v-if="echoStats" class="space-y-4">
          <!-- CPU & Memory Gauges -->
          <div class="flex justify-around">
            <DonutChart
              :value="echoStats.cpu"
              :max="100"
              :label="t('metrics.cpu')"
              :value-label="echoStats.cpu.toFixed(1) + '%'"
              color="auto"
              :size="100"
              :stroke-width="10"
            />
            <DonutChart
              :value="echoStats.memoryPercent"
              :max="100"
              :label="t('metrics.memory')"
              :value-label="formatBytes(echoStats.memory)"
              color="auto"
              :size="100"
              :stroke-width="10"
            />
          </div>

          <!-- Stats Grid -->
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
              <div class="text-gray-500 dark:text-gray-400">{{ t('metrics.threads') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ echoStats.threads }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
              <div class="text-gray-500 dark:text-gray-400">{{ t('metrics.uptime') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatUptime(echoStats.uptime) }}</div>
            </div>
          </div>
        </div>

        <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
          {{ t('metrics.noData') }}
        </div>
      </div>

      <!-- Claude Code CLI Process -->
      <div class="border dark:border-gray-700 rounded-lg p-4">
        <div class="flex items-center justify-between mb-4">
          <div class="flex items-center gap-2">
            <div :class="['w-3 h-3 rounded-full', ccCliStats ? 'bg-green-500' : 'bg-gray-400']"></div>
            <h4 class="font-medium text-gray-900 dark:text-white">{{ t('metrics.claudeCodeCli') }}</h4>
          </div>
          <span v-if="ccCliStats" class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('metrics.pid') }}: {{ ccCliStats.pid }}
          </span>
        </div>

        <div v-if="ccCliStats" class="space-y-4">
          <!-- CPU & Memory Gauges -->
          <div class="flex justify-around">
            <DonutChart
              :value="ccCliStats.cpu"
              :max="100"
              :label="t('metrics.cpu')"
              :value-label="ccCliStats.cpu.toFixed(1) + '%'"
              color="auto"
              :size="100"
              :stroke-width="10"
            />
            <DonutChart
              :value="ccCliStats.memoryPercent"
              :max="100"
              :label="t('metrics.memory')"
              :value-label="formatBytes(ccCliStats.memory)"
              color="auto"
              :size="100"
              :stroke-width="10"
            />
          </div>

          <!-- Stats Grid -->
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
              <div class="text-gray-500 dark:text-gray-400">{{ t('metrics.threads') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ ccCliStats.threads }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-gray-700/50 rounded p-2">
              <div class="text-gray-500 dark:text-gray-400">{{ t('metrics.uptime') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatUptime(ccCliStats.uptime) }}</div>
            </div>
          </div>
        </div>

        <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
          <p>{{ t('metrics.notRunning') }}</p>
          <p class="text-xs mt-1">{{ t('metrics.ccCliNotDetected') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>
