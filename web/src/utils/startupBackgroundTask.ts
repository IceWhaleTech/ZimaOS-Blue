type IdleWindow = Window & {
  requestIdleCallback?: (
    callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void,
    options?: { timeout?: number }
  ) => number
  cancelIdleCallback?: (handle: number) => void
}

export function scheduleStartupBackgroundTask(
  task: () => void,
  minDelayMs = 1500,
  idleFallbackTimeoutMs = 200
): () => void {
  const idleWindow = window as IdleWindow
  const normalizedDelayMs = Math.max(0, minDelayMs)
  const normalizedIdleFallbackTimeoutMs = Math.max(0, idleFallbackTimeoutMs)

  let cancelled = false
  let delayHandle: number | null = null
  let fallbackHandle: number | null = null
  let idleHandle: number | null = null

  const runTask = () => {
    if (cancelled) return
    task()
  }

  const scheduleIdleWork = () => {
    if (cancelled) return

    if (typeof idleWindow.requestIdleCallback === 'function') {
      idleHandle = idleWindow.requestIdleCallback(() => runTask(), {
        timeout: normalizedIdleFallbackTimeoutMs,
      })
      return
    }

    fallbackHandle = window.setTimeout(runTask, 0)
  }

  delayHandle = window.setTimeout(scheduleIdleWork, normalizedDelayMs)

  return () => {
    cancelled = true
    if (delayHandle !== null) {
      window.clearTimeout(delayHandle)
    }
    if (fallbackHandle !== null) {
      window.clearTimeout(fallbackHandle)
    }
    if (idleHandle !== null) {
      idleWindow.cancelIdleCallback?.(idleHandle)
    }
  }
}
