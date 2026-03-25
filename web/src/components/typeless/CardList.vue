<script setup lang="ts">
import type { TypelessCardList } from '@/types/typeless'
import { parseInline } from '@/utils/markdown'

defineProps<{
  card: TypelessCardList
}>()

const emit = defineEmits<{
  toggle: [itemIndex: number]
}>()

function handleToggle(index: number) {
  emit('toggle', index)
}

function renderContent(content: string): string {
  return parseInline(content, { allowUnderscoreEmphasis: false })
}
</script>

<template>
  <div
    class="list-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700"
  >
    <!-- Title -->
    <div v-if="card.title" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
      <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
    </div>

    <!-- Default List -->
    <component
      :is="card.ordered ? 'ol' : 'ul'"
      v-if="card.variant !== 'checklist' && card.variant !== 'timeline'"
      class="p-4 space-y-2"
      :class="{ 'list-decimal list-inside': card.ordered }"
    >
      <li
        v-for="(item, index) in card.items"
        :key="index"
        class="flex items-start gap-2 text-gray-700 dark:text-gray-300"
      >
        <span v-if="item.icon" class="flex-shrink-0">{{ item.icon }}</span>
        <span
          v-else-if="!card.ordered"
          class="flex-shrink-0 w-1.5 h-1.5 mt-2 rounded-full bg-gray-400"
        />
        <div class="flex-1">
          <span v-html="renderContent(item.content)" />
          <!-- Sub-items -->
          <ul v-if="item.subItems?.length" class="list-subitems mt-2 space-y-1">
            <li
              v-for="(subItem, subIndex) in item.subItems"
              :key="subIndex"
              class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-400"
            >
              <span v-if="subItem.icon">{{ subItem.icon }}</span>
              <span v-else class="flex-shrink-0 w-1 h-1 mt-2 rounded-full bg-gray-300" />
              <span v-html="renderContent(subItem.content)" />
            </li>
          </ul>
        </div>
      </li>
    </component>

    <!-- Checklist -->
    <ul v-else-if="card.variant === 'checklist'" class="p-4 space-y-2">
      <li
        v-for="(item, index) in card.items"
        :key="index"
        class="flex items-start gap-3 cursor-pointer group"
        @click="handleToggle(index)"
      >
        <div
          class="flex-shrink-0 w-5 h-5 mt-0.5 rounded border-2 flex items-center justify-center transition-colors"
          :class="
            item.checked
              ? 'bg-green-500 border-green-500'
              : 'border-gray-300 dark:border-gray-600 group-hover:border-green-400'
          "
        >
          <svg
            v-if="item.checked"
            xmlns="http://www.w3.org/2000/svg"
            class="h-3 w-3 text-white"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="3"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
        <span
          class="flex-1 text-gray-700 dark:text-gray-300 transition-colors"
          :class="{ 'line-through text-gray-400 dark:text-gray-500': item.checked }"
          v-html="renderContent(item.content)"
        />
      </li>
    </ul>

    <!-- Timeline -->
    <div v-else-if="card.variant === 'timeline'" class="p-4">
      <div class="relative">
        <!-- Timeline line -->
        <div
          class="list-timeline-line absolute top-2 bottom-2 w-0.5 bg-gray-700 dark:bg-gray-500"
        />

        <!-- Timeline items -->
        <div class="space-y-4">
          <div
            v-for="(item, index) in card.items"
            :key="index"
            class="list-timeline-item relative flex items-start gap-4"
          >
            <!-- Timeline dot -->
            <div
              class="list-timeline-dot absolute w-4 h-4 rounded-full border-2 bg-white dark:bg-gray-700"
              :class="
                index === 0
                  ? 'border-gray-900 dark:border-white bg-gray-700 dark:bg-gray-500'
                  : 'border-gray-300 dark:border-gray-600'
              "
            />
            <div class="flex-1 min-w-0">
              <p class="text-gray-700 dark:text-gray-300" v-html="renderContent(item.content)" />
              <p v-if="item.timestamp" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ item.timestamp }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.list-subitems {
  margin-inline-start: 1rem;
}

.list-timeline-line {
  inset-inline-start: 0.5rem;
}

.list-timeline-item {
  padding-inline-start: 1.5rem;
}

.list-timeline-dot {
  inset-inline-start: 0;
}
</style>
