<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{
  duration: number // Total duration in milliseconds
  currentTime: number // Current time in milliseconds
  isPlaying: boolean
  speed: number
  totalEvents: number
  currentEventIndex: number
}>()

const emit = defineEmits<{
  play: []
  pause: []
  seek: [time: number]
  speedChange: [speed: number]
  stepForward: []
  stepBackward: []
}>()

const speeds = [0.25, 0.5, 1, 1.5, 2, 4]
const showSpeedMenu = ref(false)

const progress = computed(() => {
  if (props.duration === 0) return 0
  return (props.currentTime / props.duration) * 100
})

const formattedCurrentTime = computed(() => formatTime(props.currentTime))
const formattedDuration = computed(() => formatTime(props.duration))

function formatTime(ms: number): string {
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
}

function handleSeek(event: MouseEvent) {
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const x = event.clientX - rect.left
  const percentage = x / rect.width
  const newTime = Math.floor(percentage * props.duration)
  emit('seek', Math.max(0, Math.min(newTime, props.duration)))
}

function handleSpeedChange(newSpeed: number) {
  emit('speedChange', newSpeed)
  showSpeedMenu.value = false
}

// Close speed menu when clicking outside
watch(showSpeedMenu, (isOpen) => {
  if (isOpen) {
    const closeMenu = (e: MouseEvent) => {
      const target = e.target as HTMLElement
      if (!target.closest('.speed-menu')) {
        showSpeedMenu.value = false
        document.removeEventListener('click', closeMenu)
      }
    }
    setTimeout(() => {
      document.addEventListener('click', closeMenu)
    }, 0)
  }
})
</script>

<template>
  <div class="bg-white dark:bg-gray-700 rounded-lg shadow-md p-4">
    <!-- Progress bar -->
    <div
      class="relative h-2 bg-gray-700 dark:bg-gray-500 rounded-full cursor-pointer mb-4 group"
      @click="handleSeek"
    >
      <div
        class="absolute h-full bg-gray-700 dark:bg-gray-500 rounded-full transition-all"
        :style="{ width: `${progress}%` }"
      />
      <div
        class="absolute w-4 h-4 bg-gray-700 dark:bg-gray-500 rounded-full -top-1 transform -translate-x-1/2 opacity-0 group-hover:opacity-100 transition-opacity shadow-md"
        :style="{ left: `${progress}%` }"
      />
    </div>

    <!-- Controls -->
    <div class="flex items-center justify-between">
      <!-- Left: Time display -->
      <div class="text-sm text-gray-600 dark:text-gray-400 font-mono min-w-[100px]">
        {{ formattedCurrentTime }} / {{ formattedDuration }}
      </div>

      <!-- Center: Playback controls -->
      <div class="flex items-center gap-2">
        <!-- Step backward -->
        <button
          class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          :disabled="currentEventIndex <= 0"
          :title="t('companion.replay.stepBackward')"
          @click="emit('stepBackward')"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12.066 11.2a1 1 0 000 1.6l5.334 4A1 1 0 0019 16V8a1 1 0 00-1.6-.8l-5.333 4zM4.066 11.2a1 1 0 000 1.6l5.334 4A1 1 0 0011 16V8a1 1 0 00-1.6-.8l-5.334 4z"
            />
          </svg>
        </button>

        <!-- Play/Pause -->
        <button
          class="p-3 bg-gray-700 dark:bg-gray-500 text-white rounded-full hover:bg-gray-800 dark:hover:bg-gray-400 transition-colors"
          @click="isPlaying ? emit('pause') : emit('play')"
        >
          <svg v-if="!isPlaying" class="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
            <path d="M8 5v14l11-7z" />
          </svg>
          <svg v-else class="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
            <path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" />
          </svg>
        </button>

        <!-- Step forward -->
        <button
          class="p-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          :disabled="currentEventIndex >= totalEvents - 1"
          :title="t('companion.replay.stepForward')"
          @click="emit('stepForward')"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M11.933 12.8a1 1 0 000-1.6L6.6 7.2A1 1 0 005 8v8a1 1 0 001.6.8l5.333-4zM19.933 12.8a1 1 0 000-1.6l-5.333-4A1 1 0 0013 8v8a1 1 0 001.6.8l5.333-4z"
            />
          </svg>
        </button>
      </div>

      <!-- Right: Speed and event info -->
      <div class="flex items-center gap-4 min-w-[150px] justify-end">
        <!-- Event counter -->
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ currentEventIndex + 1 }} / {{ totalEvents }}
        </div>

        <!-- Speed selector -->
        <div class="relative speed-menu">
          <button
            class="px-3 py-1.5 text-sm bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
            @click="showSpeedMenu = !showSpeedMenu"
          >
            {{ speed }}x
          </button>

          <Transition
            enter-active-class="transition-all duration-150 ease-out"
            enter-from-class="opacity-0 scale-95"
            enter-to-class="opacity-100 scale-100"
            leave-active-class="transition-all duration-100 ease-in"
            leave-from-class="opacity-100 scale-100"
            leave-to-class="opacity-0 scale-95"
          >
            <div
              v-if="showSpeedMenu"
              class="absolute bottom-full right-0 mb-2 bg-white dark:bg-gray-700 rounded-lg shadow-lg border border-gray-200 dark:border-gray-700 py-1 min-w-[80px]"
            >
              <button
                v-for="s in speeds"
                :key="s"
                :class="[
                  'w-full px-3 py-1.5 text-sm text-left hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors',
                  s === speed
                    ? 'text-gray-900 dark:text-gray-300 font-medium'
                    : 'text-gray-700 dark:text-gray-300',
                ]"
                @click="handleSpeedChange(s)"
              >
                {{ s }}x
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </div>
  </div>
</template>
