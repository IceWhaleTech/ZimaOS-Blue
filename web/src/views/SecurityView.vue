<script setup lang="ts">
import { ref, onMounted, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi } from '@/api/security'
import { apiKeyApi } from '@/api/auth'
import { mfaApi } from '@/api/mfa'
import { sandboxApi } from '@/api/sandbox'
import type { Session, SecurityStats, SecurityEvent, BlockedIP, ThreatStats, ThreatEvent } from '@/api/security'
import type { ApiKey } from '@/api/auth'
import type { MFAStatus } from '@/api/mfa'
import type { SandboxInfo, ExecutionResult } from '@/api/sandbox'

const { t } = useI18n()

// Tabs
const activeTab = ref<'overview' | 'sessions' | 'apikeys' | 'mfa' | 'events' | 'threats' | 'sandbox'>('overview')

// Data
const loading = ref(false)
const stats = ref<SecurityStats | null>(null)
const sessions = ref<Session[]>([])
const apiKeys = ref<ApiKey[]>([])
const mfaStatus = ref<MFAStatus | null>(null)
const events = ref<SecurityEvent[]>([])
const blockedIPs = ref<BlockedIP[]>([])

// Threat detection data
const threatStats = ref<ThreatStats | null>(null)
const recentThreats = ref<ThreatEvent[]>([])
const threatLoading = ref(false)
const threatRefreshInterval = ref<number | null>(null)

// Sandbox data
const sandboxInfo = ref<SandboxInfo | null>(null)
const sandboxExecutions = ref<ExecutionResult[]>([])
const sandboxLoading = ref(false)

// API Key creation
const showCreateKeyModal = ref(false)
const newKeyName = ref('')
const newKeyScopes = ref<string[]>(['read'])
const createdKey = ref<string | null>(null)

// MFA
const mfaSetupData = ref<{ secret: string; uri: string; qr_code?: string } | null>(null)
const mfaCode = ref('')
const mfaError = ref<string | null>(null)

// Computed
const mfaEnabledPercent = computed(() => {
  if (!stats.value || stats.value.total_users === 0) return 0
  return Math.round((stats.value.mfa_enabled_users / stats.value.total_users) * 100)
})

onMounted(async () => {
  await Promise.all([loadStats(), loadThreatStats()])
  // Auto-refresh threat stats every 30 seconds
  threatRefreshInterval.value = window.setInterval(loadThreatStats, 30000)
})

onUnmounted(() => {
  if (threatRefreshInterval.value) {
    clearInterval(threatRefreshInterval.value)
  }
})

async function loadStats() {
  loading.value = true
  try {
    const response = await securityApi.getStats()
    stats.value = response.data
  } catch {
    stats.value = null
  } finally {
    loading.value = false
  }
}

async function loadSessions() {
  try {
    const response = await securityApi.listSessions()
    sessions.value = response.data
  } catch {
    sessions.value = []
  }
}

async function loadApiKeys() {
  try {
    const response = await apiKeyApi.list()
    apiKeys.value = response.data
  } catch {
    apiKeys.value = []
  }
}

async function loadMFAStatus() {
  try {
    const response = await mfaApi.getStatus()
    mfaStatus.value = response.data
  } catch {
    mfaStatus.value = null
  }
}

async function loadEvents() {
  try {
    const response = await securityApi.getEvents({ limit: 50 })
    events.value = response.data
  } catch {
    events.value = []
  }
}

async function loadSandboxInfo() {
  sandboxLoading.value = true
  try {
    const response = await sandboxApi.getInfo()
    sandboxInfo.value = response.data
  } catch {
    sandboxInfo.value = null
  } finally {
    sandboxLoading.value = false
  }
}

async function loadThreatStats() {
  try {
    const response = await securityApi.getThreatStats()
    threatStats.value = response.data
  } catch {
    threatStats.value = null
  }
}

async function loadRecentThreats() {
  threatLoading.value = true
  try {
    const response = await securityApi.getRecentThreats(50)
    recentThreats.value = response.data
  } catch {
    recentThreats.value = []
  } finally {
    threatLoading.value = false
  }
}

