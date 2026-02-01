<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { providerPoolApi } from '@/api/providerPool'
import type { ImportConfig, IDEScanResult } from '@/api/providerPool'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'import-success', providerId: string): void
}>()

// Scan state for each IDE
interface IDEScanState {
  ide_type: string
  ide_name: string
  status: 'pending' | 'scanning' | 'found' | 'not_found'
  config_path?: string
}

// State
const scanning = ref(false)
const importing = ref<string | null>(null)
const error = ref<string | null>(null)
const success = ref<string | null>(null)

// IDE scan states
const ideScanStates = ref<IDEScanState[]>([
  { ide_type: 'claude-code', ide_name: 'Claude Code', status: 'pending' },
  { ide_type: 'cursor', ide_name: 'Cursor', status: 'pending' },
  { ide_type: 'windsurf', ide_name: 'Windsurf', status: 'pending' },
  { ide_type: 'continue', ide_name: 'Continue', status: 'pending' },
  { ide_type: 'copilot', ide_name: 'GitHub Copilot', status: 'pending' },
  { ide_type: 'trae', ide_name: 'Trae', status: 'pending' },
  { ide_type: 'qoder', ide_name: 'Qoder', status: 'pending' },
  { ide_type: 'antigravity', ide_name: 'Antigravity', status: 'pending' },
])

// Importable configs from scan
const importableConfigs = ref<ImportConfig[]>([])

// IDE logo URLs
const ideLogos: Record<string, string> = {
  'claude-code': '/icons/ide/claude.svg',
  'cursor': '/icons/ide/cursor.svg',
  'windsurf': '/icons/ide/windsurf.svg',
  'antigravity': '/icons/ide/antigravity.svg',
  'qoder': '/icons/ide/qoder.svg',
  'trae': '/icons/ide/trae.svg',
  'continue': '/icons/ide/continue.png',
  'copilot': '/icons/ide/copilot.svg',
}

// IDE fallback icons
const ideIcons: Record<string, string> = {
  'claude-code': '🤖',
  'cursor': '⚡',
  'windsurf': '🏄',
  'antigravity': '🚀',
  'qoder': '💻',
  'trae': '🔧',
  'continue': '▶️',
  'copilot': '🐙',
}

// Provider colors
const providerColors: Record<string, string> = {
  anthropic: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
  openai: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
  google: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
  custom: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
  github: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
}

// Computed
const foundCount = computed(() => {
  return ideScanStates.value.filter(s => s.status === 'found').length
})

const scanCompleted = computed(() => {
  return ideScanStates.value.every(s => s.status === 'found' || s.status === 'not_found')
})

const hasImportableConfigs = computed(() => {
  return importableConfigs.value.length > 0
})

// Methods
function getIDELogo(ideType: string): string | null {
  return ideLogos[ideType] || null
}

function getIDEIcon(ideType: string): string {
  return ideIcons[ideType] || '🔧'
}

async function startScan() {
  if (scanning.value) return

  scanning.value = true
  error.value = null
  success.value = null
  importableConfigs.value = []

  // Reset all states to scanning
  ideScanStates.value.forEach(state => {
    state.status = 'scanning'
  })

  try {
    // Call the API
    const [idesResponse, configsResponse] = await Promise.all([
      providerPoolApi.scanIDEs(),
      providerPoolApi.getImportableConfigs(),
    ])

    const scanResults: IDEScanResult[] = idesResponse.data.scan_results || []
    importableConfigs.value = configsResponse.data.configs || []

    // Simulate progressive updates with small delays for better UX
    for (let i = 0; i < ideScanStates.value.length; i++) {
      const state = ideScanStates.value[i]
      if (!state) continue
      const result = scanResults.find(r => r.ide_type === state.ide_type)

      // Small delay between each IDE result
      await new Promise(resolve => setTimeout(resolve, 150))

      if (result) {
        state.status = result.found ? 'found' : 'not_found'
        state.config_path = result.config_path
      } else {
        state.status = 'not_found'
      }
    }
  } catch (e) {
    error.value = t('ideDiscovery.scanError')
    console.error('Failed to scan IDEs:', e)
    // Reset to pending on error
    ideScanStates.value.forEach(state => {
      state.status = 'pending'
    })
  } finally {
    scanning.value = false
  }
}

async function importConfig(ideType: string) {
  try {
    importing.value = ideType
    error.value = null
    success.value = null

    const response = await providerPoolApi.importIDEConfig(ideType)
    success.value = t('ideDiscovery.importSuccess', { ide: ideType, provider: response.data.provider_id })
    emit('import-success', response.data.provider_id)

    // Remove from importable list
    importableConfigs.value = importableConfigs.value.filter(c => c.ide_type !== ideType)
  } catch (e: any) {
    error.value = e.response?.data?.error || t('ideDiscovery.importError')
    console.error('Failed to import config:', e)
  } finally {
    importing.value = null
  }
}

function getSourceLabel(source: string): string {
  switch (source) {
    case 'config':
      return t('ideDiscovery.sourceConfig')
    case 'env':
      return t('ideDiscovery.sourceEnv')
    case 'cc-switch':
      return t('ideDiscovery.sourceCCSwitch')
    default:
      return source
  }
}
</script>

