<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi } from '@/api/security'
import { useCompanionStore } from '@/stores/companion'
import { useCompanionStream } from '@/composables/useCompanionStream'
import type { CompanionSession, ThreatLevel, Platform } from '@/api/companion'

const { t } = useI18n()
const companionStore = useCompanionStore()

// WebSocket stream for Companion
const { isConnected, eventCount } = useCompanionStream({
  autoConnect: true,
})

// Main tab state
const mainTab = ref<'scan' | 'companion'>('scan')

// Security scan data
interface ScanItem {
  id: string
  category: string
  name: string
  description: string
  status: 'pending' | 'scanning' | 'passed' | 'warning' | 'failed'
  details?: string
}

const isScanning = ref(false)
const scanProgress = ref(0)
const scanResults = ref<ScanItem[]>([])
const scanCompleted = ref(false)

onMounted(async () => {
  // Auto-start security scan on page load
  startSecurityScan()
  // Load companion data
  await Promise.all([
    companionStore.fetchStats(),
    companionStore.fetchSessions(),
    companionStore.fetchAlerts(),
  ])
})

// Start security scan
async function startSecurityScan() {
  if (isScanning.value) return

  isScanning.value = true
  scanProgress.value = 0
  scanCompleted.value = false
  scanResults.value = []

  try {
    // Call the real API
    const response = await securityApi.runSecurityScan()
    const apiItems = response.data.items

    // Animate the scan results for visual effect
    const totalItems = apiItems.length
    const delayPerItem = 100 // Faster since we already have results

    for (let i = 0; i < totalItems; i++) {
      const apiItem = apiItems[i]

      // Add item with scanning status first
      const scanItem: ScanItem = {
        id: apiItem.id,
        category: apiItem.category,
        name: apiItem.name,
        description: apiItem.description,
        status: 'scanning',
      }
      scanResults.value.push(scanItem)

      // Brief delay for animation
      await new Promise(resolve => setTimeout(resolve, delayPerItem))

      // Update with actual result
      scanItem.status = apiItem.status as ScanItem['status']
      scanItem.details = apiItem.details || t(`security.scan.check${apiItem.status.charAt(0).toUpperCase() + apiItem.status.slice(1)}`)

      scanProgress.value = Math.round(((i + 1) / totalItems) * 100)
    }
  } catch (error) {
    console.error('Security scan failed:', error)
    // Fallback to showing error state
    scanResults.value = [{
      id: 'error',
      category: 'system',
      name: t('security.scan.error'),
      description: t('security.scan.errorDesc'),
      status: 'failed',
      details: t('security.scan.apiError'),
    }]
    scanProgress.value = 100
  }

  isScanning.value = false
  scanCompleted.value = true
}

// Get scan summary
const scanSummary = computed(() => {
  const passed = scanResults.value.filter(r => r.status === 'passed').length
  const warnings = scanResults.value.filter(r => r.status === 'warning').length
  const failed = scanResults.value.filter(r => r.status === 'failed').length
  return { passed, warnings, failed, total: scanResults.value.length }
})

// Get category label
function getCategoryLabel(category: string): string {
  const labels: Record<string, string> = {
    auth: t('security.scan.categories.auth'),
    input: t('security.scan.categories.input'),
    ai: t('security.scan.categories.ai'),
    network: t('security.scan.categories.network'),
    sandbox: t('security.scan.categories.sandbox'),
    data: t('security.scan.categories.data'),
    system: t('security.scan.categories.system'),
  }
  return labels[category] || category
}

// Get scan item name with i18n
function getScanItemName(id: string): string {
  const key = `security.scan.items.${id}.name`
  const translated = t(key)
  // If translation key doesn't exist, return the key itself (fallback)
  return translated === key ? id : translated
}

// Get scan item description with i18n
function getScanItemDescription(id: string): string {
  const key = `security.scan.items.${id}.description`
  const translated = t(key)
  return translated === key ? '' : translated
}

// Get scan item status icon and color
function getScanStatusClass(status: string): string {
  switch (status) {
    case 'passed': return 'text-green-500'
    case 'warning': return 'text-yellow-500'
    case 'failed': return 'text-red-500'
    case 'scanning': return 'text-blue-500 animate-pulse'
    default: return 'text-gray-400'
  }
}

