<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi } from '@/api/security'
import { companionApi, type CompanionSession, type Stats as CompanionStats } from '@/api/companion'
import { getActiveConnections, getConnectionStats, type Connection, type ConnectionStats } from '@/api/connections'
import SessionList from '@/components/companion/SessionList.vue'
import SessionDetail from '@/components/companion/SessionDetail.vue'
import FixPreviewDialog from '@/components/security/FixPreviewDialog.vue'

const { t, te } = useI18n()

// Tab definitions
type TabId = 'overview' | 'monitoring'
const activeTab = ref<TabId>('overview')

const tabs: { id: TabId; labelKey: string; icon: string }[] = [
  { id: 'overview', labelKey: 'security.tabs.overview', icon: 'shield' },
  { id: 'monitoring', labelKey: 'security.tabs.monitoring', icon: 'activity' },
]

/** Backend English details string -> security.scan.detailMessages key (for i18n). */
const DETAIL_MESSAGE_KEYS: Record<string, string> = {
  'Threat detector is not initialized': 'threat_detector_not_initialized',
  'XSS pattern detection is enabled in threat detector': 'xss_detection_enabled',
  'SQL injection pattern detection is enabled': 'sql_injection_enabled',
  'Command injection pattern detection is enabled': 'command_injection_enabled',
  'Prompt injection detection is active': 'prompt_injection_active',
  'Prompt injection protection is disabled. Enable PromptGuard for AI security.': 'prompt_injection_disabled',
  'AI output validation is enabled': 'ai_output_validation_enabled',
  'AI output validation is disabled. Consider enabling for safer AI operations.': 'ai_output_validation_disabled',
  'Model whitelist is enabled but no models are configured': 'model_whitelist_no_models',
  'Model whitelist is disabled. All models are accessible. Consider enabling for production.': 'model_whitelist_disabled',
  'Sensitive data filtering is enabled': 'sensitive_data_filtering_enabled',
  'Sensitive data filtering is disabled. PII may be exposed to AI models.': 'sensitive_data_filtering_disabled',
  'Rate limiting is disabled. API is vulnerable to abuse and DoS attacks.': 'rate_limiting_disabled',
  'CORS allows all origins in production. This is a security risk.': 'cors_all_origins_production',
  'CORS allows all origins. Acceptable for development, but restrict in production.': 'cors_all_origins_dev',
  'CORS is configured with no external origins allowed': 'cors_no_external_origins',
  'TLS is enabled with minimum version TLS 1.2': 'tls_12_min',
  'TLS is enabled but allows older versions. Recommend TLS 1.2 minimum.': 'tls_older_versions',
  'TLS is disabled in production. All traffic is unencrypted.': 'tls_disabled_production',
  'TLS is disabled. Enable for production deployment.': 'tls_disabled_enable',
  'Server is accessible on localhost': 'server_localhost',
  'Could not verify server binding': 'server_binding_unknown',
  'Sandbox execution is enabled': 'sandbox_enabled',
  'Sandbox is disabled. Code execution is not isolated.': 'sandbox_disabled',
  'No memory limit configured for sandbox': 'no_memory_limit',
  'No execution timeout configured': 'no_timeout_configured',
  'Network access is disabled in sandbox': 'network_disabled_sandbox',
  'Network access is enabled in sandbox. Consider disabling for better isolation.': 'network_enabled_sandbox',
  'Data directory has restricted permissions': 'data_dir_restricted',
  'Data directory may have overly permissive access': 'data_dir_permissive',
  'Could not verify data directory permissions': 'data_dir_unknown',
  'Debug mode is enabled in production. This exposes sensitive information.': 'debug_production',
  'Debug mode is enabled. Disable before production deployment.': 'debug_enabled',
  'Debug mode is disabled': 'debug_disabled',
  'Detailed error messages are exposed in production. This may leak sensitive information.': 'error_exposed_production',
  'Detailed error messages are exposed. Disable before production deployment.': 'error_exposed',
  'Error details are hidden from responses': 'error_hidden',
  'Sensitive error data may be logged. Ensure log access is restricted.': 'error_log_restrict',
  'Sensitive error data is filtered from logs': 'error_filtered_logs',
  'Running in production mode': 'running_production',
  'Running in staging mode': 'running_staging',
}

