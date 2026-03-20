<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi, type PromptFirewallConfig, type PromptFirewallRule } from '@/api/security'
import { systemApi } from '@/api/index'
import {
  approvalApi,
  type ApprovedBrowserSiteEntry,
  type ApprovedDirectoryEntry,
} from '@/api/approval'
import { companionApi, type CompanionSession, type Stats as CompanionStats } from '@/api/companion'
import {
  getActiveConnections,
  getConnectionStats,
  type Connection,
  type ConnectionStats,
} from '@/api/connections'
import SessionList from '@/components/companion/SessionList.vue'
import SessionDetail from '@/components/companion/SessionDetail.vue'
import FixPreviewDialog from '@/components/security/FixPreviewDialog.vue'
import DataMaskingSettings from '@/components/security/DataMaskingSettings.vue'
import MonitoringRetentionSettings from '@/components/security/MonitoringRetentionSettings.vue'
import NetworkSettings from '@/components/settings/NetworkSettings.vue'
import type { LogEntry } from '@/api/system'
import {
  formatSecurityScanSummary,
  getVisibleSecurityScanSummaryMetrics,
} from '@/utils/securityScanSummary'

const { t, te } = useI18n()

// Tab definitions
type TabId = 'overview' | 'controls' | 'network' | 'monitoring' | 'logs'
const activeTab = ref<TabId>('overview')

interface SecurityTabMeta {
  id: TabId
  labelKey: string
  icon: string
  fallbackLabel: string
}

const tabs: SecurityTabMeta[] = [
  {
    id: 'overview',
    labelKey: 'security.tabs.overview',
    icon: 'shield',
    fallbackLabel: 'Overview',
  },
  {
    id: 'controls',
    labelKey: 'security.tabs.controls',
    icon: 'controls',
    fallbackLabel: 'Security Controls',
  },
  {
    id: 'network',
    labelKey: 'security.tabs.network',
    icon: 'network',
    fallbackLabel: 'Network',
  },
  {
    id: 'monitoring',
    labelKey: 'security.tabs.monitoring',
    icon: 'activity',
    fallbackLabel: 'Monitoring',
  },
  {
    id: 'logs',
    labelKey: 'security.tabs.logs',
    icon: 'list',
    fallbackLabel: 'Logs',
  },
]

function selectTab(tabId: TabId) {
  activeTab.value = tabId
  if (tabId === 'logs' && logs.value.length === 0) {
    fetchLogs()
  }
}

function tr(key: string, fallback = ''): string {
  return te(key) ? t(key) : fallback
}

function getTabLabel(tab: SecurityTabMeta): string {
  return tr(tab.labelKey, tab.fallbackLabel)
}

/** Backend English details string -> security.scan.detailMessages key (for i18n). */
const DETAIL_MESSAGE_KEYS: Record<string, string> = {
  'Threat detector is not initialized': 'threat_detector_not_initialized',
  'XSS pattern detection is enabled in threat detector': 'xss_detection_enabled',
  'SQL injection pattern detection is enabled': 'sql_injection_enabled',
  'Command injection pattern detection is enabled': 'command_injection_enabled',
  'Prompt injection detection is active': 'prompt_injection_active',
  'Prompt injection protection is disabled. Enable PromptGuard for AI security.':
    'prompt_injection_disabled',
  'AI output validation is enabled': 'ai_output_validation_enabled',
  'AI output validation is disabled. Consider enabling for safer AI operations.':
    'ai_output_validation_disabled',
  'Model whitelist is enabled but no models are configured': 'model_whitelist_no_models',
  'Model whitelist is disabled. All models are accessible. Consider enabling for production.':
    'model_whitelist_disabled',
  'Sensitive data filtering is enabled': 'sensitive_data_filtering_enabled',
  'Sensitive data filtering is disabled. PII may be exposed to AI models.':
    'sensitive_data_filtering_disabled',
  'Rate limiting is disabled. API is vulnerable to abuse and DoS attacks.':
    'rate_limiting_disabled',
  'CORS allows all origins in production. This is a security risk.': 'cors_all_origins_production',
  'CORS allows all origins. Acceptable for development, but restrict in production.':
    'cors_all_origins_dev',
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
  'Network access is enabled in sandbox. Consider disabling for better isolation.':
    'network_enabled_sandbox',
  'Data directory has restricted permissions': 'data_dir_restricted',
  'Data directory may have overly permissive access': 'data_dir_permissive',
  'Could not verify data directory permissions': 'data_dir_unknown',
  'Debug mode is enabled in production. This exposes sensitive information.': 'debug_production',
  'Debug mode is enabled. Disable before production deployment.': 'debug_enabled',
  'Debug mode is disabled': 'debug_disabled',
  'Detailed error messages are exposed in production. This may leak sensitive information.':
    'error_exposed_production',
  'Detailed error messages are exposed. Disable before production deployment.': 'error_exposed',
  'Error details are hidden from responses': 'error_hidden',
  'Sensitive error data may be logged. Ensure log access is restricted.': 'error_log_restrict',
  'Sensitive error data is filtered from logs': 'error_filtered_logs',
  'Running in development mode. Ensure production settings before deployment.': 'running_development',
  'Token expiration is too long or not set. This increases risk of token theft.':
    'token_expiration_too_long_or_unset',
  'Token expiration exceeds 8 hours. This increases risk of token theft.':
    'token_expiration_too_long',
  'Token expiration is not set. Check security.jwt.expiration in the loaded security configuration.':
    'token_expiration_not_set',
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
  risk?: string // Why this is a security concern
  impact?: string // What could happen if exploited
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

// Persistent directory approvals (exec/convert allow-always)
const approvedDirs = ref<ApprovedDirectoryEntry[]>([])
const loadingApprovedDirs = ref(false)
const revokingApprovedDirId = ref<string | null>(null)
const approvedBrowserSites = ref<ApprovedBrowserSiteEntry[]>([])
const loadingApprovedBrowserSites = ref(false)
const revokingApprovedBrowserSiteId = ref<string | null>(null)

async function loadApprovedDirectories() {
  try {
    loadingApprovedDirs.value = true
    const response = await approvalApi.listApprovedDirectories()
    approvedDirs.value = response.data.entries || []
  } catch (error) {
    console.error('Failed to load approved directories:', error)
    approvedDirs.value = []
  } finally {
    loadingApprovedDirs.value = false
  }
}

async function revokeApprovedDirectory(id: string) {
  try {
    revokingApprovedDirId.value = id
    await approvalApi.revokeApprovedDirectory(id)
    approvedDirs.value = approvedDirs.value.filter((item) => item.id !== id)
  } catch (error) {
    console.error('Failed to revoke approved directory:', error)
  } finally {
    revokingApprovedDirId.value = null
  }
}

async function loadApprovedBrowserSites() {
  try {
    loadingApprovedBrowserSites.value = true
    const response = await approvalApi.listApprovedBrowserSites()
    approvedBrowserSites.value = response.data.entries || []
  } catch (error) {
    console.error('Failed to load approved browser sites:', error)
    approvedBrowserSites.value = []
  } finally {
    loadingApprovedBrowserSites.value = false
  }
}

async function revokeApprovedBrowserSite(id: string) {
  try {
    revokingApprovedBrowserSiteId.value = id
    await approvalApi.revokeApprovedBrowserSite(id)
    approvedBrowserSites.value = approvedBrowserSites.value.filter((item) => item.id !== id)
  } catch (error) {
    console.error('Failed to revoke approved browser site:', error)
  } finally {
    revokingApprovedBrowserSiteId.value = null
  }
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString()
}

// Prompt firewall state
const firewallLoading = ref(false)
const togglingFirewall = ref(false)
const togglingFirewallRuleId = ref('')
const addingFirewallRule = ref(false)
const deletingFirewallRuleId = ref('')
const firewallConfig = ref<PromptFirewallConfig>({
  enabled: true,
  rules: [],
  rule_count: 0,
})
const newFirewallKeyword = ref('')

const activeFirewallRuleCount = computed(
  () => firewallConfig.value.rules.filter((rule) => rule.enabled).length
)

const firewallSummary = computed(() => {
  if (!firewallConfig.value.enabled) return ''
  return t('security.firewall.ruleCount', { count: activeFirewallRuleCount.value })
})

function applyPromptFirewallConfig(config: PromptFirewallConfig) {
  const rules = [...(config.rules || [])]
  firewallConfig.value = {
    enabled: config.enabled,
    rules,
    rule_count: config.rule_count ?? rules.length,
  }
}

const builtinFirewallRules = computed(() =>
  firewallConfig.value.rules.filter((rule) => rule.built_in || rule.type === 'builtin')
)

const customFirewallRules = computed(() => {
  return firewallConfig.value.rules
    .filter((rule) => !rule.built_in && rule.type !== 'builtin')
    .sort((a, b) => {
      if (!a.created_at || !b.created_at) return 0
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    })
})

function isBuiltinFirewallRule(rule: PromptFirewallRule): boolean {
  return !!rule.built_in || rule.type === 'builtin'
}

function getFirewallRuleName(rule: PromptFirewallRule): string {
  if (!isBuiltinFirewallRule(rule)) return rule.keyword
  return tr(`security.firewall.builtin.${rule.id}.name`, rule.keyword)
}

function getFirewallRuleDescription(rule: PromptFirewallRule): string {
  if (!isBuiltinFirewallRule(rule)) return ''
  return tr(`security.firewall.builtin.${rule.id}.description`, rule.description || '')
}

async function loadPromptFirewall() {
  firewallLoading.value = true
  try {
    const response = await securityApi.getPromptFirewall()
    applyPromptFirewallConfig(response.data)
  } catch (error) {
    console.error('Failed to load prompt firewall config:', error)
  } finally {
    firewallLoading.value = false
  }
}

async function togglePromptFirewall() {
  if (togglingFirewall.value) return
  togglingFirewall.value = true
  try {
    const response = await securityApi.updatePromptFirewall(!firewallConfig.value.enabled)
    applyPromptFirewallConfig(response.data)
  } catch (error) {
    console.error('Failed to toggle prompt firewall:', error)
  } finally {
    togglingFirewall.value = false
  }
}

async function addPromptFirewallRule() {
  if (addingFirewallRule.value) return
  const keyword = newFirewallKeyword.value.trim()
  if (!keyword) return

  addingFirewallRule.value = true
  try {
    const response = await securityApi.addPromptFirewallRule(keyword, true)
    applyPromptFirewallConfig(response.data)
    newFirewallKeyword.value = ''
  } catch (error) {
    console.error('Failed to add prompt firewall rule:', error)
  } finally {
    addingFirewallRule.value = false
  }
}

async function togglePromptFirewallRule(rule: PromptFirewallRule) {
  if (togglingFirewallRuleId.value) return
  togglingFirewallRuleId.value = rule.id
  try {
    const response = await securityApi.updatePromptFirewallRule(rule.id, { enabled: !rule.enabled })
    applyPromptFirewallConfig(response.data)
  } catch (error) {
    console.error('Failed to update prompt firewall rule:', error)
  } finally {
    togglingFirewallRuleId.value = ''
  }
}

async function deletePromptFirewallRule(rule: PromptFirewallRule) {
  if (deletingFirewallRuleId.value) return
  if (isBuiltinFirewallRule(rule)) return
  deletingFirewallRuleId.value = rule.id
  try {
    const response = await securityApi.deletePromptFirewallRule(rule.id)
    applyPromptFirewallConfig(response.data)
  } catch (error) {
    console.error('Failed to delete prompt firewall rule:', error)
  } finally {
    deletingFirewallRuleId.value = ''
  }
}

function getScanStatusPriority(status: ScanItem['status']): number {
  switch (status) {
    case 'failed':
      return 0
    case 'warning':
      return 1
    case 'scanning':
      return 2
    case 'pending':
      return 3
    case 'passed':
      return 4
    default:
      return 5
  }
}

const prioritizedScanResults = computed(() => {
  return scanResults.value
    .map((item, index) => ({ item, index }))
    .sort((a, b) => {
      const p = getScanStatusPriority(a.item.status) - getScanStatusPriority(b.item.status)
      if (p !== 0) return p
      return a.index - b.index
    })
    .map(({ item }) => item)
})

// Logs tab state
const logs = ref<LogEntry[]>([])
const logsLoading = ref(false)
const logLevel = ref('all')
const logSearch = ref('')
const logLimit = ref(100)

async function fetchLogs() {
  logsLoading.value = true
  try {
    const response = await systemApi.getLogs({
      level: logLevel.value === 'all' ? undefined : logLevel.value,
      search: logSearch.value || undefined,
      limit: logLimit.value,
    })
    logs.value = response.data || []
  } catch (e) {
    console.error('Failed to fetch logs:', e)
  } finally {
    logsLoading.value = false
  }
}

function formatLogTime(timestamp: string) {
  return new Date(timestamp).toLocaleTimeString()
}

function getLogLevelClass(level: string) {
  if (!level) return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  switch (level.toLowerCase()) {
    case 'error':
      return 'bg-red-100 dark:bg-red-900/50 text-red-700 dark:text-red-300'
    case 'warn':
      return 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-700 dark:text-yellow-300'
    case 'info':
      return 'bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400'
    case 'debug':
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
    default:
      return 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300'
  }
}

function isRequestLog(log: LogEntry): boolean {
  return log.message === 'request' && log.fields?.method !== undefined
}

function getMethodColor(method: string | undefined): string {
  if (!method) return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  switch (method.toUpperCase()) {
    case 'GET':
      return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300'
    case 'POST':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'PUT':
      return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'PATCH':
      return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'DELETE':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    default:
      return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  }
}

function getStatusColor(status: number): string {
  if (status >= 500) return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  if (status >= 400)
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  if (status >= 300) return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  if (status >= 200)
    return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-300'
  return 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
}

function formatLatency(latency: number): string {
  if (typeof latency !== 'number') return ''
  if (latency >= 1_000_000_000) return `${(latency / 1_000_000_000).toFixed(2)}s`
  if (latency >= 1_000_000) return `${(latency / 1_000_000).toFixed(0)}ms`
  if (latency >= 1_000) return `${(latency / 1_000).toFixed(0)}µs`
  return `${latency}ns`
}

async function exportLogs() {
  const content = logs.value
    .map(
      (log) =>
        `[${log.timestamp}] [${log.level}] ${log.source ? `[${log.source}] ` : ''}${log.message}`
    )
    .join('\n')
  const blob = new Blob([content], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `echo-logs-${new Date().toISOString().split('T')[0]}.txt`
  a.click()
  URL.revokeObjectURL(url)
}

function clearLogs() {
  if (!confirm(t('system.confirmClearLogs'))) return
  logs.value = []
}

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
      const warnings = scanResults.value.filter((r) => r.status === 'warning').length
      const failed = scanResults.value.filter((r) => r.status === 'failed').length
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
      await new Promise((resolve) => setTimeout(resolve, delayPerItem))

      // Update with actual result
      scanItem.status = apiItem.status as ScanItem['status']
      scanItem.details =
        apiItem.details ||
        t(`security.scan.check${apiItem.status.charAt(0).toUpperCase() + apiItem.status.slice(1)}`)

      scanProgress.value = Math.round(((i + 1) / totalItems) * 100)
    }
  } catch (error) {
    console.error('Security scan failed:', error)
    // Fallback to showing error state
    scanResults.value = [
      {
        id: 'error',
        category: 'system',
        name: t('security.scan.error'),
        description: t('security.scan.errorDesc'),
        status: 'failed',
        details: t('security.scan.apiError'),
      },
    ]
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
  return scanResults.value.filter(
    (r) => r.auto_fixable && (r.status === 'warning' || r.status === 'failed')
  ).length
})