// Get overall security status
const securityStatus = computed(() => {
  if (!scanCompleted.value) return 'scanning'
  if (scanSummary.value.failed > 0) return 'failed'
  if (scanSummary.value.warnings > 0) return 'warning'
  return 'passed'
})

// Companion state
const selectedSession = ref<CompanionSession | null>(null)
const showSessionDetail = ref(false)
const activeCompanionTab = ref<'sessions' | 'alerts' | 'realtime'>('sessions')

// Companion computed
const stats = computed(() => companionStore.stats)
const sessions = computed(() => companionStore.sortedSessions)
const alerts = computed(() => companionStore.alerts)
const realtimeEvents = computed(() => companionStore.realtimeEvents)
const unackedAlerts = computed(() => companionStore.unacknowledgedAlerts)

function selectSession(session: CompanionSession) {
  selectedSession.value = session
  showSessionDetail.value = true
  companionStore.fetchSession(session.id)
  companionStore.fetchSessionEvents(session.id)
}

function closeSessionDetail() {
  showSessionDetail.value = false
  selectedSession.value = null
  companionStore.clearCurrentSession()
}

async function acknowledgeAlert(alertId: string) {
  await companionStore.acknowledgeAlert(alertId)
}

function exportData(format: 'json' | 'csv') {
  companionStore.exportData({ format })
}

function getThreatColor(level: ThreatLevel): string {
  const colors: Record<ThreatLevel, string> = {
    none: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300',
    low: 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300',
    medium: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300',
    high: 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300',
    critical: 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300',
  }
  return colors[level] || colors.none
}

function getStatusColor(status: string): string {
  const colors: Record<string, string> = {
    active: 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300',
    idle: 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300',
    ended: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300',
    error: 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300',
  }
  return colors[status] ?? 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
}

function getPlatformIcon(platform: Platform): string {
  const icons: Record<Platform, string> = {
    whatsapp: 'W', telegram: 'T', discord: 'D', slack: 'S',
    matrix: 'M', feishu: 'F', web: 'W', api: 'A',
  }
  return icons[platform] || '?'
}

function getEventTypeIcon(type: string): string {
  const icons: Record<string, string> = {
    session_start: '▶', session_end: '■', message_received: '←',
    message_sent: '→', tool_call: '⚙', llm_request: '🤖',
    security_threat: '⚠', error: '✕',
  }
  return icons[type] || '•'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}
</script>

