<script setup lang="ts">
import { ref } from 'vue'
import type { TypelessCardAccordion } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardAccordion
}>()

const openItems = ref<Set<number>>(new Set(
  props.card.items
    .map((item, index) => item.defaultOpen ? index : -1)
    .filter(i => i >= 0)
))

function toggleItem(index: number) {
  if (openItems.value.has(index)) {
    openItems.value.delete(index)
  } else {
    if (!props.card.allowMultiple) {
      openItems.value.clear()
    }
    openItems.value.add(index)
  }
  openItems.value = new Set(openItems.value)
}

function isOpen(index: number): boolean {
  return openItems.value.has(index)
}
</script>

<template>
  <div class="accordion-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-800">
    <!-- Title -->
    <div v-if="card.title" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Accordion items -->
    <div class="divide-y divide-gray-200 dark:divide-gray-700">
      <div v-for="(item, index) in card.items" :key="index">
        <!-- Header -->
        <button
          class="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
          @click="toggleItem(index)"
        >
          <div class="flex items-center gap-3">
            <span v-if="item.icon" class="text-lg">{{ item.icon }}</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ item.title }}</span>
          </div>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            class="h-5 w-5 text-gray-400 transition-transform duration-200"
            :class="{ 'rotate-180': isOpen(index) }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        <!-- Content -->
        <div
          class="overflow-hidden transition-all duration-200"
          :class="isOpen(index) ? 'max-h-96' : 'max-h-0'"
        >
          <div class="px-4 pb-4 text-sm text-gray-600 dark:text-gray-300">
            {{ item.content }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
