import { getCurrentInstance, onUnmounted, ref } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useSettingsStore } from '@/stores/settings'
import { i18n } from '@/i18n'
import router from '@/router'
import { getStoredAccessToken } from '@/utils/authStorage'
import { consumeSSEJsonStream } from '@/utils/sseStream'

type ApiClientModule = typeof import('@/api/client')
let apiClientModulePromise: Promise<ApiClientModule> | null = null

function loadApiClientModule(): Promise<ApiClientModule> {
  if (!apiClientModulePromise) {
    apiClientModulePromise = import('@/api/client')
  }
  return apiClientModulePromise
}

type EventPayload = Record<string, unknown>
type EventCallback = (data: EventPayload) => void
const listeners = new Map<string, Set<EventCallback>>()

function toEventPayload(data: unknown): EventPayload {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return {}
  return data as EventPayload
}

function getEventString(payload: EventPayload, key: string): string | undefined {
  const value = payload[key]
  return typeof value === 'string' ? value : undefined
}

function getEventBool(payload: EventPayload, key: string): boolean {
  return payload[key] === true
}

/** Register a callback for a specific SSE event type (e.g. "skill.install.progress"). */
export function onSSEEvent(type: string, cb: EventCallback) {
  if (!listeners.has(type)) listeners.set(type, new Set())
  listeners.get(type)!.add(cb)
}

/** Unregister a previously registered callback. */
export function offSSEEvent(type: string, cb: EventCallback) {
  listeners.get(type)?.delete(cb)
}

// Debounce timer for streaming conversation_updated events
let streamingFetchTimer: ReturnType<typeof setTimeout> | null = null

/**
 * useEventStream connects to the SSE event endpoint and dispatches
 * incoming events to the appropriate stores (chat refresh, toast, desktop notification).
 */
