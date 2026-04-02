import { afterEach, describe, expect, it, vi } from 'vitest'

import { scheduleStartupBackgroundTask } from '@/utils/startupBackgroundTask'

describe('scheduleStartupBackgroundTask', () => {
  afterEach(() => {
    vi.useRealTimers()
    delete (window as Window & { requestIdleCallback?: unknown }).requestIdleCallback
    delete (window as Window & { cancelIdleCallback?: unknown }).cancelIdleCallback
  })

  it('waits for the minimum delay before scheduling idle work', () => {
    vi.useFakeTimers()

    const idle = {
      callback: null as null | ((
        deadline: { didTimeout: boolean; timeRemaining: () => number }
      ) => void),
    }

    const requestIdleCallbackSpy = vi.fn(
      (
        callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void
      ) => {
        idle.callback = callback
        return 7
      }
    )

    Object.defineProperty(window, 'requestIdleCallback', {
      value: requestIdleCallbackSpy,
      configurable: true,
    })

    const task = vi.fn()

    scheduleStartupBackgroundTask(task, 1500, 75)

    expect(requestIdleCallbackSpy).not.toHaveBeenCalled()

    vi.advanceTimersByTime(1499)
    expect(requestIdleCallbackSpy).not.toHaveBeenCalled()
    expect(task).not.toHaveBeenCalled()

    vi.advanceTimersByTime(1)
    expect(requestIdleCallbackSpy).toHaveBeenCalledTimes(1)
    expect(requestIdleCallbackSpy).toHaveBeenCalledWith(expect.any(Function), { timeout: 75 })
    expect(task).not.toHaveBeenCalled()

    if (!idle.callback) {
      throw new Error('expected requestIdleCallback callback to be captured')
    }

    idle.callback({
      didTimeout: false,
      timeRemaining: () => 50,
    })

    expect(task).toHaveBeenCalledTimes(1)
  })

  it('cancels pending delayed and idle work during cleanup', () => {
    vi.useFakeTimers()

    const cancelIdleCallbackSpy = vi.fn()
    const requestIdleCallbackSpy = vi.fn(() => 11)

    Object.defineProperty(window, 'requestIdleCallback', {
      value: requestIdleCallbackSpy,
      configurable: true,
    })
    Object.defineProperty(window, 'cancelIdleCallback', {
      value: cancelIdleCallbackSpy,
      configurable: true,
    })

    const task = vi.fn()
    const cleanup = scheduleStartupBackgroundTask(task, 1200, 90)

    vi.advanceTimersByTime(1200)
    expect(requestIdleCallbackSpy).toHaveBeenCalledTimes(1)

    cleanup()
    expect(cancelIdleCallbackSpy).toHaveBeenCalledWith(11)

    vi.runOnlyPendingTimers()
    expect(task).not.toHaveBeenCalled()
  })
})
