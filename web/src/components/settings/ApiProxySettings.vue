<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
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
const routingEnabled = ref(true)
const togglingRouting = ref(false)
const routingRules = ref<RoutingRule[]>([])
const togglingRule = ref<string | null>(null)
const switchingBackend = ref(false)

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
    const [cfgRes, statsRes, pCfgRes, pStatsRes, modelRes, routingRes, rulesRes] = await Promise.all([
      proxyCacheApi.getConfig().catch(() => null),
      proxyCacheApi.getStats().catch(() => null),
      proxyCacheApi.getPrunerConfig().catch(() => null),
      proxyCacheApi.getPrunerStats().catch(() => null),
      proxyCacheApi.getPrunerModelStatus().catch(() => null),
      proxyCacheApi.getRoutingConfig().catch(() => null),
      proxyCacheApi.getRoutingRules().catch(() => null),
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
  } finally {
    switchingBackend.value = false
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
    await proxyCacheApi.downloadPrunerModel()
    startModelPoll()
  } catch { /* ignore */ }
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

        <div class="space-y-3">
          <!-- Backend selector -->
          <div v-if="prunerConfig?.enabled" class="flex items-center justify-between py-2 border-b border-gray-100 dark:border-gray-700">
            <div>
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('apiProxy.backend') }}</span>
              <p class="text-xs text-gray-400 dark:text-gray-500">{{ prunerConfig.backend === 'onnx' ? t('apiProxy.backendOnnxDesc') : t('apiProxy.backendLocalDesc') }}</p>
            </div>
            <select
              :value="prunerConfig.backend"
              :disabled="switchingBackend"
              class="text-sm bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white border border-gray-200 dark:border-gray-600 rounded-lg px-3 py-1.5 focus:outline-none focus:ring-2 focus:ring-gray-400 disabled:opacity-50"
              @change="switchBackend(($event.target as HTMLSelectElement).value)"
            >
              <option value="local">{{ t('apiProxy.backendLocal') }}</option>
              <option value="onnx" :disabled="!modelStatus?.ready">{{ t('apiProxy.backendOnnx') }}{{ !modelStatus?.ready ? ' (' + t('apiProxy.modelNotDownloaded') + ')' : '' }}</option>
            </select>
          </div>

          <!-- Pruner info row -->
          <div v-if="prunerConfig?.enabled" class="grid grid-cols-3 gap-3 pt-1">
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
          <div v-else-if="prunerConfig?.enabled" class="text-xs text-gray-400 dark:text-gray-500 py-2">
            {{ t('apiProxy.prunerNotAvailable') }}
          </div>

          <!-- ONNX Model Download Section -->
          <div v-if="prunerConfig?.enabled && modelStatus" class="border-t border-gray-100 dark:border-gray-700 pt-3 mt-2">
            <div class="flex items-center justify-between">
              <div>
                <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('apiProxy.modelStatus') }}</span>
                <p class="text-xs text-gray-400 dark:text-gray-500">SWE-Pruner (Qwen3-0.6B ONNX)</p>
              </div>
              <div v-if="modelStatus.ready" class="flex items-center gap-1.5">
                <span class="w-2 h-2 rounded-full bg-green-500"></span>
                <span class="text-xs text-green-600 dark:text-green-400">{{ t('apiProxy.modelReady') }}</span>
              </div>
              <div v-else-if="modelStatus.state === 'connecting'" class="flex items-center gap-2">
                <div class="animate-spin w-3.5 h-3.5 border-2 border-blue-500 border-t-transparent rounded-full"></div>
                <span class="text-xs text-blue-600 dark:text-blue-400">{{ t('apiProxy.modelConnecting') }}</span>
                <button
                  class="px-2.5 py-1 text-xs bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded-lg transition-colors"
                  @click="cancelModelDownload"
                >
                  {{ t('apiProxy.cancelDownload') }}
                </button>
              </div>
              <div v-else-if="modelStatus.downloading" class="flex items-center gap-2">
                <button
                  class="px-2.5 py-1 text-xs bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded-lg transition-colors"
                  @click="cancelModelDownload"
                >
                  {{ t('apiProxy.cancelDownload') }}
                </button>
              </div>
              <div v-else-if="modelStatus.state === 'error'" class="flex items-center gap-2">
                <button
                  class="px-3 py-1.5 text-xs bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 hover:bg-blue-200 dark:hover:bg-blue-900/50 rounded-lg transition-colors"
                  @click="startModelDownload"
                >
                  {{ t('apiProxy.retry') }}
                </button>
              </div>
              <div v-else>
                <button
                  class="px-3 py-1.5 text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600 rounded-lg transition-colors"
                  @click="startModelDownload"
                >
                  {{ t('apiProxy.downloadModel') }}
                </button>
              </div>
            </div>

            <!-- Connecting indicator -->
            <div v-if="modelStatus.state === 'connecting' && modelStatus.progress" class="mt-3 space-y-1.5">
              <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ modelStatus.progress.file }} ({{ modelStatus.progress.file_index + 1 }}/{{ modelStatus.progress.total_files }})</span>
              </div>
              <div class="w-full h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div class="h-full bg-blue-500/50 dark:bg-blue-400/50 rounded-full animate-pulse w-full"></div>
              </div>
              <div class="text-xs text-gray-400 dark:text-gray-500">{{ t('apiProxy.modelConnecting') }}</div>
            </div>

            <!-- Download progress bar -->
            <div v-if="modelStatus.state === 'downloading' && modelStatus.progress" class="mt-3 space-y-1.5">
              <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                <span>{{ modelStatus.progress.file }} ({{ modelStatus.progress.file_index + 1 }}/{{ modelStatus.progress.total_files }})</span>
                <span>{{ modelStatus.progress.percentage.toFixed(1) }}%</span>
              </div>
              <div class="w-full h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div
                  class="h-full bg-blue-500 dark:bg-blue-400 rounded-full transition-all duration-300"
                  :style="{ width: modelStatus.progress.percentage + '%' }"
                ></div>
              </div>
              <div class="flex items-center justify-between text-xs text-gray-400 dark:text-gray-500">
                <span>{{ formatBytes(modelStatus.progress.downloaded) }} / {{ modelStatus.progress.total > 0 ? formatBytes(modelStatus.progress.total) : '...' }}</span>
                <span>{{ modelStatus.progress.speed_human }} &middot; {{ modelStatus.progress.eta || '...' }}</span>
              </div>
            </div>

            <!-- Error message -->
            <div v-if="modelStatus.state === 'error' && modelStatus.error" class="mt-2 px-3 py-2 bg-red-50 dark:bg-red-900/20 rounded-lg">
              <p class="text-xs text-red-600 dark:text-red-400">{{ t('apiProxy.modelDownloadError') }}: {{ modelStatus.error }}</p>
            </div>

            <!-- Not downloaded hint -->
            <div v-if="!modelStatus.ready && !modelStatus.downloading && modelStatus.state !== 'connecting' && modelStatus.state !== 'error'" class="mt-2 text-xs text-gray-400 dark:text-gray-500">
              {{ t('apiProxy.modelNotDownloaded') }} &middot; ~1.4 GB
            </div>
          </div>
        </div>
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
    </template>
  </div>
</template>
