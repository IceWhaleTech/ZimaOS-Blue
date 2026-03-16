<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { previewApi } from '@/api/preview'

const { t } = useI18n()
const DEFAULT_TOOLTIP_WIDTH = 320
const DEFAULT_TOOLTIP_HEIGHT = 252
const VIEWPORT_PADDING = 16
const TOOLTIP_GAP = 12
const ARROW_SIZE = 16
const ARROW_PADDING = 20

const emit = defineEmits<{
  close: []
}>()

const visible = ref(false)
const tooltipRef = ref<HTMLElement | null>(null)
const tooltipStyle = ref({
  top: '60px',
  left: '16px',
})
const tooltipArrowStyle = ref({
  left: '24px',
})
const tooltipPlacement = ref<'above' | 'below'>('below')
let trackingPosition = false

function clamp(value: number, min: number, max: number) {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

function isVisibleAnchor(element: Element): element is HTMLElement {
  if (!(element instanceof HTMLElement)) return false
  const rect = element.getBoundingClientRect()
  const style = window.getComputedStyle(element)
  return (
    rect.width > 0 &&
    rect.height > 0 &&
    rect.bottom > 0 &&
    rect.right > 0 &&
    rect.top < window.innerHeight &&
    rect.left < window.innerWidth &&
    style.display !== 'none' &&
    style.visibility !== 'hidden'
  )
}

function getAnchorElement(): HTMLElement | null {
  const selectors = [
    '[data-onboarding-anchor="preview-create-account"]',
    '[data-preview-banner] button',
    '[data-preview-banner]',
  ]

  for (const selector of selectors) {
    const anchor = Array.from(document.querySelectorAll(selector)).find(isVisibleAnchor)
    if (anchor) return anchor
  }

  return null
}

function positionTooltip() {
  const tooltipWidth = tooltipRef.value?.offsetWidth || DEFAULT_TOOLTIP_WIDTH
  const tooltipHeight = tooltipRef.value?.offsetHeight || DEFAULT_TOOLTIP_HEIGHT
  const maxLeft = Math.max(VIEWPORT_PADDING, window.innerWidth - tooltipWidth - VIEWPORT_PADDING)
  const fallbackLeft = clamp(
    window.innerWidth - tooltipWidth - VIEWPORT_PADDING,
    VIEWPORT_PADDING,
    maxLeft
  )
  const anchor = getAnchorElement()

  if (!anchor) {
    tooltipPlacement.value = 'below'
    tooltipStyle.value = {
      top: '60px',
      left: `${fallbackLeft}px`,
    }
    tooltipArrowStyle.value = {
      left: `${tooltipWidth - ARROW_PADDING - ARROW_SIZE / 2}px`,
    }
    return
  }

  const rect = anchor.getBoundingClientRect()
  const anchorCenterX = rect.left + rect.width / 2
  const left = clamp(anchorCenterX - tooltipWidth / 2, VIEWPORT_PADDING, maxLeft)
  const maxTop = Math.max(VIEWPORT_PADDING, window.innerHeight - tooltipHeight - VIEWPORT_PADDING)
  const spaceBelow = window.innerHeight - rect.bottom - TOOLTIP_GAP - VIEWPORT_PADDING
  const spaceAbove = rect.top - TOOLTIP_GAP - VIEWPORT_PADDING
  const shouldPlaceAbove = spaceBelow < tooltipHeight && spaceAbove > spaceBelow

  tooltipPlacement.value = shouldPlaceAbove ? 'above' : 'below'
  tooltipStyle.value = {
    top: `${
      shouldPlaceAbove
        ? clamp(rect.top - tooltipHeight - TOOLTIP_GAP, VIEWPORT_PADDING, maxTop)
        : clamp(rect.bottom + TOOLTIP_GAP, VIEWPORT_PADDING, maxTop)
    }px`,
    left: `${left}px`,
  }

  const arrowLeft = clamp(
    anchorCenterX - left - ARROW_SIZE / 2,
    ARROW_PADDING,
    tooltipWidth - ARROW_SIZE - ARROW_PADDING
  )
  tooltipArrowStyle.value = {
    left: `${arrowLeft}px`,
  }
}

function startPositionTracking() {
  if (trackingPosition) return
  window.addEventListener('resize', positionTooltip)
  window.addEventListener('scroll', positionTooltip, true)
  trackingPosition = true
}

function stopPositionTracking() {
  if (!trackingPosition) return
  window.removeEventListener('resize', positionTooltip)
  window.removeEventListener('scroll', positionTooltip, true)
  trackingPosition = false
}

onMounted(async () => {
  try {
    // Check onboarding status from server
    const response = await previewApi.getOnboardingStatus()
    if (!response.data.seen) {
      visible.value = true
      await nextTick()
      positionTooltip()
      startPositionTracking()
    }
  } catch {
    // If API fails, show the modal (fail-open for better UX)
    visible.value = true
    await nextTick()
    positionTooltip()
    startPositionTracking()
  }
})

onUnmounted(() => {
  stopPositionTracking()
})

async function handleClose() {
  try {
    // Mark onboarding as seen on server
    await previewApi.setOnboardingSeen()
  } catch {
    // Ignore errors - the modal will close anyway
  }
  stopPositionTracking()
  visible.value = false
  emit('close')
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-50">
      <!-- Semi-transparent backdrop -->
      <div class="absolute inset-0 bg-black/40" @click="handleClose" />

      <!-- Tooltip -->
      <div
        ref="tooltipRef"
        data-testid="preview-onboarding-tooltip"
        class="absolute w-80 bg-white dark:bg-gray-700 rounded-xl shadow-2xl overflow-hidden"
        :style="tooltipStyle"
      >
        <!-- Arrow -->
        <div
          data-testid="preview-onboarding-tooltip-arrow"
          class="absolute w-4 h-4 bg-white dark:bg-gray-700 transform rotate-45"
          :class="tooltipPlacement === 'above' ? '-bottom-2' : '-top-2'"
          :style="tooltipArrowStyle"
        />

        <!-- Content -->
        <div class="relative p-4">
          <!-- Header -->
          <div class="flex items-center gap-2 mb-3">
            <span class="text-xl">👋</span>
            <h3 class="font-semibold text-gray-900 dark:text-white">
              {{ t('onboarding.welcome') }}
            </h3>
          </div>

          <!-- Preview mode info -->
          <div class="flex items-start gap-3 mb-3 p-3 bg-amber-50 dark:bg-amber-900/20 rounded-lg">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
              />
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
              />
            </svg>
            <div>
              <p class="text-sm font-medium text-amber-800 dark:text-amber-300">
                {{ t('onboarding.previewMode') }}
              </p>
              <p class="text-xs text-amber-700 dark:text-amber-400 mt-0.5">
                {{ t('onboarding.previewModeDesc') }}
              </p>
            </div>
          </div>

          <!-- Hint with arrow pointing up -->
          <div class="flex items-center gap-2 mb-4 text-sm text-gray-600 dark:text-gray-400">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-4 w-4 animate-bounce"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M5 10l7-7m0 0l7 7m-7-7v18"
              />
            </svg>
            <span>{{ t('onboarding.createAccountHintDesc') }}</span>
          </div>

          <!-- Got it button -->
          <button
            class="w-full py-2.5 px-4 bg-gray-700 dark:bg-gray-500 hover:bg-gray-800 dark:hover:bg-gray-400 text-white font-medium rounded-lg transition-colors"
            @click="handleClose"
          >
            {{ t('onboarding.gotIt') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