async function killExecution(id: string) {
  if (!confirm(t('security.sandbox.confirmKill'))) return
  try {
    await sandboxApi.kill(id)
    // Update the execution status in the list
    const execution = sandboxExecutions.value.find(e => e.id === id)
    if (execution) {
      execution.status = 'killed'
    }
  } catch {
    // Handle error
  }
}

async function revokeSession(id: string) {
  if (!confirm(t('security.confirmRevokeSession'))) return
  try {
    await securityApi.revokeSession(id)
    sessions.value = sessions.value.filter(s => s.id !== id)
  } catch {
    // Handle error
  }
}

async function revokeAllSessions() {
  if (!confirm(t('security.confirmRevokeAllSessions'))) return
  try {
    await securityApi.revokeAllSessions()
    await loadSessions()
  } catch {
    // Handle error
  }
}

async function createApiKey() {
  if (!newKeyName.value) return
  try {
    const response = await apiKeyApi.create({
      name: newKeyName.value,
      scopes: newKeyScopes.value,
    })
    createdKey.value = response.data.key
    apiKeys.value.push(response.data.api_key)
    newKeyName.value = ''
    newKeyScopes.value = ['read']
  } catch {
    // Handle error
  }
}

async function deleteApiKey(id: string) {
  if (!confirm(t('security.confirmDeleteApiKey'))) return
  try {
    await apiKeyApi.delete(id)
    apiKeys.value = apiKeys.value.filter(k => k.id !== id)
  } catch {
    // Handle error
  }
}

async function startMFASetup() {
  try {
    const response = await mfaApi.setup(true)
    mfaSetupData.value = response.data
    mfaError.value = null
  } catch {
    mfaError.value = t('security.mfaSetupFailed')
  }
}

async function verifyMFA() {
  if (!mfaSetupData.value || !mfaCode.value) return
  try {
    await mfaApi.verify(mfaCode.value, mfaSetupData.value.secret)
    mfaSetupData.value = null
    mfaCode.value = ''
    await loadMFAStatus()
  } catch {
    mfaError.value = t('security.invalidMFACode')
  }
}

async function disableMFA() {
  const password = prompt(t('security.enterPasswordToDisableMFA'))
  if (!password) return
  try {
    await mfaApi.disable(password)
    await loadMFAStatus()
  } catch {
    alert(t('security.disableMFAFailed'))
  }
}