/** Use translated details when key exists (item id+status or detailMessages map), else API details. */
function getItemDetails(item: ScanItem): string | undefined {
  if (!item.details) return undefined
  const itemKey = `security.scan.items.${item.id}.details.${item.status}`
  if (te(itemKey)) return t(itemKey)
  const msgKey = DETAIL_MESSAGE_KEYS[item.details]
  if (msgKey) {
    const fullKey = `security.scan.detailMessages.${msgKey}`
    if (te(fullKey)) return t(fullKey)
  }
  return item.details
}

// Security scan data
interface ScanItem {
  id: string
  category: string
  name: string
  description: string
  status: 'pending' | 'scanning' | 'passed' | 'warning' | 'failed'
  details?: string
  risk?: string        // Why this is a security concern
  impact?: string      // What could happen if exploited
  remediation?: string // How to fix the issue
  auto_fixable?: boolean
  fix_action?: string
}

const isScanning = ref(false)
const scanProgress = ref(0)
const scanResults = ref<ScanItem[]>([])
const scanCompleted = ref(false)
const scanResultsExpanded = ref(true)
const fixingItem = ref<string | null>(null) // ID of item being fixed
const fixPreviewVisible = ref(false)
const fixPreviewItem = ref<ScanItem | null>(null)
const expandedItemId = ref<string | null>(null) // ID of expanded item for details

// Check if scan should run (once per day)
function shouldRunScan(): boolean {
  const lastScanKey = 'security_last_scan_timestamp'
  const lastScanStr = localStorage.getItem(lastScanKey)

  if (!lastScanStr) {
    return true // Never scanned before
  }

  const lastScan = new Date(lastScanStr)
  const now = new Date()
  const hoursSinceLastScan = (now.getTime() - lastScan.getTime()) / (1000 * 60 * 60)

  return hoursSinceLastScan >= 24
}

// Save scan timestamp and results
function saveScanTimestamp() {
  const lastScanKey = 'security_last_scan_timestamp'
  const lastScanResultsKey = 'security_last_scan_results'
  localStorage.setItem(lastScanKey, new Date().toISOString())
  localStorage.setItem(lastScanResultsKey, JSON.stringify(scanResults.value))
}

// Load cached scan results
function loadCachedScanResults() {
  const lastScanResultsKey = 'security_last_scan_results'
  const cachedResultsStr = localStorage.getItem(lastScanResultsKey)

  if (cachedResultsStr) {
    try {
      const cachedResults = JSON.parse(cachedResultsStr)
      scanResults.value = cachedResults
      scanProgress.value = 100
      scanCompleted.value = true
      // Auto-collapse if no issues
      const warnings = scanResults.value.filter(r => r.status === 'warning').length
      const failed = scanResults.value.filter(r => r.status === 'failed').length
      if (warnings === 0 && failed === 0) {
        scanResultsExpanded.value = false
      }
    } catch (error) {
      console.error('Failed to load cached scan results:', error)
    }
  }
}

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
      if (!apiItem) continue

      // Add item with scanning status first
      const scanItem: ScanItem = {
        id: apiItem.id,
        category: apiItem.category,
        name: apiItem.name,
        description: apiItem.description,
        status: 'scanning',
        auto_fixable: apiItem.auto_fixable,
        fix_action: apiItem.fix_action,
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

  // Save scan timestamp
  saveScanTimestamp()

  // Auto-collapse if no issues
  if (scanSummary.value.warnings === 0 && scanSummary.value.failed === 0) {
    scanResultsExpanded.value = false
  }
}

// Fix a scan issue - show preview first
function showFixPreview(item: ScanItem) {
  if (!item.auto_fixable || !item.fix_action || fixingItem.value) return
  fixPreviewItem.value = item
  fixPreviewVisible.value = true
}

