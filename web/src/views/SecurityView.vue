<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { securityApi } from '@/api/security'

const { t } = useI18n()

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

onMounted(() => {
  // Auto-start security scan on page load
  startSecurityScan()
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
</script>

<template>
  <div class="security-view p-4 sm:p-6 max-w-4xl mx-auto">
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

    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 dark:text-white mb-6">{{ t('security.title') }}</h1>

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
              <div class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ item.name }}</div>
              <div class="text-xs text-gray-500 dark:text-slate-400 truncate">{{ item.description }}</div>
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
                {{ item.status === 'scanning' ? t('security.scan.checking') : item.status }}
              </span>
            </div>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
