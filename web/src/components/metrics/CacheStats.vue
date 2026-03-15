<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyCacheApi, type PromptCacheStats, type PrunerStats } from '@/api/proxyCache'

const { t } = useI18n()

const stats = ref<PromptCacheStats | null>(null)
const prunerStats = ref<PrunerStats | null>(null)
const cacheEnabled = ref<boolean | null>(null)
const loading = ref(false)

const hitRate = computed(() => {
  if (!stats.value) return 0
  const total = stats.value.cache_hits + stats.value.cache_misses
  if (total === 0) return 0
  return (stats.value.cache_hits / total) * 100
})

const tokensSaved = computed(() => prunerStats.value?.stats?.tokens_saved ?? 0)

function formatTokens(n: number): string {
  return n.toLocaleString()
}

async function fetchStats() {
  loading.value = true
  try {
    const [cacheRes, configRes, prunerRes] = await Promise.all([
      proxyCacheApi.getPromptCacheStats(),
      proxyCacheApi.getPromptCacheConfig().catch(() => null),
      proxyCacheApi.getPrunerStats().catch(() => null),
    ])
    stats.value = cacheRes.data
    cacheEnabled.value = configRes?.data.enabled ?? null
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
  <div class="dashboard-card-surface p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy">
          <p class="dashboard-card-label">{{ t('cache.proxyCache') }}</p>
          <p class="dashboard-card-subtitle mt-2">
            {{ cacheEnabled ? t('cache.enabled') : t('cache.disabled') }}
          </p>
        </div>
        <button :disabled="loading" class="dashboard-card-chip" @click="fetchStats">
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="loading" class="dashboard-card-empty">
        <div
          class="animate-spin rounded-full h-6 w-6 border-b-2 border-gray-900 dark:border-white"
        ></div>
      </div>

      <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-3">
        <div class="dashboard-card-subsurface p-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.entries') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats?.requests ?? '-' }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            {{ t('cache.tokensSaved') }}: {{ formatTokens(stats?.total_cache_read_tokens ?? 0) }}
          </p>
        </div>

        <div class="dashboard-card-subsurface p-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.hitRate') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats ? hitRate.toFixed(1) + '%' : '-' }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            <span class="text-green-500">{{ stats?.cache_hits ?? 0 }}</span>
            <span class="mx-1">{{ t('cache.hits') }}</span>
            <span class="text-orange-500">{{ stats?.cache_misses ?? 0 }}</span>
            <span class="ml-1">{{ t('cache.misses') }}</span>
          </p>
        </div>

        <div class="dashboard-card-subsurface p-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.tokensSaved') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
            {{ tokensSaved > 0 ? formatTokens(tokensSaved) : '-' }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            {{ t('cache.prunerCompression') }}:
            {{
              prunerStats?.stats?.pruned_requests
                ? Math.round((1 - prunerStats.stats.avg_compression_rate) * 100) + '%'
                : '-'
            }}
          </p>
        </div>

        <div class="dashboard-card-subsurface p-4">
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.status') }}</p>
          <p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
            {{ cacheEnabled ? t('cache.enabled') : t('cache.disabled') }}
          </p>
          <p class="dashboard-card-footnote mt-2">
            {{ t('cache.hitRate') }}:
            {{ stats ? Math.round((stats.reuse_ratio ?? 0) * 100) + '%' : '-' }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
