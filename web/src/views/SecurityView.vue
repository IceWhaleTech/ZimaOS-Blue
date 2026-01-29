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
const scanResultsExpanded = ref(true)

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

  // Auto-collapse if no issues
  if (scanSummary.value.warnings === 0 && scanSummary.value.failed === 0) {
    scanResultsExpanded.value = false
  }
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

// Companion computed
const stats = computed(() => companionStore.stats)
const sessions = computed(() => companionStore.sortedSessions.slice(0, 5)) // Show latest 5
const alerts = computed(() => companionStore.alerts.slice(0, 5)) // Show latest 5
const realtimeEvents = computed(() => companionStore.realtimeEvents.slice(0, 10)) // Show latest 10
const unackedAlerts = computed(() => companionStore.unacknowledgedAlerts)

// Session detail
const selectedSession = ref<CompanionSession | null>(null)
const showSessionDetail = ref(false)

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

async function deleteSession(sessionId: string) {
  if (!confirm(t('companion.deleteSessionConfirm'))) return
  try {
    await companionStore.deleteSession(sessionId)
    closeSessionDetail()
  } catch (e) {
    console.error('Failed to delete session:', e)
  }
}

async function acknowledgeAlert(alertId: string) {
  await companionStore.acknowledgeAlert(alertId)
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

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHour = Math.floor(diffMin / 60)

  if (diffSec < 60) {
    return t('companion.justNow')
  } else if (diffMin < 60) {
    return t('companion.minutesAgo', { n: diffMin })
  } else if (diffHour < 24) {
    return t('companion.hoursAgo', { n: diffHour })
  } else {
    return formatDate(dateStr)
  }
}

// Get alert title with i18n support for demo mode
function getAlertTitle(title: string): string {
  if (companionStore.isDemoMode) {
    const key = `companion.demo.alertTitles.${title}`
    const translated = t(key)
    return translated !== key ? translated : title
  }
  return title
}

// Get alert description with i18n support for demo mode
function getAlertDescription(description: string): string {
  if (companionStore.isDemoMode && description === 'demoDescription') {
    return t('companion.demo.alertDescription')
  }
  return description
}
</script>

