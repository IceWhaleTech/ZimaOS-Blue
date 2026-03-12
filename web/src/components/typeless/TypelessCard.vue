<script setup lang="ts">
import { computed, shallowRef, watch, type Component } from 'vue'
import type { TypelessCard } from '@/types/typeless'
import { canRenderFunctionally, renderCardToHtml } from '@/utils/typelessRenderers'
import { componentPool } from '@/utils/componentPool'

const props = defineProps<{
  card: TypelessCard
  actionLoading?: boolean
  activeActionId?: string
  actionError?: string
}>()

const emit = defineEmits<{
  action: [actionId: string, cardId?: string]
  select: [cardId: string, selectedIds: string[], otherText?: string]
}>()

// Check if card is streaming (incomplete)
const isStreaming = computed(() => props.card._streaming === true)

// Check if card can be rendered functionally (simple cards)
const isFunctional = computed(() => canRenderFunctionally(props.card))
const supportsActionLoading = computed(() => ['result', 'web-fetch', 'action', 'ui-review', 'choice'].includes(props.card.type))

// Pre-render HTML for functional cards
const functionalHtml = computed(() => {
  if (!isFunctional.value) return ''
  return renderCardToHtml(props.card)
})

// Dynamic component from pool
const dynamicComponent = shallowRef<Component | null>(null)
const isLoading = shallowRef(false)

// Load component from pool when card type changes
async function loadComponent(cardType: string) {
  if (isFunctional.value) return

  isLoading.value = true
  try {
    const comp = await componentPool.getComponent(cardType)
    dynamicComponent.value = comp
  } catch (err) {
    console.error(`Failed to load component for ${cardType}:`, err)
    dynamicComponent.value = null
  } finally {
    isLoading.value = false
  }
}

// Watch for card type changes
watch(
  () => [props.card.type, isFunctional.value] as const,
  ([newType, functional]) => {
    if (functional) {
      dynamicComponent.value = null
      return
    }
    loadComponent(newType)
  },
  { immediate: true }
)

function handleAction(actionId: string, cardId?: string) {
  if (cardId) {
    emit('action', actionId, cardId)
  }
}

function handleSelect(selectedIds: string[], otherText?: string) {
  // Emit selection event with card ID
  emit('select', props.card.id || '', selectedIds, otherText)
}
</script>

<template>
  <div :id="card.id" class="typeless-card my-3 relative" :class="{ 'opacity-80': isStreaming }">
    <!-- Streaming indicator for incomplete cards -->
    <div v-if="isStreaming" class="absolute top-2 right-2 z-10">
      <div class="w-2 h-2 bg-gray-700 dark:bg-gray-500 rounded-full animate-pulse" />
    </div>

    <!-- Functional rendering for simple cards (table, code, list, info, quote, alert, terminal) -->
    <!-- Dynamic components: progress, action, result, detection, chart, gallery, file, link, metric, comparison, steps, map, weather, profile, countdown, rating, accordion, audio, choice, collapsible-code, diff, video, ui-review-progress -->
    <div v-if="isFunctional" v-html="functionalHtml" />

    <!-- Loading state -->
    <div
      v-else-if="isLoading"
      class="animate-pulse bg-gray-700 dark:bg-gray-500 rounded-lg h-24"
    />

    <!-- Dynamic component from pool -->
    <template v-else-if="dynamicComponent">
      <component
        :is="dynamicComponent"
        :card="card"
        :action-loading="supportsActionLoading ? actionLoading : undefined"
        :active-action-id="supportsActionLoading ? activeActionId : undefined"
        @action="handleAction"
        @select="handleSelect"
      />
      <div
        v-if="supportsActionLoading && actionError"
        class="mt-2 rounded-lg border border-red-200 dark:border-red-800/60 bg-red-50 dark:bg-red-900/20 px-3 py-2 text-sm text-red-700 dark:text-red-200"
      >
        {{ actionError }}
      </div>
    </template>

    <!-- Fallback for unknown types: hide silently (streaming may produce partial/invalid types) -->
    <template v-else />
  </div>
</template>
