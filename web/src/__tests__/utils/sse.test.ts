import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SSEClient } from '@/utils/sse'

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn().mockReturnValue(null),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}
Object.defineProperty(global, 'localStorage', { value: localStorageMock })

describe('SSE Client', () => {
  let client: SSEClient

  beforeEach(() => {
    client = new SSEClient()
    vi.clearAllMocks()
  })

  describe('constructor', () => {
    it('should create a new SSE client', () => {
      expect(client).toBeInstanceOf(SSEClient)
      expect(client.connected).toBe(false)
    })
  })

  describe('disconnect', () => {
    it('should set connected to false', () => {
      client.disconnect()
      expect(client.connected).toBe(false)
    })
  })

  describe('connect', () => {
    it('should disconnect existing connection before connecting', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        body: {
          getReader: () => ({
            read: vi.fn().mockResolvedValue({ done: true, value: undefined }),
          }),
        },
      })
      global.fetch = mockFetch

      const onMessage = vi.fn()
      const onComplete = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onComplete }
      )

      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/conversations/conv-1/messages/stream',
        expect.objectContaining({
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            Accept: 'text/event-stream',
          },
        })
      )
    })

    it('should handle HTTP errors', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      })
      global.fetch = mockFetch

      const onError = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage: vi.fn(), onError }
      )

      expect(onError).toHaveBeenCalledWith(expect.any(Error))
    })

    it('should handle missing response body', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        body: null,
      })
      global.fetch = mockFetch

      const onError = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage: vi.fn(), onError }
      )

      expect(onError).toHaveBeenCalledWith(expect.any(Error))
    })

    it('should trigger NO_STREAM_DATA error when stream closes without data', async () => {
      // Simulate SSE stream that sends [DONE] without any actual data
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            // Return [DONE] without any content data
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: [DONE]\n\n'),
            })
          }
          return Promise.resolve({ done: true, value: undefined })
        }),
      }

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        body: { getReader: () => mockReader },
      })
      global.fetch = mockFetch

      const onMessage = vi.fn()
      const onError = vi.fn()
      const onComplete = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onError, onComplete }
      )

      // Should call onError with NO_STREAM_DATA
      expect(onError).toHaveBeenCalledWith(new Error('NO_STREAM_DATA'))
      // Should NOT call onComplete when no data received
      expect(onComplete).not.toHaveBeenCalled()
    })

    it('should call onComplete when stream has actual data before [DONE]', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            // Return actual content data
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"delta":"Hello"}\n\n'),
            })
          }
          if (callCount === 2) {
            // Return [DONE]
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: [DONE]\n\n'),
            })
          }
          return Promise.resolve({ done: true, value: undefined })
        }),
      }

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        body: { getReader: () => mockReader },
      })
      global.fetch = mockFetch

      const onMessage = vi.fn()
      const onError = vi.fn()
      const onComplete = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onError, onComplete }
      )

      // Should call onMessage with the data
      expect(onMessage).toHaveBeenCalledWith({ delta: 'Hello' })
      // Should call onComplete (not onError) when data was received
      expect(onComplete).toHaveBeenCalled()
      expect(onError).not.toHaveBeenCalled()
    })

    it('should trigger NO_STREAM_DATA error when stream closes with done:true but no delta', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            // Return done:true chunk without any delta content
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"done":true}\n\n'),
            })
          }
          return Promise.resolve({ done: true, value: undefined })
        }),
      }

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        body: { getReader: () => mockReader },
      })
      global.fetch = mockFetch

      const onMessage = vi.fn()
      const onError = vi.fn()
      const onComplete = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onError, onComplete }
      )

      // Should call onError with NO_STREAM_DATA when done:true but no actual content
      expect(onError).toHaveBeenCalledWith(new Error('NO_STREAM_DATA'))
      expect(onComplete).not.toHaveBeenCalled()
    })

    it('should trigger NO_STREAM_DATA error when stream closes abruptly without [DONE]', async () => {
      // Simulate SSE stream that closes immediately without any data or [DONE]
      const mockReader = {
        read: vi.fn().mockResolvedValue({ done: true, value: undefined }),
      }

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        body: { getReader: () => mockReader },
      })
      global.fetch = mockFetch

      const onMessage = vi.fn()
      const onError = vi.fn()
      const onComplete = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onError, onComplete }
      )

      // Should call onError with NO_STREAM_DATA when stream closes without any data
      expect(onError).toHaveBeenCalledWith(new Error('NO_STREAM_DATA'))
      expect(onComplete).not.toHaveBeenCalled()
      expect(onMessage).not.toHaveBeenCalled()
    })
  })
})