export function useEventStream() {
  const connected = ref(false)
  let abortController: AbortController | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  const RECONNECT_DELAY = 5000
  const chatStore = useChatStore()
  const notificationStore = useNotificationStore()
  const providerPoolStore = useProviderPoolStore()
  const settingsStore = useSettingsStore()
  let providerResyncTimer: ReturnType<typeof setTimeout> | null = null
  const t = (key: string) => String(i18n.global.t(key))

  function scheduleProviderResync() {
    if (providerResyncTimer) return
    providerResyncTimer = setTimeout(() => {
      providerResyncTimer = null
      providerPoolStore
        .fetchProviders()
        .then(() => {
          const llmProviders = providerPoolStore.providers.filter((provider) => provider.type !== 'media')
          settingsStore.updateFromPoolProviders(llmProviders)
        })
        .catch(() => {})
    }, 250)
  }

  async function connect() {
    if (connected.value) return

    abortController = new AbortController()

    const token = getStoredAccessToken()
    if (!token) return // not logged in

    const headers: Record<string, string> = {
      Accept: 'text/event-stream',
      Authorization: `Bearer ${token}`,
    }

    try {
      let response = await fetch('/api/v1/events', {
        headers,
        signal: abortController.signal,
      })

      // Handle 401 — refresh token and retry once
      if (response.status === 401) {
        const { ensureFreshToken } = await loadApiClientModule()
        const newToken = await ensureFreshToken()
        if (newToken) {
          headers['Authorization'] = `Bearer ${newToken}`
          response = await fetch('/api/v1/events', {
            headers,
            signal: abortController!.signal,
          })
        }
      }

      if (!response.ok || !response.body) {
        scheduleReconnect()
        return
      }

      connected.value = true
      await consumeSSEJsonStream(response.body, {
        onMessage: (event, data) => {
          handleEvent(event, data)
        },
        onParseError: ({ error }) => {
          console.error('[EventStream] JSON parse error:', error)
        },
      })
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') return
      console.warn('[EventStream] connection error:', err)
    } finally {
      connected.value = false
      abortController = null
      scheduleReconnect()
    }
  }

  function handleEvent(type: string, data: unknown) {
    try {
      const payload = toEventPayload(data)

      // Dispatch to global listeners first
      const cbs = listeners.get(type)
      if (cbs) {
        for (const cb of cbs) {
          try {
            cb(payload)
          } catch {
            /* ignore listener errors */
          }
        }
      }

      // Special handling for ask - no i18n needed
      if (type === 'ask') {
        chatStore.setPendingQuestion(payload)
        return
      }

      switch (type) {
        case 'push': {
          const conversationId = getEventString(payload, 'conversation_id')
          const reminderTextRaw = String(getEventString(payload, 'message') || t('push.defaultMessage'))
          const reminderText = reminderTextRaw.startsWith('⏰')
            ? reminderTextRaw
            : `⏰ ${reminderTextRaw}`
          const reminderTitle = t('push.reminder')

          // Show toast notification with optional action to navigate to conversation
          const toastOpts: Parameters<typeof notificationStore.info>[2] = { duration: 10000 }
          if (conversationId) {
            toastOpts.action = {
              label: t('push.viewConversation'),
              handler: () => {
                void router.push({
                  name: 'Chat',
                  query: { conversationId },
                })
              },
            }
          }
          notificationStore.info(reminderTitle, reminderText, toastOpts)

          // Refresh conversation list so the injected message shows up
          chatStore.fetchConversations()
          if (conversationId && chatStore.currentConversationId === conversationId) {
            chatStore.fetchMessages(conversationId)
          }

          // Desktop notification if page is hidden
          if (
            document.hidden &&
            'Notification' in window &&
            Notification.permission === 'granted'
          ) {
            new Notification(reminderTitle, { body: reminderText })
          }
          break
        }

        case 'conversation_updated': {
          const conversationId = getEventString(payload, 'id')
          const isStreamingUpdate = getEventBool(payload, 'streaming')

          // If this tab is already streaming this conversation, skip — we have real-time deltas
          if (
            isStreamingUpdate &&
            chatStore.streaming &&
            !chatStore.isRecovering &&
            chatStore.currentConversationId === conversationId
          ) {
            break
          }

          // Refresh conversation list (title changes, etc.)
          chatStore.fetchConversations()

          // If the updated conversation is the current one, refresh messages
          if (conversationId && chatStore.currentConversationId === conversationId) {
            if (isStreamingUpdate) {
              // Debounce streaming updates to avoid hammering the API
              if (streamingFetchTimer) clearTimeout(streamingFetchTimer)
              streamingFetchTimer = setTimeout(() => {
                streamingFetchTimer = null
                chatStore.fetchMessages(conversationId)
              }, 500)
            } else {
              // Final update — fetch immediately
              if (streamingFetchTimer) {
                clearTimeout(streamingFetchTimer)
                streamingFetchTimer = null
              }
              chatStore.fetchMessages(conversationId)
            }
          }
          break
        }

        case 'conversation_title_updated': {
          const conversationId = getEventString(payload, 'id')
          const title = getEventString(payload, 'title')

          // Update title in-place without a full fetchConversations round-trip
          if (conversationId && title) {
            const conv = chatStore.conversations.find((c: { id: string }) => c.id === conversationId)
            if (conv) {
              conv.title = title
            }
          }
          break
        }

        case 'provider_status_changed': {
          const providerID = getEventString(payload, 'provider_id')
          const providerStatus = getEventString(payload, 'status')
          if (providerID && providerStatus) {
            providerPoolStore.updateProviderStatus(providerID, providerStatus)
            // Status-only SSE payloads can leave stale last_error/details in memory.
            // Always resync the full provider list after a status transition.
            scheduleProviderResync()
          }
          break
        }

        case 'tool_approval_request': {
          chatStore.setPendingApproval(payload)
          break
        }

        case 'exec:approval-request': {
          chatStore.setPendingExecApproval(payload)
          break
        }
      }
    } catch (_e) {
      // Silently ignore errors in event handling to prevent stream interruption
    }
  }

  function scheduleReconnect() {
    if (reconnectTimer) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, RECONNECT_DELAY)
  }

  function disconnect() {
    connected.value = false
    if (abortController) {
      abortController.abort()
      abortController = null
    }
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (providerResyncTimer) {
      clearTimeout(providerResyncTimer)
      providerResyncTimer = null
    }
  }

  // Auto-cleanup when used from a component setup function.
  if (getCurrentInstance()) {
    onUnmounted(disconnect)
  }

  return { connected, connect, disconnect }
}
