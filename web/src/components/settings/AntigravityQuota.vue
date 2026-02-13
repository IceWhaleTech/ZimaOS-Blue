<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { antigravityApi, type QuotaData, type APIKeyInfo } from '@/api/antigravity'

const { t } = useI18n()

const loading = ref(false)
const error = ref<string | null>(null)
const quotaData = ref<QuotaData | null>(null)
const accessToken = ref('')
const showTokenInput = ref(false)
const detectedKeys = ref<APIKeyInfo[]>([])
const loadingKeys = ref(false)

// Group models by provider
const geminiModels = computed(() =>
  quotaData.value?.models.filter(m => m.group === 'gemini') || []
)

const claudeModels = computed(() =>
  quotaData.value?.models.filter(m => m.group === 'claude') || []
)

const lastUpdatedText = computed(() => {
  if (!quotaData.value?.last_updated) return ''
  const date = new Date(quotaData.value.last_updated * 1000)
  return date.toLocaleString()
})

// Get detected Antigravity token
const detectedAntigravityToken = computed(() =>
  detectedKeys.value.find(k => k.provider === 'antigravity')
)

// Check if trial key is available (configured or detected)
const hasTrialKey = computed(() => {
  return !!(accessToken.value || detectedAntigravityToken.value || quotaData.value)
})

// Get progress bar color based on percentage
function getProgressColor(percentage: number): string {
  if (percentage >= 70) return 'bg-green-500'
  if (percentage >= 30) return 'bg-yellow-500'
  return 'bg-red-500'
}

// Get text color based on percentage
function getTextColor(percentage: number): string {
  if (percentage >= 70) return 'text-green-600 dark:text-green-400'
  if (percentage >= 30) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
}

