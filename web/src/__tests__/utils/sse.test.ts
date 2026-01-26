import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SSEClient } from '@/utils/sse'

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
  })
})
