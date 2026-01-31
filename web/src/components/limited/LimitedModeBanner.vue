<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { featuresApi, type FeatureStatus } from '@/api/setup'

const { t } = useI18n()

const props = defineProps<{
  dismissible?: boolean
}>()

const emit = defineEmits<{
  (e: 'download-cli'): void
  (e: 'dismiss'): void
}>()

const loading = ref(true)
const featureStatus = ref<FeatureStatus | null>(null)
const dismissed = ref(false)

async function loadFeatureStatus() {
  loading.value = true
  try {
    const response = await featuresApi.getStatus()
    featureStatus.value = response.data
  } catch (e) {
    console.error('Failed to load feature status:', e)
  } finally {
    loading.value = false
  }
}

function handleDownloadCLI() {
  emit('download-cli')
}

function handleDismiss() {
  dismissed.value = true
  emit('dismiss')
}

onMounted(() => {
  loadFeatureStatus()
})
</script>

<template>
  <div
    v-if="!dismissed && featureStatus && !featureStatus.cli_installed && featureStatus.disabled_count > 0"
    class="limited-mode-banner bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4"
  >
    <div class="flex items-start gap-3">
      <!-- Warning Icon -->
      <div class="flex-shrink-0 text-yellow-600 dark:text-yellow-400">
        <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
      </div>

      <!-- Content -->
      <div class="flex-1">
        <h3 class="font-medium text-yellow-800 dark:text-yellow-200">
          {{ t('limitedMode.title') }}
        </h3>
        <p class="mt-1 text-sm text-yellow-700 dark:text-yellow-300">
          {{ t('limitedMode.description', { count: featureStatus.disabled_count }) }}
        </p>

        <!-- Disabled Features List -->
        <div v-if="Object.keys(featureStatus.disabled_reasons).length > 0" class="mt-2">
          <p class="text-sm text-yellow-600 dark:text-yellow-400 mb-1">
            {{ t('limitedMode.disabledFeatures') }}:
          </p>
          <ul class="text-sm text-yellow-700 dark:text-yellow-300 list-disc list-inside">
            <li v-for="(_reason, feature) in featureStatus.disabled_reasons" :key="feature">
              {{ feature }}
            </li>
          </ul>
        </div>

        <!-- Actions -->
        <div class="mt-3 flex items-center gap-3">
          <button
            class="px-3 py-1.5 bg-yellow-600 hover:bg-yellow-700 text-white text-sm rounded-lg transition-colors"
            @click="handleDownloadCLI"
          >
            {{ t('limitedMode.downloadCLI') }}
          </button>
          <button
            v-if="props.dismissible"
            class="text-sm text-yellow-600 dark:text-yellow-400 hover:underline"
            @click="handleDismiss"
          >
            {{ t('common.dismiss') }}
          </button>
        </div>
      </div>

      <!-- Close Button -->
      <button
        v-if="props.dismissible"
        class="flex-shrink-0 text-yellow-500 hover:text-yellow-600 dark:text-yellow-400 dark:hover:text-yellow-300"
        @click="handleDismiss"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
    </div>
  </div>
</template>
