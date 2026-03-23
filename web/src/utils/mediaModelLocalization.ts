type TranslateFn = (key: string) => string
type TranslateExistsFn = (key: string) => boolean

type MediaFallbackSource = {
  id?: string | null
  name?: string | null
  display_name?: string | null
  is_fallback?: boolean
  strategy?: string | null
  fallback_strategy?: string | null
  render_mode?: string | null
  template_id?: string | null
  style_preset?: string | null
}

type MediaFallbackDisclosureSource = {
  strategy?: string | null
  disclosure?: string | null
}

function normalizeValue(value: string | null | undefined): string {
  return value?.trim().toLowerCase() ?? ''
}

function firstNonEmpty(...values: Array<string | null | undefined>): string {
  return values.find((value) => typeof value === 'string' && value.trim().length > 0)?.trim() ?? ''
}

function normalizeStylePreset(value: string | null | undefined): string {
  const normalized = normalizeValue(value)
  if (
    normalized === 'nano_slides' ||
    normalized === 'nanoslides' ||
    normalized === 'nano slides' ||
    normalized === 'nano-slides'
  ) {
    return 'nanoslides'
  }
  if (
    normalized === 'banana_slides' ||
    normalized === 'bananaslides' ||
    normalized === 'banana slides' ||
    normalized === 'banana-slides'
  ) {
    return 'bananaslides'
  }
  return firstNonEmpty(value)
}

function getMediaFallbackTranslationKey(source: MediaFallbackSource): string | null {
  const strategy = normalizeValue(source.strategy ?? source.fallback_strategy)
  if (strategy === 'web_canvas') return 'media.fallbackWebCanvas'
  if (strategy === 'public_space') return 'media.fallbackPublicSpace'
  if (strategy === 'native_timeline') return 'media.fallbackNativeTimeline'

  const id = normalizeValue(source.id)
  if (id.startsWith('fallback-web-canvas')) return 'media.fallbackWebCanvas'
  if (id.startsWith('fallback-space')) return 'media.fallbackPublicSpace'
  if (id.startsWith('fallback-native')) return 'media.fallbackNativeTimeline'

  const names = [source.display_name, source.name].map(normalizeValue).filter(Boolean)

  if (names.some((name) => name.includes('web canvas') || name.includes('web search + canvas'))) {
    return 'media.fallbackWebCanvas'
  }

  if (
    names.some((name) => name.includes('public space') || name.includes('public creative space'))
  ) {
    return 'media.fallbackPublicSpace'
  }

  if (
    names.some(
      (name) =>
        name.includes('native timeline') ||
        name.includes('native video') ||
        name.includes('timeline renderer')
    )
  ) {
    return 'media.fallbackNativeTimeline'
  }

  if (source.is_fallback) {
    if (names.some((name) => name.includes('canvas'))) return 'media.fallbackWebCanvas'
    if (names.some((name) => name.includes('space'))) return 'media.fallbackPublicSpace'
    if (names.some((name) => name.includes('native') || name.includes('timeline'))) {
      return 'media.fallbackNativeTimeline'
    }
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

function getMediaFallbackDisclosureTranslationKey(
  source: MediaFallbackDisclosureSource
): string | null {
  switch (normalizeValue(source.strategy)) {
    case 'web_canvas':
      return 'media.fallbackDisclosureWebCanvas'
    case 'public_space':
      return 'media.fallbackDisclosurePublicSpace'
    case 'native_timeline':
      return 'media.fallbackDisclosureNativeTimeline'
    default:
      return source.disclosure ? null : 'media.fallbackDisclosureGeneric'
  }
}

export function getLocalizedMediaFallbackDisclosure(
  source: MediaFallbackDisclosureSource,
  t: TranslateFn,
  te?: TranslateExistsFn
): string {
  const key = getMediaFallbackDisclosureTranslationKey(source)
  if (key) {
    if (te && !te(key)) {
      return firstNonEmpty(source.disclosure)
    }
    const localized = t(key)
    if (localized.trim().length > 0 && localized !== key) {
      return localized
    }
  }
  return firstNonEmpty(source.disclosure)
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

export function getMediaFallbackStyleLabel(source: MediaFallbackSource): string | null {
  const normalized = normalizeStylePreset(source.style_preset)
  return normalized || null
}

function getMediaFallbackTemplateTranslationKey(source: MediaFallbackSource): string | null {
  switch (normalizeValue(source.template_id || source.render_mode)) {
    case 'cover':
      return 'media.fallbackTemplateCover'
    case 'split':
      return 'media.fallbackTemplateSplit'
    case 'text_only':
      return 'media.fallbackTemplateTextOnly'
    case 'poster':
      return 'media.fallbackTemplatePoster'
    case 'slide':
      return 'media.fallbackTemplateSlide'
    default:
      return null
  }
}

export function getLocalizedMediaFallbackTemplateLabel(
  source: MediaFallbackSource,
  t: TranslateFn,
  te?: TranslateExistsFn
): string | null {
  const key = getMediaFallbackTemplateTranslationKey(source)
  if (key) {
    if (te && te(key)) {
      const localized = t(key)
      if (localized.trim().length > 0 && localized !== key) {
        return localized
      }
    }
  }
  return getMediaFallbackTemplateLabel(source)
}

export function getMediaFallbackTemplateLabel(
  source: MediaFallbackSource
): string | null {
  const normalized = normalizeValue(source.template_id || source.render_mode)
  if (!normalized) return null

  switch (normalized) {
    case 'cover':
      return 'cover'
    case 'split':
      return 'split'
    case 'text_only':
      return 'text-only'
    case 'poster':
      return 'poster'
    case 'slide':
      return 'slide'
    default:
      return firstNonEmpty(source.template_id, source.render_mode) || null
  }
}
