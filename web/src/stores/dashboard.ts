import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { DashboardCardState, DashboardLayout } from '@/components/dashboard/types'
import { cardRegistry, getDefaultCardStates } from '@/components/dashboard/cardRegistry'

const STORAGE_KEY = 'dashboard-layout'

function getStoredLayout(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

function persistLayout(value: string): void {
  try {
    localStorage.setItem(STORAGE_KEY, value)
  } catch {
    // Ignore storage failures in restricted environments.
  }
}

function createDefaultCardState(id: string): DashboardCardState | null {
  const config = cardRegistry.find((card) => card.id === id)
  if (!config) return null

  return {
    id: config.id,
    enabled: config.defaultEnabled,
    order: config.defaultOrder,
    collapsed: false,
  }
}

function normalizeCardStates(states: DashboardCardState[]): DashboardCardState[] {
  const stateMap = new Map(states.map((state) => [state.id, state]))

  return cardRegistry.map((config) => {
    const existing = stateMap.get(config.id)
    if (existing) {
      return {
        ...existing,
        order: Number.isFinite(existing.order) ? existing.order : config.defaultOrder,
        collapsed: existing.collapsed ?? false,
      }
    }

    return {
      id: config.id,
      enabled: config.defaultEnabled,
      order: config.defaultOrder,
      collapsed: false,
    }
  })
}

export const useDashboardStore = defineStore('dashboard', () => {
  // State
  const cardStates = ref<DashboardCardState[]>([])
  const isCustomizing = ref(false)

  function syncCardStates() {
    cardStates.value = normalizeCardStates(cardStates.value)
  }

  // Initialize from localStorage or defaults
  function initialize() {
    const stored = getStoredLayout()
    if (stored) {
      try {
        const layout: DashboardLayout = JSON.parse(stored)
        cardStates.value = normalizeCardStates(layout.cards)
      } catch {
        cardStates.value = getDefaultCardStates()
      }
    } else {
      cardStates.value = getDefaultCardStates()
    }
  }

  // Save to localStorage
  function saveLayout() {
    syncCardStates()
    const layout: DashboardLayout = {
      cards: cardStates.value,
      lastModified: new Date().toISOString(),
    }
    persistLayout(JSON.stringify(layout))
  }

  // Computed
  const resolvedCardStates = computed(() => normalizeCardStates(cardStates.value))

  const enabledCards = computed(() => {
    return resolvedCardStates.value
      .filter((state) => state.enabled)
      .sort((a, b) => a.order - b.order)
      .map((state) => {
        const config = cardRegistry.find((c) => c.id === state.id)
        return { ...state, config }
      })
      .filter((card) => card.config !== undefined)
  })

  const disabledCards = computed(() => {
    return resolvedCardStates.value
      .filter((state) => !state.enabled)
      .map((state) => {
        const config = cardRegistry.find((c) => c.id === state.id)
        return { ...state, config }
      })
      .filter((card) => card.config !== undefined)
  })

  // Actions
  function toggleCard(id: string) {
    syncCardStates()

    let card = cardStates.value.find((c) => c.id === id)
    if (!card) {
      const next = createDefaultCardState(id)
      if (!next) return
      cardStates.value.push(next)
      card = next
    }

    if (card) {
      card.enabled = !card.enabled
      saveLayout()
    }
  }

  function toggleCollapse(id: string) {
    syncCardStates()
    const card = cardStates.value.find((c) => c.id === id)
    if (card) {
      card.collapsed = !card.collapsed
      saveLayout()
    }
  }

  function reorderCards(fromIndex: number, toIndex: number) {
    syncCardStates()
    const enabledList = enabledCards.value
    if (fromIndex < 0 || fromIndex >= enabledList.length) return
    if (toIndex < 0 || toIndex >= enabledList.length) return

    const movedCard = enabledList[fromIndex]
    if (!movedCard) return

    // Calculate new order values
    const newOrder = enabledList.map((_card, index) => {
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
    resolvedCardStates,
    enabledCards,
    disabledCards,

    // Actions
    initialize,
    syncCardStates,
    toggleCard,
    toggleCollapse,
    reorderCards,
    resetToDefaults,
    setCustomizing,
    saveLayout,
  }
})
