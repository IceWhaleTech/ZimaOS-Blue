import { createI18n } from 'vue-i18n'
import { hasStoredSessionHint } from '@/utils/authStorage'
import { shouldDeferLocaleEnhancementsOnDesktopStartup } from '@/utils/desktopStartup'
import buildBuiltinSkillBackfill from './builtin-skill-backfills'
import builtinToolBackfills from './builtin-tool-backfills'
import { localeKeys, localeOptions, type LocaleKey } from './locale-catalog'

export { localeKeys, localeOptions }
export type { LocaleKey }

type LocaleMessages = Record<string, unknown>
type LocaleModule = { default: LocaleMessages }
type LocaleLoader = () => Promise<LocaleModule>
type LocaleOverrideCatalog = Partial<Record<LocaleKey, LocaleMessages>>
type LocaleEnhancerModule = {
  mergeHarnessLocale: <T extends Record<string, unknown>>(localeKey: LocaleKey, messages: T) => T
}
type I18nBridge = {
  install: (app: unknown, ...options: unknown[]) => unknown
  global: {
    t: (key: string, ...args: unknown[]) => string
    te: (key: string) => boolean
    setLocaleMessage: (locale: string, message: LocaleMessages) => void
    locale: { value: string }
  }
}
type IdleWindow = Window & {
  requestIdleCallback?: (
    callback: (deadline: { didTimeout: boolean; timeRemaining: () => number }) => void,
    options?: { timeout?: number }
  ) => number
}

// Minimal fallback messages for initial render before the selected locale finishes loading.
const minimalMessages = {
  common: {
    loading: 'Loading...',
  },
} satisfies LocaleMessages

export type LocaleDirection = 'ltr' | 'rtl'

const LOCALE_KEY = 'zimaos-blue-locale'
const LOCALE_CACHE_VERSION = 'v1'
const LOCALE_CACHE_KEY_PREFIX = `zimaos-blue-locale-cache:${LOCALE_CACHE_VERSION}:`
const RTL_LANGUAGE_CODES = new Set(['ar', 'ckb', 'fa', 'he', 'ps', 'ur'])
const localeKeySet = new Set<LocaleKey>(localeKeys)
const fallbackLocale: LocaleKey = localeKeySet.has('en-US') ? 'en-US' : (localeKeys[0] ?? 'en-US')

// Map browser language codes to our locale keys.
const browserLocaleMap: Record<string, LocaleKey> = {
  ca: 'ca-ES',
  cs: 'cs-CZ',
  da: 'da-DK',
  de: 'de-DE',
  el: 'el-GR',
  en: 'en-US',
  'en-GB': 'en-GB',
  'en-US': 'en-US',
  es: 'es-ES',
  fr: 'fr-FR',
  ga: 'ga-IE',
  hr: 'hr-HR',
  hu: 'hu-HU',
  it: 'it-IT',
  ja: 'ja-JP',
  ko: 'ko-KR',
  ml: 'ml-IN',
  nb: 'nb-NO',
  nl: 'nl-NL',
  no: 'nb-NO',
  pl: 'pl-PL',
  pt: 'pt-BR',
  'pt-BR': 'pt-BR',
  'pt-PT': 'pt-PT',
  ro: 'ro-RO',
  ru: 'ru-RU',
  sk: 'sk-SK',
  sv: 'sv-SE',
  zh: 'zh-CN',
  'zh-CN': 'zh-CN',
  'zh-TW': 'zh-TW',
  'zh-HK': 'zh-TW',
}

function resolvePreferredLocale(locale: string | null | undefined): LocaleKey | null {
  const normalized = locale?.trim()
  if (!normalized) {
    return null
  }

  const exactMatch = browserLocaleMap[normalized]
  if (exactMatch && localeKeySet.has(exactMatch)) {
    return exactMatch
  }

  const langCode = normalized.split(/[-_]/)[0]
  if (langCode) {
    const langMatch = browserLocaleMap[langCode]
    if (langMatch && localeKeySet.has(langMatch)) {
      return langMatch
    }

    if (langCode === 'zh' && localeKeySet.has('zh-CN')) {
      return 'zh-CN'
    }

    if (langCode === 'en' && localeKeySet.has('en-US')) {
      return 'en-US'
    }
  }

  return null
}

