/**
 * Debounce utility for delaying function execution
 */

export type DebouncedFn<T extends (...args: any[]) => any> = ((...args: Parameters<T>) => void) & {
  cancel: () => void
  flush: () => void
  pending: () => boolean
}

/**
 * Creates a debounced function that delays invoking `fn` until after `wait` milliseconds
 * have elapsed since the last time the debounced function was invoked.
 */
export function debounce<T extends (...args: any[]) => any>(
  fn: T,
  wait: number,
  immediate = false
): DebouncedFn<T> {
  let timeoutId: ReturnType<typeof setTimeout> | null = null
  let lastArgs: Parameters<T> | null = null
  let lastThis: ThisParameterType<T> | null = null
  let result: ReturnType<T>

  const invoke = () => {
    if (lastArgs) {
      result = fn.apply(lastThis, lastArgs)
      lastArgs = null
      lastThis = null
    }
    return result
  }

  const debounced = Object.assign(
    function (this: ThisParameterType<T>, ...args: Parameters<T>) {
      lastArgs = args
      lastThis = this

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
        lastArgs = null
        lastThis = null
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
