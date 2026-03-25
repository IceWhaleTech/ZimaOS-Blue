<script setup lang="ts">
import { ref, computed, onUnmounted, watch } from 'vue'
import type { TypelessCardVideo, VideoSubtitle } from '@/types/typeless'
import { createProtectedObjectUrl, isProtectedResourceUrl } from '@/utils/protectedResource'

const props = defineProps<{
  card: TypelessCardVideo
}>()

const videoRef = ref<HTMLVideoElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)
const isPlaying = ref(false)
const currentTime = ref(0)
const duration = ref(props.card.duration || 0)
const volume = ref(1)
const isMuted = ref(props.card.muted || false)
const isFullscreen = ref(false)
const showControls = ref(true)
const controlsTimeout = ref<number | null>(null)
const videoSrc = ref('')
const posterSrc = ref('')
const subtitleTracks = ref<VideoSubtitle[]>([])

let videoObjectUrl = ''
let posterObjectUrl = ''
let subtitleObjectUrls: string[] = []
let sourceLoadSeq = 0

function revokeResolvedSources() {
  if (videoObjectUrl) {
    URL.revokeObjectURL(videoObjectUrl)
    videoObjectUrl = ''
  }
  if (posterObjectUrl) {
    URL.revokeObjectURL(posterObjectUrl)
    posterObjectUrl = ''
  }
  if (subtitleObjectUrls.length > 0) {
    for (const objectUrl of subtitleObjectUrls) {
      URL.revokeObjectURL(objectUrl)
    }
    subtitleObjectUrls = []
  }
}

async function resolveMaybeProtectedUrl(rawValue?: string): Promise<string> {
  const trimmed = String(rawValue || '').trim()
  if (!trimmed) return ''
  if (!isProtectedResourceUrl(trimmed)) return trimmed
  return (await createProtectedObjectUrl(trimmed)) || ''
}

async function resolveVideoSources() {
  const seq = ++sourceLoadSeq
  revokeResolvedSources()
  videoSrc.value = ''
  posterSrc.value = ''
  subtitleTracks.value = []

  const resolvedVideo = await resolveMaybeProtectedUrl(props.card.src)
  if (seq !== sourceLoadSeq) {
    if (resolvedVideo && resolvedVideo !== props.card.src) {
      URL.revokeObjectURL(resolvedVideo)
    }
    return
  }
  if (!resolvedVideo) return
  if (resolvedVideo !== props.card.src) {
    videoObjectUrl = resolvedVideo
  }
  videoSrc.value = resolvedVideo

  const resolvedPoster = await resolveMaybeProtectedUrl(props.card.poster)
  if (seq !== sourceLoadSeq) {
    if (resolvedPoster && resolvedPoster !== props.card.poster) {
      URL.revokeObjectURL(resolvedPoster)
    }
    return
  }
  if (resolvedPoster && resolvedPoster !== props.card.poster) {
    posterObjectUrl = resolvedPoster
  }
  posterSrc.value = resolvedPoster

  const nextTracks: VideoSubtitle[] = []
  for (const subtitle of props.card.subtitles || []) {
    const resolvedSubtitleSrc = await resolveMaybeProtectedUrl(subtitle.src)
    if (seq !== sourceLoadSeq) {
      if (resolvedSubtitleSrc && resolvedSubtitleSrc !== subtitle.src) {
        URL.revokeObjectURL(resolvedSubtitleSrc)
      }
      return
    }
    if (!resolvedSubtitleSrc) continue
    if (resolvedSubtitleSrc !== subtitle.src) {
      subtitleObjectUrls.push(resolvedSubtitleSrc)
    }
    nextTracks.push({ ...subtitle, src: resolvedSubtitleSrc })
  }
  subtitleTracks.value = nextTracks
}

function togglePlay() {
  if (!videoRef.value) return

  if (isPlaying.value) {
    videoRef.value.pause()
  } else {
    videoRef.value.play()
  }
}

function handlePlay() {
  isPlaying.value = true
}

