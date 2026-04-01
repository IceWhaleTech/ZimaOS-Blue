import { getCurrentInstance, onUnmounted, ref } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'
import { useProviderPoolStore } from '@/stores/providerPool'
import { useSettingsStore } from '@/stores/settings'
import { i18n } from '@/i18n'
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

// Global event listeners — components can subscribe to specific event types.
type EventCallback = (data: any) => void
const listeners = new Map<string, Set<EventCallback>>()

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
          const llmProviders = providerPoolStore.providers.filter((p: any) => p.type !== 'media')
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

  function handleEvent(type: string, data: any) {
    try {
      console.log('[EventStream] handleEvent called with type:', type, 'data:', data)
      // Dispatch to global listeners first
      const cbs = listeners.get(type)
      if (cbs) {
        for (const cb of cbs) {
          try {
            cb(data)
          } catch {
            /* ignore listener errors */
          }
        }
      }

      // Special handling for ask - no i18n needed
      if (type === 'ask') {
        chatStore.setPendingQuestion(data)
        return
      }

      console.log('[EventStream] Entering switch with type:', type)
      switch (type) {
        case 'push': {
          const reminderTextRaw = String(data.message || t('push.defaultMessage'))
          const reminderText = reminderTextRaw.startsWith('⏰')
            ? reminderTextRaw
            : `⏰ ${reminderTextRaw}`
          const reminderTitle = t('push.reminder')

          // Show toast notification with optional action to navigate to conversation
          const toastOpts: any = { duration: 10000 }
          if (data.conversation_id) {
            toastOpts.action = {
              label: t('push.viewConversation'),
              handler: () => {
                chatStore.selectConversation(data.conversation_id)
              },
            }
          }
          notificationStore.info(reminderTitle, reminderText, toastOpts)

          // Refresh conversation list so the injected message shows up
          chatStore.fetchConversations()
          if (data.conversation_id && chatStore.currentConversationId === data.conversation_id) {
            chatStore.fetchMessages(data.conversation_id)
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
          // If this tab is already streaming this conversation, skip — we have real-time deltas
          if (
            data.streaming &&
            chatStore.streaming &&
            !chatStore.isRecovering &&
            chatStore.currentConversationId === data.id
          ) {
            break
          }

          // Refresh conversation list (title changes, etc.)
          chatStore.fetchConversations()

          // If the updated conversation is the current one, refresh messages
          if (data.id && chatStore.currentConversationId === data.id) {
            if (data.streaming) {
              // Debounce streaming updates to avoid hammering the API
              if (streamingFetchTimer) clearTimeout(streamingFetchTimer)
              streamingFetchTimer = setTimeout(() => {
                streamingFetchTimer = null
                chatStore.fetchMessages(data.id)
              }, 500)
            } else {
              // Final update — fetch immediately
              if (streamingFetchTimer) {
                clearTimeout(streamingFetchTimer)
                streamingFetchTimer = null
              }
              chatStore.fetchMessages(data.id)
            }
          }
          break
        }

        case 'conversation_title_updated': {
          // Update title in-place without a full fetchConversations round-trip
          if (data.id && data.title) {
            const conv = chatStore.conversations.find((c: { id: string }) => c.id === data.id)
            if (conv) {
              conv.title = data.title
            }
          }
          break
        }

        case 'provider_status_changed': {
          if (data.provider_id && data.status) {
            const hasProvider = providerPoolStore.providers.some((p) => p.id === data.provider_id)
            providerPoolStore.updateProviderStatus(data.provider_id, data.status)
            if (!hasProvider || providerPoolStore.providers.length === 0) {
              scheduleProviderResync()
            }
          }
          break
        }

        case 'tool_approval_request': {
          chatStore.setPendingApproval(data)
          break
        }

        case 'exec:approval-request': {
          chatStore.setPendingExecApproval(data)
          break
        }
      }
    } catch (e) {
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
