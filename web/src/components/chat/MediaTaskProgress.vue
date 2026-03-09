<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { streamMediaTask, type MediaProgressEvent, type TaskStatus } from '@/api/media'

const { t } = useI18n()

const props = defineProps<{
  taskId: string
  type: 'image' | 'video'
  prompt?: string
}>()

const emit = defineEmits<{
  complete: [evt: MediaProgressEvent]
  error: [message: string]
}>()

const status = ref<TaskStatus>('pending')
const progress = ref(0)
const imageUrls = ref<string[]>([])
const errorMessage = ref('')
const abortController = ref<AbortController | null>(null)
const lightboxUrl = ref<string | null>(null)
const elapsed = ref(0)
let elapsedTimer: ReturnType<typeof setInterval> | null = null

const progressPercent = computed(() => Math.round(progress.value * 100))

const statusLabel = computed(() => {
  switch (status.value) {
    case 'pending': return t('media.status.pending')
    case 'processing': return t('media.status.processing')
    case 'succeeded': return t('media.status.succeeded')
    case 'cancelled': return t('media.cancelled')
    case 'failed': return t('media.status.failed')
    default: return ''
  }
})

const elapsedLabel = computed(() => {
  if (elapsed.value < 1) return ''
  return `${elapsed.value}s`
})

const isActive = computed(() => status.value === 'pending' || status.value === 'processing')

function openLightbox(url: string) {
  lightboxUrl.value = url
}

function closeLightbox() {
  lightboxUrl.value = null
}

onMounted(() => {
  // Elapsed timer
  elapsedTimer = setInterval(() => {
    if (isActive.value) elapsed.value++
  }, 1000)

  abortController.value = streamMediaTask(props.taskId, {
    onProgress(evt) {
      status.value = evt.status
      progress.value = evt.progress
    },
    onComplete(evt) {
      status.value = 'succeeded'
      progress.value = 1
      if (evt.response?.data) {
        imageUrls.value = evt.response.data.map(d => d.url).filter(Boolean)
      }
      emit('complete', evt)
    },
    onCancelled(evt) {
      status.value = 'cancelled'
      progress.value = evt.progress
      errorMessage.value = evt.error || ''
    },
    onError(err) {
      status.value = 'failed'
      errorMessage.value = err
      emit('error', err)
    },
  })
})

onUnmounted(() => {
  abortController.value?.abort()
  if (elapsedTimer) clearInterval(elapsedTimer)
})
</script>