// Fix all fixable issues
async function fixAllIssues() {
  const fixableItems = scanResults.value.filter(
    (r) => r.auto_fixable && (r.status === 'warning' || r.status === 'failed')
  )
  for (const item of fixableItems) {
    await fixScanIssue(item)
  }
}

// Get scan summary
const scanSummary = computed(() => {
  const passed = scanResults.value.filter((r) => r.status === 'passed').length
  const warnings = scanResults.value.filter((r) => r.status === 'warning').length
  const failed = scanResults.value.filter((r) => r.status === 'failed').length
  return { passed, warnings, failed, total: scanResults.value.length }
})

const scanSummaryLabels = computed(() => ({
  passed: t('security.scan.passed'),
  warnings: t('security.scan.warnings'),
  failed: t('security.scan.failed'),
}))

const visibleScanSummaryMetrics = computed(() =>
  getVisibleSecurityScanSummaryMetrics(scanSummary.value, scanSummaryLabels.value)
)

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
  return tr(key, item.name || item.id)
}

// Get scan item description with i18n
function getScanItemDescription(item: ScanItem): string {
  const key = `security.scan.items.${item.id}.description`
  return tr(key, item.description || '')
}

// Get scan item risk/impact/remediation with i18n
function getItemField(item: ScanItem, field: 'risk' | 'impact' | 'remediation'): string {
  if (!item[field]) return ''
  const key = `security.scan.items.${item.id}.${field}`
  if (te(key)) return t(key)
  return item[field] || ''
}

// Get scan item status icon and color
function getScanStatusClass(status: string): string {
  switch (status) {
    case 'passed':
      return 'text-emerald-600 dark:text-emerald-400'
    case 'warning':
      return 'text-yellow-500'
    case 'failed':
      return 'text-red-500'
    case 'scanning':
      return 'text-gray-900 dark:text-white animate-pulse'
    default:
      return 'text-gray-400'
  }
}

function getScanStatusLabel(status: ScanItem['status']): string {
  switch (status) {
    case 'scanning':
      return t('security.scan.checking')
    case 'passed':
      return t('security.scan.passed')
    case 'warning':
      return t('security.scan.warnings')
    case 'failed':
      return t('security.scan.failed')
    default:
      return status
  }
}

function hasScanItemDetails(item: ScanItem): boolean {
  return !!(item.risk || item.impact || item.remediation || item.details)
}

function toggleScanItem(item: ScanItem) {
  if (item.status === 'scanning' || !hasScanItemDetails(item)) return
  expandedItemId.value = expandedItemId.value === item.id ? null : item.id
}

// Get overall security status
const securityStatus = computed(() => {
  if (!scanCompleted.value) return 'scanning'
  if (scanSummary.value.failed > 0) return 'failed'
  if (scanSummary.value.warnings > 0) return 'warning'
  return 'passed'
})

const securityStatusBannerVisible = computed(() =>
  securityStatus.value === 'passed' || securityStatus.value === 'scanning'
)

const securityStatusTitle = computed(() => {
  switch (securityStatus.value) {
    case 'passed':
      return t('security.statusSecure')
    case 'warning':
      return t('security.statusWarning')
    case 'failed':
      return t('security.statusFailed')
    default:
      return t('security.statusScanning')
  }
})