<template>
  <div class="ide-discovery">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('ideDiscovery.description') }}
      </p>
      <button
        @click="startScan"
        :disabled="scanning"
        class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
      >
        <svg v-if="scanning" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        {{ scanning ? t('ideDiscovery.scanning') : t('ideDiscovery.scan') }}
      </button>
    </div>

    <!-- Alerts -->
    <div v-if="error" class="mb-4 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
      <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </div>

    <div v-if="success" class="mb-4 p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg">
      <p class="text-sm text-green-600 dark:text-green-400">{{ success }}</p>
    </div>

    <!-- Scan Summary (after scan completes) -->
    <div v-if="scanCompleted" class="mb-4 p-3 bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 rounded-lg">
      <div class="flex items-center gap-2 text-sm">
        <span class="text-green-500 font-medium">{{ foundCount }}</span>
        <span class="text-gray-400">/</span>
        <span class="text-gray-500">{{ ideScanStates.length }}</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('ideDiscovery.scanResultsSummary', { found: foundCount, total: ideScanStates.length }) }}
        </span>
      </div>
    </div>

    <!-- IDE List -->
    <div class="mb-6">
      <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
        {{ t('ideDiscovery.supportedIDEs') }}
      </h4>
      <div class="grid grid-cols-2 gap-3">
        <div
          v-for="state in ideScanStates"
          :key="state.ide_type"
          class="flex items-center gap-2 p-3 rounded-lg border transition-all"
          :class="{
            'bg-gray-50 dark:bg-gray-800/50 border-gray-200 dark:border-gray-700': state.status === 'pending',
            'bg-blue-50 dark:bg-blue-900/20 border-blue-200 dark:border-blue-800': state.status === 'scanning',
            'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800': state.status === 'found',
            'bg-gray-50 dark:bg-gray-800/50 border-gray-200 dark:border-gray-700 opacity-60': state.status === 'not_found',
          }"
        >
          <!-- IDE Icon -->
          <div
            class="w-8 h-8 flex-shrink-0 flex items-center justify-center rounded-lg"
            :class="{
              'bg-gray-100 dark:bg-gray-700': state.status === 'pending' || state.status === 'not_found',
              'bg-blue-100 dark:bg-blue-800': state.status === 'scanning',
              'bg-white dark:bg-gray-700 shadow-sm': state.status === 'found',
            }"
          >
            <img
              v-if="getIDELogo(state.ide_type)"
              :src="getIDELogo(state.ide_type) ?? undefined"
              :alt="state.ide_name"
              class="w-5 h-5 object-contain"
              :class="{ 'grayscale': state.status === 'not_found' }"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            />
            <span v-else class="text-lg" :class="{ 'opacity-50': state.status === 'not_found' }">{{ getIDEIcon(state.ide_type) }}</span>
          </div>

          <!-- IDE Info -->
          <div class="flex-1 min-w-0">
            <span
              class="font-medium text-sm block truncate"
              :class="{
                'text-gray-900 dark:text-white': state.status !== 'not_found',
                'text-gray-500 dark:text-gray-400': state.status === 'not_found',
              }"
            >
              {{ state.ide_name }}
            </span>
          </div>

          <!-- Status Indicator -->
          <div class="flex-shrink-0">
            <!-- Pending -->
            <span v-if="state.status === 'pending'" class="text-xs text-gray-400 dark:text-gray-500">
              {{ t('ideDiscovery.pending') }}
            </span>

            <!-- Scanning -->
            <svg v-else-if="state.status === 'scanning'" class="animate-spin h-4 w-4 text-blue-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>

            <!-- Found -->
            <svg v-else-if="state.status === 'found'" class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>

            <!-- Not Found -->
            <svg v-else-if="state.status === 'not_found'" class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4" />
            </svg>
          </div>
        </div>
      </div>
    </div>

    <!-- Importable Configs Section -->
    <div v-if="hasImportableConfigs" class="border-t border-gray-200 dark:border-gray-700 pt-6">
      <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-4 flex items-center gap-2">
        <svg class="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        {{ t('ideDiscovery.availableConfigs') }} ({{ importableConfigs.length }})
      </h4>

      <div class="space-y-3">
        <div
          v-for="config in importableConfigs"
          :key="config.ide_type"
          class="p-4 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg"
        >
          <div class="flex items-start justify-between mb-3">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 flex items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700">
                <img
                  v-if="getIDELogo(config.ide_type)"
                  :src="getIDELogo(config.ide_type) ?? undefined"
                  :alt="config.ide_name"
                  class="w-6 h-6 object-contain"
                  @error="($event.target as HTMLImageElement).style.display = 'none'"
                />
                <span v-else class="text-xl">{{ getIDEIcon(config.ide_type) }}</span>
              </div>
              <div>
                <h5 class="font-medium text-gray-900 dark:text-white">{{ config.ide_name }}</h5>
                <div class="flex items-center gap-2 mt-1">
                  <span
                    :class="providerColors[config.provider || 'custom']"
                    class="px-2 py-0.5 text-xs font-medium rounded"
                  >
                    {{ config.provider }}
                  </span>
                  <span class="px-2 py-0.5 text-xs font-medium rounded bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                    {{ getSourceLabel(config.source) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="space-y-1 text-sm text-gray-500 dark:text-gray-400 mb-4">
            <p v-if="config.api_key">
              <span class="font-medium">{{ t('ideDiscovery.apiKey') }}:</span>
              <code class="ml-1 px-1 bg-gray-100 dark:bg-gray-700 rounded text-xs">{{ config.api_key }}</code>
            </p>
            <p v-if="config.base_url">
              <span class="font-medium">{{ t('ideDiscovery.baseUrl') }}:</span>
              <span class="ml-1">{{ config.base_url }}</span>
            </p>
          </div>

          <button
            @click="importConfig(config.ide_type)"
            :disabled="importing === config.ide_type || !config.api_key"
            class="w-full px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            <svg v-if="importing === config.ide_type" class="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ importing === config.ide_type ? t('ideDiscovery.importing') : t('ideDiscovery.import') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ide-discovery {
  padding: 1.5rem;
}
</style>
