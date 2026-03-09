import api from './client'
import { WebSocketClient, type WebSocketStatus } from '@/utils/websocket'

// Types
export type MessageType = 'request' | 'response' | 'event' | 'error'

export interface GatewayMessage<T = unknown> {
  id: string
  type: MessageType
  method?: string
  payload?: T
  error?: ErrorPayload
  timestamp: number
}

export interface ErrorPayload {
  code: number
  message: string
}

export interface GatewayStatus {
  total_connections: number
  active_connections: number
  active_messages: number
}

export interface GatewayConnection {
  id: string
  user_id: string
  connected_at: string
  messages_sent: number
  messages_received: number
  metadata: Record<string, unknown>
}

// Gateway REST API
export const gatewayApi = {
  getStatus: () => api.get<GatewayStatus>('/gateway/status'),

  listConnections: () => api.get<GatewayConnection[]>('/gateway/connections'),

  getConnection: (id: string) => api.get<GatewayConnection>(`/gateway/connections/${id}`),

  closeConnection: (id: string) => api.delete(`/gateway/connections/${id}`),
}

// Gateway WebSocket Client
export type GatewayEventHandler<T = unknown> = (payload: T) => void
export type GatewayResponseHandler<T = unknown> = (response: GatewayMessage<T>) => void

interface PendingRequest {
  resolve: (response: GatewayMessage) => void
  reject: (error: Error) => void
  timeout: ReturnType<typeof setTimeout>
}

export class GatewayClient {
  private ws: WebSocketClient | null = null
  private requestId = 0
  private pendingRequests: Map<string, PendingRequest> = new Map()
  private eventHandlers: Map<string, Set<GatewayEventHandler>> = new Map()
  private statusChangeHandlers: Set<(status: WebSocketStatus) => void> = new Set()
  private messageQueue: GatewayMessage[] = []
  private maxQueueSize = 100
  private requestTimeout = 30000

  constructor(private baseUrl?: string) {}

  get isConnected(): boolean {
    return this.ws?.isConnected ?? false
  }

  get connectionStatus(): WebSocketStatus {
    return this.ws?.connectionStatus ?? 'disconnected'
  }

  connect(): void {
    if (this.ws) {
      return
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = this.baseUrl || window.location.host
    const token = localStorage.getItem('token')
    const url = `${protocol}//${host}/ws${token ? `?token=${token}` : ''}`

    this.ws = new WebSocketClient({
      url,
      reconnect: true,
      reconnectInterval: 3000,
      maxReconnectAttempts: 10,
      heartbeatInterval: 30000,
      heartbeatMessage: JSON.stringify({ type: 'ping' }),
      onStatusChange: (status) => {
        this.statusChangeHandlers.forEach((handler) => handler(status))
      },
      onMessage: (event) => {
        this.handleMessage(event)
      },
      onOpen: () => {
        this.flushMessageQueue()
      },
    })

    this.ws.connect()
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.disconnect()
      this.ws = null
    }

    // Reject all pending requests
    this.pendingRequests.forEach((pending) => {
      clearTimeout(pending.timeout)
      pending.reject(new Error('Connection closed'))
    })
    this.pendingRequests.clear()
  }

