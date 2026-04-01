export interface ConsumeSSEJsonStreamOptions<T = unknown> {
  /**
   * Called for each parsed JSON SSE message.
   * Return true to stop consuming the stream early.
   */
  onMessage: (event: string, data: T) => boolean | void
  /** Called when a `data: [DONE]` marker is received. */
  onDone?: () => void
  /** Called when JSON parsing fails; stream consumption continues. */
  onParseError?: (info: { event: string; raw: string; error: unknown }) => void
}

/**
 * Consume a fetch SSE response body and parse `event:` / `data:` lines.
 *
 * Assumptions (kept intentionally minimal to match current usage):
 * - Each JSON payload arrives on a single `data:` line.
 * - `event:` line applies to the next `data:` line, then resets.
 * - `data: [DONE]` indicates stream termination.
 */
export async function consumeSSEJsonStream<T = unknown>(
  body: ReadableStream<Uint8Array>,
  options: ConsumeSSEJsonStreamOptions<T>
): Promise<void> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let currentEvent = ''
  let shouldStop = false

  try {
    while (!shouldStop) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const rawLine of lines) {
        const line = rawLine.endsWith('\r') ? rawLine.slice(0, -1) : rawLine
        if (!line) continue
        if (line.startsWith(':')) continue // comment/keepalive

        if (line.startsWith('event:')) {
          currentEvent = line.slice('event:'.length).trim()
          continue
        }

        if (line.startsWith('data:')) {
          const raw = line.slice('data:'.length).trim()

          if (raw === '[DONE]') {
            options.onDone?.()
            shouldStop = true
            break
          }

          try {
            const data = JSON.parse(raw) as T
            const stop = options.onMessage(currentEvent, data)
            if (stop === true) {
              shouldStop = true
              break
            }
          } catch (error) {
            options.onParseError?.({ event: currentEvent, raw, error })
          } finally {
            currentEvent = ''
          }
        }
      }
    }
  } finally {
    if (shouldStop) {
      try {
        await reader.cancel()
      } catch {
        // ignore
      }
    }
  }
}