function handlePause() {
  isPlaying.value = false
}

function handleTimeUpdate() {
  if (videoRef.value) {
    currentTime.value = videoRef.value.currentTime
  }
}

function handleLoadedMetadata() {
  if (videoRef.value) {
    duration.value = videoRef.value.duration
  }
}

function handleEnded() {
  isPlaying.value = false
  if (!props.card.loop) {
    currentTime.value = 0
  }
}

function handleSeek(event: MouseEvent) {
  if (!videoRef.value) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const percent = (event.clientX - rect.left) / rect.width
  videoRef.value.currentTime = percent * duration.value
}

function handleVolumeChange(event: MouseEvent) {
  if (!videoRef.value) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const percent = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width))
  volume.value = percent
  videoRef.value.volume = percent
  isMuted.value = percent === 0
}

function toggleMute() {
  if (!videoRef.value) return
  isMuted.value = !isMuted.value
  videoRef.value.muted = isMuted.value
}

function toggleFullscreen() {
  if (!containerRef.value) return

  if (!isFullscreen.value) {
    if (containerRef.value.requestFullscreen) {
      containerRef.value.requestFullscreen()
    }
  } else {
    if (document.exitFullscreen) {
      document.exitFullscreen()
    }
  }
}

function handleFullscreenChange() {
  isFullscreen.value = !!document.fullscreenElement
}

