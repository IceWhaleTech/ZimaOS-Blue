import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { DashboardCardState, DashboardLayout } from '@/components/dashboard/types'
import { cardRegistry, getDefaultCardStates } from '@/components/dashboard/cardRegistry'

const STORAGE_KEY = 'dashboard-layout'

export const useDashboardStore = defineStore('dashboard', () => {
  // State
  const cardStates = ref<DashboardCardState[]>([])
  const isCustomizing = ref(false)

  // Initialize from localStorage or defaults
  function initialize() {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      try {
        const layout: DashboardLayout = JSON.parse(stored)
        // Merge with registry to handle new cards
        const storedIds = new Set(layout.cards.map((c) => c.id))
        const mergedCards = [...layout.cards]

        // Add any new cards from registry that aren't in stored layout
        for (const card of cardRegistry) {
          if (!storedIds.has(card.id)) {
            mergedCards.push({
              id: card.id,
              enabled: card.defaultEnabled,
              order: card.defaultOrder,
              collapsed: false,
            })
          }
        }

        // Remove cards that no longer exist in registry
        const registryIds = new Set(cardRegistry.map((c) => c.id))
        cardStates.value = mergedCards.filter((c) => registryIds.has(c.id))
      } catch {
        cardStates.value = getDefaultCardStates()
      }
    } else {
      cardStates.value = getDefaultCardStates()
    }
  }

  // Save to localStorage
  function saveLayout() {
    const layout: DashboardLayout = {
      cards: cardStates.value,
      lastModified: new Date().toISOString(),
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(layout))
  }

  // Computed
  const enabledCards = computed(() => {
    return cardStates.value
      .filter((state) => state.enabled)
      .sort((a, b) => a.order - b.order)
      .map((state) => {
        const config = cardRegistry.find((c) => c.id === state.id)
        return { ...state, config }
      })
      .filter((card) => card.config !== undefined)
  })

  const disabledCards = computed(() => {
    return cardStates.value
      .filter((state) => !state.enabled)
      .map((state) => {
        const config = cardRegistry.find((c) => c.id === state.id)
        return { ...state, config }
      })
      .filter((card) => card.config !== undefined)
  })

  // Actions
  function toggleCard(id: string) {
    const card = cardStates.value.find((c) => c.id === id)
    if (card) {
      card.enabled = !card.enabled
      saveLayout()
    }
  }

  function toggleCollapse(id: string) {
    const card = cardStates.value.find((c) => c.id === id)
    if (card) {
      card.collapsed = !card.collapsed
      saveLayout()
    }
  }

  function reorderCards(fromIndex: number, toIndex: number) {
    const enabledList = enabledCards.value
    if (fromIndex < 0 || fromIndex >= enabledList.length) return
    if (toIndex < 0 || toIndex >= enabledList.length) return

    const movedCard = enabledList[fromIndex]
    if (!movedCard) return

    // Calculate new order values
    const newOrder = enabledList.map((card, index) => {
      if (index === fromIndex) return toIndex
      if (fromIndex < toIndex) {
        if (index > fromIndex && index <= toIndex) return index - 1
      } else {
        if (index >= toIndex && index < fromIndex) return index + 1
      }
      return index
    })

    // Update order in cardStates
    enabledList.forEach((card, index) => {
      const state = cardStates.value.find((c) => c.id === card.id)
      if (state) {
        state.order = newOrder[index] ?? index
      }
    })

    saveLayout()
  }

  function resetToDefaults() {
    cardStates.value = getDefaultCardStates()
    saveLayout()
  }

  function setCustomizing(value: boolean) {
    isCustomizing.value = value
  }

  // Initialize on store creation
  initialize()

  return {
    // State
    cardStates,
    isCustomizing,

    // Computed
    enabledCards,
    disabledCards,

    // Actions
    initialize,
    toggleCard,
    toggleCollapse,
    reorderCards,
    resetToDefaults,
    setCustomizing,
    saveLayout,
  }
})
