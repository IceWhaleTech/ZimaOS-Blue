<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  proxyCacheApi,
  type CacheConfig,
  type CacheStats,
  type PrunerConfig,
  type PrunerStats,
} from '@/api/proxyCache'

const emit = defineEmits<{ 'status-change': [msg: string] }>()
const { t } = useI18n()

const loading = ref(true)
const cacheConfig = ref<CacheConfig | null>(null)
const cacheStats = ref<CacheStats | null>(null)
const prunerConfig = ref<PrunerConfig | null>(null)
const prunerStats = ref<PrunerStats | null>(null)
const clearing = ref(false)
const togglingCache = ref(false)
const togglingPruner = ref(false)
const togglingStream = ref(false)

async function fetchAll() {
  loading.value = true
  try {
    const [cfgRes, statsRes, pCfgRes, pStatsRes] = await Promise.all([
      proxyCacheApi.getConfig().catch(() => null),
      proxyCacheApi.getStats().catch(() => null),
      proxyCacheApi.getPrunerConfig().catch(() => null),
      proxyCacheApi.getPrunerStats().catch(() => null),
    ])
    if (cfgRes) cacheConfig.value = cfgRes.data
    if (statsRes) cacheStats.value = statsRes.data
    if (pCfgRes) prunerConfig.value = pCfgRes.data
    if (pStatsRes) prunerStats.value = pStatsRes.data
  } finally {
    loading.value = false
  }
}

async function toggleCache() {
  if (!cacheConfig.value || togglingCache.value) return
  togglingCache.value = true
  try {
    const res = await proxyCacheApi.updateConfig({ enabled: !cacheConfig.value.enabled })
    cacheConfig.value = res.data.config
    emit('status-change', t(cacheConfig.value.enabled ? 'apiProxy.cacheEnabled' : 'apiProxy.cacheDisabled'))
  } finally {
    togglingCache.value = false
  }
}

async function toggleStreamCache() {
  if (!cacheConfig.value || togglingStream.value) return
  togglingStream.value = true
  try {
    const res = await proxyCacheApi.updateConfig({ skip_streaming: !cacheConfig.value.skip_streaming })
    cacheConfig.value = res.data.config
    emit('status-change', t(cacheConfig.value.skip_streaming ? 'apiProxy.streamCacheOff' : 'apiProxy.streamCacheOn'))
  } finally {
    togglingStream.value = false
  }
}

async function togglePruner() {
  if (!prunerConfig.value || togglingPruner.value) return
  togglingPruner.value = true
  try {
    const res = await proxyCacheApi.updatePrunerConfig({ enabled: !prunerConfig.value.enabled })
    prunerConfig.value = res.data.config
    emit('status-change', t(prunerConfig.value.enabled ? 'apiProxy.prunerEnabled' : 'apiProxy.prunerDisabled'))
  } finally {
    togglingPruner.value = false
  }
}

async function clearCache() {
  if (clearing.value) return
  if (!confirm(t('cache.confirmClear'))) return
  clearing.value = true
  try {
    await proxyCacheApi.clearCache()
    await fetchAll()
    emit('status-change', t('apiProxy.cacheCleared'))
  } finally {
    clearing.value = false
  }
}

function formatTTL(seconds: number): string {
  if (!seconds) return '-'
  if (seconds >= 3600) return `${Math.round(seconds / 3600)}h`
  return `${Math.round(seconds / 60)}m`
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

onMounted(fetchAll)
</script>

<template>
  <div class="space-y-6">
    <!-- Loading -->
    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full mx-auto"></div>
      <p class="text-gray-400 mt-2 text-sm">{{ t('common.loading') }}</p>
    </div>

    <template v-else>
      <!-- CC Cache Section -->
      <div class="glass-card p-4">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.cacheTitle') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.cacheDesc') }}</p>
          </div>
          <div class="flex items-center gap-3">
            <button
              :disabled="clearing || !cacheConfig?.enabled"
              class="px-3 py-1.5 text-xs bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              @click="clearCache"
            >
              {{ clearing ? t('common.loading') : t('cache.clear') }}
            </button>
          </div>
        </div>

        <div class="space-y-3">
          <!-- Cache enabled toggle -->
          <div class="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
            <div>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('apiProxy.cacheSwitch') }}</span>
            </div>
            <button
              type="button"
              :disabled="togglingCache"
              :class="[
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                cacheConfig?.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                togglingCache ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
              ]"
              @click="toggleCache"
            >
              <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', cacheConfig?.enabled ? 'translate-x-6' : 'translate-x-1']" />
            </button>
          </div>

          <!-- Stream cache toggle -->
          <div class="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
            <div>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('apiProxy.streamCache') }}</span>
              <p class="text-xs text-gray-400 dark:text-gray-500">{{ t('apiProxy.streamCacheDesc') }}</p>
            </div>
            <button
              type="button"
              :disabled="togglingStream || !cacheConfig?.enabled"
              :class="[
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                cacheConfig && !cacheConfig.skip_streaming ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                (togglingStream || !cacheConfig?.enabled) ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
              ]"
              @click="toggleStreamCache"
            >
              <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', cacheConfig && !cacheConfig.skip_streaming ? 'translate-x-6' : 'translate-x-1']" />
            </button>
          </div>

          <!-- Cache info row -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-1">
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('cache.entries') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ cacheStats?.entries ?? 0 }} / {{ cacheStats?.max_entries ?? 0 }}</p>
            </div>
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('cache.hitRate') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">
                {{ cacheStats ? (cacheStats.hits + cacheStats.misses > 0 ? ((cacheStats.hits / (cacheStats.hits + cacheStats.misses)) * 100).toFixed(1) : '0.0') : '-' }}%
              </p>
            </div>
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">TTL</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ formatTTL(cacheStats?.ttl_seconds ?? 0) }}</p>
            </div>
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('apiProxy.storageType') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ cacheConfig?.storage_type ?? '-' }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Context Pruner Section -->
      <div class="glass-card p-4">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.prunerTitle') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.prunerDesc') }}</p>
          </div>
        </div>

        <div class="space-y-3">
          <!-- Pruner enabled toggle -->
          <div class="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
            <div>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('apiProxy.prunerSwitch') }}</span>
            </div>
            <button
              type="button"
              :disabled="togglingPruner || !prunerConfig"
              :class="[
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                prunerConfig?.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                (togglingPruner || !prunerConfig) ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
              ]"
              @click="togglePruner"
            >
              <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', prunerConfig?.enabled ? 'translate-x-6' : 'translate-x-1']" />
            </button>
          </div>

          <!-- Pruner info row -->
          <div v-if="prunerConfig" class="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-1">
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('apiProxy.backend') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ prunerConfig.backend }}</p>
            </div>
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('apiProxy.threshold') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ prunerConfig.threshold }}</p>
            </div>
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('cache.tokensSaved') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ formatTokens(prunerStats?.stats?.tokens_saved ?? 0) }}</p>
            </div>
            <div class="text-center">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('cache.prunerCompression') }}</p>
              <p class="text-sm font-medium text-gray-900 dark:text-white">
                {{ prunerStats?.stats ? Math.round((1 - prunerStats.stats.avg_compression_rate) * 100) + '%' : '-' }}
              </p>
            </div>
          </div>
          <div v-else class="text-xs text-gray-400 dark:text-gray-500 py-2">
            {{ t('apiProxy.prunerNotAvailable') }}
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
