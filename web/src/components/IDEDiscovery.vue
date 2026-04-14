<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { providerPoolApi } from '@/api/providerPool'
import type { ImportConfig, IDEScanResult } from '@/api/providerPool'
import { publicAsset } from '@/utils/publicAsset'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'import-success', providerId: string): void
}>()

// Temporary switch: keep IDE import, but disable OAuth token import.
const IDE_OAUTH_IMPORT_ENABLED = false

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
  { ide_type: 'vscode', ide_name: 'VS Code', status: 'pending' },
  { ide_type: 'claude-code', ide_name: 'Claude Code', status: 'pending' },
  { ide_type: 'cursor', ide_name: 'Cursor', status: 'pending' },
  { ide_type: 'windsurf', ide_name: 'Windsurf', status: 'pending' },
  { ide_type: 'codex', ide_name: 'Codex CLI (OpenAI)', status: 'pending' },
  { ide_type: 'antigravity', ide_name: 'Antigravity', status: 'pending' },
  { ide_type: 'qoder', ide_name: 'Qoder', status: 'pending' },
  { ide_type: 'trae', ide_name: 'Trae', status: 'pending' },
  { ide_type: 'kiro', ide_name: 'Kiro', status: 'pending' },
  { ide_type: 'copilot', ide_name: 'GitHub Copilot', status: 'pending' },
])

// Importable configs from scan
const importableConfigs = ref<ImportConfig[]>([])

// IDE logo URLs
const ideLogos: Record<string, string> = {
  vscode: publicAsset('icons/ide/vscode.svg'),
  'claude-code': publicAsset('icons/ide/claude.svg'),
  cursor: publicAsset('icons/ide/cursor.svg'),
  windsurf: publicAsset('icons/ide/windsurf.svg'),
  codex: publicAsset('icons/ide/codex.svg'),
  antigravity: publicAsset('icons/ide/antigravity.svg'),
  qoder: publicAsset('icons/ide/qoder.svg'),
  trae: publicAsset('icons/ide/trae.svg'),
  kiro: publicAsset('icons/ide/kiro.svg'),
  copilot: publicAsset('icons/ide/copilot.svg'),
}

// IDE fallback icons
const ideIcons: Record<string, string> = {
  'claude-code': '🌼',
  cursor: '↗',
  windsurf: '🏄',
  antigravity: '🅰',
  qoder: '🆀',
  trae: '💬',
  kiro: '🐙',
  copilot: '🤖',
  codex: '🧠',
  vscode: '∞',
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
  return ideScanStates.value.filter((s) => s.status === 'found').length
})

const scanCompleted = computed(() => {
  return ideScanStates.value.every((s) => s.status === 'found' || s.status === 'not_found')
})

function canImportViaIDE(config: ImportConfig): boolean {
  if (!config.can_import || config.already_imported) return false
  if (config.extension_config) return true
  if (config.api_key) return true
  return IDE_OAUTH_IMPORT_ENABLED && !!config.has_oauth
}

const hasImportableConfigs = computed(() => {
  return canImportConfigs.value.length > 0
})

const canImportConfigs = computed(() => {
  return importableConfigs.value.filter(canImportViaIDE)
})

// OAuth configs that are already imported but can be re-imported to update tokens
const reimportableOAuthConfigs = computed(() => {
  if (!IDE_OAUTH_IMPORT_ENABLED) return []
  return importableConfigs.value.filter((c) => c.already_imported && c.has_oauth)
})

const alreadyImportedConfigs = computed(() => {
  return importableConfigs.value.filter(
    (c) => c.already_imported && (!c.has_oauth || !IDE_OAUTH_IMPORT_ENABLED)
  )
})

