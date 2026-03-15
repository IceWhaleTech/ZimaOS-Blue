const LOCALIZABLE_ACTION_FALLBACKS: Record<string, string[]> = {
  extract_with_web_fetch: ['Extract with web_fetch', 'Extract with Web Fetch'],
  use_browser: ['Use browser'],
  recheck: ['Retry', 'Re-check'],
  check_a11y: ['Accessibility Only'],
  full_report: ['Full Report'],
}

type TranslateFn = (key: string, fallback?: string) => string

type ExistsFn = (key: string) => boolean

export interface TranslateCardActionLabelOptions {
  id: string
  fallback?: string
  t: TranslateFn
  te: ExistsFn
  scopes?: string[]
}

export function shouldTranslateCardActionLabel(id: string, fallback?: string): boolean {
  const normalizedFallback = fallback?.trim() ?? ''
  if (!normalizedFallback || normalizedFallback === id) return true
  return LOCALIZABLE_ACTION_FALLBACKS[id]?.includes(normalizedFallback) === true
}

export function translateCardActionLabel({
  id,
  fallback,
  t,
  te,
  scopes = [],
}: TranslateCardActionLabelOptions): string {
  const normalizedFallback = fallback?.trim() ?? ''
  if (!shouldTranslateCardActionLabel(id, normalizedFallback)) {
    return normalizedFallback || id
  }

  const keys = [...scopes.map((scope) => `${scope}.${id}`), `cardActions.${id}`]
  for (const key of keys) {
    if (!te(key)) continue
    const translated = t(key, normalizedFallback || id)
    if (translated !== key) {
      return translated
    }
  }

  return normalizedFallback || id
}