<template>
  <div class="media-task-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
    <!-- Loading state -->
    <div v-if="isActive" class="p-4">
      <!-- Skeleton placeholder -->
      <div
        class="relative rounded-lg overflow-hidden"
        :class="type === 'video' ? 'aspect-video max-w-[400px]' : 'aspect-square max-w-[280px]'"
      >
        <!-- Animated gradient background -->
        <div class="skeleton-bg absolute inset-0" />

        <!-- Center content -->
        <div class="absolute inset-0 flex flex-col items-center justify-center gap-3">
          <!-- Orbit animation -->
          <div class="relative w-16 h-16">
            <!-- Outer ring -->
            <svg class="absolute inset-0 w-16 h-16 orbit-ring" viewBox="0 0 64 64" fill="none">
              <circle cx="32" cy="32" r="28" stroke="currentColor" stroke-width="1.5" stroke-dasharray="6 8" class="text-gray-300 dark:text-gray-500" />
            </svg>
            <!-- Inner icon -->
            <div class="absolute inset-0 flex items-center justify-center">
              <svg v-if="type === 'image'" class="w-7 h-7 text-gray-400 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909M3.75 21h16.5A2.25 2.25 0 0 0 22.5 18.75V5.25A2.25 2.25 0 0 0 20.25 3H3.75A2.25 2.25 0 0 0 1.5 5.25v13.5A2.25 2.25 0 0 0 3.75 21Z" />
              </svg>
              <svg v-else class="w-7 h-7 text-gray-400 dark:text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z" />
              </svg>
            </div>
            <!-- Dot orbiting -->
            <div class="absolute inset-0 orbit-dot">
              <div class="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-0.5 w-2 h-2 rounded-full" :class="type === 'image' ? 'bg-blue-400' : 'bg-purple-400'" />
            </div>
          </div>

          <!-- Status text -->
          <div class="text-center space-y-0.5">
            <div class="text-sm font-medium text-gray-600 dark:text-gray-300">{{ statusLabel }}</div>
            <div v-if="elapsedLabel" class="text-xs text-gray-400 dark:text-gray-500">{{ elapsedLabel }}</div>
          </div>
        </div>
      </div>

      <!-- Progress bar -->
      <div class="mt-3 space-y-1.5">
        <div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
          <span class="flex items-center gap-1.5">
            <span class="inline-block w-1.5 h-1.5 rounded-full animate-pulse" :class="type === 'image' ? 'bg-blue-400' : 'bg-purple-400'" />
            {{ t('media.generatingWithType', { type: type === 'image' ? t('media.image') : t('media.video') }) }}
          </span>
          <span class="tabular-nums">{{ progressPercent }}%</span>
        </div>
        <div class="h-1 bg-gray-100 dark:bg-gray-600 rounded-full overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-700 ease-out progress-bar"
            :class="type === 'image' ? 'bg-blue-500' : 'bg-purple-500'"
            :style="{ width: `${Math.max(progressPercent, 3)}%` }"
          />
        </div>
        <!-- Prompt preview -->
        <p v-if="prompt" class="text-xs text-gray-400 dark:text-gray-500 truncate mt-1 italic">"{{ prompt }}"</p>
      </div>
    </div>

    <!-- Completed: show results -->
    <div v-else-if="status === 'succeeded' && imageUrls.length > 0">
      <div class="p-2 grid gap-2" :class="imageUrls.length > 1 ? 'grid-cols-2' : 'grid-cols-1'">
        <div
          v-for="(url, i) in imageUrls"
          :key="i"
          class="relative overflow-hidden rounded-lg cursor-pointer group"
          :class="imageUrls.length === 1 ? 'max-w-md' : 'aspect-square'"
          @click="openLightbox(url)"
        >
          <img
            :src="url"
            :alt="`Generated ${type} ${i + 1}`"
            class="w-full h-full rounded-lg result-image"
            :class="imageUrls.length > 1 ? 'object-cover' : 'max-h-80 object-contain'"
            loading="lazy"
          />
          <!-- Hover overlay -->
          <div class="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center">
            <svg
              class="h-8 w-8 text-white opacity-0 group-hover:opacity-100 transition-opacity drop-shadow-lg"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
            </svg>
          </div>
          <!-- Download button -->
          <a
            :href="url"
            :download="`generated-${type}-${i + 1}`"
            class="absolute bottom-2 right-2 p-1.5 rounded-lg bg-black/50 text-white hover:bg-black/70 transition-all opacity-0 group-hover:opacity-100"
            @click.stop
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
          </a>
        </div>
      </div>
    </div>

    <!-- Error -->
    <div v-else-if="status === 'cancelled'" class="px-4 py-3 flex items-start gap-2.5">
      <svg class="w-4 h-4 mt-0.5 shrink-0 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6.75 6.75l10.5 10.5m0-10.5-10.5 10.5" />
      </svg>
      <div class="min-w-0">
        <p class="text-sm text-amber-500 dark:text-amber-400">{{ t('media.cancelled') }}</p>
        <p v-if="errorMessage" class="text-xs text-gray-400 dark:text-gray-500 mt-0.5 break-all">{{ errorMessage }}</p>
      </div>
    </div>

    <!-- Error -->
    <div v-else-if="status === 'failed'" class="px-4 py-3 flex items-start gap-2.5">
      <svg class="w-4 h-4 mt-0.5 shrink-0 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
      </svg>
      <div class="min-w-0">
        <p class="text-sm text-red-500 dark:text-red-400">{{ t('media.error.failed') }}</p>
        <p v-if="errorMessage" class="text-xs text-gray-400 dark:text-gray-500 mt-0.5 break-all">{{ errorMessage }}</p>
      </div>
    </div>

    <!-- Lightbox -->
    <Teleport to="body">
      <Transition name="lightbox">
        <div
          v-if="lightboxUrl"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
          @click="closeLightbox"
        >
          <button
            class="absolute top-4 right-4 p-2 text-white hover:bg-white/10 rounded-full transition-colors z-10"
            @click.stop="closeLightbox"
          >
            <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          <div class="max-w-4xl max-h-[90vh] p-4" @click.stop>
            <img :src="lightboxUrl" alt="Generated media" class="max-w-full max-h-[80vh] object-contain rounded-lg" />
          </div>
          <!-- Download in lightbox -->
          <a
            :href="lightboxUrl"
            download="generated-media"
            class="absolute bottom-6 left-1/2 -translate-x-1/2 flex items-center gap-2 px-4 py-2 rounded-lg bg-white/10 text-white hover:bg-white/20 transition-colors text-sm"
            @click.stop
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            {{ t('common.save') }}
          </a>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
/* Skeleton gradient background */
.skeleton-bg {
  background: linear-gradient(135deg, #f0f0f0 25%, #e8e8e8 50%, #f0f0f0 75%);
  background-size: 400% 400%;
  animation: skeleton-shift 2.5s ease infinite;
}

:root.dark .skeleton-bg,
[data-theme="dark"] .skeleton-bg {
  background: linear-gradient(135deg, #374151 25%, #4b5563 50%, #374151 75%);
  background-size: 400% 400%;
}

@keyframes skeleton-shift {
  0%, 100% { background-position: 0% 50%; }
  50% { background-position: 100% 50%; }
}

/* Orbiting ring */
.orbit-ring {
  animation: orbit-spin 8s linear infinite;
}

@keyframes orbit-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Orbiting dot */
.orbit-dot {
  animation: orbit-spin 3s linear infinite;
}

/* Progress bar glow */
.progress-bar {
  box-shadow: 0 0 6px rgba(59, 130, 246, 0.3);
}

/* Result image entrance */
.result-image {
  animation: result-enter 0.6s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes result-enter {
  from {
    opacity: 0;
    transform: scale(0.92);
    filter: blur(8px);
  }
  to {
    opacity: 1;
    transform: scale(1);
    filter: blur(0);
  }
}

/* Lightbox transition */
.lightbox-enter-active {
  transition: opacity 0.25s ease;
}
.lightbox-leave-active {
  transition: opacity 0.2s ease;
}
.lightbox-enter-from,
.lightbox-leave-to {
  opacity: 0;
}
</style>
