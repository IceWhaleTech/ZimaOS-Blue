/**
 * Typeless parsing optimization utilities
 * - Web Worker for background parsing
 * - Batch DOM updates
 * - Request deduplication
 */

import type { ParsedContent } from '@/types/typeless'

// ============================================================================
// Web Worker Manager
// ============================================================================

type ParseCallback = (result: ParsedContent) => void

interface PendingRequest {
  content: string
  callbacks: ParseCallback[]
}

class TypelessParserWorker {
  private worker: Worker | null = null
  private pendingRequests = new Map<string, PendingRequest>()
  private requestId = 0
  private isSupported: boolean

  constructor() {
    // Check if Web Workers are supported
    this.isSupported = typeof Worker !== 'undefined'
  }

  private getWorker(): Worker | null {
    if (!this.isSupported) return null

    if (!this.worker) {
      try {
        // Create worker using Vite's worker import syntax
        this.worker = new Worker(
          new URL('../workers/typelessParser.worker.ts', import.meta.url),
          { type: 'module' }
        )

        this.worker.onmessage = (event) => {
          const { id, result } = event.data
          const pending = this.pendingRequests.get(id)
          if (pending) {
            pending.callbacks.forEach(cb => cb(result))
            this.pendingRequests.delete(id)
          }
        }

        this.worker.onerror = (error) => {
          console.error('Typeless parser worker error:', error)
          // Fallback to main thread parsing on error
          this.worker = null
        }
      } catch (e) {
        console.warn('Failed to create typeless parser worker:', e)
        this.isSupported = false
        return null
      }
    }

    return this.worker
  }

  /**
   * Parse content using Web Worker (async)
   * Falls back to synchronous parsing if workers unavailable
   */
  parse(content: string, callback: ParseCallback): void {
    const worker = this.getWorker()

    if (!worker) {
      // Fallback: import and use main thread parsing
      import('./typeless').then(({ parseTypelessContent }) => {
        callback(parseTypelessContent(content))
      })
      return
    }

    // Generate request ID
    const id = `parse-${++this.requestId}`

    // Check for duplicate content (deduplication)
    for (const [, pending] of this.pendingRequests) {
      if (pending.content === content) {
        // Add callback to existing request
        pending.callbacks.push(callback)
        return
      }
    }

    // Create new request
    this.pendingRequests.set(id, {
      content,
      callbacks: [callback],
    })

    worker.postMessage({ type: 'parse', id, content })
  }

  /**
   * Parse content and return a promise
   */
  parseAsync(content: string): Promise<ParsedContent> {
    return new Promise((resolve) => {
      this.parse(content, resolve)
    })
  }

  /**
   * Terminate the worker
   */
  terminate(): void {
    if (this.worker) {
      this.worker.terminate()
      this.worker = null
    }
    this.pendingRequests.clear()
  }
}

// Singleton instance
export const typelessParserWorker = new TypelessParserWorker()

// ============================================================================
// Batch DOM Updates
// ============================================================================

interface DOMUpdate {
  element: HTMLElement
  html: string
}

class BatchDOMUpdater {
  private pendingUpdates: DOMUpdate[] = []
  private rafId: number | null = null
  private isProcessing = false

  /**
   * Queue a DOM update to be batched
   */
  queue(element: HTMLElement, html: string): void {
    this.pendingUpdates.push({ element, html })
    this.scheduleFlush()
  }

  /**
   * Queue multiple updates at once
   */
  queueMany(updates: DOMUpdate[]): void {
    this.pendingUpdates.push(...updates)
    this.scheduleFlush()
  }

  private scheduleFlush(): void {
    if (this.rafId !== null) return

    this.rafId = requestAnimationFrame(() => {
      this.flush()
    })
  }

  /**
   * Flush all pending updates in a single frame
   */
  private flush(): void {
    if (this.isProcessing || this.pendingUpdates.length === 0) {
      this.rafId = null
      return
    }

    this.isProcessing = true
    const updates = this.pendingUpdates
    this.pendingUpdates = []
    this.rafId = null

    // Group updates by parent element for efficiency
    const updatesByParent = new Map<HTMLElement, DOMUpdate[]>()

    for (const update of updates) {
      const parent = update.element.parentElement
      if (parent) {
        const existing = updatesByParent.get(parent) || []
        existing.push(update)
        updatesByParent.set(parent, existing)
      } else {
        // No parent, update directly
        update.element.innerHTML = update.html
      }
    }

    // Apply grouped updates
    for (const [_parent, groupedUpdates] of updatesByParent) {
      for (const update of groupedUpdates) {
        update.element.innerHTML = update.html
      }
    }

    this.isProcessing = false

    // Check if more updates were queued during processing
    if (this.pendingUpdates.length > 0) {
      this.scheduleFlush()
    }
  }

  /**
   * Force immediate flush (use sparingly)
   */
  flushSync(): void {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId)
      this.rafId = null
    }
    this.flush()
  }

  /**
   * Clear all pending updates
   */
  clear(): void {
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId)
      this.rafId = null
    }
    this.pendingUpdates = []
  }
}

// Singleton instance
export const batchDOMUpdater = new BatchDOMUpdater()

// ============================================================================
// Utility: Debounced parsing for streaming content
// ============================================================================

export function createDebouncedParser(
  callback: (result: ParsedContent) => void,
  delay = 100
): (content: string) => void {
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  let lastContent = ''

  return (content: string) => {
    // Skip if content hasn't changed
    if (content === lastContent) return
    lastContent = content

    if (timeoutId) {
      clearTimeout(timeoutId)
    }

    timeoutId = setTimeout(() => {
      typelessParserWorker.parse(content, callback)
      timeoutId = null
    }, delay)
  }
}

// ============================================================================
// Cleanup on page unload
// ============================================================================

if (typeof window !== 'undefined') {
  window.addEventListener('beforeunload', () => {
    typelessParserWorker.terminate()
    batchDOMUpdater.clear()
  })
}
