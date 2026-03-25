<script setup lang="ts">
import type { TypelessCardProfile } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardProfile
}>()

function getInitials(): string {
  return props.card.name
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)
}

function handleLinkClick(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div
    class="profile-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
  >
    <!-- Header with gradient -->
    <div class="h-20 bg-gradient-to-r from-gray-700 to-gray-900" />

    <!-- Profile content -->
    <div class="px-6 pb-6">
      <!-- Avatar -->
      <div class="relative -mt-12 mb-4">
        <div
          v-if="card.avatar"
          class="w-24 h-24 rounded-full border-4 border-white dark:border-gray-800 overflow-hidden bg-gray-100 dark:bg-gray-700"
        >
          <img :src="card.avatar" :alt="card.name" class="w-full h-full object-cover" />
        </div>
        <div
          v-else
          class="w-24 h-24 rounded-full border-4 border-white dark:border-gray-800 bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white text-2xl font-bold"
        >
          {{ getInitials() }}
        </div>
        <!-- Verified badge -->
        <div
          v-if="card.verified"
          class="absolute bottom-0 end-0 w-7 h-7 bg-gray-700 dark:bg-gray-500 rounded-full border-2 border-white dark:border-gray-800 flex items-center justify-center"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-4 w-4 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
      </div>

      <!-- Name and title -->
      <div class="mb-4">
        <h3 class="text-xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          {{ card.name }}
        </h3>
        <p v-if="card.title" class="text-sm text-gray-500 dark:text-gray-400">{{ card.title }}</p>
      </div>

      <!-- Description -->
      <p v-if="card.description" class="text-sm text-gray-600 dark:text-gray-300 mb-4">
        {{ card.description }}
      </p>

      <!-- Stats -->
      <div
        v-if="card.stats?.length"
        class="flex gap-6 mb-4 py-4 border-y border-gray-200 dark:border-gray-700"
      >
        <div v-for="(stat, index) in card.stats" :key="index" class="text-center">
          <p class="text-xl font-bold text-gray-900 dark:text-white">{{ stat.value }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</p>
        </div>
      </div>

      <!-- Links -->
      <div v-if="card.links?.length" class="flex gap-3">
        <button
          v-for="(link, index) in card.links"
          :key="index"
          class="p-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
          :title="link.label"
          @click="handleLinkClick(link.url)"
        >
          <span class="text-xl">{{ link.icon }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
