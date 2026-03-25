<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxyApi, type SecurityAlert, type GuardStats, type AuthStats } from '@/api/proxy'

const { t } = useI18n()

const alerts = ref<SecurityAlert[]>([])
const guardStats = ref<GuardStats | null>(null)
const authStats = ref<AuthStats | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const refreshInterval = ref<number | null>(null)

const criticalAlerts = computed(() => alerts.value.filter((a) => a.severity === 'critical'))
const highAlerts = computed(() => alerts.value.filter((a) => a.severity === 'high'))
const mediumAlerts = computed(() => alerts.value.filter((a) => a.severity === 'medium'))
const lowAlerts = computed(() => alerts.value.filter((a) => a.severity === 'low'))
const unresolvedAlerts = computed(() => alerts.value.filter((a) => !a.resolved).length)

const severityColor = (severity: string) => {
  switch (severity) {
    case 'critical':
      return 'text-red-600 bg-red-100 dark:text-red-400 dark:bg-red-900/30'
    case 'high':
      return 'text-orange-600 bg-orange-100 dark:text-orange-400 dark:bg-orange-900/30'
    case 'medium':
      return 'text-yellow-600 bg-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30'
    case 'low':
      return 'text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700/50'
    default:
      return 'text-gray-600 bg-gray-100 dark:text-gray-400 dark:bg-gray-700/30'
  }
}

const severityLabel = (severity: string) => {
  switch (severity) {
    case 'critical':
      return t('security.threats.riskLevel.critical')
    case 'high':
      return t('security.threats.riskLevel.high')
    case 'medium':
      return t('security.threats.riskLevel.medium')
    case 'low':
      return t('security.threats.riskLevel.low')
    default:
      return severity
  }
}

const typeIcon = (type: string) => {
  switch (type) {
    case 'injection':
      return '🛡️'
    case 'rate_limit':
      return '⏱️'
    case 'auth_failure':
      return '🔐'
    case 'anomaly':
      return '⚠️'
    default:
      return '📋'
  }
}

const typeLabel = (type: string) => {
  switch (type) {
    case 'injection':
      return t('securityAlerts.types.injection')
    case 'rate_limit':
      return t('securityAlerts.types.rateLimit')
    case 'auth_failure':
      return t('securityAlerts.types.authFailure')
    case 'anomaly':
      return t('securityAlerts.types.anomaly')
    default:
      return type
  }
}

async function fetchData() {
  loading.value = true
  error.value = null
  try {
    const [alertsRes, guardRes, authRes] = await Promise.all([
      proxyApi.getSecurityAlerts(),
      proxyApi.getGuardStats(),
      proxyApi.getAuthStats(),
    ])
    alerts.value = alertsRes.data
    guardStats.value = guardRes.data
    authStats.value = authRes.data
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('securityAlerts.fetchFailed')
    console.error('Failed to fetch security data:', e)
  } finally {
    loading.value = false
  }
}

function startAutoRefresh() {
  refreshInterval.value = window.setInterval(fetchData, 30000)
}

function stopAutoRefresh() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
}

onMounted(() => {
  fetchData()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})
</script>

