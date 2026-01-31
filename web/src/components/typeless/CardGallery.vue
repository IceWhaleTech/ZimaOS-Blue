<script setup lang="ts">
import { ref } from 'vue'
import type { TypelessCardGallery, GalleryImage } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardGallery
}>()

const selectedImage = ref<GalleryImage | null>(null)
const scrollContainer = ref<HTMLElement | null>(null)

function openLightbox(image: GalleryImage) {
  selectedImage.value = image
}

function closeLightbox() {
  selectedImage.value = null
}

function getGridCols(): string {
  const cols = props.card.columns || 2
  return {
    2: 'grid-cols-2',
    3: 'grid-cols-3',
    4: 'grid-cols-4',
  }[cols] || 'grid-cols-2'
}

function isHorizontalLayout(): boolean {
  return props.card.layout === 'horizontal'
}

function getImageSrc(image: GalleryImage): string {
  // Use thumbnail if available, otherwise use original src
  return image.thumbnail || image.src
}

function scrollLeft() {
  if (scrollContainer.value) {
    scrollContainer.value.scrollBy({ left: -200, behavior: 'smooth' })
  }
}

function scrollRight() {
  if (scrollContainer.value) {
    scrollContainer.value.scrollBy({ left: 200, behavior: 'smooth' })
  }
}
</script>

<template>
  <div class="gallery-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <!-- Title -->
    <div v-if="card.title" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Horizontal Scroll Layout -->
    <div v-if="isHorizontalLayout()" class="relative group">
      <!-- Scroll buttons -->
      <button
        v-if="card.images.length > 2"
        class="absolute left-2 top-1/2 -translate-y-1/2 z-10 w-8 h-8 rounded-full bg-black/50 text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity hover:bg-black/70"
        @click="scrollLeft"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
        </svg>
      </button>
      <button
        v-if="card.images.length > 2"
        class="absolute right-2 top-1/2 -translate-y-1/2 z-10 w-8 h-8 rounded-full bg-black/50 text-white flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity hover:bg-black/70"
        @click="scrollRight"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
        </svg>
      </button>

      <!-- Scrollable container -->
      <div
        ref="scrollContainer"
        class="flex gap-2 p-2 overflow-x-auto scrollbar-thin scrollbar-thumb-gray-300 dark:scrollbar-thumb-gray-600 scrollbar-track-transparent"
      >
        <div
          v-for="(image, index) in card.images"
          :key="index"
          class="flex-shrink-0 relative overflow-hidden rounded-lg cursor-pointer group/item"
          :class="card.images.length === 1 ? 'w-full max-w-md' : 'w-48 h-48'"
          @click="openLightbox(image)"
        >
          <img
            :src="getImageSrc(image)"
            :alt="image.alt || ''"
            class="w-full h-full object-cover transition-transform group-hover/item:scale-105"
            :class="card.images.length === 1 ? 'max-h-80' : ''"
          />
          <!-- Overlay on hover -->
          <div class="absolute inset-0 bg-black/0 group-hover/item:bg-black/30 transition-colors flex items-center justify-center">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-8 w-8 text-white opacity-0 group-hover/item:opacity-100 transition-opacity"
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
          <!-- Caption -->
          <div v-if="image.caption" class="absolute bottom-0 left-0 right-0 p-2 bg-gradient-to-t from-black/70 to-transparent">
            <p class="text-xs text-white truncate">{{ image.caption }}</p>
          </div>
        </div>
      </div>
    </div>

    <!-- Grid Layout (default) -->
    <div v-else class="p-2 grid gap-2" :class="getGridCols()">
      <div
        v-for="(image, index) in card.images"
        :key="index"
        class="relative aspect-square overflow-hidden rounded-lg cursor-pointer group"
        @click="openLightbox(image)"
      >
        <img :src="getImageSrc(image)" :alt="image.alt || ''" class="w-full h-full object-cover transition-transform group-hover:scale-105" />
        <!-- Overlay on hover -->
        <div class="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center">
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
        <!-- Caption -->
        <div v-if="image.caption" class="absolute bottom-0 left-0 right-0 p-2 bg-gradient-to-t from-black/70 to-transparent">
          <p class="text-xs text-white truncate">{{ image.caption }}</p>
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
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <div class="max-w-4xl max-h-[90vh] p-4" @click.stop>
          <img :src="selectedImage.src" :alt="selectedImage.alt || ''" class="max-w-full max-h-[80vh] object-contain rounded-lg" />
          <p v-if="selectedImage.caption" class="mt-3 text-center text-white">{{ selectedImage.caption }}</p>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
/* Custom scrollbar for horizontal scroll */
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

.scrollbar-thin::-webkit-scrollbar-thumb:hover {
  background-color: rgba(156, 163, 175, 0.8);
}
</style>
