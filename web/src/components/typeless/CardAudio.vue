<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import type { TypelessCardAudio } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardAudio
}>()

const audioRef = ref<HTMLAudioElement | null>(null)
const isPlaying = ref(false)
const currentTime = ref(0)
const duration = ref(props.card.duration || 0)

function togglePlay() {
  if (!audioRef.value) return

  if (isPlaying.value) {
    audioRef.value.pause()
  } else {
    audioRef.value.play()
  }
  isPlaying.value = !isPlaying.value
}

function handleTimeUpdate() {
  if (audioRef.value) {
    currentTime.value = audioRef.value.currentTime
  }
}

function handleLoadedMetadata() {
  if (audioRef.value) {
    duration.value = audioRef.value.duration
  }
}

function handleEnded() {
  isPlaying.value = false
  currentTime.value = 0
}

function handleSeek(event: MouseEvent) {
  if (!audioRef.value) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const percent = (event.clientX - rect.left) / rect.width
  audioRef.value.currentTime = percent * duration.value
}

function formatTime(seconds: number): string {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

const progress = computed(() => {
  if (duration.value === 0) return 0
  return (currentTime.value / duration.value) * 100
})

onUnmounted(() => {
  if (audioRef.value) {
    audioRef.value.pause()
  }
})
</script>

<template>
  <div class="audio-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <div class="p-4 flex items-center gap-4">
      <!-- Cover image or placeholder -->
      <div class="flex-shrink-0 w-16 h-16 rounded-lg overflow-hidden bg-gradient-to-br from-purple-500 to-pink-500">
        <img v-if="card.coverImage" :src="card.coverImage" :alt="card.title" class="w-full h-full object-cover" />
        <div v-else class="w-full h-full flex items-center justify-center text-white text-2xl">
          🎵
        </div>
      </div>

      <!-- Info and controls -->
      <div class="flex-1 min-w-0">
        <!-- Title and artist -->
        <div class="mb-2">
          <h4 v-if="card.title" class="font-medium text-gray-900 dark:text-white truncate">{{ card.title }}</h4>
          <p v-if="card.artist" class="text-sm text-gray-500 dark:text-gray-400 truncate">
            {{ card.artist }}
            <span v-if="card.album"> · {{ card.album }}</span>
          </p>
        </div>

        <!-- Progress bar -->
        <div
          class="h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full cursor-pointer group"
          @click="handleSeek"
        >
          <div
            class="h-full bg-purple-500 rounded-full relative transition-all"
            :style="{ width: `${progress}%` }"
          >
            <div class="absolute right-0 top-1/2 -translate-y-1/2 w-3 h-3 bg-purple-500 rounded-full opacity-0 group-hover:opacity-100 transition-opacity" />
          </div>
        </div>

        <!-- Time display -->
        <div class="flex justify-between mt-1 text-xs text-gray-500 dark:text-gray-400">
          <span>{{ formatTime(currentTime) }}</span>
          <span>{{ formatTime(duration) }}</span>
        </div>
      </div>

      <!-- Play button -->
      <button
        class="flex-shrink-0 w-12 h-12 rounded-full bg-purple-500 hover:bg-purple-600 text-white flex items-center justify-center transition-colors"
        @click="togglePlay"
      >
        <svg v-if="!isPlaying" xmlns="http://www.w3.org/2000/svg" class="h-6 w-6 ml-1" fill="currentColor" viewBox="0 0 24 24">
          <path d="M8 5v14l11-7z" />
        </svg>
        <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="currentColor" viewBox="0 0 24 24">
          <path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" />
        </svg>
      </button>
    </div>

    <!-- Hidden audio element -->
    <audio
      ref="audioRef"
      :src="card.src"
      @timeupdate="handleTimeUpdate"
      @loadedmetadata="handleLoadedMetadata"
      @ended="handleEnded"
    />
  </div>
</template>
