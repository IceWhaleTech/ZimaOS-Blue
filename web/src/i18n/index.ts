import { createI18n } from 'vue-i18n'

type LocaleMessages = Record<string, unknown>
type LocaleModule = { default: LocaleMessages }

// Minimal fallback messages for initial render before the selected locale finishes loading.
const minimalMessages = {
  common: {
    loading: 'Loading...',
  },
} satisfies LocaleMessages

export const localeKeys = [
  'ca-ES',
  'cs-CZ',
  'da-DK',
  'de-DE',
  'el-GR',
  'en-GB',
  'en-US',
  'es-ES',
  'fr-FR',
  'ga-IE',
  'hr-HR',
  'hu-HU',
  'it-IT',
  'ja-JP',
  'ko-KR',
  'ml-IN',
  'nb-NO',
  'nl-NL',
  'pl-PL',
  'pt-BR',
  'pt-PT',
  'ro-RO',
  'ru-RU',
  'sk-SK',
  'sv-SE',
  'zh-CN',
  'zh-TW',
] as const

export type LocaleKey = (typeof localeKeys)[number]

export type LocaleDirection = 'ltr' | 'rtl'

const LOCALE_KEY = 'zimaos-blue-locale'
const RTL_LANGUAGE_CODES = new Set(['ar', 'ckb', 'fa', 'he', 'ps', 'ur'])
const localeKeySet = new Set<LocaleKey>(localeKeys)

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

export const localeOptions = [
  { value: 'ca-ES', label: 'Català' },
  { value: 'cs-CZ', label: 'Čeština' },
  { value: 'da-DK', label: 'Dansk' },
  { value: 'de-DE', label: 'Deutsch' },
  { value: 'el-GR', label: 'Ελληνικά' },
  { value: 'en-GB', label: 'English (UK)' },
  { value: 'en-US', label: 'English (US)' },
  { value: 'es-ES', label: 'Español' },
  { value: 'fr-FR', label: 'Français' },
  { value: 'ga-IE', label: 'Gaeilge' },
  { value: 'hr-HR', label: 'Hrvatski' },
  { value: 'hu-HU', label: 'Magyar' },
  { value: 'it-IT', label: 'Italiano' },
  { value: 'ja-JP', label: '日本語' },
  { value: 'ko-KR', label: '한국어' },
  { value: 'ml-IN', label: 'മലയാളം' },
  { value: 'nb-NO', label: 'Norsk bokmål' },
  { value: 'nl-NL', label: 'Nederlands' },
  { value: 'pl-PL', label: 'Polski' },
  { value: 'pt-BR', label: 'Português (Brasil)' },
  { value: 'pt-PT', label: 'Português (Portugal)' },
  { value: 'ro-RO', label: 'Română' },
  { value: 'ru-RU', label: 'Русский' },
  { value: 'sk-SK', label: 'Slovenčina' },
  { value: 'sv-SE', label: 'Svenska' },
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
] as const satisfies readonly { value: LocaleKey; label: string }[]

export function getLocaleDirection(locale: string): LocaleDirection {
  const normalized = locale.trim().toLowerCase()
  const languageCode = normalized.split(/[-_]/)[0]
  return languageCode && RTL_LANGUAGE_CODES.has(languageCode) ? 'rtl' : 'ltr'
}

function getDefaultLocale(): LocaleKey {
  if (typeof localStorage !== 'undefined') {
    const saved = localStorage.getItem(LOCALE_KEY)
    if (saved && localeKeySet.has(saved as LocaleKey)) {
      return saved as LocaleKey
    }
  }

  const browserLang = typeof navigator !== 'undefined' ? navigator.language : 'en-US'
  const exactMatch = browserLocaleMap[browserLang]
  if (exactMatch) {
    return exactMatch
  }

  const langCode = browserLang.split('-')[0]
  if (langCode) {
    const langMatch = browserLocaleMap[langCode]
    if (langMatch) {
      return langMatch
    }
  }

  return 'en-US'
}

export const i18n = createI18n({
  legacy: false,
  locale: 'en-US',
  fallbackLocale: 'en-US',
  missingWarn: false,
  fallbackWarn: false,
  messages: {
    'en-US': minimalMessages,
  },
})

type LocaleComposerBridge = {
  setLocaleMessage: (locale: string, message: LocaleMessages) => void
  locale: { value: string }
}

const localeLoaders: Record<LocaleKey, () => Promise<LocaleModule>> = {
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
}

const loadedLocales = new Set<LocaleKey>()
const loadingLocales = new Map<LocaleKey, Promise<LocaleKey>>()

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

async function loadLocaleMessages(locale: LocaleKey): Promise<LocaleKey> {
  if (loadedLocales.has(locale)) {
    return locale
  }

  const existing = loadingLocales.get(locale)
  if (existing) {
    return existing
  }

  const pending = (async () => {
    try {
      const module = await localeLoaders[locale]()
      ;(i18n.global as unknown as LocaleComposerBridge).setLocaleMessage(locale, module.default)
      loadedLocales.add(locale)
      return locale
    } catch (error) {
      if (locale === 'en-US') {
        console.warn(`Failed to load locale ${locale}, continuing with minimal fallback`, error)
        return 'en-US'
      }

      console.warn(`Failed to load locale ${locale}, falling back to en-US`, error)
      return loadLocaleMessages('en-US')
    } finally {
      loadingLocales.delete(locale)
    }
  })()

  loadingLocales.set(locale, pending)
  return pending
}

export async function setLocale(locale: LocaleKey): Promise<void> {
  const resolvedLocale = await loadLocaleMessages(locale)
  applyLocaleState(resolvedLocale)
}

export function getLocale(): LocaleKey {
  return (i18n.global as unknown as LocaleComposerBridge).locale.value as LocaleKey
}

export async function initLocale(): Promise<void> {
  const defaultLocale = getDefaultLocale()
  const resolvedLocale = await loadLocaleMessages(defaultLocale)
  applyLocaleState(resolvedLocale)
}
