<template>
  <div class="space-y-4">
    <div v-if="loading" class="text-center py-10 text-gray-500 dark:text-gray-400">Loading...</div>

    <div v-else-if="personalities.length === 0" class="text-center py-10 text-gray-500 dark:text-gray-400">
      No personalities yet. Create one to get started.
    </div>

    <div v-else class="grid gap-4">
      <div v-for="p in personalities" :key="p.id" class="border border-gray-200 dark:border-gray-700 rounded-lg p-4 bg-white dark:bg-gray-800">
        <div class="flex justify-between items-center mb-3">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ p.name }}</h3>
          <span v-if="activeId === p.id" class="px-2 py-0.5 text-xs font-medium bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300 rounded">Active</span>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400 mb-2">{{ p.description }}</p>
        <div
          class="text-xs bg-gray-50 dark:bg-gray-700/50 text-gray-600 dark:text-gray-300 p-3 rounded mb-3 max-h-48 overflow-y-auto prose-content"
          v-html="renderMarkdown(p.system_prompt)"
        />
        <div class="flex gap-2">
          <button @click="$emit('edit', p)" class="px-3 py-1.5 text-xs font-medium bg-amber-500 hover:bg-amber-600 text-white rounded transition-colors">Edit</button>
          <button
            v-if="activeId !== p.id"
            @click="$emit('activate', p.id)"
            class="px-3 py-1.5 text-xs font-medium bg-green-600 hover:bg-green-700 text-white rounded transition-colors"
          >
            Activate
          </button>
          <button @click="$emit('delete', p.id)" class="px-3 py-1.5 text-xs font-medium bg-red-500 hover:bg-red-600 text-white rounded transition-colors">Delete</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Personality } from '@/api/personality'
import { renderMarkdown } from '@/utils/markdown'

defineProps<{
  personalities: Personality[]
  activeId?: string
  loading?: boolean
}>()

defineEmits<{
  edit: [personality: Personality]
  activate: [id: string]
  delete: [id: string]
}>()
</script>