const securityStatusDescription = computed(() => {
  return scanCompleted.value
    ? formatSecurityScanSummary(scanSummary.value, scanSummaryLabels.value)
    : t('security.scanInProgress')
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
    const res = await companionApi.listSessions({
      offset: companionOffset.value,
      limit: companionLimit,
    })
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

function refreshCompanionData() {
  fetchCompanionSessions()
  fetchCompanionStats()
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
      getConnectionStats(),
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
  loadPromptFirewall()
  loadApprovedDirectories()
  loadApprovedBrowserSites()

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
  <div class="security-page dashboard-page-frame">
    <section class="security-stage dashboard-page-stage configuration-page-stage">
      <section class="security-hero">
        <div class="security-hero-heading dashboard-page-hero configuration-page-hero">
          <div class="security-hero-copy dashboard-page-copy configuration-page-copy">
            <p class="security-eyebrow dashboard-page-eyebrow">{{ t('nav.configuration') }}</p>
            <h1 class="security-title dashboard-page-title configuration-page-title">
              {{ t('security.title') }}
            </h1>
            <p class="security-description dashboard-page-description configuration-page-description">
              {{ securityStatusDescription }}
            </p>
          </div>
        </div>
      </section>

      <section class="security-shell">
        <div class="security-tab-shell">
          <nav class="security-tab-nav" aria-label="Security sections">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              type="button"
              role="tab"
              :aria-selected="activeTab === tab.id"
              class="security-tab-button"
              :class="{ 'security-tab-button--active': activeTab === tab.id }"
              @click="selectTab(tab.id)"
            >
              <span class="security-tab-button__icon">
                <svg
                  v-if="tab.icon === 'shield'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                  />
                </svg>
                <svg
                  v-else-if="tab.icon === 'controls'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M4 6h16M4 12h16M4 18h16"
                  />
                  <circle cx="8" cy="6" r="1.5" fill="currentColor" />
                  <circle cx="15" cy="12" r="1.5" fill="currentColor" />
                  <circle cx="11" cy="18" r="1.5" fill="currentColor" />
                </svg>
                <svg
                  v-else-if="tab.icon === 'firewall'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 2l7 4v6c0 5-3.4 9.7-7 10-3.6-.3-7-5-7-10V6l7-4zm-2.5 9l2 2 3-3"
                  />
                </svg>
                <svg
                  v-else-if="tab.icon === 'network'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"
                  />
                </svg>
                <svg
                  v-else-if="tab.icon === 'folder-lock'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v2M7 14h10M12 14v4m-5.5-4v4h11v-4a2.5 2.5 0 00-5 0h-1a2.5 2.5 0 00-5 0z"
                  />
                </svg>
                <svg
                  v-else-if="tab.icon === 'masking'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M3 6c0 7.5 4.5 13 9 15 4.5-2 9-7.5 9-15l-9-3-9 3z"
                  />
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 12c.6.8 1.5 1.2 3 1.2s2.4-.4 3-1.2"
                  />
                  <circle cx="9" cy="9.5" r="1" fill="currentColor" />
                  <circle cx="15" cy="9.5" r="1" fill="currentColor" />
                </svg>
                <svg
                  v-else-if="tab.icon === 'activity'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                  />
                </svg>
                <svg
                  v-else-if="tab.icon === 'list'"
                  xmlns="http://www.w3.org/2000/svg"
                  class="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
                  />
                </svg>
              </span>
              <span class="security-tab-button__body">
                <span class="security-tab-button__label-row">
                  <span class="security-tab-button__label">{{ getTabLabel(tab) }}</span>
                </span>
              </span>
              <span class="security-tab-button__state" aria-hidden="true"></span>
            </button>
          </nav>
        </div>

        <div class="security-content">
          <div v-show="activeTab === 'overview'" class="security-section-stack">
            <section
              v-if="securityStatusBannerVisible"
              class="security-status-banner dashboard-card-surface"
              :class="`is-${securityStatus}`"
            >
              <div class="security-status-copy">
                <div class="security-status-icon" :class="`is-${securityStatus}`">
                  <svg
                    v-if="securityStatus === 'passed'"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-6 w-6"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                    />
                  </svg>
                  <svg
                    v-else-if="securityStatus === 'warning'"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-6 w-6"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <svg
                    v-else-if="securityStatus === 'failed'"
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-6 w-6"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <svg
                    v-else
                    xmlns="http://www.w3.org/2000/svg"
                    class="h-6 w-6 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                    />
                  </svg>
                </div>
                <div>
                  <h2 class="security-status-title">{{ securityStatusTitle }}</h2>
                  <p class="security-status-text">{{ securityStatusDescription }}</p>
                </div>
              </div>

              <div class="security-status-metrics">
                <div
                  v-for="metric in visibleScanSummaryMetrics"
                  :key="metric.key"
                  class="security-status-metric"
                >
                  <div class="security-status-metric-value">{{ metric.value }}</div>
                  <div class="security-status-metric-label">{{ metric.label }}</div>
                </div>
              </div>
            </section>

            <section class="security-panel dashboard-card-surface security-scan-panel">
              <div class="security-scan-stack">
                <div class="security-scan-head">
                  <div class="security-scan-heading">
                    <h3 class="security-scan-title">{{ t('security.scan.title') }}</h3>
                    <p class="security-scan-description">{{ t('security.scan.description') }}</p>
                  </div>
                  <div class="security-scan-actions">
                    <button
                      v-if="fixableCount > 0 && !isScanning"
                      :disabled="!!fixingItem"
                      class="security-scan-action is-positive"
                      @click="fixAllIssues"
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                      {{ t('security.scan.fixAll') }} ({{ fixableCount }})
                    </button>
                    <button
                      :disabled="isScanning"
                      class="security-scan-action is-primary"
                      @click="startSecurityScan"
                    >
                      <svg
                        v-if="isScanning"
                        class="animate-spin h-4 w-4"
                        xmlns="http://www.w3.org/2000/svg"
                        fill="none"
                        viewBox="0 0 24 24"
                      >
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      <svg
                        v-else
                        xmlns="http://www.w3.org/2000/svg"
                        class="h-4 w-4"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                        />
                      </svg>
                      {{ isScanning ? t('security.scan.scanning') : t('security.scan.startScan') }}
                    </button>
                  </div>
                </div>

                <div v-if="isScanning || scanCompleted" class="security-scan-progress-shell">
                  <div class="security-scan-progress-card" :class="`is-${securityStatus}`">
                    <div class="security-scan-progress-meta">
                      <div>
                        <p class="security-scan-progress-label">
                          {{ isScanning ? t('security.scan.progress') : t('security.scan.completed') }}
                        </p>
                        <p class="security-scan-progress-caption">
                          {{ securityStatusDescription }}
                        </p>
                      </div>
                      <span class="security-scan-progress-value">{{ scanProgress }}%</span>
                    </div>
                    <div class="security-scan-progress-track">
                      <div
                        class="security-scan-progress-bar"
                        :class="`is-${securityStatus}`"
                        :style="{ width: `${scanProgress}%` }"
                      ></div>
                    </div>
                  </div>

                  <div v-if="scanCompleted" class="security-scan-summary-grid">
                    <div
                      v-for="metric in visibleScanSummaryMetrics"
                      :key="metric.key"
                      class="security-scan-summary-card"
                      :class="`is-${metric.key}`"
                    >
                      <div class="security-scan-summary-value">{{ metric.value }}</div>
                      <div class="security-scan-summary-label">{{ metric.label }}</div>
                    </div>
                  </div>
                </div>

                <div v-if="scanResults.length > 0" class="security-scan-results-shell">
                  <button
                    v-if="scanCompleted"
                    class="security-scan-results-toggle"
                    :aria-expanded="scanResultsExpanded"
                    @click="scanResultsExpanded = !scanResultsExpanded"
                  >
                    <span class="security-scan-results-label">{{ t('security.scan.details') }}</span>
                    <svg
                      :class="[
                        'security-scan-results-chevron',
                        scanResultsExpanded ? 'rotate-180' : '',
                      ]"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M19 9l-7 7-7-7"
                      />
                    </svg>
                  </button>

                  <div v-show="!scanCompleted || scanResultsExpanded" class="security-scan-results-list">
                    <template v-for="(item, index) in prioritizedScanResults" :key="item.id">
                      <div
                        v-if="
                          index === 0 ||
                          prioritizedScanResults[index - 1]?.category !== item.category
                        "
                        class="security-scan-category"
                      >
                        <span class="security-scan-category-pill">
                          {{ getCategoryLabel(item.category) }}
                        </span>
                      </div>

                      <article
                        class="security-scan-item"
                        :class="[
                          `is-${item.status}`,
                          {
                            'is-expanded': expandedItemId === item.id,
                            'is-expandable': item.status !== 'scanning' && hasScanItemDetails(item),
                          },
                        ]"
                        @click="toggleScanItem(item)"
                      >
                        <div class="security-scan-item-main">
                          <div class="security-scan-item-icon" :class="`is-${item.status}`">
                            <div :class="['security-scan-item-icon-symbol', getScanStatusClass(item.status)]">
                              <svg
                                v-if="item.status === 'passed'"
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                                />
                              </svg>
                              <svg
                                v-else-if="item.status === 'warning'"
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                />
                              </svg>
                              <svg
                                v-else-if="item.status === 'failed'"
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
                                />
                              </svg>
                              <svg
                                v-else-if="item.status === 'scanning'"
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5 animate-spin"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  stroke-linecap="round"
                                  stroke-linejoin="round"
                                  stroke-width="2"
                                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                                />
                              </svg>
                              <svg
                                v-else
                                xmlns="http://www.w3.org/2000/svg"
                                class="h-5 w-5"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <circle cx="12" cy="12" r="9" stroke-width="2" />
                              </svg>
                            </div>
                          </div>

                          <div class="security-scan-item-copy">
                            <div class="security-scan-item-title">
                              {{ getScanItemName(item) }}
                            </div>
                            <div class="security-scan-item-text">
                              {{ getScanItemDescription(item) }}
                            </div>
                          </div>

                          <div v-if="item.status !== 'pending'" class="security-scan-item-meta">
                            <span class="security-scan-status-pill" :class="`is-${item.status}`">
                              {{ getScanStatusLabel(item.status) }}
                            </span>
                            <button
                              v-if="
                                item.auto_fixable &&
                                (item.status === 'warning' || item.status === 'failed')
                              "
                              :disabled="fixingItem === item.id"
                              class="security-scan-inline-action"
                              @click.stop="showFixPreview(item)"
                            >
                              <span
                                v-if="fixingItem === item.id"
                                class="security-scan-inline-action-loading"
                              >
                                <svg class="animate-spin h-3 w-3" fill="none" viewBox="0 0 24 24">
                                  <circle
                                    class="opacity-25"
                                    cx="12"
                                    cy="12"
                                    r="10"
                                    stroke="currentColor"
                                    stroke-width="4"
                                  ></circle>
                                  <path
                                    class="opacity-75"
                                    fill="currentColor"
                                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                  ></path>
                                </svg>
                              </span>
                              <span v-else>{{ t('security.scan.fix') }}</span>
                            </button>
                            <span
                              v-else-if="
                                !item.auto_fixable &&
                                (item.status === 'warning' || item.status === 'failed') &&
                                item.remediation
                              "
                              class="security-scan-inline-note"
                            >
                              {{ t('security.scan.manualFix') }}
                            </span>
                            <svg
                              v-if="
                                item.status !== 'scanning' &&
                                hasScanItemDetails(item)
                              "
                              :class="[
                                'security-scan-item-chevron',
                                expandedItemId === item.id ? 'rotate-180' : '',
                              ]"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M19 9l-7 7-7-7"
                              />
                            </svg>
                          </div>
                        </div>

                        <div
                          v-if="
                            expandedItemId === item.id &&
                            hasScanItemDetails(item)
                          "
                          class="security-scan-item-details"
                        >
                          <div class="security-scan-detail-grid">
                            <div v-if="getItemDetails(item)" class="security-scan-detail-card">
                              <span class="security-scan-detail-label">
                                {{ t('security.scan.details') }}
                              </span>
                              <p class="security-scan-detail-value">
                                {{ getItemDetails(item) }}
                              </p>
                            </div>
                            <div v-if="item.risk" class="security-scan-detail-card is-risk">
                              <span class="security-scan-detail-label">
                                {{ t('security.scan.risk') }}
                              </span>
                              <p class="security-scan-detail-value">
                                {{ getItemField(item, 'risk') }}
                              </p>
                            </div>
                            <div v-if="item.impact" class="security-scan-detail-card is-impact">
                              <span class="security-scan-detail-label">
                                {{ t('security.scan.impact') }}
                              </span>
                              <p class="security-scan-detail-value">
                                {{ getItemField(item, 'impact') }}
                              </p>
                            </div>
                            <div
                              v-if="item.remediation"
                              class="security-scan-detail-card is-remediation"
                            >
                              <span class="security-scan-detail-label">
                                {{ t('security.scan.remediation') }}
                              </span>
                              <p class="security-scan-detail-value">
                                {{ getItemField(item, 'remediation') }}
                              </p>
                            </div>
                          </div>
                        </div>
                      </article>
                    </template>
                  </div>
                </div>
              </div>
            </section>
          </div>

          <div
            v-show="activeTab === 'controls'"
            class="security-section-stack security-embedded-stack"
          >
            <section class="security-panel dashboard-card-surface">
              <div class="flex items-center justify-between mb-2 gap-3">
                <div>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ tr('security.approvedDirectories', 'Authorized Directories') }}
                  </h3>
                  <p class="text-sm text-gray-500 dark:text-slate-400">
                    {{
                      tr(
                        'security.approvedDirectoriesDesc',
                        'Directories authorized via Allow Always for exec/convert.'
                      )
                    }}
                  </p>
                </div>
                <button
                  :disabled="loadingApprovedDirs"
                  class="px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-500 rounded text-gray-600 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-600 disabled:opacity-50"
                  @click="loadApprovedDirectories"
                >
                  {{ tr('common.refresh', 'Refresh') }}
                </button>
              </div>

              <div
                v-if="loadingApprovedDirs"
                class="text-sm text-gray-500 dark:text-slate-400 py-3"
              >
                {{ t('common.loading') }}
              </div>
              <div
                v-else-if="!approvedDirs.length"
                class="text-sm text-gray-500 dark:text-slate-400 py-3"
              >
                {{ tr('security.noApprovedDirectories', 'No authorized directories') }}
              </div>
              <div v-else class="space-y-2">
                <div
                  v-for="entry in approvedDirs"
                  :key="entry.id"
                  class="flex items-start justify-between gap-3 p-3 rounded-lg border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-700/50"
                >
                  <div class="min-w-0">
                    <p class="text-sm font-mono text-gray-800 dark:text-slate-200 break-all">
                      {{ entry.path }}
                    </p>
                    <p class="text-xs text-gray-500 dark:text-slate-400 mt-1">
                      {{ tr('claudecode.lastUsed', 'Last used') }}:
                      {{ formatDate(entry.last_used) }}
                    </p>
                  </div>
                  <button
                    :disabled="revokingApprovedDirId === entry.id"
                    class="px-2.5 py-1 text-xs border border-red-300 dark:border-red-700 rounded text-red-600 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50"
                    @click="revokeApprovedDirectory(entry.id)"
                  >
                    {{ tr('common.revoke', 'Revoke') }}
                  </button>
                </div>
              </div>
            </section>

            <section class="security-panel dashboard-card-surface">
              <div class="flex items-center justify-between mb-2 gap-3">
                <div>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ tr('security.approvedBrowserSites', 'Allowed Browser Sites') }}
                  </h3>
                  <p class="text-sm text-gray-500 dark:text-slate-400">
                    {{
                      tr(
                        'security.approvedBrowserSitesDesc',
                        'Websites you marked as Always Allow for browser checkpoints.'
                      )
                    }}
                  </p>
                </div>
                <button
                  :disabled="loadingApprovedBrowserSites"
                  class="px-3 py-1.5 text-sm border border-gray-300 dark:border-slate-500 rounded text-gray-600 dark:text-slate-300 hover:bg-gray-100 dark:hover:bg-slate-600 disabled:opacity-50"
                  @click="loadApprovedBrowserSites"
                >
                  {{ tr('common.refresh', 'Refresh') }}
                </button>
              </div>

              <div
                v-if="loadingApprovedBrowserSites"
                class="text-sm text-gray-500 dark:text-slate-400 py-3"
              >
                {{ t('common.loading') }}
              </div>
              <div
                v-else-if="!approvedBrowserSites.length"
                class="text-sm text-gray-500 dark:text-slate-400 py-3"
              >
                {{ tr('security.noApprovedBrowserSites', 'No allowed browser sites') }}
              </div>
              <div v-else class="space-y-2">
                <div
                  v-for="entry in approvedBrowserSites"
                  :key="entry.id"
                  class="flex items-start justify-between gap-3 p-3 rounded-lg border border-gray-200 dark:border-slate-600 bg-gray-50 dark:bg-slate-700/50"
                >
                  <div class="min-w-0">
                    <p class="text-sm font-mono text-gray-800 dark:text-slate-200 break-all">
                      {{ entry.origin }}
                    </p>
                    <p class="text-xs text-gray-500 dark:text-slate-400 mt-1">
                      {{ tr('claudecode.lastUsed', 'Last used') }}:
                      {{ formatDate(entry.last_used) }}
                    </p>
                  </div>
                  <button
                    :disabled="revokingApprovedBrowserSiteId === entry.id"
                    class="px-2.5 py-1 text-xs border border-red-300 dark:border-red-700 rounded text-red-600 dark:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 disabled:opacity-50"
                    @click="revokeApprovedBrowserSite(entry.id)"
                  >
                    {{ tr('common.revoke', 'Revoke') }}
                  </button>
                </div>
              </div>
            </section>

            <section class="security-panel dashboard-card-surface">
              <div class="flex flex-wrap items-start justify-between gap-4 mb-6">
                <div>
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                    {{ t('security.firewall.title') }}
                  </h3>
                  <p class="text-sm text-gray-500 dark:text-slate-400">
                    {{ t('security.firewall.description') }}
                  </p>
                </div>
                <div class="flex items-center gap-4 text-xs text-gray-500 dark:text-gray-400">
                  <span v-if="firewallSummary">{{ firewallSummary }}</span>
                  <button
                    type="button"
                    :disabled="firewallLoading || togglingFirewall"
                    :class="[
                      'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                      firewallConfig.enabled
                        ? 'bg-emerald-600 dark:bg-emerald-500'
                        : 'bg-gray-300 dark:bg-gray-600',
                      firewallLoading || togglingFirewall
                        ? 'opacity-50 cursor-not-allowed'
                        : 'cursor-pointer',
                    ]"
                    @click="togglePromptFirewall"
                  >
                    <span
                      :class="[
                        'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                        firewallConfig.enabled ? 'translate-x-6' : 'translate-x-1',
                      ]"
                    />
                  </button>
                </div>
              </div>

              <div class="mb-6">
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white mb-2">
                  {{ t('security.firewall.builtinTitle') }}
                </h4>
                <p class="text-xs text-gray-500 dark:text-slate-400 mb-3">
                  {{ t('security.firewall.builtinDescription') }}
                </p>
                <div class="space-y-2 max-h-72 overflow-y-auto">
                  <div
                    v-for="rule in builtinFirewallRules"
                    :key="rule.id"
                    class="flex items-center gap-3 rounded-lg border border-gray-200 dark:border-gray-700 p-3"
                  >
                    <div class="flex-1 min-w-0">
                      <div class="text-sm font-medium text-gray-900 dark:text-white break-all">
                        {{ getFirewallRuleName(rule) }}
                      </div>
                      <div
                        v-if="getFirewallRuleDescription(rule)"
                        class="text-xs text-gray-500 dark:text-slate-400 mt-0.5"
                      >
                        {{ getFirewallRuleDescription(rule) }}
                      </div>
                    </div>
                    <button
                      type="button"
                      :disabled="togglingFirewallRuleId === rule.id || togglingFirewall"
                      @click="togglePromptFirewallRule(rule)"
                      :class="[
                        'shrink-0 text-[10px] px-1.5 py-0.5 rounded-full transition-colors',
                        togglingFirewallRuleId === rule.id || togglingFirewall
                          ? 'opacity-60 cursor-not-allowed'
                          : 'cursor-pointer',
                        rule.enabled
                          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300'
                          : 'bg-gray-200 text-gray-600 dark:bg-gray-600 dark:text-gray-300',
                      ]"
                    >
                      {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
                    </button>
                  </div>
                </div>
              </div>

              <div>
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white mb-2">
                  {{ t('security.firewall.customTitle') }}
                </h4>
                <div class="flex gap-2 mb-4">
                  <input
                    v-model="newFirewallKeyword"
                    type="text"
                    :placeholder="t('security.firewall.keywordPlaceholder')"
                    class="flex-1 bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-3 py-2 border border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400"
                    :disabled="addingFirewallRule"
                    @keyup.enter="addPromptFirewallRule"
                  />
                  <button
                    class="px-4 py-2 rounded-lg text-white font-medium bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 disabled:opacity-50"
                    :disabled="addingFirewallRule || !newFirewallKeyword.trim()"
                    @click="addPromptFirewallRule"
                  >
                    {{ t('security.firewall.add') }}
                  </button>
                </div>

                <div
                  v-if="customFirewallRules.length === 0"
                  class="text-sm text-gray-500 dark:text-slate-400 py-3 text-center"
                >
                  {{ t('security.firewall.empty') }}
                </div>
                <div v-else class="space-y-2 max-h-72 overflow-y-auto">
                  <div
                    v-for="rule in customFirewallRules"
                    :key="rule.id"
                    class="flex items-center gap-2 rounded-lg border border-gray-200 dark:border-gray-700 p-3"
                  >
                    <div class="flex-1 min-w-0">
                      <div class="text-sm font-medium text-gray-900 dark:text-white break-all">
                        {{ rule.keyword }}
                      </div>
                    </div>
                    <button
                      type="button"
                      :disabled="
                        togglingFirewallRuleId === rule.id ||
                        deletingFirewallRuleId === rule.id ||
                        togglingFirewall
                      "
                      @click="togglePromptFirewallRule(rule)"
                      :class="[
                        'shrink-0 text-[10px] px-1.5 py-0.5 rounded-full transition-colors',
                        togglingFirewallRuleId === rule.id ||
                        deletingFirewallRuleId === rule.id ||
                        togglingFirewall
                          ? 'opacity-60 cursor-not-allowed'
                          : 'cursor-pointer',
                        rule.enabled
                          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300'
                          : 'bg-gray-200 text-gray-600 dark:bg-gray-600 dark:text-gray-300',
                      ]"
                    >
                      {{ rule.enabled ? t('common.enabled') : t('common.disabled') }}
                    </button>
                    <button
                      type="button"
                      class="px-2 py-1 rounded text-xs font-medium bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 hover:bg-red-200 dark:hover:bg-red-900/50"
                      :disabled="
                        !!deletingFirewallRuleId ||
                        togglingFirewallRuleId === rule.id ||
                        togglingFirewall
                      "
                      @click="deletePromptFirewallRule(rule)"
                    >
                      {{ t('security.firewall.delete') }}
                    </button>
                  </div>
                </div>
              </div>
            </section>

            <DataMaskingSettings />
          </div>

          <div
            v-show="activeTab === 'network'"
            class="security-section-stack security-embedded-stack"
          >
            <NetworkSettings :show-port-section="false" :show-security-sections="true" />
          </div>

          <div
            v-show="activeTab === 'monitoring'"
            class="security-section-stack security-embedded-stack"
          >
            <section class="security-panel dashboard-card-surface">
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
                      :class="[
                        'w-5 h-5 text-gray-500 transition-transform',
                        connectionExpanded ? 'rotate-180' : '',
                      ]"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M19 9l-7 7-7-7"
                      />
                    </svg>
                  </button>
                </div>
              </div>

              <div v-if="connectionStats" class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="text-lg font-bold text-gray-900 dark:text-white">
                    {{ connectionStats.total_connections ?? 0 }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('connections.total') }}
                  </div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="flex items-center justify-center gap-1">
                    <span class="w-2 h-2 rounded-full bg-gray-700 dark:bg-gray-500"></span>
                    <span class="text-lg font-bold text-gray-900 dark:text-white">{{
                      connectionStats.active_http ?? 0
                    }}</span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">HTTP</div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="flex items-center justify-center gap-1">
                    <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
                    <span class="text-lg font-bold text-emerald-500 dark:text-emerald-400">{{
                      connectionStats.active_websocket ?? 0
                    }}</span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">WebSocket</div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="flex items-center justify-center gap-1">
                    <span class="w-2 h-2 rounded-full bg-purple-500"></span>
                    <span class="text-lg font-bold text-purple-600 dark:text-purple-400">{{
                      connectionStats.active_sse ?? 0
                    }}</span>
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">SSE</div>
                </div>
              </div>

              <div
                v-show="connectionExpanded"
                class="border-t border-gray-200 dark:border-gray-700 pt-4"
              >
                <div class="grid grid-cols-2 gap-3 mb-4">
                  <div class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div class="text-sm font-semibold text-gray-900 dark:text-white">
                      {{ formatBytes(connectionStats?.total_bytes_sent ?? 0) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('connections.bytesSent') }}
                    </div>
                  </div>
                  <div class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
                    <div class="text-sm font-semibold text-gray-900 dark:text-white">
                      {{ formatBytes(connectionStats?.total_bytes_recv ?? 0) }}
                    </div>
                    <div class="text-xs text-gray-500 dark:text-gray-400">
                      {{ t('connections.bytesRecv') }}
                    </div>
                  </div>
                </div>

                <div class="max-h-64 overflow-y-auto space-y-2">
                  <div
                    v-if="connections.length === 0"
                    class="text-center py-4 text-gray-500 dark:text-gray-400"
                  >
                    {{ t('connections.noConnections') }}
                  </div>
                  <div
                    v-for="conn in connections"
                    :key="conn.id"
                    class="p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                  >
                    <div class="flex items-center justify-between mb-1">
                      <div class="flex items-center gap-2">
                        <span
                          :class="[
                            'w-2 h-2 rounded-full',
                            conn.status === 'active' ? 'bg-emerald-500' : 'bg-gray-400',
                          ]"
                        ></span>
                        <span
                          :class="[
                            'px-2 py-0.5 rounded text-xs font-medium uppercase',
                            conn.type === 'http'
                              ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300'
                              : conn.type === 'websocket'
                                ? 'bg-emerald-100 dark:bg-emerald-900/30 text-emerald-800 dark:text-emerald-300'
                                : 'bg-purple-100 dark:bg-purple-900/50 text-purple-700 dark:text-purple-300',
                          ]"
                        >
                          {{ conn.type }}
                        </span>
                        <span
                          class="text-xs font-mono text-gray-600 dark:text-gray-300 truncate max-w-[200px]"
                        >
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
            </section>

            <section class="security-panel dashboard-card-surface">
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
                  @click="refreshCompanionData"
                >
                  {{ t('common.refresh') }}
                </button>
              </div>

              <div v-if="companionStats" class="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4">
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="text-lg font-bold text-emerald-500 dark:text-emerald-400">
                    {{ companionStats.active_sessions }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('companion.activeSessions') }}
                  </div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="text-lg font-bold text-gray-900 dark:text-white">
                    {{ companionStats.total_sessions }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('companion.totalSessions') }}
                  </div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="text-lg font-bold text-purple-600 dark:text-purple-400">
                    {{ companionStats.total_events }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('companion.totalEvents') }}
                  </div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="text-lg font-bold text-orange-600 dark:text-orange-400">
                    {{ companionStats.total_alerts }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('companion.totalAlerts') }}
                  </div>
                </div>
                <div class="text-center p-2 bg-gray-50 dark:bg-gray-700 rounded-lg">
                  <div class="text-lg font-bold text-red-600 dark:text-red-400">
                    {{ companionStats.unacked_alerts }}
                  </div>
                  <div class="text-xs text-gray-500 dark:text-gray-400">
                    {{ t('companion.unackedAlerts') }}
                  </div>
                </div>
              </div>

              <div
                v-if="companionError"
                class="p-3 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded-lg mb-4"
              >
                {{ companionError }}
              </div>

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
            </section>

            <MonitoringRetentionSettings />
          </div>

          <div v-show="activeTab === 'logs'" class="security-section-stack">
            <section class="security-panel dashboard-card-surface">
              <div class="flex flex-wrap gap-4 items-center mb-4">
                <div class="flex-1 min-w-[200px]">
                  <input
                    v-model="logSearch"
                    type="text"
                    :placeholder="t('system.searchLogs')"
                    class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-gray-400 border border-gray-300 dark:border-gray-600"
                    @keyup.enter="fetchLogs"
                  />
                </div>
                <select
                  v-model="logLevel"
                  class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600"
                  @change="fetchLogs"
                >
                  <option value="all">{{ t('system.allLevels') }}</option>
                  <option value="error">{{ t('system.error') }}</option>
                  <option value="warn">{{ t('system.warning') }}</option>
                  <option value="info">{{ t('system.info') }}</option>
                  <option value="debug">{{ t('system.debug') }}</option>
                </select>
                <select
                  v-model="logLimit"
                  class="bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-2 border border-gray-300 dark:border-gray-600"
                  @change="fetchLogs"
                >
                  <option :value="50">50</option>
                  <option :value="100">100</option>
                  <option :value="200">200</option>
                </select>
                <div class="flex flex-wrap gap-2">
                  <button
                    class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg"
                    :disabled="logsLoading"
                    @click="fetchLogs"
                  >
                    {{ logsLoading ? t('common.loading') : t('system.refresh') }}
                  </button>
                  <button
                    class="px-3 py-2 bg-gray-100 dark:bg-gray-700/30 hover:bg-gray-200 dark:hover:bg-gray-700/50 text-gray-700 dark:text-gray-300 rounded-lg"
                    :disabled="logs.length === 0"
                    @click="exportLogs"
                  >
                    {{ t('system.exportLogs') }}
                  </button>
                  <button
                    class="px-3 py-2 bg-red-100 dark:bg-red-900/30 hover:bg-red-200 dark:hover:bg-red-900/50 text-red-700 dark:text-red-400 rounded-lg"
                    :disabled="logs.length === 0"
                    @click="clearLogs"
                  >
                    {{ t('system.clearLogs') }}
                  </button>
                </div>
              </div>

              <div v-if="logsLoading" class="p-8 text-center text-gray-500 dark:text-gray-400">
                {{ t('system.loadingLogs') }}
              </div>
              <div
                v-else-if="logs.length === 0"
                class="p-8 text-center text-gray-500 dark:text-gray-400"
              >
                {{ t('system.noLogsFound') }}
              </div>
              <div
                v-else
                class="divide-y divide-gray-200 dark:divide-gray-700 max-h-[400px] overflow-y-auto font-mono text-sm"
              >
                <div
                  v-for="(log, index) in logs"
                  :key="index"
                  class="p-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 flex gap-3 items-start"
                >
                  <span class="text-gray-400 dark:text-gray-500 flex-shrink-0 w-20">{{
                    formatLogTime(log.timestamp)
                  }}</span>
                  <span
                    :class="getLogLevelClass(log.level)"
                    class="px-2 py-0.5 rounded text-xs uppercase font-medium flex-shrink-0"
                    >{{ log.level }}</span
                  >
                  <span v-if="log.source" class="text-purple-600 dark:text-purple-400 flex-shrink-0"
                    >[{{ log.source.split('/').pop()?.split(':')[0] }}]</span
                  >
                  <template v-if="isRequestLog(log)">
                    <span
                      class="px-1.5 py-0.5 text-xs font-medium rounded"
                      :class="getMethodColor(log.fields?.method as string)"
                    >
                      {{ log.fields?.method }}
                    </span>
                    <span
                      class="text-gray-900 dark:text-gray-100 break-all flex-1 truncate"
                      :title="log.fields?.uri as string"
                    >
                      {{ log.fields?.uri }}
                    </span>
                    <span
                      class="px-1.5 py-0.5 text-xs font-medium rounded"
                      :class="getStatusColor(log.fields?.status as number)"
                    >
                      {{ log.fields?.status }}
                    </span>
                    <span class="text-gray-500 dark:text-gray-400 text-xs whitespace-nowrap">
                      {{ formatLatency(log.fields?.latency as number) }}
                    </span>
                  </template>
                  <span v-else class="text-gray-700 dark:text-gray-300 break-all">{{
                    log.message
                  }}</span>
                </div>
              </div>
            </section>
          </div>
        </div>
      </section>
    </section>

    <Teleport to="body">
      <div
        v-if="selectedSession"
        class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-4 bg-black/50 overflow-y-auto"
        @click.self="closeSessionDetail"
      >
        <div
          class="bg-white dark:bg-slate-800 rounded-xl shadow-2xl w-full max-w-4xl h-[92vh] max-h-[92vh] my-auto overflow-hidden flex flex-col min-h-0"
        >
          <SessionDetail :session="selectedSession" @close="closeSessionDetail" />
        </div>
      </div>
    </Teleport>

    <FixPreviewDialog
      :visible="fixPreviewVisible"
      :item="fixPreviewItem as any"
      @close="fixPreviewVisible = false"
      @confirm="confirmFix"
    />
  </div>
