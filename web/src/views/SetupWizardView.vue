<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { resetSetupStatus } from '@/router'

const { t } = useI18n()

interface Provider {
  id: string
  name: string
  requires_key: boolean
  models: string[]
  base_url: string
}

interface Language {
  code: string
  name: string
}

interface SetupConfig {
  language: string
  timezone: string
  llm_provider: string
  llm_api_key: string
  llm_base_url: string
  llm_model: string
  admin_username: string
  admin_password: string
  admin_password_confirm: string
  enable_mfa: boolean
  enable_home_assistant: boolean
  home_assistant_url: string
  home_assistant_token: string
  // Messaging channels
  enable_telegram: boolean
  telegram_token: string
  enable_discord: boolean
  discord_token: string
  discord_application_id: string
  enable_slack: boolean
  slack_bot_token: string
  slack_app_token: string
  slack_signing_secret: string
  enable_whatsapp: boolean
  whatsapp_phone_number: string
  enable_signal: boolean
  signal_phone_number: string
  enable_teams: boolean
  teams_app_id: string
  teams_app_password: string
  enable_googlechat: boolean
  googlechat_credentials_json: string
  enable_imessage: boolean
  enable_feishu: boolean
  feishu_app_id: string
  feishu_app_secret: string
  enable_wechat: boolean
  wechat_corp_id: string
  wechat_agent_id: string
  wechat_secret: string
  enable_matrix: boolean
  matrix_homeserver: string
  matrix_user_id: string
  matrix_access_token: string
}

interface ValidationErrors {
  [key: string]: string
}

const router = useRouter()

const currentStep = ref(1)
const totalSteps = 5
const loading = ref(false)
const testingConnection = ref(false)
const connectionTestResult = ref<{ success: boolean; message: string } | null>(null)

const providers = ref<Provider[]>([])
const languages = ref<Language[]>([])

// Detect browser language and map to supported language code
function detectBrowserLanguage(): string {
  const browserLang = navigator.language || navigator.languages?.[0] || 'en'
  // Extract primary language code (e.g., 'zh-CN' -> 'zh', 'en-US' -> 'en')
  const primaryLang = (browserLang.split('-')[0] || 'en').toLowerCase()
  // Map common language codes to supported ones (using underscore format to match server)
  const langMap: Record<string, string> = {
    'zh': 'zh_CN',
    'en': 'en',
    'ja': 'ja',
    'ko': 'ko',
    'de': 'de',
    'fr': 'fr',
    'es': 'es',
    'pt': 'pt',
    'ru': 'ru',
  }
  return langMap[primaryLang] || 'en'
}

// Detect browser timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

const config = ref<SetupConfig>({
  language: detectBrowserLanguage(),
  timezone: detectedTimezone,
  llm_provider: 'openai',
  llm_api_key: '',
  llm_base_url: '',
  llm_model: '',
  admin_username: '',
  admin_password: '',
  admin_password_confirm: '',
  enable_mfa: false,
  enable_home_assistant: false,
  home_assistant_url: '',
  home_assistant_token: '',
  // Messaging channels
  enable_telegram: false,
  telegram_token: '',
  enable_discord: false,
  discord_token: '',
  discord_application_id: '',
  enable_slack: false,
  slack_bot_token: '',
  slack_app_token: '',
  slack_signing_secret: '',
  enable_whatsapp: false,
  whatsapp_phone_number: '',
  enable_signal: false,
  signal_phone_number: '',
  enable_teams: false,
  teams_app_id: '',
  teams_app_password: '',
  enable_googlechat: false,
  googlechat_credentials_json: '',
  enable_imessage: false,
  enable_feishu: false,
  feishu_app_id: '',
  feishu_app_secret: '',
  enable_wechat: false,
  wechat_corp_id: '',
  wechat_agent_id: '',
  wechat_secret: '',
  enable_matrix: false,
  matrix_homeserver: '',
  matrix_user_id: '',
  matrix_access_token: '',
})

const errors = ref<ValidationErrors>({})

