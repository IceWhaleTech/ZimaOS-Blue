import type { LocaleKey } from './locale-catalog'
import processTraceLocaleBackfills from './process-trace-locale-backfills'

// Locale modules import this tiny shim so the heavy harness-specific merge logic
// can be loaded on demand by i18n/index.ts instead of on every initial locale load.
type MergeHarnessLocale = <T extends Record<string, unknown>>(
  localeKey: LocaleKey,
  messages: T
) => T

let nodeMergeHarnessLocale: MergeHarnessLocale | null | undefined

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function mergeLocaleNodes<T extends Record<string, unknown>>(
  base: T,
  override: Record<string, unknown>
): T {
  const merged: Record<string, unknown> = { ...base }

  for (const [key, value] of Object.entries(override)) {
    const current = merged[key]
    if (isPlainObject(current) && isPlainObject(value)) {
      merged[key] = mergeLocaleNodes(current, value)
      continue
    }
    merged[key] = value
  }

  return merged as T
}

export function mergeHarnessLocale<T extends Record<string, unknown>>(
  localeKey: LocaleKey,
  messages: T
): T {
  const mergedBaseMessages = mergeLocaleNodes(
    (processTraceLocaleBackfills[localeKey] ?? {}) as Record<string, unknown> as T,
    messages
  )

  if (typeof window === 'undefined') {
    if (typeof nodeMergeHarnessLocale === 'undefined') {
      try {
        const dynamicRequire = eval('typeof require !== "undefined" ? require : null') as
          | ((specifier: string) => { mergeHarnessLocale?: MergeHarnessLocale })
          | null

        nodeMergeHarnessLocale =
          typeof dynamicRequire === 'function'
            ? (dynamicRequire('./harness-locale-additions').mergeHarnessLocale ?? null)
            : null
      } catch {
        nodeMergeHarnessLocale = null
      }
    }

    if (nodeMergeHarnessLocale) {
      return nodeMergeHarnessLocale(localeKey, mergedBaseMessages)
    }
  }

  return mergedBaseMessages
}
