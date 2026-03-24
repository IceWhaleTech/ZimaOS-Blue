type AnyFn = (...args: any[]) => void

export type RafThrottledFn<T extends AnyFn> = ((...args: Parameters<T>) => void) & {
  cancel: () => void
  flush: () => void
}

export function rafThrottle<T extends AnyFn>(fn: T): RafThrottledFn<T> {
  let frameId: number | null = null
  let lastArgs: Parameters<T> | null = null

  const invoke = () => {
    frameId = null
    const args = lastArgs
    lastArgs = null
    if (args) {
      fn(...args)
    }
  }

  const throttled = ((...args: Parameters<T>) => {
    lastArgs = args
    if (frameId !== null) return
    frameId = window.requestAnimationFrame(invoke)
  }) as RafThrottledFn<T>

  throttled.cancel = () => {
    if (frameId !== null) {
      window.cancelAnimationFrame(frameId)
      frameId = null
    }
    lastArgs = null
  }

  throttled.flush = () => {
    if (frameId !== null) {
      window.cancelAnimationFrame(frameId)
    }
    if (!lastArgs) {
      frameId = null
      return
    }
    invoke()
  }

  return throttled
}
