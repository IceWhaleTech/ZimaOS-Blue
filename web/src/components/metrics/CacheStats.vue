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
  <div class="space-y-4">
    <!-- Header with title and refresh -->
    <div class="flex items-center justify-between">
      <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('cache.proxyCache') }}</h3>
      <button
        :disabled="loading"
        class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600 rounded-lg disabled:opacity-50 transition-colors"
        @click="fetchStats"
      >
        {{ t('common.refresh') }}
      </button>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Cache Entries Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.entries') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats?.requests ?? '-' }}
          </p>
        </div>
        <div class="p-3 bg-indigo-100 dark:bg-indigo-900/30 rounded-full">
          <svg class="w-6 h-6 text-indigo-600 dark:text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('cache.tokensSaved') }}: {{ formatTokens(stats?.total_cache_read_tokens ?? 0) }}</span>
      </div>
    </div>

    <!-- Hit Rate Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.hitRate') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ stats ? hitRate.toFixed(1) + '%' : '-' }}
          </p>
        </div>
        <div class="p-3 bg-green-100 dark:bg-green-900/30 rounded-full">
          <svg class="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm">
        <span class="text-green-500">{{ stats?.cache_hits ?? 0 }}</span>
        <span class="ml-1 text-gray-500 dark:text-gray-400">{{ t('cache.hits') }}</span>
        <span class="mx-2 text-gray-400">|</span>
        <span class="text-orange-500">{{ stats?.cache_misses ?? 0 }}</span>
        <span class="ml-1 text-gray-500 dark:text-gray-400">{{ t('cache.misses') }}</span>
      </div>
    </div>

    <!-- Tokens Saved Card (Pruner) -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.tokensSaved') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ tokensSaved > 0 ? formatTokens(tokensSaved) : '-' }}
          </p>
        </div>
        <div class="p-3 bg-purple-100 dark:bg-purple-900/30 rounded-full">
          <svg class="w-6 h-6 text-purple-600 dark:text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('cache.prunerCompression') }}: {{ prunerStats?.stats?.pruned_requests ? Math.round((1 - prunerStats.stats.avg_compression_rate) * 100) + '%' : '-' }}</span>
      </div>
    </div>

    <!-- Status Card -->
    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 shadow">
      <div class="flex items-center justify-between">
        <div>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('cache.status') }}</p>
          <p class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ cacheEnabled ? t('cache.enabled') : t('cache.disabled') }}
          </p>
        </div>
        <div
          :class="[
            'p-3 rounded-full',
            cacheEnabled
              ? 'bg-green-100 dark:bg-green-900/30'
              : 'bg-gray-100 dark:bg-gray-700'
          ]"
        >
          <svg
            :class="[
              'w-6 h-6',
              cacheEnabled
                ? 'text-green-600 dark:text-green-400'
                : 'text-gray-400'
            ]" fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
          </svg>
        </div>
      </div>
      <div class="mt-2 flex items-center text-sm text-gray-500 dark:text-gray-400">
        <span>{{ t('cache.hitRate') }}: {{ stats ? Math.round((stats.reuse_ratio ?? 0) * 100) + '%' : '-' }}</span>
      </div>
    </div>
    </div>
  </div>
</template>