// Confirm fix from preview dialog
async function confirmFix(fixAction: string) {
  fixPreviewVisible.value = false
  const item = fixPreviewItem.value
  if (!item) return

  fixingItem.value = item.id

  try {
    const response = await securityApi.fixScanIssue(fixAction)
    if (response.data.success) {
      item.status = 'passed'
      item.details = response.data.message
      item.auto_fixable = false
      item.fix_action = undefined
      saveScanTimestamp()
    } else {
      item.details = response.data.message
    }
  } catch (error: unknown) {
    console.error('Failed to fix issue:', error)
    const errorMessage = error instanceof Error ? error.message : t('security.scan.fixError')
    item.details = errorMessage
  } finally {
    fixingItem.value = null
    fixPreviewItem.value = null
  }
}

// Legacy direct fix (used by fixAll)
async function fixScanIssue(item: ScanItem) {
  if (!item.auto_fixable || !item.fix_action || fixingItem.value) return

  fixingItem.value = item.id

  try {
    const response = await securityApi.fixScanIssue(item.fix_action)
    if (response.data.success) {
      // Update item status
      item.status = 'passed'
      item.details = response.data.message
      item.auto_fixable = false
      item.fix_action = undefined
      // Save updated results
      saveScanTimestamp()
    } else {
      item.details = response.data.message
    }
  } catch (error: unknown) {
    console.error('Failed to fix issue:', error)
    const errorMessage = error instanceof Error ? error.message : t('security.scan.fixError')
    item.details = errorMessage
  } finally {
    fixingItem.value = null
  }
}

// Count of fixable issues
const fixableCount = computed(() => {
  return scanResults.value.filter(r => r.auto_fixable && (r.status === 'warning' || r.status === 'failed')).length
})

