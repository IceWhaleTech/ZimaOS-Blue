<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { resetSetupStatus } from '@/router'
import { setLocale, type LocaleKey } from '@/i18n'

const { t } = useI18n()

interface Language {
  code: string
  name: string
}

interface SetupConfig {
  language: string
  timezone: string
  admin_username: string
  admin_password: string
  admin_password_confirm: string
  enable_mfa: boolean
  enable_home_assistant: boolean
  home_assistant_url: string
  home_assistant_token: string
}

interface ValidationErrors {
  [key: string]: string
}

const router = useRouter()

const currentStep = ref(1)
const totalSteps = 3
const loading = ref(false)
const testingConnection = ref(false)
const connectionTestResult = ref<{ success: boolean; message: string } | null>(null)
const checkingUsername = ref(false)
const usernameAvailable = ref<boolean | null>(null)
let usernameCheckTimeout: ReturnType<typeof setTimeout> | null = null

const languages = ref<Language[]>([])

// Detect browser language and map to supported language code
function detectBrowserLanguage(): string {
  const browserLang = navigator.language || navigator.languages?.[0] || 'en'
  // Map browser language codes to our locale keys
  const browserLocaleMap: Record<string, string> = {
    'ca': 'ca-ES',
    'cs': 'cs-CZ',
    'da': 'da-DK',
    'de': 'de-DE',
    'el': 'el-GR',
    'en': 'en-US',
    'en-GB': 'en-GB',
    'en-US': 'en-US',
    'es': 'es-ES',
    'fr': 'fr-FR',
    'ga': 'ga-IE',
    'hr': 'hr-HR',
    'hu': 'hu-HU',
    'it': 'it-IT',
    'ja': 'ja-JP',
    'ko': 'ko-KR',
    'ml': 'ml-IN',
    'nb': 'nb-NO',
    'nl': 'nl-NL',
    'no': 'nb-NO',
    'pl': 'pl-PL',
    'pt': 'pt-BR',
    'pt-BR': 'pt-BR',
    'pt-PT': 'pt-PT',
    'ro': 'ro-RO',
    'ru': 'ru-RU',
    'sk': 'sk-SK',
    'sv': 'sv-SE',
    'zh': 'zh-CN',
    'zh-CN': 'zh-CN',
    'zh-TW': 'zh-TW',
    'zh-HK': 'zh-TW',
  }
  // Try exact match first
  const exactMatch = browserLocaleMap[browserLang]
  if (exactMatch) {
    return exactMatch
  }
  // Try language code only
  const langCode = browserLang.split('-')[0]
  if (langCode) {
    const langMatch = browserLocaleMap[langCode]
    if (langMatch) {
      return langMatch
    }
  }
  return 'en-US'
}

// Handle language change in wizard
async function handleLanguageChange(langCode: string) {
  config.value.language = langCode
  delete errors.value.language
  await setLocale(langCode as LocaleKey)
}

// Detect browser timezone
const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

const config = ref<SetupConfig>({
  language: detectBrowserLanguage(),
  timezone: detectedTimezone,
  admin_username: '',
  admin_password: '',
  admin_password_confirm: '',
  enable_mfa: false,
  enable_home_assistant: false,
  home_assistant_url: '',
  home_assistant_token: '',
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
  t('setup.securitySettings'),
  t('setup.integrations'),
])

const stepDescriptions = computed(() => [
  t('setup.basicSettingsDesc'),
  t('setup.securitySettingsDesc'),
  t('setup.integrationsDesc'),
])

const timezones = computed(() => {
  const allTimezones = (Intl as unknown as { supportedValuesOf: (key: string) => string[] }).supportedValuesOf('timeZone')
  // Put detected timezone at the top of the list
  const filtered = allTimezones.filter((tz: string) => tz !== detectedTimezone)
  return [detectedTimezone, ...filtered]
})

onMounted(async () => {
  // Check if setup is already complete, redirect to login or home
  try {
    const response = await fetch('/api/setup/status')
    if (response.ok) {
      const data = await response.json()
      if (data.completed) {
        // Setup already done, check if user is logged in
        const token = localStorage.getItem('token')
        if (token) {
          router.push('/')
        } else {
          router.push('/login')
        }
        return
      }
    }
  } catch (error) {
    console.error('Failed to check setup status:', error)
  }

  await loadDefaults()
})

