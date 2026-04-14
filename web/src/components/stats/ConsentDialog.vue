<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { statisticsApi } from '@/api/setup'

const { t } = useI18n()

const props = defineProps<{
  open?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'consent', value: boolean): void
}>()

const loading = ref(true)
const saving = ref(false)
const consentInfo = ref<{
  what_we_collect: string[]
  what_we_dont_collect: string[]
  how_we_use: string[]
  data_retention: string
} | null>(null)

async function loadConsentInfo() {
  loading.value = true
  try {
    const response = await statisticsApi.getConsentInfo()
    consentInfo.value = response.data
  } catch (e) {
    console.error('Failed to load consent info:', e)
  } finally {
    loading.value = false
  }
}

async function handleConsent(consented: boolean) {
  saving.value = true
  try {
    await statisticsApi.setConsentStatus(consented)
    emit('consent', consented)
    emit('close')
  } catch (e) {
    console.error('Failed to set consent:', e)
  } finally {
    saving.value = false
  }
}

function handleClose() {
  emit('close')
}

onMounted(() => {
  loadConsentInfo()
})
</script>

<template>
  <div
    v-if="props.open"
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50"
    @click.self="handleClose"
  >
    <div
      class="bg-white dark:bg-gray-700 rounded-2xl shadow-xl max-w-lg w-full max-h-[90vh] overflow-y-auto"
    >
      <!-- Header -->
      <div class="p-6 border-b border-gray-200 dark:border-gray-700">
        <h2 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('stats.consent.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('stats.consent.subtitle') }}
        </p>
      </div>

      <!-- Content -->
      <div class="p-6 space-y-6">
        <!-- Loading -->
        <div
          v-if="loading"
          class="flex items-center justify-center py-8"
        >
          <div
            class="animate-spin rounded-full h-8 w-8 border-2 border-gray-300 border-t-gray-900 dark:border-t-gray-400"
          />
        </div>

        <template v-else-if="consentInfo">
          <!-- What We Collect -->
          <div>
            <h3 class="font-medium text-gray-900 dark:text-white mb-2">
              {{ t('stats.consent.whatWeCollect') }}
            </h3>
            <ul class="space-y-1">
              <li
                v-for="item in consentInfo.what_we_collect"
                :key="item"
                class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300"
              >
                <svg
                  class="h-5 w-5 text-green-500 flex-shrink-0"
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
                <span>{{ item }}</span>
              </li>
            </ul>
          </div>

          <!-- What We Don't Collect -->
          <div>
            <h3 class="font-medium text-gray-900 dark:text-white mb-2">
              {{ t('stats.consent.whatWeDontCollect') }}
            </h3>
            <ul class="space-y-1">
              <li
                v-for="item in consentInfo.what_we_dont_collect"
                :key="item"
                class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300"
              >
                <svg
                  class="h-5 w-5 text-red-500 flex-shrink-0"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
                <span>{{ item }}</span>
              </li>
            </ul>
          </div>

          <!-- How We Use -->
          <div>
            <h3 class="font-medium text-gray-900 dark:text-white mb-2">
              {{ t('stats.consent.howWeUse') }}
            </h3>
            <ul class="space-y-1">
              <li
                v-for="item in consentInfo.how_we_use"
                :key="item"
                class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300"
              >
                <svg
                  class="h-5 w-5 text-gray-900 dark:text-white flex-shrink-0"
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
                <span>{{ item }}</span>
              </li>
            </ul>
          </div>

          <!-- Data Retention -->
          <div class="p-3 bg-gray-100 dark:bg-gray-700/50 rounded-lg">
            <p class="text-sm text-gray-600 dark:text-gray-300">
              <span class="font-medium">{{ t('stats.consent.dataRetention') }}:</span>
              {{ consentInfo.data_retention }}
            </p>
          </div>
        </template>
      </div>

      <!-- Footer -->
      <div
        class="p-6 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end gap-3"
      >
        <button
          :disabled="saving"
          class="px-4 py-2 text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          @click="handleConsent(false)"
        >
          {{ t('stats.consent.decline') }}
        </button>
        <button
          :disabled="saving"
          class="px-4 py-2 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white rounded-lg transition-colors disabled:opacity-50"
          @click="handleConsent(true)"
        >
          {{ saving ? t('common.saving') : t('stats.consent.accept') }}
        </button>
      </div>
    </div>
  </div>
</template>
