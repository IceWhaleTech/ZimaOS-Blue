import type { StreamChunk, SendMessageRequest } from '@/api/chat'
import { ensureFreshToken } from '@/api/client'

export interface SSEClientOptions {
  onMessage: (chunk: StreamChunk) => void
  onError?: (error: Error) => void
  onComplete?: (finalChunk?: StreamChunk) => void
  onBlocked?: (message: string, threatLevel: string) => void
  onTrialExhausted?: (message: string) => void
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
      const token = localStorage.getItem('token')
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        Accept: 'text/event-stream',
      }
      if (token) {
        headers['Authorization'] = `Bearer ${token}`
      }

      let response = await fetch(url, {
        method: 'POST',
        headers,
        body: JSON.stringify(request),
        signal: this.abortController.signal,
      })

      // Handle 401 — refresh token and retry once
      if (response.status === 401) {
        const newToken = await ensureFreshToken()
        if (newToken) {
          headers['Authorization'] = `Bearer ${newToken}`
          response = await fetch(url, {
            method: 'POST',
            headers,
            body: JSON.stringify(request),
            signal: this.abortController!.signal,
          })
        }
      }

      if (!response.ok) {
        // Handle security block (403)
        if (response.status === 403) {
          try {
            const data = await response.json()
            if (data.blocked) {
              options.onBlocked?.(data.message || 'Message blocked', data.threat_level || 'unknown')
              return
            }
          } catch {
            // Fall through to generic error
          }
        }
        // Handle trial quota exhausted (402)
        if (response.status === 402) {
          try {
            const data = await response.json()
            if (data.trial_exhausted) {
              options.onTrialExhausted?.(data.message || 'trial_quota_exhausted')
              return
            }
          } catch {
            // Fall through to generic error
          }
        }
        // Handle trial service busy (503 with trial_error flag)
        if (response.status === 503) {
          try {
            const data = await response.json()
            if (data.trial_error) {
              options.onError?.(new Error(data.message || 'trial_service_busy'))
              return
            }
          } catch {
            // Fall through to generic error
          }
        }
        // Handle other errors with message
        try {
          const data = await response.json()
          const message = data.error?.message || data.message || `HTTP error! status: ${response.status}`
          throw new Error(message)
        } catch (e) {
          if (e instanceof Error && e.message !== `HTTP error! status: ${response.status}`) {
            throw e
          }
          throw new Error(`HTTP error! status: ${response.status}`)
        }
      }

      const reader = response.body?.getReader()
      if (!reader) {
        throw new Error('No response body')
      }

      const decoder = new TextDecoder()
      let buffer = ''
      let receivedData = false
      let finalChunkData: StreamChunk | undefined

      while (this.isConnected) {
        const { done, value } = await reader.read()

        if (done) {
          // Stream closed without [DONE] - check if we received any data
          if (!receivedData) {
            options.onError?.(new Error('NO_STREAM_DATA'))
          } else if (finalChunkData) {
            // Stream closed after done:true but before [DONE] — still complete
            options.onComplete?.(finalChunkData)
          }
          break
        }

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const data = line.slice(6).trim()

            if (data === '[DONE]') {
              // [DONE] arrives after the server has persisted the message to DB.
              // Fire onComplete here (not on done:true) so fetchMessages sees the saved data.
              if (!receivedData) {
                options.onError?.(new Error('NO_STREAM_DATA'))
              } else {
                options.onComplete?.(finalChunkData)
              }
              this.isConnected = false
              break
            }

            try {
              const chunk: StreamChunk = JSON.parse(data)
              // Check for error in chunk
              if (chunk.error) {
                options.onError?.(new Error(chunk.error))
                this.isConnected = false
                break
              }
              // Mark that we received actual content
              if (chunk.delta) {
                receivedData = true
              }
              options.onMessage(chunk)

              if (chunk.done) {
                // Save the final chunk (with provider/model/stats metadata)
                // but do NOT fire onComplete yet — wait for [DONE] which arrives
                // after the server has persisted the message to the database.
                if (!receivedData && !chunk.delta) {
                  options.onError?.(new Error('NO_STREAM_DATA'))
                  this.isConnected = false
                  break
                }
                finalChunkData = chunk
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