const installedOnlyConfigs = computed(() => {
  return importableConfigs.value.filter((c) => !canImportViaIDE(c) && !c.already_imported)
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
  ideScanStates.value.forEach((state) => {
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

    if (IDE_OAUTH_IMPORT_ENABLED) {
      // Notify parent if any OAuth configs were auto-imported
      const autoImported = importableConfigs.value.filter((c) => c.has_oauth && c.already_imported)
      if (autoImported.length > 0) {
        emit('import-success', 'oauth-auto')
      }
    }

    // Simulate progressive updates with small delays for better UX
    for (let i = 0; i < ideScanStates.value.length; i++) {
      const state = ideScanStates.value[i]
      if (!state) continue
      const result = scanResults.find((r) => r.ide_type === state.ide_type)

      // Small delay between each IDE result
      await new Promise((resolve) => setTimeout(resolve, 150))

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
    ideScanStates.value.forEach((state) => {
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

    // Check if this is an extension config import
    const config = importableConfigs.value.find((c) => c.ide_type === ideType)
    if (!config || !canImportViaIDE(config)) {
      error.value = t('ideDiscovery.importError')
      return
    }
    if (config.source === 'extension' && config.extension_config) {
      const response = await providerPoolApi.importExtensionConfig(ideType)
      success.value = t('ideDiscovery.importExtSuccess', {
        ide: ideType,
        count: response.data.providers.length,
      })
      emit('import-success', response.data.providers[0] || '')
    } else {
      const response = await providerPoolApi.importIDEConfig(ideType)
      success.value = t('ideDiscovery.importSuccess', {
        ide: ideType,
        provider: response.data.provider_id,
      })
      emit('import-success', response.data.provider_id)
    }

    // Remove from importable list
    importableConfigs.value = importableConfigs.value.filter((c) => c.ide_type !== ideType)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } } }
    error.value = err.response?.data?.error || t('ideDiscovery.importError')
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
    case 'extension':
      return t('ideDiscovery.sourceExtension')
    case 'oauth':
      return t('ideDiscovery.sourceOAuth')
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
        :disabled="scanning"
        class="px-4 py-2 text-sm font-medium text-white bg-gray-700 dark:bg-gray-500 rounded-lg hover:bg-gray-700 dark:bg-gray-500 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
        @click="startScan"
      >
        <svg
          v-if="scanning"
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
          />
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          />
        </svg>
        {{ scanning ? t('ideDiscovery.scanning') : t('ideDiscovery.scan') }}
      </button>
    </div>

    <!-- Alerts -->
    <div
      v-if="error"
      class="mb-4 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg"
    >
      <p class="text-sm text-red-600 dark:text-red-400">
        {{ error }}
      </p>
    </div>

    <div
      v-if="success"
      class="mb-4 p-4 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-lg"
    >
      <p class="text-sm text-green-600 dark:text-green-400">
        {{ success }}
      </p>
    </div>

    <!-- Scan Summary (after scan completes) -->
    <div
      v-if="scanCompleted"
      class="mb-4 p-3 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded-lg"
    >
      <div class="flex items-center gap-2 text-sm">
        <span class="text-green-500 font-medium">{{ foundCount }}</span>
        <span class="text-gray-400">/</span>
        <span class="text-gray-500">{{ ideScanStates.length }}</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{
            t('ideDiscovery.scanResultsSummary', { found: foundCount, total: ideScanStates.length })
          }}
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
            'bg-gray-50 dark:bg-gray-700/50 border-gray-200 dark:border-gray-700':
              state.status === 'pending',
            'bg-gray-100 dark:bg-gray-700/30 border-gray-200 dark:border-gray-600':
              state.status === 'scanning',
            'bg-green-50 dark:bg-green-900/20 border-green-200 dark:border-green-800':
              state.status === 'found',
            'bg-gray-50 dark:bg-gray-700/50 border-gray-200 dark:border-gray-700 opacity-60':
              state.status === 'not_found',
          }"
        >
          <!-- IDE Icon -->
          <div
            class="w-8 h-8 flex-shrink-0 flex items-center justify-center rounded-lg"
            :class="{
              'bg-gray-100 dark:bg-gray-700':
                state.status === 'pending' || state.status === 'not_found',
              'bg-gray-100 dark:bg-gray-700/30': state.status === 'scanning',
              'bg-white dark:bg-gray-700 shadow-sm': state.status === 'found',
            }"
          >
            <img
              v-if="getIDELogo(state.ide_type)"
              :src="getIDELogo(state.ide_type) ?? undefined"
              :alt="state.ide_name"
              class="w-5 h-5 object-contain"
              :class="{ grayscale: state.status === 'not_found' }"
              @error="($event.target as HTMLImageElement).style.display = 'none'"
            >
            <span
              v-else
              class="text-lg"
              :class="{ 'opacity-50': state.status === 'not_found' }"
            >{{
              getIDEIcon(state.ide_type)
            }}</span>
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
            <span
              v-if="state.status === 'pending'"
              class="text-xs text-gray-400 dark:text-gray-500"
            >
              {{ t('ideDiscovery.pending') }}
            </span>

            <!-- Scanning -->
            <svg
              v-else-if="state.status === 'scanning'"
              class="animate-spin h-4 w-4 text-gray-900 dark:text-white"
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
              />
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>

            <!-- Found -->
            <svg
              v-else-if="state.status === 'found'"
              class="w-4 h-4 text-green-500"
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

            <!-- Not Found -->
            <svg
              v-else-if="state.status === 'not_found'"
              class="w-4 h-4 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M20 12H4"
              />
            </svg>
          </div>
        </div>
      </div>
    </div>

    <!-- Importable Configs Section -->
    <div
      v-if="hasImportableConfigs"
      class="border-t border-gray-200 dark:border-gray-700 pt-6"
    >
      <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-4 flex items-center gap-2">
        <svg
          class="w-4 h-4 text-green-500"
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
        {{ t('ideDiscovery.availableConfigs') }} ({{ canImportConfigs.length }})
      </h4>

      <div class="space-y-3">
        <div
          v-for="config in canImportConfigs"
          :key="config.ide_type"
          class="p-4 bg-white dark:bg-gray-700 border border-gray-200 dark:border-gray-700 rounded-lg"
        >
          <div class="flex items-start justify-between mb-3">
            <div class="flex items-center gap-3">
              <div
                class="w-10 h-10 flex items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700"
              >
                <img
                  v-if="getIDELogo(config.ide_type)"
                  :src="getIDELogo(config.ide_type) ?? undefined"
                  :alt="config.ide_name"
                  class="w-6 h-6 object-contain"
                  @error="($event.target as HTMLImageElement).style.display = 'none'"
                >
                <span
                  v-else
                  class="text-xl"
                >{{ getIDEIcon(config.ide_type) }}</span>
              </div>
              <div>
                <h5 class="font-medium text-gray-900 dark:text-white">
                  {{ config.ide_name }}
                </h5>
                <div class="flex items-center gap-2 mt-1">
                  <span
                    :class="providerColors[config.provider || 'custom']"
                    class="px-2 py-0.5 text-xs font-medium rounded"
                  >
                    {{ config.provider }}
                  </span>
                  <span
                    class="px-2 py-0.5 text-xs font-medium rounded bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300"
                  >
                    {{ getSourceLabel(config.source) }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="space-y-1 text-sm text-gray-500 dark:text-gray-400 mb-4">
            <p v-if="config.has_oauth && config.oauth_email">
              <span class="font-medium">{{ t('ideDiscovery.oauthAccount') }}:</span>
              <span class="ide-discovery-inline-gap">{{ config.oauth_email }}</span>
            </p>
            <p v-if="config.has_oauth && config.oauth_type">
              <span class="font-medium">{{ t('ideDiscovery.oauthType') }}:</span>
              <span class="ide-discovery-inline-gap">{{ config.oauth_type }}</span>
            </p>
            <p v-if="config.api_key">
              <span class="font-medium">{{ t('ideDiscovery.apiKey') }}:</span>
              <code
                class="ide-discovery-inline-gap px-1 bg-gray-100 dark:bg-gray-700 rounded text-xs"
              >{{ config.api_key }}</code>
            </p>
            <p v-if="config.base_url">
              <span class="font-medium">{{ t('ideDiscovery.baseUrl') }}:</span>
              <span class="ide-discovery-inline-gap">{{ config.base_url }}</span>
            </p>
            <!-- Extension config env vars -->
            <template v-if="config.extension_config?.env_vars?.length">
              <p
                v-for="ev in config.extension_config.env_vars"
                :key="ev.name"
              >
                <span class="font-medium">{{ ev.name }}:</span>
                <code
                  class="ide-discovery-inline-gap px-1 bg-gray-100 dark:bg-gray-700 rounded text-xs"
                >{{ ev.value }}</code>
              </p>
            </template>
          </div>

          <button
            :disabled="
              importing === config.ide_type || (!config.api_key && !config.extension_config)
            "
            class="w-full px-4 py-2 text-sm font-medium text-white bg-gray-700 dark:bg-gray-500 rounded-lg hover:bg-gray-700 dark:bg-gray-500 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            @click="importConfig(config.ide_type)"
          >
            <svg
              v-if="importing === config.ide_type"
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
              />
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
            {{
              importing === config.ide_type ? t('ideDiscovery.importing') : t('ideDiscovery.import')
            }}
          </button>
        </div>
      </div>
    </div>

    <!-- Re-importable OAuth Configs Section -->
    <div
      v-if="reimportableOAuthConfigs.length > 0"
      class="border-t border-gray-200 dark:border-gray-700 pt-6"
    >
      <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
        <svg
          class="w-4 h-4 text-orange-500"
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
        {{ t('ideDiscovery.reimportOAuth') }} ({{ reimportableOAuthConfigs.length }})
      </h4>

      <div class="space-y-3">
        <div
          v-for="config in reimportableOAuthConfigs"
          :key="config.ide_type"
          class="p-4 bg-orange-50 dark:bg-orange-900/10 border border-orange-200 dark:border-orange-800 rounded-lg"
        >
          <div class="flex items-start justify-between mb-3">
            <div class="flex items-center gap-3">
              <div
                class="w-10 h-10 flex items-center justify-center rounded-lg bg-orange-100 dark:bg-orange-900/30"
              >
                <img
                  v-if="getIDELogo(config.ide_type)"
                  :src="getIDELogo(config.ide_type) ?? undefined"
                  :alt="config.ide_name"
                  class="w-6 h-6 object-contain"
                  @error="($event.target as HTMLImageElement).style.display = 'none'"
                >
                <span
                  v-else
                  class="text-xl"
                >{{ getIDEIcon(config.ide_type) }}</span>
              </div>
              <div>
                <h5 class="font-medium text-gray-900 dark:text-white">
                  {{ config.ide_name }}
                </h5>
                <div class="flex items-center gap-2 mt-1">
                  <span
                    class="px-2 py-0.5 text-xs font-medium rounded bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400"
                  >
                    {{ t('ideDiscovery.oauth') }}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="space-y-1 text-sm text-gray-500 dark:text-gray-400 mb-4">
            <p v-if="config.oauth_email">
              <span class="font-medium">{{ t('ideDiscovery.oauthAccount') }}:</span>
              <span class="ide-discovery-inline-gap">{{ config.oauth_email }}</span>
            </p>
            <p class="text-orange-600 dark:text-orange-400">
              {{ t('ideDiscovery.reimportHint') }}
            </p>
          </div>

          <button
            :disabled="importing === config.ide_type"
            class="w-full px-4 py-2 text-sm font-medium text-white bg-orange-600 hover:bg-orange-700 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            @click="importConfig(config.ide_type)"
          >
            <svg
              v-if="importing === config.ide_type"
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
              />
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
            {{
              importing === config.ide_type
                ? t('ideDiscovery.importing')
                : t('ideDiscovery.reimport')
            }}
          </button>
        </div>
      </div>
    </div>

    <!-- Already Imported Configs Section -->
    <div
      v-if="alreadyImportedConfigs.length > 0"
      class="border-t border-gray-200 dark:border-gray-700 pt-6"
    >
      <h4 class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-4 flex items-center gap-2">
        <svg
          class="w-4 h-4 text-blue-500"
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
        {{ t('ideDiscovery.alreadyImported') }} ({{ alreadyImportedConfigs.length }})
      </h4>

      <div class="space-y-2">
        <div
          v-for="config in alreadyImportedConfigs"
          :key="config.ide_type"
          class="p-3 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded-lg"
        >
          <div class="flex items-center gap-3">
            <div
              class="w-8 h-8 flex items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700"
            >
              <img
                v-if="getIDELogo(config.ide_type)"
                :src="getIDELogo(config.ide_type) ?? undefined"
                :alt="config.ide_name"
                class="w-5 h-5 object-contain"
              >
              <span
                v-else
                class="text-lg"
              >{{ getIDEIcon(config.ide_type) }}</span>
            </div>
            <div class="flex-1">
              <h5 class="text-sm font-medium text-gray-600 dark:text-gray-300">
                {{ config.ide_name }}
              </h5>
              <p
                v-if="config.api_key"
                class="text-xs text-gray-400 dark:text-gray-500"
              >
                <code class="px-1 bg-gray-100 dark:bg-gray-700 rounded">{{ config.api_key }}</code>
              </p>
            </div>
            <span
              class="px-2 py-1 text-xs font-medium rounded bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
            >
              {{ t('ideDiscovery.imported') }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Installed but not importable Section -->
    <div
      v-if="installedOnlyConfigs.length > 0"
      class="border-t border-gray-200 dark:border-gray-700 pt-6"
    >
      <h4 class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-4 flex items-center gap-2">
        <svg
          class="w-4 h-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
        {{ t('ideDiscovery.installedOnly') }} ({{ installedOnlyConfigs.length }})
      </h4>

      <div class="space-y-2">
        <div
          v-for="config in installedOnlyConfigs"
          :key="config.ide_type"
          class="p-3 bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded-lg"
        >
          <div class="flex items-center gap-3">
            <div
              class="w-8 h-8 flex items-center justify-center rounded-lg bg-gray-100 dark:bg-gray-700"
            >
              <img
                v-if="getIDELogo(config.ide_type)"
                :src="getIDELogo(config.ide_type) ?? undefined"
                :alt="config.ide_name"
                class="w-5 h-5 object-contain opacity-50"
              >
              <span
                v-else
                class="text-lg opacity-50"
              >{{ getIDEIcon(config.ide_type) }}</span>
            </div>
            <div class="flex-1">
              <h5 class="text-sm font-medium text-gray-600 dark:text-gray-300">
                {{ config.ide_name }}
              </h5>
              <p class="text-xs text-gray-400 dark:text-gray-500">
                {{
                  config.has_oauth
                    ? t('ideDiscovery.oauthManaged')
                    : t('ideDiscovery.noApiKeyFound')
                }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ide-discovery {
  padding: 1.5rem;
}

.ide-discovery-inline-gap {
  margin-inline-start: 0.25rem;
}
</style>
