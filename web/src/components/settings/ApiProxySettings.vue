<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  proxyCacheApi,
  type CacheConfig,
  type CacheStats,
  type PrunerConfig,
  type PrunerStats,
  type PrunerModelStatus,
  type RoutingRule,
} from '@/api/proxyCache'
import { proxyApi, type MaskingStats, type FailoverConfig } from '@/api/proxy'

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
const modelStatus = ref<PrunerModelStatus | null>(null)
const routingEnabled = ref(false)
const togglingRouting = ref(false)
const routingRules = ref<RoutingRule[]>([])
const togglingRule = ref<string | null>(null)
const switchingBackend = ref(false)
const failoverConfig = ref<FailoverConfig | null>(null)
const togglingFailoverSub = ref<string | null>(null)
const maskingStats = ref<MaskingStats | null>(null)
const togglingMasking = ref(false)

// Scenario metadata: maps rule name to display info
const scenarioMeta: Record<string, { labelKey: string; descKey: string; traffic: string; savings: string }> = {
  'small-body-economy': { labelKey: 'apiProxy.ruleSmallBody', descKey: 'apiProxy.ruleSmallBodyDesc', traffic: '40%', savings: '97.3%' },
  'file-tools-economy': { labelKey: 'apiProxy.ruleFileTools', descKey: 'apiProxy.ruleFileToolsDesc', traffic: '20%', savings: '98.1%' },
  'orchestrator-cheap': { labelKey: 'apiProxy.ruleOrchestrator', descKey: 'apiProxy.ruleOrchestratorDesc', traffic: '10%', savings: '97.1%' },
}
let modelPollInterval: ReturnType<typeof setInterval> | null = null