  // Send a request and wait for response
  async request<T = unknown, R = unknown>(method: string, payload?: T): Promise<GatewayMessage<R>> {
    const id = this.generateRequestId()
    const message: GatewayMessage<T> = {
      id,
      type: 'request',
      method,
      payload,
      timestamp: Date.now(),
    }

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pendingRequests.delete(id)
        reject(new Error(`Request timeout: ${method}`))
      }, this.requestTimeout)

      this.pendingRequests.set(id, {
        resolve: resolve as (response: GatewayMessage) => void,
        reject,
        timeout,
      })

      if (!this.send(message)) {
        this.pendingRequests.delete(id)
        clearTimeout(timeout)
        reject(new Error('Failed to send request'))
      }
    })
  }

  // Send a message without waiting for response
  send<T = unknown>(message: GatewayMessage<T>): boolean {
    if (this.ws?.isConnected) {
      return this.ws.sendRaw(JSON.stringify(message))
    }

    // Queue message for later
    if (this.messageQueue.length < this.maxQueueSize) {
      this.messageQueue.push(message as GatewayMessage)
      return true
    }

    return false
  }

  // Subscribe to events
  on<T = unknown>(event: string, handler: GatewayEventHandler<T>): () => void {
    if (!this.eventHandlers.has(event)) {
      this.eventHandlers.set(event, new Set())
    }
    this.eventHandlers.get(event)!.add(handler as GatewayEventHandler)

    return () => {
      const handlers = this.eventHandlers.get(event)
      if (handlers) {
        handlers.delete(handler as GatewayEventHandler)
        if (handlers.size === 0) {
          this.eventHandlers.delete(event)
        }
      }
    }
  }

  // Subscribe to status changes
  onStatusChange(handler: (status: WebSocketStatus) => void): () => void {
    this.statusChangeHandlers.add(handler)
    return () => {
      this.statusChangeHandlers.delete(handler)
    }
  }

  // Convenience methods for common operations
  async chatSend(conversationId: string, content: string): Promise<GatewayMessage> {
    return this.request('chat.send', { conversation_id: conversationId, content })
  }

  async chatAbort(conversationId: string): Promise<GatewayMessage> {
    return this.request('chat.abort', { conversation_id: conversationId })
  }

  async browserRequest(action: string, params?: Record<string, unknown>): Promise<GatewayMessage> {
    return this.request('browser.request', { action, params })
  }

  async hooksWake(hookId: string, payload?: unknown): Promise<GatewayMessage> {
    return this.request('hooks.wake', { hook_id: hookId, payload })
  }

  private handleMessage(event: MessageEvent): void {
    try {
      const message = JSON.parse(event.data) as GatewayMessage

      // Handle pong
      if (message.type === 'response' && !message.method) {
        // This might be a pong or a response to a request
      }

      // Handle response to pending request
      if (message.type === 'response' || message.type === 'error') {
        const pending = this.pendingRequests.get(message.id)
        if (pending) {
          clearTimeout(pending.timeout)
          this.pendingRequests.delete(message.id)

          if (message.type === 'error' && message.error) {
            pending.reject(new Error(message.error.message))
          } else {
            pending.resolve(message)
          }
          return
        }
      }

      // Handle events
      if (message.type === 'event' && message.method) {
        const handlers = this.eventHandlers.get(message.method)
        if (handlers) {
          handlers.forEach((handler) => {
            try {
              handler(message.payload)
            } catch (error) {
              console.error(`Error in event handler for ${message.method}:`, error)
            }
          })
        }

        // Also dispatch to wildcard handlers
        const wildcardHandlers = this.eventHandlers.get('*')
        if (wildcardHandlers) {
          wildcardHandlers.forEach((handler) => {
            try {
              handler(message)
            } catch (error) {
              console.error('Error in wildcard event handler:', error)
            }
          })
        }
      }
    } catch {
      // Not JSON, ignore
    }
  }

  private generateRequestId(): string {
    return `req_${Date.now()}_${++this.requestId}`
  }

  private flushMessageQueue(): void {
    while (this.messageQueue.length > 0 && this.ws?.isConnected) {
      const message = this.messageQueue.shift()!
      this.ws.sendRaw(JSON.stringify(message))
    }
  }
}

// Singleton instance
let gatewayClient: GatewayClient | null = null

export function getGatewayClient(): GatewayClient {
  if (!gatewayClient) {
    gatewayClient = new GatewayClient()
  }
  return gatewayClient
}

export function resetGatewayClient(): void {
  if (gatewayClient) {
    gatewayClient.disconnect()
    gatewayClient = null
  }
}
