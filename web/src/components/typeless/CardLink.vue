<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import type { TypelessCardLink } from '@/types/typeless'
import api from '@/api/client'

const props = defineProps<{
  card: TypelessCardLink
}>()

// Local state for fetched metadata
const fetchedTitle = ref<string | null>(null)
const fetchedDescription = ref<string | null>(null)
const fetchedImage = ref<string | null>(null)
const fetchedFavicon = ref<string | null>(null)
const fetchedSiteName = ref<string | null>(null)
const isLoading = ref(false)
const hasError = ref(false)

// Use fetched data or fallback to card props
const displayTitle = computed(() => fetchedTitle.value || props.card.title)
const displayDescription = computed(() => fetchedDescription.value || props.card.description)
const displayImage = computed(() => fetchedImage.value || props.card.image)
const displayFavicon = computed(() => fetchedFavicon.value || props.card.favicon)
const displaySiteName = computed(() => fetchedSiteName.value || props.card.siteName)

function handleClick() {
  window.open(props.card.url, '_blank', 'noopener,noreferrer')
}

function getDomain(): string {
  try {
    const url = new URL(props.card.url)
    return url.hostname
  } catch {
    return props.card.url
  }
}

// Fetch link preview metadata
async function fetchLinkPreview() {
  // Skip if we already have complete metadata
  if (props.card.title && props.card.description && props.card.image) {
    return
  }

  isLoading.value = true
  hasError.value = false

  try {
    const response = await api.get('/link-preview', {
      params: { url: props.card.url },
      timeout: 10000,
    })

    if (response.data) {
      fetchedTitle.value = response.data.title || null
      fetchedDescription.value = response.data.description || null
      fetchedImage.value = response.data.image || null
      fetchedFavicon.value = response.data.favicon || null
      fetchedSiteName.value = response.data.siteName || null
    }
  } catch {
    // Silently fail - we'll just show the URL
    hasError.value = true
  } finally {
    isLoading.value = false
  }
}

onMounted(() => {
  fetchLinkPreview()
})
</script>

<template>
  <div
    class="link-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700 cursor-pointer hover:border-gray-900 dark:border-white dark:hover:border-gray-900 dark:border-white transition-colors"
    @click="handleClick"
  >
    <div class="flex">
      <!-- Image preview -->
      <div v-if="displayImage" class="flex-shrink-0 w-32 h-24 bg-gray-100 dark:bg-gray-700">
        <img :src="displayImage" :alt="displayTitle" class="w-full h-full object-cover" />
      </div>
      <!-- Loading placeholder -->
      <div
        v-else-if="isLoading"
        class="flex-shrink-0 w-32 h-24 bg-gray-100 dark:bg-gray-700 animate-pulse"
      />

      <!-- Content -->
      <div class="flex-1 p-4 min-w-0">
        <!-- Site info -->
        <div class="flex items-center gap-2 mb-2">
          <img
            v-if="displayFavicon"
            :src="displayFavicon"
            class="w-4 h-4"
            :alt="displaySiteName || ''"
          />
          <div
            v-else-if="isLoading"
            class="w-4 h-4 bg-gray-200 dark:bg-gray-600 rounded animate-pulse"
          />
          <span class="text-xs text-gray-500 dark:text-gray-400 truncate">
            {{ displaySiteName || getDomain() }}
          </span>
        </div>

        <!-- Title -->
        <h4
          v-if="!isLoading"
          class="font-medium text-gray-900 dark:text-white line-clamp-1 hover:text-gray-900 dark:text-white dark:hover:text-gray-900 dark:text-white transition-colors"
        >
          {{ displayTitle }}
        </h4>
        <div v-else class="h-5 bg-gray-200 dark:bg-gray-600 rounded animate-pulse w-3/4" />

        <!-- Description -->
        <p
          v-if="displayDescription && !isLoading"
          class="mt-1 text-sm text-gray-500 dark:text-gray-400 line-clamp-2"
        >
          {{ displayDescription }}
        </p>
        <div v-else-if="isLoading" class="mt-1 space-y-1">
          <div class="h-4 bg-gray-200 dark:bg-gray-600 rounded animate-pulse w-full" />
          <div class="h-4 bg-gray-200 dark:bg-gray-600 rounded animate-pulse w-2/3" />
        </div>
      </div>

      <!-- Arrow icon -->
      <div class="flex-shrink-0 flex items-center pe-4">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-5 w-5 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
          />
        </svg>
      </div>
    </div>
  </div>
</template>

<style scoped>
.line-clamp-1 {
  display: -webkit-box;
  -webkit-line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.line-clamp-2 {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
