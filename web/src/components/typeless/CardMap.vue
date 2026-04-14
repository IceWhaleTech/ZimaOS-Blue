<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TypelessCardMap } from '@/types/typeless'

const { t } = useI18n()

const props = defineProps<{
  card: TypelessCardMap
}>()

// Generate OpenStreetMap embed URL
const mapUrl = computed(() => {
  const lat = props.card.latitude
  const lon = props.card.longitude
  return `https://www.openstreetmap.org/export/embed.html?bbox=${lon - 0.01},${lat - 0.01},${lon + 0.01},${lat + 0.01}&layer=mapnik&marker=${lat},${lon}`
})

// Generate link to full map
const fullMapUrl = computed(() => {
  const zoom = props.card.zoom || 15
  return `https://www.openstreetmap.org/?mlat=${props.card.latitude}&mlon=${props.card.longitude}#map=${zoom}/${props.card.latitude}/${props.card.longitude}`
})

function openInMaps() {
  window.open(fullMapUrl.value, '_blank')
}
</script>

<template>
  <div
    class="map-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
  >
    <!-- Title -->
    <div
      v-if="card.title"
      class="px-4 py-3 border-b border-gray-200 dark:border-gray-700"
    >
      <h4 class="font-medium text-gray-900 dark:text-white">
        {{ card.title }}
      </h4>
    </div>

    <!-- Map embed -->
    <div class="relative aspect-video bg-gray-100 dark:bg-gray-700">
      <iframe
        :src="mapUrl"
        class="w-full h-full border-0"
        loading="lazy"
        referrerpolicy="no-referrer-when-downgrade"
      />
    </div>

    <!-- Address and actions -->
    <div class="p-4 flex items-center justify-between gap-4">
      <div class="flex items-start gap-3 min-w-0">
        <div
          class="flex-shrink-0 w-10 h-10 rounded-full bg-red-100 dark:bg-red-900/30 flex items-center justify-center"
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
              d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z"
            />
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15 11a3 3 0 11-6 0 3 3 0 016 0z"
            />
          </svg>
        </div>
        <div class="min-w-0">
          <p
            v-if="card.address"
            class="text-sm text-gray-900 dark:text-white truncate"
          >
            {{ card.address }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ card.latitude.toFixed(6) }}, {{ card.longitude.toFixed(6) }}
          </p>
        </div>
      </div>
      <button
        class="flex-shrink-0 px-3 py-2 text-sm font-medium text-gray-900 dark:text-white dark:text-white hover:bg-gray-700 dark:bg-gray-500 dark:hover:bg-gray-600 rounded-lg transition-colors"
        @click="openInMaps"
      >
        {{ t('mapCard.openInMaps', 'Open in Maps') }}
      </button>
    </div>
  </div>
</template>
