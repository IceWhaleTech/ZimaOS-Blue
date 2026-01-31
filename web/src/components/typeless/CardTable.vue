<script setup lang="ts">
import type { TypelessCardTable } from '@/types/typeless'
import { parseInline } from '@/utils/markdown'

defineProps<{
  card: TypelessCardTable
}>()

function renderContent(content: string | number): string {
  if (typeof content === 'number') {
    return String(content)
  }
  return parseInline(content)
}
</script>

<template>
  <div class="table-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <!-- Title -->
    <div v-if="card.title" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Table -->
    <div class="overflow-x-auto">
      <table class="w-full" :class="{ 'text-sm': card.compact }">
        <thead>
          <tr class="bg-gray-50 dark:bg-gray-800/50">
            <th
              v-for="(header, index) in card.headers"
              :key="index"
              class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider"
              v-html="renderContent(header)"
            />
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr
            v-for="(row, rowIndex) in card.rows"
            :key="rowIndex"
            :class="{
              'bg-gray-50 dark:bg-gray-800/30': card.striped && rowIndex % 2 === 1,
              'hover:bg-gray-50 dark:hover:bg-gray-800/50': true
            }"
          >
            <td
              v-for="(cell, cellIndex) in row"
              :key="cellIndex"
              class="px-4 py-3 text-gray-700 dark:text-gray-300"
              :class="{ 'py-2': card.compact }"
              v-html="renderContent(cell)"
            />
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Footer -->
    <div v-if="card.footer" class="px-4 py-2 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800/50">
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ card.footer }}</p>
    </div>
  </div>
</template>