</template>

<style scoped>
.security-page {
  --dashboard-page-accent: 37, 99, 235;
  max-width: 1480px;
  margin: 0 auto;
  padding: 0 0.75rem 1.8rem;
}

.security-stage {
  position: relative;
}

.security-stage::before,
.security-stage::after {
  content: '';
}

.security-hero {
  position: relative;
  padding: 0 0 0.2rem;
}

.security-hero-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
  max-width: 82rem;
  margin: 0 auto;
  padding-bottom: 0;
}

.security-hero-copy {
  flex: 1 1 0%;
  min-width: 0;
  max-width: 42rem;
  padding-top: 0.1rem;
}

.security-title {
  margin: 0;
  font-size: clamp(1.34rem, 0.7vw + 0.95rem, 1.9rem);
  line-height: 1.06;
  letter-spacing: -0.04em;
  font-weight: 700;
  color: #111827;
}

.security-description {
  margin: 0.42rem 0 0;
  max-width: 34rem;
  font-size: 0.92rem;
  line-height: 1.55;
  color: #9ca3af;
}

.security-shell {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  width: 100%;
  max-width: 82rem;
  margin: 0 auto;
}

.security-page :deep(.dashboard-card-surface) {
  background: #ffffff;
  background-image: none;
}

.security-page :deep(.dashboard-card-subsurface) {
  background: #f8fafc;
  background-image: none;
}