<template>
  <div class="security-alerts">
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('securityAlerts.title') }}
      </h2>
      <button
        :disabled="loading"
        class="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-700 dark:bg-gray-500 dark:hover:bg-gray-600 rounded-md transition-colors disabled:opacity-50"
        @click="fetchData"
      >
        {{ loading ? t('common.refreshing') : t('common.refresh') }}
      </button>
    </div>

    <div
      v-if="error"
      class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400"
    >
      {{ error }}
    </div>

    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
      <div class="p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
        <div class="text-2xl font-bold text-red-600 dark:text-red-400">
          {{ criticalAlerts.length + highAlerts.length }}
        </div>
        <div class="text-sm text-red-600/70 dark:text-red-400/70">
          {{ t('securityAlerts.summary.criticalHigh') }}
        </div>
      </div>
      <div class="p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
        <div class="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
          {{ mediumAlerts.length }}
        </div>
        <div class="text-sm text-yellow-600/70 dark:text-yellow-400/70">
          {{ t('security.threats.riskLevel.medium') }}
        </div>
      </div>
      <div class="p-3 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
        <div class="text-2xl font-bold text-gray-900 dark:text-white dark:text-white">
          {{ lowAlerts.length }}
        </div>
        <div class="text-sm text-gray-900 dark:text-white/70 dark:text-white/70">
          {{ t('security.threats.riskLevel.low') }}
        </div>
      </div>
      <div class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
        <div class="text-2xl font-bold text-gray-600 dark:text-gray-400">
          {{ unresolvedAlerts }}
        </div>
        <div class="text-sm text-gray-600/70 dark:text-gray-400/70">
          {{ t('securityAlerts.summary.unresolved') }}
        </div>
      </div>
    </div>

    <div
      v-if="guardStats"
      class="mb-4 p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700"
    >
      <h3 class="font-medium text-gray-900 dark:text-white mb-3">
        {{ t('securityAlerts.sections.promptGuard') }}
      </h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400">{{ t('common.status') }}:</span>
          <span
            :class="guardStats.enabled ? 'text-green-600' : 'text-red-600'"
            class="security-alerts-inline-value font-medium"
          >
            {{ guardStats.enabled ? t('common.enabled') : t('common.disabled') }}
          </span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.patterns') }}:</span
          >
          <span class="security-alerts-inline-value font-medium text-gray-900 dark:text-white">{{
            guardStats.pattern_count
          }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.detections') }}:</span
          >
          <span class="security-alerts-inline-value font-medium text-yellow-600">{{
            guardStats.detection_count
          }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.blocked') }}:</span
          >
          <span class="security-alerts-inline-value font-medium text-red-600">{{
            guardStats.blocked_count
          }}</span>
        </div>
      </div>
    </div>

    <div
      v-if="authStats"
      class="mb-4 p-4 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700"
    >
      <h3 class="font-medium text-gray-900 dark:text-white mb-3">
        {{ t('securityAlerts.sections.authentication') }}
      </h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.auth') }}:</span
          >
          <span
            :class="authStats.auth_enabled ? 'text-green-600' : 'text-gray-600'"
            class="security-alerts-inline-value font-medium"
          >
            {{ authStats.auth_enabled ? t('common.enabled') : t('common.disabled') }}
          </span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.apiKeys') }}:</span
          >
          <span class="security-alerts-inline-value font-medium text-gray-900 dark:text-white">{{
            authStats.api_key_count
          }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.authFailures') }}:</span
          >
          <span
            :class="authStats.auth_failures > 0 ? 'text-red-600' : 'text-green-600'"
            class="security-alerts-inline-value font-medium"
          >
            {{ authStats.auth_failures }}
          </span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400"
            >{{ t('securityAlerts.labels.rateLimited') }}:</span
          >
          <span
            :class="authStats.rate_limit_hits > 0 ? 'text-yellow-600' : 'text-green-600'"
            class="security-alerts-inline-value font-medium"
          >
            {{ authStats.rate_limit_hits }}
          </span>
        </div>
      </div>
    </div>

    <div class="space-y-2">
      <div
        v-if="alerts.length === 0 && !loading"
        class="text-center py-8 text-gray-500 dark:text-gray-400"
      >
        {{ t('common.noSecurityAlerts') }}
      </div>

      <div
        v-for="alert in alerts"
        :key="alert.id"
        class="p-3 bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700"
      >
        <div class="flex items-start gap-3">
          <span class="text-xl">{{ typeIcon(alert.type) }}</span>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span
                :class="severityColor(alert.severity)"
                class="px-2 py-0.5 text-xs font-medium rounded-full uppercase"
              >
                {{ severityLabel(alert.severity) }}
              </span>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                {{ typeLabel(alert.type) }}
              </span>
            </div>
            <p class="text-sm text-gray-900 dark:text-white">{{ alert.message }}</p>
            <p v-if="alert.details" class="text-xs text-gray-500 dark:text-gray-400 mt-1">
              {{ alert.details }}
            </p>
            <div class="flex items-center gap-4 mt-2 text-xs text-gray-400">
              <span v-if="alert.source_ip"
                >{{ t('securityAlerts.labels.ip') }}: {{ alert.source_ip }}</span
              >
              <span>{{ new Date(alert.timestamp).toLocaleString() }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.security-alerts-inline-value {
  margin-inline-start: 0.5rem;
}
</style>

<style scoped>
.security-alerts {
  @apply p-4;
}
</style>
