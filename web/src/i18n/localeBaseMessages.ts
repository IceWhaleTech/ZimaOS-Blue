import type { LocaleKey } from './locale-catalog'

// Locale modules import this tiny shim so the heavy harness-specific merge logic
// can be loaded on demand by i18n/index.ts instead of on every initial locale load.
type MergeHarnessLocale = <T extends Record<string, unknown>>(localeKey: LocaleKey, messages: T) => T

let nodeMergeHarnessLocale: MergeHarnessLocale | null | undefined

export function mergeHarnessLocale<T extends Record<string, unknown>>(
  localeKey: LocaleKey,
  messages: T
): T {
  if (typeof window === 'undefined') {
    if (typeof nodeMergeHarnessLocale === 'undefined') {
      try {
        const dynamicRequire = eval(
          'typeof require !== "undefined" ? require : null'
        ) as ((specifier: string) => { mergeHarnessLocale?: MergeHarnessLocale }) | null

        nodeMergeHarnessLocale =
          typeof dynamicRequire === 'function'
            ? dynamicRequire('./harnessLocaleAdditions').mergeHarnessLocale ?? null
            : null
      } catch {
        nodeMergeHarnessLocale = null
      }
    }

    if (nodeMergeHarnessLocale) {
      return nodeMergeHarnessLocale(localeKey, messages)
    }
  }

  return messages
}