<template>
  <div class="security-view p-4 sm:p-6 max-w-7xl mx-auto">
    <!-- Header with Title -->
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('security.title') }}</h1>

    <!-- Main Tabs: Scan / Companion -->
    <div class="flex gap-2 mb-6 border-b border-gray-200 dark:border-slate-700">
      <button
        :class="[
          'px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
          mainTab === 'scan'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
        ]"
        @click="mainTab = 'scan'"
      >
        {{ t('security.scan.title') }}
      </button>
      <button
        :class="[
          'px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px',
          mainTab === 'companion'
            ? 'border-accent text-accent'
            : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300'
        ]"
        @click="mainTab = 'companion'"
      >
        {{ t('companion.title') }}
        <span v-if="unackedAlerts.length > 0" class="ml-1 px-1.5 py-0.5 bg-red-500 text-white text-xs rounded-full">
          {{ unackedAlerts.length }}
        </span>
      </button>
    </div>

    <!-- Security Scan Tab -->
    <div v-if="mainTab === 'scan'">
      <!-- Security Status Banner -->
    <div class="mb-6">
      <div :class="[
        'rounded-lg p-4 flex items-center justify-between',
        securityStatus === 'passed'
          ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
          : securityStatus === 'warning'
            ? 'bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800'
            : securityStatus === 'failed'
              ? 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
              : 'bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800'
      ]">
        <div class="flex items-center gap-3">
          <div :class="[
            'w-10 h-10 rounded-full flex items-center justify-center',
            securityStatus === 'passed' ? 'bg-green-500' :
            securityStatus === 'warning' ? 'bg-yellow-500' :
            securityStatus === 'failed' ? 'bg-red-500' : 'bg-blue-500'
          ]">
            <svg v-if="securityStatus === 'passed'" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
            </svg>
            <svg v-else-if="securityStatus === 'warning'" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <svg v-else-if="securityStatus === 'failed'" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 text-white animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </div>
          <div>
            <h2 :class="[
              'text-lg font-semibold',
              securityStatus === 'passed' ? 'text-green-800 dark:text-green-200' :
              securityStatus === 'warning' ? 'text-yellow-800 dark:text-yellow-200' :
              securityStatus === 'failed' ? 'text-red-800 dark:text-red-200' :
              'text-blue-800 dark:text-blue-200'
            ]">
              {{ securityStatus === 'passed' ? t('security.statusSecure') :
                 securityStatus === 'warning' ? t('security.statusWarning') :
                 securityStatus === 'failed' ? t('security.statusFailed') :
                 t('security.statusScanning') }}
            </h2>
            <p :class="[
              'text-sm',
              securityStatus === 'passed' ? 'text-green-600 dark:text-green-400' :
              securityStatus === 'warning' ? 'text-yellow-600 dark:text-yellow-400' :
              securityStatus === 'failed' ? 'text-red-600 dark:text-red-400' :
              'text-blue-600 dark:text-blue-400'
            ]">
              {{ scanCompleted
                ? t('security.scanSummary', { passed: scanSummary.passed, warnings: scanSummary.warnings, failed: scanSummary.failed })
                : t('security.scanInProgress') }}
            </p>
          </div>
        </div>
      </div>
    </div>

    <!-- Security Scan Card -->
    <div class="glass-card p-6">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('security.scan.title') }}</h3>
          <p class="text-sm text-gray-500 dark:text-slate-400">{{ t('security.scan.description') }}</p>
        </div>
        <button
          :disabled="isScanning"
          :class="[
            'px-4 py-2 rounded-lg text-white font-medium transition-all flex items-center gap-2',
            isScanning
              ? 'bg-gray-400 cursor-not-allowed'
              : 'bg-accent hover:bg-accent/90'
          ]"
          @click="startSecurityScan"
        >
          <svg v-if="isScanning" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          {{ isScanning ? t('security.scan.scanning') : t('security.scan.startScan') }}
        </button>
      </div>

      <!-- Progress Bar -->
      <div v-if="isScanning || scanCompleted" class="mb-4">
        <div class="flex items-center justify-between text-sm mb-2">
          <span class="text-gray-600 dark:text-slate-300">
            {{ isScanning ? t('security.scan.progress') : t('security.scan.completed') }}
          </span>
          <span class="font-medium text-gray-900 dark:text-white">{{ scanProgress }}%</span>
        </div>
        <div class="h-2 bg-gray-200 dark:bg-slate-700 rounded-full overflow-hidden">
          <div
            class="h-full bg-gradient-to-r from-blue-500 to-green-500 transition-all duration-300 ease-out"
            :style="{ width: `${scanProgress}%` }"
          ></div>
        </div>
      </div>

      <!-- Scan Summary -->
      <div v-if="scanCompleted" class="grid grid-cols-3 gap-4 mb-4">
        <div class="bg-green-50 dark:bg-green-900/20 rounded-lg p-3 text-center">
          <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ scanSummary.passed }}</div>
          <div class="text-xs text-green-700 dark:text-green-300">{{ t('security.scan.passed') }}</div>
        </div>
        <div class="bg-yellow-50 dark:bg-yellow-900/20 rounded-lg p-3 text-center">
          <div class="text-2xl font-bold text-yellow-600 dark:text-yellow-400">{{ scanSummary.warnings }}</div>
          <div class="text-xs text-yellow-700 dark:text-yellow-300">{{ t('security.scan.warnings') }}</div>
        </div>
        <div class="bg-red-50 dark:bg-red-900/20 rounded-lg p-3 text-center">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ scanSummary.failed }}</div>
          <div class="text-xs text-red-700 dark:text-red-300">{{ t('security.scan.failed') }}</div>
        </div>
      </div>

      <!-- Scan Results List -->
      <div v-if="scanResults.length > 0" class="space-y-1 max-h-96 overflow-y-auto">
        <template v-for="(item, index) in scanResults" :key="item.id">
          <!-- Category Header -->
          <div
            v-if="index === 0 || scanResults[index - 1].category !== item.category"
            class="text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wider pt-3 pb-1"
          >
            {{ getCategoryLabel(item.category) }}
          </div>
          <!-- Scan Item -->
          <div
            :class="[
              'flex items-center gap-3 py-2 px-3 rounded-lg transition-all duration-200',
              item.status === 'scanning' ? 'bg-blue-50 dark:bg-blue-900/20' : 'hover:bg-gray-50 dark:hover:bg-slate-700/50'
            ]"
          >
            <!-- Status Icon -->
            <div :class="['flex-shrink-0', getScanStatusClass(item.status)]">
              <svg v-if="item.status === 'passed'" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <svg v-else-if="item.status === 'warning'" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <svg v-else-if="item.status === 'failed'" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <svg v-else-if="item.status === 'scanning'" xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <circle cx="12" cy="12" r="9" stroke-width="2" />
              </svg>
            </div>
            <!-- Item Info -->
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ getScanItemName(item.id) }}</div>
              <div class="text-xs text-gray-500 dark:text-slate-400 truncate">{{ getScanItemDescription(item.id) }}</div>
            </div>
            <!-- Status Badge -->
            <div v-if="item.status !== 'pending'" class="flex-shrink-0">
              <span
                :class="[
                  'px-2 py-0.5 text-xs rounded-full font-medium',
                  item.status === 'passed' ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' :
                  item.status === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300' :
                  item.status === 'failed' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' :
                  'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
                ]"
              >
                {{ item.status === 'scanning' ? t('security.scan.checking') :
                   item.status === 'passed' ? t('security.scan.passed') :
                   item.status === 'warning' ? t('security.scan.warnings') :
                   item.status === 'failed' ? t('security.scan.failed') : item.status }}
              </span>
            </div>
          </div>
        </template>
      </div>
    </div>
    </div>

    <!-- Companion Tab -->
    <div v-if="mainTab === 'companion'">
      <!-- Connection Status & Export -->
      <div class="flex items-center justify-between mb-4">
        <div class="flex items-center gap-2">
          <span :class="['w-2 h-2 rounded-full', isConnected() ? 'bg-green-500 animate-pulse' : 'bg-gray-400']" />
          <span class="text-xs text-gray-500 dark:text-slate-400">
            {{ isConnected() ? t('companion.connected') : t('companion.disconnected') }}
          </span>
        </div>
        <div class="flex gap-2">
          <button class="px-3 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors" @click="exportData('csv')">
            {{ t('companion.exportCSV') }}
          </button>
          <button class="px-3 py-2 bg-gray-200 dark:bg-slate-600 hover:bg-gray-300 dark:hover:bg-slate-500 text-gray-900 dark:text-white rounded-lg text-sm transition-colors" @click="exportData('json')">
            {{ t('companion.exportJSON') }}
          </button>
        </div>
      </div>

      <!-- Stats Cards -->
      <div v-if="stats" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4 mb-6">
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ stats.activeSessions }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.activeSessions') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ stats.totalSessions }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalSessions') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">{{ stats.totalEvents }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalEvents') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{ stats.totalAlerts }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalAlerts') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ stats.unackedAlerts }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.unackedAlerts') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ eventCount }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.realtimeEvents') }}</div>
        </div>
      </div>

      <!-- Companion Sub-Tabs -->
      <div class="flex gap-2 mb-4 border-b border-gray-200 dark:border-slate-700">
        <button :class="['px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px', activeCompanionTab === 'sessions' ? 'border-accent text-accent' : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300']" @click="activeCompanionTab = 'sessions'">
          {{ t('companion.sessions') }} ({{ sessions.length }})
        </button>
        <button :class="['px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px', activeCompanionTab === 'alerts' ? 'border-accent text-accent' : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300']" @click="activeCompanionTab = 'alerts'">
          {{ t('companion.alerts') }}
          <span v-if="unackedAlerts.length > 0" class="ml-1 px-1.5 py-0.5 bg-red-500 text-white text-xs rounded-full">{{ unackedAlerts.length }}</span>
        </button>
        <button :class="['px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px', activeCompanionTab === 'realtime' ? 'border-accent text-accent' : 'border-transparent text-gray-500 dark:text-slate-400 hover:text-gray-700 dark:hover:text-slate-300']" @click="activeCompanionTab = 'realtime'">
          {{ t('companion.realtime') }}
          <span v-if="isConnected()" class="ml-1 w-2 h-2 bg-green-500 rounded-full inline-block animate-pulse" />
        </button>
      </div>

      <!-- Sessions Tab -->
      <div v-if="activeCompanionTab === 'sessions'" class="space-y-3">
        <div v-if="companionStore.loading" class="text-center py-8 text-gray-500 dark:text-slate-400">{{ t('common.loading') }}</div>
        <div v-else-if="sessions.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">{{ t('companion.noSessions') }}</div>
        <div v-for="session in sessions" :key="session.id" class="glass-card p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-slate-700/50 transition-colors" @click="selectSession(session)">
          <div class="flex items-start justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="w-8 h-8 flex items-center justify-center bg-gray-200 dark:bg-slate-600 rounded-lg text-sm font-bold">{{ getPlatformIcon(session.platform) }}</span>
              <div>
                <div class="font-medium text-gray-900 dark:text-white">{{ session.id.slice(0, 8) }}...</div>
                <div class="text-xs text-gray-500 dark:text-slate-400">{{ session.userId }}</div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(session.status)]">{{ t(`companion.status.${session.status}`) }}</span>
              <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getThreatColor(session.threatLevel)]">{{ t(`companion.threat.${session.threatLevel}`) }}</span>
            </div>
          </div>
          <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-slate-400">
            <span>{{ t('companion.events') }}: {{ session.eventCount }}</span>
            <span>{{ t('companion.started') }}: {{ formatDate(session.startedAt) }}</span>
            <span v-if="session.duration">{{ t('companion.duration') }}: {{ formatDuration(session.duration) }}</span>
          </div>
        </div>
        <button v-if="companionStore.hasMoreSessions" class="w-full py-2 text-sm text-accent hover:text-accent-hover" @click="companionStore.loadMoreSessions()">{{ t('companion.loadMore') }}</button>
      </div>

      <!-- Alerts Tab -->
      <div v-if="activeCompanionTab === 'alerts'" class="space-y-3">
        <div v-if="companionStore.loadingAlerts" class="text-center py-8 text-gray-500 dark:text-slate-400">{{ t('common.loading') }}</div>
        <div v-else-if="alerts.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">{{ t('companion.noAlerts') }}</div>
        <div v-for="alert in alerts" :key="alert.id" :class="['glass-card p-4', alert.acknowledged ? 'opacity-60' : '']">
          <div class="flex items-start justify-between mb-2">
            <div>
              <div class="font-medium text-gray-900 dark:text-white">{{ alert.title }}</div>
              <div class="text-sm text-gray-500 dark:text-slate-400">{{ alert.description }}</div>
            </div>
            <div class="flex items-center gap-2">
              <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' : alert.severity === 'error' ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300' : alert.severity === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300' : 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300']">{{ t(`companion.alerts.${alert.severity}`) }}</span>
              <button v-if="!alert.acknowledged" class="px-2 py-1 bg-accent hover:bg-accent-hover text-white text-xs rounded transition-colors" @click.stop="acknowledgeAlert(alert.id)">{{ t('companion.acknowledge') }}</button>
            </div>
          </div>
          <div class="flex flex-wrap gap-4 text-xs text-gray-500 dark:text-slate-400">
            <span>{{ t('companion.session') }}: {{ alert.sessionId.slice(0, 8) }}...</span>
            <span>{{ formatDate(alert.createdAt) }}</span>
            <span v-if="alert.acknowledged">{{ t('companion.ackedBy') }}: {{ alert.ackedBy }}</span>
          </div>
        </div>
        <button v-if="companionStore.hasMoreAlerts" class="w-full py-2 text-sm text-accent hover:text-accent-hover" @click="companionStore.loadMoreAlerts()">{{ t('companion.loadMore') }}</button>
      </div>

      <!-- Realtime Tab -->
      <div v-if="activeCompanionTab === 'realtime'" class="space-y-2">
        <div v-if="realtimeEvents.length === 0" class="text-center py-8 text-gray-500 dark:text-slate-400">{{ t('companion.waitingForEvents') }}</div>
        <div v-for="event in realtimeEvents" :key="event.id" class="glass-card p-3 text-sm">
          <div class="flex items-center gap-2 mb-1">
            <span class="text-lg">{{ getEventTypeIcon(event.eventType) }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ t(`companion.eventType.${event.eventType}`) }}</span>
            <span class="text-xs text-gray-400 dark:text-slate-500">{{ event.sessionId.slice(0, 8) }}...</span>
            <span class="ml-auto text-xs text-gray-400 dark:text-slate-500">{{ formatDate(event.timestamp) }}</span>
          </div>
          <div v-if="event.message" class="text-xs text-gray-500 dark:text-slate-400 truncate">{{ event.message.direction }}: {{ event.message.content }}</div>
          <div v-if="event.toolCall" class="text-xs text-gray-500 dark:text-slate-400">{{ event.toolCall.toolName }} - {{ event.toolCall.status }} ({{ formatDuration(event.toolCall.duration) }})</div>
          <div v-if="event.llmRequest" class="text-xs text-gray-500 dark:text-slate-400">{{ event.llmRequest.provider }}/{{ event.llmRequest.model }} - {{ event.llmRequest.totalTokens }} tokens</div>
          <div v-if="event.security" :class="['text-xs', getThreatColor(event.security.threatLevel)]">{{ event.security.threatLevel }}: {{ event.security.threatTypes.join(', ') }}</div>
        </div>
      </div>

      <!-- Session Detail Modal -->
      <div v-if="showSessionDetail && selectedSession" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" @click.self="closeSessionDetail">
        <div class="bg-white dark:bg-slate-800 rounded-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col">
          <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-700">
            <div>
              <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('companion.sessionDetail') }}</h2>
              <p class="text-sm text-gray-500 dark:text-slate-400">{{ selectedSession.id }}</p>
            </div>
            <button class="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors" @click="closeSessionDetail">
              <span class="text-xl">&times;</span>
            </button>
          </div>
          <div class="flex-1 overflow-y-auto p-4">
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
              <div>
                <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.platform') }}</div>
                <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.platform }}</div>
              </div>
              <div>
                <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.status') }}</div>
                <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getStatusColor(selectedSession.status)]">{{ selectedSession.status }}</span>
              </div>
              <div>
                <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.threatLevel') }}</div>
                <span :class="['px-2 py-0.5 rounded-full text-xs font-medium', getThreatColor(selectedSession.threatLevel)]">{{ selectedSession.threatLevel }}</span>
              </div>
              <div>
                <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.events') }}</div>
                <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.eventCount }}</div>
              </div>
            </div>
            <h3 class="text-sm font-medium text-gray-900 dark:text-white mb-3">{{ t('companion.eventHistory') }}</h3>
            <div v-if="companionStore.loadingEvents" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('common.loading') }}</div>
            <div v-else class="space-y-2">
              <div v-for="event in companionStore.sessionEvents" :key="event.id" class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg text-sm">
                <div class="flex items-center gap-2 mb-1">
                  <span>{{ getEventTypeIcon(event.eventType) }}</span>
                  <span class="font-medium text-gray-900 dark:text-white">{{ t(`companion.eventType.${event.eventType}`) }}</span>
                  <span class="ml-auto text-xs text-gray-400 dark:text-slate-500">{{ formatDate(event.timestamp) }}</span>
                </div>
                <div v-if="event.message" class="text-xs text-gray-500 dark:text-slate-400">{{ event.message.direction }}: {{ event.message.content }}</div>
                <div v-if="event.toolCall" class="text-xs text-gray-500 dark:text-slate-400">{{ event.toolCall.toolName }} - {{ event.toolCall.status }}</div>
                <div v-if="event.llmRequest" class="text-xs text-gray-500 dark:text-slate-400">{{ event.llmRequest.provider }}/{{ event.llmRequest.model }}</div>
              </div>
            </div>
            <button v-if="companionStore.hasMoreEvents" class="w-full py-2 text-sm text-accent hover:text-accent-hover mt-4" @click="companionStore.loadMoreEvents()">{{ t('companion.loadMore') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