function switchTab(tab: typeof activeTab.value) {
  activeTab.value = tab
  if (tab === 'sessions' && sessions.value.length === 0) loadSessions()
  if (tab === 'apikeys' && apiKeys.value.length === 0) loadApiKeys()
  if (tab === 'mfa' && !mfaStatus.value) loadMFAStatus()
  if (tab === 'events' && events.value.length === 0) loadEvents()
  if (tab === 'threats' && recentThreats.value.length === 0) loadRecentThreats()
  if (tab === 'sandbox' && !sandboxInfo.value) loadSandboxInfo()
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function getSeverityColor(severity: string): string {
  switch (severity) {
    case 'critical': return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'high': return 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300'
    case 'medium': return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    default: return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function getStatusColor(status: string): string {
  switch (status) {
    case 'running': return 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
    case 'completed': return 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
    case 'failed': return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'timeout': return 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300'
    case 'killed': return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
    default: return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDuration(duration: string): string {
  // Parse Go duration string like "30s", "5m0s", etc.
  return duration
}

function getRiskLevelColor(level: string): string {
  switch (level) {
    case 'critical': return 'bg-red-500 text-white'
    case 'high': return 'bg-orange-500 text-white'
    case 'medium': return 'bg-yellow-500 text-white'
    case 'low': return 'bg-blue-500 text-white'
    default: return 'bg-green-500 text-white'
  }
}

function getThreatTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    injection: t('security.threats.types.injection'),
    xss: t('security.threats.types.xss'),
    sql_injection: t('security.threats.types.sqlInjection'),
    path_traversal: t('security.threats.types.pathTraversal'),
    command_injection: t('security.threats.types.commandInjection'),
    prompt_injection: t('security.threats.types.promptInjection'),
    brute_force: t('security.threats.types.bruteForce'),
    rate_limit: t('security.threats.types.rateLimit'),
    suspicious_ip: t('security.threats.types.suspiciousIp'),
  }
  return labels[type] || type
}
</script>

<template>
  <div class="security-view p-4 sm:p-6 max-w-6xl mx-auto">
    <!-- Security Status Banner -->
    <div v-if="threatStats" class="mb-6">
      <div :class="[
        'rounded-lg p-4 flex items-center justify-between',
        threatStats.is_secure
          ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
          : 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
      ]">
        <div class="flex items-center gap-3">
          <div :class="[
            'w-10 h-10 rounded-full flex items-center justify-center',
            threatStats.is_secure ? 'bg-green-500' : 'bg-red-500'
          ]">
            <svg v-if="threatStats.is_secure" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <div>
            <h2 :class="[
              'text-lg font-semibold',
              threatStats.is_secure ? 'text-green-800 dark:text-green-200' : 'text-red-800 dark:text-red-200'
            ]">
              {{ threatStats.is_secure ? t('security.threats.statusSecure') : t('security.threats.statusAtRisk') }}
            </h2>
            <p :class="[
              'text-sm',
              threatStats.is_secure ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'
            ]">
              {{ t('security.threats.threatsDetected', { count: threatStats.total_threats_24h }) }}
              <span v-if="threatStats.blocked_threats_24h > 0">
                ({{ t('security.threats.blocked', { count: threatStats.blocked_threats_24h }) }})
              </span>
            </p>
          </div>
        </div>
        <span :class="['px-3 py-1 rounded-full text-sm font-medium', getRiskLevelColor(threatStats.risk_level)]">
          {{ t(`security.threats.riskLevel.${threatStats.risk_level}`) }}
        </span>
      </div>
    </div>

    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('security.title') }}</h1>

    <!-- Tabs -->
    <div class="flex gap-4 mb-6 border-b border-gray-200 dark:border-slate-700 overflow-x-auto">
      <button
        v-for="tab in ['overview', 'threats', 'sessions', 'apikeys', 'mfa', 'events', 'sandbox'] as const"
        :key="tab"
        :class="[
          'pb-3 px-1 text-sm font-medium border-b-2 transition-colors whitespace-nowrap flex items-center gap-2',
          activeTab === tab
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-900 dark:hover:text-white'
        ]"
        @click="switchTab(tab)"
      >
        {{ t(`security.${tab}`) }}
        <span v-if="tab === 'threats' && threatStats && threatStats.total_threats_24h > 0" class="px-1.5 py-0.5 text-xs rounded-full bg-red-500 text-white">
          {{ threatStats.total_threats_24h }}
        </span>
      </button>
    </div>

    <!-- Overview Tab -->
    <div v-if="activeTab === 'overview'" class="space-y-6">
      <div v-if="loading" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>

      <div v-else-if="stats" class="grid grid-cols-2 lg:grid-cols-3 gap-4">
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ stats.active_sessions }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.activeSessions') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ stats.failed_logins_24h }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.failedLogins24h') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{ stats.blocked_ips }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.blockedIPs') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ mfaEnabledPercent }}%</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.mfaEnabled') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">{{ stats.api_keys_active }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.activeApiKeys') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats.total_users }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.totalUsers') }}</div>
        </div>
      </div>
    </div>

    <!-- Threats Tab -->
    <div v-if="activeTab === 'threats'" class="space-y-6">
      <!-- Threat Stats -->
      <div v-if="threatStats" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ threatStats.total_threats_24h }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.threats.total24h') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ threatStats.blocked_threats_24h }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.threats.blocked24h') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ threatStats.critical_threats_24h }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.threats.critical24h') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{ threatStats.high_threats_24h }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.threats.high24h') }}</div>
        </div>
      </div>

      <!-- Top Threat Types -->
      <div v-if="threatStats && Object.keys(threatStats.top_threat_types).length > 0" class="glass-card p-6">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('security.threats.topTypes') }}</h3>
        <div class="space-y-3">
          <div v-for="(count, type) in threatStats.top_threat_types" :key="type" class="flex items-center justify-between">
            <span class="text-gray-700 dark:text-gray-300">{{ getThreatTypeLabel(type as string) }}</span>
            <span class="px-2 py-1 bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300 rounded text-sm">{{ count }}</span>
          </div>
        </div>
      </div>

      <!-- Recent Threats -->
      <div class="glass-card p-6">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('security.threats.recentThreats') }}</h3>
          <button
            class="px-3 py-1.5 bg-gray-100 dark:bg-slate-700 hover:bg-gray-200 dark:hover:bg-slate-600 text-gray-700 dark:text-gray-300 rounded text-sm transition-colors"
            @click="loadRecentThreats"
          >
            {{ t('common.refresh') }}
          </button>
        </div>

        <div v-if="threatLoading" class="text-center py-8 text-gray-500 dark:text-slate-400">
          {{ t('common.loading') }}
        </div>

        <div v-else-if="recentThreats.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
          {{ t('security.threats.noThreats') }}
        </div>

        <div v-else class="space-y-3">
          <div v-for="threat in recentThreats" :key="threat.id" class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4">
            <div class="flex items-start justify-between">
              <div class="flex-1">
                <div class="flex items-center gap-2 mb-1 flex-wrap">
                  <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getSeverityColor(threat.severity)]">
                    {{ threat.severity }}
                  </span>
                  <span class="text-gray-900 dark:text-white font-medium">{{ getThreatTypeLabel(threat.type) }}</span>
                  <span v-if="threat.blocked" class="px-2 py-0.5 bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 rounded-full text-xs">
                    {{ t('security.threats.blockedLabel') }}
                  </span>
                </div>
                <div class="text-sm text-gray-600 dark:text-slate-300 mb-1">{{ threat.description }}</div>
                <div class="text-xs text-gray-500 dark:text-slate-400">
                  IP: {{ threat.ip_address }} | {{ t('security.threats.source') }}: {{ threat.source }}
                </div>
                <div v-if="threat.details" class="text-xs text-gray-400 dark:text-slate-500 mt-1 font-mono truncate">
                  {{ threat.details }}
                </div>
              </div>
              <span class="text-xs text-gray-400 dark:text-slate-500 whitespace-nowrap ml-4">{{ formatDate(threat.timestamp) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Sessions Tab -->
    <div v-if="activeTab === 'sessions'" class="space-y-4">
      <div class="flex justify-end">
        <button
          class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm transition-colors"
          @click="revokeAllSessions"
        >
          {{ t('security.revokeAllSessions') }}
        </button>
      </div>

      <div v-if="sessions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('security.noSessions') }}
      </div>

      <div v-else class="space-y-3">
        <div v-for="session in sessions" :key="session.id" class="glass-card p-4">
          <div class="flex items-start justify-between">
            <div>
              <div class="flex items-center gap-2 mb-1">
                <span class="text-gray-900 dark:text-white font-medium">{{ session.ip_address }}</span>
                <span v-if="session.is_current" class="px-2 py-0.5 bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300 rounded-full text-xs">
                  {{ t('security.currentSession') }}
                </span>
              </div>
              <div class="text-sm text-gray-500 dark:text-slate-400 truncate max-w-md">
                {{ session.user_agent }}
              </div>
              <div class="text-xs text-gray-400 dark:text-slate-500 mt-1">
                {{ t('security.lastActivity') }}: {{ formatDate(session.last_activity) }}
              </div>
            </div>
            <button
              v-if="!session.is_current"
              class="px-3 py-1.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded text-sm transition-colors"
              @click="revokeSession(session.id)"
            >
              {{ t('security.revoke') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- API Keys Tab -->
    <div v-if="activeTab === 'apikeys'" class="space-y-4">
      <div class="flex justify-end">
        <button
          class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
          @click="showCreateKeyModal = true"
        >
          {{ t('security.createApiKey') }}
        </button>
      </div>

      <div v-if="apiKeys.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('security.noApiKeys') }}
      </div>

      <div v-else class="space-y-3">
        <div v-for="key in apiKeys" :key="key.id" class="glass-card p-4">
          <div class="flex items-start justify-between">
            <div>
              <div class="text-gray-900 dark:text-white font-medium mb-1">{{ key.name }}</div>
              <div class="text-sm text-gray-500 dark:text-slate-400 font-mono">{{ key.key_prefix }}...</div>
              <div class="flex gap-2 mt-2">
                <span v-for="scope in key.scopes" :key="scope" class="px-2 py-0.5 bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 rounded text-xs">
                  {{ scope }}
                </span>
              </div>
              <div class="text-xs text-gray-400 dark:text-slate-500 mt-2">
                {{ t('security.created') }}: {{ formatDate(key.created_at) }}
                <span v-if="key.last_used_at"> | {{ t('security.lastUsed') }}: {{ formatDate(key.last_used_at) }}</span>
              </div>
            </div>
            <button
              class="px-3 py-1.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded text-sm transition-colors"
              @click="deleteApiKey(key.id)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Create API Key Modal -->
      <div v-if="showCreateKeyModal" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="showCreateKeyModal = false; createdKey = null">
        <div class="bg-white dark:bg-slate-800 rounded-lg p-6 w-full max-w-md mx-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('security.createApiKey') }}</h3>

          <div v-if="createdKey" class="mb-4">
            <div class="text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('security.apiKeyCreated') }}</div>
            <div class="bg-gray-100 dark:bg-slate-700 p-3 rounded font-mono text-sm break-all">{{ createdKey }}</div>
            <div class="text-xs text-red-500 mt-2">{{ t('security.apiKeyWarning') }}</div>
          </div>

          <div v-else>
            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('security.keyName') }}</label>
              <input
                v-model="newKeyName"
                type="text"
                class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-slate-600"
                :placeholder="t('security.keyNamePlaceholder')"
              />
            </div>

            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('security.scopes') }}</label>
              <div class="flex flex-wrap gap-2">
                <label v-for="scope in ['read', 'write', 'admin']" :key="scope" class="flex items-center gap-2">
                  <input v-model="newKeyScopes" type="checkbox" :value="scope" class="rounded" />
                  <span class="text-sm text-gray-700 dark:text-gray-300">{{ scope }}</span>
                </label>
              </div>
            </div>
          </div>

          <div class="flex justify-end gap-3">
            <button
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
              @click="showCreateKeyModal = false; createdKey = null"
            >
              {{ createdKey ? t('common.close') : t('common.cancel') }}
            </button>
            <button
              v-if="!createdKey"
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
              :disabled="!newKeyName"
              @click="createApiKey"
            >
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- MFA Tab -->
    <div v-if="activeTab === 'mfa'" class="space-y-4">
      <div v-if="!mfaStatus" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>

      <div v-else class="glass-card p-6">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('security.twoFactorAuth') }}</h3>
            <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.mfaDescription') }}</p>
          </div>
          <span :class="[
            'px-3 py-1 rounded-full text-sm font-medium',
            mfaStatus.enabled
              ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
              : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
          ]">
            {{ mfaStatus.enabled ? t('security.enabled') : t('security.disabled') }}
          </span>
        </div>

        <div v-if="mfaStatus.enabled">
          <div class="text-sm text-gray-500 dark:text-slate-400 mb-4">
            {{ t('security.recoveryCodesRemaining', { count: mfaStatus.recovery_codes_remaining }) }}
          </div>
          <button
            class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg text-sm transition-colors"
            @click="disableMFA"
          >
            {{ t('security.disableMFA') }}
          </button>
        </div>

        <div v-else-if="mfaSetupData">
          <div class="mb-4">
            <p class="text-sm text-gray-500 dark:text-slate-400 mb-2">{{ t('security.scanQRCode') }}</p>
            <div v-if="mfaSetupData.qr_code" class="flex justify-center mb-4">
              <img :src="mfaSetupData.qr_code" alt="QR Code" class="w-48 h-48" />
            </div>
            <div class="text-xs text-gray-400 dark:text-slate-500 font-mono break-all">
              {{ mfaSetupData.secret }}
            </div>
          </div>

          <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ t('security.enterCode') }}</label>
            <input
              v-model="mfaCode"
              type="text"
              maxlength="6"
              class="w-full bg-gray-100 dark:bg-slate-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-200 dark:border-slate-600"
              placeholder="000000"
            />
          </div>

          <div v-if="mfaError" class="text-red-500 text-sm mb-4">{{ mfaError }}</div>

          <div class="flex gap-3">
            <button
              class="px-4 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors"
              @click="mfaSetupData = null; mfaCode = ''"
            >
              {{ t('common.cancel') }}
            </button>
            <button
              class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
              :disabled="mfaCode.length !== 6"
              @click="verifyMFA"
            >
              {{ t('security.verify') }}
            </button>
          </div>
        </div>

        <div v-else>
          <button
            class="px-4 py-2 bg-accent hover:bg-accent-hover text-white rounded-lg text-sm transition-colors"
            @click="startMFASetup"
          >
            {{ t('security.enableMFA') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Events Tab -->
    <div v-if="activeTab === 'events'" class="space-y-4">
      <div v-if="events.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('security.noEvents') }}
      </div>

      <div v-else class="space-y-3">
        <div v-for="event in events" :key="event.id" class="glass-card p-4">
          <div class="flex items-start justify-between">
            <div>
              <div class="flex items-center gap-2 mb-1">
                <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getSeverityColor(event.severity)]">
                  {{ event.severity }}
                </span>
                <span class="text-gray-900 dark:text-white font-medium">{{ event.type }}</span>
              </div>
              <div class="text-sm text-gray-500 dark:text-slate-400">
                IP: {{ event.ip_address }}
              </div>
            </div>
            <span class="text-xs text-gray-400 dark:text-slate-500">{{ formatDate(event.timestamp) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Sandbox Tab -->
    <div v-if="activeTab === 'sandbox'" class="space-y-6">
      <div v-if="sandboxLoading" class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('common.loading') }}
      </div>

      <template v-else-if="sandboxInfo">
        <!-- Sandbox Status -->
        <div class="glass-card p-6">
          <div class="flex items-center justify-between mb-4">
            <div>
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('security.sandbox.title') }}</h3>
              <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.sandbox.description') }}</p>
            </div>
            <span :class="[
              'px-3 py-1 rounded-full text-sm font-medium',
              sandboxInfo.supported
                ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300'
                : 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
            ]">
              {{ sandboxInfo.supported ? t('security.sandbox.supported') : t('security.sandbox.notSupported') }}
            </span>
          </div>

          <!-- Configuration Grid -->
          <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('security.sandbox.defaultTimeout') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ formatDuration(sandboxInfo.default_timeout) }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('security.sandbox.maxTimeout') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ formatDuration(sandboxInfo.max_timeout) }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('security.sandbox.memoryLimit') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ formatBytes(sandboxInfo.memory_limit) }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('security.sandbox.cpuLimit') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ sandboxInfo.cpu_limit }} {{ t('security.sandbox.cores') }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('security.sandbox.processLimit') }}</div>
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ sandboxInfo.process_limit }}</div>
            </div>
            <div class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-3">
              <div class="text-xs text-gray-500 dark:text-slate-400 mb-1">{{ t('security.sandbox.networkAccess') }}</div>
              <div class="text-sm font-medium" :class="sandboxInfo.network_enabled ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                {{ sandboxInfo.network_enabled ? t('common.enabled') : t('common.disabled') }}
              </div>
            </div>
          </div>
        </div>

        <!-- Running Executions -->
        <div class="glass-card p-6">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('security.sandbox.executions') }}</h3>

          <div v-if="sandboxExecutions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">
            {{ t('security.sandbox.noExecutions') }}
          </div>

          <div v-else class="space-y-3">
            <div v-for="execution in sandboxExecutions" :key="execution.id" class="bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4">
              <div class="flex items-start justify-between">
                <div>
                  <div class="flex items-center gap-2 mb-1">
                    <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(execution.status)]">
                      {{ execution.status }}
                    </span>
                    <span class="text-gray-900 dark:text-white font-mono text-sm">{{ execution.id.slice(0, 8) }}...</span>
                  </div>
                  <div v-if="execution.resource_usage" class="text-xs text-gray-500 dark:text-slate-400 mt-1">
                    {{ t('security.sandbox.memory') }}: {{ formatBytes(execution.resource_usage.memory_peak_bytes) }}
                  </div>
                  <div class="text-xs text-gray-400 dark:text-slate-500 mt-1">
                    {{ t('security.sandbox.started') }}: {{ formatDate(execution.start_time) }}
                  </div>
                </div>
                <button
                  v-if="execution.status === 'running'"
                  class="px-3 py-1.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50 rounded text-sm transition-colors"
                  @click="killExecution(execution.id)"
                >
                  {{ t('security.sandbox.kill') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </template>

      <div v-else class="text-center py-8 text-gray-500 dark:text-slate-400">
        {{ t('security.sandbox.loadFailed') }}
      </div>
    </div>
  </div>
</template>
