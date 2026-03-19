type TranslateFn = (key: string) => string
type TranslateExistsFn = (key: string) => boolean

type MediaFallbackSource = {
  id?: string | null
  name?: string | null
  display_name?: string | null
  is_fallback?: boolean
  strategy?: string | null
  fallback_strategy?: string | null
}

function normalizeValue(value: string | null | undefined): string {
  return value?.trim().toLowerCase() ?? ''
}

function firstNonEmpty(...values: Array<string | null | undefined>): string {
  return values.find((value) => typeof value === 'string' && value.trim().length > 0)?.trim() ?? ''
}

function getMediaFallbackTranslationKey(source: MediaFallbackSource): string | null {
  const strategy = normalizeValue(source.strategy ?? source.fallback_strategy)
  if (strategy === 'web_canvas') return 'media.fallbackWebCanvas'
  if (strategy === 'public_space') return 'media.fallbackPublicSpace'

  const id = normalizeValue(source.id)
  if (id.startsWith('fallback-web-canvas')) return 'media.fallbackWebCanvas'
  if (id.startsWith('fallback-space')) return 'media.fallbackPublicSpace'

  const names = [source.display_name, source.name].map(normalizeValue).filter(Boolean)

  if (names.some((name) => name.includes('web canvas') || name.includes('web search + canvas'))) {
    return 'media.fallbackWebCanvas'
  }

  if (
    names.some((name) => name.includes('public space') || name.includes('public creative space'))
  ) {
    return 'media.fallbackPublicSpace'
  }

  if (source.is_fallback) {
    if (names.some((name) => name.includes('canvas'))) return 'media.fallbackWebCanvas'
    if (names.some((name) => name.includes('space'))) return 'media.fallbackPublicSpace'
  }

  return null
}

export function getLocalizedMediaFallbackLabel(
  source: MediaFallbackSource,
  t: TranslateFn,
  te?: TranslateExistsFn
): string | null {
  const key = getMediaFallbackTranslationKey(source)
  if (!key) return null
  if (te && !te(key)) return null

  const localized = t(key)
  return localized.trim().length > 0 ? localized : null
}

export function getLocalizedMediaModelName(
  source: MediaFallbackSource,
  t: TranslateFn,
  te?: TranslateExistsFn
): string {
  return (
    getLocalizedMediaFallbackLabel(source, t, te) ??
    firstNonEmpty(source.display_name, source.name, source.id)
  )
}
