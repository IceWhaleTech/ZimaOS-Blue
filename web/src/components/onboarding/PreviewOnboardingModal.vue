<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewApi } from '@/api/preview'

const { t } = useI18n()

const emit = defineEmits<{
  close: []
}>()

const visible = ref(false)
const tooltipStyle = ref({
  top: '60px',
  right: '16px',
})

function positionTooltip() {
  // Find the PreviewBanner button in the header
  const previewButton = document.querySelector('[data-preview-banner]')
  if (previewButton) {
    const rect = previewButton.getBoundingClientRect()
    tooltipStyle.value = {
      top: `${rect.bottom + 12}px`,
      right: `${window.innerWidth - rect.right}px`,
    }
  }
}

onMounted(async () => {
  try {
    // Check onboarding status from server
    const response = await previewApi.getOnboardingStatus()
    if (!response.data.seen) {
      visible.value = true
      await nextTick()
      positionTooltip()
      window.addEventListener('resize', positionTooltip)
    }
  } catch {
    // If API fails, show the modal (fail-open for better UX)
    visible.value = true
    await nextTick()
    positionTooltip()
    window.addEventListener('resize', positionTooltip)
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', positionTooltip)
})

async function handleClose() {
  try {
    // Mark onboarding as seen on server
    await previewApi.setOnboardingSeen()
  } catch {
    // Ignore errors - the modal will close anyway
  }
  visible.value = false
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-50">
      <!-- Semi-transparent backdrop -->
      <div
        class="absolute inset-0 bg-black/40"
        @click="handleClose"
      />

      <!-- Tooltip with arrow pointing up -->
      <div
        class="absolute w-80 bg-white dark:bg-gray-800 rounded-xl shadow-2xl overflow-hidden"
        :style="tooltipStyle"
      >
        <!-- Arrow pointing up -->
        <div class="absolute -top-2 right-6 w-4 h-4 bg-white dark:bg-gray-800 transform rotate-45" />

        <!-- Content -->
        <div class="relative p-4">
          <!-- Header -->
          <div class="flex items-center gap-2 mb-3">
            <span class="text-xl">👋</span>
            <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('onboarding.welcome') }}</h3>
          </div>

          <!-- Preview mode info -->
          <div class="flex items-start gap-3 mb-3 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
            </svg>
            <div>
              <p class="text-sm font-medium text-amber-800 dark:text-amber-300">{{ t('onboarding.previewMode') }}</p>
              <p class="text-xs text-amber-700 dark:text-amber-400 mt-0.5">{{ t('onboarding.previewModeDesc') }}</p>
            </div>
          </div>

          <!-- Hint with arrow pointing up -->
          <div class="flex items-center gap-2 mb-4 text-sm text-gray-600 dark:text-gray-400">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 animate-bounce" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18" />
            </svg>
            <span>{{ t('onboarding.createAccountHintDesc') }}</span>
          </div>

          <!-- Got it button -->
          <button
            class="w-full py-2.5 px-4 bg-accent hover:bg-accent/90 text-white font-medium rounded-lg transition-colors"
            @click="handleClose"
          >
            {{ t('onboarding.gotIt') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