.security-tab-shell {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.security-tab-nav {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.security-tab-button {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  min-height: 100%;
  padding: 10px 12px;
  border: 1px solid rgba(226, 232, 240, 0.96);
  border-radius: 1rem;
  background: #ffffff;
  color: #0f172a;
  text-align: left;
  box-shadow: none;
  transition:
    border-color 0.22s ease,
    background-color 0.22s ease,
    color 0.22s ease;
}

.security-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.52);
  background: #f8fafc;
}

.security-tab-button--active {
  border-color: rgba(148, 163, 184, 0.58);
  background: #f1f5f9;
  color: #0f172a;
}

.security-tab-button__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  border-radius: 0.65rem;
  background: rgba(148, 163, 184, 0.16);
  color: #64748b;
}

.security-tab-button__icon svg {
  width: 1rem;
  height: 1rem;
}

.security-tab-button--active .security-tab-button__icon {
  background: rgba(148, 163, 184, 0.22);
  color: #475569;
}

.security-tab-button__body {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.32rem;
}

.security-tab-button__label-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
}

.security-tab-button__label {
  display: inline-flex;
  font-size: 0.9rem;
  font-weight: 700;
  line-height: 1.25;
}

.security-tab-button__state {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.35);
  transition:
    transform 0.22s ease,
    background-color 0.22s ease;
}

