<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { previewApi } from '@/api/preview'

const { t } = useI18n()
const router = useRouter()
const DEFAULT_TOOLTIP_WIDTH = 320
const DEFAULT_TOOLTIP_HEIGHT = 252
const VIEWPORT_PADDING = 16
const TOOLTIP_GAP = 12
const ARROW_SIZE = 16
const ARROW_PADDING = 20
const ENHANCED_MODE_SETTINGS_ROUTE = '/settings?tab=llm#claude-code-settings'

const emit = defineEmits<{
  close: []
}>()

type OnboardingAnchorKind = 'enhanced-entry' | 'enhanced-menu' | 'fallback'

const anchorSelectors: Array<{ selector: string; kind: OnboardingAnchorKind }> = [
  { selector: '[data-onboarding-anchor="enhanced-mode-entry"]', kind: 'enhanced-entry' },
  { selector: '[data-onboarding-anchor="enhanced-mode-menu"]', kind: 'enhanced-menu' },
]

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
const activeAnchorKind = ref<OnboardingAnchorKind>('fallback')
const onboardingHintText = computed(() =>
  activeAnchorKind.value === 'enhanced-menu'
    ? t('onboarding.enhancedModeMenuHintDesc')
    : t('onboarding.enhancedModeHintDesc')
)
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

function getAnchorTarget(): { element: HTMLElement; kind: OnboardingAnchorKind } | null {
  for (const candidate of anchorSelectors) {
    const anchor = Array.from(document.querySelectorAll(candidate.selector)).find(isVisibleAnchor)
    if (anchor) {
      return {
        element: anchor,
        kind: candidate.kind,
      }
    }
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
  const anchorTarget = getAnchorTarget()

  if (!anchorTarget) {
    activeAnchorKind.value = 'fallback'
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

  activeAnchorKind.value = anchorTarget.kind
  const rect = anchorTarget.element.getBoundingClientRect()
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
    const response = await previewApi.getOnboardingStatus()
    if (!response.data.seen) {
      visible.value = true
      await nextTick()
      positionTooltip()
      startPositionTracking()
    }
  } catch {
    visible.value = true
    await nextTick()
    positionTooltip()
    startPositionTracking()
  }
})

onUnmounted(() => {
  stopPositionTracking()
})

async function markOnboardingSeen() {
  try {
    await previewApi.setOnboardingSeen()
  } catch {
    // Ignore errors - the modal will close anyway
  }
}

async function handleClose() {
  await markOnboardingSeen()
  stopPositionTracking()
  visible.value = false
  emit('close')
}

async function handleOpenEnhancedMode() {
  await markOnboardingSeen()
  stopPositionTracking()
  visible.value = false
  emit('close')
  await router.push(ENHANCED_MODE_SETTINGS_ROUTE)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-50">
      <div class="absolute inset-0 bg-black/40" @click="handleClose" />

      <div
        ref="tooltipRef"
        data-testid="preview-onboarding-tooltip"
        class="absolute w-80 bg-white dark:bg-gray-700 rounded-xl shadow-2xl overflow-hidden"
        :style="tooltipStyle"
      >
        <div
          data-testid="preview-onboarding-tooltip-arrow"
          class="absolute w-4 h-4 bg-white dark:bg-gray-700 transform rotate-45"
          :class="tooltipPlacement === 'above' ? '-bottom-2' : '-top-2'"
          :style="tooltipArrowStyle"
        />

        <div class="relative p-4">
          <div class="flex items-center gap-2 mb-3">
            <span class="text-xl">⚡</span>
            <h3 class="font-semibold text-gray-900 dark:text-white">
              {{ t('onboarding.welcome') }}
            </h3>
          </div>

          <div class="flex items-start gap-3 mb-3 p-3 bg-sky-50 dark:bg-sky-900/20 rounded-lg">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5 text-sky-600 dark:text-sky-400 flex-shrink-0 mt-0.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 3L4 14h6l-1 7 9-11h-6l1-7z"
              />
            </svg>
            <div>
              <p class="text-sm font-medium text-sky-800 dark:text-sky-300">
                {{ t('onboarding.enhancedMode') }}
              </p>
              <p class="text-xs text-sky-700 dark:text-sky-400 mt-0.5">
                {{ t('onboarding.enhancedModeDesc') }}
              </p>
            </div>
          </div>

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
            <span>{{ onboardingHintText }}</span>
          </div>

          <div class="flex items-center gap-2">
            <button
              data-testid="preview-onboarding-open-enhanced-mode"
              class="flex-1 py-2.5 px-4 bg-sky-600 hover:bg-sky-700 text-white font-medium rounded-lg transition-colors"
              @click="handleOpenEnhancedMode"
            >
              {{ t('onboarding.openEnhancedMode') }}
            </button>
            <button
              class="py-2.5 px-4 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 text-gray-700 dark:text-white font-medium rounded-lg transition-colors"
              @click="handleClose"
            >
              {{ t('onboarding.skipForNow') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
