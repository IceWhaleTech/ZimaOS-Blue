<script setup lang="ts">
// @ts-nocheck
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyApi, type FailoverConfig, type ProviderRaceOverview } from '@/api/proxy'

const emit = defineEmits<{ 'status-change': [msg: string] }>()
const { t, te } = useI18n()

const loading = ref(true)
const failoverConfig = ref<FailoverConfig | null>(null)
const providerRaceStats = ref<ProviderRaceOverview | null>(null)
const togglingProviderRace = ref(false)
const providerRaceEnabled = computed(() => failoverConfig.value?.provider_race?.enabled === true)
const providerRace = computed(() => failoverConfig.value?.provider_race || {})
const hasProviderRaceStats = computed(() => (providerRaceStats.value?.requests_total || 0) > 0)

function formatRatio(v?: number): string {
  if (typeof v !== 'number') return '-'
  return `${Math.round(v * 100)}%`
}

function formatMilliseconds(v?: number): string {
  if (typeof v !== 'number' || v <= 0) return '-'
  return `${Math.round(v)} ms`
}

function formatSeconds(v?: number): string {
  if (typeof v !== 'number') return '-'
  if (v <= 0) return '0s'
  return `${Math.round(v / 1_000_000_000)}s`
}

function tr(key: string, fallback = ''): string {
  return te(key) ? t(key) : fallback
}

async function fetchAll() {
  loading.value = true
  try {
    const overviewRes = await proxyApi.getFailoverOverview().catch(() => null)
    if (overviewRes) {
      failoverConfig.value = overviewRes.data.config
      providerRaceStats.value = overviewRes.data.provider_race || null
      return
    }

    const failoverRes = await proxyApi.getFailoverConfig().catch(() => null)
    if (failoverRes) failoverConfig.value = failoverRes.data
    providerRaceStats.value = null
  } finally {
    loading.value = false
  }
}

async function toggleProviderRace() {
  if (togglingProviderRace.value || !failoverConfig.value) return
  togglingProviderRace.value = true
  try {
    const enabled = !providerRaceEnabled.value
    const res = await proxyApi.updateFailoverConfig({
      provider_race: {
        enabled,
      },
    })
    failoverConfig.value = res.data
    emit(
      'status-change',
      t(enabled ? 'apiProxy.providerRaceEnabled' : 'apiProxy.providerRaceDisabled')
    )
  } finally {
    togglingProviderRace.value = false
  }
}

onMounted(fetchAll)
</script>

<template>
  <div class="space-y-6">
    <!-- Loading -->
    <div
      v-if="loading"
      class="text-center py-8"
    >
      <div
        class="animate-spin w-6 h-6 border-2 border-gray-900 dark:border-gray-700 border-t-transparent rounded-full mx-auto"
      />
      <p class="text-gray-400 mt-2 text-sm">
        {{ t('common.loading') }}
      </p>
    </div>

    <template v-else>
      <!-- Smart Failover Section -->
      <div
        v-if="failoverConfig"
        class="glass-card p-4"
      >
        <div class="mb-4">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('apiProxy.failoverDesc') }}
          </p>
        </div>

        <div class="grid grid-cols-4 gap-2 mt-3">
          <div
            v-for="sub in [
              {
                key: 'circuit_breaker',
                label: 'apiProxy.circuitBreaker',
                desc: 'apiProxy.circuitBreakerDesc',
              },
              {
                key: 'context_window_check',
                label: 'apiProxy.contextWindowCheck',
                desc: 'apiProxy.contextWindowCheckDesc',
              },
              {
                key: 'error_classification',
                label: 'apiProxy.errorClassification',
                desc: 'apiProxy.errorClassificationDesc',
              },
              {
                key: 'streaming_anomaly',
                label: 'apiProxy.streamingAnomaly',
                desc: 'apiProxy.streamingAnomalyDesc',
              },
            ] as const"
            :key="sub.key"
            class="group relative flex flex-col items-center gap-1.5 py-2.5 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg"
            :title="tr(sub.desc, sub.key)"
          >
            <span class="inline-flex h-2.5 w-2.5 rounded-full bg-green-500" />
            <span class="text-xs text-gray-600 dark:text-gray-400 text-center leading-tight">{{
              tr(sub.label, sub.key)
            }}</span>
          </div>
        </div>

        <div
          v-if="failoverConfig.provider_race"
          class="mt-4 border-t border-gray-100 dark:border-white/10 pt-3"
        >
          <div class="flex items-center justify-between">
            <div>
              <h4 class="text-xs font-semibold text-gray-900 dark:text-white">
                {{ t('apiProxy.providerRaceTitle') }}
              </h4>
              <p class="text-[11px] text-gray-500 dark:text-gray-400 mt-0.5">
                {{ t('apiProxy.providerRaceDesc') }}
              </p>
            </div>
            <button
              type="button"
              data-testid="provider-race-toggle"
              :disabled="togglingProviderRace"
              :class="[
                'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                providerRaceEnabled
                  ? 'bg-green-600 dark:bg-green-500'
                  : 'bg-gray-300 dark:bg-gray-600',
                togglingProviderRace ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer',
              ]"
              @click="toggleProviderRace"
            >
              <span
                :class="[
                  'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                  providerRaceEnabled ? 'translate-x-6' : 'translate-x-1',
                ]"
              />
            </button>
          </div>

          <div class="grid grid-cols-2 md:grid-cols-4 gap-2 mt-3">
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('apiProxy.providerRaceMaxParallel') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ providerRace.max_parallel ?? '-' }}
              </p>
            </div>
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('apiProxy.providerRaceMinProviders') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ providerRace.min_providers ?? '-' }}
              </p>
            </div>
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('apiProxy.providerRaceSinkThreshold') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ formatRatio(providerRace.empty_rate_sink_threshold) }}
              </p>
            </div>
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('apiProxy.providerRaceExcludeThreshold') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ formatRatio(providerRace.empty_rate_exclude_threshold) }}
              </p>
            </div>
          </div>
          <p class="text-[11px] text-gray-500 dark:text-gray-400 mt-2">
            {{
              t('apiProxy.providerRaceCooldownRule', {
                threshold: formatRatio(providerRace.empty_rate_cooldown_threshold),
                samples: providerRace.empty_rate_min_samples ?? '-',
                duration: formatSeconds(providerRace.empty_rate_cooldown),
              })
            }}
          </p>

          <div
            data-testid="provider-race-stats"
            class="grid grid-cols-2 md:grid-cols-4 gap-2 mt-3"
          >
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('cache.hitRate') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ hasProviderRaceStats ? formatRatio(providerRaceStats?.hit_rate) : '-' }}
              </p>
            </div>
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('tokenEconomy.estimated') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{
                  providerRaceStats?.estimated_savings_samples
                    ? formatMilliseconds(providerRaceStats?.estimated_latency_saved_ms)
                    : '-'
                }}
              </p>
            </div>
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('metrics.avgLatency') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{
                  providerRaceStats?.successful_races
                    ? formatMilliseconds(providerRaceStats?.avg_winner_latency_ms)
                    : '-'
                }}
              </p>
            </div>
            <div class="py-2 px-2 bg-gray-50 dark:bg-gray-700/30 rounded-lg">
              <p class="text-[11px] text-gray-400 dark:text-gray-500">
                {{ t('common.requests') }}
              </p>
              <p class="text-sm font-semibold text-gray-900 dark:text-white mt-0.5">
                {{ providerRaceStats?.requests_total ?? 0 }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