function formatTime(seconds: number): string {
  const hrs = Math.floor(seconds / 3600)
  const mins = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  if (hrs > 0) {
    return `${hrs}:${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
  }
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

function handleMouseMove() {
  showControls.value = true
  if (controlsTimeout.value) {
    clearTimeout(controlsTimeout.value)
  }
  if (isPlaying.value) {
    controlsTimeout.value = window.setTimeout(() => {
      showControls.value = false
    }, 3000)
  }
}

function handleMouseLeave() {
  if (isPlaying.value) {
    controlsTimeout.value = window.setTimeout(() => {
      showControls.value = false
    }, 1000)
  }
}

const progress = computed(() => {
  if (duration.value === 0) return 0
  return (currentTime.value / duration.value) * 100
})

watch(
  () => [props.card.src, props.card.poster, JSON.stringify(props.card.subtitles || [])],
  () => {
    void resolveVideoSources()
  },
  { immediate: true }
)

// Listen for fullscreen changes
if (typeof document !== 'undefined') {
  document.addEventListener('fullscreenchange', handleFullscreenChange)
}

onUnmounted(() => {
  if (videoRef.value) {
    videoRef.value.pause()
  }
  if (controlsTimeout.value) {
    clearTimeout(controlsTimeout.value)
  }
  revokeResolvedSources()
  if (typeof document !== 'undefined') {
    document.removeEventListener('fullscreenchange', handleFullscreenChange)
  }
})
</script>

<template>
  <div
    ref="containerRef"
    class="video-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-black"
    :class="{ 'fixed inset-0 z-50 border-0 rounded-none': isFullscreen }"
    @mousemove="handleMouseMove"
    @mouseleave="handleMouseLeave"
  >
    <!-- Title (only when not fullscreen) -->
    <div
      v-if="card.title && !isFullscreen"
      class="px-4 py-3 bg-white dark:bg-gray-700 border-b border-gray-200 dark:border-gray-700"
    >
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Video container -->
    <div
      class="relative"
      :class="isFullscreen ? 'h-full flex items-center justify-center' : 'aspect-video'"
    >
      <video
        ref="videoRef"
        class="w-full h-full object-contain"
        :src="videoSrc"
        :poster="posterSrc"
        :autoplay="card.autoplay"
        :muted="isMuted"
        :loop="card.loop"
        playsinline
        @play="handlePlay"
        @pause="handlePause"
        @timeupdate="handleTimeUpdate"
        @loadedmetadata="handleLoadedMetadata"
        @ended="handleEnded"
        @click="togglePlay"
      >
        <!-- Subtitles -->
        <track
          v-for="(subtitle, index) in subtitleTracks"
          :key="index"
          kind="subtitles"
          :src="subtitle.src"
          :srclang="subtitle.srclang"
          :label="subtitle.label"
          :default="subtitle.default"
        />
      </video>

      <!-- Play button overlay (when paused) -->
      <div
        v-if="!isPlaying"
        class="absolute inset-0 flex items-center justify-center bg-black/30 cursor-pointer"
        @click="togglePlay"
      >
        <div class="w-16 h-16 rounded-full bg-white/90 flex items-center justify-center shadow-lg">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="video-play-icon h-8 w-8 text-gray-900"
            fill="currentColor"
            viewBox="0 0 24 24"
          >
            <path d="M8 5v14l11-7z" />
          </svg>
        </div>
      </div>

      <!-- Custom controls -->
      <div
        v-if="card.controls !== false"
        class="video-controls-overlay absolute bottom-0 bg-gradient-to-t from-black/80 to-transparent px-4 py-3 transition-opacity duration-300"
        :class="showControls || !isPlaying ? 'opacity-100' : 'opacity-0'"
      >
        <!-- Progress bar -->
        <div class="h-1 bg-white/30 rounded-full cursor-pointer group mb-3" @click="handleSeek">
          <div
            class="h-full bg-gray-700 dark:bg-gray-500 rounded-full relative transition-all"
            :style="{ width: `${progress}%` }"
          >
            <div
              class="video-progress-handle absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-gray-700 dark:bg-gray-500 rounded-full opacity-0 group-hover:opacity-100 transition-opacity"
            />
          </div>
        </div>

        <!-- Controls row -->
        <div class="flex items-center gap-3">
          <!-- Play/Pause button -->
          <button
            class="text-white hover:text-gray-900 dark:text-white transition-colors"
            @click.stop="togglePlay"
          >
            <svg
              v-if="!isPlaying"
              xmlns="http://www.w3.org/2000/svg"
              class="h-6 w-6"
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path d="M8 5v14l11-7z" />
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              class="h-6 w-6"
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" />
            </svg>
          </button>

          <!-- Time display -->
          <div class="text-white text-sm">
            {{ formatTime(currentTime) }} / {{ formatTime(duration) }}
          </div>

          <div class="flex-1" />

          <!-- Volume control -->
          <div class="flex items-center gap-2 group/volume">
            <button
              class="text-white hover:text-gray-900 dark:text-white transition-colors"
              @click.stop="toggleMute"
            >
              <svg
                v-if="isMuted || volume === 0"
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2"
                />
              </svg>
              <svg
                v-else-if="volume < 0.5"
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15.536 8.464a5 5 0 010 7.072M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                />
              </svg>
              <svg
                v-else
                xmlns="http://www.w3.org/2000/svg"
                class="h-5 w-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
                />
              </svg>
            </button>
            <div
              class="w-20 h-1 bg-white/30 rounded-full cursor-pointer hidden group-hover/volume:block"
              @click.stop="handleVolumeChange"
            >
              <div
                class="h-full bg-white rounded-full"
                :style="{ width: `${isMuted ? 0 : volume * 100}%` }"
              />
            </div>
          </div>

          <!-- Fullscreen button -->
          <button
            class="text-white hover:text-gray-900 dark:text-white transition-colors"
            @click.stop="toggleFullscreen"
          >
            <svg
              v-if="!isFullscreen"
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
              />
            </svg>
            <svg
              v-else
              xmlns="http://www.w3.org/2000/svg"
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 9V4.5M9 9H4.5M9 9L3.75 3.75M9 15v4.5M9 15H4.5M9 15l-5.25 5.25M15 9h4.5M15 9V4.5M15 9l5.25-5.25M15 15h4.5M15 15v4.5m0-4.5l5.25 5.25"
              />
            </svg>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.video-play-icon {
  margin-inline-start: 0.25rem;
}

.video-controls-overlay {
  inset-inline: 0;
}

.video-progress-handle {
  inset-inline-end: 0;
}
</style>
