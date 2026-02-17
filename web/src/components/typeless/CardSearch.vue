<script setup lang="ts">
import type { TypelessCardSearch } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardSearch
}>()

function getDomain(url: string): string {
  try {
    return new URL(url).hostname
  } catch {
    return url
  }
}

function openUrl(url: string) {
  window.open(url, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div class="search-card rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden">
    <!-- Header -->
    <div class="flex items-center gap-2 px-4 py-3 border-b border-gray-100 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
      <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-gray-500 dark:text-gray-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
      </svg>
      <span class="text-sm text-gray-600 dark:text-gray-300 truncate">
        {{ props.card.query }}
      </span>
      <span class="text-xs text-gray-400 dark:text-gray-500 flex-shrink-0">
        {{ props.card.results?.length || 0 }} 条结果
      </span>
    </div>

    <!-- Results -->
    <div class="divide-y divide-gray-100 dark:divide-gray-700">
      <div
        v-for="(result, i) in props.card.results?.slice(0, 8)"
        :key="i"
        class="px-4 py-3 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
        @click="openUrl(result.url)"
      >
        <div class="text-xs text-gray-400 dark:text-gray-500 truncate mb-1">
          {{ getDomain(result.url) }}
        </div>
        <div class="text-sm font-medium text-blue-600 dark:text-blue-400 line-clamp-1">
          {{ result.title }}
        </div>
        <p v-if="result.description" class="mt-1 text-xs text-gray-500 dark:text-gray-400 line-clamp-2">
          {{ result.description }}
        </p>
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
