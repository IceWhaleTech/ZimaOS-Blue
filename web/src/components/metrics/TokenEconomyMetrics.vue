<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  proxyCacheApi,
  type PrunerStats,
  type RoutingStats,
  type ContextStats,
} from '@/api/proxyCache'

const { t } = useI18n()

const prunerStats = ref<PrunerStats | null>(null)
const routingStats = ref<RoutingStats | null>(null)
const contextStats = ref<ContextStats | null>(null)
const loading = ref(false)

const prunerTokensSaved = computed(() => prunerStats.value?.stats?.tokens_saved ?? 0)
const memoryTokensSaved = computed(
  () => contextStats.value?.memory_recall?.estimated_saved_tokens ?? 0
)

const totalTokensSaved = computed(() => {
  return (
    prunerTokensSaved.value +
    memoryTokensSaved.value +
    (routingStats.value?.tokens_routed ?? 0)
  )
})

const prunerCostSaved = computed(() => (prunerTokensSaved.value / 1_000_000) * 3.0)
const memoryCostSaved = computed(() => (memoryTokensSaved.value / 1_000_000) * 3.0)

const costSaved = computed(() => {
  const prunerCost = prunerCostSaved.value
  const memoryCost = memoryCostSaved.value
  const routerCost = routingStats.value?.cost_saved_usd ?? 0
  return prunerCost + memoryCost + routerCost
})

const detailRows = computed(() => {
  const prunerTotal = prunerStats.value?.stats?.total_requests ?? 0
  const prunerPruned = prunerStats.value?.stats?.pruned_requests ?? 0
  const prunerCompression = Math.round((prunerStats.value?.stats?.avg_compression_rate ?? 0) * 100)
  const routedRequests = routingStats.value?.routed_requests ?? 0
  const memoryTotal = contextStats.value?.memory_recall?.total ?? 0
  const memorySkipped = contextStats.value?.memory_recall?.skipped ?? 0
  const mode = contextStats.value?.memory_recall_mode ?? 'balanced'
  const minScore = contextStats.value?.memory_recall_min_score ?? 0
  const maxResults = contextStats.value?.memory_recall_limits?.max_results ?? 0

  return [
    {
      key: 'pruner',
      label: t('tokenEconomy.prunerLabel'),
      tokens: prunerTokensSaved.value,
      cost: prunerCostSaved.value,
      meta: t('tokenEconomy.prunerMeta', { pruned: prunerPruned, total: prunerTotal }),
      extra: `${t('tokenEconomy.compression')}: ${prunerCompression}%`,
    },
    {
      key: 'routing',
      label: t('tokenEconomy.routingLabel'),
      tokens: routingStats.value?.tokens_routed ?? 0,
      cost: routingStats.value?.cost_saved_usd ?? 0,
      meta: t('tokenEconomy.routingMeta', { requests: routedRequests }),
      extra: '',
    },
    {
      key: 'memory',
      label: t('tokenEconomy.memoryLabel'),
      tokens: memoryTokensSaved.value,
      cost: memoryCostSaved.value,
      meta: t('tokenEconomy.memoryMeta', { skipped: memorySkipped, total: memoryTotal }),
      extra: `${t('tokenEconomy.memoryMode', { mode: t(`tokenEconomy.modes.${mode}`) })} · min=${minScore.toFixed(2)} · max=${maxResults}`,
    },
  ]
})

const hasDetailData = computed(() => detailRows.value.some((row) => row.tokens > 0 || row.cost > 0))

function formatTokens(n: number): string {
  return n.toLocaleString()
}

function formatCost(n: number): string {
  if (n >= 100) return '$' + Math.round(n)
  if (n >= 1) return '$' + n.toFixed(2)
  if (n >= 0.01) return '$' + n.toFixed(2)
  return '$0.00'
}

function contribution(rowTokens: number): string {
  if (totalTokensSaved.value <= 0) return '0.0'
  return ((rowTokens / totalTokensSaved.value) * 100).toFixed(1)
}

async function fetchStats() {
  loading.value = true
  try {
    const [prunerRes, routingRes, contextRes] = await Promise.all([
      proxyCacheApi.getPrunerStats().catch(() => null),
      proxyCacheApi.getRoutingStats().catch(() => null),
      proxyCacheApi.getContextStats().catch(() => null),
    ])
    if (prunerRes) prunerStats.value = prunerRes.data
    if (routingRes) routingStats.value = routingRes.data
    if (contextRes) contextStats.value = contextRes.data
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
  <div class="dashboard-card-surface p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy">
          <p class="dashboard-card-label">{{ t('tokenEconomy.tokensSaved') }}</p>
          <p class="dashboard-card-subtitle mt-2">Savings breakdown</p>
        </div>
        <span class="dashboard-card-chip">{{
          totalTokensSaved > 0 ? formatTokens(totalTokensSaved) : '-'
        }}</span>
      </div>

      <div v-if="loading" class="dashboard-card-empty">
        <div
          class="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900 dark:border-white"
        ></div>
      </div>

      <template v-else>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div class="dashboard-card-subsurface p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('tokenEconomy.costSaved') }}
            </p>
            <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
              {{ costSaved > 0 ? formatCost(costSaved) : '-' }}
            </p>
            <p class="dashboard-card-footnote mt-2">{{ t('tokenEconomy.estimated') }}</p>
          </div>

          <div class="dashboard-card-subsurface p-4">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('tokenEconomy.tokensSaved') }}
            </p>
            <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
              {{ totalTokensSaved > 0 ? formatTokens(totalTokensSaved) : '-' }}
            </p>
            <p class="dashboard-card-footnote mt-2">{{ t('tokenEconomy.combined') }}</p>
          </div>
        </div>

        <div class="dashboard-card-subsurface p-3">
          <div class="mb-3 flex items-baseline justify-between gap-3">
            <p class="text-sm font-medium text-gray-700 dark:text-gray-200">
              {{ t('tokenEconomy.breakdown') }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('tokenEconomy.breakdownHint') }}
            </p>
          </div>

          <div v-if="!hasDetailData" class="py-3 text-sm text-gray-500 dark:text-gray-400">
            {{ t('tokenEconomy.noSavingsYet') }}
          </div>

          <div v-else class="space-y-3">
            <div v-for="row in detailRows" :key="row.key" class="dashboard-card-subsurface p-3">
              <div class="flex items-center justify-between gap-2">
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ row.label }}</p>
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  {{ contribution(row.tokens) }}%
                </p>
              </div>
              <div class="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                <span class="font-mono text-gray-900 dark:text-white"
                  >{{ formatTokens(row.tokens) }} {{ t('common.totalTokens') }}</span
                >
                <span class="text-green-600 dark:text-green-400"
                  >{{ t('tokenEconomy.estCost') }} {{ formatCost(row.cost) }}</span
                >
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ row.meta }}</p>
              <p v-if="row.extra" class="text-xs text-gray-500 dark:text-gray-400">
                {{ row.extra }}
              </p>
              <div class="mt-2 dashboard-card-progress">
                <span
                  :style="{
                    width: `${totalTokensSaved > 0 ? Math.max((row.tokens / totalTokensSaved) * 100, 2) : 0}%`,
                    background: 'linear-gradient(90deg, #16a34a 0%, #4ade80 100%)',
                  }"
                ></span>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