// Fix all fixable issues
async function fixAllIssues() {
  const fixableItems = scanResults.value.filter(r => r.auto_fixable && (r.status === 'warning' || r.status === 'failed'))
  for (const item of fixableItems) {
    await fixScanIssue(item)
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
function getScanItemName(item: ScanItem): string {
  const key = `security.scan.items.${item.id}.name`
  const translated = t(key)
  // If translation key doesn't exist, fall back to API-provided name (or id)
  return translated === key ? (item.name || item.id) : translated
}

// Get scan item description with i18n
function getScanItemDescription(item: ScanItem): string {
  const key = `security.scan.items.${item.id}.description`
  const translated = t(key)
  return translated === key ? (item.description || '') : translated
}

// Get scan item status icon and color
function getScanStatusClass(status: string): string {
  switch (status) {
    case 'passed': return 'text-green-700 dark:text-green-500'
    case 'warning': return 'text-yellow-500'
    case 'failed': return 'text-red-500'
    case 'scanning': return 'text-gray-900 dark:text-white animate-pulse'
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

// Companion monitoring state
const companionSessions = ref<CompanionSession[]>([])
const companionStats = ref<CompanionStats | null>(null)
const selectedSession = ref<CompanionSession | null>(null)
const companionLoading = ref(false)
const companionError = ref('')
const companionHasMore = ref(false)
const companionOffset = ref(0)
const companionLimit = 10

async function fetchCompanionSessions(append = false) {
  companionLoading.value = true
  companionError.value = ''
  try {
    const res = await companionApi.listSessions({ offset: companionOffset.value, limit: companionLimit })
    if (append) {
      companionSessions.value = [...companionSessions.value, ...(res.data.sessions || [])]
    } else {
      companionSessions.value = res.data.sessions || []
    }
    companionHasMore.value = (res.data.sessions?.length || 0) >= companionLimit
  } catch {
    companionError.value = t('companion.fetchError')
  } finally {
    companionLoading.value = false
  }
}

async function fetchCompanionStats() {
  try {
    const res = await companionApi.getStats()
    companionStats.value = res.data
  } catch {
    // Ignore stats error
  }
}

function loadMoreSessions() {
  companionOffset.value += companionLimit
  fetchCompanionSessions(true)
}

function selectSession(session: CompanionSession) {
  selectedSession.value = session
}

function closeSessionDetail() {
  selectedSession.value = null
}

// Connection monitoring state
const connections = ref<Connection[]>([])
const connectionStats = ref<ConnectionStats | null>(null)
const connectionLoading = ref(false)
const connectionExpanded = ref(false)

async function fetchConnections() {
  connectionLoading.value = true
  try {
    const [connResponse, statsResponse] = await Promise.all([
      getActiveConnections(),
      getConnectionStats()
    ])
    connections.value = connResponse.connections || []
    connectionStats.value = statsResponse
  } catch {
    // Ignore error
  } finally {
    connectionLoading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

let companionRefreshInterval: ReturnType<typeof setInterval> | null = null
let connectionRefreshInterval: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  // Load cached results first
  loadCachedScanResults()

  // Auto-start security scan only if 24 hours have passed
  if (shouldRunScan()) {
    startSecurityScan()
  }

  // Fetch companion data
  fetchCompanionSessions()
  fetchCompanionStats()
  companionRefreshInterval = setInterval(() => {
    fetchCompanionStats()
  }, 30000)

  // Fetch connection data
  fetchConnections()
  connectionRefreshInterval = setInterval(fetchConnections, 5000)
})

onUnmounted(() => {
  if (companionRefreshInterval) clearInterval(companionRefreshInterval)
  if (connectionRefreshInterval) clearInterval(connectionRefreshInterval)
})

</script>

<template>
  <div class="security-view p-4 sm:p-6 max-w-7xl mx-auto">
    <!-- Header with Title -->
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('security.title') }}</h1>

    <!-- Tab Navigation -->
    <div class="mb-6 border-b border-gray-200 dark:border-gray-700">
      <nav class="flex space-x-4" aria-label="Tabs">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :class="[
            'px-4 py-2 text-sm font-medium rounded-t-lg transition-colors flex items-center gap-2',
            activeTab === tab.id
              ? 'bg-white dark:bg-slate-800 text-gray-900 dark:text-gray-300 border-b-2 border-gray-900 dark:border-gray-700 -mb-px'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:bg-gray-50 dark:hover:bg-slate-700/50'
          ]"
          @click="activeTab = tab.id"
        >
          <!-- Shield icon for Overview -->
          <svg v-if="tab.icon === 'shield'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          <!-- Activity icon for Monitoring -->
          <svg v-else-if="tab.icon === 'activity'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <!-- List icon for Events -->
          <svg v-else-if="tab.icon === 'list'" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
          </svg>
          {{ t(tab.labelKey) }}
        </button>
      </nav>
    </div>

    <!-- Tab Content: Overview -->
    <div v-show="activeTab === 'overview'">
      <!-- Security Status Banner -->
      <div class="mb-6">
      <div
:class="[
        'rounded-lg p-4 flex items-center justify-between',
        securityStatus === 'passed'
          ? 'bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800'
          : securityStatus === 'warning'
            ? 'bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800'
            : securityStatus === 'failed'
              ? 'bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800'
              : 'bg-gray-100 dark:bg-gray-700/30 border border-gray-200 dark:border-gray-600'
      ]">
        <div class="flex items-center gap-3">
          <div
:class="[
            'w-10 h-10 rounded-full flex items-center justify-center',
            securityStatus === 'passed' ? 'bg-green-600' :
            securityStatus === 'warning' ? 'bg-yellow-500' :
            securityStatus === 'failed' ? 'bg-red-500' : 'bg-gray-700 dark:bg-gray-500'
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
            <h2
:class="[
              'text-lg font-semibold',
              securityStatus === 'passed' ? 'text-green-900 dark:text-green-400' :
              securityStatus === 'warning' ? 'text-yellow-800 dark:text-yellow-200' :
              securityStatus === 'failed' ? 'text-red-800 dark:text-red-200' :
              'text-gray-900 dark:text-white dark:text-white'
            ]">
              {{ securityStatus === 'passed' ? t('security.statusSecure') :
                 securityStatus === 'warning' ? t('security.statusWarning') :
                 securityStatus === 'failed' ? t('security.statusFailed') :
                 t('security.statusScanning') }}
            </h2>
            <p
