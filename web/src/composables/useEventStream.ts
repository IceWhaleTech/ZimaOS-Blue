import { ref, onUnmounted } from 'vue'
import { useChatStore } from '@/stores/chat'
import { useNotificationStore } from '@/stores/notification'
import { ensureFreshToken } from '@/api/client'

/**
 * useEventStream connects to the SSE event endpoint and dispatches
 * incoming events to the appropriate stores (chat refresh, toast, desktop notification).
 */
export function useEventStream() {
  const connected = ref(false)
  let abortController: AbortController | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  const RECONNECT_DELAY = 5000

  async function connect() {
    if (connected.value) return

    abortController = new AbortController()

    const token = localStorage.getItem('token')
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
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (connected.value) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        let currentEventType = ''
        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEventType = line.slice(7).trim()
          } else if (line.startsWith('data: ')) {
            const raw = line.slice(6).trim()
            try {
              const data = JSON.parse(raw)
              handleEvent(currentEventType, data)
            } catch {
              // ignore non-JSON
            }
            currentEventType = ''
          }
          // ignore comments (lines starting with ':')
        }
      }
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
    const chatStore = useChatStore()
    const notificationStore = useNotificationStore()

    switch (type) {
      case 'reminder': {
        // Show toast notification
        notificationStore.info(
          data.message || 'Reminder',
          undefined,
          { duration: 10000 },
        )

        // Desktop notification if page is hidden
        if (document.hidden && 'Notification' in window && Notification.permission === 'granted') {
          new Notification('Reminder', { body: data.message })
        }
        break
      }

      case 'conversation_updated': {
        // Refresh conversation list
        chatStore.fetchConversations()

        // If the updated conversation is the current one, refresh messages
        if (data.id && chatStore.currentConversationId === data.id) {
          chatStore.fetchMessages(data.id)
        }
        break
      }
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
  }

  // Auto-cleanup on component unmount
  onUnmounted(disconnect)

  return { connected, connect, disconnect }
}
