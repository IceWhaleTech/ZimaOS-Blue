<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { statisticsApi, type UsageStats, type ConsentStatus } from '@/api/setup'
import UsageChart from './UsageChart.vue'
import CostEstimate from './CostEstimate.vue'

const { t } = useI18n()

const loading = ref(true)
const stats = ref<UsageStats | null>(null)
const consent = ref<ConsentStatus | null>(null)
const error = ref<string | null>(null)
const exporting = ref(false)

const hasData = computed(() => {
  return stats.value && stats.value.total_calls > 0
})

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [statsResponse, consentResponse] = await Promise.all([
      statisticsApi.getStats(),
      statisticsApi.getConsentStatus(),
    ])
    stats.value = statsResponse.data
    consent.value = consentResponse.data
  } catch (e) {
    error.value = t('stats.loadError')
    console.error('Failed to load statistics:', e)
  } finally {
    loading.value = false
  }
}

async function handleExport(format: 'json' | 'csv') {
  exporting.value = true
  try {
    const response = await statisticsApi.exportStats(format)
    const blob = new Blob([response.data], {
      type: format === 'json' ? 'application/json' : 'text/csv',
    })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `echo-stats-${new Date().toISOString().split('T')[0]}.${format}`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    console.error('Failed to export stats:', e)
  } finally {
    exporting.value = false
  }
}

async function handleClearStats() {
  if (!confirm(t('stats.clearConfirm'))) return

  try {
    await statisticsApi.clearStats()
    await loadData()
  } catch (e) {
    console.error('Failed to clear stats:', e)
  }
}

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="stats-dashboard">
    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-2 border-gray-300 border-t-gray-900 dark:border-t-gray-400" />
    </div>

    <!-- Error -->
    <div v-else-if="error" class="text-center py-12">
      <p class="text-red-600 dark:text-red-400 mb-4">{{ error }}</p>
      <button
        class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg"
        @click="loadData"
      >
        {{ t('common.retry') }}
      </button>
    </div>

    <!-- Not Consented -->
    <div v-else-if="consent && !consent.consented" class="text-center py-12">
      <svg class="h-16 w-16 text-gray-400 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
      <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-2">
        {{ t('stats.notEnabled') }}
      </h3>
      <p class="text-gray-500 dark:text-gray-400">
        {{ t('stats.enableInSettings') }}
      </p>
    </div>

    <!-- Stats Content -->
    <template v-else-if="stats">
      <!-- Header -->
      <div class="flex items-center justify-between mb-6">
        <div>
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('stats.title') }}
          </h2>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('stats.period', { start: stats.period_start, end: stats.period_end }) }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button
            :disabled="exporting"
            class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg transition-colors"
            @click="handleExport('json')"
          >
            {{ t('stats.exportJSON') }}
          </button>
          <button
            :disabled="exporting"
            class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 hover:bg-gray-200 dark:hover:bg-gray-600 text-gray-700 dark:text-gray-300 rounded-lg transition-colors"
            @click="handleExport('csv')"
          >
            {{ t('stats.exportCSV') }}
          </button>
        </div>
      </div>

      <!-- No Data -->
      <div v-if="!hasData" class="text-center py-12 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
        <svg class="h-12 w-12 text-gray-400 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
        </svg>
        <p class="text-gray-500 dark:text-gray-400">{{ t('stats.noData') }}</p>
      </div>

      <!-- Stats Grid -->
      <div v-else class="space-y-6">
        <!-- Summary Cards -->
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('stats.totalCalls') }}</p>
            <p class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ stats.total_calls.toLocaleString() }}
            </p>
          </div>
          <div class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('stats.inputTokens') }}</p>
            <p class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ stats.input_tokens.toLocaleString() }}
            </p>
          </div>
          <div class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('stats.outputTokens') }}</p>
            <p class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ stats.output_tokens.toLocaleString() }}
            </p>
          </div>
          <div class="bg-white dark:bg-gray-700 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('stats.errors') }}</p>
            <p class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ stats.error_count.toLocaleString() }}
            </p>
          </div>
        </div>

        <!-- Charts -->
        <div class="grid md:grid-cols-2 gap-6">
          <UsageChart
            :title="t('stats.callsByProvider')"
            :data="stats.calls_by_provider || {}"
          />
          <UsageChart
            :title="t('stats.callsByModel')"
            :data="stats.calls_by_model || {}"
          />
        </div>

        <!-- Cost Estimate -->
        <CostEstimate :cost="stats.estimated_cost_usd ?? 0" />

        <!-- Clear Stats -->
        <div class="pt-4 border-t border-gray-200 dark:border-gray-700">
          <button
            class="text-sm text-red-600 dark:text-red-400 hover:underline"
            @click="handleClearStats"
          >
            {{ t('stats.clearAll') }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>