.security-tab-button--active .security-tab-button__state {
  transform: scale(1.05);
  background: #64748b;
}

.security-content,
.security-section-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.security-panel {
  padding: 1.35rem;
  border-radius: 1.5rem;
}

.security-status-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.15rem 1.25rem;
  border-radius: 1.5rem;
}

.security-status-banner.is-passed {
  border-color: rgba(110, 231, 183, 0.55);
  background: rgba(236, 253, 245, 0.98);
}

.security-status-banner.is-warning {
  border-color: rgba(253, 224, 71, 0.52);
  background: rgba(254, 252, 232, 0.98);
}

.security-status-banner.is-failed {
  border-color: rgba(252, 165, 165, 0.52);
  background: rgba(254, 242, 242, 0.98);
}

.security-status-copy {
  display: flex;
  align-items: center;
  gap: 0.95rem;
}

.security-status-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.8rem;
  height: 2.8rem;
  flex-shrink: 0;
  border-radius: 999px;
  color: #fff;
  background: #334155;
  box-shadow: 0 18px 36px -28px rgba(15, 23, 42, 0.24);
}

.security-status-icon.is-passed {
  background: #10b981;
}

.security-status-icon.is-warning {
  background: #f59e0b;
}

.security-status-icon.is-failed {
  background: #ef4444;
}

.security-status-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 650;
  color: #111827;
}

.security-status-text {
  margin: 0.2rem 0 0;
  font-size: 0.88rem;
  line-height: 1.5;
  color: #64748b;
}

.security-status-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.75rem;
  width: min(20rem, 100%);
}

.security-status-metric {
  min-width: 0;
  padding: 0.82rem 0.85rem;
  border-radius: 1rem;
  text-align: center;
  border: 1px solid rgba(255, 255, 255, 0.72);
  background: rgba(255, 255, 255, 0.72);
}

.security-status-metric-value {
  color: #111827;
  font-size: 1.2rem;
  font-weight: 700;
}

.security-status-metric-label {
  margin-top: 0.2rem;
  color: #64748b;
  font-size: 0.66rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.security-scan-panel {
  overflow: hidden;
}

.security-scan-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.security-scan-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.security-scan-heading {
  max-width: 40rem;
}

.security-scan-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 650;
  color: #111827;
}

.security-scan-description {
  margin: 0.35rem 0 0;
  font-size: 0.9rem;
  line-height: 1.6;
  color: #64748b;
}

.security-scan-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.65rem;
}

.security-scan-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.55rem;
  min-height: 2.75rem;
  padding: 0.72rem 1rem;
  border-radius: 999px;
  border: 1px solid transparent;
  font-size: 0.85rem;
  font-weight: 600;
  transition:
    transform 0.18s ease,
    box-shadow 0.18s ease,
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease;
}

.security-scan-action:hover {
  transform: translateY(-1px);
}

.security-scan-action:disabled {
  cursor: not-allowed;
  transform: none;
  opacity: 0.55;
  box-shadow: none;
}

.security-scan-action.is-primary {
  color: #fff;
  background: #111827;
  box-shadow: 0 18px 30px -24px rgba(15, 23, 42, 0.58);
}

.security-scan-action.is-primary:hover {
  background: #0f172a;
}

.security-scan-action.is-positive {
  color: #047857;
  border-color: rgba(16, 185, 129, 0.18);
  background: rgba(236, 253, 245, 0.98);
  box-shadow: 0 18px 28px -28px rgba(5, 150, 105, 0.5);
}

.security-scan-action.is-positive:hover {
  color: #065f46;
  border-color: rgba(16, 185, 129, 0.3);
  background: rgba(220, 252, 231, 0.98);
}

.security-scan-progress-shell {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}

.security-scan-progress-card,
.security-scan-results-shell {
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 1.25rem;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.security-scan-progress-card {
  padding: 1rem 1.05rem;
}

.security-scan-progress-card.is-passed {
  border-color: rgba(110, 231, 183, 0.44);
  background: rgba(236, 253, 245, 0.92);
}

.security-scan-progress-card.is-warning {
  border-color: rgba(253, 224, 71, 0.36);
  background: rgba(254, 252, 232, 0.92);
}

.security-scan-progress-card.is-failed {
  border-color: rgba(252, 165, 165, 0.36);
  background: rgba(254, 242, 242, 0.93);
}

.security-scan-progress-meta {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.security-scan-progress-label {
  margin: 0;
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: #94a3b8;
}

.security-scan-progress-caption {
  margin: 0.32rem 0 0;
  font-size: 0.88rem;
  line-height: 1.55;
  color: #475569;
}

.security-scan-progress-value {
  font-size: 1.45rem;
  line-height: 1;
  font-weight: 700;
  letter-spacing: -0.05em;
  color: #111827;
}

.security-scan-progress-track {
  margin-top: 0.85rem;
  height: 0.65rem;
  overflow: hidden;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.18);
}

.security-scan-progress-bar {
  height: 100%;
  border-radius: inherit;
  background: #3b82f6;
  transition:
    width 0.3s ease-out,
    background 0.18s ease;
}

.security-scan-progress-bar.is-warning {
  background: #f59e0b;
}

.security-scan-progress-bar.is-failed {
  background: #ef4444;
}

.security-scan-summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.75rem;
}

.security-scan-summary-card {
  padding: 0.95rem 1rem;
  border-radius: 1.15rem;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background: rgba(255, 255, 255, 0.72);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.security-scan-summary-card.is-passed {
  border-color: rgba(110, 231, 183, 0.38);
  background: rgba(236, 253, 245, 0.92);
}

.security-scan-summary-card.is-warning {
  border-color: rgba(253, 224, 71, 0.3);
  background: rgba(254, 252, 232, 0.92);
}

.security-scan-summary-card.is-failed {
  border-color: rgba(252, 165, 165, 0.3);
  background: rgba(254, 242, 242, 0.93);
}

.security-scan-summary-value {
  color: #111827;
  font-size: 1.55rem;
  line-height: 1;
  font-weight: 700;
  letter-spacing: -0.05em;
}

.security-scan-summary-label {
  margin-top: 0.35rem;
  color: #64748b;
  font-size: 0.72rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.security-scan-results-shell {
  padding: 0.3rem;
}

.security-scan-results-toggle {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 0.8rem 0.35rem;
  border: 0;
  border-radius: 1rem;
  background: transparent;
  text-align: left;
  transition: background-color 0.18s ease;
}

.security-scan-results-toggle:hover {
  background: rgba(15, 23, 42, 0.04);
}

.security-scan-results-label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: #94a3b8;
}

.security-scan-results-chevron,
.security-scan-item-chevron {
  width: 1rem;
  height: 1rem;
  color: #94a3b8;
  transition:
    transform 0.18s ease,
    color 0.18s ease;
}

.security-scan-results-list {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  max-height: 32rem;
  overflow-y: auto;
  padding: 0.15rem 0.2rem 0.4rem;
}

.security-scan-category {
  padding-top: 0.45rem;
}

.security-scan-category-pill {
  display: inline-flex;
  align-items: center;
  padding: 0.35rem 0.65rem;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.14);
  color: #64748b;
  font-size: 0.68rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.security-scan-item {
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 1.2rem;
  background: rgba(255, 255, 255, 0.76);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  cursor: default;
  transition:
    transform 0.18s ease,
    border-color 0.18s ease,
    background-color 0.18s ease,
    box-shadow 0.18s ease;
}

.security-scan-item.is-expandable {
  cursor: pointer;
}

.security-scan-item:hover {
  transform: translateY(-1px);
  border-color: rgba(148, 163, 184, 0.34);
  background: rgba(255, 255, 255, 0.9);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.82),
    0 16px 28px -30px rgba(15, 23, 42, 0.24);
}

.security-scan-item.is-expanded {
  border-color: rgba(96, 165, 250, 0.3);
  background: rgba(248, 250, 252, 0.98);
}

.security-scan-item.is-scanning {
  border-style: dashed;
}

.security-scan-item.is-passed {
  border-color: rgba(110, 231, 183, 0.26);
}

.security-scan-item.is-warning {
  border-color: rgba(253, 224, 71, 0.28);
}

.security-scan-item.is-failed {
  border-color: rgba(252, 165, 165, 0.3);
}

.security-scan-item-main {
  display: flex;
  align-items: flex-start;
  gap: 0.85rem;
  padding: 0.95rem 1rem;
}

.security-scan-item-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.45rem;
  height: 2.45rem;
  flex-shrink: 0;
  border-radius: 0.95rem;
  border: 1px solid rgba(226, 232, 240, 0.88);
  background: rgba(248, 250, 252, 0.9);
}

