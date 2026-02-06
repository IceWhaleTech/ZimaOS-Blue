<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Alert } from '@/api/companion'

const { t } = useI18n()

const props = defineProps<{
  alert: Alert
  duration?: number
  enableSound?: boolean
}>()

const emit = defineEmits<{
  close: []
  acknowledge: [id: string]
  viewSession: [sessionId: string]
}>()

const visible = ref(false)
const progress = ref(100)
let timer: ReturnType<typeof setInterval> | null = null
let closeTimer: ReturnType<typeof setTimeout> | null = null

const severityColors: Record<string, string> = {
  critical: 'border-l-red-500 bg-red-50 dark:bg-red-900/30',
  high: 'border-l-orange-500 bg-orange-50 dark:bg-orange-900/30',
  warning: 'border-l-yellow-500 bg-yellow-50 dark:bg-yellow-900/30',
  info: 'border-l-gray-900 dark:border-l-gray-400 bg-gray-700 dark:bg-gray-700 dark:bg-gray-700 dark:bg-gray-700/30',
}

const severityIcons: Record<string, string> = {
  critical: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
  high: 'M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
  warning: 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z',
  info: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
}

function startTimer() {
  const duration = props.duration || 5000
  const interval = 50
  const decrement = (100 / duration) * interval

  timer = setInterval(() => {
    progress.value -= decrement
    if (progress.value <= 0) {
      close()
    }
  }, interval)

  closeTimer = setTimeout(() => {
    close()
  }, duration)
}

function stopTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  if (closeTimer) {
    clearTimeout(closeTimer)
    closeTimer = null
  }
}

function close() {
  stopTimer()
  visible.value = false
  setTimeout(() => {
    emit('close')
  }, 300)
}

function playSound() {
  if (!props.enableSound) return

  try {
    const audio = new Audio('/sounds/alert.mp3')
    audio.volume = 0.5
    audio.play().catch(() => {
      // Ignore audio play errors (e.g., user hasn't interacted with page)
    })
  } catch {
    // Ignore audio errors
  }
}

function requestDesktopNotification() {
  if (!('Notification' in window)) return

  if (Notification.permission === 'granted') {
    new Notification(props.alert.title, {
      body: props.alert.description,
      icon: '/favicon.ico',
      tag: props.alert.id,
    })
  } else if (Notification.permission !== 'denied') {
    Notification.requestPermission()
  }
}

watch(() => props.alert, () => {
  visible.value = true
  progress.value = 100
  stopTimer()
  startTimer()
  playSound()
  requestDesktopNotification()
}, { immediate: true })

onMounted(() => {
  visible.value = true
  startTimer()
  playSound()
  requestDesktopNotification()
})

onUnmounted(() => {
  stopTimer()
})
</script>

<template>
  <Transition
    enter-active-class="transition-all duration-300 ease-out"
    enter-from-class="translate-x-full opacity-0"
    enter-to-class="translate-x-0 opacity-100"
    leave-active-class="transition-all duration-300 ease-in"
    leave-from-class="translate-x-0 opacity-100"
    leave-to-class="translate-x-full opacity-0"
  >
    <div
      v-if="visible"
      :class="[
        'fixed top-4 right-4 z-50 w-96 max-w-[calc(100vw-2rem)] rounded-lg shadow-lg border-l-4 overflow-hidden',
        severityColors[alert.severity] || severityColors.info
      ]"
      @mouseenter="stopTimer"
      @mouseleave="startTimer"
    >
      <!-- Progress bar -->
      <div class="absolute top-0 left-0 right-0 h-1 bg-gray-700 dark:bg-gray-700">
        <div
          class="h-full bg-gray-700 dark:bg-gray-700 transition-all duration-50"
          :style="{ width: `${progress}%` }"
        />
      </div>

      <div class="p-4 pt-5">
        <div class="flex items-start gap-3">
          <!-- Icon -->
          <div class="flex-shrink-0">
            <svg class="w-5 h-5 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="severityIcons[alert.severity] || severityIcons.info" />
            </svg>
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 mb-1">
              <span class="text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ alert.severity }}
              </span>
            </div>
            <h4 class="text-sm font-medium text-gray-900 dark:text-white truncate">
              {{ alert.title }}
            </h4>
            <p v-if="alert.description" class="mt-1 text-xs text-gray-600 dark:text-gray-400 line-clamp-2">
              {{ alert.description }}
            </p>

            <!-- Actions -->
            <div class="mt-3 flex items-center gap-2">
              <button
                class="px-2 py-1 text-xs bg-gray-700 dark:bg-gray-700 text-white rounded hover:bg-gray-700 dark:bg-gray-700/90 transition-colors"
                @click="emit('acknowledge', alert.id)"
              >
                {{ t('companion.acknowledge') }}
              </button>
              <button
                v-if="alert.session_id"
                class="px-2 py-1 text-xs text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white transition-colors"
                @click="emit('viewSession', alert.session_id)"
              >
                {{ t('companion.alerts.viewSession') }}
              </button>
            </div>
          </div>

          <!-- Close button -->
          <button
            class="flex-shrink-0 p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
            @click="close"
          >
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>
