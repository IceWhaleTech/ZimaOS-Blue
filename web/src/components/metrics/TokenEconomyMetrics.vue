<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyCacheApi, type CacheStats, type PrunerStats } from '@/api/proxyCache'

const { t } = useI18n()

const stats = ref<CacheStats | null>(null)
const prunerStats = ref<PrunerStats | null>(null)
const loading = ref(false)

const hitRate = computed(() => {
  if (!stats.value) return 0
  const total = stats.value.hits + stats.value.misses
  if (total === 0) return 0
  return (stats.value.hits / total) * 100
})

const tokensSaved = computed(() => prunerStats.value?.stats?.tokens_saved ?? 0)

const totalTokensSaved = computed(() => {
  // Cache tokens saved estimate: hits * avg_tokens (rough: 500 tokens per cached response)
  const cacheTokens = (stats.value?.hits ?? 0) * 500
  const prunerTokens = tokensSaved.value
  return cacheTokens + prunerTokens
})

const costSaved = computed(() => {
  // Estimate: $3/M input tokens (Claude Sonnet pricing)
  return (totalTokensSaved.value / 1_000_000) * 3.0
})

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

function formatCost(n: number): string {
  if (n >= 100) return '$' + Math.round(n)
  if (n >= 1) return '$' + n.toFixed(2)
  if (n >= 0.01) return '$' + n.toFixed(2)
  return '$0.00'
}

async function fetchStats() {
  loading.value = true
  try {
    const [cacheRes, prunerRes] = await Promise.all([
      proxyCacheApi.getStats(),
      proxyCacheApi.getPrunerStats().catch(() => null),
    ])
    stats.value = cacheRes.data
    if (prunerRes) prunerStats.value = prunerRes.data
  } catch {
    // Ignore errors
  } finally {
    loading.value = false
  }
}

onMounted(fetchStats)

defineExpose({ refresh: fetchStats })
</script>

<template>
  <div class="space-y-4">
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Cost Saved Card (primary) -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.costSaved') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ costSaved > 0 ? formatCost(costSaved) : '-' }}
          </p>
        </div>
        <div class="p-3 bg-green-100 dark:bg-green-900/30 rounded-full">
          <svg class="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('tokenEconomy.estimated') }}</span>
      </div>
    </div>

    <!-- Tokens Saved Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.tokensSaved') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ totalTokensSaved > 0 ? formatTokens(totalTokensSaved) : '-' }}
          </p>
        </div>
        <div class="p-3 bg-purple-100 dark:bg-purple-900/30 rounded-full">
          <svg class="w-6 h-6 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('tokenEconomy.combined') }}</span>
      </div>
    </div>

    <!-- Cache Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.cache') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats ? hitRate.toFixed(1) + '%' : '-' }}
          </p>
        </div>
        <div class="p-3 bg-indigo-100 dark:bg-indigo-900/30 rounded-full">
          <svg class="w-6 h-6 text-indigo-600 dark:text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ stats?.entries ?? 0 }} {{ t('tokenEconomy.entries') }}</span>
      </div>
    </div>

    <!-- Pruner Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.pruner') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ prunerStats?.stats ? Math.round((1 - prunerStats.stats.avg_compression_rate) * 100) + '%' : '-' }}
          </p>
        </div>
        <div class="p-3 bg-orange-100 dark:bg-orange-900/30 rounded-full">
          <svg class="w-6 h-6 text-orange-600 dark:text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.121 14.121L19 19m-7-7l7-7m-7 7l-2.879 2.879M12 12L9.121 9.121m0 5.758a3 3 0 10-4.243 4.243 3 3 0 004.243-4.243zm0-5.758a3 3 0 10-4.243-4.243 3 3 0 004.243 4.243z" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ prunerStats?.enabled ? t('tokenEconomy.prunerActive') : t('tokenEconomy.prunerInactive') }}</span>
      </div>
    </div>
    </div>
  </div>
</template>