// Format model name for display
function formatModelName(name: string): string {
  return name
    .replace(/^models\//, '')
    .replace(/-/g, ' ')
    .replace(/\b\w/g, l => l.toUpperCase())
}

// Get provider icon color
function getProviderColor(provider: string): string {
  const colors: Record<string, string> = {
    anthropic: 'text-orange-500',
    openai: 'text-green-500',
    google: 'text-gray-900 dark:text-white',
    'azure-openai': 'text-cyan-500',
    cohere: 'text-purple-500',
    mistral: 'text-indigo-500',
    groq: 'text-pink-500',
    together: 'text-teal-500',
    perplexity: 'text-violet-500',
    fireworks: 'text-red-500',
    replicate: 'text-yellow-500',
    huggingface: 'text-amber-500',
    deepseek: 'text-sky-500',
    moonshot: 'text-slate-500',
    zhipu: 'text-emerald-500',
    baichuan: 'text-rose-500',
    qwen: 'text-lime-500',
    doubao: 'text-fuchsia-500',
    antigravity: 'text-cyan-400',
  }
  return colors[provider] || 'text-gray-500'
}

// Get source label
function getSourceLabel(key: APIKeyInfo): string {
  if (key.source === 'env') return key.env_var || 'ENV'
  if (key.source === 'ide-config') return key.ide_name || 'IDE'
  return 'File'
}

// Load detected API keys
async function loadDetectedKeys() {
  loadingKeys.value = true
  try {
    const res = await antigravityApi.getDetectedKeys()
    detectedKeys.value = res.data.keys
  } catch (e) {
    console.error('Failed to load detected keys:', e)
  } finally {
    loadingKeys.value = false
  }
}

// Load quota data
async function loadQuota() {
  if (!accessToken.value) {
    showTokenInput.value = true
    return
  }

  loading.value = true
  error.value = null

  try {
    const res = await antigravityApi.getQuota(accessToken.value)
    if (res.data.success) {
      quotaData.value = res.data.data ?? null
      showTokenInput.value = false
      localStorage.setItem('antigravity_token', accessToken.value)
    } else {
      error.value = res.data.error || 'Failed to fetch quota'
    }
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// Clear token and data
function clearToken() {
  accessToken.value = ''
  quotaData.value = null
  localStorage.removeItem('antigravity_token')
  showTokenInput.value = true
}

// Auto-refresh interval
let refreshTimer: number | null = null

function startAutoRefresh() {
  if (refreshTimer) clearInterval(refreshTimer)
  refreshTimer = window.setInterval(() => {
    if (accessToken.value && !loading.value) {
      loadQuota()
    }
  }, 5 * 60 * 1000)
}

onMounted(async () => {
  // Load detected API keys first
  await loadDetectedKeys()

  // Load saved token or use detected token
  const savedToken = localStorage.getItem('antigravity_token')
  if (savedToken) {
    accessToken.value = savedToken
    loadQuota()
  } else if (detectedAntigravityToken.value) {
    // Auto-detected token found - show notification but don't auto-use
    showTokenInput.value = true
  } else {
    showTokenInput.value = true
  }
  startAutoRefresh()
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <!-- Only show component if trial key is available -->
  <div v-if="hasTrialKey || loadingKeys" class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h3 class="text-lg font-medium text-gray-900 dark:text-white">
          {{ t('settings.antigravity.title') }}
        </h3>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('settings.antigravity.description') }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          v-if="quotaData"
          class="text-sm text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
          @click="clearToken"
        >
          {{ t('common.logout') }}
        </button>
        <button
          class="text-sm text-gray-900 dark:text-gray-300 hover:text-gray-900 dark:text-gray-300/80"
          :disabled="loading"
          @click="loadQuota"
        >
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <!-- Detected API Keys Section -->
    <div v-if="detectedKeys.length > 0" class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
      <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h4 class="font-medium text-gray-900 dark:text-white flex items-center gap-2">
          <svg class="w-5 h-5 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          {{ t('settings.antigravity.detectedKeys') }}
        </h4>
      </div>
      <div class="divide-y divide-gray-100 dark:divide-gray-700">
        <div
          v-for="key in detectedKeys"
          :key="`${key.provider}-${key.source}-${key.env_var || key.ide_name}`"
          class="px-4 py-3 flex items-center justify-between"
        >
          <div class="flex items-center gap-3">
            <span class="w-2 h-2 rounded-full bg-green-500"></span>
            <div>
              <span class="font-medium text-gray-900 dark:text-white capitalize" :class="getProviderColor(key.provider)">
                {{ key.provider }}
              </span>
              <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                {{ key.masked_value }}
              </span>
            </div>
          </div>
          <span class="px-2 py-1 text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded">
            {{ getSourceLabel(key) }}
          </span>
        </div>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
      <p class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </div>

    <!-- Token Input -->
    <div v-if="showTokenInput" class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        {{ t('settings.antigravity.accessToken') }}
      </label>
      <!-- Auto-detected token hint -->
      <div v-if="detectedAntigravityToken" class="mb-3 p-2 bg-green-50 dark:bg-green-900/20 rounded-lg">
        <p class="text-xs text-green-700 dark:text-green-400">
          {{ t('settings.antigravity.tokenDetected') }} ({{ detectedAntigravityToken.ide_name || detectedAntigravityToken.env_var }})
        </p>
      </div>
      <div class="flex gap-2">
        <input
          v-model="accessToken"
          type="password"
          class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-gray-400 focus:border-transparent"
          :placeholder="t('settings.antigravity.tokenPlaceholder')"
          @keyup.enter="loadQuota"
        />
        <button
          class="px-4 py-2 text-sm bg-gray-700 dark:bg-gray-500 text-white rounded-lg hover:bg-gray-800 dark:hover:bg-gray-400 disabled:opacity-50"
          :disabled="loading || !accessToken"
          @click="loadQuota"
        >
          {{ t('common.connect') }}
        </button>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">
        {{ t('settings.antigravity.tokenHint') }}
      </p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex items-center justify-center py-8">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-700"></div>
    </div>

    <!-- Quota Display -->
    <div v-else-if="quotaData" class="space-y-4">
      <!-- Subscription Tier -->
      <div v-if="quotaData.subscription_tier" class="flex items-center gap-2">
        <span class="text-sm text-gray-600 dark:text-gray-400">
          {{ t('settings.antigravity.tier') }}:
        </span>
        <span class="px-2 py-1 text-xs font-medium bg-gray-100 dark:bg-gray-600/10 text-gray-900 dark:text-gray-300 rounded">
          {{ quotaData.subscription_tier }}
        </span>
      </div>

      <!-- Gemini Models -->
      <div v-if="geminiModels.length > 0" class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center gap-2">
          <svg class="w-5 h-5 text-gray-900 dark:text-white" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"/>
          </svg>
          <h4 class="font-medium text-gray-900 dark:text-white">
            Gemini
          </h4>
          <span class="text-xs text-gray-500">({{ geminiModels.length }} {{ t('settings.antigravity.models') }})</span>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-gray-700">
          <div
            v-for="model in geminiModels"
            :key="model.name"
            class="px-4 py-3"
          >
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ formatModelName(model.name) }}
              </span>
              <span class="text-sm font-medium" :class="getTextColor(model.percentage)">
                {{ model.percentage.toFixed(0) }}%
              </span>
            </div>
            <div class="relative w-full bg-gray-700 dark:bg-gray-500 rounded-full h-4">
              <div
                class="h-4 rounded-full transition-all duration-300"
                :class="getProgressColor(model.percentage)"
                :style="{ width: `${Math.max(model.percentage, 3)}%` }"
              ></div>
              <span class="absolute inset-0 flex items-center justify-center text-xs font-medium text-gray-700 dark:text-gray-200">
                {{ model.percentage.toFixed(0) }}%
              </span>
            </div>
            <p v-if="model.reset_time" class="text-xs text-gray-500 mt-1">
              {{ t('settings.antigravity.resetAt') }}: {{ new Date(model.reset_time).toLocaleString() }}
            </p>
          </div>
        </div>
      </div>

      <!-- Claude Models -->
      <div v-if="claudeModels.length > 0" class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
        <div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex items-center gap-2">
          <svg class="w-5 h-5 text-orange-500" viewBox="0 0 24 24" fill="currentColor">
            <circle cx="12" cy="12" r="10"/>
          </svg>
          <h4 class="font-medium text-gray-900 dark:text-white">
            Claude
          </h4>
          <span class="text-xs text-gray-500">({{ claudeModels.length }} {{ t('settings.antigravity.models') }})</span>
        </div>
        <div class="divide-y divide-gray-100 dark:divide-gray-700">
          <div
            v-for="model in claudeModels"
            :key="model.name"
            class="px-4 py-3"
          >
            <div class="flex items-center justify-between mb-2">
              <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ formatModelName(model.name) }}
              </span>
              <span class="text-sm font-medium" :class="getTextColor(model.percentage)">
                {{ model.percentage.toFixed(0) }}%
              </span>
            </div>
            <div class="relative w-full bg-gray-700 dark:bg-gray-500 rounded-full h-4">
              <div
                class="h-4 rounded-full transition-all duration-300"
                :class="getProgressColor(model.percentage)"
                :style="{ width: `${Math.max(model.percentage, 3)}%` }"
              ></div>
              <span class="absolute inset-0 flex items-center justify-center text-xs font-medium text-gray-700 dark:text-gray-200">
                {{ model.percentage.toFixed(0) }}%
              </span>
            </div>
            <p v-if="model.reset_time" class="text-xs text-gray-500 mt-1">
              {{ t('settings.antigravity.resetAt') }}: {{ new Date(model.reset_time).toLocaleString() }}
            </p>
          </div>
        </div>
      </div>

      <!-- No Models -->
      <div v-if="geminiModels.length === 0 && claudeModels.length === 0" class="text-center py-8 text-gray-500">
        {{ t('settings.antigravity.noModels') }}
      </div>

      <!-- Last Updated -->
      <p v-if="lastUpdatedText" class="text-xs text-gray-500 text-right">
        {{ t('settings.antigravity.lastUpdated') }}: {{ lastUpdatedText }}
      </p>
    </div>
  </div>
</template>
