/**
 * Debounce utility for delaying function execution
 */

export type DebouncedFn<T extends (...args: unknown[]) => unknown> = ((...args: Parameters<T>) => void) & {
  cancel: () => void
  flush: () => void
  pending: () => boolean
}

/**
 * Creates a debounced function that delays invoking `fn` until after `wait` milliseconds
 * have elapsed since the last time the debounced function was invoked.
 */
export function debounce<T extends (...args: unknown[]) => unknown>(
  fn: T,
  wait: number,
  immediate = false
): DebouncedFn<T> {
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  let pendingInvoke: (() => ReturnType<T>) | null = null
  let result: ReturnType<T>

  const invoke = () => {
    if (pendingInvoke) {
      result = pendingInvoke()
      pendingInvoke = null
    }
    return result
  }

  const debounced = Object.assign(
    function (this: ThisParameterType<T>, ...args: Parameters<T>) {
      pendingInvoke = () => fn.apply(this, args) as ReturnType<T>

      const callNow = immediate && !timeoutId

      if (timeoutId) {
        clearTimeout(timeoutId)
      }

      timeoutId = setTimeout(() => {
        timeoutId = null
        if (!immediate) {
          invoke()
        }
      }, wait)

      if (callNow) {
        return invoke()
      }

      return result
    },
    {
      cancel: () => {
        if (timeoutId) {
          clearTimeout(timeoutId)
          timeoutId = null
        }
        pendingInvoke = null
      },
      flush: () => {
        if (timeoutId) {
          clearTimeout(timeoutId)
          timeoutId = null
        }
        return invoke()
      },
      pending: () => timeoutId !== null,
    }
  ) as DebouncedFn<T>

  return debounced
}

/**
 * Vue composable for reactive debounced values
 */
import { ref, watch, type Ref, type WatchOptions } from 'vue'

export function useDebounce<T>(value: Ref<T>, delay: number): Ref<T> {
  const debouncedValue = ref(value.value) as Ref<T>

  let timeoutId: ReturnType<typeof setTimeout> | null = null

  watch(
    value,
    (newValue) => {
      if (timeoutId) {
        clearTimeout(timeoutId)
      }
      timeoutId = setTimeout(() => {
        debouncedValue.value = newValue
        timeoutId = null
      }, delay)
    },
    { immediate: false } as WatchOptions
  )

  return debouncedValue
}