<template>
  <div class="security-view p-4 sm:p-6 max-w-7xl mx-auto">
    <!-- Header with Title -->
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('security.title') }}</h1>

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
      <div v-if="scanResults.length > 0">
        <!-- Collapsible Header -->
        <button
          v-if="scanCompleted"
          class="w-full flex items-center justify-between py-2 px-1 text-left hover:bg-gray-50 dark:hover:bg-slate-700/50 rounded-lg transition-colors mb-2"
          @click="scanResultsExpanded = !scanResultsExpanded"
        >
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('security.scan.details') }}
          </span>
          <svg
            :class="['w-5 h-5 text-gray-500 transition-transform', scanResultsExpanded ? 'rotate-180' : '']"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <!-- Results Content -->
        <div v-show="!scanCompleted || scanResultsExpanded" class="space-y-1 max-h-96 overflow-y-auto">
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

    <!-- Companion Monitoring Section -->
    <div class="mt-6 space-y-6">
      <!-- Empty State with Demo -->
      <div v-if="!companionStore.isDemoMode && sessions.length === 0 && !companionStore.loading" class="glass-card p-8">
        <div class="text-center max-w-2xl mx-auto">
          <!-- Icon -->
          <div class="w-16 h-16 mx-auto mb-4 bg-gradient-to-br from-blue-500 to-purple-600 rounded-2xl flex items-center justify-center">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
          </div>
          <!-- Title -->
          <h3 class="text-xl font-semibold text-gray-900 dark:text-white mb-2">{{ t('companion.demo.title') }}</h3>
          <!-- Description -->
          <p class="text-gray-600 dark:text-gray-400 mb-6">{{ t('companion.demo.description') }}</p>
          <!-- Features -->
          <div class="text-left bg-gray-50 dark:bg-slate-700/50 rounded-lg p-4 mb-6">
            <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3">{{ t('companion.demo.features.title') }}</h4>
            <ul class="space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <li class="flex items-center gap-2">
                <svg class="w-4 h-4 text-green-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                {{ t('companion.demo.features.sessions') }}
              </li>
              <li class="flex items-center gap-2">
                <svg class="w-4 h-4 text-green-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                {{ t('companion.demo.features.events') }}
              </li>
              <li class="flex items-center gap-2">
                <svg class="w-4 h-4 text-green-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                {{ t('companion.demo.features.security') }}
              </li>
              <li class="flex items-center gap-2">
                <svg class="w-4 h-4 text-green-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                {{ t('companion.demo.features.replay') }}
              </li>
            </ul>
          </div>
          <!-- Platforms -->
          <p class="text-xs text-gray-500 dark:text-gray-500 mb-6">{{ t('companion.demo.platforms') }}</p>
          <!-- Demo Button -->
          <button
            class="px-6 py-3 bg-accent hover:bg-accent/90 text-white font-medium rounded-lg transition-colors inline-flex items-center gap-2"
            @click="companionStore.startDemo()"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {{ t('companion.demo.runDemo') }}
          </button>
        </div>
      </div>

      <!-- Demo Mode Banner -->
      <div v-if="companionStore.isDemoMode" class="bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg p-4 flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="w-3 h-3 bg-amber-500 rounded-full animate-pulse" />
          <span class="text-amber-800 dark:text-amber-200 font-medium">{{ t('companion.demo.running') }}</span>
        </div>
        <button
          class="px-4 py-2 bg-amber-600 hover:bg-amber-700 text-white text-sm font-medium rounded-lg transition-colors"
          @click="companionStore.stopDemo()"
        >
          {{ t('companion.demo.stopDemo') }}
        </button>
      </div>

      <!-- Companion Stats Cards (show when has data or demo mode) -->
      <div v-if="companionStore.isDemoMode || sessions.length > 0 || companionStore.loading" class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
        <div class="glass-card p-4">
          <div class="flex items-center gap-2 mb-1">
            <span :class="['w-2 h-2 rounded-full', isConnected() ? 'bg-green-500 animate-pulse' : 'bg-gray-400']" />
            <span class="text-xs text-gray-500 dark:text-slate-400">{{ isConnected() ? t('companion.connected') : t('companion.disconnected') }}</span>
          </div>
          <div class="text-2xl font-bold text-green-600 dark:text-green-400">{{ stats?.activeSessions || 0 }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.activeSessions') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ stats?.totalSessions || 0 }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalSessions') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-purple-600 dark:text-purple-400">{{ stats?.totalEvents || 0 }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalEvents') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-orange-600 dark:text-orange-400">{{ stats?.totalAlerts || 0 }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.totalAlerts') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-red-600 dark:text-red-400">{{ unackedAlerts.length }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.unackedAlerts') }}</div>
        </div>
        <div class="glass-card p-4">
          <div class="text-2xl font-bold text-gray-900 dark:text-white">{{ eventCount }}</div>
          <div class="text-sm text-gray-500 dark:text-slate-400">{{ t('companion.realtimeEvents') }}</div>
        </div>
      </div>

      <!-- Two Column Layout: Sessions & Alerts (show when has data or demo mode) -->
      <div v-if="companionStore.isDemoMode || sessions.length > 0 || companionStore.loading" class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Recent Sessions -->
        <div class="glass-card p-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">{{ t('companion.sessions') }}</h3>
          <div v-if="companionStore.loading" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('common.loading') }}</div>
          <div v-else-if="sessions.length === 0" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('companion.noSessions') }}</div>
          <div v-else class="space-y-2">
            <div v-for="session in sessions" :key="session.id" class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg cursor-pointer hover:bg-gray-100 dark:hover:bg-slate-700 transition-colors" @click="selectSession(session)">
              <div class="flex items-center justify-between mb-1">
                <div class="flex items-center gap-2">
                  <span class="w-6 h-6 flex items-center justify-center bg-gray-200 dark:bg-slate-600 rounded text-xs font-bold">{{ getPlatformIcon(session.platform) }}</span>
                  <span class="text-sm font-medium text-gray-900 dark:text-white">{{ session.id.slice(0, 8) }}...</span>
                </div>
                <div class="flex items-center gap-1">
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', getStatusColor(session.status)]">{{ session.status }}</span>
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', getThreatColor(session.threat_level)]">{{ session.threat_level }}</span>
                </div>
              </div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ session.event_count }} {{ t('companion.events') }} · {{ formatDate(session.started_at) }}</div>
            </div>
          </div>
        </div>

        <!-- Recent Alerts -->
        <div class="glass-card p-4">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
            {{ t('companion.alerts.label') }}
            <span v-if="unackedAlerts.length > 0" class="ml-2 px-1.5 py-0.5 bg-red-500 text-white text-xs rounded-full">{{ unackedAlerts.length }}</span>
          </h3>
          <div v-if="companionStore.loadingAlerts" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('common.loading') }}</div>
          <div v-else-if="alerts.length === 0" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('companion.noAlerts') }}</div>
          <div v-else class="space-y-2">
            <div v-for="alert in alerts" :key="alert.id" :class="['p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg', alert.acknowledged ? 'opacity-60' : '']">
              <div class="flex items-start justify-between mb-1">
                <div class="flex-1 min-w-0">
                  <div class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ getAlertTitle(alert.title) }}</div>
                  <div class="text-xs text-gray-500 dark:text-slate-400 truncate">{{ getAlertDescription(alert.description) }}</div>
                </div>
                <div class="flex items-center gap-1 ml-2">
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', alert.severity === 'critical' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' : alert.severity === 'error' ? 'bg-orange-100 dark:bg-orange-900/50 text-orange-700 dark:text-orange-300' : alert.severity === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300' : 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300']">{{ t(`companion.alerts.${alert.severity}`) }}</span>
                  <button v-if="!alert.acknowledged" class="px-1.5 py-0.5 bg-accent hover:bg-accent-hover text-white text-xs rounded transition-colors" @click.stop="acknowledgeAlert(alert.id)">{{ t('companion.acknowledge') }}</button>
                </div>
              </div>
              <div class="text-xs text-gray-400 dark:text-slate-500">{{ formatDate(alert.createdAt) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Realtime Events (show when has data or demo mode) -->
      <div v-if="companionStore.isDemoMode || sessions.length > 0 || companionStore.loading" class="glass-card p-4">
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          {{ t('companion.realtime') }}
          <span v-if="isConnected()" class="ml-2 w-2 h-2 bg-green-500 rounded-full inline-block animate-pulse" />
        </h3>
        <div v-if="realtimeEvents.length === 0" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('companion.waitingForEvents') }}</div>
        <div v-else class="space-y-2 max-h-96 overflow-y-auto">
          <div v-for="event in realtimeEvents" :key="event.id" class="p-3 bg-gray-50 dark:bg-slate-700/50 rounded-lg text-sm">
            <!-- Header: Event type, session, time -->
            <div class="flex items-center gap-2 mb-2">
              <span class="text-base">{{ getEventTypeIcon(event.eventType) }}</span>
              <span class="font-medium text-gray-900 dark:text-white">{{ event.eventType.replace(/_/g, ' ') }}</span>
              <span v-if="event.security" :class="['px-1.5 py-0.5 rounded text-xs font-medium', getThreatColor(event.security.threatLevel)]">
                {{ event.security.threatLevel }}
              </span>
              <span class="text-xs text-gray-400 dark:text-slate-500">{{ event.sessionId.slice(0, 8) }}...</span>
              <span class="ml-auto text-xs text-gray-400 dark:text-slate-500 whitespace-nowrap">{{ formatRelativeTime(event.timestamp) }}</span>
            </div>
            <!-- Event Content -->
            <div class="pl-6 space-y-1">
              <!-- Security Event Details -->
              <div v-if="event.security" class="text-xs">
                <div class="flex items-center gap-2 mb-1">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('companion.security.score') }}:</span>
                  <span class="font-medium" :class="event.security.threatScore >= 75 ? 'text-red-600 dark:text-red-400' : event.security.threatScore >= 50 ? 'text-orange-600 dark:text-orange-400' : event.security.threatScore >= 25 ? 'text-yellow-600 dark:text-yellow-400' : 'text-green-600 dark:text-green-400'">{{ event.security.threatScore }}/100</span>
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', event.security.action === 'blocked' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' : event.security.action === 'filtered' ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300' : 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300']">{{ event.security.action }}</span>
                </div>
                <div v-if="event.security.threatTypes?.length" class="text-gray-600 dark:text-slate-300">
                  <span class="text-gray-500 dark:text-slate-400">{{ t('companion.security.threatTypes') }}:</span>
                  {{ event.security.threatTypes.map(type => type.replace(/_/g, ' ')).join(', ') }}
                </div>
                <div v-if="event.security.details" class="text-gray-500 dark:text-slate-400 mt-1 truncate" :title="event.security.details">
                  {{ event.security.details }}
                </div>
              </div>
              <!-- Message Event Details -->
              <div v-else-if="event.message" class="text-xs">
                <div class="flex items-center gap-2">
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', event.message.direction === 'inbound' ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300' : 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300']">
                    {{ event.message.direction === 'inbound' ? '← ' + t('companion.eventDetails.received') : '→ ' + t('companion.eventDetails.sent') }}
                  </span>
                  <span class="text-gray-500 dark:text-slate-400">{{ event.message.contentType }}</span>
                  <span class="text-gray-400 dark:text-slate-500">{{ event.message.length }} chars</span>
                </div>
                <div v-if="event.message.content" class="text-gray-600 dark:text-slate-300 mt-1 truncate" :title="event.message.content">
                  {{ event.message.content }}
                </div>
              </div>
              <!-- Tool Call Event Details -->
              <div v-else-if="event.toolCall" class="text-xs">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{ event.toolCall.toolName }}</span>
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', event.toolCall.status === 'success' ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' : event.toolCall.status === 'error' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300']">{{ event.toolCall.status }}</span>
                  <span v-if="event.toolCall.sandboxUsed" class="px-1.5 py-0.5 rounded text-xs font-medium bg-cyan-100 dark:bg-cyan-900/50 text-cyan-700 dark:text-cyan-300">sandbox</span>
                  <span class="text-gray-400 dark:text-slate-500">{{ formatDuration(event.toolCall.duration) }}</span>
                </div>
                <div v-if="event.toolCall.inputPreview" class="text-gray-500 dark:text-slate-400 mt-1 truncate" :title="event.toolCall.inputPreview">
                  {{ t('companion.eventDetails.input') }}: {{ event.toolCall.inputPreview }}
                </div>
              </div>
              <!-- LLM Request Event Details -->
              <div v-else-if="event.llmRequest" class="text-xs">
                <div class="flex items-center gap-2">
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{ event.llmRequest.provider }}/{{ event.llmRequest.model }}</span>
                  <span :class="['px-1.5 py-0.5 rounded text-xs font-medium', event.llmRequest.status === 'success' ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' : 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300']">{{ event.llmRequest.status }}</span>
                  <span class="text-gray-400 dark:text-slate-500">{{ formatDuration(event.llmRequest.duration) }}</span>
                </div>
                <div class="text-gray-500 dark:text-slate-400 mt-1">
                  {{ t('companion.eventDetails.tokens') }}: {{ event.llmRequest.promptTokens }} → {{ event.llmRequest.completionTokens }} ({{ event.llmRequest.totalTokens }} total)
                </div>
              </div>
              <!-- Error Event Details -->
              <div v-else-if="event.error" class="text-xs text-red-600 dark:text-red-400">
                {{ event.error }}
              </div>
              <!-- Generic Event (session_start, session_end, etc.) -->
              <div v-else class="text-xs text-gray-500 dark:text-slate-400">
                {{ event.platform }} · {{ event.userId.slice(0, 8) }}...
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Session Detail Modal -->
    <div v-if="showSessionDetail && selectedSession" class="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4" @click.self="closeSessionDetail">
      <div class="bg-white dark:bg-slate-800 rounded-xl max-w-2xl w-full max-h-[80vh] overflow-hidden flex flex-col">
        <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-slate-700">
          <div>
            <h2 class="text-lg font-bold text-gray-900 dark:text-white">{{ t('companion.sessionDetail') }}</h2>
            <p class="text-sm text-gray-500 dark:text-slate-400 font-mono">{{ selectedSession.id }}</p>
          </div>
          <div class="flex items-center gap-2">
            <button
              class="px-3 py-1.5 text-sm bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 text-red-700 dark:text-red-300 rounded-lg transition-colors"
              @click="deleteSession(selectedSession.id)"
            >
              {{ t('common.delete') }}
            </button>
            <button class="p-2 hover:bg-gray-100 dark:hover:bg-slate-700 rounded-lg transition-colors" @click="closeSessionDetail">
              <span class="text-xl">&times;</span>
            </button>
          </div>
        </div>
        <div class="flex-1 overflow-y-auto p-4">
          <!-- Basic Info -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-4">
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.platform') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.platform }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.status') }}</div>
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', getStatusColor(selectedSession.status)]">{{ selectedSession.status }}</span>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.threatLevel') }}</div>
              <span :class="['px-2 py-0.5 rounded text-xs font-medium', getThreatColor(selectedSession.threat_level)]">{{ selectedSession.threat_level }}</span>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.events') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.event_count }}</div>
            </div>
          </div>

          <!-- Extended Info -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-4 p-3 bg-gray-50 dark:bg-slate-700/30 rounded-lg">
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.threatScore') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ selectedSession.threat_score }}/100</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.userId') }}</div>
              <div class="font-medium text-gray-900 dark:text-white truncate" :title="selectedSession.user_id">{{ selectedSession.user_id }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.startedAt') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatDate(selectedSession.started_at) }}</div>
            </div>
            <div>
              <div class="text-xs text-gray-500 dark:text-slate-400">{{ t('companion.duration') }}</div>
              <div class="font-medium text-gray-900 dark:text-white">{{ formatDuration(selectedSession.duration) }}</div>
            </div>
          </div>

          <!-- Metadata -->
          <div v-if="selectedSession.metadata" class="mb-4 p-3 bg-gray-50 dark:bg-slate-700/30 rounded-lg">
            <h4 class="text-xs font-medium text-gray-500 dark:text-slate-400 mb-2">{{ t('companion.metadata') }}</h4>
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
              <div>
                <span class="text-gray-500 dark:text-slate-400">{{ t('companion.messageCount') }}:</span>
                <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ selectedSession.metadata.message_count || 0 }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-slate-400">{{ t('companion.toolCallCount') }}:</span>
                <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ selectedSession.metadata.tool_call_count || 0 }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-slate-400">{{ t('companion.llmCallCount') }}:</span>
                <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ selectedSession.metadata.llm_call_count || 0 }}</span>
              </div>
              <div>
                <span class="text-gray-500 dark:text-slate-400">{{ t('companion.totalTokens') }}:</span>
                <span class="ml-1 font-medium text-gray-900 dark:text-white">{{ selectedSession.metadata.total_tokens || 0 }}</span>
              </div>
            </div>
          </div>

          <!-- Event History -->
          <h3 class="text-sm font-medium text-gray-900 dark:text-white mb-2">{{ t('companion.eventHistory') }}</h3>
          <div v-if="companionStore.loadingEvents" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('common.loading') }}</div>
          <div v-else-if="companionStore.sessionEvents.length === 0" class="text-center py-4 text-gray-500 dark:text-slate-400">{{ t('companion.noEvents') }}</div>
          <div v-else class="space-y-2 max-h-64 overflow-y-auto">
            <div v-for="event in companionStore.sessionEvents" :key="event.id" class="p-2 bg-gray-50 dark:bg-slate-700/50 rounded text-sm">
              <div class="flex items-center gap-2 mb-1">
                <span>{{ getEventTypeIcon(event.eventType) }}</span>
                <span class="font-medium text-gray-900 dark:text-white">{{ event.eventType.replace(/_/g, ' ') }}</span>
                <span v-if="event.status" :class="['px-1.5 py-0.5 rounded text-xs', event.status === 'success' ? 'bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-300' : 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300']">{{ event.status }}</span>
                <span class="ml-auto text-xs text-gray-400 dark:text-slate-500">{{ formatDate(event.timestamp) }}</span>
              </div>
              <!-- Event Details -->
              <div v-if="event.message" class="pl-6 text-xs text-gray-600 dark:text-slate-300">
                <span :class="event.message.direction === 'inbound' ? 'text-blue-600 dark:text-blue-400' : 'text-purple-600 dark:text-purple-400'">
                  {{ event.message.direction === 'inbound' ? '←' : '→' }}
                </span>
                {{ event.message.contentType }} ({{ event.message.length }} chars)
              </div>
              <div v-if="event.toolCall" class="pl-6 text-xs text-gray-600 dark:text-slate-300">
                {{ event.toolCall.toolName }} - {{ event.toolCall.status }} ({{ event.toolCall.duration }}ms)
              </div>
              <div v-if="event.llmRequest" class="pl-6 text-xs text-gray-600 dark:text-slate-300">
                {{ event.llmRequest.provider }}/{{ event.llmRequest.model }} - {{ event.llmRequest.totalTokens }} tokens
              </div>
              <div v-if="event.security" class="pl-6 text-xs">
                <span :class="getThreatColor(event.security.threatLevel)">{{ event.security.threatLevel }}</span>
                <span class="text-gray-500 dark:text-slate-400 ml-2">{{ event.security.action }}</span>
              </div>
              <div v-if="event.error" class="pl-6 text-xs text-red-600 dark:text-red-400">
                {{ event.error }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
