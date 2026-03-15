<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { TypelessCardMediaGenerate, GalleryImage } from '@/types/typeless'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  card: TypelessCardMediaGenerate
}>()

const { t } = useI18n()
const selectedImage = ref<GalleryImage | null>(null)
const pollTimer = ref<ReturnType<typeof setInterval> | null>(null)
const taskResult = ref<TypelessCardMediaGenerate | null>(null)

// Use polled result if available, otherwise use card prop
const displayCard = computed(() => taskResult.value || props.card)

const isGenerating = computed(() => displayCard.value.status === 'generating')
const isSuccess = computed(() => displayCard.value.status === 'success')
const isError = computed(() => displayCard.value.status === 'error')
const images = computed(() => displayCard.value.images || [])

function openLightbox(image: GalleryImage) {
  selectedImage.value = image
}

function closeLightbox() {
  selectedImage.value = null
}

function getImageSrc(image: GalleryImage): string {
  return image.thumbnail || image.src
}

// Poll task status when generating
async function pollTask() {
  if (!props.card.task_id || !isGenerating.value) return
  try {
    const resp = await fetch(`/api/v1/media/tasks/${props.card.task_id}`)
    if (!resp.ok) return
    const task = await resp.json()
    if (task.status === 'succeeded' && task.response?.data?.length > 0) {
      const imgs: GalleryImage[] = task.response.data.map((d: any) => ({
        src: d.url || d.original_url,
        thumbnail: d.thumbnail_url,
        caption: d.revised_prompt,
      }))
      taskResult.value = {
        ...props.card,
        status: 'success',
        images: imgs,
      }
      stopPolling()
    } else if (task.status === 'failed') {
      taskResult.value = {
        ...props.card,
        status: 'error',
        message: task.error || 'Generation failed',
      }
      stopPolling()
    }
  } catch {
    // Ignore poll errors
  }
}

function stopPolling() {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = null
  }
}

onMounted(() => {
  if (isGenerating.value && props.card.task_id) {
    pollTimer.value = setInterval(pollTask, 3000)
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div
    class="media-generate-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800"
  >
    <!-- Generating: dreamy gradient placeholder -->
    <div v-if="isGenerating" class="relative">
      <div
        class="generating-placeholder w-full aspect-[4/3] max-h-80 flex items-center justify-center"
      >
        <div class="text-center z-10">
          <div class="mb-3">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-10 w-10 text-white/80 mx-auto animate-pulse"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="1.5"
                d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
              />
            </svg>
          </div>
          <p class="text-white/90 text-sm font-medium">
            {{
              card.media_type === 'video'
                ? t('chat.generatingVideo', 'Generating video...')
                : t('chat.generatingImage', 'Generating image...')
            }}
          </p>
          <p class="text-white/60 text-xs mt-1">
            {{ t('chat.pleaseWait', 'This may take 15-60 seconds') }}
          </p>
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div v-else-if="isError" class="p-4 flex items-center gap-3">
      <div
        class="w-10 h-10 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center flex-shrink-0"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 text-red-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          />
        </svg>
      </div>
      <div>
        <p class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('chat.generationFailed', 'Generation failed') }}
        </p>
        <p v-if="displayCard.message" class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
          {{ displayCard.message }}
        </p>
      </div>
    </div>

    <!-- Success: show images -->
    <div v-else-if="isSuccess && images.length > 0" class="relative">
      <div class="flex gap-2 p-2 overflow-x-auto scrollbar-thin">
        <div
          v-for="(image, index) in images"
          :key="index"
          class="flex-shrink-0 relative overflow-hidden rounded-lg cursor-pointer group"
          :class="images.length === 1 ? 'w-full max-w-md' : 'w-48 h-48'"
          @click="openLightbox(image)"
        >
          <img
            :src="getImageSrc(image)"
            :alt="image.alt || ''"
            class="w-full h-full object-cover transition-transform group-hover:scale-105"
            :class="images.length === 1 ? 'max-h-80' : ''"
          />
          <div
            class="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-8 w-8 text-white opacity-0 group-hover:opacity-100 transition-opacity"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7"
              />
            </svg>
          </div>
          <div
            v-if="image.caption"
            class="absolute bottom-0 left-0 right-0 p-2 bg-gradient-to-t from-black/70 to-transparent"
          >
            <p class="text-xs text-white truncate">{{ image.caption }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Lightbox -->
    <Teleport to="body">
      <div
        v-if="selectedImage"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/90"
        @click="closeLightbox"
      >
        <button
          class="absolute top-4 right-4 p-2 text-white hover:bg-white/10 rounded-full transition-colors"
          @click.stop="closeLightbox"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-6 w-6"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
        <div class="max-w-4xl max-h-[90vh] p-4" @click.stop>
          <img
            :src="selectedImage.src"
            :alt="selectedImage.alt || ''"
            class="max-w-full max-h-[80vh] object-contain rounded-lg"
          />
          <p v-if="selectedImage.caption" class="mt-3 text-center text-white text-sm">
            {{ selectedImage.caption }}
          </p>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.generating-placeholder {
  background: linear-gradient(-45deg, #ee7752, #e73c7e, #c084fc, #23a6d5, #23d5ab);
  background-size: 400% 400%;
  animation: gradient-shift 6s ease infinite;
}

@keyframes gradient-shift {
  0% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
  100% {
    background-position: 0% 50%;
  }
}

.scrollbar-thin {
  scrollbar-width: thin;
}
.scrollbar-thin::-webkit-scrollbar {
  height: 6px;
}
.scrollbar-thin::-webkit-scrollbar-track {
  background: transparent;
}
.scrollbar-thin::-webkit-scrollbar-thumb {
  background-color: rgba(156, 163, 175, 0.5);
  border-radius: 3px;
}
</style>
