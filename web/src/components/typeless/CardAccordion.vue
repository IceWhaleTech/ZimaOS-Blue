<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import type { TypelessCardAccordion } from '@/types/typeless'

const props = defineProps<{
  card: TypelessCardAccordion
}>()

const settingsStore = useSettingsStore()
const THINKING_CARD_PREFIXES = ['thinking-', 'think-', 'think_context-']
const firstItem = computed(() => props.card.items[0] ?? null)
const isThinkingCard = computed(() => {
  const cardId = props.card.id ?? ''
  return THINKING_CARD_PREFIXES.some(prefix => cardId.startsWith(prefix))
})

// When showToolDetails is true, expand all accordion items by default
const openItems = ref<Set<number>>(new Set(
  props.card.items
    .map((item, index) => {
      // Thinking details should stay collapsed by default and expand only on click
      if (isThinkingCard.value) return -1
      // If showToolDetails is on, expand by default
      if (settingsStore.showToolDetails) return index
      return item.defaultOpen ? index : -1
    })
    .filter(i => i >= 0)
))

// Watch for changes to showToolDetails and update openItems accordingly
watch(
  () => settingsStore.showToolDetails,
  (showToolDetails) => {
    if (isThinkingCard.value) return
    if (showToolDetails) {
      // Expand all items when showToolDetails is enabled
      openItems.value = new Set(props.card.items.map((_, index) => index))
    } else {
      // Close to defaultOpen state when disabled
      openItems.value = new Set(
        props.card.items
          .map((item, index) => item.defaultOpen ? index : -1)
          .filter(i => i >= 0)
      )
    }
  }
)

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
  <div class="accordion-card rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden bg-white dark:bg-gray-700">
    <!-- Accordion items - single item mode for thinking cards -->
    <div v-if="card.items.length === 1 && firstItem" class="accordion-single">
      <!-- Header -->
      <button
        class="w-full px-3 py-2 flex items-center justify-between text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
        @click="toggleItem(0)"
      >
        <div class="flex items-center gap-2">
          <span v-if="firstItem.icon" class="text-sm">{{ firstItem.icon }}</span>
          <span v-if="isThinkingCard" class="thinking-shimmer text-sm font-semibold">Thinking</span>
          <span v-else class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ card.title }}</span>
        </div>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-4 w-4 text-gray-400 transition-transform duration-200"
          :class="{ 'rotate-180': isOpen(0) }"
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
        :class="isOpen(0) ? 'max-h-60' : 'max-h-0'"
      >
        <div class="px-3 pb-3 text-sm text-gray-600 dark:text-gray-300">
          {{ firstItem.content }}
        </div>
      </div>
    </div>

    <!-- Multi-item mode (original style) -->
    <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
      <!-- Title -->
      <div v-if="card.title" class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h4 class="font-medium text-gray-900 dark:text-white">{{ card.title }}</h4>
      </div>

      <div v-for="(item, index) in card.items" :key="index">
        <!-- Header -->
        <button
          class="w-full px-4 py-3 flex items-center justify-between text-left hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
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

<style scoped>
.thinking-shimmer {
  background: linear-gradient(90deg, #6b7280 0%, #9ca3af 30%, #111827 50%, #9ca3af 70%, #6b7280 100%);
  background-size: 220% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  animation: thinking-shimmer 1.8s linear infinite;
}

:global(.dark) .thinking-shimmer {
  background: linear-gradient(90deg, #9ca3af 0%, #d1d5db 30%, #f9fafb 50%, #d1d5db 70%, #9ca3af 100%);
  background-size: 220% 100%;
}

@keyframes thinking-shimmer {
  0% {
    background-position: -120% 0;
  }
  100% {
    background-position: 120% 0;
  }
}
</style>
