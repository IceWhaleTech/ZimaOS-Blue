import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { WebSocketClient } from '@/utils/websocket'
import type { WebSocketStatus } from '@/utils/websocket'

// Use the type to avoid unused import error
const _statusType: WebSocketStatus | undefined = undefined
void _statusType

// Mock WebSocket
class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  url: string
  protocols: string | string[]

  onopen: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null

  constructor(url: string, protocols?: string | string[]) {
    this.url = url
    this.protocols = protocols || []

    // Simulate connection
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN
      this.onopen?.(new Event('open'))
    }, 0)
  }

  send = vi.fn()
  close = vi.fn((code?: number) => {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.(new CloseEvent('close', { code: code || 1000 }))
  })

  // Helper to simulate receiving a message
  simulateMessage(data: unknown) {
    this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }))
  }

  // Helper to simulate error
  simulateError() {
    this.onerror?.(new Event('error'))
  }
}

// Replace global WebSocket
const originalWebSocket = global.WebSocket
beforeEach(() => {
  // @ts-expect-error - Mock WebSocket
  global.WebSocket = MockWebSocket
})

afterEach(() => {
  global.WebSocket = originalWebSocket
  vi.clearAllTimers()
})

describe('WebSocketClient', () => {
  it('should connect to WebSocket server', async () => {
    const onStatusChange = vi.fn()
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
      onStatusChange,
    })

    client.connect()

    expect(onStatusChange).toHaveBeenCalledWith('connecting')

    // Wait for connection
    await vi.waitFor(() => {
      expect(onStatusChange).toHaveBeenCalledWith('connected')
    })

    expect(client.isConnected).toBe(true)
  })

  it('should disconnect from WebSocket server', async () => {
    const onStatusChange = vi.fn()
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
      onStatusChange,
    })

    client.connect()

    await vi.waitFor(() => {
      expect(client.isConnected).toBe(true)
    })

    client.disconnect()

    expect(onStatusChange).toHaveBeenCalledWith('disconnected')
    expect(client.isConnected).toBe(false)
  })

  it('should send messages when connected', async () => {
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
    })

    client.connect()

    await vi.waitFor(() => {
      expect(client.isConnected).toBe(true)
    })

    const result = client.send('test', { data: 'hello' })

    expect(result).toBe(true)
  })

  it('should queue messages when not connected', () => {
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
      reconnect: true,
    })

    // Don't connect, just try to send
    const result = client.send('test', { data: 'hello' })

    expect(result).toBe(false)
  })

  it('should handle message events', async () => {
    const handler = vi.fn()
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
    })

    client.on('test-event', handler)
    client.connect()

    await vi.waitFor(() => {
      expect(client.isConnected).toBe(true)
    })

    // Get the mock WebSocket instance
    // @ts-expect-error - Accessing private property for testing
    const ws = client.ws as MockWebSocket

    ws.simulateMessage({
      type: 'test-event',
      payload: { message: 'hello' },
    })

    expect(handler).toHaveBeenCalledWith({ message: 'hello' })
  })

  it('should unsubscribe from message events', async () => {
    const handler = vi.fn()
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
    })

    const unsubscribe = client.on('test-event', handler)
    client.connect()

    await vi.waitFor(() => {
      expect(client.isConnected).toBe(true)
    })

    unsubscribe()

    // @ts-expect-error - Accessing private property for testing
    const ws = client.ws as MockWebSocket

    ws.simulateMessage({
      type: 'test-event',
      payload: { message: 'hello' },
    })

    expect(handler).not.toHaveBeenCalled()
  })

  it('should handle wildcard message handlers', async () => {
    const handler = vi.fn()
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
    })

    client.on('*', handler)
    client.connect()

    await vi.waitFor(() => {
      expect(client.isConnected).toBe(true)
    })

    // @ts-expect-error - Accessing private property for testing
    const ws = client.ws as MockWebSocket

    ws.simulateMessage({
      type: 'any-event',
      payload: { data: 'test' },
    })

    expect(handler).toHaveBeenCalled()
  })

  it('should return correct connection status', () => {
    const client = new WebSocketClient({
      url: 'ws://localhost:8080',
    })

    expect(client.connectionStatus).toBe('disconnected')

    client.connect()

    expect(client.connectionStatus).toBe('connecting')
  })
})