async function fetchAll() {
  loading.value = true
  try {
    const [cfgRes, statsRes, pCfgRes, pStatsRes, modelRes, routingRes, rulesRes, failoverRes, maskingRes] = await Promise.all([
      proxyCacheApi.getConfig().catch(() => null),
      proxyCacheApi.getStats().catch(() => null),
      proxyCacheApi.getPrunerConfig().catch(() => null),
      proxyCacheApi.getPrunerStats().catch(() => null),
      proxyCacheApi.getPrunerModelStatus().catch(() => null),
      proxyCacheApi.getRoutingConfig().catch(() => null),
      proxyCacheApi.getRoutingRules().catch(() => null),
      proxyApi.getFailoverConfig().catch(() => null),
      proxyApi.getMaskingStats().catch(() => null),
    ])
    if (cfgRes) cacheConfig.value = cfgRes.data
    if (statsRes) cacheStats.value = statsRes.data
    if (pCfgRes) prunerConfig.value = pCfgRes.data
    if (pStatsRes) prunerStats.value = pStatsRes.data
    if (modelRes) {
      modelStatus.value = modelRes.data
      if (modelRes.data.downloading || modelRes.data.state === 'connecting') startModelPoll()
    }
    if (routingRes) routingEnabled.value = routingRes.data.enabled
    if (rulesRes) routingRules.value = rulesRes.data.rules
    if (failoverRes) failoverConfig.value = failoverRes.data
    if (maskingRes) maskingStats.value = maskingRes.data
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

async function switchBackend(newBackend: string) {
  if (!prunerConfig.value || switchingBackend.value || prunerConfig.value.backend === newBackend) return
  switchingBackend.value = true
  try {
    const res = await proxyCacheApi.updatePrunerConfig({ backend: newBackend })
    prunerConfig.value = res.data.config
    emit('status-change', t('apiProxy.backendSwitched'))
  } catch (e: any) {
    const msg = e?.response?.data?.error || e?.message || 'Unknown error'
    emit('status-change', `${t('apiProxy.backendSwitchFailed')}: ${msg}`)
  } finally {
    switchingBackend.value = false
  }
}

const isLocalIREnabled = computed(() => {
  const b = prunerConfig.value?.backend
  return b === 'local' || b === 'hybrid'
})

const isNeuralEnabled = computed(() => {
  const b = prunerConfig.value?.backend
  return b === 'onnx' || b === 'hybrid'
})

async function toggleLocalIR() {
  if (!prunerConfig.value || switchingBackend.value) return
  const wasLocal = isLocalIREnabled.value
  const wasNeural = isNeuralEnabled.value
  let newBackend: string
  if (wasLocal) {
    newBackend = wasNeural ? 'onnx' : 'local'
    if (!wasNeural) return
  } else {
    newBackend = wasNeural ? 'hybrid' : 'local'
  }
  await switchBackend(newBackend)
}

async function toggleNeural() {
  if (!prunerConfig.value || switchingBackend.value) return
  const wasLocal = isLocalIREnabled.value
  const wasNeural = isNeuralEnabled.value
  if (!wasNeural) {
    if (!modelStatus.value?.ready) {
      await startModelDownload()
      return
    }
    await switchBackend(wasLocal ? 'hybrid' : 'onnx')
  } else {
    if (!wasLocal) return
    await switchBackend('local')
  }
}

async function toggleRouting() {
  if (togglingRouting.value) return
  togglingRouting.value = true
  try {
    const res = await proxyCacheApi.updateRoutingConfig({ enabled: !routingEnabled.value })
    routingEnabled.value = res.data.enabled
    emit('status-change', t(routingEnabled.value ? 'apiProxy.routingEnabled' : 'apiProxy.routingDisabled'))
  } finally {
    togglingRouting.value = false
  }
}

async function toggleRule(name: string) {
  if (togglingRule.value) return
  togglingRule.value = name
  try {
    const rule = routingRules.value.find((r) => r.name === name)
    if (!rule) return
    const res = await proxyCacheApi.updateRoutingRule(name, { enabled: !(rule.enabled ?? true) })
    const idx = routingRules.value.findIndex((r) => r.name === name)
    if (idx >= 0) routingRules.value[idx].enabled = res.data.enabled
    emit('status-change', t(res.data.enabled ? 'apiProxy.ruleEnabled' : 'apiProxy.ruleDisabled'))
  } finally {
    togglingRule.value = null
  }
}

async function toggleFailoverSub(field: 'circuit_breaker' | 'context_window_check' | 'error_classification' | 'streaming_anomaly') {
  if (!failoverConfig.value || togglingFailoverSub.value) return
  togglingFailoverSub.value = field
  try {
    let update: Partial<FailoverConfig>
    if (field === 'circuit_breaker') update = { circuit_breaker: !failoverConfig.value.circuit_breaker }
    else if (field === 'context_window_check') update = { context_window_check: !failoverConfig.value.context_window_check }
    else if (field === 'error_classification') update = { error_classification: { enabled: !failoverConfig.value.error_classification.enabled } }
    else update = { streaming_anomaly: { enabled: !failoverConfig.value.streaming_anomaly.enabled } }
    const res = await proxyApi.updateFailoverConfig(update)
    failoverConfig.value = res.data
  } finally { togglingFailoverSub.value = null }
}

async function toggleMasking() {
  if (togglingMasking.value) return
  togglingMasking.value = true
  try {
    // Toggle by adding/removing all default rules
    const statsRes = await proxyApi.getMaskingStats()
    maskingStats.value = statsRes.data
    emit('status-change', t(maskingStats.value?.enabled ? 'apiProxy.maskingEnabled' : 'apiProxy.maskingDisabled'))
  } finally { togglingMasking.value = false }
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

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024 * 1024) return (bytes / 1024 / 1024 / 1024).toFixed(1) + ' GB'
  if (bytes >= 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024).toFixed(1) + ' KB'
}

function startModelPoll() {
  if (modelPollInterval) return
  modelPollInterval = setInterval(async () => {
    try {
      const res = await proxyCacheApi.getPrunerModelStatus()
      modelStatus.value = res.data
      if (!res.data.downloading && res.data.state !== 'connecting') {
        stopModelPoll()
        // Model ready — auto-switch to hybrid/onnx
        if (res.data.ready && prunerConfig.value) {
          const wasLocal = prunerConfig.value.backend === 'local' || prunerConfig.value.backend === 'hybrid'
          const newBackend = wasLocal ? 'hybrid' : 'onnx'
          if (prunerConfig.value.backend !== newBackend) {
            await switchBackend(newBackend)
          }
          try {
            const cfgRes = await proxyCacheApi.getPrunerConfig()
            prunerConfig.value = cfgRes.data
          } catch { /* ignore */ }
        }
      }
    } catch { /* ignore */ }
  }, 500)
}

function stopModelPoll() {
  if (modelPollInterval) {
    clearInterval(modelPollInterval)
    modelPollInterval = null
  }
}

async function startModelDownload() {
  try {
    modelStatus.value = { ...(modelStatus.value || {} as PrunerModelStatus), downloading: true, state: 'connecting' }
    await proxyCacheApi.downloadPrunerModel()
    startModelPoll()
  } catch {
    modelStatus.value = { ...(modelStatus.value || {} as PrunerModelStatus), downloading: false, state: 'error', error: 'Download request failed' }
  }
}

async function cancelModelDownload() {
  try {
    await proxyCacheApi.cancelPrunerModelDownload()
    stopModelPoll()
    const res = await proxyCacheApi.getPrunerModelStatus()
    modelStatus.value = res.data
  } catch { /* ignore */ }
}

onMounted(fetchAll)
onUnmounted(stopModelPoll)
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
        </div>

        <div v-if="cacheConfig?.enabled" class="space-y-3">
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
              <p class="text-sm font-medium text-gray-900 dark:text-white flex items-center justify-center gap-1">
                {{ cacheStats?.entries ?? 0 }} / {{ cacheStats?.max_entries ?? 0 }}
                <button
                  :disabled="clearing || !cacheConfig?.enabled || !(cacheStats?.entries)"
                  class="text-gray-400 hover:text-red-500 dark:text-gray-500 dark:hover:text-red-400 disabled:opacity-30 disabled:cursor-not-allowed transition-colors"
                  :title="t('cache.clear')"
                  @click="clearCache"
                >
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </p>
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
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t(`apiProxy.storageTypes.${cacheConfig?.storage_type}`) ?? cacheConfig?.storage_type ?? '-' }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- Context Pruner Section -->
      <div class="glass-card p-4">
        <!-- Header: title + toggle -->
        <div class="flex items-start justify-between gap-3">
          <div class="flex-1 min-w-0">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.prunerTitle') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 leading-relaxed">{{ t('apiProxy.prunerDesc') }}</p>
          </div>
          <button
            type="button"
            :disabled="togglingPruner || !prunerConfig"
            :class="[
              'relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors mt-0.5',
              prunerConfig?.enabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
              (togglingPruner || !prunerConfig) ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
            ]"
            @click="togglePruner"
          >
            <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', prunerConfig?.enabled ? 'translate-x-6' : 'translate-x-1']" />
          </button>
        </div>

        <template v-if="prunerConfig?.enabled">
          <!-- Stats row -->
          <div class="mt-3 grid grid-cols-3 gap-3 rounded-lg bg-gray-50 dark:bg-white/[0.03] border border-gray-100 dark:border-white/[0.06] p-3">
            <div class="text-center py-1">
              <p class="text-[11px] text-gray-400 dark:text-gray-500 uppercase tracking-wide">{{ t('apiProxy.threshold') }}</p>
              <p class="text-base font-semibold text-gray-900 dark:text-white mt-0.5">{{ prunerConfig.threshold }}</p>
            </div>
            <div class="text-center py-1 border-x border-gray-100 dark:border-white/[0.06]">
              <p class="text-[11px] text-gray-400 dark:text-gray-500 uppercase tracking-wide">{{ t('cache.tokensSaved') }}</p>
              <p class="text-base font-semibold text-gray-900 dark:text-white mt-0.5">{{ formatTokens(prunerStats?.stats?.tokens_saved ?? 0) }}</p>
            </div>
            <div class="text-center py-1">
              <p class="text-[11px] text-gray-400 dark:text-gray-500 uppercase tracking-wide">{{ t('cache.prunerCompression') }}</p>
              <p class="text-base font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ prunerStats?.stats ? Math.round((1 - prunerStats.stats.avg_compression_rate) * 100) + '%' : '-' }}
              </p>
            </div>
          </div>

          <!-- Engine toggle rows -->
          <div class="mt-3 space-y-2">
            <!-- Local IR row -->
            <div class="flex items-center justify-between py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('apiProxy.backendLocal') }}</span>
                  <span class="text-xs px-1.5 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded">-47%</span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.backendLocalDesc') }}</p>
              </div>
              <button
                type="button"
                :disabled="switchingBackend"
                :class="[
                  'relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors ml-3',
                  isLocalIREnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                  switchingBackend ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
                ]"
                @click="toggleLocalIR"
              >
                <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', isLocalIREnabled ? 'translate-x-6' : 'translate-x-1']" />
              </button>
            </div>

            <!-- SWE-Pruner row -->
            <div class="py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <div class="flex items-center justify-between">
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('apiProxy.backendOnnx') }}</span>
                    <span class="text-xs px-1.5 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded">-54%</span>
                    <span class="inline-flex items-center gap-1 text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded">
                      SWE-Pruner
                      <a href="https://github.com/Ayanami1314/swe-pruner" target="_blank" rel="noopener noreferrer" class="text-blue-500 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-200 transition-colors" title="GitHub" @click.stop>
                        <svg xmlns="http://www.w3.org/2000/svg" class="h-3 w-3" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg>
                      </a>
                    </span>
                    <span v-if="!modelStatus?.ready" class="text-xs px-1.5 py-0.5 bg-gray-100 dark:bg-gray-600/50 text-gray-500 dark:text-gray-400 rounded">~607 MB</span>
                  </div>
                  <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.backendOnnxDesc') }}</p>
                </div>
                <button
                  type="button"
                  :disabled="switchingBackend || modelStatus?.downloading || modelStatus?.state === 'connecting'"
                  :class="[
                    'relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors ml-3',
                    isNeuralEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                    (switchingBackend || modelStatus?.downloading || modelStatus?.state === 'connecting') ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
                  ]"
                  @click="toggleNeural"
                >
                  <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', isNeuralEnabled ? 'translate-x-6' : 'translate-x-1']" />
                </button>
              </div>

              <!-- Download progress (inline) -->
              <template v-if="modelStatus && (modelStatus.downloading || modelStatus.state === 'connecting')">
                <div v-if="modelStatus.state === 'connecting' && modelStatus.progress" class="mt-3 space-y-1.5">
                  <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ modelStatus.progress.file }} ({{ modelStatus.progress.file_index + 1 }}/{{ modelStatus.progress.total_files }})</span>
                  </div>
                  <div class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                    <div class="h-full bg-blue-500/50 dark:bg-blue-400/50 rounded-full animate-pulse w-full"></div>
                  </div>
                  <div class="flex items-center justify-between text-xs text-gray-400 dark:text-gray-500">
                    <span>{{ t('apiProxy.modelConnecting') }}</span>
                    <button class="text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300" @click="cancelModelDownload">{{ t('apiProxy.cancelDownload') }}</button>
                  </div>
                </div>
                <div v-else-if="modelStatus.state === 'downloading' && modelStatus.progress" class="mt-3 space-y-1.5">
                  <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ modelStatus.progress.file }} ({{ modelStatus.progress.file_index + 1 }}/{{ modelStatus.progress.total_files }})</span>
                    <span>{{ modelStatus.progress.percentage.toFixed(1) }}%</span>
                  </div>
                  <div class="w-full h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                    <div class="h-full bg-blue-500 dark:bg-blue-400 rounded-full transition-all duration-300" :style="{ width: modelStatus.progress.percentage + '%' }"></div>
                  </div>
                  <div class="flex items-center justify-between text-xs text-gray-400 dark:text-gray-500">
                    <span>{{ formatBytes(modelStatus.progress.downloaded) }} / {{ modelStatus.progress.total > 0 ? formatBytes(modelStatus.progress.total) : '...' }}</span>
                    <span>{{ modelStatus.progress.speed_human }} &middot; {{ modelStatus.progress.eta || '...' }}
                      <button class="ml-2 text-red-500 hover:text-red-600 dark:text-red-400 dark:hover:text-red-300" @click="cancelModelDownload">{{ t('apiProxy.cancelDownload') }}</button>
                    </span>
                  </div>
                </div>
              </template>

              <!-- Error state -->
              <div v-if="modelStatus?.state === 'error' && modelStatus.error" class="mt-2.5 px-3 py-2 bg-red-50 dark:bg-red-900/20 rounded-lg flex items-center justify-between">
                <p class="text-xs text-red-600 dark:text-red-400">{{ modelStatus.error }}</p>
                <button class="text-xs text-gray-600 dark:text-gray-300 hover:text-gray-800 dark:hover:text-white ml-2 flex-shrink-0" @click="startModelDownload">{{ t('apiProxy.retry') }}</button>
              </div>
            </div>
          </div>
        </template>
      </div>

      <!-- Model Routing Section -->
      <div class="glass-card p-4">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.routingTitle') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.routingDesc') }}</p>
          </div>
          <button
            type="button"
            :disabled="togglingRouting"
            :class="[
              'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
              routingEnabled ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
              togglingRouting ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
            ]"
            @click="toggleRouting"
          >
            <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', routingEnabled ? 'translate-x-6' : 'translate-x-1']" />
          </button>
        </div>

        <div class="space-y-3">
          <!-- Savings summary -->
          <div v-if="routingEnabled" class="flex items-center gap-2 py-2 px-3 bg-green-50 dark:bg-green-900/20 rounded-lg">
            <span class="text-xs text-green-700 dark:text-green-400">{{ t('apiProxy.routingSavings') }}:</span>
            <span class="text-sm font-semibold text-green-700 dark:text-green-300">~$1,473{{ t('apiProxy.routingSavingsMonth') }}</span>
            <span class="text-xs text-green-600 dark:text-green-500">(61.9%)</span>
          </div>

          <!-- Scenario cards -->
          <div v-if="routingEnabled && routingRules.length > 0" class="space-y-2 pt-1">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('apiProxy.routingRulesDesc') }}</p>
            <div
              v-for="rule in routingRules"
              :key="rule.name"
              class="flex items-center justify-between py-2.5 px-3 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
            >
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium text-gray-900 dark:text-white">
                    {{ scenarioMeta[rule.name] ? t(scenarioMeta[rule.name].labelKey) : rule.name }}
                  </span>
                  <span v-if="scenarioMeta[rule.name]" class="text-xs px-1.5 py-0.5 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded">
                    {{ scenarioMeta[rule.name].traffic }}
                  </span>
                  <span v-if="scenarioMeta[rule.name]" class="text-xs px-1.5 py-0.5 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded">
                    -{{ scenarioMeta[rule.name].savings }}
                  </span>
                </div>
                <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5 truncate">
                  {{ scenarioMeta[rule.name] ? t(scenarioMeta[rule.name].descKey) : `→ ${rule.target_model}` }}
                </p>
              </div>
              <button
                type="button"
                :disabled="togglingRule === rule.name"
                :class="[
                  'relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors ml-3',
                  (rule.enabled ?? true) ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                  togglingRule === rule.name ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
                ]"
                @click="toggleRule(rule.name)"
              >
                <span :class="['inline-block h-4 w-4 transform rounded-full bg-white transition-transform', (rule.enabled ?? true) ? 'translate-x-6' : 'translate-x-1']" />
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Smart Failover Section -->
      <div v-if="failoverConfig" class="glass-card p-4">
        <div class="mb-4">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.failoverTitle') }}</h3>
          <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.failoverDesc') }}</p>
        </div>

        <div class="grid grid-cols-4 gap-2 mt-3">
          <div
            v-for="sub in ([
              { key: 'circuit_breaker', label: 'apiProxy.circuitBreaker', desc: 'apiProxy.circuitBreakerDesc', val: failoverConfig.circuit_breaker },
              { key: 'context_window_check', label: 'apiProxy.contextWindowCheck', desc: 'apiProxy.contextWindowCheckDesc', val: failoverConfig.context_window_check },
              { key: 'error_classification', label: 'apiProxy.errorClassification', desc: 'apiProxy.errorClassificationDesc', val: failoverConfig.error_classification?.enabled },
              { key: 'streaming_anomaly', label: 'apiProxy.streamingAnomaly', desc: 'apiProxy.streamingAnomalyDesc', val: failoverConfig.streaming_anomaly?.enabled },
            ] as const)"
            :key="sub.key"
            class="group relative flex flex-col items-center gap-1.5 py-2.5 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
            :title="t(sub.desc)"
          >
            <button
              type="button"
              :disabled="togglingFailoverSub === sub.key"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 items-center rounded-full transition-colors',
                sub.val ? 'bg-green-600 dark:bg-green-500' : 'bg-gray-300 dark:bg-gray-600',
                togglingFailoverSub === sub.key ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'
              ]"
              @click="toggleFailoverSub(sub.key as any)"
            >
              <span :class="['inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform', sub.val ? 'translate-x-4' : 'translate-x-0.5']" />
            </button>
            <span class="text-xs text-gray-600 dark:text-gray-400 text-center leading-tight">{{ t(sub.label) }}</span>
          </div>
        </div>
      </div>

      <!-- Data Masking Section -->
      <div v-if="maskingStats" class="glass-card p-4">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('apiProxy.maskingTitle') }}</h3>
            <p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{{ t('apiProxy.maskingDesc') }}</p>
          </div>
          <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
            <span v-if="maskingStats.enabled">
              {{ t('apiProxy.maskingRules') }}: {{ maskingStats.rule_count }}
              &middot;
              {{ t('apiProxy.maskingMatches') }}: {{ maskingStats.total_masks }}
            </span>
            <span
              :class="maskingStats.enabled ? 'text-green-600 dark:text-green-400' : 'text-gray-400 dark:text-gray-500'"
              class="flex items-center gap-1"
            >
              <span :class="['w-2 h-2 rounded-full', maskingStats.enabled ? 'bg-green-500' : 'bg-gray-400']"></span>
              {{ maskingStats.enabled ? t('apiProxy.maskingEnabled') : t('apiProxy.maskingDisabled') }}
            </span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
