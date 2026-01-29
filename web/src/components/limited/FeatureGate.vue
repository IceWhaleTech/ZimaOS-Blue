<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { featuresApi, type FeatureInfo } from '@/api/setup'

const { t } = useI18n()

const props = defineProps<{
  feature: string
  fallback?: 'hide' | 'disable' | 'placeholder'
}>()

const loading = ref(true)
const featureInfo = ref<FeatureInfo | null>(null)
const error = ref<string | null>(null)

const isEnabled = computed(() => {
  return featureInfo.value?.enabled ?? false
})

const showContent = computed(() => {
  if (loading.value) return false
  if (isEnabled.value) return true
  return props.fallback === 'disable'
})

const showPlaceholder = computed(() => {
  if (loading.value) return false
  if (isEnabled.value) return false
  return props.fallback === 'placeholder'
})

async function loadFeatureInfo() {
  loading.value = true
  error.value = null
  try {
    const response = await featuresApi.getFeature(props.feature)
    featureInfo.value = response.data
  } catch (e) {
    error.value = t('features.loadError')
    console.error('Failed to load feature info:', e)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadFeatureInfo()
})
</script>

<template>
  <div class="feature-gate">
    <!-- Loading -->
    <div v-if="loading" class="animate-pulse">
      <slot name="loading">
        <div class="h-8 bg-gray-200 dark:bg-gray-700 rounded" />
      </slot>
    </div>

    <!-- Feature Enabled - Show Content -->
    <div v-else-if="showContent" :class="{ 'opacity-50 pointer-events-none': !isEnabled }">
      <slot />
    </div>

    <!-- Feature Disabled - Show Placeholder -->
    <div v-else-if="showPlaceholder">
      <slot name="placeholder">
        <div class="p-4 bg-gray-100 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700">
          <div class="flex items-center gap-3 text-gray-500 dark:text-gray-400">
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
            <div>
              <p class="font-medium">{{ featureInfo?.name || props.feature }}</p>
              <p v-if="featureInfo?.requires_cli" class="text-sm">
                {{ t('features.requiresCLI') }}
              </p>
            </div>
          </div>
        </div>
      </slot>
    </div>

    <!-- Feature Disabled - Hide (default) -->
    <!-- Nothing rendered -->
  </div>
</template>
