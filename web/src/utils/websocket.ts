// WebSocket client with automatic reconnection and event handling

export type WebSocketStatus = 'connecting' | 'connected' | 'disconnected' | 'reconnecting' | 'error'

export interface WebSocketConfig {
  url: string
  protocols?: string | string[]
  reconnect?: boolean
  reconnectInterval?: number
  maxReconnectAttempts?: number
  heartbeatInterval?: number
  heartbeatMessage?: string
  onOpen?: (event: Event) => void
  onClose?: (event: CloseEvent) => void
  onError?: (event: Event) => void
  onMessage?: (event: MessageEvent) => void
  onStatusChange?: (status: WebSocketStatus) => void
}

export interface WebSocketMessage<T = unknown> {
  type: string
  payload: T
  timestamp?: number
}

export class WebSocketClient {
  private ws: WebSocket | null = null
  private config: Required<WebSocketConfig>
  private reconnectAttempts = 0
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private status: WebSocketStatus = 'disconnected'
  private messageHandlers: Map<string, Set<(payload: unknown) => void>> = new Map()
  private pendingMessages: WebSocketMessage[] = []

  constructor(config: WebSocketConfig) {
    this.config = {
      protocols: [],
      reconnect: true,
      reconnectInterval: 3000,
      maxReconnectAttempts: 10,
      heartbeatInterval: 30000,
      heartbeatMessage: JSON.stringify({ type: 'ping' }),
      onOpen: () => {},
      onClose: () => {},
      onError: () => {},
      onMessage: () => {},
      onStatusChange: () => {},
      ...config,
    }
  }

  get connectionStatus(): WebSocketStatus {
    return this.status
  }

  get isConnected(): boolean {
    return this.status === 'connected'
  }

  connect(): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return
    }

    this.setStatus('connecting')

    try {
      this.ws = new WebSocket(this.config.url, this.config.protocols)
      this.setupEventHandlers()
    } catch (error) {
      console.error('WebSocket connection error:', error)
      this.setStatus('error')
      this.scheduleReconnect()
    }
  }

  disconnect(): void {
    this.stopHeartbeat()
    this.clearReconnectTimer()
    this.reconnectAttempts = 0

    if (this.ws) {
      this.ws.close(1000, 'Client disconnect')
      this.ws = null
    }

    this.setStatus('disconnected')
  }

  send<T>(type: string, payload: T): boolean {
    const message: WebSocketMessage<T> = {
      type,
      payload,
      timestamp: Date.now(),
    }

    if (this.ws?.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(JSON.stringify(message))
        return true
      } catch (error) {
        console.error('WebSocket send error:', error)
        return false
      }
    }

    // Queue message for later if not connected
    if (this.config.reconnect) {
      this.pendingMessages.push(message as WebSocketMessage)
    }

    return false
  }

  sendRaw(data: string | ArrayBuffer | Blob): boolean {
    if (this.ws?.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(data)
        return true
      } catch (error) {
        console.error('WebSocket send error:', error)
        return false
      }
    }
    return false
  }

  on<T>(type: string, handler: (payload: T) => void): () => void {
    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, new Set())
    }

    const handlers = this.messageHandlers.get(type)!
    handlers.add(handler as (payload: unknown) => void)

    // Return unsubscribe function
    return () => {
      handlers.delete(handler as (payload: unknown) => void)
      if (handlers.size === 0) {
        this.messageHandlers.delete(type)
      }
    }
  }

  off(type: string, handler?: (payload: unknown) => void): void {
    if (handler) {
      const handlers = this.messageHandlers.get(type)
      if (handlers) {
        handlers.delete(handler)
        if (handlers.size === 0) {
          this.messageHandlers.delete(type)
        }
      }
    } else {
      this.messageHandlers.delete(type)
    }
  }

  private setupEventHandlers(): void {
    if (!this.ws) return

    this.ws.onopen = (event) => {
      this.setStatus('connected')
      this.reconnectAttempts = 0
      this.startHeartbeat()
      this.flushPendingMessages()
      this.config.onOpen(event)
    }

    this.ws.onclose = (event) => {
      this.stopHeartbeat()
      this.config.onClose(event)

      if (event.code !== 1000 && this.config.reconnect) {
        this.scheduleReconnect()
      } else {
        this.setStatus('disconnected')
      }
    }

    this.ws.onerror = (event) => {
      console.error('WebSocket error:', event)
      this.setStatus('error')
      this.config.onError(event)
    }

    this.ws.onmessage = (event) => {
      this.config.onMessage(event)
      this.handleMessage(event)
    }
  }

  private handleMessage(event: MessageEvent): void {
    try {
      const data = JSON.parse(event.data) as WebSocketMessage

      // Handle pong response
      if (data.type === 'pong') {
        return
      }

      // Dispatch to registered handlers
      const handlers = this.messageHandlers.get(data.type)
      if (handlers) {
        handlers.forEach((handler) => {
          try {
            handler(data.payload)
          } catch (error) {
            console.error(`Error in message handler for type "${data.type}":`, error)
          }
        })
      }

      // Also dispatch to wildcard handlers
      const wildcardHandlers = this.messageHandlers.get('*')
      if (wildcardHandlers) {
        wildcardHandlers.forEach((handler) => {
          try {
            handler(data)
          } catch (error) {
            console.error('Error in wildcard message handler:', error)
          }
        })
      }
    } catch {
      // Not JSON, ignore or handle as raw message
    }
  }

  private setStatus(status: WebSocketStatus): void {
    if (this.status !== status) {
      this.status = status
      this.config.onStatusChange(status)
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts) {
      console.error('Max reconnect attempts reached')
      this.setStatus('error')
      return
    }

    this.setStatus('reconnecting')
    this.reconnectAttempts++

    const delay = this.config.reconnectInterval * Math.min(this.reconnectAttempts, 5)

    this.reconnectTimer = setTimeout(() => {
      this.connect()
    }, delay)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private startHeartbeat(): void {
    if (this.config.heartbeatInterval <= 0) return

    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(this.config.heartbeatMessage)
      }
    }, this.config.heartbeatInterval)
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private flushPendingMessages(): void {
    while (this.pendingMessages.length > 0) {
      const message = this.pendingMessages.shift()!
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(message))
      }
    }
  }
}

// Composable for Vue components
import { ref, onMounted, onUnmounted, readonly } from 'vue'

export function useWebSocket(config: WebSocketConfig) {
  const status = ref<WebSocketStatus>('disconnected')
  const lastMessage = ref<WebSocketMessage | null>(null)
  const error = ref<Event | null>(null)

  const client = new WebSocketClient({
    ...config,
    onStatusChange: (newStatus) => {
      status.value = newStatus
      config.onStatusChange?.(newStatus)
    },
    onMessage: (event) => {
      try {
        lastMessage.value = JSON.parse(event.data)
      } catch {
        // Not JSON
      }
      config.onMessage?.(event)
    },
    onError: (event) => {
      error.value = event
      config.onError?.(event)
    },
  })

  onMounted(() => {
    client.connect()
  })

  onUnmounted(() => {
    client.disconnect()
  })

  return {
    status: readonly(status),
    lastMessage: readonly(lastMessage),
    error: readonly(error),
    isConnected: () => client.isConnected,
    send: client.send.bind(client),
    sendRaw: client.sendRaw.bind(client),
    on: client.on.bind(client),
    off: client.off.bind(client),
    connect: client.connect.bind(client),
    disconnect: client.disconnect.bind(client),
  }
}