:class="[
              'text-sm',
              securityStatus === 'passed' ? 'text-green-700 dark:text-green-300' :
              securityStatus === 'warning' ? 'text-yellow-600 dark:text-yellow-400' :
              securityStatus === 'failed' ? 'text-red-600 dark:text-red-400' :
              'text-gray-900 dark:text-white dark:text-white'
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
        <div class="flex items-center gap-2">
          <!-- Fix All Button -->
          <button
            v-if="fixableCount > 0 && !isScanning"
            :disabled="!!fixingItem"
            class="px-4 py-2 rounded-lg text-white font-medium transition-all flex items-center gap-2 bg-green-500 hover:bg-green-600 disabled:opacity-50"
            @click="fixAllIssues"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
            {{ t('security.scan.fixAll') }} ({{ fixableCount }})
          </button>
          <!-- Scan Button -->
          <button
            :disabled="isScanning"
            :class="[
              'px-4 py-2 rounded-lg text-white font-medium transition-all flex items-center gap-2',
              isScanning
                ? 'bg-gray-400 cursor-not-allowed'
                : 'bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400'
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
            class="h-full bg-gradient-to-r from-blue-500 dark:from-blue-400 to-green-500 dark:to-green-400 transition-all duration-300 ease-out"
            :style="{ width: `${scanProgress}%` }"
          ></div>
        </div>
      </div>

      <!-- Scan Summary -->
      <div v-if="scanCompleted" class="grid grid-cols-3 gap-4 mb-4">
        <div class="bg-green-50 dark:bg-green-900/20 rounded-lg p-3 text-center">
          <div class="text-2xl font-bold text-green-700 dark:text-green-400">{{ scanSummary.passed }}</div>
          <div class="text-xs text-green-600 dark:text-green-300">{{ t('security.scan.passed') }}</div>
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
            v-if="index === 0 || scanResults[index - 1]?.category !== item.category"
            class="text-xs font-semibold text-gray-500 dark:text-slate-400 uppercase tracking-wider pt-3 pb-1"
          >
            {{ getCategoryLabel(item.category) }}
          </div>
          <!-- Scan Item -->
          <div
            :class="[
              'rounded-lg transition-all duration-200 cursor-pointer',
              item.status === 'scanning' ? 'bg-gray-100 dark:bg-gray-700/30' : 'hover:bg-gray-50 dark:hover:bg-slate-700/50',
              expandedItemId === item.id ? 'bg-gray-100 dark:bg-gray-700/30' : ''
            ]"
            @click="expandedItemId = expandedItemId === item.id ? null : item.id"
          >
            <div class="flex items-center gap-3 py-2 px-3">
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
                <div class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ getScanItemName(item) }}</div>
                <div class="text-xs text-gray-500 dark:text-slate-400 truncate">{{ getScanItemDescription(item) }}</div>
              </div>
              <!-- Status Badge & Expand Icon -->
              <div v-if="item.status !== 'pending'" class="flex-shrink-0 flex items-center gap-2">
                <span
                  :class="[
                    'px-2 py-0.5 text-xs rounded-full font-medium',
                    item.status === 'passed' ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300' :
                    item.status === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300' :
                    item.status === 'failed' ? 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300' :
                    'bg-gray-100 dark:bg-gray-700/30 text-gray-600 dark:text-gray-400'
                  ]"
                >
                  {{ item.status === 'scanning' ? t('security.scan.checking') :
                     item.status === 'passed' ? t('security.scan.passed') :
                     item.status === 'warning' ? t('security.scan.warnings') :
                     item.status === 'failed' ? t('security.scan.failed') : item.status }}
                </span>
                <!-- Fix Button -->
                <button
                  v-if="item.auto_fixable && (item.status === 'warning' || item.status === 'failed')"
                  :disabled="fixingItem === item.id"
                  class="px-2 py-0.5 text-xs rounded-full font-medium bg-gray-100 dark:bg-gray-700/30 text-gray-900 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700/40 transition-colors disabled:opacity-50"
                  @click.stop="showFixPreview(item)"
                >
                  <span v-if="fixingItem === item.id" class="flex items-center gap-1">
                    <svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
                      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                  </span>
                  <span v-else>{{ t('security.scan.fix') }}</span>
                </button>
                <!-- Manual Fix Badge -->
                <span
                  v-else-if="!item.auto_fixable && (item.status === 'warning' || item.status === 'failed') && item.remediation"
                  class="px-2 py-0.5 text-xs rounded-full font-medium bg-orange-100 dark:bg-orange-900/30 text-orange-600 dark:text-orange-400"
                >
                  {{ t('security.scan.manualFix') }}
                </span>
                <!-- Expand Icon -->
                <svg
                  v-if="item.status !== 'scanning' && (item.risk || item.impact || item.remediation || item.details)"
                  :class="['w-4 h-4 text-gray-400 transition-transform', expandedItemId === item.id ? 'rotate-180' : '']"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </div>
            </div>
            <!-- Expanded Details -->
            <div
              v-if="expandedItemId === item.id && (item.risk || item.impact || item.remediation || item.details)"
              class="px-3 pb-3 pt-1 ml-8 border-l-2 border-gray-200 dark:border-slate-600"
            >
              <div v-if="getItemDetails(item)" class="text-xs text-gray-600 dark:text-slate-300 mb-2">
                <span class="font-medium">{{ t('security.scan.details') }}:</span> {{ getItemDetails(item) }}
              </div>
              <div v-if="item.risk" class="text-xs text-gray-600 dark:text-slate-300 mb-2">
                <span class="font-medium text-orange-600 dark:text-orange-400">{{ t('security.scan.risk') }}:</span> {{ item.risk }}
              </div>
              <div v-if="item.impact" class="text-xs text-gray-600 dark:text-slate-300 mb-2">
                <span class="font-medium text-red-600 dark:text-red-400">{{ t('security.scan.impact') }}:</span> {{ item.impact }}
              </div>
              <div v-if="item.remediation" class="text-xs text-gray-600 dark:text-slate-300">
                <span class="font-medium text-green-700 dark:text-green-300">{{ t('security.scan.remediation') }}:</span> {{ item.remediation }}
              </div>
            </div>
          </div>
        </template>
        </div>
      </div>
      </div>
    </div>

    <!-- Tab Content: Monitoring -->
    <div v-show="activeTab === 'monitoring'">
            <!-- Connection Monitoring Section -->
      <div class="glass-card p-6">
        <!-- Header with toggle -->
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('connections.activeConnections') }}
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('connections.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              :disabled="connectionLoading"
              class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
              @click="fetchConnections()"
            >
              {{ t('common.refresh') }}
            </button>
            <button
              class="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
              @click="connectionExpanded = !connectionExpanded"
            >
              <svg
                :class="['w-5 h-5 text-gray-500 transition-transform', connectionExpanded ? 'rotate-180' : '']"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
          </div>
        </div>

        <!-- Stats Summary -->
        <div v-if="connectionStats" class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="text-lg font-bold text-gray-900 dark:text-white">{{ connectionStats.total_connections ?? 0 }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('connections.total') }}</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="flex items-center justify-center gap-1">
              <span class="w-2 h-2 rounded-full bg-gray-700 dark:bg-gray-500"></span>
              <span class="text-lg font-bold text-gray-900 dark:text-white dark:text-white">{{ connectionStats.active_http ?? 0 }}</span>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">HTTP</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="flex items-center justify-center gap-1">
              <span class="w-2 h-2 rounded-full bg-green-400"></span>
              <span class="text-lg font-bold text-green-400 dark:text-green-400">{{ connectionStats.active_websocket ?? 0 }}</span>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">WebSocket</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="flex items-center justify-center gap-1">
              <span class="w-2 h-2 rounded-full bg-purple-500"></span>
              <span class="text-lg font-bold text-purple-600 dark:text-purple-400">{{ connectionStats.active_sse ?? 0 }}</span>
            </div>
            <div class="text-xs text-gray-500 dark:text-gray-400">SSE</div>
          </div>
        </div>

        <!-- Expanded Content -->
        <div v-show="connectionExpanded" class="border-t border-gray-200 dark:border-gray-700 pt-4">
          <!-- Traffic Stats -->
          <div class="grid grid-cols-2 gap-3 mb-4">
            <div class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <div class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ formatBytes(connectionStats?.total_bytes_sent ?? 0) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('connections.bytesSent') }}</div>
            </div>
            <div class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
              <div class="text-sm font-semibold text-gray-900 dark:text-white">
                {{ formatBytes(connectionStats?.total_bytes_recv ?? 0) }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('connections.bytesRecv') }}</div>
            </div>
          </div>

          <!-- Connection List -->
          <div class="max-h-64 overflow-y-auto space-y-2">
            <div v-if="connections.length === 0" class="text-center py-4 text-gray-500 dark:text-gray-400">
              {{ t('connections.noConnections') }}
            </div>
            <div
              v-for="conn in connections"
              :key="conn.id"
              class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
            >
              <div class="flex items-center justify-between mb-1">
                <div class="flex items-center gap-2">
                  <span :class="['w-2 h-2 rounded-full', conn.status === 'active' ? 'bg-green-400' : 'bg-gray-400']"></span>
                  <span
