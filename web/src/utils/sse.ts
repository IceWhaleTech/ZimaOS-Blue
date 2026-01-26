import type { StreamChunk, SendMessageRequest } from '@/api/chat'

export interface SSEClientOptions {
  onMessage: (chunk: StreamChunk) => void
  onError?: (error: Error) => void
  onComplete?: () => void
}

export class SSEClient {
  private abortController: AbortController | null = null
  private isConnected = false

  async connect(
    conversationId: string,
    request: SendMessageRequest,
    options: SSEClientOptions
  ): Promise<void> {
    if (this.isConnected) {
      this.disconnect()
    }

    this.abortController = new AbortController()
    this.isConnected = true

    const url = `/api/v1/conversations/${conversationId}/messages/stream`

    try {
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream',
        },
        body: JSON.stringify(request),
        signal: this.abortController.signal,
      })

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const reader = response.body?.getReader()
      if (!reader) {
        throw new Error('No response body')
      }

      const decoder = new TextDecoder()
      let buffer = ''

      while (this.isConnected) {
        const { done, value } = await reader.read()

        if (done) {
          break
        }

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6).trim()

            if (data === '[DONE]') {
              options.onComplete?.()
              this.isConnected = false
              break
            }

            try {
              const chunk: StreamChunk = JSON.parse(data)
              options.onMessage(chunk)

              if (chunk.done) {
                options.onComplete?.()
                this.isConnected = false
                break
              }
            } catch {
              // Ignore parse errors for non-JSON data
            }
          }
        }
      }
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        // Connection was intentionally aborted
        return
      }
      options.onError?.(error instanceof Error ? error : new Error(String(error)))
    } finally {
      this.isConnected = false
      this.abortController = null
    }
  }

  disconnect(): void {
    if (this.abortController) {
      this.abortController.abort()
      this.abortController = null
    }
    this.isConnected = false
  }

  get connected(): boolean {
    return this.isConnected
  }
}

// Singleton instance for convenience
let defaultClient: SSEClient | null = null

export function getSSEClient(): SSEClient {
  if (!defaultClient) {
    defaultClient = new SSEClient()
  }
  return defaultClient
}

export function createSSEClient(): SSEClient {
  return new SSEClient()
}
