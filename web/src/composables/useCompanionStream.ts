import { ref, readonly, onMounted, onUnmounted } from 'vue'
import { WebSocketClient, type WebSocketStatus } from '@/utils/websocket'
import { getCompanionStreamUrl, type SessionEvent } from '@/api/companion'
import { useCompanionStore } from '@/stores/companion'

export interface UseCompanionStreamOptions {
  sessionId?: string
  autoConnect?: boolean
  onEvent?: (event: SessionEvent) => void
  onError?: (error: Error) => void
  onStatusChange?: (status: WebSocketStatus) => void
}

export function useCompanionStream(options: UseCompanionStreamOptions = {}) {
  const { sessionId, autoConnect = true, onEvent, onError, onStatusChange } = options

  const status = ref<WebSocketStatus>('disconnected')
  const error = ref<Error | null>(null)
  const lastEvent = ref<SessionEvent | null>(null)
  const eventCount = ref(0)

  const companionStore = useCompanionStore()

  let client: WebSocketClient | null = null

  function createClient() {
    const url = getCompanionStreamUrl(sessionId)

    client = new WebSocketClient({
      url,
      reconnect: true,
      reconnectInterval: 3000,
      maxReconnectAttempts: 10,
      heartbeatInterval: 30000,
      onStatusChange: (newStatus) => {
        status.value = newStatus
        companionStore.setStreaming(newStatus === 'connected')
        onStatusChange?.(newStatus)

        if (newStatus === 'error') {
          error.value = new Error('WebSocket connection error')
          onError?.(error.value)
        } else if (newStatus === 'connected') {
          error.value = null
        }
      },
      onMessage: (event) => {
        try {
          const data = JSON.parse(event.data) as SessionEvent
          lastEvent.value = data
          eventCount.value++

          // Add to store for global access
          companionStore.addRealtimeEvent(data)

          // Call custom handler
          onEvent?.(data)
        } catch {
          // Ignore non-JSON messages (like pong)
        }
      },
      onError: (event) => {
        console.error('Companion WebSocket error:', event)
        error.value = new Error('WebSocket error')
        onError?.(error.value)
      },
    })
  }

  function connect() {
    if (!client) {
      createClient()
    }
    client?.connect()
  }

  function disconnect() {
    client?.disconnect()
    companionStore.setStreaming(false)
  }

  function reconnect() {
    disconnect()
    createClient()
    connect()
  }

  // Change session filter
  function setSessionFilter(newSessionId?: string) {
    if (newSessionId !== sessionId) {
      disconnect()
      options.sessionId = newSessionId
      createClient()
      connect()
    }
  }

  onMounted(() => {
    if (autoConnect) {
      connect()
    }
  })

  onUnmounted(() => {
    disconnect()
    client = null
  })

  return {
    // State
    status: readonly(status),
    error: readonly(error),
    lastEvent: readonly(lastEvent),
    eventCount: readonly(eventCount),

    // Computed
    isConnected: () => status.value === 'connected',
    isReconnecting: () => status.value === 'reconnecting',

    // Methods
    connect,
    disconnect,
    reconnect,
    setSessionFilter,
  }
}
