import { createI18n } from 'vue-i18n'

// Minimal fallback messages for initial render (before locale loads)
const minimalMessages = {
  common: {
    loading: 'Loading...',
  },
}

export type LocaleKey =
  | 'ca-ES'
  | 'cs-CZ'
  | 'da-DK'
  | 'de-DE'
  | 'el-GR'
  | 'en-GB'
  | 'en-US'
  | 'es-ES'
  | 'fr-FR'
  | 'ga-IE'
  | 'hr-HR'
  | 'hu-HU'
  | 'it-IT'
  | 'ja-JP'
  | 'ko-KR'
  | 'ml-IN'
  | 'nb-NO'
  | 'nl-NL'
  | 'pl-PL'
  | 'pt-BR'
  | 'pt-PT'
  | 'ro-RO'
  | 'ru-RU'
  | 'sk-SK'
  | 'sv-SE'
  | 'zh-CN'
  | 'zh-TW'

const LOCALE_KEY = 'zimaos-echo-locale'

// Map browser language codes to our locale keys
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
] as const

function getDefaultLocale(): LocaleKey {
  const saved = localStorage.getItem(LOCALE_KEY)
  if (saved && localeOptions.some((o) => o.value === saved)) {
    return saved as LocaleKey
  }

  const browserLang = navigator.language
  // Try exact match first
  const exactMatch = browserLocaleMap[browserLang]
  if (exactMatch) {
    return exactMatch
  }
  // Try language code only
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
  locale: 'en-US', // Start with en-US, will switch in initLocale after loading
  fallbackLocale: 'en-US',
  missingWarn: false,
  fallbackWarn: false,
  messages: {
    'en-US': minimalMessages, // Minimal messages, full locale loaded async
  },
})

// Track loaded locales
const loadedLocales = new Set<LocaleKey>()

// Lazy load locale messages
async function loadLocaleMessages(locale: LocaleKey): Promise<void> {
  if (loadedLocales.has(locale)) {
    return
  }

  try {
    const messages = await import(`./locales/${locale}.ts`)
    i18n.global.setLocaleMessage(locale, messages.default)
    loadedLocales.add(locale)
  } catch (error) {
    console.warn(`Failed to load locale ${locale}, falling back to en-US`, error)
  }
}

export async function setLocale(locale: LocaleKey): Promise<void> {
  // Always ensure fallback locale is fully loaded so missing keys in other locales
  // cleanly fall back to English instead of showing raw translation keys.
  await loadLocaleMessages('en-US')
  await loadLocaleMessages(locale)
  ;(i18n.global.locale as { value: string }).value = locale
  localStorage.setItem(LOCALE_KEY, locale)
  document.documentElement.lang = locale
}

export function getLocale(): LocaleKey {
  return i18n.global.locale.value as LocaleKey
}

// Initialize: always load the default locale (including en-US)
export async function initLocale(): Promise<void> {
  const defaultLocale = getDefaultLocale()
  await setLocale(defaultLocale)
}