function resolveSupportedLocale(locale: LocaleKey | string): LocaleKey {
  return resolvePreferredLocale(locale) ?? fallbackLocale
}

export function getLocaleDirection(locale: string): LocaleDirection {
  const normalized = locale.trim().toLowerCase()
  const languageCode = normalized.split(/[-_]/)[0]
  return languageCode && RTL_LANGUAGE_CODES.has(languageCode) ? 'rtl' : 'ltr'
}

function hasLocalStorageApi(): boolean {
  return (
    typeof localStorage !== 'undefined' &&
    typeof localStorage.getItem === 'function' &&
    typeof localStorage.setItem === 'function' &&
    typeof localStorage.removeItem === 'function'
  )
}

function getDefaultLocale(): LocaleKey {
  if (hasLocalStorageApi()) {
    const saved = localStorage.getItem(LOCALE_KEY)
    const savedLocale = resolvePreferredLocale(saved)
    if (savedLocale) {
      return savedLocale
    }
  }

  const browserLang = typeof navigator !== 'undefined' ? navigator.language : fallbackLocale
  return resolvePreferredLocale(browserLang) ?? fallbackLocale
}

function localeCacheKey(locale: LocaleKey): string {
  return `${LOCALE_CACHE_KEY_PREFIX}${locale}`
}

function readCachedLocaleMessages(locale: LocaleKey): LocaleMessages | null {
  if (!hasLocalStorageApi()) {
    return null
  }

  const key = localeCacheKey(locale)
  const raw = localStorage.getItem(key)
  if (!raw) {
    return null
  }

  try {
    const parsed = JSON.parse(raw)
    return isPlainObject(parsed) ? parsed : null
  } catch {
    localStorage.removeItem(key)
    return null
  }
}

function writeCachedLocaleMessages(locale: LocaleKey, messages: LocaleMessages): void {
  if (!hasLocalStorageApi()) {
    return
  }

  try {
    localStorage.setItem(localeCacheKey(locale), JSON.stringify(messages))
  } catch {
    // Ignore cache write failures such as private mode / quota pressure.
  }
}

const initialLocale = getDefaultLocale()
const initialCachedMessages = readCachedLocaleMessages(initialLocale)
const initialMessages: any = {
  [fallbackLocale]:
    initialLocale === fallbackLocale ? (initialCachedMessages ?? minimalMessages) : minimalMessages,
}

if (initialLocale !== fallbackLocale) {
  initialMessages[initialLocale] = initialCachedMessages ?? minimalMessages
}

const rawI18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale,
  missingWarn: false,
  fallbackWarn: false,
  messages: initialMessages,
})

export const i18n = rawI18n as unknown as I18nBridge

type LocaleComposerBridge = {
  getLocaleMessage: (locale: string) => LocaleMessages
  setLocaleMessage: (locale: string, message: LocaleMessages) => void
  locale: { value: string }
}

