<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { providerDetectionApi, type ProviderDetectionResult } from '@/api/setup'

const { t } = useI18n()

const emit = defineEmits<{
  (e: 'detection-complete', result: ProviderDetectionResult): void
}>()

const loading = ref(true)
const result = ref<ProviderDetectionResult | null>(null)
const error = ref<string | null>(null)

async function detectProviders() {
  loading.value = true
  error.value = null
  try {
    const response = await providerDetectionApi.detectAll()
    result.value = response.data
    emit('detection-complete', response.data)
  } catch (e) {
    error.value = t('setup.providerDetection.error')
    console.error('Failed to detect providers:', e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  detectProviders()
})
</script>

<template>
  <div class="provider-detection">
    <!-- Loading State -->
    <div v-if="loading" class="text-center py-8">
      <div class="animate-spin rounded-full h-12 w-12 border-2 border-gray-300 border-t-blue-500 mx-auto mb-4" />
      <p class="text-gray-600 dark:text-gray-400">{{ t('setup.providerDetection.detecting') }}</p>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="text-center py-8">
      <svg class="h-12 w-12 text-red-500 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <p class="text-red-600 dark:text-red-400 mb-4">{{ error }}</p>
      <button
        class="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg"
        @click="detectProviders"
      >
        {{ t('common.retry') }}
      </button>
    </div>

    <!-- Results -->
    <div v-else-if="result" class="space-y-4">
      <!-- Ollama -->
      <div
        :class="[
          'p-4 rounded-lg border',
          result.ollama.available
            ? 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20'
            : 'border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50'
        ]"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center',
              result.ollama.available
                ? 'bg-green-100 dark:bg-green-800/50 text-green-600 dark:text-green-400'
                : 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400'
            ]"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path v-if="result.ollama.available" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </div>
          <div class="flex-1">
            <h4 class="font-medium text-gray-900 dark:text-white">Ollama</h4>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              <template v-if="result.ollama.available">
                {{ t('setup.providerDetection.ollamaDetected', { endpoint: result.ollama.endpoint }) }}
              </template>
              <template v-else>
                {{ t('setup.providerDetection.ollamaNotDetected') }}
              </template>
            </p>
            <p v-if="result.ollama.models && result.ollama.models.length > 0" class="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {{ t('setup.providerDetection.models') }}: {{ result.ollama.models.join(', ') }}
            </p>
          </div>
        </div>
      </div>

      <!-- Anthropic -->
      <div
        :class="[
          'p-4 rounded-lg border',
          result.anthropic.configured
            ? 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20'
            : 'border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50'
        ]"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center',
              result.anthropic.configured
                ? 'bg-green-100 dark:bg-green-800/50 text-green-600 dark:text-green-400'
                : 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400'
            ]"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path v-if="result.anthropic.configured" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <div class="flex-1">
            <h4 class="font-medium text-gray-900 dark:text-white">Anthropic (Claude)</h4>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              <template v-if="result.anthropic.configured">
                {{ t('setup.providerDetection.apiKeyConfigured') }}
                <span v-if="result.anthropic.masked_key" class="font-mono">{{ result.anthropic.masked_key }}</span>
              </template>
              <template v-else>
                {{ t('setup.providerDetection.apiKeyNotConfigured') }}
              </template>
            </p>
          </div>
        </div>
      </div>

      <!-- OpenAI -->
      <div
        :class="[
          'p-4 rounded-lg border',
          result.openai.configured
            ? 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/20'
            : 'border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50'
        ]"
      >
        <div class="flex items-center gap-3">
          <div
            :class="[
              'flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center',
              result.openai.configured
                ? 'bg-green-100 dark:bg-green-800/50 text-green-600 dark:text-green-400'
                : 'bg-gray-200 dark:bg-gray-700 text-gray-500 dark:text-gray-400'
            ]"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path v-if="result.openai.configured" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <div class="flex-1">
            <h4 class="font-medium text-gray-900 dark:text-white">OpenAI</h4>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              <template v-if="result.openai.configured">
                {{ t('setup.providerDetection.apiKeyConfigured') }}
                <span v-if="result.openai.masked_key" class="font-mono">{{ result.openai.masked_key }}</span>
              </template>
              <template v-else>
                {{ t('setup.providerDetection.apiKeyNotConfigured') }}
              </template>
            </p>
          </div>
        </div>
      </div>

      <!-- cc-switch -->
      <div
        v-if="result.cc_switch.available"
        class="p-4 rounded-lg border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-900/20"
      >
        <div class="flex items-center gap-3">
          <div class="flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center bg-blue-100 dark:bg-blue-800/50 text-blue-600 dark:text-blue-400">
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
            </svg>
          </div>
          <div class="flex-1">
            <h4 class="font-medium text-gray-900 dark:text-white">cc-switch</h4>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('setup.providerDetection.ccSwitchDetected') }}
              <span v-if="result.cc_switch.active_profile" class="font-medium">
                ({{ result.cc_switch.active_profile }})
              </span>
            </p>
            <p v-if="result.cc_switch.profiles && result.cc_switch.profiles.length > 0" class="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {{ t('setup.providerDetection.profiles') }}: {{ result.cc_switch.profiles.join(', ') }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
