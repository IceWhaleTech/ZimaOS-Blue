<script setup lang="ts">
import { computed, shallowRef, watch, onMounted, type Component } from 'vue'
import type { TypelessCard } from '@/types/typeless'
import { canRenderFunctionally, renderCardToHtml } from '@/utils/typelessRenderers'
import { componentPool } from '@/utils/componentPool'

const props = defineProps<{
  card: TypelessCard
}>()

const emit = defineEmits<{
  action: [cardId: string, actionId: string]
  select: [cardId: string, selectedIds: string[], otherText?: string]
}>()

// Check if card can be rendered functionally (simple cards)
const isFunctional = computed(() => canRenderFunctionally(props.card))

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
  () => props.card.type,
  (newType) => {
    if (!canRenderFunctionally(props.card)) {
      loadComponent(newType)
    }
  },
  { immediate: true }
)

// Also load on mount if not functional
onMounted(() => {
  if (!isFunctional.value) {
    loadComponent(props.card.type)
  }
})

function handleAction(actionId: string, cardId?: string) {
  if (cardId) {
    emit('action', cardId, actionId)
  }
}

function handleSelect(_selectedIds: string[], _otherText?: string) {
  // Emit selection event if needed
}
</script>

<template>
  <div class="typeless-card my-3">
    <!-- Functional rendering for simple cards (table, code, list, info, quote, alert) -->
    <div v-if="isFunctional" v-html="functionalHtml" />

    <!-- Loading state -->
    <div
      v-else-if="isLoading"
      class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded-lg h-24"
    />

    <!-- Dynamic component from pool -->
    <component
      v-else-if="dynamicComponent"
      :is="dynamicComponent"
      :card="card"
      @action="handleAction"
      @select="handleSelect"
    />

    <!-- Fallback for unknown types -->
    <div v-else class="text-red-500 text-sm p-2">
      Unknown card type: {{ card.type }}
    </div>
  </div>
</template>