async function loadDefaults() {
  try {
    const response = await fetch('/api/setup/defaults')
    const data = await response.json()

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
      if (!config.value.admin_username) {
        errors.value.admin_username = t('setup.validation.usernameRequired')
      } else if (config.value.admin_username.length < 3) {
        errors.value.admin_username = t('setup.validation.usernameMinLength')
      } else if (usernameAvailable.value === false) {
        errors.value.admin_username = t('setup.validation.usernameExists')
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

    case 3:
      if (config.value.enable_home_assistant) {
        if (!config.value.home_assistant_url) {
          errors.value.home_assistant_url = t('setup.validation.homeAssistantUrlRequired')
        }
        if (!config.value.home_assistant_token) {
          errors.value.home_assistant_token = t('setup.validation.accessTokenRequired')
        }
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

  if (type === 'homeassistant') {
    testConfig = {
      url: config.value.home_assistant_url,
      token: config.value.home_assistant_token,
    }
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

// Check username availability with debounce
async function checkUsernameAvailability(username: string) {
  // Clear previous timeout
  if (usernameCheckTimeout) {
    clearTimeout(usernameCheckTimeout)
  }

  // Reset state
  usernameAvailable.value = null

  // Don't check if username is too short
  if (!username || username.length < 3) {
    return
  }

  // Debounce the check
  usernameCheckTimeout = setTimeout(async () => {
    checkingUsername.value = true
    try {
      const response = await fetch('/api/setup/check-username', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username }),
      })
      const result = await response.json()
      usernameAvailable.value = result.available
      if (!result.available) {
        errors.value.admin_username = result.message || t('setup.validation.usernameExists')
      } else {
        // Clear username error if it was about availability
        if (errors.value.admin_username === t('setup.validation.usernameExists')) {
          delete errors.value.admin_username
        }
      }
    } catch (error) {
      console.error('Failed to check username:', error)
    } finally {
      checkingUsername.value = false
    }
  }, 500)
}

// Handle username input change
function onUsernameInput(event: Event) {
  const target = event.target as HTMLInputElement
  config.value.admin_username = target.value
  // Clear error on input (except for username exists which is handled by checkUsernameAvailability)
  if (errors.value.admin_username && errors.value.admin_username !== t('setup.validation.usernameExists')) {
    delete errors.value.admin_username
  }
  checkUsernameAvailability(target.value)
}

// Clear error when user starts typing
function clearErrorOnInput(field: string) {
  if (errors.value[field]) {
    delete errors.value[field]
  }
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
              :value="config.language"
              class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="handleLanguageChange(($event.target as HTMLSelectElement).value)"
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
              @change="clearErrorOnInput('timezone')"
            >
              <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
            </select>
            <p v-if="errors.timezone" class="text-red-500 dark:text-red-400 text-sm mt-1">{{ errors.timezone }}</p>
          </div>
        </div>

        <!-- Step 2: Security Settings -->
        <div v-if="currentStep === 2" class="space-y-6">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('setup.adminUsername') }}</label>
            <div class="relative">
              <input
                :value="config.admin_username"
                type="text"
                placeholder="admin"
                autocomplete="username"
                class="w-full bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg px-4 py-3 pr-10 focus:outline-none focus:ring-2 focus:ring-blue-500"
                @input="onUsernameInput"
              />
              <!-- Username check status indicator -->
              <div class="absolute right-3 top-1/2 -translate-y-1/2">
                <svg v-if="checkingUsername" class="animate-spin h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <svg v-else-if="usernameAvailable === true && config.admin_username.length >= 3" class="h-5 w-5 text-green-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                </svg>
                <svg v-else-if="usernameAvailable === false" class="h-5 w-5 text-red-500" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
                </svg>
              </div>
            </div>
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
              @input="clearErrorOnInput('admin_password')"
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
                    {{ t('setup.passwordCheck.special') }} (!@#$%...)
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
              @input="clearErrorOnInput('admin_password_confirm')"
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

        <!-- Step 3: Integrations -->
        <div v-if="currentStep === 3" class="space-y-6">
          <div class="p-4 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <div class="flex items-center gap-3 mb-4">
              <input
                id="enable_ha"
                v-model="config.enable_home_assistant"
                type="checkbox"
                class="w-5 h-5 rounded bg-gray-200 dark:bg-gray-700 border-gray-300 dark:border-gray-600 text-blue-500 focus:ring-blue-500"
              />
              <img src="/icons/channels/homeassistant.svg" alt="Home Assistant" class="w-6 h-6" />
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
                  @input="clearErrorOnInput('home_assistant_url')"
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
                  @input="clearErrorOnInput('home_assistant_token')"
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

        <!-- Connection Test Result -->
        <div v-if="connectionTestResult" class="mt-4">
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
    </div>
  </div>
</template>