// Password strength indicators
const passwordChecks = computed(() => {
  const pwd = config.value.admin_password
  return {
    length: pwd.length >= 12,
    uppercase: /[A-Z]/.test(pwd),
    lowercase: /[a-z]/.test(pwd),
    number: /[0-9]/.test(pwd),
    special: /[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(pwd),
  }
})

const passwordStrength = computed(() => {
  const checks = passwordChecks.value
  return Object.values(checks).filter(Boolean).length
})

const stepTitles = computed(() => [
  t('setup.basicSettings'),
  t('setup.llmConfiguration'),
  t('setup.securitySettings'),
  t('setup.integrations'),
  t('setup.channels'),
])

const stepDescriptions = computed(() => [
  t('setup.basicSettingsDesc'),
  t('setup.llmConfigurationDesc'),
  t('setup.securitySettingsDesc'),
  t('setup.integrationsDesc'),
  t('setup.channelsDesc'),
])

const currentProvider = computed(() => {
  return providers.value.find(p => p.id === config.value.llm_provider)
})

const availableModels = computed(() => {
  return currentProvider.value?.models || []
})

const timezones = computed(() => {
  const allTimezones = (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone')
  // Put detected timezone at the top of the list
  const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
  return [detectedTimezone, ...filtered]
})

onMounted(async () => {
  await loadDefaults()
})

async function loadDefaults() {
  try {
    const response = await fetch('/api/setup/defaults')
    const data = await response.json()

    providers.value = data.providers || []
    languages.value = data.languages || []

    // Validate browser-detected language against supported languages
    // Only override if server provides a specific default or browser language is not supported
    if (languages.value.length > 0) {
      const supportedCodes = languages.value.map(l => l.code)
      if (!supportedCodes.includes(config.value.language)) {
        // Browser language not supported, use server default or first available
        config.value.language = data.language || supportedCodes[0] || 'en'
      }
    } else if (data.language) {
      config.value.language = data.language
    }

    if (data.llm_provider) config.value.llm_provider = data.llm_provider
    if (data.llm_model) config.value.llm_model = data.llm_model

    // Set default model for selected provider
    if (currentProvider.value && !config.value.llm_model) {
      config.value.llm_model = currentProvider.value.models[0] || ''
    }
  } catch (error) {
    console.error('Failed to load defaults:', error)
  }
}

function validateCurrentStep(): boolean {
  errors.value = {}

  switch (currentStep.value) {
    case 1:
      if (!config.value.language) {
        errors.value.language = t('setup.validation.selectLanguage')
      }
      if (!config.value.timezone) {
        errors.value.timezone = t('setup.validation.selectTimezone')
      }
      break

    case 2:
      if (!config.value.llm_provider) {
        errors.value.llm_provider = t('setup.validation.selectProvider')
      }
      if (currentProvider.value?.requires_key && !config.value.llm_api_key) {
        errors.value.llm_api_key = t('setup.validation.apiKeyRequired')
      }
      if (!config.value.llm_model) {
        errors.value.llm_model = t('setup.validation.selectModel')
      }
      if (config.value.llm_provider === 'custom' && !config.value.llm_base_url) {
        errors.value.llm_base_url = t('setup.validation.baseUrlRequired')
      }
      break

    case 3:
      if (!config.value.admin_username) {
        errors.value.admin_username = t('setup.validation.usernameRequired')
      } else if (config.value.admin_username.length < 3) {
        errors.value.admin_username = t('setup.validation.usernameMinLength')
      }
      if (!config.value.admin_password) {
        errors.value.admin_password = t('setup.validation.passwordRequired')
      } else {
        // Validate password policy
        const pwd = config.value.admin_password
        const pwdErrors: string[] = []
        if (pwd.length < 12) {
          pwdErrors.push(t('setup.validation.atLeast12Chars'))
        }
        if (!/[A-Z]/.test(pwd)) {
          pwdErrors.push(t('setup.validation.oneUppercase'))
        }
        if (!/[a-z]/.test(pwd)) {
          pwdErrors.push(t('setup.validation.oneLowercase'))
        }
        if (!/[0-9]/.test(pwd)) {
          pwdErrors.push(t('setup.validation.oneNumber'))
        }
        if (!/[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(pwd)) {
          pwdErrors.push(t('setup.validation.oneSpecialChar'))
        }
        if (pwdErrors.length > 0) {
          errors.value.admin_password = t('setup.validation.passwordMustContain', { requirements: pwdErrors.join(', ') })
        }
      }
      if (config.value.admin_password !== config.value.admin_password_confirm) {
        errors.value.admin_password_confirm = t('setup.validation.passwordsNotMatch')
      }
      break

    case 4:
      if (config.value.enable_home_assistant) {
        if (!config.value.home_assistant_url) {
          errors.value.home_assistant_url = t('setup.validation.homeAssistantUrlRequired')
        }
        if (!config.value.home_assistant_token) {
          errors.value.home_assistant_token = t('setup.validation.accessTokenRequired')
        }
      }
      break

    case 5:
      if (config.value.enable_telegram && !config.value.telegram_token) {
        errors.value.telegram_token = t('setup.validation.botTokenRequired')
      }
      if (config.value.enable_discord && !config.value.discord_token) {
        errors.value.discord_token = t('setup.validation.botTokenRequired')
      }
      break
  }

  return Object.keys(errors.value).length === 0
}

function nextStep() {
  if (!validateCurrentStep()) return

  if (currentStep.value < totalSteps) {
    currentStep.value++
    connectionTestResult.value = null
  }
}

function prevStep() {
  if (currentStep.value > 1) {
    currentStep.value--
    connectionTestResult.value = null
  }
}

async function testConnection(type: string) {
  testingConnection.value = true
  connectionTestResult.value = null

  let testConfig: Record<string, string> = {}

  switch (type) {
    case 'llm':
      testConfig = {
        provider: config.value.llm_provider,
        api_key: config.value.llm_api_key,
        base_url: config.value.llm_base_url || currentProvider.value?.base_url || '',
      }
      break
    case 'homeassistant':
      testConfig = {
        url: config.value.home_assistant_url,
        token: config.value.home_assistant_token,
      }
      break
    case 'telegram':
      testConfig = {
        token: config.value.telegram_token,
      }
      break
    case 'discord':
      testConfig = {
        token: config.value.discord_token,
      }
      break
  }

  try {
    const response = await fetch('/api/setup/test-connection', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type, config: testConfig }),
    })
    connectionTestResult.value = await response.json()
  } catch (error) {
    connectionTestResult.value = {
      success: false,
      message: t('setup.validation.connectionTestFailed'),
    }
  } finally {
    testingConnection.value = false
  }
}

