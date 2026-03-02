<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { settingsApi, type SmallModelStats, type ShadowQualityGateEvalResponse, type ShadowQualityResponse } from '@/api/settings'

const { t } = useI18n()

const stats = ref<SmallModelStats | null>(null)
const shadowQuality = ref<ShadowQualityResponse | null>(null)
const shadowGate = ref<ShadowQualityGateEvalResponse | null>(null)
const loading = ref(true)
const refreshTimer = ref<number | null>(null)
const autoRolloutRunning = ref(false)
const autoRolloutStatus = ref('')

const shortQASuccessRate = computed(() => {
  const s = stats.value
  if (!s || s.short_qa_route_attempts <= 0) return 0
  return Math.round((s.short_qa_route_success / s.short_qa_route_attempts) * 100)
})

const toolDispatchSuccessRate = computed(() => {
  const s = stats.value
  if (!s || s.tool_dispatch_route_attempts <= 0) return 0
  return Math.round((s.tool_dispatch_route_success / s.tool_dispatch_route_attempts) * 100)
})

const summarySuccessRate = computed(() => {
  const s = stats.value
  if (!s || s.summary_attempts <= 0) return 0
  return Math.round((s.summary_success / s.summary_attempts) * 100)
})

const docExtractSuccessRate = computed(() => {
  const s = stats.value
  if (!s || s.doc_extract_attempts <= 0) return 0
  return Math.round((s.doc_extract_success / s.doc_extract_attempts) * 100)
})

const fallbackTop = computed(() => {
  const reasons = stats.value?.fallback_reasons || {}
  return Object.entries(reasons)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4)
})

const recentShadowSamples = computed(() => {
  const samples = shadowQuality.value?.samples || []
  return [...samples].reverse().slice(0, 5)
})

const shadowGateRows = computed(() => shadowGate.value?.scenes || [])

const shadowGateBadgeClass = computed(() => {
  if (shadowGate.value?.overall_pass) {
    return 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300'
  }
  return 'bg-amber-100 dark:bg-amber-900/30 text-amber-700 dark:text-amber-300'
})

const successRateColor = computed(() => {
  const rate = shortQASuccessRate.value
  if (rate >= 90) return 'text-green-600 dark:text-green-400'
  if (rate >= 70) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
})

const toolDispatchSuccessRateColor = computed(() => {
  const rate = toolDispatchSuccessRate.value
  if (rate >= 90) return 'text-green-600 dark:text-green-400'
  if (rate >= 70) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
})

const summarySuccessRateColor = computed(() => {
  const rate = summarySuccessRate.value
  if (rate >= 90) return 'text-green-600 dark:text-green-400'
  if (rate >= 70) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
})

const docExtractSuccessRateColor = computed(() => {
  const rate = docExtractSuccessRate.value
  if (rate >= 90) return 'text-green-600 dark:text-green-400'
  if (rate >= 70) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
})

function normalizeReason(reason: string): string {
  return reason.split('_').join(' ')
}

function sceneLabel(scene: string): string {
  switch (scene) {
    case 'short_qa_shadow':
      return 'QA'
    case 'tool_dispatch_shadow':
      return 'Tool'
    default:
      return scene || '-'
  }
}

function formatSampleTime(value: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleTimeString([], { hour12: false })
}

function compactDigest(value?: string): string {
  const raw = (value || '').trim()
  if (!raw) return '-'
  if (raw.length <= 36) return raw
  return raw.slice(0, 36) + '...'
}

async function fetchStats() {
  try {
    const [statsRes, qualityRes, gateRes] = await Promise.all([
      settingsApi.getSmallModelStats(),
      settingsApi.getSmallModelShadowQuality({ limit: 40 }),
      settingsApi.getSmallModelShadowQualityGateEval({ limit: 200 }),
    ])
    stats.value = statsRes.data
    shadowQuality.value = qualityRes.data
    shadowGate.value = gateRes.data
  } catch (e) {
    console.error('Failed to fetch small-model stats:', e)
  } finally {
    loading.value = false
  }
}