const localeLoaders = {
  'ca-ES': () => import('./locales/ca-ES'),
  'cs-CZ': () => import('./locales/cs-CZ'),
  'da-DK': () => import('./locales/da-DK'),
  'de-DE': () => import('./locales/de-DE'),
  'el-GR': () => import('./locales/el-GR'),
  'en-GB': () => import('./locales/en-GB'),
  'en-US': () => import('./locales/en-US'),
  'es-ES': () => import('./locales/es-ES'),
  'fr-FR': () => import('./locales/fr-FR'),
  'ga-IE': () => import('./locales/ga-IE'),
  'hr-HR': () => import('./locales/hr-HR'),
  'hu-HU': () => import('./locales/hu-HU'),
  'it-IT': () => import('./locales/it-IT'),
  'ja-JP': () => import('./locales/ja-JP'),
  'ko-KR': () => import('./locales/ko-KR'),
  'ml-IN': () => import('./locales/ml-IN'),
  'nb-NO': () => import('./locales/nb-NO'),
  'nl-NL': () => import('./locales/nl-NL'),
  'pl-PL': () => import('./locales/pl-PL'),
  'pt-BR': () => import('./locales/pt-BR'),
  'pt-PT': () => import('./locales/pt-PT'),
  'ro-RO': () => import('./locales/ro-RO'),
  'ru-RU': () => import('./locales/ru-RU'),
  'sk-SK': () => import('./locales/sk-SK'),
  'sv-SE': () => import('./locales/sv-SE'),
  'zh-CN': () => import('./locales/zh-CN'),
  'zh-TW': () => import('./locales/zh-TW'),
} satisfies Record<LocaleKey, LocaleLoader>

const loadedLocales = new Set<LocaleKey>()
const enhancedLocales = new Set<LocaleKey>()
const loadingLocales = new Map<LocaleKey, Promise<LocaleKey>>()
const enhancingLocales = new Map<LocaleKey, Promise<LocaleKey>>()
const scheduledLocaleRefreshes = new Set<LocaleKey>()
const scheduledLocaleEnhancements = new Set<LocaleKey>()
let localeOverridesPromise: Promise<LocaleOverrideCatalog> | null = null
let localeEnhancerPromise: Promise<LocaleEnhancerModule> | null = null

const DEFERRED_LOCALE_ENHANCEMENT_TIMEOUT_MS = 1500
const DEFERRED_LOCALE_ENHANCEMENT_START_DELAY_MS = 600

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function deepMergeMessages(base: LocaleMessages, overrides: LocaleMessages): LocaleMessages {
  const merged: LocaleMessages = { ...base }

  for (const [key, overrideValue] of Object.entries(overrides)) {
    const baseValue = merged[key]
    if (isPlainObject(baseValue) && isPlainObject(overrideValue)) {
      merged[key] = deepMergeMessages(baseValue, overrideValue)
      continue
    }
    merged[key] = overrideValue
  }

  return merged
}

async function loadLocaleOverrides(): Promise<LocaleOverrideCatalog> {
  if (localeOverridesPromise) {
    return localeOverridesPromise
  }

  localeOverridesPromise = Promise.resolve(builtinToolBackfills as LocaleOverrideCatalog)

  return localeOverridesPromise
}

async function loadLocaleEnhancer(): Promise<LocaleEnhancerModule> {
  if (localeEnhancerPromise) {
    return localeEnhancerPromise
  }

  localeEnhancerPromise = import('./harness-locale-additions')
  return localeEnhancerPromise
}

async function applyLocaleEnhancements(
  locale: LocaleKey,
  messages: LocaleMessages
): Promise<LocaleMessages> {
  const { mergeHarnessLocale } = await loadLocaleEnhancer()
  return mergeHarnessLocale(locale, messages)
}

function storeLoadedLocaleMessages(
  locale: LocaleKey,
  messages: LocaleMessages,
  options: { cache?: boolean; enhanced?: boolean } = {}
): void {
  ;(i18n.global as unknown as LocaleComposerBridge).setLocaleMessage(locale, messages)
  if (options.cache !== false) {
    writeCachedLocaleMessages(locale, messages)
  }
  loadedLocales.add(locale)
  if (options.enhanced) {
    enhancedLocales.add(locale)
  }
}

async function buildLocaleMessages(
  locale: LocaleKey,
  messages: LocaleMessages,
  requireEnhancements: boolean
): Promise<{ messages: LocaleMessages; enhanced: boolean }> {
  if (!requireEnhancements) {
    return { messages, enhanced: false }
  }

  try {
    const enhancedMessages = await applyLocaleEnhancements(locale, messages)
    return { messages: enhancedMessages, enhanced: true }
  } catch (error) {
    console.warn(
      `Failed to load locale enhancements for ${locale}, using base locale messages`,
      error
    )
    return { messages, enhanced: false }
  }
}

