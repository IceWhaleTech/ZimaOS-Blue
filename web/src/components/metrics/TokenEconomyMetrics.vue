<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyCacheApi, type PrunerStats, type RoutingStats } from '@/api/proxyCache'
import { settingsApi, type ToolSelectorStats } from '@/api/settings'

const { t } = useI18n()

const prunerStats = ref<PrunerStats | null>(null)
const routingStats = ref<RoutingStats | null>(null)
const toolStats = ref<ToolSelectorStats | null>(null)
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
    const [prunerRes, routingRes, toolRes] = await Promise.all([
      proxyCacheApi.getPrunerStats().catch(() => null),
      proxyCacheApi.getRoutingStats().catch(() => null),
      settingsApi.getToolStats().catch(() => null),
    ])
    if (prunerRes) prunerStats.value = prunerRes.data
    if (routingRes) routingStats.value = routingRes.data
    if (toolRes) toolStats.value = toolRes.data
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
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
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
    </div>
  </div>
</template>
