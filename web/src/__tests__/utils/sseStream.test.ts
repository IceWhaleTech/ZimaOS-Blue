import { describe, it, expect, vi } from 'vitest'
import { consumeSSEJsonStream } from '@/utils/sseStream'

function createMockBody(chunks: string[]) {
  const encoder = new TextEncoder()
  let index = 0

  const reader = {
    read: vi.fn().mockImplementation(async () => {
      if (index >= chunks.length) {
        return { done: true, value: undefined as unknown as Uint8Array }
      }
      const chunk = chunks[index++]
      return { done: false, value: encoder.encode(chunk) }
    }),
    cancel: vi.fn().mockResolvedValue(undefined),
  }

  const body = {
    getReader: () => reader,
  } as unknown as ReadableStream<Uint8Array>

  return { body, reader }
}

describe('consumeSSEJsonStream', () => {
  it('parses event/data JSON messages and handles [DONE]', async () => {
    const { body } = createMockBody([
      'event: progress\n',
      'data: {"x":1}\n\n',
      'event: complete\n',
      'data: {"ok":true}\n\n',
      'data: [DONE]\n\n',
    ])

    const messages: Array<{ event: string; data: any }> = []
    const onDone = vi.fn()

    await consumeSSEJsonStream(body, {
      onMessage: (event, data) => {
        messages.push({ event, data })
      },
      onDone,
    })

    expect(messages).toEqual([
      { event: 'progress', data: { x: 1 } },
      { event: 'complete', data: { ok: true } },
    ])
    expect(onDone).toHaveBeenCalledTimes(1)
  })

  it('handles chunk boundaries inside lines and CRLF endings', async () => {
    const { body } = createMockBody([
      'event: progress\r\n' + 'data: {"a":',
      '1}\r\n\r\n' + 'event: next\r\n' + 'data: {"b":2}\r\n\r\n',
      'data: [DONE]\r\n\r\n',
    ])

    const messages: Array<{ event: string; data: any }> = []

    await consumeSSEJsonStream(body, {
      onMessage: (event, data) => {
        messages.push({ event, data })
      },
    })

    expect(messages).toEqual([
      { event: 'progress', data: { a: 1 } },
      { event: 'next', data: { b: 2 } },
    ])
  })

  it('continues after JSON parse errors', async () => {
    const { body } = createMockBody([
      'event: bad\n',
      'data: {bad json}\n\n',
      'event: good\n',
      'data: {"ok":1}\n\n',
      'data: [DONE]\n\n',
    ])

    const messages: Array<{ event: string; data: any }> = []
    const onParseError = vi.fn()

    await consumeSSEJsonStream(body, {
      onMessage: (event, data) => {
        messages.push({ event, data })
      },
      onParseError,
    })

    expect(onParseError).toHaveBeenCalledTimes(1)
    expect(messages).toEqual([{ event: 'good', data: { ok: 1 } }])
  })

  it('stops early when onMessage returns true', async () => {
    const { body, reader } = createMockBody([
      'event: complete\n',
      'data: {"ok":true}\n\n',
      'event: ignored\n',
      'data: {"x":1}\n\n',
    ])

    const messages: Array<{ event: string; data: any }> = []

    await consumeSSEJsonStream(body, {
      onMessage: (event, data) => {
        messages.push({ event, data })
        return true
      },
    })

    expect(messages).toEqual([{ event: 'complete', data: { ok: true } }])
    expect(reader.cancel).toHaveBeenCalledTimes(1)
  })
})

