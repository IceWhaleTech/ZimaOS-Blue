<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyCacheApi, type PrunerStats, type RoutingStats } from '@/api/proxyCache'
import { settingsApi, type ToolSelectorStats } from '@/api/settings'
import api from '@/api/client'

const { t } = useI18n()

const prunerStats = ref<PrunerStats | null>(null)
const routingStats = ref<RoutingStats | null>(null)
const toolStats = ref<ToolSelectorStats | null>(null)
const mediaCostUSD = ref(0)
const mediaCalls = ref(0)
const loading = ref(false)

const prunerTokensSaved = computed(() => prunerStats.value?.stats?.tokens_saved ?? 0)

const toolTokensSaved = computed(() => toolStats.value?.tokens_saved ?? 0)

const totalTokensSaved = computed(() => {
  return prunerTokensSaved.value + toolTokensSaved.value + (routingStats.value?.tokens_routed ?? 0)
})

const costSaved = computed(() => {
  // Pruner: saved tokens are input tokens (context reduction), use $3/M
  const prunerCost = (prunerTokensSaved.value / 1_000_000) * 3.0
  // Tool selection: saved tokens are input tokens (tool definitions), use $3/M
  const toolCost = (toolTokensSaved.value / 1_000_000) * 3.0
  // Router: already computed as USD on backend
  const routerCost = routingStats.value?.cost_saved_usd ?? 0
  return prunerCost + toolCost + routerCost
})

function formatTokens(n: number): string {
  return n.toLocaleString()
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
    const [prunerRes, routingRes, toolRes, mediaRes] = await Promise.all([
      proxyCacheApi.getPrunerStats().catch(() => null),
      proxyCacheApi.getRoutingStats().catch(() => null),
      settingsApi.getToolStats().catch(() => null),
      api.get<{ total_cost_usd: number; succeeded: number }>('/media/stats').catch(() => null),
    ])
    if (prunerRes) prunerStats.value = prunerRes.data
    if (routingRes) routingStats.value = routingRes.data
    if (toolRes) toolStats.value = toolRes.data
    if (mediaRes?.data) {
      mediaCostUSD.value = mediaRes.data.total_cost_usd ?? 0
      mediaCalls.value = mediaRes.data.succeeded ?? 0
    }
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

    <!-- Pruner Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.pruner') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ prunerStats?.stats?.pruned_requests ? Math.round((1 - prunerStats.stats.avg_compression_rate) * 100) + '%' : '-' }}
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

    <!-- Smart Tools Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.smartTools') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ toolTokensSaved > 0 ? formatTokens(toolTokensSaved) : '-' }}
          </p>
        </div>
        <div class="p-3 bg-cyan-100 dark:bg-cyan-900/30 rounded-full">
          <svg class="w-6 h-6 text-cyan-600 dark:text-cyan-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ toolStats?.requests ? toolStats.requests + ' ' + t('tokenEconomy.smartToolsReqs') : t('tokenEconomy.smartToolsInactive') }}</span>
      </div>
    </div>

    <!-- Media Generation Cost Card (shown only when there's data) -->
    <div v-if="mediaCalls > 0" class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('tokenEconomy.mediaGenCost') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ formatCost(mediaCostUSD) }}
          </p>
        </div>
        <div class="p-3 bg-pink-100 dark:bg-pink-900/30 rounded-full">
          <svg class="w-6 h-6 text-pink-600 dark:text-pink-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ mediaCalls }} {{ t('tokenEconomy.mediaGenReqs') }}</span>
      </div>
    </div>
    </div>
  </div>
</template>
