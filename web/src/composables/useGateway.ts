import { ref, readonly, onMounted, onUnmounted } from 'vue'
import { GatewayClient, type GatewayMessage, type GatewayEventHandler } from '@/api/gateway'
import type { WebSocketStatus } from '@/utils/websocket'

export function useGateway(autoConnect = true) {
  const client = new GatewayClient()
  const status = ref<WebSocketStatus>('disconnected')
  const error = ref<Error | null>(null)
  const lastEvent = ref<GatewayMessage | null>(null)

  // Track status changes
  const unsubscribeStatus = client.onStatusChange((newStatus) => {
    status.value = newStatus
    if (newStatus === 'error') {
      error.value = new Error('Connection error')
    } else if (newStatus === 'connected') {
      error.value = null
    }
  })

  // Connect on mount if autoConnect is true
  onMounted(() => {
    if (autoConnect) {
      client.connect()
    }
  })

  // Disconnect on unmount
  onUnmounted(() => {
    unsubscribeStatus()
    client.disconnect()
  })

  // Subscribe to events
  function on<T = unknown>(event: string, handler: GatewayEventHandler<T>): () => void {
    return client.on(event, handler)
  }

  // Subscribe to all events
  function onAny(handler: GatewayEventHandler<GatewayMessage>): () => void {
    return client.on('*', (msg) => {
      lastEvent.value = msg as GatewayMessage
      handler(msg as GatewayMessage)
    })
  }

  // Send request
  async function request<T = unknown, R = unknown>(
    method: string,
    payload?: T
  ): Promise<GatewayMessage<R>> {
    try {
      return await client.request<T, R>(method, payload)
    } catch (err) {
      error.value = err as Error
      throw err
    }
  }

  return {
    // State
    status: readonly(status),
    error: readonly(error),
    lastEvent: readonly(lastEvent),
    isConnected: () => client.isConnected,

    // Methods
    connect: () => client.connect(),
    disconnect: () => client.disconnect(),
    request,
    on,
    onAny,

    // Convenience methods
    chatSend: (conversationId: string, content: string) => client.chatSend(conversationId, content),
    chatAbort: (conversationId: string) => client.chatAbort(conversationId),
    browserRequest: (action: string, params?: Record<string, unknown>) =>
      client.browserRequest(action, params),
    hooksWake: (hookId: string, payload?: unknown) => client.hooksWake(hookId, payload),
  }
}