:class="[
                    'px-2 py-0.5 rounded text-xs font-medium uppercase',
                    conn.type === 'http' ? 'bg-gray-700 dark:bg-gray-500/50 text-gray-900 dark:text-white dark:text-white' :
                    conn.type === 'websocket' ? 'bg-green-50 dark:bg-green-900/30 text-green-200 dark:text-green-200' :
                    'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300'
                  ]">
                    {{ conn.type }}
                  </span>
                  <span class="text-xs font-mono text-gray-600 dark:text-gray-300 truncate max-w-[200px]">
                    {{ conn.method }} {{ conn.path }}
                  </span>
                </div>
              </div>
              <div class="flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ conn.client_ip }}</span>
                <span>{{ formatBytes(conn.bytes_sent) }} ↑</span>
                <span>{{ formatBytes(conn.bytes_recv) }} ↓</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- AI Agent Session Monitoring Section (Companion) -->
      <div class="glass-card p-6 mt-6">
        <!-- Header with toggle -->
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('companion.title') }}
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('companion.description') }}
            </p>
          </div>
          <button
            :disabled="companionLoading"
            class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600"
            @click="fetchCompanionSessions(); fetchCompanionStats()"
          >
            {{ t('common.refresh') }}
          </button>
        </div>

        <!-- Stats Summary -->
        <div v-if="companionStats" class="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4">
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="text-lg font-bold text-green-400 dark:text-green-400">{{ companionStats.active_sessions }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('companion.activeSessions') }}</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="text-lg font-bold text-gray-900 dark:text-white dark:text-white">{{ companionStats.total_sessions }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('companion.totalSessions') }}</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="text-lg font-bold text-purple-600 dark:text-purple-400">{{ companionStats.total_events }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('companion.totalEvents') }}</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="text-lg font-bold text-orange-600 dark:text-orange-400">{{ companionStats.total_alerts }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('companion.totalAlerts') }}</div>
          </div>
          <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
            <div class="text-lg font-bold text-red-600 dark:text-red-400">{{ companionStats.unacked_alerts }}</div>
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('companion.unackedAlerts') }}</div>
          </div>
        </div>

        <!-- Error -->
        <div v-if="companionError" class="p-3 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded-lg mb-4">
          {{ companionError }}
        </div>

        <!-- Session List (full width, no expand toggle needed) -->
        <div class="max-h-[500px] overflow-y-auto">
          <SessionList
            :sessions="companionSessions"
            :loading="companionLoading"
            :has-more="companionHasMore"
            :selected-id="selectedSession?.id"
            @select="selectSession"
            @load-more="loadMoreSessions"
          />
        </div>
      </div>
    </div>

    <!-- Session Detail Modal -->
    <Teleport to="body">
      <div
        v-if="selectedSession"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
        @click.self="closeSessionDetail"
      >
        <div class="bg-white dark:bg-slate-800 rounded-xl shadow-2xl w-full max-w-4xl max-h-[90vh] overflow-hidden">
          <SessionDetail
            :session="selectedSession"
            @close="closeSessionDetail"
          />
        </div>
      </div>
    </Teleport>

    <!-- Fix Preview Dialog -->
    <FixPreviewDialog
      :visible="fixPreviewVisible"
      :item="fixPreviewItem as any"
      @close="fixPreviewVisible = false"
      @confirm="confirmFix"
    />
  </div>
</template>
