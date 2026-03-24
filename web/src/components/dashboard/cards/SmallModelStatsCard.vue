<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { settingsApi, type SmallModelStats } from '@/api/settings'

const { t } = useI18n()

const stats = ref<SmallModelStats | null>(null)
const loading = ref(true)
const refreshTimer = ref<number | null>(null)

const shortQASuccessRate = computed(() => {
  const s = stats.value
  if (!s || s.short_qa_route_attempts <= 0) return 0
  return Math.round((s.short_qa_route_success / s.short_qa_route_attempts) * 100)
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

const successRateColor = computed(() => {
  const rate = shortQASuccessRate.value
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

async function fetchStats() {
  try {
    const statsRes = await settingsApi.getSmallModelStats()
    stats.value = statsRes.data
  } catch (e) {
    console.error('Failed to fetch small-model stats:', e)
  } finally {
    loading.value = false
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
  <div class="dashboard-card-surface p-4">
    <div class="dashboard-card-stack">
      <div class="dashboard-card-footer">
        <div class="dashboard-card-copy">
          <p class="dashboard-card-label">Routing</p>
          <p class="dashboard-card-subtitle mt-2">
            {{ t('settings.smallModel.statsTitle', 'Routing & Fallback Stats') }}
          </p>
        </div>
        <button class="dashboard-card-chip" :disabled="loading" @click="fetchStats">
          {{ t('common.refresh', 'Refresh') }}
        </button>
      </div>

      <div class="grid grid-cols-2 md:grid-cols-4 xl:grid-cols-12 gap-3">
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.shortQAAttempts', 'Short QA Attempts') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ stats?.short_qa_route_attempts || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.shortQASuccessRate', 'Short QA Success') }}
          </p>
          <p class="mt-1 text-lg font-semibold" :class="successRateColor">
            {{ shortQASuccessRate }}%
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.summarySuccessRate', 'Summary Success') }}
          </p>
          <p class="mt-1 text-lg font-semibold" :class="summarySuccessRateColor">
            {{ summarySuccessRate }}%
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.docExtractSuccessRate', 'Doc Extract Success') }}
          </p>
          <p class="mt-1 text-lg font-semibold" :class="docExtractSuccessRateColor">
            {{ docExtractSuccessRate }}%
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.fallbackTotal', 'Fallback Total') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ stats?.small_model_fallback_total || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.timeoutTotal', 'Timeout Total') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ stats?.small_model_timeout_total || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.latencyMs', 'Small-model Latency') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ (stats?.small_model_latency_ms || 0).toFixed(1) }}ms
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.deepResearchFallbacks', 'DeepResearch Fallbacks') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ stats?.no_provider_deepresearch_total || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.irTakeovers', 'IR Takeovers') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ stats?.ir_takeover_total || 0 }}
          </p>
        </div>
        <div class="dashboard-card-subsurface p-3">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('settings.smallModel.autoRollbacks', 'Auto Rollbacks') }}
          </p>
          <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ stats?.auto_rollback_total || 0 }}
          </p>
        </div>
      </div>

      <div class="dashboard-card-subsurface p-3 text-xs">
        <p class="mb-2 text-gray-500 dark:text-gray-400">
          {{ t('settings.smallModel.fallbackReasons', 'Fallback Reasons') }}
        </p>
        <div v-if="fallbackTop.length === 0" class="text-gray-400">-</div>
        <div v-else class="flex flex-wrap gap-2">
          <span v-for="[reason, count] in fallbackTop" :key="reason" class="dashboard-card-chip">
            {{ normalizeReason(reason) }} <span class="font-semibold">{{ count }}</span>
          </span>
        </div>
      </div>
    </div>
  </div>
</template>