.security-scan-item-icon.is-passed {
  border-color: rgba(110, 231, 183, 0.35);
  background: rgba(236, 253, 245, 0.95);
}

.security-scan-item-icon.is-warning {
  border-color: rgba(253, 224, 71, 0.3);
  background: rgba(254, 252, 232, 0.95);
}

.security-scan-item-icon.is-failed {
  border-color: rgba(252, 165, 165, 0.34);
  background: rgba(254, 242, 242, 0.95);
}

.security-scan-item-icon-symbol {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.security-scan-item-copy {
  min-width: 0;
  flex: 1;
}

.security-scan-item-title {
  color: #111827;
  font-size: 0.92rem;
  line-height: 1.4;
  font-weight: 650;
}

.security-scan-item-text {
  margin-top: 0.22rem;
  color: #64748b;
  font-size: 0.8rem;
  line-height: 1.55;
}

.security-scan-item-meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.45rem;
  flex-wrap: wrap;
  margin-left: auto;
}

.security-scan-status-pill,
.security-scan-inline-action,
.security-scan-inline-note {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 1.85rem;
  padding: 0.32rem 0.72rem;
  border-radius: 999px;
  font-size: 0.72rem;
  font-weight: 600;
  white-space: nowrap;
}

.security-scan-status-pill {
  border: 1px solid transparent;
}

.security-scan-status-pill.is-passed {
  color: #047857;
  border-color: rgba(110, 231, 183, 0.38);
  background: rgba(236, 253, 245, 0.92);
}

.security-scan-status-pill.is-warning {
  color: #a16207;
  border-color: rgba(253, 224, 71, 0.32);
  background: rgba(254, 252, 232, 0.92);
}

.security-scan-status-pill.is-failed {
  color: #b91c1c;
  border-color: rgba(252, 165, 165, 0.34);
  background: rgba(254, 242, 242, 0.92);
}

.security-scan-status-pill.is-scanning {
  color: #475569;
  border-color: rgba(148, 163, 184, 0.28);
  background: rgba(241, 245, 249, 0.92);
}

.security-scan-inline-action {
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(255, 255, 255, 0.9);
  color: #111827;
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease;
}

.security-scan-inline-action:hover {
  border-color: rgba(96, 165, 250, 0.32);
  background: rgba(239, 246, 255, 0.92);
  color: #1d4ed8;
}

.security-scan-inline-action:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.security-scan-inline-action-loading {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.security-scan-inline-note {
  color: #c2410c;
  border: 1px solid rgba(251, 146, 60, 0.22);
  background: rgba(255, 237, 213, 0.92);
}

.security-scan-item-details {
  padding: 0 1rem 1rem 4.3rem;
}

.security-scan-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem;
}

.security-scan-detail-card {
  min-width: 0;
  padding: 0.85rem 0.9rem;
  border-radius: 1rem;
  border: 1px solid rgba(226, 232, 240, 0.88);
  background: rgba(248, 250, 252, 0.94);
}

.security-scan-detail-card.is-risk {
  border-color: rgba(251, 191, 36, 0.28);
  background: rgba(254, 252, 232, 0.94);
}

.security-scan-detail-card.is-impact {
  border-color: rgba(252, 165, 165, 0.3);
  background: rgba(254, 242, 242, 0.94);
}

.security-scan-detail-card.is-remediation {
  border-color: rgba(110, 231, 183, 0.3);
  background: rgba(236, 253, 245, 0.94);
}

.security-scan-detail-label {
  display: block;
  margin-bottom: 0.38rem;
  color: #94a3b8;
  font-size: 0.66rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.security-scan-detail-value {
  margin: 0;
  color: #475569;
  font-size: 0.82rem;
  line-height: 1.6;
}

.security-embedded-stack :deep(.glass-card),
.security-panel :deep(.glass-card) {
  border: 1px solid rgba(255, 255, 255, 0.92);
  border-radius: 1.25rem;
  background: rgba(255, 255, 255, 0.98);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 22px 36px -34px rgba(15, 23, 42, 0.2);
}

.security-embedded-stack :deep(.glass-card:hover),
.security-panel :deep(.glass-card:hover) {
  border-color: rgba(255, 255, 255, 0.98);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 24px 38px -34px rgba(15, 23, 42, 0.24);
}

.security-embedded-stack :deep(.security-outlined-card) {
  border-color: rgba(203, 213, 225, 0.96);
}

.security-embedded-stack :deep(.security-outlined-card:hover) {
  border-color: rgba(148, 163, 184, 0.8);
}

:root.dark .security-title,
[data-theme='dark'] .security-title,
html.dark .security-title {
  color: rgb(241 245 249);
}

:root.dark .security-description,
[data-theme='dark'] .security-description,
html.dark .security-description {
  color: rgb(148 163 184);
}

:root.dark .security-page :deep(.dashboard-card-surface),
[data-theme='dark'] .security-page :deep(.dashboard-card-surface),
html.dark .security-page :deep(.dashboard-card-surface) {
  background: #1e293b;
  background-image: none;
}

:root.dark .security-page :deep(.dashboard-card-subsurface),
[data-theme='dark'] .security-page :deep(.dashboard-card-subsurface),
html.dark .security-page :deep(.dashboard-card-subsurface) {
  background: #0f172a;
  background-image: none;
}

:root.dark .security-panel,
[data-theme='dark'] .security-panel,
html.dark .security-panel,
:root.dark .security-status-banner,
[data-theme='dark'] .security-status-banner,
html.dark .security-status-banner,
:root.dark .security-embedded-stack :deep(.glass-card),
[data-theme='dark'] .security-embedded-stack :deep(.glass-card),
html.dark .security-embedded-stack :deep(.glass-card),
:root.dark .security-panel :deep(.glass-card),
[data-theme='dark'] .security-panel :deep(.glass-card),
html.dark .security-panel :deep(.glass-card) {
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(28, 41, 59, 0.98);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 24px 38px -34px rgba(2, 6, 23, 0.64);
}

:root.dark .security-embedded-stack :deep(.security-outlined-card),
[data-theme='dark'] .security-embedded-stack :deep(.security-outlined-card),
html.dark .security-embedded-stack :deep(.security-outlined-card) {
  border-color: rgba(71, 85, 105, 0.58);
}

:root.dark .security-tab-shell,
[data-theme='dark'] .security-tab-shell,
html.dark .security-tab-shell {
  background: transparent;
  border: 0;
  box-shadow: none;
}

:root.dark .security-status-banner.is-passed,
[data-theme='dark'] .security-status-banner.is-passed,
html.dark .security-status-banner.is-passed {
  background: rgba(6, 78, 59, 0.72);
}

:root.dark .security-status-banner.is-warning,
[data-theme='dark'] .security-status-banner.is-warning,
html.dark .security-status-banner.is-warning {
  background: rgba(113, 63, 18, 0.68);
}

:root.dark .security-status-banner.is-failed,
[data-theme='dark'] .security-status-banner.is-failed,
html.dark .security-status-banner.is-failed {
  background: rgba(127, 29, 29, 0.72);
}

:root.dark .security-status-title,
[data-theme='dark'] .security-status-title,
html.dark .security-status-title,
:root.dark .security-status-metric-value,
[data-theme='dark'] .security-status-metric-value,
html.dark .security-status-metric-value,
:root.dark .security-tab-button--active,
[data-theme='dark'] .security-tab-button--active,
html.dark .security-tab-button--active {
  color: rgb(241 245 249);
}

:root.dark .security-status-text,
[data-theme='dark'] .security-status-text,
html.dark .security-status-text,
:root.dark .security-status-metric-label,
[data-theme='dark'] .security-status-metric-label,
html.dark .security-status-metric-label,
:root.dark .security-tab-button,
[data-theme='dark'] .security-tab-button,
html.dark .security-tab-button {
  color: rgb(148 163 184);
}

:root.dark .security-tab-button:hover,
[data-theme='dark'] .security-tab-button:hover,
html.dark .security-tab-button:hover {
  border-color: rgba(148, 163, 184, 0.28);
  background: #1f2937;
}

:root.dark .security-tab-button--active,
[data-theme='dark'] .security-tab-button--active,
html.dark .security-tab-button--active {
  border-color: rgba(148, 163, 184, 0.4);
  background: #1f2937;
}

:root.dark .security-tab-button,
[data-theme='dark'] .security-tab-button,
html.dark .security-tab-button {
  border-color: rgba(71, 85, 105, 0.46);
  background: transparent;
}

:root.dark .security-tab-button__icon,
[data-theme='dark'] .security-tab-button__icon,
html.dark .security-tab-button__icon {
  color: #cbd5e1;
  background: rgba(148, 163, 184, 0.18);
}

:root.dark .security-tab-button__state,
[data-theme='dark'] .security-tab-button__state,
html.dark .security-tab-button__state {
  background: rgba(148, 163, 184, 0.32);
}

:root.dark .security-tab-button--active .security-tab-button__icon,
[data-theme='dark'] .security-tab-button--active .security-tab-button__icon,
html.dark .security-tab-button--active .security-tab-button__icon {
  background: rgba(148, 163, 184, 0.24);
  color: #f8fafc;
}

:root.dark .security-tab-button--active .security-tab-button__state,
[data-theme='dark'] .security-tab-button--active .security-tab-button__state,
html.dark .security-tab-button--active .security-tab-button__state {
  background: #cbd5e1;
}

:root.dark .security-status-metric,
[data-theme='dark'] .security-status-metric,
html.dark .security-status-metric {
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.26);
}

:root.dark .security-scan-title,
[data-theme='dark'] .security-scan-title,
html.dark .security-scan-title,
:root.dark .security-scan-progress-value,
[data-theme='dark'] .security-scan-progress-value,
html.dark .security-scan-progress-value,
:root.dark .security-scan-summary-value,
[data-theme='dark'] .security-scan-summary-value,
html.dark .security-scan-summary-value,
:root.dark .security-scan-item-title,
[data-theme='dark'] .security-scan-item-title,
html.dark .security-scan-item-title,
:root.dark .security-scan-inline-action,
[data-theme='dark'] .security-scan-inline-action,
html.dark .security-scan-inline-action {
  color: rgb(241 245 249);
}