async function ensureLocaleEnhancements(locale: LocaleKey): Promise<LocaleKey> {
  if (enhancedLocales.has(locale)) {
    return locale
  }

  const existing = enhancingLocales.get(locale)
  if (existing) {
    return existing
  }

  const pending = (async () => {
    try {
      const currentMessages = (i18n.global as unknown as LocaleComposerBridge).getLocaleMessage(
        locale
      )
      const baseMessages = isPlainObject(currentMessages) ? currentMessages : {}
      const { messages, enhanced } = await buildLocaleMessages(locale, baseMessages, true)
      storeLoadedLocaleMessages(locale, messages, { enhanced })
      return locale
    } finally {
      enhancingLocales.delete(locale)
    }
  })()

  enhancingLocales.set(locale, pending)
  return pending
}

function scheduleLocaleEnhancement(locale: LocaleKey, options: { minDelayMs?: number } = {}): void {
  if (enhancedLocales.has(locale) || scheduledLocaleEnhancements.has(locale)) {
    return
  }

  scheduledLocaleEnhancements.add(locale)
  const runEnhancement = () => {
    scheduledLocaleEnhancements.delete(locale)
    void ensureLocaleEnhancements(locale).catch(() => {})
  }

  const queueEnhancement = () => {
    if (typeof window !== 'undefined') {
      const idleWindow = window as IdleWindow
      if (typeof idleWindow.requestIdleCallback === 'function') {
        idleWindow.requestIdleCallback(() => runEnhancement(), {
          timeout: DEFERRED_LOCALE_ENHANCEMENT_TIMEOUT_MS,
        })
        return
      }

      window.setTimeout(runEnhancement, 0)
      return
    }

    runEnhancement()
  }

  const minDelayMs = options.minDelayMs ?? 0

  if (typeof window !== 'undefined') {
    if (minDelayMs > 0) {
      window.setTimeout(queueEnhancement, minDelayMs)
      return
    }
  }

  queueEnhancement()
}

function shouldDeferInitialLocaleEnhancements(): boolean {
  if (typeof window === 'undefined') {
    return false
  }

  return shouldDeferLocaleEnhancementsOnDesktopStartup(
    !!window.__BLUE_DESKTOP__,
    hasStoredSessionHint(),
    window.location.pathname || '/'
  )
}

function applyLocaleState(locale: LocaleKey): void {
  const direction = getLocaleDirection(locale)
  ;(i18n.global as unknown as LocaleComposerBridge).locale.value = locale

  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(LOCALE_KEY, locale)
  }

  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale
    document.documentElement.dir = direction
    document.documentElement.dataset.localeDirection = direction
  }

  if (typeof window !== 'undefined' && window.__TAURI_INTERNALS__?.invoke) {
    window.__TAURI_INTERNALS__.invoke('set_tray_locale', { locale }).catch(() => {})
  }
}

