import { ref, watch, type Ref } from 'vue'

const CHAT_CARD_DISCLOSURE_CACHE_KEY = '__zima_chat_card_disclosure_state_v1__'
const CHAT_CARD_DISCLOSURE_CACHE_MAX = 500

function getDisclosureCache(): Map<string, boolean> {
  const g = globalThis as Record<string, unknown>
  const existing = g[CHAT_CARD_DISCLOSURE_CACHE_KEY]
  if (existing instanceof Map) {
    return existing as Map<string, boolean>
  }
  const cache = new Map<string, boolean>()
  g[CHAT_CARD_DISCLOSURE_CACHE_KEY] = cache
  return cache
}

function evictOldestMapEntry<K, V>(cache: Map<K, V>): void {
  const oldestKey = cache.keys().next().value
  if (oldestKey !== undefined) {
    cache.delete(oldestKey as K)
  }
}

function setCachedDisclosureState(key: string, expanded: boolean): void {
  const cache = getDisclosureCache()
  if (cache.size >= CHAT_CARD_DISCLOSURE_CACHE_MAX && !cache.has(key)) {
    evictOldestMapEntry(cache)
  }
  if (cache.has(key)) {
    cache.delete(key)
  }
  cache.set(key, expanded)
}

export function usePersistentDisclosureState(
  keyRef: Ref<string>,
  defaultExpanded = false
): {
  expanded: Ref<boolean>
  setExpanded: (value: boolean) => void
  toggleExpanded: () => void
} {
  const expanded = ref(defaultExpanded)

  watch(
    keyRef,
    (key) => {
      if (!key) {
        expanded.value = defaultExpanded
        return
      }
      expanded.value = getDisclosureCache().get(key) ?? defaultExpanded
    },
    { immediate: true }
  )

  const setExpanded = (value: boolean) => {
    expanded.value = value
    if (!keyRef.value) return
    setCachedDisclosureState(keyRef.value, value)
  }

  const toggleExpanded = () => {
    setExpanded(!expanded.value)
  }

  return {
    expanded,
    setExpanded,
    toggleExpanded,
  }
}

export function buildChatCardUiStateKey(params: {
  conversationId: string
  messageId: string
  cardType: string
  cardId?: string
  fallbackKey?: string
}): string {
  const conversationId = params.conversationId || 'unknown-conversation'
  const messageId = params.messageId || 'unknown-message'
  const cardKey = params.cardId?.trim() || params.fallbackKey?.trim() || params.cardType
  return `${conversationId}:${messageId}:${params.cardType}:${cardKey}`
}
