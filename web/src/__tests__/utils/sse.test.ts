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

    it('should trigger PROVIDER_NO_RESPONSE when stream closes with [DONE] but without data', async () => {
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

      // Should call onError with PROVIDER_NO_RESPONSE
      expect(onError).toHaveBeenCalledWith(new Error('PROVIDER_NO_RESPONSE'))
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

      // Should call onMessage with normalized defaults
      expect(onMessage).toHaveBeenCalledWith({ delta: 'Hello', done: false })
      // Should call onComplete (not onError) when data was received
      expect(onComplete).toHaveBeenCalled()
      expect(onError).not.toHaveBeenCalled()
    })

    it('should trigger PROVIDER_RETURNED_EMPTY when stream closes with done:true but no delta', async () => {
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

      // Should call onError with PROVIDER_RETURNED_EMPTY when done:true but no actual content
      expect(onError).toHaveBeenCalledWith(new Error('PROVIDER_RETURNED_EMPTY'))
      expect(onComplete).not.toHaveBeenCalled()
    })

    it('should trigger STREAM_EMPTY when stream closes abruptly without [DONE]', async () => {
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

      // Should call onError with STREAM_EMPTY when stream closes without any data
      expect(onError).toHaveBeenCalledWith(new Error('STREAM_EMPTY'))
      expect(onComplete).not.toHaveBeenCalled()
      expect(onMessage).not.toHaveBeenCalled()
    })

    it('should fire onComplete exactly once on [DONE], not on done:true chunk', async () => {
      // Simulates the real server flow:
      //   1. delta chunks with content
      //   2. done:true chunk with provider/model/stats metadata
      //   3. [DONE] marker (sent AFTER server persists to DB)
      // onComplete must fire only on step 3 so fetchMessages sees persisted data.
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"delta":"Hello "}\n\n'),
            })
          }
          if (callCount === 2) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"delta":"world"}\n\n'),
            })
          }
          if (callCount === 3) {
            // done:true with metadata — server has NOT persisted yet
            return Promise.resolve({
              done: false,
              value: encoder.encode(
                'data: {"delta":"","done":true,"provider":"openai","model":"gpt-4o","stats":{"input_tokens":10,"output_tokens":5}}\n\n'
              ),
            })
          }
          if (callCount === 4) {
            // [DONE] — server has persisted the message
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
        { message: 'test', provider: 'openai', model: 'gpt-4o' },
        { onMessage, onError, onComplete }
      )

      // onComplete fires exactly once
      expect(onComplete).toHaveBeenCalledTimes(1)
      // It carries the metadata from the done:true chunk
      expect(onComplete).toHaveBeenCalledWith(
        expect.objectContaining({
          done: true,
          provider: 'openai',
          model: 'gpt-4o',
          stats: expect.objectContaining({ input_tokens: 10, output_tokens: 5 }),
        })
      )
      expect(onError).not.toHaveBeenCalled()
      // All 3 chunks were delivered to onMessage (2 deltas + 1 done:true)
      expect(onMessage).toHaveBeenCalledTimes(3)
    })

    it('should fire onComplete with metadata when stream closes after done:true but before [DONE]', async () => {
      // Edge case: connection drops after done:true but before [DONE]
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"delta":"Hi"}\n\n'),
            })
          }
          if (callCount === 2) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"delta":"","done":true,"provider":"anthropic","model":"claude"}\n\n'),
            })
          }
          // Stream closes without [DONE]
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
        { message: 'test', provider: 'anthropic', model: 'claude' },
        { onMessage, onError, onComplete }
      )

      // Should still fire onComplete with the saved metadata
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onComplete).toHaveBeenCalledWith(
        expect.objectContaining({ done: true, provider: 'anthropic', model: 'claude' })
      )
      expect(onError).not.toHaveBeenCalled()
    })

    it('smoke: should strip ask_gate marker and set awaiting_user_input', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode(
                'data: {"delta":"Before <ask_gate>Choose strategy A/B/C</ask_gate> after"}\n\n'
              ),
            })
          }
          if (callCount === 2) {
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

      expect(onError).not.toHaveBeenCalled()
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onMessage).toHaveBeenCalledTimes(1)

      const chunk = onMessage.mock.calls[0][0]
      expect(chunk).toEqual(
        expect.objectContaining({
          done: false,
          awaiting_user_input: true,
        })
      )
      expect(chunk.delta).toContain('Before')
      expect(chunk.delta).toContain('after')
      expect(chunk.delta).not.toContain('<ask_gate>')
      expect(chunk.delta).not.toContain('</ask_gate>')
    })

    it('should ignore stale chunks with mismatched stream_id when no injection switch is announced', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","delta":"A"}\n\n'),
            })
          }
          if (callCount === 2) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s2","delta":"SHOULD_IGNORE"}\n\n'),
            })
          }
          if (callCount === 3) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","delta":"B"}\n\n'),
            })
          }
          if (callCount === 4) {
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
      const onComplete = vi.fn()
      const onError = vi.fn()
      const onStreamId = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onComplete, onError, onStreamId }
      )

      expect(onError).not.toHaveBeenCalled()
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onMessage).toHaveBeenCalledTimes(2)
      expect(onMessage.mock.calls[0][0]).toEqual(expect.objectContaining({ delta: 'A', stream_id: 's1' }))
      expect(onMessage.mock.calls[1][0]).toEqual(expect.objectContaining({ delta: 'B', stream_id: 's1' }))
      expect(onStreamId).toHaveBeenCalledTimes(1)
      expect(onStreamId).toHaveBeenCalledWith('s1')
    })

    it('should allow one stream_id switch after injection event', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","delta":"A"}\n\n'),
            })
          }
          if (callCount === 2) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","injection":true,"user_message":"继续"}\n\n'),
            })
          }
          if (callCount === 3) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s2","delta":"B"}\n\n'),
            })
          }
          if (callCount === 4) {
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
      const onInjection = vi.fn()
      const onComplete = vi.fn()
      const onError = vi.fn()
      const onStreamId = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onInjection, onComplete, onError, onStreamId }
      )

      expect(onError).not.toHaveBeenCalled()
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onInjection).toHaveBeenCalledTimes(1)
      expect(onMessage).toHaveBeenCalledTimes(2)
      expect(onMessage.mock.calls[0][0]).toEqual(expect.objectContaining({ delta: 'A', stream_id: 's1' }))
      expect(onMessage.mock.calls[1][0]).toEqual(expect.objectContaining({ delta: 'B', stream_id: 's2' }))
      expect(onStreamId).toHaveBeenCalledTimes(2)
      expect(onStreamId.mock.calls[0][0]).toBe('s1')
      expect(onStreamId.mock.calls[1][0]).toBe('s2')
    })

    it('should ignore duplicate or out-of-order chunks by seq within one stream_id', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":1,"delta":"A"}\n\n'),
            })
          }
          if (callCount === 2) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":3,"delta":"C"}\n\n'),
            })
          }
          if (callCount === 3) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":2,"delta":"SHOULD_IGNORE"}\n\n'),
            })
          }
          if (callCount === 4) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":3,"delta":"SHOULD_IGNORE_DUP"}\n\n'),
            })
          }
          if (callCount === 5) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":4,"delta":"D"}\n\n'),
            })
          }
          if (callCount === 6) {
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
      const onComplete = vi.fn()
      const onError = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onComplete, onError }
      )

      expect(onError).not.toHaveBeenCalled()
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onMessage).toHaveBeenCalledTimes(3)
      expect(onMessage.mock.calls[0][0]).toEqual(expect.objectContaining({ delta: 'A', seq: 1 }))
      expect(onMessage.mock.calls[1][0]).toEqual(expect.objectContaining({ delta: 'C', seq: 3 }))
      expect(onMessage.mock.calls[2][0]).toEqual(expect.objectContaining({ delta: 'D', seq: 4 }))
    })

    it('should reset seq ordering after injection-triggered stream_id switch', async () => {
      const encoder = new TextEncoder()
      let callCount = 0
      const mockReader = {
        read: vi.fn().mockImplementation(() => {
          callCount++
          if (callCount === 1) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":1,"delta":"A"}\n\n'),
            })
          }
          if (callCount === 2) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s1","seq":2,"injection":true,"user_message":"继续"}\n\n'),
            })
          }
          if (callCount === 3) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s2","seq":1,"delta":"B"}\n\n'),
            })
          }
          if (callCount === 4) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s2","seq":1,"delta":"SHOULD_IGNORE_DUP"}\n\n'),
            })
          }
          if (callCount === 5) {
            return Promise.resolve({
              done: false,
              value: encoder.encode('data: {"stream_id":"s2","seq":2,"delta":"C"}\n\n'),
            })
          }
          if (callCount === 6) {
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
      const onInjection = vi.fn()
      const onComplete = vi.fn()
      const onError = vi.fn()
      const onStreamId = vi.fn()

      await client.connect(
        'conv-1',
        { message: 'test', provider: 'openai', model: 'gpt-4o-mini' },
        { onMessage, onInjection, onComplete, onError, onStreamId }
      )

      expect(onError).not.toHaveBeenCalled()
      expect(onComplete).toHaveBeenCalledTimes(1)
      expect(onInjection).toHaveBeenCalledTimes(1)
      expect(onMessage).toHaveBeenCalledTimes(3)
      expect(onMessage.mock.calls[0][0]).toEqual(expect.objectContaining({ delta: 'A', stream_id: 's1', seq: 1 }))
      expect(onMessage.mock.calls[1][0]).toEqual(expect.objectContaining({ delta: 'B', stream_id: 's2', seq: 1 }))
      expect(onMessage.mock.calls[2][0]).toEqual(expect.objectContaining({ delta: 'C', stream_id: 's2', seq: 2 }))
      expect(onStreamId).toHaveBeenCalledTimes(2)
      expect(onStreamId.mock.calls[0][0]).toBe('s1')
      expect(onStreamId.mock.calls[1][0]).toBe('s2')
    })
  })
})