async function loadLocaleMessages(
  locale: LocaleKey,
  options: { requireEnhancements?: boolean; deferEnhancements?: boolean } = {}
): Promise<LocaleKey> {
  const requireEnhancements = options.requireEnhancements ?? true

  if (loadedLocales.has(locale)) {
    if (!requireEnhancements || enhancedLocales.has(locale)) {
      return locale
    }
    return ensureLocaleEnhancements(locale)
  }

  const existingEnhancement = requireEnhancements ? enhancingLocales.get(locale) : null
  if (existingEnhancement) {
    return existingEnhancement
  }

  const existing = loadingLocales.get(locale)
  if (existing) {
    const resolvedLocale = await existing
    if (!requireEnhancements || enhancedLocales.has(resolvedLocale)) {
      return resolvedLocale
    }
    return ensureLocaleEnhancements(resolvedLocale)
  }

  const pending = (async () => {
    try {
      const localeLoader = localeLoaders[locale]
      if (!localeLoader) {
        throw new Error(`No locale loader configured for ${locale}`)
      }

      const [module, localeOverrides] = await Promise.all([
        localeLoader(),
        loadLocaleOverrides(),
      ])
      const mergedMessages = deepMergeMessages(module.default, localeOverrides[locale] || {})
      const messagesWithBuiltinSkills = deepMergeMessages(
        mergedMessages,
        buildBuiltinSkillBackfill(locale, mergedMessages)
      )
      const deferEnhancements = options.deferEnhancements ?? false
      const { messages, enhanced } = await buildLocaleMessages(
        locale,
        messagesWithBuiltinSkills,
        !deferEnhancements && requireEnhancements
      )

      storeLoadedLocaleMessages(locale, messages, {
        cache: !deferEnhancements || enhanced,
        enhanced,
      })

      if (deferEnhancements && requireEnhancements) {
        scheduleLocaleEnhancement(locale, {
          minDelayMs: DEFERRED_LOCALE_ENHANCEMENT_START_DELAY_MS,
        })
      }

      return locale
    } catch (error) {
      if (locale === fallbackLocale) {
        console.warn(`Failed to load locale ${locale}, continuing with minimal fallback`, error)
        return fallbackLocale
      }

      console.warn(`Failed to load locale ${locale}, falling back to ${fallbackLocale}`, error)
      return loadLocaleMessages(fallbackLocale)
    } finally {
      loadingLocales.delete(locale)
    }
  })()

  loadingLocales.set(locale, pending)
  return pending
}

function scheduleLocaleRefresh(
  locale: LocaleKey,
  options: { deferEnhancements?: boolean; minDelayMs?: number } = {}
): void {
  if (loadedLocales.has(locale) || scheduledLocaleRefreshes.has(locale)) {
    return
  }

  scheduledLocaleRefreshes.add(locale)
  const runRefresh = () => {
    scheduledLocaleRefreshes.delete(locale)
    void loadLocaleMessages(locale, {
      deferEnhancements: options.deferEnhancements,
    }).catch(() => {})
  }

  const queueRefresh = () => {
    if (typeof window !== 'undefined') {
      const idleWindow = window as IdleWindow
      if (typeof idleWindow.requestIdleCallback === 'function') {
        idleWindow.requestIdleCallback(() => runRefresh(), { timeout: 2000 })
        return
      }

      window.setTimeout(runRefresh, 0)
      return
    }

    runRefresh()
  }

  const minDelayMs = options.minDelayMs ?? 0

  if (typeof window !== 'undefined') {
    if (minDelayMs > 0) {
      window.setTimeout(queueRefresh, minDelayMs)
      return
    }
  }

  queueRefresh()
}

export async function setLocale(locale: LocaleKey): Promise<void> {
  const resolvedLocale = await loadLocaleMessages(resolveSupportedLocale(locale), {
    requireEnhancements: true,
  })
  applyLocaleState(resolvedLocale)
}

export function getLocale(): LocaleKey {
  return (i18n.global as unknown as LocaleComposerBridge).locale.value as LocaleKey
}

export async function initLocale(): Promise<void> {
  const defaultLocale = getDefaultLocale()
  if (readCachedLocaleMessages(defaultLocale)) {
    const deferEnhancements = shouldDeferInitialLocaleEnhancements()
    applyLocaleState(defaultLocale)
    scheduleLocaleRefresh(defaultLocale, {
      deferEnhancements,
      minDelayMs: deferEnhancements ? DEFERRED_LOCALE_ENHANCEMENT_START_DELAY_MS : 0,
    })
    return
  }

  const resolvedLocale = await loadLocaleMessages(defaultLocale, {
    requireEnhancements: true,
    deferEnhancements: shouldDeferInitialLocaleEnhancements(),
  })
  applyLocaleState(resolvedLocale)
}
