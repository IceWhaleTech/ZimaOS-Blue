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

// Computed
const criticalAlerts = computed(() => alerts.value.filter((a) => a.severity === 'critical'))
const highAlerts = computed(() => alerts.value.filter((a) => a.severity === 'high'))
const mediumAlerts = computed(() => alerts.value.filter((a) => a.severity === 'medium'))
const lowAlerts = computed(() => alerts.value.filter((a) => a.severity === 'low'))

const totalAlerts = computed(() => alerts.value.length)
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
      return 'text-blue-600 bg-blue-100 dark:text-blue-400 dark:bg-blue-900/30'
    default:
      return 'text-gray-600 bg-gray-100 dark:text-gray-400 dark:bg-gray-900/30'
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
      return 'Prompt Injection'
    case 'rate_limit':
      return 'Rate Limit'
    case 'auth_failure':
      return 'Auth Failure'
    case 'anomaly':
      return 'Anomaly'
    default:
      return type
  }
}

// Methods
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
    error.value = e instanceof Error ? e.message : 'Failed to fetch security data'
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
    <!-- Header -->
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Security Alerts</h2>
      <button
        @click="fetchData"
        :disabled="loading"
        class="px-3 py-1.5 text-sm bg-gray-100 hover:bg-gray-200 dark:bg-gray-700 dark:hover:bg-gray-600 rounded-md transition-colors disabled:opacity-50"
      >
        {{ loading ? 'Refreshing...' : 'Refresh' }}
      </button>
    </div>

    <!-- Error -->
    <div
      v-if="error"
      class="mb-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-400"
    >
      {{ error }}
    </div>

    <!-- Summary Cards -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
      <div class="p-3 bg-red-50 dark:bg-red-900/20 rounded-lg">
        <div class="text-2xl font-bold text-red-600 dark:text-red-400">
          {{ criticalAlerts.length + highAlerts.length }}
        </div>
        <div class="text-sm text-red-600/70 dark:text-red-400/70">Critical/High</div>
      </div>
      <div class="p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
        <div class="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
          {{ mediumAlerts.length }}
        </div>
        <div class="text-sm text-yellow-600/70 dark:text-yellow-400/70">Medium</div>
      </div>
      <div class="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg">
        <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">
          {{ lowAlerts.length }}
        </div>
        <div class="text-sm text-blue-600/70 dark:text-blue-400/70">Low</div>
      </div>
      <div class="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
        <div class="text-2xl font-bold text-gray-600 dark:text-gray-400">
          {{ unresolvedAlerts }}
        </div>
        <div class="text-sm text-gray-600/70 dark:text-gray-400/70">Unresolved</div>
      </div>
    </div>

    <!-- Guard Stats -->
    <div v-if="guardStats" class="mb-4 p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
      <h3 class="font-medium text-gray-900 dark:text-white mb-3">Prompt Guard Status</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400">Status:</span>
          <span :class="guardStats.enabled ? 'text-green-600' : 'text-red-600'" class="ml-2 font-medium">
            {{ guardStats.enabled ? 'Enabled' : 'Disabled' }}
          </span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Patterns:</span>
          <span class="ml-2 font-medium text-gray-900 dark:text-white">{{ guardStats.pattern_count }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Detections:</span>
          <span class="ml-2 font-medium text-yellow-600">{{ guardStats.detection_count }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Blocked:</span>
          <span class="ml-2 font-medium text-red-600">{{ guardStats.blocked_count }}</span>
        </div>
      </div>
    </div>

    <!-- Auth Stats -->
    <div v-if="authStats" class="mb-4 p-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
      <h3 class="font-medium text-gray-900 dark:text-white mb-3">Authentication Status</h3>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <span class="text-gray-500 dark:text-gray-400">Auth:</span>
          <span :class="authStats.auth_enabled ? 'text-green-600' : 'text-gray-600'" class="ml-2 font-medium">
            {{ authStats.auth_enabled ? 'Enabled' : 'Disabled' }}
          </span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">API Keys:</span>
          <span class="ml-2 font-medium text-gray-900 dark:text-white">{{ authStats.api_key_count }}</span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Auth Failures:</span>
          <span :class="authStats.auth_failures > 0 ? 'text-red-600' : 'text-green-600'" class="ml-2 font-medium">
            {{ authStats.auth_failures }}
          </span>
        </div>
        <div>
          <span class="text-gray-500 dark:text-gray-400">Rate Limited:</span>
          <span :class="authStats.rate_limit_hits > 0 ? 'text-yellow-600' : 'text-green-600'" class="ml-2 font-medium">
            {{ authStats.rate_limit_hits }}
          </span>
        </div>
      </div>
    </div>

    <!-- Alerts List -->
    <div class="space-y-2">
      <div v-if="alerts.length === 0 && !loading" class="text-center py-8 text-gray-500 dark:text-gray-400">
        No security alerts
      </div>

      <div
        v-for="alert in alerts"
        :key="alert.id"
        class="p-3 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700"
      >
        <div class="flex items-start gap-3">
          <span class="text-xl">{{ typeIcon(alert.type) }}</span>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span
                :class="severityColor(alert.severity)"
                class="px-2 py-0.5 text-xs font-medium rounded-full uppercase"
              >
                {{ alert.severity }}
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
              <span v-if="alert.source_ip">IP: {{ alert.source_ip }}</span>
              <span>{{ new Date(alert.timestamp).toLocaleString() }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.security-alerts {
  @apply p-4;
}
</style>