:root.dark .security-scan-description,
[data-theme='dark'] .security-scan-description,
html.dark .security-scan-description,
:root.dark .security-scan-progress-caption,
[data-theme='dark'] .security-scan-progress-caption,
html.dark .security-scan-progress-caption,
:root.dark .security-scan-summary-label,
[data-theme='dark'] .security-scan-summary-label,
html.dark .security-scan-summary-label,
:root.dark .security-scan-category-pill,
[data-theme='dark'] .security-scan-category-pill,
html.dark .security-scan-category-pill,
:root.dark .security-scan-item-text,
[data-theme='dark'] .security-scan-item-text,
html.dark .security-scan-item-text,
:root.dark .security-scan-detail-value,
[data-theme='dark'] .security-scan-detail-value,
html.dark .security-scan-detail-value,
:root.dark .security-scan-detail-label,
[data-theme='dark'] .security-scan-detail-label,
html.dark .security-scan-detail-label,
:root.dark .security-scan-results-label,
[data-theme='dark'] .security-scan-results-label,
html.dark .security-scan-results-label,
:root.dark .security-scan-progress-label,
[data-theme='dark'] .security-scan-progress-label,
html.dark .security-scan-progress-label,
:root.dark .security-scan-results-chevron,
[data-theme='dark'] .security-scan-results-chevron,
html.dark .security-scan-results-chevron,
:root.dark .security-scan-item-chevron,
[data-theme='dark'] .security-scan-item-chevron,
html.dark .security-scan-item-chevron {
  color: rgb(148 163 184);
}

:root.dark .security-scan-action.is-primary,
[data-theme='dark'] .security-scan-action.is-primary,
html.dark .security-scan-action.is-primary {
  color: rgb(241 245 249);
  border-color: rgba(148, 163, 184, 0.14);
  background: rgba(15, 23, 42, 0.92);
  box-shadow: 0 20px 30px -26px rgba(2, 6, 23, 0.74);
}

:root.dark .security-scan-action.is-primary:hover,
[data-theme='dark'] .security-scan-action.is-primary:hover,
html.dark .security-scan-action.is-primary:hover {
  background: rgba(30, 41, 59, 0.96);
}

:root.dark .security-scan-action.is-positive,
[data-theme='dark'] .security-scan-action.is-positive,
html.dark .security-scan-action.is-positive {
  color: rgb(167 243 208);
  border-color: rgba(16, 185, 129, 0.24);
  background: rgba(6, 78, 59, 0.74);
}

:root.dark .security-scan-action.is-positive:hover,
[data-theme='dark'] .security-scan-action.is-positive:hover,
html.dark .security-scan-action.is-positive:hover {
  color: rgb(209 250 229);
  border-color: rgba(52, 211, 153, 0.34);
  background: rgba(6, 95, 70, 0.82);
}

:root.dark .security-scan-progress-card,
[data-theme='dark'] .security-scan-progress-card,
html.dark .security-scan-progress-card,
:root.dark .security-scan-results-shell,
[data-theme='dark'] .security-scan-results-shell,
html.dark .security-scan-results-shell,
:root.dark .security-scan-summary-card,
[data-theme='dark'] .security-scan-summary-card,
html.dark .security-scan-summary-card,
:root.dark .security-scan-item,
[data-theme='dark'] .security-scan-item,
html.dark .security-scan-item,
:root.dark .security-scan-detail-card,
[data-theme='dark'] .security-scan-detail-card,
html.dark .security-scan-detail-card,
:root.dark .security-scan-item-icon,
[data-theme='dark'] .security-scan-item-icon,
html.dark .security-scan-item-icon {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(15, 23, 42, 0.28);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:root.dark .security-scan-progress-card.is-passed,
[data-theme='dark'] .security-scan-progress-card.is-passed,
html.dark .security-scan-progress-card.is-passed,
:root.dark .security-scan-summary-card.is-passed,
[data-theme='dark'] .security-scan-summary-card.is-passed,
html.dark .security-scan-summary-card.is-passed,
:root.dark .security-scan-detail-card.is-remediation,
[data-theme='dark'] .security-scan-detail-card.is-remediation,
html.dark .security-scan-detail-card.is-remediation,
:root.dark .security-scan-item-icon.is-passed,
[data-theme='dark'] .security-scan-item-icon.is-passed,
html.dark .security-scan-item-icon.is-passed {
  border-color: rgba(16, 185, 129, 0.22);
  background: rgba(6, 78, 59, 0.4);
}

:root.dark .security-scan-progress-card.is-warning,
[data-theme='dark'] .security-scan-progress-card.is-warning,
html.dark .security-scan-progress-card.is-warning,
:root.dark .security-scan-summary-card.is-warning,
[data-theme='dark'] .security-scan-summary-card.is-warning,
html.dark .security-scan-summary-card.is-warning,
:root.dark .security-scan-detail-card.is-risk,
[data-theme='dark'] .security-scan-detail-card.is-risk,
html.dark .security-scan-detail-card.is-risk,
:root.dark .security-scan-item-icon.is-warning,
[data-theme='dark'] .security-scan-item-icon.is-warning,
html.dark .security-scan-item-icon.is-warning {
  border-color: rgba(245, 158, 11, 0.22);
  background: rgba(120, 53, 15, 0.42);
}

:root.dark .security-scan-progress-card.is-failed,
[data-theme='dark'] .security-scan-progress-card.is-failed,
html.dark .security-scan-progress-card.is-failed,
:root.dark .security-scan-summary-card.is-failed,
[data-theme='dark'] .security-scan-summary-card.is-failed,
html.dark .security-scan-summary-card.is-failed,
:root.dark .security-scan-detail-card.is-impact,
[data-theme='dark'] .security-scan-detail-card.is-impact,
html.dark .security-scan-detail-card.is-impact,
:root.dark .security-scan-item-icon.is-failed,
[data-theme='dark'] .security-scan-item-icon.is-failed,
html.dark .security-scan-item-icon.is-failed {
  border-color: rgba(239, 68, 68, 0.22);
  background: rgba(127, 29, 29, 0.4);
}

:root.dark .security-scan-progress-track,
[data-theme='dark'] .security-scan-progress-track,
html.dark .security-scan-progress-track {
  background: rgba(148, 163, 184, 0.14);
}

:root.dark .security-scan-results-toggle:hover,
[data-theme='dark'] .security-scan-results-toggle:hover,
html.dark .security-scan-results-toggle:hover {
  background: rgba(148, 163, 184, 0.08);
}

:root.dark .security-scan-category-pill,
[data-theme='dark'] .security-scan-category-pill,
html.dark .security-scan-category-pill {
  background: rgba(148, 163, 184, 0.1);
}

:root.dark .security-scan-item:hover,
[data-theme='dark'] .security-scan-item:hover,
html.dark .security-scan-item:hover {
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(30, 41, 59, 0.58);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 18px 28px -30px rgba(2, 6, 23, 0.6);
}

:root.dark .security-scan-item.is-expanded,
[data-theme='dark'] .security-scan-item.is-expanded,
html.dark .security-scan-item.is-expanded {
  border-color: rgba(96, 165, 250, 0.22);
  background: rgba(30, 41, 59, 0.82);
}

:root.dark .security-scan-status-pill.is-passed,
[data-theme='dark'] .security-scan-status-pill.is-passed,
html.dark .security-scan-status-pill.is-passed {
  color: rgb(167 243 208);
  border-color: rgba(16, 185, 129, 0.24);
  background: rgba(6, 78, 59, 0.44);
}

:root.dark .security-scan-status-pill.is-warning,
[data-theme='dark'] .security-scan-status-pill.is-warning,
html.dark .security-scan-status-pill.is-warning {
  color: rgb(253 224 71);
  border-color: rgba(245, 158, 11, 0.24);
  background: rgba(120, 53, 15, 0.42);
}

:root.dark .security-scan-status-pill.is-failed,
[data-theme='dark'] .security-scan-status-pill.is-failed,
html.dark .security-scan-status-pill.is-failed {
  color: rgb(252 165 165);
  border-color: rgba(239, 68, 68, 0.24);
  background: rgba(127, 29, 29, 0.42);
}

:root.dark .security-scan-status-pill.is-scanning,
[data-theme='dark'] .security-scan-status-pill.is-scanning,
html.dark .security-scan-status-pill.is-scanning {
  color: rgb(203 213 225);
  border-color: rgba(148, 163, 184, 0.16);
  background: rgba(51, 65, 85, 0.52);
}

:root.dark .security-scan-inline-action,
[data-theme='dark'] .security-scan-inline-action,
html.dark .security-scan-inline-action {
  border-color: rgba(148, 163, 184, 0.14);
  background: rgba(30, 41, 59, 0.78);
}

:root.dark .security-scan-inline-action:hover,
[data-theme='dark'] .security-scan-inline-action:hover,
html.dark .security-scan-inline-action:hover {
  color: rgb(191 219 254);
  border-color: rgba(96, 165, 250, 0.22);
  background: rgba(30, 58, 138, 0.24);
}

:root.dark .security-scan-inline-note,
[data-theme='dark'] .security-scan-inline-note,
html.dark .security-scan-inline-note {
  color: rgb(253 186 116);
  border-color: rgba(249, 115, 22, 0.22);
  background: rgba(124, 45, 18, 0.42);
}

@media (max-width: 1100px) {
  .security-page {
    padding-top: 4rem;
  }

  .security-tab-nav {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .security-hero-heading {
    flex-direction: column;
    align-items: flex-start;
  }

  .security-tab-nav {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .security-status-banner {
    flex-direction: column;
    align-items: stretch;
  }

  .security-status-metrics {
    width: 100%;
  }

  .security-scan-item-main {
    flex-wrap: wrap;
  }

  .security-scan-item-meta {
    width: 100%;
    justify-content: flex-start;
    margin-left: 0;
  }

  .security-scan-item-details {
    padding-left: 1rem;
  }

  .security-scan-detail-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .security-page {
    padding-inline: 0.75rem;
    padding-bottom: 1rem;
  }

  .security-stage {
    padding-top: 0.7rem;
  }

  .security-stage::before,
  .security-stage::after {
    width: 12rem;
    height: 12rem;
    background-size: 12px 12px;
  }

  .security-stage::before {
    right: 0.2rem;
    top: 9.5rem;
  }

  .security-stage::after {
    left: 0;
    bottom: 0.5rem;
  }

  .security-hero {
    gap: 0.82rem;
    padding-bottom: 1.1rem;
  }

  .security-tab-shell,
  .security-panel,
  .security-status-banner {
    border-radius: 1.3rem;
  }

  .security-panel {
    padding: 1.05rem;
  }

  .security-tab-nav {
    grid-template-columns: 1fr;
  }

  .security-tab-button {
    grid-template-columns: auto minmax(0, 1fr) auto;
    padding: 14px 16px;
  }

  .security-scan-actions {
    width: 100%;
  }

  .security-scan-action {
    flex: 1 1 100%;
  }

  .security-scan-progress-meta {
    flex-direction: column;
    align-items: flex-start;
  }

  .security-scan-summary-grid {
    grid-template-columns: 1fr;
  }
}
</style>