async function completeSetup() {
  if (!validateCurrentStep()) return

  loading.value = true

  try {
    const response = await fetch('/api/setup/complete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config.value),
    })

    const result = await response.json()

    if (result.success) {
      resetSetupStatus()
      router.push('/')
    } else {
      errors.value.submit = result.message || t('setup.validation.setupFailed')
    }
  } catch (error) {
    errors.value.submit = t('setup.validation.failedToCompleteSetup')
  } finally {
    loading.value = false
  }
}

function onProviderChange() {
  // Reset model when provider changes
  config.value.llm_model = currentProvider.value?.models[0] || ''
  config.value.llm_base_url = currentProvider.value?.base_url || ''
  connectionTestResult.value = null
}

// Get translated provider name
function getProviderDisplayName(providerId: string): string {
  const key = `settings.providers.${providerId.toLowerCase()}`
  const translated = t(key)
  // If translation key doesn't exist, return original name from provider data
  if (translated === key) {
    const provider = providers.value.find(p => p.id === providerId)
    return provider?.name || providerId
  }
  return translated
}
</script>

<template>
  <div class="min-h-screen bg-gradient-to-br from-gray-100 via-gray-50 to-gray-100 dark:from-gray-900 dark:via-gray-800 dark:to-gray-900 flex items-center justify-center p-4">
    <div class="w-full max-w-2xl">
      <!-- Header -->
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-gray-900 dark:text-white mb-2">{{ t('setup.title') }}</h1>
        <p class="text-gray-500 dark:text-gray-400">{{ t('setup.subtitle') }}</p>
      </div>

      <!-- Progress Bar -->
      <div class="mb-8">
        <div class="flex justify-between mb-2">
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('setup.stepOf', { current: currentStep, total: totalSteps }) }}</span>
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ stepTitles[currentStep - 1] }}</span>
        </div>
        <div class="h-2 bg-gray-300 dark:bg-gray-700 rounded-full overflow-hidden">
          <div
            class="h-full bg-blue-500 transition-all duration-300"
            :style="{ width: `${(currentStep / totalSteps) * 100}%` }"
          />
        </div>
      </div>

      <!-- Card -->
      <div class="bg-white dark:bg-gray-800 rounded-2xl shadow-xl p-8">
        <!-- Step Title -->
        <div class="mb-6">
          <h2 class="text-xl font-semibold text-gray-900 dark:text-white">{{ stepTitles[currentStep - 1] }}</h2>
          <p class="text-gray-500 dark:text-gray-400 text-sm mt-1">{{ stepDescriptions[currentStep - 1] }}</p>
        </div>

        <!-- Step 1: Basic Settings -->
        <div v-if="currentStep === 1" class="space-y-6">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('common.language') }}</label>
            <select
              v-model="config.language"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option v-for="lang in languages" :key="lang.code" :value="lang.code">
                {{ lang.name }}
              </option>
            </select>
            <p v-if="errors.language" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.language }}</p>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.timezone') }}</label>
            <select
              v-model="config.timezone"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
            </select>
            <p v-if="errors.timezone" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.timezone }}</p>
          </div>
        </div>

        <!-- Step 2: LLM Configuration -->
        <div v-if="currentStep === 2" class="space-y-6">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.llmProvider') }}</label>
            <select
              v-model="config.llm_provider"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="onProviderChange"
            >
              <option v-for="provider in providers" :key="provider.id" :value="provider.id">
                {{ getProviderDisplayName(provider.id) }}
              </option>
            </select>
            <p v-if="errors.llm_provider" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.llm_provider }}</p>
          </div>

          <div v-if="currentProvider?.requires_key">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.apiKey') }}</label>
            <input
              v-model="config.llm_api_key"
              type="password"
              :placeholder="t('setup.enterApiKey')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p v-if="errors.llm_api_key" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.llm_api_key }}</p>
          </div>

          <div v-if="config.llm_provider === 'custom'">
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.baseUrl') }}</label>
            <input
              v-model="config.llm_base_url"
              type="url"
              :placeholder="t('setup.baseUrlPlaceholder')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p v-if="errors.llm_base_url" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.llm_base_url }}</p>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.model') }}</label>
            <select
              v-if="availableModels.length > 0"
              v-model="config.llm_model"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option v-for="model in availableModels" :key="model" :value="model">{{ model }}</option>
            </select>
            <input
              v-else
              v-model="config.llm_model"
              type="text"
              :placeholder="t('setup.enterModelName')"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p v-if="errors.llm_model" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.llm_model }}</p>
          </div>

          <div>
            <button
              :disabled="testingConnection"
              class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50"
              @click="testConnection('llm')"
            >
              {{ testingConnection ? t('setup.testing') : t('setup.testConnection') }}
            </button>
            <div v-if="connectionTestResult" class="mt-2">
              <p :class="connectionTestResult.success ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                {{ connectionTestResult.message }}
              </p>
            </div>
          </div>
        </div>

        <!-- Step 3: Security Settings -->
        <div v-if="currentStep === 3" class="space-y-6">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.adminUsername') }}</label>
            <input
              v-model="config.admin_username"
              type="text"
              placeholder="admin"
              autocomplete="username"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p v-if="errors.admin_username" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.admin_username }}</p>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.adminPassword') }}</label>
            <input
              v-model="config.admin_password"
              type="password"
              :placeholder="t('setup.strongPassword')"
              autocomplete="new-password"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <!-- Password Strength Indicator -->
            <div v-if="config.admin_password" class="mt-3 space-y-2">
              <!-- Strength Bar -->
              <div class="flex gap-1">
                <div
                  v-for="i in 5"
                  :key="i"
                  class="h-1.5 flex-1 rounded-full transition-colors"
                  :class="i <= passwordStrength ? 'bg-green-500' : 'bg-gray-300 dark:bg-gray-600'"
                />
              </div>
              <!-- Requirements Checklist -->
              <div class="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                <div class="flex items-center gap-1.5">
                  <span :class="passwordChecks.length ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'">
                    {{ passwordChecks.length ? '✓' : '○' }}
                  </span>
                  <span :class="passwordChecks.length ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'">
                    {{ t('setup.passwordCheck.length') }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span :class="passwordChecks.uppercase ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'">
                    {{ passwordChecks.uppercase ? '✓' : '○' }}
                  </span>
                  <span :class="passwordChecks.uppercase ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'">
                    {{ t('setup.passwordCheck.uppercase') }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span :class="passwordChecks.lowercase ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'">
                    {{ passwordChecks.lowercase ? '✓' : '○' }}
                  </span>
                  <span :class="passwordChecks.lowercase ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'">
                    {{ t('setup.passwordCheck.lowercase') }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span :class="passwordChecks.number ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'">
                    {{ passwordChecks.number ? '✓' : '○' }}
                  </span>
                  <span :class="passwordChecks.number ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'">
                    {{ t('setup.passwordCheck.number') }}
                  </span>
                </div>
                <div class="flex items-center gap-1.5 col-span-2">
                  <span :class="passwordChecks.special ? 'text-green-500' : 'text-gray-400 dark:text-gray-500'">
                    {{ passwordChecks.special ? '✓' : '○' }}
                  </span>
                  <span :class="passwordChecks.special ? 'text-green-600 dark:text-green-400' : 'text-gray-500 dark:text-gray-400'">
                    {{ t('setup.passwordCheck.special') }}
                  </span>
                </div>
              </div>
            </div>
            <p v-else class="text-gray-500 dark:text-gray-400 text-xs mt-1">
              {{ t('setup.passwordRequirements') }}
            </p>
            <p v-if="errors.admin_password" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.admin_password }}</p>
          </div>

          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.confirmPassword') }}</label>
            <input
              v-model="config.admin_password_confirm"
              type="password"
              :placeholder="t('setup.confirmPasswordHint')"
              autocomplete="new-password"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p v-if="errors.admin_password_confirm" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.admin_password_confirm }}</p>
          </div>

          <div class="flex items-center gap-3">
            <input
              id="enable_mfa"
              v-model="config.enable_mfa"
              type="checkbox"
              class="w-5 h-5 rounded bg-gray-100 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
            />
            <label for="enable_mfa" class="text-gray-700 dark:text-gray-300">{{ t('setup.enableMfa') }}</label>
          </div>
        </div>

        <!-- Step 4: Integrations -->
        <div v-if="currentStep === 4" class="space-y-6">
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_ha"
                v-model="config.enable_home_assistant"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_ha" class="text-gray-900 dark:text-white font-medium">{{ t('setup.homeAssistantIntegration') }}</label>
            </div>

            <div v-if="config.enable_home_assistant" class="space-y-4 pl-8">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.homeAssistantUrl') }}</label>
                <input
                  v-model="config.home_assistant_url"
                  type="url"
                  :placeholder="t('setup.homeAssistantUrlPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.home_assistant_url" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.home_assistant_url }}</p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.accessToken') }}</label>
                <input
                  v-model="config.home_assistant_token"
                  type="password"
                  :placeholder="t('setup.enterAccessToken')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.home_assistant_token" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.home_assistant_token }}</p>
              </div>

              <button
                :disabled="testingConnection"
                class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50"
                @click="testConnection('homeassistant')"
              >
                {{ testingConnection ? t('setup.testing') : t('setup.testConnection') }}
              </button>
            </div>
          </div>

          <p class="text-gray-500 dark:text-gray-400 text-sm">
            {{ t('setup.configureMoreLater') }}
          </p>
        </div>

        <!-- Step 5: Channels -->
        <div v-if="currentStep === 5" class="space-y-6">
          <!-- Telegram -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_telegram"
                v-model="config.enable_telegram"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_telegram" class="text-gray-900 dark:text-white font-medium">{{ t('setup.telegramBot') }}</label>
            </div>

            <div v-if="config.enable_telegram" class="space-y-4 pl-8">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.botToken') }}</label>
                <input
                  v-model="config.telegram_token"
                  type="password"
                  :placeholder="t('setup.telegramTokenPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.telegram_token" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.telegram_token }}</p>
              </div>

              <button
                :disabled="testingConnection"
                class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50"
                @click="testConnection('telegram')"
              >
                {{ testingConnection ? t('setup.testing') : t('setup.testConnection') }}
              </button>
            </div>
          </div>

          <!-- Discord -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_discord"
                v-model="config.enable_discord"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_discord" class="text-gray-900 dark:text-white font-medium">{{ t('setup.discordBot') }}</label>
            </div>

            <div v-if="config.enable_discord" class="space-y-4 pl-8">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.botToken') }}</label>
                <input
                  v-model="config.discord_token"
                  type="password"
                  :placeholder="t('setup.discordTokenPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.discord_token" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.discord_token }}</p>
              </div>

              <button
                :disabled="testingConnection"
                class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50"
                @click="testConnection('discord')"
              >
                {{ testingConnection ? t('setup.testing') : t('setup.testConnection') }}
              </button>
            </div>
          </div>

          <!-- Feishu -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_feishu"
                v-model="config.enable_feishu"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_feishu" class="text-gray-900 dark:text-white font-medium">{{ t('setup.feishuBot') }}</label>
            </div>

            <div v-if="config.enable_feishu" class="space-y-4 pl-8">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.appId') }}</label>
                <input
                  v-model="config.feishu_app_id"
                  type="text"
                  :placeholder="t('setup.feishuAppIdPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.feishu_app_id" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.feishu_app_id }}</p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.appSecret') }}</label>
                <input
                  v-model="config.feishu_app_secret"
                  type="password"
                  :placeholder="t('setup.feishuAppSecretPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.feishu_app_secret" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.feishu_app_secret }}</p>
              </div>

              <button
                :disabled="testingConnection"
                class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50"
                @click="testConnection('feishu')"
              >
                {{ testingConnection ? t('setup.testing') : t('setup.testConnection') }}
              </button>
            </div>
          </div>

          <!-- WeChat Work -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_wechat"
                v-model="config.enable_wechat"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_wechat" class="text-gray-900 dark:text-white font-medium">{{ t('setup.wechatWorkBot') }}</label>
            </div>

            <div v-if="config.enable_wechat" class="space-y-4 pl-8">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.corpId') }}</label>
                <input
                  v-model="config.wechat_corp_id"
                  type="text"
                  :placeholder="t('setup.wechatCorpIdPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.wechat_corp_id" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.wechat_corp_id }}</p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.agentId') }}</label>
                <input
                  v-model="config.wechat_agent_id"
                  type="text"
                  :placeholder="t('setup.wechatAgentIdPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.wechat_agent_id" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.wechat_agent_id }}</p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.secret') }}</label>
                <input
                  v-model="config.wechat_secret"
                  type="password"
                  :placeholder="t('setup.wechatSecretPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p v-if="errors.wechat_secret" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.wechat_secret }}</p>
              </div>

              <button
                :disabled="testingConnection"
                class="px-4 py-2 bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-900 dark:text-white rounded-lg transition-colors disabled:opacity-50"
                @click="testConnection('wechat')"
              >
                {{ testingConnection ? t('setup.testing') : t('setup.testConnection') }}
              </button>
            </div>
          </div>

          <!-- Slack -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_slack"
                v-model="config.enable_slack"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_slack" class="text-gray-900 dark:text-white font-medium">{{ t('setup.slackBot') }}</label>
            </div>

            <div v-if="config.enable_slack" class="space-y-4 pl-8">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">{{ t('setup.slackHint') }}</p>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.slackBotToken') }}</label>
                <input
                  v-model="config.slack_bot_token"
                  type="password"
                  :placeholder="t('setup.slackBotTokenPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.slackAppToken') }}</label>
                <input
                  v-model="config.slack_app_token"
                  type="password"
                  :placeholder="t('setup.slackAppTokenPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
          </div>

          <!-- WhatsApp -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_whatsapp"
                v-model="config.enable_whatsapp"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_whatsapp" class="text-gray-900 dark:text-white font-medium">{{ t('setup.whatsappBot') }}</label>
            </div>

            <div v-if="config.enable_whatsapp" class="space-y-4 pl-8">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">{{ t('setup.whatsappHint') }}</p>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.phoneNumber') }}</label>
                <input
                  v-model="config.whatsapp_phone_number"
                  type="tel"
                  :placeholder="t('setup.phoneNumberPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
          </div>

          <!-- Signal -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_signal"
                v-model="config.enable_signal"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_signal" class="text-gray-900 dark:text-white font-medium">{{ t('setup.signalBot') }}</label>
            </div>

            <div v-if="config.enable_signal" class="space-y-4 pl-8">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">{{ t('setup.signalHint') }}</p>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.phoneNumber') }}</label>
                <input
                  v-model="config.signal_phone_number"
                  type="tel"
                  :placeholder="t('setup.phoneNumberPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
          </div>

          <!-- Microsoft Teams -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_teams"
                v-model="config.enable_teams"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_teams" class="text-gray-900 dark:text-white font-medium">{{ t('setup.teamsBot') }}</label>
            </div>

            <div v-if="config.enable_teams" class="space-y-4 pl-8">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">{{ t('setup.teamsHint') }}</p>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.appId') }}</label>
                <input
                  v-model="config.teams_app_id"
                  type="text"
                  :placeholder="t('setup.teamsAppIdPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.appPassword') }}</label>
                <input
                  v-model="config.teams_app_password"
                  type="password"
                  :placeholder="t('setup.teamsAppPasswordPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
          </div>

          <!-- Google Chat -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_googlechat"
                v-model="config.enable_googlechat"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_googlechat" class="text-gray-900 dark:text-white font-medium">{{ t('setup.googleChatBot') }}</label>
            </div>

            <div v-if="config.enable_googlechat" class="space-y-4 pl-8">
              <p class="text-xs text-gray-500 dark:text-gray-400 mb-2">{{ t('setup.googleChatHint') }}</p>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.serviceAccountJson') }}</label>
                <textarea
                  v-model="config.googlechat_credentials_json"
                  rows="4"
                  :placeholder="t('setup.serviceAccountJsonPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono text-sm"
                />
              </div>
            </div>
          </div>

          <!-- iMessage (macOS only) -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_imessage"
                v-model="config.enable_imessage"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_imessage" class="text-gray-900 dark:text-white font-medium">{{ t('setup.imessageBot') }}</label>
            </div>

            <div v-if="config.enable_imessage" class="pl-8">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('setup.imessageHint') }}</p>
            </div>
          </div>

          <!-- Matrix -->
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_matrix"
                v-model="config.enable_matrix"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <label for="enable_matrix" class="text-gray-900 dark:text-white font-medium">{{ t('setup.matrixBot') }}</label>
            </div>

            <div v-if="config.enable_matrix" class="space-y-4 pl-8">
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.matrixHomeserver') }}</label>
                <input
                  v-model="config.matrix_homeserver"
                  type="url"
                  :placeholder="t('setup.matrixHomeserverPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.matrixUserId') }}</label>
                <input
                  v-model="config.matrix_user_id"
                  type="text"
                  :placeholder="t('setup.matrixUserIdPlaceholder')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
              <div>
                <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.accessToken') }}</label>
                <input
                  v-model="config.matrix_access_token"
                  type="password"
                  :placeholder="t('setup.enterAccessToken')"
                  class="w-full bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
          </div>

          <p class="text-gray-500 dark:text-gray-400 text-sm">
            {{ t('setup.enableMoreChannelsLater') }}
          </p>
        </div>

        <!-- Connection Test Result -->
        <div v-if="connectionTestResult && currentStep !== 2" class="mt-4">
          <p :class="connectionTestResult.success ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
            {{ connectionTestResult.message }}
          </p>
        </div>

        <!-- Submit Error -->
        <div v-if="errors.submit" class="mt-4 p-3 bg-red-100 dark:bg-red-500/20 border border-red-300 dark:border-red-500 rounded-lg">
          <p class="text-red-600 dark:text-red-400">{{ errors.submit }}</p>
        </div>

        <!-- Navigation Buttons -->
        <div class="flex justify-between mt-8">
          <button
            v-if="currentStep > 1"
            class="px-6 py-3 bg-gray-200 dark:bg-gray-700 hover:bg-gray-300 dark:hover:bg-gray-600 text-gray-900 dark:text-white rounded-lg transition-colors"
            @click="prevStep"
          >
            {{ t('common.back') }}
          </button>
          <div v-else />

          <button
            v-if="currentStep < totalSteps"
            class="px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
            @click="nextStep"
          >
            {{ t('setup.continue') }}
          </button>
          <button
            v-else
            :disabled="loading"
            class="px-6 py-3 bg-green-600 hover:bg-green-700 text-white rounded-lg transition-colors disabled:opacity-50"
            @click="completeSetup"
          >
            {{ loading ? t('setup.completing') : t('setup.completeSetup') }}
          </button>
        </div>
      </div>

      <!-- Skip Setup Link -->
      <div class="text-center mt-6">
        <button
          class="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 text-sm"
          @click="router.push('/login')"
        >
          {{ t('setup.skipSetup') }}
        </button>
      </div>
    </div>
  </div>
</template>