async function executeAutoRollout() {
  if (autoRolloutRunning.value) return
  autoRolloutRunning.value = true
  autoRolloutStatus.value = ''
  try {
    const res = await settingsApi.executeSmallModelShadowAutoRollout()
    const data = res.data
    shadowGate.value = data.gate_eval
    if (data.advanced) {
      autoRolloutStatus.value = `Rollout ${data.current_percent}% -> ${data.next_percent}%`
    } else if (data.reason === 'already_at_max') {
      autoRolloutStatus.value = 'Already at 100%'
    } else if (data.reason === 'gate_not_passed') {
      autoRolloutStatus.value = 'Gate HOLD, no rollout'
    } else {
      autoRolloutStatus.value = 'No rollout change'
    }
    await fetchStats()
  } catch (e) {
    autoRolloutStatus.value = 'Auto rollout failed'
    console.error('Failed to execute auto rollout:', e)
  } finally {
    autoRolloutRunning.value = false
  }
}

onMounted(() => {
  fetchStats()
  refreshTimer.value = window.setInterval(fetchStats, 30000)
})

onUnmounted(() => {
  if (refreshTimer.value) {
    clearInterval(refreshTimer.value)
  }
})
</script>

<template>
  <div class="rounded-xl border border-gray-200 dark:border-gray-600 bg-white dark:bg-gray-700 p-4 space-y-4">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <div class="w-2.5 h-2.5 rounded-full bg-blue-500" :class="loading ? 'animate-pulse' : ''"></div>
        <p class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('settings.smallModel.statsTitle', 'Routing & Fallback Stats') }}
        </p>
      </div>
      <button
        class="px-2 py-1 rounded border border-gray-200 dark:border-gray-600 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50"
        :disabled="loading"
        @click="fetchStats"
      >
        {{ t('common.refresh', 'Refresh') }}
      </button>
    </div>

    <div class="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-12 gap-3">
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.shortQAAttempts', 'Short QA Attempts') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.short_qa_route_attempts || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.shortQASuccessRate', 'Short QA Success') }}</p>
        <p class="mt-1 text-lg font-semibold" :class="successRateColor">{{ shortQASuccessRate }}%</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.toolDispatchAttempts', 'Tool Dispatch Attempts') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.tool_dispatch_route_attempts || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.toolDispatchSuccessRate', 'Tool Dispatch Success') }}</p>
        <p class="mt-1 text-lg font-semibold" :class="toolDispatchSuccessRateColor">{{ toolDispatchSuccessRate }}%</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.summarySuccessRate', 'Summary Success') }}</p>
        <p class="mt-1 text-lg font-semibold" :class="summarySuccessRateColor">{{ summarySuccessRate }}%</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.docExtractSuccessRate', 'Doc Extract Success') }}</p>
        <p class="mt-1 text-lg font-semibold" :class="docExtractSuccessRateColor">{{ docExtractSuccessRate }}%</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.fallbackTotal', 'Fallback Total') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.small_model_fallback_total || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.timeoutTotal', 'Timeout Total') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.small_model_timeout_total || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.latencyMs', 'Small-model Latency') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ (stats?.small_model_latency_ms || 0).toFixed(1) }}ms</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.deepResearchFallbacks', 'DeepResearch Fallbacks') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.no_provider_deepresearch_total || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.irTakeovers', 'IR Takeovers') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.ir_takeover_total || 0 }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800/40 p-3">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.autoRollbacks', 'Auto Rollbacks') }}</p>
        <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ stats?.auto_rollback_total || 0 }}</p>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-3 text-xs">
      <div class="rounded-lg border border-gray-200 dark:border-gray-600 p-3">
        <p class="mb-2 text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.shadowTraffic', 'Shadow Traffic') }}</p>
        <div class="flex flex-wrap gap-2">
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300">
            QA {{ stats?.short_qa_shadow_total || 0 }}
          </span>
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300">
            Tool {{ stats?.tool_dispatch_shadow_total || 0 }}
          </span>
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300">
            Fail {{ stats?.shadow_failures || 0 }}
          </span>
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-300">
            Delta {{ ((stats?.shadow_quality_delta || 0) * 100).toFixed(1) }}% / {{ stats?.shadow_quality_samples || 0 }}
          </span>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300">
            QA {{ (stats?.short_qa_latency_ms || 0).toFixed(1) }}ms
          </span>
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300">
            Tool {{ (stats?.tool_dispatch_latency_ms || 0).toFixed(1) }}ms
          </span>
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300">
            Summary {{ (stats?.summary_latency_ms || 0).toFixed(1) }}ms
          </span>
          <span class="inline-flex items-center rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300">
            Extract {{ (stats?.doc_extract_latency_ms || 0).toFixed(1) }}ms
          </span>
        </div>
      </div>

      <div class="rounded-lg border border-gray-200 dark:border-gray-600 p-3">
        <p class="mb-2 text-gray-500 dark:text-gray-400">{{ t('settings.smallModel.fallbackReasons', 'Fallback Reasons') }}</p>
        <div v-if="fallbackTop.length === 0" class="text-gray-400">-</div>
        <div v-else class="flex flex-wrap gap-2">
          <span
            v-for="[reason, count] in fallbackTop"
            :key="reason"
            class="inline-flex items-center gap-1 rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300"
          >
            {{ normalizeReason(reason) }} <span class="font-semibold">{{ count }}</span>
          </span>
        </div>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 dark:border-gray-600 p-3 text-xs space-y-3">
      <div class="flex items-center justify-between gap-2">
        <p class="text-gray-500 dark:text-gray-400">Shadow Gate Eval</p>
        <div class="flex items-center gap-2">
          <button
            data-testid="small-model-auto-rollout-exec"
            class="px-2 py-1 rounded border border-gray-200 dark:border-gray-600 text-xs text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 disabled:opacity-50"
            :disabled="autoRolloutRunning"
            @click="executeAutoRollout"
          >
            {{ autoRolloutRunning ? 'Running...' : 'Auto Rollout' }}
          </button>
          <span class="inline-flex items-center rounded-md px-2 py-1 font-semibold" :class="shadowGateBadgeClass">
            {{ shadowGate?.overall_pass ? 'PASS' : 'HOLD' }}
          </span>
        </div>
      </div>
      <div v-if="autoRolloutStatus" class="text-gray-500 dark:text-gray-400">
        {{ autoRolloutStatus }}
      </div>

      <div v-if="shadowGateRows.length === 0" class="text-gray-400">-</div>
      <div v-else class="flex flex-wrap gap-2">
        <span
          v-for="row in shadowGateRows"
          :key="row.scene"
          class="inline-flex items-center gap-1 rounded-md px-2 py-1 bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300"
        >
          {{ sceneLabel(row.scene) }} {{ (row.average_delta * 100).toFixed(1) }}% / {{ row.samples }}
          <span class="font-semibold">{{ row.pass ? 'PASS' : 'HOLD' }}</span>
        </span>
      </div>

      <div>
        <p class="mb-2 text-gray-500 dark:text-gray-400">Recent Shadow Samples</p>
        <div v-if="recentShadowSamples.length === 0" class="text-gray-400">-</div>
        <div v-else class="space-y-1">
          <div
            v-for="sample in recentShadowSamples"
            :key="`${sample.scene}-${sample.created_at}-${sample.delta}`"
            class="flex items-center justify-between gap-2 rounded-md px-2 py-1 bg-gray-50 dark:bg-gray-800/50 text-gray-700 dark:text-gray-300"
          >
            <span>{{ formatSampleTime(sample.created_at) }} {{ sceneLabel(sample.scene) }} {{ (sample.delta * 100).toFixed(1) }}%</span>
            <span class="text-gray-500 dark:text-gray-400">{{ compactDigest(sample.main_digest) }} -> {{ compactDigest(sample.shadow_digest) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
